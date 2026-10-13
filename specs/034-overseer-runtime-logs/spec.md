# Feature Specification: Overseer Runtime Logs

**Feature Directory**: `034-overseer-runtime-logs`
**Created**: 2026-09-03
**Status**: Draft
**Input**: Give the Overseer useful runtime logs for player connections and roles, command requests and decisions, hacking activity, and access to logs from released packages.

## User Scenarios & Testing

### User Story 1 - Trace players and command decisions (Priority: P1)

As an Overseer investigating a problem, I can follow when a player connects, which role that player receives, which command the player requests, how the request is decided, and when the player disconnects, so I can reconstruct the control flow without reproducing the session.

**Why this priority**: Player authority and command approval are the main coordination path, and missing evidence there prevents the Overseer from distinguishing user error from a system problem.

**Independent Test**: Connect an unassigned player, select active and observer roles in turn, submit commands that are approved and declined, then disconnect; verify that the records form one correlated, chronological account with the correct role at each event.

**Acceptance Scenarios**:

1. **Given** a player connection becomes available to the system, **When** its current role is resolved, **Then** a connection record identifies the logical player session and its role as active, observer, or unassigned.
2. **Given** a connected player changes role, **When** the new role takes effect, **Then** a role-change record identifies the same logical player session and both the previous and new roles.
3. **Given** a player requests a command, **When** the request is accepted for Overseer review, **Then** a request record identifies the logical player session, current role, terminal, command, and one correlation identifier without including command content or output.
4. **Given** a pending command is approved or declined, **When** the decision takes effect, **Then** a decision record uses the same correlation identifier and identifies the decision and resulting outcome.
5. **Given** a player connection ends, **When** the system processes the disconnection, **Then** one disconnection record identifies the same logical player session and its last known role.

---

### User Story 2 - Retrieve diagnostics from a released application (Priority: P1)

As an Overseer running an installed release, I can locate and open the application's retained logs without starting it from a development terminal, so I can collect diagnostic evidence from the environment where a problem actually occurred.

**Why this priority**: Complete records have little support value if they disappear with the packaged process output or their location is unknown to the person operating the release.

**Independent Test**: Install and run a release package, generate representative activity, use the application-provided log access mechanism, and verify that the current and previous run records can be read after restart without a development console.

**Acceptance Scenarios**:

1. **Given** the application is launched from a supported release package, **When** runtime activity occurs, **Then** the same required records available during development are retained in a stable per-user location outside the installed package.
2. **Given** retained logs exist, **When** the Overseer invokes the log access mechanism, **Then** the operating system opens or reveals the current log location and the active log is identifiable.
3. **Given** the operating system cannot open or reveal the log location, **When** the Overseer invokes the access mechanism, **Then** the application presents a safe, actionable failure and the exact intended location remains available for manual navigation.
4. **Given** the application restarts after a clean or unexpected stop, **When** the Overseer accesses logs, **Then** the previous run remains distinguishable from the current run within the configured retention boundary.

---

### User Story 3 - Diagnose hacking puzzle activity (Priority: P2)

As an Overseer investigating a hacking problem, I can follow the safe lifecycle and outcomes of a puzzle without exposing its password or board content, so I can tell whether the puzzle started, accepted interactions, succeeded, failed, reset, or was abandoned.

**Why this priority**: Hacking is a complex player interaction that needs its own evidence, after the shared connection and command-decision trail is established.

**Independent Test**: Exercise a puzzle through guesses, dud removal, failure, reset, success, and interruption; verify that every transition is represented with safe metadata and that neither candidates nor solutions appear in the logs.

**Acceptance Scenarios**:

