package session

import (
	"fmt"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	persistencev1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/persistence/v1"
)

// SessionToProto maps only known version-1 semantics. JSON extras remain on
// the compatibility domain values and are never discarded or serialized by protobuf.
func SessionToProto(value domain.Session) (*persistencev1.Session, error) {
	if err := domain.ValidateSession(value); err != nil {
		return nil, err
	}
	result := &persistencev1.Session{Version: int32(value.Version), Name: value.Name}
	if value.PlayerConfig != "" {
		reference := value.PlayerConfig
		result.PlayerConfig = &reference
	}
	result.Terminals = make([]*persistencev1.Terminal, 0, len(value.Terminals))
	for _, terminal := range value.Terminals {
		root, err := contentNodeToProto(terminal.Root)
		if err != nil {
			return nil, err
		}
		mapped := &persistencev1.Terminal{
			Id: terminal.ID, Name: terminal.Name, HackLevel: int32(terminal.HackLevel), IntroText: terminal.IntroText, Root: root,
		}
		if len(terminal.CommandStates) != 0 {
			mapped.CommandStates = make(map[string]*persistencev1.CommandExecutionState, len(terminal.CommandStates))
			for commandID, state := range terminal.CommandStates {
				mapped.CommandStates[commandID] = &persistencev1.CommandExecutionState{
					CompletedName:      state.CompletedName,
					ResultText:         state.ResultText,
					EntryContentChange: entryContentChangeToProto(state.EntryContentChange),
				}
			}
		}
		result.Terminals = append(result.Terminals, mapped)
	}
	result.TerminalGroups = make([]*persistencev1.TerminalGroup, 0, len(value.TerminalGroups))
	for _, group := range value.TerminalGroups {
		result.TerminalGroups = append(result.TerminalGroups, &persistencev1.TerminalGroup{
			Id: group.ID, Name: group.Name, TerminalIds: append([]string(nil), group.TerminalIDs...),
		})
	}
	if value.Facility != nil {
		facility, err := facilityToProto(value.Facility)
		if err != nil {
			return nil, err
		}
		result.Facility = facility
	}
	return result, nil
}

// SessionFromProto restores known fields and merges the caller-owned extras
// from template at the session, terminal, and recursive node levels.
func SessionFromProto(value *persistencev1.Session, template domain.Session) (domain.Session, error) {
	if value == nil {
		return domain.Session{}, fmt.Errorf("session contract is required")
	}
	template = domain.CloneSession(template)
	result := domain.Session{
		Version: int(value.GetVersion()), Name: value.GetName(), PlayerConfig: value.GetPlayerConfig(), Extra: template.Extra,
		Terminals: make([]domain.Terminal, 0, len(value.GetTerminals())),
	}
	if len(value.GetTerminalGroups()) != 0 {
		result.TerminalGroups = make([]domain.TerminalGroup, 0, len(value.GetTerminalGroups()))
	}
	for index, terminal := range value.GetTerminals() {
		var terminalTemplate domain.Terminal
		if index < len(template.Terminals) {
			terminalTemplate = template.Terminals[index]
		}
		root, err := contentNodeFromProto(terminal.GetRoot(), terminalTemplate.Root)
		if err != nil {
			return domain.Session{}, err
		}
		mapped := domain.Terminal{
			ID: terminal.GetId(), Name: terminal.GetName(), HackLevel: int(terminal.GetHackLevel()), IntroText: terminal.GetIntroText(), Root: root, Extra: terminalTemplate.Extra,
		}
		if len(terminal.GetCommandStates()) != 0 {
			mapped.CommandStates = make(map[string]domain.CommandExecutionState, len(terminal.GetCommandStates()))
			for commandID, state := range terminal.GetCommandStates() {
				mapped.CommandStates[commandID] = domain.CommandExecutionState{
					CompletedName:      state.GetCompletedName(),
					ResultText:         state.GetResultText(),
					EntryContentChange: entryContentChangeFromProto(state.GetEntryContentChange()),
				}
			}
		}
		result.Terminals = append(result.Terminals, mapped)
	}
	for _, group := range value.GetTerminalGroups() {
		result.TerminalGroups = append(result.TerminalGroups, domain.TerminalGroup{
			ID: group.GetId(), Name: group.GetName(), TerminalIDs: append([]string(nil), group.GetTerminalIds()...),
		})
	}
	if value.GetFacility() != nil {
		facility, err := facilityFromProto(value.GetFacility(), template.Facility)
		if err != nil {
			return domain.Session{}, err
		}
		result.Facility = facility
	}
	if err := domain.ValidateSession(result); err != nil {
		return domain.Session{}, err
	}
	return result, nil
}

func verifySessionContract(value domain.Session) error {
	semantic, err := SessionToProto(value)
	if err != nil {
		return err
	}
	_, err = SessionFromProto(semantic, value)
	return err
}

// ContentNodeToProto maps one already-validated authored tree without
// inventing a partial session around it. Cross-terminal links are resolved by
// the complete session validator at the application boundary, not by this
// shape-preserving private-contract adapter.
func ContentNodeToProto(node domain.ContentNode) (*persistencev1.ContentNode, error) {
	return contentNodeToProto(node)
}

// ContentNodeFromProto maps one authored tree while preserving JSON-only
// extension fields from its native template. It deliberately performs no
// session-wide reference validation because the surrounding terminal catalog
// is not part of this private bridge message.
func ContentNodeFromProto(node *persistencev1.ContentNode, template domain.ContentNode) (domain.ContentNode, error) {
	return contentNodeFromProto(node, domain.CloneContentNode(template))
}

