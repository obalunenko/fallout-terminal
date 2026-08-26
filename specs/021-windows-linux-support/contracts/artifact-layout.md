# Contract: Portable Artifact Layout

## Stable artifact names

| Target | Archive |
|---|---|
| `windows/arm64` | `Fallout-Terminal-windows-arm64.zip` |
| `windows/amd64` | `Fallout-Terminal-windows-amd64.zip` |
| `linux/arm64` | `Fallout-Terminal-linux-arm64.tar.gz` |
| `linux/amd64` | `Fallout-Terminal-linux-amd64.tar.gz` |

Each completed archive has a sibling `<archive>.sha256` sidecar using the lowercase hexadecimal SHA-256 of the archive. Sidecars are verification evidence and do not count as additional runnable archives.

The `darwin/arm64` tag-release artifact is `Fallout-Terminal-arm64.dmg` with sibling
`Fallout-Terminal-arm64.dmg.sha256`. It is an unsigned native macOS distribution whose release
eligibility is based only on exact SHA-256 verification rather than signing or notarization. A successful tagged release
contains this DMG and sidecar together with the four portable archives, four portable sidecars, and
`aggregate-index.json`; all inputs resolve to the same tag SHA.

## Local aggregate runnable payloads

A successful local `task package:all [OUTPUT=<directory>]` keeps the four archives, four checksum
sidecars, and `aggregate-index.json` at the root of `OUTPUT` and additionally exposes these exact
runnable paths:

| Target | Executable | Required adjacent tree |
|---|---|---|
| `darwin/arm64` | `bin/darwin-arm64/Fallout Terminal.app` | Complete native `.app/Contents/` bundle with executable, resources, metadata, and ad-hoc signature. |
| `windows/amd64` | `bin/windows-amd64/Fallout Terminal.exe` | `bin/windows-amd64/resources/` |
| `windows/arm64` | `bin/windows-arm64/Fallout Terminal.exe` | `bin/windows-arm64/resources/` |
| `linux/amd64` | `bin/linux-amd64/Fallout Terminal` | `bin/linux-amd64/resources/` |
| `linux/arm64` | `bin/linux-arm64/Fallout Terminal` | `bin/linux-arm64/resources/` |

Each target payload contains exactly the regular files required by its archive after removing the
single `Fallout Terminal/` archive root. Every payload file MUST match its corresponding verified
archive entry byte-for-byte, including the executable, icon, notices, and both demo session files.
Unexpected files, links, special files, missing resources, architecture mismatch, or a hash mismatch
invalidates the complete local matrix. The Darwin bundle is copied from the canonical native package
plan and must contain only directories and regular files while preserving modes and signature
metadata. Payload directories are developer conveniences and are not additional portable release
artifacts; Docker-built Windows/Linux payloads are not native launch evidence.

## Windows archive

```text
Fallout Terminal/
├── Fallout Terminal.exe
├── artifact-manifest.json
└── resources/
    ├── appicon.png
    ├── THIRD_PARTY_NOTICES.md
    └── sessions/
        ├── demo.json
        └── demo-players.json
```

- `Fallout Terminal.exe` is a Windows GUI PE executable whose machine field matches `arm64` or `amd64`.
- PE resources contain the product name, version/source identity, application manifest, and icon generated from repository-owned inputs by the pinned Wails tool.
- The executable resolves `resources` relative to its own directory, not the launch working directory.

## Linux archive

```text
Fallout Terminal/
├── Fallout Terminal
├── artifact-manifest.json
└── resources/
    ├── appicon.png
    ├── THIRD_PARTY_NOTICES.md
    └── sessions/
        ├── demo.json
        └── demo-players.json
```

- `Fallout Terminal` is an ELF executable whose machine field matches `arm64` or `amd64` and whose archive mode is executable by the owner and group/world according to the repository package policy.
- It uses the default pinned Wails GTK4/WebKitGTK 6.0 runtime path and declares a stable program/product name and icon.
- It resolves `resources` relative to its own directory, not the launch working directory.

## `artifact-manifest.json`

The manifest is deterministic build metadata with these required fields:

| Field | Meaning |
|---|---|
| `schemaVersion` | Manifest format version, initially `1`. |
| `product` | Exactly `Fallout Terminal`. |
| `sourceRevision` | Full source commit SHA. |
| `target.os` | Exactly `windows` or `linux`. |
| `target.arch` | Exactly `arm64` or `amd64`. |
| `runtime` | Required native web view/desktop/credential runtime description. |
| `files` | Sorted relative path, size, normalized mode, and SHA-256 for every other regular archive file. |

The manifest does not contain credentials, environment values, absolute builder paths, timestamps that defeat reproducibility, or its own hash. The external sidecar hashes the completed archive.

## Inventory and safety rules

- Archive entries are relative, slash-normalized, sorted, and rooted under exactly one `Fallout Terminal/` directory.
- Absolute paths, drive paths, `..`, duplicate normalized names, symlinks, hard links, devices, sockets, and named pipes are forbidden.
- The archive contains no source checkout, compiler, frontend tooling, provider executable, user session documents, private settings, or secrets.
- Both bundled demo files, the icon, third-party notices, executable, and manifest are mandatory.
- File content, order, timestamps, owners, and modes are normalized so repeated builds from the same inputs can be compared reproducibly.
- An archive is ineligible if its filename, manifest target, executable format/machine, inventory, metadata, or checksum disagree.

## Runtime boundary

Portable means no native installer and no developer toolchain is required. It does not mean the operating-system web view and desktop libraries are statically bundled:

- Windows requires a matching supported Windows 10/11 environment and WebView2 runtime.
- Linux requires a supported GTK4/WebKitGTK 6.0 environment (initially Ubuntu 24.04+ or Debian 13+ class distributions) and a Secret Service implementation for public-access credentials.
- Missing secure storage disables public access with a clear fail-closed status; it does not prevent local/LAN operation or create an unprotected credential fallback.
