# Implementation Plan: Windows and Linux Desktop Support

## Summary

Add complete Wails desktop distributions for `windows/arm64`, `windows/amd64`, `linux/arm64`, and `linux/amd64` while preserving the existing macOS arm64 package. A pinned Go Task v3 command graph will replace Make-based project automation, interoperate with Wails’ Task-backed build entrypoints, and delegate deterministic archive and verification logic to the repository-owned Go build package. Matching native runners verify binary identity, resources, metadata, and a launched window, while runtime work replaces macOS-only package detection, paths, close handling, and credentials with explicit platform adapters without changing application, session, player, or public-access contracts.

Local `task package:all` runs on `darwin/arm64`, builds the canonical native macOS application plus the current checkout in architecture-matched Docker containers, verifies the application bundle and four portable archives with their directly accessible `bin/<os>-<arch>/` executable/resource payloads, and publishes the five-target result atomically. Host and Docker failures must retain actionable prerequisite diagnostics, and local Docker static evidence remains distinct from matching-host Windows/Linux launch evidence.

SemVer tag delivery joins an unsigned, SHA-256-verified `darwin/arm64` DMG and the four-target Windows/Linux portable matrix from one tag SHA. Only after all five target outputs and checksums are eligible does the repository-pinned GoReleaser v2 flow publish one GitHub Release and one versioned GHCR artifact; signing credentials, notarization, and Gatekeeper are not part of this release gate, and a failed target or join cannot publish partial success.

**Bugfix**: 2026-08-26 — BUG-001 Updated from bugfix patch.

**Bugfix**: 2026-08-26 — BUG-002 Updated from bugfix patch.

**Bugfix**: 2026-08-26 — BUG-003 Updated from bugfix patch.

**Bugfix**: 2026-08-26 — BUG-004 Updated from bugfix patch.

## Project Structure

```text
Taskfile.yml                                   # canonical Wails-compatible developer/build/package graph
Makefile                                       # tool bootstrap plus non-mutating bootstrap help

tools/
├── task/go.mod                                # github.com/go-task/task/v3/cmd/task v3.53.1
├── task/go.sum                                # Task dependency checksums
└── */go.mod                                   # all existing isolated tools installed by bootstrap

cmd/build/main.go                              # typed target/package/aggregate implementation boundary

internal/buildtool/
├── buildtool.go                              # shared build sequence and macOS compatibility path
├── target.go                                 # supported target/host model and validation
├── preflight.go                              # cross-platform package gates and tool invocation
├── package.go                                # staging, product metadata and package plan
├── archive.go                                # deterministic ZIP/TAR.GZ and file manifests
├── verify.go                                 # PE/ELF, inventory, checksum and path-safety checks
├── aggregate.go                              # authenticated workflow dispatch/wait/download
├── docker.go                                 # local four-target Docker build/verify/publish coordinator
├── buildtool_test.go                         # target, ordering and existing macOS behavior
├── archive_test.go                           # deterministic layout and hostile-path coverage
├── aggregate_test.go                         # four-target completion/failure coordination
└── docker_test.go                            # local payload, atomicity, and Docker diagnostic contracts

build/
├── appicon.png                               # common embedded/runtime icon source
├── docker/Dockerfile.package                 # architecture-matched local portable build image
├── darwin/                                   # existing plist/entitlements/signing inputs, preserved
└── windows/
    ├── app.manifest                          # Windows application/compatibility manifest
    └── info.json                             # product/version metadata for pinned syso generation

main.go                                       # production identity and platform resource root composition
build_profile_development.go                  # development-only resource/env behavior
build_profile_production.go                   # immutable production package identity
resource_roots.go                             # macOS bundle and executable-relative resource policy
production_resources_test.go                  # production profile/resource matrix
wails_host.go                                 # portable close event and platform options
wails_host_test.go                            # exact-once close and option coverage

internal/platform/
├── paths.go                                  # pure storage-profile construction and injected roots
├── paths_darwin.go                           # existing macOS roots
├── paths_windows.go                          # Known Documents/application data roots
├── paths_linux.go                            # XDG documents/config roots and fallbacks
├── paths_test.go                             # redirected, Unicode, spaces and failure cases
├── keychain.go                               # shared secure-store error/secret contract
├── keychain_darwin.go                        # existing Keychain implementation, preserved
├── keychain_windows.go                       # Windows Credential Manager adapter
├── keychain_linux.go                         # freedesktop Secret Service adapter
├── keychain_other.go                         # unsupported targets only
└── keychain_test.go                          # shared fail-closed/error/clearing contract

internal/tunnel/
├── manager.go                                # retain precise secure-store availability state
├── model.go                                  # platform-neutral safe status wording
└── manager_test.go                           # initialization/retry/error-category coverage

.github/workflows/
├── wails-macos.yml                           # existing signed macOS build/release path
└── wails-portable.yml                        # four native jobs plus aggregate publication gate

scripts/
├── verify-windows-package.ps1                # native window/process/runtime smoke wrapper
├── verify-linux-package.sh                   # Xvfb window/process/runtime smoke wrapper
└── dependency-license-check.sh               # delegates target-union validation to build graph

go.mod                                        # direct pinned Windows/Linux credential dependencies
go.sum                                        # resolved dependency checksums
THIRD_PARTY_NOTICES.md                        # notices for every shipped target graph
README.md                                     # target table, commands, prerequisites and data locations
```

