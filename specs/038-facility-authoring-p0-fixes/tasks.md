# Tasks: Facility Authoring P0 Fixes

**Input**: `specs/038-facility-authoring-p0-fixes/` — spec.md, plan.md, research.md, data-model.md,
contracts/overseer-ui.md, quickstart.md

**Tests**: Required by the spec (Verification Requirements) and constitution. Each story's browser
spec is written first and must fail before its implementation lands.

**Shared file**: `frontend/overseer/src/overseer.js` is the single Overseer module. The plan keeps
it whole, so US1, US2, and US3 each edit their own functions in it (listed per phase). Those story
phases run one after another on one worker; their test files stay disjoint.

## Phase 1: Setup

- [x] **T001** Run the baseline `task frontend:build` and `task browser:test -- facility-authoring.spec.mjs facility-authoring-ux.spec.mjs facility-diagnostics.spec.mjs facility-lifecycle.spec.mjs demo-session.spec.mjs` with Node from `.nvmrc`; note pre-existing failures in the task close note · (no file)

## Phase 2: Foundational

Files: `frontend/overseer/src/overseer.css`, `tests/browser/helpers/overseer-forms.mjs`

**Wave 1 — independent (different files):**

- [x] **T002** [P] Add `[hidden] { display: none !important; }` at the top, plus the shared styles `.facility-row-error`, `#commandModeDecision`, `#commandDraftGuard`, and the fault starting-activity radio group, per contracts/overseer-ui.md (FR-019, FR-020) · frontend/overseer/src/overseer.css
- [x] **T003** [P] Create shared helpers: open a fixture or demo session on ОБЪЕКТЫ/ТЕРМИНАЛЫ, and `expectNoOrphanLabels(scope)` that asserts every visible `label`, `legend`, and `.field-label-inline` has a visible control (`for` target, wrapped control, or next control), and that help text referenced by a hidden control's `aria-describedby` or placed directly after it is hidden too · tests/browser/helpers/overseer-forms.mjs

**Checkpoint**: Global hidden rule and test helpers exist; story work can start.

## Phase 3: User Story 1 — Change a command's mode without losing its device action (P1)

**Goal**: No device action disappears or gets invented texts without an explicit author choice.

**Independent Test**: Spec US1 independent test on «Открыть выход и подать сигнал» plus attaching an action to an ordinary command.

Files: `tests/browser/facility-p0-command-actions.spec.mjs`, `tests/browser/facility-authoring-ux.spec.mjs`; in `overseer.js`: `renderNodeForm` command section, the `btnDiscardFacility` handler, `installFacilityBindingControls`, `updateFacilityBindingMode`, and the `facilityBindingForm` submit handler

### Tests

**Wave 1 — independent (different files):**

- [x] **T004** [P] [US1] On the demo session seeded through the fixture save (as in `facility-authoring-ux.spec.mjs`), using «Открыть выход и подать сигнал» (`n_cmd_open_evacuation`) and the ordinary command «Проверить наружные датчики» (`n_cmd_ordinary_1`), journeys for acceptance scenarios 1–7: inline decision lists both transitions and disables ПРИМЕНИТЬ; keep restores; reselecting the original mode dismisses; remove + apply saves without the action; device-action block hidden outside state-changing mode; draft guard with save and discard; ordinary command in the binding dialog requires both texts and never prefills them; the executed command `n_cmd_prime_grid` (with a device action) keeps its mode locked and shows no decision notice (FR-001…FR-007) · tests/browser/facility-p0-command-actions.spec.mjs
- [x] **T005** [P] [US1] Update existing journeys that attach an action or program to an ordinary command so they fill НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ and ЗАПРОС ПОДТВЕРЖДЕНИЯ (FR-005) · tests/browser/facility-authoring-ux.spec.mjs

### Implementation

**⟶ Wait for the US1 tests to fail, then (same file, in order):**

