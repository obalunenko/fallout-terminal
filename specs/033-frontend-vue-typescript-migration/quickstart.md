# Clean Migration Verification Quickstart

This document is the implementation-time verification procedure. It does not claim that migration implementation checks ran during planning.

## 1. Confirm the source and toolchain

```bash
git rev-parse HEAD
git status --short
node --version
npm --version
go version
task node:check
scripts/frontend-task-contract-check.sh --node-version-self-test
```

- The immutable legacy rollback source is `06696ee1c7155a1bb1135ef46ec91445dd73a2a4`.
- The implementation revision must be a descendant or reviewed successor, not a runtime toggle back to the legacy bundle.
- Run `nvm use` from the repository root and require exactly Node.js `26.8.1`. `.nvmrc`, Node preflight, dependency-compatibility evidence, clean verification, and final evidence all use that exact version; a Node upgrade is unrelated scope and requires a separate governed change.
- The Node self-test must prove `26.8.1` passes, `26.8.0` fails, and `26.8.2` fails. A missing executable, malformed fixture, or Taskfile parse error is an infrastructure failure, not an accepted negative-version result.
- Confirm `.nvmrc`, `Taskfile.yml`, `.github/workflows/wails-cross-platform.yml`, `.github/workflows/wails-portable.yml`, `frontend/package.json`, `frontend/client/package.json`, `frontend/overseer/package.json`, `frontend/package-lock.json`, `README.md`, and `CONTRIBUTING.md` agree on the exact pin.
- Begin from a clean checkout when collecting final reproducibility, generation, or package evidence.

## 2. Install one frontend workspace

```bash
task frontend:build
```

This canonical target owns the single governed frontend dependency installation and then calls both no-install per-application build targets. The root Taskfile is the public workflow owner; internal npm commands are not a second documented workflow.

Verify:

- only `frontend/package-lock.json` exists as a frontend application lockfile;
- both application manifests declare `vue` exactly `3.5.42`;
- the root workspace declares exact `typescript` `6.0.3`, `@vitejs/plugin-vue` `6.0.8`, and `vue-tsc` `3.3.11`;
- Vite, Wails, ConnectRPC, protobuf, Node, and Playwright pins are unchanged;
- the lockfile and manifests remain unchanged after the target's governed clean installation.

## 3. Run independent and workspace TypeScript gates

```bash
task frontend:typecheck:overseer
task frontend:typecheck:client
task frontend:typecheck
task frontend:policy:check
```

The per-app targets dispatch only their owning workspace check; the aggregate target runs both. None installs dependencies. All use the pinned `vue-tsc`, check strict SFC templates and application-owned TypeScript, and compile generated Player TypeScript. Final configs must contain `strict`, `noUncheckedIndexedAccess`, and `exactOptionalPropertyTypes`, with no `allowJs` or `checkJs`. Only the npm install/lock boundary, exact compiler/build tooling, and capability-neutral `frontend/tsconfig.base.json` are shared; authored declarations remain application-owned.

`task frontend:policy:check` must self-test forbidden production source and prohibited type escapes before scanning production, prove the single-lockfile and Player dependency boundaries, reject expired temporary mechanisms, and enforce final cutover. Generated Wails bindings, dependencies, output directories, and `tests/browser/*.mjs` are the only applicable exclusions.

## 4. Verify protobuf TypeScript generation

```bash
task proto:format:check
task proto:lint
task proto:breaking
task proto:drift:check
task proto:generated:check
task proto:check
```

Verify:

- `frontend/client/gen/fallout/terminal/player/v1` contains exactly five `_pb.ts` files and no generated `_pb.js` files;
- provenance records `protoc-gen-es v2.13.0` with `target=ts,import_extension=js`;
- two generations are byte-identical and checked-in output has no drift;
- the deliberate browser-TypeScript drift fixture is rejected and restored;
- Go generated output and protobuf schemas/descriptors are unchanged;
- all eight existing Player RPCs retain paths, cardinalities, binary encoding, limits, error behavior, authorization, and public-only capability.

