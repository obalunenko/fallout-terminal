# Feature Specification: Facility Editors and Language

**Feature Branch**: `041-facility-editors-language`

**Created**: 2026-10-31

**Status**: Draft

**Input**: User description: "Facility editors and language. Implement review findings F-10, F-11, F-12, F-13, and F-14 from the real-application UX review of the Overseer facility workspace (review page: https://claude.ai/artifact/7JTskVJn9xKELD4esDS78f), building on feature 040. F-10: the device dialog edits the state graph as a long form — the first field is a technical ID, half of every state row is an ID, the initial state is a separate list below, every transition is a four-field card with nested preconditions and effects, and the device type suggests nothing; expected type templates («Дверь» offers «Заперта / Открыта» and «Открыть / Запереть»), states as chips with the initial one starred on the chip, transitions as a «из → в» matrix where a cell opens the transition with its conditions and effects, and identifiers in a collapsed block at the bottom. F-11: one «Привязка объекта» dialog serves five different rules and starts with the terminal; expected a title that names the rule, the rule type chosen by tabs at the top, fields laid out as a sentence «когда устройство в состоянии — где показать — что показать», element choice showing the folder path and block choice by number and opening text, and command action configured only in the command form. F-12: a fault is configured in three places under different names («диагностическое условие», «условие», «неисправность»); expected one name «Неисправность», a «Как появляется» section listing the transitions that activate it with a warning when none do, a «Как устраняется» section listing every way to clear it (transitions, the Overseer by hand, recovery programs) with a program created from there and program transitions as one uniform list; resolve the placement of terminal-scoped faults and the meaning of «Восстановление оператором» deferred by feature 040. F-13: one entity has several names and internals are visible; expected one glossary that every Overseer string is checked against (Локация, Устройство, Состояние, Переход, Правило отображения, Действие команды, Неисправность, Программа восстановления, Готово / Отмена), dependency links described in words with a button naming the destination («К команде», «К неисправности») and the technical path under «подробности для разработчика», and every error in Russian with a hint on what to do (for example «could not read the player config»). F-14: the device list is hard to scan; expected left-aligned rows with the name and the type below, the state as a badge on the right with a yellow frame when it differs from the initial state, an active fault as a red mark with a count, a search field, and filters «Все», «Изменённые», «С неисправностью» with counts. Keep session JSON version 1 compatibility, server-authoritative state with revision checks, demo updates per Principle IX, browser tests for every story; stack on 040-facility-play-controls."

## Clarifications

### Session 2026-10-31

- Q: What does «Восстановление оператором» mean after this feature? → A: The Overseer can always clear any fault by hand. «Смотритель вручную» is always listed under «Как устраняется», the checkbox goes away, the stored value stays in the file untouched, and a fault saves without a clearing transition or program.
- Q: Where does the Overseer find terminal-scoped faults to edit and switch them during play? → A: In a «Неисправности терминалов» group at the end of the device list, always visible. Each row names the terminal, opens the fault dialog, and has the same on/off switch the device card gives device faults.
- Q: How far does "every error in Russian with a next step" reach? → A: The whole Overseer window, in two tiers. Facility workspace, player configuration, and session file failures get specific Russian messages with a next step. Every other failure gets a general Russian message with a next step, with its original text under «подробности для разработчика».
- Q: Where may the author change which faults a transition turns on or off? → A: Only in the fault dialog's «Как появляется» and «Как устраняется». The transition editor lists the effects read-only, each with «К неисправности».

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Edit a device's states and transitions on one screen (Priority: P1)

The author opens «Гермодверь Убежища 76» to add a transition. The dialog starts with the name and type. The states follow as chips, with the starting state «Заперта» starred on its chip. Below them the transitions form a «из → в» matrix: the row «Заперта» has «Открыть гермодверь» in the column «Открыта», and the reverse cell holds «Запереть гермодверь». The author chooses a cell to open its transition, its preconditions, and its fault effects. Identifiers sit in a collapsed block at the bottom.

