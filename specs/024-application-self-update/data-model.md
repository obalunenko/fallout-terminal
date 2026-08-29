# Data Model: Application Self-Update

## ApplicationUpdateManager

The launch-scoped Go owner of update discovery, consent, preparation, restart handoff, and public
status. It owns one current `UpdateAttempt`, a monotonically increasing snapshot revision, the
launch-scoped deferred version, cancellation, and serialization. It never owns or mutates session,
player-configuration, credential, or preference data.

| Field | Type | Rules |
|---|---|---|
| `installedVersion` | string | Canonical linker-injected version; `development` disables production discovery. |
| `packaged` | bool | Must be true with a release version before a network check is eligible. |
| `revision` | uint64 | Starts at zero and increments for every externally observable state or progress change. |
| `attempt` | optional `UpdateAttempt` | At most one launch-scoped attempt; replaced only by the one permitted startup check. |
| `deferredVersion` | optional string | Suppresses the same candidate during this run only; never persisted. |
| `checkOnce` | one-shot guard | Concurrent status handshakes can start no more than one production check. |
| `operation` | mutex/operation token | Serializes offer and restart decisions and rejects stale attempt identifiers. |
| `applyHandoff` | bool | When true, shutdown retains the staged unit for the already launched helper. |

## UpdateAttempt

One launch-scoped progression from discovery through a terminal state.

| Field | Type | Rules |
|---|---|---|
| `id` | opaque string | Fresh for the check; included in every decision and never derived from a path or secret. |
| `state` | `UpdateState` | Must follow the transition table below. |
| `candidate` | optional `UpdateCandidate` | Present from `available` through preparation/restart states. |
| `progress` | `UpdateProgress` | Bytes are nonnegative; total is optional until known; written never decreases. |
| `failure` | optional `UpdateFailure` | Present only in `failed`; safe for logging and UI projection. |
| `preparedUnit` | optional `PreparedApplicationUnit` | Present only after complete validation and same-volume staging. Never projected to the UI. |

## UpdateState

| State | Meaning | Allowed next states |
|---|---|---|
| `disabled` | Development, unversioned, or unpackaged application; no production check occurs. | none |
| `idle` | Packaged release is initialized and waiting for the event-first UI handshake. | `checking` |
| `checking` | The bounded GitHub release lookup is running. | `current`, `available`, `failed` |
| `current` | No strictly newer complete eligible release exists. | none |
| `available` | One complete eligible candidate is offered; no bytes have been downloaded. | `deferred`, `downloading`, `failed` |
| `deferred` | The Overseer declined this candidate for the current run. | none |
| `downloading` | The accepted archive is streaming to Wails staging. | `verifying`, `failed` |
| `verifying` | Wails is comparing the downloaded bytes with the GitHub asset SHA-256. | `staging`, `failed` |
| `staging` | The extracted manifest/package is validated and copied beside the installed unit. | `ready-to-restart`, `failed` |
| `ready-to-restart` | A durable compatible staged unit awaits a second Overseer decision. | `applying`, `failed` |
| `applying` | Helper handoff is committed and normal ordered shutdown has been requested. | terminal in this process; next launch consumes recovery journal |
| `failed` | A safe failure stage and recovery action are available; local operation continues when the process remains open. | none |

Repeated, stale, or invalid decisions do not transition state. They return the current snapshot with
`ok: false` and a stable safe error. `postpone` in `ready-to-restart` deliberately leaves the state
unchanged and only closes the current restart prompt in the frontend.

## UpdateCandidate

A release that passed channel, version, completeness, target, and evidence checks.

| Field | Type | Validation |
|---|---|---|
| `version` | string | Strict canonical v2 SemVer without leading `v`; strictly newer than installed. |
| `channel` | `stable` or `prerelease` | Stable installations accept only stable; prerelease installations accept eligible newer prerelease or stable. |
| `name` | string | Optional provider display name; bounded and treated as untrusted presentation text. |
| `releaseNotes` | string | Optional bounded cumulative changelog for every eligible version above the installed release through the candidate, grouped newest-first by explicit version heading and rendered without arbitrary HTML or script execution. |
| `publishedAt` | timestamp | Informational only; never overrides semantic-version ordering. |
| `artifact` | `ReleaseAsset` | Exactly one target match from a complete five-asset release. |

## PublishedRelease

