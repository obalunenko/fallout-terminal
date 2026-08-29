# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]

**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `$speckit-plan` command.

## Summary

[Extract the primary requirement and technical approach from the feature spec]

## Technical Context

**Language/Version**: Go 1.27; two independent Vue 3 applications using strict TypeScript
Single-File Components, the Composition API, and `<script setup lang="ts">`; Node.js 26.8.1+ for
build, code generation, type checking, and browser-test tooling only

**Primary Dependencies**: Wails v3.0.0-beta.15, `connectrpc.com/connect` v1.20.0,
`google.golang.org/protobuf` v1.36.11, `@connectrpc/connect`/`@connectrpc/connect-web` 2.1.2,
`@bufbuild/protobuf`/`@bufbuild/protoc-gen-es` 2.13.0, and Vite 8.1.5; Vue 3 at the exact
repository pin is the browser runtime dependency; TypeScript, `@vitejs/plugin-vue`, and `vue-tsc`
at exact repository pins are build and type-check tooling; Playwright 1.62.1 owns browser journeys

**Storage**: Versioned local JSON session and player-configuration files; ephemeral live, navigation, hacking, connection, and coordination state in process memory

**Testing**: Colocated Go `*_test.go` tests with deterministic fakes; strict SFC type checks through
the repository `vue-tsc`/typecheck gate; independent Overseer and player Vite builds; Playwright
specs and approved visual snapshots under `tests/browser/`; repository-wide Go lint through
`.golangci.yml` and `task lint`; no numeric coverage threshold

**Target Platform**: Wails desktop application on `darwin/arm64`, `windows/amd64`, `windows/arm64`, `linux/amd64`, and `linux/arm64`, with modern browser clients on the local network or an authenticated public endpoint

**Project Type**: Go modular desktop monolith with two independent Vue 3/Vite application roots:
the privileged Wails Overseer application and the embedded static-HTTP/ConnectRPC public player
application

**Performance Goals**: [Feature-specific responsiveness, synchronization, startup, persistence, or client-count goal]

**Constraints**: Preserve separate `frontend/overseer/` and `frontend/client/` roots, entrypoints,
bundles, dependency graphs, and trust boundaries; keep the player application free of Wails and
native capabilities; keep Wails bindings generator-owned in their supported JavaScript, JSDoc, and
declaration format; generate public browser protobuf contracts as pinned deterministic TypeScript;
preserve server-authoritative shared state, persistent JSON compatibility, governed browser-origin
policy, owned-resource cleanup, and the single-process runtime