**Structure Decision**: Make the root Taskfile the single command-orchestration surface Wails and maintainers call, keep typed build/archive policy in the existing Go build package, reduce Make to tool bootstrap only, keep OS behavior behind `internal/platform` or root Wails composition seams, and isolate the four new release jobs from the established macOS trust path.

## Constitution Check

~~The current Project Identity names macOS 13+ arm64 as the only deployment profile, makes direct Go build commands canonical, and explicitly prohibits Taskfiles. Implementation is gated on a constitution amendment that adds the four approved Windows/Linux targets, authorizes the pinned root Taskfile as the canonical Wails-compatible command graph, confines Make to installing isolated Go tools, and retains detailed build ownership in Go; no production code or release claim may land while that governing text remains contradictory.~~ This prerequisite was satisfied by constitution v6.0.2 and corrected in v6.0.3 to distinguish local `task package:all` Docker evidence from `task package:all:remote` matching-host native evidence.

| Principle | Before Research | After Design | Assessment |
|---|---|---|---|
| I. Govern the Accepted Desktop Runtime | ~~PASS with amendment prerequisite~~ PASS | ~~PASS with amendment prerequisite~~ PASS | The pinned Wails v3 runtime, one root composition, typed Go build policy, Wails-compatible Task graph, platform adapter boundary, and tunnel lifecycle remain authoritative. Constitution v6.0.3 governs local and remote aggregate packaging separately. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | PASS | No application-owned structured contract changes. Target manifests, archive metadata, and CI workflow inputs are explicitly excluded build metadata. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | PASS | Player RPCs, canonical state ownership, unary mutations, server streams, and public-access routing do not change across desktop hosts. |
| IV. Separate Public and Private Capabilities | PASS | PASS | Native dialogs, paths, and secure stores stay private platform adapters; Windows/Linux credential failures remain fail-closed and never introduce a player-facing or plaintext fallback. |
| V. Evolve Schemas Safely and Reproducibly | PASS | PASS | No protobuf schema changes are planned; existing generation and breaking-change gates remain required on every native package path. |
| VI. Preserve Portable Session JSON Version 1 | PASS | PASS | Session JSON and player configuration remain byte-compatible and portable; only their native default directories vary. |
| VII. Complete Cutovers and Remove Superseded Protocols | ~~PASS with amendment prerequisite~~ PASS | ~~PASS with amendment prerequisite~~ PASS | Make workflow aliases are removed in one cutover, the Taskfile becomes the only command graph, typed Go remains the only detailed build/package implementation, and unsupported credential behavior is narrowed to genuinely unsupported targets. |

No complexity exception is required. The governance amendment is complete and is not a waiver.

## Phase 0: Research

The resolved decisions and rejected alternatives are recorded in [research.md](./research.md). The key conclusions are to build and launch-test on native matching runners, use a pinned Task v3 graph as the Wails-compatible command surface over typed Go build policy, bootstrap every isolated Go tool through Make, use the pinned Wails native runtimes, establish compile-time production identity with executable-relative resources, adopt native Windows and Linux secure stores, and preserve macOS as a distinct compatibility path.

## Phase 1: Design and Contracts

The build, artifact, storage, credential, and verification entities and their state transitions are defined in [data-model.md](./data-model.md). Consumer-visible contracts are split into:

- [package-cli.md](./contracts/package-cli.md) for target-specific and aggregate maintainer commands;
- [task-runner.md](./contracts/task-runner.md) for Task migration, Wails interoperability, tool pinning, Make bootstrap, and legacy command parity;
- [artifact-layout.md](./contracts/artifact-layout.md) for stable names, archive inventory, metadata, and checksum evidence;
- [verification-matrix.md](./contracts/verification-matrix.md) for native runner ownership, launch evidence, failure isolation, and aggregate eligibility.

### Implementation Sequence

