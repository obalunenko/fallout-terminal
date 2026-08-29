# Feature Specification: Vue and TypeScript Frontend Migration

**Feature Branch**: `033-frontend-vue-typescript-migration`
**Created**: 2026-08-30
**Status**: Draft
**Input**: Migrate both production frontends to independently owned, strictly typed component applications while preserving every established user, protocol, security, build, and distribution contract.

## Clarifications

### Session 2026-08-30

- Q: How are frontend dependencies installed and locked? → A: One npm workspace rooted at `frontend/`, one committed `frontend/package-lock.json`, and one governed clean-install workflow; each application retains its own workspace manifest and independent type-check and build commands.
- Q: Which exact runtime package and dependency ownership rules apply? → A: Both application manifests declare the same exact compatible `vue` version, resolved once in the shared lockfile; exact direct TypeScript tooling pins use the appropriate workspace ownership without unrelated upgrades.
- Q: Which Vue architecture and prohibited additions are fixed? → A: Vue 3 Single-File Components use the Composition API and `<script setup lang="ts">`; Pinia, Vue Router, Nuxt, JSX, component and CSS frameworks, and a shared cross-boundary application store remain prohibited.
- Q: In what order must the two applications be migrated? → A: Infrastructure, TypeScript protobuf generation, shared declarations and typed desktop API, Overseer components, complete Overseer cutover, Player foundations, Player presentation adapters, complete Player cutover, then final cleanup and verification.
- Q: How is DOM ownership enforced during coexistence? → A: Vue and legacy mount boundaries remain adjacent and disjoint, cross-querying and cross-mutation are prohibited, and each wave records its mount boundaries and remaining legacy ownership.
- Q: Which imperative browser integrations are permitted? → A: Only narrow Vue-owned composables, adapters, or directives for the seven approved browser seams, with lifecycle cleanup for every owned resource and listener.
- Q: Which ConnectRPC behavior may change? → A: None; the existing transport, contracts, limits, authorization, error behavior, authority, streaming, reconnect, backpressure, cancellation, and unary fallback semantics remain unchanged, with no new public operational RPC.
- Q: What may `protoc-gen-es` `target=ts` generation change? → A: Only deterministic generated browser source representation; schemas, descriptors, wire and RPC contracts, Go output, service behavior, and capability boundaries remain unchanged.
- Q: Which JavaScript outputs are generator-owned or test-owned exceptions? → A: Checked-in protobuf TypeScript stays generator-owned, Wails bindings stay in supported generated JavaScript/JSDoc/declaration form, and Wails bindings plus Playwright `.mjs` tests are exempt from the handwritten production-JavaScript prohibition.
- Q: What do browser journeys prove? → A: They prove browser behavior and visual parity through the existing production-fidelity fixtures, not a packaged Wails runtime; native embedding, startup, binding, resource, and package checks remain separate.
- Q: May parity baselines change for Vue? → A: No; existing Playwright snapshots, selectors, copy, CSS, accessibility, focus, input, timing, and audio expectations are immutable migration baselines.
- Q: How is external data validated? → A: Runtime validation remains at browser and Wails boundaries; generated protobuf decoding supplies wire structure while existing adapters and Go services retain semantic validation and authorization.
- Q: What exactly does the final forbidden-state scan cover? → A: Handwritten production source in `frontend/client` and `frontend/overseer/src`, excluding generated Wails bindings, dependencies, build output, and `tests/browser/*.mjs`.
- Q: What does independent application validation mean inside the workspace? → A: Each application can be type-checked and built independently within the one governed npm workspace; it does not get a separate dependency installation or lockfile.
- Q: When must Vite output be byte-identical? → A: Only when Phase 0 proves the existing toolchain reproducibly emits identical bytes; otherwise deterministic sources plus equivalent manifests, content hashes, and asset inventories provide package-integrity evidence.
- Q: What population does malformed-value acceptance measure? → A: The complete explicitly defined malformed-value fixture set, not every theoretically possible invalid value.

## User Scenarios & Testing

### User Story 1 - Players retain the complete terminal experience (Priority: P1)

As a player using the local, LAN, or protected public page, I can connect, reconnect, share a logical session across browser tabs, navigate terminals, control or observe hacking, hear the expected sounds, and receive live presentation changes exactly as before the migration.

