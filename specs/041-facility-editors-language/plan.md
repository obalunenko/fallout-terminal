# Implementation Plan: Facility Editors and Language

**Branch**: `041-facility-editors-language` | **Date**: 2026-10-31 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/041-facility-editors-language/spec.md`

**Scale note**: About 40 files change, in four areas:

- One domain validation rule.
- Removal of the Overseer recovery path through session, control, the App and the bridge.
- The Overseer frontend, which carries most of the work.
- The demo plus about twenty browser specs.

Watch for three things:

- The existing browser specs locate dialogs by old names («ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ», «ПРИВЯЗКА ОБЪЕКТА», «ЗАВИСИМОСТИ ОБЪЕКТА»). They also assert row text («Загрязнён · ● 1») that the demo inventory repeats as markers.
- The desktop method count drops from 47 to 46.
- `overseer.js` is 8,100 lines. Keep each editor's code together and replace old functions instead of leaving them next to the new ones.

## Summary

Five Overseer editors are rebuilt on the existing authoring save (039), and session JSON does not change.

- **Device dialog**: state chips, a «из → в» transition matrix with an inline transition panel, type templates, and identifiers folded into «Технические идентификаторы».
- **Fault dialog**: «Неисправность» shows «Как появляется» and «Как устраняется». Both are derived from the transitions' fault effects and edited only there.
- **Display rules**: the «Правило отображения» dialog takes four type tabs and a «КОГДА / ГДЕ / БЛОК / ТЕКСТ» sentence. A new «Действие команды» dialog takes over command actions.
- **Device list**: badges, filters, and a «Неисправности терминалов» group that reuses feature 040's fault switch.

The Overseer can now clear any fault by hand. The domain rule that every fault needs a clearing recovery goes away, and so does the private `RecoverFacilityCondition` path, which only served «Восстановление оператором». Every Overseer error is shown through one helper. It shows Russian text with a next step and puts the original under «подробности для разработчика». A catalog in `overseer.js` maps the player configuration and session service messages, and Go tests keep the catalog in step with the Go messages. A browser test checks every Overseer source file against the glossary in `docs/overseer-glossary.md`.

## Technical Context

**Language/Version**: Go 1.27; Overseer browser JavaScript built with Node.js 26.8.1+ (`.nvmrc`) and Vite
**Primary Dependencies**: Wails v3 private desktop bridge; protobuf with Buf in isolated `tools/` modules; Playwright under `tests/browser/`
**Storage**: Portable session JSON version 1 through `internal/session`. Templates, list filters, and the error catalog are frontend-only.
**Testing**: Go colocated table tests with testify; `task test`, `task test:race`; `task browser:test`; `task bindings:check`; `task proto:breaking`
**Target Platform**: macOS desktop Overseer (minimum window 900×600); player browsers unchanged
**Constraints**: Every authoring change goes through `commitFacilityEdit` (revision check, «Отменить»). Every play change goes through feature 040's `SetFacilityConditionActive`. No new session fields.

## Project Structure

### Documentation (this feature)

```text
specs/041-facility-editors-language/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── contracts/
│   ├── overseer-ui.md
│   └── private-desktop.md
└── checklists/requirements.md
docs/overseer-glossary.md          # new: the glossary FR-034 names
```

### Source Code (repository root)

```text
internal/domain/
├── validate.go              # validateConditions: no clearing recovery required (R1)
└── validate_test.go
internal/session/
├── facility.go              # private-recovery world-action branch removed (R1)
└── facility_test.go, service_messages_test.go (new, R12)
internal/playerconfig/messages_test.go   # new: every failure() text has a catalog entry (R12)
internal/control/facility.go             # RecoverFacilityCondition and its helpers removed
proto/fallout/terminal/private/v1/desktop.proto   # RecoverFacilityConditionRequest removed
proto/compatibility-baseline.binpb, internal/gen/…, proto/schema-revision.txt   # regenerated
app.go, app_contract.go, desktop_service.go, main.go, app_test.go, app_contract_test.go
scripts/wails-bindings-check.sh          # method count 47 → 46
frontend/overseer/bindings/…/desktopservice.js   # regenerated
frontend/overseer/src/
├── index.html               # device, fault, program, rule, action, dependency dialogs; list filters; terminal-fault group
├── overseer.js              # editors, templates, list, links, error helper and catalog, renames
├── overseer.css             # chips, matrix, badges, filters, sentence grid
└── desktop-api.js           # recoverFacilityCondition removed
internal/platform/assets_test.go         # demo invariant no longer requires a recovery entry
sessions/demo.json, sessions/demo-capability-paths.md
tests/browser/
├── facility-device-editor.spec.mjs      # new: US1, US3
├── facility-fault-dialog.spec.mjs       # new: US2
├── facility-display-rules.spec.mjs      # new: US4
├── facility-device-list.spec.mjs        # new: US5
├── facility-vocabulary.spec.mjs         # new: US6
├── overseer-errors.spec.mjs             # new: US7
├── helpers/, fixtures/, fixture-server/main.go, desktop-api.spec.mjs, demo-session.spec.mjs
└── facility-*.spec.mjs                  # existing specs move to the new names and markup
```

**Structure Decision**: Everything stays in the existing Overseer files, because `overseer.js` and `index.html` are loaded as two classic modules, and the fixture server rewrites those paths in many routes. Go changes are removals, plus one relaxed rule. The only new non-test file is the glossary.

## Call Paths

The companion block templates are missing in this install (`plan-blocks.py` cannot find `call-paths.md`), so these blocks are written by hand.

| Path | Steps |
|---|---|
| Device dialog save | chips, matrix cell, or template → draft rows that keep `facilityOriginal` → `validateFacilityDeviceDraft` → `commitFacilityEdit('Устройство', mutate)` → `commitAuthoringEdit` → `SaveAuthoring` (revision check) |
| Fault dialog save | fault fields plus the «Как появляется» and «Как устраняется» rows → one `mutate` that writes the condition and every touched transition's `conditionEffects`, and prunes invalidated `recovery[]` entries (R2) → `commitFacilityEdit` |
| Program from a fault | «+ Программа восстановления…» → program dialog over the fault dialog, prefilled from the fault draft → its own `commitFacilityEdit` → the fault dialog re-derives its lists |
| Display rule save | tab, «КОГДА», «ГДЕ», «БЛОК», «ТЕКСТ» → conflict check against `facilityBindingCatalog` → `mutate` replaces the variant in place → `commitFacilityEdit` |
| Command action save | «Действие команды» (command fixed, or picked) → transitions or program → `stateChange.facilityAction` plus `completedName` and `confirmationText` when missing → `commitFacilityEdit` |
| Terminal fault switch | «Неисправности терминалов» row switch → `runFacilityFaultSwitch` → `desktopAPI.setFacilityConditionActive` (040) → result shown in `#facilityTerminalFaultStatus` |
| Error shown | any `result.error` → `renderOverseerError(sink, result, area)` → catalog lookup (code or text) → Russian text with a next step and a «подробности для разработчика» disclosure |
| Dependency link | `inspectSelectedFacilityDevice` → `BuildFacilityDependencyReport` (unchanged) → `describeFacilityDependency` (words, from the session) → named destination button → `navigateToFacilityDependency` |

