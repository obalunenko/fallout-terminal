# Contract: Session JSON Version 1 Entry Content State

## Additive Persistence Schema

Existing message and field numbers remain unchanged. The persisted entity name remains exactly `EntryContent`.

```proto
message EntryContentBlock {
  string id = 1;
  string initial_text = 2;
}

message EntryContentChange {
  string block_id = 1;
  string completed_text = 2;
}

message StateChangeConfig {
  string completed_name = 1;
  string confirmation_text = 2;
  EntryContentChange entry_content_change = 3;
}

message CommandExecutionState {
  string completed_name = 1;
  string result_text = 2;
  EntryContentChange entry_content_change = 3;
}

message EntryContent {
  string description = 1;
  repeated EntryContentBlock blocks = 2;
}
```

Nested message presence distinguishes no entry target from a target whose completed text is empty. Existing state-changing configurations and snapshots decode with no entry-content change.

## JSON Shape

Explicit entry blocks remain fields of the established tagged content node:

```json
{
  "id": "reactor-status",
  "type": "entry",
  "name": "REACTOR STATUS",
  "blocks": [
    {"id": "reactor-power", "initialText": "POWER: OFFLINE"},
    {"id": "reactor-cooling", "initialText": "COOLING: OFFLINE"}
  ]
}
```

A targeting command adds one optional configuration object:

```json
{
  "id": "restore-power",
  "type": "command",
  "name": "RESTORE POWER",
  "text": "Primary power restored.",
  "stateChange": {
    "completedName": "POWER RESTORED",
    "confirmationText": "Authorize reactor power restoration?",
    "entryContentChange": {
      "blockId": "reactor-power",
      "completedText": "POWER: ONLINE"
    }
  }
}
```

After the first successful execution, the existing terminal map owns the frozen outcome:

```json
{
  "commandStates": {
    "restore-power": {
      "completedName": "POWER RESTORED",
      "resultText": "Primary power restored.",
      "entryContentChange": {
        "blockId": "reactor-power",
        "completedText": "POWER: ONLINE"
      }
    }
  }
}
```

## Compatibility Rules

- Session `version` remains `1`.
- `EntryContent.description` remains protobuf field 1 and JSON field `description`.
- Existing sessions with only `description` decode, render, and re-encode without an automatic block conversion.
- An absent or empty `blocks` field uses the legacy description; if that description is also empty, the entry renders empty.
- A non-empty legacy `description` and explicit blocks may not coexist in one entry.
- Existing `StateChangeConfig` and `CommandExecutionState` field numbers 1 and 2 remain stable.
- Unknown session, terminal, and content-node fields continue through the established explicit JSON extras adapter.
- The reviewed protobuf compatibility baseline is not replaced; additive changes are checked against it.

## Mutation and Revision Rules

- EntryContent-side assignment, reassignment, removal, and new-command creation use the existing complete session document and `SaveSession` path; they add no persistence field or dedicated mutation method.
- Each accepted dialog action submits one valid authored candidate: reassignment clears the old uncompleted command's nested change and sets the new command's change together, removal clears only the nested change, and creation inserts one fully configured command into the selected same-terminal folder.
- Cancel, close, invalid input, stale dialog references, and attempts to change a completed owner submit no session save.
- First execution inserts command presentation and frozen entry change in one `commandStates` value and one document revision.
- Repeated execution is a no-op with no new document revision.
- Individual reset deletes one complete value; terminal reset clears the complete map.
- Storage failure restores the prior document and requested revision.
- A stale full-document save may not overwrite frozen snapshots.
- A full-document save that removes or retargets a frozen block while retaining its completed command is rejected.
- Deleting the owning command prunes its complete snapshot.

## Generation Contract

`internal/gen/fallout/terminal/persistence/v1/session.pb.go` and the schema revision are generated only through the repository-pinned protobuf workflow. No persistence ECMAScript output is added, and generated files are never edited manually.
