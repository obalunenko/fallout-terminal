# Data Model: Facility Editors and Language

Portable session JSON version 1 is unchanged (FR-042). This feature adds no session field and
renames none. It changes how five editors read and write fields that already exist, and the meaning
of one stored value, `privateOverseerAction` (FR-020). Every other entity below is one of two kinds:

- A derived view, computed from the session on each render and never stored.
- Transient Overseer state, which lives in `overseer.js` for one dialog or one application run.

Screen copy, DOM ids, and rendered upper-case forms (R15) are in `contracts/overseer-ui.md`. The
removed desktop method is in `contracts/private-desktop.md`. Spec strings are quoted here in the
spec's own case.

## Terminology

The glossary governs Overseer copy only. Code names (`condition`, `binding`, `DiagnosticCondition`)
and session JSON field names stay as they are.

| Glossary term | Go type | Session JSON location |
|---|---|---|
| Локация | the whole scene: `domain.Facility` plus the terminals it drives | `facility`, `terminals[]` |
| Устройство | `FacilityDevice` | `facility.devices[]` |
| Состояние | `FacilityDeviceState` | `facility.devices[].states[]` |
| Переход | `FacilityDeviceTransition` | `facility.devices[].transitions[]` |
| Неисправность | `DiagnosticCondition` | `facility.conditions[]` |
| Программа восстановления | `RecoveryProgram` | `facility.recoveryPrograms[]` |
| Правило отображения | one `FacilityTextVariant` in `facilityNameVariants[]` or `blocks[].facilityTextVariants[]`, or one `visibleWhen` or `availableWhen` equality | a content node in `terminals[].root` |
| Действие команды | `StateChangeConfig.FacilityAction` | a command's `stateChange.facilityAction` |

A fault is device-scoped when `condition.device.deviceId` is set. It is terminal-scoped when
`condition.terminal.terminalId` is set. Exactly one of the two is set.

## Persisted values (existing fields only)

### Which editor writes which field

| Session JSON field | Written by | Rule |
|---|---|---|
| `devices[].id`, `name`, `kind`, `customKind` | Device dialog | `id` is generated for a new device only. `customKind` follows today's rule: kept or set to the ID for `custom`, removed otherwise. |
| `devices[].initialStateId` | Device dialog: the starred chip | A template sets it only when the draft has no state (FR-011). |
| `devices[].currentStateId` | Device dialog for a new device only, where it equals `initialStateId`; 040 play changes | An existing device keeps its value. |
| `devices[].states[]` (`id`, `name`) | Device dialog: chips and templates | — |
| `devices[].transitions[]` (`id`, `name`, `sourceStateId`, `destinationStateId`, `preconditions[]`) | Device dialog: the matrix panel and templates | — |
| `devices[].transitions[].conditionEffects[]` | Fault dialog only (FR-017) | The device dialog carries them through unchanged. |
| `devices[].transitions[].recovery` | Never | Carried through by the row spread (R2, R8). |
| `conditions[]` (`id`, `name`, `category`, `customCategory`, `device` or `terminal`, `initialActive`, `effects[]`) | Fault dialog | Only the `capabilityBlock` effect is edited. Other effects are kept as stored. |
| `conditions[].currentActive` | Fault dialog for a new fault only, where it equals `initialActive`; the 040 fault switch, including the new terminal-fault group | — |
| `conditions[].recovery[]` | Never added to | An entry is dropped only when the same edit invalidates it (see Stored recovery entries). |
| `recoveryPrograms[]` | Program dialog | — |
| `…facilityNameVariants[]`, `…blocks[].facilityTextVariants[]`, `…visibleWhen`, `…availableWhen` | Display-rule dialog | An edit replaces the value in place (R5). |
| `…stateChange.facilityAction`, plus `completedName` and `confirmationText` when `stateChange` is created | Command-action dialog | Other `stateChange` fields are kept. |
| `facility.revision`, authoring revision | `commitFacilityEdit` → `commitAuthoringEdit` → `SaveAuthoring` (039) | One save, one revision check, and one «Отменить» entry per «Готово» (FR-043). |

Every editor keeps unknown fields. Each draft row is read as a spread over its `facilityOriginal`,
as `readFacilityDeviceDraft` does today (R8).

### Domain rule change (R1)

| | Rule in `validateConditions` (`internal/domain/validate.go`) |
|---|---|
| Before | A fault fails when `(blocksProgress \|\| len(recovery) != 0) && !hasViableRecovery`. `blocksProgress` means any `capabilityBlock` effect. |
| After | The `blocksProgress` trigger goes away. A fault with an empty `recovery[]` is valid, whatever it blocks. |
| Unchanged | Each stored entry passes `validateRecoveryReference`. It has exactly one variant, and its references resolve. A referenced transition, and every transition of a referenced program, carries `recovery: true`. `privateOverseerAction` is `true` when present. A non-empty `recovery[]` still needs one entry that clears the fault. |

