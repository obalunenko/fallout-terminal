# Persistence and Configuration Contract

`fallout.terminal.persistence.v1` and `fallout.terminal.config.v1` are private semantic packages. Their existence does not authorize public exposure or replace established file/tool schemas.

## Session-v1 known semantics

The portable file remains JSON version 1 with exact field names:

| Entity | Exact JSON fields | Protobuf semantic value |
|---|---|---|
| Session | `version`, `name`, `playerConfig`, `terminals` | `fallout.terminal.persistence.v1.Session` |
| Terminal | `id`, `name`, `hackLevel`, `introText`, `root` | `fallout.terminal.persistence.v1.Terminal` |
| Recursive content node | `id`, `type`, `name`, `children`, `text`, `description` | `fallout.terminal.persistence.v1.ContentNode` with a `oneof` node body |

Rules:

- `version` remains exactly `1`; no version 2 or implicit migration is introduced.
- `playerConfig` remains an optional normalized relative file reference.
- Existing limits remain: 1,000 terminals, 64-node depth, 100,000 nodes per terminal, 256-byte required names/IDs, 64 KiB intro text, and 1 MiB leaf text/description.
- Compatible unknown JSON fields remain attached and round-trip at session, terminal, and recursive node levels.
- The protobuf adapter covers known fields only; unknown JSON bytes stay in the compatibility model and are merged back by the current codec.
- File selection, default location, selected autosave path, ordered save revisions, private permissions, flush, and same-directory atomic rename behavior remain unchanged.
- Protobuf binary and generic ProtoJSON are never written as session files.

## Player-config-v1 known semantics

| Entity | Exact JSON fields | Protobuf semantic value |
|---|---|---|
| Player config | `version`, `name`, `roster` | `fallout.terminal.persistence.v1.PlayerConfig` |
| Roster entry | `id`, `name` | `fallout.terminal.persistence.v1.RosterEntry` |

Rules:

- `version` remains exactly `1`; `roster` remains a required array with at most 1,000 entries.
- IDs are required, at most 256 bytes, and unique; names are nonblank and at most 80 Unicode characters.
- Unknown fields, trailing JSON, unsupported versions, invalid/duplicate identities, missing/null roster, and existing validation failures remain errors.
- Roster/config association publication occurs only after the complete candidate file has been atomically saved.
- Protobuf binary and generic ProtoJSON are never written as player-config files.

## Durable exclusion

Neither file contains recognition handles, logical sessions, physical connections, presence, assignments, controller, broadcast, revision, terminal runtime, navigation, hacking, pending action, replay records, credentials, tunnel policy, or generated RPC values. Process restart restores none of them.

## Serializable application configuration

`fallout.terminal.config.v1.ApplicationConfig` groups private semantic messages for values that may cross composition boundaries or be recorded in tests/status/configuration documentation.

| Group | Serializable fields/defaults |
|---|---|
| Player listener | address, public player port `3690`, same-origin requirement, request body `8192` bytes, uncompressed message `4096` bytes, header/read/idle/shutdown timeouts where configured |
| Delivery | physical-stream queue `32`, bounded detach/shutdown timing |
| Coordination | replay-cache limit `256`, identifier limits, recognition minting policy |
| Browser | reconnect delay three seconds, only recognition-handle storage key semantics |
| Paths | Documents session directory, bundled demo, Application Support metadata, selected session/player-config paths as private values |
| Tunnel | enabled, binary, optional fixed domain, port, local URL, startup timeout, policy parent, credential pair as a private ephemeral message |
| Startup/shutdown | listener-before-desktop order, optional tunnel acquisition, reverse-order idempotent cleanup, owned-process grace period |

Defaults and environment/argument precedence remain implementation behavior in explicit adapters; protobuf definitions do not read environment variables or launch processes.

## Exact tunnel inputs

Environment keys remain:

`NGROK_ENABLED`, `NGROK_BIN`, `NGROK_DOMAIN`, `NGROK_TIMEOUT`, `NGROK_TIMEOUT_MS`, `NGROK_USERNAME`, `NGROK_PASSWORD`, `NGROK_BASIC_AUTH`.

Command-line flags remain:

`--ngrok`, `--ngrok-basic-auth`, `--ngrok-username`, `--ngrok-password`, `--ngrok-bin`, `--ngrok-domain`, `--ngrok-timeout`.

Argument credentials retain precedence over environment credentials; separate username/password retain precedence over combined environment form. Enabled public mode requires a complete valid pair and fails closed. Credentials remain process-local, redacted, and absent from public schemas, runtime status, events, logs, errors, files, and generated public documentation.

## Non-serializable exclusions

The following remain native injected implementation dependencies and are not protobuf fields:

- embedded `fs.FS` values and static asset capabilities;
- callbacks, event sinks, clocks, timers, random sources, word sources, and ID sources;
- service/repository/dialog/browser interfaces and Wails contexts;
- HTTP listeners/servers, stream channels, cancellation functions, mutexes, wait groups, and atomic counters;
- filesystem implementations, process runners/handles, stdout/stderr writers, and cleanup functions.

Tests may use deterministic fakes for these dependencies, but fakes are not serialized configuration contracts.

## Third-party schemas not duplicated

`wails.json`, every `package.json`/lockfile, Buf configuration, GitHub Actions workflows, macOS plist/entitlement files, and ngrok CLI configuration remain governed by their owning third-party schemas. Protobuf configuration messages may describe application values fed into those tools, but never duplicate the tool documents or authorize their exposure. The active ngrok launch uses no generated traffic-policy document.
