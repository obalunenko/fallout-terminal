# Tasks: Terminal Grouping

## Phase 1: Setup

No setup-only changes are required. The feature uses the repository's existing pinned protobuf, Wails binding, Go, frontend, and Playwright tooling.

## Phase 2: Foundational Contracts and Types

**Wave 1 — independent schema sources:**

- [x] **T001** [P] Add `TerminalGroup` and additive `Session.terminal_groups = 5` persistence fields without changing existing numbers · `proto/fallout/terminal/persistence/v1/session.proto`
- [x] **T002** [P] Add revisioned `ReplaceTerminalGroupsRequest` and `ReplaceTerminalGroupsResult` private contracts without exposing group management publicly · `proto/fallout/terminal/private/v1/desktop.proto`

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T003** Regenerate pinned Go and ECMAScript protobuf outputs from both updated schemas · `internal/gen/`, `frontend/client/gen/`

**⟶ Wait for T003 to finish, then:**

- [x] **T004** Add durable terminal-group, membership snapshot, group candidate, and runtime route-provenance types with deep cloning · `internal/domain/model.go`

**⟶ Wait for T004 to finish, then Wave 4 — independent adapters:**

- [x] **T005** [P] Recognize `terminalGroups` as a known JSON-v1 field while preserving legacy presence and unknown-field behavior · `internal/domain/json.go`
- [x] **T006** [P] Map terminal groups through the persistence protobuf adapter without losing order or portable JSON extras · `internal/session/contract.go`

**Checkpoint**: Generated contracts and transport-independent group types are ready for story implementation.

---

## Phase 3: User Story 1 — Organize Every Terminal Through a Group (P1)

**Goal**: Every terminal is presented beneath exactly one persistent high-level group, with singleton normalization for standalone, new, imported, and legacy terminals.

**Independent Test**: Open legacy terminals as singleton groups, create and delete terminals, save/reopen ordered groups, and prove exact-one membership with no empty groups.

### Tests

**Wave 1 — independent failing coverage:**

- [x] **T007** [P] [US1] Add JSON round-trip, legacy normalization, group-order, and deep-clone tests · `internal/domain/model_test.go`
- [x] **T008** [P] [US1] Add table-driven exact-one, non-empty, unique-name, duplicate-member, missing-member, and deterministic-order validation tests · `internal/domain/validate_test.go`
- [x] **T009** [P] [US1] Add protobuf descriptor and populated/legacy group round-trip tests with protobuf-aware comparisons · `internal/session/contract_test.go`
- [x] **T010** [P] [US1] Add session tests for singleton normalization, save/reopen, terminal create/import/delete, and canonical membership preservation · `internal/session/service_test.go`
- [x] **T011** [P] [US1] Add browser journeys for high-level group rendering, singleton terminals, ordering, and persistence · `tests/browser/terminal-grouping.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent core/UI boundaries:**

- [x] **T012** [P] [US1] Implement compatible and canonical group validation, normalized-name checks, exact-one indexing, and deterministic singleton normalization · `internal/domain/validate.go`
- [x] **T013** [P] [US1] Normalize legacy/new/deleted terminals, preserve canonical groups on generic saves, and expose detached ordered group snapshots · `internal/session/service.go`
- [x] **T014** [P] [US1] Normalize and deeply detach terminal groups in session results and events · `frontend/overseer/src/desktop-api.js`
- [x] **T015** [P] [US1] Extend the browser fixture with canonical, singleton, ordered, and legacy session data · `tests/browser/fixture-server/main.go`

**⟶ Wait for Wave 2 to finish, then Wave 3 — independent presentation structure:**

- [ ] **T016** [P] [US1] Replace the flat terminal-list container with accessible high-level group markup and terminal members · `frontend/overseer/src/index.html`
- [ ] **T017** [P] [US1] Style group hierarchy, ordered members, singleton states, selection, and responsive layout · `frontend/overseer/src/overseer.css`

**⟶ Wait for Wave 3 to finish, then:**

- [ ] **T018** [US1] Render canonical groups as the top level and keep terminal create/import/delete local state aligned with atomic singleton rules · `frontend/overseer/src/overseer.js`

**Checkpoint**: US1 independently loads, displays, creates, deletes, saves, and reopens terminals with exact-one high-level group representation.

---

## Phase 4: User Story 2 — Manage Groups Safely (P1)

**Goal**: The Overseer can create, rename, dissolve, reorder, merge/split, and move terminals with structured confirmation for destructive changes and atomic stale-safe application.

**Independent Test**: Exercise every CRUD/move flow, cancel and close confirmations, submit stale/repeated confirmations, and verify canonical state changes at most once without terminal-content loss.

### Tests

**Wave 1 — independent failing coverage:**

- [ ] **T019** [P] [US2] Add expected-session-revision compare-and-replace, dissolution, move, no-op, persistence-failure, and duplicate-submit tests · `internal/session/service_test.go`
- [ ] **T020** [P] [US2] Add expected-coordination-revision, pending-decision, active-route, seeded-order, stale, and atomic rejection tests · `internal/control/service_test.go`
- [ ] **T021** [P] [US2] Add application orchestration tests for canonical result/event ordering and stale failure projections · `app_test.go`
- [ ] **T022** [P] [US2] Add private protobuf descriptor/adapter tests and public descriptor leak assertions · `app_contract_test.go`
- [ ] **T023** [P] [US2] Add desktop method routing tests for the trusted group replacement capability · `desktop_service_test.go`
- [ ] **T024** [P] [US2] Add desktop API normalization tests for both revisions, canonical session, coordination state, and errors · `tests/browser/desktop-api.spec.mjs`
- [ ] **T025** [P] [US2] Add browser journeys for create, rename, dissolve, move, reorder, impact contents, cancel/close, stale, retry, and singleton-delete rejection · `tests/browser/terminal-grouping.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [ ] **T026** [US2] Implement complete candidate diffing, strict authored-link validation, content-preserving dissolution, singleton-delete rejection, and actionable affected-item errors · `internal/domain/validate.go`

