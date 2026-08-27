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
