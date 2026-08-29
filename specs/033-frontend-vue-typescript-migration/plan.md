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

The two applications share only the npm installation/lock boundary, exact compiler/build tooling, and capability-neutral compiler policy in `frontend/tsconfig.base.json`. They do not share an authored environment/global declaration module, application type module, application root, runtime entrypoint, bundle, state store, adapter, transport, view-state type, Wails or ConnectRPC declaration, component, composable, or privileged type. All authored declarations remain inside their owning application source or configuration boundary, and type-only imports cannot establish a cross-boundary application contract. Player's dependency graph is checked for direct and transitive paths to Overseer, Wails, `@wailsio/runtime`, generated Wails bindings, filesystem/native capabilities, private protobuf packages, or privileged state.

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
| `Taskfile.yml` | maintainers, buildtool entrypoints, quality CI, portable release matrix | Add the ten canonical `frontend:*` targets defined below while keeping Task the sole public workflow graph and buildtool/npm scripts as implementation seams rather than documented alternatives. |
| `internal/buildtool` and tests | `task prepare/build/package/dev`, all supported package plans | Insert strict app checks in protobuf → Player → bindings → Overseer order; preserve target/package behavior and standard-library-only ownership. |
| `.github/workflows/wails-cross-platform.yml`, `wails-portable.yml` | PR/main quality and five matching-host release jobs | Continue Task entry and one lockfile cache; enforce new gates through Task/buildtool instead of ad hoc npm workflow commands. |
| `tests/browser/playwright.config.mjs`, `fixture-server/main.go`, fixture bindings | all existing `.mjs` journeys and snapshots | Build production Player and production Overseer SFCs; inject only typed fake desktop transport for browser Overseer evidence; preserve selectors/baselines. |
| `main.go`, `wails_host.go`, `internal/player`, production resource tests | separate Wails and public static filesystems, startup and HTTP/Connect routes | Preserve separate embeds/routes/CSP/limits; update asset expectations only for new bundle manifests, not behavior. |
| `internal/platform/assets_test.go`, startup/release tests | raw source/build/selector/boundary assertions | Replace legacy filename/string coupling with strict SFC, typed adapter, built asset, selector, and boundary assertions without weakening intent. |
| generation/binding/secret/cutover/reproducibility/native/package scripts | Task/CI, native smoke, supported package inspection | Update `.ts` inventories, Vue source paths, final exclusions, production bundle probes, and actionable failure output. |
| README, `ARCHITECTURE.md`, `CONTRIBUTING.md`, active `docs/` | developers/operators/release maintainers | Document one install, two Vue apps, strict checks, generated TypeScript, typed Wails adapter, browser/native evidence separation. |
| Constitution 9.0.0 and `.specify/templates/*` | future specifications/plans/tasks | Keep constitution authoritative; update active templates that still name legacy JavaScript paths. Historical completed specs/evidence remain untouched. |

The explicit command, generation, output, and producer-consumer contract is [frontend-build-generation.md](./contracts/frontend-build-generation.md).

## Canonical Frontend Task Contract

The root Taskfile remains the only public frontend workflow owner. These future targets are the canonical interfaces that implementation tasks must add; underlying npm workspace commands remain private implementation details invoked by the targets and are not documented as a second workflow:

| Task target | Exact ownership |
|---|---|
| `task frontend:typecheck:overseer` | Dispatch only the Overseer workspace `typecheck`; do not install dependencies or check Player. |
| `task frontend:typecheck:client` | Dispatch only the Player workspace `typecheck`; do not install dependencies or check Overseer. |
| `task frontend:typecheck` | Run both per-application strict checks without installing dependencies. |
| `task frontend:build:overseer` | Build only the Overseer production bundle without installing dependencies. |
| `task frontend:build:client` | Build only the Player production bundle without installing dependencies. |
| `task frontend:build` | Perform the single governed frontend dependency installation, then call both per-application build targets. |
| `task frontend:compatibility:check` | Own the FR-023/SC-007 production-fidelity current-and-legacy session/player-configuration round-trip gate. |
| `task frontend:boundary:check` | Own the FR-015/SC-012 complete reviewed valid/invalid frontend boundary-fixture manifest gate. |
| `task frontend:policy:check` | Own forbidden production source, prohibited type escapes, single-lockfile policy, Player dependency boundaries, temporary-mechanism inventory, and final-cutover policy. |
| `task frontend:reproducible:check` | Own two-build byte comparison and actionable sorted tree-digest evidence for both Vite outputs. |

