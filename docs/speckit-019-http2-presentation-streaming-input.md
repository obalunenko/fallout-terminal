# Spec Kit Input: HTTP/2 Presentation Streaming

**Prepared**: 2026-08-21

**Updated**: 2026-08-24

**Prospective feature**: `019-http2-presentation-streaming`

**Status**: Investigation updated; no feature branch or specification created

## Investigation Outcome

End-to-end HTTP/2 remains feasible in this project, including through ngrok. A
true single-request browser-to-Go ConnectRPC bidirectional stream is not a
portable design for the accepted browser client:

- Connect-Go supports client-, server-, and bidirectional-streaming RPCs. Bidi
  requires end-to-end HTTP/2; the planned browser request stream also requires
  HTTPS over HTTP/2 or HTTP/3 because browser Fetch rejects streaming request
  bodies on HTTP/1.x.
- The installed Connect-Web 2.1.2 transport supports unary and server-streaming
  browser calls, but rejects request-streaming methods.
- Browser Fetch request streams are half-duplex: JavaScript cannot consume the
  response until it finishes sending the request body.
- Ngrok can terminate public HTTPS/HTTP2 and forward HTTP/2 cleartext (`h2c`) to
  the local application.

The recommended browser design is a pair of coordinated Connect streams:

```text
Browser -- client-stream PresentationUplink --> Go
Browser <-- existing server-stream Subscribe -- Go
```

This provides effective two-way streaming while respecting browser Fetch
constraints. Unsupported browsers and direct LAN access retain the unary
`SetPresentation` RPC.

BUG-010 already prevents stale request buildup with the existing unary
`SetPresentation` path: the browser retains one in-flight request and one
replaceable latest desired semantic target. Superseded unsent targets therefore
produce no canonical revision, render, reveal restart, or preview cue. The
remaining opportunity is controller-visible round-trip latency, not the stale
replay defect itself.

Streaming alone will not remove that remaining pointer latency. The prospective
feature must also:

- Render a controller-local transient hover by the next animation frame.
- Coalesce pending network intents using latest-value semantics.
- Keep canonical presentation and observer rendering server-authoritative.
- Prevent delayed authoritative revisions from replaying superseded animation
  or audio.

## Current Project Impact

- Enable HTTP/1.1 plus h2c on the player `http.Server` in
  `internal/player/server.go`.
- Enable HTTP/1.1 plus h2c on the incoming `http.Server` and configure the
  reverse proxy's outgoing transport for h2c in
  `internal/tunnel/public_ingress.go`.
- Construct the ngrok upstream as
  `ngrok.WithUpstream(request.UpstreamURL, ngrok.WithUpstreamProtocol("http2"))`
  in `internal/tunnel/ngrok.go`; `WithUpstreamProtocol` is an upstream option,
  not an endpoint option.
- Add a client-streaming RPC alongside `SetPresentation` in
  `proto/fallout/terminal/player/v1/player.proto`.
- Extend `SubscribeRequest` with the per-tab client instance identity and
  extend `SubscriptionMessage` with a targeted `ActionResult`-equivalent
  presentation result/rejection variant. The current downlink contains only
  snapshots and compound updates.
- Extend `internal/player/handler.go` and `internal/player/stream.go` so the hub
  can bind an uplink generation to one physical tab subscription and offer a
  result to that subscription without broadcasting it to every tab in the
  logical player session or allowing an ephemeral result to disturb canonical
  revision delivery.
- Exempt only the streaming procedure from whole-request buffering in
  `internal/player/http.go`; otherwise the Connect handler cannot consume
  messages until the request closes.
- Implement a browser Connect request-stream transport using Fetch
  `ReadableStream`, generated protobuf messages, and the generated service
  descriptor. Do not create handwritten network DTOs or an RPC router.
- Add an ephemeral per-tab `client_instance_id` plus an explicit uplink
  generation so results can be routed through the correct current `Subscribe`
  stream and an older stream from the same tab can be rejected.
- Keep unary fallback because browser support is not portable and direct LAN
  browser HTTP cannot rely on h2c request streaming.

## Constitution Amendment Input

Run `$speckit-constitution` with the following input before specifying the
feature. The current Principle III requires browser mutations to be unary and
prohibits requiring browser request streams.

