# Platform Packaging Guide

This guide is for maintainers building Fallout Terminal from source. Portable Windows and Linux
packages can be built one at a time on a matching native host, as a complete local Docker matrix,
or through the GitHub-hosted native matrix. Docker cross-platform packaging verifies artifact
structure and binary identity, but it is not a substitute for native launch verification.

For end-user operating-system prerequisites, launch instructions, data locations, and secure-store
troubleshooting, see [Platform Support](platform-support.md).

## Bootstrap the pinned tools

Install Go, Node.js, and the platform build prerequisites first. Then, from the repository root,
install every repository-pinned Go tool module:

```text
make help
make tools
```

`make help` describes the bootstrap and points to `task --list`; it does not run a project workflow.
`make tools` is the only state-changing Make command, and plain `make` has the same effect. It discovers every
`tools/*/go.mod` module and runs `go install tool`, including the pinned Task and Wails commands.
Use `task --list` (or plain `task`) to discover application workflows after bootstrap.

## Complete Task command reference

The root `Taskfile.yml` is the public command graph. The Go helper under `cmd/build` remains the
source of build order, target validation, package layout, and verification policy.

| Command | Purpose and important inputs |
|---|---|
| `task --list` | List the documented public tasks. Plain `task` delegates to this listing. |
| `task dev [APP_ARGS="..."]` | Prepare, build, and run the complete development application; `APP_ARGS` is forwarded to the application. |
| `task run [APP_ARGS="..."]` | Build and run the complete application; `APP_ARGS` is forwarded to the application. |
| `task prepare` | Verify/generate protobuf assets, build the player, generate Wails bindings, and build Overseer assets in the owned order. |
| `task build [GOOS=<os> GOARCH=<arch>]` | Build for the current host by default, or for one exact supported target when both variables are supplied. |
| `task package [GOOS=<os> GOARCH=<arch>]` | Preserve the macOS arm64 package path with no override, or create one matching-host Windows/Linux portable archive. |
| `task package:all [OUTPUT=<directory>]` | On `darwin/arm64`, build the native macOS bundle and all four portable targets from the current checkout locally with Docker. |
| `task package:all:remote [OUTPUT=<directory>]` | Dispatch the current clean pushed branch, wait for, verify, and download the complete native four-target GitHub Actions matrix. |
| `task release:local [OUTPUT=<directory>]` | Build and verify the exact 11 unsigned files that a version tag publishes; the default output is `build/release`. |
| `task deps` | Install both locked frontend and browser-test npm workspaces. |
| `task deps:frontend` | Install locked client and Overseer dependencies with `npm ci`. |
| `task deps:browser` | Install locked browser-test dependencies with `npm ci`. |
| `task speckit:install` | Install the repository-pinned Spec Kit and Companion extensions. |
| `task speckit:update:check` | Check the pinned Spec Kit/extension versions for updates without installing them. |
| `task speckit:update:test` | Run the network-free update-checker regression suite. |
| `task fmt` | Format all Go sources. |
| `task fmt:check` | Fail if any Go source needs formatting, without changing files. |
| `task vet` | Run Go static analysis. |
| `task lint` | Run the pinned golangci-lint tool module. |
| `task test` | Run the Go test suite. |
| `task test:race` | Run the Go test suite with the race detector. |
| `task proto:generate` | Install locked frontend dependencies, regenerate protobuf outputs, and sync the reviewed revision. |
| `task proto:check` | Verify protobuf format, lint, build, generated clients, and drift. |
| `task proto:breaking` | Run protobuf compatibility checks and all negative fixtures. |
| `task bindings:check` | Verify deterministic Wails bindings and their reviewed public surface. |
| `task browser:test` | Install locked frontend/browser dependencies and Chromium, then run Playwright journeys. |
| `task check` | Run formatting, vet, lint, race, protobuf, bindings, and Spec Kit update regression gates; any failed constituent fails the task. |
| `task release:macos:preflight` | Validate the optional macOS Developer ID and notarization prerequisites. |
| `task release:macos:signed` | Build, sign, notarize, and verify the optional manually distributed macOS DMG. |

