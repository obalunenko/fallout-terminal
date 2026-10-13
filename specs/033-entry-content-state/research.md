# Research: Command-Driven Entry Content State

## Decision: Store the block outcome inside the command snapshot

**Rationale**: The existing `commandStates` value is inserted only after a complete session candidate is validated and durably replaced. Adding an optional frozen entry-content change to that same value makes the command presentation and block outcome one state: execution inserts one map value, individual reset deletes one value, terminal reset clears one map, and command deletion prunes one value.

**Alternatives considered**:

- Add a second terminal-level entry-state map. Rejected because execution, stale-save merge, reset, and rollback would need to coordinate two owners and could expose partial state.
- Derive completed block text from the current command configuration every time. Rejected because later authoring edits would rewrite already-completed game-world state.

## Decision: Use an explicitly present nested entry-content change

**Rationale**: A command target is optional, while an intentionally empty or whitespace-only completed block value is valid. A nested `EntryContentChange` provides explicit presence and keeps the target ID and completed text together in both authored configuration and the frozen snapshot.

**Alternatives considered**:

- Use parallel scalar target/content fields and infer presence from a nonblank target. Rejected because absence has different meaning from an empty completed value and the contract requires explicit presence.
- Make every state-changing command target an entry block. Rejected because existing state-changing commands without content effects must remain compatible.

## Decision: Make block IDs unique within a terminal

**Rationale**: A command and its target always live in one terminal. A terminal-scoped block ID is sufficient to resolve exactly one target, lets entries and blocks move within that terminal without rewriting references, and avoids storing a redundant parent entry ID that could disagree after a move.

**Alternatives considered**:

- Use list position as identity. Rejected because reordering would retarget commands.
- Store both entry ID and block ID. Rejected because the entry ID is redundant under terminal-wide uniqueness and creates an avoidable consistency pair.
- Make block IDs session-global. Rejected because commands cannot cross terminal boundaries and broader uniqueness adds no user value.

## Decision: Preserve legacy descriptions until an explicit block edit

**Rationale**: `EntryContent.description` remains field 1 and old JSON remains version 1. An entry with no explicit blocks renders its legacy description exactly; the Overseer presents a non-empty legacy description as a draft first block and creates a stable block ID only when the user accepts a block edit, then clears the legacy description. An entry with neither description text nor blocks remains an empty entry. This avoids unstable synthetic IDs and avoids rewriting unrelated data merely because a session was opened and saved.

**Alternatives considered**:

- Auto-migrate every entry on load. Rejected because opening or saving would silently transform existing user data.
- Keep a non-empty legacy description and blocks active together. Rejected because two sources would make ordering and rendering ambiguous.
- Assign a constant implicit block ID to all legacy entries. Rejected because IDs must be unique within a terminal.

## Decision: Compose effective blocks on the server and keep the public player contract unchanged

**Rationale**: Block IDs, initial alternatives, and completed alternatives are private authoring/state details. The live service can apply frozen changes to a detached tree and join ordered effective block text with `\n\n` into the existing public entry description. The current client already replaces the authoritative tree, retains entry navigation, keys reveal state by text, and re-paginates changed open-entry content.

**Alternatives considered**:

- Add blocks and alternate text to the public player protobuf. Rejected because it leaks unnecessary authoring structure and forces a client migration without changing the visible result.
- Compose blocks in browser JavaScript. Rejected because it would duplicate canonical state rules outside Go and expose private alternatives.
- Concatenate without a separator. Rejected because distinct authored sections would run together and require hidden whitespace conventions.

## Decision: Route resets through the coordinator's persistence seam

**Rationale**: Command approval already calls the session store while holding the coordinator transaction and publishes only after durability. The existing application reset path writes the session first and refreshes runtime in a second step, leaving a race window. Coordinator reset methods can reuse the already-defined store operations, install the canonical returned snapshot map, clear affected command presentation, retain open-entry navigation, and publish exactly once after success.

**Alternatives considered**:

- Keep application-level write then refresh. Rejected because players may temporarily retain a completed block after the durable reset and concurrent actions can observe mismatched state.
- Add entry-only reset methods. Rejected because they can separate a block outcome from its owning completed command.

## Decision: Lock a completed command's block target until reset

