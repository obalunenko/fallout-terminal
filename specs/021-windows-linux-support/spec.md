# Feature Specification: Windows and Linux Desktop Support

## Clarifications

### Session 2026-08-26

- Q: How should maintainers package all supported platforms? → A: Separate per-target commands run on compatible operating-system hosts and coordinated by the version-tag workflow; no local or Docker aggregate is required for release.
- Q: What distribution form must each target-specific package command produce? → A: A portable archive containing the target executable and required resources.
- Q: Which repository task runner owns developer and packaging commands? → A: Migrate project automation from Make to Go Task, pin Task like the other isolated Go tools, and leave Make only as the bootstrap that installs every Go tool declared under `tools/`.
- Q: May Make retain a help target after the Task migration? → A: Yes. `make help` is a non-mutating discovery target that documents `make tools` and directs maintainers to `task --list`; it must not proxy project workflows.
- Q: What must a version tag publish for all supported targets? → A: One unsigned portable archive per supported target, built consistently and attached to a single GitHub Release; signing and notarization are not required.
- Q: What should each tagged release contain for every supported target? → A: One portable archive containing the executable and required resources.
- Q: Which target matrix must every tagged release build? → A: Windows amd64 and arm64, Linux amd64 and arm64, and macOS arm64.
- Q: When should the five-target build matrix run? → A: Version tags only.
- Q: Which tags should be treated as release tags? → A: Semantic-version tags in the form `vMAJOR.MINOR.PATCH`, including optional prerelease suffixes such as `v1.2.3-beta.1`.
- Q: What should happen if the tag already has a GitHub Release? → A: The workflow fails and leaves the existing release and its assets unchanged.

### Session 2026-08-27

- Q: How are FR-012, FR-018, and FR-025 divided without overlap? → A: FR-012 defines the exact five-target matrix, FR-018 requires all five archives to succeed before publication, and FR-025 requires every target to use the same explicit target-aware package entrypoint.
- Q: How are FR-015, FR-029, SC-007, and SC-013 divided without overlap? → A: FR-015 requires the common unsigned target build-and-archive flow, FR-029 prohibits DMG, signing, and notarization-specific release paths, SC-007 measures zero signing and notarization steps, and SC-013 measures the resulting macOS portable archive contract.
- Q: How is a partial GitHub Release recovered after publication fails? → A: The workflow performs no automated rollback, reports manual recovery instructions, and refuses automated reruns until a maintainer deletes the partial release; the maintainer may then rerun the same tag.
- Q: What runs on pull requests and pushes to the main branch if the five-target release matrix is tag-only? → A: A separate non-release quality workflow runs project quality checks, does not run the five-target release matrix, and publishes no release assets.
- Q: What does platform support mean for this hobby-project distribution feature? → A: Support means availability of the governed unsigned portable archive with its executable and required resources; native runtime and operating-system integration journeys are optional, non-gating evidence and are not acceptance claims of this feature.
- Q: How are optional local Docker aggregation and obsolete remote release commands governed? → A: Retain `task package:all` and its Docker implementation only as optional maintainer convenience outside CI and release acceptance; remove `package:all:remote`, `release:local`, and their remote aggregation and joined-release implementations.
- Q: How must tagged publication enter the repository's canonical command graph? → A: The tag workflow invokes a CI-owned `release:publish` Task command, which invokes repository-pinned GoReleaser as the sole GitHub Release publisher.
- Q: Which native build commands must final local validation run? → A: Run `task build` and `task package GOOS=darwin GOARCH=arm64`; validate unavailable matching-host targets through static contracts until the tag workflow builds them natively.
- Q: How must the tagged-release user story be independently accepted? → A: After static and local validation passes, push a maintainer-approved unused SemVer prerelease tag in this repository and verify the real five-archive GitHub Release.
- Q: Which decision does FR-013 limit to four observable conditions? → A: Per-target archive eligibility only; SemVer preflight, exact publication inventory, and existing-release safety remain separate required checks.

## User Scenarios & Testing

### User Story 1 - Obtain the complete archive for every target (Priority: P1)

As a user, I can obtain the Fallout Terminal portable archive matching my operating system and processor, so I receive the packaged executable and required resources without installing the source repository or build tools.

**Why this priority**: Complete, target-identifiable archives are the core distribution value of the feature.

