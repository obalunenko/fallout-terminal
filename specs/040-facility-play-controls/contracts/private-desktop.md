# Contract: Private Desktop Bridge — Facility Play Controls

**Producer**: Go (`app.go`, `desktop_service.go`, `app_contract.go`, `main.go`) over the narrow Wails
desktop service, backed by `internal/control` and `internal/session`. **Consumers**: the Overseer
(`frontend/overseer/src/desktop-api.js`, `overseer.js`) and the browser fixture shims
(`tests/browser/fixture-server/main.go`, `tests/browser/fixtures/desktop-bindings.js`). **Schema
source of truth**: `proto/fallout/terminal/private/v1/desktop.proto` and `coordination.proto`
(Principle II). **Base**: this contract extends the bridge as left by `039-unified-facility-authoring`
(see `specs/039-unified-facility-authoring/contracts/private-desktop.md`).

Players are unaffected. The public player contract, `runtime.proto`, and session JSON version 1 do
not change. A player sees an Overseer play change only through the existing terminal reprojection.

## Shared rules for Overseer play changes

There are four play operations. Device reset (JS `resetFacilityDevice`, Wails `ResetFacilityDevice`)
already exists. The three new operations sit next to `resetFacilityDevice` and share its revision
check and journal:

| Wails method | JS method | Request message | Payload type (`app.go`) |
|---|---|---|---|
| `ResetFacilityDevice` (existing) | `resetFacilityDevice` | `ResetFacilityDeviceRequest` | `FacilityDeviceResetPayload` |
| `ApplyFacilityDeviceTransition` | `applyFacilityDeviceTransition` | `ApplyFacilityDeviceTransitionRequest` | `FacilityDeviceTransitionPayload` |
| `ForceFacilityDeviceState` | `forceFacilityDeviceState` | `ForceFacilityDeviceStateRequest` | `FacilityDeviceForcePayload` |
| `SetFacilityConditionActive` | `setFacilityConditionActive` | `SetFacilityConditionActiveRequest` | `FacilityConditionActivePayload` |

Every play operation follows these rules:

1. **Unary and serialized.** The App holds `app.coordinationCommandMu`. The coordinator runs the whole
   operation inside one `control.Service.commit` closure, so it is ordered against player world
   actions. Two concurrent changes never both apply (SC-006).
2. **Facility-wide revision check.** The request carries `expected_facility_revision`, the
   `session.facility.revision` the Overseer last saw. Any change to any device, any fault, or any
   authoring edit moves that revision (FR-004).
3. **One durable write.** A changed result commits one candidate session through
   `commitFacilityCandidateLocked`. The facility revision goes up by exactly 1, and the session
   revision goes up. The authoring revision does not change, and `authoring_revision` in the result
   is `0`. The operation writes only `currentStateId` on devices and `currentActive` on
   conditions. It never writes `commandStates`, so no command is marked executed (FR-003, FR-026).
4. **Exact result.** Every operation returns `FacilityOperationResult` (no new result message).
5. **On a changed result**, in the same commit, the coordinator:
   - replaces `runtime.Facility` with the committed facility;
   - withdraws a pending player request that carries a device action (`invalidatePendingFacilityAction`).
     The player's terminal shows it rejected, and the coordinator writes the withdrawal record and
     the feed entries described below (FR-006);
   - reprojects every broadcast terminal (FR-005);
   - writes the operation's audit record;
   - appends the feed entry.

   The App then publishes `session-state`, followed by `coordination-state`. That carries the
   canonical session and the new feed together.
6. **On an unchanged result** (`ok`, `!changed`), nothing is written or published, no request is
   withdrawn, and no feed entry is added. The audit record says `unchanged`.
7. **On a failure**, nothing changes. Only `STALE_REVISION` returns the canonical session (see the
   table below). The audit record has the failure outcome. No feed entry is added.

### Common result cases

