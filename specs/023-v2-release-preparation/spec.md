# Feature Specification: V2 Release Preparation

**Feature Branch**: `023-v2-release-preparation`

**Created**: 2026-08-27

**Status**: Draft

**Input**: User description: "Prepare the Go module and governed release process for a v2.0.0 release, update the affected specification and active documentation, and reject release tags whose major version does not match v2."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Publish a Valid V2 Release (Priority: P1)

As a maintainer, I can create a stable or prerelease v2 tag and know that the repository identifies
the tagged source as the same major release before any platform packaging or publication begins.
Tags for an older or future major are refused early instead of producing a release whose source
identity and release identity disagree.

**Why this priority**: A mismatched major tag would create a release that cannot be consumed or
reasoned about consistently by Go tooling and downstream maintainers.

**Independent Test**: Validate representative stable, prerelease, malformed, older-major, and
future-major tags and confirm that only strict v2 tags can enter the release matrix.

**Acceptance Scenarios**:

1. **Given** the repository is prepared for major version 2, **When** a maintainer validates `v2.0.0`, **Then** the tag is accepted as a stable release candidate.
2. **Given** the repository is prepared for major version 2, **When** a maintainer validates `v2.0.0-rc.1`, **Then** the tag is accepted and identified as a prerelease candidate.
3. **Given** an otherwise valid tag whose major version is not 2, **When** release preflight runs, **Then** it fails before any target package job starts.
4. **Given** a malformed v2 tag, a numeric prerelease identifier with a leading zero, or build metadata, **When** release preflight runs, **Then** it is rejected with an actionable format error.

---

### User Story 2 - Preserve Existing Application Compatibility (Priority: P1)

As an Overseer, I can move from the existing application to the v2 release without conversion or
loss of sessions, player configurations, credentials, preferences, or gameplay behavior. The major
release changes source and release identity, not the established user-data formats or runtime
contracts.

**Why this priority**: Release numbering must not silently become a persistence-format migration or
change the behavior of the application used during a game.

**Independent Test**: Open, save, and reopen representative version-1 session and player-
configuration files using the v2-prepared source and confirm their business content and runtime
behavior remain unchanged.

**Acceptance Scenarios**:

1. **Given** an existing version-1 session and player configuration, **When** each is opened and saved by the v2-prepared application, **Then** it remains readable with the same business content and established version.
2. **Given** existing public and private application contracts, **When** generated artifacts are refreshed for the v2 source identity, **Then** message fields, service methods, directions, and wire behavior remain unchanged.
3. **Given** the v2 migration is complete, **When** existing gameplay and desktop journeys run, **Then** they observe no new user interaction, authority, persistence, or security behavior.

---

### User Story 3 - Follow Unambiguous Release Guidance (Priority: P2)

As a maintainer, I can follow the active release documentation without encountering examples that
suggest an incompatible major tag or imply that independent development-tool modules share the
application module's major-version identity.

**Why this priority**: Correct code can still be released incorrectly when active instructions
show stale examples or blur independently versioned modules.

**Independent Test**: Search active maintainer documentation and release validation fixtures and
confirm they consistently describe v2 tags while preserving completed specifications as historical
records.

**Acceptance Scenarios**:

1. **Given** a maintainer reads the active release instructions, **When** they choose a release tag, **Then** stable and prerelease examples use major version 2.
2. **Given** independently owned development-tool modules, **When** the application moves to v2, **Then** those modules retain their existing identities and remain reproducibly usable.
3. **Given** completed feature specifications that record earlier generated paths or release behavior, **When** active documentation is updated, **Then** those historical specifications remain unchanged and are not presented as current instructions.

### Edge Cases

- What happens when a syntactically valid `v1`, `v3`, or higher-major tag is pushed?
- What happens when a v2 tag contains leading-zero numeric components, invalid prerelease identifiers, or build metadata?
- What happens when generated language metadata changes while protobuf package names, field numbers, and wire behavior do not?
- What happens when a generated desktop-binding location changes but the registered methods and events do not?
- What happens when older source imports are present only in completed historical records or repository URLs?
- What happens when an independent development-tool module intentionally retains its existing module identity?
- How is version-1 session and player-configuration compatibility distinguished from application major version 2?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The governed application source identity MUST correspond to major version 2 before a v2 release is tagged.
- **FR-002**: Release preflight MUST accept strict stable and prerelease semantic versions whose major version is exactly 2.
- **FR-003**: Release preflight MUST reject every otherwise valid semantic version whose major version is not 2 before target packaging begins.
- **FR-004**: Release preflight MUST continue rejecting malformed versions, leading-zero numeric components, leading-zero numeric prerelease identifiers, and build metadata.
- **FR-005**: Every active application package and generated application binding MUST resolve through the v2 source identity with no active fallback to the previous identity.
- **FR-006**: Regenerated contract artifacts MUST preserve existing protobuf package names, messages, field numbers, services, RPC directions, and wire behavior while recording the v2 language-package identity.
- **FR-007**: The reviewed compatibility baseline MUST advance explicitly for the language-package identity change and MUST continue rejecting the established breaking-change fixtures.
- **FR-008**: Session JSON and player-configuration JSON MUST remain at version 1 with no conversion, field-name change, or loss of compatible unknown data.
- **FR-009**: Independent development-tool modules MUST retain their existing module identities and isolated dependency ownership.
- **FR-010**: Active release documentation MUST use stable and prerelease v2 tag examples and MUST explain that accepted release-tag majors match the application module major.
- **FR-011**: Completed specifications and migration records MUST retain their original paths, targets, and evidence as historical records.
- **FR-012**: The migration MUST introduce no new Overseer or player interaction, public capability, credential handling, network behavior, or runtime state.