`task frontend:compatibility:check` uses a production-fidelity fixture set containing one representative current session document, one representative legacy version-1 session document, one current player-configuration document, one legacy player-configuration document, compatible unknown fields in both document types, and the established cross-file player-configuration reference behavior. It opens each document through the migrated Overseer boundary, renders and edits without changing established meaning, saves, reopens, and compares supported fields, defaults, references, and compatible unknown fields. It rejects any loss, silent normalization, relocation, or business-meaning change. Task generation must inventory and record the exact reviewed fixture paths, reusing repository version-1 fixtures and Go codecs where suitable rather than inventing a duplicate persistence representation.

`task frontend:boundary:check` consumes one reviewed manifest planned at `tests/browser/fixtures/frontend-boundary-manifest.json`. Each manifest entry records its boundary class, fixture identifier, owning adapter or composable, expected accept/reject result, expected trusted projection or no-state-change outcome, applicable migration wave, and focused test file. The population covers Wails/native named events, Wails command results, localStorage/storage-event records, DOM/form inputs, pointer/keyboard-derived values, ConnectRPC-decoded semantic network values, clipboard outcomes, sound-manifest/asset values, and presentation-stream capability/results. The gate runs every manifest entry, rejects every listed invalid fixture before trusted state mutation, accepts every listed valid fixture, and fails when an entry lacks a test mapping. This is complete for the reviewed manifest only; it does not claim every theoretically possible invalid value.

## Implementation Waves

Every wave uses the corresponding complete DOM record in [migration-ownership.md](./migration-ownership.md). Every intermediate revision must build and pass its applicable checks.

### Wave a — Vue/TypeScript infrastructure and bounded temporary legacy checking

Add exact dependencies, the shared/app TypeScript configurations, Vue Vite plugins, workspace/app type-check commands, fixture compiler support, exact-pin/lock checks, and narrowly enumerated temporary legacy checking. Keep both production DOMs and bootstraps unchanged. Establish the rollback commit and temporary-mechanism register before component work.

**Exit**: clean workspace install, exact dependency graph, strict candidate SFC checks, both unchanged production builds, and existing browser/visual baselines pass. Temporary checking has owners and wave-e/wave-h expiries.

### Wave b — Deterministic protobuf `target=ts` generation

Switch only `proto/buf.gen.es.yaml` to `target=ts`, preserve `import_extension=js`, regenerate rather than edit, and update provenance, imports, drift tests, strict compilation, native probe consumption, scripts, buildtool, CI, and clean-checkout inventories. Confirm descriptors, Go generation, schemas, RPCs, wire behavior, and capability boundaries are unchanged.

**Exit**: exactly five deterministic checked-in `_pb.ts` files, no old generated JS, actionable deliberate-drift rejection, two identical generations, strict Player compilation/build, and all protobuf/Connect/public-private gates pass.

### Wave c — Shared compiler policy, application-owned declarations, both Vue shells, and typed desktop API

Establish the shared strict compiler policy in `frontend/tsconfig.base.json`, then create application-owned environment/global/transport/view declarations, both Vue roots and entrypoints in candidate/fixture documents, and the typed Wails alias/desktop adapter. Establish the production-code/bootstrap injection seam for the Overseer browser fixture. Add Player-owned transport/projection interfaces without changing production ownership. Do not create a shared authored application declaration or type module.

**Exit**: independent and workspace strict checks; both candidate shells build; the wave-c desktop/Wails subset of the reviewed boundary manifest passes for malformed values, ordering, subscriptions, clipboard, and method/event integrity; Player graph has no privileged dependency.

### Wave d — Overseer leaf components and composables

Migrate complete Overseer leaf families in reviewable slices: application-update presentation; approvals and confirmation dialogs; session/player management; terminal groups; public access and credentials; then remaining leaf controls. Each slice moves complete DOM/handlers/focus/pending state into Vue and removes its legacy counterpart immediately.

**Exit**: every moved subtree has one Vue owner, no legacy query/handler, unchanged focused selectors/semantics/copy/focus/visuals, strict build, and relevant browser tests. Temporary leaf root/state callbacks remain explicitly owned and expire in wave e.

### Wave e — Complete Overseer cutover and remove its legacy bootstrap

