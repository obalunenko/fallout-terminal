package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/control"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/live"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/nav"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/player"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
)

const (
	fixtureAddress           = "127.0.0.1:34119"
	fixtureEdgeUsername      = "players"
	fixtureEdgePassword      = "password-long-enough"
	fixtureRandomSeed        = uint64(0x435254)
	fixtureApprovalRequestID = "approval-request-1"
	fixturePlayerRevision    = uint64(41)

	fixtureFacilityTerminalID               = "terminal-facility-security"
	fixtureFacilityCommandID                = "command-open-security-door"
	fixtureFacilityDoorID                   = "device-security-door"
	fixtureFacilityAlarmID                  = "device-security-alarm"
	fixtureFacilityCondition                = "condition-security-authorization"
	fixtureFacilityPowerID                  = "device-primary-power"
	fixtureFacilityCoolingID                = "device-reactor-cooling"
	fixtureFacilityReactorID                = "device-main-reactor"
	fixtureFacilityNetworkID                = "device-operations-network"
	fixtureFacilityOfflineConditionID       = "condition-security-offline"
	fixtureFacilityUnpoweredConditionID     = "condition-reactor-unpowered"
	fixtureFacilityNetworkConditionID       = "condition-network-isolated"
	fixtureFacilityStorageConditionID       = "condition-archive-damaged"
	fixtureFacilityDisplayConditionID       = "condition-reactor-display"
	fixtureFacilityCustomConditionID        = "condition-cooling-contamination"
	fixtureFacilityNetworkRecoveryProgramID = "program-network-recovery"
	fixtureFacilityDiagnosticTerminalID     = "terminal-facility-diagnostics"
	fixtureFacilityDiagnosticRemoteID       = "terminal-facility-diagnostics-remote"

	fixtureFacilityReactorTerminalID     = "terminal-facility-reactor"
	fixtureFacilityMaintenanceTerminalID = "terminal-facility-maintenance"
	fixtureFacilityNetworkTerminalID     = "terminal-facility-network"
	fixtureFacilityArchiveTerminalID     = "terminal-facility-archive"

	fixtureFacilityOperationsGroupID  = "group-facility-operations"
	fixtureFacilityEngineeringGroupID = "group-facility-engineering"
	fixtureFacilityRecordsGroupID     = "group-facility-records"
)

type ids struct{ next atomic.Uint64 }

type fixtureRandom struct {
	mu     sync.Mutex
	state  uint64
	forced []int
}

type fixtureCommandStateStore struct {
	mu            sync.Mutex
	states        map[string]domain.CommandExecutionState
	revision      uint64
	executeWrites int
	failNext      bool
}

type fixtureFacilityPlayerState struct {
	mu sync.Mutex

	scenario           string
	projectionSession  *domain.Session
	resetNavigationFor string
	revision           uint64
	deviceStates       map[string]string
	lastRequestID      string
	lastResult         *fixtureFacilityResult
	resolutionAttempts int
	durableWrites      int
	successfulActions  int
	duplicateResults   int
}

type fixtureFacilityDiagnosticState struct {
	mu sync.Mutex

	active   bool
	scenario string
	session  domain.Session
	audit    fixtureFacilityDiagnosticAudit
}

type fixtureFacilityDiagnosticAudit struct {
	DurableWrites        int `json:"durableWrites"`
	ApprovedRecoveries   int `json:"approvedRecoveries"`
	PrivateRecoveries    int `json:"privateRecoveries"`
	ProjectionReplays    int `json:"projectionReplays"`
	VisualStateMutations int `json:"visualStateMutations"`
}

type fixtureFacilityDiagnosticSnapshot struct {
	ActiveCondition       map[string]string                   `json:"activeCondition"`
	Facility              fixtureFacilitySnapshot             `json:"facility"`
	BlockedCapabilities   []domain.FacilityCapability         `json:"blockedCapabilities"`
	PresentationEffects   []domain.TerminalPresentationEffect `json:"presentationEffects"`
	AuthoredRecordDigest  string                              `json:"authoredRecordDigest"`
	AuthoredContentDigest string                              `json:"authoredContentDigest"`
	Audit                 fixtureFacilityDiagnosticAudit      `json:"audit"`
}

type fixtureFacilityResult struct {
	OK                        bool                       `json:"ok"`
	Changed                   bool                       `json:"changed"`
	CorrelationID             string                     `json:"correlationId"`
	Failure                   domain.FacilityFailureCode `json:"failure"`
	PreviousFacilityRevision  uint64                     `json:"previousFacilityRevision"`
	ResultingFacilityRevision uint64                     `json:"resultingFacilityRevision"`
	AffectedDeviceIDs         []string                   `json:"affectedDeviceIds"`
	AffectedConditionIDs      []string                   `json:"affectedConditionIds"`
}

type fixtureFacilityStateResponse struct {
	Facility           fixtureFacilitySnapshot `json:"facility"`
	LastFacilityResult *fixtureFacilityResult  `json:"lastFacilityResult,omitempty"`
	Audit              fixtureFacilityAudit    `json:"audit"`
}

type fixtureFacilitySnapshot struct {
	Revision        uint64            `json:"revision"`
	DeviceStates    map[string]string `json:"deviceStates"`
	ConditionStates map[string]bool   `json:"conditionStates"`
	DeviceIDs       []string          `json:"deviceIds"`
	TerminalCount   int               `json:"terminalCount"`
	GroupCount      int               `json:"groupCount"`
}

type fixtureFacilityAudit struct {
	ResolutionAttempts     int `json:"resolutionAttempts"`
	DurableWrites          int `json:"durableWrites"`
	SuccessfulWorldActions int `json:"successfulWorldActions"`
	DuplicateResults       int `json:"duplicateResults"`
}

type fixtureFacilityAuthoringState struct {
	mu sync.Mutex

	session         domain.Session
	sessionRevision uint64
	saveCalls       int
	repairWrites    int
	previewCalls    int
	resetWrites     int
	recoveryWrites  int
	publishedEvents int
	nextFailure     string
}

type fixtureFacilityAuthoringSnapshot struct {
	Facility             *domain.Facility `json:"facility"`
	SaveCalls            int              `json:"saveCalls"`
	RepairWrites         int              `json:"repairWrites"`
	BindingCount         int              `json:"bindingCount"`
	BrokenReferenceCount int              `json:"brokenReferenceCount"`
	SessionRevision      uint64           `json:"sessionRevision"`
	PreviewCalls         int              `json:"previewCalls"`
	ResetWrites          int              `json:"resetWrites"`
	RecoveryWrites       int              `json:"recoveryWrites"`
	PublishedEvents      int              `json:"publishedEvents"`
}

type fixtureFacilityPreviewRequest struct {
	ExpectedFacilityRevision uint64                             `json:"expectedFacilityRevision"`
	TerminalID               string                             `json:"terminalId"`
	DeviceState              *domain.FacilityDeviceStatePreview `json:"deviceState,omitempty"`
	Condition                *domain.FacilityConditionPreview   `json:"condition,omitempty"`
}

type fixtureFacilityDeviceResetRequest struct {
	DeviceID                 string `json:"deviceId"`
	ExpectedFacilityRevision uint64 `json:"expectedFacilityRevision"`
	CorrelationID            string `json:"correlationId"`
}

type fixtureFacilityResetRequest struct {
	ExpectedFacilityRevision uint64 `json:"expectedFacilityRevision"`
	CorrelationID            string `json:"correlationId"`
}

type fixtureFacilityRecoveryRequest struct {
	ConditionID              string                              `json:"conditionId"`
	ExpectedFacilityRevision uint64                              `json:"expectedFacilityRevision"`
	CorrelationID            string                              `json:"correlationId"`
	Recovery                 *domain.DiagnosticRecoveryReference `json:"recovery"`
}

type fixtureFacilityInspectionResult struct {
	OK               bool                             `json:"ok"`
	Failure          domain.FacilityFailureCode       `json:"failure,omitempty"`
	Issues           []domain.FacilityIssue           `json:"issues,omitempty"`
	SessionRevision  uint64                           `json:"sessionRevision"`
	FacilityRevision uint64                           `json:"facilityRevision"`
	Report           *domain.FacilityDependencyReport `json:"report,omitempty"`
}

type fixtureFacilityAuthoringRequest struct {
	Session                  domain.Session `json:"session"`
	ExpectedSessionRevision  uint64         `json:"expectedSessionRevision"`
	ExpectedFacilityRevision uint64         `json:"expectedFacilityRevision"`
	CorrelationID            string         `json:"correlationId"`
}

type fixtureFacilityInspectionRequest struct {
	Target                   domain.FacilityEntityReference `json:"target"`
	ExpectedSessionRevision  uint64                         `json:"expectedSessionRevision"`
	ExpectedFacilityRevision uint64                         `json:"expectedFacilityRevision"`
}

func (state *fixtureFacilityAuthoringState) reset(scenario string) error {
	scenario = strings.TrimSpace(scenario)
	if scenario == "" {
		scenario = "authored"
	}
	if scenario != "empty" && scenario != "referenced-device" && scenario != "authored" && scenario != "operations" {
		return fmt.Errorf("unknown facility authoring scenario %q", scenario)
	}
	session := facilityAuthoringFixtureSession(scenario == "empty")
	if scenario == "operations" {
		for index := range session.Facility.Devices {
			device := &session.Facility.Devices[index]
			if device.ID == fixtureFacilityPowerID || device.ID == fixtureFacilityCoolingID {
				device.CurrentStateID = "online"
			}
		}
		privateRecovery := func() []domain.DiagnosticRecoveryReference {
			return []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}}
		}
		capability := func(value domain.FacilityCapability) []domain.DiagnosticEffect {
			return []domain.DiagnosticEffect{{CapabilityBlock: &domain.CapabilityBlockEffect{Capability: value}}}
		}
		session.Facility.Conditions = []domain.DiagnosticCondition{
			{
				ID: fixtureFacilityUnpoweredConditionID, Name: "Reactor controls unpowered",
				Category: domain.DiagnosticConditionCategoryUnpowered,
				Device:   &domain.DiagnosticDeviceScope{DeviceID: fixtureFacilityPowerID},
				Effects:  capability(domain.FacilityCapabilityExecuteCommand), Recovery: privateRecovery(),
			},
			{
				ID: fixtureFacilityCondition, Name: "Security authorization corrupted",
				Category:      domain.DiagnosticConditionCategoryAuthorizationCorrupted,
				Device:        &domain.DiagnosticDeviceScope{DeviceID: fixtureFacilityDoorID},
				InitialActive: false, CurrentActive: true,
				Effects: capability(domain.FacilityCapabilityViewEntry), Recovery: privateRecovery(),
			},
		}
	}
	if err := domain.ValidateSession(session); err != nil {
		return fmt.Errorf("invalid facility authoring fixture: %w", err)
	}
	state.mu.Lock()
	state.session = session
	state.sessionRevision = 1
	state.saveCalls = 0
	state.repairWrites = 0
	state.previewCalls = 0
	state.resetWrites = 0
	state.recoveryWrites = 0
	state.publishedEvents = 0
	state.nextFailure = ""
	state.mu.Unlock()
	return nil
}

func (state *fixtureFacilityAuthoringState) sessionSnapshot() (domain.Session, uint64) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return domain.CloneSession(state.session), state.sessionRevision
}

func (state *fixtureFacilityAuthoringState) inspect(
	request fixtureFacilityInspectionRequest,
) fixtureFacilityInspectionResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	facilityRevision := fixtureSessionFacilityRevision(state.session)
	result := fixtureFacilityInspectionResult{
		SessionRevision: state.sessionRevision, FacilityRevision: facilityRevision,
	}
	if request.ExpectedSessionRevision != state.sessionRevision || request.ExpectedFacilityRevision != facilityRevision {
		result.Failure = domain.FacilityFailureStaleRevision
		return result
	}
	report, issues := domain.BuildFacilityDependencyReport(state.session, request.Target)
	if len(issues) != 0 {
		result.Failure = issues[0].Code
		result.Issues = slices.Clone(issues)
		return result
	}
	result.OK = true
	result.Report = &report
	return result
}

func (state *fixtureFacilityAuthoringState) save(
	request fixtureFacilityAuthoringRequest,
) domain.FacilityOperationResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	previousFacilityRevision := fixtureSessionFacilityRevision(state.session)
	failure := func(code domain.FacilityFailureCode, issues []domain.FacilityIssue) domain.FacilityOperationResult {
		return domain.FacilityOperationResult{
			CorrelationID: request.CorrelationID, Failure: code, Issues: slices.Clone(issues),
			SessionRevision: state.sessionRevision, PreviousFacilityRevision: previousFacilityRevision,
			ResultingFacilityRevision: previousFacilityRevision,
		}
	}
	if request.ExpectedSessionRevision != state.sessionRevision ||
		request.ExpectedFacilityRevision != previousFacilityRevision {
		return failure(domain.FacilityFailureStaleRevision, nil)
	}
	if strings.TrimSpace(request.CorrelationID) == "" {
		return failure(domain.FacilityFailureInvalidConfiguration, nil)
	}

	candidate := domain.CloneSession(request.Session)
	protectFixtureFacilityCurrentValues(state.session.Facility, candidate.Facility)
	if candidate.Facility != nil {
		candidate.Facility.Revision = previousFacilityRevision
	}
	issues := domain.ValidateFacilityAuthoringCandidate(state.session, candidate)
	if len(issues) != 0 {
		return failure(domain.FacilityFailureInvalidConfiguration, issues)
	}
	if reflect.DeepEqual(state.session, candidate) {
		canonical := domain.CloneSession(state.session)
		return domain.FacilityOperationResult{
			OK: true, CorrelationID: request.CorrelationID, SessionRevision: state.sessionRevision,
			PreviousFacilityRevision: previousFacilityRevision, ResultingFacilityRevision: previousFacilityRevision,
			Session: &canonical,
		}
	}

	affectedDevices, affectedConditions := fixtureFacilityAffectedIDs(state.session.Facility, candidate.Facility)
	removedReferencedDevice := fixtureFacilityHasDevice(state.session.Facility, fixtureFacilityPowerID) &&
		!fixtureFacilityHasDevice(candidate.Facility, fixtureFacilityPowerID)
	if candidate.Facility != nil {
		candidate.Facility.Revision = previousFacilityRevision + 1
	}
	state.session = domain.CloneSession(candidate)
	state.sessionRevision++
	state.saveCalls++
	if removedReferencedDevice {
		state.repairWrites++
	}
	canonical := domain.CloneSession(state.session)
	return domain.FacilityOperationResult{
		OK: true, Changed: true, CorrelationID: request.CorrelationID,
		SessionRevision: state.sessionRevision, PreviousFacilityRevision: previousFacilityRevision,
		ResultingFacilityRevision: fixtureSessionFacilityRevision(state.session),
		AffectedDeviceIDs:         affectedDevices, AffectedConditionIDs: affectedConditions,
		Session: &canonical,
	}
}

func (state *fixtureFacilityAuthoringState) snapshot() fixtureFacilityAuthoringSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	broken := 0
	if err := domain.ValidateSession(state.session); err != nil {
		broken = 1
	}
	return fixtureFacilityAuthoringSnapshot{
		Facility: domain.CloneFacility(state.session.Facility), SaveCalls: state.saveCalls,
		RepairWrites: state.repairWrites, BindingCount: fixtureFacilityBindingCount(state.session),
		BrokenReferenceCount: broken, SessionRevision: state.sessionRevision,
		PreviewCalls: state.previewCalls, ResetWrites: state.resetWrites,
		RecoveryWrites: state.recoveryWrites, PublishedEvents: state.publishedEvents,
	}
}

func (state *fixtureFacilityAuthoringState) preview(
	request fixtureFacilityPreviewRequest,
) map[string]any {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.previewCalls++
	revision := fixtureSessionFacilityRevision(state.session)
	if request.ExpectedFacilityRevision != revision {
		return map[string]any{"ok": false, "failure": domain.FacilityFailureStaleRevision, "facilityRevision": revision}
	}
	terminal := fixtureTerminalByID(&state.session, request.TerminalID)
	if terminal == nil || (request.DeviceState == nil) == (request.Condition == nil) {
		return map[string]any{"ok": false, "failure": domain.FacilityFailureMissingReference, "facilityRevision": revision}
	}
	facility := domain.CloneFacility(state.session.Facility)
	if request.DeviceState != nil {
		device := fixtureFacilityDeviceByID(facility, request.DeviceState.DeviceID)
		if device == nil || !slices.ContainsFunc(device.States, func(value domain.FacilityDeviceState) bool {
			return value.ID == request.DeviceState.StateID
		}) {
			return map[string]any{"ok": false, "failure": domain.FacilityFailureMissingReference, "facilityRevision": revision}
		}
		device.CurrentStateID = request.DeviceState.StateID
	}
	if request.Condition != nil {
		condition := fixtureFacilityConditionByID(facility, request.Condition.ConditionID)
		if condition == nil {
			return map[string]any{"ok": false, "failure": domain.FacilityFailureMissingReference, "facilityRevision": revision}
		}
		condition.CurrentActive = request.Condition.Active
	}
	tree := domain.CloneContentNode(terminal.Root)
	fixtureProjectFacilityVariants(&tree, facility)
	projection := domain.PublicLiveState{
		TerminalID: terminal.ID, TerminalName: terminal.Name, Tree: tree,
		HackLevel: terminal.HackLevel, IntroText: terminal.IntroText,
	}
	return map[string]any{"ok": true, "facilityRevision": revision, "terminal": projection}
}

func (state *fixtureFacilityAuthoringState) nextOperationFailure(failure string) {
	state.mu.Lock()
	state.nextFailure = strings.TrimSpace(failure)
	state.mu.Unlock()
}

func (state *fixtureFacilityAuthoringState) operationFailure(
	correlationID string,
	expectedRevision uint64,
) *domain.FacilityOperationResult {
	current := fixtureSessionFacilityRevision(state.session)
	failure := state.nextFailure
	state.nextFailure = ""
	if expectedRevision != current {
		failure = "stale-revision"
	}
	if failure == "" {
		return nil
	}
	code := domain.FacilityFailurePersistenceFailed
	if failure == "stale-revision" {
		code = domain.FacilityFailureStaleRevision
	}
	return &domain.FacilityOperationResult{
		CorrelationID: correlationID, Failure: code, SessionRevision: state.sessionRevision,
		PreviousFacilityRevision: current, ResultingFacilityRevision: current,
	}
}

func (state *fixtureFacilityAuthoringState) resetDevice(
	request fixtureFacilityDeviceResetRequest,
) domain.FacilityOperationResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	if failed := state.operationFailure(request.CorrelationID, request.ExpectedFacilityRevision); failed != nil {
		return *failed
	}
	device := fixtureFacilityDeviceByID(state.session.Facility, request.DeviceID)
	if device == nil {
		return domain.FacilityOperationResult{CorrelationID: request.CorrelationID, Failure: domain.FacilityFailureMissingReference}
	}
	previous := state.session.Facility.Revision
	device.CurrentStateID = device.InitialStateID
	affectedConditions := make([]string, 0)
	for index := range state.session.Facility.Conditions {
		condition := &state.session.Facility.Conditions[index]
		if condition.Device == nil || condition.Device.DeviceID != request.DeviceID {
			continue
		}
		condition.CurrentActive = condition.InitialActive
		affectedConditions = append(affectedConditions, condition.ID)
	}
	state.session.Facility.Revision++
	state.sessionRevision++
	state.resetWrites++
	state.publishedEvents++
	canonical := domain.CloneSession(state.session)
	return domain.FacilityOperationResult{
		OK: true, Changed: true, CorrelationID: request.CorrelationID,
		SessionRevision: state.sessionRevision, PreviousFacilityRevision: previous,
		ResultingFacilityRevision: state.session.Facility.Revision,
		AffectedDeviceIDs:         []string{request.DeviceID}, AffectedConditionIDs: affectedConditions,
		Session: &canonical,
	}
}

func (state *fixtureFacilityAuthoringState) resetFacility(
	request fixtureFacilityResetRequest,
) domain.FacilityOperationResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	if failed := state.operationFailure(request.CorrelationID, request.ExpectedFacilityRevision); failed != nil {
		return *failed
	}
	previous := state.session.Facility.Revision
	deviceIDs := make([]string, 0, len(state.session.Facility.Devices))
	conditionIDs := make([]string, 0, len(state.session.Facility.Conditions))
	for index := range state.session.Facility.Devices {
		device := &state.session.Facility.Devices[index]
		device.CurrentStateID = device.InitialStateID
		deviceIDs = append(deviceIDs, device.ID)
	}
	for index := range state.session.Facility.Conditions {
		condition := &state.session.Facility.Conditions[index]
		condition.CurrentActive = condition.InitialActive
		conditionIDs = append(conditionIDs, condition.ID)
	}
	slices.Sort(deviceIDs)
	slices.Sort(conditionIDs)
	state.session.Facility.Revision++
	state.sessionRevision++
	state.resetWrites++
	state.publishedEvents++
	canonical := domain.CloneSession(state.session)
	return domain.FacilityOperationResult{
		OK: true, Changed: true, CorrelationID: request.CorrelationID,
		SessionRevision: state.sessionRevision, PreviousFacilityRevision: previous,
		ResultingFacilityRevision: state.session.Facility.Revision,
		AffectedDeviceIDs:         deviceIDs, AffectedConditionIDs: conditionIDs, Session: &canonical,
	}
}

func (state *fixtureFacilityAuthoringState) recover(
	request fixtureFacilityRecoveryRequest,
) domain.FacilityOperationResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	if failed := state.operationFailure(request.CorrelationID, request.ExpectedFacilityRevision); failed != nil {
		return *failed
	}
	condition := fixtureFacilityConditionByID(state.session.Facility, request.ConditionID)
	if condition == nil || request.Recovery == nil || request.Recovery.PrivateOverseerAction == nil ||
		!*request.Recovery.PrivateOverseerAction {
		return domain.FacilityOperationResult{CorrelationID: request.CorrelationID, Failure: domain.FacilityFailureMissingReference}
	}
	previous := state.session.Facility.Revision
	condition.CurrentActive = false
	state.session.Facility.Revision++
	state.sessionRevision++
	state.recoveryWrites++
	state.publishedEvents++
	canonical := domain.CloneSession(state.session)
	return domain.FacilityOperationResult{
		OK: true, Changed: true, CorrelationID: request.CorrelationID,
		SessionRevision: state.sessionRevision, PreviousFacilityRevision: previous,
		ResultingFacilityRevision: state.session.Facility.Revision,
		AffectedConditionIDs:      []string{request.ConditionID}, Session: &canonical,
	}
}

