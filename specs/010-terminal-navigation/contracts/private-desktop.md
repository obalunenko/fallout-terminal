# Приватный desktop-контракт

**Bugfix**: 2026-08-20 — BUG-005 Уточнены единый логический approval lifecycle и взаимное исключение типизированных command/transition/return pending-состояний.

## Coordination projection

```proto
enum TerminalNavigationDecision {
  TERMINAL_NAVIGATION_DECISION_UNSPECIFIED = 0;
  TERMINAL_NAVIGATION_DECISION_APPROVE = 1;
  TERMINAL_NAVIGATION_DECISION_REJECT = 2;
}

enum TerminalNavigationNoticeReason {
  TERMINAL_NAVIGATION_NOTICE_REASON_UNSPECIFIED = 0;
  TERMINAL_NAVIGATION_NOTICE_REASON_TARGET_MISSING = 1;
  TERMINAL_NAVIGATION_NOTICE_REASON_SELF_TARGET = 2;
  TERMINAL_NAVIGATION_NOTICE_REASON_COMMAND_STALE = 3;
  TERMINAL_NAVIGATION_NOTICE_REASON_TARGET_CHANGED = 4;
}

message PendingCommandExecution {
  string request_id = 1;
  string broadcast_id = 2;
  string terminal_id = 3;
  string command_id = 4;
  string command_name = 5;
  string confirmation_text = 6;
  // ordinary | state-change | completed-state-change
  string command_mode = 7;
}

message PendingTerminalNavigation {
  string request_id = 1;
  string broadcast_id = 2;
  fallout.terminal.player.v1.TerminalNavigationDirection direction = 3;
  string source_terminal_id = 4;
  string source_terminal_name = 5;
  string command_id = 6;
  string command_name = 7;
  string target_terminal_id = 8;
  string target_terminal_name = 9;
  uint32 route_depth = 10;
}

message TerminalNavigationNotice {
  TerminalNavigationNoticeReason reason = 1;
  string source_terminal_id = 2;
  string command_id = 3;
  optional string target_terminal_id = 4;
}

message CoordinationState {
  // existing fields 1..6 unchanged
  optional PendingCommandExecution pending_command_execution = 7;
  optional PendingTerminalNavigation pending_terminal_navigation = 8;
  optional TerminalNavigationNotice terminal_navigation_notice = 9;
}
```

`coordination.proto` переиспользует public enum direction, но private pending не попадает в public descriptor. Existing `coordination-state` named event и `GetRuntimeStatus.coordination_state` доставляют один и тот же current pending/notice для live UI и master reload.

`pending_command_execution` используется для ordinary, initial state-changing и completed state-changing command. Поле `command_mode` аддитивно фиксирует exact post-approve behavior; ordinary не требует нового authoring config, а completed state-changing сохраняет текущие displayed name и frozen result без повторной durable write. Terminal-transition и route-return продолжают использовать `pending_terminal_navigation`. По BUG-005 оба типизированных pending-вида входят в один логический approval lifecycle и взаимно исключаются на уровне coordinator: одновременно существует не более одного ordinary/state-change/terminal-transition/return request.

## Resolve operation

```proto
message ResolveTerminalNavigationRequest {
  string request_id = 1;
  TerminalNavigationDecision decision = 2;
}

message ResolveTerminalNavigationResult {
  bool ok = 1;
  optional string error = 2;
  CoordinationState state = 3;
}
```

Узкий Wails-метод:

```text
ResolveTerminalNavigation(ResolveTerminalNavigationRequest) ResolveTerminalNavigationResult
```

Метод регистрируется только на private desktop service. Он отсутствует в `PlayerService`, public descriptors, player assets и public HTTP routes.

## Master dialog flow

1. `frontend/src/master.js` получает pending из bootstrap/event и deduplicate-ит dialog по exact `requestId`.
2. Command dialog показывает exact request ID, текущее отображаемое command name, `commandMode` и применимый confirmation text; navigation dialog показывает exact request ID, direction, source terminal, command name и target terminal.
3. Positive action посылает `APPROVE`; negative action, Escape и dialog close посылают `REJECT`.
4. Пока resolve в полёте, buttons disabled; dialog epoch не позволяет stale callback закрыть новый request.
5. Frontend применяет только coordination state с revision новее current; пропущенный event восстанавливается bootstrap-ом.

## Backend resolution

`ResolveTerminalNavigation` проверяет:

- nonblank exact request ID и allowlisted enum decision;
- current broadcast ID и active source terminal;
- для ordinary/initial/completed state-change: frozen exact command identity и соответствующий current/frozen mode context;
- для forward: source command с тем же stable ID всё ещё ссылается на тот же target ID;
- для return: top route point всё ещё равна pending copy;
- latest target terminal существует в trusted session catalog и не равен source.

~~Approve одной coordinator transaction сохраняет source checkpoint, меняет route/active target, очищает pending/notice и публикует одну revision.~~ По BUG-005 approve сначала проверяет exact typed pending, затем применяет ровно один mode-specific effect: ordinary публикует authored result без durable write; initial state-change выполняет persist-once; completed state-change публикует frozen result без повторного execution/write; terminal-transition сохраняет source checkpoint и атомарно меняет route/active target; return атомарно восстанавливает предыдущую route point. Каждый путь очищает exact pending и публикует одну revision; transition/return никогда не создают `PendingTerminalSwitch` и не открывают preserve/discard dialog.

Reject очищает exact pending без mode-specific effect. Для forward terminal-transition он сохраняет на active source runtime существующий `CommandExecutionPresentation{Phase: REJECTED}` для exact source command до controller acknowledgement; terminal-return reject не создаёт command presentation и немедленно восстанавливает current root screen. Stale/duplicate decision возвращает `ok=false` и не меняет current state. Если target/command устарели после создания pending, coordinator очищает pending, сохраняет active/nav/route, ставит typed notice и возвращает safe private error.

## Invalid target notice

`terminal_navigation_notice` используется, когда player выбрал команду с missing/self/stale target или approve обнаружил изменившуюся ссылку. Master frontend отображает localized reason и IDs/name, разрешённые текущей session. Notice не содержит filesystem/storage error и очищается следующим valid transition request, manual switch, end broadcast или shutdown.

## Lifecycle and ordering

- Manual `RequestTerminalActivation`/`RequestTerminalClear` очищает любой command/terminal-navigation pending, notice и route перед своим existing switch lifecycle; поздний dialog callback становится stale.
- `EndBroadcast` и `Shutdown` очищают route/pending/notice вместе с broadcast runtimes. `StartBroadcast` начинает с пустыми значениями.
- Coordinator может синхронно читать detached session target по существующему lock order `control → session`; catalog не вызывает coordinator callbacks.
- Effect publication остаётся detached и revision ordered. Master result/event и player updates относятся к одной committed revision.

## Capability verification

- Generated Wails binding inventory содержит ровно один новый allowlisted method и не содержит generic dispatch.
- Public protobuf descriptor не содержит `PendingTerminalNavigation`, `TerminalNavigationDecision`, `ResolveTerminalNavigationRequest`, notice и private coordination fields.
- Private adapters round-trip exact enum/presence/field numbering; generated files не редактируются вручную.
