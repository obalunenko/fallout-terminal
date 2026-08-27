# Research: Node.js 26 Upgrade

## Decision 1: Pin the exact latest Node.js 26 release

**Decision**: Use Node.js `26.8.1` as the exact automation runtime and `>=26.8.1` as the minimum in every project-owned package engine declaration. Update the active maintainer prerequisite to Node.js 26.8.1 or newer within the explicitly supported major line.

**Rationale**: The official Node.js distribution index lists `v26.8.1`, released 2026-08-26 with npm 11.19.0, as the latest version 26 release at planning time. It publishes binaries for Linux x64/arm64, macOS arm64, and Windows x64/arm64, covering every release runner. Exact automation selection satisfies the constitution's reproducible-pin rule and prevents a later 26.x release from entering CI without review.

**Alternatives considered**:

- `26.x` with `check-latest`: rejected because it is a floating runtime and can change builds without a repository change.
- Node.js 24 LTS: rejected because the approved requirement explicitly targets the latest version 26.
- An exact engine equality constraint: rejected because package engines express the supported minimum while automation supplies the reviewed exact runtime.

**Sources**: [Node.js distribution index](https://nodejs.org/dist/index.json), [Node.js v26.8.1 distribution](https://nodejs.org/dist/v26.8.1/)

## Decision 2: Upgrade every warning-producing official action coherently

**Decision**: Replace the active workflow references with the current exact releases `actions/checkout@v7.0.1`, `actions/setup-node@v7.0.0`, `actions/setup-go@v7.0.0`, `actions/upload-artifact@v7.0.1`, and `actions/download-artifact@v8.0.1`.

**Rationale**: Each selected release declares the supported `node24` internal action runtime, so the workflows no longer invoke the deprecated Node.js 20 internal runtime. Their checked action contracts retain every input used by the repository: checkout ref/fetch depth, language setup and caches, artifact name/path/retention, and artifact pattern/merge/path. Exact release tags also avoid introducing new behavior through floating major references.

**Alternatives considered**:

- Upgrade only `actions/setup-node`: rejected because checkout, Go setup, and artifact actions would continue producing the same deprecation warning.
- Set `ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION`: rejected because it suppresses the migration instead of removing the deprecated runtime.
- Keep floating major tags: rejected because the constitution requires reproducible dependency pins.

**Sources**: [checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1), [setup-node v7.0.0](https://github.com/actions/setup-node/releases/tag/v7.0.0), [setup-go v7.0.0](https://github.com/actions/setup-go/releases/tag/v7.0.0), [upload-artifact v7.0.1](https://github.com/actions/upload-artifact/releases/tag/v7.0.1), [download-artifact v8.0.1](https://github.com/actions/download-artifact/releases/tag/v8.0.1)

## Decision 3: Regenerate only project-owned lockfile metadata

**Decision**: Change the four owning package manifests first, then regenerate the frontend and browser-test lockfiles with Node.js 26.8.1 and its bundled npm while preserving all dependency pins. Review the lockfile diff to ensure only project-owned engine records change.

**Rationale**: The frontend lockfile contains one root record and two workspace records, while the browser-test lockfile contains one root record. Regeneration through npm keeps those records synchronized with their manifests without treating third-party engine ranges as repository runtime declarations.

**Alternatives considered**:

- Manually edit lockfiles: rejected because it bypasses the owning package manager and can leave workspace metadata inconsistent.
- Refresh dependency versions during regeneration: rejected because dependency upgrades are outside the approved scope unless a Node.js 26 compatibility failure proves one necessary.

## Decision 4: Protect the version contract with existing acceptance tests

**Decision**: Extend `internal/platform/portable_release_test.go` to assert the exact Node.js automation pin, exact official action revisions, project-owned engine minimums, synchronized root/workspace lockfile entries, updated maintainer prerequisite, and absence of the superseded `20.19.0` reference from active surfaces.

**Rationale**: This package already owns static quality and portable-release workflow contracts. Keeping the new assertions there prevents drift without introducing another validation script or parallel workflow graph.

**Alternatives considered**:

- Add a new standalone shell checker: rejected because the existing Go acceptance suite already reads and governs these workflow surfaces.
- Rely only on successful CI execution: rejected because execution does not prove that every manifest, lockfile, and active instruction remains aligned.

## Decision 5: Preserve behavior-based and historical version references

**Decision**: Leave the `node_major >= 22` branch in `scripts/proto-generate.sh` and the Node.js 26.7.0 rollback-drill evidence unchanged.

**Rationale**: The script threshold controls a Node.js feature behavior and remains correct for version 26; it is not a supported-runtime declaration. The rollback document is immutable historical evidence and must retain the environment actually used for that drill.

**Alternatives considered**:

- Raise the feature-detection threshold to 26: rejected because it would change behavior for no technical reason and misrepresent why the workaround exists.
- Rewrite the historical environment to 26.8.1: rejected because doing so would falsify recorded evidence.
