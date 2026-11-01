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
| `n_diag_door` | diagnostic entry | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → АВАРИЙНАЯ ДИАГНОСТИКА → ПРИВОД ГЕРМОДВЕРИ НЕ ОТВЕЧАЕТ** | Switch `door-offline` on from the door card and keep the grid offline; shows its diagnostic and command block. | **НАЗАД**; `vault-door-main/open` or private recovery |
| `n_folder_actions` | folder | `t_demo1`: **ПРОТОКОЛ ВЫХОДА** | Opens ordinary, persistent, facility, recovery-program, and terminal-transition commands. | **НАЗАД** |
| `n_cmd_1` | persisted legacy state change + EntryContent | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Активировать маршрут выхода** | The shipped snapshot is already completed: completed label/result and `b_vault_shutdown_heading` persist. Reset its state in Overseer, request as controller, approve or reject as Overseer, then reopen **СВОДКА ЭВАКУАЦИИ**. | Acknowledge; **Сбросить состояние** restores the authored command form |
| `n_cmd_prime_grid` | persisted facility state change | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Запустить резервную сеть** | The shipped snapshot records completion and `power-grid-main=online`. Reset command state and the grid, request, then approve; the one-device recovery transition restores power and clears `grid-unpowered`. | Acknowledge; single-device reset returns the grid to `offline` |
| `n_cmd_open_evacuation` | atomic multi-device + EntryContent | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Открыть выход и подать сигнал** | Requires `power-grid-main=online`. Approval atomically opens `vault-door-main`, sets `evacuation-alarm=evacuation`, changes `b_vault_shutdown_body`, changes the command label, and reveals `n_entry_open_route`. | Acknowledge; reset the two devices and command state |
| `n_cmd_air_recovery` | recovery-program command | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Загрузить кассету очистки воздуха** | With `ventilation-main=contaminated` and `air-spore-bloom` active, approval runs `air-recovery-76`, purges ventilation, clears the condition, and changes bound air text. | Acknowledge; reset ventilation or the scene |
| `n_cmd_ordinary_1` | ordinary command | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Проверить наружные датчики** | Immediately shows the authored sensor report; no approval or persistent mutation. | Acknowledge |
| `n_cmd_state_change_1` | terminal transition | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Открыть аварийный канал этого пульта** | Overseer approval changes the active view to the same console's remote level-5 context `t_demo2`; observer follows the same authoritative transition. | `t_demo2` root → **Вернуться к локальному пульту** |
| `n2_entry_channel` | block entry + record substitution | `t_demo2`: **КАНАЛ СМОТРИТЕЛЯ 76/ОМЕГА** | Shows the remote-mirror record. The shipped, active `network-isolated` substitutes corrupted `b2_network_status`; switch **Закрытый канал изолирован** off under **ОБЪЕКТЫ → НЕИСПРАВНОСТИ ТЕРМИНАЛОВ** for the authored text, and connect the network for its normal bound variant. | **НАЗАД**; run `network-recovery-76` or private recovery |
| `n2_folder_diagnostics` | conditionally visible folder | `t_demo2`: **СЕТЕВАЯ ДИАГНОСТИКА** | Set `network-overseer=connected`; folder becomes visible. | **НАЗАД**; reset/isolate network to hide it |
| `n2_diag_network` | diagnostic entry | `t_demo2`: **СЕТЕВАЯ ДИАГНОСТИКА → КАНАЛ ИЗОЛИРОВАН** | Shows the authored program and private recovery choices when its folder is visible. | **НАЗАД** |
| `n2_cmd_network_recovery` | two-transition recovery-program command | `t_demo2`: **Запустить СЕТЬ-76** | The shipped snapshot has `network-overseer=isolated` and `network-isolated` active, and that fault deliberately blocks this player program. The Overseer first switches **Закрытый канал изолирован** off under **ОБЪЕКТЫ → НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**. Approval then runs `network-recovery-76` in one revision: `network-overseer/connect` connects the network and restores normal record text, and `security-turret/track` puts **Турель внешнего поста** into **Сопровождение**, so `t_demo_hack_3` **ПЕРИМЕТР** reports **ТУРЕЛЬ: СОПРОВОЖДЕНИЕ ЦЕЛЕЙ**. | Acknowledge; **СБРОСИТЬ К НАЧАЛЬНОМУ…** on **Служебная сеть Смотрителя** (back to `isolated`) and on **Турель внешнего поста** (back to `safe`), then switch **Закрытый канал изолирован** on again |
| `n2_cmd_trace` | ordinary command | `t_demo2`: **Проследить последний пакет** | Immediately reveals the RS-04 narrative lead. | Acknowledge |
| `n2_cmd_return` | reciprocal terminal transition | `t_demo2`: **Вернуться к локальному пульту** | Overseer approval returns every participant to `t_demo1`, preserving the group's local/remote identity. While `network-isolated` is active, as shipped, this transition is blocked too; switch **Закрытый канал изолирован** off first. | `t_demo1` root |
| `n3_entry_crop` | description entry | `t_demo_hack_1`: **ПОСЛЕДНИЙ УРОЖАЙ** | After the level-1 hack, shows the greenhouse hand-off record. | **НАЗАД** |
| `n3_entry_air` | block entry + state variant | `t_demo_hack_1`: **ВОЗДУШНЫЙ КОНТУР** | Shows contaminated text, then the filtered variant after `air-recovery-76`. | **НАЗАД**; reset ventilation |
| `n3_diag_display` | conditionally visible diagnostic | `t_demo_hack_1`: **НЕСТАБИЛЬНОСТЬ ЭКРАНА** | Set `ventilation-main=filtered`; switch `greenhouse-display-unstable` on under **НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**, or preview it, to demonstrate diagnostic-path and display-instability effects. | **НАЗАД**; private recovery or scene reset |
| `n3_cmd_read_sensors` | ordinary command | `t_demo_hack_1`: **Снять показания спор** | Immediately shows greenhouse sensor readings. | Acknowledge |
| `n4_entry_manifest` | block entry + state variant | `t_demo_hack_2`: **ГРУЗОВАЯ ВЕДОМОСТЬ** | After the level-2 hack, the shipped `freight-elevator=upper` state shows the upper-level block variant. | **НАЗАД**; reset lift for its lower variant |
| `n4_cmd_call_lift` | facility state change | `t_demo_hack_2`: **Вернуть кабину вниз** | From `freight-elevator=upper`, approval applies `freight-elevator/lower` and changes the manifest and completed label. | Acknowledge; reset lift to authored `lower` or edit it back to `upper` to repeat |
| `n5_entry_security` | block entry + state variant | `t_demo_hack_3`: **ПЕРИМЕТР** | After the level-3 hack, shows `security-turret=safe`; setting it to `tracking` changes the bound status. | **НАЗАД**; reset turret |
| `n5_diag_auth` | conditionally visible diagnostic | `t_demo_hack_3`: **ТАБЛИЦА ДОПУСКА ПОВРЕЖДЕНА** | Set `security-turret=tracking` and switch `guard-authorization-corrupted` on under **НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**, or preview it; demonstrates the `hack` block and diagnostic path. | **НАЗАД**; private recovery or scene reset |
| `n5_cmd_scan` | ordinary command | `t_demo_hack_3`: **Сканировать дорогу** | Immediately shows the exterior route scan. | Acknowledge |
| `n6_entry_archive` | block entry + active record substitution | `t_demo_hack_4`: **ПОСЛЕДНЯЯ ПЕРЕДАЧА** | After the level-4 hack, shipped `relay-storage-damaged` substitutes corrupted archive text and activates display instability. | **НАЗАД**; private recovery, the **Архив ретранслятора повреждён** switch under **НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**, removes the fault |
| `n6_entry_robot` | block entry + state variant | `t_demo_hack_4`: **КУРЬЕРСКИЙ ПОСТ** | Shipped `robot-pod-atrium=patrol` shows its route variant; reset shows the charging text. | **НАЗАД** |
| `n6_cmd_decode` | ordinary command | `t_demo_hack_4`: **Повторить повреждённую передачу** | Immediately advances the RS-04 failure narrative. | Acknowledge |

