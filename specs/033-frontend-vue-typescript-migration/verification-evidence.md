# Frontend Vue/TypeScript Migration Verification Evidence

Date: 2026-09-01
Primary executing host: macOS, darwin/arm64

## Overall result

All unconditional migration, frontend, protobuf, Wails binding, Go, repository, ownership,
documentation, license, and matching-host macOS package gates passed. Conditional provider,
unavailable-host, interactive native UI, and distribution-signing services are recorded separately
as `NOT RUN`; browser fixtures are not substituted for native or provider evidence.

## Unconditional command matrix

| Evidence | Command / review | Result | Evidence class |
|---|---|---|---|
| T171 | `task frontend:compatibility:check` | PASS | Current/legacy session and player-config semantic round trip. |
| T172 | `task frontend:boundary:check` | PASS | Reviewed frontend boundary manifest; 3/3 mapping assertions. |
| T173 | `scripts/frontend-policy-check.sh --self-test` | PASS | Policy checker positive/negative fixtures. |
| T173 | `task frontend:policy:check` | PASS | Final source/type/lock/dependency/root/temporary policy. |
| T174 | `scripts/reproducible-build-check.sh --self-test` | PASS | Reproducibility checker detects deliberate mismatch. |
| T174 | `task frontend:reproducible:check` | PASS | Two byte-identical Overseer and Player Vite builds. |
| T175 | `scripts/frontend-install-inventory-check.sh --self-test` | PASS | Semantic install/cache/lock/manifest inventory fixtures. |
| T175 | `task frontend:build` | PASS | Sole workspace install plus two production builds. |
| T175 | `task frontend:build:overseer`; `task frontend:build:client` | PASS | Independent no-install application builds. |
| T176 | `task frontend:typecheck:overseer`; `task frontend:typecheck:client`; `task frontend:typecheck` | PASS | Strict isolated and aggregate SFC/TypeScript checks, including five generated Player modules. |
| T177 | `task proto:format:check`; `task proto:lint`; `task proto:breaking`; `task proto:drift:check`; `task proto:generated:check`; `task proto:check` | PASS | Format, lint, five breaking fixtures, exact drift restoration, generated compilation, deterministic aggregate contract. |
| T177 | `git diff --exit-code -- proto internal/gen frontend/client/gen` | PASS | Schema and generated trees clean. |
| T178 | `task bindings:check` | PASS | Exact 39 Wails methods, seven named events, adapter/alias sync and Player isolation. |
| T178 | `git diff --exit-code -- frontend/overseer/bindings` | PASS | Generated Wails tree clean. |
| T179 | `task browser:test` | PASS | Browser/visual only: 276 passed, two credential-qualified tests skipped, zero failed. |
| T179 | `git diff --exit-code -- tests/browser/crt-rendering.spec.mjs-snapshots` | PASS | Twelve approved CRT snapshots unchanged. |
| T180 | Exact 15-row Go workflow audit | PASS | Five task-time workflow fields passed for every Go-changing task. |
| T180 | `task fmt:check`; `task vet`; `task lint`; `task test`; `task test:race` | PASS | macOS-qualified Go formatting, vet, zero lint issues, unit and race suites. |
| T180 | `task startup:check`; `task ci:quality`; `task check` | PASS | Startup, CI-quality, and aggregate repository gates. |
| T181 | `task startup:check` | PASS | Native startup/resource/workflow Go contracts. |
| T181 | `scripts/wails-v3-cutover-check.sh` | PASS | Static Wails/native cutover. |
| T181 | `scripts/secret-leak-check.sh` | PASS | Static private-secret boundary. |
| T181 | `scripts/legacy-public-access-check.sh` | PASS | Static single-runtime public-access ownership. |
| T182 | `task package` on darwin/arm64 | PASS | Governed matching-host package preparation and local signing. |
| T182 | `scripts/verify-macos-app.sh 'build/bin/Fallout Terminal.app'` | PASS | macOS arm64/13.0 identity, resources, embeds, notices, entitlements, provider isolation and signature; digest `c82bc4f007113fb58e87ab46fdfa6a904dbaf6870df270cf32c297e1f4d5f118`. |
| T183 | `scripts/frontend-task-contract-check.sh --self-test --expected-target-count 10` | PASS | Exact Node, ten targets, install/order/isolation/failure propagation. |
| T183 | macOS-qualified `go test ./internal/buildtool` | PASS | Taskfile/CI/buildtool and five package-plan contracts. |
| T184 | `scripts/dependency-license-check.sh` | PASS | Shipped Go and Vue runtime pins plus reviewed notices. |
| T192 | `task frontend:policy:check`; exact root/register counts | PASS | One root per app, 18 closed mechanisms, zero open inventory. |
| T192 | Rollback ancestry and historical-spec clean diff | PASS | Immutable rollback retained; completed historical specs unchanged. |

## Conditional and unavailable evidence

| Surface | Result | Host / exact reason |
|---|---|---|
| Two actual authenticated ngrok browser journeys | NOT RUN | External endpoint credentials were not supplied; local authenticated forwarding/fallback browser tests passed but are not provider evidence. |
| Default development-bundle reset smoke | FAIL | The stale `build/dev/Fallout Terminal.app` was rejected by LaunchServices with `kLSNoExecutableErr`; it is not accepted evidence. |
| Current packaged native approval/reset/reopen UI smoke | NOT RUN | darwin/arm64 package launched, but macOS Accessibility automation could not prepare the window. Diagnostics: `/private/tmp/fallout-native-reset.reALdA`. Browser evidence is not substituted. |
| windows/amd64 package/startup | NOT RUN | No matching Windows amd64 host. |
| windows/arm64 package/startup | NOT RUN | No matching Windows arm64 host. |
| linux/amd64 package/startup | NOT RUN | No matching Linux amd64 host. |
| linux/arm64 package/startup | NOT RUN | No matching Linux arm64 host. |
| Developer ID signing, notarization, stapling, Gatekeeper distribution | NOT RUN | Credentials/services were unavailable and the optional distribution workflow was not executed. Local personal-use package signature verification passed only for the matching macOS artifact. |

## Active documentation and template review

| Artifact | Result | Accepted content |
|---|---|---|
| `README.md` | PASS | One install, two Vue apps, exact Node, ten targets, generated TypeScript, typed Wails adapter, browser/native distinction. |
| `ARCHITECTURE.md` | PASS | Sole roots/bundles/adapters/state, public Player, private Overseer, immutable authority. |
| `CONTRIBUTING.md` | PASS | Exact Node/install/targets, RED/GREEN, exact scope, Go workflow, safe parallelism. |
| `docs/platform-packaging.md` | PASS | Separate embeds/resources, five matching-host targets, license and `NOT RUN` rules. |
| Active plan/spec/tasks templates | PASS | Exact paths/read-only inputs, DAGs, local checks, RED/GREEN, Go, temporary ownership, traceability, honest parallelism/evidence. |

Detailed command output and assertions are retained in `evidence/T171-compatibility.md` through
`evidence/T184-license.md` and `evidence/T192-final-ownership.md`. This matrix follows the
verification classes and conditional rules in `quickstart.md`.
