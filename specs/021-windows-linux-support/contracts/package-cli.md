# Contract: Packaging Commands

## Target-specific command

```text
task package GOOS=<os> GOARCH=<arch>
```

Supported pairs are exactly:

```text
GOOS=windows GOARCH=arm64
GOOS=windows GOARCH=amd64
GOOS=linux GOARCH=arm64
GOOS=linux GOARCH=amd64
```

The OS identifiers are exactly `windows` and `linux`. The architecture identifiers are exactly `arm64` and `amd64`. Parsing is case-sensitive; aliases such as `win`, `x64`, `aarch64`, and `x86_64` are invalid. The Task task delegates target validation and packaging to `cmd/build`; it does not duplicate archive policy in YAML.

The Wails-compatible equivalent may enter through the pinned Wails CLI:

```text
go tool -modfile=tools/wails/go.mod wails3 package GOOS=<os> GOARCH=<arch>
```

Wails dispatches the same root `package` task and therefore reaches the same Go package plan. The Task `package` task must never call high-level `wails3 package`, which would recurse.

### Preconditions

- The executing host OS and architecture match `GOOS` and `GOARCH` exactly.
- The source checkout is at the expected clean revision.
- `make tools` has installed the repository-pinned Go tools, including Task and Wails, or the equivalent pinned `go tool -modfile=...` invocation is used by automation.
- Windows has the required Windows build facilities; Linux has CGO, `pkg-config`, GTK4, and WebKitGTK 6.0 development libraries.

### Success

- Exit status is zero only after compilation, static verification, archive creation, and post-archive verification succeed.
- The command reports the canonical target, archive path, source revision, and SHA-256 sidecar path.
- It produces exactly one target archive and its verification sidecar in the configured output directory.

### Failure

- Invalid or mismatched targets fail before staging is modified.
- Errors identify the target and failed phase and give actionable prerequisite information without printing secrets.
- An incomplete, unverified, or wrongly targeted archive is removed or quarantined outside the success output path.

## Existing macOS compatibility command

```text
task package
```

On the supported macOS arm64 host with no `GOOS`/`GOARCH` override, the command retains the existing behavior and produces `build/bin/Fallout Terminal.app`. ~~It does not join the four-target portable matrix.~~ The same canonical plan supplies the native Darwin bundle joined by local `task package:all`, while Darwin remains outside the four-target portable archive/release matrix. Signing, DMG, notarization, and reproducibility contracts do not change. The existing pinned-Wails entrypoint `go tool -modfile=tools/wails/go.mod wails3 package` resolves to the same root Task task.

## Local Docker aggregate command

```text
task package:all [OUTPUT=<directory>]
```

The default output directory is `build/dist`. On the supported `darwin/arm64` host this command builds the canonical native macOS package and all four portable targets in architecture-matched Linux containers, then delegates isolation, collection, validation, and atomic five-target publication to the Go aggregate helper.

### Preconditions

- The runtime host is exactly `darwin/arm64`; other hosts fail before package mutation because Darwin must be built natively.
- Docker is installed, its daemon is running, and BuildKit can execute `linux/amd64` and `linux/arm64` build platforms.
- `OUTPUT` may be absent, the repository-owned default `build/dist`, or a recognized previous local aggregate directory containing its regular `aggregate-index.json` marker.
- An existing file, symlink, repository/filesystem root, or unrecognized custom directory is rejected without modification.
- The current checkout has a valid Git `HEAD`; a clean tree, named branch, remote, push, GitHub CLI, and GitHub access are not required.

### Behavior

1. Validate the native `darwin/arm64` host, resolve the current `HEAD`, execute the canonical no-target package plan, and copy the complete ad-hoc signed bundle into quarantine as `bin/darwin-arm64/Fallout Terminal.app`.
2. Pass the current Docker build context to each isolated portable target build.
3. Build Linux with native CGO inside a target-architecture Linux container and Windows with CGO-disabled Go cross-compilation inside the matching-architecture container.
4. Export each archive, checksum, executable, and required resource tree to a target-owned quarantine and report target progress independently.
5. Verify the Darwin bundle, then revalidate the source revision, exact portable target set, unique names, manifests, PE/ELF identity, checksums, and byte-for-byte agreement between each portable runnable payload and its archive; create `aggregate-index.json` for the four portable artifacts.
6. Expose the Darwin application bundle, complete portable release inventory, and `bin/<os>-<arch>/` runnable payloads in `OUTPUT`. When an owned prior output exists, retain it until verification finishes, rename it into a same-filesystem sibling work root, publish the new directory, restore the backup if final publication fails, and remove the backup only after success.

### Success

- Exit status is zero only when the verified Darwin application bundle and all four portable target records, archives, and executable payloads are present locally.
- The command reports the resolved source SHA, output directory, Darwin application bundle path, and archive, checksum, and executable path for every portable target.

### Failure

- An unavailable Docker daemon, unsupported emulation platform, failed build, missing, duplicated, mismatched-revision, or unverifiable target makes the command nonzero.
- Docker installation, daemon, or build-platform failures identify Docker as the failed prerequisite, preserve the underlying command diagnostic when available, and include an actionable install/start/platform recovery instruction. Task MUST NOT replace this diagnostic with a generic precondition failure.
- A partial matrix is reported by target but is never presented in the aggregate success output directory.
- Build or verification failure leaves an existing owned output byte-unchanged. Final publication failure restores the prior output before returning nonzero; if rollback itself fails, the command reports and retains the recovery directory instead of deleting it. An unsafe existing target is rejected before Docker starts.
- The Darwin bundle is built natively through the canonical package plan. Docker output proves Windows/Linux build and static artifact validity only; it does not satisfy their native application launch acceptance.

## Remote native aggregate command

```text
task package:all:remote [OUTPUT=<directory>]
```

This command retains the GitHub Actions orchestration contract: it requires `gh` authentication, a named clean current branch whose `HEAD` exactly matches `origin`, and an installed portable workflow. It correlates and dispatches the native matrix, reports each runner, waits for the aggregate gate, downloads the verified result into quarantine, revalidates it, and atomically exposes it in `OUTPUT`. Authentication, authorization, ref, workflow-dispatch, and download failures remain distinct and actionable.

## Help and compatibility

- `task --list` documents single-target, local Docker aggregate, and remote native aggregate packaging plus the four exact target pairs.
- Existing developer, quality, protobuf, browser, Spec Kit, and release workflows have Task equivalents defined by the task-runner contract.
- Unknown Task variables used as target inputs and invalid arguments passed through to Go fail nonzero rather than being ignored.
