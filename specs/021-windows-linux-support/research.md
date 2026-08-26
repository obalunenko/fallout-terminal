# Phase 0 Research: Windows and Linux Desktop Support

**Bugfix**: 2026-08-26 — BUG-001 clarifies the local Docker payload and diagnostic contract while preserving native release evidence.

**Bugfix**: 2026-08-26 — BUG-003 joins native `darwin/arm64` packaging to the local aggregate without changing the remote release matrix.

**Bugfix**: 2026-08-26 — BUG-004 defines the missing five-target SemVer-tag publication gate and reopens incomplete evidence.

## Native target builders and aggregate orchestration

**Decision**: Package `windows/amd64`, `windows/arm64`, `linux/amd64`, and `linux/arm64` on matching operating-system and architecture runners. `task package:all:remote` resolves the current clean pushed branch in `origin`, dispatches the dedicated GitHub Actions workflow, waits for its four target jobs, and fails unless the complete native matrix succeeds. Local `task package:all` runs only on `darwin/arm64`, reuses the canonical no-target package plan for `Fallout Terminal.app`, additionally builds the current checkout through architecture-matched Docker containers, statically verifies every portable archive, and atomically exposes the Darwin bundle plus four matching Windows/Linux executable/resource payloads under `bin/<os>-<arch>/` without claiming native Windows/Linux launch evidence. Host, Docker installation, stopped daemon, or unsupported platform failures retain their underlying diagnostic and provide actionable recovery. Each target path delegates detailed planning, archive work, and verification to `cmd/build`.

**Rationale**: Wails v3 can cross-compile several combinations, but its production guidance recommends native CI runners for each platform. Linux builds require CGO plus matching GTK and WebKitGTK development libraries, and the feature requires a real-window launch check on the target platform. Native architecture runners make the artifact build and its acceptance evidence one coherent unit. ~~An aggregate CI dispatch is also the only honest interpretation of “package all” when a single local host cannot satisfy the matching-host rule.~~ The remote aggregate is the release-evidence path; the local Docker aggregate is an explicit developer build/static-verification path whose payload/archive equality is useful without pretending to satisfy matching-host acceptance.

**Alternatives considered**: Wails Docker cross-builds remain useful developer tooling but were rejected as release evidence because they cannot provide the required matching-target window check; a local sequential `package-all` was rejected because it could not execute all four matching-host commands; four unrelated manual workflow commands were rejected because they cannot enforce complete-matrix success.

## Repository-owned target and packaging graph

**Decision**: Extend `cmd/build` and `internal/buildtool` with an explicit target value, host compatibility validation, deterministic staging, archive creation, manifest generation, and artifact verification. Make the root Taskfile the canonical command graph and Wails-compatible `build`/`package` entry surface while delegating detailed, testable build plans to Go. Use the pinned Wails CLI for bindings, icons, Windows `.syso` resources, and its supported Task integration. Preserve the existing macOS package behavior and output path through the migrated `task package` command.

The implementation pin is Wails `v3.0.0-beta.13` for the root Go runtime and isolated CLI module and `@wailsio/runtime` `3.0.0-beta.13` for the Overseer frontend. These were the newest matching beta versions published by the official Go module proxy and npm registry on 2026-08-26 and must advance together.

**Rationale**: Task is a cross-platform command runner implemented in Go, and Wails v3 routes its high-level build/package commands through root Taskfile tasks. A thin Task graph gives contributors one native command surface without moving archive or security policy into YAML. A typed Go target model removes the current macOS constants and lets tests verify artifact identity and layout. Go standard-library ZIP, TAR/GZIP, SHA-256, PE, and ELF support avoids another packager.

**Alternatives considered**: Keeping Make as a parallel alias graph was rejected because commands and variables would drift; moving detailed build policy entirely into Task YAML was rejected because typed validation, deterministic archives, and unit-testable plans belong in Go; native installers and third-party packagers were rejected because the specification requires portable archives and excludes installers.

## Task migration and tool bootstrap

**Decision**: Pin `github.com/go-task/task/v3/cmd/task` at `v3.53.1` in a new isolated `tools/task/go.mod` module. Add a root `Taskfile.yml` using schema version 3 and migrate every application workflow Make target to a documented Task task. Reduce `Makefile` to one default/phony `tools` bootstrap that discovers every `tools/*/go.mod` module and runs `go install tool` inside each—including Task—without hard-coding a partial list, plus a non-mutating `help` target that directs maintainers to `task --list`.

