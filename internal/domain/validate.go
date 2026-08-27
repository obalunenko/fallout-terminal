package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
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

// ValidateCharacterName returns the normalized player-facing character name.
func ValidateCharacterName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("character name must not be blank")
	}
	if utf8.RuneCountInString(value) > MaxCharacterNameRunes {
		return "", fmt.Errorf("character name must be at most %d characters", MaxCharacterNameRunes)
	}
	return value, nil
}

// ValidateCharacterIntelligence validates the player-facing intelligence range.
func ValidateCharacterIntelligence(value int) error {
	if value < 1 || value > 10 {
		return fmt.Errorf("character intelligence must be between 1 and 10")
	}
	return nil
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
	if err := validateTerminalGroups(session, terminalIDs); err != nil {
		return err
	}
	return nil
}

// NormalizeTerminalGroups returns a detached canonical session for the wholly
// absent legacy group shape. Malformed explicit groups are left for validation.
func NormalizeTerminalGroups(session Session) Session {
	clone := session
	if len(clone.Terminals) == 0 || len(clone.TerminalGroups) != 0 {
		return clone
	}
	return EnsureTerminalGroups(clone)
}

// EnsureTerminalGroups adds singleton groups for terminals not represented by
// the supplied groups. It is intended for trusted terminal create/import
// normalization after canonical existing memberships have been retained.
func EnsureTerminalGroups(session Session) Session {
	clone := session
	clone.TerminalGroups = CloneTerminalGroups(session.TerminalGroups)
	usedIDs := make(map[string]struct{}, len(clone.TerminalGroups))
	usedNames := make(map[string]struct{}, len(clone.TerminalGroups))
	members := make(map[string]struct{}, len(clone.Terminals))
	for _, group := range clone.TerminalGroups {
		usedIDs[group.ID] = struct{}{}
		usedNames[strings.ToLower(strings.TrimSpace(group.Name))] = struct{}{}
		for _, terminalID := range group.TerminalIDs {
			members[terminalID] = struct{}{}
		}
	}
	for _, terminal := range clone.Terminals {
		if _, exists := members[terminal.ID]; exists {
			continue
		}
		clone.TerminalGroups = append(clone.TerminalGroups, TerminalGroup{
			ID:          uniqueSingletonGroupID(terminal.ID, usedIDs),
			Name:        uniqueSingletonGroupName(terminal.Name, usedNames),
			TerminalIDs: []string{terminal.ID},
		})
	}
	return clone
}

// TerminalGroupFor returns a detached ordered group containing terminalID.
func TerminalGroupFor(session Session, terminalID string) (TerminalGroupSnapshot, bool) {
	for _, group := range session.TerminalGroups {
		for _, memberID := range group.TerminalIDs {
			if memberID == terminalID {
				return TerminalGroupSnapshot{ID: group.ID, Name: group.Name, TerminalIDs: append([]string(nil), group.TerminalIDs...)}, true
			}
		}
	}
	return TerminalGroupSnapshot{}, false
}

// TerminalGroupDiff describes the server-derived semantic impact of replacing
// the complete group set. Callers must never trust an authored impact flag.
type TerminalGroupDiff struct {
	Changed                  bool
	MembershipOrOrderChanged bool
	RemovedGroupIDs          []string
	AffectedTerminalIDs      []string
}

