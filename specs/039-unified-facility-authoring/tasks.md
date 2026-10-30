# Tasks: Unified Facility Authoring

**Input**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/private-desktop.md`,
`contracts/overseer-ui.md`, `quickstart.md`

**Structure**: The three stories share the session store, the coordinator, the Wails bridge, the
fixture server, and the single-file Overseer (plan, Structure Decision). Every file has one owner
phase, so all implementation is built in Phase 2, in story order and tagged by story. Each story
phase then owns its acceptance journey and the existing journeys its behavior changes.

- Go work follows test-first waves: each test file is written before its source file and fails
  until that source lands.
- The Go build compiles at every wave join. It may break inside a wave.
- The Overseer is not runnable from Wave 9 (T028) until Wave 16 (T038) completes; no browser
  journey runs before the Phase 2 checkpoint.
- A story journey that finds a defect in a Phase 2 file reports it back. The fix lands in that file
  through the main agent, never inside a parallel story worker.

**Format**: `- [ ] **T###** [P?] [US#] Description · file`

---

## Phase 1: Setup

**Wave 1 — single task:**

- [x] **T001** Add the shared journey helpers used by all three story specs. They open the
  Overseer on a demo copy, read `#saveStatus` text and `data-state`, wait for `saved`, press
  «ОТМЕНИТЬ» and «ПОВТОРИТЬ», and switch the two workspace tabs (`contracts/overseer-ui.md`, shared
  authoring bar) · tests/browser/helpers/authoring-bar.mjs

---

## Phase 2: Foundational (blocks every story)

**Files**:

- **Contract**: `proto/fallout/terminal/private/v1/desktop.proto`, `internal/gen/fallout/terminal/private/v1/`, `proto/compatibility-baseline.binpb`, `proto/schema-revision.txt`
- **Live**: `internal/live/facility.go`, `presentation.go`, `presentation_test.go`, `preview.go`, `preview_test.go`
- **Session**: `internal/session/service.go`, `facility.go`, `authoring.go`, `authoring_test.go`, `scene.go`, `scene_test.go`, `service_test.go`, `facility_test.go`
- **Control**: `internal/control/authoring.go`, `authoring_test.go`, `scene.go`, `scene_test.go`, `facility.go`, `service.go`, `facility_test.go`, `service_test.go`
- **Bridge**: `main.go`, `app.go`, `desktop_service.go`, `app_contract.go`, `app_contract_test.go`, `app_test.go`, `wails_host_test.go`
- **Bindings and scripts**: `frontend/overseer/bindings/`, `scripts/wails-v3-contract-check.sh`, `scripts/wails-bindings-check.sh`, `scripts/state-changing-reset-native-smoke.sh`
- **Fixtures and API**: `tests/browser/fixture-server/main.go`, `tests/browser/fixtures/desktop-bindings.js`, `frontend/overseer/src/desktop-api.js`, `tests/browser/desktop-api.spec.mjs`, `internal/platform/assets_test.go`
- **Overseer**: `frontend/overseer/src/overseer.js`, `index.html`, `overseer.css`

**Wave 1 — independent (different files):**

- [x] **T002** [P] Add every new message, enum, and additive field from
  `contracts/private-desktop.md`: `SaveAuthoringRequest`, `ResetSceneRequest`, `CommandReference`,
  `DescribePresentation*`, `TerminalPresentation`, `NodePresentation`, `BlockPresentation`,
  `PresentationRule*`, `CommandExecutedCondition`, `DiagnosticActiveCondition`,
  `PreviewDifference*` (`PresentationRule.value` is a `oneof`), and the new fields on
  `PreviewFacilityRequest`/`Result`, `FacilityOperationResult`, `SessionOperationResult`,
  `ReplaceTerminalGroupsResult`, and `PlayerConfigOperationResult`. Run `task proto:generate`;
  removed messages stay until T027 ·
  proto/fallout/terminal/private/v1/desktop.proto
- [x] **T003** [P] [US2] Extract per-target resolvers for name, visibility, availability, and block
  text from `applyFacilityProjection`, so projection and explanation share them (research R10). Keep
  behavior identical; the existing live tests must stay green · internal/live/facility.go
