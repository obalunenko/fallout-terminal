# Implementation Plan: Vue and TypeScript Frontend Migration

**Branch**: `033-frontend-vue-typescript-migration` | **Date**: 2026-08-30 | **Spec**: [spec.md](./spec.md)

**Authoritative inputs**: the clarified feature specification and Fallout Terminal Constitution 9.0.0

**Immutable pre-migration rollback commit**: `06696ee1c7155a1bb1135ef46ec91445dd73a2a4`

## Summary

Migrate the privileged Overseer and public Player interfaces incrementally from handwritten browser JavaScript to two independent Vue 3 applications made of strict TypeScript Single-File Components using the Composition API and `<script setup lang="ts">`. Preserve the existing product, protocol, security, persistence, visual, interaction, build, embedding, and package behavior while introducing one governed frontend workspace install, deterministic generated TypeScript browser contracts, typed application boundaries, explicit Vue DOM ownership, and lifecycle-safe browser integrations. Complete and prove the Overseer cutover before beginning the Player production cutover, then remove every temporary compatibility path and legacy production bootstrap.

## Technical Context

The repository remains a Go 1.27 modular desktop application with Wails `3.0.0-beta.15`, Vite `8.1.5`, Node.js `26.8.1`, ConnectRPC `2.1.2`, `@bufbuild/protobuf` and `@bufbuild/protoc-gen-es` `2.13.0`, and Playwright `1.62.1`. Phase 0 selects the exact compatible Vue, TypeScript, Vue Vite plugin, and `vue-tsc` pins without changing any of those existing pins or any unrelated dependency.

No persistent or canonical runtime model changes are planned. Protobuf schemas, Go-generated contracts, Wails methods and named events, ConnectRPC methods and cardinalities, version-1 session JSON, player configuration, authorization, request limits, state authority, revision rules, reconnect behavior, backpressure, cancellation, unary fallback, CSP, and public/private capabilities remain unchanged.

## Constitution Check

This table records both the mandatory pre-research gate and the completed post-design re-check.

| Constitution principle | Pre-research gate | Post-design re-check | Planning evidence |
|---|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | PASS | Wails remains `3.0.0-beta.15`; separate privileged/public roots, entrypoints, bundles, embeds, adapters, and state ownership are explicit; authority and domain behavior remain in existing Go owners. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | PASS | Browser generation representation alone changes to `target=ts`; schemas and Go generation stay unchanged; Wails bindings remain generator-owned behind one unknown-validating typed adapter. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | PASS | The design enumerates all eight Player RPCs and preserves paths/cardinalities, binary encoding, typed results, snapshot-first subscriptions, revisions, cancellation, streaming backpressure, and unary fallback. |
| IV. Separate Public and Private Capabilities | PASS | PASS | Player has its own config/root/bundle/adapter and dependency scans reject Overseer, Wails, native, private, and privileged paths; browser/native evidence is separated. |
| V. Evolve Schemas Safely and Reproducibly | PASS | PASS | Exact `protoc-gen-es` `2.13.0` emits five deterministic checked-in TypeScript files with provenance, strict compilation, deliberate drift rejection, and no second output. |
| VI. Preserve Portable Session JSON Version 1 | PASS | PASS | No data-model or persistence change exists; existing adapters/Go services keep semantic validation, compatibility, defaults, unknown-field behavior, and save meaning. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | PASS | Nine ordered waves have explicit mount/legacy ownership, prohibited crossings, parity gates, owners/expiries, atomic cutovers, rollback commit, and final removal scans. |

No constitutional violation is required, so no Complexity Tracking table is needed.

## Project Structure

The migration changes frontend source and build governance while preserving the existing Go application and contract ownership.

