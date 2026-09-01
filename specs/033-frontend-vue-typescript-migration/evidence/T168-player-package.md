# T168 Player package evidence

Date: 2026-09-01
Host: Darwin arm64
Result: PASS

## Matching-host package and identity

- `task package`: PASS. The governed build installed the locked frontend workspace, verified the generated contracts and platform assets, typechecked and built the Player and Overseer independently, regenerated Wails bindings, verified the dependency/license inventory, and produced `build/bin/Fallout Terminal.app`.
- The packaged executable is `Contents/MacOS/Fallout Terminal`, a Mach-O 64-bit arm64 executable. `Info.plist` names the same executable, bundle identifier `com.vaulttec.fallout-terminal`, development version `0.0.0`, and minimum macOS version `13.0`.
- `scripts/verify-macos-app.sh`: PASS. The application satisfied its designated requirement, the signature was valid, reviewed resources and entitlements were complete, both frontend distributions were offline, and the canonical bundle-manifest SHA-256 was `9dcfaf8e7bf0143ecc077607a18640591027d354f53301171ba6f85a20393f48`.

## Player runtime content

- The Player and Overseer distributions remained separate embed inputs. The Player inventory contains one `index.html`, one hashed JavaScript module, one hashed stylesheet, the emitted Fixedsys font, and 20 governed sound files. The Overseer inventory contains its own `index.html`, hashed JavaScript module, hashed stylesheet, and emitted Fixedsys font.
- The Player and Overseer emitted font SHA-256 values both equal the reviewed Player source font SHA-256: `6ee0f3573bc5e33e93b616ef6282f49bc0e227a31aa753ac76ed2e3f3d02056d`.
- Package resources contain the reviewed demo session, demo player configuration, icons, and third-party notices. The matching-host verifier proved their expected identity, permissions, and signature ordering.
- Exact filename and readable-content scans found no authored `.ts`, `.tsx`, `.vue`, or source-map output; no candidate/test-fixture path; no legacy `client.js`, `sound.js`, or `presentation-uplink.js`; and no cross-application, Wails, or private-contract reference in the Player distribution.

## Host qualification

- macOS arm64: PASS through the matching-host package and macOS verifier above.
- Linux package/startup verifier: NOT RUN on Darwin; a matching Linux host with its required display/window-manager and credential-store classification is required.
- Windows package/startup verifier: NOT RUN on Darwin; a matching Windows host and PowerShell verifier are required.

Browser-only evidence was not promoted to package evidence. Final state-changing native lifecycle verification remains independently owned by T181.
