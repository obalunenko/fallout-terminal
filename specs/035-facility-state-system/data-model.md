# Data Model: Shared Facility State System

## Ownership Boundaries

The version-1 session document is the durable owner of facility definitions, current values, and the facility revision. The session service owns mutation and atomic storage. The coordination service keeps one detached in-memory facility snapshot for transaction validation and publication; it does not persist a second copy. Terminal runtimes and terminal groups never own facility devices.

## Durable Entities

### Session

Existing fields remain unchanged. One optional facility field is added.

| Field | Presence | Meaning |
|---|---|---|
| Facility | Optional | Session-wide authored facility and current state. Absence means no facility is configured. |

A legacy document without Facility remains absent after an unrelated open/save cycle. Adding the first device creates the aggregate explicitly.

### Facility

| Field | Rules | Meaning |
|---|---|---|
| Revision | Non-negative integer | Persistent total-order identity for the current definition and world state. |
| Devices | Ordered, bounded, unique IDs | Reusable world devices available to all terminals and groups. |
| Conditions | Ordered, bounded, unique IDs | Deterministic diagnostic conditions attached to devices or terminals. |
| Recovery Programs | Ordered, bounded, unique IDs | Finite holotape-compatible selectors for authored recovery actions. |
| Extra | Unknown JSON fields | Compatible fields preserved during version-1 round trips. |

Revision starts at zero when a facility is first authored. Every committed world action, recovery, reset, or published facility-graph edit advances it by exactly one. Preview, rejection, validation failure, a repeated decision, and an already-initial reset do not advance it.

### Facility Device

| Field | Rules | Meaning |
|---|---|---|
| ID | Stable, nonblank, unique | Reference identity; changing it is a delete-and-create operation. |
| Name | Authored display text | Overseer-facing label; may change without retargeting references. |
| Kind | Bounded authored category | Door, turret, power grid, reactor, ventilation, alarm, robot pod, elevator, network segment, or custom. |
| Initial State ID | Must resolve within States | Value restored by device or facility reset. |
| Current State ID | Protected server-owned value resolving within States | Last committed value used by transition validation and projection. |
| States | Non-empty, bounded, unique IDs and names | Complete finite state set. |
| Transitions | Bounded, unique IDs | Explicit allowed movements for this device. |
| Extra | Unknown JSON fields | Compatible nested-field preservation. |

Device state IDs are scoped to their device. Display names need not be globally unique and never act as references.

### Device State Definition

| Field | Rules | Meaning |
|---|---|---|
| ID | Stable and unique within its device | Equality and transition reference. |
| Name | Nonblank authored label | Overseer-facing state description. |
| Extra | Unknown JSON fields | Compatible nested-field preservation. |

### Device Transition Definition

| Field | Rules | Meaning |
|---|---|---|
| ID | Stable and unique within its device | Command and recovery-program reference. |
| Name | Nonblank authored label | Overseer-facing action description. |
| Source State ID | Resolves within owning device | Required current state before execution. |
| Destination State ID | Resolves within owning device and differs from source | State installed on success. |
| Preconditions | Bounded AND-list of state equalities | Other required device states evaluated against the same pre-state. |
| Condition Effects | Bounded typed list | Named conditions activated or cleared with the transition. |
| Recovery | Boolean metadata | Identifies a transition that may be selected by a recovery path; it grants no extra authority. |
| Extra | Unknown JSON fields | Compatible nested-field preservation. |

A transition never contains code, expressions, timing, randomness, combat rules, or repair-success logic.

### State Equality

| Field | Rules | Meaning |
|---|---|---|
| Device ID | Resolves to one facility device | Device being compared. |
| State ID | Resolves within that device | Required equality value. |

Transition preconditions may contain several equalities and combine them only with AND. A presentation or availability rule contains exactly one equality.

### Condition Effect

| Field | Rules | Meaning |
|---|---|---|
| Condition ID | Resolves to one diagnostic condition | Condition changed by a device transition. |
| Active | Explicit boolean | Destination condition status. |

Two transitions in one action may not request contradictory statuses for the same condition.

### Diagnostic Condition

| Field | Rules | Meaning |
|---|---|---|
| ID | Stable, nonblank, unique | Transition, recovery, dependency, and audit reference. |
| Name | Authored display text | Overseer-facing fault label. |
| Category | Bounded enum plus custom authored category | Offline, unpowered, network-isolated, storage-damaged, authorization-corrupted, display-unstable, or custom. |
| Scope | Exactly one device ID or terminal ID | Entity diagnosed as affected. |
| Initial Active | Explicit boolean | Status restored by the applicable reset. |
| Current Active | Protected server-owned boolean | Last committed status. |
| Effects | Non-empty bounded typed list | Deterministic behavior while active. |
| Recovery References | Bounded references | Allowed recovery transitions, programs, or private recovery actions. |
| Extra | Unknown JSON fields | Compatible nested-field preservation. |

