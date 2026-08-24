# Phase 0 Research: Terminal Grouping

## Ordered groups are the only high-level terminal representation

**Decision**: Add a session-level `TerminalGroup` with stable ID, display name, and ordered terminal IDs. Every current terminal appears exactly once across the complete group list; a standalone terminal appears in a singleton group.

**Rationale**: One session-level collection gives membership and traversal order a single durable source of truth and directly represents groups as the Overseer's top-level organization. It avoids conflicting per-terminal membership fields and makes exact-one validation a complete-document invariant.

**Alternatives considered**: A `groupId` and numeric order on every terminal would duplicate group identity and make renames awkward. Deriving groups from authored transition links would conflict with explicit Overseer ownership and the no-auto-group requirement. Allowing ungrouped terminals or empty groups would contradict the feature model.

## Legacy sessions normalize to deterministic singleton groups

**Decision**: When a version-1 session has terminals but no `terminalGroups`, normalize the active in-memory document to one deterministic singleton group per terminal without inferring relationships from transition commands. Preserve terminal content and unknown JSON fields; persist the explicit normalized groups on the next accepted save.

**Rationale**: Runtime and UI code can enforce exact-one membership without carrying a second “legacy ungrouped” mode. Deterministic IDs and collision-safe names keep the normalized view stable across repeated opens before the first save, while deferring the write preserves explicit user-file ownership.

**Alternatives considered**: Keeping missing groups as a permanent runtime state breaks exact-one membership and complicates every lookup. Auto-coupling linked terminals changes author intent. Rewriting the file immediately on open would silently mutate user data.

## Terminal lifecycle preserves exact-one membership atomically

**Decision**: Existing terminal creation/import and deletion flows normalize groups in the same session mutation. A new or imported terminal receives a singleton group; deleting a terminal removes its membership and removes any now-empty group. Generic terminal/content saves preserve canonical group edits and synthesize only the required singleton representation for genuinely new terminals.

**Rationale**: There is no observable ungrouped state, and the existing terminal-delete confirmation remains the destructive confirmation for deleting terminal content. Group management cannot be bypassed by submitting a stale full-session payload.

**Alternatives considered**: Requiring a separate group command after terminal creation creates an invalid intermediate state. Trusting the generic save's full group list opens a stale-UI overwrite path. Retaining an empty group after terminal deletion violates the model.

## Deleting a group means dissolving its container

**Decision**: Group deletion removes only the group container. Unless the same confirmed candidate assigns them elsewhere, every former member becomes its own singleton group. Deleting a singleton group alone is rejected because it would produce no valid representation change.

**Rationale**: This gives the Overseer a meaningful Delete Group operation without coupling organizational cleanup to terminal-content deletion. The resulting candidate always satisfies exact-one membership and the no-empty-group rule.

**Alternatives considered**: Cascading terminal deletion is too destructive and violates the specification. Moving all members to an arbitrary destination invents intent. Leaving them ungrouped or retaining an empty shell violates the high-level group model.

## Destructive edits use one impact dialog and revision-guarded commit

**Decision**: The Overseer frontend builds the complete candidate group list and classifies changes by diffing it against the latest canonical groups. Creating a group from existing terminals, dissolving, merging, splitting, moving, and reordering open one confirmation dialog that lists affected groups, terminals, and navigation impact. Rename-only changes bypass the destructive dialog but use the same validated mutation path.

**Rationale**: The user reviews the exact atomic result rather than a sequence of partial operations. The repository already uses native browser dialogs and revision-bearing commands, so the UI can follow established focus, cancel, busy, error, and canonical-refresh patterns without a second editor protocol.

**Alternatives considered**: Confirming each individual drag/drop step produces dialog fatigue and can expose partial state. Confirming rename-only edits adds no safety for content or navigation. A generic browser `confirm()` cannot present structured impact or accessible stale feedback.

## The trusted mutation carries durable and runtime expectations