```text
Amend Principle III, “Use ConnectRPC and Keep State Server-Authoritative.”

Retain unary RPCs as the portable default for browser-originated canonical
mutations. Permit an optional ConnectRPC client-streaming transport for
high-frequency, ephemeral presentation intents when all of the following hold:

1. The deployment proves end-to-end HTTP/2 request-stream support.
2. The RPC and every message remain protobuf-defined and generated.
3. A functionally equivalent unary fallback remains available for unsupported
   browsers, direct LAN access, and failed stream negotiation.
4. Authoritative state, revisions, validation, authorization, and rejection
   remain owned by Go and are published through the server subscription stream.
5. Client-stream input is bounded, rate-limited, latest-wins, cancellable, and
   invalidated on controller, broadcast, terminal, or context changes.
6. Client-streaming is an optimization and must never be required for basic
   player operation.
7. Browser bidirectional request/response streaming remains prohibited until
   supported portably by the accepted browser transport.

Permit controller-local transient pointer presentation only when it is clearly
non-canonical, produces no shared gameplay mutation by itself, and reconciles
with authoritative subscription updates without replaying superseded effects.
```

## Feature Specification Input

After the constitution amendment, run feature numbering and pass the following
input to `$speckit-companion-specify`.

```text
Feature: HTTP/2 presentation-intent streaming

Reduce the remaining controller-visible round-trip latency when the active
player rapidly moves across hacking targets. Preserve BUG-010's bounded
latest-target dispatch and no-stale-replay guarantees while introducing an
optional HTTP/2 client-streaming ConnectRPC uplink for high-frequency controller
presentation intents. Retain the existing Subscribe server stream as the
authoritative downlink and `SetPresentation` as the portable fallback.

P1 — Responsive controller presentation

When the active controller moves across hacking symbols, passwords, or patterns,
the target under the pointer must be rendered locally by the next animation
frame. Network transmission must retain only the newest unsent semantic target.
Delayed authoritative updates must not replay superseded highlight animations
or preview audio. The final eligible target must converge on all assigned views.

P2 — HTTP/2 streaming public path

The public path must support:
browser HTTPS/HTTP2 → ngrok → authenticated ingress h2c → player server h2c.

The player server and ingress must retain HTTP/1.1 compatibility. The ngrok SDK
must request HTTP/2 for its upstream connection. Automated verification must
prove that the request reaching both local hops uses HTTP/2 rather than merely
proving HTTP/2 between the browser and ngrok edge.

P3 — Portable fallback and recovery

Streaming must be capability-detected and verified with an end-to-end probe;
detecting `ReadableStream` or `Request.duplex` in JavaScript alone is
insufficient. Unsupported browsers, direct LAN HTTP, failed probes, stream
negotiation failure, or stream interruption must fall back automatically to the
existing unary SetPresentation behavior without affecting navigation, guessing,
pattern activation, subscription recovery, or public authentication. The first
version must not remove SetPresentation.

Functional requirements:

- Add a protobuf client-streaming PresentationUplink RPC. Do not add a browser
  bidi RPC.
- Keep Subscribe as the authoritative server-streaming downlink.
- Use a per-tab ephemeral `client_instance_id` and explicit uplink generation to
  associate the uplink with the correct subscription and reject superseded
  generations from the same tab.
- Every intent must include request identity, recognition handle, broadcast,
  terminal, context key, and semantic presentation target.
- Validate controller authority and current broadcast, terminal, and context
  when processing each intent—not only when the stream opens.
- Maintain a capacity-one latest-value mailbox per uplink. Replace queued,
  unprocessed intents with the newest semantic target.
- Preserve coordinator ordering for mutations that are actually accepted.
- Deliver authoritative updates and targeted rejection/results through
  Subscribe because browser Fetch cannot read the client-stream response while
  the request remains open. Targeted ephemeral results must not consume,
  reorder, or evict canonical revision delivery for the tab.
- Bypass whole-body buffering only for the streaming procedure. Retain host,
  same-origin, Basic Auth, protobuf per-message size, decompression, and schema
  validation protections.
- Add per-stream rate limits, idle lifetime/rotation, maximum concurrent uplinks,
  cancellation, shutdown cleanup, and role/context invalidation.
- A malformed, oversized, stale, unauthorized, or slow stream must not mutate
  canonical state or disrupt other clients.
- Key local transient hover by context and local intent sequence so an older
  authoritative revision cannot overwrite a newer pointer target. The transient
  layer must never grant authority or alter canonical state; it is visual-only,
  while preview audio remains tied to an applicable authoritative update.
- Persistent session and player-configuration formats are unchanged.
- Other gameplay mutation RPCs remain unary.

Success criteria:

- A latency-backed hacking-grid sweep preserves BUG-010's bounded dispatch and
  no-stale-highlight/reveal/audio guarantees while the controller-local
  transient presentation tracks the pointer.
- Local controller feedback appears by the next animation frame.
- After movement stops, controller and observers converge on the final eligible
  target without rendering superseded intermediate effects.
- Repeated movement within the same semantic target sends no duplicate intent
  and triggers no duplicate cue.
- Role reassignment, terminal changes, context changes, reconnects, and multiple
  tabs invalidate obsolete uplinks safely.
- Public ngrok integration proves HTTP/2 reaches the ingress and player server.
- HTTP/1.1 LAN and unsupported-browser tests prove unary fallback.
- Existing Connect unary, Subscribe, public-auth, generated-contract, race,
  browser, native, and packaging gates remain green.

Out of scope:

- Removing unary SetPresentation in this feature.
- Converting navigation, guesses, or pattern activation to streams.
- WebSocket or WebTransport protocols.
- A single-request browser Connect bidi stream.
- Persistence changes.
```

