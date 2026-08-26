# Distribution Guidance Checklist: Windows and Linux Desktop Support

**Purpose**: Confirm that a new user or maintainer can select, launch, troubleshoot, and package every supported portable target using the published guidance in under five minutes.
**Created**: 2026-08-26
**Feature**: [spec.md](../spec.md)
**Guidance reviewed**: [README.md](../../../README.md), [platform-support.md](../../../docs/platform-support.md), [platform-packaging.md](../../../docs/platform-packaging.md)

## Five-minute user path

- [x] The README table maps each exact target (`windows/amd64`, `windows/arm64`, `linux/amd64`, `linux/arm64`) to one stable, non-colliding archive name.
- [x] The platform guide states the Windows 10/11 and WebView2 requirements and the Linux GTK4, WebKitGTK 6.0, graphical-session, and Secret Service requirements before the launch steps.
- [x] The platform guide gives checksum, extraction, and native launch commands for both ZIP and TAR.GZ distributions without requiring the source tree or developer tools.
- [x] The platform guide identifies Windows Known Documents and `%APPDATA%`, plus Linux XDG Documents and `$XDG_CONFIG_HOME`/`~/.config`, while keeping bundled resources separate.
- [x] The platform guide explains Windows Credential Manager and Linux Secret Service behavior, fail-closed public access, and continued local/LAN operation.
- [x] Troubleshooting entries turn missing runtimes, display access, locked credential stores, missing resources, storage permissions, and architecture mismatch into concrete recovery actions.

## Five-minute maintainer path

- [x] `make tools` is documented as the single bootstrap command that installs every pinned Go tool module, including Task and Wails.
- [x] `make help` documents the bootstrap without mutating the workspace and directs maintainers to `task --list` for project workflows.
- [x] The README leads with Task commands and links to the complete command reference; it does not present Make as an application workflow owner.
- [x] The packaging guide documents every root Task command, its purpose, important inputs, and nonzero failure behavior.
- [x] The four matching-host `task package GOOS=... GOARCH=...` commands map directly to the four canonical target identifiers and archive names.
- [x] `task package:all [OUTPUT=<directory>]` documents local Docker prerequisites, current-checkout behavior, static verification, quarantine, and atomic output publication; `task package:all:remote` separately documents GitHub authentication, branch synchronization, and native launch evidence.
- [x] The combined `fallout-terminal-portable` artifact and `aggregate-index.json` inventory are explicit, and any missing or unverifiable target is documented as ineligible for publication.

## Result

- [x] SC-008 guidance is complete: target choice, prerequisites, launch, data locations, protected credentials, and troubleshooting are available through the README and one linked platform page.
- [x] SC-009 guidance is complete: one tool bootstrap and the pinned Task workflow/package matrix are available through the README and one linked packaging page.
- [x] Review found no documentation step that requires unpublished knowledge or a locally installed development tool for end-user launch.

## Notes

- This is a documentation usability review, not native execution evidence. Native target and CI results are recorded separately in `validation.md`.
- No local test suite was run, as requested.
