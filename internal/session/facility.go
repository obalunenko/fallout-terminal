package session

import (
	"context"
	"reflect"
	"slices"
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// WorldActionRequest is the exact approved facility action presented to the
// durable session owner. Transitions must match the currently authored
// command action; they are never accepted as independent mutation authority.
type WorldActionRequest struct {
	CorrelationID            string
	TerminalID               string
	CommandID                string
	ExpectedFacilityRevision uint64
	Transitions              []domain.FacilityTransitionRequest
	RecoveryConditionID      string
	Recovery                 *domain.DiagnosticRecoveryReference
}

// FacilityDeviceResetRequest restores one device and conditions scoped
// directly to it against an exact facility revision.
type FacilityDeviceResetRequest struct {
	DeviceID                 string
	ExpectedFacilityRevision uint64
	CorrelationID            string
}

// FacilityResetRequest restores every facility current value against an exact
// facility revision.
type FacilityResetRequest struct {
	ExpectedFacilityRevision uint64
	CorrelationID            string
}

// FacilityAuthoringRequest is one complete trusted facility definition and
// reference replacement against exact document and facility revisions.
type FacilityAuthoringRequest struct {
	Candidate                domain.Session
	ExpectedSessionRevision  uint64
	ExpectedFacilityRevision uint64
	CorrelationID            string
}

type resolvedWorldTransition struct {
	deviceIndex int
	transition  domain.FacilityDeviceTransition
}

// SaveFacilityAuthoring validates and persists one complete authored graph
// and reference repair while retaining session-owned current values.
func (service *Service) SaveFacilityAuthoring(ctx context.Context, request FacilityAuthoringRequest) domain.FacilityOperationResult {
	if service == nil || contextError(ctx) != nil {
		return facilityAuthoringFailure(request, domain.FacilityFailureRuntimeContextEnded, 0, 0, nil, nil)
	}
	service.commandMu.Lock()
	defer service.commandMu.Unlock()
	service.documentMu.Lock()
	defer service.documentMu.Unlock()

	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return facilityAuthoringFailure(request, domain.FacilityFailureRuntimeContextEnded, 0, 0, nil, nil)
	}
	if service.active.Path == "" || service.active.Session == nil {
		sessionRevision := service.active.SavedRevision
		service.mu.Unlock()
		return facilityAuthoringFailure(request, domain.FacilityFailureRuntimeContextEnded, sessionRevision, 0, nil, nil)
	}

	canonical := domain.CloneSession(*service.active.Session)
	sessionRevision := service.active.SavedRevision
	previousFacilityRevision := facilityRevision(&canonical)
	path := service.active.Path
	if samePath(path, service.locations.BundledDemo) {
		service.mu.Unlock()
		return facilityAuthoringFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			previousFacilityRevision, &canonical, nil,
		)
	}
	if request.ExpectedSessionRevision != sessionRevision || service.active.RequestedRevision != sessionRevision ||
		request.ExpectedFacilityRevision != previousFacilityRevision {
		resultSession := &canonical
		if service.active.RequestedRevision != sessionRevision {
			resultSession = nil
		}
		service.mu.Unlock()
		return facilityAuthoringFailure(
			request, domain.FacilityFailureStaleRevision, sessionRevision,
			previousFacilityRevision, resultSession, nil,
		)
	}
	if strings.TrimSpace(request.CorrelationID) == "" {
		service.mu.Unlock()
		return facilityAuthoringFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			previousFacilityRevision, &canonical, nil,
		)
	}

	candidate := domain.CloneSession(request.Candidate)
	protectFacilityAuthoringCommandStates(&candidate, canonical)
	if canonical.Facility != nil && candidate.Facility == nil {
		service.mu.Unlock()
		return facilityAuthoringFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			previousFacilityRevision, &canonical, []domain.FacilityIssue{{
				Code: domain.FacilityFailureInvalidConfiguration, EntityKind: "facility",
			}},
		)
	}
	protectFacilityAuthoringCurrentValues(canonical.Facility, candidate.Facility)
	if candidate.Facility != nil {
		candidate.Facility.Revision = previousFacilityRevision
	}
	issues := domain.ValidateFacilityAuthoringCandidate(canonical, candidate)
	if len(issues) != 0 {
		service.mu.Unlock()
		return facilityAuthoringFailure(
			request, facilityAuthoringFailureCode(issues), sessionRevision,
			previousFacilityRevision, &canonical, issues,
		)
	}
	if sessionsEqual(candidate, canonical) {
		service.mu.Unlock()
		return facilityAuthoringSuccess(
			request.CorrelationID, false, sessionRevision, previousFacilityRevision, nil, nil, canonical,
		)
	}
	affectedDeviceIDs, affectedConditionIDs := facilityAuthoringAffectedIDs(canonical.Facility, candidate.Facility)

	resultingFacilityRevision := previousFacilityRevision
	if candidate.Facility != nil {
		resultingFacilityRevision++
		candidate.Facility.Revision = resultingFacilityRevision
	}
	saved, err := service.commitFacilityCandidateLocked(path, canonical, candidate)
	if err != nil {
		return facilityAuthoringFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			previousFacilityRevision, &canonical, []domain.FacilityIssue{{
				Code: domain.FacilityFailureInvalidConfiguration, EntityKind: "facility",
			}},
		)
	}
	if !saved.OK {
		return facilityAuthoringFailure(
			request, domain.FacilityFailurePersistenceFailed, saved.SavedRevision,
			previousFacilityRevision, &canonical, nil,
		)
	}
	return facilityAuthoringSuccess(
		request.CorrelationID, true, saved.SavedRevision, previousFacilityRevision,
		affectedDeviceIDs, affectedConditionIDs, candidate,
	)
}

