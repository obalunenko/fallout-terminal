# Build and release packaging

The repository exposes Wails-aware build operations through Task. Bootstrap the pinned tools with
`make tools`; use `make help` to discover bootstrap commands and `task --list` for project work.

## Command reference

The normal development and verification entrypoints are:

```text
task dev
task run
task prepare
task build
task package
task deps
task fmt
task vet
task lint
task test
task proto:generate
task proto:check
task proto:breaking
task bindings:check
task browser:test
task check
```

Manual macOS signing remains separate: `task release:macos:preflight` checks credentials and
`task release:macos:signed` creates a Developer ID/notarized package. Automated portable release
publication uses `task release:publish` and does not require signing credentials.

## Package one target

Set both `GOOS` and `GOARCH` and run the common entrypoint on its matching native host:

```bash
task package GOOS=windows GOARCH=amd64
task package GOOS=windows GOARCH=arm64
task package GOOS=linux GOARCH=amd64
task package GOOS=linux GOARCH=arm64
task package GOOS=darwin GOARCH=arm64
```

The governed output is exactly five non-empty archives:

| Target | Release asset |
|---|---|
| `windows/amd64` | `Fallout-Terminal-windows-amd64.zip` |
| `windows/arm64` | `Fallout-Terminal-windows-arm64.zip` |
| `linux/amd64` | `Fallout-Terminal-linux-amd64.tar.gz` |
| `linux/arm64` | `Fallout-Terminal-linux-arm64.tar.gz` |
| `darwin/arm64` | `Fallout-Terminal-darwin-arm64.zip` |

Windows archives contain `Fallout Terminal.exe` and resources, Linux archives contain the
executable `Fallout Terminal` and resources, and the Darwin ZIP contains the complete unsigned
`Fallout Terminal.app` bundle. Per-target release eligibility checks the expected asset name,
non-empty archive, executable, and required resources. Runtime GUI, dialog, credential, player,
tunnel, and signing journeys are useful optional evidence, but are not release eligibility gates.

## Optional local aggregate

On an Apple Silicon Mac, `task package:all` can create a local developer convenience output from
the current checkout. Docker builds only the four Windows/Linux targets; Darwin is packaged on the
host. Choose another destination with `task package:all OUTPUT=build/portable-local`.

This aggregate is local-only, may include unpacked verification directories, and never runs in CI.
It is not a release candidate, upload inventory, or source of release publication state. A failure
returns a nonzero exit status and preserves an already verified destination.

## Quality and tag workflows

Pull requests and pushes to `main` run the read-only quality workflow. It calls `task ci:quality`
and cannot create releases or upload governed release assets.

A push of a strict SemVer tag on the current major line, such as `v2.0.0` or `v2.0.0-rc.1`, starts
the tag-only workflow. The tag major must match the root Go module major; the current preflight
accepts only `v2`. Its five native jobs use the common packaging entrypoint, inspect one archive
each, and upload one artifact each. The publish job downloads the complete flat inventory, refuses
missing, duplicate, empty, or extra files, and calls `task release:publish`. That task runs the
repository-pinned `pinned GoReleaser` module and publishes exactly five assets. GoReleaser is
create-only: an existing release, including a draft, is refused before packaging and checked again
before publication. Prerelease suffixes produce GitHub prereleases automatically.

The workflow never deletes, replaces, or edits a GitHub release. It also never publishes checksums,
an installer, or a package-registry copy. Release validation uses static fixtures locally; the sole
end-to-end acceptance is an explicitly approved unused prerelease tag on a committed revision.

## Partial release recovery

If publication fails before GitHub creates a release, fix the cause and rerun the same tag immediately.
If GitHub contains a partial release, do not rerun yet: **Delete the partial release manually**, verify
that no release remains for the tag, and then rerun the same tag. Keep the tag itself unless a
maintainer has approved moving it to a different committed revision.

An incomplete or unverifiable inventory **не публикуется**. Every rejected target, invalid tag,
existing release, or publisher failure returns a nonzero **код завершения**.
