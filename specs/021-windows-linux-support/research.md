# Research: Constitution v8 Archive-Availability Convergence

## Decision: Define platform support as portable archive availability

**Decision**: For this feature, a platform is supported when the matching build host can produce the governed unsigned portable archive containing its executable and required resources. Native window, dialog, player, lifecycle, secure-store, tunnel, and signing journeys are optional evidence and do not gate feature completion, quality CI, platform support, or tagged releases.

**Rationale**: This is a hobby-project distribution feature whose primary value is simple, complete downloads. Constitution v8.0.0 deliberately separates archive availability from claims that every native operating-system integration journey was executed for a revision.

**Alternatives considered**:

- Require matching-host native journeys before feature completion: rejected because constitution v8 makes that evidence optional.
- Treat optional checks as passing when unavailable: rejected because unexecuted evidence must be reported honestly as `NOT RUN`.
- Remove the existing native checks: rejected because they remain useful when a maintainer chooses to run them.

## Decision: Keep quality and release automation separate

**Decision**: Retain `.github/workflows/wails-cross-platform.yml` as the non-release workflow for pull requests and pushes to `main`, and make `.github/workflows/wails-portable.yml` the only SemVer-tag release workflow. Consolidate the useful macOS CI checks into the quality workflow, rewrite the portable workflow so it no longer calls the reusable macOS workflow, then remove `.github/workflows/wails-macos.yml` and its active `scripts/proto-check.sh` reference as one ordered cutover.

**Rationale**: Constitution v8.0.0 requires the broader Go, Buf/protobuf, frontend, startup, Wails-pin, and binding checks to continue without making them release gates. Separate trigger and permission scopes make that boundary visible: quality runs are read-only and cannot publish; release runs do only the five minimal packages and one GitHub Release.

**Alternatives considered**:

- Delete PR/main CI entirely: rejected because the approved decision explicitly retains project quality automation.
- Run quality jobs inside the tag workflow: rejected because strict quality checks must not gate hobby-project releases.
- Keep the standalone macOS workflow: rejected because its current PR/main CI duplicates the quality owner and its callable DMG job is superseded.

## Decision: Use the same explicit package entrypoint on five matching runners

**Decision**: Build `windows/amd64` on `windows-2025`, `windows/arm64` on `windows-11-arm`, `linux/amd64` on `ubuntu-24.04`, `linux/arm64` on `ubuntu-24.04-arm`, and `darwin/arm64` on `macos-15`. Every matrix row invokes `task package GOOS=<os> GOARCH=<arch>` through `tools/task`.

**Rationale**: These matching runner labels already exist in the repository, and the current Go build boundary owns preparation, target validation, resources, Wails invocation, and archive behavior. Extending the accepted target parser and package plan to Darwin gives all five jobs one explicit contract without copying build policy into workflow YAML.

**Alternatives considered**:

- Cross-compile all targets from Linux: rejected because native Wails build dependencies require matching hosts for the governed package entrypoint.
- Call `go build` or Wails directly in YAML: rejected because that bypasses the repository’s ordered frontend, binding, resource, and package policy.
- Call local `task package:all`: rejected because its Docker outputs are a convenience, not native release inputs.

## Decision: Package Darwin as an unsigned portable ZIP

**Decision**: Explicit `darwin/arm64` packaging assembles the existing `Fallout Terminal.app` metadata, executable, icon, notices, and bundled sessions without a signing step, then writes `Fallout-Terminal-darwin-arm64.zip` through the common archive boundary.

**Rationale**: The application bundle is the runnable macOS unit and must remain intact. ZIP preserves the bundle hierarchy and executable mode while giving Darwin the same target command, stable archive name, workflow transport, eligibility checks, and GitHub Release treatment as Windows and Linux.

**Alternatives considered**:

- DMG: rejected because it creates a macOS-specific tagged-release path.
- Raw Mach-O executable: rejected because it omits required application metadata and resources.
- Ad-hoc or Developer ID signing, notarization, or stapling: rejected because the approved release contract is unsigned.
- `darwin/amd64`: rejected because it is outside the exact five-target matrix.

## Decision: Limit tagged-release eligibility to four observable outcomes

**Decision**: A target is release-eligible only when compilation succeeds, the archive exists and is non-empty, the target executable is present, and required resources are present. Implement a small network-free inspection seam for those checks instead of invoking the current checksum-heavy verifier or native smoke scripts.

**Rationale**: The existing `VerifyArtifact` and native workflows prove considerably more than the hobby release requires. Keeping those checks available for local, quality, or manual evidence preserves engineering value without making UI, dialogs, credentials, players, tunnels, checksums, or platform signing part of tag success.

**Alternatives considered**:

- Reuse the full current verifier: rejected because it requires checksum sidecars, manifest/hash checks, and architecture inspection beyond the approved release gate.
- Remove the richer tests and smoke scripts: rejected because only their release-gating role is removed.
- Validate only file existence: rejected because executable and required-resource presence are explicit release conditions.

## Decision: Keep pinned GoReleaser as the sole GitHub Release publisher

**Decision**: Retain `tools/goreleaser` and `.goreleaser.yaml`. The workflow enters publication through the CI-owned `release:publish` Task command, which invokes pinned GoReleaser. It lists all release states, including drafts, before starting the matrix and again immediately before publication; the configuration disables generated checksums, lists exactly five prebuilt archives, uses `draft: false`, does not enable `replace_existing_artifacts`, and uses no second publisher or post-publication mutator.

