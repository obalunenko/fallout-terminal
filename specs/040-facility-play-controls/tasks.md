# Tasks: Facility Play Controls

**Input**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/private-desktop.md`,
`contracts/overseer-ui.md`

**Scale note**: About 30 files in six areas: protobuf, domain, session, control, the App bridge, and
the Overseer, plus demo data and 14 browser journeys. Watch the three allowlists that drop unknown
values without an error: the desktop method count (44 → 47), the facility audit actions and events
in `app.go`, and the failure and reason sets in `desktop-api.js`. Seven existing journeys use UI that
this feature removes. They fail between the Phase 2 checkpoint and their story phase.

**Structure**: The six stories share the control package, the App bridge, `desktop-api.js`, the
fixture server, and the single-file Overseer (`overseer.js`, `index.html`, `overseer.css`). Every
file has one owner phase, so all implementation is built in Phase 2, in story order and tagged by
story. Each story phase then owns its acceptance journey and the existing journeys its behavior
changes. The plan names two new journey files. They are split into six here, one per story, so no
journey file belongs to two phases.

- Go work is test-first: each test file is written one wave before its source file and fails until
  that source lands.
- The Go build (`go build ./...`) compiles at every wave join. Tests compile from the Wave 5 join.
- Overseer waves (Wave 8 to Wave 14) take one task each, because each touches all three Overseer
  files. The Overseer stays runnable at every join.
- No browser journey runs before the Phase 2 checkpoint.
- A story journey that finds a defect in a Phase 2 file reports it back. The fix lands in that file
  through the main agent, never inside a parallel story worker.

**Format**: `- [ ] **T###** [P?] [US#] Description · file`

---

## Phase 1: Setup

**Wave 1 — single task:**

- [x] **T001** Add the journey helpers shared by the six story specs: open the Overseer fixture on a
  demo copy, select a device in «ОБЪЕКТЫ», locate the play card parts by their contract IDs, read
  `#facilityPlayStatus` text and `data-state`, read the feed entries (`data-kind`, `data-author`,
  `data-reason`, text), set device states and fault positions through the fixture, and open a
  connected player terminal (`contracts/overseer-ui.md`) · tests/browser/helpers/facility-play.mjs

---

## Phase 2: Foundational (blocks every story)

**Files**:

- **Contract**: `proto/fallout/terminal/private/v1/desktop.proto`, `coordination.proto`, `internal/gen/fallout/terminal/private/v1/`, `proto/schema-revision.txt`
- **Domain**: `internal/domain/facility.go`, `model.go`, `facility_test.go`, `model_test.go`
- **Session**: `internal/session/facility.go`, `facility_test.go`
- **Control**: `internal/control/feed.go` (new), `feed_test.go` (new), `facility.go`, `facility_test.go`, `service.go`, `service_test.go`, `scene.go`, `scene_test.go`, `authoring.go`, `authoring_test.go`
- **Bridge**: `main.go`, `app.go`, `app_test.go`, `app_contract.go`, `app_contract_test.go`, `desktop_service.go`
- **Bindings and scripts**: `frontend/overseer/bindings/`, `scripts/wails-bindings-check.sh`, `internal/platform/assets_test.go` (schema pins, updated at the Wave 5 join)
- **Fixtures and API**: `tests/browser/fixture-server/main.go`, `tests/browser/fixtures/desktop-bindings.js`, `frontend/overseer/src/desktop-api.js`, `tests/browser/desktop-api.spec.mjs`
- **Overseer**: `frontend/overseer/src/overseer.js`, `index.html`, `overseer.css`
- **Demo**: `sessions/demo.json`, `sessions/demo-capability-paths.md`, `tests/browser/demo-session.spec.mjs`

**Wave 1 — independent (different files):**

- [x] **T002** [P] Add `ApplyFacilityDeviceTransitionRequest`, `ForceFacilityDeviceStateRequest`,
  and `SetFacilityConditionActiveRequest` next to `ResetFacilityDeviceRequest`. Add the
  `FacilityChangeRefusalReason` enum (zero value `UNSPECIFIED`, `FAILED = 7`), the seven
  `FacilityChange*` variant messages, `FacilityChange` with its `oneof change`, and
  `CoordinationState.facility_changes = 11`, with the field numbers from
  `contracts/private-desktop.md`. Run `task proto:generate`, `task proto:lint`, and
  `task proto:breaking` against the existing baseline · proto/fallout/terminal/private/v1/desktop.proto,
  proto/fallout/terminal/private/v1/coordination.proto, internal/gen/fallout/terminal/private/v1/,
  proto/schema-revision.txt
