# Data Model: Terminal Grouping

## Durable entities

### TerminalGroup

| Field | Shape | Rules |
|---|---|---|
| `id` | required string | Stable within the session, trimmed, bounded like other authored IDs, and unique across groups. |
| `name` | required string | Nonblank and bounded; unique after trimming and case folding. Renaming does not change `id`. |
| `terminalIds` | non-empty ordered array of strings | Every value references one current terminal; a terminal appears exactly once across the complete group set. Array order is traversal order. |

`Session.terminalGroups` is an additive top-level field in portable JSON v1 and a repeated field in the persistence contract. In the canonical active document, the array is empty only when the session has no terminals. Group array order controls high-level presentation; member order inside each group is gameplay-significant.

### Singleton TerminalGroup

A singleton group is an ordinary `TerminalGroup` with exactly one member. It is the required representation of a standalone terminal, not a separate persisted type. New or imported terminals receive singleton groups atomically. Dissolving a multi-terminal group creates collision-safe singleton groups for members not assigned elsewhere by the same candidate.

### Legacy normalization marker

Field absence is observed only during JSON decoding. If a session contains terminals but has no group array or has an explicitly empty group array, the loader creates deterministic singleton groups in the active document without writing the source file. The marker itself is not persisted; the next explicit save writes the canonical groups. A partially populated, duplicated, or malformed non-empty group array is rejected rather than silently repaired.

### TerminalMembershipIndex

A derived, non-persisted index is built from the complete canonical session:

| Value | Meaning |
|---|---|
| terminal ID → group ID | Enforces exact-one membership. |
| terminal ID → member position | Provides deterministic predecessor/successor and seeded-route validation. |
| group ID → ordered terminal IDs | Supports detached snapshots and whole-set mutation validation. |

No terminal stores an independent group ID, so persistence has one membership source of truth.

## Group-change entities

### TerminalGroupCandidate

The private mutation carries the complete desired group set plus two optimistic-concurrency expectations:

| Field | Meaning |
|---|---|
| groups | Complete non-empty group records for every surviving terminal. |
| expected session revision | Durable session revision visible when the proposal was prepared. |
| expected coordination revision | Runtime coordination revision visible when the proposal was prepared. |

The candidate is never applied incrementally. The backend derives the semantic diff from canonical groups rather than trusting a client-provided change classification.

### DestructiveGroupChangeImpact

An ephemeral Overseer UI model prepared from canonical groups and one candidate:

| Field | Meaning |
|---|---|
| kind | create-from-existing, dissolve, merge, split, move, or reorder. |
| affected groups | Source, destination, created, renamed, or removed group identities and names. |
| affected terminals | Terminal identities and names whose membership or traversal position changes. |
| before/after order | The navigation-order consequence shown in the confirmation. |
| captured revisions | Session and coordination revisions sent if the Overseer confirms. |
| resolution | open, canceled, submitting, accepted, or rejected; a resolved proposal is never resubmitted by the UI. |

Rename-only changes are not destructive when stable identity, membership, and order are unchanged. The backend still validates and derives that distinction independently.

### TerminalGroupMutationResult

The private result contains success/failure, safe feedback, the latest session revision and canonical session, and the latest coordination state. Success replaces the frontend draft with canonical state. Failure also returns canonical state when available so stale proposals can be discarded without guessing.

## Runtime entities

### TerminalGroupSnapshot

A detached catalog value contains group ID, group name, ordered member IDs, and requested `TerminalTarget` values from one locked session snapshot. It is used for fresh-broadcast initialization, same-group link lookup, return approval, and group mutation validation; consumers cannot mutate session-owned values.

### TerminalReturnPoint additions

Existing authored return points retain terminal/folder/command context. Seeded points add runtime-only provenance:

| Field | Shape | Meaning |
|---|---|---|
| origin | enum | authored transition or initial group prefix; zero/legacy runtime values normalize to authored behavior. |
| group ID | optional string | Present only for a seeded prefix point. |
| group position | non-negative integer | Member position captured for a seeded point. |

Seeded points restore the target terminal at its root and carry no fabricated command identity. This metadata never enters session JSON or a player request.

### LiveBroadcast addition

| Field | Shape | Meaning |
|---|---|---|
| initial terminal established | boolean | Becomes true after the first successful terminal activation of the broadcast. It prevents later manual activations from reseeding the route. |

