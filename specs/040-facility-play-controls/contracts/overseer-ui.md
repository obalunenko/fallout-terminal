# Overseer UI Contract: Facility Play Controls

Browser tests may rely on the DOM ids, classes, and data attributes listed below. Operations are
defined in `private-desktop.md`. This contract extends and partly replaces the «ОБЪЕКТЫ» parts of
`specs/039-unified-facility-authoring/contracts/overseer-ui.md`. The branch stacks on
`039-unified-facility-authoring`.

## Copy

Buttons render the spec's UI copy in the Overseer's uppercase style. The verbatim spec copy maps to
rendered text as follows:

| Spec copy (verbatim) | Rendered | Where |
|---|---|---|
| «Принудительно в другое состояние…» | «ПРИНУДИТЕЛЬНО В ДРУГОЕ СОСТОЯНИЕ…» | `#btnForceFacilityDeviceState` |
| «Сбросить к начальному…» | «СБРОСИТЬ К НАЧАЛЬНОМУ…» | `#btnResetFacilityDevice` |
| «Удалить устройство» | «УДАЛИТЬ УСТРОЙСТВО» | `#btnDeleteFacilityDevice` (menu item) |
| «⋯» | «⋯» | `#btnFacilityDeviceMenu` |
| «Показать» | «ПОКАЗАТЬ» (open guide: «СКРЫТЬ») | `#facilityStoryGuideToggle` |
| «Что делают игроки» | «ЧТО ДЕЛАЮТ ИГРОКИ» | `#facilityTabActions` |
| «Что видят игроки» | «ЧТО ВИДЯТ ИГРОКИ» | `#facilityTabInformation` |
| «Неисправности» | «НЕИСПРАВНОСТИ» | `#facilityTabConditions` |
| «Как собрать сценарий» | «КАК СОБРАТЬ СЦЕНАРИЙ» | `#facilityStoryGuideStep` |

The ellipsis in the first two rows is the single character `…` (U+2026). The menu glyph is `⋯`
(U+22EF). Authored names (devices, states, transitions, faults, commands, terminals) are shown as
authored, without changing their case.

## Blocked controls

Play controls use the 039 pattern (FR-005 of 039, R14). A blocked control keeps focus and gets
`aria-disabled="true"`. Its `aria-describedby` points to a `.control-reason` element with the reason,
and activating it does nothing. Every play control shares these reasons:

- «НЕТ ОТКРЫТОЙ СЕССИИ»
- «ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА»: a play change, reset, or coordination command is in flight
- «ДОЖДИТЕСЬ СОХРАНЕНИЯ»: an authoring save is in flight, which would make the play change stale

The control-specific reasons appear below.

## «ОБЪЕКТЫ» detail area order (FR-017, FR-018, R11)

With a device selected, `#facilityDetailPanel` renders its children in this order:

1. `#facilityStoryGuide`: one line when collapsed.
2. `#facilitySelectionEditor`, which contains, in order:
   1. `#facilityDeviceHeader`;
   2. `#facilityPlayCard`;
   3. `#facilityDeviceActions`;
   4. `#facilitySelectionStatus` and `#facilitySelectionError`.
3. `#facilityDetailTabs` and its four tab panels.

With no device selected, `#facilitySelectionEditor` is hidden. `#facilitySelectionEmpty` («Выберите
устройство слева…») shows between the guide and the tabs, and the tabs list the whole session's
items.

**Layout guarantee (FR-018, SC-003).** The guide is collapsed and a device is selected. In a
1440×900 viewport and in a 1180×760 viewport, the bounding boxes of `#facilityDeviceHeader` and of
the first `.facility-play-transition-run` lie inside the viewport and inside the visible part of
`#facilityDetailPanel`, with `scrollTop = 0`. No two of these elements overlap. When the current
state has no outgoing transition, the check uses `#btnForceFacilityDeviceState` instead.

## Scenario guide (FR-019 to FR-021, R10)

