# Data Model: Windows and Linux Desktop Support

This feature does not change persisted session or player RPC data. Its new models belong to build orchestration, platform composition, and verification; target manifests are build metadata and do not require protobuf definitions.

**Bugfix**: 2026-08-26 — BUG-001 adds the local Docker aggregate and directly runnable payload models without changing native release eligibility.

**Bugfix**: 2026-08-26 — BUG-002 adds owned-output recognition and rollback-safe repeat publication.

**Bugfix**: 2026-08-26 — BUG-003 joins the native Darwin compatibility package to the local aggregate.

**Bugfix**: 2026-08-26 — BUG-004 adds the joined five-target tagged-release model and reopens incomplete validation.

## Platform Target

Represents one buildable desktop destination.

| Field | Type | Rules |
|---|---|---|
| `OS` | enum-like string | New public values are exactly `windows` or `linux`; existing compatibility value is `darwin`. |
| `Arch` | enum-like string | Values are exactly `arm64` or `amd64`; the existing macOS compatibility target remains `darwin/arm64`. |
| `ExecutableName` | string | `Fallout Terminal.exe` on `windows`; `Fallout Terminal` on `linux` and in the existing macOS bundle. |
| `ArchiveFormat` | enum | ZIP for `windows`; TAR.GZ for `linux`; the existing macOS package format is unchanged. |
| `BuildTags` | ordered set | Includes `production`; Linux uses the default GTK4 path and does not add `gtk3`. |
| `NativeRuntime` | descriptor | WebView2 for `windows`; GTK4/WebKitGTK 6.0 plus Secret Service for `linux`. |

Validation rules:

- Only the four requested Windows/Linux pairs are portable-archive targets.
- Target strings are canonical lowercase `<OS>/<Arch>` values; aliases, case changes, partial targets, and extra architectures are invalid.
- A target-specific package run requires both host OS and host architecture to match the target.
- Existing no-target `package` resolves only to `darwin/arm64` and does not enter the portable matrix.

## Build Host

Captures the environment executing one package plan.

| Field | Type | Rules |
|---|---|---|
| `OS` | string | Obtained from the Go runtime, not caller input. |
| `Arch` | string | Obtained from the Go runtime, not caller input. |
| `Tools` | map of name to version | Must match repository pins for Go, Node/npm, Buf/protoc tooling, and Wails CLI. |
| `NativeDependencies` | set | Target-specific development/runtime probes required before compile and launch. |
| `SourceRevision` | commit identity | Must match the clean revision requested by the workflow. |

Relationship: one Build Host owns one matching-host Target Package attempt. ~~Aggregate orchestration owns four remote Build Hosts, never impersonates them locally.~~ Remote native orchestration owns four Build Hosts; local Docker orchestration instead owns four Docker Build Environments and may produce static build evidence without impersonating or satisfying native launch evidence.

## Docker Build Environment

Captures one isolated build environment used by the local aggregate command.

| Field | Type | Rules |
|---|---|---|
| `ContainerPlatform` | string | Exactly `linux/amd64` or `linux/arm64`, matching the requested target architecture. |
| `Target` | Platform Target | One of the four portable Windows/Linux targets; Windows is CGO-free cross-compilation and Linux uses native-architecture CGO. |
| `BuildContext` | checkout snapshot | Current Docker build context, including allowed uncommitted files and excluding `.dockerignore` entries. |
| `SourceRevision` | commit identity | Current `HEAD` recorded in artifact metadata; a clean tree, named branch, remote, and push are not required. |
| `EvidenceKind` | constant | `static`; it cannot record a native window launch or release eligibility. |

Relationship: one Local Aggregate Run owns exactly four Docker Build Environments, one per portable target, and one native Darwin Package Plan.

## Package Plan

An immutable ordered plan derived from a Platform Target and Build Host.

| Field | Type | Rules |
|---|---|---|
| `Target` | Platform Target | Valid and host-compatible before any destructive staging action. |
| `StageRoot` | path | Isolated beneath `build/bin/<os>-<arch>/`; must pass owned-path allowlist checks. |
| `OutputPath` | path | Stable target-specific archive name beneath `build/dist`. |
| `Actions` | ordered list | Preflight, frontend/protobuf/bindings, resource assembly, compile, metadata, inspect, archive, verify. |
| `Environment` | key/value set | Explicit target variables and `CGO_ENABLED`; no secret-bearing values. |
| `ProductionProfile` | constant | Always production for packages and cannot be changed by runtime input. |