func fixtureFacilityConditionByID(facility *domain.Facility, conditionID string) *domain.DiagnosticCondition {
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

func fixtureProjectFacilityVariants(node *domain.ContentNode, facility *domain.Facility) {
	states := make(map[string]string, len(facility.Devices))
	for _, device := range facility.Devices {
		states[device.ID] = device.CurrentStateID
	}
	for _, variant := range node.FacilityNameVariants {
		if states[variant.When.DeviceID] == variant.When.StateID {
			node.Name = variant.Text
		}
	}
	for index := range node.Blocks {
		block := &node.Blocks[index]
		for _, variant := range block.FacilityTextVariants {
			if states[variant.When.DeviceID] == variant.When.StateID {
				block.InitialText = variant.Text
			}
		}
	}
	for index := range node.Children {
		fixtureProjectFacilityVariants(&node.Children[index], facility)
	}
}

func fixtureSessionFacilityRevision(session domain.Session) uint64 {
	if session.Facility == nil {
		return 0
	}
	return session.Facility.Revision
}

func (state *fixtureFacilityDiagnosticState) reset(scenario string) error {
	session, activeConditionID, err := facilityDiagnosticSession(strings.TrimSpace(scenario))
	if err != nil {
		return err
	}
	if err := domain.ValidateSession(session); err != nil {
		return fmt.Errorf("invalid facility diagnostic fixture: %w", err)
	}
	state.mu.Lock()
	state.active = true
	state.scenario = strings.TrimSpace(scenario)
	state.session = domain.CloneSession(session)
	state.audit = fixtureFacilityDiagnosticAudit{}
	state.mu.Unlock()
	if activeConditionID == "" {
		return errors.New("facility diagnostic fixture has no active condition")
	}
	return nil
}

func (state *fixtureFacilityDiagnosticState) deactivate() {
	state.mu.Lock()
	state.active = false
	state.mu.Unlock()
}

func (state *fixtureFacilityDiagnosticState) sessionSnapshot() (domain.Session, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.active {
		return domain.Session{}, false
	}
	return domain.CloneSession(state.session), true
}

func (state *fixtureFacilityDiagnosticState) projectionFacility() *domain.Facility {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.active {
		return nil
	}
	return domain.CloneFacility(state.session.Facility)
}

func (state *fixtureFacilityDiagnosticState) target() (domain.TerminalTarget, bool) {
	session, ok := state.sessionSnapshot()
	if !ok {
		return domain.TerminalTarget{}, false
	}
	terminal := fixtureTerminalByID(&session, fixtureFacilityDiagnosticTerminalID)
	if terminal == nil {
		return domain.TerminalTarget{}, false
	}
	return fixtureTarget(*terminal), true
}

func (state *fixtureFacilityDiagnosticState) replay() {
	state.mu.Lock()
	state.audit.ProjectionReplays++
	state.mu.Unlock()
}

func (state *fixtureFacilityDiagnosticState) recover(conditionID string, private bool) (domain.FacilityOperationResult, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.active || state.session.Facility == nil {
		return domain.FacilityOperationResult{}, errors.New("facility diagnostic fixture is inactive")
	}
	condition := fixtureDiagnosticConditionByID(state.session.Facility, conditionID)
	if condition == nil || !condition.CurrentActive {
		return domain.FacilityOperationResult{}, errors.New("diagnostic condition is not active")
	}
	previous := state.session.Facility.Revision
	condition.CurrentActive = false
	state.session.Facility.Revision++
	state.audit.DurableWrites++
	if private {
		state.audit.PrivateRecoveries++
	} else {
		state.audit.ApprovedRecoveries++
	}
	canonical := domain.CloneSession(state.session)
	return domain.FacilityOperationResult{
		OK: true, Changed: true, PreviousFacilityRevision: previous,
		ResultingFacilityRevision: state.session.Facility.Revision,
		AffectedConditionIDs:      []string{conditionID}, Session: &canonical,
	}, nil
}

func (state *fixtureFacilityDiagnosticState) snapshot() fixtureFacilityDiagnosticSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	conditionStates := make(map[string]bool)
	activeCondition := map[string]string{}
	blocked := make([]domain.FacilityCapability, 0)
	presentationEffects := make([]domain.TerminalPresentationEffect, 0)
	if state.session.Facility != nil {
		for _, condition := range state.session.Facility.Conditions {
			conditionStates[condition.ID] = condition.CurrentActive
			if !condition.CurrentActive {
				continue
			}
			activeCondition["id"] = condition.ID
			activeCondition["category"] = string(condition.Category)
			if condition.CustomCategory != "" {
				activeCondition["customCategory"] = condition.CustomCategory
			}
			for _, effect := range condition.Effects {
				if effect.CapabilityBlock != nil {
					blocked = append(blocked, effect.CapabilityBlock.Capability)
				}
				if effect.DisplayInstability != nil {
					presentationEffects = append(presentationEffects, domain.TerminalPresentationEffectDisplayUnstable)
				}
			}
		}
	}
	return fixtureFacilityDiagnosticSnapshot{
		ActiveCondition: activeCondition,
		Facility: fixtureFacilitySnapshot{
			Revision:        fixtureSessionFacilityRevision(state.session),
			ConditionStates: conditionStates,
		},
		BlockedCapabilities: blocked, PresentationEffects: presentationEffects,
		AuthoredRecordDigest:  fixtureDiagnosticDigest(fixtureDiagnosticAuthoredRecord()),
		AuthoredContentDigest: fixtureDiagnosticDigest(fixtureDiagnosticAuthoredContent()),
		Audit:                 state.audit,
	}
}

func fixtureDiagnosticConditionByID(facility *domain.Facility, conditionID string) *domain.DiagnosticCondition {
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

func fixtureDiagnosticDigest(value string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
}

func fixtureDiagnosticAuthoredRecord() string {
	return strings.Repeat("RECORD 04-B // SECTOR 7 CORRIDOR PRESSURE NOMINAL\n", 96) + "END OF AUTHORED RECORD"
}

func fixtureDiagnosticDamagedRecord() string {
	return strings.Repeat("R_C_RD 04-B // S_CT_R 7 C_RR_PT_D ??__??\n", 96) + "_ND _F D_M_G_D R_C_RD"
}

func fixtureDiagnosticAuthoredContent() string {
	return "STABLE REFERENCE\nAFFECTED ENTRY\nAFFECTED COMMAND\nREMOTE TERMINAL\nDAMAGED RECORD"
}

func facilityDiagnosticSession(scenario string) (domain.Session, string, error) {
	conditionID := map[string]string{
		"offline":                   fixtureFacilityOfflineConditionID,
		"unpowered":                 fixtureFacilityUnpoweredConditionID,
		"transition-recovery":       fixtureFacilityUnpoweredConditionID,
		"network-isolated":          fixtureFacilityNetworkConditionID,
		"program-recovery":          fixtureFacilityNetworkConditionID,
		"storage-damaged":           fixtureFacilityStorageConditionID,
		"storage-damaged-multipage": fixtureFacilityStorageConditionID,
		"authorization-corrupted":   fixtureFacilityCondition,
		"private-recovery-escape":   fixtureFacilityCondition,
		"display-unstable":          fixtureFacilityDisplayConditionID,
		"custom":                    fixtureFacilityCustomConditionID,
	}[scenario]
	if conditionID == "" {
		return domain.Session{}, "", fmt.Errorf("unknown facility diagnostic scenario %q", scenario)
	}

	privateRecovery := true
	terminalScope := func() *domain.DiagnosticTerminalScope {
		return &domain.DiagnosticTerminalScope{TerminalID: fixtureFacilityDiagnosticTerminalID}
	}
	privateReference := func() domain.DiagnosticRecoveryReference {
		return domain.DiagnosticRecoveryReference{PrivateOverseerAction: &privateRecovery}
	}
	capability := func(value domain.FacilityCapability) domain.DiagnosticEffect {
		return domain.DiagnosticEffect{CapabilityBlock: &domain.CapabilityBlockEffect{Capability: value}}
	}
	condition := func(id, name string, category domain.DiagnosticConditionCategory, effects ...domain.DiagnosticEffect) domain.DiagnosticCondition {
		active := id == conditionID
		return domain.DiagnosticCondition{
			ID: id, Name: name, Category: category, Terminal: terminalScope(),
			InitialActive: active, CurrentActive: active, Effects: effects,
			Recovery: []domain.DiagnosticRecoveryReference{privateReference()},
		}
	}

	conditions := []domain.DiagnosticCondition{
		condition(fixtureFacilityOfflineConditionID, "Security terminal offline", domain.DiagnosticConditionCategoryOffline,
			capability(domain.FacilityCapabilityViewEntry)),
		condition(fixtureFacilityUnpoweredConditionID, "Reactor controls unpowered", domain.DiagnosticConditionCategoryUnpowered,
			capability(domain.FacilityCapabilityExecuteCommand)),
		condition(fixtureFacilityNetworkConditionID, "Operations network isolated", domain.DiagnosticConditionCategoryNetworkIsolated,
			capability(domain.FacilityCapabilityTerminalTransition),
			domain.DiagnosticEffect{DiagnosticPath: &domain.DiagnosticPathEffect{
				TerminalID: fixtureFacilityDiagnosticTerminalID, NodeID: "diagnostic-isolation",
			}}),
		condition(fixtureFacilityStorageConditionID, "Archive storage damaged", domain.DiagnosticConditionCategoryStorageDamaged,
			domain.DiagnosticEffect{RecordSubstitution: &domain.RecordSubstitutionEffect{
				TerminalID: fixtureFacilityDiagnosticTerminalID, BlockID: "block-damaged-record",
				ReplacementText: fixtureDiagnosticDamagedRecord(),
			}}),
		condition(fixtureFacilityCondition, "Security authorization corrupted", domain.DiagnosticConditionCategoryAuthorizationCorrupted,
			capability(domain.FacilityCapabilityExecuteCommand)),
		condition(fixtureFacilityDisplayConditionID, "Reactor display unstable", domain.DiagnosticConditionCategoryDisplayUnstable,
			domain.DiagnosticEffect{DisplayInstability: &domain.DisplayInstabilityEffect{}}),
		condition(fixtureFacilityCustomConditionID, "Cooling loop contamination", domain.DiagnosticConditionCategoryCustom,
			capability(domain.FacilityCapabilityHack)),
	}
	conditions[len(conditions)-1].CustomCategory = "coolant-contamination"

	state := func(id, name string) domain.FacilityDeviceState {
		return domain.FacilityDeviceState{ID: id, Name: name}
	}
	devices := []domain.FacilityDevice{
		{
			ID: fixtureFacilityPowerID, Name: "Primary power grid", Kind: domain.FacilityDeviceKindPowerGrid,
			InitialStateID: "offline", CurrentStateID: "offline",
			States: []domain.FacilityDeviceState{state("offline", "Offline"), state("online", "Online")},
			Transitions: []domain.FacilityDeviceTransition{{
				ID: "restore", Name: "Restore primary power", SourceStateID: "offline", DestinationStateID: "online",
				ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: fixtureFacilityUnpoweredConditionID, Active: false}},
				Recovery:         true,
			}},
		},
		{
			ID: fixtureFacilityNetworkID, Name: "Operations network", Kind: domain.FacilityDeviceKindNetworkSegment,
			InitialStateID: "isolated", CurrentStateID: "isolated",
			States: []domain.FacilityDeviceState{state("isolated", "Isolated"), state("connected", "Connected")},
			Transitions: []domain.FacilityDeviceTransition{{
				ID: "reconnect", Name: "Reconnect operations network", SourceStateID: "isolated", DestinationStateID: "connected",
				ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: fixtureFacilityNetworkConditionID, Active: false}},
				Recovery:         true,
			}},
		},
	}
	conditions[1].Recovery = append(conditions[1].Recovery, domain.DiagnosticRecoveryReference{
		Transition: &domain.FacilityTransitionRequest{DeviceID: fixtureFacilityPowerID, TransitionID: "restore"},
	})
	programID := fixtureFacilityNetworkRecoveryProgramID
	conditions[2].Recovery = append(conditions[2].Recovery, domain.DiagnosticRecoveryReference{RecoveryProgramID: &programID})

	falseVisibility := &domain.FacilityStateEquality{DeviceID: fixtureFacilityPowerID, StateID: "online"}
	entry := func(id, name, description string) domain.ContentNode {
		return domain.ContentNode{ID: id, Type: domain.NodeEntry, Name: name, Description: description}
	}
	stableReference := entry("stable-reference", "STABLE REFERENCE", "AUTHORED CONTENT REMAINS INTACT")
	accessEntry := func(id, name string) domain.ContentNode {
		return entry(id, name, "Ошибка доступа")
	}
	diagnosticPath := domain.ContentNode{
		ID: "diagnostic-isolation", Type: domain.NodeEntry, Name: "ISOLATION DIAGNOSTICS",
		Description: "NETWORK SEGMENT ISOLATED // BACK PATH AVAILABLE", VisibleWhen: falseVisibility,
	}
	damagedRecord := domain.ContentNode{
		ID: "damaged-record", Type: domain.NodeEntry, Name: "DAMAGED RECORD",
		Blocks: []domain.EntryContentBlock{{ID: "block-damaged-record", InitialText: fixtureDiagnosticAuthoredRecord()}},
	}
	restorePower := domain.ContentNode{
		ID: "restore-primary-power", Type: domain.NodeCommand, Name: "RESTORE PRIMARY POWER", Text: "PRIMARY POWER RESTORED",
	}
	recoverNetwork := domain.ContentNode{
		ID: "run-network-recovery", Type: domain.NodeCommand, Name: "RUN NETWORK RECOVERY HOLOTAPE", Text: "NETWORK RECOVERY COMPLETE",
	}
	children := []domain.ContentNode{stableReference, diagnosticPath, damagedRecord}
	switch scenario {
	case "offline":
		children = append(children, accessEntry("affected-entry", "AFFECTED ENTRY"))
	case "unpowered":
		children = append(children, accessEntry("affected-command", "AFFECTED COMMAND"))
	case "transition-recovery":
		children = append(children, restorePower)
	case "network-isolated":
		children = append(children, accessEntry("remote-terminal", "REMOTE TERMINAL"))
	case "program-recovery":
		children = append(children, recoverNetwork)
	case "authorization-corrupted":
		children = append(children, accessEntry("security-override", "SECURITY OVERRIDE"))
	case "private-recovery-escape":
		children = append(children, accessEntry("affected-command", "AFFECTED COMMAND"))
	case "custom":
		children = append(children, accessEntry("affected-hack", "AFFECTED HACK"))
	}
	session := domain.Session{
		Version: 1, Name: "Facility diagnostic fixture",
		Terminals: []domain.Terminal{
			{
				ID: fixtureFacilityDiagnosticTerminalID, Name: "Facility diagnostics", IntroText: "FACILITY DIAGNOSTIC NETWORK",
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: children},
			},
			{
				ID: fixtureFacilityDiagnosticRemoteID, Name: "Remote diagnostics", IntroText: "REMOTE DIAGNOSTICS",
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}},
			},
		},
		Facility: &domain.Facility{
			Revision: 7, Devices: devices, Conditions: conditions,
			RecoveryPrograms: []domain.RecoveryProgram{{
				ID: fixtureFacilityNetworkRecoveryProgramID, Name: "VAULT-TEC NETWORK RECOVERY",
				Transitions: []domain.FacilityTransitionRequest{{DeviceID: fixtureFacilityNetworkID, TransitionID: "reconnect"}},
			}},
		},
	}
	return session, conditionID, nil
}

func (state *fixtureFacilityPlayerState) reset(scenario string) error {
	scenario = strings.TrimSpace(scenario)
	if scenario == "" {
		scenario = "ready"
	}
	switch scenario {
	case "ready", "stale-revision", "conflict", "persistence-failure", "concurrent-resolution", "shared-projection":
	default:
		return fmt.Errorf("unknown facility player-state scenario %q", scenario)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	state.scenario = scenario
	state.projectionSession = nil
	state.resetNavigationFor = ""
	state.revision = 0
	state.deviceStates = map[string]string{
		fixtureFacilityDoorID:  "locked",
		fixtureFacilityAlarmID: "armed",
	}
	state.lastRequestID = ""
	state.lastResult = nil
	state.resolutionAttempts = 0
	state.durableWrites = 0
	state.successfulActions = 0
	state.duplicateResults = 0
	if scenario == "shared-projection" {
		session := facilityProjectionSession()
		state.projectionSession = &session
	}
	return nil
}

func (state *fixtureFacilityPlayerState) session() (domain.Session, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.projectionSession == nil {
		return domain.Session{}, false
	}
	return domain.CloneSession(*state.projectionSession), true
}

func (state *fixtureFacilityPlayerState) target(terminalID string) (domain.TerminalTarget, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.projectionSession == nil {
		return domain.TerminalTarget{}, false
	}
	terminal := fixtureTerminalByID(state.projectionSession, terminalID)
	if terminal == nil {
		return domain.TerminalTarget{}, false
	}
	return fixtureTarget(*terminal), true
}

func (state *fixtureFacilityPlayerState) projectionFacility() *domain.Facility {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.projectionSession == nil {
		return nil
	}
	return domain.CloneFacility(state.projectionSession.Facility)
}

func (state *fixtureFacilityPlayerState) resetNavigation(terminalID string) {
	state.mu.Lock()
	state.resetNavigationFor = terminalID
	state.mu.Unlock()
}

func (state *fixtureFacilityPlayerState) takeNavigationReset(terminalID string) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.resetNavigationFor != terminalID {
		return false
	}
	state.resetNavigationFor = ""
	return true
}

func (state *fixtureFacilityPlayerState) applyProjectionTransition() error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.projectionSession == nil || state.projectionSession.Facility == nil {
		return errors.New("shared facility projection is not active")
	}
	facility := state.projectionSession.Facility
	door := fixtureFacilityDeviceByID(facility, fixtureFacilityDoorID)
	alarm := fixtureFacilityDeviceByID(facility, fixtureFacilityAlarmID)
	if door == nil || alarm == nil {
		return errors.New("shared facility projection is incomplete")
	}
	if door.CurrentStateID == "open" && alarm.CurrentStateID == "silent" {
		return nil
	}
	if door.CurrentStateID != "locked" || alarm.CurrentStateID != "armed" {
		return errors.New("shared facility projection has an invalid transition source")
	}
	door.CurrentStateID = "open"
	alarm.CurrentStateID = "silent"
	security := fixtureTerminalByID(state.projectionSession, fixtureFacilityTerminalID)
	if security == nil {
		return errors.New("shared facility security terminal is missing")
	}
	security.CommandStates = map[string]domain.CommandExecutionState{
		fixtureFacilityCommandID: {
			CompletedName: "SECURITY DOOR OPEN // LEGACY SNAPSHOT", ResultText: "Previously completed command result.",
			EntryContentChange: &domain.EntryContentChange{
				BlockID: "block-security-door", CompletedText: "SECURITY DOOR: LEGACY COMMAND COMPLETE",
			},
		},
	}
	for index := range facility.Conditions {
		if facility.Conditions[index].ID == fixtureFacilityCondition {
			facility.Conditions[index].CurrentActive = false
		}
	}
	facility.Revision++
	return nil
}

func (state *fixtureFacilityPlayerState) moveTerminal(terminalID, groupID string) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.projectionSession == nil {
		return errors.New("shared facility projection is not active")
	}
	if fixtureTerminalByID(state.projectionSession, terminalID) == nil {
		return fmt.Errorf("unknown shared facility terminal %q", terminalID)
	}
	targetGroup := -1
	for index := range state.projectionSession.TerminalGroups {
		group := &state.projectionSession.TerminalGroups[index]
		if group.ID == groupID {
			targetGroup = index
		}
		group.TerminalIDs = removeFixtureString(group.TerminalIDs, terminalID)
	}
	if targetGroup < 0 {
		return fmt.Errorf("unknown shared facility group %q", groupID)
	}
	state.projectionSession.TerminalGroups[targetGroup].TerminalIDs = append(
		state.projectionSession.TerminalGroups[targetGroup].TerminalIDs, terminalID,
	)
	return nil
}

func (state *fixtureFacilityPlayerState) scenarioForAttempt(requestID string) string {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.resolutionAttempts++
	if state.lastRequestID == "" {
		state.lastRequestID = requestID
	}
	return state.scenario
}

func (state *fixtureFacilityPlayerState) ApplyWorldAction(
	_ context.Context,
	request control.FacilityMutationRequest,
) domain.FacilityOperationResult {
	state.mu.Lock()
	scenario := state.scenario
	state.mu.Unlock()

	switch scenario {
	case "stale-revision":
		failure := state.recordFailure(request.CorrelationID, domain.FacilityFailureStaleRevision)
		return fixtureFacilityOperationResult(failure, 0, nil)
	case "persistence-failure":
		failure := state.recordFailure(request.CorrelationID, domain.FacilityFailurePersistenceFailed)
		return fixtureFacilityOperationResult(failure, 0, nil)
	}

	success := state.recordSuccess(request.CorrelationID)
	commandStates := map[string]domain.CommandExecutionState{
		fixtureFacilityCommandID: {
			CompletedName: "SECURITY DOOR OPEN",
			ResultText:    "Security door and alarm updated.",
		},
	}
	session := facilityPlayerSession(commandStates)
	session.Facility.Revision = success.ResultingFacilityRevision
	for index := range session.Facility.Devices {
		switch session.Facility.Devices[index].ID {
		case fixtureFacilityDoorID:
			session.Facility.Devices[index].CurrentStateID = "open"
		case fixtureFacilityAlarmID:
			session.Facility.Devices[index].CurrentStateID = "silent"
		}
	}
	for index := range session.Facility.Conditions {
		if session.Facility.Conditions[index].ID == fixtureFacilityCondition {
			session.Facility.Conditions[index].CurrentActive = false
		}
	}
	return fixtureFacilityOperationResult(success, 1, &session)
}

func fixtureFacilityOperationResult(
	result fixtureFacilityResult,
	sessionRevision uint64,
	session *domain.Session,
) domain.FacilityOperationResult {
	return domain.FacilityOperationResult{
		OK: result.OK, Changed: result.Changed, CorrelationID: result.CorrelationID,
		Failure: result.Failure, SessionRevision: sessionRevision,
		PreviousFacilityRevision: result.PreviousFacilityRevision, ResultingFacilityRevision: result.ResultingFacilityRevision,
		AffectedDeviceIDs: slices.Clone(result.AffectedDeviceIDs), AffectedConditionIDs: slices.Clone(result.AffectedConditionIDs),
		Session: session,
	}
}

func (state *fixtureFacilityPlayerState) currentRequestID() string {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.lastRequestID
}

func (state *fixtureFacilityPlayerState) recordRejected(requestID string) fixtureFacilityResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	result := state.resultLocked(requestID, domain.FacilityFailureRejected)
	state.lastResult = new(result)
	return result
}

func (state *fixtureFacilityPlayerState) recordFailure(
	requestID string,
	failure domain.FacilityFailureCode,
) fixtureFacilityResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	result := state.resultLocked(requestID, failure)
	state.lastResult = new(result)
	return result
}

func (state *fixtureFacilityPlayerState) recordSuccess(requestID string) fixtureFacilityResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.successfulActions != 0 {
		return state.recordDuplicateLocked(requestID)
	}
	previousRevision := state.revision
	state.revision++
	state.deviceStates[fixtureFacilityDoorID] = "open"
	state.deviceStates[fixtureFacilityAlarmID] = "silent"
	state.durableWrites++
	state.successfulActions++
	result := fixtureFacilityResult{
		OK: true, Changed: true, CorrelationID: requestID,
		PreviousFacilityRevision:  previousRevision,
		ResultingFacilityRevision: state.revision,
		AffectedDeviceIDs:         []string{fixtureFacilityDoorID, fixtureFacilityAlarmID},
		AffectedConditionIDs:      []string{fixtureFacilityCondition},
	}
	state.lastResult = new(result)
	return result
}

func (state *fixtureFacilityPlayerState) recordDuplicate(requestID string) fixtureFacilityResult {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.recordDuplicateLocked(requestID)
}

func (state *fixtureFacilityPlayerState) recordDuplicateLocked(requestID string) fixtureFacilityResult {
	state.duplicateResults++
	result := state.resultLocked(requestID, domain.FacilityFailureDuplicate)
	state.lastResult = new(result)
	return result
}

func (state *fixtureFacilityPlayerState) resultLocked(
	requestID string,
	failure domain.FacilityFailureCode,
) fixtureFacilityResult {
	return fixtureFacilityResult{
		CorrelationID:             requestID,
		Failure:                   failure,
		PreviousFacilityRevision:  state.revision,
		ResultingFacilityRevision: state.revision,
		AffectedDeviceIDs:         []string{fixtureFacilityDoorID, fixtureFacilityAlarmID},
		AffectedConditionIDs:      []string{fixtureFacilityCondition},
	}
}

func (state *fixtureFacilityPlayerState) snapshot() fixtureFacilityStateResponse {
	state.mu.Lock()
	defer state.mu.Unlock()
	deviceStates := maps.Clone(state.deviceStates)
	var lastResult *fixtureFacilityResult
	if state.lastResult != nil {
		result := *state.lastResult
		result.AffectedDeviceIDs = append([]string(nil), state.lastResult.AffectedDeviceIDs...)
		result.AffectedConditionIDs = append([]string(nil), state.lastResult.AffectedConditionIDs...)
		lastResult = &result
	}
	facilitySnapshot := fixtureFacilitySnapshot{Revision: state.revision, DeviceStates: deviceStates}
	if state.projectionSession != nil && state.projectionSession.Facility != nil {
		facilitySnapshot = projectionFacilitySnapshot(*state.projectionSession)
	}
	return fixtureFacilityStateResponse{
		Facility:           facilitySnapshot,
		LastFacilityResult: lastResult,
		Audit: fixtureFacilityAudit{
			ResolutionAttempts:     state.resolutionAttempts,
			DurableWrites:          state.durableWrites,
			SuccessfulWorldActions: state.successfulActions,
			DuplicateResults:       state.duplicateResults,
		},
	}
}

