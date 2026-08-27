# Tasks: Application Self-Update

**Input**: [spec.md](./spec.md), [plan.md](./plan.md), [research.md](./research.md),
[data-model.md](./data-model.md), and [contracts/](./contracts/)

**Tests**: Required by the specification and Constitution 8.1.0. Write the focused tests in each
story phase before the corresponding implementation and preserve the project `t.Cleanup`,
`t.Context()`, Testify, race, generated-contract, and browser conventions.

## Phase 1: Setup

**Purpose**: Establish the shared schema and transport-independent vocabulary used by every story.

**Wave 1 — independent (different files):**

- [x] **T001** [P] Define the private update states, failure stages, snapshots, offer/restart decisions, and command result exactly as approved · `proto/fallout/terminal/private/v1/update.proto`
- [x] **T002** [P] Define immutable update states, candidates, progress, failures, prepared-unit metadata, and recovery records without Wails dependencies · `internal/update/model.go`

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T003** Regenerate the private Go protobuf graph, advance the reviewed schema revision and compatibility baseline, and verify no private ECMAScript contract leaks to the player client · `internal/gen/fallout/terminal/private/v1/update.pb.go`, `proto/schema-revision.txt`, `proto/compatibility-baseline.binpb`

---

## Phase 2: Foundational

**Purpose**: Build the application-owned state and bridge seams that block all three user stories.

### Tests

**Wave 1 — independent (different files):**

- [x] **T004** [P] Add failing table/race tests for development disablement, one-shot check arming, snapshot revisions, stale attempt rejection, and operation serialization · `internal/update/manager_test.go`
- [x] **T005** [P] Add failing protobuf/native round-trip and desktop-service forwarding tests for every update state, decision, optional field, and sanitized failure · `app_contract_test.go`, `desktop_service_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then — Wave 2 independent (different files):**

- [x] **T006** [P] Implement the framework-independent manager skeleton, dependency interfaces, one-shot arming, immutable snapshots, revisions, and stale/concurrent decision guards · `internal/update/manager.go`
- [x] **T007** [P] Implement private protobuf/native adapters and native update DTOs with no provider, path, digest, helper, credential, or user-content fields · `app_contract.go`, `app.go`

**⟶ Wait for Wave 2 to finish, then:**

- [x] **T008** Inject the update manager into application dependencies and expose only `GetApplicationUpdateStatus`, `ResolveApplicationUpdateOffer`, and `ResolveApplicationUpdateRestart` through the existing desktop service · `app.go`, `desktop_service.go`

**⟶ Wait for T008 to finish, then — Wave 4 independent (different files):**

- [x] **T009** [P] Register `application-update-status`, add the root Wails updater/host adapter seam, and preserve lifecycle-first then desktop-service registration · `wails_host.go`, `wails_updater.go`
- [x] **T010** [P] Extend static private-surface contracts for exactly 38 desktop methods and seven named events · `scripts/wails-bindings-check.sh`, `tests/browser/desktop-api.spec.mjs`

**⟶ Wait for Wave 4 to finish, then:**

- [x] **T011** Regenerate Wails bindings cleanly and verify generated files are the only binding changes · `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js`, `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/models.js`

**Checkpoint**: Shared update state, generated contracts, and the narrow private bridge compile with
no network check, UI, or replacement behavior yet.

---

## Phase 3: User Story 1 — Discover an Update at Startup (Priority: P1) 🎯 MVP

**Goal**: Start exactly one nonblocking packaged-release check only after the Overseer can receive
status, offer one complete eligible release, and defer it without downloading.

**Independent Test**: Launch a packaged older version against deterministic release metadata,
confirm local session controls remain usable, see one accurate offer, and verify defer performs zero
artifact downloads and suppresses the release for the run.

### Tests

**Wave 1 — independent (different files):**

- [x] **T012** [P] Add failing provider tests for strict stable/prerelease ordering, exact five-asset completeness, GitHub SHA-256 parsing, exact-one target matching, drafts, malformed metadata, and cancellation · `wails_updater_test.go`
- [x] **T013** [P] Add failing lifecycle tests proving the event-first getter arms one bounded asynchronous check after readiness while development and update failure never affect local startup · `app_test.go`, `wails_host_test.go`
- [x] **T014** [P] Add failing facade and Playwright discovery tests for listener-before-getter ordering, newer-revision wins, one offer, current/development silence, versions/notes, defer, focus, Escape, and enabled local controls · `tests/browser/desktop-api.spec.mjs`, `tests/browser/application-update.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then — Wave 2 independent (different files):**

- [x] **T015** [P] Implement the public GitHub provider wrapper over pinned Wails discovery/download, greatest eligible channel policy, exact five-asset validation, digest injection, and safe error categories · `wails_updater.go`
- [x] **T016** [P] Implement manager discovery, bounded cancellation, candidate/defer state, launch-scoped suppression, and check diagnostics · `internal/update/manager.go`
- [x] **T017** [P] Add normalized frozen update snapshots, revision reconciliation, event-first getter, exact-once disposal, and offer decision facade methods · `frontend/overseer/src/desktop-api.js`

