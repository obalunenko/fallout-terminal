---
name: go-code-quality-review
description: Use when creating, modifying, simplifying, or reviewing Go code. Applies the Google Go Style Guide together with repository instructions, and checks idiom, errors, concurrency, dependencies, comments, tests, and logging without assuming a particular architecture.
---

# Go Code Quality Review

## Overview

A practical guide for writing and improving readable, idiomatic Go. Apply it while creating or modifying code, during behavior-preserving simplification, and when reviewing a change.

Use the current [Google Go Style documentation](https://google.github.io/styleguide/go/) as the style baseline:

1. The [core guide](https://google.github.io/styleguide/go/guide) is canonical and takes precedence within the Google documents.
2. [Style decisions](https://google.github.io/styleguide/go/decisions) are normative, more detailed, and subordinate to the core guide.
3. [Best practices](https://google.github.io/styleguide/go/best-practices) are advisory. Apply them when they improve the code in context.

When network access is available, consult the current relevant section rather than relying on a remembered or copied rule. The guidance evolves. Do not copy the entire guide into the review or treat it as an exhaustive checklist.

Repository instructions, accepted task requirements, configured formatters and linters, and established APIs still apply. Where Google guidance intentionally leaves a choice, prefer local consistency. Do not use a style preference to justify unrelated churn.

## Core Standard

Judge Go code using the guide's priorities, in order:

1. **Clarity**: purpose and rationale are apparent to a reader.
2. **Simplicity**: the code uses the simplest mechanism that meets its behavioral and performance needs.
3. **Concision**: important details are prominent and repetition or ceremony does not obscure them.
4. **Maintainability**: assumptions, APIs, dependencies, errors, tests, and extension points support safe future changes.
5. **Consistency**: when the higher priorities do not decide the issue, match nearby and project-wide Go conventions.

Always format Go source with `gofmt`. Use `MixedCaps` or `mixedCaps` for identifiers. Go has no fixed line-length limit: when a line is hard to read, improve the surrounding names or structure instead of wrapping solely to meet a column count.

Prefer the least mechanism that solves the problem: core language constructs first, then the standard library, then an existing project dependency, and only then a new dependency or abstraction.

## Operating Modes

### Creating or modifying code

- Read the target package, repository instructions, Go version, lint configuration, and relevant tests before choosing a design.
- Make values, decisions, ownership, cancellation, and error paths easy to follow from top to bottom.
- Choose names for their use in context. Avoid stutter, redundant type words, vague utility package names, and inconsistent initialism casing.
- Introduce an interface, helper, dependency, goroutine, or configuration option only when the problem requires it or it makes the code clearer for current callers.
- Add comments that explain rationale, constraints, or surprising behavior. Prefer self-explanatory names and structure for what the code does.

### Simplifying code

- Preserve observable behavior and public contracts unless the user asks for a behavior change.
- Remove needless abstraction, repeated code, redundant state, hidden control flow, and unnecessary dependencies when the result is clearer.
- Prefer readable explicit code over clever compact code. Do not optimize for fewer lines.
- Keep simplification scoped to the changed code and directly adjacent issues. Avoid repository-wide style cleanup unless requested.

### Reviewing code

- Review the requested diff or changed files. Do not nitpick stable pre-existing code unless it directly affects the change.
- Separate correctness, safety, and maintainability findings from subjective style preferences.
- For a style finding, identify the concrete readability or maintenance cost and link the relevant Google Go Style section when useful. Do not invoke the guide as authority without explaining the effect in context.
- Treat advisory best practices as suggestions unless a repository rule or correctness concern makes them required.

## Scope

Before reviewing or simplifying, identify the changed files and line ranges (`git diff` or the task description) and keep that scope throughout. Before writing, inspect the smallest surrounding area needed to understand local contracts and conventions. If `gopls` is available, use its diagnostics; compiler, formatter, vet, and configured linter findings take priority over manual style observations.

## Step 1: Error Handling

- Errors are wrapped with context on the way up: `fmt.Errorf("doing X: %w", err)` — not returned bare across meaningful boundaries, not wrapped with `%v`
- Sentinel checks use `errors.Is` / `errors.As`, never string matching or `==` on wrapped errors; sentinels defined at package level
- **Never log an error and also return it** — either return it (caller logs) or log it and swallow it deliberately; double handling produces duplicate noise
- No `panic` outside `main()` or truly unrecoverable initialization
- In gRPC handlers (when applicable): return proper status errors (`status.Error(codes.X, ...)`), not raw internal errors

## Step 2: Logging Hygiene

In Fallout Terminal, use `github.com/obalunenko/logger` for Go application and library logging and apply the rules below. In another repository, follow that repository's logging convention instead.

- Flag direct use of `fmt.Print*`, the standard-library `log` package, or another concrete logging library outside compatibility adapters and tests.
- Thread `context.Context` through logging paths and use the package's context-aware API: `logger.Info(ctx, "message")`, `logger.WithField(ctx, "key", value).Info("message")`, or `logger.WithFields(ctx, logger.Fields{"key": value}).Info("message")`.
- Attach errors structurally with `logger.WithError(ctx, err).Error("operation failed")`; do not interpolate errors into messages or log an error that is also returned.
- Initialize the logger once at the application entry point with `logger.Init`; do not reinitialize it in packages or request paths.
- Reserve `logger.Fatal` for unrecoverable entry-point failures because it terminates the process; return errors from reusable packages.

## Step 3: Idiomatic Go

- Prefer modern language forms supported by the module's declared Go version, such as `any` instead of `interface{}`.
- Use named results only when the names improve the meaning of the signature or make a deferred operation clearer.
- Avoid stutter: package names are part of identifiers, so prefer `club.ID` to `club.ClubID`.
- Use `iota` for related incremental constants when it makes their relationship clearer, not merely because constants are adjacent.
- Avoid unnecessary abstractions. Define interfaces where they are consumed and keep them limited to the behavior the consumer needs.
- Keep functions focused, but judge readability from responsibility and control flow rather than a fixed line or statement count.
- Do not enforce a fixed source line length. Refactor an unwieldy expression when that improves clarity; do not split a line solely to satisfy a column limit.

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

- Prefer self-explanatory code over comments that add no information.
- Comments usually explain **why**: a non-obvious reason, constraint, tradeoff, or gotcha. Comments may explain what when the behavior cannot be made clear through naming and structure alone.
- Do not narrate control flow or merely restate the name or signature. Write required API documentation so it stands on its own.
- No leaking another layer's implementation details (e.g., storage mechanics in an API contract's comments)
- Comment language follows the target project's convention

## Step 8: Test Quality (when the change includes tests)

- Use table-driven structure when several cases share meaningful setup and assertions; do not force unrelated scenarios into one table. Cover relevant error paths and edge cases, not just the happy path.
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
5. Confirm each style claim matches the current relevant Google Go Style document and is not merely a personal preference
