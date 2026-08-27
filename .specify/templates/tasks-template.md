---
description: "Task list template for Fallout Terminal feature implementation"
---

# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`

**Prerequisites**: `plan.md` and `spec.md`; include `research.md`, `data-model.md`, `contracts/`, and `quickstart.md` when the plan requires them

**Testing**: Colocated Go tests and Playwright browser journeys are configured. Include focused automated tests for changed behavior, repository lint with `task lint`, race testing for affected concurrent services, and concrete `task dev` journeys for native/browser interaction. Packaging and credential-dependent checks apply only when their surfaces change and the required environment is available.

**Organization**: Group tasks by prioritized user story so each story remains independently implementable and verifiable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Safe to execute in parallel because files do not overlap and prerequisites are complete
- **[Story]**: User-story traceability label such as `[US1]`
- Every task MUST name exact repository paths
- Contract changes MUST identify both producer and consumer tasks
- Verification tasks MUST name the command or observable manual journey

## Repository Paths

- Composition, lifecycle, and Wails bridge: `main.go`, `app.go`
- Models and pure rules: `internal/domain/`, `internal/nav/`, `internal/hack/`
- Canonical live and coordination state: `internal/live/`, `internal/control/`
- Persistent JSON: `internal/session/`, `internal/playerconfig/`, `sessions/*.json`
- Player HTTP asset/ConnectRPC boundary: `internal/player/`
- Platform and optional public access: `internal/platform/`, `internal/tunnel/`
- Application update and release identity: `internal/update/`, `internal/version/`, `wails_updater*.go`
- Generated contracts: `proto/`, `internal/gen/`, `frontend/client/gen/`
- Shared Go test support: `internal/testutil/`
- Overseer interface: `frontend/overseer/src/index.html`, `frontend/overseer/src/desktop-api.js`, `frontend/overseer/src/overseer.js`, `frontend/overseer/src/overseer.css`
- Player interface: `frontend/client/index.html`, `frontend/client/client.js`, `frontend/client/sound.js`, `frontend/client/client.css`
- Browser journeys: `tests/browser/*.spec.mjs`, `tests/browser/fixture-server/main.go`
- Build configuration: `go.mod`, `go.sum`, `frontend/package.json`, `frontend/package-lock.json`, `frontend/client/package.json`, `frontend/overseer/package.json`
- Cross-platform packaging/unsigned release: `build/`, `internal/buildtool/`, `.goreleaser.yaml`, `.github/workflows/`
- Optional signed macOS distribution: `scripts/build-macos.sh`

<!--
The task generator MUST replace all examples below with feature-specific tasks.
Do not add database, generic authentication, API-framework, or imaginary src/
setup tasks unless the approved feature actually introduces those changes.
-->

## Phase 1: Setup and Contract Baseline

**Purpose**: Confirm affected boundaries, compatibility requirements, and verification before implementation.

- [ ] T001 Review the feature's affected paths and current behavior in [exact paths]
- [ ] T002 Record changed persistent JSON, Wails, protobuf/ConnectRPC, or HTTP asset contracts in `specs/[###-feature]/contracts/[contract].md`
- [ ] T003 Define automated commands and interactive journeys in `specs/[###-feature]/quickstart.md`
- [ ] T004 [P] Update `go.mod`/`go.sum` or the relevant npm manifest/lockfile only if required by the approved plan

**Checkpoint**: Producers, consumers, compatibility behavior, and verification are explicit.

---

## Phase 2: Foundational Domain, Persistence, and Transport Work

**Purpose**: Implement shared behavior that blocks all user stories.

- [ ] T005 Implement model, validation, navigation, or hacking behavior in `internal/domain/`, `internal/nav/`, or `internal/hack/`
- [ ] T006 Implement canonical state and ordered coordination in `internal/live/` or `internal/control/`
- [ ] T007 Implement compatible persistence in `internal/session/` or `internal/playerconfig/` if required
- [ ] T008 Implement static HTTP/ConnectRPC validation, protocol, and publication in `internal/player/` if required
- [ ] T009 Wire lifecycle, dependencies, or the smallest required Wails API/event in `main.go` and `app.go`
- [ ] T010 Add focused colocated Go tests and deterministic fixtures in [exact `*_test.go` or `internal/testutil/` path]

**Checkpoint**: Shared rules, state ownership, persistence, and cross-boundary contracts are ready for user-story integration.

---

## Phase 3: User Story 1 - [Title] (Priority: P1) 🎯 MVP

**Goal**: [Observable value delivered]

**Independent Test**: [Concrete standalone verification journey]

### Verification for User Story 1

- [ ] T011 [P] [US1] Add focused Go tests in [exact package/test path]
- [ ] T012 [P] [US1] Add or update a Playwright journey in `tests/browser/[exact].spec.mjs` when player behavior changes
- [ ] T013 [US1] Document the `task dev` Overseer/player journey in `specs/[###-feature]/quickstart.md`

### Implementation for User Story 1

