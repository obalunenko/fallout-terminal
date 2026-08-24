# Tasks: HTTP/2 Presentation Intent Streaming

**Input**: Design documents from `specs/019-http2-presentation-streaming/`

**Bugfix**: 2026-08-25 — BUG-001 updated from bugfix patch.

**Prerequisites**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/`, and
`quickstart.md`

**Testing**: The specification and constitution require schema-first tests, colocated Go tests,
race coverage, Playwright journeys, independent HTTP/2 hop evidence, security checks, and arm64
packaging. Tests are written to fail before their corresponding implementation.

**Organization**: Tasks are grouped by prioritized user story. Explicit waves are dependency joins;
`[P]` marks tasks that touch different files and may proceed independently after the preceding join.

## Phase 1: Setup and Contract Baseline

**Purpose**: Establish the additive generated contract before any producer or consumer changes.

**Wave 1:**

- [x] **T001** [US2] Add `client_instance_id`, uplink open/intent/request/result/response messages, the targeted `SubscriptionMessage` variant, and the client-streaming `PresentationUplink` RPC with the approved field numbers and cardinality · `proto/fallout/terminal/player/v1/player.proto`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2:**

- [x] **T002** [US2] Regenerate pinned Go and ECMAScript protobuf/Connect outputs and advance the reviewed schema digest without editing generated files manually · `internal/gen/fallout/terminal/player/v1/player.pb.go`, `internal/gen/fallout/terminal/player/v1/playerv1connect/player.connect.go`, `frontend/client/gen/fallout/terminal/player/v1/player_pb.js`, `proto/schema-revision.txt`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [x] **T003** [US2] Prove the additive schema, generated client/server compilation, compatibility baseline, generation drift, and public-descriptor privacy before runtime work begins · `scripts/proto-check.sh`, `scripts/proto-breaking.sh`, `proto/compatibility-baseline.binpb`

**Checkpoint**: The generated contract is stable, compatible, secret-free, and available to every Go and browser consumer.

## Phase 2: Foundational Streaming Runtime

**Purpose**: Implement the generated adapters, bounded stream lifecycle, tab routing, targeted-result isolation, and exact streaming HTTP boundary that block every user-story integration.

### Tests

**Wave 1 — independent (different files):**

- [x] **T004** [P] [US2] Add RED protobuf-aware tests for open-frame and per-intent structural validation, required identity/context fields, generated descriptor cardinality, and secret/private-field exclusion · `internal/player/adapter_test.go`
- [x] **T005** [P] [US2] Add RED concurrency tests for client-instance registration, generation replacement, capacity-one latest server input, non-lossy and starvation-free targeted publication, canonical-queue isolation, cancellation, shutdown, and multi-tab result routing · `internal/player/stream_test.go`
- [x] **T006** [P] [US2] Add RED handler tests for open-first framing, current physical-subscription binding, per-intent authority/context validation, connection-aware coordinator dispatch, ready/result routing, limits, and stream closure · `internal/player/handler_test.go`
- [x] **T007** [P] [US2] Add RED HTTP boundary tests proving only the exact generated `PresentationUplink` procedure bypasses whole-body buffering while Host, same-origin, encoded-body, unknown-procedure, and all existing RPC protections remain unchanged · `internal/player/http_test.go`
- [x] **T008** [P] [US2] Add RED deterministic clock/rate tests for 4 KiB decoded messages, 120/s burst 32, 30-second idle, five-minute rotation, one generation per tab, 32 process-wide uplinks, and cancellation cleanup · `internal/player/limits_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — independent (different files):**

