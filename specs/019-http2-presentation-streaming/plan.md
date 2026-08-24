# Implementation Plan: HTTP/2 Presentation Intent Streaming

**Branch**: `feature/019-http2-presentation-streaming` | **Date**: 2026-08-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/019-http2-presentation-streaming/spec.md`

## Summary

Give the active controller next-frame visual response while keeping canonical presentation, observer
state, revisions, and preview audio server-authoritative. Add one optional protobuf client stream,
`PresentationUplink`, whose availability is proved through the active public ngrok path and whose
intents are bounded and latest-wins; retain `SetPresentation` as the portable fallback. Enable
HTTP/1.1 plus h2c on the player and authenticated ingress, force the ngrok and reverse-proxy upstreams
to HTTP/2, and route uplink results through the existing `Subscribe` downlink.

## Technical Context

**Language/Version**: Go 1.27; browser JavaScript modules with Node.js 20.19+ build/test tooling

**Primary Dependencies**: Existing `connectrpc.com/connect` v1.20.0,
`golang.ngrok.com/ngrok/v2` v2.1.4, `@connectrpc/connect` and
`@connectrpc/connect-web` v2.1.2, `@bufbuild/protobuf` v2.13.0, and the Go 1.27
standard `net/http` HTTP/2 protocol controls; no new runtime dependency

**Storage**: No persistent change. Client-instance, uplink-generation, probe, mailbox, result, and
local-transient state are process/tab-local and bounded.

**Testing**: Colocated Go tests with deterministic network/concurrency fakes; complete race-enabled
Go suite; Buf format/lint/breaking/generation gates; Playwright browser journeys; opt-in real ngrok
integration; arm64 packaged smoke

**Target Platform**: Wails desktop application on macOS 13+ / Apple Silicon, with HTTP/1.1 local/LAN
browser clients and modern HTTPS public browser clients

**Project Type**: Go modular desktop monolith with a Wails Overseer frontend and an embedded
same-origin player web application

**Performance Goals**: Eligible controller target feedback by the next animation frame; at most one
unprocessed uplink intent; a 100-target sweep produces no superseded effect replay; controller and
two observers converge within one second of final authoritative acceptance

**Constraints**: Preserve BUG-010 unary one-in-flight/one-latest behavior, `Subscribe` as the only
authoritative downlink, per-message 4 KiB protobuf limits, exact Host/origin/Basic Auth protections,
HTTP/1.1 local/LAN operation, process-local transient state, and the existing five-second application
shutdown budget

**Scale/Scope**: One player server, zero or one public ngrok endpoint and ingress, one current uplink
per browser tab, up to 32 concurrent uplinks process-wide, and the existing representative four-to-
seven player clients

## Constitution Check

*GATE: passed before research and re-checked after Phase 1 design.*

| Constitution principle | Pre-design | Post-design assessment |
|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | PASS — player transport remains in `internal/player`, public forwarding in `internal/tunnel`, browser-only rendering in `frontend/client`, and all owned streams close through existing lifecycle owners. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | PASS — uplink frames, tab identity, generation, probe status, intents, and targeted results are defined in `player.proto` and regenerated for Go and ECMAScript. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | PASS — only presentation intents gain an optional client stream; generated descriptors and official Connect framing helpers are used; canonical mutation, rejection, revision, audio, and downlink remain Go-owned; unary fallback remains. |
| IV. Separate Public and Private Capabilities | PASS | PASS — the player schema adds no Overseer operation, provider credential, password, secret word, or private hacking candidate. |
| V. Evolve Schemas Safely and Reproducibly | PASS | PASS — changes are additive with stable new field numbers and one RPC; pinned generation, drift, descriptor, and breaking checks remain enabled. |
| VI. Preserve Portable Session JSON Version 1 | PASS | PASS — no session or player-configuration schema, adapter, fixture, or version changes. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | PASS — no transport is superseded: `SetPresentation` is intentionally retained as compatibility behavior, while no second stream/router/protocol is introduced. |
| Dependency Rules | PASS | PASS — existing exact pins suffice; Go 1.27 `net/http` supplies h2c and the SDK already supplies the ngrok HTTP/2 upstream option. |
| Secret and Credential Governance | PASS | PASS — existing scoped Keychain use and ingress Basic Auth remain unchanged; the stream receives credentials only through ordinary same-origin public HTTP admission and public messages remain secret-free. |
| Go Development Tool Modules | PASS | PASS — existing isolated Buf and generators remain the only tools; no global or new development tool is added. |
| Testing and Quality Gates | PASS | PASS — schema, race, HTTP protocol, fallback, multi-tab, browser, real-provider, build, and package evidence are explicit; unavailable external evidence is `NOT RUN`. |
| Development Workflow / build ownership | PASS | PASS — schema-first generation and `cmd/build` ownership are preserved; Make remains an optional thin wrapper. |

There are no constitution violations and no Complexity Tracking exception.

## Project Structure

### Documentation

```text
specs/019-http2-presentation-streaming/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── contracts/
│   ├── player-protobuf.md
│   ├── presentation-uplink.md
│   └── public-http2-path.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code

