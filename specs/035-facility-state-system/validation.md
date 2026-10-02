# Validation: Shared Facility State System

This record maps the feature success criteria and Constitution Principles VIII and IX to reproducible evidence. The package, browser, contract, build, and supported-host packaging evidence below passed on the completed implementation.

| Requirement | Automated evidence | Result |
|---|---|---|
| SC-001 — 100 atomic multi-device approvals | `internal/session/facility_test.go`; `internal/control/facility_test.go`; `tests/browser/facility-player-state.spec.mjs` | Package and browser evidence passed; one save and one facility revision per successful action. |
| SC-002 — no pre-approval or partial mutation | `internal/session/facility_test.go`; `internal/control/facility_test.go`; `tests/browser/facility-player-state.spec.mjs` | Rejection, dismissal, stale, conflict, duplicate, and failed persistence leave the canonical facility unchanged. |
| SC-003 — five terminals in three groups converge within one second | `internal/live/facility_test.go`; `internal/player/public_stream_test.go`; `tests/browser/facility-player-state.spec.mjs` | Shared label, block, visibility, and availability projections converge for controller, observers, and reconnects. |
| SC-004 — 100 group moves preserve facility identity | `internal/control/service_test.go`; `internal/session/service_test.go`; `tests/browser/facility-player-state.spec.mjs` | Group changes do not clone, reset, delete, or retarget facility values. |
| SC-005 — deterministic faults and state-free visual effects | `internal/live/facility_test.go`; `internal/domain/facility_test.go`; `tests/browser/facility-diagnostics.spec.mjs` | Repeated projection is identical; visual replay changes no facility or authored-content digest. |
| SC-006 — complete lifecycle restoration | `internal/session/service_test.go`; `internal/control/service_test.go`; `app_test.go`; `tests/browser/facility-lifecycle.spec.mjs` | Broadcast, reload, process, update-handoff, and reconnect paths install the durable facility before resumed player presentation. |
| SC-007 — exact reset scope and one revision | `internal/session/facility_test.go`; `internal/control/facility_test.go`; `app_test.go`; `tests/browser/facility-authoring.spec.mjs` | Device reset affects the device and directly scoped conditions; facility reset restores every authored initial value atomically. |
| SC-008 — authoring validation and cancellation | `internal/domain/facility_test.go`; `internal/session/facility_test.go`; `tests/browser/facility-authoring.spec.mjs` | Invalid references, graphs, overlaps, and recovery paths fail before publication; cancel keeps the canonical graph. |
| SC-009 — correlated redacted retained records | `internal/control/facility_test.go`; `internal/control/service_test.go`; `app_test.go`; `internal/diagnostics/retained_log_test.go`; `tests/browser/overseer-runtime-logs.spec.mjs` | Correlated action, decision, revision, and transition evidence is retained; seeded protected values produce no matches. |
| SC-010 — version-1 compatibility and legacy behavior | `internal/domain/json_test.go`; `internal/session/contract_test.go`; `internal/session/service_test.go`; existing command, transition, hacking, role, and EntryContent suites | Absent facility data remains absent and legacy behavior continues through the additive contract. |
| SC-011 — retries and concurrency commit at most once | `internal/session/facility_test.go`; `internal/control/facility_test.go`; `tests/browser/facility-player-state.spec.mjs` | Serialized mutation, fingerprints, revision checks, and cached outcomes prevent duplicate or split commits. |
| SC-012 — escape and recovery remain available | `internal/nav/nav_test.go`; `internal/control/facility_test.go`; `tests/browser/facility-diagnostics.spec.mjs`; `tests/browser/facility-authoring.spec.mjs` | Back/acknowledgement remain authoritative and private recovery works when player capabilities are blocked. |
| Constitution VIII — every player semantic activity is visible to the Overseer | `internal/control/service_test.go`; `internal/player/server_test.go`; `internal/player/handler_test.go`; `app_test.go`; `internal/diagnostics/retained_log_test.go`; `tests/browser/overseer-runtime-logs.spec.mjs` | Request/outcome, recognition, connection, role, navigation, command, hacking, facility, recovery, reset, failure, and disconnect evidence is correlated, retained, redacted, and independent from gameplay authority. |
| Constitution IX — complete, diegetic, coherent demo | `sessions/demo.json`; `sessions/demo-capability-paths.md`; `internal/platform/assets_test.go`; `tests/browser/demo-session.spec.mjs`; `tests/browser/state-changing-command-authoring.spec.mjs` | All session-driven modes have reachable paths; levels 0–5, roles, state snapshots, facility branches, same-terminal grouping, returns, recovery, preview/reset, version-1 unknown fields, dead ends, and prohibited warnings are checked. |