**Why this priority**: The device graph is the base of every other facility editor. Today a door with two transitions does not fit in the window, and the graph cannot be seen as a whole.

**Independent Test**: Open each demo device in the device dialog at 1440×900. Check that the states and the whole matrix are visible without scrolling and that no identifier field is visible. Then add, open, and edit a transition through the matrix and save.

**Acceptance Scenarios**:

1. **Given** the door with states «Заперта» (starting) and «Открыта» and two transitions, **When** the author opens the device dialog, **Then** the states appear as two chips with a star on «Заперта», the matrix shows «Открыть гермодверь» in row «Заперта», column «Открыта», and «Запереть гермодверь» in the reverse cell, and the diagonal cells are unavailable.
2. **Given** that matrix, **When** the author chooses the empty cell of a device with three states, **Then** a new transition between that pair of states opens with a suggested name, and after «Готово» the cell shows it.
3. **Given** the filled cell «Заперта → Открыта», **When** the author chooses it, **Then** the transition opens with its name, its preconditions such as «Резервная сеть = В работе», and its fault effects, each naming the fault and whether the transition activates or clears it.
4. **Given** the author stars «Открыта» on its chip, **When** they press «Готово», **Then** «Открыта» is the starting state and no separate starting-state list exists anywhere in the dialog.
5. **Given** a new device named «Дверь в убежище», **When** the dialog is shown, **Then** no identifier field is visible. Expanding «Технические идентификаторы» shows the generated identifiers of the device, its states, and its transitions.
6. **Given** a row whose identifier conflicts with another, **When** the author presses «Готово», **Then** «Технические идентификаторы» expands, the conflicting row is marked, and focus moves to it.

---

### User Story 2 - Configure a fault in one place (Priority: P1)

The author opens «Контроллер гермодвери не отвечает». The dialog is titled «Неисправность» and holds everything about the fault: its name, where it acts, what it blocks, its position at game start, «Как появляется», and «Как устраняется». «Как появляется» warns that no transition turns the fault on, so today the Overseer can only turn it on by hand. The author adds a transition that turns it on without leaving the dialog. «Как устраняется» lists «Открыть гермодверь», the Overseer by hand, and any recovery program, and a new program can be started from there.

**Why this priority**: Making a fault appear and disappear now takes three dialogs, and nothing warns the author about a fault that never appears. It also settles the two questions feature 040 left open: where terminal-scoped faults live and what «Восстановление оператором» means.

**Independent Test**: For every demo fault, open its dialog and compare the transitions listed under «Как появляется» and «Как устраняется» with the session data. Add a transition that turns on a fault, create a recovery program from «Как устраняется», and check both after saving.

**Acceptance Scenarios**:

1. **Given** «Контроллер гермодвери не отвечает», which no transition activates, **When** the author opens it, **Then** the dialog is titled «Неисправность» and «Как появляется» warns: «Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.»
2. **Given** that dialog, **When** the author chooses «+ Включать при переходе…», picks «Гермодверь Убежища 76 — Запереть гермодверь» and presses «Готово», **Then** that transition turns the fault on when it runs, the warning is gone, and the transition's editor in the device dialog shows the effect.
3. **Given** «Споровое загрязнение воздушного контура», turned on by «Смоделировать загрязнение» and cleared by «Продуть воздушный контур» and the program «ВОЗДУХ-76», **When** the author opens it, **Then** «Как появляется» lists «Смоделировать загрязнение», and «Как устраняется» lists «Продуть воздушный контур», «ВОЗДУХ-76», and «Смотритель вручную».
4. **Given** «Как устраняется», **When** the author chooses «+ Программа восстановления…», **Then** the program dialog opens with a transition that clears this fault already in its list. After «Готово» the author is back in the fault dialog and the new program is listed.
5. **Given** a recovery program with two transitions, **When** the author opens it, **Then** both transitions appear as identical rows in one list with one add control and no multi-sentence instruction, and the dialog names the commands that run the program or warns when none does.
6. **Given** the terminal-scoped fault «Закрытый канал изолирован», **When** the Overseer looks for it during play with any device selected, **Then** they find it in the «Неисправности терминалов» group at the end of the device list, marked active, with its terminal named, and can turn it on and off with the same switch device faults have.

