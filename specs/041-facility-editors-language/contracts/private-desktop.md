# Contract: Private Desktop Bridge — Facility Editors and Language

**Producer**: Go (`app.go`, `app_contract.go`, `desktop_service.go`, `main.go`) over the narrow
Wails desktop service, backed by `internal/control` and `internal/session`. **Consumers**: the
Overseer (`frontend/overseer/src/desktop-api.js`, `overseer.js`) and the browser fixture shims
(`tests/browser/fixture-server/main.go`, `tests/browser/fixtures/desktop-bindings.js`). **Schema
source of truth**: `proto/fallout/terminal/private/v1/desktop.proto` (Principle II). **Base**: the
bridge as left by `040-facility-play-controls`
(`specs/040-facility-play-controls/contracts/private-desktop.md`). This branch stacks on
`040-facility-play-controls`.

This feature adds no method, message, field, or event. It makes three changes:

1. It removes one private method, `RecoverFacilityCondition`, together with its whole path (R1).
2. It relaxes one session validation rule (R1).
3. It pins the Go failure texts that the Overseer error catalog keys on (R12).

Players are unaffected. The public player contract, `runtime.proto`, and the seven pinned event
names do not change.

## Clearing a fault by hand

`SetFacilityConditionActive` (040) is the only Overseer operation that turns a fault off by hand,
and its contract does not change. «Смотритель вручную» is that operation with `active: false`. The
device card's switch and the «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» switch both call it, for device-scoped and
terminal-scoped faults alike. The server ignores the condition's `recovery` list, so no stored value
gates it.

## Removed method: `RecoverFacilityCondition`

| Wails method | JS method | Request message | Payload type (`app.go`) |
|---|---|---|---|
| `RecoverFacilityCondition` | `recoverFacilityCondition` | `RecoverFacilityConditionRequest` | `FacilityRecoveryPayload` |

```proto
// desktop.proto — removed
message RecoverFacilityConditionRequest {
  string condition_id = 1;
  uint64 expected_facility_revision = 2;
  string correlation_id = 3;
  fallout.terminal.persistence.v1.DiagnosticRecoveryReference recovery = 4;
}
```

`fallout.terminal.persistence.v1.DiagnosticRecoveryReference` stays, because session JSON stores it
in `facility.conditions[].recovery[]`.

### Go removals by layer

Each symbol below goes, with its tests. So does every helper that this removal leaves without a
caller, for example `facilityRecoveryDecisionResult`, `cloneFacilityRecoveryRequest`, and
`sameDiagnosticRecoveryReference` when nothing else uses them.

| Layer | Removed |
|---|---|
| `internal/session` | The recovery branch of `Service.ApplyWorldAction` (`request.RecoveryConditionID != "" \|\| request.Recovery != nil`), `applyConditionRecoveryLocked`, `resolveConditionRecovery`, `conditionAllowsRecovery`, and the `WorldActionRequest` fields `RecoveryConditionID` and `Recovery` |
| `internal/control` | `Service.RecoverFacilityCondition`, `FacilityRecoveryRequest`, `privateFacilityRecovery`, `conditionAllowsRecovery`, `facilityRecoveryAudit`, the `FacilityMutationRequest` fields `RecoveryConditionID` and `Recovery`, `FacilityAuditActionRecover` (`"recover"`), and `facilityAuditEventRecovery` (`"facility.recovery"`) |
| `main.go` | the `RecoveryConditionID` and `Recovery` mapping in the `ApplyWorldAction` adapter, and `cloneDiagnosticRecoveryReference` |
| `app.go` | `App.RecoverFacilityCondition`, `FacilityRecoveryPayload`, `coordinationFacilityRecoveryService`, `validFacilityRecoveryResultForApp`, `"facility.recovery"` in `facilityAuditEvent`, and `FacilityAuditActionRecover` in `knownFacilityAuditAction` |
| `app_contract.go` | `facilityRecoveryRequestToPrivate`, `facilityRecoveryRequestFromPrivate`, `diagnosticRecoveryReferenceToPrivate`, `diagnosticRecoveryReferenceFromPrivate` (their only callers are the two recovery converters) |
| `desktop_service.go` | `desktopService.RecoverFacilityCondition` |
| `proto` | `RecoverFacilityConditionRequest`; regenerated `internal/gen/…` and `proto/schema-revision.txt` |