Validation rules:

- All dependency, generation, resource, and license checks finish before compilation is eligible.
- Inspection finishes before archive publication.
- Commands enter through the pinned root Taskfile, delegate detailed plans to repository Go code, and use the pinned Wails tool module; Task and Wails task dispatch must not recurse.
- Failure leaves no archive or checksum that can be interpreted as successful.

## Tool Module

Represents one independently pinned Go development command under `tools/<tool>/`.

| Field | Type | Rules |
|---|---|---|
| `Directory` | path segment | Unique child of `tools/`; Task uses `task`. |
| `CommandPackage` | Go package | Exactly one `tool` directive per module. |
| `Version` | semantic version | Explicitly pinned and checksum-locked. |
| `BinaryName` | string | Installed by the Make bootstrap into the configured Go binary directory. |

Validation rules:

- The bootstrap discovers modules from the filesystem; it does not maintain a second hard-coded inventory.
- Every discovered module is installed or the bootstrap fails nonzero.
- `github.com/go-task/task/v3/cmd/task` is owned only by `tools/task` and is pinned at v3.53.1.

## Task Graph

Defines the sole repository workflow-alias surface after migration.

| Field | Type | Rules |
|---|---|---|
| `SchemaVersion` | string | Taskfile schema `3`. |
| `TaskName` | string | Stable documented command, including migrated Make tasks and package matrix tasks. |
| `Dependencies` | ordered/DAG references | Preserve current prerequisite ordering and avoid duplicate execution. |
| `Variables` | typed-by-contract strings | Forward existing inputs plus `GOOS`, `GOARCH`, output path, and application args explicitly. Local aggregate packaging has no `REF`; remote aggregate packaging resolves the current branch and revision from Git. |
| `Command` | process invocation | Calls Go build policy, pinned tools, npm, or an owned script; top-level Wails tasks cannot call the matching high-level Wails wrapper recursively. |

Relationship: the Task Graph invokes Package Plans and quality/release helpers. The Make bootstrap installs the Task Tool Module but does not invoke application workflows.

## Distribution Artifact

One portable runnable archive and its verification sidecar.

| Field | Type | Rules |
|---|---|---|
| `Target` | Platform Target | Must equal the archive name, executable header, and embedded manifest. |
| `ArchiveName` | string | One of the four names defined by the artifact contract. |
| `RootDirectory` | string | Exactly `Fallout Terminal`. |
| `Entries` | ordered list | Exact allowlisted paths; no absolute, parent, duplicate, symlink, device, or provider-executable entries. |
| `FileManifest` | document | Product, source revision, target, relative file paths, sizes, modes, and SHA-256 values. |
| `ArchiveChecksum` | SHA-256 | Sidecar over the completed archive; cannot be stored recursively inside it. |
| `Verification` | Target Verification Record | Must be eligible before upload or aggregate inclusion. |

Relationships:

- A Distribution Artifact belongs to exactly one Platform Target and source revision.
- A Remote Native Aggregate Packaging Run contains exactly one eligible artifact for each of the four targets.
- A successful Local Aggregate Run pairs every Distribution Artifact with one verified Runnable Payload and includes one verified native Darwin application bundle.
- Bundled resources are immutable inputs; user session documents and settings never appear in an archive.

## Runnable Payload

The directly accessible application tree exported by local Docker packaging for one target.

| Field | Type | Rules |
|---|---|---|
| `Target` | Platform Target | Must match its directory name, executable header, and paired Distribution Artifact. |
| `Directory` | path | Exactly `bin/<os>-<arch>/` below the aggregate output. |
| `Executable` | path | `Fallout Terminal.exe` for Windows or `Fallout Terminal` for Linux. |
| `Resources` | directory | Exact required `resources/` tree from the paired archive. |
| `Inventory` | ordered list | Must equal the paired archive inventory after removing the archive root directory. |
| `FileHashes` | map path to SHA-256 | Every executable/resource file MUST equal the corresponding verified archive entry byte-for-byte. |

Runnable Payloads are local developer output, not additional release artifacts. They carry static
identity evidence only and do not become native-launch eligible merely because their archive is
valid.

