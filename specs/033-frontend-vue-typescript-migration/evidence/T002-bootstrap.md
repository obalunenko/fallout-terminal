# T002 Bootstrap Baseline

Date: 2026-08-30

Rollback baseline: `06696ee1c7155a1bb1135ef46ec91445dd73a2a4` is an ancestor of the implementation branch.

Toolchain selected from the repository root:

- `.nvmrc`: `26.8.1`
- active Node.js: `v26.8.1`
- npm: `11.19.0`
- Task: `3.53.1`

The repository already had the `.nvmrc` Node runtime active, so no alternate runtime or install workflow was used.

## Existing-check transcript

| Command | Exit | Result |
|---|---:|---|
| `task node:check` | 0 | Existing Node preflight accepted the active `v26.8.1` runtime. |
| `npm ci --prefix frontend` | 0 | One clean workspace install completed and added 28 packages; npm reported 8 packages seeking funding. |
| `npm run build:overseer --prefix frontend` | 0 | Vite 8.1.5 transformed 40 modules and produced the existing Overseer `dist` HTML, CSS, JavaScript, and Fixedsys font assets. |
| `npm run build:client --prefix frontend` | 0 | Vite 8.1.5 transformed 145 modules and produced the existing Player `dist` HTML, CSS, JavaScript, and Fixedsys font assets. |
| `task proto:drift:check` | 0 | The governed self-test generated contracts from schema revision `769618795e7e28f3f3f490d6b85688ca28f1553ee819fbfc00d8de5f3e8946c9`, observed the intended clean-generation drift diagnostic, and reported that checked-in drift is rejected before deterministic verification. |
| `task bindings:check` | 0 | Wails bindings were deterministic with exactly 39 accepted desktop methods and seven named events. |

## Scope check

The checks used only targets and package scripts present before the Vue/TypeScript migration. They did not invoke any future `frontend:typecheck`, `frontend:policy:check`, `frontend:reproducible:check`, `frontend:compatibility:check`, or `frontend:boundary:check` target. Generated build directories remained ignored; the only tracked implementation changes after the run were Companion bookkeeping and this feature's T001/T002 records.
