# Implementation Plan: Windows and Linux Desktop Support

## Summary

Converge the implemented desktop packaging and CI surfaces on constitution v8.0.0: platform support is the availability of a governed unsigned portable archive, while native runtime and operating-system integration journeys remain optional evidence. A separate pull-request/main quality workflow keeps the configured Go, protobuf, frontend, startup, Wails-pin, and binding checks, while a qualifying semantic-version tag alone starts the five-target package matrix. Every release target uses `task package GOOS=<os> GOARCH=<arch>`, Darwin produces an unsigned ZIP containing the application bundle, and the CI-owned `release:publish` Task command invokes repository-pinned GoReleaser to publish exactly five archives to one create-only GitHub Release; final acceptance uses one maintainer-approved live prerelease tag.

## Project Structure

```text
.github/workflows/
├── wails-cross-platform.yml                   # PR/main quality checks only; no release assets
├── wails-portable.yml                         # SemVer-tag preflight, five native packages, GoReleaser publication
└── wails-macos.yml                            # remove superseded duplicate CI and Darwin DMG release-candidate workflow

.goreleaser.yaml                               # retain pinned sole publisher; exactly five prebuilt archives
Taskfile.yml                                   # keep package/package:all; add release:publish; remove remote/local joined release aliases
cmd/build/main.go                              # keep local Docker action; remove remote/join actions; add release checks

internal/buildtool/
├── target.go                                  # exact five supported target pairs and stable archive names
├── package.go                                 # common explicit package plan, including unsigned Darwin bundle staging
├── archive.go                                 # ZIP/TAR.GZ encoding; local checksum output may remain non-release metadata
├── releasecheck.go                            # SemVer, minimal archive, and exact five-file inventory checks
├── local_aggregate.go                         # local-only records shared by package:all Docker implementation
├── docker.go                                  # retained optional local package:all implementation
├── aggregate.go                               # remove remote GitHub Actions dispatch/download implementation
├── release.go                                 # remove joined local DMG release-candidate implementation
└── *_test.go                                  # target/package/release contract and local Docker regression coverage

build/docker/Dockerfile.package                # retain for optional local package:all
tools/goreleaser/                              # retain exact GoReleaser pin
tools/oras/                                    # remove unused GitHub Packages publisher
scripts/tool-modules-check.sh                  # retain GoReleaser, stop requiring ORAS
scripts/proto-check.sh                         # remove the deleted wails-macos workflow reference

internal/platform/
├── portable_release_test.go                   # static non-publishing workflow/GoReleaser contracts
└── startup_test.go                            # retained/removed Task and tool surface assertions

README.md                                      # exact five downloads and simplified CI/recovery guidance
docs/platform-packaging.md                     # quality, tag release, local Docker, and manual recovery procedures
docs/platform-support.md                       # archive-support meaning, prerequisites, optional launch, data guidance
specs/021-windows-linux-support/
├── tasks.md                                   # active unchecked constitution-v8 delta, T068-T099
├── tasks-history.md                           # superseded T001-T067 implementation history
└── history/
    ├── bugs/                                  # patched BUG-001 through BUG-004 historical records
    └── validation-v6.md                       # superseded pre-v8 validation snapshot
```

**Structure Decision**: Keep target and archive policy in the existing Go build boundary, use GitHub Actions only for workflow coordination, retain the local Docker aggregate as an isolated convenience, and keep pinned GoReleaser as the only component allowed to create a GitHub Release.

## Constitution Check

Constitution v8.0.0 governs archive availability as the platform-support boundary, keeps native runtime and operating-system integration journeys optional and non-gating, and retains the split between non-release quality checks, tag-only packaging, single-destination GoReleaser publication, optional local Docker aggregation, and manual partial-release recovery. No exception is required.

| Principle | Before Research | After Design | Assessment |
|---|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | PASS | The five matching build hosts produce the governed archives; native runtime journeys are optional evidence and are not platform-support or completion gates. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | PASS | Workflow, archive, and GoReleaser metadata are tool-native; no application-owned structured contract changes. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | PASS | Player RPC direction, state ownership, and browser behavior are unchanged. |
| IV. Separate Public and Private Capabilities | PASS | PASS | CI simplification neither exposes private desktop capabilities nor changes secret handling. |
| V. Evolve Schemas Safely and Reproducibly | PASS | PASS | No protobuf schema changes are planned; the quality workflow retains Buf formatting/linting, generation drift, breaking-change, generated-code, and binding checks. |
| VI. Preserve Portable Session JSON Version 1 | PASS | PASS | Session persistence remains unchanged; packaged demo resources remain read-only archive contents. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | PASS | Remote aggregation, the joined DMG release, ORAS, and rollback paths are removed with their tests and active documentation while local Docker stays isolated and tagged publication enters the sole Task graph through `release:publish`. |

## Phase 0: Research

