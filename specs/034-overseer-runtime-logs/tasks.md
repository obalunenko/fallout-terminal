# Tasks: Overseer Runtime Logs

## Phase 1: Setup

No standalone setup is required. The repository already contains the pinned logger, private protobuf generation, Wails binding generation, browser-test workspace, and canonical Taskfile workflows needed by this feature.

## Phase 2: Foundational Infrastructure

These tasks establish retained output, platform access, and the private contract required by every user story.

**Wave 1 — independent failing contracts:**

- [x] **T001** [P] Add failing tests for run identity, line-preserving writes, rotation, current/previous-run retention, owned-file filtering, and deterministic clocks/IDs · `internal/diagnostics/retained_log_test.go`
- [x] **T002** [P] Add failing target-aware path and native fixed-directory open tests, including missing, unsafe, and not-ready targets · `internal/platform/paths_test.go`, `internal/platform/desktop_test.go`
- [x] **T003** [P] Add failing private protobuf descriptor and native round-trip assertions for the log-access result while proving public descriptors remain unchanged · `app_contract_test.go`

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent implementations:**

- [x] **T004** [P] Implement the concurrency-safe run-scoped retained writer, text-record mirroring seam, 5 MiB segments, eight-segment retention, protected current/previous runs, safe permissions, and current-path lookup · `internal/diagnostics/retained_log.go`
- [x] **T005** [P] Resolve the fixed log directory below application support and extend the Wails platform adapter with an allowlisted directory-open operation that does not broaden HTTP(S) URL handling · `internal/platform/paths.go`, `internal/platform/desktop.go`
- [x] **T006** [P] Add the additive private `LogAccessResult` schema and regenerate its Go protobuf output · `proto/fallout/terminal/private/v1/desktop.proto`, `internal/gen/fallout/terminal/private/v1/desktop.pb.go`

**⟶ Wait for Wave 2 to finish, then:**

- [x] **T007** Compose the retained writer exactly once with the existing logger, attach `run_id` to the root logger, preserve standard-error fallback, close the owned writer after application shutdown, and inject fixed log-location dependencies · `main.go`, `app.go`

## Phase 3: User Story 1 — Trace Players and Command Decisions (P1)

**Goal**: Produce one safe, correlated, authoritative record of logical player presence, role transitions, command requests, decisions, and exceptional outcomes.

**Independent Test**: Connect an unassigned logical session, change its role, submit approved and declined commands plus duplicate and failing cases, then disconnect and verify exact ordered events and correlations.

### Tests

**Wave 1 — independent failing behavior tests:**

- [x] **T008** [P] Add failing coordinator tests for aggregate connect/disconnect semantics, role transitions, pending command creation, decisions, replay, duplicate, stale, conflict, and execution failure audit facts · `internal/control/service_test.go`
- [x] **T009** [P] Add failing root tests for audit-to-logger field/severity mapping, revision order, run correlation, and forbidden field absence · `app_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent owners:**

- [x] **T010** [P] Define the closed safe audit vocabulary and emit detached logical-session and command lifecycle events from accepted coordinator transitions without importing the logger or exposing display content · `internal/control/service.go`
- [x] **T011** [P] Route audit effects through root composition and format allowlisted `player.*` and `command.*` records with stable fields and severity · `main.go`, `app.go`

**Checkpoint**: User Story 1 is independently functional when logical connection, role, request, decision, outcome, and disconnection records correlate exactly once without character names or command content.

## Phase 4: User Story 2 — Retrieve Diagnostics from a Released Application (P1)

**Goal**: Let the Overseer open the retained-log directory and identify the active file from both startup and normal screens without a terminal.

**Independent Test**: Exercise the private action and both Overseer controls under success, failure, startup-degraded, and repeated-click conditions, then verify target-aware paths and matching-host packaged behavior.

### Tests

**Wave 1 — independent failing access tests:**

- [x] **T012** [P] Add failing root and desktop-facade tests for no-argument fixed-path opening, safe errors with the exact intended path, active-file reporting, and availability during degraded startup · `app_test.go`, `desktop_service_test.go`
- [x] **T013** [P] Add failing browser tests for both `data-action="open-log-location"` controls, pending de-duplication, accessible success/error feedback, and manual-path fallback · `tests/browser/overseer-runtime-logs.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T014** Implement `OpenLogLocation`, protobuf-native result routing, safe error mapping, and the narrow private desktop-service method without accepting a path from the frontend · `app.go`, `app_contract.go`, `desktop_service.go`

**⟶ Wait for T014 to finish, then:**

- [x] **T015** Regenerate the Wails desktop bindings for `OpenLogLocation` and confirm no unrelated generated drift · `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js`, `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/models.js`

**⟶ Wait for T015 to finish, then Wave 4 — independent UI and package evidence:**

