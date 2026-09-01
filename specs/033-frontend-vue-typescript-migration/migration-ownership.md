# DOM and Migration Ownership Record

## Rules that apply to every wave

Every rendered subtree has exactly one owner: either a named Vue application root or a named legacy renderer. Ownership includes markup creation, descendant queries, mutation, event binding, focus behavior, timing, and cleanup. Legacy code must not query, render, replace, mutate, or bind inside a Vue-owned subtree; Vue code must not query or mutate a legacy-owned subtree. State crosses an ownership boundary only as typed data or callbacks through a documented adapter, never through DOM inspection or a shared cross-boundary store.

The Frontend Migration team owns every temporary mechanism in this record. The immutable rollback source is commit `06696ee1c7155a1bb1135ef46ec91445dd73a2a4`. A wave is not complete until its focused checks pass and its removal criteria are either satisfied or explicitly carried to the named expiry wave. Existing selectors, `.mjs` journeys, screenshots, CSS, fonts, sounds, copy, accessibility semantics, focus, keyboard, pointer, CRT/typewriter timing, hacking geometry, and gesture/audio behavior are immutable parity baselines.

## Wave a — Vue/TypeScript infrastructure and bounded legacy checking

**Vue mount boundaries**: None in either production document. New SFC/compiler fixtures may mount only in isolated test documents outside production routes.

**Legacy-owned adjacent subtrees**:

- Overseer: the complete `body` produced by `frontend/overseer/src/index.html` and operated by `frontend/overseer/src/overseer.js` plus `desktop-api.js`.
- Player: the complete `body`, including `.crt`, `#screen`, and `#connOverlay`, produced by `frontend/client/index.html` and operated by `client.js`, `sound.js`, and `presentation-uplink.js`.

**Remaining legacy files and handlers**: All current production JavaScript and all current document/window listeners, timers, animation frames, observers, subscriptions, streams, audio resources, dynamic dialogs, delegated row/tree/hacking handlers, and bootstraps remain active and solely legacy-owned.

**Prohibited boundaries**: No Vue research fixture may mount into or query a production document. No production script may import a fixture. The shared TypeScript base may not include private Overseer modules in the Player graph.

**Focused checks**:

- One clean `npm ci --prefix frontend`; unchanged single lockfile and no app lockfiles.
- Exact dependency-pin and workspace-shape checks.
- Candidate strict SFC `vue-tsc` fixtures and both unchanged Vite builds.
- Full existing browser and visual suite against legacy production outputs.
- Existing Wails binding, protobuf, native embed, resource, and package checks applicable to infrastructure changes.

**Removal criteria**: Temporary `frontend/overseer/tsconfig.legacy.json` and `frontend/client/tsconfig.legacy.json`, if needed, list their exact JavaScript inputs and cannot be extended implicitly. Overseer legacy checking expires in wave e; Player legacy checking expires in wave h. No `allowJs` or `checkJs` enters `frontend/tsconfig.base.json`, `frontend/overseer/tsconfig.json`, or `frontend/client/tsconfig.json`.

**Wave-a exit evidence (T016)**: The complete Wave-a gate passed on 2026-08-30. Both production documents remain wholly legacy-owned with their original script entries and lifecycle owners; no Vue production mount or candidate selection exists. The exact eight canonical frontend targets, strict compiler programs, one workspace lockfile, exact dependency pins, public-only Player output, two byte-identical Vite output trees, focused ConnectRPC/CRT behavior, and immutable visual snapshots all passed. The T007 Overseer and T008 Player bounded legacy compiler rows remain open with their original exact inputs and e/h removal tasks.

## Wave b — Deterministic protobuf `target=ts` generation

**Vue mount boundaries**: None in production; ownership remains unchanged from wave a.

**Legacy-owned adjacent subtrees**: Both complete production documents remain legacy-owned.

**Remaining legacy files and handlers**: All application JavaScript remains. Player imports are mechanically updated only as required to consume generator-owned `_pb.ts` through `.js` ESM specifiers; no UI behavior moves.

**Prohibited boundaries**: Generated TypeScript cannot import private, persistence, configuration, Overseer, Wails, or native modules. No schema, Go generation, descriptor, service, RPC path, field number, wire encoding, message meaning, authorization rule, cardinality, request limit, or error behavior may change. No generated file may be hand-edited.

**Focused checks**:

- Two clean `protoc-gen-es` generations are byte-identical and emit only the five reviewed `_pb.ts` files with exact `2.13.0` provenance and `.js` import specifiers.
- Browser TypeScript deliberate-drift self-test fails actionably and restores the fixture.
- Strict Player compilation includes all generated TypeScript.
- Existing Go generated tree, descriptors, Buf format/lint/breaking checks, RPC inventories, Connect paths, and public/private scans remain unchanged.
- Player build plus focused ConnectRPC browser journeys pass with unchanged selectors/behavior.
- The unchanged production Player Vite build and existing CRT/rendering visual suite pass with immutable snapshots and zero selector, screenshot, CSS, copy, accessibility, layout, timing, or geometry baseline change. Task generation records the exact focused Playwright/visual command; `target=ts` conversion cannot create or approve snapshots.

**Removal criteria**: All `_pb.js` generated files and every script/import/probe assumption that checked them are replaced in the same wave. No parallel `target=js`, `target=js+dts`, or checked-in compiled-JavaScript output remains.

**Wave-b exit evidence (T030)**: The complete Wave-b gate passed on 2026-08-30. Generation emits exactly the five reviewed `_pb.ts` contracts with pinned provenance and `.js` ESM specifiers; no generated `_pb.js`, declaration sidecar, or parallel compiled tree remains checked in. The Player strict program includes exactly those five contracts, while both production documents and all application DOM/lifecycle behavior remain wholly legacy-owned. Protobuf generation, drift, schema/Go-output integrity, all eight RPC contracts, public authorization and limits, the unchanged Player build, focused ConnectRPC behavior, and immutable CRT visuals passed. The T020 deliberate-drift mechanism remains closed, and no temporary Wave-b mechanism remains open.

## Wave c — Shared compiler policy, application-owned declarations, both Vue shells, and typed desktop API

**Vue mount boundaries**:

- Production documents remain wholly legacy-owned.
- `#overseerApp` and `#playerApp` exist and mount only in isolated candidate/fixture documents built from the new application entrypoints and application-owned declarations. The Player candidate is an empty, capability-neutral shell: strict compiler participation, Player-owned declarations and ports, an empty `App.vue`, mount function, isolated document/entry, and dependency/boundary fixtures only.
- The Overseer candidate root receives a typed `DesktopPort`; the production candidate uses the Wails adapter and the browser fixture uses a deterministic fake port.

**Legacy-owned adjacent subtrees**: The complete live Overseer and Player production documents remain legacy-owned; candidate documents are separate documents, never adjacent subtrees in the same document.

**Remaining legacy files and handlers**: All current production JavaScript bootstraps and handlers remain. `desktop-api.js` remains the production bridge until Overseer ownership transfer; `client.js`, `sound.js`, and `presentation-uplink.js` remain Player production owners.

