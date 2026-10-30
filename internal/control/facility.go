package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/live"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/nav"
)

// FacilityMutationRequest is the exact approved world action passed to the
// durable session owner. Stable references select authored transitions; they
// do not grant authority to choose destination states directly.
type FacilityMutationRequest struct {
	CorrelationID            string
	TerminalID               string
	CommandID                string
	ExpectedFacilityRevision uint64
	Transitions              []domain.FacilityTransitionRequest
	RecoveryConditionID      string
	Recovery                 *domain.DiagnosticRecoveryReference
}

// FacilityStore is the one-way synchronous durability boundary for an
// approved facility-backed command.
type FacilityStore interface {
	ApplyWorldAction(context.Context, FacilityMutationRequest) domain.FacilityOperationResult
}

// FacilityRecoveryRequest identifies one allowlisted private recovery against
// the exact facility revision inspected by the Overseer.
type FacilityRecoveryRequest struct {
	ConditionID              string
	ExpectedFacilityRevision uint64
	CorrelationID            string
	Recovery                 *domain.DiagnosticRecoveryReference
}

// FacilityDependencyInspectionRequest carries the canonical session snapshot
// supplied by the private App boundary and the facility revision inspected by
// the Overseer.
type FacilityDependencyInspectionRequest struct {
	Session                  domain.Session
	Target                   domain.FacilityEntityReference
	ExpectedFacilityRevision uint64
}

// FacilityDependencyInspectionResult is a detached, read-only dependency
// report tied to one canonical facility revision.
type FacilityDependencyInspectionResult struct {
	OK               bool
	Failure          domain.FacilityFailureCode
	Issues           []domain.FacilityIssue
	FacilityRevision uint64
	Report           *domain.FacilityDependencyReport
}

// FacilityPreviewResult is one detached private effective terminal preview.
// Current and Terminal come from the same snapshot: what players see now and
// what they would see under the override. Differences names each change.
type FacilityPreviewResult struct {
	OK               bool
	Failure          domain.FacilityFailureCode
	Issues           []domain.FacilityIssue
	FacilityRevision uint64
	Terminal         *domain.PublicLiveState
	Current          *domain.PublicLiveState
	Differences      []live.PreviewDifference
}

// FacilityDeviceResetRequest restores one device and its directly scoped
// conditions against the exact current facility revision.
type FacilityDeviceResetRequest struct {
	DeviceID                 string
	ExpectedFacilityRevision uint64
	CorrelationID            string
}

type facilityPreviewLifecycle interface {
	PreviewFacility(*domain.TerminalRuntime, *domain.Facility, domain.FacilityPreview) (*domain.PublicLiveState, []domain.FacilityIssue)
}

// FacilityResetStore is the reset capability implemented by the durable
// facility owner. It remains additive to the player world-action seam.
type FacilityResetStore interface {
	ResetFacilityDevice(context.Context, FacilityDeviceResetRequest) domain.FacilityOperationResult
}

// FacilityAuditAction identifies a bounded facility operation without
// carrying authored display content.
type FacilityAuditAction string

const (
	FacilityAuditActionCommand FacilityAuditAction = "command"
	FacilityAuditActionReset   FacilityAuditAction = "reset"
	FacilityAuditActionRecover FacilityAuditAction = "recover"
)

const (
	facilityAuditEventRequestReceived = "facility.request_received"
	facilityAuditEventDecision        = "facility.decision"
	facilityAuditEventTransition      = "facility.transition"
	facilityAuditEventFailure         = "facility.failure"
	facilityAuditEventRecovery        = "facility.recovery"
	facilityAuditEventReset           = "facility.reset"
)

// FacilityDeviceAuditTransition records one log-safe device state change.
type FacilityDeviceAuditTransition struct {
	DeviceID         string
	PreviousStateID  string
	ResultingStateID string
}

// FacilityConditionAuditTransition records one log-safe diagnostic state change.
type FacilityConditionAuditTransition struct {
	ConditionID     string
	PreviousActive  bool
	ResultingActive bool
}

// FacilityAuditFacts contains only stable identities and typed operation
// metadata suitable for the retained diagnostic log boundary.
type FacilityAuditFacts struct {
	CorrelationID             string
	BroadcastID               domain.BroadcastID
	TerminalID                string
	CommandID                 string
	Action                    FacilityAuditAction
	Outcome                   string
	DeviceIDs                 []string
	ConditionIDs              []string
	DeviceTransitions         []FacilityDeviceAuditTransition
	ConditionTransitions      []FacilityConditionAuditTransition
	ResetScope                string
	Failure                   domain.FacilityFailureCode
	PreviousFacilityRevision  uint64
	ResultingFacilityRevision uint64
	// Session revisions and reset commands are set by scene resets and by the
	// outcome records of pending requests those resets resolve.
	PreviousSessionRevision  uint64
	ResultingSessionRevision uint64
	ResetCommands            []domain.CommandReference
}

// cloneFacilityMutationRequest returns a request detached from caller-owned
// transition slices and nested unknown-field values.
func cloneFacilityMutationRequest(request FacilityMutationRequest) FacilityMutationRequest {
	clone := request
	clone.Transitions = domain.CloneFacilityTransitionRequests(request.Transitions)
	if request.Recovery != nil {
		recovery := domain.CloneDiagnosticRecoveryReference(*request.Recovery)
		clone.Recovery = &recovery
	}
	return clone
}

func cloneFacilityRecoveryRequest(request FacilityRecoveryRequest) FacilityRecoveryRequest {
	clone := request
	if request.Recovery != nil {
		recovery := domain.CloneDiagnosticRecoveryReference(*request.Recovery)
		clone.Recovery = &recovery
	}
	return clone
}

// InspectFacilityDependencies returns a detached dependency report without
// entering the coordinator commit path or emitting effects.
func (service *Service) InspectFacilityDependencies(
	ctx context.Context,
	request FacilityDependencyInspectionRequest,
) FacilityDependencyInspectionResult {
	request.Session = domain.CloneSession(request.Session)
	if service == nil || ctx == nil || ctx.Err() != nil {
		return facilityDependencyInspectionFailure(domain.FacilityFailureRuntimeContextEnded, 0, "request", "")
	}
	service.mu.RLock()
	current := domain.CloneFacility(service.runtime.Facility)
	service.mu.RUnlock()
	currentRevision := facilityRevision(current)
	if current == nil {
		return facilityDependencyInspectionFailure(domain.FacilityFailureMissingReference, currentRevision, "facility", "")
	}
	if request.ExpectedFacilityRevision != currentRevision {
		return facilityDependencyInspectionFailure(domain.FacilityFailureStaleRevision, currentRevision, "facility", "")
	}
	if request.Session.Facility == nil || !reflect.DeepEqual(request.Session.Facility, current) {
		return facilityDependencyInspectionFailure(domain.FacilityFailureStaleRevision, currentRevision, "session", "")
	}
	report, issues := domain.BuildFacilityDependencyReport(request.Session, request.Target)
	result := FacilityDependencyInspectionResult{
		OK: len(issues) == 0, FacilityRevision: currentRevision,
		Issues: cloneFacilityIssues(issues), Report: cloneFacilityDependencyReport(&report),
	}
	if len(issues) != 0 {
		result.Failure = issues[0].Code
	}
	return result
}

