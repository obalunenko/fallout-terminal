# Feature Specification: Protobuf-First ConnectRPC Migration

**Feature Branch**: `005-connectrpc-protobuf-migration`  
**Created**: 2026-08-13  
**Status**: Draft  
**Baseline**: `develop` at `8f306ce42e55b199ada4761d165461e2ebedc8ae`

**Bugfix**: 2026-08-13 — BUG-001 Clarified that all protobuf generation flows through the pinned Buf CLI and checked-in Buf v2 templates, without a standalone `protoc` requirement.
**Bugfix**: 2026-08-13 — BUG-002 Clarified that authenticated-ngrok acceptance requires a real public server-streaming player journey through the configured ngrok endpoint; local protected-forwarding simulation alone is insufficient.

## Clarifications

### Session 2026-08-13

- Q: Which validation and authority rules apply to character selection versus shared terminal mutations? → A: Every mutation receives common structural, recognition, live-session, subscription, request, broadcast, and action-semantic validation; an unassigned recognized session may select without controller, terminal, or generation authority, while navigation and hacking mutations require the eligible connected active controller and current applicable identities.
- Q: Does exactly one authoritative update guarantee receipt by every physical stream? → A: No; each commit creates one logical compound update offered once per affected logical session, while only active responsive physical streams must observe it exactly once and an unhealthy stream may observe zero before bounded termination.
- Q: How do absent, malformed, unknown, and expired recognition handles behave? → A: An absent Subscribe handle creates a session; malformed present handles return `invalid_argument`; well-formed unknown or expired Subscribe handles create replacement sessions; malformed unary handles return `invalid_argument`; and well-formed unknown or expired unary handles return typed `invalid-session` without creating state.
- Q: How must protobuf variants and scalar presence be represented? → A: Mutually exclusive variants use `oneof`; parallel optionals and string discriminators are prohibited; `optional` is reserved for meaningful scalar presence; removals reserve names and numbers; and enums retain `UNSPECIFIED` zero values unless a well-known imported type governs representation.
- Q: Which public requests are subject to the request-size boundary? → A: Every public player RPC has a 4 KiB maximum uncompressed protobuf request message enforced before adapters or canonical services, plus finite HTTP-body, decompression, and lower semantic bounds.

## User Scenarios & Testing

### User Story 1 - First-time player joins and selects a character (Priority: P1)

A first-time browser player opens the same-origin player page, receives one complete personalized authoritative snapshot through the generated subscription client, retains only the opaque recognition handle, sees the available roster, and selects one available character through a separately typed mutation.

**Why this priority**: This is the smallest complete player journey and proves that generated contracts, streaming state, recognition, and a unary mutation work together.

**Independent Test**: Start a fresh process with an active broadcast and available roster, open a clean browser profile, verify the first stream item is a complete snapshot, select a character, and verify the correlated result and authoritative assignment update.

**Acceptance Scenarios**:

1. **Given** a clean same-origin browser profile with no recognition handle, **When** the player subscribes, **Then** exactly one logical session is created and the first application message is exactly one complete personalized snapshot containing an accepted opaque handle, current revision, complete player state, and exactly one terminal-presentation variant.
2. **Given** a connected, recognized, currently unassigned logical session and an available character in the current broadcast, **When** the player submits a structurally valid selection with a new request identity, **Then** the server accepts the selection without requiring an existing assignment, controller authority, terminal identity, or hacking generation, returns a typed result, and offers the authoritative assignment update once to that logical session at the committed revision; the accepted selection may establish controller ownership under current domain rules.
3. **Given** a first-time player page, **When** the snapshot and selection complete, **Then** local storage contains only the opaque recognition handle and contains no assignment, role, phase, controller, broadcast, terminal, navigation, hacking, or pending-action state.
4. **Given** a connected, recognized, currently unassigned logical session, **When** it submits navigation, password-or-filler guess, or special-pattern activation before becoming the active controller, **Then** the server returns the applicable typed rejection and creates no mutation, revision, publication, replay entry for an accepted effect, attempt, or random-source advancement.
5. **Given** Subscribe requests whose recognition handle is respectively absent, present but blank, present but oversized, structurally malformed, or well-formed but unknown or expired, **When** each request is decoded within the public size boundary, **Then** absence creates a fresh session, invalid present values return Connect `invalid_argument` without creating a session, and well-formed unknown or expired values create a fresh session with a replacement opaque handle and complete current snapshot.

---

### User Story 2 - Returning player reconnects without resetting gameplay (Priority: P1)

A returning player reconnects during the same application process with the stored recognition handle and immediately receives the current personalized state, assignment, terminal, navigation, and hacking projection without regenerating or replaying the puzzle.

**Why this priority**: Session continuity and puzzle stability are required for ordinary network interruption and page refresh.

**Independent Test**: Disconnect and reconnect an assigned player while the same process retains an active puzzle; compare the reconnect snapshot with the canonical state and inspect puzzle-generation and audio-transition counters.

**Acceptance Scenarios**:

1. **Given** a currently recognized handle, **When** a new subscription presents it, **Then** the stream reattaches to the same logical session and begins with one current complete snapshot.
2. **Given** an active puzzle changed after the prior connection closed, **When** the player reconnects, **Then** the snapshot reflects the latest board, attempts, log, patterns, removed duds, outcome, assignment, and controller state without generating a puzzle.
3. **Given** a reconnect snapshot, **When** the client applies it, **Then** no ambient transition, hacking outcome cue, stale action, or previously accepted action is replayed.
4. **Given** a well-formed nonblank handle that is unknown or expired, **When** it is used for subscription, **Then** the first snapshot contains a replacement opaque handle, a fresh logical session, and complete current state without restoring expired assignment or controller authority unless current canonical rules independently establish it.

---

### User Story 3 - Multiple tabs share one logical session (Priority: P1)

Several tabs in one browser profile converge on one accepted handle and behave as physical views of one logical player while presence remains connected until that session's final stream closes.

**Why this priority**: Multi-tab recognition is current behavior and is central to correct presence, assignment, and controller authority.

**Independent Test**: Open concurrent clean tabs in one browser profile, observe handle convergence and stream counts, then close tabs one at a time and verify aggregate presence changes only after the final stream closes.

**Acceptance Scenarios**:

1. **Given** several concurrent first-time tabs sharing same-origin local storage, **When** they establish subscriptions, **Then** they converge on one accepted handle and one logical session rather than creating durable competing identities.
2. **Given** multiple active streams with one valid handle, **When** an authoritative revision affects the logical session, **Then** that session is offered one equivalent personalized compound update and every attached physical stream that remains active and responsive through delivery observes the revision exactly once.
3. **Given** multiple streams for one logical session, **When** all but one close, **Then** raw displayed stream count decreases while aggregate logical presence remains connected.
4. **Given** one remaining stream, **When** it closes, **Then** the logical session becomes disconnected without automatically releasing its character claim or controller identity.

---

### User Story 4 - Players converge through separately typed actions (Priority: P1)

Several players use separately typed character-selection, navigation, password-or-filler guess, and generation-bound special-pattern procedures while all views converge on one server-authoritative game state.

**Why this priority**: Complete gameplay parity requires every current mutation family and authoritative projection to cross the new public protocol.

**Independent Test**: Run four to seven streams through a mixed sequence of at least 25 selections, navigation actions, guesses, pattern activations, replays, rejections, and reconnects; compare each final personalized projection with canonical state.

**Acceptance Scenarios**:

1. **Given** a connected, recognized, currently unassigned logical session, **When** it selects an available character in the current broadcast, **Then** the selection is eligible for acceptance without existing assignment, controller authority, terminal identity, or hacking generation and may establish controller ownership under current domain rules.
2. **Given** an assigned, eligible, connected active controller, **When** it submits navigation, password-or-filler guess, or special-pattern activation with the applicable current identities, **Then** the action is validated and either rejected without state change or accepted as exactly one canonical mutation, one revision advance, and one logical compound update containing every public state component changed by the action.
3. **Given** clients with different roles or personalized roster views, **When** one revision is committed, **Then** each receives only its correct personalized compound projection and may skip irrelevant revisions.
4. **Given** a browser action is pending, **When** its unary result and applicable stream update arrive in either order, **Then** the pending state clears only after both conditions are satisfied for an accepted action and immediately for a rejected action.
5. **Given** one affected physical stream cancels, disconnects, or overflows before delivery while other streams remain responsive, **When** a state-changing action commits, **Then** canonical state, revision, and logical publication occur once, responsive streams observe the revision exactly once, and only the unhealthy stream may observe zero messages before termination.