**Why this priority**: The public player journey is live gameplay; any behavioral, timing, authority, or presentation regression can interrupt a session or expose privileged state.

**Independent Test**: Run the existing player browser and visual journeys against the migrated public bundle, including a first connection, multi-tab recognition, forced reconnect, navigation, hacking pointer and keyboard input, sound, and presentation streaming, and confirm that all observable behavior and authoritative state transitions match the current production baseline.

**Acceptance Scenarios**:

1. **Given** a first-time player opens the local, LAN, or protected public URL, **When** recognition and subscription complete, **Then** the page shows the same initial state, text, focus, accessibility semantics, timing, and sound behavior as the production baseline.
2. **Given** several tabs in one browser profile start together, **When** they coordinate session initialization, **Then** they converge on one accepted logical session without duplicate initialization or conflicting stored identity.
3. **Given** an established stream disconnects, overflows, or is superseded, **When** the player reconnects, **Then** a complete authoritative snapshot restores current state without replaying stale transitions, cues, actions, or older revisions.
4. **Given** a controller and observers share a broadcast, **When** the controller navigates, points at hacking targets, submits a guess, activates a pattern, or sends presentation intent, **Then** controller-only immediate feedback and authoritative observer convergence retain their established boundaries and ordering.
5. **Given** keyboard-only or pointer input, reduced or changed viewport dimensions, and a previously focused control, **When** the terminal, hacking board, modal state, or navigation state changes, **Then** layout, targeting geometry, keyboard behavior, focus restoration, and accessible names remain equivalent to the current interface.

---

### User Story 2 - The Overseer retains every privileged control (Priority: P1)

As the Overseer, I can operate the desktop interface for broadcast presentation, public-access control, updates, terminal authoring, player and session management, and terminal grouping without relearning the interface or losing any privileged capability.

**Why this priority**: The Overseer is the authoritative control surface for a live game and must remain trustworthy throughout all management and approval workflows.

**Independent Test**: Run every existing Overseer browser and visual journey through the packaged desktop adapter and verify terminal authoring, state-changing approval, players, sessions, groups, public access, update dialogs, clipboard actions, focus behavior, and live broadcast control against the production baseline.

**Acceptance Scenarios**:

1. **Given** an existing saved session and player configuration, **When** the Overseer opens, edits, saves, closes, and reopens them, **Then** all supported content, group, player, and configuration values retain their current meaning and serialized form.
2. **Given** the Overseer performs terminal authoring or grouping operations, **When** confirmation, validation, stale-state, and failure paths occur, **Then** the same atomicity, messages, focus restoration, and no-partial-change guarantees remain in force.
3. **Given** public access is stopped, transitioning, ready, or failed, **When** the Overseer configures credentials, shares player login details, copies addresses, enables access, or stops access, **Then** the established local/LAN isolation, secret handling, modal behavior, and explicit lifecycle controls remain unchanged.
4. **Given** an application update is checking, offered, downloading, verifying, staged, deferred, failed, or ready to restart, **When** the Overseer responds, **Then** the current status, decision, failure isolation, and focus behavior are preserved.
5. **Given** desktop events or command results arrive out of order, malformed, duplicated, or stale, **When** the interface processes them, **Then** only valid and applicable values affect visible state and newer authoritative state is never replaced by older state.

---

### User Story 3 - Operators retain secure deployable applications (Priority: P1)

As an operator or release maintainer, I can build, embed, package, and run the two migrated interfaces on every supported target without changing public/private boundaries, native capabilities, generated contracts, or package contents.

**Why this priority**: A visually correct development build is not releasable unless the desktop and public bundles remain independently secured, embedded, and reproducible in every supported package.

**Independent Test**: Produce clean builds and governed packages for all supported targets, inspect their embedded assets and capability declarations, and exercise local, LAN, and public startup paths while running contract, binding, generation-drift, package-content, and secret-boundary checks.

**Acceptance Scenarios**:

1. **Given** a clean checkout with the governed toolchain, **When** dependencies are installed and both applications are built, **Then** their dependency graph, generated inputs, type checks, and output assets are reproducible with no untracked generation drift.
2. **Given** a desktop package, **When** its contents and runtime capabilities are inspected, **Then** the privileged bundle is embedded in the same role, the public bundle is served in the same role, and neither bundle gains access to the other's private contracts or capabilities.
3. **Given** each supported operating-system and architecture package, **When** package validation and startup checks run, **Then** expected assets are present, obsolete production sources are absent, and application behavior remains supported.
4. **Given** generated browser contracts or native bindings, **When** deterministic regeneration and integrity checks run, **Then** generated output either matches the checked-in files exactly or the check fails with an actionable drift report.

---

### User Story 4 - Maintainers work in two strictly typed component applications (Priority: P2)

As a frontend maintainer, I can change either production interface through typed components and focused reusable stateful units, with strict compilation identifying invalid properties, events, external values, and contract usage before a release.

**Why this priority**: The migration's maintainability benefit depends on eliminating the current application-wide document orchestration while keeping the two trust domains understandable and independently testable.

**Independent Test**: Inspect and type-check each application independently and as a workspace, deliberately introduce representative property, event, contract, and external-value type errors, and confirm strict checks reject them without relying on broad escape hatches.

**Acceptance Scenarios**:

1. **Given** either application source tree, **When** a maintainer follows state and rendering ownership, **Then** every rendered subtree has one component owner and application state is expressed through typed local state, derived state, properties, events, and focused composables.
2. **Given** a value from native events, browser storage, the document, a network response, or another external boundary, **When** it enters application state, **Then** it is validated at runtime before trusted use and remains statically typed afterward.
3. **Given** a maintainer builds or checks one application, **When** the other application has no relevant change, **Then** the selected application can still be compiled and validated independently.
4. **Given** production frontend source after final acceptance, **When** forbidden-file and escape-hatch checks scan it, **Then** no legacy bootstrap, handwritten production JavaScript, mixed document ownership, broad untyped value, blanket assertion, or unexplained suppression remains.

---

### User Story 5 - Contributors migrate incrementally without breaking the main line (Priority: P2)

As a contributor reviewing the migration, I can accept bounded intermediate revisions in which legacy and migrated code coexist only when each revision builds, passes its applicable tests, and gives every rendered subtree exactly one owner.

**Why this priority**: The migration spans two large applications and must remain reviewable and recoverable without normalizing an unsafe permanent hybrid architecture.

**Independent Test**: Review each migration increment and verify its ownership map, build output, type coverage, tests, and remaining legacy inventory; then run final forbidden-state checks after the last cutover.

**Acceptance Scenarios**:

1. **Given** an intermediate revision with both legacy and migrated frontend code, **When** its rendered surfaces are inventoried, **Then** every document subtree has exactly one active owner and no code writes into a subtree owned by the component application.
2. **Given** an intermediate revision with both JavaScript and TypeScript production sources, **When** its governed build and applicable tests run, **Then** it remains buildable and tested without weakening final strictness requirements.
3. **Given** final migration acceptance, **When** both production source trees and entry points are scanned, **Then** all temporary coexistence has been removed and the final ownership, source-language, and bootstrap constraints are satisfied.

## Edge Cases

- A reconnect snapshot arrives while an older stream, pending action, animation, sound cue, or presentation request is still completing.
- Two or more tabs race to create, renew, replace, or release the shared session-initialization lease, including malformed or expired stored values.
- A desktop event, native command result, browser storage value, generated message, document input, or clipboard result is absent, malformed, stale, or shaped differently from the trusted application model.
- A component is removed or replaced while it owns focus, an animation frame, a timer, an audio node, an abort controller, a pointer target, or an active presentation stream.
- Rapid pointer movement crosses several visual cells belonging to one semantic hacking target, or layout measurement changes between pointer-down and action submission.
- Audio remains locked until a user gesture, a sound asset fails to load or decode, a cue is superseded, or ambient playback must stop during navigation, reconnect, or unmount.
- An authoring, grouping, public-access, update, player, or session dialog receives a stale result after being closed, reopened, or rebound to a different entity.
- A saved session or player-configuration file predates the migration, contains optional or legacy fields, or is saved and reopened by a migrated interface.
- Local, LAN, and public modes expose different connection conditions while still serving the same public application and preserving authentication and same-origin behavior.
- A deterministic generation or binding-integrity check runs with missing tools, modified generated output, an unexpected source file, or a dependency-lock mismatch.
- An intermediate migration revision accidentally mounts a component application inside a subtree that legacy code continues to render or replace.
- A supported package builds successfully but embeds stale assets, includes forbidden production sources, omits a required font or sound, or changes the separation between desktop and public bundles.