```text
specs/033-frontend-vue-typescript-migration/
├── spec.md
├── plan.md
├── research.md
├── quickstart.md
├── migration-ownership.md
└── contracts/
    ├── frontend-build-generation.md
    └── trust-and-evidence-boundaries.md

frontend/
├── package.json                       # sole workspace scripts/install boundary
├── package-lock.json                  # sole frontend lockfile
├── tsconfig.base.json                 # shared final strict settings
├── client/
│   ├── package.json                   # independent Player type-check/build commands
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── index.html
│   ├── client.css                     # immutable global presentation baseline
│   ├── fonts/ and sounds/             # immutable Player static assets
│   ├── gen/                           # checked-in protoc-gen-es TypeScript
│   └── src/
│       ├── main.ts and App.vue
│       ├── components/
│       ├── composables/
│       ├── adapters/
│       ├── directives/
│       └── env.d.ts
└── overseer/
    ├── package.json                   # independent Overseer type-check/build commands
    ├── tsconfig.json
    ├── vite.config.ts
    ├── bindings/                      # unedited Wails JS/JSDoc/declarations
    ├── dist/.keep                     # clean-checkout embed marker
    └── src/
        ├── index.html
        ├── main.ts and App.vue
        ├── overseer.css and Fixedsys.ttf
        ├── components/
        ├── composables/
        ├── adapters/
        ├── directives/
        └── env.d.ts

proto/buf.gen.es.yaml                  # target=ts, import_extension=js
scripts/                               # generation, drift, binding, boundary, package checks
tests/browser/                         # unchanged .mjs journeys/selectors/snapshots
internal/buildtool/                    # canonical build/type-check/package ordering
internal/platform/                     # source/build/embed/startup contract checks
internal/player/                       # unchanged public HTTP/ConnectRPC and asset owner
main.go                                # separate Overseer and Player embeds remain
Taskfile.yml                           # sole workflow graph
.github/workflows/                     # Taskfile-driven quality/package workflows
README.md, ARCHITECTURE.md, CONTRIBUTING.md, docs/
.specify/templates/                    # active Vue/TypeScript planning/task guidance
```

**Structure Decision**: Keep all application code inside its existing trust boundary, share only strict compiler policy and the workspace install/lock graph, and represent cross-boundary behavior through generated contracts and narrow typed adapters rather than a shared application state layer.

## Architecture and Trust Boundaries

The migration changes presentation ownership, not product authority. Overseer Vue state is an application view over validated private Wails commands/events; Player Vue state is an application view over validated public ConnectRPC snapshots, updates, and action results. Existing Go services retain semantic validation, authorization, persistence, revision ordering, public-access isolation, update policy, and canonical mutation ownership.

The two applications share only the npm installation/lock boundary and strict compiler policy. They do not share an application root, runtime entrypoint, bundle, state store, adapter, transport, privileged type, or mutable composable. Player's dependency graph is checked for direct and transitive paths to Overseer, Wails, `@wailsio/runtime`, generated Wails bindings, filesystem/native capabilities, private protobuf packages, or privileged state.

Generated browser Protobuf TypeScript remains a public transport boundary. Generated Wails JavaScript/JSDoc/declarations remain a private framework boundary. Neither generated representation becomes a canonical mutable model. Application adapters convert untrusted external values into narrow trusted TypeScript projections and preserve all existing semantic checks.

## Component and Composable Map

All production components use `<script setup lang="ts">`, typed properties and emits, and the Composition API. Roots hold only orchestration state with `ref`, `reactive`, and `computed`; focused composables own workflows and lifecycles. No Pinia, Vue Router, Nuxt, JSX, component library, CSS framework, or cross-boundary store is introduced.

### Overseer application

