# Public Player Contract: `fallout.terminal.player.v1`

This is the complete public application RPC surface. The generated player graph imports no private, persistence, configuration, native-path, tunnel, credential, or secret-hacking schema. Static HTML, CSS, fonts, images, generated bundles, and sounds remain ordinary same-origin resources.

## Service and procedure identifiers

Service: `fallout.terminal.player.v1.PlayerService`

| Procedure | Cardinality | Connect path | Responsibility |
|---|---|---|---|
| `Subscribe` | server streaming | `/fallout.terminal.player.v1.PlayerService/Subscribe` | Recognition, physical-stream attachment, first complete snapshot, compound authoritative updates. |
| `SelectCharacter` | unary | `/fallout.terminal.player.v1.PlayerService/SelectCharacter` | Current-broadcast character claim for a connected recognized unassigned session. |
| `Navigate` | unary | `/fallout.terminal.player.v1.PlayerService/Navigate` | Connected active-controller navigation action. |
| `Guess` | unary | `/fallout.terminal.player.v1.PlayerService/Guess` | Connected active-controller password-word or filler target. |
| `ActivatePattern` | unary | `/fallout.terminal.player.v1.PlayerService/ActivatePattern` | Connected active-controller current unused generation-bound special pattern. |
| `SetPresentation` | unary | `/fallout.terminal.player.v1.PlayerService/SetPresentation` | Connected active-controller semantic menu, page, or hacking-preview presentation. |
| `SoundManifest` | unary | `/fallout.terminal.player.v1.PlayerService/SoundManifest` | Allowlisted same-origin sound discovery. |

No public health, reflection, diagnostics, capability discovery, client count, runtime status, server information, tunnel status, or generic command procedure is registered. Unsupported public service/procedure paths return Connect `unimplemented` rather than falling through to static content.

## Stream messages

`SubscribeRequest`:

| Protobuf field | Generated ECMAScript property | Type | Rule |
|---|---|---|---|
| `recognition_handle` | `recognitionHandle` | `optional string` | Absence creates a first-time session; present blank/oversized/malformed is `invalid_argument`; well-formed unknown/expired is replaced. |

`SubscriptionMessage` uses `oneof payload`:

- `snapshot`: `PersonalizedSnapshot`, legal only as the first application value;
- `update`: `CompoundUpdate`, legal only after the snapshot and with a strictly greater revision.

`PersonalizedSnapshot` contains required `recognition_handle`, `revision`, `player_state`, and `terminal_presentation`. `TerminalPresentation` uses `oneof presentation` with `live_terminal` and `no_live_terminal`; exactly one is set.

`LiveTerminal.controller_presentation` is the complete process-local semantic view owned by the current controller. It carries a bounded `context_key` and exactly one of `none`, menu target, information-page index, or hacking target/pattern. Raw pointer coordinates, DOM focus, viewport geometry, reveal progress, and per-document audio state are not public protocol data.

`CompoundUpdate` contains `revision` and zero or more complete `player_state`, `terminal_presentation`, `navigation`, and `hacking` message fields. Message presence is authoritative: absent means unchanged, never clear and never partial patch. An explicit clear uses `terminal_presentation.no_live_terminal`.

## Personalized player values

`PlayerState` contains the current logical-session ID, fallback name, optional assigned character, `PlayerRole`, `PlayerPhase`, optional broadcast ID, optional active terminal ID, and the complete personalized roster availability list. It exposes neither another session's identity/presence/claim nor a raw physical connection ID.

Enums define `UNSPECIFIED` at zero. Stable view semantics are:

- role: `unassigned`, `active`, `observer`;
- phase: `no-broadcast`, `selecting`, `waiting`, `controlling`, `observing`;
- roster availability: `available`, `claimed`.

Generated enum values map exhaustively to those established browser strings; unknown/`UNSPECIFIED` required values are rejected or treated as unavailable, never guessed.

## Terminal, navigation, and hacking values

`LiveTerminal` is complete: terminal identity/name, recursive public content tree, hack level, intro text, complete navigation projection, and optional complete public hacking projection. Content-node variants use `oneof`, not a string discriminator plus incompatible parallel fields.

`NavigateRequest` uses `oneof action`:

- `back` with no required node;
- `enter` with a node ID;
- `command` with a node ID;
- `entry` with a node ID.

`GuessRequest` uses `oneof target`:

- `word_id` for one public candidate;
- `filler` with bounded column and character offsets.

`ActivatePatternRequest` carries protobuf field `pattern_id` whose generated ECMAScript property and protobuf JSON name are exactly `patternId`. It remains opaque and generation-bound; the client echoes it and never derives generation identity or coordinates from it.

`PublicHackState` contains level, word length, maximum/remaining attempts, solved/failed, complete log, two complete rendered columns, public word placements, and current public patterns. It never contains the secret word, candidate text lookup, private generation ID, random source, future outcome, or private used-pattern key.

## Mutation requests and results

Each mutation request directly contains its typed fields; there is no generic mutation context or command payload.

| Request | Required semantic fields after decoding |
|---|---|
| `SelectCharacterRequest` | recognition handle, request ID, broadcast ID, character ID |
| `NavigateRequest` | recognition handle, request ID, broadcast ID, terminal ID, exactly one action variant |
| `GuessRequest` | recognition handle, request ID, broadcast ID, terminal ID, exactly one target variant |
| `ActivatePatternRequest` | recognition handle, request ID, broadcast ID, terminal ID, exact `patternId` property |
| `SetPresentationRequest` | recognition handle, request ID, broadcast ID, terminal ID, current context key, exactly one complete semantic presentation variant |