## Facility authoring, diagnostics, preview, and reset

### Prepare a story with the in-app guide

Open **ОБЪЕКТЫ → КАК СОБРАТЬ СЦЕНАРИЙ**. The guide is open while the session has no devices.
Once it has one, the guide is one line, for example **КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 1 ИЗ 5: УСТРОЙСТВО**,
so the device controls stay in view. **ПОКАЗАТЬ** opens it and **СКРЫТЬ** closes it; that choice
holds until the application closes. Select a device on the left to make the guide use that device.
The counts describe configured relationships, not a guarantee that the scenario is correct.

Use one continuous example: players encounter a locked door, request that it open from a
terminal, and see a status record change to **Путь свободен**.

| Step | Story decision | Configuration route |
|---|---|---|
| **1. Устройство** (**УСТРОЙСТВО**) | What can change? A door starts **Заперта** and can become **Открыта**. | **СОЗДАТЬ УСТРОЙСТВО** opens **УСТРОЙСТВО**. Type the name, choose **ТИП → ДВЕРЬ**, and press **ПРИМЕНИТЬ ШАБЛОН**: the chips **★ Заперта** and **Открыта** appear, and the **ИЗ → В** matrix holds **Открыть** and **Запереть**. Choose the **Заперта → Открыта** cell and rename the transition **Открыть дверь**. Identifiers come from the names (**Заперта** → `zaperta`, **Открыть дверь** → `otkryt-dver`) and appear only under **ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ**. A missing or repeated name marks its chip or cell, and the dialog counts the marks in **ИСПРАВЬТЕ ОТМЕЧЕННОЕ: <n>**. **ГОТОВО** saves and selects the device. |
| **2. Действие** (**ДЕЙСТВИЕ**) | What do players do? They request **Открыть дверь** on a terminal. | **СВЯЗАТЬ С КОМАНДОЙ** or **ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ** opens **ДЕЙСТВИЕ КОМАНДЫ**. Under **ЧТО ЗАПУСТИТЬ**, keep **ПЕРЕХОДЫ** and choose the door and its opening transition. A command without player texts asks for **НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ** and **ЗАПРОС ПОДТВЕРЖДЕНИЯ**; nothing is filled in for you. If there is no free command, the guide offers **ОТКРЫТЬ ТЕРМИНАЛЫ** to create one. A transition alone does not create a player command. |
| **3. Результат** (**РЕЗУЛЬТАТ**) | How do players notice the change? A status record says **Путь свободен**. | **НАСТРОИТЬ РЕЗУЛЬТАТ НА ТЕРМИНАЛЕ** opens **ПРАВИЛО ОТОБРАЖЕНИЯ** on **ТЕКСТ ЗАПИСИ**. Fill the sentence: **КОГДА** door = **Открыта**, **ГДЕ** the record, **БЛОК** the status block, **ТЕКСТ** **Путь свободен**. The block keeps **Дверь заперта** in the other states. **НАЗВАНИЕ В МЕНЮ**, **ПОКАЗАТЬ ЭЛЕМЕНТ**, and **РАЗРЕШИТЬ КОМАНДУ** are the other rule types. |
| **4. Осложнения** (**ОСЛОЖНЕНИЯ**) | Optional: what obstructs the scene and how is it repaired? | **ДОБАВИТЬ НЕИСПРАВНОСТЬ** opens **НЕИСПРАВНОСТЬ**. Choose its blocking effect and **В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА**, then list the transitions under **КАК ПОЯВЛЯЕТСЯ** and **КАК УСТРАНЯЕТСЯ**. **Смотритель вручную** is always listed: during play the Overseer switches the fault off by hand. **СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ** combines transitions; a command runs it after you choose **ПРОГРАММА ВОССТАНОВЛЕНИЯ** under **ЧТО ЗАПУСТИТЬ** in **ДЕЙСТВИЕ КОМАНДЫ**. **НАСТРОИТЬ АКТИВАЦИЮ И СНЯТИЕ** opens the selected device's first fault. |
| **5. Проверка** (**ПРОВЕРКА**) | Does the scene show the expected result? | Every edit is already saved. **ПРОВЕРИТЬ В ПРЕДПРОСМОТРЕ** compares the open state on the chosen terminal; then request the actual command as a player and approve it as Overseer. Preview checks presentation without performing the action. |