// fixtureFacilityLifecycle keeps the browser fixture's canonical session as
// the only facility authority while exercising the production projector.
type fixtureFacilityLifecycle struct {
	base        *live.Service
	state       *fixtureFacilityPlayerState
	diagnostics *fixtureFacilityDiagnosticState
}

type fixtureFacilityLifecycleState struct {
	mu sync.Mutex

	session                   domain.Session
	hydratedBeforePublication bool
	staleResolutions          int
	restoreSequence           []fixtureFacilityRestore
}

type fixtureFacilityRestore struct {
	Action               string `json:"action"`
	FacilityBeforePublic bool   `json:"facilityBeforePublic"`
}

type fixtureFacilityLifecycleSnapshot struct {
	Facility                  *fixtureFacilitySnapshot `json:"facility"`
	PersistedFacility         *fixtureFacilitySnapshot `json:"persistedFacility"`
	HydratedBeforePublication bool                     `json:"hydratedBeforePublication"`
	PendingRequests           int                      `json:"pendingRequests"`
	StaleResolutions          int                      `json:"staleResolutions"`
	RestoreSequence           []fixtureFacilityRestore `json:"restoreSequence"`
}

func (state *fixtureFacilityLifecycleState) reset(scenario string) error {
	scenario = strings.TrimSpace(scenario)
	if scenario == "" {
		scenario = "persisted"
	}
	var session domain.Session
	switch scenario {
	case "persisted", "pending":
		session = facilityLifecycleSession()
	case "legacy-v1":
		session = facilityLifecycleLegacySession()
	default:
		return fmt.Errorf("unknown facility lifecycle scenario %q", scenario)
	}

	state.mu.Lock()
	state.session = domain.CloneSession(session)
	state.hydratedBeforePublication = false
	state.staleResolutions = 0
	state.restoreSequence = nil
	state.mu.Unlock()
	return nil
}

func (state *fixtureFacilityLifecycleState) loadedSession() domain.Session {
	state.mu.Lock()
	defer state.mu.Unlock()
	return domain.CloneSession(state.session)
}

func (state *fixtureFacilityLifecycleState) recordHydration(action string, pendingInvalidated bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.hydratedBeforePublication = true
	if pendingInvalidated {
		state.staleResolutions++
	}
	if action != "" {
		state.restoreSequence = append(state.restoreSequence, fixtureFacilityRestore{
			Action: action, FacilityBeforePublic: true,
		})
	}
}

func (state *fixtureFacilityLifecycleState) snapshot(service *control.Service) fixtureFacilityLifecycleSnapshot {
	state.mu.Lock()
	session := domain.CloneSession(state.session)
	hydrated := state.hydratedBeforePublication
	staleResolutions := state.staleResolutions
	restoreSequence := slices.Clone(state.restoreSequence)
	state.mu.Unlock()

	var facility *fixtureFacilitySnapshot
	if session.Facility != nil {
		snapshot := projectionFacilitySnapshot(session)
		facility = &snapshot
	}
	persisted := facility
	if facility != nil {
		clone := *facility
		clone.DeviceStates = maps.Clone(facility.DeviceStates)
		clone.ConditionStates = maps.Clone(facility.ConditionStates)
		clone.DeviceIDs = slices.Clone(facility.DeviceIDs)
		persisted = &clone
	}
	pendingRequests := 0
	if service != nil && service.Snapshot().PendingCommandExecution != nil {
		pendingRequests = 1
	}
	return fixtureFacilityLifecycleSnapshot{
		Facility:                  facility,
		PersistedFacility:         persisted,
		HydratedBeforePublication: hydrated,
		PendingRequests:           pendingRequests,
		StaleResolutions:          staleResolutions,
		RestoreSequence:           restoreSequence,
	}
}

func (lifecycle *fixtureFacilityLifecycle) CreateRuntime(target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	lifecycle.state.takeNavigationReset(target.TerminalID)
	runtime, projection := lifecycle.base.CreateRuntime(target)
	applyFixturePresentationEffects(runtime, projection, target.Effects)
	return runtime, projection
}

func (lifecycle *fixtureFacilityLifecycle) UpdateRuntime(runtime *domain.TerminalRuntime, target domain.TerminalTarget) *domain.PublicLiveState {
	if lifecycle.state.takeNavigationReset(target.TerminalID) {
		runtime.Nav = nav.Default()
	}
	projection := lifecycle.base.UpdateRuntime(runtime, target)
	applyFixturePresentationEffects(runtime, projection, target.Effects)
	return projection
}

func applyFixturePresentationEffects(
	runtime *domain.TerminalRuntime,
	projection *domain.PublicLiveState,
	effects []domain.TerminalPresentationEffect,
) {
	if runtime != nil {
		runtime.Effects = slices.Clone(effects)
	}
	if projection != nil {
		projection.Effects = slices.Clone(effects)
	}
}

func (lifecycle *fixtureFacilityLifecycle) ProjectFacility(runtime *domain.TerminalRuntime, facility *domain.Facility) *domain.PublicLiveState {
	if projected := lifecycle.diagnostics.projectionFacility(); projected != nil {
		facility = projected
	} else if projected := lifecycle.state.projectionFacility(); projected != nil {
		facility = projected
	}
	fixtureEffects := slices.Clone(runtime.Effects)
	projection := lifecycle.base.ProjectFacility(runtime, facility)
	if facility == nil || len(fixtureEffects) != 0 {
		applyFixturePresentationEffects(runtime, projection, fixtureEffects)
	}
	return projection
}

func (lifecycle *fixtureFacilityLifecycle) ProjectRuntime(runtime *domain.TerminalRuntime) *domain.PublicLiveState {
	return lifecycle.base.ProjectRuntime(runtime)
}

func (lifecycle *fixtureFacilityLifecycle) SuspendRuntime(runtime *domain.TerminalRuntime) {
	lifecycle.base.SuspendRuntime(runtime)
}

func (lifecycle *fixtureFacilityLifecycle) ReactivateRuntime(
	runtime *domain.TerminalRuntime,
	target domain.TerminalTarget,
) *domain.PublicLiveState {
	if lifecycle.state.takeNavigationReset(target.TerminalID) {
		runtime.Nav = nav.Default()
	}
	projection := lifecycle.base.ReactivateRuntime(runtime, target)
	applyFixturePresentationEffects(runtime, projection, target.Effects)
	return projection
}

func (lifecycle *fixtureFacilityLifecycle) DiscardRuntime(
	target domain.TerminalTarget,
) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	lifecycle.state.takeNavigationReset(target.TerminalID)
	runtime, projection := lifecycle.base.DiscardRuntime(target)
	applyFixturePresentationEffects(runtime, projection, target.Effects)
	return runtime, projection
}

func (lifecycle *fixtureFacilityLifecycle) ResetFailedHack(
	runtime *domain.TerminalRuntime,
	target domain.TerminalTarget,
) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	replacement, projection := lifecycle.base.ResetFailedHack(runtime, target)
	applyFixturePresentationEffects(replacement, projection, target.Effects)
	return replacement, projection
}

func fixtureFacilityDeviceByID(facility *domain.Facility, deviceID string) *domain.FacilityDevice {
	if facility == nil {
		return nil
	}
	for index := range facility.Devices {
		if facility.Devices[index].ID == deviceID {
			return &facility.Devices[index]
		}
	}
	return nil
}

func removeFixtureString(values []string, target string) []string {
	result := values[:0]
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func projectionFacilitySnapshot(session domain.Session) fixtureFacilitySnapshot {
	facility := session.Facility
	if facility == nil {
		return fixtureFacilitySnapshot{}
	}
	deviceStates := make(map[string]string, len(facility.Devices))
	deviceIDs := make([]string, 0, len(facility.Devices))
	for _, device := range facility.Devices {
		deviceIDs = append(deviceIDs, device.ID)
		deviceStates[device.ID] = device.CurrentStateID
	}
	conditionStates := make(map[string]bool, len(facility.Conditions))
	for _, condition := range facility.Conditions {
		conditionStates[condition.ID] = condition.CurrentActive
	}
	return fixtureFacilitySnapshot{
		Revision:        facility.Revision,
		DeviceStates:    deviceStates,
		ConditionStates: conditionStates,
		DeviceIDs:       deviceIDs,
		TerminalCount:   len(session.Terminals),
		GroupCount:      len(session.TerminalGroups),
	}
}

var fixtureCommandResult = "Доступ в сектор разрешён.\n" +
	"ПРОТОКОЛ ДОСТУПА:\n" +
	strings.Repeat("Состояние гермозатвора подтверждено. Контрольные цепи переведены в штатный режим. "+
		"Маршрут эвакуации свободен. Аварийные блокировки сняты. Диагностический журнал сохранён.\n", 80) +
	"Конец отчёта."

type fixtureAuthoringStore struct {
	mu       sync.Mutex
	session  domain.Session
	revision uint64
}

type fixtureTerminalGroupingStore struct {
	mu                   sync.Mutex
	scenario             string
	persisted            domain.Session
	active               domain.Session
	revision             uint64
	coordinationRevision uint64
	activationHistory    []string
	lastActiveTerminalID string
}

type fixturePlayerManagementStore struct {
	mu        sync.Mutex
	revision  uint64
	nextID    uint64
	roster    []fixturePlayerProfile
	broadcast bool
	failNext  string
}

type fixturePlayerProfile struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Intelligence        int    `json:"intelligence"`
	HackerPerkAvailable bool   `json:"hackerPerkAvailable"`
}

type fixtureAddPlayerRequest struct {
	Name                string `json:"name"`
	Intelligence        int    `json:"intelligence"`
	HackerPerkAvailable *bool  `json:"hackerPerkAvailable"`
	ExpectedRevision    uint64 `json:"expectedRevision"`
}

type fixtureUpdatePlayerRequest struct {
	CharacterID         string `json:"characterId"`
	Name                string `json:"name"`
	Intelligence        int    `json:"intelligence"`
	HackerPerkAvailable *bool  `json:"hackerPerkAvailable"`
	ExpectedRevision    uint64 `json:"expectedRevision"`
}

