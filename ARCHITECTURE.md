# Fallout Terminal Architecture

This document describes the current production architecture of Fallout Terminal. It is an
orientation guide for contributors: the project constitution remains the normative source for
architecture and engineering policy, while feature specifications record the decisions and
acceptance criteria for individual changes.

Related documents:

- [Project constitution](.specify/memory/constitution.md)
- [Platform support](docs/platform-support.md)
- [Build and packaging](docs/platform-packaging.md)
- [Feature specifications](specs/)

## System Overview

Fallout Terminal is a Go modular monolith packaged as a native Wails desktop application. The
same process hosts the trusted Overseer interface, application services, an embedded player HTTP
server, and optional public ingress. Players use a separate browser application served by that
embedded server.

```mermaid
flowchart LR
    O[Overseer] --> OW[Wails desktop window]
    OW --> DB[Private desktop bridge]
    DB --> APP[Go application]

    P[Players] --> HTTP[Embedded HTTP and ConnectRPC server]
    HTTP --> APP

    APP --> CORE[Domain and application services]
    CORE --> SESSION[Session and player-config JSON]
    CORE --> KEYCHAIN[Platform secure credential store]
    CORE --> TUNNEL[Optional public-access manager]
    TUNNEL --> NGROK[Protected ngrok endpoint]
    NGROK --> HTTP

    PROTO[Versioned protobuf schemas] -. generate .-> DB
    PROTO -. generate .-> HTTP
```

The application has one authoritative process state. The desktop and player frontends are views
and command sources; neither frontend owns canonical shared gameplay state.

## Repository Structure

| Path | Responsibility |
|---|---|
| `/` | Application composition, Wails lifecycle, embedded assets, and the trusted desktop bridge |
| `internal/domain/` | Canonical domain models, validation, and stable JSON-facing representation |
| `internal/nav/` | Terminal navigation rules |
| `internal/hack/` | Hacking rules and word-bank behavior |
| `internal/live/` | Authoritative live terminal and hacking state |
| `internal/control/` | Coordination transactions, logical sessions, roster, controller authority, and ordered mutations |
| `internal/session/` | Versioned session persistence and durable command-state mutations |
| `internal/playerconfig/` | Player configuration persistence and selection workflows |
| `internal/player/` | Player assets, HTTP server, ConnectRPC adapters, subscriptions, and update delivery |
| `internal/tunnel/` | Public-access settings, secrets, ingress policy, ngrok lifecycle, and redaction |
| `internal/platform/` | Native dialogs, platform paths, secure credential stores, and OS-specific adapters |
| `internal/buildtool/` | Reproducible build, package, archive, and release orchestration |
| `internal/gen/` | Generated Go protobuf and ConnectRPC code; never edit manually |
| `frontend/overseer/` | Trusted Wails-hosted Overseer application |
| `frontend/client/` | Untrusted browser player application |
| `proto/` | Source-of-truth protobuf contracts and Buf configuration |
| `cmd/build/` | Repository-owned build and packaging command |
| `cmd/native-credential-smoke/` | Native secure-storage smoke-test command |
| `tests/browser/` | Playwright acceptance tests and the browser-test fixture server |
| `tools/` | Isolated Go modules that pin repository build tools |
| `scripts/` | Reproducible generation, validation, packaging, and smoke-test entry points |

Directories under `specs/` describe feature intent and history. They are not runtime modules.

## Runtime Composition

`main.go` creates the Wails application, embeds both built frontends, and calls the root
composition function. Composition constructs the services and adapters explicitly:

```text
Wails host
  -> platform adapters
  -> session and player-configuration services
  -> live-state service
  -> coordination service
  -> player ConnectRPC service and HTTP server
  -> public-access manager
  -> root App lifecycle
```

The root package is the composition boundary. Dependencies are passed through narrow interfaces;
domain and application services do not reach back into Wails globals.

The accepted desktop runtime is the exactly pinned Wails v3 version in `go.mod`. Node.js and Vite
build browser assets and generated bindings, but they are not separate production server
runtimes.

## Dependency Direction