## Screens

| Screen | What changes |
|---|---|
| «УСТРОЙСТВО» dialog | Name, type, template line, state chips, matrix, inline transition panel, «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ» at the bottom |
| «НЕИСПРАВНОСТЬ» dialog | Name, category, where it acts, what it blocks, start position, «КАК ПОЯВЛЯЕТСЯ», «КАК УСТРАНЯЕТСЯ»; no checkbox |
| «ПРОГРАММА ВОССТАНОВЛЕНИЯ» dialog | One list of transition rows, the commands that run it; no instruction paragraph |
| «ПРАВИЛО ОТОБРАЖЕНИЯ» dialog | Four tabs; the sentence grid «КОГДА / ГДЕ / БЛОК / ТЕКСТ»; the hint about other states |
| «ДЕЙСТВИЕ КОМАНДЫ» dialog | New: the command, «ПЕРЕХОДЫ» or «ПРОГРАММА ВОССТАНОВЛЕНИЯ», and the player texts when missing |
| «ЗАВИСИМОСТИ УСТРОЙСТВА» dialog | Each link in words with a named button; the technical path under «подробности для разработчика» |
| Device list | Search, three filters with counts, left-aligned rows with badges, «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» group |
| «НЕИСПРАВНОСТИ» tab | The «ВЫБРАННОЕ УСЛОВИЕ» recovery panel is removed; rows open the fault dialog |
| Error sinks | Russian text plus a disclosure everywhere `result.error` was shown |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Accepted desktop runtime | PASS. Wails v3 private bridge only. |
| II. Protobuf source of truth | PASS. The only contract change removes `RecoverFacilityConditionRequest`. The bindings are regenerated. |
| III. Server-authoritative state | PASS. Authoring edits keep the 039 revision check. The terminal fault switch uses the 040 operation. Conflict checks in dialogs are advisory, and the domain validators still decide. |
| IV. Public/private separation | PASS. Only a private desktop method is removed. Players gain no capability. |
| V. Safe schema evolution | PASS with a documented migration (R1). The removed message was added in feature 035 on this unreleased stack (035 is not in `develop`). The compatibility baseline is regenerated, as 035 and 039 did. |
| VI. Session JSON version 1 | PASS. No field is added or renamed, and stored `privateOverseerAction` values are never written or removed. Validation now accepts more documents than before. A file saved with a fault that has no clearing entry will not open in builds from 035 to 040, which never shipped. |
| VII. Complete cutovers | PASS. Each cutover removes what it replaces: the recovery path and its panel, the binding dialog's type select and command-action kind, the starting-state select, the separate first-transition fields in the program dialog, and their tests. |
| VIII. Player activity observable | PASS. No new player activity. The terminal fault switch is logged by the 040 audit, and terminal faults get no feed entry, because the feed belongs to devices. |
| IX. Demo sessions | PASS. Demo paths cover each FR-044 case: a second reactor transition between the same states, a program with two transitions, and the new routes in `sessions/demo-capability-paths.md`. |
| Testing and quality gates | PASS. Go table tests, catalog sync tests, race tests, Buf, the bindings check, a browser test per story, and `go fix ./...` before commit. |
| Development workflow, step 1 (branch from `develop`) | Justified violation; see Complexity Tracking. |