If a command or display destination is missing, use **ОТКРЫТЬ ТЕРМИНАЛЫ**, create the command or
entry, then return to **ОБЪЕКТЫ** to bind it to the selected device. Nothing is lost by switching
tabs: applied edits are saved, and unapplied input in the node form or terminal settings stays
and is listed in the shared bar as **НЕ ПРИМЕНЕНО: …**. Merely moving between guide
steps never saves anything.

Prerequisites are checked against the state **before** a command or recovery program runs.
For example, if opening the door requires power already on, restoring power and opening that
door cannot satisfy the prerequisite within one atomic program: restore power first with a
separate action, then request the opening action. Use faults and programs only if the story needs
them; a basic door scene does not require either.

The device's tabs distinguish **ЧТО ДЕЛАЮТ ИГРОКИ** (command actions) from **ЧТО ВИДЯТ ИГРОКИ**
(display rules), next to **НЕИСПРАВНОСТИ** and **ПРОГРАММЫ**; each tab shows its count. Rows keep
contextual **Изменить** and **Удалить** controls. The guide adds no content or saved progress merely
by moving between steps.

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

The Overseer reaches every row from **ОБЪЕКТЫ** after opening the demo. Selecting a device exposes
its readable current and starting state and its related rules beside the searchable device list.
Identifiers appear only under **ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ** in a dialog, or under
**подробности для разработчика** in the device header and in dependency rows.

The device dialog shows states as chips, with **★** on the starting state, and transitions in the
**ИЗ → В** matrix. Choosing a cell opens **ПЕРЕХОД: <из> → <в>** with **ПРЕДУСЛОВИЯ** and a read-only
**НЕИСПРАВНОСТИ** list. Its **К НЕИСПРАВНОСТИ** opens the fault dialog, the only place where a
transition is set to switch a fault on or off.

Fault rows on the **НЕИСПРАВНОСТИ** tab read **СЕЙЧАС: … · В НАЧАЛЕ: …** and open the fault dialog in
one press. Editing a fault's starting activity changes only what a reset restores; a new fault
starts in the chosen activity.

Under **ТЕРМИНАЛЫ**, the **ДЕЙСТВИЕ КОМАНДЫ** block appears only in **ИЗМЕНЯЕТ СОСТОЯНИЕ** mode.
Switching `n_cmd_open_evacuation` to another mode shows the transitions it would lose and offers
**ОСТАВИТЬ «ИЗМЕНЯЕТ СОСТОЯНИЕ»** or **УДАЛИТЬ ДЕЙСТВИЕ И ПЕРЕКЛЮЧИТЬ**; **ПРИМЕНИТЬ** stays disabled
until you choose.

Selecting `n_cmd_open_evacuation` also shows two lists. **ПРИ ОДОБРЕНИИ** names every change approval
makes: **Гермодверь Убежища 76: Заперта → Открыта**, **Сигнал эвакуации: Тишина → Эвакуация**, the
name after execution **Выход открыт**, and the new text of block 2 of **СВОДКА ЭВАКУАЦИИ**.
**ЧТО ВИДЯТ ИГРОКИ** lists, per target, every rule for the name, availability, and the rewritten block,
lowest precedence first, and marks the one players see with **ВИДЯТ ИГРОКИ**. With the door open,
the state name **Выход уже открыт** shows **ПЕРЕКРЫВАЕТ: ИСХОДНОЕ НАЗВАНИЕ, НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ**;
availability depends on `power-grid-main=online`, and an active `grid-unpowered` outranks it.
**ИЗМЕНИТЬ** opens the rule's own **ПРАВИЛО ОТОБРАЖЕНИЯ** dialog, the same one **ОБЪЕКТЫ** uses. The
terminal tree shows each name as players see it now, with the authored name in its tooltip, and
marks **СКРЫТО ОТ ИГРОКОВ** and **НЕДОСТУПНО**.

Device, fault, display rule, command action, and recovery-program dialogs save on **ГОТОВО**; a
rejected save reopens the dialog with its input and the reason. **ЗАВИСИМОСТИ** opens
**ЗАВИСИМОСТИ УСТРОЙСТВА**: one sentence per link, its place by name, and a button named for its
destination (see "Dependency destinations").

**ПРЕДПРОСМОТР СОСТОЯНИЯ** and **ПРЕДПРОСМОТР НЕИСПРАВНОСТИ** resolve automatically when opened or
when the terminal, state, or activity changes, after any save in progress. A device preview
preselects the first state different from the current one, and a fault preview the opposite
activity. Both show **СЕЙЧАС** and **ЕСЛИ** side by side, count the differences, name each one
(**ПЕРЕИМЕНУЕТСЯ**, **ТЕКСТ ИЗМЕНИТСЯ**, **ПОЯВИТСЯ**, **ИСЧЕЗНЕТ**, **СТАНЕТ ДОСТУПНО**,
**СТАНЕТ НЕДОСТУПНО**) and mark it in both trees, and never change players or revisions.
**ОБНОВИТЬ ПРЕДПРОСМОТР** remains a retry action.

