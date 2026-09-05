# Feature Specification: Shared Facility State System

**Feature Directory**: `035-facility-state-system`
**Created**: 2026-09-04
**Status**: Implemented
**Input**: Evolve Fallout Terminal into a shared facility-state system for immersive tabletop sessions.

## User Scenarios & Testing

### User Story 1 - Change shared devices through approved commands (Priority: P1)

As a player, I can request a command that moves one or more facility devices through authored transitions, so one approved action can produce a coherent world change without exposing partial or premature state.

**Why this priority**: Approved, atomic world actions are the foundation for every other facility-state experience.

**Independent Test**: Configure a command that opens a door and disables its alarm, request it as the controller, reject it once, then request and approve it; verify no state changes before approval, rejection uses the existing access-error presentation, and approval changes and saves both devices together exactly once.

**Acceptance Scenarios**:

1. **Given** a command requests valid transitions for two devices, **When** a player selects it and the approval remains pending, **Then** neither device state nor any state-driven presentation changes.
2. **Given** a pending facility action, **When** the Overseer rejects or closes the request, **Then** every device remains unchanged and players receive the current authoritative access-error presentation with an explicit rejected outcome.
3. **Given** every requested transition remains valid, **When** the Overseer approves the request, **Then** all requested device states persist as one world action, one new facility revision becomes authoritative, and the success result appears only after the save succeeds.
4. **Given** any requested device, state, transition, or precondition is missing or invalid at approval time, **When** approval is attempted, **Then** the action returns a stable structured failure and every affected device remains unchanged.
5. **Given** duplicate, stale, or concurrent copies of one request, **When** they are decided, **Then** at most one copy can commit and no device advances twice.
6. **Given** a multi-device action cannot be saved completely, **When** persistence fails, **Then** all devices retain their prior states and no successful transition is published.

---

### User Story 2 - See one facility state from every terminal (Priority: P1)

As a player, I see facility changes reflected consistently in every relevant terminal, so doors, power, alarms, and other devices feel like one shared world rather than separate terminal-local copies.

**Why this priority**: Shared projection turns approved device transitions into an immersive facility-wide experience.

**Independent Test**: Bind the same power-grid state to menu labels, command availability, visibility, and entry blocks in terminals from different groups; change the grid once and verify every open and newly opened presentation resolves from the same current state.

**Acceptance Scenarios**:

1. **Given** several terminals reference one device, **When** its state changes, **Then** connected players and the Overseer receive the same updated state-driven labels, content, availability, and visibility without reopening the session.
2. **Given** a state-driven entry is already open, **When** its referenced device changes, **Then** the open entry updates to the same effective content shown on later visits.
3. **Given** a terminal moves into or out of a group, **When** the move is saved, **Then** no facility device is cloned, reset, deleted, or retargeted.
4. **Given** both completed-command state and facility state can affect the same presentation property, **When** that property has an active facility binding, **Then** the facility binding determines that property while the command's saved completion result remains unchanged.
5. **Given** a facility binding references a missing device or state, **When** the presentation is resolved, **Then** the system exposes a stable structured reference failure and retains a usable player navigation path.

---

### User Story 3 - Author reusable devices and valid state graphs (Priority: P1)

As an Overseer, I can create reusable facility devices with stable identity, finite states, initial values, transitions, and command references through the authoring interface, so I can build reliable world behavior without editing session data by hand.

**Why this priority**: Safe authoring is required for game masters to use facility state in real sessions.

**Independent Test**: Create a reactor with offline, starting, online, and scrammed states; author allowed transitions and a multi-device command; bind its states to content in two terminals; save and reopen; then verify the complete graph and references can be inspected and used without editing JSON.

**Acceptance Scenarios**:

1. **Given** an Overseer creates a device, **When** they define its identity, display name, initial state, named states, and allowed transitions, **Then** invalid or ambiguous definitions are identified before publication.
2. **Given** a command is authored, **When** the Overseer assigns one or more device transitions, **Then** each referenced device, source state, destination state, and precondition is visible and validated.
3. **Given** content or command behavior is configured against facility state, **When** the Overseer selects a condition, **Then** the interface offers equality only against a state that belongs to the selected device.
4. **Given** an entity is referenced elsewhere, **When** the Overseer attempts to delete it or change its stable identity, **Then** the action is blocked until the references are explicitly repaired, removed, or reassigned.
5. **Given** references need repair or reassignment, **When** the Overseer applies the repair, **Then** all selected reference changes save together or none of them change.
6. **Given** a device's display name changes without changing its stable identity, **When** the session is saved, **Then** its transitions, current state, and references remain intact.

---

