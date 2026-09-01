# T093 Overseer functional parity evidence

Date: 2026-08-31

## Exact parity command

```sh
npm test --prefix tests/browser -- application-update.spec.mjs desktop-api.spec.mjs player-management.spec.mjs player-sessions-control.spec.mjs public-access-fallback.spec.mjs public-access-settings.spec.mjs state-changing-command-approval.spec.mjs state-changing-command-authoring.spec.mjs state-changing-command-sync.spec.mjs terminal-grouping.spec.mjs terminal-navigation.spec.mjs
```

Result: PASS — 173 tests passed in 2.6 minutes with one Playwright worker.

The passing run covers application-update ordering and focus, the production typed desktop adapter, player/configuration and logical-session management, public-access lifecycle and secret handling, command approval and reset synchronization, terminal authoring, terminal grouping, and terminal navigation including stale/reconnect races.

## Production-root clarification

The eleven retained suites were scanned for `candidate-main.ts`, port `34120`, `#overseerVueLeaves`, and `#legacyOverseerRoot`; all were absent. Overseer application journeys use `tests/browser/fixtures/overseer-app.ts`, which mounts the production `#overseerApp` through the typed port. No retained suite outside the dedicated desktop-adapter contract test consumes `window.desktopAPI`.

## Supporting gates

- `task frontend:typecheck:overseer`: PASS.
- `task frontend:policy:check`: PASS.
- BUG-024 through BUG-032 are patched consistently in `spec.md`, `plan.md`, and `tasks.md`.
- The task inventory remains unique and gapless from T001 through T195.

Browser evidence uses the permanent test-only fake desktop boundary and is not Wails/native evidence.
