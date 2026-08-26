# Validation Record: Windows and Linux Desktop Support — Constitution v8 Delta

**Status**: PASS — local/static and live prerelease acceptance complete
**Validated**: 2026-08-27
**Plan reviewed**: Implemented through T099

## Result

The constitution v8 implementation passes the repository's local, static, build, archive, and
non-publishing release-contract gates. The exact five-target matrix and five stable archive names,
minimal eligibility rules, quality/release workflow separation, create-only behavior, local-only
Docker boundary, pinned GoReleaser ownership, and manual recovery contract are covered by tests.

T099 used the explicitly maintainer-approved unused prerelease tag `v1.0.1-rc.1` at committed revision
`69bd8896eb4c5a776e6119f7803f278a95482910`. The real tag workflow and all five native package jobs
succeeded, pinned GoReleaser created the preserved non-draft prerelease, and an independent download
confirmed its exact five-file inventory and every archive's executable and required resources.

## Commands and evidence

All commands ran from the repository root.

| Command | Result | Evidence |
|---|---|---|
| `gofmt -l .` | PASS | Produced no paths. |
| `go vet ./...` | PASS | No diagnostics. |
| `task lint` | PASS | golangci-lint reported zero issues. |
| `go test ./...` | PASS | Full suite passed outside the restricted sandbox so localhost listener tests could bind. |
| `go test ./internal/buildtool ./cmd/build ./internal/platform` | PASS | Focused build, archive, CLI, workflow, and documentation contracts passed. |
| `task frontend:build` | PASS | Locked `npm ci` plus production Overseer and player builds passed. |
| `task ci:quality` | PASS | The composed non-publishing PR/main quality surface passed. |
| `scripts/wails-v3-cutover-check.sh` | PASS | Wails pins, tool isolation, bindings, and cutover surface passed. |
| `scripts/wails-bindings-check.sh` | PASS | Exactly 35 accepted desktop methods; deterministic bindings. |
| `task proto:check` | PASS | Format, lint, deterministic generation, isolation, generated builds, and contract tests passed. |
| `task proto:breaking` | PASS | Every maintained breaking fixture was rejected. |
| `scripts/secret-leak-check.sh` | PASS | No forbidden secret-bearing field leak was detected. |
| `scripts/tool-modules-check.sh` | PASS | Tool modules are isolated and exactly pinned; the retired ORAS module is absent. |
| `go tool -modfile=tools/goreleaser/go.mod goreleaser check --config .goreleaser.yaml` | PASS | One GoReleaser configuration validated. |
| `task build` | PASS | The default Darwin development build completed. |
| `task package GOOS=darwin GOARCH=arm64` | PASS | Created non-empty `build/dist/Fallout-Terminal-darwin-arm64.zip`. |
| `go run ./cmd/build inspect-release-archive --target darwin/arm64 --archive build/dist/Fallout-Terminal-darwin-arm64.zip` | PASS | The real local Darwin ZIP contains its executable and required resources. |

The first Darwin package attempt exposed an incorrect Linux-only `pkg-config` preflight on Darwin.
The preflight now applies only to Linux, a focused regression test covers Windows and Darwin without
`PATH`, and the required package command passed after the fix.

Benign environment warnings: the macOS linker reported objects built for newer macOS deployment
versions than the test link target, and Node reported its experimental local-storage warning during
generated-contract checks. Neither warning changed an exit status or artifact eligibility.

## Success-criteria accounting

