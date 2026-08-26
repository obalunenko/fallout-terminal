# Contract: Tag-Only Release Matrix

## Trigger and preflight

`.github/workflows/wails-portable.yml` is entered only by pushed tags matching the lightweight candidate filter `v*`. It has no branch, pull-request, schedule, reusable-call, or workflow-dispatch trigger.

Before any matrix job starts, preflight:

1. validates the tag as `vMAJOR.MINOR.PATCH` with an optional SemVer prerelease suffix;
2. rejects leading-zero numeric version or prerelease identifiers, empty identifiers, build metadata, and unrelated `v*` strings;
3. binds the run to the pushed tag revision; and
4. performs a paginated read-only lookup across published, prerelease, and draft releases and fails if any release for the tag already exists, including a partial release.

The five matrix jobs depend on successful preflight. Invalid tags and existing releases therefore consume no native package runners.

Examples:

| Tag | Result |
|---|---|
| `v1.2.3` | stable release candidate |
| `v1.2.3-beta.1` | prerelease candidate |
| `v0.0.0-rc.1` | prerelease candidate |
| `v01.2.3` | rejected |
| `v1.2.3-01` | rejected |
| `v1.2.3+build.1` | rejected |
| `vnext` | rejected |

## Exact native matrix

| Target | Runner | Build command | Workflow artifact |
|---|---|---|---|
| `windows/amd64` | `windows-2025` | `task package GOOS=windows GOARCH=amd64` | `release-windows-amd64` |
| `windows/arm64` | `windows-11-arm` | `task package GOOS=windows GOARCH=arm64` | `release-windows-arm64` |
| `linux/amd64` | `ubuntu-24.04` | `task package GOOS=linux GOARCH=amd64` | `release-linux-amd64` |
| `linux/arm64` | `ubuntu-24.04-arm` | `task package GOOS=linux GOARCH=arm64` | `release-linux-arm64` |
| `darwin/arm64` | `macos-15` | `task package GOOS=darwin GOARCH=arm64` | `release-darwin-arm64` |

Each job checks out the exact tagged revision, installs only its build prerequisites, invokes Task through `tools/task`, performs minimal archive inspection, and uploads only its one stable archive. Matrix fail-fast is disabled so each target reports its result, but publication requires all five matrix entries to succeed.

The matrix does not invoke `task package:all`, Docker, a separate macOS workflow, or any raw Go/Wails build path.

## Release eligibility

Automated eligibility consists only of:

- successful target compilation and archive creation;
- non-zero archive size;
- expected executable presence; and
- required-resource presence.

The tag workflow does not run or require:

- native window or UI automation;
- dialog or external-link journeys;
- credential-store checks;
- player, listener, or tunnel journeys;
- checksum sidecars, aggregate indexes, or external verification records;
- executable architecture inspection beyond successful matching-host compilation;
- codesign, signing, hardened runtime, notarization, stapling, or Gatekeeper;
- Docker aggregate builds; or
- multi-browser checks.

These checks may remain available as unit tests, optional manual evidence, or separately dispatched non-release checks. They never change Target Build Result eligibility.

## Publication inventory

The publication job depends on all five matrix entries and downloads their workflow artifacts into `combined/`. A network-free repository check requires exactly:

```text
Fallout-Terminal-windows-amd64.zip
Fallout-Terminal-windows-arm64.zip
Fallout-Terminal-linux-amd64.tar.gz
Fallout-Terminal-linux-arm64.tar.gz
Fallout-Terminal-darwin-arm64.zip
```

Missing, duplicate, empty, nested, or extra entries fail before publication. In particular, `.sha256`, `aggregate-index.json`, DMGs, raw executables, and verification JSON are rejected.

## Create-only GoReleaser publication

Only the publication job receives `contents: write`; it does not receive `packages: write`. Immediately before GoReleaser invocation, it repeats the read-only existing-release lookup and fails if a release now exists.

The workflow's only publication entrypoint is:

```text
go tool -modfile=tools/task/go.mod task release:publish
```

The `release:publish` Task command invokes:

```text
go tool -modfile=tools/goreleaser/go.mod goreleaser release --clean --config .goreleaser.yaml
```

`.goreleaser.yaml`:

- uses schema version 2 and the repository-pinned tool;
- skips compilation because the native matrix owns builds;
- disables GoReleaser-generated checksum assets;
- lists exactly the five files under `release.extra_files`;
- uses `draft: false` so GoReleaser both creates and exposes the release without a second mutator;
- sets `prerelease: auto`;
- does not enable `replace_existing_artifacts` or reuse an existing draft; and
- defines no publisher or upload beyond the single GitHub Release.

The workflow contains no `gh release create`, release update/PATCH, asset append, asset replacement, release deletion, ORAS, GHCR, or GitHub Packages operation. Read-only GitHub queries used for refusal and diagnostics are not publishers.

## Failure and manual recovery

- A failed target prevents publication.
- An invalid five-file inventory prevents GoReleaser invocation.
- An existing release at either refusal check prevents the matrix or publisher, respectively.
- After GoReleaser fails, an always-running read-only diagnostic looks up the tag across all release states.
- If no release exists, the diagnostic reports the publication error and that the same tag may be rerun immediately.
- If GoReleaser leaves a partial release, the diagnostic reports: delete that release manually, then rerun the same tag.
- The workflow never deletes, rolls back, replaces, or appends to the partial release.
- A rerun while the partial release exists fails preflight unchanged.

Per-tag workflow concurrency prevents overlapping automated runs for the same tag. Publication succeeds only when one GitHub Release exposes exactly the five archives from the tagged revision.

## Non-publishing validation

The PR/main quality workflow runs static and network-free contract tests that cover:

- accepted and rejected tag fixtures;
- exact trigger and permission shape;
- five and only five runner/target/command rows;
- minimal valid and invalid archive fixtures;
- exact and malformed publication inventories;
- existing-release refusal before matrix and before GoReleaser;
- exactly five GoReleaser extra files and disabled generated checksums;
- the sole `release:publish` Task-to-pinned-GoReleaser ownership chain;
- absence of release mutation, ORAS, package publication, DMG, and rollback commands; and
- distinct no-release immediate-rerun and partial-release delete-then-rerun diagnostics.

The release workflow has no `workflow_dispatch` validation mode, so these tests cannot accidentally start the native matrix or publish a release.

## Live prerelease acceptance

After implementation, static/non-publishing fixtures, local `task build`, explicit `darwin/arm64` packaging, and commit succeed, a maintainer supplies and approves one unused SemVer prerelease tag. Pushing that tag runs the real five-target workflow. Acceptance requires one GitHub Release containing exactly the five governed archives; the successful prerelease is preserved as evidence. A failed run follows the same no-release or manual partial-release recovery contract and performs no automated cleanup.
