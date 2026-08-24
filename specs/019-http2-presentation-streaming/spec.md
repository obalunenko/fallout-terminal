# Feature Specification: HTTP/2 Presentation Intent Streaming

**Feature Branch**: `feature/019-http2-presentation-streaming`

**Created**: 2026-08-24

**Status**: Draft

**Input**: Reduce the remaining controller-visible presentation latency with an optional HTTP/2
presentation-intent uplink, preserve authoritative shared state and BUG-010 bounded latest-target
behavior, and retain the existing unary path as the portable fallback.

## User Scenarios & Testing

### User Story 1 - Immediate controller presentation (Priority: P1)

As the active controller, I need the hacking target under my pointer to react by the next visual
frame so the terminal remains readable and responsive even when the network round trip is delayed.

**Why this priority**: Immediate visual feedback is the direct user value of the feature and can be
delivered without weakening shared authority or waiting for the streaming transport.

**Independent Test**: Delay authoritative presentation processing, sweep rapidly across hacking
symbols, passwords, and patterns, and verify that the controller's visual target follows each final
pointer position by the next frame while observers change only on accepted authoritative updates.

**Acceptance Scenarios**:

1. **Given** an active controller is viewing a hacking grid, **When** the pointer enters a new
   eligible semantic target, **Then** that target is rendered visually for the controller by the
   next animation frame without waiting for a network response.
2. **Given** the controller moves through many targets while authoritative processing is delayed,
   **When** older authoritative revisions arrive, **Then** they do not overwrite the newest local
   pointer target or replay superseded highlight, reveal, or preview-audio effects.
3. **Given** one semantic target spans multiple visual cells, **When** the pointer moves within that
   same target, **Then** the client sends no duplicate intent and plays no duplicate cue.
4. **Given** an observer views the same terminal, **When** the controller receives transient local
   feedback, **Then** the observer remains read-only and changes only when the server publishes an
   applicable authoritative revision.
5. **Given** controller authority, the active terminal, or the presentation context changes,
   **When** a transient target is still visible, **Then** it is invalidated without mutating shared
   gameplay state.

---

### User Story 2 - Stream presentation intents over the public path (Priority: P2)

As a controller using the authenticated public player URL, I need rapid presentation intents to use
one bounded low-overhead uplink when the complete path supports request streaming, while accepted
state continues to arrive through the normal shared subscription.

**Why this priority**: A reusable uplink removes repeated request setup from high-frequency
presentation input, but it is useful only after immediate local feedback remains safe.

**Independent Test**: Use a real or protocol-faithful public path, prove that a request stream
reaches both local proxy hops as HTTP/2, hold server processing while sending many targets, and
verify a capacity-one latest-value backlog, ordered accepted mutations, and tab-targeted results.

**Acceptance Scenarios**:

1. **Given** the browser and every public-path hop pass an end-to-end request-stream probe, **When**
   the active controller begins rapid presentation input, **Then** the client sends generated
   presentation messages over one current client stream and receives authoritative state through
   the existing server subscription.
2. **Given** the stream receives input faster than it can process it, **When** several unsent or
   unprocessed semantic targets accumulate, **Then** only the newest target remains eligible for
   later processing.
3. **Given** multiple tabs share one recognized player identity, **When** one tab sends an intent,
   **Then** its rejection or result is delivered only to that tab's current subscription while
   canonical updates remain visible to every assigned view.
4. **Given** an older uplink from the same tab remains connected, **When** a newer uplink generation
   becomes current, **Then** the older generation cannot submit an accepted mutation or receive a
   result meant for the newer generation.
5. **Given** a malformed, oversized, stale, unauthorized, excessive-rate, idle, or slow uplink,
   **When** the server handles it, **Then** it cannot mutate canonical state, exhaust the configured
   stream budget, or disrupt another client.

---

### User Story 3 - Fall back and recover without losing control (Priority: P3)

As a player on a LAN browser, an unsupported browser, or an interrupted public connection, I need
presentation control to continue automatically through the existing compatible request path.

**Why this priority**: Streaming is an optional optimization; basic player control cannot depend on
a browser capability or public-provider path that is not universally available.

**Independent Test**: Disable request-stream support, use direct LAN HTTP, fail the capability
probe, and interrupt an active uplink; in every case verify automatic unary presentation, unchanged
gameplay actions, subscription recovery, authentication, and final controller/observer convergence.

**Acceptance Scenarios**:

1. **Given** a browser lacks usable request-stream support or uses direct LAN HTTP, **When** the
   controller moves to a new target, **Then** presentation continues through the existing unary
   behavior without an error prompt or lost input.
2. **Given** JavaScript exposes request-stream APIs but an intermediary buffers or rejects the
   stream, **When** the end-to-end probe fails, **Then** streaming remains disabled and unary
   presentation continues.
