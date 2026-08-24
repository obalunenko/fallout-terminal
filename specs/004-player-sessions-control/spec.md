# Feature Specification: Player Sessions, Character Assignment, and Shared Terminal Control

> **SUPERSEDED LEGACY PLAYER TRANSPORT — HISTORICAL, NON-AUTHORITATIVE.**
> Any WebSocket or handwritten JSON player-transport description in this retained
> completed feature document has been replaced by the generated ConnectRPC contract in
> [`specs/005-connectrpc-protobuf-migration/contracts/public-player.md`](../005-connectrpc-protobuf-migration/contracts/public-player.md).

**Feature directory**: `004-player-sessions-control`  
**Scope**: Durable authored roster configuration; process-local browser sessions; broadcast-scoped character assignments; exclusive player control; observer behavior; game-master coordination; and active-terminal switching

**Bugfix**: 2026-08-12 — BUG-001 separates reusable authored roster configuration from transient player-session state and adds a durable player-config workflow.

**Bugfix**: 2026-08-12 — BUG-002 clarifies that a newly created empty roster remains the required JSON array through creation, association, and coordinator installation.

**Bugfix**: 2026-08-12 — BUG-003 clarifies that the active controller must remain able to select every hacking target category through the fully composed browser, WebSocket, coordinator, and runtime path.

**Bugfix**: 2026-08-12 — BUG-004 clarifies that the visible game-master end-broadcast control must complete the fully composed master-to-player shutdown path.

**Bugfix**: 2026-08-12 — BUG-005 restores a trusted same-terminal retry after hacking lockout without ending the broadcast or clearing player assignments.

**Bugfix**: 2026-08-12 — BUG-006 clarifies that authoritative wrong-guess, unlock, and lockout transitions retain their established one-shot player audio feedback.

**Bugfix**: 2026-08-12 — BUG-007 clarifies that the password-hacking screen must retain its complete established interaction and outcome sound set, not only CRT ambience and character-scroll audio.

**Bugfix**: 2026-08-20 — BUG-008 replaces observer-local terminal selection with controller-owned authoritative presentation that observers mirror and controller reassignment transfers atomically.

**Bugfix**: 2026-08-21 — BUG-009 separates non-blocking controller-presentation request correlation from genuinely locked observer and shared-gameplay pending presentation.

**Bugfix**: 2026-08-21 — BUG-010 bounds rapid controller presentation dispatch so superseded hacking-hover targets cannot build a stale highlight, animation, or preview-audio backlog.

## User Scenarios & Testing

### User Story 1 - Join as a Character (Priority: P1)

As a player opening the shared link, I select one available game-master-defined character and enter the current broadcast with a stable identity, so the table knows whom my device represents.

**Why this priority**: Character assignment is the eligibility boundary for every other player-facing capability in this feature.

**Independent Test**: Start a broadcast with a two-character roster, connect one new browser profile, select a character, and verify that the same character becomes claimed and the player proceeds to the current terminal.

**Acceptance Scenarios**:

1. **Given** a browser profile is unknown to the running server, **when** it opens the shared player link, **then** the server establishes one logical session with a unique fallback name.
2. **Given** the logical session has no valid assignment for the current broadcast, **when** the player connects, **then** an immersive terminal-styled selection screen shows the current roster with available and claimed characters distinguishable.
3. **Given** an available character, **when** the player selects it, **then** the server claims it for that session and the player proceeds to the active terminal or the waiting state when no terminal is active.
4. **Given** two sessions select the same available character concurrently, **when** the claims are ordered, **then** exactly one succeeds and the other remains on selection with the updated roster.
5. **Given** two unassigned sessions select different characters concurrently while no controller exists, **when** both claims complete, **then** exactly one becomes active controller and the other becomes an observer.
6. **Given** all roster characters are claimed, **when** another unassigned session connects, **then** it remains on selection or waiting for game-master intervention and cannot enter the terminal.

---

### User Story 2 - Control One Shared Terminal (Priority: P1)

As the active character, I can operate the terminal while every other assigned character observes the same authoritative screen, so one physical-table participant leads without splitting the shared experience.

**Why this priority**: Exclusive control is the central gameplay value and security boundary of the feature.

**Independent Test**: Connect one active session and two observers, exercise every existing player action from each, and verify that only active-session actions can change the canonical terminal.

**Acceptance Scenarios**:

1. **Given** a connected, character-assigned active session, **when** it submits a valid player terminal action, **then** the action is evaluated under all existing terminal and hacking rules and its authoritative result is shown to every connected assigned session.
2. **Given** an observer, **when** it submits any navigation, password, filler, or special-pattern action, **then** the action is rejected without changing canonical state, attempts, randomness, logs, or outcome.
3. **Given** an unassigned, unknown, expired, stale, or invalid session, **when** it submits any player action, **then** the action is rejected without canonical mutation.
4. ~~**Given** an observer is viewing the shared terminal, **when** it hovers, focuses, or uses a client-local control, **then** passive local feedback may change but canonical state does not.~~ Superseded by BUG-008: **Given** an observer is viewing the shared terminal, **when** it moves its pointer, changes browser focus, presses navigation or paging keys, or activates a terminal control, **then** its displayed terminal selection, highlight, page, and preview remain unchanged by that local input and continue to mirror the current controller-owned authoritative presentation.
5. **Given** the active session submits an action, **when** the result is pending, **then** shared player input remains pending until an authoritative outcome is applied and no optimistic shared mutation appears.
6. **Given** a player surface or crafted player request, **when** it attempts to invoke `ForceHackSuccess`, **then** no player operation is available and the game-master operation remains private.
7. **Given** the connected assigned active session is viewing an unfinished hacking puzzle, **when** it selects a password candidate, filler character, or unused special pattern, **then** exactly one matching shared hacking request is processed and the authoritative result releases pending input while every assigned view converges.
8. **Given** an audio-enabled connected assigned player has already applied the current unfinished hacking state, **when** it applies one newer authoritative state containing an ordinary wrong guess, a newly solved password, or the final wrong guess that locks the terminal, **then** it invokes exactly one bad, good/unlock, or bad/lockout cue respectively without invoking an outcome cue from the local click alone.
9. **Given** an audio-enabled assigned player is viewing an unfinished hacking puzzle, **when** it newly previews an individual filler target, newly previews a password candidate or unused special pattern, or selects an actionable hacking target, **then** it invokes exactly one established single-character, grouped-target, or entry cue respectively as client-local feedback while the established ambient and character-scroll sounds remain available.
10. **Given** the active controller moves the semantic terminal selection by pointer or keyboard, changes an information page, or previews an actionable hacking target, **when** the server accepts that presentation action, **then** every connected assigned session displays the same resulting controller-owned presentation in authoritative order without an optimistic observer-local change.
11. **Given** an active controller changes semantic presentation, **when** its generated `SetPresentation` request is in flight, **then** the controller retains its normal actionable appearance and cursor without grey/desaturated or locked-state flicker, may submit a newer presentation movement, and still applies semantic selection only from an authoritative projection.
12. **Given** an active controller rapidly moves across many hacking targets while `SetPresentation` is delayed, **when** newer semantic targets supersede earlier unsent intent, **then** the client keeps at most one request in flight and one replaceable latest desired target, dispatches no intermediate queued targets, and converges every assigned view on the final still-applicable target without replaying stale selected-target animation or preview audio.

---

### User Story 3 - Reuse One Device Session (Priority: P1)