type fixtureDeletePlayerRequest struct {
	CharacterID      string `json:"characterId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

type fixturePlayerManagementState struct {
	Revision     uint64                 `json:"revision"`
	PlayerConfig map[string]any         `json:"playerConfig"`
	Roster       []fixturePlayerProfile `json:"roster"`
	Sessions     []map[string]any       `json:"sessions"`
	Broadcast    map[string]any         `json:"broadcast"`
}

type fixturePlayerManagementResult struct {
	OK    bool                         `json:"ok"`
	Error string                       `json:"error,omitempty"`
	State fixturePlayerManagementState `json:"state"`
}

func (store *fixturePlayerManagementStore) reset() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.revision = fixturePlayerRevision
	store.nextID = 1
	store.roster = nil
	store.broadcast = false
	store.failNext = ""
}

func (store *fixturePlayerManagementStore) snapshot() fixturePlayerManagementState {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.snapshotLocked()
}

func (store *fixturePlayerManagementStore) failNextSave(message string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failNext = strings.TrimSpace(message)
	if store.failNext == "" {
		store.failNext = "fixture player configuration save failed"
	}
}

func (store *fixturePlayerManagementStore) advanceRevision() fixturePlayerManagementState {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.revision++
	return store.snapshotLocked()
}

func (store *fixturePlayerManagementStore) setBroadcast(active bool) fixturePlayerManagementState {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.broadcast != active {
		store.broadcast = active
		store.revision++
	}
	return store.snapshotLocked()
}

func validateFixturePlayerProfile(name string, intelligence int, hackerPerkAvailable *bool) (string, string) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "", "character name must not be blank"
	}
	if len([]rune(trimmedName)) > 80 {
		return "", "character name must be at most 80 characters"
	}
	if intelligence < 1 || intelligence > 10 {
		return "", "Intelligence must be an integer from 1 to 10"
	}
	if hackerPerkAvailable == nil {
		return "", "Hacker perk availability must be selected"
	}
	return trimmedName, ""
}

func (store *fixturePlayerManagementStore) add(request fixtureAddPlayerRequest) fixturePlayerManagementResult {
	store.mu.Lock()
	defer store.mu.Unlock()

	failure := func(message string) fixturePlayerManagementResult {
		return fixturePlayerManagementResult{Error: message, State: store.snapshotLocked()}
	}
	if store.broadcast {
		return failure("player profiles cannot be changed while a broadcast is active")
	}
	name, validationError := validateFixturePlayerProfile(request.Name, request.Intelligence, request.HackerPerkAvailable)
	if validationError != "" {
		return failure(validationError)
	}
	if request.ExpectedRevision != store.revision {
		return failure("coordination state changed; review the latest player list")
	}
	if store.failNext != "" {
		message := store.failNext
		store.failNext = ""
		return failure(message)
	}

	profile := fixturePlayerProfile{
		ID:                  fmt.Sprintf("fixture-player-%d", store.nextID),
		Name:                name,
		Intelligence:        request.Intelligence,
		HackerPerkAvailable: *request.HackerPerkAvailable,
	}
	store.nextID++
	store.roster = append(store.roster, profile)
	store.revision++
	return fixturePlayerManagementResult{OK: true, State: store.snapshotLocked()}
}

func (store *fixturePlayerManagementStore) update(request fixtureUpdatePlayerRequest) fixturePlayerManagementResult {
	store.mu.Lock()
	defer store.mu.Unlock()

	failure := func(message string) fixturePlayerManagementResult {
		return fixturePlayerManagementResult{Error: message, State: store.snapshotLocked()}
	}
	if store.broadcast {
		return failure("player profiles cannot be changed while a broadcast is active")
	}
	name, validationError := validateFixturePlayerProfile(request.Name, request.Intelligence, request.HackerPerkAvailable)
	if validationError != "" {
		return failure(validationError)
	}
	if request.ExpectedRevision != store.revision {
		return failure("coordination state changed; review the latest player list")
	}
	characterID := strings.TrimSpace(request.CharacterID)
	if characterID == "" {
		return failure("character ID must not be blank")
	}
	index := -1
	for candidateIndex := range store.roster {
		if store.roster[candidateIndex].ID == characterID {
			index = candidateIndex
			break
		}
	}
	if index < 0 {
		return failure("character does not exist")
	}
	current := store.roster[index]
	if current.Name == name && current.Intelligence == request.Intelligence && current.HackerPerkAvailable == *request.HackerPerkAvailable {
		return fixturePlayerManagementResult{OK: true, State: store.snapshotLocked()}
	}
	if store.failNext != "" {
		message := store.failNext
		store.failNext = ""
		return failure(message)
	}
	store.roster[index] = fixturePlayerProfile{
		ID:                  current.ID,
		Name:                name,
		Intelligence:        request.Intelligence,
		HackerPerkAvailable: *request.HackerPerkAvailable,
	}
	store.revision++
	return fixturePlayerManagementResult{OK: true, State: store.snapshotLocked()}
}

func (store *fixturePlayerManagementStore) delete(request fixtureDeletePlayerRequest) fixturePlayerManagementResult {
	store.mu.Lock()
	defer store.mu.Unlock()

	failure := func(message string) fixturePlayerManagementResult {
		return fixturePlayerManagementResult{Error: message, State: store.snapshotLocked()}
	}
	if store.broadcast {
		return failure("player profiles cannot be changed while a broadcast is active")
	}
	if request.ExpectedRevision != store.revision {
		return failure("coordination state changed; review the latest player list")
	}
	characterID := strings.TrimSpace(request.CharacterID)
	if characterID == "" {
		return failure("character ID must not be blank")
	}
	index := -1
	for candidateIndex := range store.roster {
		if store.roster[candidateIndex].ID == characterID {
			index = candidateIndex
			break
		}
	}
	if index < 0 {
		return failure("character does not exist")
	}
	if store.failNext != "" {
		message := store.failNext
		store.failNext = ""
		return failure(message)
	}
	store.roster = append(store.roster[:index:index], store.roster[index+1:]...)
	store.revision++
	return fixturePlayerManagementResult{OK: true, State: store.snapshotLocked()}
}

func (store *fixturePlayerManagementStore) snapshotLocked() fixturePlayerManagementState {
	roster := append([]fixturePlayerProfile(nil), store.roster...)
	if roster == nil {
		roster = []fixturePlayerProfile{}
	}
	var broadcast map[string]any
	if store.broadcast {
		broadcast = map[string]any{"id": "fixture-player-management-broadcast"}
	}
	return fixturePlayerManagementState{
		Revision: store.revision,
		PlayerConfig: map[string]any{
			"status": "loaded", "name": "Player management fixture",
			"filePath": "/private/tmp/fallout-player-management.json", "version": 1,
		},
		Roster: roster, Sessions: []map[string]any{}, Broadcast: broadcast,
	}
}

type fixtureTerminalCatalog struct {
	mu      sync.Mutex
	session domain.Session
}

func (catalog *fixtureTerminalCatalog) replace(session domain.Session) {
	catalog.mu.Lock()
	catalog.session = domain.NormalizeTerminalGroups(domain.CloneSession(session))
	catalog.mu.Unlock()
}

func (catalog *fixtureTerminalCatalog) snapshot() domain.Session {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	return domain.CloneSession(catalog.session)
}

func (catalog *fixtureTerminalCatalog) LookupTerminal(id string) (domain.TerminalTarget, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	terminal := fixtureTerminalByID(&catalog.session, id)
	if terminal == nil {
		return domain.TerminalTarget{}, false
	}
	return fixtureTarget(*terminal), true
}

func (catalog *fixtureTerminalCatalog) LookupTerminalGroup(id string) (domain.TerminalGroupSnapshot, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	return domain.TerminalGroupFor(catalog.session, id)
}

func (catalog *fixtureTerminalCatalog) LookupTerminalTransition(sourceID, commandID string) (domain.TerminalTransitionTarget, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	source := fixtureTerminalByID(&catalog.session, sourceID)
	if source == nil {
		return domain.TerminalTransitionTarget{}, false
	}
	command := fixtureNodeByID(&source.Root, commandID)
	if command == nil || command.TerminalTransition == nil {
		return domain.TerminalTransitionTarget{}, false
	}
	target := fixtureTerminalByID(&catalog.session, command.TerminalTransition.TargetTerminalID)
	if target == nil || target.ID == source.ID {
		return domain.TerminalTransitionTarget{}, false
	}
	sourceGroup, sourceGrouped := domain.TerminalGroupFor(catalog.session, source.ID)
	targetGroup, targetGrouped := domain.TerminalGroupFor(catalog.session, target.ID)
	if !sourceGrouped || !targetGrouped || sourceGroup.ID == "" || sourceGroup.ID != targetGroup.ID {
		return domain.TerminalTransitionTarget{}, false
	}
	return domain.TerminalTransitionTarget{
		SourceTerminalID: source.ID, SourceTerminalName: source.Name,
		CommandID: command.ID, CommandName: command.Name, Target: fixtureTarget(*target),
	}, true
}

func fixtureTarget(terminal domain.Terminal) domain.TerminalTarget {
	clone := domain.CloneSession(domain.Session{Version: 1, Name: "fixture", Terminals: []domain.Terminal{terminal}}).Terminals[0]
	return domain.TerminalTarget{TerminalID: clone.ID, TerminalName: clone.Name, Tree: clone.Root, CommandStates: clone.CommandStates, HackLevel: clone.HackLevel, IntroText: clone.IntroText}
}

func fixtureNodeByID(node *domain.ContentNode, id string) *domain.ContentNode {
	if node == nil {
		return nil
	}
	if node.ID == id {
		return node
	}
	for index := range node.Children {
		if found := fixtureNodeByID(&node.Children[index], id); found != nil {
			return found
		}
	}
	return nil
}

func (store *fixtureTerminalGroupingStore) orderedNavigation() bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.scenario == "ordered-navigation"
}

type fixtureSessionStateResult struct {
	OK       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	Revision uint64          `json:"revision"`
	Session  *domain.Session `json:"session,omitempty"`
}

type fixtureResetCommandRequest struct {
	TerminalID string `json:"terminalId"`
	CommandID  string `json:"commandId"`
}

type fixtureResetTerminalRequest struct {
	TerminalID string `json:"terminalId"`
}

func (store *fixtureAuthoringStore) reset() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.session = stateChangingAuthoringSession()
	store.revision = 1
}

func (store *fixtureAuthoringStore) snapshot() fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.resultLocked()
}

func (store *fixtureAuthoringStore) save(session domain.Session) fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.session = cloneFixtureSession(session)
	store.revision++
	return store.resultLocked()
}

func (store *fixtureAuthoringStore) resetCommand(request fixtureResetCommandRequest) fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	terminal := fixtureTerminalByID(&store.session, strings.TrimSpace(request.TerminalID))
	if terminal == nil {
		return fixtureSessionStateResult{Error: "terminal does not exist", Revision: store.revision}
	}
	commandID := strings.TrimSpace(request.CommandID)
	if _, exists := terminal.CommandStates[commandID]; exists {
		delete(terminal.CommandStates, commandID)
		if len(terminal.CommandStates) == 0 {
			terminal.CommandStates = nil
		}
		store.revision++
	}
	return store.resultLocked()
}

func (store *fixtureAuthoringStore) resetTerminal(request fixtureResetTerminalRequest) fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	terminal := fixtureTerminalByID(&store.session, strings.TrimSpace(request.TerminalID))
	if terminal == nil {
		return fixtureSessionStateResult{Error: "terminal does not exist", Revision: store.revision}
	}
	if len(terminal.CommandStates) != 0 {
		terminal.CommandStates = nil
		store.revision++
	}
	return store.resultLocked()
}

func (store *fixtureAuthoringStore) resultLocked() fixtureSessionStateResult {
	session := cloneFixtureSession(store.session)
	return fixtureSessionStateResult{OK: true, Revision: store.revision, Session: &session}
}

func (store *fixtureTerminalGroupingStore) reset(scenario string) error {
	session, err := terminalGroupingSession(scenario)
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.scenario = strings.TrimSpace(scenario)
	if store.scenario == "" {
		store.scenario = "canonical"
	}
	store.persisted = domain.CloneSession(session)
	store.active = normalizeFixtureTerminalGroups(session)
	store.revision = 1
	store.coordinationRevision = 1
	store.activationHistory = nil
	store.lastActiveTerminalID = ""
	return nil
}

func (store *fixtureTerminalGroupingStore) persistedSnapshot() domain.Session {
	store.mu.Lock()
	defer store.mu.Unlock()
	return domain.CloneSession(store.persisted)
}

func (store *fixtureTerminalGroupingStore) activeSnapshot() fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	session := domain.CloneSession(store.active)
	return fixtureSessionStateResult{OK: true, Revision: store.revision, Session: &session}
}

type fixtureTerminalGroupReplacementRequest struct {
	TerminalGroups               []domain.TerminalGroup `json:"terminalGroups"`
	ExpectedSessionRevision      uint64                 `json:"expectedSessionRevision"`
	ExpectedCoordinationRevision uint64                 `json:"expectedCoordinationRevision"`
}

type fixtureTerminalGroupReplacementResult struct {
	OK                bool                            `json:"ok"`
	Error             string                          `json:"error,omitempty"`
	SessionRevision   uint64                          `json:"sessionRevision"`
	Session           *domain.Session                 `json:"session,omitempty"`
	CoordinationState *domain.MasterCoordinationState `json:"coordinationState"`
}

func (store *fixtureTerminalGroupingStore) replaceGroups(request fixtureTerminalGroupReplacementRequest) fixtureTerminalGroupReplacementResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	canonical := domain.CloneSession(store.active)
	result := fixtureTerminalGroupReplacementResult{
		SessionRevision: store.revision,
		Session:         &canonical,
		CoordinationState: &domain.MasterCoordinationState{
			Revision: store.coordinationRevision,
		},
	}
	if request.ExpectedSessionRevision != store.revision || request.ExpectedCoordinationRevision != store.coordinationRevision {
		result.Error = "СОСТОЯНИЕ ИЗМЕНИЛОСЬ; ПРЕДЛОЖЕНИЕ УСТАРЕЛО, ПРОВЕРЬТЕ ГРУППЫ ЕЩЁ РАЗ"
		return result
	}
	if _, err := domain.ValidateTerminalGroupReplacement(store.active, request.TerminalGroups); err != nil {
		result.Error = err.Error()
		return result
	}
	store.active.TerminalGroups = cloneFixtureGroups(request.TerminalGroups)
	store.persisted = domain.CloneSession(store.active)
	store.revision++
	store.coordinationRevision++
	canonical = domain.CloneSession(store.active)
	result.OK = true
	result.SessionRevision = store.revision
	result.Session = &canonical
	result.CoordinationState = &domain.MasterCoordinationState{Revision: store.coordinationRevision}
	return result
}

func (store *fixtureTerminalGroupingStore) advanceRevisions(session, coordination bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if session {
		store.revision++
	}
	if coordination {
		store.coordinationRevision++
	}
}

func (store *fixtureTerminalGroupingStore) setCoordinationRevision(revision uint64) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.coordinationRevision = revision
}

func (store *fixtureTerminalGroupingStore) runtimeStatus(coordination *domain.MasterCoordinationState) map[string]any {
	store.mu.Lock()
	defer store.mu.Unlock()
	if coordination == nil {
		coordination = &domain.MasterCoordinationState{Revision: store.coordinationRevision}
	}
	return map[string]any{
		"requestedRevision": store.revision,
		"savedRevision":     store.revision,
		"coordinationState": coordination,
	}
}

func (store *fixtureTerminalGroupingStore) navigationState(coordination *domain.MasterCoordinationState) map[string]any {
	store.mu.Lock()
	defer store.mu.Unlock()
	activeTerminalID := ""
	if coordination != nil && coordination.Broadcast != nil && coordination.Broadcast.ActiveTerminalID != nil {
		activeTerminalID = *coordination.Broadcast.ActiveTerminalID
	}
	if activeTerminalID != "" && activeTerminalID != store.lastActiveTerminalID {
		store.activationHistory = append(store.activationHistory, activeTerminalID)
		store.lastActiveTerminalID = activeTerminalID
	}
	var pending *domain.MasterPendingTerminalNavigation
	var pendingCommand *domain.MasterPendingCommandExecution
	if coordination != nil {
		pending = coordination.PendingTerminalNavigation
		pendingCommand = coordination.PendingCommandExecution
	}
	return map[string]any{
		"activeTerminalId":          activeTerminalID,
		"pendingCommandExecution":   pendingCommand,
		"pendingTerminalNavigation": pending,
		"activationHistory":         append([]string(nil), store.activationHistory...),
	}
}

func cloneFixtureGroups(groups []domain.TerminalGroup) []domain.TerminalGroup {
	clone := make([]domain.TerminalGroup, len(groups))
	for index, group := range groups {
		clone[index] = group
		clone[index].TerminalIDs = append([]string(nil), group.TerminalIDs...)
	}
	return clone
}

func (store *fixtureTerminalGroupingStore) save(candidate domain.Session) fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()

	terminalByID := make(map[string]domain.Terminal, len(candidate.Terminals))
	for _, terminal := range candidate.Terminals {
		terminalByID[terminal.ID] = terminal
	}
	assigned := make(map[string]struct{}, len(candidate.Terminals))
	groups := make([]domain.TerminalGroup, 0, len(store.active.TerminalGroups)+len(candidate.Terminals))
	for _, group := range store.active.TerminalGroups {
		retainedIDs := make([]string, 0, len(group.TerminalIDs))
		for _, terminalID := range group.TerminalIDs {
			if _, exists := terminalByID[terminalID]; !exists {
				continue
			}
			retainedIDs = append(retainedIDs, terminalID)
			assigned[terminalID] = struct{}{}
		}
		if len(retainedIDs) == 0 {
			continue
		}
		group.TerminalIDs = retainedIDs
		groups = append(groups, group)
	}

	usedGroupIDs := make(map[string]struct{}, len(groups))
	usedGroupNames := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		usedGroupIDs[strings.ToLower(strings.TrimSpace(group.ID))] = struct{}{}
		usedGroupNames[strings.ToLower(strings.TrimSpace(group.Name))] = struct{}{}
	}
	for _, terminal := range candidate.Terminals {
		if _, exists := assigned[terminal.ID]; exists {
			continue
		}
		groupID := uniqueFixtureGroupValue("singleton-"+terminal.ID, usedGroupIDs)
		groupName := uniqueFixtureGroupValue(terminal.Name, usedGroupNames)
		groups = append(groups, domain.TerminalGroup{
			ID: groupID, Name: groupName, TerminalIDs: []string{terminal.ID},
		})
	}

	candidate.TerminalGroups = groups
	store.active = domain.CloneSession(candidate)
	store.persisted = domain.CloneSession(candidate)
	store.revision++
	session := domain.CloneSession(store.active)
	return fixtureSessionStateResult{OK: true, Revision: store.revision, Session: &session}
}

func uniqueFixtureGroupValue(base string, used map[string]struct{}) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "terminal-group"
	}
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", base, suffix)
		}
		key := strings.ToLower(candidate)
		if _, exists := used[key]; exists {
			continue
		}
		used[key] = struct{}{}
		return candidate
	}
}

func normalizeFixtureTerminalGroups(session domain.Session) domain.Session {
	if len(session.Terminals) == 0 || len(session.TerminalGroups) != 0 {
		return domain.CloneSession(session)
	}
	normalized := domain.CloneSession(session)
	normalized.TerminalGroups = make([]domain.TerminalGroup, 0, len(normalized.Terminals))
	usedNames := make(map[string]struct{}, len(normalized.Terminals))
	for _, terminal := range normalized.Terminals {
		normalized.TerminalGroups = append(normalized.TerminalGroups, domain.TerminalGroup{
			ID:          "legacy-singleton-" + terminal.ID,
			Name:        uniqueFixtureGroupValue(terminal.Name, usedNames),
			TerminalIDs: []string{terminal.ID},
		})
	}
	return normalized
}

func fixtureTerminalByID(session *domain.Session, terminalID string) *domain.Terminal {
	if session == nil {
		return nil
	}
	for index := range session.Terminals {
		if session.Terminals[index].ID == terminalID {
			return &session.Terminals[index]
		}
	}
	return nil
}

func cloneFixtureSession(session domain.Session) domain.Session {
	data, err := domain.EncodeSession(session)
	if err != nil {
		panic(err)
	}
	clone, err := domain.DecodeSession(data)
	if err != nil {
		panic(err)
	}
	return clone
}

func (store *fixtureCommandStateStore) reset() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.states = nil
	store.revision = 1
	store.executeWrites = 0
	store.failNext = false
}

func (store *fixtureCommandStateStore) failNextSave() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failNext = true
}

func (store *fixtureCommandStateStore) ExecuteCommandState(_ context.Context, terminalID, commandID string) (control.CommandStateMutation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failNext {
		store.failNext = false
		return control.CommandStateMutation{}, errors.New("fixture atomic persistence failed")
	}
	if terminalID == fixtureFacilityTerminalID {
		if commandID != fixtureFacilityCommandID {
			return control.CommandStateMutation{}, errors.New("fixture facility command identity is invalid")
		}
		changed := false
		if _, completed := store.states[commandID]; !completed {
			if store.states == nil {
				store.states = make(map[string]domain.CommandExecutionState)
			}
			store.states[commandID] = domain.CommandExecutionState{
				CompletedName: "SECURITY DOOR OPEN",
				ResultText:    "Security door and alarm updated.",
			}
			store.revision++
			store.executeWrites++
			changed = true
		}
		return control.CommandStateMutation{
			Changed:  changed,
			Revision: store.revision,
			Session:  facilityPlayerSession(store.states),
		}, nil
	}
	if terminalID != "terminal-stateful" {
		return control.CommandStateMutation{}, errors.New("fixture command identity is invalid")
	}
	command := fixtureStateChangingCommand(commandID)
	if command == nil {
		return control.CommandStateMutation{}, errors.New("fixture command identity is invalid")
	}
	changed := false
	if _, completed := store.states[commandID]; !completed {
		if store.states == nil {
			store.states = make(map[string]domain.CommandExecutionState)
		}
		resultText := command.Text
		if command.ID == "doors" {
			resultText = fixtureCommandResult
		}
		state := domain.CommandExecutionState{
			CompletedName: command.StateChange.CompletedName,
			ResultText:    resultText,
		}
		if change := command.StateChange.EntryContentChange; change != nil {
			clone := *change
			state.EntryContentChange = &clone
		}
		store.states[commandID] = state
		store.revision++
		store.executeWrites++
		changed = true
	}
	return store.mutationLocked(changed), nil
}

func (store *fixtureCommandStateStore) ResetCommandState(_ context.Context, terminalID, commandID string) (control.CommandStateMutation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if terminalID != "terminal-stateful" || commandID != "doors" {
		return control.CommandStateMutation{}, errors.New("fixture command identity is invalid")
	}
	_, changed := store.states[commandID]
	if changed {
		delete(store.states, commandID)
		store.revision++
	}
	return store.mutationLocked(changed), nil
}

func (store *fixtureCommandStateStore) ResetTerminalCommandStates(_ context.Context, terminalID string) (control.CommandStateMutation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if terminalID != "terminal-stateful" {
		return control.CommandStateMutation{}, errors.New("fixture terminal identity is invalid")
	}
	changed := len(store.states) != 0
	if changed {
		store.states = nil
		store.revision++
	}
	return store.mutationLocked(changed), nil
}

func (store *fixtureCommandStateStore) mutationLocked(changed bool) control.CommandStateMutation {
	return control.CommandStateMutation{
		Changed:  changed,
		Revision: store.revision,
		Session:  stateChangingApprovalSession(store.states),
	}
}

func (store *fixtureCommandStateStore) target() domain.TerminalTarget {
	store.mu.Lock()
	defer store.mu.Unlock()
	target := stateChangingApprovalTarget()
	target.CommandStates = cloneFixtureCommandStates(store.states)
	return target
}

func (store *fixtureCommandStateStore) facilityTarget() domain.TerminalTarget {
	store.mu.Lock()
	defer store.mu.Unlock()
	return facilityPlayerTarget(store.states)
}

func (store *fixtureCommandStateStore) facilitySession() domain.Session {
	store.mu.Lock()
	defer store.mu.Unlock()
	return facilityPlayerSession(store.states)
}

func (store *fixtureCommandStateStore) syncTarget() domain.TerminalTarget {
	store.mu.Lock()
	defer store.mu.Unlock()
	target := stateChangingSyncTarget()
	target.CommandStates = cloneFixtureCommandStates(store.states)
	return target
}

func (store *fixtureCommandStateStore) syncSession() domain.Session {
	store.mu.Lock()
	defer store.mu.Unlock()
	return stateChangingSyncSession(store.states)
}

func (store *fixtureCommandStateStore) session() domain.Session {
	store.mu.Lock()
	defer store.mu.Unlock()
	return stateChangingApprovalSession(store.states)
}

func (store *fixtureCommandStateStore) audit() (int, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	_, completed := store.states["doors"]
	return store.executeWrites, completed
}

func (random *fixtureRandom) Intn(limit int) int {
	if limit <= 1 {
		return 0
	}
	random.mu.Lock()
	defer random.mu.Unlock()
	if len(random.forced) != 0 {
		value := random.forced[0]
		random.forced = random.forced[1:]
		if value < 0 {
			return limit - 1
		}
		return value % limit
	}
	random.state = random.state*6364136223846793005 + 1442695040888963407
	return int(random.state % uint64(limit))
}

func (random *fixtureRandom) forceDudRemoval(position string) bool {
	random.mu.Lock()
	defer random.mu.Unlock()
	switch position {
	case "revealed":
		random.forced = []int{0, 0}
	case "pending":
		random.forced = []int{0, -1}
	default:
		return false
	}
	return true
}

func (random *fixtureRandom) reset() {
	random.mu.Lock()
	defer random.mu.Unlock()
	random.state = fixtureRandomSeed
	random.forced = nil
}

type fixtureEdge struct {
	active           atomic.Bool
	publicGeneration atomic.Uint64
	service          *control.Service
	connect          *player.ConnectService
	hub              *player.SubscriptionHub
	ingress          tunnel.PublicIngress
	publicURL        string
}

type fixturePresentationGateState struct {
	release     chan struct{}
	blocked     chan struct{}
	unary       bool
	canceled    bool
	releaseOnce sync.Once
	blockedOnce sync.Once
}

// fixturePresentationGate makes presentation races deterministic: it pauses
// exactly one streamed or unary presentation before canonical dispatch.
type fixturePresentationGate struct {
	*control.Service
	context context.Context
	mu      sync.Mutex
	state   *fixturePresentationGateState
}

func (gate *fixturePresentationGate) arm() {
	gate.armTransport(false)
}

func (gate *fixturePresentationGate) armUnary() {
	gate.armTransport(true)
}

func (gate *fixturePresentationGate) armTransport(unary bool) {
	gate.cancel()
	gate.mu.Lock()
	gate.state = &fixturePresentationGateState{
		release: make(chan struct{}),
		blocked: make(chan struct{}),
		unary:   unary,
	}
	gate.mu.Unlock()
}

func (gate *fixturePresentationGate) cancel() {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.state == nil {
		return
	}
	gate.state.canceled = true
	gate.state.releaseOnce.Do(func() { close(gate.state.release) })
}

func (gate *fixturePresentationGate) release() {
	gate.mu.Lock()
	state := gate.state
	gate.mu.Unlock()
	if state == nil {
		return
	}
	state.releaseOnce.Do(func() { close(state.release) })
}

func (gate *fixturePresentationGate) cancelAfter(delay time.Duration) {
	gate.mu.Lock()
	if gate.state == nil {
		gate.mu.Unlock()
		return
	}
	state := gate.state
	state.canceled = true
	gate.mu.Unlock()
	time.AfterFunc(delay, func() {
		state.releaseOnce.Do(func() { close(state.release) })
	})
}

func (gate *fixturePresentationGate) isBlocked() bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.state == nil {
		return false
	}
	select {
	case <-gate.state.blocked:
		return true
	default:
		return false
	}
}

func (gate *fixturePresentationGate) reset() {
	gate.cancel()
	gate.mu.Lock()
	gate.state = nil
	gate.mu.Unlock()
}

func (gate *fixturePresentationGate) DispatchPlayerAction(connectionID domain.ConnectionID, command domain.RuntimeCommand) domain.ActionResult {
	return gate.dispatchPresentation(command, false, func() domain.ActionResult {
		return gate.Service.DispatchPlayerAction(connectionID, command)
	})
}

func (gate *fixturePresentationGate) DispatchPlayerActionForRecognition(handle domain.RecognitionHandle, command domain.RuntimeCommand) domain.ActionResult {
	return gate.dispatchPresentation(command, true, func() domain.ActionResult {
		return gate.Service.DispatchPlayerActionForRecognition(handle, command)
	})
}

func (gate *fixturePresentationGate) dispatchPresentation(command domain.RuntimeCommand, unary bool, dispatch func() domain.ActionResult) domain.ActionResult {
	gate.mu.Lock()
	state := gate.state
	gate.mu.Unlock()
	if command.Kind != domain.RuntimeCommandPresentation || state == nil || state.unary != unary {
		return dispatch()
	}

	state.blockedOnce.Do(func() { close(state.blocked) })
	contextClosed := false
	select {
	case <-state.release:
	case <-gate.context.Done():
		contextClosed = true
	}
	gate.mu.Lock()
	canceled := state.canceled || contextClosed
	if gate.state == state {
		gate.state = nil
	}
	gate.mu.Unlock()
	if canceled {
		return domain.ActionResult{
			RequestID: command.RequestID,
			Reason:    domain.ActionReasonControllerDisconnected,
			Revision:  gate.Service.Snapshot().Revision,
		}
	}
	return dispatch()
}

type fixtureEdgeStatus struct {
	AuthBoundary           string `json:"authBoundary"`
	Upstream               string `json:"upstream"`
	Active                 bool   `json:"active"`
	AuthorizationForwarded bool   `json:"authorizationForwarded"`
	PublicURL              string `json:"publicUrl"`
}

func (source *ids) Next() string {
	return fmt.Sprintf("browser-fixture-%d", source.next.Add(1))
}

func (edge *fixtureEdge) reset() error {
	if edge.ingress == nil || edge.ingress.URL() == nil {
		return errors.New("fixture ingress is unavailable")
	}
	host := edge.ingress.URL().Host
	if publicURL, err := url.Parse(edge.publicURL); err == nil && publicURL.Host != "" {
		host = publicURL.Host
	}
	if err := edge.ingress.Activate(host, fixtureEdgeUsername, []byte(fixtureEdgePassword)); err != nil {
		return err
	}
	edge.active.Store(true)
	edge.publicGeneration.Store(0)
	return nil
}

type publicAccessFixtureSnapshot struct {
	Preferences            publicAccessFixturePreferences `json:"preferences"`
	ProviderTokenPresence  string                         `json:"providerTokenPresence"`
	PlayerPasswordPresence string                         `json:"playerPasswordPresence"`
	Status                 publicAccessFixtureStatus      `json:"status"`
}

type publicAccessFixturePreferences struct {
	Version  int    `json:"version"`
	Username string `json:"username"`
	Revision uint64 `json:"revision"`
}

type publicAccessFixtureStatus struct {
	State            string `json:"state"`
	Generation       uint64 `json:"generation"`
	SettingsRevision uint64 `json:"settingsRevision"`
	PublicURL        string `json:"publicUrl,omitempty"`
	ErrorCategory    string `json:"errorCategory,omitempty"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
}

func (edge *fixtureEdge) publicFailure(kind string) (publicAccessFixtureSnapshot, bool) {
	failures := map[string][2]string{
		"invalid-token":        {"provider_authentication", "The provider rejected the account credential."},
		"revoked-token":        {"provider_authentication", "The provider rejected the account credential."},
		"no-network":           {"network_unavailable", "The network is unavailable; local access remains available."},
		"dns-timeout":          {"timeout", "Public access did not become ready in time."},
		"domain-conflict":      {"domain_unavailable", "The reserved domain is unavailable for this account."},
		"keychain-locked":      {"secret_store_locked", "Unlock Keychain and try again."},
		"keychain-denied":      {"secret_store_denied", "Allow Keychain access and try again."},
		"keychain-unavailable": {"secret_store_unavailable", "Keychain is unavailable; local access remains available."},
		"policy-failure":       {"provider_failure", "The public-access provider stopped unexpectedly."},
		"provider-failure":     {"provider_failure", "The public-access provider stopped unexpectedly."},
		"unexpected-done":      {"provider_failure", "The public-access provider stopped unexpectedly."},
		"close-failure":        {"provider_failure", "The public-access provider stopped unexpectedly."},
	}
	if kind == "stale-completion" {
		generation := edge.publicGeneration.Load()
		if generation > 0 {
			generation--
		}
		return edge.publicSnapshot(publicAccessFixtureStatus{
			State: "ready", Generation: generation, PublicURL: "https://stale.invalid",
		}), true
	}
	failure, ok := failures[kind]
	if !ok {
		return publicAccessFixtureSnapshot{}, false
	}
	return edge.publicSnapshot(publicAccessFixtureStatus{
		State: "error", Generation: edge.publicGeneration.Add(1),
		ErrorCategory: failure[0], ErrorMessage: failure[1],
	}), true
}

func (edge *fixtureEdge) publicRecovery() publicAccessFixtureSnapshot {
	return edge.publicSnapshot(publicAccessFixtureStatus{
		State: "ready", Generation: edge.publicGeneration.Add(1), PublicURL: "https://recovered.example",
	})
}

func (edge *fixtureEdge) publicSnapshot(status publicAccessFixtureStatus) publicAccessFixtureSnapshot {
	status.SettingsRevision = 0
	return publicAccessFixtureSnapshot{
		Preferences:           publicAccessFixturePreferences{Version: 1, Username: "players", Revision: 0},
		ProviderTokenPresence: "present", PlayerPasswordPresence: "present", Status: status,
	}
}

func (edge *fixtureEdge) update(response http.ResponseWriter) {
	updated := fixtureTerminal()
	updated.Tree.Children[0].Children = append(updated.Tree.Children[0].Children, domain.ContentNode{
		ID: "public-update", Type: domain.NodeEntry, Name: "PUBLIC UPDATE", Description: "STREAM UPDATE RECEIVED",
	})
	if _, err := edge.service.UpdateLiveTerminal(updated.Tree, &updated.IntroText); err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (edge *fixtureEdge) activateHacking(response http.ResponseWriter) {
	target := fixtureTerminal()
	target.TerminalID = "terminal-hacking"
	target.TerminalName = "Security"
	target.HackLevel = 1
	if _, err := edge.service.RequestTerminalActivation(target); err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func activateCRTTerminal(service *control.Service, response http.ResponseWriter, target domain.TerminalTarget) bool {
	state, err := service.RequestTerminalActivation(target)
	if err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return false
	}
	if state.PendingSwitch == nil {
		return true
	}
	if _, err = service.ResolveTerminalSwitch(state.PendingSwitch.SwitchID, domain.TerminalSwitchDiscard); err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return false
	}
	return true
}

func activateLifecycleTerminal(service *control.Service, target domain.TerminalTarget) error {
	state, err := service.RequestTerminalActivation(target)
	if err != nil {
		return err
	}
	if state.PendingSwitch == nil {
		return nil
	}
	_, err = service.ResolveTerminalSwitch(state.PendingSwitch.SwitchID, domain.TerminalSwitchDiscard)
	return err
}

func activatePreservingTerminal(service *control.Service, target domain.TerminalTarget) error {
	state, err := service.RequestTerminalActivation(target)
	if err != nil {
		return err
	}
	if state.PendingSwitch == nil {
		return nil
	}
	_, err = service.ResolveTerminalSwitch(state.PendingSwitch.SwitchID, domain.TerminalSwitchPreserve)
	return err
}

func restartStateChangingBroadcast(service *control.Service, target domain.TerminalTarget) error {
	if _, err := service.EndBroadcast(); err != nil {
		return err
	}
	if _, err := service.StartBroadcast(); err != nil {
		return err
	}
	return activateLifecycleTerminal(service, target)
}

