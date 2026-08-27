# Feature Specification: Application Self-Update

**Created**: 2026-08-27
**Status**: Draft

## User Scenarios & Testing

### User Story 1 - Discover an Update at Startup (Priority: P1)

As the Overseer, I launch the packaged application normally. The application checks once for a newer eligible release without delaying access to the control interface. When a newer release exists for my system, I see a clear prompt that identifies the installed and available versions and lets me decide whether to update now or continue using the current version.

**Why this priority**: Timely discovery with explicit Overseer consent is the essential value of self-update.

**Independent Test**: Launch a packaged older version while a newer eligible release is published, confirm the control interface remains usable, and verify that one update prompt appears with accurate release information and actionable choices.

**Acceptance Scenarios**:

1. **Given** a packaged application with an older version than the latest eligible release for its platform and architecture, **When** the application starts, **Then** it checks once and asks the Overseer whether to update.
2. **Given** a newer eligible release was found, **When** the prompt is shown, **Then** it identifies the installed version, available version, and available release notes before any update download begins.
3. **Given** the update check is still running, **When** the application finishes normal startup, **Then** the Overseer can use the application without waiting for the check to finish.
4. **Given** the installed version is current or newer than every eligible published release, **When** the startup check completes, **Then** no update prompt interrupts the Overseer.
5. **Given** the Overseer chooses to continue without updating, **When** the prompt closes, **Then** the current application remains fully usable and the same release is not prompted again during that application run.

---

### User Story 2 - Apply a Trusted Update (Priority: P1)

As the Overseer, I accept an offered update and can see that it is being prepared. The application uses the release intended for my system, rejects an untrusted or damaged download, and asks before restarting. After I approve restart, the current application shuts down cleanly, replaces itself, and relaunches as the newer version without changing my sessions or settings.

**Why this priority**: Discovering an update is useful only if the application can apply it safely and preserve the Overseer's work.

**Independent Test**: Accept a valid update on a supported target, allow preparation to complete, approve restart, and confirm the application relaunches at the offered version with its pre-update user data unchanged.

**Acceptance Scenarios**:

1. **Given** a newer eligible release is offered, **When** the Overseer accepts it, **Then** the application downloads only the artifact matching the running platform and architecture and reports preparation progress.
2. **Given** an update artifact has downloaded, **When** its integrity or authenticity evidence is missing, invalid, or mismatched, **Then** the application refuses to stage or apply it and leaves the installed application unchanged.
3. **Given** a valid update is fully staged, **When** the application is ready to apply it, **Then** the Overseer is asked to restart and can postpone that restart without losing the staged result or current work.
4. **Given** the Overseer approves restart, **When** the update is applied, **Then** owned runtime resources shut down cleanly before replacement and the updated application relaunches.
5. **Given** the updated application relaunches, **When** its startup check runs, **Then** it reports the newly installed version as current and does not offer the same release again.
6. **Given** sessions, player configurations, credentials, and application preferences existed before the update, **When** the update completes, **Then** those user-owned data remain unchanged and available.

---

### User Story 3 - Keep Working When Updating Is Unavailable (Priority: P2)

As the Overseer, I can still run Fallout Terminal when the update service is offline, a release is incomplete, a download is interrupted, or replacement cannot complete. The failure is explained in actionable language, and I can retry on a later launch without repairing or reinstalling the current application first.

**Why this priority**: Update infrastructure is external to the game session and must never become a new startup or availability dependency.

**Independent Test**: Exercise unavailable discovery, interrupted download, invalid artifact, and failed replacement conditions and verify that each condition preserves a usable current installation or restores it automatically.

**Acceptance Scenarios**:

1. **Given** the device is offline or the release service rejects or times out the request, **When** the startup check fails, **Then** normal application startup and local operation continue.
2. **Given** a published release has no compatible artifact, **When** discovery evaluates it, **Then** the application does not offer or download that release and records an actionable diagnostic.
3. **Given** a download is canceled or interrupted, **When** preparation ends, **Then** no partial artifact is treated as ready and the installed application remains usable.
4. **Given** replacement or relaunch fails after the Overseer approved restart, **When** recovery completes, **Then** the last working application is retained or restored and the failure is diagnosable.
5. **Given** an update attempt failed or was postponed, **When** the application starts later and the release remains eligible, **Then** the Overseer can be offered the update again.

## Edge Cases

- How is an unversioned development build handled so it cannot mistake every published release for an update?
- What happens when the installed version is newer than the latest public release, such as a local or rollback build?
- How are stable and prerelease versions compared so a stable installation is not moved to an unintended prerelease channel?
- What happens when a release exists but its platform or architecture artifact, release notes, size, or verification evidence is missing?
- What happens when more than one release asset appears to match the running platform and architecture?
- How does the flow behave when a second check is requested while checking, downloading, verifying, staging, or restarting is already in progress?
- What happens if the Overseer closes the prompt or quits the application during download or staging?
- How are network timeout, rate limiting, partial download, insufficient disk space, and read-only installation-location failures presented?
- What happens when the packaged replacement unit is structurally incompatible with the installed application layout?
- How is the existing application recovered if shutdown completes but replacement or relaunch fails?
- How are operating-system signing, quarantine, antivirus, and executable-permission failures surfaced without reporting a successful update?
- How does updating preserve user-owned files stored beside or outside packaged application resources?

## Requirements

### Functional Requirements