func contentNodeToProto(node domain.ContentNode) (*persistencev1.ContentNode, error) {
	result := &persistencev1.ContentNode{Id: node.ID, Name: node.Name}
	result.FacilityNameVariants = facilityTextVariantsToProto(node.FacilityNameVariants)
	result.VisibleWhen = facilityStateEqualityToProto(node.VisibleWhen)
	result.AvailableWhen = facilityStateEqualityToProto(node.AvailableWhen)
	switch node.Type {
	case domain.NodeFolder:
		folder := &persistencev1.FolderContent{Children: make([]*persistencev1.ContentNode, 0, len(node.Children))}
		for _, child := range node.Children {
			mapped, err := contentNodeToProto(child)
			if err != nil {
				return nil, err
			}
			folder.Children = append(folder.Children, mapped)
		}
		result.Content = &persistencev1.ContentNode_Folder{Folder: folder}
	case domain.NodeCommand:
		command := &persistencev1.CommandContent{Text: node.Text}
		switch node.Behavior() {
		case domain.CommandBehaviorOrdinary:
			// An unset protobuf oneof is the ordinary-command variant.
		case domain.CommandBehaviorStateChange:
			command.Behavior = &persistencev1.CommandContent_StateChange{StateChange: &persistencev1.StateChangeConfig{
				CompletedName:      node.StateChange.CompletedName,
				ConfirmationText:   node.StateChange.ConfirmationText,
				EntryContentChange: entryContentChangeToProto(node.StateChange.EntryContentChange),
				FacilityAction:     facilityActionToProto(node.StateChange.FacilityAction),
			}}
		case domain.CommandBehaviorTerminalTransition:
			command.Behavior = &persistencev1.CommandContent_TerminalTransition{TerminalTransition: &persistencev1.TerminalTransitionConfig{
				TargetTerminalId: node.TerminalTransition.TargetTerminalID,
			}}
		case domain.CommandBehaviorInvalid:
			return nil, fmt.Errorf("command %q cannot contain both stateChange and terminalTransition", node.ID)
		default:
			return nil, fmt.Errorf("command %q has unsupported behavior %q", node.ID, node.Behavior())
		}
		result.Content = &persistencev1.ContentNode_Command{Command: command}
	case domain.NodeEntry:
		entry := &persistencev1.EntryContent{Description: node.Description}
		if len(node.Blocks) != 0 {
			entry.Blocks = make([]*persistencev1.EntryContentBlock, len(node.Blocks))
			for index, block := range node.Blocks {
				entry.Blocks[index] = &persistencev1.EntryContentBlock{
					Id:                   block.ID,
					InitialText:          block.InitialText,
					FacilityTextVariants: facilityTextVariantsToProto(block.FacilityTextVariants),
				}
			}
		}
		result.Content = &persistencev1.ContentNode_Entry{Entry: entry}
	default:
		return nil, fmt.Errorf("unsupported content node type %q", node.Type)
	}
	return result, nil
}

func contentNodeFromProto(node *persistencev1.ContentNode, template domain.ContentNode) (domain.ContentNode, error) {
	if node == nil {
		return domain.ContentNode{}, fmt.Errorf("content node is required")
	}
	result := domain.ContentNode{
		ID:                   node.GetId(),
		Name:                 node.GetName(),
		FacilityNameVariants: facilityTextVariantsFromProto(node.GetFacilityNameVariants(), template.FacilityNameVariants),
		VisibleWhen:          facilityStateEqualityFromProto(node.GetVisibleWhen(), template.VisibleWhen),
		AvailableWhen:        facilityStateEqualityFromProto(node.GetAvailableWhen(), template.AvailableWhen),
		Extra:                template.Extra,
	}
	switch content := node.Content.(type) {
	case *persistencev1.ContentNode_Folder:
		result.Type = domain.NodeFolder
		result.Children = make([]domain.ContentNode, 0, len(content.Folder.GetChildren()))
		for index, child := range content.Folder.GetChildren() {
			var childTemplate domain.ContentNode
			if index < len(template.Children) {
				childTemplate = template.Children[index]
			}
			mapped, err := contentNodeFromProto(child, childTemplate)
			if err != nil {
				return domain.ContentNode{}, err
			}
			result.Children = append(result.Children, mapped)
		}
	case *persistencev1.ContentNode_Command:
		result.Type, result.Text = domain.NodeCommand, content.Command.GetText()
		switch behavior := content.Command.GetBehavior().(type) {
		case nil:
			// An unset protobuf oneof is the ordinary-command variant.
		case *persistencev1.CommandContent_StateChange:
			var actionTemplate *domain.FacilityActionConfig
			if template.StateChange != nil {
				actionTemplate = template.StateChange.FacilityAction
			}
			result.StateChange = &domain.StateChangeConfig{
				CompletedName:      behavior.StateChange.GetCompletedName(),
				ConfirmationText:   behavior.StateChange.GetConfirmationText(),
				EntryContentChange: entryContentChangeFromProto(behavior.StateChange.GetEntryContentChange()),
				FacilityAction:     facilityActionFromProto(behavior.StateChange.GetFacilityAction(), actionTemplate),
			}
		case *persistencev1.CommandContent_TerminalTransition:
			result.TerminalTransition = &domain.TerminalTransitionConfig{
				TargetTerminalID: behavior.TerminalTransition.GetTargetTerminalId(),
			}
		default:
			return domain.ContentNode{}, fmt.Errorf("command %q has unsupported protobuf behavior %T", node.GetId(), behavior)
		}
	case *persistencev1.ContentNode_Entry:
		result.Type, result.Description = domain.NodeEntry, content.Entry.GetDescription()
		if len(content.Entry.GetBlocks()) != 0 {
			result.Blocks = make([]domain.EntryContentBlock, len(content.Entry.GetBlocks()))
			for index, block := range content.Entry.GetBlocks() {
				var blockTemplate domain.EntryContentBlock
				if index < len(template.Blocks) {
					blockTemplate = template.Blocks[index]
				}
				result.Blocks[index] = domain.EntryContentBlock{
					ID:                   block.GetId(),
					InitialText:          block.GetInitialText(),
					FacilityTextVariants: facilityTextVariantsFromProto(block.GetFacilityTextVariants(), blockTemplate.FacilityTextVariants),
					Extra:                blockTemplate.Extra,
				}
			}
		}
	default:
		return domain.ContentNode{}, fmt.Errorf("content node variant is required")
	}
	return result, nil
}