| Required area | Vue component ownership | Composable/adapter ownership |
|---|---|---|
| Application shell and runtime status | `App.vue`, `StartScreen.vue`, `OverseerLayout.vue`, `RuntimeHeader.vue` | `useDesktopRuntime`, `useRuntimeStatus` |
| Desktop API and named-event adapter | No component imports Wails directly | `adapters/desktop-api.ts`, `adapters/wails-service.d.ts`; exact method/event inventory check |
| Session and player management | `SessionControls.vue`, `PlayerConfigControls.vue`, `LogicalSessionDialog.vue`, `LogicalSessionRow.vue`, `PlayerManagementDialog.vue`, `PlayerProfileRow.vue`, `PlayerDeleteDialog.vue` | `useSessionDocument`, `usePlayerConfiguration`, `useLogicalSessions`, `usePlayerManagement` |
| Terminal list, selection, groups, and authoring | `TerminalSidebar.vue`, `TerminalGroupList.vue`, `TerminalGroupRow.vue`, `TerminalRow.vue`, `TerminalActionMenu.vue`, `TerminalEditor.vue`, `TerminalSettings.vue`, `TerminalTree.vue`, recursive `TerminalTreeNode.vue`, `NodeEditor.vue`, `CreateTerminalDialog.vue`, `TerminalGroupDraftDialog.vue`, `TerminalGroupImpactDialog.vue` | `useTerminalSelection`, `useTerminalGroups`, `useTerminalAuthoring` |
| Command and navigation approvals | `CommandExecutionDialog.vue`, `TerminalNavigationDialog.vue`, `TerminalSwitchDialog.vue`, `CommandStateResetDialog.vue` | `useCommandApproval`, `useTerminalNavigationApproval`, `useTerminalSwitch`; preserve request IDs, epochs, resolved-ID sets, revision gates, and idempotency |
| Broadcast and hacking controls | `BroadcastControls.vue`, `TakeOffAirDialog.vue`, `EndBroadcastDialog.vue`, `HackControlPanel.vue` | `useBroadcastControls`, `useHackControls` |
| Public-access lifecycle and credentials | `PublicAccessPanel.vue`, `PublicAccessSettingsDialog.vue`, `ProviderTokenDialog.vue`, `PlayerCredentialsDialog.vue`, `GeneratedPasswordDialog.vue` | `usePublicAccess`, `useClipboard`; preserve tuple ordering, fail-closed states, one-time secret lifetime, clearing, and clipboard failures |
| Application-update presentation | `ApplicationUpdateStatus.vue`, `ApplicationUpdateOfferDialog.vue`, `ApplicationUpdateRestartDialog.vue` | `useApplicationUpdate`; preserve cumulative revisions, attempt de-duplication, bounded text, deferral/restart, and failure isolation |
| Dialogs, focus restoration, and clipboard | Every dialog above owns its semantic markup and typed emits | `useDialogFocus` or `v-dialog-focus`, `useClipboard`; opener capture, connected-element restoration, initial focus, Escape/cancel, transient-status timer cleanup |

Existing global `overseer.css`, Fixedsys font, IDs, classes, `data-*` attributes, Russian copy, roles, labels, live regions, `hidden`/`open` behavior, and focus order remain unchanged. Components use stable entity keys so Vue reconciliation does not remount observable rows or dialogs unnecessarily.

### Player application

| Required area | Vue component ownership | Composable/adapter ownership |
|---|---|---|
| Application shell and connection overlay | `App.vue`, `CrtShell.vue`, `TerminalChrome.vue`, `CrtEffects.vue`, `ConnectionOverlay.vue`, `PlayerNotice.vue` | `usePlayerProjection`, `useConnectionOverlay` |
| ConnectRPC transport and subscription lifecycle | Components consume typed state only | `adapters/player-rpc.ts`, `usePlayerSubscription`; snapshot-first stream, generation, AbortController, fixed reconnect, stale suppression |
| Recognition, multi-tab lease, and session initialization | No storage access from components | `adapters/recognition-storage.ts`, `useRecognitionLease`; Web Locks first, storage contender/lease fallback, exact keys/timings, one logical session and one stream per tab |
| Player identity, roster, role, and controller state | `CharacterSelection.vue`, `CharacterOption.vue`, `AssignedWaiting.vue`, `PlayerStatusLine.vue` | `usePlayerIdentity`, `usePlayerAuthority`; roster validation, opaque recognition, observer read-only state |
| Terminal list, entry, navigation, pagination, and command output | `TerminalSurface.vue`, `TerminalMenu.vue`, `TerminalMenuRow.vue`, `TerminalRecord.vue`, `TerminalFooter.vue`, `PaginationControls.vue` | `usePlayerActions`, `useTerminalNavigation`, `usePaginationMeasurement`; ActionResult correlation and authoritative revision completion |
| Hacking board, log, selection, pointer, and keyboard input | `HackingSurface.vue`, `HackingAttempts.vue`, `HackingBoard.vue`, `HackingColumn.vue`, `HackingRow.vue`, `HackingCell.vue`, `HackingLog.vue`, `HackingInputPreview.vue`, `HackingBlocked.vue` | `useHackingSession`, `useHackingBoardFit`, `useHackingPointer`, `useTerminalKeyboard`; exact target grouping, focus restoration, fitting modes, attempts/outcomes |
| CRT/typewriter presentation | `CrtShell.vue`, `CrtEffects.vue`, terminal/hacking text components | `useTypewriterReveal`, `usePaginationMeasurement`; 40ms clock-based reveal, identity suppression, repeat consumption, cue ordering, cancel/complete |
| Sound and gesture activation | Components emit semantic cue requests only | `adapters/sound-manifest.ts`, `useTerminalSound`; exact categories/volumes, safe same-origin assets, gesture unlock, cue de-duplication, ambient reconciliation |
| Presentation uplink, streaming capability, cancellation, and unary fallback | Components emit typed transient presentation intent | `adapters/presentation-uplink-transport.ts`, `usePresentationQueue`, `usePresentationUplink`; secure-context probe, open/ready/result correlation, one-slot latest mailbox, cancellation, retry, unary fallback |

