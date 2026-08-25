# Feature Specification: Terminal Grouping

**Feature Directory**: `specs/020-terminal-grouping`

**Created**: 2026-08-25

**Status**: Draft

**Input**: Represent every terminal through a high-level terminal group so player-initiated forward and backward terminal transitions stay within one group while retaining Overseer approval, including when a broadcast starts from a terminal in the middle of the group. A standalone terminal is represented by a group containing only that terminal.

**Bugfix**: 2026-08-25 — BUG-001 Added measurable terminal-list hierarchy, readable-name, collapsible-group, and contextual-action requirements based on the accepted UX mockup.

**Bugfix**: 2026-08-25 — BUG-002 Added the positive legacy-transition repair flow so a dormant cross-singleton link can be made eligible by moving its target into the source group, with the accepted canonical grouping authoritative for subsequent eligibility checks.

**Bugfix**: 2026-08-25 — BUG-003 Clarified complete-candidate validation for production-shaped legacy sessions with multiple terminals and authored transition edges, including independent edge classification and fixture-faithful acceptance evidence.

**Bugfix**: 2026-08-25 — BUG-004 Clarified that legacy-repair acceptance evidence must bind the unchanged authored file, reviewed candidate, serialized production payload, executable identity, and canonical pre/post state before the repair is considered verified.

**Bugfix**: 2026-08-26 — BUG-005 Added the exact grouping-aware user-action journey and actionable partial-candidate repair requirements so a valid strict rejection cannot be mistaken for completion of the multi-edge repair.

## User Scenarios & Testing

### User Story 1 - Organize Every Terminal Through a Group (Priority: P1)

As the Overseer, I see groups as the highest-level terminal organization and can couple one or more terminals into each named, ordered group. Every terminal is always represented by exactly one group, including a standalone terminal whose group contains only that terminal.

**Why this priority**: Group membership is the authority boundary for every forward and backward terminal transition in this feature.

**Independent Test**: Start with several singleton groups, couple multiple terminals into one ordered group, move one terminal back into its own singleton group, save and reopen the session, and verify that every terminal appears beneath exactly one high-level group in the intended order.

**Acceptance Scenarios**:

1. **Given** a session contains several standalone terminals, **When** the Overseer views terminal organization, **Then** each terminal appears beneath its own singleton group and no terminal appears directly at the top level.
2. **Given** terminals A, B, and C are represented by separate groups, **When** the Overseer couples them into one named group, **Then** one high-level group contains A, B, and C in the selected order and the former empty group shells do not remain.
3. **Given** a terminal belongs to a multi-terminal group, **When** the Overseer separates it from that group, **Then** the terminal receives a singleton group rather than becoming ungrouped.
4. **Given** a terminal already belongs to one group, **When** the Overseer moves it to another group, **Then** it leaves the source group and appears only in the destination group.
5. **Given** groups and memberships have been edited, **When** the session is saved and reopened, **Then** group names, identities, order, and terminal memberships are restored without loss or reassignment.
6. **Given** a group contains one terminal, **When** the session is saved, **Then** the singleton group remains valid and the terminal remains available for direct Overseer activation but has no cross-terminal destination.
7. **Given** an operation would leave a group with zero terminals, **When** the Overseer applies it, **Then** the empty group is removed as part of the same change and no terminal is lost.
8. **Given** a proposed rename would duplicate another group name, **When** the Overseer applies it, **Then** the change is rejected with a clear explanation and the previous names remain unchanged.
9. **Given** a group contains terminals A, B, C, and D, **When** the Overseer arranges them in that order and saves the session, **Then** the same A, B, C, D order is shown after reopening it.
10. **Given** the Overseer creates a new standalone terminal, **When** creation completes, **Then** a singleton group for that terminal is created atomically and there is no intermediate ungrouped state.
11. **Given** terminal groups are shown at a supported desktop viewport, **When** the Overseer scans the terminal organization panel, **Then** each group name, member count, terminal order, full terminal name, selected state, and per-item action trigger remain visually distinct without overlapping or competing rows of inline controls.
12. **Given** a group is expanded, **When** the Overseer collapses and re-expands it by pointer or keyboard, **Then** its terminal members hide and return in the same canonical order without changing selection, membership, or persisted session data.

