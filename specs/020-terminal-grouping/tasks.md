# Tasks: Terminal Grouping

**Bugfix**: 2026-08-25 — BUG-001 Updated from bugfix patch; T017 and T035 reopened and UX correction tasks T071–T074 added.

**Bugfix**: 2026-08-25 — BUG-002 Updated from bugfix patch; T037 reopened, legacy-transition repair tasks T075–T079 added, and the traced no-production-drift outcome reconciled with T037/T078.

**Bugfix**: 2026-08-25 — BUG-003 Updated from bugfix patch; T037 and T075–T079 reopened, and production-fidelity fixture and built-desktop verification tasks T080–T081 added.

**Bugfix**: 2026-08-25 — BUG-004 Updated from bugfix patch; T037 and T075–T081 reopened, and exact-authored-file diagnostic and correction tasks T082–T085 added.

**Bugfix**: 2026-08-26 — BUG-005 Updated from bugfix patch; T077, T084, and T085 reopened, and exact grouping-aware action capture and verification tasks T086–T088 added.

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

- [x] **T016** [P] [US1] Replace the flat terminal-list container with accessible high-level group markup and terminal members · `frontend/overseer/src/index.html`
- [x] **T017** [P] [US1] ⚠️ Reopened — replace the fixed-width crowded hierarchy with the wider responsive BUG-001 layout, readable wrapping names, member counts, and collapsible group styling (reopened — BUG-001) · `frontend/overseer/src/overseer.css`

**⟶ Wait for Wave 3 to finish, then:**

- [x] **T018** [US1] Render canonical groups as the top level and keep terminal create/import/delete local state aligned with atomic singleton rules · `frontend/overseer/src/overseer.js`

**Checkpoint**: US1 independently loads, displays, creates, deletes, saves, and reopens terminals with exact-one high-level group representation.

---

## Phase 4: User Story 2 — Manage Groups Safely (P1)

**Goal**: The Overseer can create, rename, dissolve, reorder, merge/split, and move terminals with structured confirmation for destructive changes and atomic stale-safe application.

**Independent Test**: Exercise every CRUD/move flow, cancel and close confirmations, submit stale/repeated confirmations, and verify canonical state changes at most once without terminal-content loss.

### Tests

**Wave 1 — independent failing coverage:**

