# Data Model: Unified Facility Authoring

Portable session JSON version 1 is unchanged (FR-021). Every entity below is either runtime-only Go
state, a private desktop contract value (`contracts/private-desktop.md`), or transient Overseer state.

Terminology: the spec's display rules travel as `PresentationRule`; the explanation is the per-node
list of display rules.

## Server runtime state

### Authoring revision (`session.ActiveSession.AuthoringRevision`, runtime only)

| Field | Type | Rule |
|---|---|---|
| `AuthoringRevision` | `uint64` | 1 after create, open, or demo copy; +1 after an accepted mutation that changes the authored configuration |

The authored configuration is the session minus the live values: `terminals[].commandStates`,
`facility.revision`, `facility.devices[].currentStateId`, and
`facility.conditions[].currentActive`.

| Mutation | Effect on authoring revision |
|---|---|
| `SaveAuthoring` (accepted, changed) | +1 |
| `SaveAuthoring` (accepted, unchanged) | unchanged |
| `ReplaceTerminalGroups`, `AssociatePlayerConfig` (changed) | +1 |
| Command execution, world action, recovery, `ResetFacilityDevice`, `ResetCommandState`, `ResetScene` | unchanged |
| Open, create, copy demo | reset to 1 |

The value is never written to JSON (Principle VI) and never logged as content.

### Pending command execution rebase (existing `domain.PendingFacilityAction`)

On an accepted `SaveAuthoring` that changes the facility while a command with a device action is
pending:

- if the recomputed `ActionFingerprint` and `ExpectedSourceStates` are equal, set
  `ExpectedFacilityRevision` to the new facility revision and keep the request pending;
- otherwise, reject it through the existing stale path (`CommandExecutionPhaseRejected`, cached
  result, audit).

### Scene (derived, no new stored field)

| Part | Starting value restored by `ResetScene` |
|---|---|
| Device current state | `initialStateId` |
| Fault activity | `initialActive` (missing means working) |
| Executed commands | none (`commandStates` cleared on every terminal) |

The following are **not** part of the scene and are kept: the active terminal, the return route,
hacking progress, navigation, roster, and controller.

## Presentation explanation (server → Overseer, read-only)

Built by `internal/live` from the canonical session through the same resolvers the player projection
uses.

```text
TerminalPresentation
├── terminalId
└── nodes[] NodePresentation
    ├── nodeId
    ├── playerName           name players see now
    ├── visible              whether players see the node now
    ├── available?           present only when an availability rule exists
    ├── nameRules[]          PresentationRule
    ├── visibilityRules[]    PresentationRule
    ├── availabilityRules[]  PresentationRule
    └── blocks[] BlockPresentation
        ├── blockId
        ├── playerText
        └── textRules[]      PresentationRule
```

`PresentationRule`:

| Field | Meaning |
|---|---|
| `kind` | `AUTHORED_NAME`, `NAME_AFTER_EXECUTION`, `STATE_NAME`, `STATE_VISIBILITY`, `DIAGNOSTIC_FORCED_VISIBILITY`, `STATE_AVAILABILITY`, `DIAGNOSTIC_CAPABILITY_BLOCK`, `INITIAL_TEXT`, `TEXT_AFTER_EXECUTION`, `STATE_TEXT`, `DIAGNOSTIC_SUBSTITUTION` |
| `precedence` | Higher wins while its condition holds (table below) |
| `condition` | none (always), device state equality, command executed (terminal, command), or diagnostic active (condition ID) |
| `value` | `oneof`: `text`, the name or text the rule sets (name and text kinds); or `effect`, what holds while the condition holds (`VISIBLE`, `AVAILABLE`, `UNAVAILABLE`; visibility and availability kinds). `STATE_VISIBILITY` and `STATE_AVAILABILITY` also imply the opposite when the condition does not hold |
| `conditionHolds` | Whether the condition is true for the current live values |
| `playersSee` | Exactly one rule per target is true: the rule that decides what players see now |

Precedence (FR-010, current player behavior):

| Target | Lowest → highest |
|---|---|
| Name | `AUTHORED_NAME` < `NAME_AFTER_EXECUTION` < `STATE_NAME` |
| Block text | `INITIAL_TEXT` < `TEXT_AFTER_EXECUTION` < `STATE_TEXT` < `DIAGNOSTIC_SUBSTITUTION` |
| Visibility | `STATE_VISIBILITY` < `DIAGNOSTIC_FORCED_VISIBILITY` |
| Availability | `STATE_AVAILABILITY` < `DIAGNOSTIC_CAPABILITY_BLOCK` |