Task exits nonzero when an input is missing or invalid or when any owned step fails. `GOOS` and
`GOARCH` must be supplied together and are case-sensitive. Aliases such as `win`, `x64`, `x86_64`,
and `aarch64` are rejected instead of being guessed.

## Wails-compatible entrypoints

The pinned Wails CLI dispatches the corresponding root Task task. These forms are equivalent to
the direct Task commands:

```text
go tool -modfile=tools/wails/go.mod wails3 dev
go tool -modfile=tools/wails/go.mod wails3 run
go tool -modfile=tools/wails/go.mod wails3 build
go tool -modfile=tools/wails/go.mod wails3 package
go tool -modfile=tools/wails/go.mod wails3 build GOOS=linux GOARCH=amd64
go tool -modfile=tools/wails/go.mod wails3 package GOOS=linux GOARCH=amd64
```

The Task implementations call `go run ./cmd/build`; they never call the matching high-level
`wails3` command. This prevents a Wails-to-Task-to-Wails recursion loop and keeps direct Task and
Wails-dispatched outputs identical.

## Package one matching native target

Run exactly one of these commands on the matching native host:

| Native host | Canonical target | Command | Archive |
|---|---|---|---|
| Windows x64 | `windows/amd64` | `task package GOOS=windows GOARCH=amd64` | `Fallout-Terminal-windows-amd64.zip` |
| Windows arm64 | `windows/arm64` | `task package GOOS=windows GOARCH=arm64` | `Fallout-Terminal-windows-arm64.zip` |
| Linux x64 | `linux/amd64` | `task package GOOS=linux GOARCH=amd64` | `Fallout-Terminal-linux-amd64.tar.gz` |
| Linux arm64 | `linux/arm64` | `task package GOOS=linux GOARCH=arm64` | `Fallout-Terminal-linux-arm64.tar.gz` |

The four identifiers above are the complete portable target set. Each command validates that the
host matches its requested target before changing staging, performs the target’s preflight and
build steps, verifies the staged executable, creates the archive, and verifies the completed
archive. Success writes the archive and a sibling `<archive>.sha256` file under `build/dist` and
reports the canonical target, source revision, archive path, and checksum path.

With no target override, `task package` retains the macOS arm64 application bundle at
`build/bin/Fallout Terminal.app`. That compatibility package is not a fifth portable-matrix entry.

## Portable archive layout

Every archive has one top-level `Fallout Terminal/` directory. Windows ZIPs contain:

```text
Fallout Terminal/
├── Fallout Terminal.exe
├── artifact-manifest.json
└── resources/
    ├── appicon.png
    ├── THIRD_PARTY_NOTICES.md
    └── sessions/
        ├── demo.json
        └── demo-players.json
```

Linux TAR.GZ archives contain the same inventory with the native executable name and mode:

```text
Fallout Terminal/
├── Fallout Terminal
├── artifact-manifest.json
└── resources/
    ├── appicon.png
    ├── THIRD_PARTY_NOTICES.md
    └── sessions/
        ├── demo.json
        └── demo-players.json
```

`artifact-manifest.json` has schema version 1, product `Fallout Terminal`, the full source revision,
the exact target OS and architecture, the required native runtime description, and a path-sorted
record of every other regular file’s size, normalized mode, and SHA-256. It excludes its own hash,
timestamps, absolute builder paths, environment values, and credentials. The external `.sha256`
sidecar contains the lowercase archive digest, two spaces, the archive basename, and a newline.

Verification rejects unexpected or unsafe inventory, absolute/traversal paths, links or special
files, a wrong PE/ELF machine, missing product/icon/runtime metadata, incorrect modes, a target or
source mismatch, and a bad manifest or sidecar checksum.