As a player, I can refresh, reconnect, reopen the browser, or use multiple tabs from the same browser profile without losing my logical identity, character, or current control status during their defined lifetimes.

**Why this priority**: Reliable same-device continuity prevents ordinary browser behavior from disrupting a live tabletop session.

**Independent Test**: Assign a character to one browser profile, open multiple tabs, disconnect and reconnect them in varying order, and verify that one logical session and one character claim remain.

**Acceptance Scenarios**:

1. **Given** a recognized browser profile reconnects during the same server process and broadcast, **when** its session is still valid, **then** it resumes the same fallback name, character assignment, and control status without selecting again.
2. **Given** several tabs share one recognized browser identity, **when** all are open, **then** they represent one logical session and receive the same assignment, control status, and canonical terminal state.
3. **Given** several tabs share one logical session, **when** one tab closes, **then** the session remains connected while at least one other tab is open and its character stays claimed.
4. **Given** the last connection for an observer session closes, **when** presence is updated, **then** the session becomes disconnected and retains its character claim.
5. **Given** browser storage is cleared or a different profile or private context opens the link, **when** the server cannot recognize the prior identity, **then** a new logical session is established and the old character claim remains reserved.
6. **Given** a browser presents an identifier from a previous server process, **when** it connects after restart, **then** the identifier restores no old name, assignment, presence, or control state.

---

### User Story 4 - Manage Roster and Assignments (Priority: P1)

As the game master, I can maintain the character roster and correct, release, assign, or move character claims without changing terminal or puzzle progress.

**Why this priority**: Tabletop setup mistakes and device changes must be recoverable without restarting play.

**Independent Test**: Add and rename characters, create claims, release and transfer them between sessions, and compare terminal and puzzle state before and after every operation.

**Acceptance Scenarios**:

1. **Given** a live server process, **when** the game master adds or renames a roster character, **then** all affected master and player views update and existing logical sessions and terminal state remain unchanged.
2. **Given** an unclaimed roster character, **when** the game master assigns it to an unassigned session, **then** the claim becomes exclusive and the session becomes eligible for terminal control.
3. **Given** a claimed character, **when** deletion is attempted, **then** deletion is refused until the claim is released or transferred.
4. **Given** a connected session with a character, **when** the game master releases the claim, **then** the character becomes available and that session returns to character selection without terminal or puzzle mutation.
5. **Given** a character is moved from one session to another, **when** the move completes, **then** the old session loses it, the new session receives it, and neither terminal state nor puzzle progress changes.
6. **Given** two tabs of one session submit different character selections concurrently, **when** the requests are ordered, **then** the session receives no more than one assignment.
7. **Given** a player has completed selection, **when** the player attempts to choose another character independently, **then** the existing assignment remains until the game master changes it.
8. **Given** a session file references a valid player-config file, **when** the session is created or opened, **then** the authored roster is loaded automatically with the same stable character IDs and names.
9. **Given** a session has no player-config reference, **when** it is created or opened, **then** the game master is offered the explicit choices to select an existing player config or create a new one.
10. **Given** the game master cancels player-config selection or creation, **when** the session editor opens, **then** terminal authoring remains available, no error is shown, and roster and broadcast controls remain unavailable until a player config is selected or created.
11. **Given** a player config is active, **when** the game master adds, renames, or deletes an unclaimed roster entry before or during a broadcast, **then** the complete authored roster is saved to that player-config file before the new roster is published.
12. **Given** a referenced player config is missing, unreadable, or invalid, **when** session loading reaches the roster step, **then** the session remains open, a visible error is shown, and the game master can retry by selecting or creating a player config without receiving a partial roster.
13. **Given** the application restarts, **when** the same session and valid referenced player config are reopened, **then** the authored roster returns but logical sessions, presence, claims, controller assignment, broadcast state, and puzzles do not.

---

### User Story 5 - Reassign Terminal Control (Priority: P1)

As the game master, I can make a connected, character-assigned observer the active controller at any time, so control follows the table's decisions without disturbing play.

**Why this priority**: Explicit reassignment is required for turn-taking and recovery from an unavailable controller.

**Independent Test**: Reassign control between two connected characters during navigation and during an unfinished puzzle, then verify authorization and state continuity from every tab.

**Acceptance Scenarios**:

1. **Given** one active session and one connected assigned observer, **when** the game master selects the observer as controller, **then** the observer becomes active, the former controller becomes an observer, and every tab receives the new status.
2. **Given** a controller reassignment, **when** it completes, **then** character claims, current terminal, navigation, puzzle generation, attempts, board, patterns, logs, and outcome remain unchanged.
3. **Given** an action and reassignment race, **when** the action is ordered before reassignment, **then** it may complete under the former controller's authority.
4. **Given** an action and reassignment race, **when** the action is ordered after reassignment, **then** the former controller's action is rejected without mutation.
5. **Given** no eligible controller exists, **when** an assigned observer remains connected, **then** the terminal stays read-only until the game master assigns control or a later eligible first assignment establishes it.
6. **Given** an active character claim is released or moved away without explicit controller reassignment, **when** the assignment change completes, **then** control is cleared and no existing observer is promoted automatically.
7. **Given** an observer is made controller, **when** the authoritative reassignment revision is applied, **then** its terminal pointer and keyboard controls become actionable from the current shared presentation while the former controller's controls become inert and its screen begins mirroring the new controller, without resetting navigation, selection, page, preview, puzzle, or content state.

---

### User Story 6 - Handle Controller Disconnects (Priority: P1)

As a group, we keep the same shared state when the active player's connection drops, while the game master decides whether to wait or hand control to someone else.

**Why this priority**: Network and device interruptions must not elect an unintended controller or alter a puzzle.

**Independent Test**: Disconnect an active multi-tab session, reconnect it before and after game-master reassignment, and verify control and puzzle continuity.

**Acceptance Scenarios**:

1. **Given** the active session loses its last open connection, **when** it becomes disconnected, **then** it retains its character and active designation and no observer is promoted.
2. **Given** the disconnected active session has not been replaced, **when** it reconnects, **then** it automatically resumes active control.
3. **Given** the game master reassigned control while the former controller was disconnected, **when** the former controller reconnects, **then** it returns as an observer with its original character.
4. **Given** any session disconnects or reconnects, **when** presence changes, **then** puzzle generation, attempts, candidates, removed duds, pattern state, logs, navigation, and outcome remain unchanged.
5. **Given** the active session is disconnected, **when** the game master views session status, **then** that session remains visibly identified as the disconnected active session until it reconnects or control is reassigned.

---

### User Story 7 - Follow the Active Terminal (Priority: P1)

As a connected player, I automatically follow whichever configured terminal the game master presents, without reopening the link or selecting my character again.

**Why this priority**: A broadcast may span several terminals while retaining the same table participants and controller.

**Independent Test**: With active and observer sessions connected, switch among configured terminals and verify automatic transition, stable assignments, and identical terminal state.

**Acceptance Scenarios**:

1. **Given** assigned sessions are connected, **when** the game master activates another terminal, **then** every player transitions through the existing loading presentation to the same newly active terminal.
2. **Given** a terminal switch completes, **when** player status is compared, **then** logical sessions, character assignments, active controller, and observer statuses are unchanged.
3. **Given** no terminal is active during a broadcast, **when** assigned sessions are connected, **then** they retain identity and assignment while seeing an immersive waiting state.
4. **Given** a session joins after a terminal is active, **when** it completes character selection, **then** it joins the currently active terminal's canonical state.
5. **Given** a terminal is inactive, **when** a player sends an action intended for it, **then** the action cannot change that terminal or the active terminal.