Custom category changes only authored identity and presentation. It cannot define a new executable effect type.

### Diagnostic Effect

Each effect is exactly one typed variant.

| Variant | Data | Result while the condition is active |
|---|---|---|
| Capability Block | One bounded capability enum | The server rejects that capability for the affected terminal or device. |
| Diagnostic Path | Terminal ID and existing node ID | The authored diagnostic node becomes visible/reachable. |
| Record Substitution | Terminal ID, EntryContent block ID, authored replacement text | The effective record uses the deterministic damaged variant. |
| Display Instability | Safe bounded effect kind | The player receives a presentation-only visual flag. |

Supported capability values initially cover command execution, entry viewing, hacking, terminal transition, and recovery-program invocation. Back, acknowledgement, and private Overseer recovery are never blockable capabilities.

### Recovery Program

| Field | Rules | Meaning |
|---|---|---|
| ID | Stable, nonblank, unique | Command and condition recovery reference. |
| Name | Authored display text | Overseer-facing holotape program label. |
| Transition Requests | Non-empty, bounded, at most one per device | Allowlisted recovery transitions expanded on invocation. |
| Extra | Unknown JSON fields | Compatible nested-field preservation. |

A recovery program is data, not executable code. It cannot read arbitrary state, choose random actions, branch, loop, mutate content, or decide character success.

### Facility Transition Request

| Field | Rules | Meaning |
|---|---|---|
| Device ID | Resolves to one device | Owning device for the requested transition. |
| Transition ID | Resolves within that device | Exact authored transition to execute. |

The same device may occur at most once in an expanded world action.

### Facility Action Configuration

This optional structure is nested in the existing state-changing command configuration. It uses exactly one action source:

- a non-empty ordered set of direct Facility Transition Requests; or
- one Recovery Program ID whose transition requests are expanded before approval.

The existing completed command name, confirmation text, result text, optional EntryContent change, and immutable CommandExecutionState remain unchanged. A command may therefore commit its completion snapshot, one EntryContent block outcome, and one facility action together.

### Facility State Binding

| Field | Rules | Meaning |
|---|---|---|
| When | Exactly one valid State Equality | Condition selecting the bound behavior or variant. |
| Value | Typed by target property | Authored label or text value where the binding supplies content. |

Bindings are attached to these existing authored targets:

- content-node name variants;
- EntryContent block text variants;
- command availability;
- content-node visibility.

All variants for one property must use the same controlling device and unique state IDs. Visibility and availability have at most one equality rule each: equality true enables the property, equality false disables it. A binding that does not match leaves the prior precedence layer in effect.

## Runtime and Transaction Entities

### Facility Snapshot

A deep-cloned Facility stored once on the process runtime outside LiveBroadcast. It is installed from the active session on load and replaced only with a validated durable result. Terminals receive it as immutable projection input and do not retain independent current values.

### Pending World Action

Nested in the existing pending command execution and never persisted.

| Field | Meaning |
|---|---|
| Correlation ID | Existing command request identity carried through approval, audit, and result. |
| Expected Facility Revision | Persistent facility revision observed when the request became pending. |
| Action Fingerprint | Deterministic identity of the authored command action at request time. |
| Transition Requests | Fully expanded stable device and transition references. |
| Expected Source States | Detached source values used for stale detection and Overseer summary. |
| Affected Condition IDs | Safe identities of transition side effects. |

Approval never trusts this snapshot alone; it re-resolves the command and graph from current canonical data and compares the fingerprint and expected revision.

### World Action Candidate

One session clone created inside the serialized store boundary. It contains:

- an optional new CommandExecutionState for the selected command;
- simultaneous destination state for each affected device;
- simultaneous active status for each affected condition;
- one incremented facility revision; and
- all unrelated session content and unknown fields unchanged.

The candidate becomes canonical only after complete validation, round-trip verification, and one successful atomic file replacement.

### Facility Mutation Result

| Field | Meaning |
|---|---|
| Changed | Whether one durable replacement occurred. |
| Correlation ID | Request or private operation identity. |
| Outcome | Success, rejected, or one typed failure. |
| Session Revision | Durable document revision after the operation. |
| Previous Facility Revision | Revision used for comparison and audit. |
| Resulting Facility Revision | Canonical revision after success or unchanged revision after failure/no-op. |
| Affected Device IDs | Sorted safe identities. |
| Affected Condition IDs | Sorted safe identities. |
| Session | Detached canonical session for accepted UI reconciliation. |

### Facility Failure Code

