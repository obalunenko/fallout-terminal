# T039 Overseer candidate selection evidence

Date: 2026-08-30

## Candidate build

The explicit candidate-mode build used `frontend/overseer/test-fixtures/index.html`, `candidate-main.ts`, production SFC source, and the browser-only fake `DesktopPort`:

```text
./.keep
./assets/Fixedsys-C16VDDoP.ttf
./assets/index-BEbZ9K9K.js
./assets/index-C1WRKiI-.css
./index.html
```

Command: `npx --prefix frontend vite build frontend/overseer/test-fixtures --config frontend/overseer/vite.config.ts --mode candidate --outDir <isolated-output>`

Result: PASS. The output document was readable and the candidate entry compiled through the production SFC graph without the Wails plugin.

## Browser/config selection

- Importing `tests/browser/playwright.config.mjs` returned a configuration with the normal fixture server and the isolated candidate Vite server.
- `scripts/frontend-focused-browser-check.sh tests/browser/overseer-candidate-smoke.spec.mjs 'overseer candidate route resolves from test-only entry'` selected exactly one test.
- The smoke opened only `http://127.0.0.1:34120/` and observed exactly one attached `#overseerApp` root.

Result: PASS (1 test).

## Production build and absence proof

Production mode continued to select `frontend/overseer/src/index.html`, whose exact legacy inputs remain `desktop-api.js` and `overseer.js`:

```text
./.keep
./assets/Fixedsys-C16VDDoP.ttf
./assets/index-BY2w2OUv.js
./assets/index-C1WRKiI-.css
./index.html
```

Command: `npx --prefix frontend vite build frontend/overseer/src --config frontend/overseer/vite.config.ts --mode production --outDir <isolated-output>`

Every emitted readable production file passed the status-aware no-match check for candidate names and test-fixture paths. Candidate mode is explicit in `vite.config.ts`; production output contains no candidate entry, path, or marker.

Result: PASS. This is browser/test evidence only and makes no native, embedding, or package claim.
