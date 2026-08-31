# Frontend Build, Type-Check, and Generation Contract

## Workspace ownership

`frontend/` remains the sole npm installation and lock boundary.

| Contract item | Required value |
|---|---|
| Workspace root | `frontend/package.json` |
| Workspaces | `client`, `overseer` |
| Only frontend lockfile | `frontend/package-lock.json` |
| Governed clean install | `task frontend:build`, which invokes the one internal `npm ci --prefix frontend` installation before both per-app builds |
| Prohibited | `frontend/client/package-lock.json`, `frontend/overseer/package-lock.json`, app-local install docs/tasks, or another active install workflow |

The root manifest owns exact direct development dependencies `typescript: 6.0.3`, `@vitejs/plugin-vue: 6.0.8`, and `vue-tsc: 3.3.11`. Both application manifests own the exact direct runtime dependency `vue: 3.5.42`. Existing exact Vite `8.1.5`, Wails frontend `3.0.0-beta.15`, ConnectRPC `2.1.2`, protobuf `2.13.0`, Node `26.8.1`, and Playwright `1.62.1` pins remain unchanged.

## Exact Node runtime contract

Wave a changes `Taskfile.yml` `node:check` from a minimum-version check to an exact-version check. It must accept only `26.8.1`; `26.8.0` and `26.8.2` are both rejected with an actionable expected/actual message. `scripts/frontend-task-contract-check.sh --node-version-self-test` supplies deterministic positive and negative version inputs, so its bootstrap self-test is locally executable before the canonical frontend targets exist. The exact value must agree across `.nvmrc`, `Taskfile.yml`, `.github/workflows/wails-cross-platform.yml`, `.github/workflows/wails-portable.yml`, `frontend/package.json`, `frontend/client/package.json`, `frontend/overseer/package.json`, `frontend/package-lock.json`, `specs/033-frontend-vue-typescript-migration/quickstart.md`, `README.md`, and `CONTRIBUTING.md`. A newer Node runtime is not accepted by this feature and requires a separate governed change.

## Canonical Taskfile commands

The root Taskfile is the public workflow owner. The final application manifests still contain independently runnable `typecheck` and `build` scripts, but their npm invocations are implementation details called by these canonical targets and are not documented as a competing workflow:

| Target | Required behavior |
|---|---|
| `task frontend:typecheck:overseer` | Dispatch only the Overseer workspace `typecheck`; do not install dependencies or check Player. |
| `task frontend:typecheck:client` | Dispatch only the Player workspace `typecheck`; do not install dependencies or check Overseer. |
| `task frontend:typecheck` | Run both strict per-app checks, including generated Player TypeScript, without installing dependencies. |
| `task frontend:build:overseer` | Build only `frontend/overseer/dist` from the Overseer production entrypoint without installing dependencies. |
| `task frontend:build:client` | Build only `frontend/client/dist` from the Player production entrypoint without installing dependencies. |
| `task frontend:build` | Perform the single governed frontend dependency installation, then call both per-app build targets; create no other lockfile or install path. |
| `task frontend:compatibility:check` | Run the complete FR-023/SC-007 session/player-configuration compatibility fixture gate. |
| `task frontend:boundary:check` | Run every FR-015/SC-012 entry in the reviewed frontend boundary manifest and reject unmapped entries. |
| `task frontend:policy:check` | Enforce forbidden source/type escapes, one lockfile, Player dependency boundaries, temporary-mechanism inventory, and final cutover. |
| `task frontend:reproducible:check` | Build both Vite outputs twice and compare actionable sorted path/mode/size/SHA-256 tree evidence. |

The root npm scripts dispatch with workspace selection and the application manifests own their individual scripts. No npm script recursively installs dependencies. Only `task frontend:build` owns the installation step; every other canonical frontend target consumes that governed installed workspace.

## Executable task-introduction order

Wave-a task generation must preserve this producer-before-consumer order:

1. Update the four exact manifests/lockfile inputs and exact Node contract; validate with manifest inspection, `npm ci --prefix frontend`, `task node:check`, and the standalone Node-version self-test only.
2. Add `frontend/tsconfig.base.json`, `frontend/overseer/tsconfig.json`, `frontend/client/tsconfig.json`, `frontend/overseer/tsconfig.legacy.json`, and `frontend/client/tsconfig.legacy.json`; validate with direct locally available `npx --prefix frontend vue-tsc --noEmit -p <exact-config>` commands until canonical targets exist.
3. Add `frontend/overseer/vite.config.ts`, `frontend/client/vite.config.ts`, and the exact manifest scripts; validate with direct locally available workspace scripts until canonical targets exist.
4. Create the six type/build targets in `Taskfile.yml`, then execute each target and their aggregate contracts.
5. Create `scripts/frontend-policy-check.sh` and `scripts/reproducible-build-check.sh`, prove each with its standalone self-test, then create and execute `frontend:policy:check` and `frontend:reproducible:check`.
6. Introduce `frontend:compatibility:check` and `frontend:boundary:check` only after their checker, exact fixtures/manifest, and focused tests exist.

No implementation task may verify a checker, script, fixture, package script, Taskfile target, or generated artifact that is created by a later task. Every implementation task has a locally executable completion check when reached; later parity and wave-exit evidence is additive, never its only completion check. A test-first RED task records the expected failing assertion, accepted product-behavior failure signature, rejection of infrastructure/configuration failures, RED evidence path, and later GREEN task before the RED task is complete.

## Strict compiler policy

`frontend/tsconfig.base.json` is extended by both application configurations and is the only shared declaration/configuration input. It contains capability-neutral compiler policy only. Authored environment, global, transport, view-state, Wails, ConnectRPC, component, and composable declarations remain inside their owning application boundary, and type-only imports cannot create a cross-boundary application contract. Final production compilation must include all of the following:

- `strict: true`;
- `noUncheckedIndexedAccess: true`;
- `exactOptionalPropertyTypes: true`;
- `moduleResolution: "Bundler"`;
- `module: "ESNext"`;
- `isolatedModules: true`;
- `verbatimModuleSyntax: true`;
- `noEmit: true` for type-check commands;
- strict Vue template checking;
- no `allowJs` or `checkJs` in the base or final application configs.

The final repository check fails with exact file/line evidence for broad `any`, `@ts-nocheck`, blanket assertions, unexplained suppression directives, or assertions used only to bypass a type error. It excludes generator-owned Wails bindings from application-source policy and handles generated Protobuf TypeScript as generator-owned but strictly compiled source.

## Independent Vue builds

| Application | HTML/entry | Vite config | Output | Consumer |
|---|---|---|---|---|
| Overseer | `frontend/overseer/src/index.html` → `frontend/overseer/src/main.ts` | `frontend/overseer/vite.config.ts` with Vue and Wails plugins | `frontend/overseer/dist` | `main.go` privileged Wails asset filesystem |
| Player | `frontend/client/index.html` → `frontend/client/src/main.ts` | `frontend/client/vite.config.ts` with Vue plugin only | `frontend/client/dist` | `main.go`/`internal/player` public static HTTP filesystem |

The outputs remain separate and contain only runtime HTML, CSS, JavaScript bundles, fonts/sounds/static assets, and `.keep` markers required by existing embedding. They exclude TypeScript/SFC source, handwritten production JavaScript source, development tooling, ungoverned source maps, Wails binding source, and generated contracts not required at runtime.

Both Vite output trees must be byte-identical across two clean builds of one source revision and lockfile. `task frontend:reproducible:check` owns the comparison. Its reproducibility manifest sorts relative paths and records mode, size, and SHA-256 so a mismatch names the exact asset.

## Persistence compatibility gate

`task frontend:compatibility:check` is the canonical FR-023/SC-007 gate in Overseer wave e and final wave i. Its production-fidelity fixture set contains:

- one representative current session document;
- one representative legacy version-1 session document;
- one current player-configuration document;
- one legacy player-configuration document;
- compatible unknown fields in session documents, plus strict unknown-field rejection and absent legacy attribute defaults in player configurations; and
- existing cross-file player-configuration reference behavior.

Each valid fixture opens through the migrated Overseer application boundary, renders and permits an edit without changing established meaning, saves, reopens, and compares supported fields, defaults, references, preserved session extras, and player-config validation behavior. The gate fails on loss, silent normalization, relocation, validation weakening, or business-meaning change. Task generation must inventory and name the exact reviewed fixture paths. It reuses current repository fixtures and the existing Go version-1 codecs wherever suitable and does not establish a duplicate persistence format.

## Frontend boundary fixture gate

`task frontend:boundary:check` owns one reviewed manifest at `tests/browser/fixtures/frontend-boundary-manifest.json`. Every entry records:

1. boundary class;
2. fixture identifier;
3. owning adapter or composable;
4. expected accept or reject result;
5. expected trusted projection or no-state-change outcome;
6. applicable migration wave; and
7. focused test file.

The manifest enumerates valid and invalid Wails/native named events, Wails command results, localStorage/storage-event records, DOM/form inputs, pointer/keyboard-derived values, ConnectRPC-decoded semantic network values, clipboard outcomes, sound-manifest/asset values, and presentation-stream capability/results. Desktop/Wails entries are implemented and focused in waves c/e; Player storage/network/input entries in wave f; pointer/sound/stream entries in wave g; production Player completion in wave h; and the complete manifest runs in wave i. The final gate rejects every explicitly listed invalid fixture before trusted mutation, accepts every explicitly listed valid fixture, and fails if any manifest entry lacks a test mapping. Its completeness claim is limited to the reviewed manifest, not all theoretically possible invalid data.

## Frontend policy gate

`task frontend:policy:check` owns final forbidden production source, prohibited type escapes, the single-lockfile rule, direct and transitive Player dependency boundaries, temporary-mechanism inventory, and final-cutover removal. Generated Wails bindings, dependencies, build/package output, and `tests/browser/*.mjs` retain only their path-exact applicable exclusions.

## Public browser protobuf generation

The producer remains `proto/fallout/terminal/player/v1/*.proto` with exact `@bufbuild/protoc-gen-es` `2.13.0`. `proto/buf.gen.es.yaml` contains:

```yaml
out: frontend/client/gen
opt:
  - target=ts
  - import_extension=js
```

The checked-in output inventory is exactly:

```text
frontend/client/gen/fallout/terminal/player/v1/
├── hacking_pb.ts
├── navigation_pb.ts
├── player_pb.ts
├── sound_pb.ts
└── terminal_pb.ts
```

Each file records `protoc-gen-es v2.13.0` and the generation options. Cross-file imports keep `.js` specifiers, which TypeScript bundler resolution and Vite map to `_pb.ts` sources. No `_pb.js`, `_pb.d.ts`, parallel target, or checked-in transpiled output is permitted.

Generation checks must:

1. verify Buf format/lint/build and the unchanged schema revision;
2. generate Go contracts with the unchanged Go template;
3. generate browser TypeScript with the exact local npm plugin;
4. prove checked-in output equals a clean generation;
5. prove two clean generations are byte-identical;
6. check exact five-file inventory and provenance;
7. deliberately alter a browser `.ts` fixture and prove actionable drift rejection/restoration;
8. run strict Player type-check and build;
9. reject private/persistence/configuration imports and descriptor exposure;
10. prove Go module files, schemas, descriptors, field numbers, wire behavior, RPC paths/cardinality, request limits, authorization, and service behavior did not change.

## Wails binding generation and typed consumption

The only Wails producer remains:

```text
go tool -modfile=tools/wails/go.mod wails3 generate bindings -clean -d frontend/overseer/bindings ./...
```

