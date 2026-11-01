# Tasks: Facility Recheck Fixes

**Input**: Design documents from `specs/042-facility-recheck-fixes/`

**Prerequisites**: `facility-recheck-fixes.spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/`

**Scale note**: About 25 files in five areas: proto and generated code, the Go control service, the player client, the Overseer frontend, and the browser fixture. `overseer.js` is shared by three stories, so all Overseer edits sit in one phase (Phase 5) and run in sequence there. The schema change moves the pinned schema-revision digest.

**Tests**: The constitution requires Go table tests and a browser journey per story. Tests are written to fail first.

## Phase 2: Foundational (blocks every story)

**Files**: `proto/fallout/terminal/player/v1/player.proto`, `proto/fallout/terminal/private/v1/coordination.proto`, `proto/schema-revision.txt`, `internal/gen/`, the generated ES clients, `scripts/wails-v3-contract-check.sh`, `internal/domain/model.go`, `internal/domain/facility.go`, `internal/player/adapter.go`, `internal/player/adapter_test.go`, `app_contract.go`, `app_contract_test.go`, `frontend/overseer/bindings/`

**Wave 1 — single task:**

- [x] **T001** Add `ACTION_REASON_FAULT_BLOCKED = 11` to `ActionReason`, and `entry_id`, `capability` (imported `persistence.v1.FacilityCapability`), `condition_ids` and `attempts` to `FacilityChangePlayerRefusal`, exactly as `contracts/player-action-reason.md` and `contracts/facility-feed-refusal.md` state (FR-001, FR-003, FR-004) · proto/fallout/terminal/player/v1/player.proto, proto/fallout/terminal/private/v1/coordination.proto

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T002** Run `task proto:generate`, review the generated Go and ES code, then update the reviewed schema-revision digest pinned in `scripts/wails-v3-contract-check.sh` and confirm `task proto:check`, `task proto:breaking` (against the unchanged baseline) and `task wails:pins:check` pass · proto/schema-revision.txt, internal/gen/, scripts/wails-v3-contract-check.sh

**⟶ Wait for T002, then Wave 3 — independent (different files):**