**⟶ Wait for T026 to finish, then Wave 3 — independent state owners:**

- [ ] **T027** [P] [US2] Implement synchronous expected-session-revision group compare-and-replace with atomic durability and detached canonical results · `internal/session/service.go`
- [ ] **T028** [P] [US2] Add the group-store seam and coordinator-locked expected-runtime-revision, pending, active-route, and seeded-order guard · `internal/control/service.go`

**⟶ Wait for Wave 3 to finish, then:**

- [ ] **T029** [US2] Compose the control-to-session group store adapter without reversing lock or package ownership · `main.go`

**⟶ Wait for T029 to finish, then:**

- [ ] **T030** [US2] Route the trusted mutation through the application, advance both authoritative revisions after durability, and publish canonical results/events · `app.go`

**⟶ Wait for T030 to finish, then:**

- [ ] **T031** [US2] Map the revisioned request/result through generated private protobuf types · `app_contract.go`

**⟶ Wait for T031 to finish, then:**

- [ ] **T032** [US2] Expose only the narrow terminal-group replacement method on the registered desktop service · `desktop_service.go`

**⟶ Wait for T032 to finish, then:**

- [ ] **T033** [US2] Regenerate Wails bindings for the new private desktop method without hand-editing generated output · `frontend/overseer/bindings/`

**⟶ Wait for T033 to finish, then Wave 9 — independent UI surfaces:**

- [ ] **T034** [P] [US2] Add accessible group CRUD/move controls and the destructive impact confirmation dialog · `frontend/overseer/src/index.html`
- [ ] **T035** [P] [US2] Style group editing, validation, destructive impact, busy, stale, and focus states · `frontend/overseer/src/overseer.css`
- [ ] **T036** [P] [US2] Add the revisioned group command and canonical result normalization to the private desktop API facade · `frontend/overseer/src/desktop-api.js`

**⟶ Wait for Wave 9 to finish, then:**

- [ ] **T037** [US2] Implement create/rename/dissolve/move/reorder drafts, impact diffing, cancel-zero-call behavior, duplicate-submit guard, stale refresh, and canonical replacement · `frontend/overseer/src/overseer.js`

**Checkpoint**: US2 independently provides safe Overseer group management with explicit destructive confirmation and single atomic application.

---

## Phase 5: User Story 3 — Move Forward Only Within a Group (P1)

**Goal**: Only an assigned controller's valid same-group authored transition reaches the existing single Overseer approval flow.

**Independent Test**: Author A→B inside one group and A→C across groups, then verify only A→B can be requested and approved while stale/rejected requests have zero effect.

### Tests

**Wave 1 — independent failing coverage:**

- [ ] **T038** [P] [US3] Add catalog tests for same-group, cross-group, self, missing, stale-link, and detached transition lookups · `internal/session/service_test.go`
- [ ] **T039** [P] [US3] Add forward request/approve/reject/close, authority, pending, stale-membership, and exact-one-route-point tests · `internal/control/service_test.go`
- [ ] **T040** [P] [US3] Add controller/observer browser journeys for same-group forward approval and cross-group zero-effect attempts · `tests/browser/terminal-navigation.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [ ] **T041** [US3] Make transition lookup return only detached links whose endpoints currently share one canonical group · `internal/session/service.go`

**⟶ Wait for T041 to finish, then Wave 3 — independent consumers:**

- [ ] **T042** [P] [US3] Enforce same-group forward eligibility and approval-time link/group revalidation without changing existing approval cardinality · `internal/control/service.go`
- [ ] **T043** [P] [US3] Filter terminal-transition destination choices to other members of the edited terminal's current group · `frontend/overseer/src/overseer.js`

**Checkpoint**: US3 independently constrains forward navigation to one group while preserving Overseer approval and zero-effect rejection.

---

## Phase 6: User Story 4 — Return Only Within the Same Group (P1)

**Goal**: Backward navigation remains same-group and approved, and a fresh broadcast started in the middle can traverse the complete ordered group in both directions.

**Independent Test**: Start A/B/C/D at C and complete C→B→A→B→C→D, then invalidate a return membership/order and prove approval changes nothing.

### Tests

**Wave 1 — independent failing coverage:**

- [ ] **T044** [P] [US4] Add runtime clone/provenance tests for authored and initial-prefix return points · `internal/domain/model_test.go`
- [ ] **T045** [P] [US4] Add first/middle/last start, seeded LIFO, approve/reject/close, stale-order, cross-group return, and manual-activation cleanup tests · `internal/control/service_test.go`
- [ ] **T046** [P] [US4] Add the complete C→B→A→B→C→D browser journey with reconnect and no skipped/duplicated activations · `tests/browser/terminal-grouping.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [ ] **T047** [US4] Add runtime-only return-point origin/group-position provenance and fresh-broadcast initialization state cloning · `internal/domain/model.go`

