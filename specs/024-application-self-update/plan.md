# Implementation Plan: Application Self-Update

**Branch**: `024-application-self-update` | **Date**: 2026-08-27 | **Spec**: [spec.md](./spec.md)

**Bugfix**: 2026-08-29 — BUG-001 updated from bugfix patch.

**Input**: Feature specification from `specs/024-application-self-update/spec.md`

## Summary

Add one nonblocking update check to each packaged release launch, expose its state through the
existing private Overseer bridge, and require separate accept and restart decisions before any
download or replacement. Reuse the pinned Wails v3 updater for GitHub discovery, transfer,
archive extraction, and digest verification, while an application-owned manager and helper stage
and replace the complete portable application unit so the existing five archives remain usable on
Windows, Linux, and macOS. Development builds and every failed external operation remain local-use
safe, and sessions, player configurations, credentials, and preferences stay outside the
replacement boundary.

## Project Structure

```text
.
├── main.go                                  # Helper-mode dispatch and update composition
├── app.go                                   # Launch-scoped update owner and ordered shutdown handoff
├── app_contract.go                          # Private protobuf-to-native update adapters
├── desktop_service.go                       # Three explicit Overseer update methods
├── wails_host.go                            # Typed update event and Wails updater adapter wiring
├── wails_updater.go                         # Headless Wails updater and validated GitHub provider wrapper
├── internal
│   ├── update
│   │   ├── model.go                         # State, candidate, snapshot, and failure vocabulary
│   │   ├── manager.go                       # One-check state machine and two-decision workflow
│   │   ├── staging.go                       # Manifest validation and same-volume package staging
│   │   ├── helper.go                        # Backup, replacement, recovery journal, and relaunch
│   │   ├── helper_unix.go                   # Unix process/unit replacement adapter
│   │   ├── helper_windows.go                # Windows process/unit replacement adapter
│   │   └── *_test.go                        # Deterministic lifecycle, failure, and race coverage
│   ├── buildtool
│   │   ├── archive.go                       # Versioned artifact-manifest schema v2
│   │   ├── releasecheck.go                  # Manifest compatibility and release-identity gate
│   │   └── *_test.go                        # Five-target manifest/package regression coverage
│   └── platform
│       ├── paths.go                         # Non-user update journal location
│       └── paths_test.go                    # Platform path and isolation coverage
├── proto/fallout/terminal/private/v1
│   └── update.proto                         # Private update snapshot and decision messages
├── internal/gen/fallout/terminal/private/v1 # Regenerated Go protobuf output
├── proto/{schema-revision.txt,compatibility-baseline.binpb}
├── frontend/overseer
│   ├── bindings/                            # Regenerated Wails bindings; never hand-edited
│   └── src
│       ├── desktop-api.js                    # Normalized update snapshot/event facade
│       ├── index.html                        # Global status, offer, and restart surfaces
│       ├── overseer.js                       # Accessible state rendering and decision handling
│       └── overseer.css                      # Fallout-styled responsive update presentation
├── tests/browser
│   ├── application-update.spec.mjs          # Consent, progress, failure, and accessibility journeys
│   ├── desktop-api.spec.mjs                 # Private bridge inventory and race contracts
│   └── fixtures/desktop-bindings.js         # Deterministic update backend/event fake
├── scripts/wails-bindings-check.sh          # Updated method/event allowlist and drift gate
├── internal/platform/{assets_test.go,portable_release_test.go,startup_test.go}
├── docs/platform-packaging.md               # Update eligibility, recovery, and portable layout
└── README.md                                # Overseer self-update behavior and limitations
```

**Structure Decision**: Keep Wails and GitHub provider types in root composition, place the
transport-independent state machine and package replacement mechanics in `internal/update`, extend
the existing private protobuf/Wails bridge, and strengthen the current build-tool manifest rather
than creating a second release format, publisher, updater backend, or persistence owner.

## Constitution Check

