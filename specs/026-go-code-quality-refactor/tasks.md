# Tasks: Go Code Quality Refactor

**Bugfix**: 2026-08-27 — BUG-001 updated repository-wide macOS verification and contributor guidance.

## Phase 1: Setup

**Wave 1:**

- [x] **T001** Record the green Go baseline for the affected packages before refactoring (FR-010, FR-013) · `app_test.go`

## Phase 2: Foundational Domain Rules

**Wave 1 — independent (different files):**

- [x] **T002** [P] Add regression coverage for complete detached content-node copies, including nil and empty semantics (FR-001, FR-010) · `internal/domain/model_test.go`
- [x] **T003** [P] Add table-driven coverage for canonical character name and intelligence rules (FR-006, FR-012) · `internal/domain/validate_test.go`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2:**

- [x] **T004** Export the canonical domain content-node copy operation and retain complete nested-copy semantics (FR-001, FR-009, FR-011) · `internal/domain/model.go`

**⟶ Wait for T004, then:**

**Wave 3:**

- [x] **T005** Implement reusable domain character value validation using canonical limits (FR-006, FR-011) · `internal/domain/validate.go`

## Phase 3: User Story 1 - Preserve reliable runtime behavior

**Goal**: Make public projections, command failure classification, and tunnel lifecycle coordination robust without changing observable behavior.

**Independent Test**: Mutate projected nested state, classify wrapped control failures after wording changes, and exercise repeated lifecycle waits to verify detached state, stable categories, and bounded call-stack growth.

### Tests

**Wave 1 — independent (different files):**

- [x] **T006** [P] Add a live projection regression that mutates terminal-transition and raw nested values (FR-001, SC-002) · `internal/live/service_test.go`
- [x] **T007** [P] Add control error-identity coverage for stale and persistence failures through wrapping (FR-002, FR-010) · `internal/control/service_test.go`
- [x] **T008** [P] Add tunnel wait/retry stress coverage for stop and reconfiguration ownership changes (FR-003, SC-004) · `internal/tunnel/manager_test.go`

### Implementation

**⟶ Wait for Phase 2 and Wave 1 to finish, then:**

**Wave 2 — independent (different files):**