**Prohibited boundaries**: Candidate Vue apps cannot locate or mutate legacy production DOM. The applications share only the npm install/lock boundary, exact compiler/build tooling, and capability-neutral `frontend/tsconfig.base.json`; all authored environment, global, transport, view-state, Wails, ConnectRPC, component, and composable declarations remain application-owned. Wave c cannot add Player business behavior, render or replace production Player DOM, consume Wails or another privileged API, or make the Player candidate production-selected. Player feature migration cannot begin until the complete wave-e Overseer exit gate passes. Player declarations/config/import graphs, including type-only imports, cannot reference Wails, `@wailsio/runtime`, bindings, native capabilities, privileged types, Overseer state, or the Overseer candidate. The typed desktop adapter is the only authored import boundary for generated Wails service modules, runtime events, and clipboard.

**Focused checks**:

- Workspace plus independent Overseer/Player strict `vue-tsc` checks.
- Both candidate Vite builds and unchanged legacy production builds.
- Desktop adapter malformed-event/result/clipboard fixtures, listener-before-getter ordering, revision reconciliation, and exact-once unsubscribe tests.
- The wave-c Wails/native named-event and command-result subset of the reviewed frontend boundary manifest has complete focused test mappings and expected accept/reject plus trusted-projection/no-state-change outcomes.
- Player dependency-graph boundary scan.
- Existing production browser/visual suite remains unchanged.

**Removal criteria**: Candidate-only entrypoints are either promoted to the production entrypoint or deleted at their application's cutover. The typed Wails alias declaration remains only as the final adapter boundary and must stay synchronized with binding integrity; no global `window.desktopAPI` compatibility surface survives wave e unless a browser-test-only port implements the same typed interface outside production source.

**T038 candidate activation**: `frontend/overseer/test-fixtures/index.html` now solely owns its isolated `#overseerApp` root through `candidate-main.ts`, `mountOverseerApp`, and the permanent browser-only fake `DesktopPort`. The candidate remains governed by the T038/T090 temporary-register row and has no production document, route, or Vite selection.

**T039 candidate selection**: Vite candidate mode and the Playwright-only server at `127.0.0.1:34120` select the isolated Overseer document. Production mode continues to select `frontend/overseer/src/index.html` and its legacy scripts; candidate selection remains governed by the T039/T090 temporary-register rows and is not native or embedding evidence.

**T041 Player candidate activation**: `frontend/client/test-fixtures/index.html` solely owns its isolated `#playerApp` root through `candidate-main.ts` and `mountPlayerCandidate`. The entry supplies only the capability-neutral idle transport required by the empty shell. Production Player selection remains the legacy document and `client.js`; the candidate document/entry remain governed by the T041/T156 temporary-register rows.

**Wave-c exit evidence (T045)**: The complete wave-c gate passed on 2026-08-30. Both empty candidates compile and build from isolated documents; the production Overseer and Player documents remain wholly legacy-selected. The typed desktop adapter rejects malformed results, event payloads, and clipboard inputs before trusted state changes, preserves listener-before-getter ordering and monotonic revisions, releases subscriptions exactly once, and retains the exact 39-method/seven-event Wails inventory. Every wave-c boundary fixture has a focused manifest mapping, the Player foundation scan contains no privileged or business-behavior edge, all 192 unconditional browser tests passed, the two real-ngrok credential-dependent cases were skipped as expected, and immutable CRT snapshots remained unchanged. Overseer candidates remain governed through T090; Player candidate and staging mechanisms remain governed through T156.

## Wave d — Overseer leaf components and composables

**Vue mount boundaries**: One production `#overseerVueLeaves` root owns only complete body-sibling leaf subtrees migrated in reviewed slices. The preferred order is application-update status/offer/restart, then approval/confirmation dialogs, then player/session/group and public-access dialog families. Each slice moves the complete element, descendants, handlers, focus rules, and pending state together. Player has no production Vue mount.

**Legacy-owned adjacent subtrees**: `#legacyOverseerRoot` owns `#startScreen`, `#mainLayout`, `.topbar`, terminal/group sidebar, coordination panel, hacking controls, authoring tree/forms, node panel, and every dialog not yet listed as Vue-owned. Player remains wholly legacy-owned.

**Remaining legacy files and handlers**: `overseer.js` and `desktop-api.js` remain only for the explicitly inventoried legacy root and unmigrated dialog families. Remaining dynamic-dialog constructors, global queries, click/Escape handlers, and focus restoration are listed at each slice review. All Player legacy files remain.

**Prohibited boundaries**: Do not mount Vue inside `#termList`, `#treeView`, `#nodeForm`, row lists, or any container that legacy code clears/replaces. Legacy cannot open, close, query, focus, or mutate a migrated Vue dialog; it may request typed actions through props/callbacks only. Vue cannot inspect legacy selection or pending state through the DOM.

**Focused checks**:

- Overseer strict type-check and production build after each leaf family.
- Focused Playwright suites for the migrated family with unchanged IDs, roles, copy, focus restoration, Escape/cancel behavior, revision ordering, and secret handling.
- Full Overseer visual/layout parity where the family is visible.
- Desktop adapter and Wails binding integrity checks remain separate from browser assertions.
- Ownership scan proves every migrated ID is rendered by exactly one owner and has no legacy handler/query.

**Removal criteria**: A leaf leaves the legacy inventory only when its old markup, renderer, query, and handler are removed. `#overseerVueLeaves`, the remaining legacy root, and any typed state bridge all expire in wave e; no leaf may be represented by both owners.

**T046 coexistence activation**: `frontend/overseer/src/index.html` contains sibling `#legacyOverseerRoot` and `#overseerVueLeaves` roots. Legacy queries, dynamic dialog insertion, and delegated handlers are scoped to `#legacyOverseerRoot`; `frontend/overseer/src/main.ts` mounts the Vue leaf application, which remained empty until the first reviewed leaf in T048. The temporary `__overseerCoexistenceBridge` validates and detaches record messages before invoking typed callbacks, carries no DOM node or selector, and is provided to the Vue leaf application through `mountOverseerLeaves`. These two temporary mechanisms remain governed by the T046/T090 rows.

**T048 application-update migration**: `#overseerVueLeaves` now solely owns `#applicationUpdateStatusPanel`, `#applicationUpdateDialog`, `#applicationUpdateRestartDialog`, and every descendant update ID through `App.vue`, the three application-update components, and `useApplicationUpdate`. The production legacy document no longer contains those nodes, and `overseer.js` no longer queries, renders, subscribes, or handles the update lifecycle. The composable consumes detached `DesktopPort` snapshots, rejects stale/conflicting revisions, deduplicates decisions by attempt, bounds untrusted text, restores only connected focus targets, and releases its update subscription on unmount. The coexistence root and typed bridge remain temporary through T090; application-update lifecycle ownership does not.

**T050 dialog-focus lifecycle**: `useDialogFocus` owns connected-only opener capture, queued initial/restoration focus, and generation-based cancellation on unmount. The `dialogFocus` directive owns its exact `cancel` and `keydown` listeners and removes them before unmount. T048 already removed the matching application-update focus globals from `overseer.js`; focus code for other legacy-owned dialog families remains explicitly owned by their later T052–T072 slices and is not broadened or removed here.

**T052 command-approval migration**: `#overseerVueLeaves` solely owns `#commandExecutionDialog` and its descendant IDs through `CommandExecutionDialog.vue` and `useCommandApproval`. The composable consumes the fixture/production `DesktopPort` coordination stream, gates revisions and request generations, retains a bounded resolved-ID set, suppresses duplicate decisions synchronously, and releases its subscription on unmount. `overseer.js` no longer creates, queries, synchronizes, resolves, or binds keys/cancel handlers for this dialog; the legacy coordination state continues to own only adjacent non-dialog status and later approval families.

