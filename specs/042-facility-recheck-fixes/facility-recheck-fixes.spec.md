# Feature Specification: Facility Recheck Fixes

**Feature Branch**: `042-facility-recheck-fixes`

**Created**: 2026-11-01

**Status**: Draft

**Input**: User description: "Facility recheck follow-ups. Close the open items from the 2026-11-01 packaged-app recheck of the Overseer UX review (review page: https://claude.ai/artifact/7JTskVJn9xKELD4esDS78f, section «Что нашла повторная проверка»; evidence in specs/041-facility-editors-language/validation.md, section «Packaged-app recheck follow-ups»). (1) Templates: a template transition counts as present when the device already has any transition between the same two states. (2) Requests blocked by a fault get their own reason, a precise Russian message for the player, and an entry in the device's feed. (3) The diagnostics journeys must tell a blocked entry from an open one. (4) Layout and copy fixes from the recheck. (5) Record the finished 033 dialog tasks. Out of scope: 019 BUG-001 and 024 T048."

## Clarifications

### Session 2026-11-01

- Q: When does a template transition count as already present? → A: When the device has any transition from the template's source state to its destination state, whatever it is called.
- Q: What does the Overseer see about a request a fault blocked, and what does the player see? → A: The fault's device feed lists the refused request, and the player sees a precise Russian reason that names no fault.
- Q: Is 019 BUG-001 part of this feature? → A: No. It follows as a separate bugfix.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Know why a fault blocked a request (Priority: P1)

The scene starts with «Споровое загрязнение воздушного контура» active, and the spores block reading entries on «Терминал смотрителя — Убежище 76». A player opens «ДИРЕКТИВЫ VAULT-TEC» and selects «СВОДКА ЭВАКУАЦИИ». Today the player reads «ДЕЙСТВИЕ ОТКЛОНЕНО: ЭТО ДЕЙСТВИЕ СЕЙЧАС НЕДОСТУПНО.», and the Overseer sees nothing at all. With this story the player reads «ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.» The Overseer opens «Контур вентиляции теплицы» and its «ПОСЛЕДНИЕ ИЗМЕНЕНИЯ» feed shows the refused request: the time, that reading «СВОДКА ЭВАКУАЦИИ» on «Терминал смотрителя — Убежище 76» was refused, and that «Споровое загрязнение воздушного контура» blocked it. The player tries four more times; the feed still shows one entry, now counting the attempts. The Overseer switches the fault off and the player reads the entry.

**Why this priority**: The demo starts with this block, and today neither side can tell what happened. The player does not know a fault is in the way, and the Overseer does not know players are stuck.

**Independent Test**: Start the demo scene with a connected player. Try to read the blocked entry five times. Check the player's message, the single counted feed entry on the ventilation card, and the retained log. Switch the fault off and check that the entry opens.

**Acceptance Scenarios**:

1. **Given** an active fault blocks reading entries on a terminal, **When** the controlling player opens an entry there, **Then** the entry does not open and the player reads «ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.»
2. **Given** that fault belongs to a device, **When** the request is refused, **Then** the device's feed shows, newest first, the time, the terminal, the blocked entry by its name, and the fault by its name, marked as a refused player request.
3. **Given** that feed entry is the device's newest, **When** the player repeats the same request, **Then** the entry's time updates and it shows the number of attempts, and no new entry appears.
4. **Given** the same fault blocks running a command, hacking, moving to another terminal, or running a recovery program, **When** the player tries it, **Then** the request is refused with the same reason and the feed names what was blocked.
5. **Given** the Overseer switched the fault off, **When** the player opens the entry again, **Then** it opens and no refusal is recorded.

---

### User Story 2 - Apply a template without duplicating transitions (Priority: P2)

The author opens the demo door «Гермодверь Убежища 76». It already has «Заперта» and «Открыта», «Открыть гермодверь» from «Заперта» to «Открыта», and «Запереть гермодверь» back. Today the dialog offers «ПРИМЕНИТЬ ШАБЛОН», and pressing it adds «Открыть» and «Запереть» to the same matrix cells. With this story the dialog shows «ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ» as soon as it opens, and the template adds nothing.

**Why this priority**: The duplicate transitions are valid data, so nothing warns the author, and both show on the device card during play. It does not block play.

**Independent Test**: Open the demo door and check the message and that nothing can be added. Then apply the door template to a new door with no transitions, and to a door that has only «Заперта → Открыта» under another name, and check what each gains.

**Acceptance Scenarios**:

1. **Given** a door that already has a transition in both template cells, **When** its dialog opens, **Then** it shows «ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ» and the template adds nothing.
2. **Given** a door with «Заперта» and one renamed transition from «Заперта» to «Открыта», **When** the author applies the template, **Then** only «Открыта» if missing and «Запереть» are added, nothing is renamed or removed, and the starting state stays as it was.

---

### User Story 3 - Regression journeys tell a blocked request from an allowed one (Priority: P2)

A developer changes how faults block players. The diagnostics journeys must fail if a blocked entry opens. Today the fixture's affected entry contains the text «Ошибка доступа», so the journey passes whether or not the entry opened.

**Why this priority**: Story 1 changes the refusal path. Without journeys that can see the difference, a regression would go unnoticed.

**Independent Test**: Turn the block off in the fixture's server side only. The journey must fail. Turn it back on. The journey must pass.

**Acceptance Scenarios**:

1. **Given** a fixture fault that blocks reading an entry, **When** the participant opens that entry, **Then** the journey sees the fault reason, and the entry's own authored text is not on screen.
2. **Given** the fault is then switched off, **When** the participant opens the entry, **Then** the journey sees the entry's authored text.

---

### User Story 4 - A tidy workspace at 1440×900 (Priority: P3)

The recheck found small layout and copy defects. The Overseer can collapse «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» so the device list gets the room, and the choice holds for the rest of the run. A filtered list shows its one row whole. There is no empty band above the workspace. The fault dialog's start position uses the Overseer's own controls. The terminal tree reads «[КОМАНДА] Название». Rule rows read «ИЗМЕНИТЬ» and «УДАЛИТЬ» like every other control. The focus outline of a device row is drawn whole.

**Why this priority**: None of these blocks work. Together they make the workspace look unfinished.

**Independent Test**: At 1440×900, collapse the terminal-fault group and count the visible device rows. Filter to one row and check its border. Check the band above the workspace, the fault dialog's start-position control, the tree labels, the rule row buttons, and a focused device row.

**Acceptance Scenarios**:

1. **Given** the window is 1440×900 and «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» is collapsed, **When** the demo opens, **Then** at least seven device rows are visible without scrolling the list, and the collapsed heading still shows how many terminal faults are active.
2. **Given** the Overseer collapsed the group, **When** the session is reopened in the same run, **Then** the group is still collapsed.
3. **Given** a filter leaves one device, **When** the list renders, **Then** the row and its whole border are visible.

---

### Edge Cases

- The fault turns off between the request and its check: the request is checked against the state at that moment, so it either succeeds or is refused, never both.
- Two active faults block the same request: the player reads the reason once, and each blocking fault's device feed records it.
- A terminal-scoped fault blocks a request: the player reads the same reason. No device feed exists for it, so only the retained log records it.
- A player command with a device action is blocked by a fault: today it already leaves a refused entry. It keeps one entry in the same format, never two.
- Another change lands on the device between two identical refusals: the second refusal is a new entry, because the first is no longer the newest.
- The feed keeps five entries per device; refusals count toward the five.
- An observer cannot send requests, so a refusal is only ever the controller's.
- A session saved before this feature opens and saves unchanged: the feed and its counts live only for the current run.

## Requirements *(mandatory)*

### Functional Requirements

**Requests blocked by a fault (extends 040 FR-013)**

- **FR-001**: When an active fault's block stops a player request (reading an entry, running a command, hacking, moving to another terminal, or running a recovery program), the request MUST be refused with a fault-block reason that is distinct from the generic «invalid action» reason.
- **FR-002**: The player MUST read «ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.» for a fault-block refusal. The message MUST NOT name the fault.
- **FR-003**: For each blocking fault that belongs to a device, that device's «ПОСЛЕДНИЕ ИЗМЕНЕНИЯ» feed MUST list the refused request with its time, the terminal, what was blocked by its authored name, and the fault by its name.
- **FR-004**: When the same request (same terminal, same target, same fault) is refused again while its entry is the device's newest, the entry MUST update its time and show the number of attempts instead of adding an entry.
- **FR-005**: A player command with a device action that a fault blocks MUST leave exactly one feed entry, in the format of FR-003.
- **FR-006**: Each fault-block refusal MUST be written to the retained runtime log with the terminal, the blocked capability, the fault, and the request's correlation identity, and with no authored text.
- **FR-007**: The demo capability inventory MUST list the path from scene start: reading entries on «Терминал смотрителя — Убежище 76» is refused, the «Контур вентиляции теплицы» feed shows it, and switching the fault off lets the player read.

**Templates (replaces 041 FR-011)**

