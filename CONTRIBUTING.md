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
- Node.js 20.19+ and npm
- Native Wails build dependencies for your operating system
- Python 3.11+ and `uv` when working with Spec Kit

Bootstrap the pinned Go tools and locked Node.js dependencies from the repository root:

```bash
make tools
task deps
```

Use `task --list` to see all supported repository workflows. Prefer these tasks and the
repository-owned build command over ad hoc tool invocations.

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
- public/private and secret boundaries remain intact;
- session compatibility is preserved or explicitly migrated;
- tests cover success, rejection, cancellation, and cleanup where relevant;
- the applicable validation commands pass; and
- documentation describes any changed setup, behavior, or platform requirements.

If a check cannot be run locally, state which check is missing and why rather than implying the
change received evidence it did not.