## 5. Verify Wails bindings and both builds

```bash
task bindings:check
task frontend:build:overseer
task frontend:build:client
task frontend:reproducible:check
```

Verify:

- Wails output remains generator-owned JavaScript/JSDoc/declarations with exact 39 methods and seven named events;
- the typed adapter is the sole authored Wails/runtime consumer;
- Overseer and Player outputs remain separate;
- Player bundle scans contain no Wails/native/private/Overseer path;
- TypeScript/SFC source, handwritten production JavaScript source, development tools, and ungoverned source maps are absent from runtime outputs.

The per-app build targets build only their owning production bundle and do not install dependencies. `task frontend:reproducible:check` performs two complete builds of the same revision and produces byte-identical `frontend/overseer/dist` and `frontend/client/dist` trees, deterministic generated sources, identical asset manifests, and actionable sorted path/mode/size/SHA-256 differences on failure.

## 6. Verify persistence compatibility and external boundaries

```bash
task frontend:compatibility:check
task frontend:boundary:check
```

`task frontend:compatibility:check` runs in Overseer wave e and final wave i. During task generation, record the exact reviewed paths for one current and one legacy version-1 session document, one current and one legacy player-configuration document, compatible session unknown fields, legacy player-attribute defaults, strict player-config unknown-field rejection, and established cross-file player-configuration references. Reuse existing repository fixtures and Go codecs where suitable. The gate opens each valid fixture through the migrated Overseer boundary, renders and edits without changing meaning, saves, reopens, and compares supported fields, defaults, references, preserved session extras, player-config validation, location, and business meaning. Loss, silent normalization, relocation, or validation weakening fails.

`task frontend:boundary:check` consumes `tests/browser/fixtures/frontend-boundary-manifest.json`. Every entry records the boundary class, fixture identifier, owning adapter/composable, expected accept or reject result, trusted projection or no-state-change outcome, migration wave, and focused test file. The complete reviewed population covers Wails/native events and command results, storage, DOM/form and pointer/keyboard inputs, decoded network values, clipboard, sound values, and presentation-stream capability/results. The gate rejects invalid entries before trusted mutation, accepts valid entries, and fails on any missing test mapping. It claims completeness only for this reviewed manifest, not every theoretically possible invalid value.

## 7. Verify each ownership wave

At every wave, review [migration-ownership.md](./migration-ownership.md) and record:

1. active Vue mounts;
2. adjacent legacy owners;
3. remaining legacy files and handlers;
4. cross-query/cross-mutation scan result;
5. focused strict type-check and build result;
6. focused browser and visual parity result;
7. removal criteria met or exact expiry wave.

Every implementation task must also run a locally executable completion check against the files it creates or changes. A later integration/parity task adds evidence but cannot be the prerequisite task's only verification. For test-first work, record the exact expected failing assertion, accepted absent-behavior signature, rejected infrastructure/configuration signatures, RED evidence path, and later GREEN task; then the GREEN task reruns the same assertion successfully.

Wave c may create only the empty capability-neutral Player shell, strict participation, Player-owned declarations/ports, mount function, isolated candidate document/entry, and dependency/boundary fixtures. It cannot add Player business behavior, replace production DOM, use Wails/privileged APIs, or become production-selected. Do not begin Player feature migration in wave f until Overseer wave e has passed its complete cutover, temporary-removal, parity, native/binding/resource/package, forbidden-state, and ownership gates. Do not accept wave i while any temporary mechanism register entry remains.

## 8. Run browser and visual evidence

```bash
task browser:test
```

The complete existing `.mjs` suite must pass through the production-fidelity fixture architecture with unchanged selectors and visual snapshots. Confirm Player production assets are served from the built public bundle and Overseer tests mount production SFCs with only the typed desktop transport replaced by the deterministic fake.

Record this evidence only as browser/visual evidence. Do not describe it as a packaged Wails runtime test.

## 9. Run Go and repository quality gates

