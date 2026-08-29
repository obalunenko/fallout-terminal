# Tasks: Manual Public Access Control

## Phase 1: Simplified Settings Dialog

**Wave 1 — independent presentation surfaces:**

- [x] **T001** [P] [US2] Remove the behavior fieldset and startup checkbox from the settings dialog · `frontend/overseer/src/index.html`
- [x] **T002** [P] [US2] Remove styling used only by the behavior preference control · `frontend/overseer/src/overseer.css`

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T003** [US1] Remove startup-preference UI state and submit the compatibility value as disabled while preserving explicit lifecycle controls · `frontend/overseer/src/overseer.js`

## Phase 2: Verification

**Wave 1 — independent contracts:**

- [x] **T004** [P] [US2] Update embedded asset assertions for the two-group dialog and absence of automatic-start UI · `internal/platform/assets_test.go`
- [x] **T005** [P] [US1] Update browser journeys for legacy preference loading, settings saves, and explicit enable/stop actions · `tests/browser/public-access-settings.spec.mjs`

**⟶ Wait for the verification contracts to finish, then:**

- [x] **T006** Validate the frontend build, focused asset test, Go quality gates, and affected browser journeys against SC-001–SC-004 · `Taskfile.yml`

**Dependency note:** T001 and T002 unblock T003; the completed UI behavior unblocks T004 and T005; all implementation and test tasks unblock T006.