**Independent Test**: Inspect the archive produced for each target and confirm its stable name, non-zero size, expected executable, and required bundled resources without launching native UI or operating-system integrations.

**Acceptance Scenarios**:

1. **Given** a Windows target, **When** its archive is inspected, **Then** the archive name identifies Windows and its processor and the archive contains the Windows executable and required resources.
2. **Given** a Linux target, **When** its archive is inspected, **Then** the archive name identifies Linux and its processor and the archive contains the executable and required resources.
3. **Given** a user selects an archive for the wrong operating system or processor architecture, **When** they consult the filename and release documentation, **Then** they can identify the mismatch before downloading or unpacking it.

### User Story 2 - Preserve project quality outside releases (Priority: P1)

As a maintainer, I can receive the project's established quality results on pull requests and main-branch pushes without running the five-target release matrix or publishing release assets.

**Why this priority**: Simplifying hobby-project releases must not remove the ordinary feedback that protects the existing application and build contracts.

**Independent Test**: Inspect and run the non-release quality workflow on a pull request or `main` push and confirm it performs the configured Go, protobuf, frontend, startup, Wails-pin, and binding checks with read-only permissions and no release assets.

**Acceptance Scenarios**:

1. **Given** a pull request or push to `main`, **When** the quality workflow runs, **Then** it executes the configured Go tests and vet, protobuf checks, clean frontend builds, startup contracts, exact Wails-pin checks, and clean binding generation.
2. **Given** a quality run, **When** its permissions and outputs are inspected, **Then** it has read-only repository access, creates no GitHub Release, and publishes no release asset.
3. **Given** a semantic-version tag, **When** automation is selected, **Then** the non-release quality workflow is not used as a dependency or gate for the five-target release matrix.
4. **Given** an optional native UI, dialog, player, lifecycle, or secure-store check is not run, **When** quality evidence is reported, **Then** the check is marked `NOT RUN` rather than claimed as passing and it does not affect archive support.

### User Story 3 - Publish simple tagged releases for all targets (Priority: P2)

As a maintainer of a hobby project, I can push a version tag and receive one portable archive containing the executable and required resources for every supported target in a single release, without maintaining signing credentials or complex native UI automation.

**Why this priority**: A small project needs dependable downloadable builds without enterprise release ceremony.

**Independent Test**: After static and local validation passes, push a maintainer-approved unused SemVer prerelease tag from the committed implementation, wait for every native target build to finish, and confirm that the resulting real GitHub Release contains exactly one uniquely named, non-empty portable archive per supported target with its executable and required resources.

**Acceptance Scenarios**:

1. **Given** a clean tagged source revision, **When** the release workflow runs, **Then** it builds each supported operating-system and processor target independently using the same target-specific build contract.
2. **Given** a target build succeeds, **When** its release asset is prepared, **Then** the asset is one portable archive whose name identifies the target and whose contents include the executable and required resources.
3. **Given** every target build succeeds and the tag has no existing GitHub Release, **When** the workflow reaches publication, **Then** it creates the tag's GitHub Release with the complete archive set.
4. **Given** any target build or archive preparation fails, **When** the workflow completes, **Then** release publication is withheld and the failed target is clearly reported.
5. **Given** the macOS target is built, **When** its archive is prepared, **Then** it uses the same unsigned target-build and archive flow as Windows and Linux without signing or notarization.
6. **Given** the tag already has a GitHub Release, **When** publication is attempted, **Then** the workflow fails without changing that release or its assets.
7. **Given** publication creates a partial GitHub Release and then fails, **When** the failed run reports its result, **Then** it leaves the partial release unchanged and instructs a maintainer to delete it before rerunning the same tag.
8. **Given** a pull request or push to the main branch, **When** automation runs, **Then** a separate non-release quality workflow reports project quality results without running the five-target release matrix or publishing release assets.

### User Story 4 - Choose the correct distribution (Priority: P3)

As a user or maintainer, I can find supported archive targets, runtime prerequisites, artifact naming rules, launch guidance, data locations, and known platform limitations, so I can select the correct distribution without guesswork.

**Why this priority**: Documentation reduces installation and support friction after the application and release matrix are functional.

**Independent Test**: Give the release documentation and artifact list to a user unfamiliar with the build process and confirm they can choose the correct download and identify its prerequisites, launch guidance, and user-owned data locations without requiring a successful native launch demonstration.