---

### User Story 2 - Manage Groups Safely (Priority: P1)

As the Overseer, I can create, rename, dissolve, and reorganize groups, including moving terminals between them, while destructive changes remain explicit, reviewable, and safe for terminal content and active play.

**Why this priority**: Groups can govern navigation only if the Overseer can maintain them without accidental data loss or partially applied membership changes.

**Independent Test**: Create and rename a group, move terminals between groups, cancel and approve destructive changes, dissolve a multi-terminal group, and verify that confirmations describe the impact, accepted changes apply once and atomically, and no terminal content is deleted.

**Acceptance Scenarios**:

1. **Given** terminals A and B are in separate groups, **When** the Overseer creates a new group containing both terminals, **Then** a confirmation identifies A, B, their source groups, and the new membership before any change occurs.
2. **Given** a group-management confirmation is open, **When** the Overseer cancels, closes, or dismisses it, **Then** group names, memberships, order, authored links, pending decisions, and active navigation remain unchanged.
3. **Given** terminal A belongs to Group One, **When** the Overseer proposes moving A to Group Two, **Then** a confirmation identifies A, Group One, Group Two, and the resulting navigation-order change.
4. **Given** the proposed move remains valid, **When** the Overseer confirms it, **Then** A appears exactly once in Group Two and is removed from Group One as one atomic change.
5. **Given** a multi-terminal group is selected for deletion, **When** the Overseer confirms the listed impact, **Then** the group is dissolved, each terminal remains intact in a valid singleton group unless the same confirmed operation moves it elsewhere, and no empty group remains.
6. **Given** the Overseer renames a group without changing its membership or order, **When** the unique valid name is saved, **Then** the group's stable identity and all navigation relationships remain unchanged.
7. **Given** a destructive confirmation is open, **When** relevant group, link, pending-decision, or active-route state changes before confirmation, **Then** the operation is rejected with current-state feedback and no part of the stale proposal is applied.
8. **Given** a destructive group change has already been confirmed successfully, **When** the same confirmation is submitted again, **Then** it produces no duplicate move, group, or order change.
9. **Given** a proposed group change would invalidate an authored transition, pending decision, or active route, **When** the Overseer confirms it, **Then** the operation is rejected without partial changes and the affected items are identified.
10. **Given** a singleton group is selected for deletion without moving or deleting its terminal, **When** the Overseer attempts the operation, **Then** it is unavailable or rejected because the terminal must remain represented by a group.
11. **Given** the Overseer opens the action trigger for a group or terminal, **When** its contextual menu appears, **Then** the menu identifies the target, exposes every applicable management action with unabbreviated accessible labels, separates destructive actions visually, and keeps actions for other items closed.

---

### User Story 3 - Move Forward Only Within a Group (Priority: P1)

As the controlling player, I can use an authored terminal-transition command only when its source and destination terminals belong to the same group, so a terminal route cannot escape the area defined by the Overseer.

**Why this priority**: Enforcing the forward boundary is the primary gameplay protection introduced by grouping.

**Independent Test**: Configure terminals A and B in one group and C in another, exercise transition commands A to B and A to C, and verify that only A to B can reach the existing Overseer approval flow.

**Acceptance Scenarios**:

1. **Given** terminals A and B belong to the same group, **When** the Overseer configures a transition command in A, **Then** B is available as a destination.
2. **Given** terminal C belongs to another group, **When** the Overseer configures a transition command in A, **Then** C is not offered as a valid destination.
3. **Given** a valid A to B command is selected by the controlling player, **When** no other decision is pending, **Then** exactly one transition request is presented to the Overseer and A remains active until the decision.
4. **Given** an A to B request is pending, **When** the Overseer approves it and both terminals still share the same group, **Then** B becomes active and one return point to A is added to the route.
5. **Given** an A to B request is pending, **When** the Overseer rejects or closes it, **Then** A remains active and the route is unchanged.
6. **Given** a command links terminals in different groups, includes an endpoint missing from valid group representation, or refers to a removed terminal, **When** a player attempts to select it, **Then** no approval request, route change, or terminal activation occurs.
7. **Given** group membership or the authored link changes while a request is pending, **When** the Overseer attempts to approve it, **Then** the transition is rejected safely and the currently active terminal remains unchanged.
8. **Given** an observer or unassigned player selects a valid same-group transition, **When** authority is checked, **Then** no request or state change is created.
9. **Given** an ordered group A, B, C, D and a fresh broadcast whose first active terminal is C, **When** the controlling player follows approved forward transitions, **Then** starting at C does not prevent the player from reaching D or from later traversing forward from A through B and C to D.

---

### User Story 4 - Return Only Within the Same Group (Priority: P1)

As the controlling player, I can request a return to the previous terminal only while the current terminal and saved return terminal remain in the same group, and the return still requires Overseer approval.

**Why this priority**: A forward-only boundary would be incomplete if the existing return route could cross the same boundary in reverse.

**Independent Test**: In an ordered A, B, C, D group, start a fresh broadcast at C, return with approval through B to A, then follow approved forward transitions through B, C, and D; verify that starting in the middle does not make any group member unreachable and that a changed or cross-group return target cannot create or complete a return.

**Acceptance Scenarios**:

1. **Given** A to B was approved within one group, **When** the controlling player requests return from B at its root, **Then** exactly one return request to A is presented to the Overseer.
2. **Given** a return request is pending, **When** the Overseer approves it and both endpoints still share the same group, **Then** A becomes active and exactly one route point is removed.
3. **Given** a return request is pending, **When** the Overseer rejects or closes it, **Then** B remains active and the route is unchanged.
4. **Given** the route is A to B to C inside one group, **When** the Overseer approves two successive returns, **Then** the route unwinds C to B and then B to A in last-in-first-out order.
5. **Given** current membership would place the active and return terminals in different groups, **When** return is requested or approved, **Then** the return is unavailable or rejected without changing the active terminal or route.
6. **Given** no earlier terminal remains in the route, **When** the player views the terminal root, **Then** no cross-terminal return action is available.
7. **Given** an ordered group A, B, C, D and a fresh broadcast whose first active terminal is C, **When** the controlling player requests a backward transition from C, **Then** a request to B is presented to the Overseer even though B was not visited earlier in that broadcast.
8. **Given** the broadcast started at C and the Overseer approves successive backward transitions, **When** the player continues backward, **Then** navigation reaches B and then A without leaving the group or skipping a member.
9. **Given** the broadcast started at C, then approved navigation reaches D and subsequently moves backward, **When** the player continues through approved backward transitions, **Then** navigation can unwind through C, B, and A in group order.

---

### User Story 5 - Preserve Safe Sessions and Active Play (Priority: P2)

As the Overseer, I receive clear feedback when a group edit would invalidate authored links or an active route, while older sessions still open without losing terminal content.

**Why this priority**: Safe editing and compatibility protect authored campaigns after the core grouping and navigation rules work.

**Independent Test**: Open an older session without groups, attempt conflicting regrouping during authored and active routes, and verify that invalid changes are blocked without data loss while ordinary terminal behavior continues.

Also move the target of one dormant legacy transition into its source terminal's normalized singleton group, save and reopen, and verify that the preserved transition becomes same-group eligible.

Repeat the repair with a three-terminal legacy chain equivalent to `t-krel-service` -> `t-krel-admin` -> `t-krel-emergency`: prove that each authored edge is classified from the complete candidate, a joined edge is never reported as cross-group, and a candidate joining all three terminals succeeds through save and reopen.

