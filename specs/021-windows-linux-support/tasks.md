# Tasks: Windows and Linux Desktop Support — Constitution v8 Delta

**Input**: Design documents in `specs/021-windows-linux-support/`

**Historical record**: The prior `T001`–`T067` implementation record remains in [tasks-history.md](./tasks-history.md); its unchecked legacy validation item is superseded with the rest of that queue. This active list contains only the unchecked constitution-v8 convergence delta from `T068` through `T099` and deliberately starts at `T068` so historical Companion events cannot complete new work.

**Tests**: Constitution v8 requires static, network-free contract tests and focused Go tests before implementation, followed by local build/package validation and one maintainer-approved live prerelease acceptance after commit. Native UI, dialog, credential-store, player, tunnel, and signing journeys may still be run manually or separately, but they do not define platform support or gate tagged releases.

## Phase 1: Setup

**Purpose**: Confirm that no new dependency or project scaffold is needed before the delta.

No setup task is required. The existing Go test structure, pinned Task/Wails/GoReleaser modules, workflow directory, and buildtool package are the approved foundations; adding a baseline install/build task would not produce implementation value.

## Phase 2: Foundational — Exact Targets and Non-Publishing Checks

**Purpose**: Establish shared target identity and network-free release validation before any user-story implementation.

**Wave 1 — independent failing contract tests (different files):**

- [x] **T068** [P] Extend table-driven target tests to accept exactly `windows/amd64`, `windows/arm64`, `linux/amd64`, `linux/arm64`, and `darwin/arm64`, reject aliases/case variants/`darwin/amd64`, assert stable archive names and formats, and require exact matching hosts · `internal/buildtool/target_test.go`
- [x] **T069** [P] Add table-driven, network-free failing fixtures for strict release SemVer, minimal ZIP/TAR.GZ eligibility, exact five-file inventory, cancellation, missing executable/resources, empty archives, unexpected sidecars/indexes/raw files/DMGs/verification-record assets, and safe test-owned cleanup with `t.Cleanup`; keep forbidden-extra-asset checks separate from the four-condition per-archive eligibility contract · `internal/buildtool/releasecheck_test.go`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent shared implementations (different files):**

- [x] **T070** [P] Make the target model a closed five-pair set with runner-independent stable ZIP/TAR.GZ names, expected executable paths, required-resource paths, and explicit `darwin/arm64` portable identity · `internal/buildtool/target.go`
- [x] **T071** [P] Implement pure, network-free release-tag, minimal archive, and exact publication-inventory validators with cancellation-aware readers and no checksum, launch, architecture, signing, or external-service eligibility requirements · `internal/buildtool/releasecheck.go`

**Checkpoint**: Exact target identity and all non-publishing release decisions are independently testable without GitHub access or native UI automation.

## Phase 3: User Story 1 — Obtain a Complete Archive for Every Supported Target (P1)

**Goal**: Produce the fifth portable target through the same explicit package flow while preserving the four existing Windows/Linux archives and the resources each archive requires.

**Independent Test**: Run focused target, package, and archive tests to prove each exact target has a stable non-empty portable archive contract and that Darwin contains the unsigned application bundle, executable, metadata, notices, icon, and demo resources. Native launch evidence is optional, non-gating, and not required to claim archive availability.

### Tests

**Wave 1 — independent failing package contracts (different files):**

