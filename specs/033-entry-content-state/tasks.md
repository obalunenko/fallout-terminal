# Tasks: Command-Driven Entry Content State

**Input**: Design artifacts in `specs/033-entry-content-state/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, and `contracts/`
**Tests**: Required by the specification and constitution; each story's tests are written before its implementation.
**Organization**: Tasks are grouped by user story and ordered in dependency waves. `[P]` means tasks in that wave touch different files and have no incomplete dependency.

## Phase 1: Setup

Prepare shared compatibility and demonstration data used across the story phases.

**Wave 1 — independent (different files):**

- [x] **T001** [P] Extend the version-1 state-changing session fixture with legacy entries, explicit ordered blocks, empty and identical block text, five distinct block targets, and frozen outcomes · `internal/testutil/testdata/session-v1-state-changing.json`
- [x] **T002** [P] Add an authored demonstration entry whose independent blocks are targeted by different commands while preserving existing demo behavior · `sessions/demo.json`

---

## Phase 2: Foundational — Contracts, Domain Model, and Validation

These shared schema and domain changes block every user story.

### Tests

**Wave 1 — independent failing tests (different files):**

- [ ] **T003** [P] Add table-driven domain tests for block clone/JSON round trips, terminal-scoped identity, legacy exclusivity, missing or cross-terminal targets, duplicate ownership, and frozen-target validity · `internal/domain/model_test.go`, `internal/domain/validate_test.go`
- [ ] **T004** [P] Add failing descriptor and adapter tests for additive field numbers, explicit nested presence, empty completed text, unknown-field preservation, and the exact `EntryContent` identifier · `app_contract_test.go`, `internal/session/contract_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2:**

- [ ] **T005** Add `EntryContentBlock` and `EntryContentChange`, extend `StateChangeConfig`, `CommandExecutionState`, and `EntryContent` additively, then regenerate the governed Go contract and schema revision · `proto/fallout/terminal/persistence/v1/session.proto`, `internal/gen/fallout/terminal/persistence/v1/session.pb.go`, `proto/schema-revision.txt`

**⟶ Wait for Wave 2 to finish, then Wave 3:**

- [ ] **T006** Add ordered block and optional entry-change domain values, JSON fields, and deep-clone ownership without altering legacy description behavior · `internal/domain/model.go`, `internal/domain/json.go`

**⟶ Wait for Wave 3 to finish, then Wave 4 — independent (different files):**

- [ ] **T007** [P] Enforce terminal-wide block identity, entry representation exclusivity, target resolution, one-command ownership, frozen-state consistency, and body bounds with actionable errors · `internal/domain/validate.go`
- [ ] **T008** [P] Map authored blocks and explicitly present configured/frozen changes through the version-1 persistence adapter while preserving unknown JSON fields · `internal/session/contract.go`

**Checkpoint**: Additive contracts, portable JSON, domain cloning, and authoritative validation support all later stories.

---

## Phase 3: User Story 1 — Change Visible Entry Content Through a Command (P1)

**Goal**: One approved command atomically completes its own presentation and exactly one target block, while players receive one authoritative effective entry update.

**Independent Test**: Execute a fixture command against one block in a two-block entry and verify the command and target change together while the untargeted block and navigation remain unchanged.

### Tests

**Wave 1 — independent failing tests (different files):**

- [ ] **T009** [P] [US1] Add execution tests for frozen block capture, empty completed text, rejection, save rollback, repeat idempotence, malformed store results, and durability-before-publication · `internal/session/service_test.go`, `internal/control/service_test.go`
- [ ] **T010** [P] [US1] Add effective-tree tests for independent frozen block application, ordered two-newline composition, detached projections, and unchanged legacy descriptions · `internal/live/service_test.go`
- [ ] **T011** [P] [US1] Add approval and synchronization journeys covering five commands in varied orders, rejection/failure atomicity, an already-open entry, observer convergence, and reconnect · `tests/browser/state-changing-command-approval.spec.mjs`, `tests/browser/state-changing-command-sync.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2:**

- [ ] **T012** [US1] Freeze the validated entry block ID and completed text inside the existing command snapshot transaction with rollback and no-op repeat semantics · `internal/session/service.go`

**⟶ Wait for Wave 2 to finish, then Wave 3 — independent (different files):**

- [ ] **T013** [P] [US1] Apply frozen block outcomes to detached trees and compose effective entry descriptions server-side without exposing authoring alternatives · `internal/live/service.go`
- [ ] **T014** [P] [US1] Install and publish the canonical combined command/block result only after durable execution succeeds, preserving revision and navigation ordering · `internal/control/service.go`
- [ ] **T015** [P] [US1] Extend browser fixtures with canonical block configurations, atomic execution outcomes, failure controls, and effective player projections · `tests/browser/fixture-server/main.go`, `tests/browser/fixtures/desktop-bindings.js`

**Checkpoint**: User Story 1 is independently functional and testable through fixture-configured commands without the authoring UI.

---

## Phase 4: User Story 2 — Author Independent Entry Blocks and Command Targets (P1)

**Goal**: The Overseer can author stable ordered blocks and one-block command targets, see the relationship from both editors, and cannot create conflicting ownership.

**Independent Test**: Author and reorder two blocks, target each from a different command, reopen the session, and verify exact command-side entry/position/preview labels plus entry-side targeting-command names.

### Tests

**Wave 1 — failing browser contract:**

- [ ] **T016** [US2] Add authoring journeys for block add/edit/reorder/delete/reopen, stable IDs, legacy conversion, target conflicts, deletion guards, completed-target locking, 48-code-point previews, `ПУСТО`, and `[data-entry-block-owner]` command names · `tests/browser/state-changing-command-authoring.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2:**