- [x] **T003** [P] Add `domain.FacilityChange` with one pointer per variant and the refusal reason
  type with its JSON values (`declined` … `failed`, `unspecified`). Add
  `ProcessRuntime.FacilityChanges map[string][]FacilityChange`, `ProcessRuntime.FacilityChangeSequence`,
  and `MasterCoordinationState.FacilityChanges` (`json:"facilityChanges,omitempty"`). Copy them deeply
  in `cloneProcessRuntime` and `CloneMasterCoordinationState`. Test the deep copies and the JSON keys
  of every variant (`sceneReset`, `playerCommand`, `playerRefusal`) (FR-012, R6) ·
  internal/domain/facility.go, internal/domain/model.go, internal/domain/facility_test.go,
  internal/domain/model_test.go
- [x] **T004** [P] Add the state `{ "id": "critical-overheat", "name": "Критический перегрев" }` to
  `reactor-main` with no incoming transition, and the `b_reactor_status` text variant
  «РЕАКТОР: КРИТИЧЕСКИЙ ПЕРЕГРЕВ» on `t_demo1`. List the six play paths from `data-model.md` (Demo
  data additions) in the inventory, and replace «СБРОСИТЬ УСТРОЙСТВО», the always-open guide, the
  rename field, and «РЕДАКТИРОВАТЬ ГРАФ» with the new copy. Update the inventory and copy
  assertions in the demo journey. The file stays session JSON version 1 (FR-026, FR-027, SC-008, R12)
  · sessions/demo.json, sessions/demo-capability-paths.md, tests/browser/demo-session.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — tests first, independent (different files):**

- [x] **T005** [P] Write table tests for `Service.ApplyFacilityDeviceTransition`,
  `ForceFacilityDeviceState`, and `SetFacilityConditionActive`. Cover success with the exact
  affected IDs, stale revision, missing device, transition, state, and condition, the device not in
  the source state (`invalid-transition`), the first unmet precondition in authored order with its
  device as the entity, conflicting effects, the no-op cases, a storage failure that rolls back the
  candidate, the bundled demo guard, no write to `commandStates`, `initialActive` unchanged, a
  terminal-scoped fault accepted, and a recovery list without `privateOverseerAction` ignored
  (FR-003, FR-004, FR-007, FR-009, FR-010, FR-026) · internal/session/facility_test.go
- [x] **T006** [P] Write the feed tests: at most five entries per device with the oldest dropped,
  newest first, sequence numbers strictly increasing across the run, entries of one commit sharing
  one `Config.Now` time, the snapshot flattened newest first by sequence, pruning devices missing
  from the facility, the refusal reason for each facility failure code (`data-model.md`), and
  entries carrying IDs only (FR-012, FR-016, R6, R7) · internal/control/feed_test.go
- [x] **T007** [P] Write the control tests for the three new operations and the device reset
  changes. Cover the revision check, durable-result validation, one audit record per call with the
  actions `transition`, `force`, and `fault` and the facts `TransitionID`, `TargetStateID`, and
  `TargetActive`, and reprojection of broadcast terminals. Cover the withdrawal of a pending request
  with its `command.request_outcome` record (`superseded`) and `WITHDRAWN` entries, including on
  device reset. Cover one feed entry per changed call, none on unchanged or rejected calls, a
  fault-only reset that succeeds with a `reset` entry, and a scan of every log record for
  passwords, credentials, and hack content (FR-004 to FR-006, FR-011, FR-015, FR-016, SC-005) ·
  internal/control/facility_test.go
- [x] **T008** [P] Write the service tests for the feed. An approved command applied adds one
  `playerCommand` entry per moved device. A declined, precondition-failed, transition-unavailable,
  fault-blocked, stale, or failed request adds `playerRefusal` entries with the matching reason for
  each device in the command's authored action. A `duplicate` decision, hack, entry-view, and
  terminal-transition requests add none. Every path that appends publishes a master snapshot that
  carries the feed. `ReplaceFacility` clears the feed and keeps the sequence. `Config.Now` defaults
  to `time.Now` (FR-012 to FR-014, R7) · internal/control/service_test.go
