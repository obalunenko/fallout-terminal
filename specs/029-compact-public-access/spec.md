# Feature Specification: Compact Public Access

**Created**: 2026-08-28
**Status**: Draft

## User Scenarios & Testing

### User Story 1 - Control public access at a glance (Priority: P1)

As a game master, I can see whether public access is stopped, changing, ready, or failed and use one lifecycle action without navigating through technical settings.

**Why this priority**: Starting and stopping access is the frequent operation and must remain obvious during a live session.

**Independent Test**: Load each public-access runtime state and verify that the compact panel shows the state, the relevant address, and exactly one available lifecycle action.

#### Acceptance Scenarios

1. **Given** public access is stopped and valid credentials are already saved, **when** the game master views the panel, **then** the panel shows the stopped state, the configured reserved domain when present, and an action to enable access.
2. **Given** public access is ready, **when** the game master views the panel, **then** the panel shows the ready state, the active public URL, a copy action, and an action to stop access.
3. **Given** public access is starting or stopping, **when** the game master views the panel, **then** the transition is visible and the lifecycle action cannot be submitted again.
4. **Given** public access fails, **when** the failure is reported, **then** the compact panel shows the failure without implying that local access has stopped.

---

### User Story 2 - Configure public access only when needed (Priority: P1)

As a game master, I can open public-access settings in a familiar dialog so that ngrok credentials and player authentication do not occupy the everyday workspace.

**Why this priority**: Technical configuration is necessary for first use and occasional maintenance, but it should not compete with terminal authoring controls.

**Independent Test**: Open the settings dialog from the compact panel, inspect and update every existing public-access setting, save it, and return focus to the panel.

#### Acceptance Scenarios

1. **Given** the game master selects public-access settings, **when** the dialog opens, **then** it contains the existing setup guide, preference, reserved domain, player name, ngrok token, player password, secret-presence controls, and password generation actions.
2. **Given** settings are changed successfully, **when** the game master saves them, **then** the dialog closes and the compact panel immediately reflects the saved domain and latest status.
3. **Given** settings cannot be saved, **when** the operation completes, **then** the dialog remains open and displays the error without discarding entered non-secret values.
4. **Given** the dialog is dismissed, **when** it closes, **then** focus returns to the settings control and no settings are submitted implicitly.

---

### User Story 3 - Recover from missing first-use configuration (Priority: P2)

As a first-time game master, I am directed to the required configuration instead of receiving an unexplained start failure.

**Why this priority**: The compact surface removes fields from view, so it must retain a clear path to completing initial setup.

**Independent Test**: Load a snapshot without a saved provider token or player password, select the enable action, and verify that the settings dialog opens with a clear setup prompt and no tunnel start is requested.

#### Acceptance Scenarios

1. **Given** either required secret is not saved, **when** the game master selects the enable action, **then** the settings dialog opens and identifies that setup is required before public access can start.
2. **Given** the required settings have been saved, **when** the game master subsequently selects the enable action, **then** startup is requested directly without reopening the dialog.

## Edge Cases

- No reserved domain is configured before startup, so no address is shown until ngrok provides the active public URL.
- A configured domain changes while the tunnel is active and the existing restart confirmation remains required.
- A delayed status event arrives after a newer generation or settings revision and must not restore stale panel or dialog content.
- A secure credential store reports unknown presence, which must be treated as not ready for direct startup.
- The settings dialog is closed while a save or password-generation command is pending.
- Clipboard access is unavailable and the existing native clipboard fallback must remain usable.

## Requirements

### Functional Requirements

- **FR-001**: The application MUST replace the always-visible public-access form with a compact surface that presents runtime status, available public address, lifecycle control, and access to settings.
- **FR-002**: The compact surface MUST display the active public URL when access is ready and MUST display the configured reserved domain while stopped when one exists.
- **FR-003**: The compact surface MUST offer URL copying only when an address is available and MUST expose copy success or failure through an accessible status message.
- **FR-004**: The compact surface MUST present one lifecycle action appropriate to the current state: enable while stopped or failed, stop while ready, and no actionable duplicate while starting or stopping.
- **FR-005**: The application MUST place all existing public-access configuration fields, guidance, secret-presence controls, save behavior, and password-generation behavior inside a modal settings dialog.
- **FR-006**: The settings dialog MUST follow the established application dialog interaction pattern, including explicit dismissal, modal focus containment, Escape handling, and focus restoration.
- **FR-007**: Selecting enable without both required saved secrets MUST open the settings dialog with a clear setup-required message and MUST NOT request tunnel startup.
- **FR-008**: Saving settings successfully MUST refresh the compact surface and close the dialog, while a failed save MUST keep the dialog open and show the failure.
- **FR-009**: Existing stale-snapshot suppression, active-access restart confirmations, one-time generated-password handling, secret clearing, and native clipboard fallback MUST remain intact.
- **FR-010**: The change MUST preserve the public-access backend contract, secure credential storage policy, tunnel lifecycle, and the surrounding Overseer layout.

## Success Criteria

### Measurable Outcomes

- **SC-001**: In stopped and ready states, a game master can identify status, address availability, and the next lifecycle action from the compact panel without scrolling.
- **SC-002**: Starting or stopping already-configured public access requires one lifecycle selection from the compact panel.
- **SC-003**: Every existing public-access setting and configuration action remains reachable within one dialog opened from the compact panel.
- **SC-004**: A first-use enable attempt reaches the required settings dialog without issuing a failed tunnel-start request.
- **SC-005**: Automated frontend asset checks cover stopped, ready, transitioning, missing-configuration, dialog save-success, and dialog save-failure presentation paths.

## Assumptions

- The reserved domain remains optional; when absent, the runtime-generated URL appears only after startup succeeds.
- The compact panel remains in the existing right-hand public-access section and the rest of the application window is unchanged.
- The existing save command remains the single authority for validating and persisting public-access preferences and secrets.
- The settings dialog may reuse established terminal-management and player-session dialog styling instead of introducing a new visual system.

## Approach

- Replace the inline form in `frontend/overseer/src/index.html` with compact status, address, lifecycle, copy, and settings controls; move the existing form and guide into a modal dialog.
- Adapt `frontend/overseer/src/overseer.js` so render and event handling coordinate the compact lifecycle control, dialog state, first-use routing, successful close, and focus restoration without changing desktop API calls.
- Refine `frontend/overseer/src/overseer.css` using existing dialog and responsive conventions, then extend the repository's embedded-asset checks for the new structure and copy.
- Keep the native public-access service, bindings, secret storage, and tunnel implementation unchanged.