**Scale/Scope**: One Overseer desktop process, one active broadcast, and [expected connected player-client count or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research and be re-checked after Phase 1 design.*

- [ ] Runtime ownership remains within `main.go`/`app.go`, the relevant `internal/` packages, `frontend/overseer/src/`, `frontend/client/`, and `sessions/` boundaries.
- [ ] Cross-boundary Wails, protobuf/ConnectRPC, and HTTP asset contracts document producer, consumer, payload, validation, failure, ordering, and reconnect behavior.
- [ ] Shared navigation, hacking, roster, and controller behavior remains server-authoritative and reconnect-safe.
- [ ] Wails method exposure, CSP, external URL handling, player-origin/input checks, and public-access secret protections are preserved where applicable.
- [ ] Session or player-configuration changes define versioning, defaults, references, migration, and backward compatibility.
- [ ] Runtime-only state remains outside persistent JSON unless persistence is explicitly approved.
- [ ] `frontend/overseer/` and `frontend/client/` remain separate Vue application roots, Vite entrypoints, production bundles, dependency graphs, and trust boundaries.
- [ ] The player application has no Wails runtime, generated-binding, filesystem, native-capability, or indirect privileged dependency path.
- [ ] Vue owns every rendered DOM subtree; any imperative Web Audio, focus, measurement, CRT/typewriter timing, hacking-board geometry, or streaming integration is Vue-owned, narrowly bounded, and registers lifecycle cleanup.
- [ ] Strict TypeScript and repository `vue-tsc` SFC checks cover both applications, capability-neutral shared modules, declarations, and generated TypeScript protobuf contracts.
- [ ] Vue, TypeScript, `@vitejs/plugin-vue`, and `vue-tsc` are exact direct pins, all other governed runtime/generator/tool pins remain exact, and the committed lockfile reproduces them.
- [ ] Any legacy-DOM/Vue or JavaScript/TypeScript coexistence is bounded by an approved migration plan with an owner, single ownership per DOM subtree, parity gates, expiry, and removal gates.
- [ ] Final cutover removes legacy bootstrap code, handwritten production JavaScript application modules, `allowJs`/`checkJs`, mixed DOM ownership, temporary shims or switches, broad `any`, `@ts-nocheck`, unexplained suppressions, and assertions used only to bypass type errors.
- [ ] New dependencies or structural changes have a concrete, documented need and reproducible pinning; Pinia, Vue Router, Nuxt, component libraries, and CSS frameworks are not baseline requirements.
- [ ] Verification uses the configured Go, `vue-tsc`, Playwright, visual-snapshot, Vite, Wails, Taskfile, packaging, and CI gates that apply; unavailable conditional checks are reported rather than claimed.
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
wails_updater.go                # Wails/provider application-update adapter
internal/
├── buildtool/                  # Typed build, package, archive, and verification policy
├── domain/                     # Models, JSON codecs, cloning, validation
├── gen/                        # Generated protobuf and ConnectRPC contracts
├── nav/                        # Transport-independent navigation rules
├── hack/                       # Transport-independent hacking rules and wordbank
├── live/                       # Canonical live terminal/navigation/hack state
├── control/                    # Sessions, roster, controller, broadcast coordination
├── session/                    # Session persistence and native file workflow
├── playerconfig/               # Player configuration persistence and references
├── player/                     # HTTP assets, ConnectRPC service, public protocol
├── platform/                   # Wails desktop adapters and supported-platform paths
├── tunnel/                     # Optional embedded public-endpoint lifecycle
├── update/                     # Framework-independent application-update lifecycle
├── version/                    # Canonical embedded application identity
└── testutil/                   # Shared deterministic test fakes and fixtures
frontend/
├── package.json
├── package-lock.json
├── client/
│   ├── package.json
│   ├── vite.config.ts          # Public-player-only Vite build
│   ├── tsconfig.json           # Strict player TypeScript/SFC configuration
│   ├── index.html
│   ├── src/
│   │   ├── main.ts             # Player Vue application entrypoint
│   │   ├── App.vue             # Public player application root
│   │   ├── components/         # Vue-owned player DOM subtrees
│   │   ├── composables/        # Narrow reactive state/lifecycle ownership
│   │   ├── adapters/           # ConnectRPC, streaming, Web Audio boundaries
│   │   ├── directives/         # Focus, measurement, and geometry seams
│   │   ├── env.d.ts            # Player/Vite TypeScript declarations
│   │   └── styles/             # Player CRT presentation styles
│   ├── gen/                    # Generated TypeScript protobuf/Connect contracts
│   └── sounds/                 # Static browser audio assets
└── overseer/
    ├── package.json
    ├── vite.config.ts          # Privileged Overseer-only Vite build
    ├── tsconfig.json           # Strict Overseer TypeScript/SFC configuration
    ├── index.html
    ├── bindings/               # Generator-owned Wails JS/JSDoc/declaration output
    └── src/
        ├── main.ts             # Overseer Vue application entrypoint
        ├── App.vue             # Privileged Overseer application root
        ├── components/         # Vue-owned Overseer DOM subtrees
        ├── composables/        # Narrow reactive state/lifecycle ownership
        ├── adapters/           # Typed Wails binding and named-event adapters
        ├── directives/         # Focus, measurement, and timing seams
        ├── env.d.ts            # Overseer/Vite/Wails TypeScript declarations
        └── styles/             # Overseer CRT presentation styles
tests/browser/
├── *.spec.mjs                  # Playwright player journeys
├── *-snapshots/                # Approved visual snapshot parity baselines
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

[Document bound method/event names, directions, protobuf-defined payloads, typed Overseer adapters,
public projections, validation, errors, readiness, and shutdown behavior, or N/A. Keep Wails-generated
JavaScript, JSDoc, and declaration files generator-owned and unedited.]

### Protobuf/ConnectRPC and HTTP Assets

[Document RPC methods/cardinality, pinned deterministic generated TypeScript messages and clients,
static routes, directions, origin and size checks, server validation, authorization, revisions,
publications, action results, and reconnect state, or N/A]

### Runtime-State Lifecycle

[Describe creation, mutation, publication, clearing, reconnection, and the persistence boundary for live and coordination state, or N/A]

### Platform, Tunnel, and Packaging

[Describe supported-platform paths/dialogs, embedded provider resources, OS secure-store secrets, temporary material, embedding, build, or release implications, or N/A]

## Implementation Phases

### Phase 0: Research and Decisions

- [Resolve actual unknowns; omit generic research]
- [Confirm platform, protocol, persistence, concurrency, or compatibility decisions]
- [Choose a new dependency only when existing tools cannot satisfy a documented need]
- [For migration work, inventory legacy DOM/JavaScript ownership and define bounded Vue ownership,
  parity, visual snapshot, expiry, and removal gates]

### Phase 1: Contracts and Data Design

- [Define persistent JSON, Wails, protobuf/ConnectRPC, and HTTP asset contracts as applicable]
- [Define validation, public projection, ordering, compatibility, and reconnection behavior]
- [Map producer and consumer changes to exact Go paths, Vue SFCs, composables, adapters, directives,
  generated TypeScript contracts, and generator-owned Wails bindings]
- [Re-run the Constitution Check after design]

### Phase 2: Domain, Persistence, and Transport Foundations

- [Implement pure models/rules in `internal/domain/`, `internal/nav/`, or `internal/hack/`]
- [Implement canonical state and coordination in `internal/live/` or `internal/control/`]
- [Implement persistence in `internal/session/` or `internal/playerconfig/`]
- [Implement static HTTP and ConnectRPC behavior in `internal/player/`]

### Phase 3: Desktop and Presentation Integration

- [Wire services or privileged commands/events in `main.go` and `app.go`]
- [Implement Overseer behavior through `frontend/overseer/src/main.ts`, `App.vue`, and Vue-owned SFCs]
- [Implement player behavior through `frontend/client/src/main.ts`, `App.vue`, and Vue-owned SFCs
  without Wails or native-capability dependencies]
- [Keep imperative browser integrations narrow, Vue-owned, and lifecycle-cleaned]
- [Deliver independently verifiable vertical user-story slices]

### Phase 4: Integration and Packaging

- [Verify multi-client synchronization, reconnection, persistence, security, and shutdown]
- [Verify strict SFC type checking, both independent Vite builds, Playwright journeys, approved
  visual snapshot parity, Vite embedding, and Wails startup/build behavior]
- [For final migration cutover, verify removal of legacy bootstrap, handwritten production
  JavaScript application modules, `allowJs`/`checkJs`, mixed DOM ownership, temporary shims, and
  prohibited type-system escape hatches]
- [Verify credential-gated public-provider or signed-release behavior only when affected and prerequisites are available]

## Verification Plan

| Surface | Automated check | Interactive/manual check | Expected result |
|---|---|---|---|
| Go domain/services | `task test` | [Focused scenario if needed] | [Result] |
| Concurrent runtime | `task test:race` when affected | [Stress/reconnect scenario] | [Result] |
| Go quality | `task fmt:check`, `task vet`, and `task lint` | N/A | Formatting, vet, and repository lint succeed |
| Vue frontends and browser parity | `npm ci --prefix frontend`, `npm run typecheck --prefix frontend` (`vue-tsc`), `npm run build:overseer --prefix frontend`, `npm run build:client --prefix frontend`, `npm ci --prefix tests/browser`, and `npm test --prefix tests/browser` when affected | `task dev` + [Overseer, multi-client, audio, reconnect, and visual snapshot parity journeys] | Strict SFC checks, both Vite bundles, Playwright journeys, and approved visual snapshots pass |
| Package/release candidate | `task package` and optional `task package:all` when affected | [Packaged target smoke] | [Result] |
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
