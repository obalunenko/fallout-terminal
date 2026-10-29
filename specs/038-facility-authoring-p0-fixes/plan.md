# Implementation Plan: Facility Authoring P0 Fixes

**Branch**: `038-facility-authoring-p0-fixes` | **Date**: 2026-10-29 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/038-facility-authoring-p0-fixes/spec.md`

## Summary

Fix four defects confirmed in the packaged Overseer: Cyrillic names that collapse into identical
identifiers (F-01), labels that stay visible when their controls are hidden (F-02), device actions
that silently disappear or get invented texts when a command changes mode (F-03), and fault
starting activity that cannot be authored (F-04). Every fix lives in the Overseer frontend. The
persistence schema, Go domain, server validation, reset semantics, Wails bridge, and player
protocol already support the required values (`research.md`, "Existing behavior confirmed").

## Technical Context

**Language/Version**: Browser JavaScript modules (Overseer) built with Node.js 26.8.1+ and Vite
8.3.4; Go 1.27 unchanged

**Primary Dependencies**: Existing only — Wails v3.0.0-beta.28 bindings, Vite 8.3.4, Playwright
1.64.0; no new dependency

**Storage**: Portable session JSON version 1, unchanged shape

**Testing**: Playwright specs under `tests/browser/` against the in-process fixture server; Go
gates run as regression checks only

**Target Platform**: Wails desktop Overseer on all supported platforms

**Project Type**: Go modular desktop monolith with a Wails Overseer frontend

**Performance Goals**: Identifier suggestions update within one input event; no additional server
round trips

**Constraints**: No change to session JSON, protobuf, Wails methods, or player behavior; existing
identifiers never rewritten; work stays compatible with the explicit facility draft until feature
039 unifies saving

**Scale/Scope**: One Overseer; facility sizes like the ten-device demo

## Constitution Check

*GATE: Must pass before Phase 0 research and be re-checked after Phase 1 design.*

| Principle | Assessment | Evidence |
|---|---|---|
| I. Govern the accepted desktop runtime | PASS | Overseer frontend only; no runtime or composition change |
| II. Protobuf as contract source of truth | PASS | `initial_active` already in `session.proto:133`; no schema edits or generated-file changes |
| III. ConnectRPC, server-authoritative state | PASS | Save, reset, and current-value protection stay server-side (`internal/control/facility.go:826–847, 1278–1284`) |
| IV. Separate public and private capabilities | PASS | No player endpoint changes |
| V. Evolve schemas safely | PASS | No schema evolution |
| VI. Preserve portable session JSON version 1 | PASS | Shape unchanged; identifiers never rewritten; round-trip check in quickstart |
| VII. Complete cutovers | PASS | The invented-text fallback (`stateChange ||=`) and the old identifier helper are removed, not kept beside the replacements |
| VIII. Player activity observable | PASS | No new player request or outcome; existing audit records unaffected |
| IX. Demo sessions complete and coherent | PASS | `demo.json` unchanged; `sessions/demo-capability-paths.md` documents the new authoring routes |

Template checklist:

- [x] Runtime ownership remains within `frontend/overseer/src/`, `tests/browser/`, and `sessions/` documentation.
- [x] No cross-boundary contract changes; UI contract documented in `contracts/overseer-ui.md`.
- [x] Shared state remains server-authoritative and reconnect-safe.
- [x] Wails exposure, CSP, origin, and secret protections untouched.
- [x] Session compatibility defined: version 1 unchanged, defaults unchanged.
- [x] Runtime-only form state stays in memory (see `data-model.md`).
- [x] No new dependencies.
- [x] Verification uses Taskfile, Vite, and Playwright gates; unavailable checks will be reported.
- [x] Naming and style follow `overseer.js` conventions (DOM-id constants, `setFacilityFeedback`, upper-case Russian copy).

**Branching exception**: Development Workflow step 1 requires branching from `develop`. Feature 035
is not yet merged there, so this branch starts from `035-facility-state-system` (as did 036 and
037). The exception is recorded in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/038-facility-authoring-p0-fixes/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── overseer-ui.md
├── checklists/
│   └── requirements.md
└── tasks.md            # next step
```

### Source Code (repository root)

```text
frontend/overseer/src/
├── overseer.css        # global [hidden] rule; row-error and notice styles
├── index.html          # fault starting-activity radio group
└── overseer.js         # identifier helper, row validation, mode decision, draft guard,
                        # binding texts, fault activity, discard helper extraction
tests/browser/
├── helpers/overseer-forms.mjs                # new: workspace opening and orphan-label assertion
├── facility-p0-command-actions.spec.mjs      # new: US1 journeys
├── facility-p0-identifiers.spec.mjs          # new: US2 journeys
├── facility-p0-fault-activity.spec.mjs       # new: US3 journeys
├── facility-p0-visible-fields.spec.mjs       # new: US4 sweep of every dialog and mode
└── facility-authoring-ux.spec.mjs            # adjust journeys that attach actions to ordinary commands
sessions/
└── demo-capability-paths.md         # authoring-route documentation
```

