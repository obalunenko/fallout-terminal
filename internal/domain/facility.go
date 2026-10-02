package domain

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
)

// FacilityDeviceKind identifies an authored category of world device.
type FacilityDeviceKind string

const (
	FacilityDeviceKindUnspecified    FacilityDeviceKind = ""
	FacilityDeviceKindDoor           FacilityDeviceKind = "door"
	FacilityDeviceKindTurret         FacilityDeviceKind = "turret"
	FacilityDeviceKindPowerGrid      FacilityDeviceKind = "power-grid"
	FacilityDeviceKindReactor        FacilityDeviceKind = "reactor"
	FacilityDeviceKindVentilation    FacilityDeviceKind = "ventilation"
	FacilityDeviceKindAlarm          FacilityDeviceKind = "alarm"
	FacilityDeviceKindRobotPod       FacilityDeviceKind = "robot-pod"
	FacilityDeviceKindElevator       FacilityDeviceKind = "elevator"
	FacilityDeviceKindNetworkSegment FacilityDeviceKind = "network-segment"
	FacilityDeviceKindCustom         FacilityDeviceKind = "custom"
)

// DiagnosticConditionCategory identifies a bounded authored fault category.
type DiagnosticConditionCategory string

const (
	DiagnosticConditionCategoryUnspecified            DiagnosticConditionCategory = ""
	DiagnosticConditionCategoryOffline                DiagnosticConditionCategory = "offline"
	DiagnosticConditionCategoryUnpowered              DiagnosticConditionCategory = "unpowered"
	DiagnosticConditionCategoryNetworkIsolated        DiagnosticConditionCategory = "network-isolated"
	DiagnosticConditionCategoryStorageDamaged         DiagnosticConditionCategory = "storage-damaged"
	DiagnosticConditionCategoryAuthorizationCorrupted DiagnosticConditionCategory = "authorization-corrupted"
	DiagnosticConditionCategoryDisplayUnstable        DiagnosticConditionCategory = "display-unstable"
	DiagnosticConditionCategoryCustom                 DiagnosticConditionCategory = "custom"
)

// FacilityCapability is a bounded player capability that a diagnostic
// condition may block.
type FacilityCapability string

const (
	FacilityCapabilityUnspecified        FacilityCapability = ""
	FacilityCapabilityExecuteCommand     FacilityCapability = "execute-command"
	FacilityCapabilityViewEntry          FacilityCapability = "view-entry"
	FacilityCapabilityHack               FacilityCapability = "hack"
	FacilityCapabilityTerminalTransition FacilityCapability = "terminal-transition"
	FacilityCapabilityRunRecoveryProgram FacilityCapability = "run-recovery-program"
)

// TerminalPresentationEffect is a safe, presentation-only terminal effect.
type TerminalPresentationEffect string

const (
	TerminalPresentationEffectUnspecified     TerminalPresentationEffect = ""
	TerminalPresentationEffectDisplayUnstable TerminalPresentationEffect = "display-unstable"
)

// Facility is the one session-wide durable facility aggregate.
type Facility struct {
	Revision         uint64                     `json:"revision"`
	Devices          []FacilityDevice           `json:"devices"`
	Conditions       []DiagnosticCondition      `json:"conditions"`
	RecoveryPrograms []RecoveryProgram          `json:"recoveryPrograms"`
	Extra            map[string]json.RawMessage `json:"-"`
}

// FacilityDevice is one reusable world device with a finite state graph.
type FacilityDevice struct {
	ID             string                     `json:"id"`
	Name           string                     `json:"name"`
	Kind           FacilityDeviceKind         `json:"kind"`
	CustomKind     string                     `json:"customKind,omitempty"`
	InitialStateID string                     `json:"initialStateId"`
	CurrentStateID string                     `json:"currentStateId"`
	States         []FacilityDeviceState      `json:"states"`
	Transitions    []FacilityDeviceTransition `json:"transitions,omitempty"`
	Extra          map[string]json.RawMessage `json:"-"`
}

// FacilityDeviceState is one named value in a device's finite state set.
type FacilityDeviceState struct {
	ID    string                     `json:"id"`
	Name  string                     `json:"name"`
	Extra map[string]json.RawMessage `json:"-"`
}

// FacilityDeviceTransition is one explicit allowed movement in a device's
// state graph.
type FacilityDeviceTransition struct {
	ID                 string                     `json:"id"`
	Name               string                     `json:"name"`
	SourceStateID      string                     `json:"sourceStateId"`
	DestinationStateID string                     `json:"destinationStateId"`
	Preconditions      []FacilityStateEquality    `json:"preconditions,omitempty"`
	ConditionEffects   []FacilityConditionEffect  `json:"conditionEffects,omitempty"`
	Recovery           bool                       `json:"recovery,omitempty"`
	Extra              map[string]json.RawMessage `json:"-"`
}

// FacilityStateEquality requires one device to equal one of its named states.
type FacilityStateEquality struct {
	DeviceID string                     `json:"deviceId"`
	StateID  string                     `json:"stateId"`
	Extra    map[string]json.RawMessage `json:"-"`
}

// FacilityTransitionRequest identifies an authored transition on one device.
type FacilityTransitionRequest struct {
	DeviceID     string                     `json:"deviceId"`
	TransitionID string                     `json:"transitionId"`
	Extra        map[string]json.RawMessage `json:"-"`
}

// FacilityConditionEffect explicitly activates or clears one condition.
type FacilityConditionEffect struct {
	ConditionID string                     `json:"conditionId"`
	Active      bool                       `json:"active"`
	Extra       map[string]json.RawMessage `json:"-"`
}

// DiagnosticDeviceScope attaches a condition to one device.
type DiagnosticDeviceScope struct {
	DeviceID string                     `json:"deviceId"`
	Extra    map[string]json.RawMessage `json:"-"`
}

