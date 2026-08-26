# Feature Specification: Windows and Linux Desktop Support

## Clarifications

### Session 2026-08-26

- Q: How should maintainers package all supported platforms? → A: Separate per-target commands run on matching operating-system hosts, plus an aggregate command that packages the complete matrix.
- Q: What distribution form must each target-specific package command produce? → A: A portable runnable archive for every target.
- Q: Which repository task runner owns developer and packaging commands? → A: Migrate project automation from Make to Go Task, pin Task like the other isolated Go tools, and leave Make only as the bootstrap that installs every Go tool declared under `tools/`.
- Q: May Make retain a help target after the Task migration? → A: Yes. `make help` is a non-mutating discovery target that documents `make tools` and directs maintainers to `task --list`; it must not proxy project workflows.
- Q: What must local `task package:all` expose? → A: On `darwin/arm64`, the canonical native signed application bundle plus the four governed archives and four directly accessible Windows/Linux target payloads under `bin/<os>-<arch>/`, with the five targets published atomically.
- Q: How must a missing or stopped Docker daemon fail? → A: Nonzero with an actionable diagnostic that preserves the Docker failure reason and tells the maintainer to install or start Docker; a generic Task precondition error is insufficient.
- Q: What must a SemVer tag publish for all supported targets? → A: One joined five-target release from the tag SHA: the established signed/notarized `darwin/arm64` DMG plus all four verified Windows/Linux portable archives, published only as a complete set to GitHub Releases and GitHub Packages through pinned GoReleaser v2 orchestration.

**Bugfix**: 2026-08-26 — BUG-001 aligned local Docker aggregate output, diagnostics, and verification across the feature artifacts.

**Bugfix**: 2026-08-26 — BUG-002 makes local aggregate publication safely repeatable when an owned prior output exists.

**Bugfix**: 2026-08-26 — BUG-003 adds the supported native `darwin/arm64` package to local `package:all`.

**Bugfix**: 2026-08-26 — BUG-004 adds the missing five-target tagged-release contract and reopens false completion evidence.

## User Scenarios & Testing

### User Story 1 - Run the desktop application on every supported target (Priority: P1)

As an Overseer, I can obtain the Fallout Terminal desktop application for my Windows or Linux computer and launch it as a complete native application, so I can host a game without owning a Mac or installing development tools.

**Why this priority**: A usable application on each requested operating-system and processor combination is the core value of the feature.

**Independent Test**: On a clean machine for each target, unpack or install the matching artifact, launch it, confirm the Overseer window opens, and load the bundled demo without a development toolchain or repository checkout.

**Acceptance Scenarios**:

1. **Given** a supported Windows computer and the matching processor artifact, **When** the user launches Fallout Terminal, **Then** the Overseer window opens with the bundled application assets and demo available.
2. **Given** a supported Linux computer and the matching processor artifact, **When** the user launches Fallout Terminal, **Then** the Overseer window opens with the bundled application assets and demo available.
3. **Given** a user selects an artifact for the wrong operating system or processor architecture, **When** they consult the artifact name and release documentation, **Then** they can identify the mismatch before attempting normal use.

### User Story 2 - Host the same game workflow across platforms (Priority: P1)

As an Overseer running Windows or Linux, I can create and save sessions, control terminals, serve the player experience, open external links, and manage public-access credentials with the same security and lifecycle guarantees as the existing desktop application.

**Why this priority**: Launching alone is insufficient; the new builds must preserve the workflows that make the application useful during a game.

**Independent Test**: On one supported machine per operating system, complete a representative game-hosting journey from opening the bundled demo through connecting a player browser, saving a session, using native dialogs, exercising configured public access, and closing the application cleanly.

**Acceptance Scenarios**:

1. **Given** the application is running on a supported Windows or Linux target, **When** the Overseer opens or saves a session, **Then** the native dialog starts from an appropriate user location and the selected JSON file is handled correctly.
2. **Given** a player is on the same reachable network as the host, **When** they open the address shown by the application, **Then** they receive the synchronized player terminal and can participate in the same session workflow as with the existing application.
3. **Given** the operating system provides an available secure credential service, **When** the Overseer stores public-access credentials, **Then** the credentials persist in that protected service and never appear in session files, settings files, logs, or public state.
4. **Given** the Overseer closes the application, **When** shutdown completes, **Then** owned listeners and public-access resources are released without leaving a background application process.

### User Story 3 - Produce trustworthy artifacts for all targets (Priority: P2)

As a maintainer, I can produce and inspect a complete release matrix of portable runnable archives for Windows and Linux and build the native Darwin package plus the same four target payloads directly from my current checkout, so every advertised artifact and local executable has clear target identity and evidence that it contains the required application resources.

