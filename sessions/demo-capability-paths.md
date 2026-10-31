# Demo capability paths

This inventory is the review and acceptance map for `sessions/demo.json` and
`sessions/demo-players.json`. It describes only behavior that the version-1 demo can actually
initialize or exercise. Packaging, updates, credential storage, and public-tunnel creation are
deliberately outside this inventory.

## Starting contexts

1. Open `sessions/demo.json` in Overseer. The linked roster is
   `sessions/demo-players.json`.
2. The **Overseer** selects a terminal, starts the broadcast, decides pending commands, edits or
   previews facility state under **ОБЪЕКТЫ**, and performs confirmed resets or private recovery.
3. The first joined player who chooses a character is the **controller**. The controller navigates,
   hacks, and requests commands; a second joined player is an **observer** and receives the same
   authoritative terminal, menu, result, facility projection, and role changes without being able
   to drive them.
4. Unless a row says otherwise, return with **НАЗАД** until the terminal root is visible. A command
   result returns with its acknowledgement control; rejecting a state-changing command leaves the
   same command reachable and changes no facility state.
5. Authoring has one save model on both tabs. Every applied edit, on **ТЕРМИНАЛЫ** or
   **ОБЪЕКТЫ**, is saved at once and reaches players on live terminals at once; facility dialogs
   confirm with **ГОТОВО**. The shared bar next to the tabs shows one save status, **ОТМЕНИТЬ** and
   **ПОВТОРИТЬ** for authoring edits from either tab, and **СБРОСИТЬ СЦЕНУ**. **ПРЕДПРОСМОТР**
   compares the current view with a hypothetical state of the applied configuration. During
   play, the selected device's **ВО ВРЕМЯ ИГРЫ** card changes live state (see "Play controls on
   the device card"); **СБРОСИТЬ СЦЕНУ** restores the whole scene.

## Terminal installations and access

Every terminal belongs to exactly one group. A multi-member group is one machine with different
access contexts; each singleton group is a separate machine, even when it is in the same building.
To start any player row, the Overseer selects the listed terminal and starts or replaces the
broadcast. Levels 1–5 lead through the normal character selection and hacking flow; level 0 opens
without a hack.

| Group / installation | Terminal and access context | Hack | Narrative starting point |
|---|---|---:|---|
| `vault-76-overseer-console` — Центральный терминал Смотрителя 76 | `t_demo1` — local Overseer console | 0 | Day of Reclamation local console; no hack |
| `vault-76-overseer-console` — same console | `t_demo2` — emergency remote mirror | 5 | Reach from `t_demo1` through **ПРОТОКОЛ ВЫХОДА → Открыть аварийный канал этого пульта**, or select it directly to exercise level 5 |
| `vault-76-greenhouse` — independent greenhouse terminal | `t_demo_hack_1` — agronomist console | 1 | Select terminal; complete level-1 hack |
| `vault-76-freight-lift` — independent lift terminal | `t_demo_hack_2` — lower machine-room console | 2 | Select terminal; complete level-2 hack |
| `vault-76-outer-security` — independent security terminal | `t_demo_hack_3` — exterior guard post | 3 | Select terminal; complete level-3 hack |
| `atlas-76-relay` — independent remote-site terminal | `t_demo_hack_4` — ATLAS-76 relay | 4 | Select terminal; complete level-4 hack |

The ordered transition pair `n_cmd_state_change_1` (`t_demo1` → `t_demo2`) and `n2_cmd_return`
(`t_demo2` → `t_demo1`) changes local/remote access to the same console. No authored transition
crosses into an independent group.

## Player-visible content and command paths

Each path starts at the terminal root after its access step above. A folder row demonstrates nested
navigation; an entry row demonstrates either legacy `description` content or block-based content.
The controller performs actions while the observer verifies the same authoritative result.