The intended dependency direction is inward toward domain behavior:

```mermaid
flowchart TD
    UI[Overseer and player frontends] --> ADAPTERS[Desktop and network adapters]
    ADAPTERS --> CONTROL[Coordination and application services]
    CONTROL --> LIVE[Live, navigation, and hacking services]
    LIVE --> DOMAIN[Domain models and validation]

    PERSIST[Persistence adapters] --> DOMAIN
    PLATFORM[Platform adapters] --> CONTROL
    ROOT[Root composition] --> UI
    ROOT --> ADAPTERS
    ROOT --> CONTROL
    ROOT --> PERSIST
    ROOT --> PLATFORM
```

Key constraints:

- `internal/domain/` must remain independent of Wails, ConnectRPC, ngrok, and generated transport
  types.
- Navigation, hacking, live-state, and coordination rules remain transport-independent.
- Transport handlers adapt generated messages to application services; handlers do not own domain
  rules or canonical mutable state.
- Wails objects and browser/runtime APIs remain in root composition or platform adapters.
- Public-access code must not become a second authoritative player server.

## Key Runtime Sequences

The diagrams below show ownership and ordering at the major system boundaries. They intentionally
omit implementation-only calls that do not affect authority, durability, or resource lifetime.

### Application Startup and Shutdown

```mermaid
sequenceDiagram
    autonumber
    actor OS
    participant Main as main.go
    participant Wails as Wails host
    participant App as Root App
    participant Player as Player server
    participant Public as Public-access manager
    participant Session as Session service
    participant Desktop as Desktop runtime

    OS->>Main: Launch process
    Main->>Wails: Create application and desktop window
    Main->>App: Compose services and adapters
    Main->>Wails: Register desktop service and lifecycle hooks
    Main->>Wails: Run
    Wails->>App: Startup callback
    App->>Player: Start(startup context)
    Player-->>App: Local and LAN server information
    App->>Public: Initialize preferences and redacted status
    App-->>Wails: Emit player-server status
    App->>Desktop: Ready(runtime context)
    Desktop-->>App: Desktop ready
    App-->>Wails: Emit public-access status

    alt Host shutdown or process cancellation
        Wails->>App: Shutdown callback
        App->>Public: Shutdown(bounded context)
        Public-->>App: Endpoint closed
        App->>Player: Stop(bounded context)
        Player-->>App: Listener and subscriptions stopped
        App->>Session: Shutdown(bounded context)
        Session-->>App: Persistence worker stopped
        App->>Desktop: Close(bounded context)
        Desktop-->>App: Desktop closed
        App-->>Wails: Shutdown complete
    end
```

Construction wires dependencies but does not acquire external resources. Startup owns listeners
and provider endpoints; shutdown releases them through explicit, bounded lifecycle calls.

### Authoritative Player Mutation and Publication

```mermaid
sequenceDiagram
    autonumber
    actor Controller as Controller browser
    actor Observer as Observer browsers
    participant RPC as ConnectRPC handler
    participant Control as Coordination service
    participant Domain as Domain and live services
    participant Durable as Session service
    participant Stream as Player subscription stream

    Controller->>RPC: Generated unary request
    RPC->>Control: Adapt and submit request
    Control->>Control: Validate identity, authority, and revision

    alt Request accepted
        Control->>Domain: Apply canonical mutation
        opt Mutation changes durable session state
            Control->>Durable: Persist mutation
            Durable-->>Control: Authoritative document and revision
        end
        Domain-->>Control: Authoritative live state
        Control-->>RPC: Accepted result and current revision
        RPC->>Stream: Publish detached projection
        Stream-->>Controller: Authoritative update
        Stream-->>Observer: Authoritative update
        RPC-->>Controller: Generated RPC response
    else Request rejected
        Control-->>RPC: Rejection and current authoritative state
        RPC-->>Controller: Generated error result
    end
```

The browser does not commit shared state locally. Rejections return enough authoritative context
for the requester to discard stale drafts and converge without a second state owner.

### Trusted Session Update and Autosave