func entryContentChangeToProto(change *domain.EntryContentChange) *persistencev1.EntryContentChange {
	if change == nil {
		return nil
	}
	return &persistencev1.EntryContentChange{
		BlockId:       change.BlockID,
		CompletedText: change.CompletedText,
	}
}

func entryContentChangeFromProto(change *persistencev1.EntryContentChange) *domain.EntryContentChange {
	if change == nil {
		return nil
	}
	return &domain.EntryContentChange{
		BlockID:       change.GetBlockId(),
		CompletedText: change.GetCompletedText(),
	}
}

func facilityToProto(facility *domain.Facility) (*persistencev1.Facility, error) {
	result := &persistencev1.Facility{Revision: facility.Revision}
	if facility.Devices != nil {
		result.Devices = make([]*persistencev1.FacilityDevice, len(facility.Devices))
		for index, device := range facility.Devices {
			mapped, err := facilityDeviceToProto(device)
			if err != nil {
				return nil, err
			}
			result.Devices[index] = mapped
		}
	}
	if facility.Conditions != nil {
		result.Conditions = make([]*persistencev1.DiagnosticCondition, len(facility.Conditions))
		for index, condition := range facility.Conditions {
			mapped, err := diagnosticConditionToProto(condition)
			if err != nil {
				return nil, err
			}
			result.Conditions[index] = mapped
		}
	}
	if facility.RecoveryPrograms != nil {
		result.RecoveryPrograms = make([]*persistencev1.RecoveryProgram, len(facility.RecoveryPrograms))
		for index, program := range facility.RecoveryPrograms {
			result.RecoveryPrograms[index] = &persistencev1.RecoveryProgram{
				Id: program.ID, Name: program.Name, Transitions: facilityTransitionRequestsToProto(program.Transitions),
			}
		}
	}
	return result, nil
}

func facilityDeviceToProto(device domain.FacilityDevice) (*persistencev1.FacilityDevice, error) {
	kind, err := facilityDeviceKindToProto(device.Kind)
	if err != nil {
		return nil, fmt.Errorf("device %q: %w", device.ID, err)
	}
	result := &persistencev1.FacilityDevice{
		Id: device.ID, Name: device.Name, Kind: kind,
		InitialStateId: device.InitialStateID, CurrentStateId: device.CurrentStateID,
	}
	if device.CustomKind != "" {
		result.CustomKind = new(device.CustomKind)
	}
	if device.States != nil {
		result.States = make([]*persistencev1.FacilityDeviceState, len(device.States))
		for index, state := range device.States {
			result.States[index] = &persistencev1.FacilityDeviceState{Id: state.ID, Name: state.Name}
		}
	}
	if device.Transitions != nil {
		result.Transitions = make([]*persistencev1.FacilityDeviceTransition, len(device.Transitions))
		for index, transition := range device.Transitions {
			result.Transitions[index] = &persistencev1.FacilityDeviceTransition{
				Id: transition.ID, Name: transition.Name,
				SourceStateId: transition.SourceStateID, DestinationStateId: transition.DestinationStateID,
				Preconditions:    facilityStateEqualitiesToProto(transition.Preconditions),
				ConditionEffects: facilityConditionEffectsToProto(transition.ConditionEffects),
				Recovery:         transition.Recovery,
			}
		}
	}
	return result, nil
}

