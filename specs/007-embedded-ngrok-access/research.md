# Phase 0 Research: Embedded ngrok Public Access

**Date**: 2026-08-15  
**Target profile**: packaged macOS 13+ application on Apple Silicon (`arm64`)  
**Evidence rule**: repository fakes prove deterministic behavior only. They do not prove that a
real ngrok endpoint is externally reachable.

**Bugfix**: 2026-08-15 — ANALYZE-S1/U1 resolves public-ingress classification independently of
remote `Host` preservation and makes the pre-removal rollback reference mechanically reproducible.

**Bugfix**: 2026-08-15 — BUG-001 records the target Darwin `EADDRNOTAVAIL` result for unassigned
`127.0.0.2` and adopts ngrok Agent Endpoint Basic Auth for the personal-game threat model.

**Bugfix**: 2026-08-16 — BUG-003 records the real static-success/`Subscribe`-stall result, rejects
Traffic Policy Basic Auth for the active candidate, and selects an application-owned private
loopback ingress that authenticates without buffering the existing ConnectRPC handler.

**Bugfix**: 2026-08-16 — BUG-003 verification follow-up removes the remaining active endpoint-auth
rationale; retained BUG-001 text is explicitly non-normative history.

**Bugfix**: 2026-08-16 — BUG-003 test-ergonomics follow-up selects a four-name dev/test-only
environment adapter for repeat runs while retaining Keychain as the sole production secret store.

**Bugfix**: 2026-08-16 — BUG-003 second verification reconciliation distinguishes effective
dev/test secret sourcing from removed legacy production env configuration.

**Bugfix**: 2026-08-16 — BUG-003 non-secret username reconciliation permits approved master
settings/snapshot exposure while keeping password confinement unchanged.

**Bugfix**: 2026-08-16 — BUG-003 verification correction distinguishes permitted production
Keychain storage from forbidden application-managed persistence outside Keychain.

## Real SDK acceptance harness decision (2026-08-15)

The real SDK test is strictly opt-in with `FALLOUT_NGROK_INTEGRATION=1`; its account credential and
optional owned reserved domain are read only by the test process from an external harness. The test
never logs either value or raw SDK diagnostics. Without explicit opt-in or a non-empty external
credential it reports `NOT RUN`, and that result is never substituted for deterministic policy,
streaming, or fake lifecycle evidence. When enabled, the harness uses the production private-ingress
path to the existing player service at `127.0.0.1:3690`, probes a random or exact reserved HTTPS
endpoint, checks exact/unknown Host, missing/wrong/correct Basic Auth plus a non-empty incremental
`Subscribe`, and closes the endpoint and ingress with a five-second context.

For repeat development and real-service runs, canonical dev/test root composition may also resolve
the exact FR-056 variables before persisted/Keychain sources. This is the same embedded SDK and
private-ingress path, not a second harness or tunnel runtime. Domain and username may prefill the
master form; token/password remain presence-only, transient scoped inputs. Empty/unset values fall
back normally. The adapter performs no save or auto-start, never prints values, and is not
constructed by packaged production.

## 1. Embedded ngrok runtime

**Decision**: Add the official `golang.ngrok.com/ngrok/v2` module at exactly `v2.1.4` to the root Go
module. As of 2026-08-15, `v2.1.4` is the latest stable tagged release. It was released on
2026-04-27, is MIT licensed, declares Go `1.25.7`, and is compatible with this repository's pinned
Go 1.26 toolchain. Use an explicitly constructed agent rather than the package default so the
authtoken comes only from the scoped effective-secret callback—production `SecretStore` or the
exact FR-056 dev/test override—and never implicitly from `NGROK_AUTHTOKEN`:

1. `ngrok.NewAgent(ngrok.WithAuthtoken(token), ngrok.WithAutoConnect(false), ...)`;
2. `Agent.Connect(startContext)`;
3. start the application-owned private ingress in deny-all mode and retain player credentials
   outside the SDK;
4. `Agent.Forward(startContext, ngrok.WithUpstream(privateIngressURL), endpointOptions...)` without
   Traffic Policy or a custom upstream dialer;
5. read and strictly validate `EndpointForwarder.URL()`;
6. observe `EndpointForwarder.Done()` and provider disconnect events;
7. on every stop path call bounded `CloseWithContext`, then `Agent.Disconnect`, and drop all SDK
   references.

