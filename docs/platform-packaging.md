# Build and release packaging

The repository exposes Wails-aware build operations through Task. Bootstrap the pinned tools with
`make tools`; use `make help` to discover bootstrap commands and `task --list` for project work.

## Command reference

The normal development and verification entrypoints are:

```text
task dev
task prepare
task build
task package
task release:tag
task deps
task fmt
task vet
task lint
task test
task proto:generate
task proto:check
task proto:breaking
task bindings:check
task browser:test
task check
```

Manual macOS signing remains separate: `task release:macos:preflight` checks credentials and
`task release:macos:signed` creates a Developer ID/notarized package. Automated portable release
publication uses `task release:publish` and does not require signing credentials.

Create a stable release tag from a clean, committed checkout with:

```bash
task release:tag
```

The task uses the pinned `svu` tool and Conventional Commit history to propose the next version,
falling back to a patch increment when no commit requests a larger change. When the latest tag is
from an older major line, it first advances to the root Go module's major. The candidate must pass
the same strict release-tag validation as CI. Task then asks for confirmation before creating the
local tag and pushing that exact tag to `origin`; a failed push removes the newly created local tag.

Pass a SemVer prerelease identifier to propose a prerelease tag:

```bash
task release:tag PRERELEASE=rc.1
```

## Package one target

Set both `GOOS` and `GOARCH` and run the common entrypoint on its matching native host:

```bash
task package GOOS=windows GOARCH=amd64
task package GOOS=windows GOARCH=arm64
task package GOOS=linux GOARCH=amd64
task package GOOS=linux GOARCH=arm64
task package GOOS=darwin GOARCH=arm64
```

The governed output is exactly five non-empty archives:

| Target | Release asset |
|---|---|
| `windows/amd64` | `Fallout-Terminal-windows-amd64.zip` |
| `windows/arm64` | `Fallout-Terminal-windows-arm64.zip` |
| `linux/amd64` | `Fallout-Terminal-linux-amd64.tar.gz` |
| `linux/arm64` | `Fallout-Terminal-linux-arm64.tar.gz` |
| `darwin/arm64` | `Fallout-Terminal-darwin-arm64.zip` |

Each matching-host package is prepared from one locked `frontend/` install and two independent Vue
production builds. The privileged `frontend/overseer/dist` filesystem is embedded only into the
Wails desktop host; the public `frontend/client/dist` filesystem is embedded only into the Player
HTTP server. A package must contain runtime HTML/JavaScript/CSS, the reviewed Fixedsys font, Player
sounds/static files, demo session/player configuration, platform icons/metadata, and
`THIRD_PARTY_NOTICES.md`; it must not contain TypeScript/SFC sources, test fixtures, candidate or
legacy bundles, Wails bindings as public Player assets, provider executables, or development URLs.

Run `scripts/dependency-license-check.sh` before accepting a package. It verifies both the shipped
Go runtime/build-tool inventory and the Vue runtime closure against the reviewed notices. Then use
the exact matching verifier: `scripts/verify-macos-app.sh`, `scripts/verify-windows-package.ps1`, or
`scripts/verify-linux-package.sh`.

Evidence is target- and class-specific. On a non-matching host, record that package/startup row as
`NOT RUN`; cross-compilation and another host's archive do not replace it. Browser Playwright and
visual snapshots are browser evidence, not Wails/native startup evidence. Native UI, Accessibility,
secure-store, provider-credential, signing, notarization, and stapling journeys are `NOT RUN` when
their prerequisites are unavailable, with the host and reason recorded; they are never inferred
from a static package scan.

Windows archives contain `Fallout Terminal.exe` and resources, Linux archives contain the
executable `Fallout Terminal` and resources, and the Darwin ZIP contains the complete unsigned
`Fallout Terminal.app` bundle. The user-facing `RUNNING.md` launch guide remains in the repository
and may be linked from release notes, but is not part of the governed schema-v2 payload because
v2.0.0 validates that inventory exactly. Per-target release eligibility checks the expected asset
name, non-empty archive, executable, and required resources. Runtime GUI, dialog, credential,
player, tunnel, and signing journeys are useful optional evidence, but are not release eligibility
gates.

