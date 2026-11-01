# Implementation Plan: Facility Recheck Fixes

**Branch**: `042-facility-recheck-fixes` | **Date**: 2026-11-01 | **Spec**: [facility-recheck-fixes.spec.md](facility-recheck-fixes.spec.md)

**Input**: Feature specification from `/specs/042-facility-recheck-fixes/facility-recheck-fixes.spec.md`

**Scale note**: About 25 files change, in five areas: two proto files and their generated code, the Go control service and player adapter, the player client, the Overseer frontend, and the browser fixture with its journeys. Watch for three things:

- The schema change moves the reviewed schema-revision digest pinned in `scripts/wails-v3-contract-check.sh`, and the Wails bindings regenerate because the feed record gains fields.
- A refused command with a device action already writes feed entries for the devices it targets. The new fault-block entries must merge with those, one entry per device, never two.
- The diagnostics fixture runs the real `control.Service`, yet its blocked entry opens today. The first task must find out why before the journeys are rewritten.

## Summary

A request that an active fault blocks gets its own player reason, `ACTION_REASON_FAULT_BLOCKED`, in place of the generic `ACTION_REASON_INVALID_ACTION`. The player client shows «ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.». The control service also records the refusal in the feed of every device whose fault blocked it. The existing `FacilityChangePlayerRefusal` record gains the blocked entry, the capability, the blocking faults, and an attempt count, and a repeat of the newest refusal bumps that count instead of adding an entry.

The template rule moves to the transition matrix cell, and the diagnostics fixture is made to fail when a blocked entry opens. The remaining work is CSS and copy in the Overseer, plus records for features 033 and 041.

## Technical Context

**Language/Version**: Go 1.27; browser JavaScript built with Node.js 26.8.1+ (`.nvmrc`) and Vite
**Primary Dependencies**: Wails v3 private desktop bridge; protobuf with Buf in isolated `tools/` modules; Playwright under `tests/browser/`
**Storage**: Portable session JSON version 1, unchanged. The feed and its attempt counts live in the process runtime for the current run.
**Testing**: Go colocated table tests with testify; `task test`, `task test:race`; `task proto:check`, `task proto:breaking`, `task bindings:check`, `task wails:pins:check`; `task browser:test`
**Target Platform**: macOS desktop Overseer at 1440×900; player browsers
**Constraints**: Server-authoritative refusals; no fault name reaches a player; additive proto change only.

## Project Structure

### Documentation (this feature)

```text
specs/042-facility-recheck-fixes/
├── facility-recheck-fixes.spec.md
├── plan.md
├── research.md
├── data-model.md
├── contracts/
│   ├── player-action-reason.md
│   ├── facility-feed-refusal.md
│   └── overseer-ui.md
├── checklists/requirements.md
└── tasks.md            (speckit.companion.tasks)
```

### Source Code (repository root)

```text
proto/
├── fallout/terminal/player/v1/player.proto          # ACTION_REASON_FAULT_BLOCKED
├── fallout/terminal/private/v1/coordination.proto   # FacilityChangePlayerRefusal fields
└── schema-revision.txt                              # synced by proto:generate
internal/
├── gen/fallout/terminal/...                         # regenerated
├── domain/model.go                                  # ActionReasonFaultBlocked
├── domain/facility.go                               # refusal record fields, clone
├── control/facility.go                              # blocking conditions, refusal facts
├── control/feed.go                                  # merge and repeat count
├── control/service.go                               # fault-block branch of player actions
└── player/adapter.go                                # reason to proto
frontend/
├── client/client.js                                 # reason name and message
├── client/gen/...                                   # regenerated
└── overseer/
    ├── bindings/...                                 # regenerated
    └── src/{overseer.js,overseer.css,index.html,desktop-api.js}
scripts/wails-v3-contract-check.sh                   # schema-revision pin
sessions/demo-capability-paths.md                    # spores path
tests/browser/
├── fixture-server/main.go                           # diagnostics fixture
├── facility-diagnostics.spec.mjs
├── facility-play-feed.spec.mjs
├── facility-device-templates.spec.mjs
├── facility-device-list.spec.mjs
├── facility-fault-dialog.spec.mjs
├── facility-workspace-layout.spec.mjs
└── connectrpc-player.spec.mjs
specs/033-entry-content-state/{tasks.md,validation.md}
specs/041-facility-editors-language/spec.md
docs/overseer-glossary.md
```

**Structure Decision**: Extend the existing refusal record and player reason rather than add a feed variant or a side channel; every change lands in a file the 035 to 041 features already own.

## Call Paths

The companion block templates are missing in this install (`plan-blocks.py` cannot find `call-paths.md`), so these blocks are written by hand, as in 041.