---

### User Story 5 - Character selection and controller authority remain exact (Priority: P1)

A connected, recognized, currently unassigned logical session can select an available character without preexisting controller authority, while only the assigned, eligible, connected active controller can perform shared terminal mutations; observers, unassigned sessions, stale sessions, unknown sessions, and disconnected controllers cannot navigate or mutate hacking state.

**Why this priority**: Preserving server-side authority prevents divergent or unauthorized gameplay.

**Independent Test**: Select an available character from a new unassigned session, then exercise navigation and hacking families from active-controller, observer, unassigned, disconnected-controller, stale-broadcast, stale-terminal, stale-generation, unknown-session, and structurally invalid contexts and compare revision, state, publication, replay records, attempts, and randomness before and after.

**Acceptance Scenarios**:

1. **Given** a connected, recognized, currently unassigned logical session, **When** it selects an available character with current request and broadcast identities, **Then** the selection is evaluated under current character-availability and conflict rules without requiring controller, terminal, or hacking-generation authority.
2. **Given** an unknown, unavailable, conflicting, malformed, stale, or duplicate character-selection attempt, **When** it is handled, **Then** it is rejected through the applicable Connect error or typed result without changing shared state; an exact retry whose result remains retained instead replays only that original result under bounded replay rules, with no new mutation or publication.
3. **Given** an observer, unassigned session, non-controller, disconnected session, stale broadcast, stale terminal, or stale generation, **When** it sends navigation or hacking mutation, **Then** it receives the applicable typed rejection with no mutation, revision, publication, accepted-effect replay entry, attempt, or random-source advancement.
4. **Given** the controller's final stream has closed, **When** a navigation or hacking mutation is attempted with its recognized handle, **Then** the controller identity remains visible but the action is rejected as `controller-disconnected`.
5. **Given** a unary request with a missing, blank, oversized, or structurally malformed recognition handle, **When** it is handled, **Then** it returns Connect `invalid_argument` before canonical service invocation and creates no session, replacement handle, mutation, revision, publication, replay record, or random-source advancement.
6. **Given** a unary request with a well-formed nonblank recognition handle that is unknown or expired, **When** it is handled, **Then** it returns typed `invalid-session` and creates no logical session, replacement handle, mutation, revision, publication, replay record, or random-source advancement.

---

### User Story 6 - Retained duplicate requests replay safely (Priority: P1)

A player can retry a mutation after an uncertain network response and receive the original result while its request-result record remains in the bounded cache, without creating a second canonical effect.

**Why this priority**: Safe bounded replay is necessary when unary responses and stream updates can be interrupted independently.

**Independent Test**: Submit an accepted mutation, replay it while retained, reuse the identity with changed procedure and payload fingerprints, fill the cache beyond its bound, and verify the precise guarantees before and after eviction.

**Acceptance Scenarios**:

1. **Given** an exact request identity, procedure, and payload fingerprint remains retained, **When** it is retried, **Then** the original typed result and revision are returned with no second mutation or publication.
2. **Given** a retained request identity, **When** it is reused with a different procedure or payload fingerprint, **Then** the result is `duplicate` with no canonical effect.
3. **Given** a record has been evicted, cleared, removed with its logical session, or lost on restart, **When** that request identity is used again, **Then** it is evaluated as a new request against current state and may be accepted if still valid.
4. **Given** sustained requests, **When** the cache reaches its configured limit, **Then** deterministic eviction keeps it within the bound without promising a public eviction order or post-eviction replay.

---

### User Story 7 - Concurrent special-pattern activation consumes randomness once (Priority: P1)

Several requests race to activate one current unused special pattern, but only one request wins and only that accepted activation consumes the unchanged ordinary random sequence.

**Why this priority**: This protects both puzzle fairness and canonical consistency under concurrency.

**Independent Test**: Run at least 100 races against a deterministic random source and inspect accepted results, revisions, board effects, outcome draws, and dud-selection draws.

**Acceptance Scenarios**:

1. **Given** one unused current-generation pattern, **When** concurrent eligible activations race, **Then** exactly one mutation is accepted and committed.
2. **Given** one winning activation, **When** it applies, **Then** it consumes one outcome draw and at most one additional dud-selection draw under unchanged feature-003 rules.
3. **Given** stale, used, duplicate, rejected, or losing concurrent activations, **When** they are evaluated, **Then** they consume zero random calls and produce no mutation or revision.

---

### User Story 8 - Slow subscribers are isolated (Priority: P1)

A slow, blocked, overflowing, canceled, or disconnected player stream cannot delay canonical mutations, responsive players, effect publication, detachment, or application shutdown.

**Why this priority**: One unhealthy browser must not interrupt a live game or prevent owned resources from closing.

**Independent Test**: Deterministically block one subscriber, fill its bounded buffer, mutate canonical state, verify a responsive subscriber, and invoke shutdown under a fixed five-second deadline.

**Acceptance Scenarios**:

1. **Given** one blocked stream and one responsive stream, **When** an accepted mutation commits, **Then** the mutation completes and the responsive stream receives its update before the blocked stream is released.
2. **Given** a subscriber cancels, disconnects, or its buffer overflows before an offered revision is physically delivered, **When** publication continues, **Then** that stream may observe zero messages for the revision, only that stream is canceled or closed, no old incremental update is retried as acknowledged delivery, and the logical session stays connected if another stream remains.
3. **Given** the terminated stream reconnects, **When** it subscribes again, **Then** recovery begins with one complete current authoritative snapshot rather than replaying old incremental events.
4. **Given** shutdown begins with active, blocked, or canceled streams, **When** owned resources close, **Then** streams, presence, listener, tunnel resources, and workers are released idempotently within the bounded five-second deadline.

---

### User Story 9 - Game master retains the private desktop experience (Priority: P1)

The game master continues using every current named Wails operation and event with the same JavaScript-facing arguments, values, object shapes, and business outcomes while private protobuf contracts and exhaustive adapters govern their semantics behind the compatibility surface.

**Why this priority**: Player transport migration must not regress the trusted game-master application or accidentally publish private capabilities.

**Independent Test**: Exercise every inventoried method and event through the current desktop facade, compare inputs and outputs with baseline fixtures, and run adapter exhaustiveness and public-reachability checks.

**Acceptance Scenarios**:

1. **Given** the current game-master frontend, **When** any inventoried method or event is used, **Then** its name, argument behavior, result behavior, and native JavaScript shape remain unchanged.
2. **Given** a structured private request, result, status, or event, **When** it crosses the bridge, **Then** explicit typed adapters map compatibility values to private contract values and transport-independent application values without serialized protobuf carriers or generic maps.
3. **Given** a private schema field or variant is added or changed, **When** verification runs without a matching adapter update, **Then** verification fails.
4. **Given** crafted public service requests, **When** capabilities are enumerated and invoked, **Then** no path reaches trusted desktop operations, private hacking data, native paths, credentials, server information, or runtime status.

---

### User Story 10 - Existing files remain compatible (Priority: P1)

The game master loads and saves existing session version-1 and player-config version-1 files with their established field names, validation, unknown-field behavior, selected paths, ordering, and atomic replacement guarantees unchanged.

**Why this priority**: Campaign files and roster files are user-owned durable data and cannot be silently converted or made lossy by a transport migration.

**Independent Test**: Round-trip representative and adversarial version-1 fixtures through the existing codecs, including unknown session fields, strict player-config failures, normalized references, ordered saves, and injected atomic-write failures.

**Acceptance Scenarios**:

1. **Given** a valid session-v1 file with compatible unknown fields at supported levels, **When** it is opened and saved, **Then** all known fields and compatible unknown fields are preserved with established JSON names.
2. **Given** a player-config-v1 file with an unknown field, trailing value, unsupported version, invalid identity, or duplicate identity, **When** it is opened, **Then** the established strict validation rejects it without partial publication.
3. **Given** a roster or association change requiring a complete-file save, **When** atomic save fails, **Then** no roster, association, claim, controller, terminal, or puzzle change is published.
4. **Given** runtime player activity, **When** durable files are inspected, **Then** recognition, logical sessions, presence, assignments, controller, broadcasts, revisions, terminal runtime, navigation, hacking, pending actions, and replay caches remain absent.

---

### User Story 11 - Developer evolves contracts reproducibly (Priority: P2)