### User Story 4 - Run deterministic faults and recovery (Priority: P1)

As an Overseer, I can model authored diagnostic conditions on terminals and devices, so power loss, isolation, damage, corruption, and instability create predictable obstacles that players can diagnose and recover from through explicit game actions.

**Why this priority**: Diagnostic play is part of the same facility model and must obey the same durable, atomic rules from the outset.

**Independent Test**: Apply an authored network-isolated condition that blocks a command, reveals a diagnostic menu, and substitutes a damaged record variant; replay the same state several times, then recover through an approved transition and verify deterministic presentation with no random content loss or incidental world mutation.

**Acceptance Scenarios**:

1. **Given** an authored offline, unpowered, isolated, damaged, corrupted, unstable, or custom condition is active, **When** players use an affected terminal or device, **Then** only the capabilities, diagnostics, and authored content variants assigned to that condition are affected.
2. **Given** identical facility and condition states, **When** their presentation is rendered repeatedly or after restart, **Then** the same authored records and capability decisions appear each time.
3. **Given** a display instability effect plays, **When** the effect finishes, **Then** no content, device state, condition state, or saved world data changes merely because of the visual effect.
4. **Given** an active condition has an authored recovery path, **When** its approved transition, compatible holotape program, or confirmed private Overseer action succeeds, **Then** the recovery state persists and publishes through the same facility revision rules.
5. **Given** a condition blocks normal navigation or commands, **When** a player or Overseer needs to leave the affected presentation, **Then** an authoritative escape or recovery control remains available.

---

### User Story 5 - Inspect, preview, and reset the facility (Priority: P2)

As an Overseer, I can inspect dependencies, preview states and faults, and reset one device or the whole facility, so I can prepare scenes, recover from authoring mistakes, and reuse a session safely.

**Why this priority**: Operational controls make the facility model manageable once core transitions and projections work.

**Independent Test**: Inspect every reference to one device, preview each state and fault without saving, reset that device, then reset the facility; verify previews cause no mutation and each confirmed reset produces one atomic saved revision with the expected authored initial values.

**Acceptance Scenarios**:

1. **Given** a device is selected, **When** the Overseer inspects dependencies, **Then** every command, transition, terminal, entry block, availability rule, visibility rule, condition, and recovery path that references it is identifiable.
2. **Given** an Overseer previews a device state or condition, **When** they close the preview, **Then** canonical state, revisions, pending approvals, and connected player presentations remain unchanged.
3. **Given** one device differs from its authored initial state, **When** the Overseer confirms its reset, **Then** that device and its authored initial conditions reset in one persisted facility revision while unrelated devices remain unchanged.
4. **Given** multiple devices differ from their authored initial values, **When** the Overseer confirms a facility reset, **Then** every device and condition returns to its authored initial value in one atomic save.
5. **Given** any reset cannot be validated or persisted, **When** it fails, **Then** every device, condition, revision, and player presentation retains the last committed facility state.

---

### User Story 6 - Resume the same world after lifecycle changes (Priority: P2)

As an Overseer, I can stop broadcasting, restart or update the application, and reopen a session without losing the facility, so long-running campaigns resume with the same world state.

**Why this priority**: Durable continuity is essential for campaigns, after state transitions themselves are trustworthy.

**Independent Test**: Change several devices and conditions, stop and restart broadcast, restart the application, perform a supported self-update, and reopen the session; verify the same facility revision and effective presentation return each time.

**Acceptance Scenarios**:

1. **Given** approved device and condition changes were saved, **When** broadcast stops and restarts, **Then** all current values and the facility revision remain unchanged.
2. **Given** approved facility state exists, **When** the application restarts, self-updates, or reloads the session, **Then** the last fully committed facility state is restored before player presentation becomes available.
3. **Given** a version-1 session contains no facility model, **When** it is opened and saved, **Then** its existing content and behaviors remain usable without manual conversion or invented device state.
4. **Given** a pending approval exists during shutdown, terminal replacement, or session reload, **When** the lifecycle change completes, **Then** the pending request cannot later commit against the new runtime context.

---

### User Story 7 - Trace facility actions without treating logs as state (Priority: P3)

As an Overseer investigating a session, I can correlate requests, approvals, transitions, failures, and resets in retained logs, so I can diagnose what happened without exposing authored content or relying on logs to rebuild the world.

**Why this priority**: Diagnostics support complex sessions after the player and authoring journeys are complete.

**Independent Test**: Exercise approved, rejected, stale, failed, concurrent, and reset actions across multiple devices; verify correlated retained records identify safe device and revision metadata, exclude authored content, and cannot override or reconstruct canonical state.

