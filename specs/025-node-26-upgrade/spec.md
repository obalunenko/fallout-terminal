# Feature Specification: Node.js 26 Upgrade

**Created**: 2026-08-27
**Status**: Draft

## User Scenarios & Testing

### User Story 1 - Use One Current Node.js Runtime (Priority: P1)

As a maintainer, I can install dependencies, build both frontend applications, and run browser tests with the same current Node.js 26 release declared throughout the repository, so local and automated results do not depend on an older runtime.

**Why this priority**: A single supported runtime is the core outcome and prevents local development from drifting away from automation.

**Independent Test**: Use a clean environment with Node.js 26.8.1, install every locked JavaScript dependency set, build both frontend applications, and run the browser-test suite successfully.

**Acceptance Scenarios**:

1. **Given** a clean checkout and Node.js 26.8.1, **When** a maintainer installs the locked frontend dependencies, **Then** installation completes without an engine mismatch from any project-owned package.
2. **Given** the locked dependencies are installed with Node.js 26.8.1, **When** the maintainer builds the player and Overseer frontends, **Then** both builds complete successfully.
3. **Given** the browser-test dependencies are installed with Node.js 26.8.1, **When** the maintainer runs the browser tests, **Then** the suite completes without a project-runtime incompatibility.
4. **Given** any project-owned JavaScript package declaration, **When** its supported runtime is inspected, **Then** it identifies Node.js 26.8.1 as the minimum supported version.

---

### User Story 2 - Run Quality and Releases on Node.js 26 (Priority: P1)

As a release maintainer, I can trust that pull-request quality checks and every portable release target use Node.js 26.8.1, so the runtime tested before merge is also the runtime used to produce release artifacts.

**Why this priority**: Release confidence requires the local runtime contract and every automated build path to agree.

**Independent Test**: Run the repository quality workflow and one complete five-target portable release workflow, then verify that every JavaScript-dependent job selects Node.js 26.8.1 and succeeds.

**Acceptance Scenarios**:

1. **Given** a pull request or main-branch update, **When** repository quality automation runs, **Then** its JavaScript-dependent work uses Node.js 26.8.1.
2. **Given** a governed release tag, **When** the portable release matrix runs, **Then** every target that builds frontend assets uses Node.js 26.8.1.
3. **Given** a complete quality or release run, **When** its annotations are inspected, **Then** no warning reports use of a deprecated Node.js 20 runtime by an upgradeable automation component.
4. **Given** the Node.js upgrade is complete, **When** all project-owned runtime declarations are compared, **Then** none still selects or permits Node.js 20 as the repository's supported runtime.

## Edge Cases

- What happens if the current Node.js 26 release is not yet present on one of the Windows, Linux, or macOS automation runners?
- How is a partial update prevented when one workspace or browser-test manifest still declares the former minimum version?
- How are root lockfile runtime declarations synchronized without rewriting runtime constraints owned by third-party dependencies?
- What happens if a frontend dependency is incompatible with Node.js 26 despite allowing an older runtime?
- How are warnings from an automation component's internal runtime distinguished from the Node.js version selected for project commands?
- How is the upgrade kept reproducible after a newer Node.js 26 patch release becomes available?

## Requirements

### Functional Requirements

- **FR-001**: Every project-owned JavaScript package MUST declare Node.js 26.8.1 as its minimum supported runtime.
- **FR-002**: Every quality and release automation job that executes JavaScript project commands MUST select Node.js 26.8.1.
- **FR-003**: Project-owned entries in committed JavaScript lockfiles MUST reflect the same Node.js 26.8.1 minimum as their corresponding package declarations.
- **FR-004**: The upgrade MUST preserve third-party dependency runtime metadata rather than rewriting constraints owned by external packages.
- **FR-005**: Locked dependency installation, both frontend builds, and the browser-test suite MUST succeed under Node.js 26.8.1.
- **FR-006**: All five supported portable release targets MUST remain buildable after the runtime upgrade.
- **FR-007**: Upgradeable automation components used by the quality and portable-release workflows MUST NOT depend on a deprecated Node.js 20 internal runtime.
- **FR-008**: The completed upgrade MUST leave no active project-owned runtime declaration selecting Node.js 20.19.0.
- **FR-009**: Application behavior, frontend dependency versions, and the supported release-target inventory MUST remain unchanged unless a compatibility change is required to run successfully on Node.js 26.8.1.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All four project-owned JavaScript package declarations and both root lockfile records identify Node.js 26.8.1 as the minimum supported runtime.
- **SC-002**: Both JavaScript-dependent automation workflows select Node.js 26.8.1 in every applicable job.
- **SC-003**: A clean locked installation and both frontend builds complete successfully with Node.js 26.8.1.
- **SC-004**: The complete browser-test suite passes with Node.js 26.8.1.
- **SC-005**: One complete portable-release run produces all five expected target archives after the upgrade.
- **SC-006**: Automated inspection finds zero active project-owned references to Node.js 20.19.0.
- **SC-007**: Quality and release runs report zero deprecated-Node.js-20 warnings attributable to automation components for which a maintained nondeprecated revision exists.
- **SC-008**: Existing application acceptance tests report zero behavior regressions caused by the runtime upgrade.

## Assumptions

- The official Node.js release index identifies version 26.8.1, released on 2026-08-26, as the latest Node.js 26 release when this specification is created.
- Node.js 26 is not an LTS line at specification time; the explicit request for the latest version 26 takes precedence over an LTS-only policy.
- The supported local runtime floor moves directly to Node.js 26.8.1; compatibility with Node.js 20, 22, or 24 is not retained as a project guarantee.
- Lockfiles are regenerated only as needed to synchronize project-owned runtime declarations; dependency versions remain pinned.
- Automation components may use an upstream-managed internal runtime other than Node.js 26, provided it is supported and no longer triggers the deprecated Node.js 20 warning.
- This feature does not change application functionality, add JavaScript dependencies, or alter the five-target release inventory.
