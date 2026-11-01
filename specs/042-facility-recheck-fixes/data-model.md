# Data Model: Facility Recheck Fixes

Session JSON version 1 does not change. Every entity below lives in the process runtime for the current run, or in the frontend.

## Action reason (player contract)

`ActionReason` gains one value.

| Proto value | Domain value | Meaning |
|---|---|---|
| `ACTION_REASON_FAULT_BLOCKED = 11` | `fault-blocked` | An active fault's capability block refused the request. |

Rules:

- Returned only by the capability-blocked branch of player actions. Every other refusal keeps its reason.
- Carries no fault identity.

## Fault-block refusal (feed record)

`FacilityChangePlayerRefusal` keeps `terminal_id`, `command_id` and `reason`, and gains:

| Field | Type | Meaning |
|---|---|---|
| `entry_id` | string | The entry the player tried to read; empty for any other target. |
| `capability` | `persistence.v1.FacilityCapability` | The blocked capability; unspecified for refusals that are not fault blocks. |
| `condition_ids` | repeated string | The active faults that blocked the request, sorted; empty for other refusals. |
| `attempts` | uint32 | How many identical refusals this entry stands for; 1 for a first refusal; 0 reads as 1. |

Target by capability:

| Capability | Target fields |
|---|---|
| `view-entry` | `entry_id` |
| `execute-command`, `terminal-transition`, `run-recovery-program` | `command_id` |
| `hack` | neither; the terminal is the target |

Rules:

- `reason` is `fault-blocked` whenever `condition_ids` is non-empty.
- A fault-block refusal is recorded once per device in the union of the blocking device-scoped faults' devices and, for a command with a device action, the devices it targets (research R3).
- Two refusals are the same when `terminal_id`, `command_id`, `entry_id`, `capability`, `condition_ids` and `reason` all match.

## Feed entry (`FacilityChange`)

No new variant. The repeat rule changes how a player refusal is appended:

```text
new refusal for device D
  ├─ D's newest entry is the same refusal → replace it: new time, new sequence, attempts + 1
  └─ otherwise                             → prepend; keep the newest five
```

Sequences stay strictly increasing and are never reused. A replaced entry's old sequence disappears.

## Facility audit facts

`FacilityAuditFacts` gains `Capability` and `EntryID`. A fault-block refusal logs action `capability-block`, outcome `fault-blocked`, the terminal, the command or entry ID, the capability and the condition IDs. No authored text.

## Device template (frontend)

Unchanged shape. A template transition is present when the draft has any transition in the cell from the state named like its source to the state named like its destination, names compared case-insensitively. The apply button is blocked with «ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ» when no state and no transition is missing.

## Terminal-fault group state (frontend)

One in-memory flag for the run, like `facilityGuideChoice` (040): collapsed or expanded. The group starts expanded at launch and keeps the Overseer's choice when a session is reopened.