**Acceptance Scenarios**:

1. **Given** a facility action enters the approval lifecycle, **When** it is requested, decided, committed, rejected, or fails, **Then** each applicable retained record carries the same correlation identity and safe outcome category.
2. **Given** one-device and whole-facility resets occur, **When** their records are inspected, **Then** the records identify reset scope, actor category, prior and resulting facility revisions, and safe success or failure outcome.
3. **Given** logs contain facility events, **When** they are retained or shared for support, **Then** they omit authored content, labels, secret values, and raw private errors.
4. **Given** retained logs are missing, truncated, or unavailable, **When** a session loads, **Then** canonical facility state still comes only from the saved session and normal world behavior remains available.

## Edge Cases

- A command has an empty transition list, repeats the same device, requests contradictory destinations, or creates a transition cycle across its multi-device preconditions.
- A referenced device, state, transition, condition, terminal, block, command, or recovery path is deleted or changes identity after a request is created but before approval.
- A device is already at the requested destination, has moved to another valid state, or has the expected value but a newer revision when a stale request is approved.
- Two approved actions touch disjoint devices at the same facility revision, or overlap on only some devices.
- Persistence succeeds incompletely, the application stops during save, or publication fails after the durable commit.
- A device or condition has duplicate identities, no states, an unknown initial value, unreachable states, or a recovery path that cannot be completed.
- Several facility bindings target the same presentation property, or a facility binding and a completed-command snapshot both supply it.
- A visibility or availability rule hides the only normal navigation path from a diagnostic screen.
- An authored corrupted record is empty, spans multiple pages, or is open while its controlling condition changes.
- A terminal belongs to several logical views over time or is moved while players are connected.
- A reset is requested while a transition approval is pending, or an old approval arrives after a reset has advanced the facility revision.
- A version-1 session uses ordinary commands, legacy state-changing commands, terminal transitions, hacking, player roles, and entry content without any facility definitions.

## Requirements

### Functional Requirements

