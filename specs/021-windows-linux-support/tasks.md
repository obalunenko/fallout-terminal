# Tasks: Windows and Linux Desktop Support

This oversized feature is ordered as explicit execution waves. Tests are authored before their corresponding implementation where the constitution requires them, but no test or build command is run locally for this feature; validation executes in CI or on matching native hosts.

**Bugfix**: 2026-08-26 — BUG-001 Updated from bugfix patch.

**Bugfix**: 2026-08-26 — BUG-002 Updated from bugfix patch.

**Bugfix**: 2026-08-26 — BUG-003 Updated from bugfix patch.

**Bugfix**: 2026-08-26 — BUG-004 Updated from bugfix patch.

## Phase 1: Setup — Governance and Task Tooling

This phase removes the policy and bootstrap blockers shared by every user story.

**Wave 1 — governance prerequisite:**

- [x] **T001** Amend the deployment profile and Go-tool orchestration rules to authorize `windows`/`linux` on `arm64`/`amd64`, the pinned root Taskfile, Make-only tool bootstrap, and retained Go build-policy ownership · `.specify/memory/constitution.md`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent tooling files:**

- [x] **T002** [P] Add the isolated Go Task module pin for `github.com/go-task/task/v3/cmd/task` v3.53.1 with its committed dependency checksums · `tools/task/go.mod`, `tools/task/go.sum`
- [x] **T003** [P] Replace the Make workflow graph with the sole default `tools` bootstrap that discovers each `tools/*/go.mod` and runs `go install tool` in deterministic order, plus non-mutating bootstrap help · `Makefile`
- [x] **T004** [P] Create the schema-v3 Wails-compatible Task graph and migrate every existing Make workflow, variable, dependency, and failure contract without Wails→Task recursion · `Taskfile.yml`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent cutover guards:**

- [x] **T005** [P] Replace active Taskfile-prohibition and direct-Go-entrypoint checks with root-Taskfile presence, recursion-safety, and single-command-graph assertions · `scripts/wails-v3-contract-check.sh`, `scripts/wails-v3-cutover-check.sh`
- [x] **T006** [P] Extend isolated-tool validation to discover `tools/task`, verify v3.53.1 and every module’s one-tool contract, and prove tool resolution leaves the root module unchanged · `scripts/tool-modules-check.sh`
- [x] **T007** [P] Update startup/build ownership tests for `make tools`, non-mutating `make help`, the migrated Task surface, Wails dispatch, and the absence of Make-owned application workflows · `internal/platform/startup_test.go`

## Phase 2: Foundational — Target-Aware Build Graph

This phase supplies the typed target, host, CLI, and portable preflight infrastructure that blocks all four stories.

### Tests

**Wave 1 — independent failing contract tests:**

- [x] **T008** [P] Add table-driven tests for exact `windows`/`linux` and `arm64`/`amd64` parsing, host mismatch, aliases, case changes, and the existing macOS default · `internal/buildtool/target_test.go`
- [x] **T009** [P] Refactor build-plan tests to require portable tool invocation, locked preparation order, target environment isolation, and unchanged macOS resource/signature ordering · `internal/buildtool/buildtool_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — canonical target model:**

- [x] **T010** Implement immutable Platform Target and Build Host values, exact validation, executable/archive properties, and actionable mismatch errors · `internal/buildtool/target.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent target consumers:**

- [x] **T011** [P] Extend the owned build CLI to accept validated GOOS/GOARCH target inputs and internal aggregate actions while preserving existing action compatibility and nonzero usage failures · `cmd/build/main.go`
- [x] **T012** [P] Refactor the shared build sequence into platform-neutral preparation and native preflight actions that preserve frontend→protobuf→client→bindings→Overseer→resources ordering · `internal/buildtool/buildtool.go`, `internal/buildtool/preflight.go`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — shared package plan:**

- [x] **T013** Add the immutable Package Plan, isolated target staging roots, explicit production environment, owned cleanup allowlist, and failure-without-success-output semantics · `internal/buildtool/package.go`

## Phase 3: User Story 1 — Run the Desktop Application on Every Target (P1)

**Goal**: Produce a runnable native application for all four exact Windows/Linux targets with packaged resources, identity, icon, and bundled demo.

**Independent Test**: On each clean matching host, run the target package task, extract the archive, launch from an unrelated working directory, observe the Overseer window, and load the bundled demo without repository or developer tooling.

### Tests