```text
proto/
├── fallout/terminal/player/v1/player.proto       # additive uplink/downlink contract
└── schema-revision.txt                           # reviewed schema digest
internal/
├── gen/fallout/terminal/player/v1/               # regenerated Go messages/Connect service
├── player/
│   ├── adapter.go / adapter_test.go              # generated frames ↔ runtime mutation validation
│   ├── handler.go / handler_test.go              # Subscribe binding and PresentationUplink
│   ├── stream.go / stream_test.go                # tab routing, canonical/result queues, uplinks
│   ├── limits.go                                 # per-message/rate/lifetime/concurrency bounds
│   ├── http.go / http_test.go                    # stream-only body-buffer exemption
│   ├── server.go / server_test.go                # HTTP/1.1 plus unencrypted HTTP/2
│   └── public_stream_test.go                     # generated public stream integration
└── tunnel/
    ├── public_ingress.go / public_ingress_test.go # incoming HTTP/1.1+h2c, outgoing h2c
    ├── ngrok.go / ngrok_test.go                   # exact HTTP/2 upstream option
    └── ngrok_integration_test.go                  # real two-local-hop protocol evidence
frontend/client/
├── client.js                                     # local transient, reconciliation, fallback
├── presentation-uplink.js                        # generated-descriptor Fetch request transport
└── gen/fallout/terminal/player/v1/player_pb.js   # regenerated ECMAScript contract
tests/browser/
├── fixture-server/main.go                        # delay/protocol/failure fixtures
├── player-sessions-control.spec.mjs              # next-frame/latest/convergence journeys
├── connectrpc-player.spec.mjs                    # authenticated public stream/probe journey
└── public-access-fallback.spec.mjs                # LAN/unsupported/interruption fallback
```

**Structure Decision**: Keep all wire framing and generated-contract adaptation at the player
boundary, keep canonical ordering in the existing control service, extend the existing physical
subscription hub for tab-targeted delivery, and isolate the browser request-stream transport in one
small module used only by presentation dispatch.

## Contract and State Design

### Persistent JSON

No changes. Per-tab identity uses browser session memory, not local/session storage; uplink and local
transient state are never serialized. Existing version-1 session and player-configuration round-trip
tests remain regression gates.

### Wails Bridge and Runtime Events

No changes. The browser player remains independent of Wails and private desktop services.

### HTTP and ConnectRPC

- Add `client_instance_id` to `SubscribeRequest` and preserve absence for older clients.
- Add generated uplink-open, presentation-intent, status/result, and client-stream frame messages.
- Add `PresentationUplink(stream PresentationUplinkRequest) returns (PresentationUplinkResponse)` beside the
  existing unary `SetPresentation` and server-streaming `Subscribe`; do not add bidi.
- Extend `SubscriptionMessage` with one tab-targeted presentation result/status variant. Canonical
  snapshots and compound updates retain their existing fields and revision semantics.
- Require an open frame before intents. It binds one `client_instance_id` and monotonically newer
  uplink generation to the physical `Subscribe`; a targeted ready status observed while the request
  body remains open is the end-to-end capability proof.
- Validate every intent through the same generated adapter and canonical coordinator path as
  `SetPresentation`, with the bound physical connection used for authority and tab routing.
- Preserve host/origin/authentication/decompression/schema checks. Only the exact
  `PresentationUplink` procedure bypasses whole-body buffering; Connect's per-message read limit
  remains active.

### Runtime-State Lifecycle

