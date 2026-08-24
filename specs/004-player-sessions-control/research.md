# Research: Player Sessions, Character Assignment, and Shared Terminal Control

> **SUPERSEDED LEGACY PLAYER TRANSPORT — HISTORICAL, NON-AUTHORITATIVE.**
> Any WebSocket or handwritten JSON player-transport description in this retained
> completed feature document has been replaced by the generated ConnectRPC contract in
> [`specs/005-connectrpc-protobuf-migration/contracts/public-player.md`](../005-connectrpc-protobuf-migration/contracts/public-player.md).

**Bugfix**: 2026-08-12 — BUG-001 Updated from bugfix patch

**Bugfix**: 2026-08-20 — BUG-008 Updated from bugfix patch

## Decision 1: Use one outer coordinator for every authoritative runtime transition

**Decision**: Add `internal/control.Service` as the single serialized owner of logical sessions, presence, roster, broadcast lifetime, assignments, controller identity, active-terminal selection, suspended terminal runtimes, player authorization, and ordered revision issuance. It delegates terminal mechanics to `internal/live` while committing detached publication effects through a non-reentrant callback before the coordinator mutex is released.

**Rationale**: Player sockets dispatch concurrently, Wails commands may run at the same time, and the current post-mutation `Publish*` calls can take a newer snapshot than the command that triggered them. One transaction boundary is the only direct way to make claims, actions, reassignments, and terminal switches share the unambiguous order required by FR-066 through FR-069.

**Alternatives considered**:

- Put roster and assignments in a separately locked service: rejected because authorization would span locks and action/reassignment races could observe inconsistent ownership.
- Expand `internal/live` to own connections and master coordination directly: rejected because it would mix terminal mechanics with transport membership and make the existing package too broad.
- Rely on timestamps or caller ordering: rejected because concurrent goroutines and WebSocket writers do not provide a canonical total order.

## Decision 2: Separate browser recognition from logical session identity

**Decision**: The server issues one cryptographically opaque `browserToken` during the initial `SESSION_HELLO`/`SESSION_WELCOME` exchange. The browser stores it in same-origin `localStorage` and serializes first-use handshakes under a cross-tab Web Locks guard with a storage-based fallback, so a later tab sends the first tab's stored token. The server maps that token to a newly generated process-local `sessionId`; only the mapping and logical session carry fallback name, presence, assignment, and role. An unknown token receives a fresh replacement token and session.

**Rationale**: `localStorage` survives ordinary refresh, navigation, reopen, and browser restart and is shared by tabs in one origin/profile while remaining isolated across profiles, private contexts, and cleared storage. Server issuance keeps token quality and replacement policy authoritative, while the separate logical identity means a token presented after application restart creates a new session and cannot restore any prior runtime state. Cross-tab handshake serialization prevents two simultaneously opened first tabs from receiving different tokens.

**Alternatives considered**:

- Use the raw WebSocket connection ID: rejected because refreshes and multiple tabs would become different sessions.
- Persist the server logical session in campaign JSON: rejected because process-local expiry is required and the version-1 schema must remain unchanged.
- Use only a server cookie: rejected because simultaneous first-page requests can race before a shared cookie exists, and JavaScript still needs an explicit testable handshake state before enabling selection or terminal input.
- Treat the browser token as an account or authentication credential: rejected because the feature adds neither accounts nor access control; the token is only a best-effort profile recognition handle.

## Decision 3: Model process, broadcast, and terminal lifetimes independently

**Decision**: Keep logical sessions and fallback names at process scope. ~~Keep roster entries at process scope.~~ BUG-001 supersedes that roster lifetime: load authored roster IDs/names from the active player config into the process runtime, while availability remains derived from the current broadcast. Put exclusive claims, controller identity, active terminal, and per-terminal runtime slots inside an opaque broadcast epoch. Ending a broadcast removes the entire broadcast aggregate but retains the process-scoped registry and loaded roster; starting another broadcast issues a fresh `broadcastId` and requires new claims.

**Rationale**: The requirements explicitly retain recognized devices and loaded roster configuration across broadcast endings while clearing assignments and control. BUG-001 additionally requires the authored roster to survive process restart through an explicit reload from durable configuration; this does not extend any logical-session or claim lifetime. A broadcast may also remain live with no active terminal, so terminal clearing cannot continue to mean broadcast termination.

**Alternatives considered**:

