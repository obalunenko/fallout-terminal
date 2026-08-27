# Native Approval Notification Contract

## Boundary

This is an internal native Overseer contract between root application composition and the operating-system
notification service supplied by the pinned Wails runtime. It exposes no method, event, binding, RPC, route, or
schema to either frontend. Existing private coordination protobufs and App decision methods remain the only
application-owned structured contracts involved.

## Stable identifiers

| Purpose | Identifier | Rule |
|---|---|---|
| Notification category | `fallout-terminal.command-approval` | Registered once after native service startup. |
| Approve action | `APPROVE` | Maps only to the current record's existing approve decision. |
| Reject action | `REJECT` | Maps only to the current record's existing reject decision and is marked destructive where supported. |
| Notification ID prefix | `fallout-terminal.command-approval:` | Followed by a cryptographically random launch ID, request kind, and the server-owned request ID; never parsed to recover authority. |
| Command kind | `command-execution` | Routes a validated response to `ResolveCommandExecution`. |
| Navigation kind | `terminal-navigation` | Routes a validated response to `ResolveTerminalNavigation`. |

The action titles are `ОДОБРИТЬ` and `ОТКЛОНИТЬ`, matching the existing in-app controls. The notification title is
`ТРЕБУЕТСЯ РЕШЕНИЕ СМОТРИТЕЛЯ`. These strings are presentation values, not authorization inputs.

## Presentation

### Command execution

```text
Title: ТРЕБУЕТСЯ РЕШЕНИЕ СМОТРИТЕЛЯ
Body:  КОМАНДА: <command name or command ID>
       <existing confirmation text>
Actions: ОДОБРИТЬ | ОТКЛОНИТЬ
```

The adapter may omit an empty confirmation line after trimming only surrounding blank space. It must not add
terminal content, result text, player identity, session name, credentials, or opaque callback data.

### Terminal navigation

```text
Title: ТРЕБУЕТСЯ РЕШЕНИЕ СМОТРИТЕЛЯ
Body:  КОМАНДА: <command name or command ID>
       <source terminal name or ID> → <target terminal name or ID>
Actions: ОДОБРИТЬ | ОТКЛОНИТЬ
```

This reuses the exact source, command, and target information already shown by the in-app navigation prompt.
Direction and route depth remain in the in-app view and are not required in the notification.

## Delivery contract

1. A valid current request receives one delivery attempt when notification availability becomes ready.
2. Repeated coordination snapshots with the same kind and request ID do not send or update another notification.
3. A replacement request invalidates and cleans up the previous ID before the new request becomes actionable.
4. Send failure is nonfatal, is logged without private data, and leaves the in-app request unchanged.
5. The default operating-system sound and active interruption level may be used; no custom sound, attachment,
   schedule, text reply, or critical-alert entitlement is introduced.
6. Callback `Data`/`UserInfo` is empty and ignored.

## Response contract

The native callback is accepted only when all of these checks pass:

1. the adapter is not stopped;
2. the native result contains no error;
3. a non-invalidated current record exists;
4. response ID equals the current notification ID, including its current-launch correlation value;
5. response category equals `fallout-terminal.command-approval`;
6. action is exactly `APPROVE` or `REJECT`;
7. no notification decision is already pending for the record.

After validation, the adapter obtains kind and request ID only from its current record. It calls the existing App
method once with the matching approve/reject domain value. App and control validation remain authoritative, so a
simultaneous in-app decision can win safely and cause the native call to return stale.

Default notification activation, dismissals, empty actions, reply actions, malformed payloads, mismatched IDs,
old-process callbacks (including a restored request with the same kind and request ID), repeated callbacks, and
callbacks received after resolution never call App.

## Authorization and lifecycle contract

1. Wails starts the application-owned lifecycle wrapper before the core application lifecycle service.
2. The wrapper starts the native service and registers the action category. Failure disables native delivery but
   returns successful Wails service startup so the application and in-app prompt continue.
3. Authorization is checked after startup without holding application locks. At most one consent request is made
   per launch when authorization is absent.
4. Denial, timeout, revocation, or authorization error disables delivery for the launch and never resolves a
   command.
5. Shutdown marks callbacks stale before calling native shutdown. A late authorization or response result is
   ignored.

## Cleanup contract

- Clear, successful decision, explicit rejection, replacement, broadcast end, terminal switch, and shutdown all
  invalidate action authority before requesting native removal.
- Pending and delivered removal are attempted for the exact notification ID on supported platforms.
- Removal errors are nonfatal. A delivered Windows toast may remain visible because the pinned backend cannot
  remove it; the response contract still makes it inert.
- The adapter never calls remove-all operations, so it cannot delete unrelated application or system
  notifications.

## Verification contract

Tests code against the category, action IDs, ID prefix, title, content rules, and response checks above. A fake
native notifier records startup, authorization, category, send, remove, callback, and shutdown calls; a fake App
target records decisions. The tests must prove one delivery per request and one effective decision under at least
100 repeated, delayed, stale, or concurrent responses.