- [x] **T009** [P] [US2] Adapt generated uplink open and presentation intents into validated transport-independent runtime mutations while reusing the existing presentation and `ActionResult` vocabularies · `internal/player/adapter.go`
- [x] **T010** [P] [US2] Implement fixed stream size, rate, idle, lifetime, per-tab, and process concurrency limits with deterministic clock seams and no new dependency or user setting · `internal/player/limits.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [x] **T011** [US2] Extend `Subscription` and `SubscriptionHub` with ephemeral client identity, current generation ownership, a separate capacity-one non-lossy targeted mailbox, bounded canonical-first scheduling without targeted starvation, one receiver/processor uplink, capacity-one latest intent replacement, blocking publication scoped to the originating uplink, and idempotent cancellation/shutdown · `internal/player/stream.go`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent (different files):**

- [x] **T012** [P] [US2] Implement generated `PresentationUplink`, bind it to the current physical `Subscribe`, dispatch each intent through the connection-aware canonical coordinator, publish ready/action results before the next mutation, and close all uplinks with subscriptions · `internal/player/handler.go`
- [x] **T013** [P] [US2] Register the generated stream procedure and exempt only its open request body from full-body buffering while preserving per-message Connect limits and every existing public HTTP boundary · `internal/player/http.go`

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5:**

- [x] **T014** [US2] Add generated-client integration coverage for open/ready, latest server intent, ordered canonical mutation, non-lossy and non-starved targeted results, two tabs sharing recognition, stale generations, blocked targeted delivery, cancellation, and healthy-sibling isolation · `internal/player/public_stream_test.go`

**Checkpoint**: The bounded generated uplink works through the player boundary without changing canonical ownership or canonical subscription capacity.

## Phase 3: User Story 1 — Immediate Controller Presentation (Priority: P1) 🎯 MVP

**Goal**: The active controller sees each eligible semantic target by the next animation frame while observers, revisions, gameplay, and preview audio remain authoritative.

**Independent Test**: Delay `SetPresentation`, sweep at least 100 hacking targets, and verify next-frame local rendering, no duplicate semantic target, no stale visual/audio replay, and final convergence with two observers.

### Tests

**Wave 1:**

- [ ] **T015** [US1] ⚠️ Reopened — Add RED Playwright coverage for next-frame local menu/page/hacking presentation, a 100-target sweep, same-target duplicate suppression, delayed older revisions, authoritative-only preview audio, exactly one final authoritative menu and hacking cue under eligible audio, context/role invalidation, and final controller-plus-two-observer convergence (reopened — BUG-001) · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2:**

- [x] **T016** [US1] Implement the visual-only local transient keyed by authoritative context and monotonically increasing local sequence; render it on the next frame, reconcile canonical updates without stale overwrite/effect replay, keep audio authoritative, and clear it on authority/context/subscription invalidation · `frontend/client/client.js`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [ ] **T017** [US1] ⚠️ Reopened — Run the focused controller/observer journey and verify next-frame feedback, exactly one final authoritative menu and hacking cue, zero superseded highlight/reveal/audio replay, duplicate suppression, and one-second final convergence (reopened — BUG-001) · `npm test --prefix tests/browser -- player-sessions-control.spec.mjs`

**Checkpoint**: User Story 1 is independently functional and testable through the existing unary path before request streaming is enabled.

## Phase 4: User Story 2 — Stream Presentation Intents over the Public Path (Priority: P2)

**Goal**: An eligible authenticated HTTPS tab uses one bounded request stream only after an end-to-end ready probe, while both application-owned hops are proven HTTP/2 and canonical updates remain on `Subscribe`.

**Independent Test**: Traverse a protocol-faithful or real ngrok path, observe HTTP/2 independently at ingress and player, stall browser and server consumers during a 100-target sweep, and verify latest-only input, non-lossy targeted results, tab isolation, and final authoritative convergence.

### Tests

**Wave 1 — independent (different files):**

- [x] **T018** [P] [US2] Add RED player-listener tests for HTTP/1.1 static/unary/`Subscribe` compatibility, unencrypted HTTP/2 request streaming, independent `ProtoMajor == 2` observation, cancellation, and bounded shutdown · `internal/player/server_test.go`
- [x] **T019** [P] [US2] Add RED ingress tests for HTTP/1.1+h2c admission, h2c-only reverse-proxy transport, exact Host/Basic Auth checks, credential stripping, independent ingress/player protocol observations, cancellation, and local-player failure isolation · `internal/tunnel/public_ingress_test.go`
- [x] **T020** [P] [US2] Add RED adapter assertions that the SDK receives exactly `ngrok.WithUpstream(request.UpstreamURL, ngrok.WithUpstreamProtocol("http2"))` as an upstream option without changing endpoint policy · `internal/tunnel/ngrok_test.go`
- [x] **T021** [P] [US2] Extend browser fixtures and the authenticated Connect journey for HTTPS eligibility, generated open-frame probing, ready-before-request-EOF, stalled request-body demand, at most one handed-off plus one pending browser intent, targeted-result routing, two observers, and no handwritten procedure path/DTO · `tests/browser/fixture-server/main.go`, `tests/browser/connectrpc-player.spec.mjs`
- [x] **T022** [P] [US2] Extend the opt-in real-provider test to open the generated client stream through ngrok, observe ready and action/canonical downlinks, record HTTP/2 independently at ingress and player, rotate generation, and report missing credentials/connectivity as `NOT RUN` · `internal/tunnel/ngrok_integration_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — independent (different files):**

