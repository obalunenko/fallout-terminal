# Contract: Version-1 Session Facility State

## Compatibility Boundary

The portable document remains JSON version 1. Protobuf continues to define known application fields, while the explicit JSON adapter remains the storage format and preserves compatible unknown fields. Existing field names and numbers remain unchanged; all additions use new numbers.

`Session.facility` is optional. Its absence means no facility is configured and does not synthesize devices, conditions, programs, current values, or a facility revision during an unrelated save.

## Persistence Schema Additions

The following is the planned contract shape. Final schema formatting and generated outputs come only from the pinned repository generation workflow.

```proto
enum FacilityDeviceKind {
  FACILITY_DEVICE_KIND_UNSPECIFIED = 0;
  FACILITY_DEVICE_KIND_DOOR = 1;
  FACILITY_DEVICE_KIND_TURRET = 2;
  FACILITY_DEVICE_KIND_POWER_GRID = 3;
  FACILITY_DEVICE_KIND_REACTOR = 4;
  FACILITY_DEVICE_KIND_VENTILATION = 5;
  FACILITY_DEVICE_KIND_ALARM = 6;
  FACILITY_DEVICE_KIND_ROBOT_POD = 7;
  FACILITY_DEVICE_KIND_ELEVATOR = 8;
  FACILITY_DEVICE_KIND_NETWORK_SEGMENT = 9;
  FACILITY_DEVICE_KIND_CUSTOM = 10;
}

enum DiagnosticConditionCategory {
  DIAGNOSTIC_CONDITION_CATEGORY_UNSPECIFIED = 0;
  DIAGNOSTIC_CONDITION_CATEGORY_OFFLINE = 1;
  DIAGNOSTIC_CONDITION_CATEGORY_UNPOWERED = 2;
  DIAGNOSTIC_CONDITION_CATEGORY_NETWORK_ISOLATED = 3;
  DIAGNOSTIC_CONDITION_CATEGORY_STORAGE_DAMAGED = 4;
  DIAGNOSTIC_CONDITION_CATEGORY_AUTHORIZATION_CORRUPTED = 5;
  DIAGNOSTIC_CONDITION_CATEGORY_DISPLAY_UNSTABLE = 6;
  DIAGNOSTIC_CONDITION_CATEGORY_CUSTOM = 7;
}

enum FacilityCapability {
  FACILITY_CAPABILITY_UNSPECIFIED = 0;
  FACILITY_CAPABILITY_EXECUTE_COMMAND = 1;
  FACILITY_CAPABILITY_VIEW_ENTRY = 2;
  FACILITY_CAPABILITY_HACK = 3;
  FACILITY_CAPABILITY_TERMINAL_TRANSITION = 4;
  FACILITY_CAPABILITY_RUN_RECOVERY_PROGRAM = 5;
}

message FacilityStateEquality {
  string device_id = 1;
  string state_id = 2;
}

message FacilityTransitionRequest {
  string device_id = 1;
  string transition_id = 2;
}

message FacilityConditionEffect {
  string condition_id = 1;
  bool active = 2;
}

message FacilityDeviceState {
  string id = 1;
  string name = 2;
}

message FacilityDeviceTransition {
  string id = 1;
  string name = 2;
  string source_state_id = 3;
  string destination_state_id = 4;
  repeated FacilityStateEquality preconditions = 5;
  repeated FacilityConditionEffect condition_effects = 6;
  bool recovery = 7;
}

message FacilityDevice {
  string id = 1;
  string name = 2;
  FacilityDeviceKind kind = 3;
  optional string custom_kind = 4;
  string initial_state_id = 5;
  string current_state_id = 6;
  repeated FacilityDeviceState states = 7;
  repeated FacilityDeviceTransition transitions = 8;
}

message DiagnosticDeviceScope {
  string device_id = 1;
}

message DiagnosticTerminalScope {
  string terminal_id = 1;
}

message CapabilityBlockEffect {
  FacilityCapability capability = 1;
}

message DiagnosticPathEffect {
  string terminal_id = 1;
  string node_id = 2;
}

message RecordSubstitutionEffect {
  string terminal_id = 1;
  string block_id = 2;
  string replacement_text = 3;
}

message DisplayInstabilityEffect {}

message DiagnosticEffect {
  oneof effect {
    CapabilityBlockEffect capability_block = 1;
    DiagnosticPathEffect diagnostic_path = 2;
    RecordSubstitutionEffect record_substitution = 3;
    DisplayInstabilityEffect display_instability = 4;
  }
}

message DiagnosticRecoveryReference {
  oneof recovery {
    FacilityTransitionRequest transition = 1;
    string recovery_program_id = 2;
    bool private_overseer_action = 3;
  }
}

message DiagnosticCondition {
  string id = 1;
  string name = 2;
  DiagnosticConditionCategory category = 3;
  optional string custom_category = 4;
  oneof scope {
    DiagnosticDeviceScope device = 5;
    DiagnosticTerminalScope terminal = 6;
  }
  bool initial_active = 7;
  bool current_active = 8;
  repeated DiagnosticEffect effects = 9;
  repeated DiagnosticRecoveryReference recovery = 10;
}

message RecoveryProgram {
  string id = 1;
  string name = 2;
  repeated FacilityTransitionRequest transitions = 3;
}

message FacilityActionConfig {
  oneof action {
    FacilityTransitionList transitions = 1;
    string recovery_program_id = 2;
  }
}

message FacilityTransitionList {
  repeated FacilityTransitionRequest transitions = 1;
}

message FacilityTextVariant {
  FacilityStateEquality when = 1;
  string text = 2;
}

message Facility {
  uint64 revision = 1;
  repeated FacilityDevice devices = 2;
  repeated DiagnosticCondition conditions = 3;
  repeated RecoveryProgram recovery_programs = 4;
}
```

