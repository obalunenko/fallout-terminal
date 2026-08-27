# Feature Specification: Go Code Quality Refactor

**Bugfix**: 2026-08-27 — BUG-001 requires repository-governed, warning-free macOS validation.

## User Scenarios & Testing

### User Story 1 - Preserve reliable runtime behavior (Priority: P1)

As an operator or player, I need live state, command failures, and tunnel lifecycle operations to remain reliable while their internal implementation is simplified, so maintenance work cannot introduce hidden state sharing, unstable error handling, or unbounded retries.

**Why this priority**: These paths affect correctness, concurrency, and externally observable failures; defects can corrupt state or prevent startup and shutdown.

**Independent Test**: Exercise live-state projections, classified command failures, and repeated tunnel state changes, then verify returned state is independent, error categories remain stable, and each operation completes without recursive growth.

**Acceptance Scenarios**:

1. **Given** a live-state snapshot containing transition details, **when** a consumer mutates any nested value in the snapshot, **then** the authoritative live state and later snapshots remain unchanged.
2. **Given** a control command fails in a known category, **when** the application presents the failure, **then** the category is selected from structured error identity rather than message wording.
3. **Given** tunnel state changes repeatedly while start, stop, or reconfiguration waits for another operation, **when** the operation resumes, **then** it reaches the same final state and side effects without recursively re-entering itself.

---

### User Story 2 - Keep player actions atomic and consistent (Priority: P1)

As a player, I need navigation and terminal activation to preserve their existing results while shared action mechanics are consolidated, so every route applies the same common state changes without losing route-specific behavior.

**Why this priority**: Player actions are central runtime behavior and must remain atomic across persistence, mutation, and response generation.

**Independent Test**: Dispatch representative navigation and activation actions, including invalid and conflicting actions, and verify committed state, revisions, responses, and error outcomes match the established behavior.

**Acceptance Scenarios**:

1. **Given** a valid player action, **when** it is dispatched, **then** validation, state mutation, persistence, and response construction complete as one atomic operation.
2. **Given** multiple routes activate terminal content, **when** each route succeeds, **then** common activation effects are identical and each route retains its distinct navigation effects.
3. **Given** an invalid or conflicting player action, **when** it is dispatched, **then** no partial state change is committed.

---

### User Story 3 - Apply one set of domain rules and command flows (Priority: P2)

As a maintainer, I need character validation and session commands to use shared rules and orchestration, so equivalent inputs cannot drift into different outcomes across application boundaries.

**Why this priority**: Consolidated rules reduce future defects while preserving necessary validation at every trust boundary.

**Independent Test**: Run equivalent valid and invalid character inputs through each boundary and execute new, open, and demo-copy session commands, verifying consistent validation and unchanged command-specific results.

**Acceptance Scenarios**:

1. **Given** equivalent character data at different application boundaries, **when** it is validated, **then** value constraints produce consistent results while boundary-specific presence checks remain enforced.
2. **Given** a new, open, or demo-copy session request, **when** it succeeds or fails, **then** shared orchestration is used while command-specific labels, side effects, and errors remain unchanged.

---

### User Story 4 - Make complex validation and source areas easier to review (Priority: P3)

As a reviewer, I need complex manifest validation and large source areas separated into cohesive responsibilities, so changes are easier to understand without altering package boundaries or public behavior.

**Why this priority**: Clear organization lowers review cost after the higher-risk behavior has been protected and consolidated.

**Independent Test**: Review the resulting source layout and run the full affected test suite to verify each helper or file has a focused responsibility and every established validation failure is still rejected.

**Acceptance Scenarios**:

1. **Given** an extracted update manifest with any established invalid condition, **when** it is validated after decomposition, **then** it is rejected with the same category of diagnostic.
2. **Given** a maintainer locates code for a specific control, live-state, or application command concern, **when** they inspect its package, **then** related logic is grouped by feature without changing package ownership or public interfaces.
3. **Given** repository-wide Go validation runs on macOS, **when** the canonical Task targets execute it, **then** compilation and linking use the supported macOS 13 deployment environment without target-version warnings.

## Edge Cases