## Package the complete matrix locally on macOS with Docker

`package:all` builds the native `darwin/arm64` application bundle and all four portable targets from
the current checkout. It requires a `darwin/arm64` host plus Docker and a running Docker daemon with
BuildKit support. Other hosts fail before packaging or output replacement:

```text
task package:all
task package:all OUTPUT=build/portable-local
```

The helper first executes the same canonical native plan as no-target `task package`, copies the
complete ad-hoc signed application bundle into quarantine, and verifies its required inventory,
ARM64 Mach-O executable, and signature. It then runs four isolated `docker build` operations from
`build/docker/Dockerfile.package`.
Linux targets run with `--platform=linux/amd64` or `linux/arm64` so their CGO, GTK4, and WebKitGTK
compilation happens on the target architecture. Windows uses Go's CGO-free cross-compilation inside
the corresponding architecture container. The image selects the Go 1.27 and Node.js 24 image lines and
installs the Linux desktop development packages required by Wails.

Unlike the remote command, the local command does not require a named branch, clean working tree,
push, `origin`, `gh`, or GitHub access. Docker receives the current build context, including tracked
and untracked source changes not excluded by `.dockerignore`; `.env` files, VCS metadata, dependency
trees, and previous outputs are excluded. The archive manifest identifies the current `HEAD` source
revision, just like a one-target local `task package` build.

Each target is exported into an owned quarantine. The helper rejects an incomplete or invalid Darwin
bundle and missing or extra portable payload files, revalidates every archive, manifest, PE/ELF
architecture, and checksum, writes
`aggregate-index.json`, verifies that every exported executable and resource matches its archive,
and publishes the complete matrix to `OUTPUT`. The repository-owned default `build/dist` may
already exist, so repeated canonical runs need no manual cleanup. A custom existing directory is
replaceable only when its regular `aggregate-index.json` identifies it as a previous aggregate;
files, symlinks, repository/filesystem roots, and unrelated custom directories are rejected before
Docker starts.

An existing owned output remains untouched throughout build and verification. At final publication
the helper moves it into a same-filesystem sibling work root, renames the verified matrix into place, restores the
backup if that rename fails, and removes the backup after success. If rollback itself fails, the
error reports the retained recovery directory instead of deleting it. Any failed target removes the
temporary matrix and leaves the previous successful output available.

The root of `OUTPUT` retains the nine Windows/Linux release files. It also exposes the native Darwin
bundle and the same runnable payload that each target-specific `package` command stages:

```text
OUTPUT/
├── Fallout-Terminal-<os>-<arch>.<zip|tar.gz>
├── Fallout-Terminal-<os>-<arch>.<zip|tar.gz>.sha256
├── aggregate-index.json
└── bin/
    ├── darwin-arm64/Fallout Terminal.app/
    ├── windows-amd64/Fallout Terminal.exe
    ├── windows-arm64/Fallout Terminal.exe
    ├── linux-amd64/Fallout Terminal
    └── linux-arm64/Fallout Terminal
```

The Darwin path contains the entire signed `.app`; every Windows/Linux `bin/<os>-<arch>/` directory
also contains the required `resources/` tree. The coordinator rejects an executable payload whose
exact inventory or file hashes differ from its verified archive.
The runnable Darwin path is `bin/darwin-arm64/Fallout Terminal.app` beneath `OUTPUT`.

Darwin is built natively. Docker packaging cannot launch Windows or Linux desktop applications on
their native UI stack; use the remote native matrix before treating those artifacts as release evidence.

## Build the exact release candidate locally

`release:local` wraps the complete local matrix, creates an unsigned compressed DMG from the verified
Darwin bundle, and publishes only the files configured in `.goreleaser.yaml`:

```text
task release:local
task release:local OUTPUT=build/release-check
```

