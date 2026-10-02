# Tasks: Shared Facility State System

## Phase 1: Setup

Prepare reusable fixtures and test entrypoints without changing production behavior.

**Wave 1 — independent (different files):**

- [x] **T001** [P] Add a representative version-1 facility session fixture with devices, transitions, conditions, bindings, recovery programs, and legacy command state · internal/testutil/testdata/session-v1-facility.json
- [x] **T002** [P] Add reusable browser fixture builders for facility authoring, multi-terminal projection, diagnostic faults, and lifecycle restoration · tests/browser/fixtures/facility-session.mjs
- [x] **T003** [P] Register the focused facility browser suites in the Overseer test entrypoint · frontend/overseer/package.json

---

## Phase 2: Foundational

Build the schema, domain, persistence, projection, and transaction foundations that block every user story.

### Tests

**Wave 1 — independent failing contract and model tests (different files):**

- [x] **T004** [P] Add failing tests for facility cloning, finite state graphs, equality rules, typed failures, and protected current values · internal/domain/facility_test.go
- [x] **T005** [P] Add failing version-1 JSON tests for absent-facility compatibility and unknown-field preservation on every nested facility entity · internal/domain/json_test.go
- [x] **T006** [P] Add failing protobuf round-trip and field-presence tests for the facility persistence contract · internal/session/contract_test.go
- [x] **T007** [P] Add failing descriptor tests for additive persistence, private facility results, command availability, and presentation-effect fields · app_contract_test.go
- [x] **T008** [P] Add failing tests for protected facility state during ordinary saves and distinct document/facility revisions · internal/session/service_test.go
- [x] **T009** [P] Add failing tests for one shared coordinator facility snapshot outside broadcast-owned state · internal/control/service_test.go
- [x] **T010** [P] Add failing tests for deterministic precedence and detached facility projection inputs · internal/live/service_test.go
- [x] **T011** [P] Add failing public adapter tests for absent-is-available command presence and bounded terminal effects · internal/player/adapter_test.go

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — independent protobuf sources (different files):**

- [x] **T012** [P] Add the optional Facility aggregate, device graph, condition effects, recovery programs, bindings, and StateChangeConfig facility action with additive field numbers · proto/fallout/terminal/persistence/v1/session.proto
- [x] **T013** [P] Add optional command availability and bounded terminal presentation effects while preserving existing player message variants · proto/fallout/terminal/player/v1/terminal.proto
- [x] **T014** [P] Add redacted pending facility-action summaries and facility revision visibility to private coordination state · proto/fallout/terminal/private/v1/coordination.proto
- [x] **T015** [P] Add typed facility issues, operation results, authoring, preview, reset, and recovery request messages · proto/fallout/terminal/private/v1/desktop.proto

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — generated contract join:**

- [x] **T016** Regenerate governed Go and ECMAScript protobuf outputs, update the reviewed schema revision, and refresh the compatibility baseline without manual generated edits · proto/schema-revision.txt

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — core domain owner:**

- [x] **T017** Implement facility devices, finite states, transitions, state equalities, conditions, typed effects, recovery programs, bindings, actions, revisions, results, and dependency identities · internal/domain/facility.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — independent adapters and invariants (different files):**

- [x] **T018** [P] Extend session and runtime deep clones for every nested facility value without aliasing canonical state · internal/domain/model.go
- [x] **T019** [P] Extend explicit version-1 JSON decoding and encoding with facility known-field sets and nested unknown-field preservation · internal/domain/json.go
- [x] **T020** [P] Add bounded whole-session facility graph validation, reference validation, equality restrictions, overlap rejection, and escape/recovery validation · internal/domain/validate.go

**Checkpoint**: Facility contracts and canonical domain values are defined, generated, clonable, round-trippable, and reject malformed graphs before story implementation begins.

---

## Phase 3: User Story 1 - Change shared devices through approved commands (Priority: P1)

**Goal**: One approved state-changing command can atomically persist its completion snapshot and one or more validated device transitions, while rejection and every failure leave the facility unchanged.

