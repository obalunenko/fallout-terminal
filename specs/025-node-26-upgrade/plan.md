# Implementation Plan: Node.js 26 Upgrade

**Bugfix**: 2026-08-27 — BUG-001 adds local runtime selection and an early Task preflight.

**Feature**: [spec.md](./spec.md)
**Research**: [research.md](./research.md)
**Configuration Model**: [data-model.md](./data-model.md)

## Summary

Move every active project-owned JavaScript runtime declaration to the exact reviewed Node.js 26.8.1 release while preserving the existing dependency graph and application behavior. Align both quality and portable-release workflows on that runtime, advance their official GitHub Actions to exact releases backed by the supported Node.js 24 action runtime, and extend the existing static workflow acceptance tests so future drift is rejected. Regenerate only owning lockfile metadata and retain all five release targets and create-only publication behavior.

## Project Structure

```text
.
├── .github/workflows/
│   ├── wails-cross-platform.yml        # quality runtime and official action pins
│   └── wails-portable.yml              # five-target release runtime/action pins
├── frontend/
│   ├── package.json                     # workspace engine minimum
│   ├── package-lock.json                # root/client/overseer engine records
│   ├── client/package.json              # player client engine minimum
│   └── overseer/package.json            # Overseer engine minimum
├── internal/platform/
│   └── portable_release_test.go         # static runtime and workflow contracts
├── tests/browser/
│   ├── package.json                     # browser-test engine minimum
│   └── package-lock.json                # browser-test root engine record
└── README.md                            # active maintainer prerequisite
```

**Structure Decision**: Keep version ownership in each tool's existing native manifest or workflow and extend the established Go acceptance-test owner; add no new script, workflow, dependency, or application package.

## Constitution Check

| Principle / Gate | Assessment | Evidence |
|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | Node.js remains build, generation, and browser-test tooling; no Wails composition or application runtime boundary changes. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | Package manifests, lockfiles, and workflows are explicitly tool-native metadata outside protobuf governance. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | No RPC, transport, mutation, or authoritative state changes. |
| IV. Separate Public and Private Capabilities | PASS | No player or Overseer capability surface changes. |
| V. Evolve Schemas Safely and Reproducibly | PASS | No schema or generated-code edits; existing generation checks run under the new exact toolchain. |
| VI. Preserve Portable Session JSON Version 1 | PASS | No session files, persistence adapters, or user data are touched. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | Active Node.js 20.19.0 declarations and deprecated action revisions are removed together; no dual runtime switch remains. |
| Dependency Rules | PASS | Node.js, npm-generated locks, and official actions remain in their native owning graphs with exact reviewed automation pins and no new dependency. |
| Go Development Tool Modules | PASS | The Task graph and isolated Go tool modules remain unchanged; workflow entrypoints continue through the existing pinned Task graph. |
| Testing and Quality Gates | PASS | Existing static workflow contracts are extended; clean frontend installs/builds, browser tests, quality composition, and five-target release evidence are required. |
| Secret and Credential Governance | PASS | No credential, environment-secret, logging, or public-access surface changes. |

No constitutional violation or complexity exception is required.

## Implementation Strategy

1. Extend the existing platform acceptance tests with the approved Node.js and official action pins, package/lock alignment, README prerequisite, and superseded-version absence checks. Confirm the focused contract fails before configuration changes.
2. Update all four project-owned package engine declarations to `>=26.8.1` and the active maintainer prerequisite to Node.js 26.8.1.
3. With Node.js 26.8.1 and npm 11.19.0, regenerate the frontend and browser-test lockfiles without changing dependency versions; review that only project-owned engine metadata changes.
4. Update both workflows to select `NODE_VERSION: 26.8.1` and replace every affected official action reference with the exact researched release.
5. Add `.nvmrc`, validate Node.js 26.8.1 or newer before Task-managed npm commands, and keep active contributor setup aligned with the same minimum.
6. Run focused static contracts first, then clean dependency installation, both frontend builds, browser tests, and the repository quality composition. Inspect the final diff for historical evidence preservation and absence of active Node.js 20.19.0 references.
7. After the change reaches GitHub, collect one successful quality run and one normal governed five-target tagged release run as final cross-runner evidence; do not add a release-only test path or bypass existing publication gates.

## Verification Plan

### Static and repository contracts

- Run the focused `internal/platform` workflow acceptance tests and confirm the exact runtime/action pins, package/lock records, and documentation contract.
- Search active sources for `20.19.0`, `actions/checkout@v4`, `actions/setup-node@v4`, `actions/setup-go@v5`, `actions/upload-artifact@v4`, and `actions/download-artifact@v4`; only immutable historical material may retain an environment version.
- Verify `git diff --check` and confirm dependency versions, integrity hashes, workflow triggers, permissions, caches, target matrix, and release inventory did not drift.

### JavaScript toolchain

- Confirm the executing runtime reports Node.js 26.8.1 and npm 11.19.0.
- Confirm `task frontend:build` performs its Node.js preflight before npm, emits no `EBADENGINE` output on Node.js 26.8.1, and gives one actionable failure on an older runtime.
- Run clean locked installation for `frontend/`, then build the Overseer and player client.
- Run clean locked installation for `tests/browser/`, then execute the complete browser-test suite.

### Repository quality and packaging

- Run the existing non-publishing repository quality composition through the pinned Task graph.
- Confirm a GitHub quality run uses Node.js 26.8.1 and reports no deprecated-Node.js-20 annotation from the upgraded official actions.
- Confirm the governed tagged-release workflow builds and publishes the unchanged five-archive inventory across Windows amd64/arm64, Linux amd64/arm64, and macOS arm64.

## Rollback Plan

Revert the runtime declarations, lockfile owner records, active prerequisite, official action revisions, and their static acceptance expectations as one reviewed change. Do not roll back only a subset: mixed manifest/lock or workflow/action versions violate the single-runtime contract. A failed GitHub release remains governed by the existing create-only recovery procedure; this feature adds no new rollback mechanism.