### Impacted Application Surfaces *(mandatory)*

- **Composition and Wails bridge (`main.go`, `app.go`)**: Affected only by the application source identity and generated desktop-binding namespace; registered methods, events, lifecycle, and payload semantics remain unchanged.
- **Domain and canonical state (`internal/domain/`, `internal/nav/`, `internal/hack/`, `internal/live/`, `internal/control/`)**: Source identity references are affected; models, validation, state transitions, authority, and public projections are not.
- **Persistence (`internal/session/`, `internal/playerconfig/`, `sessions/`)**: Source identity references are affected; version-1 JSON shape, validation, references, defaults, and storage behavior are not.
- **Player transport (`internal/player/`)**: Source and generated-client identities are affected; HTTP and ConnectRPC routes, validation, publication, and reconnect behavior are not.
- **Platform and public access (`internal/platform/`, `internal/tunnel/`)**: Source identity and reviewed generated-contract digests are affected; platform paths, desktop operations, secure-store behavior, and provider lifecycle are not.
- **Overseer UI (`frontend/overseer/src/`)**: The generated desktop-binding location is affected; the visible interface and desktop operation allowlist are not.
- **Player UI (`frontend/client/`)**: Generated contract metadata is affected; browser behavior, presentation, audio, and public service consumption are not.
- **Tests and fixtures (`internal/**/*_test.go`, `tests/browser/`, `internal/testutil/`)**: Import paths, binding locations, release-tag fixtures, compatibility digests, and browser import maps are affected.
- **Build and packaging (`go.mod`, `frontend/`, `build/`, `scripts/`)**: The application module identity, release preflight, generated paths, compatibility baseline, and active release guidance are affected; supported targets and archive inventory are not.

### State and Contract Requirements *(include when applicable)*

- **Session/player-config compatibility**: Both formats remain version 1 and require no migration or default change.
- **Wails bridge and event contract**: The generated module-qualified location changes to the v2 identity; the one registered desktop service, 35-method allowlist, named events, payloads, validation, and error behavior remain unchanged.
- **Protobuf/ConnectRPC contract**: Language-package metadata moves to the v2 identity. Protobuf package names, messages, fields, services, RPC cardinality, validation, ordering, publication, and rejection behavior remain unchanged.
- **Reconnect and multi-tab behavior**: Unchanged; reconnecting clients receive the same authoritative state and retain the same recognition rules.
- **HTTP/static contract**: Unchanged; no route, method, response, or origin-policy change is introduced.
- **Runtime-state lifecycle**: Unchanged; the migration creates no new runtime state and changes no shutdown or persistence boundary.

### Security and Privacy Requirements *(include when applicable)*

- The migration MUST NOT widen the generated desktop-service allowlist or expose filesystem, process, environment, provider, credential, or private player capabilities.
- Public descriptors, player payloads, errors, logs, and generated artifacts MUST remain free of credentials and reusable secrets.
- Existing local/LAN and authenticated public-access boundaries MUST remain unchanged.

### Verification Requirements *(mandatory)*

- **Go tests**: Release-tag validation, root-module validation, generated packages, and all affected application packages MUST compile and pass their existing tests under the v2 identity.
- **Race testing**: The repository race suite MUST pass because imports and generated identities cross concurrent runtime packages even though concurrency behavior is unchanged.
- **Browser tests**: The desktop facade and complete browser journey suite MUST resolve the v2 generated-binding location without behavior regressions.
- **Interactive verification**: No new interactive journey is required; existing Overseer and player behavior is unchanged.
- **Packaging/release verification**: Static release preflight MUST accept only v2 stable/prerelease tags, reject other majors before packaging, and preserve the existing five-archive publication contract.

The repository-wide Go lint baseline is defined by `.golangci.yml` and executed with `task lint`.
No numeric coverage threshold is currently defined; verification uses the existing repository-wide
quality, generation, compatibility, frontend-build, and browser gates.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: One hundred percent of tested strict v2 stable and prerelease candidates are accepted, and one hundred percent of tested non-v2 major candidates are rejected before packaging.
- **SC-002**: All active application packages resolve under one v2 source identity, with zero active imports or generated application-binding paths using the previous identity.
- **SC-003**: Two clean generations produce identical contract and desktop-binding outputs, and every established breaking-change fixture remains rejected against the advanced baseline.
- **SC-004**: Representative version-1 sessions and player configurations complete open, save, and reopen checks with unchanged business content and established version values.
- **SC-005**: All repository Go, lint, race, frontend-build, compatibility, binding, and affected browser checks pass with zero new warnings treated as failures.
- **SC-006**: Active release documentation contains zero stable or prerelease examples whose major version differs from the accepted application module major.

## Assumptions

- The next stable application release is `v2.0.0`, with `v2.0.0-rc.1` available for prerelease acceptance.
- Application release major version and root application module major version remain aligned for the v2 release line.
- Version-1 session and player-configuration numbers are persistence-contract versions independent of the application release major.
- Development-tool modules are independently owned and are not versioned by the root application's release tag.
- Existing generated protocol and desktop contracts are behaviorally accepted and require identity regeneration rather than functional redesign.

## Out of Scope

- Application self-update discovery, download, replacement, or relaunch behavior.
- Embedding the application release version in platform metadata or the running executable.
- Checksums, signing, notarization, installers, or changes to the five-archive release inventory.
- A future major-version-3 migration or support for publishing multiple application majors from one source revision.
- Session, player-configuration, protobuf wire-package, RPC, gameplay, authority, or security migrations.