**T062 terminal-group dialog migration**: `#overseerVueLeaves` solely owns `#terminalGroupDraftDialog`, `#terminalGroupImpactDialog`, their complete forms, impact disclosure, validation feedback, modal state, focus lifecycle, pending state, and `replaceTerminalGroups` calls through `TerminalGroupDraftDialog.vue` and `TerminalGroupImpactDialog.vue`. The components validate detached bridge projections, reject duplicate submissions, ignore stale async UI completions after close/rebind/unmount, and clear their bridge subscriptions on unmount. `overseer.js` retains only the still-unmigrated terminal/group model calculations and sends typed record projections; it no longer queries, renders, opens, focuses, submits, or binds listeners to either migrated dialog.

**T064 public-access status migration**: `#overseerVueLeaves` solely owns the in-layout `#publicAccessSection` subtree through `PublicAccessPanel.vue` and `usePublicAccess.ts`. The composable subscribes before the initial getter, applies generation/settings-revision ordering fail closed, owns start/stop/copy lifecycle state, preserves local-mode failure messaging, and releases the runtime subscription exactly once on unmount. `overseer.js` no longer queries, renders, subscribes, or binds handlers for the status panel; until T065–T068 migrate the dialog family, it receives only detached public-access snapshots and explicit settings-open requests through the typed coexistence bridge.

**T065 public-access settings migration**: `#overseerVueLeaves` solely owns `#publicAccessSettingsDialog`, its setup guide, connection draft, credential-presence summary, documentation actions, validation/error state, save pending state, modal lifecycle, and focus restoration through `PublicAccessSettingsDialog.vue`. Provider-token input is cleared immediately after submission and on close/unmount, invalid reserved-domain shapes are rejected before the privileged port call, and retained newer status wins over stale command completion. `overseer.js` no longer queries, renders, submits, focuses, or binds listeners to the settings subtree; the still-legacy T066/T067 child dialogs receive only explicit open/share requests through the coexistence bridge.

**T066 provider-token migration**: `#overseerVueLeaves` solely owns `#publicAccessProviderTokenDialog`, its password input, validation/error state, replacement/deletion calls, pending state, modal lifecycle, and exact opener focus through `ProviderTokenDialog.vue`. The token is never reconstructed from stored presence, is cleared immediately after request dispatch and again on close/unmount, and is not retained in detached command records. `overseer.js` no longer queries, opens, renders, submits, focuses, or binds listeners to the provider-token dialog.

**T067 player-credential migration**: `#overseerVueLeaves` solely owns `#publicAccessPlayerCredentialsDialog`, username/password drafts, password-length validation, save/delete/generate calls, native credential sharing, pending/error state, modal lifecycle, and exact opener focus through `PlayerCredentialsDialog.vue`. Password drafts are cleared immediately after request dispatch and on close/unmount; generated values cross only the explicit one-time T068 handoff and are never stored in public snapshots. `overseer.js` no longer queries, opens, renders, submits, focuses, shares, generates, or binds listeners to the player-credential dialog.

**T068 generated-password and clipboard migration**: `#overseerVueLeaves` solely owns `#generatedPasswordDialog`, its one-time value, modal lifecycle, copy/dismiss actions, and generate-button focus restoration through `GeneratedPasswordDialog.vue`; `useClipboard.ts` validates non-empty text, uses the browser clipboard with the typed native port as a bounded fallback, isolates failures, and cancels its transient status timer on clear/unmount. The generated value is cleared on copy, dismiss, cancel, and unmount. `overseer.js` no longer queries, opens, mutates, or binds handlers to the generated-password dialog, and `desktop-api.js` no longer exposes the migrated legacy clipboard helper. The governed T063 expected RED assertion is GREEN.

**T069 application-update join**: `App.vue` mounts the status, offer, and restart leaves from one `useApplicationUpdate` instance under the single `#overseerVueLeaves` application. Each governed update ID has exactly one DOM owner, cumulative revision filtering is shared across all three leaves, offer/restart focus is stable, and the single runtime subscription is released exactly once by the composable unmount lifecycle. No update element is queried or handled by the legacy root.

**T070 approval/reset join**: `App.vue` mounts command approval, terminal-navigation approval, terminal-switch resolution, and command-state reset leaves under one Vue application. Every governed dialog ID has one owner and zero legacy-root copies; the shared coordination subscriptions and bridge listeners are released on the first unmount and remain unchanged on repeated unmount. Focus lifecycle stays within the joined Vue leaves, while only detached state/request messages cross the temporary bridge.

**T071 session/player/group join**: `App.vue` mounts session-document controls, player-configuration controls, logical sessions, player management/delete, and terminal-group draft/impact leaves under one Vue application. Each migrated dialog has one owner and no legacy-root copy; in-layout controls use only their explicit Teleport targets, keyed rows remain component-owned, and coordination/bridge resources release on the first unmount without repeated cleanup. No migrated leaf reads or mutates legacy DOM state.

**T072 public-access join**: `App.vue` mounts the public-access panel, settings, provider-token, player-credentials, and generated-password leaves from one public-access snapshot owner. Every governed ID has exactly one DOM owner and each dialog has no legacy-root copy. First unmount releases the sole status subscription, repeated unmount performs no additional cleanup, all provider/player/generated secrets are discarded, and the clipboard status timer is component-owned and cleared on unmount.

**Wave-d exit evidence (T075)**: Every T048–T072 leaf is exclusively Vue-owned under the single `#overseerVueLeaves` application. The migrated-ID scan is empty across `overseer.js`, `desktop-api.js`, `main.ts`, and `mount.ts`; the only production-document references to migrated dialog IDs are the two semantic `aria-controls` links from the still-legacy terminal-group and logical-session trigger buttons. Full browser parity passed 213 tests with two credential-gated real-ngrok journeys skipped and no CRT snapshot diff; the exact focus/desktop cleanup pair passed 15 tests.

The remaining Wave-e legacy inventory is exact. `overseer.js` owns the start/runtime shell, server-link and legacy status presentation, adjacent server/client/hack/session/coordination projections, the typed coexistence request dispatcher, terminal/group model and action menus, terminal list/tree/authoring/settings controls, broadcast/end/take-off-air/create-terminal dialogs, and hacking controls. `desktop-api.js` owns the temporary 39-method/seven-event facade, validation/normalization, generated-binding adaptation, subscription registry, and hot-disposal cleanup. Neither file queries, renders, opens, focuses, or binds a migrated leaf. Both legacy files, the two coexistence roots, and the typed bridge remain governed until unconditional T090 removal.

**T077 runtime-shell migration**: `#overseerVueLeaves` owns `#startScreen` through `StartScreen.vue` and the shell orchestration marker through `OverseerLayout.vue`; `RuntimeHeader.vue` is the sole renderer of `#runtimeHeader`, `#sessionFileLabel`, `#serverUrl`, and `#clientCount` inside the explicit empty `#runtimeHeaderVueLeaf` Teleport target. `useRuntimeStatus` preserves starting, ready-local, ready-public, warning, and fatal presentation semantics; `useDesktopRuntime` owns event-before-snapshot server/client projections, native URL handoff, and exact release of both subscriptions. `useSessionDocument` exposes the acquired filename as readonly typed state. `overseer.js` no longer queries or renders the header, subscribes to server/client runtime events, requests startup status, opens the player URL, publishes startup projections, or mutates the filename. The outer legacy layout and its remaining main subtree stay adjacent and temporary through T090.