**Why this priority**: Repeatable release evidence prevents users from receiving mislabeled, incomplete, or architecture-incompatible downloads.

**Independent Test**: Start from a clean checkout in the supported automation environment, produce the full target matrix, and verify that each archive is uniquely named, can be unpacked without an installer, has the declared operating system and architecture, contains the required resources, and passes its platform launch check. Separately, on `darwin/arm64`, inspect a local aggregate from the current checkout and verify the complete signed application bundle plus exactly four Windows/Linux `bin/<os>-<arch>/` payloads, byte identity with their archives, atomic failure behavior, and actionable host/Docker prerequisite errors without treating Docker output as native Windows/Linux launch evidence.

**Acceptance Scenarios**:

1. **Given** a clean source revision and suitable target builders, **When** the aggregate packaging command runs, **Then** it coordinates the target-specific commands and produces one non-colliding artifact for each of the four requested target combinations.
2. **Given** a produced artifact, **When** its target and resource inventory are inspected, **Then** its operating system, processor architecture, executable metadata, application assets, bundled demo, and required notices match the declared target.
3. **Given** any target build or target-specific validation fails, **When** the workflow completes, **Then** that target is not presented as a successful release artifact and the failing target is clearly reported.
4. **Given** a current checkout that may contain uncommitted changes and a usable Docker daemon, **When** `task package:all` succeeds, **Then** its output contains all four archives and exactly one directly runnable executable/resource payload for each target under `bin/<os>-<arch>/`, and every payload file matches the corresponding verified archive.
5. **Given** Docker is missing, stopped, or cannot execute a required build platform, **When** `task package:all` starts, **Then** it exits nonzero without publishing a partial output and reports the underlying Docker cause plus an actionable recovery instruction.
6. **Given** the repository-owned default output or a recognized previous aggregate output already exists, **When** `task package:all` completes successfully, **Then** it replaces that output only after the new five-target local result is fully verified; if build, verification, or publication fails, the previous output remains available unchanged.
7. **Given** the command runs on the supported `darwin/arm64` host with Docker available, **When** `task package:all` succeeds, **Then** it publishes the native ad-hoc signed `Fallout Terminal.app` produced through the canonical `task package` plan under `bin/darwin-arm64/` together with all four Windows/Linux archives and runnable payloads.
8. **Given** a valid SemVer tag whose source revision passes all native target gates and macOS release credentials are available, **When** tag delivery succeeds, **Then** the same GitHub Release and versioned GitHub Packages artifact contain the signed/notarized Darwin DMG, its checksum, all four Windows/Linux archives and sidecars, and the aggregate index; if any target, trust gate, inventory check, or publication step fails, no partial five-target release is presented as successful.

### User Story 4 - Choose and operate the correct distribution (Priority: P3)

As a user or maintainer, I can find the supported versions, runtime prerequisites, artifact naming rules, launch instructions, storage locations, and known platform limitations, so I can select and operate the correct distribution without guesswork.

**Why this priority**: Documentation reduces installation and support friction after the application and release matrix are functional.

**Independent Test**: Give the release documentation and artifact list to a user unfamiliar with the build process and confirm they can choose the correct download, identify prerequisites, launch it, and locate user-owned data.

**Acceptance Scenarios**:

1. **Given** the available Windows and Linux artifacts, **When** a user consults the documentation, **Then** they can select the artifact matching their operating system and processor architecture.
2. **Given** a target has an operating-system runtime prerequisite, **When** a user prepares the machine, **Then** the documentation states the prerequisite and how to recognize when it is unavailable.
3. **Given** a user needs to back up or troubleshoot Fallout Terminal, **When** they consult the platform guidance, **Then** they can locate session documents and private application settings without exposing stored secrets.

## Edge Cases

- What happens when an artifact is launched on the wrong operating system or processor architecture?
- How does startup fail when a required native web-view, desktop library, or secure credential service is missing or locked?
- How are user-data and application-resource locations resolved when the home directory is redirected, read-only, contains spaces or non-ASCII characters, or follows a non-default layout?
- How does the application distinguish packaged resources from a development checkout on each operating system?
- What happens when a saved session or settings file from macOS is opened on Windows or Linux, and vice versa?
- How are native dialog filters and external-link behavior kept consistent despite operating-system differences?
- What happens when the player-listener port is already occupied or public access cannot start?
- How does shutdown behave when players are connected or public access is active?
- How are artifacts prevented from overwriting one another when all four targets are produced from the same source revision?
- How is a target withheld when its application resources, metadata, executable permissions, or architecture cannot be verified?
- How does local aggregate packaging preserve the Docker diagnostic and avoid exposing a partial output when the daemon or a required build platform is unavailable?

## Requirements

### Functional Requirements