**Rationale**: GoReleaser is already pinned and accepted by constitution v8.0.0. Its v2.18 GitHub client can otherwise update a non-immutable existing release, so create-only behavior must be enforced by the workflow’s refusal checks and by never invoking it on an existing tag release. A second check closes the long build-window gap, while per-tag workflow concurrency prevents overlapping automated runs.

**Alternatives considered**:

- GitHub CLI as publisher: rejected because the approved decision retains GoReleaser as sole publisher.
- Direct GoReleaser invocation from workflow YAML: rejected because release automation must enter through the canonical pinned Task graph.
- `replace_existing_artifacts: true`: rejected because it directly violates create-only publication.
- Reuse an existing draft or append missing files: rejected because every existing release, including a partial draft, must block automation.
- Create with one tool and expose with another: rejected because that creates a second release mutator.

## Decision: Publish only the five portable archives

**Decision**: Each matrix job transports only its archive to the publication job, and GoReleaser receives exactly the five stable filenames. Remove ORAS/GitHub Packages, DMG assets, checksum sidecars as release assets, aggregate indexes, raw executables, verification records, and any joined multi-destination inventory.

**Rationale**: The release archive is the complete user-downloadable unit. Exact filename and inventory validation already proves that all targets are present; extra release assets create duplicate or implementation-facing choices without improving the hobby-project contract.

**Alternatives considered**:

- Keep SHA-256 sidecars: rejected because the accepted GitHub Release contains exactly five assets.
- Keep `aggregate-index.json`: rejected because the exact five-file inventory and workflow dependency graph replace it.
- Keep GHCR through ORAS: rejected because GitHub Releases are the sole automated destination.

## Decision: Report partial publication and require manual recovery

**Decision**: After GoReleaser fails, an always-running read-only diagnostic looks up the tag. If no release exists, it reports that the same tag can be rerun immediately. If a partial release exists, it reports that a maintainer must delete that release and then rerun the same tag. The workflow never deletes a release or asset, and every rerun refuses to invoke GoReleaser while any release for that tag exists.

**Rationale**: A single-destination hobby release does not justify destructive rollback automation. Distinguishing the two failure states avoids telling maintainers to delete a nonexistent release while preserving the explicit, recoverable create-only procedure for partial publication.

**Alternatives considered**:

- Automatically delete the partial release: rejected by the approved recovery decision.
- Resume or append missing assets on rerun: rejected because reruns cannot modify an existing release.
- Treat a partial release as successful: rejected because the release contract requires all five assets.

## Decision: Retain only the local Docker aggregate

**Decision**: Keep `task package:all`, `package-all-docker`, `internal/buildtool/docker.go`, and `build/docker/Dockerfile.package` as optional local tooling. Move every dependency retained by Docker—result and record types, artifact interface, target helpers, cloning and validation functions, directory verifier, constants, and index structures—into a local-only boundary with its regression tests, then remove `package:all:remote`, the remote `package-all` CLI/action implementation, `aggregate.go`, `release:local`, the `release-candidate` implementation, and ORAS.

**Rationale**: The local command remains useful to maintainers and was explicitly retained, but it must have no dataflow edge into either CI workflow. Separating its shared types allows the obsolete remote coordinator and joined release implementation to be removed cleanly without rewriting the Docker convenience.

**Alternatives considered**:

- Delete all Docker aggregation: rejected because the approved decision retains it locally.
- Keep the remote implementation dormant: rejected by the complete-cutover rule and the explicit removal decision.
- Reuse the local aggregate as a release join: rejected because tagged releases must use five matching native runners directly.

## Decision: Validate contracts statically, then publish one acceptance prerelease

**Decision**: Use table-driven Go fixtures and static repository acceptance tests to validate strict tag syntax, exact workflow triggers, five matrix rows, archive eligibility, exact publication inventory, Task-to-GoReleaser ownership, existing-release refusal, and manual recovery wording. Do not add `workflow_dispatch` to the release workflow. After implementation, static checks, local native build/package validation, and commit all succeed, push one maintainer-approved unused SemVer prerelease tag and preserve its real five-archive GitHub Release as acceptance evidence.

**Rationale**: Static fixtures provide fast deterministic regression coverage, while the approved live prerelease proves the matching-host matrix and create-only publisher work end to end in the real repository. Deferring the live run until every local gate passes limits runner use and avoids publishing from an unvalidated revision.

**Alternatives considered**:

- A non-publishing release workflow dispatch: rejected because the five-target release matrix is tag-only.
- Static validation only: rejected because the approved independent acceptance requires one real prerelease publication.
- Automatic cleanup of the successful acceptance release: rejected because the prerelease is retained as acceptance evidence and release automation never deletes releases.
- Rely only on manual YAML review: rejected because regressions in triggers, inventory, and forbidden commands are machine-checkable.

## Decision: Preserve completed work outside a fresh delta task list

**Decision**: Preserve the superseded `T001` through `T067` record in `tasks-history.md` and keep only the active unchecked constitution-v8 delta beginning at `T068` in `tasks.md`.

**Rationale**: The feature has extensive implemented history that must remain auditable, but mixing checked historical items with the new convergence delta obscures remaining work. Continuing task IDs also prevents existing Companion completion events from auto-completing newly generated tasks.

**Alternatives considered**:

- Leave the superseded tasks in the active list: rejected because the user requested a fresh delta-focused queue.
- Restart IDs at `T001`: rejected because recorded completion events already use those IDs.
- Delete the prior task list: rejected because it would discard implementation history.