| Case | `ok` | `changed` | `failure` | `session` | Revisions |
|---|---|---|---|---|---|
| Change applied | true | true | — | canonical session after the write | `previous = expected`, `resulting = previous + 1 = session.facility.revision`, `session_revision > 0` |
| Already in the requested position | true | false | — | canonical session | `previous = resulting = expected`; both affected lists empty |
| `expected_facility_revision` differs | false | false | `STALE_REVISION` | **current canonical session** | `previous = resulting = current facility revision = session.facility.revision`; `session_revision` is current |
| Blank or padded ID or correlation ID | false | false | `INVALID_CONFIGURATION` (`entity_kind: "request"`) | none | `previous = resulting = expected` |
| Bundled demo open | false | false | `INVALID_CONFIGURATION` (`entity_kind: "session"`) | none | unchanged |
| No facility in the session | false | false | `MISSING_REFERENCE` (`entity_kind: "facility"`) | none | unchanged |
| No session, shutdown, or no coordinator | false | false | `RUNTIME_CONTEXT_ENDED` | none | unchanged |
| Write failure, or a result the App cannot verify | false | false | `PERSISTENCE_FAILED` | none | canonical and runtime state unchanged |

Change for this feature: a `STALE_REVISION` result now carries `session` for all four play
operations, device reset included. The Overseer applies that session and re-renders, as it already
does after a stale `ResetScene` (FR-004, R4). Every other failure still carries no `session`.

## Added methods

### `ApplyFacilityDeviceTransition(FacilityDeviceTransitionPayload) → FacilityOperationResult`

The Overseer runs one authored transition from the play card (FR-001 to FR-003).

```proto
message ApplyFacilityDeviceTransitionRequest {
  string device_id = 1;
  string transition_id = 2;
  uint64 expected_facility_revision = 3;
  string correlation_id = 4;
}
```

```go
type FacilityDeviceTransitionPayload struct {
	DeviceID                 string `json:"deviceId"`
	TransitionID             string `json:"transitionId"`
	ExpectedFacilityRevision uint64 `json:"expectedFacilityRevision"`
	CorrelationID            string `json:"correlationId"`
}
```

The session layer resolves the request with `resolveWorldTransitions` for the single
`{DeviceID, TransitionID}`. It applies the same source-state, precondition, and fault-effect rules as
a player command, all or nothing. It does not check terminal capability blocks, because the request
comes from the Overseer and not from a terminal (spec Assumptions).

| Case | Result |
|---|---|
| Source state matches, every precondition holds, effects consistent | `ok`, `changed`; `affected_device_ids = [device_id]`; `affected_condition_ids` = sorted, unique faults that the transition's `condition_effects` name, even when a value was already equal, the same rule as a player command (may be empty) |
| Device not found | `MISSING_REFERENCE`, issue `entity_kind: "device"`, `entity_id: device_id` |
| Transition not found on the device | `MISSING_REFERENCE`, issue `entity_kind: "transition"`, `entity_id: transition_id` |
| Device is not in the transition's source state | `INVALID_TRANSITION`, issue `entity_kind: "device"` |
| First unmet precondition | `PRECONDITION_FAILED`, issue `entity_kind: "device"`, `entity_id` = the precondition's device |
| Precondition or effect names a missing device or condition | `MISSING_REFERENCE` with that entity |
| Conflicting effects on one condition | `CONFLICT`, issue `entity_kind: "condition"` |

A successful transition always changes the device, because authored transitions never loop to
their own source state. The App and JS validators therefore reject `ok && !changed` for this
method.

### `ForceFacilityDeviceState(FacilityDeviceForcePayload) → FacilityOperationResult`

The Overseer sets a device's current state directly, without checking transitions or preconditions
(FR-007, FR-008).

```proto
message ForceFacilityDeviceStateRequest {
  string device_id = 1;
  string state_id = 2;
  uint64 expected_facility_revision = 3;
  string correlation_id = 4;
}
```

```go
type FacilityDeviceForcePayload struct {
	DeviceID                 string `json:"deviceId"`
	StateID                  string `json:"stateId"`
	ExpectedFacilityRevision uint64 `json:"expectedFacilityRevision"`
	CorrelationID            string `json:"correlationId"`
}
```