// PreviewFacility evaluates one detached override without entering the commit
// path, changing revisions, or emitting player/master effects.
func (service *Service) PreviewFacility(
	ctx context.Context,
	preview domain.FacilityPreview,
) FacilityPreviewResult {
	if service == nil || ctx == nil || ctx.Err() != nil {
		return facilityPreviewFailure(domain.FacilityFailureRuntimeContextEnded, 0, "request", "")
	}
	previewer, ok := service.terminals.(facilityPreviewLifecycle)
	if !ok {
		return facilityPreviewFailure(
			domain.FacilityFailureRuntimeContextEnded,
			service.currentFacilityRevision(),
			"preview",
			"",
		)
	}

	service.mu.RLock()
	currentRevision := facilityRevision(service.runtime.Facility)
	facility := domain.CloneFacility(service.runtime.Facility)
	terminal := previewTerminalRuntime(&service.runtime, preview.TerminalID, service.terminalCatalog)
	service.mu.RUnlock()
	if preview.ExpectedFacilityRevision != currentRevision {
		return facilityPreviewFailure(domain.FacilityFailureStaleRevision, currentRevision, "facility", "")
	}
	if facility == nil {
		return facilityPreviewFailure(domain.FacilityFailureMissingReference, currentRevision, "facility", "")
	}
	if terminal == nil {
		return facilityPreviewFailure(domain.FacilityFailureMissingReference, currentRevision, "terminal", preview.TerminalID)
	}

	projection, issues := previewer.PreviewFacility(terminal, facility, cloneFacilityPreview(preview))
	if len(issues) == 0 && projection == nil {
		issues = []domain.FacilityIssue{{Code: domain.FacilityFailureInvalidConfiguration, EntityKind: "preview"}}
	}
	if len(issues) != 0 {
		return FacilityPreviewResult{
			Failure: issues[0].Code, Issues: cloneFacilityIssues(issues), FacilityRevision: currentRevision,
		}
	}
	// The current view is the same preview with the override set to the
	// value already installed, so both views come from one code path.
	current, issues := previewer.PreviewFacility(terminal, facility, currentFacilityPreview(preview, facility))
	if len(issues) != 0 || current == nil {
		return facilityPreviewFailure(domain.FacilityFailureInvalidConfiguration, currentRevision, "preview", "")
	}
	differences, issues := live.PreviewDifferences(terminal, facility, cloneFacilityPreview(preview))
	if len(issues) != 0 {
		return FacilityPreviewResult{
			Failure: issues[0].Code, Issues: cloneFacilityIssues(issues), FacilityRevision: currentRevision,
		}
	}
	return FacilityPreviewResult{
		OK: true, FacilityRevision: currentRevision, Terminal: clonePublicLiveState(projection),
		Current: clonePublicLiveState(current), Differences: differences,
	}
}

// currentFacilityPreview returns the preview whose override equals the
// installed value of the same device or condition.
func currentFacilityPreview(preview domain.FacilityPreview, facility *domain.Facility) domain.FacilityPreview {
	current := cloneFacilityPreview(preview)
	if current.DeviceState != nil {
		for _, device := range facility.Devices {
			if device.ID == current.DeviceState.DeviceID {
				current.DeviceState.StateID = device.CurrentStateID
			}
		}
	}
	if current.Condition != nil {
		for _, condition := range facility.Conditions {
			if condition.ID == current.Condition.ConditionID {
				current.Condition.Active = condition.CurrentActive
			}
		}
	}
	return current
}

func previewTerminalRuntime(
	runtime *domain.ProcessRuntime,
	terminalID string,
	catalog TerminalCatalog,
) *domain.TerminalRuntime {
	if runtime != nil && runtime.Broadcast != nil {
		if terminal := runtime.Broadcast.TerminalRuntimes[terminalID]; terminal != nil {
			return cloneTerminalRuntime(terminal)
		}
	}
	if catalog == nil {
		return nil
	}
	target, ok := catalog.LookupTerminal(terminalID)
	if !ok {
		return nil
	}
	return &domain.TerminalRuntime{
		TerminalID: target.TerminalID, TerminalName: target.TerminalName,
		AuthoredTree: domain.CloneContentNode(target.Tree), Tree: domain.CloneContentNode(target.Tree),
		CommandStates: cloneCommandStates(target.CommandStates), HackLevel: target.HackLevel,
		IntroText: target.IntroText, Nav: nav.Default(), Lifecycle: domain.TerminalLifecycleSuspended,
	}
}

func cloneFacilityPreview(preview domain.FacilityPreview) domain.FacilityPreview {
	clone := preview
	if preview.DeviceState != nil {
		value := *preview.DeviceState
		clone.DeviceState = &value
	}
	if preview.Condition != nil {
		value := *preview.Condition
		clone.Condition = &value
	}
	return clone
}

func facilityDependencyInspectionFailure(
	code domain.FacilityFailureCode,
	revision uint64,
	entityKind string,
	entityID string,
) FacilityDependencyInspectionResult {
	return FacilityDependencyInspectionResult{
		Failure: code, FacilityRevision: revision,
		Issues: facilityOperationIssues(code, entityKind, entityID),
	}
}

func facilityPreviewFailure(
	code domain.FacilityFailureCode,
	revision uint64,
	entityKind string,
	entityID string,
) FacilityPreviewResult {
	return FacilityPreviewResult{
		Failure: code, FacilityRevision: revision,
		Issues: facilityOperationIssues(code, entityKind, entityID),
	}
}

func facilityOperationIssues(
	code domain.FacilityFailureCode,
	entityKind string,
	entityID string,
) []domain.FacilityIssue {
	issue := domain.FacilityIssue{Code: code, EntityKind: entityKind}
	if entityID != "" {
		issue.EntityID = new(entityID)
	}
	return []domain.FacilityIssue{issue}
}

func cloneFacilityIssues(issues []domain.FacilityIssue) []domain.FacilityIssue {
	if issues == nil {
		return nil
	}
	clone := slices.Clone(issues)
	for index := range clone {
		clone[index].EntityID = cloneString(clone[index].EntityID)
		clone[index].ReferenceKind = cloneString(clone[index].ReferenceKind)
		clone[index].ReferenceID = cloneString(clone[index].ReferenceID)
	}
	return clone
}