## Recommended Workflow

BUG-010 is patched in commit `805183b` on `fix/004-player-sessions-control`; its
focused rapid-hover regression passes and the worktree was clean when this
input was updated. Start the prospective feature from the intended integration
base rather than continuing to treat BUG-010 as unfinished work.

1. `$speckit-constitution` with the amendment above.
2. `$speckit-feature-numbering-before-specify http2-presentation-streaming`
   (current result: `specs/019-http2-presentation-streaming`)
3. `$speckit-companion-specify` with the feature input above.
4. `$speckit-companion-plan`
5. `$speckit-companion-tasks`
6. `$speckit-analyze`
7. `$speckit-companion-implement`

## Design Guardrails for Planning

- Do not model the browser path as one bidi RPC. It cannot expose server
  responses while its Fetch request body remains open.
- Do not remove unary fallback in the initial feature.
- Keep gameplay mutations such as navigation and guessing unary.
- Treat the local hover layer as transient rendering, never canonical state.
- Treat BUG-010's one-in-flight/one-latest unary behavior as the compatibility
  baseline and fallback, not as unfinished work to replace.
- Coalesce on both sides of the latency boundary so a faster transport does not
  merely deliver more stale hover targets.
- Require an end-to-end request-stream probe through the selected deployment
  path before enabling the uplink; JavaScript API-surface detection alone does
  not prove that an intermediary avoids buffering.
- Rotate long-lived upload streams or otherwise bound their resource lifetime.
- Apply protobuf message-size limits per stream message. A fixed whole-body
  limit cannot be applied to an indefinitely open request body.
- Preserve same-origin checks, public Basic Auth, controller authorization,
  revision ordering, stream shutdown, and slow-client isolation.

## Primary References

- [Connect-Go streaming](https://connectrpc.com/docs/go/streaming/)
- [Connect-Web v2.1.2 transport source](https://github.com/connectrpc/connect-es/blob/v2.1.2/packages/connect-web/src/connect-transport.ts)
- [Chrome: streaming requests with Fetch](https://developer.chrome.com/docs/capabilities/web-apis/fetch-streaming-requests)
- [MDN `RequestInit.duplex`](https://developer.mozilla.org/en-US/docs/Web/API/RequestInit#duplex)
- [Go `net/http.Protocols`](https://pkg.go.dev/net/http#Protocols)
- [ngrok end-to-end HTTP/2 support](https://ngrok.com/blog/http2-support)
- [ngrok Go SDK `WithUpstreamProtocol`](https://pkg.go.dev/golang.ngrok.com/ngrok/v2#WithUpstreamProtocol)
