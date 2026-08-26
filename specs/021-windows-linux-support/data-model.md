# Data Model: Windows and Linux Desktop Support

This feature does not change persisted session or player RPC data. Its new models belong to build orchestration, platform composition, and verification; target manifests are build metadata and do not require protobuf definitions.

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

Relationship: one Build Host owns one Target Package attempt. Aggregate orchestration owns four remote Build Hosts, never impersonates them locally.

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
| `Variables` | typed-by-contract strings | Forward existing inputs plus `GOOS`, `GOARCH`, `REF`, output path, and application args explicitly. |
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
- An Aggregate Packaging Run contains exactly one eligible artifact for each of the four targets.
- Bundled resources are immutable inputs; user session documents and settings never appear in an archive.

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

## Aggregate Packaging Run

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
