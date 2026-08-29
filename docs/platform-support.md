# Platform support

For portable releases, support means **archive availability**: the release provides a governed,
non-empty archive with the expected executable and resources for each listed target. Launching the
native GUI is optional acceptance evidence and is recorded as `NOT RUN` when no suitable host is
available; it does not invalidate an otherwise eligible archive.

| Target | Archive | Minimum host |
|---|---|---|
| `windows/amd64` | `Fallout-Terminal-windows-amd64.zip` | Windows 10 or Windows 11, x64 |
| `windows/arm64` | `Fallout-Terminal-windows-arm64.zip` | Windows 11, ARM64 |
| `linux/amd64` | `Fallout-Terminal-linux-amd64.tar.gz` | x64 Linux desktop |
| `linux/arm64` | `Fallout-Terminal-linux-arm64.tar.gz` | ARM64 Linux desktop |
| `darwin/arm64` | `Fallout-Terminal-darwin-arm64.zip` | macOS 13+, Apple Silicon |

Choose by both OS and CPU. On Windows, check **Settings → System → About → System type**. On Linux,
`uname -m` reports `x86_64` for amd64 and `aarch64` for arm64. The Darwin archive is an unsigned
portable ZIP, not a signed installer.

## Runtime prerequisites

Windows requires Microsoft **WebView2** matching the host architecture. Windows 10 and Windows 11
often include its Evergreen runtime; install or repair it if the application opens no window.
Protected public-access secrets use **Windows Credential Manager**.

Linux requires a graphical session, **GTK4**, and **WebKitGTK 6.0** with their transitive runtime
libraries. Public-access secrets additionally require an unlocked freedesktop **Secret Service**
provider in the signed-in user's D-Bus session. Missing secure storage disables public tunnelling,
but does not disable the локальный player service.

macOS requires macOS 13 or later on Apple Silicon and the system WebKit runtime. Protected secrets
use **Keychain**. Because the portable app is unsigned, Finder may block the first launch; after
confirming the archive came from the expected release, use **System Settings → Privacy & Security →
Open Anyway**. Signed/notarized packages are a separate manual maintainer option.

## Overseer approval notifications

When a player command needs approval, the Overseer keeps showing the existing in-app prompt and
also attempts to show a system notification with **ОДОБРИТЬ** and **ОТКЛОНИТЬ** actions. System
notifications are optional: denied permission, an unavailable notification service, or any native
delivery error leaves the in-app approval flow available and authoritative.

- **macOS:** notification authorization is requested by the system on first use. Reliable native
  delivery requires launching the packaged application bundle; distribution builds should be
  signed for normal end-user notification behavior.
- **Windows:** interactive toast actions are supported. The pinned native backend cannot remove an
  already delivered toast, so a resolved toast may remain visible, but its stale actions are inert.
- **Linux:** the graphical session must provide a freedesktop notification daemon over the user's
  D-Bus session. If it is absent, delivery fails locally and the in-app prompt remains usable.

Matching-host approve and reject checks are manual acceptance evidence. Record them as `PASS` only
when they were actually exercised on that platform; otherwise record `NOT RUN`.

## Extract and launch

Fully extract the chosen archive so resources remain beside the executable:

- Windows: run `Fallout Terminal.exe` from the extracted directory.
- Linux: run `./Fallout Terminal`; if transfer removed its mode, use `chmod 0755 "./Fallout Terminal"`.
- macOS: open the extracted `Fallout Terminal.app` bundle.

Do not run from an archive viewer or copy only the executable. The bundled demos, icon, third-party
notices, `RUNNING.md`, and artifact manifest are part of the governed application package.

## Sessions, settings, and credentials

User data is stored separately from the extracted archive:

| Platform | Sessions | Non-secret settings | Protected credentials |
|---|---|---|---|
| Windows | Known Documents → `Fallout Terminal\Sessions` | `%APPDATA%\com.vaulttec.fallout-terminal` | Windows Credential Manager |
| Linux | XDG Documents; fallback `~/Documents/Fallout Terminal/Sessions` | `$XDG_CONFIG_HOME`; fallback `~/.config/com.vaulttec.fallout-terminal` | Secret Service |
| macOS | Documents → `Fallout Terminal/Sessions` | `~/Library/Application Support/com.vaulttec.fallout-terminal` | Keychain |

Session files and non-secret preferences never contain provider tokens or player passwords. If a
credential store is locked, denied, missing, or cancelled, public access fails closed while local
and LAN sessions remain available.

## устранение неполадок

- **Wrong format or architecture:** recheck System type or `uname -m`, then download the exact name
  from the table. The five governed names are `Fallout-Terminal-windows-amd64.zip`,
  `Fallout-Terminal-windows-arm64.zip`, `Fallout-Terminal-linux-amd64.tar.gz`,
  `Fallout-Terminal-linux-arm64.tar.gz`, and `Fallout-Terminal-darwin-arm64.zip`.
- **Windows window is missing:** install or repair WebView2 for the selected architecture, fully
  extract the ZIP, and rerun `Fallout Terminal.exe`.
- **Linux reports a missing library:** run `ldd "./Fallout Terminal"`, install the distribution's
  GTK4/WebKitGTK 6.0 runtime packages, and retry in a graphical session.
- **macOS blocks launch:** confirm the source, extract the entire unsigned ZIP, and approve
  `Fallout Terminal.app` in Privacy & Security. Do not remove files from the bundle.
- **Secure storage is unavailable:** unlock Windows Credential Manager, Secret Service, or Keychain
  for the signed-in user. Retry credentials afterward; do not create a plaintext fallback.
- **Sessions do not save:** check the resolved Documents directory and the platform settings path.
  Do not save sessions inside the extracted application directory; preserve its resources and keep
  the directory writable when using self-update.