---

### User Story 3 - Start a device from its type's template (Priority: P2)

The author creates a device, names it «Шлюз теплицы», and picks the type «Дверь». The dialog offers the «Дверь» template and names what it adds: «Заперта, Открыта · Открыть, Запереть». One press adds both states, the starting state, and both transitions. The author renames what they need and saves.

**Why this priority**: It turns the most common setup into one action. It depends on the new editor from User Story 1.

**Independent Test**: Create a device of each type with a template and apply it. Check the added states, starting state, and transitions. Apply the template to a device that already has some of them and check that nothing existing changes.

**Acceptance Scenarios**:

1. **Given** a new device of type «Дверь» with no states, **When** the author presses «Применить шаблон», **Then** the device has «Заперта» (starred) and «Открыта» and the transitions «Открыть» (Заперта → Открыта) and «Запереть» (Открыта → Заперта).
2. **Given** a door that already has «Заперта» and a renamed transition, **When** the author applies the template, **Then** only the missing «Открыта» and the missing transitions are added, nothing is renamed or removed, and the starting state stays as it was.
3. **Given** the type «Другой», **When** the dialog is shown, **Then** no template is offered.
4. **Given** a device with states, **When** the author changes its type, **Then** its states and transitions stay unchanged until a template is applied.

---

### User Story 4 - Write a display rule as a sentence (Priority: P2)

The author wants the evacuation summary to say the door is open. They open «Правило отображения» and pick the «Текст записи» tab. The fields read as a sentence. «КОГДА» is «Гермодверь Убежища 76 = Открыта». «ГДЕ» is «Терминал смотрителя › Директивы Vault-Tec › Сводка эвакуации». «БЛОК» is «3 · «ГЕРМОДВЕРЬ: ЗАПЕРТА»». «ТЕКСТ» is the new line. The dialog notes that the other states keep the block's original text. A command's device action is no longer a type here: it opens from the command form.

**Why this priority**: The author thinks «when the door is open, show…», and the form should follow that order. It is the second most used facility editor.

**Independent Test**: Create one rule of each of the four types from the demo content, check the field order and the visible fields for each type, and check what a connected player terminal shows in the matching and the other states.

**Acceptance Scenarios**:

1. **Given** a new rule, **When** the dialog opens, **Then** it is titled «Правило отображения», the tabs «Текст записи», «Название в меню», «Показать элемент», «Разрешить команду» sit at the top, and «Действие команды» is not among them.
2. **Given** the «Текст записи» tab, **When** the fields are shown, **Then** they read in the order «КОГДА» (device = state), «ГДЕ» (element path), «БЛОК», «ТЕКСТ», and the element list offers only records, each with its folder path.
3. **Given** the «Разрешить команду» tab, **When** the fields are shown, **Then** only «КОГДА» and «ГДЕ» appear, «ГДЕ» offers only commands, and no label without a field is visible.
4. **Given** the author filled «КОГДА» and «ГДЕ» on one tab, **When** they switch to a tab whose element list also contains the chosen element, **Then** the device, state, and element stay filled.
5. **Given** a command with a device action, **When** the author opens the command form and chooses its action, **Then** a dialog titled «Действие команды» opens with the command's transitions or program, and the saved action works as before.
6. **Given** a menu name already varied by «Гермодверь Убежища 76», **When** the author adds a menu-name rule for the same element driven by another device, **Then** the dialog names the device already in use and does not save.

---