---

### User Story 8 - Decide an Unfinished Puzzle's Fate (Priority: P1)

As the game master, I explicitly preserve, discard, or keep playing an unfinished puzzle before switching terminals, so progress is never silently lost or altered.

**Why this priority**: Terminal switching introduces a destructive boundary for runtime puzzle progress that requires an explicit table decision.

**Independent Test**: Attempt to switch away from an unfinished puzzle and exercise preserve, discard, and cancel, then return to the original terminal where applicable. Separately fail the active puzzle, retry it from the game-master surface, and verify a fresh shared puzzle with unchanged broadcast identity, assignments, controller, and active terminal.

**Acceptance Scenarios**:

1. **Given** the active terminal has an unfinished puzzle, **when** the game master requests another terminal, **then** the switch pauses and offers preserve, discard, or cancel.
2. **Given** preserve is chosen, **when** the old terminal becomes inactive, **then** its puzzle is suspended and cannot receive player actions.
3. **Given** a preserved puzzle, **when** its terminal is activated again, **then** the same board, attempts, removed duds, pattern state, progress log, and outcome are restored.
4. **Given** discard is chosen, **when** the terminal is later activated and hacking begins, **then** a fresh puzzle is created under the existing generation rules.
5. **Given** cancel is chosen, **when** the decision completes, **then** the original terminal and puzzle remain active and unchanged.
6. **Given** the active terminal has no unfinished puzzle, **when** another terminal is activated, **then** no unfinished-puzzle decision is required.
7. **Given** the active hacking puzzle is failed and every assigned player sees the blocked state, **when** the game master invokes the trusted retry control, **then** exactly one fresh puzzle for the same active terminal is published to every assigned player without ending the broadcast, changing character assignments, or changing the active controller.

---

### User Story 9 - End and Restart Broadcast Lifetimes (Priority: P2)

As the game master, I can end one live broadcast and start another with clean character and control assignments while recognized devices remain convenient within the same server process.

**Why this priority**: Clear lifetime boundaries prevent stale ownership from leaking into a later tabletop scene.

**Independent Test**: Assign characters and control, end the broadcast, start another without restarting, and then repeat after a server restart.

**Acceptance Scenarios**:

1. **Given** a broadcast has assigned characters and an active controller, **when** the game master ends it, **then** the active terminal is removed from player control and all claims and controller assignment are cleared.
2. **Given** the broadcast ended without a server restart, **when** connected devices remain or reconnect, **then** their logical sessions and fallback names remain recognized but their former characters do not.
3. **Given** a new broadcast starts in the same process, **when** recognized sessions join, **then** every session must select a character again and the first eligible completed assignment becomes the initial controller.
4. **Given** the server application restarts, **when** players reconnect, **then** all prior logical sessions, fallback-name changes, presence, character claims, and controller assignment are gone.
5. **Given** a broadcast ends or the server restarts, **when** durable terminal data is inspected, **then** configured terminals and existing unlocked-terminal behavior remain unchanged.
6. **Given** the reopened session references a valid player config, **when** the server process restarts, **then** the authored roster is restored independently while every prior claim and controller assignment remains cleared.
7. **Given** a broadcast is active and the game-master end-broadcast control is visible, **when** the game master activates and confirms it, **then** exactly one end-broadcast command completes and every connected player leaves the active terminal from the resulting authoritative no-broadcast state.

## Edge Cases

- An empty roster leaves every unassigned session on the immersive selection or waiting surface without terminal access.
- Duplicate character names may be confusing but do not merge distinct roster identities or claims.
- A rename arriving while selection is open updates the same roster entry and does not invalidate a valid claim already being processed.
- Releasing a disconnected session's claim makes the character available while leaving the old logical session recognized.
- A stale selection from an earlier broadcast cannot claim a character in the current broadcast.
- A stale action from a former controller cannot mutate state after reassignment.
- Pointer, keyboard, focus, paging, or preview input in an observer document cannot move either its displayed terminal selection or the authoritative controller-owned presentation.
- A presentation action racing controller reassignment follows the same coordinator order as other player actions: only the session that is controller at its processing point may change the shared presentation.
- Closing one of several tabs cannot mark the shared logical session disconnected.
- Refreshing the active browser cannot trigger observer promotion.
- Clearing local browser state cannot release the old session's claim.
- A session with an invalidated assignment returns to selection even if another tab still displays the terminal.
- Moving the active character without an explicit control reassignment clears control rather than transferring it implicitly.
- Switching terminals while the unfinished-puzzle decision is pending leaves the current terminal authoritative and actionable only according to the existing game-master flow.
- A failed-puzzle retry racing with a player action follows one coordinator order: an action ordered before retry observes the failed puzzle, while an old-generation action ordered after retry cannot mutate the fresh puzzle.
- Rapid duplicate activation of the game-master retry control creates no more than one fresh puzzle generation.
- A retry request arriving after the active terminal, broadcast, or puzzle outcome changes is rejected without mutating either the former or current runtime.
- Rapid duplicate one-use actions from one or several tabs can produce no more than one accepted mutation.
- If an authoritative result rejects an action, pending input still resolves without requiring an animation timeout.
- The private game-master application remains operational when no player is connected, assigned, or active.

## Requirements

### Functional Requirements

#### Logical sessions and presence

- **FR-001**: The system MUST resolve every player connection to exactly one server-recognized logical session.
- **FR-002**: A logical session MUST have an opaque stable identity for the lifetime of the current server process.
- **FR-003**: A newly established logical session MUST receive a unique automatically generated fallback display name.
- **FR-004**: The game master MUST be able to rename a logical session's fallback display name without changing its identity, character assignment, control status, or terminal state.
- **FR-005**: Reopening, refreshing, reconnecting, navigating away and returning, or reopening the browser from the same recognized browser profile MUST reuse the same logical session while the server process remains active.
- **FR-006**: Multiple tabs sharing one recognized browser identity MUST represent one logical session.
- **FR-007**: A logical session MUST be connected while at least one of its player connections remains open.
- **FR-008**: Closing one of several connections for a logical session MUST NOT disconnect the session or release its character.
- **FR-009**: A different browser, browser profile, private-browsing context, cleared local identity, or otherwise unrecognized device context MUST establish a separate logical session.
- **FR-010**: Establishing a new logical session MUST NOT release any claim held by an older disconnected session.

#### Broadcast and assignment lifetimes

- **FR-011**: Logical session identity and fallback-name changes MUST remain process-local and MUST expire when the server process restarts.
- **FR-012**: Character assignments MUST belong to the current live broadcast rather than to an individual configured terminal.
- **FR-013**: Switching active terminals, starting or finishing a puzzle, and disconnecting or reconnecting MUST NOT clear a valid current-broadcast character assignment.
- **FR-014**: Ending a broadcast MUST clear every character claim and the active-controller assignment.
- **FR-015**: Ending a broadcast MUST retain recognized logical sessions until server restart.
- **FR-016**: Starting a new broadcast MUST require every logical session to obtain a new character assignment.
- **FR-017**: A session identifier issued by a previous server process MUST restore none of its previous name, assignment, presence, or control state.
- **FR-018**: Runtime session, fallback-name, presence, character-claim, and controller data MUST NOT be added to the persisted version-1 terminal/session schema.

#### Character roster and claims

