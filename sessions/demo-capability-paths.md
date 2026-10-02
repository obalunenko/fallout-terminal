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
5. Facility authoring uses one explicit draft. In **ОБЪЕКТЫ**, changes made with
   **ПРИМЕНИТЬ К ЧЕРНОВИКУ** remain private until **СОХРАНИТЬ ИЗМЕНЕНИЯ СЕССИИ**; use
   **ОТМЕНИТЬ ИЗМЕНЕНИЯ** to restore the saved configuration. **ПРЕДПРОСМОТР** requires a clean
   draft and compares the saved current value with a detached hypothetical value. Use
   **СБРОСИТЬ УСТРОЙСТВО** or **СБРОСИТЬ ВЕСЬ ОБЪЕКТ** for confirmed live-state recovery.

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
| `n_entry_facility_status` | block entry + five bindings | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → СОСТОЯНИЕ УБЕЖИЩА** | Shows bound text for `power-grid-main`, `ventilation-main`, `reactor-main`, `robot-pod-atrium`, and `water-purifier-76`; initial/current deltas are already present. | **НАЗАД**; device/facility reset restores authored initial variants |
| `n_folder_diagnostics` | conditionally visible folder | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → АВАРИЙНАЯ ДИАГНОСТИКА** | Set or reset `power-grid-main=offline`; folder appears and exposes diagnostic entries. | **НАЗАД**; restore the grid to hide it |
| `n_diag_grid` | diagnostic entry | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → АВАРИЙНАЯ ДИАГНОСТИКА → НЕТ ПИТАНИЯ** | With the grid offline, explains the authored recovery transition. | **НАЗАД**; approve `n_cmd_prime_grid` or private recovery |
| `n_diag_door` | diagnostic entry | `t_demo1`: **СИСТЕМЫ ОБЪЕКТА → АВАРИЙНАЯ ДИАГНОСТИКА → ПРИВОД ГЕРМОДВЕРИ НЕ ОТВЕЧАЕТ** | Activate `door-offline` and keep the grid offline; shows its diagnostic and command block. | **НАЗАД**; `vault-door-main/open` or private recovery |
| `n_folder_actions` | folder | `t_demo1`: **ПРОТОКОЛ ВЫХОДА** | Opens ordinary, persistent, facility, recovery-program, and terminal-transition commands. | **НАЗАД** |
| `n_cmd_1` | persisted legacy state change + EntryContent | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Активировать маршрут выхода** | The shipped snapshot is already completed: completed label/result and `b_vault_shutdown_heading` persist. Reset its state in Overseer, request as controller, approve or reject as Overseer, then reopen **СВОДКА ЭВАКУАЦИИ**. | Acknowledge; **Сбросить состояние** restores the authored command form |
| `n_cmd_prime_grid` | persisted facility state change | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Запустить резервную сеть** | The shipped snapshot records completion and `power-grid-main=online`. Reset command state and the grid, request, then approve; the one-device recovery transition restores power and clears `grid-unpowered`. | Acknowledge; single-device reset returns the grid to `offline` |
| `n_cmd_open_evacuation` | atomic multi-device + EntryContent | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Открыть выход и подать сигнал** | Requires `power-grid-main=online`. Approval atomically opens `vault-door-main`, sets `evacuation-alarm=evacuation`, changes `b_vault_shutdown_body`, changes the command label, and reveals `n_entry_open_route`. | Acknowledge; reset the two devices and command state |
| `n_cmd_air_recovery` | recovery-program command | `t_demo1`: **ПРОТОКОЛ ВЫХОДА → Загрузить кассету очистки воздуха** | With `ventilation-main=contaminated` and `air-spore-bloom` active, approval runs `air-recovery-76`, purges ventilation, clears the condition, and changes bound air text. | Acknowledge; reset ventilation or the whole facility |
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
| `n3_diag_display` | conditionally visible diagnostic | `t_demo_hack_1`: **НЕСТАБИЛЬНОСТЬ ЭКРАНА** | Set `ventilation-main=filtered`; activate or preview `greenhouse-display-unstable` to demonstrate diagnostic-path and display-instability effects. | **НАЗАД**; private recovery or whole-facility reset |
| `n3_cmd_read_sensors` | ordinary command | `t_demo_hack_1`: **Снять показания спор** | Immediately shows greenhouse sensor readings. | Acknowledge |
| `n4_entry_manifest` | block entry + state variant | `t_demo_hack_2`: **ГРУЗОВАЯ ВЕДОМОСТЬ** | After the level-2 hack, the shipped `freight-elevator=upper` state shows the upper-level block variant. | **НАЗАД**; reset lift for its lower variant |
| `n4_cmd_call_lift` | facility state change | `t_demo_hack_2`: **Вернуть кабину вниз** | From `freight-elevator=upper`, approval applies `freight-elevator/lower` and changes the manifest and completed label. | Acknowledge; reset lift to authored `lower` or edit it back to `upper` to repeat |
| `n5_entry_security` | block entry + state variant | `t_demo_hack_3`: **ПЕРИМЕТР** | After the level-3 hack, shows `security-turret=safe`; setting it to `tracking` changes the bound status. | **НАЗАД**; reset turret |
| `n5_diag_auth` | conditionally visible diagnostic | `t_demo_hack_3`: **ТАБЛИЦА ДОПУСКА ПОВРЕЖДЕНА** | Set `security-turret=tracking` and activate or preview `guard-authorization-corrupted`; demonstrates the `hack` block and diagnostic path. | **НАЗАД**; private recovery or whole-facility reset |
| `n5_cmd_scan` | ordinary command | `t_demo_hack_3`: **Сканировать дорогу** | Immediately shows the exterior route scan. | Acknowledge |
| `n6_entry_archive` | block entry + active record substitution | `t_demo_hack_4`: **ПОСЛЕДНЯЯ ПЕРЕДАЧА** | After the level-4 hack, shipped `relay-storage-damaged` substitutes corrupted archive text and activates display instability. | **НАЗАД**; private recovery removes the condition |
| `n6_entry_robot` | block entry + state variant | `t_demo_hack_4`: **КУРЬЕРСКИЙ ПОСТ** | Shipped `robot-pod-atrium=patrol` shows its route variant; reset shows the charging text. | **НАЗАД** |
| `n6_cmd_decode` | ordinary command | `t_demo_hack_4`: **Повторить повреждённую передачу** | Immediately advances the RS-04 failure narrative. | Acknowledge |

