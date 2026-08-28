# Data Model: Protobuf-First ConnectRPC Migration

Generated protobuf messages in this document are detached boundary values. Canonical mutable state remains in the transport-independent domain and coordination services; established JSON codecs remain authoritative for files.

## Contract Inventory Item

One row in `contracts/inventory.md` with:

- `name`: current application-owned structured boundary or serializable configuration field group;
- `owner`: package or composition boundary that defines the current meaning;
- `producer` and `consumers`: concrete paths that create and read the value;
- `exposure`: public player, private desktop, private persistence, private configuration, third-party, or non-serializable dependency;
- `classification`: the versioned protobuf message that governs it, the third-party schema that governs it, or the explicit exclusion rationale;
- `verification`: descriptor, adapter, fixture, bundle, or source check that prevents drift.

Validation: final inventory contains no unclassified application-owned public/private DTO and no unclassified serializable configuration field.

## Canonical ProcessRuntime

The existing `internal/control.Service` remains the single mutable owner.

| Field | Type | Rule |
|---|---|---|
| `revision` | `uint64` | Starts at zero; advances exactly once for each accepted state-changing transition. |
| `sessionsByID` | map of `LogicalSession` | Process-local only; cleared on shutdown/restart. |
| `sessionIDByRecognitionHandle` | opaque handle → session ID | Process-local recognition only; never authentication or persistence. |
| `rosterByID`, `rosterOrder` | authored character registry | Loaded from player-config-v1; mutation publishes only after atomic save. |
| `activePlayerConfig` | private path/version/name handle | Private process state; no public projection or credential behavior. |
| `broadcast` | optional `LiveBroadcast` | Owns assignments, controller, terminal runtimes, and replay scope. |
| `pendingSwitch` | optional `TerminalSwitchDecision` | Trusted desktop-only unfinished-puzzle decision. |
| `streamSinksBySession` | logical session → physical stream sinks | Non-serializable injected delivery dependencies, stored outside protobuf/domain persistence. |

Invariant: an accepted mutation is applied to a deep working copy, all detached logical publications receive the committed revision, and those publications are offered before the unary result completes.

## RecognitionHandle

An opaque process-local string issued by the server.

| Property | Rule |
|---|---|
| Minted form | 32 lowercase hexadecimal characters from the existing cryptographic ID source. |
| Accepted form | 1–128 UTF-8 bytes from `[A-Za-z0-9_-]`; blank, oversized, or malformed present values are structural errors. |
| Browser storage | The only persistent player value; same-origin `localStorage`. |
| Authority | None; every assignment, presence, controller, broadcast, terminal, gameplay, and replay rule is revalidated. |
| Lifetime | Current process only; an old well-formed value may be replaced only by `Subscribe`. |

State transitions:

- Absent on `Subscribe` → create a fresh logical session and return a new handle.
- Present and recognized on `Subscribe` → reattach to the same logical session.
- Present, well-formed, unknown/expired on `Subscribe` → create a fresh session and replacement handle.
- Present but blank/oversized/malformed on `Subscribe` → Connect `invalid_argument`, no session.
- Missing/blank/oversized/malformed on unary mutation → Connect `invalid_argument`, no canonical invocation.
- Well-formed unknown/expired on unary mutation → typed `invalid-session`, no new session or replay entry.

## LogicalSession

| Field | Type | Rule |
|---|---|---|
| `id` | opaque internal identifier | Process-local; only the session's own personalized state may carry it. |
| `fallbackName` | string | Existing unique private/display behavior, maximum 80 Unicode characters. |
| `physicalStreamIDs` | set | Aggregate presence is connected iff non-empty. |
| `requestResultsByBroadcast` | broadcast ID → bounded replay map | Maximum `256` retained records for the current session and broadcast. |

Relationships:

- One recognition handle maps to one logical session.
- One logical session has zero or more physical streams.
- One logical session has at most one current-broadcast character assignment.
- Several physical streams attached to one logical session receive equivalent personalized values for a revision while responsive.

## PhysicalStream

