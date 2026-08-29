# План реализации: переходы между терминалами

**Feature**: `010-terminal-navigation` | **Date**: 2026-08-18 | **Spec**: [spec.md](./spec.md)

**Bugfix**: 2026-08-19 — BUG-001 Updated from bugfix patch
**Bugfix**: 2026-08-19 — BUG-002 Updated from bugfix patch
**Bugfix**: 2026-08-19 — BUG-003 Updated from bugfix patch
**Bugfix**: 2026-08-19 — BUG-004 Updated from bugfix patch
**Bugfix**: 2026-08-20 — BUG-005 Updated from bugfix patch
**Bugfix**: 2026-08-28 — BUG-006 Updated from bugfix patch
**Bugfix**: 2026-08-28 — BUG-007 Updated from bugfix patch
**Bugfix**: 2026-08-28 — BUG-008 Updated from bugfix patch

## Summary

Функция добавляет к обычной авторской команде необязательную ссылку на другой терминал того же session JSON version 1. Выбор такой команды и возврат из корня создают один server-authoritative approve/reject request; только approve атомарно меняет активный терминал и LIFO-маршрут.

Существующий coordinator остаётся единым владельцем broadcast state, per-terminal checkpoints, revisions, replay protection и stream publication. Переход использует отдельный runtime lifecycle вместо ручного `PendingTerminalSwitch`, поэтому сохраняет исходный checkpoint без второго диалога, повторно проверяет актуальную session перед approve и рассылает всем игрокам одну полную ревизионную проекцию.

По BUG-002 отдельный protocol lifecycle перехода сохраняется, но его ожидающая public projection переиспользует в player UI полноэкранный record-description renderer изменяющей состояние команды: меню скрыто, показан точный текст «Выполняется запрос», а server-authoritative active terminal и маршрут не меняются до решения мастера.

По BUG-003 тот же полноэкранный record-description renderer применяется и к ожидающему возврату: текущий terminal screen и notice overlay скрыты, показан точный текст «Выполняется запрос», а active terminal и верхняя точка LIFO-маршрута остаются неизменными до одобрения мастера.

По BUG-004 `CommandContent.state_change` и `terminal_transition` сохраняют field numbers `2`/`3` и JSON v1 names, но становятся вариантами одного реального protobuf `oneof`. Master editor заменяет два независимых checkbox одним выбором ordinary/state-change/transition, очищает неактивный config и сохраняет completed-state guard.

По BUG-005 каждый выбранный command проходит единый логический master-approval boundary до любого результата или mode-specific эффекта. Ordinary/unset, initial/completed state-change и terminal-transition используют один exact-pending/competing-action invariant и один полноэкранный record-description экран «Выполняется запрос» для controller/observers/reconnect, но после approve сохраняют разную семантику: ordinary только показывает authored result, initial state-change делает один durable write, completed state-change показывает frozen result без write, transition атомарно меняет terminal/route. Это правило явно supersede-ит approval-free ordinary/completed-repeat ветки feature 009 без добавления обязательного config к ordinary-командам.

По BUG-006 ordinary reject/close больше не очищает command-execution presentation с немедленным возвратом в source menu. Coordinator публикует существующую фазу `REJECTED`, а player controller, observers и reconnect используют тот же полноэкранный record-description экран «Ошибка доступа», что initial state-changing rejection. Только `Back` или `Enter` controller очищает presentation и синхронно возвращает всех в неизменённое меню; authored result, durable state, terminal/route и notification-specific player behavior не добавляются. ~~Completed state-changing,~~ ~~terminal-transition~~ и return rejection semantics не меняются; BUG-007 supersedes forward terminal-transition rejection, а BUG-008 supersedes completed state-changing rejection.

По BUG-007 forward terminal-transition reject/close сохраняет отдельный `PendingTerminalNavigation` lifecycle до решения, но после reject публикует для exact source command существующий detached `CommandExecutionPresentation{Phase: REJECTED}`. Controller, observers и reconnect видят общий полноэкранный экран «Ошибка доступа» до `Back`/`Enter` controller, затем синхронно возвращаются в неизменённое source menu без activation/route/checkpoint/persistence effect. Новые protobuf fields и notification-specific player state не требуются; ~~completed state-changing и~~ terminal-return rejection semantics не меняются. BUG-008 supersedes только completed state-changing rejection.