## Requirements

### Functional Requirements

- **FR-001**: The production Overseer and player interfaces MUST be migrated as two independent applications using Vue 3, strict TypeScript Single-File Components, the Composition API, and `<script setup lang="ts">`.
- **FR-002**: The migration MUST preserve the exact rendered appearance, CSS behavior, user-visible text, accessibility semantics, keyboard behavior, pointer behavior, focus restoration, CRT and typewriter timing, and audio behavior of both current production interfaces.
- **FR-003**: The public player application MUST preserve connection, reconnect, multi-tab recognition and session initialization, terminal presentation and navigation, hacking rendering and input, sound, and presentation-uplink behavior.
- **FR-004**: The privileged Overseer application MUST preserve desktop API adaptation, presentation control, public-access control, update presentation, terminal authoring, player and session management, and terminal grouping behavior.
- **FR-005**: The two applications MUST remain separate build and trust boundaries with separate entry points, bundles, dependency-facing adapters, and no shared cross-boundary application store.
- **FR-006**: Vue MUST be the sole owner of every rendered production document subtree at final acceptance.
- **FR-007**: Imperative production code MAY be used only in narrow Vue-owned composables, adapters, or directives for Web Audio, focus, element measurement, CRT and typewriter timing, hacking-board fitting and pointer geometry, and presentation streaming.
- **FR-008**: Application state and presentation MUST use typed component properties and events together with Vue reactive references, reactive objects, and derived values, without adding Pinia, Vue Router, Nuxt, JSX, a component library, or a CSS framework.
- **FR-009**: The root frontend workspace and both application workspace manifests MUST expose strict Single-File Component type-check and build commands that reject production JavaScript type-check fallbacks, while each application's commands remain independently runnable inside the governed workspace.
- **FR-010**: Final handwritten production source in `frontend/client` and `frontend/overseer/src` MUST contain no `.js` application module, legacy application bootstrap, temporary mount switch, or mixed ownership of rendered document subtrees; generated Wails bindings, dependencies, build output, and `tests/browser/*.mjs` are excluded from this prohibition.
- **FR-011**: The repository MUST retain one npm workspace rooted at `frontend/`, one committed `frontend/package-lock.json`, one governed clean-install workflow, and separate `frontend/overseer/package.json` and `frontend/client/package.json` workspace manifests, without per-application lockfiles or competing install workflows.
- **FR-012**: The public protobuf browser contracts MUST be generated as deterministic checked-in generator-owned TypeScript using the existing exactly pinned `protoc-gen-es` with `target=ts`.
- **FR-013**: Generated protobuf browser contracts and generated Wails bindings MUST NOT be manually edited.
- **FR-014**: Generator-owned Wails bindings MUST remain unedited in the pinned generator's supported JavaScript, JSDoc, and declaration format, MUST NOT be manually migrated to TypeScript, and MUST be consumed only through a handwritten typed desktop adapter.
- **FR-015**: The typed desktop adapter and player transport boundary MUST runtime-validate localStorage values, Wails events and command results, DOM and browser inputs, clipboard results, and other external values before trusted use; generated protobuf decoding supplies structural wire validation while existing adapters and Go services retain semantic validation and authorization.
- **FR-016**: Production TypeScript MUST contain no broad untyped values, file-wide type-check disabling, blanket assertions, or unexplained compiler or linter suppressions.
- **FR-017**: ConnectRPC MUST remain the application network transport with every existing RPC name, protobuf message meaning, field number, RPC path, wire encoding, cardinality, Connect error behavior, typed action result, request limit, authorization rule, and public or private capability boundary unchanged, and no public health, reflection, status, or administration RPC may be added.
- **FR-018**: The player application MUST preserve complete-snapshot-first reconnect behavior, strictly increasing applicable revisions, stale-state suppression, slow-stream termination and recovery, correlated pending-action behavior, and server-authoritative canonical state.
- **FR-019**: The player application MUST preserve one logical session across qualifying concurrent tabs, one physical stream per connected tab, recognition-handle opacity, lease recovery, and established session-initialization coordination.
- **FR-020**: The player application MUST preserve controller-only immediate presentation feedback, observer read-only behavior, latest-value presentation backpressure, request cancellation, authoritative convergence, and functionally equivalent unary fallback behavior for unsupported or failed presentation streaming.
- **FR-021**: The migration MUST preserve terminal navigation, hacking attempts and outcomes, target geometry, board fitting, special-pattern behavior, presentation cues, ambient sound, input sounds, and gesture-based audio activation without optimistic mutation of canonical gameplay state.
- **FR-022**: The Overseer application MUST preserve authoritative revision comparisons, stale-result suppression, event subscription lifetimes, command correlation, confirmation atomicity, and idempotent handling across all privileged workflows.
- **FR-023**: Existing session and player-configuration JSON MUST load and save with identical supported fields, defaults, compatibility behavior, and business meaning.
- **FR-024**: Existing Go APIs, Wails method contracts, desktop event names and payload meaning, ConnectRPC services, protobuf schemas, persistence contracts, and server behavior MUST remain unchanged except for deterministic browser-code generation required by this migration.
- **FR-025**: The Wails v3 capability allowlist and the public/private security boundary MUST remain no broader than the production baseline.
- **FR-026**: Local, LAN, and protected public access MUST retain current startup, authentication, same-origin, reconnect, credential, clipboard, tunnel, and failure-isolation behavior.
- **FR-027**: Overseer update discovery and decision presentation MUST retain current state ordering, cumulative information, deferral, restart, failure-isolation, and secret-redaction behavior.
- **FR-028**: Terminal authoring, player and session management, and terminal grouping MUST retain current validation, modal, confirmation, accessibility, persistence, focus, and atomic no-partial-change guarantees.
- **FR-029**: The migration MUST preserve separate public and privileged build outputs, existing embedding locations, required fonts and sounds, executable startup behavior, package inventory, and supported Windows amd64, Windows arm64, Linux amd64, Linux arm64, and macOS arm64 packages.
- **FR-030**: Existing Playwright browser and visual journeys MUST remain `.mjs` files, run through the existing production-fidelity fixture architecture, preserve their selectors, snapshots, and behavior, and MUST NOT be presented as evidence of a real packaged Wails runtime.
- **FR-031**: Every intermediate migration revision MUST be buildable and covered by its applicable automated tests.
- **FR-032**: During intermediate coexistence, legacy code MAY own a subtree adjacent to a Vue mount, but each rendered document subtree MUST have exactly one owner; legacy code MUST NOT query, render, replace, mutate, or bind handlers inside a Vue-owned subtree, and Vue code MUST NOT mutate a legacy-owned subtree.
- **FR-033**: Every migration wave MUST record its Vue mount boundaries and remaining legacy ownership, and intermediate JavaScript and TypeScript coexistence MUST remain bounded by that tracked migration inventory without weakening the final production compiler configuration.
- **FR-034**: Final acceptance MUST include deterministic checks for strict Vue Single-File Component compilation, generated protobuf drift, forbidden legacy production files and temporary switches, forbidden type-system escape hatches, both production builds, Wails binding integrity, native embedding and startup, embedded resources, supported package contents, and the complete existing quality suite.
- **FR-035**: Active developer and release documentation, dependency workflows, build scripts, generation scripts, validation scripts, and continuous-integration checks MUST describe and enforce the migrated applications.
- **FR-036**: Historical completed specifications and their recorded evidence MUST remain unchanged.
- **FR-037**: The migration MUST preserve all existing production-supported packages and MUST NOT introduce an additional runtime service, public endpoint, native capability, or browser privilege.
- **FR-038**: The dependency and build configuration MUST permit each application to be type-checked and built independently inside the single governed npm workspace while retaining one shared dependency installation, one lockfile, and one workspace-wide validation path.
- **FR-039**: Production build outputs MUST exclude TypeScript source, handwritten production JavaScript source, development-only tooling, source maps not already governed for release, and generated artifacts not required at runtime.
- **FR-040**: Every deterministic check introduced by the migration MUST fail with an actionable identification of the stale, forbidden, missing, or mismatched artifact.
- **FR-041**: Both application workspace manifests MUST declare the same exact compatible `vue` runtime version, resolved once in `frontend/package-lock.json`.
- **FR-042**: TypeScript, `@vitejs/plugin-vue`, and `vue-tsc` MUST be exact direct development dependencies in their plan-determined workspace ownership, and the migration MUST NOT upgrade unrelated dependencies.
- **FR-043**: Migration work MUST begin with Vue and TypeScript infrastructure plus bounded temporary legacy checking before application component cutover.
- **FR-044**: Deterministic public browser generation MUST switch to `protoc-gen-es` `target=ts` before shared declarations, application shells, or typed transport consumers are completed.
- **FR-045**: Shared declarations, both Vue shells, and the typed desktop API adapter MUST be established before Overseer leaf components and composables are migrated.
- **FR-046**: The Overseer leaf components and composables MUST be migrated and the Overseer legacy bootstrap MUST be fully removed before the main Player application cutover begins.
- **FR-047**: The Player cutover MUST establish its shell, identity, transport, session-initialization, navigation, and presentation components before migrating hacking, CRT and typewriter, sound, and presentation-uplink adapters.
- **FR-048**: The Player legacy application MUST be fully cut over before final strict cleanup, complete verification, packaging, and active-documentation updates begin.
- **FR-049**: Changing generated browser output from `target=js` to `target=ts` MUST NOT change protobuf schemas, descriptors, field numbers, wire encoding, RPC paths, Go-generated contracts, service behavior, or public and private capability boundaries.
- **FR-050**: Every permitted imperative integration MUST register lifecycle cleanup for its timers, animation frames, subscriptions, audio nodes, abort controllers, streams, observers, and document or window listeners.