func diagnosticConditionToProto(condition domain.DiagnosticCondition) (*persistencev1.DiagnosticCondition, error) {
	category, err := diagnosticConditionCategoryToProto(condition.Category)
	if err != nil {
		return nil, fmt.Errorf("condition %q: %w", condition.ID, err)
	}
	result := &persistencev1.DiagnosticCondition{
		Id: condition.ID, Name: condition.Name, Category: category,
		InitialActive: condition.InitialActive, CurrentActive: condition.CurrentActive,
	}
	if condition.CustomCategory != "" {
		result.CustomCategory = new(condition.CustomCategory)
	}
	switch {
	case condition.Device != nil:
		result.Scope = &persistencev1.DiagnosticCondition_Device{Device: &persistencev1.DiagnosticDeviceScope{
			DeviceId: condition.Device.DeviceID,
		}}
	case condition.Terminal != nil:
		result.Scope = &persistencev1.DiagnosticCondition_Terminal{Terminal: &persistencev1.DiagnosticTerminalScope{
			TerminalId: condition.Terminal.TerminalID,
		}}
	default:
		return nil, fmt.Errorf("condition %q has no scope", condition.ID)
	}
	if condition.Effects != nil {
		result.Effects = make([]*persistencev1.DiagnosticEffect, len(condition.Effects))
		for index, effect := range condition.Effects {
			mapped, err := diagnosticEffectToProto(effect)
			if err != nil {
				return nil, fmt.Errorf("condition %q effect %d: %w", condition.ID, index, err)
			}
			result.Effects[index] = mapped
		}
	}
	if condition.Recovery != nil {
		result.Recovery = make([]*persistencev1.DiagnosticRecoveryReference, len(condition.Recovery))
		for index, reference := range condition.Recovery {
			mapped, err := diagnosticRecoveryToProto(reference)
			if err != nil {
				return nil, fmt.Errorf("condition %q recovery %d: %w", condition.ID, index, err)
			}
			result.Recovery[index] = mapped
		}
	}
	return result, nil
}

func diagnosticEffectToProto(effect domain.DiagnosticEffect) (*persistencev1.DiagnosticEffect, error) {
	result := &persistencev1.DiagnosticEffect{}
	switch {
	case effect.CapabilityBlock != nil:
		capability, err := facilityCapabilityToProto(effect.CapabilityBlock.Capability)
		if err != nil {
			return nil, err
		}
		result.Effect = &persistencev1.DiagnosticEffect_CapabilityBlock{CapabilityBlock: &persistencev1.CapabilityBlockEffect{
			Capability: capability,
		}}
	case effect.DiagnosticPath != nil:
		result.Effect = &persistencev1.DiagnosticEffect_DiagnosticPath{DiagnosticPath: &persistencev1.DiagnosticPathEffect{
			TerminalId: effect.DiagnosticPath.TerminalID, NodeId: effect.DiagnosticPath.NodeID,
		}}
	case effect.RecordSubstitution != nil:
		result.Effect = &persistencev1.DiagnosticEffect_RecordSubstitution{RecordSubstitution: &persistencev1.RecordSubstitutionEffect{
			TerminalId: effect.RecordSubstitution.TerminalID, BlockId: effect.RecordSubstitution.BlockID,
			ReplacementText: effect.RecordSubstitution.ReplacementText,
		}}
	case effect.DisplayInstability != nil:
		result.Effect = &persistencev1.DiagnosticEffect_DisplayInstability{
			DisplayInstability: &persistencev1.DisplayInstabilityEffect{},
		}
	default:
		return nil, fmt.Errorf("diagnostic effect variant is required")
	}
	return result, nil
}

func diagnosticRecoveryToProto(reference domain.DiagnosticRecoveryReference) (*persistencev1.DiagnosticRecoveryReference, error) {
	result := &persistencev1.DiagnosticRecoveryReference{}
	switch {
	case reference.Transition != nil:
		result.Recovery = &persistencev1.DiagnosticRecoveryReference_Transition{
			Transition: facilityTransitionRequestToProto(*reference.Transition),
		}
	case reference.RecoveryProgramID != nil:
		result.Recovery = &persistencev1.DiagnosticRecoveryReference_RecoveryProgramId{
			RecoveryProgramId: *reference.RecoveryProgramID,
		}
	case reference.PrivateOverseerAction != nil:
		result.Recovery = &persistencev1.DiagnosticRecoveryReference_PrivateOverseerAction{
			PrivateOverseerAction: *reference.PrivateOverseerAction,
		}
	default:
		return nil, fmt.Errorf("diagnostic recovery variant is required")
	}
	return result, nil
}

func facilityActionToProto(action *domain.FacilityActionConfig) *persistencev1.FacilityActionConfig {
	if action == nil {
		return nil
	}
	result := &persistencev1.FacilityActionConfig{}
	switch {
	case action.Transitions != nil:
		result.Action = &persistencev1.FacilityActionConfig_Transitions{Transitions: &persistencev1.FacilityTransitionList{
			Transitions: facilityTransitionRequestsToProto(action.Transitions.Transitions),
		}}
	case action.RecoveryProgramID != nil:
		result.Action = &persistencev1.FacilityActionConfig_RecoveryProgramId{RecoveryProgramId: *action.RecoveryProgramID}
	}
	return result
}

func facilityStateEqualitiesToProto(equalities []domain.FacilityStateEquality) []*persistencev1.FacilityStateEquality {
	if equalities == nil {
		return nil
	}
	result := make([]*persistencev1.FacilityStateEquality, len(equalities))
	for index, equality := range equalities {
		result[index] = &persistencev1.FacilityStateEquality{DeviceId: equality.DeviceID, StateId: equality.StateID}
	}
	return result
}

func facilityStateEqualityToProto(equality *domain.FacilityStateEquality) *persistencev1.FacilityStateEquality {
	if equality == nil {
		return nil
	}
	return &persistencev1.FacilityStateEquality{DeviceId: equality.DeviceID, StateId: equality.StateID}
}

func facilityConditionEffectsToProto(effects []domain.FacilityConditionEffect) []*persistencev1.FacilityConditionEffect {
	if effects == nil {
		return nil
	}
	result := make([]*persistencev1.FacilityConditionEffect, len(effects))
	for index, effect := range effects {
		result[index] = &persistencev1.FacilityConditionEffect{ConditionId: effect.ConditionID, Active: effect.Active}
	}
	return result
}