## Facility authoring, diagnostics, preview, and reset

### Prepare a story with the in-app guide

Open **ОБЪЕКТЫ → КАК СОБРАТЬ СЦЕНАРИЙ · 5 ШАГОВ**. The guide is open by default;
collapse it during play to give the device controls more space. Select a device on the left
to make the guide use that device's draft. The counts describe configured relationships, not
a guarantee that the scenario is correct.

Use one continuous example: players encounter a locked door, request that it open from a
terminal, and see a status record change to **Путь свободен**.

| Step | Story decision | Configuration route |
|---|---|---|
| **1. Объект** | What can change? A door starts **Заперта** and can become **Открыта**. | Create a door device, its two named states and initial state. Add the **Открыть дверь** transition from closed to open. Applying selects the device in the guide. |
| **2. Действие** | What do players do? They request **Открыть дверь** on a terminal. | Use **СВЯЗАТЬ С КОМАНДОЙ** or edit an existing command action; choose the door and opening transition. If there is no suitable command, the guide routes to **ТЕРМИНАЛЫ** to create one. A transition alone does not create a player command. |
| **3. Результат** | How do players notice the change? A status record says **Путь свободен**. | Open the display-rule editor; choose the terminal, entry, block, and open state. Keep **Дверь заперта** as the block's original text and use **Путь свободен** for the open-state variant. Name, visibility, and availability rules are alternative display choices. |
| **4. Осложнения** | Optional: what obstructs the scene and how is it repaired? | Add a fault and choose its blocking effect; activate/remove it in transition effects. Operator recovery is a manual escape path. A recovery program combines transitions; return to **2. Действие**, open the command action, and choose that program under **ЧТО ЗАПУСТИТЬ**. |
| **5. Проверка** | Does the saved scene show the expected result? | Save the shared draft once, then preview the open state on the relevant terminal. Finally request the actual command as a player and approve it as Overseer. Preview checks presentation without performing the action. |

If a command or display destination is missing while your facility draft has changes, use
**СОХРАНИТЬ ЧЕРНОВИК И ОТКРЫТЬ ТЕРМИНАЛЫ**. This is an explicit save checkpoint:
terminal editing autosaves separately, so the guide first saves the device work and only
switches tabs after success. A rejected save keeps you on the guide with your selected draft
intact. Create the command or entry, then return to **ОБЪЕКТЫ** to bind it to the selected
device. Those new binding edits remain a draft until you explicitly save again. Merely
moving between guide steps never saves anything.

Prerequisites are checked against the state **before** a command or recovery program runs.
For example, if opening the door requires power already on, restoring power and opening that
door cannot satisfy the prerequisite within one atomic program: restore power first with a
separate action, then request the opening action. Use faults/programs only if the story needs
them; a basic door scene does not require either.

Relationship groups distinguish **Что делают игроки** (command actions) from **Что видят игроки**
(display and availability rules). Both retain contextual edit/remove controls. The guide adds no
content or saved progress merely by moving between steps; all changes still use the shared draft.

### Inspect and maintain existing scenes

The Overseer reaches every row from **ОБЪЕКТЫ** after opening the demo. Selecting an item exposes
its readable current/initial value and related rules beside the searchable device list. Technical
IDs remain under **Технические идентификаторы**. Device graphs summarize each transition as
source → destination; expand **ПРЕДУСЛОВИЯ И ЭФФЕКТЫ** only when editing advanced rules, including
the explicit **АКТИВИРОВАТЬ НЕИСПРАВНОСТЬ** / **СНЯТЬ НЕИСПРАВНОСТЬ** choice.

