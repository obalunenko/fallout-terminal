# Contract: Private Overseer Facility Operations

## Boundary Rules

Facility authoring, dependency inspection, preview, reset, and private recovery are trusted Overseer capabilities exposed only through the narrow desktop service. Every structured request and result is protobuf-defined and explicitly adapted at App and desktop-service boundaries. No public player route can invoke these operations or receive the authored facility graph.

The existing `session-state` event remains the canonical full-session update after durable change. The existing `coordination-state` event remains the pending-approval and runtime-authority update. No new generic event bus or facility-specific source of truth is introduced.

## Structured Result

```proto
enum FacilityFailureCode {
  FACILITY_FAILURE_CODE_UNSPECIFIED = 0;
  FACILITY_FAILURE_CODE_REJECTED = 1;
  FACILITY_FAILURE_CODE_MISSING_REFERENCE = 2;
  FACILITY_FAILURE_CODE_INVALID_TRANSITION = 3;
  FACILITY_FAILURE_CODE_PRECONDITION_FAILED = 4;
  FACILITY_FAILURE_CODE_STALE_REVISION = 5;
  FACILITY_FAILURE_CODE_CONFLICT = 6;
  FACILITY_FAILURE_CODE_DUPLICATE = 7;
  FACILITY_FAILURE_CODE_INVALID_CONFIGURATION = 8;
  FACILITY_FAILURE_CODE_PERSISTENCE_FAILED = 9;
  FACILITY_FAILURE_CODE_RUNTIME_CONTEXT_ENDED = 10;
}

message FacilityIssue {
  FacilityFailureCode code = 1;
  string entity_kind = 2;
  optional string entity_id = 3;
  optional string reference_kind = 4;
  optional string reference_id = 5;
}

message FacilityOperationResult {
  bool ok = 1;
  bool changed = 2;
  string correlation_id = 3;
  FacilityFailureCode failure = 4;
  repeated FacilityIssue issues = 5;
  uint64 session_revision = 6;
  uint64 previous_facility_revision = 7;
  uint64 resulting_facility_revision = 8;
  repeated string affected_device_ids = 9;
  repeated string affected_condition_ids = 10;
  fallout.terminal.persistence.v1.Session session = 11;
}
```

The failure enum and stable IDs drive UI state and tests. A localized private explanation may accompany the application result, but callers never branch by matching its text. Raw storage or dependency errors never cross the boundary.

## Pending Approval Extension

The existing PendingCommandExecution adds an optional redacted facility summary:

```proto
message PendingFacilityActionSummary {
  uint64 expected_facility_revision = 1;
  repeated string device_ids = 2;
  repeated string condition_ids = 3;
  optional string recovery_program_id = 4;
}

message PendingCommandExecution {
  string request_id = 1;
  string broadcast_id = 2;
  string terminal_id = 3;
  string command_id = 4;
  string command_name = 5;
  string confirmation_text = 6;
  string command_mode = 7;
  optional PendingFacilityActionSummary facility_action = 8;
}
```

The summary displays impact without exposing transition prose, EntryContent, damaged records, or secret values. The existing ResolveCommandExecution request remains unchanged. Its result adds optional FacilityOperationResult so an approval that fails after revalidation has a typed private outcome.

## Facility Authoring Operations

### InspectFacilityDependencies

Input identifies one facility entity by typed kind and stable ID plus the expected session and facility revisions. The result returns a detached dependency report with typed references from devices, states, transitions, conditions, recovery programs, commands, menu names, EntryContent blocks, visibility, availability, and diagnostic effects.

Inspection is read-only: it emits no session-state event, coordination-state event, revision, player update, or retained world-transition record.

### PreviewFacility

Input contains the expected facility revision, one terminal ID, and exactly one detached preview override: device state or condition active status. The backend validates the override against a cloned facility and runs the same projector and navigation repair used for live state.

The result contains a private effective terminal preview, active safe effects, and typed validation issues. It does not install state, modify pending requests, write the session, publish to players, advance revisions, or emit a session-state event.