**Independent Test**: Request a two-device action, reject it, then approve it; verify no pre-approval mutation, one successful document replacement and facility revision, stable failures, and no duplicate transition under retries or concurrency.

### Tests

**Wave 1 — independent failing tests (different files):**

- [x] **T021** [P] [US1] Add failing store tests for simultaneous pre-state validation, one transition per device, condition effects, one write, rollback, no-op replay, and facility revision increments · internal/session/facility_test.go
- [x] **T022** [P] [US1] Add failing coordinator tests for pending snapshots, approval revalidation, rejection, stale/duplicate/concurrent decisions, and write-before-publication · internal/control/facility_test.go
- [x] **T023** [P] [US1] Add failing application tests for typed approval outcomes, canonical session-state publication, and the existing rejected access-error lifecycle · app_test.go
- [x] **T024** [P] [US1] Add failing controller/observer browser journeys for pending, reject, approve, stale, failed-save, repeated, and concurrent facility commands · tests/browser/facility-player-state.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — durable world-action owner:**

- [x] **T025** [US1] Implement one serialized world-action candidate that captures command completion, validates all transition sources and preconditions against one pre-state, applies all destinations/effects, and performs one atomic replacement · internal/session/facility.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent coordination contracts (different files):**

- [x] **T026** [P] [US1] Add the narrow FacilityStore seam, detached mutation result, pending action fingerprint, and safe audit payloads · internal/control/facility.go
- [x] **T027** [P] [US1] Add typed sentinel-to-FacilityFailureCode mappings without error-message matching · internal/control/errors.go
- [x] **T028** [P] [US1] Add explicit private protobuf adapters for facility approval summaries and structured outcomes · app_contract.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — coordinator transaction integration:**

- [x] **T029** [US1] Extend command selection and ResolveCommandExecution to snapshot facility intent, hold the coordinator transaction across one store call, install only the canonical result, and publish rejection or success exactly once · internal/control/service.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — independent application and fixture consumers (different files):**

- [x] **T030** [P] [US1] Route typed facility approval results, publish changed session state after durability, and preserve the current player rejection surface · app.go
- [x] **T031** [P] [US1] Extend the browser fixture server with deterministic approval, stale-revision, conflict, and persistence-failure responses · tests/browser/fixture-server/main.go
- [x] **T032** [P] [US1] Show the redacted facility impact in the existing approval dialog and reconcile typed failure results without creating another player error view · frontend/overseer/src/overseer.js

**Checkpoint**: User Story 1 is independently functional and testable as one atomic approved world action through the existing private approval lifecycle.

---

## Phase 4: User Story 2 - See one facility state from every terminal (Priority: P1)

**Goal**: Every terminal derives labels, EntryContent blocks, availability, and visibility from one shared facility snapshot with deterministic precedence and safe navigation repair.

**Independent Test**: Bind one device across five terminals in three groups, commit one transition, and verify identical current and reconnect projections, correct open-entry updates, preserved completed-command results, and no state change from group moves.

### Tests

**Wave 1 — independent failing tests (different files):**

- [x] **T033** [P] [US2] Add failing projection tests for base → completed-command → device-binding → diagnostic precedence, open EntryContent updates, hidden nodes, and unavailable commands · internal/live/facility_test.go
- [x] **T034** [P] [US2] Add failing navigation tests for unavailable command rejection, hidden-target repair, nearest valid parent fallback, and unconditional Back/acknowledgement · internal/nav/nav_test.go
- [x] **T035** [P] [US2] Add failing stream tests for complete effective snapshots, monotonic revisions, observer convergence, and reconnect after a facility change · internal/player/public_stream_test.go
- [x] **T036** [P] [US2] Add failing group-move tests proving facility identity, current state, bindings, and pending actions are neither cloned nor retargeted · internal/control/service_test.go
- [x] **T037** [P] [US2] Add failing multi-terminal browser journeys for live labels, blocks, visibility, availability, group moves, and responsive open-entry repagination · tests/browser/facility-player-state.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — pure effective projection:**