- **FR-019**: The game master MUST be able to define a roster of player characters before or during a live broadcast.
- **FR-020**: Each roster entry MUST have a ~~stable identity for the current server process~~ stable identity stored in the active player config, a player-facing name, and a runtime available-or-claimed state. The process-only identity lifetime is superseded by BUG-001 because authored entries must survive restart.
- **FR-021**: The game master MUST be able to add and rename roster entries without mutating logical sessions, controller status, or terminal state.
- **FR-022**: Renaming a roster entry MUST update every affected game-master and player view without creating a new assignment.
- **FR-023**: The system MUST refuse to delete a claimed roster entry until its claim is released or transferred.
- **FR-024**: One logical session MUST have no more than one character assignment during a broadcast.
- **FR-025**: One roster character MUST be claimed by no more than one logical session during a broadcast.
- **FR-026**: Character availability and claim decisions MUST be authoritative at the server when each selection is processed.
- **FR-027**: Concurrent claims for one character MUST result in exactly one successful claim.
- **FR-028**: Concurrent different selections from one logical session MUST result in no more than one character assignment.
- **FR-029**: A rejected claimant MUST remain unassigned and receive the current roster state.
- **FR-030**: A disconnected session's character MUST remain claimed until release, transfer, broadcast end, or server restart.
- **FR-031**: A player MUST NOT independently replace a completed character assignment.
- **FR-032**: The game master MUST be able to assign an available character to an unassigned logical session.
- **FR-033**: Releasing a character MUST make it available and return its connected former session to character selection without canonical terminal or puzzle mutation.
- **FR-034**: Moving a character MUST remove it from the old session and assign it to the new session as one authoritative operation.
- **FR-035**: Character release, correction, and transfer MUST NOT mutate terminal navigation, puzzle state, logs, attempts, randomness, or outcome.
- **FR-036**: Player-facing roster state MUST distinguish available from claimed characters without exposing private connection information.

#### Exclusive controller assignment

- **FR-037**: No more than one character-assigned logical session MUST be designated active controller at any time.
- **FR-038**: The first eligible character assignment processed while the broadcast has no established controller MUST atomically designate exactly one initial active controller.
- **FR-039**: Character-assigned sessions completing selection while a controller exists MUST begin as observers.
- **FR-040**: Raw connection order MUST NOT establish controller status before successful character assignment.
- **FR-041**: Concurrent first-time assignments MUST establish exactly one active controller.
- **FR-042**: Controller assignment MUST apply globally across the live broadcast and all configured terminals.
- **FR-043**: The game master MUST be able to designate a connected, character-assigned observer as the active controller.
- **FR-044**: Reassignment MUST make the selected session active and the previous active session an observer as one authoritative change.
- **FR-045**: Controller reassignment MUST preserve character assignments and all canonical terminal and puzzle state.
- **FR-046**: Releasing or moving the active session's character without explicit reassignment MUST clear controller assignment and MUST NOT promote an observer.
- **FR-047**: When no eligible controller is designated, player terminal actions MUST remain read-only until game-master reassignment or a later eligible initial assignment establishes control.

#### Disconnect behavior

- **FR-048**: Disconnecting the active session MUST retain its active designation and character claim.
- **FR-049**: Disconnecting the active session MUST NOT automatically promote or elect another session.
- **FR-050**: Reconnecting the unchanged active session before reassignment MUST restore its ability to control the terminal.
- **FR-051**: Reconnecting a former controller after game-master reassignment MUST restore it as an observer with its existing character.
- **FR-052**: Disconnecting or reconnecting any session MUST NOT mutate terminal or puzzle state.

#### Player authorization and shared state

- **FR-053**: A player action MUST be eligible for canonical processing only when its connection resolves to a current logical session with a valid current-broadcast character assignment.
- **FR-054**: A terminal-mutating player action MUST be eligible only when the owning logical session is the active controller at processing time and is currently connected.
- **FR-055**: Controller authorization MUST be enforced authoritatively for every player action rather than relying on disabled player controls.
- **FR-056**: Observer, unassigned, unknown, expired, invalid, and stale-controller actions MUST leave all canonical state unchanged.
- **FR-057**: Rejected player actions MUST NOT consume attempts, activate patterns, advance randomness, navigate content, alter logs, trigger outcomes, or otherwise mutate puzzle or terminal state.
- **FR-058**: An unassigned session MUST NOT submit or cause terminal actions before character selection completes.
- **FR-059**: The system MUST accept existing player-side navigation, menu, password-candidate, filler-character, special-pattern, and other terminal actions for canonical processing only from the active controller. BUG-003 clarifies that this eligibility MUST remain actionable through the fully composed rendered-target, browser-gating, WebSocket, coordinator, runtime-mutation, projection, and correlated-result path rather than being proven only at isolated boundaries.
- **FR-060**: ~~The player client MUST retain its established hover, focus, paging, and preview feedback for observers as client-local behavior that submits no shared action and changes no canonical state.~~ Superseded by BUG-008: Observer pointer movement, DOM focus changes, terminal navigation keys, paging controls, and preview controls MUST NOT independently change the observer's displayed terminal selection, highlight, page, preview, or other controller-owned presentation state; the observer MUST continue rendering the latest authoritative presentation.
- **FR-061**: ~~Any other control exposed to an observer MUST be limited to client-local behavior and MUST NOT grant shared-action authority.~~ Superseded by BUG-008: Every terminal control exposed in an observer document MUST be inert for both local terminal presentation and shared mutation, while non-terminal document effects MAY remain local only when they cannot change what terminal content or selection is displayed.
- **FR-062**: The player surface MUST present observer controls as visibly read-only within the existing terminal aesthetic. BUG-008 clarifies that visibly read-only presentation MUST be backed by inert pointer/keyboard/focus handlers and authoritative controller-presentation mirroring rather than styling and outbound-action gating alone.
- **FR-063**: The player surface MUST expose no operation that invokes `ForceHackSuccess`.
- **FR-064**: Active-controller status MUST NOT grant access to any private game-master operation.
- **FR-065**: Accepted active-controller actions MUST continue to follow all existing password, likeness, attempt, special-pattern, dud-removal, restoration, lockout, navigation, and content rules.

#### Ordering, pending input, and convergence

- **FR-066**: Character claims, player actions, controller changes, roster changes, and active-terminal changes MUST have one unambiguous authoritative order.
- **FR-067**: An action ordered before controller reassignment MUST be evaluated using the former controller's authority at that processing point.
- **FR-068**: An action ordered after controller reassignment MUST be rejected when submitted by the former controller.
- **FR-069**: A race between action processing and reassignment MUST NOT produce duplicate or unauthorized canonical mutation.
- **FR-070**: After submitting a shared action, player input MUST enter a pending state until an authoritative outcome is applied.
- **FR-071**: Player clients MUST NOT optimistically mutate canonical shared state.
- **FR-072**: The authoritative outcome of an accepted or rejected action MUST end its pending state without relying only on an arbitrary animation delay.
- **FR-073**: Repeated submission of the same logical request—identified by one logical session, `requestId`, and action fingerprint—for character selection or any shared navigation or hacking action MUST return its original authoritative result and produce no more than one canonical mutation. Conflicting reuse of a `requestId` MUST be rejected without mutation.
- **FR-074**: Every connected assigned session MUST receive accepted terminal navigation, hacking state, logs, loading transitions, and outcomes from the same canonical state.
- **FR-075**: All tabs belonging to one logical session MUST receive the same assignment and controller status changes.

#### Active terminal and unfinished puzzles

