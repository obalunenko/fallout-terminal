# Contract: Facility Feed Refusal

## Proto (`proto/fallout/terminal/private/v1/coordination.proto`)

```proto
import "fallout/terminal/persistence/v1/session.proto";

// A player request affecting the device was not applied.
message FacilityChangePlayerRefusal {
  string terminal_id = 1;
  string command_id = 2;
  FacilityChangeRefusalReason reason = 3;
  // The entry a fault blocked the player from reading; empty otherwise.
  string entry_id = 4;
  // The capability a fault blocked; unspecified unless reason is FAULT_BLOCKED.
  fallout.terminal.persistence.v1.FacilityCapability capability = 5;
  // The active faults that blocked the request, sorted; empty unless reason is FAULT_BLOCKED.
  repeated string condition_ids = 6;
  // Identical refusals this entry stands for; 0 reads as 1.
  uint32 attempts = 7;
}
```

Additive. `FACILITY_CHANGE_REFUSAL_REASON_FAULT_BLOCKED` already exists.

## Coordination JSON (Wails bridge, `facilityChanges[].playerRefusal`)

| Key | Type | Notes |
|---|---|---|
| `terminalId` | string | unchanged |
| `commandId` | string | unchanged; empty for an entry or a hack |
| `reason` | string | unchanged set; `fault-blocked` for these refusals |
| `entryId` | string | new, omitted when empty |
| `capability` | string | new: `view-entry`, `execute-command`, `hack`, `terminal-transition`, `run-recovery-program`; omitted when unspecified |
| `conditionIds` | string[] | new, omitted when empty |
| `attempts` | number | new, at least 1 |

`desktop-api.js` `normalizeFacilityChange` adds `entryId`, `capability`, `conditionIds` (through `facilityIDs`) and `attempts` (a safe integer of at least 1, else 1).

## Feed rules

- One entry per device in the union of the blocking device faults' devices and, for a command with a device action, its target devices.
- A refusal equal to the device's newest entry replaces it with a new sequence and time and `attempts + 1`.
- The feed keeps five entries per device, newest first.
- A refusal that records or replaces an entry commits a new coordination revision, and the player's result carries it. The Overseer drops a state at a revision it has already seen, so an unchanged revision would hide the entry. This holds for every refusal recorded at request time, including 040's precondition refusals. A refusal that feeds no device keeps the revision.

## Feed text (`#facilityPlayFeedList li.facility-feed-entry`)

```text
HH:MM · <target> · запрос игрока отклонён: мешает неисправность «<fault>»[, «<fault>»] · <terminal>[ · ×N]
```

| Capability | `<target>` |
|---|---|
| `view-entry` | `чтение записи «<entry name>»` |
| `execute-command`, `terminal-transition`, `run-recovery-program` | `«<command name>»` |
| `hack` | `взлом` |

`×N` appears only when `attempts` is 2 or more. Names come from the current session, and a missing name shows its ID, as today. The entry carries `data-kind="player-refusal"`, `data-reason`, `data-capability` and `data-attempts`.