| Element | Contract |
|---|---|
| `#facilityStoryGuide` (`<details>`) | No hard-coded `open` in `index.html`; `overseer.js` sets it |
| `#facilityStoryGuideSummary` (`<summary>`) | One line: `#facilityStoryGuideStep`, then `#facilityStoryGuideToggle` |
| `#facilityStoryGuideStep` | «КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ N ИЗ 5: <STEP>», where `<STEP>` is «ОБЪЕКТ», «ДЕЙСТВИЕ», «РЕЗУЛЬТАТ», «ОСЛОЖНЕНИЯ», or «ПРОВЕРКА» and N is the guide's current step |
| `#facilityStoryGuideToggle` | «ПОКАЗАТЬ» while collapsed, «СКРЫТЬ» while open; `aria-hidden="true"` (the summary is the control) |
| `.facility-story-steps` | Not sticky: computed `position` is `static`. At any viewport width from 900 px up, scrolling `#facilityDetailPanel` never makes its box intersect another visible control or text box |

**Open rule.** When the author has made an explicit choice in this run, the guide follows that
choice. Otherwise it is open exactly when the session has no devices.

- An explicit choice is a user toggle of `#facilityStoryGuideSummary`. Programmatic toggles by the
  rule are not choices.
- The choice lives in a module variable in `overseer.js`. Tab switches, device selection, and
  re-renders keep it.
- When the session's device count goes from 0 to 1, the choice is cleared, so the guide collapses
  on the first saved device.
- Nothing is stored in the session file, `localStorage`, or anywhere else. After a restart the rule
  applies again.

## Device header (FR-017)

`#facilityDeviceHeader` (`<header>`) replaces `#facilityDeviceSummary` and the «ВЫБРАННОЕ
УСТРОЙСТВО» heading `#facilitySelectionHeading`. `#facilitySelectionEditor` is labelled by
`#facilityDeviceHeaderName`.

| Element | Text | Data |
|---|---|---|
| `#facilityDeviceHeaderName` (`h2`) | device name | — |
| `#facilityDeviceHeaderKind` | kind label from `#facilityDeviceKind` options, for example «ДВЕРЬ» | `data-kind` |
| `#facilityDeviceHeaderTerminals` | «ТЕРМИНАЛЫ: n», counting terminals with a binding to the device | `data-count` |
| `#facilityDeviceHeaderCurrent` | «СЕЙЧАС: <state>» | `data-state-id` |
| `#facilityDeviceHeaderInitial` | «В НАЧАЛЕ: <state>» | `data-state-id` |
| `#facilityDeviceHeaderFaults` | «АКТИВНЫХ НЕИСПРАВНОСТЕЙ: n», counting device-scoped faults with `currentActive` | `data-count` |
| `#btnClearFacilitySelection` | «ВСЯ СЕССИЯ» (moved here from `.facility-detail-heading`) | — |

The technical details line (`id · kind · currentStateId`) moves into the header.

## Play card (`#facilityPlayCard`)

`#facilityPlayCard` is a `section` labelled by `#facilityPlayCardHeading` «ВО ВРЕМЯ ИГРЫ».
`#facilityPlayStatus` (`role="status"`, `aria-live="polite"`) reports play outcomes, with
`data-state` set to `ok`, `stale`, or `error`:

| Outcome | Text |
|---|---|
| Transition applied | «ПЕРЕХОД ВЫПОЛНЕН» |
| Forced change applied | «СОСТОЯНИЕ ИЗМЕНЕНО ПРИНУДИТЕЛЬНО» |
| Fault switched | «НЕИСПРАВНОСТЬ АКТИВИРОВАНА» / «НЕИСПРАВНОСТЬ СНЯТА» |
| Reset applied | «УСТРОЙСТВО СБРОШЕНО» |
| Stale (FR-004, R4) | «СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС» |
| Other failure | `facilityIssueMessage(result)` |

On a stale result, the Overseer first applies `result.session` (`applyLiveSession`) and re-renders.
Only then does it show the stale text, so the card already shows the state that won. It never
retries automatically.

### Transitions (FR-001, FR-002, R5)

`#facilityPlayTransitions` (`role="list"`) holds one `li.facility-play-transition` per authored
transition whose `sourceStateId` is the device's current state, in authored order. Recovery
transitions are included.