- **FR-076**: The game master MUST determine which single configured terminal, if any, is currently presented to players.
- **FR-077**: Activating another terminal MUST transition every connected player automatically through the existing terminal loading presentation.
- **FR-078**: Active-terminal switching MUST preserve logical sessions, character assignments, and controller assignment.
- **FR-079**: A newly assigned player MUST join the terminal currently selected by the game master.
- **FR-080**: When no terminal is active during a broadcast, assigned sessions MUST retain identity and assignment while seeing an immersive waiting state.
- **FR-081**: An inactive terminal MUST reject player actions until it becomes active again.
- **FR-082**: Requesting a switch away from an unfinished puzzle MUST pause the switch for an explicit preserve, discard, or cancel decision by the game master.
- **FR-083**: Choosing preserve MUST suspend the unfinished puzzle in process-local state and make it unable to receive player actions while inactive.
- **FR-084**: Reactivating a terminal with a preserved puzzle MUST restore its exact board, remaining attempts, removed duds, available and used patterns, progress log, and outcome.
- **FR-085**: Choosing discard MUST cause the terminal's next hacking attempt to create a fresh puzzle under the existing generation rules.
- **FR-086**: Choosing cancel MUST leave the current terminal and unfinished puzzle active and unchanged.
- **FR-087**: The system MUST NOT silently discard, solve, fail, restart, or otherwise alter an unfinished puzzle during a terminal-switch request.

#### Game-master coordination and scope boundaries

- **FR-088**: The private game-master application MUST show every currently connected logical session with its fallback name, character assignment if any, presence, and controller-or-observer status.
- **FR-089**: The private game-master application MUST keep a disconnected active session visibly identified until it reconnects or control is reassigned.
- **FR-090**: The game-master session view MUST show both fallback session name and character name when needed to resolve device or assignment problems.
- **FR-091**: Character name MUST remain the primary player-facing identity after assignment, while fallback session name remains a separate technical label.
- **FR-092**: Ending a broadcast MUST remove the active terminal from player control and return connected clients to an immersive waiting or selection state. BUG-004 clarifies that this transition MUST remain actionable through the fully composed visible game-master control, confirmation, desktop facade, coordinator, player publication, and authoritative client-rendering path.
- **FR-093**: Ending a broadcast MUST NOT delete configured terminals or silently alter durable unlocked-terminal state.
- **FR-094**: Server restart MUST discard logical sessions, fallback-name changes, presence, character claims, controller assignment, and any active runtime puzzle allowed to expire by the existing persistence boundary.
- **FR-095**: Player connection, assignment, and controller status MUST NOT remove the private game-master capabilities for session and terminal authoring, roster and assignment management, broadcast lifecycle, terminal activation and switch resolution, or `ForceHackSuccess`; each operation remains subject only to its own canonical eligibility rules. `ResetFailedHack` remains available only under FR-116 through FR-121.
- **FR-096**: This feature MUST NOT redesign existing terminal or hacking logs to add historical character ownership.
- **FR-097**: This feature MUST NOT add accounts, authentication, persistent player profiles, individual invitation links, unassigned spectators, multiple simultaneous controllers, or automatic controller election after disconnect.
- **FR-098**: This feature MUST NOT import or manage character sheets, attributes, skills, perks, eligibility, rules tests, inventory, or campaign history.
- **FR-099**: This feature MUST NOT change existing password-guessing, likeness, attempt, special-pattern, dud-removal, attempt-restoration, lockout, terminal-content, or game-master success rules.

#### Durable roster configuration — BUG-001

- **FR-100**: Authored roster configuration MUST be stored in a separate UTF-8 JSON player-config file containing `version`, `name`, and a `roster` array of stable `id` and `name` pairs.
- **FR-101**: Player-config files created by the application MUST declare `version: 1`, derive their initial `name` from the chosen filename, and begin with an empty roster serialized as `"roster": []`; the create, association, and installation path MUST preserve that empty array rather than treating it as missing or `null`. BUG-002 clarifies the representation invariant after an empty Go slice was converted to nil between these boundaries.
- **FR-102**: A version-1 session JSON MAY contain an optional `playerConfig` string referencing a player-config file; adding this optional reference MUST NOT make existing session files without it invalid.
- **FR-103**: A stored `playerConfig` reference MUST be normalized relative to the session file's directory so a session and its player config remain portable when their directory tree is moved together.
- **FR-104**: After successful session creation or opening, the application MUST automatically load a valid referenced player config or, when no usable reference exists, offer the game master explicit native-dialog actions to select an existing player config or create a new one.
- **FR-105**: Canceling player-config selection or creation MUST keep the selected session open without an error, MUST NOT invent an in-memory roster, and MUST leave roster mutation and broadcast-start controls unavailable until a player config is active.
- **FR-106**: Opening a player config MUST validate supported version, nonblank config name, a roster array, unique nonblank character IDs, and character names under the existing 80-code-point limit before replacing the current roster.
- **FR-107**: Selecting or creating a player config MUST be allowed only when no broadcast is active and MUST replace the authored roster as one operation without deleting recognized logical sessions.
- **FR-108**: Establishing a player-config association MUST save the relative `playerConfig` reference to the active session file; if that session save fails, the previous active association and roster MUST remain unchanged and a newly created standalone player-config file MAY remain available for a later retry.
- **FR-109**: Adding, renaming, or deleting a roster entry MUST require an active player config and MUST atomically save the complete candidate roster before publishing the corresponding coordination revision.
- **FR-110**: A failed player-config write MUST leave the active roster, claims, controller, terminal, puzzle, and saved player-config contents unchanged and MUST return a visible error.
- **FR-111**: Loading a player config after restart MUST restore only authored roster IDs and names; availability MUST be recalculated from a fresh broadcast with no restored claims.
- **FR-112**: Player-config JSON and the session's `playerConfig` reference MUST exclude browser tokens, logical-session IDs and labels, connection IDs, presence, claims, assignments, controller state, broadcast IDs, revisions, request results, active or suspended terminal state, navigation, and puzzles.
- **FR-113**: The same valid player-config file MAY be referenced by more than one session file, and roster changes made through any one of them MUST be visible when another referencing session later loads that config.
- **FR-114**: A missing, unreadable, unsupported, or invalid referenced player config MUST NOT prevent the terminal session from opening and MUST NOT partially load roster entries.
- **FR-115**: Durable roster configuration MUST remain distinct from the excluded account, authentication, persistent character-profile, character-sheet, and campaign-history capabilities.

#### Failed-puzzle retry — BUG-005

- **FR-116**: When the current active hacking puzzle is failed, the private game-master application MUST expose an actionable `ПОВТОРИТЬ ВЗЛОМ` control that invokes exactly one trusted `ResetFailedHack` command and awaits its authoritative result.
- **FR-117**: `ResetFailedHack` MUST be accepted only while a broadcast has the same active hacking-enabled terminal and its current puzzle is failed; no broadcast, no active terminal, a non-hacking terminal, an unfinished puzzle, a solved puzzle, or a stale terminal identity MUST be rejected without canonical mutation.
- **FR-118**: An accepted `ResetFailedHack` MUST discard the failed puzzle's complete private runtime and create a fresh server-owned puzzle from the active terminal's latest validated authored content and hacking level, with a new generation identity, the configured maximum attempts, a fresh board and log, and navigation gated until success under the unchanged hacking rules.
- **FR-119**: An accepted `ResetFailedHack` MUST preserve the current broadcast ID, active terminal ID, logical sessions, presence, character assignments, active controller, roster, other suspended terminal runtimes, configured durable terminals, and durable unlocked-terminal state.
- **FR-120**: Failed-puzzle retry MUST commit under the coordinator order and publish exactly one revisioned fresh terminal/hacking projection to every connected assigned session; player requests carrying targets from the discarded generation MUST NOT mutate the replacement puzzle.
- **FR-121**: `ResetFailedHack` MUST remain a trusted Wails-only game-master operation with no player WebSocket message, browser global, DOM control, keyboard shortcut, query parameter, or public endpoint, and it MUST NOT broaden or rename `ForceHackSuccess`.