The published v2.1.0 binary has a conflicting seven-file exact inventory. A manually installed
v2.1.0 therefore requires one manual application replacement to reach the forward-fix line; one
archive cannot satisfy both its seven-file contract and v2.0.0's six-file contract.

## Runtime self-update contract

Self-update is enabled only for a versioned application running from a packaged portable layout.
Development, unversioned, and unpackaged runs do not initialize the update provider or make a
production update request. An eligible packaged run checks once, in the background, after the
Overseer interface can present status; failure never prevents local startup or session use.

Discovery uses the public GitHub Releases API and accepts only a strictly newer eligible v2 release.
Its asset inventory must be exactly the five archives listed above, with no missing, duplicate,
empty, or extra asset. Every asset must have GitHub state `uploaded` and a valid
`sha256:<64 hex digits>` digest. The running target must match exactly one asset. GitHub's digest
metadata verifies the bytes after consent; no checksum sidecar, updater manifest, installer, DMG,
raw executable, or additional release asset is part of the contract. Stable installations see only
stable releases; prerelease installations may see a newer prerelease or the next stable release.

The Overseer controls two separate consent boundaries:

1. An available release can be accepted or deferred. Deferring performs no download and suppresses
   the offer only for the current run.
2. After download, digest verification, manifest/package validation, and staging complete, restart
   can be approved or postponed. Postponing keeps the staged unit ready, leaves the application
   usable, and allows the restart prompt to be reopened without another download.

Staging and replacement require a fully extracted portable installation in a writable location.
The application must be able to create and rename sibling paths on the same volume as the installed
unit. Running from an archive, a read-only mount, or a system-owned directory without sufficient
permissions cannot be updated in place. Move or re-extract the package to a user-writable location,
or correct the directory permissions, and retry on a later launch. Windows and Linux replace the
complete portable directory; macOS replaces the complete `.app` bundle. Session files, player
configurations, credentials, preferences, Documents, and application-support data stay outside that
replacement boundary.

On restart approval, the application copies a temporary helper outside the installed unit, records
non-sensitive recovery state, and performs the normal ordered shutdown. The helper waits for the
parent to exit, renames the installed unit to a sibling backup, promotes the staged unit, and
relaunches it. If promotion or relaunch fails, it restores and relaunches the last working unit when
possible; the next launch reports a sanitized failure stage and recovery action. Attempt-owned
staging, backup, and helper remnants are cleaned up best-effort. Update errors must never include
credentials, provider authorization, user-document content, or local paths in the Overseer contract.

For check, download, verification, disk-space, permission, or package-shape failures, continue using
the installed application, correct the reported condition, and retry on a later launch. Do not edit
or replace assets on a published release. Before publication, a source rollback is allowed; after a
self-update-capable release is published, every correction uses a new, higher strict v2 forward-fix
tag and a newly governed five-archive release.

## Release version identity

The application module is `github.com/obalunenko/Fallout-Terminal/v2`. The tag major must match the
root Go module major, so release preflight accepts strict stable or prerelease v2 tags and rejects
other majors, build metadata, leading-zero numeric components, and malformed prerelease identifiers.

Preflight preserves the raw tag as the GitHub release identity and removes only its leading `v` to
produce the canonical build value. Thus `v2.0.0` produces `VERSION=2.0.0`, while
`v2.0.0-rc.1` produces `VERSION=2.0.0-rc.1`. A canonical `VERSION` never contains a leading `v`.
Preflight exports that value once; every native package job passes it unchanged to the common
packager and to release inspection. The same value is linker-injected into the Go executable and
renders the target-isolated Darwin or Windows metadata. Checked-in `.tmpl` files remain immutable
inputs and are not independent version authorities.

Numeric-only native fields are derived from that canonical value; they are never separately set:

| Mode | Executable and human-readable metadata | Darwin numeric core | Windows four-part numeric |
|---|---|---|---|
| Stable tag `v2.0.0` | `2.0.0` | `2.0.0` | `2.0.0.0` |
| Prerelease tag `v2.0.0-rc.1` | `2.0.0-rc.1` | `2.0.0` | `2.0.0.0` |
| Local package with empty `VERSION` | `development` | `0.0.0` | `0.0.0.0` |

