# Data Model: Quality and Tagged Release Workflows

This change introduces no persisted product data and does not alter session JSON, settings, credentials, player state, or protobuf contracts. The models below describe repository build tooling and GitHub Actions state.

## Platform Target

One accepted portable-archive build destination.

| Field | Type | Rules |
|---|---|---|
| `OS` | closed string | Exactly `windows`, `linux`, or `darwin`. |
| `Arch` | closed string | Exactly `amd64` or `arm64`, constrained by the allowed pairs. |
| `Runner` | workflow label | Must be the matching native GitHub-hosted runner. |
| `ArchiveName` | string | Stable and unique across the five targets. |
| `ArchiveFormat` | enum | ZIP for Windows and Darwin; TAR.GZ for Linux. |
| `ExecutablePath` | archive-relative path | Exact executable required for eligibility. |
| `RequiredResources` | ordered paths | Runtime files whose presence is required for eligibility. |

Allowed values are exactly:

| Target | Runner | Archive |
|---|---|---|
| `windows/amd64` | `windows-2025` | `Fallout-Terminal-windows-amd64.zip` |
| `windows/arm64` | `windows-11-arm` | `Fallout-Terminal-windows-arm64.zip` |
| `linux/amd64` | `ubuntu-24.04` | `Fallout-Terminal-linux-amd64.tar.gz` |
| `linux/arm64` | `ubuntu-24.04-arm` | `Fallout-Terminal-linux-arm64.tar.gz` |
| `darwin/arm64` | `macos-15` | `Fallout-Terminal-darwin-arm64.zip` |

`darwin/amd64`, aliases, case variants, and every other pair are invalid.

## Distribution Artifact

The sole publishable unit for one Platform Target.

| Field | Type | Rules |
|---|---|---|
| `Target` | Platform Target | Exactly one allowed target. |
| `Name` | string | Must equal the target’s stable archive name. |
| `SourceRevision` | commit identity | Must be the revision referenced by the tag. |
| `Size` | integer | Greater than zero. |
| `ExecutablePresent` | boolean | Must be true. |
| `ResourcesPresent` | boolean | Must be true for every required runtime resource. |
| `WorkflowArtifactName` | string | `release-<os>-<arch>`; temporary CI transport only. |

Relationships and invariants:

- One Distribution Artifact belongs to one Platform Target, one source revision, and one Tagged Release Run.
- A published release has exactly five Distribution Artifacts with no duplicate target or filename.
- GitHub Release assets are only these archives. Raw executables, checksum sidecars, aggregate indexes, DMGs, verification records, and package-registry copies are forbidden.
- A local checksum sidecar or aggregate index produced by optional `task package:all` is not a Distribution Artifact and cannot enter a Tagged Release Run.

## Target Build Result

The terminal release-matrix result for one Platform Target.

| Field | Type | Rules |
|---|---|---|
| `Target` | Platform Target | Matrix identity. |
| `Status` | state | `pending`, `building`, `archiving`, `eligible`, or `failed`. |
| `Artifact` | optional Distribution Artifact | Present only for `eligible`. |
| `Failure` | optional diagnostic | Present only for `failed`; identifies target and phase without secrets. |

```text
pending -> building -> archiving -> eligible
             |            |
             +------------+-> failed
```

`eligible` requires only successful compilation, non-empty archive creation, executable presence, and required-resource presence. Native UI, dialog, credential-store, player, tunnel, checksum, signing, notarization, and stapling evidence are not fields and cannot affect this state.

## Quality Run

One non-release automation run for a pull request or push to `main`.

| Field | Type | Rules |
|---|---|---|
| `Trigger` | enum | `pull_request` or `main_push`. |
| `SourceRevision` | commit identity | Revision under review. |
| `Checks` | fixed set | Go tests, Go vet, Buf/protobuf formatting, linting, generation-drift and breaking-change checks, generated-code compilation, clean Overseer build, clean player-client build, startup contracts, Wails pin consistency, and clean binding generation. |
| `Status` | state | `running`, `passed`, or `failed`. |
| `ReleaseAssetsPublished` | integer | Always zero. |

Quality Run has no relationship to Target Build Result or Tagged Release Run. It may build the current host as a quality check, but it does not instantiate the five-target matrix and cannot publish a GitHub Release.

Native UI, dialog, player, lifecycle, secure-store, tunnel, and signing checks are optional evidence outside the fixed quality set. An unexecuted optional check is `NOT RUN`, not failed or passed, and cannot change archive availability, Quality Run status, or Tagged Release Run status.

## Tagged Release Run

One create-only attempt for a qualifying tag.

| Field | Type | Rules |
|---|---|---|
| `Tag` | string | `vMAJOR.MINOR.PATCH` with an optional SemVer prerelease suffix; no build metadata. |
| `SourceRevision` | commit identity | Revision referenced by the pushed tag. |
| `Prerelease` | boolean | True only when the tag contains a valid prerelease suffix. |
| `ExistingRelease` | boolean | Must be false before matrix execution and immediately before publication. |
| `BuildResults` | map target to Target Build Result | Exactly five terminal results before publication. |
| `PublicationEntrypoint` | constant | CI-owned `release:publish` Task command. |
| `Publisher` | constant | Repository-pinned GoReleaser only, invoked by `release:publish`. |
| `Status` | state | See transitions below. |

```text
preflight -> rejected_existing_release
    |
    +-> rejected_invalid_tag
    |
    +-> building -> ready -> publishing -> published
           |                   |      |
           +-> failed          |      +-> failed_without_release
                               +-> manual_recovery_required
```

Validation rules:

- Invalid tags and existing releases are rejected before any target build begins.
- `ready` requires five `eligible` results from the same tag revision and exactly the five stable filenames.
- Publication rechecks `ExistingRelease`; true prevents GoReleaser invocation.
- `published` requires one GitHub Release containing exactly five archives.
- `failed_without_release` means GoReleaser failed and a read-only lookup found no release; automation reports that the same tag can be rerun immediately.
- `manual_recovery_required` means publication created a partial release before failing. Automation leaves it unchanged and reports manual deletion plus same-tag rerun instructions.
- A rerun while any release for the tag exists transitions to `rejected_existing_release`; after a maintainer deletes the partial release, the same tag may start a new run.
- No automatic rollback, replacement, append, asset deletion, second publication destination, or resume state exists.
- Live acceptance is one Tagged Release Run for a maintainer-approved unused SemVer prerelease tag after static/local validation and commit; a successful release is preserved as acceptance evidence.

## Local Package-All Run

An optional maintainer-initiated Docker aggregate from the current checkout.

| Field | Type | Rules |
|---|---|---|
| `Invocation` | constant | `task package:all [OUTPUT=<directory>]`. |
| `Host` | Platform Target | Supported local `darwin/arm64` host. |
| `DockerTargets` | fixed set | Four Windows/Linux targets. |
| `DarwinBundle` | path | Locally built application bundle. |
| `Output` | local directory | May contain local checksums, index metadata, and extracted payloads. |
| `Status` | state | `building`, `verified`, or `failed`. |

Local Package-All Run is not a CI entity, is never consumed by Tagged Release Run, and supplies no GitHub Release eligibility evidence. Its Docker-specific checksums, index, atomic local replacement, and recovery behavior may remain because they are scoped to a local filesystem rather than automated release publication.

## Existing Product Models

The following remain unchanged:

- platform storage profiles and native user-data locations;
- secure credential backends and fail-closed public-access behavior;
- portable session JSON version 1;
- player and private desktop protobuf contracts;
- application lifecycle, listener, and tunnel ownership.
