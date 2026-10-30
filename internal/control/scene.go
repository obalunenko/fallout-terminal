package control

import (
	"context"
	"slices"
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// SceneResetRequest returns every device, fault, and executed command to its
// starting value, counted against exact session and facility revisions.
type SceneResetRequest struct {
	ExpectedSessionRevision  uint64
	ExpectedFacilityRevision uint64
	CorrelationID            string
}

// SceneResetStore persists one scene reset as a single durable write.
type SceneResetStore interface {
	ResetScene(context.Context, SceneResetRequest) domain.FacilityOperationResult
}

// ResetScene restores the starting scene in one durable write and one
// coordinator transition. Players keep their active terminal, return route,
// and hacking progress; a pending device action is invalidated and a pending
// request for a reset command is cleared.
func (service *Service) ResetScene(
	ctx context.Context,
	request SceneResetRequest,
) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
	if service == nil {
		result := sceneResetDecisionResult(request, 0, domain.FacilityFailureRuntimeContextEnded)
		return nil, &result, facilityFailureError(result.Failure)
	}
	if ctx == nil {
		result := sceneResetDecisionResult(request, service.currentFacilityRevision(), domain.FacilityFailureRuntimeContextEnded)
		return service.Snapshot(), &result, facilityFailureError(result.Failure)
	}

	var state *domain.MasterCoordinationState
	var operation *domain.FacilityOperationResult
	var operationErr error
	committed := service.commit(func(runtime *domain.ProcessRuntime) transition {
		state = masterSnapshot(runtime)
		currentRevision := facilityRevision(runtime.Facility)
		fail := func(code domain.FacilityFailureCode, outcome string) transition {
			result := sceneResetDecisionResult(request, currentRevision, code)
			operation = &result
			operationErr = facilityFailureError(code)
			change := transition{}
			change.audit(sceneResetAudit(request, result, outcome, nil, nil))
			return change
		}

		if ctx.Err() != nil {
			return fail(domain.FacilityFailureRuntimeContextEnded, "failed")
		}
		if strings.TrimSpace(request.CorrelationID) == "" {
			return fail(domain.FacilityFailureInvalidConfiguration, "invalid")
		}
		if request.ExpectedFacilityRevision != currentRevision {
			return fail(domain.FacilityFailureStaleRevision, "stale")
		}
		store, ok := service.facilityStore.(SceneResetStore)
		if !ok {
			return fail(domain.FacilityFailurePersistenceFailed, "failed")
		}

		durable := cloneFacilityOperationResult(store.ResetScene(ctx, request))
		if !durable.OK {
			if !validSceneResetFailure(durable, request, currentRevision) {
				return fail(domain.FacilityFailurePersistenceFailed, "failed")
			}
			failed := normalizedFacilityFailureResult(durable)
			failed.AuthoringRevision = durable.AuthoringRevision
			operation = &failed
			operationErr = facilityFailureError(failed.Failure)
			outcome := "failed"
			if failed.Failure == domain.FacilityFailureStaleRevision {
				outcome = "stale"
			}
			change := transition{}
			change.audit(sceneResetAudit(request, failed, outcome, nil, nil))
			return change
		}
		if !validSceneResetSuccess(durable, request, currentRevision) {
			return fail(domain.FacilityFailurePersistenceFailed, "failed")
		}
		operation = &durable
		if !durable.Changed {
			change := transition{}
			change.audit(sceneResetAudit(request, durable, "unchanged", nil, nil))
			return change
		}

		change := transition{accepted: true}
		change.audit(sceneResetAudit(request, durable, "succeeded", runtime.Facility, durable.Session.Facility))
		if event, ok := sceneResetPendingOutcome(runtime, request, durable); ok {
			change.audit(event)
		}
		runtime.Facility = domain.CloneFacility(durable.Session.Facility)
		service.invalidatePendingFacilityAction(runtime)
		if broadcast := runtime.Broadcast; broadcast != nil {
			for terminalID, terminal := range broadcast.TerminalRuntimes {
				if terminal == nil {
					continue
				}
				clearResetCommandPresentation(runtime, terminal, "", true)
				if durableTerminal := terminalByStableID(durable.Session, terminalID); durableTerminal != nil {
					terminal.CommandStates = cloneCommandStates(durableTerminal.CommandStates)
				}
			}
		}
		projection := service.reprojectTerminalRuntimes(runtime)
		state = masterSnapshot(runtime)
		change.effects = append(change.effects, stateEffects(runtime)...)
		if projection != nil {
			change.effects = append(change.effects, Effect{Live: projection})
		}
		return change
	})
	if state == nil {
		state = service.Snapshot()
	}
	state.Revision = committed.revision
	if operation == nil {
		result := sceneResetDecisionResult(request, service.currentFacilityRevision(), domain.FacilityFailurePersistenceFailed)
		operation = &result
		operationErr = facilityFailureError(result.Failure)
	}
	result := cloneFacilityOperationResult(*operation)
	return domain.CloneMasterCoordinationState(state), &result, operationErr
}