- [x] **T009** [P] Replace the incomplete live content-node copier with the canonical domain copy operation (FR-001, FR-011) · `internal/live/service.go`
- [x] **T010** [P] Replace the session content-node copier with the canonical domain copy operation (FR-001, FR-011) · `internal/session/service.go`
- [x] **T011** [P] Replace the control content-node copier with the canonical domain copy operation (FR-001, FR-011) · `internal/control/service.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [x] **T012** Define stable control command-resolution error identities and wrap all stale and persistence failure paths (FR-002, FR-009) · `internal/control/errors.go`

**⟶ Wait for T012, then:**

**Wave 4 — independent (different files):**

- [x] **T013** [P] Replace application message parsing with wrapped control error classification while retaining safe output messages (FR-002, SC-003) · `app.go`
- [x] **T014** [P] Convert stop and reconfiguration lifecycle re-entry into lock-releasing loops that retain mutation ownership (FR-003, FR-010) · `internal/tunnel/manager.go`

**Checkpoint**: User Story 1 is independently functional and its high-risk projection, error, and lifecycle behavior is regression-tested.

## Phase 4: User Story 2 - Keep player actions atomic and consistent

**Goal**: Reduce player-action branching and activation duplication while retaining one authoritative transaction and ordered effects.

**Independent Test**: Dispatch direct, linked-terminal, return-route, invalid, duplicate, and conflicting actions and compare results, revisions, committed state, and effect order with established behavior.

### Tests

**Wave 1:**

- [x] **T015** Strengthen representative action coverage around shared activation effects and route-specific navigation state (FR-004, FR-005, FR-010) · `internal/control/service_test.go`

### Implementation

**⟶ Wait for T015, then:**

**Wave 2:**

- [x] **T016** Extract transaction-local player-action validation, terminal-navigation preparation, and accepted-result helpers inside the existing commit boundary (FR-004, FR-005, FR-011) · `internal/control/service.go`

**Checkpoint**: User Story 2 is independently functional with one unchanged commit boundary and common activation mechanics.

## Phase 5: User Story 3 - Apply one set of domain rules and command flows

**Goal**: Share character value rules and session command orchestration without removing trust-boundary checks or command-specific behavior.

**Independent Test**: Submit equivalent character values through application and control boundaries, then run create, open, and demo-copy session commands and compare their telemetry, state resets, and results.

### Tests

**Wave 1 — independent (different files):**

- [x] **T017** [P] Add application-boundary cases proving character value consistency and required pointer presence (FR-006, FR-012) · `app_test.go`
- [x] **T018** [P] Add session facade cases proving create, open, and demo-copy retain distinct callbacks and operation outcomes (FR-007, FR-010) · `internal/session/service_test.go`

### Implementation

**⟶ Wait for Phase 2 and Wave 1 to finish, then:**

**Wave 2 — independent (different files):**

- [x] **T019** [P] Reuse domain character rules at control and application boundaries while retaining boundary-specific validation (FR-006, FR-011) · `internal/control/service.go`
- [x] **T020** [P] Consolidate create, open, and demo-copy application orchestration behind a command-specific helper (FR-007, FR-011) · `app.go`

**Checkpoint**: User Story 3 is independently functional with consistent domain values and unchanged session command results.

## Phase 6: User Story 4 - Make complex validation and source areas easier to review

**Goal**: Make staged-update validation auditable and keep source organization driven by cohesive responsibility.

**Independent Test**: Run all established invalid manifest fixtures and verify the same rejection categories, then inspect changed source files for cohesive helpers and unchanged package ownership.

### Tests

**Wave 1:**

- [x] **T021** Extend manifest validation coverage across identity, package shape, record ordering, and evidence-stage boundaries (FR-008, FR-010, FR-012) · `internal/update/staging_test.go`

### Implementation

**⟶ Wait for T021, then:**

**Wave 2:**

- [x] **T022** Decompose extracted-manifest validation into ordered loading, identity, shape, and file-evidence helpers without changing diagnostics (FR-008, FR-009, FR-011) · `internal/update/staging.go`

**Checkpoint**: User Story 4 is independently functional and every manifest rejection remains covered by focused validation stages.

## Phase 7: Polish and Cross-Cutting Verification

**Wave 1:**

- [x] **T023** Apply the Go quality and simplification review skills to the complete diff, removing only demonstrated duplication and preserving test cleanup ownership (FR-009, FR-011, FR-012) · `AGENTS.md`

**⟶ Wait for T023, then:**

**Wave 2:**

- [x] **T024** Reopened and completed — Run formatting, static analysis, complete unit tests, and race-enabled tests through canonical Task targets; require warning-free fresh-cache macOS unit and race runs (reopened — BUG-001) (FR-010, FR-013, FR-014, SC-001, SC-006, SC-007) · `Taskfile.yml`

**⟶ Wait for T024, then:**

**Wave 3:**

- [x] **T025** Document canonical macOS Go validation and the required deployment environment for equivalent direct commands (BUG-001, FR-014, SC-007) · `AGENTS.md`

## Dependencies & Execution Order

- Phase 1 establishes a verified baseline; Phase 2 creates shared domain primitives.
- User Story 1 depends on the canonical copy primitive; its tests precede each runtime edit and error identity precedes application classification.
- User Story 2 follows the control regression baseline and changes only transaction-local organization.
- User Story 3 depends on the character-rule primitive; its application and control changes may proceed independently from session orchestration.
- User Story 4 is independent of Stories 2 and 3 after baseline, but its tests precede validation decomposition.
- Polish begins only after all story checkpoints; full verification is the final task.

Within each phase, every Wave 1 task marked `[P]` is independent; each explicit join blocks the following wave exactly as shown.