| Element | Contract |
|---|---|
| `li.facility-play-transition` | `data-transition-id`; `data-available="true"` or `"false"` |
| `button.facility-play-transition-run` | Text «<transition name> → <destination state name>», for example «Открыть гермодверь → Открыта». One press runs `desktopAPI.applyFacilityDeviceTransition`. There is no confirmation dialog (SC-001) |
| `ul.facility-play-preconditions` | One `li.facility-play-precondition` per precondition, with `data-device-id`, `data-state-id`, and `data-met="true"` or `"false"`. Text «<device name> = <state name> · ВЫПОЛНЕНО» or «<device name> = <state name> · НЕ ВЫПОЛНЕНО» |
| no preconditions | `ul.facility-play-preconditions` holds one item «БЕЗ ПРЕДУСЛОВИЙ» without `data-met` |
| unavailable | When any precondition is unmet, the run button is blocked with reason «НЕ ВЫПОЛНЕНО: <device name> = <state name>», naming the first unmet precondition |
| `#facilityPlayTransitionsEmpty` | «ИЗ ЭТОГО СОСТОЯНИЯ НЕТ ПЕРЕХОДОВ», shown instead of the list when no transition leaves the current state. Forced change, fault switches, and reset stay available |

A precondition is met when `devices[deviceId].currentStateId === stateId` in `state.session`. A
missing device counts as unmet. The card's verdict is for display only, and the server result
decides (SC-002).

### Forced change (FR-007, FR-008)

| Element | Contract |
|---|---|
| `#btnForceFacilityDeviceState` | «ПРИНУДИТЕЛЬНО В ДРУГОЕ СОСТОЯНИЕ…», `btn-secondary`, `aria-haspopup="dialog"`, `aria-controls="facilityForceStateDialog"`, `aria-describedby="btnForceFacilityDeviceStateReason"`. On a one-state device it is blocked with «У УСТРОЙСТВА ОДНО СОСТОЯНИЕ» |
| `#facilityForceStateDialog` | `aria-modal="true"`, labelled by `#facilityForceStateDialogTitle` «ПРИНУДИТЕЛЬНАЯ СМЕНА СОСТОЯНИЯ» |
| `#facilityForceStateTarget` (`select`) | Every state of the device except the current one, in authored order. The first is preselected |
| `#facilityForceStateSummary` | «<device name>: <current state> → <target state>», updated when the selection changes |
| `#facilityForceStateWarning` | «ПЕРЕХОДЫ И ПРЕДУСЛОВИЯ НЕ ПРОВЕРЯЮТСЯ. ЭФФЕКТЫ НЕИСПРАВНОСТЕЙ НЕ ПРИМЕНЯЮТСЯ.» |
| `#btnCancelFacilityForceState` | «ОТМЕНА». Closes the dialog. Nothing is sent or recorded |
| `#btnConfirmFacilityForceState` | «ПЕРЕВЕСТИ», `btn-danger`. Runs `desktopAPI.forceFacilityDeviceState` |
| `#facilityForceStateError` | `role="alert"`. A non-stale failure keeps the dialog open and shows the reason. A stale result closes the dialog and shows the stale text on the card |

### Fault switches (FR-009, FR-010)

`#facilityPlayFaults` (`role="list"`) holds one `li.facility-play-fault` per condition whose scope
is the selected device, in authored order. Terminal-scoped faults never appear on a device card.

| Element | Contract |
|---|---|
| `li.facility-play-fault` | `data-condition-id`; `data-active="true"` or `"false"` |
| `button.facility-play-fault-switch` | `role="switch"`, `aria-checked` = current position, accessible name = fault name. One press runs `desktopAPI.setFacilityConditionActive` with the opposite position, without confirmation. The «Восстановление оператором» setting never blocks it |
| `.facility-play-fault-current` | «АКТИВНА» or «НЕАКТИВНА» |
| `.facility-play-fault-initial` | «В НАЧАЛЕ ИГРЫ: ИСПРАВНО» or «В НАЧАЛЕ ИГРЫ: НЕИСПРАВНОСТЬ АКТИВНА». The switch never changes it |
| `#facilityPlayFaultsEmpty` | «У УСТРОЙСТВА НЕТ НЕИСПРАВНОСТЕЙ», with no switches |

### Reset (FR-011)

