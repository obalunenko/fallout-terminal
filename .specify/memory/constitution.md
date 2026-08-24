<!--
Sync Impact Report
- Version change: 5.1.1 -> 5.2.0
- Modified principles:
  - Principle III (optional browser presentation client-streaming with portable unary fallback)
- Added principles: None
- Added sections: None
- Removed sections: None
- Expanded guidance:
  - Controller-local transient presentation remains non-canonical and must reconcile with
    authoritative server updates
  - Development workflow now recognizes constitution-authorized optional browser client streams
- Follow-up TODOs: None
-->
# Fallout Terminal Constitution

## Project Identity

Fallout Terminal is a desktop application for tabletop RPG Overseers. The native Overseer
interface edits and publishes Fallout-style terminal content, while the embedded Go player server
synchronizes authoritative content and state with browser-based player clients. Saved campaign
state uses the portable version-1 JSON session document; live terminal, navigation, hacking,
connection, startup, and tunnel state is owned by the running application.

The production architecture is a Go 1.27 modular monolith whose sole accepted production desktop
runtime MUST be the repository's exactly pinned Wails v3.0.0-beta.10 implementation. Runtime, CLI,
frontend, and generated-binding versions MUST remain mutually compatible and reproducibly pinned
in their owning dependency graphs.

The root Go module owns application composition, the trusted desktop bridge, and the embedded
player server. `frontend/` is the single npm workspace: `frontend/overseer/` owns the Vite-built
browser-JavaScript Overseer interface and private Wails bindings, while `frontend/client/` owns the
separately built and embedded browser-JavaScript player interface and public generated contracts.
`internal/` contains application services, domain logic, adapters, and platform integrations.
Node.js is build, code-generation, and browser-test tooling, not an application runtime. The
supported deployment profile is macOS 13+ on Apple Silicon (`arm64`).

The Electron-to-Wails and Wails-v2-to-Wails-v3 migrations are complete. Their rollback and migration
records MUST be preserved only as clearly labeled historical materials. Legacy Electron and Wails
v2 source, behavior, records, and executable digests MUST NOT govern active architecture,
dependencies, compatibility, acceptance, rollback, or release decisions.

## Core Principles

### I. Govern the Accepted Desktop Runtime

- The Wails v3 architecture MUST use explicit application, service, window, event, dialog,
  browser, asset, and lifecycle APIs. Root `main.go` and `app.go` own their composition and also
  compose filesystem-backed persistence, player-server startup, and optional tunnel startup.
- Wails application objects, services, windows, events, dialogs, browser integration, asset
  servers, lifecycle hooks, generated bindings, and `@wailsio/runtime` imports MUST remain platform
  adapters or composition concerns. Domain, control, session, player-configuration, live, and
  player services MUST remain independent of Wails.
- `frontend/overseer/` MUST access privileged desktop operations only through one narrow, explicitly
  registered desktop service and named events. Wails v3 generated bindings and
  `@wailsio/runtime` MAY implement this private transport. They MUST NOT expose a generic
  dispatcher or arbitrary filesystem, process, or environment access.
- `frontend/client/` owns the browser player experience and MUST operate without Wails, native desktop, or
  filesystem APIs and MUST have no path to desktop capabilities.
- `internal/domain/`, `internal/nav/`, `internal/hack/`, `internal/live/`, and
  `internal/control/` own domain, navigation, hacking, live-state, and coordination behavior.
  Their canonical logic MUST remain transport-independent and server-authoritative.
- `internal/session/` and `internal/playerconfig/` own durable JSON storage and native selection
  workflows. `internal/player/` owns the generated player RPC boundary and static asset delivery.
  `internal/platform/` owns desktop adapters and platform paths. `internal/tunnel/` owns the
  optional public-access endpoint/provider lifecycle, credential use, reconfiguration, and
  shutdown whether a provider uses an embedded runtime SDK or an explicitly owned external
  process separately authorized by a feature plan.
- `internal/tunnel/` MUST remain independent of Wails and domain rules. Root composition and
  platform adapters MUST provide UI, secure-storage, and application-lifecycle wiring through
  narrow interfaces.
- A public-access implementation MUST be fail-closed, idempotent, time-bounded, cancellable, and
  testable through deterministic network, provider, and clock fakes. It MUST NOT create a second
  authoritative player server. Local and LAN access MUST remain functional when provider,
  network, account, secure-storage, or public-endpoint operations fail.
