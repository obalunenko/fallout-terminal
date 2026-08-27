# Update Release and Replacement Contract

## Discoverable release

An update candidate comes only from the public `obalunenko/Fallout-Terminal` GitHub Releases API.
It is discoverable when all of these conditions hold:

1. the release is published, not a draft;
2. its tag is a strict canonical v2 SemVer with an optional prerelease suffix and no build metadata;
3. it is strictly newer than the installed version under SemVer precedence;
4. a stable install sees only stable releases, while a prerelease install may see a newer
   prerelease or stable release;
5. its release assets are exactly the five entries below, with no missing, duplicate, empty, or
   extra asset;
6. every asset reports GitHub state `uploaded` and one valid `sha256:<64 hex digits>` digest;
7. the current runtime maps to exactly one of those assets.

| Target | Exact asset |
|---|---|
| `windows/amd64` | `Fallout-Terminal-windows-amd64.zip` |
| `windows/arm64` | `Fallout-Terminal-windows-arm64.zip` |
| `linux/amd64` | `Fallout-Terminal-linux-amd64.tar.gz` |
| `linux/arm64` | `Fallout-Terminal-linux-arm64.tar.gz` |
| `darwin/arm64` | `Fallout-Terminal-darwin-arm64.zip` |

No checksum sidecar, update manifest, raw executable, installer, DMG, signature asset, package-copy,
or aggregate index is added. GitHub's asset metadata supplies the verification digest and Wails
compares it with the bytes streamed after consent.

## Publication

The existing tag preflight, matching-native five-target matrix, archive inspection, flat inventory,
and repository-pinned GoReleaser remain the sole publication path. Each matching-native inspection
also requires artifact-manifest schema v2 and equality between the canonical workflow version,
manifest version, executable `--version`, and applicable native metadata.

GoReleaser creates a GitHub draft while the five artifacts upload and undrafts the release only
after successful upload. Runtime discovery still validates the complete asset/digest inventory and
therefore rejects an exceptional or manually damaged partial release. Existing-release refusal,
create-only behavior, and manual deletion of a partial failed publication remain unchanged.

## Archive and manifest shape

The archive names and installed portable layout remain stable.

### Windows and Linux

```text
Fallout Terminal/
├── Fallout Terminal[.exe]
├── artifact-manifest.json
└── resources/
    ├── THIRD_PARTY_NOTICES.md
    ├── appicon.png
    └── sessions/{demo.json,demo-players.json}
```

The replacement unit is the complete extracted `Fallout Terminal` directory.

### macOS

```text
Fallout Terminal/
├── artifact-manifest.json
└── Fallout Terminal.app/Contents/
    ├── Info.plist
    ├── MacOS/Fallout Terminal
    └── Resources/{THIRD_PARTY_NOTICES.md,icon.icns,sessions/...}
```

The outer root and manifest are validated first; the replacement unit is the nested
`Fallout Terminal.app` bundle.

### Artifact manifest v2

```json
{
  "schemaVersion": 2,
  "product": "Fallout Terminal",
  "version": "2.1.0",
  "sourceRevision": "<full-lowercase-git-sha>",
  "target": { "os": "darwin", "arch": "arm64" },
  "runtime": "WebKit and Apple Keychain",
  "files": [
    { "path": "...", "size": 1, "mode": "0444", "sha256": "<64-hex>" }
  ]
}
```

The existing deterministic ordering, normalized timestamps, exact inventory, regular-file policy,
path traversal rejection, modes, and per-file digests remain in force. Update staging rejects schema
v1, a development version, an accepted-version mismatch, an unsupported target, or an unexpected
replacement-unit shape.

## Preparation and staging

1. Wails downloads the accepted exact asset to its private temporary directory.
2. Wails compares the streamed bytes with the GitHub SHA-256 and extracts the single outer root with
   its traversal, symlink, entry-count, and expanded-size limits.
3. The application validates manifest v2 and the exact target package tree.
4. The application selects the Windows/Linux outer directory or nested macOS `.app` as the
   replacement unit.
5. It copies that unit to a unique sibling of the installed unit, syncs files/directories as
   supported, and validates the staged tree before publishing `ready-to-restart`.
6. Failure removes only attempt-owned temporary/sibling paths and leaves the installed unit and all
   user-owned paths unchanged.

Paths never cross the private desktop contract and are omitted from ordinary logs because they may
contain a local account name.

## Apply and recovery protocol

Restart approval performs this ordered protocol:

1. copy the running executable to an attempt-owned temporary helper path;
2. atomically write recovery state `applying` outside the replacement unit;
3. launch the copied helper with target, staged unit, relative launch path, parent PID, and recovery
   record supplied through private environment variables;
4. request Wails host quit, which runs existing ordered application shutdown;
5. helper waits up to the bounded parent-exit deadline;
6. rename the installed unit to a unique sibling backup;
7. rename the same-volume staged unit into the installed location;
8. relaunch the target executable or macOS `.app`;
9. on replacement or relaunch failure, remove the broken target if necessary, restore the backup,
   record a safe failed stage/action, and relaunch the restored application;
10. on success, record `applied` and remove the backup and attempt-owned remnants best-effort.

The helper never replaces anything until the parent exits. It refuses filesystem roots, empty or
non-sibling target/stage paths, traversal in the relative launch path, mismatched attempt ownership,
or a missing validated unit. It never deletes session, player-configuration, credential,
Application Support, or Documents locations.

## Failure contract

| Failure point | Installed unit | User-owned data | Visible recovery |
|---|---|---|---|
| check/download/verify | unchanged | untouched | Continue locally; retry on a later launch. |
| manifest/package staging | unchanged | untouched | Explain incompatible or unwritable package and retain normal operation. |
| parent fails to exit | unchanged; staged cleanup deferred | untouched | Helper record/log identifies shutdown timeout. |
| target backup fails | unchanged | untouched | Restored app or next launch reports apply failure. |
| promotion fails | backup restored | untouched | Restored app reports recovery result. |
| updated relaunch fails | backup restored and relaunched | untouched | Restored app reports relaunch failure. |
| backup cleanup fails after relaunch | updated unit remains active | untouched | Nonfatal safe diagnostic; cleanup retried best-effort. |
