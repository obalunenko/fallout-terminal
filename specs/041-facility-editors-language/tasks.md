# Tasks: Facility Editors and Language

**Input**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/overseer-ui.md`,
`contracts/private-desktop.md`

**Scale note**: About 45 files in four areas. The work is:

- Go: one relaxed domain rule, and the recovery path removed through session, control, the App and the proto.
- The browser bridge and fixtures.
- The single-file Overseer.
- Demo data, plus seven new and fifteen updated browser journeys.

Watch for these:

- **Method count**: the desktop method count drops from 47 to 46, in the Go tests, the bindings script, and `desktop-api.spec.mjs`.
- **Old names and markers**: the journeys locate dialogs by names this feature renames. Every journey that names old UI fails between the Phase 2 checkpoint and its story phase.
- **Demo marker**: the inventory marker «Загрязнён · ● 1» is asserted in four places.

**Structure**: The seven stories share the single-file Overseer (`overseer.js`, `index.html`, `overseer.css`). They also share the bridge, the fixtures and the demo. So, as in 040, every implementation task lives in Phase 2, in story order and tagged by story. Each story phase then owns its new acceptance journey and the existing journeys its behavior changes. Every file has one owner phase.

- Go work is test-first where it adds behavior. Removal tasks delete code and its tests together.
- `go build ./...` compiles at every wave join. The recovery path is removed from callers down to the proto, so no join leaves a dangling reference.
- The Overseer waves (Wave 4 to Wave 12) take one task each, because each touches all three Overseer files. The Overseer stays runnable at every join.
- No browser journey runs before the Phase 2 checkpoint. A story journey that finds a defect in a Phase 2 file reports it back. The fix lands through the main agent, never inside a parallel story worker.

**Format**: `- [ ] **T###** [P?] [US#] Description · file`

---

## Phase 1: Setup

**Files**: `tests/browser/helpers/facility-editors.mjs` (new)

**Wave 1 — single task:**

- [x] **T001** Add the journey helpers shared by the seven story specs. They locate parts by their `contracts/overseer-ui.md` ids. Each helper drives one part:
  - **Device dialog**: open it for a device; read the state chips with their star and the matrix cells (`data-from`, `data-to`, names, summary); open a cell's transition panel.
  - **Fault dialog**: open it; read the «КАК ПОЯВЛЯЕТСЯ» and «КАК УСТРАНЯЕТСЯ» rows.
  - **Rule dialog**: open it on a tab; fill the «КОГДА / ГДЕ / БЛОК / ТЕКСТ» sentence.
  - **«ДЕЙСТВИЕ КОМАНДЫ»**: open it from each opener.
  - **Device list**: read the rows, badges and filter counts; use the «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» group.
  - **Errors**: read a rendered error's message and its «подробности для разработчика» text.

  · tests/browser/helpers/facility-editors.mjs

---

## Phase 2: Foundational (blocks every story)

**Files**:

- **Domain**: `internal/domain/validate.go`, `internal/domain/validate_test.go`
- **Recovery removal**: `app.go`, `app_test.go`, `app_contract.go`, `app_contract_test.go`, `desktop_service.go`, `main.go`, `internal/control/facility.go`, `internal/control/facility_test.go`, `internal/session/facility.go`, `internal/session/facility_test.go`
- **Contract**: `proto/fallout/terminal/private/v1/desktop.proto`, `internal/gen/fallout/terminal/private/v1/`, `proto/schema-revision.txt`, `proto/compatibility-baseline.binpb`, `frontend/overseer/bindings/`, `scripts/wails-bindings-check.sh`
- **Catalog sync tests**: `internal/playerconfig/messages_test.go` (new), `internal/session/service_messages_test.go` (new)
- **Bridge and fixtures**: `frontend/overseer/src/desktop-api.js`, `tests/browser/desktop-api.spec.mjs`, `tests/browser/fixtures/desktop-bindings.js`, `tests/browser/fixture-server/main.go`
- **Overseer**: `frontend/overseer/src/overseer.js`, `frontend/overseer/src/index.html`, `frontend/overseer/src/overseer.css`
- **Glossary**: `docs/overseer-glossary.md` (new)
- **Demo**: `sessions/demo.json`, `sessions/demo-capability-paths.md`, `internal/platform/assets_test.go`, `tests/browser/demo-session.spec.mjs`