#### Authoritative hacking outcome audio — BUG-006

- **FR-122**: ~~When a continuously connected assigned player applies one newer authoritative hacking state whose remaining-attempt count decreased without solving the puzzle, including the final decrease that newly fails and locks the terminal, the player MUST play the established bad-guess cue exactly once.~~ When an audio-enabled continuously connected assigned player applies that transition, the client MUST invoke the established bad-guess cue exactly once. BUG-006 limits the mandatory playback assertion to a document whose own audio eligibility has been established.
- **FR-123**: ~~When a continuously connected assigned player applies one newer authoritative hacking state that changes the puzzle from unsolved to solved, the player MUST play the established good/unlock cue exactly once and retain the existing delayed transition to normal navigation.~~ When an audio-enabled continuously connected assigned player applies that transition, the client MUST invoke the established good/unlock cue exactly once and retain the existing delayed transition to normal navigation. BUG-006 limits the mandatory playback assertion to a document whose own audio eligibility has been established.
- **FR-124**: A local target click, a correlated `ACTION_RESULT` without its authoritative hacking projection, a rejected action, an initial or reconnect `TERMINAL_LIVE` snapshot, a stale or duplicate revision, and a re-render of unchanged hacking state MUST NOT play a bad, good/unlock, or lockout outcome cue.
- **FR-125**: Hacking outcome audio MUST remain a client-local, non-canonical side effect: playback MUST submit no player request, change no puzzle or terminal state, and neither delay nor prevent authoritative pending-input completion when browser audio is unavailable or playback fails.

#### Complete hacking-screen audio — BUG-007

- **FR-126**: In an audio-enabled assigned player document showing an unfinished hacking puzzle, newly previewing one individual filler target MUST invoke exactly one established `single` cue, while newly previewing one password candidate or unused special pattern MUST invoke exactly one established `multiple` cue. Repeated pointer movement within the same current target MUST NOT replay its preview cue.
- **FR-127**: In an audio-enabled assigned player document, selecting an actionable password candidate, filler target, or unused special pattern MUST invoke exactly one established `enter` cue as client-local feedback. That cue MUST NOT authorize an observer, optimistically mutate canonical state, or replace the authoritative bad/good outcome behavior in FR-122 through FR-125.
- **FR-128**: The packaged player MUST retain non-empty, browser-native-decodable assets and successful same-origin manifest/static delivery for the established `ambient`, `charscroll`, `single`, `multiple`, `enter`, `hack-bad`, and `hack-good` hacking-screen sound families.
- **FR-129**: Verification of required hacking-screen audio MUST exercise the production sound adapter with the browser's native audio decoder and an output-capable runtime. A fake `AudioContext`, successful source-start callback, manifest response, or asset-presence assertion MAY support localization but MUST NOT alone be accepted as proof of audible playback.

#### Authoritative observer presentation — BUG-008

- **FR-130**: The semantic terminal selection that determines the visible selected menu row, information page, and actionable hacking preview MUST be controller-owned process-local runtime state for the active terminal rather than independent per-browser state; raw pointer coordinates and browser chrome state MUST NOT become canonical.
- **FR-131**: A pointer or keyboard interaction that changes controller-owned presentation MUST be eligible only for the connected, character-assigned active controller of the current broadcast and terminal, MUST be validated against the current navigation/content/puzzle context, and MUST commit in the same authoritative order as controller reassignment and other player actions.
- **FR-132**: Observer presentation input MUST produce no local terminal-selection change, no accepted presentation mutation, and no outbound player mutation request; a crafted observer presentation request MUST be rejected without revision advance or terminal, puzzle, presentation, audio-eligibility, or randomness mutation.
- **FR-133**: Every connected assigned session MUST receive each accepted controller-presentation revision and render the same semantic selection, page, and preview; the complete current presentation MUST also be included in late-join, new-tab, and reconnect snapshots.
- **FR-134**: Controller reassignment MUST atomically transfer presentation authority: the new controller becomes interactive from the unchanged current presentation, while the former controller becomes inert and follows subsequent authoritative presentation revisions without retaining an independent highlight or preview.
- **FR-135**: Terminal activation, navigation changes, content publication, hacking generation or outcome changes, pagination recalculation, and target removal MUST revalidate or deterministically reset controller-owned presentation so no client renders a stale or invalid selection, page, or preview.
- **FR-136**: Raw pointer position, browser DOM focus, connection overlays, reveal-animation progress, per-document audio eligibility, and playback success MAY remain client-local, but none MAY select different terminal content or override the authoritative semantic presentation shown to an observer.
- **FR-137**: BUG-008 player presentation actions and projections MUST extend the active generated ConnectRPC/protobuf public contract and MUST NOT restore or depend on the superseded handwritten WebSocket protocol.
- **FR-138**: Existing menu-focus and hacking preview cues MUST remain per-document audio side effects of applying an applicable newer authoritative controller-presentation state; observer-local pointer, focus, or keyboard input MUST NOT independently trigger a terminal selection or preview cue.

#### Non-blocking controller-presentation pending — BUG-009

- **FR-139**: A current controller's in-flight `SetPresentation` request MUST remain correlated and non-optimistic without applying observer-read-only or blocking-shared-input affordances to that controller's terminal controls; presentation-only pending MUST NOT change their normal cursor, opacity, saturation, or actionable appearance and MUST NOT prevent a newer eligible presentation movement from superseding the pending target.
- **FR-140**: Separating presentation-only pending feedback MUST NOT weaken observer inertness, crafted-request authorization, authoritative-only semantic selection, revision ordering, or the established visibly blocked treatment for gameplay/navigation/command input that is actually unavailable while its shared mutation is pending.

#### Bounded latest-target presentation dispatch — BUG-010

- **FR-141**: Each active-controller document MUST keep at most one generated `SetPresentation` request in flight. While it is unresolved, each newer eligible semantic pointer or keyboard target MUST replace one client-local desired-presentation slot rather than dispatch another concurrent request; repeated intent equal to the authoritative, in-flight, or queued target MUST add no request.
- **FR-142**: After the in-flight presentation is authoritatively completed or rejected, the client MUST dispatch at most the latest queued presentation that is still eligible for the current controller, broadcast, terminal, and semantic context. Loss of control, broadcast or terminal replacement, context change, navigation revalidation, disconnect, or teardown MUST discard or revalidate queued intent so it cannot mutate a stale surface.
- **FR-143**: Superseded unsent presentation targets MUST produce no canonical revision, highlight change, selected-target/reveal animation restart, or preview cue. Accepted in-flight and final follow-up projections MUST remain authoritative, ordered, and visible to every assigned player without optimistic local selection, observer divergence, or weakening BUG-009's non-blocking controller styling.

## Key Entities