| Principle | Assessment | Evidence |
|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | The exactly pinned Wails v3 updater remains a root composition adapter; the application manager is Wails-independent, uses one typed event, and hands restart to the existing ordered host shutdown. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | Update snapshots and both decisions are added to the private v1 protobuf graph before native DTOs and generated bindings; artifact manifests remain native build-tool metadata. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | No public RPC or player state changes; the Go update manager is the sole authority and the browser only renders snapshots and submits decisions. |
| IV. Separate Public and Private Capabilities | PASS | Update operations exist only on the allowlisted desktop service and `application-update-status`; the player client receives no updater API, path, release metadata, or privileged action. |
| V. Evolve Schemas Safely and Reproducibly | PASS | New private messages use new field numbers, generated outputs and the reviewed baseline advance deterministically, and no existing field is renamed or reused. |
| VI. Preserve Portable Session JSON Version 1 | PASS | Session and player-configuration schemas, storage paths, adapters, fixtures, and business content remain unchanged and outside the staged application unit. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | The feature adds one accepted update path with no legacy updater, compatibility switch, duplicate bridge, or permanent dual release format. |
| Dependency Rules | PASS | No new runtime or tool dependency is added: root composition uses the already pinned Wails updater and the application-owned manager uses the standard library. |
| Secret and Credential Governance | PASS | Public GitHub releases require no token; snapshots, logs, events, helper arguments, and the recovery journal exclude credentials, URLs with authorization, user documents, and raw provider payloads. |
| Go Development Tool Modules | PASS | Task remains the sole workflow graph; pinned Wails, Buf, protobuf, Connect, lint, and GoReleaser modules retain their owners and Make gains no alias. |
| Testing and Quality Gates | PASS | The plan adds race, lifecycle, failure, generated-contract, browser, five-target manifest, package, and secret-leak coverage while preserving exactly five release assets and the allowed tag gate. |
| Development Workflow and Governance | PASS | The plan identifies producers, consumers, state owner, replacement boundary, generated artifacts, recovery path, package gate, rollback, and cutover without an exception. |

No constitution violations require Complexity Tracking.

## Implementation Approach

1. Add the private update contract and regenerate the Go protobuf graph and Wails bindings. The
   contract exposes one revisioned snapshot, an offer decision, and a restart decision; it never
   exposes filesystem paths, provider URLs, digests, helper state, or raw errors.
2. Build a launch-scoped `internal/update.Manager`. The first update-status handshake after the
   Overseer subscribes starts one bounded check through `sync.Once`; development or unversioned
   builds return `disabled` without network access. Attempt identifiers and snapshot revisions
   reject stale, duplicate, and concurrent decisions.
3. Configure `app.Updater` with `WindowNone` and no interval. A root GitHub-provider wrapper selects
   the eligible stable/prerelease release, requires the exact five governed assets, rejects zero or
   multiple target matches, reads the selected asset's GitHub SHA-256 `digest`, and supplies that
   digest to Wails verification. `Check` runs before consent; `DownloadAndInstall` runs only after
   accept.
4. After Wails verifies and extracts the archive, validate `artifact-manifest.json` schema v2,
   product, version, target, file inventory, and package shape. Copy the complete replacement unit
   to a same-volume sibling stage: the portable directory on Windows/Linux and the nested `.app`
   bundle on macOS. Do not publish `ready-to-restart` until this stage is durable and writable.
5. On restart approval, copy the current executable to a temporary helper location, write a
   non-sensitive applying journal, launch helper mode, and call the Wails host's normal `Quit`.
   The helper waits for process exit, renames the old unit to a backup, promotes the staged unit,
   relaunches it, and restores/relaunches the backup on replacement or launch failure. The next
   launch consumes the journal so restored failures are visible and successful same-version
   releases are not offered again.
6. Add a persistent global update status surface plus separate accessible offer and restart
   dialogs. Accept closes the offer while preparation continues without blocking the main UI;
   postponing keeps the staged state available during the run. Event-before-snapshot reconciliation
   and monotonically increasing revisions prevent startup and command-result races.
7. Advance artifact-manifest generation and matching-native inspection so every governed archive
   proves its embedded manifest version and canonical release version before upload. Keep the
   GitHub Release inventory at exactly five archives: GitHub supplies each asset digest, and pinned
   GoReleaser uploads to a draft before making the complete release discoverable.
8. Preserve the exact schema-v2 payload inventory compiled into v2.0.0 for the forward-fix archive.
   Treat user-facing launch documentation as release-page or repository documentation until an
   intentional package-schema migration can be consumed by all older releases that may discover it.
   Before publication, validate candidate archives with the oldest supported published updater
   contract instead of only the candidate revision's validator. Document any already-published
   conflicting strict inventory as requiring one manual replacement.
9. Build the update offer's release notes from every eligible published version above the installed
   version through the selected candidate. Sort groups by semantic-version precedence from newest
   to oldest, retain version headings for empty notes, and continue projecting the result through
   the existing bounded plain-text update field and dialog.