## Key Entities

- **Player Application**: The public, browser-delivered terminal experience, including one page lifetime, its recognized logical session, physical subscription, authoritative state, transient controller presentation, input, audio, and reconnect coordination.
- **Overseer Application**: The privileged desktop control experience, including current session, players, terminals, groups, public access, update state, authoring dialogs, approvals, and native command and event integration.
- **Desktop API Adapter**: The sole handwritten trusted boundary between the Overseer application and generator-owned native bindings; it validates external values and exposes typed application operations and subscriptions.
- **Player Transport Adapter**: The typed public boundary that sends generated requests, consumes generated snapshots and updates, coordinates reconnect and pending actions, and keeps server authority separate from transient presentation.
- **Authoritative Revision**: The monotonically ordered server or desktop state version used to reject stale values and converge each affected view.
- **Browser Recognition State**: The opaque stored handle, page identity, contender and lease records, and coordination state through which qualifying tabs share one logical session without sharing one physical stream.
- **Rendered Subtree Ownership**: The exclusive relationship between a document subtree and either a migrated component owner or, only during bounded transition, one legacy owner.
- **Migration Wave Ownership Record**: The reviewable inventory for one migration wave, including every Vue mount boundary, every adjacent legacy-owned subtree, the remaining legacy source, and the wave's parity and removal evidence.
- **Generated Contract Artifact**: Deterministically produced browser protobuf or Wails binding output whose source schema or generator is authoritative and whose checked-in integrity is verified.
- **Frontend Workspace**: The single dependency and clean-install boundary rooted at `frontend/`, containing the two application manifests and one committed lockfile while allowing independent type-check and build commands.
- **Build Artifact**: One independently produced privileged or public asset bundle, including its required static resources and its governed embedding or serving destination.

