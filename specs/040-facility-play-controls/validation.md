# Validation: Facility Play Controls

Date: 2026-10-31 · Branch: `040-facility-play-controls` (stacked on `039-unified-facility-authoring`
at e67edea) · macOS arm64, Node.js 26.8.1 from `.nvmrc`.

## Automated gates

Each command was run and its exit code recorded in `.spec-context.json`.

| Command | Result |
|---|---|
| `go fix ./...`, then `go fix -diff ./...` | No edits proposed (exit 0) |
| `task fmt:check` | PASS (exit 0) |
| `task vet` | PASS (exit 0) |
| `task lint` | PASS (exit 0) |
| `task test:race` | PASS (exit 0) |
| `task proto:check` | PASS (exit 0) |
| `task proto:breaking` | PASS (exit 0) against the existing baseline; the baseline was not refreshed |
| `task bindings:check` | PASS (exit 0), 47 desktop methods, seven events |
| `task frontend:build` | PASS (exit 0) |
| `task secret-leak:check` | PASS (exit 0) |
| Full browser suite (`npm test --prefix tests/browser`, serialized) | 365 passed, 2 skipped (public-network opt-ins), 0 failed (exit 0) |
| `task package` | PASS (exit 0), `build/bin/Fallout Terminal.app` |

Pins updated on purpose: the schema revision digest and the descriptor-shape hash in
`internal/platform/assets_test.go`, and the desktop method count (44 → 47) in
`app_contract_test.go`, `scripts/wails-bindings-check.sh`, and `tests/browser/desktop-api.spec.mjs`.

## Story journeys

| Story | Journey | Result |
|---|---|---|
| US1 | `facility-play-transitions.spec.mjs` | 7/7 PASS |
| US2 | `facility-workspace-layout.spec.mjs`, updated `facility-authoring-ux.spec.mjs` | 8/8 and 21/21 PASS |
| US3 | `facility-play-force.spec.mjs` | 6/6 PASS |
| US4 | `facility-play-faults.spec.mjs`, updated `facility-p0-fault-activity.spec.mjs` | 6/6 and 3/3 PASS |
| US5 | `facility-play-feed.spec.mjs` | 6/6 PASS |
| US6 | `facility-device-actions.spec.mjs` and seven updated journeys | 56/56 PASS |
| Demo | `demo-session.spec.mjs` | 6/6 PASS |

## Success criteria

| Criterion | Evidence |
|---|---|
| SC-001 two actions, no dialog | US1 journey: select the device, one press, no open dialog |
| SC-002 card verdict matches the player command | `TestDemoTransitionVerdictsMatchThePlayerCommandPath`: all 22 demo cases agree between the card operation and the player command path with no blocking fault. The US1 journey compares the card's `data-available` with the same golden table |
| SC-003 header and first control visible | US2 journey at 1440×900 and 1180×760 for every demo device, in Chromium |
| SC-004 players updated in the same update | US1, US3 journeys on the player-state fixture: the player sees the change with no reconnect |
| SC-005 every play change in the feed and the log | Control tests: one audit record and one feed entry in the same commit for each changed play change, none for unchanged or rejected ones; App tests: the new events, actions, and outcomes reach the log |
| SC-006 never over a newer state | Session, control, and App stale tests; `TestConcurrentFacilityPlayChangesApplyAtMostOnce`; US1 and US3 stale journeys |
| SC-007 no destructive control outside «⋯» | US6 journey |
| SC-008 demo files open and save unchanged | Demo journey, the session round-trip tests, `TestDesktopSessionFacadeSavesRealDemoCrossTerminalLinkAndReopensIt` |

## Packaged app (T051), 2026-10-31

`task package` (exit 0), then `build/bin/Fallout Terminal.app` was driven with computer use on
`~/Documents/Fallout Terminal/Sessions/check-040-demo.json`, a copy of `sessions/demo.json`. One
headless Playwright player (controller) ran against `http://127.0.0.1:3690` inside «ПРОТОКОЛ ВЫХОДА»
and recorded its screen every second. Durable results were read from the session file.

Limit: macOS grants this app only click-level control, so the window cannot be dragged or resized to
exact sizes. The layout was checked at the default window, 1200×780 (close to 1180×760), and in full
screen, about 1470×956 (larger than 1440×900).

| Check | Result |
|---|---|
| Guide and device list | PASS. The guide is one line «КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 1 ИЗ 5: ОБЪЕКТ» with «ПОКАЗАТЬ». Rows show the current state, and the ventilation row reads «Загрязнён · ● 1». Whole-session tabs read 5 / 21 / 7 / 2 |
| Layout at 1200×780 | PASS for the door, the reactor, and the ventilation loop: header, every play card part, and both action groups visible without scrolling |
| Layout in full screen | PASS for the door, the network segment, and the ventilation loop; the door's tabs read 1 / 3 / 1 / 0 |
| Card transition | PASS. «Открыть гермодверь → Открыта» ran on one press. The player saw «Выход уже открыт» with no reconnect (zero navigations). The file had the door `open` and no `n_cmd_open_evacuation` command state |
| Forced change | PASS. The dialog named «Реактор Убежища 76: Дежурный режим → Критический перегрев». After «ПЕРЕВЕСТИ» the file had `critical-overheat`, the card read «ИЗ ЭТОГО СОСТОЯНИЯ НЕТ ПЕРЕХОДОВ», and the feed recorded the forced change |
| Fault switch | PASS. «Контроллер гермодвери не отвечает» on: the file had `currentActive` true and `initialActive` false; the header and the row counted it |
| Reset | PASS. The reset restored the door and its fault; the player saw «Открыть выход и подать сигнал» again |
| Fault-only reset (R9) | PASS. With the door sealed and only the fault on, «СБРОСИТЬ К НАЧАЛЬНОМУ…» showed «УСТРОЙСТВО СБРОШЕНО» and the feed «сброс к начальному: неисправности восстановлены» |
| Feed | PASS. Five entries newest first with local times and authors |
| «⋯» menu | PASS. It opened with «УДАЛИТЬ УСТРОЙСТВО» and closed on an outside click |
| Restart and old session | PASS. After quitting and relaunching, a copy of the pre-feature `sessions/demo.json` (`check-040-before.json`) opened with the play card and an empty feed; its SHA-256 was unchanged after opening |
