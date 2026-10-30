package session

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// SceneResetRequest restores the whole scene against the exact session and
// facility revisions the caller counted the reset against.
type SceneResetRequest struct {
	ExpectedSessionRevision  uint64
	ExpectedFacilityRevision uint64
	CorrelationID            string
}

// CommandReference names one command of one terminal, as ResetScene reports
// it in FacilityOperationResult.ResetCommands.
type CommandReference = domain.CommandReference

// ResetScene returns every device and fault to its starting value and clears
// the executed commands of every terminal as one durable write. A failed write
// rolls the whole candidate back. The facility revision steps only when a
// device or fault changed, and the authoring revision never changes.
func (service *Service) ResetScene(ctx context.Context, request SceneResetRequest) domain.FacilityOperationResult {
	if service == nil || contextError(ctx) != nil {
		return sceneResetFailure(request, domain.FacilityFailureRuntimeContextEnded, 0, 0, 0, nil, "request")
	}
	service.commandMu.Lock()
	defer service.commandMu.Unlock()
	service.documentMu.Lock()
	defer service.documentMu.Unlock()

	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return sceneResetFailure(request, domain.FacilityFailureRuntimeContextEnded, 0, 0, 0, nil, "request")
	}
	if service.active.Path == "" || service.active.Session == nil {
		sessionRevision := service.active.SavedRevision
		service.mu.Unlock()
		return sceneResetFailure(
			request, domain.FacilityFailureRuntimeContextEnded, sessionRevision, 0, 0, nil, "session",
		)
	}

	canonical := domain.CloneSession(*service.active.Session)
	sessionRevision := service.active.SavedRevision
	previousFacilityRevision := facilityRevision(&canonical)
	authoringRevision := service.active.AuthoringRevision
	path := service.active.Path
	fail := func(failure domain.FacilityFailureCode, entityKind string) domain.FacilityOperationResult {
		service.mu.Unlock()
		return sceneResetFailure(
			request, failure, sessionRevision, previousFacilityRevision, authoringRevision, &canonical, entityKind,
		)
	}

	if samePath(path, service.locations.BundledDemo) {
		return fail(domain.FacilityFailureInvalidConfiguration, "session")
	}
	if request.ExpectedSessionRevision != sessionRevision || service.active.RequestedRevision != sessionRevision {
		return fail(domain.FacilityFailureStaleRevision, "session")
	}
	if request.ExpectedFacilityRevision != previousFacilityRevision {
		return fail(domain.FacilityFailureStaleRevision, "facility")
	}
	if strings.TrimSpace(request.CorrelationID) == "" {
		return fail(domain.FacilityFailureInvalidConfiguration, "request")
	}

	candidate := domain.CloneSession(canonical)
	affectedDeviceIDs, affectedConditionIDs := resetSceneFacility(candidate.Facility)
	resetCommands := resetSceneCommandStates(&candidate)
	facilityChanged := len(affectedDeviceIDs) != 0 || len(affectedConditionIDs) != 0
	if !facilityChanged && len(resetCommands) == 0 {
		service.mu.Unlock()
		result := worldActionSuccess(
			request.CorrelationID, false, sessionRevision, previousFacilityRevision, previousFacilityRevision,
			nil, nil, canonical,
		)
		result.AuthoringRevision = authoringRevision
		return result
	}

	resultingFacilityRevision := previousFacilityRevision
	if facilityChanged {
		resultingFacilityRevision++
		candidate.Facility.Revision = resultingFacilityRevision
	}
	saved, err := service.commitFacilityCandidateLocked(path, canonical, candidate)
	if err != nil {
		return sceneResetFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision, previousFacilityRevision,
			authoringRevision, &canonical, "candidate",
		)
	}
	if !saved.OK {
		return sceneResetFailure(
			request, domain.FacilityFailurePersistenceFailed, saved.SavedRevision, previousFacilityRevision,
			authoringRevision, &canonical, "persistence",
		)
	}
	result := worldActionSuccess(
		request.CorrelationID, true, saved.SavedRevision, previousFacilityRevision, resultingFacilityRevision,
		affectedDeviceIDs, affectedConditionIDs, candidate,
	)
	result.AuthoringRevision = authoringRevision
	result.ResetCommands = resetCommands
	return result
}

// resetSceneFacility returns every device and fault of facility to its
// starting value and reports the sorted IDs of those that changed. A nil
// facility has nothing to reset.
func resetSceneFacility(facility *domain.Facility) (deviceIDs, conditionIDs []string) {
	if facility == nil {
		return nil, nil
	}
	for index := range facility.Devices {
		device := &facility.Devices[index]
		if device.CurrentStateID != device.InitialStateID {
			device.CurrentStateID = device.InitialStateID
			deviceIDs = append(deviceIDs, device.ID)
		}
	}
	for index := range facility.Conditions {
		condition := &facility.Conditions[index]
		if condition.CurrentActive != condition.InitialActive {
			condition.CurrentActive = condition.InitialActive
			conditionIDs = append(conditionIDs, condition.ID)
		}
	}
	slices.Sort(deviceIDs)
	slices.Sort(conditionIDs)
	return deviceIDs, conditionIDs
}

// resetSceneCommandStates clears the executed commands of every terminal and
// reports them ordered by terminal ID, then command ID.
func resetSceneCommandStates(session *domain.Session) []CommandReference {
	var commands []CommandReference
	for index := range session.Terminals {
		terminal := &session.Terminals[index]
		if len(terminal.CommandStates) == 0 {
			continue
		}
		for _, commandID := range slices.Sorted(maps.Keys(terminal.CommandStates)) {
			commands = append(commands, CommandReference{TerminalID: terminal.ID, CommandID: commandID})
		}
		terminal.CommandStates = nil
	}
	slices.SortFunc(commands, func(left, right CommandReference) int {
		return cmp.Or(cmp.Compare(left.TerminalID, right.TerminalID), cmp.Compare(left.CommandID, right.CommandID))
	})
	return commands
}

func sceneResetFailure(
	request SceneResetRequest,
	failure domain.FacilityFailureCode,
	sessionRevision uint64,
	facilityRevision uint64,
	authoringRevision uint64,
	session *domain.Session,
	entityKind string,
) domain.FacilityOperationResult {
	result := facilityResetFailure(
		request.CorrelationID, failure, sessionRevision, facilityRevision, session, entityKind, "",
	)
	result.AuthoringRevision = authoringRevision
	return result
}