| Stable node | Mode | Exact menu route | Prerequisite and expected outcome | Return / recovery |
|---|---|---|---|---|
| `n_folder_directives` | folder | `t_demo1`: **ДИРЕКТИВЫ VAULT-TEC** | Opens a nested menu. | **НАЗАД** |
| `n_entry_vault_parameters` | description entry | `t_demo1`: **ДИРЕКТИВЫ VAULT-TEC → ПРОТОКОЛ ВОЗРОЖДЕНИЯ** | Shows the Reclamation directive as one legacy description. | **НАЗАД** |
| `n_entry_vault_shutdown` | block entry + state variants | `t_demo1`: **ДИРЕКТИВЫ VAULT-TEC → СВОДКА ЭВАКУАЦИИ** | Shows multiple ordered blocks. Powering/opening the facility changes bound block text; completed `n_cmd_1` also demonstrates persisted EntryContent in `b_vault_shutdown_heading`. | **НАЗАД**; reset affected commands/devices in Overseer for the initial form |
| `n_entry_open_route` | conditionally visible entry | `t_demo1`: **ДИРЕКТИВЫ VAULT-TEC → НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН** | Initially hidden while `vault-door-main=sealed`; approve `n_cmd_open_evacuation` to make it visible. | **НАЗАД**; reset `vault-door-main` to hide it again |
| `n_folder_facility` | folder | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА** | Opens shared facility status and diagnostics. | **НАЗАД** |
| `n_entry_facility_status` | block entry + five bindings | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → СОСТОЯНИЕ УБЕЖИЩА** | Shows bound text for `power-grid-main`, `ventilation-main`, `reactor-main`, `robot-pod-atrium`, and `water-purifier-76`; initial/current deltas are already present. | **НАЗАД**; device or scene reset restores authored initial variants |
| `n_folder_diagnostics` | conditionally visible folder | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → АВАРИЙНАЯ ДИАГНОСТИКА** | Set or reset `power-grid-main=offline`; folder appears and exposes diagnostic entries. | **НАЗАД**; restore the grid to hide it |
| `n_diag_grid` | diagnostic entry | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → АВАРИЙНАЯ ДИАГНОСТИКА → НЕТ ПИТАНИЯ** | With the grid offline, explains the authored recovery transition. | **НАЗАД**; approve `n_cmd_prime_grid` or private recovery |
| `n_diag_door` | diagnostic entry | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → АВАРИЙНАЯ ДИАГНОСТИКА → ПРИВОД ГЕРМОДВЕРИ НЕ ОТВЕЧАЕТ** | Activate `door-offline` and keep the grid offline; shows its diagnostic and command block. | **НАЗАД**; `vault-door-main/open` or private recovery |
| `n_folder_actions` | folder | `t_demo1`: **ПРОТОКОЛ ВЫХОДА** | Opens ordinary, persistent, facility, recovery-program, and terminal-transition commands. | **НАЗАД** |
| `n_cmd_1` | persisted legacy state change + EntryContent | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Активировать маршрут выхода** | The shipped snapshot is already completed: completed label/result and `b_vault_shutdown_heading` persist. Reset its state in Overseer, request as controller, approve or reject as Overseer, then reopen **СВОДКА ЭВАКУАЦИИ**. | Acknowledge; **Сбросить состояние** restores the authored command form |
| `n_cmd_prime_grid` | persisted facility state change | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Запустить резервную сеть** | The shipped snapshot records completion and `power-grid-main=online`. Reset command state and the grid, request, then approve; the one-device recovery transition restores power and clears `grid-unpowered`. | Acknowledge; single-device reset returns the grid to `offline` |
| `n_cmd_open_evacuation` | atomic multi-device + EntryContent | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Открыть выход и подать сигнал** | Requires `power-grid-main=online`. Approval atomically opens `vault-door-main`, sets `evacuation-alarm=evacuation`, changes `b_vault_shutdown_body`, changes the command label, and reveals `n_entry_open_route`. | Acknowledge; reset the two devices and command state |
| `n_cmd_air_recovery` | recovery-program command | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Загрузить кассету очистки воздуха** | With `ventilation-main=contaminated` and `air-spore-bloom` active, approval runs `air-recovery-76`, purges ventilation, clears the condition, and changes bound air text. | Acknowledge; reset ventilation or the scene |
| `n_cmd_ordinary_1` | ordinary command | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Проверить наружные датчики** | Immediately shows the authored sensor report; no approval or persistent mutation. | Acknowledge |
| `n_cmd_state_change_1` | terminal transition | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Открыть аварийный канал этого пульта** | Overseer approval changes the active view to the same console's remote level-5 context `t_demo2`; observer follows the same authoritative transition. | `t_demo2` root → **Вернуться к локальному пульту** |
| `n2_entry_channel` | block entry + record substitution | `t_demo2`: **КАНАЛ СМОТРИТЕЛЯ 76/ОМЕГА** | Shows the remote-mirror record. Activate `network-isolated` to substitute corrupted `b2_network_status`; connect the network for its normal bound variant. | **НАЗАД**; run `network-recovery-76` or private recovery |
| `n2_folder_diagnostics` | conditionally visible folder | `t_demo2`: **СЕТЕВАЯ ДИАГНОСТИКА** | Set `network-overseer=connected`; folder becomes visible. | **НАЗАД**; reset/isolate network to hide it |
| `n2_diag_network` | diagnostic entry | `t_demo2`: **СЕТЕВАЯ ДИАГНОСТИКА → КАНАЛ ИЗОЛИРОВАН** | Shows the authored program and private recovery choices when its folder is visible. | **НАЗАД** |
| `n2_cmd_network_recovery` | recovery-program command | `t_demo2`: **Запустить СЕТЬ-76** | With `network-overseer=isolated` and `network-isolated` inactive (the shipped current snapshot), approval runs `network-recovery-76`, connects the network, and restores normal record text. If the condition is activated, it deliberately blocks this player program and requires its private escape path. | Acknowledge; reset network to `isolated` |
| `n2_cmd_trace` | ordinary command | `t_demo2`: **Проследить последний пакет** | Immediately reveals the RS-04 narrative lead. | Acknowledge |
| `n2_cmd_return` | reciprocal terminal transition | `t_demo2`: **Вернуться к локальному пульту** | Overseer approval returns every participant to `t_demo1`, preserving the group's local/remote identity. | `t_demo1` root |
| `n3_entry_crop` | description entry | `t_demo_hack_1`: **ПОСЛЕДНИЙ УРОЖАЙ** | After the level-1 hack, shows the greenhouse hand-off record. | **НАЗАД** |
| `n3_entry_air` | block entry + state variant | `t_demo_hack_1`: **ВОЗДУШНЫЙ КОНТУР** | Shows contaminated text, then the filtered variant after `air-recovery-76`. | **НАЗАД**; reset ventilation |
| `n3_diag_display` | conditionally visible diagnostic | `t_demo_hack_1`: **НЕСТАБИЛЬНОСТЬ ЭКРАНА** | Set `ventilation-main=filtered`; activate or preview `greenhouse-display-unstable` to demonstrate diagnostic-path and display-instability effects. | **НАЗАД**; private recovery or scene reset |
| `n3_cmd_read_sensors` | ordinary command | `t_demo_hack_1`: **Снять показания спор** | Immediately shows greenhouse sensor readings. | Acknowledge |
| `n4_entry_manifest` | block entry + state variant | `t_demo_hack_2`: **ГРУЗОВАЯ ВЕДОМОСТЬ** | After the level-2 hack, the shipped `freight-elevator=upper` state shows the upper-level block variant. | **НАЗАД**; reset lift for its lower variant |
| `n4_cmd_call_lift` | facility state change | `t_demo_hack_2`: **Вернуть кабину вниз** | From `freight-elevator=upper`, approval applies `freight-elevator/lower` and changes the manifest and completed label. | Acknowledge; reset lift to authored `lower` or edit it back to `upper` to repeat |
| `n5_entry_security` | block entry + state variant | `t_demo_hack_3`: **ПЕРИМЕТР** | After the level-3 hack, shows `security-turret=safe`; setting it to `tracking` changes the bound status. | **НАЗАД**; reset turret |
| `n5_diag_auth` | conditionally visible diagnostic | `t_demo_hack_3`: **ТАБЛИЦА ДОПУСКА ПОВРЕЖДЕНА** | Set `security-turret=tracking` and activate or preview `guard-authorization-corrupted`; demonstrates the `hack` block and diagnostic path. | **НАЗАД**; private recovery or scene reset |
| `n5_cmd_scan` | ordinary command | `t_demo_hack_3`: **Сканировать дорогу** | Immediately shows the exterior route scan. | Acknowledge |
| `n6_entry_archive` | block entry + active record substitution | `t_demo_hack_4`: **ПОСЛЕДНЯЯ ПЕРЕДАЧА** | After the level-4 hack, shipped `relay-storage-damaged` substitutes corrupted archive text and activates display instability. | **НАЗАД**; private recovery removes the condition |
| `n6_entry_robot` | block entry + state variant | `t_demo_hack_4`: **КУРЬЕРСКИЙ ПОСТ** | Shipped `robot-pod-atrium=patrol` shows its route variant; reset shows the charging text. | **НАЗАД** |
| `n6_cmd_decode` | ordinary command | `t_demo_hack_4`: **Повторить повреждённую передачу** | Immediately advances the RS-04 failure narrative. | Acknowledge |

