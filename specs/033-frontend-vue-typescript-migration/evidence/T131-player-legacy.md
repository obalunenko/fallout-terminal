# T131 — Legacy production Player remains unchanged

Date: 2026-09-01
Host: macOS
Evidence class: production legacy browser/visual and selection separation

## Governed command

```text
npm test --prefix tests/browser -- connectrpc-player.spec.mjs crt-rendering.spec.mjs
git diff --exit-code -- tests/browser/crt-rendering.spec.mjs-snapshots
scripts/frontend-assert-no-match.sh 'candidate([-_/.]*(main|player|index\.html))|player[-_/.]*candidate|test[-_/.]*fixtures' frontend/client/index.html
```

## Result

- Production legacy browser/visual suite: 41 PASS, 2 credential-qualified real-ngrok tests SKIPPED, 0 failures.
- Immutable CRT snapshot directory diff: clean.
- Production `frontend/client/index.html` candidate/test-fixture reference scan: PASS, pattern absent.
- Production document still selects `client.js` and retains the legacy-owned `.crt`, `#screen`, and `#connOverlay` tree; the candidate runs only on its isolated test server.

The first sandboxed attempt was unable to bind the local fixture server. The unchanged command passed after local loopback permission was granted. The two real-ngrok tests remain conditional because matching endpoint credentials were not present; all local and protected-forwarding behavior ran.