- **FR-001**: Every packaged production launch MUST initiate exactly one update check after the application can present Overseer-facing status.
- **FR-002**: The startup update check MUST run without blocking normal application readiness or local operation.
- **FR-003**: The application MUST compare the installed application version with published release versions using semantic-version precedence.
- **FR-004**: The application MUST offer only a strictly newer release that is eligible for the installed release channel, running platform, and running architecture.
- **FR-005**: When an eligible update exists, the application MUST present the Overseer with the installed version, available version, release notes when supplied, and a choice to update now or continue without updating.
- **FR-006**: The application MUST NOT download, stage, replace, or restart for an update until the Overseer explicitly accepts the offered update.
- **FR-007**: Deferring or dismissing an update MUST leave the current application fully usable and MUST suppress another prompt for the same release during that application run.
- **FR-008**: The update flow MUST select exactly one artifact that matches the running platform and architecture and MUST reject missing or ambiguous matches.
- **FR-009**: The application MUST verify every downloaded update against publisher-provided integrity or authenticity evidence before staging it.
- **FR-010**: Missing, malformed, or mismatched verification evidence MUST prevent the update from being staged or applied.
- **FR-011**: The update flow MUST accept only a replacement unit compatible with the installed application's platform-specific package layout.
- **FR-012**: The application MUST report checking, downloading, verifying, staging, ready-to-restart, and failed states to the Overseer without exposing sensitive values.
- **FR-013**: A prepared update MUST require a separate Overseer decision before the application restarts to apply it.
- **FR-014**: Applying an update MUST use the application's normal ordered shutdown so owned resources are released before replacement begins.
- **FR-015**: A successfully applied update MUST relaunch the application at the accepted newer version.
- **FR-016**: If replacement or relaunch fails, the update process MUST retain or restore the last working installation and MUST NOT report success.
- **FR-017**: Update discovery, download, verification, staging, replacement, and relaunch failures MUST NOT corrupt or modify sessions, player configurations, credentials, preferences, or other user-owned data.
- **FR-018**: Failure to check for or prepare an update MUST NOT cause application startup to fail or disable local operation.
- **FR-019**: The application MUST prevent concurrent update checks or installations from creating competing state or replacement attempts.
- **FR-020**: Unversioned development builds MUST NOT perform a production update check or present a production update prompt.
- **FR-021**: Every governed release for a supported target MUST embed its exact release version in the packaged application.
- **FR-022**: Every governed release MUST publish the compatible target artifacts and their verification evidence as one complete release before any installed application can discover it.
- **FR-023**: Release publication MUST withhold an update from discovery when any required supported-target artifact or verification record is absent or invalid.
- **FR-024**: The update feature MUST support the existing governed targets: Windows amd64, Windows arm64, Linux amd64, Linux arm64, and macOS arm64.
- **FR-025**: Update diagnostics MUST identify the failed stage and provide an actionable next step while excluding credentials, authorization values, and user document contents.

## Key Entities

- **Installed Application Version**: The semantic version embedded in the running packaged application, together with its release channel, platform, and architecture identity.
- **Published Release**: A discoverable application version with release notes and a complete set of supported-target artifacts and verification evidence.
- **Update Candidate**: A strictly newer published release that is eligible for the installed version's channel and has exactly one compatible artifact for the running system.
- **Update Artifact**: The platform-specific replacement unit, including its size, package-layout identity, and publisher-provided verification evidence.
- **Update Attempt**: One launch-scoped progression through discovery, Overseer decision, download, verification, staging, restart decision, replacement, relaunch, or failure.

## Success Criteria

### Measurable Outcomes

- **SC-001**: In all supported-target acceptance runs with a newer eligible release, exactly one update prompt appears during startup and the control interface remains usable while discovery completes.
- **SC-002**: In all acceptance runs where the installed version is current, newer, unversioned, or has no eligible compatible artifact, zero update prompts are shown.
- **SC-003**: Offline, timeout, rate-limit, malformed-release, and unavailable-service tests produce zero startup failures and leave local application operation available.
- **SC-004**: For each of the five supported targets, accepting a valid update selects exactly the matching platform and architecture artifact and rejects every non-matching artifact.
- **SC-005**: All corrupted, incomplete, unverified, ambiguous, or structurally incompatible update artifacts are rejected before replacement, with the installed application remaining usable.
- **SC-006**: In successful end-to-end acceptance, the application relaunches at the offered version and a subsequent check offers that same version zero times.
- **SC-007**: In controlled replacement and relaunch failures, the last working installation is retained or restored in every run and no run is reported as successful.
- **SC-008**: Representative sessions, player configurations, credentials, and preferences have identical business content before and after both successful and failed update attempts.
- **SC-009**: Every published governed release exposes a complete update path for all five supported targets, with zero discoverable releases missing required verification evidence.
- **SC-010**: Every simulated failure stage produces a diagnostic that identifies the stage and a recovery action while automated secret scanning finds zero leaked sensitive values.

## Assumptions

- Existing governed GitHub Releases remain the public source of application updates; no new account, license service, or private update backend is introduced.
- The update check runs once per packaged application launch after the Overseer interface is available and uses a bounded background operation.
- Stable installations receive stable releases only. A prerelease installation may receive a newer prerelease or the next stable release, but a stable installation is never moved to a prerelease automatically.
- “Continue without updating” means “not again during this run”; it does not permanently skip that version, so it can be offered on a later launch.
- Release artifacts and their verification evidence are published atomically, and the application fails closed when that evidence cannot validate the downloaded bytes.
- The existing portable release matrix remains supported. Planning may adjust its artifact layout or publication inventory where required to make each target safely replaceable.
- Update application resources are distinct from user-owned data; sessions, player configurations, credentials, and preferences remain outside the replacement boundary.
- The update flow does not add an installer, an unattended forced-update policy, downgrade support, or a background update service that runs while the application is closed.