## Success Criteria

### Measurable Outcomes

- **SC-001**: One hundred percent of existing player and Overseer browser journeys pass through the current production-fidelity fixture architecture with unchanged selectors and immutable visual snapshots, and with no intentional screenshot, copy, CSS, accessibility, timing, focus, pointer, keyboard, or audio baseline change.
- **SC-002**: A final scan of handwritten production source in `frontend/client` and `frontend/overseer/src` finds zero `.js` application modules, legacy bootstraps, temporary mount switches, mixed-ownership document mutations, `allowJs`, `checkJs`, `@ts-nocheck`, broad `any`, blanket assertions, or unexplained suppressions after excluding generated Wails bindings, dependencies, build output, and `tests/browser/*.mjs`.
- **SC-003**: Workspace-wide and per-application strict component compilation complete with zero type errors on a clean dependency installation.
- **SC-004**: Clean dependency installation and two consecutive generation-and-build runs MUST produce an unchanged lockfile, byte-identical generated browser contracts, and equivalent privileged-bundle, public-bundle, content-hash, and embedded-asset inventories; byte-identical Vite bundles are additionally required only if Phase 0 research proves reproducible bytes with the existing toolchain.
- **SC-005**: Contract tests confirm 100% preservation of existing public method names, message meanings, and cardinalities, with zero new private contracts or capabilities reachable from the public bundle.
- **SC-006**: Reconnect, stale-update, pending-action, slow-stream, and multi-tab race journeys produce zero regressing revisions, duplicate logical sessions for qualifying tabs, replayed stale effects, or optimistic canonical-state mutations.
- **SC-007**: Representative current and legacy session and player-configuration fixtures round-trip through the migrated interfaces with no loss or change in supported business content.
- **SC-008**: Local, LAN, and protected public journeys all pass for startup, authentication, reconnect, explicit public-access lifecycle, credential editing and sharing, clipboard failure, and tunnel failure without crossing the public/private boundary.
- **SC-009**: Supported package validation passes for all five governed operating-system and architecture targets, with every required frontend asset present and every forbidden source or stale bundle absent.
- **SC-010**: Deterministic drift, forbidden-state, binding-integrity, embedded-asset, package-content, production-build, and complete repository quality checks all pass from a clean checkout.
- **SC-011**: Every reviewed intermediate revision has one recorded owner per rendered subtree, a successful applicable production build, and a passing applicable test set; final acceptance has zero entries remaining in the legacy inventory.
- **SC-012**: Automated malformed-boundary tests reject 100% of the complete explicitly defined invalid native-event, storage, document-input, network-value, clipboard-result, and command-result fixture set before trusted state changes, while accepting all valid production fixtures; this criterion does not quantify theoretically possible invalid values outside that fixture set.

