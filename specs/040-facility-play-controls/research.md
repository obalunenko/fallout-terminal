# Research: Facility Play Controls

Each entry records a design choice this plan settles. File references point at the code the choice copies or changes.

## R1. A card transition is its own private session operation

**Decision**: Add `session.Service.ApplyFacilityDeviceTransition(ctx, request)`. It takes the same locks as `resetFacility` (`commandMu`, `documentMu`, `mu`), calls `resolveWorldTransitions` with one `{DeviceID, TransitionID}` request, and applies the destination state and `conditionValues` in one candidate. It bumps `Facility.Revision` once and commits through `commitFacilityCandidateLocked`. It does not write `CommandStates`.

**Rationale**: `resolveWorldTransitions` (`internal/session/facility.go:640`) already enforces source state, preconditions, and conflicting fault effects, all or nothing. That gives FR-003 for free. `ApplyWorldAction` cannot be reused because it requires a terminal and a command and checks the request against that command's authored action. Its doc rule ("transitions are never accepted as independent mutation authority") still holds for the public player service. This operation is private to the Overseer (Principle IV).

**Alternatives considered**: Running the transition through a synthetic command was rejected because it would invent command state and tie the card to command authoring. Copying the resolver into control was rejected because it would duplicate the rules.

## R2. Three named desktop methods, one result type

**Decision**: Add `ApplyFacilityDeviceTransition`, `ForceFacilityDeviceState`, and `SetFacilityConditionActive` as unary desktop methods next to `ResetFacilityDevice`. Each has its own request message and returns `FacilityOperationResult`.

**Rationale**: Each operation has a different exact-result rule. A transition may touch faults. A forced change touches exactly one device. A fault switch touches exactly one condition. Named methods keep each validator short, and they read like `ResetFacilityDevice` and `RecoverFacilityCondition`.

**Alternatives considered**: One `ChangeFacilityDevice` method with a `oneof` was rejected. It would still need a validator per variant, and the per-method allowlists (`APP_METHODS`, the method count, the bindings script) would not get any smaller.

## R3. The fault switch is new; recovery stays

**Decision**: `SetFacilityConditionActive{condition_id, active}` sets `CurrentActive` on any fault and never touches `InitialActive`. `RecoverFacilityCondition` and the «ВЫБРАННОЕ УСЛОВИЕ» panel stay unchanged.

**Rationale**: Recovery can only clear a fault, and only when the fault's authored recovery list allows `privateOverseerAction` (`conditionAllowsRecovery`). The clarification says the switch ignores that setting. Recovery still serves terminal-scoped faults, which have no device card. The server accepts any fault because the card's restriction to device-scoped faults is presentation, and a narrower server check would add a rule for no gain.

**Alternatives considered**: Extending recovery with an "activate" variant was rejected because it would change what «Восстановление оператором» means, and feature 041 owns that.

## R4. The facility-wide revision stays; a stale rejection refreshes the card

**Decision**: Every play change carries `expected_facility_revision`. A stale rejection returns the canonical session. The Overseer applies it and re-renders, as scene reset already does, and shows the existing stale message «СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС».

**Rationale**: Today `reconcileFacilityOperation` discards `result.session` on failure, so the card would keep showing the old state (FR-004). Scene reset already shows the pattern to follow. Per-device revisions would change the shared concurrency model for one message.

**Alternatives considered**: Per-device revisions were rejected as a model change across session, control, and App. Silent automatic retry was rejected because the Overseer must see that the scene moved.

## R5. The card computes precondition verdicts; the server decides

**Decision**: The Overseer computes each transition's availability from `state.session` with the server's rule, in the same order: source state first, then each precondition `devices[p.deviceId].currentStateId === p.stateId`. A missing device counts as unmet. The server's result remains authoritative.

**Rationale**: The server stops at the first unmet precondition and names its device, so it cannot provide a per-precondition met or unmet list (FR-002). The inputs are already in the session snapshot. SC-002 is guarded by a parity table run through both the client function and the real session operation.

**Alternatives considered**: Adding server-computed availability to the snapshot was rejected as extra contract for a pure function of data the client already holds.

## R6. The feed is runtime state carried by the coordination event

**Decision**: `domain.ProcessRuntime.FacilityChanges` holds up to five entries per device, newest first, plus a run-wide sequence. `masterSnapshot` projects it as `MasterCoordinationState.FacilityChanges`. The App converts it to `CoordinationState.facility_changes` (field 11), which reaches the Overseer through the existing coordination-state event. `ReplaceFacility` clears it. Entries carry IDs only, and the Overseer resolves names from the session. Timestamps come from a new `control.Config.Now` that defaults to `time.Now`.

**Rationale**: The clarification keeps the feed out of the session file. Principle VIII forbids rebuilding it from retained logs. The coordination event already carries pending-request state, so the feed lands in the same render pass. The event-name lists pinned in `app_contract_test.go` and the bindings script stay unchanged. An injected clock satisfies the deterministic-fake rule for tests.

**Alternatives considered**: A new event was rejected because it means editing two pinned lists for no behavior gain. Unbounded history was rejected because the retained log is the full history (spec Assumptions).

