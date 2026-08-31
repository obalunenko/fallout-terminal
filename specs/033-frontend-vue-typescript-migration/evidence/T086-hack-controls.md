# T086 — Hacking controls

The hacking status panel and both control actions are now exclusively Vue-owned. The composable accepts authoritative snapshots under a monotonic receipt revision, rejects stale explicit revisions, invalidates state when the live terminal changes, builds reset requests from detached terminal data, and never applies command results as hack state.

The exact assertion covers stale-event suppression, unchanged state after command completion, synchronous duplicate-command rejection, the complete failed-hack reset payload, subsequent authoritative convergence, DOM removal, and exact-once subscription cleanup.

Validation:

- Overseer strict and bounded legacy typechecks — PASS
- candidate Vite build — PASS, 109 modules transformed
- exact `Overseer hacking controls preserve authoritative revisions and cleanup` assertion — PASS, 1/1
- adjacent retained-progress terminal-navigation assertion — PASS, 1/1
- legacy hacking-control owner scan — PASS, zero matches
- changed-code simplification review — PASS, no behavior-preserving reduction warranted
- `git diff --check` — PASS