The closed failure set is:

- unspecified;
- rejected;
- missing-reference;
- invalid-transition;
- precondition-failed;
- stale-revision;
- conflict;
- duplicate;
- invalid-configuration;
- persistence-failed;
- runtime-context-ended.

Human-readable private guidance may accompany a code, but control flow, tests, and logging use the enum and stable IDs.

### Facility Dependency Report

A detached read model listing every direct reference to selected device, state, transition, condition, or recovery program. Each item contains a typed source kind, stable source identity, target identity, property, and terminal context where applicable. Display labels may be added only in the private Overseer projection and are not used for repair.

### Facility Preview

A detached candidate facility state or condition override plus one terminal ID. The pure live evaluator returns an effective private terminal preview and validation issues. Preview is never installed into Session, ProcessRuntime, LiveBroadcast, revisions, pending requests, player streams, or retained state events.

## Effective Projection Rules

For each property, projection applies these layers in order:

1. authored base content;
2. valid immutable completed-command snapshot;
3. the matching device-state binding, if any;
4. the active diagnostic-condition override, if any.

The resulting tree is detached. Invisible nodes are omitted. Unavailable commands remain present with an explicit unavailable marker. EntryContent blocks are resolved independently and joined by the existing block composition rule. Active display-instability effects become safe terminal flags; they never alter text or canonical state.

After projection, navigation and controller presentation are revalidated. A hidden or blocked selected target returns to the nearest valid parent or root. Back, resolved-command acknowledgement, and private Overseer recovery remain available even when authored capabilities are blocked.

## World Action State Transitions

| Current state | Trigger | Result |
|---|---|---|
| Authored command available | Controller selects command | Existing pending approval plus Pending World Action; no durable change. |
| Pending | Overseer rejects or dismisses | Pending clears, facility stays unchanged, existing rejected/access-error presentation appears. |
| Pending, current revision and graph match | Overseer approves | One candidate writes; command snapshot, devices, conditions, and facility revision install together; one coordination revision publishes. |
| Pending, graph or revision changed | Overseer approves | Typed stale or missing-reference failure; no write; rejected/access-error presentation appears. |
| Pending, precondition false | Overseer approves | Typed precondition failure; no write; rejected/access-error presentation appears. |
| Pending, storage fails | Overseer approves | Typed persistence failure; prior session and runtime remain authoritative; failure presentation appears. |
| Completed command selected again | Existing approval completes | Frozen command result is shown; facility action is not repeated and no facility revision advances. |
| Any state | Confirmed single-device reset | Device and device-scoped conditions return to authored initial values together; one revision if changed. |
| Any state | Confirmed whole-facility reset | Every device and condition returns to authored initial values together; one revision if changed. |
| Active condition | Approved transition/program/private recovery | Referenced conditions and device states change through the same candidate rules. |
| Any state | Preview | Detached effective result only; no revisions, events, or state changes. |
| Any pending state | Session replacement or newer facility commit | Pending action becomes incapable of commit and later resolution returns a typed stale/runtime-context result. |

## Validation and Reference Rules

- Every stable identity is nonblank, bounded, valid UTF-8, and unique in its defined scope.
- All device initial/current states and transition endpoints resolve within the owning device.
- All precondition, condition-effect, scope, binding, diagnostic-path, EntryContent block, command, and recovery-program references resolve in the same complete session candidate.
- A world action contains at most one transition per device and no contradictory condition effects.
- A state transition's source and destination differ.
- Presentation variants for one property cannot depend on more than one device or duplicate a state.
- At most one diagnostic condition may control the same record substitution or diagnostic path at a time unless validation proves the conditions mutually exclusive; the initial implementation rejects overlap rather than inferring exclusivity.
- Every active condition that can block normal progress has at least one valid recovery reference or the private Overseer recovery path.
- Deleting or changing stable identity is rejected while dependencies remain. Display-name edits preserve identity and state.
- Multi-reference repair is validated and persisted as one complete authoring candidate.
- Ordinary browser-authored session saves cannot supply facility current values or revision; canonical values are merged from the session owner.
- New devices initialize current state from authored initial state. Removed unreferenced devices remove their current state with the same facility graph revision.
- Unknown JSON fields on every new nested facility entity survive decode, mutation, and encode.
- Nil Facility remains nil for legacy sessions unless an explicit facility authoring action creates it.

## Reset Semantics

A single-device reset changes only the selected device's current state and conditions directly scoped to that device. Terminal-scoped conditions remain unchanged. A whole-facility reset restores all device and condition current values. Both operations compare an expected facility revision, invalidate older pending world actions on success, and use the same one-write mutation primitive. A no-op reset returns the canonical session with Changed false and does not advance any revision.
