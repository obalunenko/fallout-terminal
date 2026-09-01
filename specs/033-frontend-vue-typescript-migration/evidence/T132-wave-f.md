# T132 — Wave-f exit

Date: 2026-09-01
Host: macOS
Status: PASS with two credential-conditional skips

## Unconditional results

- Player strict `vue-tsc`: PASS.
- Exact Vite candidate build from `frontend/client/test-fixtures` in mode `candidate`: PASS, 186 modules.
- Candidate runtime inventory: `index.html`, one hashed JavaScript bundle, one hashed CSS bundle, and `Fixedsys` font only.
- Frontend policy/public-only Player dependency boundary: PASS.
- Candidate transport, recognition, navigation, and cleanup suites: 10/10 PASS.
- Legacy production ConnectRPC/CRT browser and visual suite: 41/41 unconditional PASS.
- Approved CRT snapshot directory: clean diff.
- T128 identity/navigation/action integration: 9/9 PASS.
- T130 boundary manifest and candidate boundary suites: 6/6 PASS.

## Cleanup inventory

Exact focused evidence accounts for subscription and action AbortControllers, async iterator return, reconnect and recognition lease timers, recognition storage listeners, ResizeObserver, animation frames, late font-ready callbacks, queued focus work, and Vue unmount. Repeated disposal produces no additional release and late results do not change state.

## Ownership and evidence boundary

The candidate document has one `#playerApp` lifecycle and no adjacent legacy subtree. Production `frontend/client/index.html` remains separately selected and wholly owned by `client.js`, `sound.js`, and `presentation-uplink.js`. Candidate evidence does not claim native embedding or package contents. All candidate/staging and Player legacy temporary mechanisms remain governed through T156.

## Conditional results

The two actual authenticated ngrok endpoint tests were not run because matching endpoint credentials were unavailable. Protected local forwarding, five-client convergence, reconnect, navigation, hacking, sound, and stale shutdown all ran unconditionally.