- Before publishing a public URL, its complete provider-endpoint authentication policy MUST be
  active. For personal-use sharing, the provider endpoint MAY own Basic Auth and public routing;
  the player application MUST NOT be required to infer public ingress from `RemoteAddr`, forwarding
  headers, or Host. Stop, reconfiguration, and failure paths MUST withdraw the reusable URL and
  close the owned endpoint before publishing a replacement. A separate application deny mutation
  before close is not required when that endpoint is the sole public acceptance boundary.
- Shutdown MUST release embedded endpoints, any owned external processes separately authorized by
  a feature plan, goroutines, listeners, and temporary secret material within the application's
  current shutdown budget.
- `sessions/` contains versioned JSON examples or data, not executable logic.

Changes crossing these boundaries MUST identify every affected producer, consumer, state owner,
contract adapter, and verification surface in the feature specification and implementation plan.
Generated protobuf messages MUST remain application-boundary representations; they MUST NOT become
canonical mutable domain aggregates or acquire domain, navigation, live-state, or hacking logic.

### II. Make Protobuf the Application Contract Source of Truth

Protocol Buffer schemas are the sole source of truth for every application-owned externally
observable or serialized structured contract, including:

- player RPC requests, responses, server streams, events, and public state;
- desktop bridge requests, results, events, and runtime-status DTOs, even while Wails remains the
  private transport;
- public-access settings, commands, redacted statuses, and events;
- every known field of the version-1 session document; and
- serializable application runtime, player-server, startup, and tunnel configuration.

Generated Go and ECMAScript types and explicit boundary adapters MUST implement those contracts.
Application code MUST NOT maintain handwritten duplicates of transport DTOs.

Secret fields MAY exist only in narrow private mutation inputs and the one-time
generated-player-password result permitted by Secret and Credential Governance. Secret fields MUST
NOT appear in public contracts, reusable status or configuration projections, public descriptors,
player payloads, or serialized persistence models.

Third-party and tool-native manifests, schemas, and metadata are outside this rule and MUST NOT be
duplicated in protobuf. This exclusion includes repository Go build orchestration, package
manifests, npm and Go lockfiles, framework-generated binding metadata, Buf configuration, GitHub
Actions workflows, and macOS plist files. The exclusion covers tool orchestration and metadata,
not application-owned structured desktop requests, results, events, runtime statuses, or
serializable configuration values. Non-serializable dependency-injection values, including
`fs.FS`, callbacks, interfaces, application or window objects, and process handles, likewise MUST
remain native implementation values rather than protobuf fields.

Static HTML, CSS, fonts, sounds, images, and other assets MAY use normal HTTP delivery because they
are resources, not RPC contracts. Asset delivery MUST NOT be used to bypass protobuf governance for
structured application messages or state.

### III. Use ConnectRPC and Keep State Server-Authoritative

All network RPC communication MUST use ConnectRPC with code generated from the governed schemas for
Go and ECMAScript. Handwritten JSON wire envelopes, handwritten RPC routers, and duplicated network
transport DTOs are prohibited.

Browser-originated canonical mutations MUST retain unary RPCs as the portable default.
Authoritative live updates MUST use server-streaming RPCs. An optional client-streaming browser
transport MAY carry high-frequency, ephemeral presentation intents only when all of the following
conditions hold:

- The selected deployment proves end-to-end HTTP/2 request-stream support before enabling the
  optimization.
- The RPC and every message remain protobuf-defined and generated.
- A functionally equivalent unary fallback remains available for unsupported browsers, direct LAN
  access, failed capability probes, and interrupted or failed streams.
- Go retains ownership of authoritative state, revisions, validation, authorization, rejection,
  and ordered publication through the server subscription stream.
- Client-stream input is bounded, rate-limited, latest-wins, cancellable, and invalidated when the
  controller, broadcast, terminal, presentation context, or stream generation changes.
- Client-streaming remains an optimization and MUST NOT be required for basic player operation.

Bidirectional-streaming browser request bodies MUST NOT be used until the accepted browser
transport can consume responses while its request body remains open and the deployment proves that
support end to end.

