# Feature Specification: Vue and TypeScript Frontend Migration

**Feature Branch**: `033-frontend-vue-typescript-migration`
**Created**: 2026-08-30
**Status**: Draft
**Input**: Migrate both production frontends to independently owned, strictly typed component applications while preserving every established user, protocol, security, build, and distribution contract.

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

- **FR-001**: The production Overseer and player interfaces MUST be migrated as two independent applications using Vue 3, strict TypeScript Single-File Components, the Composition API, and script setup with TypeScript.
- **FR-002**: The migration MUST preserve the exact rendered appearance, CSS behavior, user-visible text, accessibility semantics, keyboard behavior, pointer behavior, focus restoration, CRT and typewriter timing, and audio behavior of both current production interfaces.
- **FR-003**: The public player application MUST preserve connection, reconnect, multi-tab recognition and session initialization, terminal presentation and navigation, hacking rendering and input, sound, and presentation-uplink behavior.
- **FR-004**: The privileged Overseer application MUST preserve desktop API adaptation, presentation control, public-access control, update presentation, terminal authoring, player and session management, and terminal grouping behavior.
- **FR-005**: The two applications MUST remain separate build and trust boundaries with separate entry points, bundles, dependency-facing adapters, and no shared cross-boundary application store.
- **FR-006**: Vue MUST be the sole owner of every rendered production document subtree at final acceptance.
- **FR-007**: Imperative production code MAY be used only in narrow Vue-owned adapters, composables, or directives for Web Audio, focus and element measurement, CRT and typewriter animation, hacking-board fitting and pointer geometry, and presentation streaming.
- **FR-008**: Application state and presentation MUST use typed component properties and events together with Vue reactive references, reactive objects, and derived values, without adding Pinia, Vue Router, Nuxt, JSX, a component library, or a CSS framework.
- **FR-009**: Each application and the frontend workspace MUST provide a strict Single-File Component type-check command that rejects production JavaScript type-check fallbacks.
- **FR-010**: Final production source under the player and Overseer source boundaries MUST contain no handwritten JavaScript, legacy application bootstrap, or mixed ownership of rendered document subtrees.
- **FR-011**: The frontend dependency manifests and lockfile MUST pin one exact reproducible Vue production dependency and exact reproducible TypeScript, Vue build-plugin, and Single-File Component type-checker development dependencies.
- **FR-012**: The public protobuf browser contracts MUST be generated as deterministic checked-in TypeScript using the repository's existing pinned protobuf browser generator.
- **FR-013**: Generated protobuf browser contracts and generated Wails bindings MUST NOT be manually edited.
- **FR-014**: Generator-owned Wails JavaScript, documentation annotations, and declarations MAY remain in the bindings directory and MUST be consumed only through a handwritten typed desktop adapter.
- **FR-015**: The typed desktop adapter and player transport boundary MUST validate untrusted native events, command results, browser storage, document and browser inputs, network values, and other external values at runtime before they enter trusted application state.
- **FR-016**: Production TypeScript MUST contain no broad untyped values, file-wide type-check disabling, blanket assertions, or unexplained compiler or linter suppressions.
- **FR-017**: The migration MUST preserve every existing ConnectRPC method, message meaning, transport encoding, and unary, server-streaming, or client-streaming cardinality.
- **FR-018**: The player application MUST preserve complete-snapshot-first reconnect behavior, strictly increasing applicable revisions, stale-state suppression, correlated pending-action behavior, and server-authoritative canonical state.
- **FR-019**: The player application MUST preserve one logical session across qualifying concurrent tabs, one physical stream per connected tab, recognition-handle opacity, lease recovery, and established session-initialization coordination.
- **FR-020**: The player application MUST preserve controller-only immediate presentation feedback, observer read-only behavior, latest-value presentation backpressure, request cancellation, fallback, and authoritative convergence semantics.
- **FR-021**: The migration MUST preserve terminal navigation, hacking attempts and outcomes, target geometry, board fitting, special-pattern behavior, presentation cues, ambient sound, input sounds, and gesture-based audio activation without optimistic mutation of canonical gameplay state.
- **FR-022**: The Overseer application MUST preserve authoritative revision comparisons, stale-result suppression, event subscription lifetimes, command correlation, confirmation atomicity, and idempotent handling across all privileged workflows.
- **FR-023**: Existing session and player-configuration JSON MUST load and save with identical supported fields, defaults, compatibility behavior, and business meaning.
- **FR-024**: Existing Go APIs, Wails method contracts, desktop event names and payload meaning, ConnectRPC services, protobuf schemas, persistence contracts, and server behavior MUST remain unchanged except for deterministic browser-code generation required by this migration.
- **FR-025**: The Wails v3 capability allowlist and the public/private security boundary MUST remain no broader than the production baseline.
- **FR-026**: Local, LAN, and protected public access MUST retain current startup, authentication, same-origin, reconnect, credential, clipboard, tunnel, and failure-isolation behavior.
- **FR-027**: Overseer update discovery and decision presentation MUST retain current state ordering, cumulative information, deferral, restart, failure-isolation, and secret-redaction behavior.
- **FR-028**: Terminal authoring, player and session management, and terminal grouping MUST retain current validation, modal, confirmation, accessibility, persistence, focus, and atomic no-partial-change guarantees.
- **FR-029**: The migration MUST preserve separate public and privileged build outputs, existing embedding locations, required fonts and sounds, executable startup behavior, package inventory, and supported Windows amd64, Windows arm64, Linux amd64, Linux arm64, and macOS arm64 packages.
- **FR-030**: Existing Playwright browser and visual journeys MUST remain ECMAScript module files and MUST all pass against production-fidelity migrated builds.
- **FR-031**: Every intermediate migration revision MUST be buildable and covered by its applicable automated tests.
- **FR-032**: During intermediate coexistence, each rendered document subtree MUST have exactly one owner and legacy code MUST NOT render, replace, or mutate a Vue-owned subtree.
- **FR-033**: Intermediate JavaScript and TypeScript coexistence MUST be explicitly bounded by a tracked migration inventory and MUST NOT weaken the final production compiler configuration.
- **FR-034**: Final acceptance MUST include deterministic checks for strict Vue Single-File Component compilation, generated protobuf drift, forbidden legacy production files, forbidden type-system escape hatches, both production builds, Wails binding integrity, embedded assets, supported package contents, and the complete existing quality suite.
- **FR-035**: Active developer and release documentation, dependency workflows, build scripts, generation scripts, validation scripts, and continuous-integration checks MUST describe and enforce the migrated applications.
- **FR-036**: Historical completed specifications and their recorded evidence MUST remain unchanged.
- **FR-037**: The migration MUST preserve all existing production-supported packages and MUST NOT introduce an additional runtime service, public endpoint, native capability, or browser privilege.
- **FR-038**: The dependency and build configuration MUST permit each application to be installed, type-checked, built, and tested independently while also supporting one governed workspace-wide validation path.
- **FR-039**: Production build outputs MUST exclude TypeScript source, handwritten production JavaScript source, development-only tooling, source maps not already governed for release, and generated artifacts not required at runtime.
- **FR-040**: Every deterministic check introduced by the migration MUST fail with an actionable identification of the stale, forbidden, missing, or mismatched artifact.