- [x] **T004** [P] [US1] Add the runtime-only `AuthoringRevision` to `ActiveSession` (FR-007,
  `data-model.md`). It is 1 after create, open, and copy demo, and +1 when
  `ReplaceTerminalGroups` or `AssociatePlayerConfig` changes the document. Return it from `Snapshot`
  and the session results · internal/session/service.go

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — tests first, independent (different files):**

- [x] **T005** [P] [US2] Explanation tests. For every terminal of every file in `sessions/` and the
  browser fixture sessions, the `playersSee` rule's value equals the projected name, visibility,
  availability, and block text. Rules are ordered by the FR-010 precedence, and exactly one rule per
  target is marked. For the demo command «Открыть выход и подать сигнал», the state name outranks
  the name after execution, and the power-grid availability rule is listed.
  `BenchmarkExplainDemo` records the 50 ms budget · internal/live/presentation_test.go
- [x] **T006** [P] [US3] Preview difference tests on the demo with the door going from sealed to
  open: block 3 of «СВОДКА ЭВАКУАЦИИ» changes text, the command name becomes «Выход уже открыт»,
  and «НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН» appears. Also cover the availability change cases, tree and
  block ordering, an empty diff for an identical state, and that inputs are never mutated (FR-018,
  FR-020). `BenchmarkPreviewDifferencesDemo` records the 50 ms budget ·
  internal/live/preview_test.go
- [x] **T007** [P] [US1] `SaveAuthoring` store tests (FR-001, FR-007, FR-012, FR-021, SC-007):
  - accepted after a command execution or a device change;
  - `STALE_REVISION` after an authored change, returning the canonical session;
  - live values in the candidate ignored;
  - a facility and binding edit accepted in the same save as a terminal edit;
  - no-op unchanged;
  - invalid candidate rejected with issues and nothing written;
  - bundled demo refused;
  - write failure leaves canonical state unchanged;
  - authoring revision increments only on change;
  - every `sessions/` file round-trips byte-identically with identical projections.
  · internal/session/authoring_test.go
- [x] **T008** [P] [US2] `ResetScene` store tests (FR-014):
  - one write restores devices and faults to their starting values and clears every terminal's
    command states;
  - stale session or facility revision changes nothing;
  - a forced write failure rolls back fully;
  - the authoring revision is unchanged;
  - an already-initial scene returns `!changed`.
  · internal/session/scene_test.go
- [x] **T009** [P] [US1] Coordinator `SaveAuthoring` tests (FR-001c, R4, R5):
  - every broadcast terminal is refreshed, with hacking and navigation kept;
  - a pending device action whose fingerprint is unchanged is rebased and stays approvable;
  - a pending action whose command changed is rejected through the existing path;
  - a world action interleaved with saves never rejects a save;
  - the audit and operation record carry revisions and no authored text;
  - for each pending player request the save rebases or rejects: exactly one correlated outcome
    record with safe prior and resulting revisions, none on retry, no authored text; with a failing
    log sink the decision and publication are unchanged (Principle VIII);
  - a concurrent save, world action, and scene reset pass `task test:race`.
  · internal/control/authoring_test.go
- [x] **T010** [P] [US2] Coordinator `ResetScene` tests (FR-014, FR-015):
  - the facility is installed only after a durable success;
  - reset commands lose their presentation, and a pending request for one is cleared;
  - a pending device action is invalidated, and any other pending request stays;
  - the active terminal, return route, and hacking are untouched;
  - one audit record per outcome carries the correlation ID, revisions, and counts;
  - for each pending player request the reset clears or invalidates: exactly one correlated
    outcome record with safe prior and resulting revisions, none on retry, no authored text; with a
    failing log sink the decision and publication are unchanged (Principle VIII);
  - a failure leaves the runtime unchanged.
  · internal/control/scene_test.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent (different files):**

- [x] **T011** [P] [US2] Implement the session presentation explanation (R10, `data-model.md`). It
  uses the T003 resolvers and covers legacy state-change names and blocks next to device rules
  (FR-012). T005 must pass · internal/live/presentation.go
- [x] **T012** [P] [US3] Implement the preview differences from per-block projections, with kinds,
  ordering, and before and after values (R13). T006 must pass · internal/live/preview.go