**Acceptance Scenarios**:

1. **Given** the five available Windows, Linux, and macOS artifacts, **When** a user consults the documentation, **Then** they can select the artifact matching their operating system and processor architecture.
2. **Given** a target has an operating-system runtime prerequisite, **When** a user prepares the machine, **Then** the documentation states the prerequisite and how to recognize when it is unavailable.
3. **Given** a user needs to back up or troubleshoot Fallout Terminal, **When** they consult the platform guidance, **Then** they can locate session documents and private application settings without exposing stored secrets.

## Edge Cases

- What happens when a user selects an archive for the wrong operating system or processor architecture?
- How does archive inspection report a missing executable, required resource, or empty file?
- How are packaged read-only resources distinguished from user-owned data that must not enter an archive?
- How are target archives kept uniquely named when every supported target is built from the same tag?
- What happens when one target build fails after other target archives have already completed?
- If a qualifying version tag already has a GitHub Release, the workflow fails without replacing, deleting, or adding assets to that release.
- If publication creates a partial GitHub Release and then fails, the workflow performs no automated rollback, reports instructions to delete the partial release manually, and refuses to modify it on a rerun; after deletion, a maintainer can rerun the same tag.
- If GoReleaser fails without creating a GitHub Release, the workflow reports that the same tag may be rerun immediately and does not instruct the maintainer to delete a nonexistent release.
- A live acceptance tag must be an unused maintainer-approved SemVer prerelease tag; if its run succeeds, the created prerelease remains as repository acceptance evidence rather than being automatically deleted.
- Native launch, UI, dialog, player, lifecycle, secure-store, tunnel, and signing behavior remain existing product concerns outside this archive-availability feature and may be reported as optional `NOT RUN` evidence.

## Requirements

### Functional Requirements