| Capability | Stable assets | Exact Overseer route and expected outcome | Recovery |
|---|---|---|---|
| All ten device kinds | `power-grid-main` (`power-grid`), `vault-door-main` (`door`), `evacuation-alarm` (`alarm`), `network-overseer` (`network-segment`), `ventilation-main` (`ventilation`), `reactor-main` (`reactor`), `security-turret` (`turret`), `robot-pod-atrium` (`robot-pod`), `freight-elevator` (`elevator`), `water-purifier-76` (`custom:water-purifier`) | **ОБЪЕКТЫ → Поиск устройств → [device] → ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ** opens **УСТРОЙСТВО** with the state chips and the **ИЗ → В** matrix in view; **ГОТОВО** saves the complete graph. **ЗАВИСИМОСТИ** lists every link to the device before destructive edits. | **ОТМЕНА** leaves the device untouched; **ОТМЕНИТЬ** on the shared bar reverses a saved edit |
| Apply a template to a new device | the **ДВЕРЬ** template | **ОБЪЕКТЫ → + УСТРОЙСТВО** → **НАЗВАНИЕ** «Шлюз продуктового склада» → **ТИП → ДВЕРЬ**. The offer reads **ШАБЛОН: ДВЕРЬ** with **Заперта, Открыта · Открыть, Запереть**. **ПРИМЕНИТЬ ШАБЛОН** adds the chips **★ Заперта** and **Открыта** and the transitions **Открыть** and **Запереть**, and the dialog reads **ИЗ ШАБЛОНА ДОБАВЛЕНО: СОСТОЯНИЙ — 2, ПЕРЕХОДОВ — 2**. **ГОТОВО** saves the door, and the list then reads **ВСЕ 11**. | **ОТМЕНИТЬ** on the shared bar removes the new device |
| Template already present | `security-turret` and the **ТУРЕЛЬ** template | **ОБЪЕКТЫ → Турель внешнего поста → ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ** shows **ШАБЛОН: ТУРЕЛЬ**. **ПРИМЕНИТЬ ШАБЛОН** is blocked with **ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ**, because both states and both transitions already exist by name. Changing **ТИП** changes only the offer, never the states. | **ОТМЕНА** |
| Matrix cell with two transitions | `reactor-main/standby` («Перевести в дежурный режим») and `reactor-main/backup-grid-start` («Пуск от резервной сети»), both scrammed → standby | **ОБЪЕКТЫ → Реактор Убежища 76 → ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ**. In the **ИЗ → В** matrix, row **Аварийная остановка**, column **Дежурный режим**, the cell lists **Перевести в дежурный режим** and **Пуск от резервной сети** with **1 предусловие**. Choosing it opens **ПЕРЕХОД: Аварийная остановка → Дежурный режим**, with both names as choices; **Пуск от резервной сети** holds the precondition **Резервная сеть Убежища 76** = **В работе**. **+ ПЕРЕХОД** would add a third transition to the same cell. In play, press **Остановить реактор → Аварийная остановка** on the card: it then offers **Перевести в дежурный режим → Дежурный режим** (**БЕЗ ПРЕДУСЛОВИЙ**) and **Пуск от резервной сети → Дежурный режим** (**Резервная сеть Убежища 76 = В работе · ВЫПОЛНЕНО**). With the grid shut down, only the second is blocked: **НЕ ВЫПОЛНЕНО: Резервная сеть Убежища 76 = В работе**. | **ОТМЕНА** closes the dialog unsaved; on the card, either transition returns the reactor to the shipped **Дежурный режим** |
| Direct single-device transition | `n_cmd_prime_grid` → `power-grid-main/restore` | Player route above, controller request, Overseer approval; exactly one device changes. | Reject leaves state unchanged; reset device |
| Direct atomic multi-device transition | `n_cmd_open_evacuation` → `vault-door-main/open` + `evacuation-alarm/sound-evacuation` | Player route above; both transitions commit together only after approval. **ТЕРМИНАЛЫ → ПРОТОКОЛ ВЫХОДА → Открыть выход и подать сигнал** lists them under **ПРИ ОДОБРЕНИИ** with the name and block they change, and **ЧТО ВИДЯТ ИГРОКИ** marks the winning rule per target. | Reject or failure changes neither; reset devices or the scene |
| Command action from the command form | `n_cmd_open_evacuation`, `n2_cmd_network_recovery` | **ТЕРМИНАЛЫ → Терминал смотрителя — Убежище 76 → ПРОТОКОЛ ВЫХОДА → Открыть выход и подать сигнал → ДЕЙСТВИЕ КОМАНДЫ → ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ** opens **ДЕЙСТВИЕ КОМАНДЫ** for the fixed command **Терминал смотрителя — Убежище 76 › ПРОТОКОЛ ВЫХОДА › Открыть выход и подать сигнал**. **ЧТО ЗАПУСТИТЬ** is **ПЕРЕХОДЫ**, with the rows **Гермодверь Убежища 76 / Открыть гермодверь** and **Сигнал эвакуации / Подать сигнал эвакуации**; **+ ПЕРЕХОД** adds a row. The player texts stay hidden because the command already has them. **ГОТОВО** saves, and the form's summary and **ПРИ ОДОБРЕНИИ** follow. **ТЕРМИНАЛЫ → Терминал смотрителя — аварийный удалённый доступ → Запустить СЕТЬ-76 → ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ** opens the same dialog on **ПРОГРАММА ВОССТАНОВЛЕНИЯ** with **СЕТЬ-76** selected. | **ОТМЕНА** saves nothing; **ОТМЕНИТЬ** reverses a saved edit |
| Two-transition recovery program | `network-recovery-76` («СЕТЬ-76»): `network-overseer/connect`, then `security-turret/track`; also `air-recovery-76` («ВОЗДУХ-76») | **ОБЪЕКТЫ → Турель внешнего поста → ПРОГРАММЫ → СЕТЬ-76 → Изменить** opens **ПРОГРАММА ВОССТАНОВЛЕНИЯ**. **ПЕРЕХОДЫ** holds two identical rows, **Служебная сеть Смотрителя / Восстановить служебную сеть** and **Турель внешнего поста / Включить сопровождение**, and **+ ПЕРЕХОД** adds a third. **ЗАПУСКАЮТ КОМАНДЫ** lists **Терминал смотрителя — аварийный удалённый доступ › Запустить СЕТЬ-76**. **ГОТОВО** saves and publishes it. Execute it through `n2_cmd_network_recovery`: the network connects and the turret starts tracking together. `air-recovery-76` runs through `n_cmd_air_recovery`. | Reset **Служебная сеть Смотрителя** and **Турель внешнего поста**, or the scene; referenced programs explain why removal is blocked |
| Display rule: **ТЕКСТ ЗАПИСИ** | `b_vault_door_status` in `n_entry_vault_shutdown` (`facilityTextVariants`) | **ОБЪЕКТЫ → Гермодверь Убежища 76 → ЧТО ВИДЯТ ИГРОКИ →** row **Терминал смотрителя — Убежище 76 › СВОДКА ЭВАКУАЦИИ › Блок 3** **→ Изменить** opens **ПРАВИЛО ОТОБРАЖЕНИЯ** on **ТЕКСТ ЗАПИСИ**. **КОГДА** Гермодверь Убежища 76 = Открыта; **ГДЕ** Терминал смотрителя — Убежище 76 › ДИРЕКТИВЫ VAULT-TEC › СВОДКА ЭВАКУАЦИИ; **БЛОК** 3 · «ГЕРМОДВЕРЬ: ЗАПЕРТА»; **ТЕКСТ** ГЕРМОДВЕРЬ: ОТКРЫТА // МАРШРУТ СВОБОДЕН. It says **В остальных состояниях останется исходный текст блока.** The other tabs are blocked with **ЧТОБЫ СМЕНИТЬ ТИП, СОЗДАЙТЕ НОВОЕ ПРАВИЛО**. Players read the open-state text after the door opens. | **ОТМЕНА**; close the door to restore the original text |
| Display rule: **НАЗВАНИЕ В МЕНЮ** | `n_cmd_open_evacuation` (`facilityNameVariants`) | **ОБЪЕКТЫ → Гермодверь Убежища 76 → ЧТО ВИДЯТ ИГРОКИ →** row **Терминал смотрителя — Убежище 76 › Открыть выход и подать сигнал** **→ Изменить** opens **НАЗВАНИЕ В МЕНЮ**: **КОГДА** Гермодверь Убежища 76 = Открыта, **ТЕКСТ** Выход уже открыт, and **В остальных состояниях останется исходное название.** | **ОТМЕНА** |
| Display rule: **ПОКАЗАТЬ ЭЛЕМЕНТ** | `n_entry_open_route` (`visibleWhen`) | **ОБЪЕКТЫ → Гермодверь Убежища 76 → ЧТО ВИДЯТ ИГРОКИ →** row **Терминал смотрителя — Убежище 76 › НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН** **→ Изменить** opens **ПОКАЗАТЬ ЭЛЕМЕНТ** with no **ТЕКСТ** field and **В остальных состояниях элемент скрыт.** | **ОТМЕНА** |
| Display rule: **РАЗРЕШИТЬ КОМАНДУ** | `n_cmd_open_evacuation` (`availableWhen`) | **ОБЪЕКТЫ → Резервная сеть Убежища 76 → ЧТО ВИДЯТ ИГРОКИ →** row **Терминал смотрителя — Убежище 76 › Открыть выход и подать сигнал** («Разрешить команду · Когда Резервная сеть Убежища 76: В работе») **→ Изменить** opens **РАЗРЕШИТЬ КОМАНДУ**: **КОГДА** Резервная сеть Убежища 76 = В работе, **ГДЕ** offers commands only, and **В остальных состояниях команда видна, но её нельзя запросить.** A new rule from **ДОБАВИТЬ ПРАВИЛО** starts with the selected device and keeps device, state, and text across tabs. | **ОТМЕНА** |
| Bound name, content, visibility, and availability in play | `facilityNameVariants`, `facilityTextVariants`, `visibleWhen`, `availableWhen` | Use **ПРЕДПРОСМОТР СОСТОЯНИЯ** with the referenced state and terminal, then follow the corresponding player rows. Preview changes only the dialog; a saved edit or an approved transition publishes the canonical variant to controller and observer. | Close preview; reset device or the scene |
| State preview | any device, for example `vault-door-main=open` on `t_demo1` | **ОБЪЕКТЫ → Поиск устройств → Гермодверь Убежища 76 → ПРЕДПРОСМОТР СОСТОЯНИЯ** opens **ПРЕДПРОСМОТР УСТРОЙСТВА**. It preselects **Открыта** on **Терминал смотрителя — Убежище 76**, shows **СЕЙЧАС: … Заперта** beside **ЕСЛИ: … Открыта**, and lists **ИЗМЕНЕНИЙ: 3**: the text of block 3 of **СВОДКА ЭВАКУАЦИИ**, the appearing **НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН**, and the name **Выход уже открыт**. Edit the door's block text rule and reopen: the preview shows the new text. Nothing changes for players or revisions. | **ЗАКРЫТЬ** |
| Fault preview | any fault, for example `relay-storage-damaged` on `t_demo_hack_4` | **ОБЪЕКТЫ → НЕИСПРАВНОСТИ ТЕРМИНАЛОВ → Архив ретранслятора повреждён → ИЗМЕНИТЬ → ПРЕДПРОСМОТР НЕИСПРАВНОСТИ**. **АКТИВНО** starts opposite to the current activity, so it is cleared. Choose **ТЕРМИНАЛ → Терминал ретранслятора — станция АТЛАС-76**: **СЕЙЧАС: Архив ретранслятора повреждён: Активно** beside **ЕСЛИ: Архив ретранслятора повреждён: Неактивно** shows the archive record without the substitution, only in preview. | **ЗАКРЫТЬ**, then **ОТМЕНА** |
| Fault without an activating transition | `door-offline` («Контроллер гермодвери не отвечает») | **ОБЪЕКТЫ → Гермодверь Убежища 76 → НЕИСПРАВНОСТИ → Контроллер гермодвери не отвечает** (**СЕЙЧАС: НЕАКТИВНА · В НАЧАЛЕ: ИСПРАВНО · Гермодверь Убежища 76**) opens **НЕИСПРАВНОСТЬ**. **КАК ПОЯВЛЯЕТСЯ** lists no transition and warns **Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.** **КАК УСТРАНЯЕТСЯ** lists **Гермодверь Убежища 76 — Открыть гермодверь** and **Смотритель вручную** (**во время игры — переключатель на карточке устройства**). **+ ВКЛЮЧАТЬ ПРИ ПЕРЕХОДЕ…** offers every transition not yet listed for this fault; choosing one removes the warning. | **ОТМЕНА** saves nothing |
| Fault with an activating transition | `air-spore-bloom` («Споровое загрязнение воздушного контура») | **ОБЪЕКТЫ → Контур вентиляции теплицы → НЕИСПРАВНОСТИ → Споровое загрязнение воздушного контура**. **КАК ПОЯВЛЯЕТСЯ** reads **Активна с начала игры.** and lists **Контур вентиляции теплицы — Смоделировать загрязнение** with **УБРАТЬ**. **КАК УСТРАНЯЕТСЯ** lists **Контур вентиляции теплицы — Продуть воздушный контур**, **ВОЗДУХ-76** (программа восстановления) with **К ПРОГРАММЕ**, and **Смотритель вручную**. **К ПРОГРАММЕ** opens **ПРОГРАММА ВОССТАНОВЛЕНИЯ** over the fault dialog; closing it returns there. **+ ПРОГРАММА ВОССТАНОВЛЕНИЯ…** starts a new program for this fault from its first clearing transition. | **ОТМЕНА** in each dialog |
| Terminal faults in their own group | `network-isolated`, `relay-storage-damaged`, `guard-authorization-corrupted`, `greenhouse-display-unstable` | **ОБЪЕКТЫ → НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**, below the device list, stays visible with or without a selected device, and search and filters never narrow it. In authored order it lists **Закрытый канал изолирован** · **АКТИВНА** · **ТЕРМИНАЛ: Терминал смотрителя — аварийный удалённый доступ**; **Архив ретранслятора повреждён** · **АКТИВНА** · **ТЕРМИНАЛ: Терминал ретранслятора — станция АТЛАС-76**; **Таблица допуска внешнего поста повреждена** · **НЕАКТИВНА** · **ТЕРМИНАЛ: Терминал внешней охраны — пост у гермодвери**; and **Видеоконтроллер теплицы нестабилен** · **НЕАКТИВНА** · **ТЕРМИНАЛ: Терминал агронома — теплица Убежища 76**. One press on a name switches that fault, without confirmation, and the group reports **НЕИСПРАВНОСТЬ СНЯТА** or **НЕИСПРАВНОСТЬ АКТИВИРОВАНА**. **ИЗМЕНИТЬ** opens its **НЕИСПРАВНОСТЬ** dialog, where **Смотритель вручную** reads **во время игры — переключатель в списке «Неисправности терминалов»**. | Press the switch again, or **СБРОСИТЬ СЦЕНУ** |
| Single-device reset | a device whose current state differs from its starting one, including shipped `power-grid-main`, `reactor-main`, `robot-pod-atrium`, `freight-elevator`, or `water-purifier-76` | **ОБЪЕКТЫ → [device] → ВО ВРЕМЯ ИГРЫ → СБРОСИТЬ К НАЧАЛЬНОМУ… → СБРОСИТЬ**. The dialog says **Текущее состояние устройства и его неисправности вернутся к начальным значениям.** It restores that device and its own faults in one revision without changing unrelated devices. A reset that only restores faults also succeeds. | Reapply an authored transition if desired |
| Scene reset | the shipped deltas: devices and faults away from their starting values, and executed `n_cmd_1` and `n_cmd_prime_grid` | Shared bar → **СБРОСИТЬ СЦЕНУ**. The dialog counts devices, faults, and executed commands per terminal, says players stay on their terminal with their route and hacking progress, and names what happens to a pending player request. **СБРОСИТЬ СЦЕНУ** returns every device, fault, and executed command to its starting value in one write; if it fails, nothing is reset. Afterwards the control reads **СЦЕНА УЖЕ В НАЧАЛЬНОМ СОСТОЯНИИ**. | Continue from the authored initial scenario |