**T078 terminal-selection migration**: `TerminalSidebar.vue` and keyed `TerminalRow.vue` instances are the sole renderers of `#termList` terminal rows through the explicit empty legacy target. `useTerminalSelection` accepts only increasing, structurally valid detached snapshots, emits revision-bound selection requests, handles component-owned focus restoration, and releases its bridge subscription on the first unmount. `overseer.js` retains the terminal model but no longer queries or mutates `#termList`, creates rows/action triggers, binds row/menu listeners, or owns collapsed-list state; it publishes detached ordered selection projections and rejects stale selection requests before mutation. Group composition and the complete action set remain assigned to T079–T080, while the temporary coexistence target expires unconditionally at T090.

**T079 terminal-group migration**: `TerminalGroupList.vue` and keyed `TerminalGroupRow.vue` instances now exclusively compose the direct `#termList` group hierarchy, group headers, collapse state, group action menus, and ordered terminal membership. `useTerminalGroups` rejects non-increasing revisions, duplicate group IDs, empty groups, and duplicate terminal membership before reactive state changes; group actions are bound to the current detached projection revision. Document-level outside-click and Escape listeners are component-owned and removed on unmount. BUG-018 also makes `useSessionDocument` refresh post-acquisition runtime status and transfer the authoritative saved revision, while the temporary facade returns a fresh status for that explicit call; legacy group confirmation now receives the correct expected session revision and the focused test observes an accepted persisted reorder. `overseer.js` retains only group-domain candidate calculations/dialog handoff until the later authoring integration slices and no longer owns any group-list DOM renderer or handler.

**T080 terminal action/editor/settings migration**: `TerminalActionMenu.vue` replaces the temporary per-row trigger with revision-bound rename, regroup, reorder, and delete actions plus component-owned outside-click, Escape, mutual-menu, rename-draft, and cleanup lifecycles. `TerminalEditor.vue` is the single validated projection owner for the selected-terminal header and composes `TerminalSettings.vue` through the explicit `#terminalEditorVueLeaf` and `#terminalSettingsVueLeaf` targets. Settings remain local drafts until Apply; blank or cancelled renames and stale editor/row requests cannot mutate or save. Publish acknowledgement timing is component-owned and cleared on unmount. `overseer.js` no longer queries or renders any migrated header/settings ID, creates the reset button or rename input, binds their handlers, or owns the publish timer; it retains detached model mutations and privileged command dispatch until T083 removes the remaining authoring bridge. BUG-019 records the existing row/projection and adjacent group-menu coordination seams required for exclusive integration.

**T081 terminal tree/node-editor migration**: `TerminalTree.vue` is the sole revisioned bridge subscriber and owns the explicit `#terminalTreeVueLeaf` and `#nodeEditorVueLeaf` targets. Recursive keyed `TerminalTreeNode.vue` instances own every `#treeView` descendant, expansion control, selection event, and add-node focus handoff; `NodeEditor.vue` owns every `#nodeForm` field, mode surface, validation error, delete confirmation, and command-state reset trigger. Invalid or non-increasing snapshots are rejected before reactive state changes, and every request carries the selected terminal and current projection revision. `overseer.js` retains only terminal-tree domain lookup/mutation, persistence, canonical reset dispatch, and detached projections; it no longer queries, creates, replaces, focuses, or binds any tree/editor DOM node. The component releases its bridge subscription and owned element reference on unmount.

**T082 create-terminal dialog migration**: `CreateTerminalDialog.vue` exclusively renders and operates `#createTerminalDialog` and all descendants under the single Vue root. It owns draft clearing, blank-name validation, modal open/close, Escape cancellation, invalid-field focus, pending controls, revision filtering, native-dialog cleanup, and bridge release. The adjacent still-legacy `#btnAddTerminal` trigger publishes an open snapshot and restores its own focus only after the Vue subscriber synchronously closes the dialog. `overseer.js` accepts only current-revision requests and retains terminal construction, group-domain normalization, persistence, and post-create selection; it contains no create-dialog query, markup, modal call, field mutation, or descendant handler.

**T083 terminal-authoring integration**: `useTerminalAuthoring.ts` is the single Vue state and request boundary for terminal selection, groups, editor/settings, tree/node editor, and create-terminal state. It accepts one structurally validated `terminal-authoring-snapshot`, applies the group/terminal/editor/tree/create slices under one increasing revision, drives the typed sidebar props/events, and provides a filtered child bridge for the already-migrated read-only components. Stale authoring requests are rejected before the legacy callback, one underlying legacy subscription is released on unmount, and no composable inspects authoring DOM. `overseer.js` now publishes and replays one atomic authoring projection and uses one request revision; the five separate selection/group/editor/tree/create projection slots and revision counters have been removed.

**T084 broadcast-controls migration**: `BroadcastControls.vue` exclusively owns `#coordinationPanel`, broadcast summary, start/end/take-off triggers, logical-session summary/trigger, coordination feedback, and the nested player-configuration Teleport target through `#broadcastControlsVueLeaf`. `useBroadcastControls` accepts non-decreasing authoritative coordination revisions so same-revision pending/status projections remain visible while older state is rejected; it owns command pending state, confirmation requests, focus requests, and the single bridge subscription cleanup. `overseer.js` retains shared coordination-domain convergence and temporary legacy confirmation dialogs, but contains no migrated broadcast-control query, mutation, or event handler. BUG-020 records the App/document integration seam required for exclusive mounting.

**T085 broadcast-confirmation migration**: `EndBroadcastDialog.vue` and `TakeOffAirDialog.vue` are the sole owners of their native dialog elements, descriptions, errors, controls, initial focus, Escape behavior, and close-on-unmount cleanup. The shared `useBroadcastControls` lifecycle token makes each completion applicable only while mounted, on the same confirmation generation, and at the same authoritative coordination revision; duplicate submits are pending-gated, stale results cannot close or refocus newer state, and clear failures keep the take-off dialog open with confirm focus. `overseer.js` contains no query, modal operation, field mutation, or listener for either migrated dialog. BUG-021 records why the async lifecycle owner had to be writable in this integration slice.

**T086 hacking-controls migration**: `HackControlPanel.vue` exclusively owns `#hackStatus`, its status line, force-success and failed-hack reset controls, and local error presentation through `#hackControlsVueLeaf`. `useHackControls` consumes the authoritative hack stream directly, assigns monotonic receipt revisions to production snapshots, rejects older explicit revisions, clears state across live-terminal identity changes, and never mutates hack state from command results. Commands are synchronously pending-gated; late completions cannot affect a newer terminal, coordination revision, hack revision, or unmounted application. Reset payloads come from a detached live-terminal context projection, and the hack event plus bridge subscriptions release on unmount. `overseer.js` retains only coordination convergence and detached live-terminal context; it has no hack-state subscription, control query, renderer, mutation, or event handler. BUG-020 records the App/document integration seam required for exclusive mounting.

## Wave e — Complete Overseer cutover and remove its legacy bootstrap

**BUG-023 cutover correction**: The production root cannot replace the coexistence roots until T089 installs one DOM-free typed application controller that owns every remaining projection/action retained in `overseer.js` and converts every bridge consumer. T090 cannot delete the candidate entry or port-34120 selection until every retained Overseer suite mounts the production `#overseerApp` through permanent test-only infrastructure. The one-root final boundary and all eight unconditional removal rows remain unchanged.