**Structure Decision**: All behavior belongs to the Overseer authoring workflow that features
035–037 placed in `frontend/overseer/src/overseer.js`; extend those seams without a new module,
framework, store, or transport. Tests follow the existing facility Playwright suites and fixture
server.

## Contract and State Design

### Persistent JSON

N/A — no field changes. `initialActive` and `currentActive` already exist; a missing
`initialActive` keeps meaning "working". A new fault's `currentActive` is set from `initialActive`
by the draft and again by the server on save.

### Wails Bridge and Runtime Events

N/A — existing `saveFacilityAuthoring` and terminal autosave are reused. The command-form draft
guard calls the same save/discard paths as the facility workspace buttons.

### Protobuf/ConnectRPC and HTTP Assets

N/A.

### Runtime-State Lifecycle

Unchanged server lifecycle. New transient Overseer form state (pending mode decision, draft guard,
row errors) is documented in `data-model.md` and never persisted.

### Platform, Tunnel, and Packaging

N/A beyond the normal Overseer asset build embedded by `task package`.

## Implementation Phases

### Phase 0: Research and Decisions — done

See `research.md` R1–R8: identifier algorithm and prefix rules, row validation, global `[hidden]`
rule with a sweep test, inline mode decision, dirty-draft guard, binding texts, fault starting
activity, tests and documentation.

### Phase 1: Contracts and Data Design — done

`contracts/overseer-ui.md` (DOM ids, copy, focus), `data-model.md` (identifier rules, command and
fault rules, transient state), `quickstart.md` (automated and interactive validation).
Constitution Check re-run after design: still PASS.

### Phase 2: Foundations

- Add the global `[hidden]` rule and the label-sweep test helper first (US4); it protects the forms
  touched later.
- Replace `stableFacilityID` with the new helper and update its five call sites (US2).

### Phase 3: Story slices (each independently testable)

1. **US4 — visible fields**: sweep all dialogs and modes; fix any remaining orphan outside the
   global rule.
2. **US2 — identifiers and row validation**: helper tests through the device, fault, and program
   dialogs; per-row messages and focus in the device dialog.
3. **US1 — device actions**: extract `discardFacilityDraft()`; inline mode decision; device-action
   block visibility; draft guard; binding-dialog texts for ordinary commands; remove the
   `stateChange ||=` fallback.
4. **US3 — fault starting activity**: radio group, draft write, row and summary wording.

### Phase 4: Integration, documentation, packaging

- Update `sessions/demo-capability-paths.md`.
- Run the full facility and demo browser suites, frontend build, Go regression gates, and
  `task package`; repeat quickstart steps in the packaged app with a demo copy.
- Record results in `validation.md`.

## Verification Plan

| Surface | Automated check | Interactive/manual check | Expected result |
|---|---|---|---|
| Go domain/services | `task test` (regression) | N/A | Passes unchanged |
| Concurrent runtime | N/A — not affected | N/A | — |
| Go quality | `task fmt:check`, `task vet`, `task lint` | N/A | Pass unchanged |
| Overseer frontend | `task frontend:build` | Quickstart steps 1–6 in packaged app | Build succeeds; all six journeys behave as specified |
| Browser journeys | `task browser:test` for the four `facility-p0-*.spec.mjs` files plus existing facility and demo suites | N/A | All pass, including label sweep across every dialog and mode |
| Session compatibility | Demo save round-trip in quickstart | Diff of saved demo copy | Identifiers, modes, actions, activities identical |
| Package | `task package` | Launch packaged app, open demo copy | Launches; fixes visible |
| Signed macOS / public provider | N/A — not affected | N/A | — |

## Project-Specific Complexity Factors

- Two save models coexist (terminal autosave and facility draft); the draft guard avoids stale
  drafts without unifying them, which is feature 039's job.
- The command form reads both the saved node and the draft node; the effective device action must
  come from the draft when one exists.
- Identifier helpers run on every keystroke; the algorithm must stay deterministic and cheap.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| Development Workflow step 1 (branch from `develop`): this branch starts from `035-facility-state-system`. Owner: the integrator of feature 035 (PR #50). Bounded duration: until PR #50 merges; then 038 is rebased onto `develop` before its own integration. | The defects being fixed exist only in feature 035/036 code, which `develop` does not contain yet. | Waiting for the 035 merge blocks P0 fixes for code under active review; committing the fixes directly to `035-facility-state-system` would enlarge PR #50 and mix review findings with the feature it reviews. |
