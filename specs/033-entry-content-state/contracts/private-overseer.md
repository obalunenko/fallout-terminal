# Contract: Private Overseer Entry Block Authoring and Reset

## Desktop Surface

No new desktop method or event is introduced. The trusted Overseer continues to use:

- `SaveSession` for accepted authoring changes;
- `ResolveCommandExecution` for exact pending approval or rejection;
- `ResetCommandState` with `{terminalId, commandId}` for individual reset;
- `ResetTerminalCommandStates` with `{terminalId}` for terminal-wide reset;
- `UpdateLiveTerminal` for explicit authored-content publication; and
- `session-state` for the canonical session document and document revision after durable state mutation.

The exact desktop allowlist count and public/private separation remain unchanged. New persistence fields travel only inside the already-contracted session document.

## Entry Block Editor Contract

Stable structure for accessibility and browser verification:

- `#entryContentBlocks`: ordered block-list container.
- `#btnAddEntryContentBlock`: appends one new block with a generated terminal-unique ID.
- `.entry-content-block[data-block-id]`: one stable block row.
- `[data-entry-block-content]`: editable initial-text control for its containing block.
- `[data-entry-block-owner][data-command-id]`: read-only ownership text for a targeted block, containing the targeting command's current displayed name.
- `[data-entry-block-action="move-up"]`: moves the existing block one position earlier.
- `[data-entry-block-action="move-down"]`: moves the existing block one position later.
- `[data-entry-block-action="delete"]`: requests deletion of the existing block.

Editing text or moving a row preserves its `data-block-id`. Empty and whitespace-only text is accepted. A legacy description appears as a draft block; its real ID is generated only when an edit is applied.

The ownership text reads `ИЗМЕНЯЕТСЯ КОМАНДОЙ: <command name>` and is absent for an untargeted block. It uses the completed command name while a durable snapshot exists and the authored initial name otherwise. Deleting a targeted block, an entry containing one, or an ancestor subtree containing one is rejected through `#nodeValidationError`, and the message identifies every targeting command that must first be reset/reassigned or deleted.

## Command Target Contract

The existing state-change fields remain, with these additions:

- `#fldEntryBlockTarget`: optional selector whose value is a stable block ID and whose options include blocks from the current terminal only.
- `#fldCompletedEntryBlockContent`: completed text for the selected block; may be empty or whitespace-only.

Each target option's accessible text uses `<entry name> · БЛОК <one-based position> · <preview>`. The preview collapses line breaks and repeated whitespace, keeps at most 48 Unicode code points, and adds an ellipsis when truncated. Empty or whitespace-only authored text uses `ПУСТО`, so visually identical content never removes the entry-and-position identifier.

Selecting no target removes the complete nested entry-content change. Selecting a block already owned by another command is rejected through `#nodeValidationError` with the owning command identified. A completed command's target and completed block text are disabled until its state is reset; the disabled selector keeps the exact target label visible and existing frozen-state presentation shows the effective owned block outcome.

## Reset Contract

- Existing confirmation, cancellation, pending-disable, canonical-revision, and error surfaces remain in use.
- Individual confirmation removes the command and owned block outcome together.
- Terminal-wide confirmation removes every command and owned block outcome in that terminal together.
- Cancel and backend failure cause no local mutation.
- A successful response is accepted only when its document revision is newer and its canonical `commandStates` shape proves the requested snapshot removal.
- The coordinator publishes any active-terminal player update only after the same durable result is installed.

## Authoring and Publication Rules

- The Overseer mirrors target/conflict/deletion validation for immediate feedback; backend session validation remains authoritative.
- Target options and entry-block ownership labels are rebuilt from the current entry tree, command configurations, and command snapshots whenever the node form renders; no block-to-command back-reference is persisted.
- Accepted edits autosave through the existing session path.
- Ordinary authoring changes do not implicitly publish a live terminal.
- Explicit publication preserves valid frozen snapshots and sends only effective text to players.
- Browser state stores no completed entry state outside the canonical session document received from the backend.
