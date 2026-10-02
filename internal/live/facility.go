package live

import (
	"maps"
	"slices"
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/nav"
)

// facilityProjection is the detached result of evaluating one terminal
// against the shared facility snapshot. BlockedCapabilities is a lookup of
// explicit diagnostic denials; absence means the capability is not blocked.
type facilityProjection struct {
	Tree                domain.ContentNode
	BlockedCapabilities map[domain.FacilityCapability]bool
	Effects             []domain.TerminalPresentationEffect
}

type projectedDeviceState struct {
	current string
	valid   bool
}

type diagnosticProjection struct {
	forcedVisible       map[string]bool
	recordSubstitutions map[string]string
	blockedCapabilities map[domain.FacilityCapability]bool
	displayUnstable     bool
}

// PreviewFacility evaluates one detached device-state or condition override
// against a coordinator-owned terminal checkpoint. It never changes the
// supplied checkpoint, facility, or the service's installed live state.
func (service *Service) PreviewFacility(
	state *domain.TerminalRuntime,
	facility *domain.Facility,
	preview domain.FacilityPreview,
) (*domain.PublicLiveState, []domain.FacilityIssue) {
	if service == nil || state == nil {
		return nil, facilityPreviewIssue(domain.FacilityFailureRuntimeContextEnded, "terminal", "")
	}
	if facility == nil {
		return nil, facilityPreviewIssue(domain.FacilityFailureMissingReference, "facility", "")
	}
	if preview.ExpectedFacilityRevision != facility.Revision {
		return nil, facilityPreviewIssue(domain.FacilityFailureStaleRevision, "facility", "")
	}
	if preview.TerminalID == "" || strings.TrimSpace(preview.TerminalID) != preview.TerminalID ||
		preview.TerminalID != state.TerminalID {
		return nil, facilityPreviewIssue(domain.FacilityFailureMissingReference, "terminal", preview.TerminalID)
	}
	if (preview.DeviceState == nil) == (preview.Condition == nil) {
		return nil, facilityPreviewIssue(domain.FacilityFailureInvalidConfiguration, "preview", "")
	}

	candidate := domain.CloneFacility(facility)
	if preview.DeviceState != nil {
		if issues := applyDeviceStatePreview(candidate, *preview.DeviceState); len(issues) != 0 {
			return nil, issues
		}
	} else if issues := applyConditionPreview(candidate, *preview.Condition); len(issues) != 0 {
		return nil, issues
	}

	service.mu.RLock()
	defer service.mu.RUnlock()
	authored := state.AuthoredTree
	if authored.ID == "" {
		authored = state.Tree
	}
	projection := projectFacility(authored, state.CommandStates, candidate, state.TerminalID)
	previewRuntime := *state
	previewRuntime.AuthoredTree = domain.CloneContentNode(authored)
	previewRuntime.Tree = projection.Tree
	previewRuntime.CommandStates = cloneCommandStates(state.CommandStates)
	previewRuntime.Effects = slices.Clone(projection.Effects)
	previewRuntime.Nav = nav.Revalidate(state.Nav, previewRuntime.Tree)
	revalidateControllerPresentation(&previewRuntime)
	return publicTerminalRuntime(&previewRuntime), nil
}

func applyDeviceStatePreview(
	facility *domain.Facility,
	preview domain.FacilityDeviceStatePreview,
) []domain.FacilityIssue {
	if facility == nil || preview.DeviceID == "" || strings.TrimSpace(preview.DeviceID) != preview.DeviceID ||
		preview.StateID == "" || strings.TrimSpace(preview.StateID) != preview.StateID {
		return facilityPreviewIssue(domain.FacilityFailureInvalidConfiguration, "device-state", preview.StateID)
	}
	deviceIndex := -1
	for index := range facility.Devices {
		if facility.Devices[index].ID != preview.DeviceID {
			continue
		}
		if deviceIndex != -1 {
			return facilityPreviewIssue(domain.FacilityFailureConflict, "device", preview.DeviceID)
		}
		deviceIndex = index
	}
	if deviceIndex == -1 {
		return facilityPreviewIssue(domain.FacilityFailureMissingReference, "device", preview.DeviceID)
	}
	knownState := false
	for _, state := range facility.Devices[deviceIndex].States {
		if state.ID != preview.StateID {
			continue
		}
		if knownState {
			return facilityPreviewIssue(domain.FacilityFailureConflict, "device-state", preview.StateID)
		}
		knownState = true
	}
	if !knownState {
		return facilityPreviewIssue(domain.FacilityFailureMissingReference, "device-state", preview.StateID)
	}
	facility.Devices[deviceIndex].CurrentStateID = preview.StateID
	return nil
}