| Criterion | Local/static status | Evidence or remaining gate |
|---|---|---|
| SC-001 | PASS | The successful tag run produced exactly five unique portable archives. |
| SC-002 | PASS | All five published archives are non-empty and passed independent executable/resource inspection. |
| SC-003 | PASS | Eligibility checks inspect archives only and invoke no native journey. |
| SC-004 | PASS | The real release contains the five collision-free OS/architecture-qualified names. |
| SC-005 | PASS (static/local) | Archive exclusion and release-inventory contracts reject secret-bearing external records; live assets will be rechecked in T099. |
| SC-006 | PASS | Optional journeys below are explicitly `NOT RUN` and are absent from all gates. |
| SC-007 | PASS | The successful Darwin package job used the common unsigned flow with zero signing, notarization, or stapling steps. |
| SC-008 | PASS | Timed distribution-guidance checklist completed in under five minutes. |
| SC-009 | PASS | `make tools`/`make help`, Task ownership, pin, and tool-module contracts pass. |
| SC-010 | PASS (static) | Failure fixtures and workflow dependency contracts withhold publication and retain target identity. |
| SC-011 | PASS | The preserved GitHub prerelease contains only the five governed archives. |
| SC-012 | PASS | Workflow contracts contain none of the prohibited strict native or rollback gates. |
| SC-013 | PASS | The published non-empty Darwin ARM64 ZIP independently passed the common archive inspector. |
| SC-014 | PASS (static) | Two paginated existing-release refusals precede creation, and tests prohibit mutation. |
| SC-015 | PASS (static) | Failure diagnostics distinguish immediate rerun from manual partial-release deletion; automation contains no deletion path. |
| SC-016 | PASS (static/local) | PR/main workflow calls only `task ci:quality`; the composed command passed and contains no release matrix or publisher. |

## Constitution-v8 boundary evidence

- **FR-033**: PASS — `task package:all` and `package-all-docker` remain local-only; neither workflow
  invokes them.
- **FR-034**: PASS — the remote aggregate/local joined-release actions, implementations, active
  documentation, standalone macOS workflow, and ORAS module are removed.
- **FR-035**: PASS — the tag workflow enters publication only through `task release:publish`, which
  invokes the repository-pinned GoReleaser module.

## Optional journeys

These checks are outside archive availability and were not required or executed during T098:

- Native Windows and Linux GUI startup: `NOT RUN`.
- Native file dialogs and lifecycle/shutdown: `NOT RUN`.
- Windows Credential Manager, Linux Secret Service, and macOS Keychain journeys: `NOT RUN`.
- Native player/session synchronization journeys: `NOT RUN`.
- Public-tunnel/external-service journey: `NOT RUN`.
- Developer ID signing, notarization, and stapling: `NOT RUN`.
- Optional local Docker aggregate: `NOT RUN`.

## Live prerelease acceptance

- Approved tag: `v1.0.1-rc.1`.
- Tagged implementation revision: `69bd8896eb4c5a776e6119f7803f278a95482910`.
- Workflow: [Wails Portable run 33023975250](https://github.com/obalunenko/fallout-terminal/actions/runs/33023975250), conclusion `success`.
- Release: [v1.0.1-rc.1](https://github.com/obalunenko/fallout-terminal/releases/tag/v1.0.1-rc.1), `isDraft=false`, `isPrerelease=true`.
- Successful package jobs: Linux AMD64 `98360942570`, Linux ARM64 `98360942593`, Windows AMD64
  `98360942609`, Windows ARM64 `98360942671`, and Darwin ARM64 `98360942751`.
- Publish job `98362589034` succeeded after downloading all five artifacts, inspecting the exact
  inventory, repeating the create-only refusal, and invoking pinned GoReleaser once.

Published inventory:

| Asset | Size (bytes) |
|---|---:|
| `Fallout-Terminal-windows-amd64.zip` | 10,961,611 |
| `Fallout-Terminal-windows-arm64.zip` | 10,272,134 |
| `Fallout-Terminal-linux-amd64.tar.gz` | 10,441,476 |
| `Fallout-Terminal-linux-arm64.tar.gz` | 9,831,686 |
| `Fallout-Terminal-darwin-arm64.zip` | 10,843,664 |

`gh release download v1.0.1-rc.1` retrieved the preserved assets into an isolated temporary
directory. `inspect-release-inventory` accepted exactly those five files, and
`inspect-release-archive` accepted each target archive independently. No checksum sidecar, raw
executable, aggregate index, DMG, external verification record, or package-registry asset exists in
the release. No recovery action was necessary; the failure-only recovery job was skipped.

Superseded constitution v6 evidence remains in [history/validation-v6.md](./history/validation-v6.md).
Historical BUG-001 through BUG-004 reports remain archived under [history/](./history/README.md) and
were not reopened.
