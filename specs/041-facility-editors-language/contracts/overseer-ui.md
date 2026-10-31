# Overseer UI Contract: Facility Editors and Language

Browser tests and helpers may rely on the DOM ids, classes, data attributes, copy, and JavaScript
names listed below. Operations are defined in `private-desktop.md`. The branch stacks on
`040-facility-play-controls`. This contract extends
`specs/040-facility-play-controls/contracts/overseer-ui.md` and replaces its «Device list» section,
its stale play text, and its «ОБЪЕКТ» guide step. An id that this contract does not name keeps its
039 or 040 meaning. Every removed id is listed under «Removed elements».

## Conventions

- **Case (R15).** Controls, labels, legends, dialog titles, tab names, and statuses render in upper
  case. Hints, warnings, sentences, and error messages stay in sentence case. Authored names
  (devices, states, transitions, faults, programs, terminals, elements) keep their case. The
  Overseer has no CSS `text-transform`, so every string below is literal in the source.
- **Blocked controls.** As in 039 and 040: `aria-disabled="true"`, focus kept, `aria-describedby`
  pointing to a `.control-reason` element, and activation does nothing.
- **Draft links.** A link that leaves a dialog with unsaved changes is blocked with the reason
  «СНАЧАЛА НАЖМИТЕ «ГОТОВО»». This covers «К НЕИСПРАВНОСТИ» in the transition panel, the destination
  buttons of a state's uses, and `#btnPreviewFacilityCondition`. A dialog has unsaved changes when
  its draft differs from the session it opened on.
- **Stacked dialogs.** The program dialog opened from the fault dialog opens with `showModal` over
  it. The fault dialog stays open, and its draft stays in the DOM. Closing the upper dialog returns
  focus to the control that opened it.
- **Saving.** Every «ГОТОВО» commits one `commitFacilityEdit(label, mutate)`, which gives one
  revision check and one «ОТМЕНИТЬ» entry (FR-043). The labels are «Устройство «<name>»»,
  «Неисправность «<name>»», «Программа «<name>»», «Правило «<element>»», and «Действие команды
  «<command>»».

## Verbatim copy map

Every verbatim string from the spec renders as follows. `…` is the single character U+2026.

| Spec copy (verbatim) | Rendered | Where |
|---|---|---|
| `040-facility-play-controls` | — | base branch of this feature |
| «Локация» | declined in sentences («в локации»), «…ЛОКАЦИИ» in labels | glossary term for the whole scene; `#facilityDetailPanel` `aria-label="Сведения о локации"`, `#facilityDependencyDialogDescription`, facility error messages |
| «Устройство» | «УСТРОЙСТВО» | `#facilityDeviceDialogTitle`; guide step label; `#facilityConditionScope` option |
| «Состояние» | «СОСТОЯНИЯ», «НАЗВАНИЕ СОСТОЯНИЯ» | legend of `#facilityDeviceStatesField`; label of `#facilityStateEditorName` |
| «Переход» | «ПЕРЕХОДЫ», «ПЕРЕХОД: <из> → <в>» | legend of `#facilityDeviceTransitionsField`; `#facilityTransitionPanelHeading` |
| «Правило отображения» | «ПРАВИЛО ОТОБРАЖЕНИЯ» | `#facilityBindingDialogTitle` |
| «Действие команды» | «ДЕЙСТВИЕ КОМАНДЫ» | `#facilityCommandActionDialogTitle`; `#commandFacilityActionHeading` |
| «Неисправность» | «НЕИСПРАВНОСТЬ» | `#facilityConditionDialogTitle` |
| «Программа восстановления» | «ПРОГРАММА ВОССТАНОВЛЕНИЯ» | `#facilityRecoveryProgramDialogTitle`; action mode option |
| «Готово» | «ГОТОВО» | submit button of every facility editing dialog |
| «Отмена» | «ОТМЕНА» | cancel button of every facility editing dialog |
| «Применить шаблон» | «ПРИМЕНИТЬ ШАБЛОН» | `#btnApplyFacilityDeviceTemplate` |
| «+ Состояние» | «+ СОСТОЯНИЕ» | `#btnAddFacilityState` |
| «Технические идентификаторы» | «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ» | `summary` of `#facilityDeviceTechnical`, `#facilityConditionTechnical`, `#facilityRecoveryProgramTechnical` |
| «Изменить состояния и переходы» | «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ» | `#btnEditFacilityDevice` and every guide control that opens an existing device's dialog |
| «Заперта», «Открыта», «Открыть», «Запереть» | as written | the «Дверь» template (authored names) |
| «Текст записи» | «ТЕКСТ ЗАПИСИ» | `#facilityRuleTabEntryContent` |
| «Название в меню» | «НАЗВАНИЕ В МЕНЮ» | `#facilityRuleTabMenuName` |
| «Показать элемент» | «ПОКАЗАТЬ ЭЛЕМЕНТ» | `#facilityRuleTabVisibility` |
| «Разрешить команду» | «РАЗРЕШИТЬ КОМАНДУ» | `#facilityRuleTabAvailability` |
| «КОГДА», «ГДЕ», «БЛОК», «ТЕКСТ» | as written | sentence labels of the rule dialog |
| «Неисправности терминалов» | «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» | `#facilityTerminalFaultsHeading` |
| «Как появляется» | «КАК ПОЯВЛЯЕТСЯ» | `#facilityFaultAppearsHeading` |
| «Как устраняется» | «КАК УСТРАНЯЕТСЯ» | `#facilityFaultClearsHeading` |
| «+ Включать при переходе…» | «+ ВКЛЮЧАТЬ ПРИ ПЕРЕХОДЕ…» | `#btnAddFaultActivation` |
| «+ Программа восстановления…» | «+ ПРОГРАММА ВОССТАНОВЛЕНИЯ…» | `#btnAddFaultRecoveryProgram` |
| «Смотритель вручную» | «Смотритель вручную» | the `data-kind="overseer"` row of `#facilityFaultClearsList`. It is a list entry next to authored names, so it keeps sentence case |
| «Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.» | as written | `#facilityFaultAppearsWarning` |
| «К команде» | «К КОМАНДЕ» | dependency and use destination button |
| «К неисправности» | «К НЕИСПРАВНОСТИ» | dependency, use, and transition-panel destination button |
| «подробности для разработчика» | «подробности для разработчика» | `summary` of every developer disclosure. It belongs to the message it follows, so it keeps sentence case |
| «Все» | «ВСЕ <n>» | `#facilityDeviceFilterAll`; «ВСЕ» on `#btnFacilityDeviceFilterReset` |
| «Изменённые» | «ИЗМЕНЁННЫЕ <n>» | `#facilityDeviceFilterChanged` |
| «С неисправностью» | «С НЕИСПРАВНОСТЬЮ <n>» | `#facilityDeviceFilterFaulty` |
| «Название или состояние» | «Название или состояние» | `placeholder` of `#facilityDeviceSearch` |
| «Не удалось прочитать файл игроков. Выберите его заново или создайте новый.» | as written | `#playerConfigError` for `could not read the player config` |

The other destination buttons render «К ПРОГРАММЕ», «К УСТРОЙСТВУ», and «К ПРАВИЛУ ОТОБРАЖЕНИЯ».

**FR-036 and destination links.** FR-036 covers edit controls: every button whose purpose is to edit
an existing device reads «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ». «К УСТРОЙСТВУ» is a destination link
(FR-037), not an edit control. It keeps its name even though it opens the device dialog (R13).

## Glossary and vocabulary check (FR-034, FR-035, R14)

`docs/overseer-glossary.md` is the one glossary. Its human part is a table with the columns
«Термин», «Что это», and «Не называть». It has one row for each of «Локация», «Устройство»,
«Состояние», «Переход», «Правило отображения», «Действие команды», «Неисправность», «Программа
восстановления», and «Готово / Отмена». The «Смотритель вручную» row replaces «Восстановление
оператором».

Its machine part is two fenced blocks that `tests/browser/facility-vocabulary.spec.mjs` reads from
the file:

````text
```vocabulary-banned
(?<!\p{L})объект\p{L}*
диагностическ\p{L}*\s+услови\p{L}*
(?<!\p{L})услови\p{L}*
восстановить\s+услови\p{L}*
восстановлени\p{L}*\s+оператором
```

```vocabulary-allowed
ОБЪЕКТЫ И СОСТОЯНИЯ
ОБЪЕКТЫ
```
````

- Each `vocabulary-banned` line is a JavaScript regular expression source, compiled with the flags
  `iu`. `\b` is never used, because it does not work on Cyrillic. The lookbehind lets «предусловие»
  pass.
- Each `vocabulary-allowed` line is a case-sensitive literal phrase. The test removes every
  occurrence of each phrase from the source before matching, longest phrase first. Only the tab
  «ОБЪЕКТЫ» and the heading «ОБЪЕКТЫ И СОСТОЯНИЯ» use them.
- The test fetches the served `index.html`, `overseer.js`, and `desktop-api.js` through the fixture
  server. Comments count. It reports each hit as `<file>:<line>: <match>` and passes only with zero
  hits (SC-007).
- The 039 `REMOVED_COPY` sweep stays as it is.

## Device list (FR-030 to FR-033, R11)

This section replaces the 040 «Device list» section. `#facilityDeviceSidebar` children appear in
this order:

1. the header with `#facilityDevicesHeading` «УСТРОЙСТВА» and `#btnAddFacilityDevice`;
2. `label[for="facilityDeviceSearch"]` «Поиск устройств» and `#facilityDeviceSearch`;
3. `#facilityDeviceFilters`;
4. `#facilityDeviceHiddenSelection`;
5. `#facilityDeviceList` (`aria-label="Устройства локации"`);
6. `#facilityTerminalFaults`.

The sidebar element is `section.facility-device-sidebar` and gains the id `facilityDeviceSidebar`.

### Search and filters

| Element | Contract |
|---|---|
| `#facilityDeviceSearch` | `placeholder="Название или состояние"`. Matches, case-insensitively after trimming, a substring of the device name or of its current state name. It no longer matches the ID |
| `#facilityDeviceFilters` | `role="group"`, `aria-label="Фильтр устройств"`. It holds three `button.facility-device-filter` in this order |
| `#facilityDeviceFilterAll` | `data-filter="all"`, text «ВСЕ <n>», where n counts every device in the session |
| `#facilityDeviceFilterChanged` | `data-filter="changed"`, text «ИЗМЕНЁННЫЕ <n>», where n counts devices with `currentStateId !== initialStateId` |
| `#facilityDeviceFilterFaulty` | `data-filter="faulty"`, text «С НЕИСПРАВНОСТЬЮ <n>», where n counts devices with at least one active device-scoped fault |
| `span.facility-filter-count` | inside each filter, holds n; the filter also has `data-count="<n>"` |