По BUG-008 repeated completed state-changing reject/close больше не очищает command-execution presentation с немедленным возвратом в source menu. Coordinator публикует для exact completed command существующую фазу `REJECTED`, а controller, observers и reconnect видят общий полноэкранный экран «Ошибка доступа» без frozen result до `Back`/`Enter` controller. Acknowledgement возвращает неизменённое source menu без повторного execution, durable write, command-state, terminal/route или notification-specific effect; completed approve и terminal-return reject не меняются, новые protobuf fields не требуются.

## Project Structure

```text
specs/010-terminal-navigation/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
└── contracts/
    ├── session-v1.md
    ├── public-player.md
    └── private-desktop.md

proto/fallout/terminal/
├── persistence/v1/session.proto       # authored targetTerminalId
├── player/v1/terminal.proto           # route/pending player projection
└── private/v1/
    ├── coordination.proto             # exact master-only pending request
    └── desktop.proto                  # approve/reject request and result

internal/
├── domain/{model.go,json.go,validate.go} # durable and runtime aggregates
├── session/{contract.go,service.go}      # v1 adapter and trusted terminal catalog
├── nav/nav.go                           # moved-folder and ancestor fallback
├── live/service.go                      # checkpoint activation with explicit nav placement
├── control/service.go                   # pending request, LIFO route, atomic resolution
├── player/adapter.go                    # public protobuf boundary
└── gen/fallout/terminal/{persistence,player,private}/v1/
                                                    # generated Go only

main.go                                             # session-catalog composition
app.go                                              # private resolve operation and lifecycle clearing
app_contract.go                                     # explicit private protobuf adapters
desktop_service.go                                  # narrow Wails method

frontend/src/{master.js,master.css,desktop-api.js}  # authoring and master decision dialog
frontend/bindings/                                  # Wails-generated bindings only
client/{client.js,client.css}                       # pending lock and root return control
client/gen/fallout/terminal/player/v1/              # generated ECMAScript only

tests/browser/
├── terminal-navigation.spec.mjs
├── fixture-server/main.go
└── fixtures/desktop-bindings.js
```

**Structure Decision**: Долговечная ссылка остаётся в `domain`/`session`, маршрут и checkpoint-ы — в `control`/`live`, а public/private браузеры получают только свои типизированные protobuf-проекции через существующие узкие границы.

## Constitution Check

| Principle | До исследования | После дизайна | Обоснование |
|---|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | PASS | Wails остаётся в root/private bridge; `domain`, `session`, `nav`, `live`, `control` и `player` не зависят от desktop runtime. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | PASS | Все known persistence, public player и private desktop structures зафиксированы в versioned protobuf до реализации и имеют явные адаптеры. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | PASS | Контроллер по-прежнему отправляет unary `PlayerService.Navigate`; coordinator один меняет route/checkpoints и рассылает revisioned stream updates. |
| IV. Separate Public and Private Capabilities | PASS | PASS | Public projection содержит route depth/top target и secret-free pending; decision ID, full prompt, notice и approve/reject остаются в private desktop service. |
| V. Evolve Schemas Safely and Reproducibly | PASS | PASS | По BUG-004 существующие поля `state_change = 2` и `terminal_transition = 3` объединяются в один real oneof без перенумерации и изменения JSON names; generated API delta проходит explicit review, pinned generation и protobuf breaking checks. |
| VI. Preserve Portable Session JSON Version 1 | PASS | PASS | JSON `version` остаётся `1`; optional `terminalTransition` имеет legacy default, unknown fields round-trip, runtime route/hack state в файл не попадают. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | PASS | Новый transport или dual runtime не вводится; existing Navigate/Subscribe/coordination paths расширяются целиком, а manual switch остаётся отдельной действующей семантикой. |

Нарушений конституции и оснований для Complexity Tracking нет.

## Implementation Strategy

