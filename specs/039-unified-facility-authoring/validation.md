# Validation: Unified Facility Authoring

Date: 2026-10-30 · Branch: `039-unified-facility-authoring` (stacked on
`038-facility-authoring-p0-fixes` at 7fc65a7) · macOS arm64, Node.js 26.8.1 from `.nvmrc`.

## Automated gates

| Command | Result |
|---|---|
| `go fix ./...`, then `go fix -diff ./...` | No edits proposed (exit 0) |
| `task proto:check` | PASS (exit 0) |
| `task proto:breaking` | PASS (exit 0); the five documented removals are the only breaks against the old baseline, which was refreshed |
| `task bindings:check` | PASS (exit 0), 44 desktop methods, seven events |
| `task fmt:check` | PASS (exit 0) |
| `task vet` | PASS (exit 0) |
| `task lint` | PASS, 0 issues |
| `task test` | PASS (exit 0) |
| `task test:race` | First run: 1 failure, `internal/player` `TestPresentationUplinkBodyActivityResetsOnlyIdleDeadline` (idle-deadline timing under load; the package is untouched by this feature). It passed `-race -count=5` alone and as a package; the full rerun PASSED (exit 0) |
| `task frontend:build` | PASS (exit 0) |
| `task browser:test` (full suite, 318 tests) | 315 passed, 2 skipped (public-network opt-ins), 1 failed: `terminal-grouping.spec.mjs` still expected the old «Сохранено» status text. Fixed to assert `data-state="saved"`; rerun of that spec plus `state-changing-command-authoring.spec.mjs`: 42/42 PASS |
| `task package` | PASS (exit 0), signed `build/bin/Fallout Terminal.app` |

Story journeys, run on their own before the full suite:

| Specs | Result |
|---|---|
| US1: `facility-unified-save` + nine updated journeys | 107/107 PASS (46 of the 98 pre-existing tests failed before the update) |
| US2 + US3 + demo: `facility-command-lists`, `facility-preview-diff`, `demo-session` | 22/22 PASS |

## Sweep (`rg`, outside `specs/`)

No removed method (`SaveFacilityAuthoring`, `ResetTerminalCommandStates`, `UpdateLiveTerminal`,
`SaveSession` result types, `ResetFacility`) or removed control (`#btnPublish`, `#btnSaveFacility`,
`#btnDiscardFacility`, `#btnResetFacility`, `#facilityResetDialog`, `#commandDraftGuard`) remains
outside negative assertions in tests and the contract guards in `app_contract_test.go`,
`internal/platform/assets_test.go`, and `scripts/state-changing-reset-native-smoke.sh`. The removed
copy (draft labels, «ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ», «СБРОСИТЬ ВСЕ СОСТОЯНИЯ», «СБРОСИТЬ ВЕСЬ ОБЪЕКТ») appears
only in negative assertions. `README.md` and `sessions/demo-capability-paths.md` were updated.

## Success Criteria

| Criterion | Evidence |
|---|---|
| SC-001 ten alternating edits, none lost | `facility-unified-save.spec.mjs` (ten edits across tabs; burst of five with one `SaveAuthoring` in flight; second edit rejected while the first saves) |
| SC-002 same status on both tabs | `facility-unified-save.spec.mjs` (saving, saved, unapplied, rejected) |
| SC-003 one vocabulary | `facility-unified-save.spec.mjs` and `facility-p0-visible-fields.spec.mjs` («ГОТОВО» on every edit dialog, DOM and served-source sweep) |
| SC-004 command lists match players | `facility-command-lists.spec.mjs` (effects for every state-changing demo command; winner and overrides; player parity before and after approval on `facility-player-state`); `internal/live` explanation and projection parity tests |
| SC-005 one scene reset | `facility-command-lists.spec.mjs` (counts, forced failure resets nothing, player stays on the terminal); `internal/session/scene_test.go`, `internal/control/scene_test.go` |
| SC-006 preview names differences | `facility-preview-diff.spec.mjs` (every demo device on every terminal; door preselects «Открыта»; summary and marks) |
| SC-007 byte-identical projections | `internal/session/authoring_test.go` `TestSaveAuthoringRoundTripsEverySessionFileWithIdenticalProjections`, `TestSaveAuthoringNoOpLeavesDocumentAndRevisionsUnchanged` |

## NOT RUN

- Superseded: the packaged-app quickstart ran in Phase 7 (T063, below).

## Convergence (Phase 7), 2026-10-30

| Task | Change | Evidence |
|---|---|---|
| T058 | `#btnResetCommandState` gets `aria-disabled` and «ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА» in `#btnResetCommandStateReason` while a session-state command runs; the fake `ResetCommandState` is now tracked so a journey can hold it | `facility-unified-save.spec.mjs` «the single-command reset says on the control why it waits for another reset (FR-005)» |
| T059 | Browser coverage only | `facility-command-lists.spec.mjs` «a fault that forces a command visible outranks its state visibility (FR-009, FR-010)» |
| T060 | No change. Session validation forbids two holding rules of one kind for one target: name and text variants use one controlling device with distinct states, and each record substitution or diagnostic path has one owning fault. The cancelled-rule case cannot reach the command form | `internal/domain/validate.go` (`validateTextVariants`, `recordSubstituteOwners`, `diagnosticPathOwners`) |
| T061 | Preview nodes carry every difference kind (`data-kind` is a space-separated list, one badge per kind) | `facility-preview-diff.spec.mjs` «an element with two differences carries both marks in both panes (FR-018)» |
| T062 | The entry-block command dialog and the create-terminal dialog reopen with the author's input and the reason after a rejected save | `facility-unified-save.spec.mjs` «a rejected terminal dialog edit reopens the dialog with its input and the reason (AS6)» |

