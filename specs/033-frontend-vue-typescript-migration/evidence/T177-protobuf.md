# T177 Protobuf Generation and Contract Evidence

Date: 2026-09-01

## Result

PASS — the complete protobuf format, lint, breaking-change, drift, generated-build, and aggregate
contract gates passed. The five generated Player TypeScript modules, protobuf schemas, generated Go
packages, RPC surface, and security boundary remained unchanged.

## Commands

| Command | Result | Evidence |
|---|---|---|
| `task proto:format:check` | PASS | Pinned Buf reported no format diff. |
| `task proto:lint` | PASS | Pinned Buf reported no lint violations. |
| `task proto:breaking` | PASS | Field-number, field-type, enum-value, package-name, and service-method fixtures were rejected. |
| `task proto:drift:check` | PASS | Exact generated TypeScript drift was rejected and all owned bytes were restored. |
| `task proto:generated:check` | PASS | Generated Go packages compiled and the Player production build completed. |
| `task proto:check` | PASS | Deterministic generation, isolation, exhaustive contracts, Go checks, and Player build completed; contract digest `c8fed61c96e24b871dbfccee0928ee149fa0691bf3869b58121dc518f68196e1`. |
| `git diff --exit-code -- proto internal/gen frontend/client/gen` | PASS | Generated and schema trees have no working-tree diff. |

## Bugfix Closure

BUG-039 corrected the canonical `proto:drift:check` caller to supply its governed generated Player
target and exact actionable diagnostic. No schema, generator, generated output, RPC, or security
contract changed.
