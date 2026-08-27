# Contributing to Fallout Terminal

Thanks for helping improve Fallout Terminal. This is a hobby and home-use project, so the
contribution process aims to stay lightweight while preserving the application boundaries,
portable session format, and reproducible build.

## Before You Start

Read the documents relevant to your change:

- [README](README.md) for product behavior and development setup
- [Architecture](ARCHITECTURE.md) for runtime structure and module ownership
- [Project constitution](.specify/memory/constitution.md) for normative engineering rules
- [Platform support](docs/platform-support.md) for supported operating systems
- [Build and packaging](docs/platform-packaging.md) for build and release workflows
- [Feature specifications](specs/) for existing behavior and design history

The project does not use a separate ADR directory. Record feature-level decisions in the relevant
Spec Kit specification and plan, and update `ARCHITECTURE.md` or the constitution when a decision
changes project-wide structure or policy.

## Development Requirements

Install the versions documented in the README and dependency manifests:

- Go 1.27.x
- Node.js 26.8.1+ and npm
- Native Wails build dependencies for your operating system
- Python 3.11+ and `uv` when working with Spec Kit

The repository includes `.nvmrc`. NVM users should select the reviewed runtime before running
project tasks:

```bash
nvm use
```

Bootstrap the pinned Go tools and locked Node.js dependencies from the repository root:

```bash
make tools
task deps
```

Use `task --list` to see all supported repository workflows. Prefer these tasks and the
repository-owned build command over ad hoc tool invocations.

## V2 Module and Release Identity

The root application module is exactly `github.com/obalunenko/Fallout-Terminal/v2`. Active
application imports and generated Wails bindings must use that `/v2` identity. Modules under
`tools/` are independently versioned development-tool owners and intentionally retain their
unsuffixed `github.com/obalunenko/Fallout-Terminal/tools/...` identities.

Release identity has one source of truth:

- release tags are strict v2 semantic versions such as `v2.0.0` or `v2.0.0-rc.1`;
- preflight removes only the leading `v` and produces one canonical value such as `2.0.0-rc.1`;
- every native package job receives that value through `VERSION` without re-deriving it;
- Go linker metadata and rendered Darwin or Windows metadata derive from the same value; and
- packaged executables report only the canonical value plus one newline from `--version`, before
  Wails or application services start.

Do not add a hard-coded production version to source files or platform metadata. The checked-in
`build/darwin/Info.plist.tmpl`, `build/windows/info.json.tmpl`, and
`build/windows/app.manifest.tmpl` files are immutable templates; packaging renders target-owned
copies under `build/bin/`. An empty local `VERSION` intentionally produces the non-release identity
`development` with zero-valued native numeric fields.

For release or packaging changes, exercise the governed flow on a matching native host:

```bash
VERSION="$(go run ./cmd/build validate-release-tag --tag v2.0.0-rc.1)"
task package GOOS=darwin GOARCH=arm64 VERSION="$VERSION"
go run ./cmd/build inspect-release-archive \
  --target darwin/arm64 \
  --archive build/dist/Fallout-Terminal-darwin-arm64.zip \
  --version "$VERSION"
```

Use the target matching your host. Inspection must succeed before an archive is eligible for
upload; `development`, missing, malformed, or mismatched versions must fail release inspection.
Publication remains owned by the tag-only GitHub Actions workflow and the pinned GoReleaser tool.

## Choosing the Change Scope

Small, local corrections can be implemented directly. A feature or architectural change should
have a Spec Kit specification that identifies:

- the canonical state owner and mutation authority;
- affected public and private contracts;
- persistence and backward-compatibility behavior;
- frontend and platform consumers;
- startup, shutdown, cancellation, and failure behavior; and
- unit, integration, browser, and native verification.

Keep changes focused. Do not combine unrelated cleanup, dependency upgrades, generated-code drift,
or broad formatting changes with a feature or fix.

## Architecture Boundaries

Preserve these core boundaries:

- Go owns authoritative gameplay, coordination, lifecycle, and durable state.
- `frontend/overseer/` uses only the narrow private Wails bridge and named events.
- `frontend/client/` uses only public player contracts and has no native desktop capabilities.
- `internal/domain/`, `internal/nav/`, `internal/hack/`, `internal/live/`, and
  `internal/control/` remain independent of Wails transport details.
- `internal/player/` adapts public ConnectRPC requests to application services.
- `internal/tunnel/` adds optional protected ingress to the existing player server; it is not a
  second state owner.
