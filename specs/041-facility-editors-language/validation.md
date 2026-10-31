# Validation: Facility Editors and Language

Run on 2026-10-31, on macOS arm64, with Go 1.27.3, Node 26.8.1 and branch `041-facility-editors-language`, with the working tree uncommitted. Every Go command ran with the macOS 13 deployment-target environment from `CLAUDE.md`.

## Suites (T046)

| Gate | Result |
|---|---|
| `go fix ./...`, then `go fix -diff ./...` | PASS. Applied, then no pending edits (empty diff) |
| `task fmt:check` | PASS |
| `task vet` | PASS |
| `task lint` | PASS |
| `task test` | PASS |
| `task test:race` | PASS |
| `task proto:check` | PASS. Generation and drift match the sources |
| `task proto:breaking` | PASS against the regenerated baseline, with every negative fixture rejected |
| `task bindings:check` | PASS. 46 desktop methods and seven named events |
| `task wails:pins:check` | PASS. The pinned revision and baseline digests were updated in T009; the revision pin was already stale on 040 |
| `task frontend:build` | PASS |
| `task secret-leak:check` | PASS |
| `task browser:test` | PASS. 438 passed, 2 skipped (the live ngrok journeys, which need credentials), 0 failed, 6.1 min |

Browser runs share fixed port 34119 with the parallel Wails 3.0.0 worktree. Every run went through one lock, so each run hit this tree's fixture server.

## Success criteria

| Criterion | Evidence |
|---|---|
| SC-001: states and the full matrix are visible at 1440×900 | `facility-device-editor.spec.mjs` checks every demo device: chips, the unscrolled matrix and the buttons all sit in the viewport. Packaged app: see T047 |
| SC-002: no identifier field until «Технические идентификаторы» opens | `facility-device-editor.spec.mjs` AS5; `facility-p0-identifiers.spec.mjs` |
| SC-003: a templated door takes three actions | `facility-device-templates.spec.mjs` (SC-003, AS1) |
| SC-004: the fault lists match the session data | `facility-fault-dialog.spec.mjs` checks every demo fault against the transitions' fault effects and its programs |
| SC-005: all four rule types, sentence order, no orphan labels | `facility-display-rules.spec.mjs`; `facility-p0-visible-fields.spec.mjs` |
| SC-006: «Изменённые» 5 and «С неисправностью» 1 on the demo | `facility-device-list.spec.mjs` |
| SC-007: zero banned wordings | `facility-vocabulary.spec.mjs`. It scans `index.html`, `overseer.js` and `desktop-api.js` against `docs/overseer-glossary.md`; a self-test proves the patterns bite |
| SC-008: no English outside «подробности для разработчика», with a next step | `overseer-errors.spec.mjs` covers player config, session, facility save, validation entity names, public access and update. The Go catalog sync tests are `internal/playerconfig/messages_test.go` and `internal/session/service_messages_test.go` |
| SC-009: the demo opens and saves with no format change | `demo-session.spec.mjs`, `internal/platform/assets_test.go`, and the session round-trip tests. Packaged app: see T047 |

## Found and fixed during implement

- **T049:** a fault switched off kept its effect on a connected player. The cause was in the browser fixture only: `fixtureFacilityLifecycle.ProjectFacility` wrote earlier facility effects back over each new projection. The real app path was proven correct by a new control test, `TestFacilityFaultSwitchedOffReachesConnectedPlayers`.
- **T048:** `overseer-runtime-logs.spec.mjs` had no owning task. It was added to Phase 9.
- **Spec example:** User Story 2, scenario 2 named a transition that runs together with the clearing transition in one command. The domain refuses that combination, so the example now uses «Запереть гермодверь», and the pickers hide such transitions.

## Packaged app (T047), 2026-10-31

`task package` exited 0 and produced `build/bin/Fallout Terminal.app`, which was driven with computer use. Two copies were used:

- `~/Documents/Fallout Terminal/Sessions/check-041-before.json`: `sessions/demo.json` from the 040 tip `514eee3`.
- `~/Documents/Fallout Terminal/Sessions/check-041-demo.json`: the current demo.

One headless Playwright player joined `http://127.0.0.1:3690` as «Пайпер Райт» and logged its screen once a second. Durable results were read from the session files.

The app had two limits:

- macOS grants this app only click-level control: no typing, no key presses, and no window resizing. An exact 1440×900 window could not be set. Layout was checked in the default window (about 1200×780, smaller than 1440×900) and in full screen (about 1470×956, larger).
- The «Текст записи» and «Название в меню» rules, and naming a new device, need typing. Those paths are covered by `facility-display-rules.spec.mjs` (which includes a connected player) and `facility-device-templates.spec.mjs`, not by this run.

| Check | Result |
|---|---|
| Pre-feature session opens unchanged | PASS. The SHA-256 of `check-041-before.json` stayed `d4a4a3cf…c398` after opening |
| Pre-feature session saves without a format change | PASS. Starring «Открыта» on the door saved: the header read «В НАЧАЛЕ: Открыта», the door badge was framed, and «ИЗМЕНЁННЫЕ» went from 5 to 6 at once. After «ОТМЕНИТЬ», the file is byte-identical to the original except `facility.revision` (12 → 14, one per commit) |
| SC-001: device dialogs | PASS. All ten demo devices were opened in full screen. In each, the chips, the whole matrix and «ОТМЕНА / ГОТОВО» were visible with nothing scrolled: the reactor's 3×3 matrix with the two-transition cell «Перевести в дежурный режим / Пуск от резервной сети · 1 предусловие», and «1 предусловие · снимает 1 неисправность» on the door. The door was also checked in the default window |
| Templates | PASS. Each templated type shows its offer. The turret and the lift read «ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ», and «ДРУГОЙ» offers no template |
| Device list | PASS. «ВСЕ 10», «ИЗМЕНЁННЫЕ 5», «С НЕИСПРАВНОСТЬЮ 1»; framed badges on the changed devices; «● 1» on the ventilation loop; live recount after play changes. Defect found and fixed below |
| Terminal-fault switch (FR-021) | PASS. «Закрытый канал изолирован» was switched off and on from «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» with a device selected. Status lines «НЕИСПРАВНОСТЬ СНЯТА» / «НЕИСПРАВНОСТЬ АКТИВИРОВАНА». The file had `currentActive` false, then true, while `initialActive` and the stored `recovery[]` (including `privateOverseerAction`) stayed unchanged |
| Fault dialog | PASS. «НЕИСПРАВНОСТЬ» with category, «ГДЕ ДЕЙСТВУЕТ: ТЕРМИНАЛ» and start position. «КАК ПОЯВЛЯЕТСЯ» reads «Активна с начала игры» plus «Изолировать служебную сеть». «КАК УСТРАНЯЕТСЯ» lists «Восстановить служебную сеть», «СЕТЬ-76» with «К ПРОГРАММЕ», and «Смотритель вручную». There is no checkbox |
| «Показать элемент» rule with a player | PASS. Rule «КОГДА Сигнал эвакуации = Эвакуация, ГДЕ Терминал смотрителя — Убежище 76 › ДИРЕКТИВЫ VAULT-TEC», with the «ГДЕ» list showing folder paths. The player lost the folder at revision 11. After «Подать сигнал эвакуации» on the card, the folder came back at revision 12. There was one page load for the whole run, so no reconnect |
| «Разрешить команду» rule and conflict | PASS. Choosing «Открыть выход и подать сигнал» was refused with «У элемента уже есть правило «Разрешить команду»: Резервная сеть Убежища 76 = В работе…» and «ОТКРЫТЬ ПРАВИЛО», and nothing was saved. «Проверить наружные датчики» saved `availableWhen` alarm = evacuation; the existing rule was untouched |

### Found in the packaged app and fixed

- **Badge wrap**: in the narrow sidebar the state badge broke mid-word («Загрязнё / н», «Патрул / ь»). The badge is now `nowrap` and does not shrink, and the name column wraps instead.
- **Custom type label**: a custom device was listed by its identifier-derived kind («water-purifier»). The dialog has no field for it, so the list and header now read «ДРУГОЙ». The contract row and two journeys are updated.
- **Stale conflict notice**: the rule conflict notice stayed after the author changed «ГДЕ». Any input in the sentence now clears it.
- **Folder path**: rule rows named only «<terminal> › <element>». They now use the same folder path as «ГДЕ».

The full browser suite was re-run after these fixes; see below.

### Re-run after the fixes

`task browser:test`: 438 passed, 2 skipped, 0 failed (6.2 min). `task fmt:check` and the two catalog sync tests pass again.