**Wave 1 — independent failing story tests:**

- [x] **T014** [P] Add target package-plan tests for executable names, GUI/CGO flags, isolated staging, complete resource inventory, metadata generation, and collision-free outputs · `internal/buildtool/package_test.go`
- [x] **T015** [P] Extend production resource tests for compile-time package identity, macOS bundle roots, executable-relative Windows/Linux roots, unrelated working directories, and missing resources · `production_resources_test.go`
- [x] **T016** [P] Add Wails host tests for Windows/Linux application options, stable program identity, icon presence, platform fallback registration, and exact-once close behavior · `wails_host_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent runtime/package inputs:**

- [x] **T017** [P] Replace bundle-path heuristics with immutable development/production profiles and platform-specific packaged resource roots · `build_profile_development.go`, `build_profile_production.go`, `resource_roots.go`
- [x] **T018** [P] Add reviewed Windows compatibility, product/version, and icon-generation inputs without committing architecture-specific generated objects · `build/windows/app.manifest`, `build/windows/info.json`
- [x] **T019** [P] Split Darwin-only close fallback from portable Wails host composition and supply Windows/Linux product name and icon options · `wails_host.go`, `wails_host_darwin.go`, `wails_host_other.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — target package assembly:**

- [x] **T020** Implement Windows GUI and Linux CGO compilation, build-scoped `.syso` generation, executable-relative resource staging, both bundled demos, icon, and notices for each exact target · `internal/buildtool/package.go`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — canonical maintainer command:**

- [x] **T021** Wire `task package GOOS=<os> GOARCH=<arch>` and Wails-dispatched `package` to the same Go plan while retaining no-variable macOS arm64 behavior · `Taskfile.yml`

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — matching-host launch harnesses:**

- [x] **T022** Add Windows and Linux native smoke harnesses that extract under a spaced path, launch from another directory, observe the Overseer window within 60 seconds, load the bundled demo, and close cleanly · `scripts/verify-windows-package.ps1`, `scripts/verify-linux-package.sh`

**Checkpoint**: Each of the four target-specific commands now produces an independently launchable application archive on its matching host.

## Phase 4: User Story 2 — Host the Same Game Workflow Across Platforms (P1)

**Goal**: Preserve session, player, native desktop, public-access credential, and shutdown behavior on Windows and Linux with OS-appropriate storage and fail-closed security.

**Independent Test**: On one matching Windows and Linux host, open the demo, save/reopen JSON through native dialogs, connect a player, exercise synchronized controls and secure credentials, open an allowed external URL, and close with no listener, tunnel, or process left behind.

### Tests

**Wave 1 — independent failing story tests:**

- [x] **T023** [P] Expand path tests for Windows Known Folders, Linux XDG roots/fallbacks, preserved macOS paths, redirection, read-only roots, spaces, Unicode, and resource/data separation · `internal/platform/paths_test.go`
- [x] **T024** [P] Expand desktop adapter tests for native Windows/Linux JSON open/save filters, initial directories, cancel outcomes, and HTTP/HTTPS-only external links · `internal/platform/desktop_test.go`
- [x] **T025** [P] Expand the shared secure-store contract tests for Windows/Linux presence, replace, delete, scoped use, byte clearing, and not-found/locked/denied/unavailable mapping · `internal/platform/keychain_test.go`
- [x] **T026** [P] Add public-access manager tests that preserve secure-store initialization failures, recover on retry, keep local/LAN access available, and never downgrade failure to “credential missing” · `internal/tunnel/manager_test.go`
- [x] **T027** [P] Add application lifecycle tests for startup failure and normal close with connected players/public access, exact-once shutdown, and bounded listener/tunnel/process cleanup · `app_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent platform foundations:**

- [x] **T028** [P] Introduce an injectable native-directory provider and implement preserved Darwin, Windows Known Folder/application-data, and Linux XDG document/config storage profiles · `internal/platform/paths.go`, `internal/platform/paths_darwin.go`, `internal/platform/paths_windows.go`, `internal/platform/paths_linux.go`
- [x] **T029** [P] Make native dialog filters and external-link handling portable while retaining privileged-boundary validation and lifetime guards · `internal/platform/desktop.go`
- [x] **T030** [P] Generalize secure-store errors/accounts, narrow the unsupported build tags, and add direct pinned runtime dependencies for Windows Credential Manager and Linux D-Bus · `internal/platform/keychain.go`, `internal/platform/keychain_other.go`, `go.mod`, `go.sum`
- [x] **T031** [P] Preserve precise secure-store availability across initialization/start/retry and use platform-neutral redacted status wording · `internal/tunnel/manager.go`, `internal/tunnel/model.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent native credential providers:**