`#btnResetFacilityDevice` moves onto the card. Its text becomes «СБРОСИТЬ К НАЧАЛЬНОМУ…» and its
class `btn-secondary`. It keeps `aria-controls="facilityDeviceResetDialog"` and
`#btnResetFacilityDeviceReason`. `#facilityDeviceResetDialog`, its text, and
`#btnConfirmFacilityDeviceReset` («СБРОСИТЬ», `btn-danger`) do not change. A reset that changes only
faults succeeds and shows «УСТРОЙСТВО СБРОШЕНО».

### Feed (FR-012 to FR-014)

| Element | Contract |
|---|---|
| `#facilityPlayFeed` | `section` labelled by `#facilityPlayFeedHeading` «ПОСЛЕДНИЕ ИЗМЕНЕНИЯ» |
| `#facilityPlayFeedList` (`ol`) | `aria-live="polite"`. Up to five `li.facility-feed-entry` for the selected device, newest first by `sequence` |
| `li.facility-feed-entry` | `data-sequence`; `data-kind` (the normalized `kind`); `data-author` (`overseer` or `terminal`); `data-reason` on `player-refusal` entries. It starts with `<time datetime="<ISO 8601>">HH:MM</time>` in local time |
| `#facilityPlayFeedEmpty` | «В этом запуске изменений ещё не было.» |

The list re-renders on every `coordination-state` event and every session change while the card
is open, without moving focus (FR-014). Entry text after the time, in sentence case:

| `data-kind` | Text after «HH:MM · » |
|---|---|
| `transition` | «<transition>»: <from> → <to> · Смотритель |
| `forced` | принудительно: <from> → <to> · Смотритель |
| `fault` | неисправность «<fault>» активирована · Смотритель, or … снята · Смотритель |
| `reset` | сброс к начальному: <from> → <to> · Смотритель (only faults restored: сброс к начальному: неисправности восстановлены · Смотритель) |
| `scene-reset` | сброс сцены: <from> → <to> · Смотритель |
| `player-command` | «<command>»: <from> → <to> · <terminal> |
| `player-refusal` | «<command>» · запрос игрока отклонён: <reason> · <terminal> |

Refusal reasons render as follows:

| Reason | Text |
|---|---|
| `declined` | «Смотрителем» |
| `precondition-failed` | «предусловие не выполнено» |
| `transition-unavailable` | «переход больше недоступен» |
| `fault-blocked` | «мешает неисправность» |
| `stale` | «сцена изменилась» |
| `withdrawn` | «отозван после действия Смотрителя» |
| `unspecified` | «причина неизвестна» |

Names come from the current session. A missing name renders as its raw ID. For example, a declined
door request reads «20:47 · «Открыть выход и подать сигнал» · запрос игрока отклонён: Смотрителем ·
<terminal>».

## Device actions (FR-022, FR-023, SC-007)

`#facilityDeviceActions` contains, in order:

| Element | Contract |
|---|---|
| `#facilityDeviceInspectActions` (`role="group"`, `aria-label="Проверка"`) | `#btnPreviewFacilityDevice` «ПРЕДПРОСМОТР СОСТОЯНИЯ», then `#btnInspectFacilityDependencies` «ЗАВИСИМОСТИ». Ids and behavior are unchanged |
| `#facilityDeviceChangeActions` (`role="group"`, `aria-label="Изменение"`) | `#btnEditFacilityDevice` «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ», `btn-secondary`, `aria-haspopup="dialog"`, `aria-controls="facilityDeviceDialog"`. It opens the device dialog, where `#facilityDeviceName` renames the device. It replaces the id-less «РЕДАКТИРОВАТЬ ГРАФ» button |
| `#btnFacilityDeviceMenu` | Text «⋯», `aria-label="ДРУГИЕ ДЕЙСТВИЯ С УСТРОЙСТВОМ"`, `aria-haspopup="menu"`, `aria-expanded`, `aria-controls="facilityDeviceMenu"` |
| `#facilityDeviceMenu` | `role="menu"`, hidden until opened |
| `#btnDeleteFacilityDevice` | Inside `#facilityDeviceMenu`, `role="menuitem"`, «УДАЛИТЬ УСТРОЙСТВО». It still opens `#facilityRepairDialog` |