Existing global `client.css`, font and sound locations, all IDs/classes/data attributes, semantic elements, accessible names/live regions, focus behavior, hacking geometry, visual snapshots, and test observers remain parity contracts. Vue templates use text bindings rather than unsafe HTML for authored/external content.

## Imperative Browser Ownership and Cleanup

Imperative browser behavior is restricted to the seven approved seams: Web Audio, focus, element measurement, CRT/typewriter timing, hacking-board fitting, pointer geometry, and presentation streaming. The owning composable or directive acquires resources only after its Vue scope exists and immediately registers cleanup:

- timers and animation frames are cleared/cancelled on replacement, context invalidation, and unmount;
- Connect and Wails subscriptions retain exact unsubscribe/cancel handles and release them once;
- audio fetches are aborted, nodes stopped/disconnected, ambient media paused/cleared, and the `AudioContext` closed;
- AbortControllers abort replaced subscriptions, actions, sound loads, and presentation streams;
- stream iterators are cancelled/returned, mailboxes closed, and late results rejected by generation/context;
- `ResizeObserver` instances disconnect, font-ready callbacks check disposed state, and temporary fit probes are always removed;
- document/window/storage/pointer/keyboard/resize listeners are removed by the scope that installed them;
- focus restoration runs only for a still-connected valid opener and queued focus work is cancelled on unmount.

No approved imperative seam may independently render or mutate Vue-owned descendants. Exact resource contracts are specified in [trust-and-evidence-boundaries.md](./contracts/trust-and-evidence-boundaries.md).

## Producer and Consumer Impact Map

