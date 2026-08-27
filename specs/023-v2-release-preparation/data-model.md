# Data Model: V2 Release Preparation

**Bugfix**: 2026-08-27 — BUG-001 added the canonical packaged release version model.

This feature introduces no application runtime or persistent data entity. Its governed model is a
set of repository identities and compatibility relationships that must transition atomically.

## Application Source Identity

Represents the canonical Go identity of the shipped application source.

| Attribute | Value | Validation |
|---|---|---|
| Repository | `github.com/obalunenko/Fallout-Terminal` | Stable repository identity. |
| Module path | `github.com/obalunenko/Fallout-Terminal/v2` | Exact root `go.mod` declaration and root-build guard. |
| Major | `2` | Must match every accepted release tag major. |
| Active imports | `/v2/...` | All buildable application packages and generated Go imports use one identity. |
| Legacy active fallback | None | Unsuffixed application imports and generated paths are rejected outside historical records. |

## Release Tag Candidate

Represents one tag presented to release preflight.

| Attribute | Type | Validation |
|---|---|---|
| Raw tag | string | Exact `vMAJOR.MINOR.PATCH` plus optional SemVer prerelease suffix. |
| Major | unsigned decimal string | No leading zero; must equal `2`. |
| Minor | unsigned decimal string | No leading zero except the single value `0`. |
| Patch | unsigned decimal string | No leading zero except the single value `0`. |
| Prerelease | optional dot-separated identifiers | Alphanumeric/hyphen identifiers; numeric identifiers have no leading zero. |
| Build metadata | absent | Any `+...` suffix is rejected. |
| Classification | stable or prerelease | Derived only after all validation succeeds. |
| State | rejected or preflight-approved | Approval is required before any package job starts. |

## Generated Contract Identity

Represents language metadata layered onto stable application contracts.

| Attribute | Rule |
|---|---|
| Protobuf package | Existing `fallout.terminal.*.v1` names remain unchanged. |
| Go package option | Resolves under `github.com/obalunenko/Fallout-Terminal/v2/internal/gen/...`. |
| Go/Connect output | Regenerated under `internal/gen/fallout/terminal/`. |
| ECMAScript output | Regenerated under `frontend/client/gen/fallout/terminal/player/v1/`. |
| Wails binding namespace | Generated under `frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/`. |
| Behavioral contract | Existing messages, fields, services, routes, methods, events, and directions remain unchanged. |

## Packaged Release Version

Represents the single version identity embedded into one tagged release's five target packages.

| Attribute | Value or rule | Validation |
|---|---|---|
| Raw tag | `vMAJOR.MINOR.PATCH[-PRERELEASE]` | Must already be accepted by release preflight. |
| Canonical version | `MAJOR.MINOR.PATCH[-PRERELEASE]` | Remove only the leading `v`; this is the sole non-empty release `VERSION` input. |
| Local identity | `development` | Used when local build/package input has no `VERSION`; rejected by tagged-release inspection. |
| Numeric core | `MAJOR.MINOR.PATCH` or `0.0.0` | Derived from a release version; local non-release metadata uses zeroes. |
| Four-part numeric version | `MAJOR.MINOR.PATCH.0` or `0.0.0.0` | Deterministic Windows representation for release or local mode. |
| Human-readable version | Canonical version or `development` | Darwin/Windows descriptive metadata and executable report retain the selected identity. |
| Go embedded version | Canonical version or `development` | Linker-injected for releases; `<executable> --version` exits before application startup. |
| VCS metadata | Retained | Revision/time/modified evidence remains available and is not a substitute for release version. |
| Release state | non-release, candidate, verified, or rejected | Only verified candidates may be uploaded. |

## Compatibility Baseline

Represents the reviewed descriptor state after the language-package identity changes.

| Attribute | Rule |
|---|---|
| Schema revision | Hashes the complete checked-in protobuf source set. |
| Descriptor image | Built from the reviewed schema set with pinned Buf. |
| Allowed delta | `go_package` identity only for this feature. |
| Negative fixtures | Field-number, field-type, enum-value, package-name, and service-method breaks remain rejected. |
| Reproducibility | Two clean generations must be identical and must not modify root module dependency files. |

## Persistent Document Compatibility

| Document | Version | Compatibility behavior |
|---|---|---|
| Session JSON | `1` | Known fields and recursive compatible unknown fields survive open/save/reopen. |
| Player-configuration JSON | `1` | Known fields survive open/save/reopen; unsupported unknown fields remain rejected without rewriting the source. |

Application release major 2 has no state relationship to either persistence version beyond the
requirement that the v2 application continue reading and writing their established version-1
contracts.

## Relationships

- One Application Source Identity owns all active application Go imports and generated Go/Wails
  namespaces.
- One accepted Release Tag Candidate must have the same major as the Application Source Identity
  before it can create one Packaged Release Version and start the unchanged five-target package
  matrix.
- One Packaged Release Version is shared by all five target jobs and generates their Go linker,
  human-readable platform, and numeric platform representations.
- One reviewed Compatibility Baseline records the Generated Contract Identity while preserving
  protocol behavior.
- Persistent Document Compatibility remains independent of source and release major numbering.
- Each `tools/*` module remains independently versioned and is not a child of the Application
  Source Identity.

## State Transitions

### Source cutover

`unsuffixed application identity` → update root/import/schema metadata → regenerate contracts and
bindings → remove old active namespace → validate one `/v2` identity

### Release preflight

`raw tag` → strict syntax validation → major-equality validation → classify stable/prerelease →
`preflight-approved` → package matrix

Any failed validation transitions directly to `rejected` and starts no package job.

### Package version verification

`preflight-approved tag` → derive canonical `VERSION` once → compile/link and render platform
metadata → assemble package → inspect executable and applicable metadata → `verified` → upload

A missing, malformed, non-release, or mismatched value transitions to `rejected`; it never reaches
upload. Local builds without an accepted tag use `development` plus zero-valued numeric metadata
and remain explicitly `non-release`.

### Release rollback

`pre-publication cutover` → restore immutable branch point and regenerate prior artifacts → verify
one prior identity, or `published release` → preserve tag/release → create a new forward-fix v2 tag

Rollback never creates a revision containing both root module or Wails binding identities.

### Compatibility review

`reviewed pre-v2 descriptor baseline` → verify only `go_package` changes → regenerate twice → run
breaking fixtures → record updated revision/baseline → `reviewed v2 language-identity baseline`