- `Subscription` owns immutable tab identity, its canonical update queue, and a separate capacity-one
  non-lossy targeted result mailbox. Publishing waits for the mailbox slot or lifecycle cancellation
  and never replaces an undelivered result. While publishing is blocked, that uplink dispatches no
  further canonical mutations; targeted traffic can never consume, evict, reorder, or close the
  canonical queue. When both queues are ready, the send loop emits one canonical update first and
  then services the pending targeted result before another canonical update, preventing starvation
  without weakening canonical priority.
- The hub indexes the current subscription by physical connection and `client_instance_id`; a newer
  generation cancels the older uplink without replacing the subscription.
- Each uplink has one receiver, one capacity-one latest-intent mailbox, and one ordered processor.
  Replacement occurs before coordinator dispatch; accepted mutations still enter the sole canonical
  commit order.
- The browser uplink has one capacity-one latest-intent outbound mailbox and one request-body puller.
  A new semantic target atomically replaces the older unsent target. At most one intent is handed to
  the Fetch stream while one newer intent waits, so network backpressure cannot accumulate
  intermediate pointer or keyboard targets.
- Defaults are 120 received intent frames per second with a burst of 32, 30 seconds idle lifetime,
  five-minute maximum lifetime, one current generation per tab, and 32 concurrent uplinks
  process-wide. The exact stream HTTP boundary resets idle time on body activity and cancels a
  blocked receive at idle or absolute expiry; rate and concurrency stay in the uplink runtime.
  Limits are constants with deterministic seams in tests, not user settings.
- Disconnect, role/broadcast/terminal/context invalidation, generation replacement, cancellation,
  limit rejection, and server shutdown clear mailboxes and local transient state.
- The controller keeps a visual-only transient target keyed by context and local sequence. Rendering
  may use it immediately; observers, canonical mirrors, revisions, and preview audio continue to use
  applicable authoritative updates only.

### Platform, Tunnel, and Packaging

- Both `http.Server` instances explicitly enable HTTP/1 and unencrypted HTTP/2 through Go 1.27
  `http.Protocols`.
- The ingress reverse proxy uses an owned `http.Transport` configured for unencrypted HTTP/2 to the
  player server while retaining incoming HTTP/1.1 for existing clients.
- ngrok uses exactly
  `ngrok.WithUpstream(request.UpstreamURL, ngrok.WithUpstreamProtocol("http2"))`.
- Public Basic Auth remains at the ingress, is checked before forwarding, and is stripped before the
  player service. Local/LAN requests continue directly to the player listener.
- No new binary, runtime service, port, secret, stored setting, package resource, or dependency is
  introduced.

## Implementation Phases

### Phase 0: Research and Decisions

The integration and concurrency choices are recorded in [research.md](research.md). The load-bearing
decisions are native Go 1.27 h2c, a generated-descriptor Connect transport wrapper, an open-frame
probe bound to `Subscribe`, separate canonical/result queues, and unchanged unary compatibility.

### Phase 1: Contracts and Data Design

- [data-model.md](data-model.md) defines tab, subscription, uplink, intent, result, mailbox, transient,
  and lifecycle state.
- [player-protobuf.md](contracts/player-protobuf.md) fixes additive protobuf names, cardinality,
  presence, and compatibility.
- [presentation-uplink.md](contracts/presentation-uplink.md) defines browser/server framing,
  generated-descriptor use, probe, validation, ordering, limits, reconciliation, and fallback.
- [public-http2-path.md](contracts/public-http2-path.md) defines HTTP/1.1+h2c admission and the exact
  ngrok/ingress/player protocol evidence.

### Phase 2: Schema and Server Foundations

1. Update `player.proto`, regenerate Go/ECMAScript contracts, advance `schema-revision.txt`, and run
   format/lint/breaking/drift/descriptor privacy checks.
2. Extend adapters with structural validation for the open frame, intent, and targeted result.
3. Add tab identity and separate targeted-result delivery to `SubscriptionHub`; add generation-aware
   uplink ownership, capacity-one input, bounds, cancellation, and shutdown.
4. Implement `PresentationUplink` using the existing `control.Service.DispatchPlayerAction`
   connection-aware boundary so authority and canonical ordering remain transport-independent.
5. Exempt only the generated uplink procedure from whole-body buffering and retain Connect's
   per-message decompressed limit.

### Phase 3: HTTP/2 and Browser Integration

1. Enable HTTP/1.1 and unencrypted HTTP/2 on player and ingress servers, use an h2c-only reverse-
   proxy transport, and request the exact ngrok HTTP/2 upstream option.
