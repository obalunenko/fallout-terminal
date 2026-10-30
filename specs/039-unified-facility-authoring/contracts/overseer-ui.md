# Overseer UI Contract: Unified Facility Authoring

Identifiers below are DOM ids or classes that browser tests may rely on. Visible copy is Russian
and upper case, matching Overseer conventions. The spec's sentence-case names map to buttons as
follows: «Отменить» → «ОТМЕНИТЬ», «Повторить» → «ПОВТОРИТЬ», «Применить снова» → «ПРИМЕНИТЬ СНОВА»,
«Сбросить сцену» → «СБРОСИТЬ СЦЕНУ». Operations are defined in `private-desktop.md`.

## Shared authoring bar (both tabs)

`#authoringBar` sits next to `#terminalWorkspaceTab` and `#facilityWorkspaceTab`. It stays visible
on both tabs and contains:

| Element | Role and copy | Behavior |
|---|---|---|
| `#saveStatus` (moved from the terminal sidebar) | `role="status"`, `aria-live="polite"` | Shows a primary part, in this priority: «ОШИБКА СОХРАНЕНИЯ: <reason>», «СОХРАНЕНИЕ…», «СОХРАНЕНО · HH:MM:SS». When input is unapplied, it adds a secondary part, for example «ОШИБКА СОХРАНЕНИЯ: <reason> · НЕ ПРИМЕНЕНО: <label>[, <label>…]». `data-state` holds the primary state (`error`, `saving`, `saved`, `idle`), and `data-unapplied-count` holds the number of unapplied items |
| `#btnUndo` | «ОТМЕНИТЬ» | Undoes the newest edit. `aria-describedby` names it: «ОТМЕНИТЬ: <label>» |
| `#btnRedo` | «ПОВТОРИТЬ» | Redoes the newest undone edit |
| `#btnReapplyEdit` | «ПРИМЕНИТЬ СНОВА» | Shown only after a stale rejection; re-runs the rejected edit on the newer session |
| `#btnResetScene` | «СБРОСИТЬ СЦЕНУ», `aria-haspopup="dialog"` | Opens `#sceneResetDialog` |

A blocked control (FR-005, R14) keeps focus and gets `aria-disabled="true"`. Its
`aria-describedby` points to a `.control-reason` element with one of these reasons:

- «ДОЖДИТЕСЬ СОХРАНЕНИЯ»
- «ДЕМО ОТКРЫТО ТОЛЬКО ДЛЯ ЧТЕНИЯ — СОЗДАЙТЕ КОПИЮ»
- «НЕТ ОТКРЫТОЙ СЕССИИ»
- «НЕЧЕГО ОТМЕНЯТЬ»
- «НЕЧЕГО ПОВТОРЯТЬ»
- «СЕССИЯ ИЗМЕНИЛАСЬ ПОСЛЕ ЭТОЙ ПРАВКИ»
- «ИСТОРИЯ ХРАНИТ 20 ПОСЛЕДНИХ ПРАВОК»
- «СЦЕНА УЖЕ В НАЧАЛЬНОМ СОСТОЯНИИ»
- «ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА»
- the server's issue text for a rejected undo or redo

Activating a blocked control does nothing.

## Removed elements and copy (FR-004, FR-006, FR-001c, FR-015)

Removed elements: `#btnSaveFacility`, `#btnDiscardFacility`, `#btnResetFacility`,
`#facilityResetDialog`, `#commandDraftGuard` and its buttons, `#btnPublish`, and the per-terminal
«СБРОСИТЬ ВСЕ СОСТОЯНИЯ».

None of these strings may remain anywhere in `index.html` or `overseer.js`:

- «ПРИМЕНИТЬ К ЧЕРНОВИКУ»
- «СОХРАНИТЬ ИЗМЕНЕНИЯ СЕССИИ»
- «СОХРАНИТЬ ЧЕРНОВИК СЦЕНАРИЯ»
- «СОХРАНИТЬ ЧЕРНОВИК И ОТКРЫТЬ ТЕРМИНАЛЫ»
- «ОТМЕНИТЬ ИЗМЕНЕНИЯ»
- «ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ»
- «СБРОСИТЬ ВСЕ СОСТОЯНИЯ»
- «СБРОСИТЬ ВЕСЬ ОБЪЕКТ»
- «ЧЕРНОВИК», in any case

A browser sweep asserts this. `#createTerminalDialogDescription` becomes «Новый терминал сохранится
сразу и не появится у игроков, пока вы не сделаете его активным.»

## Facility dialogs

- Every facility dialog that edits data confirms with a submit button «ГОТОВО» and cancels with
  «ОТМЕНА». This covers device, condition, recovery program, and binding.
- The device detail form's apply button `#btnSaveFacilityDeviceChanges` reads «ГОТОВО».
- Confirmation-only dialogs keep verb buttons («СБРОСИТЬ», «ВОССТАНОВИТЬ»), because they run live
  operations, not edits. The single-device reset and recovery dialogs are examples.
- «ГОТОВО» commits through the shared save pipeline. The dialog closes when the edit is applied
  locally. A server failure reopens the dialog with the author's input and the reason in its error
  element.