Player navigation, character selection, controller actions, and hacking actions are requests, not
local state changes. Go services MUST validate requests, mutate canonical process state, and publish
detached public projections over server streams. Browser clients MUST converge on authoritative
state and MUST NOT create divergent optimistic-only transitions.

A controller-local transient pointer presentation MAY render before an authoritative round trip
only when it is explicitly non-canonical, visual-only, incapable of granting authority or mutating
shared gameplay, and keyed so older authoritative revisions cannot overwrite a newer local intent.
Shared presentation, observer rendering, and preview audio MUST remain tied to applicable
authoritative updates, and reconciliation MUST NOT replay superseded visual or audio effects.

Contract changes MUST specify RPC direction, message type, validation and rejection behavior,
authorization, ordering or revision semantics, stream reconnection behavior, and compatibility
impact. Transport handlers and generated messages MUST adapt to transport-independent application
services; they MUST NOT own domain rules or authoritative mutable state.

### IV. Separate Public and Private Capabilities

Public player services and private Overseer capabilities MUST remain separate at schema,
service, adapter, listener, and authorization boundaries. The player Connect service and every
public-access endpoint MUST NEVER expose native dialogs, arbitrary file access, external URL
opening, `ForceHackSuccess`, provider credentials, private hacking candidates, passwords, random
outcomes, secret words, or any equivalent trusted capability or secret state.

The private Wails bridge MAY remain the transport for trusted desktop-only operations, but every
structured request, result, event, runtime-status payload, and serializable configuration value
crossing it MUST have a protobuf-defined contract and an explicit adapter. The Overseer frontend
MUST reach privileged operations only through one narrow, explicitly registered desktop service
and named events. Wails v3 generated bindings and `@wailsio/runtime` MAY implement that transport,
but the bridge MUST NOT expose a generic dispatcher, arbitrary filesystem, process, or environment
access, or any player-facing route to desktop capabilities.

Browser-controlled values, file references, runtime commands, and external URLs MUST be validated
again at the privileged Go boundary. Content Security Policy MUST remain restrictive, and external
URL handling MUST allowlist HTTP and HTTPS. Public-access mode MUST fail closed without valid
credentials and supported secure storage. A provider-owned endpoint policy MAY protect every
public static and ConnectRPC request before forwarding while the player service remains unchanged.
That protected endpoint MUST exist before its URL is published, and its URL MUST be withdrawn when
the endpoint is stopped, replaced, or fails. Local and LAN player access MUST continue independently
of public-access availability and MUST NOT be forced through the provider policy.

### V. Evolve Schemas Safely and Reproducibly

- Protobuf APIs MUST use versioned packages.
- Published field numbers MUST remain stable. Removed fields MUST reserve both their numbers and
  names, and field numbers or names MUST NOT be silently reused.
- Every enum MUST define an `UNSPECIFIED` zero value.
- Fields MUST use explicit presence when absence has different meaning from the scalar default.
- Variant payloads MUST use `oneof` rather than parallel optional fields or ad hoc discriminator
  strings.
- Compatible additions MUST follow protobuf evolution rules. A breaking change MUST introduce a
  new versioned package or an explicit, documented migration.

Code generation MUST be deterministic and reproducible. Generator and protobuf runtime versions
MUST be pinned. Generated files MUST NOT be edited manually, and generation MUST produce no
unexplained working-tree drift. Once the repository establishes a protobuf compatibility baseline,
CI MUST run Buf formatting and linting, generation-drift checks, and breaking-change checks against
that baseline.

### VI. Preserve Portable Session JSON Version 1

The existing portable session JSON version 1 remains a compatibility contract. Adding protobuf
schemas MUST NOT switch persistence to protobuf binary or generic ProtoJSON, change established JSON
field names, discard compatible unknown JSON fields, or otherwise make existing version-1 documents
unreadable or lossy. Session persistence MUST use an explicit adapter that maps the protobuf-defined
known fields to the established JSON representation while preserving compatible unknown fields.

Specifications that change the session or player-configuration JSON shape MUST define versioning,
validation, defaults, references, unknown-field preservation, and migration or backward-
compatibility behavior before implementation. Saving MUST remain explicit about the target file and
MUST NOT silently overwrite, relocate, or transform unrelated user data.

