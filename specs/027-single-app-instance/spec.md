# Feature Specification: Single App Instance

## User Scenarios & Testing

### User Story 1 - Prevent duplicate desktop instances (Priority: P1)

As an Overseer, I can launch Fallout Terminal repeatedly without creating duplicate desktop instances, so only one copy owns the local services, windows, and campaign state for my login session.

**Why this priority**: Preventing duplicate ownership of local resources and state is the core safety and reliability value of this feature.

**Independent Test**: Launch the desktop application, attempt several additional launches from the same operating-system login session, and confirm that exactly one interactive application remains active with no duplicate window or local service.

**Acceptance Scenarios**:

1. **Given** Fallout Terminal is not running, **When** the user launches it, **Then** one interactive desktop instance starts normally.
2. **Given** Fallout Terminal is already running, **When** the user launches it again, **Then** the later launch exits without creating another window, local service, or state owner.
3. **Given** several desktop launches begin at nearly the same time, **When** launch coordination settles, **Then** exactly one instance remains active and all others exit cleanly.
4. **Given** the active instance has exited, **When** the user launches Fallout Terminal again, **Then** the new launch becomes the active instance without manual recovery.

### User Story 2 - Return to the active window (Priority: P2)

As an Overseer who launches Fallout Terminal while it is already open, I am returned to the existing Overseer window so the repeated launch behaves like a request to resume work.

**Why this priority**: Reactivating the existing window makes duplicate prevention understandable and avoids making the application appear unresponsive.

**Independent Test**: Start Fallout Terminal, minimize or place its window behind another application, launch Fallout Terminal again, and confirm the original window becomes visible and receives focus while no new instance remains.

**Acceptance Scenarios**:

1. **Given** the active Overseer window is minimized, **When** a second desktop launch occurs, **Then** the existing window is restored and focused.
2. **Given** the active Overseer window is behind other windows, **When** a second desktop launch occurs, **Then** the existing window is brought to the foreground.
3. **Given** a second launch occurs before the active instance has finished creating its window, **When** the launch is handled, **Then** no duplicate instance survives and the active instance continues startup without an error.

## Edge Cases

- Several processes may race to become the active instance during a cold start; only one may win ownership.
- A second launch may arrive after the active process owns the application identity but before its window is ready.
- The active window may be minimized, hidden behind other windows, or already focused.
- The active process may terminate normally or unexpectedly; a stale ownership claim must not permanently block later launches.
- Arguments, working directories, or other data supplied by a later launch may be malformed or hostile and must not trigger commands or state changes.
- Informational and maintenance invocations that do not start the interactive desktop application must continue to run without depending on desktop-instance ownership.

## Requirements

### Functional Requirements

- **FR-001**: The product MUST allow at most one interactive Fallout Terminal desktop instance for the same operating-system login session at a time.
- **FR-002**: A later desktop launch MUST exit without creating an additional application window, local service, or owner of campaign state.
- **FR-003**: A later desktop launch MUST request that the active Overseer window be restored and focused when that window is available.
- **FR-004**: Concurrent or near-concurrent desktop launches MUST resolve to one active instance without crashing or disrupting that instance's startup.
- **FR-005**: Normal or unexpected termination of the active instance MUST release ownership so the next desktop launch can start without manual cleanup.
- **FR-006**: The application identity used to coordinate launches MUST remain stable across application restarts and version upgrades.
- **FR-007**: Data received from a later launch MUST be treated as untrusted and MUST NOT execute commands, navigate content, or modify application state.
- **FR-008**: Informational and maintenance invocations that do not start the interactive desktop application MUST preserve their existing behavior without requiring desktop-instance ownership.
- **FR-009**: Duplicate prevention and active-window reactivation MUST have the same user-visible behavior on every supported desktop operating system.
- **FR-010**: Duplicate prevention MUST be enabled by default and MUST require no user configuration.

### Key Entities

- **Active Application Instance**: The single interactive Fallout Terminal process that owns the application identity for one operating-system login session and may create the Overseer window and local services.
- **Later Launch**: Any additional interactive launch attempt while an active application instance owns the same identity; it may request window reactivation but cannot become another state owner.
- **Application Identity**: The stable product-level identity used to distinguish Fallout Terminal from other applications and coordinate ownership across launches and upgrades.

## Success Criteria

### Measurable Outcomes

- **SC-001**: In a 20-attempt rapid-launch test, exactly one interactive Fallout Terminal instance remains active after launch coordination settles.
- **SC-002**: Every later launch creates zero additional Overseer windows, local listeners, or campaign-state owners.
- **SC-003**: When the active window is ready, a later launch restores and focuses it within two seconds on each supported desktop operating system.
- **SC-004**: After the active instance terminates, the next launch starts successfully on its first attempt with no manual lock or process cleanup.
- **SC-005**: Automated acceptance coverage exercises the initial launch configuration, a later launch with a ready window, and a later launch before window readiness.
- **SC-006**: Existing informational and maintenance launch checks continue to pass with zero behavior changes.

## Assumptions

- Single-instance ownership is scoped to one operating-system login session; separate users on the same machine may run their own instance.
- A repeated interactive launch is interpreted only as a request to reactivate the existing Overseer window; launch arguments and other transferred data are ignored.
- No notification or error dialog is required when a later launch exits because bringing the existing window forward supplies the user feedback.
- Version reporting and the application-update replacement helper are non-interactive invocations and remain outside this feature's ownership scope.
- The existing supported desktop operating-system matrix remains unchanged.

## Approach

- Configure the Wails application host in `wails_host.go` with a stable Fallout Terminal single-instance identity and a concurrency-safe activation handler that tolerates a window that is not ready yet.
- Bind the Overseer window to that handler from `main.go` before the desktop event loop runs, while leaving version reporting and update-helper dispatch unchanged.
- Extend `wails_host_test.go` to verify the configured identity and the restore/focus behavior for ready, unavailable, and repeated activation requests.
- Use the existing Wails runtime and repository validation workflows; add no dependency or user-facing configuration.
