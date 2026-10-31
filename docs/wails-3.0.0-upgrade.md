# Wails v3.0.0 upgrade record

## Decision

Advance the accepted Wails v3 baseline from `v3.0.0-beta.28` to the final
`v3.0.0` release across the root Go runtime, isolated `wails3` tool module,
and `@wailsio/runtime` frontend package. All three surfaces remain pinned to
the same exact release and their generated checksums, lockfile, and bindings
stay committed. No nightly (`v3.1.0-nightly.*`) is used. Upstream now ships
weekly minor releases and patch hotfixes; each later move is a separate,
recorded upgrade, not a floating range.

The beta.29-to-v3.0.0 comparison contains no release-noted removal from the
application, window, service, event, dialog, browser, clipboard, notification,
or updater APIs that this repository uses, and the root module compiles
unchanged against v3.0.0. The changes that touch this repository are:

- **Updater staging location.** Beta.31 introduced this as a Linux
  cross-filesystem fix, and in v3.0.0 it applies on every platform: the updater
  downloads and extracts a release into a `wails-update-*` directory beside its
  replacement target instead of the system temporary directory. On macOS
  that is beside the `.app` bundle; on Windows and Linux it is inside the
  portable unit directory, beside the executable. The application-owned stager
  rejects a replacement unit that lies inside the installed unit, so without an
  adaptation every Windows and Linux self-update would fail at staging, and the
  emptied download directory would remain beside the macOS bundle.
  `isolateApplicationUpdatePackage` in `wails_updater.go` now moves a package
  that Wails extracted inside the installed unit to an attempt-owned sibling
  (`.<unit>.download-<attempt>-*`) on the same volume, and removes the Wails
  download directory and that sibling once staging finishes or fails.
- **Updater restart and verification hardening (v3.0.0).** Wails' own
  `Restart` now refuses a second restart and validates the payload before its
  swap, and a configured `PublicKey` now makes signatures mandatory. This
  application stages and promotes updates with its own replacement helper and
  configures digest-only verification without a public key, so neither change
  alters its update flow.
- **Bindings generator (beta.33).** The v3 bindings generator additionally
  emits `bindings/events/index.js`, which exports one name constant for each of
  the seven registered events. It is committed with the other generated files.
  The Overseer does not import it, and the reviewed surface is unchanged: 40
  desktop methods, the seven named events, and identical model and service
  files. The Go 1.27 `encoding/json` alias fix (beta.31) produces no output
  change for this repository's models.
- **macOS window events (beta.30).** `WindowDidUpdate` and `WindowWillUpdate`
  are no longer posted. The application does not observe them. Removing them
  also stops macOS 27 from consuming the transient user activation, so the
  Overseer's primary `navigator.clipboard.writeText` path can succeed before
  its native `Clipboard.SetText` fallback is needed.
- **macOS window teardown (beta.30).** Duplicate window teardown is prevented
  inside Wails. The application's `WindowClosing` hook and `WindowWillClose`
  fallback still share one exactly-once quit request.
- **Windows notifications (beta.31).** WinRT toast failures are returned
  instead of falling back to PowerShell. Approval notifications already treat
  any native delivery error as optional, and the in-app approval prompt stays
  authoritative.
- **Security and platform fixes.** The release blocks reserved `wails:`
  messages from cross-origin child frames on Linux (GHSA-2p5f-2gx8-mh8m,
  which the v3.0.0 notes list as partially fixed and which is not yet public)
  and from iframes on macOS. It also hardens updater archive extraction and
  contains WebView2, Windows menu, and dialog fixes. Server mode
  (`-tags server`) hardening does not apply because this application does not
  build in server mode. The Overseer embeds no iframes.
- **Toolchain.** The Wails module now declares Go 1.26 as its minimum. The
  repository stays on Go 1.27.

Minimum version selection raises `golang.org/x/net` from v0.56.0 to v0.60.0 in
the root module, an indirect runtime dependency of Wails, and the notice
inventory records the new version. The isolated tool module also raises its
own indirect dependencies, including `golang.org/x/crypto`, `golang.org/x/image`,
`golang.org/x/net`, and `github.com/klauspost/compress`. They stay owned by
`tools/wails/` and do not enter the application module. The published
`@wailsio/runtime` 3.0.0 package matches 3.0.0-beta.28 byte for byte except for
`package.json`, which changes the version and adds a `./cancellable` subpath
export.

Sources:

- [Wails v3.0.0-beta.29 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.29)
- [Wails v3.0.0-beta.30 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.30)
- [Wails v3.0.0-beta.31 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.31)
- [Wails v3.0.0-beta.32 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.32)
- [Wails v3.0.0-beta.33 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.33)
- [Wails v3.0.0-beta.34 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.34)
- [Wails v3.0.0 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0)
- [Wails changelog](https://github.com/wailsapp/wails/blob/master/docs/mpress/content/changelog.md)
- [`@wailsio/runtime` package](https://www.npmjs.com/package/@wailsio/runtime)

## Compatibility boundaries

- Root composition and `internal/platform` remain the only Wails-dependent Go
  boundaries; domain and application services gain no framework dependency.
  The updater adaptation stays in the root Wails adapter, and `internal/update`
  keeps its rule that a replacement unit must lie outside the installed unit.
- The isolated CLI module stays separate from the production module.
- The Overseer remains the only frontend that imports `@wailsio/runtime`.
- Completed specifications and rollback records retain the dependency versions
  that they originally validated; the constitution (8.3.2) and active
  dependency contracts own the new accepted baseline.
- No protobuf, session JSON, ConnectRPC, player, or desktop-service API change
  is introduced by this dependency update. The generated desktop method list
  and the seven named events are unchanged.

## Verification plan

1. Tidy the root and isolated Wails Go modules from their owning directories.
2. Install the exact frontend runtime and regenerate Wails bindings with the
   v3.0.0 CLI.
3. Run Wails pin, tool-module, dependency-license, cutover, and binding drift
   checks.
4. Run `go fix ./...`, formatting, vet, lint, tests, race tests, frontend builds,
   browser journeys, and the matching-host application build and package gates.
5. Confirm generated bindings expose the same reviewed desktop method and event
   surface and that notification and updater adapters still compile.
6. Exercise the real v3.0.0 updater download, verification, and extraction
   pipeline through the application stager in both installed-unit layouts.

Results on macOS arm64 (Darwin 27.0), Go 1.27.3, Node.js 26.8.1, and Task
3.54.0, recorded on 2026-10-31. Only the matching darwin/arm64 host was
exercised; Windows targets were cross-compiled and vetted, and Linux was not
built.

| Gate | Result |
| --- | --- |
| `go get` and `go mod tidy` in the root and `tools/wails/` modules | `PASS`. The root module raises only `golang.org/x/net`, to v0.60.0; the tool module raises its own indirect dependencies |
| `npm install --save-exact @wailsio/runtime@3.0.0` (Overseer workspace) | `PASS`. The lockfile integrity matches the registry `dist.integrity` |
| `go tool -modfile=tools/wails/go.mod wails3 generate bindings -clean -d frontend/overseer/bindings ./...` (v3.0.0 CLI) | `PASS`. 1 service, 40 methods, 7 enums, 60 models, and 7 events. The only new file is `bindings/events/index.js` |
| `task wails:pins:check` | `PASS` |
| `scripts/tool-modules-check.sh` and `--self-test` | `PASS` |
| `scripts/dependency-license-check.sh` | `PASS` |
| `scripts/wails-v3-cutover-check.sh` and `--self-test` | `PASS` |
| `task secret-leak:check` | `PASS` |
| `task bindings:check` | `PASS`. Deterministic output, exactly 40 accepted desktop methods, and seven named events |
| `go fix ./...` | `PASS`. No edits proposed |
| `task fmt:check` | `PASS` |
| `task vet` | `PASS`. `GOOS=windows` vet and `windows/amd64` and `windows/arm64` builds of the root package also passed |
| `task lint` | `PASS`. 0 issues |
| `task test` | `PASS` |
| `task test:race` | `PASS` |
| `task frontend:build` | `PASS` |
| `task browser:test` | `PASS`. 212 passed and 2 skipped: the two live authenticated-ngrok journeys, which need provider credentials |
| `task build` | `PASS`. `build/bin/Fallout Terminal`; the regenerated bindings matched the committed tree |
| `task package` | `PASS`. The ad-hoc signed `build/bin/Fallout Terminal.app` passes `codesign --verify --deep --strict` and reports `development` for `--version` |
| `task package GOOS=darwin GOARCH=arm64` | `PASS`. `build/dist/Fallout-Terminal-darwin-arm64.zip` and its checksum |
| Real updater pipeline, portable layout (download inside the installed unit) | `PASS` with the adaptation. A test binary placed in a `Fallout Terminal/` unit downloaded the darwin/arm64 archive through `pkg/updater`. Wails extracted it to `Fallout Terminal/wails-update-*/`, the package was moved out, and an adjacent staged unit was produced, with no download directory left behind. Without the adaptation the same run fails with `source and installed application units must be separate` and leaves `wails-update-*` inside the unit |
| Real updater pipeline, bundle layout (download beside the `.app`) | `PASS`. The package is staged from beside the bundle, and the Wails download directory is removed afterwards |

## Rollback

If compatibility validation fails, restore the exact `v3.0.0-beta.28` Go, CLI,
and npm pins together, regenerate sums, lockfiles, and bindings (removing
`bindings/events/`), and rerun the same contract and build gates. The
package isolation in `wails_updater.go` stays correct under beta.28, which
extracts into the system temporary directory: that package is never inside the
installed unit, so it is staged in place and its download directory is removed
afterwards. Do not retain a mixed-version runtime, adopt a nightly, or add a
version-selection switch.
