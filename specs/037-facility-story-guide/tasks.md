# Tasks: Facility Story Guide

## Phase 1: Guided authoring

**Wave 1 — narrative surface:**

- [x] **T001** Add the ordered guide, narrative grouping labels, contextual form instructions, and responsive styling per FR-001–002, FR-005–006, FR-008 · frontend/overseer/src/index.html, frontend/overseer/src/overseer.css

**⟶ Wait for Wave 1, then:**

**Wave 2 — contextual actions:**

- [x] **T002** Derive guide context from the current draft, handle missing prerequisites, connect existing editors/save/preview, allow choosing a recovery program in command actions, and group relationship rendering per FR-003–007 · frontend/overseer/src/overseer.js

**⟶ Wait for Wave 2, then:**

**Wave 3 — user journeys:**

- [x] **T003** Verify empty and populated narrative setup, selection/discard state, keyboard access, explicit save, and responsive screenshots per FR-001–008 and SC-001–004 · tests/browser/facility-authoring-ux.spec.mjs

**⟶ Wait for Wave 3, then:**

**Wave 4 — handoff:**

- [x] **T004** Document the in-app story route, inspect desktop/narrow layouts, review integration, run focused browser and frontend build checks, and record validation per SC-002–004 · sessions/demo-capability-paths.md, specs/037-facility-story-guide/validation.md

## Phase 2: Convergence

**Depends on:** all prior phases.

**Wave 1 — safe prerequisite creation:**

- [x] T005 Fix the guide's missing-command and missing-display-destination routes so creating terminal content does not strand an existing facility draft per FR-003–004, FR-007, US1/AC3–4, SC-003 (partial; F1; HIGH). In `frontend/overseer/src/overseer.js`, account for terminal `addNode`/`autosave` advancing the canonical session revision while `facilityNodeCatalog` still reads the retained dirty draft. Explain and offer an explicitly labeled save checkpoint before terminal editing when necessary; proceed only after successful save, preserve draft/selection on failure, and retain conflict protection without implicit publication or silent discard. On return, expose the newly created command/entry to the selected device's guide. Extend `tests/browser/facility-authoring-ux.spec.mjs` beyond tab navigation: start with staged device/transition changes and no suitable destination, create the missing command or entry through the UI, return, bind it, and save without losing edits; cover a rejected checkpoint. Update `sessions/demo-capability-paths.md` and `specs/037-facility-story-guide/validation.md` with the actual save boundary and Taskfile validation results.

**⟶ Wait for Wave 1, then:**

**Wave 2 — optional-step prerequisite guidance:**

- [x] T006 Explain recovery-program prerequisites when the selected device has no transitions per FR-003, US1/AC3 (partial; F2; MEDIUM). In the optional-complications branch of `renderFacilityStoryGuide` in `frontend/overseer/src/overseer.js`, replace silent omission of program creation with contextual Russian guidance and a clearly labeled action opening the selected device's transition editor; keep faults/programs optional rather than reporting a readiness error. Extend `tests/browser/facility-authoring-ux.spec.mjs` to jump directly to step 4 with a transitionless device, follow the prerequisite action, apply a transition, and return to available program creation without durable writes. Record focused Taskfile validation in `specs/037-facility-story-guide/validation.md`.
