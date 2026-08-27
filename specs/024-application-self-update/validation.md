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
| SC-006 | NOT RUN | The minimum live journey is a real Darwin ARM64 tagged-prerelease download, replacement, relaunch at the offered version, and subsequent no-repeat check. It requires a spare tag and an installed prior version; no qualifying release was published for this validation. |
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

The single required live update/relaunch journey on Darwin ARM64 was **NOT RUN** because no qualifying
spare tag containing the self-update implementation was published. Windows AMD64, Windows ARM64,
Linux AMD64, and Linux ARM64 runtime journeys are outside the reduced live-acceptance scope; the
release must still publish the exact five governed archives, and deterministic release/provider,
verification, staging, helper, recovery, UI, and browser suites continue to cover those paths.