- [x] **T038** [US2] Implement the deterministic facility evaluator and effective-tree projection with property conflict rejection, EntryContent composition, capability results, and detached outputs · internal/live/facility.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent server consumers (different files):**

- [x] **T039** [P] [US2] Enforce effective visibility and command availability and repair invalid presentation paths while preserving Back · internal/nav/nav.go
- [x] **T040** [P] [US2] Adapt unavailable commands and safe terminal effects into the additive public protobuf contract · internal/player/adapter.go
- [x] **T041** [P] [US2] Preserve the session-wide facility unchanged through terminal-group compare-and-replace mutations · internal/session/service.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — coordinator projection integration:**

- [x] **T042** [US2] Reproject active and suspended terminal runtimes from the one shared facility after commits, reactivation, authored updates, and group moves · internal/control/service.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — independent player presentation changes (different files):**

- [x] **T043** [P] [US2] Render authoritative unavailable commands, suppress their local activation, and reconcile changed open entries from newer snapshots · frontend/client/client.js
- [x] **T044** [P] [US2] Add accessible unavailable-command styling without changing visible menu order or hiding the item · frontend/client/client.css

**Checkpoint**: User Story 2 is independently functional and testable across terminals, groups, active pages, observers, and reconnects.

---

## Phase 5: User Story 3 - Author reusable devices and valid state graphs (Priority: P1)

**Goal**: The Overseer can create and validate devices, states, transitions, bindings, conditions, programs, and references without editing JSON, and can repair dependencies atomically.

**Independent Test**: Author a reactor graph and cross-terminal bindings, inspect its references, save and reopen it, rename its display label safely, then prove referenced identity deletion is blocked until one complete repair is applied.

### Tests

**Wave 1 — independent failing tests (different files):**

- [x] **T045** [P] [US3] Add failing dependency-index and authoring-validation tests for stable IDs, references, equality ownership, transition graphs, and repair candidates · internal/domain/facility_test.go
- [x] **T046** [P] [US3] Add failing revision-aware authoring tests for protected current values, new-entity initialization, graph revision, stale drafts, and atomic reference repair · internal/session/facility_test.go
- [x] **T047** [P] [US3] Add failing private request/result round-trip and desktop allowlist tests for facility authoring and dependency inspection · app_contract_test.go
- [x] **T048** [P] [US3] Add failing accessible browser journeys for complete device, transition, binding, condition, program, dependency, rename, delete, cancel, and repair authoring · tests/browser/facility-authoring.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — dependency and repair model:**

- [x] **T049** [US3] Implement deterministic dependency indexing, reference-impact reports, identity-change detection, and complete repair-candidate validation · internal/domain/facility.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — durable authoring owner:**

- [x] **T050** [US3] Implement revision-aware facility-authoring replacement that protects canonical current state, initializes new entities, validates all references, and writes repairs once · internal/session/facility.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — coordinator authoring integration:**

- [x] **T051** [US3] Serialize facility authoring with player actions, invalidate stale pending actions on graph revision, install the durable facility, and refresh effective runtime projections · internal/control/facility.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — private contract adapters:**

- [x] **T052** [US3] Add authoring and dependency request/result adapters with typed issues and exact revision presence · app_contract.go

**⟶ Wait for Wave 5 to finish, then:**

**Wave 6 — application methods:**

- [x] **T053** [US3] Add App facility-authoring and dependency-inspection methods with canonical event reconciliation · app.go

**⟶ Wait for Wave 6 to finish, then:**

**Wave 7 — desktop service boundary:**

- [x] **T054** [US3] Expose only the typed facility authoring and inspection methods through the registered desktop service · desktop_service.go

**⟶ Wait for Wave 7 to finish, then:**

**Wave 8 — generated desktop bindings:**

- [x] **T055** [US3] Regenerate Wails bindings and review the exact private method/model surface without manual edits · frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js

**⟶ Wait for Wave 8 to finish, then:**