**Decision**: Replace the complete group set through one private request carrying `expected_session_revision` and `expected_coordination_revision`. The coordinator compares the runtime revision while serialized with player actions, then calls a synchronous session-owned compare-and-replace against the durable revision. Any mismatch or invariant failure returns the latest canonical session and coordination state without applying part of the proposal.

**Rationale**: Group validity depends on both durable authored links and broadcast-scoped pending/route state. After a successful destructive mutation both observable revisions advance, so a retry or double-submit with old expectations cannot repeat the change. Returning canonical state lets a stale UI refresh rather than guess what won.

**Alternatives considered**: A client-only check races player navigation. A session revision alone misses a newly pending decision or changed route. A server-stored preview token adds a second state machine without improving the compare-and-commit guarantee. A validate-then-autosave sequence leaves a race between validation and persistence.

## Group changes use one coordinator-guarded private capability

**Decision**: The coordinator owns runtime eligibility and serializes group replacement with navigation. It calls a narrow synchronous group store owned by `internal/session` while following the existing control-to-session lock direction. The root application routes the protobuf-defined private request/result and publishes canonical session and coordination updates only after durability succeeds.

**Rationale**: Player actions already enter the coordinator, while the session service owns durable documents and revisions. This division keeps Wails out of domain packages, prevents generic-save bypasses, and avoids reverse callbacks from persistence into runtime coordination.

**Alternatives considered**: Injecting the coordinator into the session service reverses ownership and lock direction. Freezing all broadcasts during every group edit rejects safe rename-only operations and unrelated edits. Exposing group management on the player service violates the private capability boundary.

## Middle-start support seeds a backward prefix

**Decision**: On the first successful terminal activation of a fresh broadcast, query its ordered group and seed the route with root-level return points for every preceding member, oldest first. For A, B, C, D started at C, the seeded stack is A then B, so backward approval visits B then A. Later authored forward transitions append their normal context-rich return points.

**Rationale**: This reuses the existing root Back action, pending decision, approval, route-depth, reconnect, and last-in-first-out machinery. It makes every earlier group member reachable without introducing optimistic client positions or a second public navigation command.

**Alternatives considered**: A new previous/next RPC duplicates existing navigation semantics. Treating C as an empty route makes A and B unreachable. Seeding every manual activation would violate the existing direct-activation route-clearing behavior.

## Seeded points carry internal provenance

**Decision**: Mark seeded return points with internal group ID and ordered position metadata; keep this runtime-only and outside session JSON. Approval and destructive group mutation revalidate that provenance against the latest catalog snapshot.

**Rationale**: Ordinary authored return points may jump between non-adjacent members and cannot be validated as adjacent group positions. Provenance distinguishes the initialized ordered prefix from later authored history and safely rejects a reorder that would change its meaning.

**Alternatives considered**: Inferring provenance from a blank command ID is fragile. Rejecting every membership or reorder change whenever any broadcast exists is safe but unnecessarily blocks changes unrelated to the active route.

## Existing public navigation contracts remain sufficient

**Decision**: Do not add a player RPC or expose group metadata publicly. Continue using the existing unary navigation action, `TerminalNavigationPresentation`, return target, pending direction, server revision, and stream reconnection behavior.

**Rationale**: Players need an available backward target and authoritative approval state, all of which are already represented. Group names, management controls, and ordering are private authoring data.

**Alternatives considered**: Publishing full groups leaks unnecessary private authoring structure and creates a second client-side eligibility authority. An explicit public group-position field is unnecessary for the required interaction and adds compatibility risk.

## The demo is explicitly grouped; legacy behavior stays in tests

**Decision**: Add an explicit ordered group to `sessions/demo.json` so its documented transition remains usable after grouping ships. Preserve missing-group compatibility with focused fixtures and tests rather than leaving the active demo dependent on normalization.

**Rationale**: The README presents the demo as the working transition example. Explicit grouping keeps that journey intentional without automatically coupling terminals in user sessions.

**Alternatives considered**: Leaving the demo implicit obscures the canonical format. Auto-grouping user sessions from links violates the feature's non-goal.