**Rationale**: The supplied Task documentation defines `Taskfile.yml`, version 3 syntax, named tasks, variables, and cross-platform shell interpretation; the supplied Wails guide demonstrates that Wails’ `build`, `package`, and platform dispatch use Taskfiles. Isolating Task exactly like Buf, Wails, generators, and the linter preserves tool-version ownership. A single bootstrap avoids the circular requirement that Task must already exist before it can install itself, while discovery ensures newly added tool modules cannot be omitted silently.

**Alternatives considered**: Installing Task globally or through an unpinned setup action was rejected because it bypasses repository version ownership; retaining Make workflow aliases after migration was rejected because it creates two command graphs, while informational `make help` remains safe because it performs no workflow; installing only Task in Make was rejected because the user requires one bootstrap for all Go tools and a hard-coded list would drift from `tools/`; making Task invoke `wails3 build` from the `build` task was rejected because Wails dispatches back to that task and would recurse.

## Portable build prerequisites and runtime baselines

**Decision**: Use the pinned Wails v3 default Linux stack—GTK4, WebKitGTK 6.0, and CGO—and document Ubuntu 24.04-class runtime packages as the initial supported Linux baseline. Use the WebView2 runtime on Windows, with Windows 11 ARM as the arm64 acceptance baseline and supported Windows 10/11 versions for amd64 as permitted by the pinned runtime. Runtime libraries remain operating-system prerequisites rather than being bundled into the archives.

**Rationale**: These are the native stacks selected by the accepted Wails version. Building on native runners with the same dependency family catches missing dynamic libraries before upload. Keeping platform runtimes external produces normal portable application archives while avoiding an undocumented custom runtime distribution.

**Alternatives considered**: Wails’ legacy GTK3 build mode was rejected because it is already a compatibility path scheduled for removal; statically bundling GTK/WebKitGTK or WebView2 was rejected because it would greatly expand the distribution and security-update surface; claiming support below the pinned runtime’s tested baselines was rejected because it would lack acceptance evidence.

## Packaged identity and resource resolution

**Decision**: Replace path-shape detection of a package with a compile-time production build profile. Development builds resolve resources from the checkout; production macOS builds retain `Contents/Resources`; production Windows and Linux builds resolve a read-only `resources` directory relative to the executable. The same production profile gates secret-bearing environment overrides and secure-store namespaces.

**Rationale**: The current `.app/Contents/MacOS` heuristic classifies every extracted Windows or Linux binary as a development build. That is both a startup bug and a security bug because it enables development credential overrides. A build-tagged profile is immutable at runtime and independent of executable naming or the current working directory.

**Alternatives considered**: Adding `.exe` and ELF path heuristics was rejected because renamed or relocated executables would still be ambiguous; using the current working directory in production was rejected because launchers and file managers do not guarantee it; a mutable linker string was rejected because it makes the security boundary easier to misconfigure than a production build tag.

## Platform storage profiles

**Decision**: Keep storage ownership in `internal/platform`, split OS-specific root discovery behind an injectable directory provider, and derive session documents and private application settings from native locations. Preserve the current macOS locations; use Windows Known Folders and application data; use XDG documents/config locations on Linux with documented home-directory fallbacks. Bundled resources never share a writable user-data root.

**Rationale**: The current `~/Documents` and `~/Library/Application Support` construction is not valid on Windows or XDG systems. An injected provider keeps path-policy tests deterministic for redirected, non-ASCII, spaced, and unavailable directories while the production adapter can use the pinned Wails path API or the corresponding OS facility.

**Alternatives considered**: Literal home-relative paths were rejected because they ignore redirection and XDG policy; storing settings next to the portable executable was rejected because extracted application directories may be read-only and would mix user data with trusted resources; environment-only resolution was rejected because it is incomplete on Windows and hard to validate consistently.

## Secure credential adapters

**Decision**: Preserve the existing `tunnel.SecretStore` contract and Darwin Keychain adapter. Add a Windows Credential Manager adapter using the pinned `github.com/danieljoos/wincred` library and a Linux Secret Service adapter using context-aware `github.com/godbus/dbus/v5` calls. Promote the actual runtime libraries to direct, version-pinned dependencies; map missing, locked, denied, and unavailable services to the existing fail-closed error identities; clear temporary secret bytes; and never add a file, environment, or settings fallback.

**Rationale**: Windows Credential Manager and the freedesktop Secret Service are the platform-protected stores needed by FR-008. Direct adapters retain precise error semantics, support bounded/cancelable Linux D-Bus calls, and keep secret lifetimes visible to the application. The public-access manager must preserve an initialization failure instead of later rewriting it as a generic missing-credential state.