**Wave 1 — independent (different files):**

- [x] **T002** [P] [US2] Add validation table cases. Accepted:
  - a fault with a capability block and no `recovery` entry;
  - a fault whose entries are all valid and none of which clears it.

  Still rejected:
  - `privateOverseerAction: false`;
  - an entry naming a missing transition or program;
  - a `transition` entry whose transition lacks `recovery: true`.

  The new accept cases fail until T010 (FR-020, FR-042, `contracts/private-desktop.md` «Validation change») · internal/domain/validate_test.go
- [x] **T003** [P] [US7] Write the player-config catalog sync test. Collect every literal passed to `failure(…)` in `service.go`, plus `errStorageUnavailable`'s message, with `go/parser`. Read `frontend/overseer/src/overseer.js` as text. Fail when:
  - a text has no `'player-config'` `byText` entry;
  - an entry matches no text;
  - a key sits in both areas.

  It fails until T012 (FR-039, R12) · internal/playerconfig/messages_test.go
- [x] **T004** [P] [US7] Write the same sync test for the session service. It collects literals passed to `sessionFailure(…)` and the last argument of `saveFailure(…)` in `service.go`, checked against the `'session'` area. It fails until T012 (FR-039, R12) · internal/session/service_messages_test.go
- [x] **T005** [P] [US6] Write the glossary as `data-model.md` «Glossary» defines it. It covers each term with its meaning, its banned synonyms, the allowed «ОБЪЕКТЫ» tab and «ОБЪЕКТЫ И СОСТОЯНИЯ» heading, and the case convention (R15) (FR-034) · docs/overseer-glossary.md
- [x] **T006** [P] [US2] Remove the private recovery path layer by layer, with each symbol's tests, per `contracts/private-desktop.md` «Go removals by layer» and «Bridge allowlists». Cover the root package (`App.RecoverFacilityCondition`, payload, converters, audit allowlists, desktop method), the `main.go` adapter mapping, control (`RecoverFacilityCondition` and its helpers, audit action and event), and the session `ApplyWorldAction` recovery branch with its `WorldActionRequest` fields. The pinned method list drops to 46. Remove every helper the removal leaves without a caller (FR-020, R1) · app.go, app_test.go, app_contract.go, app_contract_test.go, desktop_service.go, main.go, internal/control/facility.go, internal/control/facility_test.go, internal/session/facility.go, internal/session/facility_test.go
- [x] **T007** [P] [US7] Update the browser bridge and fixtures per `contracts/private-desktop.md` «Bridge allowlists» and «Browser fixture hooks»:
  - **Removed**: `recoverFacilityCondition` and its normalizer and validator branch in `desktop-api.js`, the method from `desktop-api.spec.mjs` (count 46), the shim export and recover routes.
  - **Renamed**: the diagnostics route becomes `set-condition-active`.
  - **Changed**: `record()` returns a queued terminal-action result once, and `next-operation` also applies to `save-authoring`.

  (FR-020, FR-039) · frontend/overseer/src/desktop-api.js, tests/browser/desktop-api.spec.mjs, tests/browser/fixtures/desktop-bindings.js, tests/browser/fixture-server/main.go
- [x] **T008** [P] Add the demo data from R16:
  - The reactor transition «Пуск от резервной сети» (Аварийная остановка → Дежурный режим, precondition «Резервная сеть Убежища 76 = В работе»).
  - A second transition in one recovery program, whose source state holds whenever that program's command can run.

  Stored `recovery[]` values stay unchanged (FR-044, Principle IX) · sessions/demo.json

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent (different files):**

- [x] **T009** [US2] Remove `RecoverFacilityConditionRequest`. Run `task proto:generate` and regenerate the Wails bindings. Regenerate `proto/compatibility-baseline.binpb`, and set `scripts/wails-bindings-check.sh` to 46 methods. `task proto:breaking` and `task bindings:check` must pass (R1, Principle V) · proto/fallout/terminal/private/v1/desktop.proto, internal/gen/fallout/terminal/private/v1/, proto/schema-revision.txt, proto/compatibility-baseline.binpb, frontend/overseer/bindings/, scripts/wails-bindings-check.sh
- [x] **T010** [P] [US2] Drop the "must provide recovery that clears the condition" rule from `validateConditions`. Keep `validateRecoveryReference` unchanged. T002 passes (FR-020, R1) · internal/domain/validate.go

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — single task:**

