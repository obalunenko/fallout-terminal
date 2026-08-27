# Tasks: Single App Instance

## Phase 1: Foundational

No separate foundational task is required; the feature reuses the existing Wails host, desktop entrypoint, window contract, and repository validation commands.

## Phase 2: User Story 1 - Prevent duplicate desktop instances (P1)

### Tests

**Wave 1 — single task:**

- [ ] **T001** [US1] Add failing host-option contract coverage for the stable identity, default-on single-instance behavior, zero later-launch exit code, and absence of additional transferred data · `wails_host_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — single task:**

- [ ] **T002** [US1] Configure `application.Options.SingleInstance` with the stable product identity, exit code zero, and an injected activation callback while retaining the existing cross-platform application options · `wails_host.go`

## Phase 3: User Story 2 - Return to the active window (P2)

### Tests

**Wave 1 — single task:**

- [ ] **T003** [US2] Add failing concurrency-safe activation tests for a ready window, a launch before window binding, repeated launches, restore-before-focus ordering, and ignored untrusted payload data · `wails_host_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — single task:**

- [ ] **T004** [US2] Implement the narrow mutex-protected Overseer window activation coordinator with one pending activation and window calls outside the lock · `wails_host.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — single task:**

- [ ] **T005** [US2] Create the activation coordinator only on the interactive application path, inject its callback into host construction, and bind the created Overseer window before running the host · `main.go`

## Phase 4: Polish

**Wave 1 — single task:**

- [ ] **T006** Format the changed Go files and apply the repository Go quality review to the single-instance implementation and tests · `main.go`, `wails_host.go`, `wails_host_test.go`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — single task:**

- [ ] **T007** Validate the implementation with `task vet`, `task test`, and `task test:race`, and report any optional native rapid-launch/window-focus evidence honestly as PASS or NOT RUN · `Taskfile.yml`, `specs/027-single-app-instance/spec.md`

**Dependency**: T001 → T002 → T003 → T004 → T005 → T006 → T007; every task is sequential because later work depends on the preceding contract or touches the same Go files.
