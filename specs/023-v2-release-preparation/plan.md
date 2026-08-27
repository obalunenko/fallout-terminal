# Implementation Plan: V2 Release Preparation

**Branch**: `023-v2-release-preparation` | **Date**: 2026-08-27 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/023-v2-release-preparation/spec.md`

**Bugfix**: 2026-08-27 — BUG-001 updated from bugfix patch and post-analysis remediation.

## Summary

Prepare the application for the v2 release line by moving the root Go module and every active
application consumer to the `/v2` source identity, regenerating governed protobuf and Wails
artifacts without changing their runtime contracts, and rejecting non-v2 release tags before the
five-target package matrix begins. Preserve version-1 persistence, gameplay, security boundaries,
tool-module ownership, archive inventory, and historical records while updating active release
guidance. ~~Application version embedding remains excluded by the current spec and is tracked for a
subsequent surgical patch in [BUG-001](./bugs/BUG-001.md).~~ BUG-001 brings version identity into
scope: release preflight derives one canonical `VERSION`, every package job consumes it, Go and
platform metadata are generated from it, and per-target inspection rejects disagreement before
upload.

## Project Structure

```text
.
├── .github/workflows/wails-portable.yml        # One tag-derived VERSION for all package jobs
├── go.mod                                      # Root application /v2 module identity
├── main.go, app.go, app_contract.go            # Composition and private bridge imports
├── desktop_service.go, wails_host.go           # Module-qualified Wails service identity
├── cmd
│   ├── build/                                  # Release preflight CLI and fixtures
│   └── native-credential-smoke/                # Application imports under /v2
├── internal
│   ├── buildtool/                              # Root validation and strict v2 tag policy
│   ├── gen/fallout/terminal/                   # Regenerated Go/Connect contract outputs
│   ├── domain/, nav/, hack/, live/, control/   # Source imports; behavior unchanged
│   ├── version/                                # Linker-injected canonical application version
│   ├── session/, playerconfig/                 # Version-1 persistence compatibility
│   ├── player/, platform/, tunnel/             # Transport/platform imports and gates
│   └── testutil/                               # Compatibility fixtures and test fakes
├── proto
│   ├── fallout/terminal/**/v1/*.proto          # Stable packages; /v2 go_package metadata
│   ├── schema-revision.txt                     # Reviewed schema-source identity
│   └── compatibility-baseline.binpb            # Reviewed descriptor baseline
├── frontend
│   ├── client/gen/                             # Regenerated ECMAScript descriptors
│   └── overseer
│       ├── bindings/github.com/obalunenko/Fallout-Terminal/v2/
│       └── src/desktop-api.js                  # One generated desktop-service consumer
├── build
│   ├── darwin/Info.plist.tmpl                  # Checked-in macOS metadata template
│   └── windows/{info.json,app.manifest}.tmpl   # Checked-in Windows metadata templates
├── tests/browser
│   ├── fixture-server/main.go                  # /v2 imports and fixture import maps
│   └── desktop-api.spec.mjs                    # Generated binding-path contract
├── scripts
│   ├── proto-*.sh                              # Deterministic generation/compatibility gates
│   ├── wails-bindings-check.sh                 # /v2 binding namespace and allowlist
│   ├── wails-v3-contract-check.sh              # Reviewed descriptor digests
│   ├── tool-modules-check.sh                   # Independent tool-module ownership
│   └── secret-leak-check.sh                    # Generated private-model allowlist
├── README.md                                   # Active v2 release guidance
├── docs/platform-packaging.md                  # Canonical tag/package procedure
└── specs/023-v2-release-preparation/
    ├── spec.md
    ├── plan.md
    ├── research.md
    ├── data-model.md
    ├── contracts/release-identity.md
    └── bugs/BUG-001.md
```

**Structure Decision**: Perform one repository-wide source-identity cutover through existing module,
generation, build-tool, frontend, test, and documentation owners. BUG-001 adds one small
application-version package plus package-time metadata generation inside the existing build-tool
boundary; it introduces no compatibility module, duplicate binding tree, dependency, protocol
version, persistence format, or second release-policy owner.

## Constitution Check

| Principle | Assessment | Evidence |
|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | Wails v3 remains the only runtime; composition and the one desktop service change only module-qualified imports and generated path. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | All structured application contracts remain protobuf-defined; `go_package`, linker values, plist, PE resources, and manifests remain native to their owning tools. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | RPC methods, directions, routes, validation, revisions, publication, and canonical state ownership remain unchanged. |
| IV. Separate Public and Private Capabilities | PASS | The private bridge retains exactly 35 allowlisted methods and six events; the non-interactive version report exits before Wails startup and adds no Overseer or player capability. |
| V. Evolve Schemas Safely and Reproducibly | PASS | Schema diffs are limited to language-package identity, generation remains pinned/deterministic, the baseline advances explicitly, and all breaking fixtures remain rejected. |
| VI. Preserve Portable Session JSON Version 1 | PASS | Session and player-configuration formats, adapters, validation, defaults, storage, and version values are unchanged and covered by existing round trips. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | Active imports and bindings move atomically to `/v2`; the old application identity is removed from active source while historical records are preserved. |
| Dependency Rules | PASS | Root production dependencies and all isolated `tools/*` modules retain their owning graphs and exact pins. |
| Secret and Credential Governance | PASS | No credential path, schema, event, log, status, persistence, or public projection changes. |
| Go Development Tool Modules | PASS | Existing pinned Buf, protobuf, Connect, Wails, Task, lint, and GoReleaser modules execute through the canonical Task/build graph without identity migration. |
| Testing and Quality Gates | PASS | Constitution 8.1.0 permits deterministic tag-to-artifact identity verification as a tagged-release gate; the plan applies it without adding prohibited native UI, credential, signing, tunnel, or browser gates. |
| Governance and rollback | PASS | The rollback plan below restores the immutable pre-cutover revision before publication and requires a forward-fix release after publication, preserving create-only release history. |

No constitution violations require Complexity Tracking.

The final design re-check remains PASS: the design changes source and generated metadata identity
without introducing a second runtime, protocol, persistence format, build policy owner, or
privileged surface.

## Technical Context

**Language/Version**: Go 1.27.0; browser JavaScript modules with Node.js 20.19+ tooling

**Primary Dependencies**: Existing pinned Wails v3.0.0-beta.13, ConnectRPC 1.20.0, protobuf
1.36.11, Buf 1.72.0, protoc-gen-es 2.13.0, and repository-pinned Task/GoReleaser; no new dependency

**Storage**: Existing version-1 session and player-configuration JSON files; no data migration

**Target Platform**: `darwin/arm64`, `windows/amd64`, `windows/arm64`, `linux/amd64`, and
`linux/arm64` portable archives

**Performance Goals**: No runtime performance change; release preflight must still reject an
invalid tag before any package job starts

**Constraints**: One exact `/v2` application identity, one canonical release `VERSION`,
deterministic generated outputs and platform metadata, stable wire/JSON behavior, independent tool
modules, unchanged five-archive inventory, and no rewrite of historical evidence

## Contract and Compatibility Design

### Source and release identity

- The root module is exactly `github.com/obalunenko/Fallout-Terminal/v2`.
- Every buildable application package imports through that identity; active source contains no
  fallback to the unsuffixed application module.
- `ValidateReleaseTag` accepts only the documented strict v2 stable/prerelease subset and reports
  whether an accepted candidate is a prerelease.
- The tag preflight remains the dependency of all five native package jobs, so invalid or non-v2
  candidates cannot enter the matrix.
- Root-repository validation compares the exact declared module identity rather than accepting a
  prefix or substring that could also match another major.
- An accepted raw tag such as `v2.0.0-rc.1` yields canonical `VERSION=2.0.0-rc.1`; preflight exposes
  that one normalized value to every package job rather than re-deriving independent versions per
  target.
- `internal/version` owns an unexported `development` default and a linker-set canonical value.
  `<executable> --version` accepts no additional arguments, writes only that value plus one newline
  to standard output, writes nothing to standard error, and exits successfully before Wails,
  listeners, storage, or tunnel services start.

### Protobuf and ConnectRPC

- All 13 schema files retain their existing protobuf `package` declarations and change only the
  `go_package` option to `/v2/internal/gen/...`.
- All messages, field numbers, enums, services, RPC cardinalities, procedure paths, validation,
  ordering, publication, and reconnect behavior remain byte- and behavior-compatible.
- Checked-in Go, ConnectRPC, and ECMAScript outputs are regenerated through pinned tools. The schema
  revision and descriptor baseline advance after review, and the five established breaking
  fixtures continue to fail.

### Wails bridge

- Clean generation produces one binding tree at
  `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/` and removes the prior
  application namespace.
- `frontend/overseer/src/desktop-api.js`, browser fixture import maps, tests, and validation scripts
  consume that exact path.
- The accepted surface remains one desktop service, 35 methods, six named events, and no generic
  dispatcher or filesystem/process/environment capability.

### Persistent JSON and runtime behavior

- Sessions and player configurations remain version 1. Session compatible unknown fields continue
  to round-trip recursively; unsupported player-configuration unknown fields continue to be
  rejected without rewriting the selected file.
- No default, field name, reference, save target, storage location, state transition, player route,
  public-access boundary, or gameplay behavior changes.

### Build, package, and documentation

- `cmd/build` and `internal/buildtool` remain the sole detailed release-policy owners beneath the
  root Task graph. GitHub Actions continues delegating tag validation and packaging to those seams.
- A non-empty package `VERSION` selects release mode and must be a strict canonical v2 semantic
  version without a leading `v`. An empty local `VERSION` selects non-release mode and embeds
  `development`; tagged workflow jobs always supply the preflight output and fail before upload if
  it is absent, malformed, non-release, or mismatched.
- Package planning adds the matching `-X` linker assignment for release mode, retains Go VCS build
  information in both modes, and renders target metadata from checked-in templates into
  target-owned staging paths without modifying the worktree.
- Darwin renders `build/darwin/Info.plist.tmpl` directly to
  `build/bin/darwin-arm64/stage/Fallout Terminal/Fallout Terminal.app/Contents/Info.plist`.
  Windows renders `build/windows/info.json.tmpl` and `build/windows/app.manifest.tmpl` to
  `build/bin/windows-<arch>/metadata/info.json` and
  `build/bin/windows-<arch>/metadata/app.manifest` before generating the target `.syso` resource.
- Darwin human-readable metadata and the executable report retain the canonical semantic version.
  Windows string metadata and the executable report do the same; numeric-only Darwin/Windows
  fields derive `MAJOR.MINOR.PATCH` or `MAJOR.MINOR.PATCH.0` as required by the owning format.
  Local non-release metadata uses human-readable `development` and numeric `0.0.0` or `0.0.0.0`.
- Package verification invokes the non-interactive report on each matching native runner and
  inspects applicable plist/PE/manifest values before the archive is uploaded. Missing, malformed,
  non-release, or mismatched values fail the target job.
- The five target pairs, stable archive names, required contents, create-only publication, and
  partial-release recovery procedure remain unchanged.
- README and platform-packaging guidance use `v2.0.0` and `v2.0.0-rc.1` as current examples and
  state that tag and root-module majors agree. Completed specifications and rollback evidence keep
  their historical paths and examples.

## Implementation Phases

### Phase 1: Root source identity

1. Change the root `go.mod` declaration to the exact `/v2` module path.
2. Update all buildable application and test imports, including root composition, internal
   packages, commands, and the browser fixture server.
3. Tighten the repository-root build guard and add an exact-identity regression fixture.
4. Verify active-source searches find no unsuffixed application import while excluding historical
   records and repository URLs that are not Go module identities.

### Phase 2: Generated contract identity

1. Update only protobuf `go_package` options and review the schema diff before baseline changes.
2. Regenerate Go, ConnectRPC, and ECMAScript outputs twice with the pinned tool modules.
3. Advance the schema revision, compatibility descriptor, and reviewed digests; rerun every
   established breaking fixture.
4. Clean-regenerate Wails bindings, update every active consumer/import map/check, and prove the old
   generated namespace is absent.

### Phase 3: Governed v2 release preflight

1. Add the explicit v2 major rule to the existing strict tag validator and preserve malformed,
   leading-zero, prerelease, and build-metadata behavior.
2. Extend CLI and build-tool table fixtures for accepted stable/prerelease v2 tags and rejected
   older/future majors.
3. Update active README and packaging guidance ~~without changing Task, workflow, target,
   inventory, or publisher ownership~~ without duplicating tag policy in Task or workflow and
   without changing target, inventory, or publisher ownership; BUG-001 requires the workflow to
   propagate the build tool's canonical version output.

### Phase 3A: BUG-001 tag-to-artifact version propagation

1. Extend release-tag parsing to return the canonical semantic version and export that one
   `VERSION` from preflight into every package matrix job.
2. Add the application version owner and exact `<executable> --version` report, inject the
   canonical value through Go linker flags, retain VCS build information, and prove runtime
   composition is not entered.
3. Rename checked-in platform metadata to `.tmpl` inputs and render target-specific Darwin and
   Windows outputs in isolated staging paths, including deterministic numeric-field derivation for
   prereleases and zero-valued numeric metadata for local `development` packages.
4. Extend target package and archive inspection to compare executable and applicable platform
   metadata with the triggering tag before upload, with stable, prerelease, missing, malformed, and
   mismatch fixtures.
5. Document empty-local-`VERSION` behavior, explicit `development` identity, strict release-mode
   validation, and the one-version release invariant.

### Phase 4: Compatibility and cutover verification

1. Run focused persistence round trips for sessions and player configurations and retain their
   respective unknown-field policies.
2. Run full Go, race, lint, generation, breaking, binding, frontend, browser, build, package,
   tool-isolation, and secret-leak checks.
3. Search active source and guidance for stale application identities or non-v2 release examples;
   preserve completed historical records.
4. Accept the cutover only when one source identity and one generated binding namespace remain.
5. Exercise the pre-publication rollback procedure against the immutable rollback reference and
   confirm no dual application or binding identity remains after either forward cutover or rollback.

## Verification Plan

| Surface | Check | Expected result |
|---|---|---|
| Release tag contract | `go test ./internal/buildtool ./cmd/build` | Stable/prerelease v2 fixtures pass; malformed and non-v2 majors fail before packaging. |
| Canonical version propagation | Focused workflow, CLI, build-plan, and linker-argument tests | One tag-derived `VERSION` reaches every matrix target; missing or malformed release values fail before compilation/upload. |
| Executable version report | Focused `internal/version` and application-entry tests plus each matching native package job | `<executable> --version` prints only the canonical stable/prerelease value plus one newline, writes no stderr, and exits without starting Wails or services. |
| Platform version metadata | Package-plan tests plus native plist and PE/manifest inspection | Checked-in templates remain unchanged; staged human-readable values equal the canonical version and numeric-only fields equal the deterministic platform representation. |
| Local non-release identity | Focused build-plan, metadata-rendering, and archive-inspection tests | Empty local `VERSION` embeds `development` with zero-valued numeric metadata and cannot pass inspection for an expected release version. |
| Root module and tool ownership | `scripts/tool-modules-check.sh` and focused build-tool tests | Exact root `/v2` identity is accepted; independent tool modules and root sums remain isolated. |
| Protobuf generation | `task proto:check` | Formatting/lint, two clean generations, drift checks, provenance, generated compilation, and client build pass. |
| Protobuf compatibility | `task proto:breaking` | The advanced baseline accepts the metadata-only schema set and all five breaking fixtures remain rejected. |
| Wails bindings | `task bindings:check` | Two generations match; exactly one `/v2` service tree, 35 methods, and six events remain. |
| Persistence | `go test ./internal/domain ./internal/session ./internal/playerconfig` | Version-1 content, session unknown fields, player-config rejection, and save/reopen behavior remain unchanged. |
| Go quality/runtime | `task fmt:check`, `task vet`, `task lint`, `task test`, and `task test:race` | Repository compiles and passes without identity or concurrency regressions. |
| Frontends and browser journeys | `task frontend:build` and `task browser:test` | Generated paths resolve and existing Overseer/player behavior passes unchanged. |
| Contract/cutover checks | `scripts/wails-v3-contract-check.sh`, `scripts/wails-v3-cutover-check.sh`, and `scripts/secret-leak-check.sh` | Reviewed hashes, single-runtime boundaries, and secret exclusions remain valid. |
| Native build/package | `task build` and `task package GOOS=darwin GOARCH=arm64` on the supported host | The v2-identity application builds, its binary/metadata versions agree, and the governed archive inventory/layout remains unchanged. |
| Active guidance | Focused repository search plus `go test ./internal/platform` | Current docs and executable fixtures use v2 examples; historical records remain untouched. |

## Rollback Plan

The immutable pre-cutover reference is
`3f2b6e584aee4c5279a3d54ae70aa44ee578a21a` (`develop` at feature-branch creation).

- **Before any complete v2 GitHub Release is published**: abandon or revert the feature change set
  to the immutable reference, regenerate protobuf and Wails outputs using that revision's pinned
  tools, and rerun its accepted build/package gates. Rollback restores the prior root module,
  generated namespaces, release validator, platform metadata, and executable behavior together; it
  never ships both identities in one revision.
- **After a complete v2 GitHub Release is published**: preserve the published tag, release, and
  archives. Correct defects forward on a new strict v2 patch or prerelease tag; do not delete,
  replace, append to, or reuse the published release identity.
- **Partial publication failure**: retain the existing documented manual recovery procedure for an
  incomplete release. It does not authorize replacing a complete published release.
- **Removal gate**: forward cutover or pre-publication rollback is accepted only when active-source
  scans, deterministic generation, and binding checks prove exactly one root application identity
  and one generated Wails namespace remain.

## Known Bugfix Boundary

~~[BUG-001](./bugs/BUG-001.md) identifies a conflict between the current scope exclusion and the
desired invariant that platform metadata and packaged executables carry the triggering release
version. This plan deliberately does not design or schedule that work until
`$speckit-bugfix-patch` updates the governing spec and this plan; the existing platform metadata,
linker flags, and minimal archive-eligibility contract otherwise remain unchanged.~~

BUG-001 is now patched into the governing artifacts. The implementation must complete Phase 3A and
the added verification rows before the version-identity bug can be considered fixed; the supported
target set and five-archive publication inventory remain unchanged.