**⟶ Wait for Wave 2 to finish, then — Wave 3 independent (different files):**

- [x] **T018** [P] Wire the packaged release/version gate, recovery-free initial snapshot, asynchronous getter handshake, and safe update event publication without changing `startupError` · `main.go`, `app.go`, `wails_host.go`
- [x] **T019** [P] Add the persistent global update status and accessible offer dialog with installed/available versions, bounded release notes, safe default focus, and nonblocking defer behavior · `frontend/overseer/src/index.html`, `frontend/overseer/src/overseer.js`, `frontend/overseer/src/overseer.css`
- [x] **T020** [P] Extend the deterministic Wails binding fixture with revisioned discovery snapshots, deferred getter/check control, event emission, decisions, and download-call accounting · `tests/browser/fixtures/desktop-bindings.js`

**⟶ Wait for Wave 3 to finish, then:**

- [x] **T021** Make the focused Go and browser discovery tests pass and confirm the player client has no update method, event, import, or route · `wails_updater_test.go`, `app_test.go`, `tests/browser/desktop-api.spec.mjs`, `tests/browser/application-update.spec.mjs`

**Checkpoint**: User Story 1 is independently functional—discovery is one-shot, nonblocking,
complete-release-only, and consent precedes all download work.

---

## Phase 4: User Story 2 — Apply a Trusted Update (Priority: P1)

**Goal**: Download the accepted matching archive, verify it, stage the complete compatible package,
ask separately for restart, then replace/relaunch with backup recovery and unchanged user data.

**Independent Test**: Accept a deterministic valid update on each modeled target, observe ordered
progress, postpone and reopen restart, approve restart, and verify clean shutdown, replacement,
relaunch version, and unchanged representative user-owned data.

### Tests

**Wave 1 — independent (different files):**