- [x] **T019** [P] [US2] Add expected-session-revision compare-and-replace, dissolution, move, no-op, persistence-failure, and duplicate-submit tests · `internal/session/service_test.go`
- [x] **T020** [P] [US2] Add expected-coordination-revision, pending-decision, active-route, seeded-order, stale, and atomic rejection tests · `internal/control/service_test.go`
- [x] **T021** [P] [US2] Add application orchestration tests for canonical result/event ordering and stale failure projections · `app_test.go`
- [x] **T022** [P] [US2] Add private protobuf descriptor/adapter tests and public descriptor leak assertions · `app_contract_test.go`
- [x] **T023** [P] [US2] Add desktop method routing tests for the trusted group replacement capability · `desktop_service_test.go`
- [x] **T024** [P] [US2] Add desktop API normalization tests for both revisions, canonical session, coordination state, and errors · `tests/browser/desktop-api.spec.mjs`
- [x] **T025** [P] [US2] Add browser journeys for create, rename, dissolve, move, reorder, impact contents, cancel/close, stale, retry, and singleton-delete rejection · `tests/browser/terminal-grouping.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T026** [US2] Implement complete candidate diffing, strict authored-link validation, content-preserving dissolution, singleton-delete rejection, and actionable affected-item errors · `internal/domain/validate.go`

**⟶ Wait for T026 to finish, then Wave 3 — independent state owners:**

- [x] **T027** [P] [US2] Implement synchronous expected-session-revision group compare-and-replace with atomic durability and detached canonical results · `internal/session/service.go`
- [x] **T028** [P] [US2] Add the group-store seam and coordinator-locked expected-runtime-revision, pending, active-route, and seeded-order guard · `internal/control/service.go`

**⟶ Wait for Wave 3 to finish, then:**

- [x] **T029** [US2] Compose the control-to-session group store adapter without reversing lock or package ownership · `main.go`

**⟶ Wait for T029 to finish, then:**

- [x] **T030** [US2] Route the trusted mutation through the application, advance both authoritative revisions after durability, and publish canonical results/events · `app.go`

**⟶ Wait for T030 to finish, then:**

- [x] **T031** [US2] Map the revisioned request/result through generated private protobuf types · `app_contract.go`

**⟶ Wait for T031 to finish, then:**

- [x] **T032** [US2] Expose only the narrow terminal-group replacement method on the registered desktop service · `desktop_service.go`

**⟶ Wait for T032 to finish, then:**

- [x] **T033** [US2] Regenerate Wails bindings for the new private desktop method without hand-editing generated output · `frontend/overseer/bindings/`

**⟶ Wait for T033 to finish, then Wave 9 — independent UI surfaces:**

- [x] **T034** [P] [US2] Add accessible group CRUD/move controls and the destructive impact confirmation dialog · `frontend/overseer/src/index.html`
- [x] **T035** [P] [US2] ⚠️ Reopened — style target-specific overflow menus, clear focus states, and visually separated destructive actions without inline control crowding (reopened — BUG-001) · `frontend/overseer/src/overseer.css`
- [x] **T036** [P] [US2] Add the revisioned group command and canonical result normalization to the private desktop API facade · `frontend/overseer/src/desktop-api.js`

**⟶ Wait for Wave 9 to finish, then:**

- [x] **T037** [US2] ⚠️ Reopened — verify and regression-cover create/rename/dissolve/move/reorder drafts, impact diffing, cancel-zero-call behavior, duplicate-submit guard, stale refresh, and canonical replacement; capture the exact authored-file UI gesture sequence and prove the reviewed resultant membership is the candidate submitted when moving dormant legacy transition targets (reopened — BUG-002; reopened — BUG-003; reopened — BUG-004) · `frontend/overseer/src/overseer.js`, `tests/browser/terminal-grouping.spec.mjs`

**Checkpoint**: US2 independently provides safe Overseer group management with explicit destructive confirmation and single atomic application.

---

## Phase 5: User Story 3 — Move Forward Only Within a Group (P1)

**Goal**: Only an assigned controller's valid same-group authored transition reaches the existing single Overseer approval flow.

**Independent Test**: Author A→B inside one group and A→C across groups, then verify only A→B can be requested and approved while stale/rejected requests have zero effect.

### Tests

**Wave 1 — independent failing coverage:**

- [x] **T038** [P] [US3] Add catalog tests for same-group, cross-group, self, missing, stale-link, and detached transition lookups · `internal/session/service_test.go`
- [x] **T039** [P] [US3] Add forward request/approve/reject/close, authority, pending, stale-membership, and exact-one-route-point tests · `internal/control/service_test.go`
- [x] **T040** [P] [US3] Add controller/observer browser journeys for same-group forward approval and cross-group zero-effect attempts · `tests/browser/terminal-navigation.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T041** [US3] Make transition lookup return only detached links whose endpoints currently share one canonical group · `internal/session/service.go`

**⟶ Wait for T041 to finish, then Wave 3 — independent consumers:**

- [x] **T042** [P] [US3] Enforce same-group forward eligibility and approval-time link/group revalidation without changing existing approval cardinality · `internal/control/service.go`
- [x] **T043** [P] [US3] Filter terminal-transition destination choices to other members of the edited terminal's current group · `frontend/overseer/src/overseer.js`

**Checkpoint**: US3 independently constrains forward navigation to one group while preserving Overseer approval and zero-effect rejection.

---

## Phase 6: User Story 4 — Return Only Within the Same Group (P1)

**Goal**: Backward navigation remains same-group and approved, and a fresh broadcast started in the middle can traverse the complete ordered group in both directions.

**Independent Test**: Start A/B/C/D at C and complete C→B→A→B→C→D, then invalidate a return membership/order and prove approval changes nothing.

### Tests

**Wave 1 — independent failing coverage:**