Device, binding, and recovery-program dialogs only update the shared draft. Review all staged
changes, then use the single **СОХРАНИТЬ ИЗМЕНЕНИЯ СЕССИИ** action. A failed save keeps the draft;
**ОТМЕНИТЬ ИЗМЕНЕНИЯ** discards it as one unit. Binding and program rows provide **Изменить** and
**Удалить** actions. **ЗАВИСИМОСТИ → ОТКРЫТЬ** follows readable references to their command, entry
block, device transition, condition, or recovery program. Selecting a command under
**ТЕРМИНАЛЫ** also shows **ДЕЙСТВИЕ УСТРОЙСТВ** and opens that command's preselected action editor.

**ПРЕДПРОСМОТР** resolves automatically when opened or when its terminal/state/condition changes.
It labels the saved baseline and hypothetical target, highlights changed and unavailable content,
lists hidden items, and never changes players or revisions. **ОБНОВИТЬ ПРЕДПРОСМОТР** remains a
retry action. Save or discard a dirty draft before preview, reset, or private recovery.

| Capability | Stable assets | Exact Overseer route and expected outcome | Recovery |
|---|---|---|---|
| All ten device kinds | `power-grid-main` (`power-grid`), `vault-door-main` (`door`), `evacuation-alarm` (`alarm`), `network-overseer` (`network-segment`), `ventilation-main` (`ventilation`), `reactor-main` (`reactor`), `security-turret` (`turret`), `robot-pod-atrium` (`robot-pod`), `freight-elevator` (`elevator`), `water-purifier-76` (`custom:water-purifier`) | **ОБЪЕКТЫ → Поиск устройств → [device] → РЕДАКТИРОВАТЬ ГРАФ**. Inspect readable transition summaries and expand advanced rules; **ПРИМЕНИТЬ К ЧЕРНОВИКУ** stages the complete graph. **ЗАВИСИМОСТИ** lists every command, binding, condition, and recovery reference before destructive edits. | Cancel leaves the form untouched; discard the shared draft or save it once |
| Direct single-device transition | `n_cmd_prime_grid` → `power-grid-main/restore` | Player route above, controller request, Overseer approval; exactly one device changes. | Reject leaves state unchanged; reset device |
| Direct atomic multi-device transition | `n_cmd_open_evacuation` → `vault-door-main/open` + `evacuation-alarm/sound-evacuation` | Player route above; both transitions commit together only after approval. | Reject/failure changes neither; reset devices/facility |
| Recovery program | `air-recovery-76`, `network-recovery-76` | **ОБЪЕКТЫ → [related device] → ПРОГРАММЫ ВОССТАНОВЛЕНИЯ → Изменить** for authoring; stage with **ПРИМЕНИТЬ К ЧЕРНОВИКУ** and publish with the session-wide save. Execute through `n_cmd_air_recovery` or `n2_cmd_network_recovery`. | Reset target device/facility; referenced programs explain why removal is blocked |
| Bound name/content/visibility/availability | `facilityNameVariants`, `facilityTextVariants`, `visibleWhen`, `availableWhen` | Use **ПРЕДПРОСМОТР** with the referenced state and terminal, then follow the corresponding player rows. Preview changes only the dialog; saving or an approved transition publishes the canonical variant to controller and observer. | Close preview; reset device/facility |
| State preview | any device, for example `vault-door-main=open` on `t_demo1` | With no dirty draft: **ОБЪЕКТЫ → Поиск устройств → Гермодверь Убежища 76 → ПРЕДПРОСМОТР СОСТОЯНИЯ → СОСТОЯНИЕ: Открыта → ТЕРМИНАЛ: t_demo1**. It resolves automatically and compares the saved state with the open-route projection without changing revision or players. | **ЗАКРЫТЬ** |
| Condition preview | any condition, for example `relay-storage-damaged` on `t_demo_hack_4` | With no dirty draft: **ОБЪЕКТЫ → ДИАГНОСТИЧЕСКИЕ УСЛОВИЯ → Архив ретранслятора повреждён → ПРЕДПРОСМОТР УСЛОВИЯ → АКТИВНО**. The comparison resolves automatically and shows substitution/instability only in preview. | **ЗАКРЫТЬ** |
| Single-device reset | a device whose current differs from initial, including shipped `power-grid-main`, `reactor-main`, `robot-pod-atrium`, `freight-elevator`, or `water-purifier-76` | **ОБЪЕКТЫ → УСТРОЙСТВА → [device] → СБРОСИТЬ УСТРОЙСТВО → confirm**. Restores that device and its device-scoped initial conditions in one revision without changing unrelated devices. | Reapply an authored transition if desired |
| Whole-facility reset | `facility.revision=12` graph | **ОБЪЕКТЫ → СБРОСИТЬ ВЕСЬ ОБЪЕКТ → confirm**. All devices and conditions return to authored initial values in one revision. | Continue from the authored initial scenario |

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