// ResetFacilityDevice restores one authored device and only conditions scoped
// directly to that device.
func (service *Service) ResetFacilityDevice(
	ctx context.Context,
	request FacilityDeviceResetRequest,
) domain.FacilityOperationResult {
	return service.resetFacility(
		ctx,
		request.CorrelationID,
		request.ExpectedFacilityRevision,
		new(request.DeviceID),
	)
}

// ResetFacility restores every device and condition to its authored initial
// value as one atomic session replacement.
func (service *Service) ResetFacility(
	ctx context.Context,
	request FacilityResetRequest,
) domain.FacilityOperationResult {
	return service.resetFacility(ctx, request.CorrelationID, request.ExpectedFacilityRevision, nil)
}

func (service *Service) resetFacility(
	ctx context.Context,
	correlationID string,
	expectedFacilityRevision uint64,
	deviceID *string,
) domain.FacilityOperationResult {
	if service == nil || contextError(ctx) != nil {
		return facilityResetFailure(
			correlationID,
			domain.FacilityFailureRuntimeContextEnded,
			0,
			0,
			nil,
			"request",
			"",
		)
	}
	service.commandMu.Lock()
	defer service.commandMu.Unlock()
	service.documentMu.Lock()
	defer service.documentMu.Unlock()

	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return facilityResetFailure(
			correlationID,
			domain.FacilityFailureRuntimeContextEnded,
			0,
			0,
			nil,
			"request",
			"",
		)
	}
	if service.active.Path == "" || service.active.Session == nil {
		sessionRevision := service.active.SavedRevision
		service.mu.Unlock()
		return facilityResetFailure(
			correlationID,
			domain.FacilityFailureRuntimeContextEnded,
			sessionRevision,
			0,
			nil,
			"session",
			"",
		)
	}

	canonical := domain.CloneSession(*service.active.Session)
	sessionRevision := service.active.SavedRevision
	previousFacilityRevision := facilityRevision(&canonical)
	path := service.active.Path
	fail := func(
		failure domain.FacilityFailureCode,
		entityKind string,
		entityID string,
	) domain.FacilityOperationResult {
		service.mu.Unlock()
		return facilityResetFailure(
			correlationID,
			failure,
			sessionRevision,
			previousFacilityRevision,
			&canonical,
			entityKind,
			entityID,
		)
	}

	if samePath(path, service.locations.BundledDemo) {
		return fail(domain.FacilityFailureInvalidConfiguration, "session", "")
	}
	if canonical.Facility == nil {
		return fail(domain.FacilityFailureMissingReference, "facility", "")
	}
	if expectedFacilityRevision != previousFacilityRevision {
		return fail(domain.FacilityFailureStaleRevision, "facility", "")
	}
	if strings.TrimSpace(correlationID) == "" {
		return fail(domain.FacilityFailureInvalidConfiguration, "request", "")
	}
	if deviceID != nil && strings.TrimSpace(*deviceID) == "" {
		return fail(domain.FacilityFailureInvalidConfiguration, "device", *deviceID)
	}

	candidate := domain.CloneSession(canonical)
	affectedDeviceIDs := make([]string, 0, len(candidate.Facility.Devices))
	affectedConditionIDs := make([]string, 0, len(candidate.Facility.Conditions))
	if deviceID == nil {
		for index := range candidate.Facility.Devices {
			device := &candidate.Facility.Devices[index]
			if device.CurrentStateID == device.InitialStateID {
				continue
			}
			device.CurrentStateID = device.InitialStateID
			affectedDeviceIDs = append(affectedDeviceIDs, device.ID)
		}
		for index := range candidate.Facility.Conditions {
			condition := &candidate.Facility.Conditions[index]
			if condition.CurrentActive == condition.InitialActive {
				continue
			}
			condition.CurrentActive = condition.InitialActive
			affectedConditionIDs = append(affectedConditionIDs, condition.ID)
		}
	} else {
		deviceIndex := slices.IndexFunc(candidate.Facility.Devices, func(device domain.FacilityDevice) bool {
			return device.ID == *deviceID
		})
		if deviceIndex < 0 {
			return fail(domain.FacilityFailureMissingReference, "device", *deviceID)
		}
		device := &candidate.Facility.Devices[deviceIndex]
		if device.CurrentStateID != device.InitialStateID {
			device.CurrentStateID = device.InitialStateID
			affectedDeviceIDs = append(affectedDeviceIDs, device.ID)
		}
		for index := range candidate.Facility.Conditions {
			condition := &candidate.Facility.Conditions[index]
			if condition.Device == nil || condition.Device.DeviceID != *deviceID ||
				condition.CurrentActive == condition.InitialActive {
				continue
			}
			condition.CurrentActive = condition.InitialActive
			affectedConditionIDs = append(affectedConditionIDs, condition.ID)
		}
	}
	slices.Sort(affectedDeviceIDs)
	slices.Sort(affectedConditionIDs)
	if len(affectedDeviceIDs) == 0 && len(affectedConditionIDs) == 0 {
		service.mu.Unlock()
		return worldActionSuccess(
			correlationID,
			false,
			sessionRevision,
			previousFacilityRevision,
			previousFacilityRevision,
			nil,
			nil,
			canonical,
		)
	}

	candidate.Facility.Revision++
	saved, err := service.commitFacilityCandidateLocked(path, canonical, candidate)
	if err != nil {
		return facilityResetFailure(
			correlationID,
			domain.FacilityFailureInvalidConfiguration,
			sessionRevision,
			previousFacilityRevision,
			&canonical,
			"candidate",
			"",
		)
	}
	if !saved.OK {
		return facilityResetFailure(
			correlationID,
			domain.FacilityFailurePersistenceFailed,
			saved.SavedRevision,
			previousFacilityRevision,
			&canonical,
			"persistence",
			"",
		)
	}
	return worldActionSuccess(
		correlationID,
		true,
		saved.SavedRevision,
		previousFacilityRevision,
		candidate.Facility.Revision,
		affectedDeviceIDs,
		affectedConditionIDs,
		candidate,
	)
}