The last unchanged rule keeps R2's stale-entry case an error. Pruning (below) is what keeps the
editors' saves valid. Validation now accepts every document it accepted before, plus faults with no
recovery entry (Constitution VI).

### Stored recovery entries (R2)

`condition.recovery[]` stays in the file, and no editor adds to it. An edit drops an entry only when
that same edit invalidates it. Here F is the fault that owns the entry:

| Entry variant | Dropped when the edit… |
|---|---|
| `transition` `{deviceId, transitionId}` | removes the referenced transition, or removes its `{conditionId: F, active: false}` effect |
| `recoveryProgramId` | deletes the program, leaves the program with no transition that clears F, or adds a program transition whose `recovery` flag is not set |
| `privateOverseerAction` | never |

- **Edits that prune**: the fault dialog, when a clearing transition is removed. The program dialog,
  when rows change or a program is deleted. That includes entries of every fault that referenced the
  program.
- **Edits that block**: the device dialog never prunes. A transition that a stored entry references
  counts as an outside use and blocks its deletion (R10).
- **Lists**: «Как устраняется» is derived from effects, not from `recovery[]`. Pruning therefore never
  changes what the author sees.

### `privateOverseerAction` (FR-020)

The value is read and kept, but never written or removed. It no longer gates anything, because the
private recovery path that read it goes away (R1). «Смотритель вручную» is feature 040's fault
switch, which works for any fault in any scope.

## Glossary (FR-034, FR-035, R14)

`docs/overseer-glossary.md` holds one row per term:

| Field | Content |
|---|---|
| Term | Локация, Устройство, Состояние, Переход, Правило отображения, Действие команды, Неисправность, Программа восстановления, and the pair Готово / Отмена |
| Rendered form | The upper-case form controls and titles use (R15) |
| Meaning | One sentence, tied to the model entity in Terminology |
| Banned synonyms | The wordings the check rejects for that term |

The vocabulary check (`tests/browser/facility-vocabulary.spec.mjs`) fetches the served
`index.html`, `overseer.js`, and `desktop-api.js`. It matches case-insensitively, with the flags
`iu` and the boundaries `(?<!\p{L})…(?!\p{L})`.

| Banned pattern | Names wrongly | Passes |
|---|---|---|
| Any form of «объект» except the two allowed strings | a device or the location | «ОБЪЕКТЫ» (the tab), «ОБЪЕКТЫ И СОСТОЯНИЯ» (the heading) |
| «диагностическ…» followed by «услови…» | a fault | — |
| A standalone «услови…» | a fault | «предусловие», because a letter precedes it |
| «Восстановить условие» | clearing a fault | — |
| «Восстановление оператором» | the removed setting | — |

Authored session content, such as the demo folder «СИСТЕМЫ ОБЪЕКТА», is never scanned. The 039
`REMOVED_COPY` test stays. The scenario guide's first step label changes from «Объект» to
«Устройство».

## Device dialog (US1)

### Device draft

One draft per open dialog. It is held in the dialog's markup, and `readFacilityDeviceDraft` reads
it. It saves as one edit.

| Field | Control | Rule |
|---|---|---|
| `name` | «НАЗВАНИЕ» | Required, trimmed, at most 256 bytes |
| `kind` | «ТИП» | Required. One of the ten `FacilityDeviceKind` values. |
| Template offer | One line under the type | Present when `kind` has a template (see Device template) |
| `states[]` | State chips, in authored order | At least 1, at most 1,000 |
| `initialStateId` | The star on one chip | Exactly one chip is starred whenever states exist. A new device's first state is starred (FR-003). |
| `currentStateId` | none | Kept for an existing device. A new device gets the starred state. |
| `transitions[]` | The matrix and its panel | At most 10,000 |
| Identifiers | «Технические идентификаторы» | See Technical identifiers |

### State chip

| Field | Rule |
|---|---|
| `name` | Required, trimmed, at most 256 bytes. Unique within the device after trimming and lower-casing, the domain's normalized name. |
| `id` | See Technical identifiers. Prefix `state`, without the prefix kept (`suggestFacilityID('state', name, ids, false)`). |
| Starred | True for the chip whose `id` is `initialStateId` |
| `facilityOriginal` | The stored state, or `{}` for a new chip |

Choosing a chip opens its inline editor, which offers rename, «сделать начальным», and «удалить».
«+ Состояние» appends a chip and opens its editor.

### Transition matrix (derived)

The matrix is recomputed from the draft on every change. It is an authoring view and runs nothing.