## Facility authoring, diagnostics, preview, and reset

### Prepare a story with the in-app guide

Open **ОБЪЕКТЫ → КАК СОБРАТЬ СЦЕНАРИЙ**. The guide is open while the session has no devices.
Once it has one, the guide is one line, for example **КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 1 ИЗ 5: ОБЪЕКТ**,
so the device controls stay in view. **ПОКАЗАТЬ** opens it and **СКРЫТЬ** closes it; that choice
holds until the application closes. Select a device on the left to make the guide use that device. The counts describe configured relationships, not
a guarantee that the scenario is correct.

Use one continuous example: players encounter a locked door, request that it open from a
terminal, and see a status record change to **Путь свободен**.

| Step | Story decision | Configuration route |
|---|---|---|
| **1. Объект** | What can change? A door starts **Заперта** and can become **Открыта**. | Create a door device, its two named states and initial state. Add the **Открыть дверь** transition from closed to open. Russian names produce readable identifiers on their own (**Заперта** → `zaperta`, **Открыть дверь** → `otkryt-dver`); a missing or repeated state name is marked on its own row. Applying selects the device in the guide. |
| **2. Действие** | What do players do? They request **Открыть дверь** on a terminal. | Use **СВЯЗАТЬ С КОМАНДОЙ** or edit an existing command action; choose the door and opening transition. An ordinary command becomes state-changing, so the editor asks for **НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ** and **ЗАПРОС ПОДТВЕРЖДЕНИЯ**; nothing is filled in for you. If there is no suitable command, the guide routes to **ТЕРМИНАЛЫ** to create one. A transition alone does not create a player command. |
| **3. Результат** | How do players notice the change? A status record says **Путь свободен**. | Open the display-rule editor; choose the terminal, entry, block, and open state. Keep **Дверь заперта** as the block's original text and use **Путь свободен** for the open-state variant. Name, visibility, and availability rules are alternative display choices. |
| **4. Осложнения** | Optional: what obstructs the scene and how is it repaired? | Add a fault and choose its blocking effect and whether it is active **В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА**; activate/remove it in transition effects. Operator recovery is a manual escape path. A recovery program combines transitions; return to **2. Действие**, open the command action, and choose that program under **ЧТО ЗАПУСТИТЬ**. |
| **5. Проверка** | Does the scene show the expected result? | Every edit is already saved. Preview the open state on the relevant terminal, then request the actual command as a player and approve it as Overseer. Preview checks presentation without performing the action. |