- [x] **T044** [P] [US4] Add runtime clone/provenance tests for authored and initial-prefix return points · `internal/domain/model_test.go`
- [x] **T045** [P] [US4] Add first/middle/last start, seeded LIFO, approve/reject/close, stale-order, cross-group return, and manual-activation cleanup tests · `internal/control/service_test.go`
- [x] **T046** [P] [US4] Add the complete C→B→A→B→C→D browser journey with reconnect and no skipped/duplicated activations · `tests/browser/terminal-grouping.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T047** [US4] Add runtime-only return-point origin/group-position provenance and fresh-broadcast initialization state cloning · `internal/domain/model.go`

**⟶ Wait for T047 to finish, then:**

- [x] **T048** [US4] Seed preceding members once, enforce same-group LIFO returns, revalidate provenance/order at approval, and preserve later direct-activation cleanup · `internal/control/service.go`

**Checkpoint**: US4 independently provides approved full-group backward/forward traversal from any starting member.

---

## Phase 7: User Story 5 — Preserve Safe Sessions and Active Play (P2)

**Goal**: Legacy content, active broadcasts, reconnect behavior, direct activation, and unrelated command behavior remain safe and compatible.

**Independent Test**: Open a no-group legacy session, exercise dormant old links and conflicting edits, reconnect multiple players, and run ordinary/state-changing/manual-activation regressions without content loss.

### Tests

**Wave 1 — independent failing/regression coverage:**

- [x] **T049** [P] [US5] Add malformed partial-group, legacy cross-link, unknown-extra preservation, and compatibility validation cases · `internal/domain/validate_test.go`
- [x] **T050** [P] [US5] Add generic-save stale-group protection, legacy dormant-link, terminal lifecycle, rollback, and revision-coalescing regressions · `internal/session/service_test.go`
- [x] **T051** [P] [US5] Add legacy normalization, conflicting edit, reconnect, ordinary/state-changing command, and direct activation browser regressions · `tests/browser/terminal-grouping.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent compatibility fixtures/docs:**

- [x] **T052** [P] [US5] Complete grouped, legacy, pending, route, reconnect, and ordinary-command fixture states · `tests/browser/fixture-server/main.go`
- [x] **T053** [P] [US5] Add an explicit ordered group for the demo's authored transition without changing unrelated content · `sessions/demo.json`
- [x] **T054** [P] [US5] Document group management, singleton compatibility, confirmation, middle-start navigation, and approval behavior · `README.md`

**Checkpoint**: US5 independently proves compatibility and active-play safety across legacy, regression, and reconnect journeys.

---

## Phase 8: Polish and Cross-Cutting Verification

**Wave 1 — review and simplify the completed implementation:**

- [x] **T055** Apply code-simplification and Go-quality review findings without changing specified behavior · `internal/domain/`, `internal/session/`, `internal/control/`, `app.go`, `app_contract.go`, `desktop_service.go`, `frontend/overseer/src/`

**⟶ Wait for T055 to finish, then:**

- [x] **T056** Run protobuf format/lint/generation/breaking checks and Wails binding drift checks, resolving only feature-owned drift · `proto/`, `internal/gen/`, `frontend/client/gen/`, `frontend/overseer/bindings/`

**⟶ Wait for T056 to finish, then:**

- [x] **T057** Run Go formatting, vet, unit tests, and race tests for domain, session, control, application, and private-boundary behavior · `./`

**⟶ Wait for T057 to finish, then:**

- [x] **T058** Run clean Overseer/client builds and all browser acceptance suites covering SC-001 through SC-014 · `frontend/`, `tests/browser/`

**⟶ Wait for T058 to finish, then:**

- [x] **T059** Run the owned application build and available package smoke, then audit every success criterion and report unavailable conditional checks honestly · `build/`, `specs/020-terminal-grouping/spec.md`

## Dependencies & Execution Order

- Phase 2 contracts/types block every user story.
- US1 establishes canonical exact-one groups and blocks group mutation and navigation stories.
- US2 establishes guarded group CRUD and blocks final concurrent-navigation safety evidence.
- US3 and US4 consume the canonical catalog and can be delivered sequentially as forward then backward navigation slices.
- US5 runs after the navigation slices so compatibility and reconnect regressions cover the complete behavior.
- Polish runs only after all story checkpoints.
- Within each phase, every numbered wave blocks the next join line; tasks marked `[P]` touch different files and have no incomplete dependency within their wave.
- BUG-002 Wave 1 establishes failing candidate-integrity coverage before reopened T037 and T078 trace the production path and correct the first stale integration projection if production already preserves the candidate; T079 is the final BUG-002 verification join.
- BUG-003 starts with T080; the checked-in production-fidelity fixture then feeds reopened T075–T077 in parallel, whose failing evidence blocks reopened T037 and T078. T079 verifies the corrected focused suites, and T081 is the final built-desktop save/reopen acceptance join.
- BUG-004 starts with T082 against an unchanged exact authored-file copy. T082 feeds reopened T080 and T075–T077 plus T083–T084; their evidence blocks reopened T037/T078 and T085. T079 verifies the focused correction, and reopened T081 is the final rebuilt-desktop exact-file save/reopen acceptance join.
- BUG-005 starts with T086 against the exact grouping-aware screenshot-producing gesture. T086 feeds reopened T084 and T087 in parallel; their evidence blocks reopened T085, which then blocks reopened T077. T088 re-verifies the previously completed boundary joins and is the final rebuilt-desktop partial-to-amended repair acceptance join.