The normalized GitHub Release metadata evaluated by the provider wrapper.

| Field | Type | Validation |
|---|---|---|
| `tag` | string | Strict `v2.MINOR.PATCH[-prerelease]`; build metadata and malformed identifiers rejected. |
| `draft` | bool | Must be false. |
| `prerelease` | bool | Must agree with tag prerelease status. |
| `assets` | exactly five `ReleaseAsset` values | Names must equal the governed target inventory with no duplicate or extra asset. |

## ReleaseAsset

| Field | Type | Validation |
|---|---|---|
| `id` | positive integer | Stable only within the GitHub release; used by the provider, never exposed. |
| `name` | string | Must be one exact governed archive name. |
| `state` | string | Must be `uploaded`. |
| `size` | int64 | Must be greater than zero and agree with download progress when supplied. |
| `digestAlgorithm` | string | Must be `sha256`. |
| `digest` | 32 bytes | Decoded from one lowercase or uppercase 64-hex GitHub asset digest. |
| `downloadURL` | URL | HTTPS GitHub release asset URL used only by the backend; never projected. |
| `target` | OS/architecture | Derived from the exact archive name, never from substring-first matching. |

## ArtifactManifestV2

Build-tool-owned JSON stored at the existing archive package-root manifest path.

| Field | Type | Validation |
|---|---|---|
| `schemaVersion` | integer | Exactly `2`. |
| `product` | string | Exactly `Fallout Terminal`. |
| `version` | string | Canonical build version; for update candidates must equal the accepted release version. |
| `sourceRevision` | string | Full lowercase Git source revision under the existing manifest rule. |
| `target.os` | string | One of `windows`, `linux`, `darwin`; must equal runtime OS. |
| `target.arch` | string | One of `amd64`, `arm64`; must equal runtime architecture and governed matrix. |
| `runtime` | string | Existing target runtime description; informational after exact target validation. |
| `files` | ordered file records | Exact target inventory; each path, size, mode, and SHA-256 must validate under existing archive rules. |

Development packages may record `development`, but tagged release inspection rejects that value.
Self-update accepts only schema v2 candidates because schema v1 does not bind the extracted tree to
the accepted release version.

## PreparedApplicationUnit

Backend-only record for an accepted, verified, structurally compatible adjacent stage.

| Field | Type | Rules |
|---|---|---|
| `attemptID` | string | Must match the active attempt. |
| `version` | string | Must match candidate and manifest. |
| `target` | platform/architecture | Must match runtime and manifest. |
| `installedUnit` | absolute path | Windows/Linux portable root directory or enclosing macOS `.app`; never logged or projected. |
| `stagedUnit` | absolute path | Same-parent, same-volume sibling owned by this attempt; never logged or projected. |
| `launchRelativePath` | relative path | Executable relative to the unit; traversal and absolute paths forbidden. |

For Windows/Linux the replacement unit is the extracted `Fallout Terminal` directory. For macOS it
is the nested `Fallout Terminal.app` selected only after validating the outer archive root and its
manifest.

## UpdateFailure

| Field | Type | Rules |
|---|---|---|
| `stage` | `check`, `download`, `verify`, `stage`, `apply`, `relaunch`, or `recovery` | Required and stable. |
| `message` | string | Actionable sanitized summary; no URL query, token, path, user document content, or raw response body. |
| `recoveryAction` | string | Concrete next step such as continue locally, retry next launch, choose a writable installation, or inspect the safe helper log. |

## UpdateRecoveryRecord

An atomic, versioned, non-sensitive record under Application Support, distinct from user-owned
session/configuration files.

| Field | Type | Rules |
|---|---|---|
| `schemaVersion` | integer | Exactly `1`. |
| `attemptID` | string | Opaque attempt correlation only. |
| `expectedVersion` | string | Candidate version. |
| `state` | `applying`, `applied`, or `failed` | Written atomically by the manager/helper. |
| `failedStage` | optional failure stage | Present for `failed`. |
| `message` | optional safe string | No filesystem paths or provider payloads. |
| `recoveryAction` | optional safe string | Present for `failed`. |
| `updatedAt` | timestamp | Diagnostic ordering only. |

The next normal launch consumes the record. `failed` becomes the initial visible update failure;
`applied` is cleared after confirming the running version; stale `applying` is converted to a safe
recovery diagnostic. The record never causes startup failure.
