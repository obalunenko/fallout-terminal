# Tasks: Interactive Approval Notifications

**Input**: [spec.md](./spec.md), [plan.md](./plan.md), [research.md](./research.md),
[data-model.md](./data-model.md), and [approval-notification contract](./contracts/approval-notification.md)

**Tests**: Go tests are required by the specification and constitution. Each story writes or extends the focused
tests first and observes the intended failure before implementing the behavior.

## Phase 1: Setup

**Purpose**: Establish the deterministic native-notification test boundary shared by every story.

**Wave 1 — single setup task:**

- [x] **T001** Define the fake native notifier, fake App decision target, callback controls, call recorder, and stable contract fixtures used to verify FR-001–FR-016 · `approval_notifications_test.go`

---

## Phase 2: Foundational

**Purpose**: Add the detached observation and best-effort Wails lifecycle seams that block every user story.

### Tests

**Wave 1 — independent (different files):**

- [x] **T002** [P] Write failing tests proving accepted coordination states reach one detached observer, regressing revisions do not, and observer mutation cannot alter stored or emitted state for FR-009, FR-010, and FR-016 · `app_test.go`
- [x] **T003** [P] Write failing tests for notification lifecycle registration order, nonfatal native startup/category failure, and ordered shutdown without exposing another frontend service for FR-012, FR-013, and FR-015 · `wails_host_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent (different files):**

- [x] **T004** [P] Add the narrow coordination observer dependency and invoke it with a detached accepted snapshot outside the App lock while preserving existing event ordering for FR-008, FR-009, and FR-016 · `app.go`
- [x] **T005** [P] Add the root native-notifier interface, lifecycle wrapper, availability/current-record model, logger boundary, and testable construction seams for FR-007, FR-012, FR-013, FR-015, and FR-016 without adding a dependency or frontend binding · `approval_notifications.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — lifecycle integration:**

- [x] **T006** Register the notification lifecycle wrapper before the core application lifecycle and retain reverse ordered shutdown while keeping startup failures nonfatal for FR-012, FR-013, and FR-015 · `wails_host.go`

**Checkpoint**: The App can publish detached states to an inert notification adapter, and the optional native
service can start or fail without changing application availability.

---

## Phase 3: User Story 1 — Notice every approval request (Priority: P1)

**Goal**: Deliver one recognizable native notification for every new command or command-driven terminal
navigation approval while keeping the existing prompt visible.

**Independent Test**: Feed authorized background and foreground scenarios for all command modes, repeated
snapshots, replacement, and terminal-navigation requests into the adapter and verify one exact notification per
request with only the existing prompt information.

### Tests

**Wave 1 — failing delivery contract:**

- [x] **T007** [US1] Write failing table tests for FR-001–FR-005, FR-008, FR-011, and FR-016 across ordinary, state-changing, completed, and terminal-navigation content; exact category/actions; privacy fields; authorization completion; deduplication; replacement; and state-clear cleanup · `approval_notifications_test.go`

### Implementation

**⟶ Wait for Wave 1 to fail as expected, then:**

**Wave 2 — notification state machine:**