```mermaid
sequenceDiagram
    autonumber
    actor Overseer
    participant UI as Overseer frontend
    participant Bridge as Private Wails bridge
    participant App as Root App
    participant Session as Session service
    participant Storage as JSON storage adapter
    participant FS as Selected session file

    Overseer->>UI: Apply authored change
    UI->>Bridge: Generated private request
    Bridge->>App: Validated command payload
    App->>Session: Save(candidate, expected revision)
    Session->>Session: Validate and serialize known fields
    Session->>Storage: Write while preserving compatible unknown fields
    Storage->>FS: Atomic file replacement
    FS-->>Storage: Durable write complete
    Storage-->>Session: Success
    Session-->>App: Authoritative document and revision
    App-->>UI: Session-state event
    Bridge-->>UI: Command result
    UI-->>Overseer: Render saved authoritative state
```

The selected path remains the autosave target. A successful UI result follows the session service's
durable mutation result rather than an optimistic frontend-only edit.

### Public Access Start and Stop

```mermaid
sequenceDiagram
    autonumber
    actor Overseer
    participant UI as Overseer frontend
    participant App as Root App
    participant Manager as Public-access manager
    participant Settings as Preferences and secret stores
    participant Ingress as Loopback admission boundary
    participant Provider as Provider tunnel
    participant Player as Existing player server

    Overseer->>UI: Enable public access
    UI->>App: Start(expected revision)
    App->>Manager: Start(bounded context, revision)
    Manager->>Settings: Load preferences and credential
    Settings-->>Manager: Configuration and secret
    Manager->>Ingress: Start deny-all proxy to local server
    Ingress->>Player: Configure loopback forwarding
    Manager->>Provider: Create tunnel to private ingress URL
    Provider-->>Manager: Canonical public URL and host
    Manager->>Ingress: Activate expected host and Basic Auth
    Ingress-->>Manager: Admission policy active
    Manager-->>App: Publish redacted running snapshot
    App-->>UI: Public-access status event
    UI-->>Overseer: Display reusable URL without secrets

    Overseer->>UI: Disable public access
    UI->>App: Stop(expected revision)
    App->>Manager: Stop(bounded context, revision)
    Manager->>Ingress: Deny new requests
    Manager->>Provider: Close owned endpoint
    Provider-->>Manager: Endpoint closed
    Manager->>Ingress: Close loopback proxy
    Ingress-->>Manager: Ingress closed
    Manager-->>App: Publish disabled snapshot
    App-->>UI: Public-access status event
```

The public URL is published only after its authentication policy is active and is withdrawn when
the endpoint stops or fails. The provider forwards to the existing player server and never becomes
an independent state owner.

## Frontend Boundaries

### Overseer

`frontend/overseer/` is the trusted native interface. It may invoke only the narrow desktop
service registered by the Go application and consume named application events. Wails-generated
bindings implement that private transport boundary.

The desktop bridge must not expose a generic dispatcher or arbitrary filesystem, process, or
environment access. Values received from the frontend are validated again at the privileged Go
boundary.

### Player

`frontend/client/` is a standalone browser application. It communicates with the embedded player
server through generated ConnectRPC clients and receives authoritative updates through server
streams. It has no Wails, native desktop, filesystem, credential, or private Overseer capability.

Player actions are requests. Go validates authority and revisions, applies accepted mutations,
and publishes the resulting state. The browser converges on that published state rather than
maintaining an independent canonical model.

## Contracts and Generated Code

Application-owned structured boundaries originate in versioned protobuf schemas under
`proto/fallout/terminal/`:

- `player/v1/` defines the public player service and state.
- `private/v1/` defines trusted desktop requests, results, events, and runtime projections.
- `persistence/v1/` defines known session and player-configuration fields.
- `config/v1/` defines serializable runtime and public-access configuration.

Buf generates Go code into `internal/gen/` and ECMAScript player contracts into
`frontend/client/gen/`. Wails generates private desktop bindings into
`frontend/overseer/bindings/`.

Generated files are build products checked for deterministic drift. Change the source schema or
registered Go service, run the repository generation task, and review the generated result; do not
patch generated files directly.