### User Story 5 - Spot changed and broken devices in the list (Priority: P2)

During play the Overseer wants to see what has moved. The device list shows each name on the left with its type under it, and the state as a badge on the right. A badge has a yellow frame when the state differs from the starting one. A red mark with a count shows active faults. The filters «Все 10», «Изменённые 5», «С неисправностью 1» narrow the list in one press, and the search field matches names and states.

**Why this priority**: It speeds up play but blocks nothing. Feature 040 already put the state and fault count on each row.

**Independent Test**: Open the demo, read each row's badge and mark against the session data, use each filter and the search, then change a device's state and a fault during play and watch the rows, badges, and counts update.

**Acceptance Scenarios**:

1. **Given** the demo session, **When** the list is shown, **Then** each row is left-aligned with the name above the type, and exactly the five devices whose state differs from the starting one have a framed badge.
2. **Given** the ventilation loop with one active fault, **When** the list is shown, **Then** its row has a red mark «● 1».
3. **Given** «Изменённые 5», **When** the Overseer presses it, **Then** only those five devices are listed. Typing «В работе» in the search leaves only «Резервная сеть Убежища 76».
4. **Given** the filter «С неисправностью» is on, **When** the last active fault of a listed device clears, **Then** the device leaves the list, the count drops, and the selected device's details stay open.
5. **Given** a filter and a search that match nothing, **When** the list is shown, **Then** it says that no device matches and offers «Все».

---

### User Story 6 - One name for each thing, and links that say where they lead (Priority: P2)

The author never meets two names for one thing. A device is «Устройство», the whole scene is «Локация», and a fault is «Неисправность» everywhere. In «Зависимости устройства» each link says in words what depends on what, for example «Команда запускает переход двери», with the place by name. Its button names where it leads: «К команде», «К неисправности». The technical path stays under «подробности для разработчика».

**Why this priority**: Mixed names and code paths are why authors ask "is this the same thing?". The glossary also fixes the wording of every other story in this feature.

**Independent Test**: Run the automated vocabulary check over the Overseer copy. Open the dependencies of each demo device and follow every link to its destination.

**Acceptance Scenarios**:

1. **Given** the Overseer copy, **When** the vocabulary check runs, **Then** it finds no «объект» naming a device or the location, and no «диагностическое условие» or «условие» naming a fault.
2. **Given** the door's dependencies, **When** the dialog opens, **Then** each link reads in words with the names of the terminal and the command, its button reads «К команде», «К неисправности», «К программе», «К устройству» or «К правилу отображения», and no technical path is visible until «подробности для разработчика» is expanded.
3. **Given** a facility validation problem, **When** it is shown, **Then** it names the device, state, transition, or fault by name in Russian, and internal kinds and identifiers appear only under «подробности для разработчика».
4. **Given** any control that opens an existing device's dialog, **When** it is shown, **Then** it reads «Изменить состояния и переходы».

---

### User Story 7 - Errors in Russian with a next step (Priority: P3)

The player configuration file cannot be read. Instead of «could not read the player config», the Overseer reads «Не удалось прочитать файл игроков. Выберите его заново или создайте новый.» Every error the Overseer sees is in Russian and says what to do next. The original technical text stays in the log and under «подробности для разработчика».

**Why this priority**: Errors are rare but block the Overseer when they happen. This story depends on the glossary from User Story 6.

**Independent Test**: Trigger each covered failure in the browser fixture and check that the message is in Russian, contains a next step, and shows no English text outside «подробности для разработчика».

**Acceptance Scenarios**:

1. **Given** an unreadable player configuration file, **When** the Overseer opens the terminals tab, **Then** the message reads «Не удалось прочитать файл игроков. Выберите его заново или создайте новый.»
2. **Given** a session file that cannot be saved, **When** the failure is shown, **Then** the message is a specific Russian message that names the file problem and a next step.
3. **Given** a public access or update failure, **When** it is shown, **Then** the Overseer sees a general Russian message with a next step, and the original English text only under «подробности для разработчика».

