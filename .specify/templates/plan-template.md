# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]

**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `$speckit-plan` command.

## Summary

[Extract the primary requirement and technical approach from the feature spec]

## Technical Context

**Language/Version**: Go 1.27; browser JavaScript modules with Node.js 20.19+ build/test tooling

**Primary Dependencies**: Wails v3.0.0-beta.13, `connectrpc.com/connect` v1.20.0, `google.golang.org/protobuf` v1.36.11, `@connectrpc/connect`/`@connectrpc/connect-web` 2.1.2, and Vite 8.1.5; Playwright 1.62.1 for browser journeys

**Storage**: Versioned local JSON session and player-configuration files; ephemeral live, navigation, hacking, connection, and coordination state in process memory

**Testing**: Colocated Go `*_test.go` tests with deterministic fakes; Playwright specs under `tests/browser/*.spec.mjs`; repository-wide Go lint through `.golangci.yml` and `task lint`; no numeric coverage threshold

**Target Platform**: Wails desktop application on `darwin/arm64`, `windows/amd64`, `windows/arm64`, `linux/amd64`, and `linux/arm64`, with modern browser clients on the local network or an authenticated public endpoint

**Project Type**: Go modular desktop monolith with a Wails Overseer frontend and an embedded static-HTTP/ConnectRPC player frontend

**Performance Goals**: [Feature-specific responsiveness, synchronization, startup, persistence, or client-count goal]

**Constraints**: Preserve narrow Wails bindings, generated protobuf/ConnectRPC contracts, server-authoritative shared state, persistent JSON compatibility, governed browser-origin policy, owned-resource cleanup, and the single-process runtime

