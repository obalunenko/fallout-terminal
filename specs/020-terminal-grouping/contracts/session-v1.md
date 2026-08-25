# Contract: Portable Session JSON v1 and Persistence Protobuf

## Protobuf additions

`fallout.terminal.persistence.v1` adds:

```proto
message TerminalGroup {
  string id = 1;
  string name = 2;
  repeated string terminal_ids = 3;
}

message Session {
  int32 version = 1;
  string name = 2;
  optional string player_config = 3;
  repeated Terminal terminals = 4;
  repeated TerminalGroup terminal_groups = 5;
}
```

Fields 1–4 of `Session` remain unchanged. Field 5 is additive; no existing number or name is reused. Generated Go and ECMAScript code is regenerated through the repository's pinned tooling and is never edited manually.

## Portable JSON shape

The additive top-level field is `terminalGroups`:

```json
{
  "version": 1,
  "name": "Vault 76",
  "terminals": [
    { "id": "a", "name": "A", "hackLevel": 0, "introText": "", "root": {} },
    { "id": "b", "name": "B", "hackLevel": 0, "introText": "", "root": {} }
  ],
  "terminalGroups": [
    {
      "id": "main-route",
      "name": "Main Route",
      "terminalIds": ["a", "b"]
    }
  ]
}
```

The example abbreviates terminal roots only for readability; existing terminal JSON requirements remain unchanged.

## Canonical shape

- Every group has a stable unique ID, a normalized unique nonblank name, and at least one terminal ID.
- Every terminal ID appears exactly once across the complete group list.
- `terminalIds` order is the authoritative traversal order.
- A standalone terminal is represented by a normal group whose `terminalIds` contains only that terminal.
- An empty terminal session has an empty group list; a non-empty canonical session has at least one group.

## Legacy compatibility

- A version-1 document with terminals and a missing or empty `terminalGroups` value loads without losing content.
- The active in-memory document normalizes that legacy shape to one deterministic singleton group per terminal.
- Normalization never infers groups from terminal-transition commands and does not rewrite the source file merely because it was opened.
- The next explicitly accepted save persists the canonical singleton groups.
- A partially populated, duplicated, empty-member, unknown-member, or otherwise malformed explicit group set is rejected rather than silently repaired.
- Compatible unknown top-level, terminal, and node fields remain preserved by the explicit JSON adapter.
- Session version stays 1; persistence does not switch to protobuf binary or generic ProtoJSON.
- Legacy transition commands remain authored, but cross-singleton links are player-ineligible until the Overseer intentionally groups both endpoints.

## Terminal lifecycle normalization

Generic session saves preserve canonical groups for existing terminals so stale frontend data cannot overwrite group management. Within the same accepted mutation:

- a genuinely new or imported terminal receives one singleton group;
- a deleted terminal is removed from its group;
- an empty group caused by terminal deletion is removed; and
- the resulting document must satisfy exact-one membership and compatible authored-reference validation; an existing cross-group legacy link remains dormant rather than blocking an unrelated content save.

Only the trusted group mutation may otherwise replace memberships or traversal order.
