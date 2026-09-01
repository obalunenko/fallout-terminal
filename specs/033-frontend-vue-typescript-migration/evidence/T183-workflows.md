# T183 Taskfile, CI, and Buildtool Contract Evidence

Date: 2026-09-01

## Result

PASS — Taskfile, CI workflows, and the buildtool agree on the exact Node runtime, ten canonical
frontend targets, one workspace lock/install/cache owner, ordered protobuf/Player/bindings/Overseer
preparation, and all five governed package plans.

| Command | Result | Evidence |
|---|---|---|
| `scripts/frontend-task-contract-check.sh --self-test --expected-target-count 10` | PASS | Node 26.8.1 accepted; adjacent versions rejected; exact ten-target inventory, application isolation, install ownership, order, and failure propagation verified. |
| macOS-qualified `go test ./internal/buildtool` | PASS | Buildtool producer/consumer, preflight, workflow, resource/signature ordering, and five package-plan tests passed. |

No direct competing npm install/build workflow was introduced.
