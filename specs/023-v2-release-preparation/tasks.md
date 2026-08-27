# Tasks: V2 Release Preparation

**Input**: Design documents from `specs/023-v2-release-preparation/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, and
`contracts/release-identity.md`

**Bugfix**: BUG-001 is included in User Story 1. Its tasks establish one tag-derived `VERSION`,
generate Go and platform metadata from it, and reject package-version mismatches before upload.
Post-analysis remediation applies Constitution 8.1.0, exact local/report/template contracts, a
corrected wave DAG, precise generated inventories, and rollback verification.

**Testing**: The specification requires focused Go tests, workflow and generated-contract checks,
race testing, browser journeys, and native package inspection. Tests in each story are written or
strengthened before their implementation wave.

**Organization**: Tasks are grouped by prioritized user story. `[P]` means every task in that wave
touches different files and has no incomplete dependency within the wave.

## Phase 1: Setup

**Purpose**: Confirm shared project and tooling prerequisites.

No setup change is required. The approved plan reuses the checked-in Go tool modules, Task graph,
build tool, release workflow, protocol generators, and browser harness without adding dependencies
or scaffolding.

**Checkpoint**: Existing project structure and pinned tooling are sufficient for implementation.

---

## Phase 2: Foundational V2 Source Identity

**Purpose**: Establish the exact root `/v2` identity and an active-source cutover gate. This phase
blocks every user story.

### Tests

**Wave 1 — independent (different files):**

- [x] **T001** [P] Add exact-root-module regression cases that reject the unsuffixed module, substring matches, and other majors · `internal/buildtool/buildtool_test.go`
- [x] **T002** [P] Extend the active-source cutover fixture to detect unsuffixed Go imports while excluding repository URLs, tool modules, and completed specifications · `scripts/wails-v3-cutover-check.sh`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2:**

- [x] **T003** Set the exact root module to `github.com/obalunenko/Fallout-Terminal/v2` and make repository-root validation compare that complete identity · `go.mod`, `internal/buildtool/buildtool.go`

**⟶ Wait for Wave 2 to finish, then Wave 3:**

- [x] **T004** Complete the atomic `/v2` import cutover for every file in the exact T004 inventory below until the cutover gate and `go test ./...` find no active fallback · `main.go`, `app.go`, `app_contract.go`, `desktop_service.go`, `wails_host.go`, `cmd/build/main.go`, `cmd/build/main_test.go`, `cmd/native-credential-smoke/main.go`, `tests/browser/fixture-server/main.go`

**T004 active Go import inventory**:

```text
app.go
app_contract.go
app_contract_test.go
app_test.go
desktop_service.go
desktop_service_test.go
main.go
wails_host.go
wails_host_test.go
cmd/build/main.go
cmd/build/main_test.go
cmd/native-credential-smoke/main.go
internal/control/service.go
internal/control/service_test.go
internal/hack/hack.go
internal/hack/hack_test.go
internal/live/service.go
internal/live/service_test.go
internal/nav/nav.go
internal/nav/nav_test.go
internal/platform/assets_test.go
internal/platform/keychain.go
internal/platform/keychain_darwin_integration_test.go
internal/platform/keychain_test.go
internal/platform/keychain_windows_test.go
internal/player/adapter.go
internal/player/adapter_test.go
internal/player/handler.go
internal/player/handler_test.go
internal/player/http.go
internal/player/http_test.go
internal/player/public_stream_test.go
internal/player/server.go
internal/player/server_test.go
internal/player/stream.go
internal/player/stream_test.go
internal/playerconfig/contract.go
internal/playerconfig/contract_test.go
internal/playerconfig/service.go
internal/playerconfig/service_test.go
internal/playerconfig/storage.go
internal/session/contract.go
internal/session/contract_test.go
internal/session/service.go
internal/session/service_test.go
internal/session/storage_test.go
internal/testutil/public_access_fakes.go
internal/testutil/public_access_fakes_test.go
internal/tunnel/manager_test.go
internal/tunnel/ngrok_integration_test.go
internal/tunnel/public_ingress_test.go
internal/tunnel/settings.go
internal/tunnel/settings_test.go
tests/browser/fixture-server/main.go
```

**Checkpoint**: The repository builds through exactly one `/v2` application source identity and
independent `tools/*` module identities remain unchanged.

---

## Phase 3: User Story 1 — Publish a Valid V2 Release (Priority: P1) 🎯 MVP

**Goal**: A strict stable or prerelease v2 tag supplies one canonical application version to every
target, and each package proves its executable and native metadata match before upload.

**Independent Test**: Exercise accepted, malformed, non-v2, missing-version, prerelease, and
mismatch fixtures; build a native package with `VERSION=2.0.0-rc.1`; confirm the executable reports
that full value, numeric metadata derives `2.0.0`/`2.0.0.0`, and runtime composition never starts.

### Tests

**Wave 1 — independent (different files):**

- [x] **T005** [P] [US1] Add failing table cases for strict tag parsing, canonical version extraction, numeric-core/four-part derivation, non-v2 majors, leading zeroes, build metadata, malformed non-empty `VERSION`, and empty local-version mode · `internal/buildtool/releasecheck_test.go`
- [x] **T006** [P] [US1] Add failing tests for the linker-set application version, exact `development` default, and refusal to treat that default as a tagged version · `internal/version/version_test.go`
- [x] **T007** [P] [US1] Add failing entrypoint tests proving `<executable> --version` accepts no additional arguments, writes only the identity plus one newline to stdout, writes no stderr, exits 0, and enters no Wails/application-service composition · `main_test.go`
- [x] **T008** [P] [US1] Add failing package-plan tests for release/local modes, linker arguments, retained VCS metadata, immutable templates, exact staged render paths, and stable/prerelease/development representations · `internal/buildtool/package_test.go`
- [x] **T009** [P] [US1] Add failing archive-inspection fixtures for exact `--version` output and Darwin/Windows human-readable and numeric metadata matches, including missing, `development`, malformed, and mismatched values · `internal/buildtool/releaseversion_test.go`
- [x] **T010** [P] [US1] Add failing static workflow assertions that preflight exports one canonical `VERSION`, all five package jobs consume it, verification precedes upload, and no target re-derives it · `internal/platform/portable_release_test.go`
- [x] **T011** [P] [US1] Add failing CLI cases for canonical tag output and version-aware archive inspection usage and diagnostics · `cmd/build/main_test.go`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent (different files):**

- [x] **T012** [P] [US1] Extend the strict release model to return the canonical semantic version and deterministic numeric representations from one accepted v2 tag · `internal/buildtool/releasecheck.go`
- [x] **T013** [P] [US1] Create the dependency-free application version owner with an explicit non-release default and a linker-set canonical value · `internal/version/version.go`

**⟶ Wait for Wave 2 to finish, then Wave 3 — independent (different files):**

- [x] **T014** [P] [US1] Implement deterministic rendering from validated release or `development` representations to the exact Darwin and Windows staging paths without mutating checked-in templates · `internal/buildtool/packageversion.go`
- [x] **T015** [P] [US1] Implement the exact `<executable> --version` stdout/stderr/exit contract before runtime composition · `main.go`

**⟶ Wait for Wave 3 to finish, then Wave 4 — independent (different files):**

- [x] **T016** [P] [US1] Select `development` for empty local `VERSION`, reject malformed non-empty release values, inject release values with `-X`, retain VCS metadata, and render isolated target metadata before native tooling runs · `internal/buildtool/buildtool.go`, `internal/buildtool/package.go`
- [x] **T017** [P] [US1] Rename hard-coded production metadata to immutable templates with renderer tokens and no independent release value · `build/darwin/Info.plist` → `build/darwin/Info.plist.tmpl`, `build/windows/info.json` → `build/windows/info.json.tmpl`, `build/windows/app.manifest` → `build/windows/app.manifest.tmpl`

**⟶ Wait for Wave 4 to finish, then Wave 5:**

- [x] **T018** [US1] Verify packaged executables on matching native hosts and inspect Darwin/Windows metadata against canonical and numeric expected values before declaring an archive eligible · `internal/buildtool/releasecheck.go`, `internal/buildtool/packageversion.go`

**⟶ Wait for Wave 5 to finish, then Wave 6:**

- [x] **T019** [US1] Expose canonical tag output and required `inspect-release-archive --version <canonical>` validation through actionable `cmd/build` flags and status output · `cmd/build/main.go`

**⟶ Wait for Wave 6 to finish, then Wave 7:**

- [x] **T020** [US1] Export canonical `VERSION` once from preflight, pass it unchanged into every native package and `--version` inspection step, reject an empty output, and keep upload dependent on successful inspection · `.github/workflows/wails-portable.yml`

**Checkpoint**: User Story 1 is independently functional: strict v2 tags enter the matrix, all five
jobs share one version, and a mismatched executable or native metadata value cannot be uploaded.

---

## Phase 4: User Story 2 — Preserve Existing Application Compatibility (Priority: P1)

**Goal**: The `/v2` release changes source and generated language identity only; protocol, bridge,
persistence, runtime, security, and gameplay behavior remain unchanged.

**Independent Test**: Regenerate contracts and Wails bindings twice, run all breaking fixtures,
open/save/reopen representative version-1 session and player-configuration data, and run the
existing desktop/browser contract journeys.

### Tests

**Wave 1 — independent (different files):**

- [x] **T021** [P] [US2] Strengthen descriptor and generated-asset assertions so only `/v2` `go_package` metadata changes while protobuf packages, fields, services, directions, and browser descriptors remain stable · `internal/platform/assets_test.go`
- [x] **T022** [P] [US2] Retain version-1 session and player-configuration round trips, including recursive session unknown-field preservation and player-config unknown-field rejection without source rewrite · `internal/session/service_test.go`, `internal/playerconfig/service_test.go`
- [x] **T023** [P] [US2] Assert the clean `/v2` Wails binding path, one desktop service, 35 methods, six events, and unchanged browser facade behavior · `tests/browser/desktop-api.spec.mjs`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent (different files):**

- [x] **T024** [P] [US2] Change only the exact protobuf inputs below to `/v2` `go_package` values and run `task proto:generate` to reproduce the exact Go, ConnectRPC, and ECMAScript outputs below · `proto/fallout/terminal/config/v1/config.proto`, `proto/fallout/terminal/persistence/v1/session.proto`, `proto/fallout/terminal/player/v1/player.proto`, `proto/fallout/terminal/private/v1/desktop.proto`
- [x] **T025** [P] [US2] Run the exact clean Wails generator for the output inventory below, remove the unsuffixed tree, and move the active facade to the generated `/v2` service · `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js`, `frontend/overseer/src/desktop-api.js`

**T024 exact protobuf input/output inventory**:

```text
proto/fallout/terminal/config/v1/config.proto
proto/fallout/terminal/config/v1/public_access.proto
proto/fallout/terminal/persistence/v1/player_config.proto
proto/fallout/terminal/persistence/v1/session.proto
proto/fallout/terminal/player/v1/hacking.proto
proto/fallout/terminal/player/v1/navigation.proto
proto/fallout/terminal/player/v1/player.proto
proto/fallout/terminal/player/v1/sound.proto
proto/fallout/terminal/player/v1/terminal.proto
proto/fallout/terminal/private/v1/coordination.proto
proto/fallout/terminal/private/v1/desktop.proto
proto/fallout/terminal/private/v1/public_access.proto
proto/fallout/terminal/private/v1/runtime.proto
internal/gen/fallout/terminal/config/v1/config.pb.go
internal/gen/fallout/terminal/config/v1/public_access.pb.go
internal/gen/fallout/terminal/persistence/v1/player_config.pb.go
internal/gen/fallout/terminal/persistence/v1/session.pb.go
internal/gen/fallout/terminal/player/v1/hacking.pb.go
internal/gen/fallout/terminal/player/v1/navigation.pb.go
internal/gen/fallout/terminal/player/v1/player.pb.go
internal/gen/fallout/terminal/player/v1/playerv1connect/player.connect.go
internal/gen/fallout/terminal/player/v1/sound.pb.go
internal/gen/fallout/terminal/player/v1/terminal.pb.go
internal/gen/fallout/terminal/private/v1/coordination.pb.go
internal/gen/fallout/terminal/private/v1/desktop.pb.go
internal/gen/fallout/terminal/private/v1/public_access.pb.go
internal/gen/fallout/terminal/private/v1/runtime.pb.go
frontend/client/gen/fallout/terminal/player/v1/hacking_pb.js
frontend/client/gen/fallout/terminal/player/v1/navigation_pb.js
frontend/client/gen/fallout/terminal/player/v1/player_pb.js
frontend/client/gen/fallout/terminal/player/v1/sound_pb.js
frontend/client/gen/fallout/terminal/player/v1/terminal_pb.js
```

**T025 command and exact output inventory**:

```text
go tool -modfile=tools/wails/go.mod wails3 generate bindings -clean -d frontend/overseer/bindings ./...
frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js
frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/index.js
frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/internal/domain/index.js
frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/internal/domain/models.js
frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/internal/session/index.js
frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/internal/session/models.js
frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/models.js
frontend/overseer/bindings/github.com/wailsapp/wails/v3/internal/eventcreate.js
frontend/overseer/bindings/github.com/wailsapp/wails/v3/internal/eventdata.d.ts
frontend/overseer/src/desktop-api.js
```

**⟶ Wait for Wave 2 to finish, then Wave 3 — independent (different files):**

- [x] **T026** [P] [US2] Review the metadata-only schema delta, advance the schema revision and descriptor baseline, and update reviewed digests while retaining all five negative fixtures · `proto/schema-revision.txt`, `proto/compatibility-baseline.binpb`, `scripts/wails-v3-contract-check.sh`
- [x] **T027** [P] [US2] Update binding and secret-scan gates for the one `/v2` generated namespace without widening the desktop-service or private-model allowlists · `scripts/wails-bindings-check.sh`, `scripts/secret-leak-check.sh`

**Checkpoint**: User Story 2 is independently functional: deterministic generation passes,
breaking changes remain rejected, version-1 data round-trips, and existing runtime journeys retain
their accepted contracts.

---

## Phase 5: User Story 3 — Follow Unambiguous Release Guidance (Priority: P2)

**Goal**: Active maintainer instructions consistently teach v2 release tags, the one-version
invariant, and independent tool-module ownership while completed specifications remain historical.

**Independent Test**: Search active documentation and executable fixtures for release examples;
confirm stable/prerelease examples use v2, package guidance explains version equality, tool-module
checks pass, and completed specs are excluded from rewrites.

### Tests

**Wave 1 — independent (different files):**

- [x] **T028** [P] [US3] Extend active-guidance assertions for v2 stable/prerelease examples, the module/tag major match, the canonical `VERSION` invariant, and explicit non-release local builds · `internal/platform/startup_test.go`
- [x] **T029** [P] [US3] Verify every `tools/*` module retains its independent identity, isolated sums, and pinned execution ownership after the root `/v2` cutover · `scripts/tool-modules-check.sh`

### Implementation

**⟶ Wait for Wave 1 to finish, then Wave 2 — independent (different files):**

- [x] **T030** [P] [US3] Update active release examples and explain the `/v2` module-major, tag-major, canonical `VERSION`, and non-release build relationship · `README.md`
- [x] **T031** [P] [US3] Document stable/prerelease tag normalization, platform numeric representations, per-target inspection, and mismatch recovery without changing archive inventory · `docs/platform-packaging.md`

**Checkpoint**: User Story 3 is independently functional: maintainers have one current v2 release
procedure, tool modules remain independent, and historical records remain untouched.

---

## Final Phase: Cross-Cutting Verification and Polish

**Purpose**: Validate the complete feature against SC-001 through SC-008 with one owner for suite
execution; no post-implement hook claims validation ownership.

**Wave 1 — independent (different files):**

- [x] **T032** [P] Run the protocol, binding, cutover, tool-isolation, secret-leak, and reproducibility gates and resolve only feature-caused failures · `scripts/proto-check.sh`, `scripts/proto-breaking.sh`, `scripts/wails-bindings-check.sh`, `scripts/wails-v3-contract-check.sh`, `scripts/wails-v3-cutover-check.sh`, `scripts/tool-modules-check.sh`, `scripts/secret-leak-check.sh`, `scripts/reproducible-build-check.sh`
- [x] **T033** [P] Search active source, metadata templates, workflow, and current docs for unsuffixed application imports, independent production version literals, non-v2 release examples, or more than one release `VERSION` derivation; preserve matches in completed specifications · `go.mod`, `build/darwin/Info.plist.tmpl`, `build/windows/info.json.tmpl`, `build/windows/app.manifest.tmpl`, `.github/workflows/wails-portable.yml`, `README.md`, `docs/platform-packaging.md`

**⟶ Wait for Wave 1 to finish, then Wave 2:**

- [x] **T034** Run `task fmt:check`, `task vet`, `task lint`, `task test`, `task test:race`, `task proto:check`, `task proto:breaking`, `task bindings:check`, `task frontend:build`, `task browser:test`, and `task build`; record commands, host constraints, and results against SC-001 through SC-008 · `specs/023-v2-release-preparation/validation.md`

**⟶ Wait for Wave 2 to finish, then Wave 3:**

- [x] **T035** Run native `task package` with empty `VERSION`, `VERSION=2.0.0`, and `VERSION=2.0.0-rc.1`; prove `development` cannot pass release inspection, stable/prerelease executable and platform metadata match, templates remain unchanged, and archive layout is stable · `specs/023-v2-release-preparation/validation.md`

**⟶ Wait for Wave 3 to finish, then Wave 4:**

- [x] **T036** Validate the pre-publication rollback procedure from immutable revision `3f2b6e584aee4c5279a3d54ae70aa44ee578a21a` in an isolated temporary tree, prove either direction retains one root module and one Wails namespace, and record that complete published releases require a new forward-fix tag · `specs/023-v2-release-preparation/validation.md`

**Checkpoint**: All success criteria have reproducible evidence, no independent hard-coded release
version remains, and BUG-001 is ready for post-implementation consistency verification.

---

## Dependencies & Execution Order

- Phase 1 confirms there is no setup work; Phase 2 is the shared blocker for all user stories.
- Phase 2: Wave 1 tests and scan fixtures block Wave 2's root declaration; Wave 2 blocks Wave 3's atomic import cutover.
- Phase 3 / US1: Wave 1 failing contracts block Wave 2 parser/version owners; Wave 2 blocks Wave 3 renderer/report consumers; Wave 3 blocks Wave 4 package/template integration; Wave 4 blocks Wave 5 inspection; Wave 5 blocks Wave 6 CLI; Wave 6 blocks Wave 7 workflow propagation.
- Phase 4 / US2: Wave 1 compatibility tests block Wave 2 regeneration; Wave 2 blocks Wave 3 baseline and guard updates.
- Phase 5 / US3: Wave 1 guidance/tool assertions block Wave 2 documentation updates.
- US1 and US2 may proceed independently after Phase 2 because their listed files do not overlap; US3 consumes the settled release contract and follows them for documentation accuracy.
- Polish waits for all user stories: its independent static gates join before the full suite, the full suite joins before native package evidence, and package evidence joins before rollback validation.

## Parallel Opportunities

- Within each `[P]` wave, tasks have separate owners and files and can proceed in any order.
- The release parser and application-version owner in US1 can be built in parallel after their
  failing tests land; the renderer and executable-report consumer can then proceed in parallel.
- Protocol regeneration and Wails regeneration in US2 can run in parallel before their separate
  baseline and allowlist joins.
- README and platform-packaging guidance can be updated in parallel after their assertions define
  the required wording.

## Implementation Strategy

1. Lock the exact `/v2` foundation and its active-source regression gate.
2. Deliver US1 as the MVP: one accepted tag, one canonical version, five verified package jobs.
3. Verify US2 independently to prove the source/version cutover did not alter contracts or data.
4. Update US3 guidance only after the executable release flow is settled.
5. Run the single-owner cross-cutting suite and native package checks, then invoke BUG-001
   consistency verification.

## Notes

- Do not mark formatting, lint, tests, generation, browser, package, or release checks successful
  unless the named command actually ran.
- Follow the repository test-cleanup rule: register test-owned resources immediately with
  `t.Cleanup`; use `context.WithoutCancel(t.Context())` plus a bounded timeout when cleanup can
  block.
- Never rewrite completed specifications or migration records to remove historical paths or
  version examples.
- A live release tag, upload, signing, notarization, installer, checksum-policy change, and archive
  inventory change remain out of scope.