3. **Given** an active uplink is interrupted, rotated, or times out, **When** a newer eligible target
   exists, **Then** the client falls back automatically, preserves at most the latest target, and
   later may re-enable streaming only after a fresh successful probe and generation.
4. **Given** fallback is active, **When** the controller navigates, guesses, activates a pattern, or
   reconnects its subscription, **Then** those behaviors remain unchanged and no presentation-only
   state blocks gameplay.
5. **Given** public access is authenticated, **When** streaming probes, streams, static requests,
   unary fallback, and subscriptions traverse the ingress, **Then** the same admission policy
   protects each request and credentials are not forwarded into the player service.

### Edge Cases

- The pointer leaves the hacking grid while a transient target or network intent is pending.
- The final queued target becomes invalid because the puzzle, terminal, broadcast, role, or context
  changes before processing.
- A tab reconnects its subscription while its uplink remains open, or reconnects the uplink while
  its subscription remains open.
- Two physical tabs reuse the same recognition handle but have different client instance IDs.
- A stale generation races a current generation during close, rotation, reassignment, or shutdown.
- The client stream closes normally without a readable response until its request body is closed.
- A targeted result and a canonical update are produced for the same accepted intent.
- A slow subscription cannot accept a targeted result without risking canonical revision delivery.
- The end-to-end probe succeeds initially but a later intermediary or connection downgrades or
  buffers the request stream.
- Public access is unavailable while direct LAN HTTP remains usable.
- The application shuts down while streams, probes, or queued intents are active.

## Requirements

### Functional Requirements

- **FR-001**: The active controller MUST receive visual feedback for an eligible hacking target by
  the next animation frame without waiting for authoritative network completion.
- **FR-002**: Controller-local transient presentation MUST remain visual-only and MUST NOT grant
  authority, mutate canonical state, alter observer rendering, or trigger preview audio.
- **FR-003**: Local transient presentation MUST be keyed by presentation context and local intent
  sequence so an older authoritative revision cannot overwrite a newer eligible pointer target.
- **FR-004**: Shared presentation, observer rendering, and preview audio MUST remain tied to an
  applicable newer authoritative update.
- **FR-005**: The client MUST retain only the newest distinct unsent semantic presentation target
  and MUST suppress authoritative, in-flight, queued, and same-target duplicates.
- **FR-006**: Delayed accepted updates MUST NOT replay superseded highlight, reveal, or audio
  effects, and all assigned views MUST converge on the final eligible target after movement stops.
- **FR-007**: The system MUST expose an optional protobuf-defined, generated client-streaming
  presentation-intent operation without adding a browser bidirectional stream.
- **FR-008**: The existing server subscription MUST remain the authoritative downlink for canonical
  updates and MUST also carry tab-targeted presentation results or rejections.
- **FR-009**: Each browser tab MUST use an ephemeral client instance identity and an explicit uplink
  generation to bind one current uplink to its current subscription and reject older generations.
- **FR-010**: Every presentation intent MUST identify the request, recognized player, broadcast,
  terminal, presentation context, and semantic presentation target.
- **FR-011**: The server MUST validate controller authority and the current broadcast, terminal,
  presentation context, and stream generation for every intent rather than only when a stream opens.
- **FR-012**: Each uplink MUST maintain a capacity-one latest-value mailbox that replaces queued,
  unprocessed intents with the newest semantic target.
- **FR-013**: Accepted presentation mutations MUST preserve coordinator ordering and canonical
  revision semantics.
- **FR-014**: A targeted ephemeral result MUST NOT consume, reorder, evict, or broadcast canonical
  revision delivery for the associated tab or logical player session.
- **FR-015**: The authenticated public path MUST be browser HTTPS/HTTP2 to ngrok to authenticated
  ingress h2c to player server h2c. Verification MUST observe HTTP/2 on both local hops rather than
  only between the browser and the ngrok edge.
- **FR-016**: The player server and authenticated ingress MUST retain HTTP/1.1 compatibility for
  existing static, unary, and server-streaming behavior.
- **FR-017**: Streaming MUST be enabled only after browser capability detection and an end-to-end
  probe prove that the selected deployment accepts an open request body without intermediary
  buffering.
- **FR-018**: Unsupported browsers, direct LAN HTTP, failed probes, stream negotiation failures, and
  interrupted streams MUST fall back automatically to the existing unary presentation behavior.
- **FR-019**: The existing unary presentation operation MUST remain available in the first release
  and MUST preserve BUG-010's one-in-flight/one-latest bounded behavior.
- **FR-020**: Whole-request buffering MUST be bypassed only for the presentation streaming
  procedure; all other public procedures MUST retain their existing encoded-body boundary.
- **FR-021**: The streaming procedure MUST retain host validation, same-origin enforcement, public
  authentication, per-message protobuf size limits, decompression limits, and schema validation.