- [x] **T009** [P] Write the scene reset tests: one `sceneReset` entry for each device whose state or
  device-scoped faults changed, none for unchanged devices, and `WITHDRAWN` entries after the
  existing `superseded` record (FR-006, FR-012) · internal/control/scene_test.go
- [x] **T010** [P] Write the authoring tests: a save that cannot rebase a pending request adds
  `playerRefusal` entries with `STALE`, and a save that deletes a device drops that device's feed
  (FR-013, data-model Rules) · internal/control/authoring_test.go
- [x] **T011** [P] Write the App tests for the three methods: blank or padded IDs rejected as
  `invalid-configuration` (entity `request`) before the coordinator runs, the exact-result
  validators, `persistence-failed` on a result that does not verify, and `stale-revision` results
  that carry the canonical session for all four play operations, filled from the active snapshot
  when control rejected first. Add a fault-only `ResetFacilityDevice` through the App that now
  succeeds (R9). Check that each new audit event, action, and outcome reaches the log and is not
  dropped or recorded as `invalid` (FR-004, FR-011, FR-015) · app_test.go
- [x] **T012** [P] Raise the pinned method count and forwarded count from 44 to 47 and add the three
  names to the pinned list. Add round trips for the three request converters and a
  `CoordinationState` round trip that keeps every feed variant, reason, and field. Add descriptor
  checks for the new field numbers (FR-012, R2, R6) · app_contract_test.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent (different packages):**

- [x] **T013** [P] Add `FacilityDeviceTransitionRequest`, `FacilityDeviceForceRequest`, and
  `FacilityConditionActiveRequest`, and the three `Service` methods. Each takes the `resetFacility`
  locks (`commandMu`, `documentMu`, `mu`), refuses the bundled demo, checks the revision, bumps
  `Facility.Revision` once, and commits through `commitFacilityCandidateLocked`. The card transition
  resolves one `{DeviceID, TransitionID}` through `resolveWorldTransitions` and writes no
  `CommandStates`. T005 passes (FR-003, FR-007 to FR-010, R1, R3) · internal/session/facility.go
- [x] **T014** [P] Create the feed helpers: `facilityChangeLimit = 5`, append with the bound and
  the run-wide sequence, one builder per variant, the failure-code-to-reason map, the newest-first
  flattening for `masterSnapshot`, and the prune of missing devices. T006 passes (FR-012, FR-013,
  FR-016, R6, R7) · internal/control/feed.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent (different files; the control package compiles at the join):**

Seams already in the tree, shared by the four tasks. Use them as they are:

- `Config.Now` and `service.now()` (defaults to `time.Now`). Read the clock once per commit,
  `at := service.now()`, and pass that `at` to every `appendFacilityChanges` and
  `invalidatePendingFacilityAction` call of that commit.
- `service.invalidatePendingFacilityAction(runtime, at, reason)` rejects the pending request and
  appends its refusal entries. Call it after the operation's own entries and after installing the
  resulting facility. It writes no audit record: the caller writes the `superseded` outcome
  record (`pendingRequestOutcomeAudit`). Callers pass `WITHDRAWN`, except the authoring path,
  which passes `STALE`.
- `feed.go` (T014) provides `appendFacilityChanges`, the per-variant builders,
  `facilityChangeSnapshot`, `pruneFacilityChanges`, and `cloneFacilityChangeFeed`.

- [x] **T015** [P] Add `Config.Now func() time.Time`, defaulting to `time.Now`. Make
  `ReplaceFacility` clear `FacilityChanges` and keep the sequence. Copy the feed deeply in
  `cloneProcessRuntime` (`cloneFacilityChangeFeed`), so a rejected commit leaks no entry. Project
  the feed in `masterSnapshot`. Append `playerCommand` entries when an approved command applies, and
  `playerRefusal` entries on decline, failure, capability block (the `facilityCapabilityBlocked`
  path), and stale approval. Make the paths that today only cache a rejection publish the snapshot.
  T008 passes (FR-012 to FR-014, R6, R7) · internal/control/service.go