| Field | Type | Rule |
|---|---|---|
| `id` | opaque server-local ID | Counts toward raw `client-count`; never public player state. |
| `sessionID` | logical-session reference | Installed during atomic attach and removed idempotently. |
| `queue` | capacity-`32` channel of detached subscription values | Non-blocking offer; overflow cancels only this stream. |
| `lastSentRevision` | `uint64` | Snapshot revision is baseline; later delivered revisions are strictly increasing. |
| `cancel` | function/context | Non-serializable dependency; used for disconnect, overflow, request cancel, and shutdown. |

Lifecycle:

1. Resolve recognition, register the sink, and capture snapshot revision R under the coordinator order.
2. Send exactly one `PersonalizedSnapshot` as the first application value.
3. Drain only updates with revision greater than R and greater than the previous delivered revision.
4. On cancellation, disconnect, or overflow, close the stream and detach it within bounded time.
5. Recovery always creates a new subscription beginning with a complete snapshot; incremental values are not replayed.

## LiveBroadcast and Replay Scope

| Field | Type | Rule |
|---|---|---|
| `id` | opaque broadcast ID, max 128 bytes | Every mutation presents the current ID. |
| `assignmentsBySession` | session → character | Exclusive in both directions. |
| `sessionByCharacter` | character → session | Reverse index for conflict checks. |
| `controllerSessionID` | optional session | Must be assigned; shared terminal actions also require active presence. |
| `activeTerminalID` | optional terminal | Absence is a valid no-live-terminal state. |
| `terminalRuntimes` | terminal ID → private checkpoint | Owns navigation and complete private hacking state. |
| replay records | per-session map | Cleared with broadcast end; retained exact results only. |

Broadcast end clears claims, controller, terminal runtimes, replay records, and pending switch while retaining recognized sessions, fallback names, and the loaded authored roster.

## RequestReplayRecord

| Field | Type | Rule |
|---|---|---|
| `requestID` | 1–128 byte opaque token | Unique only within retained session/broadcast records. |
| `procedure` | fully qualified procedure name | Prevents cross-procedure reuse. |
| `fingerprint` | digest of deterministic validated protobuf payload | Excludes recognition and request ID; includes every procedure-specific semantic field. |
| `result` | detached `ActionResult` | Original acceptance/reason/revision returned for exact replay. |

Transitions:

- No retained record → validate against current state and optionally store the result.
- Same ID, procedure, and fingerprint → return original result/revision; no effect.
- Same retained ID but different procedure/fingerprint → `duplicate`; no effect.
- Record evicted/cleared/lost → evaluate as a new request; no exactly-once promise remains.

## PersonalizedSnapshot

The first `SubscriptionMessage` value.

| Field | Presence | Rule |
|---|---|---|
| `recognitionHandle` | required nonblank | Accepted or replacement opaque handle. |
| `revision` | required | Atomic snapshot revision. |
| `playerState` | required | Complete personalized identity, assignment, role, phase, roster, broadcast, controller-relevant state. |
| `terminalPresentation` | required | Exactly one `liveTerminal` or `noLiveTerminal` variant. |

Snapshot creation reads current terminal/navigation/hacking projections only. It never calls board generation, consumes randomness, replays actions, or produces sound cues.

## CompoundUpdate

One logical publication for one affected session and committed revision.

| Field | Presence meaning |
|---|---|
| `revision` | Required and greater than the subscriber's last delivered revision. |
| `playerState` | Present means complete current personalized player state; absent means unchanged. |
| `terminalPresentation` | Present means complete live-terminal or explicit no-live-terminal replacement; absent means unchanged. |
| `navigation` | Present means complete current navigation projection; absent means unchanged. |
| `hacking` | Present means complete current public hacking projection; absent means unchanged. |

Several components may be present together. No component is a partial patch, and no accepted action creates several subscription messages at the same revision for one subscriber.

## PlayerMutation and ActionResult

Every mutation has a distinct generated request type. Shared fields are recognition handle, request ID, and broadcast ID; navigation/guess/pattern requests also carry terminal identity, while the `patternId` generated ECMAScript property carries generation-bound pattern identity.

`ActionResult` contains:

- `requestId`: exact correlation identifier;
- `accepted`: true only for the one committed state-changing winner;
- `reason`: one of `accepted`, `invalid-session`, `stale-broadcast`, `unassigned`, `not-controller`, `controller-disconnected`, `stale-terminal`, `invalid-action`, `conflict`, or `duplicate`;
- `revision`: committed revision for acceptance, or authoritative current revision for rejection/replay.

Character selection requires recognized active presence, current broadcast, an unassigned session, and an available character; it does not require controller, terminal, or generation authority. Navigation, guess, and pattern activation additionally require assignment, eligible connected controller status, current terminal, and current generation where applicable.

## BrowserPendingAction

Client-local, nonpersistent reconciliation state.

| Field | Type | Rule |
|---|---|---|
| `requestId` | string | Correlates one generated unary call. |
| `procedure` | enum/local tag | Presentation only; not a generic wire command. |
| `result` | optional `ActionResult` | Rejection clears immediately; acceptance records result revision. |
| `streamRevision` | optional `uint64` | Latest applicable authoritative projection. |

Accepted pending state clears only when both the correlated unary result and an applicable stream state at or beyond its revision are present. It is discarded on reconnect because the replacement snapshot is complete canonical state, not action replay.

## TerminalPresentation

A protobuf `oneof`:

- `liveTerminal`: complete terminal identity/name/content tree/intro/hack level plus current navigation and public hacking projections;
- `noLiveTerminal`: explicit empty marker.

No parallel optional variants or discriminator string is permitted. Changing live terminal or clearing it publishes the complete selected variant.

## PublicHackingProjection

Contains only player-safe board columns, word placements, attempts, log, solved/failed state, and current public patterns. Every pattern exposes the generated ECMAScript identifier `patternId` plus row/start/end/used state. Secret word, candidate lookup, generation internals, random outcomes, and future pattern outcomes remain in private canonical terminal runtime only.

An accepted unused current pattern consumes one outcome draw and at most one dud-selection draw. Every stale, used, duplicate, rejected, or losing request consumes zero random calls.

## SoundManifest

| Field | Rule |
|---|---|
| `category` | One of `ambient`, `hack-good`, `hack-bad`, `menu-focus`, `single`, `multiple`, `enter`, `charscroll`; `UNSPECIFIED` is invalid. |
| `paths` | Deterministically sorted safe relative names/paths using `.mp3`, `.wav`, `.ogg`, `.m4a`, or `.webm`. |

No absolute origin, native path, embedded filesystem value, caller-supplied path fragment, or directory traversal input exists. A valid missing/unreadable/empty category directory returns an empty successful manifest.

## Private Desktop Contract Values

Generated `fallout.terminal.private.v1` values define the fields and variants for current Wails requests, results, runtime status, server information, coordination state, terminal switches, commands, and events. They are transient adapter values only. Exact Wails compatibility DTOs and native JavaScript values remain the bridge representation; protobuf binary, Base64, and ProtoJSON never cross Wails.

## Persistence Contract Values

Generated `fallout.terminal.persistence.v1` values define known semantics for:

- session-v1: `version`, `name`, `playerConfig`, `terminals`;
- terminal: `id`, `name`, `hackLevel`, `introText`, `root`;
- recursive content node: `id`, `type`, `name`, `children`, `text`, `description`;
- player-config-v1: `version`, `name`, `roster`;
- roster entry: `id`, `name`.

They do not replace file representation. Session JSON preserves compatible unknown fields at the session, terminal, and recursive node levels. Player-config JSON rejects unknown fields and trailing data. Existing validation, normalized relative references, explicit selected paths, ordered revisions, and atomic replacement remain unchanged.

## Serializable Configuration

Generated `fallout.terminal.config.v1` values govern serializable application, listener, queue, request-limit, reconnect, replay, path, tunnel, startup, shutdown, and process-grace settings. Defaults include public player port `3690`, delivery queue `32`, replay cache `256`, request message `4 KiB`, encoded body `8 KiB`, reconnect delay three seconds, no configured fixed ngrok domain, current tunnel startup timeout, and current shutdown/grace behavior.

Embedded filesystems, callbacks, event sinks, clocks, random/ID sources, service interfaces, contexts, process handles, listeners, and runners remain non-serializable injected values and never become protobuf fields.
