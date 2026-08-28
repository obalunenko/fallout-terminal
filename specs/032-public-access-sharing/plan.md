# Implementation Plan: Public Access Credential Sharing

## Summary

Replace inline player-login editing in public-access settings with a read-only credential summary and a focused child dialog. Add a native one-shot share command that reads the saved player password through a scoped secure-store callback, writes a two-line login/password block directly to the system clipboard, clears temporary buffers, and returns only success or a safe error. Update the exact desktop allowlist, deterministic generated bindings, presentation logic, and security-focused Go and browser coverage.

## Project Structure

```text
app.go                                           # native share command and clipboard seam
app_test.go                                      # native success/failure and secret-lifetime tests
main.go                                          # production clipboard dependency wiring
desktop_service.go                               # private Wails allowlist forwarding method
app_contract_test.go                             # exact desktop method inventory
internal/tunnel/secret.go                        # scoped player-password callback helper
internal/tunnel/secret_test.go                   # validation, clearing, and redaction tests
frontend/overseer/src/
├── desktop-api.js                               # secret-free share command facade
├── index.html                                   # summary, share action, credential dialog
├── overseer.js                                  # rendering, dialog lifecycle, mutations
└── overseer.css                                 # summary and child-dialog layout
frontend/overseer/bindings/.../desktopservice.js # regenerated Wails binding
internal/platform/assets_test.go                  # embedded markup/controller contract
tests/browser/
├── fixtures/desktop-bindings.js                 # private method fixture and clipboard outcome
└── public-access-settings.spec.mjs              # credential-management/share journeys
scripts/wails-bindings-check.sh                   # exact 39-method allowlist
```

**Structure Decision**: Keep secure password access in `internal/tunnel`, compose the OS clipboard at the root, expose only a no-secret `CommandResult` through the existing private desktop service, and keep all visible behavior in the established Overseer public-access module.

## Constitution Check

No `.specify/memory/constitution.md` is present. The applicable repository rules are checked below.

| Principle | Assessment |
|---|---|
| Go clarity, simplicity, and local consistency | PASS — the new interface is consumed by `App`, and the existing scoped-secret pattern is extended without a general secret getter. |
| Test resource ownership through `t.Cleanup` | PASS — no new long-lived test resource is planned; any acquired resource will register cleanup immediately. |
| Go modernization before final validation | PASS — run `go fix ./...`, review its diff, then format and validate. |
| macOS validation through Taskfile targets | PASS — use `task vet`, `task test`, and `task test:race` for repository-wide Go validation. |
| Node runtime and Taskfile workflows | PASS — use the `.nvmrc` runtime and Taskfile frontend/browser targets. |
| Secret-free reusable contracts | PASS — the browser receives only `{ok,error}`; the clipboard side effect remains native and one-shot. |

Post-design re-check: PASS. The UI contract adds identifiers only for controls and never introduces a serializable credential response.

## Phase 0: Research

See [research.md](./research.md) for the decisions behind the native clipboard boundary, scoped player-password helper, credential-pair dialog, generation behavior, and unchanged protobuf surface.

## Phase 1: Design

- [data-model.md](./data-model.md) defines credential summary, dialog draft, and one-shot share state.
- [contracts/desktop-ui.md](./contracts/desktop-ui.md) defines the private desktop command and accessible UI identifiers/actions.