**Kept.** `knownFacilityRecoveryFailureForApp` stays, because the preview, inspection, and play
validators still call it. A rename would be unrelated churn.

**Logs.** No producer writes `facility.recovery` or `facility_action: recover` any more. Records
already in retained log files are not rewritten.

### Bridge allowlists

| Place | Change |
|---|---|
| `app_contract_test.go` | The pinned method list loses `RecoverFacilityCondition`. `require.Len(…, 47)` becomes `46` in both places. The `assertDescriptorFields` case for `RecoverFacilityConditionRequest` is removed |
| `app_test.go` | The `RecoverFacilityCondition` tests, `recordingFacilityRecoveryCoordination`, and the `facility.recovery` audit fixture are removed |
| `scripts/wails-bindings-check.sh` | `RecoverFacilityCondition` leaves the expected method list. The count becomes `test … = 46`, and the final message reads «… exactly 46 accepted desktop methods and seven named events.» Its `assert_method_signature` line is removed |
| `frontend/overseer/bindings/…/desktopservice.js` | Regenerated |
| `frontend/overseer/src/desktop-api.js` | Removed: `APP_METHODS.recoverFacilityCondition`, the public `recoverFacilityCondition`, `normalizeFacilityRecoveryPayload`, and the `'recovery'` branch of `validateFacilityMutationResult`. `FACILITY_DEPENDENCY_KINDS` and `FACILITY_ENTITY_KINDS` do not change |
| `tests/browser/desktop-api.spec.mjs` | The facade test title and count read 46 methods. `RecoverFacilityCondition` leaves both method lists, and its normalizer and validator cases are removed |
| `tests/browser/fixture-server/main.go` | The shim export `RecoverFacilityCondition`, the route `POST /__fixture/facility-authoring/recover`, and `fixtureFacilityAuthoringState.recover` are removed. The diagnostics route `POST /__fixture/facility-diagnostics/recover-private` becomes `POST /__fixture/facility-diagnostics/set-condition-active`. It takes the `SetFacilityConditionActive` payload (`conditionId`, `active`, `expectedFacilityRevision`, `correlationId`), so the escape-hatch test now proves that the fault switch works while every player capability is blocked |
| `tests/browser/fixtures/desktop-bindings.js` | The `RecoverFacilityCondition` export is removed |

## Unchanged operations

`SaveAuthoring`, `SetFacilityConditionActive`, `ApplyFacilityDeviceTransition`,
`ForceFacilityDeviceState`, `ResetFacilityDevice`, `ResetScene`, `PreviewFacility`, and
`InspectFacilityDependencies` keep their requests, results, and validators.

- **Dependency report.** `BuildFacilityDependencyReport` and its proto stay the same. The Overseer
  turns each `FacilityDependency` into words on its own side (`overseer-ui.md`, «Dependency
  dialog»).
- **Authoring save.** Every edit in the new dialogs is one `SaveAuthoring` call through
  `commitFacilityEdit`, with the 039 revision check and «ОТМЕНИТЬ» history (FR-043).
- **Fault edit.** A fault edit that changes other devices' transitions is still one candidate
  session and one save (R3).

## Session JSON version 1 (FR-042, SC-009)

No field is added, renamed, or removed. The only stored value whose meaning changes is
`facility.conditions[].recovery[].privateOverseerAction`. It no longer gates anything, and the
Overseer never writes or removes it.

### Validation change (`internal/domain/validate.go`)

`facilityGraphValidator.validateConditions` drops this rule:

```go
// removed
if (blocksProgress || len(condition.Recovery) != 0) && !hasViableRecovery {
	return fmt.Errorf("%s must provide recovery that clears the condition", path)
}
```

`validateRecoveryReference` still checks every entry that exists:

- exactly one variant per entry;
- a `transition` must name an existing transition with `recovery: true`;
- a `recoveryProgramId` must name an existing program whose transitions all exist and have
  `recovery: true`;
- `privateOverseerAction`, when present, must be `true`.

| Document | Before | After |
|---|---|---|
| A fault with a capability block and no `recovery` entry | rejected | accepted |
| A fault whose entries are all valid but none clears it | rejected | accepted |
| An entry with `privateOverseerAction: false` | rejected | rejected |
| An entry naming a missing transition or program | rejected | rejected |
| Any file that 040 accepts | accepted | accepted, unchanged |

