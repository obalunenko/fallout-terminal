# Validation: Facility recheck follow-ups

Branch `042-facility-recheck-fixes`, from `develop` at `d50efcb`. Runs made on 2026-11-01 on macOS with Go 1.27.3 and the Node.js version from `.nvmrc`.

## Repository checks (T028)

| Check | Result |
|---|---|
| `go fix ./...` | No edits |
| `task fmt:check` | Pass |
| `task vet` | Pass |
| `task lint` | 0 issues. The first run found two helpers left unused, `facilityCapabilityBlocked` and the fixture's `fixtureCapabilityBlocked`, after their callers moved to the condition lists. Both were deleted |
| `task test` | Pass, 18 packages |
| `task test:race` | Pass, 18 packages |
| `task proto:check` | Pass |
| `task proto:breaking` | Pass. The new enum value and fields are additive |
| `task bindings:check` | Pass, 46 desktop methods and seven events |
| `task wails:pins:check` | Pass, with the schema revision digest `80ab384c…` pinned |
| `task frontend:build` | Pass |
| `task secret-leak:check` | Pass |
| `task browser:test` | 446 passed, 2 skipped. The two skips are the opt-in live ngrok journeys in `connectrpc-player.spec.mjs`, which need `FALLOUT_PUBLIC_TEST_*` settings |

The first `task test` failed on two pins in `internal/platform/assets_test.go`, the descriptor shape and the `schema-revision.txt` digest. They are meant to change with an intended schema change, as in 040 and 041, and were updated to the new values. The first `task test:race` also failed once in `TestEmbeddedNgrokSDKOptInAuthenticatedGeneratedSubscribe/missing`: the live ngrok edge answered 404 where the test expects 401. `internal/tunnel` is unchanged from `develop`. The test passed in three separate reruns and in the full rerun.

## Success criteria

| Criterion | Evidence |
|---|---|
| SC-001 | `facility-play-feed.spec.mjs`: `reads a fault blocks are fed once to the fault device with an attempt count`. Five refused reads of «СВОДКА ЭВАКУАЦИИ» leave one entry ×5 on «Контур вентиляции теплицы». The packaged-app check is T029 |
| SC-002 | `facility-diagnostics.spec.mjs`: the five capability scenarios and `the terminal-fault switch clears a fault while every player capability is blocked` expect the notice `ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.` In Go, `TestFaultBlockedPlayerRequestsRefuseWithFaultReasonAndFeedTheFaultDevice` refuses every capability with `fault-blocked`, not `invalid-action` |
| SC-003 | `facility-device-templates.spec.mjs`: the demo door adds nothing and its apply button says `ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ` as soon as the dialog opens. A door with neither state gets 2 states and 2 transitions. `a filled cell counts as present whatever its transition is called` covers a partly filled door |
| SC-004 | With the block check in `internal/control/service.go` turned off by a scratch edit, 7 of the 13 diagnostics journeys failed: the five capability scenarios, the terminal-fault switch journey and `a blocked entry stays closed until the Overseer switches its fault off`. The 6 that still passed are effects and recoveries that no capability block decides. With the check on, all 13 pass |
| SC-005 | `facility-device-list.spec.mjs`: `the terminal-fault group collapses with its active count and frees room for seven rows at 1440×900` measures at least 7 whole rows at a 1440×869 viewport, which is 1440×900 minus the title bar. Row padding, the list gap and the sidebar gap were tightened to fit them |
| SC-006 | `facility-device-list.spec.mjs`: `a filtered row and a focused row are drawn whole`. Before the fix, the one-row search result had 0 whole rows: the list shrank with the expanded fault group. The list now keeps a 120 px minimum height |
| SC-007 | `specs/033-entry-content-state/tasks.md` has 0 unchecked tasks. The evidence is in `specs/033-entry-content-state/validation.md` |
| SC-008 | The full browser suite passes (above). The packaged-app part is T029 |

## Packaged app (T029)

The app was built with `task package` from this branch. It was opened at exactly 1440×900 through Accessibility on copies of `sessions/demo.json` from `develop`, which this feature does not change. A headless player was connected through `http://127.0.0.1:3690` after the warden terminal went on air.

| Check | Result |
|---|---|
| FR-024, version 1 round trip | PASS. Switching «Споровое загрязнение воздушного контура» off and on again saved the file. It stayed `version: 1`, kept its size of 45128 bytes and differed from the original only in `facility.revision` (12 → 14). Nothing new is stored |
| SC-001 | PASS after the fix below. Five reads of «СВОДКА ЭВАКУАЦИИ» each showed `ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.` with no fault name, and the entry stayed closed. «Контур вентиляции теплицы» showed one entry: `чтение записи «СВОДКА ЭВАКУАЦИИ» · запрос игрока отклонён: мешает неисправность «Споровое загрязнение воздушного контура» · Терминал смотрителя — Убежище 76 · ×5`. With the fault switched off, the same read opened the entry. The runtime log holds five `fault-blocked` outcomes with `capability=view-entry`, `condition_ids=[air-spore-bloom]` and `entry_id=n_entry_vault_shutdown` |
| SC-003 | PASS. The demo door's dialog opened with `ПРИМЕНИТЬ ШАБЛОН` disabled and `ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ` as its help and visible reason. Pressing it added nothing |
| SC-005 | PASS. With «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» collapsed (`▸`, `АКТИВНО 2`), 9 of the 10 device rows were whole in the list. Expanded, 5 were |
| SC-006 | PASS. With the group expanded, a search leaving one row drew that row whole, inside a list of 121 points |

### Defect found and fixed: the feed stopped counting at ×2

The first packaged run showed `×2` after five refusals, although the runtime log held all five. A fault-block refusal changed the feed but kept the coordination revision, and the Overseer drops a state at a revision it has already seen (`applyCoordinationState`). The first two refusals came through only because cursor presentations moved the revision with them. The browser journey could not catch this: its fixture moves its revision whenever the feed changes.

`rejectPlayerAction` in `internal/control/service.go` now commits a new revision whenever a refusal records a feed entry, publishes the state ahead of the result, and gives the result that revision. The same fix covers 040's refusals at request time, such as a missing transition, which had the same gap. A refusal that feeds no device keeps the revision. Three Go tests assert this: `TestFaultBlockedPlayerRequestsRefuseWithFaultReasonAndFeedTheFaultDevice` (now including a repeat), `TestPlayerCommandFeedRecordsRefusalReasons` and `TestActiveDiagnosticConditionsBlockSelectedCapabilities`. With the revision step turned off, 9 of their cases fail. After the fix, the packaged run above showed `×5`, and the logged request revisions advanced per refusal (10, 13, 14, 15, 16).

After the fix, `go fix ./...` made no edits. `task fmt:check`, `task vet`, `task lint`, `task test`, `task test:race`, `task wails:pins:check` and `task secret-leak:check` passed again, and `task browser:test` passed again with 446 passed and 2 skipped.