1. Amend the constitution’s supported deployment profile and command-orchestration rules for `windows` and `linux` on `arm64` and `amd64`: authorize the pinned Taskfile, restrict Make to the all-tool bootstrap, retain typed Go build ownership, and preserve the exact Wails runtime and macOS guarantees.
2. Add `tools/task/go.mod`/`go.sum` pinning `github.com/go-task/task/v3/cmd/task` v3.53.1. Replace the Makefile with one default `tools` target that discovers every `tools/*/go.mod` and runs `go install tool` inside each module, plus non-mutating `help` that points to Task discovery; prove the bootstrap includes Task, Wails, Buf, generators, the linter, and future tool modules without a hard-coded partial list.
3. Create a schema-v3 root `Taskfile.yml` and migrate every current Make workflow with the same inputs, dependencies, ordering, and exit behavior. Define Wails-compatible top-level `build`, `package`, `run`, and `dev` tasks without recursion; make CI, scripts, and README invoke Task and update repository guards that currently reject Taskfiles or require direct Go entrypoints.
4. Add the typed target, host, package, verification, and aggregate-run models to `internal/buildtool`; accept the four exact GOOS/GOARCH pairs from Task/Wails variables and preserve the macOS arm64 package plan behind `task package` with no target override.
5. Move package-critical platform-shell assumptions into portable Go preflight actions while preserving the established order: locked frontend install, protobuf verification, client build, Wails bindings, Overseer build, resource assembly, dependency/license gate, compile, inspect, archive. Retain shell/PowerShell files only as native smoke or thin compatibility helpers called by Task.
6. Add deterministic target staging and archive writers, Windows GUI/product/icon resource generation through the pinned Wails CLI, Linux executable/icon metadata, per-file manifests, archive checksum sidecars, and PE/ELF plus exact-inventory verification. A failed stage must remove or quarantine incomplete output and never emit a success artifact.
7. Replace the macOS bundle-path package heuristic with compile-time development/production profiles. Resolve packaged Windows/Linux resources from the executable directory, retain macOS `Contents/Resources`, keep development checkout resolution, and use the same immutable profile to disable secret-bearing development environment overrides.
8. Refactor platform storage paths through an injectable OS directory provider; preserve macOS defaults, use Windows Known Folders/application data, and use Linux XDG document/config roots with explicit fallbacks. Audit path-dependent tests and startup/assets consumers for native separator, redirect, space, and Unicode behavior.
9. Add Windows Credential Manager and Linux Secret Service implementations behind the existing secret-store contract, with bounded Linux D-Bus operations, precise unavailable/locked/denied/not-found mapping, temporary-buffer clearing, and no insecure fallback. Preserve the first secure-store initialization error through public-access start/retry status.
10. Isolate any Darwin-only window-close fallback behind platform files, keep the common Wails closing hook exact-once, supply Windows/Linux application options and icons, and verify that normal close and startup failure release the player listener, tunnel resources, goroutines, and process.
11. Add `.github/workflows/wails-portable.yml` with four independent native target jobs and one aggregate gate. Each target bootstraps pinned tools, executes `task package GOOS=<os> GOARCH=<arch>`, verifies and launches the extracted artifact, closes it, and uploads only on success; the aggregate job requires exactly four unique verified outputs.
12. Implement `task package:all:remote` as a correlated GitHub workflow dispatch/wait/download path for the current clean pushed branch over a Go helper, including authenticated CLI prerequisite, `origin` repository and exact source identity, clear per-target progress, complete-matrix failure semantics, and collision-free local download into `build/dist`.
13. Implement `task package:all` as a hybrid local aggregate on `darwin/arm64`: execute the canonical no-target package plan for the native ad-hoc signed application bundle, then build the four Windows/Linux targets using architecture-matched Linux containers, quarantined per-target exports, complete static verification, and atomic publication of the Darwin bundle plus all four archives and exact `bin/<os>-<arch>/` executable/resource payloads. Preserve host/Docker causes and recovery instructions. Permit repeat runs by retaining an existing owned output until the new matrix is verified, swapping it through a backup inside a same-filesystem sibling work root, and restoring it if final publication fails; reject unsafe or unrecognized existing targets. Keep Docker-built Windows/Linux launch evidence exclusive to matching-host CI.
14. Promote shipped credential modules to direct pins, update target-union license checks and third-party notices, document the Make-to-Task mapping, tool bootstrap, target selection, WebView2 and GTK4/WebKitGTK6/Secret Service prerequisites, launch steps, native data locations, aggregate packaging, and actionable troubleshooting.
15. Run the complete verification matrix only in CI and matching target environments, then confirm the macOS package passes build, launch, resource, reproducibility, DMG inventory, and SHA-256 checks through the migrated Task entrypoints.
16. On SemVer tag pushes, build an unsigned Darwin DMG and four native Windows/Linux packages against the exact tag SHA, quarantine their outputs until a five-target join verifies the Darwin SHA-256 sidecar plus four portable archives/sidecars and aggregate index, then let only the repository-pinned GoReleaser v2 publication step create or update the GitHub Release and versioned GHCR artifact. Preserve prerelease suffix behavior and fail closed before either destination reports partial success. [FR-029, SC-013]

