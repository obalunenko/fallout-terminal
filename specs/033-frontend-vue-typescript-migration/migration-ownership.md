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

**Removal criteria**: Temporary `tsconfig.legacy-overseer.json` and `tsconfig.legacy-client.json`, if needed, list their exact JavaScript inputs and cannot be extended implicitly. Overseer legacy checking expires in wave e; Player legacy checking expires in wave h. No `allowJs` or `checkJs` enters `tsconfig.base.json` or either final app configuration.

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

**Removal criteria**: All `_pb.js` generated files and every script/import/probe assumption that checked them are replaced in the same wave. No parallel `target=js`, `target=js+dts`, or checked-in compiled-JavaScript output remains.

## Wave c — Shared declarations, both Vue shells, and typed desktop API

**Vue mount boundaries**:

- Production documents remain wholly legacy-owned.
- `#overseerApp` and `#playerApp` exist and mount only in isolated candidate/fixture documents built from the new application entrypoints.
- The Overseer candidate root receives a typed `DesktopPort`; the production candidate uses the Wails adapter and the browser fixture uses a deterministic fake port.

**Legacy-owned adjacent subtrees**: The complete live Overseer and Player production documents remain legacy-owned; candidate documents are separate documents, never adjacent subtrees in the same document.

**Remaining legacy files and handlers**: All current production JavaScript bootstraps and handlers remain. `desktop-api.js` remains the production bridge until Overseer ownership transfer; `client.js`, `sound.js`, and `presentation-uplink.js` remain Player production owners.

**Prohibited boundaries**: Candidate Vue apps cannot locate or mutate legacy production DOM. Player declarations/config/import graphs cannot reference Wails, `@wailsio/runtime`, bindings, native capabilities, privileged types, Overseer state, or the Overseer candidate. The typed desktop adapter is the only authored import boundary for generated Wails service modules, runtime events, and clipboard.

**Focused checks**:

- Workspace plus independent Overseer/Player strict `vue-tsc` checks.
- Both candidate Vite builds and unchanged legacy production builds.
- Desktop adapter malformed-event/result/clipboard fixtures, listener-before-getter ordering, revision reconciliation, and exact-once unsubscribe tests.
- Player dependency-graph boundary scan.
- Existing production browser/visual suite remains unchanged.

**Removal criteria**: Candidate-only entrypoints are either promoted to the production entrypoint or deleted at their application's cutover. The typed Wails alias declaration remains only as the final adapter boundary and must stay synchronized with binding integrity; no global `window.desktopAPI` compatibility surface survives wave e unless a browser-test-only port implements the same typed interface outside production source.

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
- Wails binding integrity, embedding, startup, native accessibility/dialog behavior, resources, secret checks, and package checks as separate evidence.

**Removal criteria**: Delete `frontend/overseer/src/overseer.js`, `desktop-api.js`, legacy script tags, `#legacyOverseerRoot`, `#overseerVueLeaves`, temporary bridges, temporary mounts, and Overseer legacy-check configuration. The final forbidden-state scan must pass for `frontend/overseer/src` before wave f starts.

## Wave f — Player shell, identity, transport, session, navigation, and presentation foundations

**Vue mount boundaries**:

- Production Player route remains one legacy-owned document with no Vue mount.
- A separate candidate Player document mounts `#playerApp`; Vue owns its complete candidate subtree, including shell, connection overlay, identity/roster/role/controller presentation, terminal list/entry/navigation/pagination, and foundational presentation state.

**Legacy-owned adjacent subtrees**: None inside the candidate document. The production legacy document is separate and wholly owns `.crt`, `#screen`, `#connOverlay`, and descendants.

**Remaining legacy files and handlers**: Production `client.js`, `sound.js`, and `presentation-uplink.js` remain. Candidate Vue has not yet accepted hacking-board, typewriter/CRT timing, sound, or presentation-uplink parity.

