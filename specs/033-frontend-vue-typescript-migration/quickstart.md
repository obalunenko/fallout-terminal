# Clean Migration Verification Quickstart

This document is the implementation-time verification procedure. It does not claim that migration implementation checks ran during planning.

## 1. Confirm the source and toolchain

```bash
git rev-parse HEAD
git status --short
node --version
npm --version
go version
```

- The immutable legacy rollback source is `06696ee1c7155a1bb1135ef46ec91445dd73a2a4`.
- The implementation revision must be a descendant or reviewed successor, not a runtime toggle back to the legacy bundle.
- Run `nvm use` from the repository root and require Node.js `26.8.1` or newer under the repository's exact policy.
- Begin from a clean checkout when collecting final reproducibility, generation, or package evidence.

## 2. Install one frontend workspace

```bash
task deps:frontend
```

Verify:

- only `frontend/package-lock.json` exists as a frontend application lockfile;
- both application manifests declare `vue` exactly `3.5.42`;
- the root workspace declares exact `typescript` `6.0.3`, `@vitejs/plugin-vue` `6.0.8`, and `vue-tsc` `3.3.11`;
- Vite, Wails, ConnectRPC, protobuf, Node, and Playwright pins are unchanged;
- the lockfile and manifests remain unchanged after `npm ci`.

## 3. Run independent and workspace TypeScript gates

```bash
npm run typecheck:overseer --prefix frontend
npm run typecheck:client --prefix frontend
npm run typecheck --prefix frontend
```

All commands must use the pinned `vue-tsc`, check strict SFC templates and owned TypeScript, and compile generated Player TypeScript. Final configs must contain `strict`, `noUncheckedIndexedAccess`, and `exactOptionalPropertyTypes`, with no `allowJs` or `checkJs`.

Run the final forbidden-state task introduced by the migration and confirm that its self-test rejects each prohibited fixture before it scans production source. Generated Wails bindings, dependencies, output directories, and `tests/browser/*.mjs` are the only applicable exclusions.

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
npm run build:overseer --prefix frontend
npm run build:client --prefix frontend
task frontend:build
```

Verify:

- Wails output remains generator-owned JavaScript/JSDoc/declarations with exact 39 methods and seven named events;
- the typed adapter is the sole authored Wails/runtime consumer;
- Overseer and Player outputs remain separate;
- Player bundle scans contain no Wails/native/private/Overseer path;
- TypeScript/SFC source, handwritten production JavaScript source, development tools, and ungoverned source maps are absent from runtime outputs.

Run the governed reproducibility task/script after a clean install. Two complete builds of the same revision must produce byte-identical `frontend/overseer/dist` and `frontend/client/dist` trees, deterministic generated sources, and identical asset manifests.

## 6. Verify each ownership wave

At every wave, review [migration-ownership.md](./migration-ownership.md) and record:

1. active Vue mounts;
2. adjacent legacy owners;
3. remaining legacy files and handlers;
4. cross-query/cross-mutation scan result;
5. focused strict type-check and build result;
6. focused browser and visual parity result;
7. removal criteria met or exact expiry wave.

Do not begin Player wave f until Overseer wave e has passed its complete cutover and forbidden-state gate. Do not accept wave i while any temporary mechanism register entry remains.

## 7. Run browser and visual evidence

```bash
task browser:test
```

The complete existing `.mjs` suite must pass through the production-fidelity fixture architecture with unchanged selectors and visual snapshots. Confirm Player production assets are served from the built public bundle and Overseer tests mount production SFCs with only the typed desktop transport replaced by the deterministic fake.

Record this evidence only as browser/visual evidence. Do not describe it as a packaged Wails runtime test.

## 8. Run Go and repository quality gates

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

## 9. Verify native embedding, resources, secrets, and packages

Run the governed native/resource checks referenced by Task/buildtool and the repository scripts, including Wails pin/cutover, secret-leak, legacy-public-access, production-resource, native startup/accessibility, and package-content checks applicable to the host.

On each matching supported host, build and inspect exactly one governed package:

```bash
task package GOOS=darwin GOARCH=arm64
task package GOOS=windows GOARCH=amd64
task package GOOS=windows GOARCH=arm64
task package GOOS=linux GOARCH=amd64
task package GOOS=linux GOARCH=arm64
```

Each command is run only on its matching host. Verify the executable, separate embedded frontend roles, required font/sound/static resources, package inventory, identity, startup, and absence of forbidden source/stale bundles.

## 10. Verify active documentation

Confirm the accepted Vue/TypeScript architecture and commands appear consistently in:

- `README.md`;
- `ARCHITECTURE.md`;
- `CONTRIBUTING.md`;
- applicable active files under `docs/`;
- `Taskfile.yml` and buildtool-owned command documentation;
- `.specify/templates/plan-template.md`, `tasks-template.md`, and applicable active template guidance.

Do not rewrite completed historical specifications or recorded evidence.

## 11. Record results honestly

Use `PASS`, `FAIL`, or `NOT RUN` with the command, host, and reason. Never convert a deterministic fake into real-provider evidence or a browser fixture into native evidence.

| Conditional evidence | Expected planning status until actually executed |
|---|---|
| Real credential-dependent public endpoint | `NOT RUN` unless explicitly opted in with user-supplied credentials |
| Signing, notarization, stapling, Gatekeeper | `NOT RUN` unless credentials/services are available and the workflow is explicitly executed |
| Windows amd64/arm64 matching-host startup/package | `NOT RUN` on a non-matching host |
| Linux amd64/arm64 matching-host startup/package | `NOT RUN` on a non-matching host |
| Optional matching-host native UI/audio/secure-store journeys | `NOT RUN` when unavailable; not required to claim another host's support |