**Wave 9 — independent Overseer client surfaces (different files):**

- [x] **T056** [P] [US3] Add normalized desktop API calls and revision-safe result handling for facility authoring and inspection · frontend/overseer/src/desktop-api.js
- [x] **T057** [P] [US3] Add labelled facility workspace, editors, dependency report, repair dialog, and accessible validation/status regions · frontend/overseer/src/index.html
- [x] **T058** [P] [US3] Add facility workspace layout, state graph rows, dependency impact, invalid-reference, and responsive dialog styles · frontend/overseer/src/overseer.css

**⟶ Wait for Wave 9 to finish, then:**

**Wave 10 — authoring interaction integration:**

- [x] **T059** [US3] Implement draft isolation, stable-ID generation, device/state/transition editors, command and EntryContent bindings, conditions, recovery programs, dependency repair, deletion guards, canonical saves, and cancellation · frontend/overseer/src/overseer.js

**Checkpoint**: User Story 3 is independently functional and testable through the Overseer UI with no manual session-data editing.

---

## Phase 6: User Story 4 - Run deterministic faults and recovery (Priority: P1)

**Goal**: Authored diagnostic conditions deterministically block selected capabilities, expose diagnostic paths, substitute records, drive visual effects, and recover only through explicit approved actions.

**Independent Test**: Activate a network-isolated condition, repeat its rendering before and after restart, exercise each typed effect, then recover via a transition, bounded holotape program, and private action without random or incidental mutation.

### Tests

**Wave 1 — independent failing tests (different files):**

- [x] **T060** [P] [US4] Add failing condition tests for categories, typed scopes/effects, recovery references, overlap rejection, and bounded holotape expansion · internal/domain/facility_test.go
- [x] **T061** [P] [US4] Add failing deterministic projection tests for capability blocks, diagnostic paths, record substitution, effect precedence, and repeatability · internal/live/facility_test.go
- [x] **T062** [P] [US4] Add failing coordinator tests for approved transition recovery, recovery-program expansion, private recovery, stale revisions, and permanent escape authority · internal/control/facility_test.go
- [x] **T063** [P] [US4] Add failing browser journeys for every standard fault, custom faults, damaged multipage records, unstable display, blocked capabilities, recovery, and escape · tests/browser/facility-diagnostics.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — deterministic condition semantics:**

- [x] **T064** [US4] Implement typed condition activation, recovery-reference validation, capability sets, conflict-free record effects, and finite recovery-program expansion · internal/domain/facility.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent persistence and projection changes (different files):**

- [x] **T065** [P] [US4] Apply transition condition effects and private/program recovery through the same one-write facility candidate · internal/session/facility.go
- [x] **T066** [P] [US4] Apply active condition overrides and safe presentation effects deterministically without mutating authored or canonical state · internal/live/facility.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — coordinator recovery integration:**

- [x] **T067** [US4] Enforce capability blocks at the server, preserve Back/acknowledgement/private recovery, and route approved or private recovery through the facility transaction · internal/control/facility.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — independent application and player consumers (different files):**

- [x] **T068** [P] [US4] Add typed private recovery routing and canonical session/coordination publication · app.go
- [x] **T069** [P] [US4] Render authoritative display-instability effects, cancel obsolete effects on newer revisions, and keep animation callbacks state-free · frontend/client/client.js
- [x] **T070** [P] [US4] Add bounded display-instability and diagnostic presentation styles that do not remove source content · frontend/client/client.css

**Checkpoint**: User Story 4 is independently functional and testable with deterministic diagnostics, explicit recovery, and an always-available escape.

---

## Phase 7: User Story 5 - Inspect, preview, and reset the facility (Priority: P2)

**Goal**: The Overseer can inspect dependencies, preview without side effects, reset one device, reset the whole facility, and recover an unusable presentation safely.

**Independent Test**: Inspect one device, preview every state and fault, close previews without mutation, reset the device, then reset the facility and verify exact scope, one revision per changed operation, and complete rollback on failure.