- [x] **T013** [P] [US1] Implement `SaveAuthoring` (R1):
  - check the authoring revision;
  - merge canonical command states, device current states, fault activity, terminal groups, and the
    facility revision;
  - validate with `verifySessionContract`, `validateAuthoredTransitionChanges`, and
    `ValidateFacilityAuthoringCandidate`;
  - write once through `commitFacilityCandidateLocked`;
  - step the facility revision only when the facility changed.

  T007 must pass · internal/session/authoring.go
- [x] **T014** [P] [US2] Implement `ResetScene` (R12): one candidate with devices and faults at their
  starting values and command states cleared, written with `rollbackOnFailure`. T008 must pass ·
  internal/session/scene.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent (different files):**

- [x] **T015** [P] [US1] Implement coordinator `SaveAuthoring`:
  - serialize through `commit`;
  - install the facility and refresh every broadcast terminal via
    `refreshFacilityAuthoringRuntimes`;
  - rebase or reject the pending device action by `ActionFingerprint` (R4);
  - reproject and publish;
  - define the store interface.

  T009 must pass · internal/control/authoring.go
- [x] **T016** [P] [US2] Implement coordinator `ResetScene`:
  - install the facility only after success;
  - `clearResetCommandPresentation` terminal-wide for each broadcast terminal, then reproject;
  - apply the pending rules and write the audit;
  - define the store interface.

  T010 must pass · internal/control/scene.go
- [x] **T017** [P] [US3] Extend `PreviewFacility` to return `Current` from the same snapshot, plus
  `Differences` from `internal/live` (FR-017, FR-018) · internal/control/facility.go

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — independent (different files):**

- [x] **T018** [P] Add adapters and round-trip tests for every new request, result, enum, and
  additive field (Principle II, `contracts/private-desktop.md`). Reject a `PresentationRule` whose
  `value` case does not match its `kind`. Compare messages with `cmp.Diff(…, protocmp.Transform())`
  and run `prototest` conformance for every new message · app_contract.go, app_contract_test.go
- [x] **T019** [P] Add the `SaveAuthoring` and `ResetScene` store adapters on
  `sessionCommandStateStore` that satisfy the T015 and T016 store interfaces · main.go

**⟶ Wait for Wave 5 to finish, then:**

**Wave 6 — single task:**

- [x] **T020** Add the bridge methods with tests:
  - Wails `SaveAuthoring`, `ResetScene`, and `DescribePresentation` (read-only, from the session
    snapshot via T011);
  - `PreviewFacility` checks `expectedAuthoringRevision` (FR-019);
  - session open, create, and copy results and group and player-config results carry the authoring
    revision;
  - `recordOperation("authoring.save", …)` records revisions only;
  - runtime save state is updated from `SaveAuthoring`.

  · app.go, desktop_service.go, app_test.go

**⟶ Wait for Wave 6 to finish, then:**

**Wave 7 — cutover removals, independent (different files; compiles at the join):**

- [x] **T021** [P] [US1] Remove the Wails `SaveSession`, `SaveFacilityAuthoring`, `ResetFacility`,
  `ResetTerminalCommandStates`, and `UpdateLiveTerminal` methods with their payloads and tests.
  Also delete the legacy-fake fallback that calls the coordinator's `UpdateLiveTerminal` (around
  `app.go:3341–3350`), `UpdateLiveTerminal` from `coordinationTerminalService` (`app.go:160`), and
  the recording fake method (`app_test.go:5527`). Point the binding test at `SaveAuthoring`
  (Principle VII, R5, R15) · app.go, desktop_service.go, app_test.go, wails_host_test.go
- [x] **T022** [P] Remove the adapters and contract tests for `SaveSessionResult`,
  `SaveFacilityAuthoringRequest`, `ResetFacilityRequest`, `ResetTerminalCommandStatesRequest`, and
  `LiveTerminalUpdateRequest` · app_contract.go, app_contract_test.go
- [x] **T023** [P] Remove the `SaveFacilityAuthoring`, `ResetFacility`, and
  `ResetTerminalCommandStates` store adapters · main.go
- [x] **T024** [P] [US2] Remove coordinator `SaveFacilityAuthoring`, whole-facility
  `ResetFacility`, `ResetTerminalCommandStates`, and `UpdateLiveTerminal` with its tests. Keep
  `ResetFacilityDevice`, `ResetCommandState`, and `RefreshActiveTerminal`. Migrate the surviving
  tests (FR-015) · internal/control/facility.go, internal/control/service.go,
  internal/control/facility_test.go, internal/control/service_test.go
