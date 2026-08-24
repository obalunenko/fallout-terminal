# Data Model: HTTP/2 Presentation Intent Streaming

All entities in this document are transient runtime values. None changes session JSON, player-
configuration JSON, Keychain records, or public-access settings.

## Client instance

One physical browser tab owns one client instance for the page lifetime.

| Field | Type | Rules |
|---|---|---|
| `client_instance_id` | opaque string | Generated once with browser cryptographic UUID support when available; non-empty, public-field bounded, never persisted; different tabs must differ. |
| recognition handle | opaque string | Existing browser recognition identity; may be shared by tabs but does not grant authority. |
| subscription generation | local integer | Existing reconnect counter for `Subscribe`; not sent as uplink authority. |
| uplink generation | unsigned integer | Starts at 1 and increases for every probe, reconnect, rotation, or replacement in the tab. |

The client instance is destroyed on page unload. A reload creates a new identity even if the
recognition handle survives in existing browser storage.

## Physical subscription

One active `Subscribe` call owns one server physical connection.

| Field | Type | Rules |
|---|---|---|
| connection ID | `domain.ConnectionID` | Server-generated; remains the canonical coordinator presence identity. |
| logical session ID | `domain.LogicalSessionID` | Resolved by the coordinator from recognition; shared by same-player tabs. |
| client instance ID | optional opaque string | Copied from `SubscribeRequest.client_instance_id`; absent for compatible older clients. |
| canonical revision | unsigned integer | Starts from the initial snapshot; strictly monotonic for compound updates. |
| canonical queue | bounded FIFO | Existing queue and overflow-close semantics remain unchanged. |
| targeted result mailbox | capacity-one non-lossy value | Separate from canonical capacity; publishing waits for capacity or cancellation and cannot close or reorder canonical delivery. |
| current uplink generation | optional unsigned integer | Monotonic for this client instance while the subscription is current. |

### Subscription invariants

- One physical connection belongs to exactly one logical session.
- At most one active physical subscription is indexed by a given `client_instance_id`.
- A client instance may bind an uplink only to its current physical subscription.
- Canonical snapshots/updates never enter the targeted result mailbox.
- Targeted results never update `lastRevision` and never enter the canonical queue.
- If both queues remain ready, the send loop emits one canonical update first and then the pending
  targeted result before selecting another canonical update.

## Browser outbound intent mailbox

One active browser uplink owns one process-local latest-value mailbox.

| Field | Type | Rules |
|---|---|---|
| pending intent | optional `PresentationIntent` | A new eligible intent replaces the older unsent value. |
| handed-off intent | optional request ID | At most one message has been pulled by the request body but has not completed transport handoff. |
| state | open/closed | Closing clears pending state and wakes the request-body puller. |

The mailbox has these operations:

- `Offer(intent)`: atomically replace the pending unsent intent.
- `Pull(ctx)`: remove the newest pending intent only when the Fetch request body requests another
  message.
- `Close(cause)`: reject offers, clear pending state, and unblock the puller.
- `TakeLatestForFallback()`: return at most one still-eligible target after invalidating the stream
  generation.

The mailbox never buffers intermediate pointer or keyboard targets. At most one intent is handed to
the Fetch stream while one newer intent waits for transport demand.

## Presentation uplink

One browser client-stream request owns one bounded server uplink.

| Field | Type | Rules |
|---|---|---|
| client instance ID | opaque string | Must match a current subscription. |
| generation | unsigned integer | Must be greater than the previously accepted generation for that tab. |
| recognition handle | opaque string | Must resolve to the same logical session as the bound subscription. |
| physical connection ID | `domain.ConnectionID` | Captured only after successful open validation; supplied to canonical dispatch. |
| state | enum below | Process-local and cancellable. |
| intent mailbox | capacity-one latest value | A new semantic intent replaces an unprocessed older value. |
| receive budget | fixed token window | 120 frames/second, burst 32. |
| opened/last-received times | monotonic clock values | Enforce five-minute maximum and 30-second idle lifetimes. |
| cancel cause | error category | Replacement, invalidation, limits, peer close, server shutdown, or normal rotation. |

### Uplink state transitions