### Tests

**Wave 1 — independent failing tests (different files):**

- [x] **T071** [P] [US5] Add failing store tests for single-device reset scope, whole-facility reset, initial conditions, no-op behavior, stale revision, and storage rollback · internal/session/facility_test.go
- [x] **T072** [P] [US5] Add failing coordinator tests for detached inspection/preview, no events or revisions from preview, reset invalidation, and one publication on success · internal/control/facility_test.go
- [x] **T073** [P] [US5] Add failing private contract and App tests for inspect, preview, reset, recovery, typed failures, and canonical newer-revision acceptance · app_test.go
- [x] **T074** [P] [US5] Add failing browser journeys for dependency inspection, state/fault preview, reset confirmations, cancel, stale/failure handling, and private escape recovery · tests/browser/facility-authoring.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — detached preview projection:**

- [x] **T075** [US5] Implement side-effect-free candidate preview and effective terminal projection without installing state or emitting events · internal/live/facility.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — durable reset owner:**

- [x] **T076** [US5] Implement single-device and whole-facility reset through the shared candidate, expected revision, initial values, no-op detection, and one atomic replacement · internal/session/facility.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — coordinator operations:**

- [x] **T077** [US5] Implement dependency inspection, detached preview, coordinated reset, stale-pending invalidation, navigation repair, and private recovery escape · internal/control/facility.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — private contract adapters:**

- [x] **T078** [US5] Add inspect, preview, single-reset, whole-reset, and recovery adapters to the private contract boundary · app_contract.go

**⟶ Wait for Wave 5 to finish, then:**

**Wave 6 — application methods:**

- [x] **T079** [US5] Add App methods with explicit confirmation outcomes, typed failures, and durable session-state publication · app.go

**⟶ Wait for Wave 6 to finish, then:**

**Wave 7 — desktop service boundary:**

- [x] **T080** [US5] Expose the final typed inspect, preview, reset, and recovery methods through the registered desktop service · desktop_service.go

**⟶ Wait for Wave 7 to finish, then:**

**Wave 8 — generated desktop bindings:**

- [x] **T081** [US5] Regenerate the final Wails facility method and model bindings and update the allowlist contract · frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js

**⟶ Wait for Wave 8 to finish, then:**

**Wave 9 — independent Overseer operation clients (different files):**

- [x] **T082** [P] [US5] Add normalized inspect, preview, reset, and recovery calls with exact revision checks · frontend/overseer/src/desktop-api.js
- [x] **T083** [P] [US5] Implement dependency navigation, detached previews, confirmation dialogs, reset/recovery reconciliation, and focus restoration · frontend/overseer/src/overseer.js

**Checkpoint**: User Story 5 is independently functional and testable for safe inspection, preview, reset, and recovery operations.

---

## Phase 8: User Story 6 - Resume the same world after lifecycle changes (Priority: P2)

**Goal**: The last fully committed facility survives broadcast stop, application restart, self-update handoff, and session reload, while old pending requests cannot cross runtime contexts.

**Independent Test**: Commit device and condition changes, exercise every lifecycle boundary, and verify the same facility revision projects before interaction resumes; load a legacy session and prove no facility is invented.

### Tests

**Wave 1 — independent failing tests (different files):**

- [x] **T084** [P] [US6] Add failing reopen and fresh-process tests for persistent facility revisions, complete values, nil legacy defaults, and incomplete-data rejection · internal/session/service_test.go
- [x] **T085** [P] [US6] Add failing lifecycle tests for broadcast stop/start, session replacement, pending invalidation, shared-facility hydration, and reactivation from current state · internal/control/service_test.go
- [x] **T086** [P] [US6] Add failing App tests for new/open/copy/reload ordering, session epoch replacement, and update restart restoration before publication · app_test.go
- [x] **T087** [P] [US6] Add failing browser journeys for stop/start, reconnect, full restart fixture reload, self-update handoff simulation, and version-1 sessions without facility data · tests/browser/facility-lifecycle.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — session lifecycle authority:**