- [x] **T008** [US1] Implement FR-001–FR-005, FR-008, FR-011, and FR-016 through pending-request reduction, stable notification identity, bounded presentation content, one-attempt deduplication, asynchronous delivery, replacement invalidation, and best-effort exact-ID cleanup · `approval_notifications.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — application composition:**

- [x] **T009** [US1] Construct, inject, and bind the approval notification adapter before host startup so authoritative App publication drives both native and existing in-app surfaces for FR-001, FR-009, and FR-015 · `main.go`

**Checkpoint**: User Story 1 independently produces one correctly scoped notification per new approval request,
without changing the in-app dialog or any player/session contract.

---

## Phase 4: User Story 2 — Decide from the notification (Priority: P1)

**Goal**: Approve or reject the exact current request through the same authoritative App commands used by the
in-app prompt.

**Independent Test**: Trigger command and navigation requests, invoke both native actions, and verify exact App
routing plus one effective outcome under repeated, stale, malformed, old-process, and concurrent callbacks.

### Tests

**Wave 1 — failing response contract:**

- [x] **T010** [US2] Write failing tests for FR-006, FR-007, FR-010, and FR-014–FR-016 covering approve/reject routing, current-record correlation, ignored callback metadata, default/unknown/error callbacks, replacement and old-process staleness, simultaneous in-app/native decisions, and at least 100 repeated or concurrent responses · `approval_notifications_test.go`

### Implementation

**⟶ Wait for Wave 1 to fail as expected, then:**

**Wave 2 — trusted action routing:**

- [x] **T011** [US2] Implement FR-006, FR-007, FR-010, and FR-014–FR-016 through callback validation, pre-call decision claiming, exact recorded request routing to `ResolveCommandExecution` or `ResolveTerminalNavigation`, failure release, and stale-result suppression outside adapter locks · `approval_notifications.go`

**Checkpoint**: User Story 2 independently resolves each request at most once and converges with the existing
in-app flow on the same server-authoritative outcome.

---

## Phase 5: User Story 3 — Retain a dependable in-app fallback (Priority: P2)

**Goal**: Keep application startup and the in-app decision path usable through every notification permission,
service, delivery, response, cleanup, and shutdown failure.

**Independent Test**: Deny or fail each native operation, deliver late authorization and response results, and
verify that no failure changes coordination state, blocks startup/shutdown, or reports a successful decision.

### Tests

**Wave 1 — failing failure-isolation matrix:**

- [x] **T012** [US3] Write failing table and race tests for FR-011–FR-014 and FR-016 covering authorization granted/denied/revoked/error/timeout, one consent request per launch, native startup/category/send/remove/shutdown failures, late results after stop, missing Linux service behavior, and preserved in-app authority · `approval_notifications_test.go`

### Implementation

**⟶ Wait for Wave 1 to fail as expected, then:**

**Wave 2 — nonfatal lifecycle completion:**

- [x] **T013** [US3] Complete FR-011–FR-014 and FR-016 through asynchronous authorization, generation checks, sanitized failure logging, nonblocking stop, exact pending/delivered cleanup, and fail-closed delivery behavior without waiting on user consent during host startup or shutdown · `approval_notifications.go`

**Checkpoint**: User Story 3 independently proves notification support is optional and every unavailable or failed
native path falls back to the unchanged in-app approval surface.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Document platform behavior and validate all measurable outcomes through the governed workflows.

**Wave 1 — platform documentation:**

- [x] **T014** Document FR-011–FR-013 platform behavior: macOS authorization and packaged/signed requirements, Windows delivered-toast cleanup degradation, Linux notification-daemon requirements, in-app fallback, and honest `NOT RUN` native-smoke reporting · `docs/platform-support.md`

**⟶ Wait for Wave 1 and every story checkpoint to finish, then:**

**Wave 2 — single-owner validation:**

- [x] **T015** Validate FR-001–FR-016 and SC-001–SC-006 with focused notification tests plus `task vet`, `task test`, `task test:race`, `task lint`, `task frontend:build`, `task bindings:check`, `task build`, and `task package`; record matching-host native approve/reject checks as PASS or `NOT RUN` without claiming unavailable evidence · `Taskfile.yml`

---

## Phase 7: Convergence

**Depends on:** Phase 6 (T015).

**Wave 1 — serialized because every task extends the same test file:**

- [x] T016 Add deterministic simultaneous in-app/native decision and App-decision-failure retry tests that prove one effective outcome, stale cleanup, and preserved in-app authority per FR-010, FR-014, US2/AC3, US3/AC3, and T010 (partial) · `approval_notifications_test.go`
- [x] T017 Add controllable authorization and captured-callback tests covering a pending request during consent, revocation or authorization failure, nonblocking shutdown, ignored late authorization completion, ignored post-stop responses, and one consent request per launch per FR-012, FR-016, edge cases, and T012–T013 (partial) · `approval_notifications_test.go`
- [x] T018 Add notification-content edge-case tests for blank-name ID fallback, long and multiline Unicode bounds, privacy fields, and fail-closed simultaneous command/navigation pending state per FR-003–FR-004, edge cases, and T007 (partial) · `approval_notifications_test.go`

---

## Dependencies & Execution Order

### Phase Dependencies

1. **Setup (Phase 1)** establishes the fake boundary used by all notification tests.
2. **Foundational (Phase 2)** depends on Setup and blocks every user story.
3. **User Story 1 (Phase 3)** depends on Foundational and creates the notification delivery path.
4. **User Story 2 (Phase 4)** depends on User Story 1's current-record state machine and adds trusted actions.
5. **User Story 3 (Phase 5)** depends on the delivery and response paths and hardens every fallback.
6. **Polish (Phase 6)** begins after all story checkpoints; validation is the final task.
7. **Convergence (Phase 7)** depends on Phase 6 and closes the remaining test-coverage gaps.

### Wave Order

- Phase 1: T001.
- Phase 2: T002 + T003 → T004 + T005 → T006.
- User Story 1: T007 → T008 → T009.
- User Story 2: T010 → T011.
- User Story 3: T012 → T013.
- Polish: T014 → T015.
- Convergence: T016 → T017 → T018.

### Parallel Opportunities

- T002 and T003 are independent failing-test tasks in different files.
- After those tests exist, T004 and T005 are independent implementations in different files.
- All remaining tasks intentionally serialize because they extend the same adapter test or implementation file,
  integrate a completed contract, or own final validation.

---

## Phase 8: Convergence

**Depends on:** all prior phases.

**Wave 1 — current-launch response authority:**

- [x] T019 Add current-launch response correlation and deterministic restart coverage so an old-process notification cannot act on a restored request with the same kind and request ID while the current-launch notification still can per FR-016, the restart edge case, and research Decision 3 (partial) · `approval_notifications.go`, `approval_notifications_test.go`, `contracts/approval-notification.md`, `data-model.md`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — shutdown ordering:**

- [x] T020 Serialize or generation-guard notification cleanup and shutdown so queued remove or send work cannot call the native notifier after native shutdown while authorization shutdown remains nonblocking per the plan lifecycle and data-model concurrency rules (partial) · `approval_notifications.go`, `approval_notifications_test.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — revocation transition:**

- [x] T021 Detect revoked authorization after delivery failure, transition native delivery to unavailable without repeated send or permission attempts, and prove later requests remain available only through the in-app path per FR-012, US3/AC1–2, and the authorization contract (partial) · `approval_notifications.go`, `approval_notifications_test.go`