A developer updates versioned schemas first, regenerates type-safe Go and ECMAScript artifacts from the same schema revision, and receives immediate feedback for formatting, lint, generation drift, private dependency leakage, adapter omissions, or incompatible contract changes.

**Why this priority**: Reproducible contract governance makes future changes safe after parity is established.

**Independent Test**: Perform two clean generations, compare artifacts and repository state, then intentionally introduce representative incompatible edits, a private import into the public graph, and an unmapped private field.

**Acceptance Scenarios**:

1. **Given** an unchanged clean checkout, **When** generation runs twice, **Then** both Go and ECMAScript artifacts are identical and the second run leaves no diff.
2. **Given** a committed compatibility baseline, **When** a field number, field type, enum, package, or service is changed incompatibly, **Then** the contract check rejects it.
3. **Given** a public schema or generated player input, **When** its transitive dependencies are inspected, **Then** no private desktop, persistence, native-path, resolved-configuration, tunnel, credential, or private hacking contract is present.
4. **Given** generated files, **When** repository verification runs, **Then** manual edits and schema/code revision mismatches are detected.
5. **Given** a clean generation run, **When** Go and ECMAScript artifacts are produced, **Then** both generations execute through the pinned Buf CLI with `proto/buf.gen.go.yaml` and `proto/buf.gen.es.yaml`, no standalone Google `protoc` distribution is downloaded or invoked, and Buf-generated Go headers reporting `protoc unknown` are accepted.

---

### User Story 12 - Operator retains local, protected public, and packaged use (Priority: P2)

An operator starts the application with the same one-command development lifecycle and uses the same-origin player experience in local-network and authenticated ngrok modes, including streaming, every mutation, sounds, reconnect, offline packaged assets, and fail-closed Basic Auth.

**Why this priority**: The migration is complete only when production access, security, audio, lifecycle, and packaging remain usable.

**Independent Test**: Run the thin vertical proof and full journeys in local mode, protected ngrok mode, invalid-auth mode, and a clean packaged offline build.

**Acceptance Scenarios**:

1. **Given** local development, **When** `wails dev` starts the application, **Then** the player page, generated client, subscription, one mutation, sound discovery, and static assets work from the same origin.
2. **Given** protected ngrok mode with valid credentials, **When** an authenticated player connects, **Then** the same page and RPC procedures work through the configured public domain without wildcard CORS.
3. **Given** missing or invalid Basic Auth, **When** a public request reaches the tunnel, **Then** HTTP `401` is returned before player capabilities are reached.
4. **Given** a packaged application without external network availability, **When** it starts, **Then** all generated player code, fonts, images, scripts, sounds, and application assets load without a CDN, development server, or network-time package download.
5. **Given** oversized unary mutation, Subscribe, SoundManifest, compressed, and unknown-field-enlarged public requests, **When** each crosses the applicable uncompressed-message, encoded-body, or decompression limit, **Then** it returns Connect `resource_exhausted` at the transport or decoding boundary and a canonical-service spy records zero invocations and zero side effects.
6. **Given** an in-size SoundManifest request, **When** its category is absent, `UNSPECIFIED`, outside the existing catalog, path-like, or arbitrary filesystem input, **Then** it returns Connect `invalid_argument` before canonical service invocation and exposes no filesystem behavior.
7. **Given** ngrok is enabled on the configured public domain and valid Basic Auth credentials are supplied, **When** a clean browser opens the public player URL, **Then** its generated Connect `Subscribe` request receives one complete first snapshot, the `УСТАНОВКА СВЯЗИ...` overlay is hidden, the current terminal is rendered, and subsequent stream updates or reconnect snapshots continue through that same authenticated public endpoint.

---

### User Story 13 - Maintainer completes one-protocol cutover (Priority: P2)

A maintainer proves parity through a thin vertical slice and migration tests, then removes the legacy WebSocket protocol so the production application has one public player protocol and no permanent dual stack.

**Why this priority**: Removing the superseded path prevents security drift, duplicate semantics, and indefinite maintenance cost.

**Independent Test**: Scan source, dependency files, routes, built assets, fixtures, policy headers, and active documentation after all parity journeys pass.

**Acceptance Scenarios**:

1. **Given** bulk migration has not started, **When** the vertical proof is reviewed, **Then** it covers generated subscription and snapshot, one unary mutation, local same-origin mode, authenticated ngrok, Basic Auth failure, and bundled packaged assets.
2. **Given** temporary side-by-side transport exists on the migration branch, **When** parity acceptance is reached, **Then** the legacy path is removed before final acceptance.
3. **Given** final source and built artifacts, **When** cutover scans run, **Then** they find no active WebSocket route, upgrade, browser constructor, handwritten JSON player envelope, legacy fixture, direct legacy dependency, WebSocket policy allowance, or active documentation that treats the old protocol as authoritative.
4. **Given** historical completed feature documents remain, **When** they mention WebSocket contracts, **Then** those contracts are marked superseded and non-authoritative.

## Edge Cases

- Two clean tabs race before either has stored a recognition handle; both must converge without treating recognition as authentication.
- A Subscribe request omits its recognition handle, presents a blank, oversized, or structurally malformed handle, or presents a well-formed nonblank unknown or expired handle; only omission and the well-formed unknown-or-expired cases create fresh sessions, while invalid present values fail with `invalid_argument` and no session.
- A unary request has a missing, blank, oversized, structurally malformed, well-formed unknown, or expired recognition handle; structural cases fail with `invalid_argument`, while only well-formed unknown or expired values reach typed `invalid-session`, and neither path creates or replaces a session.
- A new unassigned session selects an available character and then attempts navigation or hacking before controller ownership is established; selection remains eligible but shared terminal actions are rejected without effects.
- A character selection is unavailable, unknown, conflicting, malformed, stale, or duplicated; it must not change shared state, and accepted selection alone may establish controller ownership under current domain rules.
- Any public request, including Subscribe or SoundManifest, exceeds the 4 KiB uncompressed message limit, encoded-body limit, or decompression limit; rejection occurs before application adapters or canonical services and creates no state or random effects.
- A compressed public request or a request enlarged only with unknown protobuf fields crosses a transport bound and must fail with `resource_exhausted` before canonical service invocation.
- A unary result arrives before its stream update, its stream update arrives before its result, the stream closes between them, or an irrelevant later revision is the next projection received.
- A snapshot is captured while a mutation commits; attachment must be gap-free without duplicate or missing relevant revisions.
- One revision changes player, terminal presentation, navigation, and hacking together, but affects different logical sessions differently.
- A retained request identity is replayed exactly, reused across procedures, reused with a changed payload, or retried after deterministic eviction.
- An action is decoded correctly but becomes stale between validation stages because broadcast, controller, assignment, terminal, puzzle generation, or pattern availability changed.
- Many contenders activate the same pattern while a terminal switch, failed-hack reset, disconnect, or broadcast end is ordered concurrently.
- One stream for a multi-tab session overflows while sibling streams remain healthy; the affected stream may miss the revision and terminates within the bounded lifecycle, sibling streams receive it exactly once, and aggregate presence remains connected.
- A stream is canceled or the listener shuts down while its outbound buffer is full or a send is blocked.
- A valid sound category has a missing, unreadable, empty, mixed-extension, or case-varied folder; discovery remains safe, deterministic, and non-blocking.
- A player loses connectivity after an accepted mutation commits but before receiving its result, its update, or both, then reconnects to a complete later snapshot.
- The application restarts while browsers retain old handles; no runtime identity or state may be reconstructed from those handles.
- `develop` advances after this specification; compatible newer behavior must be inventoried and preserved rather than reverted to the recorded baseline.
- A session-v1 file contains compatible unknown fields recursively, while a player-config-v1 file contains the same kind of unknown field and must still be rejected under its strict codec.
- An added private contract field has no Wails mapping, or a public schema accidentally imports a private package; verification must fail before release.
- Credentials, native paths, secret words, private candidates, future random outcomes, internal errors, or raw request material appear in an error or generated public descriptor; security inspection must reject the build.

## Requirements

### Functional Requirements

#### Baseline and contract governance

