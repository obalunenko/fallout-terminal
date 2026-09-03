# Implementation Plan: Command-Driven Entry Content State

## Summary

Extend session version 1 so an entry may contain ordered, stable text blocks and a state-changing command may own one optional block transition. Freeze that transition inside the command's existing durable execution snapshot, compose effective entry text on the server, and publish it through the unchanged player entry-description contract. In the Overseer editor, support the relationship from both directions: commands retain their target selector, while every EntryContent block opens a dialog that can assign an eligible existing command, create a fully configured command in a chosen folder, atomically reassign an uncompleted owner, or remove an uncompleted assignment. Keep the command configuration as the only canonical authored owner and reuse the existing whole-session save path, so this dialog increment requires no schema, Go service, desktop method, or player-contract change.

## Project Structure

```text
proto/fallout/terminal/persistence/v1/session.proto   # additive block, target, and frozen-state fields
proto/schema-revision.txt                            # regenerated contract revision
internal/gen/fallout/terminal/persistence/v1/
└── session.pb.go                                    # generated only

internal/domain/
├── model.go                                         # block and entry-content-change value types
├── json.go                                          # known version-1 JSON block field
├── validate.go                                      # terminal-wide identity/reference/ownership rules
└── {model,validate}_test.go                          # round-trip, clone, legacy, and invalid-shape coverage
internal/session/
├── contract.go                                      # explicit domain ↔ persistence protobuf mapping
├── service.go                                       # frozen snapshot capture and stale-save protection
└── {contract,service}_test.go                        # atomic execute/reset/reopen/failure coverage
internal/live/
├── service.go                                       # effective block application and entry composition
└── service_test.go                                  # independent blocks and detached projection coverage
internal/control/
├── service.go                                       # durability-gated execute/reset publication
└── service_test.go                                  # ordering, malformed store, and concurrency coverage

app.go                                               # trusted reset delegation and canonical session event
app_test.go                                          # application-level reset/publication ordering
app_contract_test.go                                 # imported persistence descriptor/bridge round trip

frontend/overseer/src/
├── index.html                                       # accessible block command-assignment dialog
├── overseer.js                                      # block editor, dialog draft/apply flow, bidirectional labels
└── overseer.css                                     # compact block, dialog, target, and owner-reference controls

tests/browser/
├── fixtures/desktop-bindings.js                     # authoritative session/block fixture behavior
├── fixture-server/main.go                           # execution/reset audit and effective state fixture
├── state-changing-command-authoring.spec.mjs       # authoring, conflicts, deletion, and reset journeys
├── state-changing-command-approval.spec.mjs        # reject/failure/repeat atomicity journeys
└── state-changing-command-sync.spec.mjs            # live open-entry, ordering, reconnect, and lifecycle journeys

internal/testutil/testdata/session-v1-state-changing.json          # version-1 block-state compatibility fixture
sessions/demo.json                                                 # authored demonstration of independent block changes
```

**Structure Decision**: Keep authored and durable state in the existing domain/session ownership boundary, keep coordination and effective projection in control/live, expose no new player or desktop methods, and implement the new reverse-authoring workflow entirely in the Overseer's existing static dialog, editor module, styles, and browser contract tests.

## Constitution Check

| Principle | Before research | After design | Assessment |
|---|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | PASS | Domain, session, live, and control remain Wails-independent; the Overseer dialog continues through the narrow registered desktop service and existing save method. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | PASS | Persisted structures remain defined in `session.proto`; the dialog creates the existing session shape and introduces no handwritten transport DTO. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | PASS | Approval and reset commit in Go before the existing server stream publishes one authoritative effective tree; browsers do not perform optimistic state mutation. |
| IV. Separate Public and Private Capabilities | PASS | PASS | Players receive only flattened effective entry text. The block assignment dialog, authored targets, alternate content, session state, approval, and reset remain private. |
| V. Evolve Schemas Safely and Reproducibly | PASS | PASS | Existing field numbers remain stable; the dialog increment adds no fields, while pinned generation/drift/breaking checks continue to protect the established schema. |
| VI. Preserve Portable Session JSON Version 1 | PASS | PASS | Version remains 1, legacy `description` remains readable and unchanged until an explicit block edit, and unknown-field preservation remains in the explicit adapter path. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | PASS | No parallel relationship, state map, public protocol, or desktop method is introduced; both editor directions update the command-owned target. |