Existing messages receive only these additions:

```proto
message StateChangeConfig {
  string completed_name = 1;
  string confirmation_text = 2;
  EntryContentChange entry_content_change = 3;
  optional FacilityActionConfig facility_action = 4;
}

message EntryContentBlock {
  string id = 1;
  string initial_text = 2;
  repeated FacilityTextVariant facility_text_variants = 3;
}

message ContentNode {
  string id = 1;
  string name = 2;
  oneof content {
    FolderContent folder = 3;
    CommandContent command = 4;
    EntryContent entry = 5;
  }
  repeated FacilityTextVariant facility_name_variants = 6;
  optional FacilityStateEquality visible_when = 7;
  optional FacilityStateEquality available_when = 8;
}

message Session {
  int32 version = 1;
  string name = 2;
  optional string player_config = 3;
  repeated Terminal terminals = 4;
  repeated TerminalGroup terminal_groups = 5;
  optional Facility facility = 6;
}
```

`available_when` is valid only on command nodes. Existing command behavior remains unchanged: facility actions are a field of StateChangeConfig, so state-changing commands retain their completion snapshot and approval behavior. CommandExecutionState receives no facility data; device-driven presentation always derives from Facility.

## JSON Shape

A representative additive document fragment is:

```json
{
  "version": 1,
  "name": "Vault 81",
  "facility": {
    "revision": 12,
    "devices": [
      {
        "id": "reactor-main",
        "name": "Main Reactor",
        "kind": "reactor",
        "initialStateId": "offline",
        "currentStateId": "online",
        "states": [
          {"id": "offline", "name": "Offline"},
          {"id": "online", "name": "Online"}
        ],
        "transitions": [
          {
            "id": "start",
            "name": "Start reactor",
            "sourceStateId": "offline",
            "destinationStateId": "online",
            "preconditions": [
              {"deviceId": "cooling-loop", "stateId": "online"}
            ],
            "conditionEffects": [
              {"conditionId": "reactor-unpowered", "active": false}
            ]
          }
        ]
      }
    ],
    "conditions": [],
    "recoveryPrograms": []
  }
}
```

An EntryContent block remains exactly an `EntryContent` block and adds only optional state variants:

```json
{
  "id": "reactor-status",
  "initialText": "REACTOR: OFFLINE",
  "facilityTextVariants": [
    {
      "when": {"deviceId": "reactor-main", "stateId": "online"},
      "text": "REACTOR: ONLINE"
    }
  ]
}
```

## Persistence and Revision Rules

- The JSON field `version` remains `1`.
- Every existing protobuf field number and JSON field name remains stable.
- All new enums use an UNSPECIFIED zero value; persisted required enum fields reject UNSPECIFIED.
- Oneofs distinguish absent scope, effect, recovery, and action variants from malformed multiple variants.
- Empty text remains valid for authored EntryContent and diagnostic record substitutions.
- All new nested durable records preserve compatible unknown JSON fields through explicit Extra maps and adapters.
- Ordinary SaveSession input cannot overwrite Facility revision or current device/condition values. The session owner merges canonical protected values by stable ID, as it already does for command snapshots.
- Facility definition, binding, condition, program, and multi-reference repair changes use the revision-aware facility-authoring operation and advance Facility revision once after one successful atomic document replacement.
- A player world action captures the existing command completion snapshot and every facility destination in one candidate and one write.
- A failed encode, validation, round trip, temporary-file write, sync, or rename leaves the previous complete document and Facility revision authoritative.
- Unknown or incomplete committed facility data causes session load to fail with a stable private configuration category before player publication; no partial facility is installed.

## Compatibility Verification

- Existing version-1 fixtures with no `facility` field decode, render, and re-encode without an invented facility.
- Existing ordinary, state-changing, EntryContent, terminal-transition, hacking, role, and terminal-group fixtures retain their behavior.
- Existing StateChangeConfig values without `facilityAction` remain valid.
- Existing ContentNode and EntryContentBlock values without bindings retain their effective names, visibility, availability, and text.
- The generated schema revision and compatibility baseline are updated only after format, lint, generation-drift, and breaking checks pass.