- **FR-001**: The feature MUST preserve the Go 1.26 modular-monolith architecture on Wails v2.13.0 and all behavior compatible with `develop` commit `8f306ce42e55b199ada4761d165461e2ebedc8ae`, including feature-003 hacking behavior and feature-004 player-session and control behavior.
- **FR-002**: If `develop` advances before implementation, the implementation MUST inventory and preserve compatible newer behavior rather than revert to the recorded baseline.
- **FR-003**: Before public transport implementation begins, the project MUST inventory every application-owned public player contract, private desktop payload, known persistence field, and serializable application configuration field.
- **FR-004**: Every inventory item MUST be classified as represented by a versioned protobuf contract, governed by a third-party schema, or excluded as a non-serializable implementation dependency.
- **FR-005**: The completed inventory MUST contain zero unclassified application-owned structured boundaries and zero unclassified serializable configuration fields.
- **FR-006**: Versioned public schemas MUST govern recognition exchange, personalized player state, roles, phases, assignments, controller state, roster availability, terminal content and presentation, navigation, public hacking patterns, typed mutation requests and results, complete snapshots, compound updates, and sound manifests.
- **FR-007**: Versioned private schemas MUST govern known session-v1, terminal, recursive content-node, player-config-v1, roster, Wails request, Wails result, status, event, session-operation, player-config-operation, command, coordination, terminal-switch, server-information, runtime-status, and serializable configuration semantics.
- **FR-008**: Serializable configuration contracts MUST cover application, player-server, coordination, queue, timeout, path, tunnel, startup, and shutdown values that can cross or be recorded at application-owned boundaries.
- **FR-009**: Embedded filesystems, callbacks, event sinks, clocks, random sources, service interfaces, process handles, listeners, and other dependency-injection objects MUST remain excluded from serializable configuration contracts.
- **FR-010**: Third-party schemas including `wails.json`, `package.json`, Buf configuration, GitHub Actions workflows, plist files, and third-party traffic-policy formats MUST NOT be duplicated in application protobuf schemas.
- **FR-011**: Public and private schemas MUST use separate versioned packages and MUST preserve an inspectable one-way boundary in which public schemas never import private schemas.
- **FR-012**: Generated player-client inputs and their transitive bundle MUST contain no private desktop, persistence, native-path, resolved-configuration, tunnel, credential, or private hacking contract.
- **FR-013**: Protobuf schema existence MUST NOT authorize public exposure of any message or capability.
- **FR-014**: Generated protobuf values MUST remain detached boundary values and MUST NOT become canonical mutable domain, navigation, live, coordination, or hacking aggregates.
- **FR-015**: Public transport MUST contain no handwritten JSON envelopes, generic maps, generic command procedure, generic RPC dispatcher, duplicated public transport DTO, or manually maintained RPC router.

#### Public player service and browser portability

- **FR-016**: The public player service MUST expose exactly one server-streaming subscription responsibility and separate unary responsibilities for character selection, navigation action, password-or-filler guess, special-pattern activation, and sound-manifest discovery.
- **FR-017**: The public service MUST NOT merge player mutations into a generic command procedure.
- **FR-018**: Browser-originated mutations MUST use unary procedures and authoritative state delivery MUST use server streaming.
- **FR-019**: The browser player MUST NOT require client-streaming or bidirectional-streaming request bodies.
- **FR-020**: The player page, generated player procedures, static resources, and sound assets MUST remain same-origin in local and ngrok modes without wildcard CORS.
- **FR-021**: The public listener MUST NOT expose runtime status, server information, health, reflection, generic capability discovery, diagnostics, tunnel status, or client-count procedures.
- **FR-022**: The public listener MUST NOT expose `ForceHackSuccess`, `ResetFailedHack`, native dialogs, arbitrary file operations, URL opening, tunnel configuration, private candidates, secret words, future outcomes, credentials, raw connection identities, or another session's private information.
- **FR-023**: The browser Connect transport MAY use generated protobuf JSON or binary encoding, but planning MUST choose and pin one encoding using official ConnectRPC guidance.

#### Recognition, logical sessions, and presence

- **FR-024**: Recognition handles MUST remain opaque to clients, and clients MUST NOT parse them or infer session identity, age, validity, assignment, presence, controller authority, or any lifecycle state from their contents.
- **FR-025**: The Subscribe request MUST use explicit protobuf presence for its recognition-handle scalar so absence is distinguishable from a present empty value.
- **FR-026**: A Subscribe request with the recognition handle absent MUST create exactly one process-local logical session and return its opaque handle in the initial complete snapshot.
- **FR-027**: A Subscribe request with a present blank, oversized, or structurally malformed recognition handle MUST return Connect `invalid_argument` without creating a logical session or replacement handle.
- **FR-028**: A Subscribe request with a currently recognized handle MUST reattach to the same logical session.
- **FR-029**: A Subscribe request with a well-formed nonblank handle that is unknown or expired MUST create a fresh logical session, issue a replacement handle, and return a complete current snapshot without restoring expired assignment or controller authority unless existing canonical rules independently establish it.
- **FR-030**: A unary request with a missing, blank, oversized, or structurally malformed recognition handle MUST return Connect `invalid_argument`.
- **FR-031**: A unary request with a well-formed nonblank handle that is unknown or expired MUST return typed `invalid-session` without creating a logical session or replacement handle and without mutation, revision advancement, random-source advancement, publication, or a replay result for the nonexistent session.
- **FR-032**: Recognition-handle validation MUST impose a finite documented semantic bound while preserving handle opacity.
- **FR-033**: Concurrent first-time tabs in one browser profile MUST converge on one accepted recognition handle and one logical session.
- **FR-034**: Every active stream using one valid handle MUST attach to the same logical session and receive equivalent personalized state.
- **FR-035**: Raw displayed browser connection count MUST equal active public server streams.
- **FR-036**: Aggregate logical-session presence MUST remain connected while at least one stream for that session remains active.
- **FR-037**: Closing a logical session's final stream MUST mark it disconnected without automatically releasing its character claim or controller identity; reconnecting within the same process MUST restore current presence and authoritative assignment, while process restart MUST restore no recognition mapping, logical session, claim, controller, broadcast, revision, live terminal, navigation, hacking, pending action, or replay-cache state.

#### Snapshot, update, revision, and pending-action semantics

- **FR-038**: The first application message on every new or reconnecting subscription MUST be exactly one complete personalized compound snapshot.
- **FR-039**: Every complete snapshot MUST contain the accepted or replacement recognition handle, complete current personalized player state, current authoritative revision, and exactly one terminal-presentation variant.
- **FR-040**: The snapshot terminal-presentation variant MUST be either the complete current public live-terminal projection or an explicit no-live-terminal state.
- **FR-041**: Snapshot creation MUST never generate or regenerate a hacking puzzle.
- **FR-042**: Reconnect snapshots MUST NOT replay ambient transitions, hacking outcome cues, stale actions, or prior accepted actions.
- **FR-043**: Stream attachment and snapshot capture MUST form a gap-free boundary such that a snapshot at revision R is followed by no application message at revision R or lower.
- **FR-044**: Every relevant committed revision after a subscriber's snapshot MUST be offered once to its affected logical session; every attached physical stream that remains active and responsive through delivery MUST observe that revision exactly once, while a canceled, disconnected, or overflowing stream MAY observe zero messages for it and MUST terminate so recovery can use a new complete snapshot.
- **FR-045**: One accepted mutation MUST commit exactly one coordinator revision.
- **FR-046**: Each affected logical session MUST be offered exactly once the accepted mutation's single logical compound authoritative update, personalized to contain every public state component changed for that session by the action.
- **FR-047**: No physical stream MUST observe the same revision more than once, and every physical stream attached to an affected logical session that remains active and responsive through delivery MUST observe the session's equivalent personalized update exactly once.
- **FR-048**: The compound-update contract MUST permit several complete changed player, terminal-presentation, navigation, and hacking projections in one value.
- **FR-049**: An omitted compound-update component MUST mean unchanged and MUST NOT act as an ambiguous partial patch.
- **FR-050**: A subscriber MUST NOT receive several player, terminal, navigation, or hacking application messages with the same revision.
- **FR-051**: Revisions observed by one subscriber after its snapshot MUST be strictly increasing while revisions irrelevant to that personalized projection MAY be skipped.
- **FR-052**: Publication MUST permit one committed revision to produce different personalized compound updates for different logical sessions.
- **FR-053**: Clients MUST NOT optimistically mutate assignment, role, active terminal, navigation, attempts, logs, patterns, removed duds, puzzle result, or any other canonical shared state.
- **FR-054**: An accepted action MUST remain pending until the client has both its correlated unary result and an applicable authoritative stream state at the result revision or a later relevant revision.
- **FR-055**: A rejected action MUST clear its correlated pending state immediately.

#### Mutation, replay, authority, and randomness