The integration and cutover choices are recorded in [research.md](./research.md). The load-bearing findings are that the current native runner labels and Task entrypoint can be reused, GoReleaser v2 remains the sole release publisher, and local Docker aggregation can survive independently after its shared records are separated from the remote coordinator. Static release contracts are proven without a dispatch or publication event; final tagged-release acceptance separately uses the approved live prerelease after local validation and commit.

## Phase 1: Design and Contracts

The workflow entities and recovery states are defined in [data-model.md](./data-model.md). Consumer-visible contracts are defined in:

- [package-cli.md](./contracts/package-cli.md) for the five explicit native package commands, retained local Docker command, removed remote/joined commands, and non-publishing release checks;
- [artifact-layout.md](./contracts/artifact-layout.md) for the exact five release filenames and archive contents;
- [quality-workflow.md](./contracts/quality-workflow.md) for the independent pull-request/main quality gates and zero-publication boundary;
- [verification-matrix.md](./contracts/verification-matrix.md) for SemVer preflight, the matching-runner matrix, minimal release eligibility, GoReleaser publication, and manual recovery;
- [task-runner.md](./contracts/task-runner.md) for retained Task/GoReleaser ownership and removed aggregate/release aliases.

### Implementation Sequence

1. Extend the canonical target model and add repository-owned, network-free release checks. Explicit parsing accepts exactly `windows/amd64`, `windows/arm64`, `linux/amd64`, `linux/arm64`, and `darwin/arm64`; strict SemVer, minimal per-target archive eligibility, and exact five-file publication inventory checks validate only compilation outcome, non-empty archive creation, executable presence, and required-resource presence. Checksum, UI, dialog, credential-store, player, tunnel, and signing evidence remains excluded from tagged-release eligibility.
2. Extend the package plan so `task package GOOS=darwin GOARCH=arm64` builds an unsigned `Fallout Terminal.app` on `macos-15` and archives the complete bundle as `Fallout-Terminal-darwin-arm64.zip` through the common archive boundary without codesign, DMG, notarization, or stapling. Package and archive tests prove that all target outputs exclude user-owned documents, private settings, credentials, plaintext secret fallbacks, and secret-bearing verification records without adding those checks to tagged-release eligibility.
3. Rewrite `.github/workflows/wails-cross-platform.yml` as the sole pull-request/main quality workflow. It runs Go tests and vet, Buf formatting/linting, protobuf generation drift and breaking-change checks, generated-code compilation, clean Overseer and player-client builds, startup contracts, exact Wails/runtime/tool pin checks, and clean Wails binding generation through repository-owned Task commands; it has read-only contents permission and no release or asset-publication step.
4. Replace `.github/workflows/wails-portable.yml` with a tag-only flow. A lightweight preflight validates the candidate `v*` tag and searches all release states, including drafts, to refuse any existing GitHub Release before matrix execution; the five matching native runners then invoke the identical pinned Task package entrypoint and upload only their archive.
5. Make the publication job depend on all five matrix entries, download exactly the five stable archives, run the non-publishing inventory check, repeat the existing-release refusal immediately before publication, and invoke `go tool -modfile=tools/task/go.mod task release:publish`. The Task command invokes `go tool -modfile=tools/goreleaser/go.mod goreleaser release --clean --config .goreleaser.yaml`, so GoReleaser remains the sole publisher while release automation enters through the canonical Task graph. An always-running, read-only failure diagnostic distinguishes no-release failure from partial-release failure: the former reports immediate same-tag rerun, while the latter reports manual deletion followed by same-tag rerun. It performs no delete, replacement, append, or rollback operation.
6. Narrow `.goreleaser.yaml` to five prebuilt `release.extra_files`, disabled generated checksums, `draft: false`, automatic prerelease classification, and no replacement permission. Remove the Darwin DMG, checksum sidecars, aggregate index, raw executables, ORAS/GitHub Packages destination, separate draft-exposure choreography, and multi-destination rollback from the automated release path.
7. Retain `task package:all`, `package-all-docker`, `internal/buildtool/docker.go`, and `build/docker/Dockerfile.package` as local-only convenience code. Move every Docker-retained dependency out of the remote coordinator—including local result/record types, artifact interfaces, target helpers, clone/validation functions, directory verification, constants, and index structures—before removing `internal/buildtool/aggregate.go`. Remove `task package:all:remote`, the remote `package-all` CLI action, `task release:local`, the `release-candidate` CLI action, `internal/buildtool/release.go`, and `tools/oras`. Rewrite `.github/workflows/wails-portable.yml` before deleting `.github/workflows/wails-macos.yml`, and remove the deleted workflow reference from `scripts/proto-check.sh` in the same final cutover.
8. Replace joined-release assertions with static workflow/config checks and table-driven, non-publishing fixtures for accepted/rejected tags, exact/malformed five-file inventories, minimal archive presence, failed targets, existing releases, both no-release and partial-release failure diagnostics, and the `release:publish` Task-to-pinned-GoReleaser ownership chain. Native UI, dialog, secure-store, player, lifecycle, tunnel, and signing checks remain optional, non-gating evidence; unexecuted checks are reported as `NOT RUN` and do not affect feature completion or platform support.
9. Update active README, packaging guidance, and platform-support guidance to distinguish archive availability from optional native evidence, the PR/main quality workflow, tag-only release, optional local Docker aggregate, exact five downloadable archives, unsigned Darwin ZIP, create-only publication, prerequisites, user-data locations, and manual partial-release cleanup. Remove active instructions for the remote aggregate, local joined release, DMG tag assets, checksum assets, ORAS/GitHub Packages, and automated rollback.
10. Run exact local completion commands including `task build` and `task package GOOS=darwin GOARCH=arm64`, then—after all static/local gates pass and the implementation is committed—push one maintainer-approved unused SemVer prerelease tag. Preserve the successful five-archive prerelease as acceptance evidence; a failed partial publication follows the same manual recovery contract.
11. Keep `tasks.md` as the active unchecked implementation delta beginning at `T068`; the superseded `T001` through `T067` record remains in `tasks-history.md` so historical Companion completion events cannot mark new work complete.

