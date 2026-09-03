# Data Model: Overseer Runtime Logs

## Application Run

Represents one process launch and groups every retained record written before that process stops.

| Field | Meaning | Validation |
|---|---|---|
| Run ID | Opaque correlation identity created before the application logger starts | Non-empty, random, safe for filenames and log fields; never reused intentionally |
| Started at | UTC instant used for ordering and the retained filename | Set once at creation |
| State | `active`, `degraded`, or `closed` | Begins active when the retained file opens; becomes degraded on retained-output failure; closes once |
| Directory path | App-owned per-user log directory | Absolute, below application support, outside packaged resources |
| Active segment path | File currently receiving retained records | Empty only when retained output could not be established |

## Retained Log Set

Owns the bounded collection of run-specific log segments and the writer used by the existing application logger.

| Field | Meaning | Validation |
|---|---|---|
| Directory | Parent of all retained log segments | Created with user-only permissions where supported; symbolic or non-directory targets are rejected |
| Segment size limit | Rotation threshold for one segment | 5 MiB; a single complete record is never split |
| Segment count limit | Maximum retained segments | Eight; at least the newest current-run and previous-run segment are protected |
| Active segment | Current run's writable segment | Exactly one while retained output is healthy |
| Historical segments | Closed segments ordered by run, segment, and creation time | Only files matching the owned naming grammar are eligible for cleanup |
| Fallback state | Whether retained writes failed and standard error remains available | Failure is reported once per degraded period without recursively logging through the failed writer |

### Segment lifecycle

```text
create current → append complete records → reach threshold → close/rotate
       │                                      │
       └──────── retained write failure ──────┴→ degraded; stderr remains

closed segment → protected current/previous selection → keep or prune oldest
```

The writer serializes writes and path queries. Cleanup never follows links, removes unknown files, deletes the active segment, or removes the newest segment belonging to the immediately previous run.

## Runtime Audit Event

A detached safe fact produced by an authoritative coordinator transition and consumed by root logging.

| Field | Meaning | Presence rule |
|---|---|---|
| Category | Closed event category from the runtime-log contract | Always present |
| Revision | Accepted coordination revision ordering the event | Present for committed coordinator facts |
| Session ID | Logical player session involved | Present for player events and player-originated command or hack events when resolved |
| Role | Effective role at the event | Present when a session is resolved |
| Previous role | Role before an effective role transition | Role-change events only |
| Request ID | Pending approval or player request correlation | Command events only |
| Terminal ID | Stable active terminal identity | Command and hacking events when resolved |
| Command ID | Stable authored command identity | Only after canonical lookup validates the command |
| Puzzle ID | Private generation identity used only as a safe correlation token | Hacking events only |
| Previous puzzle ID | Puzzle replaced by a reset | Reset events only |
| Decision | `approve` or `decline` | Decision events only |
| Outcome or reason | Closed safe category defined by the event contract | Interaction, decision, rejection, and interruption events |
| Hack level | Configured puzzle difficulty | Start events only |
| Attempts maximum / remaining | Non-secret bounded counts | Start and relevant hacking interaction events |

### Validation and safety rules

- Event construction uses an allowlist per category; fields not allowed for that category are discarded before the event reaches the logger.
- Event values are enums, opaque identifiers, revisions, or bounded counts. No arbitrary display text or raw error is accepted.
- Browser tokens, recognition handles, physical connection IDs, character IDs or names, command names, confirmation text, payload fingerprints, targets, board coordinates, words, activity-log text, and dependency errors are forbidden.
- A rejected request may omit session, terminal, command, or puzzle identity when validation failed before that identity was safely resolved.
- Event order follows coordinator revision and effect order; several events may share a revision but retain their emission order.

## Logical Player Presence Trace

Represents the diagnostic view of one logical session rather than an individual transport connection.

```text
unknown/disconnected ── first physical connection ─→ connected(unassigned|observer|active)
connected(role A) ── authority or assignment change ─→ connected(role B)
connected(role) ── final physical connection ends ─→ disconnected(last role)
disconnected(role) ── recognized reconnect ─→ connected(current role)
```

Additional physical streams do not emit another connection event, and a non-final stream closure does not emit a disconnection event.

## Command Request Trace

Represents one safe request lifecycle correlated by the coordinator-issued pending request ID.

```text
player action
  ├─ invalid/duplicate/conflicting ─→ request_outcome(reason)
  └─ canonical command selected ─→ request_received(pending request ID)
                                      ├─ decline ─→ decision(decline, declined)
                                      └─ approve ─→ decision(approve, succeeded|failed|stale)
```

The player action request ID may appear on an early outcome. Once a request becomes pending, the coordinator-issued request ID is the authoritative correlation for Overseer decisions.

## Hacking Puzzle Trace

Represents the safe lifecycle of one puzzle generation.

```text
absent ─→ started(active)
active ── guess ─→ active | succeeded | failed
active ── pattern ─→ active(dud removed | attempts replenished)
active ── terminal/broadcast transition ─→ interrupted or suspended
failed ── reset ─→ reset(previous ID, new ID) ─→ started(new active puzzle)
active ── Overseer force success ─→ succeeded
```

A guess event never stores its target, word, match count, or player-facing activity text. A pattern event never stores its coordinates or removed candidate identity.

## Log Access Result

Private desktop result returned to the Overseer UI.

| Field | Meaning | Validation |
|---|---|---|
| OK | Whether the native open operation was started | Always present |
| Error | Safe actionable failure | Present only on failure; contains no raw native dependency detail |
| Directory path | Exact intended retained-log directory | Always present so manual navigation remains possible |
| Active log path | Current segment path | Present when retained output is active |

This result is private to the desktop service and is never published through player RPC, server streams, browser storage, named public events, or session persistence.