- [x] **T011** Stop requiring a recovery entry for every demo condition. Pin the new reactor transition and the two-transition program in the demo graph checks. Update any schema pin that T009 moved. `TestBundledDemoVersionOneSaveAndReopenPreservesCompatibleUnknownFields` still passes (FR-042, FR-044) · internal/platform/assets_test.go

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — single task:**

- [x] **T012** [US7] Add `renderOverseerError(sink, result, area)` and `OVERSEER_ERROR_MESSAGES` per `contracts/overseer-ui.md` «Errors».
  - **Catalog**: specific `player-config` and `session` messages, one general message per other area, and the «подробности для разработчика» disclosure.
  - **Sinks**: replace every `result?.error || '…'` sink.
  - **Rewording**: reword `facilityFailureMessages` to the glossary, including the `stale-revision` text that replaces «СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ».
  - **Validation problems**: make `facilityIssueMessage` name entities by kind label and name, with raw kinds and IDs under the disclosure.
  - **Helper**: give `facilityTechnicalDetails` a summary parameter.

  T003 and T004 pass (FR-038 to FR-041, R12) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — single task:**

- [x] **T013** [US1] Rebuild the device dialog per `contracts/overseer-ui.md` «Device dialog»:
  - **Order**: name, type, the template line placeholder, state chips with «★» and the inline state editor, the matrix of cell buttons, `#facilityTransitionPanel`, and «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ» holding every identifier input.
  - **Removed**: `#facilityDeviceInitialState`. The first state of a new device is starred.
  - **Transition panel**: the precondition device list leaves out the transition's own device and devices already used. Fault effects show read-only with «К НЕИСПРАВНОСТИ», which is blocked while the draft is unsaved. `appendFacilityTransitionConditionEffect` is removed.
  - **State deletion**: follows R10. The starting and current states are refused, and outside uses block deletion.
  - **Drafts**: rows keep `facilityOriginal`.
  - **Links**: dependency links for transitions open the transition panel. The title is «УСТРОЙСТВО».

  (FR-001 to FR-008, FR-017, FR-043, R8, R10) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 5 to finish, then:**

**Wave 6 — single task:**

- [x] **T014** [US3] Add `FACILITY_DEVICE_TEMPLATES` with the nine templates from `contracts/overseer-ui.md` «Template». Wire the template line, which names the states and transitions and offers «ПРИМЕНИТЬ ШАБЛОН» for every type except «ДРУГОЙ». Applying adds missing states and transitions by trimmed, case-insensitive name, sets the starting state only when there is none, and generates IDs with `suggestFacilityID`. A type change leaves the states alone (FR-009 to FR-012, R9) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 6 to finish, then:**

**Wave 7 — single task:**

- [x] **T015** [US2] Rebuild the fault and program dialogs per `contracts/overseer-ui.md` «Fault dialog» and «Recovery program dialog».
  - **Fault fields**: the title «НЕИСПРАВНОСТЬ», name, category, where it acts, what it blocks, and its position at game start.
  - **Derived lists**: «КАК ПОЯВЛЯЕТСЯ» with its warning and «+ ВКЛЮЧАТЬ ПРИ ПЕРЕХОДЕ…». «КАК УСТРАНЯЕТСЯ» with clearing transitions, programs, «Смотритель вручную», add and remove, and «+ ПРОГРАММА ВОССТАНОВЛЕНИЯ…». Both derive from `conditionEffects` and never offer a transition for both lists.
  - **Removed from the fault dialog**: the operator-recovery checkbox and the "УКАЖИТЕ ДОСТУПНЫЙ ПУТЬ ВОССТАНОВЛЕНИЯ" check.
  - **Fault save**: one `mutate` that writes the condition and every touched transition and prunes invalidated `recovery[]` entries (R2, R3).
  - **Program from a fault**: stacked over the fault dialog, prefilled from the draft, and saved as its own edit (R4).
  - **Program dialog**: one uniform list of rows built from `appendFacilityActionRequest` that respects one transition per device. It names the commands that run the program and warns when there are none. The first-transition fields and the instruction paragraph are removed.

  (FR-013 to FR-018, FR-020, FR-043) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 7 to finish, then:**

**Wave 8 — single task:**

