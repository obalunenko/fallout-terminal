# Quickstart: Validate HTTP/2 Presentation Intent Streaming

This guide validates the behavior defined by [spec.md](spec.md), [plan.md](plan.md), the
[runtime data model](data-model.md), and the
[`PresentationUplink` contract](contracts/presentation-uplink.md). It does not replace the complete
quality gates in the implementation plan.

## Prerequisites

- macOS 13+ on Apple Silicon with the repository's pinned Go 1.27 and Node.js toolchain.
- Frontend and browser-test dependencies installed from their committed lockfiles.
- Optional real-public-path verification requires user-supplied ngrok credentials and external
  connectivity. Without them, record that check as `NOT RUN`.

## Contract and server checks

From the repository root, run:

```sh
make proto-check
make proto-breaking
go test ./internal/player
make test-race
```

Expected results:

- generated Go and ECMAScript contracts have no drift or breaking changes;
- the server retains only the newest unprocessed intent;
- a full targeted-result mailbox blocks only its matching uplink processor;
- canonical subscription delivery remains independent;
- cancellation and shutdown leave no blocked publisher, goroutine, or stale generation.

## Browser checks

Run:

```sh
npm run build:client --prefix frontend
npm test --prefix tests/browser
```

The browser journey must delay authority, stall the request-body consumer, sweep at least 100
distinct targets, and keep two observer windows connected. Verify:

- every eligible local target is visible by the next animation frame;
- the browser retains at most one handed-off and one pending newest intent;
- intermediate targets do not accumulate and the final eligible target is sent;
- observers render only authoritative updates and all views converge on the final accepted target;
- superseded highlight, reveal, preview, and audio effects do not replay;
- processed request IDs receive tab-targeted results while their subscription and generation remain
  active;
- interruption, unsupported request streaming, failed probe, and direct LAN HTTP use the unary
  one-in-flight/one-latest fallback.

## HTTP protocol checks

Run the focused player and tunnel tests selected by the implementation tasks. Verify independently
that:

- player and authenticated ingress accept HTTP/1.1;
- ingress-to-player request streaming uses h2c and observes `ProtoMajor == 2`;
- ngrok-to-ingress request streaming observes `ProtoMajor == 2`;
- ngrok is constructed with
  `ngrok.WithUpstream(request.UpstreamURL, ngrok.WithUpstreamProtocol("http2"))`;
- Basic Auth protects the public stream and is stripped before the player service;
- local and LAN access remain available when public access fails.

When credentials are available, run the opt-in real-provider check:

```sh
FALLOUT_NGROK_INTEGRATION=1 go test ./internal/tunnel \
  -run TestEmbeddedNgrokSDKOptInAuthenticatedGeneratedSubscribe -count=1
```

The real check opens generated `Subscribe` and `PresentationUplink`, observes ready/action/canonical
downlinks, and records the player-side HTTP/2 hop. The focused ingress test independently proves
HTTP/2 admission at ingress and h2c forwarding to player. A deterministic fake is protocol test
coverage, not evidence of a real public endpoint.

## Full build and package gates

Run:

```sh
make check
go run ./cmd/build build
go run ./cmd/build package
```

Run the packaged controller/observer presentation sweep:

```sh
scripts/state-changing-reset-native-smoke.sh \
  --presentation 'build/bin/Fallout Terminal.app'
```

The harness launches the packaged arm64 application, loads a temporary copy of the checked-in
session fixture through the native Wails UI, verifies next-frame controller feedback, one-second
controller/observer convergence, inert observer input, unary fallback on direct HTTP, and normal
application shutdown. The packaged app must remain self-contained and shut down all stream and
tunnel resources within the existing application deadline.