- [x] **T088** [US6] Install and expose the loaded canonical facility independently from process-local document revision while rejecting incomplete committed facility data · internal/session/service.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — coordinator lifecycle replacement:**

- [x] **T089** [US6] Add explicit facility install/replace/clear operations outside LiveBroadcast and invalidate pending requests and stale runtime projections on session replacement · internal/control/service.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — application lifecycle ordering:**

- [x] **T090** [US6] Order new/open/copy/reload completion so coordination receives the loaded facility before broadcast or player snapshots resume · app.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — root application composition:**

- [x] **T091** [US6] Wire the facility store/catalog and lifecycle installation seams through root application composition · main.go

**Checkpoint**: User Story 6 is independently functional and testable across broadcast, process, update, reconnect, and session compatibility lifecycles.

---

## Phase 9: User Story 7 - Trace facility actions and all player activity (Priority: P3)

**Goal**: The Overseer can reconstruct every player semantic request and authoritative outcome, especially state transitions, through correlated redacted retained logs that never become world state.

**Independent Test**: Exercise every public action kind plus approved, rejected, stale, duplicate, concurrent, failed, recovery, and reset facility flows; verify exact correlations and transitions, zero forbidden values, retained access, and unchanged gameplay when logging fails.

### Tests

**Wave 1 — independent failing observability tests (different files):**

- [x] **T092** [P] [US7] Add failing facility audit tests for request, decision, transition, failure, recovery, reset, revisions, sorted affected IDs, and exactly-once commit evidence · internal/control/facility_test.go
- [x] **T093** [P] [US7] Add a failing matrix proving every public semantic request and authoritative outcome emits correlated audit facts for accepted, rejected, replayed, duplicate, stale, unauthorized, canceled, and failed paths · internal/control/service_test.go
- [x] **T094** [P] [US7] Add failing application log tests for safe field mapping, state-transition evidence, redaction, correlation, and absence of raw errors or authored values · app_test.go
- [x] **T095** [P] [US7] Add failing retained-log tests for bounded retention, current/previous run access, sink failure isolation, and safe fallback warnings · internal/diagnostics/retained_log_test.go
- [x] **T096** [P] [US7] Add failing browser coverage proving the Overseer can reach current retained records and identify correlated player/facility activity · tests/browser/overseer-runtime-logs.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — complete semantic audit production:**

- [x] **T097** [US7] Extend closed audit values with facility event categories, state/revision transitions, safe sorted identities, reset scope, and failure outcomes · internal/control/facility.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — universal semantic audit emission:**

- [x] **T098** [US7] Emit request and authoritative-outcome audit facts for every player semantic action, role/controller transition, navigation, command, hacking interaction, replay, rejection, and conflict without duplicating committed transitions · internal/control/service.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent audit consumers and failure handling (different files):**

- [x] **T099** [P] [US7] Correlate connection, disconnection, recognition, and public request boundary records with safe logical-session and role fields · internal/player/server.go
- [x] **T100** [P] [US7] Route the expanded closed audit facts into retained logs with allowlisted fields, run identity, redaction, and no state-read path · app.go
- [x] **T101** [P] [US7] Keep retained-log write, rotation, and location failures isolated from gameplay while emitting a safe fallback warning when available · internal/diagnostics/retained_log.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — Overseer visibility:**

- [x] **T102** [US7] Surface retained-log availability warnings and preserve the existing open/reveal log workflow without exposing raw player or authored content in UI state · frontend/overseer/src/overseer.js

**Checkpoint**: User Story 7 and Constitution Principle VIII are independently testable with complete player-activity and state-transition evidence available to the Overseer.

---

## Phase 10: Polish and Cross-Cutting Validation

Finish documentation, generated-artifact review, compatibility evidence, simplification, and the single owner run of Success Criteria validation.

**Wave 1 — independent documentation and review updates (different files):**