- [x] **T022** [P] Add failing five-target tests for artifact-manifest schema v2, canonical version equality, deterministic bytes, target/package shape, and matching-native release inspection · `internal/buildtool/archive_test.go`, `internal/buildtool/releasecheck_test.go`
- [x] **T023** [P] Add failing staging/helper tests for Windows/Linux directory selection, macOS bundle selection, same-volume sibling staging, sync/cleanup, unsafe paths, bounded parent wait, backup, promotion, relaunch, and restore · `internal/update/staging_test.go`, `internal/update/helper_test.go`
- [x] **T024** [P] Add failing manager and browser tests for accept-only download, progress ordering, digest/manifest gates, ready state, postpone retention, restart exactly once, and continued UI use · `internal/update/manager_test.go`, `tests/browser/application-update.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then — Wave 2 independent (different files):**

- [x] **T025** [P] Advance the artifact manifest to schema v2 with canonical version and make existing matching-native inspection reject schema, version, target, or inventory disagreement · `internal/buildtool/archive.go`, `internal/buildtool/package.go`, `internal/buildtool/releasecheck.go`
- [x] **T026** [P] Implement extracted-root validation, per-target replacement-unit selection, safe installed-unit resolution, and same-volume sibling staging · `internal/update/staging.go`, `internal/platform/paths.go`
- [x] **T027** [P] Implement copied helper mode, private environment protocol, bounded parent wait, backup/promote/relaunch/restore algorithm, and platform process adapters · `internal/update/helper.go`, `internal/update/helper_unix.go`, `internal/update/helper_windows.go`

**⟶ Wait for Wave 2 to finish, then — Wave 3 independent (different files):**

- [x] **T028** [P] Implement accept-triggered Wails preparation, progress translation, manifest/stage validation, ready retention, postpone, restart handoff, and apply-aware shutdown cleanup · `internal/update/manager.go`
- [x] **T029** [P] Dispatch helper mode before normal CLI/Wails startup, compose concrete updater/stager/helper dependencies, and route restart approval through normal host quit · `main.go`, `wails_updater.go`, `wails_host.go`
- [x] **T030** [P] Render nonmodal download/verify/stage progress and the separate accessible restart dialog with postpone/reopen and duplicate-action protection · `frontend/overseer/src/index.html`, `frontend/overseer/src/overseer.js`, `frontend/overseer/src/overseer.css`
- [x] **T031** [P] Extend browser fixtures with controlled progress, preparation failure, staged retention, and restart handoff calls · `tests/browser/fixtures/desktop-bindings.js`

**⟶ Wait for Wave 3 to finish, then:**

- [x] **T032** Make the focused build-tool, helper, manager, lifecycle, and Playwright apply tests pass under the race detector where applicable · `internal/buildtool/*_test.go`, `internal/update/*_test.go`, `app_test.go`, `tests/browser/application-update.spec.mjs`

**Checkpoint**: User Story 2 is independently functional—verified complete packages stage only
after acceptance, restart remains a separate choice, and apply uses ordered shutdown plus recovery.

---

## Phase 5: User Story 3 — Keep Working When Updating Is Unavailable (Priority: P2)

**Goal**: Preserve the current installation and local operation across discovery, transfer,
verification, staging, replacement, and relaunch failures, with safe actionable diagnostics.

**Independent Test**: Inject every failure stage, including post-shutdown promotion/relaunch failure,
and verify a usable current or restored installation, no user-data change, one safe stage/action, and
no sensitive value in UI, logs, events, journal, or process arguments.

### Tests

**Wave 1 — independent (different files):**

- [x] **T033** [P] Add failing failure/recovery tests for timeouts, cancellation, insufficient space, read-only install, stale applying journals, backup cleanup, restored relaunch, and user-data isolation · `internal/update/manager_test.go`, `internal/update/helper_test.go`
- [x] **T034** [P] Add failing provider/release security tests for missing or malformed digests, ambiguous assets, incomplete five-target releases, redacted URLs/tokens/paths, and exact five-archive publication invariants · `wails_updater_test.go`, `internal/platform/portable_release_test.go`, `scripts/secret-leak-check.sh`
- [x] **T035** [P] Add failing browser journeys for offline/check failure, interrupted download, verify/stage/apply recovery status, actionable messages, stale-event suppression, and uninterrupted local controls · `tests/browser/application-update.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then — Wave 2 independent (different files):**

- [x] **T036** [P] Implement the atomic non-sensitive recovery journal, next-launch applied/failed/stale consumption, attempt-owned cleanup, and restored-application diagnostics · `internal/update/recovery.go`, `internal/platform/paths.go`, `main.go`
- [x] **T037** [P] Implement stable stage/category sanitization and recovery actions across provider, manager, helper, events, logs, and command results · `wails_updater.go`, `internal/update/manager.go`, `internal/update/helper.go`
- [x] **T038** [P] Render durable nonfatal failure status/actions without changing startup readiness, hiding local controls, stacking dialogs, or persisting provider details in browser state · `frontend/overseer/src/overseer.js`, `frontend/overseer/src/overseer.css`

**⟶ Wait for Wave 2 to finish, then:**

- [x] **T039** Make the complete deterministic failure matrix, secret-leak scan, portable-release contracts, and Playwright recovery journeys pass · `internal/update/*_test.go`, `wails_updater_test.go`, `internal/platform/portable_release_test.go`, `tests/browser/application-update.spec.mjs`

**Checkpoint**: User Story 3 is independently functional—external update infrastructure cannot
break startup/local use, and failed apply restores or retains the last working application.

---

## Phase 6: Polish & Cross-Cutting Validation

**Purpose**: Finish generated integrity, documentation, full quality gates, and Success Criteria
evidence after all stories work independently.

**Wave 1 — independent (different files):**

- [x] **T040** [P] Update self-update, five-asset digest, writable portable installation, postpone, recovery, and forward-fix guidance · `README.md`, `docs/platform-packaging.md`
- [x] **T041** [P] Add static accessibility and privilege-separation assertions for update dialogs/live regions and the absence of frontend GitHub access · `internal/platform/assets_test.go`
- [x] **T042** [P] Update reviewed binding/protobuf digests and ensure regeneration leaves no unexplained working-tree drift · `scripts/wails-v3-contract-check.sh`, `scripts/wails-bindings-check.sh`, `proto/schema-revision.txt`, `proto/compatibility-baseline.binpb`

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T043** Run formatting, vet, pinned lint, race tests, protobuf format/lint/generation/breaking gates, Wails binding checks, frontend clean builds, browser tests, startup contracts, and secret scanning against SC-001–SC-010 · `Taskfile.yml`, `.github/workflows/wails-cross-platform.yml`

**⟶ Wait for T043 to finish, then:**

- [x] **T044** Record current-host Darwin ARM64 package evidence and one real Darwin ARM64 tagged prerelease update/relaunch journey, marking unavailable spare-tag execution honestly as `NOT RUN`; retain automated exact-five-archive publication coverage without requiring Windows/Linux runtime journeys · `specs/024-application-self-update/validation.md`

---

## Dependencies & Execution Order

### Phase dependencies

1. **Setup** supplies the private schema and shared model.
2. **Foundational** depends on Setup and blocks every story.
3. **User Story 1** depends on Foundational and is the MVP discovery/consent slice.
4. **User Story 2** depends on the User Story 1 candidate/bridge path and adds preparation/apply.
5. **User Story 3** depends on the complete User Story 2 lifecycle and closes every failure path.
6. **Polish** depends on all stories and owns the single full Success Criteria suite run.

### Wave joins

- Setup: `T001,T002` → `T003`.
- Foundational: `T004,T005` → `T006,T007` → `T008` → `T009,T010` → `T011`.
- User Story 1: `T012,T013,T014` → `T015,T016,T017` → `T018,T019,T020` → `T021`.
- User Story 2: `T022,T023,T024` → `T025,T026,T027` → `T028,T029,T030,T031` → `T032`.
- User Story 3: `T033,T034,T035` → `T036,T037,T038` → `T039`.
- Polish: `T040,T041,T042` → `T043` → `T044`.

### Parallel opportunities

Tasks marked `[P]` touch different files and have no incomplete dependency within their wave. The
test-first wave in each story can proceed independently, followed by the independent backend,
frontend, packaging, or fixture owners shown in that story's implementation wave. Join before any
integration or validation task so generated files, shared state, and suite results keep a single
owner.