// DiagnosticTerminalScope attaches a condition to one terminal.
type DiagnosticTerminalScope struct {
	TerminalID string                     `json:"terminalId"`
	Extra      map[string]json.RawMessage `json:"-"`
}

// CapabilityBlockEffect blocks one bounded player capability while active.
type CapabilityBlockEffect struct {
	Capability FacilityCapability         `json:"capability"`
	Extra      map[string]json.RawMessage `json:"-"`
}

// DiagnosticPathEffect exposes an authored diagnostic node while active.
type DiagnosticPathEffect struct {
	TerminalID string                     `json:"terminalId"`
	NodeID     string                     `json:"nodeId"`
	Extra      map[string]json.RawMessage `json:"-"`
}

// RecordSubstitutionEffect replaces one authored entry block while active.
type RecordSubstitutionEffect struct {
	TerminalID      string                     `json:"terminalId"`
	BlockID         string                     `json:"blockId"`
	ReplacementText string                     `json:"replacementText"`
	Extra           map[string]json.RawMessage `json:"-"`
}

// DisplayInstabilityEffect marks a bounded, presentation-only visual effect.
type DisplayInstabilityEffect struct {
	Extra map[string]json.RawMessage `json:"-"`
}

// DiagnosticEffect is an explicit effect variant. Validation requires exactly
// one pointer to be non-nil.
type DiagnosticEffect struct {
	CapabilityBlock    *CapabilityBlockEffect     `json:"capabilityBlock,omitempty"`
	DiagnosticPath     *DiagnosticPathEffect      `json:"diagnosticPath,omitempty"`
	RecordSubstitution *RecordSubstitutionEffect  `json:"recordSubstitution,omitempty"`
	DisplayInstability *DisplayInstabilityEffect  `json:"displayInstability,omitempty"`
	Extra              map[string]json.RawMessage `json:"-"`
}

// DiagnosticRecoveryReference is one allowlisted recovery variant. Validation
// requires exactly one pointer to be non-nil.
type DiagnosticRecoveryReference struct {
	Transition            *FacilityTransitionRequest `json:"transition,omitempty"`
	RecoveryProgramID     *string                    `json:"recoveryProgramId,omitempty"`
	PrivateOverseerAction *bool                      `json:"privateOverseerAction,omitempty"`
	Extra                 map[string]json.RawMessage `json:"-"`
}

// DiagnosticCondition is one deterministic authored fault and its current
// protected active value. Validation requires exactly one scope.
type DiagnosticCondition struct {
	ID             string                        `json:"id"`
	Name           string                        `json:"name"`
	Category       DiagnosticConditionCategory   `json:"category"`
	CustomCategory string                        `json:"customCategory,omitempty"`
	Device         *DiagnosticDeviceScope        `json:"device,omitempty"`
	Terminal       *DiagnosticTerminalScope      `json:"terminal,omitempty"`
	InitialActive  bool                          `json:"initialActive"`
	CurrentActive  bool                          `json:"currentActive"`
	Effects        []DiagnosticEffect            `json:"effects,omitempty"`
	Recovery       []DiagnosticRecoveryReference `json:"recovery,omitempty"`
	Extra          map[string]json.RawMessage    `json:"-"`
}

// RecoveryProgram is a finite named list of authored transition requests.
type RecoveryProgram struct {
	ID          string                      `json:"id"`
	Name        string                      `json:"name"`
	Transitions []FacilityTransitionRequest `json:"transitions"`
	Extra       map[string]json.RawMessage  `json:"-"`
}

// ExpandRecoveryProgram resolves one authored recovery program to its finite,
// ordered world action. The returned requests and issues are detached from the
// facility definition.
func ExpandRecoveryProgram(facility *Facility, programID string) ([]FacilityTransitionRequest, []FacilityIssue) {
	if facility == nil {
		return nil, facilityExpansionIssues(
			FacilityFailureMissingReference,
			FacilityEntityKindRecoveryProgram,
			programID,
			nil,
		)
	}
	if err := validateFacilityID("recoveryProgramId", programID); err != nil {
		return nil, facilityExpansionIssues(
			FacilityFailureInvalidConfiguration,
			FacilityEntityKindRecoveryProgram,
			programID,
			nil,
		)
	}

	var program *RecoveryProgram
	for index := range facility.RecoveryPrograms {
		candidate := &facility.RecoveryPrograms[index]
		if candidate.ID != programID {
			continue
		}
		if program != nil {
			return nil, facilityExpansionIssues(
				FacilityFailureInvalidConfiguration,
				FacilityEntityKindRecoveryProgram,
				programID,
				nil,
			)
		}
		program = candidate
	}
	if program == nil {
		return nil, facilityExpansionIssues(
			FacilityFailureMissingReference,
			FacilityEntityKindRecoveryProgram,
			programID,
			nil,
		)
	}
	if len(program.Transitions) == 0 || len(program.Transitions) > maxFacilityItemsPerList {
		return nil, facilityExpansionIssues(
			FacilityFailureInvalidConfiguration,
			FacilityEntityKindRecoveryProgram,
			programID,
			nil,
		)
	}

	devices := make(map[string]*FacilityDevice, len(facility.Devices))
	for index := range facility.Devices {
		device := &facility.Devices[index]
		if _, duplicate := devices[device.ID]; duplicate {
			return nil, facilityExpansionIssues(
				FacilityFailureInvalidConfiguration,
				FacilityEntityKindDevice,
				device.ID,
				nil,
			)
		}
		devices[device.ID] = device
	}

	requested := make(map[string]FacilityDeviceTransition, len(program.Transitions))
	conditionEffects := make(map[string]bool)
	for _, request := range program.Transitions {
		if _, duplicate := requested[request.DeviceID]; duplicate {
			return nil, facilityExpansionIssues(
				FacilityFailureConflict,
				FacilityEntityKindRecoveryProgram,
				programID,
				new(request.DeviceID),
			)
		}
		device, exists := devices[request.DeviceID]
		if !exists {
			return nil, facilityExpansionIssues(
				FacilityFailureMissingReference,
				FacilityEntityKindDevice,
				request.DeviceID,
				nil,
			)
		}

		transition, exists, duplicate := deviceTransitionByID(device.Transitions, request.TransitionID)
		if duplicate {
			return nil, facilityExpansionIssues(
				FacilityFailureInvalidConfiguration,
				FacilityEntityKindDeviceTransition,
				request.TransitionID,
				new(request.DeviceID),
			)
		}
		if !exists {
			return nil, facilityExpansionIssues(
				FacilityFailureMissingReference,
				FacilityEntityKindDeviceTransition,
				request.TransitionID,
				new(request.DeviceID),
			)
		}
		requested[request.DeviceID] = transition
		for _, effect := range transition.ConditionEffects {
			active, seen := conditionEffects[effect.ConditionID]
			if seen && active != effect.Active {
				return nil, facilityExpansionIssues(
					FacilityFailureConflict,
					FacilityEntityKindRecoveryProgram,
					programID,
					new(effect.ConditionID),
				)
			}
			conditionEffects[effect.ConditionID] = effect.Active
		}
	}
	if err := validateFacilityActionDependencies("recoveryProgram", requested); err != nil {
		return nil, facilityExpansionIssues(
			FacilityFailureConflict,
			FacilityEntityKindRecoveryProgram,
			programID,
			nil,
		)
	}
	return CloneFacilityTransitionRequests(program.Transitions), nil
}