- **FR-001**: The authoring experience MUST let an Overseer define reusable facility devices with a stable identity, display name, device kind, authored initial state, current state, and a finite set of named states.
- **FR-002**: Each facility device transition MUST identify one device, one allowed source state, one destination state, and an authored transition name.
- **FR-003**: Facility validation MUST reject duplicate identities, duplicate state names within a device, unknown initial states, and transitions whose source or destination does not belong to the device.
- **FR-004**: Facility devices and their current states MUST be owned by the session-wide facility rather than by a terminal or terminal group.
- **FR-005**: Moving a terminal between groups MUST NOT clone, reset, delete, or retarget any facility device or reference.
- **FR-006**: A terminal command MUST be able to request one or more authored device transitions as one world action through the existing private approval lifecycle.
- **FR-007**: A requested world action MUST NOT mutate device state, condition state, saved revisions, or state-driven presentation before approval.
- **FR-008**: Rejection or dismissal of a requested player world action MUST leave facility state unchanged and MUST reuse the current authoritative access-error presentation with an explicit rejected result.
- **FR-009**: Approval MUST revalidate every requested device, transition, expected source state, precondition, and relevant revision against the current canonical facility state.
- **FR-010**: A multi-device world action MUST apply all requested transitions or leave every affected device unchanged.
- **FR-011**: A successful world action MUST advance the canonical facility revision once and MUST perform at most one durable commit.
- **FR-012**: A persistence failure MUST leave the prior facility state authoritative and MUST NOT publish a successful result or partial projection.
- **FR-013**: Repeated decisions for the same correlated request MUST NOT produce more than one committed world action.
- **FR-014**: Concurrent world actions with overlapping affected devices MUST serialize through the centralized player-action transaction so at most one action can commit from the same expected state.
- **FR-015**: Missing references, invalid transitions, failed preconditions, stale revisions, conflicts, and persistence failures MUST return stable structured failure categories with safe affected identities.
- **FR-016**: Facility failure handling MUST NOT depend on matching human-readable error message text.
- **FR-017**: An Overseer MUST be able to bind a menu label variant to equality between one device and one known state.
- **FR-018**: An Overseer MUST be able to bind an EntryContent block variant to equality between one device and one known state.
- **FR-019**: An Overseer MUST be able to bind command availability to equality between one device and one known state.
- **FR-020**: An Overseer MUST be able to bind simple content visibility to equality between one device and one known state.
- **FR-021**: Authoring validation MUST reject a presentation or behavior condition whose compared state does not belong to its referenced device.
- **FR-022**: Connected player and Overseer presentations MUST refresh state-driven labels, blocks, availability, and visibility from the newly committed facility state without requiring session reload.
- **FR-023**: When a facility binding and a completed-command snapshot affect the same presentation property, the facility binding MUST determine that property's effective value.
- **FR-024**: The existing completed-command result and completion lifecycle MUST remain authoritative for command-result presentation even when that command also requests facility transitions.
- **FR-025**: Device-driven content MUST be derived from canonical facility state and authored variants rather than persisted as another completed-command snapshot.
- **FR-026**: A diagnostic condition MUST have a stable identity, authored category, affected terminal or device, authored initial status, current status, and explicitly selected effects.
- **FR-027**: The authored diagnostic categories MUST include offline, unpowered, network-isolated, storage-damaged, authorization-corrupted, and display-unstable conditions, and MUST allow additional named authored faults with the same bounded behavior model.
- **FR-028**: A diagnostic condition MUST be able to block explicitly selected capabilities without changing unrelated capabilities.
- **FR-029**: A diagnostic condition MUST be able to expose an authored diagnostic menu or diagnostic content path.
- **FR-030**: A diagnostic condition MUST be able to select authored partial-record variants without randomly deleting or permanently altering source content.
- **FR-031**: Repeated rendering of the same facility and condition state MUST produce the same content, visibility, availability, and navigation result.
- **FR-032**: Visual corruption or instability effects MUST NOT mutate facility state, condition state, authored content, or durable session data merely because an effect plays.
- **FR-033**: Recovery from an active condition MUST occur only through an approved authored device transition, a compatible authored holotape program, or a confirmed private Overseer action.
- **FR-034**: A compatible holotape recovery program MUST be limited to selecting authored recovery actions and MUST NOT execute arbitrary scripts or expressions.
- **FR-035**: Approved device states, condition states, and facility revisions MUST persist across broadcast stop, application restart, supported self-update, and session reload.
- **FR-036**: Pending world-action approvals MUST be transient and MUST become incapable of committing after their broadcast, session, or application runtime context ends.
- **FR-037**: The Overseer MUST be able to inspect every direct dependency on a selected device, state, transition, or condition before changing its identity or deleting it.
- **FR-038**: Previewing a device state or condition MUST NOT change canonical state, revisions, pending approvals, saved data, or connected player presentation.
- **FR-039**: A confirmed single-device reset MUST restore that device and its device-scoped conditions to authored initial values in one facility revision without changing unrelated devices.
- **FR-040**: A confirmed whole-facility reset MUST restore all devices and diagnostic conditions to authored initial values through one atomic durable commit.
- **FR-041**: A failed reset MUST leave all device states, condition states, revisions, and player projections at their last committed values.
- **FR-042**: Every player presentation affected by device state or diagnostic conditions MUST retain an authoritative escape path, and the Overseer MUST retain a private recovery action when authored restrictions make normal player progress unusable.
- **FR-043**: The Overseer interface MUST support creating, editing, validating, and inspecting devices, states, transitions, conditions, references, and recovery paths without manual JSON editing.
- **FR-044**: Deleting or changing the stable identity of a referenced facility entity MUST be blocked until every affected reference is explicitly removed, repaired, or reassigned.
- **FR-045**: An explicit multi-reference repair or reassignment MUST save all selected reference changes together or leave every reference unchanged.
- **FR-046**: Changing a facility entity's display name while preserving its stable identity MUST preserve its current state, transitions, conditions, and references.
- **FR-047**: Publication of authored facility changes MUST validate the complete affected reference and recovery graph before those changes become available to players.
- **FR-048**: Facility requests, approvals, rejections, transitions, structured failures, and resets MUST emit retained runtime-log records with a shared correlation identity for each action.
- **FR-049**: Facility log records MUST identify safe device identities, action category, outcome category, and relevant facility revisions without recording authored labels, content, secrets, or raw private errors.
- **FR-050**: Retained logs MUST remain diagnostic evidence and MUST NOT be read as a source for facility state, replay, recovery, or conflict resolution.
- **FR-051**: Ordinary commands, legacy state-changing commands, terminal transitions, hacking, player roles, and existing command-driven EntryContent completion behavior MUST remain usable without facility configuration.
- **FR-052**: Version-1 sessions without facility data MUST load and save without manual conversion, invented devices, or changes to their existing player-visible behavior.
- **FR-053**: Facility state loading MUST reject or safely quarantine incomplete committed facility data rather than exposing a partially restored world to players.
- **FR-054**: Reset, transition, and recovery actions MUST invalidate any older pending request whose expected facility revision or affected state is no longer current.

## Key Entities

