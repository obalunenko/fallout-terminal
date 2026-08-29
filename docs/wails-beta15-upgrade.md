# Wails v3.0.0-beta.15 upgrade record

## Decision

Advance the accepted Wails v3 baseline from `v3.0.0-beta.13` to
`v3.0.0-beta.15` across the root Go runtime, isolated `wails3` tool module,
and `@wailsio/runtime` frontend package. All three surfaces remain pinned to
the same exact prerelease and their generated checksums stay committed.

Beta.14 fixes macOS Control-key accelerator naming and Windows tray icon and
theme behavior. Beta.15 increases the WebView2 embedding timeout to 60 seconds.
The upstream beta.13-to-beta.15 comparison contains runtime and platform fixes
but no release-noted application API removal. Sources:

- [Wails v3.0.0-beta.14 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.14)
- [Wails v3.0.0-beta.15 release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.15)
- [`@wailsio/runtime` package](https://www.npmjs.com/package/@wailsio/runtime)

## Compatibility boundaries

- Root composition and `internal/platform` remain the only Wails-dependent Go
  boundaries; domain and application services gain no framework dependency.
- The isolated CLI module stays separate from the production module.
- The Overseer remains the only frontend that imports `@wailsio/runtime`.
- Completed specifications and rollback records retain the dependency versions
  that they originally validated; the constitution and active dependency
  contracts own the new accepted baseline.
- No protobuf, session JSON, ConnectRPC, player, or desktop-service API change
  is introduced by this dependency update.

## Verification plan

1. Tidy the root and isolated Wails Go modules from their owning directories.
2. Install the exact frontend runtime and regenerate Wails bindings with the
   beta.15 CLI.
3. Run Wails pin, tool-module, dependency-license, cutover, and binding drift
   checks.
4. Run `go fix ./...`, formatting, vet, lint, tests, race tests, frontend builds,
   and the matching-host application build and package gates.
5. Confirm generated bindings expose the same reviewed desktop method and event
   surface and that notification and updater adapters still compile.

## Rollback

If compatibility validation fails, restore the exact beta.13 Go, CLI, and npm
pins together, regenerate sums, lockfiles, and bindings, and rerun the same
contract and build gates. Do not retain a mixed-version runtime or add a
beta-selection switch.
