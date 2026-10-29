# Contract: Runtime Audit Records

## Record envelope

Every retained audit line uses the existing logger's text record format and includes:

- `time`: record timestamp;
- `level`: severity;
- `msg`: stable human-readable summary (`runtime audit event`);
- `event`: one category from the table below;
- `outcome`: one closed category value for that event;
- `run_id`: application-run correlation;
- `revision`: accepted coordination revision;
- `broadcast_id`: opaque active-broadcast identity when a broadcast is resolved;
- event-specific allowlisted fields only.

The retained file and standard-error output receive the same event coverage. Outcomes `failed`, `declined`, `rejected`, `stale`, `superseded`, `duplicate`, `invalid`, `conflict`, `interrupted`, and the command rejections `unassigned`, `not-controller`, `controller-disconnected`, `stale-broadcast`, and `stale-terminal` use warning severity; every other audit outcome uses informational severity. Retained-output degradation is reported once per run as a warning on standard error (see [runtime logs](../../../docs/runtime-logs.md)).

## Event categories

| Event | Outcome | Required fields | Optional safe fields | Meaning |
|---|---|---|---|---|
| `player.connected` | `connected` | `session_id`, `role` | — | First physical connection for a disconnected or new logical session |
| `player.role_changed` | `selected` | `session_id`, `previous_role`, `role` | — | Effective role changed while the logical session remained known |
| `player.disconnected` | `disconnected` | `session_id`, `role` | — | Final physical connection ended; role is the last known role |
| `command.request_received` | `accepted` | `request_id`, `session_id`, `role`, `terminal_id`, `command_id` | `mode` | Canonical command entered Overseer review |
| `command.request_outcome` | command outcome | `request_id`, `outcome` | `session_id`, `role`, `terminal_id`, `command_id`, `mode` | Player action was replayed, rejected, duplicated, or conflicted before a pending request existed, or a pending request was superseded |
| `command.decision` | command outcome | `request_id`, `decision` | `session_id`, `role`, `terminal_id`, `command_id`, `mode` | Overseer approval or decline and the synchronous execution result |
| `hack.started` | `started` | `puzzle_id`, `terminal_id`, `hack_level`, `attempts_max`, `attempts_left` | `session_id`, `role`, `reason` | A puzzle generation became the current interaction |
| `hack.guess` | guess outcome | `terminal_id`, `attempts_left` | `puzzle_id`, `session_id`, `role` | A guess was evaluated or safely rejected; no guess or match content is present |
| `hack.pattern` | pattern outcome | `terminal_id`, `attempts_left` | `puzzle_id`, `session_id`, `role` | A pattern was rejected, removed a dud, or replenished attempts |
| `hack.succeeded` | `succeeded` | `puzzle_id`, `terminal_id`, `attempts_left` | `session_id`, `role` | Puzzle reached solved state through a player or trusted Overseer action |
| `hack.failed` | `failed` | `puzzle_id`, `terminal_id`, `attempts_left` | `session_id`, `role` | Puzzle exhausted its attempts |
| `hack.reset` | `reset` | `previous_puzzle_id`, `puzzle_id`, `terminal_id`, `attempts_left` | `session_id`, `role` | Failed puzzle was replaced by a new generation; a `hack.started` record for the new `puzzle_id` follows |
| `hack.interrupted` | `interrupted` | `puzzle_id`, `terminal_id`, `reason`, `attempts_left` | `session_id`, `role` | An unfinished puzzle stopped being the current interaction because of a safe lifecycle reason |

A rejected guess or pattern omits `puzzle_id` when validation failed before a puzzle was resolved.

## Closed values

- Roles: `unassigned`, `active`, `observer`.
- Decisions: `approve`, `decline`.
- Command outcomes: `accepted`, `replayed`, `declined`, `succeeded`, `failed`, `stale`, `superseded`, `duplicate`, `invalid`, `conflict`, `unassigned`, `not-controller`, `controller-disconnected`, `stale-broadcast`, `stale-terminal`. The outcome `unassigned` means the requesting session had no character; it is distinct from the `unassigned` role value.
- Forward terminal-transition commands reuse the same outcomes: a changed or missing target after review is `stale`, and an approved transition whose activation fails is `failed`.
- Command modes: `ordinary`, `state-change`, `completed-state-change`, `terminal-transition`.
- Guess outcomes: `rejected`, `mismatch`, `succeeded`, `failed`. Any value other than `rejected` means the guess was accepted and evaluated; `mismatch` is an accepted guess that neither solved nor failed the puzzle.
- Pattern outcomes: `rejected`, `dud-removed`, `attempts-replenished`.
- Start reasons: `resumed` when a suspended terminal's unfinished puzzle becomes current again with the same `puzzle_id`; absent for a new generation, including one created by a reset. A resumed solved or failed puzzle emits no start record.
- Interruption reasons: `terminal-suspended`, `terminal-discarded`, `terminal-cleared`, `broadcast-ended`, `controller-unavailable` when the corresponding transition actually ends or suspends interaction. A `terminal-suspended` interruption may be followed by `hack.started` with reason `resumed` for the same `puzzle_id`.

Every value is a coordinator-owned constant or an opaque identifier. The root formatter emits only the fields listed in the envelope and table and never substitutes free text, display content, or raw errors.

## Ordering and correlation

- The coordinator assigns `revision` to accepted transitions before audit effects leave the transaction.
- Effects from one transition preserve their declared order.
- `request_id` from `command.request_received` is reused by its terminal `command.decision` or superseding `command.request_outcome` record, and keeps the requesting `session_id` even when roles change while the request is pending.
- `puzzle_id` is the existing private puzzle-generation identity and is never exposed to the player protocol.
- `run_id` is attached by the root logger to all records, including pre-coordinator lifecycle messages.

## Forbidden fields and values

Records must not include browser tokens, recognition handles, physical connection IDs, character names, display names, terminal names, command names, confirmation text, command payloads or output, navigation targets, puzzle targets, board positions, board text, candidate or solution words, hacking activity text, credentials, external provider values, private paths other than the app-owned log paths returned by the private access action, or raw dependency errors that may contain those values.