If a command or display destination is missing, use **ОТКРЫТЬ ТЕРМИНАЛЫ**, create the command or
entry, then return to **ОБЪЕКТЫ** to bind it to the selected device. Nothing is lost by switching
tabs: applied edits are saved, and unapplied input in the node form or terminal settings stays
and is listed in the shared bar as **НЕ ПРИМЕНЕНО: …**. Merely moving between guide
steps never saves anything.

Prerequisites are checked against the state **before** a command or recovery program runs.
For example, if opening the door requires power already on, restoring power and opening that
door cannot satisfy the prerequisite within one atomic program: restore power first with a
separate action, then request the opening action. Use faults/programs only if the story needs
them; a basic door scene does not require either.

The device's tabs distinguish **Что делают игроки** (command actions) from **Что видят игроки**
(display and availability rules), next to **Неисправности** and **Программы**; each tab shows its
count. Both retain contextual edit/remove controls. The guide adds no
content or saved progress merely by moving between steps.

### Autosave, undo, and redo across both tabs

Open a copy of the demo. On **ОБЪЕКТЫ**, rename **Гермодверь Убежища 76** in **ИЗМЕНИТЬ СОСТОЯНИЯ И
ПЕРЕХОДЫ** with **ГОТОВО**; on
**ТЕРМИНАЛЫ**, edit **ПРОТОКОЛ ВЫХОДА → Открыть выход и подать сигнал** and **ПРИМЕНИТЬ**; return to
**ОБЪЕКТЫ**. Both edits are saved, the shared bar reads **СОХРАНЕНО · <time>** on both tabs, and no
stale or "session changed" message exists. Press **ОТМЕНИТЬ** twice: the command edit, then the
rename, are reversed and saved, newest first; device states, faults, and executed commands are
never reversed. Press **ПОВТОРИТЬ** once to bring the rename back; any new edit clears what is left
to redo. A control that cannot run says why, for example **НЕЧЕГО ОТМЕНЯТЬ** or
**ДОЖДИТЕСЬ СОХРАНЕНИЯ**. If another change makes an edit stale, the bar offers
**ПРИМЕНИТЬ СНОВА**.

