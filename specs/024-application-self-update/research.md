# Phase 0 Research: Application Self-Update

## Decision 1: Use the pinned Wails updater headlessly behind the existing application owner

**Decision**: Configure the repository's pinned Wails v3.0.0-beta.13 `app.Updater` with
`WindowNone`, no periodic interval, and a validated GitHub provider. Call `Check` for startup
discovery and call `DownloadAndInstall` only after the Overseer accepts. Keep the user-facing state
machine, decisions, diagnostics, and lifecycle ownership in an application-owned manager.

**Rationale**: The pinned implementation already provides semantic-version comparison, bounded
GitHub HTTP operations, progress events, secure archive extraction, streaming digest verification,
and staging. Headless use preserves the project's single Overseer experience and lets the feature
enforce two distinct consent boundaries. The official [Wails self-update tutorial](https://v3.wails.io/tutorials/04-self-update-a-wails-app/)
confirms the same `app.Updater` integration and warns that `CheckAndInstall` combines discovery and
preparation, which is too eager for this specification.

**Alternatives considered**: `CheckAndInstall` and the built-in updater window were rejected because
they begin download from the combined flow and do not fit the existing private bridge or global
Overseer UI. A new updater dependency or a from-scratch downloader/verifier was rejected because
the pinned runtime already owns those mechanics.

## Decision 2: Use GitHub release-asset digests without adding release sidecars

**Decision**: Wrap the Wails GitHub provider. After it identifies a candidate, validate the release
inventory and read the selected asset's `sha256:` digest from GitHub's release metadata, decode it,
and populate `updater.Release.Verification`. Reject a release before offering it when the exact five
governed archive assets, their uploaded state, or any SHA-256 digest is missing, malformed,
duplicated, or ambiguous.

**Rationale**: GitHub's official [release asset API](https://docs.github.com/en/rest/releases/assets)
returns a SHA-256 `digest` for each uploaded asset. Passing that value into Wails makes its streaming
verification fail closed without publishing another file. This preserves the constitution's exact
five-archive release inventory and prohibition on checksum sidecars or aggregate indexes while
satisfying the specification's integrity-evidence requirement. The public repository needs no
runtime token or secret.

**Alternatives considered**: Publishing the five already generated `.sha256` files was rejected
because it would create ten release assets and directly violate Constitution 8.1.0. A signed update
manifest was rejected for the same extra-asset/index conflict and because it would introduce private
key custody not required by the approved specification. Trusting `Verification == nil` was rejected
because the Wails verifier treats that as no verification rather than an error.

## Decision 3: Keep the governed archives and add a complete-package replacement helper

**Decision**: Let Wails verify and extract the existing one-root portable archive, then validate its
artifact manifest and copy its replacement unit beside the installed unit on the same volume. Use an
application-owned temporary helper to replace the Windows/Linux portable directory or the macOS
`.app`, relaunch, and restore the backup when replacement or relaunch fails.

**Rationale**: Current Windows/Linux archives contain an executable plus sibling resources, and the
macOS archive has an outer package directory around the `.app`. Wails' built-in restart helper
targets only the running executable on Windows/Linux or the enclosing `.app` on macOS, so invoking
it directly would either replace a file with a directory or discard required resources. A copied
temporary helper can wait for the normal application process to finish, replace the complete
portable unit with same-volume renames, and avoid Windows locks on a helper executing from inside
the unit it must replace.

**Alternatives considered**: Repacking Windows/Linux as bare executables was rejected because it
would strand or freeze governed resources and change the portable product contract. Publishing a
second set of updater-only assets was rejected by the exact five-asset rule. Patching or vendoring
Wails was rejected because the project pins an upstream runtime and should not acquire a private
framework fork.

## Decision 4: Advance `artifact-manifest.json` to schema v2 with canonical version identity

**Decision**: Add the canonical application version to the artifact manifest, increment its schema
to v2, and require the candidate release tag, manifest version, target platform/architecture, and
package inventory to agree before creating the adjacent stage.

**Rationale**: The existing manifest already proves product, source revision, target, runtime, and
per-file identities, but it cannot prove that the extracted replacement unit is the version the
Overseer accepted. A build-tool-owned version field uses the same canonical value already injected
into the executable and native metadata, lets the application validate without executing downloaded
code, and fits the constitution's deterministic release-identity gate.

**Alternatives considered**: Executing the staged binary with `--version` on the user's machine was
rejected because downloaded code should not run before replacement approval and matching-native CI
already performs that executable check. Inferring version only from the release tag was rejected
because it would not detect a structurally valid but wrongly packaged archive.

## Decision 5: Start discovery from an event-first Overseer status handshake

**Decision**: Add `application-update-status` and `GetApplicationUpdateStatus`. The frontend
subscribes first, then calls the getter; the first eligible getter call arms a launch-scoped
`sync.Once` asynchronous check and returns immediately with the current revisioned snapshot.

**Rationale**: `Desktop.Ready` establishes the native runtime but does not prove JavaScript event
listeners are installed. The event-first getter pattern already prevents missed startup state in
`desktop-api.js`, and it ensures the update check begins only when the Overseer can present its
status. The manager remains authoritative when events and snapshots arrive out of order.

**Alternatives considered**: Emitting the first check result at the end of `App.Start` was rejected
because the frontend can miss the event. Adding update state to the existing runtime-status barrier
was rejected because an external update service must not delay or affect normal startup readiness.

## Decision 6: Add one private protobuf update contract and three allowlisted methods

**Decision**: Define update state, failure stage, snapshot, offer decision, restart decision, and
command result in `fallout.terminal.private.v1`. Expose exactly
`GetApplicationUpdateStatus`, `ResolveApplicationUpdateOffer`, and
`ResolveApplicationUpdateRestart` through the existing desktop service, plus the named status
event.

**Rationale**: This follows the repository's private desktop patterns, keeps the Go manager as the
state owner, makes every structured bridge payload schema-governed, and provides explicit decision
points that tests can audit. Attempt identifiers and revisions make repeated clicks and stale
completions rejectable without browser-owned truth.

**Alternatives considered**: Direct Wails updater events were rejected because they expose
framework/provider shapes rather than the application's contract. A generic update command or
dispatcher was rejected by the private-capability boundary. A public ConnectRPC endpoint was
rejected because players must never control application replacement.

## Decision 7: Preserve one-run defer and staged-restart semantics without persistent skip policy

**Decision**: Record deferment and prepared state in the launch-scoped manager. Deferring suppresses
the offered version for the rest of the run; postponing restart keeps the adjacent stage available
during the run. Ordinary shutdown removes unused staged data, while an approved apply handoff keeps
it for the helper. Later launches may offer the release again.

**Rationale**: This matches the specification exactly and avoids introducing a long-lived skip
preference. The only persistent update record is a small, non-sensitive recovery journal used to
report post-shutdown apply or relaunch failure on the restored application's next launch.

**Alternatives considered**: Persisting skipped versions was rejected because “continue” is scoped
to one run. Treating postpone as immediate quit was rejected because restart consent is a separate
decision. Retaining abandoned stages indefinitely was rejected as unnecessary disk and lifecycle
state.

## Decision 8: Keep release publication create-only and discover only complete published releases

**Decision**: Retain repository-pinned GoReleaser as the sole publisher and the exact five archive
names. The runtime provider ignores drafts and refuses any release whose complete five-target asset
and digest inventory is not present.

**Rationale**: The pinned GoReleaser GitHub client creates a draft while artifacts upload and
undrafts only during publication, so partial upload state is not a normal discoverable release.
Runtime completeness validation also fails closed for manually damaged or exceptional partial
releases. Existing create-only checks and manual partial-release deletion remain valid.

**Alternatives considered**: Adding another publisher or a workflow step that mutates an existing
release was rejected by the constitution's sole-publisher and create-only rules. Treating the first
filename match as eligible was rejected because it cannot prove whole-matrix completeness or reject
ambiguity.