func deviceTransitionByID(
	transitions []FacilityDeviceTransition,
	transitionID string,
) (FacilityDeviceTransition, bool, bool) {
	var result FacilityDeviceTransition
	found := false
	for _, transition := range transitions {
		if transition.ID != transitionID {
			continue
		}
		if found {
			return FacilityDeviceTransition{}, false, true
		}
		result = transition
		found = true
	}
	return result, found, false
}

func facilityExpansionIssues(
	code FacilityFailureCode,
	kind FacilityEntityKind,
	entityID string,
	ownerID *string,
) []FacilityIssue {
	issue := FacilityIssue{Code: code, EntityKind: string(kind), EntityID: new(entityID)}
	if ownerID != nil {
		referenceKind := "owner"
		issue.ReferenceKind = &referenceKind
		issue.ReferenceID = cloneString(ownerID)
	}
	return []FacilityIssue{issue}
}

// FacilityTransitionList wraps a direct multi-device action so its presence
// remains distinct from an absent facility action.
type FacilityTransitionList struct {
	Transitions []FacilityTransitionRequest `json:"transitions"`
	Extra       map[string]json.RawMessage  `json:"-"`
}

// FacilityActionConfig selects either direct transitions or one recovery
// program. Validation requires exactly one action variant.
type FacilityActionConfig struct {
	Transitions       *FacilityTransitionList    `json:"transitions,omitempty"`
	RecoveryProgramID *string                    `json:"recoveryProgramId,omitempty"`
	Extra             map[string]json.RawMessage `json:"-"`
}

// FacilityTextVariant supplies authored text when one state equality matches.
type FacilityTextVariant struct {
	When  FacilityStateEquality      `json:"when"`
	Text  string                     `json:"text"`
	Extra map[string]json.RawMessage `json:"-"`
}

// PendingFacilityAction is the detached world-action intent captured during
// the existing private command approval lifecycle.
type PendingFacilityAction struct {
	ExpectedFacilityRevision uint64                      `json:"expectedFacilityRevision"`
	ActionFingerprint        string                      `json:"-"`
	TransitionRequests       []FacilityTransitionRequest `json:"-"`
	ExpectedSourceStates     []FacilityStateEquality     `json:"-"`
	AffectedConditionIDs     []string                    `json:"-"`
	RecoveryProgramID        *string                     `json:"recoveryProgramId,omitempty"`
}

// MarshalJSON exposes only the redacted approval summary carried by the
// private desktop contract. Transition identities, source states, and the
// action fingerprint remain coordinator-private.
func (action PendingFacilityAction) MarshalJSON() ([]byte, error) {
	var deviceIDs []string
	if action.TransitionRequests != nil {
		deviceIDs = make([]string, len(action.TransitionRequests))
		for index, request := range action.TransitionRequests {
			deviceIDs[index] = request.DeviceID
		}
	}
	return json.Marshal(struct {
		ExpectedFacilityRevision uint64   `json:"expectedFacilityRevision"`
		DeviceIDs                []string `json:"deviceIds,omitempty"`
		ConditionIDs             []string `json:"conditionIds,omitempty"`
		RecoveryProgramID        *string  `json:"recoveryProgramId,omitempty"`
	}{
		ExpectedFacilityRevision: action.ExpectedFacilityRevision,
		DeviceIDs:                deviceIDs,
		ConditionIDs:             slices.Clone(action.AffectedConditionIDs),
		RecoveryProgramID:        cloneString(action.RecoveryProgramID),
	})
}

// FacilityFailureCode is a stable structured facility operation outcome.
type FacilityFailureCode string

const (
	FacilityFailureUnspecified          FacilityFailureCode = "unspecified"
	FacilityFailureRejected             FacilityFailureCode = "rejected"
	FacilityFailureMissingReference     FacilityFailureCode = "missing-reference"
	FacilityFailureInvalidTransition    FacilityFailureCode = "invalid-transition"
	FacilityFailurePreconditionFailed   FacilityFailureCode = "precondition-failed"
	FacilityFailureStaleRevision        FacilityFailureCode = "stale-revision"
	FacilityFailureConflict             FacilityFailureCode = "conflict"
	FacilityFailureDuplicate            FacilityFailureCode = "duplicate"
	FacilityFailureInvalidConfiguration FacilityFailureCode = "invalid-configuration"
	FacilityFailurePersistenceFailed    FacilityFailureCode = "persistence-failed"
	FacilityFailureRuntimeContextEnded  FacilityFailureCode = "runtime-context-ended"
)

