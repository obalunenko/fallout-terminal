---
status: migrated
feature: Public ngrok Access
source: existing implementation
---

# Feature Specification: Public ngrok Access

**Migration status**: Reverse-engineered from the existing implementation on 2026-08-09  
**Scope**: Optional authenticated ngrok tunnel startup, configuration, process lifecycle, temporary credential policy, public-address reporting, and game-master status display

## Purpose

Public ngrok access lets a game master expose the application's existing local HTTP and WebSocket server to remote players through a configured ngrok endpoint. The mode is opt-in and fails closed: a tunnel is not started without valid Basic Auth credentials. The Wails composition root coordinates startup and shutdown, while `internal/tunnel/` owns credential validation, temporary traffic-policy creation, child-process management, and public URL discovery.

## User Scenarios and Acceptance

### User Story 1 — Start protected public access (Priority: P1)

As a game master, I can deliberately start the application in public mode with credentials so that remote players can use the same terminal experience without exposing it anonymously.

**Independent verification**: Configure ngrok and valid Basic Auth credentials, run the single root `wails dev` command with `NGROK_ENABLED=1`, open the displayed public URL anonymously and with credentials, and attempt both the player page and its WebSocket connection.

**Acceptance scenarios**:

1. **Given** ngrok is configured and valid credentials are available, **when** the application starts with `--ngrok` or `NGROK_ENABLED=1`, **then** the local server starts first and an ngrok process targets its actual port.
2. **Given** the tunnel starts successfully, **when** a remote player requests the public HTTP endpoint without credentials, **then** ngrok denies access through the enforced Basic Auth traffic policy.
3. **Given** the tunnel starts successfully, **when** a remote player supplies the configured credentials, **then** the player page and WebSocket upgrade can reach the local application through the same endpoint.
4. **Given** public mode is requested without a complete valid credential, **when** tunnel startup is attempted, **then** no unprotected tunnel is started and the local server remains available.
5. **Given** public mode was not requested, **when** the application starts normally, **then** no ngrok module or process is started and the local player URL remains the displayed address.

---

### User Story 2 — Configure the tunnel without changing project data (Priority: P1)

As a game master, I can configure credentials, domain, and executable location outside session files so that public access works in my environment without storing secrets in campaign data or the repository.

**Independent verification**: Start the tunnel with paired credential variables, the combined credential variable, a custom domain, and a custom binary path; inspect the spawned arguments and project/session files.

**Acceptance scenarios**:

1. **Given** both `NGROK_USERNAME` and `NGROK_PASSWORD` are set, **when** configuration is resolved, **then** they form the Basic Auth credential and take precedence over `NGROK_BASIC_AUTH`.
2. **Given** neither paired variable is set and `NGROK_BASIC_AUTH` contains `username:password`, **when** configuration is resolved, **then** the combined value is used.
3. **Given** only one paired variable is set, **when** configuration is resolved, **then** startup fails instead of falling back to the combined value.
4. **Given** a credential is validated, **when** its username is empty or contains a line break, or its password is outside 8–128 characters or contains a line break, **then** startup fails with a descriptive error.
5. **Given** `NGROK_DOMAIN` or `NGROK_BIN` is set, **when** ngrok starts, **then** the configured endpoint or executable overrides the corresponding default.
6. **Given** no custom domain or executable is configured, **when** ngrok starts, **then** the provider assigns a random public endpoint and the application resolves `ngrok` through the process environment.

---

### User Story 3 — See and open public-access status (Priority: P2)

As a game master, I can distinguish the public player address from the local address and see startup failures so that I know which link to share.

**Independent verification**: Observe the master header during successful tunnel startup and during missing-binary, invalid-credential, and timeout failures, then click the displayed address.

**Acceptance scenarios**:

1. **Given** the local server has started and the tunnel is still pending, **when** the master window loads, **then** the local player URL is initially available.
2. **Given** ngrok reports an HTTPS public URL, **when** the Go runtime receives it, **then** the master header changes to the public URL and retains the local URL in its explanatory tooltip.
3. **Given** tunnel startup fails, **when** the failure reaches the master frontend, **then** the header shows `NGROK: ОШИБКА`, exposes the error and local address in its tooltip, and retains the local URL as its click target.
4. **Given** a displayed local or public URL uses HTTP or HTTPS, **when** the game master clicks it, **then** the bound desktop method may open that validated URL externally.

---

### User Story 4 — Clean up tunnel resources (Priority: P1)

As a game master, I can close the application or encounter a failed startup without leaving tunnel processes or plaintext policy files behind.

**Independent verification**: Inspect the temporary directory and child process after successful startup, credential-policy failure, missing binary, timeout, process exit, and application shutdown.

**Acceptance scenarios**:

1. **Given** valid credentials, **when** tunnel startup begins, **then** they are written into a private temporary traffic-policy file rather than a project or session file.
2. **Given** the traffic-policy file was created successfully, **when** startup succeeds or later fails, **then** the temporary policy directory is removed on a best-effort basis.
3. **Given** tunnel startup fails before success, **when** the failure is handled, **then** the child is terminated if necessary and the active-process reference is cleared.
4. **Given** a tunnel process is active, **when** Wails begins quitting, **then** the application asks the ngrok process group to terminate and clears its active-process reference.
5. **Given** a tunnel process is already active, **when** another start is requested in the same process, **then** the second request is rejected.

## Edge Cases and Observed Behavior

- A missing ngrok executable is translated into an explicit binary-not-found error containing the selected path or command.
- Startup rejects after approximately 20 seconds by default if no recognizable public URL is emitted.
- URL discovery first parses JSON log lines and then falls back to an HTTPS URL on a line containing `started tunnel`.
- Stderr is retained only up to the most recent 4,000 characters for a startup failure.
- A custom endpoint already beginning with `http://` or `https://` is passed through; otherwise `https://` is prepended.
- A successfully started tunnel that later exits clears the process reference and logs a non-zero exit, but the master UI is not notified and may keep a stale public URL.
- Calling stop when no tunnel exists has no effect.
- Temporary-file removal is best effort; cleanup failures are intentionally suppressed.

## Requirements

### Functional Requirements

- **FR-001**: Public mode MUST remain opt-in through `--ngrok`, `NGROK_ENABLED=1`, or the `start:ngrok` npm script.
- **FR-002**: The embedded local server MUST start before the tunnel and its resolved port MUST be supplied to ngrok.
- **FR-003**: Public-mode startup MUST reject missing, partial, or structurally invalid Basic Auth credentials before spawning ngrok.
- **FR-004**: Paired username/password configuration MUST take precedence over the combined credential variable when either paired variable is present.
- **FR-005**: The username MUST be non-empty and MUST NOT contain carriage returns or line feeds.
- **FR-006**: The password MUST contain between 8 and 128 characters and MUST NOT contain carriage returns or line feeds.
- **FR-007**: The ngrok endpoint MUST enforce Basic Auth through a generated traffic policy for incoming requests.
- **FR-008**: Credentials MUST be written only to a mode-`0600` policy file in a newly created operating-system temporary directory.
- **FR-009**: After traffic-policy creation returns successfully, its temporary directory MUST be removed on successful URL discovery and later handled startup-failure paths, on a best-effort basis.
- **FR-010**: Configuration MUST support executable and domain overrides while retaining the implemented defaults.
- **FR-011**: Only one ngrok child process MAY be tracked as active within the application process.
- **FR-012**: The tunnel child MUST receive the local port, endpoint URL, traffic-policy path, stdout logging, and JSON log-format arguments.
- **FR-013**: Startup MUST resolve only after an HTTPS public URL is recognized in ngrok output.
- **FR-014**: Startup MUST reject on invalid credentials, policy preparation failure, missing binary, premature child exit, other spawn errors, or URL-discovery timeout.
- **FR-015**: A startup failure MUST preserve local server operation and MUST be reported to the master frontend without exposing the configured credential.
- **FR-016**: Successful startup MUST report both the public URL and the retained local URL to the master frontend.
- **FR-017**: Wails shutdown MUST request termination of the active ngrok process group.
- **FR-018**: The master UI MUST distinguish local, public, and failed-tunnel address states and MUST open only main-process-validated HTTP(S) addresses.

### Impacted Application Surfaces

- **Wails composition root (`main.go`, `app.go`)**: Detects opt-in mode, starts ngrok after the local server, forwards success/failure information, and stops the child during shutdown.
- **Bound desktop facade (`frontend/src/desktop-api.js`)**: Uses the narrow `server-info` listener and validated `OpenURL` request; it receives neither credentials nor process control.
- **Master UI (`frontend/src/`)**: Displays local, public, and error states and provides the click target.
- **Go tunnel/player services (`internal/tunnel/`, `internal/player/`)**: Own tunnel process policy and the HTTP/WebSocket server; the player protocol has no tunnel-specific messages.
- **Player UI (`client/`)**: Not directly affected — it is served unchanged through ngrok and derives secure WebSocket transport from the HTTPS page.
- **Session data (`sessions/`)**: Not affected — no credential, tunnel setting, URL, or process state is persisted.
- **Packaging/public access**: Affected — packaged runs may opt in with `--ngrok`, and the ngrok executable must be available separately through the configured binary path or environment.

### Configuration Contract

