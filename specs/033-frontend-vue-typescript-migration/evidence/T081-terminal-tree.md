# T081 — Terminal tree and node editor

Implemented the terminal authoring tree as one Vue-owned projection surface with recursive stable-key rendering and a colocated Vue-owned node editor.

- `TerminalTree.vue` accepts increasing structurally validated snapshots, emits revision-bound actions, and owns both explicit Teleport leaves.
- `TerminalTreeNode.vue` recursively renders folders, commands, and entries by node ID and preserves row identity across adjacent updates.
- `NodeEditor.vue` keeps field drafts local, validates command-mode requirements before requests, restores invalid-field focus, and owns delete confirmation.
- `overseer.js` retains model mutation, autosave, and canonical reset commands but contains no legacy tree/editor DOM query, renderer, dynamic markup, or handler.
- The exact browser assertion proves nested creation, keyed DOM identity, focus handoff, validation without persistence, and one owner per migrated surface.

Validation:

- `task frontend:typecheck:overseer` — PASS
- `frontend/node_modules/.bin/tsc -p frontend/overseer/tsconfig.legacy.json --noEmit` — PASS
- candidate Vite build — PASS, 98 modules transformed
- exact focused browser assertion `terminal tree preserves recursive stable keys and node validation` — PASS, 1/1
- complete `state-changing-command-authoring.spec.mjs` — PASS, 15/15
- `git diff --check` — PASS
- legacy source scan for `treeView`, `nodeForm`, toolbar IDs, and former render functions — PASS, zero matches