On Darwin, the human-readable value is `CFBundleShortVersionString` and the numeric core is
`CFBundleVersion`. On Windows, string `FileVersion` and `ProductVersion` retain the full canonical
stable or prerelease value; fixed file/product versions and the manifest assembly version use the
four-part numeric value. Linux has no additional platform version field. Empty `VERSION` is an
explicit local non-release mode: it keeps `task build` and `task package` usable, but a
`development` package cannot pass tagged-release inspection.

## Inspect every package before upload

Each matrix job packages and inspects its archive on the matching native host before its upload
step can run. The workflow uses the canonical preflight output for both operations:

```bash
VERSION="$(go run ./cmd/build validate-release-tag --tag v2.0.0-rc.1)"
task package GOOS=darwin GOARCH=arm64 VERSION="$VERSION"
go run ./cmd/build inspect-release-archive \
  --target darwin/arm64 \
  --archive build/dist/Fallout-Terminal-darwin-arm64.zip \
  --version "$VERSION"
```

Use the corresponding target and archive name for each of the other four native jobs. Inspection
extracts the package, runs its executable with `--version`, and requires exactly the canonical value
plus one newline on standard output, no standard-error output, and a successful exit before Wails
or application services start. Darwin inspection also compares both plist version values; Windows
inspection compares both string values, both fixed versions, and the assembly version. A missing,
malformed, `development`, or mismatched value fails the target job before upload.

If inspection reports a mismatch, do not edit a staged plist, Windows resource, checked-in
template, or archive by hand, and do not upload the rejected archive manually. Re-run tag preflight,
confirm that its canonical output is the exact non-empty `VERSION` passed to that target, rebuild
the target from the tagged commit on its matching native host, and rerun the same inspection command.
If a correction requires a source change after a complete release exists, preserve the published
tag and release and use a new strict v2 forward-fix tag.

Version inspection adds no file to an archive and no release asset. The five archive names, their
internal executable/resource inventory, the flat upload inventory, create-only publication, and the
partial-release recovery procedure below remain unchanged.

## Optional local aggregate

On an Apple Silicon Mac, `task package:all` can create a local developer convenience output from
the current checkout. Docker builds only the four Windows/Linux targets; Darwin is packaged on the
host. Choose another destination with `task package:all OUTPUT=build/portable-local`.

This aggregate is local-only, may include unpacked verification directories, and never runs in CI.
It is not a release candidate, upload inventory, or source of release publication state. A failure
returns a nonzero exit status and preserves an already verified destination.

## Quality and tag workflows

Pull requests and pushes to `main` run the read-only quality workflow. It calls `task ci:quality`
and cannot create releases or upload governed release assets.

A push of a strict SemVer tag on the current major line, such as `v2.0.0` or `v2.0.0-rc.1`, starts
the tag-only workflow. The tag major must match the root Go module major; the current preflight
accepts only `v2`. Its five native jobs use the common packaging entrypoint, inspect one archive
each, and upload one artifact each. The publish job downloads the complete flat inventory, refuses
missing, duplicate, empty, or extra files, and calls `task release:publish`. That task runs the
repository-pinned `pinned GoReleaser` module and publishes exactly five assets. GoReleaser is
create-only: an existing release, including a draft, is refused before packaging and checked again
before publication. Prerelease suffixes produce GitHub prereleases automatically.

The workflow never deletes, replaces, or edits a GitHub release. It also never publishes checksums,
an installer, or a package-registry copy. Release validation uses static fixtures locally; the sole
end-to-end acceptance is an explicitly approved unused prerelease tag on a committed revision.

## Partial release recovery

If publication fails before GitHub creates a release, fix the cause and rerun the same tag immediately.
If GitHub contains a partial release, do not rerun yet: **Delete the partial release manually**, verify
that no release remains for the tag, and then rerun the same tag. Keep the tag itself unless a
maintainer has approved moving it to a different committed revision.

An incomplete or unverifiable inventory **не публикуется**. Every rejected target, invalid tag,
existing release, or publisher failure returns a nonzero **код завершения**.
