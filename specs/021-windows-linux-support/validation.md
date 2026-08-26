# Validation Record: Windows and Linux Desktop Support

**Recorded**: 2026-08-26
**Working branch**: `021-windows-linux-support`
**Base revision inspected**: `083aec8fa94a028d6c06797dbae0a3b765558201`

## Execution boundary

The feature is present only in the working tree: the root `Taskfile.yml` and `.github/workflows/wails-portable.yml` are not in the recorded Git revision, and the working tree is not clean. Dispatching either delivery workflow at the base revision would therefore validate different code. GitHub CLI authentication was also unavailable: `gh auth status` reported that the active token is invalid and requires `gh auth login`.

No commit, push, or workflow dispatch was performed because those external state changes were not requested. No local test suite was run, as explicitly requested. In particular, `task check`, `task test`, `task test:race`, browser tests, native package builds, and native smoke tests were not run locally.

## Static validation completed

| Check | Result | Evidence |
|---|---|---|
| Tool ownership and exact pins | PASS | `scripts/tool-modules-check.sh` accepted every isolated tool module, Wails `v3.0.0-beta.13`, Task `v3.53.1`, the tools-only bootstrap, non-mutating Make help, and qualified invocations. |
| Wails v3 contract | PASS | `scripts/wails-v3-contract-check.sh` accepted Task orchestration, exact Wails pins, the reviewed schema, and the absence of floating/global Wails commands. |
| Wails cutover | PASS | `scripts/wails-v3-cutover-check.sh` accepted deterministic 35-method bindings and found no active v2, dual-runtime, floating-tool, generated-global, dependency, bundle, script, CI, or operating-document surface. |
| Task command graph | PASS | The pinned Task binary parsed `Taskfile.yml` and listed the migrated developer, verification, build, package, aggregate, release, and Spec Kit tasks. |
| Local aggregate contract | READY FOR CI | Controlled Go coverage now exercises all four `bin/<os>-<arch>/` payloads against their verified archives, rejects inventory/hash drift and malformed partial exports, preserves atomic replacement/rollback, and checks missing/stopped/unsupported Docker diagnostics for both cause and recovery. The suite has not been run locally; T055 owns its clean-revision CI execution. |
| Configuration syntax | PASS | Bash syntax passed for the changed shell scripts; Ruby YAML parsing passed for `Taskfile.yml`, `wails-macos.yml`, and `wails-portable.yml`; Python AST parsing passed for the changed update checker. |
| Go/editor diagnostics | PASS | Scoped `gopls check` passed for changed build, platform, credential, lifecycle, and related test files; `gofmt -l .` returned no files. |
| Patch hygiene | PASS | `git diff --check` reported no whitespace errors. |
| Windows PowerShell parser | NOT RUN | A PowerShell parser is not available on this macOS host; the script is exercised only by the matching Windows workflow. |

## Success Criteria

| Criterion | Result | Evidence or reason |
|---|---|---|
| SC-001 — exactly four portable archives | NOT RUN | Requires the clean pushed portable workflow and authenticated aggregate dispatch. Static workflow inspection defines exactly four independent targets and one fail-closed aggregate join. |
| SC-002 — all four artifacts launch within 60 seconds | NOT RUN | Requires matching Windows amd64/arm64 and Linux amd64/arm64 runners. The workflow contains native launch gates, but no eligible revision was dispatched. |
| SC-003 — representative Windows and Linux host/player journey | NOT RUN | Requires matching native hosts and the clean revision. No local or remote test suite was run. |
| SC-004 — artifact identity and resource inventory | NOT RUN | PE/ELF, manifest, archive, checksum, and inventory verifiers are wired into every native workflow job, but no feature artifact exists yet. |
| SC-005 — secrets absent from files, logs, and public state | NOT RUN | Static secret-leak contracts and native secure-store adapters are present; complete target evidence requires native runs. |
| SC-006 — secure-store outage fails closed | NOT RUN | Windows Credential Manager and Linux Secret Service errors map to fail-closed public-access status while local/LAN remains available; matching-host outage evidence was not produced. |
| SC-007 — existing macOS distribution remains valid | NOT RUN | The changed macOS workflow and package path require a clean pushed revision. No local macOS package or test gate was run. |
| SC-008 — published guidance supports a five-minute selection path | PASS | `checklists/distribution-guidance.md` records the completed target-selection, prerequisites, launch, data-location, credential, and troubleshooting review for all four targets. |
| SC-009 — one bootstrap exposes all migrated Task workflows | PARTIAL | Static inspection passed: Make exposes only `tools` plus non-mutating `help`, every tool module is discovered, and the pinned Task binary lists every migrated workflow. The mutating `make tools` installation and the workflows themselves were not executed locally. |
| SC-010 — local payload matrix and actionable host/Docker failure | NOT RUN | T062 adds controlled tests for all four exact payloads, archive/hash agreement, malformed-output quarantine, atomic publication/rollback, and actionable missing/stopped/unsupported Docker diagnostics. T055 must replace this entry only after those tests pass on the clean pushed revision; no local test/build run was performed. |
| SC-011 — repeatable owned-output replacement and rollback | NOT RUN | BUG-002 defines replacement recognition, same-filesystem sibling work-root backup, rollback, and unsafe-target rejection. T063 implementation is inspected statically; controlled checks are not run locally. |
| SC-012 — native Darwin bundle joins local aggregate | NOT RUN | BUG-003 defines `darwin/arm64` host validation, canonical package-plan reuse, bundle staging, and five-target publication. T064 implementation is inspected statically; no local package build or test was run. |
| SC-013 — tagged five-target GitHub Release publication | NOT RUN | T060 implements the one-SHA unsigned Darwin DMG with exact SHA-256 verification plus four-target portable join, exact 11-file inventory, GoReleaser-only GitHub Release publication, prerelease handling, and a non-publishing manual workflow dispatch without signing/notarization secrets. T055 must replace this entry only with clean-revision joined-workflow evidence; no tag or production publication was executed. |

## Native CI follow-up

After the changes are reviewed, complete T056–T058, T060, and T062 in CI, commit them and push the current branch to `origin`, authenticate with `gh auth login`, then run `task package:all:remote OUTPUT=<new-directory>`. Preserve the correlated portable workflow URL and its `fallout-terminal-portable` artifact. Run the macOS workflow at the same revision and exercise the five-target tag join in non-publishing validation before completing reopened T055. Replace the `NOT RUN` entries above only with evidence from those matching-host jobs. Any failed or missing target must remain ineligible rather than being reported as a partial success. A local Docker run may additionally satisfy SC-010 static-output evidence, but it never replaces matching-host launch evidence.