func applyConditionPreview(
	facility *domain.Facility,
	preview domain.FacilityConditionPreview,
) []domain.FacilityIssue {
	if facility == nil || preview.ConditionID == "" || strings.TrimSpace(preview.ConditionID) != preview.ConditionID {
		return facilityPreviewIssue(domain.FacilityFailureInvalidConfiguration, "condition", preview.ConditionID)
	}
	conditionIndex := -1
	for index := range facility.Conditions {
		if facility.Conditions[index].ID != preview.ConditionID {
			continue
		}
		if conditionIndex != -1 {
			return facilityPreviewIssue(domain.FacilityFailureConflict, "condition", preview.ConditionID)
		}
		conditionIndex = index
	}
	if conditionIndex == -1 {
		return facilityPreviewIssue(domain.FacilityFailureMissingReference, "condition", preview.ConditionID)
	}
	facility.Conditions[conditionIndex].CurrentActive = preview.Active
	return nil
}

func facilityPreviewIssue(
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

// projectFacility applies every deterministic presentation layer to detached
// inputs. Callers remain responsible for navigation policy over the effective
// tree and for rejecting invalid authored graphs before they become canonical.
func projectFacility(
	authored domain.ContentNode,
	completed map[string]domain.CommandExecutionState,
	facility *domain.Facility,
	terminalID string,
) facilityProjection {
	tree := domain.CloneContentNode(authored)
	deviceStates := indexProjectedDeviceStates(facility)
	completedBlocks := completedBlockTexts(completed)
	diagnostics := evaluateDiagnostics(authored, facility, terminalID)
	markDiagnosticPathAncestors(authored, diagnostics.forcedVisible)
	applyFacilityProjection(&tree, completed, completedBlocks, deviceStates, diagnostics, true)

	effects := []domain.TerminalPresentationEffect(nil)
	if diagnostics.displayUnstable {
		effects = []domain.TerminalPresentationEffect{domain.TerminalPresentationEffectDisplayUnstable}
	}
	return facilityProjection{
		Tree:                tree,
		BlockedCapabilities: maps.Clone(diagnostics.blockedCapabilities),
		Effects:             effects,
	}
}

func indexProjectedDeviceStates(facility *domain.Facility) map[string]projectedDeviceState {
	states := make(map[string]projectedDeviceState)
	if facility == nil {
		return states
	}
	for _, device := range facility.Devices {
		if _, duplicate := states[device.ID]; duplicate {
			states[device.ID] = projectedDeviceState{}
			continue
		}
		known := false
		seenStateIDs := make(map[string]struct{}, len(device.States))
		valid := device.ID != ""
		for _, state := range device.States {
			if _, duplicate := seenStateIDs[state.ID]; duplicate {
				valid = false
			}
			seenStateIDs[state.ID] = struct{}{}
			known = known || state.ID == device.CurrentStateID
		}
		states[device.ID] = projectedDeviceState{current: device.CurrentStateID, valid: valid && known}
	}
	return states
}

func completedBlockTexts(completed map[string]domain.CommandExecutionState) map[string]string {
	texts := make(map[string]string)
	conflicts := make(map[string]bool)
	for _, commandID := range slices.Sorted(maps.Keys(completed)) {
		change := completed[commandID].EntryContentChange
		if change == nil || conflicts[change.BlockID] {
			continue
		}
		if _, duplicate := texts[change.BlockID]; duplicate {
			delete(texts, change.BlockID)
			conflicts[change.BlockID] = true
			continue
		}
		texts[change.BlockID] = change.CompletedText
	}
	return texts
}

func evaluateDiagnostics(
	authored domain.ContentNode,
	facility *domain.Facility,
	terminalID string,
) diagnosticProjection {
	projection := diagnosticProjection{
		forcedVisible:       make(map[string]bool),
		recordSubstitutions: make(map[string]string),
		blockedCapabilities: make(map[domain.FacilityCapability]bool),
	}
	if facility == nil {
		return projection
	}
	referencedDevices := terminalReferencedDevices(authored, facility)
	diagnosticPathConflicts := make(map[string]bool)
	recordConflicts := make(map[string]bool)
	for _, condition := range facility.Conditions {
		if !condition.CurrentActive || (condition.Device == nil) == (condition.Terminal == nil) {
			continue
		}
		conditionApplies := diagnosticConditionApplies(referencedDevices, condition, terminalID)
		for _, effect := range condition.Effects {
			if diagnosticEffectVariantCount(effect) != 1 {
				continue
			}
			switch {
			case effect.DiagnosticPath != nil && diagnosticTargetApplies(condition, conditionApplies, effect.DiagnosticPath.TerminalID, terminalID):
				nodeID := effect.DiagnosticPath.NodeID
				if diagnosticPathConflicts[nodeID] {
					continue
				}
				if projection.forcedVisible[nodeID] {
					delete(projection.forcedVisible, nodeID)
					diagnosticPathConflicts[nodeID] = true
					continue
				}
				projection.forcedVisible[nodeID] = true
			case effect.RecordSubstitution != nil && diagnosticTargetApplies(condition, conditionApplies, effect.RecordSubstitution.TerminalID, terminalID):
				blockID := effect.RecordSubstitution.BlockID
				if recordConflicts[blockID] {
					continue
				}
				if _, duplicate := projection.recordSubstitutions[blockID]; duplicate {
					delete(projection.recordSubstitutions, blockID)
					recordConflicts[blockID] = true
					continue
				}
				projection.recordSubstitutions[blockID] = effect.RecordSubstitution.ReplacementText
			case effect.CapabilityBlock != nil && conditionApplies && validProjectedCapability(effect.CapabilityBlock.Capability):
				projection.blockedCapabilities[effect.CapabilityBlock.Capability] = true
			case effect.DisplayInstability != nil && conditionApplies:
				projection.displayUnstable = true
			}
		}
	}
	return projection
}

func diagnosticConditionApplies(
	referencedDevices map[string]bool,
	condition domain.DiagnosticCondition,
	terminalID string,
) bool {
	if condition.Terminal != nil {
		return condition.Terminal.TerminalID == terminalID
	}
	return condition.Device != nil && referencedDevices[condition.Device.DeviceID]
}

func diagnosticTargetApplies(
	condition domain.DiagnosticCondition,
	conditionApplies bool,
	targetTerminalID string,
	terminalID string,
) bool {
	if targetTerminalID != terminalID {
		return false
	}
	return condition.Device != nil || conditionApplies
}

func terminalReferencedDevices(authored domain.ContentNode, facility *domain.Facility) map[string]bool {
	referenced := make(map[string]bool)
	expandedPrograms := make(map[string][]domain.FacilityTransitionRequest)
	invalidPrograms := make(map[string]bool)
	addEquality := func(equality *domain.FacilityStateEquality) {
		if equality != nil && equality.DeviceID != "" {
			referenced[equality.DeviceID] = true
		}
	}
	addRequests := func(requests []domain.FacilityTransitionRequest) {
		for _, request := range requests {
			if request.DeviceID != "" {
				referenced[request.DeviceID] = true
			}
		}
	}

	var visit func(domain.ContentNode)
	visit = func(node domain.ContentNode) {
		addEquality(node.VisibleWhen)
		addEquality(node.AvailableWhen)
		for index := range node.FacilityNameVariants {
			addEquality(&node.FacilityNameVariants[index].When)
		}
		for blockIndex := range node.Blocks {
			for variantIndex := range node.Blocks[blockIndex].FacilityTextVariants {
				addEquality(&node.Blocks[blockIndex].FacilityTextVariants[variantIndex].When)
			}
		}
		if node.StateChange != nil && node.StateChange.FacilityAction != nil {
			action := node.StateChange.FacilityAction
			if action.Transitions != nil {
				addRequests(action.Transitions.Transitions)
			}
			if action.RecoveryProgramID != nil {
				programID := *action.RecoveryProgramID
				requests, expanded := expandedPrograms[programID]
				if !expanded && !invalidPrograms[programID] {
					var issues []domain.FacilityIssue
					requests, issues = domain.ExpandRecoveryProgram(facility, programID)
					if len(issues) != 0 {
						invalidPrograms[programID] = true
					} else {
						expandedPrograms[programID] = requests
					}
				}
				addRequests(requests)
			}
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(authored)
	return referenced
}

func diagnosticEffectVariantCount(effect domain.DiagnosticEffect) int {
	count := 0
	if effect.CapabilityBlock != nil {
		count++
	}
	if effect.DiagnosticPath != nil {
		count++
	}
	if effect.RecordSubstitution != nil {
		count++
	}
	if effect.DisplayInstability != nil {
		count++
	}
	return count
}

func validProjectedCapability(capability domain.FacilityCapability) bool {
	switch capability {
	case domain.FacilityCapabilityExecuteCommand, domain.FacilityCapabilityViewEntry,
		domain.FacilityCapabilityHack, domain.FacilityCapabilityTerminalTransition,
		domain.FacilityCapabilityRunRecoveryProgram:
		return true
	default:
		return false
	}
}

func markDiagnosticPathAncestors(node domain.ContentNode, visible map[string]bool) bool {
	containsVisiblePath := visible[node.ID]
	for _, child := range node.Children {
		containsVisiblePath = markDiagnosticPathAncestors(child, visible) || containsVisiblePath
	}
	if containsVisiblePath {
		visible[node.ID] = true
	}
	return containsVisiblePath
}

func applyFacilityProjection(
	node *domain.ContentNode,
	completed map[string]domain.CommandExecutionState,
	completedBlocks map[string]string,
	deviceStates map[string]projectedDeviceState,
	diagnostics diagnosticProjection,
	root bool,
) bool {
	if node == nil {
		return false
	}
	if !root && node.VisibleWhen != nil &&
		!facilityEqualityMatches(*node.VisibleWhen, deviceStates) && !diagnostics.forcedVisible[node.ID] {
		return false
	}

	node.Available = nil
	if state, ok := completed[node.ID]; ok && node.Type == domain.NodeCommand {
		node.Name = state.CompletedName
		node.Text = state.ResultText
	}
	if name, ok := matchingFacilityText(node.FacilityNameVariants, deviceStates); ok {
		node.Name = name
	}
	if node.AvailableWhen != nil {
		node.Available = new(facilityEqualityMatches(*node.AvailableWhen, deviceStates))
	}
	if node.Type == domain.NodeEntry && len(node.Blocks) != 0 {
		parts := make([]string, len(node.Blocks))
		for index, block := range node.Blocks {
			text := block.InitialText
			if completedText, ok := completedBlocks[block.ID]; ok {
				text = completedText
			}
			if facilityText, ok := matchingFacilityText(block.FacilityTextVariants, deviceStates); ok {
				text = facilityText
			}
			if diagnosticText, ok := diagnostics.recordSubstitutions[block.ID]; ok {
				text = diagnosticText
			}
			parts[index] = text
		}
		node.Description = strings.Join(parts, "\n\n")
	}

	if node.Children != nil {
		children := make([]domain.ContentNode, 0, len(node.Children))
		for index := range node.Children {
			if applyFacilityProjection(&node.Children[index], completed, completedBlocks, deviceStates, diagnostics, false) {
				children = append(children, node.Children[index])
			}
		}
		node.Children = children
	}
	return true
}

func matchingFacilityText(
	variants []domain.FacilityTextVariant,
	deviceStates map[string]projectedDeviceState,
) (string, bool) {
	match := ""
	matches := 0
	for _, variant := range variants {
		if !facilityEqualityMatches(variant.When, deviceStates) {
			continue
		}
		match = variant.Text
		matches++
	}
	return match, matches == 1
}

func facilityEqualityMatches(
	equality domain.FacilityStateEquality,
	deviceStates map[string]projectedDeviceState,
) bool {
	state, ok := deviceStates[equality.DeviceID]
	return ok && state.valid && equality.StateID != "" && state.current == equality.StateID
}