- [x] **T025** [P] [US1] Remove `Service.Save`, `preserveCanonicalFacility`,
  `validateFacilityBindingsUnchanged`, session `SaveFacilityAuthoring`, whole `ResetFacility`, and
  `ResetTerminalCommandStates`. Keep the serialized writer used by `commitFacilityCandidateLocked`.
  Migrate the surviving tests · internal/session/service.go, internal/session/facility.go,
  internal/session/service_test.go, internal/session/facility_test.go
- [x] **T026** [P] Move the five fixture `service.UpdateLiveTerminal` calls to
  `RefreshActiveTerminal` with a canonical terminal target, so the Go build stays green when T024
  removes the coordinator method · tests/browser/fixture-server/main.go

**⟶ Wait for Wave 7 to finish, then:**

**Wave 8 — single task:**

- [x] **T027** Delete the five superseded messages and run `task proto:generate`. Refresh
  `proto/compatibility-baseline.binpb`, record the revision in `proto/schema-revision.txt`, and
  update the pinned digests (`contracts/private-desktop.md`, Migration). `task proto:check` and
  `task proto:breaking` must pass · proto/fallout/terminal/private/v1/desktop.proto,
  scripts/wails-v3-contract-check.sh

**⟶ Wait for Wave 8 to finish, then:**

**Wave 9 — independent (different files):**

- [x] **T028** [P] Regenerate the Wails bindings and update the method allowlist and signatures
  for the three added and five removed methods. `task bindings:check` must pass ·
  frontend/overseer/bindings/, scripts/wails-bindings-check.sh
- [x] **T029** [P] [US2] Replace the `ResetTerminalCommandStates` checks with `ResetCommandState`
  and `ResetScene` · scripts/state-changing-reset-native-smoke.sh
- [x] **T030** [P] Update the fixture server:
  - shim exports `SaveAuthoring`, `ResetScene`, and `DescribePresentation`, and drops the five
    removed exports;
  - the facility-authoring fake gains authoring-revision checks, live-value merge, `reset-scene`,
    and `describe`;
  - the fake's preview and describe use the exported `internal/live` functions (R16);
  - the `facility-player-state` store implements the new store interfaces on the real coordinator;
  - a `force-failure` scenario covers scene reset.

  · tests/browser/fixture-server/main.go
- [x] **T031** [P] Replace the removed methods with `SaveAuthoring`, `ResetScene`, and
  `DescribePresentation` fakes, and record their calls for journeys ·
  tests/browser/fixtures/desktop-bindings.js

**⟶ Wait for Wave 9 to finish, then:**

**Wave 10 — single task:**

- [x] **T032** Add `saveAuthoring`, `resetScene`, and `describePresentation`, plus the preview
  `expectedAuthoringRevision` and the result normalization for `current`, `differences`,
  `authoringRevision`, and `resetCommands`. Remove `saveSession`, `saveFacilityAuthoring`,
  `resetFacility`, `resetTerminalCommandStates`, and `updateLiveTerminal`. Update the allowlist
  assertions · frontend/overseer/src/desktop-api.js, tests/browser/desktop-api.spec.mjs,
  internal/platform/assets_test.go

**⟶ Wait for Wave 10 to finish, then:**

**Wave 11 — single task (Overseer files from here on, one task per wave):**

- [x] **T033** [US1] Add the save pipeline (R3, R6, `data-model.md`):
  - `commitAuthoringEdit({ label, scope, mutate })`;
  - one request in flight and one request per edit, sent in order; a rejection reverts only that
    edit, replays later queued edits on the canonical session, and reopens the rejected edit's
    dialog or form with its input;
  - authoring revision tracking;
  - success, validation-failure, and stale outcomes, with «ПРИМЕНИТЬ СНОВА» re-running the
    retained `mutate`;
  - `mergeLiveValues` for `onSessionState` and for reset and recovery results.

  Delete `autosave()`, `facilityDraft*`, and `saveFacilityCandidate` · frontend/overseer/src/overseer.js

**⟶ Wait for Wave 11 to finish, then:**

**Wave 12 — single task:**

- [x] **T034** [US1] Convert every commit into a pure `mutate` passed to `commitAuthoringEdit`:
  - terminal create, rename, delete, settings, node form apply, and the entry-block command
    dialog;
  - every facility dialog and the device detail form;
  - the story-guide actions.

  No other save path may remain (FR-001, FR-002) · frontend/overseer/src/overseer.js