**BUG-004 acceptance clarification**: Evidence for the legacy repair MUST use an unchanged disposable copy of the reported authored file and record its content identity, the active executable/build identity, the precise UI gesture sequence, the complete membership reviewed by the Overseer, the semantically equivalent candidate at each production boundary, and the canonical session and revision before and after submission. Evidence from a minimized equivalent fixture does not close a contradictory authored-file result until that equivalence and the first divergent boundary have been demonstrated.

**Acceptance Scenarios**:

1. **Given** an older session has terminals and transition commands but no groups, **When** it is opened, **Then** all terminals and command content load unchanged and every terminal is represented by its own singleton group.
2. **Given** an older transition command connects terminals that were placed into separate singleton groups, **When** a player selects it before the Overseer couples both endpoints into one group, **Then** it cannot start a terminal transition.
3. **Given** a group edit would leave an authored transition crossing groups, **When** the Overseer attempts to apply the edit, **Then** the edit is rejected and the affected terminals and commands are identified.
4. **Given** terminals participate in a pending transition or active return route, **When** a group edit would split an applicable route pair, **Then** the edit is rejected until that navigation state is resolved or cleared.
5. **Given** a group is removed after its terminals and links are safely reassigned or cleared, **When** the change is saved, **Then** the group disappears without deleting any terminal.
6. **Given** the Overseer directly activates a terminal in another group, **When** the existing manual activation flow completes, **Then** direct activation remains available and clears the player-created return route according to existing behavior.
7. **Given** players reconnect during a pending or completed valid transition, **When** they receive the current broadcast state, **Then** they converge on the same active terminal, pending decision, and available return action.
8. **Given** ordinary commands, state-changing commands, or navigation within one terminal are used, **When** the feature is enabled, **Then** their existing approval and execution behavior remains unchanged.
9. **Given** an older A to B transition is dormant because A and B normalized into separate singleton groups, **When** the Overseer confirms moving B into A's existing singleton group, **Then** the complete candidate is accepted, B's empty source group is removed, the command remains authored, and A to B becomes eligible after save and reopen.
10. **Given** an older S to A to E transition chain normalized into three singleton groups, **When** the Overseer confirms either a partial candidate that joins A and E or a complete candidate that joins S, A, and E, **Then** every authored edge is classified independently from that candidate, A to E is not reported as cross-group in either case, only the still-split S to A edge rejects the partial candidate, and the complete candidate is accepted and remains eligible after save and reopen.
11. **Given** the Overseer uses a grouping action on an older multi-edge chain and the resultant candidate would join one authored edge while leaving another split, **When** the impact is reviewed or the candidate is rejected, **Then** the UI shows the complete resultant membership, identifies the independently split command and endpoints, preserves zero partial changes, and provides a direct way to amend the proposal into one valid atomic candidate containing every required endpoint.

## Edge Cases

- Two groups are given names that differ only by capitalization or surrounding whitespace.
- A session contains terminals but has no group representation because it predates the feature.
- A new terminal is created while other groups and active navigation already exist.
- The last terminal is moved out of a group, or the only terminal in a singleton group is deleted.
- A terminal is assigned to a new group while it has incoming and outgoing transition commands.
- A group is deleted while it still contains terminals or while one of its terminals is active.
- The source or destination terminal is deleted while a transition request is awaiting approval.
- A terminal is moved to another group while it appears in an active return route.
- A stale browser submits a transition authored before the most recent group change.
- A session contains a valid terminal link but missing, duplicated, or malformed group membership data.
- Multiple transition commands connect the same pair of terminals inside one group.
- A route forms a cycle inside one group and must still unwind one return point at a time.
- An Overseer manual activation crosses group boundaries while a player transition is pending.
- A fresh broadcast starts at the first, middle, or last terminal of an ordered group.
- The Overseer attempts to reorder a group while its broadcast navigation position, pending decision, or active route depends on the previous order.
- The group, membership, authored links, pending decision, or active route changes after a destructive confirmation opens but before it is accepted.
- The Overseer submits the same destructive confirmation more than once because of a retry, double click, or stale browser.
- The Overseer cancels, dismisses, or loses the group-change confirmation before accepting it.
- The Overseer attempts to delete a singleton group without moving or deleting its terminal in the same operation.
- The same terminal is selected more than once while creating a group or proposing a bulk move.
- A legacy transition is already cross-group because its endpoints normalized into separate singleton groups, and the proposed move joins the target to the source rather than creating a new conflict.
- A legacy session contains a chain or graph of multiple dormant transition commands, so one candidate repairs some edges while other edges either remain split or are joined by the same complete proposal.

