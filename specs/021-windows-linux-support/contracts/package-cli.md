# Contract: Target Packaging and Release Checks

## Target-specific package command

Every tagged-release target uses the same repository entrypoint:

```text
task package GOOS=<os> GOARCH=<arch>
```

Supported pairs are exactly:

```text
GOOS=windows GOARCH=amd64
GOOS=windows GOARCH=arm64
GOOS=linux GOARCH=amd64
GOOS=linux GOARCH=arm64
GOOS=darwin GOARCH=arm64
```

The identifiers are exact and case-sensitive: `windows`, `linux`, `darwin`, `amd64`, and `arm64`. `darwin/arm64` is the only supported macOS target. Aliases and `darwin/amd64` fail.

The Task task delegates parsing, host validation, preparation order, staging, compilation, and archive creation to `cmd/build` and `internal/buildtool`; workflow YAML does not duplicate that policy.

Successful production of the governed non-empty archive with its executable and required resources is the platform-support boundary for this feature. Native UI and operating-system integration journeys are optional evidence and do not change that result.

### Preconditions

- The host operating system and architecture exactly match the requested target.
- The checkout is the pushed tag revision.
- Pinned Go, Node.js, Task, Wails, and locked frontend dependencies are available.
- Linux runners provide the GTK4/WebKitGTK 6.0 build dependencies.
- No Docker daemon, signing identity, notarization credential, GitHub CLI, GoReleaser token, or package-registry credential is required to package one target.

### Success

- Compilation succeeds.
- Exactly one stable portable archive for the requested target exists in `build/dist/` and is non-empty.
- The archive contains the expected executable and required runtime resources.
- `darwin/arm64` contains the complete unsigned `Fallout Terminal.app` bundle inside `Fallout-Terminal-darwin-arm64.zip`.
- No codesign, DMG, notarization, stapling, or Gatekeeper step executes in the explicit Darwin path.

### Failure

- Invalid or host-mismatched targets fail before target output is published.
- Failed compilation, staging, or archive creation returns nonzero and does not report an eligible archive.
- Diagnostics identify the target and phase without printing secrets or user data.

## Network-free release contract checks

The tagged-release workflow uses lower-level repository checks; these commands never contact GitHub or publish anything.

### Tag validation

```text
go run ./cmd/build validate-release-tag --tag <tag>
```

The command accepts `vMAJOR.MINOR.PATCH` with an optional valid SemVer prerelease suffix, rejects leading-zero numeric identifiers and build metadata, and reports whether the release is a prerelease.

### Per-target archive inspection

```text
go run ./cmd/build inspect-release-archive \
  --target <os>/<arch> \
  --archive <path>
```

The command validates only the exact supported target, stable filename, non-zero archive, expected executable, and required resources. It does not require a checksum sidecar, launch the application, open a dialog, access a credential store, start a player or tunnel, inspect signing, or contact GitHub.

### Publication inventory inspection

```text
go run ./cmd/build inspect-release-inventory --directory <path>
```

The command accepts exactly the five stable non-empty archives and rejects directories, missing targets, duplicates, extras, checksum sidecars, aggregate indexes, raw executables, DMGs, and external verification records.

## Optional local aggregate

```text
task package:all
task package:all OUTPUT=<directory>
```

This command retains the existing local `package-all-docker` implementation on its supported `darwin/arm64` host. It may produce local checksum/index evidence and directly accessible payloads, but it is a maintainer convenience only:

- neither PR/main quality CI nor the tag release workflow invokes it;
- its Docker artifacts are not matching-native release evidence;
- GoReleaser never consumes its output; and
- its success or failure cannot gate a tagged release.

## Default macOS developer command

```text
task package
```

The no-argument command may continue producing the current local macOS package for developer use. Tagged releases always use explicit `GOOS=darwin GOARCH=arm64` and therefore cannot fall into a signed or DMG-specific default path.

## Removed command surface

These aliases and backing CLI actions are removed:

```text
task package:all:remote
task release:local
go run ./cmd/build package-all
go run ./cmd/build release-candidate
```

The retained `package-all-docker` action backs only `task package:all`. Pushing a qualifying tag is the sole automated five-target release entrypoint.
