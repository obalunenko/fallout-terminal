# Модель данных: переходы между терминалами

**Bugfix**: 2026-08-19 — BUG-004 Уточнён дискриминированный режим содержимого команды.
**Bugfix**: 2026-08-20 — BUG-005 Уточнён один логический approval lifecycle и взаимное исключение типизированных command/transition/return pending-состояний.
**Bugfix**: 2026-08-28 — BUG-006 Уточнён ordinary-command `REJECTED` lifecycle до controller acknowledgement.

## Границы жизненного цикла

| Состояние | Владелец | Жизненный цикл | Persistence |
|---|---|---|---|
| `TerminalTransitionConfig` | session/domain | Пока существует command-узел | session JSON version 1 |
| `TerminalReturnPoint` | coordinator | Один broadcast | Нет |
| `PendingCommandExecution` | coordinator | Один broadcast, взаимоисключён с terminal-navigation pending | Нет |
| `CommandExecutionPresentation` | active terminal runtime / detached public projection | `PENDING` до решения; ordinary и initial state-change `REJECTED` до controller acknowledgement | Нет |
| `PendingTerminalNavigation` | coordinator | Один broadcast, не более одного запроса | Нет |
| `TerminalNavigationNotice` | coordinator/private master projection | До следующего transition/lifecycle action | Нет |
| `TerminalRuntime` | coordinator/live | Один broadcast | Нет |
| `TerminalNavigationPresentation` | detached public projection | Одна snapshot/update revision | Нет |

## Durable entities

### `TerminalTransitionConfig`

| Поле | Тип | Правила |
|---|---|---|
| `TargetTerminalID` | `string` | Обязательный stable ID другого терминала в той же session. |

~~`ContentNode` получает независимый `TerminalTransition *TerminalTransitionConfig` рядом с `StateChange`, а их взаимоисключение выражается только validation.~~ По BUG-004 command content имеет один дискриминированный режим: ordinary/unset, `StateChange` или `TerminalTransition`. Persistence protobuf выражает два configured-варианта общим `oneof behavior`; JSON v1 сохраняет прежние `stateChange`/`terminalTransition` names и допускает не более одного из них. Transition разрешён только при `Type == NodeCommand` и не позволяет target, равный содержащему terminal ID.

### Session reference validation

1. Проверить version, session fields, лимит терминалов, все terminal ID/name и уникальность IDs.
2. Построить множество всех terminal IDs.
3. Проверить каждое дерево, node variants, command states и единственный command behavior variant.
4. Отклонить blank, missing, self-target и malformed JSON/import input с `stateChange` + `terminalTransition`, даже если protobuf producer уже ограничен общим oneof.

Отсутствие обоих `stateChange` и `terminalTransition` — legacy default «обычная команда». Неизвестные JSON-поля на session, terminal и node продолжают round-trip через `Extra`.

## Broadcast runtime entities

### `TerminalReturnPoint`

| Поле | Тип | Назначение |
|---|---|---|
| `TerminalID` | `string` | Терминал, из которого был одобрен прямой переход. |
| `TerminalName` | `string` | Отделённая подпись для prompt/projection; identity определяет ID. |
| `FolderID` | `string` | Stable ID папки, в чьём списке была выбрана команда; `root` разрешён. |
| `AncestorFolderIDs` | `[]string` | Цепочка от `root` до parent `FolderID` для nearest-survivor fallback. |
| `CommandID` | `string` | Stable identity команды, создавшей точку. |
| `CommandName` | `string` | Имя на момент approve для понятного return prompt. |

`LiveBroadcast.Route []TerminalReturnPoint` имеет семантику stack: direct approve добавляет одину точку; return approve удаляет только последнюю. Reject/stale/error stack не меняют.

### `PendingTerminalNavigation`

| Поле | Тип | Правила |
|---|---|---|
| `RequestID` | `string` | Непредсказуемый server-owned ID решения. |
| `BroadcastID` | `BroadcastID` | Должен совпасть с текущим broadcast при resolve. |
| `ControllerSessionID` | `LogicalSessionID` | Инициатор; private, не публикуется. |
| `Direction` | `forward` \| `return` | Обязательный вариант. |
| `SourceTerminalID`, `SourceTerminalName` | `string` | Активный source на момент запроса. |
| `CommandID`, `CommandName` | `string` | Выбранная forward-команда или команда верхней return point. |
| `TargetTerminalID`, `TargetTerminalName` | `string` | Кандидат цели; перепроверяется при approve. |
| `ReturnPoint` | `TerminalReturnPoint` | Для forward — кандидат push; для return — точная копия верхней точки, которая ещё не извлечена. |