Validation only widens, so every file saved before this feature opens and saves unchanged. A file
that 041 saves with a fault lacking a clearing entry will not open in builds 035 to 040. Those
builds never shipped (plan, Constitution VI).

`internal/platform/assets_test.go` stops requiring a recovery entry for every demo condition. The
demo files keep their stored `recovery[]` values (R16).

### Authoring invariants the Overseer keeps

The domain validators stay authoritative. These rules keep the Overseer's own saves valid without
rewriting data the author never touched (R2):

1. It never adds an entry to `condition.recovery[]`.
2. It never writes or removes an entry that carries `privateOverseerAction`.
3. It never writes `transition.recovery`. Device edits keep it through `facilityOriginal`.
4. In the same `mutate` as the edit that invalidates it, it drops a stored entry:
   - a `transition` entry, when its transition is deleted or no longer clears the condition;
   - a `recoveryProgramId` entry, when its program is deleted, loses its last transition that
     clears the condition, or gains a transition without `recovery: true`.
5. It never writes a transition that both activates and clears one condition, and never repeats
   an effect (R3).

## Error texts the Overseer keys on (R12)

The Go failure texts stay English, for the logs and for developers. The Overseer maps them to
Russian through `OVERSEER_ERROR_MESSAGES` in `overseer.js` (shape and values in `overseer-ui.md`,
«Errors»). Two Go tests keep both sides in step. Each test reads the source file with `go/parser`
and reads `frontend/overseer/src/overseer.js` as text:

| Test | Go texts collected | Catalog area |
|---|---|---|
| `internal/playerconfig/messages_test.go` | every string literal passed to `failure(…)` in `service.go`, plus the message of `errStorageUnavailable`, which reaches `failure(errStorageUnavailable.Error())` | `'player-config'` |
| `internal/session/service_messages_test.go` | every string literal passed to `sessionFailure(…)`, and every string literal passed as the last argument of `saveFailure(…)`, in `service.go` | `'session'` |

Each test fails in three cases:

- a collected text has no `byText` line `'<text>': '…',` inside its area block;
- a `byText` key in its area block matches no collected text;
- a key appears in both areas.

So changing a Go text means changing its catalog key in the same change. Dynamic texts
(`failure(err.Error())` with a wrapped error) are not collected. The Overseer shows them with the
area's general message, with the original under «подробности для разработчика».

Facility results carry no text: the Overseer keys them on `failure` codes through
`facilityFailureMessages`. The code set (`FACILITY_FAILURES` in `desktop-api.js`) does not
change.

## Browser fixture hooks (US7)

`overseer-errors.spec.mjs` triggers one failure from each area through these hooks. None of them
adds a route.

| Area | Hook |
|---|---|
| `player-config`, `session` | `__desktopFixture.setNextTerminalActionResult(method, { ok: false, error: '<Go text>' })`. `record(method, args)` in `tests/browser/fixtures/desktop-bindings.js` now honours the queue the way `terminalAction` does: a queued result is returned once, then cleared. This covers `OpenPlayerConfig`, `NewPlayerConfig`, `LoadReferencedPlayerConfig`, `NewSession`, `CopyDemo`, and `OpenSession` when no authoring or lifecycle fixture is active |
| `facility` (save) | `POST /__fixture/facility-authoring/next-operation` with `{"failure":"persistence-failed"}` now also applies to the next `save-authoring` call. That call returns `persistence-failed` and saves nothing. Before, the hook applied only to play, reset, and preview operations |
| `public-access` | the existing `POST /__fixture/public-access/failure/<kind>` |
| `update` | the existing stale-offer path: `ResolveApplicationUpdateOffer` with a superseded `attemptId` returns the English text «Application update offer is no longer available» |

## Compatibility and rollout

- **Proto.** Removing `RecoverFacilityConditionRequest` is a breaking change to the private schema.
  The message was added in 035, which is not in `develop`, so no shipped consumer depends on it.
  `proto/compatibility-baseline.binpb` is regenerated, as 035 and 039 did on this stack
  (Principle V, R1). After that, `task proto:breaking` passes against the new baseline.
- **Bindings.** `task proto:generate` updates `internal/gen/…` and `proto/schema-revision.txt`.
  `task bindings:check` must pass with 46 methods and seven events.
- **Data.** No data migration exists. Stored `privateOverseerAction` values stay in files as they
  are.
- **Rollback.** Rollback reverts the whole change, baseline included.