// ValidateTerminalGroupReplacement validates one complete trusted group
// candidate against the current canonical session. Unlike compatible document
// loading, this boundary also requires every authored transition to remain
// inside one group.
func ValidateTerminalGroupReplacement(session Session, groups []TerminalGroup) (TerminalGroupDiff, error) {
	candidate := CloneSession(session)
	candidate.TerminalGroups = CloneTerminalGroups(groups)
	if err := ValidateSession(candidate); err != nil {
		return TerminalGroupDiff{}, fmt.Errorf("terminal group candidate is invalid: %w", err)
	}

	groupByTerminal := make(map[string]string, len(candidate.Terminals))
	for _, group := range candidate.TerminalGroups {
		for _, terminalID := range group.TerminalIDs {
			groupByTerminal[terminalID] = group.ID
		}
	}
	var transitionConflicts []string
	for terminalIndex, terminal := range candidate.Terminals {
		_, references, err := validateTree(fmt.Sprintf("terminals[%d].root", terminalIndex), terminal.Root)
		if err != nil {
			return TerminalGroupDiff{}, err
		}
		for _, reference := range references {
			if groupByTerminal[terminal.ID] != groupByTerminal[reference.targetTerminalID] {
				transitionConflicts = append(transitionConflicts, fmt.Sprintf(
					"terminal transition command %q in terminal %q targets terminal %q and crosses groups %q and %q",
					reference.commandID, terminal.ID, reference.targetTerminalID,
					groupByTerminal[terminal.ID], groupByTerminal[reference.targetTerminalID],
				))
			}
		}
	}
	if len(transitionConflicts) > 0 {
		return TerminalGroupDiff{}, fmt.Errorf(
			"terminal group candidate invalidates authored transitions: %s",
			strings.Join(transitionConflicts, "; "),
		)
	}

	diff := deriveTerminalGroupDiff(session.TerminalGroups, candidate.TerminalGroups)
	if err := rejectSingletonIdentityChurn(session.TerminalGroups, candidate.TerminalGroups); err != nil {
		return TerminalGroupDiff{}, err
	}
	return diff, nil
}

func deriveTerminalGroupDiff(current, candidate []TerminalGroup) TerminalGroupDiff {
	diff := TerminalGroupDiff{Changed: !terminalGroupsEqual(current, candidate)}
	if !diff.Changed {
		return diff
	}

	currentByID := make(map[string]TerminalGroup, len(current))
	candidateByID := make(map[string]TerminalGroup, len(candidate))
	for _, group := range current {
		currentByID[group.ID] = group
	}
	for _, group := range candidate {
		candidateByID[group.ID] = group
	}
	affected := make(map[string]struct{})
	for _, group := range current {
		next, exists := candidateByID[group.ID]
		if !exists {
			diff.RemovedGroupIDs = append(diff.RemovedGroupIDs, group.ID)
			diff.MembershipOrOrderChanged = true
			for _, terminalID := range group.TerminalIDs {
				affected[terminalID] = struct{}{}
			}
			continue
		}
		if !stringSlicesEqual(group.TerminalIDs, next.TerminalIDs) {
			diff.MembershipOrOrderChanged = true
			for _, terminalID := range group.TerminalIDs {
				affected[terminalID] = struct{}{}
			}
			for _, terminalID := range next.TerminalIDs {
				affected[terminalID] = struct{}{}
			}
		}
	}
	for _, group := range candidate {
		if _, exists := currentByID[group.ID]; exists {
			continue
		}
		diff.MembershipOrOrderChanged = true
		for _, terminalID := range group.TerminalIDs {
			affected[terminalID] = struct{}{}
		}
	}
	for _, terminal := range candidateTerminalOrder(current, candidate) {
		if _, exists := affected[terminal]; exists {
			diff.AffectedTerminalIDs = append(diff.AffectedTerminalIDs, terminal)
		}
	}
	return diff
}

func rejectSingletonIdentityChurn(current, candidate []TerminalGroup) error {
	candidateByMember := make(map[string]TerminalGroup)
	for _, group := range candidate {
		if len(group.TerminalIDs) == 1 {
			candidateByMember[group.TerminalIDs[0]] = group
		}
	}
	for _, group := range current {
		if len(group.TerminalIDs) != 1 {
			continue
		}
		replacement, remainsSingleton := candidateByMember[group.TerminalIDs[0]]
		if remainsSingleton && replacement.ID != group.ID {
			return fmt.Errorf("singleton group %q for terminal %q cannot be dissolved without moving the terminal", group.Name, group.TerminalIDs[0])
		}
	}
	return nil
}