- [x] **T103** [P] Document facility authority, three revisions, transaction order, projection precedence, lifecycle hydration, and audit ownership · ARCHITECTURE.md
- [x] **T104** [P] Document Overseer facility authoring, recovery, reset, preview, and retained player-activity log workflows · README.md
- [x] **T105** [P] Add an end-to-end validation record mapping SC-001 through SC-012 and Constitution Principle VIII to automated and conditional evidence · specs/035-facility-state-system/validation.md
- [x] **T106** [P] Extend the Wails surface guard for the reviewed facility methods and regenerated binding set · scripts/wails-bindings-check.sh

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — cross-cutting simplification review:**

- [x] **T107** Apply the Go quality and simplification reviews to facility changes, remove duplicate evaluators or state copies, and keep package boundaries and error semantics idiomatic · internal/domain/facility.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — required Go modernization and formatting:**

- [x] **T108** Run `go fix ./...`, review every generated modernization, retain only intentional edits, and run final formatting · Taskfile.yml

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent contract and static gates:**

- [x] **T109** [P] Run protobuf format/lint/generation/breaking checks and the Wails binding drift check, resolving only facility-related failures · Taskfile.yml
- [x] **T110** [P] Run frontend clean builds and focused facility browser suites under the `.nvmrc` Node runtime, resolving facility-related failures · frontend/package.json

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — single-owner Success Criteria suite:**

- [x] **T111** Run `task vet`, `task lint`, `task test`, and `task test:race`, then reconcile all SC-001–SC-012 and player-activity logging evidence without duplicating the earlier contract/browser suite runs · Taskfile.yml

**Checkpoint**: The complete facility system satisfies the specification, amended constitution, compatibility contract, generated-code gates, and documented Success Criteria evidence.

## Dependencies & Execution Order

- Phase 1 Setup precedes Phase 2 Foundational; Phase 2 blocks every user-story phase.
- P1 stories execute in dependency order: User Story 1 atomic actions → User Story 2 shared projection → User Story 3 authoring → User Story 4 diagnostics and recovery.
- P2 stories follow the complete P1 base: User Story 5 inspection/preview/reset → User Story 6 lifecycle restoration.
- P3 User Story 7 consumes every completed player and facility action path so its observability matrix can prove full coverage.
- Phase 10 Polish begins only after all seven story checkpoints.
- Setup Wave 1 joins before Foundational tests; Foundational tests join before protobuf sources, generation, the domain owner, then adapters and validation.
- User Story 1 tests join before the durable store, then control contracts, coordinator integration, and application/UI consumers.
- User Story 2 tests join before the pure projector, then server consumers, coordinator reprojection, and player rendering.
- User Story 3 tests join before dependency indexing, durable authoring, coordinator authoring, private boundary generation, and the Overseer UI.
- User Story 4 tests join before condition semantics, persistence/projection, coordinator recovery, and player effects.
- User Story 5 tests join before preview, reset persistence, coordinator operations, private bindings, and Overseer controls.
- User Story 6 tests join before session hydration, coordinator replacement, App ordering, and root composition.
- User Story 7 tests join before facility audit values, universal semantic audit emission, retained-log consumers, and Overseer visibility.
- Polish documentation runs independently, then simplification, Go modernization, parallel contract/frontend gates, and the final single-owner Success Criteria suite.

## Phase 11: Convergence

**Depends on:** all prior phases.

**Wave 1 — independent quantitative success-criteria coverage:**

- [x] T112 [P] Add a 100-consecutive-action test proving each approved multi-device world action persists all transitions through exactly one write and one facility revision, or leaves every affected device unchanged, per SC-001 (partial) · internal/session/facility_test.go
- [x] T113 [P] Add an explicit one-second bound to the five-terminal, three-group browser convergence journey for effective labels, EntryContent blocks, command availability, and visibility, per SC-003 (partial) · tests/browser/facility-player-state.spec.mjs
- [x] T114 [P] Add a 100-terminal group-move invariant test proving zero facility device, state, identity, and reference changes, per SC-004 (partial) · internal/control/service_test.go
- [x] T115 [P] Add a deterministic 100-replay matrix for every supported diagnostic condition category, comparing capability, content, navigation, authored-content, and persistent-state results, per SC-005 (partial) · internal/live/facility_test.go
- [x] T116 [P] Add a 1,000-attempt duplicate, stale, disjoint, and overlapping facility-action stress test proving at-most-once commit per expected state and no split facility state, per SC-011 (partial) · internal/control/facility_test.go