- **Facility**: The session-wide world-state boundary that owns reusable devices, diagnostic conditions, the current facility revision, and their authored initial values independently of terminal grouping.
- **Facility Device**: A reusable world object such as a door, turret, power grid, reactor, ventilation system, alarm, robot pod, elevator, or network segment. It has stable identity, descriptive metadata, a finite state set, an initial state, and one current state.
- **Device State**: One authored named value available to a specific device. State equality can select presentation variants and command behavior.
- **Device Transition**: One explicitly allowed movement from a source state to a destination state for a device, with any authored preconditions and recovery role.
- **World Action**: One correlated request that groups one or more device transitions for approval, validation, atomic persistence, revision, publication, and a structured outcome.
- **Facility Revision**: The monotonic identity of the last fully committed facility state, used to detect stale or conflicting actions.
- **Diagnostic Condition**: A deterministic authored fault attached to a terminal or device, with an initial and current status, selected blocked capabilities, diagnostic paths, authored content variants, and explicit recovery actions.
- **State Binding**: An authored equality relationship from one device state to a menu label, EntryContent block variant, command availability decision, or visibility decision.
- **Recovery Path**: An explicit approved transition, compatible holotape action, or private Overseer action that can clear or change a diagnostic condition.
- **Structured Facility Result**: A stable success, rejection, or failure category with correlation and safe affected identities, separate from localized player-facing message text.

## Success Criteria

### Measurable Outcomes

- **SC-001**: In 100 consecutive approved multi-device action tests, every action either persists all requested transitions in one facility revision or leaves every affected device unchanged.
- **SC-002**: Across pending, rejected, dismissed, failed-save, stale, duplicate, and conflicting request tests, zero facility states change before a valid approval and zero partial successes reach player presentation.
- **SC-003**: With at least five terminals across three groups referencing the same device, every connected presentation shows the same effective label, block content, availability, and visibility within one second of a committed transition under normal local-network conditions.
- **SC-004**: Moving 100 terminals between groups produces zero device creations, deletions, resets, identity changes, or reference retargets.
- **SC-005**: Replaying each supported diagnostic condition 100 times from identical authored state produces identical capability, content, and navigation outcomes and zero persistent mutations caused solely by visual effects.
- **SC-006**: After broadcast restart, application restart, supported self-update, and session reload, 100% of lifecycle checks restore the last complete facility revision and its device and condition values before player interaction resumes.
- **SC-007**: Individual reset tests restore exactly one selected device and its device-scoped conditions, while whole-facility reset tests restore 100% of devices and conditions through one committed revision.
- **SC-008**: In authoring tests, every broken reference, unknown state, invalid transition, conflicting binding, and unusable recovery graph is identified before publication, and cancellation leaves the prior authored graph unchanged.
- **SC-009**: Every supported facility action outcome produces a correlated retained record, while seeded authored content, labels, secrets, and raw private errors produce zero matching values in retained logs.
- **SC-010**: All maintained version-1 compatibility sessions open and save successfully, and their ordinary commands, legacy state changes, terminal transitions, hacking, roles, and EntryContent behavior pass unchanged.
- **SC-011**: In 1,000 duplicate, stale, and concurrent request attempts against overlapping devices, each originating expected state produces at most one committed transition and no split facility state.
- **SC-012**: In every authored fault configuration, a player can leave an affected presentation or an Overseer can invoke a private recovery action without editing session data.

## Assumptions

- A facility belongs to the loaded session, so all terminals and groups in that session reference the same canonical device identities and current values.
- Device kinds provide authoring organization and presentation vocabulary; they do not introduce device-specific scripting or hidden transition behavior.
- Each simple presentation or behavior rule compares one device's current state for equality with one of that device's authored states.
- When a facility binding is configured for a property also affected by legacy completed-command state, the facility binding has precedence for that property; the legacy completion result and lifecycle remain intact.
- A single-device reset includes conditions scoped directly to that device. Terminal-scoped and facility-wide conditions change only through their recovery path, explicit private action, or whole-facility reset.
- Private Overseer recovery and reset actions require explicit confirmation but do not enter the player command approval queue because the Overseer is already the authority making the decision.
- Publication after a durable commit may be retried from canonical saved state; publication failure does not roll back an already confirmed durable revision or make logs authoritative.
- Version-1 remains the compatible session format, with absent facility data interpreted as no configured facility rather than a default facility.

## Out of Scope

- Arbitrary scripting or a general expression language.
- Random hardware degradation or nondeterministic fault activation.
- Permanent destruction of authored terminal content.
- Combat resolution or automatic determination of a character's repair success.
- Smart-lighting integration, physical smoke control, and scene-cue automation.

## Verbatim Constraints

- Existing command-driven content blocks remain named `EntryContent` blocks.