Move the shell/runtime status, terminal/group list, selection, authoring tree/forms, coordination, broadcast, hacking controls, public access, updates, and all dialogs into one `#overseerApp`. Remove `overseer.js`, `desktop-api.js`, dynamic renderers, global facade, temporary islands/bridges/mounts, legacy script tags, and Overseer `allowJs`/`checkJs` configuration. Complete the desktop/Wails boundary-manifest cases and run `task frontend:compatibility:check` against the reviewed current/legacy session and player-configuration fixture set.

**Exit**: complete Overseer strict/build/browser/visual parity, `task frontend:compatibility:check`, applicable desktop/Wails boundary cases, and separate Wails/native/embed/resource/package gates pass; the Overseer forbidden-state and ownership inventories are empty. Player wave f cannot begin earlier.

### Wave f — Player shell, identity, transport, session initialization, navigation, and presentation foundations

Build the complete candidate shell, connection overlay, recognition/lease coordination, snapshot-first subscription/reconnect, player identity/roster/role/controller state, terminal list/entry/navigation/pagination/command output, action correlation, and transient presentation foundations in a separate candidate document. Add the Player storage, storage-event, decoded-network, DOM/form, and navigation-input entries and focused tests to the reviewed boundary manifest. Production Player remains wholly legacy-owned.

**Exit**: candidate focused type/build/browser checks prove first connection, multi-tab convergence, reconnect, revisions, authority, navigation, pending actions, and accessibility; production legacy suite still passes; all acquired lifecycle resources have cleanup.

### Wave g — Player hacking, CRT/typewriter, sound, and presentation-uplink integrations

Complete candidate hacking components, target geometry/fitting/focus/input, CRT/typewriter timing, pagination measurement, sound/gesture activation, cue de-duplication, presentation streaming capability/probe/mailbox/cancellation/retry, and unary fallback. Add pointer/keyboard-derived, sound-manifest/asset, and presentation-stream capability/result entries and focused tests to the reviewed boundary manifest. Keep all imperative work in the named Vue-owned composables/adapters/directives.

**Exit**: complete candidate Player `.mjs` and immutable visual suite passes with exact timing/geometry/audio/stream behavior and cleanup evidence; production legacy suite remains green; no baseline is intentionally updated.

### Wave h — Complete Player cutover

Atomically transfer the production document to the single `#playerApp` root. Remove `client.js`, `sound.js`, `presentation-uplink.js`, old script tag, candidate selection, staging entry/mount, legacy handlers/resources, and Player `allowJs`/`checkJs` configuration. Complete every Player-owned boundary-manifest entry and focused test before declaring the production transfer complete.

**Exit**: independent/workspace strict checks, Player build, complete browser/visual suite, public dependency scan, reconnect/multi-tab/authority/hacking/timing/sound/uplink stress checks, and separate native Player serving/package checks pass. The legacy and temporary Player inventories are empty.

### Wave i — Final strict cleanup, complete verification, packaging, and documentation

Run the canonical frontend Task gates, final forbidden-state self-tests/scans, clean installation, all generation/binding/browser/visual/Go/native/resource/secret/package gates, supported matching-host packages, and active documentation/template updates. `task frontend:compatibility:check` reruns the complete persistence fixture set; `task frontend:boundary:check` runs every reviewed manifest entry and rejects unmapped entries; `task frontend:policy:check` proves final source/boundary/cutover policy; and `task frontend:reproducible:check` proves byte-identical output trees. Remove every temporary mechanism and ensure the package contains only accepted runtime artifacts.

**Exit**: zero legacy/mixed/temporary/type-escape findings; all unconditional gates pass; conditional credentials/signing/notarization/unavailable host checks are recorded honestly as `NOT RUN`; active documentation describes only the accepted architecture.

## Verification Plan

