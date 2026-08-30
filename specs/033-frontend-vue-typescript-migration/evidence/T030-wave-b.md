# T030 Wave-b Exit Evidence

Date: 2026-08-30

## Ownership conclusion

- Overseer production remains wholly legacy-owned by `frontend/overseer/src/index.html`, `overseer.js`, and `desktop-api.js`.
- Player production remains wholly legacy-owned by `frontend/client/index.html`, `client.js`, `sound.js`, and `presentation-uplink.js`.
- Generated browser contracts are generator-owned TypeScript inputs only; no application DOM, selector, CSS, copy, accessibility, timing, geometry, or lifecycle ownership changed.
- The T020 deliberate protobuf drift mutation was removed in-command, and Wave b has no remaining temporary mechanism.

## Generated inventory

The exact sorted browser contract inventory is:

```text
hacking_pb.ts
navigation_pb.ts
player_pb.ts
sound_pb.ts
terminal_pb.ts
```

Every file carries protoc-gen-es `2.13.0` provenance for `target=ts,import_extension=js`. No `_pb.js`, `_pb.d.ts`, or checked-in parallel compiled JavaScript output remains. The five protobuf schemas and generated Go Player tree have no Git diff.

## Verification

The Wave-b exit gates completed successfully:

```text
task proto:check
task frontend:typecheck:client
task frontend:build:client
go test ./internal/buildtool
go test ./internal/platform -run TestAssets
go test ./internal/player
scripts/verify-linux-package.sh --self-test
scripts/frontend-task-contract-check.sh --self-test --expected-target-count 8
npm test --prefix tests/browser -- connectrpc-player.spec.mjs
npm test --prefix tests/browser -- crt-rendering.spec.mjs
git diff --exit-code -- tests/browser/crt-rendering.spec.mjs-snapshots/compact-hacking-hover-darwin.png tests/browser/crt-rendering.spec.mjs-snapshots/medium-hacking-hover-darwin.png tests/browser/crt-rendering.spec.mjs-snapshots/large-hacking-hover-darwin.png
```

Results:

- Protobuf format, lint, deterministic two-run generation, checked-in drift, Go/session/player-config/platform/private contract checks, and the legacy Player build passed. The generated-tree digest was `c8fed61c96e24b871dbfccee0928ee149fa0691bf3869b58121dc518f68196e1`.
- The strict Player compiler program accepted exactly the five generated TypeScript contracts, and the unchanged production Player Vite build passed.
- Direct Go coverage locks all eight RPC names, paths, input/output types, cardinalities, public request limits, typed Connect errors/results, and edge authorization boundaries.
- Focused ConnectRPC Playwright evidence passed 11 local/product cases; two real-ngrok cases remained skipped under their existing opt-in contract.
- The focused CRT rendering suite passed and the three named immutable hacking-hover snapshots had no Git diff.
- Linux generated-contract/package-probe self-tests passed. The equivalent Windows self-test is implemented but was `NOT RUN` because this validation host is macOS without PowerShell/Windows.

The first sandboxed ConnectRPC Playwright attempt could not bind its local fixture server. It passed unchanged when rerun with local bind permission. Playwright's `NO_COLOR`/`FORCE_COLOR` notices and local TLS-probe handshake messages were observed and judged benign because the focused assertions passed.