| Path | Steps |
|---|---|
| Fault-blocked player request | player action → `Service.DispatchPlayerAction` → `preparePlayerAction` → `requestedFacilityCapability` → `blockingFacilityConditions` (new, returns the active conditions) → `rejectedAction(…, ActionReasonFaultBlocked, …)` → `recordFacilityBlockRefusal` (new) → `appendFacilityChanges` (merge, repeat count) → master snapshot effect → audit `player.action.outcome` with facility facts |
| Player sees the reason | `ActionResult.Reason` → `player/adapter.go` `ACTION_REASON_FAULT_BLOCKED` → `client.js` `actionReasonName` → `ACTION_REASON_MESSAGES['fault-blocked']` → `#playerNotice` |
| Overseer feed | coordination snapshot `facilityChanges` → `desktop-api.js` `normalizeFacilityChange` (new fields) → `renderFacilityPlayFeed` → `facilityFeedText` (target, fault names, attempts) |
| Template check | device dialog open or type change → `renderFacilityDeviceTemplate` → `facilityDraftTemplateAdditions` (cell rule) → `explainControl(… 'ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ')` |

## Screens

| Screen | What changes |
|---|---|
| Player terminal | Fault-block refusals read «ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.» |
| Device card «ПОСЛЕДНИЕ ИЗМЕНЕНИЯ» | A refusal names what was blocked, the faults, the terminal, and «×N» when repeated |
| «УСТРОЙСТВО» dialog | The template line says «ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ» when every cell is filled |
| Device sidebar | «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» collapses to its heading with an active count; the list takes the height; rows and focus outlines draw whole |
| Workspace header | No empty band while the location status is empty |
| «НЕИСПРАВНОСТЬ» dialog | «В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА» drawn as Overseer segmented controls |
| Terminal tree | «[КОМАНДА] Название» with a space |
| Rule, action, and program rows | «ИЗМЕНИТЬ» / «УДАЛИТЬ» in upper case |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Accepted desktop runtime | PASS. Wails v3 private bridge only; bindings regenerated. |
| II. Protobuf source of truth | PASS. The reason and the refusal fields are added to the proto first, then generated. |
| III. Server-authoritative state | PASS. The control service decides the refusal and records the feed entry; clients only render. |
| IV. Public/private separation | PASS. Players get one new enum value and no fault data. The fault IDs stay in the private coordination state. |
| V. Safe schema evolution | PASS. One enum value and four fields are added; `proto:breaking` runs against the existing baseline, which does not change. The schema-revision record and its pinned digest are updated. |
| VI. Session JSON version 1 | PASS. No session field changes; the feed is run-only. |
| VII. Complete cutovers | PASS. The generic reason stops being used for fault blocks; the name-based template rule is replaced, not kept beside the cell rule. |
| VIII. Player activity observable | PASS. Each fault-block refusal is in the retained log with terminal, capability, faults and correlation, without authored text. |
| IX. Demo sessions | PASS. The spores path is recorded in `sessions/demo-capability-paths.md`; the demo session itself does not change. |
| Testing and quality gates | PASS. Go table tests for the branch, merge and repeat; browser journeys per story; `go fix ./...` before commit; packaged-app check. |
| Development workflow | PASS. Branched from `develop` at `d50efcb`. |

Re-checked after Phase 1: the contracts add one enum value and four optional fields, keep every DOM id the journeys rely on, and add no session field. No assessment changes.

## Implementation Phases

### Phase 0: Research (done)

[research.md](research.md) settles the reason, the record shape, which feeds get an entry, the repeat rule, the log facts, the fixture investigation, and the layout choices.

### Phase 1: Contracts and data design (done)

[data-model.md](data-model.md) and [contracts/](contracts/).

### Phase 2: Foundations

Proto first: the enum value and the refusal fields, regenerated Go, ES and bindings, then the synced schema revision and its pin. Then the domain types and their clone.

### Phase 3: Stories

- US1: blocking conditions helper, the refusal branch, the feed merge, the log facts, the client message, the Overseer feed text, the demo inventory.
- US2: the cell rule and the 041 spec update.
- US3: find why the fixture entry opens, then the fixture text and the journeys.
- US4: CSS and copy, and the collapsible group with its remembered state.

### Phase 4: Integration

The 033 records, the full suites, and the packaged-app check with a connected player.

## Verification Plan

| Requirement group | Evidence |
|---|---|
| FR-001 to FR-006, SC-001, SC-002 | `internal/control` table tests for each capability, the merge and the repeat count; `facility-play-feed.spec.mjs`; `connectrpc-player.spec.mjs` for the message; packaged app with the demo spores path |
| FR-007 | `sessions/demo-capability-paths.md` and `demo-session.spec.mjs` |
| FR-008 to FR-011, SC-003 | `facility-device-templates.spec.mjs` on the demo door, an empty door, and a door with one renamed transition |
| FR-012 to FR-014, SC-004 | `facility-diagnostics.spec.mjs`, run once against a server with the block disabled to confirm it fails |
| FR-015 to FR-022, SC-005, SC-006 | `facility-device-list.spec.mjs`, `facility-workspace-layout.spec.mjs`, `facility-fault-dialog.spec.mjs`, `facility-vocabulary.spec.mjs` at 1440×900 |
| FR-023, SC-007 | `specs/033-entry-content-state/tasks.md` with no open box, and its validation record |
| FR-024, SC-008 | Session round-trip tests, the full browser suite, the packaged app |