func terminalGroupsEqual(left, right []TerminalGroup) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID || left[index].Name != right[index].Name ||
			!stringSlicesEqual(left[index].TerminalIDs, right[index].TerminalIDs) {
			return false
		}
	}
	return true
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func candidateTerminalOrder(current, candidate []TerminalGroup) []string {
	seen := make(map[string]struct{})
	ordered := make([]string, 0)
	for _, groups := range [][]TerminalGroup{current, candidate} {
		for _, group := range groups {
			for _, terminalID := range group.TerminalIDs {
				if _, exists := seen[terminalID]; exists {
					continue
				}
				seen[terminalID] = struct{}{}
				ordered = append(ordered, terminalID)
			}
		}
	}
	return ordered
}

func validateTerminalGroups(session Session, terminalIDs map[string]struct{}) error {
	if len(session.TerminalGroups) == 0 {
		return nil
	}
	groupIDs := make(map[string]struct{}, len(session.TerminalGroups))
	groupNames := make(map[string]struct{}, len(session.TerminalGroups))
	members := make(map[string]string, len(session.Terminals))
	for groupIndex, group := range session.TerminalGroups {
		path := fmt.Sprintf("terminalGroups[%d]", groupIndex)
		if err := validateRequiredString(path+".id", group.ID, maxNameBytes); err != nil {
			return err
		}
		if _, exists := groupIDs[group.ID]; exists {
			return fmt.Errorf("%s.id duplicates %q", path, group.ID)
		}
		groupIDs[group.ID] = struct{}{}
		if err := validateRequiredString(path+".name", group.Name, maxNameBytes); err != nil {
			return err
		}
		normalizedName := strings.ToLower(strings.TrimSpace(group.Name))
		if _, exists := groupNames[normalizedName]; exists {
			return fmt.Errorf("%s.name duplicates normalized group name %q", path, strings.TrimSpace(group.Name))
		}
		groupNames[normalizedName] = struct{}{}
		if len(group.TerminalIDs) == 0 {
			return fmt.Errorf("%s.terminalIds must contain at least one terminal", path)
		}
		for memberIndex, terminalID := range group.TerminalIDs {
			memberPath := fmt.Sprintf("%s.terminalIds[%d]", path, memberIndex)
			if err := validateRequiredString(memberPath, terminalID, maxNameBytes); err != nil {
				return err
			}
			if _, exists := terminalIDs[terminalID]; !exists {
				return fmt.Errorf("%s references unknown terminal %q", memberPath, terminalID)
			}
			if priorGroup, exists := members[terminalID]; exists {
				return fmt.Errorf("%s assigns terminal %q more than once (already in group %q)", memberPath, terminalID, priorGroup)
			}
			members[terminalID] = group.ID
		}
	}
	for terminalIndex, terminal := range session.Terminals {
		if _, exists := members[terminal.ID]; !exists {
			return fmt.Errorf("terminals[%d].id %q is missing from terminalGroups", terminalIndex, terminal.ID)
		}
	}
	return nil
}

func uniqueSingletonGroupID(terminalID string, used map[string]struct{}) string {
	digest := sha256.Sum256([]byte(terminalID))
	base := fmt.Sprintf("singleton-%x", digest[:12])
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate += "-" + strconv.Itoa(suffix)
		}
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

func uniqueSingletonGroupName(terminalName string, used map[string]struct{}) string {
	base := strings.TrimSpace(terminalName)
	if base == "" {
		base = "Terminal"
	}
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			ending := " (" + strconv.Itoa(suffix) + ")"
			candidate = truncateUTF8Bytes(base, maxNameBytes-len(ending)) + ending
		}
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if _, exists := used[normalized]; !exists {
			used[normalized] = struct{}{}
			return candidate
		}
	}
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
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
	commandID        string
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
					path: nodePath + ".terminalTransition", commandID: node.ID,
					targetTerminalID: node.TerminalTransition.TargetTerminalID,
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