**Rationale**: Frozen name/result values already remain authoritative until reset, and the command mode is already locked while completed. Locking its block target and completed block text follows the same mental model. Backend merge validation rejects removal or retargeting of a still-frozen block, while deleting the owning command prunes the complete snapshot and restores initial block content.

**Alternatives considered**:

- Silently reset the command when its target changes. Rejected because it bypasses the explicit reset confirmation and makes a durable world-state change look like ordinary authoring.
- Preserve the old frozen target while allowing a new authored target. Rejected because one command would appear to own two targets and deletion/conflict feedback would become misleading.

## Decision: Reuse the pinned toolchain and add no dependency

**Rationale**: Existing protobuf generation, session storage, control transactions, live projection, Wails bridge, browser rendering, and Playwright fixtures already provide every required capability. The feature needs additive schema and application changes, not a new runtime, state library, editor framework, or transport.

**Alternatives considered**:

- Add a frontend state-management dependency for block editing. Rejected because the editor is local to the existing node form and established plain-module state is sufficient.
- Introduce a new RPC or desktop method. Rejected because current save, approval, reset, session-state, and subscription paths already carry the required authoritative transitions.

## Decision: Derive bidirectional authoring labels from the command target

**Rationale**: The command configuration remains the only authored owner of a block transition. During editor rendering, one traversal can produce command selector options labeled with the containing entry name, the block's one-based position, and a normalized preview of its authored text. The same configuration can be indexed by block ID so a targeted block displays the command's current effective name. Identical or whitespace-only text remains unambiguous because position is always present and an empty preview uses an explicit marker.

**Alternatives considered**:

- Persist a targeting command ID on each block. Rejected because it duplicates the relationship, creates two values that can disagree, and complicates moves, deletion, and stale-save handling.
- Display only internal block and command IDs. Rejected because the accepted clarification requires human-readable identification without an ID lookup.
- Label targets with text alone. Rejected because identical, empty, or long blocks would be ambiguous and text edits would look like target changes even though stable identity is unchanged.

## Decision: Use one EntryContent-side native dialog with an isolated draft

**Rationale**: The Overseer already uses accessible native dialogs and keeps the complete session document in browser state. A single block command-assignment dialog can switch between selecting an eligible existing command and fully authoring a new one, while holding every field in dialog-local controls until Apply. Cancel, Escape, or close therefore discards the draft without touching the content tree or calling `SaveSession`.

**Alternatives considered**:

- Expand every block row with all command fields. Rejected because it makes the entry editor dense, repeats a large form for every block, and weakens the requested dialog workflow.
- Mutate the session as each dialog field changes and restore it on cancel. Rejected because rollback is error-prone and could leak transient invalid ownership into autosave or rerendering.
- Add a new desktop method dedicated to block assignment. Rejected because the operation is ordinary session authoring and the existing whole-document save already validates and persists it.

## Decision: Keep command configuration canonical during reverse assignment

**Rationale**: EntryContent initiates the workflow but does not gain a command reference. Assigning an existing command writes its nested entry-content change; atomic reassignment clears the old uncompleted owner's nested change and sets the new owner's change in one candidate; removal clears only the old nested change. This preserves the existing one-owner domain validation and makes both editor directions converge from the same source.

**Alternatives considered**:

- Persist the owner command ID on the block. Rejected because it recreates the duplicate relationship already rejected by the original design.
- Save removal and assignment as separate operations. Rejected because the intermediate document is either unowned or conflicting and a failure between writes would violate the clarified atomic authoring behavior.
- Allow reassignment or removal of a completed owner. Rejected because it would separate the authored target from its frozen durable outcome without the required reset.

## Decision: Create a complete command in an explicitly selected folder

**Rationale**: New-command mode collects the existing required state-changing-command fields, the completed block content, and one destination folder from the current terminal. Apply allocates one stable node ID, builds the complete command with the selected block target, appends it to that folder using the established tree insertion convention, and performs one session save. Requiring the folder avoids surprising root placement and does not broaden command targets across terminals.

**Alternatives considered**:

- Always insert beside the entry. Rejected by clarification because the Overseer must choose the destination folder.
- Insert at the terminal root. Rejected because it ignores the authored content hierarchy and the explicit destination decision.
- Create a placeholder and navigate to the command editor for completion. Rejected because cancellation could leave invalid partial content and the clarified workflow requires full configuration within the dialog.
