# Contract: V2 Source and Release Identity

**Bugfix**: 2026-08-27 — BUG-001 added the tag-to-artifact version contract.

**Remediation**: 2026-08-27 — Constitution 8.1.0 authorizes the narrow identity gate and the
contract now fixes local identity, executable output, staged metadata, and rollback behavior.

## Root application module

The active application module declaration is exactly:

```text
module github.com/obalunenko/Fallout-Terminal/v2
```

All buildable application imports and generated Go imports resolve through that module. The
unsuffixed path may remain only in completed historical records or ordinary repository links; it
must not remain as an active Go import or Wails binding namespace.

Modules under `tools/*/go.mod` retain their existing independent identities and dependency graphs.

## Release preflight CLI

The governed command is:

```text
go run ./cmd/build validate-release-tag --tag <v2.MINOR.PATCH[-PRERELEASE]>
```

The root Task/release workflow may invoke this lower-level checked-in implementation seam; it must
not duplicate tag policy in YAML or shell.

### Accepted candidates

- `v2.0.0` — stable
- `v2.0.0-rc.1` — prerelease
- `v2.1.3-beta.1` — prerelease

### Rejected candidates

- A syntactically valid tag whose major is not 2, including `v0.0.0-rc.1`, `v1.2.3`, and
  `v3.0.0`.
- A tag without the lowercase `v` prefix or with surrounding whitespace.
- A major, minor, or patch number with a leading zero.
- A numeric prerelease identifier with a leading zero.
- An empty or malformed prerelease suffix.
- Any build-metadata suffix such as `+build.1`.

Validation failure returns a nonzero exit status with an actionable format or major-mismatch error.
Success classifies the candidate as stable or prerelease. Every package job depends on this
preflight, so rejection occurs before target-specific work begins.

## Protobuf language-package identity

Every schema keeps its existing `fallout.terminal.*.v1` protobuf package and changes only its Go
package option to the corresponding path under:

```text
github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/
```

The following remain unchanged:

- message and enum names;
- field names, numbers, types, and presence;
- service and method names;
- unary, server-streaming, and client-streaming directions;
- ConnectRPC procedure paths and HTTP/static routes;
- validation, authorization, ordering, revision, publication, cancellation, and reconnect
  behavior;
- version-1 session and player-configuration JSON representations.

The schema revision and compatibility descriptor advance only after review confirms this
metadata-only delta. All established breaking-change fixtures must remain rejected.

## Wails binding identity

Clean binding generation produces the application service at:

```text
frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js
```

The old unsuffixed binding tree must be absent. Active consumers and browser fixtures import the
new path. The observable private bridge contract remains exactly one registered desktop service,
35 allowlisted methods, and these six named events:

```text
server-info
client-count
hack-state
coordination-state
session-state
public-access-status
```

No generic dispatcher or arbitrary filesystem, process, environment, browser, or player-service
capability is introduced.

## Persistence compatibility

The source/release major does not renumber data formats:

- session JSON remains version 1 and preserves compatible recursive unknown fields;
- player-configuration JSON remains version 1 and preserves its existing strict rejection of
  unsupported unknown fields without rewriting the source;
- save targets, references, defaults, storage locations, and bundled sample behavior remain
  unchanged.

## Package and publication boundary

After preflight, the existing matrix continues producing exactly one governed archive for each of
`windows/amd64`, `windows/arm64`, `linux/amd64`, `linux/arm64`, and `darwin/arm64`. Archive names,
required resources, unsigned distribution, create-only GitHub Release publication, and manual
partial-release recovery remain unchanged.

~~Under the unpatched specification, the accepted tag was not required to be embedded in platform
metadata or the running executable. That separate conflict was recorded in
`../bugs/BUG-001.md` and was not treated as resolved by this contract.~~

## Tag-to-artifact version contract

For an accepted release tag, preflight removes only the leading `v` and exposes the result as the
one canonical `VERSION` consumed by all five package jobs:

| Raw tag | Canonical `VERSION` | Numeric core | Four-part numeric |
|---|---|---|---|
| `v2.0.0` | `2.0.0` | `2.0.0` | `2.0.0.0` |
| `v2.0.0-rc.1` | `2.0.0-rc.1` | `2.0.0` | `2.0.0.0` |

The build tool owns validation and propagation below the Task graph:

- Go linker flags inject the canonical value into the application version owner.
- Useful Go VCS build metadata remains enabled and identifies the source revision independently.
- `<executable> --version` accepts no additional arguments, writes only the selected identity plus
  one newline to standard output, writes nothing to standard error, exits successfully, and does so
  before Wails, storage, listeners, player services, or public-access services start.
- Local builds without an accepted tag use an explicit non-release value and cannot pass tagged
  package verification.

Package-time metadata rendering follows the owning platform syntax and never modifies the
worktree:

- Darwin renders `build/darwin/Info.plist.tmpl` directly to
  `build/bin/darwin-arm64/stage/Fallout Terminal/Fallout Terminal.app/Contents/Info.plist`;
  human-readable version metadata uses the canonical value and numeric bundle fields use the
  numeric core where required.
- Windows string `FileVersion` and `ProductVersion` metadata uses the canonical value; fixed file
  and product versions and the manifest assembly identity use the four-part numeric value.
  `build/windows/info.json.tmpl` and `build/windows/app.manifest.tmpl` render to
  `build/bin/windows-<arch>/metadata/info.json` and
  `build/bin/windows-<arch>/metadata/app.manifest` before native resource generation.
- Linux has no separate platform metadata file; its packaged executable still carries and reports
  the canonical value.
- Checked-in production metadata must not retain an independent release number such as `1.0.0` or
  `1.0.0.0`.

With no local `VERSION`, the executable and human-readable metadata use `development`; numeric-only
fields use `0.0.0` or `0.0.0.0`. Release inspection always requires an expected canonical version
and rejects `development`, missing, malformed, or mismatched values.

Each matching native package job verifies before upload that the packaged executable reports the
canonical value and that applicable human-readable and numeric platform fields equal the derived
representations above. Missing, malformed, non-release, or mismatched values fail the job. These
checks extend release eligibility without changing the five archive names, layouts, target set, or
create-only publication contract.

## Rollback boundary

Before a complete v2 release is published, the whole cutover may return to immutable revision
`3f2b6e584aee4c5279a3d54ae70aa44ee578a21a` and regenerate that revision's prior identities. After
publication, the tag, release, and archives remain immutable and correction uses a new strict v2
patch or prerelease tag. Neither path permits simultaneous active root module or Wails binding
identities.
