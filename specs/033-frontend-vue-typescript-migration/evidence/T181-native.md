# T181 Native Startup, Resource, and Security Evidence

Date: 2026-09-01
Host: macOS, darwin/arm64

## Evidence matrix

| Command | Result | Evidence class / reason |
|---|---|---|
| `task startup:check` | PASS | Native host/startup, Taskfile, workflow, portable release, and distribution Go contracts. |
| `scripts/wails-v3-cutover-check.sh` | PASS | Static native/runtime cutover: pinned tools and bindings; no v2, dual-runtime, generated-global, stale bundle/script, CI, or operating-document surface. |
| `scripts/secret-leak-check.sh` | PASS | Static secret-boundary scan: secret-bearing fields remain confined to private inputs/results. |
| `scripts/legacy-public-access-check.sh` | PASS | Static public-access ownership: one embedded production runtime and no legacy CLI/process/PATH/shared-domain/provider seam. |
| `scripts/state-changing-reset-native-smoke.sh` | FAIL | The stale default development bundle was rejected by LaunchServices with `kLSNoExecutableErr`; this result is not used as passing evidence. |
| `scripts/state-changing-reset-native-smoke.sh 'build/bin/Fallout Terminal.app'` | NOT RUN | The current packaged app launched, but macOS Accessibility automation could not prepare the native window. Grant Accessibility access to the invoking terminal and rerun. Diagnostics: `/private/tmp/fallout-native-reset.reALdA`. |

## Classification

Native static/startup/resource/secret/public-access evidence passed. Interactive native UI reset
evidence is explicitly NOT RUN because Accessibility control was unavailable to this session; no
browser or fake-port result is substituted for it. Signing, notarization, external provider, and
other-host evidence are not inferred here.
