# Project Instructions

## Go tests

- Register cleanup for every test-owned resource immediately after acquisition with `t.Cleanup`.
- Do not use `defer` for test-lifetime resource cleanup. Use `t.Cleanup` so the resource is owned by the correct test or subtest and cleanup still runs after `Fatal` or `FailNow`.
- If cleanup accepts a context, derive it from `context.WithoutCancel(t.Context())`, because `t.Context()` is canceled before cleanup functions run. Add a bounded timeout when shutdown can block.
- Function-scope `defer` remains appropriate for lexical control flow that cannot be postponed until test cleanup, such as unlocking a mutex or balancing `WaitGroup.Done`.