2. Implement `presentation-uplink.js` as a narrow transport wrapper using `PlayerService`, the
   generated method/message descriptors, Buf `create()` normalization, and official Connect
   serialization/envelope/header helpers.
3. Generate one ephemeral `client_instance_id` per page lifetime, attach it to `Subscribe`, open a
   newer generation only on eligible HTTPS deployments, and enable it only after a targeted ready
   status arrives while the request body remains open.
4. Add a capacity-one browser outbound mailbox, next-frame visual transient presentation,
   authoritative reconciliation, stale effect suppression, lifecycle invalidation, and automatic
   BUG-010 unary fallback after unsupported capability, failed probe, interruption, or rotation. On
   stream failure, transfer only the newest still-eligible mailbox or local target to unary dispatch.

### Phase 4: Integration and Packaging

1. Prove HTTP/1 and h2c compatibility locally and capture `ProtoMajor == 2` independently at ingress
   and player hops; keep a nil-cost test observer out of public production contracts.
2. Extend Playwright fixtures and journeys for 100-target input, a stalled request-body consumer,
   next-frame local presentation, two observers, duplicate suppression,
   reassignment/context/reconnect/multi-tab invalidation, and all fallback paths. Assert that the
   browser retains at most one handed-off and one pending intent and ultimately sends the final
   eligible target.
3. Run schema, generated-contract, Go, race, frontend, browser, security, build, and arm64 packaging
   gates.
4. Run the credential-gated real ngrok client-stream journey when prerequisites exist; report
   unavailable credentials/connectivity as `NOT RUN`, never as simulated public proof.

## Verification Plan

| Surface | Automated check | Interactive/manual check | Expected result |
|---|---|---|---|
| Protobuf/generated contracts | `make proto-check` and `make proto-breaking` | Inspect public descriptor | Additive generated stream only; no private/secret imports or drift |
| Player adapters/handler/hub | `go test ./internal/player` | N/A | Per-intent validation, capacity-one input replacement, non-lossy targeted delivery, exact routing, canonical queue isolation, cancellation, and shutdown pass |
| HTTP protocol path | Focused player and tunnel tests with HTTP/1 and h2c clients | Inspect opt-in real hop evidence | HTTP/1 remains usable; both local stream hops report HTTP/2 |
| Concurrent runtime | `make test-race` | Rapid reconnect/reassign/close exercise | A full targeted mailbox blocks only its uplink processor; cancellation unblocks it with no race, leak, deadlock, stale generation, or cross-tab result |
| Browser player | `npm run build:client --prefix frontend` and `npm test --prefix tests/browser` | Delayed controller, stalled request-body consumer, plus two observer windows | Next-frame local visual, bounded latest-only transport, authoritative audio, final convergence, and unary fallback |
| Public access/security | Tunnel tests, `scripts/secret-leak-check.sh`, real ngrok opt-in | Correct/wrong Basic Auth and interrupted public stream | Auth remains fail-closed, credentials stripped, local/LAN unaffected, stream falls back safely |
| Full quality | `make check` | N/A | Formatting, vet, lint, Go/race, schema, generation, bindings all pass |
| Build/package | `go run ./cmd/build build` and `go run ./cmd/build package` | Launch packaged arm64 app and exercise controller/observer | Self-contained app serves generated client and preserves shutdown/fallback |
| Conditional real provider | `FALLOUT_NGROK_INTEGRATION=1 go test ./internal/tunnel -run TestEmbeddedNgrokSDKOptInAuthenticatedGeneratedSubscribe -count=1` plus configured Playwright journey | Public HTTPS page | PASS with generated uplink and player-hop HTTP/2 evidence, or honestly `NOT RUN`; focused ingress tests independently prove the ingress hop |

## Project-Specific Complexity Factors

- Two independent streams per eligible tab must bind without creating a browser bidi contract.
- A fast input stream must coalesce before canonical mutation while the existing ordered coordinator
  and subscription revision queue remain unchanged.
- Targeted ephemeral results share one HTTP response stream with canonical updates but cannot consume
  or evict canonical capacity.
- Browser request-body streaming is deployment-dependent and must fail back without changing
  navigation, guessing, pattern activation, auth, or subscription recovery.
- Public HTTPS terminates outside the process, while both application-owned loopback hops must be
  demonstrably h2c and still serve HTTP/1.1 compatibility traffic.