No constitutional violation or Complexity Tracking entry is required. Implementation must begin on a dedicated feature branch before source changes. Go work follows the repository's Google Go style review, immediate `t.Cleanup` ownership, `go fix ./...` before final formatting, and macOS Taskfile validation rules.

## Phase 0: Research

[research.md](./research.md) records the decisions for command-owned block snapshots, terminal-scoped block identity, legacy conversion, server-side composition, coordinator-owned reset publication, completed-target locking, derived bidirectional authoring labels, and a draft-isolated block assignment dialog that commits through one existing session save.

## Phase 1: Contracts and Data Model

- [data-model.md](./data-model.md) defines authored blocks, optional command targets, frozen snapshots, the transient dialog draft, validation, effective composition, and state transitions.
- [contracts/session-v1.md](./contracts/session-v1.md) defines additive persistence fields and JSON compatibility.
- [contracts/public-player.md](./contracts/public-player.md) defines the unchanged player message surface and authoritative update semantics.
- [contracts/private-overseer.md](./contracts/private-overseer.md) defines unchanged desktop methods plus the block-authoring UI contract.

## Implementation Strategy

### 1. Additive contracts and pure domain rules

Add `EntryContentBlock` and explicitly present `EntryContentChange` messages to the persistence schema without renumbering existing fields. Represent ordered blocks and optional changes in domain values, update deep clones and explicit protobuf adapters, and validate terminal-wide block identity, target ownership, frozen-state consistency, body limits, and legacy exclusivity. Regenerate only with the pinned protobuf workflow and retain the established compatibility baseline for breaking-change comparison.

### 2. Atomic persistence and stale-save protection

Extend `ExecuteCommandState` to freeze the configured block ID and completed text in the same `commandStates[commandID]` value as the completed command name/result. Reuse the current serialized document mutation, revision allocation, atomic replace, rollback, idempotent repeat, and canonical stale-save merge. Reject full authoring saves that remove, retarget, or duplicate a block still owned by a completed command; deleting the owning command prunes the complete snapshot and restores the block's initial text.

### 3. Server-authoritative projection and resets

Extend effective-tree projection to apply every frozen block snapshot independently and compose each explicit entry as ordered block text separated by `\n\n`; legacy entries continue to use `description` exactly. Route individual and terminal-wide resets through new coordinator methods that call the existing `CommandStateStore` reset operations under the one-way control-to-session order, install the returned canonical state, clear affected command presentation, retain open-entry navigation, and publish only after durable success.

### 4. Overseer authoring and existing player rendering

Replace the single entry textarea with an ordered block editor that preserves IDs during text edits and moves, generates IDs only for new blocks, and converts a legacy description only on the first accepted block edit. Add an optional current-terminal block target and completed block text to state-change command editing. Build each target label from the containing entry name, the block's current one-based position, and a normalized authored-text preview with an explicit empty marker. Build a reverse ownership index from command configurations during form rendering so each targeted block shows the targeting command's current displayed name without persisting a second relationship.

Give every block a command-assignment action backed by one native modal dialog. Populate existing-command choices from same-terminal state-changing commands that own no other block, populate creation destinations from the terminal's folders, and hold all selections and new-command fields in a detached dialog draft. On apply, validate the complete draft before mutating the session: assigning updates the selected command; reassigning an uncompleted block clears its old owner's nested entry change and assigns the new owner in the same synchronous candidate; removal clears only the old command's nested entry change; creation allocates one stable command ID, builds its complete state-change configuration, and appends it to the selected folder. Call the existing autosave exactly once after the candidate is valid, then rebuild the tree and both relationship views. Cancel and close discard the draft without mutation or save, while a completed owner exposes its effective name but disables assignment changes until reset. Keep the player client and player protobuf unchanged; verify that the existing live-tree replacement and pagination logic redraw an already-open entry.

### 5. Fixtures, compatibility, and regression proof