func cloneFacilityDependencyReport(report *domain.FacilityDependencyReport) *domain.FacilityDependencyReport {
	if report == nil {
		return nil
	}
	clone := *report
	clone.Target.OwnerID = cloneString(report.Target.OwnerID)
	clone.Dependencies = slices.Clone(report.Dependencies)
	for index := range clone.Dependencies {
		clone.Dependencies[index].TerminalID = cloneString(clone.Dependencies[index].TerminalID)
	}
	return &clone
}

// RecoverFacilityCondition applies one explicitly allowlisted private
// Overseer recovery through the same durable world-action boundary used by
// approved player commands.
func (service *Service) RecoverFacilityCondition(
	ctx context.Context,
	request FacilityRecoveryRequest,
) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
	if service == nil {
		result := facilityRecoveryDecisionResult(request, 0, domain.FacilityFailureRuntimeContextEnded)
		return nil, &result, facilityFailureError(result.Failure)
	}
	if ctx == nil {
		result := facilityRecoveryDecisionResult(
			request,
			service.currentFacilityRevision(),
			domain.FacilityFailureRuntimeContextEnded,
		)
		return service.Snapshot(), &result, facilityFailureError(result.Failure)
	}

	request = cloneFacilityRecoveryRequest(request)
	var state *domain.MasterCoordinationState
	var operation *domain.FacilityOperationResult
	var operationErr error
	committed := service.commit(func(runtime *domain.ProcessRuntime) transition {
		state = masterSnapshot(runtime)
		currentRevision := facilityRevision(runtime.Facility)
		fail := func(code domain.FacilityFailureCode, outcome string) transition {
			result := facilityRecoveryDecisionResult(request, currentRevision, code)
			operation = new(result)
			operationErr = facilityFailureError(code)
			change := transition{}
			change.audit(facilityRecoveryAudit(request, result, outcome, nil, nil))
			return change
		}

		if ctx.Err() != nil {
			return fail(domain.FacilityFailureRuntimeContextEnded, "failed")
		}
		if strings.TrimSpace(request.CorrelationID) == "" || strings.TrimSpace(request.ConditionID) == "" ||
			!privateFacilityRecovery(request.Recovery) {
			return fail(domain.FacilityFailureInvalidConfiguration, "invalid")
		}
		if runtime.Facility == nil {
			return fail(domain.FacilityFailureMissingReference, "failed")
		}
		if request.ExpectedFacilityRevision != currentRevision {
			return fail(domain.FacilityFailureStaleRevision, "stale")
		}
		condition := facilityCondition(runtime.Facility, request.ConditionID)
		if condition == nil {
			return fail(domain.FacilityFailureMissingReference, "failed")
		}
		if !conditionAllowsRecovery(*condition, *request.Recovery) {
			return fail(domain.FacilityFailureInvalidConfiguration, "invalid")
		}
		if service.facilityStore == nil {
			return fail(domain.FacilityFailurePersistenceFailed, "failed")
		}

		mutation := FacilityMutationRequest{
			CorrelationID:            request.CorrelationID,
			ExpectedFacilityRevision: request.ExpectedFacilityRevision,
			RecoveryConditionID:      request.ConditionID,
			Recovery:                 request.Recovery,
		}
		durable := cloneFacilityOperationResult(service.facilityStore.ApplyWorldAction(
			ctx,
			cloneFacilityMutationRequest(mutation),
		))
		if !durable.OK {
			if !validFacilityRecoveryFailure(durable, request) {
				return fail(domain.FacilityFailurePersistenceFailed, "failed")
			}
			failed := normalizedFacilityFailureResult(durable)
			operation = new(cloneFacilityOperationResult(failed))
			operationErr = facilityFailureError(failed.Failure)
			change := transition{}
			change.audit(facilityRecoveryAudit(request, failed, "failed", nil, nil))
			return change
		}
		if !validFacilityRecoverySuccess(durable, request, runtime.Facility) {
			return fail(domain.FacilityFailurePersistenceFailed, "failed")
		}

		operation = new(cloneFacilityOperationResult(durable))
		if !durable.Changed {
			cached := cloneFacilityOperationResult(durable)
			service.lastFacilityResult = &cached
			change := transition{}
			change.audit(facilityRecoveryAudit(request, durable, "unchanged", nil, nil))
			return change
		}

		audit := facilityRecoveryAudit(
			request, durable, "succeeded", runtime.Facility, durable.Session.Facility,
		)
		runtime.Facility = domain.CloneFacility(durable.Session.Facility)
		service.invalidatePendingFacilityAction(runtime)
		cached := cloneFacilityOperationResult(durable)
		service.lastFacilityResult = &cached
		projection := service.reprojectTerminalRuntimes(runtime)
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		if projection != nil {
			effects = append(effects, Effect{Live: projection})
		}
		change := transition{accepted: true, effects: effects}
		change.audit(audit)
		return change
	})
	if state == nil {
		state = service.Snapshot()
	}
	state.Revision = committed.revision
	if operation == nil {
		result := facilityRecoveryDecisionResult(
			request,
			service.currentFacilityRevision(),
			domain.FacilityFailurePersistenceFailed,
		)
		operation = &result
		operationErr = facilityFailureError(result.Failure)
	}
	result := cloneFacilityOperationResult(*operation)
	return domain.CloneMasterCoordinationState(state), &result, operationErr
}

// ResetFacilityDevice restores one device and its directly scoped conditions
// through the shared durable facility owner.
func (service *Service) ResetFacilityDevice(
	ctx context.Context,
	request FacilityDeviceResetRequest,
) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
	return service.resetFacility(ctx, request)
}