// ApplyWorldAction validates and persists one approved command completion and
// all of its facility transitions as one complete session replacement.
func (service *Service) ApplyWorldAction(ctx context.Context, request WorldActionRequest) domain.FacilityOperationResult {
	request = cloneWorldActionRequest(request)
	if service == nil || contextError(ctx) != nil {
		return worldActionFailure(request, domain.FacilityFailureRuntimeContextEnded, 0, 0, nil, "request", "")
	}
	service.commandMu.Lock()
	defer service.commandMu.Unlock()
	service.documentMu.Lock()
	defer service.documentMu.Unlock()

	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return worldActionFailure(request, domain.FacilityFailureRuntimeContextEnded, 0, 0, nil, "request", "")
	}
	if service.active.Path == "" || service.active.Session == nil {
		revision := service.active.SavedRevision
		service.mu.Unlock()
		return worldActionFailure(request, domain.FacilityFailureRuntimeContextEnded, revision, 0, nil, "session", "")
	}

	canonical := domain.CloneSession(*service.active.Session)
	sessionRevision := service.active.SavedRevision
	path := service.active.Path
	if samePath(path, service.locations.BundledDemo) {
		service.mu.Unlock()
		return worldActionFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			facilityRevision(&canonical), &canonical, "session", "",
		)
	}
	if request.RecoveryConditionID != "" || request.Recovery != nil {
		return service.applyConditionRecoveryLocked(request, canonical, sessionRevision, path)
	}

	terminal := terminalByID(&canonical, request.TerminalID)
	if terminal == nil {
		service.mu.Unlock()
		return worldActionFailure(
			request, domain.FacilityFailureMissingReference, sessionRevision,
			facilityRevision(&canonical), &canonical, "terminal", request.TerminalID,
		)
	}
	command := contentNodeByID(&terminal.Root, request.CommandID)
	if command == nil {
		service.mu.Unlock()
		return worldActionFailure(
			request, domain.FacilityFailureMissingReference, sessionRevision,
			facilityRevision(&canonical), &canonical, "command", request.CommandID,
		)
	}
	if _, completed := terminal.CommandStates[request.CommandID]; completed {
		currentFacilityRevision := facilityRevision(&canonical)
		service.mu.Unlock()
		return worldActionSuccess(
			request.CorrelationID, false, sessionRevision, currentFacilityRevision,
			currentFacilityRevision, nil, nil, canonical,
		)
	}
	if command.Type != domain.NodeCommand || command.StateChange == nil || command.StateChange.FacilityAction == nil {
		service.mu.Unlock()
		return worldActionFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			facilityRevision(&canonical), &canonical, "command", request.CommandID,
		)
	}
	if canonical.Facility == nil {
		service.mu.Unlock()
		return worldActionFailure(
			request, domain.FacilityFailureMissingReference, sessionRevision, 0, &canonical, "facility", "",
		)
	}
	previousFacilityRevision := canonical.Facility.Revision
	if request.ExpectedFacilityRevision != previousFacilityRevision {
		service.mu.Unlock()
		return worldActionFailure(
			request, domain.FacilityFailureStaleRevision, sessionRevision,
			previousFacilityRevision, &canonical, "facility", "",
		)
	}
	if strings.TrimSpace(request.CorrelationID) == "" {
		service.mu.Unlock()
		return worldActionFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			previousFacilityRevision, &canonical, "request", "",
		)
	}

	resolved, conditionValues, affectedDeviceIDs, affectedConditionIDs, failure, entityKind, entityID :=
		resolveWorldTransitions(canonical.Facility, request.Transitions)
	if failure != domain.FacilityFailureUnspecified {
		service.mu.Unlock()
		return worldActionFailure(
			request, failure, sessionRevision, previousFacilityRevision, &canonical, entityKind, entityID,
		)
	}
	authoredTransitions, authoredFailure, authoredEntityKind, authoredEntityID :=
		resolveAuthoredAction(canonical.Facility, command.StateChange.FacilityAction)
	if authoredFailure != domain.FacilityFailureUnspecified {
		service.mu.Unlock()
		return worldActionFailure(
			request, authoredFailure, sessionRevision, previousFacilityRevision,
			&canonical, authoredEntityKind, authoredEntityID,
		)
	}
	if !sameTransitionRequests(request.Transitions, authoredTransitions) {
		service.mu.Unlock()
		return worldActionFailure(
			request, domain.FacilityFailureConflict, sessionRevision,
			previousFacilityRevision, &canonical, "command", request.CommandID,
		)
	}

	candidate := domain.CloneSession(canonical)
	for _, transition := range resolved {
		candidate.Facility.Devices[transition.deviceIndex].CurrentStateID = transition.transition.DestinationStateID
	}
	conditionsByID := make(map[string]int, len(candidate.Facility.Conditions))
	for index, condition := range candidate.Facility.Conditions {
		conditionsByID[condition.ID] = index
	}
	for conditionID, active := range conditionValues {
		candidate.Facility.Conditions[conditionsByID[conditionID]].CurrentActive = active
	}
	candidate.Facility.Revision++
	candidateTerminal := terminalByID(&candidate, request.TerminalID)
	if candidateTerminal.CommandStates == nil {
		candidateTerminal.CommandStates = make(map[string]domain.CommandExecutionState)
	}
	commandState := domain.CommandExecutionState{
		CompletedName: command.StateChange.CompletedName,
		ResultText:    command.Text,
	}
	if change := command.StateChange.EntryContentChange; change != nil {
		commandState.EntryContentChange = &domain.EntryContentChange{
			BlockID: change.BlockID, CompletedText: change.CompletedText,
		}
	}
	candidateTerminal.CommandStates[request.CommandID] = commandState

	saved, err := service.commitFacilityCandidateLocked(path, canonical, candidate)
	if err != nil {
		return worldActionFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			previousFacilityRevision, &canonical, "candidate", "",
		)
	}
	if !saved.OK {
		return worldActionFailure(
			request, domain.FacilityFailurePersistenceFailed, saved.SavedRevision,
			previousFacilityRevision, &canonical, "persistence", "",
		)
	}
	return worldActionSuccess(
		request.CorrelationID, true, saved.SavedRevision, previousFacilityRevision,
		candidate.Facility.Revision, affectedDeviceIDs, affectedConditionIDs, candidate,
	)
}

