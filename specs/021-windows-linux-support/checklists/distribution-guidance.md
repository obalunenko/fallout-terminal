# Distribution Guidance Checklist: Windows and Linux Desktop Support

**Purpose**: Confirm that a new user or maintainer can select, launch, troubleshoot, and package every supported portable target using the published guidance in under five minutes.
**Created**: 2026-08-27
**Feature**: [spec.md](../spec.md)
**Guidance reviewed**: [README.md](../../../README.md), [platform-support.md](../../../docs/platform-support.md), [platform-packaging.md](../../../docs/platform-packaging.md)

## Five-minute user path

- [x] The README table maps each exact target (`windows/amd64`, `windows/arm64`, `linux/amd64`, `linux/arm64`, `darwin/arm64`) to one stable, non-colliding portable archive name.
- [x] The platform guide defines support as availability of the governed archive and makes native launch evidence optional and non-gating.
- [x] The platform guide states Windows, Linux, and unsigned macOS runtime prerequisites before optional launch steps.
- [x] The platform guide gives extraction and optional native launch commands for ZIP and TAR.GZ distributions without requiring the source tree or developer tools.
- [x] The platform guide identifies platform-specific user documents, settings, and session locations while keeping bundled resources separate.
- [x] The platform guide explains protected credential behavior and actionable recovery for runtime, display, storage, resource, and architecture mismatches.

## Five-minute maintainer path

- [x] `make tools` is documented as the bootstrap for repository-pinned Go tools, including Task, Wails, Buf, lint, and GoReleaser ownership where applicable.
- [x] `make help` documents bootstrap behavior and directs maintainers to `task --list` for project workflows.
- [x] The README leads with Task commands and links to the complete command reference; it does not present Make as an application workflow owner.
- [x] The packaging guide documents every retained root Task command, its purpose, important inputs, and nonzero failure behavior.
- [x] The five matching-host `task package GOOS=... GOARCH=...` commands map directly to the five canonical target identifiers and archive names.
- [x] `task package:all [OUTPUT=<directory>]` is documented as optional local Docker convenience that never runs in CI or gates tagged releases; the retired remote aggregate and local release commands are absent.
- [x] The tag workflow is documented as entering through the CI-owned `release:publish` Task command and publishing exactly five portable archives through pinned GoReleaser, with no checksum sidecars, aggregate index, raw executable, DMG, ORAS, or package-registry asset.
- [x] Create-only refusal and the conditional recovery paths are explicit: rerun immediately when no release exists, or manually delete a partial release before rerunning the same tag.
- [x] The acceptance procedure requires static/local validation and commit before a maintainer explicitly approves one unused SemVer prerelease tag, and it preserves the successful prerelease as evidence.

## Result

- [x] SC-008 guidance is complete: target choice, archive-availability meaning, prerequisites, optional launch, data locations, protected credentials, and troubleshooting are available through the README and one linked platform page.
- [x] SC-009 guidance is complete: tool bootstrap and the pinned Task workflow/package matrix are available through the README and one linked packaging page.
- [x] Review found no documentation step that requires unpublished knowledge or a locally installed development tool to select and unpack an archive.

## Notes

- T095 completed on 2026-08-27. A timed static walkthrough found both the user path and maintainer path in under five minutes; the focused documentation contract test passed.
- This is a documentation usability review, not native execution evidence. Optional native results, if run, are recorded separately in `validation.md`.