## Key Entities

- **Player Application**: The public, browser-delivered terminal experience, including one page lifetime, its recognized logical session, physical subscription, authoritative state, transient controller presentation, input, audio, and reconnect coordination.
- **Overseer Application**: The privileged desktop control experience, including current session, players, terminals, groups, public access, update state, authoring dialogs, approvals, and native command and event integration.
- **Desktop API Adapter**: The sole handwritten trusted boundary between the Overseer application and generator-owned native bindings; it validates external values and exposes typed application operations and subscriptions.
- **Player Transport Adapter**: The typed public boundary that sends generated requests, consumes generated snapshots and updates, coordinates reconnect and pending actions, and keeps server authority separate from transient presentation.
- **Authoritative Revision**: The monotonically ordered server or desktop state version used to reject stale values and converge each affected view.
- **Browser Recognition State**: The opaque stored handle, page identity, contender and lease records, and coordination state through which qualifying tabs share one logical session without sharing one physical stream.
- **Rendered Subtree Ownership**: The exclusive relationship between a document subtree and either a migrated component owner or, only during bounded transition, one legacy owner.
- **Generated Contract Artifact**: Deterministically produced browser protobuf or Wails binding output whose source schema or generator is authoritative and whose checked-in integrity is verified.
- **Build Artifact**: One independently produced privileged or public asset bundle, including its required static resources and its governed embedding or serving destination.