// applyConditionRecoveryLocked validates one authored condition recovery and
// persists its complete facility candidate. The caller holds service.mu and
// the world-action serialization locks; this method releases service.mu before
// every return.
func (service *Service) applyConditionRecoveryLocked(
	request WorldActionRequest,
	canonical domain.Session,
	sessionRevision uint64,
	path string,
) domain.FacilityOperationResult {
	facility := canonical.Facility
	currentRevision := facilityRevision(&canonical)
	fail := func(
		code domain.FacilityFailureCode,
		entityKind string,
		entityID string,
		issues []domain.FacilityIssue,
	) domain.FacilityOperationResult {
		service.mu.Unlock()
		result := worldActionFailure(
			request, code, sessionRevision, currentRevision, &canonical, entityKind, entityID,
		)
		if issues != nil {
			result.Issues = cloneFacilityIssues(issues)
		}
		return result
	}

	if facility == nil {
		return fail(domain.FacilityFailureMissingReference, "facility", "", nil)
	}
	if request.ExpectedFacilityRevision != currentRevision {
		return fail(domain.FacilityFailureStaleRevision, "facility", "", nil)
	}
	if strings.TrimSpace(request.CorrelationID) == "" ||
		strings.TrimSpace(request.RecoveryConditionID) == "" ||
		request.Recovery == nil || request.TerminalID != "" || request.CommandID != "" || len(request.Transitions) != 0 {
		return fail(domain.FacilityFailureInvalidConfiguration, "request", "", nil)
	}

	conditionIndex := -1
	for index := range facility.Conditions {
		if facility.Conditions[index].ID == request.RecoveryConditionID {
			conditionIndex = index
			break
		}
	}
	if conditionIndex < 0 {
		return fail(domain.FacilityFailureMissingReference, "condition", request.RecoveryConditionID, nil)
	}
	condition := facility.Conditions[conditionIndex]
	if !conditionAllowsRecovery(condition, *request.Recovery) {
		return fail(domain.FacilityFailureInvalidConfiguration, "condition", condition.ID, nil)
	}

	transitions, private, issues := resolveConditionRecovery(facility, *request.Recovery)
	if len(issues) != 0 {
		code := facilityRecoveryFailureCode(issues)
		return fail(code, "condition", condition.ID, issues)
	}
	if !condition.CurrentActive {
		service.mu.Unlock()
		return worldActionSuccess(
			request.CorrelationID, false, sessionRevision, currentRevision, currentRevision,
			nil, nil, canonical,
		)
	}

	var (
		resolved             []resolvedWorldTransition
		conditionValues      map[string]bool
		affectedDeviceIDs    []string
		affectedConditionIDs []string
	)
	if private {
		conditionValues = map[string]bool{condition.ID: false}
		affectedConditionIDs = []string{condition.ID}
	} else {
		var failure domain.FacilityFailureCode
		var entityKind, entityID string
		resolved, conditionValues, affectedDeviceIDs, affectedConditionIDs, failure, entityKind, entityID =
			resolveWorldTransitions(facility, transitions)
		if failure != domain.FacilityFailureUnspecified {
			return fail(failure, entityKind, entityID, nil)
		}
		for _, transition := range resolved {
			if !transition.transition.Recovery {
				return fail(
					domain.FacilityFailureInvalidConfiguration,
					"transition",
					transition.transition.ID,
					nil,
				)
			}
		}
		if clears, exists := conditionValues[condition.ID]; !exists || clears {
			return fail(domain.FacilityFailureInvalidConfiguration, "condition", condition.ID, nil)
		}
	}

	candidate := domain.CloneSession(canonical)
	for _, transition := range resolved {
		candidate.Facility.Devices[transition.deviceIndex].CurrentStateID = transition.transition.DestinationStateID
	}
	conditionsByID := make(map[string]int, len(candidate.Facility.Conditions))
	for index, candidateCondition := range candidate.Facility.Conditions {
		conditionsByID[candidateCondition.ID] = index
	}
	for conditionID, active := range conditionValues {
		candidate.Facility.Conditions[conditionsByID[conditionID]].CurrentActive = active
	}
	candidate.Facility.Revision++

	saved, err := service.commitFacilityCandidateLocked(path, canonical, candidate)
	if err != nil {
		return worldActionFailure(
			request, domain.FacilityFailureInvalidConfiguration, sessionRevision,
			currentRevision, &canonical, "candidate", "",
		)
	}
	if !saved.OK {
		return worldActionFailure(
			request, domain.FacilityFailurePersistenceFailed, saved.SavedRevision,
			currentRevision, &canonical, "persistence", "",
		)
	}
	return worldActionSuccess(
		request.CorrelationID, true, saved.SavedRevision, currentRevision,
		candidate.Facility.Revision, affectedDeviceIDs, affectedConditionIDs, candidate,
	)
}

