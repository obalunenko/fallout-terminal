# T155 — Player production promotion

Date: 2026-09-01 (Asia/Tbilisi)

Result: PASS.

- Production source selects exactly one `#playerApp`, `/src/main.ts`, and `mountPlayerApp` call.
- The accepted CSP is byte-exact.
- Vue owns the sole production `.crt`, `#screen`, and `#connOverlay` hierarchy.
- The production build retains the stylesheet, Fixedsys font, and all 20 accepted sounds.
- Candidate/staging declarations remain isolated and unselected until T156.
- `task frontend:typecheck:client`: PASS.
- `task frontend:build:client`: PASS, 217 modules; emitted `index.html`, one hashed JS, one hashed CSS, Fixedsys, `.keep`, and all 20 governed sound assets.
- Exact accepted CSP/root/main/mount/source scans: PASS.
- Sorted emitted-file scans found no candidate, fixture, legacy-script, staging, or mixed-ownership marker.
- Focused production selector smoke: 1 passed; `.crt`, `#screen`, and `#connOverlay` each occur exactly once under `#playerApp`.
