# Implementation Plan: Interactive Approval Notifications

**Feature**: `028-interactive-approval-notifications` | **Date**: 2026-08-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/028-interactive-approval-notifications/spec.md`

## Summary

Add a best-effort native notification adapter at root composition that observes the same authoritative
Overseer coordination state already published to the in-app approval dialogs. The adapter presents one
interactive notification for each pending command or terminal-navigation request, validates notification
responses against the current request, and delegates approve or reject to the existing trusted App methods.
Notification authorization, delivery, response, and cleanup failures remain isolated from application
startup and from the current in-app approval flow.

## Project Structure

```text
.
├── approval_notifications.go       # Native notification lifecycle, state observer, and response routing
├── approval_notifications_test.go  # Deterministic authorization, deduplication, action, failure, and race tests
├── main.go                         # Compose and bind the notification adapter to the application
├── app.go                          # Narrow observer seam on authoritative coordination publication
├── app_test.go                     # Observer ordering, detachment, and stale-revision coverage
├── wails_host.go                   # Register the best-effort notification lifecycle service
├── wails_host_test.go              # Service registration order and nonfatal lifecycle contracts
└── docs
    └── platform-support.md         # Permission, packaging, and Linux notification-daemon behavior
```

**Structure Decision**: Keep every Wails notification type and operating-system lifecycle concern in a
root adapter, add only a transport-neutral observation seam to the root App, and reuse the existing control
state and decision methods without changing domain packages, protobufs, generated bindings, frontends, or
session persistence.

## Constitution Check

| Principle | Assessment | Evidence |
|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | The exactly pinned Wails notification service stays inside root composition behind a testable adapter; domain, control, session, live, player, and frontend code remain independent of it. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | Existing protobuf-defined private coordination state remains the application contract; notification options and callbacks are third-party platform metadata and injected runtime values, so no parallel application DTO is introduced. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | Player requests continue through the existing public service, while notification actions call the same Go-owned trusted decision methods and cannot mutate browser-local state. |
| IV. Separate Public and Private Capabilities | PASS | The adapter is registered only in the native Overseer host and adds no Wails frontend method, player RPC, HTTP route, event, or remote approval capability. |
| V. Evolve Schemas Safely and Reproducibly | PASS | No protobuf field, enum, package, generated file, or compatibility baseline changes. |
| VI. Preserve Portable Session JSON Version 1 | PASS | Notifications are process-local and do not read or write session or player-configuration documents. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | This is an additive presentation path over the one existing approval authority, not a second protocol or state owner. |
| Dependency Rules | PASS | Root composition uses the already pinned Wails v3.0.0-beta.13 notifications package; no dependency or version changes are needed. |
| Secret and Credential Governance | PASS | Notifications contain only existing command prompt information, omit player/session/credential data, and do not trust callback metadata as authority. |
| Go Development Tool Modules | PASS | No tool or workflow changes are introduced; Task remains the canonical validation and packaging graph. |
| Testing and Quality Gates | PASS | Colocated fake-driven tests cover concurrency and failures, followed by `task vet`, `task test`, `task test:race`, lint, build, and matching-host package checks. |
| Development Workflow and Governance | PASS | The plan identifies the producer, observer, native adapter, trusted decision consumer, lifecycle ownership, failure isolation, verification, and rollback. |

No constitution violations require Complexity Tracking.

## Implementation Approach

1. Add a narrow coordination-state observer dependency to `App`. After accepting a non-stale detached
   state, `publishCoordinationState` passes another detached snapshot to the observer and continues emitting
   the existing named event. The observer must return quickly and must never become an authority for state.
2. Implement a root approval-notification coordinator that reduces either pending command representation
   to one private notification record. It tracks the current request and delivered request IDs, creates one
   notification per new request, and schedules delivery/removal work away from the coordinator and App locks.
3. Wrap the pinned native notification service in a Wails lifecycle service. Native startup, category
   registration, authorization checks, the one consent request per launch, delivery, removal, and shutdown are
   best-effort operations whose failures are sanitized and logged without failing application startup.
4. Register one category with approve and reject actions. Command-execution notifications show the existing
   command name and confirmation text; terminal-navigation notifications show the source, command, and target
   information from the existing navigation dialog. No notification carries player identity, session content,
   result text, credentials, or user-supplied callback authority.
5. Bind the coordinator to `App` before the Wails host starts. On a response, accept only the current
   notification ID, category, request kind, and approve/reject action, then call `ResolveCommandExecution` or
   `ResolveTerminalNavigation` with the recorded server-owned request ID. Default clicks, malformed callbacks,
   old-process toasts, and repeated responses are inert.
6. When authoritative state clears or replaces the request, invalidate its action immediately and ask the
   native service to remove pending and delivered copies. Cleanup is advisory because Windows cannot remove an
   already delivered toast through the pinned service; stale-response validation remains the portable safety
   boundary.
7. Document macOS permission and packaged/signed requirements, Windows cleanup degradation, and the Linux
   notification-daemon dependency. The in-app dialogs remain unchanged and require no generated bindings or
   frontend work.

## Verification Strategy

- Use a deterministic fake native notifier and fake App decision target to test authorized, denied, revoked,
  startup-failure, category-failure, send-failure, response-error, and cleanup-failure behavior without using
  an operating-system notification center.
- Table-test command modes and terminal-navigation prompts for notification identity, visible content,
  approve/reject routing, exact request correlation, default-click rejection, and absence of private fields.
- Exercise repeated snapshots, replacement, broadcast/terminal invalidation, old-process responses, one hundred
  stale or concurrent responses, and simultaneous in-app/native decisions. Assert exactly one effective App
  decision and run these tests under the race detector.
- Extend App tests to prove observers receive detached accepted states only, cannot mutate the stored/event
  projection, and do not receive regressing revisions.
- Extend Wails host tests to prove registration occurs before host run, native startup failure is absorbed, and
  shutdown remains ordered and bounded. Every test-owned resource uses immediate `t.Cleanup`; blocking cleanup
  derives a bounded context from `context.WithoutCancel(t.Context())`.
- Run `task vet`, `task test`, `task test:race`, `task lint`, `task frontend:build`, `task build`, and `task package`
  on the current macOS host. No bindings are expected to change; the binding drift gate must stay clean.
- On available matching hosts, verify one packaged notification approve and reject journey for macOS, Windows,
  and Linux. Native evidence is supplemental under the constitution: unexecuted host checks are reported as
  `NOT RUN`, while deterministic adapter tests and governed package gates remain the completion evidence.

## Cutover and Rollback

The cutover is additive: the existing in-app dialogs remain active before, during, and after notification
enablement, and the notification adapter owns no canonical or durable state. A source rollback removes the
observer dependency, lifecycle registration, adapter, tests, and documentation together; no session, protobuf,
generated binding, player contract, or migration data needs reversal. If a platform backend proves unreliable,
delivery can fail closed to the unchanged in-app flow without a runtime compatibility switch.

## Post-Design Constitution Re-check

| Gate | Result | Final design evidence |
|---|---|---|
| Runtime ownership | PASS | Wails notification types are restricted to one root adapter and its tests; transport-independent packages remain untouched. |
| Contract ownership | PASS | Existing private coordination projections and App decision methods are reused; notification metadata is platform-native and no application schema is duplicated. |
| Authority and privilege | PASS | The current server-owned request ID and existing App validation decide every outcome; no public or browser capability is added. |
| Failure isolation | PASS | Startup, permission, daemon, delivery, callback, and cleanup failures cannot resolve a request or disable the in-app path. |
| Persistence and privacy | PASS | No persistent format changes, and visible notification data is limited to the current in-app prompt. |
| Dependency and workflow ownership | PASS | The already pinned runtime supplies the service; Task and existing tool pins remain unchanged. |
| Verification and rollback | PASS | Fake-driven race coverage proves the portable contract, optional matching-host checks are reported honestly, and rollback removes only additive root integration. |

The final design introduces no constitution violation and requires no Complexity Tracking entry.