## Focused evidence

- `npm test --prefix tests/browser -- facility-player-state.spec.mjs`
- `npm test --prefix tests/browser -- facility-authoring.spec.mjs`
- `npm test --prefix tests/browser -- facility-diagnostics.spec.mjs`
- `npm test --prefix tests/browser -- facility-lifecycle.spec.mjs`
- `npm test --prefix tests/browser -- overseer-runtime-logs.spec.mjs`
- `npm test --prefix tests/browser -- demo-session.spec.mjs`
- `go test ./internal/platform -run TestBundledDemo`
- `go test ./internal/domain ./internal/session ./internal/live ./internal/control ./internal/player ./internal/diagnostics`

## Final gates

T111 completed the final repository-wide gates on 2026-09-04 after the intentional protobuf descriptor, compatibility-baseline, schema-revision, and deterministic display-instability asset contracts were updated:

- `task vet` — passed with the repository's macOS 13 deployment and CGO settings.
- `task lint` — passed with `0 issues`.
- `task test` — passed for every Go package, including HTTP, gRPC, tunnel, persistence, platform, player, control, and session integration packages.
- `task test:race` — passed for every Go package; the atomic facility transaction, coordination, projection, retained-log, and lifecycle paths reported no races.

T109 separately passed protobuf format, lint, deterministic generation, drift, breaking-change, and Wails binding checks. T110 passed the clean frontend build and 40 focused browser checks: 35 facility/retained-log scenarios, four desktop facility API checks, and one CRT display-instability check.

## Constitutional convergence evidence

T117–T123 reconciled the completed feature with the amended Constitution on 2026-09-04. The final results were:

- `go fix ./...` — passed with no modernization drift; all changed Go files were then formatted and `gopls check` reported no diagnostics.
- `task check` — passed: formatting, `go vet`, pinned lint (`0 issues`), the complete race-enabled Go suite, protobuf format/lint/generation/drift/breaking checks, generated client build, deterministic Wails bindings, and Spec Kit updater tests.
- `task test` — passed for every Go package without the race detector as the separately required ordinary test gate.
- `task frontend:build` — passed for the Overseer and player production bundles from the locked Node.js 26.8.1 workspace.
- `npm test --prefix tests/browser` — passed 252 journeys; the two real authenticated ngrok journeys were conditionally skipped and are recorded below as `NOT RUN`.
- `scripts/wails-v3-cutover-check.sh --self-test` and `scripts/wails-v3-cutover-check.sh` — passed; Wails `v3.0.0-beta.15` is the accepted runtime and all v2 migration/rollback material is historical-only.
- `task startup:check` — passed the current startup, Taskfile, quality-workflow, portable-release, and distribution contracts.
- `scripts/secret-leak-check.sh` — passed; secret-bearing fields remain confined to the allowed private boundaries.
- `task build` — passed the governed macOS arm64 desktop build after deterministic contract and binding generation.
- `task package` — passed on the supported macOS arm64 host, including both frontend builds, dependency-license verification, resource installation, compilation, and application-bundle signing.
- `git diff --check HEAD` — passed after the final documentation and generated-artifact reconciliation.

### Conditional checks

- `NOT RUN` — two real authenticated ngrok Playwright journeys; no real endpoint credentials were supplied, so the complete browser suite reported them as conditional skips.
- `NOT RUN` — `scripts/runtime-logs-macos-smoke.sh`; this optional packaged-application runtime smoke requires launching and inspecting an external desktop process and is not a required local completion gate.
- `NOT RUN` — matching-host packages for `windows/amd64`, `windows/arm64`, `linux/amd64`, and `linux/arm64`; the current supported host is macOS arm64.
- `NOT RUN` — optional `task package:all`; the governed matching-host `task package` gate passed and no remote aggregate is constitutionally required.