- **FR-001**: The product MUST make governed portable archives available for Windows on arm64, Windows on amd64, Linux on arm64, Linux on amd64, and macOS on arm64.
- **FR-002**: The product MUST document the minimum supported operating-system versions and required system runtime dependencies for each target.
- **FR-003**: Every target artifact MUST include the complete Overseer interface, player interface, bundled demo data, product identity, application icon, and required third-party notices.
- **FR-004**: A user MUST be able to unpack each target archive and locate its packaged executable and required resources without installing the source repository, a compiler, or frontend build tools.
- **FR-005**: This feature MUST preserve existing application behavior without making session, control, player, dialog, lifecycle, or public-access journeys a platform-support or tagged-release acceptance requirement.
- **FR-006**: Native UI, dialog, external-link, player, lifecycle, credential-store, tunnel, and signing checks MUST remain optional and non-gating, and unexecuted checks MUST be reported as `NOT RUN` rather than passing evidence.
- **FR-007**: Every archive MUST contain only packaged read-only application resources and MUST NOT contain user-owned session documents or private application settings.
- **FR-008**: Every archive and automated release output MUST contain zero public-access credentials, plaintext secret fallbacks, or secret-bearing verification records.
- **FR-009**: Archive eligibility and platform support MUST NOT require application startup, native interaction, or shutdown evidence.
- **FR-010**: Maintainers MUST have a documented packaging command for each supported target that runs on a matching operating-system host without requiring source changes between targets.
- **FR-011**: Each produced artifact MUST have a stable, unique name that identifies its operating system and processor architecture.
- **FR-012**: The automated delivery workflow MUST define exactly this five-target build matrix from one clean tagged source revision: Windows amd64, Windows arm64, Linux amd64, Linux arm64, and macOS arm64.
- **FR-013**: Automated per-target archive eligibility validation MUST be limited to successful target compilation, non-empty archive creation, and presence of the executable and required resources; strict native UI, dialog, credential-store, player-journey, signing, and notarization checks MUST NOT gate hobby-project releases.
- **FR-014**: A failed or unverifiable target MUST NOT be reported or published as a successful artifact.
- **FR-015**: The supported macOS target MUST enter the same explicit target-specific unsigned build-and-portable-archive flow as every Windows and Linux target.
- **FR-016**: User-facing documentation MUST explain target selection, prerequisites, launch steps, data locations, credential-service expectations, and platform-specific troubleshooting.
- **FR-017**: Documentation and archive diagnostics MUST provide actionable information for platform or architecture mismatches, missing archive contents, and unavailable runtime prerequisites.
- **FR-018**: The version-tag workflow MUST withhold GitHub Release publication unless all five required target archives succeed.
- **FR-019**: Each target-specific packaging command MUST produce a portable archive containing the target executable and required resources without requiring a native installer.
- **FR-020**: The repository MUST migrate its developer, verification, build, package, release, and Spec Kit command aliases from the root Makefile into a root Taskfile compatible with the pinned Wails task-based workflow.
- **FR-021**: Go Task MUST be version-pinned in its own isolated Go tool module under `tools/` and MUST be installed through the same repository-owned bootstrap as every other Go development tool.
- **FR-022**: The root Makefile MUST retain only one bootstrap responsibility: installing every Go tool declared by the isolated modules under `tools/`, including Go Task; it MAY expose a non-mutating `help` target that points to Task discovery, but MUST NOT remain a parallel owner or proxy of application workflows.
- **FR-023**: Existing maintainer workflows currently exposed through Make MUST have documented Task equivalents with preserved ordering, inputs, and failure behavior.
- **FR-024**: The accepted Wails Go runtime, isolated CLI tool, and frontend runtime MUST remain pinned exactly to the accepted `v3.0.0-beta.13` / `3.0.0-beta.13` baseline, with committed Go and npm checksums.
- **FR-025**: Every supported target MUST be built from the tagged revision on a compatible runner by invoking the same explicit target-aware repository package entrypoint with that target's operating-system and processor-architecture inputs.
- **FR-026**: The release workflow MUST NOT depend on Docker aggregate builds, native UI automation, credential-store integration tests, public-tunnel tests, GitHub Packages publication, signing, notarization, or rollback orchestration.
- **FR-027**: Pushing a semantic-version tag in the form `vMAJOR.MINOR.PATCH`, with an optional prerelease suffix, MUST trigger the complete supported-target build matrix and a single GitHub Release publication flow; other tags, pull requests, and ordinary branch pushes MUST NOT trigger that matrix.
- **FR-028**: The GitHub Release MUST contain exactly one uniquely named portable archive per supported target, and each archive MUST contain that target's executable and required runtime resources.
- **FR-029**: The macOS tagged-release path MUST NOT create or require a DMG, signature, notarization, stapling, or any macOS-specific publication path.
- **FR-030**: If a qualifying tag already has a GitHub Release, the workflow MUST fail and MUST leave the existing release and all of its assets unchanged.
- **FR-031**: If publication creates a partial GitHub Release and then fails, the workflow MUST leave it unchanged, report instructions for a maintainer to delete it manually, refuse automated reruns while it exists, and allow the same tag to be rerun after its deletion.
- **FR-032**: A separate non-release quality workflow MUST run project quality checks for pull requests and pushes to the main branch, MUST NOT run the five-target release matrix, and MUST publish no release assets.
- **FR-033**: The repository MUST retain `task package:all` and its Docker implementation as optional maintainer convenience that runs neither in CI nor as a platform-support, quality-workflow, feature-completion, or tagged-release gate.
- **FR-034**: The repository MUST remove `package:all:remote`, `release:local`, and their remote aggregation and joined-release implementations, tests, tool dependencies, and active documentation.
- **FR-035**: Tagged publication MUST enter through the CI-owned `release:publish` Task command, which MUST invoke repository-pinned GoReleaser as the sole GitHub Release publisher.

## Key Entities

