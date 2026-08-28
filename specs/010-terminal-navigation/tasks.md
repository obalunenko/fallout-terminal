# Задачи: переходы между терминалами

**Bugfix**: 2026-08-19 — BUG-001 Updated from bugfix patch
**Bugfix**: 2026-08-19 — BUG-002 Updated from bugfix patch
**Bugfix**: 2026-08-19 — BUG-003 Updated from bugfix patch
**Bugfix**: 2026-08-19 — BUG-004 Updated from bugfix patch
**Bugfix**: 2026-08-20 — BUG-005 Updated from bugfix patch; ~~T031/T036 packaged acceptance verification remains pending without the recorded controller + two-observer approve/reject/close evidence.~~ ~~T031/T036 are complete with accepted packaged controller + two-observer approve/reject/close evidence recorded in `.spec-context.json`.~~ The BUG-005 packaged evidence remains accepted, but T036 was reopened by BUG-006/BUG-007 and remains pending only for direct packaged macOS Notification Center rejection-action evidence.
**Bugfix**: 2026-08-28 — BUG-006 Updated from bugfix patch; ordinary reject/close MUST publish «Ошибка доступа» until controller acknowledgement.
**Bugfix**: 2026-08-28 — BUG-007 Updated from bugfix patch; forward terminal-transition reject/close MUST publish «Ошибка доступа» until controller acknowledgement.

**Input**: design artifacts from `specs/010-terminal-navigation/`

