# Tasks: Public Access Credential Editing

## Phase 1: Structured Settings Dialog

**Wave 1 — independent presentation surfaces:**

- [x] **T001** [P] [US1] Add the configured-token summary, token-management dialog, and three ordered settings groups · `frontend/overseer/src/index.html`
- [x] **T002** [P] [US2] Style grouped settings, compact credential rows, adjacent password actions, and the token-management modal · `frontend/overseer/src/overseer.css`

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T003** [US1] Render configured versus missing token states and implement replace, delete, dismiss, focus, confirmation, and cleanup behavior · `frontend/overseer/src/overseer.js`

## Phase 2: Verification

**Wave 1 — independent contracts:**

- [x] **T004** [P] [US1] Extend embedded asset assertions for the token summary, nested dialog, secret non-disclosure, and grouped field order · `internal/platform/assets_test.go`
- [x] **T005** [P] [US1] Update Playwright journeys for configured, absent, replacement, deletion, dismissal, failure, active-access, and unknown-presence states · `tests/browser/public-access-settings.spec.mjs`

**⟶ Wait for the verification contracts to finish, then:**

- [x] **T006** Validate frontend build, focused asset tests, repository Go gates, and affected browser journeys against SC-001–SC-005 · `Taskfile.yml`

**Dependency note:** T001 and T002 unblock T003; the completed UI behavior unblocks T004 and T005; all implementation and test tasks unblock T006.
