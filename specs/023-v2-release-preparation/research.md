# Research: V2 Release Preparation

**Bugfix**: 2026-08-27 — BUG-001 updated the version-embedding decision after the governing scope
conflict was patched. Post-analysis remediation incorporates Constitution 8.1.0 and fixes the
local/release, executable-report, metadata-template, and rollback contracts.

## Decision 1: Use Go semantic import versioning for the application module

**Decision**: Change the root application module identity to
`github.com/obalunenko/Fallout-Terminal/v2` and update every active application import to that
identity in one cutover.

**Rationale**: Go requires a `/v2` suffix for a version-2 module. Moving the root declaration,
application imports, generated Go metadata, browser fixture imports, and Wails binding namespace
together prevents an active fallback path and makes the source identity agree with accepted v2
release tags.

**Alternatives considered**: Keeping the unsuffixed module while publishing v2 would conflict with
Go module semantics. A temporary compatibility module or import alias would create a second active
identity with no consumer requirement and would violate the completed-cutover principle.

## Decision 2: Regenerate contracts without changing application protocol behavior

**Decision**: Change only each protobuf file's `go_package` option to the `/v2` module identity,
then regenerate the checked-in Go, ConnectRPC, and ECMAScript outputs with the existing pinned
tool modules.

**Rationale**: `go_package` is language metadata, while the protobuf package names, messages,
field numbers, services, RPC directions, procedure paths, and serialization behavior remain the
accepted contract. Regeneration is required because descriptor data embedded in both Go and
ECMAScript outputs records the option even when browser import paths and RPC routes do not change.

**Alternatives considered**: Editing generated files would violate reproducibility. Renaming the
protobuf v1 packages to v2 would incorrectly turn an application-module release into a wire and
persistence migration.

## Decision 3: Advance the reviewed protobuf baseline explicitly

**Decision**: Update `proto/schema-revision.txt` and rebuild
`proto/compatibility-baseline.binpb` only after confirming that every schema diff is the intended
`go_package` identity change. Update the reviewed hashes and retain all established breaking-change
fixtures.

**Rationale**: Buf's descriptor image includes file options, so the old baseline cannot represent
the new language-package identity. Explicit review prevents the baseline update from masking an
accidental message, field, enum, package, or service change, while the existing negative fixtures
continue proving those changes are rejected.

**Alternatives considered**: Leaving the old baseline would make the accepted generated identity
and reviewed descriptor state disagree. Replacing the baseline without reviewing the source diff
would weaken the compatibility gate.

## Decision 4: Regenerate the Wails binding namespace atomically

**Decision**: Run the existing clean Wails binding generator so the checked-in service namespace
moves from `github.com/obalunenko/Fallout-Terminal` to
`github.com/obalunenko/Fallout-Terminal/v2`, then update every active Overseer import, browser
fixture import map, binding assertion, and secret-scan allowlist in the same cutover.

**Rationale**: The Wails output path is derived from the Go module identity. Clean generation
removes the old directory, while existing checks retain the one-service, 35-method, six-event, and
no-generic-capability boundaries.

**Alternatives considered**: Retaining both generated namespaces would create a permanent dual
bridge path. Hand-moving the files would bypass the generator and could leave relative imports or
consumer paths inconsistent.

## Decision 5: Enforce the v2 release major in the existing preflight boundary

**Decision**: Keep strict release-tag parsing in `internal/buildtool.ValidateReleaseTag`, require
the parsed major to equal 2, and preserve the existing `cmd/build validate-release-tag` entrypoint
used by release preflight before the five-target matrix begins.

**Rationale**: The dependency-free build tool already owns tag validation and is exercised by
network-free table-driven tests. Adding the major check there preserves the Task/build ownership
model and makes malformed, older-major, and future-major failures occur before packaging.

**Alternatives considered**: Duplicating the major rule in GitHub Actions or Task would create a
second policy owner. Deriving the allowed major dynamically from an arbitrary substring in
`go.mod` would make validation less explicit; root validation should compare the exact declared
module identity.

