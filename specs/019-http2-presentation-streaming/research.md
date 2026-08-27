# Research: HTTP/2 Presentation Intent Streaming

## Native h2c support

**Decision**: Configure Go 1.27 `http.Server.Protocols` with HTTP/1 and unencrypted HTTP/2 for both
the player server and private ingress. Configure the ingress `http.Transport.Protocols` for
unencrypted HTTP/2 to the player upstream.

**Rationale**: The repository targets Go 1.27, whose standard `net/http` directly supports
unencrypted HTTP/2. It preserves HTTP/1.1 on the same listeners and avoids the deprecated
`golang.org/x/net/http2/h2c` handler, including its first-upgrade-request buffering behavior.

**Alternatives considered**: `h2c.NewHandler` was rejected because the standard library now owns the
feature and the deprecated wrapper has less desirable upgrade semantics. A second HTTP/2-only
listener was rejected because it would duplicate lifecycle and violate the one-player-server
boundary.

## Exact ngrok upstream protocol

**Decision**: Change only the upstream construction to
`ngrok.WithUpstream(request.UpstreamURL, ngrok.WithUpstreamProtocol("http2"))`.

**Rationale**: The pinned ngrok v2.1.4 source defines `WithUpstreamProtocol` as an `UpstreamOption`.
The selected call therefore requests HTTP/2 on the ngrok-to-ingress hop without altering endpoint
policy, Basic Auth ownership, reserved-domain handling, or lifecycle.

**Alternatives considered**: Passing the option with endpoint options does not type-check and would
configure the wrong boundary. Relying on provider auto-negotiation was rejected because the input
requires explicit and testable HTTP/2 upstream selection.

## Browser Connect request-stream transport

**Decision**: Add a small `presentation-uplink.js` transport wrapper. Delegate unary and
server-streaming methods to the existing Connect-Web transport, and implement only the generated
`PresentationUplink` client-stream method with Fetch `ReadableStream`, `duplex: "half"`, the
generated `PlayerService` method descriptor, generated message schemas, and official exports from
`@connectrpc/connect/protocol` and `@connectrpc/connect/protocol-connect` for method URL, headers,
serialization, and envelopes.

**Rationale**: Connect-Web 2.1.2 explicitly throws for non-server-streaming request bodies, while
the core package already exports the same framing primitives used by its own transport. Exact npm
pins make the internal protocol-connect helpers reproducible. Generated descriptors remain the
router and schema source; the wrapper owns only browser request-body plumbing.

**Alternatives considered**: Hand-encoding Connect frames or writing a path switch was rejected by
the protobuf/Connect constitution. Replacing the ordinary player transport was rejected because it
would expand risk to every unary and `Subscribe` call. WebSocket, WebTransport, and bidi were
rejected as explicitly out of scope.

## End-to-end capability proof

**Decision**: The first uplink frame is an open/bind frame. After validating its tab identity,
generation, recognition, and current physical subscription, the server sends a targeted ready
status through `Subscribe` while the request body remains open. The browser enables streaming only
after observing that status before a short probe deadline.

**Rationale**: JavaScript API checks prove only that the browser can construct the request. A ready
status delivered over the independent downlink proves the body frame reached the application
through the selected intermediaries without waiting for body completion. It also binds subsequent
results to the correct current tab.

**Alternatives considered**: A unary health RPC cannot detect request buffering. Mutating a current
presentation as a probe risks duplicate canonical work. Reading the client-stream response while
the body remains open is not portable Fetch behavior and would turn the design into an unsupported
bidi assumption.

## Protobuf frame shape

**Decision**: Add a `PresentationUplinkRequest` oneof containing one opening frame or one
`PresentationIntent`; add a targeted `PresentationUplinkResult` variant to `SubscriptionMessage`.
Use an empty `PresentationUplinkResponse` for normal stream closure and keep `ActionResult` as the
nested authoritative result vocabulary.

**Rationale**: An explicit opening frame separates stream binding/probing from canonical intent and
lets every actual intent retain the required request, recognition, broadcast, terminal, context,
and semantic target fields. Reusing `ActionResult` preserves reason/revision semantics and avoids a
parallel rejection vocabulary.

**Alternatives considered**: Streaming `SetPresentationRequest` directly cannot carry tab identity
or generation. Repeating the open binding on every intent adds bytes without improving validation;
the processor checks the bound generation and live subscription for every message.

## Coordinator and targeted routing

**Decision**: Extend the narrow `ConnectCoordinator` boundary with the existing connection-aware
`DispatchPlayerAction(connectionID, command)` operation. Bind the uplink to the physical
subscription connection and offer its returned result to that subscription after canonical dispatch.

**Rationale**: `DispatchPlayerActionForRecognition` intentionally chooses one sorted connection and
cannot distinguish two tabs sharing one logical recognized player. The control service already
implements physical-connection authority, idempotency, ordering, revision stamping, and effects.
Using it avoids transport rules in the canonical service and routes stream results to the originating
tab without changing ordinary unary response behavior.

