# Implementation Plan: Protobuf-First ConnectRPC Migration

**Branch**: `005-connectrpc-protobuf-migration` | **Date**: 2026-08-13 | **Spec**: `specs/005-connectrpc-protobuf-migration/spec.md`

**Bugfix**: 2026-08-13 — BUG-001 Updated from bugfix patch
**Bugfix**: 2026-08-13 — BUG-002 Tightened authenticated-ngrok verification so a real public generated `Subscribe` stream, first snapshot, overlay dismissal, terminal render, and reconnect are required evidence rather than inferred from a local protected proxy and unary RPC.

## Summary

Replace the handwritten public WebSocket/JSON player protocol with a protobuf-first ConnectRPC boundary while preserving the current Wails desktop surface, portable JSON files, server-authoritative coordination, gameplay, sound, ngrok, and packaged-offline behavior. Versioned public, private-desktop, persistence, and configuration schemas will generate detached Go boundary values; the public package alone will also generate ECMAScript descriptors consumed by a bundled same-origin Connect browser client. The migration starts with a thin generated subscription/snapshot and unary-selection proof, then moves every responsibility to typed procedures and compound revisioned publications before removing the WebSocket route, dependency, fixtures, CSP allowances, and active legacy documentation.

## Project Structure

```text
specs/005-connectrpc-protobuf-migration/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
└── contracts/
    ├── inventory.md
    ├── public-player.md
    ├── private-desktop.md
    └── persistence-configuration.md

proto/
├── buf.yaml
├── buf.gen.go.yaml
├── buf.gen.es.yaml
└── fallout/terminal/
    ├── player/v1/
    │   ├── player.proto
    │   ├── terminal.proto
    │   ├── navigation.proto
    │   ├── hacking.proto
    │   └── sound.proto
    ├── private/v1/
    │   ├── desktop.proto
    │   ├── coordination.proto
    │   └── runtime.proto
    ├── persistence/v1/
    │   ├── session.proto
    │   └── player_config.proto
    └── config/v1/
        └── config.proto

internal/gen/fallout/terminal/             # Checked-in deterministic generated Go values and Connect handlers
├── player/v1/
├── private/v1/
├── persistence/v1/
└── config/v1/

main.go                                     # Generated handler composition, static embedding, effect routing
app.go                                      # Unchanged named Wails methods and compatibility DTO surface
app_contract.go                             # Explicit private-protobuf/Wails/domain adapters
app_contract_test.go                        # Private descriptor and adapter-exhaustiveness checks

internal/
├── domain/
│   ├── model.go                            # Canonical transport-independent aggregates and projections
│   ├── validate.go                         # Existing persistence validation plus bounded public identities
│   └── *_test.go                           # Detached model, compatibility, and finite-bound regressions
├── control/
│   ├── service.go                          # Atomic recognition, stream attachment, mutation, revision, replay
│   └── service_test.go                     # Gap-free snapshots, compound commits, authority, replay, races
├── live/
│   ├── service.go                          # Unchanged navigation/hacking mechanics and random-source ownership
│   └── service_test.go                     # Snapshot non-generation and unchanged feature-003 rules
├── player/
│   ├── adapter.go                          # Public protobuf ↔ transport-independent request/projection mapping
│   ├── handler.go                          # Generated Connect service implementation and error mapping
│   ├── limits.go                           # Body, decompression/message, field, and category limits
│   ├── stream.go                           # Per-physical-stream bounded queue and cancellation lifecycle
│   ├── http.go                             # Generated RPC routes plus ordinary same-origin static resources
│   ├── server.go                           # Listener ownership, registry, shutdown, and client-count events
│   └── *_test.go                           # RPC, descriptor, limit, stream, same-origin, and cutover coverage
├── session/
│   ├── contract.go                         # Persistence protobuf ↔ session-v1 JSON/domain adapter
│   └── *_test.go                           # Known fields plus recursive unknown-field preservation
├── playerconfig/
│   ├── contract.go                         # Persistence protobuf ↔ strict player-config-v1 adapter
│   └── *_test.go                           # Strict decode and save-before-publication compatibility
├── tunnel/
│   ├── config.go                           # Existing precedence and credential validation
│   └── *_test.go                           # Protected Connect/static traffic and redaction checks
├── platform/
│   └── assets_test.go                      # Public/private bundle and legacy-protocol source scans
└── testutil/testdata/
    ├── protobuf/                           # Binary/JSON/golden descriptor and malformed request fixtures
    └── persistence/                        # Session-v1 and player-config-v1 compatibility fixtures

client/
├── package.json
├── package-lock.json
├── vite.config.js                          # Offline generated-client bundle into client/dist
├── gen/fallout/terminal/player/v1/         # Checked-in public ECMAScript output only
├── index.html
├── client.js                               # Generated Connect client, recognition, pending reconciliation
├── sound.js                                # Typed SoundManifest use and authoritative cue handling
├── client.css
├── sounds/
└── dist/.keep                              # Clean-checkout embed marker; build output remains generated

frontend/src/
├── desktop-api.js                          # Exact named Wails facade and event compatibility
└── master.js                               # Existing native-object shapes and authoritative rendering

tests/browser/
├── fixture-server/main.go                  # Production-shaped generated Connect fixture
├── connectrpc-player.spec.mjs              # Subscription, unary, replay, reconnect, multi-tab, sound journeys
├── player-sessions-control.spec.mjs        # Migrated feature-004 parity journeys
└── hacking-camouflage.spec.mjs             # Migrated feature-003 parity journeys

go.mod                                      # Pinned runtimes and Go 1.26 tool directives
go.sum
wails.json                                  # One-command master and player build orchestration
.github/workflows/wails-macos.yml            # Schema, generation, build, test, race, and package gates
README.md                                   # Connect operation and superseded WebSocket guidance
```