On macOS, user-created sessions SHOULD default to `~/Documents/Fallout Terminal/Sessions/` after
explicit confirmation. Bundled samples inside the read-only application bundle MUST be copied only
after an explicit user action. App-managed metadata belongs in
`~/Library/Application Support/com.vaulttec.fallout-terminal/`. Autosave MUST continue targeting the
explicitly selected session path. Runtime-only state MUST NOT enter persistent JSON unless a feature
explicitly changes the persistence contract compatibly.

### VII. Complete Cutovers and Remove Superseded Protocols

A final feature cutover MUST remove its superseded transports, dependencies, generated or
handwritten fixtures, adapters, tests, and active documentation after parity is proven. Historical
records MAY remain when clearly labeled as history and MUST NOT be treated as current operating
instructions or acceptance criteria.

Temporary coexistence MUST have a bounded migration plan, an owner, parity criteria, and a removal
gate. Permanent dual protocols are prohibited unless an explicit, separately specified
compatibility requirement identifies the consumers, duration, verification, and retirement policy.

Completed historical specifications and rollback records MUST retain their original targets and
MUST NOT be rewritten as though they governed the accepted architecture. A new migration MAY use
temporary coexistence only within its bounded plan and MUST remove every superseded runtime,
dependency, configuration path, generated binding, and dual-runtime switch before cutover is
accepted.

## Dependency Rules

- Root composition and `internal/platform/` adapters MAY depend on the repository's exactly pinned
  `github.com/wailsapp/wails/v3` v3.0.0-beta.10 runtime because they are the Wails v3 composition and
  platform boundaries. No other `internal/` package MAY import Wails.
- Protobuf schema modules are upstream contract dependencies. Generated Go and ECMAScript outputs
  MUST depend only on pinned generators and runtimes and MUST be consumed through explicit boundary
  adapters.
- `internal/domain/`, `internal/nav/`, `internal/hack/`, `internal/live/`, `internal/control/`,
  `internal/session/`, `internal/playerconfig/`, and `internal/player/` MUST remain independent of
  Wails. Their existing permitted domain, protobuf-adapter, ConnectRPC, HTTP, and asset dependencies
  remain governed by the package-specific rules below.
- `internal/domain/`, `internal/nav/`, `internal/hack/`, `internal/live/`, and
  `internal/control/` MUST also remain independent of ConnectRPC, HTTP handlers, generated protobuf
  types as mutable state owners, and browser code.
- `internal/session/` and `internal/playerconfig/` MAY depend on domain models and protobuf-defined
  contract types through explicit JSON adapters; protobuf definitions MUST NOT replace the portable
  version-1 JSON persistence format.
- `internal/player/` MAY depend on ConnectRPC, generated Go service code, HTTP asset delivery, and
  narrow application-service interfaces. It MUST NOT depend on the Overseer frontend or expose
  private Overseer services.
- `internal/platform/` contains Wails, OS secure-store, and other platform adapters behind testable
  interfaces. `internal/tunnel/` contains provider-neutral optional public-access lifecycle and MAY
  own an embedded provider SDK or a separately authorized external-process adapter. It MUST NOT
  import Wails or OS credential-store implementations. Neither package owns domain rules, and core
  or domain packages MUST NOT depend on Wails, a provider SDK, Keychain, or another concrete secure
  store. Only serializable application-owned configuration crossing a boundary belongs in
  protobuf.
- `frontend/overseer/` MAY call only the narrow registered desktop service through generated Wails bindings
  and consume named events through `@wailsio/runtime`. Every structured bridge payload MUST
  originate from a protobuf schema and pass through an explicit adapter; a generic dispatch surface
  is prohibited.
- `frontend/client/` MAY use browser APIs, generated ECMAScript Connect clients, server-streaming
  responses, constitution-authorized optional client-streaming presentation intents, and static
  HTTP assets. It MUST NOT depend on Wails, filesystem APIs, private services, or handwritten RPC
  envelopes.
- Repository Go build orchestration, package manifests, plist files, framework-generated binding
  metadata, other third-party tool configuration, and non-serializable injected dependencies MUST
  remain native to their owning tools or language and MUST NOT acquire parallel protobuf
  definitions.
- Go test assertions MUST use `github.com/stretchr/testify/assert` or
  `github.com/stretchr/testify/require`. Tests involving protobuf messages or descriptors MUST use
  `github.com/google/go-cmp/cmp` with the appropriate helpers under
  `google.golang.org/protobuf/testing`. These test-only dependencies MUST remain out of production
  package APIs.