## Requirements

### Functional Requirements

- **FR-001**: The Overseer MUST be able to create, rename, and dissolve terminal groups and to move terminals between groups within the current session.
- **FR-002**: Each terminal MUST belong to exactly one terminal group at all times.
- **FR-003**: A terminal that is not coupled with another terminal MUST be represented by a singleton group containing only that terminal.
- **FR-004**: Each terminal group MUST have a stable identity and a non-empty display name unique within the session after trimming and case normalization.
- **FR-005**: Saved sessions MUST preserve terminal groups and each terminal's membership across close and reopen.
- **FR-006**: Dissolving or merging a group MUST preserve every terminal and its content while leaving no zero-member group after the operation.
- **FR-007**: The transition destination editor MUST offer only other terminals that currently share the source terminal's group.
- **FR-008**: A forward terminal transition MUST be eligible only when its source and destination exist and currently belong to the same non-empty group.
- **FR-009**: Selecting an eligible forward transition MUST create exactly one pending Overseer decision without changing the active terminal or route.
- **FR-010**: Approving a forward transition MUST revalidate the authored link, both terminals, and their shared group before changing the active terminal.
- **FR-011**: Rejecting or closing a forward transition decision MUST leave the active terminal and route unchanged.
- **FR-012**: An approved forward transition MUST add exactly one return point for its source terminal.
- **FR-013**: A backward terminal return MUST be eligible only when the active terminal and saved return terminal currently belong to the same non-empty group.
- **FR-014**: Selecting an eligible backward return MUST create exactly one pending Overseer decision without changing the active terminal or route.
- **FR-015**: Approving a backward return MUST revalidate the current route point and shared group before changing the active terminal.
- **FR-016**: An approved backward return MUST remove exactly the most recent route point.
- **FR-017**: Rejecting or closing a backward return decision MUST leave the active terminal and route unchanged.
- **FR-018**: The system MUST reject any authored or submitted transition whose endpoints are cross-group, missing from group representation, missing as terminals, or identical.
- **FR-019**: The system MUST reject a group edit that would leave an authored terminal-transition command crossing a group boundary.
- **FR-020**: The system MUST reject a group edit that would invalidate a pending transition or an active return-route pair.
- **FR-021**: A rejected group edit MUST identify the affected terminal or transition without partially applying the edit.
- **FR-022**: Group creation, membership changes, and group removal MUST be available only to the Overseer.
- **FR-023**: Sessions created before terminal grouping MUST open with their existing terminal content intact and with one singleton group representing each terminal.
- **FR-024**: Existing transition commands in an older session MUST remain authored but MUST NOT execute until both endpoints are intentionally coupled into one group.
- **FR-025**: Direct Overseer terminal activation MAY cross group boundaries and MUST retain its existing route-clearing behavior.
- **FR-026**: Existing approval behavior for ordinary and state-changing commands MUST remain unchanged.
- **FR-027**: Connected player views MUST converge on the same authoritative terminal-navigation state after each accepted same-group transition or return.
- **FR-028**: Group or terminal changes made while a decision is pending MUST be revalidated at approval time before any transition effect occurs.
- **FR-029**: Each terminal group MUST preserve one explicit, deterministic order for all of its member terminals.
- **FR-030**: Starting a fresh broadcast with any grouped terminal as its first active terminal MUST establish that terminal's position in the full ordered group without discarding members before or after it.
- **FR-031**: When a fresh broadcast starts from a non-first group member, backward navigation MUST make each preceding member reachable in reverse group order even though those terminals were not visited earlier in the broadcast.
- **FR-032**: Starting a fresh broadcast from a non-first group member MUST NOT prevent approved forward navigation from reaching every following member or from later traversing from the group's first member through the full ordered group.
- **FR-033**: Every forward or backward transition created from the initialized group position MUST require exactly one Overseer approval before changing the active terminal or navigation position.
- **FR-034**: The system MUST reject a group reorder that would invalidate the current initialized position, a pending transition, or an active route without partially applying the reorder.
- **FR-035**: Terminal groups MUST be the highest-level terminal representation, with terminals presented as members beneath groups rather than as ungrouped top-level items.
- **FR-036**: Every persisted terminal group MUST contain at least one terminal.
- **FR-037**: Creating or importing a standalone terminal MUST atomically create a singleton group that represents it.
- **FR-038**: Moving or deleting the last member of a group MUST atomically remove the empty source group while keeping every surviving terminal represented by exactly one group.
- **FR-039**: Creating a group by reassigning existing terminals, dissolving or merging groups, splitting members, moving terminals, or changing traversal order MUST require explicit Overseer confirmation before any state changes.
- **FR-040**: A destructive group-change confirmation MUST identify the proposed action, every affected group and terminal, and the resulting membership or traversal-order impact.
- **FR-041**: Cancelling, closing, or dismissing a destructive group-change confirmation MUST leave durable group data and current navigation state unchanged.
- **FR-042**: Confirming a destructive group change MUST revalidate the current groups, authored links, pending decisions, active terminal, and active route before applying the proposal.
- **FR-043**: A confirmed destructive group change MUST either apply exactly once as one complete change or be rejected without partial effects.
- **FR-044**: A repeated or stale destructive confirmation MUST produce no additional group or navigation change and MUST explain that the proposal is no longer applicable.
- **FR-045**: A rename-only change MUST preserve the group's stable identity, membership order, authored links, pending decisions, and active route.
- **FR-046**: Dissolving a multi-terminal group MUST preserve its terminals by representing each remaining member through a singleton group unless the same confirmed operation moves that member to another group.
- **FR-047**: Dissolving a singleton group without moving or deleting its terminal in the same operation MUST be unavailable or rejected.
- **FR-048**: At supported desktop viewports, the terminal organization panel MUST give group and terminal names priority over controls, preserve a visually distinct group-to-member hierarchy, and expose the complete name through wrapping or an equivalent non-ambiguous presentation without overlapping adjacent content.
- **FR-049**: Every group MUST expose its expanded or collapsed state, full display name, terminal count, and one target-specific contextual action trigger, while every visible terminal member MUST expose its explicit order, full display name, selected or live state when applicable, and one target-specific contextual action trigger.
- **FR-050**: Group and terminal rename, move, reorder, dissolve, and delete actions MUST be available from the applicable target-specific contextual menu rather than as a permanently visible row of individual buttons; destructive actions MUST remain visually differentiated and retain all existing confirmation requirements.
- **FR-051**: Collapsing or expanding a group MUST be operable by pointer and keyboard, MUST NOT mutate canonical group data or terminal selection, and MUST retain the current in-memory disclosure state across unrelated re-renders while that group continues to exist.
- **FR-052**: Authored-transition validation for a terminal-group replacement MUST use the complete proposed membership set; when a candidate joins both endpoints of an existing dormant cross-group transition, the system MUST accept that link as same-group and MUST NOT reject the candidate using their pre-change singleton memberships.
- **FR-053**: For a legacy session with multiple authored transition edges, terminal-group replacement validation MUST classify every edge independently from the complete proposed membership index; it MUST exclude every edge joined by the candidate from cross-group rejection feedback, MUST continue to identify any different edge that remains split, and, when no other invariant fails, MUST accept a candidate that joins every authored endpoint pair.
- **FR-054**: When a user-selected grouping action produces a partial candidate for a multi-edge authored graph, the Overseer UI MUST present the complete resultant terminal-to-group membership before submission, identify each edge that would remain split by command, source, and target, and provide a direct way to amend the proposal into one complete valid candidate without committing an invalid intermediate grouping; the amended candidate MUST pass unchanged through the existing atomic mutation path.

