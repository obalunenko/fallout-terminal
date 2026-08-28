# Исследование: переходы между терминалами

**Bugfix**: 2026-08-19 — BUG-004 Уточнены общий persistence `oneof` и единый authoring mode selector.
**Bugfix**: 2026-08-28 — BUG-007 Уточнено переиспользование общего rejected-command presentation без объединения pending lifecycle.

## 1. Авторская ссылка в session JSON version 1

**Decision**: ~~Добавить к protobuf `CommandContent` независимый optional `TerminalTransitionConfig` рядом с optional `state_change`.~~ По BUG-004 сохранить optional JSON-семантику `terminalTransition` и `version: 1`, но объединить protobuf `state_change = 2` и `terminal_transition = 3` в один реальный `oneof behavior`; unset oneof означает обычную команду.

**Rationale**: Связь — это авторское свойство команды, а стабильный terminal ID переживает переименование. Необязательный oneof сохраняет прежнее поведение всех существующих v1-файлов; те же field numbers/JSON names сохраняют wire/JSON shape валидных документов, а прямое domain↔protobuf отображение и `nodeFields` оставляют unknown-field preservation без изменения storage pipeline.

**Alternatives considered**: Отдельная таблица связей на уровне session дублирует identity команды; хранение имени цели ломает rename; новая JSON-версия не нужна для совместимого optional-расширения; два независимых optional config оставляют взаимоисключающий инвариант вне generated type и отклонены BUG-004.

## 2. Двухпроходная валидация и взаимоисключающие режимы команды

**Decision**: `ValidateSession` сначала собирает все terminal IDs, затем валидирует деревья и cross-terminal references. `terminalTransition` разрешён только у command, должен ссылаться на существующий другой терминал и взаимоисключающ с `stateChange`. По BUG-004 persistence protobuf выражает это одним oneof; validation остаётся защитой для session JSON/import boundary и malformed legacy input.

**Rationale**: Два прохода разрешают ссылки на терминал, идущий позже в массиве, и не делают валидность зависимой от порядка. Взаимоисключение убирает неопределённость между durable execution approval и terminal-navigation approval; completed state-changing command сначала надо сбросить.

**Alternatives considered**: Приоритет одного config над другим делает session неоднозначной и осложняет UI; однопроходная проверка ложно отклоняет forward references.

## 3. Отдельный pending lifecycle вместо ручного switch

**Decision**: Добавить в coordinator отдельный `PendingTerminalNavigation` с direction `forward`/`return` и private `ResolveTerminalNavigation` с decision `approve`/`reject`. Существующий `PendingTerminalSwitch` остаётся только для ручной смены с preserve/discard/cancel.

**Rationale**: Прямой переход должен всегда сохранять checkpoint и завершаться одним решением. Ручной switch открывает второй unfinished-hack dialog и пока ожидает, не блокирует player navigation, что прямо противоречит требованиям.

**Alternatives considered**: Переиспользование `PendingCommandExecution` смешивает runtime navigation с durable command state; переиспользование `PendingTerminalSwitch` ломает single-decision и блокировку.

## 4. Существующий `Navigate` для прямого перехода и возврата

**Decision**: Прямой переход начинает существующий `NavigateCommand.node_id`. Межтерминальная кнопка возврата на корне отправляет существующий `NavigateBack`; server интерпретирует root-list back как return request только при непустом route.

**Rationale**: Оба действия остаются в одном unary mutation family с теми же authorization, fingerprint, replay, revision и ConnectRPC guarantees. Player не передаёт target ID, а обычный back внутри папок/записей/результатов не меняется.

**Alternatives considered**: Новый public RPC дублирует всю mutation envelope; передача target ID из browser создаёт лишний недоверенный input.

## 5. LIFO-маршрут и восстановление перемещённых папок

