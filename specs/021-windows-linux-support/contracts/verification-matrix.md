# Contract: Native Verification Matrix

## Target ownership

| Target | Native runner | Build/runtime prerequisites | Archive |
|---|---|---|---|
| `windows/amd64` | GitHub-hosted Windows x64 runner | Pinned Go/Node/tools, Windows build facilities, WebView2 | ZIP |
| `windows/arm64` | GitHub-hosted Windows 11 arm64 runner, or documented matching self-hosted fallback | Pinned Go/Node/tools, Windows build facilities, WebView2 | ZIP |
| `linux/amd64` | Ubuntu 24.04 x64 runner | Pinned Go/Node/tools, CGO, GTK4, WebKitGTK 6.0, Xvfb, Secret Service test session | TAR.GZ |
| `linux/arm64` | Ubuntu 24.04 arm64 runner, or documented matching self-hosted fallback | Pinned Go/Node/tools, CGO, GTK4, WebKitGTK 6.0, Xvfb, Secret Service test session | TAR.GZ |

Workflow runner labels are pinned explicitly in `.github/workflows/wails-portable.yml`, not inherited from an ambiguous `latest` label. If a hosted arm64 label is unavailable to the repository, the documented fallback must still be a matching native arm64 host and must produce the same checks; emulation does not satisfy launch acceptance.

## Per-target gate

Every target job starts from a clean checkout of the aggregate run’s resolved SHA and runs independently with matrix fail-fast disabled.

Required phases, in order:

1. Verify pinned tool versions and native build dependencies.
2. Run repository lint, protobuf/generation, Go, frontend, dependency, and license gates required by the package graph.
3. Run `task package GOOS=<os> GOARCH=<arch>` through the repository-pinned Task binary.
4. Verify archive safety, exact inventory, manifest, checksum, and PE/ELF target identity.
5. Extract to a clean path containing spaces and launch from an unrelated working directory.
6. Observe a real Overseer application window within 60 seconds and load the bundled demo.
7. Exercise native open/save and external-link adapters in the platform acceptance harness.
8. Verify secure-store success when the native service is available and a clear fail-closed state when it is intentionally unavailable/locked.
9. Connect one local player, observe synchronized state, close the application, and confirm listener, public-access resources, child processes, and application process exit.
10. Upload the archive, checksum, and redacted verification record only after all phases succeed.

No local test execution is required while creating this plan. These gates execute in CI or matching target environments during implementation and acceptance.

## Failure semantics

- A target job reports its canonical target and failed phase even when another target fails.
- Missing native runtime dependencies produce an actionable prerequisite failure, not a silent exit.
- Secrets and credential payloads never appear in command output, logs, manifests, screenshots, or uploaded records.
- A failed, timed-out, canceled, mismatched, or unverified target uploads no runnable artifact.
- Retry creates a new verification record; it does not mutate failed evidence into success.

## Aggregate gate

The aggregate job runs even when a target fails so it can provide a complete matrix report. It is successful only when:

- all four canonical targets have exactly one successful verification record;
- all records resolve the same source SHA;
- archive names are the four stable names with no collision or extra target;
- each sidecar verifies its archive and each manifest agrees with the job target;
- every target includes native window, demo-load, and clean-shutdown evidence; and
- the existing macOS workflow remains a separate required compatibility check for release readiness.

On success the job publishes one combined downloadable workflow artifact containing the four portable archives, their sidecars, and a redacted aggregate index. On any partial result it publishes only diagnostic job logs/records permitted by repository policy and marks the aggregate run failed.

## Tagged five-target publication gate

The four-target aggregate remains the native Windows/Linux evidence owner. On a valid SemVer tag, a
separate macOS arm64 job at the same tag SHA runs the established Developer ID signing,
hardened-runtime, notarization, stapling, DMG, Gatekeeper, launch/resource, and checksum gates. Its
eligible output is exactly `Fallout-Terminal-arm64.dmg` plus
`Fallout-Terminal-arm64.dmg.sha256`.

Publication eligibility requires all of the following before either destination reports success:

- the macOS job and all four portable target jobs resolved the exact tag SHA and succeeded;
- the Darwin DMG and sidecar, four portable archives and sidecars, and `aggregate-index.json` are
  present, nonempty, uniquely named, and contain no unexpected release input;
- every checksum verifies and the aggregate index identifies the same tag SHA;
- the repository-pinned GoReleaser v2 configuration accepts the complete inventory and preserves
  prerelease suffix behavior; and
- the versioned GHCR artifact uses the same tag and exact joined inventory as the GitHub Release.

GoReleaser owns GitHub Release creation/update and consumes preverified inputs without compiling.
The repository-pinned ORAS client publishes the versioned GitHub Packages artifact. A missing
macOS credential, failed trust gate, failed portable target, mismatch, unexpected input, or
publication error is nonzero and cannot be presented as a complete tagged release. Diagnostic
workflow artifacts remain distinct from GitHub Release assets and versioned GHCR success.

## Evidence retention

The aggregate index records the workflow run, source SHA, target, archive name, archive SHA-256, native runner identity, and check outcomes. It contains no absolute user paths, credentials, session data created during smoke tests, or mutable “latest” target labels. Documentation links each supported target to its prerequisite and troubleshooting guidance.
