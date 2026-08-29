# Feature Specification: Manual Public Access Control

**Created**: 2026-08-29
**Status**: Draft

## User Scenarios & Testing

### User Story 1 - Control public access explicitly (Priority: P1)

As a game master, I explicitly enable and stop public access from the main window so that opening the application or saving settings never changes external availability on my behalf.

**Why this priority**: Public exposure is a deliberate operational action and must remain under the game master's direct control.

**Independent Test**: Launch with previously saved public-access settings, save credential or domain changes, and verify that no public endpoint starts or stops until the matching main-window action is selected.

#### Acceptance Scenarios

1. **Given** public-access settings were saved previously, **when** the application starts, **then** public access remains stopped until the game master selects the enable action in the main window.
2. **Given** public access is stopped, **when** the game master saves settings or changes an ngrok token, **then** the settings are saved without starting public access.
3. **Given** public access is active, **when** the game master selects the stop action in the main window, **then** the public endpoint is stopped through the existing explicit lifecycle control.

### User Story 2 - Configure only actionable settings (Priority: P1)

As a game master, I see only connection and player-login settings in the dialog so that every displayed control has an immediate and understandable effect.

**Why this priority**: Removing the ineffective startup checkbox prevents false expectations and shortens an already dense settings dialog.

**Independent Test**: Open the public-access settings dialog and verify that connection and player-login groups lead directly to the footer, with no behavior group or startup checkbox.

#### Acceptance Scenarios

1. **Given** the settings dialog opens, **when** the game master scans it from top to bottom, **then** the ngrok connection group is followed by the player-login group and then the footer actions.
2. **Given** the settings dialog is visible, **when** controls and labels are inspected, **then** no behavior group or option to enable public access at application startup is present.
3. **Given** an older saved configuration contains a startup preference, **when** it is loaded, **then** the preference is not presented and has no effect on public-access state.

## Edge Cases

- An existing settings file contains the former startup preference set to true.
- A credential replacement is saved while public access is stopped.
- Settings are edited while public access is active and the existing restart confirmation is declined.
- Public access fails to start; local and LAN access remain available and the user may retry explicitly.

## Requirements

### Functional Requirements

- **FR-001**: The public-access settings dialog MUST contain only the ngrok connection and player-login groups before its footer.
- **FR-002**: The settings dialog MUST NOT display a behavior group or an option to enable public access when the application starts.
- **FR-003**: Application launch MUST leave public access stopped regardless of any previously saved startup preference.
- **FR-004**: Enabling and stopping public access MUST remain explicit actions available from the main window.
- **FR-005**: Saving settings or credential changes MUST NOT independently enable or stop public access, except for the already-established confirmed restart of an active endpoint when its configuration changes.
- **FR-006**: The former startup preference MUST remain inert for backward compatibility and MUST be written as disabled by new settings mutations.
- **FR-007**: Automated checks MUST cover the simplified dialog, legacy-preference loading, settings saves, and explicit main-window lifecycle actions.

## Success Criteria

### Measurable Outcomes

- **SC-001**: The settings dialog presents exactly two named groups before the footer: ngrok connection and player login.
- **SC-002**: No startup checkbox, behavior legend, or equivalent automatic-start control is visible or keyboard reachable.
- **SC-003**: Loading a saved true startup preference produces zero automatic public-access start requests.
- **SC-004**: Automated presentation journeys pass for explicit enable, explicit stop, settings save, and legacy-preference cases.

## Assumptions

- The existing enable and stop buttons in the main window remain the authoritative lifecycle controls.
- The existing active-access confirmation remains necessary when saving changes that require restarting an already active endpoint.
- The serialized and private-contract field for the former preference remains in place only for compatibility; removing or renumbering that field is outside this change.
- Credential management, domain validation, player login, secure storage, and tunnel implementation remain unchanged.

## Approach

- Remove the behavior fieldset and its obsolete checkbox styling from the public-access settings dialog.
- Remove checkbox state handling from the Overseer controller and submit the compatibility preference as disabled in settings mutations.
- Update embedded-asset and browser checks to prove the dialog has two groups, legacy true values do not start access, and lifecycle changes remain explicit.