| Element | Definition |
|---|---|
| Rows | Draft states in authored order, as sources |
| Columns | Draft states in authored order, as destinations |
| Cell (s, d) | Draft transitions with `sourceStateId = s` and `destinationStateId = d`, in authored order. Zero or more. |
| Diagonal cell (s, s) | Unavailable, because the domain requires the destination to differ from the source |
| Filled cell label | The transitions' names, then a summary such as «1 предусловие · снимает 1 неисправность». It counts preconditions, faults turned on, and faults cleared, with Russian plural agreement, and leaves out zero parts. |
| Cell control | A `<button>` with `data-from` and `data-to`. A diagonal cell is not a control. |

- **One state**: a device with one state shows a single unavailable cell and says a transition needs a
  second state.
- **Many states**: the matrix scrolls inside its own frame. The headers stay readable, and the dialog
  buttons stay visible.

| Cell chosen | Result |
|---|---|
| Empty | A new transition `{name: <suggested>, sourceStateId: s, destinationStateId: d, preconditions: [], conditionEffects: []}` opens in the panel (FR-005). The suggested name's copy is in the UI contract. |
| One transition | That transition opens in the panel |
| Several transitions | The cell lists their names and offers another transition. Choosing a name opens it. |

### Transition panel

The panel sits under the matrix, inside the same form. It edits one draft transition.

| Field | Rule |
|---|---|
| `name` | Required, trimmed, at most 256 bytes |
| `sourceStateId`, `destinationStateId` | Draft states. They must differ. Changing either moves the transition to another cell. |
| `preconditions[]` `{deviceId, stateId}` | At most 1,000. The device list leaves out this device and any device already used in this transition. `stateId` must be a state of that device. |
| Fault effects | Read-only. Each row names the fault and whether the transition turns it on («включает») or clears it («снимает»). Each row has «К неисправности», which is blocked with «Сначала нажмите «Готово»» while the draft has unsaved changes. |
| Delete | Allowed unless an outside use names the transition: a command action, a program, or a stored `recovery[]` `transition` entry. Each use is listed with its destination button, as in state deletion. |

### Technical identifiers (FR-007)

- **Block**: one collapsed `<details>` «Технические идентификаторы» at the bottom of the form holds the
  device ID, then every state ID, then every transition ID, in dialog order.
- **New rows**: a new row's ID follows its name through `suggestFacilityID` until the author edits the
  ID (`data-manual`).
- **Saved IDs**: an ID that exists in the saved session is read-only and never regenerated.
- **ID rule**: trimmed, non-empty, at most 256 bytes, and unique. Device IDs are unique across the
  facility. State IDs and transition IDs are unique within the device.
- **ID problems**: `setFacilityFeedback` opens the block, marks the row `aria-invalid`, and focuses
  it. That covers an empty or a duplicate ID.

### State deletion (FR-008, R10)

«удалить» on the chip for state S of device D runs these checks in order:

| Check | Source | Outcome |
|---|---|---|
| S is starred | Draft | Refused: the dialog says to star another state first |
| S is D's saved `currentStateId` | Session | Refused: the dialog says to move the device first |
| Outside uses | Session | Blocks deletion. Each use is listed by name with its destination button (Dependency link). |
| Inner uses | Draft | A confirmation names the transitions that go with S. Confirming deletes S and those transitions. |

Outside uses of S:

- Another device's precondition `{D, S}`.
- A display rule whose equality is `{D, S}`: a name variant, a text variant, `visibleWhen`, or
  `availableWhen`.
- A command action, a program, or a stored `recovery[]` `transition` entry that references one of
  D's transitions starting or ending in S.

Inner uses are D's draft transitions with `sourceStateId` or `destinationStateId` equal to S. New
draft transitions have no outside uses.

### Validation on «Готово»

The client checks run in this order and stop at the first problem. Each one focuses the field.

1. Name present. Kind chosen. At least one state.
2. Row problems in reading order:
   - A state name that is empty or a duplicate.
   - A state ID that is empty or a duplicate.
   - A transition name that is empty.
   - A transition ID that is empty or a duplicate.
   - A source or destination that is unknown or equal.
3. Device ID unique across the facility.
4. The starred state exists. The saved current state exists.
5. Every precondition names another device and one of its states. No device appears twice in one
   transition.

Then `commitFacilityEdit('Устройство', mutate)` saves, and the server's validation decides (R8). A
rejected save reopens the dialog with the draft intact and the message from Overseer error.

```text
closed ──open──▶ editing ──chip──▶ chip editor ──done──▶ editing
                   │ ──cell──▶ (cell list) ──▶ transition panel ──close──▶ editing
                   │ ──«Применить шаблон»──▶ editing (draft extended)
                   └─«Готово»─▶ client checks ──problem──▶ editing, field focused
                                     └─ok─▶ saving ──accepted──▶ closed
                                                └─rejected──▶ editing, draft kept
```

## Device template (US3, FR-009 to FR-012, R9)

`FACILITY_DEVICE_TEMPLATES` in `overseer.js` holds the templates. They are editor suggestions, and
no session stores them.