## Phase 9: Convergence

- [x] T060 Reject newly authored or retargeted cross-group terminal transitions at the generic session-save boundary while preserving unchanged legacy dormant links, with focused session tests in `internal/session/service_test.go` per FR-018 (partial)
- [x] T061 Preserve sanitized session-revision and candidate-validation feedback through the terminal-group store/coordinator boundary and identify affected pending endpoints without exposing persistence details per FR-021 and FR-044 (partial)
- [x] T062 Track and validate the initialized active group position, seeded successor chain, and pending-return adjacency so terminal reorders that would invalidate backward navigation are rejected atomically per FR-034 (partial)
- [x] T063 Include every removed and collision-adjusted resultant singleton group plus exact terminal-to-group membership in the destructive dissolution impact dialog and its browser assertions per FR-040 (partial)
- [x] T064 Add browser acceptance journeys that reject an authored-link-conflicting group edit and prove cross-group direct Overseer activation clears the player-created return route per plan: Browser acceptance (missing)

## Phase 10: Convergence

- [x] T065 Reject a group reorder that preserves stored seeded points but moves the current active terminal away from the remaining seeded return target after earlier prefix returns, with focused coordinator regression coverage in `internal/control/service_test.go` per FR-034 (partial)

## Phase 11: Convergence

- [x] T066 Reject a group reorder that preserves the current authored return target but breaks the deeper adjacency between an authored route point and the remaining seeded prefix it will expose, with mixed-route atomic regression coverage in `internal/control/service_test.go` per FR-034 (partial)
- [x] T067 Add a confirmed split-to-singleton operation that separates one terminal from a multi-terminal group while retaining every other source member, generates a collision-safe valid singleton candidate, and covers the complete browser journey in `frontend/overseer/src/overseer.js` and `tests/browser/terminal-grouping.spec.mjs` per US1/AC3 (missing)
- [x] T068 Include stable authored-command identity in sanitized cross-group replacement rejection feedback and assert it through domain, coordinator/application, and browser coverage so multiple links with the same terminal endpoints remain distinguishable per US5/AC3 (partial)

## Phase 12: Convergence

- [x] T069 Add an exact three-group, ten-terminal save/reopen acceptance matrix that preserves every group identity, name, group order, member order, and exact-one membership in `internal/session/service_test.go` per SC-001 (partial)
- [x] T070 Aggregate deterministic sanitized identities for every authored command invalidated by one cross-group replacement, including multiple commands with the same endpoints, and assert the complete feedback through domain, coordinator/application, and browser coverage per US5/AC3 (partial)

## Phase 13: BUG-001 — Terminal List UX Correction

**Visual reference**: `bugs/assets/BUG-001-terminal-list-ux-mockup.png`

**Wave 1 — failing UX coverage:**

- [x] **T071** [US1] [US2] Add browser journeys for 1280×720 and 1600×900 layout integrity, readable Russian names, group disclosure, target-specific menu labels, destructive-action separation, selection preservation, and pointer/keyboard reachability per FR-048–FR-051 and SC-015–SC-016 · `tests/browser/terminal-grouping.spec.mjs`

**⟶ Wait for T071 to finish, then complete reopened T017 and T035 and:**

- [x] **T072** [US1] [US2] Render member counts, persistent in-memory disclosure state, and one accessible target-specific action menu for every group and terminal while reusing existing mutation handlers · `frontend/overseer/src/index.html`, `frontend/overseer/src/overseer.js`

**⟶ Wait for T017, T035, and T072 to finish, then:**

- [x] **T073** [US1] [US2] Add Escape/outside-click menu closure, deterministic focus restoration, and regression coverage for selection and dialog transitions initiated from contextual menus · `frontend/overseer/src/overseer.js`, `tests/browser/terminal-grouping.spec.mjs`

**⟶ Wait for T073 to finish, then:**