- [x] **T003** [P] Add `ActionReasonFaultBlocked = "fault-blocked"`, and the refusal fields `EntryID`, `Capability`, `ConditionIDs` and `Attempts` with JSON keys `entryId`, `capability`, `conditionIds` and `attempts` (omitted when empty), extend `CloneFacilityChanges` to copy `ConditionIDs`, and map the new fields both ways in the private facility-change adapter with a round-trip test case (data-model.md; the adapter was found by T002's contract test) · internal/domain/model.go, internal/domain/facility.go, app_contract.go, app_contract_test.go
- [x] **T004** [P] Map `domain.ActionReasonFaultBlocked` to `ACTION_REASON_FAULT_BLOCKED` with a table test case (FR-001) · internal/player/adapter.go, internal/player/adapter_test.go

**⟶ Wait for Wave 3 to finish, then:**

- [x] **T005** Regenerate the Wails bindings with `go tool -modfile=tools/wails/go.mod wails3 generate bindings -clean ./...` for the new refusal fields, review the diff, and confirm `task bindings:check` · frontend/overseer/bindings/

---

## Phase 3: User Story 1 — Know why a fault blocked a request (Priority: P1, server and player) 🎯 MVP

**Goal**: A fault-blocked request is refused with its own reason, the player reads a precise Russian message, and the blocking device's feed records one counted entry.

**Independent Test**: Go table tests for every capability and the merge; then the player message through Phase 4's journey and the feed text through Phase 5's journey.

**Files**: `internal/control/facility.go`, `internal/control/feed.go`, `internal/control/service.go`, `internal/control/service_test.go`, `internal/control/feed_test.go`, `internal/control/facility_test.go`, `app.go`, `app_test.go`, `frontend/client/client.js`, `frontend/overseer/src/desktop-api.js`, `tests/browser/desktop-api.spec.mjs`, `sessions/demo-capability-paths.md`, `tests/browser/demo-session.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T006** [P] [US1] Add failing table tests: each of the five capabilities blocked by an active fault returns `ActionReasonFaultBlocked`; a device fault records one refusal on its device with the entry or command, capability and sorted condition IDs; a terminal fault records none; a blocked command with a device action records one entry per device in the union of fault and target devices; the audit carries action `capability-block`, the capability and the condition IDs and no authored text (FR-001, FR-003, FR-005, FR-006) · internal/control/service_test.go
- [x] **T007** [P] [US1] Add failing table tests for the merge: a refusal equal to the device's newest entry replaces it with a new sequence and time and `Attempts` plus one; a different refusal, or one after another change, prepends; the feed still keeps five entries (FR-004) · internal/control/feed_test.go

### Implementation

**⟶ Wait for the tests, then Wave 2 — independent (different files):**

- [x] **T008** [P] [US1] Add `blockingFacilityConditions` returning the active conditions that block a capability on a terminal (keep `facilityCapabilityBlocked` as its non-empty check), add `Capability` and `EntryID` to `FacilityAuditFacts` and the action `capability-block`, and build the fault-block refusal changes for the device union of research R3 · internal/control/facility.go
- [x] **T009** [P] [US1] Merge an identical player refusal into the device's newest entry in `appendFacilityChanges`, comparing terminal, command, entry, capability, condition IDs and reason (research R4) · internal/control/feed.go

**⟶ Wait for Wave 2 to finish, then:**

- [x] **T010** [US1] In the capability-blocked branch of player actions return `ActionReasonFaultBlocked`, record the refusal through the new helper for every capability (replacing the target-only `refuseFacilityCommand` call there), publish the master snapshot when an entry is recorded, and attach the facility audit facts, logged by `app.go` with the capability, faults and entry and a warn severity; update the 035 capability test that pinned the generic reason; make T006 and T007 pass (FR-001, FR-003 to FR-006) · internal/control/service.go, internal/control/facility_test.go, app.go, app_test.go

**⟶ Wait for T010, then Wave 4 — independent (different files):**

- [x] **T011** [P] [US1] Map `ActionReason.FAULT_BLOCKED` to `fault-blocked` in `actionReasonName` and add its message `ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.` to `ACTION_REASON_MESSAGES` (FR-002) · frontend/client/client.js
- [x] **T012** [P] [US1] Normalize `entryId`, `capability`, `conditionIds` and `attempts` in `normalizeFacilityChange`, with a desktop API journey case for a fault-block refusal (contracts/facility-feed-refusal.md) · frontend/overseer/src/desktop-api.js, tests/browser/desktop-api.spec.mjs
- [x] **T013** [P] [US1] Add the «Request blocked by a fault» row to the play table of the demo inventory: reading `n_entry_vault_shutdown` on `t_demo1` at scene start is refused, the ventilation card's feed shows it, and switching `air-spore-bloom` off lets the player read; keep `demo-session.spec.mjs` in step if it checks the table (FR-007) · sessions/demo-capability-paths.md, tests/browser/demo-session.spec.mjs

**⟶ Found at the Phase 4 probe (T014), then:**

- [x] **T030** [US1] Keep a refused request's notice when the controller's cursor presentation is sent or accepted right after it: only a presentation rejection's own notice is cleared by the next presentation; the refusal notice disappeared after about 15 ms (FR-002) · frontend/client/client.js

**Checkpoint**: The server refuses fault-blocked requests with their own reason and records counted feed entries; the player client and the Overseer bridge understand both.

---

## Phase 4: User Story 3 — Regression journeys tell a blocked request from an allowed one (Priority: P2)

**Goal**: The diagnostics journeys fail whenever a blocked entry opens.

**Independent Test**: Disable the block on the server side only and see the journey fail; restore it and see it pass.

**Files**: `tests/browser/fixture-server/main.go`, `tests/browser/facility-diagnostics.spec.mjs`

- [x] **T014** [US3] Reproduce the gap: run the `offline` scenario, open «AFFECTED ENTRY», and find why the server accepts it although `terminal-facility-diagnostics` has an active view-entry block; record the cause for validation.md (research R6) · tests/browser/fixture-server/main.go

**⟶ Wait for T014, then:**

- [x] **T015** [US3] Route the fixture participant through the same refusal branch as the app, and give every blockable fixture entry authored text that differs from any refusal or error message (FR-012, FR-013) · tests/browser/fixture-server/main.go

**⟶ Wait for T015, then:**

- [x] **T016** [US3] Rewrite the blocked-capability journeys: assert `ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.`, that the entry's authored text is absent while blocked and shown after the fault is switched off, and the same reason for command, terminal-transition, recovery-program and hack blocks; run once with the server block disabled to confirm the journeys fail (FR-002, FR-014, SC-002, SC-004) · tests/browser/facility-diagnostics.spec.mjs

**Checkpoint**: The journeys detect a blocked request that slips through.

---

## Phase 5: Overseer frontend — US1 feed text, US2 templates, US4 workspace (Priorities P1, P2, P3)

**Goal**: The Overseer renders refusals, applies templates by cell, and fixes the recheck's layout and copy defects.

**Independent Test**: Each journey below on the demo seed at 1440×900.

**Files**: `frontend/overseer/src/overseer.js`, `frontend/overseer/src/overseer.css`, `frontend/overseer/src/index.html`, `tests/browser/facility-play-feed.spec.mjs`, `tests/browser/facility-device-templates.spec.mjs`, `tests/browser/facility-device-list.spec.mjs`, `tests/browser/facility-workspace-layout.spec.mjs`, `tests/browser/facility-fault-dialog.spec.mjs`, `tests/browser/facility-vocabulary.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T017** [P] [US1] Feed journey: on the demo seed a participant reads «СВОДКА ЭВАКУАЦИИ» five times at scene start; the ventilation card shows one entry «чтение записи «СВОДКА ЭВАКУАЦИИ» · запрос игрока отклонён: мешает неисправность «Споровое загрязнение воздушного контура» · Терминал смотрителя — Убежище 76 · ×5»; update the existing refusal assertions to the new format (FR-003, FR-004, SC-001) · tests/browser/facility-play-feed.spec.mjs
- [x] **T018** [P] [US2] Template journey: the demo door shows `ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ` on open and adds nothing; an empty door gains 2 states and 2 transitions; a door with one renamed «Заперта → Открыта» gains only the missing state and «Запереть» (FR-008 to FR-010, SC-003) · tests/browser/facility-device-templates.spec.mjs
- [x] **T019** [P] [US4] Sidebar journey at 1440×900: `#btnToggleTerminalFaults` collapses the group, `#facilityTerminalFaultsActiveCount` reads `АКТИВНО N` in both states, at least seven demo rows are visible, the choice holds after reopening the session, a one-row filter shows its whole border, and a keyboard-focused row's outline is not clipped (FR-015 to FR-017, FR-022, SC-005, SC-006) · tests/browser/facility-device-list.spec.mjs
- [x] **T020** [P] [US4] Workspace journey: `#facilityStatus` takes no height while empty, tree rows read `[КОМАНДА] <name>`, and rule, action and program row buttons read `ИЗМЕНИТЬ` and `УДАЛИТЬ` with unchanged accessible names (FR-018, FR-020, FR-021) · tests/browser/facility-workspace-layout.spec.mjs
- [x] **T021** [P] [US4] Fault dialog journey: `В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА` renders as Overseer segmented controls, not system radios, and `#facilityConditionInitialWorking` / `#facilityConditionInitialActive` still check (FR-019) · tests/browser/facility-fault-dialog.spec.mjs

### Implementation

**⟶ Wait for the tests, then Wave 2 — independent (different files):**

- [x] **T022** [P] [US1] Render fault-block refusals in `facilityFeedText`: the target by capability, the fault names, the terminal, and `×N` for two or more attempts; set `data-capability` and `data-attempts` (contracts/facility-feed-refusal.md) · frontend/overseer/src/overseer.js
- [x] **T023** [P] [US4] Add `#btnToggleTerminalFaults` inside `#facilityTerminalFaultsHeading` and `#facilityTerminalFaultsActiveCount`; style the sidebar as a column so the device list takes the freed height, pad the list so rows and focus outlines draw whole, hide an empty `#facilityStatus` from layout, draw the start-position radios as segmented controls, and space tree kind tags (contracts/overseer-ui.md) · frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 2 to finish, then (same file as T022, in sequence):**

- [x] **T024** [US2] Count a template transition as present when its from→to cell has any transition, whatever its name (research R7) · frontend/overseer/src/overseer.js

**⟶ Wait for T024, then:**

- [x] **T025** [US4] Wire the terminal-fault toggle with a run-long collapsed flag kept across reopening, render the active count, keep `#facilityStatus` hidden while empty, write `ИЗМЕНИТЬ` / `УДАЛИТЬ` in `facilityRowActions`, and render the tree kind tag with a space; then make T017 to T021 pass and keep `facility-vocabulary.spec.mjs` green · frontend/overseer/src/overseer.js, tests/browser/facility-vocabulary.spec.mjs

**Checkpoint**: The Overseer shows counted refusals, applies templates by cell, and the workspace defects are gone.

---

## Phase 6: Records and Polish

**Files**: `specs/033-entry-content-state/tasks.md`, `specs/033-entry-content-state/validation.md`, `specs/041-facility-editors-language/spec.md`, `specs/042-facility-recheck-fixes/validation.md`

**Wave 1 — independent (different files):**

- [x] **T026** [P] Verify the 033 block command dialog against its FR-023 to FR-026 and SC-009 through the passing journeys in `state-changing-command-authoring.spec.mjs`, mark T026 to T030 of 033 done, and record the evidence (FR-023, SC-007) · specs/033-entry-content-state/tasks.md, specs/033-entry-content-state/validation.md
- [x] **T027** [P] Replace 041 FR-011 and User Story 3 scenario 2 with the cell rule, pointing to this feature (FR-011) · specs/041-facility-editors-language/spec.md

**⟶ Wait for Wave 1 and Phases 2 to 5, then:**

- [x] **T028** Run `go fix ./...` and review every edit, then `task fmt:check`, `task vet`, `task lint`, `task test`, `task test:race`, `task proto:check`, `task proto:breaking`, `task bindings:check`, `task wails:pins:check`, `task frontend:build`, `task secret-leak:check` and `task browser:test`; record each result and the evidence for SC-001 to SC-008 (FR-024) · specs/042-facility-recheck-fixes/validation.md

**⟶ Wait for T028, then:**

- [x] **T029** Build the packaged app (`task package`) and, at exactly 1440×900 on a demo copy with a connected player, check SC-001 (five refused reads, one ×5 entry, then reading after the fault is off), SC-003 (the demo door template) and SC-005 (seven rows with the group collapsed); reopen a session saved before this feature and save it unchanged (FR-024, SC-008) · specs/042-facility-recheck-fixes/validation.md

---

## Dependencies & Execution Order

- **Phases**: Foundational → US1 (Phase 3) → US3 (Phase 4) → Overseer (Phase 5) → Records and Polish (Phase 6). Phase 4 needs Phase 3's reason; Phase 5's feed journey needs Phase 3's records; Phase 6's T026 and T027 can run any time.
- **Foundational**: T001 → T002 → T003 and T004 → T005.
- **US1**: T006 and T007 → T008 and T009 → T010 → T011, T012 and T013.
- **US3**: T014 → T015 → T016.
- **Overseer**: T017 to T021 → T022 and T023 → T024 → T025.
- **Polish**: T026 and T027 → T028 → T029.
