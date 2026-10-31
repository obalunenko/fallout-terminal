# Data Model: Facility Play Controls

Portable session JSON version 1 is unchanged (FR-026). This feature adds no session field. Every
entity below is one of three things: a private desktop contract value, runtime-only Go state, or
transient Overseer state. Wire details are in `contracts/private-desktop.md`, and screen copy and
layout are in `contracts/overseer-ui.md`.

Terminology: the spec's "fault" is a `domain.DiagnosticCondition`. Its "current position" is
`currentActive`, and its "starting position" («в начале игры») is `initialActive`. A fault is
"scoped to the device" when `condition.device.deviceId` names that device.

## Persisted values (existing fields only)

| Session JSON field | Written by |
|---|---|
| `facility.revision` | +1 on every accepted play change that changes something |
| `facility.devices[].currentStateId` | card transition, forced change, device reset |
| `facility.conditions[].currentActive` | card transition (its fault effects), fault switch, device reset |
| `terminals[].commandStates` | never by a play change (FR-003) |
| `facility.devices[].initialStateId`, `facility.conditions[].initialActive` | never by a play change (FR-010) |

The session's saved revision advances with each durable commit. The authoring revision from feature
039 does not change: the three new operations join the row "Command execution, world action,
recovery, `ResetFacilityDevice` …" in that table. They are live changes, so «ОТМЕНИТЬ» never
reverses them.

## Overseer play change

One unary private desktop request, checked against the whole-facility revision the Overseer last
saw (FR-004). `ResetFacilityDevice` already exists. The other three are new (R2).

### Requests

| Operation | Proto request (`desktop.proto`) | Fields, in field-number order |
|---|---|---|
| Card transition | `ApplyFacilityDeviceTransitionRequest` | `device_id`, `transition_id`, `expected_facility_revision`, `correlation_id` |
| Forced change | `ForceFacilityDeviceStateRequest` | `device_id`, `state_id`, `expected_facility_revision`, `correlation_id` |
| Fault switch | `SetFacilityConditionActiveRequest` | `condition_id`, `active`, `expected_facility_revision`, `correlation_id` |
| Device reset | `ResetFacilityDeviceRequest` (unchanged) | `device_id`, `expected_facility_revision`, `correlation_id` |

Go types follow the reset layering. App payloads are `FacilityDeviceTransitionPayload`,
`FacilityDeviceForcePayload`, and `FacilityConditionActivePayload`, with JSON keys `deviceId`,
`transitionId`, `stateId`, `conditionId`, `active`, `expectedFacilityRevision`, and `correlationId`.
The control and session request types are `FacilityDeviceTransitionRequest`,
`FacilityDeviceForceRequest`, and `FacilityConditionActiveRequest`. Each operation has its own store
capability interface next to `FacilityResetStore`, and `main.go` adapts it to the session service.

### Common checks (in order, every layer)

1. The App rejects an empty ID or correlation ID, or one with surrounding whitespace, as
   `invalid-configuration` (entity `request`).
2. Control: a canceled context gives `runtime-context-ended`; an empty ID gives
   `invalid-configuration`; no runtime facility gives `missing-reference`. If
   `expected_facility_revision` differs from the runtime facility revision, the result is
   `stale-revision`.
3. Session: a closed service or no active session gives `runtime-context-ended`. The bundled demo
   gives `invalid-configuration` (entity `session`, which suggests a demo copy). No facility gives
   `missing-reference`. A revision mismatch gives `stale-revision`. After these checks come the
   checks of each operation below.

### Rules per operation

| Operation | Moves | Does not check or run | Unchanged (no-op) when | Operation failures |
|---|---|---|---|---|
| Card transition | Device to the transition's destination; each fault in its effects to the effect's value; one commit through `resolveWorldTransitions` (R1) | Faults that block terminal capabilities; command state | Never: a transition's source and destination always differ | `missing-reference` (device, transition, precondition device, effect fault), `invalid-transition` (device not in the source state), `precondition-failed` (first unmet precondition, in authored order; entity is that precondition's device), `conflict` |
| Forced change | Device to `state_id` | Transitions, preconditions, fault effects (FR-008) | `state_id` equals the current state | `missing-reference` (device, or entity `state` when the device has no such state) |
| Fault switch | `currentActive` of `condition_id` to `active` | The fault's recovery list and scope (R3); `initialActive` | `active` equals `currentActive` | `missing-reference` (condition) |
| Device reset | Device to `initialStateId`; each fault scoped to the device to `initialActive` | — | Nothing differs from the starting values | `missing-reference` (device) |