- Every runtime, generator, or build dependency MUST have a concrete need recorded in the plan and
  be pinned reproducibly in its owning production module, isolated Go development-tool module,
  Buf configuration, or appropriate npm lockfile. The `github.com/wailsapp/wails/v3` runtime
  module, isolated `wails3` tool module, `@wailsio/runtime` package, and its Vite plugin subpath MUST
  use mutually compatible exact versions. All owning `go.mod`, `go.sum`, and npm package lockfiles
  MUST be committed. Reproducible builds and CI MUST reject `@latest`, floating prerelease
  versions, uncommitted tool-module resolution, and any unrecorded Go-module, CLI, or frontend-
  runtime version mismatch.
- An exactly pinned embedded provider SDK MAY be a production/runtime dependency in the root Go
  module only when its concrete need, license, compatibility, module graph, reproducibility, and
  removal of any superseded runtime are recorded in the feature plan. The constitution does not
  mandate any vendor or provider; each feature plan MUST justify the selected provider/runtime
  against these boundary, security, dependency, lifecycle, and removal rules.

## Secret and Credential Governance

Provider account tokens, player passwords, and every other public-access secret MUST be stored by
the production application only in a supported OS secure credential store. Secret values MUST NOT
be persisted in session JSON, player-configuration JSON, plain Application Support configuration,
URLs, process arguments, public schemas, runtime statuses, named events, logs, diagnostics,
analytics, or test fixtures. A hard-coded shared developer or provider token, password, or account
credential is prohibited in every shipped artifact.

Non-secret Application Support configuration MAY persist versioned settings, a username or domain,
an enabled preference, and opaque indicators or references that show whether a secret exists. It
MUST NOT persist the secret value. A stored secret MUST NOT be read back through the UI or any
private API; supported operations are limited to presence indication, replacement, and deletion.

A narrow, trusted, Overseer-only mutation request MAY accept a new user-entered secret and pass it to
the privileged Go boundary. A newly and cryptographically generated player password MAY be returned
through a narrow private protobuf-defined result exactly once before or during its initial storage
to support an explicit Copy action. This one-time result MUST NOT permit readback of an existing
secret, publication through a named event, persistence in frontend state or storage, or exposure to
players.

For an embedded provider endpoint, a scoped account token and player username/password MAY cross
transiently from the secure-store callback into the pinned provider SDK solely to authenticate the
Agent and construct the active endpoint policy. The application MUST NOT write provider policy
secrets to a file, environment variable, process argument, reusable DTO, event, status, diagnostic,
fixture, or application-managed persistence. Provider-side handling is an explicit external trust
boundary that the governing feature plan MUST disclose.

Every private request or result that temporarily contains a new secret MUST have the minimum
possible lifetime, redact errors, prohibit logging and serialization outside that call, and have
dedicated leak tests. Public descriptors and player payloads MUST remain entirely secret-free. The
OS secure-store implementation MUST be a platform adapter behind a testable interface; core and
domain packages MUST remain independent of Keychain or any other concrete credential store.

Packaged UX MUST NOT depend on environment variables, command-line secrets, a Terminal session, or
an external provider binary. Narrow environment or test-injection seams MAY exist only for
automation over the single production runtime. The governing feature plan and tests MUST bound
those seams explicitly, and they MUST NOT create a second provider path.

## Go Development Tool Modules

Every Go executable used for repository development, generation, validation, build, packaging, or
release automation MUST be declared and executed as a repository-owned Go tool. This includes Buf,
Wails, protobuf and Connect generators, and any future Go-based command introduced into the
development workflow. Operating-system tools and non-Go tools remain governed by their native
installation and lock mechanisms.

The dependency-free `cmd/build` command and `internal/buildtool` package are the sole owners of the
application build and package graph and its protobuf -> player -> bindings -> Overseer -> native or
package order. Direct `go run ./cmd/build dev|build|package` commands are canonical owned entrypoints
and MUST continue to work without Make.

- Each Go development tool MUST have one independent module at `tools/<tool>/`, containing its own
  `go.mod` and committed `go.sum`. A tool module MUST declare exactly one direct tool command with a
  Go `tool` directive; unrelated tool commands MUST NOT share that module.
