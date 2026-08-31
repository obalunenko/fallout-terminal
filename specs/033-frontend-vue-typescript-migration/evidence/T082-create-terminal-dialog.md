# T082 — Create-terminal dialog

`CreateTerminalDialog.vue` now owns the complete create-terminal modal subtree and lifecycle. The legacy trigger crosses the temporary boundary only through revisioned snapshots and requests; the legacy application retains the terminal-domain mutation and persistence operation.

The focused assertion proves Escape cancellation and trigger-focus restoration, blank-name validation with no save, stale-request suppression, trimmed atomic creation with one save, no implicit activation, and native dialog/bridge cleanup on unmount.

Validation:

- `task frontend:typecheck:overseer` — PASS
- `frontend/node_modules/.bin/tsc -p frontend/overseer/tsconfig.legacy.json --noEmit` — PASS
- candidate Vite build — PASS, 100 modules transformed
- exact focused browser assertion `create-terminal dialog preserves validation atomicity and focus` — PASS, 1/1
- legacy create-dialog owner scan — PASS, zero migrated dialog references