- [x] **T023** [P] [US2] Configure the player `http.Server.Protocols` for HTTP/1.1 plus unencrypted HTTP/2 on its existing listener while preserving its owned context, five-second shutdown budget, and LAN URL · `internal/player/server.go`
- [x] **T024** [P] [US2] Configure ingress `http.Server.Protocols` for HTTP/1.1+h2c and an owned h2c-only reverse-proxy `http.Transport`, retaining fail-closed policy, Basic Auth, header stripping, cancellation, and public-only failure · `internal/tunnel/public_ingress.go`
- [x] **T025** [P] [US2] Supply the exact HTTP/2 upstream option to the ngrok SDK while leaving reserved-domain endpoint options and lifecycle/error redaction unchanged · `internal/tunnel/ngrok.go`
- [x] **T026** [P] [US2] Implement the narrow generated-descriptor Fetch transport with official Connect URL/header/serialization/envelope helpers, `ReadableStream` plus `duplex: "half"`, a capacity-one outbound mailbox/puller, same-origin credentials, capability checks, and delegation of all non-uplink procedures · `frontend/client/presentation-uplink.js`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent (different files):**

- [x] **T027** [P] [US2] Generate one ephemeral in-memory `client_instance_id` per page, attach it to `Subscribe`, manage monotonically newer uplink generations, accept only matching ready/results, enable streaming only after the end-to-end probe, and route semantic presentation through the shared latest-only dispatcher · `frontend/client/client.js`
- [x] **T028** [P] [US2] Add production-handler integration proving HTTP/1.1 compatibility, both local h2c hops, exact stream-only body behavior, canonical/result queue isolation, and connection-aware multi-tab ordering with generated clients · `internal/player/public_stream_test.go`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4:**

- [x] **T029** [US2] Run focused player/tunnel tests and the authenticated browser stream journey; verify the 100-target bounds, final target, processed-result delivery, HTTP/1.1 compatibility, independent HTTP/2 hop evidence, auth stripping, and no cross-tab result · `go test ./internal/player ./internal/tunnel`, `npm test --prefix tests/browser -- connectrpc-player.spec.mjs`

**Checkpoint**: User Story 2 is independently functional on an eligible public path and does not alter the authoritative downlink or portable unary behavior.

## Phase 5: User Story 3 — Fall Back and Recover without Losing Control (Priority: P3)

**Goal**: Direct LAN HTTP, unsupported browsers, failed probes, stream interruption, rotation, and lifecycle invalidation continue automatically through BUG-010's bounded unary dispatcher.

**Independent Test**: Disable request-stream support, use direct LAN HTTP, fail and buffer the probe, interrupt an active generation, and change role/context/reconnect state; each case sends at most the latest eligible target through unary and leaves all other gameplay unchanged.

### Tests

**Wave 1 — independent (different files):**

