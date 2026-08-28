# Запуск portable-релиза Fallout Terminal

Эта инструкция относится к готовым архивам со страницы
[GitHub Releases](https://github.com/obalunenko/Fallout-Terminal/releases). Скачивайте архив,
который одновременно соответствует операционной системе и архитектуре компьютера.

| Система | Архитектура компьютера | Архив |
|---|---|---|
| Windows 10/11 | x64 | `Fallout-Terminal-windows-amd64.zip` |
| Windows 11 | ARM64 | `Fallout-Terminal-windows-arm64.zip` |
| Linux | x86_64 | `Fallout-Terminal-linux-amd64.tar.gz` |
| Linux | aarch64/ARM64 | `Fallout-Terminal-linux-arm64.tar.gz` |
| macOS 13+ | Apple Silicon | `Fallout-Terminal-darwin-arm64.zip` |

Полностью распакуйте архив перед запуском. Не запускайте приложение из окна архиватора и не
переносите executable или `.app` отдельно от остальных файлов каталога `Fallout Terminal`.
Для автоматического обновления каталог приложения должен быть доступен текущему пользователю для
записи.

## Windows

1. Откройте ZIP в Проводнике и выберите **Извлечь всё**.
2. Откройте распакованный каталог `Fallout Terminal`.
3. Запустите `Fallout Terminal.exe`.

То же самое из PowerShell после перехода в распакованный каталог:

```powershell
& '.\Fallout Terminal.exe'
```

Windows требует Microsoft WebView2 той же архитектуры, что и система. Windows 10 и 11 обычно уже
содержат Evergreen Runtime. Если процесс запускается, но окно не появляется, установите или
восстановите WebView2 и повторите запуск. Если Windows показывает предупреждение для неподписанного
portable-приложения, продолжайте только после проверки, что архив скачан из официального релиза.

## Linux

Распакуйте архив, перейдите в каталог приложения и запустите бинарник:

```bash
tar -xzf Fallout-Terminal-linux-amd64.tar.gz
cd 'Fallout Terminal'
./Fallout\ Terminal
```

Для ARM64 замените имя архива на `Fallout-Terminal-linux-arm64.tar.gz`. Если после переноса файла
утратился executable bit, восстановите его:

```bash
chmod 0755 './Fallout Terminal'
./Fallout\ Terminal
```

Требуется графическая сессия, GTK4 и WebKitGTK 6.0. Для защищённых настроек публичного доступа также
нужен разблокированный Secret Service в пользовательской D-Bus-сессии. Отсутствие Secret Service не
мешает локальному player-серверу.

## macOS

Архив предназначен для macOS 13 или новее на Apple Silicon. Распакуйте его в Finder или командой:

```bash
ditto -x -k Fallout-Terminal-darwin-arm64.zip .
open './Fallout Terminal/Fallout Terminal.app'
```

Portable-приложение не подписано и не нотарифицировано. Если Gatekeeper блокирует первый запуск,
сначала убедитесь, что архив получен из официального релиза, затем откройте **System Settings →
Privacy & Security** и выберите **Open Anyway**.

Если кнопка не появилась, можно удалить карантин только у проверенного приложения:

```bash
xattr -dr com.apple.quarantine './Fallout Terminal/Fallout Terminal.app'
open './Fallout Terminal/Fallout Terminal.app'
```

Не отключайте Gatekeeper глобально и не применяйте эту команду к приложениям из недоверенных
источников.

## Проверка версии

Версию можно проверить без запуска окна и внутренних сервисов:

```text
# Windows PowerShell, из каталога Fallout Terminal
& '.\Fallout Terminal.exe' --version

# Linux, из каталога Fallout Terminal
./Fallout\ Terminal --version

# macOS, из каталога Fallout Terminal
'./Fallout Terminal.app/Contents/MacOS/Fallout Terminal' --version
```

Официальный пакет выводит версию релиза без ведущей `v`, например `2.0.0`. Значение `development`
означает локальную, а не опубликованную release-сборку.

## После запуска

Окно смотрителя запускает встроенный player-сервер. Локальный адрес обычно
`http://127.0.0.1:3690`, а адрес для устройств в локальной сети отображается в приложении. Интернет
для локального режима не требуется.

Сессии, настройки и защищённые credentials хранятся вне распакованного каталога приложения и не
удаляются при его обновлении. Дополнительные требования и устранение неполадок описаны в
[руководстве по платформам](https://github.com/obalunenko/Fallout-Terminal/blob/develop/docs/platform-support.md).
