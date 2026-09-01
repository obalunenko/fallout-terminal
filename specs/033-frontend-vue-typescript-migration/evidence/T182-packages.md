# T182 Governed Package Matrix

Date: 2026-09-01
Executing host: macOS, darwin/arm64

| Target | Result | Matching-host evidence / reason |
|---|---|---|
| darwin/arm64 | PASS | `task package` completed the governed prepare order and signed bundle; `scripts/verify-macos-app.sh 'build/bin/Fallout Terminal.app'` verified arm64-only Mach-O, macOS 13.0 minimum, identity, separate offline embeds, native frameworks, resources, read-only demo data, reviewed notices, entitlements, no provider/PATH runtime, and final signature. Canonical bundle-manifest SHA-256: `c82bc4f007113fb58e87ab46fdfa6a904dbaf6870df270cf32c297e1f4d5f118`. |
| windows/amd64 | NOT RUN | Requires a matching Windows amd64 host; not inferred from macOS artifacts or package-plan tests. |
| windows/arm64 | NOT RUN | Requires a matching Windows arm64 host; not inferred from macOS artifacts or package-plan tests. |
| linux/amd64 | NOT RUN | Requires a matching Linux amd64 host; not inferred from macOS artifacts or package-plan tests. |
| linux/arm64 | NOT RUN | Requires a matching Linux arm64 host; not inferred from macOS artifacts or package-plan tests. |

Signing verification is local personal-use evidence. Notarization, distribution credentials, and
unavailable-host runtime startup are not claimed.