## Production Storage Profile

Separates read-only product resources from user-owned writable data.

| Field | Type | Rules |
|---|---|---|
| `Packaged` | compile-time boolean | True only for production-tagged application builds. |
| `ResourceRoot` | path | macOS `Contents/Resources`; Windows/Linux `<executable-dir>/resources`; checkout root only in development. |
| `SessionDocumentsRoot` | path | Native Documents location plus the product directory. |
| `PrivateSettingsRoot` | path | macOS Application Support, Windows application data, or Linux XDG config location. |
| `CredentialNamespace` | string | Production and development namespaces remain separate; selected from immutable build profile. |

Validation rules:

- ResourceRoot cannot be derived from the current working directory in production.
- Writable data roots cannot be children of packaged ResourceRoot.
- Redirected paths, spaces, Unicode, and native separators remain valid.
- A missing or unsafe native root yields an actionable error; it does not silently select the application directory.

## Credential Backend Profile

Maps the unchanged application secret contract to an OS-protected provider.

| Field | Type | Rules |
|---|---|---|
| `OS` | string | Darwin, `windows`, or `linux`. |
| `Provider` | enum | Apple Keychain, Windows Credential Manager, or freedesktop Secret Service. |
| `Service` | string | Stable product/environment namespace. |
| `Accounts` | closed set | Existing fixed public-access account identifiers only. |
| `Availability` | state | Available, locked, denied, unavailable, or timed out; never inferred as “missing credential.” |

Validation rules:

- Presence, replace, delete, and scoped use preserve the existing `tunnel.SecretStore` semantics.
- Secret material is never serialized into a session, settings, manifest, log, status, environment, or process argument.
- Temporary byte buffers are cleared after use; Linux provider operations are context-bounded.
- Provider failure has no file or plaintext fallback and does not disable local/LAN player access.

## Target Verification Record

Captures evidence for one native package attempt.

| Field | Type | Rules |
|---|---|---|
| `Target` | Platform Target | Matches the package attempt and artifact manifest. |
| `SourceRevision` | commit identity | Same revision for every target in one aggregate run. |
| `Checks` | named result set | Build gates, header, inventory, metadata, checksum, runtime prerequisite, launch, demo, close, and listener release. |
| `WindowObservedAt` | timestamp | Present only after a real target window is detected within 60 seconds. |
| `Failure` | structured build error | Names the target, failed phase, and actionable cause without secrets. |
| `Eligibility` | state | `pending`, `building`, `packaged`, `verified`, `eligible`, or `failed`. |

State transition:

```text
pending -> building -> packaged -> verified -> eligible
             |           |           |
             +-----------+-----------+-> failed
```

- `packaged` means an archive exists only in target-job staging; it is not uploadable yet.
- `verified` requires all static checks and native launch/close evidence.
- `eligible` permits the target job to upload its archive and checksum.
- Any failure is terminal for that attempt and invalidates/quarantines partial output.

## Remote Native Aggregate Packaging Run

Coordinates the complete native matrix for one source revision.

| Field | Type | Rules |
|---|---|---|
| `CorrelationID` | opaque string | Unique per dispatch and included in workflow inputs/title for unambiguous run discovery. |
| `SourceRevision` | git ref plus resolved SHA | All target jobs must resolve the same SHA. |
| `Targets` | fixed set | Exactly `windows/arm64`, `windows/amd64`, `linux/arm64`, and `linux/amd64`. |
| `Records` | map target to Target Verification Record | Exactly one terminal record per target. |
| `OutputDirectory` | path | Owned destination, default `build/dist`; collisions are rejected or replaced only under the current run’s allowlist. |
| `Status` | state | `dispatching`, `running`, `succeeded`, or `failed`. |

State transition:

```text
dispatching -> running -> succeeded
                    |
                    +-> failed
```

- `succeeded` requires four eligible records, four unique archive names, one source SHA, and valid checksum sidecars.
- A canceled, missing, duplicated, or failed target makes the aggregate run failed.
- Downloads occur only after the workflow aggregate gate succeeds; partial matrices are not copied into the success output directory.

## Tagged Release Run

Joins every supported native distribution for one SemVer tag before either publication destination reports success.

