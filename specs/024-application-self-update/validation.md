# Application Self-Update Validation

**Validation date**: 2026-08-27
**Branch**: `024-application-self-update`
**Source revision**: `e739c034673f93629c5a6873c643a941a7883f29`

## Automated quality gates

| Gate | Result | Evidence |
|---|---|---|
| `task check` | PASS | Formatting, vet, pinned lint, full Go race suite, protobuf format/lint/generation/drift/breaking checks, deterministic Wails contracts/bindings, Spec Kit tests, and repository checks passed. |
| `task ci:quality` | PASS | Go tests/vet, clean frontend installs and builds, startup checks, Wails contracts, and protobuf quality gates passed. The command required approved loopback access for test HTTP servers. |
| `npm test` in `tests/browser` | PASS | 175 tests passed and 2 external authenticated ngrok journeys were skipped as designed. |
| `gopls check` | PASS | No Go diagnostics were reported. |
| `scripts/secret-leak-check.sh --self-test` and `scripts/secret-leak-check.sh` | PASS | The scanner detected its self-test fixture and reported no leak in the repository scan. |
| `git diff --check` | PASS | No whitespace errors were reported. |

The repository-level `task browser:test` wrapper was stopped while redundantly installing
Chromium. Chromium was already installed, so the underlying locked Playwright suite was run
directly with `npm test` and produced the result above.

## Success Criteria evidence

| Criterion | Result | Evidence and limits |
|---|---|---|
| SC-001 | PASS | Deterministic application lifecycle and browser tests verify one eligible startup offer and continued local-control availability. |
| SC-002 | PASS | Version/channel/provider tests cover current, newer-installed, development, and incompatible or absent artifacts with no offer. |
| SC-003 | PASS | Provider, manager, application, and browser failure tests cover offline, timeout, rate-limit, malformed-release, and unavailable-service behavior without blocking startup. |
| SC-004 | PASS (automated) | Build, provider, and staging tests exercise the exact five-target inventory and exact target selection/rejection. The reduced live-acceptance scope requires a native journey only on Darwin ARM64. |
| SC-005 | PASS | Digest, completeness, ambiguity, compatibility, extraction, and staging tests reject unsafe artifacts before replacement. |
| SC-006 | PASS (manual, screenshot evidence) | A Darwin ARM64 run shows `2.1.0-autoupdate01` offering `2.1.0-autoupdate02`, the accepted update reaching ready-to-restart, and the relaunched application reporting `2.1.0-autoupdate02` with no repeated offer visible. |
| SC-007 | PASS (automated) | Helper and recovery tests inject promotion and relaunch failures and verify retention or restoration. Additional manual physical failure injection is outside the reduced live-acceptance scope. |
| SC-008 | PASS (automated) | Recovery tests verify representative user-data isolation across success and failure paths. An additional manual cross-version credentials/preferences comparison is outside the reduced live-acceptance scope. |
| SC-009 | PASS (contract) | Release/build checks enforce exactly one archive for each of the five targets and require valid GitHub SHA-256 digest metadata, with no checksum sidecars or extra assets. Publication from the schema-v2 self-update implementation was **NOT RUN**. |
| SC-010 | PASS | The deterministic failure matrix verifies stable stage/action diagnostics, and the secret scanner reports no sensitive-value leak. |

## Current-host package evidence

Host: macOS 26.5.2 (build 25F84), Darwin/arm64.

`task package GOOS=darwin GOARCH=arm64` passed after rerunning with approved access to the Go
build cache. The initial sandboxed attempt reached native compilation and failed only because the
Go cache under the user Library was not writable from the sandbox.

The successful command produced:

- `build/dist/Fallout-Terminal-darwin-arm64.zip` (about 10 MiB, 7 archive entries)
- `build/dist/Fallout-Terminal-darwin-arm64.zip.sha256`
- archive SHA-256: `3d606efae92765a47a74b9ae61b5e23fa1e1afc94f9a672356bd28244a9ddb72`
- manifest schema: `2`
- manifest version: `development`
- manifest target: `darwin/arm64`
- manifest source revision: `e739c034673f93629c5a6873c643a941a7883f29`

This is unsigned development-package evidence. It does not substitute for tagged-release update
acceptance.

## Minimum live tagged prerelease journey

The user completed the Darwin ARM64 acceptance journey on 2026-08-27. The captured sequence shows:

1. [`sc-006-update-offer.png`](./evidence/sc-006-update-offer.png): the installed
   `v2.1.0-autoupdate01` application offers `v2.1.0-autoupdate02` before downloading; the UI renders
   both versions without the conventional `v` tag prefix.
2. [`sc-006-update-ready.png`](./evidence/sc-006-update-ready.png): the accepted update finishes
   preparation and requests the separate restart decision.
3. [`sc-006-updated-version.png`](./evidence/sc-006-updated-version.png): after restart, the
   application reports version `2.1.0-autoupdate02`; the ready local interface has no repeated update
   offer visible.

Local Git refs confirm the exact tags `v2.1.0-autoupdate01` and `v2.1.0-autoupdate02`; both point to
source revision `e58884465b8df190968493a0c50323353db0c641`, which also appears in the captured release
notes. A text runtime log was not retained, but the screenshots and tag refs preserve the release
identity, ready-to-restart boundary, and post-relaunch version required for this reduced live
acceptance. Windows AMD64, Windows ARM64, Linux AMD64, and Linux ARM64 runtime journeys remain
outside its scope; deterministic release/provider, verification, staging, helper, recovery, UI, and
browser suites continue to cover those paths.

### Screenshot checksums

| Evidence | SHA-256 |
|---|---|
| `sc-006-update-offer.png` | `69c1c93c215466a259cd7c4eeb2ced9060014c6561d053fbf3b88abd24fb9813` |
| `sc-006-update-ready.png` | `c9ca24d61b921a654fc48c924be9d740cf81a937c75f247248e353312f1116ba` |
| `sc-006-updated-version.png` | `732b8ca73f0c7a104428abb2ab55c20dbd1d7bb489c4b2b5e1e7f1d80e8bf112` |

## BUG-001 forward-fix evidence

Validation date: 2026-08-29.

The published v2.1.0 Darwin ARM64 archive has SHA-256
`bd2769ee733f522627be30698d95e941e042aa55b83ef0f3af02266b839f980b` and contains eight ZIP
entries. Its schema-v2 manifest governs seven payload files, including `RUNNING.md`; the published
v2.0.0 updater accepts exactly the six original Darwin payload files and therefore rejects v2.1.0
before adjacent staging.

BUG-001 restores the schema-v2 package inventory compiled into v2.0.0 and adds fixed predecessor
contract assertions in both the build producer and staging consumer. The following gates passed:

- focused `go test ./internal/buildtool ./internal/update`;
- `go fix ./...`, followed by `gofmt` and `git diff --check`;
- `task vet`;
- `task test`;
- `task test:race`.

A local unsigned Darwin ARM64 candidate was built with `VERSION=2.1.1`. The repository release
inspector accepted it as version 2.1.1. It has SHA-256
`3e7d83648ac56206257b19e5bb44b9b89232d52a72f9d2b44330880b14a7d7cf`, seven ZIP entries, and six
manifest files matching the v2.0.0 Darwin inventory; `RUNNING.md` remains repository documentation
and is not in the governed payload.

The real v2.0.0-to-forward-fix tagged update and relaunch is **NOT RUN**. It requires a committed
source revision and a newly published, complete five-target release. The v2.1.0 tag and assets were
not moved or replaced. Because the published v2.1.0 binary requires the conflicting seven-file
inventory, a manually installed v2.1.0 cannot accept the six-file forward-fix archive and needs one
manual application replacement. No single exact-inventory archive can satisfy both binaries.

## Cumulative update changelog evidence

Validation date: 2026-08-29.

The provider now collects every non-draft release newer than the installed version that is eligible
for the installed stable or prerelease channel. It sorts those releases by semantic-version
precedence and projects one plain-text changelog with an explicit heading for each version, including
versions whose release notes are empty. Release discovery requests the provider maximum of 100
releases per page and traverses subsequent pages with a bounded failure instead of returning a
partial history. Provider tests cover unordered input, excluded stable-channel prereleases, drafts,
the installed version, empty notes, a prerelease-to-stable sequence, and a 101-release history.

The following gates passed after `go fix ./...` and formatting:

- focused cumulative provider tests;
- `task vet`;
- `task test`;
- `task test:race`;
- `task fmt:check` and `gopls check wails_updater.go wails_updater_test.go`;
- `task browser:test`: 188 passed and the two external ngrok journeys were skipped by design.

The browser journey verifies the renamed `ИСТОРИЯ ИЗМЕНЕНИЙ` section and multiple version
groups in the offered update without changing the existing accept/defer behavior.