// FacilityIssue identifies one stable configuration or operation failure.
type FacilityIssue struct {
	Code          FacilityFailureCode `json:"code"`
	EntityKind    string              `json:"entityKind"`
	EntityID      *string             `json:"entityId,omitempty"`
	ReferenceKind *string             `json:"referenceKind,omitempty"`
	ReferenceID   *string             `json:"referenceId,omitempty"`
}

// FacilityOperationResult is the detached outcome of one serialized facility
// mutation attempt.
type FacilityOperationResult struct {
	OK                        bool                `json:"ok"`
	Changed                   bool                `json:"changed"`
	CorrelationID             string              `json:"correlationId"`
	Failure                   FacilityFailureCode `json:"failure,omitempty"`
	Issues                    []FacilityIssue     `json:"issues,omitempty"`
	SessionRevision           uint64              `json:"sessionRevision"`
	PreviousFacilityRevision  uint64              `json:"previousFacilityRevision"`
	ResultingFacilityRevision uint64              `json:"resultingFacilityRevision"`
	AffectedDeviceIDs         []string            `json:"affectedDeviceIds,omitempty"`
	AffectedConditionIDs      []string            `json:"affectedConditionIds,omitempty"`
	Session                   *Session            `json:"session,omitempty"`
}

// FacilityEntityKind identifies a durable entity addressed by dependency
// inspection and repair.
type FacilityEntityKind string

const (
	FacilityEntityKindUnspecified      FacilityEntityKind = "unspecified"
	FacilityEntityKindDevice           FacilityEntityKind = "device"
	FacilityEntityKindDeviceState      FacilityEntityKind = "device-state"
	FacilityEntityKindDeviceTransition FacilityEntityKind = "device-transition"
	FacilityEntityKindCondition        FacilityEntityKind = "condition"
	FacilityEntityKindRecoveryProgram  FacilityEntityKind = "recovery-program"
)

// FacilityDependencyKind identifies where one facility reference originates.
type FacilityDependencyKind string

const (
	FacilityDependencyKindUnspecified               FacilityDependencyKind = "unspecified"
	FacilityDependencyKindTransitionPrecondition    FacilityDependencyKind = "transition-precondition"
	FacilityDependencyKindTransitionConditionEffect FacilityDependencyKind = "transition-condition-effect"
	FacilityDependencyKindRecoveryReference         FacilityDependencyKind = "recovery-reference"
	FacilityDependencyKindRecoveryProgramTransition FacilityDependencyKind = "recovery-program-transition"
	FacilityDependencyKindCommandAction             FacilityDependencyKind = "command-action"
	FacilityDependencyKindNameVariant               FacilityDependencyKind = "name-variant"
	FacilityDependencyKindEntryContentVariant       FacilityDependencyKind = "entry-content-variant"
	FacilityDependencyKindVisibility                FacilityDependencyKind = "visibility"
	FacilityDependencyKindAvailability              FacilityDependencyKind = "availability"
	FacilityDependencyKindDiagnosticScope           FacilityDependencyKind = "diagnostic-scope"
	FacilityDependencyKindDiagnosticEffect          FacilityDependencyKind = "diagnostic-effect"
)

// FacilityEntityReference identifies a dependency target. OwnerID qualifies a
// device-scoped state or transition identity.
type FacilityEntityReference struct {
	Kind     FacilityEntityKind `json:"kind"`
	EntityID string             `json:"entityId"`
	OwnerID  *string            `json:"ownerId,omitempty"`
}

// FacilityDependency identifies one direct reference to a facility entity.
type FacilityDependency struct {
	Kind       FacilityDependencyKind `json:"kind"`
	SourceID   string                 `json:"sourceId"`
	TargetID   string                 `json:"targetId"`
	Property   string                 `json:"property"`
	TerminalID *string                `json:"terminalId,omitempty"`
}

// FacilityDependencyReport is a detached list of references to one entity.
type FacilityDependencyReport struct {
	Target       FacilityEntityReference `json:"target"`
	Dependencies []FacilityDependency    `json:"dependencies"`
}

type facilityDependencyEdge struct {
	source     FacilityEntityReference
	target     FacilityEntityReference
	dependency FacilityDependency
}

// BuildFacilityDependencyReport returns every direct relationship involving
// target in deterministic order. The returned report and issues are detached
// from the supplied session.
func BuildFacilityDependencyReport(session Session, target FacilityEntityReference) (FacilityDependencyReport, []FacilityIssue) {
	target = cloneFacilityEntityReference(target)
	report := FacilityDependencyReport{Target: target}
	if err := ValidateSession(session); err != nil {
		return report, []FacilityIssue{{
			Code: FacilityFailureInvalidConfiguration, EntityKind: "facility",
		}}
	}
	if !facilityEntityExists(session.Facility, target) {
		return report, []FacilityIssue{missingFacilityEntityIssue(target)}
	}

	for _, edge := range buildFacilityDependencyEdges(session) {
		if sameFacilityEntity(edge.source, target) || sameFacilityEntity(edge.target, target) {
			report.Dependencies = append(report.Dependencies, cloneFacilityDependency(edge.dependency))
		}
	}
	slices.SortFunc(report.Dependencies, compareFacilityDependencies)
	return report, nil
}