**⟶ Wait for Wave 12 to finish, then:**

**Wave 13 — single task:**

- [x] **T035** [US1] Add the shared `#authoringBar` next to the workspace tabs and move `#saveStatus`
  into it. Render a primary part in priority order (error, saving, saved) with `data-state`, and a
  secondary «НЕ ПРИМЕНЕНО: …» part with `data-unapplied-count`.
  Add `#btnUndo`, `#btnRedo`, `#btnReapplyEdit`, and `#btnResetScene` markup. Remove save wording
  from `#facilityStatus` (FR-003, SC-002) · frontend/overseer/src/index.html,
  frontend/overseer/src/overseer.css, frontend/overseer/src/overseer.js

**⟶ Wait for Wave 13 to finish, then:**

**Wave 14 — single task:**

- [x] **T036** [US1] Add undo and redo (R9):
  - normalized shared snapshots, depth 20;
  - the equality precondition;
  - saves through the pipeline;
  - redo cleared on a new edit, and both stacks cleared on open, create, or copy;
  - control reasons and the server's rejection reason (FR-001a, FR-001b).

  · frontend/overseer/src/overseer.js

**⟶ Wait for Wave 14 to finish, then:**

**Wave 15 — single task:**

- [x] **T037** [US1] Add the unapplied-input registry for the node form, terminal settings, and
  device detail form (R7):
  - capture on the first input and restore on re-render;
  - set `data-unapplied`;
  - clear on apply or cancel;
  - list «НЕ ПРИМЕНЕНО: …» in the status;
  - show the missing-entity notice (FR-002, FR-003).

  · frontend/overseer/src/overseer.js

**⟶ Wait for Wave 15 to finish, then:**

**Wave 16 — single task:**

- [x] **T038** [US1] Vocabulary and explained controls (R14, FR-001c, FR-004, FR-005, FR-006):
  - «ГОТОВО» on every facility edit dialog and on `#btnSaveFacilityDeviceChanges`;
  - remove `#btnSaveFacility`, `#btnDiscardFacility`, the draft guide actions and texts,
    `#commandDraftGuard`, `#btnPublish`, and the settings `updateLiveTerminal` call;
  - the new `#createTerminalDialogDescription`;
  - `controlBlockReason` with `aria-disabled` and `.control-reason` on reset, preview, and recovery
    controls.

  · frontend/overseer/src/index.html, frontend/overseer/src/overseer.js,
  frontend/overseer/src/overseer.css

**⟶ Wait for Wave 16 to finish, then:**

**Wave 17 — single task:**

- [x] **T039** [US2] Presentation fetch and cache (R11). Call `describePresentation` after open,
  each accepted save, reset, or recovery, and live events, letting the newest revision win. Tree
  labels show `playerName` with `data-authored-name`, a «В РЕДАКТОРЕ» title, and the
  `.tree-badge-hidden` and `.tree-badge-unavailable` badges (FR-011) ·
  frontend/overseer/src/overseer.js, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 17 to finish, then:**

**Wave 18 — single task:**

- [x] **T040** [US2] Command form lists:
  - `#commandEffects` «ПРИ ОДОБРЕНИИ» from the authored command, with the transition-only case
    (FR-008);
  - `#commandRules` «ЧТО ВИДЯТ ИГРОКИ» grouped by target, with «ВИДЯТ ИГРОКИ», «ПЕРЕКРЫВАЕТ», and
    `aria-busy` (FR-009, FR-010);
  - «ИЗМЕНИТЬ» opens the owning editor, so a rule edited here and on ОБЪЕКТЫ is the same edit
    (FR-013).

  · frontend/overseer/src/overseer.js, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 18 to finish, then:**

**Wave 19 — single task:**

- [x] **T041** [US2] Add «СБРОСИТЬ СЦЕНУ»:
  - `#sceneResetDialog` with counts, the players-stay sentence, and the pending-request outcome;
  - `#btnConfirmSceneReset` calls `resetScene` with the counted revisions;
  - a stale result refreshes the counts, and a failure shows its message;
  - `mergeLiveValues` runs on success.

  Remove `#btnResetFacility`, `#facilityResetDialog`, and the per-terminal «СБРОСИТЬ ВСЕ
  СОСТОЯНИЯ» (FR-014, FR-015) · frontend/overseer/src/index.html,
  frontend/overseer/src/overseer.js