- **FR-022**: The system MUST enforce configured per-stream rate limits, idle lifetime or rotation,
  maximum concurrent uplinks, cancellation, and shutdown cleanup.
- **FR-023**: Malformed, oversized, stale, unauthorized, excessive-rate, idle, canceled, or slow
  streams MUST NOT mutate canonical state or disrupt other clients.
- **FR-024**: Role reassignment, broadcast replacement, terminal changes, presentation-context
  changes, reconnects, generation replacement, and shutdown MUST invalidate obsolete transient and
  streaming state.
- **FR-025**: Navigation, guessing, pattern activation, character selection, and other gameplay
  mutations MUST remain unary and behaviorally unchanged.
- **FR-026**: Session and player-configuration formats MUST remain unchanged.
- **FR-027**: Public presentation contracts and payloads MUST remain free of private Overseer
  capabilities, provider credentials, player passwords, secret words, and private hacking data.
- **FR-028**: Public access failure or unavailability MUST NOT prevent local or LAN player access.
- **FR-029**: The ngrok tunnel MUST request an HTTP/2 upstream by constructing it with
  `ngrok.WithUpstream(request.UpstreamURL, ngrok.WithUpstreamProtocol("http2"))`;
  `WithUpstreamProtocol` MUST be supplied as an upstream option rather than an endpoint option.
- **FR-030**: The browser request-stream transport MUST use Fetch `ReadableStream`, generated
  protobuf messages, and the generated service descriptor. It MUST NOT introduce handwritten
  network DTOs or a handwritten RPC router.

### Impacted Application Surfaces

- **Composition and Wails bridge (`main.go`, `app.go`)**: Not affected beyond consuming the existing
  player and public-access lifecycles; no new desktop capability or Wails contract is introduced.
- **Domain and canonical state (`internal/domain/`, `internal/nav/`, `internal/hack/`,
  `internal/live/`, `internal/control/`)**: Affected only where ordered presentation intent metadata
  and invalidation must remain transport-independent and server-authoritative.
- **Persistence (`internal/session/`, `internal/playerconfig/`, `sessions/`)**: Not affected; no
  persistent field, version, reference, or migration changes.
- **Player transport (`internal/player/server.go`, `internal/player/http.go`,
  `internal/player/handler.go`, `internal/player/stream.go`)**: Affected by HTTP/1.1 plus h2c serving,
  client-stream handling, targeted downlink results, per-message limits, routing, cancellation, and
  shutdown.
- **Platform and public access (`internal/platform/`, `internal/tunnel/public_ingress.go`,
  `internal/tunnel/ngrok.go`)**: The ingress accepts HTTP/1.1 plus h2c, its reverse proxy uses h2c
  toward the player server, and the ngrok upstream explicitly requests HTTP/2; platform and
  secure-store contracts remain intact.
- **Overseer UI (`frontend/overseer/src/`)**: Not affected.
- **Player UI (`frontend/client/`)**: Affected by transient presentation, capability probing, a
  Fetch `ReadableStream` transport built from generated protobuf messages and the generated service
  descriptor, unary fallback, reconnection, invalidation, and audio reconciliation.
- **Tests and fixtures (`internal/**/*_test.go`, `tests/browser/`, `internal/testutil/`)**: Affected by
  protocol, concurrency, backpressure, browser, multi-tab, public-path, fallback, and shutdown
  verification.
- **Build and packaging (`go.mod`, `frontend/`, `build/`, `scripts/`)**: Generated contracts,
  frontend embedding, schema drift gates, dependency inventory, and packaged public-player smoke
  tests are affected; no new runtime dependency is assumed.

### State and Contract Requirements

- **Session/player-config compatibility**: Version-1 session JSON and player configuration retain
  their current shape and round-trip behavior.
- **Wails bridge and event contract**: No change.
- **Public player contract**: One optional presentation client stream is added alongside the
  existing unary operation; the existing server subscription gains tab identity and a targeted
  result variant while retaining snapshots and ordered compound updates.
- **Reconnect and multi-tab behavior**: A per-tab identity survives a subscription/uplink reconnect
  only for that tab's lifetime; each new uplink generation supersedes older generations without
  changing the logical recognized-player identity.
- **HTTP/static contract**: Static requests, unary procedures, and subscriptions retain existing
  host, origin, authentication, and body protections; only the streaming procedure uses an open
  request body and per-message limits.
- **Runtime-state lifecycle**: Transient presentation and uplink state are process-local, bounded,
  cancellable, non-persistent, and cleared on invalidation, disconnect, replacement, or shutdown.

### Security and Privacy Requirements

- Public Basic Auth MUST protect the capability probe and client stream before either reaches the
  player service, and credentials MUST be stripped before forwarding to that service.
- Recognition, tab identity, and generation identify routing context but do not grant controller
  authority; authority MUST be revalidated for every intent.