### Device list filters

The filters sit above the device list. Their counts cover the whole session: search and the chosen
filter never change them, and play changes from either side update them at once. The chosen
filter survives tab switches and session changes until the application closes.

| Capability | Stable assets | Exact Overseer route and expected outcome | Recovery |
|---|---|---|---|
| Filter **ВСЕ** | all ten devices | **ОБЪЕКТЫ → ВСЕ 10**, the default, lists every device in authored order: name and type on the left, the current-state badge and any fault mark on the right. Rows show no identifiers on hover. | — |
| Filter **ИЗМЕНЁННЫЕ** | `power-grid-main`, `reactor-main`, `robot-pod-atrium`, `freight-elevator`, `water-purifier-76` | **ОБЪЕКТЫ → ИЗМЕНЁННЫЕ 5** lists the five devices whose current state differs from the starting one. Their badges **В работе**, **Дежурный режим**, **Патруль**, **Верхний уровень**, and **Питьевая вода** have a yellow border. Typing **В работе** in **Поиск устройств** (**Название или состояние**) then leaves only **Резервная сеть Убежища 76**. | **ВСЕ 10**; clear the search |
| Filter **С НЕИСПРАВНОСТЬЮ** | `ventilation-main` with `air-spore-bloom` | **ОБЪЕКТЫ → С НЕИСПРАВНОСТЬЮ 1** lists only **Контур вентиляции теплицы**, with the badge **Загрязнён** and the red fault mark **● 1**. If another device is selected, the list shows **Выбранное устройство скрыто фильтром** and its details stay open. A search that matches nothing shows **Ни одно устройство не подходит под фильтр и поиск.** and a **ВСЕ** button that clears the search and selects **ВСЕ**. | **ВСЕ 10** |

