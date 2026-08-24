# Contract: `PresentationUplink` and Browser Reconciliation

## Scope

`PresentationUplink` is an optional ConnectRPC client stream for high-frequency presentation intents
only. `Subscribe` remains the authoritative server-streaming downlink and `SetPresentation` remains
the portable unary fallback. Navigation, guessing, pattern activation, and character selection do
not use this stream.

## Browser transport boundary

The browser module must:

1. consume the generated `PlayerService` descriptor and generated message schemas;
2. delegate existing unary calls and `Subscribe` to `createConnectTransport`;
3. handle only the generated `PresentationUplink` client-stream descriptor specially;
4. serialize messages through official Connect client-method serializers;
5. frame them through official `encodeEnvelope` support;
6. build the procedure URL and streaming headers from official Connect protocol helpers;
7. send a Fetch `ReadableStream` request body with `duplex: "half"` and same-origin credentials;
8. use no handwritten network DTO, RPC path table, JSON wire envelope, or RPC router.

The exact generated procedure path is derived from the descriptor, never duplicated as an active
browser string constant.

## Eligibility and probe

Streaming is attempted only when all local checks pass:

- the page is served over HTTPS;
- Fetch, `ReadableStream`, `AbortController`, and request-body `duplex` construction are supported;
- the tab has a current generated `Subscribe` carrying `client_instance_id`;
- the player is recognized and the tab has not been invalidated.

These checks do not enable streaming. The browser then:

1. increments its uplink generation;
2. opens `PresentationUplink` and sends exactly one open frame;
3. keeps the request body open;
4. waits for a matching targeted ready result on `Subscribe`;
5. marks the generation active only if ready arrives before the probe deadline and the subscription,
   recognition, role, broadcast, terminal, and context are still current.

A timeout, stream error, subscription replacement, mismatched ready result, or intermediary body
buffering aborts the generation and selects unary fallback.

## Intent dispatch

- Pointer/keyboard input first produces one normalized semantic presentation target.
- A distinct eligible target receives a local sequence and renders visual feedback no later than the
  next animation frame.
- One shared semantic dispatcher suppresses equality with authoritative, in-flight, queued, or
  newest local state.
- When the uplink is active, the dispatcher offers generated `PresentationIntent` messages to a
  capacity-one outbound mailbox. The Fetch request-body puller takes only the newest pending value
  when transport demand permits. Pointer and keyboard events never enqueue directly into a
  `ReadableStream`, `TransformStream`, or unbounded async iterator. Otherwise the dispatcher uses
  BUG-010's one-in-flight/one-latest `SetPresentation` dispatcher.
- Never send the same semantic target concurrently through both transports.
- On stream failure, retain only the newest still-eligible target and hand it to unary dispatch after
  invalidating the stream generation.

## Server processing

1. Admit no more than the process-wide concurrent limit.
2. Require and validate the open frame before starting intent processing.
3. Bind the generation to the physical subscription identified by `client_instance_id` and the same
   recognition session; cancel an older generation from the same tab.
4. Offer a targeted ready result without using canonical queue capacity.
5. Receive generated messages under per-message size/decompression/schema and rate limits.
6. Replace an unprocessed mailbox value with the newest intent.
7. Before each dispatch, confirm the bound subscription and generation are still current and run the
   ordinary generated adapter validation.
8. Dispatch through the connection-aware canonical coordinator method. The existing transaction
   remains the only authority/order/revision owner.
9. Publish the returned `ActionResult` to the bound subscription's targeted mailbox before
   dispatching another intent. Wait only for targeted mailbox capacity or lifecycle cancellation;
   never hold coordinator or canonical-queue locks while waiting. Canonical effects continue through
   ordinary compound updates.

## Downlink ordering

- The initial complete snapshot is always the first `Subscribe` message.
- Canonical updates remain strictly revision-ordered and use only the existing canonical queue.
- Targeted messages carry no canonical revision ordering of their own and do not change
  `lastRevision`.
