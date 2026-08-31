# T085 — Broadcast confirmation dialogs

The take-off-air and end-broadcast confirmations are now separate Vue-owned native dialogs backed by one lifecycle-scoped broadcast composable.

The exact assertion proves distinct cancellation and focus restoration, zero commands on cancel, duplicate-submit suppression, newer-revision stale completion suppression, surviving-dialog state, and native close/removal on unmount. Existing authoring journeys additionally prove clear failure presentation, unfinished-progress handoff, authoritative clear convergence, and focus on the surviving broadcast control.

Validation:

- Overseer strict and bounded legacy typechecks — PASS
- candidate Vite build — PASS, 106 modules transformed
- exact T085 assertion — PASS, 1/1
- combined affected browser specs — PASS, 18/18
- legacy dialog-owner scan — PASS, zero matches
- `git diff --check` — PASS