## Verification Strategy

| Surface | Required evidence |
|---|---|
| Quality workflow | Static contract proves only pull requests and main pushes trigger it; it covers Go tests, vet, Buf/protobuf checks, both clean frontend builds, startup contracts, Wails pin checks, and binding drift while using no write permission, release command, or release asset upload. |
| Target parsing and host matching | Table-driven Go tests accept exactly the five target pairs, reject aliases and `darwin/amd64`, and require an equal native host OS/architecture. |
| Darwin package | Package-plan/archive tests prove `task package GOOS=darwin GOARCH=arm64` emits one non-empty unsigned ZIP containing the application bundle, executable, metadata, icon, notices, and bundled sessions with no signing or DMG step. |
| Windows/Linux packages | Existing package and archive tests continue proving native executable and required-resource assembly for both architectures. |
| Minimal release checks | Network-free fixtures cover valid and corrupt ZIP/TAR.GZ archives, stable filenames, non-empty archives, missing executable/resources, the exact five-file set, unexpected sidecars/indexes/raw files, and cancellation. |
| Release triggers and preflight | Static workflow tests prove only `v*` tag pushes can enter preflight; strict SemVer fixtures reject malformed versions, and an existing-release fixture prevents every matrix job from starting. |
| Matrix ownership | Exactly five runner/target rows call `task package GOOS=<os> GOARCH=<arch>` with fail-fast disabled and upload only one archive each; no Docker or native journey gate is present. |
| GoReleaser publication | Configuration and workflow tests prove `release:publish` is the sole Task publication entrypoint, it invokes repository-pinned GoReleaser as the sole publisher, only the five archives are inputs, generated checksums are disabled, existing releases are refused twice, and no replacement, append, ORAS, package permission, or deletion command exists. |
| Publication-failure recovery | Controlled fixtures prove that failure without a release reports immediate same-tag rerun, while a partial release reports manual deletion followed by same-tag rerun; repository searches find no automated rollback or release deletion. |
| Local Docker isolation | Focused `PackageAllDocker` tests remain green after every retained result, artifact, target, validation, verifier, constant, and index dependency moves to the local boundary; workflow contracts prove neither CI workflow invokes it. |
| Removed machinery | Repository assertions find no active `package:all:remote`, remote `package-all` action, `release:local`, `release-candidate`, ORAS tool/module, GHCR publication, joined release candidate, tagged DMG path, or active reference to the deleted macOS workflow. |
| Regression gates | The separate quality path runs the required Go, protobuf, frontend, startup, Wails-pin, and binding checks; final validation includes pinned `task lint`, `task build`, and `task package GOOS=darwin GOARCH=arm64`; focused buildtool tests follow `t.Cleanup` ownership and use bounded `context.WithoutCancel(t.Context())` cleanup when shutdown can block. |
| Optional native evidence | Native UI, dialog, player, lifecycle, secure-store, tunnel, and signing checks may be `NOT RUN`; validation records that status honestly without treating it as a feature, quality, platform-support, or release failure. |
| Live prerelease acceptance | After static/local validation and commit, a maintainer-approved unused SemVer prerelease tag runs the real five-target matrix; the resulting GitHub Release is preserved and inspected for the exact five archives, while failures use the documented create-only recovery path. |

## Task-List Handoff

The superseded implementation record is preserved in `tasks-history.md`, patched bug reports and pre-v8 validation are under `history/`, and `tasks.md` now contains the dependency-ordered unchecked constitution-v8 delta from `T068` through live-acceptance task `T099`. Artifact alignment stops before implementation; the revised queue must pass a fresh `$speckit-analyze` review before `$speckit-companion-implement`.
