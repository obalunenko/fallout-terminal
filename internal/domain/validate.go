package domain

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	// MaxTerminals bounds terminals in one version-1 session.
	MaxTerminals = 1000
	// MaxNodeDepth counts the root as depth one.
	MaxNodeDepth = 64
	// MaxNodes bounds recursive content nodes within one terminal.
	MaxNodes = 100000
	// MaxRosterEntries bounds authored characters in one player config.
	MaxRosterEntries = 1000
	// MaxCharacterNameRunes is the shared player-facing character-name limit.
	MaxCharacterNameRunes = 80
	// MaxRecognitionHandleBytes bounds the opaque process-local browser handle.
	MaxRecognitionHandleBytes = 128
	// MaxRequestIDBytes bounds a browser-generated mutation identity.
	MaxRequestIDBytes = 128
	// MaxBroadcastIDBytes bounds a process-local broadcast identity.
	MaxBroadcastIDBytes = 128
	// MaxGenerationIDBytes bounds a private/current puzzle generation identity.
	MaxGenerationIDBytes = 128
	// MaxTerminalIDBytes bounds an authored/current terminal identity.
	MaxTerminalIDBytes = 256
	// MaxCharacterIDBytes bounds an authored roster identity.
	MaxCharacterIDBytes = 256
	// MaxActionTargetBytes bounds node, guess, and opaque pattern targets.
	MaxActionTargetBytes = 256
	// MaxPresentationPageIndex bounds controller-owned page ordinals before a
	// responsive client clamps them to its rendered page count.
	MaxPresentationPageIndex = 10000
	// MaxSoundCategoryBytes bounds typed sound adapter input before lookup.
	MaxSoundCategoryBytes = 32
	maxNameBytes          = 256
	maxIntroBytes         = 64 * 1024
	maxBodyBytes          = 1024 * 1024
)

// PublicField identifies one finitely bounded public scalar category.
type PublicField string

const (
	PublicFieldRecognitionHandle PublicField = "recognition handle"
	PublicFieldRequestID         PublicField = "request ID"
	PublicFieldBroadcastID       PublicField = "broadcast ID"
	PublicFieldGenerationID      PublicField = "generation ID"
	PublicFieldTerminalID        PublicField = "terminal ID"
	PublicFieldCharacterID       PublicField = "character ID"
	PublicFieldActionTarget      PublicField = "action target"
)

// ValidatePublicField validates one required opaque/identity scalar without
// interpreting its lifecycle or authority. Values are ASCII-safe, nonblank,
// and bounded before they can reach canonical services.
func ValidatePublicField(field PublicField, value string) error {
	limit := publicFieldLimit(field)
	if limit == 0 {
		return fmt.Errorf("unsupported public field %q", field)
	}
	if value == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return fmt.Errorf("%s must be a nonblank valid UTF-8 value", field)
	}
	if len([]byte(value)) > limit {
		return fmt.Errorf("%s exceeds %d bytes", field, limit)
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return fmt.Errorf("%s contains an invalid character", field)
		}
	}
	return nil
}

// ValidateSoundCategory returns the stable category or rejects arbitrary path
// and filesystem input before any asset capability is consulted.
func ValidateSoundCategory(value string) (SoundCategory, error) {
	if value == "" || len([]byte(value)) > MaxSoundCategoryBytes || strings.ContainsAny(value, `/\\`) {
		return "", fmt.Errorf("sound category is invalid")
	}
	category := SoundCategory(value)
	switch category {
	case SoundCategoryAmbient,
		SoundCategoryHackGood,
		SoundCategoryHackBad,
		SoundCategoryMenuFocus,
		SoundCategorySingle,
		SoundCategoryMultiple,
		SoundCategoryEnter,
		SoundCategoryCharscroll:
		return category, nil
	default:
		return "", fmt.Errorf("sound category %q is unsupported", value)
	}
}

func publicFieldLimit(field PublicField) int {
	switch field {
	case PublicFieldRecognitionHandle:
		return MaxRecognitionHandleBytes
	case PublicFieldRequestID:
		return MaxRequestIDBytes
	case PublicFieldBroadcastID:
		return MaxBroadcastIDBytes
	case PublicFieldGenerationID:
		return MaxGenerationIDBytes
	case PublicFieldTerminalID:
		return MaxTerminalIDBytes
	case PublicFieldCharacterID:
		return MaxCharacterIDBytes
	case PublicFieldActionTarget:
		return MaxActionTargetBytes
	default:
		return 0
	}
}