Extend one version-1 fixture and the demo with commands targeting separate blocks. Prove five distinct commands can complete against five blocks in varied orders, along with individual/all resets, repeat idempotence, rejection/cancel/save failure, command deletion, blocked target deletion, reconnect, broadcast/terminal/application lifecycle, stale saves, malformed store results, and monotonic stream revisions. Existing state-changing commands and legacy single-description entries remain valid and keep their current presentation.

## Verification Plan

| Surface | Verification | Required evidence |
|---|---|---|
| Persistence schema | `task proto:generate`, `task proto:check`, `task proto:breaking` | Additive field numbers, deterministic generated output, unchanged compatibility baseline, generated Go compilation. |
| Domain and session | Focused `go test` for `./internal/domain` and `./internal/session` | Legacy round trip, exact block order, empty content, unknown extras, conflicts, atomic execute/reset, failure rollback, stale save, and reopen. |
| Runtime coordination | Focused `go test -race` for `./internal/control`, `./internal/live`, and `./internal/player` | No publication before durability, no partial fake-store install, one revisioned update, independent block composition, reconnect convergence. |
| Trusted application boundary | Focused root package tests and `task bindings:check` | Existing reset payloads/method count remain unchanged; session-state carries the canonical new fields; no public capability appears. |
| Overseer authoring | Focused Playwright authoring journey | Add/edit/reorder/reopen, terminal-only targeting, command-side entry/position/preview labels, entry-side command-name ownership labels, existing-command assignment, new-command creation into a selected folder, atomic reassignment/removal, dialog cancellation, identical/empty preview handling, conflict feedback, completed-owner lock, deletion guard, and reset confirmation. |
| Player synchronization | Focused Playwright approval/sync journeys | Controller and observers see an already-open entry update within the existing one-second bound, preserve navigation, converge after reconnect, and never see partial state. |
| Frontend builds | After `nvm use`, `task frontend:build` | Overseer production build succeeds and the unchanged player client still compiles against its generated contract. |
| Repository Go quality | `go fix ./...`, review diff, `gofmt -l .`, `task vet`, `task lint`, `task test`, `task test:race` | Intentional modernization only; format, static analysis, unit/integration tests, and race detector pass with macOS deployment flags. |
| Full local gates | `task check`, then `task browser:test` when Chromium is available | Governed protobuf, binding, Go, Companion, and browser journeys pass; unavailable interactive evidence is reported honestly. |

## Risks and Controls

| Risk | Control |
|---|---|
| A second entry-state map drifts from command state | Store the frozen block transition only inside the existing command snapshot. |
| A stale browser save removes or retargets a completed effect | Merge canonical snapshots by command ID and reject changes that invalidate a frozen target until reset or command deletion. |
| Two commands overwrite one block | Enforce terminal-wide block identity and one authored/frozen owner per block in backend validation and mirror the check in the Overseer. |
| Empty completed text is mistaken for no target | Use explicit nested message/pointer presence; text itself is never the presence signal. |
| Legacy sessions are silently rewritten | Preserve `description` on load/save and convert only after an explicit accepted block edit. |
| Player contracts expose authored alternate content | Flatten only effective text into the existing public description; keep block IDs and alternate text private. |
| Display labels drift into a second source of relationship truth | Derive command options and block-owner labels from the canonical command target on every editor render; persist no block-to-command back-reference. |
| The block dialog partially mutates two commands during reassignment | Keep edits in a detached draft, validate the complete candidate, update both command configurations synchronously, and invoke the existing whole-session save once. |
| A newly created command appears in an unexpected location | Require a same-terminal destination folder and append the complete new command only to that selected folder. |
| Closing the dialog leaks draft edits into the session | Do not mutate the tree until Apply; Cancel, Escape, and close discard the draft and perform no save. |
| Reset is durable before runtime but published late or inconsistently | Move reset orchestration into the coordinator transaction and publish only the canonical store result. |
| An open entry loses navigation or stale pagination | Preserve `ViewEntryID`, publish a full effective tree, and exercise page preservation/clamping in browser tests. |
| Generated code is edited or drifts | Change schemas first and use pinned Taskfile generation, drift, lint, and breaking gates. |
| Existing unrelated working-tree edits are overwritten | Limit implementation to listed feature files and review every generated or modernization diff before retaining it. |
