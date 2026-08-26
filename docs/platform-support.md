# Windows and Linux support

Fallout Terminal is distributed as a portable archive for four native targets. “Portable” means that the archive does not need an installer or a development toolchain; the operating system's desktop and web-view runtimes are still required. Select the archive that matches both the operating system and processor architecture:

| Target | Supported host | Archive |
|---|---|---|
| `windows/amd64` | Windows 10 or Windows 11 on an x64 processor | `Fallout-Terminal-windows-amd64.zip` |
| `windows/arm64` | Windows 11 on an ARM64 processor | `Fallout-Terminal-windows-arm64.zip` |
| `linux/amd64` | An x64 GTK4 desktop, initially Ubuntu 24.04+ or Debian 13+ class distributions | `Fallout-Terminal-linux-amd64.tar.gz` |
| `linux/arm64` | An ARM64 GTK4 desktop, initially Ubuntu 24.04+ or Debian 13+ class distributions | `Fallout-Terminal-linux-arm64.tar.gz` |

Do not select an archive from the operating-system label alone. On Windows, open **Settings → System → About → System type**. On Linux, `uname -m` reports `x86_64` for `amd64` and `aarch64` for `arm64`. An archive built for another architecture will not start.

## Runtime prerequisites

Windows 10 and Windows 11 require the Microsoft Edge **WebView2** runtime matching the host architecture. Current Windows installations commonly include the Evergreen runtime, but it remains an operating-system prerequisite and is not bundled in the ZIP. If Fallout Terminal opens no window or Windows reports a missing WebView2 loader/runtime, install or repair the matching Evergreen WebView2 runtime from Microsoft and retry.

Linux requires **GTK4** and **WebKitGTK 6.0**, including their transitive desktop libraries. Install the runtime packages supplied by the distribution; do not substitute GTK3 or WebKitGTK 4.x. A graphical session and a working display are also required. If launch reports a missing shared object, use `ldd "./Fallout Terminal"` from the extracted application directory to identify the package that is absent.

Protected public access on Linux additionally requires a freedesktop **Secret Service** provider on the user's session D-Bus, such as the provider integrated with the desktop keyring. The collection must be available and unlockable for the signed-in user. This credential service is separate from GTK4 and WebKitGTK 6.0 and is not bundled in the TAR.GZ.

## Verify, extract, and launch

Keep the downloaded archive beside its `.sha256` sidecar. The sidecar names the archive whose digest it covers. If the digest does not match, discard both downloads rather than launching the application.

### Windows

In PowerShell, select the matching ZIP, compare `Get-FileHash -Algorithm SHA256` with its sidecar, and extract it. For example, on Windows amd64:

```powershell
$archive = ".\Fallout-Terminal-windows-amd64.zip"
Get-FileHash -Algorithm SHA256 $archive
Get-Content "$archive.sha256"
Expand-Archive -LiteralPath $archive -DestinationPath ".\Fallout-Terminal"
& ".\Fallout-Terminal\Fallout Terminal\Fallout Terminal.exe"
```

The native executable is `Fallout Terminal.exe`. Repeat the commands with `Fallout-Terminal-windows-arm64.zip` on a Windows 11 ARM64 host. Do not run either executable from inside the ZIP viewer; extract the complete `Fallout Terminal` directory so its adjacent `resources` directory remains available.

### Linux

In a terminal, select the matching TAR.GZ, verify it with `sha256sum`, extract it, and launch the native executable. For example, on Linux amd64:

```sh
sha256sum -c Fallout-Terminal-linux-amd64.tar.gz.sha256
mkdir -p "./Fallout-Terminal"
tar -xzf Fallout-Terminal-linux-amd64.tar.gz -C "./Fallout-Terminal"
cd "./Fallout-Terminal/Fallout Terminal"
"./Fallout Terminal"
```

Use `Fallout-Terminal-linux-arm64.tar.gz` on an ARM64 host. The executable path is `./Fallout Terminal`; keep its `resources` directory beside it. The archive records executable mode `0755`. If a transfer tool removed that mode, restore it only after the checksum succeeds with `chmod 0755 "./Fallout Terminal"`.