A `STATE_NAME` or `STATE_TEXT` variant applies only when it is the **only** variant whose
condition holds (`matchingFacilityText`); two matching variants cancel each other. Two active
diagnostics forcing the same node, or substituting the same block, likewise cancel.

Diagnostic capability blocks never change the player projection: the coordinator rejects the
request instead (`facilityCapabilityBlocked`). `available` reports what players can do, so it is
the projected availability, or `false` while an active capability block applies to the command's
capability (`requestedFacilityCapability`: terminal transition, recovery program, or execute
command).

Invariant (Go test): for every node and block of every terminal, `playerName`, `visible`, and
`playerText` equal the player projection of the same session; `available` equals the projected
availability combined with the active capability blocks; and the `playersSee` rule's value produces
them.

## Preview result additions

| Field | Meaning |
|---|---|
| `current` | Player projection under the current values, from the same snapshot as `terminal` |
| `terminal` | Existing hypothetical projection |
| `differences[]` | `kind` (`NAME_CHANGED`, `TEXT_CHANGED`, `APPEARS`, `DISAPPEARS`, `BECOMES_AVAILABLE`, `BECOMES_UNAVAILABLE`), `nodeId`, optional `blockId`, optional `before`, optional `after` |

Differences are ordered by tree order, then by block order. An empty list means that nothing players
see would change.

## Overseer transient state (`frontend/overseer/src/overseer.js`)

### Authoring edit

| Field | Meaning |
|---|---|
| `label` | Human label, for example «Устройство «Гермодверь Убежища 76»» |
| `scope` | `terminal` or `facility` |
| `mutate(session)` | Pure function over captured inputs; reused by «ПРИМЕНИТЬ СНОВА» |
| `before`, `after` | Normalized authored snapshots (shared between consecutive entries) |

Normalization removes the live values listed above and `terminalGroups`.

### Save pipeline

```text
idle ──commit──▶ saving ──ok──────────▶ idle (lastSavedAt = now)
  ▲                │ ├─invalid────────▶ error(reason)      only this edit reverts; later queued edits replay
  │                │ └─stale──────────▶ stale(reason)      «ПРИМЕНИТЬ СНОВА» enabled
  │                └─later commits wait in order; one request per edit
  └──── reapply / next commit ◀────────────────────────────┘
```

| Field | Meaning |
|---|---|
| `workingCopy` | Session shown and edited |
| `lastSaved` | Last canonical session acknowledged by the server |
| `authoringRevision` | Last authoring revision returned by the server |
| `inFlight`, `queued[]` | At most one request; each queued edit is sent on its own, in order |
| `lastSavedAt`, `lastError` | Drive the shared status |

### Undo and redo history

| Rule | Value |
|---|---|
| Depth | 20 entries (FR-001a minimum) |
| Undo allowed | The newest undo entry's `after` equals the normalized `lastSaved`, and the pipeline is idle |
| Redo allowed | The newest redo entry's `before` equals the normalized `lastSaved`, and the pipeline is idle |
| New edit | Clears redo |
| Open, create, copy | Clears both |
| Rejected undo or redo | Stacks unchanged; the server's reason is shown on the control |

### Unapplied input

| Field | Meaning |
|---|---|
| key | `node:<terminalId>:<nodeId>`, `settings:<terminalId>`, or `device:<deviceId>` |
| `label` | Name shown in the status: «НЕ ПРИМЕНЕНО: …» |
| `values` | Field values keyed by field ID |

An entry is created on the first input and removed on apply or cancel. Re-rendering the same entity
restores the values. An entry whose entity no longer exists after a live change or an undo stays
listed and opens a notice: «ОБЪЕКТ ПРАВКИ БОЛЬШЕ НЕ СУЩЕСТВУЕТ».

### Scene reset confirmation

Computed from `workingCopy` when the dialog opens:

- devices whose current state is not the initial state;
- faults whose activity is not the initial activity;
- executed commands per terminal;
- whether a request is pending, and whether it has a device action.

The dialog sends the session and facility revisions it counted against.

## Validation rules carried over

- Candidate validation: `domain.ValidateFacilityAuthoringCandidate`, `verifySessionContract`,
  `validateAuthoredTransitionChanges`. Failures return `INVALID_CONFIGURATION` or
  `MISSING_REFERENCE` with `FacilityIssue` entries. The Overseer turns them into the reason it shows.
- A completed command must keep its state-change configuration (`mergeCanonicalSessionState`).
- The bundled demo is read-only. Saves and resets return a reason that suggests a demo copy.