- [x] **T016** [US2] Add «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» after the device list per `contracts/overseer-ui.md` «Terminal faults». It lists every terminal-scoped fault, with its terminal, an active mark, an open button and a `role="switch"` that calls `runFacilityFaultSwitch`. It has `#facilityTerminalFaultStatus`, and search and filters do not narrow it. Remove the «ВЫБРАННОЕ УСЛОВИЕ» panel, «ВОССТАНОВИТЬ УСЛОВИЕ», its confirmation dialog and its block reason. Every fault row in the «НЕИСПРАВНОСТИ» tab opens the fault dialog (FR-019 to FR-021, R1, R11) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 8 to finish, then:**

**Wave 9 — single task:**

- [x] **T017** [US4] Rebuild the binding dialog as «ПРАВИЛО ОТОБРАЖЕНИЯ» per `contracts/overseer-ui.md` «Display rule dialog»:
  - **Tabs**: four keyboard-navigable tabs that replace the type select. An existing rule's other tabs are blocked with a reason.
  - **Sentence**: «КОГДА» (device = state), then «ГДЕ» (one list across terminals, filtered by type, labelled by path), then «БЛОК» («<n> · «<opening text>»»), then «ТЕКСТ».
  - **Hint**: the line about the other states.
  - **Conflicts**: checked for another device, a repeated state, and existing visibility or availability, with «ОТКРЫТЬ ПРАВИЛО».
  - **Save**: a variant is replaced in place.
  - **Removed**: the `command-action` kind and the terminal select.

  (FR-022 to FR-028, R5, R6) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 9 to finish, then:**

**Wave 10 — single task:**

- [x] **T018** [US4] Add `#facilityCommandActionDialog` «ДЕЙСТВИЕ КОМАНДЫ» per `contracts/overseer-ui.md` «Command action dialog».
  - **Command**: fixed when opened from the command form; picked from commands only when opened from the «ЧТО ДЕЛАЮТ ИГРОКИ» tab, the guide's action step or a list row.
  - **Action**: «ПЕРЕХОДЫ» or «ПРОГРАММА ВОССТАНОВЛЕНИЯ».
  - **Player texts**: shown when the command has no `stateChange` yet.
  - **Openers**: rewire `openFacilityCommandActionEditor`, `#btnAddFacilityActionBinding`, the guide step and the list rows. A program-backed action opens here. `#btnEditCommandFacilityAction` is enabled for every command.

  (FR-029, FR-043, R7) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 10 to finish, then:**

**Wave 11 — single task:**

- [x] **T019** [US5] Rebuild the device list per `contracts/overseer-ui.md` «Device list»:
  - **Rows**: left-aligned, with the name and the type under it, `.facility-state-badge` with `data-changed`, and the red `.facility-fault-mark` with its count. The existing data attributes stay.
  - **Search and filters**: search by name and state, with the placeholder «Название или состояние». `#facilityDeviceFilters` holds «ВСЕ», «ИЗМЕНЁННЫЕ» and «С НЕИСПРАВНОСТЬЮ» with counts, kept in module state.
  - **Empty and hidden states**: the hidden-selection line, and the empty state with «ВСЕ».
  - **Hover**: `row.title = id` is removed from `facilityListRow`.

  (FR-030 to FR-033, R11) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 11 to finish, then:**

**Wave 12 — single task:**

- [x] **T020** [US6] Apply `contracts/overseer-ui.md` «Renamed copy» across the Overseer.
  - **Removed wordings**: «объект» naming a device or the location, «диагностическое условие», «условие» naming a fault, and «Восстановление оператором».
  - **Guide**: its first step becomes «УСТРОЙСТВО». «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ» replaces the guide's five device-dialog labels.
  - **Dependencies**: rebuild the dialog as «ЗАВИСИМОСТИ УСТРОЙСТВА». `describeFacilityDependency` writes each link in words with its place by name. The destination buttons read «К КОМАНДЕ», «К НЕИСПРАВНОСТИ», «К ПРОГРАММЕ», «К УСТРОЙСТВУ» and «К ПРАВИЛУ ОТОБРАЖЕНИЯ». Transition sources match the full `device/transition` id, and the path sits under «подробности для разработчика».

  (FR-034 to FR-037, R13, R14) · frontend/overseer/src/overseer.js, frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 12 to finish, then:**

**Wave 13 — single task:**

