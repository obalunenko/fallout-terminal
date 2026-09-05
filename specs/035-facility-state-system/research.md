# Phase 0 Research: Shared Facility State System

## Decision 1: Keep facility authority in the version-1 session

**Decision**: Add one optional session-wide facility aggregate to the existing version-1 session document. It contains authored definitions, current device and condition values, and a persistent facility revision. A missing aggregate means the session has no configured facility.

**Rationale**: The session service already owns durable campaign state, preserves compatible unknown fields, and writes one complete JSON document atomically. Keeping facility state there makes restart, self-update, session reload, and backup behavior follow an established authority.

**Alternatives considered**: A separate facility file would create cross-file atomicity and relocation problems. Runtime logs cannot provide authoritative state. Terminal-local copies would violate facility-wide identity and group-move requirements.

## Decision 2: Preserve three distinct revisions

**Decision**: Keep the document revision, persistent facility revision, and process coordination revision separate. Every committed facility graph or world-state mutation increments the facility revision once; the document revision identifies the containing session save; the coordination revision orders live publication.

**Rationale**: Each revision protects a different boundary. A persisted facility revision survives process restart and invalidates stale approvals, while existing document and coordination revisions continue to protect authoring and streaming order.

**Alternatives considered**: Reusing the process revision would lose conflict identity on restart. Reusing the document revision would make unrelated session saves invalidate world actions and would expose internal save scheduling as gameplay state.

## Decision 3: Extend state-changing commands with a composable facility action

**Decision**: Add an optional facility action to the existing state-changing command configuration. The command continues to create its immutable completion snapshot, while the same approved execution may also request one or more device transitions. Ordinary and terminal-transition command variants remain unchanged and exclusive under the current behavior model.

**Rationale**: This directly builds on the established one-command completion lifecycle and private approval UI. It allows command completion and facility mutation to be saved in one document without adding another command or text-state mechanism.

**Alternatives considered**: A new command behavior would duplicate approval and result handling. Attaching facility actions to every command variant would complicate terminal-navigation ordering and weaken the current oneof invariant. Treating device projection as a command snapshot would make shared state terminal-local and one-use.

## Decision 4: Commit command completion and facility state through one store operation

**Decision**: Replace the two-step possibility with one narrow world-action store call. It builds one candidate session containing the optional command snapshot, all device destinations, all condition effects, and one new facility revision; validates the entire candidate; performs one atomic replacement; and returns a detached canonical result.

**Rationale**: The existing command-state mutation already has the required lock order, rollback-on-failure behavior, and write-before-publication rule. Generalizing that path is the only way to guarantee no split state when command completion and several devices change together.

**Alternatives considered**: Calling the existing command mutation and a new facility mutation sequentially could persist only half of the world action. Compensating writes would still expose intermediate durable revisions and could fail independently.

## Decision 5: Validate all transition inputs against one pre-state

**Decision**: A world action may contain at most one transition per device. Approval resolves every transition ID, required source state, equality precondition, and condition effect against the same canonical pre-state before applying any destination. Preconditions form an AND-list of typed device-state equalities.

**Rationale**: Simultaneous evaluation makes the result independent of request ordering and prevents hidden expression semantics. A single global facility revision makes all requests created from an older world snapshot stale, including otherwise disjoint actions.

**Alternatives considered**: Applying transitions in list order would make cycles and cross-device preconditions ambiguous. Per-device revisions would permit more concurrency but complicate multi-device conflict proofs and retained evidence without practical benefit under the existing single pending approval lifecycle.

## Decision 6: Use one pure facility evaluator for projection and authorization

**Decision**: Extend the live service's detached effective-tree projection with a pure evaluator that consumes authored content, immutable command snapshots, facility state, and diagnostic conditions. The coordinator uses the same evaluator for command availability, visibility, capability blocks, navigation repair, and player projection.

**Rationale**: One evaluator prevents the browser from showing a disabled command that the server accepts, or hiding content while stale navigation still reaches it. It also preserves the current rule that browsers render authoritative snapshots rather than mutate world state locally.

**Alternatives considered**: Frontend-only conditions would be advisory and bypassable. Separate authoring and runtime evaluators would drift. Persisting effective text would introduce the prohibited parallel text-state mechanism.

## Decision 7: Define deterministic presentation precedence

**Decision**: Resolve each presentation property in this order: authored base value, legacy completed-command snapshot, matching device-state binding, then an active diagnostic-condition override. A facility binding controls one device and has at most one variant per state. Authoring rejects multiple condition effects that could control the same property at once.

**Rationale**: The order keeps legacy completion durable and visible when no newer facility rule applies, makes explicitly bound facility state authoritative, and lets an active authored fault predictably alter the current presentation. Conflict rejection avoids hidden map or list ordering.

**Alternatives considered**: Last-edited or list-order precedence would be fragile. Letting multiple devices or conditions compete would require priorities or a general expression language. Overwriting command snapshots would destroy legacy semantics and reset behavior.