- **Platform Target**: One supported operating-system and processor-architecture combination, including its compatibility baseline and runtime prerequisites.
- **Distribution Artifact**: The governed portable archive for one platform target, including its target identity, product metadata, executable content, resources, and verification status.
- **Release Build Matrix**: The five independent compatible-runner builds for Windows amd64, Windows arm64, Linux amd64, Linux arm64, and macOS arm64 from one version tag.
- **Target Build Result**: The success or failure of compiling one target and creating its non-empty portable archive with executable and required resources.
- **Tagged Release Run**: One workflow run that builds every supported target from a version tag and attaches the complete archive set to that tag's GitHub Release.
- **Non-Release Quality Run**: A pull-request or main-branch workflow run that evaluates project quality without invoking the five-target release matrix or publishing release assets.
- **Local Package-All Run**: An optional maintainer-initiated Docker aggregate from the current checkout that is isolated from CI, platform support, feature completion, and tagged releases.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A successful version-tag run produces exactly five uniquely identified portable archives: Windows amd64, Windows arm64, Linux amd64, Linux arm64, and macOS arm64.
- **SC-002**: Every published archive is non-empty and contains its target executable plus all required runtime resources.
- **SC-003**: Archive inspection for every target requires zero native window, dialog, player, lifecycle, credential-store, tunnel, or signing execution steps.
- **SC-004**: Every archive name identifies its operating system and processor architecture with zero collisions across the release.
- **SC-005**: Public-access credentials, plaintext secret fallbacks, and secret-bearing verification records are absent from all five archives and every automated release asset.
- **SC-006**: Every optional native or external-service check that is unavailable or unexecuted is reported as `NOT RUN` and changes zero archive-eligibility, platform-support, quality-workflow, or tagged-release outcomes.
- **SC-007**: A successful macOS target job executes zero signing, notarization, or stapling steps.
- **SC-008**: Using only the published guidance, a maintainer can identify the correct artifact, its prerequisites, and its user-data locations for any supported target in under five minutes.
- **SC-009**: After one tool-bootstrap command, a maintainer can run every migrated repository workflow through the pinned Task binary, and automated inspection finds no application workflow remaining in the Makefile.
- **SC-010**: A controlled target-build failure prevents the tag's release from being published as complete and identifies the failed target.
- **SC-011**: A successful version-tag workflow attaches the full supported-target archive set to one GitHub Release without publishing a duplicate raw executable, checksum sidecar, aggregate index, or package-registry copy.
- **SC-012**: The release workflow contains zero strict native UI, dialog, secure-store, public-tunnel, signing, notarization, or rollback-orchestration gates.
- **SC-013**: The macOS release asset is one non-empty portable archive whose stable name identifies macOS arm64 and whose contents include the packaged application executable and required resources under the same archive eligibility contract as Windows and Linux.
- **SC-014**: Re-running publication for a tag that already has a GitHub Release makes zero changes to that release or its assets and reports failure.
- **SC-015**: A controlled publication failure after partial release creation performs zero automated deletion or replacement actions, reports the manual deletion-and-rerun procedure, and permits the same tag to publish only after a maintainer deletes the partial release.
- **SC-016**: Every pull-request and main-branch quality run publishes zero release assets and invokes zero jobs from the five-target release matrix.

## Assumptions

- “Support” means availability of a governed unsigned portable archive containing the desktop executable and required resources, not verified native runtime behavior, an installer, or a signed platform-native distribution.
- The release scope covers the four requested Windows/Linux targets plus the existing macOS arm64 target; macOS amd64 and other targets are not implied.
- Each platform's minimum supported version will follow the compatibility range of the project's accepted desktop runtime and native dependencies and will be stated explicitly in release documentation.
- Governed portable release archives are required for all five targets; native installers and platform signing are not required by this feature.
- Existing session, player, native UI, dialog, lifecycle, credential-store, and public-access behavior is unchanged and outside the acceptance scope of this archive-availability feature.
- Detailed native and operating-system integration behavior may be checked manually when useful, but those checks do not gate feature completion, quality CI, platform support, or hobby-project tagged releases.
- The five-target build matrix is release automation only and runs for semantic-version tags in the form `vMAJOR.MINOR.PATCH`, including optional prerelease suffixes; it does not run for other tags, pull requests, or ordinary branch pushes.
- Pull requests and pushes to the main branch continue to run project quality checks in a separate non-release workflow that publishes no release assets.
- Optional local `task package:all` may retain Docker-specific checksums and aggregate metadata because those outputs never enter CI or a tagged release.
- Live acceptance uses a maintainer-approved unused SemVer prerelease tag only after the implementation is committed and local/static validation passes; a successful acceptance prerelease is preserved as evidence.
- The Taskfile is the command-orchestration surface; existing Go packages and commands may continue to own detailed build, archive, and verification logic so that orchestration is not duplicated in YAML.
- Maintainers own deletion of a partial GitHub Release before rerunning its tag; release automation never rolls back or modifies an existing release.

## Verbatim Constraints

- Operating systems: `windows`, `linux`
- Processor architectures: `arm64`, `amd64`
- Existing macOS target: `darwin/arm64`
- CI publisher task: `release:publish`
