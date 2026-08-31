# T074 Wave-d Browser and Visual Parity Evidence

**Date**: 2026-08-31
**Result**: PASS

## Exact gate

The required command completed successfully:

```text
task browser:test
git diff --exit-code -- tests/browser/crt-rendering.spec.mjs-snapshots
```

Results:

- Playwright: 213 passed and two existing real authenticated ngrok journeys were skipped under their credential-dependent opt-in contract.
- Application update, approval/reset, session, player, terminal-group, public-access, clipboard, accessibility, focus, ConnectRPC, navigation, and CRT rendering journeys passed.
- The complete immutable CRT snapshot directory has no Git diff; no visual baseline was updated.
- Browser evidence remains browser/fixture evidence only and does not claim native embedding, real-provider behavior, signing, packaging, or distribution verification.

## Parity correction discovered by the gate

The first complete run produced 205 passes, two expected skips, and eight public-access failures. BUG-015 traced those failures to three integration defects: a raw fallback/reload fixture did not remount the candidate, the closed generated-password dialog's accessible name collided with the established player-password field locator, and App/dialog command projections were detached from the composable that derived panel URL/failure state.

The surgical correction:

- made `usePublicAccess` the single ordered owner for port events, command projections, panel URL/failure state, and legacy-bridge settings snapshots;
- retained the generated dialog in the owned DOM for cleanup evidence while exposing its historical accessible name only while open;
- mounted the candidate explicitly in the established fallback journey and after its intentional raw-page reload.

Both public-access specs then passed 29/29, and the exact full gate passed unchanged. Strict Overseer and bounded legacy compiler checks also passed before the final browser rerun.

## Execution notes

The initial sandboxed Taskfile attempt stalled while Playwright checked/installed Chromium. It was interrupted and the exact gate was rerun with browser-download and local-server permission. Local TLS-probe handshake messages, `NO_COLOR`/`FORCE_COLOR` notices, and missing-Teleport-target warnings from the deliberately empty isolated candidate smoke document were observed; all corresponding governed assertions passed.