- Exactly one filter has `aria-pressed="true"`, and «ВСЕ» is the default. Pressing a filter
  selects it.
- Counts are taken over the whole session. Search and the active filter never change them.
- Search and filter combine with AND.
- The choice lives in a module variable next to `selectedFacilityDeviceID`. Re-renders, tab
  switches, and session changes keep it. Nothing is stored, so a restart starts at «ВСЕ».
- With the demo session: «ВСЕ 10», «ИЗМЕНЁННЫЕ 5», «С НЕИСПРАВНОСТЬЮ 1». With «ИЗМЕНЁННЫЕ» on,
  searching «В работе» leaves only «Резервная сеть Убежища 76» (SC-006).

### Rows

Each row is `button[data-facility-kind="device"][data-facility-id="<deviceId>"]`, in authored order:

| Part | Contract |
|---|---|
| `span.facility-device-row-text` | left-aligned. `strong` holds the device name, and `span.facility-device-row-kind` below it holds the kind label from `facilityDeviceKindLabel` (for example «ДВЕРЬ»; a custom device reads «ДРУГОЙ», because its stored kind is derived from the identifier) |
| `span.facility-state-badge` | right side. Holds the current state name. `data-changed="true"` when the current state differs from the starting state, otherwise `"false"` |
| `span.facility-fault-mark` | after the badge, only when n > 0 device-scoped faults are active. Text «● <n>», `aria-label="Активных неисправностей: <n>"` |
| data attributes | `data-current-state-id`, `data-active-faults` (`"0"` when none), and `data-facility-selected` stay. `aria-pressed` stays |
| `title` | none. Rows no longer reveal IDs on hover |

- **Layout.** The name and kind start at the row's left content edge. The badge and the mark end
  at its right content edge.
- **Colors.** A `data-changed="true"` badge has a visible yellow border: in its computed
  `border-top-color`, the red and green channels are each at least twice the blue channel. A
  `"false"` badge has no such border. The fault mark is red: its computed `color` has a red channel
  greater than both the green and blue channels.
- **Live updates (FR-032).** The list re-renders on every session change, so rows, badges, marks,
  counts, and the filtered set follow play changes from either side without a reload.

### Hidden selection and empty list (FR-033)

| Element | Contract |
|---|---|
| `#facilityDeviceHiddenSelection` | «Выбранное устройство скрыто фильтром». Shown only while a device is selected and the filter or search hides its row. The details area stays open |
| empty list | When devices exist but none matches, `#facilityDeviceList` holds `p.facility-empty` «Ни одно устройство не подходит под фильтр и поиск.» and `#btnFacilityDeviceFilterReset` «ВСЕ». That button selects «ВСЕ», clears the search, and focuses `#facilityDeviceFilterAll` |
| no devices | The 040 text «Добавьте первое устройство, затем задайте его состояния и переходы.» stays |

### Terminal faults (FR-021, R11)

`section#facilityTerminalFaults` is labelled by `#facilityTerminalFaultsHeading` «НЕИСПРАВНОСТИ
ТЕРМИНАЛОВ». It is always visible, with or without a selected device, and search and filters never
narrow it.

| Element | Contract |
|---|---|
| `ul#facilityTerminalFaultList` | `role="list"`. One item per terminal-scoped fault, in authored order |
| `li.facility-terminal-fault` | `data-facility-kind="terminal-condition"`, `data-facility-id` and `data-condition-id` set to the fault ID, `data-active="true"` or `"false"` |
| `button.facility-play-fault-switch` | The 040 switch: `role="switch"`, `aria-checked` set to the current position, text = fault name. One press runs `runFacilityFaultSwitch(id, !active)`, which calls `desktopAPI.setFacilityConditionActive` without confirmation |
| `.facility-play-fault-current` | «АКТИВНА» or «НЕАКТИВНА» |
| `.facility-terminal-fault-terminal` | «ТЕРМИНАЛ: <terminal name>» |
| `button.facility-terminal-fault-open` | «ИЗМЕНИТЬ», `aria-label="Изменить неисправность: <name>"`, `aria-haspopup="dialog"`. Opens `#facilityConditionDialog` for the fault |
| `span.control-reason` | `id="facilityTerminalFaultReason<i>"`, the switch's reason |
| `#facilityTerminalFaultsEmpty` | «Неисправностей терминалов нет.», shown instead of the list when the session has none |
| `#facilityTerminalFaultStatus` | `role="status"`, `aria-live="polite"`, with `data-state` set to `ok`, `stale`, or `error`. Success reads «НЕИСПРАВНОСТЬ АКТИВИРОВАНА» or «НЕИСПРАВНОСТЬ СНЯТА». A failure is rendered by `renderOverseerError(…, 'facility')`. A stale result first applies `result.session`, as on the 040 card |

The switch shares the 040 play reasons: «НЕТ ОТКРЫТОЙ СЕССИИ», «ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА», and
«ДОЖДИТЕСЬ СОХРАНЕНИЯ». Device cards still list only device-scoped faults. With the demo session,
the group lists «Закрытый канал изолирован» (active, «ТЕРМИНАЛ: Терминал смотрителя — аварийный
удалённый доступ») and the other three terminal faults.

## Device dialog (FR-001 to FR-012, R8 to R10)

`#facilityDeviceDialog` and `#facilityDeviceForm` keep their ids. `#facilityDeviceForm` renders its
children in this order:

1. `#facilityDeviceDialogTitle` «УСТРОЙСТВО»;
2. `#facilityDeviceDialogDescription`, one sentence: «Название и тип, затем состояния и переходы
   между ними.»;
3. `label[for="facilityDeviceName"]` «НАЗВАНИЕ» and `#facilityDeviceName`;
4. `label[for="facilityDeviceKind"]` «ТИП» and `#facilityDeviceKind`, with unchanged options;
5. `#facilityDeviceTemplate`;
6. `fieldset#facilityDeviceStatesField`;
7. `fieldset#facilityDeviceTransitionsField`;
8. `details#facilityDeviceTechnical`;
9. `#facilityDeviceStatus` and `#facilityDeviceError`;
10. the actions `#btnCancelFacilityDevice` «ОТМЕНА» and `#btnSaveFacilityDevice` «ГОТОВО».

The dialog opens with focus on `#facilityDeviceName`. One draft holds every state and transition.
Each draft row keeps `facilityOriginal` and is read with a spread, so `transition.recovery`,
`conditionEffects`, and unknown fields survive (R8).

### Template (FR-009 to FR-012, R9)

| Element | Contract |
|---|---|
| `#facilityDeviceTemplate` | Hidden when `#facilityDeviceKind` is empty or `custom`. Shown for the nine other kinds |
| `#facilityDeviceTemplateLabel` | «ШАБЛОН: <kind label>», for example «ШАБЛОН: ДВЕРЬ» |
| `#facilityDeviceTemplateSummary` | The template's state names joined by «, », then « · », then its transition names joined by «, ». For «Дверь»: «Заперта, Открыта · Открыть, Запереть» |
| `#btnApplyFacilityDeviceTemplate` | «ПРИМЕНИТЬ ШАБЛОН», `aria-describedby="btnApplyFacilityDeviceTemplateReason"`. Blocked with «ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ» when applying would add nothing |
| status | After an apply, `#facilityDeviceStatus` reads «ИЗ ШАБЛОНА ДОБАВЛЕНО: СОСТОЯНИЙ — <s>, ПЕРЕХОДОВ — <t>» |

**Apply rules.**

- A template state counts as present when a draft state has the same name after trimming,
  compared case-insensitively. Missing states are appended.
- A template transition counts as present when a draft transition has the same name, compared the
  same way, between the same pair of states. The pair is resolved through the state names. Missing
  transitions are appended.
- The starting state is set only when the draft has none. Nothing is renamed or removed.
- IDs come from `suggestFacilityID`, as for typed names.
- Changing `#facilityDeviceKind` never changes states or transitions. It only changes the offer.
- A template is offered and never applied by itself.

`FACILITY_DEVICE_TEMPLATES` in `overseer.js` holds the table below. Templates are editor-only and
are never stored (Key Entities). Names follow the demo device of the same kind, with the object
dropped from the verb. ★ marks the starting state.

| Kind (`value`) | States | Transitions |
|---|---|---|
| `door` | ★«Заперта», «Открыта» | «Открыть» (Заперта → Открыта), «Запереть» (Открыта → Заперта) |
| `turret` | ★«Безопасный режим», «Сопровождение» | «Включить сопровождение» (Безопасный режим → Сопровождение), «Вернуть безопасный режим» (Сопровождение → Безопасный режим) |
| `power-grid` | ★«Нет питания», «В работе» | «Запустить» (Нет питания → В работе), «Отключить» (В работе → Нет питания) |
| `reactor` | ★«Аварийная остановка», «Дежурный режим» | «Перевести в дежурный режим» (Аварийная остановка → Дежурный режим), «Остановить» (Дежурный режим → Аварийная остановка) |
| `ventilation` | ★«Загрязнён», «Очищен» | «Продуть» (Загрязнён → Очищен), «Смоделировать загрязнение» (Очищен → Загрязнён) |
| `alarm` | ★«Тишина», «Эвакуация» | «Подать сигнал» (Тишина → Эвакуация), «Отключить сигнал» (Эвакуация → Тишина) |
| `robot-pod` | ★«Зарядка», «Патруль» | «Отправить в патруль» (Зарядка → Патруль), «Вернуть на зарядку» (Патруль → Зарядка) |
| `elevator` | ★«Нижний уровень», «Верхний уровень» | «Поднять кабину» (Нижний уровень → Верхний уровень), «Опустить кабину» (Верхний уровень → Нижний уровень) |
| `network-segment` | ★«Изолирована», «Подключена» | «Подключить» (Изолирована → Подключена), «Изолировать» (Подключена → Изолирована) |
| `custom` | — | no template (FR-009) |

SC-003: after the name is typed, a new door takes three actions: pick «ДВЕРЬ», press «ПРИМЕНИТЬ
ШАБЛОН», press «ГОТОВО».

### States (FR-002, FR-003, FR-008, R10)