// ValidateSession validates every known version-1 field without mutating data.
func ValidateSession(session Session) error {
	if session.Version != 1 {
		return fmt.Errorf("version must be 1")
	}
	if err := validateRequiredString("name", session.Name, maxNameBytes); err != nil {
		return err
	}
	if session.Terminals == nil {
		return fmt.Errorf("terminals must be an array")
	}
	if session.PlayerConfig != "" {
		if filepath.IsAbs(session.PlayerConfig) || filepath.Clean(session.PlayerConfig) == "." {
			return fmt.Errorf("playerConfig must be a relative file path")
		}
		if strings.ContainsRune(session.PlayerConfig, '\x00') {
			return fmt.Errorf("playerConfig contains an invalid path character")
		}
	}
	if len(session.Terminals) > MaxTerminals {
		return fmt.Errorf("terminals exceeds limit %d", MaxTerminals)
	}
	if err := validateExtras("session", session.Extra, sessionFields); err != nil {
		return err
	}

	terminalIDs := make(map[string]struct{}, len(session.Terminals))
	type transitionReference struct {
		path, sourceTerminalID, targetTerminalID string
	}
	var transitionReferences []transitionReference
	for index := range session.Terminals {
		terminal := session.Terminals[index]
		path := fmt.Sprintf("terminals[%d]", index)
		if err := validateRequiredString(path+".id", terminal.ID, maxNameBytes); err != nil {
			return err
		}
		if _, exists := terminalIDs[terminal.ID]; exists {
			return fmt.Errorf("%s.id duplicates %q", path, terminal.ID)
		}
		terminalIDs[terminal.ID] = struct{}{}
		if err := validateRequiredString(path+".name", terminal.Name, maxNameBytes); err != nil {
			return err
		}
		if terminal.HackLevel < 0 || terminal.HackLevel > 5 {
			return fmt.Errorf("%s.hackLevel must be between 0 and 5", path)
		}
		if len([]byte(terminal.IntroText)) > maxIntroBytes {
			return fmt.Errorf("%s.introText exceeds %d bytes", path, maxIntroBytes)
		}
		if terminal.Root.ID != "root" || terminal.Root.Type != NodeFolder {
			return fmt.Errorf("%s.root must be folder root", path)
		}
		if err := validateExtras(path, terminal.Extra, terminalFields); err != nil {
			return err
		}
		nodesByID, references, err := validateTree(path+".root", terminal.Root)
		if err != nil {
			return err
		}
		for _, reference := range references {
			transitionReferences = append(transitionReferences, transitionReference{
				path: reference.path, sourceTerminalID: terminal.ID, targetTerminalID: reference.targetTerminalID,
			})
		}
		for commandID, state := range terminal.CommandStates {
			statePath := fmt.Sprintf("%s.commandStates[%q]", path, commandID)
			node, exists := nodesByID[commandID]
			if !exists {
				return fmt.Errorf("%s references an unknown command", statePath)
			}
			if node.Type != NodeCommand || node.StateChange == nil {
				return fmt.Errorf("%s must reference a state-changing command", statePath)
			}
			if err := validateRequiredString(statePath+".completedName", state.CompletedName, maxNameBytes); err != nil {
				return err
			}
			if err := validateRequiredString(statePath+".resultText", state.ResultText, maxBodyBytes); err != nil {
				return err
			}
		}
	}
	for _, reference := range transitionReferences {
		if reference.targetTerminalID == reference.sourceTerminalID {
			return fmt.Errorf("%s.targetTerminalId must reference another terminal", reference.path)
		}
		if _, exists := terminalIDs[reference.targetTerminalID]; !exists {
			return fmt.Errorf("%s.targetTerminalId references unknown terminal %q", reference.path, reference.targetTerminalID)
		}
	}
	return nil
}

// ValidatePlayerConfig validates a complete standalone authored roster.
func ValidatePlayerConfig(config PlayerConfig) error {
	if config.Version != 1 {
		return fmt.Errorf("version must be 1")
	}
	if err := validateRequiredString("name", config.Name, maxNameBytes); err != nil {
		return err
	}
	if config.Roster == nil {
		return fmt.Errorf("roster must be an array")
	}
	if len(config.Roster) > MaxRosterEntries {
		return fmt.Errorf("roster exceeds limit %d", MaxRosterEntries)
	}
	ids := make(map[CharacterID]struct{}, len(config.Roster))
	for index, entry := range config.Roster {
		path := fmt.Sprintf("roster[%d]", index)
		if err := validateRequiredString(path+".id", string(entry.ID), maxNameBytes); err != nil {
			return err
		}
		if _, exists := ids[entry.ID]; exists {
			return fmt.Errorf("%s.id duplicates %q", path, entry.ID)
		}
		ids[entry.ID] = struct{}{}
		if strings.TrimSpace(entry.Name) == "" {
			return fmt.Errorf("%s.name must not be blank", path)
		}
		if utf8.RuneCountInString(entry.Name) > MaxCharacterNameRunes {
			return fmt.Errorf("%s.name exceeds %d characters", path, MaxCharacterNameRunes)
		}
		if entry.Intelligence < 1 || entry.Intelligence > 10 {
			return fmt.Errorf("%s.intelligence must be between 1 and 10", path)
		}
	}
	return nil
}