**T089 production promotion**: Production now selects exactly one `#overseerApp`, `main.ts`, and `mountOverseerApp` path. `controllers/overseer-controller.ts` owns the remaining session, terminal-authoring, grouping, coordination, reset-correlation, dialog-orchestration, public-access ordering, autosave, and privileged-command projections/actions without DOM access; every production component/composable formerly importing the coexistence bridge injects this controller. Candidate-mode and legacy files remain unselected and ledgered only for unconditional T090 removal and retained-suite harness migration.

**T090 unconditional removal**: All eight Overseer temporary mechanisms are closed. The bounded compiler, combined candidate document/entry, candidate Vite selection, candidate Playwright route/smoke, coexistence roots, typed callbacks, legacy script tags, and legacy lifecycle modules are absent. Retained browser suites now load the production `#overseerApp` through `tests/browser/fixtures/overseer-app.ts`; its browser-test adapter aliases are excluded from production builds and native evidence.

**Vue mount boundaries**: The single production `#overseerApp` root owns the entire rendered Overseer application: start/runtime status, application shell, terminal/groups, authoring, coordination, approvals, broadcast/hacking controls, public access, updates, and all dialogs.

**Legacy-owned adjacent subtrees**: None in the Overseer document. Player remains wholly legacy-owned in its separate document.

**Remaining legacy files and handlers**: Player `client.js`, `sound.js`, and `presentation-uplink.js` only. No Overseer legacy application file, dynamic renderer, global facade, mount island, or document handler remains. Generated Wails JavaScript remains adapter input and is not a DOM owner.

**Prohibited boundaries**: Overseer components may use DOM access only through approved Vue-owned focus/measurement directives or composables. No component imports generated Wails code or `@wailsio/runtime` outside `adapters/desktop-api.ts`. Player remains unable to import Overseer or native capabilities.

**Focused checks**:

- Overseer independent and workspace strict type-checks; Overseer production build.
- Complete Overseer Playwright suite with unchanged selectors and visual expectations.
- Focus restoration, keyboard, pointer, modal, stale-result, revision, confirmation atomicity, secret, clipboard, update, public-access, session, player, terminal, group, authoring, approval, broadcast, and hacking-control journeys.
- `task frontend:compatibility:check` opens, edits, saves, reopens, and compares the reviewed current/legacy session and player-configuration fixtures, compatible unknown fields, defaults, references, locations, and business meaning.
- The complete desktop/Wails subset of the reviewed frontend boundary manifest passes before Overseer cutover is accepted.
- Wails binding integrity, embedding, startup, native accessibility/dialog behavior, resources, secret checks, and package checks as separate evidence.

**Removal criteria**: Delete `frontend/overseer/src/overseer.js`, `desktop-api.js`, legacy script tags, `#legacyOverseerRoot`, `#overseerVueLeaves`, temporary bridges, temporary mounts, and Overseer legacy-check configuration. The final forbidden-state scan must pass for `frontend/overseer/src` before wave f starts.

**Wave-e exit evidence (T106)**: The complete Wave-e gate passed on 2026-08-31. Production Overseer has one `#overseerApp` root and zero legacy bootstrap, candidate-selection, coexistence-root, bridge, or bounded-legacy-compiler mechanisms. Strict/build/compatibility/policy/reproducibility, all 225 unconditional browser tests, immutable visuals, the 39-method/seven-event binding boundary, Wails/secret/license scanners, focused platform and Player Go gates, Linux verifier self-test, macOS arm64 package verification, and the real packaged approval/reset/convergence/reopen journey passed. The two credential-qualified real-ngrok tests and the Windows matching-host package verifier are the only conditional checks not run; their reasons and follow-up are recorded. Player production remains wholly legacy-owned, its candidate rows remain governed through T156, and no Player feature task ran before this checkpoint.

## Wave f — Player shell, identity, transport, session, navigation, and presentation foundations

Wave f is the first wave permitted to implement Player business behavior. It cannot begin until every wave-e Overseer exit condition, including removal and absence checks, has passed.

**T107 candidate selection**: Explicit Vite `candidate` mode selects only `frontend/client/test-fixtures/index.html` and its `candidate-main.ts` entry, and the Playwright-only server at `127.0.0.1:34120` exposes that isolated document. Production mode continues to select `frontend/client/index.html` and legacy `client.js`; candidate selection is browser/test evidence only and remains governed by the T107/T156 temporary-register rows. Native embed/resource selection remains deferred to T165/T181 and package-content evidence to T168/T182.

**T124 candidate lifecycle integration**: The isolated `#playerApp` now composes one recognition lease, one snapshot-first subscription, typed projection/identity/authority/action/navigation state, and the shell/identity/menu/footer leaves under a single Vue scope. Generated public ConnectRPC and the validated recognition-storage adapter are injected at the mount boundary; candidate tests may replace only those two explicit ports. First unmount aborts and returns the physical stream and releases the storage listener, and repeated teardown adds no cleanup. Production remains separately selected and wholly legacy-owned.

**Vue mount boundaries**:

- Production Player route remains one legacy-owned document with no Vue mount.
- A separate candidate Player document mounts `#playerApp`; Vue owns its complete candidate subtree, including shell, connection overlay, identity/roster/role/controller presentation, terminal list/entry/navigation/pagination, and foundational presentation state.

**Legacy-owned adjacent subtrees**: None inside the candidate document. The production legacy document is separate and wholly owns `.crt`, `#screen`, `#connOverlay`, and descendants.

**Remaining legacy files and handlers**: Production `client.js`, `sound.js`, and `presentation-uplink.js` remain. Candidate Vue has not yet accepted hacking-board, typewriter/CRT timing, sound, or presentation-uplink parity.

**Prohibited boundaries**: Candidate and production documents cannot share state through DOM or a runtime store. Candidate transport uses only generated public TypeScript and ConnectRPC. No Wails/native/Overseer dependency is allowed. Canonical gameplay mutations remain server requests; no optimistic canonical state enters Vue.

**Focused checks**:

- Player independent and workspace strict type-checks and candidate build.
- Candidate first connection, complete-snapshot-first subscription, fixed reconnect, stale revision suppression, pending ActionResult correlation, multi-tab recognition/lease, roster/role/controller, navigation, pagination, cancellation, and public/private graph tests.
- Reviewed localStorage/storage-event, decoded-network, DOM/form, and navigation-input boundary-manifest entries have focused test mappings and expected accept/reject plus trusted-projection/no-state-change outcomes.
- Legacy production full browser/visual suite continues to pass until candidate parity is complete.
- Candidate DOM selector/accessibility checks compare against the immutable production fixture without updating baselines.

**Removal criteria**: Candidate-only test selection expires in wave h. Foundations advance to wave g only after their complete focused parity suite passes and all timers, subscriptions, AbortControllers, streams, observers, and listeners have registered scope cleanup.

**Wave-f exit evidence (T132)**: Player strict compilation, the exact isolated candidate build and four-file runtime inventory, public-only dependency policy, all 10 transport/recognition/navigation/cleanup candidate tests, all 41 unconditional legacy production ConnectRPC/CRT tests, and immutable snapshot diff passed on 2026-09-01. The two credential-qualified real-ngrok tests were skipped for missing endpoint credentials. Boundary mappings cover valid and invalid decoded-network, recognition-storage, DOM-action, and navigation inputs with no-state-change rejection. Every wave-f AbortController, iterator, reconnect/lease timer, storage listener, observer, RAF/font callback, and focus job has exact teardown evidence. Production remains wholly legacy-selected; Player candidate/staging/Vite/Playwright/legacy temporary rows stay open through T156.