**Scale/Scope**: One Overseer desktop process, one active broadcast, and [expected connected player-client count or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research and be re-checked after Phase 1 design.*

- [ ] Runtime ownership remains within `main.go`/`app.go`, the relevant `internal/` packages, `frontend/overseer/src/`, `frontend/client/`, and `sessions/` boundaries.
- [ ] Cross-boundary Wails, protobuf/ConnectRPC, and HTTP asset contracts document producer, consumer, payload, validation, failure, ordering, and reconnect behavior.
- [ ] Shared navigation, hacking, roster, and controller behavior remains server-authoritative and reconnect-safe.
- [ ] Wails method exposure, CSP, external URL handling, player-origin/input checks, and public-access secret protections are preserved where applicable.
- [ ] Session or player-configuration changes define versioning, defaults, references, migration, and backward compatibility.
- [ ] Runtime-only state remains outside persistent JSON unless persistence is explicitly approved.
- [ ] New dependencies or structural changes have a concrete, documented need and reproducible pinning.
- [ ] Verification uses the configured Go, Playwright, Vite, Wails, and CI gates that apply; unavailable checks are reported rather than claimed.
- [ ] Naming and code style match the conventions of the affected files.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── spec.md
├── plan.md
├── research.md          # Include only when research decisions are needed
├── data-model.md        # Include for persistent or runtime model changes
├── quickstart.md        # Feature verification instructions
├── contracts/           # Include for Wails/protobuf/ConnectRPC/HTTP asset/JSON contracts
└── tasks.md
```

### Source Code (repository root)

```text
main.go                         # Wails entry point, embedding, service composition
app.go                          # Privileged Wails bridge and application lifecycle
internal/
├── domain/                     # Models, JSON codecs, cloning, validation
├── nav/                        # Transport-independent navigation rules
├── hack/                       # Transport-independent hacking rules and wordbank
├── live/                       # Canonical live terminal/navigation/hack state
├── control/                    # Sessions, roster, controller, broadcast coordination
├── session/                    # Session persistence and native file workflow
├── playerconfig/               # Player configuration persistence and references
├── player/                     # HTTP assets, ConnectRPC service, public protocol
├── platform/                   # Wails desktop adapters and supported-platform paths
├── tunnel/                     # Optional embedded public-endpoint lifecycle
└── testutil/                   # Shared deterministic test fakes and fixtures
frontend/
├── package.json
├── package-lock.json
├── client/
│   ├── index.html
│   ├── client.js               # Player state/rendering and ConnectRPC client
│   ├── sound.js                # Browser audio behavior
│   ├── client.css
│   ├── gen/
│   └── sounds/
└── overseer/
    ├── package.json
    ├── vite.config.js
    ├── bindings/               # Generated narrow Wails bridge
    └── src/
        ├── index.html
        ├── desktop-api.js      # Narrow Wails binding/event adapter
        ├── overseer.js         # Overseer state and interactions
        └── overseer.css
tests/browser/
├── *.spec.mjs                  # Playwright player journeys
├── fixture-server/main.go      # In-process browser-test fixture
└── playwright.config.mjs
sessions/
└── demo.json                   # Versioned example session
build/                          # Platform metadata, icon, hooks, and output location
scripts/build-macos.sh          # Optional manual signed/notarized macOS distribution pipeline
```

**Structure Decision**: [List affected paths, their ownership, and why the feature belongs in each]

## Contract and State Design

### Persistent JSON

[Document changed session/player-configuration fields, versioning, references, validation, defaults, migration, atomic-save behavior, and fixture updates, or N/A]

### Wails Bridge and Runtime Events

[Document bound method/event names, directions, payloads, public projections, validation, errors, readiness, and shutdown behavior, or N/A]

### Protobuf/ConnectRPC and HTTP Assets

[Document RPC methods/cardinality, generated messages, static routes, directions, origin and size checks, server validation, authorization, revisions, publications, action results, and reconnect state, or N/A]

### Runtime-State Lifecycle

[Describe creation, mutation, publication, clearing, reconnection, and the persistence boundary for live and coordination state, or N/A]

### Platform, Tunnel, and Packaging

[Describe supported-platform paths/dialogs, embedded provider resources, OS secure-store secrets, temporary material, embedding, build, or release implications, or N/A]

## Implementation Phases

### Phase 0: Research and Decisions

- [Resolve actual unknowns; omit generic research]
- [Confirm platform, protocol, persistence, concurrency, or compatibility decisions]
- [Choose a new dependency only when existing tools cannot satisfy a documented need]

### Phase 1: Contracts and Data Design

- [Define persistent JSON, Wails, protobuf/ConnectRPC, and HTTP asset contracts as applicable]
- [Define validation, public projection, ordering, compatibility, and reconnection behavior]
- [Map producer and consumer changes to exact paths]
- [Re-run the Constitution Check after design]

### Phase 2: Domain, Persistence, and Transport Foundations

- [Implement pure models/rules in `internal/domain/`, `internal/nav/`, or `internal/hack/`]
- [Implement canonical state and coordination in `internal/live/` or `internal/control/`]
- [Implement persistence in `internal/session/` or `internal/playerconfig/`]
- [Implement static HTTP and ConnectRPC behavior in `internal/player/`]

### Phase 3: Desktop and Presentation Integration

- [Wire services or privileged commands/events in `main.go` and `app.go`]
- [Implement Overseer behavior in `frontend/overseer/src/`]
- [Implement player behavior in `frontend/client/`]
- [Deliver independently verifiable vertical user-story slices]

### Phase 4: Integration and Packaging

- [Verify multi-client synchronization, reconnection, persistence, security, and shutdown]
- [Verify Vite embedding and Wails startup/build behavior]
- [Verify credential-gated public-provider or signed-release behavior only when affected and prerequisites are available]

## Verification Plan

| Surface | Automated check | Interactive/manual check | Expected result |
|---|---|---|---|
| Go domain/services | `task test` | [Focused scenario if needed] | [Result] |
| Concurrent runtime | `task test:race` when affected | [Stress/reconnect scenario] | [Result] |
| Go quality | `task fmt:check`, `task vet`, and `task lint` | N/A | Formatting, vet, and repository lint succeed |
| Overseer frontend | `npm ci --prefix frontend` and `npm run build --prefix frontend` | `task dev` + [Overseer journey] | [Result] |
| Player browser(s) | `npm ci --prefix tests/browser` and `npm test --prefix tests/browser` when affected | [Multi-client/audio/reconnect journey] | [Result] |
| Package/release candidate | `task package` and `task release:local` when affected | [Packaged target smoke] | [Result] |
| Signed macOS distribution/public provider | [`task release:macos:preflight` or N/A] | [Credential-dependent journey] | [Result or explicitly unavailable] |

## Project-Specific Complexity Factors

- Concurrent lifecycle and shutdown across Wails, persistence workers, HTTP/ConnectRPC clients, and optional embedded public endpoints
- Server-authoritative state projections shared across Overseer and player presentation surfaces
- Backward-compatible user-owned JSON files and cross-file player-configuration references
- Browser identity, multi-tab recognition, controller authority, revisions, and reconnect convergence
- Cross-platform application embedding and packaging, unsigned tagged releases, and optional manual macOS signing/notarization requirements

## Complexity Tracking

> Fill only when a Constitution Check violation requires justification.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| [Constitution rule] | [Concrete need] | [Why a compliant approach is insufficient] |
