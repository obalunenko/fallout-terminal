# Overseer UI Contract: Facility Authoring P0 Fixes

The existing `saveFacilityAuthoring`, `inspectFacilityDependencies`, `previewFacility`,
`resetFacilityDevice`, `resetFacility`, recovery, and terminal autosave operations remain
unchanged. No serialized contract, Wails method, event, or protobuf field is added.

Identifiers below are DOM ids or classes that browser tests may rely on. Visible copy is in
Russian and upper case, matching the surrounding Overseer conventions.

## Global rule

- `overseer.css` starts with `[hidden] { display: none !important; }`.
- In every Overseer dialog and form, a visible `label`, `legend`, or `.field-label-inline` always
  describes a visible control (FR-019, FR-020).

## Identifier suggestion (device, state, transition, fault, recovery program dialogs)

- Typing a name updates the identifier field only while the field has not been edited by hand and
  only for new entities. Existing identifiers are read-only as today and never rewritten.
- Suggested values follow `data-model.md` (FR-008 table, prefix rules, `-2` collisions, numbered
  fallback, 256-character limit).
- On apply, the device dialog marks each offending state or transition control with
  `aria-invalid="true"` and a `.facility-row-error` message (linked via `aria-describedby`) that
  names the problem, for example «УКАЖИТЕ НАЗВАНИЕ СОСТОЯНИЯ», «НАЗВАНИЕ УЖЕ ИСПОЛЬЗУЕТСЯ: Открыта», or
  «ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: otkryta». `#facilityDeviceError` summarizes «ИСПРАВЬТЕ ОТМЕЧЕННЫЕ СТРОКИ: N». Focus moves to the
  first offending control.

## Command form (ТЕРМИНАЛЫ → command properties)

- `#commandModeDecision` (role `status`) appears directly below `#fldCommandMode` when the selected
  mode is not «ИЗМЕНЯЕТ СОСТОЯНИЕ» and the command has an effective device action (draft action if
  a facility draft exists, otherwise the saved action). It lists the action summary and offers:
  - `#btnKeepStateChangeMode` — «ОСТАВИТЬ «ИЗМЕНЯЕТ СОСТОЯНИЕ»»: selector returns to state-changing;
    notice disappears; nothing is saved.
  - `#btnRemoveDeviceAction` — «УДАЛИТЬ ДЕЙСТВИЕ И ПЕРЕКЛЮЧИТЬ»: decision recorded; notice shows the
    chosen outcome; nothing is saved until ПРИМЕНИТЬ.
- While the decision is pending, `#btnApplyNode` is disabled and the notice says «ВЫБЕРИТЕ, ЧТО
  СДЕЛАТЬ С ДЕЙСТВИЕМ УСТРОЙСТВ». Re-selecting «ИЗМЕНЯЕТ СОСТОЯНИЕ» dismisses the notice.
- `.command-facility-action` renders only while `#fldCommandMode` is «ИЗМЕНЯЕТ СОСТОЯНИЕ». For a
  command not yet saved in that mode, it shows «ПРИМЕНИТЕ РЕЖИМ, ЧТОБЫ ДОБАВИТЬ ДЕЙСТВИЕ
  УСТРОЙСТВ» and `#btnEditCommandFacilityAction` is disabled.
- Draft guard: when the facility draft has unsaved changes and the command has an effective device
  action, `#fldCommandMode` and `#btnApplyNode` are disabled and `#commandDraftGuard` (role
  `status`) explains «ЕСТЬ НЕСОХРАНЁННЫЙ ЧЕРНОВИК ОБЪЕКТОВ» with:
  - `#btnSaveFacilityDraftFromCommand` — «СОХРАНИТЬ ЧЕРНОВИК ОБЪЕКТОВ» (same operation as
    `#btnSaveFacility`);
  - `#btnDiscardFacilityDraftFromCommand` — «ОТМЕНИТЬ ЧЕРНОВИК» (same operation as
    `#btnDiscardFacility`).
  After either succeeds the form re-renders unlocked; on failure the guard stays and shows the
  existing facility error text.

## Binding dialog, kind «ДЕЙСТВИЕ КОМАНДЫ»

- When the selected node is a command without `stateChange` in the draft, the fieldset
  `#facilityBindingAction` shows `#facilityBindingCompletedName` («НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ») and
  `#facilityBindingConfirmationText` («ЗАПРОС ПОДТВЕРЖДЕНИЯ»), both empty with example placeholders
  and both required. Otherwise both fields and their labels are hidden.
- Submitting with either empty focuses it with «ЗАПОЛНИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ И ЗАПРОС
  ПОДТВЕРЖДЕНИЯ». A successful submit writes both texts and the action into the draft command.
- The editor never derives these texts from the command's original name or result text.

## Fault dialog and fault views

- `#facilityConditionDialog` contains a fieldset «В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА» with radios
  `name="conditionInitialActive"`: `#facilityConditionInitialWorking` («ИСПРАВНО», default for new
  faults) and `#facilityConditionInitialActive` («НЕИСПРАВНОСТЬ АКТИВНА»), preselected from the
  existing fault when editing.
- Help text (sentence case, like other dialog help): «Так неисправность начнёт игру, и к этому же вернёт сброс. Текущее состояние меняется
  только у новой неисправности.»
- Fault rows in `#facilityConditionList` and `#facilitySelectedConditionSummary` show
  «СЕЙЧАС: АКТИВНА|НЕАКТИВНА · В НАЧАЛЕ: АКТИВНА|ИСПРАВНО».

## Accessibility expectations

New notices use `role="status"` and are announced once; disabled buttons keep their accessible
names; radios form a labelled group; focus after keep/remove stays on the mode selector; focus
after a guard action returns to the mode selector.
