# T107 Player candidate selection evidence

Date: 2026-09-01

## Candidate browser build

The explicit `candidate` mode selected `frontend/client/test-fixtures/index.html` and its
`candidate-main.ts` entry. It omitted the production sound-copy plugin and compiled the isolated
Vue mount plus the stylesheet/font referenced by that entry:

```text
assets/Fixedsys-C16VDDoP.ttf
assets/index-3BoYKyIP.js
assets/index-ViXS5qWC.css
index.html
```

Command: `npx --prefix frontend vite build frontend/client/test-fixtures --config frontend/client/vite.config.ts --mode candidate --outDir <isolated-output>`

Result: PASS. `index.html` was readable and the Player strict TypeScript program also passed.

## Playwright configuration and exact route smoke

- Importing `tests/browser/playwright.config.mjs` returned a configuration with three servers: the
  normal browser fixture, production Overseer, and isolated Player candidate Vite server.
- The Player candidate server is test-only at `http://127.0.0.1:34120/`.
- `scripts/frontend-focused-browser-check.sh tests/browser/player-candidate-smoke.spec.mjs 'player candidate route resolves from test-only entry'` selected exactly one test and passed it.
- The smoke observed the candidate title, exactly one `#playerApp`, no legacy `#screen`, and no
  page/console error. Playwright bypasses CSP only because Vite's development CSS transform injects
  a style element; the source CSP and production build are unchanged.

Result: PASS — 1 test.

## Production browser build and absence proof

Production mode continued to select `frontend/client/index.html`, whose line 108 retains the exact
legacy entry `<script type="module" src="client.js"></script>`:

```text
.keep
assets/Fixedsys-C16VDDoP.ttf
assets/index-ViXS5qWC.css
assets/index-YvZQN5ET.js
index.html
sounds/.DS_Store
sounds/README.txt
sounds/ambient/obj_computerzax_hum_lp.wav
sounds/charscroll/ui_hacking_charscroll.wav
sounds/enter/ui_hacking_charenter_01.wav
sounds/enter/ui_hacking_charenter_02.wav
sounds/enter/ui_hacking_charenter_03.wav
sounds/hack-bad/ui_hacking_passbad.wav
sounds/hack-good/ui_hacking_passgood.wav
sounds/menu-focus/ui_menu_focus.wav
sounds/multiple/ui_hacking_charmultiple_01.wav
sounds/multiple/ui_hacking_charmultiple_02.wav
sounds/multiple/ui_hacking_charmultiple_03.wav
sounds/multiple/ui_hacking_charmultiple_04.wav
sounds/single/ui_hacking_charsingle_01.wav
sounds/single/ui_hacking_charsingle_02.wav
sounds/single/ui_hacking_charsingle_03.wav
sounds/single/ui_hacking_charsingle_04.wav
sounds/single/ui_hacking_charsingle_05.wav
sounds/single/ui_hacking_charsingle_06.wav
sounds/single/ui_hacking_charsingle_07.wav
sounds/single/ui_hacking_charsingle_08.wav
```

Command: `npx --prefix frontend vite build frontend/client --config frontend/client/vite.config.ts --mode production --outDir <isolated-output>`

Every emitted readable production file passed the status-aware no-match check for candidate names,
Player-candidate markers, and test-fixture paths. `frontend/client/vite.config.ts` contains the
explicit candidate-mode/test-fixture selection, and no other mode selects it.

Result: PASS. This is browser/Vite selection evidence only. Native embed/resource selection remains
owned by T165/T181, while package-content and final package integrity remain owned by T168/T182.
