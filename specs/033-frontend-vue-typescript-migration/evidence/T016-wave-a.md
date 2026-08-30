# T016 Wave-a Exit Evidence

Date: 2026-08-30

## Ownership conclusion

- Overseer production remains wholly owned by `frontend/overseer/src/index.html`, `overseer.js`, and `desktop-api.js`.
- Player production remains wholly owned by `frontend/client/index.html`, `client.js`, `sound.js`, and `presentation-uplink.js`.
- No production Vue mount, candidate input, cross-owned subtree, or shared application store was introduced.
- The bounded legacy compiler rows created by T007 and T008 remain open with their exact two-file and three-file inventories.

## Verification

The exact T016 command completed successfully:

```text
task frontend:typecheck
task frontend:build
task frontend:policy:check
task frontend:reproducible:check
npm test --prefix tests/browser -- connectrpc-player.spec.mjs crt-rendering.spec.mjs
git diff --exit-code -- tests/browser/crt-rendering.spec.mjs-snapshots/compact-active-character-option-darwin.png tests/browser/crt-rendering.spec.mjs-snapshots/medium-active-character-option-darwin.png tests/browser/crt-rendering.spec.mjs-snapshots/large-active-character-option-darwin.png
```

Results:

- Overseer, Player, and aggregate strict type-checks passed.
- The aggregate build performed one clean `npm ci --prefix frontend`, then both isolated production builds passed.
- Policy validation passed for exact pins, workspace/lockfile ownership, compiler policy, and the Player privilege boundary.
- Two clean builds produced identical sorted mode/size/SHA-256 manifests:
  - Overseer: `d72a8504f405e124fcefc7269e4d85e7a4d359b99c3fb72afaef8fecb201d5fc`
  - Player: `12cdc02aa72923ec920184525d9804b435262a52a43bd263fcac1bc721d87436`
- Playwright: 41 passed, 2 credential/provider-dependent journeys skipped by their existing opt-in contract.
- The three named immutable CRT snapshots had no Git diff.

The first sandboxed Playwright attempt could not bind the local fixture server. The same focused command was rerun with local bind permission and passed; this was an execution-environment restriction, not a product or test failure.