- Public and private capabilities remain separate in schemas, handlers, listeners, and tests.

If a change crosses one of these boundaries, update the relevant specification and architecture
documentation in the same contribution.

## Contracts and Generated Code

Application-owned structured contracts originate in `proto/`. Do not manually edit files under:

- `internal/gen/`
- `frontend/client/gen/`
- `frontend/overseer/bindings/`

Regenerate and verify contracts with:

```bash
task proto:generate
task proto:check
task proto:breaking
task bindings:check
```

Protobuf changes must preserve published field numbers and names, use versioned packages, and keep
public player contracts independent of private, persistence, and configuration packages.

## Session Compatibility

Version-1 session JSON is a compatibility contract. Changes must preserve established field names,
compatible unknown fields, and explicit file ownership. Do not switch persistence to protobuf
binary or generic ProtoJSON.

Runtime-only connection, navigation, hacking, and public-access state must not enter the durable
session document unless a specification explicitly evolves the persistence contract.

## Code Style

The root `.editorconfig` defines basic whitespace conventions. In addition:

- Format Go code with `gofmt`.
- Use lowercase Go package names and snake_case Go filenames.
- Use kebab-case for handwritten JavaScript files where a multiword name is needed.
- Use snake_case protobuf filenames under versioned package directories.
- Wrap errors with useful operation context when the underlying error remains relevant.
- Use `github.com/obalunenko/logger` consistently for application logging.
- Never log credentials, player passwords, provider tokens, or unredacted secret-bearing values.
- Keep `//nolint` directives narrow, justified, and accepted by `nolintlint` if that linter is
  enabled in the repository baseline.

The repository-level `.golangci.yml` is the accepted lint policy. Add a new linter only in a focused
change that reviews and resolves its findings rather than suppressing them broadly.

## Go Tests and Resource Cleanup

Register cleanup for every test-owned resource immediately after acquisition with `t.Cleanup`.
Do not use `defer` for test-lifetime resource cleanup.

When cleanup accepts a context, derive it from `context.WithoutCancel(t.Context())`, because the
test context is canceled before cleanup functions run. Add a bounded timeout when shutdown can
block.

Function-scope `defer` remains appropriate for lexical control flow that cannot wait until test
cleanup, such as unlocking a mutex or balancing `WaitGroup.Done`.

Prefer deterministic fakes for networks, clocks, secure stores, native dialogs, and provider
lifecycle behavior. Tests must not depend on a live ngrok account or external service unless they
are explicitly marked and documented as integration tests.

## Verification

Run the checks proportional to the change. The main local quality gate is:

```bash
task check
```

It covers Go formatting, vet, the pinned golangci-lint configuration, race tests, protobuf checks,
generated bindings, and Spec Kit update-checker tests.

Useful focused commands include:

```bash
task fmt:check
task vet
task lint
task test
task test:race
task browser:test
task proto:check
task proto:breaking
task bindings:check
```

Frontend behavior changes should run the relevant Playwright specifications. Platform, lifecycle,
secure-storage, native-window, or packaging changes require the applicable platform-specific build
or smoke evidence described in the platform documentation. Cross-compilation alone is not native
acceptance evidence.

Release-identity changes should also run the focused contract suites before the full gate:

```bash
go test ./internal/buildtool ./internal/version ./cmd/build ./internal/platform
```

For package changes, record the matching-host package command, the executable `--version` output,
the applicable native metadata values, and the archive-inspection result. Never claim an
unavailable platform check as passing evidence.

## Commits and Review

Recent project history uses concise Conventional Commit-style subjects:

```text
feat: add terminal grouping
fix: preserve authored transitions
ci: stabilize hosted native validation
```

Use an imperative subject that explains the outcome. Keep commits reviewable and avoid committing
generated or packaged artifacts unless the repository intentionally tracks them.

Before requesting review, confirm that:

- the change has a clear user-visible or maintenance purpose;
- architecture and specification artifacts match the implementation;
- generated code was produced by pinned tools and has no unexplained drift;
- active application imports and generated bindings use the exact `/v2` module identity;
- public/private and secret boundaries remain intact;
- session compatibility is preserved or explicitly migrated;
- package and release changes preserve the single canonical `VERSION` and pass matching-host
  release inspection;
- tests cover success, rejection, cancellation, and cleanup where relevant;
- the applicable validation commands pass; and
- documentation describes any changed setup, behavior, or platform requirements.

If a check cannot be run locally, state which check is missing and why rather than implying the
change received evidence it did not.