The default output is `build/release`. It contains exactly 11 regular files: the Darwin DMG and its
sidecar, four Windows/Linux archives and their sidecars, and `aggregate-index.json`. The command
recomputes all five SHA-256 values, revalidates the four portable archive manifests, and checks the
exact inventory before atomically replacing a previous recognized candidate.

GoReleaser does not compile this Wails application. Its checked-in configuration intentionally skips
builds and uploads these prebuilt files. Platform packaging remains in `cmd/build`, where the app
bundle, native metadata, resources, CGO requirements, and portable layouts are handled consistently.

## Package the complete matrix through GitHub Actions

`package:all:remote` is remote native orchestration. Install the GitHub CLI,
authenticate it for the current repository, and confirm the session before dispatch:

```text
gh auth login
gh auth status
task package:all:remote
```

`package:all:remote` selects the current branch automatically and derives the target GitHub repository
from the `origin` remote, independent of any repository saved by `gh repo set-default`. The working
tree must be clean, the branch must exist in `origin`, and its remote SHA must exactly match local
`HEAD`; commit and push first when any of those checks fails. The helper resolves the branch to one
exact source SHA before dispatch. It generates a unique correlation ID,
dispatches `.github/workflows/wails-portable.yml` with both identities, discovers only the matching
run, reports each native target independently, and waits for the always-running aggregate gate.

The default local output is `build/dist`. Select a different non-existing destination when needed:

```text
task package:all:remote OUTPUT=build/portable-<name>
```

The helper refuses to replace an existing output path. Downloads remain in a quarantined temporary
directory until their correlation ID, source SHA, exact four-target set, archive names, manifests,
and checksums all verify. Only then is the complete directory exposed atomically as `OUTPUT`.

If local waiting is interrupted after dispatch, the helper makes a bounded attempt to cancel the
remote workflow and preserves the original cancellation error and observed target records. Check
the reported workflow URL when remote cancellation cannot be confirmed.

## CI artifacts and acceptance evidence

The portable workflow uses four independent native jobs with fail-fast disabled:

| Target | Runner | Verified per-target artifact |
|---|---|---|
| `windows/amd64` | `windows-2025` | `portable-windows-amd64` |
| `windows/arm64` | `windows-11-arm` | `portable-windows-arm64` |
| `linux/amd64` | `ubuntu-24.04` | `portable-linux-amd64` |
| `linux/arm64` | `ubuntu-24.04-arm` | `portable-linux-arm64` |

Each job checks out the aggregate request's exact clean source SHA, installs pinned Go/Node tools
and native prerequisites, packages with pinned Task, validates PE or ELF identity and exact archive
inventory, launches the extracted application on the matching host, observes its player endpoint
and secure-store state, closes it, checks resource cleanup, and uploads only the verified archive
and sidecar. Windows also validates native locked/denied/unavailable error classification. Linux
runs a second packaged-app journey without Secret Service, requires the explicit fail-closed status,
and proves local session and player synchronization remain available. A build alone is not
publication evidence.

The always-running aggregate job first requires all native jobs to be successful. It downloads the
four `portable-*` artifacts, rejects any missing, duplicate, or extra file, recomputes every
checksum, and writes `aggregate-index.json`. The index joins `schemaVersion`, `correlationID`,
`sourceRevision`, and four records containing `target`, `archiveName`, and `checksum`.

Only a successful join uploads the combined GitHub Actions artifact named
`fallout-terminal-portable`. It contains exactly the four canonical archives, their four checksum
sidecars, and `aggregate-index.json`; these files are the CI evidence consumed and revalidated by
`task package:all:remote`.

## Publish a tagged release

Pushing a semantic version tag triggers one five-target release transaction automatically:

```text
git tag v1.2.3
git push origin v1.2.3
```