func (service *Service) resetFacility(
	ctx context.Context,
	deviceRequest FacilityDeviceResetRequest,
) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
	requestRevision := deviceRequest.ExpectedFacilityRevision
	correlationID := deviceRequest.CorrelationID
	if service == nil {
		result := facilityResetDecisionResult(correlationID, 0, domain.FacilityFailureRuntimeContextEnded)
		return nil, &result, facilityFailureError(result.Failure)
	}
	if ctx == nil {
		result := facilityResetDecisionResult(
			correlationID,
			service.currentFacilityRevision(),
			domain.FacilityFailureRuntimeContextEnded,
		)
		return service.Snapshot(), &result, facilityFailureError(result.Failure)
	}

	var state *domain.MasterCoordinationState
	var operation *domain.FacilityOperationResult
	var operationErr error
	committed := service.commit(func(runtime *domain.ProcessRuntime) transition {
		state = masterSnapshot(runtime)
		currentRevision := facilityRevision(runtime.Facility)
		fail := func(code domain.FacilityFailureCode, outcome string) transition {
			result := facilityResetDecisionResult(correlationID, currentRevision, code)
			operation = new(result)
			operationErr = facilityFailureError(code)
			change := transition{}
			change.audit(facilityResetAudit(deviceRequest, result, outcome, nil, nil))
			return change
		}

		if ctx.Err() != nil {
			return fail(domain.FacilityFailureRuntimeContextEnded, "failed")
		}
		if strings.TrimSpace(correlationID) == "" || strings.TrimSpace(deviceRequest.DeviceID) == "" {
			return fail(domain.FacilityFailureInvalidConfiguration, "invalid")
		}
		if runtime.Facility == nil {
			return fail(domain.FacilityFailureMissingReference, "failed")
		}
		if requestRevision != currentRevision {
			return fail(domain.FacilityFailureStaleRevision, "stale")
		}
		store, ok := service.facilityStore.(FacilityResetStore)
		if !ok {
			return fail(domain.FacilityFailurePersistenceFailed, "failed")
		}

		durable := cloneFacilityOperationResult(store.ResetFacilityDevice(ctx, deviceRequest))
		if !durable.OK {
			if !validFacilityResetFailure(durable, correlationID, currentRevision, runtime.Facility) {
				return fail(domain.FacilityFailurePersistenceFailed, "failed")
			}
			failed := normalizedFacilityFailureResult(durable)
			operation = new(failed)
			operationErr = facilityFailureError(failed.Failure)
			change := transition{}
			change.audit(facilityResetAudit(deviceRequest, failed, "failed", nil, nil))
			return change
		}
		if !validFacilityResetSuccess(durable, deviceRequest, runtime.Facility) {
			return fail(domain.FacilityFailurePersistenceFailed, "failed")
		}

		operation = new(cloneFacilityOperationResult(durable))
		if !durable.Changed {
			change := transition{}
			change.audit(facilityResetAudit(deviceRequest, durable, "unchanged", nil, nil))
			return change
		}

		audit := facilityResetAudit(
			deviceRequest, durable, "succeeded", runtime.Facility, durable.Session.Facility,
		)
		runtime.Facility = domain.CloneFacility(durable.Session.Facility)
		service.invalidatePendingFacilityAction(runtime)
		projection := service.reprojectTerminalRuntimes(runtime)
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		if projection != nil {
			effects = append(effects, Effect{Live: projection})
		}
		change := transition{accepted: true, effects: effects}
		change.audit(audit)
		return change
	})
	if state == nil {
		state = service.Snapshot()
	}
	state.Revision = committed.revision
	if operation == nil {
		result := facilityResetDecisionResult(
			correlationID,
			service.currentFacilityRevision(),
			domain.FacilityFailurePersistenceFailed,
		)
		operation = &result
		operationErr = facilityFailureError(result.Failure)
	}
	result := cloneFacilityOperationResult(*operation)
	return domain.CloneMasterCoordinationState(state), &result, operationErr
}

func facilityResetDecisionResult(
	correlationID string,
	currentRevision uint64,
	failure domain.FacilityFailureCode,
) domain.FacilityOperationResult {
	return domain.FacilityOperationResult{
		CorrelationID:             correlationID,
		Failure:                   failure,
		PreviousFacilityRevision:  currentRevision,
		ResultingFacilityRevision: currentRevision,
	}
}

func validFacilityResetFailure(
	result domain.FacilityOperationResult,
	correlationID string,
	currentRevision uint64,
	current *domain.Facility,
) bool {
	if result.OK || result.Changed || result.CorrelationID != correlationID || !knownFacilityFailure(result.Failure) ||
		result.Failure == domain.FacilityFailureRejected || result.Failure == domain.FacilityFailureUnspecified ||
		result.PreviousFacilityRevision != currentRevision ||
		result.ResultingFacilityRevision != currentRevision ||
		len(result.AffectedDeviceIDs) != 0 || len(result.AffectedConditionIDs) != 0 {
		return false
	}
	if result.Session == nil {
		return true
	}
	return domain.ValidateSession(*result.Session) == nil && reflect.DeepEqual(result.Session.Facility, current)
}

func validFacilityResetSuccess(
	result domain.FacilityOperationResult,
	request FacilityDeviceResetRequest,
	current *domain.Facility,
) bool {
	if current == nil || !result.OK ||
		(result.Failure != "" && result.Failure != domain.FacilityFailureUnspecified) ||
		result.CorrelationID != request.CorrelationID || result.Session == nil || result.Session.Facility == nil ||
		result.PreviousFacilityRevision != request.ExpectedFacilityRevision ||
		result.Session.Facility.Revision != result.ResultingFacilityRevision ||
		domain.ValidateSession(*result.Session) != nil {
		return false
	}

	expected := domain.CloneFacility(current)
	affectedDevices, affectedConditions, ok := applyExpectedFacilityReset(expected, request.DeviceID)
	if !ok {
		return false
	}
	changed := len(affectedDevices) != 0 || len(affectedConditions) != 0
	if result.Changed != changed {
		return false
	}
	if result.Changed {
		expected.Revision++
	}
	return result.ResultingFacilityRevision == expected.Revision &&
		slices.Equal(result.AffectedDeviceIDs, affectedDevices) &&
		slices.Equal(result.AffectedConditionIDs, affectedConditions) &&
		reflect.DeepEqual(result.Session.Facility, expected)
}

func applyExpectedFacilityReset(
	facility *domain.Facility,
	deviceID string,
) ([]string, []string, bool) {
	if facility == nil {
		return nil, nil, false
	}
	affectedDevices := []string(nil)
	affectedConditions := []string(nil)
	deviceIndex := slices.IndexFunc(facility.Devices, func(device domain.FacilityDevice) bool {
		return device.ID == deviceID
	})
	if deviceIndex < 0 {
		return nil, nil, false
	}
	device := &facility.Devices[deviceIndex]
	if device.CurrentStateID != device.InitialStateID {
		device.CurrentStateID = device.InitialStateID
		affectedDevices = append(affectedDevices, device.ID)
	}
	for index := range facility.Conditions {
		condition := &facility.Conditions[index]
		if condition.Device != nil && condition.Device.DeviceID == deviceID &&
			condition.CurrentActive != condition.InitialActive {
			condition.CurrentActive = condition.InitialActive
			affectedConditions = append(affectedConditions, condition.ID)
		}
	}
	slices.Sort(affectedDevices)
	slices.Sort(affectedConditions)
	return affectedDevices, affectedConditions, true
}

func facilityResetAudit(
	request FacilityDeviceResetRequest,
	result domain.FacilityOperationResult,
	outcome string,
	previous *domain.Facility,
	resulting *domain.Facility,
) AuditEvent {
	deviceIDs := sortedUniqueStrings(result.AffectedDeviceIDs)
	if len(deviceIDs) == 0 && request.DeviceID != "" {
		deviceIDs = []string{request.DeviceID}
	}
	facts := FacilityAuditFacts{
		CorrelationID: request.CorrelationID, Action: FacilityAuditActionReset,
		Outcome: outcome, DeviceIDs: deviceIDs,
		ConditionIDs: sortedUniqueStrings(result.AffectedConditionIDs), ResetScope: "device",
		Failure: result.Failure, PreviousFacilityRevision: result.PreviousFacilityRevision,
		ResultingFacilityRevision: result.ResultingFacilityRevision,
	}
	addFacilityAuditTransitions(&facts, previous, resulting)
	return AuditEvent{Name: facilityAuditEventReset, Outcome: outcome, RequestID: request.CorrelationID, Facility: &facts}
}

