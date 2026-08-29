# Contract: Public Access Credential Management and Sharing

## Private Desktop Method

### `CopyPublicAccessCredentials()`

- Exposed only through `desktopService` to the trusted Overseer window.
- Accepts no password, username, revision, or arbitrary clipboard text from the page.
- Returns the existing `CommandResult` shape:
  - `ok: true`, no secret-bearing fields, after the native clipboard accepts the credential block.
  - `ok: false`, `error: <safe message>` when preferences, secure storage, stored password validation, or clipboard access fails.
- MUST NOT return, emit, log, or persist the saved password.

## Clipboard Text

```text
Логин: <saved login>
Пароль: <saved password>
```

## UI Contract

Main public-access settings player group:

- `#publicAccessUsernameSummary`: read-only saved login text.
- `#publicAccessPasswordMask`: displays `*****` only for present password state.
- `#publicAccessPasswordPresence`: absence/unavailable status when the mask is not shown.
- `#btnOpenPublicAccessPlayerCredentials`: opens the child dialog.
- `#btnSharePublicAccessCredentials`: invokes the native share command and is disabled unless credentials are shareable.
- `#publicAccessSettingsCopyStatus`: announces share success or safe failure.

Player-credential dialog:

- `#publicAccessPlayerCredentialsDialog`: modal child dialog.
- `#publicAccessPlayerCredentialsForm`: replacement form.
- `#publicAccessReplacementUsername`: current login draft.
- `#publicAccessReplacementPlayerPassword`: empty replacement password input.
- `#btnSavePublicAccessPlayerCredentials`: saves the draft.
- `#btnDeletePublicAccessPlayerCredentials`: deletes the saved password and restores `players`.
- `#btnCancelPublicAccessPlayerCredentials`: closes without mutation.
- `#publicAccessPlayerCredentialsError`: safe mutation error status.

## Accessibility and Focus

- Opening the child dialog focuses the login field.
- Escape and Cancel clear the password draft and return focus to the Edit action.
- A pending command disables settings, child-dialog, and Share controls.
- The fixed mask is text presentation, never a password input value.