- [x] **T016** [P] Add the audit actions `transition`, `force`, and `fault`, the facts
  `TransitionID`, `TargetStateID`, and `TargetActive`, the `FacilityPlayStore` interface, and the
  three operations, each in one `commit` closure that follows `ResetFacilityDevice`. Give
  `invalidatePendingFacilityAction` the `pendingRequestOutcomeAudit(…, "superseded", …)` record and
  refusal entries with a reason the caller passes: `WITHDRAWN` for play changes, device reset, and
  scene reset, `STALE` for the authoring path (T010, T018). Append the `reset` entry on a changed
  reset, including a fault-only one. T007 passes (FR-004 to FR-006, FR-011, FR-015, R8) ·
  internal/control/facility.go
- [x] **T017** [P] Append one `sceneReset` entry per changed device and the `WITHDRAWN` entries in
  the scene reset commit. T009 passes (FR-006, FR-012) · internal/control/scene.go
- [x] **T018** [P] Append `STALE` refusal entries when an authoring save cannot rebase a pending
  request, and prune the feed of deleted devices after an accepted save. T010 passes (FR-013) ·
  internal/control/authoring.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — independent (different files; the root package compiles at the join):**

- [x] **T019** [P] Adapt `sessionCommandStateStore` to `controlservice.FacilityPlayStore` with three
  methods that convert to the session request types, and add the
  `var _ controlservice.FacilityPlayStore` assertion · main.go
- [x] **T020** [P] Add `FacilityDeviceTransitionPayload`, `FacilityDeviceForcePayload`, and
  `FacilityConditionActivePayload`, one coordinator seam per method, and the three `App` methods.
  Each holds `coordinationCommandMu`, round-trips through proto, and checks trimmed IDs. Add
  validators per `contracts/private-desktop.md`: changed is required for a transition, and
  `FacilityFailureRejected` is allowed. Fix `validFacilityResetResultForApp` for fault-only resets
  (R9). Fill the stale session from the active snapshot. Extend `facilityAuditEvent`,
  `facilityAuditOutcome`, and `knownFacilityAuditAction`. Publish `session-state`, then
  `coordination-state`. T011 passes (FR-004, FR-011, FR-015) · app.go
- [x] **T021** [P] Add `facilityDeviceTransitionRequestToPrivate`/`…FromPrivate`,
  `facilityDeviceForceRequestToPrivate`/`…FromPrivate`, and
  `facilityConditionActiveRequestToPrivate`/`…FromPrivate`, and convert `FacilityChange` in
  `coordinationStateToPrivate` and `coordinationStateFromPrivate`. T012 passes (FR-012, R6) ·
  app_contract.go
- [x] **T022** [P] Add the three one-line forwarding methods · desktop_service.go

**⟶ Wait for Wave 5 to finish, then:**

**Wave 6 — independent (different files):**

- [x] **T023** [P] Regenerate the Wails bindings. In the check script, add the three names to the
  method list, set the count and the final message to 47, and add `assert_method_signature` for
  `ApplyFacilityDeviceTransition` (`$models.FacilityDeviceTransitionPayload`),
  `ForceFacilityDeviceState` (`$models.FacilityDeviceForcePayload`), and
  `SetFacilityConditionActive` (`$models.FacilityConditionActivePayload`). `task bindings:check`
  passes with 47 methods and seven events · frontend/overseer/bindings/, scripts/wails-bindings-check.sh
- [x] **T024** [P] Extend both fixture paths. The facility-authoring fake store gets
  `/__fixture/facility-authoring/apply-transition`, `/force-state`, and `/set-condition-active`. It
  enforces source state and preconditions, returns the session on stale, accepts a fault-only
  reset, and adds contract-shaped `facilityChanges` to its `coordination-state`, including for its
  player approve and decline paths. `facilityPlayerState`, the store behind the fixture's real
  `control.Service`, implements `control.FacilityPlayStore`, so player journeys get the real feed ·
  tests/browser/fixture-server/main.go
- [x] **T025** [P] Export `ApplyFacilityDeviceTransition`, `ForceFacilityDeviceState`, and
  `SetFacilityConditionActive` through `facilityOperationFixture` · tests/browser/fixtures/desktop-bindings.js
- [x] **T026** [P] Write the desktop API cases first: the three normalizers (`active: value.active
  === true`), the validator branches `device-transition`, `device-force`, `condition-active`, and
  the R9 `device-reset` branch, a failure with `session` accepted only for `stale-revision` at the
  matching revision, and feed normalization (frozen entries, dropped entries with zero or two
  variants, a blank `deviceId`, or a non-safe-integer number, and an unknown reason mapped to
  `unspecified`) (FR-004, FR-011, FR-012) · tests/browser/desktop-api.spec.mjs

