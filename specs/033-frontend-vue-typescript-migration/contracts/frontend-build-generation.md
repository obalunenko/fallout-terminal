# Frontend Build, Type-Check, and Generation Contract

## Workspace ownership

`frontend/` remains the sole npm installation and lock boundary.

| Contract item | Required value |
|---|---|
| Workspace root | `frontend/package.json` |
| Workspaces | `client`, `overseer` |
| Only frontend lockfile | `frontend/package-lock.json` |
| Governed clean install | `npm ci --prefix frontend` through the root Taskfile/buildtool workflows |
| Prohibited | `frontend/client/package-lock.json`, `frontend/overseer/package-lock.json`, app-local install docs/tasks, or another active install workflow |

The root manifest owns exact direct development dependencies `typescript: 6.0.3`, `@vitejs/plugin-vue: 6.0.8`, and `vue-tsc: 3.3.11`. Both application manifests own the exact direct runtime dependency `vue: 3.5.42`. Existing exact Vite `8.1.5`, Wails frontend `3.0.0-beta.15`, ConnectRPC `2.1.2`, protobuf `2.13.0`, Node `26.8.1`, and Playwright `1.62.1` pins remain unchanged.

## Commands

The final manifests expose these independently runnable commands from the one installed workspace:

| Scope | Command | Required behavior |
|---|---|---|
| Workspace | `npm run typecheck --prefix frontend` | Run strict `vue-tsc --noEmit` for Overseer and Player and include generated browser TypeScript. |
| Overseer | `npm run typecheck:overseer --prefix frontend` | Dispatch the Overseer workspace `typecheck` script only. |
| Player | `npm run typecheck:client --prefix frontend` | Dispatch the Player workspace `typecheck` script only. |
| Overseer | `npm run build:overseer --prefix frontend` | Build only `frontend/overseer/dist` from the Overseer Vue entrypoint. |
| Player | `npm run build:client --prefix frontend` | Build only `frontend/client/dist` from the Player Vue entrypoint. |
| Workspace | `npm run build --prefix frontend` | Build both applications without installing dependencies or creating another lockfile. |

Each application manifest contains its own `typecheck` and `build` scripts. The root scripts dispatch with npm workspace selection; they do not invoke `npm install` or `npm ci` recursively.

## Strict compiler policy

`frontend/tsconfig.base.json` is extended by both application configurations. Final production compilation must include all of the following:

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

Both Vite output trees must be byte-identical across two clean builds of one source revision and lockfile. The reproducibility manifest sorts relative paths and records mode, size, and SHA-256 so a mismatch names the exact asset.

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

`Taskfile.yml` exposes this policy and adds focused frontend type-check/forbidden-state/reproducibility tasks without duplicating buildtool internals. `.github/workflows/wails-cross-platform.yml` and `.github/workflows/wails-portable.yml` continue to enter through Task targets and cache only `frontend/package-lock.json`.

## Producer and consumer update matrix

| Producer | Consumers that must change together |
|---|---|
| Root/app manifests and lockfile | Task `node:check`, `deps:frontend`, frontend type/build tasks, buildtool plans/tests, CI cache/preflight, dependency/license validation, README/contributor docs |
| Shared/app TypeScript configs | Workspace/app scripts, `vue-tsc`, generated browser contracts, SFCs, forbidden-state checks, CI |
| `vite.config.ts` and HTML/entrypoints | Vite builds, Wails plugin, browser fixture build, Go embeds, production resource tests, package checks |
| CSS/fonts/sounds/static assets | Vue templates, Vite asset graph, SoundManifest serving, visual tests, resource and package inventories |
| `proto/buf.gen.es.yaml` | generation/check/drift scripts, Player imports, native package probe fixtures, buildtool preflight, CI, docs |
| Wails generator output | desktop adapter alias/inventory, browser fake port, binding integrity, buildtool, native startup/package checks |
| Browser fixture build | `tests/browser/playwright.config.mjs`, `fixture-server/main.go`, fixture bindings, selectors/snapshots; never native evidence |
| Final source layout | `internal/platform/assets_test.go`, secret/boundary/legacy scans, state-changing native smoke, active Spec Kit templates and docs |