## Wave g — Player hacking, CRT/typewriter, sound, and presentation-uplink integration

**Vue mount boundaries**: Same two separate documents as wave f. Candidate `#playerApp` now owns its entire candidate document, including hacking, CRT/typewriter, sound, pointer/keyboard geometry, measurement, and presentation uplink. Production remains wholly legacy-owned.

**Legacy-owned adjacent subtrees**: None within either document; each document has one complete owner.

**Remaining legacy files and handlers**: Production `client.js`, `sound.js`, and `presentation-uplink.js` remain solely for the legacy production route until atomic cutover.

**Prohibited boundaries**: Vue composables may imperatively access only their owned element refs or approved browser APIs. No composable may independently render Vue descendants. Controller-local presentation stays transient and context-keyed; observers remain read-only. Streaming cannot replace or weaken unary fallback.

**Focused checks**:

- Hacking target geometry, fitting modes, focus reconciliation, pointer grouping, keyboard input, patterns, attempts/outcomes, and no optimistic state.
- Exact 40ms typewriter progression, completion/cancel/repeat behavior, cue de-duplication, pagination/measurement, and CRT snapshots at every existing viewport.
- Gesture-gated Web Audio, manifest validation, ambient and one-shot volumes/cues, failure isolation, teardown, and no replay after reconnect/rejection.
- Streaming capability probe, ready/result correlation, latest-value mailbox, request cancellation, retry timing, authoritative convergence, stale-result rejection, and unary fallback.
- Reviewed pointer/keyboard-derived, sound-manifest/asset, and presentation-stream capability/result boundary-manifest entries have focused test mappings and expected accept/reject plus trusted-projection/no-state-change outcomes.
- Complete candidate Player `.mjs` and visual suite without baseline changes; legacy production suite still passes.

**Removal criteria**: Every candidate lifecycle resource has an automated cleanup assertion or observable unmount/reconnect proof. Candidate reaches complete parity and is eligible for the single wave-h production ownership transfer.

**Candidate integration evidence (T151)**: The single candidate `#playerApp` now coordinates authoritative hacking projection/actions, board geometry and input, the 40ms reveal/CRT state, gesture-owned sound, and context-correlated presentation uplink through Vue-scope owners. The focused integration fixture proves the candidate entry has one lifecycle owner and unmount releases subscription and document resources without cross-root mutation.

**Wave-g exit evidence (T154)**: Player strict compilation, the exact 217-module candidate build, policy checks, 11 boundary tests, 26 complete candidate tests, and 41 unconditional legacy ConnectRPC/CRT tests all passed on 2026-09-01. The two credential-qualified real-ngrok journeys were skipped because their external endpoint credentials were unavailable. All immutable snapshots stayed clean. Wave g is closed and only the atomic wave-h production cutover is authorized; every Player temporary mechanism remains registered until unconditional T156 removal.

**T155 atomic production promotion**: `frontend/client/index.html` now selects one `#playerApp` and `/src/main.ts`; `main.ts` performs exactly one `mountPlayerApp` call through the production RPC and recognition-storage adapters. Vue exclusively owns `.crt`, `#screen`, `#connOverlay`, and their descendants in the production document. The old candidate document, selection, declarations, and legacy files remain unselected and ledgered only through unconditional T156 removal.

**T156 unconditional Player removal**: All nine Player mechanisms are closed: bounded legacy compiler; candidate document; candidate entry; alternate Vite selection; alternate Playwright selection and route smoke; staging mount; staging bridge; legacy script tags; and legacy lifecycle owners. Production and browser test serving now select the same Vue root, and no removable Player mechanism remains.

## Wave h — Complete Player cutover

**Vue mount boundaries**: The single production `#playerApp` root owns the entire Player document, including `.crt`, `#screen`, `#connOverlay`, all terminal/identity/hacking/footer/effect descendants, and all input/audio/streaming lifecycles.

**Legacy-owned adjacent subtrees**: None in either production application.

**Remaining legacy files and handlers**: None in production application source. Generated Wails JavaScript and browser `.mjs` tests remain governed exclusions, not legacy application modules.

**Prohibited boundaries**: No legacy query, render, handler, timer, observer, subscription, stream, or listener may remain. Player cannot import Wails, `@wailsio/runtime`, bindings, native capabilities, private types, Overseer application source/state, or a shared cross-boundary store.

**Focused checks**:

- Player independent and workspace strict type-checks and production build.
- Complete unchanged Player and cross-application Playwright suite plus all immutable visual snapshots.
- Reconnect, multi-tab, revision, pending-action, authority, navigation, hacking, CRT/typewriter, geometry, sound, streaming, cancellation, fallback, and cleanup stress journeys.
- Every Player-owned entry in the reviewed frontend boundary manifest has its focused test mapping and passes against the production Player root.
- Production bundle capability scan and separate native Player serving/startup/package probes.

**Removal criteria**: Delete `frontend/client/client.js`, `sound.js`, `presentation-uplink.js`, old script tag, candidate entry/selector, any staging root or bridge, and Player legacy-check configuration. Final Player ownership inventory is empty before wave i.

## Wave i — Final strict cleanup, verification, packaging, and documentation

**Vue mount boundaries**: `#overseerApp` owns the complete privileged document; `#playerApp` owns the complete public document. The roots are in separate bundles and trust domains.

**Legacy-owned adjacent subtrees**: None.

**Remaining legacy files and handlers**: None. The only JavaScript in governed exceptions is generator-owned Wails output, dependency/build output, and `tests/browser/*.mjs`.

**Prohibited boundaries**: Final scans reject handwritten production `.js`, legacy bootstraps, temporary mount switches, candidate entries, mixed ownership, `allowJs`, `checkJs`, broad `any`, `@ts-nocheck`, blanket assertions, unexplained suppressions, and forbidden Player dependency paths. Generated Wails bindings, dependencies, build output, and `tests/browser/*.mjs` are explicitly excluded only from applicable scans.

**Focused checks**:

- `task frontend:build` performs the one clean workspace installation and both builds; `task frontend:build:overseer` and `task frontend:build:client` retain independent no-install build ownership.
- `task frontend:typecheck`, `task frontend:typecheck:overseer`, and `task frontend:typecheck:client` run workspace and per-app strict `vue-tsc` checks without installing dependencies.
- `task frontend:compatibility:check` reruns the full FR-023/SC-007 reviewed current/legacy persistence fixture set.
- `task frontend:boundary:check` runs every reviewed valid/invalid boundary-manifest entry and rejects missing test mappings.
- `task frontend:policy:check` proves forbidden-source/type, one-lockfile, Player-boundary, temporary-mechanism, and final-cutover policy.
- `task frontend:reproducible:check` performs both Vite builds twice and emits actionable byte-identical tree-digest evidence.
- Protobuf format/lint/breaking/generation/drift/strict compilation; Wails binding generation/integrity.
- Complete Playwright and visual suite through production-fidelity fixtures.
- `go fix ./...` before final formatting when Go source changed, then repository Taskfile Go quality/test/race gates.
- Separate native embedding/startup/binding/resource/secret/package-content checks and supported matching-host packages.
- Active README, architecture, contribution, packaging, CI, Taskfile, buildtool, and Spec Kit template checks.
- Credential-dependent, real-provider, signing, notarization, stapling, Gatekeeper, and unavailable matching-host checks are recorded `NOT RUN` unless actually executed.