Re-checked after Phase 1: the contracts add no proto message and no session field, and they keep every DOM id that tests and helpers rely on. That id list is in `contracts/overseer-ui.md`. No assessment changes.

## Implementation Phases

### Phase 0: Research (done)

`research.md` records the decisions R1 to R16.

### Phase 1: Contracts and data design (done)

`data-model.md`, `contracts/overseer-ui.md`, `contracts/private-desktop.md`.

### Phase 2: Foundations

1. Relax `validateConditions` and remove the recovery path (domain, session, control, App, proto, bindings, `desktop-api.js`), with tests and the 47 → 46 method count.
2. Build `renderOverseerError` and the message catalog, with the two Go catalog sync tests.
3. Add the glossary document and the vocabulary browser test. The test starts red and turns green as the stories rename the copy.
4. Add shared helpers: a summary parameter for `facilityTechnicalDetails`, a folder-path element catalog, and conflict lookups over `facilityBindingCatalog`.

### Phase 3: Stories

- US1, then US3 (the device dialog). US2 follows (the fault and program dialogs, the terminal-fault group).
- Then US4 (the rule and action dialogs), US5 (the list), US6 (renames and links), and US7 (error sinks).
- Each story gets its browser test, its existing-test updates, and its demo inventory rows.

### Phase 4: Integration

- Demo data and capability paths.
- The full `task browser:test`, `task test:race`, `task proto:breaking` and `task bindings:check`.
- A packaged-app check of the device dialog at 1440×900 (SC-001).
- `validation.md`.

## Verification Plan

| Requirement group | Evidence |
|---|---|
| FR-001 to FR-012, SC-001 to SC-003 | `facility-device-editor.spec.mjs` at a 1440×900 viewport, then a packaged-app check |
| FR-013 to FR-021, SC-004 | `facility-fault-dialog.spec.mjs` (every demo fault against session data), `internal/domain/validate_test.go`, `desktop-api.spec.mjs` without the recovery method |
| FR-022 to FR-029, SC-005 | `facility-display-rules.spec.mjs` with a connected player terminal for each type |
| FR-030 to FR-033, SC-006 | `facility-device-list.spec.mjs` on the demo, then live play changes |
| FR-034 to FR-038, SC-007 | `facility-vocabulary.spec.mjs`, plus a dependency-link walk in `facility-display-rules.spec.mjs` and `facility-fault-dialog.spec.mjs` |
| FR-039 to FR-041, SC-008 | `overseer-errors.spec.mjs` (fixture-triggered failures from each area); `internal/playerconfig/messages_test.go`, `internal/session/service_messages_test.go` |
| FR-042 to FR-045, SC-009 | `demo-session.spec.mjs`, `internal/platform/assets_test.go`, session round-trip tests |

## Complexity Tracking

| Violation | Why needed | Simpler alternative rejected |
|---|---|---|
| Development Workflow step 1 (branch from `develop`): this branch starts from `040-facility-play-controls` (PR #61), which stacks on 039 (#59), 038 (#58) and 035. The integrator of the 035 stack owns it. It lasts until 035 to 040 merge; then 041 is rebased onto `develop`. | The device card, the fault switch, and the authoring save this feature builds on exist only in 035 to 040. | Waiting for the merges blocks F-10 to F-14. Folding this work into 040 would mix editor redesign into a play-controls PR. |