// commitFacilityCandidateLocked queues one rollback-safe complete-session
// replacement. The caller holds service.mu; this method releases it on every
// return and waits for the queued revision to become durable.
func (service *Service) commitFacilityCandidateLocked(
	path string,
	canonical domain.Session,
	candidate domain.Session,
) (SaveResult, error) {
	data, err := encodeAcceptedSession(candidate)
	if err != nil {
		service.mu.Unlock()
		return SaveResult{}, err
	}

	reply := make(chan SaveResult, 1)
	epoch := service.epoch
	priorRevision := service.active.RequestedRevision
	nextSessionRevision := priorRevision + 1
	if nextSessionRevision <= service.active.SavedRevision {
		nextSessionRevision = service.active.SavedRevision + 1
	}
	service.active.RequestedRevision = nextSessionRevision
	service.active.SaveState = SaveStateSaving
	service.pending = append(service.pending, savePayload{
		epoch: epoch, path: path, revision: nextSessionRevision,
		session: candidate, data: append([]byte(nil), data...),
		rollbackOnFailure: true, priorSession: worldActionSessionPointer(canonical), priorRevision: priorRevision,
	})
	service.waiters = append(service.waiters, saveWaiter{
		epoch: epoch, revision: nextSessionRevision, requestedRevision: nextSessionRevision, reply: reply,
	})
	service.signalWorkerLocked()
	service.mu.Unlock()

	return <-reply, nil
}

func cloneWorldActionRequest(request WorldActionRequest) WorldActionRequest {
	request.Transitions = domain.CloneFacilityTransitionRequests(request.Transitions)
	if request.Recovery != nil {
		recovery := domain.CloneDiagnosticRecoveryReference(*request.Recovery)
		request.Recovery = &recovery
	}
	return request
}

func conditionAllowsRecovery(condition domain.DiagnosticCondition, requested domain.DiagnosticRecoveryReference) bool {
	for _, authored := range condition.Recovery {
		if sameDiagnosticRecoveryReference(authored, requested) {
			return true
		}
	}
	return false
}

func sameDiagnosticRecoveryReference(left, right domain.DiagnosticRecoveryReference) bool {
	switch {
	case left.Transition != nil && right.Transition != nil:
		return left.RecoveryProgramID == nil && left.PrivateOverseerAction == nil &&
			right.RecoveryProgramID == nil && right.PrivateOverseerAction == nil &&
			left.Transition.DeviceID == right.Transition.DeviceID &&
			left.Transition.TransitionID == right.Transition.TransitionID
	case left.RecoveryProgramID != nil && right.RecoveryProgramID != nil:
		return left.Transition == nil && left.PrivateOverseerAction == nil &&
			right.Transition == nil && right.PrivateOverseerAction == nil &&
			*left.RecoveryProgramID == *right.RecoveryProgramID
	case left.PrivateOverseerAction != nil && right.PrivateOverseerAction != nil:
		return left.Transition == nil && left.RecoveryProgramID == nil &&
			right.Transition == nil && right.RecoveryProgramID == nil &&
			*left.PrivateOverseerAction && *right.PrivateOverseerAction
	default:
		return false
	}
}

func resolveConditionRecovery(
	facility *domain.Facility,
	recovery domain.DiagnosticRecoveryReference,
) ([]domain.FacilityTransitionRequest, bool, []domain.FacilityIssue) {
	switch {
	case recovery.Transition != nil:
		return domain.CloneFacilityTransitionRequests([]domain.FacilityTransitionRequest{*recovery.Transition}), false, nil
	case recovery.RecoveryProgramID != nil:
		transitions, issues := domain.ExpandRecoveryProgram(facility, *recovery.RecoveryProgramID)
		return transitions, false, issues
	case recovery.PrivateOverseerAction != nil && *recovery.PrivateOverseerAction:
		return nil, true, nil
	default:
		return nil, false, []domain.FacilityIssue{{
			Code: domain.FacilityFailureInvalidConfiguration, EntityKind: "recovery",
		}}
	}
}

