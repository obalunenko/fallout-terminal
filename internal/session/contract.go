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
					Id:          block.ID,
					InitialText: block.InitialText,
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
	result := domain.ContentNode{ID: node.GetId(), Name: node.GetName(), Extra: template.Extra}
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
			result.StateChange = &domain.StateChangeConfig{
				CompletedName:      behavior.StateChange.GetCompletedName(),
				ConfirmationText:   behavior.StateChange.GetConfirmationText(),
				EntryContentChange: entryContentChangeFromProto(behavior.StateChange.GetEntryContentChange()),
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
				result.Blocks[index] = domain.EntryContentBlock{
					ID:          block.GetId(),
					InitialText: block.GetInitialText(),
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