- **Logical Connection Session**: A temporary identity for one recognized device and browser profile during one server process. It has an opaque identity, unique fallback name, aggregate connected presence, optional current-broadcast character assignment, and controller-or-observer status. It is not an account or campaign profile.
- **Browser Connection**: One currently open player connection belonging to a logical session. Several connections may belong to the same session, and aggregate presence remains connected until the final one closes.
- **Character Roster Entry**: A game-master-defined, player-facing character identity. ~~It is available only within the current server process and has a stable process-local identity.~~ Under BUG-001 it is loaded from the active player config and has a stable durable roster identity, a mutable display name, and a runtime available-or-claimed state, but no character-sheet data.
- **Player Config**: A separately selected versioned JSON file containing the reusable authored roster. A session may hold a relative reference to it; it contains no browser recognition, presence, assignment, controller, terminal-runtime, puzzle, account, or character-sheet data.
- **Character Assignment**: The exclusive relationship between one logical session and one roster entry for the current live broadcast. It is independent from controller status.
- **Live Terminal Broadcast**: The runtime period in which the game master may present one configured terminal at a time to connected players. It owns the current character-assignment lifetime and controller assignment.
- **Active Controller Assignment**: The exclusive designation of at most one character-assigned logical session as authorized to submit player terminal actions for the current broadcast.
- **Observer Session**: ~~Any character-assigned logical session that is not the active controller. It mirrors canonical state and may use only passive or client-local interactions.~~ Under BUG-008, any character-assigned logical session that is not the active controller. It mirrors canonical terminal and controller-owned presentation state; its terminal pointer, keyboard, focus, paging, and preview interactions are inert.
- **Controller-Owned Terminal Presentation**: Process-local semantic state for the active terminal's selected menu target, information page, or hacking preview target. Only the current connected controller may change it; every assigned player renders it, while raw pointer coordinates and browser-only technical state remain local.
- **Audio-Enabled Player Client**: One player document with Web Audio support that has received its own user activation and successfully loaded the applicable sound asset. Audio eligibility belongs to that browser document and is never shared, inferred from another player, or treated as canonical state.
- **Active Terminal**: The single configured terminal currently presented to players, or no terminal while assigned players wait.
- **Suspended Puzzle**: An unfinished process-local puzzle preserved for an inactive terminal, unavailable for actions until that terminal is active again.
- **Authoritative Action Outcome**: The accepted or rejected result that determines whether canonical state changes and releases the player's pending input state.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Across at least 100 concurrent two-session trials for one available character, every trial produces exactly one successful claim and one rejection.
- **SC-002**: Across at least 100 concurrent first-assignment trials with different characters, every trial finishes with exactly one active controller and all other assigned sessions as observers.
- **SC-003**: A suite covering refresh, reopen, transient disconnect, navigation away and back, and at least three simultaneous tabs produces one logical session and one character claim for the same recognized browser profile.
- **SC-004**: Reconnecting a recognized session during the same broadcast restores its character without showing selection again in every tested reconnect path.
- **SC-005**: A different browser profile, private context, or cleared local identity creates a distinct logical session in every tested case.
- **SC-006**: A claimed character remains unavailable through at least 100 disconnect and competing-claim trials until the game master releases or transfers it.
- **SC-007**: One hundred observer attempts spanning every player action category produce zero canonical navigation, puzzle, attempt, randomness, log, or outcome mutations. Under BUG-008 the same attempts also produce zero observer-local terminal-selection changes, zero presentation revisions, and zero outbound mutation requests.
- **SC-008**: Only a connected, character-assigned active session produces accepted player mutations in authorization tests covering active, observer, unassigned, disconnected, stale, unknown, and expired sessions.
- **SC-009**: Disconnecting the active session produces zero automatic observer promotions across all tested single-tab and multi-tab disconnect sequences.
- **SC-010**: Reconnecting the unchanged active session restores control in every trial, while reconnecting after reassignment restores observer status in every trial.
- **SC-011**: Game-master reassignment changes authorization and all affected tab statuses while producing zero changes to assignments, terminal navigation, board, attempts, patterns, logs, or outcome. Under BUG-008 the same authoritative revision makes the new controller interactive from the unchanged shared presentation and makes the former controller inert without retaining an independent selection.
- **SC-012**: Character rename, session rename, claim correction, release, and transfer each produce zero terminal or puzzle state changes in before-and-after comparisons.
- **SC-013**: At least 100 deliberately interleaved action-and-reassignment trials follow one authoritative order and produce no duplicate or unauthorized mutation.
- **SC-014**: All connected player views converge on identical canonical terminal state after every accepted action in multi-client navigation and hacking scenarios, including the current controller-owned semantic selection, page, and preview under BUG-008.
- **SC-015**: Every connected client follows at least ten active-terminal switches without reconnecting or selecting its character again.
- **SC-016**: A preserved unfinished puzzle returns with an exact match for board, attempts, removed duds, pattern state, progress log, and outcome after switching away and back.
- **SC-017**: Preserve, discard, and cancel tests show no silent puzzle solve, failure, restart, or loss during terminal-switch decisions.
- **SC-018**: Ending a broadcast clears all character and controller assignments while retaining all recognized logical sessions until restart.
- **SC-019**: Starting a second broadcast requires new character selection from every recognized session, and restarting the server restores no prior logical session or claim.
- **SC-020**: Persistence comparison before and after the feature shows no logical-session, fallback-name, presence, ~~authored roster,~~ character-claim, or controller data in the version-1 session body. BUG-001 permits only the relative `playerConfig` reference there and stores the authored roster in its separate file.
- **SC-021**: Player-surface and crafted-player-action checks find zero path to `ForceHackSuccess`, while the existing game-master operation remains available.
- **SC-022**: Existing regression suites for password guesses, likeness, attempts, special patterns, dud removal, attempt restoration, lockout, terminal content, and game-master forced success pass unchanged.
- **SC-023**: Creating a player config, adding at least three roster entries, restarting the application, and reopening the referencing session restores exactly the same roster IDs, order, and names without manual re-entry.
- **SC-024**: Session files without `playerConfig` continue to open successfully and present the select-or-create workflow in every compatibility test.
- **SC-025**: Create, select, cancel, missing-file, malformed-file, unsupported-version, duplicate-ID, and failed-write tests produce no partial roster, association, claim, controller, terminal, or puzzle mutation; the create test MUST distinguish a valid empty `[]` roster from a missing or `null` roster through the complete create-to-install path.
- **SC-026**: Every successful add, rename, and unclaimed delete is present after the active player-config file is reopened, while every simulated failed write leaves both disk and coordination snapshots unchanged.
- **SC-027**: Inspection of session and player-config JSON after complete runtime activity finds zero browser token, logical-session, connection, presence, claim, assignment, controller, broadcast, revision, request, navigation, or puzzle field.
- **SC-028**: A production-composed browser journey using the real player WebSocket server and coordinator lets the current active controller select a password candidate, filler cell, and unused special pattern; each selection produces exactly one accepted canonical mutation, all assigned clients converge, pending input resolves authoritatively, and equivalent observer selections produce zero outbound shared actions and zero canonical mutation. BUG-008 additionally requires zero observer-local highlight/preview change from those attempts and convergence on the controller's authoritative preview.
- **SC-029**: A production-shaped game-master journey starts with a visible and enabled `ЗАВЕРШИТЬ ТРАНСЛЯЦИЮ` control, accepts its confirmation, observes exactly one `EndBroadcast` invocation, and verifies that all connected players receive the authoritative no-broadcast context and terminal clear while logical sessions, fallback names, authored roster, and configured durable terminals remain unchanged.
- **SC-030**: A production-shaped game-master journey exhausts the active puzzle, observes the blocked state, activates `ПОВТОРИТЬ ВЗЛОМ`, observes exactly one `ResetFailedHack` invocation, and verifies that active and observer players converge on one fresh generation with full attempts while broadcast ID, active terminal, assignments, controller, sessions, roster, configured terminals, and durable unlocked state remain unchanged.
- **SC-031**: Concurrent duplicate retry and stale-generation action tests create exactly one fresh puzzle and produce zero post-reset mutation from discarded-generation targets, while inspection of every player asset and protocol path finds zero way to invoke `ResetFailedHack`.
- **SC-032**: ~~A production-shaped active-and-observer browser journey with observable Web Audio playback applies an ordinary wrong guess, a correct-password unlock, and a final-attempt lockout; every continuously connected assigned view records exactly one bad, good, and bad cue for those respective new authoritative transitions and records zero outcome cues for local pre-acknowledgement clicks, rejected actions, duplicate/stale projections, reconnect snapshots, and unchanged re-renders.~~ Across three fresh hacking-puzzle cases—ordinary wrong guess, correct-password unlock, and final-attempt lockout—every audio-enabled continuously connected active and observer view receives its own enabling gesture and records exactly one bad, good, and bad cue invocation respectively. Those views record zero outcome-cue invocations for local pre-acknowledgement clicks, rejected actions, duplicate or stale projections, reconnect snapshots, and unchanged re-renders.
- **SC-033**: In an output-capable production browser served by the packaged player, one explicitly audio-enabled hacking journey audibly retains CRT ambience and character-scroll as working controls and produces the applicable `single`, `multiple`, `enter`, `hack-bad`, and `hack-good` cues across individual filler preview, word/pattern preview, target selection, ordinary wrong guess, final lockout, and unlock cases. Native decode/output evidence is recorded for every required family; synthetic source-start counters alone do not satisfy this criterion.
- **SC-034**: In a production-composed controller-and-observer browser matrix, 100 observer pointer, focus, arrow-key, paging, back, activation, and hacking-preview attempts produce zero local terminal-selection changes, zero outbound presentation/gameplay mutations, and zero canonical revision changes, while every accepted controller presentation movement appears in every observer view in authoritative order.
- **SC-035**: Across at least 100 controller-presentation-action versus reassignment interleavings, only the session that is controller at the processing point changes presentation; every former controller becomes inert, every new controller becomes interactive from the unchanged shared presentation, and no client retains a divergent selection, page, or preview.
- **SC-036**: Late-join, new-tab, and reconnect tests render the current controller-owned menu selection, page, or hacking preview from the first complete snapshot in every trial, with no default-selection flash or observer-local reset after the snapshot is applied.
- **SC-037**: With `SetPresentation` responses deliberately delayed, at least 100 active-controller pointer and keyboard movements produce zero `shared-input-pending`/read-only styling, zero `not-allowed` cursor, zero opacity or saturation drop, and zero visible locked-state flash; ~~newer eligible movements remain dispatchable~~ under BUG-010 newer eligible movements remain accepted as replaceable latest intent for bounded follow-up dispatch, the selected semantic target changes only from authoritative revisions, and observer plus gameplay-pending regressions remain unchanged.
- **SC-038**: With the first generated `SetPresentation` request held in flight, a rapid sweep across at least 100 distinct filler, password, and unused-pattern hover transitions sends exactly one request before release and at most one follow-up carrying the final still-applicable target after release. No skipped intermediate target produces a revision, selected-target/reveal animation restart, or `single`/`multiple` cue; active and observer views converge on the final target, and role/context changes discard stale queued intent in every tested case.