- **FR-001**: The product MUST support distributable desktop application targets for Windows on arm64, Windows on amd64, Linux on arm64, and Linux on amd64.
- **FR-002**: The product MUST document the minimum supported operating-system versions and required system runtime dependencies for each target.
- **FR-003**: Every target artifact MUST include the complete Overseer interface, player interface, bundled demo data, product identity, application icon, and required third-party notices.
- **FR-004**: A user MUST be able to launch each target artifact without installing the source repository, a compiler, or frontend build tools.
- **FR-005**: Every target MUST preserve the existing session authoring, terminal control, navigation, hacking, sound, local player connection, and configured public-access workflows.
- **FR-006**: Every target MUST provide working native open-file, save-file, and external-link interactions with JSON file selection behavior consistent across operating systems.
- **FR-007**: Every target MUST keep user-owned session documents, private application settings, and read-only bundled resources in distinct locations appropriate to that operating system.
- **FR-008**: Every target MUST store public-access secrets only in an operating-system-protected credential service and MUST fail closed with a clear status when no secure service is available.
- **FR-009**: Every target MUST release owned listeners, public-access resources, and background activity when the application exits normally or startup fails.
- **FR-010**: Maintainers MUST have a documented packaging command for each supported target that runs on a matching operating-system host without requiring source changes between targets.
- **FR-011**: Each produced artifact MUST have a stable, unique name that identifies its operating system and processor architecture.
- **FR-012**: The automated delivery workflow MUST build and report all four target combinations independently from a clean source revision.
- **FR-013**: The delivery workflow MUST verify each artifact's declared target, required resource inventory, product metadata, and ability to reach an opened application window on a matching platform.
- **FR-014**: A failed or unverifiable target MUST NOT be reported or published as a successful artifact.
- **FR-015**: Existing macOS application behavior and its supported distribution MUST remain available without regression while Windows and Linux support is added.
- **FR-016**: User-facing documentation MUST explain target selection, prerequisites, launch steps, data locations, credential-service expectations, and platform-specific troubleshooting.
- **FR-017**: Platform or architecture mismatches and missing runtime dependencies MUST produce actionable failure information rather than silent exit or partial startup.
- **FR-018**: Maintainers MUST have an additional aggregate packaging command that coordinates all four target-specific commands and reports failure unless every target package succeeds.
- **FR-019**: Each target-specific packaging command MUST produce a portable runnable archive that requires no native installer.
- **FR-020**: The repository MUST migrate its developer, verification, build, package, release, and Spec Kit command aliases from the root Makefile into a root Taskfile compatible with the pinned Wails task-based workflow.
- **FR-021**: Go Task MUST be version-pinned in its own isolated Go tool module under `tools/` and MUST be installed through the same repository-owned bootstrap as every other Go development tool.
- **FR-022**: The root Makefile MUST retain only one bootstrap responsibility: installing every Go tool declared by the isolated modules under `tools/`, including Go Task; it MAY expose a non-mutating `help` target that points to Task discovery, but MUST NOT remain a parallel owner or proxy of application workflows.
- **FR-023**: Existing maintainer workflows currently exposed through Make MUST have documented Task equivalents with preserved ordering, inputs, and failure behavior.
- **FR-024**: The accepted Wails Go runtime, isolated CLI tool, and frontend runtime MUST be upgraded together and pinned exactly to the latest published beta, `v3.0.0-beta.13` / `3.0.0-beta.13`, with committed Go and npm checksums.
- **FR-025**: Maintainers MUST be able to build and statically verify the complete four-target portable matrix from the current local checkout with Docker, including a directly accessible executable and required resource tree for every target, without requiring a clean or pushed branch; partial output MUST remain unpublished and native launch verification MUST remain explicitly separate.
- **FR-026**: Local aggregate packaging prerequisite failures MUST identify Docker as the failing dependency, preserve the underlying diagnostic when available, provide an actionable install/start/platform recovery instruction, and MUST NOT be replaced by a generic task-runner precondition failure.
- **FR-027**: Local aggregate packaging MUST safely replace an existing repository-owned default output or recognized previous aggregate output only after complete verification, MUST restore or preserve the previous output on publication failure, and MUST reject replacement of a file, symlink, repository root, filesystem root, or unrecognized custom directory.
- **FR-028**: Local `task package:all` MUST require the supported `darwin/arm64` host, build the macOS application through the same canonical plan as no-target `task package`, verify and expose its complete application bundle at `OUTPUT/bin/darwin-arm64/Fallout Terminal.app`, and publish it in the same all-or-nothing transaction as the four Docker-built Windows/Linux targets.
- **FR-029**: A valid `vMAJOR.MINOR.PATCH` tag with an optional prerelease suffix MUST coordinate the established signed, hardened-runtime, notarized, stapled, and Gatekeeper-verified `darwin/arm64` DMG with the complete four-target Windows/Linux native matrix from one tag SHA, and the repository-pinned GoReleaser v2 flow MUST publish the complete joined inventory to one GitHub Release and one versioned GHCR artifact in GitHub Packages; any missing, failed, mismatched, or unverifiable target or publication step MUST withhold partial release success.