**⟶ Wait for T047 to finish, then:**

- [ ] **T048** [US4] Seed preceding members once, enforce same-group LIFO returns, revalidate provenance/order at approval, and preserve later direct-activation cleanup · `internal/control/service.go`

**Checkpoint**: US4 independently provides approved full-group backward/forward traversal from any starting member.

---

## Phase 7: User Story 5 — Preserve Safe Sessions and Active Play (P2)

**Goal**: Legacy content, active broadcasts, reconnect behavior, direct activation, and unrelated command behavior remain safe and compatible.

**Independent Test**: Open a no-group legacy session, exercise dormant old links and conflicting edits, reconnect multiple players, and run ordinary/state-changing/manual-activation regressions without content loss.

### Tests

**Wave 1 — independent failing/regression coverage:**

- [ ] **T049** [P] [US5] Add malformed partial-group, legacy cross-link, unknown-extra preservation, and compatibility validation cases · `internal/domain/validate_test.go`
- [ ] **T050** [P] [US5] Add generic-save stale-group protection, legacy dormant-link, terminal lifecycle, rollback, and revision-coalescing regressions · `internal/session/service_test.go`
- [ ] **T051** [P] [US5] Add legacy normalization, conflicting edit, reconnect, ordinary/state-changing command, and direct activation browser regressions · `tests/browser/terminal-grouping.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent compatibility fixtures/docs:**

- [ ] **T052** [P] [US5] Complete grouped, legacy, pending, route, reconnect, and ordinary-command fixture states · `tests/browser/fixture-server/main.go`
- [ ] **T053** [P] [US5] Add an explicit ordered group for the demo's authored transition without changing unrelated content · `sessions/demo.json`
- [ ] **T054** [P] [US5] Document group management, singleton compatibility, confirmation, middle-start navigation, and approval behavior · `README.md`

**Checkpoint**: US5 independently proves compatibility and active-play safety across legacy, regression, and reconnect journeys.

---

## Phase 8: Polish and Cross-Cutting Verification

**Wave 1 — review and simplify the completed implementation:**

- [ ] **T055** Apply code-simplification and Go-quality review findings without changing specified behavior · `internal/domain/`, `internal/session/`, `internal/control/`, `app.go`, `app_contract.go`, `desktop_service.go`, `frontend/overseer/src/`

**⟶ Wait for T055 to finish, then:**

- [ ] **T056** Run protobuf format/lint/generation/breaking checks and Wails binding drift checks, resolving only feature-owned drift · `proto/`, `internal/gen/`, `frontend/client/gen/`, `frontend/overseer/bindings/`

**⟶ Wait for T056 to finish, then:**

- [ ] **T057** Run Go formatting, vet, unit tests, and race tests for domain, session, control, application, and private-boundary behavior · `./`

**⟶ Wait for T057 to finish, then:**

- [ ] **T058** Run clean Overseer/client builds and all browser acceptance suites covering SC-001 through SC-014 · `frontend/`, `tests/browser/`

**⟶ Wait for T058 to finish, then:**

- [ ] **T059** Run the owned application build and available package smoke, then audit every success criterion and report unavailable conditional checks honestly · `build/`, `specs/020-terminal-grouping/spec.md`

## Dependencies & Execution Order

- Phase 2 contracts/types block every user story.
- US1 establishes canonical exact-one groups and blocks group mutation and navigation stories.
- US2 establishes guarded group CRUD and blocks final concurrent-navigation safety evidence.
- US3 and US4 consume the canonical catalog and can be delivered sequentially as forward then backward navigation slices.
- US5 runs after the navigation slices so compatibility and reconnect regressions cover the complete behavior.
- Polish runs only after all story checkpoints.
- Within each phase, every numbered wave blocks the next join line; tasks marked `[P]` touch different files and have no incomplete dependency within their wave.