---

### Edge Cases

- A device with many states: the matrix scrolls inside its own frame, the state headers stay readable, and the dialog's buttons stay visible.
- Two transitions between the same pair of states: the cell lists both names and offers adding a third.
- A device with one state: the matrix has a single unavailable cell, and the dialog says that a transition needs a second state.
- Deleting a state used by this device's transitions: the dialog names those transitions and deletes them with the state after confirmation. A state used outside the device (another device's precondition, a display rule, a fault, a program) is not deleted until those uses change. Each use links to its place.
- Applying a template whose state name the device already has under another spelling case: the existing state counts as present, because state names are unique regardless of case.
- A fault that starts active and is turned on by no transition: «Как появляется» says it is active from the start and warns that after it clears, only the Overseer can turn it on again.
- A program created from a fault and then canceled: nothing is saved, and the fault dialog is unchanged.
- A fault with no clearing transition and no program: it saves, and «Как устраняется» lists only «Смотритель вручную».
- A transition that a command or a program runs together with another transition that has the opposite effect on the same fault: it is not offered under «Как появляется» or «Как устраняется», because one action cannot both turn a fault on and clear it.
- An element with a visibility or availability rule already: adding a second one of the same type offers to edit the existing rule instead.
- A rule whose element was deleted while the dialog was open: «Готово» reports the missing element by name, and nothing saves.
- The selected device leaves the active filter during play: the details stay open and the list shows it as hidden by the filter.
- A failure arrives while a dialog is open: the message appears in the dialog, and nothing the author typed is lost.
- Sessions saved before this feature: they open unchanged, and every existing rule, action, fault, and program opens in its new dialog.

## Requirements *(mandatory)*

### Functional Requirements

**Device editor (F-10)**

- **FR-001**: The device dialog MUST show, in order: name, type, the type's template offer, states, transitions, and a collapsed «Технические идентификаторы» block at the bottom.
- **FR-002**: States MUST be shown as chips, with the starting state starred on its chip. Choosing a chip MUST let the author rename the state, make it the starting state, or delete it. «+ Состояние» MUST add a state.
- **FR-003**: The separate starting-state list MUST be removed. A new device's first state MUST be the starting state until the author stars another.
- **FR-004**: Transitions MUST be shown as a matrix with one row per source state and one column per destination state. Cells where source and destination match MUST be unavailable. A filled cell MUST show its transitions' names and a summary of their preconditions and fault effects, for example «1 предусловие · снимает 1 неисправность».
- **FR-005**: Choosing an empty cell MUST start a new transition between that pair of states. Choosing a filled cell MUST open its transition, and a cell with several transitions MUST list them and offer adding another.
- **FR-006**: The transition editor MUST show the transition's name, source and destination states, and preconditions, all editable. It MUST also list its fault effects, each naming the fault and whether the transition activates or clears it, with a «К неисправности» link. Fault effects are edited in the fault dialog (FR-017).
- **FR-007**: Identifiers of the device, its states, and its transitions MUST be generated from names as today and MUST appear only inside «Технические идентификаторы». Saved identifiers MUST NOT change. When a problem concerns an identifier, the block MUST expand and focus that row.
- **FR-008**: Deleting a state MUST name every use of it. This device's transitions that start or end in it MUST be deleted with it after confirmation. A state MUST NOT be deleted while it is used outside the device, directly or through one of those transitions, and each use MUST link to its place. The starting state and the device's current state MUST NOT be deleted: the dialog says to star another state or to move the device first.

**Templates (F-10)**