- [x] **T006** [US1] Extract the `btnDiscardFacility` handler body into `discardFacilityDraft()` and use it from the handler (FR-007) · frontend/overseer/src/overseer.js
- [x] **T007** [US1] In `renderNodeForm`, derive the effective device action (draft node when a draft exists, else saved node); render `.command-facility-action` only while the selector is «ИЗМЕНЯЕТ СОСТОЯНИЕ»; for a command not yet saved in that mode show «ПРИМЕНИТЕ РЕЖИМ, ЧТОБЫ ДОБАВИТЬ ДЕЙСТВИЕ УСТРОЙСТВ» and disable `#btnEditCommandFacilityAction` (FR-004) · frontend/overseer/src/overseer.js
- [x] **T008** [US1] Add `#commandModeDecision` below `#fldCommandMode` with `#btnKeepStateChangeMode` and `#btnRemoveDeviceAction`; disable `#btnApplyNode` while pending; reselecting the original mode dismisses; "remove" only records the decision and the existing apply path saves without `stateChange` (FR-001, FR-002, FR-003, FR-006) · frontend/overseer/src/overseer.js
- [x] **T009** [US1] Add `#commandDraftGuard` when `facilityDraftDirty` and an effective action exists: lock `#fldCommandMode` and `#btnApplyNode`, offer `#btnSaveFacilityDraftFromCommand` (`saveFacilityCandidate`) and `#btnDiscardFacilityDraftFromCommand` (`discardFacilityDraft`), re-render unlocked after success, keep the guard with the facility error on failure (FR-007) · frontend/overseer/src/overseer.js
- [x] **T010** [US1] In the binding dialog add `#facilityBindingCompletedName` and `#facilityBindingConfirmationText` to `#facilityBindingAction`, shown and required only for a command without `stateChange` in the draft, refreshed on node change; submit writes both texts with the action; delete the `stateChange ||= {…}` fallback (FR-005) · frontend/overseer/src/overseer.js

**Checkpoint**: US1 journeys pass; no silent removal or invented texts remain.

## Phase 4: User Story 2 — Create the guide's door with Russian names (P1)

**Goal**: Russian names produce readable, unique identifiers and precise row-level errors.

**Independent Test**: Spec US2 independent test: «Дверь в убежище» with «Заперта», «Открыта», «Открыть дверь», saved and reopened without touching identifier fields.

Files: `tests/browser/facility-p0-identifiers.spec.mjs`; in `overseer.js`: `stableFacilityID` and its five callers, `appendFacilityStateRow`, `appendFacilityTransitionRow`, `validateFacilityDeviceDraft`, the device-form submit handler, and the fault and program submit messages

### Tests

- [x] **T011** [US2] Journeys: `device-dver-v-ubezhishche`, `zaperta`, `otkryta`, `otkryta-2`, `otkryt-dver`; «★★★» → `state-1`; `condition-…` and `program-…` suggestions from Russian names; renaming an existing entity keeps its ID; «Ґанок» → `ganok`, «Café» → `cafe`, a 300-character name stays within 256 characters and unique; an empty third state name marks only that row, names the problem, and receives focus; with two invalid rows both show messages and the first in reading order receives focus; save and reopen keep IDs (FR-008…FR-013) · tests/browser/facility-p0-identifiers.spec.mjs

### Implementation

**⟶ Wait for T011 to fail, then (same file, in order):**

- [x] **T012** [US2] Replace `stableFacilityID` with `suggestFacilityID`: lower-case, FR-008 table before `NFKD`, strip `\p{M}`, collapse to `[a-z0-9-]`, apply prefix rules, check collisions on final values with `-2…`, cap at 256, numbered fallback; update the device, state, transition, fault, and program callers (FR-008…FR-012) · frontend/overseer/src/overseer.js
- [x] **T013** [US2] Row-level validation in the device dialog: per-row `.facility-row-error` with `aria-invalid` and `aria-describedby`, summary «ИСПРАВЬТЕ ОТМЕЧЕННЫЕ СТРОКИ: N» in `#facilityDeviceError`, focus on the first offending control, messages cleared when the row changes; fault and program messages name their field (FR-013) · frontend/overseer/src/overseer.js

**Checkpoint**: The guide's Russian door saves on the first attempt.

## Phase 5: User Story 3 — Start a scene with a fault already active (P1)

**Goal**: Starting activity of a fault is authorable and visible beside current activity.

**Independent Test**: Spec US3 independent test: new fault active at start, saved, then both resets; an existing demo fault shows and edits its starting activity.

Files: `frontend/overseer/src/index.html`, `tests/browser/facility-p0-fault-activity.spec.mjs`; in `overseer.js`: `openFacilityConditionDialog`, the `facilityConditionForm` submit handler, and the fault rows and summary in `renderFacilityWorkspace`

### Tests and markup

**Wave 1 — independent (different files):**