- [x] **T072** [P] [US1] Extend package-plan tests for explicit `darwin/arm64` native compilation, complete unsigned application-bundle staging, stable collision-free output, exclusion of user-owned documents, private settings, credentials, and plaintext secret fallbacks, no DMG/sign/notarize/staple command, and failure cleanup registered immediately with `t.Cleanup` · `internal/buildtool/package_test.go`
- [x] **T073** [P] [US1] Extend archive tests for deterministic Darwin ZIP bundle layout, executable mode, required resources, non-empty output, unsafe-path rejection, cancellation, preservation of existing Windows/Linux behavior, and absence across target archives of user-owned documents, private settings, credentials, plaintext secret fallbacks, and secret-bearing verification records, using `t.Cleanup` for test-owned files · `internal/buildtool/archive_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent Darwin package components (different files):**

- [x] **T074** [P] [US1] Extend the common target-aware package plan to stage the complete unsigned `Fallout Terminal.app` for explicit `darwin/arm64` while keeping implicit developer packaging separate from tagged-release inputs · `internal/buildtool/package.go`, `internal/buildtool/buildtool.go`
- [x] **T075** [P] [US1] Extend the common archive writer to emit `Fallout-Terminal-darwin-arm64.zip` containing the intact application bundle while retaining local-only checksum output for optional `package:all` and excluding it from release eligibility · `internal/buildtool/archive.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — package-entrypoint integration:**

- [x] **T076** [US1] Wire and document the explicit `task package GOOS=darwin GOARCH=arm64` path through the existing `cmd/build package --target` boundary, preserve paired input validation and implicit local packaging, and expose no signing or DMG branch to tagged callers · `Taskfile.yml`, `cmd/build/main.go`

**Checkpoint**: User Story 1 is independently functional: all five targets have one explicit native package entrypoint and one stable portable archive contract.

## Phase 4: User Story 2 — Preserve Product Quality Outside Releases (P1)

**Goal**: Retain project quality and startup contracts for pull requests and `main` pushes without coupling them to the five-target release matrix or publishing assets.

**Independent Test**: Static repository tests prove that the quality workflow has only pull-request/main triggers, read-only permissions, all required quality checks, zero target-matrix rows, zero release publishers, and zero release assets; optional native journeys remain non-gating evidence.

### Tests

**Wave 1 — failing workflow separation contract:**

- [x] **T077** [US2] Replace duplicate-CI assertions with static tests for the sole PR/main quality workflow: Go tests and vet, locked frontend install, clean Overseer/player builds, startup contracts, exact Wails pins, clean binding generation, Buf/protobuf formatting, lint, generation-drift, breaking, and generated-code checks, read-only permissions, no five-target matrix, and no release/package publication · `internal/platform/portable_release_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — repository-owned quality composition:**

- [x] **T078** [US2] Add a deterministic CI-quality Task composition using locked dependencies for Go tests/vet, both frontend production builds, startup/Wails-pin contracts, binding generation, and Buf/protobuf formatting, lint, generation-drift, breaking, and generated-code checks while keeping packaging, native journeys, signing, and publication outside the task · `Taskfile.yml`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — quality workflow cutover:**

- [x] **T079** [US2] Rewrite the cross-platform workflow as the sole read-only pull-request/main quality workflow, invoke the repository-owned quality composition including Buf/protobuf checks, remove `workflow_dispatch` and target-matrix/release-config jobs, and upload no distribution or release asset · `.github/workflows/wails-cross-platform.yml`

**Checkpoint**: User Story 2 is independently testable: PR/main automation preserves required quality checks and has no path to release publication.

## Phase 5: User Story 3 — Publish Simple Tagged Releases for All Targets (P2)

**Goal**: Build exactly five native archives only for qualifying SemVer tags and publish them create-only to one GitHub Release through repository-pinned GoReleaser.

**Independent Test**: First run network-free Go fixtures and static workflow/config tests proving strict tag refusal, preflight before matrix, exact native runner/command rows, minimal archive checks, all-five success gating, exact assets, sole GoReleaser publication, existing-release refusal, and manual partial-release recovery without mutation. After all local/static gates pass and the implementation is committed, push one explicitly maintainer-approved unused SemVer prerelease tag and verify the preserved real five-archive GitHub Release.

### Tests

**Wave 1 — independent failing release cutover contracts (different files):**

- [x] **T081** [P] [US3] Replace aggregate/DMG/native-smoke publication assertions with static tests for tag-only triggers, strict preflight before matrix, the exact five native runner rows and Task commands, minimal non-gating validation, all-five publication dependency, exact archive transport with no separate secret-bearing verification record, sole `release:publish` Task entrypoint invoking pinned GoReleaser, two existing-release refusals including drafts, distinct no-release immediate-rerun versus partial-release manual-delete/rerun diagnostics with no rollback/mutation, and absence of stale standalone-macOS/proto-script references · `internal/platform/portable_release_test.go`
- [x] **T082** [P] [US3] Add CLI tests for `validate-release-tag`, `inspect-release-archive`, and `inspect-release-inventory`, retained `package-all-docker`, rejected obsolete `package-all`/`release-candidate` actions, usage diagnostics, cancellation, and test-owned cleanup via `t.Cleanup` · `cmd/build/main_test.go`
- [x] **T083** [P] [US3] Update startup/tool-surface assertions for retained explicit package and local `package:all`, added CI-owned `release:publish`, removed `package:all:remote`/`release:local`, pinned GoReleaser ownership, ORAS absence, no CI Docker/manual-signed invocations, and Make remaining bootstrap/help-only · `internal/platform/startup_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent publication foundations and cutovers (different files):**