- **FR-009**: Every device type except «Другой» MUST have a template with at least two states, a starting state, and transitions in both directions. The «Дверь» template MUST be the states «Заперта» (starting) and «Открыта» with the transitions «Открыть» (Заперта → Открыта) and «Запереть» (Открыта → Заперта).
- **FR-010**: The dialog MUST name the template's states and transitions next to «Применить шаблон» before it is applied.
- **FR-011**: Applying a template MUST add only the states and transitions whose names the device does not have yet. It MUST NOT rename or remove anything, and MUST set the starting state only when the device has none.
- **FR-012**: Changing a device's type MUST NOT change its states or transitions.

**Faults and recovery programs (F-12)**

- **FR-013**: The fault dialog MUST be titled «Неисправность» and MUST hold every fault setting: name, category, where it acts (a device or a terminal), what it blocks or changes, its position at game start, «Как появляется», and «Как устраняется».
- **FR-014**: «Как появляется» MUST list every transition that activates the fault, by device and transition name, and MUST let the author add one with «+ Включать при переходе…» or remove one. With none, it MUST warn that the fault can only be turned on by hand.
- **FR-015**: «Как устраняется» MUST list every transition that clears the fault, every recovery program that contains such a transition, and «Смотритель вручную». It MUST let the author add or remove a clearing transition and create a program with «+ Программа восстановления…».
- **FR-016**: A program created from «Как устраняется» MUST start with a transition that clears that fault, and «Готово» or «Отмена» in the program dialog MUST return the author to the fault dialog.
- **FR-017**: A transition's fault effects MUST be edited only in the fault dialog. Every other place MUST show them read-only and link to the fault.
- **FR-018**: The recovery program dialog MUST list its transitions as one uniform list of rows (device and transition), each removable, with one add control and no multi-sentence instruction. It MUST name the commands that run the program and warn when none does.
- **FR-019**: Choosing a fault in any list MUST open its fault dialog.
- **FR-020**: The Overseer MUST be able to clear any fault by hand, device-scoped or terminal-scoped. «Как устраняется» MUST always list «Смотритель вручную», and the «Восстановление оператором» checkbox MUST be removed. A stored «Восстановление оператором» value MUST stay in the file unchanged, and a fault MUST save without a clearing transition or program.
- **FR-021**: Terminal-scoped faults MUST be listed in a «Неисправности терминалов» group at the end of the device list, visible whether or not a device is selected. Each row MUST name the fault and its terminal, mark it when active, open its fault dialog, and offer the same on/off switch the device card gives device faults. Search and filters MUST NOT narrow this group.

**Display rules and command actions (F-11)**

- **FR-022**: The display-rule dialog MUST be titled «Правило отображения» and MUST offer its type as tabs at the top: «Текст записи», «Название в меню», «Показать элемент», «Разрешить команду».
- **FR-023**: Fields MUST appear in sentence order: «КОГДА» (device «=» state), «ГДЕ» (element), «БЛОК» (only «Текст записи»), «ТЕКСТ» (only «Текст записи» and «Название в меню»). Only the selected type's fields and their labels MUST be visible.
- **FR-024**: The element choice MUST show each element's path through folders and MUST offer only elements the type applies to: records for «Текст записи», commands for «Разрешить команду», and any element except the terminal's root for the others.
- **FR-025**: The block choice MUST label each block by its number and opening text.
- **FR-026**: Switching tabs on a new rule MUST keep the device, the state, and the element when the new type accepts them. An existing rule's type MUST NOT change: its other tabs are unavailable with a hint to create a new rule.
- **FR-027**: The dialog MUST say what players see in the device's other states, for example «В остальных состояниях останется исходный текст блока».
- **FR-028**: The dialog MUST name the existing rule, offer to open it, and MUST NOT save a conflicting rule when:
  - the element's name or text rules already depend on another device;
  - a rule for the same element, device, and state already exists;
  - the element already has a visibility or availability rule.
- **FR-029**: «Действие команды» MUST NOT be a type in the display-rule dialog. A command's device action MUST be edited in its own dialog «Действие команды». That dialog MUST open from the command form, from the «Что делают игроки» tab, and from the scenario guide's action step. It MUST offer either transitions or a recovery program. Existing actions MUST keep working and open there, including those that run a program.

