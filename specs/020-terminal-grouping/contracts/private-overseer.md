# Contract: Private Overseer Group Management

## Capability boundary

Terminal group management is exposed only by the registered private desktop service. It is not added to the public player Connect service, static asset routes, or player schemas.

## Request

`fallout.terminal.private.v1` adds one complete-set request:

```proto
message ReplaceTerminalGroupsRequest {
  repeated fallout.terminal.persistence.v1.TerminalGroup terminal_groups = 1;
  uint64 expected_session_revision = 2;
  uint64 expected_coordination_revision = 3;
}
```

The desktop method accepts the desired canonical group list as one candidate. Create, dissolve, merge, split, reorder, and terminal moves are expressed by the diff from the current canonical list. Rename-only changes use the same request. There are no independently persisted partial group operations.

The backend derives whether membership or traversal order changed; it does not trust a client-provided “destructive” flag.

## Result

`fallout.terminal.private.v1` adds a result that returns both authoritative state owners:

```proto
message ReplaceTerminalGroupsResult {
  bool ok = 1;
  optional string error = 2;
  uint64 session_revision = 3;
  fallout.terminal.persistence.v1.Session session = 4;
  CoordinationState coordination_state = 5;
}
```

- On success, `session_revision`, `session`, and `coordination_state` are the complete canonical values after the atomic mutation.
- On rejection, `error` is safe and actionable; the latest canonical session and coordination state are returned when available.
- A stale, retried, or double-submitted request cannot apply after either expected revision has advanced.
- The Overseer always replaces its draft with the returned or subsequently published canonical state and never infers success from optimistic local mutation.

## Confirmation contract

Before invoking the method for a destructive candidate, the Overseer UI must show a modal impact summary containing:

- the operation category;
- every affected group name;
- every terminal whose membership or traversal position changes;
- source and destination groups for moves; and
- the before/after traversal-order consequence.

Create-from-existing, dissolve, merge, split, move, and reorder are destructive. Cancel, close, and Escape discard the proposal and call no mutation. Rename-only changes do not show this destructive confirmation when identity, membership, and order are unchanged.

During submission, confirm and cancel controls are disabled to prevent local duplicate dispatch. Backend revision checks remain authoritative even if a retry bypasses that UI guard. On stale rejection, the dialog closes or refreshes to the latest canonical state and requires the Overseer to review a newly prepared proposal.

## Ordering and atomicity

1. The coordinator serializes the request with player navigation and verifies `expected_coordination_revision`.
2. It derives the group diff and validates pending decisions, the active terminal, active route pairs, and seeded group provenance against the complete candidate.
3. It invokes the session-owned synchronous compare-and-replace using `expected_session_revision` in the established control-to-session lock order.
4. The session service validates group references, exact-one membership, authored links, and terminal lifecycle normalization while preserving command states and compatible extras.
5. Persistence replaces the document atomically.
6. Only after durability succeeds does the coordinator advance its revision and the application publish canonical session and coordination events.

Generic `SaveSession` requests retain canonical group state for existing terminals and therefore cannot bypass this capability.

## Rejection categories

The trusted UI receives a non-secret explanation for at least:

- blank or duplicate normalized group name;
- duplicate group ID;
- empty group;
- missing, repeated, or multiply assigned terminal;
- a terminal missing from the complete candidate;
- attempted standalone dissolution of a singleton group;
- authored transition with endpoints outside one shared group;
- pending forward/return decision invalidated by the candidate;
- active route or seeded order invalidated by the candidate;
- stale session revision;
- stale coordination revision; and
- persistence failure.

No filesystem path, player credential, or private runtime identifier not already visible to the Overseer is included.