func facilityRecoveryFailureCode(issues []domain.FacilityIssue) domain.FacilityFailureCode {
	if len(issues) == 0 || issues[0].Code == "" || issues[0].Code == domain.FacilityFailureUnspecified {
		return domain.FacilityFailureInvalidConfiguration
	}
	return issues[0].Code
}

func resolveWorldTransitions(
	facility *domain.Facility,
	requests []domain.FacilityTransitionRequest,
) (
	[]resolvedWorldTransition,
	map[string]bool,
	[]string,
	[]string,
	domain.FacilityFailureCode,
	string,
	string,
) {
	if len(requests) == 0 {
		return nil, nil, nil, nil, domain.FacilityFailureInvalidConfiguration, "action", ""
	}
	devicesByID := make(map[string]int, len(facility.Devices))
	for index, device := range facility.Devices {
		devicesByID[device.ID] = index
	}
	conditionsByID := make(map[string]struct{}, len(facility.Conditions))
	for _, condition := range facility.Conditions {
		conditionsByID[condition.ID] = struct{}{}
	}

	seenDevices := make(map[string]struct{}, len(requests))
	resolved := make([]resolvedWorldTransition, 0, len(requests))
	conditionValues := make(map[string]bool)
	affectedDeviceIDs := make([]string, 0, len(requests))
	affectedConditionSet := make(map[string]struct{})
	for _, request := range requests {
		if _, duplicate := seenDevices[request.DeviceID]; duplicate {
			return nil, nil, nil, nil, domain.FacilityFailureDuplicate, "device", request.DeviceID
		}
		seenDevices[request.DeviceID] = struct{}{}
		deviceIndex, exists := devicesByID[request.DeviceID]
		if !exists {
			return nil, nil, nil, nil, domain.FacilityFailureMissingReference, "device", request.DeviceID
		}
		device := facility.Devices[deviceIndex]
		transition, exists := facilityTransitionByID(device, request.TransitionID)
		if !exists {
			return nil, nil, nil, nil, domain.FacilityFailureMissingReference, "transition", request.TransitionID
		}
		if device.CurrentStateID != transition.SourceStateID {
			return nil, nil, nil, nil, domain.FacilityFailureInvalidTransition, "device", request.DeviceID
		}
		for _, precondition := range transition.Preconditions {
			preconditionDeviceIndex, exists := devicesByID[precondition.DeviceID]
			if !exists {
				return nil, nil, nil, nil, domain.FacilityFailureMissingReference, "device", precondition.DeviceID
			}
			if facility.Devices[preconditionDeviceIndex].CurrentStateID != precondition.StateID {
				return nil, nil, nil, nil, domain.FacilityFailurePreconditionFailed, "device", precondition.DeviceID
			}
		}
		for _, effect := range transition.ConditionEffects {
			if _, exists := conditionsByID[effect.ConditionID]; !exists {
				return nil, nil, nil, nil, domain.FacilityFailureMissingReference, "condition", effect.ConditionID
			}
			if active, exists := conditionValues[effect.ConditionID]; exists && active != effect.Active {
				return nil, nil, nil, nil, domain.FacilityFailureConflict, "condition", effect.ConditionID
			}
			conditionValues[effect.ConditionID] = effect.Active
			affectedConditionSet[effect.ConditionID] = struct{}{}
		}
		resolved = append(resolved, resolvedWorldTransition{deviceIndex: deviceIndex, transition: transition})
		affectedDeviceIDs = append(affectedDeviceIDs, request.DeviceID)
	}
	affectedConditionIDs := make([]string, 0, len(affectedConditionSet))
	for conditionID := range affectedConditionSet {
		affectedConditionIDs = append(affectedConditionIDs, conditionID)
	}
	slices.Sort(affectedDeviceIDs)
	slices.Sort(affectedConditionIDs)
	return resolved, conditionValues, affectedDeviceIDs, affectedConditionIDs,
		domain.FacilityFailureUnspecified, "", ""
}

func resolveAuthoredAction(
	facility *domain.Facility,
	action *domain.FacilityActionConfig,
) ([]domain.FacilityTransitionRequest, domain.FacilityFailureCode, string, string) {
	switch {
	case action == nil:
		return nil, domain.FacilityFailureInvalidConfiguration, "action", ""
	case action.Transitions != nil && action.RecoveryProgramID != nil:
		return nil, domain.FacilityFailureInvalidConfiguration, "action", ""
	case action.Transitions != nil:
		return domain.CloneFacilityTransitionRequests(action.Transitions.Transitions),
			domain.FacilityFailureUnspecified, "", ""
	case action.RecoveryProgramID != nil:
		transitions, issues := domain.ExpandRecoveryProgram(facility, *action.RecoveryProgramID)
		if len(issues) == 0 {
			return transitions, domain.FacilityFailureUnspecified, "", ""
		}
		failure := facilityRecoveryFailureCode(issues)
		entityKind := "recovery-program"
		entityID := *action.RecoveryProgramID
		if issues[0].EntityKind != "" {
			entityKind = issues[0].EntityKind
		}
		if issues[0].EntityID != nil {
			entityID = *issues[0].EntityID
		}
		return nil, failure, entityKind, entityID
	default:
		return nil, domain.FacilityFailureInvalidConfiguration, "action", ""
	}
}