Any operation can also fail with `persistence-failed` or `invalid-configuration` (entity
`candidate`) when the durable commit fails. A failed commit leaves the session file and the runtime
facility unchanged.

### Exact results (`FacilityOperationResult`)

Results reuse `FacilityOperationResult`. The App validators and `validateFacilityMutationResult` in
`desktop-api.js` apply the same rules:

| Operation | `changed` result: `affectedDeviceIds` | `changed` result: `affectedConditionIds` |
|---|---|---|
| Card transition | exactly `[deviceId]` | the sorted, unique fault IDs that the transition's effects name, even when a value was already equal (the same rule as a player command) |
| Forced change | exactly `[deviceId]` | empty |
| Fault switch | empty | exactly `[conditionId]` |
| Device reset (R9 fix) | empty or exactly `[deviceId]` | only faults scoped to `deviceId`; at least one ID across both lists |

Rules shared by all four:

- **Changed**: `ok`, `previousFacilityRevision` equals the expected revision,
  `resultingFacilityRevision` is the previous one +1, `session.facility.revision` equals the
  resulting revision, and `sessionRevision` is above 0. The returned facility equals the expected
  facility computed from the runtime facility by the same rule.
- **Unchanged**: `ok`, the revision is the same, and both ID lists are empty.
- **Failure**: not `ok`, not `changed`, the previous revision equals the resulting revision, and
  both ID lists are empty. On `stale-revision` the result carries the canonical session at the
  current facility revision (R4). When the control check rejected the request before the store
  ran, the App fills the session from the active session snapshot. Other failures carry no
  session.

### Effects of an accepted, changed play change (one `commit` closure)

1. Validate the durable result against the expected facility. On a mismatch the result is
   `persistence-failed` and nothing below happens.
2. Write the audit record (see Audit additions).
3. Install the durable facility as `runtime.Facility`.
4. Withdraw a pending player request whose `PendingFacilityAction` is set, using
   `invalidatePendingFacilityAction`. The player's terminal shows `CommandExecutionPhaseRejected`.
   The withdrawal writes a `command.request_outcome` record with outcome `superseded`, and a
   `withdrawn` feed entry for each device that the request's transitions name (FR-006). Device
   reset gains this record and this entry too.
5. Reproject the terminal runtimes. Players get the new names, texts, and fault effects in the same
   update as after a player command (FR-005).
6. Append the feed entry for the play change, then the withdrawal entries.
7. Emit the master snapshot and the player effects.

Unchanged and rejected play changes write only their audit record: no feed entry, no withdrawal,
no reprojection.

### Device current state

```text
          card transition (source = current, preconditions met, effects applied)
current ─────────────────────────────────────────────────────────────▶ destination
   │ forced change (any other state of the device, no effects)
   ├─────────────────────────────────────────────────────────────────▶ chosen state
   │ device reset / scene reset
   └─────────────────────────────────────────────────────────────────▶ initialStateId
```

### Fault current position

```text
inactive ◀──── fault switch (active=false) / transition effect / reset to initialActive ────┐
   │                                                                                       │
   └──── fault switch (active=true) / transition effect / reset to initialActive ───▶ active
```

`initialActive` changes only through authoring.

## Device change feed (runtime only)

The last five changes of each device, for the current application run (FR-012, R6). The feed is
never saved and never rebuilt from logs (Principle VIII).

### Storage and projection