- [x] **T014** [P] [US3] Journeys: new fault with НЕИСПРАВНОСТЬ АКТИВНА reads «СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: АКТИВНА» before and after save; device and facility reset keep it active; editing an existing fault preselects its stored value and changing it leaves current activity unchanged; a saved session whose fault has no `initialActive` field opens with ИСПРАВНО preselected (FR-014…FR-018, FR-021) · tests/browser/facility-p0-fault-activity.spec.mjs
- [x] **T015** [P] [US3] Add the fieldset «В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА» with radios `#facilityConditionInitialWorking` (default) and `#facilityConditionInitialActive`, plus the help text from the contract (FR-014) · frontend/overseer/src/index.html

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T016** [US3] Preselect the radios on open; on submit set `initialActive`, and for a new fault set `currentActive` to the same value; show «СЕЙЧАС: … · В НАЧАЛЕ: …» in fault rows and `#facilitySelectedConditionSummary` (FR-014…FR-017) · frontend/overseer/src/overseer.js

**Checkpoint**: A scene can start with an active fault without editing JSON.

## Phase 6: User Story 4 — Forms show only the fields that apply (P1)

**Goal**: No visible label without its control anywhere in Overseer.

**Independent Test**: Spec US4 independent test: every dialog, every mode.

Files: `tests/browser/facility-p0-visible-fields.spec.mjs`

- [x] **T017** [US4] Sweep with `expectNoOrphanLabels`: `facilityBindingDialog` in all five kinds, `facilityConditionDialog` in both scopes, then `facilityDeviceDialog`, `facilityRecoveryProgramDialog`, `facilityPreviewDialog` (device and condition), `facilityDependencyDialog`, `facilityRepairDialog`, `facilityDeviceResetDialog`, `facilityResetDialog`, `facilityRecoveryConfirmationDialog`, `terminalGroupDraftDialog`, `terminalGroupImpactDialog`, `logicalSessionDialog`, `playerManagementDialog`, `playerDeleteDialog`, `createTerminalDialog`, `takeOffAirDialog`, `endBroadcastDialog`, `terminalSwitchDialog`, `publicAccessSettingsDialog`, `publicAccessProviderTokenDialog`, `publicAccessPlayerCredentialsDialog`, `entryBlockCommandDialog`, `generatedPasswordDialog`, `applicationUpdateDialog`, `applicationUpdateRestartDialog`, and `resetConfirmationDialog`; a dialog the fixture cannot open is listed as `NOT RUN` in validation.md; the global rule from T002 makes it pass (FR-019, FR-020) · tests/browser/facility-p0-visible-fields.spec.mjs

**Checkpoint**: The label sweep passes across every dialog and mode.

## Phase 7: Polish

Files: `sessions/demo-capability-paths.md`, `specs/038-facility-authoring-p0-fixes/validation.md`

**Wave 1 — independent (different files):**

- [x] **T018** [P] Document the mode-change decision, the dirty-draft guard, the texts requested for ordinary commands, and fault starting activity in «Prepare a story with the in-app guide» and «Inspect and maintain existing scenes» (FR-022) · sessions/demo-capability-paths.md
- [x] **T019** [P] Validate against Success Criteria: `task frontend:build`; `task browser:test` for the four `facility-p0-*.spec.mjs` files and the existing facility and demo suites; `task fmt:check`, `task vet`, `task test`, `task lint`; record commands and results, marking unavailable checks `NOT RUN` (SC-001…SC-005, FR-021) · specs/038-facility-authoring-p0-fixes/validation.md

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T020** `task package`, launch the packaged app with a demo copy, run quickstart steps 1–6, save the untouched demo copy and diff identifiers, modes, actions, and activities; append results (SC-001…SC-005, FR-021) · specs/038-facility-authoring-p0-fixes/validation.md

## Dependencies & Execution Order

- Setup (T001) → Foundational (T002, T003) → US1 → US2 → US3 → US4 → Polish.
- US1, US2, US3 run sequentially because they edit `overseer.js`; US4 only adds a test and could run right after Foundational.
- Foundational: one wave (T002 ∥ T003).
- US1: tests wave (T004 ∥ T005) blocks T006 → T007 → T008 → T009 → T010.
- US2: T011 blocks T012 → T013.
- US3: wave (T014 ∥ T015) blocks T016.
- US4: T017 alone.
- Polish: wave (T018 ∥ T019) blocks T020.
