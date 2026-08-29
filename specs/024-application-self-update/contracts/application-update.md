# Private Application Update Contract

## Boundary

This contract is Overseer-only. It is defined in
`fallout.terminal.private.v1`, adapted to native Wails DTOs at the root boundary, and exposed only
through the existing registered desktop service and one named event. No message, method, or event is
added to the public player service or browser client.

## Protobuf identifiers

The new source file is `proto/fallout/terminal/private/v1/update.proto` with the existing package and
Go package option.

### Enums

- `ApplicationUpdateState`
  - `APPLICATION_UPDATE_STATE_UNSPECIFIED`
  - `APPLICATION_UPDATE_STATE_DISABLED`
  - `APPLICATION_UPDATE_STATE_IDLE`
  - `APPLICATION_UPDATE_STATE_CHECKING`
  - `APPLICATION_UPDATE_STATE_CURRENT`
  - `APPLICATION_UPDATE_STATE_AVAILABLE`
  - `APPLICATION_UPDATE_STATE_DEFERRED`
  - `APPLICATION_UPDATE_STATE_DOWNLOADING`
  - `APPLICATION_UPDATE_STATE_VERIFYING`
  - `APPLICATION_UPDATE_STATE_STAGING`
  - `APPLICATION_UPDATE_STATE_READY_TO_RESTART`
  - `APPLICATION_UPDATE_STATE_APPLYING`
  - `APPLICATION_UPDATE_STATE_FAILED`
- `ApplicationUpdateFailureStage`
  - `APPLICATION_UPDATE_FAILURE_STAGE_UNSPECIFIED`
  - `APPLICATION_UPDATE_FAILURE_STAGE_CHECK`
  - `APPLICATION_UPDATE_FAILURE_STAGE_DOWNLOAD`
  - `APPLICATION_UPDATE_FAILURE_STAGE_VERIFY`
  - `APPLICATION_UPDATE_FAILURE_STAGE_STAGE`
  - `APPLICATION_UPDATE_FAILURE_STAGE_APPLY`
  - `APPLICATION_UPDATE_FAILURE_STAGE_RELAUNCH`
  - `APPLICATION_UPDATE_FAILURE_STAGE_RECOVERY`
- `ApplicationUpdateOfferDecision`
  - `APPLICATION_UPDATE_OFFER_DECISION_UNSPECIFIED`
  - `APPLICATION_UPDATE_OFFER_DECISION_ACCEPT`
  - `APPLICATION_UPDATE_OFFER_DECISION_DEFER`
- `ApplicationUpdateRestartDecision`
  - `APPLICATION_UPDATE_RESTART_DECISION_UNSPECIFIED`
  - `APPLICATION_UPDATE_RESTART_DECISION_RESTART`
  - `APPLICATION_UPDATE_RESTART_DECISION_POSTPONE`

### Messages

```proto
message ApplicationUpdateSnapshot {
  uint64 revision = 1;
  optional string attempt_id = 2;
  ApplicationUpdateState state = 3;
  string installed_version = 4;
  optional string available_version = 5;
  optional string release_notes = 6;
  uint64 bytes_downloaded = 7;
  optional uint64 download_size = 8;
  ApplicationUpdateFailureStage failed_stage = 9;
  optional string error_message = 10;
  optional string recovery_action = 11;
}

message ApplicationUpdateStatusEvent {
  ApplicationUpdateSnapshot snapshot = 1;
}

message ResolveApplicationUpdateOfferRequest {
  string attempt_id = 1;
  ApplicationUpdateOfferDecision decision = 2;
}

message ResolveApplicationUpdateRestartRequest {
  string attempt_id = 1;
  ApplicationUpdateRestartDecision decision = 2;
}

message ApplicationUpdateCommandResult {
  bool ok = 1;
  optional string error = 2;
  ApplicationUpdateSnapshot snapshot = 3;
}
```

No field contains a provider URL, asset identifier, digest, signature, local path, helper variable,
credential, authorization value, raw response, or user document content.

## Wails desktop surface

| Wails method | Input | Output | Semantics |
|---|---|---|---|
| `GetApplicationUpdateStatus` | none | native `ApplicationUpdateSnapshot` | Returns immediately. The first eligible call after event subscription starts the one launch check asynchronously. |
| `ResolveApplicationUpdateOffer` | `ApplicationUpdateOfferDecisionPayload { attemptId, decision }` | native `ApplicationUpdateCommandResult` | Accepts only `accept` or `defer` for the current available attempt. Download begins only for `accept`. |
| `ResolveApplicationUpdateRestart` | `ApplicationUpdateRestartDecisionPayload { attemptId, decision }` | native `ApplicationUpdateCommandResult` | Accepts only `restart` or `postpone` for the current ready attempt. Shutdown begins only for `restart`. |

The native bridge uses lower-camel JSON field names and string projections:

- state: `disabled`, `idle`, `checking`, `current`, `available`, `deferred`, `downloading`,
  `verifying`, `staging`, `ready-to-restart`, `applying`, `failed`
- failed stage: `check`, `download`, `verify`, `stage`, `apply`, `relaunch`, `recovery`
- offer decision: `accept`, `defer`
- restart decision: `restart`, `postpone`

The methods are thin forwards on the already registered `desktopService`; the root `App` itself is
never registered.

## Named event

`application-update-status` carries one native `ApplicationUpdateSnapshot`. It is registered as a
typed Wails event and emitted after every externally visible revision. Progress emission may be
rate-limited, but emitted byte counts never decrease and terminal transitions are never dropped.

Frontend subscription order is mandatory:

1. register `Events.On('application-update-status', ...)`;
2. call `GetApplicationUpdateStatus`;
3. keep the snapshot with the greatest `revision`;
4. release the runtime listener exactly once on facade disposal.

An event received before the getter wins over an older getter result. A command result cannot
regress a newer event. Unknown states or malformed payloads normalize to safe inert values.

## Decision behavior

- A missing, unknown, or stale `attemptId` returns `ok: false` and the current snapshot.
- A decision not allowed in the current state returns `ok: false`; it does not start work.
- Repeated accept or restart requests cannot start a second operation.
- `defer` transitions the active candidate to `deferred` and suppresses it for this run.
- `postpone` returns `ok: true` without changing `ready-to-restart`; the staged result remains
  available and the UI may reopen the restart dialog.
- Backend errors are sanitized before entering `error` or the snapshot. Raw provider/helper errors
  may be wrapped internally for debugging but never cross this contract.

## Overseer presentation identifiers

The update surface remains available on both startup and main layouts.

- Global status: `applicationUpdateStatusPanel`, `applicationUpdateStatus`,
  `applicationUpdateError`, `applicationUpdateProgress`, `btnShowApplicationUpdate`
- Offer dialog: `applicationUpdateDialog`, `applicationUpdateInstalledVersion`,
  `applicationUpdateAvailableVersion`, `applicationUpdateReleaseNotes`,
  `btnAcceptApplicationUpdate`, `btnDeferApplicationUpdate`
- Restart dialog: `applicationUpdateRestartDialog`, `btnRestartApplicationUpdate`,
  `btnPostponeApplicationUpdate`

The status uses a polite live region; failures use an assertive alert. Both dialogs are modal and
labelled, restore focus, treat Escape as the safe choice, and disable duplicate actions while a
command is pending. Download and staging progress are nonmodal so sessions remain usable.
The offer's release-notes field contains the cumulative eligible changelog from the selected
version down to, but excluding, the installed version. Each release uses an explicit version
heading, including releases whose notes are empty, and the field remains bounded plain text.