- [x] **T084** [P] [US3] Expose the three network-free release-check actions in `cmd/build`, preserve `package-all-docker`, and remove the remote aggregate and joined-release action dispatch/help surface without adding any network publisher · `cmd/build/main.go`
- [x] **T085** [P] [US3] Move every dependency retained by optional Docker packaging—local result/record types, artifact interfaces, exact-target helpers, clone/validation functions, directory verification, constants, and index structures—out of the remote aggregate coordinator into a local-only boundary; update Docker aggregation and tests to depend only on that boundary before the coordinator is deleted · `internal/buildtool/local_aggregate.go`, `internal/buildtool/aggregate.go`, `internal/buildtool/docker.go`, `internal/buildtool/docker_test.go`
- [x] **T086** [P] [US3] Narrow GoReleaser v2 configuration to five prebuilt archives, disabled generated checksums, `draft: false`, automatic prerelease classification, and no keep-existing, replacement, append, second publisher, or extra asset · `.goreleaser.yaml`
- [x] **T087** [P] [US3] Add CI-owned `release:publish` invoking `go tool -modfile=tools/goreleaser/go.mod goreleaser release --clean --config .goreleaser.yaml`; remove `package:all:remote` and `release:local` while retaining explicit `package`, optional local `package:all` → `package-all-docker`, and optional manual `release:macos:*` commands outside CI · `Taskfile.yml`
- [x] **T088** [P] [US3] Remove the isolated ORAS tool module and change tool discovery checks to require pinned GoReleaser while proving ORAS and GitHub Packages publication are absent · `tools/oras/go.mod`, `tools/oras/go.sum`, `scripts/tool-modules-check.sh`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent workflow and obsolete-code cutovers (different files):**

- [x] **T089** [P] [US3] Replace the portable workflow with `v*` push-only coordination: strict SemVer and paginated all-release-state refusal before matrix, exact five matching native runners invoking `task package GOOS=<os> GOARCH=<arch>`, one minimally inspected archive per job, all-five join, exact inventory check, second refusal, one pinned `task release:publish` entrypoint to GoReleaser, per-tag concurrency, and an always-run read-only failure lookup that reports immediate same-tag rerun when no release exists or manual deletion then same-tag rerun when a partial release exists, with no append/replace/delete/rollback · `.github/workflows/wails-portable.yml`
- [x] **T090** [P] [US3] Delete the remote GitHub Actions aggregate coordinator and joined local release implementation/tests after their local Docker dependencies have moved, leaving no remote `package-all` CLI action, release-candidate action, remote download, joined DMG, or rollback implementation while preserving local `package-all-docker` · `internal/buildtool/aggregate.go`, `internal/buildtool/aggregate_test.go`, `internal/buildtool/release.go`, `internal/buildtool/release_test.go`

