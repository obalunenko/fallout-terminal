# Feature Specification: Facility Play Controls

**Feature Branch**: `040-facility-play-controls`

**Created**: 2026-10-30

**Status**: Draft

**Input**: User description: "Facility play controls and workspace layout. Implement review findings F-07 and F-09 from the real-application UX review of the Overseer facility workspace (review page: https://claude.ai/artifact/7JTskVJn9xKELD4esDS78f), building on feature 039. F-07: during play the Overseer cannot move a device to another state by hand; expected transition buttons on the device card with preconditions checked as for a player command, a forced move to any state with confirmation and a journal entry, a toggle for each fault of the device, and a short feed of recent changes including rejected player requests. F-09: at 1440×900 the story guide takes the top of the screen and the selected device's current state is cut off; expected a guide that collapses to one line after the first device and remembers that, the device header first, buttons grouped by role with delete in a «⋯» menu and no separate rename field, and bindings, faults, and programs as tabs inside the device; the device list shows each device's current state and fault count. Keep session JSON version 1 compatibility, server-authoritative state with revision checks, demo updates per Principle IX, browser tests for every story; stack on 039-unified-facility-authoring."

## Clarifications

### Session 2026-10-30

- Q: Which faults does the play card's switch work for? → A: Every fault scoped to the device. The authored «Восстановление оператором» setting does not limit the switch, stays unchanged in the data, and is revisited by feature 041 (F-12).
- Q: Does the device's recent-changes feed survive an application restart? → A: No. It covers the current run only, is not saved in the session file, and the retained runtime log stays the full history.
- Q: Does pressing a transition on the play card ask for confirmation? → A: No. A transition runs on one press. Only forced change and reset to the starting state ask for confirmation.
- Q: How does the guide remember being expanded or collapsed? → A: A rule plus memory for the current run. The guide is collapsed when the session has at least one device. The author's explicit choice holds until the application closes, and after a restart the rule applies again. Nothing new is stored.
- Q: Does a card transition mark a player command that runs the same transition as executed? → A: No. The card changes only the device and its faults, like reset and forced change. The command stays unexecuted. If a player runs it later, the usual «transition unavailable» refusal applies and appears in the feed.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run a device's transitions from its card (Priority: P1)

During a game the players break a door with a crowbar, so the Overseer selects «Гермодверь Убежища 76» and presses «Открыть гермодверь → Открыта» on the device card. The card lists every transition that leaves the current state. Next to each one it shows that transition's preconditions, each marked as met or unmet. The change follows the same rules and fault effects as a player command, and the players' terminals show the new state right away.

**Why this priority**: The Overseer cannot steer the scene during play without this. It is the core of F-07, and every other play control sits on the same card.

**Independent Test**: Open the demo session and select the door. Run an available transition from the card, then check that the device state, the fault effects, and the connected player terminal all match the result of the same transition run by a player command.

**Acceptance Scenarios**:

1. **Given** a door in «Заперта» with the transition «Открыть гермодверь» to «Открыта» whose precondition «Резервная сеть = В работе» is met, **When** the Overseer presses that transition on the card, **Then** the door becomes «Открыта», the card shows the new state and the transitions that leave it, and a connected player terminal shows the door open without reconnecting.
2. **Given** the same transition while the power grid is «Нет питания», **When** the Overseer looks at the card, **Then** the transition is unavailable, and the unmet precondition «Резервная сеть = В работе» is named next to it.
3. **Given** a transition that activates a fault, **When** the Overseer runs it from the card, **Then** the fault becomes active exactly as it would after a player command, and the device state change and fault change land together or not at all.
4. **Given** a player command changed the door after the Overseer opened the card, **When** the Overseer presses a transition based on the old state, **Then** nothing changes, the card refreshes to the current state, and it says that the scene changed meanwhile.

---

### User Story 2 - See the selected device first (Priority: P1)