`fieldset#facilityDeviceStatesField` has the legend «СОСТОЯНИЯ».

| Element | Contract |
|---|---|
| `#facilityDeviceStateChips` | `role="list"`, `aria-label="Состояния устройства"`. One `span[role="listitem"]` per draft state, in draft order, each holding one chip |
| `button.facility-state-chip` | `data-state-id` (the draft ID), `data-initial="true"` or `"false"`, `aria-expanded`, `aria-controls="facilityStateEditor"`. Text: `span.facility-state-star` «★» (`aria-hidden="true"`, only on the starting state), then `span.facility-state-chip-name` with the state name. The starting chip's `aria-label` is «<name>, начальное состояние» |
| `#btnAddFacilityState` | «+ СОСТОЯНИЕ». Appends a state named «Новое состояние», with the next free «Новое состояние N» on a repeat, and opens its editor with the name selected |
| starting state | A new device's first state is starred until the author stars another. `#facilityDeviceInitialState` no longer exists anywhere |

Choosing a chip opens `#facilityStateEditor`, which is hidden until then and lies inside the
fieldset:

| Element | Contract |
|---|---|
| `label[for="facilityStateEditorName"]` | «НАЗВАНИЕ СОСТОЯНИЯ» |
| `#facilityStateEditorName` | class `facility-state-name`. Renames the state as the author types, and updates the chip and the matrix headers |
| `#btnMakeFacilityStateInitial` | «СДЕЛАТЬ НАЧАЛЬНЫМ». Moves the star. Blocked with «УЖЕ НАЧАЛЬНОЕ» on the starting state |
| `#btnDeleteFacilityState` | «УДАЛИТЬ», `btn-danger`. Blocked with «НАЧАЛЬНОЕ СОСТОЯНИЕ: СНАЧАЛА ОТМЕТЬТЕ ЗВЕЗДОЙ ДРУГОЕ» on the starting state. Blocked with «УСТРОЙСТВО СЕЙЧАС В ЭТОМ СОСТОЯНИИ: СНАЧАЛА ПЕРЕВЕДИТЕ ЕГО В ДРУГОЕ» on a saved device's current state |
| `#facilityStateUses` | `role="list"`. Lists uses after a delete press (see below) |
| `#facilityStateDeleteConfirm` | Hidden until needed. Holds `#facilityStateDeleteSummary`, `#btnConfirmDeleteFacilityState` «УДАЛИТЬ С ПЕРЕХОДАМИ» (`btn-danger`), and `#btnCancelDeleteFacilityState` «ОТМЕНА» |

**Delete press (FR-008, R10).** The client collects the state's uses from the draft and the
session:

1. **No use.** The state is removed from the draft.
2. **Only this device's transitions start or end in it.** `#facilityStateDeleteSummary` reads
   «Вместе с состоянием удалятся переходы: «<name>», «<name>».» Confirming removes the state and
   those transitions from the draft. Cancelling changes nothing.
3. **A use outside the device**, directly or through one of those transitions, blocks the delete.
   `#facilityStateUses` lists every use as `li.facility-state-use`, with `data-destination` set to
   `device`, `rule`, `command`, `program`, or `condition`. Each item holds a sentence in the
   dependency form (see «Dependency dialog») and its destination button. The outside uses are:
   - another device's precondition (`device`);
   - a display rule on this device and state (`rule`);
   - a command action (`command`), a recovery program (`program`), or a fault's stored
     `recovery[]` entry (`condition`) that names one of the transitions going with the state.

   Nothing is removed.

Nothing is saved until «ГОТОВО».

### Transition matrix (FR-004, FR-005)

`fieldset#facilityDeviceTransitionsField` has the legend «ПЕРЕХОДЫ».

| Element | Contract |
|---|---|
| `.facility-transition-matrix-frame` | Wraps the table and scrolls on its own (`overflow: auto`). The header row and the header column are sticky, so state names stay readable while it scrolls |
| `table#facilityTransitionMatrix` | `caption` «ИЗ → В». One `thead th[scope="col"][data-state-id]` per destination state and one `tbody tr` per source state, both in draft state order. Each row starts with `th[scope="row"][data-state-id]` holding the state name |
| `button.facility-transition-cell` | One per off-diagonal cell, with `data-from` and `data-to` set to the draft state IDs, `data-count="<k>"`, `aria-expanded`, and `aria-controls="facilityTransitionPanel"` |
| empty cell | Text «+». `aria-label` «Новый переход: <из> → <в>» |
| filled cell | For each transition in authored order: `span.facility-transition-cell-name` with the name and `span.facility-transition-cell-summary` with the summary. `aria-label` «<из> → <в>: <names joined by «, »>» |
| diagonal cell | `td.facility-transition-cell-unavailable[data-from][data-to]`, text «—», with no button inside |
| `#facilityTransitionMatrixHint` | «Добавьте состояния, чтобы задать переходы.» with no state. «Для перехода нужно второе состояние.» with one state, where the matrix has a single unavailable cell. Hidden otherwise |

**Summary format.** The parts are, in order, «<n> предусловие» (plural forms: «предусловия»,
«предусловий»), «включает <n> неисправность» (forms: «неисправности», «неисправностей»), and
«снимает <n> неисправность» (same forms). A zero part is left out. The parts are joined by « · »,
for example «1 предусловие · снимает 1 неисправность». A transition with no precondition and no
effect has an empty summary. Plural forms follow the Russian rule. The first form applies when
`n % 10 === 1 && n % 100 !== 11`. The second applies when `n % 10` is 2 to 4 and `n % 100` is not
12 to 14. The third applies otherwise.

**Choosing a cell.**

- **Empty cell.** Appends a draft transition between the pair and opens it in the panel. The
  suggested name is the device template's transition name for that pair of state names, or
  otherwise «Перевести в «<в>»». The name is selected.
- **Cell with one transition.** Opens it in the panel.
- **Cell with two or more.** Opens the panel with the cell's list first.

The cell updates from the draft immediately. «ГОТОВО» saves.

With the demo door, the row «Заперта» has «Открыть гермодверь» in the column «Открыта», and the
row «Открыта» has «Запереть гермодверь» in the column «Заперта». With the demo reactor (R16), the
cell «Аварийная остановка → Дежурный режим» lists «Перевести в дежурный режим» and «Пуск от
резервной сети».

### Transition panel (FR-005, FR-006, FR-017)

`section#facilityTransitionPanel` lies inside `#facilityDeviceTransitionsField`, after the matrix.
It is hidden until a cell is chosen, and it is labelled by `#facilityTransitionPanelHeading`
«ПЕРЕХОД: <из> → <в>».

| Element | Contract |
|---|---|
| `ul#facilityTransitionCellList` | Shown only for a cell with two or more transitions. One `button.facility-transition-choice[data-transition-id]` per transition with its name. The open one has `aria-pressed="true"` |
| `#btnAddFacilityCellTransition` | «+ ПЕРЕХОД». Shown when the cell already holds a transition. Appends another between the same pair (edge case: a third) |
| `.facility-transition-row[data-transition-id]` | Exactly one visible: the chosen transition. It contains the rows below |
| `label` «НАЗВАНИЕ ПЕРЕХОДА» + `input.facility-transition-name` | editable |
| `label` «ИЗ СОСТОЯНИЯ» + `select.facility-transition-source` | editable. Changing it moves the transition to another cell |
| `label` «В СОСТОЯНИЕ» + `select.facility-transition-destination` | editable |
| `fieldset.facility-transition-preconditions` | Legend «ПРЕДУСЛОВИЯ». One `.facility-precondition-row` per precondition: `select.facility-precondition-device`, the text «=», `select.facility-precondition-state`, and a remove button «УДАЛИТЬ» (`aria-label="Удалить предусловие"`). The device list leaves out this device and any device another row of this transition already uses |
| `button.facility-add-precondition` | «+ ПРЕДУСЛОВИЕ» |
| `section.facility-transition-fault-effects` | Heading «НЕИСПРАВНОСТИ». Read-only. One `li.facility-transition-fault-effect[data-condition-id][data-active]` per effect, with text «Включает «<fault>»» or «Снимает «<fault>»», and `button.facility-fault-link` «К НЕИСПРАВНОСТИ», which opens that fault's dialog. With no effect: `p.facility-empty` «Переход не включает и не снимает неисправности. Это задаётся в окне неисправности.» |
| `button[data-action="remove-facility-transition"]` | «УДАЛИТЬ ПЕРЕХОД», `btn-danger`. Removes the transition from the draft |
| `#btnCloseFacilityTransitionPanel` | «СВЕРНУТЬ». Hides the panel and returns focus to the cell |

Fault effects are edited only in the fault dialog (FR-017). The panel has no control that adds,
removes, or changes an effect.

### Technical identifiers (FR-007, SC-002)

`details#facilityDeviceTechnical` is the last block before the status. It is closed when the
dialog opens, and its `summary` is «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ». It holds every identifier input,
and no other place in the dialog shows one:

| Row | Contract |
|---|---|
| device | `label[for="facilityDeviceID"]` «УСТРОЙСТВО» + `#facilityDeviceID` |
| states | `.facility-state-row[data-state-id]`, one per draft state in draft order. Its label holds the state name, and its input is `input.facility-state-id` |
| transitions | `.facility-transition-id-row[data-transition-id]`, one per draft transition. Its label reads «<name>: <из> → <в>», and its input is `input.facility-transition-id` |

- IDs are generated from names through `suggestFacilityID` until the author edits that input.
- Saved IDs, meaning the device and every state and transition that came from the session, render
  `readonly` and never change (FR-007).
- When validation names an identifier, `setFacilityFeedback` opens `#facilityDeviceTechnical`,
  marks the row (`aria-invalid="true"` on the input and a `p.facility-row-error` note), and
  focuses the input.

### Validation

On «ГОТОВО», `validateFacilityDeviceDraft` runs before the save. The 039 messages stay, except
that the starting-state message becomes «ОТМЕТЬТЕ ЗВЕЗДОЙ НАЧАЛЬНОЕ СОСТОЯНИЕ». The focus target
for it is the first chip.

- A state-name problem opens that chip's editor and marks `#facilityStateEditorName`.
- A transition-name or state-pair problem opens that transition in the panel and marks the field.
- An identifier problem opens the technical block (FR-007).
- Every chip and cell with a problem gets `data-problem="true"`.
- `#facilityDeviceError` reads «ИСПРАВЬТЕ ОТМЕЧЕННОЕ: <n>».

### Layout guarantee (SC-001)