| Field | Type | Rules |
|---|---|---|
| `Tag` | SemVer tag | Exactly `vMAJOR.MINOR.PATCH` with an optional prerelease suffix. |
| `SourceRevision` | commit identity | The tag's immutable SHA; every Darwin and portable input MUST resolve to it. |
| `DarwinArtifact` | trusted distribution | Exactly `Fallout-Terminal-arm64.dmg` and its SHA-256 sidecar after Developer ID signing, hardened runtime, notarization, stapling, Gatekeeper, and release verification. |
| `PortableArtifacts` | fixed artifact set | Exactly four eligible Windows/Linux archives and sidecars plus `aggregate-index.json`. |
| `ReleasePublisher` | tool identity | Repository-pinned GoReleaser v2; it consumes preverified inputs and does not rebuild them. |
| `PackagePublisher` | tool identity | Repository-pinned ORAS client publishing the identical joined inventory as a versioned GHCR artifact. |
| `Destinations` | fixed set | One GitHub Release and one GitHub Packages GHCR reference for the same tag. |
| `Status` | state | `building`, `joining`, `publishing`, `succeeded`, or `failed`. |

State transition:

```text
building -> joining -> publishing -> succeeded
    |          |            |
    +----------+------------+-> failed
```

- `joining` succeeds only when all five target distributions, every checksum, and the portable aggregate index are present, nonempty, verified, and bound to one tag SHA.
- A prerelease suffix marks the GitHub Release as a prerelease and retains the exact tag in the GHCR version.
- Any missing target, signing/notarization failure, source mismatch, invalid checksum, unexpected file, GitHub Release failure, or GHCR failure makes the run failed and MUST NOT be represented as a successful complete release.
- Diagnostic workflow artifacts may be retained under repository policy, but they are not release assets or versioned package success.

## Local Aggregate Run

Coordinates the canonical native Darwin package and complete portable static matrix from the current checkout.

| Field | Type | Rules |
|---|---|---|
| `SourceRevision` | commit identity | Current `HEAD`; uncommitted build-context changes are allowed and no remote identity is required. |
| `NativeTarget` | Platform Target | Exactly `darwin/arm64`, built on the matching runtime host through the canonical no-target package plan. |
| `PortableTargets` | fixed set | Exactly `windows/arm64`, `windows/amd64`, `linux/arm64`, and `linux/amd64`. |
| `Builds` | map target to Docker Build Environment | Exactly one terminal build result per target. |
| `DarwinBundle` | native application tree | `bin/darwin-arm64/Fallout Terminal.app`, copied from `build/bin/Fallout Terminal.app` with exact regular-file inventory and modes after canonical packaging. |
| `Artifacts` | map target to Distribution Artifact | Exactly four statically verified archives and checksum sidecars. |
| `Payloads` | map target to Runnable Payload | Exactly four payload directories whose inventory and hashes equal their paired archives. |
| `OutputDirectory` | path | Owned destination, default `build/dist`; it may be absent, the repository-owned default, or a recognized previous aggregate with a regular aggregate index. Files, symlinks, roots, and unrecognized custom directories are rejected. |
| `PreviousOutput` | optional path | Existing owned output retained unchanged until complete verification, then moved into a same-filesystem sibling work root for the final replacement transaction. |
| `Failure` | structured build error | Names Docker installation/daemon/platform or target/phase failure, preserves the underlying diagnostic when available, and gives an actionable recovery instruction. |
| `Status` | state | `checking`, `building`, `verifying`, `succeeded`, or `failed`. |

State transition:

```text
checking -> building -> verifying -> succeeded
    |          |           |
    +----------+-----------+-> failed
```

- `succeeded` requires the complete verified Darwin application bundle, four verified archives, four checksum sidecars, one aggregate index, and four matching portable Runnable Payloads.
- `failed` removes or quarantines all temporary output and never exposes the requested output directory as a partial success.
- Build and verification failures leave `PreviousOutput` untouched. A final publish failure restores its backup before the command returns nonzero; successful replacement removes the backup and stale prior matrix entries together.
- A non-`darwin/arm64` host, Darwin package/copy/verification failure, Docker absence, stopped daemon, unsupported build platform, target failure, inventory mismatch, or hash mismatch is nonzero and actionable.
- The Darwin bundle is a native build result but is not new launch evidence by itself. Static local success MUST NOT create Windows/Linux native window, runtime, lifecycle, or secure-store acceptance evidence.
