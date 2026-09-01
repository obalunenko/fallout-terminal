# T130 — Wave-f Player candidate build and public boundary

Date: 2026-09-01
Host: macOS
Evidence class: strict compiler, isolated browser build, runtime-path inventory, dependency policy, browser boundary

## Governed input

- Config: `frontend/client/vite.config.ts`
- Mode: `candidate`
- Input root: `frontend/client/test-fixtures`
- Output: a fresh `/tmp/fallout-player-T130.*` directory, removed after inspection

## Results

- `task frontend:typecheck:client`: PASS.
- `task frontend:typecheck`: PASS for Player and Overseer strict programs.
- Candidate Vite build: PASS, 186 modules transformed.
- `task frontend:policy:check`: PASS, including the Player public-only dependency boundary.
- `frontend-boundary-manifest.spec.mjs` plus `player-candidate-boundary.spec.mjs`: 6/6 PASS.
- Exact `candidate App owns one session one stream and cascading cleanup`: PASS.

Normalized runtime inventory:

```text
assets/Fixedsys-C16VDDoP.ttf
assets/index-BYpo_6bn.js
assets/index-ViXS5qWC.css
index.html
```

The exact filename/path scan returned no `.ts`, `.tsx`, `.vue`, authored source map, candidate fixture source, generated-source path, Wails, Overseer, private/internal/shared-store path, or legacy `client.js`, `sound.js`, or `presentation-uplink.js` artifact.

The initial sandboxed browser attempt was infrastructure-only (`listen 127.0.0.1:34119: operation not permitted`). The unchanged complete command was rerun with local loopback permission and passed; no product assertion was changed.

Production Player selection remains legacy and is not claimed by this candidate-build evidence.