// sceneResetPendingOutcome returns the outcome record for a pending request
// the reset will resolve: one with a device action, or one for a command the
// reset clears. Any other pending request stays pending and gets no record.
func sceneResetPendingOutcome(
	runtime *domain.ProcessRuntime,
	request SceneResetRequest,
	result domain.FacilityOperationResult,
) (AuditEvent, bool) {
	pending := runtime.PendingCommandExecution
	if pending == nil {
		return AuditEvent{}, false
	}
	reset := slices.Contains(result.ResetCommands, domain.CommandReference{
		TerminalID: pending.TerminalID, CommandID: pending.CommandID,
	})
	if pending.FacilityAction == nil && !reset {
		return AuditEvent{}, false
	}
	event := pendingRequestOutcomeAudit(
		pending, "superseded", request.ExpectedFacilityRevision, result.ResultingFacilityRevision,
	)
	event.Facility.PreviousSessionRevision = request.ExpectedSessionRevision
	event.Facility.ResultingSessionRevision = result.SessionRevision
	return event, true
}

func sceneResetAudit(
	request SceneResetRequest,
	result domain.FacilityOperationResult,
	outcome string,
	previous *domain.Facility,
	resulting *domain.Facility,
) AuditEvent {
	resultingSessionRevision := request.ExpectedSessionRevision
	if result.Changed {
		resultingSessionRevision = result.SessionRevision
	}
	facts := FacilityAuditFacts{
		CorrelationID: request.CorrelationID, Action: FacilityAuditActionReset, Outcome: outcome,
		DeviceIDs: sortedUniqueStrings(result.AffectedDeviceIDs), ConditionIDs: sortedUniqueStrings(result.AffectedConditionIDs),
		ResetScope: "scene", Failure: result.Failure,
		PreviousFacilityRevision: result.PreviousFacilityRevision, ResultingFacilityRevision: result.ResultingFacilityRevision,
		PreviousSessionRevision: request.ExpectedSessionRevision, ResultingSessionRevision: resultingSessionRevision,
		ResetCommands: slices.Clone(result.ResetCommands),
	}
	addFacilityAuditTransitions(&facts, previous, resulting)
	return AuditEvent{Name: facilityAuditEventReset, Outcome: outcome, RequestID: request.CorrelationID, Facility: &facts}
}

func sceneResetDecisionResult(
	request SceneResetRequest,
	currentRevision uint64,
	failure domain.FacilityFailureCode,
) domain.FacilityOperationResult {
	return domain.FacilityOperationResult{
		CorrelationID:             request.CorrelationID,
		Failure:                   failure,
		SessionRevision:           request.ExpectedSessionRevision,
		PreviousFacilityRevision:  currentRevision,
		ResultingFacilityRevision: currentRevision,
	}
}

func validSceneResetFailure(
	result domain.FacilityOperationResult,
	request SceneResetRequest,
	currentFacilityRevision uint64,
) bool {
	return !result.OK && !result.Changed && result.CorrelationID == request.CorrelationID &&
		knownFacilityFailure(result.Failure) && result.Failure != domain.FacilityFailureRejected &&
		result.PreviousFacilityRevision == currentFacilityRevision &&
		result.ResultingFacilityRevision == currentFacilityRevision &&
		len(result.ResetCommands) == 0
}

// validSceneResetSuccess accepts a durable result only when it starts from
// the installed facility and holds the starting scene: every device and fault
// at its starting value and no executed command left.
func validSceneResetSuccess(
	result domain.FacilityOperationResult,
	request SceneResetRequest,
	currentFacilityRevision uint64,
) bool {
	if !result.OK || (result.Failure != "" && result.Failure != domain.FacilityFailureUnspecified) ||
		result.CorrelationID != request.CorrelationID || result.Session == nil ||
		result.PreviousFacilityRevision != currentFacilityRevision ||
		facilityRevision(result.Session.Facility) != result.ResultingFacilityRevision ||
		domain.ValidateSession(*result.Session) != nil {
		return false
	}
	if facility := result.Session.Facility; facility != nil {
		for _, device := range facility.Devices {
			if device.CurrentStateID != device.InitialStateID {
				return false
			}
		}
		for _, condition := range facility.Conditions {
			if condition.CurrentActive != condition.InitialActive {
				return false
			}
		}
	}
	for _, terminal := range result.Session.Terminals {
		if len(terminal.CommandStates) != 0 {
			return false
		}
	}
	return slices.IsSortedFunc(result.ResetCommands, func(left, right domain.CommandReference) int {
		if order := strings.Compare(left.TerminalID, right.TerminalID); order != 0 {
			return order
		}
		return strings.Compare(left.CommandID, right.CommandID)
	})
}
