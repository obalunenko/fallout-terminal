# Contract: Task Runner and Tool Bootstrap

## Ownership boundary

- Root `Taskfile.yml` is the only project workflow-alias graph and uses Task schema version `3`.
- `cmd/build` and `internal/buildtool` remain the typed implementation of build order, target validation, staging, archives, and artifact verification.
- The pinned Wails CLI may dispatch the root `build`, `package`, `run`, and `dev` tasks and may provide low-level generators/tools.
- A Task reached from Wails cannot call the matching high-level Wails wrapper, preventing `wails3 build -> task build -> wails3 build` recursion.
- Owned shell and PowerShell scripts remain implementation helpers, not a second discoverable task graph.

## Pinned Task tool

`tools/task/go.mod` owns exactly this tool:

```text
github.com/go-task/task/v3/cmd/task v3.53.1
```

It follows the same isolated-module rules as `tools/wails`, `tools/buf`, `tools/golangci-lint`, `tools/protoc-gen-go`, and `tools/protoc-gen-connect-go`: an explicit Go version, one tool directive, an exact module version, committed checksums, no dependency leakage into the root application module, and update/drift/license coverage.

## Make bootstrap

The Makefile exposes one default bootstrap target and one non-mutating discovery target:

```text
make tools
make help
```

Behavior:

1. Discover every immediate `tools/*/go.mod` in deterministic path order.
2. Fail if none exist or if a discovered module is malformed.
3. Enter each module and run `go install tool`, which installs every tool directive at that module’s selected version into Go’s configured binary directory.
4. Fail nonzero if any module cannot be downloaded, authenticated, compiled, or installed.
5. Report each module/tool without embedding a separate hard-coded inventory.

The Makefile contains no application build, development, dependency, test, generation, package, release, or Spec Kit alias. `make help` lists the bootstrap targets and directs maintainers to `task --list` without installing anything. `make` and `make tools` have the same bootstrap effect. Tool installation may require network access; normal Task workflows use the installed/pinned tools and do not mutate tool modules.

## Migrated Task surface

| Previous Make command | Canonical Task command | Preserved inputs/behavior |
|---|---|---|
| `make help` | `make help`, then `task --list` | Bootstrap help remains available; Task lists all project workflows. |
| `make dev` | `task dev` | `APP_ARGS`; complete development application. |
| `make run` | `task run` | `APP_ARGS`; complete built application. |
| `make prepare` | `task prepare` | Protobuf, player, bindings, Overseer assets in existing order. |
| `make build` | `task build` | Current host by default; `GOOS`/`GOARCH` when explicitly supported. |
| `make package` | `task package` | Existing macOS default plus the four explicit portable targets. |
| — | `task package:all [OUTPUT=<directory>]` | Dispatch the current clean pushed branch, wait for, verify, and download the complete four-target matrix. |
| `make deps` | `task deps` | Locked frontend and browser dependencies. |
| `make deps-frontend` | `task deps:frontend` | Locked frontend workspace install. |
| `make deps-browser` | `task deps:browser` | Locked browser-test install. |
| `make speckit-install` | `task speckit:install` | Existing pinned Spec Kit/extension version variables. |
| `make speckit-update-check` | `task speckit:update:check` | Read-only update check with existing pins. |
| `make speckit-update-test` | `task speckit:update:test` | Network-free updater regressions. |
| `make fmt` | `task fmt` | Go formatting. |
| `make fmt-check` | `task fmt:check` | Non-mutating formatting gate. |
| `make vet` | `task vet` | Go vet. |
| `make lint` | `task lint` | Pinned golangci-lint. |
| `make test` | `task test` | Go tests. |
| `make test-race` | `task test:race` | Go race tests. |
| `make proto-generate` | `task proto:generate` | Locked dependencies and reviewed revision sync. |
| `make proto-check` | `task proto:check` | Format/lint/build/generation drift. |
| `make proto-breaking` | `task proto:breaking` | Full compatibility fixtures. |
| `make bindings-check` | `task bindings:check` | Deterministic Wails bindings. |
| `make browser-test` | `task browser:test` | Locked browser dependencies, Chromium, journeys. |
| `make check` | `task check` | Same principal quality-gate dependency set. |
| `make release-preflight` | `task release:preflight` | Existing macOS signing/notary prerequisites. |
| `make release` | `task release` | Existing signed/notarized macOS DMG workflow. |

Task dependencies must preserve the current one-time dependency semantics: for example, the combined dependency task installs both npm workspaces, protobuf tasks obtain frontend dependencies before generation/checking, browser acceptance obtains both workspaces, and the quality aggregate fails on any constituent gate.

## Wails compatibility

- `go tool -modfile=tools/wails/go.mod wails3 build [GOOS=... GOARCH=...]` dispatches root `task build`.
- `go tool -modfile=tools/wails/go.mod wails3 package [GOOS=... GOARCH=...]` dispatches root `task package`.
- Direct `task build`/`task package` and Wails-dispatched equivalents reach the same Go plans and produce the same outputs.
- The Taskfile accepts Wails’ `GOOS`, `GOARCH`, build-tag, and passthrough variables only through an allowlisted translation into `cmd/build`; unknown platform/architecture values fail.
- Wails-generated platform Task assets may be incorporated only when pinned/reviewed and must not create a second source of product resource, signing, or archive truth.

## Cutover verification

- Repository checks that previously prohibited Taskfiles are replaced with assertions for the root Taskfile, pinned Task module, recursion safety, and absence of parallel Make workflow targets.
- CI and documentation contain no active `make <workflow>` examples after migration except the `make tools` bootstrap and non-mutating `make help` discovery.
- Existing scripts that call `go run ./cmd/build` directly are either migrated to Task or explicitly retained as lower-level implementation tests; public maintainer guidance uses Task.
- Task graph listing, dependency ordering, variable forwarding, and representative commands are checked on Windows, Linux, and the existing macOS host.