- Tool modules MUST pin exact module versions and an explicit Go language version. They MUST NOT
  use pseudo-install scripts, floating versions, `@latest`, or depend on whichever executable is
  first on `PATH`.
- Repository commands MUST invoke a third-party tool through its owning module from the repository
  root, using `go tool -modfile=tools/<tool>/go.mod <command> ...` directly or from the checked-in
  standard-library-only `cmd/build` command. Taskfiles are prohibited. A root `Makefile` MAY exist
  only as an optional thin convenience wrapper over `go run ./cmd/build ...` and existing
  repository scripts. It MAY provide short aliases and sequentially compose already governed
  quality gates, but MUST NOT duplicate compilation, generation, signing, packaging, or runtime-
  lifecycle logic; introduce a second build graph, hidden dependencies, or a separate frontend or
  player server; or own dependency versions. `make dev|build|package` MAY be documented aliases,
  but the corresponding canonical Go commands MUST remain independently functional.
- First-party orchestration in `cmd/build` and `internal/buildtool` MAY live in the root application
  module because it is repository source rather than a separately versioned executable dependency;
  it MUST use only the Go standard library and invoke versioned third-party tools through their
  isolated modules.
- The root application `go.mod` MUST contain production/runtime dependencies only and MUST contain
  no `tool` directive or tool block. It MUST NOT contain a `require`, `replace`, or other module
  entry whose only purpose is to build, install, pin, or execute a development tool. The root
  `go.sum` MUST NOT gain entries solely from resolving development tools.
- A module used by both application code and a development tool MAY appear in the root application
  module only when application packages actually require it at runtime or compile time. Its
  application version is governed independently from the tool module. When a product runtime and
  its CLI share an upstream project, the runtime remains pinned in the application module and the
  CLI remains independently pinned in its `tools/<tool>/` module.
- Running, downloading, tidying, or upgrading a tool through `tools/<tool>/go.mod` MUST NOT modify
  the root `go.mod` or root `go.sum`. Tool checksums and transitive tool dependencies belong only to
  that tool's `go.sum` and module graph.
- Each tool module MUST be tidied, reproducible, and verified independently. A tool-version change
  MUST update that module's `go.mod` and `go.sum`, compatibility research where applicable, every
  coupled runtime/frontend pin, and the generated or acceptance evidence affected by the change.
- CI MUST verify the expected set of tool modules, exact direct tool declarations, committed sums,
  zero `tool` directives and zero tool-only dependency entries in the root `go.mod`, no root module
  drift after tool resolution, and absence of global-install or unqualified Go-tool invocations in
  active scripts and documentation. CI and release automation MAY invoke a canonical Go command or
  a verified thin Make alias, but dependency and version ownership MUST NOT move to Make.

This isolation prevents generator and build dependencies from polluting the product module, makes
the invoked executable part of the repository's reviewed dependency graph, and lets tools evolve
independently without sacrificing deterministic local and CI behavior.

## Testing and Quality Gates

Go code MUST use colocated tests and deterministic fakes at filesystem, dialog, process, provider,
secure-store, random, network, clock, stream, and event boundaries. Concurrency-sensitive code MUST
pass the race detector. Browser journeys use Playwright under `tests/browser/`. No numeric coverage
threshold or repository-wide linter is currently defined; plans MUST choose verification
proportionate to the affected behavior instead of inventing or claiming either gate.

Go tests MUST follow these conventions:

- Ordinary assertions MUST use `github.com/stretchr/testify/assert` when the test can safely
  continue after a failure, or `github.com/stretchr/testify/require` when a failure invalidates the
  remaining test steps. Handwritten equality and error assertions MUST NOT duplicate those helpers.
- Tests MUST be table-driven when multiple cases share setup, execution, and verification and
  differ primarily by inputs or expected outputs. A focused single-case test MAY be used when a
  table would obscure materially different behavior or lifecycle requirements.
- Every test needing a context MUST use `t.Context()` (`testing.T.Context`) as its root test-scoped
  context and derive cancellation, timeout, or values from it. `context.Background()` and
  `context.TODO()` MUST NOT replace `t.Context()` in tests except when the behavior of those exact
  contexts is itself the subject of the test.