### Dependency destinations

**ОБЪЕКТЫ → [device] → ЗАВИСИМОСТИ** opens **ЗАВИСИМОСТИ УСТРОЙСТВА** with **НАЙДЕНО ССЫЛОК: n**. Each
row is a sentence, its place by name, and a button named for its destination; the technical path
stays under a collapsed **подробности для разработчика**. Each button closes the dialog and opens
its destination in the newest session.

| Destination | Stable assets | Exact Overseer route and expected outcome | Recovery |
|---|---|---|---|
| **К УСТРОЙСТВУ** | `vault-door-main/open` precondition on `power-grid-main` | **ОБЪЕКТЫ → Резервная сеть Убежища 76 → ЗАВИСИМОСТИ**: «Переход «Открыть гермодверь» устройства «Гермодверь Убежища 76» требует: Резервная сеть Убежища 76 = В работе.», place **Гермодверь Убежища 76 › Открыть гермодверь**. **К УСТРОЙСТВУ** opens the door's **УСТРОЙСТВО** dialog with **ПЕРЕХОД: Заперта → Открыта** open on that transition. The same dialog also lists **Пуск от резервной сети** of the reactor. | **ОТМЕНА** |
| **К НЕИСПРАВНОСТИ** | `air-spore-bloom` scoped to `ventilation-main` | **ОБЪЕКТЫ → Контур вентиляции теплицы → ЗАВИСИМОСТИ**: «Неисправность «Споровое загрязнение воздушного контура» действует на это устройство.» **К НЕИСПРАВНОСТИ** opens its **НЕИСПРАВНОСТЬ** dialog. «Неисправность «Споровое загрязнение воздушного контура» устраняется переходом «Продуть воздушный контур».» leads to the same dialog. | **ОТМЕНА** |
| **К ПРОГРАММЕ** | `network-recovery-76` step `security-turret/track` | **ОБЪЕКТЫ → Турель внешнего поста → ЗАВИСИМОСТИ**: «Программа восстановления «СЕТЬ-76» запускает переход «Включить сопровождение».» **К ПРОГРАММЕ** opens **ПРОГРАММА ВОССТАНОВЛЕНИЯ** for **СЕТЬ-76**. | **ОТМЕНА** |
| **К КОМАНДЕ** | `n_cmd_open_evacuation` action on `vault-door-main/open` | **ОБЪЕКТЫ → Гермодверь Убежища 76 → ЗАВИСИМОСТИ**: «Команда «Открыть выход и подать сигнал» запускает переход «Открыть гермодверь».», place **Терминал смотрителя — Убежище 76 › ПРОТОКОЛ ВЫХОДА › Открыть выход и подать сигнал**. **К КОМАНДЕ** switches to **ТЕРМИНАЛЫ** with that command selected and focus on **ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ**. | Return to **ОБЪЕКТЫ** |
| **К ПРАВИЛУ ОТОБРАЖЕНИЯ** | `n_entry_open_route` visibility on `vault-door-main` | **ОБЪЕКТЫ → Гермодверь Убежища 76 → ЗАВИСИМОСТИ**: «Элемент «НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН» виден, только когда Гермодверь Убежища 76 = Открыта.» **К ПРАВИЛУ ОТОБРАЖЕНИЯ** opens **ПРАВИЛО ОТОБРАЖЕНИЯ** on **ПОКАЗАТЬ ЭЛЕМЕНТ** for that rule. The door's name and block text rules lead there the same way. | **ОТМЕНА** |