## Success Criteria

### Measurable Outcomes

- **SC-001**: One hundred percent of existing player and Overseer browser journeys and visual comparisons pass with no intentional screenshot, copy, accessibility, timing, focus, pointer, keyboard, or audio baseline changes.
- **SC-002**: A final production-source scan finds zero handwritten JavaScript files, zero legacy application bootstraps, zero mixed-ownership document mutations, zero broad untyped values, zero file-wide type-check disabling directives, and zero unexplained suppressions in the governed player and Overseer source boundaries.
- **SC-003**: Workspace-wide and per-application strict component compilation complete with zero type errors on a clean dependency installation.
- **SC-004**: Clean dependency installation and two consecutive generation-and-build runs produce identical lockfile, generated browser contracts, privileged bundle, public bundle, and embedded-asset inventories.
- **SC-005**: Contract tests confirm 100% preservation of existing public method names, message meanings, and cardinalities, with zero new private contracts or capabilities reachable from the public bundle.
- **SC-006**: Reconnect, stale-update, pending-action, slow-stream, and multi-tab race journeys produce zero regressing revisions, duplicate logical sessions for qualifying tabs, replayed stale effects, or optimistic canonical-state mutations.
- **SC-007**: Representative current and legacy session and player-configuration fixtures round-trip through the migrated interfaces with no loss or change in supported business content.
- **SC-008**: Local, LAN, and protected public journeys all pass for startup, authentication, reconnect, explicit public-access lifecycle, credential editing and sharing, clipboard failure, and tunnel failure without crossing the public/private boundary.
- **SC-009**: Supported package validation passes for all five governed operating-system and architecture targets, with every required frontend asset present and every forbidden source or stale bundle absent.
- **SC-010**: Deterministic drift, forbidden-state, binding-integrity, embedded-asset, package-content, production-build, and complete repository quality checks all pass from a clean checkout.
- **SC-011**: Every reviewed intermediate revision has one recorded owner per rendered subtree, a successful applicable production build, and a passing applicable test set; final acceptance has zero entries remaining in the legacy inventory.
- **SC-012**: Automated malformed-boundary tests reject 100% of representative invalid native events, storage records, document inputs, network values, and command results before trusted state changes, while accepting all valid production fixtures.

## Assumptions

- The current production behavior and completed specifications through feature 032 are the acceptance baseline; this migration intentionally changes maintainability and compile-time safety, not product behavior.
- Exact dependency versions will be selected during planning from mutually compatible supported releases and committed without ranges in the existing lockfile.
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
- Required component script form: `script setup lang=ts`
- Required reactive primitives: `ref`, `reactive`, `computed`
- Required production dependency identifier: `Vue`
- Required development dependency identifiers: `TypeScript`, `@vitejs/plugin-vue`, `vue-tsc`
- Prohibited framework and library additions: `Pinia`, `Vue Router`, `Nuxt`, `JSX`
- Existing browser protobuf generator identifier: `protoc-gen-es`
- Prohibited production compiler options: `allowJs`, `checkJs`
- Prohibited type escape: `any`
- Prohibited file-wide suppression: `@ts-nocheck`
- Preserved browser-test extension: `.mjs`