- [x] **T032** [P] Implement Windows Credential Manager presence/replace/delete/scoped-use semantics with stable service/account names, native error mapping, and cleared credential blobs · `internal/platform/keychain_windows.go`
- [x] **T033** [P] Implement context-bounded freedesktop Secret Service collection/unlock/prompt/GetSecret/replace/delete behavior with precise fail-closed error mapping and cleared buffers · `internal/platform/keychain_linux.go`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — application composition join:**

- [x] **T034** Wire production profile, packaged resources, storage profiles, native secure stores, and exact-once lifecycle cleanup through the existing root application composition without changing player/session contracts · `main.go`, `app.go`

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — secret and UX cutover:**

- [x] **T035** Extend leak enforcement and replace macOS-specific “Keychain” user wording with redacted “secure credential store” status while retaining zero persistence/log/public-state exposure · `scripts/secret-leak-check.sh`, `frontend/overseer/src/overseer.js`

**Checkpoint**: A representative host-and-player journey is independently functional on Windows and Linux, including native dialogs, protected credentials, local/LAN continuity, and clean shutdown.

## Phase 5: User Story 3 — Produce Trustworthy Artifacts for All Targets (P2)

**Goal**: Produce deterministic, correctly identified artifacts and expose complete remote-native and local-Docker aggregate commands that withhold every failed or unverifiable output.

**Independent Test**: From one clean current branch fully pushed to `origin`, run `task package:all:remote`, verify exactly four unique archives and sidecars, inspect each manifest and PE/ELF identity, and confirm the aggregate fails with no success download when any target check fails. Separately validate the local aggregate contract for the native Darwin bundle and four exact Windows/Linux `bin/<os>-<arch>/` executable/resource payloads, byte identity with their archives, atomic failure, and actionable host/Docker diagnostics without requiring a local five-target build or claiming native Windows/Linux launch evidence.

### Tests

**Wave 1 — independent failing story tests:**

- [x] **T036** [P] Add deterministic ZIP/TAR.GZ tests for sorted safe paths, normalized timestamps/modes, exact inventory, duplicate/traversal/symlink rejection, and stable per-file manifests · `internal/buildtool/archive_test.go`
- [x] **T037** [P] Add PE/ELF, product metadata, target/name, checksum, executable-mode, required-resource, and corrupted/mismatched artifact verification tests · `internal/buildtool/verify_test.go`
- [x] **T038** [P] Add aggregate-run tests for correlation IDs, exact four-target/source-SHA joins, independent failure reporting, cancellation, partial download quarantine, and atomic success exposure · `internal/buildtool/aggregate_test.go`
- [x] **T039** [P] Add repository contract tests for explicit native runner labels, fail-fast disabled, upload-after-verification, aggregate gating, pinned Task use, and separate macOS trust workflow · `internal/platform/portable_release_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent artifact services:**

- [x] **T040** [P] Implement deterministic ZIP/TAR.GZ writers, safe normalized inventory, schema-v1 file manifests, and archive SHA-256 sidecars · `internal/buildtool/archive.go`
- [x] **T041** [P] Implement archive extraction inspection, PE/ELF machine validation, product/resource/mode/manifest checks, and actionable verification failures · `internal/buildtool/verify.go`
- [x] **T042** [P] Implement correlated GitHub workflow dispatch, target progress, wait/cancel semantics, aggregate verification, quarantined partial downloads, and atomic success output · `internal/buildtool/aggregate.go`, `cmd/build/main.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent command, smoke, trust, and license integrations:**

- [x] **T043** [P] Add the remote aggregate command (now `task package:all:remote OUTPUT=<directory>`) over the Go helper with automatic current-branch/`origin` resolution, authenticated `gh` prerequisite, exact pushed-SHA validation, and complete-matrix exit semantics · `Taskfile.yml`, `internal/buildtool/aggregate.go`
- [x] **T044** [P] Extend the native smoke harnesses with PE/ELF identity, runtime prerequisite, exact inventory, metadata, checksum, player connection, secure-store state, and resource-release evidence · `scripts/verify-windows-package.ps1`, `scripts/verify-linux-package.sh`
- [x] **T045** [P] Migrate macOS CI/release/reproducibility entrypoints to pinned Task commands without weakening bundle identity, signature, notarization, DMG, Gatekeeper, or hash checks · `.github/workflows/wails-macos.yml`, `scripts/build-macos.sh`, `scripts/reproducible-build-check.sh`
- [x] **T046** [P] Validate the union of shipped target dependency graphs and add Windows Credential Manager, D-Bus/Secret Service, and Task tooling licenses/notices to packaged compliance evidence · `scripts/dependency-license-check.sh`, `THIRD_PARTY_NOTICES.md`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — native delivery matrix join:**