| Affected producer | Existing and planned consumers | Planning action |
|---|---|---|
| `frontend/package.json`, both app manifests, `frontend/package-lock.json` | npm workspace resolution, Task `deps:frontend`, buildtool prepare, CI caches, dependency/license checks, README | Add only the four researched exact pins and type-check scripts; retain one install/lock workflow and unchanged unrelated pins. |
| `frontend/tsconfig.base.json`, app configs, Vite configs | `vue-tsc`, SFCs, generated Player TypeScript, both Vite builds, editors/CI | Establish shared strict policy, app-specific include/trust boundaries, Vue plugins, unchanged roots/outputs/assets. |
| Both HTML documents and Vue entrypoints | Vite, Playwright fixtures/selectors, CSP, Wails/public asset serving, Go embeds | Replace bootstraps only at owned cutovers; preserve semantics/copy/CSP and one final root per application. |
| `overseer.css`, `client.css`, fonts, sounds, static assets | Vue templates, Vite manifests, SoundManifest service, snapshots, resource/package checks | Keep paths/content/global selectors stable; no scoped rewrite or intentional baseline update. |
| `proto/buf.gen.es.yaml` and public schemas | `scripts/proto-*`, checked-in `frontend/client/gen`, Player imports, buildtool preflight, native package probes, CI | Change only `target=ts`; retain `.js` specifiers, exact pins, schema/Go output; update deterministic inventories and direct-JS probe consumers. |
| Wails binding producer and generated tree | typed desktop adapter, Wails Vite plugin, browser fake port, integrity scripts, buildtool, native checks | Keep output unedited; add strict unknown-validating alias/adapter and synchronized exact method/event checks. |
| `Taskfile.yml` | maintainers, buildtool entrypoints, quality CI, portable release matrix | Add focused type-check/forbidden/reproducibility tasks while keeping Task the sole workflow graph and buildtool the detailed policy owner. |
| `internal/buildtool` and tests | `task prepare/build/package/dev`, all supported package plans | Insert strict app checks in protobuf → Player → bindings → Overseer order; preserve target/package behavior and standard-library-only ownership. |
| `.github/workflows/wails-cross-platform.yml`, `wails-portable.yml` | PR/main quality and five matching-host release jobs | Continue Task entry and one lockfile cache; enforce new gates through Task/buildtool instead of ad hoc npm workflow commands. |
| `tests/browser/playwright.config.mjs`, `fixture-server/main.go`, fixture bindings | all existing `.mjs` journeys and snapshots | Build production Player and production Overseer SFCs; inject only typed fake desktop transport for browser Overseer evidence; preserve selectors/baselines. |
| `main.go`, `wails_host.go`, `internal/player`, production resource tests | separate Wails and public static filesystems, startup and HTTP/Connect routes | Preserve separate embeds/routes/CSP/limits; update asset expectations only for new bundle manifests, not behavior. |
| `internal/platform/assets_test.go`, startup/release tests | raw source/build/selector/boundary assertions | Replace legacy filename/string coupling with strict SFC, typed adapter, built asset, selector, and boundary assertions without weakening intent. |
| generation/binding/secret/cutover/reproducibility/native/package scripts | Task/CI, native smoke, supported package inspection | Update `.ts` inventories, Vue source paths, final exclusions, production bundle probes, and actionable failure output. |
| README, `ARCHITECTURE.md`, `CONTRIBUTING.md`, active `docs/` | developers/operators/release maintainers | Document one install, two Vue apps, strict checks, generated TypeScript, typed Wails adapter, browser/native evidence separation. |
| Constitution 9.0.0 and `.specify/templates/*` | future specifications/plans/tasks | Keep constitution authoritative; update active templates that still name legacy JavaScript paths. Historical completed specs/evidence remain untouched. |

The explicit command, generation, output, and producer-consumer contract is [frontend-build-generation.md](./contracts/frontend-build-generation.md).

## Implementation Waves

Every wave uses the corresponding complete DOM record in [migration-ownership.md](./migration-ownership.md). Every intermediate revision must build and pass its applicable checks.

### Wave a — Vue/TypeScript infrastructure and bounded temporary legacy checking

Add exact dependencies, the shared/app TypeScript configurations, Vue Vite plugins, workspace/app type-check commands, fixture compiler support, exact-pin/lock checks, and narrowly enumerated temporary legacy checking. Keep both production DOMs and bootstraps unchanged. Establish the rollback commit and temporary-mechanism register before component work.

**Exit**: clean workspace install, exact dependency graph, strict candidate SFC checks, both unchanged production builds, and existing browser/visual baselines pass. Temporary checking has owners and wave-e/wave-h expiries.

### Wave b — Deterministic protobuf `target=ts` generation

Switch only `proto/buf.gen.es.yaml` to `target=ts`, preserve `import_extension=js`, regenerate rather than edit, and update provenance, imports, drift tests, strict compilation, native probe consumption, scripts, buildtool, CI, and clean-checkout inventories. Confirm descriptors, Go generation, schemas, RPCs, wire behavior, and capability boundaries are unchanged.

**Exit**: exactly five deterministic checked-in `_pb.ts` files, no old generated JS, actionable deliberate-drift rejection, two identical generations, strict Player compilation/build, and all protobuf/Connect/public-private gates pass.