The route remains broadcast-scoped and last-in-first-out. For A, B, C, D first activated at C, the initial route is `[A(seed 0), B(seed 1)]`; B is the public return target.

## Validation rules

### Compatible document loading

1. Session version remains exactly 1 and existing terminal/content validation still applies.
2. A missing or empty group array with terminals is treated as a legacy document and normalized to one singleton group per terminal in memory.
3. If any explicit group is present, every group has a unique stable ID, a normalized unique name, and at least one member.
4. Every explicit member references an existing terminal, and every terminal appears exactly once across the complete explicit group set.
5. Existing transition commands still require existing, non-self targets.
6. Normalization never couples terminals based on transition commands; legacy cross-singleton links remain authored but runtime-ineligible.
7. Compatible unknown JSON fields remain attached to their owning session, terminal, and content nodes.

### Generic terminal/content save

1. Canonical membership and group order for existing terminals cannot be replaced by a stale full-session payload.
2. Each genuinely new or imported terminal receives exactly one collision-safe singleton group in the same accepted save.
3. Deleted terminal IDs are removed from membership and any resulting empty group is removed in the same accepted save.
4. The resulting complete session passes exact-one, no-empty-group, and compatible authored-reference validation before persistence; an already-authored cross-group legacy link remains dormant rather than blocking unrelated content saves.

### Trusted group mutation

1. The submitted session and coordination expectations must equal the current authoritative revisions.
2. The complete candidate passes group identity, normalized-name, non-empty membership, reference, exact-one, and authored-link rules.
3. No candidate splits the endpoints of an authored terminal transition.
4. No candidate invalidates a pending forward/return decision or any active route pair.
5. A seeded route point retains its group identity, target, successor relationship, and relative ordered position.
6. Dissolving a multi-terminal group leaves all its terminals represented, while dissolving a singleton alone is rejected as a no-op that would imply an ungrouped terminal.
7. Any mismatch or failure identifies affected items and applies no durable or runtime fragment.

## State transitions

### Destructive group authoring

```text
canonical groups + current revisions
  → build complete candidate and impact summary
  → open confirmation (no state mutation)
  → cancel/close: discard proposal
  → confirm: submit candidate + captured revisions once
  → coordinator revision check and runtime guard
  → synchronous session compare-and-replace
  → commit one durable revision and one coordination revision
  → publish and replace draft with canonical result
```

Stale, retried, or double-submitted requests retain their old expectations and therefore return canonical state with no additional mutation.

### Rename-only authoring

```text
canonical group + valid unique name
  → complete candidate with unchanged identity/membership/order
  → submit through the same revisioned mutation without destructive dialog
  → atomic validation, durability, and canonical refresh
```

### Group dissolution

```text
multi-terminal group selected
  → candidate removes container
  → each otherwise-unassigned member receives a singleton group
  → impact dialog lists removed group and resulting groups
  → confirmed candidate follows destructive authoring transition
```

### First terminal activation

```text
fresh broadcast, initial terminal not established
  → activate target successfully
  → lookup current ordered group
  → append root return points for members before target
  → mark initial terminal established
  → publish active terminal + route target at one coordinator revision
```

A first-position or singleton target produces an empty route but still marks initialization complete.

### Forward transition

```text
controller selects authored command
  → catalog proves source and target share a current group
  → one pending Overseer decision
  → approval revalidates link + shared group
  → append authored return point and activate target
```

Reject or close changes neither terminal nor route.

### Backward transition

```text
controller selects Back at terminal root with non-empty route
  → one pending Overseer decision for route top
  → approval revalidates route top + shared group
  → seeded point additionally revalidates group provenance/order
  → pop one point and activate target
```

Reject or close leaves the route top and active terminal unchanged.

### Manual activation

A later direct Overseer activation keeps the existing behavior: pending navigation and the route are cleared, the requested terminal is activated through the terminal-switch lifecycle, and no group prefix is seeded again.

## Clone and compatibility obligations

- `CloneSession` deep-copies groups and member ID slices.
- Catalog and mutation results deep-copy group/member values and terminal content.
- Compatible unknown JSON fields remain attached to their owning session, terminal, and content nodes.
- Runtime provenance, route, pending decisions, confirmation impact, and initialization flags are never serialized.
- Existing session protobuf fields 1–4 and every existing field name/number remain unchanged.