func facilityTransitionRequestsToProto(requests []domain.FacilityTransitionRequest) []*persistencev1.FacilityTransitionRequest {
	if requests == nil {
		return nil
	}
	result := make([]*persistencev1.FacilityTransitionRequest, len(requests))
	for index, request := range requests {
		result[index] = facilityTransitionRequestToProto(request)
	}
	return result
}

func facilityTransitionRequestToProto(request domain.FacilityTransitionRequest) *persistencev1.FacilityTransitionRequest {
	return &persistencev1.FacilityTransitionRequest{DeviceId: request.DeviceID, TransitionId: request.TransitionID}
}

func facilityTextVariantsToProto(variants []domain.FacilityTextVariant) []*persistencev1.FacilityTextVariant {
	if variants == nil {
		return nil
	}
	result := make([]*persistencev1.FacilityTextVariant, len(variants))
	for index, variant := range variants {
		result[index] = &persistencev1.FacilityTextVariant{
			When: &persistencev1.FacilityStateEquality{DeviceId: variant.When.DeviceID, StateId: variant.When.StateID},
			Text: variant.Text,
		}
	}
	return result
}

func facilityFromProto(value *persistencev1.Facility, template *domain.Facility) (*domain.Facility, error) {
	result := &domain.Facility{Revision: value.GetRevision()}
	if template != nil {
		result.Extra = template.Extra
	}
	if value.GetDevices() != nil {
		result.Devices = make([]domain.FacilityDevice, len(value.GetDevices()))
		for index, device := range value.GetDevices() {
			var deviceTemplate domain.FacilityDevice
			if template != nil && index < len(template.Devices) {
				deviceTemplate = template.Devices[index]
			}
			mapped, err := facilityDeviceFromProto(device, deviceTemplate)
			if err != nil {
				return nil, err
			}
			result.Devices[index] = mapped
		}
	}
	if value.GetConditions() != nil {
		result.Conditions = make([]domain.DiagnosticCondition, len(value.GetConditions()))
		for index, condition := range value.GetConditions() {
			var conditionTemplate domain.DiagnosticCondition
			if template != nil && index < len(template.Conditions) {
				conditionTemplate = template.Conditions[index]
			}
			mapped, err := diagnosticConditionFromProto(condition, conditionTemplate)
			if err != nil {
				return nil, err
			}
			result.Conditions[index] = mapped
		}
	}
	if value.GetRecoveryPrograms() != nil {
		result.RecoveryPrograms = make([]domain.RecoveryProgram, len(value.GetRecoveryPrograms()))
		for index, program := range value.GetRecoveryPrograms() {
			var programTemplate domain.RecoveryProgram
			if template != nil && index < len(template.RecoveryPrograms) {
				programTemplate = template.RecoveryPrograms[index]
			}
			result.RecoveryPrograms[index] = domain.RecoveryProgram{
				ID: program.GetId(), Name: program.GetName(),
				Transitions: facilityTransitionRequestsFromProto(program.GetTransitions(), programTemplate.Transitions),
				Extra:       programTemplate.Extra,
			}
		}
	}
	return result, nil
}

func facilityDeviceFromProto(value *persistencev1.FacilityDevice, template domain.FacilityDevice) (domain.FacilityDevice, error) {
	kind, err := facilityDeviceKindFromProto(value.GetKind())
	if err != nil {
		return domain.FacilityDevice{}, fmt.Errorf("device %q: %w", value.GetId(), err)
	}
	result := domain.FacilityDevice{
		ID: value.GetId(), Name: value.GetName(), Kind: kind, CustomKind: value.GetCustomKind(),
		InitialStateID: value.GetInitialStateId(), CurrentStateID: value.GetCurrentStateId(), Extra: template.Extra,
	}
	if value.GetStates() != nil {
		result.States = make([]domain.FacilityDeviceState, len(value.GetStates()))
		for index, state := range value.GetStates() {
			var stateTemplate domain.FacilityDeviceState
			if index < len(template.States) {
				stateTemplate = template.States[index]
			}
			result.States[index] = domain.FacilityDeviceState{
				ID: state.GetId(), Name: state.GetName(), Extra: stateTemplate.Extra,
			}
		}
	}
	if value.GetTransitions() != nil {
		result.Transitions = make([]domain.FacilityDeviceTransition, len(value.GetTransitions()))
		for index, transition := range value.GetTransitions() {
			var transitionTemplate domain.FacilityDeviceTransition
			if index < len(template.Transitions) {
				transitionTemplate = template.Transitions[index]
			}
			result.Transitions[index] = domain.FacilityDeviceTransition{
				ID: transition.GetId(), Name: transition.GetName(),
				SourceStateID: transition.GetSourceStateId(), DestinationStateID: transition.GetDestinationStateId(),
				Preconditions: facilityStateEqualitiesFromProto(transition.GetPreconditions(), transitionTemplate.Preconditions),
				ConditionEffects: facilityConditionEffectsFromProto(
					transition.GetConditionEffects(), transitionTemplate.ConditionEffects,
				),
				Recovery: transition.GetRecovery(), Extra: transitionTemplate.Extra,
			}
		}
	}
	return result, nil
}