- Store assignments on terminals: rejected because assignments and controller authority must follow terminal switches.
- Clear logical sessions or roster on broadcast end: rejected because it would defeat same-process convenience and contradict the stated lifetimes.
- Reuse one numeric epoch after restart: rejected because stale client requests could collide; production identifiers must be opaque and process-unique.

## Decision 4: Preserve exact private terminal runtime and refresh authored content on reactivation

**Decision**: A preserved terminal retains its complete private `LiveState`, including secret hacking data, generation identity, board mutations, attempts, patterns, log, outcome, and navigation. When reactivated, the coordinator reapplies the latest validated authored name/tree/intro data and uses the existing navigation revalidation behavior while retaining the exact puzzle. Deleting an active or suspended unfinished terminal requires the same explicit discard decision as switching away.

**Rationale**: A public snapshot cannot reconstruct secret words, candidate lookup, generation-bound pattern history, or random-dependent state. Applying the latest authored content prevents a preserved runtime from reviving stale terminal copy, while navigation revalidation already provides the safe established merge rule. Governing deletion closes a path that could otherwise silently discard an unfinished puzzle.

**Alternatives considered**:

- Serialize a public puzzle snapshot and reconstruct later: rejected because it is incomplete and would expose or lose private canonical data.
- Freeze authored content together with the puzzle: rejected because game-master edits made while inactive would disappear on reactivation.
- Silently discard on terminal deletion: rejected because it bypasses the preserve/discard/cancel requirement.

## Decision 5: Correlate all shared actions and make state revisions explicit

**Decision**: Every `CHARACTER_SELECT`, `NAV_ACTION`, `HACK_GUESS`, and `HACK_PATTERN` request carries a client-generated `requestId`; claim and gameplay requests also carry the displayed `broadcastId`, and gameplay requests carry `terminalId`. Every request receives an `ACTION_RESULT` with its acceptance, stable reason, and committed revision. Canonical player envelopes carry revisions, and the client releases accepted pending input only after the required state at that revision has been applied.

**Rationale**: Rejected actions currently produce no response, so a client cannot end pending state reliably. Broadcast and terminal identifiers prevent delayed requests from being reinterpreted after lifecycle changes. Revision correlation lets the client distinguish an acknowledgement from the authoritative state that completes it and allows duplicate request IDs to replay a result without repeating mutation.

**Alternatives considered**:

- Clear pending on an animation timeout: rejected because latency and rejection are not animations and FR-072 explicitly requires an authoritative outcome.
- Treat any later snapshot as acknowledgement: rejected because unrelated game-master or player transitions can overtake the request.
- Add separate request types for controller and observer behavior: rejected because the same action vocabulary should remain stable and the server must authorize it at processing time.

## Decision 6: Use complete personalized player context plus shared terminal snapshots

**Decision**: Send `PLAYER_STATE` as the complete per-logical-session projection of fallback identity, character assignment, role, broadcast status, roster availability, and active-terminal identity. Continue to send canonical terminal content with the existing `TERMINAL_LIVE`, `TERMINAL_UPDATE`, `TERMINAL_CLEAR`, `NAV_STATE`, and `HACK_STATE` families, now revisioned. Send player-state changes to every connection of the same logical session; send the initiating connection its `ACTION_RESULT`.

**Rationale**: Assignment and controller status are personalized, while terminal content is shared. A complete player context avoids partially ordered role/roster messages and gives selection, waiting, observer, and controller rendering one authoritative source. Retaining the existing terminal message families minimizes change to stable navigation and hacking behavior.

**Alternatives considered**:

- Broadcast the game-master session list to every player: rejected because it would disclose fallback and connection information that players do not need.
- Encode role into every terminal snapshot only: rejected because unassigned and no-terminal clients also need an authoritative state.
- Add many small roster/role/presence deltas: rejected because reconnect and ordering recovery are simpler with one complete detached context.

## Decision 7: Keep one master coordination snapshot behind the narrow Wails facade

**Decision**: Add one `coordination-state` Wails event with the matching `coordinationState` field in `GetRuntimeStatus`, plus explicit validated App commands for roster, logical-session labels, assignments, controller, broadcast lifecycle, and terminal-switch decisions. The frontend awaits command results and renders the returned/eventual authoritative snapshot rather than mutating `state.liveTerminalId` optimistically.

**Rationale**: The existing event-plus-runtime-status pattern prevents subscription races. A complete master snapshot keeps roster, sessions, presence, claims, controller, active terminal, and pending switch at one revision, while explicit commands preserve the current narrow privileged boundary and make refusal cases visible.

**Alternatives considered**:

- Let the master browser own roster and role state: rejected because concurrent player selection and disconnect changes would diverge.
- Emit one independent Wails event per field: rejected because the UI could render a mixed revision.
- Expose the coordinator directly through generated bindings: rejected because `app.go` must remain the validation and privilege boundary.

## Decision 8: Add no runtime dependency

**Decision**: Implement identifiers with the standard library and extend the existing Go tests, asset-source contracts, and already pinned Playwright suite. Broaden the Playwright configuration to include the new browser specification without changing its pinned dependency.

**Rationale**: The repository already has cryptographic opaque-ID generation, deterministic fakes, strict protocol tests, real WebSocket integration tests, and browser interaction infrastructure. No external state store, framework, or synchronization library is necessary for process-local coordination.

**Alternatives considered**:

- ~~Add a database or durable session store: rejected because all new state intentionally expires at process or broadcast boundaries.~~ Add a database or general durable runtime store: rejected because browser recognition, presence, claims, controller, broadcasts, and puzzles still intentionally expire at process or broadcast boundaries. BUG-001 uses only ordinary JSON filesystem persistence for the authored roster.
- Add a frontend state framework: rejected because the established master and player surfaces use small plain-DOM render functions.
- Add a second browser-testing tool: rejected because Playwright already covers executable interaction and multi-page contexts.

## Decision 9: Persist the authored roster in a separate player-config file

**Decision**: Store reusable roster IDs and names in a separate version-1 JSON player config. Add only an optional normalized relative `playerConfig` reference to version-1 session JSON. After session creation/opening, automatically load that reference or offer native select-existing/create-new actions. Player-config selection or replacement is allowed only without an active broadcast, and successful roster CRUD atomically saves the complete candidate roster before the coordinator publishes it.

**Rationale**: Authored character names are repeatable campaign setup, unlike browser tokens, logical sessions, presence, claims, controller identity, and puzzles. A separate file lets one roster be reused by multiple terminal sessions, preserves the existing terminal-session body, and keeps the privacy and lifetime boundary explicit. A relative session reference makes the pair portable when moved together. Saving before publication prevents the UI and connected players from observing a roster revision that was not durably recorded.

**Alternatives considered**:

- Embed the full roster in session JSON: rejected because it duplicates a roster when several terminal configurations use the same player group and makes reuse require copying or merging session files.
- Prompt for a player config on every open without remembering the selection: rejected because it removes name re-entry but does not restore the session-to-roster association requested by BUG-001.
- Store an absolute player-config path in session JSON: rejected because it breaks when the session directory is moved or shared to another machine.
- Persist logical sessions, claims, controller state, or browser tokens beside the roster: rejected because those values retain their existing process/broadcast lifetimes and would create stale ownership and privacy risks.
- Add a database or background file watcher: rejected as unnecessary for the explicit open/save workflow; concurrent multi-process editing remains outside this bugfix.

## Decision 10: Make semantic terminal presentation controller-owned

**Decision**: Under BUG-008, store the active terminal's semantic menu selection, information-page ordinal, or hacking-preview target in its process-local runtime and change it only through a typed generated ConnectRPC mutation authorized for the current connected controller. Publish the complete semantic presentation in the live-terminal snapshot and applicable compound updates so observers, new tabs, late joiners, and reconnects render the controller's current view. Raw pointer coordinates, DOM focus, viewport geometry, reveal progress, audio eligibility, and playback state remain per-document and non-canonical.

**Rationale**: Gating outbound navigation/hacking requests does not make an observer a read-only broadcast when its local pointer or keyboard can still move the highlighted row or preview. Stable semantic targets can be validated against the current content/navigation/puzzle, ordered with reassignment, recovered by snapshot, and rendered across different viewport geometries without exposing browser implementation details.

**Alternatives considered**:

- Leave selection local and only disable activation: rejected because observers can still diverge visually from the controller and therefore are not watching the same terminal presentation.
- Disable observer input but keep the controller's selection local: rejected because observers would freeze on a default or stale highlight instead of following controller movement and reconnect snapshots could not restore the current view.
- Stream raw pointer coordinates or DOM focus: rejected because those values are layout-dependent, noisy, and not stable across responsive clients; only the semantic terminal target/page belongs in shared state.
- Transfer presentation state between browser sessions during reassignment: rejected because the state belongs to the active terminal runtime. Reassignment changes authority atomically while leaving that runtime state unchanged.
- Reintroduce handwritten WebSocket messages: rejected because the active public boundary is generated protobuf/ConnectRPC and feature 005 removed the legacy route and client.