**Decision**: `LiveBroadcast` хранит LIFO-массив `TerminalReturnPoint`. Точка содержит source terminal ID/name, stable containing-folder ID, цепочку ancestor folder IDs, command ID/name. Возврат сначала ищет containing folder по stable ID в актуальном дереве и восстанавливает его новую ancestry; если он удалён, ищет сохранившегося предка от ближайшего к корню.

**Rationale**: Стабильный folder ID переживает rename и move. Сохранённые ancestors дают детерминированный nearest-parent fallback, а command ID сохраняет идентичность перехода. Pop выполняется только в approve-транзакции.

**Alternatives considered**: Только старый `NavState.Path` не находит папку после move; только command ID не даёт fallback, если команда/папка удалена; static graph не воспроизводит фактическую history в циклах.

## 6. Текущая session как trusted terminal catalog

**Decision**: Добавить в `control.Config` узкий synchronous `TerminalCatalog`, построенный над каноническим `session.Service.Snapshot()`. Coordinator получает latest validated `TerminalTarget` при создании pending и повторно при approve.

**Rationale**: Browser и master frontend не должны поставлять target payload для player-driven transition. Повторный lookup отклоняет удалённую цель или команду, чей target ID изменился пока мастер рассматривал запрос. Однонаправленный lock order `control → session` уже используется `CommandStateStore` и не допускает callback в coordinator.

**Alternatives considered**: Замороженный target payload в pending не замечает stale/delete; повторная передача payload из frontend расширяет privileged trust boundary.

## 7. Checkpoint-ы для hack continuity, но явная navigation placement

**Decision**: Переиспользовать `LiveBroadcast.TerminalRuntimes` и `SuspendRuntime`/`ReactivateRuntime` для точного private hack checkpoint. После reactivation прямой переход явно ставит destination `Nav` в root list, а return — в восстановленный folder list. Ни один из этих путей не генерирует новую hack board для существующего checkpoint.

**Rationale**: Существующий runtime уже хранит generation, secret, attempts, log, solved/failed и used patterns в process memory; public projection скрывает secret. Поэтому нужна новая семантика placement, а не второй hack store.

**Alternatives considered**: Сохранение hack state в session JSON нарушает runtime-only boundary; `DiscardRuntime` теряет solved/unfinished state; обычный `ReactivateRuntime` неверно восстанавливает старую destination navigation вместо root.

## 8. Минимальная public projection, полный private prompt

**Decision**: Публичный `LiveTerminal` получает optional `TerminalNavigationPresentation`: route depth, верхнюю return target и secret-free pending direction/target. Private `CoordinationState` получает полный `PendingTerminalNavigation` с decision ID, direction, source, command и target. Полный route stack остаётся coordinator-private. По BUG-007 forward transition pending остаётся отдельным terminal-navigation lifecycle, но exact reject/close после его очистки публикует для source command существующий detached `CommandExecutionPresentation{Phase: REJECTED}` как общий post-decision screen до controller acknowledgement; return reject не публикует command presentation.

**Rationale**: Player-клиенту нужны только авторитетное ожидание и возможность вернуться; master-клиенту нужны exact ID и все поля понятного решения. Оба получают state в первом snapshot и в монотонных updates.

**Alternatives considered**: Публикация decision ID/всего route излишне расширяет public surface; client-local route ломает reconnect и multi-player convergence; ~~переиспользование `CommandExecutionPresentation` смешивает два независимых lifecycle~~ BUG-007 различает запрещённое объединение pending state и допустимое переиспользование уже detached rejected-command presentation после решения, поэтому новые protocol fields не нужны.

## 9. Без новых зависимостей

**Decision**: Использовать только текущие Go, Protobuf, ConnectRPC, Wails и Playwright toolchains.

**Rationale**: Нужные mutex/revision/replay, schema generation, stream и browser-test primitives уже есть в репозитории.

**Alternatives considered**: Новый state-machine, graph или persistence package не даёт функции новой гарантии и увеличивает стоимость cutover.
