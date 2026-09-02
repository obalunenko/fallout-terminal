# Feature Specification: Command-Driven Entry Content State

## Clarifications

### Session 2026-09-03

- Q: What identifying details should the Overseer see for each command-to-block relationship? → A: The command shows the entry name, block position, and text preview; the entry block shows the targeting command name.

## User Scenarios & Testing

### User Story 1 - Change visible entry content through a command (Priority: P1)

As a player, I can execute an approved command that changes part of an information entry, so the terminal reflects the resulting game-world state the next time I read that entry.

**Why this priority**: The player-visible content change is the core value of the feature and must behave as one reliable game action.

**Independent Test**: Open an entry with two authored content blocks, execute a command that targets one block, approve it, and verify that the targeted block changes while the other block and the rest of the entry remain unchanged.

**Acceptance Scenarios**:

1. **Given** an entry contains a block targeted by an uncompleted command, **When** the Overseer approves that command and the change is saved successfully, **Then** all players see the block's completed content and the command's own completed presentation.
2. **Given** an entry contains targeted and untargeted blocks, **When** a command changes its target block, **Then** every untargeted block retains its current content and position.
3. **Given** a player is already viewing the affected entry, **When** the command completes, **Then** the open entry updates to the same effective content that players see when opening it again.
4. **Given** the Overseer rejects or closes the approval request, **When** the request finishes, **Then** neither the command nor its target entry block changes state.
5. **Given** the command's new state cannot be saved completely, **When** execution fails, **Then** neither the command nor its target entry block changes state or presentation.
6. **Given** a command has already completed, **When** it is selected again and approved under the existing command lifecycle, **Then** its saved result is shown without applying the entry block change again.

---

### User Story 2 - Author independent entry blocks and command targets (Priority: P1)

As an Overseer, I can divide an entry into independently changeable blocks and configure a state-changing command to replace one specific block, so several commands can affect the same entry without overwriting one another.

**Why this priority**: Stable, unambiguous targets are required before command-driven content changes can be safe to use.

**Independent Test**: Author one entry with two blocks, link a different command to each block, save and reopen the session, then execute the commands in either order and verify that both completed block values coexist.

**Acceptance Scenarios**:

1. **Given** an Overseer edits an entry, **When** they add and arrange multiple blocks, **Then** each block can be authored with initial content and retains a stable identity through saving and reopening.
2. **Given** a state-changing command is being configured, **When** the Overseer selects an entry block and provides its completed content, **Then** the command retains that target and content after saving and reopening the session.
3. **Given** two commands target different blocks in the same entry, **When** both commands complete in either order, **Then** both block changes are visible and neither completion overwrites the other.
4. **Given** a block is already targeted by another command, **When** the Overseer tries to assign a second command to that same block, **Then** the conflicting configuration is rejected with an explanation identifying the existing command.
5. **Given** a command targets a block, **When** the Overseer attempts to delete that block or its containing entry, **Then** deletion is prevented until the command is reassigned, changed to a non-targeting mode, or deleted.
6. **Given** a targeted entry, block, or command is renamed or moved without changing its identity, **When** the session is saved, **Then** the target relationship and any completed state remain intact.
7. **Given** a command targets an entry block, **When** the Overseer views the command, **Then** the target is identified by the entry name, block position, and a preview of the block's authored text.
8. **Given** an entry block is targeted by a command, **When** the Overseer views that block in the entry editor, **Then** the targeting command is identified by name.

---

### User Story 3 - Reset command and entry states safely (Priority: P1)

As an Overseer, I can reset a command-driven entry change through the same individual and terminal-wide controls used for command state, so I can restore the authored starting state without leaving the command and entry inconsistent.

**Why this priority**: Reset is essential for correcting game flow and replaying a state-changing event while preserving a single authoritative state.

**Independent Test**: Complete two commands that affect different blocks, reset one command, and verify that only its command and block return to their initial states; then reset all terminal command states and verify that every remaining command-driven block returns to its initial content.

**Acceptance Scenarios**:

1. **Given** a completed command changed an entry block, **When** the Overseer confirms the individual command reset, **Then** that command and its target block return to their initial states in one saved change.
2. **Given** multiple completed commands changed multiple blocks, **When** the Overseer resets one command, **Then** blocks owned by the other completed commands remain unchanged.
3. **Given** a terminal has completed commands with and without entry targets, **When** the Overseer confirms the terminal-wide reset, **Then** all command states and all command-driven entry block states in that terminal return to their initial values in one saved change.
4. **Given** an individual or terminal-wide reset confirmation is open, **When** the Overseer cancels it, **Then** no command or entry block state changes.
5. **Given** a reset cannot be saved completely, **When** the reset fails, **Then** players and the Overseer continue to see the last successfully saved command and entry states.

---

### User Story 4 - Preserve state across sessions and live updates (Priority: P2)

As an Overseer, I can rely on command-driven entry changes surviving normal application and broadcast lifecycles, so the displayed terminal content remains consistent with the saved game world.

**Why this priority**: Persistence and publication make the feature dependable beyond the immediate command execution, after the core interaction works.

**Independent Test**: Complete commands affecting two blocks, end and restart the broadcast, close and reopen the session, and verify that both effective block states and both command states are restored; then reset them and verify the initial content is restored after another reopen.

**Acceptance Scenarios**:

1. **Given** a command and its target block completed successfully, **When** the broadcast ends, the active terminal changes, or the application closes, **Then** their completed states remain part of the saved session.
2. **Given** a saved session contains completed entry block states, **When** the session is reopened and its terminal is broadcast, **Then** players see the completed block content and completed command presentation.
3. **Given** an existing session contains entries without independently stateful blocks, **When** it is opened, **Then** its entry text renders unchanged and begins with no command-driven block state.
4. **Given** the Overseer publishes authored changes to a live terminal, **When** existing completed states still refer to valid commands and blocks, **Then** those states remain effective rather than being silently discarded.

## Edge Cases

- An entry with no blocks renders as an empty entry and cannot be selected as a command target until it has a block.
- Empty or whitespace-only initial or completed block content is allowed when intentionally authored; block identity and target validity do not depend on visible text.
- A command cannot target an entry, block, or terminal that does not exist in the same terminal content tree.
- Two commands may target different blocks within one entry, but two commands may not target the same block.
- Duplicate block identities within a terminal make the session invalid rather than allowing an ambiguous command target.
- Target labels remain unambiguous when blocks have identical or empty text by including the block position; an empty preview is displayed explicitly rather than as missing information.
- Removing a command that has completed also removes its owned entry block state, matching the existing lifecycle of command-owned state.
- Renaming or moving a command, entry, or block preserves state; deleting and recreating it produces a new identity and does not inherit the deleted state.
- Reloading a legacy session with a single entry description preserves the rendered text and introduces no completed state.
- A player disconnecting or reconnecting during a successful change receives the latest fully saved effective entry content, never a partial command/block combination.

## Requirements

### Functional Requirements