For every demo device, the dialog is opened from `#btnEditFacilityDevice` in a 1440×900 viewport
with the panel closed. The bounding boxes of `#facilityDeviceStateChips`,
`#facilityTransitionMatrix`, and `.facility-dialog-actions` then lie inside the viewport and
inside the visible part of `#facilityDeviceForm`, with `scrollTop = 0`. With eight states, the
matrix frame scrolls instead of the form, and `.facility-dialog-actions` stays in the viewport.

## Fault dialog (FR-013 to FR-020, R1 to R4)

`#facilityConditionDialog`, `#facilityConditionForm`, and the field ids below keep their ids.
`#facilityConditionForm` renders its children in this order:

| # | Element | Contract |
|---|---|---|
| 1 | `#facilityConditionDialogTitle` | «НЕИСПРАВНОСТЬ» |
| 2 | `#facilityConditionDialogDescription` | One sentence: «Что мешает героям, как это появляется и как устраняется.» |
| 3 | `#facilityConditionName` | label «НАЗВАНИЕ» |
| 4 | `#facilityConditionCategory` | label «КАТЕГОРИЯ», unchanged options |
| 5 | `#facilityConditionScope` | label «ГДЕ ДЕЙСТВУЕТ», options «УСТРОЙСТВО» (`device`) and «ТЕРМИНАЛ» (`terminal`). It shows either `#facilityConditionDevice` (label «УСТРОЙСТВО») or `#facilityConditionTerminal` (label «ТЕРМИНАЛ») |
| 6 | `#facilityConditionCapability` | label «ЧТО БЛОКИРУЕТ», unchanged options |
| 7 | `fieldset.facility-condition-initial` | legend «В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА», `#facilityConditionInitialWorking` «ИСПРАВНО», `#facilityConditionInitialActive` «НЕИСПРАВНОСТЬ АКТИВНА» |
| 8 | `section#facilityFaultAppears` | «КАК ПОЯВЛЯЕТСЯ» (below) |
| 9 | `section#facilityFaultClears` | «КАК УСТРАНЯЕТСЯ» (below) |
| 10 | `#btnPreviewFacilityCondition` | «ПРЕДПРОСМОТР НЕИСПРАВНОСТИ», moved here with `#btnPreviewFacilityConditionReason`. Hidden for a new fault. Blocked by the draft-link rule and by the 040 play reasons. Opens `#facilityPreviewDialog` over the fault dialog |
| 11 | `details#facilityConditionTechnical` | `summary` «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ», holding `label[for="facilityConditionID"]` «ИДЕНТИФИКАТОР» and `#facilityConditionID` (`readonly` for a saved fault) |
| 12 | `#facilityConditionStatus`, `#facilityConditionError` | unchanged roles |
| 13 | actions | `#btnCancelFacilityCondition` «ОТМЕНА», `#btnSaveFacilityCondition` «ГОТОВО» |

`#facilityConditionPrivateRecovery` («ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ») is removed. The dialog opens with
focus on `#facilityConditionName`. Choosing a fault anywhere opens this dialog (FR-019): a row of
the «НЕИСПРАВНОСТИ» tab, «ИЗМЕНИТЬ» in the terminal-fault group, a transition panel's «К
НЕИСПРАВНОСТИ», and a dependency's «К НЕИСПРАВНОСТИ».

Both sections show the draft over the newest session. Rows come from every device's
`transitions[].conditionEffects` for this fault, plus the adds and removes made in this dialog
(R2). Each transition row's name is «<device name> — <transition name>».

### «КАК ПОЯВЛЯЕТСЯ» (FR-014)

| Element | Contract |
|---|---|
| `#facilityFaultAppearsHeading` | «КАК ПОЯВЛЯЕТСЯ» |
| `#facilityFaultAppearsInitial` | «Активна с начала игры.», shown while the draft start position is «НЕИСПРАВНОСТЬ АКТИВНА» |
| `ul#facilityFaultAppearsList` | `role="list"`. One `li.facility-fault-way[data-kind="transition"][data-device-id][data-transition-id]` per transition with an `active: true` effect on this fault. Its `span.facility-fault-way-name` holds «<device> — <transition>», and `button.facility-fault-way-remove` «УБРАТЬ» (`aria-label="Убрать: <device> — <transition>"`) removes the effect |
| `#facilityFaultAppearsWarning` | Shown while the list is empty. When the start position is working, it reads «Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.» When the start position is active, it reads «Ни один переход не включает эту неисправность снова. После устранения её можно включить только вручную.» |
| `#btnAddFaultActivation` | «+ ВКЛЮЧАТЬ ПРИ ПЕРЕХОДЕ…». Shows `#facilityFaultActivationPicker` and focuses it |
| `select#facilityFaultActivationPicker` | Hidden until the add button is pressed. The placeholder is «ВЫБЕРИТЕ ПЕРЕХОД». Each option is «<device> — <transition>» and carries `data-device-id` and `data-transition-id`; the option values are opaque. It lists every transition of every device, except those already in either list for this fault. Choosing one adds the row and hides the picker. Escape hides it |

### «КАК УСТРАНЯЕТСЯ» (FR-015, FR-016, FR-020)

| Element | Contract |
|---|---|
| `#facilityFaultClearsHeading` | «КАК УСТРАНЯЕТСЯ» |
| `ul#facilityFaultClearsList` | `role="list"`, rows in this order: transitions, programs, the Overseer |
| transition row | `li.facility-fault-way[data-kind="transition"][data-device-id][data-transition-id]` for each `active: false` effect on this fault, with `span.facility-fault-way-name` and `button.facility-fault-way-remove` «УБРАТЬ» |
| program row | `li.facility-fault-way[data-kind="program"][data-program-id]` for each recovery program that contains at least one transition listed above (draft-aware). `span.facility-fault-way-name` holds the program name, followed by `span.facility-fault-way-kind` «программа восстановления» and `button.facility-fault-way-open` «К ПРОГРАММЕ», which opens the program dialog stacked. It has no remove button, because the row follows from the transitions |
| Overseer row | `li.facility-fault-way[data-kind="overseer"]`, always last and always present. `span.facility-fault-way-name` «Смотритель вручную» and `span.facility-fault-way-hint`: «во время игры — переключатель на карточке устройства» for a device fault, «во время игры — переключатель в списке «Неисправности терминалов»» for a terminal fault. No remove button |
| `#btnAddFaultClearing` | «+ УСТРАНЯТЬ ПРИ ПЕРЕХОДЕ…». Shows `#facilityFaultClearingPicker`, which follows the activation picker's rules |
| `#btnAddFaultRecoveryProgram` | «+ ПРОГРАММА ВОССТАНОВЛЕНИЯ…». Opens the program dialog stacked (R4) |

With the demo session, «Споровое загрязнение воздушного контура» lists «Контур вентиляции теплицы
— Смоделировать загрязнение» under «КАК ПОЯВЛЯЕТСЯ». Under «КАК УСТРАНЯЕТСЯ» it lists «Контур
вентиляции теплицы — Продуть воздушный контур», «ВОЗДУХ-76», and «Смотритель вручную» (SC-004).
«Контроллер гермодвери не отвечает» shows the working-start warning.

**Draft limits (R3).** Neither picker offers a transition that is already in either list for this
fault. One transition therefore never both activates and clears the fault, and an effect never
repeats.

### Program from a fault (R4)

1. `#btnAddFaultRecoveryProgram` opens `#facilityRecoveryProgramDialog` for a new program, stacked
   over the fault dialog.
2. Its list starts with one row: the first transition of the fault draft's «КАК УСТРАНЯЕТСЯ» list.
   It starts empty when there is none.
3. `#facilityRecoveryProgramFault` reads «Для неисправности «<fault name>»».
4. «ГОТОВО» saves the program as its own edit. «ОТМЕНА» saves nothing.
5. Either way, the program dialog closes, focus returns to `#btnAddFaultRecoveryProgram`, the fault
   draft is untouched, and both sections re-derive against the newest session. A saved program that
   contains a clearing transition appears as a program row.

### Save (R2, R3, FR-020)

«ГОТОВО» runs one `commitFacilityEdit('Неисправность «<name>»', mutate)`. The `mutate` finds every
target by ID, so it replays on a newer session. It:

- writes the condition's fields;
- writes `conditionEffects` on every transition the dialog touched: it adds `{conditionId, active}`
  for an added row and removes the effect for a removed row;
- drops each stored `recovery[]` entry that the edit invalidates (see `private-desktop.md`,
  «Session JSON version 1»);
- never writes or removes an entry with `privateOverseerAction`, never adds to `recovery[]`, and
  never writes `transition.recovery`.

A fault with no clearing transition and no program saves (FR-020).

## «НЕИСПРАВНОСТИ» tab

- `#facilityConditionSelectionEditor` and everything inside it are removed (R1).
- Each `#facilityConditionList` row is `button[data-facility-kind="condition"][data-facility-id]`
  with `aria-haspopup="dialog"`. Its text is the name, then «СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: ИСПРАВНО ·
  <device or terminal name>» from `facilityConditionActivity`. One press opens
  `#facilityConditionDialog`.
- Rows no longer select a fault. `selectedFacilityConditionID` and `data-facility-selected` on
  fault rows are removed, and only device selection hides `#facilitySelectionEmpty`.
- `#btnAddFacilityCondition` reads «ДОБАВИТЬ НЕИСПРАВНОСТЬ».

## Recovery program dialog (FR-016, FR-018)

`#facilityRecoveryProgramForm` renders its children in this order:

| # | Element | Contract |
|---|---|---|
| 1 | `#facilityRecoveryProgramDialogTitle` | «ПРОГРАММА ВОССТАНОВЛЕНИЯ» |
| 2 | `#facilityRecoveryProgramDialogDescription` | One sentence: «Переходы выполняются вместе, когда команда запускает программу.» |
| 3 | `#facilityRecoveryProgramFault` | Shown only when the dialog opened from a fault (R4) |
| 4 | `#facilityRecoveryProgramName` | label «НАЗВАНИЕ» |
| 5 | `fieldset#facilityProgramTransitionsField` | Legend «ПЕРЕХОДЫ». `#facilityProgramTransitions` (`role="list"`) holds one `.facility-action-request-row` per transition: `select.facility-action-device`, `select.facility-action-transition`, and a remove button «УДАЛИТЬ» (`aria-label="Удалить переход: <device> — <transition>"`). Every row looks the same, including the first |
| 6 | `#btnAddFacilityProgramTransition` | «+ ПЕРЕХОД», the only add control |
| 7 | `section#facilityProgramCommands` | Heading «ЗАПУСКАЮТ КОМАНДЫ». One `li` per command whose action runs this program, as «<terminal> › … › <command>». With none, `#facilityProgramCommandsWarning` reads «Ни одна команда не запускает эту программу. Выберите её в действии команды, чтобы игроки могли её запустить.» |
| 8 | `details#facilityRecoveryProgramTechnical` | `summary` «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ», holding `label[for="facilityRecoveryProgramID"]` «ИДЕНТИФИКАТОР» and `#facilityRecoveryProgramID` (`readonly` when saved) |
| 9 | status, error, actions | `#facilityRecoveryProgramStatus`, `#facilityRecoveryProgramError`, `#btnCancelFacilityRecoveryProgram` «ОТМЕНА», `#btnSaveFacilityRecoveryProgram` «ГОТОВО» |