- A live-state projection contains nil, empty, and populated nested transition values.
- A structured error is wrapped one or more times before reaching the application boundary.
- Tunnel state changes or cancellation occurs repeatedly while another lifecycle operation owns the transition.
- A player action fails after validation but before persistence, or conflicts with the latest revision.
- Character input is absent at a transport boundary versus present with an invalid value.
- A session command fails before state replacement, during state replacement, or while generating its result label.
- An update archive contains missing, duplicate, malformed, escaping, or unexpected manifest entries.
- File reorganization encounters generated code or code whose initialization order is observable.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST return fully detached public live-state projections, including every nested terminal-transition value, while preserving nil and empty-value semantics.
- **FR-002**: The system MUST expose stable structured identities for known control-command failure categories and MUST classify wrapped failures without inspecting human-readable message text.
- **FR-003**: Tunnel start, stop, and reconfiguration operations MUST handle repeated in-flight state changes with bounded call-stack growth while preserving cancellation, ownership, buffered mutation, and final-state behavior.
- **FR-004**: Player-action dispatch MUST retain atomic validation, mutation, persistence, revision, and response behavior when decomposed into cohesive phases.
- **FR-005**: Terminal activation paths MUST share their common state-transition behavior while preserving route-specific navigation and failure semantics.
- **FR-006**: Character value constraints MUST have one canonical domain definition reused at application and control boundaries, while each boundary MUST retain its required presence and trust checks.
- **FR-007**: New-session, open-session, and demo-copy commands MUST share common orchestration while preserving each command's labels, inputs, side effects, and errors.
- **FR-008**: Extracted-update manifest validation MUST be separated into focused validation stages while preserving every existing rejection rule and diagnostic category.
- **FR-009**: Cohesive source concerns MAY be moved into feature-focused files within their existing packages, but package ownership, exported interfaces, initialization behavior, and generated files MUST remain unchanged.
- **FR-010**: Refactoring MUST preserve all established user-visible behavior, persisted data compatibility, public/private boundaries, and concurrency guarantees.
- **FR-011**: All changed Go code MUST follow the repository's Go quality and simplification guidance, including clarity, minimal interfaces, explicit error context, dependency discipline, and removal of redundant state or helpers.
- **FR-012**: Every changed Go test MUST register test-owned resource cleanup immediately with test cleanup hooks; cleanup requiring a context MUST use a cancellation-independent bounded context, while lexical cleanup MAY continue to use function-scoped deferral.
- **FR-013**: The completed change MUST pass formatting, static analysis, unit tests, and race-enabled tests applicable to the affected runtime paths.
- **FR-014**: Repository-wide Go validation on macOS MUST use the canonical Task targets, or an explicitly equivalent environment, so the deployment target and CGO flags match the supported macOS 13 baseline without linker target-version warnings.

## Key Entities

- **Public Live-State Projection**: A consumer-visible snapshot of authoritative live state whose nested values must not share mutable storage with the source.
- **Control Failure Identity**: A stable machine-readable category that remains recognizable through error wrapping while retaining human-readable context.
- **Tunnel Lifecycle Operation**: A start, stop, or reconfiguration transition with one owner, queued mutations, cancellation, and a final observable state.
- **Player Action Transaction**: One atomic unit spanning action validation, domain mutation, persistence, revision advancement, and response construction.
- **Character Rules**: Canonical constraints on character values, separate from boundary-specific presence and authorization checks.
- **Session Command**: An application request that prepares or loads state, installs it as the active session, and reports a command-specific result.
- **Extracted Manifest**: The update metadata and file inventory validated before staged content can be accepted.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All affected pre-existing automated tests pass without weakening or deleting behavior assertions.
- **SC-002**: New regression coverage proves mutation of every nested public live-state projection leaves authoritative state unchanged.
- **SC-003**: Known wrapped command failures are classified correctly after their message text is changed in tests.
- **SC-004**: Stress coverage exercises repeated tunnel state changes and completes without stack growth proportional to the number of state changes.
- **SC-005**: Every functional requirement maps to at least one completed implementation task and an automated test or a documented structural verification.
- **SC-006**: Formatting, static analysis, the complete unit test suite, and race-enabled tests for affected concurrent paths all complete successfully.
- **SC-007**: Fresh-cache canonical unit and race test runs on macOS complete with zero linker target-version warnings.

## Assumptions

- This is a behavior-preserving refactor; user-facing features, commands, routes, and messages are not intentionally redesigned.
- Existing package boundaries remain authoritative unless a requirement explicitly permits moving code between files in the same package.
- Existing test fixtures define compatibility for diagnostic categories where exact text is not an external contract.
- Generated protobuf and framework binding files are outside the refactor scope.
- Source-file splitting is applied only where it produces a cohesive feature grouping after logic consolidation; arbitrary line-count reduction is not a goal.