type treeTransitionReference struct {
	path             string
	targetTerminalID string
}

func validateTree(path string, root ContentNode) (map[string]ContentNode, []treeTransitionReference, error) {
	nodesByID := make(map[string]ContentNode)
	var transitionReferences []treeTransitionReference
	count := 0
	var visit func(string, ContentNode, int) error
	visit = func(nodePath string, node ContentNode, depth int) error {
		count++
		if count > MaxNodes {
			return fmt.Errorf("%s exceeds node limit %d", path, MaxNodes)
		}
		if depth > MaxNodeDepth {
			return fmt.Errorf("%s exceeds depth limit %d", nodePath, MaxNodeDepth)
		}
		if err := validateRequiredString(nodePath+".id", node.ID, maxNameBytes); err != nil {
			return err
		}
		if _, exists := nodesByID[node.ID]; exists {
			return fmt.Errorf("%s.id duplicates %q", nodePath, node.ID)
		}
		nodesByID[node.ID] = node
		if err := validateRequiredString(nodePath+".name", node.Name, maxNameBytes); err != nil {
			return err
		}
		if err := validateExtras(nodePath, node.Extra, nodeFields); err != nil {
			return err
		}

		switch node.Type {
		case NodeFolder:
			if node.StateChange != nil || node.TerminalTransition != nil {
				return fmt.Errorf("%s folder cannot contain command configuration", nodePath)
			}
			if node.Children == nil {
				return fmt.Errorf("%s.children must be an array", nodePath)
			}
			if node.Text != "" || node.Description != "" {
				return fmt.Errorf("%s folder cannot contain leaf body fields", nodePath)
			}
			for index := range node.Children {
				if err := visit(fmt.Sprintf("%s.children[%d]", nodePath, index), node.Children[index], depth+1); err != nil {
					return err
				}
			}
		case NodeCommand:
			if len(node.Children) != 0 || node.Description != "" {
				return fmt.Errorf("%s command must be a leaf", nodePath)
			}
			if len([]byte(node.Text)) > maxBodyBytes {
				return fmt.Errorf("%s.text exceeds %d bytes", nodePath, maxBodyBytes)
			}
			switch node.Behavior() {
			case CommandBehaviorInvalid:
				return fmt.Errorf("%s command cannot contain both stateChange and terminalTransition", nodePath)
			case CommandBehaviorStateChange:
				if err := validateRequiredString(nodePath+".text", node.Text, maxBodyBytes); err != nil {
					return err
				}
				if err := validateRequiredString(nodePath+".stateChange.completedName", node.StateChange.CompletedName, maxNameBytes); err != nil {
					return err
				}
				if err := validateRequiredString(nodePath+".stateChange.confirmationText", node.StateChange.ConfirmationText, maxBodyBytes); err != nil {
					return err
				}
			case CommandBehaviorTerminalTransition:
				if err := validateRequiredString(nodePath+".terminalTransition.targetTerminalId", node.TerminalTransition.TargetTerminalID, MaxTerminalIDBytes); err != nil {
					return err
				}
				transitionReferences = append(transitionReferences, treeTransitionReference{
					path: nodePath + ".terminalTransition", targetTerminalID: node.TerminalTransition.TargetTerminalID,
				})
			case CommandBehaviorOrdinary:
				// No optional command behavior is configured.
			default:
				return fmt.Errorf("%s command has unsupported behavior %q", nodePath, node.Behavior())
			}
		case NodeEntry:
			if node.StateChange != nil || node.TerminalTransition != nil {
				return fmt.Errorf("%s entry cannot contain command configuration", nodePath)
			}
			if len(node.Children) != 0 || node.Text != "" {
				return fmt.Errorf("%s entry must be a leaf", nodePath)
			}
			if len([]byte(node.Description)) > maxBodyBytes {
				return fmt.Errorf("%s.description exceeds %d bytes", nodePath, maxBodyBytes)
			}
		default:
			return fmt.Errorf("%s.type %q is unsupported", nodePath, node.Type)
		}
		return nil
	}
	if err := visit(path, root, 1); err != nil {
		return nil, nil, err
	}
	return nodesByID, transitionReferences, nil
}

func validateRequiredString(path, value string, maxBytes int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be blank", path)
	}
	if len([]byte(value)) > maxBytes {
		return fmt.Errorf("%s exceeds %d bytes", path, maxBytes)
	}
	return nil
}

func validateExtras(path string, extras map[string]json.RawMessage, known map[string]struct{}) error {
	for field, value := range extras {
		if _, exists := known[field]; exists {
			return fmt.Errorf("%s extra field %q shadows a known field", path, field)
		}
		if !json.Valid(value) {
			return fmt.Errorf("%s extra field %q contains invalid JSON", path, field)
		}
	}
	return nil
}