| Surface | Per-wave evidence | Final governed evidence |
|---|---|---|
| Frontend install/pins | Lockfile and exact-pin checks after dependency/config waves | `task frontend:build` owns the one clean install before both builds; `task frontend:policy:check` proves one lockfile and exact dependency/boundary policy |
| Strict TypeScript/SFC | Changed app independent strict check plus workspace check | `task frontend:typecheck:overseer`, `task frontend:typecheck:client`, and `task frontend:typecheck`; none installs dependencies |
| Vite builds | Changed app build after each slice; immutable assets/selectors | `task frontend:build:overseer`, `task frontend:build:client`, and governed `task frontend:build` |
| Vite reproducibility | Focused tree comparison after output-affecting waves | `task frontend:reproducible:check`; two byte-identical builds and actionable sorted path/mode/size/SHA-256 evidence for both outputs |
| Protobuf/ConnectRPC | Wave-b generation, compile, RPC/path/cardinality/public scans; transport tests later | Task `proto:format:check`, `proto:lint`, `proto:breaking`, `proto:drift:check`, `proto:generated:check`, `proto:check` |
| Wails bridge | Adapter fixtures after wave c; binding checks after affected slices | `task bindings:check`, Wails pins/cutover checks, exact 39 methods/seven events, separate native evidence |
| Persistence compatibility | Reviewed existing/current/legacy fixture paths selected during task generation; focused Overseer checks in wave e | `task frontend:compatibility:check` in waves e and i; open/edit/save/reopen comparison of fields, defaults, references, unknown fields, location, and meaning |
| External boundary validation | Desktop/Wails manifest subset in c/e; Player storage/network/input subset in f; pointer/sound/stream subset in g; production Player completion in h | `task frontend:boundary:check` in wave i runs the complete reviewed valid/invalid manifest and rejects missing test mappings |
| DOM/accessibility/visual parity | Focused unchanged `.mjs` tests and ownership record per wave | `task browser:test`, complete visual snapshots, final single-owner scan |
| Frontend policy/final cutover | Focused ownership, dependency, temporary-mechanism, and source scans after each cutover | `task frontend:policy:check` owns forbidden source/type escapes, one lockfile, Player boundary, temporary inventory, and final removal |
| Go behavior/quality | Focused package tests when orchestration/checks change | `go fix ./...` before relevant commits, then `task fmt:check`, `task vet`, `task lint`, `task test`, `task test:race`, `task check`, `task ci:quality` |
| Native embed/startup/resources/secrets | Focused static/native checks after build/entry changes | `task startup:check`, secret/cutover/resource/native smoke checks, separate privileged/public embed inspection |
| Packages | Current-host package after cutover-relevant waves | `task package GOOS=<os> GOARCH=<arch>` on each of five matching hosts; governed content/identity/resource inspection |
| Documentation/governance | Update only when owning workflow/source settles | README/architecture/contribution/package docs and active Spec Kit templates agree; historical specs unchanged |

Browser journeys use the production Player bundle and production Overseer SFCs with a typed fake desktop port. They do not prove Wails/native behavior. Wails binding integrity, native embedding/startup, resources, secure-store/native clipboard/dialog behavior, and packages remain separate evidence classes as defined in [trust-and-evidence-boundaries.md](./contracts/trust-and-evidence-boundaries.md).

The typed fake `DesktopPort` is permanent test-only browser evidence infrastructure outside production source and bundles. It remains after wave e, is never embedded or packaged, is contract-checked against the production `DesktopPort`, and cannot satisfy or replace Wails/native evidence. Only the candidate/test entrypoint, selector, and candidate build selection expire in wave e.

Real provider credentials, signing, notarization, stapling, Gatekeeper, or unavailable matching-host checks are never inferred from deterministic tests. They are recorded `NOT RUN` unless actually executed with the required host and credentials.

## Rollback, Compatibility, and Final Removal

Commit `06696ee1c7155a1bb1135ef46ec91445dd73a2a4` is the immutable pre-migration rollback revision. Rollback is source-control reversion to that complete revision; no legacy runtime switch or duplicate bundle ships in the accepted implementation.

Every temporary legacy check, candidate entry, Vue leaf root, state callback, mount flag, or compatibility alias created during implementation must be added to the temporary mechanism register with the Frontend Migration owner, an expiry no later than Overseer wave e or Player wave h, a parity gate, and a removal task. Permanent test-only evidence infrastructure is governed separately and must remain outside production source, bundles, embeds, and packages. Wave i fails while any temporary item, legacy bootstrap, handwritten production JavaScript module, mixed DOM owner, or prohibited compiler/type escape remains.

## Design Artifacts

- [research.md](./research.md) records resolved technical decisions and experiments.
- [migration-ownership.md](./migration-ownership.md) is the mandatory per-wave DOM/legacy/verification/removal inventory.
- [quickstart.md](./quickstart.md) defines clean verification and honest evidence recording.
- [frontend-build-generation.md](./contracts/frontend-build-generation.md) defines the one-workspace, strict-check, two-build, generated-TypeScript, and Wails binding contract.
- [trust-and-evidence-boundaries.md](./contracts/trust-and-evidence-boundaries.md) defines application trust, runtime validation/lifecycle, preserved RPC behavior, and browser/native evidence separation.

No `data-model.md` is created because persistent and canonical runtime models do not change.