| Field | Type | Rule |
|---|---|---|
| key | `FacilityDeviceKind` | Every kind except `custom`: nine templates |
| `states` | Ordered state names | At least two |
| `initial` | One of `states` | The starting state |
| `transitions` | `{name, from, to}`, with state names | At least one in each direction between the states |

The «Дверь» (`door`) template has the states «Заперта» (starting) and «Открыта», and the transitions
«Открыть» (Заперта → Открыта) and «Запереть» (Открыта → Заперта). The other eight follow the demo
device of the same kind. Their names are listed under Templates in `contracts/overseer-ui.md`.

- **Offer**: shown when the draft's `kind` has a template. It reads «Применить шаблон» with the
  summary «<states> · <transitions>», for example «Заперта, Открыта · Открыть, Запереть» (FR-010).
  Neither `custom` nor an empty kind shows it.
- **Type change**: changing the kind updates the offer and nothing else (FR-012).

Applying a template changes only the draft. «Готово» saves it.

1. For each template state, in order: if no draft state has the same normalized name (trimmed,
   lower-cased), append `{id: suggestFacilityID('state', name, ids, false), name}`.
2. If the draft had no state before step 1, star the template's `initial` state. Otherwise the star
   stays where it is.
3. Resolve each template state name to a draft state ID: the matching existing state, or the new
   one.
4. For each template transition: if no draft transition has the same normalized name between the
   same pair of resolved states, append it with a generated ID, no preconditions, and no effects.

Nothing is renamed or removed. Applying twice adds nothing the second time.

## Fault dialog (US2, FR-013 to FR-021)

### Fault draft

| Field | Control | Rule |
|---|---|---|
| `name` | «НАЗВАНИЕ» | Required, trimmed, at most 256 bytes |
| `category` | «КАТЕГОРИЯ» | Required. One of the seven categories. `customCategory` follows today's rule. |
| Scope | Where it acts: a device or a terminal | Exactly one, and it must exist. A terminal scope must match the terminal of every stored `diagnosticPath` and `recordSubstitution` effect. The dialog names a mismatched effect instead of saving. |
| Blocked capability | One select | One of the five `FacilityCapability` values, or none. It writes the single `capabilityBlock` effect. A new fault must choose one, as today. |
| Other effects | none | `diagnosticPath`, `recordSubstitution`, and `displayInstability` are kept as stored |
| `initialActive` | Position at game start | Either value |
| Activating list | «Как появляется» | Starts as derived from the session. The author adds and removes rows. |
| Clearing list | «Как устраняется» | Starts as derived from the session. The author adds and removes transition rows. |
| `id` | «Технические идентификаторы» | Generated from the name for a new fault. Read-only once saved. |

The «Восстановление оператором» checkbox is removed.

### «Как появляется» and «Как устраняется» (derived, R2)

T is every `(device, transition)` pair in the session with the draft's list edits applied. Rows are
ordered by device order, then by transition order.

| List | Rows |
|---|---|
| «Как появляется» | Each t in T with the effect `{conditionId: F, active: true}`, shown as «<device> — <transition>» with a remove control |
| «Как устраняется», transitions | Each t in T with the effect `{conditionId: F, active: false}`, removable |
| «Как устраняется», programs | Each program in the session that contains at least one of those clearing transitions. A row opens its program and is not removable here. |
| «Как устраняется», last row | «Смотритель вручную», always present (FR-020) |

Warnings in «Как появляется»:

| Draft | Warning |
|---|---|
| No activating row, and `initialActive` false | «Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.» |
| No activating row, and `initialActive` true | The fault is active from the start. After it clears, only the Overseer can turn it on again. |

A fault with no clearing transition and no program saves. Its «Как устраняется» lists only
«Смотритель вручную».

### Pickers

«+ Включать при переходе…» and the clearing add control both offer T as «<device> — <transition>»,
grouped by device. The domain would refuse some combinations, so these are left out:

- A transition already in the same list.
- A transition already in the other list. One transition cannot both turn on and clear F (R3).
- A transition that shares a command action or program with another transition that has the
  opposite effect on F. The domain refuses contradictory effects in one action.

### Save (R3)

«Готово» makes one `commitFacilityEdit` with one `mutate`. It finds every target by ID, so it
replays on a newer session.

1. Upsert the condition by its original ID, or append a new one. A new fault gets
   `currentActive = initialActive`.
2. For every transition in the saved or the draft lists, find it by device ID and transition ID.
   Remove its effect for F, then add `{conditionId: F, active}` when the draft lists it. An effect
   whose value did not change keeps its stored object and position. A target that no longer exists
   is skipped, and the server decides.
3. Prune the stored `recovery[]` entries of F that the edit invalidates (Stored recovery entries).
   `privateOverseerAction` is never touched.

The «Отменить» label is «Неисправность «<name>»». A rejected save keeps the dialog open with the
draft intact.