1. **Given** a hacking puzzle begins, **When** the puzzle becomes active, **Then** a record identifies the terminal, puzzle instance, initiating logical player session, current role, difficulty, and allowed attempts.
2. **Given** a player submits a guess, **When** the system evaluates it, **Then** a record identifies the puzzle instance, whether the interaction was accepted, its outcome category, and attempts remaining without recording the guessed or correct word.
3. **Given** a dud is removed or attempts are replenished, **When** the effect is applied, **Then** a record identifies the puzzle instance and effect category without recording board content.
4. **Given** a puzzle succeeds, fails, resets, or ends because the active terminal or controller changes, **When** that transition takes effect, **Then** one terminal record identifies the final state and a safe reason category.

---

### User Story 4 - Keep retained diagnostics safe and manageable (Priority: P3)

As an Overseer, I can use retained logs for support without exposing secrets or allowing diagnostics to grow without limit, so the feature remains safe for normal long-running use.

**Why this priority**: Safety and bounded storage are required before release, but they build on the core diagnostic coverage and access journey.

**Independent Test**: Generate high-volume activity containing distinctive secret and gameplay markers across multiple restarts, then verify bounded retention, complete event correlation, zero forbidden values, and unchanged application behavior.

**Acceptance Scenarios**:

1. **Given** runtime operations contain credentials, passwords, puzzle words, terminal content, command output, character names, or private provider errors, **When** records are retained, **Then** none of those raw values appear.
2. **Given** log volume reaches its configured retention boundary, **When** new records are written, **Then** older retained data is removed predictably while the current log remains writable and identifiable.
3. **Given** the preferred log location is unavailable or becomes unwritable, **When** the application emits a record, **Then** gameplay and Overseer commands continue with a safe diagnostic warning through an available fallback channel.
4. **Given** multiple players act nearly simultaneously, **When** their records are inspected, **Then** timestamps, event categories, run identity, and correlation identifiers allow each flow to be reconstructed without ambiguity.

## Edge Cases

- A connection ends before any role is selected; the disconnection must report the unassigned role rather than omit the event.
- A browser reconnects to the same logical player session, or multiple transport connections temporarily represent one logical session.
- A role changes while a command request or hacking puzzle is pending.
- A command request is duplicated, superseded, declined after the requester disconnects, approved but fails during execution, or receives the same decision twice.
- A hacking interaction is rejected without consuming an attempt, or a puzzle is replaced by a terminal switch, broadcast stop, controller loss, or reset.
- Events from multiple players and puzzles arrive at the same displayed timestamp.
- The retained-log directory is missing, read-only, full, or replaced by an unsafe filesystem object.
- Rotation or cleanup occurs while the application is running or while the Overseer opens the active log.
- An older retained log is truncated or malformed; current-run logging and access must remain usable.
- A packaged application is installed in a read-only location or upgraded while prior logs exist.

## Requirements

### Functional Requirements

- **FR-001**: The application MUST record each logical player connection with a timestamp, application-run identifier, stable logical-session identifier, and resolved role.
- **FR-002**: The application MUST record each logical player disconnection exactly once with the same logical-session identifier and its last known role.
- **FR-003**: The application MUST record every effective role transition with the logical-session identifier and the previous and new roles.
- **FR-004**: The application MUST record each command request accepted for Overseer review with a unique correlation identifier, requesting logical session, current role, terminal identifier, and safe command identifier.
- **FR-005**: The application MUST record each approval or decline with the originating request correlation identifier, decision, and resulting execution outcome when applicable.
- **FR-006**: Duplicate, superseded, invalid, or execution-failed command requests MUST be distinguishable through safe outcome categories without recording raw command content or output.
- **FR-007**: The application MUST record hacking puzzle start, accepted or rejected interaction, dud-removal, attempt-replenishment, success, failure, reset, and interruption events when those events occur.
- **FR-008**: Each hacking record MUST include the application-run identifier, puzzle correlation identifier, terminal identifier, acting logical session and role when available, event category, and safe outcome metadata applicable to that event.
- **FR-009**: Hacking records MUST NOT contain candidate words, the correct password, memory-board content, or other data that can reveal the puzzle solution.
- **FR-010**: Required runtime records MUST use consistent event categories, timestamps, severity, and correlation fields so connection, command, and puzzle flows can be reconstructed in order.
- **FR-011**: Development runs and supported release packages MUST retain the same required runtime-event coverage in a stable per-user location outside application installation files.
- **FR-012**: The Overseer interface MUST provide a log access action that opens or reveals the current retained-log location without requiring a development terminal.
- **FR-013**: When the operating system cannot open or reveal the retained-log location, the application MUST show a safe actionable error that includes the exact intended location.
- **FR-014**: Retained logs MUST distinguish application runs and preserve at least the current and immediately previous run after both clean and unexpected shutdown, subject to bounded retention.
- **FR-015**: Retention MUST impose a documented finite storage boundary and remove older log data without deleting or blocking the active log.
- **FR-016**: Logging or retained-storage failures MUST NOT change player authority, command decisions, puzzle behavior, application lifecycle ordering, or existing user-visible results.
- **FR-017**: Retained records MUST exclude provider tokens, player and generated passwords, character names, session and terminal content, command output, puzzle solution data, and unredacted private dependency errors.
- **FR-018**: Automated checks MUST cover connection and role transitions, command request decisions and exceptional outcomes, hacking lifecycle events, packaged-log access, retention, fallback behavior, and forbidden-value redaction.