func facilityRecoveryDecisionResult(
	request FacilityRecoveryRequest,
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

func validFacilityRecoveryFailure(
	result domain.FacilityOperationResult,
	request FacilityRecoveryRequest,
) bool {
	if result.OK || result.Changed || result.CorrelationID != request.CorrelationID ||
		!knownFacilityFailure(result.Failure) || result.Failure == domain.FacilityFailureRejected ||
		result.Failure == domain.FacilityFailureUnspecified ||
		len(result.AffectedDeviceIDs) != 0 || len(result.AffectedConditionIDs) != 0 ||
		result.PreviousFacilityRevision != result.ResultingFacilityRevision {
		return false
	}
	if result.Session == nil {
		return true
	}
	return domain.ValidateSession(*result.Session) == nil &&
		facilityRevision(result.Session.Facility) == result.ResultingFacilityRevision
}

func validFacilityRecoverySuccess(
	result domain.FacilityOperationResult,
	request FacilityRecoveryRequest,
	current *domain.Facility,
) bool {
	if current == nil || !result.OK ||
		(result.Failure != "" && result.Failure != domain.FacilityFailureUnspecified) ||
		result.CorrelationID != request.CorrelationID || result.Session == nil || result.Session.Facility == nil ||
		result.PreviousFacilityRevision != request.ExpectedFacilityRevision ||
		result.Session.Facility.Revision != result.ResultingFacilityRevision ||
		len(result.AffectedDeviceIDs) != 0 || domain.ValidateSession(*result.Session) != nil {
		return false
	}

	expected := domain.CloneFacility(current)
	expectedAffectedConditions := []string(nil)
	if result.Changed {
		expected.Revision++
		condition := facilityCondition(expected, request.ConditionID)
		if condition == nil || !condition.CurrentActive {
			return false
		}
		condition.CurrentActive = false
		expectedAffectedConditions = []string{request.ConditionID}
	}
	if result.ResultingFacilityRevision != expected.Revision ||
		!slices.Equal(result.AffectedConditionIDs, expectedAffectedConditions) {
		return false
	}
	return reflect.DeepEqual(result.Session.Facility, expected)
}

func privateFacilityRecovery(recovery *domain.DiagnosticRecoveryReference) bool {
	return recovery != nil && recovery.Transition == nil && recovery.RecoveryProgramID == nil &&
		recovery.PrivateOverseerAction != nil && *recovery.PrivateOverseerAction
}

func conditionAllowsRecovery(
	condition domain.DiagnosticCondition,
	requested domain.DiagnosticRecoveryReference,
) bool {
	for _, recovery := range condition.Recovery {
		if sameDiagnosticRecoveryReference(recovery, requested) {
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

func facilityCondition(facility *domain.Facility, conditionID string) *domain.DiagnosticCondition {
	if facility == nil {
		return nil
	}
	for index := range facility.Conditions {
		if facility.Conditions[index].ID == conditionID {
			return &facility.Conditions[index]
		}
	}
	return nil
}

func facilityRecoveryAudit(
	request FacilityRecoveryRequest,
	result domain.FacilityOperationResult,
	outcome string,
	previous *domain.Facility,
	resulting *domain.Facility,
) AuditEvent {
	facts := FacilityAuditFacts{
		CorrelationID:             request.CorrelationID,
		Action:                    FacilityAuditActionRecover,
		Outcome:                   outcome,
		DeviceIDs:                 sortedUniqueStrings(result.AffectedDeviceIDs),
		ConditionIDs:              sortedUniqueStrings(result.AffectedConditionIDs),
		Failure:                   result.Failure,
		PreviousFacilityRevision:  result.PreviousFacilityRevision,
		ResultingFacilityRevision: result.ResultingFacilityRevision,
	}
	if len(facts.ConditionIDs) == 0 && request.ConditionID != "" {
		facts.ConditionIDs = []string{request.ConditionID}
	}
	addFacilityAuditTransitions(&facts, previous, resulting)
	return AuditEvent{Name: facilityAuditEventRecovery, Outcome: outcome, RequestID: request.CorrelationID, Facility: &facts}
}

func (service *Service) invalidatePendingFacilityAction(runtime *domain.ProcessRuntime) {
	if runtime == nil || runtime.PendingCommandExecution == nil || runtime.PendingCommandExecution.FacilityAction == nil {
		return
	}
	pending := runtime.PendingCommandExecution
	stale := facilityDecisionResult(pending, runtime.Facility, domain.FacilityFailureStaleRevision)
	if runtime.Broadcast != nil {
		if terminal := runtime.Broadcast.TerminalRuntimes[pending.TerminalID]; terminal != nil {
			terminal.CommandExecution = &domain.CommandExecutionPresentation{
				Phase: domain.CommandExecutionPhaseRejected, CommandID: pending.CommandID,
			}
		}
	}
	cached := cloneFacilityOperationResult(stale)
	service.lastFacilityResult = &cached
	runtime.PendingCommandExecution = nil
}

func requestedFacilityCapability(
	command domain.RuntimeCommand,
	authored *domain.ContentNode,
	commandSelected bool,
) (domain.FacilityCapability, bool) {
	if commandSelected {
		return domain.CommandFacilityCapability(*authored), true
	}
	switch {
	case command.Kind == domain.RuntimeCommandNavigate && command.Action == "entry":
		return domain.FacilityCapabilityViewEntry, true
	case command.Kind == domain.RuntimeCommandGuess || command.Kind == domain.RuntimeCommandActivatePattern:
		return domain.FacilityCapabilityHack, true
	default:
		return domain.FacilityCapabilityUnspecified, false
	}
}

func facilityCapabilityBlocked(
	facility *domain.Facility,
	terminal *domain.TerminalRuntime,
	capability domain.FacilityCapability,
) bool {
	if facility == nil || terminal == nil || capability == domain.FacilityCapabilityUnspecified {
		return false
	}
	authored := terminal.AuthoredTree
	if authored.ID == "" {
		authored = terminal.Tree
	}
	for _, condition := range facility.Conditions {
		if !condition.CurrentActive || !diagnosticConditionAppliesToTerminal(condition, authored, facility, terminal.TerminalID) {
			continue
		}
		for _, effect := range condition.Effects {
			if effect.CapabilityBlock != nil && effect.CapabilityBlock.Capability == capability {
				return true
			}
		}
	}
	return false
}

func diagnosticConditionAppliesToTerminal(
	condition domain.DiagnosticCondition,
	authored domain.ContentNode,
	facility *domain.Facility,
	terminalID string,
) bool {
	if (condition.Device == nil) == (condition.Terminal == nil) {
		return false
	}
	if condition.Terminal != nil {
		return condition.Terminal.TerminalID == terminalID
	}
	return controlContentReferencesDevice(authored, facility, condition.Device.DeviceID)
}

func controlContentReferencesDevice(
	node domain.ContentNode,
	facility *domain.Facility,
	deviceID string,
) bool {
	if controlEqualityReferencesDevice(node.VisibleWhen, deviceID) ||
		controlEqualityReferencesDevice(node.AvailableWhen, deviceID) {
		return true
	}
	for _, variant := range node.FacilityNameVariants {
		if variant.When.DeviceID == deviceID {
			return true
		}
	}
	for _, block := range node.Blocks {
		for _, variant := range block.FacilityTextVariants {
			if variant.When.DeviceID == deviceID {
				return true
			}
		}
	}
	if node.StateChange != nil && node.StateChange.FacilityAction != nil {
		action := node.StateChange.FacilityAction
		if action.Transitions != nil {
			for _, request := range action.Transitions.Transitions {
				if request.DeviceID == deviceID {
					return true
				}
			}
		}
		if action.RecoveryProgramID != nil {
			requests, issues := domain.ExpandRecoveryProgram(facility, *action.RecoveryProgramID)
			if len(issues) == 0 {
				for _, request := range requests {
					if request.DeviceID == deviceID {
						return true
					}
				}
			}
		}
	}
	for _, child := range node.Children {
		if controlContentReferencesDevice(child, facility, deviceID) {
			return true
		}
	}
	return false
}

func controlEqualityReferencesDevice(equality *domain.FacilityStateEquality, deviceID string) bool {
	return equality != nil && equality.DeviceID == deviceID
}

func (service *Service) currentFacilityRevision() uint64 {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return facilityRevision(service.runtime.Facility)
}

func facilityRevision(facility *domain.Facility) uint64 {
	if facility == nil {
		return 0
	}
	return facility.Revision
}

func knownFacilityFailure(failure domain.FacilityFailureCode) bool {
	switch failure {
	case domain.FacilityFailureMissingReference,
		domain.FacilityFailureInvalidTransition,
		domain.FacilityFailurePreconditionFailed,
		domain.FacilityFailureStaleRevision,
		domain.FacilityFailureConflict,
		domain.FacilityFailureDuplicate,
		domain.FacilityFailureInvalidConfiguration,
		domain.FacilityFailurePersistenceFailed,
		domain.FacilityFailureRuntimeContextEnded:
		return true
	default:
		return false
	}
}

func canRefreshFacilityAuthoringRuntimes(
	runtime *domain.ProcessRuntime,
	session *domain.Session,
	lifecycle TerminalRuntimeLifecycle,
) bool {
	if runtime == nil || runtime.Broadcast == nil || len(runtime.Broadcast.TerminalRuntimes) == 0 {
		return true
	}
	if lifecycle == nil || session == nil {
		return false
	}
	for terminalID, terminal := range runtime.Broadcast.TerminalRuntimes {
		if terminal != nil && terminalByStableID(session, terminalID) == nil {
			return false
		}
	}
	return true
}

func refreshFacilityAuthoringRuntimes(
	runtime *domain.ProcessRuntime,
	session *domain.Session,
	lifecycle TerminalRuntimeLifecycle,
) {
	if runtime == nil || runtime.Broadcast == nil || session == nil || lifecycle == nil {
		return
	}
	for _, terminalID := range slices.Sorted(maps.Keys(runtime.Broadcast.TerminalRuntimes)) {
		terminal := runtime.Broadcast.TerminalRuntimes[terminalID]
		durable := terminalByStableID(session, terminalID)
		if terminal == nil || durable == nil {
			continue
		}
		lifecycle.UpdateRuntime(terminal, domain.TerminalTarget{
			TerminalID: terminalID, TerminalName: durable.Name,
			Tree: domain.CloneContentNode(durable.Root), CommandStates: cloneCommandStates(durable.CommandStates),
			HackLevel: durable.HackLevel, IntroText: durable.IntroText,
		})
	}
}

// cloneFacilityOperationResult detaches a store result before it can become
// coordinator state or cross a publication boundary.
func cloneFacilityOperationResult(result domain.FacilityOperationResult) domain.FacilityOperationResult {
	clone := result
	if result.Issues != nil {
		clone.Issues = slices.Clone(result.Issues)
		for index := range result.Issues {
			clone.Issues[index].EntityID = cloneString(result.Issues[index].EntityID)
			clone.Issues[index].ReferenceKind = cloneString(result.Issues[index].ReferenceKind)
			clone.Issues[index].ReferenceID = cloneString(result.Issues[index].ReferenceID)
		}
	}
	clone.AffectedDeviceIDs = slices.Clone(result.AffectedDeviceIDs)
	clone.AffectedConditionIDs = slices.Clone(result.AffectedConditionIDs)
	clone.ResetCommands = slices.Clone(result.ResetCommands)
	if result.Session != nil {
		session := domain.CloneSession(*result.Session)
		clone.Session = &session
	}
	return clone
}

// normalizedFacilityFailureResult removes state-bearing fields that are not
// authoritative on a failed facility operation.
func normalizedFacilityFailureResult(result domain.FacilityOperationResult) domain.FacilityOperationResult {
	result.OK = false
	result.Changed = false
	result.AffectedDeviceIDs = nil
	result.AffectedConditionIDs = nil
	result.ResetCommands = nil
	result.Session = nil
	return result
}

// pendingFacilityAction resolves one authored action into the immutable intent
// captured by the existing private command-approval lifecycle.
func pendingFacilityAction(
	facility *domain.Facility,
	action *domain.FacilityActionConfig,
) (*domain.PendingFacilityAction, domain.FacilityFailureCode) {
	if facility == nil {
		return nil, domain.FacilityFailureMissingReference
	}
	transitions, recoveryProgramID, failure := resolveFacilityAction(facility, action)
	if failure != domain.FacilityFailureUnspecified {
		return nil, failure
	}

	devices := make(map[string]*domain.FacilityDevice, len(facility.Devices))
	for index := range facility.Devices {
		device := &facility.Devices[index]
		devices[device.ID] = device
	}
	expectedSources := make([]domain.FacilityStateEquality, 0, len(transitions))
	affectedConditions := make([]string, 0)
	seenDevices := make(map[string]struct{}, len(transitions))
	seenConditions := make(map[string]struct{})
	for _, request := range transitions {
		if _, duplicate := seenDevices[request.DeviceID]; duplicate {
			return nil, domain.FacilityFailureConflict
		}
		seenDevices[request.DeviceID] = struct{}{}
		device := devices[request.DeviceID]
		if device == nil {
			return nil, domain.FacilityFailureMissingReference
		}
		transition := facilityTransition(device, request.TransitionID)
		if transition == nil {
			return nil, domain.FacilityFailureInvalidTransition
		}
		expectedSources = append(expectedSources, domain.FacilityStateEquality{
			DeviceID: request.DeviceID,
			StateID:  device.CurrentStateID,
		})
		for _, effect := range transition.ConditionEffects {
			if _, seen := seenConditions[effect.ConditionID]; seen {
				continue
			}
			seenConditions[effect.ConditionID] = struct{}{}
			affectedConditions = append(affectedConditions, effect.ConditionID)
		}
	}
	slices.Sort(affectedConditions)

	pending := &domain.PendingFacilityAction{
		ExpectedFacilityRevision: facility.Revision,
		ActionFingerprint:        facilityActionFingerprint(action, transitions),
		TransitionRequests:       domain.CloneFacilityTransitionRequests(transitions),
		ExpectedSourceStates:     cloneFacilityEqualities(expectedSources),
		AffectedConditionIDs:     slices.Clone(affectedConditions),
		RecoveryProgramID:        cloneString(recoveryProgramID),
	}
	return pending, domain.FacilityFailureUnspecified
}

// revalidatePendingFacilityAction compares the pending intent with the current
// facility and authored command action immediately before durability.
func revalidatePendingFacilityAction(
	facility *domain.Facility,
	action *domain.FacilityActionConfig,
	pending *domain.PendingFacilityAction,
) domain.FacilityFailureCode {
	if pending == nil {
		return domain.FacilityFailureInvalidConfiguration
	}
	if facility == nil {
		return domain.FacilityFailureMissingReference
	}
	if facility.Revision != pending.ExpectedFacilityRevision {
		return domain.FacilityFailureStaleRevision
	}
	current, failure := pendingFacilityAction(facility, action)
	if failure != domain.FacilityFailureUnspecified {
		return domain.FacilityFailureConflict
	}
	if current.ActionFingerprint != pending.ActionFingerprint ||
		!equalFacilityTransitionRequests(current.TransitionRequests, pending.TransitionRequests) ||
		!equalFacilityStateEqualities(current.ExpectedSourceStates, pending.ExpectedSourceStates) ||
		!slices.Equal(current.AffectedConditionIDs, pending.AffectedConditionIDs) {
		return domain.FacilityFailureConflict
	}
	return domain.FacilityFailureUnspecified
}

// facilityDecisionResult constructs the stable, redacted result for a
// coordinator-side decision that does not reach durable facility mutation.
func facilityDecisionResult(
	pending *domain.PendingCommandExecution,
	facility *domain.Facility,
	failure domain.FacilityFailureCode,
) domain.FacilityOperationResult {
	result := domain.FacilityOperationResult{Failure: failure}
	if pending != nil {
		result.CorrelationID = pending.RequestID
		if pending.FacilityAction != nil {
			result.PreviousFacilityRevision = pending.FacilityAction.ExpectedFacilityRevision
			result.ResultingFacilityRevision = pending.FacilityAction.ExpectedFacilityRevision
			deviceIDs := make([]string, len(pending.FacilityAction.TransitionRequests))
			for index, request := range pending.FacilityAction.TransitionRequests {
				deviceIDs[index] = request.DeviceID
			}
			result.AffectedDeviceIDs = sortedUniqueStrings(deviceIDs)
			result.AffectedConditionIDs = sortedUniqueStrings(pending.FacilityAction.AffectedConditionIDs)
		}
	}
	if facility != nil {
		result.PreviousFacilityRevision = facility.Revision
		result.ResultingFacilityRevision = facility.Revision
	}
	return result
}

func validFacilityStoreSuccess(
	result domain.FacilityOperationResult,
	pending *domain.PendingCommandExecution,
	authored *domain.ContentNode,
) (*domain.Terminal, bool) {
	if pending == nil || pending.FacilityAction == nil || !result.OK ||
		(result.Failure != "" && result.Failure != domain.FacilityFailureUnspecified) || result.CorrelationID != pending.RequestID ||
		result.Session == nil || result.Session.Facility == nil ||
		result.PreviousFacilityRevision != pending.FacilityAction.ExpectedFacilityRevision ||
		result.Session.Facility.Revision != result.ResultingFacilityRevision {
		return nil, false
	}
	wantResultingRevision := result.PreviousFacilityRevision
	if result.Changed {
		wantResultingRevision++
	}
	if result.ResultingFacilityRevision != wantResultingRevision {
		return nil, false
	}
	expectedDeviceIDs := make([]string, len(pending.FacilityAction.TransitionRequests))
	for index, request := range pending.FacilityAction.TransitionRequests {
		expectedDeviceIDs[index] = request.DeviceID
	}
	if !slices.Equal(sortedUniqueStrings(result.AffectedDeviceIDs), sortedUniqueStrings(expectedDeviceIDs)) ||
		!slices.Equal(sortedUniqueStrings(result.AffectedConditionIDs), sortedUniqueStrings(pending.FacilityAction.AffectedConditionIDs)) {
		return nil, false
	}
	if err := domain.ValidateSession(*result.Session); err != nil {
		return nil, false
	}
	durableTerminal := terminalByStableID(result.Session, pending.TerminalID)
	if durableTerminal == nil {
		return nil, false
	}
	completed, exists := durableTerminal.CommandStates[pending.CommandID]
	if !exists || !durableCommandStateMatchesAuthored(completed, authored) {
		return nil, false
	}
	return durableTerminal, true
}

func facilityMutationRequest(pending *domain.PendingCommandExecution) (FacilityMutationRequest, bool) {
	if pending == nil || pending.FacilityAction == nil {
		return FacilityMutationRequest{}, false
	}
	return FacilityMutationRequest{
		CorrelationID:            pending.RequestID,
		TerminalID:               pending.TerminalID,
		CommandID:                pending.CommandID,
		ExpectedFacilityRevision: pending.FacilityAction.ExpectedFacilityRevision,
		Transitions:              domain.CloneFacilityTransitionRequests(pending.FacilityAction.TransitionRequests),
	}, true
}

func resolveFacilityAction(
	facility *domain.Facility,
	action *domain.FacilityActionConfig,
) ([]domain.FacilityTransitionRequest, *string, domain.FacilityFailureCode) {
	if action == nil || (action.Transitions == nil) == (action.RecoveryProgramID == nil) {
		return nil, nil, domain.FacilityFailureInvalidConfiguration
	}
	if action.Transitions != nil {
		if len(action.Transitions.Transitions) == 0 {
			return nil, nil, domain.FacilityFailureInvalidConfiguration
		}
		return domain.CloneFacilityTransitionRequests(action.Transitions.Transitions), nil, domain.FacilityFailureUnspecified
	}
	programID := strings.TrimSpace(*action.RecoveryProgramID)
	if programID == "" {
		return nil, nil, domain.FacilityFailureInvalidConfiguration
	}
	transitions, issues := domain.ExpandRecoveryProgram(facility, programID)
	if len(issues) != 0 {
		failure := issues[0].Code
		if failure == "" || failure == domain.FacilityFailureUnspecified {
			failure = domain.FacilityFailureInvalidConfiguration
		}
		return nil, nil, failure
	}
	return transitions, new(programID), domain.FacilityFailureUnspecified
}

func facilityTransition(device *domain.FacilityDevice, transitionID string) *domain.FacilityDeviceTransition {
	for index := range device.Transitions {
		if device.Transitions[index].ID == transitionID {
			return &device.Transitions[index]
		}
	}
	return nil
}

func facilityActionFingerprint(
	action *domain.FacilityActionConfig,
	transitions []domain.FacilityTransitionRequest,
) string {
	type transitionIdentity struct {
		DeviceID     string `json:"device_id"`
		TransitionID string `json:"transition_id"`
	}
	type fingerprintInput struct {
		Kind              string               `json:"kind"`
		RecoveryProgramID string               `json:"recovery_program_id,omitempty"`
		Transitions       []transitionIdentity `json:"transitions"`
	}
	input := fingerprintInput{Kind: "transitions", Transitions: make([]transitionIdentity, len(transitions))}
	if action != nil && action.RecoveryProgramID != nil {
		input.Kind = "recovery-program"
		input.RecoveryProgramID = strings.TrimSpace(*action.RecoveryProgramID)
	}
	for index, transition := range transitions {
		input.Transitions[index] = transitionIdentity{
			DeviceID: transition.DeviceID, TransitionID: transition.TransitionID,
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func facilityAuditFacts(
	pending *domain.PendingCommandExecution,
	result domain.FacilityOperationResult,
	action FacilityAuditAction,
	outcome string,
) FacilityAuditFacts {
	facts := FacilityAuditFacts{
		Action:                    action,
		Outcome:                   outcome,
		Failure:                   result.Failure,
		PreviousFacilityRevision:  result.PreviousFacilityRevision,
		ResultingFacilityRevision: result.ResultingFacilityRevision,
		DeviceIDs:                 sortedUniqueStrings(result.AffectedDeviceIDs),
		ConditionIDs:              sortedUniqueStrings(result.AffectedConditionIDs),
	}
	if pending != nil {
		facts.CorrelationID = pending.RequestID
		facts.BroadcastID = pending.BroadcastID
		facts.TerminalID = pending.TerminalID
		facts.CommandID = pending.CommandID
		if pending.FacilityAction != nil {
			if facts.PreviousFacilityRevision == 0 && facts.ResultingFacilityRevision == 0 {
				facts.PreviousFacilityRevision = pending.FacilityAction.ExpectedFacilityRevision
				facts.ResultingFacilityRevision = pending.FacilityAction.ExpectedFacilityRevision
			}
			if len(facts.DeviceIDs) == 0 {
				deviceIDs := make([]string, len(pending.FacilityAction.TransitionRequests))
				for index, request := range pending.FacilityAction.TransitionRequests {
					deviceIDs[index] = request.DeviceID
				}
				facts.DeviceIDs = sortedUniqueStrings(deviceIDs)
			}
			if len(facts.ConditionIDs) == 0 {
				facts.ConditionIDs = sortedUniqueStrings(pending.FacilityAction.AffectedConditionIDs)
			}
		}
	}
	if facts.CorrelationID == "" {
		facts.CorrelationID = result.CorrelationID
	}
	return facts
}

func addFacilityAuditTransitions(
	facts *FacilityAuditFacts,
	previous *domain.Facility,
	resulting *domain.Facility,
) {
	if facts == nil || previous == nil || resulting == nil {
		return
	}
	previousDevices := make(map[string]string, len(previous.Devices))
	resultingDevices := make(map[string]string, len(resulting.Devices))
	for _, device := range previous.Devices {
		previousDevices[device.ID] = device.CurrentStateID
	}
	for _, device := range resulting.Devices {
		resultingDevices[device.ID] = device.CurrentStateID
	}
	for _, deviceID := range facts.DeviceIDs {
		previousState, hadPrevious := previousDevices[deviceID]
		resultingState, hasResulting := resultingDevices[deviceID]
		if hadPrevious && hasResulting && previousState != resultingState {
			facts.DeviceTransitions = append(facts.DeviceTransitions, FacilityDeviceAuditTransition{
				DeviceID: deviceID, PreviousStateID: previousState, ResultingStateID: resultingState,
			})
		}
	}

	previousConditions := make(map[string]bool, len(previous.Conditions))
	resultingConditions := make(map[string]bool, len(resulting.Conditions))
	for _, condition := range previous.Conditions {
		previousConditions[condition.ID] = condition.CurrentActive
	}
	for _, condition := range resulting.Conditions {
		resultingConditions[condition.ID] = condition.CurrentActive
	}
	for _, conditionID := range facts.ConditionIDs {
		previousActive, hadPrevious := previousConditions[conditionID]
		resultingActive, hasResulting := resultingConditions[conditionID]
		if hadPrevious && hasResulting && previousActive != resultingActive {
			facts.ConditionTransitions = append(facts.ConditionTransitions, FacilityConditionAuditTransition{
				ConditionID: conditionID, PreviousActive: previousActive, ResultingActive: resultingActive,
			})
		}
	}
}

func cloneFacilityEqualities(values []domain.FacilityStateEquality) []domain.FacilityStateEquality {
	if values == nil {
		return nil
	}
	clone := make([]domain.FacilityStateEquality, len(values))
	for index := range values {
		clone[index] = domain.FacilityStateEquality{
			DeviceID: values[index].DeviceID,
			StateID:  values[index].StateID,
		}
	}
	return clone
}

func equalFacilityTransitionRequests(first, second []domain.FacilityTransitionRequest) bool {
	if (first == nil) != (second == nil) || len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index].DeviceID != second[index].DeviceID || first[index].TransitionID != second[index].TransitionID {
			return false
		}
	}
	return true
}

func equalFacilityStateEqualities(first, second []domain.FacilityStateEquality) bool {
	if (first == nil) != (second == nil) || len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index].DeviceID != second[index].DeviceID || first[index].StateID != second[index].StateID {
			return false
		}
	}
	return true
}

func sortedUniqueStrings(values []string) []string {
	if values == nil {
		return nil
	}
	clone := slices.Clone(values)
	slices.Sort(clone)
	return slices.Compact(clone)
}