### Openers (FR-019)

Choosing a fault opens this dialog from each of these places:

- A row of the «Неисправности» tab. The «ВЫБРАННОЕ УСЛОВИЕ» panel is removed.
- A row of the «Неисправности терминалов» group.
- A fault row on the device card.
- «К неисправности» in the transition panel.
- A dependency link to a fault.

```text
fault dialog (draft) ──«+ Программа восстановления…»──▶ program dialog, modal over it (R4)
      ▲                                                     │ «Готово»: its own save
      │                                                     │ «Отмена»: nothing saved
      └──── focus returns, fault draft untouched, lists re-derived ◀┘
fault dialog ──«Готово»──▶ one save (condition, effects, pruned entries)
                                ├─accepted──▶ closed
                                └─rejected──▶ fault dialog, draft kept, message shown
```

## Recovery program dialog (FR-016, FR-018, R4)

| Field | Rule |
|---|---|
| `name` | Required, trimmed, at most 256 bytes |
| `id` | In «Технические идентификаторы». Generated with prefix `program`, and read-only once saved. |
| `transitions[]` `{deviceId, transitionId}` | One uniform list of removable rows with one add control. At least 1 row, at most 1,000. |
| Commands that run it (derived) | Every command whose `stateChange.facilityAction.recoveryProgramId` is this ID, shown as «<terminal> › … › <command>». With none, the dialog warns that no command runs the program. |

The rows follow the domain's rules for one action (`validateTransitionRequests`):

- Each device appears at most once. The add control leaves out devices already used.
- The transition exists on its device.
- No two rows have opposite effects on one fault.
- A row's precondition on another row's device must equal that row's source state.
- Preconditions form no cycle.

The separate first-transition fields and the multi-sentence instruction are removed.

When the dialog opens from «Как устраняется», its draft carries a return target:

| Field | Value |
|---|---|
| Return target | The open fault dialog |
| Prefill | The first transition in the fault draft's clearing list, or no row when that list is empty |

«Готово» saves the program as its own edit. «Отмена» saves nothing. Either way focus returns to the
fault dialog. The fault dialog's draft is untouched and its lists are re-derived against the new
session.

## Display rule (US4, FR-022 to FR-028)

### Types

| Tab | Property key | Stored as | «ГДЕ» offers | Fields shown |
|---|---|---|---|---|
| «Текст записи» | `entry-content` | `node.blocks[b].facilityTextVariants[i]` = `{when, text}` | Records (`entry`) that have blocks | «КОГДА», «ГДЕ», «БЛОК», «ТЕКСТ» |
| «Название в меню» | `menu-name` | `node.facilityNameVariants[i]` = `{when, text}` | Any element except a terminal root | «КОГДА», «ГДЕ», «ТЕКСТ» |
| «Показать элемент» | `visibility` | `node.visibleWhen` = `{deviceId, stateId}` | Any element except a terminal root | «КОГДА», «ГДЕ» |
| «Разрешить команду» | `availability` | `node.availableWhen` = `{deviceId, stateId}` | Commands only | «КОГДА», «ГДЕ» |

«Действие команды» is not a type here (FR-029). Only the selected type's fields and labels are
visible.

### Element catalog (R5)

The catalog is built when the dialog opens and walks every terminal. Option values index into it.

| Field | Content |
|---|---|
| `terminalId`, `nodeId`, `type` | The element's identity and type (`folder`, `command`, `entry`) |
| `label` | «<terminal> › <folder> › … › <element>» |
| `isRoot` | True for a terminal root, which «ГДЕ» never offers |
| `blocks[]` | For a record: `{id, label}`, where the label is «<n> · «<opening text>»». `n` is the block's position from 1, and the text comes from `normalizedEntryBlockPreview`. |

The separate «ТЕРМИНАЛ» select is removed.

### Rule draft

| Field | Rule |
|---|---|
| Type | The selected tab |
| `deviceId`, `stateId` («КОГДА») | Both required. The state belongs to the device. |
| Element («ГДЕ») | Required. It must pass the type's filter and still exist at «Готово». If it does not, the dialog names the missing element and nothing saves. |
| `blockId` («БЛОК») | «Текст записи» only. Required, and it must be a block of the element. |
| `text` («ТЕКСТ») | «Текст записи», up to 1 MiB, and «Название в меню», up to 256 bytes. The dialog requires it for both, as today. |
| Editing | `null` for a new rule, otherwise the rule's key `{terminalID, nodeID, blockID, property, index}` (`facilityBindingKey`) |

Tab switching:

- **New rule** (FR-026): the device and state stay. The element stays when the new type's filter
  accepts it. Otherwise it is cleared. A field the new tab does not show is not saved.
- **Existing rule**: the type is fixed. The other tabs carry `aria-disabled` with the reason «Чтобы
  сменить тип, создайте новое правило».

