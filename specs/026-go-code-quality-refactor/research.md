# Research: Go Code Quality Refactor

## Canonical deep-copy ownership

**Decision**: Export the existing complete content-node copy operation from the domain package and reuse it at service boundaries that project domain content.

**Rationale**: The domain implementation already handles state-change configuration, terminal-transition configuration, raw extension data, children, and nil slice semantics. The live implementation omits terminal-transition cloning, while session and control maintain near-duplicates. One canonical copy operation prevents new fields from being missed independently.

**Alternatives considered**: Fix only the live helper, which leaves the duplication defect pattern intact; use serialization as a copier, which is slower and can change nil/empty or unknown-field semantics.

## Stable control errors

**Decision**: Define exported sentinel identities for stale command resolution and durable-state persistence failures, wrap them at the control boundary, and use `errors.Is` in application presentation.

**Rationale**: Human-readable messages remain useful context, but they are not a stable control-flow contract. Sentinel identities work through wrapping and require no new type hierarchy.

**Alternatives considered**: Keep substring matching, which breaks when wording changes; add numeric error codes to transport contracts, which is outside this behavior-preserving scope.

## Iterative tunnel coordination

**Decision**: Replace recursive calls after awaited lifecycle ownership with loops inside `Stop` and `Reconfigure`; preserve the owned mutation buffer for the full reconfiguration call.

**Rationale**: Each retry is a state-machine iteration, and representing it as a loop bounds stack use while preserving the same lock-release, wait, cancellation, revision, and ownership checks.

**Alternatives considered**: Spawn a coordinator goroutine, which adds ownership and shutdown complexity; cap recursive retries, which would create a new externally visible failure mode.

## Player-action decomposition

**Decision**: Extract transaction-local validation, terminal-transition preparation, and accepted-result construction helpers while keeping the single `commit` call as the atomic boundary.

**Rationale**: The dispatch method currently mixes request identity, authorization, route handling, command resolution, persistence-facing state, and effect construction. Small pure or transaction-local helpers improve reviewability without adding interfaces or locks.

**Alternatives considered**: Split action handling into new service objects, which would obscure transaction ownership; leave the method intact and add comments, which does not reduce branching or duplication.

## Shared character rules

**Decision**: Add domain helpers for character display-name and intelligence value validation and call them from both the application and control boundaries, while retaining pointer-presence and authorization checks at their original boundaries.

**Rationale**: The domain already owns the limits. Sharing value validation prevents drift without weakening defense in depth.

**Alternatives considered**: Validate only in control, which would remove early application feedback; keep duplicated checks, which already differ in form and literal use.

## Session command orchestration

**Decision**: Use one application helper parameterized by operation name and session-service command for create, open, and demo-copy flows.

**Rationale**: These commands share locking, service availability, status capture, ordering reset, player-config reset, routing, and operation recording. The helper can preserve the exact command-specific operation name and callback.

**Alternatives considered**: Keep three facade methods, which repeats every orchestration step; merge session-service methods, which would erase distinct user intents and inputs.

## Manifest validation stages

**Decision**: Separate manifest loading, identity validation, package-shape validation, and per-file evidence validation into focused helpers within the update package.

**Rationale**: The current function performs several independent security checks in one block. Stage helpers make rejection coverage auditable while keeping the same ordered fail-closed behavior and error categories.

**Alternatives considered**: Introduce a generic validation framework, which adds abstraction without reuse; reorder checks for compactness, which risks changing diagnostics and work performed on malformed input.

## Source organization

**Decision**: Prefer logical extraction within existing files during this refactor and add a same-package file only for a cohesive exported error contract.

**Rationale**: The requested outcome is reviewable responsibility boundaries, not lower line counts. Limiting moves keeps the behavior changes and their tests easy to review.

**Alternatives considered**: Split each large source file by line count, which creates churn without simplifying dependencies; change package boundaries, which conflicts with the constitution and scope.