**Structure Decision**: Keep schemas upstream under `proto/`, check generated language outputs into isolated `internal/gen/` and `client/gen/` trees, and colocate explicit adapters with the existing boundary owners. `internal/control` remains the sole canonical transaction/revision owner, `internal/player` becomes the generated Connect/static transport boundary, and no generated message becomes a mutable domain or persistence aggregate.

## Constitution Check

The pre-research gate passes, and the post-design re-check passes against the completed data model and contracts. No constitutional exception or temporary permanent-dual-stack waiver is required.

| Principle | Assessment |
|---|---|
| I. Govern the Current Production Architecture | PASS: composition remains in `main.go`/`app.go`; canonical coordination and gameplay stay in `internal/control`, `internal/live`, `internal/nav`, and `internal/hack`; generated values are detached at adapters; all affected producers, consumers, owners, and tests are listed above. |
| II. Make Protobuf the Application Contract Source of Truth | PASS: every inventoried application-owned public, Wails, persistence-known-field, and serializable configuration boundary is classified and assigned to a versioned schema; third-party schemas and injected dependencies remain explicitly excluded. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS: `PlayerService` uses one server stream and five separately typed unary procedures; `control.Service` validates and commits canonical state once; browser code waits for typed results and authoritative stream revisions and carries no handwritten transport DTO. |
| IV. Separate Public and Private Capabilities | PASS: the public package has no private imports or trusted procedures; generated player inputs and the bundled graph are inspected; Wails retains exact named native-object methods/events through exhaustive private adapters. |
| V. Evolve Schemas Safely and Reproducibly | PASS: packages are versioned, variants use `oneof`, meaningful scalar presence uses `optional`, enums have `UNSPECIFIED`, tools/runtimes are pinned, generated files are never hand-edited, and format/lint/generation/breaking checks are planned. |
| VI. Preserve Portable Session JSON Version 1 | PASS: protobuf defines known semantics but `domain.DecodeSession`/`EncodeSession` and strict player-config JSON remain the file codecs; names, validation, relative references, recursive session extras, atomic saves, and runtime-state exclusion remain unchanged. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS: coexistence is branch-only and gated by a thin vertical proof plus parity tests; final acceptance removes every active WebSocket route, constructor, dependency, envelope, fixture, CSP allowance, and authoritative document. |

## Implementation Strategy

### 1. Freeze the inventory and toolchain

Treat `contracts/inventory.md` as the blocking migration ledger. Add Buf v2 configuration and pin Buf CLI `v1.72.0`, ~~Protocol Buffers compiler/toolchain `v35.0`,~~ `protoc-gen-go`/Go runtime `v1.36.11`, Connect-Go/generator `v1.20.0`, Protobuf-ES/generator `2.13.0`, and Connect-ES browser packages `2.1.2`. **BUG-001 correction**: the standalone compiler pin is superseded because the pinned Buf CLI owns the in-process protobuf compiler used by `buf generate`; generation MUST NOT download or invoke Google `protoc`. Go 1.26 tool directives own the Go generators and Buf CLI; `client/package-lock.json` owns the ECMAScript generator, runtime, Connect transport, and Vite bundle. Generation writes only to the isolated generated trees through the checked-in `proto/buf.gen.go.yaml` and `proto/buf.gen.es.yaml` templates, and a second clean generation must produce zero diff. Buf-generated Go headers reporting `protoc unknown` are acceptable when generated markers, plugin pins, schema revision, deterministic hashes, and output isolation all verify.

