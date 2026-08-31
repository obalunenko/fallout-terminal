# T084 — Broadcast controls

The coordination panel and broadcast triggers are now exclusively Vue-owned. The composable applies authoritative revisions in order, allows same-revision pending/status updates, rejects older projections, exposes confirmation state for T085, and releases its bridge subscription on unmount.

Validation:

- `task frontend:typecheck:overseer` — PASS
- bounded legacy TypeScript compiler — PASS
- candidate Vite build — PASS, 102 modules transformed
- exact `broadcast controls preserve revision ordering and cleanup` assertion — PASS, 1/1
- complete affected authoring suite — PASS, 16/16
- legacy migrated-control scan — PASS, zero control-ID references
- `git diff --check` — PASS