func diagnosticConditionFromProto(value *persistencev1.DiagnosticCondition, template domain.DiagnosticCondition) (domain.DiagnosticCondition, error) {
	category, err := diagnosticConditionCategoryFromProto(value.GetCategory())
	if err != nil {
		return domain.DiagnosticCondition{}, fmt.Errorf("condition %q: %w", value.GetId(), err)
	}
	result := domain.DiagnosticCondition{
		ID: value.GetId(), Name: value.GetName(), Category: category, CustomCategory: value.GetCustomCategory(),
		InitialActive: value.GetInitialActive(), CurrentActive: value.GetCurrentActive(), Extra: template.Extra,
	}
	switch scope := value.GetScope().(type) {
	case *persistencev1.DiagnosticCondition_Device:
		result.Device = &domain.DiagnosticDeviceScope{DeviceID: scope.Device.GetDeviceId()}
		if template.Device != nil {
			result.Device.Extra = template.Device.Extra
		}
	case *persistencev1.DiagnosticCondition_Terminal:
		result.Terminal = &domain.DiagnosticTerminalScope{TerminalID: scope.Terminal.GetTerminalId()}
		if template.Terminal != nil {
			result.Terminal.Extra = template.Terminal.Extra
		}
	default:
		return domain.DiagnosticCondition{}, fmt.Errorf("condition %q scope is required", value.GetId())
	}
	if value.GetEffects() != nil {
		result.Effects = make([]domain.DiagnosticEffect, len(value.GetEffects()))
		for index, effect := range value.GetEffects() {
			var effectTemplate domain.DiagnosticEffect
			if index < len(template.Effects) {
				effectTemplate = template.Effects[index]
			}
			mapped, err := diagnosticEffectFromProto(effect, effectTemplate)
			if err != nil {
				return domain.DiagnosticCondition{}, fmt.Errorf("condition %q effect %d: %w", value.GetId(), index, err)
			}
			result.Effects[index] = mapped
		}
	}
	if value.GetRecovery() != nil {
		result.Recovery = make([]domain.DiagnosticRecoveryReference, len(value.GetRecovery()))
		for index, reference := range value.GetRecovery() {
			var recoveryTemplate domain.DiagnosticRecoveryReference
			if index < len(template.Recovery) {
				recoveryTemplate = template.Recovery[index]
			}
			mapped, err := diagnosticRecoveryFromProto(reference, recoveryTemplate)
			if err != nil {
				return domain.DiagnosticCondition{}, fmt.Errorf("condition %q recovery %d: %w", value.GetId(), index, err)
			}
			result.Recovery[index] = mapped
		}
	}
	return result, nil
}

func diagnosticEffectFromProto(value *persistencev1.DiagnosticEffect, template domain.DiagnosticEffect) (domain.DiagnosticEffect, error) {
	result := domain.DiagnosticEffect{Extra: template.Extra}
	switch effect := value.GetEffect().(type) {
	case *persistencev1.DiagnosticEffect_CapabilityBlock:
		capability, err := facilityCapabilityFromProto(effect.CapabilityBlock.GetCapability())
		if err != nil {
			return domain.DiagnosticEffect{}, err
		}
		result.CapabilityBlock = &domain.CapabilityBlockEffect{Capability: capability}
		if template.CapabilityBlock != nil {
			result.CapabilityBlock.Extra = template.CapabilityBlock.Extra
		}
	case *persistencev1.DiagnosticEffect_DiagnosticPath:
		result.DiagnosticPath = &domain.DiagnosticPathEffect{
			TerminalID: effect.DiagnosticPath.GetTerminalId(), NodeID: effect.DiagnosticPath.GetNodeId(),
		}
		if template.DiagnosticPath != nil {
			result.DiagnosticPath.Extra = template.DiagnosticPath.Extra
		}
	case *persistencev1.DiagnosticEffect_RecordSubstitution:
		result.RecordSubstitution = &domain.RecordSubstitutionEffect{
			TerminalID: effect.RecordSubstitution.GetTerminalId(), BlockID: effect.RecordSubstitution.GetBlockId(),
			ReplacementText: effect.RecordSubstitution.GetReplacementText(),
		}
		if template.RecordSubstitution != nil {
			result.RecordSubstitution.Extra = template.RecordSubstitution.Extra
		}
	case *persistencev1.DiagnosticEffect_DisplayInstability:
		result.DisplayInstability = &domain.DisplayInstabilityEffect{}
		if template.DisplayInstability != nil {
			result.DisplayInstability.Extra = template.DisplayInstability.Extra
		}
	default:
		return domain.DiagnosticEffect{}, fmt.Errorf("diagnostic effect variant is required")
	}
	return result, nil
}

func diagnosticRecoveryFromProto(value *persistencev1.DiagnosticRecoveryReference, template domain.DiagnosticRecoveryReference) (domain.DiagnosticRecoveryReference, error) {
	result := domain.DiagnosticRecoveryReference{Extra: template.Extra}
	switch recovery := value.GetRecovery().(type) {
	case *persistencev1.DiagnosticRecoveryReference_Transition:
		transitionTemplate := domain.FacilityTransitionRequest{}
		if template.Transition != nil {
			transitionTemplate = *template.Transition
		}
		transition := facilityTransitionRequestFromProto(recovery.Transition, transitionTemplate)
		result.Transition = &transition
	case *persistencev1.DiagnosticRecoveryReference_RecoveryProgramId:
		result.RecoveryProgramID = new(recovery.RecoveryProgramId)
	case *persistencev1.DiagnosticRecoveryReference_PrivateOverseerAction:
		result.PrivateOverseerAction = new(recovery.PrivateOverseerAction)
	default:
		return domain.DiagnosticRecoveryReference{}, fmt.Errorf("diagnostic recovery variant is required")
	}
	return result, nil
}