**Prerequisites**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/session-v1.md`, `contracts/public-player.md`, `contracts/private-desktop.md`

**Testing**: Изменение затрагивает versioned protobuf, portable session JSON, конкурентный coordinator, private Wails bridge и два браузерных UI. Для каждой пользовательской истории сначала добавляются падающие focused Go/Playwright tests; финальная фаза выполняет schema, binding, race, build и browser gates из плана.

**Organization**: Задачи сгруппированы по приоритетным пользовательским историям. `[P]` означает независимую задачу текущей волны в других файлах; `[US#]` связывает работу с историей из `spec.md`.

## Phase 1: Setup — protobuf contracts and generated baseline

**Purpose**: Зафиксировать совместимые persistent, public и private контракты до изменения их producers/consumers.

**Wave 1 — independent (different files):**

- [x] **T001** [P] ⚠️ Reopened — ~~добавить `terminal_transition` как independent optional field рядом со `state_change`~~; по BUG-004 объединить `CommandContent.state_change = 2` и `terminal_transition = 3` в один реальный `oneof behavior`, оставить unset ordinary mode и сохранить JSON version 1/known names `(reopened — BUG-004)` · `proto/fallout/terminal/persistence/v1/session.proto`
- [x] **T002** [P] Добавить public direction/route/pending presentation и optional field 9 без новой player RPC · `proto/fallout/terminal/player/v1/terminal.proto`
- [x] **T003** [P] Добавить private pending/notice/decision contracts и exact resolve request/result с `UNSPECIFIED = 0` · `proto/fallout/terminal/private/v1/coordination.proto`, `proto/fallout/terminal/private/v1/desktop.proto`

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T004** ⚠️ Reopened — регенерировать pinned Go/ECMAScript protobuf outputs и reviewed schema revision штатным `make proto-generate` без ручного редактирования generated code; по BUG-004 подтвердить shared `CommandContent` oneof generated API и сохранённые field numbers/JSON names `(reopened — BUG-004)` · `internal/gen/fallout/terminal/persistence/v1/session.pb.go`, `internal/gen/fallout/terminal/player/v1/terminal.pb.go`, `internal/gen/fallout/terminal/private/v1/coordination.pb.go`, `internal/gen/fallout/terminal/private/v1/desktop.pb.go`, `client/gen/fallout/terminal/player/v1/terminal_pb.js`, `proto/schema-revision.txt`

---

## Phase 2: Foundational — durable models, trusted lookup, restoration, and adapters

**Purpose**: Построить общие модели, compatible persistence и detached boundary projections, блокирующие все пользовательские истории.

### Tests

**Wave 1 — independent (different files), write these tests to fail first:**

- [x] **T005** [P] ⚠️ Reopened — покрыть deep clone, known/unknown JSON fields, two-pass validation, forward references, missing/self target и конфликт со `stateChange`; по BUG-004 проверить ordinary/state-change/transition discriminated behavior и malformed dual-config JSON rejection `(reopened — BUG-004)` · `internal/domain/model_test.go`, `internal/domain/validate_test.go`
- [x] **T006** [P] ⚠️ Reopened — покрыть legacy/new v1 round-trip, terminal-order independence, atomic invalid-save rejection и detached trusted catalog lookup; по BUG-004 проверить shared-oneof adapter round-trip каждого варианта и сохранение field numbers/JSON names `(reopened — BUG-004)` · `internal/session/contract_test.go`, `internal/session/service_test.go`
- [x] **T007** [P] Покрыть поиск folder по stable ID, восстановление новой ancestry и nearest-ancestor/root fallback после move/delete · `internal/nav/nav_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T008** ⚠️ Reopened — добавить durable transition config, broadcast route/pending/notice/presentation aggregates, deep clones, JSON known fields и two-pass cross-terminal validation; по BUG-004 выразить command behavior как один discriminated ordinary/state-change/transition variant и сохранить defensive dual-config JSON/import rejection `(reopened — BUG-004)` · `internal/domain/model.go`, `internal/domain/json.go`, `internal/domain/validate.go`

**⟶ Wait for T008 to finish, then Wave 3 — independent (different files):**

- [x] **T009** [P] ⚠️ Reopened — реализовать explicit protobuf mapping и current-session `TerminalCatalog`, возвращающий detached target/command snapshot без изменения storage/revision pipeline; по BUG-004 map ровно один generated `CommandContent.behavior` variant либо unset ordinary и отклонять невозможное/невалидное состояние на boundary `(reopened — BUG-004)` · `internal/session/contract.go`, `internal/session/service.go`
- [x] **T010** [P] Реализовать pure stable-folder lookup и детерминированное восстановление current ancestry с parent/root fallback · `internal/nav/nav.go`

**⟶ Wait for Wave 3 to finish, then:**

- [x] **T011** Подключить public/private protobuf projections, clone-safe adapters и catalog seam в composition, не раскрывая decision ID/full route публичному клиенту · `internal/player/adapter.go`, `app_contract.go`, `internal/control/service.go`, `main.go`

**Checkpoint**: Version-1 link, runtime navigation types, folder restoration, trusted lookup and typed boundary projections are independently testable foundations.

---

## Phase 3: User Story 1 — перейти по команде в другой терминал (Priority: P1) 🎯 MVP

**Goal**: Контролирующий игрок выбирает authored link, мастер видит один exact approve/reject dialog, а approve атомарно активирует target root без второго switch dialog.

**Independent Test**: Настроить A → B, сохранить/переоткрыть session, выбрать link контроллером и проверить pending, approve, reject, stale target, replay и блокировку конкурирующих действий.

### Tests

**Wave 1 — independent (different files), write these tests to fail first:**

- [x] **T012** [P] [US1] ⚠️ Reopened — Добавить coordinator tests для controller-only linked command, exact-one pending, 20 replayed requests, competing-action conflict, approve/reject, stale/missing/self target и atomic route/active revision; по BUG-005 расширить матрицу до ordinary, initial/completed state-change и terminal-transition command, запретив result/write/terminal/route effect до exact approve `(reopened — BUG-005)`; по BUG-007 forward transition reject/close MUST очистить pending, сохранить exact `REJECTED` presentation и source nav/route/checkpoints до acknowledgement `(reopened — BUG-007)` · `internal/control/service_test.go`
- [x] **T013** [P] [US1] ⚠️ Reopened — добавить Playwright tests для mutually-exclusive authoring, completed-state guard, save/reopen, inbound-delete guard и master forward decision dialog; согласовать fixture round-trip с полной session, не теряющей второй terminal `(reopened — BUG-001)`; по BUG-004 заменить checkbox assertions на один ordinary/state-change/transition selector и проверить отсутствие inactive config после каждого switch/save/reopen `(reopened — BUG-004)` · `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/fixtures/desktop-bindings.js`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent (different files):**

- [x] **T014** [P] [US1] ⚠️ Reopened — Перехватывать linked `NavigateCommand` после authority/replay checks, создавать один `PendingTerminalNavigation`, блокировать gameplay и атомарно approve/reject со свежим catalog lookup; по BUG-005 перенести interception перед dispatch любого command mode, обеспечить один общий pending invariant и выполнять ordinary/state-change/transition effect только после exact approve `(reopened — BUG-005)`; по BUG-007 forward transition reject/close MUST после очистки navigation pending сохранить на source runtime exact `CommandExecutionPresentation{Phase: REJECTED}` без activation/route/checkpoint/persistence effect, не меняя return reject `(reopened — BUG-007)` · `internal/control/service.go`
- [x] **T015** [P] [US1] ⚠️ Reopened — ~~использовать два command-editor checkbox toggle с программным mutual exclusion~~; по BUG-004 добавить один ordinary/state-change/terminal-transition mode selector, очищать inactive config, сохранять target select, local validation, completed-state guard, inbound-reference delete guard и deduplicated master dialog `(reopened — BUG-004)` · `frontend/src/master.js`, `frontend/src/master.css`, `frontend/src/desktop-api.js`
- [x] **T016** [P] [US1] [US3] ⚠️ Reopened — отображать authoritative pending status и делать shared controls inert без optimistic terminal switch; по BUG-002 для прямого pending скрывать меню и переиспользовать полноэкранный record-description renderer с точным текстом «Выполняется запрос», не объединяя отдельные protocol lifecycles; по BUG-003 применять тот же renderer к return pending, скрывать текущий экран и notice overlay и не выполнять преждевременный pop маршрута; по BUG-005 применять этот renderer и server-authoritative input lock к ordinary, initial/completed state-change и terminal-transition pending без раннего result `(reopened — BUG-002)` `(reopened — BUG-003)` `(reopened — BUG-005)`; по BUG-007 подтвердить, что existing rejected-command renderer/reconnect/`Back`/`Enter` acknowledgement принимает detached forward-transition `REJECTED` без нового player state и не затрагивает return reject `(reopened — BUG-007)` · `client/client.js`, `client/client.css`

**⟶ Wait for Wave 2 to finish, then:**

- [x] **T017** [US1] Провести exact resolve через App и единственный private desktop method, валидировать enum/request ID, публиковать только newer coordination state и очищать pending/notice по contract · `app.go`, `desktop_service.go`, `app_contract.go`

**⟶ Wait for T017 to finish, then:**

- [x] **T018** [US1] ⚠️ Reopened — Регенерировать allowlisted Wails binding и завершить browser fixture/server wiring для полного authored-link → pending → approve/reject journey; по BUG-005 fixtures/private wiring MUST переносить exact command identity и mode-specific context для ordinary, initial/completed state-change и transition requests без обязательного ordinary config `(reopened — BUG-005)` · `frontend/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js`, `frontend/bindings/github.com/obalunenko/Fallout-Terminal/models.js`, `tests/browser/fixture-server/main.go`, `tests/browser/fixtures/desktop-bindings.js`

**Checkpoint**: User Story 1 independently persists and executes one master-approved forward transition, while rejection and invalid/stale links preserve the source state.

---

## Phase 4: User Story 2 — не взламывать повторно уже открытый терминал (Priority: P1)

**Goal**: Broadcast-scoped terminal checkpoints retain solved, unfinished and failed hack state; forward approve places the destination at root without recreating its hack runtime.

**Independent Test**: Впервые открыть защищённый B, решить или частично пройти hack, уйти и вернуться не менее 10 раз; убедиться в сохранении состояния и в отсутствии второго preserve/discard dialog.

### Tests

**Wave 1 — independent (different files), write these tests to fail first:**

- [x] **T019** [P] [US2] Добавить tests для first-entry hack, solved/unfinished/failed reactivation, explicit reset/discard, root placement и single-decision source checkpoint preservation · `internal/live/service_test.go`, `internal/control/service_test.go`
- [x] **T020** [P] [US2] Расширить browser journey проверками первого взлома, повторного входа без взлома и retained blocked/progress state · `tests/browser/terminal-navigation.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T021** [US2] Переиспользовать `TerminalRuntimes`/`SuspendRuntime`/`ReactivateRuntime`, сохранять private hack checkpoint и задавать destination root при approved transition без `PendingTerminalSwitch` · `internal/live/service.go`, `internal/control/service.go`

**Checkpoint**: User Story 2 independently preserves hack continuity for every revisited terminal within the current broadcast.

---

## Phase 5: User Story 3 — вернуться в предыдущий терминал (Priority: P1)

**Goal**: Root back creates one return approval; approve pops exactly one LIFO point and restores the source folder, its moved location, nearest surviving ancestor or root.

**Independent Test**: Пройти A/nested → B → C, отклонить и затем одобрить возвраты; проверить B затем A, точный saved folder, moved-folder recovery и delete fallback.

### Tests

**Wave 1 — independent (different files), write these tests to fail first:**

- [x] **T022** [P] [US3] Добавить coordinator/nav tests для root-only return, unchanged-top validation, reject/close immutability, LIFO/cycles, pop-after-approve и moved/deleted folder restoration · `internal/control/service_test.go`, `internal/nav/nav_test.go`
- [x] **T023** [P] [US3] ⚠️ Reopened — расширить Playwright journey root return control, pending lock, approve/reject, nested restore, A → B → C unwind и no-route absence; по BUG-003 во время return pending требовать общий полноэкранный record-description screen с точным текстом «Выполняется запрос», скрытым текущим экраном и восстановлением root screen/return control после reject/close `(reopened — BUG-003)` · `tests/browser/terminal-navigation.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent (different files):**

- [x] **T024** [P] [US3] Интерпретировать root `NavigateBack` как return request, хранить immutable top copy и при approve restore folder/fallback и pop ровно одну route point · `internal/control/service.go`
- [x] **T025** [P] [US3] Показывать authoritative return target только в root list и отправлять existing `NavigateBack`, сохраняя прежний intra-terminal back · `client/client.js`, `client/client.css`

**Checkpoint**: User Story 3 independently unwinds direct transitions one approved LIFO step at a time and restores the intended source menu.

---

## Phase 6: User Story 4 — сохранить общий и устойчивый маршрут (Priority: P2)

**Goal**: Все игроки и переподключившиеся клиенты сходятся к одной revisioned projection; observer, stale edits and lifecycle resets cannot move to a wrong terminal or retain an old route.

**Independent Test**: Использовать controller и двух observers, reconnect во время pending, изменить/delete source/target до approve/return, затем проверить monotonic convergence и empty route после manual switch/end/shutdown/new broadcast.

### Tests

**Wave 1 — independent (different files), write these tests to fail first:**

- [x] **T026** [P] [US4] ⚠️ Reopened — Добавить authority, concurrent request, snapshot/reconnect, monotonic stream, stale edit, manual/end/shutdown clearing и public/private capability-separation tests; по BUG-005 доказать один pending across command modes, private-only decision и отсутствие mode-specific эффекта до approve `(reopened — BUG-005)` · `internal/control/service_test.go`, `internal/player/adapter_test.go`, `internal/player/public_stream_test.go`, `app_test.go`, `app_contract_test.go`, `wails_host_test.go`
- [x] **T027** [P] [US4] ⚠️ Reopened — расширить journey controller + two observers проверками pending reconnect, convergence ≤2s, stale target safe failure, moved/deleted return path и new-broadcast cleanup; по BUG-002 доказать для прямого pending одинаковый полноэкранный record-description screen с точным текстом «Выполняется запрос», скрытым меню и заблокированными `Back`/`Enter` до решения; по BUG-003 доказать ту же поверхность и блокировку для return pending у controller/observers/reconnect; по BUG-005 распространить тот же экран, блокировку и reconnect convergence на ordinary, initial/completed state-change и terminal-transition commands `(reopened — BUG-002)` `(reopened — BUG-003)` `(reopened — BUG-005)`; по BUG-007 доказать rejected forward transition convergence у controller/two observers/reconnect до controller acknowledgement `(reopened — BUG-007)` · `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/fixture-server/main.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T028** [US4] ⚠️ Reopened — Завершить single-revision public/private publication, reconnect projection, typed master notice и route/pending/notice clearing при manual activation/clear, end broadcast и shutdown; по BUG-005 публиковать exact generic command identity/mode в private coordination и единый secret-free waiting state публично для каждого command mode `(reopened — BUG-005)` · `internal/control/service.go`, `internal/player/adapter.go`, `app.go`, `frontend/src/master.js`, `client/client.js`

**Checkpoint**: User Story 4 independently proves authoritative multi-client convergence, safe stale handling and broadcast-scoped cleanup.

---

## Final Phase: Polish and Success-Criteria validation

**Purpose**: Проверить совместимость, отсутствие generated drift/private leakage и все измеримые outcomes одной полной реализацией.

**Wave 1:**

- [x] **T029** ⚠️ Reopened — проверить protobuf format/lint/generation/breaking и exact Wails allowlist командами `make proto-check proto-breaking bindings-check`; по BUG-004 проверить descriptor с одним real oneof, field numbers `2`/`3`, reviewed generated-API delta и отсутствие drift/public leakage `(reopened — BUG-004)` · `proto/`, `internal/gen/`, `client/gen/`, `frontend/bindings/`, `app_contract.go`

**⟶ Wait for T029 to finish, then:**

- [x] **T030** ⚠️ Reopened — выполнить единственную полную Success Criteria validation: `go test ./internal/domain ./internal/session ./internal/nav`, `go test -race ./internal/control ./internal/live ./internal/player`, `go test ./...`, frontend/client builds, focused `terminal-navigation.spec.mjs`, затем `make check`, включая SC-009 `(reopened — BUG-001)`, SC-010 `(reopened — BUG-002)`, SC-011 `(reopened — BUG-003)`, SC-012 shared oneof/single-choice authoring `(reopened — BUG-004)`, SC-013–SC-014 universal command approval `(reopened — BUG-005)`, SC-015 ordinary rejection acknowledgement `(reopened — BUG-006)` и SC-016 forward-transition rejection acknowledgement `(reopened — BUG-007)` · `specs/010-terminal-navigation/spec.md`, `Makefile`, `frontend/package.json`, `client/package.json`, `tests/browser/package.json`

**⟶ Wait for T030 to finish, then:**

- [x] **T031** ⚠️ Reopened — собрать accepted Wails runtime и пройти native master + controller + two observers smoke для approve/reject, 10 revisits, three-terminal unwind, reconnect and shutdown cleanup; дополнительно открыть реальный `sessions/demo.json`, применить `t_demo1` → `t_demo2`, дождаться durable revision и проверить цель после reopen; по BUG-002 доказать полноэкранный прямой pending «Выполняется запрос» без одновременно видимого меню у controller/observers/reconnect; по BUG-003 доказать ту же полноэкранную поверхность для return pending и восстановление текущего root screen после reject/close; по BUG-004 пройти native ordinary/state-change/transition selector, switch cleanup и save/reopen ровно одного config; по BUG-005 выбрать в native journey ordinary, initial/completed state-change и transition command, получить exact master request и один общий waiting screen до единственного mode-specific approve effect, отдельно проверить reject/close без эффекта; по BUG-006 отклонить ordinary-команду explicit reject и close-as-reject, проверить «Ошибка доступа» у controller/two observers/reconnect до `Back`/`Enter` acknowledgement и неизменённое source menu после него; по BUG-007 повторить explicit reject/close для forward transition, проверить тот же экран и acknowledgement без target activation/route/checkpoint effect и неизменный return-reject baseline; зафиксировать любой недоступный manual gate без ложного PASS `(reopened — BUG-001)` `(reopened — BUG-002)` `(reopened — BUG-003)` `(reopened — BUG-004)` `(reopened — BUG-005)` `(reopened — BUG-006)` `(reopened — BUG-007)` ~~`(verification pending — BUG-005: recorded native evidence does not satisfy the complete two-observer approve/reject/close matrix)`~~ `(verified — BUG-005: accepted packaged controller + two-observer approve/reject/close evidence is recorded in .spec-context.json)` · `main.go`, `app.go`, `wails.json`, `sessions/demo.json`, `specs/010-terminal-navigation/spec.md`

---

## Dependencies & Execution Order

- Phase order: Setup → Foundational → US1 (MVP) → US2 → US3 → US4 → Polish. US2 и US3 используют forward lifecycle US1; US4 проверяет и завершает общий маршрут после всех P1 slices.
- Phase 1: Wave 1 (`T001–T003`) → `T004`.
- Phase 2: test Wave 1 (`T005–T007`) → `T008` → implementation Wave 3 (`T009–T010`) → `T011`.
- Phase 3: test Wave 1 (`T012–T013`) → implementation Wave 2 (`T014–T016`) → `T017` → `T018`.
- Phase 4: test Wave 1 (`T019–T020`) → `T021`.
- Phase 5: test Wave 1 (`T022–T023`) → implementation Wave 2 (`T024–T025`).
- Phase 6: test Wave 1 (`T026–T027`) → `T028`.
- Polish: `T029` → `T030` → `T031`.
- BUG-001 corrective DAG overrides the earlier completion state: `T034` failing regression → `T035` correction → reopened `T013` and `T033` → reopened `T030` → reopened `T031` → `T036` accepted packaged-runtime gate.
- BUG-002 corrective DAG overrides the earlier completion state: `T037` failing presentation regression → reopened `T016` correction → reopened `T027` multi-player/reconnect proof → reopened `T030` full gate → reopened `T031` native smoke → reopened `T036` accepted packaged-runtime gate.
- BUG-003 corrective DAG overrides the earlier completion state: `T038` failing return-presentation regression → reopened `T016` correction → reopened `T023` return journey and `T027` multi-player/reconnect proof → reopened `T030` full gate → reopened `T031` native smoke → reopened `T036` accepted packaged-runtime gate.
- BUG-004 corrective DAG overrides phase ordering for its reopened tasks: schema branch `T039` failing regression → reopened `T001` → `T004` → reopened `T005` and `T006` fail/pass cycle → reopened `T008` → `T009`; authoring branch `T040` failing regression → reopened `T013` fail/pass cycle → reopened `T015`; both branches → reopened `T029` → `T030` → `T031`.
- BUG-005 corrective DAG overrides phase ordering for its reopened tasks: `T041` plus reopened `T012/T026/T037` failing universal-approval regressions → `T042` plus reopened `T014/T018/T028` correction and `T015/T017` focused audit → `T043` plus reopened `T016/T027` shared-presentation correction → `T044` focused mode matrix → reopened `T030` full gate → reopened `T031` native smoke → reopened `T036` packaged-runtime gate.
- Phase 14 post-verification convergence tail: `T036` packaged-runtime gate → `T045` exact master-dialog lifecycle coverage → `T046` CRT approval/rendering and full-browser coverage.
- BUG-006 corrective DAG overrides earlier completion state: parallel failing regressions `T047/T048` → reopened `T042` coordinator correction → reopened `T043` player/reconnect correction → `T049` focused cross-surface verification → reopened `T044` mode matrix → reopened `T030` full gate → reopened `T031` native smoke → reopened `T036` packaged-runtime gate.
- BUG-007 corrective DAG overrides the forward-transition rejection baseline: parallel failing regressions `T050/T051` plus reopened `T012/T027/T037/T041/T047/T048` → reopened `T014` coordinator correction → reopened `T016/T043` projection/reconnect audit → `T052` focused cross-surface and notification verification → reopened `T044` mode matrix → reopened `T030` full gate → reopened `T031` native smoke → reopened `T036` packaged-runtime gate.

## Parallel Opportunities

- Contract files `T001–T003`, foundation test files `T005–T007`, and foundation implementations `T009–T010` are independent inside their declared waves.
- After the foundation, each story's Go tests and Playwright tests can be authored together; implementation work marked `[P]` separates coordinator, master UI and player UI files.
- Tasks that revisit `internal/control/service.go`, `internal/control/service_test.go`, `client/client.js`, `frontend/src/master.js`, or `tests/browser/terminal-navigation.spec.mjs` stay ordered by phase even when they cover different stories.
- BUG-005 work touching the shared coordinator, private dialog, player renderer or browser fixture follows `T041 → T042 → T043 → T044`; prior feature-009 ordinary/completed regression expectations are changed only inside that ordered correction.
- BUG-006 failing Go/stream and browser regressions `T047/T048` are independent; all correction and verification work then follows the BUG-006 DAG, while completed-state-change, ~~terminal-transition~~ and return rejection tests remain unchanged guards; BUG-007 supersedes only the forward terminal-transition guard.
- BUG-007 failing Go/stream and browser regressions `T050/T051` are independent; the coordinator correction remains ordered before projection/reconnect audit and focused verification, while terminal-return rejection stays an unchanged guard.

## Phase 7: Convergence

- [x] T032 **CRITICAL** Сделать снятие блокировки обычной folder/entry/back-навигации независимым от порядка authoritative stream и unary result и добавить regression coverage для replay, multi-player, public-fallback и shared-lifecycle сценариев per FR-028 / Constitution Testing and Quality Gates (contradicts) · `client/client.js`, `tests/browser/connectrpc-player.spec.mjs`, `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/public-access-fallback.spec.mjs`, `tests/browser/state-changing-command-sync.spec.mjs`
- [x] T033 **CRITICAL** ⚠️ Reopened — разделить browser-assertions bundled demo для state-changing и terminal-transition команд, не требуя `text`/`stateChange` от валидной команды перехода, восстановить полный Playwright gate и доказать, что fixture round-trip сохраняет полный terminal set per plan: Verification Strategy / Constitution Testing and Quality Gates `(reopened — BUG-001)` · `tests/browser/state-changing-command-authoring.spec.mjs`, `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/fixtures/desktop-bindings.js`, `sessions/demo.json`

## Phase 8: BUG-001 — complete demo session across the native save boundary

**Purpose**: Воспроизвести и устранить ложное отклонение существующей цели `t_demo2`, не ослабляя missing/self-target validation.

### Tests

- [x] **T034** [US1] Добавить падающий regression для полного `sessions/demo.json` через production App/desktop `SaveSession` boundary: на входе и после contract/Wails round-trip присутствуют `t_demo1` и `t_demo2`, ссылка сохраняется и переоткрывается; отдельно сохранить negative assertions для missing/self target · `app_test.go`, `wails_host_test.go`, `internal/session/service_test.go`, `sessions/demo.json`

### Implementation

**⟶ Wait for T034 to fail for the reported reason, then:**

- [x] **T035** [US1] Зафиксировать payload по обе стороны Wails `SaveSession`, определить и исправить подтверждённый master/Wails/native projection либо stale-resource selection, теряющий `t_demo2`; передавать полный candidate document и не ослаблять `domain.ValidateSession` · `frontend/src/master.js`, `frontend/src/desktop-api.js`, `frontend/bindings/`, `desktop_service.go`, `app.go`, `internal/session/service.go`

**Checkpoint**: `T034` проходит после `T035`; затем выполнить переоткрытые `T013` и `T033`, полный gate `T030` и native acceptance `T031`.

## Phase 9: Convergence

- [ ] T036 **HIGH** ⚠️ Reopened — пройти и зафиксировать на accepted packaged Wails runtime полный native master + controller + two observers journey для approve/reject, 10 повторных посещений без повторного взлома, трёхтерминального LIFO unwind, reconnect во время pending и shutdown/new-broadcast cleanup; по BUG-002 доказать SC-010: прямой pending использует полноэкранный record-description screen «Выполняется запрос» без одновременно видимого меню у controller/observers/reconnect; по BUG-003 доказать SC-011: return pending использует ту же поверхность без текущего terminal screen/notice overlay и reject/close восстанавливает неизменённый root screen/return control; по BUG-005 доказать SC-013–SC-014 на ordinary, initial/completed state-change и transition approve/reject/close с exact master request, общим waiting renderer и ровно одним либо нулевым mode-specific effect; по BUG-006 доказать SC-015 на ordinary explicit reject/close и notification reject: controller/two observers/reconnect видят «Ошибка доступа» до controller acknowledgement и неизменённое source menu после него без эффекта; по BUG-007 доказать SC-016 на forward-transition explicit reject/close/notification reject с тем же access-error acknowledgement, нулевым target/route/checkpoint effect и неизменным return-reject baseline; не считать browser-fixture coverage или отдельный `sessions/demo.json` save/reopen smoke заменой этому acceptance gate per SC-001–SC-016 / plan: Verification Strategy and interactive acceptance / T031 (partial) `(reopened — BUG-002)` `(reopened — BUG-003)` `(reopened — BUG-005)` `(reopened — BUG-006)` `(reopened — BUG-007)` ~~`(verification pending — BUG-005: complete packaged acceptance evidence is not recorded)`~~ `(verified — BUG-005: complete packaged acceptance evidence is recorded in .spec-context.json)` · `build/bin/Fallout Terminal.app`, `sessions/demo.json`, `specs/010-terminal-navigation/.spec-context.json`

## Phase 10: BUG-002 — full-screen pending presentation for direct transitions

**Purpose**: Заменить notice overlay ожидающего прямого перехода единым полноэкранным экраном записи «Выполняется запрос», не меняя server-authoritative navigation lifecycle.

### Tests

- [x] **T037** [US1] [US4] ⚠️ Reopened — Добавить падающий Playwright regression для controller + two observers + reconnect: после выбора команды перехода source menu скрыто, отображается тот же record-description renderer, что у ожидающей state-changing команды, body точно равен «Выполняется запрос», `Back`/`Enter`/shared actions неэффективны, approve показывает target initial screen, а ~~reject/close восстанавливает неизменённое source menu~~ по BUG-007 reject/close показывает «Ошибка доступа» до controller acknowledgement и только затем восстанавливает unchanged source menu `(reopened — BUG-007)`; по BUG-005 использовать pending-поверхность как baseline и доказать тот же pending contract для ordinary и initial/completed state-change `(reopened — BUG-005)` · `tests/browser/terminal-navigation.spec.mjs`

### Implementation and verification

**⟶ Wait for T037 to fail for the reported overlay reason, then:** выполнить переоткрытую **T016**; после неё выполнить переоткрытые **T027** → **T030** → **T031** → **T036**.

**Checkpoint**: Прямой pending использует общий полноэкранный renderer во всех player views, а state, resolve и lifecycle semantics переходов остаются прежними.

## Phase 11: BUG-003 — full-screen pending presentation for returns

**Purpose**: Заменить notice overlay ожидающего межтерминального возврата единым полноэкранным экраном записи «Выполняется запрос», не меняя server-authoritative active terminal и LIFO route lifecycle.

### Tests

- [x] **T038** [US3] [US4] Добавить падающий Playwright regression для controller + two observers + reconnect: после выбора root return текущий terminal screen и `playerNotice` скрыты, отображается тот же record-description renderer, что у ожидающей state-changing команды и прямого перехода, body точно равен «Выполняется запрос», `Back`/`Enter`/shared actions неэффективны, approve восстанавливает previous terminal menu, а reject/close возвращает неизменённый current root screen и return control · `tests/browser/terminal-navigation.spec.mjs`

### Implementation and verification

**⟶ Wait for T038 to fail for the reported return-overlay reason, then:** выполнить переоткрытую **T016**; после неё выполнить переоткрытые **T023** и **T027** → **T030** → **T031** → **T036**.

**Checkpoint**: Forward и return pending используют общий полноэкранный renderer во всех player views, а active terminal, route mutation, resolve и lifecycle semantics остаются прежними.

## Phase 12: BUG-004 — one exclusive command behavior

**Purpose**: Выразить state-change и terminal-transition как один structural command behavior в persistence contract и один явный выбор режима в master UI, сохранив ordinary commands, JSON v1 и field numbers.

### Tests

- [x] **T039** [P] [US1] Добавить падающий descriptor/generated-API и adapter regression: `CommandContent.state_change = 2` и `terminal_transition = 3` принадлежат одному real `oneof behavior`, unset представляет ordinary command, каждый legacy valid variant сохраняет wire/JSON round-trip, а malformed dual-config JSON/import отклоняется · `internal/session/contract_test.go`, `internal/domain/model_test.go`, `internal/domain/validate_test.go`
- [x] **T040** [P] [US1] Добавить падающий Playwright regression для одного command-mode selector: ordinary/state-change/terminal-transition взаимоисключающи, switch скрывает и удаляет inactive config, completed-state guard сохраняется, а save/reopen восстанавливает ровно один mode · `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/state-changing-command-authoring.spec.mjs`, `tests/browser/fixtures/desktop-bindings.js`

### Implementation and verification

**⟶ After the regressions fail:** выполнить schema branch **T039** → **T001** → **T004** → **T005/T006** → **T008** → **T009** и authoring branch **T040** → **T013** → **T015**; после завершения обеих веток выполнить **T029** → **T030** → **T031**.

**Checkpoint**: SC-012 доказана на descriptor, adapters, browser и packaged native paths; каждый command имеет ordinary/unset либо ровно один configured behavior, а существующие session v1 field numbers и JSON names не меняются.

## Phase 13: BUG-005 — universal master approval for every command

**Purpose**: Поместить единый server-authoritative master approval gate перед эффектом каждого command mode и показывать всем игрокам один record-description экран «Выполняется запрос» до решения, не превращая ordinary-команды в durable state changes и не объединяя mode-specific approved outcomes.

### Wave 1 — failing regression and artifact-conflict proof

- [x] **T041** [US1] [US4] ⚠️ Reopened — Добавить падающую cross-mode regression matrix для ordinary, initial state-changing, completed state-changing и terminal-transition commands: каждый controller selection создаёт один exact master request, controller + two observers + reconnect видят один полноэкранный «Выполняется запрос», 20 replays/competing actions не создают второй pending, а до approve отсутствуют result, durable write и terminal/route mutation; отдельно зафиксировать падение feature-009 ordinary/completed no-request expectations per FR-028/FR-033–FR-036, SC-013; по BUG-007 расширить terminal-transition reject/close ветку до exact `REJECTED` projection, acknowledgement и zero target/route/checkpoint effect, сохраняя return reject baseline `(reopened — BUG-007)` · `internal/control/service_test.go`, `internal/live/service_test.go`, `internal/player/public_stream_test.go`, `tests/browser/state-changing-command-approval.spec.mjs`, `tests/browser/terminal-navigation.spec.mjs`

**⟶ T041 and reopened T012/T026/T037 must fail for the approval bypass before Wave 2.**

### Wave 2 — universal approval boundary and private decision

- [x] **T042** [US1] ⚠️ Reopened — Реализовать behavior-independent command interception после authority/replay и немутирующих mode-specific preflight checks, но до mode dispatch: сохранить safe pre-pending rejection invalid/stale/missing/self-target, хранить не более одного pending across ordinary/state-change/transition/return, проецировать exact command name/mode и применимый existing context в private master dialog, принимать exact approve/reject/close, затем направлять approve в ordinary result без write, initial state-change persist-once, completed frozen result без write или atomic terminal transition; ~~reject/close сохраняет существующую mode-specific post-decision presentation без эффекта~~ по BUG-006 ordinary reject/close очищает pending, но публикует exact `REJECTED` presentation до controller acknowledgement без result/write/terminal/route/nav effect, сохраняя completed-state-change/transition behavior `(reopened — BUG-006)`. Переиспользовать существующие private/public contracts либо сделать только минимальное additive расширение и обновить superseded feature-009 ordinary/completed tests `(reopened T014/T018/T028; audit T015/T017)` · `internal/control/service.go`, `internal/live/service.go`, `internal/player/adapter.go`, `app.go`, `app_contract.go`, `desktop_service.go`, `frontend/src/master.js`, `frontend/src/desktop-api.js`, `tests/browser/fixture-server/main.go`, `tests/browser/fixtures/desktop-bindings.js`

**⟶ T042 and reopened T014/T018/T028 plus the T015/T017 audit must finish before Wave 3.**

### Wave 3 — shared player presentation and reconnect

- [x] **T043** [US1] [US4] ⚠️ Reopened — Провести ordinary, initial/completed state-change и transition pending через один record-description presentation primitive с точным текстом «Выполняется запрос», скрытым menu/result/terminal screen, server + UI блокировкой `Back`/`Enter`/shared actions, monotonic controller/observer/reconnect projection и без optimistic result; ~~сохранить существующие mode-specific post-decision screens~~ по BUG-006 ordinary `REJECTED` должен использовать общий полноэкранный «Ошибка доступа» renderer у controller/observers/reconnect, скрывать menu/result и очищаться только `Back`/`Enter` current controller; ~~сохраняя остальные mode-specific screens~~ по BUG-007 forward terminal-transition `REJECTED` MUST переиспользовать ту же detached projection/renderer/reconnect/acknowledgement, сохраняя completed-state-change и terminal-return screens `(reopened — BUG-006)` `(reopened — BUG-007)` `(reopened T016/T027)` · `client/client.js`, `client/client.css`, `internal/player/adapter.go`, `internal/player/stream.go`, `tests/browser/state-changing-command-approval.spec.mjs`, `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/fixture-server/main.go`

**⟶ T043 and reopened T016/T027 must finish before Wave 4.**

### Wave 4 — focused verification

- [x] **T044** [US1] [US4] ⚠️ Reopened — Проверить controller + two observers + reconnect для approve/reject/close каждого command mode, exact visible master identity/context, 20 replayed/concurrent selections, lifecycle cancellation и отсутствие раннего эффекта; доказать ordinary/completed zero-write, initial state-change exactly-one durable write, transition exactly-one active-terminal/route mutation и reject/close zero-effect; по BUG-006 ordinary explicit reject/close MUST сохранять exact `REJECTED` «Ошибка доступа» до controller acknowledgement и затем восстанавливать unchanged source menu `(reopened — BUG-006)`; по BUG-007 forward transition explicit reject/close MUST сохранять ту же exact rejection presentation без target/route/checkpoint effect, а return reject baseline MUST оставаться unchanged `(reopened — BUG-007)`, после чего передать SC-013–SC-016 в переоткрытые T030 → T031 → T036 · `internal/control/service_test.go`, `internal/live/service_test.go`, `internal/player/public_stream_test.go`, `tests/browser/state-changing-command-authoring.spec.mjs`, `tests/browser/state-changing-command-approval.spec.mjs`, `tests/browser/terminal-navigation.spec.mjs`

**Checkpoint**: Любая команда требует private master approval, все игроки видят один server-authoritative экран ожидания, а approve сохраняет ровно исходную семантику выбранного command mode.

## Phase 14: Convergence

- [x] T045 HIGH Обновить `tests/browser/state-changing-command-sync.spec.mjs`, чтобы helper и три lifecycle journey проверяли обязательные exact request ID, command mode и command name в master dialog, сохраняя controller + two observers convergence, disconnect, restart/reopen и zero-extra-write assertions, затем провести isolated и полный browser gate per FR-035 / plan: Verification Strategy (contradicts)
- [x] T046 HIGH Провести ordinary-command состояния CRT fixture через явное private master approval до diagnostic result/reveal assertions, сохранить отдельную проверку полноэкранного `Выполняется запрос` и input lock и восстановить все viewport/reveal и полный browser gate в `tests/browser/crt-rendering.spec.mjs` и `tests/browser/fixture-server/main.go` per FR-033, FR-034, FR-036, SC-013, SC-014 / plan: Phase 5 (contradicts)

## Phase 15: BUG-006 — ordinary rejection access-error presentation

**Purpose**: Сохранить ordinary-команду effect-free при reject/close, но показать всем игрокам server-authoritative полноэкранный экран «Ошибка доступа» до controller acknowledgement вместо немедленного возврата в source menu.

### Wave 1 — independent failing regressions

- [x] **T047** [P] [US1] [US4] ⚠️ Reopened — Добавить падающий coordinator/player regression для ordinary explicit reject и close-as-reject: exact pending очищается, active runtime публикует `CommandExecutionPhaseRejected` для той же команды, controller + two observers + reconnect сходятся на projection, source nav остаётся неизменной, а authored result, durable write, terminal и route effects равны нулю; ~~отдельно сохранить completed-state-change/transition rejection guards~~ по BUG-007 сохранить completed-state-change guard, но заменить forward-transition direct-menu guard на exact `REJECTED` projection до acknowledgement, не меняя return reject `(reopened — BUG-007)` · `internal/control/service_test.go`, `internal/player/adapter_test.go`, `internal/player/public_stream_test.go`
- [x] **T048** [P] [US1] [US4] ⚠️ Reopened — Добавить падающий Playwright regression ordinary explicit reject/close: у controller + two observers + reconnect menu/result скрыты и body точно равен «Ошибка доступа» до controller `Back` и отдельного `Enter` journey, observers остаются read-only, затем все синхронно возвращаются в byte-for-byte unchanged source menu; ~~сохранить transition/return rejection baselines~~ по BUG-007 применить тот же access-error flow к forward transition и сохранить только return rejection baseline `(reopened — BUG-007)` · `tests/browser/state-changing-command-approval.spec.mjs`, `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/fixture-server/main.go`

**⟶ T047 and T048 must fail for the direct-menu ordinary rejection before executing reopened T042 → T043.**

### Wave 2 — focused cross-surface verification

- [x] **T049** [US1] [US4] После T042/T043 прогнать focused ordinary reject/close Go, stream и browser матрицу, доказать notification-action reject через тот же authoritative App decision path без отдельной player semantics, проверить lifecycle clearing и передать SC-015 в reopened T044 → T030 → T031 → T036 · `approval_notifications_test.go`, `app_test.go`, `internal/control/service_test.go`, `internal/player/public_stream_test.go`, `tests/browser/state-changing-command-approval.spec.mjs`

**Checkpoint**: Ordinary explicit reject, close-as-reject и notification reject показывают всем «Ошибка доступа» до controller acknowledgement, затем возвращают неизменённое source menu без command, persistence, terminal или route effect; completed-state-change, ~~terminal-transition~~ и return rejection semantics не изменены. BUG-007 supersedes только terminal-transition rejection semantics.

## Phase 16: BUG-007 — forward terminal-transition rejection access-error presentation

**Purpose**: Сохранить forward terminal-transition effect-free при reject/close, но показать всем игрокам server-authoritative полноэкранный экран «Ошибка доступа» до controller acknowledgement вместо немедленного возврата в source menu, не меняя terminal-return rejection.

### Wave 1 — independent failing regressions

- [x] **T050** [P] [US1] [US4] Добавить падающий coordinator/player regression для forward transition explicit reject и close-as-reject: exact `PendingTerminalNavigation` очищается, source runtime публикует `CommandExecutionPhaseRejected` для exact transition command, controller + two observers + reconnect сходятся на projection, source nav/terminal/checkpoint и route остаются неизменными, target не активируется; отдельно доказать controller `Back`/`Enter` acknowledgement и unchanged terminal-return reject guard · `internal/control/service_test.go`, `internal/player/adapter_test.go`, `internal/player/public_stream_test.go`
- [x] **T051** [P] [US1] [US4] Добавить падающий Playwright regression forward transition explicit reject/close: у controller + two observers + reconnect source menu и target terminal скрыты, body точно равен «Ошибка доступа» до controller `Back` и отдельного `Enter` journey, observers read-only, затем все синхронно возвращаются в byte-for-byte unchanged source menu; сохранить immediate current-root restoration для return reject · `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/fixture-server/main.go`

**⟶ T050/T051 and reopened T012/T027/T037/T041/T047/T048 must fail for direct-menu forward-transition rejection before executing reopened T014 → T016/T043.**

### Wave 2 — focused cross-surface verification

- [x] **T052** [US1] [US4] После T014/T043 прогнать focused forward-transition reject/close Go, stream и browser матрицу, доказать notification-action reject через тот же authoritative App terminal-navigation decision path без отдельной player semantics, проверить lifecycle clearing и отсутствие protobuf/generated delta, затем передать SC-016 в reopened T044 → T030 → T031 → T036 · `approval_notifications_test.go`, `app_test.go`, `internal/control/service_test.go`, `internal/player/public_stream_test.go`, `tests/browser/terminal-navigation.spec.mjs`

**Checkpoint**: Forward terminal-transition explicit reject, close-as-reject и notification reject показывают всем «Ошибка доступа» до controller acknowledgement, затем возвращают неизменённое source menu без terminal activation, route push, checkpoint or persistence effect; completed-state-change и terminal-return rejection semantics не изменены.