## Key Entities

- **Terminal Group**: The highest-level terminal representation: an Overseer-authored navigable area with a stable identity, a display name, and one or more member terminals in one explicit traversal order.
- **Singleton Terminal Group**: The mandatory high-level representation of one standalone terminal; it has exactly one member and no cross-terminal navigation target.
- **Terminal Membership**: The required exclusive relationship between one terminal and exactly one terminal group.
- **Group Navigation Position**: The active terminal's place in the ordered group, initialized from the first terminal activated for a fresh broadcast so terminals on both sides remain reachable.
- **Terminal Transition Command**: An authored forward link from a command in one terminal to another terminal; it is eligible only when both endpoints share a group.
- **Terminal Route**: The broadcast-scoped last-in-first-out history of approved forward transitions used to determine the next possible return.
- **Pending Terminal Navigation Decision**: One exact forward or backward request awaiting Overseer approval, including the source, destination, direction, and route context that must still be valid at resolution time.
- **Destructive Group Change Confirmation**: One reviewable proposal for a membership or traversal-order change, including the affected groups and terminals, its expected navigation impact, the authoritative state against which it was prepared, and its single-use outcome.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Saving and reopening a session with at least three groups and ten terminals restores 100% of group identities, names, order, and memberships, with every terminal assigned to exactly one group.
- **SC-002**: For every tested source terminal, the destination editor exposes 100% of eligible same-group terminals and zero cross-group, missing-membership, self, or missing terminals.
- **SC-003**: Same-group forward and backward journeys each create exactly one Overseer decision and produce zero terminal or route changes before approval.
- **SC-004**: Cross-group and malformed missing-membership forward or backward attempts create zero pending decisions, zero route changes, and zero active-terminal changes.
- **SC-005**: Changing group membership or an authored link during a pending request causes approval to make zero transition changes and leaves the current terminal usable.
- **SC-006**: A three-terminal same-group route unwinds through two approved returns in exact last-in-first-out order with no skipped or duplicated route point.
- **SC-007**: An older session without grouping data opens with 100% of its terminals and content intact, exactly one singleton group per terminal, and transitions inactive until their endpoints are intentionally coupled.
- **SC-008**: The controller and at least two observers converge on the approved active terminal and return availability within two seconds after each accepted forward or backward transition.
- **SC-009**: Regression journeys for direct Overseer activation, ordinary commands, state-changing commands, intra-terminal navigation, broadcast lifecycle, and reconnect all retain their existing outcomes.
- **SC-010**: For an ordered four-terminal group A, B, C, D, a fresh broadcast started at C can complete the approved journey C to B to A to B to C to D with zero skipped, duplicated, or cross-group terminal activations.
- **SC-011**: Across create, import, merge, split, move, delete, save, and reopen journeys, 100% of existing terminals appear beneath exactly one high-level group and zero empty groups remain.
- **SC-012**: In 100% of tested destructive create, dissolve, merge, split, move, and reorder journeys, an impact confirmation appears and zero group or navigation changes occur before acceptance.
- **SC-013**: Cancelling, closing, or dismissing any tested destructive group-change confirmation produces zero durable group changes and zero active-navigation changes.
- **SC-014**: Across stale, retried, and double-submitted destructive confirmations, each accepted proposal changes state at most once and every rejected proposal leaves 100% of the current state intact.
- **SC-015**: At 1280×720 and 1600×900 browser viewports, groups containing realistic Russian names render with zero overlapping controls, every group and terminal name is available without ellipsis-only identification, and every applicable action is reachable from exactly one target-specific menu.
- **SC-016**: In pointer and keyboard browser journeys, 100% of tested group disclosure and group/terminal action-menu operations expose the correct target and action labels, preserve selection during disclosure changes, and keep destructive actions visually separate before the existing confirmation flow.
- **SC-017**: In the tested legacy A to B repair journey, moving B into A's normalized singleton group succeeds exactly once, removes B's empty singleton group, preserves the authored command and terminal content across save and reopen, and makes A to B eligible with zero stale pre-move group IDs used for candidate validation.
- **SC-018**: In a production-fidelity version-1 session with three singleton-normalized terminals and authored S to A and A to E transitions, 100% of validation feedback reflects the complete candidate: a joined edge is never reported as split, an independently split edge is identified by its own command and endpoints, and one candidate joining S, A, and E persists and reopens with both transitions eligible.
- **SC-019**: In the exact authored-file grouping-aware journey reported by BUG-005, 100% of partial-candidate reviews and rejections show the complete proposed memberships and independently split command endpoints, offer a direct route to amend the same repair into an all-endpoint candidate, apply zero invalid intermediate changes, and allow the amended candidate to save, reopen, and leave both authored transitions eligible.