**⟶ Wait for Wave 19 to finish, then:**

**Wave 20 — single task:**

- [x] **T042** [US3] Preview dialog (R13):
  - preselect the first different state, or the opposite fault activity;
  - a one-state device shows «НЕЧЕГО СРАВНИВАТЬ» with no request;
  - `#facilityPreviewCurrent` and `#facilityPreviewHypothetical` side by side, stacking when
    narrow;
  - `#facilityPreviewSummary` with counted kinds and matching tree marks;
  - send only after the save queue is idle, with `expectedAuthoringRevision` and a retry on stale;
  - the new `#facilityPreviewContext` copy;
  - remove the old self-comparison code.

  (FR-016–FR-020) · frontend/overseer/src/index.html, frontend/overseer/src/overseer.js,
  frontend/overseer/src/overseer.css

**Checkpoint**: `task test`, `task test:race`, `task vet`, `task frontend:build`, and
`task bindings:check` pass. Every server operation and the whole Overseer exist. Existing journeys
that use removed UI are fixed in their story phases.

---

## Phase 3: User Story 1 — Edit both tabs without losing work (P1, MVP)

**Goal**: one save model, one vocabulary, a shared status, undo and redo, and unapplied input
that survives.

**Independent Test**: spec US1 Independent Test on a demo copy (quickstart step 1).

**Files**:

- **New journey**: `tests/browser/facility-unified-save.spec.mjs`
- **Existing journeys**: `tests/browser/facility-authoring.spec.mjs`,
  `facility-authoring-ux.spec.mjs`, `facility-p0-command-actions.spec.mjs`,
  `facility-p0-fault-activity.spec.mjs`, `facility-p0-identifiers.spec.mjs`,
  `facility-p0-visible-fields.spec.mjs`, `state-changing-command-authoring.spec.mjs`,
  `state-changing-command-sync.spec.mjs`, `terminal-navigation.spec.mjs`

### Tests

**Wave 1 — single task:**

- [x] **T043** [US1] Write the US1 journey:
  - acceptance scenarios 1–8;
  - SC-001: ten alternating edits across tabs, none lost, no stale or draft message; two quick
    edits where the second is invalid: the first is saved, and the second reopens with its input;
  - a burst of five edits never has more than one `SaveAuthoring` call in flight (fixture call log);
  - SC-002: identical status on both tabs at every step;
  - SC-003: «ГОТОВО» on every facility dialog, and the removed-copy sweep over the DOM;
  - FR-005 control reasons;
  - edge cases: a live change while editing, a stale rejection then «ПРИМЕНИТЬ СНОВА», undo that
    no longer applies, open clearing the history, unapplied input across tabs;
  - FR-001c: a player sees an applied edit at once, with hacking kept.

  · tests/browser/facility-unified-save.spec.mjs

### Existing journeys

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent (different files):**

- [x] **T044** [P] [US1] Replace the draft save and discard steps with autosave and the shared
  status, «ГОТОВО» labels, and the new preview context copy ·
  tests/browser/facility-authoring.spec.mjs
- [x] **T045** [P] [US1] Replace the guide's draft actions with «ОТКРЫТЬ ТЕРМИНАЛЫ» and
  autosave expectations · tests/browser/facility-authoring-ux.spec.mjs
- [x] **T046** [P] [US1] Replace the draft-guard journey with "no guard; the edit saves" (FR-006) ·
  tests/browser/facility-p0-command-actions.spec.mjs
- [x] **T047** [P] [US1] Update the dialog labels, and replace «СБРОСИТЬ ВЕСЬ ОБЪЕКТ» with
  «СБРОСИТЬ СЦЕНУ» where starting values are restored · tests/browser/facility-p0-fault-activity.spec.mjs
- [x] **T048** [P] [US1] Update the dialog labels and save steps ·
  tests/browser/facility-p0-identifiers.spec.mjs
- [x] **T049** [P] [US1] Extend the label sweep to the shared bar and the scene-reset dialog, and
  drop the removed buttons · tests/browser/facility-p0-visible-fields.spec.mjs
- [x] **T050** [P] [US1] Replace `#btnPublish` with "a player sees the applied edit at once" ·
  tests/browser/state-changing-command-authoring.spec.mjs