The dialog says what players see in the device's other states (FR-027):

| Type | Other states |
|---|---|
| «Текст записи» | The block keeps its original text: «В остальных состояниях останется исходный текст блока» |
| «Название в меню» | The element keeps its authored name |
| «Показать элемент» | The element is hidden |
| «Разрешить команду» | The command is unavailable |

### Conflicts (FR-028, R6)

The dialog checks before saving. It reads `facilityBindingCatalog` and leaves out the rule being
edited.

| Conflict | Types | Detected when |
|---|---|---|
| Another controlling device | «Название в меню» (the node's list), «Текст записи» (the block's list) | The list has a variant whose `when.deviceId` differs from the draft's device |
| Repeated state | The same two types | The list has a variant with the draft's device and state |
| Existing equality | «Показать элемент», «Разрешить команду» | The node already has `visibleWhen` or `availableWhen` |

On a hit the dialog names the existing rule by type, element, device, and state. It offers «Открыть
правило», which loads that rule into the dialog, and does not save.

### Save

The `mutate` finds the node by terminal ID and node ID.

- **New rule**: it appends to the list, or sets the equality.
- **Existing rule**: it replaces the variant at its index, or overwrites the equality, so the
  variant order stays (R5).

Variant extras are kept. The «Отменить» label is «Правило отображения «<element>»».

## Command action (FR-029, R7)

`#facilityCommandActionDialog` holds one draft:

| Field | Rule |
|---|---|
| Command | Fixed when the dialog opens from the command form or a command's row. Otherwise picked from a commands-only «КОМАНДА» list. The list holds command nodes without `terminalTransition`, labeled with their folder path. |
| Mode («ЧТО ЗАПУСТИТЬ») | `transitions` («ПЕРЕХОДЫ») or `program` («ПРОГРАММА ВОССТАНОВЛЕНИЯ»). It opens on the stored variant, or on `transitions` when no action exists. |
| Transition rows | Mode `transitions`. Same row rules as a recovery program. At least one row. |
| Program | Mode `program`. Must name an existing program. |
| `completedName`, `confirmationText` | Shown and required only when the command has no `stateChange` yet |

| Opened from | Command |
|---|---|
| The command form («ДЕЙСТВИЕ УСТРОЙСТВ», enabled for every command) | Fixed |
| A row of the «Что делают игроки» tab | Fixed to that row's command |
| The add control of the «Что делают игроки» tab | Picked |
| The scenario guide's action step | Picked |

A program-backed action opens this dialog with its program selected. It no longer opens the program
dialog.

The save does three things:

- It sets `stateChange` to `{completedName, confirmationText}` when the command has none.
- It replaces `stateChange.facilityAction` with exactly one of `{recoveryProgramId}` or
  `{transitions: {transitions: [...]}}`.
- It keeps every other `stateChange` field, such as `entryContentChange`, and all extras.

The «Отменить» label is «Действие команды «<command>»». Existing actions open here and run as
before.

## Device list (US5, FR-030 to FR-033, R11)

### Row (derived per render)

| Value | Definition | DOM |
|---|---|---|
| Name | `device.name` | Row title, left-aligned |
| Type | `facilityDeviceKindLabel(device)` | Under the name |
| State badge | The name of `currentStateId` | `.facility-state-badge` on the right; `data-current-state-id` |
| Changed | `currentStateId !== initialStateId` | `data-changed` on the badge, drawn with a yellow frame |
| Active faults | Device-scoped faults of this device with `currentActive` | `.facility-fault-mark` «● N» in red when N > 0; `data-active-faults` |
| Selected | `id === selectedFacilityDeviceID` | `data-facility-selected`, `aria-pressed` |

Rows keep the authored device order. `row.title = id` is removed, so IDs no longer show on hover.

### Filter and search (transient, current run)

| State | Values | Lifetime |
|---|---|---|
| `facilityDeviceFilter` (module `let`) | `'all'` (default), `'changed'`, `'faulted'` | The application run, across session switches |
| Search text | `#facilityDeviceSearch`, placeholder «Название или состояние» | The application run |

- **Search**: an empty query matches every device. Otherwise the trimmed query, lower-cased with
  `toLocaleLowerCase`, must be a substring of the name or the current state name. IDs are no longer
  searched.
- **Filter**: `all` matches every device, `changed` the changed ones, and `faulted` those with active
  faults above 0.
- **Visible rows**: a row shows when it matches both the filter and the search (FR-031).
- **Counts**: «Все» counts every device, «Изменённые» the changed ones, and «С неисправностью» those
  with active faults. Counts cover the whole session and ignore the search.
- **No match**: the list says that no device matches and offers «Все», which sets the filter to
  `all` and clears the search (FR-033).
- **Hidden selection**: when the filter or search hides the selected device, its details stay open.
  The line «Выбранное устройство скрыто фильтром» sits above the list.