- Stream messages MUST be bounded and validated after decompression, and resource budgets MUST
  isolate malformed or slow clients.
- Public descriptors and payloads MUST remain free of privileged operations and secrets.

### Verification Requirements

- **Go tests**: Cover public schemas/adapters, client-stream lifecycle, capacity-one coalescing,
  targeted routing, HTTP protocol selection, reverse proxy behavior, validation, cancellation,
  limits, fallback preservation, and shutdown.
- **Race testing**: Run the complete race-enabled Go suite because player streams, hubs, tunnel
  ingress, generation replacement, and shutdown are concurrent.
- **Browser tests**: Cover immediate local presentation, no stale visual/audio replay, generated
  streaming transport, end-to-end probe outcomes, unsupported and HTTP/1.1 fallback, interruption,
  role/context invalidation, reconnect, multi-tab routing, and observer convergence.
- **Interactive verification**: Exercise controller and observer views through the canonical
  development entrypoint with delayed presentation processing and both local and public paths.
- **Packaging/release verification**: Build the self-contained arm64 application and run a packaged
  controller/observer smoke; real ngrok verification remains opt-in when credentials are supplied.

No numeric coverage threshold or repository-wide linter is defined. Verification must use the
concrete behavioral gates above.

### Key Entities

- **Presentation intent**: One semantic controller target with request identity, recognized player,
  broadcast, terminal, presentation context, and target variant.
- **Client instance**: An ephemeral per-tab identity used only to associate one tab's downlink and
  uplink; it does not grant authority.
- **Uplink generation**: The current incarnation of a tab's presentation stream; a newer generation
  invalidates every older generation for that tab.
- **Targeted presentation result**: A non-canonical acknowledgement or rejection routed to one
  physical tab without displacing canonical updates.
- **Local transient presentation**: A visual-only controller layer keyed by context and intent
  sequence that reconciles with authoritative presentation state.
- **Canonical presentation state**: The server-owned shared presentation and revision applied by
  the controller and observers.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Under delayed authoritative processing, each eligible controller pointer transition
  becomes visually observable no later than the next animation frame.
- **SC-002**: A sweep across at least 100 distinct hacking targets retains no more than one pending
  latest semantic target and produces zero superseded highlight, reveal, preview, or audio replays.
- **SC-003**: After movement stops, the controller and at least two observers converge on the final
  eligible target within one second of its authoritative acceptance.
- **SC-004**: Repeated movement within one semantic target produces zero duplicate intents and zero
  duplicate preview cues.
- **SC-005**: Reassignment, terminal and context changes, reconnects, generation replacement, and
  multiple tabs produce zero accepted mutation or targeted result from an obsolete uplink.
- **SC-006**: Real public-path verification through ngrok observes HTTP/2 at both the authenticated
  ingress and player server for the open request stream, rather than only between the browser and
  the ngrok edge. When credentials or external connectivity are unavailable, the check is reported
  as `NOT RUN`.
- **SC-007**: Direct LAN HTTP, unsupported-browser, failed-probe, negotiation-failure, and
  interruption journeys all continue through unary fallback and converge on the final target.
- **SC-008**: One malformed, oversized, unauthorized, excessive-rate, idle, or slow stream causes
  zero canonical mutations and does not prevent a healthy client from receiving its next update.
- **SC-009**: Existing navigation, guessing, pattern activation, character selection,
  subscriptions, public authentication, local/LAN access, and persistence compatibility journeys
  remain behaviorally unchanged.
- **SC-010**: Formatting, static analysis, complete Go tests, race-enabled Go tests, deterministic
  contract generation, schema compatibility, frontend production build, complete browser tests,
  and arm64 packaging all pass; credential-dependent real-public verification is reported as pass
  or `NOT RUN` rather than replaced by a fake.

## Assumptions

- The accepted public deployment serves the browser over HTTPS and can be configured to forward
  HTTP/2 cleartext through both loopback hops.
- Browser request-stream support remains non-portable and therefore cannot replace unary behavior.
- The installed Connect and ngrok dependencies already expose the required server and upstream
  protocol primitives; no new runtime dependency is assumed.
- BUG-010's bounded unary dispatch and authoritative-only shared presentation are the compatibility
  baseline for this feature.
- A browser tab can retain an ephemeral client instance identity for its lifetime without writing
  that identity into persistent session or player-configuration data.
- Preview audio remains authoritative; immediate transient feedback is visual-only.

## Verbatim Constraints

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

## Out of Scope

- Removing unary `SetPresentation` in this feature.
- Converting navigation, guesses, pattern activation, or character selection to streams.
- Adding a browser bidirectional RPC.
- Adding WebSocket or WebTransport protocols.
- Changing persistent session or player-configuration formats.
- Replacing the existing authoritative subscription downlink.
