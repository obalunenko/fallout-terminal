# Data Model: Command-Driven Entry Content State

## Authored Entry Content Block

One ordered text section inside an entry.

| Field | Type | Rules |
|---|---|---|
| ID | String | Required, stable, no surrounding whitespace, bounded like existing authored IDs, and unique across every block in one terminal. |
| Initial text | String | May be empty or whitespace-only; retained exactly and bounded by the existing entry-body limit. |

Blocks are ordered by their position in the containing entry. Moving or reordering a block retains its ID. Block IDs are independent from content-node IDs but share terminal-local uniqueness rules within their own identity set.

## Entry Content Change

An optional command-owned transition for one block.

| Field | Type | Rules |
|---|---|---|
| Block ID | String | Resolves exactly one authored block in the same terminal as the command. |
| Completed text | String | May be empty or whitespace-only; retained exactly and bounded by the existing entry-body limit. |

The entire change is explicitly present or absent. Completed text does not act as a presence signal.

## State-Changing Command Configuration

The existing state-changing configuration gains one optional entry-content change.

| Field | Existing/new | Rules |
|---|---|---|
| Completed command name | Existing | Required and used after successful execution. |
| Confirmation text | Existing | Required and shown only to the Overseer approval flow. |
| Entry-content change | New, optional | At most one block target and completed text. Absence preserves existing state-changing-command behavior. |

Within one terminal, no two command configurations may target the same block. A completed command's target and completed block text remain locked until the command is reset; deleting the owning command removes the complete frozen state.

## Command Execution State

The existing durable map remains the only owner of completed command and block outcomes:

```text
Terminal.commandStates[commandID]
├── completed command name
├── result text
└── optional frozen entry-content change
    ├── block ID
    └── completed text
```

The frozen entry-content change is copied from the validated authored command configuration at the first successful execution. Later edits to initial block text, command completed name, or command result do not change the already-frozen outcome. A repeated execution returns the same snapshot without another write.

## Entry Representation

An entry has two mutually exclusive persisted forms:

| Form | Legacy description | Explicit blocks | Player-visible value |
|---|---|---|---|
| Legacy | Present or empty | Absent | Description exactly as stored; an empty description renders an empty entry. |
| Explicit | Empty | One or more ordered blocks | Effective block texts joined in order with `\n\n`. |

The Overseer may display a non-empty legacy description as a draft first block, but it creates a stable block ID and clears the legacy description only when the user accepts a block edit. Deleting the last explicit block produces an empty entry; omission versus an explicit empty block array is not durable state. Merely loading, viewing, or saving unrelated content does not convert it.

## Validation Relationships

Validation operates on the complete terminal tree so commands may appear before or after their target entry.

1. Collect every content node using the existing node-ID and tree-depth/count rules.
2. Collect every explicit entry block by terminal-scoped block ID.
3. Reject blocks on folder or command nodes and reject an entry that has both a non-empty legacy description and explicit blocks.
4. Bound each block value and the composed entry text by the existing entry-body byte limit.
5. Resolve every authored entry-content change to exactly one block in the same terminal.
6. Reject a second authored command that targets a block already owned by another command, naming the conflicting command.
7. Resolve every frozen change from `commandStates` to its command and block.
8. Require a frozen change to match the completed command's still-authored target and reject multiple frozen owners.
9. Continue accepting legacy state-changing configurations and snapshots that contain no entry-content change.

## Derived Overseer Relationship View

The Overseer derives both human-readable relationship labels from the canonical command configuration; neither label is persisted.

| View | Canonical source | Display |
|---|---|---|
| Command target option | Entry tree plus the command's block ID | Containing entry name, current one-based block position, and a normalized preview of authored initial text. |
| Entry block owner | Reverse index of authored command targets by block ID | Targeting command's current displayed name; the stable command ID remains a non-visible element attribute for interaction and testing. |

The text preview collapses line breaks and repeated whitespace for display, is bounded to 48 Unicode code points with an ellipsis when truncated, and uses an explicit `ПУСТО` marker when the authored text is empty or whitespace-only. The entry name and block position remain visible even when previews are identical. The reverse ownership index is rebuilt whenever the node form renders, so command or entry renames, block reordering, text edits, completion, and reset cannot leave a stale label. The effective completed command name is shown while a command snapshot exists; otherwise its authored initial name is shown.

## State Transitions

| Current state | Action | Result |
|---|---|---|
| Initial command, initial block | Player selects command | Existing approval-pending presentation; no durable state change. |
| Pending | Overseer rejects/closes | Pending clears through the existing lifecycle; command and block remain initial. |
| Pending | Overseer approves, save succeeds | One command snapshot containing command and block outcomes is stored; one authoritative update exposes both completed values. |
| Pending | Validation/storage fails | Prior session and runtime remain authoritative; no completed value is published. |
| Completed | Player selects and approval succeeds | Frozen result is shown; no new snapshot or block transition is created. |
| Completed | Overseer confirms individual reset | The one map entry is removed; command and owned block return to authored initial values. |
| Any completed states | Overseer confirms terminal reset | The terminal map is cleared; all commands and owned blocks return to authored initial values. |
| Completed | Overseer edits target/removes target block | Save is rejected until explicit reset; ordinary text edits and moves preserving IDs remain allowed. |
| Completed | Overseer deletes owning command | Snapshot is pruned with the command; the formerly owned block uses its initial text. |

## Clone and Ownership Rules

- Content-tree clones copy block slices and nested authored changes.
- Session, terminal-target, runtime, and command-state clones copy nested frozen changes; no pointer may alias the canonical owner.
- The session service owns durable snapshots and document revisions.
- The control service owns process transactions and publication ordering.
- The live service owns detached effective projection and composition.
- Browsers own only editor drafts or rendered authoritative projections.