### 2. Define public and private schema graphs

Create `fallout.terminal.player.v1` as the only player-bundle input. It exposes `PlayerService` with `Subscribe`, `SelectCharacter`, `Navigate`, `Guess`, `ActivatePattern`, and `SoundManifest`; it imports no private, persistence, configuration, native-path, tunnel, credential, or secret-hacking package. Define `fallout.terminal.private.v1`, `fallout.terminal.persistence.v1`, and `fallout.terminal.config.v1` as separate private graphs. Generate Go for all four graphs with `buf generate --template proto/buf.gen.go.yaml` and ECMAScript only for the public graph with `buf generate --template proto/buf.gen.es.yaml`, then verify descriptors and transitive imports.

### 3. Build the thin vertical proof before bulk migration

Mount the generated Connect handler beside ordinary static assets on the current listener, explicitly select binary protobuf in the browser transport, and prove one absent-handle `Subscribe` snapshot plus `SelectCharacter`. Exercise the same generated bundle in local same-origin mode, authenticated `fixed-host.example` mode, invalid Basic Auth HTTP `401`, and a clean packaged offline build. Temporary WebSocket coexistence is permitted only on this feature branch while these parity tests are built; no production capability is duplicated into a generic dispatcher.

For BUG-002, the acceptance run MUST target the actual configured public ngrok endpoint in a clean browser, capture redacted browser/network/application/tunnel diagnostics for the streaming request, and assert that the connection overlay is dismissed and the current terminal is visible. The local protected-forwarding fixture remains a component test and is not a substitute for this public-stream proof. Diagnose failures across forwarded Host/Origin, Basic Auth propagation, Connect content type/status, response flushing or buffering, and cancellation/reconnect behavior before selecting the production fix.

The confirmed BUG-002 boundary is ngrok traffic-policy processing itself: a non-empty Connect streaming POST is held until cancellation even when its Basic Auth rule does not match, while an otherwise identical tunnel without a traffic-policy file streams immediately. The corrected design therefore forwards ngrok HTTP traffic without credential-bearing arguments or a traffic-policy file and enforces the same fail-closed Basic Auth pair inside the player application for every request whose Host exactly matches the configured public domain. This keeps the entire public surface protected, leaves local/LAN hosts unchallenged, and preserves existing same-origin validation.

### 4. Make snapshot attachment and compound publication atomic

Refactor the canonical coordinator boundary so stream recognition, logical-session resolution, physical-stream registration, and snapshot capture occur under the coordinator order. Register the bounded stream sink before returning the revision-R snapshot; the handler sends that snapshot first and then drains only queued revisions greater than R. Each accepted state-changing action mutates the cloned canonical aggregate once, advances one revision, constructs one complete personalized `CompoundUpdate` per affected logical session, offers it once before the unary response completes, and produces no same-revision component messages.

Each physical subscription owns a queue of `32` complete stream values. Offering is non-blocking; overflow cancels only that stream and old incremental values are never retried. Multiple streams for one logical session receive equivalent values while active and responsive. First attach and final detach drive aggregate presence; raw client count continues to equal physical public streams.

### 5. Move every mutation to a typed unary procedure

Adapt the existing selection, navigation, guessing, and pattern rules to separately generated request types. Validate transport size, structural variants, field bounds, sound category, handle form, active subscription, request identity, broadcast, assignment, controller, terminal, generation, and action semantics before gameplay or randomness. Character selection deliberately stops before controller/terminal/generation checks; the other three mutation families require the assigned connected active controller and current identities. Keep the replay cache at `256` records per logical session and broadcast, include procedure and deterministic protobuf payload bytes in the fingerprint, and retain the current deterministic-but-nonpublic eviction policy.

### 6. Replace browser transport state with generated values

Preserve the Web Locks/storage-lease first-tab election, but hold it until the first generated snapshot returns an accepted recognition handle. Persist only that opaque handle. Apply `PersonalizedSnapshot` and `CompoundUpdate` through explicit generated-value-to-view-model adapters; never persist or optimistically mutate assignment, roles, terminal, navigation, hacking, or pending state. Reconcile accepted actions only after both the unary result and an applicable stream revision arrive in either order; reject results clear immediately. Retain the exact three-second reconnect delay and recover terminated streams only through a new complete snapshot.

