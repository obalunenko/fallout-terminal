# Quickstart: Validate Unified Facility Authoring

## Prerequisites

- Branch `039-unified-facility-authoring` (stacked on `038-facility-authoring-p0-fixes`), Node.js
  from `.nvmrc` (`nvm use`), and the Go toolchain.
- A writable copy of `sessions/demo.json` outside the repository, made through the app's demo-copy
  action, for the interactive checks. The bundled demo is read-only.
- Two player browsers on the local player URL for the live checks.

## Automated checks

```bash
go fix ./...                       # review every edit; keep only intentional modernization
task proto:generate                # then refresh the compatibility baseline (contracts/private-desktop.md, Migration)
task proto:check
task proto:breaking
task bindings:check
task fmt:check
task vet
task lint
task test
task test:race
task frontend:build
task browser:test -- facility-unified-save.spec.mjs facility-command-lists.spec.mjs facility-preview-diff.spec.mjs
task browser:test                  # full suite, including updated facility, state-changing, and demo specs
task package
```

Expected: everything passes. `rg` finds none of the removed method names or copy
(`contracts/private-desktop.md`, `contracts/overseer-ui.md`) outside `specs/`.

## Interactive checks (`task dev` or the packaged app, demo copy open)

1. **One save model (US1)**
   1. ОБЪЕКТЫ → «Гермодверь Убежища 76» → rename it to «Гермодверь 76» → ГОТОВО. The bar shows
      «СОХРАНЕНО · …» on both tabs.
   2. ТЕРМИНАЛЫ → «Открыть выход и подать сигнал» → change the result text → apply. Back on
      ОБЪЕКТЫ, the rename is still there, and no message about a changed session appears.
   3. Press ОТМЕНИТЬ twice: the result text, then the name, return to their originals.
   4. Press ПОВТОРИТЬ once: the rename returns.
   5. Type in the command form without applying, then switch tabs and back: the input is still
      there, and the bar shows «НЕ ПРИМЕНЕНО: Команда «…»».
2. **Live never rejects (US1, FR-007)**
   1. Start a broadcast on «Терминал смотрителя — Убежище 76».
   2. Have a player request a state-changing command. While it is pending, rename an unrelated
      device: the request stays pending.
   3. Approve it, then edit any terminal text: the edit saves, and players see it at once with their
      position and hacking progress kept.
3. **Command lists (US2)**
   1. Open «Открыть выход и подать сигнал». «ПРИ ОДОБРЕНИИ» lists the door and alarm transitions,
      «Выход открыт», and block 2 of «СВОДКА ЭВАКУАЦИИ».
   2. «ЧТО ВИДЯТ ИГРОКИ» lists the door-state name «Выход уже открыт», marked to override the
      completed name, the power-grid availability rule, and the block rules.
   3. Approve the command as a player: the marked rules move to «ВИДЯТ ИГРОКИ», and the tree shows
      «Выход уже открыт», exactly as the player sees it.
4. **Scene reset (US2)** — СБРОСИТЬ СЦЕНУ. The dialog counts the changed devices, faults, and
   executed commands, and says players stay on their terminal. After you confirm, every device and
   fault is at its starting value, no command is executed, and players are still on the same
   terminal.
5. **Preview (US3)**
   1. Select «Гермодверь Убежища 76» → ПРЕДПРОСМОТР СОСТОЯНИЯ on «Терминал смотрителя —
      Убежище 76». «Открыта» is preselected, «СЕЙЧАС: Заперта» and «ЕСЛИ: Открыта» are side by
      side, and the summary names block 3 of «СВОДКА ЭВАКУАЦИИ», the name «Выход уже открыт», and
      the appearing «НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН».
   2. Change the door text rule → ГОТОВО → reopen the preview: it shows the new text.
   3. Closing the preview changes nothing for the players.

## Compatibility check

Open a copy of every file in `sessions/`, apply a no-op save (the Go test `SaveAuthoring` round
trip covers this automatically), and compare. The JSON is byte-identical apart from intended
formatting, and player projections are identical (SC-007).

Record commands, results, and anything `NOT RUN` in `validation.md`.