- [x] **T016** [P] Add the normalized desktop API call, startup and normal log buttons, pending state, and shared accessible result feedback · `frontend/overseer/src/desktop-api.js`, `frontend/overseer/src/index.html`, `frontend/overseer/src/overseer.js`
- [x] **T017** [P] Add an optional macOS matching-host packaged log-access smoke and expose it through the canonical Taskfile without making unavailable native evidence a release gate · `scripts/runtime-logs-macos-smoke.sh`, `Taskfile.yml`

**Checkpoint**: User Story 2 is independently functional when a packaged Overseer can reach the fixed log directory in two interactions or receive the exact path for manual navigation.

## Phase 5: User Story 3 — Diagnose Hacking Puzzle Activity (P2)

**Goal**: Record safe puzzle-generation, guess, pattern, terminal outcome, reset, force-success, and interruption facts without solution-bearing data.

**Independent Test**: Drive deterministic puzzles through rejected and accepted guesses, dud removal, attempt replenishment, failure, reset, success, trusted force-success, suspension, discard, and broadcast end; verify categories, counts, correlations, and redaction.

### Tests

**Wave 1:**

- [x] **T018** Add failing authoritative tests for `hack.started`, `hack.guess`, `hack.pattern`, `hack.succeeded`, `hack.failed`, `hack.reset`, and `hack.interrupted`, including puzzle correlation and forbidden-value canaries · `internal/control/service_test.go`

### Implementation

**⟶ Wait for T018 to finish, then:**

- [x] **T019** Emit hacking audit facts from canonical before/after puzzle state and explicit player actions, normalizing outcomes without carrying targets, words, board data, activity text, or raw errors · `internal/control/service.go`

**Checkpoint**: User Story 3 is independently functional when a complete puzzle journey can be reconstructed by event, puzzle ID, terminal ID, session role, and attempt counts while its solution remains undiscoverable from logs.

## Phase 6: User Story 4 — Keep Retained Diagnostics Safe and Manageable (P3)

**Goal**: Prove bounded storage, recoverable failures, safe concurrency, and complete redaction under sustained use.

**Independent Test**: Generate concurrent high-volume records and injected filesystem failures across restarts, then verify the bound, protected current/previous evidence, one fallback warning, race safety, and zero secret or gameplay canary leaks.

### Tests

**Wave 1 — independent failure and redaction tests:**