If implementation changed Go source, before each commit containing those changes run:

```bash
go fix ./...
```

Review every generated modernization edit and keep only intentional changes. Then run from macOS through the governed Taskfile targets:

```bash
task fmt:check
task vet
task lint
task test
task test:race
task startup:check
task ci:quality
task check
```

These gates cover buildtool ordering, workflow contracts, Go behavior, concurrency, binding/generation integration, native embedding declarations, resources, public/private separation, and active CI policy.

This is a global per-task pre-commit rule. Every task that changes a `.go` file must cite and perform it before its commit. The final wave-i Go-fix task is an audit of recorded compliance and cannot repair missing earlier compliance retroactively.

## 10. Verify native embedding, resources, secrets, and packages

Run the exact governed native/resource and packaging checks that consume the migrated layouts:

```bash
scripts/wails-bindings-check.sh
scripts/wails-v3-cutover-check.sh
scripts/secret-leak-check.sh
scripts/legacy-public-access-check.sh
scripts/state-changing-reset-native-smoke.sh
scripts/reproducible-build-check.sh
scripts/dependency-license-check.sh
scripts/verify-linux-package.sh --self-test
```

On Windows, run `scripts/verify-windows-package.ps1` through its documented self-test/package workflow. Also run the Taskfile/buildtool gates that exercise `main.go`, `internal/platform/assets_test.go`, `internal/player/http_test.go`, `internal/player/public_stream_test.go`, `internal/buildtool/buildtool.go`, `internal/buildtool/preflight.go`, `internal/buildtool/buildtool_test.go`, and `internal/buildtool/package_test.go`. Before the first consuming gate, tasks must update any legacy filename, root, entrypoint, bundle, or generated-JavaScript assumption in those exact files. `scripts/dependency-license-check.sh` and `THIRD_PARTY_NOTICES.md` must account for the reviewed shipped Vue runtime and exact pinned frontend dependencies.

On each matching supported host, build and inspect exactly one governed package:

```bash
task package GOOS=darwin GOARCH=arm64
task package GOOS=windows GOARCH=amd64
task package GOOS=windows GOARCH=arm64
task package GOOS=linux GOARCH=amd64
task package GOOS=linux GOARCH=arm64
```

Each command is run only on its matching host. Verify the executable, separate embedded frontend roles, required font/sound/static resources, package inventory, identity, startup, and absence of forbidden source/stale bundles.

## 11. Verify active documentation

Confirm the accepted Vue/TypeScript architecture and commands appear consistently in:

- `README.md`;
- `ARCHITECTURE.md`;
- `CONTRIBUTING.md`;
- `docs/platform-packaging.md`;
- `Taskfile.yml`;
- `internal/buildtool/buildtool.go`;
- `internal/buildtool/preflight.go`;
- `.specify/templates/plan-template.md`;
- `.specify/templates/spec-template.md`;
- `.specify/templates/tasks-template.md`.

Treat `docs/wails-beta15-upgrade.md`, `docs/wails-migration-rollback.md`, `docs/wails-v3-migration-rollback.md`, and completed historical specifications as read-only comparison inputs; do not rewrite them.

## 12. Record results honestly

Use `PASS`, `FAIL`, or `NOT RUN` with the command, host, and reason. Never convert a deterministic fake into real-provider evidence or a browser fixture into native evidence.

| Conditional evidence | Expected planning status until actually executed |
|---|---|
| Real credential-dependent public endpoint | `NOT RUN` unless explicitly opted in with user-supplied credentials |
| Signing, notarization, stapling, Gatekeeper | `NOT RUN` unless credentials/services are available and the workflow is explicitly executed |
| Windows amd64/arm64 matching-host startup/package | `NOT RUN` on a non-matching host |
| Linux amd64/arm64 matching-host startup/package | `NOT RUN` on a non-matching host |
| Optional matching-host native UI/audio/secure-store journeys | `NOT RUN` when unavailable; not required to claim another host's support |