### Inspect and maintain existing scenes

The Overseer reaches every row from **ОБЪЕКТЫ** after opening the demo. Selecting an item exposes
its readable current/initial value and related rules beside the searchable device list. Technical
IDs remain under **Технические идентификаторы**. Device graphs summarize each transition as
source → destination; expand **ПРЕДУСЛОВИЯ И ЭФФЕКТЫ** only when editing advanced rules, including
the explicit **АКТИВИРОВАТЬ НЕИСПРАВНОСТЬ** / **СНЯТЬ НЕИСПРАВНОСТЬ** choice.

Fault rows read **СЕЙЧАС: … · В НАЧАЛЕ: …**. Editing a fault's starting activity changes only what a
reset restores; a new fault starts in the chosen activity. For example, open
**ОБЪЕКТЫ → Контур вентиляции теплицы → ЗАВИСИМОСТИ → Область неисправности → ОТКРЫТЬ** to edit `air-spore-bloom`.

Under **ТЕРМИНАЛЫ**, the **ДЕЙСТВИЕ УСТРОЙСТВ** block appears only in **ИЗМЕНЯЕТ СОСТОЯНИЕ** mode.
Switching `n_cmd_open_evacuation` to another mode shows the transitions it would lose and asks to keep
the mode or remove the action; **ПРИМЕНИТЬ** stays disabled until you choose.

Selecting `n_cmd_open_evacuation` also shows two lists. **ПРИ ОДОБРЕНИИ** names every change approval
makes: **Гермодверь Убежища 76: Заперта → Открыта**, **Сигнал эвакуации: Тишина → Эвакуация**, the
name after execution **Выход открыт**, and the new text of block 2 of **СВОДКА ЭВАКУАЦИИ**.
**ЧТО ВИДЯТ ИГРОКИ** lists, per target, every rule for the name, availability, and the rewritten block,
lowest precedence first, and marks the one players see with **ВИДЯТ ИГРОКИ**. With the door open,
the state name **Выход уже открыт** shows **ПЕРЕКРЫВАЕТ: ИСХОДНОЕ НАЗВАНИЕ, НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ**;
availability depends on `power-grid-main=online`, and an active `grid-unpowered` outranks it.
**ИЗМЕНИТЬ** opens the rule's own editor, the same one **ОБЪЕКТЫ** uses. The terminal tree shows each
name as players see it now, with the authored name in its tooltip, and marks **СКРЫТО ОТ ИГРОКОВ** and
**НЕДОСТУПНО**.

