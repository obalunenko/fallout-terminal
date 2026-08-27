# Contract: Portable Release Archives

## Exact GitHub Release inventory

A successful tagged release contains exactly these five assets:

| Target | Archive |
|---|---|
| `windows/amd64` | `Fallout-Terminal-windows-amd64.zip` |
| `windows/arm64` | `Fallout-Terminal-windows-arm64.zip` |
| `linux/amd64` | `Fallout-Terminal-linux-amd64.tar.gz` |
| `linux/arm64` | `Fallout-Terminal-linux-arm64.tar.gz` |
| `darwin/arm64` | `Fallout-Terminal-darwin-arm64.zip` |

The GitHub Release contains no raw executable, checksum sidecar, aggregate index, external verification record, DMG, installer, signed/notarized variant, or package-registry duplicate. Local `task package:all` metadata is outside this inventory and must never be passed to GoReleaser.

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

The executable is non-empty. Adjacent resources resolve from the executable location, so launch requires neither a repository checkout nor the build working directory.

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

The executable is non-empty and retains executable mode. GTK4/WebKitGTK 6.0 and Secret Service remain operating-system prerequisites rather than bundled files.

## Darwin archive

```text
Fallout Terminal/
├── artifact-manifest.json
└── Fallout Terminal.app/
    └── Contents/
        ├── Info.plist
        ├── MacOS/
        │   └── Fallout Terminal
        └── Resources/
            ├── icon.icns
            ├── THIRD_PARTY_NOTICES.md
            └── sessions/
                ├── demo.json
                └── demo-players.json
```

The application bundle is produced on `darwin/arm64` and its packaged executable is present and non-empty. The tagged-release path performs no codesign, DMG creation, signing, hardened-runtime validation, notarization, stapling, or Gatekeeper operation.

## Internal archive metadata

`artifact-manifest.json` may remain inside each archive as build metadata produced by the common archive implementation.

| Field | Rule |
|---|---|
| `schemaVersion` | Current archive manifest version. |
| `product` | Exactly `Fallout Terminal`. |
| `sourceRevision` | Full tagged source commit. |
| `target.os` | Exact `windows`, `linux`, or `darwin` target value. |
| `target.arch` | Exact `amd64` or `arm64`; Darwin is `arm64` only. |
| `runtime` | Required native desktop/runtime description. |
| `files` | Sorted archive-relative executable and resource inventory. |

The manifest contains no credentials, user documents, absolute builder paths, mutable environment values, or release-destination metadata. It is not published as a separate asset.

## Tagged-release eligibility

The release job checks only:

1. the expected stable filename;
2. non-zero archive size;
3. expected executable presence; and
4. required-resource presence.

Archive creation may continue enforcing safe normalized paths and atomic target output. Tagged-release eligibility does not require a checksum sidecar, executable architecture inspection, manifest hash verification, native launch, UI/dialog interaction, credential-store access, player/tunnel operation, or signing evidence.

## Runtime boundary

Portable means no native installer or developer toolchain is required. Native prerequisites remain documented:

- Windows 10/11 with WebView2 for `windows/amd64`, and Windows 11 on ARM for `windows/arm64`;
- the project’s GTK4/WebKitGTK 6.0 baseline for `linux/amd64` and `linux/arm64`;
- macOS 13+ on Apple Silicon for `darwin/arm64`.

Secure-store unavailability disables public access with a clear fail-closed status; it does not create a plaintext fallback or prevent local/LAN play.

For this feature, platform support ends at availability of this governed archive contract. Native launch, UI, dialog, player, lifecycle, secure-store, tunnel, and signing checks are optional evidence and are not archive, quality, feature-completion, or tagged-release gates.
