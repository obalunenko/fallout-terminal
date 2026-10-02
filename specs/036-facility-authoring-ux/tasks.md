# Tasks: Facility Authoring UX

## Phase 1: Setup

Existing tooling and UI contracts are reused; no installation/configuration source changes are needed.

## Phase 2: Foundational

**Wave 1 — single owner:**

- [x] **T001** Add failing multi-transition preservation and draft-boundary regressions (FR-001–004) · tests/browser/facility-authoring-ux.spec.mjs

**⟶ Wait for Wave 1 to finish, then begin the safe-editing story.**

## Phase 3: US1 — Safe editing and explicit draft publication

**Goal**: Preserve authored relationships and make one save/discard boundary explicit.

**Independent Test**: Edit a multi-transition device, stage and discard, save and reopen; verify references and durable write counts.

**Wave 2 — single owner:**

- [x] **T002** Preserve references/attributes and validate incomplete rows (FR-001–002) · frontend/overseer/src/overseer.js

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — single owner:**

- [x] **T003** Unify staging/save/discard, stale-draft handling, and live-operation guards (FR-003–004, FR-007) · frontend/overseer/src/overseer.js

**Checkpoint**: Safe graph authoring and explicit publication are functional.

## Phase 4: US2 — Adjacent details and readable operation

**Goal**: Find and inspect devices without losing position.

**Independent Test**: Full demo at desktop and narrow sizes; search/select and inspect current/initial state and faults.

**Wave 4 — independent (different files):**

- [x] **T004** [P] Build responsive list/detail markup, readable form structure, persistent draft actions and sticky dialog footers (FR-005, FR-011) · frontend/overseer/src/index.html, frontend/overseer/src/overseer.css
- [x] **T005** [P] Render search, selected details, readable statuses/IDs, scoped relationships, reset/recovery consequences (FR-005–007, FR-013) · frontend/overseer/src/overseer.js

**⟶ Wait for Wave 4 to finish, then integrate and begin relationship editing.**

**Checkpoint**: Device selection and status remain usable with full demo content.

## Phase 5: US3 — Relationship maintenance and command context

**Goal**: Follow and edit each authored relationship in context.

**Independent Test**: Edit/remove bindings and programs; follow block/transition dependencies; edit a multi-device command.

**Wave 5 — single owner:**

- [x] **T006** Add edit/remove for bindings and recovery programs, preserving unrelated values (FR-009) · frontend/overseer/src/overseer.js

**⟶ Wait for Wave 5 to finish, then:**

**Wave 6 — single owner:**

- [x] **T007** Resolve readable dependency targets and expose contextual command facility editing (FR-008, FR-010, FR-013) · frontend/overseer/src/overseer.js

**Checkpoint**: Every supported relationship is reachable and maintainable.

## Phase 6: US4 — Explain transitions and preview consequences

**Goal**: Read authored actions and see detached consequences.

**Independent Test**: Inspect/expand transition rules and compare previews with unavailable/hidden content.

**Wave 7 — single owner:**

- [x] **T008** Add transition summaries, expandable advanced rules, explicit fault effects, and comparative server previews (FR-011–012) · frontend/overseer/src/overseer.js

**Checkpoint**: Preview and transition forms explain their effects without live mutation.

## Phase 7: Polish and validation

**Wave 8 — independent (different files):**

- [x] **T009** [P] Expand UX browser regressions and adapt existing authoring expectations (FR-014) · tests/browser/facility-authoring-ux.spec.mjs, tests/browser/facility-authoring.spec.mjs
- [x] **T010** [P] Update demo authoring instructions for the new workflow · sessions/demo-capability-paths.md

**⟶ Wait for Wave 8 to finish, then:**

**Wave 9 — single owner:**

- [x] **T011** Review simplification, fix integration issues, and run production builds, affected browser journeys and repository validation · frontend/overseer/src/overseer.js, specs/036-facility-authoring-ux/validation.md

**⟶ Wait for Wave 9 to finish, then:**

**Wave 10 — single owner:**

- [x] **T012** Visually inspect demo desktop/narrow and dialogs; record evidence and final requirement coverage · specs/036-facility-authoring-ux/validation.md

## Dependencies & Execution Order

Setup → Foundational → US1 → US2 → US3 → US4 → Polish. Waves 1–3 run sequentially. Wave 4's markup/styles and behavior have a pinned UI contract and independent ownership, joining before Wave 5. Waves 5–7 change the same behavior file and run sequentially. Wave 8's tests and documentation are independent, joining before validation and visual review. Only the main agent materializes task journal entries. Polish owns validation; no project hook claims it.

## Phase 8: Convergence

**Depends on:** all prior phases.

**Wave 11 — stale-draft verification:**

- [x] T013 Add a browser regression that stages a facility draft, emits a newer durable session update, verifies the draft remains visible and cannot overwrite canonical data, then discards into the newer session per FR-004 and US1/AC4 (partial) · tests/browser/facility-authoring-ux.spec.mjs

**⟶ Wait for Wave 11 to finish, then:**

**Wave 12 — complete dependency destinations:**

- [x] T014 Exercise condition and recovery-program dependency destinations plus missing-target feedback, and complete condition navigation to its editable authoring destination if the regression exposes the current summary-only path per FR-008 and SC-004 (partial) · frontend/overseer/src/overseer.js, tests/browser/facility-authoring-ux.spec.mjs

**⟶ Wait for Wave 12 to finish, then:**

**Wave 13 — full-demo layout evidence:**

- [x] T015 Load the actual ten-device demo into the authoring fixture, verify and capture the 1440×900 adjacent layout and 800×700 overflow-free stack, and correct the recorded visual evidence per SC-003 and plan: verification (partial) · tests/browser/facility-authoring-ux.spec.mjs, specs/036-facility-authoring-ux/validation.md