## Assumptions

- The existing shared terminal, server-authoritative navigation, hacking puzzle, special-pattern behavior, player presentation, loading animation, and private game-master operation are stable dependencies of this feature.
- BUG-005 clarifies that the migrated hacking feature's “restart live terminal” recovery means replacing only the failed active terminal runtime inside the current broadcast; it does not mean ending and recreating the broadcast epoch.
- BUG-006 clarifies that existing incorrect-guess and correct-guess audio belongs to newly applied authoritative hacking transitions; it is not a new audio system or an optimistic response to local selection. The affected surfaces are player-side transition detection and Web Audio playback, player HTTP sound-manifest and static-asset delivery only if reproduced as defective, browser regression fixtures, and packaged native verification; no canonical model or WebSocket payload changes.
- BUG-007 clarifies that the migrated hacking feature's established audio contract also includes individual-target preview, grouped word/pattern preview, and target-entry cues. The working ambient and character-scroll families are diagnostic controls, not sufficient proof that the remaining Web Audio assets decode and play audibly in the packaged player.
- BUG-008 supersedes the earlier `live-broadcast-shared-navigation` rule that pointer/keyboard highlight remains local per browser whenever feature 004 controller/observer roles are active. Semantic selection, paging, and preview are then controller-owned authoritative presentation; only raw pointer/DOM state and non-terminal document effects remain local.
- BUG-009 clarifies that authoritative, non-optimistic presentation does not make the active controller visually read-only during each `SetPresentation` round trip. Request correlation remains pending internally while normal controller affordances stay stable; observer and genuinely blocked shared-gameplay states retain their distinct locked presentation.
- BUG-010 clarifies that BUG-009's latest-wins eligibility requires bounded dispatch rather than one concurrent mutation per transient pointer target. Only one presentation request may be in flight per controller document, one queued desired target may be replaced, and superseded unsent targets have no canonical, visual, or audio effect.
- A browser identity can be recognized across ordinary reopen and reconnect behavior within a server process; the mechanism is deliberately deferred to planning.
- ~~The roster definition is process-local, remains available across broadcast endings within that process, and is cleared on server restart; claims are still cleared at every broadcast end.~~ Superseded by BUG-001: authored roster IDs and names come from the active player config and survive restart, while availability, claims, controller state, and every other coordination value retain their existing runtime lifetimes.
- When control has been cleared, the next newly completed eligible character assignment may establish control, but already assigned observers are never promoted automatically.
- Moving a character away from the active session without an explicit simultaneous controller reassignment clears control and does not transfer control with the character.
- Duplicate character display names are permitted because stable roster identity, not the visible name, determines claims; the game master is responsible for choosing distinguishable names.
- A rejected player action yields an authoritative outcome sufficient to clear local pending input even though its exact transport representation is deferred to planning.
- Computers, laptops, and tablets are supported; mobile-phone-specific layout work is outside this feature.
- The shared player link and existing network exposure model remain unchanged; this feature adds no access-control guarantee for the link itself.

## Scope Boundaries

This feature excludes user accounts and passwords, ~~persistent player or campaign profiles,~~ persistent player or campaign profiles beyond the BUG-001 roster-only player config, character-sheet and rules automation, hacking eligibility, individual invitation links, additional internet-access controls, unassigned spectators, simultaneous multi-session control, automatic controller election after disconnect, persistence of logical sessions or claims beyond their stated lifetimes, historical action attribution, per-terminal controller assignments, mobile-phone-specific presentation, localization, ~~audio-system work,~~ ~~new audio controls, asset families, configuration, or audio-system redesign beyond restoring the established BUG-006 outcome cues,~~ new audio controls, new asset families, configuration, or audio-system redesign beyond restoring the established BUG-007 hacking-screen interaction and outcome cues, and any visual redesign.

## Verbatim Constraints

- The trusted game-master operation name MUST remain exactly `ForceHackSuccess`.
- The trusted failed-puzzle retry operation name MUST remain exactly `ResetFailedHack`.