- [ ] T014 [P] [US1] Implement Overseer changes in `frontend/overseer/src/[exact file]`
- [ ] T015 [P] [US1] Implement player changes in `frontend/client/[exact file]`
- [ ] T016 [US1] Integrate Wails/ConnectRPC producers and consumers in [exact Go and JavaScript paths]
- [ ] T017 [US1] Update JSON defaults, validation, versioning, references, or `sessions/demo.json` if the persistent contract changes
- [ ] T018 [US1] Verify the independent journey with one Overseer and [client count] player browsers

**Checkpoint**: User Story 1 works independently and all connected clients converge on authoritative state.

---

## Phase 4: User Story 2 - [Title] (Priority: P2)

**Goal**: [Observable value delivered]

**Independent Test**: [Concrete standalone verification journey]

### Verification for User Story 2

- [ ] T019 [P] [US2] Add focused automated verification in [exact Go or Playwright path]
- [ ] T020 [US2] Add the interactive journey to `specs/[###-feature]/quickstart.md`

### Implementation for User Story 2

- [ ] T021 [P] [US2] Implement domain/runtime changes in `internal/[exact package]/[exact file]`
- [ ] T022 [P] [US2] Implement presentation changes in `frontend/overseer/src/[exact file]` or `frontend/client/[exact file]`
- [ ] T023 [US2] Integrate and validate changed contracts in [producer path] and [consumer path]
- [ ] T024 [US2] Verify initial connection, multiple tabs/clients, controller authority, and reconnection as applicable

**Checkpoint**: User Stories 1 and 2 remain independently functional.

---

## Phase 5: User Story 3 - [Title] (Priority: P3)

**Goal**: [Observable value delivered]

**Independent Test**: [Concrete standalone verification journey]

### Implementation and Verification for User Story 3

- [ ] T025 [P] [US3] Implement isolated changes in [exact path]
- [ ] T026 [US3] Integrate cross-boundary behavior in [exact paths]
- [ ] T027 [US3] Add automated verification in [exact Go or Playwright path]
- [ ] T028 [US3] Verify the documented independent journey

**Checkpoint**: All selected user stories are independently functional.

---

## Final Phase: Cross-Cutting Verification and Polish

- [ ] T029 [P] Review Wails method exposure, CSP, external URL handling, and privileged input validation in `app.go` and `frontend/overseer/src/`
- [ ] T030 [P] Review ConnectRPC origin/input validation, public projections, revisions, and reconnect synchronization in `internal/player/` and `frontend/client/`
- [ ] T031 [P] Open and save existing compatible files from `sessions/` without data loss when persistence changes
- [ ] T032 Run `task fmt:check`, `task vet`, `task lint`, and `task test`
- [ ] T033 Run `task test:race` when concurrent runtime behavior changes
- [ ] T034 Run `npm ci --prefix frontend` and `npm run build --prefix frontend` when the Overseer UI, bridge, embedding, or package changes
- [ ] T035 Run `npm ci --prefix tests/browser` and `npm test --prefix tests/browser` when affected browser journeys are available
- [ ] T036 Run `task dev` and complete the documented Overseer/player smoke journeys
- [ ] T037 Run `task package` and optional `task package:all` for packaging/release-sensitive changes on supported hosts
- [ ] T038 Run approved credential-gated public-provider or optional `task release:macos:signed` gates when affected and prerequisites are available; otherwise record them as unavailable
- [ ] T039 Update `README.md`, contracts, fixtures, and CI configuration when setup, operation, protocol, or user-visible workflows changed

---

## Dependencies and Execution Order

- Contract/setup tasks precede changes to contract producers and consumers.
- Foundational domain, persistence, control, player-server, or composition work blocks dependent UI stories.
- Within a story, pure rules precede canonical-state and transport integration; producer and consumer changes precede end-to-end verification.
- Persistent JSON migration/default logic precedes validation with older user files.
- User stories may proceed in parallel only after shared foundations are stable and their exact files do not overlap.
- Cross-cutting verification follows all selected user stories.

## Parallel Opportunities

- Independent `frontend/overseer/src/` and `frontend/client/` presentation work may run in parallel after their shared contract is fixed.
- Pure Go tests may run in parallel with isolated CSS/HTML work when their files and prerequisites do not overlap.
- Security, documentation, fixtures, and packaging review may run in parallel when they touch different files.
- Tasks changing `main.go`, `app.go`, `internal/control/`, `internal/player/`, shared frontend state, or the same contract are not parallel merely because they have different story labels.

## Implementation Strategy

1. Deliver the smallest P1 vertical slice across every required Go and browser boundary.
2. Verify it with focused automated checks and the documented Overseer/player journey.
3. Add P2 and P3 stories incrementally without breaking earlier journeys.
4. Finish with race, multi-client, reconnection, persistent-data, security, shutdown, and packaging checks proportional to the change.

## Notes

- Do not claim formatting, vet, test, race, browser, CI, packaging, or release success unless the relevant command or journey actually ran.
- Record unavailable target-platform build environments, credentials, browsers, or manual checks explicitly.
- Keep runtime-only live and coordination state out of persistent JSON unless persistence is an approved requirement.
- Avoid vague tasks, generic database/authentication work, and producer-only contract changes.
