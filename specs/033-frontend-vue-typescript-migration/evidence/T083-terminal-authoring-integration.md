# T083 — Terminal-authoring composable integration

All migrated terminal authoring surfaces now consume one typed, atomic authoring state boundary.

- `useTerminalAuthoring.ts` validates ordered groups and exact terminal membership before applying a snapshot.
- Selection, group actions, editor actions, tree mutations, and creation requests share one current revision gate.
- The composable provides a filtered bridge to the read-only migrated components and owns the single underlying legacy subscription cleanup.
- `overseer.js` publishes/replays one `terminal-authoring-snapshot`; separate authoring projection variables and counters are absent.
- The exact assertion proves revision equality across all five surfaces, monotonic update after selection, source-level DOM independence, obsolete-state absence, and unmount cleanup.

Validation:

- `task frontend:typecheck:overseer` — PASS
- bounded legacy TypeScript compiler — PASS
- candidate Vite build — PASS, 99 modules transformed
- exact T083 browser assertion — PASS, 1/1
- complete authoring suite — PASS, 16/16
- complete terminal-grouping suite — PASS, 20/20
- `git diff --check` — PASS
