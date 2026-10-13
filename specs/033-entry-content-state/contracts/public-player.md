# Contract: Public Player Entry Presentation

## Message Surface

The public player protobuf remains unchanged:

```proto
message ContentEntry {
  string description = 1;
}
```

No block ID, authored initial alternative, completed alternative, command ownership, session document, or reset capability is exposed to players. `proto/fallout/terminal/player/v1/terminal.proto` and its generated browser module should remain byte-for-byte unchanged.

## Effective Description

Before adapting the live tree to the player contract, the server produces a detached effective tree:

1. For every explicit block, start with its authored initial text.
2. Replace only blocks referenced by valid frozen command snapshots.
3. Preserve authored block order.
4. Join effective block values with exactly two newline characters (`\n\n`).
5. Store the composed value in the cloned entry description used by the public projection.
6. For a legacy entry with no explicit blocks, preserve its existing description exactly.

The authored tree and durable snapshots remain unchanged by projection.

## Update and Navigation Semantics

- Approval publishes no completed description until the combined command/block snapshot is durably saved.
- Successful execution publishes one newer authoritative terminal update containing the completed command presentation and effective entry description.
- Rejection, cancellation, invalid target, malformed store result, or storage failure publishes no partial completed description.
- A player already viewing the affected entry retains its authoritative entry ID and receives the changed description through the same full-tree update.
- Existing client reveal identity detects the changed text and restarts rendering; pagination preserves or clamps the authoritative page index to the new page count.
- Unrelated entries, blocks, menu position, and navigation state remain unchanged.
- Reconnect and broadcast/terminal reactivation receive a complete effective snapshot derived from durable state.
- Individual and terminal-wide resets publish initial effective text only after reset durability succeeds.

## Authorization and Ordering

- Players continue to request commands through the existing controller-only unary action path.
- Entry content state cannot be mutated through a public method.
- Approval and reset remain Overseer-only capabilities.
- The existing server subscription revision rules govern ordering, regression rejection, disconnection, and reconnection.