**⟶ Wait for Wave 6 to finish, then:**

**Wave 7 — single task:**

- [x] **T027** Add `APP_METHODS.applyFacilityDeviceTransition`, `.forceFacilityDeviceState`, and
  `.setFacilityConditionActive`, the public methods through `facilityMutationCommand` (operations
  `device-transition`, `device-force`, `condition-active`), the three normalizers,
  `FACILITY_CHANGE_REFUSAL_REASONS`, the new and R9 branches of `validateFacilityMutationResult`, and
  feed normalization on the `coordination-state` callback. T026 passes (FR-004, FR-011, FR-012) ·
  frontend/overseer/src/desktop-api.js

**⟶ Wait for Wave 7 to finish, then:**

**Wave 8 — single task:**

- [x] **T028** [US2] Reorder the detail area per `contracts/overseer-ui.md`. Replace
  `#facilityDeviceSummary` and `#facilitySelectionHeading` with `#facilityDeviceHeader`
  (name, kind, linked terminals, current and starting state, active faults, the technical line, and
  `#btnClearFacilitySelection`). Add the empty `#facilityPlayCard` section with
  «ВО ВРЕМЯ ИГРЫ» and `#facilityPlayStatus`, then the `#facilityDeviceActions` container. Clear the
  selection when the selected device disappears (FR-017, FR-018) · frontend/overseer/src/index.html,
  frontend/overseer/src/overseer.js, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 8 to finish, then:**

**Wave 9 — single task:**

- [x] **T029** [US2] Drop the hard-coded `open` on `#facilityStoryGuide`. Add
  `#facilityStoryGuideSummary` with `#facilityStoryGuideStep` («КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ N ИЗ 5:
  <STEP>») and `#facilityStoryGuideToggle` («ПОКАЗАТЬ» or «СКРЫТЬ»). Add `facilityGuideChoice` and
  `facilityGuideDeviceCount` with the open rule. Only a user toggle sets the choice, and a device
  count going from 0 to 1 or more clears it. Remove `position: sticky` from `.facility-story-steps`
  (FR-019 to FR-021, R10) · frontend/overseer/src/index.html, frontend/overseer/src/overseer.js,
  frontend/overseer/src/overseer.css

**⟶ Wait for Wave 9 to finish, then:**

**Wave 10 — single task:**

- [x] **T030** [US1] Render `#facilityPlayTransitions` with one `li.facility-play-transition` per
  transition leaving the current state, recovery ones included, its preconditions with
  `data-met`, «БЕЗ ПРЕДУСЛОВИЙ», and `#facilityPlayTransitionsEmpty`. Add the pure
  `facilityTransitionVerdict` in the server's order, and the one-press run through
  `desktopAPI.applyFacilityDeviceTransition`. Add the blocked-control reasons shared by every play
  control (no session, a command in flight, a save in flight). Make `reconcileFacilityOperation`
  apply `result.session` on `stale-revision` through `applyLiveSession` before it shows
  «СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС», for every play control (FR-001 to FR-004, R4,
  R5) · frontend/overseer/src/index.html, frontend/overseer/src/overseer.js,
  frontend/overseer/src/overseer.css

**⟶ Wait for Wave 10 to finish, then:**

**Wave 11 — single task:**

- [x] **T031** [US3] Add `#btnForceFacilityDeviceState` (blocked with «У УСТРОЙСТВА ОДНО
  СОСТОЯНИЕ» on a one-state device) and `#facilityForceStateDialog` with its target select,
  summary, warning, «ОТМЕНА», and «ПЕРЕВЕСТИ». The device ID, current state, and revision are
  captured when the dialog opens. A non-stale failure keeps the dialog open, and a stale result
  closes it and shows the stale text on the card (FR-007, FR-008) · frontend/overseer/src/index.html,
  frontend/overseer/src/overseer.js, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 11 to finish, then:**

**Wave 12 — single task:**