func facilityPlayerCoordinationState(service *control.Service) (map[string]any, error) {
	encoded, err := json.Marshal(service.Snapshot())
	if err != nil {
		return nil, fmt.Errorf("encode facility coordination state: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(encoded, &state); err != nil {
		return nil, fmt.Errorf("decode facility coordination state: %w", err)
	}
	pending, ok := state["pendingCommandExecution"].(map[string]any)
	if !ok || pending["commandId"] != fixtureFacilityCommandID {
		return state, nil
	}
	pending["facilityAction"] = map[string]any{
		"expectedFacilityRevision": 0,
		"deviceIds":                []string{fixtureFacilityDoorID, fixtureFacilityAlarmID},
		"conditionIds":             []string{fixtureFacilityCondition},
	}
	return state, nil
}

func resolveFacilityPlayerCommand(
	ctx context.Context,
	service *control.Service,
	fixture *fixtureFacilityPlayerState,
	requestID string,
	decision domain.CommandExecutionDecision,
) (*domain.MasterCoordinationState, fixtureFacilityResult, bool, error) {
	if requestID == "" {
		if pending := service.Snapshot().PendingCommandExecution; pending != nil {
			requestID = pending.RequestID
		} else {
			requestID = fixture.currentRequestID()
		}
	}
	scenario := fixture.scenarioForAttempt(requestID)
	if decision == domain.CommandExecutionApprove &&
		(scenario == "stale-revision" || scenario == "persistence-failure" || scenario == "conflict") {
		failure := domain.FacilityFailureCode(scenario)
		if scenario == "persistence-failure" {
			failure = domain.FacilityFailurePersistenceFailed
		}
		coordination, _, _, resolveErr := service.ResolveCommandExecution(ctx, requestID, domain.CommandExecutionReject)
		return coordination, fixture.recordFailure(requestID, failure), false, resolveErr
	}

	coordination, _, facilityResult, resolveErr := service.ResolveCommandExecution(ctx, requestID, decision)
	if resolveErr != nil && facilityResult == nil {
		return coordination, fixture.recordDuplicate(requestID), false, resolveErr
	}
	if decision == domain.CommandExecutionReject {
		return coordination, fixture.recordRejected(requestID), resolveErr == nil, resolveErr
	}
	if facilityResult != nil {
		if facilityResult.Failure == domain.FacilityFailureDuplicate {
			return coordination, fixture.recordDuplicate(requestID), false, resolveErr
		}
		result := fixtureFacilityResult{
			OK: facilityResult.OK, Changed: facilityResult.Changed, CorrelationID: facilityResult.CorrelationID,
			Failure:                  facilityResult.Failure,
			PreviousFacilityRevision: facilityResult.PreviousFacilityRevision, ResultingFacilityRevision: facilityResult.ResultingFacilityRevision,
			AffectedDeviceIDs:    slices.Clone(facilityResult.AffectedDeviceIDs),
			AffectedConditionIDs: slices.Clone(facilityResult.AffectedConditionIDs),
		}
		return coordination, result, result.OK, resolveErr
	}
	return coordination, fixture.recordDuplicate(requestID), false, resolveErr
}

func main() {
	rootContext := context.Background()
	ctx, stop := signal.NotifyContext(rootContext, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func validFixtureRuntimeLogID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' || character == ':' {
			continue
		}
		return false
	}
	return true
}