| Case | Result |
|---|---|
| State exists and differs from the current state | `ok`, `changed`; `affected_device_ids = [device_id]`; `affected_condition_ids = []`. No transition's fault effects run, and no authored transition is created or changed |
| State equals the current state | `ok`, `!changed` |
| Device not found | `MISSING_REFERENCE`, issue `entity_kind: "device"` |
| State not found on the device | `MISSING_REFERENCE`, issue `entity_kind: "state"`, `entity_id: state_id` |

### `SetFacilityConditionActive(FacilityConditionActivePayload) → FacilityOperationResult`

The Overseer turns one fault on or off (FR-009, FR-010, R3).

```proto
message SetFacilityConditionActiveRequest {
  string condition_id = 1;
  bool active = 2;
  uint64 expected_facility_revision = 3;
  string correlation_id = 4;
}
```

```go
type FacilityConditionActivePayload struct {
	ConditionID              string `json:"conditionId"`
	Active                   bool   `json:"active"`
	ExpectedFacilityRevision uint64 `json:"expectedFacilityRevision"`
	CorrelationID            string `json:"correlationId"`
}
```

| Case | Result |
|---|---|
| `active` differs from `currentActive` | `ok`, `changed`; `affected_device_ids = []`; `affected_condition_ids = [condition_id]`. Only `currentActive` changes. `initialActive` never changes |
| `active` equals `currentActive` | `ok`, `!changed` |
| Condition not found | `MISSING_REFERENCE`, issue `entity_kind: "condition"` |

The server accepts any condition, device-scoped or terminal-scoped. It ignores the condition's
`recovery` list, so «Восстановление оператором» does not limit the switch. Showing only
device-scoped faults is a presentation choice of the play card. `RecoverFacilityCondition` and its
`conditionAllowsRecovery` rule do not change.

## Changed method: `ResetFacilityDevice`

The request and payload do not change. Behavior changes:

| Change | Rule |
|---|---|
| Fault-only reset (FR-011, R9) | A changed result is valid when `affected_device_ids` is `[]` or exactly `[device_id]`, `affected_condition_ids` holds only faults scoped to `device_id`, and at least one ID is affected. This applies to `validFacilityResetResultForApp` in `app.go` and to the `device-reset` branch of `validateFacilityMutationResult` in `desktop-api.js`. Today both reject the empty device list, so the App reports `PERSISTENCE_FAILED` even though the session layer saved the reset |
| Stale rejection | Returns the current canonical `session`, as in the common table |
| Withdrawal | Writes the withdrawal record and feed entries below. Today `invalidatePendingFacilityAction` logs nothing |
| Feed | A changed reset appends one `reset` entry for `device_id` |

## Coordination state: the device feed (FR-012 to FR-014, R6, R7)

The feed reaches the Overseer through the existing `coordination-state` event. No event is added,
so the seven pinned event names stay the same.