## Key Entities

- **Diagnostic Record**: One timestamped operational fact with an application-run identity, event category, severity, safe attributes, and optional correlation identifiers.
- **Application Run**: One launch-to-stop interval that groups records and distinguishes current activity from retained evidence produced by previous executions.
- **Logical Player Session**: The stable runtime identity used to correlate a player's connections, role transitions, command requests, and hacking actions without relying on a character name.
- **Command Request Trace**: The lifecycle of one command request from receipt through approval or decline and, when approved, its execution outcome.
- **Hacking Puzzle Trace**: The safe lifecycle of one puzzle instance and its interactions, effects, and terminal outcome without solution-bearing content.
- **Retained Log Set**: The active and historical application-run records kept in the per-user diagnostic location under a finite retention boundary.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A test journey covering connect, unassigned-to-active role selection, command request, approval, execution outcome, active-to-observer transition, declined request, and disconnect produces every expected event exactly once with matching session and request correlations.
- **SC-002**: A hacking test journey covering start, rejected interaction, accepted guess, dud removal, attempt replenishment, failure, reset, success, and interruption produces every applicable event exactly once with matching puzzle correlation.
- **SC-003**: Target-aware package and path checks for every supported desktop operating system confirm that retained logs resolve to per-user application data outside installed package files.
- **SC-004**: After a clean restart and a simulated unexpected stop, both the current and immediately previous application runs remain identifiable and readable.
- **SC-005**: A high-volume retention test remains within the documented storage boundary while preserving a writable, identifiable active log.
- **SC-006**: Diagnostic captures seeded with distinctive credentials, character names, command output, terminal content, puzzle candidates, and solutions contain zero forbidden raw values.
- **SC-007**: Existing player connection, authority, command approval, hacking, persistence, startup, and shutdown acceptance journeys continue to pass with retained logging enabled and with retained storage unavailable.
- **SC-008**: On each matching host where optional packaged UI evidence is collected, an Overseer reaches the active retained-log location in no more than two interactions and identifies current-run records within 10 seconds; unavailable host evidence is reported as not run.

## Assumptions

- A player's selected role is the effective active, observer, or unassigned role; connection records use the resolved role, and later selections are represented as role transitions.
- Logical-session and terminal identifiers are safe operational identifiers; display names, character names, authored content, and command output remain private.
- Identifying a command means recording its stable safe identifier, not its authored label, request payload, or result text.
- Hacking diagnostics describe event and outcome categories plus counts such as attempts remaining; they do not reproduce the player-facing hacking activity text.
- The packaged-app access mechanism opens or reveals retained logs through the operating system and exposes the intended path on failure; a full searchable log viewer inside the application is not required.
- Existing application logging remains the baseline for lifecycle and trusted Overseer operations; this feature extends it rather than replacing its event vocabulary or safety rules.