## Unapplied input

- These forms keep their unapplied values across tab switches and live re-renders, keyed as in
  `data-model.md`: the terminal node form (`#nodeForm`), terminal settings (`#introText`,
  `#hackLevelSelect`), and the device detail form.
- An element whose value differs from the applied value has `data-unapplied="true"`.

## Command form: lists (ТЕРМИНАЛЫ → state-changing command)

- `#commandEffects`, heading «ПРИ ОДОБРЕНИИ»: one `li.command-effect` per effect, with
  `data-kind` one of `transition`, `recovery`, `completed-name`, `entry-block`, `terminal-transition`.
  - Transitions read «<device>: <from> → <to>».
  - The block effect reads «<entry> · блок <n>: <new text>».
  - A terminal-transition command shows only «ПЕРЕХОД: <target terminal>», with `data-kind`
    `terminal-transition`.
- `#commandRules`, heading «ЧТО ВИДЯТ ИГРОКИ»: one `section.command-rule-target` per target, with
  `data-target` one of `name`, `visibility`, `availability`, or `block:<blockId>`. Each holds
  `li.command-rule` items in precedence order, lowest first. Each item has:
  - `data-kind`, the rule kind in lower kebab case, for example `state-name`;
  - the condition text (for example «КОГДА Гермодверь Убежища 76 = Открыта», «ПОСЛЕ ВЫПОЛНЕНИЯ», or
    «ВСЕГДА») and the value;
  - `.rule-players-see` «ВИДЯТ ИГРОКИ» on exactly one item per target;
  - `.rule-overrides` «ПЕРЕКРЫВАЕТ: <lower kinds>» on every item that outranks another item of the
    same target while its condition holds;
  - an «ИЗМЕНИТЬ» button that opens the owning editor: the node form field, the binding dialog, or
    the condition dialog.
- While the explanation for the current authoring revision is loading, `#commandRules` has
  `aria-busy="true"` and keeps its previous content.

## Terminal tree

- Each `.tree-node` label shows the player name. `data-authored-name` holds the authored name, and
  `title` reads «В РЕДАКТОРЕ: <authored>» when the two differ.
- `.tree-badge-hidden` shows «СКРЫТО ОТ ИГРОКОВ», and `.tree-badge-unavailable` shows «НЕДОСТУПНО».

## Scene reset

- `#sceneResetDialog` lists counts: «УСТРОЙСТВА: n», «НЕИСПРАВНОСТИ: n», «ВЫПОЛНЕННЫЕ КОМАНДЫ: n
  (<terminal>: k, …)».
- It also states «ИГРОКИ ОСТАНУТСЯ НА ТЕКУЩЕМ ТЕРМИНАЛЕ; МАРШРУТ И ПРОГРЕСС ВЗЛОМА СОХРАНЯТСЯ.»
- When a request is pending, it adds «ОЖИДАЮЩИЙ ЗАПРОС ИГРОКА: <command> — <outcome>», where the
  outcome is «БУДЕТ ОТКЛОНЁН» for a device action or a reset command, and «ОСТАНЕТСЯ В ОЖИДАНИИ»
  otherwise.
- Buttons: «ОТМЕНА», and a danger button «СБРОСИТЬ СЦЕНУ» (`#btnConfirmSceneReset`).
- A stale result refreshes the counts in place and says «СЦЕНА ИЗМЕНИЛАСЬ — ПРОВЕРЬТЕ СПИСОК».
  A failure says «СЦЕНА НЕ СБРОШЕНА: <reason>».

## Preview dialog (`#facilityPreviewDialog`)

- `#facilityPreviewContext` reads «ПРЕДПРОСМОТР ПОКАЗЫВАЕТ ПРИМЕНЁННЫЕ ПРАВКИ И НЕ МЕНЯЕТ МИР ДЛЯ
  ИГРОКОВ.»
- `#facilityPreviewState` preselects the first state that differs from the current one.
  `#facilityPreviewActive` preselects the opposite of the current fault activity.
- A one-state device shows «НЕЧЕГО СРАВНИВАТЬ: У УСТРОЙСТВА ОДНО СОСТОЯНИЕ» and makes no request.
- `#facilityPreviewCurrent` («СЕЙЧАС: <state>») and `#facilityPreviewHypothetical` («ЕСЛИ:
  <state>») render side by side, stacking on narrow widths.
- `#facilityPreviewSummary` reads «ИЗМЕНЕНИЙ: n» followed by one `li` per difference, with
  `data-kind` one of `name-changed`, `text-changed`, `appears`, `disappears`, `becomes-available`,
  `becomes-unavailable`. Each is labeled «ПЕРЕИМЕНУЕТСЯ», «ТЕКСТ ИЗМЕНИТСЯ», «ПОЯВИТСЯ», «ИСЧЕЗНЕТ»,
  «СТАНЕТ ДОСТУПНО», or «СТАНЕТ НЕДОСТУПНО», and names the element.
- Each differing tree item in both panes carries the same `data-kind` and the same badge word.
- With no difference, the summary reads «ИГРОКИ НЕ УВИДЯТ ИЗМЕНЕНИЙ».