Generated JavaScript, JSDoc, `eventdata.d.ts`, model files, and generator suppressions remain unedited. The generated directory is excluded from handwritten-production-JavaScript and application-owned suppression scans but remains included in deterministic generation, exact method/event inventory, private/public boundary, and package integrity checks.

`frontend/overseer/src/adapters/desktop-api.ts` is the only authored consumer. A TypeScript/Vite alias declaration describes all generated method names without duplicating structured DTOs and exposes untrusted results/event payloads as `unknown`; the adapter validates and narrows them. Binding integrity compares that declaration with the exact 39 generated desktop methods and seven named events, including `application-update-status`, `client-count`, `coordination-state`, `hack-state`, `public-access-status`, `server-info`, and `session-state`. The adapter's application-facing operation set remains exactly the current facade set: generated `CopyDemo` stays present and integrity-checked but remains intentionally unexposed to the UI.

## Canonical workflow integration

`internal/buildtool` remains the detailed build owner. Its prepare order becomes:

1. clean frontend workspace install;
2. deterministic protobuf and generated-client verification;
3. Player strict type-check;
4. Player production build;
5. clean Wails binding generation/integrity;
6. Overseer strict type-check;
7. Overseer production build;
8. native build/package step selected by the existing target plan.

`Taskfile.yml` exposes this policy through the ten canonical `frontend:*` targets above without duplicating buildtool or npm internals. `.github/workflows/wails-cross-platform.yml` and `.github/workflows/wails-portable.yml` continue to enter through Task targets and cache only `frontend/package-lock.json`.

## Producer and consumer update matrix

