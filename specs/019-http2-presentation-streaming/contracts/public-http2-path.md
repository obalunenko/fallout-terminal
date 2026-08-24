# Contract: Public HTTP/2 Path

## Required topology

```text
browser HTTPS/HTTP2
  → ngrok edge
  → authenticated ingress h2c
  → player server h2c
  → generated ConnectRPC PresentationUplink handler
```

HTTP/2 at the browser/provider edge alone is insufficient. Both application-owned local request
hops must independently observe HTTP/2 for the open client stream.

## Listener and transport matrix

| Boundary | Accepted/selected protocols | Authentication | Compatibility |
|---|---|---|---|
| Player listener (`internal/player/server.go`) | HTTP/1.1 and unencrypted HTTP/2 | Existing Host/same-origin/player authorization rules | Direct local/LAN HTTP/1.1 static, unary, and `Subscribe` remain available |
| Ingress listener (`internal/tunnel/public_ingress.go`) | HTTP/1.1 and unencrypted HTTP/2 | Exact active public Host plus Basic Auth before proxying | Existing HTTP/1.1 public static/unary/server-stream traffic remains available |
| Ingress → player reverse proxy | Unencrypted HTTP/2 | `Authorization` and `Proxy-Authorization` stripped | No HTTP/1 fallback for this owned upstream; failure is public-only |
| ngrok → ingress upstream | HTTP/2 selected by SDK | Provider endpoint reaches ingress; ingress owns Basic Auth | Public failure does not affect direct player listener |

Both servers use Go 1.27 `http.Protocols` with HTTP/1 and unencrypted HTTP/2 enabled. The reverse
proxy owns a configured `http.Transport`; it does not mutate `http.DefaultTransport`.

## Exact ngrok construction

`internal/tunnel/ngrok.go` must construct the upstream exactly as:

```go
ngrok.WithUpstream(request.UpstreamURL, ngrok.WithUpstreamProtocol("http2"))
```

`ngrok.WithUpstreamProtocol("http2")` is an upstream option, not an endpoint option. Reserved-domain
endpoint options remain separate and unchanged.

## Admission and body handling

1. Ingress deny-all/activation, exact Host validation, constant-time Basic Auth comparison, and
   authorization stripping remain before reverse-proxy dispatch.
2. Player valid-Host and same-origin checks remain before the generated handler.
3. Static requests and every existing RPC retain current whole-body buffering and encoded-body
   limits.
4. Only the generated `PresentationUplink` procedure bypasses whole-body buffering so Connect may
   parse frames while the request stays open.
5. Per-message protobuf read/decompression limits and schema validation remain active on the stream.
6. Unsupported public RPC paths remain generated Connect `unimplemented`; the stream exemption is
   an exact procedure comparison, never a prefix or service-wide bypass.

## Cancellation and shutdown

- Browser abort propagates through ngrok and ingress request contexts to the player handler.
- Reverse-proxy failure cancels only the affected request and does not stop local/LAN service.
- Public reconfiguration first denies ingress, withdraws/closes the endpoint, and closes ingress
  under existing lifecycle ordering; active uplinks terminate through request cancellation.
- Player shutdown cancels uplinks/subscriptions before HTTP shutdown and remains bounded by the
  application deadline.

## Deterministic evidence

Automated tests must prove:

- an HTTP/1.1 client reaches player static, unary, and `Subscribe` behavior unchanged;
- an unencrypted HTTP/2 client reaches the player and the handler observes `ProtoMajor == 2`;
- the ingress accepts both HTTP/1.1 and h2c after correct Host/Basic Auth;
- the ingress handler observes HTTP/2 from an h2c caller;
- the player handler independently observes HTTP/2 from the ingress reverse proxy;
- wrong/missing auth reaches neither player nor stream handler;
- authorization headers are absent at the player hop;
- only `PresentationUplink` is readable before request EOF;
- ngrok adapter tests inspect the SDK upstream value/protocol without logging credentials.

Protocol observers are injected test seams or local test handlers. Production responses must not
expose topology through diagnostic headers.

## Real ngrok evidence

The opt-in integration uses user-supplied credentials to:

1. start an HTTP/1.1+h2c player probe server;
2. start an authenticated HTTP/1.1+h2c ingress with h2c proxy transport;
3. start the ngrok endpoint with the exact `http2` upstream option;
4. authenticate a generated client and establish `Subscribe` with `client_instance_id`;
5. open `PresentationUplink`, send its open frame, and observe targeted ready before closing the body;
6. send an intent and observe targeted result plus canonical update;
7. record HTTP/2 independently at ingress and player;
8. cancel/reconnect and prove a newer generation works.

When credentials or external connectivity are unavailable, this evidence is reported as `NOT RUN`.
Deterministic fixtures remain required but are not described as real-provider proof.

## Verbatim constraints preserved

- `PresentationUplink`
- `Subscribe`
- `SetPresentation`
- `client_instance_id`
- `http2`
- `ReadableStream`
- `ngrok.WithUpstream(request.UpstreamURL, ngrok.WithUpstreamProtocol("http2"))`
- `internal/player/server.go`
- `internal/tunnel/public_ingress.go`
- `internal/tunnel/ngrok.go`
- `specs/019-http2-presentation-streaming`
