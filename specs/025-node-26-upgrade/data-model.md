# Data Model: Node.js 26 Upgrade

This feature changes repository-owned configuration records only. It introduces no application runtime state, persistence schema, RPC message, or user data.

## Entity: Node Runtime Policy

Represents the one reviewed JavaScript toolchain version accepted by the repository.

### Fields

- **Exact automation version**: `26.8.1`
- **Package engine minimum**: `>=26.8.1`
- **Bundled package-manager version used for regeneration**: `11.19.0`
- **Release provenance**: official Node.js distribution index entry dated 2026-08-26

### Validation Rules

- Automation selects the exact version and does not use `26.x`, `latest`, or `check-latest` resolution.
- Every project-owned package engine uses the same minimum.
- The selected release provides binaries for Linux x64/arm64, macOS arm64, and Windows x64/arm64.
- No active project-owned configuration or instruction retains `20.19.0`.

## Entity: Package Engine Declaration

Represents a project-owned statement of the minimum supported Node.js runtime.

### Owners

- Frontend workspace root: `frontend/package.json`
- Player client workspace: `frontend/client/package.json`
- Overseer workspace: `frontend/overseer/package.json`
- Browser tests: `tests/browser/package.json`

### Relationships

- The frontend root, client, and Overseer declarations are mirrored in their corresponding `packages` records in `frontend/package-lock.json`.
- The browser-test declaration is mirrored in the root `packages` record in `tests/browser/package-lock.json`.
- All declarations derive from the Node Runtime Policy.

### Validation Rules

- All four owners use `>=26.8.1`.
- Manifest and lockfile records agree exactly.
- Third-party package engine ranges remain unchanged.
- Dependency versions, resolved locations, and integrity values remain unchanged unless a separately reviewed compatibility fix is proven necessary.

## Entity: Workflow Toolchain Selection

Represents the JavaScript runtime and official action revisions used by automated quality and release work.

### Owners

- Quality automation: `.github/workflows/wails-cross-platform.yml`
- Portable release automation: `.github/workflows/wails-portable.yml`

### Fields

- **Project runtime**: `NODE_VERSION=26.8.1`
- **Checkout action**: `actions/checkout@v7.0.1`
- **Node setup action**: `actions/setup-node@v7.0.0`
- **Go setup action**: `actions/setup-go@v7.0.0`
- **Artifact upload action**: `actions/upload-artifact@v7.0.1`
- **Artifact download action**: `actions/download-artifact@v8.0.1`
- **Expected action internal runtime**: supported upstream `node24`

### Validation Rules

- Both workflows use the same exact Node Runtime Policy.
- Every occurrence of a listed action uses its selected exact release.
- Existing permissions, triggers, cache inputs, matrix targets, artifact inventory, create-only publication behavior, and job ordering remain unchanged.
- Workflow annotations contain no deprecated-Node.js-20 warning from an upgraded listed action.

## Entity: Maintainer Prerequisite

Represents the active setup requirement communicated to contributors.

### Owner

- `README.md`

### Validation Rules

- The prerequisite identifies Node.js 26.8.1 as the supported starting version.
- It does not imply support for Node.js 20, 22, or 24.
- Historical environment evidence in migration and rollback records is not rewritten.

## Entity: Static Acceptance Contract

Represents repository tests that prevent runtime and workflow pins from drifting apart.

### Owner

- `internal/platform/portable_release_test.go`

### Validation Rules

- The contract checks both workflow runtime pins and every selected action revision.
- The contract checks the four package declarations, the four corresponding lockfile records, and the maintainer prerequisite.
- The contract rejects active remnants of the superseded `20.19.0` policy.
- Existing release identity, five-target inventory, and publication-order assertions remain intact.

## State Transition

1. **Legacy aligned**: active manifests, locks, documentation, and workflows select Node.js 20.19.0; workflows use actions with deprecated Node.js 20 internal runtimes.
2. **Policy declared**: project-owned manifests and active documentation declare Node.js 26.8.1.
3. **Locks synchronized**: only owning engine metadata in both lockfiles changes; dependency graph records remain stable.
4. **Automation aligned**: both workflows select Node.js 26.8.1 and exact supported official action releases.
5. **Governed**: static acceptance checks, clean installs, builds, browser tests, quality automation, and the release matrix prove the aligned policy.

Rollback is atomic: restore the legacy values and action revisions across all owners together. A mixed state is invalid.