| Producer | Consumers that must change together |
|---|---|
| `.nvmrc`, `frontend/package.json`, `frontend/client/package.json`, `frontend/overseer/package.json`, `frontend/package-lock.json` | `Taskfile.yml` `node:check` and canonical frontend targets; `scripts/frontend-task-contract-check.sh`; `internal/buildtool/buildtool.go`; `internal/buildtool/preflight.go`; `internal/buildtool/buildtool_test.go`; `.github/workflows/wails-cross-platform.yml`; `.github/workflows/wails-portable.yml`; `scripts/dependency-license-check.sh`; `THIRD_PARTY_NOTICES.md`; `README.md`; `CONTRIBUTING.md`; `specs/033-frontend-vue-typescript-migration/quickstart.md` |
| `frontend/tsconfig.base.json`, `frontend/overseer/tsconfig.json`, `frontend/client/tsconfig.json`, `frontend/overseer/tsconfig.legacy.json`, `frontend/client/tsconfig.legacy.json`, application-owned declarations | Exact workspace/app scripts in the three manifests; strict generated browser contracts; owned SFCs; `scripts/frontend-compiler-fixture-check.sh`; `scripts/frontend-policy-check.sh`; both workflow files; no authored cross-app declaration module |
| `frontend/overseer/vite.config.ts`, `frontend/client/vite.config.ts`, `frontend/overseer/src/index.html`, `frontend/overseer/src/main.ts`, `frontend/client/index.html`, `frontend/client/src/main.ts` | `Taskfile.yml`; `tests/browser/playwright.config.mjs`; `tests/browser/fixture-server/main.go`; `main.go`; `internal/platform/assets_test.go`; `internal/player/http_test.go`; `internal/player/public_stream_test.go`; `scripts/state-changing-reset-native-smoke.sh`; `scripts/secret-leak-check.sh`; `scripts/verify-linux-package.sh`; `scripts/verify-windows-package.ps1`; `scripts/reproducible-build-check.sh`; `internal/buildtool/buildtool.go`; `internal/buildtool/preflight.go`; `internal/buildtool/buildtool_test.go` |
| `frontend/client/client.css`, `frontend/client/fonts/Fixedsys.ttf`, exact files under `frontend/client/sounds/`, `frontend/overseer/src/overseer.css`, `frontend/overseer/src/Fixedsys.ttf` | Vue templates and Vite asset graphs; `internal/player/http.go` SoundManifest; exact browser visual tests and snapshots; `internal/platform/assets_test.go`; `scripts/verify-linux-package.sh`; `scripts/verify-windows-package.ps1`; matching-host package inventories. Regenerated task fields enumerate each known sound/test path. |
| `proto/buf.gen.es.yaml`, `frontend/client/gen/fallout/terminal/player/v1/hacking_pb.ts`, `frontend/client/gen/fallout/terminal/player/v1/navigation_pb.ts`, `frontend/client/gen/fallout/terminal/player/v1/player_pb.ts`, `frontend/client/gen/fallout/terminal/player/v1/sound_pb.ts`, `frontend/client/gen/fallout/terminal/player/v1/terminal_pb.ts` | `scripts/proto-generate.sh`; `scripts/proto-check.sh`; `scripts/proto-drift-test.sh`; exact Player imports; `tests/browser/fixture-server/main.go`; `internal/platform/assets_test.go`; `scripts/verify-linux-package.sh`; `scripts/verify-windows-package.ps1`; `internal/buildtool/buildtool.go`; `internal/buildtool/preflight.go`; `.github/workflows/wails-cross-platform.yml`; `.github/workflows/wails-portable.yml`; `specs/033-frontend-vue-typescript-migration/quickstart.md` |
| `frontend/overseer/bindings/` generator output | `frontend/overseer/src/adapters/wails-service.d.ts`; `frontend/overseer/src/adapters/desktop-api.ts`; `scripts/wails-bindings-check.sh`; `internal/buildtool/buildtool.go`; `internal/buildtool/preflight.go`; native startup and matching-host package checks |
| `frontend/overseer/test-fixtures/index.html`, `frontend/overseer/test-fixtures/candidate-main.ts`, `frontend/overseer/test-fixtures/fake-desktop-port.ts` | `frontend/overseer/vite.config.ts`; `tests/browser/playwright.config.mjs`; `tests/browser/fixture-server/main.go`; `tests/browser/desktop-api.spec.mjs`; `tests/browser/fixtures/frontend-boundary-manifest.json`; browser selectors/snapshots only; fake port remains permanent test-only infrastructure outside production and never native evidence |
| Exact existing session/player-config fixtures and Go version-1 codecs selected during task regeneration | `task frontend:compatibility:check`; exact tests named by its producer task; Overseer wave-e and final wave-i evidence. Read-only input paths must be listed separately from files the task modifies. |
| Final `frontend/overseer/dist` and `frontend/client/dist` layouts | `main.go`; `internal/platform/assets_test.go`; `internal/player/http_test.go`; `internal/player/public_stream_test.go`; `scripts/secret-leak-check.sh`; `scripts/state-changing-reset-native-smoke.sh`; `scripts/verify-linux-package.sh`; `scripts/verify-windows-package.ps1`; `scripts/dependency-license-check.sh`; `scripts/reproducible-build-check.sh`; `internal/buildtool/buildtool.go`; `internal/buildtool/preflight.go`; `internal/buildtool/buildtool_test.go`; `.github/workflows/wails-cross-platform.yml`; `.github/workflows/wails-portable.yml`; `README.md`; `CONTRIBUTING.md`; `.specify/templates/plan-template.md`; `.specify/templates/spec-template.md`; `.specify/templates/tasks-template.md` |

Every generated task uses complete repository-relative paths in `Files (modify/create/delete)` and a separate `Read-only inputs` field. Bare basenames, directory inheritance, prose aliases such as “both manifests,” known-inventory globs, and umbrella phrases are invalid. Go-changing tasks additionally cite the global pre-commit rule: run `go fix ./...`, review every modernization edit, retain only intentional edits, format, and then run that task's Go validation gates. The wave-i Go-fix task is an audit of prior compliance, not a retroactive substitute.