func facilityActionFromProto(value *persistencev1.FacilityActionConfig, template *domain.FacilityActionConfig) *domain.FacilityActionConfig {
	if value == nil {
		return nil
	}
	result := &domain.FacilityActionConfig{}
	if template != nil {
		result.Extra = template.Extra
	}
	switch action := value.GetAction().(type) {
	case *persistencev1.FacilityActionConfig_Transitions:
		result.Transitions = &domain.FacilityTransitionList{}
		var requestTemplates []domain.FacilityTransitionRequest
		if template != nil && template.Transitions != nil {
			result.Transitions.Extra = template.Transitions.Extra
			requestTemplates = template.Transitions.Transitions
		}
		result.Transitions.Transitions = facilityTransitionRequestsFromProto(action.Transitions.GetTransitions(), requestTemplates)
	case *persistencev1.FacilityActionConfig_RecoveryProgramId:
		result.RecoveryProgramID = new(action.RecoveryProgramId)
	}
	return result
}

func facilityStateEqualitiesFromProto(values []*persistencev1.FacilityStateEquality, templates []domain.FacilityStateEquality) []domain.FacilityStateEquality {
	if values == nil {
		return nil
	}
	result := make([]domain.FacilityStateEquality, len(values))
	for index, value := range values {
		result[index] = domain.FacilityStateEquality{DeviceID: value.GetDeviceId(), StateID: value.GetStateId()}
		if index < len(templates) {
			result[index].Extra = templates[index].Extra
		}
	}
	return result
}

func facilityStateEqualityFromProto(value *persistencev1.FacilityStateEquality, template *domain.FacilityStateEquality) *domain.FacilityStateEquality {
	if value == nil {
		return nil
	}
	result := &domain.FacilityStateEquality{DeviceID: value.GetDeviceId(), StateID: value.GetStateId()}
	if template != nil {
		result.Extra = template.Extra
	}
	return result
}

func facilityConditionEffectsFromProto(values []*persistencev1.FacilityConditionEffect, templates []domain.FacilityConditionEffect) []domain.FacilityConditionEffect {
	if values == nil {
		return nil
	}
	result := make([]domain.FacilityConditionEffect, len(values))
	for index, value := range values {
		result[index] = domain.FacilityConditionEffect{ConditionID: value.GetConditionId(), Active: value.GetActive()}
		if index < len(templates) {
			result[index].Extra = templates[index].Extra
		}
	}
	return result
}

func facilityTransitionRequestsFromProto(values []*persistencev1.FacilityTransitionRequest, templates []domain.FacilityTransitionRequest) []domain.FacilityTransitionRequest {
	if values == nil {
		return nil
	}
	result := make([]domain.FacilityTransitionRequest, len(values))
	for index, value := range values {
		var template domain.FacilityTransitionRequest
		if index < len(templates) {
			template = templates[index]
		}
		result[index] = facilityTransitionRequestFromProto(value, template)
	}
	return result
}

func facilityTransitionRequestFromProto(value *persistencev1.FacilityTransitionRequest, template domain.FacilityTransitionRequest) domain.FacilityTransitionRequest {
	return domain.FacilityTransitionRequest{
		DeviceID: value.GetDeviceId(), TransitionID: value.GetTransitionId(), Extra: template.Extra,
	}
}

func facilityTextVariantsFromProto(values []*persistencev1.FacilityTextVariant, templates []domain.FacilityTextVariant) []domain.FacilityTextVariant {
	if values == nil {
		return nil
	}
	result := make([]domain.FacilityTextVariant, len(values))
	for index, value := range values {
		var template domain.FacilityTextVariant
		if index < len(templates) {
			template = templates[index]
		}
		when := value.GetWhen()
		result[index] = domain.FacilityTextVariant{
			When: domain.FacilityStateEquality{
				DeviceID: when.GetDeviceId(), StateID: when.GetStateId(), Extra: template.When.Extra,
			},
			Text: value.GetText(), Extra: template.Extra,
		}
	}
	return result
}

func facilityDeviceKindToProto(value domain.FacilityDeviceKind) (persistencev1.FacilityDeviceKind, error) {
	switch value {
	case domain.FacilityDeviceKindDoor:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_DOOR, nil
	case domain.FacilityDeviceKindTurret:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_TURRET, nil
	case domain.FacilityDeviceKindPowerGrid:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_POWER_GRID, nil
	case domain.FacilityDeviceKindReactor:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_REACTOR, nil
	case domain.FacilityDeviceKindVentilation:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_VENTILATION, nil
	case domain.FacilityDeviceKindAlarm:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_ALARM, nil
	case domain.FacilityDeviceKindRobotPod:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_ROBOT_POD, nil
	case domain.FacilityDeviceKindElevator:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_ELEVATOR, nil
	case domain.FacilityDeviceKindNetworkSegment:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_NETWORK_SEGMENT, nil
	case domain.FacilityDeviceKindCustom:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_CUSTOM, nil
	default:
		return persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_UNSPECIFIED, fmt.Errorf("unsupported facility device kind %q", value)
	}
}