- [x] **T021** Rewrite the inventory routes and markers to the new names. That includes the badge text that replaces «Загрязнён · ● 1» and «ШАГ 1 ИЗ 5: УСТРОЙСТВО». Add rows with exact routes for every FR-044 case:
  - templates;
  - the two-transition cell;
  - each rule type;
  - the action dialog from the command form;
  - a fault with and without an activating transition;
  - the two-transition program;
  - the terminal-fault group;
  - each link destination;
  - each filter with its demo count.

  Update `demo-session.spec.mjs` markers, the recovery-kind expectation, and the removed-copy list to match (FR-044, Principle IX) · sessions/demo-capability-paths.md, tests/browser/demo-session.spec.mjs

**Found during implement (runs beside Waves 9 to 13; Go files no other task owns):**

- [x] **T049** [US2] Fix live projection of a fault switched off. At the T016 join, turning a fault off (terminal-scoped from the new group, or device-scoped from the 040 card) saved correctly, but a connected player kept the effect until reconnecting; turning it on reached players live. Reproduce in a Go test first, fix the cause where every caller passes through, and prove it with a connected player in the browser fixture (FR-005 of 040, FR-021) · internal/live/ or internal/control/ (the files the cause lives in), with their tests

**Checkpoint**: every implementation file is final. `go build ./...`, `go test ./...` and `task frontend:build` pass. The journeys owned by the story phases may fail until their phase runs.

---

## Phase 3: User Story 1 — Edit a device's states and transitions on one screen (P1, MVP)

**Goal**: the author reads and edits the whole state graph on one screen, with identifiers out of the way.

**Independent Test**: spec US1 Independent Test on a demo copy at 1440×900.

**Files**: `tests/browser/facility-device-editor.spec.mjs` (new), `tests/browser/facility-p0-identifiers.spec.mjs`, `tests/browser/facility-workspace-layout.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T022** [P] [US1] Write the US1 journey. It covers acceptance scenarios 1–6. It checks SC-001 for every demo device at 1440×900 and SC-002 for a new device. It also covers these edge cases: the two-transition reactor cell, a device with one state, deleting a state with this device's transitions, deleting a state used outside the device, and refusing the starting and current states. A saved transition edit is one «ОТМЕНИТЬ» entry, and a reopened device keeps `transition.recovery` (FR-001 to FR-008, FR-043, FR-045) · tests/browser/facility-device-editor.spec.mjs
- [x] **T023** [P] [US1] Move the identifier journeys to the folded «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ» block. That covers suggestions, transliteration, and the conflict that expands the block and focuses the row. Replace the «ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ» dialog lookup with «НЕИСПРАВНОСТЬ» (FR-007) · tests/browser/facility-p0-identifiers.spec.mjs
- [x] **T024** [P] [US1] Replace the «УСТРОЙСТВО ОБЪЕКТА» dialog lookup with «УСТРОЙСТВО», along with any other reference in this file to the removed device-dialog markup · tests/browser/facility-workspace-layout.spec.mjs

**Checkpoint**: US1 works and is tested on its own.

---

## Phase 4: User Story 2 — Configure a fault in one place (P1)

**Goal**: one «НЕИСПРАВНОСТЬ» dialog says how a fault appears and clears, and every fault can be switched by hand.

**Independent Test**: spec US2 Independent Test on a demo copy.

**Files**: `tests/browser/facility-fault-dialog.spec.mjs` (new), `tests/browser/facility-p0-fault-activity.spec.mjs`, `tests/browser/facility-play-faults.spec.mjs`, `tests/browser/facility-diagnostics.spec.mjs`, `tests/browser/facility-authoring.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T025** [P] [US2] Write the US2 journey. It covers acceptance scenarios 1–6. For every demo fault, «КАК ПОЯВЛЯЕТСЯ» and «КАК УСТРАНЯЕТСЯ» must equal what the session data derives (SC-004). The other checks:
  - A new capability-blocking fault saves with only «Смотритель вручную» (FR-020).
  - A fault edit across two devices is one «ОТМЕНИТЬ» entry (FR-043).
  - A program created from a fault, then canceled, saves nothing.
  - The terminal-fault switch reaches a connected player terminal while a device is selected (FR-021).

  (FR-013 to FR-021, FR-045) · tests/browser/facility-fault-dialog.spec.mjs