- [x] **T051** [P] [US1] Replace the publish step and `resetTerminalCommandStates` with
  autosave publication and `ResetScene` · tests/browser/state-changing-command-sync.spec.mjs
- [x] **T052** [P] [US1] Assert `SaveAuthoring` calls instead of `SaveSession` for the
  state-change, transition, and ordinary command saves · tests/browser/terminal-navigation.spec.mjs

**Checkpoint**: US1 works and is verified on its own. `task browser:test` passes for these ten
specs.

---

## Phase 4: User Story 2 — See everything a command changes (P1)

**Goal**: both command lists with marked winners, player names in the tree, and one scene reset.

**Independent Test**: spec US2 Independent Test (quickstart steps 3–4).

**Files**: `tests/browser/facility-command-lists.spec.mjs`

### Tests

**Wave 1 — single task:**

- [x] **T053** [US2] Write the US2 journey on the demo command «Открыть выход и подать сигнал»:
  - acceptance scenarios 1–6;
  - both lists, with «ВИДЯТ ИГРОКИ» and «ПЕРЕКРЫВАЕТ» for the name, the blocks, and power-grid
    availability;
  - SC-004 breadth: for every state-changing demo command, the «ПРИ ОДОБРЕНИИ» items equal its
    authored transitions or program, completed name, and rewritten block;
  - FR-013: a rule edited from the command form appears on ОБЪЕКТЫ;
  - edge cases: legacy-only and device-only commands, a state text over a rewritten block, a
    diagnostic substitution, a terminal-transition command;
  - SC-004 player parity on the `facility-player-state` fixture: the tree name and the marked
    winner equal the player's view before and after approval;
  - SC-005: reset counts, players stay on their terminal, and a forced failure leaves everything
    unchanged;
  - the pending-request note.

  · tests/browser/facility-command-lists.spec.mjs

**Checkpoint**: US2 works and is verified on its own.

---

## Phase 5: User Story 3 — Preview shows the difference (P2)

**Goal**: a different preselected state, side-by-side views, named differences, and the edited
configuration.

**Independent Test**: spec US3 Independent Test (quickstart step 5).

**Files**: `tests/browser/facility-preview-diff.spec.mjs`

### Tests

**Wave 1 — single task:**

- [x] **T054** [US3] Write the US3 journey:
  - acceptance scenarios 1–6 on «Гермодверь Убежища 76»: «Открыта» preselected, side-by-side
    panes, a summary naming block 3, «Выход уже открыт», and «НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН», and
    matching tree marks;
  - a fault preview with the opposite activity;
  - a one-state device notice;
  - FR-019: an applied text-rule edit appears on reopen, including when the preview opens while a
    save is still in flight;
  - SC-006 over every demo device;
  - FR-020: player, live state, and revisions unchanged after close.

  · tests/browser/facility-preview-diff.spec.mjs

**Checkpoint**: US3 works and is verified on its own.

---

## Phase 6: Polish

**Files**: `sessions/demo-capability-paths.md`, `tests/browser/demo-session.spec.mjs`,
`specs/039-unified-facility-authoring/validation.md`

**Wave 1 — independent (different files):**

- [x] **T055** [P] Add the authoring paths (R17, FR-022): cross-tab autosave with undo and redo,
  both command lists on «Открыть выход и подать сигнал», «СБРОСИТЬ СЦЕНУ» from the shared bar, and
  the preview from «Гермодверь Убежища 76». Remove the draft wording ·
  sessions/demo-capability-paths.md
- [x] **T056** [P] Assert the new inventory entries, and replace removed labels and draft steps in
  the demo journeys · tests/browser/demo-session.spec.mjs

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — single task:**

- [x] **T057** Validate against the Success Criteria:
  1. Run `go fix ./...` and review every edit.
  2. Run `task proto:check`, `task proto:breaking`, `task bindings:check`, `task fmt:check`,
     `task vet`, `task lint`, `task test`, `task test:race`, `task frontend:build`,
     `task browser:test`, and `task package`.
  3. Run an `rg` sweep confirming that no removed method name or copy remains outside `specs/`.
  4. Repeat `quickstart.md` interactive steps 1–5 and the compatibility check in the packaged app
     on a demo copy.
  5. Record every command, result, and anything `NOT RUN`, mapped to SC-001–SC-007.

  · specs/039-unified-facility-authoring/validation.md

