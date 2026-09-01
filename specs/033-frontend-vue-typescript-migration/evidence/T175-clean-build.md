# T175 — Governed clean frontend build

Date: 2026-09-01 (Asia/Tbilisi)

Result: PASS.

## Inventory self-test

The new semantic inventory checker accepted a sole workspace dependency installation plus application `.vite-temp` configuration caches. It rejected, with actionable diagnostics, an app-local dependency, app-local `node_modules/package.json`, package binary, sibling cache, additional lockfile, dependency/cache payload, and missing required path.

## Before and after inventories

- `dependency-install-roots`: exactly `frontend/node_modules` after the governed install.
- `vite-cache-paths`: exactly `frontend/client/node_modules/.vite-temp` and `frontend/overseer/node_modules/.vite-temp`; both app-local `node_modules` directories contain no other direct child.
- `lockfiles`: exactly `frontend/package-lock.json` before and after.
- `manifests`: exactly `frontend/package.json`, `frontend/client/package.json`, and `frontend/overseer/package.json` before and after.

`task frontend:build` performed the single locked workspace install and both builds. Independent `task frontend:build:overseer` and `task frontend:build:client` then passed without installing.

## Immutable manifest/lock hashes

- `frontend/package.json`: `7f43c4faa35e2b992b95af87402f6f3c427d978411e543255eefc877b89777a8`
- `frontend/client/package.json`: `d395e1c2ef225b5d4b056e1432e13a96843a7ff4192cb1829513dc1d5f651fe8`
- `frontend/overseer/package.json`: `e3835a88dcf43c46191d10968a302899098be1663d04989a87a86f849972a5f1`
- `frontend/package-lock.json`: `ffe94b6044bf853745bf02f1d629d426cfeed8f3b6d5701516a5f46b2e6fe0ce`

The exact four-file hash set, Git diff, and lockfile inventory matched before and after. `.vite-temp` was reported only as generated configuration cache, never as an install root, dependency payload, or runtime artifact.
