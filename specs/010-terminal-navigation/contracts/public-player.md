# Публичный player-контракт

## Мутации

Новый RPC не добавляется. Контроллер использует существующий unary `PlayerService.Navigate`:

- `NavigateCommand.node_id` выбирает command; backend один разрешает hidden `terminalTransition` и target.
- `NavigateBack` на root list при непустом route создаёт return request. Во всех прежних navigation modes его семантика не меняется.

Request сохраняет existing `recognition_handle`, `request_id`, `broadcast_id` и `terminal_id`. Player не передаёт target terminal ID, route, decision ID или approve/reject.

## Совместимые protobuf-добавления

```proto
enum TerminalNavigationDirection {
  TERMINAL_NAVIGATION_DIRECTION_UNSPECIFIED = 0;
  TERMINAL_NAVIGATION_DIRECTION_FORWARD = 1;
  TERMINAL_NAVIGATION_DIRECTION_RETURN = 2;
}

message TerminalReturnTarget {
  string terminal_id = 1;
  string terminal_name = 2;
}

message PendingTerminalNavigationPresentation {
  TerminalNavigationDirection direction = 1;
  string target_terminal_id = 2;
  string target_terminal_name = 3;
}

message TerminalNavigationPresentation {
  uint32 route_depth = 1;
  optional TerminalReturnTarget return_target = 2;
  optional PendingTerminalNavigationPresentation pending = 3;
}

message LiveTerminal {
  // existing fields 1..8 unchanged
  optional TerminalNavigationPresentation terminal_navigation = 9;
}
```

`terminal_navigation` присутствует, когда route не пуст или transition ожидает решения. `return_target` показывает только верхнюю точку; full route не публикуется. `pending` не содержит master request ID, command config или controller identity.

## Authorization, replay and rejection

1. Existing session/connection, request-ID/fingerprint, broadcast, assignment, controller и active-terminal checks выполняются до transition logic.
2. Observer/unassigned/disconnected controller получает существующий typed `ActionReason` и не меняет state.
3. Exact replay того же `request_id`/того же fingerprint возвращает cached `ActionResult`. Тот же ID с другим payload возвращает `DUPLICATE`.
4. Пока `PendingTerminalNavigation` существует, любой новый Navigate/Guess/ActivatePattern текущего controller возвращает `CONFLICT` без gameplay call, RNG и revision.
5. Missing/self/stale target при player request возвращает `INVALID_ACTION`; committed revision может продвинуться только для private typed notice мастеру, но public active terminal, nav, route и checkpoints не меняются.

## Stream ordering and reconnect

- Создание pending публикует более новую revision с тем же active terminal и `terminal_navigation.pending`.
- Approve публикует одну более новую revision: новый `PlayerState.active_terminal_id`, полную `TerminalPresentation`, обновлённый route depth и no pending.
- Reject forward-перехода публикует одну revision, которая убирает navigation pending и добавляет для exact source command существующий `command_execution = REJECTED`; active terminal, nav и route совпадают с pre-request state. Reject terminal-return убирает pending без command presentation и сохраняет прежнее непосредственное восстановление current root screen.
- Stale approve убирает pending и оставляет active/nav/route без изменений.
- `PersonalizedSnapshot` всегда содержит текущие active terminal, pending и route projection; reconnect не восстанавливает client-local экран.
- Existing strictly-monotonic stream revision, overflow close и resubscribe baseline остаются без изменений.

## Player presentation

- На root list при `return_target` client показывает межтерминальный back control с target terminal name. В folder/entry/command screen существующий back остаётся intra-terminal.
- Pending показывает общий status перехода/возврата и делает shared controls inert; server-side conflict check остаётся окончательным.
- После reject forward-перехода общий command renderer показывает «Ошибка доступа» controller, observers и reconnect до `Back`/`Enter` controller, затем синхронно возвращает неизменённое source menu.
- Controller не меняет terminal/nav оптимистически; `pendingSharedAction` ждёт unary result и accepted stream revision.
- Observer видит ту же projection, но его controls остаются read-only.

## Compatibility

Existing fields `LiveTerminal` 1–8, all `NavigateRequest` fields/action numbers, `ActionReason` values и `PlayerService` methods не меняются. Old clients ignore field 9 and не показывают return control; feature acceptance требует regenerated bundled client, а не permanent dual behavior.