- [x] **T020** [P] Extend retained-writer tests for concurrent writes/path reads, oversized records, rotation failure, disk-full behavior, malformed historical files, symlink rejection, cleanup failure, idempotent close, and one-shot fallback warnings · `internal/diagnostics/retained_log_test.go`
- [x] **T021** [P] Add integration canaries covering credentials, names, command content/output, puzzle targets/words/board/activity text, raw dependency errors, and fixed-path access results · `app_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent hardening:**

- [x] **T022** [P] Harden retention and fallback behavior so storage failures never propagate into gameplay, active files are never pruned, unknown files are untouched, and the writer remains race-safe and closable · `internal/diagnostics/retained_log.go`
- [x] **T023** [P] Extend repository secret-leak and application contract checks to cover retained runtime logs and forbid public/player log-access exposure · `scripts/secret-leak-check.sh`, `app_contract_test.go`

**Checkpoint**: User Story 4 is independently functional when retained diagnostics stay within the documented bound, survive restart as specified, degrade to standard error safely, and expose zero forbidden raw values.

## Phase 7: Polish and Cross-Cutting Validation

**Wave 1:**

- [x] **T024** Run `go fix ./...`, review every modernization edit, regenerate protobuf and Wails bindings through repository workflows, apply final formatting, and confirm generated drift contains only intentional contract changes · repository-wide generated and Go sources

**⟶ Wait for T024 to finish, then:**

- [x] **T025** Validate SC-001 through SC-008 with `task vet`, `task test`, `task test:race`, `task lint`, frontend clean builds under the `.nvmrc` runtime, focused and full browser tests, secret-leak checks, `task build`, `task package`, and the optional matching-host runtime-log smoke when available · `Taskfile.yml`, `scripts/`, `tests/browser/`

## Dependencies & Execution Order

- Phase 1 confirms no additional setup is needed.
- Phase 2 Wave 1 writes independent failing contracts; Wave 2 implements their separate owners; T007 joins them into the single application logger and blocks all user stories.
- Phase 3 Wave 1 writes independent coordinator and root tests; Wave 2 implements coordinator facts and root formatting in separate owners.
- Phase 4 Wave 1 writes root/facade and browser tests; T014 adds the private method, T015 regenerates bindings, then T016 and T017 complete independent UI and package evidence.
- Phase 5 is sequential because both the failing tests and implementation touch the coordinator owner.
- Phase 6 Wave 1 writes independent writer and integration tests; Wave 2 hardens the writer and repository-wide leak contract independently.
- Phase 7 serializes generation/formatting before the single-owner validation task.
- Story delivery order is US1 and US2 (P1) → US3 (P2) → US4 (P3) → Polish. Phase 4 depends on the foundational path, contract, writer, and composition but not on completed US1 behavior; Phase 5 depends on the shared audit vocabulary from US1.

## Phase 8: Convergence

**Depends on:** all prior phases.

**Wave 1 — independent security, command, and retention corrections:**

- [x] T026 CRITICAL replace every raw dependency-error field that can reach retained logs with allowlisted error categories, and add real retained-file canaries proving credentials, private paths, provider details, and injected dependency messages cannot escape per Constitution: Secret and Credential Governance and FR-017 (contradicts) · `main.go`, `app.go`, `app_test.go`, `internal/player/server.go`, `internal/player/server_test.go`
- [x] T027 represent the command decision separately from its execution outcome and emit exactly-once safe categories for stale, duplicate, invalid, superseded, declined, approved-success, and approved-failure paths per FR-005 and FR-006 (partial) · `internal/control/service.go`, `internal/control/service_test.go`, `app.go`, `app_test.go`
- [x] T028 enforce the documented finite byte boundary for oversized records and add deterministic clean-restart, unexpected-stop, rotation-failure, disk-full, cleanup-failure, malformed-history, current-file, and previous-run tests per FR-015, SC-004, and SC-005 (partial) · `internal/diagnostics/retained_log.go`, `internal/diagnostics/retained_log_test.go`

**⟶ Wait for T027 to finish, then:**

**Wave 2 — complete hacking interaction categories:**

- [x] T029 emit correlated safe records for rejected guesses and patterns and distinguish dud removal from attempt replenishment without carrying pattern, board, candidate, or solution data per FR-007 and US3/AC2-3 (partial) · `internal/control/service.go`, `internal/control/service_test.go`, `app.go`, `app_test.go`

**⟶ Wait for T029 to finish, then:**

**Wave 3 — complete hacking lifecycle context:**

- [x] T030 carry available acting session and role into hacking terminal outcomes, add safe puzzle difficulty and interruption-reason metadata, and prevent solved or failed puzzles from later being mislabeled as interrupted per FR-008 and US3/AC1,AC4 (partial) · `internal/control/service.go`, `internal/control/service_test.go`, `app.go`, `app_test.go`

**⟶ Wait for Waves 1–3 to finish, then:**

**Wave 4 — end-to-end conformance and optional native evidence:**

- [x] T031 add exact-once retained-log journeys for connection, role, command, and complete hacking lifecycles plus actual-file redaction, unavailable-storage continuity, cross-platform composed paths, and actionable UI failure feedback per FR-018 and SC-001, SC-002, SC-006, SC-007 (partial) · `app_test.go`, `internal/control/service_test.go`, `internal/diagnostics/retained_log_test.go`, `internal/platform/paths_test.go`, `tests/browser/overseer-runtime-logs.spec.mjs`
- [x] T032 make the optional matching-host package smoke exercise the Overseer log-access action and two-interaction/ten-second evidence, or explicitly report that UI portion as `NOT RUN` without presenting retention-only evidence as an SC-008 pass per T017 and SC-008 (partial) · `scripts/runtime-logs-macos-smoke.sh`, `Taskfile.yml`

## Phase 9: Convergence

**Depends on:** all prior phases.

**Wave 1 — independent controller-lifecycle and retention-bound corrections:**

- [x] T033 emit exactly one correlated `hack.interrupted` event with reason `controller-unavailable` when an active puzzle loses its controller without a terminal or puzzle-state change, and add authoritative release/move coverage proving solved and failed puzzles are not mislabeled per FR-007 and US3/AC4 (partial) · `internal/control/service.go`, `internal/control/service_test.go`
- [x] T034 prevent cleanup failures from admitting retained segments beyond the configured finite boundary, degrade safely to fallback without further growth, and add deterministic repeated-rotation cleanup-failure coverage per FR-015 and SC-005 (contradicts) · `internal/diagnostics/retained_log.go`, `internal/diagnostics/retained_log_test.go`

## Phase 10: Convergence

**Depends on:** all prior phases.

**Wave 1 — independent controller-handoff and fallback-warning corrections:**

- [x] T035 emit exactly one old-controller-correlated `hack.interrupted` event when `SetActiveController` directly hands an active puzzle to another controller, and add authoritative handoff coverage proving solved and failed puzzles are not mislabeled per FR-007 and US3/AC4 (partial) · `internal/control/service.go`, `internal/control/service_test.go`
- [x] T036 ensure one retained-storage initialization failure produces exactly one safe degradation warning across the retained writer and production root composition, and add composed count and raw-error-redaction coverage per plan: retained fallback decision (partial) · `main.go`, `internal/diagnostics/retained_log.go`, `app_test.go`

## Phase 11: Convergence

**Depends on:** all prior phases.

**Wave 1 — close the long-running retention filename boundary:**

- [x] T037 keep every generated retained segment recognizable and prunable after the three-digit ordinal range, and add deterministic coverage crossing 1,000 rotations that proves owned file count and retained bytes never exceed the configured bounds per FR-015 and SC-005 (contradicts) · `internal/diagnostics/retained_log.go`, `internal/diagnostics/retained_log_test.go`