// ValidateFacilityAuthoringCandidate validates one complete replacement and
// reports unresolved stable-identity changes without relying on error text.
// A candidate that repairs every reference is accepted as a complete graph.
func ValidateFacilityAuthoringCandidate(current, candidate Session) []FacilityIssue {
	if err := ValidateSession(current); err != nil {
		return []FacilityIssue{{Code: FacilityFailureInvalidConfiguration, EntityKind: "facility"}}
	}
	if err := ValidateSession(candidate); err == nil {
		return nil
	}

	edges := buildFacilityDependencyEdges(current)
	removed := removedFacilityEntities(current.Facility, candidate.Facility)
	issues := make([]FacilityIssue, 0, len(removed))
	for _, entity := range removed {
		if !facilityEntityHasInboundReference(current.Facility, edges, entity) {
			continue
		}
		entityID := entity.EntityID
		issue := FacilityIssue{
			Code: FacilityFailureConflict, EntityKind: string(entity.Kind), EntityID: &entityID,
		}
		if entity.OwnerID != nil {
			referenceKind := "owner"
			ownerID := *entity.OwnerID
			issue.ReferenceKind = &referenceKind
			issue.ReferenceID = &ownerID
		}
		issues = append(issues, issue)
	}
	if len(issues) != 0 {
		return issues
	}
	return []FacilityIssue{{Code: FacilityFailureInvalidConfiguration, EntityKind: "facility"}}
}

func buildFacilityDependencyEdges(session Session) []facilityDependencyEdge {
	if session.Facility == nil {
		return nil
	}
	edges := make([]facilityDependencyEdge, 0)
	add := func(source, target FacilityEntityReference, kind FacilityDependencyKind, sourceID, targetID, property string, terminalID *string) {
		edges = append(edges, facilityDependencyEdge{
			source: cloneFacilityEntityReference(source), target: cloneFacilityEntityReference(target),
			dependency: FacilityDependency{
				Kind: kind, SourceID: sourceID, TargetID: targetID, Property: property,
				TerminalID: cloneString(terminalID),
			},
		})
	}
	addEquality := func(source FacilityEntityReference, sourceID, property string, equality FacilityStateEquality, kind FacilityDependencyKind, terminalID *string) {
		add(source, FacilityEntityReference{Kind: FacilityEntityKindDevice, EntityID: equality.DeviceID}, kind, sourceID, equality.DeviceID, property+".deviceId", terminalID)
		add(source, FacilityEntityReference{Kind: FacilityEntityKindDeviceState, EntityID: equality.StateID, OwnerID: new(equality.DeviceID)}, kind, sourceID, equality.StateID, property+".stateId", terminalID)
	}
	addTransitionRequest := func(source FacilityEntityReference, sourceID, property string, request FacilityTransitionRequest, kind FacilityDependencyKind, terminalID *string) {
		add(source, FacilityEntityReference{Kind: FacilityEntityKindDevice, EntityID: request.DeviceID}, kind, sourceID, request.DeviceID, property+".deviceId", terminalID)
		add(source, FacilityEntityReference{Kind: FacilityEntityKindDeviceTransition, EntityID: request.TransitionID, OwnerID: new(request.DeviceID)}, kind, sourceID, request.TransitionID, property+".transitionId", terminalID)
	}

	for deviceIndex := range session.Facility.Devices {
		device := &session.Facility.Devices[deviceIndex]
		for transitionIndex := range device.Transitions {
			transition := &device.Transitions[transitionIndex]
			source := FacilityEntityReference{
				Kind: FacilityEntityKindDeviceTransition, EntityID: transition.ID, OwnerID: new(device.ID),
			}
			sourceID := device.ID + "/" + transition.ID
			for index, precondition := range transition.Preconditions {
				addEquality(source, sourceID, indexedProperty("preconditions", index), precondition, FacilityDependencyKindTransitionPrecondition, nil)
			}
			for index, effect := range transition.ConditionEffects {
				add(source, FacilityEntityReference{Kind: FacilityEntityKindCondition, EntityID: effect.ConditionID},
					FacilityDependencyKindTransitionConditionEffect, sourceID, effect.ConditionID,
					indexedProperty("conditionEffects", index)+".conditionId", nil)
			}
		}
	}

	for conditionIndex := range session.Facility.Conditions {
		condition := &session.Facility.Conditions[conditionIndex]
		source := FacilityEntityReference{Kind: FacilityEntityKindCondition, EntityID: condition.ID}
		if condition.Device != nil {
			add(source, FacilityEntityReference{Kind: FacilityEntityKindDevice, EntityID: condition.Device.DeviceID},
				FacilityDependencyKindDiagnosticScope, condition.ID, condition.Device.DeviceID, "device.deviceId", nil)
		} else if condition.Terminal != nil {
			terminalID := condition.Terminal.TerminalID
			add(source, FacilityEntityReference{Kind: FacilityEntityKindUnspecified, EntityID: terminalID},
				FacilityDependencyKindDiagnosticScope, condition.ID, terminalID, "terminal.terminalId", &terminalID)
		}
		for index, effect := range condition.Effects {
			property := indexedProperty("effects", index)
			switch {
			case effect.CapabilityBlock != nil:
				targetID := string(effect.CapabilityBlock.Capability)
				add(source, FacilityEntityReference{Kind: FacilityEntityKindUnspecified, EntityID: targetID},
					FacilityDependencyKindDiagnosticEffect, condition.ID, targetID, property+".capabilityBlock", nil)
			case effect.DiagnosticPath != nil:
				targetID := effect.DiagnosticPath.NodeID
				terminalID := effect.DiagnosticPath.TerminalID
				add(source, FacilityEntityReference{Kind: FacilityEntityKindUnspecified, EntityID: targetID},
					FacilityDependencyKindDiagnosticEffect, condition.ID, targetID, property+".diagnosticPath", &terminalID)
			case effect.RecordSubstitution != nil:
				targetID := effect.RecordSubstitution.BlockID
				terminalID := effect.RecordSubstitution.TerminalID
				add(source, FacilityEntityReference{Kind: FacilityEntityKindUnspecified, EntityID: targetID},
					FacilityDependencyKindDiagnosticEffect, condition.ID, targetID, property+".recordSubstitution", &terminalID)
			case effect.DisplayInstability != nil:
				targetID := string(TerminalPresentationEffectDisplayUnstable)
				add(source, FacilityEntityReference{Kind: FacilityEntityKindUnspecified, EntityID: targetID},
					FacilityDependencyKindDiagnosticEffect, condition.ID, targetID, property+".displayInstability", nil)
			}
		}
		for index, recovery := range condition.Recovery {
			property := indexedProperty("recovery", index)
			if recovery.Transition != nil {
				addTransitionRequest(source, condition.ID, property+".transition", *recovery.Transition, FacilityDependencyKindRecoveryReference, nil)
			}
			if recovery.RecoveryProgramID != nil {
				add(source, FacilityEntityReference{Kind: FacilityEntityKindRecoveryProgram, EntityID: *recovery.RecoveryProgramID},
					FacilityDependencyKindRecoveryReference, condition.ID, *recovery.RecoveryProgramID,
					property+".recoveryProgramId", nil)
			}
		}
	}

	for programIndex := range session.Facility.RecoveryPrograms {
		program := &session.Facility.RecoveryPrograms[programIndex]
		source := FacilityEntityReference{Kind: FacilityEntityKindRecoveryProgram, EntityID: program.ID}
		for index, request := range program.Transitions {
			addTransitionRequest(source, program.ID, indexedProperty("transitions", index), request,
				FacilityDependencyKindRecoveryProgramTransition, nil)
		}
	}

	for terminalIndex := range session.Terminals {
		terminal := &session.Terminals[terminalIndex]
		terminalID := terminal.ID
		var visit func(*ContentNode)
		visit = func(node *ContentNode) {
			source := FacilityEntityReference{Kind: FacilityEntityKindUnspecified, EntityID: node.ID}
			for index, variant := range node.FacilityNameVariants {
				addEquality(source, node.ID, indexedProperty("facilityNameVariants", index), variant.When,
					FacilityDependencyKindNameVariant, &terminalID)
			}
			if node.VisibleWhen != nil {
				addEquality(source, node.ID, "visibleWhen", *node.VisibleWhen, FacilityDependencyKindVisibility, &terminalID)
			}
			if node.AvailableWhen != nil {
				addEquality(source, node.ID, "availableWhen", *node.AvailableWhen, FacilityDependencyKindAvailability, &terminalID)
			}
			for blockIndex := range node.Blocks {
				block := &node.Blocks[blockIndex]
				blockSource := FacilityEntityReference{Kind: FacilityEntityKindUnspecified, EntityID: block.ID}
				for variantIndex, variant := range block.FacilityTextVariants {
					addEquality(blockSource, block.ID, indexedProperty("facilityTextVariants", variantIndex), variant.When,
						FacilityDependencyKindEntryContentVariant, &terminalID)
				}
			}
			if node.StateChange != nil && node.StateChange.FacilityAction != nil {
				action := node.StateChange.FacilityAction
				if action.Transitions != nil {
					for index, request := range action.Transitions.Transitions {
						addTransitionRequest(source, node.ID, indexedProperty("stateChange.facilityAction.transitions", index),
							request, FacilityDependencyKindCommandAction, &terminalID)
					}
				}
				if action.RecoveryProgramID != nil {
					add(source, FacilityEntityReference{Kind: FacilityEntityKindRecoveryProgram, EntityID: *action.RecoveryProgramID},
						FacilityDependencyKindCommandAction, node.ID, *action.RecoveryProgramID,
						"stateChange.facilityAction.recoveryProgramId", &terminalID)
				}
			}
			for childIndex := range node.Children {
				visit(&node.Children[childIndex])
			}
		}
		visit(&terminal.Root)
	}
	return edges
}