func sameTransitionRequests(left, right []domain.FacilityTransitionRequest) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].DeviceID != right[index].DeviceID || left[index].TransitionID != right[index].TransitionID {
			return false
		}
	}
	return true
}

func facilityTransitionByID(
	device domain.FacilityDevice,
	transitionID string,
) (domain.FacilityDeviceTransition, bool) {
	for _, transition := range device.Transitions {
		if transition.ID == transitionID {
			return transition, true
		}
	}
	return domain.FacilityDeviceTransition{}, false
}

func facilityRevision(session *domain.Session) uint64 {
	if session == nil || session.Facility == nil {
		return 0
	}
	return session.Facility.Revision
}

func facilityResetFailure(
	correlationID string,
	failure domain.FacilityFailureCode,
	sessionRevision uint64,
	facilityRevision uint64,
	session *domain.Session,
	entityKind string,
	entityID string,
) domain.FacilityOperationResult {
	return worldActionFailure(
		WorldActionRequest{CorrelationID: correlationID},
		failure,
		sessionRevision,
		facilityRevision,
		session,
		entityKind,
		entityID,
	)
}

func worldActionFailure(
	request WorldActionRequest,
	failure domain.FacilityFailureCode,
	sessionRevision uint64,
	facilityRevision uint64,
	session *domain.Session,
	entityKind string,
	entityID string,
) domain.FacilityOperationResult {
	result := domain.FacilityOperationResult{
		CorrelationID:             request.CorrelationID,
		Failure:                   failure,
		SessionRevision:           sessionRevision,
		PreviousFacilityRevision:  facilityRevision,
		ResultingFacilityRevision: facilityRevision,
	}
	if entityKind != "" {
		issue := domain.FacilityIssue{Code: failure, EntityKind: entityKind}
		if entityID != "" {
			issue.EntityID = new(entityID)
		}
		result.Issues = []domain.FacilityIssue{issue}
	}
	if session != nil {
		result.Session = worldActionSessionPointer(*session)
	}
	return result
}

func worldActionSuccess(
	correlationID string,
	changed bool,
	sessionRevision uint64,
	previousFacilityRevision uint64,
	resultingFacilityRevision uint64,
	affectedDeviceIDs []string,
	affectedConditionIDs []string,
	session domain.Session,
) domain.FacilityOperationResult {
	return domain.FacilityOperationResult{
		OK:                        true,
		Changed:                   changed,
		CorrelationID:             correlationID,
		Failure:                   domain.FacilityFailureUnspecified,
		SessionRevision:           sessionRevision,
		PreviousFacilityRevision:  previousFacilityRevision,
		ResultingFacilityRevision: resultingFacilityRevision,
		AffectedDeviceIDs:         append([]string(nil), affectedDeviceIDs...),
		AffectedConditionIDs:      append([]string(nil), affectedConditionIDs...),
		Session:                   worldActionSessionPointer(session),
	}
}

func worldActionSessionPointer(session domain.Session) *domain.Session {
	clone := domain.CloneSession(session)
	return &clone
}

func protectFacilityAuthoringCommandStates(candidate *domain.Session, canonical domain.Session) {
	canonicalTerminals := make(map[string]domain.Terminal, len(canonical.Terminals))
	for _, terminal := range canonical.Terminals {
		canonicalTerminals[terminal.ID] = terminal
	}
	for index := range candidate.Terminals {
		terminal := &candidate.Terminals[index]
		canonicalTerminal, exists := canonicalTerminals[terminal.ID]
		if !exists {
			terminal.CommandStates = nil
			continue
		}
		terminal.CommandStates = domain.CloneCommandExecutionStates(canonicalTerminal.CommandStates)
	}
}

func protectFacilityAuthoringCurrentValues(current, candidate *domain.Facility) {
	if candidate == nil {
		return
	}
	currentDevices := make(map[string]domain.FacilityDevice)
	currentConditions := make(map[string]domain.DiagnosticCondition)
	if current != nil {
		currentDevices = make(map[string]domain.FacilityDevice, len(current.Devices))
		for _, device := range current.Devices {
			currentDevices[device.ID] = device
		}
		currentConditions = make(map[string]domain.DiagnosticCondition, len(current.Conditions))
		for _, condition := range current.Conditions {
			currentConditions[condition.ID] = condition
		}
	}
	for index := range candidate.Devices {
		device := &candidate.Devices[index]
		if currentDevice, exists := currentDevices[device.ID]; exists {
			device.CurrentStateID = currentDevice.CurrentStateID
		} else {
			device.CurrentStateID = device.InitialStateID
		}
	}
	for index := range candidate.Conditions {
		condition := &candidate.Conditions[index]
		if currentCondition, exists := currentConditions[condition.ID]; exists {
			condition.CurrentActive = currentCondition.CurrentActive
		} else {
			condition.CurrentActive = condition.InitialActive
		}
	}
}