**⟶ Wait for T089 to finish, then:**

**Wave 4 — ordered standalone-workflow removal:**

- [x] **T080** [US3] Delete the superseded standalone macOS workflow only after the portable workflow no longer calls it, remove the deleted workflow reference from the protobuf-check script, and preserve optional manual signed-macOS Task commands outside CI · `.github/workflows/wails-macos.yml`, `scripts/proto-check.sh`

**Checkpoint**: User Story 3 is independently functional: a qualifying tag can create one complete five-archive GitHub Release, while any failed target or existing release prevents publication and partial publication requires manual recovery.

## Phase 6: User Story 4 — Choose and Operate the Correct Distribution (P3)

**Goal**: Make the exact five downloads, prerequisites, launch/data guidance, CI split, local Docker convenience, and create-only recovery procedure discoverable without obsolete release paths.

**Independent Test**: Give only the README and platform guides to a new user or maintainer and verify they can choose the right archive, identify prerequisites and data locations, distinguish quality from release CI, run explicit packaging, and recover a partial release in under five minutes.

### Tests

**Wave 1 — failing documentation contract:**

- [x] **T091** [US4] Replace stale aggregate/DMG/checksum/package-registry documentation assertions with the exact five names, archive-availability support boundary, portable launch/prerequisite/data/credential guidance that makes native success optional, quality-versus-tag workflow split, explicit package commands, optional local Docker boundary, unsigned Darwin ZIP, create-only refusal, and manual partial-release deletion/rerun procedure · `internal/platform/startup_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — independent guidance surfaces (different files):**

- [x] **T092** [P] [US4] Update the project quickstart and download/release overview for the exact five portable archives, unsigned Darwin ZIP, separate PR/main quality checks, tag-only GoReleaser publication, optional local Docker aggregation, and manual partial-release recovery · `README.md`
- [x] **T093** [P] [US4] Rewrite packaging guidance around the common five-target Task entrypoint, native runner matrix, minimal per-target release eligibility, exact five assets, `release:publish` → pinned-GoReleaser create-only ownership, optional local Docker output, live prerelease acceptance, and deletion-before-rerun recovery; remove active remote aggregate, joined release, DMG tag asset, checksum asset, ORAS, and rollback instructions · `docs/platform-packaging.md`
- [x] **T094** [P] [US4] Update platform support guidance to define support as availability of the governed archive for each exact OS/architecture, document minimum runtime prerequisites and optional portable launch steps including unsigned macOS behavior, explain user/session/settings locations and protected credential expectations, and provide actionable mismatch/runtime failures without making successful native execution an acceptance gate · `docs/platform-support.md`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — documentation usability acceptance:**

- [x] **T095** [US4] Re-run and record the under-five-minute distribution-guidance acceptance against the v8 documents, including all five filenames, archive-availability support meaning, optional native launch guidance, prerequisites, data locations, quality/release distinction, explicit package inputs, local-only Docker status, and manual partial-release recovery · `specs/021-windows-linux-support/checklists/distribution-guidance.md`

**Checkpoint**: User Story 4 is independently functional: users and maintainers can select, package, launch, troubleshoot, and recover the supported distributions without relying on superseded paths.

## Phase 7: Polish and Cross-Cutting Validation

**Wave 1 — independent refinement and specification audit (different files):**

- [x] **T096** [P] Run simplification and idiomatic Go quality review over the changed build/release seams, remove redundant helpers or band-aid state, preserve precise cancellation/errors, and ensure every test-owned resource uses immediate `t.Cleanup` ownership with `context.WithoutCancel(t.Context())` plus a bounded timeout when cleanup can block · `cmd/build/main.go`, `cmd/build/main_test.go`, `internal/buildtool/target.go`, `internal/buildtool/target_test.go`, `internal/buildtool/package.go`, `internal/buildtool/package_test.go`, `internal/buildtool/archive.go`, `internal/buildtool/archive_test.go`, `internal/buildtool/releasecheck.go`, `internal/buildtool/releasecheck_test.go`, `internal/buildtool/local_aggregate.go`, `internal/buildtool/docker.go`, `internal/buildtool/docker_test.go`
- [x] **T097** [P] Re-audit the active spec checklist against constitution v8, all settled clarifications, the fresh delta tasks, and archived bug boundaries; record any unmet requirement without reopening superseded BUG-001–BUG-004 records · `specs/021-windows-linux-support/checklists/requirements.md`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — local and static Success Criteria validation:**

- [x] **T098** Run `gofmt -l .`, `go vet ./...`, `task lint`, `go test ./...`, focused buildtool/platform contract tests, locked frontend production builds, Wails pin and binding checks, `task proto:check`, `task proto:breaking`, `scripts/secret-leak-check.sh`, tool-module checks, pinned GoReleaser config validation, `task build`, and `task package GOOS=darwin GOARCH=arm64`; use static/non-publishing fixtures for release behavior, record the local/static evidence relevant to SC-001–SC-016 and FR-033–FR-035 with exact commands, keep every tag-dependent criterion pending T099, and mark optional native UI/dialog/credential/player/tunnel/signing journeys honestly as `NOT RUN` without failing archive support or the release contract · `specs/021-windows-linux-support/validation.md`

**⟶ Wait for Wave 2 to finish and for the implementation to be committed, then:**

**Wave 3 — live prerelease acceptance:**

- [ ] **T099** [US3] With an explicitly maintainer-approved unused SemVer prerelease tag, push the committed implementation tag, wait for the real five-target workflow, verify the preserved GitHub prerelease exposes exactly the five non-empty governed archives with expected executable/resource contents and no extra assets, and record the tag, release URL, job results, and SC-001/SC-002/SC-004/SC-007/SC-011/SC-013 evidence; if publication fails, follow and record the existing no-release or manual partial-release recovery procedure without automated deletion · `specs/021-windows-linux-support/validation.md`

## Dependencies & Execution Order

- Phase order is strict: Setup → Foundational → User Story 1 → User Story 2 → User Story 3 → User Story 4 → Polish.
- Phase 1 adds no prerequisite work; Phase 2 begins immediately.
- Phase 2: independent failing tests T068–T069 block independent implementations T070–T071.
- Phase 3: independent failing tests T072–T073 block independent package components T074–T075; their join blocks entrypoint integration T076.
- Phase 4: quality contract T077 blocks Task composition T078, which blocks workflow cutover T079.
- Phase 5: independent failing contracts T081–T083 block independent foundations T084–T088; their join blocks independent tag-workflow and obsolete-code cutovers T089–T090. T085 must finish before T090 deletes the remote coordinator. T089 must finish before T080 deletes the standalone macOS workflow and cleans its protobuf-script reference.
- Phase 6: documentation contract T091 blocks independent guidance T092–T094; their join blocks acceptance record T095.
- Phase 7: independent review/audit T096–T097 block local/static validation T098; T098 and a committed implementation block live prerelease acceptance T099.
- User Story 1 is the portable-artifact MVP. User Story 2 depends on shared foundations but remains a non-release increment. User Story 3 depends on Stories 1–2 for the package and quality/release separation. User Story 4 documents the settled behavior from Stories 1–3.

## Parallel Opportunities

- In Foundational, T068 and T069 can run together; after their join, T070 and T071 can run together.
- In User Story 1, T072 and T073 can run together; after their join, T074 and T075 can run together.
- In User Story 3, T081–T083 can run together; after their join, T084–T088 touch independent surfaces and can run together; after that join, T089 and T090 can run together, and T080 follows T089 as the ordered workflow-removal cutover.
- In User Story 4, T092–T094 can run together after T091.
- In Polish, T096 and T097 can run together before T098 performs local/static validation; T099 then performs the sole live prerelease acceptance after commit.