- [x] **T030** [P] [US3] Add RED public-access fallback journeys for direct LAN HTTP, missing request-stream APIs, failed/timeout/buffered probes, wrong credentials, interrupted or rotated uplinks, public unavailability, and later fresh-probe recovery · `tests/browser/public-access-fallback.spec.mjs`
- [ ] **T031** [P] [US3] ⚠️ Reopened — Add RED controller regression journeys proving stream failure transfers only the newest eligible target to unary, preserves exactly one final authoritative menu and hacking cue through interruption/rotation and later recovery, never sends one target through both transports, clears stale local state on role/broadcast/terminal/context/reconnect changes, and leaves navigation, guessing, pattern activation, character selection, and observer convergence unchanged (reopened — BUG-001) · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2:**

- [x] **T032** [US3] Integrate transport invalidation and fallback ownership so unsupported/ineligible pages never probe, failures abort the generation and request body, only the newest still-eligible mailbox/local target transfers to the unchanged one-in-flight/one-latest unary dispatcher, and obsolete transient/stream state clears on every lifecycle boundary · `frontend/client/client.js`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [ ] **T033** [US3] ⚠️ Reopened — Run focused fallback, controller, HTTP/1.1, auth, and existing gameplay journeys and verify automatic convergence, exactly-once final movement cues and later cue recovery, with no duplicate mutation or loss of local/LAN service (reopened — BUG-001) · `npm test --prefix tests/browser -- public-access-fallback.spec.mjs player-sessions-control.spec.mjs`, `go test ./internal/player ./internal/tunnel`

**Checkpoint**: All three user stories are independently functional; streaming remains an optional optimization and cannot block basic player control.

## Phase 6: Polish and Cross-Cutting Validation

**Purpose**: Reconcile active documentation and run the single owned Success-Criteria validation sequence.

**Wave 1:**

- [x] **T034** Reconcile implementation details, runnable commands, limits, fallback behavior, and honest conditional evidence with the accepted design; remove any stale latest-result/drop wording · `specs/019-http2-presentation-streaming/plan.md`, `specs/019-http2-presentation-streaming/research.md`, `specs/019-http2-presentation-streaming/data-model.md`, `specs/019-http2-presentation-streaming/contracts/`, `specs/019-http2-presentation-streaming/quickstart.md`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2:**

- [x] **T035** Validate schema formatting/lint/build, deterministic generation, breaking compatibility, generated compilation, descriptor privacy, secret leakage, and unchanged session/player-configuration round trips · `make proto-check`, `make proto-breaking`, `scripts/secret-leak-check.sh`, `go test ./internal/session ./internal/playerconfig`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [x] **T036** Validate formatting, vet, pinned lint, complete Go behavior, concurrent stream/tunnel lifecycle, cancellation, shutdown, and leak safety · `make fmt-check`, `go vet ./...`, `make lint`, `go test ./...`, `go test -race ./...`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4:**

- [ ] **T037** ⚠️ Reopened — Validate the player production build and complete Playwright suite, including ≥100-target next-frame/latest-only behavior, exactly one final authoritative menu and hacking cue with zero superseded cues, cue recovery after interruption, two-observer convergence, all fallback paths, HTTP auth, reconnect, and unchanged gameplay (reopened — BUG-001) · `npm run build:client --prefix frontend`, `npm test --prefix tests/browser`

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5:**

- [x] **T038** Build and package the arm64 application, run the documented controller/observer packaged smoke, and execute the credential-gated real-ngrok journey when prerequisites exist; record unavailable external evidence as `NOT RUN` · `go run ./cmd/build build`, `go run ./cmd/build package`, `internal/tunnel/ngrok_integration_test.go`, `specs/019-http2-presentation-streaming/quickstart.md`

## Phase 7: BUG-001 — Authoritative Movement Cue Recovery

**Purpose**: Reproduce and fix missing final menu/hacking cues across public-stream cancellation,
fallback, and recovery without making transient or superseded targets audible.

**Wave 1 — independent (different browser test files):**