### Wave c — Shared declarations, both Vue shells, and typed desktop API

Create both Vue roots and entrypoints in candidate/fixture documents, shared capability-neutral declarations, application-specific environment declarations, and the typed Wails alias/desktop adapter. Establish the production-code/bootstrap injection seam for the Overseer browser fixture. Add Player transport/projection interfaces without changing production ownership.

**Exit**: independent and workspace strict checks; both candidate shells build; desktop boundary malformed-value, ordering, subscription, clipboard, and method/event integrity tests pass; Player graph has no privileged dependency.

### Wave d — Overseer leaf components and composables

Migrate complete Overseer leaf families in reviewable slices: application-update presentation; approvals and confirmation dialogs; session/player management; terminal groups; public access and credentials; then remaining leaf controls. Each slice moves complete DOM/handlers/focus/pending state into Vue and removes its legacy counterpart immediately.

**Exit**: every moved subtree has one Vue owner, no legacy query/handler, unchanged focused selectors/semantics/copy/focus/visuals, strict build, and relevant browser tests. Temporary leaf root/state callbacks remain explicitly owned and expire in wave e.

### Wave e — Complete Overseer cutover and remove its legacy bootstrap

Move the shell/runtime status, terminal/group list, selection, authoring tree/forms, coordination, broadcast, hacking controls, public access, updates, and all dialogs into one `#overseerApp`. Remove `overseer.js`, `desktop-api.js`, dynamic renderers, global facade, temporary islands/bridges/mounts, legacy script tags, and Overseer `allowJs`/`checkJs` configuration.

**Exit**: complete Overseer strict/build/browser/visual parity and separate Wails/native/embed/resource/package gates pass; the Overseer forbidden-state and ownership inventories are empty. Player wave f cannot begin earlier.

### Wave f — Player shell, identity, transport, session initialization, navigation, and presentation foundations

Build the complete candidate shell, connection overlay, recognition/lease coordination, snapshot-first subscription/reconnect, player identity/roster/role/controller state, terminal list/entry/navigation/pagination/command output, action correlation, and transient presentation foundations in a separate candidate document. Production Player remains wholly legacy-owned.

**Exit**: candidate focused type/build/browser checks prove first connection, multi-tab convergence, reconnect, revisions, authority, navigation, pending actions, and accessibility; production legacy suite still passes; all acquired lifecycle resources have cleanup.

### Wave g — Player hacking, CRT/typewriter, sound, and presentation-uplink integrations

Complete candidate hacking components, target geometry/fitting/focus/input, CRT/typewriter timing, pagination measurement, sound/gesture activation, cue de-duplication, presentation streaming capability/probe/mailbox/cancellation/retry, and unary fallback. Keep all imperative work in the named Vue-owned composables/adapters/directives.

**Exit**: complete candidate Player `.mjs` and immutable visual suite passes with exact timing/geometry/audio/stream behavior and cleanup evidence; production legacy suite remains green; no baseline is intentionally updated.

### Wave h — Complete Player cutover

Atomically transfer the production document to the single `#playerApp` root. Remove `client.js`, `sound.js`, `presentation-uplink.js`, old script tag, candidate selection, staging entry/mount, legacy handlers/resources, and Player `allowJs`/`checkJs` configuration.

**Exit**: independent/workspace strict checks, Player build, complete browser/visual suite, public dependency scan, reconnect/multi-tab/authority/hacking/timing/sound/uplink stress checks, and separate native Player serving/package checks pass. The legacy and temporary Player inventories are empty.

### Wave i — Final strict cleanup, complete verification, packaging, and documentation

Run final forbidden-state self-tests/scans, clean installation, all strict/type/build/generation/binding/browser/visual/Go/native/resource/secret/package gates, byte-reproducible Vite builds, supported matching-host packages, and active documentation/template updates. Remove every temporary mechanism and ensure the package contains only accepted runtime artifacts.