- [x] **T032** [US4] Render `#facilityPlayFaults` with one `role="switch"` per device-scoped fault,
  its current and starting position, and `#facilityPlayFaultsEmpty`. One press calls
  `desktopAPI.setFacilityConditionActive` with the opposite position, whatever the recovery
  setting. Move `#btnResetFacilityDevice` onto the card as «СБРОСИТЬ К НАЧАЛЬНОМУ…»
  (`btn-secondary`) with its dialog unchanged, so a fault-only reset shows «УСТРОЙСТВО СБРОШЕНО»
  (FR-009 to FR-011) · frontend/overseer/src/index.html, frontend/overseer/src/overseer.js,
  frontend/overseer/src/overseer.css

**⟶ Wait for Wave 12 to finish, then:**

**Wave 13 — single task:**

- [x] **T033** [US5] Render `#facilityPlayFeed` from `state.coordination.facilityChanges`: up to
  five `li.facility-feed-entry` for the selected device, newest first, with `<time>` in local
  `HH:MM`, the per-kind text, the reason text, names resolved from the session (raw ID when gone),
  and `#facilityPlayFeedEmpty`. Re-render it on every `coordination-state` event and session change
  without moving focus (FR-012 to FR-014) · frontend/overseer/src/index.html,
  frontend/overseer/src/overseer.js, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 13 to finish, then:**

**Wave 14 — single task:**

- [x] **T034** [US6] Group the actions into `#facilityDeviceInspectActions` and
  `#facilityDeviceChangeActions`. Rename the graph button to `#btnEditFacilityDevice`
  «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ». Add `#btnFacilityDeviceMenu` «⋯» and `#facilityDeviceMenu` with
  `#btnDeleteFacilityDevice` as its only `menuitem`, and the keyboard and close rules. Remove
  `#facilitySelectedDeviceName`, `#btnSaveFacilityDeviceChanges`,
  `#btnCancelFacilityDeviceChanges`, and the `device:<id>` unapplied-input key. Add
  `#facilityDetailTabs` with four ARIA tabs, counts, and `facilityDetailTab`, and split
  `#facilityBindingList` into `#facilityActionBindingList` and `#facilityInformationBindingList`.
  Give device list rows `.facility-device-row-state`, `span.facility-fault-count`,
  `data-current-state-id`, and `data-active-faults`, and keep search on the state name (FR-022 to
  FR-025, R11) · frontend/overseer/src/index.html, frontend/overseer/src/overseer.js,
  frontend/overseer/src/overseer.css

**Checkpoint**: `task test`, `task test:race`, `task vet`, `task proto:check`,
`task proto:breaking`, `task frontend:build`, and `task bindings:check` pass. Every operation and
the whole Overseer exist. The existing journeys that use removed UI are fixed in their story phases.

---

## Phase 3: User Story 1 — Run a device's transitions from its card (P1, MVP)

**Goal**: the Overseer runs an authored transition from the card, with the same rules as a player
command.

**Independent Test**: spec US1 Independent Test on a demo copy.

**Files**: `internal/session/facility_parity_test.go` (new),
`internal/session/testdata/demo-transition-verdicts.json` (new),
`tests/browser/facility-play-transitions.spec.mjs` (new)

### Tests

**Wave 1 — single task:**

- [x] **T035** [US1] Write the SC-002 parity test. Load `sessions/demo.json`. For every transition,
  build the facility states where all preconditions hold and where each one fails in turn. Run
  each through `ApplyFacilityDeviceTransition` and through the player path (`ApplyWorldAction` with
  a command that names the transition, on a terminal with no blocking fault), and require the same
  outcome. Write the verdicts to the golden table and fail when it drifts (FR-002, FR-003, SC-002) ·
  internal/session/facility_parity_test.go, internal/session/testdata/demo-transition-verdicts.json

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — single task:**

- [x] **T036** [US1] Write the US1 journey: acceptance scenarios 1–4, SC-001 (select, then one press,
  no dialog), the player terminal showing the result without reconnecting (FR-005, SC-004), no
  command marked executed (FR-003), and the edge cases "no transition leaves the current state" and
  "a player runs a command the Overseer already performed". For every row of the golden table, set
  the states through the fixture and compare the card's `data-available` with the row (SC-002) ·
  tests/browser/facility-play-transitions.spec.mjs

**Checkpoint**: US1 works and is tested on its own.

---

## Phase 4: User Story 2 — See the selected device first (P1)

**Goal**: the header and the first transition are visible without scrolling, and the guide stays
out of the way.