**Alternatives considered**: Adding tab concepts to `internal/control` was rejected as transport
leakage. Broadcasting results to the logical session was rejected because another tab could consume
them. Routing every coordinator `Effect.Result` through `Subscribe` was rejected because it would
duplicate existing unary results.

## Backpressure and ordering

**Decision**: Give each uplink a capacity-one replaceable intent mailbox and one ordered processor.
Give each subscription a separate capacity-one non-lossy targeted-result mailbox while preserving
the existing canonical revision queue unchanged. Publishing a processed result waits only for that
targeted slot or lifecycle cancellation, and the same uplink dispatches no further canonical
mutation until publication succeeds. When both queues are ready, drain one canonical update first,
then service the pending targeted result before another canonical update.

**Rationale**: Receiver and processor separation lets HTTP continue consuming newer frames while a
canonical mutation is executing. Latest replacement bounds work at the latency boundary. A separate
result mailbox makes it structurally impossible for an acknowledgement/rejection to consume,
reorder, evict, or overflow-close the canonical queue. Waiting in only the originating uplink
processor preserves delivery of processed results and prevents a slow tab from causing additional
canonical mutations while its result route is blocked. Bounded canonical priority preserves revision
ordering without allowing continuous canonical traffic to starve a targeted result.

**Alternatives considered**: Processing directly in the receive loop turns TCP flow control into an
unbounded stale-intent queue. Enlarging the canonical queue for results violates the delivery
invariant. Replacing or silently dropping an undelivered processed result was rejected because it
can leave the active tab without its targeted rejection/result. Blocking the canonical subscription
queue or holding coordinator locks while waiting was rejected; only the originating uplink
processor may wait.

## Resource limits

**Decision**: Use fixed defaults of 120 intent frames per second with burst 32, 30 seconds idle
lifetime, five minutes maximum lifetime, one current uplink per tab, 32 concurrent uplinks
process-wide, and existing 4 KiB decoded protobuf message limits. Rotate or reopen on demand after a
normal limit close.

**Rationale**: The browser coalesces toward animation cadence, so 120/s leaves headroom while making
abusive input finite. The existing game normally has four to seven clients; 32 permits multi-tab
testing and stale overlap without permitting unbounded goroutines. Idle and absolute lifetimes bound
provider and server resources without making the optimization a prerequisite.

**Alternatives considered**: User-configurable limits add persistence and UI scope with no user
value. Unlimited or connection-lifetime streams violate the explicit bounded-resource requirement.
A new token-bucket dependency is unnecessary for these fixed limits.

**Implementation note**: The exact `PresentationUplink` HTTP boundary wraps the live request body
with activity-aware idle and absolute timers. Timeout cancellation closes the body and request
context so a blocked Connect receive cannot outlive either deadline. Per-frame rate enforcement and
process/tab concurrency remain in the uplink runtime.

## Local transient and reconciliation

**Decision**: Keep a distinct visual-only local presentation keyed by authoritative context and
monotonic local sequence. Render it on the next animation frame, but never assign it to the
authoritative `controllerPresentation`, never expose it to observers, and never call preview audio
from it. Clear or reconcile it only on applicable canonical state, context/authority invalidation,
or transport reset.

**Rationale**: This gives immediate feedback without violating server authority. The local sequence
lets delayed older revisions update the canonical mirror without overwriting a newer pointer target
or replaying a superseded local effect.

**Alternatives considered**: Optimistically replacing the canonical mirror was rejected because it
would diverge from observers and grant stale input visual authority. Local preview audio was
rejected because the accepted contract keeps audio authoritative.

## Fallback ownership

**Decision**: Retain BUG-010's unary dispatcher unchanged as the compatibility engine. The uplink
adapter accepts semantic targets from the same client queue only after a successful probe; any
unsupported API, HTTP page origin, failed probe, interruption, rejection of the current generation,
or rotation hands the latest eligible target back to unary dispatch.

**Rationale**: One semantic queue prevents duplicate intent across transports and guarantees the
latest target survives a stream failure. Direct LAN HTTP never attempts request streaming.

**Alternatives considered**: Parallel unary and stream dispatch can double-mutate one target.
Removing unary was rejected by the feature scope and browser portability constraints.

## HTTP/2 verification

**Decision**: Assert `request.ProtoMajor == 2` separately at the ingress handler and player handler
using deterministic test observers, while also proving HTTP/1 requests still work. Extend the
credential-gated ngrok integration to send an open generated client stream and collect both local
observations.

**Rationale**: Browser-to-edge HTTP/2 evidence alone says nothing about the two loopback hops where
request buffering matters. Independent handler observations prove the effective protocol after each
forwarding boundary.

**Alternatives considered**: Provider dashboard or client ALPN evidence was rejected as incomplete.
Production diagnostic headers were rejected because they expose internal topology and are not needed
for runtime behavior.