Device, binding, and recovery-program dialogs save on **ГОТОВО**; a rejected save reopens the dialog
with its input and the reason. Binding and program rows provide **Изменить** and **Удалить** actions. **ЗАВИСИМОСТИ → ОТКРЫТЬ** follows readable references to their command, entry
block, device transition, condition, or recovery program. Selecting a command under
**ТЕРМИНАЛЫ** also shows **ДЕЙСТВИЕ УСТРОЙСТВ** and opens that command's preselected action editor.

**ПРЕДПРОСМОТР** resolves automatically when opened or when its terminal/state/condition changes,
after any save in progress. It preselects the first state different from the current one (the
opposite activity for a fault), shows **СЕЙЧАС** and **ЕСЛИ** side by side, counts the differences,
names each one (**ПЕРЕИМЕНУЕТСЯ**, **ТЕКСТ ИЗМЕНИТСЯ**, **ПОЯВИТСЯ**, **ИСЧЕЗНЕТ**, **СТАНЕТ ДОСТУПНО**,
**СТАНЕТ НЕДОСТУПНО**) and marks it in both trees, and never changes players or revisions.
**ОБНОВИТЬ ПРЕДПРОСМОТР** remains a retry action.

| Capability | Stable assets | Exact Overseer route and expected outcome | Recovery |
|---|---|---|---|
| All ten device kinds | `power-grid-main` (`power-grid`), `vault-door-main` (`door`), `evacuation-alarm` (`alarm`), `network-overseer` (`network-segment`), `ventilation-main` (`ventilation`), `reactor-main` (`reactor`), `security-turret` (`turret`), `robot-pod-atrium` (`robot-pod`), `freight-elevator` (`elevator`), `water-purifier-76` (`custom:water-purifier`) | **ОБЪЕКТЫ → Поиск устройств → [device] → ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ**. Inspect readable transition summaries and expand advanced rules; **ГОТОВО** saves the complete graph. **ЗАВИСИМОСТИ** lists every command, binding, condition, and recovery reference before destructive edits. | **ОТМЕНА** leaves the form untouched; **ОТМЕНИТЬ** on the shared bar reverses a saved edit |
| Direct single-device transition | `n_cmd_prime_grid` → `power-grid-main/restore` | Player route above, controller request, Overseer approval; exactly one device changes. | Reject leaves state unchanged; reset device |
| Direct atomic multi-device transition | `n_cmd_open_evacuation` → `vault-door-main/open` + `evacuation-alarm/sound-evacuation` | Player route above; both transitions commit together only after approval. **ТЕРМИНАЛЫ → ПРОТОКОЛ ВЫХОДА → Открыть выход и подать сигнал** lists them under **ПРИ ОДОБРЕНИИ** with the name and block they change, and **ЧТО ВИДЯТ ИГРОКИ** marks the winning rule per target. | Reject/failure changes neither; reset devices or the scene |
| Recovery program | `air-recovery-76`, `network-recovery-76` | **ОБЪЕКТЫ → [related device] → ПРОГРАММЫ → Изменить** for authoring; **ГОТОВО** saves and publishes it. Execute through `n_cmd_air_recovery` or `n2_cmd_network_recovery`. | Reset target device or the scene; referenced programs explain why removal is blocked |
| Bound name/content/visibility/availability | `facilityNameVariants`, `facilityTextVariants`, `visibleWhen`, `availableWhen` | Use **ПРЕДПРОСМОТР** with the referenced state and terminal, then follow the corresponding player rows. Preview changes only the dialog; a saved edit or an approved transition publishes the canonical variant to controller and observer. | Close preview; reset device or the scene |
| State preview | any device, for example `vault-door-main=open` on `t_demo1` | **ОБЪЕКТЫ → Поиск устройств → Гермодверь Убежища 76 → ПРЕДПРОСМОТР СОСТОЯНИЯ**. It preselects **Открыта** on **Терминал смотрителя — Убежище 76**, shows **СЕЙЧАС: … Заперта** beside **ЕСЛИ: … Открыта**, and lists **ИЗМЕНЕНИЙ: 3**: the text of block 3 of **СВОДКА ЭВАКУАЦИИ**, the appearing **НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН**, and the name **Выход уже открыт**. Edit the door's block text rule and reopen: the preview shows the new text. Nothing changes for players or revisions. | **ЗАКРЫТЬ** |
| Condition preview | any condition, for example `relay-storage-damaged` on `t_demo_hack_4` | **ОБЪЕКТЫ → НЕИСПРАВНОСТИ → Архив ретранслятора повреждён → ПРЕДПРОСМОТР УСЛОВИЯ**. **АКТИВНО** is preselected opposite to the current activity; the comparison shows substitution/instability only in preview. | **ЗАКРЫТЬ** |
| Single-device reset | a device whose current differs from initial, including shipped `power-grid-main`, `reactor-main`, `robot-pod-atrium`, `freight-elevator`, or `water-purifier-76` | **ОБЪЕКТЫ → [device] → ВО ВРЕМЯ ИГРЫ → СБРОСИТЬ К НАЧАЛЬНОМУ… → СБРОСИТЬ**. Restores that device and its device-scoped initial conditions in one revision without changing unrelated devices. A reset that only restores faults also succeeds. | Reapply an authored transition if desired |
| Scene reset | the shipped deltas: devices and faults away from their starting values, and executed `n_cmd_1` and `n_cmd_prime_grid` | Shared bar → **СБРОСИТЬ СЦЕНУ**. The dialog counts devices, faults, and executed commands per terminal, says players stay on their terminal with their route and hacking progress, and names what happens to a pending player request. **СБРОСИТЬ СЦЕНУ** returns every device, fault, and executed command to its starting value in one write; if it fails, nothing is reset. Afterwards the control reads **СЦЕНА УЖЕ В НАЧАЛЬНОМ СОСТОЯНИИ**. | Continue from the authored initial scenario |