No multi-sentence instruction remains. `#facilityRecoveryProgramDevice`,
`#facilityRecoveryProgramTransition`, `#facilityProgramExtraTransitions`, and «ДОБАВИТЬ ПЕРЕХОД
ПРОГРАММЫ» are removed. A new program opened from the «ПРОГРАММЫ» tab starts with one empty row.

## Display rule dialog (FR-022 to FR-028, R5, R6)

`#facilityBindingDialog`, `#facilityBindingForm`, `#facilityBindingNode`, `#facilityBindingDevice`,
`#facilityBindingState`, `#facilityBindingBlock`, `#facilityBindingText`, `#facilityBindingStatus`,
`#facilityBindingError`, `#btnCancelFacilityBinding`, and `#btnSaveFacilityBinding` keep their ids.
The title `#facilityBindingDialogTitle` reads «ПРАВИЛО ОТОБРАЖЕНИЯ».
`#facilityBindingDialogDescription` keeps one fixed sentence, «Когда устройство в состоянии, где и
что показать игрокам.» The per-kind instructions are removed.

### Tabs

`#facilityRuleTabs` has `role="tablist"` and `aria-label="Тип правила"`. It sits directly under the
title and uses the 040 tab pattern: automatic activation, ArrowLeft and ArrowRight with wrap, Home
and End, and a roving `tabindex`.

| Tab | Label | `data-kind` | Stored as |
|---|---|---|---|
| `#facilityRuleTabEntryContent` | «ТЕКСТ ЗАПИСИ» | `entry-content` | `block.facilityTextVariants[]` |
| `#facilityRuleTabMenuName` | «НАЗВАНИЕ В МЕНЮ» | `menu-name` | `node.facilityNameVariants[]` |
| `#facilityRuleTabVisibility` | «ПОКАЗАТЬ ЭЛЕМЕНТ» | `visibility` | `node.visibleWhen` |
| `#facilityRuleTabAvailability` | «РАЗРЕШИТЬ КОМАНДУ» | `availability` | `node.availableWhen` |

- No «ДЕЙСТВИЕ КОМАНДЫ» tab exists. `#facilityRuleSentence` (`role="tabpanel"`) is labelled by the
  active tab.
- **Default tab.** A new rule opens on «ТЕКСТ ЗАПИСИ» when its target element is a record with
  blocks, and on «НАЗВАНИЕ В МЕНЮ» otherwise.
- **New rule (FR-026).** Switching tabs keeps the device, the state, and the text. It keeps the
  element when the new tab offers it, and clears it otherwise. It keeps the block while the
  element stays.
- **Existing rule.** Its tab is fixed. The other tabs have `aria-disabled="true"` and
  `aria-describedby="facilityRuleTabReason"`, whose reason reads «ЧТОБЫ СМЕНИТЬ ТИП, СОЗДАЙТЕ НОВОЕ
  ПРАВИЛО». Saving replaces the variant in place, so its position stays (R5).

### Sentence (FR-023, FR-027)

`#facilityRuleSentence` holds `div.facility-rule-clause[data-clause]` items in this order. Only the
clauses of the active tab are rendered visible, and a hidden clause hides its label too, so
`expectNoOrphanLabels` passes on every tab (SC-005).

| `data-clause` | Label | Controls | Tabs |
|---|---|---|---|
| `when` | `legend` «КОГДА» | `#facilityBindingDevice` (`aria-label="Устройство"`), `span.facility-rule-equals` «=», `#facilityBindingState` (`aria-label="Состояние"`) | all |
| `where` | `label[for="facilityBindingNode"]` «ГДЕ» | `#facilityBindingNode` | all |
| `block` | `label[for="facilityBindingBlock"]` «БЛОК» | `#facilityBindingBlock` | «ТЕКСТ ЗАПИСИ» |
| `text` | `label[for="facilityBindingText"]` «ТЕКСТ» | `#facilityBindingText` | «ТЕКСТ ЗАПИСИ», «НАЗВАНИЕ В МЕНЮ» |

`#facilityRuleOtherStates` follows the sentence. It names what players see in the device's other
states:

| Tab | Text |
|---|---|
| «ТЕКСТ ЗАПИСИ» | «В остальных состояниях останется исходный текст блока.» |
| «НАЗВАНИЕ В МЕНЮ» | «В остальных состояниях останется исходное название.» |
| «ПОКАЗАТЬ ЭЛЕМЕНТ» | «В остальных состояниях элемент скрыт.» |
| «РАЗРЕШИТЬ КОМАНДУ» | «В остальных состояниях команда видна, но её нельзя запросить.» |

A new rule opened while a device is selected preselects that device.

### Element and block choices (FR-024, FR-025, R5)

- **«ГДЕ».** `#facilityBindingNode` lists the elements of every terminal. Each option reads
  «<terminal> › <folder> › … › <element>», with the root folder left out, for example «Терминал
  смотрителя — Убежище 76 › Директивы Vault-Tec › Сводка эвакуации». Each option carries
  `data-terminal-id` and `data-node-id`; its `value` is an opaque index into a catalog built when
  the dialog opens. The options per tab are:
  - «ТЕКСТ ЗАПИСИ»: records only;
  - «РАЗРЕШИТЬ КОМАНДУ»: commands only;
  - «НАЗВАНИЕ В МЕНЮ» and «ПОКАЗАТЬ ЭЛЕМЕНТ»: any element except a terminal root.

  The separate «ТЕРМИНАЛ» select is removed.
- **«БЛОК».** `#facilityBindingBlock` lists the chosen record's blocks as «<n> · «<opening text>»».
  n starts at 1, and the opening text comes from `normalizedEntryBlockPreview` (48 code points, then
  «…»; an empty block reads «ПУСТО»). Example: «3 · «ГЕРМОДВЕРЬ: ЗАПЕРТА»».

### Conflicts and missing elements (FR-028, R6)

On «ГОТОВО», before any save, the dialog checks `facilityBindingCatalog()` of the current session.
On a hit, `#facilityRuleConflict` (`role="alert"`) names the existing rule, and
`#btnOpenConflictingFacilityRule` «ОТКРЫТЬ ПРАВИЛО» reopens the dialog on that rule. Nothing is
saved.

| Case | Message |
|---|---|
| The element's name or block text rules depend on another device | «Название этого элемента уже зависит от устройства «<device>». Все правила одного элемента должны зависеть от одного устройства.» For a block: «Текст этого блока уже зависит от устройства «<device>». Все правила одного блока должны зависеть от одного устройства.» |
| A rule for the same element, device, and state exists | «Для «<device> = <state>» у этого элемента уже есть правило.» |
| The element already has `visibleWhen` («ПОКАЗАТЬ ЭЛЕМЕНТ») or `availableWhen` («РАЗРЕШИТЬ КОМАНДУ») | «У элемента уже есть правило «<tab label in sentence case>»: <device> = <state>. Измените его вместо нового.» |

When the chosen element no longer exists, `#facilityBindingError` reads «Элемент «<name>» удалён.
Выберите другой элемент в «ГДЕ».», focus moves to `#facilityBindingNode`, and nothing is saved.

## Command action dialog (FR-029, R7)

`dialog#facilityCommandActionDialog` is new. It has `aria-modal="true"`, is labelled by
`#facilityCommandActionDialogTitle` «ДЕЙСТВИЕ КОМАНДЫ», and is described by
`#facilityCommandActionDialogDescription` and `#facilityCommandActionError`.
`#facilityCommandActionForm` renders its children in this order:

| Element | Contract |
|---|---|
| `#facilityCommandActionDialogDescription` | One sentence: «Что запускает команда после подтверждения Смотрителем.» |
| `label[for="facilityCommandActionCommand"]` «КОМАНДА» + `#facilityCommandActionCommand` | Commands only, labelled «<terminal> › … › <command>», with the same option attributes as «ГДЕ». Shown only when the command is picked |
| `#facilityCommandActionCommandFixed` | «<terminal> › … › <command>», shown instead of the select when the command is fixed |
| `fieldset#facilityCommandActionMode` | Legend «ЧТО ЗАПУСТИТЬ». Radios `#facilityCommandActionModeTransitions` (`value="transitions"`, «ПЕРЕХОДЫ») and `#facilityCommandActionModeProgram` (`value="program"`, «ПРОГРАММА ВОССТАНОВЛЕНИЯ») |
| `#facilityCommandActionTransitions` | `role="list"`. `.facility-action-request-row` rows as in the program dialog. Shown in «ПЕРЕХОДЫ» mode, starting with one row (preset to the selected device) when empty |
| `#btnAddFacilityCommandActionTransition` | «+ ПЕРЕХОД», in «ПЕРЕХОДЫ» mode |
| `label[for="facilityCommandActionProgram"]` «ПРОГРАММА ВОССТАНОВЛЕНИЯ» + `#facilityCommandActionProgram` | In «ПРОГРАММА ВОССТАНОВЛЕНИЯ» mode. Lists every program, with the placeholder «ВЫБЕРИТЕ ПРОГРАММУ» |
| `#facilityCommandActionCompletedName` | label «НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ», placeholder «Например: Выход открыт» |
| `#facilityCommandActionConfirmationText` | label «ЗАПРОС ПОДТВЕРЖДЕНИЯ», placeholder «Например: Открыть гермодверь и включить сигнал?» |
| `#facilityCommandActionStatus`, `#facilityCommandActionError` | `role="status"` and `role="alert"` |
| actions | `#btnCancelFacilityCommandAction` «ОТМЕНА», `#btnSaveFacilityCommandAction` «ГОТОВО» |