- **FR-056**: Every player mutation MUST validate request structure and finite bounds, recognition handle, live logical session with the required active subscription relationship, request identity, broadcast identity, and action-specific semantic fields before domain mutation.
- **FR-057**: Character selection MUST be allowed only for a connected, recognized, currently unassigned logical session; MUST NOT require an existing assignment, controller authority, terminal identity, or hacking generation; MUST reject unavailable, unknown, conflicting, malformed, stale, or duplicate selection attempts without shared-state mutation, except that an exact request whose result remains retained replays only that result without a new effect under FR-062; and MUST preserve the current domain behavior by which acceptance can establish controller ownership.
- **FR-058**: Navigation actions, password-or-filler guesses, and special-pattern activation MUST require an assigned, eligible, connected active controller, the applicable current terminal or broadcast identity, and the current hacking generation when generation-bound; requests from unassigned sessions, observers, non-controllers, disconnected sessions, stale broadcasts, stale terminals, or stale generations MUST be rejected without mutation or random-source advancement.
- **FR-059**: A rejected, stale, duplicate, structurally invalid, observer-originated, disconnected-controller, or non-actionable request MUST NOT mutate canonical state, increment revision, publish an update, consume an attempt, or consume hacking randomness.
- **FR-060**: Replay storage MUST default to 256 request-result records per logical session and broadcast.
- **FR-061**: Exact replay MUST be guaranteed only while the matching request-result record remains retained.
- **FR-062**: An identical retained request MUST return its original result and revision without another canonical effect.
- **FR-063**: A retained request identity reused with a different procedure or payload fingerprint MUST return `duplicate` without mutation.
- **FR-064**: Replay-cache eviction MUST be deterministic but MUST NOT be presented as a public protocol guarantee.
- **FR-065**: After eviction, cache clearing, logical-session removal, or process restart, a reused request identity MUST be evaluated as new against current state and MAY be accepted if still valid.
- **FR-066**: The feature MUST NOT claim broadcast-lifetime or process-lifetime exactly-once behavior after replay-record loss.
- **FR-067**: Concurrent activation of one unused current-generation special pattern MUST produce exactly one accepted mutation.
- **FR-068**: Randomness consumption MUST be limited to the winning accepted special-pattern activation.
- **FR-069**: An accepted special-pattern activation MUST consume one outcome draw and at most one additional dud-selection draw under unchanged feature-003 rules.
- **FR-070**: Every stale, used, duplicate, rejected, or losing concurrent special-pattern activation MUST consume zero random calls.
- **FR-071**: The feature MUST preserve current pattern probabilities, dud-removal rules, attempts-reset rules, board-generation rules, and opaque generation-bound `patternId` semantics.
- **FR-072**: `HACK_ADMIN` MUST remain removed from every public contract, handler, generated client, fixture, and active document.
- **FR-144**: Every accepted state-changing action MUST mutate canonical state exactly once, advance the authoritative revision exactly once, produce exactly one logical compound update for the committed revision, and offer it once to each affected logical session before completing the unary response.

#### Error classification and public safety

- **FR-073**: Malformed bounded protobuf messages, illegal or missing variants, prohibited required `UNSPECIFIED` enums, a present invalid Subscribe recognition handle, a missing or invalid unary recognition handle, blank or structurally invalid request identity, missing required broadcast or terminal identity, invalid sound category, and structurally invalid action targets MUST return Connect `invalid_argument` before canonical service invocation.
- **FR-074**: Every public player RPC, including Subscribe and SoundManifest, MUST enforce a maximum 4 KiB uncompressed protobuf request-message size at the Connect transport or decoding boundary before application adapters or canonical services are invoked; exceeding that limit MUST return Connect `resource_exhausted`.
- **FR-075**: Unsupported public services or procedures MUST return Connect `unimplemented`.
- **FR-076**: Request or stream cancellation MUST return Connect `canceled`.
- **FR-077**: Temporary public service unavailability MUST return Connect `unavailable`.
- **FR-078**: Unexpected failures outside authoritative domain rejection MUST return Connect `internal`.
- **FR-079**: Decoded structurally valid player mutations rejected by current state or domain rules MUST return a typed authoritative action result.
- **FR-080**: Typed action results MUST use only `accepted`, `invalid-session`, `stale-broadcast`, `unassigned`, `not-controller`, `controller-disconnected`, `stale-terminal`, `invalid-action`, `conflict`, or `duplicate` as stable reasons.
- **FR-081**: Stale-generation, used-pattern, absent-puzzle, non-actionable-puzzle, solved-puzzle, failed-puzzle, exhausted-puzzle, and well-formed invalid-target rejections MUST use `invalid-action` unless `stale-terminal` is more specific.
- **FR-082**: Invalid Basic Auth MUST fail closed with HTTP `401` before player capabilities are reached.
- **FR-083**: Public errors MAY include safe correction or retry guidance but MUST NOT expose request bytes, legacy JSON, recognition handles, credentials, native paths, stack traces, private identities, private candidates, secret words, future outcomes, tunnel policy details, or internal dependency errors.
- **FR-145**: Planning MUST record and tests MUST enforce a finite encoded HTTP-body limit and decompression limit consistent with Connect framing overhead, including compressed requests and messages enlarged with unknown protobuf fields.
- **FR-146**: Recognition handles, request identities, broadcast identities, terminal and generation identifiers, pattern identifiers, navigation targets, guesses, and sound categories MUST each have finite semantic limits below the public request-message maximum.
- **FR-147**: Sound categories MUST be validated against the existing allowed catalog and MUST NOT accept paths, path fragments, or arbitrary filesystem input.
- **FR-148**: Rejection at any public message-size, encoded-body, decompression, structural, finite field-bound, or sound-category boundary check MUST create no logical session, mutation, revision, logical publication, replay entry, or random-source advancement and MUST occur before canonical service invocation.

#### Private Wails semantic contracts and adapters

- **FR-084**: Wails MUST remain the trusted private desktop transport and MUST preserve its native structured-object transport.
- **FR-085**: Every structured Wails request, result, status, and event MUST have a versioned private protobuf semantic contract.
- **FR-086**: Explicit typed adapters MUST exhaustively map existing Wails-facing compatibility values, generated private protobuf values, and transport-independent application or domain values.
- **FR-087**: Wails-facing compatibility DTOs MUST remain narrow private adapter projections and MUST NOT become reusable public transport models or an independent source of business semantics.
- **FR-088**: Adding or changing a private protobuf field or variant without updating its Wails adapter MUST fail verification.
- **FR-089**: Wails MAY retain framework-native internal JSON marshalling but MUST NOT carry protobuf binary, Base64, ProtoJSON, serialized protobuf envelopes, or generic maps.
- **FR-090**: Existing named Wails methods and events MUST NOT be replaced by a generic protobuf or RPC dispatcher.
- **FR-091**: Private business and validation outcomes MUST remain explicit structured result objects rather than ad hoc error strings.
- **FR-092**: All private desktop capabilities for sessions, player configurations, roster and logical-session coordination, broadcast control, terminal switching, failed-hack reset, `ForceHackSuccess`, URL opening, runtime status, server information, and desktop events MUST remain available under their current eligibility rules.
- **FR-093**: Every frontend-facing private method and event listed in the Verbatim Constraints section MUST preserve its exact name, argument behavior, result behavior, and native JavaScript value or object shape.

#### Persistence and configuration compatibility

- **FR-094**: Known `Session`, `Terminal`, recursive `ContentNode`, `PlayerConfig`, and roster fields MUST have protobuf definitions without replacing their established persistence codecs.
- **FR-095**: Persisted sessions MUST remain portable JSON version-1 documents and persisted player configurations MUST remain separate strict JSON version-1 documents.
- **FR-096**: Neither governed persistence format MUST be replaced by protobuf binary or generic ProtoJSON.
- **FR-097**: Session and player-config JSON field names MUST remain exactly compatible with their current version-1 documents.
- **FR-098**: Session-compatible unknown fields MUST be preserved at every currently supported level.
- **FR-099**: Current persistence validation limits, defaults, selected-path behavior, normalized relative player-config references, ordered save revisions, and atomic-save behavior MUST remain unchanged.
- **FR-100**: Player-config decoding MUST continue to reject unknown fields, trailing data, unsupported versions, invalid identities, duplicate identities, and all other current validation failures.
- **FR-101**: Runtime recognition, assignments, controller state, broadcasts, revisions, live-terminal runtime, navigation, hacking, pending actions, and replay caches MUST remain absent from durable session and player-config data.
- **FR-102**: A roster or association change MUST NOT be published before its required complete-file atomic save succeeds.
- **FR-103**: Environment variables and command-line arguments MUST retain their current precedence and validation.
- **FR-104**: The player listener MUST retain default port 3690, delivery queue default 32, and current startup-timeout behavior.
- **FR-105**: Protected public access MUST retain a provider-assigned public endpoint when no fixed domain is configured, credential-pair validation, and fail-closed Basic Auth.
- **FR-106**: Tunnel credentials MUST remain process-local and redacted.
- **FR-107**: Credentials MUST be absent from public schemas, runtime status, desktop events, logs, public errors, session files, player-config files, and generated public documentation.
- **FR-108**: Private protobuf configuration contracts MUST NOT authorize configuration exposure through public procedures.

