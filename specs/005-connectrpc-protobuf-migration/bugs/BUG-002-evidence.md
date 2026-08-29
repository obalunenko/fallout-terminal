# BUG-002 Reproduction Evidence

**Captured**: 2026-08-13
**Public endpoint**: configured `https://fixed-host.example`
**Credential handling**: Basic Auth values were supplied only through process environment and are omitted from this record.

## Reproduction

A clean headless Chromium context opened the authenticated public URL. Within
15 seconds it observed:

- player shell navigation: HTTP `200`;
- all eight generated `SoundManifest` calls: HTTP `200` with
  `application/proto`;
- generated `Subscribe`: request issued as
  `application/connect+proto` with a five-byte empty-message frame;
- no `Subscribe` response headers or first value before timeout;
- `#connOverlay` still visible with `УСТАНОВКА СВЯЗИ...`;
- no recognition handle stored and no player presentation rendered.

The same clean-browser journey against `http://127.0.0.1:3690` received the
first snapshot, stored an opaque recognition handle, and hid the overlay.

## Boundary isolation

Redacted ngrok inspection showed that the public request preserved the expected
Host and Origin (`fixed-host.example`), carried Connect protocol version
`1`, and eventually recorded HTTP `200` with
`application/connect+proto`. The recorded request duration matched browser
cancellation rather than first-snapshot delivery.

Byte-count probes isolated the failure:

| Route | ngrok traffic-policy processing | Subscribe request body | Result within 3 seconds |
|---|---:|---:|---|
| local listener | no | five-byte Connect frame | HTTP 200, first frame delivered |
| configured ngrok endpoint | yes | empty body | HTTP 200, first frame delivered |
| configured ngrok endpoint | yes | five-byte Connect frame | timeout, zero response bytes |
| temporary ngrok endpoint | no | five-byte Connect frame | HTTP 200, first frame delivered |

The temporary unprotected endpoint was stopped immediately after this
comparison. These observations exclude Host/Origin rejection, credential
propagation, response content type, the Connect handler, and browser decoding as
the primary cause. A follow-up run proved that even a non-matching rule in an
attached traffic-policy file retains the failure, while ngrok's deprecated
agent-level Basic Auth streams but would expose credentials in process
arguments. The safe recovery is therefore to launch the ngrok forwarder without
a traffic-policy file or credential-bearing arguments and enforce the identical
credential pair inside the application for every request whose Host exactly
matches the configured public domain, before static or Connect capabilities
run.

## Security constraints for the fix

- every unauthenticated public static or Connect request remains HTTP `401`;
- local/LAN access remains unchallenged;
- public Host/Origin validation remains unchanged;
- credentials remain out of URLs, logs, schemas, runtime status, and persisted
  state;
- all other public static and unary traffic remains protected by the ngrok
  Basic Auth action.
