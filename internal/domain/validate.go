package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
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
	MaxSoundCategoryBytes           = 32
	maxNameBytes                    = 256
	maxIntroBytes                   = 64 * 1024
	maxBodyBytes                    = 1024 * 1024
	maxFacilityDevices              = 1000
	maxFacilityStatesPerDevice      = 1000
	maxFacilityTransitionsPerDevice = 10000
	maxFacilityConditions           = 1000
	maxFacilityRecoveryPrograms     = 1000
	maxFacilityItemsPerList         = 1000
	maxFacilityItems                = 100000
	maxFacilityReferences           = 100000
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
		if err := validateTerminalEntryContent(path, terminal.Root, terminal.CommandStates, nodesByID); err != nil {
			return err
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
	if err := validateFacility(session, terminalIDs); err != nil {
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
		if slices.Contains(group.TerminalIDs, terminalID) {
			return TerminalGroupSnapshot{ID: group.ID, Name: group.Name, TerminalIDs: append([]string(nil), group.TerminalIDs...)}, true
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

type facilityDeviceIndex struct {
	device      FacilityDevice
	states      map[string]struct{}
	transitions map[string]FacilityDeviceTransition
}

type facilityTerminalIndex struct {
	nodes  map[string]ContentNode
	blocks map[string]struct{}
}

type facilityPresentationTarget struct {
	terminalID string
	targetID   string
}

type facilityGraphValidator struct {
	facility               *Facility
	devices                map[string]facilityDeviceIndex
	conditions             map[string]DiagnosticCondition
	programs               map[string]RecoveryProgram
	terminals              map[string]facilityTerminalIndex
	itemCount              int
	referenceCount         int
	diagnosticPathOwners   map[facilityPresentationTarget]string
	recordSubstituteOwners map[facilityPresentationTarget]string
}

func validateFacility(session Session, terminalIDs map[string]struct{}) error {
	validator := facilityGraphValidator{
		facility:               session.Facility,
		devices:                make(map[string]facilityDeviceIndex),
		conditions:             make(map[string]DiagnosticCondition),
		programs:               make(map[string]RecoveryProgram),
		terminals:              make(map[string]facilityTerminalIndex, len(session.Terminals)),
		diagnosticPathOwners:   make(map[facilityPresentationTarget]string),
		recordSubstituteOwners: make(map[facilityPresentationTarget]string),
	}
	for _, terminal := range session.Terminals {
		index := facilityTerminalIndex{nodes: make(map[string]ContentNode), blocks: make(map[string]struct{})}
		collectFacilityTerminalIndex(terminal.Root, index)
		validator.terminals[terminal.ID] = index
	}
	if session.Facility == nil {
		return validator.validateTerminalBindingsAndActions(session)
	}
	if err := validateExtras("facility", session.Facility.Extra, facilityFields); err != nil {
		return err
	}
	if session.Facility.Devices == nil {
		return fmt.Errorf("facility.devices must be an array")
	}
	if session.Facility.Conditions == nil {
		return fmt.Errorf("facility.conditions must be an array")
	}
	if session.Facility.RecoveryPrograms == nil {
		return fmt.Errorf("facility.recoveryPrograms must be an array")
	}
	if err := validateFacilityListLimit("facility.devices", len(session.Facility.Devices), maxFacilityDevices); err != nil {
		return err
	}
	if err := validateFacilityListLimit("facility.conditions", len(session.Facility.Conditions), maxFacilityConditions); err != nil {
		return err
	}
	if err := validateFacilityListLimit("facility.recoveryPrograms", len(session.Facility.RecoveryPrograms), maxFacilityRecoveryPrograms); err != nil {
		return err
	}
	if err := validator.addItems("facility", len(session.Facility.Devices)+len(session.Facility.Conditions)+len(session.Facility.RecoveryPrograms)); err != nil {
		return err
	}
	if err := validator.indexDevices(); err != nil {
		return err
	}
	if err := validator.indexConditions(terminalIDs); err != nil {
		return err
	}
	if err := validator.indexPrograms(); err != nil {
		return err
	}
	if err := validator.validateDeviceReferences(); err != nil {
		return err
	}
	if err := validator.validateConditions(); err != nil {
		return err
	}
	if err := validator.validateRecoveryPrograms(); err != nil {
		return err
	}
	return validator.validateTerminalBindingsAndActions(session)
}

func collectFacilityTerminalIndex(node ContentNode, index facilityTerminalIndex) {
	index.nodes[node.ID] = node
	for _, block := range node.Blocks {
		index.blocks[block.ID] = struct{}{}
	}
	for _, child := range node.Children {
		collectFacilityTerminalIndex(child, index)
	}
}

func (validator *facilityGraphValidator) indexDevices() error {
	for deviceIndex := range validator.facility.Devices {
		device := validator.facility.Devices[deviceIndex]
		path := fmt.Sprintf("facility.devices[%d]", deviceIndex)
		if err := validateFacilityID(path+".id", device.ID); err != nil {
			return err
		}
		if _, exists := validator.devices[device.ID]; exists {
			return fmt.Errorf("%s.id duplicates %q", path, device.ID)
		}
		if err := validateRequiredString(path+".name", device.Name, maxNameBytes); err != nil {
			return err
		}
		if err := validateFacilityDeviceKind(path, device); err != nil {
			return err
		}
		if err := validateExtras(path, device.Extra, facilityDeviceFields); err != nil {
			return err
		}
		if len(device.States) == 0 {
			return fmt.Errorf("%s.states must contain at least one state", path)
		}
		if err := validateFacilityListLimit(path+".states", len(device.States), maxFacilityStatesPerDevice); err != nil {
			return err
		}
		if err := validateFacilityListLimit(path+".transitions", len(device.Transitions), maxFacilityTransitionsPerDevice); err != nil {
			return err
		}
		if err := validator.addItems(path, len(device.States)+len(device.Transitions)); err != nil {
			return err
		}

		states := make(map[string]struct{}, len(device.States))
		stateNames := make(map[string]struct{}, len(device.States))
		for stateIndex, state := range device.States {
			statePath := fmt.Sprintf("%s.states[%d]", path, stateIndex)
			if err := validateFacilityID(statePath+".id", state.ID); err != nil {
				return err
			}
			if _, exists := states[state.ID]; exists {
				return fmt.Errorf("%s.id duplicates %q", statePath, state.ID)
			}
			states[state.ID] = struct{}{}
			if err := validateRequiredString(statePath+".name", state.Name, maxNameBytes); err != nil {
				return err
			}
			normalizedName := strings.ToLower(strings.TrimSpace(state.Name))
			if _, exists := stateNames[normalizedName]; exists {
				return fmt.Errorf("%s.name duplicates normalized state name %q", statePath, strings.TrimSpace(state.Name))
			}
			stateNames[normalizedName] = struct{}{}
			if err := validateExtras(statePath, state.Extra, facilityDeviceStateFields); err != nil {
				return err
			}
		}
		stateReferences := []struct {
			field string
			id    string
		}{
			{field: "initialStateId", id: device.InitialStateID},
			{field: "currentStateId", id: device.CurrentStateID},
		}
		for _, reference := range stateReferences {
			if err := validateFacilityID(path+"."+reference.field, reference.id); err != nil {
				return err
			}
			if _, exists := states[reference.id]; !exists {
				return fmt.Errorf("%s.%s references unknown state %q", path, reference.field, reference.id)
			}
		}

		transitions := make(map[string]FacilityDeviceTransition, len(device.Transitions))
		for transitionIndex, transition := range device.Transitions {
			transitionPath := fmt.Sprintf("%s.transitions[%d]", path, transitionIndex)
			if err := validateFacilityID(transitionPath+".id", transition.ID); err != nil {
				return err
			}
			if _, exists := transitions[transition.ID]; exists {
				return fmt.Errorf("%s.id duplicates %q", transitionPath, transition.ID)
			}
			transitions[transition.ID] = transition
			if err := validateRequiredString(transitionPath+".name", transition.Name, maxNameBytes); err != nil {
				return err
			}
			if err := validateExtras(transitionPath, transition.Extra, facilityTransitionFields); err != nil {
				return err
			}
			if _, exists := states[transition.SourceStateID]; !exists {
				return fmt.Errorf("%s.sourceStateId references unknown state %q", transitionPath, transition.SourceStateID)
			}
			if _, exists := states[transition.DestinationStateID]; !exists {
				return fmt.Errorf("%s.destinationStateId references unknown state %q", transitionPath, transition.DestinationStateID)
			}
			if transition.SourceStateID == transition.DestinationStateID {
				return fmt.Errorf("%s destination state must differ from source state", transitionPath)
			}
			if err := validateFacilityListLimit(transitionPath+".preconditions", len(transition.Preconditions), maxFacilityItemsPerList); err != nil {
				return err
			}
			if err := validateFacilityListLimit(transitionPath+".conditionEffects", len(transition.ConditionEffects), maxFacilityItemsPerList); err != nil {
				return err
			}
			if err := validator.addItems(transitionPath, len(transition.Preconditions)+len(transition.ConditionEffects)); err != nil {
				return err
			}
		}
		validator.devices[device.ID] = facilityDeviceIndex{device: device, states: states, transitions: transitions}
	}
	return nil
}

func (validator *facilityGraphValidator) indexConditions(terminalIDs map[string]struct{}) error {
	for conditionIndex, condition := range validator.facility.Conditions {
		path := fmt.Sprintf("facility.conditions[%d]", conditionIndex)
		if err := validateFacilityID(path+".id", condition.ID); err != nil {
			return err
		}
		if _, exists := validator.conditions[condition.ID]; exists {
			return fmt.Errorf("%s.id duplicates %q", path, condition.ID)
		}
		validator.conditions[condition.ID] = condition
		if err := validateRequiredString(path+".name", condition.Name, maxNameBytes); err != nil {
			return err
		}
		if err := validateDiagnosticConditionCategory(path, condition); err != nil {
			return err
		}
		if err := validateExtras(path, condition.Extra, diagnosticConditionFields); err != nil {
			return err
		}
		if (condition.Device == nil) == (condition.Terminal == nil) {
			return fmt.Errorf("%s must configure exactly one device or terminal scope", path)
		}
		if condition.Device != nil {
			if err := validateExtras(path+".device", condition.Device.Extra, diagnosticDeviceScopeFields); err != nil {
				return err
			}
			if _, exists := validator.devices[condition.Device.DeviceID]; !exists {
				return fmt.Errorf("%s.device.deviceId references unknown device %q", path, condition.Device.DeviceID)
			}
		}
		if condition.Terminal != nil {
			if err := validateExtras(path+".terminal", condition.Terminal.Extra, diagnosticTerminalScopeFields); err != nil {
				return err
			}
			if _, exists := terminalIDs[condition.Terminal.TerminalID]; !exists {
				return fmt.Errorf("%s.terminal.terminalId references unknown terminal %q", path, condition.Terminal.TerminalID)
			}
		}
		if len(condition.Effects) == 0 {
			return fmt.Errorf("%s.effects must contain at least one effect", path)
		}
		if err := validateFacilityListLimit(path+".effects", len(condition.Effects), maxFacilityItemsPerList); err != nil {
			return err
		}
		if err := validateFacilityListLimit(path+".recovery", len(condition.Recovery), maxFacilityItemsPerList); err != nil {
			return err
		}
		if err := validator.addItems(path, len(condition.Effects)+len(condition.Recovery)); err != nil {
			return err
		}
	}
	return nil
}

func (validator *facilityGraphValidator) indexPrograms() error {
	for programIndex, program := range validator.facility.RecoveryPrograms {
		path := fmt.Sprintf("facility.recoveryPrograms[%d]", programIndex)
		if err := validateFacilityID(path+".id", program.ID); err != nil {
			return err
		}
		if _, exists := validator.programs[program.ID]; exists {
			return fmt.Errorf("%s.id duplicates %q", path, program.ID)
		}
		validator.programs[program.ID] = program
		if err := validateRequiredString(path+".name", program.Name, maxNameBytes); err != nil {
			return err
		}
		if err := validateExtras(path, program.Extra, recoveryProgramFields); err != nil {
			return err
		}
		if len(program.Transitions) == 0 {
			return fmt.Errorf("%s.transitions must contain at least one transition", path)
		}
		if err := validateFacilityListLimit(path+".transitions", len(program.Transitions), maxFacilityItemsPerList); err != nil {
			return err
		}
		if err := validator.addItems(path, len(program.Transitions)); err != nil {
			return err
		}
	}
	return nil
}

func (validator *facilityGraphValidator) validateDeviceReferences() error {
	for deviceIndex, device := range validator.facility.Devices {
		for transitionIndex, transition := range device.Transitions {
			path := fmt.Sprintf("facility.devices[%d].transitions[%d]", deviceIndex, transitionIndex)
			preconditionDevices := make(map[string]struct{}, len(transition.Preconditions))
			for preconditionIndex, precondition := range transition.Preconditions {
				preconditionPath := fmt.Sprintf("%s.preconditions[%d]", path, preconditionIndex)
				if precondition.DeviceID == device.ID {
					return fmt.Errorf("%s must reference another device", preconditionPath)
				}
				if _, exists := preconditionDevices[precondition.DeviceID]; exists {
					return fmt.Errorf("%s repeats precondition device %q", preconditionPath, precondition.DeviceID)
				}
				preconditionDevices[precondition.DeviceID] = struct{}{}
				if err := validator.validateEquality(preconditionPath, precondition); err != nil {
					return err
				}
			}
			conditionEffects := make(map[string]bool, len(transition.ConditionEffects))
			for effectIndex, effect := range transition.ConditionEffects {
				effectPath := fmt.Sprintf("%s.conditionEffects[%d]", path, effectIndex)
				if err := validator.addReference(effectPath); err != nil {
					return err
				}
				if err := validateExtras(effectPath, effect.Extra, facilityConditionEffectFields); err != nil {
					return err
				}
				if _, exists := validator.conditions[effect.ConditionID]; !exists {
					return fmt.Errorf("%s.conditionId references unknown condition %q", effectPath, effect.ConditionID)
				}
				if active, exists := conditionEffects[effect.ConditionID]; exists {
					if active != effect.Active {
						return fmt.Errorf("%s contradicts another effect for condition %q", effectPath, effect.ConditionID)
					}
					return fmt.Errorf("%s repeats condition %q", effectPath, effect.ConditionID)
				}
				conditionEffects[effect.ConditionID] = effect.Active
			}
		}
	}
	return nil
}

func (validator *facilityGraphValidator) validateConditions() error {
	for conditionIndex, condition := range validator.facility.Conditions {
		path := fmt.Sprintf("facility.conditions[%d]", conditionIndex)
		blocksProgress := false
		for effectIndex, effect := range condition.Effects {
			effectPath := fmt.Sprintf("%s.effects[%d]", path, effectIndex)
			if err := validator.validateDiagnosticEffect(effectPath, condition, effect); err != nil {
				return err
			}
			blocksProgress = blocksProgress || effect.CapabilityBlock != nil
		}
		hasViableRecovery := false
		for recoveryIndex, recovery := range condition.Recovery {
			clearsCondition, err := validator.validateRecoveryReference(
				fmt.Sprintf("%s.recovery[%d]", path, recoveryIndex),
				condition.ID,
				recovery,
			)
			if err != nil {
				return err
			}
			hasViableRecovery = hasViableRecovery || clearsCondition
		}
		if (blocksProgress || len(condition.Recovery) != 0) && !hasViableRecovery {
			return fmt.Errorf("%s must provide recovery that clears the condition", path)
		}
	}
	return nil
}

func (validator *facilityGraphValidator) validateDiagnosticEffect(
	path string,
	condition DiagnosticCondition,
	effect DiagnosticEffect,
) error {
	if err := validator.addReference(path); err != nil {
		return err
	}
	if err := validateExtras(path, effect.Extra, diagnosticEffectFields); err != nil {
		return err
	}
	variants := 0
	if effect.CapabilityBlock != nil {
		variants++
		if err := validateExtras(path+".capabilityBlock", effect.CapabilityBlock.Extra, capabilityBlockEffectFields); err != nil {
			return err
		}
		if !validFacilityCapability(effect.CapabilityBlock.Capability) {
			return fmt.Errorf("%s.capabilityBlock.capability %q is unsupported", path, effect.CapabilityBlock.Capability)
		}
	}
	if effect.DiagnosticPath != nil {
		variants++
		value := effect.DiagnosticPath
		if err := validateExtras(path+".diagnosticPath", value.Extra, diagnosticPathEffectFields); err != nil {
			return err
		}
		terminal, exists := validator.terminals[value.TerminalID]
		if !exists {
			return fmt.Errorf("%s.diagnosticPath.terminalId references unknown terminal %q", path, value.TerminalID)
		}
		if _, exists := terminal.nodes[value.NodeID]; !exists {
			return fmt.Errorf("%s.diagnosticPath.nodeId references unknown node %q in terminal %q", path, value.NodeID, value.TerminalID)
		}
		if condition.Terminal != nil && value.TerminalID != condition.Terminal.TerminalID {
			return fmt.Errorf("%s.diagnosticPath.terminalId must match condition terminal scope %q", path, condition.Terminal.TerminalID)
		}
		target := facilityPresentationTarget{terminalID: value.TerminalID, targetID: value.NodeID}
		if owner, exists := validator.diagnosticPathOwners[target]; exists {
			return fmt.Errorf("%s.diagnosticPath overlaps condition %q", path, owner)
		}
		validator.diagnosticPathOwners[target] = condition.ID
	}
	if effect.RecordSubstitution != nil {
		variants++
		value := effect.RecordSubstitution
		if err := validateExtras(path+".recordSubstitution", value.Extra, recordSubstitutionFields); err != nil {
			return err
		}
		if len([]byte(value.ReplacementText)) > maxBodyBytes {
			return fmt.Errorf("%s.recordSubstitution.replacementText exceeds %d bytes", path, maxBodyBytes)
		}
		terminal, exists := validator.terminals[value.TerminalID]
		if !exists {
			return fmt.Errorf("%s.recordSubstitution.terminalId references unknown terminal %q", path, value.TerminalID)
		}
		if _, exists := terminal.blocks[value.BlockID]; !exists {
			return fmt.Errorf("%s.recordSubstitution.blockId references unknown block %q in terminal %q", path, value.BlockID, value.TerminalID)
		}
		if condition.Terminal != nil && value.TerminalID != condition.Terminal.TerminalID {
			return fmt.Errorf("%s.recordSubstitution.terminalId must match condition terminal scope %q", path, condition.Terminal.TerminalID)
		}
		target := facilityPresentationTarget{terminalID: value.TerminalID, targetID: value.BlockID}
		if owner, exists := validator.recordSubstituteOwners[target]; exists {
			return fmt.Errorf("%s.recordSubstitution overlaps condition %q", path, owner)
		}
		validator.recordSubstituteOwners[target] = condition.ID
	}
	if effect.DisplayInstability != nil {
		variants++
		if err := validateExtras(path+".displayInstability", effect.DisplayInstability.Extra, nil); err != nil {
			return err
		}
	}
	if variants != 1 {
		return fmt.Errorf("%s must configure exactly one diagnostic effect", path)
	}
	return nil
}

func (validator *facilityGraphValidator) validateRecoveryReference(
	path string,
	conditionID string,
	recovery DiagnosticRecoveryReference,
) (bool, error) {
	if err := validator.addReference(path); err != nil {
		return false, err
	}
	if err := validateExtras(path, recovery.Extra, diagnosticRecoveryFields); err != nil {
		return false, err
	}
	variants := 0
	clearsCondition := false
	if recovery.Transition != nil {
		variants++
		if err := validator.validateTransitionRequests(path+".transition", []FacilityTransitionRequest{*recovery.Transition}); err != nil {
			return false, err
		}
		transition := validator.devices[recovery.Transition.DeviceID].transitions[recovery.Transition.TransitionID]
		if !transition.Recovery {
			return false, fmt.Errorf("%s.transition must reference a recovery transition", path)
		}
		clearsCondition = transitionClearsCondition(transition, conditionID)
	}
	if recovery.RecoveryProgramID != nil {
		variants++
		if err := validateFacilityID(path+".recoveryProgramId", *recovery.RecoveryProgramID); err != nil {
			return false, err
		}
		program, exists := validator.programs[*recovery.RecoveryProgramID]
		if !exists {
			return false, fmt.Errorf("%s.recoveryProgramId references unknown recovery program %q", path, *recovery.RecoveryProgramID)
		}
		for _, request := range program.Transitions {
			device, exists := validator.devices[request.DeviceID]
			if !exists {
				return false, fmt.Errorf("%s.recoveryProgramId references a program with unknown device %q", path, request.DeviceID)
			}
			transition, exists := device.transitions[request.TransitionID]
			if !exists {
				return false, fmt.Errorf("%s.recoveryProgramId references a program with unknown transition %q", path, request.TransitionID)
			}
			if !transition.Recovery {
				return false, fmt.Errorf("%s.recoveryProgramId references a program with non-recovery transition %q", path, request.TransitionID)
			}
			clearsCondition = clearsCondition || transitionClearsCondition(transition, conditionID)
		}
	}
	if recovery.PrivateOverseerAction != nil {
		variants++
		if !*recovery.PrivateOverseerAction {
			return false, fmt.Errorf("%s.privateOverseerAction must be true when configured", path)
		}
		clearsCondition = true
	}
	if variants != 1 {
		return false, fmt.Errorf("%s must configure exactly one recovery variant", path)
	}
	return clearsCondition, nil
}

func transitionClearsCondition(transition FacilityDeviceTransition, conditionID string) bool {
	for _, effect := range transition.ConditionEffects {
		if effect.ConditionID == conditionID && !effect.Active {
			return true
		}
	}
	return false
}

func (validator *facilityGraphValidator) validateRecoveryPrograms() error {
	for programIndex, program := range validator.facility.RecoveryPrograms {
		path := fmt.Sprintf("facility.recoveryPrograms[%d].transitions", programIndex)
		if err := validator.validateTransitionRequests(path, program.Transitions); err != nil {
			return err
		}
	}
	return nil
}

func (validator *facilityGraphValidator) validateTerminalBindingsAndActions(session Session) error {
	for terminalIndex, terminal := range session.Terminals {
		var visit func(string, ContentNode) error
		visit = func(path string, node ContentNode) error {
			if err := validator.validateTextVariants(path+".facilityNameVariants", node.FacilityNameVariants, maxNameBytes, false); err != nil {
				return err
			}
			if node.VisibleWhen != nil {
				if node.ID == "root" {
					return fmt.Errorf("%s.visibleWhen cannot hide the terminal root", path)
				}
				if err := validator.validateEquality(path+".visibleWhen", *node.VisibleWhen); err != nil {
					return err
				}
			}
			if node.AvailableWhen != nil {
				if node.Type != NodeCommand {
					return fmt.Errorf("%s.availableWhen is valid only on commands", path)
				}
				if err := validator.validateEquality(path+".availableWhen", *node.AvailableWhen); err != nil {
					return err
				}
			}
			for blockIndex, block := range node.Blocks {
				blockPath := fmt.Sprintf("%s.blocks[%d]", path, blockIndex)
				if err := validateExtras(blockPath, block.Extra, entryContentBlockFields); err != nil {
					return err
				}
				if err := validator.validateTextVariants(blockPath+".facilityTextVariants", block.FacilityTextVariants, maxBodyBytes, true); err != nil {
					return err
				}
			}
			if node.StateChange != nil && node.StateChange.FacilityAction != nil {
				if err := validator.validateFacilityAction(path+".stateChange.facilityAction", *node.StateChange.FacilityAction); err != nil {
					return err
				}
			}
			for childIndex, child := range node.Children {
				if err := visit(fmt.Sprintf("%s.children[%d]", path, childIndex), child); err != nil {
					return err
				}
			}
			return nil
		}
		if err := visit(fmt.Sprintf("terminals[%d].root", terminalIndex), terminal.Root); err != nil {
			return err
		}
	}
	return nil
}

func (validator *facilityGraphValidator) validateTextVariants(path string, variants []FacilityTextVariant, maxTextBytes int, allowEmpty bool) error {
	if err := validateFacilityListLimit(path, len(variants), maxFacilityItemsPerList); err != nil {
		return err
	}
	controllingDevice := ""
	states := make(map[string]struct{}, len(variants))
	for index, variant := range variants {
		variantPath := fmt.Sprintf("%s[%d]", path, index)
		if err := validateExtras(variantPath, variant.Extra, facilityTextVariantFields); err != nil {
			return err
		}
		if err := validator.validateEquality(variantPath+".when", variant.When); err != nil {
			return err
		}
		if controllingDevice == "" {
			controllingDevice = variant.When.DeviceID
		} else if controllingDevice != variant.When.DeviceID {
			return fmt.Errorf("%s must use one controlling device", path)
		}
		if _, exists := states[variant.When.StateID]; exists {
			return fmt.Errorf("%s repeats state %q", path, variant.When.StateID)
		}
		states[variant.When.StateID] = struct{}{}
		if allowEmpty {
			if len([]byte(variant.Text)) > maxTextBytes {
				return fmt.Errorf("%s.text exceeds %d bytes", variantPath, maxTextBytes)
			}
		} else if err := validateRequiredString(variantPath+".text", variant.Text, maxTextBytes); err != nil {
			return err
		}
	}
	return nil
}

func (validator *facilityGraphValidator) validateFacilityAction(path string, action FacilityActionConfig) error {
	if validator.facility == nil {
		return fmt.Errorf("%s requires a configured facility", path)
	}
	if err := validateExtras(path, action.Extra, facilityActionFields); err != nil {
		return err
	}
	if (action.Transitions == nil) == (action.RecoveryProgramID == nil) {
		return fmt.Errorf("%s must configure exactly one transition list or recovery program", path)
	}
	if action.Transitions != nil {
		if err := validateExtras(path+".transitions", action.Transitions.Extra, facilityTransitionListFields); err != nil {
			return err
		}
		return validator.validateTransitionRequests(path+".transitions.transitions", action.Transitions.Transitions)
	}
	if err := validateFacilityID(path+".recoveryProgramId", *action.RecoveryProgramID); err != nil {
		return err
	}
	if _, exists := validator.programs[*action.RecoveryProgramID]; !exists {
		return fmt.Errorf("%s.recoveryProgramId references unknown recovery program %q", path, *action.RecoveryProgramID)
	}
	return nil
}

func (validator *facilityGraphValidator) validateTransitionRequests(path string, requests []FacilityTransitionRequest) error {
	if len(requests) == 0 {
		return fmt.Errorf("%s must contain at least one transition", path)
	}
	if err := validateFacilityListLimit(path, len(requests), maxFacilityItemsPerList); err != nil {
		return err
	}
	requested := make(map[string]FacilityDeviceTransition, len(requests))
	conditionEffects := make(map[string]bool)
	for requestIndex, request := range requests {
		requestPath := fmt.Sprintf("%s[%d]", path, requestIndex)
		if err := validator.addReference(requestPath); err != nil {
			return err
		}
		if err := validateExtras(requestPath, request.Extra, facilityRequestFields); err != nil {
			return err
		}
		device, exists := validator.devices[request.DeviceID]
		if !exists {
			return fmt.Errorf("%s.deviceId references unknown device %q", requestPath, request.DeviceID)
		}
		if _, exists := requested[request.DeviceID]; exists {
			return fmt.Errorf("%s repeats device %q", requestPath, request.DeviceID)
		}
		transition, exists := device.transitions[request.TransitionID]
		if !exists {
			return fmt.Errorf("%s.transitionId references unknown transition %q on device %q", requestPath, request.TransitionID, request.DeviceID)
		}
		requested[request.DeviceID] = transition
		for _, effect := range transition.ConditionEffects {
			if active, exists := conditionEffects[effect.ConditionID]; exists && active != effect.Active {
				return fmt.Errorf("%s creates contradictory effects for condition %q", path, effect.ConditionID)
			}
			conditionEffects[effect.ConditionID] = effect.Active
		}
	}
	return validateFacilityActionDependencies(path, requested)
}

func validateFacilityActionDependencies(path string, requested map[string]FacilityDeviceTransition) error {
	edges := make(map[string][]string, len(requested))
	for deviceID, transition := range requested {
		for _, precondition := range transition.Preconditions {
			other, exists := requested[precondition.DeviceID]
			if !exists {
				continue
			}
			if precondition.StateID != other.SourceStateID {
				return fmt.Errorf("%s cannot satisfy precondition on transitioning device %q from one pre-state", path, precondition.DeviceID)
			}
			edges[deviceID] = append(edges[deviceID], precondition.DeviceID)
		}
	}
	visiting := make(map[string]bool, len(requested))
	visited := make(map[string]bool, len(requested))
	var visit func(string) bool
	visit = func(deviceID string) bool {
		if visiting[deviceID] {
			return true
		}
		if visited[deviceID] {
			return false
		}
		visiting[deviceID] = true
		if slices.ContainsFunc(edges[deviceID], visit) {
			return true
		}
		visiting[deviceID] = false
		visited[deviceID] = true
		return false
	}
	for deviceID := range requested {
		if visit(deviceID) {
			return fmt.Errorf("%s contains a cyclic transition precondition", path)
		}
	}
	return nil
}

func (validator *facilityGraphValidator) validateEquality(path string, equality FacilityStateEquality) error {
	if err := validator.addReference(path); err != nil {
		return err
	}
	if err := validateExtras(path, equality.Extra, facilityEqualityFields); err != nil {
		return err
	}
	device, exists := validator.devices[equality.DeviceID]
	if !exists {
		return fmt.Errorf("%s.deviceId references unknown device %q", path, equality.DeviceID)
	}
	if _, exists := device.states[equality.StateID]; !exists {
		return fmt.Errorf("%s.stateId references unknown state %q on device %q", path, equality.StateID, equality.DeviceID)
	}
	return nil
}

func (validator *facilityGraphValidator) addReference(path string) error {
	validator.referenceCount++
	if validator.referenceCount > maxFacilityReferences {
		return fmt.Errorf("%s exceeds facility reference limit %d", path, maxFacilityReferences)
	}
	return nil
}

func (validator *facilityGraphValidator) addItems(path string, count int) error {
	validator.itemCount += count
	if validator.itemCount > maxFacilityItems {
		return fmt.Errorf("%s exceeds facility item limit %d", path, maxFacilityItems)
	}
	return nil
}

func validateFacilityDeviceKind(path string, device FacilityDevice) error {
	switch device.Kind {
	case FacilityDeviceKindDoor, FacilityDeviceKindTurret, FacilityDeviceKindPowerGrid,
		FacilityDeviceKindReactor, FacilityDeviceKindVentilation, FacilityDeviceKindAlarm,
		FacilityDeviceKindRobotPod, FacilityDeviceKindElevator, FacilityDeviceKindNetworkSegment:
		if device.CustomKind != "" {
			return fmt.Errorf("%s.customKind is valid only for a custom device", path)
		}
	case FacilityDeviceKindCustom:
		if err := validateRequiredString(path+".customKind", device.CustomKind, maxNameBytes); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s.kind %q is unsupported", path, device.Kind)
	}
	return nil
}

func validateDiagnosticConditionCategory(path string, condition DiagnosticCondition) error {
	switch condition.Category {
	case DiagnosticConditionCategoryOffline, DiagnosticConditionCategoryUnpowered,
		DiagnosticConditionCategoryNetworkIsolated, DiagnosticConditionCategoryStorageDamaged,
		DiagnosticConditionCategoryAuthorizationCorrupted, DiagnosticConditionCategoryDisplayUnstable:
		if condition.CustomCategory != "" {
			return fmt.Errorf("%s.customCategory is valid only for a custom condition", path)
		}
	case DiagnosticConditionCategoryCustom:
		if err := validateRequiredString(path+".customCategory", condition.CustomCategory, maxNameBytes); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s.category %q is unsupported", path, condition.Category)
	}
	return nil
}

func validFacilityCapability(capability FacilityCapability) bool {
	switch capability {
	case FacilityCapabilityExecuteCommand, FacilityCapabilityViewEntry, FacilityCapabilityHack,
		FacilityCapabilityTerminalTransition, FacilityCapabilityRunRecoveryProgram:
		return true
	default:
		return false
	}
}

func validateFacilityID(path, value string) error {
	if err := validateRequiredString(path, value, maxNameBytes); err != nil {
		return err
	}
	if strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return fmt.Errorf("%s must be trimmed valid UTF-8", path)
	}
	return nil
}

func validateFacilityListLimit(path string, length, limit int) error {
	if length > limit {
		return fmt.Errorf("%s exceeds limit %d", path, limit)
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
			if node.Text != "" || node.Description != "" || node.Blocks != nil {
				return fmt.Errorf("%s folder cannot contain leaf body fields", nodePath)
			}
			for index := range node.Children {
				if err := visit(fmt.Sprintf("%s.children[%d]", nodePath, index), node.Children[index], depth+1); err != nil {
					return err
				}
			}
		case NodeCommand:
			if len(node.Children) != 0 || node.Description != "" || node.Blocks != nil {
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

type entryBlockReference struct {
	path string
}

type entryReference struct {
	path   string
	blocks []EntryContentBlock
}

type commandEntryChangeReference struct {
	path      string
	commandID string
	change    EntryContentChange
}

func validateTerminalEntryContent(
	terminalPath string,
	root ContentNode,
	states map[string]CommandExecutionState,
	nodesByID map[string]ContentNode,
) error {
	blocksByID := make(map[string]entryBlockReference)
	ownersByBlockID := make(map[string]commandEntryChangeReference)
	authoredByCommandID := make(map[string]commandEntryChangeReference)
	var entries []entryReference
	var authoredChanges []commandEntryChangeReference

	var collect func(string, ContentNode) error
	collect = func(nodePath string, node ContentNode) error {
		switch node.Type {
		case NodeEntry:
			if node.Description != "" && len(node.Blocks) != 0 {
				return fmt.Errorf("%s entry cannot contain both description and blocks", nodePath)
			}
			if len(node.Blocks) != 0 {
				entries = append(entries, entryReference{path: nodePath, blocks: node.Blocks})
			}
			for index, block := range node.Blocks {
				blockPath := fmt.Sprintf("%s.blocks[%d]", nodePath, index)
				if err := validateEntryBlockID(blockPath+".id", block.ID); err != nil {
					return err
				}
				if previous, exists := blocksByID[block.ID]; exists {
					return fmt.Errorf("%s.id duplicates %q from %s", blockPath, block.ID, previous.path)
				}
				if len([]byte(block.InitialText)) > maxBodyBytes {
					return fmt.Errorf("%s.initialText exceeds %d bytes", blockPath, maxBodyBytes)
				}
				blocksByID[block.ID] = entryBlockReference{path: blockPath}
			}
		case NodeCommand:
			if node.StateChange != nil && node.StateChange.EntryContentChange != nil {
				changePath := nodePath + ".stateChange.entryContentChange"
				if err := validateEntryContentChange(changePath, *node.StateChange.EntryContentChange); err != nil {
					return err
				}
				authoredChanges = append(authoredChanges, commandEntryChangeReference{
					path: changePath, commandID: node.ID, change: *node.StateChange.EntryContentChange,
				})
			}
		}
		for index, child := range node.Children {
			if err := collect(fmt.Sprintf("%s.children[%d]", nodePath, index), child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collect(terminalPath+".root", root); err != nil {
		return err
	}
	if err := validateComposedEntryText(entries, nil, "initial"); err != nil {
		return err
	}

	authoredTextByBlockID := make(map[string]string, len(authoredChanges))
	for _, reference := range authoredChanges {
		if _, exists := blocksByID[reference.change.BlockID]; !exists {
			return fmt.Errorf(
				"%s.blockId %q does not reference an entry block in terminal %s",
				reference.path, reference.change.BlockID, terminalPath,
			)
		}
		if owner, exists := ownersByBlockID[reference.change.BlockID]; exists {
			return fmt.Errorf(
				"%s.blockId %q is already targeted by command %q; command %q cannot also target it",
				reference.path, reference.change.BlockID, owner.commandID, reference.commandID,
			)
		}
		ownersByBlockID[reference.change.BlockID] = reference
		authoredByCommandID[reference.commandID] = reference
		authoredTextByBlockID[reference.change.BlockID] = reference.change.CompletedText
	}
	if err := validateComposedEntryText(entries, authoredTextByBlockID, "authored completed"); err != nil {
		return err
	}

	frozenTextByBlockID := make(map[string]string)
	frozenOwnerByBlockID := make(map[string]string)
	commandIDs := make([]string, 0, len(states))
	for commandID := range states {
		commandIDs = append(commandIDs, commandID)
	}
	slices.Sort(commandIDs)
	for _, commandID := range commandIDs {
		frozen := states[commandID].EntryContentChange
		if frozen == nil {
			continue
		}
		if owner, exists := frozenOwnerByBlockID[frozen.BlockID]; exists {
			return fmt.Errorf(
				"%s.commandStates[%q].entryContentChange.blockId %q duplicates frozen owner command %q",
				terminalPath, commandID, frozen.BlockID, owner,
			)
		}
		frozenOwnerByBlockID[frozen.BlockID] = commandID
	}
	for _, commandID := range commandIDs {
		state := states[commandID]
		statePath := fmt.Sprintf("%s.commandStates[%q]", terminalPath, commandID)
		node := nodesByID[commandID]
		authored, authoredExists := authoredByCommandID[commandID]
		frozen := state.EntryContentChange
		switch {
		case !authoredExists && frozen == nil:
			continue
		case authoredExists && frozen == nil:
			return fmt.Errorf(
				"%s.entryContentChange must retain command %q authored target %q",
				statePath, commandID, authored.change.BlockID,
			)
		case !authoredExists && frozen != nil:
			return fmt.Errorf(
				"%s.entryContentChange targets %q but command %q has no authored entry content change",
				statePath, frozen.BlockID, commandID,
			)
		}
		if err := validateEntryContentChange(statePath+".entryContentChange", *frozen); err != nil {
			return err
		}
		if _, exists := blocksByID[frozen.BlockID]; !exists {
			return fmt.Errorf(
				"%s.entryContentChange.blockId %q does not reference an entry block in terminal %s",
				statePath, frozen.BlockID, terminalPath,
			)
		}
		if frozen.BlockID != authored.change.BlockID {
			return fmt.Errorf(
				"%s for command %q targets %q but its authored target is %q",
				statePath+".entryContentChange", node.ID, frozen.BlockID, authored.change.BlockID,
			)
		}
		frozenTextByBlockID[frozen.BlockID] = frozen.CompletedText
	}
	return validateComposedEntryText(entries, frozenTextByBlockID, "frozen completed")
}

func validateEntryContentChange(path string, change EntryContentChange) error {
	if err := validateEntryBlockID(path+".blockId", change.BlockID); err != nil {
		return err
	}
	if len([]byte(change.CompletedText)) > maxBodyBytes {
		return fmt.Errorf("%s.completedText exceeds %d bytes", path, maxBodyBytes)
	}
	return nil
}

func validateEntryBlockID(path, value string) error {
	if err := validateRequiredString(path, value, maxNameBytes); err != nil {
		return err
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("%s must not contain surrounding whitespace", path)
	}
	return nil
}

func validateComposedEntryText(entries []entryReference, replacements map[string]string, phase string) error {
	for _, entry := range entries {
		composedBytes := 0
		for index, block := range entry.blocks {
			if index != 0 {
				composedBytes += 2
			}
			text := block.InitialText
			if replacement, exists := replacements[block.ID]; exists {
				text = replacement
			}
			composedBytes += len([]byte(text))
			if composedBytes > maxBodyBytes {
				return fmt.Errorf("%s.blocks %s composed text exceeds %d bytes", entry.path, phase, maxBodyBytes)
			}
		}
	}
	return nil
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
