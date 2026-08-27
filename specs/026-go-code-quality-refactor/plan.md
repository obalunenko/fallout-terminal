# Implementation Plan: Go Code Quality Refactor

**Bugfix**: 2026-08-27 — BUG-001 updated repository-wide macOS verification to use the governed deployment environment.

## Summary

Refactor the highest-risk Go maintenance seams while preserving all runtime, persistence, and transport behavior. The implementation will make domain deep-copy behavior reusable, replace message parsing with wrapped error identities, convert lifecycle re-entry to iterative coordination, and extract cohesive helpers for player actions, character rules, session commands, and update-manifest validation. Existing package ownership and public contracts remain unchanged, with focused regression tests preceding each risky edit.

## Project Structure

```text
app.go                                  # application command orchestration and error presentation
app_test.go                             # application-boundary regressions
internal/control/
├── service.go                          # atomic control transactions and terminal activation
├── service_test.go                     # action, error, and character-rule regressions
└── errors.go                           # stable control failure identities
internal/domain/
├── model.go                            # canonical detached content-node copy
├── validate.go                         # canonical character value rules
├── model_test.go
└── validate_test.go
internal/live/
├── service.go                          # public live projections
└── service_test.go
internal/session/
├── service.go                          # session projections
└── service_test.go
internal/tunnel/
├── manager.go                          # iterative lifecycle coordination
└── manager_test.go
internal/update/
├── staging.go                          # staged manifest validation phases
└── staging_test.go
```

**Structure Decision**: Keep every concern in its current package and introduce only one small control error file; move code between same-package files only when the move creates a clear feature grouping and does not obscure review history.

## Constitution Check

| Principle | Assessment | Evidence |
|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | Domain, control, live, session, tunnel, and update ownership remains unchanged; no framework dependency enters an internal service. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | No structured application contract, schema, generated type, or boundary DTO changes. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | Player mutations remain inside the authoritative control commit; projections become more completely detached. |
| IV. Separate Public and Private Capabilities | PASS | Error classification and internal helpers do not expose privileged state or alter public descriptors. |
| V. Evolve Schemas Safely and Reproducibly | PASS | No schema evolution or generated-file edit is planned. |
| VI. Preserve Portable Session JSON Version 1 | PASS | Session cloning and command orchestration preserve the existing version-1 model and adapters. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | No protocol or runtime coexistence is introduced; duplicated local helpers are removed when their canonical replacement lands. |

Post-design review: the design artifacts retain the same boundaries and introduce no constitutional exception.

## Verification Strategy

- Add focused regressions in the existing package test files before changing detached projection and error identity behavior.
- Exercise tunnel wait paths with deterministic fakes and race-enabled tests; retain cancellation and shutdown time bounds.
- Reuse existing player-action, session, character, and manifest fixtures rather than creating parallel test harnesses.
- Register every newly acquired test resource immediately with `t.Cleanup`; use `context.WithoutCancel(t.Context())` plus a bounded timeout for potentially blocking cleanup.
- ~~Run `task fmt:check`, `go vet ./...`, `task lint`, `go test ./...`, and `go test -race ./...` after the full change.~~ Superseded by BUG-001 because raw repository-wide Go commands bypass the governed Darwin deployment environment.
- Run `task fmt:check`, `task vet`, `task lint`, `task test`, and `task test:race` after the full change. On macOS, verify fresh-cache unit and race runs emit no linker target-version warnings.
