# Phase 0 Research: Interactive Approval Notifications

## Decision 1: Wrap the pinned native service in a nonfatal root lifecycle adapter

**Decision**: Use the notifications package already present in the exactly pinned Wails
v3.0.0-beta.13 runtime, but do not register that service directly. Register an application-owned root
lifecycle adapter that starts and stops the native service, registers the action category, and converts every
startup or runtime failure into a sanitized warning and an unavailable notification path.

**Rationale**: Wails aborts application startup when a registered service returns an error from startup. The
native notification service can fail startup for an unsigned or incorrectly bundled macOS app or when Linux
cannot connect to the session bus. Direct registration would violate the requirement that notifications never
break the existing in-app flow. Root composition is an explicitly permitted Wails boundary and can absorb the
optional service failure without weakening the host lifecycle.

**Alternatives considered**: Direct registration was rejected because optional notification availability could
stop the entire application. Moving the adapter into control or domain packages was rejected because those
packages must remain Wails-independent. Adding another notification dependency was rejected because the pinned
runtime already supplies the required cross-platform behavior.

## Decision 2: Observe authoritative coordination publication and reuse App decisions

**Decision**: Add a narrow observer to the App's accepted coordination-publication path. The observer receives a
detached state, derives a notification from the current pending request, and routes an accepted action to the
existing `ResolveCommandExecution` or `ResolveTerminalNavigation` method.

**Rationale**: `publishCoordinationState` is the existing root boundary that rejects regressing revisions, stores
the authoritative private state, and feeds the in-app dialogs. Observing it keeps both surfaces synchronized and
covers player-created requests plus their resolution, replacement, broadcast, and terminal lifecycle changes.
Calling the existing App methods preserves current serialization, validation, durable command effects, event
ordering, safe errors, and the in-app outcome without duplicating control logic.

**Alternatives considered**: Watching frontend events was rejected because it would move native behavior into the
browser and require a second privileged binding. Calling `internal/control.Service` directly was rejected because
it would bypass App-owned publication and durable-session event handling. Adding notification logic inside
control was rejected because notification delivery is not a domain transition.

## Decision 3: Correlate responses with private in-memory state, not callback metadata

**Decision**: Maintain one current notification record containing request kind, server-owned request ID, and
notification ID. A callback is actionable only when its notification ID and category match that current record
and its action is approve or reject. Do not use notification `Data`, titles, bodies, or other callback fields as
decision authority.

**Rationale**: Stable notification IDs provide deduplication and safe update/removal while the server-owned request
ID remains the authoritative correlation value. Old Windows Action Center toasts can activate a later process, and
all platforms may deliver delayed or repeated callbacks. Requiring a current in-memory record makes old-process,
stale, malformed, default-click, and mismatched callbacks inert before they reach App validation; App validation
then supplies a second guard against concurrent in-app decisions.

**Alternatives considered**: Trusting `UserInfo` was rejected because callback metadata is an external platform
payload and need not become an authorization input. Encoding all request context into the notification ID was
rejected because only kind and request identity are needed. Persisting handled IDs was rejected because pending
approval state is process-local and the existing App still rejects stale request IDs.

## Decision 4: Request authorization once per launch without blocking startup

**Decision**: After successful native startup and category registration, check authorization asynchronously. If
authorization is absent, make one operating-system authorization request during that launch; enable delivery only
after approval. Keep the latest current request eligible for delivery if authorization completes while it remains
pending.

**Rationale**: The pinned macOS implementation may wait for user input and has a long authorization timeout, so
performing the request in Wails service startup would delay the Overseer window. Windows and Linux return
authorized immediately. One attempt respects denial and avoids prompt loops, while an asynchronous check keeps
the current in-app approval surface immediately available. The official
[Wails notification guide](https://v3.wails.io/features/notifications/overview/) confirms macOS authorization and
the always-authorized Windows/Linux behavior.

**Alternatives considered**: Blocking startup was rejected because approval notifications are optional. Rechecking
and prompting on every command was rejected as coercive and noisy. Adding a new settings preference was rejected
because notification preferences are outside this feature.

## Decision 5: Treat native removal as advisory and stale validation as mandatory

**Decision**: On request resolution or invalidation, remove pending and delivered notifications where supported,
but always invalidate the in-memory record first. On Windows, where the pinned service cannot remove a delivered
toast, leave any visible copy harmless and reject its later response as stale.

**Rationale**: macOS and Linux can remove delivered notifications, while the pinned Windows backend documents
delivered-removal as a no-op and update-by-ID as redelivery. Portable correctness therefore cannot depend on visual
cleanup. Invalidating local authority before best-effort cleanup guarantees that a stale toast never repeats an
effect or acts on a replacement request.

**Alternatives considered**: Assuming removal succeeds everywhere was rejected by the pinned implementation.
Updating every resolved toast was rejected because Windows would create another notification. Leaving current
records active after resolution was rejected because it would turn a visual platform limitation into an approval
race.

## Decision 6: Verify logic with fakes and report native evidence honestly

**Decision**: Put the portable acceptance burden on deterministic root adapter tests, App observer tests, and race
coverage. Keep matching-host packaged notification journeys as supplemental quality evidence and report any
unavailable platform run as `NOT RUN`.

**Rationale**: Authorization prompts, system notification centers, Windows toast activation, and Linux daemons are
not deterministic CI resources. The constitution explicitly prevents unavailable native UI checks from gating
feature completion or package support. A fake native notifier can exhaustively prove content, lifecycle,
deduplication, failure isolation, and one-decision safety, while governed matching-host packaging proves the code is
part of each supported artifact.

**Alternatives considered**: Requiring live notifications on all five target builds was rejected because it would
contradict the platform-evidence policy and is unavailable from one host. Omitting all native smoke guidance was
rejected because matching-host checks remain valuable when available.
