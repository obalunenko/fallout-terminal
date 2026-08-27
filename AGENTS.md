# Project Instructions

## Go style and quality

- Apply [Google's Go Style Guide](https://google.github.io/styleguide/go/) when creating, modifying, simplifying, or reviewing Go code. Use the [core guide](https://google.github.io/styleguide/go/guide) as the canonical baseline, [style decisions](https://google.github.io/styleguide/go/decisions) as subordinate normative guidance, and [best practices](https://google.github.io/styleguide/go/best-practices) as advisory guidance.
- Prioritize the guide's principles in order: clarity, simplicity, concision, maintainability, and consistency. Prefer standard language constructs and the standard library when they are sufficient.
- Follow more specific repository instructions, configured formatters and linters, and established package conventions where the Google guidance leaves a choice. Do not use style guidance to justify unrelated churn in stable code.
- Use `.agents/skills/go-code-quality-review/SKILL.md` for Go code creation, modification, simplification, and review.

## Go tests

- Register cleanup for every test-owned resource immediately after acquisition with `t.Cleanup`.
- Do not use `defer` for test-lifetime resource cleanup. Use `t.Cleanup` so the resource is owned by the correct test or subtest and cleanup still runs after `Fatal` or `FailNow`.
- If cleanup accepts a context, derive it from `context.WithoutCancel(t.Context())`, because `t.Context()` is canceled before cleanup functions run. Add a bounded timeout when shutdown can block.
- Function-scope `defer` remains appropriate for lexical control flow that cannot be postponed until test cleanup, such as unlocking a mutex or balancing `WaitGroup.Done`.

## Go validation

- On macOS, use `task vet`, `task test`, and `task test:race` for repository-wide validation. These targets apply the project's macOS 13 deployment target and matching CGO flags, preventing linker target-version warnings.
- An equivalent direct Go command on macOS must set `MACOSX_DEPLOYMENT_TARGET=13.0`, `CGO_CFLAGS=-mmacosx-version-min=13.0`, and `CGO_LDFLAGS=-mmacosx-version-min=13.0`.

## Node.js validation

- Use the Node.js version selected by `.nvmrc`; NVM users should run `nvm use` from the repository root before JavaScript work.
- Use Taskfile workflows for dependency installation, frontend builds, and browser tests. Their `node:check` preflight rejects runtimes older than Node.js 26.8.1 before npm runs.