---

## Dependencies & Execution Order

- **Phases**: Setup → Foundational → US1, US2, US3 (independent of each other; disjoint files)
  → Polish. Polish waits for all three stories.
- **Setup**: Wave 1 (T001) blocks only the story journeys.
- **Foundational**:
  - W1 (T002–T004) → W2 tests (T005–T010) → W3 live and session (T011–T014) → W4 coordinator
    (T015–T017) → W5 adapters (T018–T019) → W6 bridge (T020) → W7 removals (T021–T026).
  - Then W8 proto cutover (T027) → W9 bindings, scripts, and fixtures (T028–T031) → W10 desktop
    API (T032) → W11–W20 Overseer, one task per wave, in order T033 → T042.
- **US1**: W1 journey (T043) → W2 existing journeys (T044–T052, parallel).
- **US2**: W1 journey (T053).
- **US3**: W1 journey (T054).
- **Polish**: W1 (T055–T056, parallel) → W2 validation (T057).
- **Parallel opportunities**: Foundational W2, W3, W4, W7, and W9; the three story phases with
  each other; US1 W2; Polish W1.

---

## Phase 7: Convergence

- [x] **T058** [US1] Make the single-command reset `#btnResetCommandState` say why it cannot run.
  While another session-state command is pending, give it `aria-disabled="true"`, keep it focusable,
  and show «ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА» in an `aria-describedby` `.control-reason`, through
  `controlBlockReason`/`explainControl`. Today it gets a plain `disabled` with no reason. Extend the
  FR-005 journey to cover it, per FR-005, US1/AC4 (partial) · frontend/overseer/src/overseer.js,
  tests/browser/facility-unified-save.spec.mjs
- [x] **T059** [US2] Add a browser case for the «ЧТО ВИДЯТ ИГРОКИ» visibility target. On a demo copy,
  author a state-based visibility rule on a command through the binding dialog. Assert that
  `#commandRules section[data-target="visibility"]` lists it with its condition and «ВИДЯТ ИГРОКИ».
  With a fault that forces the command visible active, assert that `diagnostic-forced-visibility`
  outranks it, is marked «ВИДЯТ ИГРОКИ», and shows «ПЕРЕКРЫВАЕТ», per Verification Requirements
  (browser: "including visibility and availability"), FR-009, FR-010 (partial) ·
  tests/browser/facility-command-lists.spec.mjs
- [x] **T060** [US2] Leave cancelled rules out of the «ПЕРЕКРЫВАЕТ» marks in `renderCommandRules`.
  Two or more holding `state-name` or `state-text` variants, or two or more active
  `diagnostic-substitution` rules, of one target cancel each other (`data-model.md`). They must not
  be labelled as overriding while a lower rule is marked «ВИДЯТ ИГРОКИ». Add a browser or fixture
  case with two matching name variants, per FR-010, US2/AC3 (partial) ·
  frontend/overseer/src/overseer.js, tests/browser/facility-command-lists.spec.mjs
- [x] **T061** [US3] Mark every difference kind of a node in both preview panes, not only the last
  one. Replace the one-kind-per-node map so that, for example, a command that is renamed and becomes
  unavailable carries both badge words and both `data-kind` values, matching the summary. Cover it
  in the preview journey or with a fixture device, per FR-018, US3/AC4 (partial) ·
  frontend/overseer/src/overseer.js, tests/browser/facility-preview-diff.spec.mjs
- [x] **T062** [US1] Give the entry-block command dialog (`commitEntryBlockCommandEdit`) and the
  create-terminal dialog a `reopen` callback. When the server rejects the edit, the dialog reopens
  with the author's input and shows the reason in its error element, as the facility dialogs already
  do. Add one rejection case to the US1 journey, per T033, US1/AC6 (partial) ·
  frontend/overseer/src/overseer.js, tests/browser/facility-unified-save.spec.mjs
- [x] **T063** After T058–T062, repeat `quickstart.md` interactive steps 1–5 and the compatibility
  check in the packaged app (`task package`) on a demo copy, with two player browsers for step 2.
  Rerun the browser specs touched above and record every result, replacing the NOT RUN entry, per
  Verification Requirements (interactive), T057 step 4 (missing) ·
  specs/039-unified-facility-authoring/validation.md
