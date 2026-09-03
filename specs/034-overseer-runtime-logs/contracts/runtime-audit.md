# Contract: Runtime Audit Records

## Record envelope

Every retained audit line uses the existing logger's text record format and includes:

- `time`: record timestamp;
- `level`: severity;
- `msg`: stable human-readable summary;
- `event`: one category from the table below;
- `run_id`: application-run correlation;
- event-specific allowlisted fields only.

The retained file and standard-error output receive the same event coverage. Ordinary lifecycle and accepted outcomes use informational severity; rejected actions, failed decisions, interruptions caused by unavailable state, and retained-output degradation use warning or error severity as appropriate.

## Event categories

| Event | Required fields | Optional safe fields | Meaning |
|---|---|---|---|
| `player.connected` | `revision`, `session_id`, `role` | — | First physical connection for a disconnected or new logical session |
| `player.role_changed` | `revision`, `session_id`, `previous_role`, `role` | — | Effective role changed while the logical session remained known |
| `player.disconnected` | `revision`, `session_id`, `role` | `reason` | Final physical connection ended; role is the last known role |
| `command.request_received` | `revision`, `request_id`, `session_id`, `role`, `terminal_id`, `command_id` | `mode` | Canonical command entered Overseer review |
| `command.request_outcome` | `revision`, `request_id`, `outcome` | `session_id`, `role`, `terminal_id`, `command_id` | Player action was accepted, replayed, rejected, duplicated, superseded, or conflicted before or while creating a pending request |
| `command.decision` | `revision`, `request_id`, `decision`, `outcome` | `session_id`, `role`, `terminal_id`, `command_id`, `mode` | Overseer approval or decline and the synchronous execution result |
| `hack.started` | `revision`, `puzzle_id`, `terminal_id`, `hack_level`, `attempts_max`, `attempts_left` | `session_id`, `role`, `reason` | A new puzzle generation became active |
| `hack.guess` | `revision`, `puzzle_id`, `terminal_id`, `outcome`, `attempts_left` | `session_id`, `role` | A guess was accepted or safely rejected; no guess or match content is present |
| `hack.pattern` | `revision`, `puzzle_id`, `terminal_id`, `outcome` | `session_id`, `role`, `attempts_left` | A pattern was rejected, removed a dud, or replenished attempts |
| `hack.succeeded` | `revision`, `puzzle_id`, `terminal_id`, `outcome` | `session_id`, `role`, `attempts_left` | Puzzle reached solved state through a player or trusted Overseer action |
| `hack.failed` | `revision`, `puzzle_id`, `terminal_id`, `attempts_left` | `session_id`, `role` | Puzzle exhausted its attempts |
| `hack.reset` | `revision`, `previous_puzzle_id`, `puzzle_id`, `terminal_id`, `outcome` | — | Failed puzzle was replaced by a new generation |
| `hack.interrupted` | `revision`, `puzzle_id`, `terminal_id`, `reason` | `session_id`, `role` | Active puzzle stopped being the current interaction because of a safe lifecycle reason |

## Closed values

- Roles: `unassigned`, `active`, `observer`.
- Decisions: `approve`, `decline`.
- Command outcomes: `accepted`, `replayed`, `declined`, `succeeded`, `failed`, `stale`, `duplicate`, `invalid`, `conflict`, `unassigned`, `not-controller`, `controller-disconnected`, `stale-broadcast`, `stale-terminal`.
- Guess outcomes: `rejected`, `mismatch`, `succeeded`, `failed`.
- Pattern outcomes: `rejected`, `dud-removed`, `attempts-replenished`.
- Interruption reasons: `terminal-suspended`, `terminal-discarded`, `terminal-cleared`, `broadcast-ended`, `controller-unavailable` when the corresponding transition actually ends or suspends interaction.
- Success sources: `player` or `overseer` in the `outcome` field.

Unknown values are normalized to `invalid` or omitted according to the category allowlist; raw strings are never substituted.

## Ordering and correlation

- The coordinator assigns `revision` to accepted transitions before audit effects leave the transaction.
- Effects from one transition preserve their declared order.
- `request_id` from `command.request_received` is reused by its terminal `command.decision` record.
- `puzzle_id` is the existing private puzzle-generation identity and is never exposed to the player protocol.
- `run_id` is attached by the root logger to all records, including pre-coordinator lifecycle messages.

## Forbidden fields and values

Records must not include browser tokens, recognition handles, physical connection IDs, character names, display names, terminal names, command names, confirmation text, command payloads or output, navigation targets, puzzle targets, board positions, board text, candidate or solution words, hacking activity text, credentials, external provider values, private paths other than the app-owned log paths returned by the private access action, or raw dependency errors that may contain those values.