| Location | Field | Rule |
|---|---|---|
| `domain.ProcessRuntime` | `FacilityChanges map[string][]FacilityChange` | Key is the device ID. Each list is newest first and holds at most `facilityChangeLimit = 5` entries. `cloneProcessRuntime` copies it deeply. |
| `domain.ProcessRuntime` | `FacilityChangeSequence uint64` | The last sequence number assigned. It is run-wide and never reset, so `ReplaceFacility` keeps it. |
| `domain.MasterCoordinationState` | `FacilityChanges []FacilityChange` (`json:"facilityChanges"`) | `masterSnapshot` flattens all devices, newest first by `sequence`. `CloneMasterCoordinationState` copies it deeply. |
| `privatev1.CoordinationState` | `repeated FacilityChange facility_changes = 11` | Carried by the existing `coordination-state` event. No new event name. |

The App's round trip through `CoordinationState` keeps every entry. A round-trip test pins it,
because the App and JS allowlists drop unknown values silently.

### Entry (`domain.FacilityChange`, proto `FacilityChange`)

The wire shape is fixed in `contracts/private-desktop.md`. The domain type mirrors it field for
field, with one non-nil pointer per `oneof` variant.

| Field | Type | Rule |
|---|---|---|
| `sequence` | `uint64` | Unique and increasing across the run. It is the entry's identity. |
| `occurredAtUnixMs` | `int64` | `control.Config.Now().UnixMilli()`. `Now` defaults to `time.Now`. All entries from one commit share one value. |
| `deviceId` | `string` | The feed the entry belongs to. It names an existing device when appended. |
| one variant | `oneof change` | Exactly one is set. The variant implies the author. |

| Variant | Fields | Author | Produced by |
|---|---|---|---|
| `transition` | `transitionId`, `fromStateId`, `toStateId` | Overseer | card transition |
| `forced` | `fromStateId`, `toStateId` | Overseer | forced change |
| `fault` | `conditionId`, `active` (resulting position) | Overseer | fault switch on a fault scoped to `deviceId` |
| `reset` | `fromStateId`, `toStateId` (equal when only faults were restored), `conditionIds` | Overseer | device reset |
| `sceneReset` | `fromStateId`, `toStateId`, `conditionIds` | Overseer | scene reset |
| `playerCommand` | `terminalId`, `commandId`, `transitionId`, `fromStateId`, `toStateId` | player terminal `terminalId` | an approved player command applied |
| `playerRefusal` | `terminalId`, `commandId`, `reason` | player terminal `terminalId` | a player request declined, refused, or withdrawn |

`conditionIds` lists the device-scoped faults the reset restored, sorted. Fault changes caused by a
transition's effects are not listed in the feed. The audit record carries them.

### Producers (R7)

| Producer (file) | Variant | Feeds |
|---|---|---|
| Card transition, forced change, fault switch, device reset (`facility.go`) | matching variant | the target device. A fault switch writes to the fault's device scope; a terminal-scoped fault gets no entry. |
| Scene reset (`scene.go`) | `sceneReset` | each device whose state or device-scoped faults changed |
| Approved command applied (`service.go`) | `playerCommand`, one per moved device | each device in the command's action, in device ID order |
| Declined, failed, blocked, or stale request (`service.go`); 039 authoring-save rejection (`authoring.go`) | `playerRefusal` | each existing device named by the command's authored action (its transitions, or its recovery program's transitions) |
| Withdrawal by a changed play change, device reset, or scene reset | `playerRefusal` with `WITHDRAWN` | each device in the pending request's transitions |

Only commands with an authored facility action produce entries. Hack, entry-view, and
terminal-transition requests never do, so the feed holds no hacking content (FR-016).

### Refusal reasons (`FacilityChangeRefusalReason`)

| Source | Reason |
|---|---|
| The Overseer rejects the request (facility failure `rejected`) | `DECLINED` |
| `precondition-failed` at request or approval | `PRECONDITION_FAILED` |
| `invalid-transition`: the device is not in the source state, or the transition is gone | `TRANSITION_UNAVAILABLE` |
| Capability blocked by an active fault before the request became pending (`facilityCapabilityBlocked`) | `FAULT_BLOCKED` |
| `stale-revision` at approval, or a 039 authoring save that could not rebase the request | `STALE` |
| Superseded by an Overseer play change, device reset, or scene reset | `WITHDRAWN` |
| `missing-reference`, `conflict`, `invalid-configuration`, `persistence-failed`, `runtime-context-ended` | `FAILED` |
| `duplicate` decision on a request that is already resolved | no entry |

### Rules