**Device list (F-14)**

- **FR-030**: Each device row MUST be left-aligned, with the name and the type under it. The state MUST be a badge on the right with a yellow frame when it differs from the starting state. Active faults MUST be a red mark with their count.
- **FR-031**: Above the list MUST sit a search field «Название или состояние» and the filters «Все», «Изменённые», «С неисправностью», each with its count over the whole session. «Изменённые» MUST mean the current state differs from the starting state. Search and filter MUST combine.
- **FR-032**: Rows, badges, marks, counts, and filter results MUST update while the list is open when a state or fault changes.
- **FR-033**: When nothing matches, the list MUST say so and offer «Все». The selected device's details MUST stay open when the filter hides its row. The filter choice MUST last for the current application run only.

**Vocabulary and links (F-13)**

- **FR-034**: One glossary MUST define the facility terms Локация, Устройство, Состояние, Переход, Правило отображения, Действие команды, Неисправность, Программа восстановления, and the dialog pair Готово / Отмена. Every Overseer facility string MUST use them.
- **FR-035**: «объект» MUST NOT name a device or the whole location, and «диагностическое условие», «условие», and «Восстановить условие» MUST NOT name a fault or its clearing. An automated check MUST fail when a banned wording appears in the Overseer copy.
- **FR-036**: Every control that opens an existing device's dialog MUST read «Изменить состояния и переходы».
- **FR-037**: Each dependency MUST be described in words, naming what depends on what and where by name. Its button MUST name the destination: «К команде», «К неисправности», «К программе», «К устройству», or «К правилу отображения». The technical path MUST appear only under a collapsed «подробности для разработчика».
- **FR-038**: Facility validation problems MUST name entities in Russian by name and kind. Internal kinds and identifiers MUST appear only under «подробности для разработчика».

**Errors (F-13)**

- **FR-039**: Every error anywhere in the Overseer window MUST be in Russian and MUST say what to do next. English or technical text MUST appear only under «подробности для разработчика» and in the logs. Facility workspace, player configuration, and session file failures MUST each have a specific message.
- **FR-040**: The unreadable player configuration error MUST read «Не удалось прочитать файл игроков. Выберите его заново или создайте новый.»
- **FR-041**: Every other failure, for example from public access, updates, or credential storage, MUST show a general Russian message with a next step and its original text under «подробности для разработчика». It MUST NOT show raw text alone.

**Compatibility, demo, tests**

- **FR-042**: Sessions MUST stay in portable session JSON version 1. No field is added or renamed. The only stored value whose meaning changes is «Восстановление оператором» (FR-020). Files saved before this feature MUST open and save unchanged.
- **FR-043**: Every edit in the new dialogs MUST save through the existing authoring save, with its revision check and «Отменить» history.
- **FR-044**: The demo sessions and the capability-to-path inventory MUST include a reachable path for each case: applying a template, a matrix cell with two transitions, each of the four rule types, a command action opened from the command form, a fault with and without an activating transition, a recovery program with at least two transitions, a terminal-scoped fault in its place, every dependency destination, and each list filter with a nonzero count.
- **FR-045**: Each user story MUST have a browser test that exercises it in the Overseer and, where players see the result, on a connected player terminal.

### Key Entities