**Alternatives considered**: `github.com/zalando/go-keyring` was rejected for this boundary because its string-based API prevents reliable buffer clearing, does not expose context for Linux calls, and coarsens error mapping; encrypted or plaintext files were rejected because the operating system, not an application-owned key, must protect the secrets; disabling public access silently was rejected because the specification requires a clear fail-closed status.

## Target metadata, layout, and deterministic evidence

**Decision**: Produce `Fallout-Terminal-windows-{amd64,arm64}.zip` and `Fallout-Terminal-linux-{amd64,arm64}.tar.gz`. Each archive contains the native executable, a `resources` tree with both bundled demos and notices, product/icon metadata, and a machine-readable manifest/checksum. Generate Windows PE resources through the pinned Wails `generate syso` path without committing architecture-specific generated output; verify PE/ELF architecture, exact inventory, safe relative paths, executable mode, metadata, and checksum before an artifact is eligible for upload.

**Rationale**: Stable names let users choose a target before extraction, while an internal manifest gives the aggregate workflow an exact four-artifact completeness contract. Deterministic timestamps, modes, and ordering make archive comparison meaningful. Header inspection prevents a correctly named archive from hiding a wrongly compiled executable.

**Alternatives considered**: Reusing one filename was rejected because parallel outputs would collide; relying only on archive names was rejected because labels are not proof of binary architecture or content; committing `.syso` files for both Windows architectures was rejected because generated target-specific objects can drift or be linked into the wrong build.

## CI evidence and publication gate

**Decision**: Add a portable-artifact workflow separate from the existing macOS release workflow. Its four independent native jobs install target prerequisites, build, inspect, unpack, launch to an observed application window, close cleanly, and upload only verified target artifacts. A final always-running aggregation job downloads successful outputs, verifies that exactly four unique target manifests/checksums exist, and exposes the combined set only when every target job succeeded.

**Rationale**: Separating workflows preserves macOS native build, reproducibility, checksum, and single-job assertions while giving each new platform an independent failure report. Upload-after-verification and an explicit aggregation gate implement the requirement that incomplete or unverifiable targets are never presented as successful output.

**Alternatives considered**: Extending the macOS job into a matrix was rejected because it couples unrelated platform and release surfaces; uploading first and validating later was rejected because failed binaries could appear successful; one build job plus inspection-only jobs was rejected because each platform must own its native build and launch evidence.

**Tagged-release decision**: Keep macOS and portable targets on their appropriate native runners, but join their verified outputs before publication. A SemVer tag resolves one immutable source SHA; the macOS path produces an unsigned `Fallout-Terminal-arm64.dmg` whose eligibility is determined only by its verified SHA-256 sidecar, while the portable path produces four eligible archives, their sidecars, and the aggregate index. Repository-pinned GoReleaser v2 owns the single GitHub Release publication only after the exact five-target inventory is present, and the same inventory is pushed as the versioned GHCR artifact.

**Tagged-release rationale**: Separate native jobs preserve platform build ownership, while the final checksum join makes “release all targets” observable and fail-closed. Publishing the four portable targets before the Darwin DMG checksum succeeds would create a misleading partial release.

**Tagged-release alternatives considered**: Leaving Darwin to an unrelated manual release was rejected because a SemVer tag would not represent all supported targets; requiring Developer ID/notarization was rejected by the maintainer because Darwin validation is checksum-only; allowing two independent GitHub Release writers was rejected because their failure and replacement behavior cannot provide one atomic eligibility decision.

## Existing macOS behavior and governance

**Decision**: Treat macOS packaging as an unchanged compatibility path and make a constitution amendment the first implementation prerequisite. The amendment must replace the macOS-only deployment statement with the approved target table, authorize the pinned Taskfile as the canonical command graph, reduce Make to the Go-tool bootstrap, and update platform-aware verification language without weakening the accepted Wails version, detailed Go build ownership, security, signing, or generated-code rules.

**Rationale**: The feature is explicitly authorized, but the current constitution still declares macOS 13+ arm64 as the only supported deployment profile and explicitly prohibits Taskfiles. Updating both governing decisions before implementation keeps code, CI, and project policy aligned. Keeping the macOS path distinct minimizes regression risk to native app, DMG, SHA-256, and reproducibility checks.

**Alternatives considered**: Implementing under standing deployment and Taskfile violations was rejected because both are governance decisions; folding macOS into the new portable archives was rejected because it has a distinct native bundle/DMG layout; omitting the macOS checksum gate was rejected because FR-015 requires preservation of verified artifact identity.