func facilityEntityExists(facility *Facility, target FacilityEntityReference) bool {
	if facility == nil || target.EntityID == "" {
		return false
	}
	switch target.Kind {
	case FacilityEntityKindDevice:
		if target.OwnerID != nil {
			return false
		}
		for _, device := range facility.Devices {
			if device.ID == target.EntityID {
				return true
			}
		}
	case FacilityEntityKindDeviceState, FacilityEntityKindDeviceTransition:
		if target.OwnerID == nil || *target.OwnerID == "" {
			return false
		}
		for _, device := range facility.Devices {
			if device.ID != *target.OwnerID {
				continue
			}
			if target.Kind == FacilityEntityKindDeviceState {
				for _, state := range device.States {
					if state.ID == target.EntityID {
						return true
					}
				}
			} else {
				for _, transition := range device.Transitions {
					if transition.ID == target.EntityID {
						return true
					}
				}
			}
		}
	case FacilityEntityKindCondition:
		if target.OwnerID != nil {
			return false
		}
		for _, condition := range facility.Conditions {
			if condition.ID == target.EntityID {
				return true
			}
		}
	case FacilityEntityKindRecoveryProgram:
		if target.OwnerID != nil {
			return false
		}
		for _, program := range facility.RecoveryPrograms {
			if program.ID == target.EntityID {
				return true
			}
		}
	}
	return false
}

func removedFacilityEntities(current, candidate *Facility) []FacilityEntityReference {
	if current == nil {
		return nil
	}
	entities := facilityEntities(current)
	removed := make([]FacilityEntityReference, 0)
	for _, entity := range entities {
		if !facilityEntityExists(candidate, entity) {
			removed = append(removed, entity)
		}
	}
	slices.SortFunc(removed, compareFacilityEntities)
	return removed
}

func facilityEntities(facility *Facility) []FacilityEntityReference {
	if facility == nil {
		return nil
	}
	entities := make([]FacilityEntityReference, 0)
	for _, device := range facility.Devices {
		entities = append(entities, FacilityEntityReference{Kind: FacilityEntityKindDevice, EntityID: device.ID})
		for _, state := range device.States {
			entities = append(entities, FacilityEntityReference{
				Kind: FacilityEntityKindDeviceState, EntityID: state.ID, OwnerID: new(device.ID),
			})
		}
		for _, transition := range device.Transitions {
			entities = append(entities, FacilityEntityReference{
				Kind: FacilityEntityKindDeviceTransition, EntityID: transition.ID, OwnerID: new(device.ID),
			})
		}
	}
	for _, condition := range facility.Conditions {
		entities = append(entities, FacilityEntityReference{Kind: FacilityEntityKindCondition, EntityID: condition.ID})
	}
	for _, program := range facility.RecoveryPrograms {
		entities = append(entities, FacilityEntityReference{Kind: FacilityEntityKindRecoveryProgram, EntityID: program.ID})
	}
	return entities
}