```proto
// coordination.proto (additive)

enum FacilityChangeRefusalReason {
  FACILITY_CHANGE_REFUSAL_REASON_UNSPECIFIED = 0;
  FACILITY_CHANGE_REFUSAL_REASON_DECLINED = 1;               // the Overseer rejected the request
  FACILITY_CHANGE_REFUSAL_REASON_PRECONDITION_FAILED = 2;    // an unmet precondition
  FACILITY_CHANGE_REFUSAL_REASON_TRANSITION_UNAVAILABLE = 3; // the device is no longer in the source state
  FACILITY_CHANGE_REFUSAL_REASON_FAULT_BLOCKED = 4;          // an active fault blocked the capability before the request became pending
  FACILITY_CHANGE_REFUSAL_REASON_STALE = 5;                  // the facility changed meanwhile
  FACILITY_CHANGE_REFUSAL_REASON_WITHDRAWN = 6;              // superseded by an Overseer play change or a scene reset
  FACILITY_CHANGE_REFUSAL_REASON_FAILED = 7;                 // any other facility failure
}

message FacilityChangeTransition {   // Overseer ran an authored transition from the card
  string transition_id = 1;
  string from_state_id = 2;
  string to_state_id = 3;
}

message FacilityChangeForced {       // Overseer forced a state
  string from_state_id = 1;
  string to_state_id = 2;
}

message FacilityChangeFault {        // Overseer switched a fault scoped to this device
  string condition_id = 1;
  bool active = 2;                   // the resulting position
}

message FacilityChangeReset {        // Overseer reset this device
  string from_state_id = 1;
  string to_state_id = 2;            // equals from_state_id when only faults were restored
  repeated string condition_ids = 3; // device-scoped faults restored, sorted
}

message FacilityChangeSceneReset {   // a scene reset changed this device
  string from_state_id = 1;
  string to_state_id = 2;
  repeated string condition_ids = 3;
}

message FacilityChangePlayerCommand { // an approved player command moved this device
  string terminal_id = 1;
  string command_id = 2;
  string transition_id = 3;
  string from_state_id = 4;
  string to_state_id = 5;
}

message FacilityChangePlayerRefusal { // a player request affecting this device was not applied
  string terminal_id = 1;
  string command_id = 2;
  FacilityChangeRefusalReason reason = 3;
}

message FacilityChange {
  uint64 sequence = 1;               // run-wide, strictly increasing, never reused in a run
  string device_id = 2;
  int64 occurred_at_unix_ms = 3;     // from control.Config.Now (defaults to time.Now)
  oneof change {
    FacilityChangeTransition transition = 4;
    FacilityChangeForced forced = 5;
    FacilityChangeFault fault = 6;
    FacilityChangeReset reset = 7;
    FacilityChangeSceneReset scene_reset = 8;
    FacilityChangePlayerCommand player_command = 9;
    FacilityChangePlayerRefusal player_refusal = 10;
  }
}

message CoordinationState {
  // fields 1–10 unchanged
  repeated FacilityChange facility_changes = 11; // newest first; at most 5 entries per device_id
}
```

### Rules

- **Bound.** `ProcessRuntime.FacilityChanges` keeps the five newest entries per device. The
  projection `MasterCoordinationState.FacilityChanges` lists them by `sequence`, newest first.
- **Lifetime.** The feed is runtime-only. It is never written to the session file and never rebuilt
  from retained logs (Principle VIII). `control.Service.ReplaceFacility` clears it. That runs on
  session open, new session, copy demo, and startup restore, so the feed starts empty after a
  restart or when another session opens. The sequence counter is not reset.
- **IDs only.** An entry carries authored IDs, revisions, and enum values only. It never carries
  names, record text, passwords, credentials, or hacking content (FR-016). The Overseer resolves
  names from the session and shows the raw ID for anything no longer in the session.
- **Author.** The `transition`, `forced`, `fault`, `reset`, and `scene_reset` variants are by the
  Overseer. The `player_command` and `player_refusal` variants are by the player terminal
  `terminal_id`.
- **Same closure as the audit record.** Each entry is appended in the `commit` closure that writes
  the matching audit record, so neither appears without the other (SC-005).

### Producers

| Producer | Variant | Devices that get an entry |
|---|---|---|
| `ApplyFacilityDeviceTransition`, changed | `transition` | `device_id` |
| `ForceFacilityDeviceState`, changed | `forced` | `device_id` |
| `SetFacilityConditionActive`, changed | `fault` | the condition's scope device. A terminal-scoped fault gets no entry |
| `ResetFacilityDevice`, changed | `reset` | `device_id` |
| `ResetScene`, changed (`scene.go`) | `scene_reset` | each device whose state or device-scoped faults the reset changed |
| Approved player command applied (`service.go`) | `player_command` | each device that a transition of the command's action moved |
| Player request declined, failed, blocked, or found stale | `player_refusal` with `DECLINED`, `PRECONDITION_FAILED`, `TRANSITION_UNAVAILABLE`, `FAULT_BLOCKED`, `STALE`, or `FAILED` | each device named by the command's authored action |
| Pending request withdrawn by a changed play change or scene reset | `player_refusal` with `WITHDRAWN` | each device the pending request's action targets |

Unchanged results, rejected Overseer play changes, and private condition recovery add no entry.

### Event JSON (`coordination-state` payload)

The App routes the state through the proto round trip (`routeCoordinationEvent`). The Overseer
receives the domain JSON form, where each `oneof` variant is an optional object named after the
proto field:

```json
{
  "revision": 51,
  "facilityRevision": 9,
  "facilityChanges": [
    {
      "sequence": 14,
      "deviceId": "vault-door-main",
      "occurredAtUnixMs": 1793390820000,
      "playerRefusal": { "terminalId": "t_demo", "commandId": "open-exit", "reason": "declined" }
    },
    {
      "sequence": 12,
      "deviceId": "vault-door-main",
      "occurredAtUnixMs": 1793390700000,
      "transition": { "transitionId": "open", "fromStateId": "sealed", "toStateId": "open" }
    }
  ]
}
```

Variant keys: `transition`, `forced`, `fault`, `reset`, `sceneReset`, `playerCommand`,
`playerRefusal`. Reason values are `declined`, `precondition-failed`, `transition-unavailable`,
`fault-blocked`, `stale`, `withdrawn`, `failed`, and `unspecified`. `facilityChanges` is omitted when the feed is empty.

`desktop-api.js` normalizes every entry to one frozen object. This is the shape `overseer.js` codes
against:

```js
{
  sequence, deviceId, occurredAtUnixMs,
  kind,          // 'transition' | 'forced' | 'fault' | 'reset' | 'scene-reset' | 'player-command' | 'player-refusal'
  author,        // 'overseer' | 'terminal'
  transitionId, fromStateId, toStateId,  // '' when the variant has none
  conditionId, active, conditionIds,     // fault: conditionId, active; reset kinds: conditionIds
  terminalId, commandId,                 // player kinds
  reason,        // player-refusal: one of the reason values above; otherwise ''
}
```

The normalizer drops an entry that has no variant or more than one, a blank `deviceId`, or a
non-safe-integer `sequence` or `occurredAtUnixMs`. It maps an unknown reason to `unspecified`.

## Retained runtime log (FR-015, FR-016, R8)

| Item | Values |
|---|---|
| `controlservice.FacilityAuditAction` (new) | `transition`, `force`, `fault` (next to `command`, `reset`, `recover`) |
| Audit event names (new) | `facility.overseer_transition`, `facility.force`, `facility.fault` (next to `facility.reset`) |
| Outcomes | `succeeded`, `unchanged`, `stale`, `failed`, `invalid` |
| `FacilityAuditFacts` (new fields) | `TransitionID` → log field `transition_id`; `TargetStateID` → `target_state_id` (force); `TargetActive *bool` → `target_active` (fault) |

Each play change, whether accepted or rejected, writes one record with these fields:

- `event`, `outcome`, `correlation_id`, `facility_action`;
- `previous_facility_revision`, `resulting_facility_revision`;
- `device_ids`, `condition_ids`;
- `device_transitions` and `condition_transitions` (the before and after values) when the change
  was accepted;
- the requested target (`transition_id`, `target_state_id`, or `target_active`);
- `facility_failure` on a failure.

`facilityAuditEvent`, `facilityAuditOutcome`, and `knownFacilityAuditAction` in `app.go` must list
every new value. Otherwise the log silently drops it or records it as `invalid`. The validators for
the new methods accept every known failure code except `UNSPECIFIED`. Unlike
`knownFacilityRecoveryFailureForApp`, they do not exclude `REJECTED`.

**Withdrawal record.** Device reset and the three new operations write
`pendingRequestOutcomeAudit(pending, "superseded", previous, resulting)` when they withdraw a pending
request. The event is `command.request_outcome` with outcome `superseded`, the same record a scene
reset writes through `sceneResetPendingOutcome`.

## Go layering (for tests)

| Layer | Additions |
|---|---|
| `internal/session` | `Service.ApplyFacilityDeviceTransition`, `Service.ForceFacilityDeviceState`, `Service.SetFacilityConditionActive`, with request types `FacilityDeviceTransitionRequest`, `FacilityDeviceForceRequest`, and `FacilityConditionActiveRequest`. They take the same locks as `resetFacility` (`commandMu`, `documentMu`, `mu`) and refuse the bundled demo |
| `internal/control` | The same three `Service` methods with matching request types, each returning `(*domain.MasterCoordinationState, *domain.FacilityOperationResult, error)`. The store interface `FacilityPlayStore` declares the three session methods, and `main.go` adapts `sessionCommandStateStore` to it. Also `Config.Now func() time.Time` and `feed.go` |
| `app.go` | `App.ApplyFacilityDeviceTransition`, `App.ForceFacilityDeviceState`, `App.SetFacilityConditionActive`, each returning `domain.FacilityOperationResult`; a coordinator seam interface per method; result validators; the R9 reset fix |
| `app_contract.go` | Converters `facilityDeviceTransitionRequestToPrivate` / `…FromPrivate`, `facilityDeviceForceRequestToPrivate` / `…FromPrivate`, and `facilityConditionActiveRequestToPrivate` / `…FromPrivate`; `FacilityChange` conversion in `coordinationStateToPrivate` / `coordinationStateFromPrivate` |
| `desktop_service.go` | Three one-line forwarding methods |

