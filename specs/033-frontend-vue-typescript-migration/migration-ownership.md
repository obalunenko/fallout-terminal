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

**T041 Player candidate activation**: `frontend/client/test-fixtures/candidate-index.html` solely owns its isolated `#playerApp` root through `candidate-main.ts` and `mountPlayerCandidate`. The entry supplies only the capability-neutral idle transport required by the empty shell. Production Player selection remains the legacy document and `client.js`; the candidate document/entry remain governed by the T041/T156 temporary-register rows.

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

## Wave e — Complete Overseer cutover and remove its legacy bootstrap

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

## Wave f — Player shell, identity, transport, session, navigation, and presentation foundations

Wave f is the first wave permitted to implement Player business behavior. It cannot begin until every wave-e Overseer exit condition, including removal and absence checks, has passed.

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
| Overseer typed legacy/Vue callbacks | `frontend/overseer/src/overseer.js`, `frontend/overseer/src/mount.ts`; leaf request/state callbacks | T046 | Frontend Migration; typed data/callback crossing only, no cross-root DOM access | e | T069, T070, T071, T072, T073, T074, T087, T088, T089 | T090 | `test ! -e frontend/overseer/src/overseer.js`, `scripts/frontend-assert-no-match.sh 'overseerVueLeaves|legacyOverseerRoot|overseer\.js|desktop-api\.js|typedCoexistence|legacyToVue|vueToLegacy|temporaryMount|coexistenceBridge|mountOverseerLeaves|legacyOverseer|overseerVue' frontend/overseer/src/index.html frontend/overseer/src/main.ts frontend/overseer/src/mount.ts`, and `task frontend:policy:check` |
| Overseer legacy script tags | `frontend/overseer/src/index.html`; `overseer.js`, `desktop-api.js` script entries | T001 | Frontend Migration; existing production ownership until atomic Overseer cutover | e | T073, T074, T087, T088, T089 | T090 | `scripts/frontend-assert-no-match.sh 'overseerVueLeaves|legacyOverseerRoot|overseer\.js|desktop-api\.js|typedCoexistence|legacyToVue|vueToLegacy|temporaryMount|coexistenceBridge|mountOverseerLeaves|legacyOverseer|overseerVue' frontend/overseer/src/index.html frontend/overseer/src/main.ts frontend/overseer/src/mount.ts` and `task frontend:policy:check` |
| Overseer legacy lifecycle owners | `frontend/overseer/src/overseer.js`, `frontend/overseer/src/desktop-api.js`; document/window listeners, timers, animation frames, observers, subscriptions, and temporary dialog nodes | T001 | Frontend Migration; existing legacy root only; each migrated slice deletes matching acquisition and cleanup | e | T048, T050, T052, T053, T054, T055, T057, T058, T059, T060, T061, T062, T064, T065, T066, T067, T068, T069, T070, T071, T072, T073, T074, T087, T088, T089 | T090 | `test ! -e frontend/overseer/src/overseer.js`, `test ! -e frontend/overseer/src/desktop-api.js`, `scripts/frontend-assert-no-match.sh 'legacy.*(addEventListener|setTimeout|setInterval|requestAnimationFrame|ResizeObserver|MutationObserver|subscribe|unsubscribe|dialog)|legacy(Dialog|Listener|Timer|Frame|Observer|Subscription)|temporary.*(Listener|Timer|Frame|Observer|Subscription|Dialog)' frontend/overseer/vite.config.ts tests/browser/playwright.config.mjs frontend/overseer/src/index.html frontend/overseer/src/main.ts frontend/overseer/src/mount.ts`, and the two T090 exact focused production-root/cleanup checks |
| Player candidate document | `frontend/client/test-fixtures/candidate-index.html`; `#playerApp` | T041 | Frontend Migration; empty capability-neutral shell in c, candidate feature work only in f–g | h | T130, T131, T132, T152, T153, T154, T155 | T156 | `test ! -e frontend/client/test-fixtures/candidate-index.html` and `task frontend:policy:check` |
| Player candidate entry | `frontend/client/test-fixtures/candidate-main.ts`; imports `frontend/client/src/mount.ts` | T041 | Frontend Migration; empty capability-neutral shell in c, candidate feature work only in f–g | h | T130, T131, T132, T152, T153, T154, T155 | T156 | `test ! -e frontend/client/test-fixtures/candidate-main.ts` and `task frontend:policy:check` |
| Player candidate Vite selection | `frontend/client/vite.config.ts`; candidate input `frontend/client/test-fixtures/candidate-index.html` | T107 | Frontend Migration; candidate build only, never public production selection before h | h | T130, T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'candidate([-_/.]*(main|player|index\.html))|player[-_/.]*candidate|test[-_/.]*fixtures' frontend/client/vite.config.ts tests/browser/playwright.config.mjs` and `task frontend:policy:check` |
| Player candidate Playwright selection | `tests/browser/playwright.config.mjs`; Player candidate project/route | T107 | Frontend Migration; candidate browser parity only | h | T126, T127, T128, T129, T130, T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'candidate([-_/.]*(main|player|index\.html))|player[-_/.]*candidate|test[-_/.]*fixtures' frontend/client/vite.config.ts tests/browser/playwright.config.mjs` and `task frontend:policy:check` |
| Player staging mount | `frontend/client/src/mount.ts`; candidate-only `#playerApp` mount selection | T040 | Frontend Migration; isolated candidate document only until atomic h cutover | h | T124, T130, T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'client\.js|sound\.js|presentation-uplink\.js|stagingMount|candidateMount|candidateBridge|stagingBridge|candidateSelection|temporaryMount|legacyPlayerRoot|playerLegacy' frontend/client/index.html frontend/client/src/main.ts frontend/client/src/mount.ts` and the T156 exact focused production-root check |
| Player staging bridge | `frontend/client/src/mount.ts`; candidate-only mount-options/selection bridge | T040 | Frontend Migration; capability-neutral candidate mount options only until atomic h cutover | h | T124, T130, T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'client\.js|sound\.js|presentation-uplink\.js|stagingMount|candidateMount|candidateBridge|stagingBridge|candidateSelection|temporaryMount|legacyPlayerRoot|playerLegacy' frontend/client/index.html frontend/client/src/main.ts frontend/client/src/mount.ts` and the T156 exact focused cleanup check |
| Player legacy script tags | `frontend/client/index.html`; `client.js`, `sound.js`, `presentation-uplink.js` script entries | T001 | Frontend Migration; existing public production ownership through g | h | T131, T132, T152, T153, T154, T155 | T156 | `scripts/frontend-assert-no-match.sh 'client\.js|sound\.js|presentation-uplink\.js|stagingMount|candidateMount|candidateBridge|stagingBridge|candidateSelection|temporaryMount|legacyPlayerRoot|playerLegacy' frontend/client/index.html frontend/client/src/main.ts frontend/client/src/mount.ts` and `task frontend:policy:check` |
| Player legacy lifecycle owners | `frontend/client/client.js`, `frontend/client/sound.js`, `frontend/client/presentation-uplink.js`; listeners, timers, animation frames, observers, subscriptions, streams/iterators, AbortControllers, and audio owners | T001 | Frontend Migration; production legacy document only; no sharing with candidate | h | T129, T131, T132, T152, T153, T154, T155 | T156 | `test ! -e frontend/client/client.js`, `test ! -e frontend/client/sound.js`, `test ! -e frontend/client/presentation-uplink.js`, `scripts/frontend-assert-no-match.sh 'legacy.*(addEventListener|setTimeout|setInterval|requestAnimationFrame|ResizeObserver|MutationObserver|subscribe|stream|AbortController|AudioContext)|legacy(Player|Listener|Timer|Frame|Observer|Subscription|Stream|Audio)|temporary.*(Listener|Timer|Frame|Observer|Subscription|Stream|Abort|Audio)' frontend/client/vite.config.ts tests/browser/playwright.config.mjs frontend/client/index.html frontend/client/src/main.ts frontend/client/src/mount.ts`, and the T156 exact focused cleanup check |
| Deliberate protobuf drift mutation | `frontend/client/gen/fallout/terminal/player/v1/player_pb.ts`; self-test-only content mutation | T020 | Frontend Migration; only inside the governed drift command and never committed | b, same command | T020 | T020 | `target=frontend/client/gen/fallout/terminal/player/v1/player_pb.ts && test -f "$target" && test -r "$target" && before_hash="$(git hash-object "$target")" && scripts/proto-drift-test.sh --target "$target" --expect-diagnostic 'generated protobuf drift: frontend/client/gen/fallout/terminal/player/v1/player_pb.ts' && test -f "$target" && test -r "$target" && test "$(git hash-object "$target")" = "$before_hash" && git diff --exit-code -- "$target"` |

T020 closed the deliberate protobuf drift mechanism on 2026-08-30 in the same governed command that created it. The exact expected diagnostic named `frontend/client/gen/fallout/terminal/player/v1/player_pb.ts`; the pre/post Git object hash was `6fa19fd589ad213eef2d7eed9f338004ee5c1d68`, and the complete owned protobuf input/output manifest was unchanged after restoration.

## Retained browser evidence and test infrastructure

The typed fake `DesktopPort` used by the browser fixtures is permanent test-only evidence infrastructure, not a temporary production compatibility mechanism. It lives outside production source and bundles, remains after wave e, is never embedded or packaged, and is verified against the production `DesktopPort` contract. It may prove browser DOM/application parity only and is explicitly excluded from native Wails claims. No temporary-mechanism register row may combine it with an expiring candidate entrypoint, selector, mount, or build selection.