## Assumptions

- The current production behavior and completed specifications through feature 032 are the acceptance baseline; this migration intentionally changes maintainability and compile-time safety, not product behavior.
- Exact mutually compatible `vue`, TypeScript, `@vitejs/plugin-vue`, and `vue-tsc` versions and their appropriate workspace ownership will be selected during planning, committed without ranges in `frontend/package-lock.json`, and introduced without unrelated dependency upgrades.
- Existing CSS, fonts, sounds, user-visible copy, protobuf schemas, Go services, Wails capabilities, and persistence formats remain authoritative inputs rather than redesign opportunities.
- The generated Wails binding directory remains outside the handwritten-production-source prohibition because it is generator-owned and consumed only through the typed adapter.
- Existing test journeys may gain typed build setup and additional assertions, but their `.mjs` format and behavioral expectations remain unchanged.

## Out of Scope

- Redesigning either interface, changing user-visible copy, altering CSS appearance, or changing interaction timing.
- Changing Go APIs, protobuf service semantics, persistence formats, authentication, tunnel behavior, update policy, or security boundaries.
- Combining the public and privileged applications or introducing a shared application store, routing system, meta-framework, JSX layer, component library, or CSS framework.
- Rewriting historical completed specifications or their archived validation evidence.

## Verbatim Constraints

- Feature identifier: `033-frontend-vue-typescript-migration`
- Production application boundaries: `frontend/overseer` and `frontend/client`
- Final Overseer handwritten-source boundary: `frontend/overseer/src`
- Required component script form: `<script setup lang="ts">`
- Required reactive primitives: `ref`, `reactive`, `computed`
- Required production dependency identifier: `vue`
- Required development dependency identifiers: `typescript`, `@vitejs/plugin-vue`, `vue-tsc`
- Required workspace lockfile: `frontend/package-lock.json`
- Required protobuf generation mode: `target=ts`
- Prohibited framework and library additions: `Pinia`, `Vue Router`, `Nuxt`, `JSX`
- Existing browser protobuf generator identifier: `protoc-gen-es`
- Prohibited production compiler options: `allowJs`, `checkJs`
- Prohibited type escape: `any`
- Prohibited file-wide suppression: `@ts-nocheck`
- Final handwritten production JavaScript extension: `.js`
- Browser-test exclusion boundary: `tests/browser/*.mjs`
- Preserved browser-test extension: `.mjs`