### 7. Govern Wails, persistence, and configuration without changing their transports

Keep every current App method, desktop facade function, native JavaScript object shape, and `server-info`, `client-count`, `hack-state`, and `coordination-state` event. Explicit adapters map Wails compatibility DTOs to generated private messages and transport-independent values; descriptor-driven exhaustiveness tests fail when either side adds a field or variant without a mapping. Wails continues native structured-object marshalling and carries no protobuf serialization.

Keep session-v1 and player-config-v1 JSON codecs authoritative. Persistence adapters cover all known fields while session extras remain attached at every supported level and player-config extras remain errors. Configuration schemas describe serializable values and defaults but never credentials in public output; injected filesystems, services, callbacks, clocks, random sources, processes, listeners, and event sinks remain native exclusions.

### 8. Complete the one-protocol cutover

After all Go, browser, local, ngrok, sound, reconnect, concurrency, and packaged parity gates pass, remove `internal/player/protocol.go`, the active WebSocket server/client implementation, `github.com/coder/websocket`, legacy protocol fixtures and source assertions, `GET /api/sounds/{folder}`, browser `WebSocket` construction, and `ws:`/`wss:` CSP allowances. Update active documentation to ConnectRPC and mark retained completed WebSocket feature records as superseded and non-authoritative. Scan both source and built assets before final acceptance.

## Verification Plan

| Surface | Automated evidence | Expected result |
|---|---|---|
| Inventory and schema shape | Inventory classifier; `buf format --diff --exit-code`; `buf lint`; descriptor assertions | Zero unclassified items, zero lint/format findings, exact packages/procedures/variants |
| Reproducible generation | Two clean pinned-Buf generation passes through `proto/buf.gen.go.yaml` and `proto/buf.gen.es.yaml`; generated-marker/plugin/schema-revision/hash checks compatible with `protoc unknown`; `git diff --exit-code`; scan for standalone `protoc` download or invocation | Byte-identical Go/ECMAScript output, one schema revision, and no standalone compiler path |
| Compatibility | `buf breaking` against the established git baseline; negative fixture edits | Field-number/type, enum, package, and service breaks are rejected once a baseline exists |
| Public/private separation | Descriptor import walk, generated-client dependency walk, bundled-source scan | No private/native/persistence/tunnel/credential/secret contract reaches the player |
| Go adapters and domain | `gofmt -l .`; `go vet ./...`; `go test ./...` | No formatting paths; all generated, adapter, domain, persistence, and transport tests pass |
| Concurrency and streaming | `go test -race ./...`; 100 recognition/replay/pattern races; deterministic blocked subscriber | One mutation/revision/logical update, strict stream revisions, isolated overflow, bounded shutdown |
| Request safety | Crafted binary, malformed, compressed, unknown-field, oversized, canceled, unsupported requests | Exact Connect codes; zero adapter/service calls and zero state, replay, publication, or random effects at rejected boundaries |
| Master frontend | `npm ci --prefix frontend`; `npm run build --prefix frontend`; Playwright/native Wails method/event matrix | Exact method names, argument/result shapes, events, and private capabilities remain usable |
| Player bundle | `npm ci --prefix client`; `npm run build --prefix client`; bundle inspection | Generated public client is self-contained, same-origin, offline, and private-contract-free |
| Browser journeys | `npm ci --prefix tests/browser`; `npm test --prefix tests/browser` | First snapshot, five unary methods, compound updates, pending order, multi-tab, reconnect, sound, authority, Basic Auth; authenticated-ngrok streaming acceptance opens the actual public URL in a clean browser and observes first snapshot, overlay dismissal, terminal render, update/action, and reconnect |
| Long-lived operation | Three-to-four-hour local and authenticated-ngrok soak | Idle stream stays usable or reconnects to a complete current snapshot; the ngrok portion uses the actual configured public endpoint or is recorded as `NOT RUN`, never passed from synthetic protected-forwarding coverage alone |
| Packaging | `wails build -clean -platform darwin/arm64`; packaged `.app` smoke with networking disabled | Generated player code and all static/sound assets load without CDN or development server |
| Final cutover | Source, dependency, route, CSP, fixture, built-bundle, and docs scans | Zero active WebSocket implementation, dependency, envelope, fixture, allowance, or permanent dual stack |