func facilityDeviceKindFromProto(value persistencev1.FacilityDeviceKind) (domain.FacilityDeviceKind, error) {
	switch value {
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_DOOR:
		return domain.FacilityDeviceKindDoor, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_TURRET:
		return domain.FacilityDeviceKindTurret, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_POWER_GRID:
		return domain.FacilityDeviceKindPowerGrid, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_REACTOR:
		return domain.FacilityDeviceKindReactor, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_VENTILATION:
		return domain.FacilityDeviceKindVentilation, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_ALARM:
		return domain.FacilityDeviceKindAlarm, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_ROBOT_POD:
		return domain.FacilityDeviceKindRobotPod, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_ELEVATOR:
		return domain.FacilityDeviceKindElevator, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_NETWORK_SEGMENT:
		return domain.FacilityDeviceKindNetworkSegment, nil
	case persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_CUSTOM:
		return domain.FacilityDeviceKindCustom, nil
	default:
		return domain.FacilityDeviceKindUnspecified, fmt.Errorf("unsupported protobuf facility device kind %d", value)
	}
}

func diagnosticConditionCategoryToProto(value domain.DiagnosticConditionCategory) (persistencev1.DiagnosticConditionCategory, error) {
	switch value {
	case domain.DiagnosticConditionCategoryOffline:
		return persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_OFFLINE, nil
	case domain.DiagnosticConditionCategoryUnpowered:
		return persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_UNPOWERED, nil
	case domain.DiagnosticConditionCategoryNetworkIsolated:
		return persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_NETWORK_ISOLATED, nil
	case domain.DiagnosticConditionCategoryStorageDamaged:
		return persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_STORAGE_DAMAGED, nil
	case domain.DiagnosticConditionCategoryAuthorizationCorrupted:
		return persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_AUTHORIZATION_CORRUPTED, nil
	case domain.DiagnosticConditionCategoryDisplayUnstable:
		return persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_DISPLAY_UNSTABLE, nil
	case domain.DiagnosticConditionCategoryCustom:
		return persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_CUSTOM, nil
	default:
		return persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_UNSPECIFIED, fmt.Errorf("unsupported diagnostic condition category %q", value)
	}
}

func diagnosticConditionCategoryFromProto(value persistencev1.DiagnosticConditionCategory) (domain.DiagnosticConditionCategory, error) {
	switch value {
	case persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_OFFLINE:
		return domain.DiagnosticConditionCategoryOffline, nil
	case persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_UNPOWERED:
		return domain.DiagnosticConditionCategoryUnpowered, nil
	case persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_NETWORK_ISOLATED:
		return domain.DiagnosticConditionCategoryNetworkIsolated, nil
	case persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_STORAGE_DAMAGED:
		return domain.DiagnosticConditionCategoryStorageDamaged, nil
	case persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_AUTHORIZATION_CORRUPTED:
		return domain.DiagnosticConditionCategoryAuthorizationCorrupted, nil
	case persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_DISPLAY_UNSTABLE:
		return domain.DiagnosticConditionCategoryDisplayUnstable, nil
	case persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_CUSTOM:
		return domain.DiagnosticConditionCategoryCustom, nil
	default:
		return domain.DiagnosticConditionCategoryUnspecified, fmt.Errorf("unsupported protobuf diagnostic condition category %d", value)
	}
}

func facilityCapabilityToProto(value domain.FacilityCapability) (persistencev1.FacilityCapability, error) {
	switch value {
	case domain.FacilityCapabilityExecuteCommand:
		return persistencev1.FacilityCapability_FACILITY_CAPABILITY_EXECUTE_COMMAND, nil
	case domain.FacilityCapabilityViewEntry:
		return persistencev1.FacilityCapability_FACILITY_CAPABILITY_VIEW_ENTRY, nil
	case domain.FacilityCapabilityHack:
		return persistencev1.FacilityCapability_FACILITY_CAPABILITY_HACK, nil
	case domain.FacilityCapabilityTerminalTransition:
		return persistencev1.FacilityCapability_FACILITY_CAPABILITY_TERMINAL_TRANSITION, nil
	case domain.FacilityCapabilityRunRecoveryProgram:
		return persistencev1.FacilityCapability_FACILITY_CAPABILITY_RUN_RECOVERY_PROGRAM, nil
	default:
		return persistencev1.FacilityCapability_FACILITY_CAPABILITY_UNSPECIFIED, fmt.Errorf("unsupported facility capability %q", value)
	}
}

func facilityCapabilityFromProto(value persistencev1.FacilityCapability) (domain.FacilityCapability, error) {
	switch value {
	case persistencev1.FacilityCapability_FACILITY_CAPABILITY_EXECUTE_COMMAND:
		return domain.FacilityCapabilityExecuteCommand, nil
	case persistencev1.FacilityCapability_FACILITY_CAPABILITY_VIEW_ENTRY:
		return domain.FacilityCapabilityViewEntry, nil
	case persistencev1.FacilityCapability_FACILITY_CAPABILITY_HACK:
		return domain.FacilityCapabilityHack, nil
	case persistencev1.FacilityCapability_FACILITY_CAPABILITY_TERMINAL_TRANSITION:
		return domain.FacilityCapabilityTerminalTransition, nil
	case persistencev1.FacilityCapability_FACILITY_CAPABILITY_RUN_RECOVERY_PROGRAM:
		return domain.FacilityCapabilityRunRecoveryProgram, nil
	default:
		return domain.FacilityCapabilityUnspecified, fmt.Errorf("unsupported protobuf facility capability %d", value)
	}
}
