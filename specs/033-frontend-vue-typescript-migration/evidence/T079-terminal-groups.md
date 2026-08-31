# T079 terminal-group evidence

Date: 2026-08-31

The terminal-group list moved to keyed Vue group rows with exact-one terminal membership validation, revision-gated detached actions, local collapse state, component-owned menu focus/listener cleanup, and no legacy `#termList` rendering.

BUG-018 corrected the post-open revision seam discovered by the governed assertion: `useSessionDocument` refreshes runtime status after document acquisition, transfers the validated `savedRevision`, and initializes the temporary legacy mutation owner before group confirmation. The assertion requires the backend to accept and persist the intended reorder; counting a rejected command is insufficient.

Validation:

- `task frontend:typecheck:overseer` — passed.
- `frontend/node_modules/.bin/tsc -p frontend/overseer/tsconfig.legacy.json --noEmit` — passed.
- `npx --prefix frontend vite build frontend/overseer/test-fixtures --config frontend/overseer/vite.config.ts --mode candidate --outDir <isolated-output>` — passed with 84 modules transformed.
- Focused assertion `terminal group list preserves order atomicity and stable keys` — passed, 1 test.
- Affected runtime-shell regression assertion — passed, 1 test.
- `git diff --check` — passed.
- BUG-018 structural consistency — passed with 18 patched reports, 195 unique task IDs, and T079 active until this evidence was folded.

The focused assertion verifies direct group ownership, exact membership/order, stale projection suppression, collapse retention, zero mutation before confirmation, dialog-trigger focus restoration, accepted canonical persistence, keyed DOM identity through reorder, and repeated-unmount/late-projection cleanup.