### SaveFacilityAuthoring

Input contains one complete candidate session, expected session revision, and expected facility revision. It is the only browser-authored boundary allowed to change facility definitions, bindings, diagnostic effects, recovery programs, or reference repairs.

Processing rules:

1. Re-resolve the expected active document and revisions.
2. Preserve canonical current device/condition values for stable existing IDs.
3. Initialize only new devices and conditions from authored initial values.
4. Build a complete dependency index across the candidate session.
5. Reject unresolved, ambiguous, duplicate, or unsafe references with FacilityIssue entries.
6. Reject stable-identity deletion/change while references remain.
7. Validate the complete candidate and protobuf round trip.
8. Write once, advance facility revision once, install the canonical facility in coordination, repair the active projection, then publish one session-state and one newer coordination state.

Cancel, close, preview, stale revision, invalid input, and failed storage perform no write and leave the editor open with typed issues. Display-name edits preserve identities. A multi-reference repair succeeds as one complete candidate or not at all.

## Facility State Operations

### ResetFacilityDevice

Input contains device ID, expected facility revision, and one operation correlation ID. After explicit UI confirmation, control serializes the request with player actions and asks the world-action store to restore the device and directly device-scoped conditions to authored initial values.

An already-initial device returns success with Changed false and no new revision or write. A successful change produces one FacilityOperationResult and one canonical session-state update. Unrelated devices and terminal-scoped conditions remain unchanged.

### ResetFacility

Input contains expected facility revision and one operation correlation ID. After explicit UI confirmation, all devices and conditions return to authored initial values in one candidate and one durable write. A failure leaves every value and revision unchanged.

### RecoverFacilityCondition

Input contains condition ID, expected facility revision, operation correlation ID, and exactly one authored private recovery reference. Control verifies that the condition currently allows the reference, expands its bounded transitions/effects, and uses the same atomic world-action store. It cannot submit arbitrary destination states or content.

## Overseer Authoring Surface

The facility workspace must provide keyboard-accessible, labelled controls for:

- facility summary and current revision;
- ordered devices and their states/transitions;
- equality preconditions and condition effects;
- command facility-action assignment;
- menu-name and EntryContent text variants;
- visibility and command-availability rules;
- diagnostic condition scope, category, effects, and recovery references;
- recovery programs and their allowlisted transition requests;
- dependency inspection and repair/reassignment;
- detached state or condition preview;
- single-device reset, whole-facility reset, and private condition recovery.

All dialogs keep drafts isolated until Apply. Destructive/reset actions require explicit confirmation. Escape and Cancel discard drafts. Validation errors use accessible alert/status regions and retain focus in the relevant editor. Successful responses are accepted only when their session and facility revisions are current or newer than the local view.

## Approval and Failure Presentation

- Selecting a facility-backed command opens the existing command approval dialog with the facility impact summary.
- Reject and dialog dismissal continue through the existing rejection decision.
- Approval re-resolves the command and facility graph; the UI does not treat an earlier preview or summary as authority.
- Success closes the pending dialog after the durable result is installed and session-state is published.
- Stale, missing-reference, precondition, conflict, duplicate, invalid-configuration, and persistence failures are distinguished by FacilityFailureCode.
- Player presentation for rejection or failed approved execution remains the existing access-error record; the private UI may show typed repair guidance.

## Retained Audit Contract

Facility events use the existing retained runtime-log sink and closed AuditEvent path. Required event families are:

- `facility.request_received`;
- `facility.decision`;
- `facility.transition`;
- `facility.failure`;
- `facility.recovery`;
- `facility.reset`.

Applicable records carry run ID from logger composition plus correlation/request ID, broadcast ID where present, terminal ID, command ID, action category, sorted device IDs, sorted condition IDs, reset scope, failure/outcome category, previous facility revision, and resulting facility revision. Records exclude device/state display names, transition and confirmation prose, EntryContent, diagnostic record replacements, character names, secrets, raw private errors, and filesystem details.

Logs are never read by session open, world-action validation, replay, conflict resolution, recovery, reset, or projection.