### Play controls on the device card

Select a device on **ОБЪЕКТЫ**. Its header comes first: name, type, linked terminals, **СЕЙЧАС**,
**В НАЧАЛЕ**, and active faults. The **ВО ВРЕМЯ ИГРЫ** card follows. Every play change is checked
against the facility revision, reaches players at once, withdraws a pending player request, and
is written to the runtime log. **ОТМЕНИТЬ** never reverses it. The card's **ПОСЛЕДНИЕ ИЗМЕНЕНИЯ**
feed lists the device's last five changes in this run, newest first, and starts empty when the
application starts or another session opens. Each device row shows its current-state badge, with
a yellow border when it differs from the starting state, and a red fault mark when its own faults
are active: `ventilation-main` with `air-spore-bloom` shows the badge **Загрязнён** and the mark
**● 1** («Активных неисправностей: 1»).

| Capability | Stable assets | Exact Overseer route and expected outcome | Recovery |
|---|---|---|---|
| Card transition | `vault-door-main/open` with shipped `power-grid-main=online` | **Гермодверь Убежища 76 → ВО ВРЕМЯ ИГРЫ → Открыть гермодверь → Открыта**. One press, no dialog; the precondition **Резервная сеть Убежища 76 = В работе · ВЫПОЛНЕНО** is listed. The door opens on `t_demo1` without reconnecting, `n_cmd_open_evacuation` stays unexecuted, and the feed adds the transition by **Смотритель**. | **Запереть гермодверь → Заперта** on the card |
| Blocked card transition | `power-grid-main/shutdown`, then `vault-door-main/open` | On **Резервная сеть Убежища 76**, press **Отключить резервную сеть → Нет питания**. On the door, **Открыть гермодверь** is unavailable with **НЕ ВЫПОЛНЕНО: Резервная сеть Убежища 76 = В работе**. | **Запустить резервную сеть → В работе** |
| Forced change | `reactor-main=critical-overheat` («Критический перегрев»), which no transition reaches; `b_reactor_status` on `t_demo1` | **Реактор Убежища 76 → ПРИНУДИТЕЛЬНО В ДРУГОЕ СОСТОЯНИЕ… → Критический перегрев → ПЕРЕВЕСТИ**. The dialog names the reactor, **Дежурный режим**, and **Критический перегрев**. No fault effect runs. Players read **РЕАКТОР: КРИТИЧЕСКИЙ ПЕРЕГРЕВ**, and the feed records a forced change. | Force **Дежурный режим**, or **СБРОСИТЬ К НАЧАЛЬНОМУ…** |
| Fault switch | `door-offline` («Контроллер гермодвери не отвечает») | **Гермодверь Убежища 76 → ВО ВРЕМЯ ИГРЫ → Контроллер гермодвери не отвечает** switch on. It blocks `execute-command` and exposes `n_diag_door` at once; **В НАЧАЛЕ ИГРЫ: ИСПРАВНО** stays. The header counts it, the device row gains the mark **● 1**, and **С НЕИСПРАВНОСТЬЮ** counts the door. | Switch it off |
| Rejected player request in the feed | `n_cmd_open_evacuation` («Открыть выход и подать сигнал») | A player requests it on `t_demo1`; the Overseer rejects it. The door's feed reads **«Открыть выход и подать сигнал» · запрос игрока отклонён: Смотрителем · Терминал смотрителя — Убежище 76**. | Request it again |
| Device row with an active fault | `ventilation-main` with `air-spore-bloom` | The list row shows the badge **Загрязнён** and the fault mark **● 1**. Switching the fault off on the card removes the mark, keeps the badge **Загрязнён**, and changes the filter to **С НЕИСПРАВНОСТЬЮ 0**. | Switch it on, or reset the device |