| Input | Meaning | Implemented precedence/default |
|---|---|---|
| `--ngrok` | Enable public mode | Enables when present |
| `NGROK_ENABLED` | Enable public mode | Enables only when exactly `1` |
| `NGROK_USERNAME` + `NGROK_PASSWORD` | Paired Basic Auth credential | Used when either paired value is present; both are required |
| `NGROK_BASIC_AUTH` | Combined `username:password` credential | Used only when neither paired value is present |
| `NGROK_DOMAIN` | Reserved/public endpoint | Empty by default; the provider assigns a random endpoint |
| `NGROK_BIN` | ngrok executable | Defaults to `ngrok` |

Programmatic `startNgrok` options override the corresponding environment-derived credential, binary, domain, and timeout values. They are an internal CommonJS module contract rather than a renderer or player API.

### State and Contract Requirements

- **Session compatibility**: No session schema impact; tunnel state remains process-local.
- **Desktop bridge contract**: Go runtime → master `server-info` carries the ordinary local server object, or adds `{ localUrl, url, tunnel: true }` after success, or adds `{ tunnelError }` after failure. Master → bound `OpenURL` carries a string that is reparsed and restricted to HTTP(S).
- **WebSocket contract**: No application message change. Authentication is applied by ngrok before requests and upgrades reach the existing WebSocket server.
- **Reconnect behavior**: After authenticating at the public endpoint, player reconnection follows the existing live-broadcast behavior. Tunnel process recovery itself is not implemented.
- **HTTP/static contract**: No route change. The same Express application is exposed behind ngrok's Basic Auth ingress policy.

### Security and Privacy Requirements

- Public mode MUST fail closed rather than spawn an unauthenticated endpoint.
- Credentials MUST NOT enter session JSON, renderer messages, UI text, logs emitted by this feature, or repository files.
- Temporary policy material MUST be private to the creating user where filesystem permissions are honored and MUST be cleaned up after ngrok has consumed it.
- The sandboxed renderer MUST receive only status and addresses; it MUST NOT control child processes or read environment credentials.
- External address opening MUST remain restricted to HTTP and HTTPS in the bound Go method.

## Key Entities

- **Tunnel configuration**: Enablement, Basic Auth credential, executable, domain, and startup timeout resolved from options, CLI arguments, defaults, and environment variables.
- **Temporary traffic policy**: Ephemeral JSON containing one enforced Basic Auth action and the configured credential.
- **Tunnel process**: At most one tracked ngrok child process with stdout/stderr and lifecycle listeners.
- **Displayed server information**: The local server address plus optional public URL, tunnel marker, or tunnel error sent to the master frontend.

## Success Criteria

- **SC-001**: With valid configuration, the application displays one HTTPS public player URL while retaining the local URL for diagnostics.
- **SC-002**: Anonymous HTTP access and WebSocket connection attempts through the public endpoint are rejected, while the configured credential permits both.
- **SC-003**: Every invalid or absent credential case prevents an unprotected ngrok process from being started.
- **SC-004**: After successful traffic-policy creation, successful URL discovery and later handled startup failures leave no feature-created traffic-policy directory behind when best-effort removal succeeds.
- **SC-005**: Closing the Wails application while the tunnel is active requests child-process termination.
- **SC-006**: Missing binary, policy creation failure, early exit, and timeout leave the local server usable and produce a visible tunnel-error state.
- **SC-007**: Normal `wails dev` behavior, session files, application WebSocket messages, and player rendering remain unchanged when public mode is not enabled.

## Assumptions

- The game master has separately installed and authenticated an ngrok binary capable of the used `--url` and `--traffic-policy-file` options.
- The selected ngrok domain is available to the configured ngrok account.
- The operating system honors restrictive file modes and permits temporary-directory creation and child-process spawning.
- ngrok applies the supplied HTTP request policy to page requests and WebSocket upgrade requests before forwarding them locally.
- The public URL appears in ngrok output as an HTTPS URL associated with a `started tunnel` message.

## Out of Scope

- Installing ngrok or configuring the account auth token.
- Creating, rotating, recovering, or persisting player credentials.
- Automatic tunnel restart or failover after a successful tunnel later exits.
- In-application controls for starting, stopping, or reconfiguring public mode.
- Changing the local Express/WebSocket protocol, live state, session schema, or player presentation.
- Providing TLS independently of ngrok.

## Identified Gaps

1. No committed automated tests cover credential validation, policy generation, URL parsing, process lifecycle, or status reporting.
2. No repeatable integration test verifies anonymous denial and authenticated success for both HTTP and WebSocket traffic.
3. A post-start tunnel exit is not propagated to the master UI, leaving a potentially stale public link.
4. Startup diagnostics retain stderr but ignore non-URL stdout details, so some ngrok failures may collapse to a generic exit-code message.
5. A custom `http://` endpoint is accepted for the child arguments even though startup recognizes only an HTTPS public URL.
6. The timeout error text always names 20 seconds even when the internal timeout option is overridden.
7. If temporary-directory creation succeeds but writing the policy file fails, `writeTemporaryPolicy()` throws before returning its cleanup callback and may leave the new directory behind.
