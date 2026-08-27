# Validation: v2 Release Preparation

**Date**: 2026-08-27
**Host**: macOS arm64
**Feature**: `023-v2-release-preparation`

## T034 — Repository verification suite

| Command | Result | Evidence / notes |
|---|---|---|
| `task fmt:check` | PASS | All Go files are formatted. |
| `task vet` | PASS | Run with workspace-local `GOCACHE` because the sandbox cannot write the user cache. |
| `task lint` | PASS | `0 issues`; the lint cache emitted sandbox-only persistence warnings. The first run found and prompted fixes for two unchecked version-output writes and thirteen capitalized error strings. |
| `task test` | PASS | Full Go suite passed after granting test-only localhost binding. The sandboxed attempt failed only because `httptest` and tunnel tests could not bind loopback ports. |
| `task test:race` | PASS | Full race suite passed on the clean rerun. The first run saw the existing tunnel integration status flake (`401` expected, transient `404`); no race was reported. |
| `task proto:check` | PASS | Two clean generations were identical; schema revision `3a25a18fd5a8c4dc7bcee5dceabe88993629d7c0e24a139f63962672a9134840`; generated contracts built successfully. |
| `task proto:breaking` | PASS | All five fixtures (`field-number`, `field-type`, `enum-value`, `package-name`, `service-method`) were rejected. |
| `task bindings:check` | PASS | Deterministic Wails bindings expose exactly 35 accepted desktop methods. |
| `task frontend:build` | PASS | Client and Overseer production bundles built successfully. |
| `task browser:test` | PASS | 160 passed, 2 opt-in ngrok tests skipped. Browser fixture TLS handshake diagnostics and `NO_COLOR` notices were non-failing. |
| `task build` | PASS | Native build completed. Node emitted its known experimental local-storage notice; Go could not persist a module stat-cache temporary file in the sandbox, without affecting the build. |

### Success-criteria evidence

- **SC-001**: strict v2 stable/prerelease parser tests and package preflight tests pass.
- **SC-002**: cutover and binding gates find one `/v2` application identity and no active unsuffixed fallback.
- **SC-003**: deterministic protobuf/Wails generation and all five breaking fixtures pass.
- **SC-004**: full Go tests include version-1 session and player-configuration round trips.
- **SC-005**: formatting, vet, lint, Go, race, frontend, compatibility, binding, browser, and build checks pass.
- **SC-006**: platform guidance tests and the active-guidance audit find only v2 release examples.
- **SC-007**: target version evidence is covered by buildtool/archive tests; native package evidence is recorded below.
- **SC-008**: static/reproducibility gates find only generated metadata tokens and one workflow `VERSION` derivation.

## T035 — Native package version evidence

Native explicit-target packaging was exercised on macOS `26.5.2` (`arm64`) with
`task package GOOS=darwin GOARCH=arm64`. Each run produced the canonical
`build/dist/Fallout-Terminal-darwin-arm64.zip` archive and exited successfully.

| `VERSION` input | Packaged executable `--version` | `CFBundleShortVersionString` | `CFBundleVersion` | Release inspection |
|---|---|---|---|---|
| empty | `development` | `development` | `0.0.0` | Expected rejection against `--version 2.0.0`: `packaged executable --version mismatch: want "2.0.0\\n", got "development\\n"` (exit 1) |
| `2.0.0` | `2.0.0` | `2.0.0` | `2.0.0` | PASS: `inspect-release-archive --target darwin/arm64 --archive .../Fallout-Terminal-darwin-arm64.zip --version 2.0.0` |
| `2.0.0-rc.1` | `2.0.0-rc.1` | `2.0.0-rc.1` | `2.0.0` | PASS: `inspect-release-archive --target darwin/arm64 --archive .../Fallout-Terminal-darwin-arm64.zip --version 2.0.0-rc.1` |

The development rejection proves an untagged package cannot satisfy tagged-release
inspection. Both tagged inspections verified the executable, Darwin metadata, and
artifact manifest against the same supplied canonical version.

The ordered archive entry list was identical for all three packages (`cmp` exit 0
for development/stable and development/prerelease), with seven entries and the
same layout SHA-256,
`2bd7e466904b5602228e6ba84cdea510d3c6dc4cb48f975ffd921bbf55ff9cc6`:

```text
Fallout Terminal/Fallout Terminal.app/Contents/Info.plist
Fallout Terminal/Fallout Terminal.app/Contents/MacOS/Fallout Terminal
Fallout Terminal/Fallout Terminal.app/Contents/Resources/THIRD_PARTY_NOTICES.md
Fallout Terminal/Fallout Terminal.app/Contents/Resources/icon.icns
Fallout Terminal/Fallout Terminal.app/Contents/Resources/sessions/demo-players.json
Fallout Terminal/Fallout Terminal.app/Contents/Resources/sessions/demo.json
Fallout Terminal/artifact-manifest.json
```