- **Bound**: appending to a device's list beyond five drops that device's oldest entry.
- **Order**: within one commit, the play change's own entries come first, then the withdrawal
  entries, each in device ID order. Sequence numbers follow that order. The newest-first display
  therefore shows a withdrawal above its cause.
- **Publication** (FR-014): a commit that appends an entry returns an accepted or persisted
  transition and emits a master snapshot that carries the feed. This includes the paths that
  today only cache a player rejection. Snapshots are complete, not deltas, and the App drops only
  snapshots strictly older than the last one, so the newest feed wins.
- **Clearing**: `ReplaceFacility` (session create, open, demo copy, startup restore) empties
  `FacilityChanges`. After an accepted authoring save, entries of devices no longer in the
  facility are dropped, so the keys stay a subset of the facility's device IDs.
- **No-op**: unchanged results and rejected Overseer play changes append nothing.
- **Content**: entries carry IDs, booleans, a reason, and a time. They never carry names, record
  text, command text, passwords, or credentials. The Overseer resolves names from `state.session`
  when it renders the feed.

```text
run start / session opened ──▶ empty ──append (≤5 per device, newest first)──▶ entries
                                  ▲                                               │
                                  └────── ReplaceFacility (another session) ──────┘
```

## Audit additions (R8)

| Item | Change |
|---|---|
| `control.FacilityAuditAction` | new values `transition`, `force`, and `fault`; `reset` stays |
| `control.FacilityAuditFacts` | new `TransitionID` (log `transition_id`), `TargetStateID` (`target_state_id`, force), `TargetActive *bool` (`target_active`, fault) |
| Event names | `facility.overseer_transition`, `facility.force`, `facility.fault`; device reset keeps `facility.reset` |
| `app.go` allowlists | `facilityAuditEvent`, `facilityAuditOutcome`, and `knownFacilityAuditAction` accept the additions |
| Outcomes | `succeeded`, `unchanged`, `stale`, `invalid`, and `failed` (all already allowed). The withdrawal record uses `superseded`, as scene reset does. |

Every play change, accepted or rejected, writes one record with the device IDs, condition IDs, the
action, the outcome, the failure, and both facility revisions (FR-015). A fault switch records
`ConditionIDs = [conditionId]`, plus `DeviceIDs` with the fault's device scope when it has one.
Accepted records add `DeviceTransitions` and `ConditionTransitions` (states and positions before
and after). Rejected records carry no diff, as device reset does today. Records carry stable IDs
only (FR-016), and a logging failure leaves the outcome unchanged.

## Overseer derived and transient state (`frontend/overseer/src/overseer.js`)

Everything here is computed from `state.session` and `state.coordination` on each render, or held in
module variables for the current run. Nothing is written to the session, `localStorage`, or any
other storage.

### Device header (FR-017)

| Field | Source |
|---|---|
| name | `device.name` |
| type | kind option label, or `customKind` for `custom` |
| linked terminals | the number of distinct terminals with a binding that uses the device (`facilityBindingUsesDevice`), as the summary counts today |
| current state, starting state | `facilityStateName(device, currentStateId / initialStateId)` |
| active faults | count of faults scoped to the device with `currentActive` |

### Transition verdict (FR-002, R5)

`facilityTransitionVerdict(facility, deviceId, transition)` is a pure function that applies the
server's order:

1. If the device is not in `transition.sourceStateId`, the verdict is unavailable because of the
   source state. The card never lists such a transition, but the parity table uses this case.
2. Each precondition, in authored order, is met when `devices[p.deviceId].currentStateId ===
   p.stateId`. A missing device counts as unmet, and its ID stands in for the name.
3. The transition is available when every precondition is met. Otherwise the first unmet
   precondition is named as «<device name> = <state name>», which is the one the server would
   report.

Active faults that block terminal capabilities play no part in the verdict (spec Assumptions). The
verdict only drives the display. The server's result decides. SC-002 parity runs as a Go table over
every transition in `sessions/demo.json` (R12).

### Play card (derived)

