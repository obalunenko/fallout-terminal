# Implementation Plan: Unified Facility Authoring

**Branch**: `039-unified-facility-authoring` | **Date**: 2026-10-29 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/039-unified-facility-authoring/spec.md`

## Summary

Replace the two save models with one. Every terminal and facility edit, and every undo or redo, is
one `SaveAuthoring` request. The request is checked against a new runtime-only authoring revision
that live play never advances, and is published to players at once. The Overseer gains a shared
save bar with «ОТМЕНИТЬ», «ПОВТОРИТЬ», «ПРИМЕНИТЬ СНОВА» and «СБРОСИТЬ СЦЕНУ», keeps unapplied
input, and loses the facility draft, the draft guard, and the publish button (F-05).

A server-side presentation explanation, built from the same resolvers as the player projection,
feeds three things (F-06):

- two lists on each command form: everything the command changes, and the rules that decide what
  players see, with the winner marked;
- a terminal tree that shows player-visible names;
- one atomic `ResetScene` operation.

Preview returns the current and hypothetical projections with named differences, preselects a
different state, and is checked against the authoring revision, so it reflects the latest applied
edit (F-08). The session JSON shape does not change.

## Technical Context

**Language/Version**: Go 1.27; browser JavaScript (Overseer) built with Node.js 26.8.1+ (`.nvmrc`)
and Vite 8.3.4

**Primary Dependencies**: Existing only. Wails v3.0.0-beta.28 bindings, `google.golang.org/protobuf`
v1.36.12 with pinned Buf, Vite 8.3.4, Playwright 1.64.0. No new dependency.

**Storage**: Portable session JSON version 1, shape unchanged. The authoring revision and the
Overseer history are in memory only.

**Testing**: Colocated Go table tests (`internal/session`, `internal/control`, `internal/live`,
`app_contract_test.go`); `task test:race`; Playwright specs under `tests/browser/` against the
fixture server; protobuf and bindings gates.

**Target Platform**: Wails desktop Overseer on all supported platforms; players unaffected beyond
receiving edits at once.

**Project Type**: Go modular desktop monolith with a Wails Overseer frontend

**Performance Goals**:

- An applied edit shows «СОХРАНЕНИЕ…» before its save request resolves, and «СОХРАНЕНО» after one
  durable write.
- At most one save request is in flight; each edit is its own request, sent in order.
- Explanation and preview stay under 50 ms of Go time for demo-sized sessions (about 38 KiB, ten
  devices), checked by benchmarks.

**Constraints**:

- Session JSON v1 is unchanged.
- State stays server-authoritative, with revision checks.
- The narrow Wails allowlist is preserved.
- The private contract changes only additively, plus a documented removal of superseded messages.
- Player contract and `runtime.proto` are unchanged.
- Live state is never reversed by undo.

**Scale/Scope**: One Overseer, one broadcast, existing player limits; undo depth of 20 edits

## Constitution Check

*GATE: Must pass before Phase 0 research and be re-checked after Phase 1 design.*

| Principle | Assessment | Evidence |
|---|---|---|
| I. Govern the accepted desktop runtime | PASS | Changes stay in `main.go`/`app.go`/`desktop_service.go`/`app_contract.go`, `internal/{session,control,live}`, `frontend/overseer/src/`, `tests/browser/`, `sessions/` docs |
| II. Protobuf as contract source of truth | PASS | New requests and results are defined in `desktop.proto` with explicit adapters (`contracts/private-desktop.md`); no handwritten DTOs |
| III. ConnectRPC, server-authoritative state | PASS | Save merge, live-value protection, pending rebase, scene reset, explanation, and preview diff all run in Go; the Overseer converges on returned canonical sessions (research R1, R4, R10, R12, R13) |
| IV. Separate public and private capabilities | PASS | Three new private Wails methods; `scripts/wails-bindings-check.sh` allowlist updated; no player-facing route |
| V. Evolve schemas safely | PASS | Additions use new field numbers and enums with `UNSPECIFIED`. Five superseded private messages are removed under the documented migration in `contracts/private-desktop.md`, which Principle V permits; the baseline and digests are refreshed |
| VI. Preserve portable session JSON v1 | PASS | No field changes; the authoring revision is runtime-only; round-trip and projection tests cover all `sessions/` files (SC-007) |
| VII. Complete cutovers | PASS | Removes `SaveSession`, `SaveFacilityAuthoring`, `ResetFacility`, `ResetTerminalCommandStates`, `UpdateLiveTerminal` (Wails), the facility draft, the draft guard, the publish button, `Service.Save`, `preserveCanonicalFacility`, and their tests, fixtures, and scripts |
| VIII. Player activity observable | PASS | No new player request. Pending requests rejected by an authoring save use the existing rejection records. Scene reset emits one correlated audit with revisions and counts, and no authored content |
| IX. Demo sessions complete and coherent | PASS | `demo.json` already exercises every path; `sessions/demo-capability-paths.md` and `demo-session.spec.mjs` gain the new authoring paths (R17) |

Template checklist:

- [x] Runtime ownership remains within `main.go`/`app.go`, the relevant `internal/` packages, `frontend/overseer/src/`, and `sessions/` boundaries.
- [x] Cross-boundary Wails and protobuf contracts document producer, consumer, payload, validation, failure, ordering, and reconnect behavior (`contracts/private-desktop.md`).
- [x] Shared navigation, hacking, roster, and controller behavior remains server-authoritative and reconnect-safe; scene reset and live publication keep the route and hacking.
- [x] Wails method exposure, CSP, origin, and secret protections are preserved; authored text is never logged.
- [x] Session compatibility is defined: version 1 unchanged, no migration.
- [x] Runtime-only state (authoring revision, history, unapplied input) stays outside persistent JSON.
- [x] No new dependencies; one structural change (the presentation resolvers in `internal/live`) with a documented need (R10).
- [x] Verification uses the configured Go, Buf, bindings, Vite, Playwright, and package gates; unavailable checks will be reported.
- [x] Naming and style follow the affected files (`overseer.js` DOM-id constants and upper-case copy; Go table tests; Google Go style per `CLAUDE.md`).

**Branching exception**: as for features 036–038, this branch starts from
`038-facility-authoring-p0-fixes`, not `develop`. See Complexity Tracking.

**Post-design re-check (after Phase 1)**: still PASS. The design adds no persistent field and no
public surface. The only schema removal is the documented private migration.

## Project Structure

### Documentation (this feature)

```text
specs/039-unified-facility-authoring/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── private-desktop.md
│   └── overseer-ui.md
├── checklists/
│   └── requirements.md
└── tasks.md            # next step (/speckit-tasks)
```

### Source Code (repository root)

```text
proto/fallout/terminal/private/v1/desktop.proto   # new requests/results; removed superseded messages
proto/compatibility-baseline.binpb, proto/schema-revision.txt
internal/gen/fallout/terminal/private/v1/          # regenerated only
internal/live/
├── facility.go                 # per-target resolvers shared by projection and explanation
└── presentation.go             # new: explanation and preview differences
internal/session/
├── service.go                  # AuthoringRevision; remove Save, preserveCanonicalFacility, terminal-wide reset
└── facility.go                 # SaveAuthoring (replaces SaveFacilityAuthoring); ResetScene (replaces ResetFacility)
internal/control/
├── facility.go                 # SaveAuthoring, ResetScene, pending rebase, publication
└── service.go                  # remove ResetTerminalCommandStates
main.go                         # store adapters for SaveAuthoring and ResetScene
app.go, desktop_service.go, app_contract.go       # Wails methods, payloads, adapters
frontend/overseer/bindings/     # regenerated only
frontend/overseer/src/
├── index.html                  # #authoringBar, #sceneResetDialog, command lists, preview panes; removed draft UI
├── overseer.css                # bar, lists, badges, side-by-side preview
├── desktop-api.js              # saveAuthoring, resetScene, describePresentation; removed methods
└── overseer.js                 # save pipeline, history, unapplied input, live merge, lists, tree, reset, preview
tests/browser/
├── fixture-server/main.go      # shim and routes for new methods; real live functions for explanation/preview
├── facility-unified-save.spec.mjs        # new: US1
├── facility-command-lists.spec.mjs       # new: US2 (lists, tree, scene reset, player parity)
├── facility-preview-diff.spec.mjs        # new: US3
└── *.spec.mjs                  # update journeys using removed labels, #btnPublish, or draft buttons
scripts/wails-bindings-check.sh, scripts/wails-v3-contract-check.sh, scripts/state-changing-reset-native-smoke.sh
sessions/demo-capability-paths.md
```

**Structure Decision**: Server behavior extends the seams features 035–038 created:

- the session store owns durable merges and revisions;
- the coordinator owns ordering, pending requests, and publication;
- `internal/live` owns projection.

The explanation and diff live in `internal/live` because they must share projection code (R10). The
Overseer keeps its single-file structure (`overseer.js` has no module imports, and the fixture
rewrites script paths per route), as feature 038 decided. The new logic is grouped into clearly
named sections: save pipeline, history, unapplied input, presentation.

## Contract and State Design

### Persistent JSON

N/A. No field, default, or reference changes. `SaveAuthoring` writes the same document shape
through the existing encoder and serialized writer. Unknown fields are preserved by the existing
adapter.

### Wails Bridge and Runtime Events

See `contracts/private-desktop.md`:

- **Added**: `SaveAuthoring`, `ResetScene`, `DescribePresentation`.
- **Extended**: `PreviewFacility`, `FacilityOperationResult`, `SessionOperationResult`,
  `ReplaceTerminalGroupsResult`, `PlayerConfigOperationResult`.
- **Removed**: five methods and five messages, under a documented migration.

The `session-state` and coordination events are unchanged. The Overseer now merges only live values
from them (R6).

### Protobuf/ConnectRPC and HTTP Assets

The player contract is unchanged. Private additions are unary with explicit adapters; ordering
follows `control.Service.commit`.

### Runtime-State Lifecycle

- **Authoring revision**: created on open, incremented by authored mutations only, discarded on
  close (`data-model.md`).
- **Scene reset**: changes facility live values and command states in one durable write and one
  coordinator transition; it never touches broadcast routing or hacking.
- **Authoring saves**: refresh every broadcast terminal with `UpdateRuntime` (hacking and navigation
  kept), then reproject.
- **Pending device actions**: rebased by fingerprint or rejected.

### Platform, Tunnel, and Packaging

N/A beyond regenerated bindings and the normal Overseer build embedded by `task package`.

## Implementation Phases

### Phase 0: Research and Decisions — done

`research.md` R1–R17: the unified save, the authoring revision, the client pipeline with
«ПРИМЕНИТЬ СНОВА», the pending rebase, live publication, live-value merge, unapplied input, the
shared bar, undo and redo, server explanation, command lists and tree, scene reset, preview,
vocabulary and explained controls, protobuf migration, tests, and demo.

### Phase 1: Contracts and Data Design — done

`contracts/private-desktop.md`, `contracts/overseer-ui.md`, `data-model.md`, `quickstart.md`. The
Constitution Check was re-run and still passes.

### Phase 2: Foundations: server, contract, bridge, fixtures, and Overseer

All three stories share these files, so every story's implementation lands here, in story order
(`tasks.md`, Phase 2). US1 cannot ship on its own, because all three stories share the single-file
Overseer.

1. `desktop.proto` additions and removals; regenerate; refresh the baseline and digests; update the
   bindings allowlist (R15).
2. `internal/live`: extract the per-target resolvers from `applyFacilityProjection`; add the
   explanation and the preview diff; add parity tests over `sessions/` and the fixture sessions
   (R10, R13).
3. `internal/session`: `AuthoringRevision`; `SaveAuthoring` merge and validation; `ResetScene` with
   rollback; remove `Save`, `preserveCanonicalFacility`, `validateFacilityBindingsUnchanged`,
   `SaveFacilityAuthoring`, `ResetFacility`, and terminal-wide reset (R1, R2, R12).
4. `internal/control`: `SaveAuthoring` with publication and the pending rebase; `ResetScene` with
   pending handling and audit; remove the superseded paths (R4, R5, R12).
5. `main.go`, `app.go`, `desktop_service.go`, `app_contract.go`: methods, payloads, adapters,
   `recordOperation`, and removal of the old methods.
6. Fixture server, fake bindings, and `desktop-api.js` for the new and removed methods.
7. Overseer, one task at a time, in story order:
   - **US1**: `commitAuthoringEdit` with one request per edit; every terminal and facility commit
     as a `mutate`; the shared bar and status; undo, redo, and «ПРИМЕНИТЬ СНОВА»; the
     unapplied-input registry; the live-value merge; «ГОТОВО» vocabulary and explained controls;
     removal of the draft, the guard, and the publish button.
   - **US2**: `describePresentation` fetch and cache; player names and badges in the tree; the
     «ПРИ ОДОБРЕНИИ» and «ЧТО ВИДЯТ ИГРОКИ» lists with editors; the «СБРОСИТЬ СЦЕНУ» dialog.
   - **US3**: preview preselection, one-state notice, side-by-side panes, summary and tree marks,
     and the authoring-revision check after the queue drains.

### Phase 3: Story acceptance (each story verified on its own)

1. **US1 (P1)**: `facility-unified-save.spec.mjs`, plus the updates to the nine existing journeys
   that used the draft, publish, or old reset UI.
2. **US2 (P1)**: `facility-command-lists.spec.mjs`, including player parity on the real coordinator.
3. **US3 (P2)**: `facility-preview-diff.spec.mjs`.

### Phase 4: Integration, Documentation, Packaging

- Update `sessions/demo-capability-paths.md` and `demo-session.spec.mjs`.
- Run `go fix ./...` and review its edits, then the full gates (Verification Plan), then `task
  package`. Repeat `quickstart.md` in the packaged app on a demo copy. Record the results and any
  `NOT RUN` in `validation.md`.

## Verification Plan

| Surface | Automated check | Interactive/manual check | Expected result |
|---|---|---|---|
| Go domain/services | `task test`: SaveAuthoring merge, stale and accepted cases, live-value protection, pending rebase, publication; ResetScene atomicity and rollback; explanation and projection parity; preview differences; v1 round trip and projection identity for `sessions/` | N/A | All pass |
| Concurrent runtime | `task test:race`: SaveAuthoring concurrent with world actions and ResetScene | Quickstart step 2 with two players | No race; live never rejects an edit; nothing is half-reset |
| Contracts | `task proto:check`, `task proto:breaking` (refreshed baseline), `task bindings:check`, `app_contract_test.go` round trips | N/A | Additive descriptors; documented removals only; allowlist exact |
| Go quality | `go fix ./...` (reviewed), `task fmt:check`, `task vet`, `task lint` | N/A | Pass |
| Overseer frontend | `task frontend:build` | Quickstart steps 1–5 | Builds; journeys behave as specified |
| Browser journeys | `task browser:test` with the three new specs plus the full suite | N/A | All pass, including the vocabulary sweep and player parity |
| Package/release candidate | `task package` | Launch the packaged app, open a demo copy, repeat the quickstart | Launches; stories pass |
| Signed macOS distribution/public provider | N/A, not affected | N/A | — |

## Project-Specific Complexity Factors

- Every commit handler in `overseer.js` (terminal forms, facility dialogs, guide actions) must
  become a pure `mutate`. Missing one would bring back a second save path, so the vocabulary and
  save sweeps guard this.
- Queued saves and live events interleave. The Overseer must merge live values without
  touching queued authored content, and must order responses by revision.
- Projection and explanation must stay identical; the resolvers are shared and covered by parity
  tests.
- Publishing every edit at once means a broken or partial terminal edit reaches players.
  Server-side validation must reject it before the write, as `verifySessionContract` already does.
- Removing private messages touches pinned scripts and the compatibility baseline in one reviewed
  change.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| Development Workflow step 1 (branch from `develop`): this branch starts from `038-facility-authoring-p0-fixes`, which stacks on `035-facility-state-system`. Owner: the integrator of feature 035 (PR #50). Bounded duration: until 035 and 038 merge; then 039 is rebased onto `develop` before its own integration. | The two save models, the draft guard, and the preview being unified exist only in 035–038 code, which `develop` does not contain yet. | Waiting for the merges blocks the P1 review findings; committing them to 038 would mix a model redesign into a P0 fix branch. |
