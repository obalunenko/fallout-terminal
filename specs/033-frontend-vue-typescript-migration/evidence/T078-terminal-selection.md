# T078 terminal-selection evidence

Date: 2026-08-31

The terminal selection slice moved row rendering and action-trigger ownership from the legacy renderer into keyed Vue components. The legacy side now publishes detached, monotonically revised snapshots and accepts only requests bound to the current revision.

Validation:

- `task frontend:typecheck:overseer` — passed.
- `npx --prefix frontend vite build frontend/overseer/test-fixtures --config frontend/overseer/vite.config.ts --mode candidate --outDir <isolated-output>` — passed with 79 modules transformed.
- `npx tsc -p frontend/overseer/tsconfig.legacy.json --noEmit` — passed.
- Focused assertion `terminal selection preserves stable rows stale suppression and focus` — passed, 1 test.

The focused browser assertion verifies unique stable terminal IDs, keyed DOM identity through selection, Enter-key activation with retained focus, one accessible action trigger per row, stale projection suppression, and idempotent unmount cleanup with no late re-render.