- [x] **T074** [US1] [US2] Run the Overseer build and terminal-grouping browser suite, verify the supported viewport matrix against the BUG-001 mockup, and record all BUG-001 requirements as satisfied · `frontend/overseer/`, `tests/browser/`, `specs/020-terminal-grouping/bugs/BUG-001.md`

## Phase 14: BUG-002 — Legacy Transition Repair

**Wave 1 — independent failing legacy-repair coverage:**

- [x] **T075** [P] [US5] ⚠️ Reopened — retain the A to B repair and load the exact BUG-004 authored document plus the T080 fixture in domain regressions to prove their equivalent per-edge complete-candidate classification for partial and full repairs per FR-052–FR-053 and SC-017–SC-018 (reopened — BUG-003; reopened — BUG-004) · `internal/domain/validate_test.go`
- [x] **T076** [P] [US2] [US5] ⚠️ Reopened — add session and application regressions that open an unchanged copy of the exact BUG-004 no-group legacy document, carry exact partial and complete candidates through production replacement, persist and reopen atomically, and preserve both command identities and terminal content per FR-043 and FR-052–FR-053 (reopened — BUG-003; reopened — BUG-004) · `internal/session/service_test.go`, `app_contract_test.go`, `app_test.go`
- [x] **T077** [P] [US2] [US5] ⚠️ Reopened — extend the exact-authored-file browser journey to begin with the BUG-005 screenshot-producing gesture, assert the complete partial membership and actionable independently split-edge feedback, amend that same repair into one all-three-terminal proposal, reopen the session, and prove both commands become eligible per US5/AC10–AC11 and SC-018–SC-019 (reopened — BUG-003; reopened — BUG-004; reopened — BUG-005) · `tests/browser/fixture-server/main.go`, `tests/browser/terminal-grouping.spec.mjs`

**⟶ Wait for T075–T077 to fail for the reported path, then complete reopened T037 and:**

- [x] **T078** [US2] [US5] ⚠️ Reopened — trace the exact BUG-004 authored-file partial and complete candidates through the desktop facade, private route, coordinator, session validator, and persistence boundary; compare each with the reviewed membership, correct the first stale membership or error-classification boundary found, and retain rejection only for edges genuinely split by the resultant candidate (reopened — BUG-003; reopened — BUG-004) · `frontend/overseer/src/desktop-api.js`, `app_contract.go`, `app.go`, `internal/control/service.go`, `internal/session/service.go`, `tests/browser/fixture-server/main.go`, `specs/020-terminal-grouping/bugs/BUG-002.md`, `specs/020-terminal-grouping/bugs/BUG-003.md`, `specs/020-terminal-grouping/bugs/BUG-004.md`

**⟶ Wait for T037 and T078 to finish, then:**

- [x] **T079** [US2] [US5] ⚠️ Reopened — run the focused domain, session, application, desktop-facade, and terminal-grouping browser suites against both the exact BUG-004 authored-file copy and T080; verify per-edge feedback, full-candidate save/reopen, canonical pre/post state, and eligibility for both transitions without weakening genuine cross-group rejection (reopened — BUG-003; reopened — BUG-004) · `internal/domain/`, `internal/session/`, `app*_test.go`, `tests/browser/`, `specs/020-terminal-grouping/bugs/BUG-002.md`, `specs/020-terminal-grouping/bugs/BUG-003.md`, `specs/020-terminal-grouping/bugs/BUG-004.md`

## Phase 15: BUG-003 — Production-Fidelity Multi-Link Legacy Repair

**Wave 1 — source-faithful regression fixture:**

- [x] **T080** [US5] ⚠️ Reopened — compare the sanitized checked-in `session-05-cold-storage` fixture with an unchanged copy of the exact BUG-004 source at SHA-256 `b4ca8b89b7d7af32e05a9b598a007e36a747ef59ce3e2bd15a60d0b3f0ec9438`, document every intentional difference, and prove behavioral equivalence before using the fixture as permanent evidence per FR-053 and SC-018 (reopened — BUG-004) · `tests/fixtures/`, `tests/browser/fixture-server/main.go`

**⟶ Wait for T080, then complete reopened T075–T077; wait for their failing evidence, then complete reopened T037 and T078; wait for T079, then:**