- [x] **T047** Add the clean-checkout four-job native matrix plus always-running aggregate gate, exact source SHA, upload-after-launch verification, redacted records, and combined-success artifact · `.github/workflows/wails-portable.yml`

**Checkpoint**: The aggregate command independently produces exactly four verified archives from one revision and refuses to expose a partial or mislabeled matrix.

## Phase 6: User Story 4 — Choose and Operate the Correct Distribution (P3)

**Goal**: Let users and maintainers identify the correct target, install prerequisites, launch it, locate non-secret data, bootstrap tools, and troubleshoot failures without guesswork.

**Independent Test**: Give only the README and platform guides to a new maintainer/user and confirm they select the correct archive, identify prerequisites and data locations, launch it, and find the Task packaging commands in under five minutes.

### Tests

**Wave 1 — documentation contract tests:**

- [x] **T048** Add documentation assertions for all four exact identifiers/names, `make tools`, migrated Task commands, OS/runtime baselines, launch steps, data locations, secure-store behavior, aggregate packaging, and actionable failures · `internal/platform/startup_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent guidance surfaces:**

- [x] **T049** [P] Replace active Make workflow guidance with the tool bootstrap, Task quickstart, four-target download table, portable launch summary, and links to detailed platform/release guidance · `README.md`
- [x] **T050** [P] Document Windows 10/11 WebView2 and Linux GTK4/WebKitGTK6/Secret Service prerequisites, archive selection/launch, OS-native session/settings locations, secure-store expectations, and troubleshooting · `docs/platform-support.md`
- [x] **T051** [P] Document every migrated Task command, Wails-compatible invocation, matching-host package commands, remote aggregate authentication/current-branch/`origin`/output behavior, artifact layouts, CI evidence, and failure semantics · `docs/platform-packaging.md`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — usability acceptance record:**

- [x] **T052** Create and complete the five-minute target-selection, prerequisite, launch, data-location, tool-bootstrap, and package-matrix documentation checklist · `specs/021-windows-linux-support/checklists/distribution-guidance.md`

**Checkpoint**: A user or maintainer can independently choose, launch, operate, troubleshoot, and package the correct distribution using published guidance alone.

## Phase 7: Polish and Cross-Cutting Validation

**Wave 1 — implementation refinement:**

- [x] **T053** Simplify and review changed Go orchestration/platform code and the Wails beta.13 dependency cutover for duplicate helpers, unnecessary state, context/error handling, cleanup ownership, secret lifetime, dependency hygiene, mutually compatible runtime/tool/frontend pins, and idiomatic quality · `cmd/build/main.go`, `internal/buildtool/target.go`, `internal/buildtool/preflight.go`, `internal/buildtool/package.go`, `internal/buildtool/archive.go`, `internal/buildtool/verify.go`, `internal/buildtool/aggregate.go`, `internal/platform/paths.go`, `internal/platform/keychain.go`, `internal/platform/keychain_windows.go`, `internal/platform/keychain_linux.go`, `internal/tunnel/manager.go`, `main.go`, `app.go`, `wails_host.go`, `go.mod`, `go.sum`, `tools/wails/go.mod`, `tools/wails/go.sum`, `frontend/overseer/package.json`, `frontend/package-lock.json`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — cutover audit:**

- [x] **T054** Audit active repository commands and dependencies; enforce the exact Wails beta.13 runtime/CLI/frontend pins; and remove remaining non-bootstrap Make workflow references, unqualified/global Go tools, stale Taskfile prohibitions, and obsolete macOS-only support claims · `Taskfile.yml`, `Makefile`, `README.md`, `docs/platform-support.md`, `docs/platform-packaging.md`, `scripts/tool-modules-check.sh`, `scripts/wails-v3-contract-check.sh`, `scripts/wails-v3-cutover-check.sh`, `scripts/build-macos.sh`, `scripts/reproducible-build-check.sh`, `internal/platform/startup_test.go`, `.github/workflows/wails-macos.yml`, `.github/workflows/wails-portable.yml`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — single-owner Success Criteria validation in CI (reopened by BUG-004; final execution follows Phase 8):**

- [x] **T055** ⚠️ Reopened (reopened — BUG-004): After T056–T058, T060, and T062 complete, run the pinned Task quality gates and both native delivery workflows from one clean revision on matching hosts, exercise the joined five-target tag-release gate without publishing a production tag, and record SC-001 through SC-013 results with honest `NOT RUN` evidence only where credentials or external services are intentionally unavailable; run no local test suite · `specs/021-windows-linux-support/validation.md`, `.github/workflows/wails-portable.yml`, `.github/workflows/wails-macos.yml`

## Dependencies & Execution Order

- Phase order is strict: Setup → Foundational → User Story 1 → User Story 2 → User Story 3 → User Story 4 → Polish.
- Phase 1: T001 blocks the independent T002–T004 tooling wave; its join blocks the independent T005–T007 cutover-guard wave.
- Phase 2: independent failing tests T008–T009 block T010; T010 blocks independent consumers T011–T012; their join blocks T013.
- Phase 3: independent tests T014–T016 block independent runtime inputs T017–T019; their join blocks T020 → T021 → T022.
- Phase 4: independent tests T023–T027 block independent foundations T028–T031; their join blocks independent native adapters T032–T033 → composition T034 → secret/UX cutover T035.
- Phase 5: independent tests T036–T039 block independent artifact services T040–T042; their join blocks independent integrations T043–T046; that join blocks workflow T047.
- Phase 6: T048 blocks independent guidance T049–T051; their join blocks checklist T052.
- Phase 7: refinement T053 blocks cutover audit T054. BUG-004 reopens the single CI validation owner T055 as the final join after Phase 8 tasks T056–T058, T060, and T062.
- Phase 8: T061 supplies the local Docker aggregate implementation; BUG-001 follow-up T062 verifies its payload, atomicity, and prerequisite-diagnostic contract independently of T056–T058 native acceptance.
- Phase 8: BUG-002 task T063 depends on T061 and extends T062's controlled contract surface with safe repeat publication and rollback; it remains independent of T056–T058 native acceptance.
- Phase 8: BUG-003 task T064 depends on T063, joins the canonical native Darwin package before the Docker matrix, and remains independent of T056–T058 Windows/Linux native acceptance.
- Phase 8: BUG-004 reopens T060; it depends on the existing macOS release path T045 and portable native aggregate T047, and it must finish before the reopened final validation owner T055.
- User Story 1 is the MVP artifact slice. User Story 2 depends on its runnable target composition; User Story 3 depends on Stories 1–2 for artifact and native acceptance evidence; User Story 4 documents the settled commands and behavior from Stories 1–3.

## Phase 8: Convergence

**Purpose**: Close the remaining native acceptance gap without running a local test suite.

- [x] **T056** [P] Extend both matching-host package smoke harnesses to save the opened demo through the native JSON dialog, reopen the saved copy, perform one player control action, verify the resulting synchronized state, and exercise an allowed HTTP/HTTPS external link without weakening existing shutdown/resource assertions [US2/AC1, US2/AC2, FR-005, FR-006, SC-003] · `scripts/verify-windows-package.ps1`, `scripts/verify-linux-package.sh`
- [x] **T057** [P] Add matching-host secure-store acceptance coverage that writes, reads, replaces, and deletes disposable public-access credentials in Windows Credential Manager and Linux Secret Service; also exercise unavailable/locked service behavior, prove local/LAN continuity, and scan owned files, logs, and public state for secret leakage [US2/AC3, FR-008, SC-005, SC-006] · `scripts/verify-windows-package.ps1`, `scripts/verify-linux-package.sh`, `scripts/secret-leak-check.sh`
- [ ] **T058** After T056–T057, run only the matching native delivery workflows from a clean pushed revision, require every portable matrix job to execute the expanded acceptance harness, and replace applicable `NOT RUN` entries with correlated Windows/Linux evidence while keeping unavailable credential-dependent public-tunnel checks explicitly `NOT RUN` [US2/AC1, US2/AC2, US2/AC3, US2/AC4, SC-003, SC-005, SC-006] · `.github/workflows/wails-portable.yml`, `specs/021-windows-linux-support/validation.md`
- [x] **T059** Add an automatic matching-runner CI build matrix for Windows/Linux on amd64/arm64 through the pinned Task entrypoint, verify each executable output, and document the build gate separately from the full portable packaging workflow [FR-001, FR-002, FR-013, SC-002] · `.github/workflows/wails-cross-platform.yml`, `README.md`
- [x] **T060** ⚠️ Reopened (reopened — BUG-004): Extend the implemented four-target tag publication into a one-SHA five-target release by running the established macOS Developer ID signing, hardened-runtime, notarization, stapling, DMG, Gatekeeper, and checksum path on the tag; join that Darwin artifact with the four eligible Windows/Linux archives and aggregate index; and let only repository-pinned GoReleaser v2 publish the exact complete inventory to one GitHub Release and versioned GHCR artifact with prerelease handling and no partial success [US3/AC8, FR-013, FR-014, FR-015, FR-029, SC-001, SC-004, SC-007, SC-013, BUG-004] · `.goreleaser.yaml`, `.github/workflows/wails-portable.yml`, `.github/workflows/wails-macos.yml`, `scripts/build-macos.sh`, `tools/goreleaser/go.mod`, `tools/goreleaser/go.sum`, `tools/oras/go.mod`, `tools/oras/go.sum`, `scripts/tool-modules-check.sh`, `internal/platform/portable_release_test.go`, `README.md`, `docs/platform-packaging.md`, `specs/021-windows-linux-support/contracts/artifact-layout.md`, `specs/021-windows-linux-support/contracts/verification-matrix.md`, `specs/021-windows-linux-support/data-model.md`, `specs/021-windows-linux-support/validation.md`
- [x] **T061** Build and statically verify all four portable targets from the current checkout through architecture-matched Docker builds, export each executable with its required resources, atomically publish only the complete matrix through `task package:all`, retain native GitHub orchestration as `task package:all:remote`, and document that local output is not native launch evidence [US3/AC1, FR-018, FR-025, SC-001] · `.dockerignore`, `build/docker/Dockerfile.package`, `Taskfile.yml`, `cmd/build/main.go`, `internal/buildtool/buildtool.go`, `internal/buildtool/docker.go`, `README.md`, `docs/platform-packaging.md`
- [x] **T062** Add controlled Go/contract verification for all four local `bin/<os>-<arch>/` executable/resource payloads, exact archive inventory/hash equality, atomic no-partial-output behavior, and missing/stopped/unsupported Docker diagnostics that preserve the cause and recovery instruction; run it in CI without requiring developers to execute the four-target Docker build locally [US3/AC4, US3/AC5, FR-025, FR-026, SC-010, BUG-001] · `internal/buildtool/docker.go`, `internal/buildtool/docker_test.go`, `internal/platform/startup_test.go`, `specs/021-windows-linux-support/validation.md`
- [x] **T063** Implement repeatable local aggregate publication: allow replacement of the repository-owned default or a recognized previous aggregate only after full verification, preserve it through build failures, swap through a same-filesystem sibling work-root backup with rollback on final publish failure, reject unsafe existing targets, and update packaging guidance and controlled contract coverage without running a local four-target build [US3/AC6, FR-027, SC-011, BUG-002] · `internal/buildtool/docker.go`, `internal/buildtool/docker_test.go`, `README.md`, `docs/platform-packaging.md`, `specs/021-windows-linux-support/contracts/package-cli.md`, `specs/021-windows-linux-support/data-model.md`, `specs/021-windows-linux-support/validation.md`
- [x] **T064** Require `darwin/arm64` for local `task package:all`, execute the existing canonical no-target package plan, safely copy and verify the complete ad-hoc signed application bundle into `OUTPUT/bin/darwin-arm64/Fallout Terminal.app`, report its path beside the four Docker payloads, include all five targets in one replacement transaction, and update contracts/guidance without changing the remote Windows/Linux release matrix or running local builds/tests [US3/AC7, FR-015, FR-028, SC-007, SC-012, BUG-003] · `internal/buildtool/docker.go`, `internal/buildtool/docker_test.go`, `internal/buildtool/aggregate.go`, `cmd/build/main.go`, `README.md`, `docs/platform-packaging.md`, `specs/021-windows-linux-support/contracts/artifact-layout.md`, `specs/021-windows-linux-support/contracts/package-cli.md`, `specs/021-windows-linux-support/contracts/task-runner.md`, `specs/021-windows-linux-support/data-model.md`, `specs/021-windows-linux-support/validation.md`