- **Player texts.** The two text fields and their labels are shown and required only when the
  chosen command has no `stateChange` yet.
- **Save.** «ГОТОВО» writes `stateChange.facilityAction`: `transitions.transitions[]` in
  «ПЕРЕХОДЫ» mode, or `recoveryProgramId` in program mode. It also writes `completedName` and
  `confirmationText` when they were missing. It saves through
  `commitFacilityEdit('Действие команды «<command>»', …)`.
- **Existing actions.** Every existing action opens here and saves to the same shape. That
  includes a program-backed action, which opens in program mode with its program selected
  (SC-009).

**Openers.** Every opener has `aria-haspopup="dialog"` and `aria-controls="facilityCommandActionDialog"`:

| Opener | Command |
|---|---|
| `#btnEditCommandFacilityAction` in the command form, «ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ» or «ДОБАВИТЬ ДЕЙСТВИЕ КОМАНДЫ». It is enabled for every command in the state-change mode, with or without `stateChange` | fixed |
| «Изменить» on a `#facilityActionBindingList` row | fixed |
| `#btnAddFacilityActionBinding` «ДОБАВИТЬ ДЕЙСТВИЕ КОМАНДЫ» | picked; the first command without an action is preselected |
| guide step «ДЕЙСТВИЕ»: «ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ» | fixed |
| guide step «ДЕЙСТВИЕ»: «СВЯЗАТЬ С КОМАНДОЙ», «СВЯЗАТЬ С ДРУГОЙ КОМАНДОЙ» | picked; the guide's target command is preselected |

In the command form, `#commandFacilityActionHeading` reads «ДЕЙСТВИЕ КОМАНДЫ». The hint «ПРИМЕНИТЕ
РЕЖИМ, ЧТОБЫ ДОБАВИТЬ ДЕЙСТВИЕ УСТРОЙСТВ» is removed. The removal notice reads «ДЕЙСТВИЕ КОМАНДЫ
БУДЕТ УДАЛЕНО, КОГДА ВЫ ПРИМЕНИТЕ ФОРМУ.»

## Dependency dialog (FR-037, R13)

| Element | Contract |
|---|---|
| `#facilityDependencyDialogTitle` | «ЗАВИСИМОСТИ УСТРОЙСТВА» |
| `#facilityDependencyDialogDescription` | «Что в локации зависит от устройства «<device name>».» |
| `#facilityDependencyList` | `role="list"`, `aria-label="Зависимости устройства"` |
| `.facility-dependency-row` | `role="listitem"`, `data-kind` set to the dependency kind, and `data-destination` set to `device`, `condition`, `program`, `command`, `rule`, or `none` |
| `.facility-dependency-sentence` | what depends on what, by name (table below) |
| `.facility-dependency-place` | where it is, by name |
| `button.facility-dependency-navigation` | the destination name, with `aria-label` «<destination>: <source name>». It is absent when `data-destination="none"` |
| `details.facility-technical-details` | Collapsed. `summary` «подробности для разработчика»; the content reads «<kind> · <sourceId> → <targetId> · <property>», with « · терминал <terminalId>» when present |
| `#facilityDependencyStatus` | unchanged: «НАЙДЕНО ССЫЛОК: n» or «ССЫЛКИ НЕ НАЙДЕНЫ» |

The `.facility-dependency-kind` and `.facility-dependency-property` lines and the «ОТКРЫТЬ» button
are removed. No technical path is visible until the disclosure opens.

`describeFacilityDependency(dependency, session)` returns `{ sentence, place, destination }`.
Notation in the table: D is the inspected device, S is the state read from the indexed equality in
`property` with the regex `\[(\d+)\]`, D2/T is a transition found by matching
`${device.id}/${transition.id}` against `sourceId` over all devices (never by splitting), F is a
fault, P is a program, C is a command, N is an element, E is a record, and n is a block number. An
unresolved name falls back to its raw ID, which shows only in the sentence and the disclosure.

| `kind` | Sentence | Place | Button → target |
|---|---|---|---|
| `transition-precondition` | «Переход «T» устройства «D2» требует: D = S.» | «D2 › T» | «К УСТРОЙСТВУ» → D2's device dialog with T open in the panel, focus on its name |
| `transition-condition-effect` | «Переход «T» устройства «D2» включает или снимает неисправность «F».» | «D2 › T» | «К УСТРОЙСТВУ» → as above |
| `recovery-reference` | «Неисправность «F» устраняется переходом «T».» | «F» | «К НЕИСПРАВНОСТИ» → F's dialog |
| `diagnostic-scope` | «Неисправность «F» действует на это устройство.» | «F» | «К НЕИСПРАВНОСТИ» → F's dialog |
| `diagnostic-effect` | «Неисправность «F» меняет работу терминала.» | «F» | «К НЕИСПРАВНОСТИ» → F's dialog |
| `recovery-program-transition` | «Программа восстановления «P» запускает переход «T».» | «P» | «К ПРОГРАММЕ» → P's dialog |
| `command-action` | «Команда «C» запускает переход «T».» | «<terminal> › … › C» | «К КОМАНДЕ» → the «ТЕРМИНАЛЫ» tab with C selected and focus on `#btnEditCommandFacilityAction` |
| `name-variant` | «Название «N» в меню меняется, когда D = S.» | «<terminal> › … › N» | «К ПРАВИЛУ ОТОБРАЖЕНИЯ» → the rule dialog on that catalog entry |
| `entry-content-variant` | «Текст записи «E» (блок n) меняется, когда D = S.» | «<terminal> › … › E › блок n» | «К ПРАВИЛУ ОТОБРАЖЕНИЯ» → as above. The block ID resolves through `entryBlockLocation` |
| `visibility` | «Элемент «N» виден, только когда D = S.» | «<terminal> › … › N» | «К ПРАВИЛУ ОТОБРАЖЕНИЯ» |
| `availability` | «Команду «C» можно запросить, только когда D = S.» | «<terminal> › … › C» | «К ПРАВИЛУ ОТОБРАЖЕНИЯ» |
| `unspecified` or unknown | «Другая ссылка на устройство.» | — | none |

A destination that no longer exists shows the 039 dependency error, reworded to «Связанный элемент
больше не существует. Закройте окно и откройте зависимости снова.» The Go report and its proto
stay unchanged.

**Technical details helper.** `facilityTechnicalDetails(parent, text, summary = 'подробности для
разработчика')` gains the `summary` parameter. Every developer disclosure uses the default. That
includes `#facilityDeviceHeaderTechnical`, whose old summary «Технические идентификаторы» goes away.

## Scenario guide

- **Step label.** The first step is «УСТРОЙСТВО»: the nav button reads «1. Устройство», and
  `#facilityStoryGuideStep` reads «КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 1 ИЗ 5: УСТРОЙСТВО». This replaces 040's
  «ОБЪЕКТ».
- **Device dialog controls.** Every guide control that opens an existing device's dialog reads
  «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ». This replaces «НАСТРОИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ», «ДОБАВИТЬ
  ПЕРЕХОД В УСТРОЙСТВО», «ИЗМЕНИТЬ ПЕРЕХОДЫ И ПРЕДУСЛОВИЯ», and «ДОБАВИТЬ ПЕРЕХОД ДЛЯ ПРОГРАММЫ». A
  step shows at most one such control.
- **Step 4.** «НАСТРОИТЬ АКТИВАЦИЮ И СНЯТИЕ» opens `#facilityConditionDialog` for the selected
  device's first fault. It is left out when the device has no fault (FR-017).
- **Steps 2 and 3.** Step 2 controls open `#facilityCommandActionDialog`. Step 3 controls open
  `#facilityBindingDialog`.
- **Step 4 example.** The example drops «Восстановление оператором». It reads: «Пример: «Привод двери
  повреждён» мешает выполнить команду. В окне неисправности укажите, какой переход её включает и
  какой устраняет. Смотритель может снять её вручную во время игры, а программа объединяет
  несколько переходов.»

## Errors (FR-039 to FR-041, R12)

### Helper

```js
// overseerErrorMessage(result, area) → { message, key, original }
// renderOverseerError(sink, result, area, { prefix = '' } = {}) → void
```

- `result` is a desktop result (`{ error }`, or a facility result `{ failure, issues }`), a plain
  string, or `null`. `area` is one of the areas below. An unknown area is a programming error, and
  the helper throws.
- `overseerErrorMessage` picks the Russian message:
  1. a facility `failure` code goes through `facilityFailureMessages`;
  2. an `error` text goes through every `byText` table of `OVERSEER_ERROR_MESSAGES`, where keys are
     unique across areas;
  3. otherwise the area's `general` message is used.

  `key` is the matched code or text, or `general`. `original` is the raw text, or for a facility
  result «failure: <code>» with « · <entityKind> <entityId>» and « · <referenceKind>
  <referenceId>» when present. It is empty when there is nothing raw.
- `renderOverseerError` replaces the sink's content with the following, then shows the sink and
  sets `data-error-area` and `data-error-key`:
  - `span.overseer-error-prefix` holding `prefix`, only when a prefix is given;
  - `p.overseer-error-message` holding the message;
  - when `original` is not empty, `details.overseer-error-details` (collapsed) with `summary`
    «подробности для разработчика» and `p.overseer-error-original` holding `original`.
- `renderOverseerError(sink, null, area)` empties and hides the sink.
- The original text also goes to the log, as today.

**Message rules (SC-008).**

- Every catalog value is in Russian, has at least two sentences, and its last sentence says what
  to do next.
- Catalog values contain no Latin letters. Authored names are inserted only inside «…».
- A browser test strips «…» segments from each visible `.overseer-error-message` and asserts that
  no `[A-Za-z]` remains. Latin text appears only inside `.overseer-error-details`.

**Facility entity phrase (FR-038).** When a facility result's first issue names an entity found in
the session, the message gains « Затронуто: <phrase>.», where the phrase is one of:

- «устройство «<name>»»;
- «состояние «<name>» устройства «<device>»»;
- «переход «<name>» устройства «<device>»»;
- «неисправность «<name>»»;
- «программа восстановления «<name>»».

Other entity kinds (`request`, `session`, `facility`, `desktop-result`) add nothing. Raw kinds and
IDs go only into `original`. `facilityIssueMessage(result)` returns
`overseerErrorMessage(result, 'facility').message`.

### Areas and sinks

