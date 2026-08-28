# Research: Public Access Credential Sharing

## Decision: Copy saved credentials entirely inside the native boundary

**Rationale**: The existing secure-store contract exposes secrets only to scoped callbacks and clears their byte buffers afterward. A new native command can read only the player password, format the saved login/password pair, write it through an injected clipboard interface, and return the existing secret-free command result. The browser never receives the password, so page state, events, snapshots, and generated bindings remain reusable and secret-free.

**Alternatives considered**:

- Return the stored password to JavaScript and use the existing runtime clipboard helper. Rejected because the password would cross into page memory and become observable through command results.
- Keep only manual password copying. Rejected because it cannot share an already saved password, which is the requested workflow.
- Read both ngrok and player secrets through the existing combined helper. Rejected because sharing player credentials must not require or expose the provider token.

## Decision: Add a player-password scoped secure-store helper

**Rationale**: A dedicated helper preserves the existing no-`Get` design, validates the stored player password, guarantees callback-scoped clearing, and redacts storage errors consistently. It also prevents the application layer from duplicating secret lifecycle rules.

**Alternatives considered**:

- Call `SecretStore.WithSecrets` directly from the application. Rejected because it would duplicate validation, clearing, and error-redaction behavior outside the tunnel credential owner.
- Add a general export method to `SecretStore`. Rejected because reusable secret-returning APIs violate the current security boundary.

## Decision: Treat player credentials as one focused editing unit

**Rationale**: The main public-access settings dialog should remain a read-only credential summary with Edit and Share actions. The child dialog pre-fills only the non-secret login, leaves replacement password empty, preserves an existing password on an empty replacement, and deletes the pair by restoring the default login and deleting the password.

**Alternatives considered**:

- Keep login editing inline and move only password editing. Rejected because it splits one Basic Auth credential pair across two interaction contexts.
- Show a disabled password input containing the mask. Rejected because a real form value can be mistaken for the stored password and may be submitted accidentally.

## Decision: Preserve password generation as an immediate secure mutation

**Rationale**: The existing generator saves a new password first and displays it once. It will move into the credential dialog without changing that contract; any drafted login remains available for a subsequent explicit save.

**Alternatives considered**:

- Generate only a local draft and save it later with the login. Rejected because it broadens secret lifetime in JavaScript and changes the established one-time handling contract.

## Decision: Extend only the private desktop allowlist, not protobuf schemas

**Rationale**: Credential sharing is an Overseer-only native side effect and needs no reusable data model. The new desktop method returns the existing `CommandResult`, so player-facing and persistence protobuf contracts remain unchanged. Deterministic Wails bindings and their exact allowlist checks will be regenerated and updated.

**Alternatives considered**:

- Add a private protobuf response containing credentials. Rejected because it creates a serializable secret-bearing contract with no cross-process need.