- **Updates**: `renderAll` rebuilds rows, badges, marks, counts, and results on every state or fault
  change (FR-032).

```text
        press «Все» / «Все» in the empty state
  ┌──────────────────────────────────────────┐
  ▼                                          │
 all ◀── press ──▶ changed ◀── press ──▶ faulted
```

Exactly one filter is pressed at a time, shown by `aria-pressed`. In the current demo the counts are
«Все 10», «Изменённые 5», and «С неисправностью 1». The changed devices are the grid, the reactor,
the robot post, the elevator, and the water purifier. The faulted device is the ventilation loop.

### Terminal fault group (FR-021)

`<ul id="facilityTerminalFaultList">` sits after `#facilityDeviceList`. It renders whether or not a
device is selected, and search and filters never narrow it.

| Value | Definition |
|---|---|
| Rows | Every fault with a `terminal` scope, in authored order |
| Row identity | `<li data-facility-kind="terminal-condition" data-facility-id="<conditionId>">` |
| Name, terminal | The fault name and the scope terminal's name |
| Active mark | `currentActive` |
| Open control | Opens the fault dialog |
| Switch | `role="switch"` with `aria-checked` set to `currentActive`, copied from `renderFacilityPlayFaults`. It calls `runFacilityFaultSwitch(id, !active)`, which runs 040's `SetFacilityConditionActive` with the facility revision check. It is disabled for the 040 block reasons. |
| Result | `#facilityTerminalFaultStatus`, next to the group |

The demo has four rows. Two are active: «Закрытый канал изолирован» and «Архив ретранслятора
повреждён». A terminal fault switch adds no device feed entry, as in 040.

```text
inactive ──switch on (expected revision)──▶ active
active ──switch off / transition effect / reset to initialActive──▶ inactive
stale revision ──▶ unchanged, canonical session shown, message in #facilityTerminalFaultStatus
```

## Dependency link (US6, FR-037, R13)

The input is one `FacilityDependency` `{kind, sourceId, targetId, property, terminalId?}` from the
unchanged `BuildFacilityDependencyReport`. For a device target the report contains only the kinds
below. `describeFacilityDependency(dependency, session)` derives the view:

| Field | Content |
|---|---|
| Sentence | Words naming what depends on what, for example «Команда запускает переход «Открыть гермодверь»» |
| Place | By name. For an element, «<terminal> › … › <element>». For a transition, «<device> — <transition>». Otherwise the fault or program name. |
| Button | Names the destination (table below) |
| Technical path | `property`, `sourceId`, and `targetId` under a collapsed «подробности для разработчика», built by `facilityTechnicalDetails(parent, text, summary)` |

| Kind | Source resolved by | Button | Opens |
|---|---|---|---|
| `transition-precondition` | `${device.id}/${transition.id}` matched against `sourceId` over all devices. The ID is never split, because IDs may contain `/`. | «К устройству» | That device's dialog, with the transition open in the panel |
| `recovery-reference`, `diagnostic-scope` | The fault with ID `sourceId` | «К неисправности» | The fault dialog |
| `recovery-program-transition` | The program with ID `sourceId` | «К программе» | The program dialog |
| `command-action` | Node `sourceId` in terminal `terminalId` | «К команде» | The command in the terminal workspace |
| `name-variant`, `entry-content-variant`, `visibility`, `availability` | Node `sourceId`. For `entry-content-variant` it is a block ID, resolved through `entryBlockLocation`. | «К правилу отображения» | The display-rule dialog on the matching catalog entry |

The list index inside `property` is read with `\[(\d+)\]`. It finds the precondition, variant, or
request that the sentence names. When a source no longer resolves, the button reports that the
linked element is gone and does not navigate.

## Overseer error (US7, FR-038 to FR-041, R12)

### Catalog

| Table | Key | Value |
|---|---|---|
| `OVERSEER_ERROR_MESSAGES['player-config']` | The exact Go text of each literal `failure(...)` in `internal/playerconfig/service.go` | A specific Russian message with a next step |
| `OVERSEER_ERROR_MESSAGES['session']` | The exact Go text of each literal `sessionFailure(...)` and `saveFailure(...)` in `internal/session/service.go` | A specific Russian message with a next step |
| `facilityFailureMessages` | `FacilityFailureCode` | A facility message reworded to the glossary |
| General messages | One entry per other area, such as public access, updates, or credential storage | One Russian message with a next step for that area |

«could not read the player config» maps to «Не удалось прочитать файл игроков. Выберите его заново
или создайте новый.» (FR-040). A failure built from `err.Error()` has no fixed text and gets its
area's general message.

### Rendered error

`renderOverseerError(sink, result, area)` replaces every `result?.error || '<fallback>'` sink.

