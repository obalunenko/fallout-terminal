# Validation: Facility Authoring P0 Fixes

Date: 2026-10-29 · Branch: `038-facility-authoring-p0-fixes` (from `035-facility-state-system` at 4d5984b)
· macOS, Node.js 26.8.1 from `.nvmrc`.

## Baseline (before changes)

| Command | Result |
|---|---|
| `task frontend:build` | PASS |
| `task browser:test -- facility-authoring.spec.mjs facility-authoring-ux.spec.mjs facility-diagnostics.spec.mjs facility-lifecycle.spec.mjs demo-session.spec.mjs` | PASS, 49/49 |

## Tests written first

| Spec | Before implementation | After |
|---|---|---|
| `facility-p0-command-actions.spec.mjs` | 4 failed, 1 passed (executed-command lock is existing behavior) | 5/5 pass |
| `facility-p0-identifiers.spec.mjs` | 2 failed (`device-device`, no row errors) | 2/2 pass |
| `facility-p0-fault-activity.spec.mjs` | 2 failed, 1 passed (legacy default) | 3/3 pass |
| `facility-p0-visible-fields.spec.mjs` | Written after T002; rerun with the global rule removed: 2 failed (`БЛОК ЗАПИСИ`, `ГРУППА НАЗНАЧЕНИЯ`) | 3/3 pass |

## Final automated gates

| Command | Result |
|---|---|
| `task frontend:build` | PASS (exit 0) |
| `task browser:test` (full suite, 290 tests in 24 files) | PASS (exit 0) |
| `task fmt:check` | PASS (exit 0) |
| `task vet` | PASS (exit 0) |
| `task test` | PASS (exit 0) |
| `task lint` | PASS, 0 issues |

`go fix ./...` was not run: the change contains no Go source edits.

## Label sweep coverage (FR-019, FR-020)

Opened through real routes in every kind, scope, or mode: binding (5 kinds × 3 node types), fault
(device and terminal scope), device (new with rows, existing with expanded rules), recovery program
(with and without extra transitions), device and condition preview, dependencies, repair, device
reset, facility reset, recovery confirmation, create terminal, logical sessions, terminal group draft,
entry block command (both modes), command state reset. Every other dialog in `index.html`
(application update ×2, terminal group impact, player management, player delete, take off air, end
broadcast, terminal switch, public access settings, provider token, player credentials, generated
password) is opened in its default state.

## Findings during implementation

- The server already rejects state names that repeat within a device (case-insensitive,
  `internal/domain/validate.go:671–675`). The spec assumed otherwise; the device dialog now reports a
  repeated name on its row before saving, and the spec, data model, contract, and quickstart were
  corrected.
- Existing faults can be opened for editing only through **ЗАВИСИМОСТИ → Область неисправности**, and
  terminal-scoped faults cannot be opened at all. Out of scope here; belongs to feature 041 (F-12).

## Packaged application (T020)

`task package` succeeded. The packaged app was driven through macOS Accessibility on a fresh copy of
`sessions/demo.json`, without injecting code into the page.

| Quickstart step | Observed in the packaged app |
|---|---|
| 1. Readable identifiers | «Дверь в убежище» → `device-dver-v-ubezhishche`; «Заперта», «Открыта», «Открыта!» → `zaperta`, `otkryta`, `otkryta-2`; renaming the third to «открыта» shows «НАЗВАНИЕ УЖЕ ИСПОЛЬЗУЕТСЯ: открыта» on that row with focus there |
| 2. Mode decision | `n_cmd_open_evacuation` switched to ОБЫЧНАЯ КОМАНДА: inline notice lists both transitions, ПРИМЕНИТЬ disabled; ОСТАВИТЬ restores ИЗМЕНЯЕТ СОСТОЯНИЕ and hides the notice |
| 3. Draft guard | Door renamed in the draft: the command form shows the guard and locks mode and apply; ОТМЕНИТЬ ЧЕРНОВИК unlocks it |
| 4. Texts for an ordinary command | Binding dialog, ДЕЙСТВИЕ КОМАНДЫ on «Проверить наружные датчики»: НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ and ЗАПРОС ПОДТВЕРЖДЕНИЯ shown empty; no orphan labels |
| 5. Starting activity | Fault dialog shows ИСПРАВНО (selected) and НЕИСПРАВНОСТЬ АКТИВНА |
| 6. No orphan labels | Confirmed in the binding dialog above; full sweep is automated |

Round trip: applying an unchanged command made the app rewrite the demo copy. Compared with
`sessions/demo.json`: device, state, and transition IDs (10 devices), fault activities (7), recovery
programs (2), and command modes with device actions (13 commands) are identical.

Screenshots of these screens are stored outside the repository with the review evidence.