**Removal criteria**: Zero entries remain in the legacy or temporary inventory; both application roots are sole owners; all final checks pass or conditional checks are honestly recorded `NOT RUN`; documentation describes only the accepted Vue/TypeScript architecture.

**Wave-h exit evidence (T169)**: Player strict compilation, production build, final policy, and the complete 278-journey browser gate passed on 2026-09-01; 276 unconditional journeys passed and the two credential-qualified real-ngrok journeys were skipped. All twelve immutable CRT snapshots remained clean. The matching macOS arm64 package evidence remains PASS, Linux and Windows remain host-qualified `NOT RUN`, and the temporary-mechanism register has no open Overseer or Player mechanism. Wave h is closed and wave i is authorized.

**Wave-i final ownership evidence (T192)**: The final policy gate, exact root counts, register audit,
rollback ancestry check, and historical-spec diff all passed on 2026-09-01. `#overseerApp` and
`#playerApp` are the sole production roots in separate bundles and trust domains. All eight
Overseer mechanisms, all nine Player mechanisms, and the command-local protobuf drift mechanism
are closed; zero legacy/candidate/mixed owner or prohibited type escape remains. The complete
browser gate passed with only two credential-qualified real-provider journeys `NOT RUN`; the
matching darwin/arm64 package passed exact inspection, other matching hosts remain `NOT RUN`, and
interactive macOS native UI remains `NOT RUN` because Accessibility control was unavailable. No
browser result is substituted for native or provider evidence.

## Temporary mechanism register

Every generated creation or removal task must name the corresponding row and reproduce its exact paths. The task IDs below are the current planning bindings; `$speckit-tasks` must update the row and task together if regeneration changes an ID. A newly discovered mechanism cannot be created until a new exact row, owner, expiry, removal task, and executable absence check are added.