- [x] **T081** [US2] [US5] ⚠️ Reopened — rebuild the owned desktop application, record its identity, run it against an unchanged disposable copy of the exact BUG-004 authored file, correlate the reviewed and submitted complete candidate, save and reopen, verify both authored transitions are eligible, and record final BUG-004 acceptance evidence (reopened — BUG-004) · `build/`, `tests/browser/`, `specs/020-terminal-grouping/bugs/BUG-003.md`, `specs/020-terminal-grouping/bugs/BUG-004.md`

## Phase 16: BUG-004 — Exact Authored-File Candidate Trace

**Wave 1 — capture the contradictory production path:**

- [x] **T082** [US2] [US5] Reproduce from an unchanged disposable copy of the exact reported authored JSON; verify its SHA-256, record the executable/build identity and precise UI gesture sequence, and capture the reviewed membership plus canonical session and revision immediately before and after rejection · `build/`, `tests/fixtures/`, `specs/020-terminal-grouping/bugs/BUG-004.md`

**⟶ Wait for T082, then Wave 2 — independent boundary and regression evidence:**

- [x] **T083** [P] [US2] [US5] Capture and assert semantic equality of the reviewed terminal-to-group index and serialized request at the desktop facade, private protobuf adapter, application, coordinator, session validator, and persistence boundary, including canonical pre/post revision assertions · `frontend/overseer/src/desktop-api.js`, `app_contract_test.go`, `app_test.go`, `internal/control/`, `internal/session/`
- [x] **T084** [P] [US2] [US5] ⚠️ Reopened — extend the exact-authored-file regressions from the BUG-005 screenshot-producing action; assert the editor's partial membership, deterministic actionable split-edge feedback with zero mutation, and amendment into the exact atomic all-three-terminal candidate per FR-054 and SC-019 (reopened — BUG-005) · `internal/domain/validate_test.go`, `internal/session/service_test.go`, `app_test.go`, `tests/browser/fixture-server/main.go`, `tests/browser/terminal-grouping.spec.mjs`

**⟶ Wait for reopened T037, T075–T078, T080, and T083–T084, then:**

- [x] **T085** [US2] [US5] ⚠️ Reopened — correct the first boundary that changes the BUG-005 reviewed candidate or, when the action is intentionally partial, retain the draft and provide complete resultant membership, actionable independently split-edge feedback, and direct amendment into one valid all-endpoint proposal; retain strict rejection for genuinely split edges per FR-054 (reopened — BUG-005) · `frontend/overseer/src/`, `app_contract.go`, `app.go`, `internal/control/`, `internal/session/`

**⟶ Wait for T085, then complete reopened T079; wait for T079, then complete reopened T081.**

**BUG-005 supersession**: Phase 17 replaces this earlier completion join for the newly reported grouping-aware action path.

## Phase 17: BUG-005 — Grouping-Aware Action Repair

**Wave 1 — capture the exact post-fix user path:**

- [x] **T086** [US2] [US5] Reproduce the screenshot-producing gesture in the current owned grouping-aware bundle; record the executable and source hashes, exact menu action, editor selections, confirmation contents, serialized candidate, and canonical session/coordination revisions before and after rejection · `build/`, `frontend/overseer/src/`, `specs/020-terminal-grouping/bugs/BUG-005.md`

**⟶ Wait for T086, then complete reopened T084 and Wave 2 in parallel:**

- [x] **T087** [P] [US2] [US5] Correlate the BUG-005 editor and review terminal-to-group index with the desktop facade, private protobuf adapter, application, coordinator, session validator, and persistence boundary; identify whether the live proposal is partial or loses membership in transit and retain canonical pre/post assertions · `frontend/overseer/src/desktop-api.js`, `app_contract_test.go`, `app_test.go`, `internal/control/`, `internal/session/`, `specs/020-terminal-grouping/bugs/BUG-005.md`

**⟶ Wait for T084 and T087, then complete reopened T085; wait for T085, then complete reopened T077; wait for T077, then:**

- [x] **T088** [US2] [US5] Re-verify T037, T078–T079, T081, and T083 against the captured BUG-005 live candidate, rebuild the owned desktop bundle, execute the exact user-visible partial-to-amended repair against an unchanged authored-file copy, save/reopen, exercise `svc-access-admin` and `adm-emergency`, and record final SC-019 evidence · `build/`, `internal/`, `app*_test.go`, `tests/browser/`, `specs/020-terminal-grouping/bugs/BUG-005.md`