#### Sound and static-resource compatibility

- **FR-109**: The structured sound-list endpoint MUST be replaced by the typed unary sound-manifest responsibility.
- **FR-110**: Sound discovery MUST preserve all current categories and allowlisted extensions listed in Verbatim Constraints.
- **FR-111**: A successful sound manifest MUST return the requested allowlisted category and deterministically sorted allowlisted filenames or safe relative asset paths.
- **FR-112**: A sound manifest MUST NOT return absolute origins, native filesystem paths, embedded filesystem capabilities, or arbitrary caller-supplied paths.
- **FR-113**: The browser MUST resolve sound asset URLs against the current same origin.
- **FR-114**: A valid category with a missing, unreadable, or empty folder MUST return an empty successful manifest.
- **FR-115**: An absent or `UNSPECIFIED` sound category MUST return `invalid_argument`.
- **FR-116**: Sound discovery and prefetch MUST remain asynchronous, optional, and non-blocking.
- **FR-117**: Ambient and hacking cues MUST continue to follow newly applied authoritative transitions.
- **FR-118**: Reconnect snapshots, stale updates, rejected actions, exact replays, and rerenders MUST NOT replay authoritative outcome cues.
- **FR-119**: Static HTML, CSS, fonts, images, generated scripts, and sound files MUST remain ordinary embedded same-origin HTTP resources.
- **FR-120**: Packaged operation MUST remain fully offline without a CDN or external development server.

#### Streaming lifecycle, migration, and cutover

- **FR-121**: Every subscriber MUST have bounded buffering.
- **FR-122**: A slow, blocked, overflowing, canceled, or disconnected subscriber MUST NOT block canonical mutation, another subscriber, effect publication, detachment, or shutdown.
- **FR-123**: Subscriber cancellation, disconnection, or buffer overflow MUST cancel or close only the affected physical stream within bounded time, release its resources, and MUST NOT block canonical mutation, other logical sessions, or sibling physical streams.
- **FR-149**: The server MUST NOT retry old incremental events as though physical delivery had been acknowledged; a terminated stream MUST recover only by reconnecting to a complete current authoritative snapshot.
- **FR-124**: Shutdown MUST cancel streams, release subscribers and logical presence, close the listener, stop tunnel-owned resources, wait for owned workers, and remain idempotent.
- **FR-125**: The browser MUST retain the current three-second reconnect delay unless planning documents an equivalent or safer bounded policy.
- **FR-126**: Before bulk migration, planning MUST require a thin vertical proof covering generated subscription and snapshot, at least one unary mutation, same-origin local mode, authenticated ngrok mode, Basic Auth failure, and bundled packaged assets.
- **FR-150**: Acceptance of authenticated ngrok mode MUST include a clean-browser, generated-code `Subscribe` journey through the actual configured public ngrok endpoint that proves first-snapshot delivery, connection-overlay dismissal, terminal rendering, and continued streaming or reconnect behavior. Local or synthetic protected-forwarding evidence MUST NOT by itself satisfy authenticated-ngrok streaming acceptance.
- **FR-127**: Any temporary side-by-side player transport MUST be limited to the migration branch and the period while parity tests are constructed.
- **FR-128**: Final acceptance MUST contain exactly one application-owned public player protocol.
- **FR-129**: Final cutover MUST remove active WebSocket routes and upgrades, browser `WebSocket` construction, legacy JSON request decoders and envelopes, legacy protocol fixtures, the direct `github.com/coder/websocket` dependency, WebSocket CSP allowances, and active documentation describing the superseded protocol.
- **FR-130**: Any retained historical completed feature document MUST mark legacy WebSocket contracts as superseded and non-authoritative.

#### Generated code and contract evolution

- **FR-131**: The same public schemas MUST generate type-safe Go public handlers and values plus ECMAScript player values and clients.
- **FR-132**: Private schemas MUST generate the types required by explicit Wails and persistence adapters without entering the player bundle.
- **FR-133**: ~~The Buf CLI, protobuf compiler and generators, ConnectRPC generator, Go runtime packages, ECMAScript runtime packages, and package-manager dependencies MUST be pinned.~~ **Superseded by BUG-001** because the wording incorrectly implied a separate Google `protoc` installation. The Buf CLI, protobuf and ConnectRPC code-generation plugins, Go runtime packages, ECMAScript runtime packages, and package-manager dependencies MUST be pinned; the pinned Buf CLI owns the protobuf compiler implementation used by generation.
- **FR-134**: Generated files MUST be deterministic, reproducible, handled according to repository policy, and never manually edited. Every checked-in Go and ECMAScript generated output MUST be produced through the pinned Buf CLI with the applicable checked-in Buf v2 template.
- **FR-135**: The packaged application MUST contain all required generated and bundled player code.
- **FR-136**: Clean development startup MUST remain one root `wails dev` command.
- **FR-137**: Contract verification MUST cover formatting, lint, deterministic zero-diff generation, one schema revision for Go and ECMAScript outputs, breaking-change detection, public/private dependency inspection, and exhaustive private adapter mapping. Every generation pass MUST run through the pinned Buf CLI with the checked-in `proto/buf.gen.go.yaml` and `proto/buf.gen.es.yaml` templates; verification MUST NOT require a standalone `protoc` version and MUST accept Buf-generated `protoc unknown` header provenance while still validating generated markers, plugin pins, schema revision, hashes, and output isolation.
- **FR-138**: Breaking-change checks MUST reject incompatible field-number, field-type, enum, package, and service changes against the committed baseline once it exists.
- **FR-139**: Compatible unknown protobuf fields MUST follow standard forward-compatible protobuf behavior while known request fields remain authoritatively validated.
- **FR-140**: Removed protobuf fields, mutually exclusive variants, and enum values MUST reserve their former names and numbers as applicable.
- **FR-141**: Every protobuf enum MUST define an `UNSPECIFIED` zero value unless the schema imports a well-known type whose governing rules already define its representation.
- **FR-142**: Protobuf `optional` MUST be reserved for scalar presence when absence is semantically different from the scalar default.
- **FR-143**: Mutually exclusive protobuf payload variants MUST use `oneof`; parallel optional variant fields and string discriminator fields are prohibited.

### Existing Private Wails Inventory

The following current bridge operations are compatibility requirements, not proposed public RPC procedures. Names are recorded verbatim from `frontend/src/desktop-api.js` and the bound `App` methods at the baseline.