| Mechanism | Owning file and selector/root/entry/config | Creation task | Owner and permitted scope | Expiry | Parity prerequisite task IDs | Unconditional removal task | Absence verification |
|---|---|---|---|---:|---|---|---|
| Overseer bounded legacy compiler program | `frontend/overseer/tsconfig.legacy.json`; exact inputs `frontend/overseer/src/overseer.js`, `frontend/overseer/src/desktop-api.js` | T007 | Frontend Migration; compile only the two named legacy modules | e | T073, T074, T087, T088, T089 | T090 | `test ! -e frontend/overseer/tsconfig.legacy.json` and `task frontend:policy:check` |
| Player bounded legacy compiler program | `frontend/client/tsconfig.legacy.json`; exact inputs `frontend/client/client.js`, `frontend/client/sound.js`, `frontend/client/presentation-uplink.js` | T008 | Frontend Migration; compile only the three named legacy modules | h | T130, T131, T132, T152, T153, T154, T155 | T156 | `test ! -e frontend/client/tsconfig.legacy.json` and `task frontend:policy:check` |
| Overseer candidate document and entry | `frontend/overseer/test-fixtures/index.html`; `#overseerApp`; `frontend/overseer/test-fixtures/candidate-main.ts`; imports `frontend/overseer/src/mount.ts` | T038 | Frontend Migration; isolated browser/test route only | e | T043, T044, T073, T074, T087, T088, T089 | T090 | `test ! -e frontend/overseer/test-fixtures/index.html`, `test ! -e frontend/overseer/test-fixtures/candidate-main.ts`, and `task frontend:policy:check` |
| Overseer candidate Vite selection | `frontend/overseer/vite.config.ts`; candidate input `frontend/overseer/test-fixtures/index.html` | T039 | Frontend Migration; test build only, never the embedded production input | e | T043, T073, T074, T087, T088, T089 | T090 | `scripts/frontend-assert-no-match.sh 'candidate([-_/.]*(main|overseer|index\.html))|overseer[-_/.]*candidate|test[-_/.]*fixtures' frontend/overseer/vite.config.ts tests/browser/playwright.config.mjs` and `task frontend:policy:check` |
| Overseer candidate Playwright selection | `tests/browser/playwright.config.mjs`; Overseer candidate project/route | T039 | Frontend Migration; browser parity only, never native evidence | e | T044, T073, T074, T087, T088, T089 | T090 | `scripts/frontend-assert-no-match.sh 'candidate([-_/.]*(main|overseer|index\.html))|overseer[-_/.]*candidate|test[-_/.]*fixtures' frontend/overseer/vite.config.ts tests/browser/playwright.config.mjs` and `task frontend:policy:check` |
| Overseer coexistence roots | `frontend/overseer/src/index.html`; `#overseerVueLeaves` and `#legacyOverseerRoot` | T046 | Frontend Migration; complete reviewed leaf subtrees only | e | T069, T070, T071, T072, T073, T074, T087, T088, T089 | T090 | `scripts/frontend-assert-no-match.sh 'overseerVueLeaves|legacyOverseerRoot|overseer\.js|desktop-api\.js|typedCoexistence|legacyToVue|vueToLegacy|temporaryMount|coexistenceBridge|mountOverseerLeaves|legacyOverseer|overseerVue' frontend/overseer/src/index.html frontend/overseer/src/main.ts frontend/overseer/src/mount.ts` and `task frontend:policy:check` |
| Overseer typed legacy/Vue callbacks | `frontend/overseer/src/main.ts`; `__overseerCoexistenceBridge`; `frontend/overseer/src/mount.ts`; `frontend/overseer/src/overseer.js`; leaf request/state callbacks | T046 | Frontend Migration; typed detached data/callback crossing only, no cross-root DOM access | e | T069, T070, T071, T072, T073, T074, T087, T088, T089 | T090 | `test ! -e frontend/overseer/src/overseer.js`, `scripts/frontend-assert-no-match.sh 'overseerVueLeaves|legacyOverseerRoot|overseer\.js|desktop-api\.js|typedCoexistence|legacyToVue|vueToLegacy|temporaryMount|coexistenceBridge|mountOverseerLeaves|legacyOverseer|overseerVue' frontend/overseer/src/index.html frontend/overseer/src/main.ts frontend/overseer/src/mount.ts`, and `task frontend:policy:check` |
| Overseer legacy script tags | `frontend/overseer/src/index.html`; `overseer.js`, `desktop-api.js` script entries | T001 | Frontend Migration; existing production ownership until atomic Overseer cutover | e | T073, T074, T087, T088, T089 | T090 | `scripts/frontend-assert-no-match.sh 'overseerVueLeaves|legacyOverseerRoot|overseer\.js|desktop-api\.js|typedCoexistence|legacyToVue|vueToLegacy|temporaryMount|coexistenceBridge|mountOverseerLeaves|legacyOverseer|overseerVue' frontend/overseer/src/index.html frontend/overseer/src/main.ts frontend/overseer/src/mount.ts` and `task frontend:policy:check` |
| Overseer legacy lifecycle owners | `frontend/overseer/src/overseer.js`, `frontend/overseer/src/desktop-api.js`; remaining document/window listeners, timers, animation frames, observers, subscriptions, and temporary dialog nodes; application-update ownership removed in T048, shared Vue focus cleanup established in T050, command-approval ownership removed in T052, terminal-navigation approval ownership removed in T053, terminal-switch dialog/listener ownership removed in T054, command-state reset dialog/listener ownership removed in T055, session-document control ownership removed in T057, logical-session dialog/row ownership removed in T058, player-configuration control ownership removed in T059, player-management dialog/profile-row ownership removed in T060, player-delete confirmation ownership removed in T061, terminal-group draft/impact dialog ownership removed in T062, public-access status subscription/panel ownership removed in T064, public-access settings dialog/form ownership removed in T065, provider-token dialog/secret ownership removed in T066, player-credential dialog/share/generate ownership removed in T067, and generated-password/clipboard ownership removed in T068 | T001 | Frontend Migration; existing legacy root only; each migrated slice deletes matching acquisition and cleanup | e | T069, T070, T071, T072, T073, T074, T087, T088, T089 | T090 | `test ! -e frontend/overseer/src/overseer.js`, `test ! -e frontend/overseer/src/desktop-api.js`, `scripts/frontend-assert-no-match.sh 'legacy.*(addEventListener|setTimeout|setInterval|requestAnimationFrame|ResizeObserver|MutationObserver|subscribe|unsubscribe|dialog)|legacy(Dialog|Listener|Timer|Frame|Observer|Subscription)|temporary.*(Listener|Timer|Frame|Observer|Subscription|Dialog)' frontend/overseer/vite.config.ts tests/browser/playwright.config.mjs frontend/overseer/src/index.html frontend/overseer/src/main.ts frontend/overseer/src/mount.ts`, and the two T090 exact focused production-root/cleanup checks |
| Player candidate document | `frontend/client/test-fixtures/index.html`; `#playerApp` | T041 | Frontend Migration; empty capability-neutral shell in c, candidate feature work only in f–g | h | T130, T131, T132, T152, T153, T154, T155 | T156 | `test ! -e frontend/client/test-fixtures/index.html` and `task frontend:policy:check` |
| Player candidate entry | `frontend/client/test-fixtures/candidate-main.ts`; imports `frontend/client/src/mount.ts` | T041 | Frontend Migration; empty capability-neutral shell in c, candidate feature work only in f–g | h | T130, T131, T132, T152, T153, T154, T155 | T156 | `test ! -e frontend/client/test-fixtures/candidate-main.ts` and `task frontend:policy:check` |
| Player candidate Vite selection | `frontend/client/vite.config.ts`; candidate input `frontend/client/test-fixtures/index.html` | T107 | Frontend Migration; candidate build only, never public production selection before h | h | T130, T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'candidate([-_/.]*(main|player|index\.html))|player[-_/.]*candidate|test[-_/.]*fixtures' frontend/client/vite.config.ts tests/browser/playwright.config.mjs` and `task frontend:policy:check` |
| Player candidate Playwright selection | `tests/browser/playwright.config.mjs`; Player candidate project/route | T107 | Frontend Migration; candidate browser parity only | h | T126, T127, T128, T129, T130, T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'candidate([-_/.]*(main|player|index\.html))|player[-_/.]*candidate|test[-_/.]*fixtures' frontend/client/vite.config.ts tests/browser/playwright.config.mjs` and `task frontend:policy:check` |
| Player staging mount | `frontend/client/src/mount.ts`; candidate-only `#playerApp` mount selection | T040 | Frontend Migration; isolated candidate document only until atomic h cutover | h | T124, T130, T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'client\.js|sound\.js|presentation-uplink\.js|stagingMount|candidateMount|candidateBridge|stagingBridge|candidateSelection|temporaryMount|legacyPlayerRoot|playerLegacy' frontend/client/index.html frontend/client/src/main.ts frontend/client/src/mount.ts` and the T156 exact focused production-root check |
| Player staging bridge | `frontend/client/src/mount.ts`; candidate-only mount-options/selection bridge | T040 | Frontend Migration; capability-neutral candidate mount options only until atomic h cutover | h | T124, T130, T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'client\.js|sound\.js|presentation-uplink\.js|stagingMount|candidateMount|candidateBridge|stagingBridge|candidateSelection|temporaryMount|legacyPlayerRoot|playerLegacy' frontend/client/index.html frontend/client/src/main.ts frontend/client/src/mount.ts` and the T156 exact focused cleanup check |
| Player legacy script tags | `frontend/client/index.html`; `client.js`, `sound.js`, `presentation-uplink.js` script entries | T001 | Frontend Migration; existing public production ownership through g | h | T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'client\.js|sound\.js|presentation-uplink\.js|stagingMount|candidateMount|candidateBridge|stagingBridge|candidateSelection|temporaryMount|legacyPlayerRoot|playerLegacy' frontend/client/index.html frontend/client/src/main.ts frontend/client/src/mount.ts` and `task frontend:policy:check` |
| Player legacy lifecycle owners | `frontend/client/client.js`, `frontend/client/sound.js`, `frontend/client/presentation-uplink.js`; listeners, timers, animation frames, observers, subscriptions, streams/iterators, AbortControllers, and audio owners | T001 | Frontend Migration; production legacy document only; no sharing with candidate | h | T129, T131, T132, T152, T153, T154, T155 | T156 | `test ! -e frontend/client/client.js`, `test ! -e frontend/client/sound.js`, `test ! -e frontend/client/presentation-uplink.js`, `scripts/frontend-assert-no-match.sh 'legacy.*(addEventListener|setTimeout|setInterval|requestAnimationFrame|ResizeObserver|MutationObserver|subscribe|stream|AbortController|AudioContext)|legacy(Player|Listener|Timer|Frame|Observer|Subscription|Stream|Audio)|temporary.*(Listener|Timer|Frame|Observer|Subscription|Stream|Abort|Audio)' frontend/client/vite.config.ts tests/browser/playwright.config.mjs frontend/client/index.html frontend/client/src/main.ts frontend/client/src/mount.ts`, and the T156 exact focused cleanup check |
| Deliberate protobuf drift mutation | `frontend/client/gen/fallout/terminal/player/v1/player_pb.ts`; self-test-only content mutation | T020 | Frontend Migration; only inside the governed drift command and never committed | b, same command | T020 | T020 | `target=frontend/client/gen/fallout/terminal/player/v1/player_pb.ts && test -f "$target" && test -r "$target" && before_hash="$(git hash-object "$target")" && scripts/proto-drift-test.sh --target "$target" --expect-diagnostic 'generated protobuf drift: frontend/client/gen/fallout/terminal/player/v1/player_pb.ts' && test -f "$target" && test -r "$target" && test "$(git hash-object "$target")" = "$before_hash" && git diff --exit-code -- "$target"` |

T020 closed the deliberate protobuf drift mechanism on 2026-08-30 in the same governed command that created it. The exact expected diagnostic named `frontend/client/gen/fallout/terminal/player/v1/player_pb.ts`; the pre/post Git object hash was `6fa19fd589ad213eef2d7eed9f338004ee5c1d68`, and the complete owned protobuf input/output manifest was unchanged after restoration.

## Retained browser evidence and test infrastructure

The typed fake `DesktopPort` used by the browser fixtures is permanent test-only evidence infrastructure, not a temporary production compatibility mechanism. It lives outside production source and bundles, remains after wave e, is never embedded or packaged, and is verified against the production `DesktopPort` contract. It may prove browser DOM/application parity only and is explicitly excluded from native Wails claims. No temporary-mechanism register row may combine it with an expiring candidate entrypoint, selector, mount, or build selection.
