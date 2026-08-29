# Phase 0 Research: Vue and TypeScript Frontend Migration

## Exact Vue and TypeScript dependency pins

**Decision**: Add the following exact pins and no unrelated dependency changes:

| Package | Exact version | Ownership |
|---|---:|---|
| `vue` | `3.5.42` | Direct production dependency in both `frontend/client/package.json` and `frontend/overseer/package.json` |
| `typescript` | `6.0.3` | Direct development dependency in `frontend/package.json` |
| `@vitejs/plugin-vue` | `6.0.8` | Direct development dependency in `frontend/package.json` |
| `vue-tsc` | `3.3.11` | Direct development dependency in `frontend/package.json` |

The shared development tools belong to the workspace root because both applications use one compiler policy and one installed toolchain. Each application manifest still owns an independently runnable `typecheck` and `build` script, while both manifests declare the same exact Vue runtime and the one `frontend/package-lock.json` resolves the runtime once through npm workspace hoisting.

**Rationale**: The official npm registry metadata reports [Vue `3.5.42`](https://registry.npmjs.org/vue/3.5.42); [`@vitejs/plugin-vue` `6.0.8`](https://registry.npmjs.org/%40vitejs%2Fplugin-vue/6.0.8) accepts Vue `^3.2.25`, Vite `^8.0.0`, and Node `^20.19.0 || >=22.12.0`; [`vue-tsc` `3.3.11`](https://registry.npmjs.org/vue-tsc/3.3.11) declares TypeScript `>=5.0.0`; and [TypeScript `6.0.3`](https://registry.npmjs.org/typescript/6.0.3) supports the repository's Node runtime. A throwaway fixture using Node `26.8.1`, Vite `8.1.5`, the four selected pins, strict options, one `<script setup lang="ts">` SFC, generated `protoc-gen-es` TypeScript, and a Wails adapter declaration passed both `vue-tsc --noEmit` and `vite build`. The fixture resolved exactly the selected versions.

TypeScript `7.0.2` was tested and rejected: `vue-tsc` `3.3.11` failed before checking the project because TypeScript 7 no longer exports `typescript/lib/tsc`. That makes `6.0.3` the newest tested compatible TypeScript line for this plan rather than an arbitrary downgrade. Vue documents `<script setup>` as the recommended Composition API syntax for SFCs and supports typed properties and emits directly ([Vue `<script setup>` documentation](https://vuejs.org/api/sfc-script-setup)).

**Alternatives considered**:

- TypeScript `7.0.2`: rejected by the actual `vue-tsc` compatibility fixture.
- Per-application TypeScript/plugin/`vue-tsc` declarations: rejected because they duplicate one toolchain and invite version drift inside the single workspace.
- A range such as `^3.5.42`: rejected by Constitution 9.0.0 and the clarified exact-pin requirement.
- Upgrading Vite, ConnectRPC, protobuf, Wails, Node, Playwright, or other packages: rejected as unrelated scope and prohibited by the specification.

## Strict TypeScript configuration

**Decision**: Add `frontend/tsconfig.base.json` as the sole shared capability-neutral compiler policy and application-specific `frontend/client/tsconfig.json` and `frontend/overseer/tsconfig.json`. The shared final settings include `strict: true`, `noUncheckedIndexedAccess: true`, `exactOptionalPropertyTypes: true`, `moduleResolution: "Bundler"`, `module: "ESNext"`, `target: "ES2022"`, `isolatedModules: true`, `verbatimModuleSyntax: true`, `noEmit: true`, DOM libraries, and explicit strict Vue template checking. Each application configuration includes only its application root and owned declarations; Player additionally includes `frontend/client/gen/**/*.ts`. Environment, global, transport, view-state, Wails, ConnectRPC, component, and composable declarations stay inside their owning application source or configuration boundary. No shared authored application type module is created, and a type-only import cannot establish a cross-boundary application contract.

`allowJs` and `checkJs` may appear only in explicit migration-only configurations introduced in wave a. Those files have the Frontend Migration owner, expire at the relevant application cutover, may check only the listed legacy files, and are deleted no later than wave e for Overseer and wave h for Player. They never enter the final base or application configurations.

**Rationale**: This shares compiler policy and exact tooling without sharing authored application declarations, runtime types, or trust-boundary state. `moduleResolution: "Bundler"` also lets TypeScript resolve generated `_pb.ts` sources through their emitted-runtime `.js` specifiers, matching Vite's ESM behavior. The candidate fixture compiled generated TypeScript with all required strict flags and no source suppression.

**Alternatives considered**:

- One monolithic application `tsconfig`: rejected because it obscures the Player/Overseer trust boundary and prevents independent checks.
- Permanent `allowJs` or `checkJs`: rejected because it would normalize the transition state as final architecture.
- Broad `skipLibCheck` as a substitute for binding isolation: rejected; generated Wails JavaScript stays outside application compilation, and application-owned declarations are checked normally.

## `protoc-gen-es` `target=ts` output and imports

**Decision**: Change only `proto/buf.gen.es.yaml` from `target=js` to `target=ts`; retain `import_extension=js`. Keep `@bufbuild/protoc-gen-es` and `@bufbuild/protobuf` at `2.13.0`, the output at `frontend/client/gen`, and the schema inputs unchanged.

**Rationale**: The Protobuf-ES manual states that `target=ts` emits `_pb.ts` and that `import_extension=js` independently emits ESM-compatible `.js` specifiers ([Protobuf-ES manual](https://github.com/bufbuild/protobuf-es/blob/main/MANUAL.md)). A clean throwaway generation with the repository's exact generator produced the same five logical files as TypeScript, retained descriptor bytes and service metadata, and used imports such as `./hacking_pb.js`. Two outputs generated with and without `import_extension=js` differed only in provenance and those cross-file suffixes. The strict Vue/TypeScript fixture successfully imported `player_pb.js` while compiling the underlying `player_pb.ts`, and Vite bundled it without a second generated format.

Generation and drift checks must therefore change their inventory and provenance assertions from five `_pb.js` files to five `_pb.ts` files, compile those files under the Player strict configuration, deliberately corrupt one checked-in browser TypeScript file in the drift self-test, and continue to prove two byte-identical generations. Scripts that currently execute checked-in generated JavaScript directly on Linux or Windows must instead consume the governed compiled test fixture or built Player graph; they must not create or check in a parallel JavaScript generator output.

**Alternatives considered**:

- `target=ts` with extensionless imports: rejected because it changes the established ESM import convention and is less representative of emitted JavaScript paths.
- `target=ts+js` or a separately compiled checked-in JavaScript tree: rejected because it creates duplicate generated sources and a second drift surface.
- `target=js+dts`: rejected because the clarified requirement fixes `target=ts`.
- Schema, descriptor, RPC, Go generation, or service changes: rejected as prohibited and unnecessary.

## Wails JavaScript, JSDoc, and declaration consumption

**Decision**: Leave `frontend/overseer/bindings` exactly as emitted by Wails `3.0.0-beta.15`. Add an application-owned module alias declaration that exposes each allowlisted generated service function with typed arguments where safe and an `unknown` result at the trust boundary; map the same alias to the generated `desktopservice.js` in `frontend/overseer/vite.config.ts`. The handwritten `frontend/overseer/src/adapters/desktop-api.ts` is the only consumer of that alias and of `@wailsio/runtime`. It runtime-validates results, event payloads, clipboard outcomes, and required method availability before returning trusted application types.

The generated `eventdata.d.ts` remains generator-owned evidence of the seven named events and is checked by the binding-integrity workflow. Application code does not rely on its model imports as trusted values; the adapter receives event data as `unknown` and narrows it. A deterministic contract check compares the alias declaration and adapter allowlist with the generated service/event inventory so the shadow type surface cannot silently drift.

**Rationale**: The current generator emits service and model JavaScript with JSDoc, plus `eventdata.d.ts`; it does not emit `desktopservice.d.ts`. A strict TypeScript probe with `allowJs: false`, `checkJs: false`, and `noImplicitAny` could not directly import `desktopservice.js` (`TS7016`). Enabling `allowJs` makes the generated JavaScript part of the application program and violates final-cutover policy. A throwaway strict fixture proved that a TypeScript/Vite alias declaration backed by the unchanged generated JavaScript compiles and builds with neither option enabled. Using `unknown` at this external boundary is intentional: generated transport construction does not replace the adapter's semantic and security validation.

**Alternatives considered**:

- Convert or edit Wails bindings to TypeScript: prohibited and rejected.
- Keep final `allowJs`/`checkJs`: prohibited and rejected.
- Generate a second declaration tree with TypeScript: rejected because it adds a derived generator, cleanup order, and drift surface that the narrow adapter does not need.
- Suppress the direct import with `@ts-ignore` or type it as `any`: rejected because it weakens the final strict boundary.

## Vue roots, entrypoints, and Vite integration

**Decision**: Retain exactly two independent Vite builds.

- Overseer keeps the `frontend/overseer/src` Vite root, relative base, output `frontend/overseer/dist`, Wails Vite plugin, font/CSS paths, and `.keep` restoration. `frontend/overseer/src/index.html` becomes a minimal document shell with one `#overseerApp` mount and one `main.ts` entry.
- Player keeps the `frontend/client` Vite root, output `frontend/client/dist`, sound copy behavior, font/CSS paths, and `.keep` restoration. `frontend/client/index.html` becomes a minimal document shell with one `#playerApp` mount and one `src/main.ts` entry.
- Both `vite.config.js` files become `vite.config.ts` and add `@vitejs/plugin-vue`; only Overseer retains the Wails plugin and Wails alias. The applications share only the npm install/lock boundary, exact compiler/build tooling, and `frontend/tsconfig.base.json`; no authored environment, global, transport, view-state, Wails, ConnectRPC, component, composable, entry, adapter, state, or bundle module is shared across the capability boundary.

Global CSS is imported unchanged and remains unscoped during the migration. Existing markup, IDs, classes, semantic elements, accessible names, `hidden` behavior, data attributes, copy, focus order, fonts, sounds, and CSP remain immutable parity inputs.

**Rationale**: This preserves the current embedding and serving topology and limits Vue to a source-architecture change. Separate app configurations make it possible to build and type-check either application independently from the one installed workspace.

**Alternatives considered**:

- One Vue root or multi-page shared Vite application: rejected because it collapses trust and bundle ownership.
- Shared component/state package: rejected because no cross-boundary application state is needed and presentation reuse risks a privileged dependency edge.
- Pinia, Vue Router, Nuxt, JSX, component libraries, or CSS frameworks: rejected because Vue primitives and the existing single-document interaction model are sufficient.

## Browser fixture integration

**Decision**: Preserve all `.mjs` journeys, selectors, and visual snapshots while changing the Overseer fixture producer from raw JavaScript source rewriting to a Vite-built test entrypoint. Production exports a small typed `mountOverseerApp(desktopPort)` bootstrap used by both `main.ts` and the test-only entrypoint. Production `main.ts` supplies the Wails-backed adapter; the test entrypoint supplies a typed fake `DesktopPort` through the same production-owned interface. The candidate/test entrypoint is temporary and is deleted in wave e. The typed fake is permanent test-only evidence infrastructure outside production source and bundles; it is retained after wave e, never embedded or packaged, checked against the production `DesktopPort` contract, and never treated as native Wails evidence. The fixture build reuses the production SFCs, CSS, and templates.

Player browser tests continue to consume the production `frontend/client/dist`. Browser evidence remains separate from native evidence: the fixture proves Vue behavior and visual parity, while binding generation, embedding, startup, resources, dialogs, native accessibility, and package behavior remain governed by Wails/native/package checks.

**Rationale**: `tests/browser/fixture-server/main.go` currently rewrites and serves raw Overseer HTML/JavaScript, which cannot execute `.vue` or TypeScript directly. A typed injected port preserves production application code without claiming that a browser double is Wails. It also avoids a runtime production switch: the alternative entry exists only in the browser-test build graph.

**Alternatives considered**:

- Teach the Go fixture server to compile SFCs dynamically: rejected because it creates another build workflow.
- Keep a legacy JavaScript fixture application: rejected because it would stop testing production behavior.
- Run browser Playwright against a packaged Wails process and call it equivalent: rejected because browser and native evidence have different trust and lifecycle claims.

## Vite byte reproducibility

**Decision**: Require byte-identical Overseer and Player Vite output trees across two clean builds with the exact lockfile, in addition to deterministic generated sources, equivalent manifests, content hashes, and asset inventories.

**Rationale**: A Phase 0 experiment copied the current workspace to a throwaway directory, reused the locked installation, ran both Vite `8.1.5` builds twice, and calculated sorted relative-path plus SHA-256 tree digests. Overseer produced the same digest `47d1d197145d55523e367c6135434b14a8e1b53f2f4227b3e3b0bf45385df018` on both runs; Player produced `62317dc11ac49b8ed4461345e2bec3f620405eabc6b56ccd492d870f388045ce` on both runs. The existing `scripts/reproducible-build-check.sh` independently encodes repeated tree hashing for both frontend distributions as part of the full package build. Because current Vite output is demonstrably byte-reproducible, the specification's conditional stronger gate applies. Final verification repeats the clean experiment after the Vue cutover; the historical digests above are research evidence, not expected Vue output values.

**Alternatives considered**:

- Require only equivalent inventories and content manifests: rejected because Phase 0 proved the stronger baseline.
- Freeze current bundle filenames or digests as post-migration values: rejected because Vue legitimately changes implementation bytes; equality is required between repeated builds of the same accepted source revision, not against the legacy bundle.

## DOM ownership and migration shape

**Decision**: Use the mandatory waves in order, finish Overseer before Player production ownership changes, and keep Player's visible DOM wholly legacy-owned until one atomic wave-h transfer. Overseer may migrate complete body-sibling leaf/dialog subtrees as disjoint Vue islands in wave d, but only when the full markup and all handlers for that subtree move together. The main Overseer document subtree transfers atomically in wave e. Every temporary mount, type-check configuration, candidate entry, adapter, or production compatibility seam has the Frontend Migration owner, a named expiry wave, parity gates, and an explicit removal task in [migration-ownership.md](./migration-ownership.md). Permanent test-only evidence infrastructure is recorded separately and cannot appear in production source, bundles, embeds, or packages.

**Rationale**: The current applications are single large document orchestrators. Migrating a Vue parent while legacy code still renders or replaces descendants would violate single ownership and make focus, pointer, timing, and selector parity unreviewable. Body-sibling Overseer dialogs are separable leaves; Player hacking, pagination, reveal, streaming, and audio state are tightly coupled, so an atomic visible transfer is safer.

**Alternatives considered**:

- Mount Vue inside legacy-rendered lists, trees, hacking columns, or terminal content: rejected because legacy `replaceChildren`, query, and handler delegation would cross ownership.
- Move Player leaf subtrees one by one in production: rejected because terminal presentation, geometry, audio, reveal, authority, and streaming lifecycles share observable timing and focus.
- A permanent compatibility bridge or dual entrypoint: rejected by the constitution and final forbidden-state requirements.

## Model and contract artifacts

**Decision**: Do not create `data-model.md`. Create only two contracts: the build/generation/type-check contract and the trust/evidence-boundary contract.

**Rationale**: The migration introduces application-local TypeScript view types and composable state but does not change persistent entities, canonical Go models, protobuf schemas, DTO meanings, or runtime state transitions. A data-model document would falsely imply a product-model change. Build, generation, fixture, trust, and verification surfaces do change and have multiple producers and consumers, so explicit contracts add value.

**Alternatives considered**:

- Restate all existing protobuf and persistence models in a new data model: rejected as duplication and a drift risk.
- Create no contracts: rejected because the one-workspace command graph, deterministic TypeScript generation, Wails exemption, separate embeds, and evidence separation need enforceable identifiers and checks.

## Rollback and temporary compatibility governance

**Decision**: Use commit `06696ee1c7155a1bb1135ef46ec91445dd73a2a4` as the immutable pre-migration rollback source. Rollback means reverting the feature commits to that complete source revision; it does not preserve a runtime toggle, legacy bundle, alternate install workflow, or dual architecture in the accepted product.

Every temporary mechanism is listed per wave with an owner, expiry, parity gate, and removal criterion. No mechanism may outlive wave e for Overseer, wave h for Player, or wave i for final compiler/build governance.

**Rationale**: A source-control rollback is reviewable and does not weaken the final architecture or package. It also preserves historical completed specifications and their evidence unchanged.

**Alternatives considered**:

- Ship both legacy and Vue bundles behind a switch: rejected as a permanent dual protocol and mixed ownership risk.
- Rewrite completed historical specs to point at Vue: rejected by the constitution and the feature scope.