Accepted tags use `vMAJOR.MINOR.PATCH` with an optional prerelease suffix, for example
`v1.2.3-beta.1`. A suffix marks the GitHub Release as a prerelease. The portable workflow accepts
all four matching-runner Windows/Linux packages, invokes the reusable macOS checksum workflow at the
same tag SHA, and requires an unsigned `Fallout-Terminal-arm64.dmg` plus a sidecar that matches its
SHA-256 exactly. Developer ID, hardened runtime, notarization, stapling, Gatekeeper, and macOS
release secrets are not eligibility requirements. Publication starts only after the
five-target join has checked the exact source revision, every sidecar, and the complete inventory;
a missing, failed, or unverifiable target prevents both publication destinations.

The workflow publishes the exact 11-file joined inventory in two forms:

- GitHub Release assets: `Fallout-Terminal-arm64.dmg`, its `.sha256` sidecar, the four portable
  archives, their four `.sha256` sidecars, and `aggregate-index.json`, published by the
  repository-pinned GoReleaser v2 configuration;
- one generic OCI artifact in GitHub Packages (GHCR), tagged as
  `ghcr.io/<owner>/<repository>:<git-tag>` and associated with the source repository and revision.

GitHub Release publication is owned by `.goreleaser.yaml` with configuration schema `version: 2`
and the pinned `tools/goreleaser` module. GoReleaser skips compilation because the matching native
jobs already built and verified the complete matrix; `release.extra_files` uploads only those 11
joined files and `prerelease: auto` follows the SemVer suffix. It runs through
`go tool -modfile=tools/goreleaser/go.mod goreleaser release`, never a floating action or global CLI.
GoReleaser first creates a draft. The workflow exposes that release only after the matching GHCR
artifact succeeds; if either destination fails, the workflow removes the draft and versioned GHCR
manifest so a partial cross-destination publication is not presented as a successful release.

The GHCR artifact is published by the pinned ORAS tool module using the workflow-scoped
`GITHUB_TOKEN`; no registry secret or globally installed CLI is required. Package visibility follows
the repository/package settings. To retrieve the complete OCI inventory from a checkout with the
pinned tools available:

```text
go tool -modfile=tools/oras/go.mod oras pull ghcr.io/<owner>/<repository>:v1.2.3
```

For a non-publishing acceptance run, dispatch `Wails Portable` with the clean pushed revision,
the normal correlation identifier, and `validate_release=true`. The workflow executes the same
unsigned Darwin build, SHA-256 verification, and exact five-target join, uploads only the short-lived
`fallout-terminal-release-candidate` workflow artifact, and deliberately skips both GoReleaser and
ORAS because no SemVer tag event exists. No macOS signing or notarization secrets are required.

GitHub Releases provide the normal end-user download surface through GoReleaser v2. GitHub Packages
provides the same versioned files as a machine-consumable OCI artifact rather than pretending portable
desktop archives are an npm, Maven, NuGet, or container-image package.

## Fail-closed behavior

Packaging and aggregation never convert partial work into a successful distribution:

- A local target failure exits nonzero and removes or quarantines incomplete or unverified output.
- Invalid, unsupported, or host-mismatched `GOOS`/`GOARCH` values fail before staging is modified.
- Authentication, authorization, ref resolution, workflow dispatch, timeout, cancellation, and
  download errors are reported distinctly without printing credentials or tokens.
- Every native job records its own result; one failure does not hide the other three results, but
  the aggregate gate fails.
- A missing, duplicate, mislabeled, wrong-revision, wrong-machine, or checksum-invalid target makes
  the complete matrix ineligible.
- A failed aggregate keeps downloads quarantined and never exposes a partial `OUTPUT`. Local Docker
  packaging may replace only the repository-owned default or a recognized previous aggregate after
  full verification, with backup/rollback; remote native download still refuses an existing
  destination.

In short: an incomplete or unverifiable matrix **не публикуется**, and every such failure returns a
nonzero **код завершения**. Exit status zero means the requested target archive—or all four joined
archives—has passed every required verification and is present at the reported success path.