In a 1440×900 or 1180×760 window the Overseer selects a device and sees its header first: name, type, how many terminals use it, current state, starting state, and active faults. The play card with the transition controls follows directly. The scenario guide no longer pushes them below the window edge: once the session has a device, the guide shrinks to one line. That line shows the current step and lets the author open the guide again.

**Why this priority**: The card from User Story 1 only helps during play if the Overseer can see it without scrolling. This is the core of F-09.

**Independent Test**: Open the demo session in a 1440×900 window and then in a 1180×760 window, select each device in turn, and check that the header and the first transition control are fully visible without scrolling and that nothing overlaps.

**Acceptance Scenarios**:

1. **Given** a session with no devices, **When** the Overseer opens «Объекты», **Then** the guide is expanded.
2. **Given** the author creates the first device, **When** the device is saved, **Then** the guide collapses to one line such as «Как собрать сценарий · шаг 1 из 5: Объект» with a «Показать» control.
3. **Given** the author expanded the guide in a session with devices, **When** they switch tabs or select another device, **Then** the guide stays expanded until the application closes; after a restart it is collapsed again.
4. **Given** a selected device in a 1440×900 window or a 1180×760 window with the guide collapsed, **When** the detail area is shown, **Then** the device header comes first, and the header and the first transition control are fully visible without scrolling.
5. **Given** the guide is expanded at any window width from 900 points up, **When** the author scrolls the detail area, **Then** the guide's step navigation never covers other text or controls.

---

### User Story 3 - Force a device into any state (Priority: P2)

The story needs the reactor to overheat even though no authored transition leads there. The Overseer chooses «Принудительно в другое состояние…», picks «Критический перегрев», and confirms a dialog that names the device and both states. The device moves without checking transitions or preconditions, and the journal records the change as forced.

**Why this priority**: Authored transitions cover planned play. This covers everything the author did not plan, so it comes right after User Story 1.

**Independent Test**: Force a demo device into a state that no transition reaches from its current state, then check the new state, the confirmation text, the journal entry marked as forced, and the player terminal.

**Acceptance Scenarios**:

1. **Given** the reactor is «Дежурный режим», **When** the Overseer opens «Принудительно в другое состояние…», **Then** every state except the current one is offered, including «Критический перегрев», which no transition reaches.
2. **Given** the Overseer picked «Критический перегрев», **When** they confirm, **Then** the reactor becomes «Критический перегрев», no transition's fault effects run, and the journal and the device feed record a forced change from «Дежурный режим» to «Критический перегрев».
3. **Given** the confirmation dialog is open, **When** the Overseer cancels, **Then** nothing changes and nothing is recorded.

---

### User Story 4 - Turn a device's faults on and off (Priority: P2)

The Overseer wants the door controller to stop answering. The play card lists every fault attached to the door with its current position and a switch. The Overseer turns «Контроллер гермодвери не отвечает» on, and the players feel its effects at once. Turning it off later works the same way.

**Why this priority**: Faults are the second half of a device's play state. Without the switch, starting a complication during play takes a JSON edit or a scripted command.

**Independent Test**: Toggle a demo device's fault on and then off from the card, and check the fault's current value, its unchanged starting value, its effects on a connected player terminal, and the feed entries.

**Acceptance Scenarios**:

1. **Given** the door has the inactive fault «Контроллер гермодвери не отвечает», **When** the Overseer turns it on, **Then** the fault becomes active, its effects reach players (for example a blocked command or a replaced record), and the device header and the device list count it.
2. **Given** that fault is active, **When** the Overseer turns it off, **Then** it becomes inactive, and its starting position «в начале игры» stays as authored.
3. **Given** a device has no faults, **When** the card is shown, **Then** it says so and offers no switches.

---

### User Story 5 - See what happened to a device without watching (Priority: P2)

The Overseer was busy with another table. When they come back to the door, the card shows the latest changes newest first. Each entry gives the time, what changed, and who changed it. Rejected player requests are included, for example «20:47 · «Открыть выход и подать сигнал» · запрос игрока отклонён». The feed updates while the card is open.