## Verification Strategy

No tests or builds are run locally during this planning work, per the user’s instruction. Implementation evidence is collected by CI and matching target hosts; every Go test that acquires a resource registers `t.Cleanup` immediately, and cleanup contexts derive from `context.WithoutCancel(t.Context())` with a timeout when shutdown can block.

| Surface | Required evidence |
|---|---|
| Target and CLI contract | Table-driven parsing for all four exact targets, unsupported/case-changed inputs, host OS/architecture mismatch, aggregate ref/output validation, macOS no-argument compatibility, nonzero failures, and no stale success artifacts. |
| Task migration and bootstrap | Task schema/version pin, Wails `build`/`package` dispatch without recursion, complete legacy Make-to-Task parity, variable forwarding, cross-platform command syntax, Make containing only the discovery-based all-tool installer plus non-mutating help, every `tools/*/go.mod` installed, and CI using the pinned Task binary. |
| Build ordering and portability | Cross-platform command construction; locked frontend/protobuf/binding/resource/license gates before compile/archive; Task-to-Go ownership boundary; pinned Wails calls; spaces and non-ASCII checkout paths; cancellation and child-process cleanup. |
| Archive and executable identity | Deterministic ZIP/TAR.GZ order, timestamps and modes; exact safe inventory; both demo files and notices; Windows PE machine/subsystem/version/icon; Linux ELF machine/executable mode; manifest hashes and archive sidecars; path traversal and duplicate-entry rejection. |
| Local Docker aggregate | Contract tests with controlled Docker command outcomes for missing executable, stopped daemon, unsupported platform, and per-target failure; exact four-target `bin/<os>-<arch>/` inventory; executable/resource hashes equal the verified archive; no partial output; repeat publication over the default or recognized prior aggregate through same-filesystem backup/rollback; rejection of files, symlinks, roots, and unrecognized custom directories. The four-target build itself runs only in CI or an explicitly selected Docker environment, not as a required local test. |
| Local Darwin package join | Host validation rejects every runtime except `darwin/arm64` before mutation; the canonical no-target package plan produces the ad-hoc signed bundle; controlled tree-copy tests preserve exact regular-file inventory and modes, reject links/special files, require executable/plist/resources/signature evidence, report `bin/darwin-arm64/Fallout Terminal.app`, and include it in the same replacement transaction as the four Docker payloads. |
| Tagged five-target release | A tag-scoped macOS runner produces an unsigned DMG and verifies its SHA-256 sidecar; four matching Windows/Linux runners produce eligible portable archives; the join requires one source SHA and exact DMG/archive/checksum/index inventory; GoReleaser v2 and GHCR publication run only after that join, preserve prerelease semantics, and expose no partial success under injected target, checksum, or destination failure. |
| Production identity and resources | Development checkout behavior; production `.app`, `.exe`, and ELF layouts; launch from an unrelated working directory; immutable environment-override gating; missing/corrupt resource failure; bundled demo loading. |
| Platform storage and desktop adapters | Redirected and unavailable native roots, XDG fallbacks, Windows Known Folders, separators, Unicode and spaces; JSON dialog filters; HTTP/HTTPS external links; platform options/icon; exact-once close with listener/tunnel/process release. |
| Secure credentials | Shared replace/presence/delete/use semantics; native not-found/locked/denied/unavailable mapping; Linux context timeout and prompt behavior; Windows blob clearing; zero plaintext fallback; redacted logs/status; initialization failure preservation and recovery. |
| Windows native acceptance | On `windows/amd64` and `windows/arm64`, unpack the ZIP, verify WebView2 or report an actionable prerequisite, observe the real Overseer window within 60 seconds, load the bundled demo, close it, and confirm process/listener exit. |
| Linux native acceptance | On `linux/amd64` and `linux/arm64`, unpack the TAR.GZ under Xvfb or a native desktop, verify GTK4/WebKitGTK6 and Secret Service availability or actionable status, observe the real window within 60 seconds, load the demo, close it, and confirm process/listener exit. |
| Workflow and aggregate gate | Clean-checkout native matrix with fail-fast disabled, upload-after-verification, independent target reports, failure injection for each job, exactly four unique names/manifests/checksums, correlated workflow dispatch, wait/download behavior, and no combined artifact on partial success; local Docker contract evidence separately covers payload publication and prerequisite diagnostics. |
| Application parity and macOS regression | Representative Windows and Linux session open/save, one player connection, synchronized control, public-access status, and clean shutdown; existing macOS tests, package path, signing/notary/DMG, launch, resource, license, and reproducibility gates remain green. |