```text
created
  ├─ invalid/missing open ───────────────> rejected
  ├─ concurrency limit ─────────────────> rejected
  └─ valid open + current Subscribe ────> probing

probing
  ├─ targeted ready offered ────────────> active
  ├─ subscription/generation invalid ───> rejected
  └─ canceled/deadline ─────────────────> closed

active
  ├─ newer generation for same tab ─────> replaced
  ├─ authority/context invalidation ────> invalidated
  ├─ idle/max lifetime/rate limit ──────> limited
  ├─ client/server cancellation ────────> closed
  └─ normal request completion ─────────> closed
```

Every terminal state cancels the receiver and processor, clears the pending intent, unregisters the
generation, decrements concurrent-uplink accounting, and completes without persisting state.

## Presentation intent

One semantic controller target eligible for canonical validation.

| Field | Type | Validation |
|---|---|---|
| recognition handle | opaque string | Existing public-field rules; must still resolve to the bound session. |
| request ID | opaque string | Existing request identity rules and canonical idempotency cache. |
| broadcast ID | opaque string | Must equal the current broadcast for every processed intent. |
| terminal ID | opaque string | Must equal the active terminal for every processed intent. |
| context key | opaque string | Must equal both the generated presentation context and current projected context. |
| presentation | generated oneof | Exactly one none/menu/page/hacking semantic target with existing target/page validation. |

Two intents are semantically equal when context key, presentation variant, target/pattern, and page
index match. Client dispatch suppresses equality with the current authoritative, in-flight, pending,
or latest local target. Request identity and transport generation do not change semantic equality.

## Latest-value intent mailbox

The mailbox has these atomic operations:

- `Offer(intent)`: if empty, store; if occupied, replace the stored unprocessed intent.
- `Take(ctx)`: remove the current value for one ordered canonical dispatch, or wait for offer/cancel.
- `Close(cause)`: reject future offers, clear the stored value, and wake the processor.

There is one receiver and one processor per uplink. At most one canonical mutation may be executing
for that uplink and at most one newer intent may wait.

## Targeted presentation result

One ephemeral downlink message belongs to one current physical tab and generation.

| Field | Type | Rules |
|---|---|---|
| client instance ID | opaque string | Must equal the subscription identity; browser rejects mismatch. |
| uplink generation | unsigned integer | Browser accepts only its current probing/active generation. |
| payload | ready or `ActionResult` | Ready proves stream-open delivery; action carries existing request, acceptance, reason, and revision. |

The result does not mutate canonical state or advance subscription revision. A processed result is
never replaced by another result. `Publish(ctx, result)` waits until the targeted slot becomes
available or the uplink/subscription is cancelled. The ordered uplink processor does not dispatch
another intent until the previous result has entered the targeted mailbox. Cancellation may abandon
delivery, after which the browser reconciles from canonical state and uses unary fallback for only
the newest still-eligible target.

## Local transient presentation

One controller-only visual layer overlays the canonical presentation.

| Field | Type | Rules |
|---|---|---|
| context key | string | Must equal the current authoritative presentation context. |
| local sequence | unsigned integer | Strictly increases for every distinct eligible local target. |
| semantic target | presentation value | Same semantic variants as the generated contract, stored as browser render state only. |
| request ID | optional string | Links the dispatched transport intent; does not grant authority. |

### Reconciliation

- Render local transient selection by the next animation frame.
- Never copy transient state into canonical `controllerPresentation`.
- Never play preview audio from transient state.
- An authoritative update that matches the latest local target clears the transient after canonical
  application without replaying its local visual effect.
- An older/different authoritative target updates the canonical mirror but does not displace a newer
  valid local target or trigger superseded controller animation/audio.
- Role, broadcast, terminal, context, subscription, or tab invalidation clears the transient
  immediately and renders the applicable canonical state.

## Relationships

```text
logical player session
  └── physical Subscribe connection (one per tab)
        ├── canonical revision queue
        ├── targeted result mailbox
        └── client_instance_id
              └── current PresentationUplink generation
                    ├── capacity-one intent mailbox
                    └── ordered canonical dispatch by physical connection ID
```

Multiple tabs may share one logical recognition handle, but each has a distinct client instance,
physical subscription, current generation, targeted result route, and local transient layer.
