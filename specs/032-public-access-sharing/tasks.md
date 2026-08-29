# Tasks: Public Access Credential Sharing

## Phase 1: Foundational Native Boundary

### Tests

**Wave 1:**

- [x] **T001** [US3] Add failing coverage for player-password-only scoped access, validation, clearing, and error redaction · `internal/tunnel/secret_test.go`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2:**

- [x] **T002** [US3] Implement the scoped player-password callback helper without adding a reusable secret getter · `internal/tunnel/secret.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [x] **T003** [US3] Add failing native tests for credential formatting, clipboard success, storage failure, clipboard failure, and secret-free results/logs · `app_test.go`

**⟶ Wait for Wave 3 to finish, then:**

### Implementation

**Wave 4:**

- [x] **T004** [US3] Add the injected clipboard seam and native `CopyPublicAccessCredentials` command that returns only `CommandResult` · `app.go`

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — independent composition surfaces:**

- [x] **T005** [P] [US3] Wire the Wails clipboard manager into application composition · `main.go`
- [x] **T006** [P] [US3] Expose the copy command through the private desktop allowlist and update its exact Go contract · `desktop_service.go`, `app_contract_test.go`

## Phase 2: User Story 1 — Safe Saved-Credential Summary

**Goal**: Show the saved login and a fixed five-character password mask without inline credential inputs.

**Independent Test**: Open configured and unconfigured settings snapshots and inspect the summary without opening the editor.

### Implementation

**Wave 1 — independent presentation surfaces:**

- [x] **T007** [P] [US1] Replace inline player credential controls with a read-only login/password summary and Edit/Share actions · `frontend/overseer/src/index.html`
- [x] **T008** [P] [US1] Style the credential summary, fixed mask, and responsive action layout · `frontend/overseer/src/overseer.css`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2:**

- [x] **T009** [US1] Render saved login, exact `*****` mask, presence states, and share availability from authoritative snapshots · `frontend/overseer/src/overseer.js`

**Checkpoint**: User Story 1 is independently functional and testable.

## Phase 3: User Story 2 — Focused Credential Editing

**Goal**: Replace, preserve, delete, or cancel player credentials in a child dialog patterned after token management.

**Independent Test**: Exercise the child dialog from present and absent password states, including focus, cleanup, validation, save, and deletion.

### Implementation

**Wave 1:**

- [x] **T010** [US2] Add the player-credential child dialog and implement its save, generate, delete, cancel, focus, and transient-secret lifecycle · `frontend/overseer/src/index.html`, `frontend/overseer/src/overseer.css`, `frontend/overseer/src/overseer.js`

**Checkpoint**: User Story 2 is independently functional and testable.

## Phase 4: User Story 3 — One-Click Credential Sharing

**Goal**: Invoke the native no-secret command from the Share action and report its outcome.

**Independent Test**: Share configured credentials and verify the clipboard result plus browser-visible absence of the password; exercise safe failure states.

### Implementation

**Wave 1:**

- [x] **T011** [US3] Regenerate the 39-method Wails binding, update the frontend facade and browser fixture, and connect the Share action to the native command · `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js`, `frontend/overseer/src/desktop-api.js`, `frontend/overseer/src/overseer.js`, `tests/browser/fixtures/desktop-bindings.js`, `scripts/wails-bindings-check.sh`

**Checkpoint**: User Story 3 is independently functional and testable.

## Phase 5: Polish and Verification

**Wave 1 — independent verification contracts:**

- [x] **T012** [P] [US1] [US2] Update embedded asset assertions for the fixed mask, read-only summary, and credential child dialog · `internal/platform/assets_test.go`
- [x] **T013** [P] [US1] [US2] [US3] Update browser journeys for masking, editing, preservation, deletion, sharing, and failures · `tests/browser/public-access-settings.spec.mjs`, `tests/browser/desktop-api.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2:**

- [x] **T014** Validate against SC-001–SC-005 with `go fix`, formatting, focused tests, deterministic bindings, secret-leak checks, frontend build, browser tests, and repository Go quality gates · `Taskfile.yml`

## Dependencies & Execution Order

- Foundational order: T001 → T002 → T003 → T004 → {T005, T006}.
- Story order: the native boundary unblocks {T007, T008} → T009 → T010 → T011.
- Verification order: completed behavior unblocks {T012, T013} → T014.