Template SHA-256 values were identical before and after all three package runs:

| Immutable template | Before and after SHA-256 |
|---|---|
| `build/darwin/Info.plist.tmpl` | `e5757542cc4bcd270a0252c7c210ed472f995abc86f79e58c5ab1cd6d0fe05be` |
| `build/windows/info.json.tmpl` | `ba882c3a1961f7cc48a21de074a9c54a0ae9ded0bccdbf64cc991652d2770bca` |
| `build/windows/app.manifest.tmpl` | `8ae3f3d2c79f5531b1408bf508dee4e79d9acd906bba335876ce9c725b527139` |

The package commands emitted a sandbox-only Go module stat-cache write warning;
it did not affect any exit status or artifact verification.

## T036 — Isolated rollback evidence

The rollback source was exported from immutable revision
`3f2b6e584aee4c5279a3d54ae70aa44ee578a21a` without switching, resetting, or
otherwise modifying the current worktree. The exact isolation commands were:

```text
mktemp -d /tmp/fallout-terminal-t036.XXXXXX
# /tmp/fallout-terminal-t036.uGWbvj
mkdir /tmp/fallout-terminal-t036.uGWbvj/tree
git archive --format=tar --output=/tmp/fallout-terminal-t036.uGWbvj/rollback.tar 3f2b6e584aee4c5279a3d54ae70aa44ee578a21a
tar -xf /tmp/fallout-terminal-t036.uGWbvj/rollback.tar -C /tmp/fallout-terminal-t036.uGWbvj/tree
```

The aggregate cutover check contains a clean-index assertion for completed
historical records. To exercise that assertion without attaching the archive to
the live repository, the archived files were committed once to a disposable Git
repository inside `tree/` with `git init -q`, `git add .`, and
`git -c user.name=T036 -c user.email=t036@example.invalid commit -q -m rollback-snapshot`.

### Single-identity evidence

The following commands were run once with the archived `tree/` as the working
directory and once at the current repository root:

```text
awk '$1 == "module" { count++; print } END { print "root-module-count=" count; exit count == 1 ? 0 : 1 }' go.mod
find frontend/overseer/bindings/github.com/obalunenko -type f -name desktopservice.js -print
rg -n 'bindings/github.com/obalunenko/Fallout-Terminal' frontend/overseer/src tests/browser -g '*.js' -g '*.mjs'
```

| Snapshot | Root application identity | Wails application namespace | Result |
|---|---|---|---|
| Immutable rollback revision | One declaration: `module github.com/obalunenko/Fallout-Terminal` | One service: `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js`; all active Overseer/browser imports use that unsuffixed path | PASS |
| Current forward tree | One declaration: `module github.com/obalunenko/Fallout-Terminal/v2` | One service: `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js`; all active Overseer/browser imports use the `/v2` path | PASS |

Neither direction contained a second `desktopservice.js` application namespace,
and both module scans reported `root-module-count=1`.

### Offline static and generation checks

All Go-backed commands below set `GOPROXY=off`, `GOSUMDB=off`, and
`GOCACHE=/private/tmp/fallout-terminal-codex-gocache`. The rollback protobuf
check additionally set `npm_config_offline=true`; it installed only packages
already present in the local npm cache.

| Snapshot | Exact command | Result / evidence |
|---|---|---|
| Rollback and forward | `scripts/wails-v3-contract-check.sh` | PASS: Wails contracts remained qualified, pinned, tool-isolated, and schema-stable. |
| Rollback and forward | `scripts/tool-modules-check.sh` | PASS: all development tools resolved through their exactly pinned owning modules without changing the root module. |
| Rollback and forward | `scripts/wails-bindings-check.sh` | PASS: two clean generations were identical and exposed exactly one 35-method desktop service in the snapshot's matching namespace. |
| Rollback and forward | `scripts/wails-v3-cutover-check.sh` | PASS: the aggregate static, tool, generation, historical-record, and resolved-module checks accepted each single-namespace tree. |
| Rollback | `task proto:check` | PASS: two clean Go/Connect/ES generations matched revision `5e16608a2e30b784305702f8cc91418c6d2b51ced0a1fb40aa6c8199e2eb32dd`; generated code and the client compiled, and `git status --short` remained empty. |

This drill was deliberately source/static/generation-only. It made no network
request and did not run a second native package, GUI, signing, or publication
operation from the historical snapshot. Current native package evidence is
recorded in T035, and the full current verification matrix is recorded in T034.

### Publication boundary

The active operator guidance at `docs/platform-packaging.md:115` requires a
maintainer to preserve an existing complete release and use a **new strict v2
forward-fix tag**. The reviewed release contract at
`contracts/release-identity.md:170` likewise permits the immutable rollback only
before publication and requires a new strict v2 patch or prerelease tag after
publication. Therefore this rollback procedure cannot delete, replace, append
to, or reuse a complete published release identity.
