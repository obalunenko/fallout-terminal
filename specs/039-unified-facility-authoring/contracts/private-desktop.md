# Contract: Private Desktop Bridge Changes

**Producer**: Go (`app.go`, `desktop_service.go`, `app_contract.go`) over the narrow Wails desktop
service. **Consumer**: Overseer (`frontend/overseer/src/desktop-api.js`, `overseer.js`) and the
browser fixture shim (`tests/browser/fixture-server/main.go`). **Schema source of truth**:
`proto/fallout/terminal/private/v1/desktop.proto` (Principle II). Players are unaffected: the public
player contract and `runtime.proto` do not change.

Every method below is unary, private to the Overseer, and validated again at the Go boundary through
`app_contract.go` round trips. Every mutation is serialized with player world actions by the
coordinator (`app.coordinationCommandMu`, `control.Service.commit`).

## Added methods

### `SaveAuthoring(AuthoringSavePayload) → FacilityOperationResult`

The one save for every terminal and facility edit, and for undo and redo (FR-001, FR-001c, FR-007).

```proto
message SaveAuthoringRequest {
  fallout.terminal.persistence.v1.Session session = 1;  // complete authored session
  uint64 expected_authoring_revision = 2;
  string correlation_id = 3;                             // non-blank
}
```

| Case | Result |
|---|---|
| Authoring revision matches, candidate valid, content changed | `ok`, `changed`, new `authoring_revision`, `session_revision`, facility revisions, canonical `session`; every broadcast terminal is refreshed and published |
| Content unchanged after merging live values | `ok`, `!changed`, same revisions |
| Authoring revision differs | `STALE_REVISION`, current canonical `session`, current `authoring_revision` |
| Invalid candidate | `INVALID_CONFIGURATION` or `MISSING_REFERENCE` with `issues`; nothing written |
| Bundled demo, no session, shutdown | `INVALID_CONFIGURATION` (`entity_kind: "session"`) or `RUNTIME_CONTEXT_ENDED` |
| Write failure | `PERSISTENCE_FAILED`; canonical and runtime unchanged |

The request never needs a session or facility revision: live values in the candidate are ignored
and replaced by canonical ones. A pending command with a device action is rebased or rejected
(`data-model.md`). The operation is recorded with `app.recordOperation("authoring.save", …)`, with
revisions only and no authored content.

### `ResetScene(SceneResetPayload) → FacilityOperationResult`

```proto
message ResetSceneRequest {
  uint64 expected_session_revision = 1;
  uint64 expected_facility_revision = 2;
  string correlation_id = 3;
}

message CommandReference {
  string terminal_id = 1;
  string command_id = 2;
}
```

| Case | Result |
|---|---|
| Revisions match | `ok`; `changed` when anything differed; `affected_device_ids`, `affected_condition_ids`, `reset_commands`; one durable write; one facility revision step when the facility changed; authoring revision unchanged |
| Revision differs | `STALE_REVISION`, nothing reset |
| Write failure | `PERSISTENCE_FAILED`, durable rollback, runtime untouched (FR-014) |

On success, the coordinator:

- clears reset command presentation;
- clears a pending request for a reset command;
- invalidates a pending device action;
- reprojects every broadcast terminal.

It never changes the active terminal, the return route, or hacking. It emits one audit record per
outcome with the correlation ID, the prior and resulting revisions, and counts (Principle VIII field
rules).

### `DescribePresentation(PresentationPayload) → PresentationResult`

Read-only. Revisions never change.

```proto
message DescribePresentationRequest {
  uint64 expected_authoring_revision = 1;  // 0 = whatever is current
}

message DescribePresentationResult {
  bool ok = 1;
  FacilityFailureCode failure = 2;
  uint64 session_revision = 3;
  uint64 facility_revision = 4;
  uint64 authoring_revision = 5;
  repeated TerminalPresentation terminals = 6;
}

message TerminalPresentation {
  string terminal_id = 1;
  repeated NodePresentation nodes = 2;
}

message NodePresentation {
  string node_id = 1;
  string player_name = 2;
  bool visible = 3;
  optional bool available = 4;
  repeated PresentationRule name_rules = 5;
  repeated PresentationRule visibility_rules = 6;
  repeated PresentationRule availability_rules = 7;
  repeated BlockPresentation blocks = 8;
}

message BlockPresentation {
  string block_id = 1;
  string player_text = 2;
  repeated PresentationRule text_rules = 3;
}

enum PresentationRuleKind {
  PRESENTATION_RULE_KIND_UNSPECIFIED = 0;
  PRESENTATION_RULE_KIND_AUTHORED_NAME = 1;
  PRESENTATION_RULE_KIND_NAME_AFTER_EXECUTION = 2;
  PRESENTATION_RULE_KIND_STATE_NAME = 3;
  PRESENTATION_RULE_KIND_STATE_VISIBILITY = 4;
  PRESENTATION_RULE_KIND_DIAGNOSTIC_FORCED_VISIBILITY = 5;
  PRESENTATION_RULE_KIND_STATE_AVAILABILITY = 6;
  PRESENTATION_RULE_KIND_DIAGNOSTIC_CAPABILITY_BLOCK = 7;
  PRESENTATION_RULE_KIND_INITIAL_TEXT = 8;
  PRESENTATION_RULE_KIND_TEXT_AFTER_EXECUTION = 9;
  PRESENTATION_RULE_KIND_STATE_TEXT = 10;
  PRESENTATION_RULE_KIND_DIAGNOSTIC_SUBSTITUTION = 11;
}

enum PresentationRuleEffect {
  PRESENTATION_RULE_EFFECT_UNSPECIFIED = 0;
  PRESENTATION_RULE_EFFECT_VISIBLE = 1;
  PRESENTATION_RULE_EFFECT_AVAILABLE = 2;
  PRESENTATION_RULE_EFFECT_UNAVAILABLE = 3;
}

message CommandExecutedCondition {
  string terminal_id = 1;
  string command_id = 2;
}

message DiagnosticActiveCondition {
  string condition_id = 1;
}

message PresentationRule {
  PresentationRuleKind kind = 1;
  uint32 precedence = 2;
  oneof condition {  // unset = always
    fallout.terminal.persistence.v1.FacilityStateEquality device_state = 3;
    CommandExecutedCondition command_executed = 4;
    DiagnosticActiveCondition diagnostic_active = 5;
  }
  oneof value {
    string text = 6;                    // name and text kinds
    PresentationRuleEffect effect = 7;  // visibility and availability kinds
  }
  bool condition_holds = 8;
  bool players_see = 9;
}
```