Omit `ngrok.WithURL` for a provider-assigned URL. For a reserved domain, normalize it to a bare DNS
name and pass the exact `https://<domain>` value with `ngrok.WithURL`; the returned URL must match
that host exactly. SDK or account/domain errors are mapped to a small redacted application error
taxonomy; raw SDK errors never cross the desktop bridge.

The SDK endpoint contract has `URL`, `Close`, `CloseWithContext`, `Done`, and `Wait`. Source review
also shows that `Forward` starts forwarding under the supplied context, while explicit close owns
endpoint release. Cancellation is therefore a trigger for cleanup, not a substitute for bounded
`CloseWithContext`. `Done` is only a terminal signal, not an error result, so the adapter must
distinguish intentional close from unexpected completion and use the SDK disconnect event only
for redacted classification.

~~The selected pin's `WithUpstreamDialer` was to bind `127.0.0.2` as a transport-local ingress
discriminator.~~ BUG-001 proved this unassigned source cannot bind on the target macOS host without
privileged interface mutation. The current design uses the ordinary SDK upstream connection and
does not classify provider traffic inside the player server.

The SDK's declared module requirements at this pin include
`github.com/jpillora/backoff v1.0.0`, `go.uber.org/multierr v1.11.0`,
`golang.ngrok.com/muxado/v2 v2.0.1`, `golang.org/x/net v0.50.0`, and
`google.golang.org/protobuf v1.36.11`. The implementation must let Go resolve and record the full
transitive graph in `go.mod`/`go.sum`, then capture `go list -m all`; no `@latest`, floating branch,
runtime download, or separately managed version is allowed.

**Rationale**: This is the provider's supported embedded Agent SDK, removes the PATH and external
binary dependency, exposes a cancellable endpoint lifecycle, and can forward directly to the one
existing player server.

**Alternatives considered**:

- Retain or wrap the ngrok CLI: rejected because packaged operation would still depend on a child
  process, PATH, CLI installation, log parsing, and process-secret concerns.
- Use `ngrok.DefaultAgent`: rejected because its documented default reads `NGROK_AUTHTOKEN` from
  the environment and makes credential ownership implicit.
- Use `Listen` and start another HTTP server on the returned listener: rejected because the feature
  must forward to the existing authoritative server and must not create a second player server.
- Pin an older v1 SDK or an untagged commit: rejected because v2 is the current stable API and an
  exact stable tag gives a reviewable compatibility point.

**Primary sources**:

- [ngrok-go v2.1.4 release](https://github.com/ngrok/ngrok-go/releases/tag/v2.1.4)
- [ngrok-go v2.1.4 API reference](https://pkg.go.dev/golang.ngrok.com/ngrok/v2@v2.1.4)
- [Official Agent SDK documentation](https://ngrok.com/docs/agent-sdks)
- [Official Go SDK quickstart](https://ngrok.com/docs/getting-started/go)
- [ngrok-go repository, Go policy, and MIT license](https://github.com/ngrok/ngrok-go)

## 2. Forwarding and ConnectRPC streaming

**Decision (BUG-003)**: ~~BUG-001 configured one SDK forwarder directly to
`http://127.0.0.1:3690` with ngrok Basic Auth Traffic Policy.~~ Configure exactly one SDK forwarder
to one loopback-only application ingress without Traffic Policy. The ingress authenticates exact
Host/Basic Auth and streams to the unchanged single Connect handler/player service on port 3690; it
is not another Connect service or game-state owner.

Official SDK documentation explicitly supports forwarding to an existing HTTP upstream. At the
selected pin, the forwarder accepts edge connections and serves/copies them to the configured
upstream; it does not require the application to expose a second domain service. That makes the
existing non-empty server stream technically compatible, but documentation and source review are
not acceptance evidence. Integration and real-network browser tests must prove that a snapshot and
later update arrive incrementally before `Subscribe` completes, and that reconnect converges.

**Rationale (BUG-003)**: ~~Public authentication at the ngrok endpoint kept direct local/LAN traffic
unchanged but depended on unproven Traffic Policy streaming.~~ The private ingress preserves the
same topology separation while keeping authentication in application-owned header middleware and
delegating immediately to the unchanged generated Connect handler. The focused real test remains
mandatory evidence that the complete path preserves non-empty incremental delivery.

**Alternatives considered**:

- Serve the Connect handler directly on an SDK listener: rejected because it introduces a second
  HTTP serving path and duplicates player-server lifecycle.
- Proxy to a LAN or wildcard address: rejected because the provider adapter needs one fixed,
  least-privilege loopback upstream.
- Treat SDK documentation or a deterministic fake as sufficient streaming proof: rejected; the
  public acceptance journey needs a real credential-gated run.

## 3. Basic Auth and Traffic Policy

**Decision (BUG-003)**: ~~**BUG-001** configured one ngrok Agent Endpoint Basic Auth policy before
publication.~~ A real endpoint loaded the player HTML but did not deliver the initial `Subscribe`
snapshot, matching the previously recorded streaming-intermediary risk. The SDK now owns endpoint
transport only and receives no player credential or Traffic Policy.

Start one application-owned private loopback ingress in deny-all mode. The SDK forwards only to
that ingress; after the returned public URL is validated, lifecycle atomically installs its exact
Host plus scoped Basic Auth credentials and only then publishes the URL. The ingress authenticates
headers and delegates immediately to a streaming reverse proxy targeting the unchanged
`http://127.0.0.1:3690` player server. It holds no game state, exposes no LAN listener, and is not a
second player service. Stop/failure/reconfigure/shutdown deny the Host before endpoint/ingress close.

~~The player username/password no longer cross the provider SDK boundary. They enter only the scoped
in-memory ingress activation call. Production credentials do not come from environment; the exact
FR-056 dev/test password name is the sole transient exception. No policy file, process argument,
log, event, or reusable status contains them.~~ **BUG-003 username reconciliation**: Neither value
crosses the provider SDK boundary. The ingress receives normalized username from effective
non-secret preferences and password from the scoped effective-secret callback. Username may appear
only in approved non-secret settings/master snapshot/UI surfaces; password never appears in a policy
file, process argument, log, event, reusable status, or application-managed persistence outside
Keychain. Production password comes from Keychain, with only the exact FR-056 dev/test password
override as a transient exception. Direct local/LAN requests bypass the ingress by topology and
remain unauthenticated.

The following pre-BUG-001 source-classification paragraphs are retained as superseded design
history and are non-normative:

~~The application MUST NOT infer direct local/LAN ingress from `Host` alone when a request was
forwarded by the public endpoint. Before implementation, authoritative documentation/source for the
selected pinned runtime must establish that public traffic is rejected at the edge or reaches the
upstream only with the exact endpoint authority, never with `localhost`, loopback, or a LAN authority
supplied by the remote client. A credential-gated real endpoint probe must test the same property
when its prerequisites exist; `NOT RUN` is not substitute proof. A deterministic fake cannot prove
this property. If authoritative provider behavior does not guarantee it, the shared listener design
is unsafe and the plan must be revised to add a trustworthy ingress discriminator before any
endpoint may reach `ready`; Traffic Policy Basic Auth is not an automatic fallback.~~

~~The application MUST classify ingress before it classifies `Host`. SDK-forwarded connections are
identified only by the owned `tcp4` dialer's dedicated loopback source `127.0.0.2`; that ingress
class is always public and can never receive local/LAN bypass, even if the remote request reaches
the upstream with `Host: localhost`, a loopback authority, or a LAN authority. Direct connections
are eligible for unauthenticated local/LAN admission only when their actual transport source is
loopback/private/link-local but not the dedicated public source and their normalized authority
belongs to the concrete local/LAN allow set. Unknown direct or public authorities remain denied.~~

~~This decision does not rely on ngrok preserving, rewriting, or routing a particular `Host`, and it
does not move Basic Auth into Traffic Policy. Deterministic tests MUST prove source binding, target
restriction, no default-dialer fallback, public classification of every `127.0.0.2` connection,
local/LAN continuity, and fail-closed behavior when the dedicated source cannot be bound. The
credential-gated real endpoint probe still records Host overrides and observed upstream source when
available; `NOT RUN` remains honest lack of real-service evidence but no longer leaves the
application security discriminator undefined.~~

**Superseded rationale**: The current repository records a streaming-buffering concern for edge policy, and
the feature has no independent evidence that an ngrok edge authentication action preserves a
non-empty, long-lived ConnectRPC server stream at the selected SDK/service version. Moving auth to
the edge would change the security boundary and could create an untested streaming intermediary.
Header-only application auth has directly testable semantics and preserves the response writer.

**Alternatives considered**:

- ~~ngrok Traffic Policy Basic Auth: BUG-001 accepted it for personal sharing pending one focused
  opt-in streaming/auth check.~~ **BUG-003** rejects it after the available real path loaded static
  content but stalled before the first snapshot.
- Dual edge and application Basic Auth: rejected because it produces two credential authorities and
  can create confusing double challenges.
- Host-only local/public classification: rejected because a forwarded request can carry a local
  authority unless a separate trusted transport discriminator exists.
- ~~A private loopback ingress/proxy remained a future fallback if the focused real Traffic Policy
  test proved streaming incompatibility.~~ **BUG-003** selects that fallback. It is an
  application-owned authentication/streaming adapter, not an authoritative player server, and the
  sole game service remains on port 3690.
- Application lockout/rate limiting: rejected by the approved specification; provider throttling is
  an external condition only.

## 4. macOS Keychain

**Decision**: Implement the production `SecretStore` adapter in `internal/platform` using
`github.com/keybase/go-keychain` exactly at `v0.0.1`. The module is MIT licensed, declares Go 1.21,
supports macOS 10.9+, and calls Apple's CoreFoundation and Security frameworks through cgo; it does
not launch `/usr/bin/security` or any shell command. The repository already builds the target with
`CGO_ENABLED=1`.

Use generic-password items with synchronisation disabled. Production and development bundles use
separate service namespaces so a development build cannot silently consume production credentials:

| Bundle profile | Keychain service |
|---|---|
| packaged production | `com.vaulttec.fallout-terminal.public-access` |
| development | `com.vaulttec.fallout-terminal.dev.public-access` |

Use the fixed accounts `ngrok-authtoken` and `player-basic-auth-password`. Do not set an access
group. Query presence without requesting item data; read data only inside a trusted `Use` callback.
Replacement uses `SecItemUpdate` semantics when present and `SecItemAdd` when absent; deletion uses
`SecItemDelete` and treats not-found as idempotent success. Secret buffers are copied only into the
callback scope and zeroed on return where Go representation permits. SDK-held token and active
policy password references are dropped/zeroed during stop before the endpoint lifecycle completes.

Map `errSecInteractionNotAllowed`, `errSecAuthFailed`, `errSecNotAvailable`, missing/invalid
keychain, user cancellation, and read-only failures to locked, denied, unavailable, absent, or
internal redacted categories. Never format the query, account data, returned bytes, or raw library
error with a secret. A deterministic fake implements presence, replacement, deletion, scoped use,
injected delay/failure, and operation recording.

Apple Keychain access and prompts can be affected by application identity and the user's Keychain
state. The stable service/account key finds the item, while acceptance must separately exercise
ad-hoc development, the packaged identity, locked/denied behavior, and—only when credentials are
available—the Developer ID build. A signing or notarization result is never inferred from a fake or
an ad-hoc package.

**Rationale**: This is a small native wrapper over the supported macOS credential store, has an
exact stable tag, works with the repository's existing cgo package build, and avoids shell/process
access completely.

**Alternatives considered**:

- `github.com/zalando/go-keyring`: rejected for this feature because its Darwin approach may invoke
  the `security` command instead of providing the selected direct Security.framework boundary.
- Store encrypted bytes in Application Support: rejected because the application would also own the
  decryption key and would violate the required OS secure credential store.
- Direct handwritten cgo bindings: rejected because the pinned library already provides reviewed
  `SecItem` mapping; the application still wraps it behind its own narrower interface.
- Share dev and production service names: rejected to prevent development/test runs from consuming
  production secrets.

**Primary sources**:

- [keybase/go-keychain v0.0.1](https://github.com/keybase/go-keychain/releases/tag/v0.0.1)
- [keybase/go-keychain repository](https://github.com/keybase/go-keychain)
- [Apple generic-password item attributes](https://developer.apple.com/documentation/security/ksecclassgenericpassword)
- [Apple Keychain result codes](https://developer.apple.com/documentation/security/security-framework-result-codes)
- [Apple Keychain accessibility guidance](https://developer.apple.com/documentation/security/restricting-keychain-item-accessibility)

## 5. Non-secret settings persistence

**Decision**: Add a dedicated version-1 file at
`~/Library/Application Support/com.vaulttec.fallout-terminal/public-access.json`. It contains only
the enabled preference, optional reserved domain, username, and non-authoritative Keychain presence
hints. The live UI presence state is reconciled through Keychain attribute-only queries and is
tri-state: present, absent, or unknown when Keychain is locked/denied/unavailable.

Use an explicit protobuf-to-JSON adapter rather than ProtoJSON or a handwritten duplicate contract.
Follow the existing session storage durability pattern: create the application directory with
`0700`, write a same-directory `0600` exclusive temporary file, `Sync`, atomically `Rename`, and
remove the temporary file on every failure. Validate version and every field before publication.
On malformed, unsupported, or unreadable content, keep public access disabled, return safe defaults
(`players`, no domain, no active URL), expose a redacted recovery error, and preserve one `0600`
quarantine copy for inspection. No recovery path reads or writes session JSON or player-config JSON.

The enabled preference is presentation data only. Every process launch starts in `disabled` and
requires an explicit UI Start action; the preference never auto-opens a network endpoint.

**Rationale**: The repository already owns the Application Support path and a tested atomic storage
pattern. A separate file protects both version-1 game-data formats and permits fail-safe recovery.

**Alternatives considered**:

- Add fields to session JSON or player-config JSON: rejected by compatibility requirements.
- Put secrets in the settings file: prohibited.
- Trust persisted presence hints as proof: rejected because a user can remove an item directly from
  Keychain and Keychain itself may be unavailable.
- Auto-start when the enabled preference is true: rejected because the approved UX requires an
  explicit Start on every application run.

## 6. URL and reserved-domain behavior

**Decision**: An empty domain omits `WithURL` and accepts the SDK-assigned URL only after strict
validation: HTTPS, non-empty DNS Host, no user information, no query or fragment, and no path beyond
`/`. A configured reserved domain is normalized as a DNS host with no scheme/path/credentials and
requires the returned URL's exact normalized Host to match. There is no hard-coded default domain.

The adapter maps provider codes into redacted categories such as invalid token, account access,
reserved-domain ownership/conflict, unavailable network, timeout, and provider failure. The UI may
show the category and safe corrective text, never the raw token, password, provider diagnostic, or
account metadata.

**Rationale**: Random URLs work for accounts without a reserved domain, while exact comparison
prevents a requested-domain failure from being disguised as success on another URL.

**Alternatives considered**:

- Keep any repository-wide fixed host as a default: rejected because it is not owned by every user
  and contradicts optional-domain behavior.
- Fall back from a requested reserved domain to a random URL: rejected because it hides an
  ownership/availability error and surprises the user.

## 7. Development and test injection

**Decision**: Remove all application production parsing for `NGROK_BIN`, `NGROK_ENABLED`,
`NGROK_DOMAIN`, `NGROK_USERNAME`, `NGROK_PASSWORD`, `NGROK_BASIC_AUTH`, timeout environment
variables, and their command-line equivalents. Retain one constructor-level development/test
override that supplies settings and scoped temporary credentials to the same `TunnelService` and
official SDK adapter. It is not registered as a Wails operation, is not compiled into packaged UX,
cannot select a process runner, and cannot choose a second provider implementation.

Canonical dev/test root composition may read only `FALLOUT_NGROK_AUTHTOKEN`,
`FALLOUT_NGROK_RESERVED_DOMAIN`, `FALLOUT_PUBLIC_TEST_USERNAME`, and
`FALLOUT_PUBLIC_TEST_PASSWORD`. Non-empty values override the matching source for that process;
domain/username may prefill while secrets remain presence-only and scoped. External real-network
automation uses the same names and embedded path. Packaged production does not construct the
adapter. Missing credentials or connectivity produces an explicit `NOT RUN` record except for the
blocking BUG-003 closure rerun.

**Rationale**: Tests retain deterministic and credential-gated seams without preserving a hidden
production mode or process fallback.

**Alternatives considered**:

- Preserve `NGROK_BIN` or external-process overrides: rejected because they leave a second runtime.
- Remove all injection: rejected because deterministic lifecycle/race tests and opt-in real-service
  tests need controlled inputs.
- Expose an environment/argument switch in the packaged production composition: rejected because
  packaged users must use only the UI and production secrets must not depend on process metadata.

## 8. Reproducibility, packaging, and release impact

**Decision**: Both direct runtime modules are exactly pinned in root `go.mod`; all transitive
versions and checksums are committed in `go.mod`/`go.sum`. Preserve the canonical
`go run ./cmd/build dev|build|package` graph and its protobuf → player → Wails bindings → master →
native/package order. Do not move dependency ownership into Make, scripts, or CI.

Before deleting the CLI runtime, record and review:

- `go list -m all` and `go mod graph` with no unreviewed upgrade to existing pinned Wails,
  ConnectRPC, or protobuf versions;
- MIT licenses for ngrok-go and go-keychain plus licenses for every newly selected transitive module;
- clean `go mod tidy`, vulnerability/license review, generated protobuf/binding drift, and the
  repository's two-build reproducibility check;
- pre/post stripped `darwin/arm64` executable and `.app` sizes, with the measured delta reported
  rather than guessed;
- arm64/minimum-macOS linkage, CoreFoundation/Security framework linkage, entitlements, ad-hoc
  signing, bundle hash, and packaged offline double-click launch;
- absence of a bundled ngrok executable, CLI lookup, runtime download, or PATH dependency;
- conditional Developer ID, notarization, stapling, DMG, Gatekeeper, provider-plan, and real-ngrok
  results as `PASS`, `FAIL`, or `NOT RUN` according to actual prerequisites.

Offline launch means the packaged master and local/LAN player experience start without contacting
ngrok. Public Start correctly reaches a redacted failure when offline; it is not expected to create
an endpoint without network connectivity.

**Rationale**: The embedded SDK and native Keychain adapter change the module graph, binary, and
signing surface even though no external executable is bundled. Existing gates can measure those
effects without weakening reproducibility or claiming unavailable release evidence.

**Alternatives considered**:

- Vendor or download the SDK at runtime: rejected because it adds a second dependency mechanism and
  breaks reproducible/offline packaging.
- Claim a fixed binary-size impact during planning: rejected; only the final pinned implementation
  build is meaningful evidence.
- Make real ngrok or Developer ID credentials mandatory in ordinary CI: rejected because those are
  user/external prerequisites and absence must remain `NOT RUN`.

## 9. Brownfield cutover conclusion

~~The present runtime is not a runner swap. It is startup-static, process-owned, and fail-open for
an unknown syntactically valid external Host. The implementation therefore needs a restartable,
generation-aware manager and a dynamic player-boundary policy before deleting the CLI path.~~
**BUG-001**: The present runtime remains startup-static and process-owned. The cutover needs a
restartable generation-aware embedded SDK manager, policy-protected endpoint construction, and URL
withdrawal/endpoint-close sequencing before deleting the CLI path. **BUG-003** supersedes endpoint
policy construction with deny-all private ingress acquisition, exact-Host/auth activation before
publication, and deny-before-endpoint-close sequencing.

The cutover order is:

1. introduce protobuf contracts, stores, Keychain/SDK adapters, ~~policy~~ **BUG-001** ephemeral
   endpoint-policy input, state machine, and fakes;
2. prove protected-endpoint publication/withdrawal order, streaming, local fallback, bounded cleanup, and UI
   operations through the embedded path;
3. redirect composition and packaged UX exclusively to the embedded path;
4. ~~delete process runner/guardian, log URL parser, CLI/env configuration and secrets, hard-coded
   domain, process-specific tests, and active CLI documentation;~~ **BUG-003 reconciliation** delete
   process runner/guardian, log URL parser, legacy production CLI/env configuration and secrets,
   hard-coded domain, process-specific tests, and active CLI documentation while retaining only the
   exact FR-056 dev/test adapter;
5. retain their security and lifecycle guarantees in provider-neutral tests and reject any second
   production tunnel mechanism.

Temporary coexistence is owned by root composition and `internal/tunnel` only for the duration of
feature 007. It expires immediately after the embedded-only security, package, reproducibility, and
rollback gates pass; deletion must occur in the same feature. Before deletion, record an immutable
pre-removal Git tree/commit reference and package digest in feature 007 evidence and perform a clean
rollback/rebuild drill. The reference is recovery evidence only: it must not introduce or preserve
a selectable dual-runtime production switch.