func facilityAuthoringFailureCode(issues []domain.FacilityIssue) domain.FacilityFailureCode {
	for _, issue := range issues {
		switch issue.Code {
		case domain.FacilityFailureMissingReference, domain.FacilityFailureConflict:
			return issue.Code
		}
	}
	return domain.FacilityFailureInvalidConfiguration
}

func facilityAuthoringFailure(
	request FacilityAuthoringRequest,
	failure domain.FacilityFailureCode,
	sessionRevision uint64,
	facilityRevision uint64,
	session *domain.Session,
	issues []domain.FacilityIssue,
) domain.FacilityOperationResult {
	result := domain.FacilityOperationResult{
		CorrelationID:             request.CorrelationID,
		Failure:                   failure,
		Issues:                    cloneFacilityIssues(issues),
		SessionRevision:           sessionRevision,
		PreviousFacilityRevision:  facilityRevision,
		ResultingFacilityRevision: facilityRevision,
	}
	if session != nil {
		result.Session = worldActionSessionPointer(*session)
	}
	return result
}

func facilityAuthoringSuccess(
	correlationID string,
	changed bool,
	sessionRevision uint64,
	previousFacilityRevision uint64,
	affectedDeviceIDs []string,
	affectedConditionIDs []string,
	session domain.Session,
) domain.FacilityOperationResult {
	resultingFacilityRevision := previousFacilityRevision
	if session.Facility != nil {
		resultingFacilityRevision = session.Facility.Revision
	}
	return domain.FacilityOperationResult{
		OK:                        true,
		Changed:                   changed,
		CorrelationID:             correlationID,
		Failure:                   domain.FacilityFailureUnspecified,
		SessionRevision:           sessionRevision,
		PreviousFacilityRevision:  previousFacilityRevision,
		ResultingFacilityRevision: resultingFacilityRevision,
		AffectedDeviceIDs:         slices.Clone(affectedDeviceIDs),
		AffectedConditionIDs:      slices.Clone(affectedConditionIDs),
		Session:                   worldActionSessionPointer(session),
	}
}

func facilityAuthoringAffectedIDs(current, candidate *domain.Facility) ([]string, []string) {
	currentDevices := make(map[string]domain.FacilityDevice)
	candidateDevices := make(map[string]domain.FacilityDevice)
	currentConditions := make(map[string]domain.DiagnosticCondition)
	candidateConditions := make(map[string]domain.DiagnosticCondition)
	if current != nil {
		currentDevices = make(map[string]domain.FacilityDevice, len(current.Devices))
		for _, device := range current.Devices {
			currentDevices[device.ID] = device
		}
		currentConditions = make(map[string]domain.DiagnosticCondition, len(current.Conditions))
		for _, condition := range current.Conditions {
			currentConditions[condition.ID] = condition
		}
	}
	if candidate != nil {
		candidateDevices = make(map[string]domain.FacilityDevice, len(candidate.Devices))
		for _, device := range candidate.Devices {
			candidateDevices[device.ID] = device
		}
		candidateConditions = make(map[string]domain.DiagnosticCondition, len(candidate.Conditions))
		for _, condition := range candidate.Conditions {
			candidateConditions[condition.ID] = condition
		}
	}

	deviceIDs := make(map[string]struct{}, len(currentDevices)+len(candidateDevices))
	for id := range currentDevices {
		deviceIDs[id] = struct{}{}
	}
	for id := range candidateDevices {
		deviceIDs[id] = struct{}{}
	}
	affectedDeviceIDs := make([]string, 0, len(deviceIDs))
	for id := range deviceIDs {
		currentDevice, currentExists := currentDevices[id]
		candidateDevice, candidateExists := candidateDevices[id]
		currentDevice.CurrentStateID = ""
		candidateDevice.CurrentStateID = ""
		if currentExists != candidateExists || !reflect.DeepEqual(currentDevice, candidateDevice) {
			affectedDeviceIDs = append(affectedDeviceIDs, id)
		}
	}
	slices.Sort(affectedDeviceIDs)

	conditionIDs := make(map[string]struct{}, len(currentConditions)+len(candidateConditions))
	for id := range currentConditions {
		conditionIDs[id] = struct{}{}
	}
	for id := range candidateConditions {
		conditionIDs[id] = struct{}{}
	}
	affectedConditionIDs := make([]string, 0, len(conditionIDs))
	for id := range conditionIDs {
		currentCondition, currentExists := currentConditions[id]
		candidateCondition, candidateExists := candidateConditions[id]
		currentCondition.CurrentActive = false
		candidateCondition.CurrentActive = false
		if currentExists != candidateExists || !reflect.DeepEqual(currentCondition, candidateCondition) {
			affectedConditionIDs = append(affectedConditionIDs, id)
		}
	}
	slices.Sort(affectedConditionIDs)
	return affectedDeviceIDs, affectedConditionIDs
}

func cloneFacilityIssues(issues []domain.FacilityIssue) []domain.FacilityIssue {
	if issues == nil {
		return nil
	}
	clone := make([]domain.FacilityIssue, len(issues))
	for index, issue := range issues {
		clone[index] = issue
		if issue.EntityID != nil {
			clone[index].EntityID = new(*issue.EntityID)
		}
		if issue.ReferenceKind != nil {
			clone[index].ReferenceKind = new(*issue.ReferenceKind)
		}
		if issue.ReferenceID != nil {
			clone[index].ReferenceID = new(*issue.ReferenceID)
		}
	}
	return clone
}