### Diagnostic modes and authored escape paths

| Fault | Category | Switched on by | User-visible effects | Reachable recovery |
|---|---|---|---|---|
| `door-offline` | `offline` | no transition: only the Overseer's switch | blocks `execute-command`; exposes `n_diag_door` | `vault-door-main/open` or private Overseer recovery (the switch on the door card) |
| `grid-unpowered` | `unpowered` | `power-grid-main/shutdown` | blocks `execute-command`; exposes `n_diag_grid` | `power-grid-main/restore` or private Overseer recovery (the switch on the grid card) |
| `network-isolated` | `network-isolated` | `network-overseer/isolate` | blocks `terminal-transition` and `run-recovery-program`; exposes `n2_diag_network`; substitutes `b2_network_status` | Has an authored `network-recovery-76` reference; while the program capability is blocked, private Overseer recovery (the switch under **НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**) is the reachable escape |
| `relay-storage-damaged` | `storage-damaged` | no transition | substitutes `b6_archive_record`; applies display instability | private Overseer recovery (the switch under **НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**) |
| `guard-authorization-corrupted` | `authorization-corrupted` | no transition | blocks `hack`; exposes `n5_diag_auth` | private Overseer recovery (the switch under **НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**) |
| `greenhouse-display-unstable` | `display-unstable` | no transition | applies display instability; exposes `n3_diag_display` | private Overseer recovery (the switch under **НЕИСПРАВНОСТИ ТЕРМИНАЛОВ**) |
| `air-spore-bloom` | `custom:biohazard` | `ventilation-main/contaminate` | blocks `view-entry` | `ventilation-main/purge`, `air-recovery-76`, or private Overseer recovery (the switch on the ventilation card) |

The matrix covers all four effect shapes (`capabilityBlock`, `diagnosticPath`,
`recordSubstitution`, and `displayInstability`), all five authored capability-block instances, and
all three stored recovery-reference shapes (device transition, recovery program, and private
Overseer action). The stored `recovery[]` entries are unchanged. In the fault dialog,
**КАК УСТРАНЯЕТСЯ** derives its rows from the clearing transitions and the programs that contain
them, and always ends with **Смотритель вручную**. Back and acknowledgement remain available while
capabilities are blocked, so every fault has an explicit escape path and no player route is an
accidental dead end.

## Version-1 round trip

Use **Open session → `sessions/demo.json` → Save session**, reopen the saved file, and start the
broadcast again. The result must remain `version: 1` and retain all terminals, exact-one group
membership, ordered local/remote transitions, command snapshots, EntryContent changes, facility
references, the linked player roster, and compatible unknown fields. Repeat the local/remote
transition pair and one ordinary command after reopening to prove the saved graph remains usable.