| Go method | Current frontend facade | Argument behavior | Result behavior |
|---|---|---|---|
| `GetRuntimeStatus` | internal adapter initialization | none | Native runtime-status object containing server info, stream count, public hack state, startup/save status, revisions, and private coordination state |
| `NewSession` | `newSession()` | none; native destination dialog | Structured session result |
| `OpenSession` | `openSession()` | none; native source dialog | Structured session result |
| `CopyDemo` | no current facade method | none; native destination flow | Structured session result; exported session operation remains private |
| `SaveSession` | `saveSession(session)` | complete session-v1 compatibility object | Structured ordered-save result |
| `LoadReferencedPlayerConfig` | `loadReferencedPlayerConfig()` | none | Structured player-config operation result |
| `NewPlayerConfig` | `newPlayerConfig()` | none; native destination dialog | Structured player-config operation result |
| `OpenPlayerConfig` | `openPlayerConfig()` | none; native source dialog | Structured player-config operation result |
| `RequestTerminalActivation` | `requestTerminalActivation(payload)` | terminal identity, name, tree, hacking level, and intro text | Structured terminal-switch result |
| `UpdateLiveTerminal` | `updateLiveTerminal(payload)` | content tree and optional intro text | Structured coordination result |
| `RequestTerminalClear` | `requestTerminalClear()` | none | Structured terminal-switch result |
| `ResolveTerminalSwitch` | `resolveTerminalSwitch(payload)` | opaque switch identity and allowlisted decision | Structured terminal-switch result |
| `ForceHackSuccess` | `forceHackSuccess()` | none | Structured command result |
| `ResetFailedHack` | `resetFailedHack(payload)` | current terminal target payload | Structured coordination result |
| `AddCharacter` | `addCharacter(name)` | character display name string | Structured coordination result |
| `RenameCharacter` | `renameCharacter(payload)` | character identity and display name | Structured coordination result |
| `DeleteCharacter` | `deleteCharacter(characterId)` | character identity string | Structured coordination result |
| `RenameLogicalSession` | `renameLogicalSession(payload)` | logical-session identity and fallback name | Structured coordination result |
| `AssignCharacter` | `assignCharacter(payload)` | logical-session and character identities | Structured coordination result |
| `ReleaseCharacter` | `releaseCharacter(sessionId)` | logical-session identity string | Structured coordination result |
| `MoveCharacter` | `moveCharacter(payload)` | character identity and destination session identity | Structured coordination result |
| `SetActiveController` | `setActiveController(sessionId)` | logical-session identity string | Structured coordination result |
| `StartBroadcast` | `startBroadcast()` | none | Structured coordination result |
| `EndBroadcast` | `endBroadcast()` | none | Structured coordination result |
| `OpenURL` | `openUrl(url)` | final HTTP(S) URL string, validated again in Go | Structured command result |

The current desktop runtime events are:

| Event name | Current facade subscription | Native payload behavior |
|---|---|---|
| `server-info` | `onServerInfo(callback)` | Server-information object with safe local/public status; subscription returns an unsubscribe function |
| `client-count` | `onClientCount(callback)` | Raw active-stream count integer; subscription returns an unsubscribe function |
| `hack-state` | `onHackState(callback)` | Public hacking projection or null; subscription returns an unsubscribe function |
| `coordination-state` | `onCoordinationState(callback)` | Detached private game-master coordination projection; subscription returns an unsubscribe function |

Wails lifecycle callbacks remain private application lifecycle boundaries and are not public player capabilities. The exported application lifecycle service methods `Start` and `Shutdown` are not generic desktop dispatch surfaces and retain their existing lifecycle semantics.

### Non-Goals

- Adding player roles, controller rules, assignment rules, or remembered-selection behavior beyond feature 004.
- Adding accounts, login-based identity, cross-device identity, or durable logical-session restoration.
- Persisting live, navigation, hacking, assignment, controller, recognition, pending-action, or replay-cache state across restart.
- Promising broadcast-lifetime or process-lifetime exactly-once behavior after replay-cache eviction or loss.
- Introducing session version 2 or player-config version 2.
- Replacing session or player-config JSON with protobuf binary or generic ProtoJSON.
- Redesigning gameplay, navigation, hacking, special patterns, character selection, controller behavior, or sound behavior.
- Creating a separately deployed backend or microservice.
- Exposing the private Wails bridge through the player listener.
- Serializing protobuf binary, Base64, or ProtoJSON through Wails.
- Replacing named Wails operations with a generic dispatcher.
- Adding public health, runtime-status, server-information, reflection, capability-discovery, or diagnostic services.
- Supporting old external WebSocket clients after final cutover.
- Introducing wildcard CORS, a CDN, or a required player development server.
- Changing the public ngrok domain, default player port, credential rules, release signing, notarization, or supported packaging scope.

## Key Entities

- **Contract Inventory Item**: One application-owned structured boundary or serializable configuration field, with an owner, producer, consumers, classification, schema location or exclusion rationale, and public-or-private exposure designation.
- **Recognition Handle**: An opaque, finitely bounded browser-stored value that maps to one process-local logical session and carries no authentication or authorization power; clients never parse it or infer lifecycle state from it.
- **Logical Session**: The process-local identity shared by all active streams using one accepted handle; it owns aggregate presence, fallback name, current assignment reference, and bounded request replay records.
- **Physical Stream**: One active server-streaming subscription for a browser tab; several streams may belong to one logical session and together determine aggregate presence.
- **Personalized Snapshot**: The mandatory first stream value containing the accepted handle, complete player state, one revision, and exactly one complete terminal-presentation variant.
- **Compound Authoritative Update**: The single revisioned logical publication for one committed action, personalized when offered once to each affected session so it contains every complete public projection changed for that session while omitted components mean unchanged; physical receipt depends on a stream remaining active and responsive through delivery.
- **Player Mutation**: One separately typed unary request with recognition, request, broadcast, and procedure-specific semantic data; character selection requires no controller, terminal, or generation authority, while shared terminal mutations carry applicable terminal and generation identity and require the connected active controller.
- **Action Result**: A correlated authoritative mutation outcome containing acceptance, stable reason, and the authoritative revision relevant to pending-action reconciliation.
- **Request Replay Record**: A bounded per-logical-session, per-broadcast association of request identity, procedure-and-payload fingerprint, original result, and revision.
- **Terminal Presentation**: The exclusive public variant representing either a complete live-terminal projection or explicit no-live-terminal state.
- **Public Hacking Projection**: Player-safe puzzle state containing visible board, attempts, log, outcome, and opaque generation-bound patterns without secret candidates, secret word, or future outcomes.
- **Sound Manifest**: A typed result for one allowlisted category containing deterministically ordered safe relative asset names.
- **Private Desktop Contract**: A versioned semantic definition for one Wails request, result, status, event, or private configuration value, mapped exhaustively to the unchanged compatibility shape.
- **Persistence Contract**: Protobuf-defined known semantics for session-v1 or player-config-v1 data whose established JSON codec remains authoritative for file representation and compatibility behavior.
- **Serializable Runtime Configuration**: Application-owned data values controlling startup, listener, coordination, queue, timeout, path, tunnel, and shutdown behavior, excluding injected dependencies and third-party schema ownership.

## Success Criteria

### Measurable Outcomes