- **FR-001**: The authoring experience MUST allow an entry's displayed content to be organized as an ordered collection of independently identifiable blocks.
- **FR-002**: Each entry block MUST retain its identity, authored initial content, and position across saving and reopening a session.
- **FR-003**: A state-changing command MUST be able to target no more than one entry block in the same terminal and define the completed content for that block.
- **FR-004**: The authoring experience MUST reject a command target that does not resolve to exactly one existing entry block in the command's terminal.
- **FR-005**: The authoring experience MUST prevent more than one command from targeting the same entry block.
- **FR-006**: Multiple commands MUST be able to target different blocks within the same entry and complete in any order without overwriting one another's state.
- **FR-007**: Successful execution of a command with an entry target MUST save the command's own completed state and the target block's completed state as one indivisible change.
- **FR-008**: Rejected, canceled, invalid, or unsuccessfully saved execution MUST leave both the command state and target block state unchanged.
- **FR-009**: Re-executing an already completed command MUST NOT apply a second entry block transition or create an additional saved state change.
- **FR-010**: Player views MUST compose each entry from its ordered blocks using the latest successfully saved state of each block.
- **FR-011**: A successfully changed block MUST become visible to every connected player, including players already viewing its entry, without altering unrelated blocks or navigation state.
- **FR-012**: An individual Overseer command-state reset MUST reset that command and its owned entry block state as one indivisible saved change.
- **FR-013**: A terminal-wide Overseer command-state reset MUST reset all command states and all command-owned entry block states for that terminal as one indivisible saved change.
- **FR-014**: Canceling or failing any reset MUST preserve the last successfully saved command and entry block states.
- **FR-015**: The authoring experience MUST prevent deletion of a targeted block or its containing entry until every targeting command is removed or reassigned.
- **FR-016**: Renaming or moving an existing command, entry, or block MUST preserve its target relationship and completed state.
- **FR-017**: Deleting a command MUST remove both its saved command state and any entry block state owned by that command.
- **FR-018**: Completed command and entry block states MUST survive broadcast termination, terminal switching, application restart, and session reopening.
- **FR-019**: Existing sessions whose entries contain only a single description MUST retain the same player-visible text and MUST begin without command-driven entry state.
- **FR-020**: A session with missing, duplicate, cross-terminal, or otherwise ambiguous entry block targets MUST be rejected with actionable feedback instead of being loaded or saved with unpredictable behavior.
- **FR-021**: The command authoring experience MUST identify a selected entry block target by its containing entry name, block position, and a preview of its authored text.
- **FR-022**: The entry authoring experience MUST identify each targeted block's targeting command by the command's name.

## Key Entities

- **Entry Content**: The information page players open from a terminal menu. It owns an ordered set of content blocks.
- **Entry Content Block**: An independently identifiable portion of an entry, with authored initial content, an effective current content, and a stable position among sibling blocks.
- **State-Changing Command**: An Overseer-approved terminal action with its own initial and completed presentation. It may own a transition for one entry content block.
- **Entry Block State**: The durable completed value for one block, owned by the command that produced it and applied only after the command and block change are saved together.
- **Terminal State**: The collection of durable command and entry block states scoped to one terminal and affected by the terminal-wide reset.

## Success Criteria

### Measurable Outcomes

- **SC-001**: In 100% of approved, successfully saved targeted-command executions, players observe both the completed command presentation and completed target block content, with no partial outcome.
- **SC-002**: At least five commands can target five distinct blocks in one entry and complete in any tested order with all five resulting block values visible simultaneously.
- **SC-003**: Individual reset testing restores exactly one command and its one owned block while preserving 100% of unrelated completed command and block states.
- **SC-004**: Terminal-wide reset testing restores every command and command-owned entry block in the selected terminal through one confirmed action and one saved revision.
- **SC-005**: Existing sessions with single-description entries reopen with identical player-visible entry text in all compatibility fixtures.
- **SC-006**: Across rejection, cancellation, invalid-target, save-failure, and reset-failure tests, zero cases expose a partial command/block state to players or the Overseer.
- **SC-007**: Completed entry block content remains correct after broadcast restart, terminal switching, application restart, and session reopening in all lifecycle tests.
- **SC-008**: In every configured command-to-block relationship, the Overseer can navigate from the command to the exact entry block and from the entry block to the targeting command without consulting an internal identifier.

## Assumptions

- Entry content blocks are ordered text sections; rich media, conditional layout, and nested block structures are outside this feature.
- One command owns at most one block transition, and one block may be owned by at most one command. More complex multi-step or competing transitions are outside this feature.
- Commands that target entry content continue to use the existing Overseer approval lifecycle and existing completed command name/result behavior.
- Individual and terminal-wide command reset controls are the authoritative reset paths; a separate entry-only reset would risk leaving the owning command inconsistent and is not introduced.
- Empty text can be a deliberate game-world result, so target validation relies on stable identity rather than content.
- Existing session compatibility is preserved by treating a legacy entry description as one initial block with no completed state.

## Verbatim Constraints

- The persisted entry content entity is named `EntryContent`.