## Decision 8: Keep raw facility state private and extend only necessary player fields

**Decision**: Continue streaming a fully resolved content tree. Add an optional command-availability field whose absence means available, plus a bounded safe terminal-effect enum for presentation-only effects such as display instability. Do not expose device graphs, conditions, recovery definitions, authored alternatives, or private structured failures to players.

**Rationale**: Visibility and text variants can be resolved by the server, but availability must remain visually distinct from invisibility and display instability needs a safe rendering signal. The player has no need for authoring identities or world-state authority.

**Alternatives considered**: Sending the complete facility would enlarge the public attack surface and duplicate evaluation in JavaScript. Removing unavailable commands would erase the requested distinction between availability and visibility. Encoding effects as free-form strings would weaken validation.

## Decision 9: Model diagnostic conditions as bounded facility definitions

**Decision**: Store diagnostic conditions inside the facility aggregate. Each has a stable ID, category, device-or-terminal scope, authored initial/current active status, and typed effects chosen from capability block, diagnostic path exposure, authored record substitution, and display instability. Device transitions may activate or clear named conditions.

**Rationale**: Conditions then share the same validation, revision, atomic persistence, reset, and projection model as devices. Custom faults can vary their authored name and selected effects without adding executable behavior.

**Alternatives considered**: Random fault generation conflicts with deterministic sessions. A separate diagnostics state store would split authority. Free-form effect expressions would become the excluded scripting language.

## Decision 10: Add bounded holotape recovery definitions without a runtime interpreter

**Decision**: Introduce authored recovery-program definitions with stable IDs and allowlisted recovery action references. A state-changing command may invoke one such program through the existing approval lifecycle; invocation expands only to its referenced authored transitions and condition effects.

**Rationale**: The repository has no holotape or program subsystem. A bounded selector satisfies compatible recovery without inventing script execution, character checks, inventory simulation, or another approval path.

**Alternatives considered**: A script engine is explicitly out of scope. Deferring all program support would leave a stated recovery path unimplemented. Treating a program name as trusted executable input would violate private-boundary validation.

## Decision 11: Protect facility state from browser-authored saves

**Decision**: Add a revision-aware private facility-authoring operation for complete validated graph changes and multi-reference repair. Ordinary session saves preserve canonical facility current values and revision just as they preserve canonical command snapshots. Facility authoring is serialized through control to the session store, reprojects active content after durability, and invalidates pending actions when the graph changes.

**Rationale**: The current browser sends a complete session candidate, so a stale authoring view could otherwise overwrite a player transition. A dedicated typed boundary distinguishes definitions from protected current world state and provides structured dependency results.

**Alternatives considered**: Trusting the browser's current state would violate server authority. Parsing string errors from the generic save path would not support repair. Blocking all authoring during broadcast would be safe but unnecessarily restrictive.

## Decision 12: Use typed results privately and the existing access-error presentation publicly

**Decision**: Define facility failure enums for missing reference, invalid transition, failed precondition, stale revision, conflict, duplicate, persistence failure, and rejection. Private results and retained audit records carry safe IDs and correlation data. Player-facing rejected or failed facility commands enter the existing rejected command presentation and authoritative Back flow.

**Rationale**: Stable categories support UI, tests, and diagnostics without message matching, while preserving the current player experience and public/private capability separation.

**Alternatives considered**: New player error pages would duplicate navigation behavior. Raw error strings would be unstable and could leak dependency details. Logging full actions or content would violate redaction requirements.

## Decision 13: Hydrate and replace facility state explicitly on session lifecycle changes

**Decision**: Successful new, open, copy, and reload operations install the loaded facility into the coordinator before player interaction resumes and invalidate pending approvals from the previous document. Facility state remains outside broadcast-owned data, so broadcast stop cannot erase it.

**Rationale**: The current session service changes document epochs independently of broadcast runtime. An explicit handoff prevents an old approval from targeting a new document with coincident IDs and ensures reconnect snapshots use the loaded facility.

**Alternatives considered**: Lazy hydration risks a window with stale projections. Keeping facility state inside the broadcast would lose it on stop. Matching only terminal and command IDs would not distinguish session replacement.

## Decision 14: Treat publication as convergence after durable success

**Decision**: After a successful durable world commit, the coordinator installs the canonical facility, repairs navigation against the effective tree, advances one coordination revision, and publishes complete projections. A delivery failure does not roll back disk; reconnect and later snapshots converge from the saved session.

**Rationale**: Existing stream revisions and full snapshots already provide ordered convergence. Once the atomic session replacement succeeds, a compensating rollback would create a second failure-prone write and could make disk older than emitted evidence.

**Alternatives considered**: Holding the durable commit hostage to every client delivery is impossible with disconnected observers. Treating logs as a replay queue would create a second source of world state.