- **FR-008**: A template transition MUST count as present when the device already has a transition from the template's source state to its destination state, whatever that transition is called. States match by name, ignoring case.
- **FR-009**: Applying a template MUST add only the missing states and the transitions missing under FR-008. It MUST NOT rename or remove anything, and MUST set the starting state only when the device has none.
- **FR-010**: When nothing is missing, the dialog MUST show «ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ» as soon as it opens, and the template MUST NOT be applicable.
- **FR-011**: The 041 specification MUST state the new rule in place of its FR-011 and in User Story 3, scenario 2.

**Diagnostics journeys**

- **FR-012**: The browser fixture MUST refuse a blocked request through the same refusal path as the application.
- **FR-013**: Every fixture entry that a fault can block MUST carry authored text that differs from any refusal or error message.
- **FR-014**: The diagnostics journeys MUST check the refusal reason, that the entry's authored text is not shown while the fault is active, and that it is shown after the fault is switched off.

**Workspace layout and copy**

- **FR-015**: «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» MUST be collapsible. Collapsed, its heading MUST show the number of active terminal faults, and the device list MUST take the freed height.
- **FR-016**: The group's collapsed or expanded state MUST hold for the rest of the run, including when the session is reopened, as the 040 story guide's choice does.
- **FR-017**: Every device row the list shows MUST be drawn whole, including its border, whatever the filter.
- **FR-018**: The workspace MUST NOT reserve space for the location status line while it has nothing to show.
- **FR-019**: The fault dialog's «В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА» choice MUST use the Overseer's own control style, not the system's default radio buttons.
- **FR-020**: The terminal tree MUST separate an element's kind from its name with a space, for example «[КОМАНДА] Название».
- **FR-021**: The edit and delete buttons of rule, action, and program rows MUST read «ИЗМЕНИТЬ» and «УДАЛИТЬ», following the glossary's case convention. Their accessible names stay as they are.
- **FR-022**: A keyboard-focused device row MUST show its whole focus outline.

**Records and compatibility**

- **FR-023**: Tasks T026 to T030 of feature 033 MUST be marked done, with the evidence that the block command dialog journeys pass and cover 033's FR-023 to FR-026 and SC-009.
- **FR-024**: Session files of version 1 MUST open and save unchanged. Nothing this feature adds is stored in a session file.

### Key Entities

- **Fault-block refusal**: one refused player request. It records the time, the terminal, the target (an entry, a command, hacking, a terminal move, or a recovery program), the blocked capability, and the faults that blocked it. It lives for the current run only.
- **Feed entry**: one line of a device's «ПОСЛЕДНИЕ ИЗМЕНЕНИЯ». A refusal entry adds an attempt count that grows while the same refusal repeats.
- **Device template**: the suggested states, starting state, and transitions for one device type. Its transitions now match the device's by their source and destination states.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In the demo at scene start, every attempt to read «СВОДКА ЭВАКУАЦИИ» shows the fault reason to the player, and five attempts leave exactly one entry on the ventilation card, showing five attempts.
- **SC-002**: For each of the five blockable capabilities, a blocked request is refused with the fault reason in 5 of 5 journeys, and never with the generic reason.
- **SC-003**: The door template adds 0 states and 0 transitions to the demo door, and 2 states and 2 transitions to a door that has neither.
- **SC-004**: The diagnostics journeys fail when the server stops enforcing the block, and pass when it enforces it.
- **SC-005**: At 1440×900 with the terminal-fault group collapsed, at least 7 of the 10 demo device rows are visible without scrolling the list, up from about 4.5.
- **SC-006**: No visible device row or focus outline is clipped in any filter state.
- **SC-007**: Feature 033's task list has 0 unchecked tasks.
- **SC-008**: The full browser suite passes, and the packaged app passes the checks of SC-001, SC-003, and SC-005 with a connected player.

## Assumptions

- The player message names no fault, because fault names are written for the Overseer and can give away the scene.
- A terminal-scoped fault has no device card, so its refusals go to the player and the retained log only.
- The terminal-fault group starts expanded, as today, so a running game keeps its switches in view until the Overseer collapses it.
- Ten demo rows cannot fit at 1440×900 without shrinking the rows that 041 laid out, so the target is seven.
- The feed stays run-only, as 040 defined it.

## Out of Scope

- 019 BUG-001, movement cues over ngrok. It follows as a separate bugfix.
- 024 T048, the release-candidate update check.
- Showing the fault's name to players.
- A feed for terminal-scoped faults.

## Verbatim Constraints

- `ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.`
- `ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ`
- `ПОСЛЕДНИЕ ИЗМЕНЕНИЯ`
- `НЕИСПРАВНОСТИ ТЕРМИНАЛОВ`
- `В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА`
- `[КОМАНДА] Название`
- `ИЗМЕНИТЬ`
- `УДАЛИТЬ`