## Phase 12: Convergence

**Depends on:** all prior phases.

**Wave 1 — independent constitutional blockers:**

- [x] **T117** [P] **CRITICAL** Expand the portable version-1 demo into one coherent in-world narrative that exposes every behaviorally distinct session-driven capability, including all node and command modes, persistent command and EntryContent changes, controller and observer roles, hacking levels and mechanics, transitions and returns, facility bindings and device actions, diagnostics and effects, authored recovery paths, and Overseer preview/reset operations, with no player-visible license or hobby-project warning, per Constitution IX (missing) · sessions/demo.json
- [x] **T118** [P] **CRITICAL** Reclassify the completed Wails v2-to-v3 migration materials as clearly historical, remove active Wails v2 rollback authority from current documentation, preserve the historical beta.8 targets, and make the cutover guard enforce the accepted beta.15 runtime plus historical-label semantics, per Constitution VII (contradicts) · README.md, docs/wails-v3-migration-rollback.md, scripts/wails-v3-cutover-check.sh, internal/platform/assets_test.go
- [x] **T119** [P] **CRITICAL** Diagnose and correct the reproducible CRT replacement-generation fixture conflict (`204` expected, `409` received) and the full-suite terminal grouping/navigation failures while preserving authoritative generation and navigation behavior, then make `npm test --prefix tests/browser` pass without suppressing coverage, per Constitution Testing and Quality Gates and plan: Phase 6/8 (partial) · tests/browser/crt-rendering.spec.mjs, tests/browser/fixture-server/main.go, tests/browser/terminal-grouping.spec.mjs, tests/browser/terminal-navigation.spec.mjs, frontend/client/client.js

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — canonical demo terminal identity:**

- [x] **T120** **CRITICAL** Rework demo terminal groups so each group is one independent in-world installation and every member is explicitly an access level or local/remote view of that same terminal, preserve ordered same-terminal transitions, move independent machines to separate groups, and document those meanings for authors, per Constitution IX (contradicts) · sessions/demo.json, README.md

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — reviewable demo route inventory:**

- [x] **T121** **CRITICAL** Add a capability-to-path inventory for every demo behavior with its session asset, starting terminal and group, required role or access context, exact reachable menu route, state prerequisite, expected outcome, and return or recovery path, per Constitution IX (missing) · sessions/demo-capability-paths.md

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — executable demo acceptance evidence:**

- [x] **T122** **CRITICAL** Add automated acceptance coverage that validates the inventory against the demo, proves every listed path is reachable without JSON editing for applicable Overseer/controller/observer journeys, exercises all hacking levels and facility modes, verifies exact-one group membership and same-terminal group identity, proves version-1 load/save/reopen and compatible unknown-field preservation, detects accidental dead ends, and rejects out-of-world warnings, per Constitution IX and Constitution Testing and Quality Gates (missing) · internal/platform/assets_test.go, tests/browser/state-changing-command-authoring.spec.mjs, tests/browser/demo-session.spec.mjs

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — constitutional reconciliation and final evidence:**

- [x] **T123** **CRITICAL** Reconcile the feature Constitution Check, status, and validation record with Principles VIII and IX; run and record only actual passing demo, complete browser, cutover, version-1 compatibility, Go, frontend, build, and supported-host package gates; and leave unavailable conditional checks explicitly `NOT RUN`, per Constitution Governance (partial) · specs/035-facility-state-system/plan.md, specs/035-facility-state-system/spec.md, specs/035-facility-state-system/validation.md, specs/035-facility-state-system/.spec-context.json
