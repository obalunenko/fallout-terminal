# Tasks: Node.js 26 Upgrade

**Bugfix**: 2026-08-27 — BUG-001 adds local runtime selection, preflight, and active contributor guidance.

**Input**: [spec.md](./spec.md), [plan.md](./plan.md), [research.md](./research.md), and [data-model.md](./data-model.md)

**Tests**: Required by the specification and Constitution 8.1.0. Extend the existing static workflow acceptance owner first, then validate the exact NVM-managed runtime through clean installs, builds, browser tests, repository quality, and normal GitHub workflow evidence.

## Phase 1: Setup

**Purpose**: Turn the approved runtime and action pins into a failing repository-owned contract before changing configuration.

- [x] **T001** Add failing static assertions for Node.js 26.8.1, the exact official action releases, aligned manifests/lock records/README, and absence of active 20.19.0 references · `internal/platform/portable_release_test.go`

## Phase 2: Foundational

**Purpose**: Establish the single project-owned Node.js policy that both local and automated stories consume.

**Wave 1:**

- [x] **T002** Set every project-owned package engine to `>=26.8.1` and update the active maintainer prerequisite without changing dependencies or historical evidence · `frontend/package.json`, `frontend/client/package.json`, `frontend/overseer/package.json`, `tests/browser/package.json`, `README.md`

**⟶ Wait for T002 to finish, then:**

- [x] **T003** Regenerate only project-owned engine metadata with NVM Node.js 26.8.1/npm 11.19.0 and prove dependency pins, resolved URLs, integrity values, and third-party engines did not drift · `frontend/package-lock.json`, `tests/browser/package-lock.json`

## Phase 3: User Story 1 — Use One Current Node.js Runtime (Priority: P1) 🎯 MVP

**Goal**: Prove clean local installation, both production builds, and browser journeys on the exact approved runtime.

**Independent Test**: Activate NVM Node.js 26.8.1, perform clean locked installs, build both frontends, and run the complete browser suite with no engine or runtime compatibility failure.

### Implementation

- [x] **T004** [US1] Run the clean frontend install, Overseer build, player-client build, browser-test install, and complete browser suite under NVM Node.js 26.8.1/npm 11.19.0 · `frontend/package-lock.json`, `tests/browser/package-lock.json`

**Checkpoint**: User Story 1 is independently functional—every local JavaScript workflow uses and passes on Node.js 26.8.1.

## Phase 4: User Story 2 — Run Quality and Releases on Node.js 26 (Priority: P1)

**Goal**: Align quality and all five portable targets on Node.js 26.8.1 while removing deprecated internal Node.js 20 action runtimes.

**Independent Test**: Inspect and execute the workflow contracts, then confirm each JavaScript-dependent job selects Node.js 26.8.1, uses the approved exact node24-backed action releases, and preserves existing triggers, permissions, caches, matrix, and artifact inventory.

### Implementation

**Wave 1 — independent (different files):**

- [x] **T005** [P] [US2] Pin the quality workflow to Node.js 26.8.1 plus exact checkout/setup-go/setup-node releases without changing its read-only trigger, cache, or Task entrypoint · `.github/workflows/wails-cross-platform.yml`
- [x] **T006** [P] [US2] Pin the portable workflow to Node.js 26.8.1 plus exact checkout/setup-go/setup-node/upload/download releases without changing its five-target or create-only publication contracts · `.github/workflows/wails-portable.yml`

**⟶ Wait for Wave 1 to finish, then:**

- [x] **T007** [US2] Make the focused static quality/release contracts pass and verify the exact action pins occur in every governed workflow location · `internal/platform/portable_release_test.go`, `.github/workflows/wails-cross-platform.yml`, `.github/workflows/wails-portable.yml`

**Checkpoint**: User Story 2 is independently functional—quality and release automation are aligned on Node.js 26.8.1 and no upgraded official action uses internal Node.js 20.

## Phase 5: Polish

**Purpose**: Validate the complete cutover and collect evidence against every Success Criterion.

- [x] **T008** Run diff/legacy-reference checks, focused platform tests, the full pinned repository quality composition, and verify the next GitHub quality and governed five-target release runs as available without creating a bypass path · `README.md`, `internal/platform/portable_release_test.go`, `.github/workflows/wails-cross-platform.yml`, `.github/workflows/wails-portable.yml`

## Phase 6: BUG-001 Local Runtime Guard

**Wave 1:**

- [x] **T009** Add failing runtime-policy assertions for the NVM selector, contributor requirement, and Task preflight ordering (BUG-001, FR-010, SC-009) · `internal/platform/portable_release_test.go`

**⟶ Wait for T009, then:**

**Wave 2:**

- [x] **T010** Add the Node.js 26.8.1 NVM selector and Task preflight, route npm workflows through it, and correct active contributor and agent setup (BUG-001, FR-010, SC-009) · `.nvmrc`, `Taskfile.yml`, `CONTRIBUTING.md`, `AGENTS.md`

## Dependencies & Execution Order

- Phase 1 must fail on the approved new contract before configuration changes.
- Phase 2 updates owning manifests before regenerating either lockfile.
- User Story 1 depends on the synchronized manifests and lockfiles, then proves local compatibility.
- User Story 2 depends on the shared runtime policy; T005 and T006 are independent, and both must finish before T007.
- Polish depends on both user-story checkpoints and owns the single complete validation pass.