- Tests involving protobuf messages or descriptors MUST compare them with
  `github.com/google/go-cmp/cmp` and protobuf-aware helpers under
  `google.golang.org/protobuf/testing`, normally `protocmp.Transform()` or more specific
  `protocmp` options. Applicable message conformance checks MUST use `prototest`. Direct
  `reflect.DeepEqual` or generic Testify equality assertions on protobuf messages are prohibited.

Applicable commands MUST succeed before a change is considered complete:

- `gofmt -l .` produces no Go source paths.
- `go vet ./...` succeeds.
- `go test ./...` succeeds.
- `go test -race ./...` succeeds for changes affecting concurrent runtime, player, live, control,
  session, stream, startup, or tunnel behavior.
- `npm ci --prefix frontend` installs the single locked frontend workspace, and
  `npm run build:overseer --prefix frontend` succeeds for Overseer, bridge, embedding, or packaging
  changes.
- `npm run build:client --prefix frontend` succeeds for player-client, embedding, generated
  ECMAScript, asset, or packaging changes.
- `npm ci --prefix tests/browser` and `npm test --prefix tests/browser` succeed for affected player
  journeys when the required local environment is available.
- `go run ./cmd/build dev` is the canonical owned repository-root development entrypoint and passes
  affected interactive Overseer/client journeys without a separately started frontend or player
  server. A verified thin `make dev` alias MAY invoke it without changing ownership.
- `go tool -modfile=tools/wails/go.mod wails3 generate bindings -clean -d frontend/overseer/bindings ./...` succeeds and produces
  no unexplained working-tree drift; both generated bindings and protobuf generation remain
  deterministic and MUST NOT be edited manually.
- `go run ./cmd/build build` succeeds after both `frontend/overseer/` and `frontend/client/`
  production builds succeed.
- `go run ./cmd/build package` succeeds and
  produces a self-contained macOS Apple Silicon application for packaging-sensitive changes.
- Release candidates pass signing, hardened-runtime, notarization, stapling, DMG, and Gatekeeper
  checks when release credentials are available.

Schema and RPC changes MUST additionally verify:

- Buf formatting and linting;
- deterministic regeneration and a clean generation-drift check;
- Buf breaking-change detection against the established protobuf baseline once that baseline
  exists;
- generated Go and ECMAScript compilation;
- protobuf-aware Go comparisons and applicable message conformance checks;
- unary mutation validation and rejection behavior;
- server-stream ordering, revision handling, cancellation, disconnection, and reconnection;
- public/private service separation and the absence of privileged or secret fields from public
  descriptors and payloads; and
- version-1 JSON round trips, established field names, defaults, and preservation of compatible
  unknown JSON fields for session-contract changes.

Public-access and secret-handling changes MUST additionally verify:

- secure-store locked, denied, and unavailable behavior, including local/LAN fallback;
- dedicated secret-leak scans across persistence, schemas, statuses, events, errors, logs,
  diagnostics, fixtures, process arguments, URLs, and frontend state or storage;
- concurrent start, stop, and reconfiguration races; cancellation and time bounds; idempotence;
  stale-completion rejection; and cleanup of endpoints, owned processes, goroutines, listeners, and
  temporary secret material;
- ConnectRPC server-stream compatibility through public access, protected-endpoint-before-URL
  publication, URL withdrawal plus bounded endpoint close, direct local/LAN continuity, and
  preservation of a single authoritative player server;
- a packaged double-click smoke test that requires neither a Terminal nor an external provider
  binary; and
- real external-service tests only as explicit opt-in checks using user-supplied credentials. When
  credentials or connectivity are unavailable, the result MUST be reported as `NOT RUN`. A
  deterministic fake MUST NOT be presented as evidence of a real public endpoint.

CI MUST invoke Buf and every other Go-based development command through its owning `tools/<tool>`
module. It MUST enforce Buf formatting/linting, generation drift, and generated-code compilation
when protobuf schemas are present, and MUST add the breaking-change gate when a protobuf baseline
exists.
The GitHub Actions workflow MUST continue to enforce its configured Go test, Go vet, frontend clean-
build, player-client clean-build, startup-contract, exact Wails pin-consistency, clean Wails v3
binding-generation, and unsigned arm64 packaging gates. Native-dialog, audio, public-tunnel,
multi-browser, and signed-release checks MAY remain documented manual gates where reliable
automation or credentials are unavailable; unavailable checks MUST be reported, not claimed.