**Independent Test**: spec US2 Independent Test at 1440×900 and 1180×760.

**Files**: `tests/browser/facility-workspace-layout.spec.mjs` (new),
`tests/browser/facility-authoring-ux.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T037** [P] [US2] Write the US2 journey: acceptance scenarios 1–5. Check the header fields
  (FR-017). For every demo device at 1440×900 and 1180×760, check the layout guarantee in
  `contracts/overseer-ui.md` (FR-018, SC-003). Check the guide rule and the run memory across tabs
  and selections (FR-019, FR-020), and that `.facility-story-steps` is static and never overlaps
  while scrolling at 900, 1180, and 1440 (FR-021) · tests/browser/facility-workspace-layout.spec.mjs
- [x] **T038** [P] [US2] Update the guide journey for the collapsed-by-default guide and its
  summary toggle. Replace every other reference to removed UI in this file: the rename field,
  «ГОТОВО», «РЕДАКТИРОВАТЬ ГРАФ», `#facilityBindingList`, and `#facilityDeviceSummary` ·
  tests/browser/facility-authoring-ux.spec.mjs

**Checkpoint**: US2 works and is tested on its own.

---

## Phase 5: User Story 3 — Force a device into any state (P2)

**Goal**: the Overseer moves a device to any state after one confirmation.

**Independent Test**: spec US3 Independent Test with the reactor and «Критический перегрев».

**Files**: `tests/browser/facility-play-force.spec.mjs` (new)

### Tests

**Wave 1 — single task:**

- [x] **T039** [US3] Write the US3 journey: acceptance scenarios 1–3, the confirmation naming the
  device and both states (FR-008), no fault effect run, the `forced` feed entry, the reactor text
  variant on the player terminal, the one-state edge case, and a stale result while the dialog is
  open (FR-007, FR-008). The audit record is covered by T007 · tests/browser/facility-play-force.spec.mjs

**Checkpoint**: US3 works and is tested on its own.

---

## Phase 6: User Story 4 — Turn a device's faults on and off (P2)

**Goal**: the Overseer switches any fault of the device during play.

**Independent Test**: spec US4 Independent Test with «Контроллер гермодвери не отвечает».

**Files**: `tests/browser/facility-play-faults.spec.mjs` (new),
`tests/browser/facility-p0-fault-activity.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T040** [P] [US4] Write the US4 journey: acceptance scenarios 1–3, the effect on the player
  terminal (FR-010), the header and list counts, a fault whose recovery list excludes the Overseer
  still switching (FR-009), and a reset that only restores faults succeeding (FR-011) ·
  tests/browser/facility-play-faults.spec.mjs
- [x] **T041** [P] [US4] Update the reset steps for «СБРОСИТЬ К НАЧАЛЬНОМУ…» on the play card ·
  tests/browser/facility-p0-fault-activity.spec.mjs

**Checkpoint**: US4 works and is tested on its own.

---

## Phase 7: User Story 5 — See what happened to a device without watching (P2)

**Goal**: the card shows the device's last five changes, live.

**Independent Test**: spec US5 Independent Test, one change of each kind.

**Files**: `tests/browser/facility-play-feed.spec.mjs` (new)

### Tests

**Wave 1 — single task:**

- [x] **T042** [US5] Write the US5 journey: acceptance scenarios 1–4. Add one entry of each kind with
  its time, text, and author, newest first, and a sixth change that drops the oldest. Cover each
  refusal reason the fixture can produce, a pending request withdrawn by a play change (FR-006),
  a live update with focus kept (FR-014), the empty text, and an empty feed after opening another
  session (FR-012). Check that no entry shows a password or hack text (FR-016). Drive player
  commands on the facility-authoring fixture through `POST /__fixture/facility-authoring/player-command`
  (`{terminalId, commandId, decision}`); `withdrawn` needs the player-state fixture, which has a
  real pending request ·
  tests/browser/facility-play-feed.spec.mjs

**Checkpoint**: US5 works and is tested on its own.

---

## Phase 8: User Story 6 — Find device settings by role (P2)

**Goal**: actions grouped by role, delete in «⋯», details in tabs, and state and faults in the
device list.

**Independent Test**: spec US6 Independent Test on a demo copy.

**Files**:

- **New journey**: `tests/browser/facility-device-actions.spec.mjs`
- **Existing journeys**: `tests/browser/facility-unified-save.spec.mjs`, `facility-authoring.spec.mjs`,
  `facility-p0-visible-fields.spec.mjs`, `facility-p0-command-actions.spec.mjs`,
  `facility-command-lists.spec.mjs`, `facility-p0-identifiers.spec.mjs`, `facility-preview-diff.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T043** [P] [US6] Write the US6 journey: acceptance scenarios 1–4. Check that no
  `.btn-danger` sits outside the menu (SC-007), the menu's keyboard and close rules, renaming in the
  device dialog, the tab counts against the session with and without a selection, arrow, Home, and
  End keys, an empty tab with its add control, and the row text «Загрязнён · ● 1» updating after a
  play change (FR-022 to FR-025) · tests/browser/facility-device-actions.spec.mjs