## Decision 6: Preserve version-1 persistence behavior, not merely version numbers

**Decision**: Keep session and player-configuration documents at version 1 and run their existing
round-trip suites unchanged. Sessions continue preserving compatible recursive unknown fields;
player configurations continue rejecting unsupported unknown fields without rewriting the source
file.

**Rationale**: The module-major transition changes source identity only. Each persistence service
already has an accepted compatibility policy, and broadening or narrowing either policy would be a
separate application behavior change.

**Alternatives considered**: Renumbering JSON documents to version 2 would conflate application and
format versions. Forcing identical unknown-field behavior across the two formats would expand the
feature beyond release preparation and risk silently changing persisted data handling.

## Decision 7: Preserve independent tool modules and historical records

**Decision**: Leave every `tools/*/go.mod` identity and dependency graph unchanged, update only
active release guidance and executable fixtures, and retain completed specifications and rollback
records verbatim as historical evidence.

**Rationale**: Development tools are independently owned modules rather than subpackages of the
application module. Active instructions must teach v2 release tags, while rewriting dated records
would falsify the paths and examples that were valid when their evidence was captured.

**Alternatives considered**: Adding `/v2` to tool-module identities would create unrelated module
migrations. A repository-wide replacement of old version strings would corrupt persistence,
protocol, dependency, Wails-v3, Spec Kit, and historical version references.

## Decision 8: Propagate one canonical application version into every release artifact

~~**Previous decision**: Under the original specification, do not add tag propagation into linker
flags, platform metadata generation, or packaged-version equality checks; defer that work to
BUG-001 because the scope explicitly excluded it.~~

**Decision**: With BUG-001 patched into scope, normalize an accepted tag by removing only its
leading `v`, pass that canonical semantic version as the sole release `VERSION`, inject it into Go,
render Darwin and Windows metadata from it, and verify every package before upload.

**Rationale**: The release tag, Go binary, and platform metadata otherwise have independent version
identities and can disagree while the existing archive-presence gate still passes. One input makes
that disagreement impossible to hide and keeps the existing build tool as the policy owner.

For a prerelease such as `2.0.0-rc.1`, human-readable metadata and the executable report retain the
complete value. Fields restricted to numeric components derive `2.0.0` or `2.0.0.0` according to
their native format; they do not become a second version source. The non-interactive report exits
before Wails composition so this maintainer/build contract creates no Overseer or player feature.

`<executable> --version` is the exact inspection seam: it accepts no additional arguments, writes
only the embedded value plus one newline to standard output, writes nothing to standard error, and
exits successfully before runtime composition. Local builds and packages with no `VERSION` embed
`development`; native numeric fields use `0.0.0` or `0.0.0.0`, and tagged-release inspection rejects
that identity. Checked-in `.tmpl` files are immutable inputs; rendered plist, JSON, and manifest
files live only in target-isolated staging paths.

**Alternatives considered**: Keeping checked-in platform versions would preserve the bug. Relying
only on Go VCS metadata would identify a commit but not the application release. Deriving versions
separately in each target job would allow drift. Adding the version to protobuf runtime status or a
desktop method would unnecessarily widen application contracts.

## Decision 9: Roll back only before publication and fix published releases forward

**Decision**: Use the immutable feature branch point
`3f2b6e584aee4c5279a3d54ae70aa44ee578a21a` as the pre-publication rollback reference. A complete
published release is never replaced or retagged; corrections use a new strict v2 version.

**Rationale**: The atomic source/generated identity cutover cannot safely retain dual module or
binding paths, and create-only release governance makes a published tag an immutable identity.
Pre-publication rollback can restore the prior source and regenerated artifacts together; after
publication, only a forward fix preserves traceability.

**Alternatives considered**: Temporary dual identities violate completed-cutover governance.
Deleting or replacing a complete release violates create-only publication and breaks consumer
traceability.