- **Device template**: the suggested states, starting state, and transitions for one device type. It exists only in the editor and is never stored in a session.
- **Transition matrix**: the device's transitions arranged by source state (rows) and destination state (columns). A cell holds zero or more transitions.
- **Display rule**: what a terminal shows while a device is in a state. It has four types: record text, menu name, element visibility, and command availability. Each has a device and state («КОГДА»), an element («ГДЕ»), and for text types a block and a text.
- **Command action**: the transitions or recovery program a player command runs. It belongs to the command, not to a display rule.
- **Fault**: a problem that blocks or changes what a terminal can do, scoped to one device or one terminal. «Как появляется» and «Как устраняется» are views over the transitions, programs, and Overseer controls that turn it on and off.
- **Recovery program**: a named list of transitions that a command runs to clear faults.
- **Glossary**: the one list of facility terms and banned synonyms that the Overseer copy is checked against.
- **Dependency link**: one place that refers to a device, described in words, with a named destination and a technical path for developers.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At 1440×900, every demo device's states and full transition matrix are visible in the device dialog without scrolling.
- **SC-002**: No facility editing dialog shows an identifier field until «Технические идентификаторы» is expanded.
- **SC-003**: After naming it, a new door with two states and both transitions takes three actions: pick «Дверь», press «Применить шаблон», press «Готово».
- **SC-004**: For every demo fault, «Как появляется» and «Как устраняется» list exactly the transitions and programs that turn it on and off in the session data.
- **SC-005**: Each of the four display-rule types can be created with fields in the order «КОГДА», «ГДЕ», then what to show, and no label without a field appears on any tab.
- **SC-006**: In the demo, «Изменённые» lists exactly the five devices whose state differs from the starting one, and «С неисправностью» lists exactly the devices with an active fault.
- **SC-007**: The vocabulary check finds zero banned wordings in the Overseer copy.
- **SC-008**: No error message in the Overseer window shows English text outside «подробности для разработчика», and each one names a next step.
- **SC-009**: Every demo session file opens and saves with no format change.

## Assumptions

- The tab keeps its name «Объекты», as in the review's own mockups. It names the workspace, not a device or the location. The scenario guide's first step becomes «Устройство».
- The glossary and the vocabulary check cover the facility workspace and the dialogs it opens. Other Overseer dialogs keep their buttons («СОХРАНИТЬ…», «ЗАКРЫТЬ»), and confirmation dialogs keep naming their action («СБРОСИТЬ», «ПЕРЕВЕСТИ»).
- Templates for types other than «Дверь» follow the demo device of the same type. Their exact names are chosen in the plan.
- A template is offered, never applied by itself.
- Feature 040's play controls and the fault switch keep their behavior, and the matrix is an authoring view. It does not run transitions.
- «Изменённые» compares only states. A device whose faults differ from their starting positions shows under «С неисправностью» when one is active.
- The player terminal's own wording is not part of this feature.

## Out of Scope

- Drawing the state graph as nodes and arrows. The matrix replaces the transition list.
- Transitions with several source states or an "any state" source.
- Renaming the «Объекты» tab or the session file's field names.
- Changing what players see, how commands run, or the precedence rules for display.
- New play controls beyond giving terminal-scoped faults the switch that device faults already have.

## Verbatim Constraints

- `040-facility-play-controls`: this feature stacks on that branch.
- Glossary terms: «Локация», «Устройство», «Состояние», «Переход», «Правило отображения», «Действие команды», «Неисправность», «Программа восстановления», «Готово», «Отмена».
- Device dialog copy: «Применить шаблон», «+ Состояние», «Технические идентификаторы», «Изменить состояния и переходы».
- «Дверь» template: «Заперта», «Открыта», «Открыть», «Запереть».
- Display-rule copy: «Правило отображения», «Текст записи», «Название в меню», «Показать элемент», «Разрешить команду», «КОГДА», «ГДЕ», «БЛОК», «ТЕКСТ».
- Fault copy: «Неисправность», «Неисправности терминалов», «Как появляется», «Как устраняется», «+ Включать при переходе…», «+ Программа восстановления…», «Смотритель вручную», «Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.»
- Link copy: «К команде», «К неисправности», «подробности для разработчика».
- Device list copy: «Все», «Изменённые», «С неисправностью», «Название или состояние».
- Error copy: «Не удалось прочитать файл игроков. Выберите его заново или создайте новый.»