func facilityEntityHasInboundReference(facility *Facility, edges []facilityDependencyEdge, entity FacilityEntityReference) bool {
	for _, edge := range edges {
		if sameFacilityEntity(edge.target, entity) {
			return true
		}
	}
	if facility == nil || entity.Kind != FacilityEntityKindDeviceState || entity.OwnerID == nil {
		return false
	}
	for _, device := range facility.Devices {
		if device.ID != *entity.OwnerID {
			continue
		}
		if device.InitialStateID == entity.EntityID || device.CurrentStateID == entity.EntityID {
			return true
		}
		for _, transition := range device.Transitions {
			if transition.SourceStateID == entity.EntityID || transition.DestinationStateID == entity.EntityID {
				return true
			}
		}
	}
	return false
}

func sameFacilityEntity(left, right FacilityEntityReference) bool {
	if left.Kind != right.Kind || left.EntityID != right.EntityID {
		return false
	}
	if left.OwnerID == nil || right.OwnerID == nil {
		return left.OwnerID == nil && right.OwnerID == nil
	}
	return *left.OwnerID == *right.OwnerID
}

func missingFacilityEntityIssue(target FacilityEntityReference) FacilityIssue {
	entityID := target.EntityID
	issue := FacilityIssue{
		Code: FacilityFailureMissingReference, EntityKind: string(target.Kind), EntityID: &entityID,
	}
	if target.OwnerID != nil {
		referenceKind := "owner"
		ownerID := *target.OwnerID
		issue.ReferenceKind = &referenceKind
		issue.ReferenceID = &ownerID
	}
	return issue
}

func cloneFacilityEntityReference(reference FacilityEntityReference) FacilityEntityReference {
	clone := reference
	clone.OwnerID = cloneString(reference.OwnerID)
	return clone
}

func cloneFacilityDependency(dependency FacilityDependency) FacilityDependency {
	clone := dependency
	clone.TerminalID = cloneString(dependency.TerminalID)
	return clone
}

func indexedProperty(property string, index int) string {
	return property + "[" + strconv.Itoa(index) + "]"
}

func compareFacilityDependencies(left, right FacilityDependency) int {
	leftTerminal, rightTerminal := "", ""
	if left.TerminalID != nil {
		leftTerminal = *left.TerminalID
	}
	if right.TerminalID != nil {
		rightTerminal = *right.TerminalID
	}
	for _, comparison := range []int{
		cmp.Compare(string(left.Kind), string(right.Kind)),
		cmp.Compare(leftTerminal, rightTerminal),
		cmp.Compare(left.SourceID, right.SourceID),
		cmp.Compare(left.Property, right.Property),
		cmp.Compare(left.TargetID, right.TargetID),
	} {
		if comparison != 0 {
			return comparison
		}
	}
	return 0
}

func compareFacilityEntities(left, right FacilityEntityReference) int {
	leftOwner, rightOwner := "", ""
	if left.OwnerID != nil {
		leftOwner = *left.OwnerID
	}
	if right.OwnerID != nil {
		rightOwner = *right.OwnerID
	}
	for _, comparison := range []int{
		cmp.Compare(string(left.Kind), string(right.Kind)),
		cmp.Compare(leftOwner, rightOwner),
		cmp.Compare(left.EntityID, right.EntityID),
	} {
		if comparison != 0 {
			return comparison
		}
	}
	return 0
}

// FacilityDeviceStatePreview is a detached state override used only for
// projection previews.
type FacilityDeviceStatePreview struct {
	DeviceID string
	StateID  string
}

// FacilityConditionPreview is a detached condition override used only for
// projection previews.
type FacilityConditionPreview struct {
	ConditionID string
	Active      bool
}

// FacilityPreview identifies one side-effect-free preview request. Validation
// requires exactly one override.
type FacilityPreview struct {
	ExpectedFacilityRevision uint64
	TerminalID               string
	DeviceState              *FacilityDeviceStatePreview
	Condition                *FacilityConditionPreview
}

// CloneFacility returns a deeply detached copy of a facility aggregate.
func CloneFacility(facility *Facility) *Facility {
	if facility == nil {
		return nil
	}

	clone := *facility
	clone.Extra = cloneRawMessages(facility.Extra)
	if facility.Devices != nil {
		clone.Devices = make([]FacilityDevice, len(facility.Devices))
		for index := range facility.Devices {
			clone.Devices[index] = cloneFacilityDevice(facility.Devices[index])
		}
	}
	if facility.Conditions != nil {
		clone.Conditions = make([]DiagnosticCondition, len(facility.Conditions))
		for index := range facility.Conditions {
			clone.Conditions[index] = cloneDiagnosticCondition(facility.Conditions[index])
		}
	}
	if facility.RecoveryPrograms != nil {
		clone.RecoveryPrograms = make([]RecoveryProgram, len(facility.RecoveryPrograms))
		for index := range facility.RecoveryPrograms {
			clone.RecoveryPrograms[index] = cloneRecoveryProgram(facility.RecoveryPrograms[index])
		}
	}
	return &clone
}

func cloneFacilityDevice(device FacilityDevice) FacilityDevice {
	clone := device
	clone.Extra = cloneRawMessages(device.Extra)
	if device.States != nil {
		clone.States = make([]FacilityDeviceState, len(device.States))
		for index := range device.States {
			clone.States[index] = device.States[index]
			clone.States[index].Extra = cloneRawMessages(device.States[index].Extra)
		}
	}
	if device.Transitions != nil {
		clone.Transitions = make([]FacilityDeviceTransition, len(device.Transitions))
		for index := range device.Transitions {
			clone.Transitions[index] = cloneFacilityDeviceTransition(device.Transitions[index])
		}
	}
	return clone
}