## Sessions, settings, and resources

The bundled demos and icon are read-only application resources beneath the extracted application's `resources` directory. User sessions and settings are stored separately and are never written back into the portable archive directory.

| Platform | Default session folder | Non-secret public-access settings |
|---|---|---|
| Windows | The user's Known Documents folder, then `Fallout Terminal\Sessions` | `%APPDATA%\com.vaulttec.fallout-terminal\public-access.json` |
| Linux | The XDG Documents folder, then `Fallout Terminal/Sessions`; fallback `~/Documents/Fallout Terminal/Sessions` | `$XDG_CONFIG_HOME/com.vaulttec.fallout-terminal/public-access.json`; fallback `~/.config/com.vaulttec.fallout-terminal/public-access.json` |

Windows Known Documents and `%APPDATA%` can be redirected by the user, an organization, or cloud-folder policy; the application asks Windows for the active locations. On Linux, desktop/XDG configuration can likewise redirect the documents and configuration roots. Native Open and Save dialogs start from these resolved locations and create the session directory only after a save is confirmed.

`public-access.json` contains non-secret preferences only. Provider tokens and player passwords are never written there, to a session document, or beside the executable.

## Credential storage and network availability

On Windows, secrets are generic credentials in **Windows Credential Manager**, scoped to Fallout Terminal's production public-access service and account names. On Linux, secrets are items in the user's **Secret Service** collection. Replacing and deleting a public-access credential updates the native store rather than a plaintext application file.

If the native store is missing, locked, access-denied, or the unlock prompt is cancelled, Fallout Terminal reports public access as unavailable and does not start or publish a public tunnel. It does not silently reinterpret the failure as a missing password, and it does not create an environment, settings-file, or other unprotected fallback.

The application and its local player listener remain usable when secure storage fails. Local and direct LAN players can continue to connect using the address shown by the application; only provider-backed public access is unavailable. Fix or unlock the native credential service, then retry initialization or saving the credentials.

## устранение неполадок

- **Windows сообщает об отсутствующем WebView2 или окно не открывается.** Установите или восстановите Evergreen WebView2 той же архитектуры, что Windows и выбранный ZIP. Затем снова запустите извлечённый `Fallout Terminal.exe`.
- **Linux сообщает `error while loading shared libraries`.** Запустите `ldd "./Fallout Terminal"`, установите пакеты, предоставляющие отсутствующие библиотеки GTK4 или WebKitGTK 6.0, и повторите запуск в графическом сеансе. Не переходите на GTK3.
- **Linux не может подключиться к дисплею.** Проверьте, что запуск выполняется из графического сеанса и переменная `DISPLAY` или `WAYLAND_DISPLAY` принадлежит этому сеансу. SSH-сеанс без перенаправления дисплея не может открыть нативное окно.
- **Приложение сообщает, что хранилище секретов недоступно или заблокировано.** В Windows откройте Windows Credential Manager под тем же пользователем. В Linux запустите или разблокируйте Secret Service в пользовательском D-Bus-сеансе. После исправления повторите сохранение учётных данных; не переносите секреты в `public-access.json`.
- **Публичный адрес недоступен из-за хранилища секретов.** Это ожидаемый безопасный отказ: публичный туннель не запускается. Локальный режим и прямое подключение по LAN остаются доступны (`локальный/LAN доступ`), пока устраняется проблема с защищённым хранилищем.
- **Не найден демонстрационный сеанс или значок.** Убедитесь, что рядом с исполняемым файлом находится исходный каталог `resources`. Повторно извлеките весь архив в каталог с правом чтения; не копируйте только исполняемый файл.
- **Сеанс не сохраняется.** Проверьте права на фактическую папку Known Documents в Windows или XDG Documents в Linux. Для настроек проверьте `%APPDATA%` либо `$XDG_CONFIG_HOME`/`~/.config`; каталог приложения с ресурсами не должен быть доступен для записи данных пользователя.
- **Система сообщает о неверном формате или архитектуре.** Снова проверьте System type или `uname -m`, выберите точное имя архива из таблицы и проверьте SHA-256 перед запуском.
