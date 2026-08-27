---
name: go-code-quality-review
description: Use when reviewing written or modified Go code in any repository — checking idiomatic style, error handling, concurrency safety, common Go pitfalls, dependency hygiene, comment quality, unit-test quality, and consistent logging with github.com/obalunenko/logger. Makes no assumptions about a specific architecture or directory layout.
---

# Go Code Quality Review

## Overview

A structured review checklist for Go code quality that applies to any Go project. It prescribes `github.com/obalunenko/logger` for Go logging but otherwise excludes project-specific rules (architecture layers, error libraries, directory layout) — check those separately against the target project's own instruction files (AGENTS.md, CLAUDE.md, or similar) and lint config when they exist.

## Scope

Review **recently written or modified code only**. Do not nitpick stable pre-existing code unless it directly interacts with the change.

Before starting: identify the changed files and line ranges (`git diff` or the task description) and keep that scope throughout. If a Go LSP (gopls) is available, pull diagnostics first — compiler and vet findings beat manual spotting.

## Step 1: Error Handling

- Errors are wrapped with context on the way up: `fmt.Errorf("doing X: %w", err)` — not returned bare across meaningful boundaries, not wrapped with `%v`
- Sentinel checks use `errors.Is` / `errors.As`, never string matching or `==` on wrapped errors; sentinels defined at package level
- **Never log an error and also return it** — either return it (caller logs) or log it and swallow it deliberately; double handling produces duplicate noise
- No `panic` outside `main()` or truly unrecoverable initialization
- In gRPC handlers (when applicable): return proper status errors (`status.Error(codes.X, ...)`), not raw internal errors

## Step 2: Logging Hygiene

- Use `github.com/obalunenko/logger` for Go application and library logging. Flag direct use of `fmt.Print*`, the standard-library `log` package, or another concrete logging library outside compatibility adapters and tests.
- Thread `context.Context` through logging paths and use the package's context-aware API: `logger.Info(ctx, "message")`, `logger.WithField(ctx, "key", value).Info("message")`, or `logger.WithFields(ctx, logger.Fields{"key": value}).Info("message")`.
- Attach errors structurally with `logger.WithError(ctx, err).Error("operation failed")`; do not interpolate errors into messages or log an error that is also returned.
- Initialize the logger once at the application entry point with `logger.Init`; do not reinitialize it in packages or request paths.
- Reserve `logger.Fatal` for unrecoverable entry-point failures because it terminates the process; return errors from reusable packages.

## Step 3: Idiomatic Go

- `any` instead of `interface{}`
- Named return values only when they meaningfully improve clarity (e.g., with deferred closes)
- No stuttering: `club.ClubID` → `club.ID`; package name is part of the identifier
- Related constant sequences grouped with `iota`
- No unnecessary abstractions — no helpers or interfaces created for a single use site
- Interfaces defined at the **consumer** side, kept small (1–3 methods preferred)
- Functions stay focused: flag anything past ~100 lines / ~50 statements as a split candidate; lines past ~140 chars as a readability concern (defer to the project's lint config when it sets different limits)

## Step 4: Concurrency Safety

For any code touching goroutines, channels, or shared state:

- Every goroutine has a defined exit condition — no leaks
- `sync.WaitGroup.Add()` is called **before** launching the goroutine, never inside it
- `context` is threaded through goroutine trees and cancellation is respected
- No unsynchronized reads/writes to shared maps, slices, or structs across goroutines — that is a data race even if it "works"
- `sync.Once` for one-time initialization
- No `time.Sleep` polling in production code — tickers or context-aware waits
- Concurrent work needing error collection uses `errgroup` (`golang.org/x/sync/errgroup`)

## Step 5: Common Go Pitfalls

- **Loop variable capture** in goroutines/closures (pre-Go 1.22 the loop var is shared across iterations)
- **Typed nil in interface**: returning a typed nil pointer as an interface value yields a non-nil interface — inspect interface-returning paths
- **Slice aliasing**: `append` may reuse a shared backing array
- **`defer` in a loop** accumulates until the function returns, not per iteration
- **Copying non-copyable types**: `sync.Mutex`, `sync.WaitGroup`, etc. must not be copied — watch value receivers and by-value struct passing
- **Missing `defer cancel()`** after `context.WithCancel` / `context.WithTimeout` (discarding cancel with `_` is the same bug)
- **Context not threaded** through calls that do I/O or block
- **Struct comparison with `==`** when the struct contains slices or maps

## Step 6: Dependency Hygiene

Flag imports of deprecated packages in favor of maintained replacements:

| Deprecated | Use instead |
|---|---|
| `github.com/golang/protobuf` | `google.golang.org/protobuf` |
| `github.com/satori/go.uuid` | `github.com/google/uuid` (or the project's uuid wrapper) |
| `math/rand` (non-test code) | `math/rand/v2` |
| `github.com/golang/mock` | `go.uber.org/mock` |

## Step 7: Comment Quality

- Comments only where truly needed — self-explanatory code gets none (the default)
- Comments explain **WHY** (non-obvious reason, constraint, gotcha), never **HOW** (step-by-step narration of the body)
- No restating the name/signature in prose; no doc comment retelling the function's control flow
- No leaking another layer's implementation details (e.g., storage mechanics in an API contract's comments)
- Comment language follows the target project's convention

## Step 8: Test Quality (when the change includes tests)

- Table-driven structure; error paths and edge cases covered, not just the happy path
- Deterministic: no un-injected `time.Now()`, no real network/DB/filesystem I/O in unit tests
- Register cleanup for every test-owned resource immediately after acquisition with `t.Cleanup`
- Do not use `defer` for test-lifetime resource cleanup; use `t.Cleanup` so cleanup is owned by the correct test or subtest and still runs after `Fatal`/`FailNow`
- When cleanup needs a context, derive it from `context.WithoutCancel(t.Context())` because the test context is canceled before cleanup functions run; add a bounded timeout when shutdown can block
- Keep function-scope `defer` only for lexical control flow that cannot be moved to test cleanup, such as unlocking a mutex or balancing `WaitGroup.Done`
- Generated mocks are regenerated via `go generate`, never hand-edited
- `t.Helper()` in helpers; `t.Fatalf` when continuing would panic; never `t.Fatal` inside goroutines
- Proto messages compared with `cmp.Diff` + `protocmp.Transform()`, not `reflect.DeepEqual`

## Output Format

**Summary**: 2–3 sentences on overall quality and primary concerns.

**Issues** grouped by severity, each with file:line, the problem, and a corrected snippet where helpful:

- 🔴 **Critical** — bugs, data races, goroutine leaks, security issues; must fix before merge
- 🟠 **Major** — error-handling gaps, deprecated dependencies, concurrency smells; should fix
- 🟡 **Minor** — style, naming, idiom violations; consider fixing
- 🔵 **Suggestion** — clarity, testability, performance; optional

**Positives**: briefly note what was done well.

## Self-Verification

Before posting the review:

1. Confirm every raised issue is a real problem (no false positives — compile the claim in your head)
2. Confirm nothing flagged is untouched pre-existing code
3. Confirm suggested fixes are valid Go
4. Re-scan the changed code once more for concurrency and error-handling misses — these are the two categories reviews miss most
