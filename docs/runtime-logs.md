# Runtime logs

Fallout Terminal writes the same structured runtime records to standard error and to a bounded,
per-user retained log. Released packages keep the retained copy, so the Overseer can collect
diagnostics without starting the application from a terminal.

## Opening the logs

Both the startup screen and the main Overseer screen have an **ОТКРЫТЬ ЛОГИ** button. It asks the
operating system to open the fixed log directory and reports the active log file:

- `ЛОГИ ОТКРЫТЫ · <active file>` — the directory was handed to the operating system.
- `<error> · <directory>` — the directory could not be opened; navigate to the shown path manually.
- `… · ЗАПИСЬ ЛОГОВ В ФАЙЛ НЕДОСТУПНА` — retained storage is degraded for this run, so records reach
  only standard error.

The button is available while startup is degraded and does not depend on a session, player
configuration, broadcast, or public access. Player browsers have no access to logs.

## Location

Logs live in a `logs` directory below the application's per-user data directory, never inside the
installed package, so read-only installs and upgrades keep earlier evidence.

| Operating system | Log directory |
|---|---|
| macOS | `~/Library/Application Support/com.vaulttec.fallout-terminal/logs` |
| Windows | `%APPDATA%\com.vaulttec.fallout-terminal\logs` (the Roaming AppData known folder) |
| Linux | `$XDG_CONFIG_HOME/com.vaulttec.fallout-terminal/logs`, or `~/.config/…` when unset |

The directory is created with user-only permissions. A symbolic link or non-directory at that path
is rejected, and logging falls back to standard error.

## Files and retention

Each launch receives a random run identifier, attached to every record as `run_id`. Segment files
are named:

```text
application-<UTC start time>-<run id>-<segment ordinal>.log
```

| Limit | Value |
|---|---|
| Segment size | 5 MiB; a record is never split across segments |
| Retained segments | 8 owned segments, so at most 40 MiB |
| Always kept | the active segment and the newest segment of the immediately previous run |

The oldest owned segments are removed first when a new segment needs room. Files that do not match
the naming pattern are never touched. A single record larger than one segment is written only to
standard error. If cleanup fails, the application stops retaining new records instead of exceeding
the limit, and it reports one degradation warning:

```text
level=WARN run_id=<run id> operation=diagnostics.retention outcome=degraded error_category=storage_unavailable
```

Logging failures never change player authority, command decisions, hacking behavior, or startup
and shutdown ordering.

## What is recorded

Runtime audit records use stable `event` categories: `player.*` for logical connections, role
changes, and disconnections; `command.*` for requests, Overseer decisions, and exceptional
outcomes; `hack.*` for the puzzle lifecycle. Records carry only opaque identifiers, roles,
categories, revisions, and counts. Rejected, failed, declined, stale, superseded, and interrupted
outcomes are logged as warnings; the rest are informational. The full vocabulary is defined in
[the runtime audit contract](../specs/034-overseer-runtime-logs/contracts/runtime-audit.md).

Logs never contain credentials, player or provider secrets, character or display names, terminal
or command text, command output, navigation targets, hacking candidates, passwords, board content,
or raw dependency errors.
