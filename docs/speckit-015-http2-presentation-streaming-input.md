# Spec Kit Input: HTTP/2 Presentation Streaming

**Prepared**: 2026-08-21

**Prospective feature**: `015-http2-presentation-streaming`

**Status**: Investigation complete; no feature branch or specification created

## Investigation Outcome

End-to-end HTTP/2 is feasible in this project, including through ngrok. A true
single-request browser-to-Go ConnectRPC bidirectional stream is not currently a
viable portable design:

- Connect-Go supports client and bidirectional streams over HTTP/2.
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

Streaming alone will not remove pointer latency. The feature must also:

- Render a controller-local transient hover by the next animation frame.
- Coalesce pending network intents using latest-value semantics.
- Keep canonical presentation and observer rendering server-authoritative.
- Prevent delayed authoritative revisions from replaying superseded animation
  or audio.

## Current Project Impact

- Enable HTTP/1.1 plus h2c on the player `http.Server` in
  `internal/player/server.go`.
- Enable h2c on the incoming and outgoing sides of the authenticated reverse
  proxy in `internal/tunnel/public_ingress.go`.
- Pass `ngrok.WithUpstreamProtocol("http2")` in `internal/tunnel/ngrok.go`.
- Add a client-streaming RPC alongside `SetPresentation` in
  `proto/fallout/terminal/player/v1/player.proto`.
- Exempt only the streaming procedure from whole-request buffering in
  `internal/player/http.go`; otherwise the Connect handler cannot consume
  messages until the request closes.
- Implement a browser Connect request-stream transport using Fetch
  `ReadableStream`, generated protobuf messages, and the generated service
  descriptor. Do not create handwritten network DTOs or an RPC router.
- Add an ephemeral per-tab `client_instance_id` so uplink results can be routed
  through the correct existing `Subscribe` stream.
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

Eliminate visible lag and stale selected-symbol animation when the active
player rapidly moves across hacking targets. Introduce an optional HTTP/2
client-streaming ConnectRPC uplink for high-frequency controller presentation
intents while retaining the existing Subscribe server stream as the
authoritative downlink.

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

Streaming must be capability-detected. Unsupported browsers, direct LAN HTTP,
stream negotiation failure, or stream interruption must fall back automatically
to the existing unary SetPresentation behavior without affecting navigation,
guessing, pattern activation, subscription recovery, or public authentication.
The first version must not remove SetPresentation.

Functional requirements:

- Add a protobuf client-streaming PresentationUplink RPC. Do not add a browser
  bidi RPC.
- Keep Subscribe as the authoritative server-streaming downlink.
- Use a per-tab ephemeral client_instance_id to associate the uplink with the
  correct subscription and reject superseded stream generations.
- Every intent must include request identity, recognition handle, broadcast,
  terminal, context key, and semantic presentation target.
- Validate controller authority and current broadcast, terminal, and context
  when processing each intent—not only when the stream opens.
- Maintain a capacity-one latest-value mailbox per uplink. Replace queued,
  unprocessed intents with the newest semantic target.
- Preserve coordinator ordering for mutations that are actually accepted.
- Deliver authoritative updates and targeted rejection/results through
  Subscribe because browser Fetch cannot read the client-stream response while
  the request remains open.
- Bypass whole-body buffering only for the streaming procedure. Retain host,
  same-origin, Basic Auth, protobuf per-message size, decompression, and schema
  validation protections.
- Add per-stream rate limits, idle lifetime/rotation, maximum concurrent uplinks,
  cancellation, shutdown cleanup, and role/context invalidation.
- A malformed, oversized, stale, unauthorized, or slow stream must not mutate
  canonical state or disrupt other clients.
- Local transient hover must never grant authority, alter canonical state, or
  trigger duplicate shared audio.
- Persistent session and player-configuration formats are unchanged.
- Other gameplay mutation RPCs remain unary.

Success criteria:

- A latency-backed hacking-grid sweep no longer trails the pointer through stale
  selected-symbol animations or preview cues.
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

The current `fix/004-player-sessions-control` worktree contains uncommitted
BUG-010 changes. Finish or checkpoint that work before starting the new feature.

1. `$speckit-constitution` with the amendment above.
2. `$speckit-feature-numbering-before-specify http2-presentation-streaming`
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
- Coalesce on both sides of the latency boundary so a faster transport does not
  merely deliver more stale hover targets.
- Rotate long-lived upload streams or otherwise bound their resource lifetime.
- Apply protobuf message-size limits per stream message. A fixed whole-body
  limit cannot be applied to an indefinitely open request body.
- Preserve same-origin checks, public Basic Auth, controller authorization,
  revision ordering, stream shutdown, and slow-client isolation.

## Primary References

- [Connect-Go streaming](https://connectrpc.com/docs/go/streaming/)
- [Connect-Web v2.1.2 transport source](https://github.com/connectrpc/connect-es/blob/v2.1.2/packages/connect-web/src/connect-transport.ts)
- [Chrome: streaming requests with Fetch](https://developer.chrome.com/docs/capabilities/web-apis/fetch-streaming-requests)
- [Go `net/http.Protocols`](https://pkg.go.dev/net/http#Protocols)
- [ngrok end-to-end HTTP/2 support](https://ngrok.com/blog/http2-support)
- [ngrok Go SDK `WithUpstreamProtocol`](https://pkg.go.dev/golang.ngrok.com/ngrok/v2#WithUpstreamProtocol)