func run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("fixture context is required")
	}
	playerAssets, err := fs.Sub(os.DirFS("../../frontend/client"), "dist")
	if err != nil {
		return fmt.Errorf("open built player assets: %w", err)
	}
	fixtureHackRandom := &fixtureRandom{state: fixtureRandomSeed}
	approvalStore := &fixtureCommandStateStore{}
	approvalStore.reset()
	facilityPlayerState := &fixtureFacilityPlayerState{}
	if err := facilityPlayerState.reset("ready"); err != nil {
		return fmt.Errorf("reset facility player-state fixture: %w", err)
	}
	facilityLifecycleState := &fixtureFacilityLifecycleState{}
	if err := facilityLifecycleState.reset("persisted"); err != nil {
		return fmt.Errorf("reset facility lifecycle fixture: %w", err)
	}
	facilityAuthoringState := &fixtureFacilityAuthoringState{}
	if err := facilityAuthoringState.reset("authored"); err != nil {
		return fmt.Errorf("reset facility authoring fixture: %w", err)
	}
	facilityDiagnosticState := &fixtureFacilityDiagnosticState{}
	authoringStore := &fixtureAuthoringStore{}
	authoringStore.reset()
	terminalGroupingStore := &fixtureTerminalGroupingStore{}
	if err := terminalGroupingStore.reset("canonical"); err != nil {
		return fmt.Errorf("reset terminal-grouping fixture: %w", err)
	}
	playerManagementStore := &fixturePlayerManagementStore{}
	playerManagementStore.reset()
	runtimeLogRoot, err := os.MkdirTemp("", "fallout-terminal-browser-runtime-logs-")
	if err != nil {
		return fmt.Errorf("create retained-log fixture directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(runtimeLogRoot) }()
	runtimeLogDirectory := filepath.Join(runtimeLogRoot, "logs")
	if err := os.MkdirAll(runtimeLogDirectory, 0o700); err != nil {
		return fmt.Errorf("create retained-log fixture: %w", err)
	}
	runtimeLogPath := filepath.Join(runtimeLogDirectory, "application-current.log")
	var runtimeLogMu sync.RWMutex
	navigationCatalog := &fixtureTerminalCatalog{}
	navigationCatalog.replace(terminalNavigationSession())
	var navigationPending atomic.Bool
	var navigationProjectionRevision atomic.Uint64
	liveService := live.New(fixtureHackRandom, nil)
	facilityLifecycle := &fixtureFacilityLifecycle{
		base: liveService, state: facilityPlayerState, diagnostics: facilityDiagnosticState,
	}
	var connectPlayer *player.ConnectService
	service := control.New(control.Config{
		IDs:               &ids{},
		Runtime:           liveService,
		Terminals:         facilityLifecycle,
		TrustedHack:       liveService,
		CommandStateStore: approvalStore,
		FacilityStore:     facilityPlayerState,
		TerminalCatalog:   navigationCatalog,
		Enqueue: func(effect control.Effect) {
			if connectPlayer != nil {
				connectPlayer.PublishEffect(effect)
			}
		},
	})
	if _, err := service.ReplaceFacility(facilityPlayerSession(nil).Facility); err != nil {
		return fmt.Errorf("install facility player-state fixture: %w", err)
	}
	presentationGate := &fixturePresentationGate{Service: service, context: ctx}
	presentationHub := player.NewSubscriptionHub()
	connectPlayer, err = player.NewConnectService(player.ConnectServiceConfig{
		Coordinator: presentationGate,
		Hub:         presentationHub,
		Assets:      playerAssets,
	})
	if err != nil {
		return fmt.Errorf("construct fixture Connect service: %w", err)
	}
	rpcPath, rpcHandler := player.NewConnectHandler(connectPlayer)
	applicationHandler := player.NewApplicationHandler(playerAssets, rpcPath, rpcHandler)
	edge := &fixtureEdge{service: service, connect: connectPlayer, hub: presentationHub}

	for _, name := range []string{"Mara", "Boone", "Arcade", "Cass", "Veronica", "Raul", "Lily"} {
		if _, err := service.AddCharacter(domain.CharacterCreatePayload{
			Name: name, Intelligence: 1, ExpectedRevision: service.Snapshot().Revision,
		}); err != nil {
			return err
		}
	}
	if _, err := service.StartBroadcast(); err != nil {
		return err
	}
	if _, err := service.RequestTerminalActivation(fixtureTerminal()); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /__fixture/desktop-api", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response, `<!doctype html><meta charset="utf-8">
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>
<script type="module" src="/__fixture/desktop-api.js"></script>`)
	})
	mux.HandleFunc("GET /__fixture/player-management", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.Replace(page, `class="start-screen" id="startScreen"`, `class="start-screen" id="startScreen" style="display:none"`, 1)
		page = strings.Replace(page, `id="mainLayout" style="display:none"`, `id="mainLayout" style="display:flex"`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/facility-diagnostics/overseer.js", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/overseer.js"))
		if readErr != nil {
			http.Error(response, "fixture overseer script is unavailable", http.StatusInternalServerError)
			return
		}
		script := strings.Replace(string(raw), `>ОДОБРИТЬ</button>`, `>ПОДТВЕРДИТЬ</button>`, 1)
		response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = response.Write([]byte(script))
	})
	mux.HandleFunc("POST /__fixture/player-management/reset", func(response http.ResponseWriter, _ *http.Request) {
		playerManagementStore.reset()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/player-management/state", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.snapshot())
	})
	mux.HandleFunc("POST /__fixture/player-management/add", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload fixtureAddPlayerRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid player profile", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.add(payload))
	})
	mux.HandleFunc("POST /__fixture/player-management/update", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload fixtureUpdatePlayerRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid player profile", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.update(payload))
	})
	mux.HandleFunc("POST /__fixture/player-management/delete", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload fixtureDeletePlayerRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid player deletion", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.delete(payload))
	})
	mux.HandleFunc("POST /__fixture/player-management/set-broadcast", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload struct {
			Active *bool `json:"active"`
		}
		if err := decoder.Decode(&payload); err != nil || payload.Active == nil {
			http.Error(response, "invalid broadcast state", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.setBroadcast(*payload.Active))
	})
	mux.HandleFunc("POST /__fixture/player-management/fail-next-save", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(request.Body).Decode(&payload)
		playerManagementStore.failNextSave(payload.Error)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/player-management/advance-revision", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.advanceRevision())
	})
	mux.HandleFunc("GET /__fixture/public-access-settings", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := string(raw)
		page = strings.Replace(page, `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.Replace(page, `class="start-screen" id="startScreen"`, `class="start-screen" id="startScreen" style="display:none"`, 1)
		page = strings.Replace(page, `id="mainLayout" style="display:none"`, `id="mainLayout" style="display:flex"`, 1)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("POST /__fixture/runtime-logs/seed", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload struct {
			CorrelationID string            `json:"correlationId"`
			Forbidden     map[string]string `json:"forbidden"`
		}
		if err := decoder.Decode(&payload); err != nil || !validFixtureRuntimeLogID(payload.CorrelationID) {
			http.Error(response, "invalid retained-log fixture", http.StatusBadRequest)
			return
		}
		contents := fmt.Sprintf(
			"event=command.request_received outcome=pending request_id=%s role=active\n"+
				"event=facility.request_received outcome=pending request_id=%s correlation_id=%s facility_action=command previous_facility_revision=7 resulting_facility_revision=7\n"+
				"event=facility.decision decision=approve outcome=succeeded request_id=%s correlation_id=%s facility_action=command previous_facility_revision=7 resulting_facility_revision=8\n",
			payload.CorrelationID,
			payload.CorrelationID,
			payload.CorrelationID,
			payload.CorrelationID,
			payload.CorrelationID,
		)
		runtimeLogMu.Lock()
		err := os.WriteFile(runtimeLogPath, []byte(contents), 0o600)
		runtimeLogMu.Unlock()
		if err != nil {
			http.Error(response, "retained-log fixture is unavailable", http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/runtime-logs/current", func(response http.ResponseWriter, _ *http.Request) {
		runtimeLogMu.RLock()
		contents, err := os.ReadFile(runtimeLogPath)
		runtimeLogMu.RUnlock()
		if err != nil {
			http.Error(response, "retained-log fixture is unavailable", http.StatusNotFound)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(struct {
			Path     string `json:"path"`
			Contents string `json:"contents"`
		}{Path: runtimeLogPath, Contents: string(contents)})
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-authoring", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-authoring/reset", func(response http.ResponseWriter, _ *http.Request) {
		authoringStore.reset()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-authoring/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(authoringStore.snapshot())
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-authoring/save", func(response http.ResponseWriter, request *http.Request) {
		var session domain.Session
		if err := json.NewDecoder(request.Body).Decode(&session); err != nil {
			http.Error(response, "invalid authoring session", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(authoringStore.save(session))
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-authoring/reset-command", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureResetCommandRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid reset-command request", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(authoringStore.resetCommand(payload))
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-authoring/reset-terminal", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureResetTerminalRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid reset-terminal request", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(authoringStore.resetTerminal(payload))
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/terminal-grouping/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/desktop-bindings.js", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = fmt.Fprint(response, `export * from "/__fixture/desktop-bindings.js";

export async function OpenSession() {
  const response = await fetch("/__fixture/terminal-grouping/open-session");
  const result = response.ok ? await response.json() : null;
  return {
    ok: response.ok && result?.ok === true,
    error: response.ok ? (result?.error ?? "") : "terminal grouping fixture is unavailable",
    filePath: "/private/tmp/fallout-terminal-grouping.json",
    session: result?.session ?? null,
  };
}

export async function SaveSession(session) {
  const retained = structuredClone(session);
  const response = await fetch("/__fixture/terminal-grouping/save", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(retained),
  });
  const result = response.ok ? await response.json() : null;
  return {
    ok: response.ok && result?.ok === true,
    error: response.ok ? (result?.error ?? "") : "terminal grouping save failed",
    savedRevision: Number(result?.revision ?? 0),
  };
}

export async function RequestTerminalActivation(payload) {
  const response = await fetch("/__fixture/terminal-grouping/activate-terminal", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  return response.ok
    ? response.json()
    : { ok: false, error: "terminal grouping activation fixture is unavailable" };
}
`)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/reset", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Scenario string `json:"scenario"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid terminal-grouping reset", http.StatusBadRequest)
			return
		}
		if err := terminalGroupingStore.reset(payload.Scenario); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		active := terminalGroupingStore.activeSnapshot().Session
		if active == nil || len(active.Terminals) == 0 {
			http.Error(response, "terminal-grouping scenario has no terminal", http.StatusInternalServerError)
			return
		}
		navigationCatalog.replace(*active)
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		initialTerminalID := active.Terminals[0].ID
		if strings.TrimSpace(payload.Scenario) == "ordered-navigation" {
			initialTerminalID = "gamma"
		}
		initial, ok := navigationCatalog.LookupTerminal(initialTerminalID)
		if !ok {
			http.Error(response, "terminal-grouping initial terminal is unavailable", http.StatusInternalServerError)
			return
		}
		if _, err := service.RequestTerminalActivation(initial); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		terminalGroupingStore.setCoordinationRevision(service.Snapshot().Revision)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.persistedSnapshot())
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/open-session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.activeSnapshot())
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/save", func(response http.ResponseWriter, request *http.Request) {
		var candidate domain.Session
		if err := json.NewDecoder(request.Body).Decode(&candidate); err != nil {
			http.Error(response, "invalid terminal-grouping session", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.save(candidate))
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/status", func(response http.ResponseWriter, _ *http.Request) {
		var coordination *domain.MasterCoordinationState
		if terminalGroupingStore.orderedNavigation() {
			coordination = service.Snapshot()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.runtimeStatus(coordination))
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/navigation-state", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.navigationState(service.Snapshot()))
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/activate-terminal", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			TerminalID string `json:"terminalId"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid terminal grouping activation", http.StatusBadRequest)
			return
		}
		target, ok := navigationCatalog.LookupTerminal(payload.TerminalID)
		if !ok {
			http.Error(response, "terminal grouping activation target is unavailable", http.StatusNotFound)
			return
		}
		state, err := service.RequestTerminalActivation(target)
		result := map[string]any{"ok": err == nil, "status": "activated", "state": state}
		if err != nil {
			result["error"] = err.Error()
			result["status"] = ""
		} else if state.PendingSwitch != nil {
			result["status"] = "decision-required"
			result["switchId"] = state.PendingSwitch.SwitchID
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/resolve-navigation", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string                            `json:"requestId"`
			Decision  domain.TerminalNavigationDecision `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid terminal grouping navigation decision", http.StatusBadRequest)
			return
		}
		state, err := service.ResolveTerminalNavigation(payload.RequestID, payload.Decision)
		result := map[string]any{"ok": err == nil, "state": state}
		if err != nil {
			result["error"] = err.Error()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/resolve-command", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string                          `json:"requestId"`
			Decision  domain.CommandExecutionDecision `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid terminal grouping command decision", http.StatusBadRequest)
			return
		}
		state, _, _, err := service.ResolveCommandExecution(request.Context(), payload.RequestID, payload.Decision)
		result := map[string]any{"ok": err == nil, "state": state}
		if err != nil {
			result["error"] = err.Error()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/replace-groups", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureTerminalGroupReplacementRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid terminal group replacement", http.StatusBadRequest)
			return
		}
		result := terminalGroupingStore.replaceGroups(payload)
		if result.OK && result.Session != nil {
			navigationCatalog.replace(*result.Session)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/advance-revisions", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Session      bool `json:"session"`
			Coordination bool `json:"coordination"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid revision advance", http.StatusBadRequest)
			return
		}
		terminalGroupingStore.advanceRevisions(payload.Session, payload.Coordination)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/terminal-navigation/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/terminal-navigation/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(navigationCatalog.snapshot())
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/save", func(response http.ResponseWriter, request *http.Request) {
		var candidate domain.Session
		if err := json.NewDecoder(request.Body).Decode(&candidate); err != nil {
			http.Error(response, "invalid terminal navigation session", http.StatusBadRequest)
			return
		}
		candidate.TerminalGroups = navigationCatalog.snapshot().TerminalGroups
		candidate = domain.EnsureTerminalGroups(candidate)
		if err := domain.ValidateSession(candidate); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		navigationCatalog.replace(candidate)
		navigationProjectionRevision.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"ok": true, "revision": navigationProjectionRevision.Load()})
	})
	mux.HandleFunc("GET /__fixture/terminal-navigation/state", func(response http.ResponseWriter, _ *http.Request) {
		state := service.Snapshot()
		state.Revision += navigationProjectionRevision.Load()
		if navigationPending.Load() {
			state.PendingTerminalNavigation = &domain.MasterPendingTerminalNavigation{
				RequestID: "navigation-forward-1", BroadcastID: state.Broadcast.ID,
				Direction:        domain.TerminalNavigationForward,
				SourceTerminalID: "residential", SourceTerminalName: "Жилой терминал",
				CommandID: "go-security", CommandName: "ПЕРЕЙТИ В ОХРАНУ",
				TargetTerminalID: "security", TargetTerminalName: "Терминал охраны",
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(state)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/reset", func(response http.ResponseWriter, _ *http.Request) {
		fixtureHackRandom.reset()
		navigationCatalog.replace(terminalNavigationSession())
		navigationPending.Store(false)
		navigationProjectionRevision.Store(0)
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		source, _ := navigationCatalog.LookupTerminal("residential")
		if _, err := service.RequestTerminalActivation(source); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/group-full-route", func(response http.ResponseWriter, _ *http.Request) {
		session := navigationCatalog.snapshot()
		session.TerminalGroups = []domain.TerminalGroup{{
			ID: "navigation-full-route", Name: "Полный навигационный маршрут",
			TerminalIDs: []string{"residential", "security", "vault"},
		}}
		navigationCatalog.replace(session)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/pending-forward", func(response http.ResponseWriter, _ *http.Request) {
		navigationPending.Store(true)
		navigationProjectionRevision.Add(1)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/resolve", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string `json:"requestId"`
			Decision  string `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || (payload.Decision != "approve" && payload.Decision != "reject") {
			http.Error(response, "invalid terminal navigation decision", http.StatusBadRequest)
			return
		}
		var state *domain.MasterCoordinationState
		if navigationPending.Load() && payload.RequestID == "navigation-forward-1" {
			navigationPending.Store(false)
			navigationProjectionRevision.Add(1)
			state = service.Snapshot()
		} else if pending := service.Snapshot().PendingCommandExecution; pending != nil && pending.RequestID == payload.RequestID {
			resolved, _, _, resolveErr := service.ResolveCommandExecution(request.Context(), payload.RequestID, domain.CommandExecutionDecision(payload.Decision))
			if resolveErr != nil {
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]any{"ok": false, "error": resolveErr.Error(), "state": resolved})
				return
			}
			state = resolved
		} else {
			decision := domain.TerminalNavigationReject
			if payload.Decision == "approve" {
				decision = domain.TerminalNavigationApprove
			}
			resolved, resolveErr := service.ResolveTerminalNavigation(payload.RequestID, decision)
			if resolveErr != nil {
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]any{"ok": false, "error": resolveErr.Error(), "state": resolved})
				return
			}
			state = resolved
		}
		state.Revision += navigationProjectionRevision.Load()
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"ok": true, "state": state})
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/switch-source", func(response http.ResponseWriter, _ *http.Request) {
		target, _ := navigationCatalog.LookupTerminal("residential")
		if err := activatePreservingTerminal(service, target); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/switch-target", func(response http.ResponseWriter, _ *http.Request) {
		target, _ := navigationCatalog.LookupTerminal("security")
		if err := activatePreservingTerminal(service, target); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/force-hack", func(response http.ResponseWriter, _ *http.Request) {
		if _, ok := service.ForceHackSuccess(); !ok {
			http.Error(response, "active terminal has no unfinished hack", http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/move-source-folder", func(response http.ResponseWriter, _ *http.Request) {
		session := navigationCatalog.snapshot()
		source := fixtureTerminalByID(&session, "residential")
		if source == nil {
			http.Error(response, "source terminal is unavailable", http.StatusConflict)
			return
		}
		var navigation domain.ContentNode
		for index := range source.Root.Children {
			if source.Root.Children[index].ID == "navigation" {
				navigation = source.Root.Children[index]
				source.Root.Children = append(source.Root.Children[:index], source.Root.Children[index+1:]...)
				break
			}
		}
		source.Root.Children = append(source.Root.Children, domain.ContentNode{
			ID: "archive", Type: domain.NodeFolder, Name: "АРХИВ", Children: []domain.ContentNode{navigation},
		})
		navigationCatalog.replace(session)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/delete-source-folder", func(response http.ResponseWriter, _ *http.Request) {
		session := navigationCatalog.snapshot()
		source := fixtureTerminalByID(&session, "residential")
		if source == nil {
			http.Error(response, "source terminal is unavailable", http.StatusConflict)
			return
		}
		for index := range source.Root.Children {
			if source.Root.Children[index].ID == "navigation" {
				source.Root.Children = append(source.Root.Children[:index], source.Root.Children[index+1:]...)
				break
			}
		}
		navigationCatalog.replace(session)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/remove-target", func(response http.ResponseWriter, _ *http.Request) {
		session := navigationCatalog.snapshot()
		for index := range session.Terminals {
			if session.Terminals[index].ID == "security" {
				session.Terminals = append(session.Terminals[:index], session.Terminals[index+1:]...)
				break
			}
		}
		navigationCatalog.replace(session)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/new-broadcast", func(response http.ResponseWriter, _ *http.Request) {
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		source, _ := navigationCatalog.LookupTerminal("residential")
		if _, err := service.RequestTerminalActivation(source); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-approval/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-approval/state", func(response http.ResponseWriter, _ *http.Request) {
		state := service.Snapshot()
		if state.PendingCommandExecution != nil {
			state.PendingCommandExecution.RequestID = fixtureApprovalRequestID
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(state)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-approval/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(approvalStore.session())
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-approval/audit", func(response http.ResponseWriter, _ *http.Request) {
		executeWrites, completed := approvalStore.audit()
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"executeWrites": executeWrites,
			"completed":     completed,
		})
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-approval/reset", func(response http.ResponseWriter, _ *http.Request) {
		approvalStore.reset()
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := service.RequestTerminalActivation(approvalStore.target()); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-approval/reemit-pending", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-approval/fail-next-save", func(response http.ResponseWriter, _ *http.Request) {
		approvalStore.failNextSave()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-approval/resolve", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string `json:"requestId"`
			Decision  string `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid fixture command decision", http.StatusBadRequest)
			return
		}
		requestID := payload.RequestID
		if requestID == fixtureApprovalRequestID {
			if current := service.Snapshot().PendingCommandExecution; current != nil {
				requestID = current.RequestID
			}
		}
		state, _, _, resolveErr := service.ResolveCommandExecution(request.Context(), requestID, domain.CommandExecutionDecision(payload.Decision))
		result := map[string]any{"ok": resolveErr == nil, "state": state}
		if resolveErr != nil {
			result["error"] = resolveErr.Error()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-sync/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-sync/state", func(response http.ResponseWriter, _ *http.Request) {
		state := service.Snapshot()
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(state)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-sync/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(approvalStore.syncSession())
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/reset", func(response http.ResponseWriter, _ *http.Request) {
		approvalStore.reset()
		if err := restartStateChangingBroadcast(service, approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/execute-command", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			CommandID string `json:"commandId"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid fixture command execution", http.StatusBadRequest)
			return
		}
		if _, err := approvalStore.ExecuteCommandState(request.Context(), "terminal-stateful", payload.CommandID); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.RefreshActiveTerminal(approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/resolve", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string `json:"requestId"`
			Decision  string `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid fixture command decision", http.StatusBadRequest)
			return
		}
		requestID := payload.RequestID
		if requestID == fixtureApprovalRequestID {
			if current := service.Snapshot().PendingCommandExecution; current != nil {
				requestID = current.RequestID
			}
		}
		state, _, _, resolveErr := service.ResolveCommandExecution(request.Context(), requestID, domain.CommandExecutionDecision(payload.Decision))
		result := map[string]any{"ok": resolveErr == nil, "state": state}
		if resolveErr != nil {
			result["error"] = resolveErr.Error()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/switch-away", func(response http.ResponseWriter, _ *http.Request) {
		if err := activateLifecycleTerminal(service, stateChangingSyncReserveTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/switch-back", func(response http.ResponseWriter, _ *http.Request) {
		if err := activateLifecycleTerminal(service, approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/restart-broadcast", func(response http.ResponseWriter, _ *http.Request) {
		if err := restartStateChangingBroadcast(service, approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/reopen-session", func(response http.ResponseWriter, _ *http.Request) {
		if err := restartStateChangingBroadcast(service, approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-sync/audit", func(response http.ResponseWriter, _ *http.Request) {
		executeWrites, completed := approvalStore.audit()
		pendingRequests := 0
		if service.Snapshot().PendingCommandExecution != nil {
			pendingRequests = 1
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"executeWrites":   executeWrites,
			"pendingRequests": pendingRequests,
			"completed":       completed,
		})
	})
	mux.HandleFunc("GET /__fixture/facility-diagnostics/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/facility-diagnostics/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/facility-diagnostics/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/facility-diagnostics/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/facility-diagnostics/desktop-bindings.js", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = fmt.Fprint(response, `import * as base from "/__fixture/desktop-bindings.js";
export * from "/__fixture/desktop-bindings.js";

async function coordinationState() {
  const response = await fetch("/__fixture/facility-diagnostics/coordination");
  if (!response.ok) throw new Error("facility diagnostics coordination is unavailable");
  return response.json();
}

export const Events = {
  On(name, callback) {
    if (name !== "coordination-state") return base.Events.On(name, callback);
    let active = true;
    let previous = "";
    const poll = async () => {
      try {
        const state = await coordinationState();
        const encoded = JSON.stringify(state);
        if (!active || encoded === previous) return;
        previous = encoded;
        callback({ data: state });
      } catch {
        // The next deterministic poll retries while the fixture is active.
      }
    };
    void poll();
    const interval = setInterval(() => { void poll(); }, 25);
    return () => { active = false; clearInterval(interval); };
  },
};

export async function GetRuntimeStatus() {
  const status = await base.GetRuntimeStatus();
  return { ...status, coordinationState: await coordinationState() };
}

export async function OpenSession() {
  const response = await fetch("/__fixture/facility-diagnostics/session");
  return {
    ok: response.ok,
    error: response.ok ? "" : "facility diagnostics session is unavailable",
    filePath: "/private/tmp/fallout-facility-diagnostics.json",
    session: response.ok ? await response.json() : null,
  };
}

export async function ResolveCommandExecution(payload) {
  const response = await fetch("/__fixture/facility-diagnostics/resolve", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload ?? {}),
  });
  return response.json();
}
`)
	})
	mux.HandleFunc("POST /__fixture/facility-diagnostics/reset", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Scenario string `json:"scenario"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility diagnostic reset", http.StatusBadRequest)
			return
		}
		if err := facilityDiagnosticState.reset(payload.Scenario); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		if err := facilityPlayerState.reset("ready"); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		approvalStore.reset()
		fixtureHackRandom.reset()
		presentationGate.reset()
		navigationCatalog.replace(func() domain.Session {
			session, _ := facilityDiagnosticState.sessionSnapshot()
			return session
		}())
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		target, ok := facilityDiagnosticState.target()
		if !ok {
			http.Error(response, "facility diagnostic terminal is unavailable", http.StatusInternalServerError)
			return
		}
		if _, err := service.RequestTerminalActivation(target); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/facility-diagnostics/session", func(response http.ResponseWriter, _ *http.Request) {
		session, ok := facilityDiagnosticState.sessionSnapshot()
		if !ok {
			http.Error(response, "facility diagnostic fixture is inactive", http.StatusConflict)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(session)
	})
	mux.HandleFunc("GET /__fixture/facility-diagnostics/coordination", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(service.Snapshot())
	})
	mux.HandleFunc("GET /__fixture/facility-diagnostics/state", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityDiagnosticState.snapshot())
	})
	mux.HandleFunc("POST /__fixture/facility-diagnostics/replay-projection", func(response http.ResponseWriter, _ *http.Request) {
		target, ok := facilityDiagnosticState.target()
		if !ok {
			http.Error(response, "facility diagnostic fixture is inactive", http.StatusConflict)
			return
		}
		facilityDiagnosticState.replay()
		if _, err := service.RefreshActiveTerminal(target); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/facility-diagnostics/resolve", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string                          `json:"requestId"`
			Decision  domain.CommandExecutionDecision `json:"decision"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil ||
			(payload.Decision != domain.CommandExecutionApprove && payload.Decision != domain.CommandExecutionReject) {
			http.Error(response, "invalid facility diagnostic decision", http.StatusBadRequest)
			return
		}
		pending := service.Snapshot().PendingCommandExecution
		commandID := ""
		if pending != nil && pending.RequestID == payload.RequestID {
			commandID = pending.CommandID
		}
		state, _, _, resolveErr := service.ResolveCommandExecution(request.Context(), payload.RequestID, payload.Decision)
		var facilityResult *domain.FacilityOperationResult
		if resolveErr == nil && payload.Decision == domain.CommandExecutionApprove {
			conditionID := map[string]string{
				"restore-primary-power": fixtureFacilityUnpoweredConditionID,
				"run-network-recovery":  fixtureFacilityNetworkConditionID,
			}[commandID]
			if conditionID != "" {
				result, recoverErr := facilityDiagnosticState.recover(conditionID, false)
				if recoverErr != nil {
					resolveErr = recoverErr
				} else {
					result.CorrelationID = payload.RequestID
					facilityResult = &result
					target, _ := facilityDiagnosticState.target()
					state, resolveErr = service.RefreshActiveTerminal(target)
				}
			}
		}
		result := map[string]any{"ok": resolveErr == nil, "state": state, "facilityResult": facilityResult}
		if resolveErr != nil {
			result["error"] = "facility diagnostic decision failed"
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/facility-diagnostics/recover-private", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			ConditionID              string `json:"conditionId"`
			ExpectedFacilityRevision uint64 `json:"expectedFacilityRevision"`
			CorrelationID            string `json:"correlationId"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil || strings.TrimSpace(payload.CorrelationID) == "" {
			http.Error(response, "invalid private facility recovery", http.StatusBadRequest)
			return
		}
		before := facilityDiagnosticState.snapshot().Facility.Revision
		if payload.ExpectedFacilityRevision != before {
			http.Error(response, "stale facility revision", http.StatusConflict)
			return
		}
		result, recoverErr := facilityDiagnosticState.recover(payload.ConditionID, true)
		if recoverErr != nil {
			http.Error(response, recoverErr.Error(), http.StatusConflict)
			return
		}
		result.CorrelationID = payload.CorrelationID
		target, _ := facilityDiagnosticState.target()
		if _, err := service.RefreshActiveTerminal(target); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("GET /__fixture/facility-authoring/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/facility-authoring/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/facility-authoring/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/facility-authoring/desktop-bindings.js", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = fmt.Fprint(response, `import * as base from "/__fixture/desktop-bindings.js";
export * from "/__fixture/desktop-bindings.js";

async function fixtureStatus() {
  const response = await fetch("/__fixture/facility-authoring/status");
  if (!response.ok) throw new Error("facility authoring fixture is unavailable");
  return response.json();
}

export async function GetRuntimeStatus() {
  const [status, fixture] = await Promise.all([base.GetRuntimeStatus(), fixtureStatus()]);
  return { ...status, savedRevision: fixture.sessionRevision };
}

export async function OpenSession() {
  const response = await fetch("/__fixture/facility-authoring/session");
  return {
    ok: response.ok,
    error: response.ok ? "" : "facility authoring session is unavailable",
    filePath: "/private/tmp/fallout-facility-authoring.json",
    session: response.ok ? await response.json() : null,
  };
}

async function post(path, payload) {
  const response = await fetch("/__fixture/facility-authoring/" + path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(structuredClone(payload ?? {})),
  });
  if (!response.ok) throw new Error("facility authoring " + path + " failed");
  return response.json();
}

export const SaveFacilityAuthoring = payload => post("save", payload);
export const InspectFacilityDependencies = payload => post("inspect", payload);
export const PreviewFacility = payload => post("preview", payload);
export const ResetFacilityDevice = payload => post("reset-device", payload);
export const ResetFacility = payload => post("reset-facility", payload);
export const RecoverFacilityCondition = payload => post("recover", payload);
`)
	})
	mux.HandleFunc("POST /__fixture/facility-authoring/reset", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Scenario string `json:"scenario"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility authoring reset", http.StatusBadRequest)
			return
		}
		if err := facilityAuthoringState.reset(payload.Scenario); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/facility-authoring/status", func(response http.ResponseWriter, _ *http.Request) {
		_, revision := facilityAuthoringState.sessionSnapshot()
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]uint64{"sessionRevision": revision})
	})
	mux.HandleFunc("GET /__fixture/facility-authoring/session", func(response http.ResponseWriter, _ *http.Request) {
		session, _ := facilityAuthoringState.sessionSnapshot()
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(session)
	})
	mux.HandleFunc("GET /__fixture/facility-authoring/state", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityAuthoringState.snapshot())
	})
	mux.HandleFunc("POST /__fixture/facility-authoring/inspect", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureFacilityInspectionRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility dependency inspection", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityAuthoringState.inspect(payload))
	})
	mux.HandleFunc("POST /__fixture/facility-authoring/save", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureFacilityAuthoringRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility authoring save", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityAuthoringState.save(payload))
	})
	mux.HandleFunc("POST /__fixture/facility-authoring/next-operation", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Failure string `json:"failure"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid next facility operation", http.StatusBadRequest)
			return
		}
		facilityAuthoringState.nextOperationFailure(payload.Failure)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/facility-authoring/preview", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureFacilityPreviewRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility preview", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityAuthoringState.preview(payload))
	})
	mux.HandleFunc("POST /__fixture/facility-authoring/reset-device", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureFacilityDeviceResetRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility device reset", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityAuthoringState.resetDevice(payload))
	})
	mux.HandleFunc("POST /__fixture/facility-authoring/reset-facility", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureFacilityResetRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility reset", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityAuthoringState.resetFacility(payload))
	})
	mux.HandleFunc("POST /__fixture/facility-authoring/recover", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureFacilityRecoveryRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility recovery", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityAuthoringState.recover(payload))
	})

	mux.HandleFunc("GET /__fixture/facility-player-state/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/facility-player-state/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/facility-player-state/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/facility-diagnostics/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/facility-player-state/desktop-bindings.js", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = fmt.Fprint(response, `import * as base from "/__fixture/desktop-bindings.js";
export * from "/__fixture/desktop-bindings.js";

async function coordinationState() {
  const response = await fetch("/__fixture/facility-player-state/coordination");
  if (!response.ok) throw new Error("facility coordination fixture is unavailable");
  return response.json();
}

export const Events = {
  On(name, callback) {
    if (name !== "coordination-state") return base.Events.On(name, callback);
    let active = true;
    let previous = "";
    const poll = async () => {
      try {
        const state = await coordinationState();
        const encoded = JSON.stringify(state);
        if (!active || encoded === previous) return;
        previous = encoded;
        callback({ data: state });
      } catch {
        // The next deterministic poll retries while the fixture is active.
      }
    };
    void poll();
    const interval = setInterval(() => { void poll(); }, 25);
    return () => {
      active = false;
      clearInterval(interval);
    };
  },
};

export async function GetRuntimeStatus() {
  const status = await base.GetRuntimeStatus();
  return { ...status, coordinationState: await coordinationState() };
}

export async function OpenSession() {
  const response = await fetch("/__fixture/facility-player-state/session");
  return {
    ok: response.ok,
    error: response.ok ? "" : "facility session fixture is unavailable",
    filePath: "/private/tmp/fallout-facility-player-state.json",
    session: response.ok ? await response.json() : null,
  };
}

export async function ResolveCommandExecution(payload) {
  const response = await fetch("/__fixture/facility-player-state/resolve", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload ?? {}),
  });
  return response.json();
}
`)
	})
	mux.HandleFunc("POST /__fixture/facility-player-state/reset", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Scenario string `json:"scenario"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility player-state reset", http.StatusBadRequest)
			return
		}
		if err := facilityPlayerState.reset(payload.Scenario); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		if facilityPlayerState.projectionFacility() == nil {
			if _, err := service.ReplaceFacility(facilityPlayerSession(nil).Facility); err != nil {
				http.Error(response, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		facilityDiagnosticState.deactivate()
		approvalStore.reset()
		fixtureHackRandom.reset()
		presentationGate.reset()
		if err := edge.reset(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		target := approvalStore.facilityTarget()
		if sharedTarget, ok := facilityPlayerState.target(fixtureFacilityTerminalID); ok {
			target = sharedTarget
		}
		if _, err := service.RequestTerminalActivation(target); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/facility-player-state/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if session, ok := facilityPlayerState.session(); ok {
			_ = json.NewEncoder(response).Encode(session)
			return
		}
		_ = json.NewEncoder(response).Encode(approvalStore.facilitySession())
	})
	mux.HandleFunc("GET /__fixture/facility-player-state/coordination", func(response http.ResponseWriter, _ *http.Request) {
		state, stateErr := facilityPlayerCoordinationState(service)
		if stateErr != nil {
			http.Error(response, stateErr.Error(), http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(state)
	})
	mux.HandleFunc("GET /__fixture/facility-player-state/state", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityPlayerState.snapshot())
	})
	mux.HandleFunc("POST /__fixture/facility-player-state/resolve", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string                          `json:"requestId"`
			Decision  domain.CommandExecutionDecision `json:"decision"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil ||
			(payload.Decision != domain.CommandExecutionApprove && payload.Decision != domain.CommandExecutionReject) {
			http.Error(response, "invalid facility command decision", http.StatusBadRequest)
			return
		}
		coordination, facilityResult, ok, resolveErr := resolveFacilityPlayerCommand(
			request.Context(), service, facilityPlayerState, payload.RequestID, payload.Decision,
		)
		result := map[string]any{
			"ok": ok, "state": coordination, "facilityResult": facilityResult,
		}
		if resolveErr != nil {
			result["error"] = "facility command decision failed"
		} else if !ok {
			result["error"] = "facility command could not change shared state"
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/facility-player-state/repeat-current-decision", func(response http.ResponseWriter, request *http.Request) {
		if pending := service.Snapshot().PendingCommandExecution; pending != nil {
			facilityPlayerState.scenarioForAttempt(pending.RequestID)
			result := map[string]any{
				"ok": false, "state": service.Snapshot(),
				"facilityResult": facilityPlayerState.recordDuplicate(pending.RequestID),
				"error":          "facility command decision failed",
			}
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(result)
			return
		}
		coordination, facilityResult, ok, resolveErr := resolveFacilityPlayerCommand(
			request.Context(), service, facilityPlayerState, "", domain.CommandExecutionApprove,
		)
		result := map[string]any{
			"ok": ok, "state": coordination, "facilityResult": facilityResult,
		}
		if resolveErr != nil {
			result["error"] = "facility command decision failed"
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/facility-player-state/activate-terminal", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			TerminalID string `json:"terminalId"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid shared facility terminal activation", http.StatusBadRequest)
			return
		}
		target, ok := facilityPlayerState.target(payload.TerminalID)
		if !ok {
			http.Error(response, "unknown shared facility terminal", http.StatusBadRequest)
			return
		}
		facilityPlayerState.resetNavigation(payload.TerminalID)
		if _, err := service.RequestTerminalActivation(target); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/facility-player-state/apply-projection-transition", func(response http.ResponseWriter, _ *http.Request) {
		if err := facilityPlayerState.applyProjectionTransition(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		coordination := service.Snapshot()
		if coordination.Broadcast == nil || coordination.Broadcast.ActiveTerminalID == nil {
			http.Error(response, "shared facility terminal is not active", http.StatusConflict)
			return
		}
		target, ok := facilityPlayerState.target(*coordination.Broadcast.ActiveTerminalID)
		if !ok {
			http.Error(response, "active shared facility terminal is unknown", http.StatusConflict)
			return
		}
		if _, err := service.RefreshActiveTerminal(target); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/facility-player-state/move-terminal", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			TerminalID string `json:"terminalId"`
			GroupID    string `json:"groupId"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid shared facility group move", http.StatusBadRequest)
			return
		}
		if err := facilityPlayerState.moveTerminal(payload.TerminalID, payload.GroupID); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		coordination := service.Snapshot()
		if coordination.Broadcast != nil && coordination.Broadcast.ActiveTerminalID != nil {
			target, ok := facilityPlayerState.target(*coordination.Broadcast.ActiveTerminalID)
			if !ok {
				http.Error(response, "active shared facility terminal is unknown", http.StatusConflict)
				return
			}
			if _, err := service.RefreshActiveTerminal(target); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		}
		response.WriteHeader(http.StatusNoContent)
	})

	installFacilityLifecycle := func(action string, restartBroadcast bool) error {
		loaded := facilityLifecycleState.loadedSession()
		navigationCatalog.replace(loaded)
		pendingInvalidated := service.Snapshot().PendingCommandExecution != nil
		if restartBroadcast {
			if _, err := service.EndBroadcast(); err != nil {
				return err
			}
		}
		if _, err := service.ReplaceFacility(loaded.Facility); err != nil {
			return err
		}
		facilityLifecycleState.recordHydration(action, pendingInvalidated)
		if !restartBroadcast {
			return nil
		}
		if _, err := service.StartBroadcast(); err != nil {
			return err
		}
		return activateLifecycleTerminal(service, fixtureTarget(loaded.Terminals[0]))
	}
	mux.HandleFunc("GET /__fixture/facility-lifecycle/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/facility-lifecycle/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/facility-lifecycle/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/facility-lifecycle/desktop-bindings.js", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = fmt.Fprint(response, `import * as base from "/__fixture/desktop-bindings.js";
export * from "/__fixture/desktop-bindings.js";

async function coordinationState() {
  const response = await fetch("/__fixture/facility-lifecycle/coordination");
  if (!response.ok) throw new Error("facility lifecycle coordination fixture is unavailable");
  return response.json();
}

export const Events = {
  On(name, callback) {
    if (name !== "coordination-state") return base.Events.On(name, callback);
    let active = true;
    let previous = "";
    const poll = async () => {
      try {
        const state = await coordinationState();
        const encoded = JSON.stringify(state);
        if (!active || encoded === previous) return;
        previous = encoded;
        callback({ data: state });
      } catch {
        // The next deterministic poll retries while the fixture is active.
      }
    };
    void poll();
    const interval = setInterval(() => { void poll(); }, 25);
    return () => {
      active = false;
      clearInterval(interval);
    };
  },
};

export async function GetRuntimeStatus() {
  const status = await base.GetRuntimeStatus();
  return { ...status, coordinationState: await coordinationState() };
}

export async function OpenSession() {
  const response = await fetch("/__fixture/facility-lifecycle/session");
  return {
    ok: response.ok,
    error: response.ok ? "" : "facility lifecycle session fixture is unavailable",
    filePath: "/private/tmp/fallout-facility-lifecycle.json",
    session: response.ok ? await response.json() : null,
  };
}
`)
	})
	mux.HandleFunc("POST /__fixture/facility-lifecycle/reset", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Scenario string `json:"scenario"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid facility lifecycle reset", http.StatusBadRequest)
			return
		}
		if err := facilityLifecycleState.reset(payload.Scenario); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		if err := facilityPlayerState.reset("ready"); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		facilityDiagnosticState.deactivate()
		approvalStore.reset()
		fixtureHackRandom.reset()
		presentationGate.reset()
		if err := edge.reset(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := installFacilityLifecycle("", true); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/facility-lifecycle/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityLifecycleState.loadedSession())
	})
	mux.HandleFunc("GET /__fixture/facility-lifecycle/coordination", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(service.Snapshot())
	})
	mux.HandleFunc("GET /__fixture/facility-lifecycle/state", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(facilityLifecycleState.snapshot(service))
	})
	mux.HandleFunc("POST /__fixture/facility-lifecycle/stop-start-broadcast", func(response http.ResponseWriter, _ *http.Request) {
		if err := installFacilityLifecycle("", true); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/facility-lifecycle/reload-session", func(response http.ResponseWriter, _ *http.Request) {
		if err := installFacilityLifecycle("", false); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	for _, action := range []string{"restart-process", "self-update-handoff"} {
		mux.HandleFunc("POST /__fixture/facility-lifecycle/"+action, func(response http.ResponseWriter, _ *http.Request) {
			if err := installFacilityLifecycle(action, true); err != nil {
				http.Error(response, err.Error(), http.StatusInternalServerError)
				return
			}
			response.WriteHeader(http.StatusNoContent)
		})
	}
	mux.HandleFunc("GET /__fixture/overseer.css", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "../../frontend/overseer/src/overseer.css")
	})
	mux.HandleFunc("GET /__fixture/overseer.js", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "../../frontend/overseer/src/overseer.js")
	})
	mux.HandleFunc("GET /__fixture/desktop-api.js", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "../../frontend/overseer/src/desktop-api.js")
	})
	mux.HandleFunc("GET /__fixture/desktop-bindings.js", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "fixtures/desktop-bindings.js")
	})
	mux.HandleFunc("POST /__fixture/reset", func(response http.ResponseWriter, _ *http.Request) {
		facilityDiagnosticState.deactivate()
		fixtureHackRandom.reset()
		presentationGate.reset()
		if err := edge.reset(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := service.RequestTerminalActivation(fixtureTerminal()); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/reassign-controller", func(response http.ResponseWriter, _ *http.Request) {
		state := service.Snapshot()
		var target domain.LogicalSessionID
		for _, session := range state.Sessions {
			if session.Connected && session.Character != nil && session.Role == domain.PlayerRoleObserver {
				target = session.ID
				break
			}
		}
		if target == "" {
			http.Error(response, "connected assigned observer does not exist", http.StatusConflict)
			return
		}
		updated, err := service.SetActiveController(target)
		if err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"controllerSessionId": target,
			"revision":            updated.Revision,
		})
	})
	mux.HandleFunc("POST /__fixture/update", func(response http.ResponseWriter, _ *http.Request) {
		updated := fixtureTerminal()
		updated.Tree.Children = append(updated.Tree.Children, domain.ContentNode{
			ID: "public-update", Type: domain.NodeEntry, Name: "PUBLIC UPDATE", Description: "STREAM UPDATE RECEIVED",
		})
		if _, err := service.UpdateLiveTerminal(updated.Tree, &updated.IntroText); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/force-hack", func(response http.ResponseWriter, _ *http.Request) {
		if _, ok := service.ForceHackSuccess(); !ok {
			http.Error(response, "active terminal has no unfinished hack", http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/local/hacking", func(response http.ResponseWriter, _ *http.Request) {
		edge.activateHacking(response)
	})
	mux.HandleFunc("POST /__fixture/local/crt/approve-command", func(response http.ResponseWriter, request *http.Request) {
		pending := service.Snapshot().PendingCommandExecution
		if pending == nil {
			http.Error(response, "CRT command approval is not pending", http.StatusConflict)
			return
		}
		if _, _, _, err := service.ResolveCommandExecution(request.Context(), pending.RequestID, domain.CommandExecutionApprove); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/local/crt/{state}", func(response http.ResponseWriter, request *http.Request) {
		state := request.PathValue("state")
		switch state {
		case "content":
			if _, err := service.RequestTerminalActivation(crtFixtureTerminal()); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "unchanged":
			target := crtFixtureTerminal()
			if _, err := service.UpdateLiveTerminal(target.Tree, &target.IntroText); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "replacement":
			target := crtReplacementTerminal()
			if _, err := service.UpdateLiveTerminal(target.Tree, &target.IntroText); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "display-unstable":
			target := crtFixtureTerminal()
			target.TerminalID = "terminal-crt-display-unstable"
			target.Effects = []domain.TerminalPresentationEffect{domain.TerminalPresentationEffectDisplayUnstable}
			if !activateCRTTerminal(service, response, target) {
				return
			}
		case "display-stable":
			target := crtFixtureTerminal()
			target.TerminalID = "terminal-crt-display-stable"
			if !activateCRTTerminal(service, response, target) {
				return
			}
		case "waiting":
			if _, err := service.RequestTerminalClear(); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "hacking", "blocked":
			if !activateCRTTerminal(service, response, crtHackingTerminal("a")) {
				return
			}
		case "hacking-unchanged":
			target := crtHackingTerminal("a")
			if _, err := service.UpdateLiveTerminal(target.Tree, &target.IntroText); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "hacking-replacement":
			if !activateCRTTerminal(service, response, crtHackingTerminal("b")) {
				return
			}
		default:
			http.NotFound(response, request)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/local/crt/hacking-dud/{position}", func(response http.ResponseWriter, request *http.Request) {
		if !fixtureHackRandom.forceDudRemoval(request.PathValue("position")) {
			http.NotFound(response, request)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/local/disconnect", func(response http.ResponseWriter, _ *http.Request) {
		connectPlayer.CloseSubscriptions()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/presentation-uplinks/close", func(response http.ResponseWriter, _ *http.Request) {
		presentationHub.CloseUplinks(errors.New("fixture presentation uplinks closed"))
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/public-access/failure/", func(response http.ResponseWriter, request *http.Request) {
		kind := strings.TrimPrefix(request.URL.Path, "/__fixture/public-access/failure/")
		snapshot, ok := edge.publicFailure(kind)
		if !ok || strings.Contains(kind, "/") {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(snapshot)
	})
	mux.HandleFunc("POST /__fixture/public-access/recover", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(edge.publicRecovery())
	})
	mux.HandleFunc("GET /__fixture/edge/status", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(fixtureEdgeStatus{
			AuthBoundary: "application-ingress", Upstream: "http://" + fixtureAddress,
			Active: edge.active.Load(), AuthorizationForwarded: request.Header.Get("Authorization") != "",
			PublicURL: edge.publicURL,
		})
	})
	mux.HandleFunc("POST /__fixture/edge/update", func(response http.ResponseWriter, _ *http.Request) {
		edge.update(response)
	})
	mux.HandleFunc("POST /__fixture/edge/hacking", func(response http.ResponseWriter, _ *http.Request) {
		edge.activateHacking(response)
	})
	mux.HandleFunc("POST /__fixture/edge/disconnect", func(response http.ResponseWriter, _ *http.Request) {
		edge.connect.CloseSubscriptions()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/presentation-gate/arm", func(response http.ResponseWriter, _ *http.Request) {
		presentationGate.arm()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/presentation-gate/arm-unary", func(response http.ResponseWriter, _ *http.Request) {
		presentationGate.armUnary()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/edge/presentation-gate/blocked", func(response http.ResponseWriter, _ *http.Request) {
		if !presentationGate.isBlocked() {
			response.WriteHeader(http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/presentation-gate/release", func(response http.ResponseWriter, _ *http.Request) {
		presentationGate.release()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/presentation-gate/cancel-uplinks", func(response http.ResponseWriter, _ *http.Request) {
		edge.hub.CloseUplinks(errors.New("fixture presentation uplinks rotated"))
		presentationGate.cancelAfter(2 * time.Second)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/disable", func(response http.ResponseWriter, _ *http.Request) {
		edge.ingress.Deny()
		edge.active.Store(false)
		edge.connect.CloseSubscriptions()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/__fixture/protected/", func(response http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != fixtureEdgeUsername || password != fixtureEdgePassword {
			response.Header().Set("WWW-Authenticate", `Basic realm="Fallout Terminal Players"`)
			http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		forwarded := request.Clone(request.Context())
		forwarded.URL.Path = "/" + strings.TrimPrefix(request.URL.Path, "/__fixture/protected/")
		applicationHandler.ServeHTTP(response, forwarded)
	})
	mux.Handle("/", applicationHandler)

	listener, err := net.Listen("tcp4", fixtureAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", fixtureAddress, err)
	}
	ingress, err := tunnel.NewPublicIngressFactory().Start(ctx, "http://"+fixtureAddress)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("start fixture public ingress: %w", err)
	}
	edge.ingress = ingress
	publicTLS := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !edge.active.Load() {
			http.NotFound(response, request)
			return
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != fixtureEdgeUsername || password != fixtureEdgePassword {
			response.Header().Set("WWW-Authenticate", `Basic realm="Fallout Terminal Players"`)
			http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		forwarded := request.Clone(request.Context())
		forwarded.Header.Del("Authorization")
		forwarded.Header.Del("Proxy-Authorization")
		mux.ServeHTTP(response, forwarded)
	}))
	publicTLS.EnableHTTP2 = true
	publicTLS.StartTLS()
	edge.publicURL = publicTLS.URL
	if err := edge.reset(); err != nil {
		publicTLS.Close()
		_ = ingress.Close(ctx)
		_ = listener.Close()
		return fmt.Errorf("activate fixture public ingress: %w", err)
	}
	fixtureProtocols := new(http.Protocols)
	fixtureProtocols.SetHTTP1(true)
	fixtureProtocols.SetUnencryptedHTTP2(true)
	httpServer := &http.Server{Handler: mux, Protocols: fixtureProtocols}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}

	shutdownDeadline, stopShutdownDeadline := context.WithTimeoutCause(context.WithoutCancel(ctx), 3*time.Second, errors.New("fixture shutdown timed out"))
	shutdownContext, cancelShutdown := context.WithCancelCause(shutdownDeadline)
	defer func() {
		cancelShutdown(errors.New("fixture shutdown complete"))
		stopShutdownDeadline()
	}()
	ingress.Deny()
	publicTLS.Close()
	return errors.Join(ingress.Close(shutdownContext), httpServer.Shutdown(shutdownContext))
}

func fixtureTerminal() domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: "terminal-1", TerminalName: "Overseer", HackLevel: 0, IntroText: "WELCOME",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{
				{ID: "docs", Type: domain.NodeFolder, Name: "DOCS", Children: []domain.ContentNode{
					{ID: "report", Type: domain.NodeEntry, Name: "REPORT", Description: "SYSTEM NOMINAL"},
				}},
				{ID: "status", Type: domain.NodeEntry, Name: "STATUS", Description: "ALL SYSTEMS OPERATIONAL"},
			},
		},
	}
}

