# Tasks: Compact Public Access

## Wave 1 — Compact Structure

- [x] **T001** Move the existing public-access configuration form and guide into an accessible settings dialog, leaving a compact control surface in `frontend/overseer/src/index.html`.
- [x] **T002** Add compact-panel and modal layout styles that reuse established dialog behavior and remain usable at narrow desktop widths in `frontend/overseer/src/overseer.css`.

## Wave 2 — Interaction and State

- [x] **T003** Update public-access element bindings and rendering for effective domain display, one lifecycle action, transition states, and accessible copy feedback in `frontend/overseer/src/overseer.js`.
- [x] **T004** Implement settings-dialog open, dismiss, focus restoration, save-success close, and save-failure retention behavior in `frontend/overseer/src/overseer.js`.
- [x] **T005** Route first-use enable attempts with missing saved secrets into the settings dialog without invoking tunnel startup in `frontend/overseer/src/overseer.js`.

## Wave 3 — Verification

- [x] **T006** Extend embedded frontend asset assertions for the compact panel, settings dialog, lifecycle copy, and removal of the inline configuration surface in `internal/platform/assets_test.go`.
- [x] **T007** Run frontend build and focused asset tests, then inspect the resulting diff for unintended changes.
- [x] **T008** Apply Go modernization and repository-wide formatting/validation required before completion because the verification test is Go source.

## Phase 1: Convergence

**Depends on:** all prior phases.

**Wave 1 — Address semantics:**

- [x] **T009** Suppress a configured domain in the compact surface when public access is in an error state, while retaining it for valid stopped and transitional states, per FR-002 and US1/AC4 (partial).

**⟶ Wait for the compact address semantics before aligning browser expectations.**

**Wave 2 — Browser contracts:**

- [x] **T010** Update and run the existing public-access Playwright journeys for modal settings, first-use routing, compact lifecycle actions, effective domain display, save outcomes, and error recovery per FR-005–FR-009 and SC-005 (contradicts).