- [ ] **T039** [P] [US1] Add a deterministic RED public-HTTPS/ngrok-faithful journey that records cue dispatch, forces `PresentationUplink` cancellation or rotation, stops menu and hacking movement, and asserts exactly one matching final authoritative cue for each category with zero superseded cues · `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/fixture-server/main.go`
- [ ] **T040** [P] [US3] Add a RED recovery journey proving that the surviving final target and later accepted menu/hacking movements dispatch exactly one cue after unary fallback and after a fresh uplink generation becomes ready · `tests/browser/public-access-fallback.spec.mjs`

**Reopened task join**: T015 and T031 must be RED with the BUG-001 lower-bound assertions before
Wave 2 begins.

**⟶ Wait for Wave 1 and the reopened task join to finish, then:**

**Wave 2:**

- [ ] **T041** [US1] [US3] Reproduce the failing stage and fix the identified cue-reconciliation or audio-retry path so final applicable authoritative transitions remain exactly-once across cancellation, fallback, and recovery; add bounded test diagnostics that distinguish dispatch from manifest, fetch, decode, `AudioContext` resume, and source-start failure without exposing secrets or adding noisy first-party console output · `frontend/client/client.js`, `frontend/client/sound.js`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [ ] **T042** [US1] [US3] Complete reopened T017, T033, and T037; run the focused and full browser suites, production build, packaged controller/observer smoke, and credential-gated real-ngrok journey, recording external prerequisites as `NOT RUN` and confirming exactly one final menu/hacking cue, zero superseded cues, and later cue recovery · `npm test --prefix tests/browser -- player-sessions-control.spec.mjs public-access-fallback.spec.mjs`, `npm test --prefix tests/browser`, `npm run build:client --prefix frontend`, `go run ./cmd/build build`, `go run ./cmd/build package`, `internal/tunnel/ngrok_integration_test.go`

## Dependencies & Execution Order

- Phase 1 Wave 1 fixes the protobuf source; Wave 2 regenerates all consumers; Wave 3 gates compatibility and privacy before runtime work.
- Phase 2 Wave 1 adds independent RED adapter, stream, handler, HTTP, and limit tests; Wave 2 implements adapters and limits independently; Wave 3 joins them in runtime ownership; Wave 4 implements handler and HTTP boundary independently; Wave 5 proves their generated-client integration.
- Phase 3 Wave 1 establishes the RED immediate-feedback journey; Wave 2 implements the local transient; Wave 3 verifies the independent unary-backed MVP.
- Phase 4 Wave 1 adds independent player, ingress, ngrok, browser, and real-provider tests; Wave 2 implements each subsystem independently; Wave 3 joins browser and server integration; Wave 4 runs the focused public-stream proof.
- Phase 5 Wave 1 adds independent public-path and gameplay fallback tests; Wave 2 integrates fallback ownership in the shared browser state; Wave 3 verifies recovery and compatibility.
- Phase 6 updates design evidence first, then runs schema/security/persistence, Go/race, browser, and build/package/real-provider validation in order.
- Phase 7 runs T015, T031, T039, and T040 as the RED join; T041 applies the proven fix; T017, T033, T037, and T042 then provide focused, full, packaged, and conditional real-provider verification.
- Overall order is Setup → Foundational → US1 → US2 → US3 → Polish → BUG-001. The bugfix phase must keep every earlier checkpoint green.

## Parallel Opportunities

- Phase 2 Wave 1 tests are independent by Go file; adapter and limit implementation in Wave 2 is independent after those tests settle.
- Phase 2 Wave 4 handler and HTTP boundary work touches different files after stream ownership is stable.
- Phase 4 Wave 1 tests and Wave 2 subsystem implementations are independent by player, ingress, ngrok, and browser file boundaries.
- Phase 4 Wave 3 browser integration and generated-client Go integration touch different files after all protocol producers are complete.
- Phase 5 fallback tests may be authored independently, but the shared `frontend/client/client.js` implementation remains single-owner and follows both.
- BUG-001 T039 and T040 may proceed in parallel because they use different browser journey files;
  T041 is a single-owner browser runtime change after both RED proofs, and T042 is sequential.
- Validation waves are intentionally sequential so generation, race, browser, package, and external-service evidence cannot interfere or mask drift.