func facilityPlayerTarget(states map[string]domain.CommandExecutionState) domain.TerminalTarget {
	locked := domain.FacilityStateEquality{DeviceID: fixtureFacilityDoorID, StateID: "locked"}
	return domain.TerminalTarget{
		TerminalID:   fixtureFacilityTerminalID,
		TerminalName: "Security control",
		HackLevel:    0,
		IntroText:    "SECURITY CONTROL // FACILITY NETWORK",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: fixtureFacilityCommandID, Type: domain.NodeCommand,
				Name: "OPEN SECURITY DOOR", Text: "Security door and alarm updated.",
				FacilityNameVariants: []domain.FacilityTextVariant{{
					When: domain.FacilityStateEquality{DeviceID: fixtureFacilityDoorID, StateID: "open"},
					Text: "SECURITY DOOR OPEN",
				}},
				AvailableWhen: &locked,
				StateChange: &domain.StateChangeConfig{
					CompletedName:    "SECURITY DOOR OPEN",
					ConfirmationText: "Authorize the security-sector world action?",
					FacilityAction: &domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
						Transitions: []domain.FacilityTransitionRequest{
							{DeviceID: fixtureFacilityDoorID, TransitionID: "open"},
							{DeviceID: fixtureFacilityAlarmID, TransitionID: "silence"},
						},
					}},
				},
			}},
		},
		CommandStates: cloneFixtureCommandStates(states),
	}
}

func facilityPlayerSession(states map[string]domain.CommandExecutionState) domain.Session {
	target := facilityPlayerTarget(states)
	return domain.Session{
		Version: 1,
		Name:    "Facility player-state fixture",
		Terminals: []domain.Terminal{{
			ID: target.TerminalID, Name: target.TerminalName, HackLevel: target.HackLevel,
			IntroText: target.IntroText, Root: target.Tree, CommandStates: target.CommandStates,
		}},
		Facility: &domain.Facility{
			Devices: []domain.FacilityDevice{
				{
					ID: fixtureFacilityDoorID, Name: "Security sector door", Kind: domain.FacilityDeviceKindDoor,
					InitialStateID: "locked", CurrentStateID: "locked",
					States: []domain.FacilityDeviceState{{ID: "locked", Name: "Locked"}, {ID: "open", Name: "Open"}},
					Transitions: []domain.FacilityDeviceTransition{{
						ID: "open", Name: "Open", SourceStateID: "locked", DestinationStateID: "open",
						ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: fixtureFacilityCondition, Active: false}},
						Recovery:         true,
					}},
				},
				{
					ID: fixtureFacilityAlarmID, Name: "Security alarm", Kind: domain.FacilityDeviceKindAlarm,
					InitialStateID: "armed", CurrentStateID: "armed",
					States: []domain.FacilityDeviceState{{ID: "armed", Name: "Armed"}, {ID: "silent", Name: "Silent"}},
					Transitions: []domain.FacilityDeviceTransition{{
						ID: "silence", Name: "Silence", SourceStateID: "armed", DestinationStateID: "silent",
					}},
				},
			},
			Conditions: []domain.DiagnosticCondition{{
				ID: fixtureFacilityCondition, Name: "Security authorization corrupted",
				Category:      domain.DiagnosticConditionCategoryAuthorizationCorrupted,
				Device:        &domain.DiagnosticDeviceScope{DeviceID: fixtureFacilityDoorID},
				InitialActive: true, CurrentActive: true,
				Effects: []domain.DiagnosticEffect{{
					CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityHack},
				}},
				Recovery: []domain.DiagnosticRecoveryReference{{
					Transition: &domain.FacilityTransitionRequest{DeviceID: fixtureFacilityDoorID, TransitionID: "open"},
				}},
			}},
			RecoveryPrograms: []domain.RecoveryProgram{},
		},
	}
}

func facilityProjectionSession() domain.Session {
	equality := func(deviceID, stateID string) domain.FacilityStateEquality {
		return domain.FacilityStateEquality{DeviceID: deviceID, StateID: stateID}
	}
	variant := func(deviceID, stateID, text string) domain.FacilityTextVariant {
		return domain.FacilityTextVariant{When: equality(deviceID, stateID), Text: text}
	}
	entry := func(id, name string, blocks ...domain.EntryContentBlock) domain.ContentNode {
		return domain.ContentNode{ID: id, Type: domain.NodeEntry, Name: name, Blocks: blocks}
	}
	terminal := func(id, name string, children ...domain.ContentNode) domain.Terminal {
		return domain.Terminal{
			ID: id, Name: name, IntroText: strings.ToUpper(name) + " // FACILITY NETWORK",
			Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: children},
		}
	}

	security := terminal(fixtureFacilityTerminalID, "Security control",
		func() domain.ContentNode {
			node := entry("entry-security-status", "FACILITY STATUS",
				domain.EntryContentBlock{
					ID: "block-security-power", InitialText: "PRIMARY POWER: OFFLINE",
					FacilityTextVariants: []domain.FacilityTextVariant{variant(fixtureFacilityPowerID, "online", "PRIMARY POWER: ONLINE")},
				},
				domain.EntryContentBlock{
					ID: "block-security-door", InitialText: "SECURITY DOOR: LOCKED",
					FacilityTextVariants: []domain.FacilityTextVariant{variant(fixtureFacilityDoorID, "open", "SECURITY DOOR: OPEN")},
				},
			)
			node.FacilityNameVariants = []domain.FacilityTextVariant{variant(fixtureFacilityDoorID, "open", "FACILITY STATUS // ACCESS OPEN")}
			return node
		}(),
		domain.ContentNode{
			ID: fixtureFacilityCommandID, Type: domain.NodeCommand, Name: "OPEN SECURITY DOOR",
			Text:          "Security door and alarm updated.",
			AvailableWhen: new(equality(fixtureFacilityPowerID, "online")),
			StateChange: &domain.StateChangeConfig{
				CompletedName: "SECURITY DOOR OPEN", ConfirmationText: "Authorize the security-sector world action?",
				EntryContentChange: &domain.EntryContentChange{
					BlockID: "block-security-door", CompletedText: "SECURITY DOOR: LEGACY COMMAND COMPLETE",
				},
				FacilityAction: &domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
					Transitions: []domain.FacilityTransitionRequest{
						{DeviceID: fixtureFacilityDoorID, TransitionID: "open"},
						{DeviceID: fixtureFacilityAlarmID, TransitionID: "silence"},
					},
				}},
			},
		},
		domain.ContentNode{
			ID: "folder-restricted-archive", Type: domain.NodeFolder, Name: "RESTRICTED ARCHIVE",
			VisibleWhen: new(equality(fixtureFacilityDoorID, "open")),
			Children: []domain.ContentNode{{
				ID: "entry-security-clearance", Type: domain.NodeEntry, Name: "CLEARANCE ACCEPTED",
				Description: "Protected records are now available.",
			}},
		},
	)
	reactor := terminal(fixtureFacilityReactorTerminalID, "Reactor control",
		entry("entry-reactor-status", "REACTOR STATUS",
			domain.EntryContentBlock{
				ID: "block-reactor-power", InitialText: "CONTROL POWER: OFFLINE",
				FacilityTextVariants: []domain.FacilityTextVariant{variant(fixtureFacilityPowerID, "online", "CONTROL POWER: ONLINE")},
			},
			domain.EntryContentBlock{
				ID: "block-reactor-core", InitialText: "REACTOR CORE: OFFLINE",
				FacilityTextVariants: []domain.FacilityTextVariant{variant(fixtureFacilityReactorID, "online", "REACTOR CORE: ONLINE")},
			},
		),
	)
	maintenance := terminal(fixtureFacilityMaintenanceTerminalID, "Maintenance station",
		domain.ContentNode{
			ID: "folder-maintenance-diagnostics", Type: domain.NodeFolder, Name: "DIAGNOSTIC TOOLS",
			Children: []domain.ContentNode{{
				ID: "command-network-recovery", Type: domain.NodeCommand, Name: "RUN NETWORK RECOVERY HOLOTAPE",
				Text: "Network recovery program completed.",
			}},
		},
	)
	network := terminal(fixtureFacilityNetworkTerminalID, "Network operations",
		entry("entry-network-report", "NETWORK STATUS", domain.EntryContentBlock{
			ID: "block-network-status", InitialText: "NETWORK: ISOLATED",
			FacilityTextVariants: []domain.FacilityTextVariant{variant(fixtureFacilityNetworkID, "connected", "NETWORK: CONNECTED")},
		}),
	)
	archive := terminal(fixtureFacilityArchiveTerminalID, "Records archive",
		entry("entry-archive-record", "RECORD 04-B", domain.EntryContentBlock{
			ID: "block-archive-record", InitialText: "RECORD 04-B // SECTOR 7 CORRIDOR PRESSURE NOMINAL",
		}),
	)

	condition := func(id, name string, category domain.DiagnosticConditionCategory, active bool) domain.DiagnosticCondition {
		return domain.DiagnosticCondition{
			ID: id, Name: name, Category: category, Terminal: &domain.DiagnosticTerminalScope{TerminalID: fixtureFacilityTerminalID},
			InitialActive: active, CurrentActive: active,
		}
	}
	conditions := []domain.DiagnosticCondition{
		condition("condition-security-offline", "Security terminal offline", domain.DiagnosticConditionCategoryOffline, false),
		condition("condition-reactor-unpowered", "Reactor controls unpowered", domain.DiagnosticConditionCategoryUnpowered, false),
		condition("condition-network-isolated", "Operations network isolated", domain.DiagnosticConditionCategoryNetworkIsolated, false),
		condition("condition-archive-damaged", "Archive storage damaged", domain.DiagnosticConditionCategoryStorageDamaged, false),
		condition(fixtureFacilityCondition, "Security authorization corrupted", domain.DiagnosticConditionCategoryAuthorizationCorrupted, true),
		condition("condition-reactor-display", "Reactor display unstable", domain.DiagnosticConditionCategoryDisplayUnstable, false),
		condition("condition-cooling-contamination", "Cooling loop contamination", domain.DiagnosticConditionCategoryCustom, false),
	}
	conditions[4].Device = &domain.DiagnosticDeviceScope{DeviceID: fixtureFacilityDoorID}
	conditions[4].Terminal = nil

	device := func(id, name string, kind domain.FacilityDeviceKind, initial, current string, states ...domain.FacilityDeviceState) domain.FacilityDevice {
		return domain.FacilityDevice{
			ID: id, Name: name, Kind: kind, InitialStateID: initial, CurrentStateID: current, States: states,
		}
	}
	devices := []domain.FacilityDevice{
		device(fixtureFacilityPowerID, "Primary power grid", domain.FacilityDeviceKindPowerGrid, "offline", "online",
			domain.FacilityDeviceState{ID: "offline", Name: "Offline"}, domain.FacilityDeviceState{ID: "online", Name: "Online"}),
		device(fixtureFacilityCoolingID, "Reactor cooling loop", domain.FacilityDeviceKindVentilation, "offline", "online",
			domain.FacilityDeviceState{ID: "offline", Name: "Offline"}, domain.FacilityDeviceState{ID: "online", Name: "Online"}),
		device(fixtureFacilityReactorID, "Main reactor", domain.FacilityDeviceKindReactor, "offline", "online",
			domain.FacilityDeviceState{ID: "offline", Name: "Offline"}, domain.FacilityDeviceState{ID: "online", Name: "Online"}),
		device(fixtureFacilityDoorID, "Security sector door", domain.FacilityDeviceKindDoor, "locked", "locked",
			domain.FacilityDeviceState{ID: "locked", Name: "Locked"}, domain.FacilityDeviceState{ID: "open", Name: "Open"}),
		device(fixtureFacilityAlarmID, "Security alarm", domain.FacilityDeviceKindAlarm, "armed", "armed",
			domain.FacilityDeviceState{ID: "armed", Name: "Armed"}, domain.FacilityDeviceState{ID: "silent", Name: "Silent"}),
		device(fixtureFacilityNetworkID, "Operations network", domain.FacilityDeviceKindNetworkSegment, "isolated", "connected",
			domain.FacilityDeviceState{ID: "isolated", Name: "Isolated"}, domain.FacilityDeviceState{ID: "connected", Name: "Connected"}),
	}
	devices[3].Transitions = []domain.FacilityDeviceTransition{{
		ID: "open", Name: "Open security door", SourceStateID: "locked", DestinationStateID: "open",
		Preconditions:    []domain.FacilityStateEquality{equality(fixtureFacilityPowerID, "online")},
		ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: fixtureFacilityCondition, Active: false}},
	}}
	devices[4].Transitions = []domain.FacilityDeviceTransition{{
		ID: "silence", Name: "Silence security alarm", SourceStateID: "armed", DestinationStateID: "silent",
	}}

	return domain.Session{
		Version: 1, Name: "Multi-terminal facility projection fixture",
		Terminals: []domain.Terminal{security, reactor, maintenance, network, archive},
		TerminalGroups: []domain.TerminalGroup{
			{ID: fixtureFacilityOperationsGroupID, Name: "Operations", TerminalIDs: []string{fixtureFacilityTerminalID, fixtureFacilityNetworkTerminalID}},
			{ID: fixtureFacilityEngineeringGroupID, Name: "Engineering", TerminalIDs: []string{fixtureFacilityReactorTerminalID, fixtureFacilityMaintenanceTerminalID}},
			{ID: fixtureFacilityRecordsGroupID, Name: "Records", TerminalIDs: []string{fixtureFacilityArchiveTerminalID}},
		},
		Facility: &domain.Facility{Revision: 7, Devices: devices, Conditions: conditions, RecoveryPrograms: []domain.RecoveryProgram{}},
	}
}

func facilityLifecycleSession() domain.Session {
	open := domain.FacilityStateEquality{DeviceID: fixtureFacilityDoorID, StateID: "open"}
	return domain.Session{
		Version: 1,
		Name:    "Persistent facility lifecycle fixture",
		Terminals: []domain.Terminal{{
			ID: fixtureFacilityTerminalID, Name: "Security control", IntroText: "FACILITY STATE RESTORED",
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{
					{
						ID: "entry-lifecycle-door", Type: domain.NodeEntry, Name: "SECURITY DOOR: LOCKED",
						Description: "The durable facility state controls this record.",
						FacilityNameVariants: []domain.FacilityTextVariant{{
							When: open, Text: "SECURITY DOOR: OPEN",
						}},
					},
					{
						ID: "command-secure-lifecycle-door", Type: domain.NodeCommand, Name: "SECURE SECURITY DOOR",
						Text: "Security door request recorded.",
						StateChange: &domain.StateChangeConfig{
							CompletedName: "SECURITY DOOR SECURED", ConfirmationText: "Secure the security door?",
						},
					},
				},
			},
		}},
		Facility: &domain.Facility{
			Revision: 12,
			Devices: []domain.FacilityDevice{
				{
					ID: fixtureFacilityDoorID, Name: "Security sector door", Kind: domain.FacilityDeviceKindDoor,
					InitialStateID: "locked", CurrentStateID: "open",
					States: []domain.FacilityDeviceState{{ID: "locked", Name: "Locked"}, {ID: "open", Name: "Open"}},
				},
				{
					ID: fixtureFacilityAlarmID, Name: "Security alarm", Kind: domain.FacilityDeviceKindAlarm,
					InitialStateID: "armed", CurrentStateID: "silent",
					States: []domain.FacilityDeviceState{{ID: "armed", Name: "Armed"}, {ID: "silent", Name: "Silent"}},
				},
			},
			Conditions:       []domain.DiagnosticCondition{},
			RecoveryPrograms: []domain.RecoveryProgram{},
		},
	}
}

func facilityLifecycleLegacySession() domain.Session {
	return domain.Session{
		Version: 1,
		Name:    "Legacy lifecycle fixture",
		Terminals: []domain.Terminal{{
			ID: "terminal-legacy-lifecycle", Name: "Legacy terminal", IntroText: "VERSION 1 SESSION RESTORED",
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{{
					ID: "entry-legacy-ready", Type: domain.NodeEntry, Name: "LEGACY TERMINAL READY",
					Description: "No facility state is authored for this session.",
				}},
			},
		}},
	}
}

func facilityAuthoringFixtureSession(empty bool) domain.Session {
	equality := func(deviceID, stateID string) domain.FacilityStateEquality {
		return domain.FacilityStateEquality{DeviceID: deviceID, StateID: stateID}
	}
	securityEntry := domain.ContentNode{
		ID: "entry-security-status", Type: domain.NodeEntry, Name: "FACILITY STATUS",
		Blocks: []domain.EntryContentBlock{{ID: "block-security-power", InitialText: "PRIMARY POWER: OFFLINE"}},
	}
	startReactor := domain.ContentNode{
		ID: "command-start-reactor", Type: domain.NodeCommand, Name: "START MAIN REACTOR",
		Text: "Main reactor startup complete.",
	}
	if !empty {
		securityEntry.Blocks[0].FacilityTextVariants = []domain.FacilityTextVariant{{
			When: equality(fixtureFacilityPowerID, "online"), Text: "PRIMARY POWER: ONLINE",
		}}
		startReactor.AvailableWhen = new(equality(fixtureFacilityPowerID, "online"))
	}
	terminal := func(id, name string, children ...domain.ContentNode) domain.Terminal {
		return domain.Terminal{
			ID: id, Name: name, Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: children,
			},
		}
	}
	session := domain.Session{
		Version: 1, Name: "Facility authoring browser fixture",
		Terminals: []domain.Terminal{
			terminal(fixtureFacilityTerminalID, "Security control", securityEntry),
			terminal(fixtureFacilityReactorTerminalID, "Reactor control", startReactor),
		},
		Facility: &domain.Facility{
			Devices: []domain.FacilityDevice{}, Conditions: []domain.DiagnosticCondition{},
			RecoveryPrograms: []domain.RecoveryProgram{},
		},
	}
	if empty {
		return session
	}
	restorableDevice := func(id, name string, kind domain.FacilityDeviceKind) domain.FacilityDevice {
		return domain.FacilityDevice{
			ID: id, Name: name, Kind: kind, InitialStateID: "offline", CurrentStateID: "offline",
			States: []domain.FacilityDeviceState{{ID: "offline", Name: "Offline"}, {ID: "online", Name: "Online"}},
			Transitions: []domain.FacilityDeviceTransition{{
				ID: "restore", Name: "Restore", SourceStateID: "offline", DestinationStateID: "online", Recovery: true,
			}},
		}
	}
	session.Facility = &domain.Facility{
		Revision: 7,
		Devices: []domain.FacilityDevice{
			restorableDevice(fixtureFacilityPowerID, "Primary power grid", domain.FacilityDeviceKindPowerGrid),
			restorableDevice(fixtureFacilityCoolingID, "Reactor cooling loop", domain.FacilityDeviceKindVentilation),
			{
				ID: fixtureFacilityDoorID, Name: "Security sector door", Kind: domain.FacilityDeviceKindDoor,
				InitialStateID: "locked", CurrentStateID: "locked",
				States: []domain.FacilityDeviceState{{ID: "locked", Name: "Locked"}, {ID: "open", Name: "Open"}},
			},
		},
		Conditions: []domain.DiagnosticCondition{},
		RecoveryPrograms: []domain.RecoveryProgram{{
			ID: "program-network-recovery", Name: "VAULT-TEC NETWORK RECOVERY",
			Transitions: []domain.FacilityTransitionRequest{{DeviceID: fixtureFacilityPowerID, TransitionID: "restore"}},
		}},
	}
	return session
}