**Why this priority**: It makes the play controls safe to use between players' actions. It adds value on top of User Stories 1, 3, and 4.

**Independent Test**: Make one change of each kind to a demo device: a player command, an Overseer transition, a forced change, a fault toggle, a reset, and a rejected player request. Then check that the feed lists each one with time, kind, and author, newest first.

**Acceptance Scenarios**:

1. **Given** a player ran «Открыть выход и подать сигнал» on the door, **When** the Overseer opens the door's card, **Then** the feed shows the time, the command name, the states it moved between, and the terminal it came from.
2. **Given** the Overseer rejected a player's request to change the door, or the request failed on an unmet precondition or an active fault, **When** the feed is shown, **Then** it lists the request as rejected with the reason.
3. **Given** the card is open, **When** a player command changes the door, **Then** the new entry appears at the top without reopening the card.
4. **Given** the application has just started, **When** the Overseer opens a device, **Then** the feed says that nothing has changed in this run yet.

---

### User Story 6 - Find device settings by role (Priority: P2)

The selected device's controls are grouped by role. Inspection («Предпросмотр», «Зависимости») sits apart from changes («Изменить состояния и переходы»). «Удалить устройство» sits in a «⋯» menu, and play controls live only on the play card. Renaming happens in the device dialog, so the separate name field goes away. The device's bindings, faults, and recovery programs open as tabs inside the device, each showing its item count, instead of one long scroll. The device list shows each device's current state and its active fault count.

**Why this priority**: It removes the visual noise that F-09 measured: six equally weighted buttons, two of them red. It does not block play.

**Independent Test**: Select a demo device, check the button groups, rename the device through the dialog, switch between the tabs and compare their counts with the session content, and read each device's state and fault count in the list.

**Acceptance Scenarios**:

1. **Given** a selected device, **When** its actions are shown, **Then** no destructive control appears outside the «⋯» menu, and «Удалить устройство» is reachable in two actions.
2. **Given** the Overseer wants to rename the device, **When** they open «Изменить состояния и переходы», **Then** the name is editable there, and no separate name field sits under the header.
3. **Given** the door has one command, three player-visible texts, one fault, and no recovery program, **When** its details are shown, **Then** the tabs «Что делают игроки 1», «Что видят игроки 3», «Неисправности 1», and «Программы 0» are shown, and only the open tab's items are listed.
4. **Given** the ventilation loop is «Загрязнён» with one active fault, **When** the device list is shown, **Then** its row reads «Загрязнён · ● 1», and the row updates when the state or fault changes.

---

### Edge Cases

