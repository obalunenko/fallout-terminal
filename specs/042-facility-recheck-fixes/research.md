# Research: Facility Recheck Fixes

## R1: The player reason

**Decision**: Add `ACTION_REASON_FAULT_BLOCKED = 11` to `ActionReason` in `player/v1/player.proto` and `ActionReasonFaultBlocked = "fault-blocked"` to `internal/domain/model.go`. The capability-blocked branch of player actions in `internal/control/service.go` returns it instead of `ActionReasonInvalidAction`. `client.js` maps it to «ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.».

**Rationale**: The constitution keeps structured results separate from localized text, and the spec asks for a reason distinct from the generic one. One enum value is the smallest additive change. The message carries no fault name, so no Overseer data crosses to players.

**Alternatives considered**: A detail string on the existing reason, rejected because it would put free text on the public contract. A client-side guess from the terminal state, rejected because the client does not know which capabilities are blocked.

## R2: The refusal record

**Decision**: Extend `FacilityChangePlayerRefusal` in `private/v1/coordination.proto` and its domain twin with four fields: `entry_id` (the blocked entry, empty otherwise), `capability` (the blocked capability, using `persistence.v1.FacilityCapability` through an import), `condition_ids` (the blocking faults, sorted), and `attempts` (1 for a first refusal). `command_id` keeps naming a command target. A hack refusal names neither.

**Rationale**: The feed already models refused player requests, the Overseer already renders them, and the refusal reason `FAULT_BLOCKED` already exists. Adding fields keeps one renderer and one merge rule. Importing the persistence enum reuses the capability names the session already stores, as `runtime.proto` and `desktop.proto` already import that file.

**Alternatives considered**: A new `FacilityChange` variant for blocked capabilities, rejected because it would duplicate the refusal variant and its rendering. A string capability, rejected because the enum already exists.

## R3: Which feeds get an entry

**Decision**: A fault-block refusal goes to the union of two device sets: the devices of the active device-scoped faults that block it, and, for a command with a device action, the devices that action targets. Each device in the union gets one entry carrying the same record. Terminal-scoped faults contribute no device.

**Rationale**: FR-003 puts the entry on the blocking fault's device. FR-005 keeps today's entries on targeted devices and forbids two entries in one feed. A union with one entry per device satisfies both.

**Alternatives considered**: Only the fault's device, rejected because it drops today's entry on the targeted door. Separate entries for "blocked" and "targeted", rejected by FR-005.

## R4: Repeats

**Decision**: `appendFacilityChanges` in `internal/control/feed.go` compares a new fault-block refusal with the device's newest entry. When the terminal, command, entry, capability, condition IDs and reason match, it replaces that entry with the new time, a new sequence, and `attempts` plus one. Otherwise it prepends as today. Other refusals never merge: each Overseer decision or failure stays its own entry.

**Rationale**: The server owns the feed and keeps five entries per device, so counting must happen there. A fresh sequence keeps the snapshot's newest-first order and the rule that sequences are never reused.

**Alternatives considered**: Counting in the Overseer, rejected because it sees only the five kept entries. A time window, rejected as a second rule the spec does not ask for.

## R5: Log facts

**Decision**: The rejection's existing `player.action.outcome` audit gets `Facility` facts: action `capability-block`, the terminal, the command or entry ID, the capability, and the condition IDs, with outcome `fault-blocked`. No authored text is logged.

**Rationale**: FR-006 asks for terminal, capability, faults and correlation; the audit context already carries the request ID and broadcast. `FacilityAuditFacts` already has `TerminalID`, `CommandID` and `ConditionIDs`; it gains a capability field and an entry ID.

**Alternatives considered**: A separate audit event, rejected because the outcome event already exists for every player action.

## R6: The diagnostics fixture

**Decision**: The first fixture task reproduces the gap before changing anything. It runs the `offline` scenario, opens «AFFECTED ENTRY», and records whether the server accepted the navigation. The fixture's faults are terminal-scoped on `terminal-facility-diagnostics`, so `facilityCapabilityBlocked` should refuse it; the open entry therefore points to either a participant on another terminal or a facility the runtime does not hold. Whatever the cause, the fix routes the request through the same refusal branch as the app. The affected entries then get authored text such as «AFFECTED ENTRY TEXT // READABLE», and the journeys assert the reason, the missing text, and the text after the fault is switched off.

**Rationale**: FR-012 to FR-014 require that a regression makes the journey fail. Rewriting the assertions without knowing why the entry opens could keep the gap.

**Alternatives considered**: Changing only the entry text, rejected because the journey might then fail for a reason unrelated to the block.

## R7: Templates

**Decision**: `facilityDraftTemplateAdditions` in `overseer.js` keeps a template transition only when its source or destination state is missing or `facilityDraftCellRows(source, destination)` is empty. Names no longer matter.

**Rationale**: FR-008. The cell rows are already how the matrix finds transitions.

**Alternatives considered**: None; the user chose the cell rule.

## R8: Sidebar and layout

**Decision**: «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» becomes a `<details>` group with a summary showing «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ · АКТИВНО N». Its open state is kept in memory for the run, as the 040 story guide keeps `facilityGuideChoice`; the Overseer uses no browser storage. The sidebar becomes a column flex box so the device list takes the height the group frees. The list keeps its scrolling but gets inner padding so row borders and focus outlines are never clipped. `#facilityStatus` is hidden while empty. The fault start-position radios become the segmented controls the Overseer already uses for its filters. Tree rows get a space between kind and name. `facilityRowActions` writes «ИЗМЕНИТЬ» and «УДАЛИТЬ»; the accessible names keep their sentence case.

**Rationale**: Each item uses a pattern the Overseer already has: the guide's remembered collapse, the filter segments, and `explainControl` text. Seven rows at 1440×900 need about 440 points for the list; collapsing the group frees about 180.

**Alternatives considered**: Compact one-line device rows, rejected because 041 FR-030 lays out name over type. A second scroll area for terminal faults, rejected because it adds a scroll inside a scroll.