**Exit**: zero legacy/mixed/temporary/type-escape findings; all unconditional gates pass; conditional credentials/signing/notarization/unavailable host checks are recorded honestly as `NOT RUN`; active documentation describes only the accepted architecture.

## Verification Plan

| Surface | Per-wave evidence | Final governed evidence |
|---|---|---|
| Frontend install/pins | Lockfile and exact-pin checks after dependency/config waves | `task deps:frontend`; unchanged one lockfile; dependency graph/boundary validation |
| Strict TypeScript/SFC | Changed app independent `vue-tsc` plus workspace check | `npm run typecheck:overseer --prefix frontend`, `typecheck:client`, and workspace `typecheck` |
| Vite builds | Changed app build after each slice; immutable assets/selectors | Both independent builds, `task frontend:build`, and two-build byte-identical tree comparison |
| Protobuf/ConnectRPC | Wave-b generation, compile, RPC/path/cardinality/public scans; transport tests later | Task `proto:format:check`, `proto:lint`, `proto:breaking`, `proto:drift:check`, `proto:generated:check`, `proto:check` |
| Wails bridge | Adapter fixtures after wave c; binding checks after affected slices | `task bindings:check`, Wails pins/cutover checks, exact 39 methods/seven events, separate native evidence |
| DOM/accessibility/visual parity | Focused unchanged `.mjs` tests and ownership record per wave | `task browser:test`, complete visual snapshots, final single-owner scan |
| Go behavior/quality | Focused package tests when orchestration/checks change | `go fix ./...` before relevant commits, then `task fmt:check`, `task vet`, `task lint`, `task test`, `task test:race`, `task check`, `task ci:quality` |
| Native embed/startup/resources/secrets | Focused static/native checks after build/entry changes | `task startup:check`, secret/cutover/resource/native smoke checks, separate privileged/public embed inspection |
| Packages | Current-host package after cutover-relevant waves | `task package GOOS=<os> GOARCH=<arch>` on each of five matching hosts; governed content/identity/resource inspection |
| Documentation/governance | Update only when owning workflow/source settles | README/architecture/contribution/package docs and active Spec Kit templates agree; historical specs unchanged |

Browser journeys use the production Player bundle and production Overseer SFCs with a typed fake desktop port. They do not prove Wails/native behavior. Wails binding integrity, native embedding/startup, resources, secure-store/native clipboard/dialog behavior, and packages remain separate evidence classes as defined in [trust-and-evidence-boundaries.md](./contracts/trust-and-evidence-boundaries.md).

Real provider credentials, signing, notarization, stapling, Gatekeeper, or unavailable matching-host checks are never inferred from deterministic tests. They are recorded `NOT RUN` unless actually executed with the required host and credentials.

## Rollback, Compatibility, and Final Removal

Commit `06696ee1c7155a1bb1135ef46ec91445dd73a2a4` is the immutable pre-migration rollback revision. Rollback is source-control reversion to that complete revision; no legacy runtime switch or duplicate bundle ships in the accepted implementation.

Every temporary legacy check, candidate entry, fake port, Vue leaf root, state callback, mount flag, or alias created during implementation must be added to the temporary mechanism register with the Frontend Migration owner, an expiry no later than Overseer wave e or Player wave h, a parity gate, and a removal task. Wave i fails while any temporary item, legacy bootstrap, handwritten production JavaScript module, mixed DOM owner, or prohibited compiler/type escape remains.

## Design Artifacts

- [research.md](./research.md) records resolved technical decisions and experiments.
- [migration-ownership.md](./migration-ownership.md) is the mandatory per-wave DOM/legacy/verification/removal inventory.
- [quickstart.md](./quickstart.md) defines clean verification and honest evidence recording.
- [frontend-build-generation.md](./contracts/frontend-build-generation.md) defines the one-workspace, strict-check, two-build, generated-TypeScript, and Wails binding contract.
- [trust-and-evidence-boundaries.md](./contracts/trust-and-evidence-boundaries.md) defines application trust, runtime validation/lifecycle, preserved RPC behavior, and browser/native evidence separation.

No `data-model.md` is created because persistent and canonical runtime models do not change.