- [x] **T026** [P] [US2] Move the start-position and activity journeys to the «НЕИСПРАВНОСТЬ» dialog, without the operator-recovery checkbox. Point the dependency-dialog lookup at «ЗАВИСИМОСТИ УСТРОЙСТВА» · tests/browser/facility-p0-fault-activity.spec.mjs
- [x] **T027** [P] [US2] Replace the «operator recovery does not limit the switch» case with a fault that has no recovery entry. Move the row-text assertions to the new badge and fault mark · tests/browser/facility-play-faults.spec.mjs
- [x] **T028** [P] [US2] Move the escape-hatch journey to `set-condition-active`. It proves that the terminal-fault switch clears a fault while every player capability is blocked (FR-021) · tests/browser/facility-diagnostics.spec.mjs
- [x] **T029** [P] [US2] Update the authoring journey. Faults are created without the checkbox. Clearing uses the switch, not «ВОССТАНОВИТЬ УСЛОВИЕ». The binding-kind list becomes the four rule tabs, dependency buttons get their new names, and `STALE` gets the new `stale-revision` text · tests/browser/facility-authoring.spec.mjs

**Checkpoint**: US2 works and is tested on its own.

---

## Phase 5: User Story 3 — Start a device from its type's template (P2)

**Goal**: one press fills a new device from its type.

**Independent Test**: spec US3 Independent Test.

**Files**: `tests/browser/facility-device-templates.spec.mjs` (new)

### Tests

**Wave 1 — single task:**

- [x] **T030** [US3] Write the US3 journey. It covers acceptance scenarios 1–4 and SC-003 (pick «ДВЕРЬ», «ПРИМЕНИТЬ ШАБЛОН», «ГОТОВО»). Every type except «ДРУГОЙ» offers its template, and the template line names it before it applies. A case-only name difference counts as present (FR-009 to FR-012, FR-045) · tests/browser/facility-device-templates.spec.mjs

**Checkpoint**: US3 works and is tested on its own.

---

## Phase 6: User Story 4 — Write a display rule as a sentence (P2)

**Goal**: rules read «когда — где — что», and command actions have their own dialog.

**Independent Test**: spec US4 Independent Test with a connected player terminal.

**Files**: `tests/browser/facility-display-rules.spec.mjs` (new), `tests/browser/facility-p0-command-actions.spec.mjs`, `tests/browser/facility-command-lists.spec.mjs`, `tests/browser/facility-p0-visible-fields.spec.mjs`, `tests/browser/facility-preview-diff.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T031** [P] [US4] Write the US4 journey. It covers acceptance scenarios 1–6 and SC-005, and creates one rule of each type. A connected player terminal is checked in the matching state and in another state. The journey also checks:
  - The three conflicts of FR-028.
  - A fixed tab on an existing rule.
  - An edited variant that keeps its position.
  - A deleted element reported on «ГОТОВО».
  - «ДЕЙСТВИЕ КОМАНДЫ» from the command form, the tab and the guide, including a program-backed action.

  (FR-022 to FR-029, FR-045) · tests/browser/facility-display-rules.spec.mjs
- [x] **T032** [P] [US4] Move the command-action journeys from the binding dialog to «ДЕЙСТВИЕ КОМАНДЫ». That includes the mode-change warning and the player texts (FR-029) · tests/browser/facility-p0-command-actions.spec.mjs
- [x] **T033** [P] [US4] Point the command-list journeys that open the binding dialog at «ДЕЙСТВИЕ КОМАНДЫ» or «ПРАВИЛО ОТОБРАЖЕНИЯ», whichever each one means · tests/browser/facility-command-lists.spec.mjs
- [x] **T034** [P] [US4] Replace the `#facilityBindingKind` selections with the rule tabs. Keep the no-orphan-label check for every tab and for the new dialogs (FR-023) · tests/browser/facility-p0-visible-fields.spec.mjs
- [x] **T035** [P] [US4] Fill the preview-diff rule through the sentence fields and the new ids · tests/browser/facility-preview-diff.spec.mjs

**Checkpoint**: US4 works and is tested on its own.

---

## Phase 7: User Story 5 — Spot changed and broken devices in the list (P2)

**Goal**: changed and faulty devices stand out and can be filtered in one press.

**Independent Test**: spec US5 Independent Test on the demo, then during play.

