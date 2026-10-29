# Quickstart: Validate Facility Authoring P0 Fixes

## Prerequisites

- Branch `038-facility-authoring-p0-fixes`, Node.js from `.nvmrc` (`nvm use`), Go toolchain.
- A writable copy of `sessions/demo.json` outside the repository for interactive checks.

## Automated checks

```bash
task frontend:build
task browser:test -- facility-p0-command-actions.spec.mjs facility-p0-identifiers.spec.mjs facility-p0-fault-activity.spec.mjs facility-p0-visible-fields.spec.mjs
task browser:test -- facility-authoring.spec.mjs facility-authoring-ux.spec.mjs facility-diagnostics.spec.mjs facility-lifecycle.spec.mjs demo-session.spec.mjs
task vet
task test
task lint
```

Expected: all pass. Go checks are regression gates only; no Go source changes are planned.

## Interactive checks (packaged or `task dev`)

Open the demo copy with **ОТКРЫТЬ СЕССИЮ**.

1. **Readable identifiers (US2)** — ОБЪЕКТЫ → + УСТРОЙСТВО. Name «Дверь в убежище», type ДВЕРЬ, add
   states «Заперта», «Открыта», «Открыта!», initial «Заперта», transition «Открыть дверь».
   Expected IDs: `device-dver-v-ubezhishche`, `zaperta`, `otkryta`, `otkryta-2`, `otkryt-dver`.
   Clear the third state's name and apply: only that row shows «УКАЖИТЕ НАЗВАНИЕ СОСТОЯНИЯ» and has
   focus. Type «открыта» there: the row shows «НАЗВАНИЕ УЖЕ ИСПОЛЬЗУЕТСЯ». Restore «Открыта!», apply,
   save, reopen: the IDs are unchanged.
2. **Mode decision (US1)** — ТЕРМИНАЛЫ → Терминал смотрителя — Убежище 76 → ПРОТОКОЛ ВЫХОДА →
   Открыть выход и подать сигнал. Set РЕЖИМ КОМАНДЫ to ОБЫЧНАЯ КОМАНДА: the notice lists both
   transitions and ПРИМЕНИТЬ is disabled. Choose ОСТАВИТЬ: nothing changes. Repeat and choose
   УДАЛИТЬ ДЕЙСТВИЕ И ПЕРЕКЛЮЧИТЬ, then ПРИМЕНИТЬ: ОБЪЕКТЫ → Гермодверь no longer lists the command
   under «Что делают игроки». (Use a fresh demo copy afterwards.)
3. **Draft guard (US1)** — ОБЪЕКТЫ: rename any device and apply to the draft without saving.
   Return to the same command: the mode selector and ПРИМЕНИТЬ are disabled with the draft notice.
   ОТМЕНИТЬ ЧЕРНОВИК unlocks the form.
4. **Texts for an ordinary command (US1)** — ОБЪЕКТЫ → ДОБАВИТЬ ПРИВЯЗКУ → ДЕЙСТВИЕ КОМАНДЫ, pick an
   ordinary command: НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ and ЗАПРОС ПОДТВЕРЖДЕНИЯ are empty and required.
5. **Starting activity (US3)** — ОБЪЕКТЫ → ДОБАВИТЬ УСЛОВИЕ, choose НЕИСПРАВНОСТЬ АКТИВНА, apply,
   save: the row reads «СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: АКТИВНА». Edit «Резервная сеть обесточена»: the
   radio shows its stored starting activity. СБРОСИТЬ ВЕСЬ ОБЪЕКТ restores starting values.
6. **No orphan labels (US4)** — open the binding dialog and switch all five kinds; open the fault
   dialog and switch УСТРОЙСТВО/ТЕРМИНАЛ: every visible label has its control.

## Compatibility check

Open `sessions/demo.json` (a copy), save without edits, and compare: identifiers, command modes,
device actions, and fault activities are byte-identical.

Record commands, results, and anything `NOT RUN` in `validation.md`.