Public and private contracts stay separate at schema, handler, listener, and authorization
boundaries. Public player routes must never expose native dialogs, file access, provider
credentials, private hacking data, or trusted control operations.

## State and Persistence

The system has several explicit state owners:

| State | Owner | Lifetime |
|---|---|---|
| Session document, facility, and durable command state | `internal/session/` | Versioned JSON file |
| Player configuration | `internal/playerconfig/` | Versioned JSON file |
| Live terminal and hacking state | `internal/live/` | Process lifetime |
| Logical sessions, roster, controller, and coordination revisions | `internal/control/` | Process lifetime, with explicit durable seams |
| Public-access preferences | `internal/tunnel/` settings store | Application-support JSON |
| Public-access credentials | Platform secure credential store | OS-managed secret storage |
| Server, startup, and lifecycle status | Root `App` | Process lifetime |

Session JSON version 1 is a compatibility contract. Persistence adapters map known protobuf-owned
fields to the established JSON representation while preserving compatible unknown fields. The
project does not use a relational database or ORM.

Autosave targets the explicitly selected session file and uses the session service as the durable
mutation owner. Runtime-only connection, navigation, hacking, and tunnel state does not enter the
session document unless a feature explicitly evolves that contract.

### Facility Authority and Revisions

The optional facility aggregate is session-wide. `internal/session/` owns its durable JSON value
and performs atomic world-action, authoring, recovery, and reset writes. `internal/control/` holds
one detached process snapshot outside `LiveBroadcast` and serializes every operation against it.
`internal/live/` is a pure projection service; terminals, terminal groups, and frontends do not own
or independently mutate device or diagnostic state.

Three revisions describe different boundaries and must not be substituted for one another:

| Revision | Meaning |
|---|---|
| Session revision | Guards replacement of the complete durable session document. |
| Facility revision | Guards the facility graph and current device/condition values inside that document. |
| Coordination revision | Orders accepted process-runtime changes and complete master/player publications. |

A facility write returns the resulting session and facility revisions. The coordinator installs
that canonical result and then advances its own revision. No-op and rejected operations retain the
current facility revision; callers compare each expected revision only with its matching owner.

### Facility Transaction Order

1. A facility-backed player command enters the existing private approval lifecycle. Control stores
   a detached action fingerprint, expected source states, and expected facility revision; no world
   state changes at this point.
2. Approval holds the coordinator transaction lock, re-resolves the authored command, and checks
   the current graph, fingerprint, source states, preconditions, and facility revision.
3. The session service evaluates every requested transition against one immutable pre-state, builds
   one candidate containing the command snapshot and all device/condition results, and performs at
   most one atomic document replacement.
4. Control validates the returned detached canonical session before installing its facility and
   command states. It then reprojects all terminal runtimes, repairs navigation, advances the
   coordination revision once, and publishes complete snapshots.

Private authoring, recovery, and reset use the same serialized durability boundary. A stale,
duplicate, invalid, concurrent, or persistence-failed operation returns a typed result and leaves
every affected device and condition unchanged.

### Facility Projection and Lifecycle

Effective terminal presentation is rebuilt deterministically in this order: authored base tree,
completed-command snapshots, matching device-state bindings, then active diagnostic overrides.
EntryContent variants compose by stable block ID. Hidden nodes are omitted, unavailable commands
remain visible with explicit availability, and bounded diagnostic effects are output values only;
projection never mutates facility state or authored input.

Session open, reload, restart restoration, and update handoff replace the coordinator facility
snapshot before resumed publications. An absent facility explicitly clears it for compatible
version-1 sessions. Broadcast stop does not clear the facility. Active and suspended terminal
runtimes retain an authored tree and are reprojected from the current shared snapshot on activation,
reactivation, authored terminal changes, and group moves. Group moves never clone or retarget
facility entities, and facility replacement invalidates pending actions captured from an older
graph.

### Audit Ownership