func cloneFacilityDeviceTransition(transition FacilityDeviceTransition) FacilityDeviceTransition {
	clone := transition
	clone.Preconditions = cloneFacilityStateEqualities(transition.Preconditions)
	if transition.ConditionEffects != nil {
		clone.ConditionEffects = make([]FacilityConditionEffect, len(transition.ConditionEffects))
		for index := range transition.ConditionEffects {
			clone.ConditionEffects[index] = transition.ConditionEffects[index]
			clone.ConditionEffects[index].Extra = cloneRawMessages(transition.ConditionEffects[index].Extra)
		}
	}
	clone.Extra = cloneRawMessages(transition.Extra)
	return clone
}

func cloneDiagnosticCondition(condition DiagnosticCondition) DiagnosticCondition {
	clone := condition
	if condition.Device != nil {
		device := *condition.Device
		device.Extra = cloneRawMessages(condition.Device.Extra)
		clone.Device = &device
	}
	if condition.Terminal != nil {
		terminal := *condition.Terminal
		terminal.Extra = cloneRawMessages(condition.Terminal.Extra)
		clone.Terminal = &terminal
	}
	if condition.Effects != nil {
		clone.Effects = make([]DiagnosticEffect, len(condition.Effects))
		for index := range condition.Effects {
			clone.Effects[index] = cloneDiagnosticEffect(condition.Effects[index])
		}
	}
	if condition.Recovery != nil {
		clone.Recovery = make([]DiagnosticRecoveryReference, len(condition.Recovery))
		for index := range condition.Recovery {
			clone.Recovery[index] = CloneDiagnosticRecoveryReference(condition.Recovery[index])
		}
	}
	clone.Extra = cloneRawMessages(condition.Extra)
	return clone
}

func cloneDiagnosticEffect(effect DiagnosticEffect) DiagnosticEffect {
	clone := effect
	if effect.CapabilityBlock != nil {
		capabilityBlock := *effect.CapabilityBlock
		capabilityBlock.Extra = cloneRawMessages(effect.CapabilityBlock.Extra)
		clone.CapabilityBlock = &capabilityBlock
	}
	if effect.DiagnosticPath != nil {
		diagnosticPath := *effect.DiagnosticPath
		diagnosticPath.Extra = cloneRawMessages(effect.DiagnosticPath.Extra)
		clone.DiagnosticPath = &diagnosticPath
	}
	if effect.RecordSubstitution != nil {
		recordSubstitution := *effect.RecordSubstitution
		recordSubstitution.Extra = cloneRawMessages(effect.RecordSubstitution.Extra)
		clone.RecordSubstitution = &recordSubstitution
	}
	if effect.DisplayInstability != nil {
		displayInstability := *effect.DisplayInstability
		displayInstability.Extra = cloneRawMessages(effect.DisplayInstability.Extra)
		clone.DisplayInstability = &displayInstability
	}
	clone.Extra = cloneRawMessages(effect.Extra)
	return clone
}

// CloneDiagnosticRecoveryReference returns a detached recovery reference.
func CloneDiagnosticRecoveryReference(reference DiagnosticRecoveryReference) DiagnosticRecoveryReference {
	clone := reference
	if reference.Transition != nil {
		transition := *reference.Transition
		transition.Extra = cloneRawMessages(reference.Transition.Extra)
		clone.Transition = &transition
	}
	clone.RecoveryProgramID = cloneString(reference.RecoveryProgramID)
	if reference.PrivateOverseerAction != nil {
		clone.PrivateOverseerAction = new(*reference.PrivateOverseerAction)
	}
	clone.Extra = cloneRawMessages(reference.Extra)
	return clone
}

func cloneRecoveryProgram(program RecoveryProgram) RecoveryProgram {
	clone := program
	clone.Transitions = CloneFacilityTransitionRequests(program.Transitions)
	clone.Extra = cloneRawMessages(program.Extra)
	return clone
}

// CloneFacilityTransitionRequests returns a detached ordered request list.
func CloneFacilityTransitionRequests(requests []FacilityTransitionRequest) []FacilityTransitionRequest {
	if requests == nil {
		return nil
	}
	clone := slices.Clone(requests)
	for index := range requests {
		clone[index].Extra = cloneRawMessages(requests[index].Extra)
	}
	return clone
}

func cloneFacilityStateEqualities(equalities []FacilityStateEquality) []FacilityStateEquality {
	if equalities == nil {
		return nil
	}
	clone := slices.Clone(equalities)
	for index := range equalities {
		clone[index].Extra = cloneRawMessages(equalities[index].Extra)
	}
	return clone
}

// CloneFacilityActionConfig returns a deeply detached action configuration.
func CloneFacilityActionConfig(action *FacilityActionConfig) *FacilityActionConfig {
	if action == nil {
		return nil
	}
	clone := *action
	if action.Transitions != nil {
		transitions := *action.Transitions
		transitions.Transitions = CloneFacilityTransitionRequests(action.Transitions.Transitions)
		transitions.Extra = cloneRawMessages(action.Transitions.Extra)
		clone.Transitions = &transitions
	}
	clone.RecoveryProgramID = cloneString(action.RecoveryProgramID)
	clone.Extra = cloneRawMessages(action.Extra)
	return &clone
}

// CloneFacilityTextVariants returns a deeply detached ordered variant list.
func CloneFacilityTextVariants(variants []FacilityTextVariant) []FacilityTextVariant {
	if variants == nil {
		return nil
	}
	clone := slices.Clone(variants)
	for index := range variants {
		clone[index].When.Extra = cloneRawMessages(variants[index].When.Extra)
		clone[index].Extra = cloneRawMessages(variants[index].Extra)
	}
	return clone
}
