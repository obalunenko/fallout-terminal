# Contract: Public Player Facility Projection

## Public Data Boundary

The player continues to receive one server-authoritative effective terminal tree and ordered compound updates. Device definitions, current raw states, diagnostic identities, transition graphs, preconditions, recovery programs, dependency reports, and private failure details remain absent from the public protocol.

Two additive fields support behavior that cannot be represented by resolved text alone:

```proto
enum TerminalPresentationEffect {
  TERMINAL_PRESENTATION_EFFECT_UNSPECIFIED = 0;
  TERMINAL_PRESENTATION_EFFECT_DISPLAY_UNSTABLE = 1;
}

message ContentCommand {
  string text = 1;
  optional bool available = 2;
}

message LiveTerminal {
  string terminal_id = 1;
  string terminal_name = 2;
  ContentNode tree = 3;
  int32 hack_level = 4;
  string intro_text = 5;
  NavigationState navigation = 6;
  PublicHackState hacking = 7;
  optional CommandExecutionPresentation command_execution = 8;
  optional TerminalNavigationPresentation terminal_navigation = 9;
  ControllerTerminalPresentation controller_presentation = 10;
  repeated TerminalPresentationEffect effects = 11;
}
```

Absence of ContentCommand.available means available, preserving existing clients and sessions. The explicit false value displays the command as unavailable and prevents local selection. The server remains authoritative and rejects a stale or crafted request that targets an unavailable command.

## Effective Tree Contract

For every publication or reconnect snapshot, the server builds a detached effective tree:

1. Clone the authored terminal content and immutable command completion snapshots.
2. Apply existing completed-command labels and EntryContent block values.
3. Apply the matching facility device-state binding to each bound name, block, visibility rule, or command availability rule.
4. Apply active deterministic diagnostic overrides.
5. Remove invisible nodes, mark unavailable commands, resolve diagnostic paths, compose EntryContent blocks, and derive safe presentation-effect enums.
6. Revalidate navigation and controller presentation against the effective tree before adapting it to the public protobuf.

The authored tree, command snapshots, facility state, and conditions remain unchanged by projection. Map iteration, client viewport, animation timing, or random values cannot influence the resulting text, node set, availability, or effect flags.

## Approval Lifecycle

- Selecting an available facility-backed command uses the existing Navigate command request and controller authorization.
- Accepted selection creates the existing PENDING command presentation for all players; facility state and effective content remain at the prior revision.
- Overseer rejection or dismissal produces the existing REJECTED command presentation and current authoritative access-error record.
- Approval success publishes a newer complete terminal projection only after the combined command/facility session mutation is durable.
- Approval failure caused by stale state, missing reference, failed precondition, conflict, invalid configuration, or persistence produces the same access-error presentation. Private results and logs retain the typed reason.
- Back, Enter, Escape, and Backspace retain the established controller acknowledgement behavior for a resolved rejection. Observers cannot dismiss it locally.
- Re-selecting a completed state-changing command follows the existing approval and frozen-result behavior; it does not repeat the facility action or advance facility revision.

No second player error component, facility mutation method, or optimistic world transition is added.

## Availability and Visibility

- A visible unavailable command remains in menu order with its authoritative name and an inaccessible visual state.
- The player client does not send command activation for a locally unavailable item.
- The server evaluates the same current facility rules and rejects stale or forged activation.
- An invisible node is omitted from the effective tree and cannot be reached through navigation requests.
- If a newer facility projection hides or blocks the current selection or open entry, the server repairs navigation to the nearest valid parent or root in the same update.
- Back and acknowledgement are not subject to authored capability blocks.
- A diagnostic path exposed by an active condition is an ordinary authored node in the effective tree and disappears deterministically when the condition clears.

## Diagnostic Presentation

- Record damage appears only as an authored deterministic EntryContent block substitution.
- Display instability is driven only by the safe TerminalPresentationEffect enum.
- The client may animate or style the current authoritative effect but cannot edit text, delete content, alter availability, change navigation authority, or submit a world mutation because an effect played.
- A newer authoritative revision cancels or replaces obsolete visual effects.
- Reconnecting or resizing may replay rendering from the current snapshot, but it produces the same effective records and does not change world state.

## Synchronization and Lifecycle

- One committed facility action produces one newer coordination/publication revision containing the complete effective terminal state.
- Controller and observers converge on the same projected tree, navigation repair, and effect set.
- An already open EntryContent page keeps its stable entry identity when still visible and receives the new effective description through the normal reveal and pagination path.
- Reconnect receives the current complete projection before later updates.
- Broadcast restart and terminal reactivation project from the shared session facility rather than from terminal-local caches.
- Moving terminals between groups changes route membership only and never changes the public facility projection for a given terminal and facility revision.
- A stream delivery failure does not roll back durable world state; the next snapshot converges from the committed session.

## Public Compatibility

- Existing clients interpret absent command availability as available and ignore unknown additive terminal-effect fields according to protobuf compatibility rules.
- Sessions without facility data produce the same effective tree and behavior as before the feature.
- Ordinary commands, terminal transitions, hacking, player roles, command completion, pending/rejected screens, EntryContent pagination, and controller/observer semantics keep their existing routes and message variants.
- Public ActionResult retains its established safe reasons. Facility-specific failure detail remains private and in redacted diagnostics.