Control emits the authoritative player and facility lifecycle facts, including correlations, typed
outcomes, revisions, sorted safe IDs, and committed previous and resulting states. The player
server emits separate redacted transport-boundary records for recognition, connection, and request
ingress/egress. Root composition maps these closed facts into the retained runtime logger, while
`internal/diagnostics/` owns retention and failure isolation. Audit records exclude authored text,
display names, secrets, and raw errors, and no load, validation, replay, recovery, reset, or
projection path reads them as state.

## Player Server and Publication

`internal/player/` owns one in-process HTTP listener. It serves the built player assets and mounts
the generated ConnectRPC service. Unary RPCs carry portable browser mutations; server streams
publish detached authoritative projections.

The coordination service orders authority checks and mutations. After a successful state change,
the player service publishes an updated projection to connected clients. Delivery queues and
request sizes are bounded by runtime configuration.

Static HTML, CSS, fonts, sounds, and images use ordinary HTTP delivery. Structured application
messages use protobuf and ConnectRPC.

## Public Access

`internal/tunnel/` owns optional public access as an adapter around the existing player server:

1. Load non-secret preferences and credentials from separate stores.
2. Establish a protected provider endpoint and ingress policy.
3. Publish the public URL only after the complete authentication policy is active.
4. Forward accepted requests to the existing local player server.
5. Withdraw the URL and close the owned endpoint on stop, replacement, failure, or shutdown.

Public access is fail-closed and optional. Provider, network, account, or secure-storage failure
must not prevent local and LAN play. Status projections are redacted and never contain reusable
credentials.

## Lifecycle and Concurrency

The root `App` owns startup and shutdown ordering. Resource acquisition occurs during startup, not
construction. Services that own listeners, endpoints, workers, or platform resources expose
explicit shutdown boundaries and use bounded contexts where shutdown can block.

Tests register resource cleanup immediately with `t.Cleanup`. Cleanup operations that accept a
context derive it from `context.WithoutCancel(t.Context())` and add a bounded timeout when needed,
because the test context is canceled before cleanup callbacks run.

## Build and Deployment

The repository supports native packages for:

- macOS 13+ on Apple Silicon (`darwin/arm64`)
- Windows 10/11 (`windows/amd64`)
- Windows 11 on ARM (`windows/arm64`)
- Linux on the documented GTK4/WebKitGTK baseline (`linux/amd64` and `linux/arm64`)

The root `Taskfile.yml` is the development and verification interface. `Makefile` bootstraps the
pinned tools from isolated modules under `tools/`. The repository-owned `cmd/build` command
prepares protobufs, frontends, Wails bindings, native binaries, platform packages, and release
artifacts.

GitHub Actions separates macOS validation, cross-platform native builds, and portable release
assembly. GoReleaser publishes preverified artifacts; it does not compile the Wails application
for release.

## Verification Strategy

| Surface | Verification |
|---|---|
| Domain and application services | Colocated Go unit tests |
| Resource ownership and concurrency | Go race tests and lifecycle-focused tests |
| Public and private adapters | Go handler, contract, platform, and integration tests |
| Browser behavior | Playwright journeys under `tests/browser/` |
| Protobuf evolution | Buf formatting, linting, generation-drift, and breaking-change checks |
| Wails bridge | Deterministic binding and public-surface checks |
| Packaging | Platform package verification and native smoke tests |
| Repository policy | Tool-module, secret-leak, dependency-license, and cutover checks |

The root `.golangci.yml` explicitly defines the accepted Go linter baseline used by `task lint`.
The primary local quality gate is `task check`. Platform-specific packaging and native UI tests
add evidence that cannot be supplied by ordinary unit tests or cross-compilation.

## Changing the Architecture

Architecture-affecting work starts with a feature specification. A change should identify:

1. The canonical state owner and mutation authority.
2. Every producer, consumer, adapter, and generated contract affected.
3. Public versus private capability impact.
4. Persistence and backward-compatibility behavior.
5. Startup, shutdown, cancellation, and failure behavior.
6. Verification at unit, integration, browser, and platform boundaries.

When a migration completes, remove superseded active transports, dependencies, fixtures, and
instructions. Historical migration records may remain when they are clearly labeled and cannot be
mistaken for the current architecture.