### Phase 1 — Схемы и чистые доменные правила

1. ~~Добавить только additive persistence/public/private protobuf messages и field numbers из [contracts/](./contracts/).~~ Зафиксировать versioned persistence/public/private protobuf contracts и field numbers; по BUG-004 объединить существующие `CommandContent` fields в shared oneof, затем регенерировать Go/ECMAScript artifacts и schema revision только штатными scripts.
2. Расширить domain models, deep clones, JSON known fields и двухпроходную session validation.
3. Добавить в `internal/nav` pure helper, который находит folder по stable ID во всём дереве, восстанавливает current ancestry и применяет nearest-ancestor/root fallback.

По BUG-004 persistence schema MUST объединить существующие поля `CommandContent.state_change = 2` и `terminal_transition = 3` в один `oneof behavior`, оставляя unset-вариант обычной командой; explicit adapters и domain mapping MUST разбирать ровно один вариант, сохранять JSON v1 shape и продолжать отклонять malformed legacy JSON с обоими config.

### Phase 2 — Authoring и trusted catalog

1. Обновить explicit session protobuf adapter и clone/round-trip tests, не меняя atomic storage и revision pipeline.
2. ~~Добавить в master command editor target toggle/select и отдельный `stateChange` toggle с взаимным снятием checkbox.~~ По BUG-004 заменить их одним command-mode selector с вариантами ordinary/state-change/terminal-transition; при смене режима скрывать и удалять inactive config, сохраняя target select, local validation, completed-state guard и inbound-reference guard при удалении терминала.
3. Скомпоновать узкий `TerminalCatalog`, который строит detached `TerminalTarget` только из current validated session snapshot.

### Phase 3 — Атомарный coordinator lifecycle

1. Добавить route, pending и notice в broadcast/process aggregate и все clone/snapshot paths.
2. ~~Перехватывать linked `NavigateCommand` и root `NavigateBack` после authorization/replay checks, но до ordinary live action. Создавать только один pending и блокировать competing gameplay.~~ По BUG-005 перехватывать каждый валидный `NavigateCommand` после authorization/replay и немутирующих mode-specific preflight checks, но до ordinary result, state-change execution/replay или terminal transition; invalid/stale/missing/self-target сохраняет существующий safe rejection до pending, а root `NavigateBack` — существующий return request. Создавать не более одного pending среди всех типизированных command/transition/return вариантов, блокировать competing gameplay и только после exact approve направлять frozen command identity в соответствующий mode-specific effect.
3. Реализовать exact private resolve: re-read source/target, approve как single commit с checkpoint preservation и explicit nav placement; reject/stale без route/active mutation.
4. Очищать route/pending/notice при manual activation/clear, end broadcast и shutdown. Не изменять existing manual unfinished-hack switch semantics.

Ordinary и completed state-changing requests не требуют нового authoring config: private projection показывает exact request/command identity и mode, а существующие state-change/transition поля добавляются только когда применимы. Resolve ordinary публикует authored result без session write; resolve completed публикует frozen snapshot без нового execution/write; reject/close любого command request не применяет mode-specific effect. ~~Post-decision presentation сохраняется по режиму: state-change «Ошибка доступа» до acknowledgement, transition source menu, ordinary прежнее menu без результата.~~ ~~По BUG-006 ordinary reject/close, как и initial state-change, сохраняет `CommandExecutionPresentation{Phase: REJECTED}` до controller acknowledgement; completed state-change и transition сохраняют прежнее поведение.~~ ~~По BUG-007 ordinary, initial state-change и forward terminal-transition reject/close сохраняют для exact command `CommandExecutionPresentation{Phase: REJECTED}` до controller acknowledgement; completed state-change сохраняет прежнее поведение, а terminal-return rejection остаётся в отдельном navigation lifecycle с immediate current-root restoration.~~ По BUG-008 ordinary, initial/completed state-change и forward terminal-transition reject/close сохраняют для exact command `CommandExecutionPresentation{Phase: REJECTED}` до controller acknowledgement; только terminal-return rejection остаётся в отдельном navigation lifecycle с immediate current-root restoration.