**BUG-004 evidence clarification (SC-017–SC-018)**: Closure evidence MUST correlate one unchanged authored-file copy and one identified executable build with the exact reviewed membership, the candidate observed at every production boundary, and the canonical session/revision before and after the attempted mutation; both partial-candidate feedback and complete-candidate save/reopen acceptance remain required.

**BUG-005 evidence clarification (SC-019)**: Closure evidence MUST begin with the exact grouping-aware gesture that produced the supplied screenshot, capture the editor selection and review contents before submission, and correlate the amended complete candidate through the owned rebuilt desktop runtime rather than substituting a separately constructed test-only candidate.

## Assumptions

- A standalone terminal is never ungrouped; it is represented by a singleton group and can be activated directly by the Overseer.
- Empty groups are not part of the user model. Group edits are applied atomically so each surviving group has at least one member.
- A newly created or imported terminal receives a singleton group whose initial display name follows the terminal name; the Overseer may rename or merge it later.
- Group names are compared after trimming whitespace and ignoring letter case, while group identities remain stable across renames.
- Group edits are authored as one complete change; the system does not silently delete or retarget existing transition commands to make an edit valid.
- Existing manual terminal activation is an Overseer operation rather than a player-created forward or backward transition, so it is intentionally allowed across groups.
- Group membership is durable session authoring data; pending decisions and terminal routes remain broadcast-scoped runtime data.
- Membership order defines the group's traversal order; for a fresh broadcast started from the middle, members before the starting terminal form the initial backward path, while approved forward transitions continue to build the normal return path.
- The special middle-start initialization applies to the first terminal activated for a fresh broadcast; later direct Overseer activation retains its existing route-clearing behavior.
- Creating a group from existing terminals is destructive because it changes their exclusive memberships; creating the automatic singleton group for a newly created terminal is not.
- Moving terminals, coupling or splitting groups, dissolving a group, and changing terminal traversal order are destructive group changes because they can alter navigation eligibility or sequence.
- Renaming a group is not destructive when it changes only the display name, so it does not require the destructive-change confirmation; uniqueness validation still applies.
- Deleting a group means dissolving its organizational container, not deleting its terminals or terminal content. By default, members become singleton groups unless the same confirmed operation assigns them elsewhere.
- A singleton group cannot be dissolved by itself because its terminal would become ungrouped; the Overseer must move or delete the terminal as part of the same operation instead.

## Out of Scope

- Automatically coupling terminals by inferring groups from existing terminal links; legacy terminals receive independent singleton groups instead.
- Automatically moving, deleting, or retargeting transition commands when group membership changes.
- Allowing players to create, rename, delete, or inspect private group-management controls.
- Replacing the existing Overseer approval step with automatic transition approval.
- Restricting direct Overseer activation to the current terminal group.
- Providing multi-step undo or a historical restore log for confirmed group-management changes.