| Area | Catalog | Sinks |
|---|---|---|
| `player-config` | specific, `byText` | `#playerConfigError`; `#coordinationError` and `#logicalSessionDialogError` mirror it |
| `session` | specific, `byText` | `#startStatus` (open, new, copy demo); player-config association failures in `#playerConfigError` |
| `facility` | specific, `facilityFailureMessages` | `#facilityValidation`, `#facilitySelectionError`, `#facilityDeviceError`, `#facilityConditionError`, `#facilityRecoveryProgramError`, `#facilityBindingError`, `#facilityCommandActionError`, `#facilityDependencyError`, `#facilityPreviewError`, `#facilityForceStateError`, `#facilityRepairError`, `#sceneResetStatus` (prefix «СЦЕНА НЕ СБРОШЕНА:»), `#facilityPlayStatus` and `#facilityTerminalFaultStatus` on failure, and `#coordinationError` for command state |
| `public-access` | general | `#publicAccessError`, `#publicAccessSettingsError` |
| `credentials` | general | `#publicAccessProviderTokenError`, `#publicAccessPlayerCredentialsError` (token, credentials, password, copy) |
| `update` | general | `#applicationUpdateError` for offer and restart decisions |
| `players` | general | `#playerManagementError`, `#coordinationError` for character changes |
| `terminal-groups` | general | `#terminalGroupError`, `#terminalGroupImpactError` |
| `broadcast` | general | `#coordinationError` (start broadcast), `#takeOffAirError`, `#terminalSwitchError` |
| `commands` | general | `#coordinationError` for command, navigation, and session-state decisions (prefix «ОШИБКА ИЗМЕНЕНИЯ СОСТОЯНИЯ:» where 039 had it) |
| `logs` | general | `[data-log-access-status]`. The log directory path stays after the message, because it is the next step |
| `links` | general | the server URL link's `title`, as message text only, because a `title` cannot hold a disclosure. The raw text goes to the log |

`#saveStatus` keeps its 039 form «ОШИБКА СОХРАНЕНИЯ: <message>», where the message comes from
`overseerErrorMessage(result, 'facility')`. It has no disclosure there. The dialog that reopens on
the failure shows the disclosure in its error sink. Client-side validation text that never came
from Go (`#createTerminalError`, `#entryBlockCommandError`, dialog field checks) is out of scope.

A failure that arrives while a dialog is open renders in that dialog's sink. The author's input
stays (edge case).

### Catalog shape

The catalog sits next to `facilityFailureMessages` in `overseer.js`. Go sync tests parse it as text
(`private-desktop.md`), so its layout is part of the contract:

```js
const OVERSEER_ERROR_MESSAGES = Object.freeze({
  'player-config': Object.freeze({
    general: 'Не удалось выполнить действие с файлом игроков. Выберите файл заново или создайте новый.',
    byText: Object.freeze({
      'could not read the player config': 'Не удалось прочитать файл игроков. Выберите его заново или создайте новый.',
      // one entry per line
    }),
  }),
  'session': Object.freeze({
    general: 'Не удалось выполнить действие с файлом сессии. Повторите попытку. Если ошибка повторится, сохраните журнал.',
    byText: Object.freeze({
      'could not read the selected session': 'Не удалось прочитать файл сессии. Проверьте, что файл доступен, и откройте его снова.',
    }),
  }),
  'facility': Object.freeze({
    general: 'Не удалось изменить локацию. Повторите попытку. Если ошибка повторится, сохраните журнал.',
  }),
  // one block per general-only area, in the same form
});
```

- Area keys are always single-quoted.
- Each `byText` entry sits on one line in the form `'<exact Go text>': '<message>',`.
- An area block starts at the line `  '<area>': Object.freeze({` and ends at the next line `  }),`.
- Messages use «» and never contain a single quote.

### `facility` messages (`facilityFailureMessages`, reworded)

| Code | Message |
|---|---|
| `missing-reference` | «Ссылка ведёт на удалённый элемент локации. Выберите существующий элемент и повторите.» |
| `invalid-transition` | «Переход недоступен из текущего состояния устройства. Выберите другой переход или переведите устройство вручную.» |
| `precondition-failed` | «Предусловие перехода не выполнено. Сначала переведите нужное устройство в требуемое состояние.» |
| `stale-revision` | «Локация изменилась, пока выполнялся запрос. Проверьте новое состояние и повторите.» |
| `conflict` | «Правка противоречит текущим данным локации. Проверьте связанные элементы и повторите.» |
| `duplicate` | «Этот запрос уже выполнен. Повторять его не нужно.» |
| `invalid-configuration` | «Настройка локации не прошла проверку. Исправьте последнюю правку или отмените её кнопкой «Отменить».» |
| `persistence-failed` | «Не удалось записать файл сессии. Проверьте, что диск доступен и файл не защищён от записи, затем повторите.» |
| `runtime-context-ended` | «Сессия закрылась до сохранения. Откройте её снова и повторите.» |
| `rejected` | «Смотритель отклонил запрос. Повторять его не нужно.» |
| no code, or an unknown code | `OVERSEER_ERROR_MESSAGES['facility'].general`: «Не удалось изменить локацию. Повторите попытку. Если ошибка повторится, сохраните журнал.» |

The 040 stale play text «СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС» is replaced by the
`stale-revision` message. The card still applies `result.session` first.

### `player-config` messages (`internal/playerconfig/service.go`)

| Go text (key) | Message |
|---|---|
| `new player config was canceled` | «Создание файла игроков прервано. Нажмите «Создать файл» ещё раз.» |
| `new player config dialog is unavailable` | «Окно выбора файла недоступно. Перезапустите приложение и создайте файл игроков снова.» |
| `could not choose a new player config destination` | «Не удалось выбрать место для файла игроков. Нажмите «Создать файл» и выберите другую папку.» |
| `could not verify the new player config contract` | «Не удалось проверить новый файл игроков. Создайте его снова; если ошибка повторится, сохраните журнал.» |
| `could not write the new player config` | «Не удалось записать файл игроков. Выберите папку, доступную для записи, и создайте файл снова.» |
| `open player config was canceled` | «Выбор файла игроков прерван. Нажмите «Выбрать файл» ещё раз.» |
| `open player config dialog is unavailable` | «Окно выбора файла недоступно. Перезапустите приложение и выберите файл игроков снова.» |
| `could not choose a player config` | «Не удалось выбрать файл игроков. Нажмите «Выбрать файл» и повторите выбор.» |
| `active session path is invalid` | «Путь к файлу сессии недействителен. Откройте сессию заново, затем выберите файл игроков.» |
| `player config reference must be a relative file path` | «Сессия ссылается на файл игроков по неверному пути. Выберите файл игроков заново.» |
| `could not read the player config` | «Не удалось прочитать файл игроков. Выберите его заново или создайте новый.» |
| `the selected file is not a valid version-1 player config` | «Выбранный файл не является файлом игроков версии 1. Выберите другой файл или создайте новый.» |
| `the selected file does not satisfy the player config contract` | «Выбранный файл игроков повреждён или заполнен неверно. Выберите другой файл или создайте новый.» |
| `player config storage is unavailable` | «Хранилище файлов игроков недоступно. Перезапустите приложение и повторите.» |
| `general` | «Не удалось выполнить действие с файлом игроков. Выберите файл заново или создайте новый.» |

### `session` messages (`internal/session/service.go`)

| Go text (key) | Message |
|---|---|
| `new session is unavailable` | «Создать сессию сейчас нельзя. Перезапустите приложение и повторите.» |
| `new session dialog is unavailable` | «Окно выбора файла недоступно. Перезапустите приложение и создайте сессию снова.» |
| `could not choose a new session destination` | «Не удалось выбрать место для новой сессии. Нажмите «Новая сессия» и выберите другую папку.» |
| `new session was canceled` | «Создание сессии прервано. Нажмите «Новая сессия» ещё раз.» |
| `could not verify the new session contract` | «Не удалось проверить новую сессию. Создайте её снова; если ошибка повторится, сохраните журнал.» |
| `could not prepare the new session` | «Не удалось подготовить новую сессию. Создайте её снова; если ошибка повторится, сохраните журнал.» |
| `session storage is unavailable` | «Хранилище сессий недоступно. Перезапустите приложение и повторите.» |
| `could not write the new session` | «Не удалось записать файл сессии. Выберите папку, доступную для записи, и создайте сессию снова.» |
| `open session is unavailable` | «Открыть сессию сейчас нельзя. Перезапустите приложение и повторите.» |
| `open session dialog is unavailable` | «Окно выбора файла недоступно. Перезапустите приложение и откройте сессию снова.» |
| `could not choose a session to open` | «Не удалось выбрать файл сессии. Нажмите «Открыть сессию» и повторите выбор.» |
| `open session was canceled` | «Открытие сессии прервано. Нажмите «Открыть сессию» ещё раз.» |
| `the bundled demo is read-only; copy it before editing` | «Встроенное демо открыто только для чтения. Создайте его копию и редактируйте её.» |
| `could not read the selected session` | «Не удалось прочитать файл сессии. Проверьте, что файл доступен, и откройте его снова.» |
| `the selected file is not a valid version-1 session` | «Выбранный файл не является сессией версии 1. Выберите другой файл.» |
| `the selected file does not satisfy the session contract` | «Файл сессии повреждён или заполнен неверно. Выберите другой файл или восстановите его из копии.» |
| `demo copy is unavailable` | «Скопировать демо сейчас нельзя. Перезапустите приложение и повторите.» |
| `demo copy dialog is unavailable` | «Окно выбора файла недоступно. Перезапустите приложение и скопируйте демо снова.» |
| `could not choose a demo destination` | «Не удалось выбрать место для копии демо. Повторите копирование и выберите другую папку.» |
| `demo copy was canceled` | «Копирование демо прервано. Повторите копирование.» |
| `could not read the bundled demo` | «Не удалось прочитать встроенное демо. Переустановите приложение.» |
| `the bundled demo is not a valid version-1 session` | «Встроенное демо повреждено. Переустановите приложение.» |
| `the bundled demo does not satisfy the session contract` | «Встроенное демо заполнено неверно. Переустановите приложение.» |
| `the bundled demo player config reference is invalid` | «Встроенное демо ссылается на файл игроков неверно. Переустановите приложение.» |
| `could not read the bundled demo player config` | «Не удалось прочитать файл игроков встроенного демо. Переустановите приложение.» |
| `the bundled demo player config is not valid version 1` | «Файл игроков встроенного демо повреждён. Переустановите приложение.» |
| `could not create a writable demo player config copy` | «Не удалось записать копию файла игроков демо. Выберите папку, доступную для записи, и повторите.» |
| `could not create a writable demo copy` | «Не удалось записать копию демо. Выберите папку, доступную для записи, и повторите.» |
| `player config association was canceled` | «Сохранение выбора файла игроков прервано. Выберите файл игроков ещё раз.» |
| `there is no active session` | «Сессия не открыта. Откройте или создайте сессию.» |
| `player config path must be absolute` | «Путь к файлу игроков недействителен. Выберите файл игроков заново.» |
| `could not create a relative player config reference` | «Не удалось сохранить путь к файлу игроков относительно сессии. Выберите файл игроков рядом с файлом сессии.» |
| `could not verify the player config association` | «Не удалось проверить выбор файла игроков. Выберите файл ещё раз; если ошибка повторится, сохраните журнал.» |
| `could not prepare the player config association` | «Не удалось подготовить сохранение выбора файла игроков. Выберите файл ещё раз; если ошибка повторится, сохраните журнал.» |
| `could not save the player config association` | «Не удалось записать выбор файла игроков в файл сессии. Проверьте, что файл сессии доступен для записи, и повторите.» |
| `active session changed while saving the player config association` | «Сессия изменилась во время сохранения выбора файла игроков. Выберите файл игроков ещё раз.» |
| `could not save the session` | «Не удалось сохранить файл сессии. Проверьте, что файл доступен для записи, и повторите.» |
| `session service is shut down` | «Приложение завершает работу. Перезапустите его и повторите.» |
| `general` | «Не удалось выполнить действие с файлом сессии. Повторите попытку. Если ошибка повторится, сохраните журнал.» |