**Files**: `tests/browser/facility-device-list.spec.mjs` (new), `tests/browser/facility-device-actions.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T036** [P] [US5] Write the US5 journey. It covers acceptance scenarios 1–5 and SC-006 (Все 10, Изменённые 5, С неисправностью 1). It also checks:
  - Live updates after a card transition and a fault switch.
  - The hidden-selection line.
  - That search ignores device IDs.
  - That the filter does not narrow «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ».

  (FR-030 to FR-033, FR-045) · tests/browser/facility-device-list.spec.mjs
- [x] **T037** [P] [US5] Update the row journeys. The row now holds the name, type and badge, not exactly `[STRONG, state]`. The ID-search case and the old empty-conditions text are replaced. Dependency buttons get their new names (FR-030, FR-031) · tests/browser/facility-device-actions.spec.mjs

**Checkpoint**: US5 works and is tested on its own.

---

## Phase 8: User Story 6 — One name for each thing, and links that say where they lead (P2)

**Goal**: one vocabulary across the Overseer, and dependency links that read as sentences.

**Independent Test**: spec US6 Independent Test.

**Files**: `tests/browser/facility-vocabulary.spec.mjs` (new), `tests/browser/facility-authoring-ux.spec.mjs`, `tests/browser/facility-unified-save.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T038** [P] [US6] Write the US6 journey with two parts.
  - **Vocabulary check**: fetch the served `index.html`, `overseer.js` and `desktop-api.js`. Fail on each banned pattern from `docs/overseer-glossary.md`, using Unicode-aware boundaries, with the «ОБЪЕКТЫ» tab and the «ОБЪЕКТЫ И СОСТОЯНИЯ» heading allowed (SC-007). Also cover acceptance scenarios 1, 3 and 4.
  - **Dependency walk**: for each demo device, read every link's words and follow every destination button. Check that the technical path stays hidden until «подробности для разработчика» is expanded (acceptance scenario 2).

  (FR-034 to FR-038, FR-045) · tests/browser/facility-vocabulary.spec.mjs
- [x] **T039** [P] [US6] Move the guide and authoring-UX journeys to the new names, dialogs and controls: «УСТРОЙСТВО», «НЕИСПРАВНОСТЬ», «ПРАВИЛО ОТОБРАЖЕНИЯ», «ЗАВИСИМОСТИ УСТРОЙСТВА», the named destination buttons in place of `/^Открыть ссылку/`, the matrix in place of `.facility-transition-row`, and the guide step «УСТРОЙСТВО» (FR-036, FR-037) · tests/browser/facility-authoring-ux.spec.mjs
- [x] **T040** [P] [US6] Keep the 039 `REMOVED_COPY` sweep, and add the removed recovery controls to it. Replace the «ВОССТАНОВИТЬ УСЛОВИЕ» and «УСЛОВИЕ ВОССТАНОВЛЕНО» cases with the fault switch (FR-020, FR-035) · tests/browser/facility-unified-save.spec.mjs

**Checkpoint**: US6 works and is tested on its own.

---

## Phase 9: User Story 7 — Errors in Russian with a next step (P3)

**Goal**: no error in the Overseer window shows raw English, and each one names a next step.

**Independent Test**: spec US7 Independent Test through the fixture hooks.

**Files**: `tests/browser/overseer-errors.spec.mjs` (new), `tests/browser/public-access-settings.spec.mjs`, `tests/browser/state-changing-command-authoring.spec.mjs`, `tests/browser/facility-play-transitions.spec.mjs`, `tests/browser/facility-play-force.spec.mjs`, `tests/browser/overseer-runtime-logs.spec.mjs`

### Tests

**Wave 1 — independent (different files):**

- [x] **T041** [P] [US7] Write the US7 journey. It covers acceptance scenarios 1–3 and triggers one failure from each area through the hooks in `contracts/private-desktop.md`: player config, session, facility save, public access and update. Each message must be Russian, name a next step, and show no Latin text outside «подробности для разработчика» (SC-008). A failure inside an open dialog keeps what the author typed. A facility validation problem names entities by name (FR-038 to FR-041, FR-045) · tests/browser/overseer-errors.spec.mjs
- [x] **T042** [P] [US7] Replace the English error assertions with the general public-access message plus its disclosure text (FR-041) · tests/browser/public-access-settings.spec.mjs
- [x] **T043** [P] [US7] Replace the English error assertions with the catalog messages · tests/browser/state-changing-command-authoring.spec.mjs
- [x] **T044** [P] [US7] Replace `STALE` with the `stale-revision` message · tests/browser/facility-play-transitions.spec.mjs
- [x] **T045** [P] [US7] Replace `STALE` with the `stale-revision` message · tests/browser/facility-play-force.spec.mjs
- [x] **T048** [P] [US7] Replace the log-access error assertions with the `logs` catalog message, its disclosure text, and `p.log-access-location`. Found at the T012 join: no task owned this file · tests/browser/overseer-runtime-logs.spec.mjs