- No transition leaves the current state: the card says so, and forced change, fault switches, and reset to the starting state stay available.
- A device has a single state: forced change is unavailable, and the card gives that as the reason.
- The Overseer presses the wrong transition: «Отменить» does not reverse play changes (feature 039 keeps live state out of the undo history), so the Overseer corrects it with another transition or a forced change.
- A player's state-changing request is waiting for Overseer approval when the Overseer changes the facility by hand. The waiting request is withdrawn exactly as after a device reset today: the player's terminal shows it rejected, and the feed records it.
- A player command and an Overseer play change arrive together: only one applies. The other is rejected because its facility revision is stale, and the card shows the state that won.
- The device is deleted or its graph is edited while its card is open: the card follows the saved graph, or closes when the device is gone.
- A scene reset restores every device: the feed of each device it changed records one scene-reset entry.
- A fault scoped to a terminal rather than a device does not appear on any device card. Feature 041 (F-12) places those faults.
- A player runs a command whose transition the Overseer already made from the card: the usual refusal applies because the device is no longer in the source state, and the feed records it.
- A tab has no items: it stays visible with count 0 and its add control.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The selected device's play card MUST show one control for each authored transition that leaves the device's current state, labeled with the transition name and its destination state. A transition control MUST run on one press, without a confirmation step.
- **FR-002**: Each transition control MUST list the transition's preconditions, each marked met or unmet against the current facility state, and MUST be unavailable, naming the unmet precondition, while any precondition is unmet.
- **FR-003**: A transition run from the card MUST follow the same rules as a player command running that transition: it checks the source state and preconditions, applies the transition's fault effects, and applies all of them or none. It MUST NOT mark any command as executed: commands keep their own state.
- **FR-004**: Every Overseer play change (transition, forced change, fault switch, device reset) MUST be checked against the facility revision the Overseer last saw. The revision covers the whole facility, so any change to any device or any authoring edit makes an older play change stale. A stale change MUST be rejected with nothing changed, and the card MUST show the current state returned with the rejection and say that the scene changed meanwhile.
- **FR-005**: After an Overseer play change, connected player terminals MUST show the result (menu names, record text, fault effects) the same way they do after a player command, without reconnecting.
- **FR-006**: An Overseer play change MUST withdraw a pending player request the same way a device reset does today, and MUST record the withdrawal in the retained log and in the feed of each device the request targeted.
- **FR-007**: The card MUST offer «Принудительно в другое состояние…», which lists every state of the device except the current one and moves the device to the chosen state without checking transitions or preconditions.
- **FR-008**: A forced change MUST ask for confirmation that names the device, its current state, and the target state, and MUST NOT run any transition's fault effects.
- **FR-009**: The card MUST list every fault scoped to the device with its current position and a switch that activates or clears it, whatever the fault's «Восстановление оператором» setting.
- **FR-010**: Switching a fault MUST change only its current position, never its authored starting position, and its effects MUST reach players as in FR-005.
- **FR-011**: «Сбросить к начальному…» MUST stay on the card with its current confirmation and effects, and MUST succeed when only the device's faults differ from their starting positions.
- **FR-012**: The card MUST show the device's five most recent changes, newest first, each with time, what happened (transition name, forced change with both states, reset, scene reset, fault activated or cleared, player command), and who did it (the Overseer, or the player terminal that sent the command). The feed MUST cover the current application run only: it is not saved in the session file and starts empty after a restart or when another session opens.
- **FR-013**: The feed MUST include player requests affecting the device that were rejected by the Overseer or refused by the facility, with the reason (unmet precondition, transition no longer available, active fault, changed meanwhile, withdrawn).
- **FR-014**: The feed MUST update while the card is open, without reopening the device.
- **FR-015**: Every Overseer play change, accepted or rejected, MUST be written to the retained runtime log with the device, the action, the states before and after, and the outcome, as device reset is logged today.
- **FR-016**: Feed entries and log records of play changes MUST NOT contain passwords, credentials, or hacking content.
- **FR-017**: The selected device's header MUST come first in the detail area and show its name, type, number of linked terminals, current state, starting state, and number of active faults.
- **FR-018**: In a 1440×900 window and in a 1180×760 window, with a device selected and the guide collapsed, the device header and the first transition control MUST be fully visible without scrolling.
- **FR-019**: The scenario guide MUST be expanded when the session has no devices, MUST collapse to a single line showing the current step (for example «шаг 1 из 5: Объект») when the first device is saved, and MUST reopen on demand.
- **FR-020**: The guide MUST keep the author's last explicit choice (expanded or collapsed) until the application closes. After a restart it MUST follow FR-019 again. Nothing about the guide is stored in the session file or elsewhere.
- **FR-021**: The guide's step navigation MUST NOT cover other content at any window width from the application's minimum of 900 points up.
- **FR-022**: The selected device's actions MUST be grouped by role, with inspection (preview, dependencies) separate from changes (edit states and transitions). «Удалить устройство» MUST sit only in a «⋯» menu, and play controls MUST appear only on the play card.
- **FR-023**: The separate name field under the selected device MUST be removed, together with the «ГОТОВО» and «ОТМЕНА» controls that serve only it. The device keeps being renamed in its device dialog.
- **FR-024**: The device's details MUST be grouped into tabs «Что делают игроки», «Что видят игроки», «Неисправности», and «Программы», each showing its item count. Only the open tab's items MUST be listed.
- **FR-025**: Each device list row MUST show the device's current state and, when nonzero, its number of active faults, and MUST update when either changes.
- **FR-026**: Sessions MUST stay in portable session JSON version 1: files saved before this feature open unchanged, and play changes save only the device states and fault positions they changed.
- **FR-027**: The demo sessions MUST include reachable paths for each case: an available card transition, a card transition blocked by an unmet precondition, a forced change, a fault switch, a rejected player request in the feed, and a device whose list row shows an active fault.
- **FR-028**: Each user story MUST have a browser test that exercises it in the Overseer and, where players see the result, on a connected player terminal.

