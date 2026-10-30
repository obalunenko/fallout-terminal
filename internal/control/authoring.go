package control

import (
	"context"
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// AuthoringSaveRequest is one complete authored session saved against the
// authoring revision the Overseer last saw. Live values in Candidate are
// ignored: the durable owner keeps its own.
type AuthoringSaveRequest struct {
	Candidate                 domain.Session
	ExpectedAuthoringRevision uint64
	CorrelationID             string
}

// AuthoringStore persists one authoring save while keeping canonical live
// values. Live changes never make a save stale; only another authored change
// does.
type AuthoringStore interface {
	SaveAuthoring(context.Context, AuthoringSaveRequest) domain.FacilityOperationResult
}

// SaveAuthoring serializes one terminal or facility edit with player world
// actions, installs the durable result, refreshes every broadcast terminal so
// players see the edit at once, and keeps or rejects a pending device action
// depending on whether the edit changed it.
func (service *Service) SaveAuthoring(
	ctx context.Context,
	request AuthoringSaveRequest,
) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
	if service == nil {
		result := authoringDecisionResult(request, 0, domain.FacilityFailureRuntimeContextEnded)
		return nil, &result, facilityFailureError(result.Failure)
	}
	if ctx == nil {
		result := authoringDecisionResult(request, service.currentFacilityRevision(), domain.FacilityFailureRuntimeContextEnded)
		return service.Snapshot(), &result, facilityFailureError(result.Failure)
	}

	request.Candidate = domain.CloneSession(request.Candidate)
	var state *domain.MasterCoordinationState
	var operation *domain.FacilityOperationResult
	var operationErr error
	committed := service.commit(func(runtime *domain.ProcessRuntime) transition {
		state = masterSnapshot(runtime)
		currentRevision := facilityRevision(runtime.Facility)
		fail := func(code domain.FacilityFailureCode) transition {
			result := authoringDecisionResult(request, currentRevision, code)
			operation = &result
			operationErr = facilityFailureError(code)
			return transition{}
		}

		if ctx.Err() != nil {
			return fail(domain.FacilityFailureRuntimeContextEnded)
		}
		if strings.TrimSpace(request.CorrelationID) == "" {
			return fail(domain.FacilityFailureInvalidConfiguration)
		}
		if service.authoringStore == nil {
			return fail(domain.FacilityFailurePersistenceFailed)
		}
		if !canRefreshFacilityAuthoringRuntimes(runtime, &request.Candidate, service.terminals) {
			return fail(domain.FacilityFailureInvalidConfiguration)
		}

		durable := cloneFacilityOperationResult(service.authoringStore.SaveAuthoring(ctx, AuthoringSaveRequest{
			Candidate:                 domain.CloneSession(request.Candidate),
			ExpectedAuthoringRevision: request.ExpectedAuthoringRevision,
			CorrelationID:             request.CorrelationID,
		}))
		if !durable.OK {
			if !validAuthoringFailure(durable, request, currentRevision) {
				return fail(domain.FacilityFailurePersistenceFailed)
			}
			durable.Changed = false
			durable.AffectedDeviceIDs = nil
			durable.AffectedConditionIDs = nil
			durable.ResetCommands = nil
			operation = &durable
			operationErr = facilityFailureError(durable.Failure)
			return transition{}
		}
		if !validAuthoringSuccess(durable, request, runtime) {
			return fail(domain.FacilityFailurePersistenceFailed)
		}
		operation = &durable
		if !durable.Changed {
			return transition{}
		}

		runtime.Facility = domain.CloneFacility(durable.Session.Facility)
		refreshFacilityAuthoringRuntimes(runtime, durable.Session, service.terminals)
		change := transition{accepted: true}
		if event, ok := service.reconcilePendingFacilityAction(runtime, durable.Session, currentRevision); ok {
			change.audit(event)
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
		result := authoringDecisionResult(request, service.currentFacilityRevision(), domain.FacilityFailurePersistenceFailed)
		operation = &result
		operationErr = facilityFailureError(result.Failure)
	}
	result := cloneFacilityOperationResult(*operation)
	return domain.CloneMasterCoordinationState(state), &result, operationErr
}

// reconcilePendingFacilityAction keeps a pending device action approvable
// when the saved session left the action unchanged, and rejects it through
// the existing stale path otherwise. It returns the request's outcome record
// whenever the request's expected facility revision had to change.
func (service *Service) reconcilePendingFacilityAction(
	runtime *domain.ProcessRuntime,
	session *domain.Session,
	previousFacilityRevision uint64,
) (AuditEvent, bool) {
	pending := runtime.PendingCommandExecution
	if pending == nil || pending.FacilityAction == nil {
		return AuditEvent{}, false
	}
	resultingFacilityRevision := facilityRevision(runtime.Facility)
	if rebased, ok := rebasedPendingFacilityAction(runtime.Facility, session, pending); ok {
		if rebased.ExpectedFacilityRevision == pending.FacilityAction.ExpectedFacilityRevision {
			return AuditEvent{}, false
		}
		pending.FacilityAction = rebased
		return pendingRequestOutcomeAudit(pending, "rebased", previousFacilityRevision, resultingFacilityRevision), true
	}
	event := pendingRequestOutcomeAudit(pending, "superseded", previousFacilityRevision, resultingFacilityRevision)
	service.invalidatePendingFacilityAction(runtime)
	return event, true
}

// rebasedPendingFacilityAction returns the pending action moved to the
// facility's revision when the authored command still resolves to the same
// transitions, source states, and affected conditions.
func rebasedPendingFacilityAction(
	facility *domain.Facility,
	session *domain.Session,
	pending *domain.PendingCommandExecution,
) (*domain.PendingFacilityAction, bool) {
	terminal := terminalByStableID(session, pending.TerminalID)
	if facility == nil || terminal == nil {
		return nil, false
	}
	command, ok := findContentNode(&terminal.Root, pending.CommandID)
	if !ok || command.StateChange == nil || command.StateChange.FacilityAction == nil {
		return nil, false
	}
	rebased := *pending.FacilityAction
	rebased.ExpectedFacilityRevision = facility.Revision
	if revalidatePendingFacilityAction(facility, command.StateChange.FacilityAction, &rebased) !=
		domain.FacilityFailureUnspecified {
		return nil, false
	}
	return &rebased, true
}

// pendingRequestOutcomeAudit records what an Overseer operation did to one
// pending player request, with the facility revisions it moved between.
func pendingRequestOutcomeAudit(
	pending *domain.PendingCommandExecution,
	outcome string,
	previousFacilityRevision uint64,
	resultingFacilityRevision uint64,
) AuditEvent {
	event := commandDecisionAudit(pending, "", outcome)
	event.Name = "command.request_outcome"
	event.Facility = &FacilityAuditFacts{
		CorrelationID: pending.RequestID, BroadcastID: pending.BroadcastID,
		TerminalID: pending.TerminalID, CommandID: pending.CommandID, Outcome: outcome,
		PreviousFacilityRevision: previousFacilityRevision, ResultingFacilityRevision: resultingFacilityRevision,
	}
	return event
}

func authoringDecisionResult(
	request AuthoringSaveRequest,
	currentRevision uint64,
	failure domain.FacilityFailureCode,
) domain.FacilityOperationResult {
	return domain.FacilityOperationResult{
		CorrelationID:             request.CorrelationID,
		Failure:                   failure,
		PreviousFacilityRevision:  currentRevision,
		ResultingFacilityRevision: currentRevision,
	}
}

func validAuthoringFailure(
	result domain.FacilityOperationResult,
	request AuthoringSaveRequest,
	currentFacilityRevision uint64,
) bool {
	if result.OK || result.CorrelationID != request.CorrelationID || !knownFacilityFailure(result.Failure) ||
		result.Failure == domain.FacilityFailureRejected ||
		result.PreviousFacilityRevision != result.ResultingFacilityRevision {
		return false
	}
	if result.Failure != domain.FacilityFailureStaleRevision && result.PreviousFacilityRevision != currentFacilityRevision {
		return false
	}
	if result.Session == nil {
		return true
	}
	return domain.ValidateSession(*result.Session) == nil &&
		facilityRevision(result.Session.Facility) == result.ResultingFacilityRevision
}

// validAuthoringSuccess accepts a durable result only when it belongs to this
// request, starts from the installed facility, and kept the installed live
// values.
func validAuthoringSuccess(
	result domain.FacilityOperationResult,
	request AuthoringSaveRequest,
	runtime *domain.ProcessRuntime,
) bool {
	if !result.OK || (result.Failure != "" && result.Failure != domain.FacilityFailureUnspecified) ||
		result.CorrelationID != request.CorrelationID || result.Session == nil ||
		result.PreviousFacilityRevision != facilityRevision(runtime.Facility) ||
		facilityRevision(result.Session.Facility) != result.ResultingFacilityRevision ||
		domain.ValidateSession(*result.Session) != nil {
		return false
	}
	wantAuthoringRevision := request.ExpectedAuthoringRevision
	if result.Changed {
		wantAuthoringRevision++
	}
	return result.AuthoringRevision == wantAuthoringRevision &&
		sameFacilityLiveValues(runtime.Facility, result.Session.Facility)
}

// sameFacilityLiveValues reports whether every device and condition present
// in both facilities has the same current value.
func sameFacilityLiveValues(installed, durable *domain.Facility) bool {
	if installed == nil || durable == nil {
		return true
	}
	states := make(map[string]string, len(installed.Devices))
	for _, device := range installed.Devices {
		states[device.ID] = device.CurrentStateID
	}
	for _, device := range durable.Devices {
		if current, ok := states[device.ID]; ok && current != device.CurrentStateID {
			return false
		}
	}
	active := make(map[string]bool, len(installed.Conditions))
	for _, condition := range installed.Conditions {
		active[condition.ID] = condition.CurrentActive
	}
	for _, condition := range durable.Conditions {
		if current, ok := active[condition.ID]; ok && current != condition.CurrentActive {
			return false
		}
	}
	return true
}
