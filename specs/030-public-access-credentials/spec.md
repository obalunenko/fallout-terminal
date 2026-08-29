# Feature Specification: Public Access Credential Editing

**Created**: 2026-08-28
**Status**: Draft

## User Scenarios & Testing

### User Story 1 - Manage an already configured ngrok token (Priority: P1)

As a game master, I can see that an ngrok token is configured without mistaking an empty replacement field for a missing credential, and I can deliberately replace or delete it when needed.

**Why this priority**: Returning users should understand their credential state immediately without exposing the saved secret or encouraging unnecessary replacement.

**Independent Test**: Load settings with a saved token, verify that no token input is shown in the main dialog, open the token-management dialog, and exercise replacement, deletion, dismissal, and failure outcomes.

#### Acceptance Scenarios

1. **Given** an ngrok token is saved, **when** the settings dialog opens, **then** it shows a configured status and an action to change the token instead of an empty token input.
2. **Given** a saved token, **when** the game master selects the change action, **then** a focused modal explains that the current token cannot be viewed and offers a new secret input, replacement, deletion, and cancellation.
3. **Given** a replacement token is submitted successfully, **when** the operation completes, **then** the token dialog closes and the main settings dialog still reports the token as configured without revealing its value.
4. **Given** replacement or deletion fails, **when** the operation completes, **then** the token dialog remains open, reports the failure, and keeps non-secret settings intact.
5. **Given** the token dialog is dismissed, **when** it closes, **then** no credential mutation is submitted and focus returns to the change-token control.

### User Story 2 - Scan settings in task order (Priority: P1)

As a game master, I can move through provider connection, player login, and startup behavior as clearly separated groups so that occasional technical setup is easier to understand.

**Why this priority**: The dialog contains two different credential domains and an optional behavior preference; grouping them prevents configuration mistakes.

**Independent Test**: Open the settings dialog and verify the visual and keyboard order of the ngrok connection group, player-login group, behavior group, and footer actions.

#### Acceptance Scenarios

1. **Given** the settings dialog opens, **when** the game master scans from top to bottom, **then** ngrok connection settings appear before player-login settings and startup behavior appears last.
2. **Given** the player-login group is visible, **when** the password field is inspected, **then** password generation is adjacent to that field rather than competing with the primary save action in the footer.
3. **Given** the game master reaches the dialog footer, **when** actions are presented, **then** cancellation is secondary and saving all settings is the single primary action.

### User Story 3 - Configure a missing token directly (Priority: P2)

As a first-time game master, I can enter an ngrok token directly in the connection group when no token is saved.

**Why this priority**: The configured-state simplification must not add an extra step to initial setup.

**Independent Test**: Load settings without a saved token and verify that the main dialog shows a new-token input and saves it through the existing settings action.

#### Acceptance Scenarios

1. **Given** no ngrok token is saved, **when** the settings dialog opens, **then** the connection group shows a password-style token input rather than the configured-token summary.
2. **Given** the missing token is entered with the remaining settings, **when** the game master saves successfully, **then** the dialog closes and the next settings visit shows the configured-token summary.

## Edge Cases

- Credential presence is unknown because secure storage is unavailable; it must be treated as not configured without implying that a token can be read back.
- The token dialog is dismissed while no command is pending; its transient token value must be cleared immediately.
- Replacement or deletion is requested while public access is active; the existing restart confirmation and lifecycle ordering must remain in force.
- A delayed credential result arrives after a newer settings revision; stale state must not overwrite the current dialog or panel.
- Deleting the token makes direct public-access startup unavailable but must not affect local or LAN access.
- Saving unrelated settings while a token is already configured must not replace or delete the token.

## Requirements

### Functional Requirements

- **FR-001**: The settings dialog MUST group controls in this order: ngrok connection, player login, startup behavior, and footer actions.
- **FR-002**: When token presence is configured, the main settings dialog MUST hide the token input and MUST show a configured status with an explicit change-token action.
- **FR-003**: When token presence is absent or unknown, the main settings dialog MUST show a password-style token input for initial configuration.
- **FR-004**: The change-token action MUST open a modal that states the saved token cannot be viewed and offers replacement, deletion, and cancellation without revealing the stored value.
- **FR-005**: Successful replacement or deletion MUST refresh credential presence and close the token modal, while failure MUST keep it open and display the error.
- **FR-006**: Dismissing the token modal MUST clear transient secret input, submit no mutation, and restore focus to the change-token action.
- **FR-007**: Password generation MUST be presented adjacent to the player-password field, while save remains the sole primary footer action and cancellation remains secondary.
- **FR-008**: The revised interaction MUST preserve active-access restart confirmation, stale-result suppression, one-time generated-password handling, native clipboard fallback, and local/LAN failure isolation.
- **FR-009**: The application MUST continue to expose saved credentials only through presence, replacement, and deletion and MUST NOT reconstruct or display a stored secret.
- **FR-010**: Automated checks MUST cover configured, absent, replacement-success, replacement-failure, deletion, dismissal, active-access, and unknown-presence paths.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A returning game master can identify that the ngrok token is configured without seeing an empty credential field.
- **SC-002**: Replacing or deleting a configured token is reachable through one change-token action and one focused modal.
- **SC-003**: Initial token setup remains available directly in the main settings dialog when no token is saved.
- **SC-004**: The primary reading order contains exactly three named groups before the footer: connection, player login, and behavior.
- **SC-005**: Automated presentation journeys pass for every credential-presence and token-mutation outcome named in FR-010.

## Assumptions

- The existing settings mutation remains the authority for token replacement and deletion; no credential-read operation is introduced.
- Unknown credential presence follows the safer first-use presentation and may fail on save with the existing secure-store error.
- Token replacement and deletion share the same active-access confirmation already used by public-access settings changes.
- The password-management security model remains unchanged; this feature only repositions its existing actions.
- The surrounding compact public-access panel and backend tunnel lifecycle remain unchanged.

## Approach

- Reorder and group the existing public-access settings markup in `frontend/overseer/src/index.html`, adding a configured-token summary and a focused token-management dialog.
- Adapt `frontend/overseer/src/overseer.js` to render configured versus missing token states, route replacement/deletion through the existing settings mutation, and preserve focus and transient-secret cleanup.
- Extend `frontend/overseer/src/overseer.css` with compact grouped-field and credential-dialog layouts that reuse the established modal system.
- Update embedded-asset and Playwright coverage without changing public-access protobuf contracts, secure storage, or tunnel services.