| Part | Content |
|---|---|
| transitions | Every transition with `sourceStateId === currentStateId`, in authored order, recovery ones included. Each has its name, destination state name, preconditions with met or unmet, and its verdict. When the list is empty, the card says so. |
| forced change | Targets are every state except the current one. Unavailable with a reason when the device has one state. |
| fault switches | Every fault scoped to the device, with its name, current position, and starting position. When there are none, the card says so and shows no switches. |
| reset | The existing «Сбросить к начальному…» confirmation. It succeeds when only faults differ (R9). |
| feed | The first five entries of `state.coordination.facilityChanges` whose `deviceId` matches. Each shows the local time `HH:MM`, what happened, and who did it. A referenced entity that no longer exists shows its ID. With no entries, the feed says nothing has changed in this run yet. |
| block reason | Every play control is unavailable, with a reason, in three cases: no session is open (`НЕТ ОТКРЫТОЙ СЕССИИ`), another Overseer desktop command is still in flight (`coordinationCommandPending`, `ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА`), or the save pipeline is not idle (`ДОЖДИТЕСЬ СОХРАНЕНИЯ`, as scene reset waits today). A player request waiting for approval does not block the card: a play change withdraws it (FR-006). |

The card exists only while a device is selected. If the selected device disappears after an edit,
an undo, or a live change, the selection clears and the whole-session view shows.

### Play change submission

```text
idle ──press transition / switch fault / confirm dialog──▶ submitting (one at a time)
  ▲                                                          │
  │   ok, changed ──▶ apply result.session, re-render ───────┤
  │   ok, unchanged ─▶ re-render ────────────────────────────┤
  │   stale-revision ▶ apply result.session, re-render,      │
  │                    «СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ.        │
  │                     ПОВТОРИТЕ ЗАПРОС» on the card ───────┤
  └── other failure ─▶ show the reason on the card ◀─────────┘
```

A request sends `facilityRevision()` from the card as rendered. A transition and a fault switch run
on one press (FR-001). On stale, `reconcileFacilityOperation` no longer discards `result.session`
(R4).

### Forced-change dialog

| Field | Meaning |
|---|---|
| `deviceId`, `fromStateId` | Captured when the dialog opens |
| `targetStateId` | Chosen from the targets. Confirming is disabled until one is chosen. |
| `facilityRevision` | Captured when the dialog opens and sent on confirm, so a scene that changed while the dialog was open is rejected as stale |

```text
closed ──«Принудительно в другое состояние…»──▶ choosing ──pick──▶ ready ──confirm──▶ submitting ──▶ closed
   ▲                                               │                 │                    │ stale: the dialog closes,
   └──────────────────── cancel / Escape ──────────┴─────────────────┘                    │ the card shows the message
                         (nothing sent, nothing recorded)                                 ▼
```

The confirmation names the device, its current state, and the target state (FR-008).

### Device list row (FR-025)

Secondary text: `<current state name>`, or `<current state name> · ● <n>` when `n` (active faults
scoped to the device) is above 0. The accessible text spells out the count. The type moves to the
header. Search still matches name, ID, and current state name. The row re-renders on every session
update.

### Detail tabs (FR-024)

| Tab ID | Label | Items (selected device) | Items (no device selected) |
|---|---|---|---|
| `actions` | «Что делают игроки N» | command bindings that use the device (`Array.isArray(binding.requests)`) | all command bindings |
| `information` | «Что видят игроки N» | other bindings that use the device | all other bindings |
| `conditions` | «Неисправности N» | faults scoped to the device | all faults |
| `programs` | «Программы N» | recovery programs with a transition of the device | all programs |

`facilityDetailTab` holds the open tab for the run (default `actions`). Only the open tab's items
are rendered. A tab with no items stays visible with count 0 and its add control. Selecting a fault
in `conditions` still opens «ВЫБРАННОЕ УСЛОВИЕ» with `RecoverFacilityCondition` (R3).

### Device actions menu

`facilityDeviceMenuOpen` is transient. The menu holds one item, «Удалить устройство». It closes on
Escape, an outside click, a selection change, or after its item runs. The other actions fall into
two groups: inspection («Предпросмотр», «Зависимости») and change («Изменить состояния и
переходы»).

### Scenario guide (FR-019 to FR-021, R10)