func protectFixtureFacilityCurrentValues(current, candidate *domain.Facility) {
	if candidate == nil {
		return
	}
	devices := make(map[string]string)
	conditions := make(map[string]bool)
	if current != nil {
		for _, device := range current.Devices {
			devices[device.ID] = device.CurrentStateID
		}
		for _, condition := range current.Conditions {
			conditions[condition.ID] = condition.CurrentActive
		}
	}
	for index := range candidate.Devices {
		device := &candidate.Devices[index]
		device.CurrentStateID = device.InitialStateID
		if value, ok := devices[device.ID]; ok {
			device.CurrentStateID = value
		}
	}
	for index := range candidate.Conditions {
		condition := &candidate.Conditions[index]
		condition.CurrentActive = condition.InitialActive
		if value, ok := conditions[condition.ID]; ok {
			condition.CurrentActive = value
		}
	}
}

func fixtureFacilityAffectedIDs(current, candidate *domain.Facility) ([]string, []string) {
	deviceBefore := make(map[string]domain.FacilityDevice)
	deviceAfter := make(map[string]domain.FacilityDevice)
	conditionBefore := make(map[string]domain.DiagnosticCondition)
	conditionAfter := make(map[string]domain.DiagnosticCondition)
	if current != nil {
		for _, device := range current.Devices {
			device.CurrentStateID = ""
			deviceBefore[device.ID] = device
		}
		for _, condition := range current.Conditions {
			condition.CurrentActive = false
			conditionBefore[condition.ID] = condition
		}
	}
	if candidate != nil {
		for _, device := range candidate.Devices {
			device.CurrentStateID = ""
			deviceAfter[device.ID] = device
		}
		for _, condition := range candidate.Conditions {
			condition.CurrentActive = false
			conditionAfter[condition.ID] = condition
		}
	}
	deviceIDs := fixtureChangedFacilityIDs(deviceBefore, deviceAfter)
	conditionIDs := fixtureChangedFacilityIDs(conditionBefore, conditionAfter)
	return deviceIDs, conditionIDs
}

func fixtureChangedFacilityIDs[T any](before, after map[string]T) []string {
	ids := make(map[string]struct{}, len(before)+len(after))
	for id := range before {
		ids[id] = struct{}{}
	}
	for id := range after {
		ids[id] = struct{}{}
	}
	changed := make([]string, 0, len(ids))
	for id := range ids {
		left, leftOK := before[id]
		right, rightOK := after[id]
		if leftOK != rightOK || !reflect.DeepEqual(left, right) {
			changed = append(changed, id)
		}
	}
	slices.Sort(changed)
	return changed
}

func fixtureFacilityHasDevice(facility *domain.Facility, deviceID string) bool {
	if facility == nil {
		return false
	}
	return slices.ContainsFunc(facility.Devices, func(device domain.FacilityDevice) bool {
		return device.ID == deviceID
	})
}

func fixtureFacilityBindingCount(session domain.Session) int {
	count := 0
	var visit func(domain.ContentNode)
	visit = func(node domain.ContentNode) {
		count += len(node.FacilityNameVariants)
		if node.VisibleWhen != nil {
			count++
		}
		if node.AvailableWhen != nil {
			count++
		}
		for _, block := range node.Blocks {
			count += len(block.FacilityTextVariants)
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	for _, terminal := range session.Terminals {
		visit(terminal.Root)
	}
	return count
}

func stateChangingApprovalTarget() domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: "terminal-stateful", TerminalName: "Терминал охраны", HackLevel: 0,
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{
				{
					ID: "approval-guide", Type: domain.NodeFolder, Name: "СПРАВКА",
					Children: []domain.ContentNode{{
						ID: "archive-note", Type: domain.NodeEntry, Name: "ПАМЯТКА",
						Description: "Команды требуют разрешения смотрителя.",
					}},
				},
				{
					ID: "renderer-reference", Type: domain.NodeEntry, Name: "ЭТАЛОН РЕНДЕРА",
					Description: fixtureCommandResult,
				},
				{
					ID: "reactor-state", Type: domain.NodeEntry, Name: "СОСТОЯНИЕ РЕАКТОРА",
					Blocks: []domain.EntryContentBlock{
						{ID: "b_reactor_power", InitialText: "ПИТАНИЕ: ОТКЛЮЧЕНО"},
						{ID: "b_reactor_cooling", InitialText: "СТАТУС: НЕИЗВЕСТЕН"},
						{ID: "b_reactor_air", InitialText: "СТАТУС: НЕИЗВЕСТЕН"},
						{ID: "b_reactor_note", InitialText: ""},
						{ID: "b_reactor_lock", InitialText: "БЛОКИРОВКА: ВКЛЮЧЕНА"},
					},
				},
				{
					ID: "diagnostics", Type: domain.NodeCommand, Name: "Запустить диагностику",
					Text: "Диагностика завершена.",
				},
				{
					ID: "doors", Type: domain.NodeCommand, Name: "Открыть двери",
					Text: "Доступ в сектор разрешён.",
					StateChange: &domain.StateChangeConfig{
						CompletedName:    "Двери открыты",
						ConfirmationText: "Разрешить доступ в защищённый сектор?",
					},
				},
				fixtureReactorCommand(
					"n_reactor_power", "Включить питание реактора", "Питание реактора включено",
					"Подтвердить включение питания реактора?", "Питание реактора включено.",
					"b_reactor_power", "ПИТАНИЕ: ВКЛЮЧЕНО",
				),
				fixtureReactorCommand(
					"n_reactor_cooling", "Запустить охлаждение", "Охлаждение работает",
					"Подтвердить запуск охлаждения реактора?", "Охлаждение реактора запущено.",
					"b_reactor_cooling", "ОХЛАЖДЕНИЕ: НОРМА",
				),
				fixtureReactorCommand(
					"n_reactor_air", "Проверить вентиляцию", "Вентиляция проверена",
					"Подтвердить проверку вентиляции реактора?", "Вентиляция проверена.",
					"b_reactor_air", "ВЕНТИЛЯЦИЯ: НОРМА",
				),
				fixtureReactorCommand(
					"n_reactor_note", "Очистить примечание", "Примечание очищено",
					"Удалить служебное примечание?", "Примечание очищено.",
					"b_reactor_note", "",
				),
				fixtureReactorCommand(
					"n_reactor_lock", "Снять блокировку реактора", "Блокировка реактора снята",
					"Подтвердить снятие блокировки реактора?", "Блокировка реактора снята.",
					"b_reactor_lock", "БЛОКИРОВКА: СНЯТА",
				),
			},
		},
	}
}

func fixtureReactorCommand(id, name, completedName, confirmationText, resultText, blockID, completedText string) domain.ContentNode {
	return domain.ContentNode{
		ID: id, Type: domain.NodeCommand, Name: name, Text: resultText,
		StateChange: &domain.StateChangeConfig{
			CompletedName: completedName, ConfirmationText: confirmationText,
			EntryContentChange: &domain.EntryContentChange{BlockID: blockID, CompletedText: completedText},
		},
	}
}

func fixtureStateChangingCommand(commandID string) *domain.ContentNode {
	target := stateChangingApprovalTarget()
	var find func(*domain.ContentNode) *domain.ContentNode
	find = func(node *domain.ContentNode) *domain.ContentNode {
		if node.ID == commandID && node.Type == domain.NodeCommand && node.StateChange != nil {
			return node
		}
		for index := range node.Children {
			if command := find(&node.Children[index]); command != nil {
				return command
			}
		}
		return nil
	}
	return find(&target.Tree)
}

func stateChangingApprovalSession(states map[string]domain.CommandExecutionState) domain.Session {
	target := stateChangingApprovalTarget()
	return domain.Session{
		Version: 1,
		Name:    "State-changing approval fixture",
		Terminals: []domain.Terminal{{
			ID: target.TerminalID, Name: target.TerminalName, HackLevel: target.HackLevel,
			IntroText: target.IntroText, Root: target.Tree,
			CommandStates: cloneFixtureCommandStates(states),
		}},
	}
}

func stateChangingAuthoringSession() domain.Session {
	return domain.Session{
		Version: 1,
		Name:    "State-changing authoring fixture",
		Terminals: []domain.Terminal{{
			ID: "terminal-stateful", Name: "Терминал охраны",
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{
					{
						ID: "emergency-lights", Type: domain.NodeCommand,
						Name: "Включить аварийный свет", Text: "Аварийное освещение включено.",
					},
					{
						ID: "doors", Type: domain.NodeCommand, Name: "Открыть двери",
						Text: "Новая редакция результата открытия.",
						StateChange: &domain.StateChangeConfig{
							CompletedName: "Двери разблокированы", ConfirmationText: "Открыть двери?",
						},
					},
					{
						ID: "alarm", Type: domain.NodeCommand, Name: "Включить тревогу",
						Text: "Сигнал тревоги активирован.",
						StateChange: &domain.StateChangeConfig{
							CompletedName: "Сигнал тревоги активен", ConfirmationText: "Включить тревогу?",
						},
					},
				},
			},
			CommandStates: map[string]domain.CommandExecutionState{
				"doors": {CompletedName: "Двери открыты", ResultText: "Доступ в сектор разрешён."},
				"alarm": {CompletedName: "Тревога включена", ResultText: "Охрана сектора предупреждена."},
			},
		}},
	}
}

func terminalGroupingSession(scenario string) (domain.Session, error) {
	scenario = strings.TrimSpace(scenario)
	if scenario == "" {
		scenario = "canonical"
	}
	terminal := func(id, name string) domain.Terminal {
		return domain.Terminal{
			ID: id, Name: name,
			Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}},
		}
	}
	switch scenario {
	case "canonical":
		return domain.Session{
			Version: 1, Name: "Terminal grouping canonical fixture",
			Terminals: []domain.Terminal{
				terminal("residential", "Жилой терминал"),
				terminal("security", "Терминал охраны"),
				terminal("vault", "Терминал хранилища"),
			},
			TerminalGroups: []domain.TerminalGroup{
				{ID: "vault-route", Name: "Маршрут хранилища", TerminalIDs: []string{"security", "residential"}},
				{ID: "vault-standalone", Name: "Отдельное хранилище", TerminalIDs: []string{"vault"}},
			},
		}, nil
	case "singleton":
		return domain.Session{
			Version: 1, Name: "Terminal grouping singleton fixture",
			Terminals: []domain.Terminal{
				terminal("medical", "Медицинский терминал"),
				terminal("reactor", "Терминал реактора"),
				terminal("archive", "Архивный терминал"),
			},
			TerminalGroups: []domain.TerminalGroup{
				{ID: "medical-singleton", Name: "Медицинский блок", TerminalIDs: []string{"medical"}},
				{ID: "reactor-singleton", Name: "Реакторный блок", TerminalIDs: []string{"reactor"}},
				{ID: "archive-singleton", Name: "Архивный блок", TerminalIDs: []string{"archive"}},
			},
		}, nil
	case "ordered":
		return domain.Session{
			Version: 1, Name: "Terminal grouping ordered fixture",
			Terminals: []domain.Terminal{
				terminal("alpha", "Терминал Альфа"),
				terminal("beta", "Терминал Бета"),
				terminal("gamma", "Терминал Гамма"),
				terminal("delta", "Терминал Дельта"),
				terminal("epsilon", "Терминал Эпсилон"),
			},
			TerminalGroups: []domain.TerminalGroup{
				{ID: "south-route", Name: "Южный маршрут", TerminalIDs: []string{"delta", "beta"}},
				{ID: "north-route", Name: "Северный маршрут", TerminalIDs: []string{"gamma", "alpha"}},
				{ID: "epsilon-singleton", Name: "Терминал Гамма", TerminalIDs: []string{"epsilon"}},
			},
		}, nil
	case "legacy":
		legacyOne := terminal("legacy-one", "Старый терминал 1")
		legacyOne.Root.Children = append(legacyOne.Root.Children, domain.ContentNode{
			ID: "legacy-transition", Type: domain.NodeCommand, Name: "СТАРЫЙ ПЕРЕХОД",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "legacy-two"},
		})
		return domain.Session{
			Version: 1, Name: "Terminal grouping legacy fixture",
			Terminals: []domain.Terminal{
				legacyOne,
				terminal("legacy-two", "Старый терминал 2"),
				terminal("legacy-three", "Старый терминал 3"),
			},
		}, nil
	case "legacy-multi-link":
		service := terminal("t-krel-service", "K-REL / СЕРВИСНЫЙ КОНТУР")
		service.Root.Children = append(service.Root.Children, domain.ContentNode{
			ID: "svc-access-admin", Type: domain.NodeCommand, Name: "ВХОД АДМИНИСТРАТОРА",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "t-krel-admin"},
		})
		admin := terminal("t-krel-admin", "K-REL / АДМИНИСТРАТОР")
		admin.Root.Children = append(admin.Root.Children, domain.ContentNode{
			ID: "adm-emergency", Type: domain.NodeCommand, Name: "АВАРИЙНОЕ УПРАВЛЕНИЕ",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "t-krel-emergency"},
		})
		emergency := terminal("t-krel-emergency", "K-REL / АВАРИЙНОЕ УПРАВЛЕНИЕ")
		emergency.HackLevel = 4
		return domain.Session{
			Version: 1, Name: "session-05-cold-storage",
			Terminals: []domain.Terminal{service, admin, emergency},
		}, nil
	case "legacy-multi-link-authored":
		return bug004AuthoredSession()
	case "ordered-navigation":
		alpha := terminal("alpha", "Терминал Альфа")
		alpha.Root.Children = append(alpha.Root.Children, domain.ContentNode{
			ID: "go-beta", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ТЕРМИНАЛ БЕТА",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "beta"},
		})
		beta := terminal("beta", "Терминал Бета")
		beta.Root.Children = append(beta.Root.Children,
			domain.ContentNode{
				ID: "go-gamma", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ТЕРМИНАЛ ГАММА",
				TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "gamma"},
			},
			domain.ContentNode{
				ID: "go-gamma-backup", Type: domain.NodeCommand, Name: "РЕЗЕРВНЫЙ МАРШРУТ К ГАММЕ",
				TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "gamma"},
			},
		)
		gamma := terminal("gamma", "Терминал Гамма")
		gamma.Root.Children = append(gamma.Root.Children,
			domain.ContentNode{ID: "check-link", Type: domain.NodeCommand, Name: "ПРОВЕРИТЬ СВЯЗЬ", Text: "СВЯЗЬ СТАБИЛЬНА"},
			domain.ContentNode{
				ID: "go-delta", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ТЕРМИНАЛ ДЕЛЬТА",
				TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "delta"},
			},
		)
		return domain.Session{
			Version: 1, Name: "Terminal grouping ordered navigation fixture",
			Terminals: []domain.Terminal{
				alpha, beta, gamma, terminal("delta", "Терминал Дельта"), terminal("epsilon", "Терминал Эпсилон"),
			},
			TerminalGroups: []domain.TerminalGroup{
				{ID: "ordered-route", Name: "Полный маршрут", TerminalIDs: []string{"alpha", "beta", "gamma", "delta"}},
				{ID: "epsilon-standalone", Name: "Отдельный терминал", TerminalIDs: []string{"epsilon"}},
			},
		}, nil
	default:
		return domain.Session{}, fmt.Errorf("unknown terminal-grouping scenario %q", scenario)
	}
}

func bug004AuthoredSession() (domain.Session, error) {
	const exactSHA256 = "b4ca8b89b7d7af32e05a9b598a007e36a747ef59ce3e2bd15a60d0b3f0ec9438"
	exactPath := strings.TrimSpace(os.Getenv("FALLOUT_BUG004_SOURCE"))
	candidates := []string{exactPath}
	if exactPath == "" {
		candidates = []string{
			filepath.Join("..", "fixtures", "session-05-cold-storage.json"),
			filepath.Join("tests", "fixtures", "session-05-cold-storage.json"),
			filepath.Join("..", "..", "tests", "fixtures", "session-05-cold-storage.json"),
		}
	}
	var raw []byte
	var sourcePath string
	var readErr error
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		raw, readErr = os.ReadFile(candidate)
		if readErr == nil {
			sourcePath = candidate
			break
		}
	}
	if readErr != nil || sourcePath == "" {
		return domain.Session{}, fmt.Errorf("read BUG-004 authored fixture: %w", readErr)
	}
	if exactPath != "" && fmt.Sprintf("%x", sha256.Sum256(raw)) != exactSHA256 {
		return domain.Session{}, fmt.Errorf("BUG-004 authored source SHA-256 does not match %s", exactSHA256)
	}
	session, err := domain.DecodeSession(raw)
	if err != nil {
		return domain.Session{}, fmt.Errorf("decode BUG-004 authored fixture %s: %w", sourcePath, err)
	}
	return session, nil
}

func terminalNavigationSession() domain.Session {
	return domain.Session{
		Version: 1, Name: "Terminal navigation fixture",
		Terminals: []domain.Terminal{
			{
				ID: "residential", Name: "Жилой терминал",
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
					{ID: "ordinary", Type: domain.NodeCommand, Name: "ЗАПУСТИТЬ ДИАГНОСТИКУ", Text: "СИСТЕМА ИСПРАВНА"},
					{ID: "go-security", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ОХРАНУ", TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "security"}},
					{ID: "navigation", Type: domain.NodeFolder, Name: "НАВИГАЦИЯ", Children: []domain.ContentNode{
						{ID: "go-security-nested", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ОХРАНУ ИЗ ПАПКИ", TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "security"}},
					}},
					{ID: "completed", Type: domain.NodeCommand, Name: "ЗАВЕРШЁННАЯ КОМАНДА", Text: "Done", StateChange: &domain.StateChangeConfig{CompletedName: "ЗАВЕРШЁННАЯ КОМАНДА", ConfirmationText: "Run?"}},
				}},
				CommandStates: map[string]domain.CommandExecutionState{"completed": {CompletedName: "ЗАВЕРШЁННАЯ КОМАНДА", ResultText: "Done"}},
			},
			{
				ID: "security", Name: "Терминал охраны", HackLevel: 1,
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
					{ID: "security-summary", Type: domain.NodeCommand, Name: "ЗАПРОСИТЬ СВОДКУ БЕЗОПАСНОСТИ", Text: "СЕКТОР БЕЗОПАСЕН"},
					{ID: "go-vault", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ХРАНИЛИЩЕ", TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "vault"}},
				}},
			},
			{
				ID: "vault", Name: "Терминал хранилища",
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}},
			},
		},
		TerminalGroups: []domain.TerminalGroup{
			{ID: "navigation-route", Name: "Навигационный маршрут", TerminalIDs: []string{"residential", "security"}},
			{ID: "vault-singleton", Name: "Терминал хранилища", TerminalIDs: []string{"vault"}},
		},
	}
}

func stateChangingSyncTarget() domain.TerminalTarget {
	target := stateChangingApprovalTarget()
	target.TerminalName = "Терминал синхронизации"
	target.Tree.Children = append(target.Tree.Children, domain.ContentNode{
		ID: "archive", Type: domain.NodeFolder, Name: "АРХИВ",
		Children: []domain.ContentNode{{
			ID: "journal", Type: domain.NodeEntry, Name: "ЖУРНАЛ",
			Description: "Состояние дверей зарегистрировано.",
		}},
	})
	return target
}

func stateChangingSyncReserveTarget() domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: "terminal-reserve", TerminalName: "Резервный терминал", HackLevel: 0,
		Tree: domain.ContentNode{
			ID: "reserve-root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: "reserve-status", Type: domain.NodeEntry, Name: "РЕЗЕРВНЫЙ СТАТУС",
				Description: "Резервный канал активен.",
			}},
		},
	}
}

func stateChangingSyncSession(states map[string]domain.CommandExecutionState) domain.Session {
	target := stateChangingSyncTarget()
	return domain.Session{
		Version: 1,
		Name:    "State-changing synchronization fixture",
		Terminals: []domain.Terminal{{
			ID: target.TerminalID, Name: target.TerminalName, HackLevel: target.HackLevel,
			IntroText: target.IntroText, Root: target.Tree,
			CommandStates: cloneFixtureCommandStates(states),
		}},
	}
}

func cloneFixtureCommandStates(states map[string]domain.CommandExecutionState) map[string]domain.CommandExecutionState {
	if len(states) == 0 {
		return nil
	}
	clone := make(map[string]domain.CommandExecutionState, len(states))
	maps.Copy(clone, states)
	return clone
}

func crtFixtureTerminal() domain.TerminalTarget {
	children := make([]domain.ContentNode, 0, 25)
	for index := 1; index <= 21; index++ {
		children = append(children, domain.ContentNode{
			ID:          fmt.Sprintf("crt-row-%02d", index),
			Type:        domain.NodeEntry,
			Name:        fmt.Sprintf("ARCHIVE %02d", index),
			Description: fmt.Sprintf("CRT ARCHIVE LINE %02d", index),
		})
	}
	children = append(children,
		domain.ContentNode{ID: "crt-empty", Type: domain.NodeFolder, Name: "EMPTY", Children: []domain.ContentNode{}},
		domain.ContentNode{
			ID: "crt-record", Type: domain.NodeEntry, Name: "LONG RECORD",
			Description: strings.Repeat("ROBCO RECORD LINE\n", 48) + "RECORD COMPLETE",
		},
		domain.ContentNode{
			ID: "crt-command", Type: domain.NodeCommand, Name: "RUN DIAGNOSTIC",
			Text: strings.Repeat("DIAGNOSTIC OUTPUT\n", 96) + "DIAGNOSTIC COMPLETE",
		},
		domain.ContentNode{
			ID: "crt-literal", Type: domain.NodeEntry, Name: `<img data-crt-injected src=x onerror="window.__crtInjected=true">`,
			Description: `<script>window.__crtInjected=true</script> & literal terminal text`,
		},
	)

	return domain.TerminalTarget{
		TerminalID: "terminal-crt", TerminalName: "CRT Acceptance", HackLevel: 0,
		IntroText: "CRT PRESENTATION ACCEPTANCE",
		Tree:      domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: children},
	}
}

func crtReplacementTerminal() domain.TerminalTarget {
	target := crtFixtureTerminal()
	target.IntroText = "CRT REPLACEMENT ACCEPTANCE"
	target.Tree.Children = []domain.ContentNode{
		{ID: "crt-replacement-a", Type: domain.NodeEntry, Name: "REPLACEMENT ALPHA", Description: "ALPHA"},
		{ID: "crt-replacement-b", Type: domain.NodeEntry, Name: "REPLACEMENT BETA", Description: "BETA"},
		{ID: "crt-replacement-c", Type: domain.NodeEntry, Name: "REPLACEMENT GAMMA", Description: "GAMMA"},
	}
	return target
}

func crtHackingTerminal(identity string) domain.TerminalTarget {
	target := crtFixtureTerminal()
	target.TerminalID = "terminal-crt-hacking-" + identity
	target.TerminalName = "CRT Security " + strings.ToUpper(identity)
	target.HackLevel = 1
	return target
}