### Phase 4 — Границы и UI

1. Обновить public/private adapters, App result, Wails method, desktop API normalization, generated bindings и exact allowlist/descriptor tests.
2. Добавить master dialog с direction/source/command/target, request-ID deduplication, close-as-reject и stale callback guards; typed notice показывать отдельно. По BUG-005 тот же private master surface MUST принимать ordinary и completed command requests, всегда показывать exact command name/mode и не требовать отсутствующий `confirmationText` или transition target.
3. Добавить player route/pending mapping, root return control и input lock, сохранив current unary+stream acknowledgement discipline и observer read-only behavior. По BUG-002 прямой pending MUST скрывать меню и переиспользовать общий полноэкранный record-description renderer с точным текстом «Выполняется запрос», не объединяя `TerminalNavigationPresentation` с `CommandExecutionPresentation` и не выполняя optimistic terminal switch. По BUG-003 return pending MUST скрывать текущий terminal screen и notice overlay и использовать тот же renderer и точный текст без преждевременного pop маршрута. По BUG-005 любой command pending независимо от ordinary/state-change/transition mode MUST сходиться на той же поверхности и блокировке; protocol projections MAY оставаться типизированными, но UI и server action semantics образуют один approval contract. По BUG-006 ordinary `REJECTED` MUST проходить через существующий command record renderer с точным текстом «Ошибка доступа», скрывать menu/result, оставлять observers read-only и принимать `Back`/`Enter` только от controller как общий acknowledgement. По BUG-007 forward terminal-transition reject/close MUST публиковать ту же detached `CommandExecutionPresentation{Phase: REJECTED}` и переиспользовать renderer/reconnect/acknowledgement без изменения pending protocol; return reject не публикует command rejection. По BUG-008 completed state-changing reject/close MUST публиковать ту же exact `REJECTED` projection и переиспользовать renderer/reconnect/acknowledgement, скрывая frozen result и не меняя completed command state.

### Phase 5 — Верификация и cutover

1. Покрыть legacy/new JSON, cross-reference validation, moved/deleted folder restoration, coordinator atomicity/replay/authority, hack continuity, stream reconnect и private capability separation; по BUG-004 добавить descriptor/generated-API и adapter coverage общего `CommandContent` oneof, unset ordinary и malformed dual-config input.
2. Добавить Playwright journey master + controller + observers для approve/reject, LIFO/cycle, pending reconnect, stale target и moved/deleted return location. Для BUG-002 отдельно сравнить полноэкранную поверхность прямого pending с renderer ожидающей state-changing команды, проверить точный текст, отсутствие меню, блокировку `Back`/`Enter` и восстановление того же экрана после reconnect. Для BUG-003 применить ту же матрицу к return pending и дополнительно доказать неизменность route top до approve и восстановление текущего root screen после reject/close. Для BUG-004 проверить single-choice command mode, переключение всех трёх вариантов, очистку inactive config, completed-state guard и save/reopen. Для BUG-005 пройти ordinary, initial/completed state-change и terminal-transition через один pending-screen/request invariant, approve/reject/close, 20 replayed selections, controller + two observers и reconnect; до approve не допускаются result/write/terminal/route effects, а после approve проверяется ровно один mode-specific outcome. Для BUG-006 проверить explicit reject, close-as-reject и notification reject ordinary-команды у controller + two observers + reconnect: все видят точное «Ошибка доступа» без menu/result до controller acknowledgement, затем синхронно возвращаются в неизменённое source menu с нулевым эффектом. Для BUG-007 применить ту же reject/close/notification и `Back`/`Enter` матрицу к forward terminal-transition, дополнительно доказав отсутствие target activation, route push и checkpoint mutation и сохранив terminal-return reject baseline. Для BUG-008 применить ту же матрицу к repeated completed state-changing command, скрыть frozen result до acknowledgement и доказать отсутствие повторного execution/write или command-state mutation.
3. Пройти protobuf format/lint/generation/breaking, Wails binding inventory, Go race, frontend/client builds и browser suite; generated drift и временные protocol paths не оставлять. Изменение generated API oneof по BUG-004 MUST быть явно reviewed при сохранённых wire field numbers и JSON names.