App boundary checks follow `App.ResetFacilityDevice`. Every ID and the correlation ID must be
non-blank and unpadded after the proto round trip. Otherwise the App returns
`INVALID_CONFIGURATION` with `entity_kind: "request"` without calling the coordinator.

## Bridge allowlists to update

| Place | Change |
|---|---|
| `app_contract_test.go`, `app_test.go` | Desktop method count 44 → 47; add the three names to the pinned method list |
| `scripts/wails-bindings-check.sh` | Method list, `test … = 47`, the final message, and `assert_method_signature ApplyFacilityDeviceTransition '$models.FacilityDeviceTransitionPayload' '$CancellablePromise<domain$0.FacilityOperationResult>'`, plus the same for `ForceFacilityDeviceState` (`$models.FacilityDeviceForcePayload`) and `SetFacilityConditionActive` (`$models.FacilityConditionActivePayload`) |
| `frontend/overseer/bindings/…/desktopservice.js` | Regenerated |
| `desktop-api.js` | `APP_METHODS.applyFacilityDeviceTransition`, `.forceFacilityDeviceState`, and `.setFacilityConditionActive`; public methods through `facilityMutationCommand` with operations `device-transition`, `device-force`, and `condition-active`; normalizers `normalizeFacilityDeviceTransitionPayload`, `normalizeFacilityDeviceForcePayload`, and `normalizeFacilityConditionActivePayload` (`active: value.active === true`); new set `FACILITY_CHANGE_REFUSAL_REASONS`; feed normalization on the `coordination-state` callback |
| `validateFacilityMutationResult` | `device-transition`: changed is required, and `affectedDeviceIds = [deviceId]`. `device-force`: `affectedDeviceIds = [deviceId]`, `affectedConditionIds = []`. `condition-active`: `affectedDeviceIds = []`, `affectedConditionIds = [conditionId]`. `device-reset`: per R9. A failure that carries `session` is accepted only for `stale-revision` and only when `session.facility.revision === resultingFacilityRevision`. Anything else becomes `conflict` |
| `tests/browser/fixture-server/main.go` | Shim exports `ApplyFacilityDeviceTransition`, `ForceFacilityDeviceState`, and `SetFacilityConditionActive`, posting to `/__fixture/facility-authoring/apply-transition`, `/force-state`, and `/set-condition-active`. The fixture enforces source state and preconditions for `apply-transition`, and `coordination-state` carries `facilityChanges` |
| `tests/browser/fixtures/desktop-bindings.js` | The same three exports through `facilityOperationFixture` |
| `tests/browser/desktop-api.spec.mjs` | Normalizer, validator, and feed-normalization cases for each new branch |

## Session JSON version 1

No field is added, renamed, or removed (FR-026, SC-008). A play change writes only the existing
`facility.devices[].currentStateId`, `facility.conditions[].currentActive`, and `facility.revision`.
Files saved before this feature open unchanged.

## Compatibility and rollout

All schema changes are additive: three request messages, the `FacilityChange*` messages, the
`FacilityChangeRefusalReason` enum (zero value `UNSPECIFIED`), and `CoordinationState` field 11.
`task proto:breaking` passes against the existing `proto/compatibility-baseline.binpb`, so the
baseline is not refreshed. `task proto:generate` updates `internal/gen/…` and
`proto/schema-revision.txt`. `task bindings:check` must pass with 47 methods and seven events. No
data migration exists. Rollback reverts the whole change.