В `ProcessRuntime` может быть не более одного `PendingTerminalNavigation`. По BUG-005 `PendingCommandExecution` для ordinary/initial/completed state-change и `PendingTerminalNavigation` для terminal-transition/return являются типизированными проекциями одного логического approval lifecycle и взаимно исключаются: одновременно во всём broadcast существует не более одного pending любого из этих видов. Пока любой из них существует, все shared navigation/hack actions отклоняются как conflict после обычных identity/authority checks.

По BUG-006 exact reject или close-as-reject ordinary-команды очищает `PendingCommandExecution`, но сохраняет на active runtime `CommandExecutionPresentation{Phase: REJECTED, CommandID: exact command}`. Эта detached projection показывает controller, observers и reconnect полноэкранный record-description «Ошибка доступа» без authored result или source menu. Только `Back` или `Enter` controller очищает presentation и возвращает общую проекцию к неизменённой source navigation; acknowledgement не выполняет command и не меняет persistence, active terminal или route. Completed state-changing, terminal-transition и return rejection lifecycles не меняются.

### `TerminalNavigationNotice`

Приватное, не содержащее diagnostic details уведомление с reason `target_missing`, `self_target`, `command_stale` или `target_changed`, а также source/command/target IDs. Оно даёт master UI понятное объяснение, но не публикует file paths, errors или полную session players.

## Terminal checkpoint semantics

`LiveBroadcast.TerminalRuntimes[terminalID]` остаётся единственным checkpoint-хранилищем. Переход не копирует hack state в route/pending:

- первый вход создаёт runtime и обычную hack board;
- suspend сохраняет весь private `HackState`;
- reactivation применяет latest authored content, не меняя solved/failed/attempts/generation/log;
- direct approve устанавливает destination navigation в root list;
- return approve устанавливает source navigation в restored folder list;
- explicit reset/discard сохраняет прежние правила создания нового checkpoint.

## State transitions

| Событие | Preconditions | Результат |
|---|---|---|
| Controller выбирает linked command | Current broadcast/terminal/controller; нет pending; config и target актуальны | Создать forward pending, route/active/checkpoint не менять. |
| Controller нажимает root return | Root list; route не пуст; нет pending | Создать return pending из верхней точки, но не pop. |
| Master approve forward | Exact pending, source всё ещё active, latest command всё ещё указывает на existing target | Suspend source, push point, create/reactivate target, set target root, clear pending/notice, publish one revision. |
| Master approve return | Exact pending и unchanged top point, latest source terminal exists | Suspend current, reactivate return target, restore folder/fallback, pop one point, clear pending/notice, publish one revision. |
| Master reject/close | Exact pending | Clear pending; active terminal, nav, route и checkpoints не менятся. |
| Master reject/close ordinary command | Exact `PendingCommandExecution` mode ordinary | Clear pending; set exact command presentation to `REJECTED`; authored result, active terminal, nav, route, checkpoints and persistence не менять. |
| Controller acknowledges ordinary rejection | Exact ordinary `REJECTED`; `Back` или `Enter` от current controller | Clear command presentation and publish unchanged source menu to all players; no command effect or durable write. |
| Stale approve | Target/command/source/route больше не совпадают | Clear pending, set private notice/error, active/nav/route не менятся. |
| Invalid linked command | Missing/self/stale target до pending | Rejected `invalid-action`; одна private-notice revision, но active/nav/route/checkpoints не меняются. |
| Competing action | Любой command/transition/return pending exists | Rejected `conflict`, без новой revision, gameplay call или RNG. |
| Manual activate/clear | Master action | Cancel pending navigation and clear route before existing manual switch semantics. |
| End broadcast / shutdown | Lifecycle action | Drop route, pending, notice и all terminal runtimes; durable session unchanged. |

## Clone and rollback invariants

- `TerminalTransitionConfig`, route entries, ancestor slices, pending return point, notice и public/private projections deep-clone at every transaction/snapshot boundary.
- Обычное authorization/conflict rejection не меняет cloned terminal slots, route и revision. Invalid-link notice и exact master reject — это канонические private/pending changes с одной новой revision, но без active/nav/route/checkpoint mutation.
- `TerminalCatalog` возвращает detached target; coordinator не хранит pointer на session-owned state.
- Stream/public adapters не публикуют private `HackState`, decision ID, controller session ID или full route stack.