## Verification Strategy

- Run focused `internal/update` tests under `go test -race`, including concurrent check triggers,
  duplicate decisions, cancellation, partial downloads, digest failure, structural rejection,
  same-volume staging, helper backup/restore, relaunch failure, journal recovery, and cleanup.
- Extend build-tool tests across all five targets for manifest schema v2, canonical version,
  malformed or mismatched target/version, deterministic archives, exact inventory, and canceled
  inspection. Every test-owned resource registers `t.Cleanup` immediately; blocking cleanup uses a
  bounded context derived from `context.WithoutCancel(t.Context())`.
- Add a predecessor-consumer regression gate that feeds each proposed target archive to the exact
  inventory contract of the oldest supported published updater. On the current native host, retain
  one real oldest-supported-to-candidate update/relaunch as publication evidence, and verify that
  release guidance names any incompatible intermediate cohort honestly.
- Regenerate and verify protobufs and Wails bindings through the pinned Task graph; run Buf format,
  lint, breaking, generation drift, generated-code compilation, private-surface allowlists, and the
  secret-leak check.
- Extend desktop service, application lifecycle, Wails host, static asset, startup, and portable
  release contracts. Confirm update failure cannot change `startupError`, disable local session
  controls, expose a player capability, or mutate session/player-configuration storage.
- Run the focused Playwright updater journey and the complete browser suite for event/snapshot
  ordering, versions and notes, consent separation, nonblocking progress, failure actions,
  postpone/reopen, restart exactly once, stale suppression, Escape handling, focus, live regions,
  and double-click protection.
- Add provider and browser coverage for unordered multi-release input, stable/prerelease channel
  filtering, empty notes, provider pagination, explicit version headings, and descending cumulative
  changelog order.
- Run `task check`, `task ci:quality`, `task package` on the current Darwin ARM64 host, and the
  governed tagged five-target matrix for final package evidence. The minimum live acceptance is one
  real GitHub prerelease update/relaunch journey on Darwin ARM64; Windows/Linux runtime journeys and
  manual failure injection are not required beyond the deterministic automated coverage. Report the
  Darwin journey as `NOT RUN` when a spare tag is unavailable.

## Cutover and Rollback

The cutover is additive until generated bindings, UI, package-manifest v2, and the provider/helper
gates pass together; no release containing the feature is eligible while any part is missing. Before
the first published self-update-capable release, rollback is a normal source revert that restores
the prior private surface and manifest schema. After publication, never move or replace a tag or
mutate its assets: ship a higher strict v2 forward-fix release. A failed on-device application
retains or restores the previous unit from its sibling backup, records a safe recovery result, and
relaunches that unit; user-owned data is never part of either unit.

## Post-Design Constitution Re-check

| Gate | Result | Final design evidence |
|---|---|---|
| Contract and generation | PASS | `update.proto` owns every structured private payload; generated Go and Wails artifacts remain tool-owned and drift-checked. |
| Public/private and secret boundary | PASS | Three allowlisted desktop methods and one named event expose only sanitized revisioned state; no player API, generic dispatcher, provider URL, path, digest, token, or user content crosses the bridge. |
| Runtime and lifecycle | PASS | Wails stays in root composition; `internal/update` is framework-independent; helper promotion waits for the existing ordered shutdown and restores the last working unit on failure. |
| Persistence compatibility | PASS | Session/player-configuration formats and stores are untouched; the recovery journal is separate application metadata and never enters the replacement unit or version-1 JSON. |
| Dependency and workflow ownership | PASS | No dependency, tool module, Make alias, release publisher, or asset class is added; Task and pinned GoReleaser retain ownership. |
| Release and package gates | PASS | Manifest v2 strengthens canonical identity, the provider requires exactly five uploaded assets and GitHub SHA-256 evidence, and no prohibited sidecar or native/signing journey gates publication. |
| Cutover and rollback | PASS | There is one update path, no coexistence switch, source rollback is allowed before publication, published releases use forward fixes, and device failure restores the backup. |

The original design introduced no constitution violation or Complexity Tracking entry. BUG-001
adds the release-compatibility edge case below without changing an architectural boundary.

## Complexity Tracking

| Edge case | Required handling |
|---|---|
| A same-schema release adds or removes a governed payload path | Reject publication unless the candidate remains accepted by the oldest supported updater; use a new schema only with an explicit migration strategy, and document manual recovery for any already-published conflicting inventory. |