- When both queues are ready, the subscription send loop drains one available canonical message
  first and then services the pending targeted message before selecting another canonical message;
  continuous canonical traffic cannot starve the targeted mailbox.
- Targeted results are not silently replaced or dropped while the matching subscription and
  generation remain active.
- If the targeted mailbox is occupied, only the matching uplink processor waits. Canonical delivery
  and other subscriptions continue independently.
- Cancellation unblocks a waiting publisher and causes the browser to reconcile from authoritative
  state before falling back.
- A result is accepted by the browser only when both `client_instance_id` and uplink generation
  match the current tab and its request ID matches current/pending presentation work.

## Local transient and effects

- The local layer affects controller visual selection, page indication, or hacking highlight/preview
  text only.
- It never changes `controllerPresentation`, canonical revision, observer DOM, authority, gameplay,
  or preview audio.
- Authoritative updates always advance the canonical mirror when revision-valid.
- A delayed authoritative target superseded by a newer local sequence cannot replace the effective
  controller visual target or replay highlight/reveal/audio effects.
- An applicable authoritative match reconciles and clears the transient. Preview audio may play once
  from that authoritative match.
- Observers render only canonical updates and never create local transient presentation from pointer
  activity.

## Limits and closure

| Limit | Default | Result |
|---|---:|---|
| decoded protobuf message | 4 KiB | Connect resource-exhausted; no mutation |
| intent receive rate | 120/s, burst 32 | cancel limited generation; latest target may fall back unary |
| idle lifetime | 30 seconds | cancel body/context; unary fallback and fresh probe remain available |
| absolute lifetime | 5 minutes | cancel body/context and rotate to a newer generation |
| current uplinks per tab | 1 | newer replaces older |
| process concurrent uplinks | 32 | reject probe; unary remains available |
| pending server intents | 1 per uplink | latest replaces unprocessed older value |
| pending targeted results | 1 per subscription | publisher waits; cancellation unblocks it; delivered results are never replaced |

Server shutdown cancels all uplinks before or with subscription closure and then shuts down the HTTP
server within the existing application deadline.

## Fallback matrix

| Condition | Streaming action | User-visible behavior |
|---|---|---|
| Direct LAN HTTP | Do not probe | Existing unary presentation and all gameplay continue |
| Browser lacks request-stream support | Do not probe | Existing unary presentation continues |
| Ready result misses deadline | Abort generation | Latest eligible target drains unary |
| Negotiation or HTTP downgrade buffers/fails | Abort generation | Latest eligible target drains unary |
| Active stream interrupts | Invalidate generation | Latest eligible target drains unary; later probe may recover |
| Role/broadcast/terminal/context changes | Cancel and clear transient | Canonical view wins; no stale target is resent |
| Subscription reconnects | Cancel old generation; attach client ID to new `Subscribe` | Probe a newer generation before streaming resumes |
| Rate/concurrency/lifetime limit | Close or reject bounded stream | Gameplay remains usable through unary |

## Acceptance

- With the request-body consumer stalled, a 100-target sweep retains at most one handed-off plus one
  pending newest browser intent; the final eligible target is eventually sent with no duplicate
  semantic send.
- A 100-target server sweep has at most one executing plus one pending newest server intent.
- With the targeted mailbox occupied, a second intent does not enter the coordinator until the prior
  result is published; canonical updates remain deliverable, and cancellation leaves no blocked
  publisher or goroutine.
- Every processed request ID receives its matching targeted result while its subscription and
  generation remain active. Otherwise the uplink terminates and the browser reconciles before
  handing only the newest eligible target to unary fallback.
- Local controller visual feedback appears by the next animation frame under delayed authority.
- After input stops, controller and at least two observers converge to the final accepted target
  within one second.
- Reassignment, context replacement, reconnect, stale generation, and multiple tabs produce no
  accepted stale mutation and no cross-tab result.
- No delayed superseded controller highlight/reveal/audio effect replays.