## R7. Feed producers and refusal reasons

**Decision**: Entries are appended inside the same `commit` closure that writes the matching audit record, through `feed.go` helpers:

| Producer | Entry | Device(s) |
|---|---|---|
| The three new ops and device reset | Overseer transition, forced, fault, reset | the target device; a fault entry uses the fault's device scope, and a terminal-scoped fault gets none |
| Scene reset (`scene.go`) | scene reset | each device it changed |
| Approved player command applied (`service.go`) | player command, with the terminal | each device in the command's action |
| Player request declined, failed, blocked, or withdrawn | player refusal with reason | each device in the command's authored action |

Refusal reasons: `DECLINED` (Overseer rejected), `PRECONDITION_FAILED`, `TRANSITION_UNAVAILABLE` (source state no longer matches), `FAULT_BLOCKED` (capability blocked before the request became pending, `service.go:2636`), `STALE` (including a feature 039 authoring save that could not rebase the pending request), `WITHDRAWN` (superseded by an Overseer play change or scene reset), and `FAILED` for the remaining facility failures, so every refusal carries a reason. Unchanged (no-op) results add no entry. `data-model.md` maps each failure code to a reason.

**Rationale**: Writing the entry and the audit in one closure keeps SC-005: neither appears without the other. The command's authored action names its devices even when the request was refused before it became pending.

## R8. Audit additions and withdrawal records

**Decision**: Add `FacilityAuditAction` values `transition`, `force`, and `fault`. Add `TransitionID` to `FacilityAuditFacts`. Extend `facilityAuditEvent`, `facilityAuditOutcome`, and `knownFacilityAuditAction` in `app.go`. Device reset and the three new ops call the scene-reset withdrawal pattern (`sceneResetPendingOutcome` / `pendingRequestOutcomeAudit("superseded")`) when they withdraw a pending request. New validators allow `FacilityFailureRejected` where `knownFacilityRecoveryFailureForApp` excludes it.

**Rationale**: Today `invalidatePendingFacilityAction` marks the player's request rejected but logs nothing, which leaves a Principle VIII gap. Unknown actions are dropped from the log, and unknown outcomes are logged as "invalid". Transition IDs are authored technical identifiers of the same class as the device IDs already logged.

## R9. Fault-only reset fix

**Decision**: `validFacilityResetResultForApp` and the `device-reset` branch of `validateFacilityMutationResult` accept a changed result with either zero affected devices or exactly `[deviceId]`, and with affected conditions limited to faults scoped to that device. At least one ID must be affected.

**Rationale**: The session layer already allows a reset that only restores faults (`internal/session/facility.go:165-178`). The App then reports `persistence-failed` even though the session saved, and the JS layer flags a conflict. The fault switch makes this the common case (FR-011).

## R10. Guide rule and run memory

**Decision**: The guide is open when the author made an explicit choice to open it, or, with no choice made, when the session has no devices. The explicit choice lives in a module variable in `overseer.js`. It is cleared when the device count goes from zero to one, so the guide collapses on the first saved device. `index.html` drops the hard-coded `open`, and JS sets it. The summary line shows «КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ N ИЗ 5: <step>» with «ПОКАЗАТЬ» or «СКРЫТЬ». `.facility-story-steps` loses `position: sticky`.

**Rationale**: This follows the clarification and needs no storage. The sticky bar inside the scrolling panel is what covers text at 1180 points (`overseer.css:2030`).

## R11. Detail layout, menu, and tabs

**Decision**: The detail area follows the order in `contracts/overseer-ui.md`. «⋯» is a menu button (`aria-haspopup="menu"`) with one `menuitem`, «УДАЛИТЬ УСТРОЙСТВО»; Escape and outside clicks close it. The tabs follow the ARIA tabs pattern with arrow-key movement, and the active tab is remembered for the run. «РЕДАКТИРОВАТЬ ГРАФ» becomes «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ». The name field, `#btnSaveFacilityDeviceChanges`, and `#btnCancelFacilityDeviceChanges` are removed. The combined bindings list is split into the two player tabs, using the groups `overseer.js` already builds. The device row reads `<state>` or `<state> · ● <n>`, and search still matches the state name.

**Rationale**: These are F-09 C and D. Renaming already works in the device dialog.

## R12. Demo and tests

**Decision**: `reactor-main` gains the state «Критический перегрев» with no incoming transition, plus one record variant on a reactor terminal that names it. `sessions/demo-capability-paths.md` lists six paths:

- a card transition;
- a blocked transition (power the grid down, then the door);
- a forced change (reactor);
- a fault switch (door controller);
- a declined player request in the feed;
- the ventilation row with an active fault.

Browser journeys run on a demo copy, as today, because the bundled demo cannot be opened. SC-002 parity is a Go table test over every transition in `sessions/demo.json`, because the browser fixture's own transition helper skips preconditions.

**Rationale**: Every demo device has exactly two states with transitions both ways, so forced change has no meaningful demo path without a third state (Principle IX). A parity test against the fixture would prove nothing.