**Checkpoint**: US7 works and is tested on its own.

---

## Phase 10: Polish

**Files**: `specs/041-facility-editors-language/validation.md` (new)

**Wave 1 — single task:**

- [x] **T046** Run `go fix ./...` and review every edit. Then run `task fmt:check`, `task vet`, `task lint`, `task test`, `task test:race`, `task proto:check`, `task proto:breaking`, `task bindings:check`, `task frontend:build`, `task browser:test` and `task secret-leak:check`. Record each result and the evidence for SC-001 to SC-009 · specs/041-facility-editors-language/validation.md

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — single task:**

- [x] **T047** Build the packaged app (`task package`) and open a demo copy. Check each item and record every result (FR-042, SC-001, SC-009) · specs/041-facility-editors-language/validation.md
  - At 1440×900, open every demo device's dialog and check that the states and the whole matrix are visible without scrolling.
  - With one player browser connected, create a rule of each type and switch a terminal fault.
  - Reopen a session saved before this feature, then save it and check that the file does not change.

---

## Phase 11: Packaged-app recheck follow-ups (2026-11-01)

The 2026-11-01 recheck of the UX review in the packaged app confirmed F-01 to F-14 closed and found
the items below. Each task has its own files.

- [x] **T050** [US6] «ОСТАВИТЬ «ИЗМЕНЯЕТ СОСТОЯНИЕ»» restores the mode by script, which fires no input event, so the status kept «НЕ ПРИМЕНЕНО: Команда …». Restage the form in the handler and assert the unapplied count in the F-03 journey · frontend/overseer/src/overseer.js, tests/browser/facility-p0-command-actions.spec.mjs
- [x] **T051** [US6] Remove internal strings from Overseer copy (FR-035): the root folder reads «ГЛАВНОЕ МЕНЮ» instead of «ROOT»; the broadcast summary names the live terminal and shows no broadcast ID; rule buttons name the rule type instead of its property; the save status has no English `aria-label` and keeps its visible Russian text, while the native-smoke evidence moves to visually hidden text that describes the status (`aria-describedby`). WebKit exposes neither `aria-description` nor a group's described-by text as AXHelp, so the native smoke reads the hidden text's value · frontend/overseer/src/index.html, frontend/overseer/src/overseer.js, scripts/state-changing-reset-native-smoke.sh, docs/overseer-glossary.md, tests/browser/facility-vocabulary.spec.mjs, tests/browser/facility-authoring.spec.mjs, tests/browser/state-changing-command-authoring.spec.mjs, tests/browser/terminal-grouping.spec.mjs
- [x] **T052** [US7] Player rejection notices show a Russian reason instead of the reason code (for example «ДЕЙСТВИЕ ОТКЛОНЕНО: invalid-action» when a fault blocks reading an entry) · frontend/client/client.js, tests/browser/connectrpc-player.spec.mjs

---

## Dependencies & Execution Order

- **Phases**: Setup → Foundational → US1 to US7 → Polish.
  - The seven story phases depend only on Phase 1 and Phase 2. They own disjoint files and can run in parallel.
  - US1 and US2 are the MVP.
- **Setup**: W1 (T001).
- **Foundational**:
  - W1: validation tests, catalog sync tests, glossary, Go recovery removal, bridge and fixtures, demo data.
  - W2: proto and bindings, then the relaxed rule.
  - W3: demo asset checks.
  - W4 to W12: one Overseer task each, in this order: errors (US7), device dialog (US1), templates (US3), fault and program dialogs (US2), terminal faults (US2), rule dialog (US4), action dialog (US4), list (US5), renames and links (US6).
  - W13: inventory and demo journey.
- **US1, US2, US4, US5, US6, US7**: one wave each; their tasks touch different files.
- **US3**: one task.
- **Polish**: W1 (suites) → W2 (packaged app); both write `validation.md`.