### Play controls on the device card

Select a device on **ОБЪЕКТЫ**. Its header comes first: name, type, linked terminals, **СЕЙЧАС**,
**В НАЧАЛЕ**, and active faults. The **ВО ВРЕМЯ ИГРЫ** card follows. Every play change is checked
against the facility revision, reaches players at once, withdraws a pending player request, and
is written to the runtime log. **ОТМЕНИТЬ** never reverses it. The card's **ПОСЛЕДНИЕ ИЗМЕНЕНИЯ**
feed lists the device's last five changes in this run, newest first, and starts empty when the
application starts or another session opens. The device list shows each row's current state and
active fault count, for example **Загрязнён · ● 1** for `ventilation-main` with `air-spore-bloom`.

| Capability | Stable assets | Exact Overseer route and expected outcome | Recovery |
|---|---|---|---|
| Card transition | `vault-door-main/open` with shipped `power-grid-main=online` | **Гермодверь Убежища 76 → ВО ВРЕМЯ ИГРЫ → Открыть гермодверь → Открыта**. One press, no dialog; the precondition **Резервная сеть Убежища 76 = В работе · ВЫПОЛНЕНО** is listed. The door opens on `t_demo1` without reconnecting, `n_cmd_open_evacuation` stays unexecuted, and the feed adds the transition by **Смотритель**. | **Запереть гермодверь → Заперта** on the card |
| Blocked card transition | `power-grid-main/shutdown`, then `vault-door-main/open` | On **Резервная сеть Убежища 76**, press **Отключить резервную сеть → Нет питания**. On the door, **Открыть гермодверь** is unavailable with **НЕ ВЫПОЛНЕНО: Резервная сеть Убежища 76 = В работе**. | **Запустить резервную сеть → В работе** |
| Forced change | `reactor-main=critical-overheat` («Критический перегрев»), which no transition reaches; `b_reactor_status` on `t_demo1` | **Реактор Убежища 76 → ПРИНУДИТЕЛЬНО В ДРУГОЕ СОСТОЯНИЕ… → Критический перегрев → ПЕРЕВЕСТИ**. The dialog names the reactor, **Дежурный режим**, and **Критический перегрев**. No fault effect runs. Players read **РЕАКТОР: КРИТИЧЕСКИЙ ПЕРЕГРЕВ**, and the feed records a forced change. | Force **Дежурный режим**, or **СБРОСИТЬ К НАЧАЛЬНОМУ…** |
| Fault switch | `door-offline` («Контроллер гермодвери не отвечает») | **Гермодверь Убежища 76 → ВО ВРЕМЯ ИГРЫ → Контроллер гермодвери не отвечает** switch on. It blocks `execute-command` and exposes `n_diag_door` at once; **В НАЧАЛЕ ИГРЫ: ИСПРАВНО** stays. The header and the device row count it. | Switch it off |
| Rejected player request in the feed | `n_cmd_open_evacuation` («Открыть выход и подать сигнал») | A player requests it on `t_demo1`; the Overseer rejects it. The door's feed reads **«Открыть выход и подать сигнал» · запрос игрока отклонён: Смотрителем · Терминал смотрителя — Убежище 76**. | Request it again |
| Device row with an active fault | `ventilation-main` with `air-spore-bloom` | The list row reads **Загрязнён · ● 1**. Switching the fault off on the card changes it to **Загрязнён**. | Switch it on, or reset the device |

