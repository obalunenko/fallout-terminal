# Phase 1 Data Model: Interactive Approval Notifications

All models in this document are process-local root-adapter state. Existing domain and protobuf models remain
unchanged and authoritative.

## ApprovalNotificationRequest

A detached presentation of the one current Overseer approval request.

| Field | Type | Validation |
|---|---|---|
| `kind` | `command-execution` or `terminal-navigation` | Required; derived from exactly one pending field in authoritative coordination state. |
| `requestID` | string | Required, non-blank, server-owned, and copied from the pending projection. |
| `notificationID` | string | Required identifier derived from a cryptographically random launch ID, kind, and request ID; stable for the current process request and distinct across launches. |
| `commandName` | string | Existing prompt value; fall back to command ID only when name is blank. |
| `body` | string | Command confirmation text, or the existing navigation source/command/target summary; bounded for native presentation. |
| `categoryID` | string | The one registered approval category. |

When both pending fields are nil, there is no notification request. If both are unexpectedly non-nil, the
adapter fails closed, logs a sanitized invariant warning, and creates neither notification; the in-app state
remains authoritative.

## CurrentApprovalNotification

The coordinator's in-memory correlation record.

| Field | Type | Rules |
|---|---|---|
| `request` | `ApprovalNotificationRequest` | Immutable detached values for the current request. |
| `delivery` | `waiting`, `attempting`, `delivered`, or `failed` | At most one delivery attempt becomes a newly delivered notification for the request. |
| `decisionPending` | bool | Set before calling App so concurrent and repeated callbacks cannot start a second decision. |
| `invalidated` | bool | Set before native cleanup; once true, no callback can act. |

This record and its launch ID are never persisted. A response from a toast delivered by an earlier process has
no matching current notification ID, even when coordination restores the same kind and request ID, and is stale.

## NotificationAvailability

The optional native service lifecycle for the current application process.

| State | Meaning | Allowed next states |
|---|---|---|
| `starting` | Native service has not completed startup. | `authorizing`, `unavailable`, `stopped` |
| `authorizing` | Service and action category are ready; authorization check or the single consent request is in progress. | `ready`, `unavailable`, `stopped` |
| `ready` | Interactive delivery may be attempted. | `unavailable`, `stopped` |
| `unavailable` | Startup, category, authorization, or fatal delivery capability failed or was denied. | `stopped` |
| `stopped` | Shutdown invalidated callbacks and future work. | none |

The authorization operation runs outside App/control locks. Shutdown marks the adapter stopped immediately and
ignores any later result from the pinned service's internally bounded authorization call, so host shutdown never
waits for user input.

## NotificationResponseIntent

A validated local interpretation of a native callback.

| Field | Type | Validation |
|---|---|---|
| `notificationID` | string | Must exactly match the non-invalidated current record. |
| `categoryID` | string | Must equal the registered approval category. |
| `action` | `approve` or `reject` | Default clicks, text replies, unknown values, and empty actions are inert. |
| `requestKind` | request kind from current record | Never accepted from callback metadata. |
| `requestID` | request ID from current record | Never accepted from callback metadata or user data. |

## Relationships

```text
MasterCoordinationState
        │ detached observation
        ▼
ApprovalNotificationRequest ── owns ──► CurrentApprovalNotification
        │                                      │
        │ native presentation                  │ validates callback
        ▼                                      ▼
system notification                    NotificationResponseIntent
                                               │ exact recorded ID
                                               ▼
                        App.ResolveCommandExecution
                                     or
                        App.ResolveTerminalNavigation
                                               │
                                               ▼
                              authoritative coordination state
```

## State Transitions

| Current condition | Input | Result |
|---|---|---|
| no current request | new valid pending request | Store current record; deliver once when availability is `ready`. |
| same current request | repeated or newer state snapshot | Keep the record; do not redeliver. |
| current request A | replacement request B | Invalidate A, request cleanup for A, store B, then deliver B once. |
| current request | state clears or invalidates it | Invalidate first, clear current authority, then request native cleanup. |
| current request, no decision pending | matching approve/reject callback | Mark decision pending and call the corresponding App method once. |
| current request, decision pending | another callback or in-app race | Do not start another notification decision; App remains authoritative. |
| callback does not match current record | any action | Ignore as stale or malformed; no App call. |
| App decision succeeds | resulting authoritative clear state | Existing publication clears the record and synchronizes the in-app dialog. |
| App decision fails and request remains pending | safe failure result | Clear only `decisionPending`; retain current in-app request and do not imply success. |
| native delivery or cleanup fails | safe error | Record/log adapter failure only; do not mutate coordination state. |

## Concurrency and Lifetime Rules

- Observation performs only detached reduction and short synchronized state changes; native calls and App
  decisions run outside App, coordinator, and adapter locks.
- Invalidation occurs before cleanup, and `decisionPending` occurs before App invocation.
- Authorization completion, delivery completion, response callbacks, state replacement, and shutdown all verify
  the same process generation/current record before applying a result.
- No callback retains mutable domain state or a Wails window/application object.
- Any queued native operation after `stopped` is discarded without touching App.