`SetPresentationRequest` repeats the semantic context key as a stale-context precondition. Only the currently connected assigned controller may commit it; observer, stale-context, invalid-target, and replay-conflict requests are rejected without advancing the canonical revision.

Every mutation returns `ActionResult` with request ID, acceptance, `ActionReason`, and authoritative revision. The stable external reason mapping is exactly:

`accepted`, `invalid-session`, `stale-broadcast`, `unassigned`, `not-controller`, `controller-disconnected`, `stale-terminal`, `invalid-action`, `conflict`, `duplicate`.

Stale generation, used pattern, absent/non-actionable/solved/failed/exhausted puzzle, and a well-formed invalid target map to `invalid-action` unless `stale-terminal` is more specific.

## Validation and authority order

1. HTTP encoded-body limit.
2. Connect framing, decompression, and 4 KiB uncompressed protobuf message limit.
3. Protobuf decoding, `oneof` legality, required enum/presence checks, and finite field/category bounds.
4. Recognition-handle structural validation.
5. For mutations, logical-session lookup and required active subscription relationship.
6. Request ID and retained replay check.
7. Current broadcast and procedure-specific fields.
8. Character-selection availability, or shared-action assignment/controller/presence/terminal/generation rules.
9. Canonical gameplay mutation and randomness, only for the one accepted winner.

Structural rejection occurs before the application adapter and canonical service. Domain rejection returns `ActionResult`. No rejected path creates a session, replacement handle, mutation, revision, logical publication, replay entry for a nonexistent session, attempt, or random-source advancement.

## Finite bounds

| Value | Limit |
|---|---|
| Uncompressed protobuf request message, every RPC | `4 KiB` (`4096` bytes) |
| Encoded HTTP request body | `8 KiB` (`8192` bytes) |
| Recognition handle, request ID, broadcast ID, generation ID | 128 UTF-8 bytes |
| Terminal ID, character ID, node/navigation target, guess target, `patternId` | 256 UTF-8 bytes |
| Presentation context key and semantic target | 256 UTF-8 bytes |
| Sound category adapter input | 32 bytes and one allowed enum value |

Unknown protobuf fields count toward the 4 KiB message limit. Compressed messages that expand past it return `resource_exhausted` before canonical invocation.

## Connect error mapping

| Condition | Code |
|---|---|
| Malformed protobuf, illegal/missing variant, prohibited required `UNSPECIFIED`, invalid present Subscribe handle, invalid unary handle/request ID/required identity/target/category | `invalid_argument` |
| Message, encoded body, or decompressed content over its limit | `resource_exhausted` |
| Unsupported public service or procedure | `unimplemented` |
| Request/stream cancellation | `canceled` |
| Temporary service/listener shutdown unavailability | `unavailable` |
| Unexpected non-domain failure | `internal` |

Errors may contain safe corrective guidance. They never contain request bytes, legacy JSON, recognition handles, credentials, native paths, stack traces, private identities/candidates, secret words, future outcomes, tunnel policy, or dependency errors.

## Same-origin, Basic Auth, and static resources

- The page, generated client, RPC paths, sound manifest, and sound/static assets share the current origin on local port `3690` and `fallout-terminal.ngrok.app`.
- Browser `Origin`, when present, must match the request host; no wildcard CORS response is emitted.
- Invalid Basic Auth is rejected by protected public access with HTTP `401` before static or RPC capability handling.
- `SoundManifestRequest` contains only the `SoundCategory` enum. Successful values return that category and sorted safe relative assets for `ambient`, `hack-good`, `hack-bad`, `menu-focus`, `single`, `multiple`, `enter`, or `charscroll`, filtered to `.mp3`, `.wav`, `.ogg`, `.m4a`, or `.webm`.
- `GET /api/sounds/{folder}` is removed. The browser resolves manifest paths against its current origin and keeps discovery/prefetch optional and non-blocking.

## Recognition, multi-tab, reconnect, and pending behavior

The first-tab Web Locks/storage-lease election is retained around the generated `Subscribe` call. The winning tab stores the first snapshot's handle before releasing the election; other tabs then subscribe with that value. The only stored application value is the handle.

Every subscription begins with a complete snapshot. The reconnect delay remains exactly three seconds. A reconnect never regenerates a puzzle or replays ambient transitions, hacking outcome cues, stale actions, or accepted-action cues. Accepted browser actions remain pending until both their unary result and an applicable authoritative stream revision are present in either order; rejected results clear immediately.

## Superseded protocol identifiers

Final generated descriptors, handlers, client code, fixtures, and active operational documentation contain no procedures or message types named `SESSION_HELLO`, `CHARACTER_SELECT`, `NAV_ACTION`, `HACK_GUESS`, `HACK_PATTERN`, `SESSION_WELCOME`, `PLAYER_STATE`, `ACTION_RESULT`, `TERMINAL_LIVE`, `TERMINAL_UPDATE`, `TERMINAL_CLEAR`, `NAV_STATE`, or `HACK_STATE`. The removed `HACK_ADMIN` request does not return in any public contract or asset. Historical completed feature records may retain the strings only when clearly marked superseded and non-authoritative.