- [ ] **T017** [US2] Implement the ordered block editor, terminal-local target selector, completed block text, derived entry/position/preview labels, reverse command-owner labels, conflict/deletion feedback, completed locks, and compact accessible styling · `frontend/overseer/src/overseer.js`, `frontend/overseer/src/overseer.css`

**Checkpoint**: User Story 2 is independently functional and testable; both directions identify the relationship without displaying internal IDs or persisting a back-reference.

---

## Phase 5: User Story 3 — Reset Command and Entry States Safely (P1)

**Goal**: Individual and terminal-wide command resets restore their owned blocks through the same durable, atomic reset path and preserve unrelated state.

**Independent Test**: Complete two commands against different blocks, reset one, verify the other remains completed, then reset the terminal and verify every command-owned block returns to its initial text.

### Tests

**Wave 1 — independent failing tests (different files):**

- [ ] **T018** [P] [US3] Add Go tests for individual and terminal-wide reset atomicity, unrelated-state preservation, cancellation/failure behavior, canonical revisions, command deletion pruning, and publication ordering · `internal/session/service_test.go`, `internal/control/service_test.go`, `app_test.go`
- [ ] **T019** [P] [US3] Add Overseer reset journeys proving confirmation/cancellation, backend-failure preservation, owner-label restoration, and exact individual/all reset effects · `tests/browser/state-changing-command-authoring.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2:**

- [ ] **T020** [US3] Make individual reset, terminal reset, and owning-command deletion remove complete command snapshots so owned blocks return to authored initial text in one durable revision · `internal/session/service.go`

**⟶ Wait for Wave 2 to finish, then Wave 3:**

- [ ] **T021** [US3] Route existing trusted reset methods through coordinator transactions, install canonical store results, and publish once after durability without adding a desktop method or event · `internal/control/service.go`, `app.go`

**Checkpoint**: User Story 3 is independently functional and testable for individual, terminal-wide, canceled, and failed resets.

---

## Phase 6: User Story 4 — Preserve State Across Sessions and Live Updates (P2)

**Goal**: Authored and completed block state survives session and broadcast lifecycles while legacy version-1 entries remain byte-for-byte equivalent in player-visible text.

**Independent Test**: Complete two block-changing commands, restart broadcast and application/session state, verify both results, then reset and reopen to verify initial text and legacy compatibility.

### Tests

**Wave 1 — independent failing tests (different files):**

- [ ] **T022** [P] [US4] Add persistence tests for legacy round trips, explicit blocks, empty presence, unknown extras, stale full-document saves, target-preserving moves/renames, command deletion, and reopen recovery · `internal/session/contract_test.go`, `internal/session/service_test.go`
- [ ] **T023** [P] [US4] Add lifecycle journeys for broadcast stop/start, terminal switching, application/session reopen, reconnect, monotonic revisions, and retained valid frozen outcomes after publication · `tests/browser/state-changing-command-sync.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2:**

- [ ] **T024** [US4] Preserve frozen block snapshots during valid stale-save merges and authored publication, reject invalid retarget/removal, and retain legacy description and unknown-field compatibility across reopen · `internal/session/service.go`, `internal/session/contract.go`

**Checkpoint**: User Story 4 is independently functional and testable across persistence, publication, switching, restart, and reconnect lifecycles.

---

## Phase 7: Polish and Cross-Cutting Validation

**Wave 1 — single validation owner:**

- [ ] **T025** Review changed Go with the repository Go quality and modern-Go guidance; run `go fix ./...` and retain only intentional edits; run formatting, protobuf generation/drift/breaking checks, bindings checks, frontend builds, `task vet`, `task lint`, `task test`, `task test:race`, and affected browser suites; verify SC-001 through SC-008 and that the public player contract is unchanged · `Taskfile.yml`

## Dependencies & Execution Order

### Phase dependencies

- Phase 1 (Setup) starts immediately.
- Phase 2 (Foundational) depends on Phase 1 and blocks every user story.
- Phase 3 (US1) and Phase 4 (US2) both depend on Phase 2; US1 is the runtime MVP and US2 is its independently testable authoring surface.
- Phase 5 (US3) depends on US1's durable execution path and US2's relationship presentation.
- Phase 6 (US4) depends on the execution, authoring, and reset behaviors from US1–US3.
- Phase 7 (Polish) depends on all story checkpoints.

### Wave order

- Setup: Wave 1 joins before Foundational work.
- Foundational: failing tests → schema/generation → domain values → validation and adapters.
- US1: failing service/live/browser tests → durable snapshot execution → live/control/fixture integration.
- US2: failing authoring journey → Overseer JavaScript and CSS implementation.
- US3: failing Go/browser reset tests → session reset semantics → coordinator/application delegation.
- US4: failing persistence/browser lifecycle tests → compatibility and stale-save implementation.
- Polish: one repository-wide validation wave after every prior task completes.

### Parallel opportunities

- Tasks marked `[P]` within a wave may be assigned independently because their file sets do not overlap.
- After Foundational completes, US1 runtime work and US2 authoring work can proceed independently if their own wave order is preserved; join both before US3.
- MVP scope is Phases 1–3; it proves atomic command-driven player-visible block changes before the authoring, reset, and lifecycle increments are added.
