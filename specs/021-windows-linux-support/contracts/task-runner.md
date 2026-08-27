# Contract: Task Runner and Release Ownership

## Ownership boundary

- Root `Taskfile.yml` remains the only developer and CI workflow-alias graph.
- `cmd/build` and `internal/buildtool` own typed targets, preparation order, staging, archives, local Docker aggregation, and network-free release checks.
- GitHub Actions owns trigger, runner, dependency, temporary artifact, and permission coordination.
- Repository-pinned GoReleaser is the sole GitHub Release publisher.
- Root `Makefile` remains limited to isolated Go-tool bootstrap and non-mutating help.

## Retained package surface

| Command | Behavior |
|---|---|
| `task package GOOS=windows GOARCH=amd64` | Build the Windows amd64 portable ZIP on a matching host. |
| `task package GOOS=windows GOARCH=arm64` | Build the Windows arm64 portable ZIP on a matching host. |
| `task package GOOS=linux GOARCH=amd64` | Build the Linux amd64 portable TAR.GZ on a matching host. |
| `task package GOOS=linux GOARCH=arm64` | Build the Linux arm64 portable TAR.GZ on a matching host. |
| `task package GOOS=darwin GOARCH=arm64` | Build the unsigned Darwin arm64 ZIP containing the application bundle on a matching host. |
| `task package` | Preserve current-host developer package behavior; tagged releases do not use this implicit path. |
| `task package:all [OUTPUT=<directory>]` | Preserve the optional local Darwin-plus-Docker aggregate; never used by CI or tags. |
| `task release:publish` | CI-owned tagged-publication entrypoint; invoke pinned GoReleaser for the already validated five-archive inventory. |
| `task --list` | Discover supported developer workflows and explicit package inputs. |

`GOOS` and `GOARCH` must be supplied together. Unknown inputs and unsupported pairs fail rather than falling back to the host.

The optional manual `release:macos:preflight` and `release:macos:signed` commands may remain for maintainer-initiated distribution outside automated tags. Neither CI workflow invokes them, and their DMG/signing output is not a tagged-release asset.

## Quality surface

The PR/main quality workflow invokes repository-owned tasks or their tested lower-level seams for:

- Go tests;
- Go vet;
- Buf/protobuf formatting, linting, generation drift, generated-code compilation, and breaking-change checks;
- locked frontend dependency installation;
- Overseer and player-client production builds;
- startup contracts;
- exact Wails runtime/CLI/frontend pin consistency; and
- deterministic Wails binding generation.

These tasks publish no release asset and do not invoke a five-target package aggregate.
Repository completion validation additionally runs pinned `task lint`; tagged-release jobs do not.

## Removed aggregate and joined-release aliases

After convergence, the Taskfile exposes none of these commands:

```text
package:all:remote
release:local
```

Their backing `cmd/build package-all` and `cmd/build release-candidate` actions are also removed. `package-all-docker` remains solely because `task package:all` retains it.

Pushing a qualifying tag is the only automated release trigger. After repository-owned checks, the workflow invokes pinned Task's `release:publish` command; that command invokes pinned GoReleaser, which remains the sole GitHub Release publisher.

## Tool ownership

- `tools/task`, `tools/wails`, and `tools/goreleaser` remain exact isolated Go tool modules.
- `tools/oras` is removed because GitHub Packages publication is removed.
- Tool discovery and checks require GoReleaser and no longer require ORAS.
- The root application module remains free of development-tool-only dependencies.
- No global or floating Task, Wails, GoReleaser, or ORAS invocation is accepted.

## Wails compatibility

- `go tool -modfile=tools/wails/go.mod wails3 package GOOS=<os> GOARCH=<arch>` may dispatch the same root package task.
- Direct Task and Wails-dispatched equivalents reach the same Go plan.
- The Task package task never calls the high-level Wails package wrapper, preventing recursion.
- Explicit `darwin/arm64` follows the same target-aware package path as `windows` and `linux`.

## Verification

Repository assertions cover:

- Task schema version and pinned Task invocation;
- paired `GOOS`/`GOARCH` validation and the five exact target pairs;
- retained local `package:all` and `package-all-docker` wiring;
- CI-owned `release:publish` wiring to the isolated pinned GoReleaser module;
- absence of `package:all:remote`, `release:local`, `package-all`, and `release-candidate` remote/joined actions;
- pinned GoReleaser presence and ORAS absence;
- no CI reference to local Docker aggregation or manual signed-macOS commands;
- no GoReleaser replacement or append configuration;
- Wails dispatch without recursion; and
- Make remaining limited to tool bootstrap and discovery help.