The adapter rejects a rule whose `value` case does not match its `kind`.

A mismatched expected authoring revision returns `STALE_REVISION` with the current revisions. The
Overseer re-requests once its save queue is idle. Text values are authored content and are never
logged.

## Changed methods (additive fields only)

| Message | Added field | Meaning |
|---|---|---|
| `PreviewFacilityRequest` | `uint64 expected_authoring_revision = 5` | 0 skips the check; otherwise a mismatch returns `STALE_REVISION` (FR-019) |
| `PreviewFacilityResult` | `fallout.terminal.player.v1.LiveTerminal current = 6` | Current projection from the same snapshot as `terminal` |
| `PreviewFacilityResult` | `repeated PreviewDifference differences = 7` | Ordered by tree order, then block order |
| `FacilityOperationResult` | `uint64 authoring_revision = 12` | Set by `SaveAuthoring` and `ResetScene`; zero on other operations |
| `FacilityOperationResult` | `repeated CommandReference reset_commands = 13` | Set by `ResetScene` |
| `SessionOperationResult` | `uint64 authoring_revision = 6` | Create, open, copy demo |
| `ReplaceTerminalGroupsResult` | `uint64 authoring_revision = 6` | After group replacement |
| `PlayerConfigOperationResult` | `uint64 authoring_revision = 8` | After player-config association |

```proto
enum PreviewDifferenceKind {
  PREVIEW_DIFFERENCE_KIND_UNSPECIFIED = 0;
  PREVIEW_DIFFERENCE_KIND_NAME_CHANGED = 1;
  PREVIEW_DIFFERENCE_KIND_TEXT_CHANGED = 2;
  PREVIEW_DIFFERENCE_KIND_APPEARS = 3;
  PREVIEW_DIFFERENCE_KIND_DISAPPEARS = 4;
  PREVIEW_DIFFERENCE_KIND_BECOMES_AVAILABLE = 5;
  PREVIEW_DIFFERENCE_KIND_BECOMES_UNAVAILABLE = 6;
}

message PreviewDifference {
  PreviewDifferenceKind kind = 1;
  string node_id = 2;
  optional string block_id = 3;
  optional string before = 4;
  optional string after = 5;
}
```

Preview remains read-only: no write, no publication, no revision change (FR-020).

## Removed methods and messages (cutover)

| Removed Wails method | Replaced by | Removed message |
|---|---|---|
| `SaveSession` | `SaveAuthoring` | `SaveSessionResult` |
| `SaveFacilityAuthoring` | `SaveAuthoring` | `SaveFacilityAuthoringRequest` |
| `ResetFacility` | `ResetScene` | `ResetFacilityRequest` |
| `ResetTerminalCommandStates` | `ResetScene` | `ResetTerminalCommandStatesRequest` |
| `UpdateLiveTerminal` | `SaveAuthoring` publication | `LiveTerminalUpdateRequest` |

`ResetFacilityDevice`, `ResetCommandState`, `RecoverFacilityCondition`, `PreviewFacility`, and
`InspectFacilityDependencies` remain.

### Migration

The private desktop contract is produced and consumed inside one application build: Wails bindings
and `desktop-api.js` are regenerated and shipped together, and no file, network peer, or other
process reads these messages. Removing the five messages is therefore a breaking schema change
without external consumers, handled as Principle V's documented migration:

1. In one change, delete the messages, their adapters, Wails methods, bindings, JS entries, fixture
   shim entries, and tests.
2. Run `task proto:generate`, then refresh `proto/compatibility-baseline.binpb` from the new schema
   and record the reviewed revision in `proto/schema-revision.txt`.
3. Update the pinned digests in `scripts/wails-v3-contract-check.sh` and the method list and
   signatures in `scripts/wails-bindings-check.sh`.
4. Confirm `task proto:check`, `task proto:breaking` (against the refreshed baseline), and
   `task bindings:check` pass, and that `rg` finds no remaining reference to the removed names
   outside `specs/` history.

Rollback is a revert of the whole change. No data migration exists, because the session JSON is
unchanged.