| Output | Rule |
|---|---|
| Message | The first hit in this order: the facility failure code, the exact text in the area's table, the area's general message, the global general message. It is always Russian and always names a next step. |
| Disclosure | A `<details>` «подробности для разработчика» holding the original text. Omitted only when there is no original text. |
| Log | The original text is logged as today |

Raw text never appears alone. A failure that arrives while a dialog is open shows in that dialog,
and the dialog keeps its draft.

Two Go tests fail when a Go text has no catalog key: `internal/playerconfig/messages_test.go` and
`internal/session/service_messages_test.go`. Both read `frontend/overseer/src/overseer.js`.

### Facility validation problem (FR-038)

`facilityIssueMessage` turns each `FacilityIssue` `{code, entityKind, entityId, referenceKind,
referenceId}` into «<kind> «<name>»: <message for code>».

| `entityKind` | Kind label | Name looked up |
|---|---|---|
| `device` | Устройство | The device by ID |
| `device-state` | Состояние | The state on the owner device (`referenceKind` `owner`, `referenceId`) |
| `device-transition` | Переход | The transition on the owner device |
| `condition` | Неисправность | The fault by ID |
| `recovery-program` | Программа восстановления | The program by ID |

When the name cannot be found, the message uses the kind label alone. Raw kinds and IDs appear only
in the disclosure. The authoring validator returns only `invalid-configuration` for most graph
errors, so the client checks in the device, rule, and deletion flows catch the common cases first.

## Removed (Principle VII)

| Removed | Replaced by |
|---|---|
| `RecoverFacilityCondition` (control, App, contract converters, desktop method), `RecoverFacilityConditionRequest`, the session recovery branch (`applyConditionRecoveryLocked`, `resolveConditionRecovery`, `WorldActionRequest.RecoveryConditionID` and `.Recovery`), `desktopAPI.recoverFacilityCondition` | The 040 fault switch (R1). The desktop method count goes from 47 to 46. |
| The «ВЫБРАННОЕ УСЛОВИЕ» panel, «ВОССТАНОВИТЬ УСЛОВИЕ», its confirmation, and the block reason «ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ НЕ НАСТРОЕНО» | The fault dialog and the switches |
| `#facilityConditionPrivateRecovery` | «Смотритель вручную», always listed |
| `#facilityDeviceInitialState` | The starred chip |
| The binding dialog's «ТЕРМИНАЛ» select, its type select, and its command-action kind | Type tabs, the cross-terminal «ГДЕ», and the command-action dialog |
| The program dialog's separate first-transition fields and instruction paragraph | One uniform row list |
| Device search by ID, and `row.title = id` | Search by name and state |

## Demo data additions (Principle IX, R16)

| File | Change |
|---|---|
| `sessions/demo.json`, `reactor-main` | A second transition «Пуск от резервной сети» from «Аварийная остановка» to «Дежурный режим», with the precondition «Резервная сеть Убежища 76 = В работе». The matrix then shows a cell with two transitions. |
| `sessions/demo.json`, one recovery program | A second transition, on a device the program does not use yet. It needs `recovery: true`, because both demo programs are referenced by a `recoveryProgramId` entry. Its effects must not contradict the first transition's. A precondition on the first transition's device must equal that transition's source state. Its source state must hold whenever the program's command can run, so the command still succeeds. |
| `sessions/demo.json`, stored `recovery[]` | Unchanged |
| `sessions/demo-capability-paths.md` | Every old route and marker uses the new names, including the row text «Загрязнён · ● 1». New rows cover templates, the two-transition cell, each rule type, the command-action dialog, a fault with and without an activating transition, the two-transition program, the terminal-fault group, each link destination, and each filter with its count. |
| `internal/platform/assets_test.go` | Stops requiring a recovery entry for every condition |

Current states do not change, so the list counts stay «Все 10», «Изменённые 5», and «С
неисправностью 1». The file stays session JSON version 1 (SC-009).

## Requirement traceability

| Requirements | Entities |
|---|---|
| FR-001 to FR-003, FR-007, SC-001, SC-002 | Device draft, state chip, technical identifiers |
| FR-004 to FR-006 | Transition matrix, transition panel |
| FR-008 | State deletion |
| FR-009 to FR-012, SC-003 | Device template |
| FR-013 to FR-017, FR-019, FR-020, SC-004 | Fault dialog, stored recovery entries, `privateOverseerAction`, domain rule change |
| FR-016, FR-018 | Recovery program dialog |
| FR-021 | Terminal fault group |
| FR-022 to FR-028, SC-005 | Display rule |
| FR-029 | Command action |
| FR-030 to FR-033, SC-006 | Device list |
| FR-034 to FR-036, SC-007 | Glossary, terminology |
| FR-037 | Dependency link |
| FR-038 to FR-041, SC-008 | Overseer error |
| FR-042, FR-043, SC-009 | Persisted values, demo data additions |
| FR-044 | Demo data additions |