### General messages

| Area | `general` |
|---|---|
| `public-access` | «Не удалось изменить публичный доступ. Повторите попытку. Если ошибка повторится, сохраните журнал.» |
| `credentials` | «Не удалось сохранить данные доступа. Проверьте введённые значения и повторите попытку. Если ошибка повторится, сохраните журнал.» |
| `update` | «Не удалось выполнить обновление приложения. Повторите попытку позже. Если ошибка повторится, сохраните журнал.» |
| `players` | «Не удалось изменить список игроков. Повторите попытку. Если ошибка повторится, сохраните журнал.» |
| `terminal-groups` | «Не удалось изменить группы терминалов. Повторите попытку. Если ошибка повторится, сохраните журнал.» |
| `broadcast` | «Не удалось изменить трансляцию. Повторите попытку. Если ошибка повторится, сохраните журнал.» |
| `startup` | «Сервер для игроков не запустился. Закройте приложение и запустите его снова. Если ошибка повторится, сохраните журнал.» Used by `#startStatus` for a fatal startup error, prefix «ЗАПУСК НЕ ЗАВЕРШЁН:», where the session buttons are disabled |
| `commands` | «Не удалось обработать запрос игрока. Повторите решение. Если ошибка повторится, сохраните журнал.» |
| `logs` | «Не удалось открыть журналы. Откройте папку журналов вручную.» |
| `links` | «Не удалось открыть ссылку. Скопируйте адрес и откройте его в браузере.» |

## Renamed copy (FR-034 to FR-036)

Every hit of the vocabulary check in today's sources, and every other facility string that names an
entity with a non-glossary word, changes as follows. Removed elements are listed in the next
section.

| Where | Before | After |
|---|---|---|
| `.facility-feedback` `aria-label` | «Состояние объектов» | «Состояние локации» |
| `#facilityDeviceList` `aria-label` | «Устройства объекта» | «Устройства локации» |
| `#facilityDetailPanel` `aria-label` | «Сведения об объекте» | «Сведения о локации» |
| guide nav button | «1. Объект» | «1. Устройство» |
| `#btnAddFacilityCondition` | «ДОБАВИТЬ УСЛОВИЕ» | «ДОБАВИТЬ НЕИСПРАВНОСТЬ» |
| `#btnAddFacilityBinding` | «ДОБАВИТЬ ПРИВЯЗКУ» | «ДОБАВИТЬ ПРАВИЛО» |
| `#btnAddFacilityActionBinding` | «ДОБАВИТЬ КОМАНДУ» | «ДОБАВИТЬ ДЕЙСТВИЕ КОМАНДЫ» |
| `#facilityDeviceDialogTitle` | «УСТРОЙСТВО ОБЪЕКТА» | «УСТРОЙСТВО» |
| `#facilityPreviewDialogTitle` | «ПРЕДПРОСМОТР ОБЪЕКТА» | «ПРЕДПРОСМОТР УСТРОЙСТВА» or «ПРЕДПРОСМОТР НЕИСПРАВНОСТИ», set per preview kind |
| preview actions `aria-label` | «Предпросмотр объекта» | «Предпросмотр» |
| `#facilityDeviceResetDialog` text | «…и связанные условия вернутся…» | «Текущее состояние устройства и его неисправности вернутся к начальным значениям.» |
| `#facilityConditionDialogTitle` | «ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ» | «НЕИСПРАВНОСТЬ» |
| fault actions `aria-label` | «Редактирование диагностического условия» | «Редактирование неисправности» |
| `#facilityConditionScope` label | «ОБЛАСТЬ УСЛОВИЯ» | «ГДЕ ДЕЙСТВУЕТ» |
| fault validation | «УКАЖИТЕ УНИКАЛЬНЫЙ ИДЕНТИФИКАТОР УСЛОВИЯ», «УКАЖИТЕ НАЗВАНИЕ УСЛОВИЯ», «ВЫБЕРИТЕ КАТЕГОРИЮ УСЛОВИЯ» | «…НЕИСПРАВНОСТИ» in each |
| `#facilityBindingDialogTitle` | «ПРИВЯЗКА ОБЪЕКТА» | «ПРАВИЛО ОТОБРАЖЕНИЯ» |
| rule actions `aria-label` | «Редактирование привязки объекта» | «Редактирование правила отображения» |
| `#facilityDependencyDialogTitle` | «ЗАВИСИМОСТИ ОБЪЕКТА» | «ЗАВИСИМОСТИ УСТРОЙСТВА» |
| `#facilityRepairDialog` actions `aria-label` | «Исправление и удаление объекта» | «Исправление ссылок и удаление устройства» |
| `#commandExecutionFacilityImpact` `aria-label` | «Влияние на состояние объекта» | «Влияние на локацию» |
| command impact rows | «ОЖИДАЕМАЯ РЕВИЗИЯ ОБЪЕКТА», «ДИАГНОСТИЧЕСКИЕ УСЛОВИЯ» | «ОЖИДАЕМАЯ РЕВИЗИЯ ЛОКАЦИИ», «НЕИСПРАВНОСТИ» |
| authoring history label | «— ОБЪЕКТ ПРАВКИ БОЛЬШЕ НЕ СУЩЕСТВУЕТ» | «— ИЗМЕНЁННЫЙ ЭЛЕМЕНТ БОЛЬШЕ НЕ СУЩЕСТВУЕТ» |
| `facilityIssueMessage` fallback | «КОНФИГУРАЦИЯ ОБЪЕКТОВ НЕ СОХРАНЕНА» | the `facility` `general` message |
| command form | «ДЕЙСТВИЕ УСТРОЙСТВ», «ИЗМЕНИТЬ / ДОБАВИТЬ ДЕЙСТВИЕ УСТРОЙСТВ» | «ДЕЙСТВИЕ КОМАНДЫ», «ИЗМЕНИТЬ / ДОБАВИТЬ ДЕЙСТВИЕ КОМАНДЫ» |
| source comments | «the binding dialog that ОБЪЕКТЫ uses» and similar | English wording without the banned forms |

## Removed elements

These are removed together with their handlers and tests (Principle VII):

| Removed | Replaced by |
|---|---|
| `#facilityDeviceInitialState` and its label and help | the star on `button.facility-state-chip` |
| `#facilityDeviceStates`, `#facilityStateRowTemplate`, per-row state name inputs | `#facilityDeviceStateChips`, `#facilityStateEditor`, the state rows in `#facilityDeviceTechnical` |
| `#facilityDeviceTransitions`, `#facilityTransitionRowTemplate`, `#btnAddFacilityTransition` | `#facilityTransitionMatrix`, `#facilityTransitionPanel`, `#btnAddFacilityCellTransition` |
| `.facility-transition-references`, `.facility-transition-summary`, `.facility-transition-condition-effects`, `.facility-condition-effect-row`, `.facility-effect-condition`, `.facility-effect-active`, «ДОБАВИТЬ ЭФФЕКТ УСЛОВИЯ» | read-only `.facility-transition-fault-effects`; editing in the fault dialog |
| `#facilityConditionPrivateRecovery` | «Смотритель вручную» row |
| `#facilityConditionSelectionEditor`, `#facilityConditionSelectionHeading`, `#facilitySelectedConditionSummary`, `#btnRecoverFacilityCondition`, `#btnRecoverFacilityConditionReason`, block reason «ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ НЕ НАСТРОЕНО» | fault rows that open the dialog; the fault switch |
| `#facilityRecoveryConfirmationDialog`, `#facilityRecoveryConfirmationDialogTitle`, `#btnCancelFacilityRecovery`, `#btnConfirmFacilityRecovery` | — |
| `#facilityRecoveryProgramDevice`, `#facilityRecoveryProgramTransition`, `#facilityProgramExtraTransitions`, «ДОБАВИТЬ ПЕРЕХОД ПРОГРАММЫ» | `#facilityProgramTransitions`, `#btnAddFacilityProgramTransition` |
| `#facilityBindingKind` and «ТИП ПРИВЯЗКИ» | `#facilityRuleTabs` |
| `#facilityBindingTerminal` | terminal inside the «ГДЕ» path |
| `#facilityBindingAction`, `#facilityBindingProgram`, `#facilityBindingActionRequests`, `#facilityBindingCompletedName`, `#facilityBindingConfirmationText`, «АТОМАРНЫЕ ПЕРЕХОДЫ КОМАНДЫ», kind `command-action` in the rule dialog | `#facilityCommandActionDialog` |
| `.facility-device-row-state`, `.facility-fault-count`, `title` on list rows | `.facility-state-badge`, `.facility-fault-mark` |
| `.facility-dependency-kind`, `.facility-dependency-property`, «ОТКРЫТЬ» in dependency rows | sentence, place, named button, disclosure |
| `desktopAPI.recoverFacilityCondition` | `desktopAPI.setFacilityConditionActive` (040) |