| Variable | Values | Rule |
|---|---|---|
| `facilityGuideChoice` | `null`, `'expanded'`, or `'collapsed'` | Set only by a toggle the author makes. The guide's own `open` changes made by JS do not set it. |
| `facilityGuideDeviceCount` | the last device count seen, or `null` at start | When it goes from 0 to 1 or more, `facilityGuideChoice` resets to `null` |
| `facilityStoryStep` (existing) | 0 to 4 | The current step, kept for the run |

The guide is expanded when `facilityGuideChoice === 'expanded'`. With no choice made, it is
expanded when the session has no devices. When collapsed, the summary reads «КАК СОБРАТЬ СЦЕНАРИЙ ·
ШАГ N ИЗ 5: <label>», where the label is Объект, Действие, Результат, Осложнения, or Проверка,
followed by «ПОКАЗАТЬ». When expanded, it ends with «СКРЫТЬ». `index.html` drops the hard-coded
`open`, and `.facility-story-steps` drops `position: sticky`.

```text
               first device saved (count 0 → ≥1): choice := null
                ┌───────────────────────────────────────────┐
                ▼                                           │
[choice null] ── devices = 0 ─▶ expanded      devices ≥ 1 ─▶ collapsed
      │ author toggles                                      │ author toggles
      ▼                                                     ▼
[choice set] ── holds across tabs and selections until the application closes,
                unless the device count next goes from 0 to 1 or more
```

Because the reset keys on the device count, opening a session with devices after an empty one also
collapses the guide. That matches FR-019 for the first saved device and keeps the rule to a single
check.

### Removed state (complete cutover, Principle VII)

- The `#facilitySelectedDeviceName` field, `#btnSaveFacilityDeviceChanges`, and
  `#btnCancelFacilityDeviceChanges` go away, together with the 039 unapplied-input key
  `device:<deviceId>`. Unapplied-input keys are now `node:<terminalId>:<nodeId>` and
  `settings:<terminalId>` only.
- `#btnResetFacilityDevice` moves to the play card as «Сбросить к начальному…».
  `#btnDeleteFacilityDevice` moves into the «⋯» menu.
- The single combined bindings list is split into the `actions` and `information` tabs.

## Demo data additions (Principle IX, R12)

| File | Change |
|---|---|
| `sessions/demo.json`, `reactor-main` | A new state `{ "id": "critical-overheat", "name": "Критический перегрев" }`, which no transition reaches |
| `sessions/demo.json`, `t_demo1` block `b_reactor_status` | A new `facilityTextVariants` entry: when `reactor-main` = `critical-overheat`, the text is «РЕАКТОР: КРИТИЧЕСКИЙ ПЕРЕГРЕВ» |
| `sessions/demo-capability-paths.md` | Six paths. (1) Card transition: the door's «Открыть гермодверь» with the grid «В работе». (2) Blocked transition: shut down the grid, then the door. (3) Forced change: the reactor from «Дежурный режим» to «Критический перегрев». (4) Fault switch: «Контроллер гермодвери не отвечает». (5) A declined «Открыть выход и подать сигнал» in the door's feed. (6) The ventilation row «Загрязнён · ● 1». |

The file stays session JSON version 1, and every other demo value is unchanged (SC-008).

## Requirement traceability

| Requirements | Entities |
|---|---|
| FR-001 to FR-003, SC-001, SC-002 | Card transition rules, transition verdict, play card |
| FR-004, SC-006 | Common checks, failure results with the canonical session on stale, play change submission |
| FR-005, SC-004 | Effects of an accepted play change (reprojection) |
| FR-006 | Withdrawal step, `withdrawn` refusal entries, `superseded` record |
| FR-007, FR-008 | Forced-change rules and dialog |
| FR-009 to FR-011 | Fault switch rules, device reset result fix |
| FR-012 to FR-014, FR-016, SC-005 | Device change feed |
| FR-015 | Audit additions |
| FR-017, FR-018 | Device header, play card order (`contracts/overseer-ui.md`) |
| FR-019 to FR-021 | Scenario guide |
| FR-022 to FR-025, SC-007 | Device actions menu, detail tabs, device list row, removed state |
| FR-026, FR-027, SC-008 | Persisted values, demo data additions |