### Diagnostic modes and authored escape paths

| Condition | Category | User-visible effects | Reachable recovery |
|---|---|---|---|
| `door-offline` | `offline` | blocks `execute-command`; exposes `n_diag_door` | `vault-door-main/open` or confirmed private Overseer recovery |
| `grid-unpowered` | `unpowered` | blocks `execute-command`; exposes `n_diag_grid` | `power-grid-main/restore` or confirmed private Overseer recovery |
| `network-isolated` | `network-isolated` | blocks `terminal-transition` and `run-recovery-program`; exposes `n2_diag_network`; substitutes `b2_network_status` | Has an authored `network-recovery-76` reference; while the program capability is blocked, confirmed private Overseer recovery is the reachable escape |
| `relay-storage-damaged` | `storage-damaged` | substitutes `b6_archive_record`; applies display instability | confirmed private Overseer recovery |
| `guard-authorization-corrupted` | `authorization-corrupted` | blocks `hack`; exposes `n5_diag_auth` | confirmed private Overseer recovery |
| `greenhouse-display-unstable` | `display-unstable` | applies display instability; exposes `n3_diag_display` | confirmed private Overseer recovery |
| `air-spore-bloom` | `custom:biohazard` | blocks `view-entry` | `ventilation-main/purge`, `air-recovery-76`, or confirmed private Overseer recovery |

The matrix covers all four effect shapes (`capabilityBlock`, `diagnosticPath`,
`recordSubstitution`, and `displayInstability`), all five authored capability-block instances, and
all three recovery-reference shapes (device transition, recovery program, and private Overseer
action). Back and acknowledgement remain available while capabilities are blocked, so every
condition has an explicit escape path and no player route is an accidental dead end.

## Version-1 round trip

Use **Open session → `sessions/demo.json` → Save session**, reopen the saved file, and start the
broadcast again. The result must remain `version: 1` and retain all terminals, exact-one group
membership, ordered local/remote transitions, command snapshots, EntryContent changes, facility
references, the linked player roster, and compatible unknown fields. Repeat the local/remote
transition pair and one ordinary command after reopening to prove the saved graph remains usable.
