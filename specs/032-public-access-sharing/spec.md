# Feature Specification: Public Access Credential Sharing

**Created**: 2026-08-29
**Status**: Draft

## User Scenarios & Testing

### User Story 1 - Recognize saved player credentials safely (Priority: P1)

As a game master, I can immediately see the saved player login and whether a password exists without exposing the password on screen.

**Why this priority**: Returning users need an accurate, safe summary before they decide to edit or share access details.

**Independent Test**: Open public-access settings with saved player credentials and verify that the login is shown, the password is represented by a fixed five-character mask, and no saved password value appears in the page.

#### Acceptance Scenarios

1. **Given** a player login and password are saved, **when** public-access settings open, **then** the login is displayed as saved and the password is displayed only as `*****`.
2. **Given** no player password is saved, **when** public-access settings open, **then** the interface reports that the password is not saved and does not show the fixed mask.
3. **Given** the saved password has any supported length, **when** it is represented in settings, **then** the visible mask remains exactly five asterisks and does not reveal its length.

### User Story 2 - Edit player credentials in a focused dialog (Priority: P1)

As a game master, I can open a dedicated player-credential dialog to replace the login or password, remove the saved credentials, or cancel without changing them.

**Why this priority**: Credential changes are sensitive and should be separated from ordinary ngrok and domain settings, following the existing token-management interaction.

**Independent Test**: Open the credential dialog from configured and unconfigured states, exercise replacement, deletion, cancellation, validation failure, and successful save, and verify focus and transient-field cleanup.

#### Acceptance Scenarios

1. **Given** saved player credentials exist, **when** the change action is selected, **then** a focused dialog shows the current login, an empty replacement-password field, and actions to save, delete, or cancel.
2. **Given** a replacement login and password are valid, **when** they are saved, **then** both become the active player credentials and the main settings summary refreshes without revealing the password.
3. **Given** the password field is left empty while a password is already saved, **when** a changed login is saved, **then** the saved password is preserved.
4. **Given** saved credentials exist, **when** deletion succeeds, **then** the saved password is removed, the login returns to the default player login, and sharing becomes unavailable until a password is saved again.
5. **Given** a mutation fails or the dialog is cancelled, **when** control returns to the dialog or settings window, **then** no secret value is exposed, unrelated settings remain intact, and focus returns to the initiating control.

### User Story 3 - Share saved login details with players (Priority: P1)

As a game master, I can use one Share action in public-access settings to copy both the saved login and saved password in a readable block for sending to players.

**Why this priority**: Players need both Basic Auth values, and copying them together avoids omissions and manual transcription errors.

**Independent Test**: With saved credentials, select Share and verify that the system clipboard receives both values while the password never appears in the page, result payload, logs, or reusable application state.

#### Acceptance Scenarios

1. **Given** a saved login and password are available, **when** Share is selected, **then** one readable clipboard value contains both the login and password and a success status is announced.
2. **Given** the password is absent or unavailable, **when** public-access settings render, **then** Share is unavailable and no credential-read request is made.
3. **Given** secure storage or clipboard access fails, **when** Share is selected, **then** the settings window reports a safe error and no password is returned to the page or retained in application state.

## Edge Cases

- Secure credential storage reports an unknown, locked, denied, or unavailable password state.
- A saved password is shorter or longer than the visible five-character mask.
- The login changes while public access is active and the existing restart confirmation is declined.
- A credential mutation completes after a newer public-access snapshot has already arrived.
- The player-credential dialog is dismissed while a generated or manually entered password is transiently present.
- Clipboard writing fails after the password has been read from secure storage.
- The saved password exists but the non-secret login is empty or invalid because of legacy or corrupt settings.

## Requirements

### Functional Requirements

- **FR-001**: The public-access settings window MUST show the saved player login as read-only summary information rather than an inline editing field.
- **FR-002**: When a saved player password is present, the settings window MUST display exactly `*****` regardless of the password's actual length.
- **FR-003**: When a saved player password is absent or unavailable, the settings window MUST communicate that state without displaying the fixed mask.
- **FR-004**: Player-login editing MUST occur in a dedicated modal dialog patterned after the existing ngrok token dialog.
- **FR-005**: The player-credential dialog MUST support replacing the login and password together, changing the login while preserving an existing password, deleting the saved credential pair, and cancelling without mutation.
- **FR-006**: Deleting saved player credentials MUST remove the saved password and restore the default player login.
- **FR-007**: Every credential-dialog close path MUST clear transient password values and restore focus appropriately.
- **FR-008**: The settings window MUST provide one Share action that is enabled only when a valid saved login and password are available.
- **FR-009**: The Share action MUST copy one readable value containing both the saved login and saved password.
- **FR-010**: The saved password MUST be read only inside a bounded trusted operation that writes the clipboard, clears temporary buffers, and returns only success or a safe error to the page.
- **FR-011**: The implementation MUST preserve active-access confirmation, stale-snapshot suppression, generated-password handling, credential validation, and local/LAN failure isolation.
- **FR-012**: Automated checks MUST cover fixed masking, configured and unconfigured dialogs, replacement, preservation, deletion, cancellation, sharing success, and secure-store or clipboard failure.

## Key Entities

- **Player Login Credentials**: The non-secret login name and secure saved password used by players for Basic Auth; the password may be present, absent, or unavailable.
- **Credential Summary**: The read-only settings representation containing the saved login, password presence state, fixed mask when present, and Edit and Share actions.
- **Share Operation**: A one-shot trusted action that reads the saved password, combines it with the saved login for the clipboard, and exposes only the outcome to the interface.

## Success Criteria

### Measurable Outcomes

- **SC-001**: For every saved password from 8 through 128 supported characters, the settings window displays exactly five asterisks and zero password characters.
- **SC-002**: Editing or deleting player credentials is reachable through one change action and one focused dialog.
- **SC-003**: A successful Share action places both required player-login values in the clipboard with one interaction.
- **SC-004**: No saved password appears in page text, form defaults, events, logs, snapshots, or command results during viewing, editing, or sharing.
- **SC-005**: All automated journeys named in FR-012 pass without regressing explicit public-access lifecycle control.

## Assumptions

- The default player login remains `players` and is restored when the credential pair is deleted.
- An empty replacement-password field preserves the saved password only when one is already present; initial setup still requires a valid password.
- The Share action copies a two-line, human-readable login/password block suitable for pasting into a message.
- Clipboard contents are intentionally user-controlled output and remain subject to the operating system's normal clipboard lifetime.
- The ngrok token, reserved domain, public URL controls, and explicit enable/stop actions remain outside this credential-dialog change.

## Verbatim Constraints

- Saved player passwords are represented by the exact fixed mask `*****`.