**Menu behavior.** Clicking «⋯», or pressing Enter, Space, or ArrowDown on it, opens the menu and
focuses the first item. Escape closes it and returns focus to «⋯». Tab, an outside click, and
choosing an item also close it. Deleting therefore takes two actions: «⋯», then «УДАЛИТЬ
УСТРОЙСТВО».

**Destructive controls.** Inside `#facilitySelectionEditor`, no `.btn-danger` exists outside
`#facilityDeviceMenu`. Play controls (`#facilityPlayCard`) appear nowhere else.

**Removed.** These elements are removed, together with their tests:

- `#facilitySelectedDeviceName` and its label;
- `#btnSaveFacilityDeviceChanges` («ГОТОВО»);
- `#btnCancelFacilityDeviceChanges` («ОТМЕНА»);
- the `device:<id>` unapplied-input entry for the device detail form;
- the «РЕДАКТИРОВАТЬ ГРАФ» copy.

This cancels the 039 contract's lines about `#btnSaveFacilityDeviceChanges` and about unapplied
input in the device detail form.

## Detail tabs (FR-024, R11)

`#facilityDetailTabs` has `role="tablist"` and `aria-label="Сведения об устройстве"`. It follows the
ARIA tabs pattern with automatic activation:

- ArrowLeft and ArrowRight move between tabs and wrap; Home and End jump to the first and last tab;
- the active tab has `tabindex="0"`, and the other tabs have `-1`;
- the active tab is remembered in a module variable for the run, and the first tab is the default.

| Tab | Label | Panel | Panel content |
|---|---|---|---|
| `#facilityTabActions` | «ЧТО ДЕЛАЮТ ИГРОКИ» + count | `#facilityTabPanelActions` | `#facilityActionBindingList` (command actions), `#btnAddFacilityActionBinding` «ДОБАВИТЬ КОМАНДУ» (opens `#facilityBindingDialog` with kind `command-action`) |
| `#facilityTabInformation` | «ЧТО ВИДЯТ ИГРОКИ» + count | `#facilityTabPanelInformation` | `#facilityInformationBindingList` (names, texts, visibility, availability), `#btnAddFacilityBinding` «ДОБАВИТЬ ПРИВЯЗКУ» |
| `#facilityTabConditions` | «НЕИСПРАВНОСТИ» + count | `#facilityTabPanelConditions` | `#facilityConditionSelectionEditor` (unchanged, shown when a condition is selected), `#facilityConditionList`, `#btnAddFacilityCondition` |
| `#facilityTabPrograms` | «ПРОГРАММЫ» + count | `#facilityTabPanelPrograms` | `#facilityRecoveryProgramList`, `#btnAddFacilityRecoveryProgram` |

- Each tab is `role="tab"` with `aria-controls` pointing to its panel, `aria-selected`, and
  `data-count`. The count renders in `span.facility-tab-count`, so the accessible name reads, for
  example, «ЧТО ВИДЯТ ИГРОКИ 3».
- Each count equals the number of rows its list renders under the current selection: the selected
  device's items, or the whole session's items when no device is selected. «ПРОГРАММЫ» counts
  recovery programs that include a transition of the selected device.
- Only the active panel is not `hidden` (`role="tabpanel"`, `aria-labelledby` its tab).
- A tab with no items stays visible with count 0. Its panel shows the empty text and the add
  control.
- `#facilityBindingList` is removed and split into `#facilityActionBindingList` and
  `#facilityInformationBindingList`, using the groups `overseer.js` already builds.

## Device list (FR-025)

Each `#facilityDeviceList` row (`button[data-facility-kind="device"][data-facility-id]`) has these
parts:

- `strong`: the device name;
- `.facility-device-row-state`: exactly «<current state name>», or «<current state name> · ● <n>»
  when n device-scoped faults are active, for example «Загрязнён · ● 1»;
- `span.facility-fault-count`: wraps «● <n>», with `aria-label="Активных неисправностей: <n>"`;
- data attributes on the row: `data-current-state-id` and `data-active-faults` (`<n>`, `0` when
  none).

The kind label leaves the row; the header shows it. Search keeps matching the name, the ID, and the
current state name. Rows re-render on every session change, so the state and the count follow play
changes from either side.