## Key Entities

- **Platform Target**: One supported operating-system and processor-architecture combination, including its compatibility baseline and runtime prerequisites.
- **Distribution Artifact**: The portable runnable archive for one platform target, including its target identity, product metadata, executable content, resources, and verification status.
- **Runnable Payload**: The directly accessible executable and required resource tree exported for one local Docker target; its exact inventory and file hashes must match the corresponding verified Distribution Artifact.
- **Local Aggregate Run**: One atomic five-target build from the current checkout that owns the canonical native `darwin/arm64` package, four quarantined Docker target outputs, static verification, actionable host/Docker prerequisite diagnostics, and final publication without claiming native Windows/Linux launch evidence.
- **Platform Storage Profile**: The target-specific locations for user session documents, private non-secret settings, protected credentials, and bundled read-only resources.
- **Target Verification Record**: Evidence associated with one artifact showing its declared target, required resource inventory, launch result, and release eligibility.
- **Tagged Release Run**: One five-target release transaction for a SemVer tag that joins the trusted Darwin DMG with four eligible portable artifacts from the same SHA before GoReleaser-owned GitHub Release and GHCR publication.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A clean aggregate packaging run with suitable target builders produces exactly four independently identified portable runnable archives, one for each requested operating-system and processor combination.
- **SC-002**: On a clean matching system, 100% of the four artifacts open the Overseer window and load the bundled demo within 60 seconds without developer tooling.
- **SC-003**: A representative host-and-player journey passes on both Windows and Linux, including session open/save, one connected player, synchronized control, and clean shutdown.
- **SC-004**: Artifact inspection confirms the declared operating system, processor architecture, product identity, and required resource inventory for all four artifacts with zero mismatches.
- **SC-005**: Public-access credentials are absent from session files, non-secret settings, logs, and public state in every supported target verification.
- **SC-006**: When a secure credential service is unavailable, every affected verification reports public access as unavailable and stores zero secrets in an unprotected fallback.
- **SC-007**: The existing macOS distribution continues to meet its current build and launch acceptance checks after the new targets are introduced.
- **SC-008**: Using only the published guidance, a maintainer can identify the correct artifact, its prerequisites, and its user-data locations for any supported target in under five minutes.
- **SC-009**: After one tool-bootstrap command, a maintainer can run every migrated repository workflow through the pinned Task binary, and automated inspection finds no application workflow remaining in the Makefile.
- **SC-010**: A successful local `task package:all` output contains the complete Darwin bundle plus exactly four Windows/Linux target payload directories and directly accessible executables with complete required resources, zero inventory or hash differences from their corresponding archives, while an unavailable host or Docker prerequisite yields an actionable nonzero error and no partial output directory.
- **SC-011**: Two consecutive successful `task package:all` runs using the same owned output require no manual deletion; the second exposes only its complete verified matrix, while injected build or final-publication failure leaves the first result available and byte-unchanged and unsafe existing output targets are rejected.
- **SC-012**: On a supported `darwin/arm64` host, a successful local `task package:all` reports and exposes exactly one complete macOS application bundle plus the four Windows/Linux runnable payloads; a non-Darwin or non-arm64 host fails before packaging with no output replacement.
- **SC-013**: For every successful SemVer tag publication, GitHub Releases and the versioned GHCR artifact expose exactly one verified Darwin DMG with checksum, four verified Windows/Linux archives with sidecars, and one aggregate index from the same source SHA; controlled failure of any of the five target gates produces zero successful partial release publication.

## Assumptions

- “Support” means a complete desktop distribution with functional parity for existing game-hosting workflows, not merely a cross-compiled command-line executable.
- The scope covers the four requested Windows/Linux targets; additional operating systems and processor architectures are not implied.
- Each platform's minimum supported version will follow the compatibility range of the project's accepted desktop runtime and native dependencies and will be stated explicitly in release documentation.
- Portable runnable release archives are required for the four new targets; native installers and platform signing are not required by this feature.
- Session and player-configuration file formats remain portable across supported operating systems.
- Acceptance evidence may be produced by automation or matching platform hosts; this specification step does not require local test execution.
- The Taskfile is the command-orchestration surface; existing Go packages and commands may continue to own detailed build, archive, and verification logic so that orchestration is not duplicated in YAML.

## Verbatim Constraints

- Operating systems: `windows`, `linux`
- Processor architectures: `arm64`, `amd64`