- [x] **T044** [P] [US6] Rename through the device dialog instead of `#facilitySelectedDeviceName`,
  «ГОТОВО», and «ОТМЕНА». Use `#btnEditFacilityDevice`, the reset button on the card, and the split
  binding lists. Drop the `device:<id>` unapplied-input case · tests/browser/facility-unified-save.spec.mjs
- [x] **T045** [P] [US6] Delete through «⋯», rename through the dialog, and read the split binding
  lists · tests/browser/facility-authoring.spec.mjs
- [x] **T046** [P] [US6] Update the label sweep for the removed controls, the menu item, the card
  reset, and `#btnEditFacilityDevice` · tests/browser/facility-p0-visible-fields.spec.mjs
- [x] **T047** [P] [US6] Replace the rename field step with the device dialog ·
  tests/browser/facility-p0-command-actions.spec.mjs
- [x] **T048** [P] [US6] Read `#facilityActionBindingList` or `#facilityInformationBindingList`
  instead of `#facilityBindingList` · tests/browser/facility-command-lists.spec.mjs
- [x] **T049** [P] [US6] Open the device dialog through «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ» ·
  tests/browser/facility-p0-identifiers.spec.mjs
- [x] **T052** [P] [US6] Open the «НЕИСПРАВНОСТИ» or «ЧТО ВИДЯТ ИГРОКИ» tab before using the rows it
  holds (found at the Phase 2 checkpoint: the fault preview and the applied text rule cases) ·
  tests/browser/facility-preview-diff.spec.mjs

**Checkpoint**: US6 works and is tested on its own. No journey uses removed UI.

---

## Phase 9: Polish

**Files**: `specs/040-facility-play-controls/validation.md` (new)

**Wave 1 — single task:**

- [x] **T050** Run `go fix ./...` and review every edit. Then run `task fmt:check`, `task vet`,
  `task lint`, `task test`, `task test:race`, `task proto:check`, `task proto:breaking`,
  `task bindings:check`, `task frontend:build`, `task browser:test`, and `task secret-leak:check`.
  Record each result and the evidence for SC-001 to SC-008 · specs/040-facility-play-controls/validation.md

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — single task:**

- [x] **T051** Build the packaged app (`task package`) and open a demo copy. At 1440×900 and
  1180×760, check the header and first transition of every demo device with the guide collapsed.
  With one player browser connected, run a card transition, a forced change, a fault switch, and a
  reset, and check the player's terminal and the feed. Reopen a session saved before this feature.
  Record every result (FR-018, FR-026, SC-003, SC-004, SC-008) ·
  specs/040-facility-play-controls/validation.md

---

## Dependencies & Execution Order

- **Phases**: Setup → Foundational → US1 to US6 → Polish. The six story phases depend only on
  Phase 2 and Phase 1, own disjoint files, and can run in parallel. US1 and US2 are the MVP.
- **Setup**: W1 (T001).
- **Foundational**: W1 (contract, domain, demo) → W2 (all Go tests) → W3 (session operations, feed
  helpers) → W4 (control service, facility, scene, authoring) → W5 (bridge) → W6 (bindings,
  fixtures, API tests) → W7 (`desktop-api.js`) → W8 to W14 (one Overseer task each, in story order
  US2, US2, US1, US3, US4, US5, US6).
- **US1**: W1 (parity test and golden table) → W2 (journey that reads the table).
- **US2, US4, US6**: one wave each; their tasks touch different files.
- **US3, US5**: one task each.
- **Polish**: W1 (suites) → W2 (packaged app); both write `validation.md`.