### Key Entities

- **Play card**: the play-time view of one selected device. It shows the current state, the transitions that leave it with their preconditions, forced change, fault switches, reset, and the recent-changes feed.
- **Overseer play change**: one change the Overseer makes during play. It is a transition, a forced change, a fault switch, or a device reset. It carries the facility revision it was based on and is recorded in the log and the feed.
- **Device change entry**: one line in a device's feed. It records the time, the kind of change, the author (Overseer or player terminal), the states or fault position before and after, and for rejected requests the reason. Entries cover the current application run.
- **Guide choice**: the author's explicit choice to keep the scenario guide expanded or collapsed. It lasts for the current application run only.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With «Объекты» open during play, the Overseer moves a device through an authored transition in two actions (select the device, press the transition), without opening a dialog.
- **SC-002**: For every transition in the demo sessions, the card's available or unavailable verdict matches the outcome of a player command running that transition in the same facility state, on a terminal where no fault blocks commands, with no exceptions.
- **SC-003**: In 1440×900 and 1180×760 windows with the guide collapsed, the header and the first transition control of every demo device are fully visible without scrolling.
- **SC-004**: Every accepted Overseer play change reaches connected player terminals in the same update that a player command's result would, and none needs a reconnect.
- **SC-005**: Every Overseer play change appears both in the device feed and in the retained log. No play change goes unrecorded.
- **SC-006**: No Overseer play change ever applies over a facility state newer than the one it was based on.
- **SC-007**: In the selected device's controls, the number of destructive buttons outside the «⋯» menu drops from two to zero.
- **SC-008**: Every demo session file opens and saves with no format change.

## Assumptions

- Five feed entries are enough on the card. The retained runtime log stays the full history.
- An Overseer transition is not subject to faults that block player capabilities on a terminal, because it comes from the Overseer, not from a terminal. Its preconditions and fault effects apply as for a player.
- A forced change does not create or modify authored transitions. It only sets the current state.
- Every transition that leaves the current state appears on the card, including transitions marked for recovery.
- «Linked terminals» in the header counts the terminals with a binding to the device, as the device summary does today.
- The «Программы» tab lists the recovery programs that include a transition of the selected device. With no device selected, the tabs show the whole session's items.
- The device list's current state and fault count are in scope. Badges and filters (F-14) stay in feature 041.

## Out of Scope

- Review findings F-10 to F-14: the transition matrix and templates, the «когда — где — что» rule dialog, placing faults in one place (including terminal-scoped faults and the meaning of «Восстановление оператором»), terminology, and list badges and filters. These are planned as feature 041.
- Changing how players run commands or what the precedence rules show them.
- Running several devices' transitions as one Overseer action. Multi-device changes stay on commands and recovery programs.

## Verbatim Constraints

- `resetFacilityDevice`: the new play operations sit next to this desktop operation and share its revision check and journal.
- `039-unified-facility-authoring`: this feature stacks on that branch.
- UI copy (buttons render these words in the Overseer's uppercase style): «Принудительно в другое состояние…», «Сбросить к начальному…», «Удалить устройство», «⋯», «Показать», «Что делают игроки», «Что видят игроки», «Неисправности», «Как собрать сценарий».