Reviews MUST verify production module boundaries, protobuf contract coverage, schema evolution,
generated-file integrity, persistence compatibility, authoritative synchronization, privileged-
interface exposure, provider-endpoint authentication, secret non-disclosure, secure-store
isolation, macOS storage behavior, local/LAN failure isolation, stale-completion rejection, final
cutover cleanup, and owned-resource shutdown.

## Development Workflow

1. Branch from `develop` into a dedicated feature branch.
2. Specify user-visible behavior, independently testable scenarios, affected public and private
   capabilities, and every application-owned structured contract.
3. Update versioned protobuf schemas first, identifying RPC cardinality, presence, variants, stable
   field numbers, compatibility, and any version-1 JSON adapter impact before implementation.
4. Plan every affected producer, consumer, adapter, state owner, persistence rule, security
   boundary, generated artifact, cutover, rollback of the feature change, parity gate, package
   gate, and dependency-pin consistency gate. A migration that needs temporary coexistence MUST
   identify its owner, expiry, parity criteria, immutable rollback reference, and removal gate.
   Public-access plans MUST also identify provider/runtime selection, secure-store ownership,
   secret lifetime, endpoint authentication ownership, local fallback, and shutdown obligations.
5. Regenerate pinned Go and ECMAScript code deterministically through the isolated
   `tools/<tool>` modules; never edit generated files or install a global Go tool.
6. Implement the smallest coherent vertical slice. Keep generated types at boundaries, domain logic
   transport-independent, canonical browser mutations unary by default, live updates
   server-streamed, and private capabilities outside the public player service. A browser
   client-stream MAY carry only the optional high-frequency presentation intents permitted by
   Principle III and MUST preserve its unary fallback.
7. Run the automated and interactive verification defined in the plan. Go tests MUST follow the
   governed assertion, table-driven, `t.Context()`, and protobuf-comparison conventions. Run all
   applicable Buf, generation-drift, breaking-change, streaming, privilege-separation, and
   session-compatibility gates. Public-access changes MUST also run secure-store failure, secret-
   leak, lifecycle-race, stale-completion, protected-endpoint publication, local-fallback, and
   packaged double-click gates. Record unavailable conditional checks honestly as `NOT RUN`.
8. Prove parity and pass package and rollback gates, then remove superseded transports,
   dependencies, fixtures, tests, and active documentation unless a separate compatibility
   requirement explicitly retains them. A cutover MUST remove every superseded runtime import,
   CLI or configuration path, generated binding, and dual-runtime switch before the replacement
   becomes accepted production architecture.
9. Update README, schema documentation, fixtures, compatibility specifications, and historical
   records when setup, operation, or governed behavior changes. Development, generation, CI,
   packaging, and release commands MUST continue to resolve every Go tool through its checked-in
   isolated module. Direct `go run ./cmd/build dev|build|package` commands MUST remain canonical;
   any Make aliases MUST remain optional thin wrappers and MUST pass their ownership checks.

## Governance

This constitution governs Spec Kit artifacts and feature work in this repository. Amendments MUST
reflect an intentional project decision, document their rationale and migration impact, update the
Sync Impact Report, and increment the version below. Existing behavior is evidence, but an
accidental inconsistency, a legacy Electron implementation, or a superseded transport does not
automatically become a standard.

Constitution versions follow semantic versioning: MAJOR for backward-incompatible governance,
principle removal, or principle redefinition; MINOR for a new principle or materially expanded
guidance that remains backward-compatible; and PATCH for non-semantic clarification or correction.
The original ratification date MUST remain unchanged. The Last Amended date MUST record the date of
the adopted amendment.

Constitution checks are required during specification, planning, after design, and at final review.
Every plan MUST identify applicable contract, generation, compatibility, public/private boundary,
secret-handling, provider-lifecycle, build-ownership, and cutover gates. Any violation MUST be
listed in the plan's Complexity Tracking table with a concrete rationale, an owner, a bounded
duration, and a rejected simpler alternative. Reviewers MUST reject unrecorded exceptions,
manually edited generated files, schema-breaking field reuse, public capability or secret leakage,
stored-secret readback, generic bridge dispatchers, Make-owned build graphs, and permanent dual
protocols without an explicit compatibility requirement.

**Version**: 5.2.0 | **Ratified**: 2026-08-09 | **Last Amended**: 2026-08-24