**Prohibited boundaries**: Candidate and production documents cannot share state through DOM or a runtime store. Candidate transport uses only generated public TypeScript and ConnectRPC. No Wails/native/Overseer dependency is allowed. Canonical gameplay mutations remain server requests; no optimistic canonical state enters Vue.

**Focused checks**:

- Player independent and workspace strict type-checks and candidate build.
- Candidate first connection, complete-snapshot-first subscription, fixed reconnect, stale revision suppression, pending ActionResult correlation, multi-tab recognition/lease, roster/role/controller, navigation, pagination, cancellation, and public/private graph tests.
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
- Production bundle capability scan and separate native Player serving/startup/package probes.

**Removal criteria**: Delete `frontend/client/client.js`, `sound.js`, `presentation-uplink.js`, old script tag, candidate entry/selector, any staging root or bridge, and Player legacy-check configuration. Final Player ownership inventory is empty before wave i.

## Wave i — Final strict cleanup, verification, packaging, and documentation

**Vue mount boundaries**: `#overseerApp` owns the complete privileged document; `#playerApp` owns the complete public document. The roots are in separate bundles and trust domains.

**Legacy-owned adjacent subtrees**: None.

**Remaining legacy files and handlers**: None. The only JavaScript in governed exceptions is generator-owned Wails output, dependency/build output, and `tests/browser/*.mjs`.

**Prohibited boundaries**: Final scans reject handwritten production `.js`, legacy bootstraps, temporary mount switches, candidate entries, mixed ownership, `allowJs`, `checkJs`, broad `any`, `@ts-nocheck`, blanket assertions, unexplained suppressions, and forbidden Player dependency paths. Generated Wails bindings, dependencies, build output, and `tests/browser/*.mjs` are explicitly excluded only from applicable scans.

**Focused checks**:

- Clean workspace installation; workspace and per-app strict `vue-tsc`; both Vite production builds twice with byte-identical output trees.
- Protobuf format/lint/breaking/generation/drift/strict compilation; Wails binding generation/integrity.
- Complete Playwright and visual suite through production-fidelity fixtures.
- `go fix ./...` before final formatting when Go source changed, then repository Taskfile Go quality/test/race gates.
- Separate native embedding/startup/binding/resource/secret/package-content checks and supported matching-host packages.
- Active README, architecture, contribution, packaging, CI, Taskfile, buildtool, and Spec Kit template checks.
- Credential-dependent, real-provider, signing, notarization, stapling, Gatekeeper, and unavailable matching-host checks are recorded `NOT RUN` unless actually executed.

**Removal criteria**: Zero entries remain in the legacy or temporary inventory; both application roots are sole owners; all final checks pass or conditional checks are honestly recorded `NOT RUN`; documentation describes only the accepted Vue/TypeScript architecture.

## Temporary mechanism register

| Mechanism | Owner | Introduced | Expiry | Parity gate | Mandatory removal |
|---|---|---:|---:|---|---|
| Overseer bounded legacy JS check config | Frontend Migration | a | e | Overseer legacy build and applicable browser suite | Delete config and remove `allowJs`/`checkJs` |
| Player bounded legacy JS check config | Frontend Migration | a | h | Player legacy build and complete Player suite | Delete config and remove `allowJs`/`checkJs` |
| Overseer candidate/test entrypoint and typed fake port | Frontend Migration | c | e | Same SFCs/selectors through production-fidelity fixture | Promote production bootstrap; retain fake only under tests |
| `#overseerVueLeaves` plus typed legacy/Vue state callbacks | Frontend Migration | d | e | Per-leaf browser, focus, visual, revision, and ownership checks | Consolidate into `#overseerApp`; delete legacy root/callbacks |
| Player candidate document/build selection | Frontend Migration | f | h | Complete candidate `.mjs` and visual suite | Promote `#playerApp`; delete candidate selector/entry |
| Any temporary source alias, mount flag, or compatibility switch discovered during implementation | Frontend Migration | owning wave | e for Overseer, h for Player, never later than i | Named focused parity test before merge | Add an explicit task immediately; final scan must reject it |