## Verification Strategy

| Surface | Ключевые проверки |
|---|---|
| Session/domain | Legacy absence, new config round-trip, unknown fields, forward-reference order, missing/self target, state-change conflict, inbound delete guard; по BUG-004 — общий descriptor oneof, unset ordinary, ровно один mapped variant, сохранённые field numbers/JSON names и malformed dual-config rejection. |
| Navigation/live | Rename/move/delete folder, nearest parent/root fallback, destination root placement, solved/unfinished/failed hack retention, explicit reset/discard. |
| Coordinator | Approve/reject/stale, exact-one pending across ordinary/initial-completed state-change/transition/return, 20 replays, concurrent distinct IDs, отсутствие result/write/terminal/route effect до approve, mode-specific single effect после approve, LIFO A→B→C, cycle A→B→A, pop-after-approve, manual/end/shutdown clearing; по BUG-006 ordinary reject/close очищает pending, публикует `REJECTED` для exact command, сохраняет source nav и даёт нулевой effect; по BUG-007 forward transition reject/close делает то же без activation/route/checkpoint effect, сохраняя immediate-return behavior для return reject; по BUG-008 completed state-change reject/close также публикует exact `REJECTED` без frozen result, repeat execution/write или command-state mutation. |
| Player stream | Controller-only mutation, observers read-only, monotonic complete update, pending snapshot/reconnect с тем же полноэкранным экраном «Выполняется запрос» для каждого command mode, forward и return, active-terminal convergence без optimistic result/switch или преждевременного pop маршрута, overflow resubscribe; по BUG-006 ordinary `REJECTED` сходится у controller/observers/reconnect и очищается только controller acknowledgement; по BUG-007 та же projection/acknowledgement гарантия применяется к rejected forward transition; по BUG-008 — к rejected completed state-changing command без frozen result leakage. |
| Private desktop | Exact request/command identity и mode-specific prompt fields без обязательного config для ordinary/completed, dialog dedup/close/stale callback, typed notice, allowlisted Wails methods, no public decision capability. |
| Browser | Master, controller and at least two observers; ordinary/initial-completed state-change/transition approve/reject/close, return, nested source folder, stale target, reconnect while pending, retained hacking; для каждого command mode, прямого и return pending — общий record-description renderer, точный текст «Выполняется запрос», скрытый текущий экран и заблокированные `Back`/`Enter`; ~~reject/close даёт существующую mode-specific rejection/acknowledgement presentation без эффекта~~ по BUG-006 ordinary reject/close показывает всем «Ошибка доступа» без menu/result до `Back`/`Enter` controller и затем возвращает неизменённое source menu; по BUG-007 forward transition reject/close использует тот же access-error flow без activation/route/checkpoint effect; по BUG-008 completed state-change reject/close использует тот же flow без frozen result или repeat execution/write, а return reject сохраняет current-root restoration; approve даёт ровно один mode-specific outcome; по BUG-004 — один ordinary/state-change/transition selector, inactive config cleanup и save/reopen каждого режима. |
| Master/native persistence | Реальный `sessions/demo.json`: полный terminal set до и после Wails `SaveSession`, успешная durable revision для `t_demo1` → `t_demo2`, reopen с сохранённой целью; missing/self target по-прежнему отклоняются; по BUG-004 каждый command сохраняет не более одного config, а ordinary command — ни одного. |

Applicable commands:

```bash
make proto-generate
make proto-check proto-breaking bindings-check
go test ./internal/domain ./internal/session ./internal/nav
go test -race ./internal/control ./internal/live ./internal/player
go test ./...
npm ci --prefix frontend && npm run build --prefix frontend
npm ci --prefix client && npm run build --prefix client
npm ci --prefix tests/browser
npm test --prefix tests/browser -- terminal-navigation.spec.mjs
make check
```

`make check` не заменяет Playwright journey. Packaging и interactive `go run ./cmd/build dev` проводятся перед acceptance, поскольку изменяются embedded frontend/client assets и Wails bindings.