- **SC-001**: The completed contract inventory reports zero unclassified application-owned DTOs and zero unclassified serializable configuration fields.
- **SC-002**: Formatting and lint checks executed by the pinned Buf CLI complete with zero findings.
- **SC-003**: Two consecutive clean generations through the pinned Buf CLI and checked-in Buf v2 templates produce byte-identical Go and ECMAScript artifacts and leave zero repository diff after the second generation, with no standalone `protoc` download or invocation.
- **SC-004**: Once a compatibility baseline exists, representative incompatible field-number, field-type, enum, package, and service edits are each rejected by automated breaking-change checks.
- **SC-005**: Generated public Go and ECMAScript outputs identify the same source schema revision in every clean generation; Buf-generated Go headers reporting `protoc unknown` are valid provenance when generated markers, plugin pins, schema revision, and deterministic hashes pass verification.
- **SC-006**: Every new or reconnecting stream in automated journeys begins with exactly one complete personalized snapshot and no earlier application value.
- **SC-007**: Snapshot creation tests and 100 reconnect trials record zero additional puzzle generations.
- **SC-008**: Concurrent first-time tabs in one browser profile converge on one accepted handle and one logical session in every trial.
- **SC-009**: In multi-tab disconnect tests, raw stream count decreases per closed stream while aggregate logical presence changes to disconnected only when the final stream closes.
- **SC-010**: Across at least 100 concurrent character-claim trials for one character, each trial accepts at most one claimant.
- **SC-011**: Authorization tests prove that a new connected, recognized, unassigned session can select an available character without controller, terminal, or generation authority, that acceptance can establish controller ownership under current rules, and that the same unassigned session cannot perform navigation or hacking mutations before it becomes the assigned, eligible, connected active controller.
- **SC-012**: At least 100 exact request replays whose records remain retained return the original result and revision with zero second canonical effects.
- **SC-013**: Reuse of a retained request identity with a changed procedure or fingerprint returns `duplicate` with zero canonical effects in every tested case.
- **SC-014**: Replay-cache stress tests prove the configured record bound is never exceeded and make no replay claim for evicted records.
- **SC-015**: Across at least 100 concurrent activations of one special pattern, every trial produces exactly one accepted mutation, exactly one normal accepted outcome draw, at most one accepted dud-selection draw, and zero random calls from rejected contenders.
- **SC-016**: For every accepted state-changing action, tests observe exactly one canonical mutation, one revision advance, one logical compound update containing all changed public components, and one offer to each affected logical session; every physical stream that remains active and responsive through delivery observes that revision exactly once, no stream observes it more than once, and canceled, disconnected, or overflowing streams may observe zero.
- **SC-017**: Every subscriber's post-snapshot revisions are strictly increasing while tests confirm irrelevant revisions may be skipped.
- **SC-018**: Four to seven simultaneous browser streams converge after at least 25 mixed character-selection, navigation, guess, pattern, replay, rejection, and reconnect operations.
- **SC-019**: Result-first and stream-first tests prove accepted pending actions clear only after both required conditions are met, while rejected actions clear immediately.
- **SC-020**: Before a deterministic blocked subscriber is released and within five seconds, a canonical mutation completes, a responsive subscriber receives its update, overflow closes only the blocked stream, and shutdown completes.
- **SC-021**: Wails adapter tests exercise every private method and event in the inventory while preserving all current JavaScript-facing values and object shapes.
- **SC-022**: Adding a private protobuf field without updating its Wails adapter causes verification to fail.
- **SC-023**: Repository scans find zero protobuf-binary Wails carriers, Base64 Wails codecs, ProtoJSON Wails bridges, or generic desktop dispatchers.
- **SC-024**: Public service enumeration and crafted requests find zero path to trusted desktop capabilities, native operations, runtime status, server information, credentials, private candidates, secret words, future outcomes, raw connection identities, or other sessions' private information.
- **SC-025**: The generated player code and public service descriptor transitively contain zero private desktop, persistence, credential, native-path, tunnel, or private hacking contracts.
- **SC-026**: Session-v1 fixture round trips preserve every known field, normalized relative player-config reference, and compatible unknown field at every currently supported level.
- **SC-027**: Player-config-v1 fixture tests preserve every current strict validation case and prove no roster or association publication occurs before successful atomic save.
- **SC-028**: Oversized unary mutation, Subscribe, SoundManifest, compressed, and unknown-field-enlarged requests exceeding the 4 KiB uncompressed message limit or planned encoded-body or decompression limit return `resource_exhausted`, and malformed bounded requests return `invalid_argument`; spies prove zero application-adapter and canonical-service invocations and zero session, mutation, revision, publication, replay, or random-source effects for every boundary rejection.
- **SC-029**: Sound tests cover all eight current categories, five allowlisted extensions, deterministic ordering, empty successful manifests, asynchronous failure, and authoritative one-shot cues.
- **SC-030**: Local-network and authenticated-ngrok journeys cover page loading, first snapshot, all five unary responsibilities, compound updates, multi-tab behavior, reconnect, HTTP `401`, sound discovery, and sound playback. This criterion requires evidence from the actual configured public ngrok endpoint; a local protected proxy alone does not satisfy it.
- **SC-031**: Before final cutover, a manual or scheduled soak representative of a three-to-four-hour game proves an idle local and authenticated-ngrok stream either remains usable or reconnects after interruption and receives a complete current snapshot. This criterion requires evidence from the actual configured public ngrok endpoint; a local protected proxy alone does not satisfy it.
- **SC-032**: A packaged offline run loads all generated player code and embedded assets without a CDN, external development server, or network-time package download.
- **SC-033**: `gofmt`, `go vet`, `go test`, `go test -race`, pinned-Buf-only schema and generation checks, frontend and player builds, browser journeys, and the affected macOS package smoke test all pass.
- **SC-034**: Final scans of source, dependencies, routes, built assets, fixtures, CSP, and active documentation find zero active WebSocket implementation, browser constructor, handwritten JSON player envelope, direct legacy dependency, active legacy fixture, or permanent dual stack.
- **SC-035**: A credential-gated clean-browser journey through the actual configured ngrok public URL receives a complete first player snapshot, hides `#connOverlay`, renders the current live terminal, and observes a later update or reconnect snapshot. If external ngrok credentials or connectivity are unavailable, the result MUST be recorded as `NOT RUN` and MUST NOT be used to mark SC-030 or SC-031 as passed.

## Assumptions

- Exact public protobuf package, service, procedure, and message names will be finalized during planning while preserving every separately typed responsibility in this specification.
- Planning will select browser protobuf JSON or binary encoding from current official ConnectRPC browser guidance and pin the chosen runtime and generation toolchain.
- The current process-local coordinator remains the canonical owner of navigation, hacking, assignment, controller, broadcast, revision, terminal runtime, presence, and replay behavior.
- The 4 KiB maximum measures the uncompressed protobuf request message for every public player RPC before application-adapter or canonical-service invocation; planning will record finite encoded HTTP-body and decompression limits that accommodate only documented Connect framing overhead.
- Planning will derive finite lower semantic bounds for recognition handles, request IDs, broadcast IDs, terminal and generation identifiers, pattern IDs, navigation targets, guesses, and sound categories without making recognition handles client-readable.
- Existing same-origin local-storage coordination used by first-time tabs may be adapted to generated subscriptions, but the only persistent value remains the accepted recognition handle.
- A later relevant revision can satisfy an accepted pending action when its personalized projection demonstrates state at or beyond the result revision even if irrelevant revisions were skipped.
- The current sound filename response may become safe relative paths if planning proves all values remain within the same allowlisted embedded category and origin.
- Historical feature documents remain useful records but no longer define an active protocol after they are clearly marked superseded.
- Contract details mandated by this migration are intentionally technical because they define externally observable compatibility and security boundaries; planning still owns file layout and implementation sequencing.

## Verbatim Constraints

- Feature title: `Protobuf-First ConnectRPC Migration`.
- Baseline commit: `8f306ce42e55b199ada4761d165461e2ebedc8ae`.
- Feature directory: `specs/005-connectrpc-protobuf-migration`.
- Development lifecycle command: `wails dev`.
- Legacy sound endpoint to remove: `GET /api/sounds/{folder}`.
- Public player port: `3690`.
- Default player delivery queue size: `32`.
- Default replay-cache limit: `256` entries per logical session and broadcast.
- Maximum uncompressed public player RPC request message: `4 KiB`.
- Current reconnect delay: `three seconds`.
- Public ngrok domain: provider-assigned unless a fixed domain is explicitly configured and saved.
- Invalid Basic Auth response: HTTP `401`.
- Connect error codes: `invalid_argument`, `resource_exhausted`, `unimplemented`, `canceled`, `unavailable`, `internal`.
- Stable action-result reasons: `accepted`, `invalid-session`, `stale-broadcast`, `unassigned`, `not-controller`, `controller-disconnected`, `stale-terminal`, `invalid-action`, `conflict`, `duplicate`.
- Legacy request names to supersede: `SESSION_HELLO`, `CHARACTER_SELECT`, `NAV_ACTION`, `HACK_GUESS`, `HACK_PATTERN`.
- Legacy publication/result names to supersede: `SESSION_WELCOME`, `PLAYER_STATE`, `ACTION_RESULT`, `TERMINAL_LIVE`, `TERMINAL_UPDATE`, `TERMINAL_CLEAR`, `NAV_STATE`, `HACK_STATE`.
- Removed request that must not return: `HACK_ADMIN`.
- Opaque generation-bound request field: `patternId`.
- Current Wails event names: `server-info`, `client-count`, `hack-state`, `coordination-state`.
- Current sound categories: `ambient`, `hack-good`, `hack-bad`, `menu-focus`, `single`, `multiple`, `enter`, `charscroll`.
- Current allowlisted sound extensions: `.mp3`, `.wav`, `.ogg`, `.m4a`, `.webm`.
- Session-v1 known JSON fields: `version`, `name`, `playerConfig`, `terminals`.
- Terminal known JSON fields: `id`, `name`, `hackLevel`, `introText`, `root`.
- Recursive content-node known JSON fields: `id`, `type`, `name`, `children`, `text`, `description`.
- Player-config-v1 known JSON fields: `version`, `name`, `roster`.
- Roster known JSON fields: `id`, `name`.
- Current tunnel environment keys: `NGROK_ENABLED`, `NGROK_BIN`, `NGROK_DOMAIN`, `NGROK_TIMEOUT`, `NGROK_TIMEOUT_MS`, `NGROK_USERNAME`, `NGROK_PASSWORD`, `NGROK_BASIC_AUTH`.
- Current tunnel command-line flags: `--ngrok`, `--ngrok-basic-auth`, `--ngrok-username`, `--ngrok-password`, `--ngrok-bin`, `--ngrok-domain`, `--ngrok-timeout`.