| Command | Result |
|---|---|
| `task frontend:build` | PASS (exit 0) |
| `task browser:test -- facility-unified-save.spec.mjs facility-command-lists.spec.mjs facility-preview-diff.spec.mjs` | 30/30 PASS |
| `task browser:test` (full suite) | 320 passed, 2 skipped (public-network opt-ins), 0 failed (exit 0) |
| `go test ./internal/platform/... .` (macOS deployment flags) | PASS |

The only Go change in this phase is the browser fixture server (`stripFixturePreviewBlocks`, below):
`go vet` and `go fix -diff` on `./tests/browser/fixture-server/` are clean. No application Go source
changed, so `task vet`, `task test`, and `task test:race` were not rerun.

### T063: packaged app, 2026-10-30

`task package` (exit 0), then `build/bin/Fallout Terminal.app` was driven with computer use on a copy
of `sessions/demo.json` and `demo-players.json` placed in `~/Documents/Fallout Terminal/Sessions/`
(the Overseer has no demo-copy control, so the copy was made in the shell). Two headless Playwright
players (controller and observer) ran against `http://127.0.0.1:3690`. Durable results were read from
the session file after each step.

Limit: macOS grants this app only click-level control (its name contains "Terminal"), so nothing could
be typed. Each typed edit in the quickstart was replaced by a click-only edit of the same kind.

| Step | Result |
|---|---|
| 1. One save model | PASS. Facility edit: the door's starting state via the graph dialog, «ГОТОВО». Terminal edit: hack level, «ПРИМЕНИТЬ». Both saved at once with the same «СОХРАНЕНО · time» on both tabs and no stale message. «ОТМЕНИТЬ» twice reversed them newest first, and «ПОВТОРИТЬ» once restored only the device edit (file checked each time). An unapplied hack level survived two tab switches and showed «НЕ ПРИМЕНЕНО: Настройки «Терминал смотрителя — Убежище 76»» on both tabs |
| 2. Live never rejects | Partly run. 2.1 broadcast on the warden terminal: PASS. 2.2 NOT POSSIBLE: the approval dialog is modal and Escape rejects the request, so the Overseer cannot edit anything while a request is pending; the pending rebase (R4) is reachable only through the Go and fixture tests. 2.3 PASS: after approval, «+ ЗАПИСЬ» reached both players at once while they stayed inside ПРОТОКОЛ ВЫХОДА; «ОТМЕНИТЬ» removed it from both at once and did not reverse the door, alarm, or executed command. The terminal had no hacking, so hacking progress was not checked here (the FR-001c journey covers it) |
| 3. Command lists | PASS. «ПРИ ОДОБРЕНИИ» listed both transitions, «Выход открыт», and block 2 of «СВОДКА ЭВАКУАЦИИ». «ЧТО ВИДЯТ ИГРОКИ» listed the door-state name and the power-grid availability rule. After approval, the tree, the «ВИДЯТ ИГРОКИ» rule, and both players showed «Выход уже открыт», with «ПЕРЕКРЫВАЕТ: ИСХОДНОЕ НАЗВАНИЕ, НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ» |
| 4. Scene reset | PASS. The dialog counted 7 devices, 2 faults, 3 executed commands (matching the file) and said players stay. After confirming, the file had every device and fault at its starting value and no executed command; both players stayed on the terminal and inside the folder, seeing the reset names; the control then read as blocked |
| 5. Preview | PASS after a fix. 5.1 «Открыта» preselected, «СЕЙЧАС»/«ЕСЛИ» side by side, «ИЗМЕНЕНИЙ: 3». **Defect found:** the summary read «ТЕКСТ ИЗМЕНИТСЯ: СВОДКА ЭВАКУАЦИИ» without «· блок 3», because the player projection joins entry blocks and carries no block IDs, while the fixture served them. Fixed in `overseer.js` (block numbers come from the authored terminal) and in the fixture (`stripFixturePreviewBlocks` mirrors the player contract); the rebuilt package then showed «· блок 3». 5.2 (click-only): deleting the door's block-3 text rule and reopening showed «ИЗМЕНЕНИЙ: 2»; «ОТМЕНИТЬ» restored the rule. 5.3 closing the preview left the file hash and both players unchanged |
| Reopen | Reopening the session cleared «ОТМЕНИТЬ» and «ПОВТОРИТЬ» |
| Compatibility | Not repeated by hand; covered by `TestSaveAuthoringRoundTripsEverySessionFileWithIdenticalProjections` |

After the fix: `task browser:test` (full suite) 320 passed, 2 skipped, 0 failed (exit 0).
