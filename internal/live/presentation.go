package live

import (
	"maps"
	"slices"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// PresentationRuleKind names what a display rule sets. Within one target
// (name, visibility, availability, or block text) the kind fixes the rule's
// precedence (data-model.md, FR-010).
type PresentationRuleKind string

// Display rule kinds, grouped by target, lowest precedence first.
const (
	PresentationRuleKindUnspecified PresentationRuleKind = ""

	PresentationRuleKindAuthoredName       PresentationRuleKind = "authored-name"
	PresentationRuleKindNameAfterExecution PresentationRuleKind = "name-after-execution"
	PresentationRuleKindStateName          PresentationRuleKind = "state-name"

	PresentationRuleKindStateVisibility            PresentationRuleKind = "state-visibility"
	PresentationRuleKindDiagnosticForcedVisibility PresentationRuleKind = "diagnostic-forced-visibility"

	PresentationRuleKindStateAvailability         PresentationRuleKind = "state-availability"
	PresentationRuleKindDiagnosticCapabilityBlock PresentationRuleKind = "diagnostic-capability-block"

	PresentationRuleKindInitialText            PresentationRuleKind = "initial-text"
	PresentationRuleKindTextAfterExecution     PresentationRuleKind = "text-after-execution"
	PresentationRuleKindStateText              PresentationRuleKind = "state-text"
	PresentationRuleKindDiagnosticSubstitution PresentationRuleKind = "diagnostic-substitution"
)

// PresentationRuleEffect is what a visibility or availability rule makes true
// while its condition holds. Name and text rules carry text instead.
type PresentationRuleEffect string

// Visibility and availability rule effects.
const (
	PresentationRuleEffectUnspecified PresentationRuleEffect = ""
	PresentationRuleEffectVisible     PresentationRuleEffect = "visible"
	PresentationRuleEffectAvailable   PresentationRuleEffect = "available"
	PresentationRuleEffectUnavailable PresentationRuleEffect = "unavailable"
)

// PresentationCondition is the live value a display rule depends on. At most
// one field is set; none means the rule always holds.
type PresentationCondition struct {
	DeviceState      *domain.FacilityStateEquality `json:"deviceState,omitempty"`
	CommandExecuted  *CommandExecutedCondition     `json:"commandExecuted,omitempty"`
	DiagnosticActive *DiagnosticActiveCondition    `json:"diagnosticActive,omitempty"`
}

// CommandExecutedCondition holds while the terminal has executed the command.
type CommandExecutedCondition struct {
	TerminalID string `json:"terminalId"`
	CommandID  string `json:"commandId"`
}

// DiagnosticActiveCondition holds while the facility fault is active.
type DiagnosticActiveCondition struct {
	ConditionID string `json:"conditionId"`
}

// PresentationRule is one display rule of one target. A rule with a higher
// precedence wins while its condition holds. A state visibility or
// availability rule also decides the opposite while its condition does not
// hold.
type PresentationRule struct {
	Kind       PresentationRuleKind  `json:"kind"`
	Precedence uint32                `json:"precedence"`
	Condition  PresentationCondition `json:"condition"`
	// Text is the name or block text the rule sets; only name and text kinds
	// carry it.
	Text string `json:"text,omitempty"`
	// Effect is set by visibility and availability kinds only.
	Effect         PresentationRuleEffect `json:"effect,omitempty"`
	ConditionHolds bool                   `json:"conditionHolds"`
	// PlayersSee marks the rule that decides what players see now. At most
	// one rule per target is marked; none is when the default applies.
	PlayersSee bool `json:"playersSee"`
}

// BlockExplanation explains the text players see for one entry block.
type BlockExplanation struct {
	BlockID    string             `json:"blockId"`
	PlayerText string             `json:"playerText"`
	TextRules  []PresentationRule `json:"textRules"`
}

// NodeExplanation explains what players see of one authored node.
type NodeExplanation struct {
	NodeID     string `json:"nodeId"`
	PlayerName string `json:"playerName"`
	// Visible is the node's own visibility decision. Players reach the node
	// only when it and all its ancestors are visible.
	Visible bool `json:"visible"`
	// Available is set only when the node has an availability rule. It is
	// false while an active fault blocks the capability the command needs,
	// because the coordinator then rejects the request.
	Available         *bool              `json:"available,omitempty"`
	NameRules         []PresentationRule `json:"nameRules"`
	VisibilityRules   []PresentationRule `json:"visibilityRules"`
	AvailabilityRules []PresentationRule `json:"availabilityRules"`
	Blocks            []BlockExplanation `json:"blocks,omitempty"`
}

// TerminalExplanation explains every authored node of one terminal, hidden
// ones included, in tree order.
type TerminalExplanation struct {
	TerminalID string            `json:"terminalId"`
	Nodes      []NodeExplanation `json:"nodes"`
}

// ExplainSession explains, for every terminal of the session in session order,
// what players see of each node and which display rule decides it. It uses
// the session's live values and the resolvers of the player projection, so
// the explanation matches what players see. The result shares no memory with
// the session.
func ExplainSession(session domain.Session) []TerminalExplanation {
	deviceStates := indexProjectedDeviceStates(session.Facility)
	explanations := make([]TerminalExplanation, len(session.Terminals))
	for index, terminal := range session.Terminals {
		explanations[index] = newTerminalExplainer(terminal, session.Facility, deviceStates).explain()
	}
	return explanations
}

// terminalExplainer holds the projection inputs of one terminal (see
// projectFacility) and the rules that can reach each of its targets.
type terminalExplainer struct {
	terminal        domain.Terminal
	facility        *domain.Facility
	deviceStates    map[string]projectedDeviceState
	completedBlocks map[string]string
	diagnostics     diagnosticProjection
	faults          terminalFaults
	// rewrites holds the text-after-execution rules of each block ID.
	rewrites map[string][]PresentationRule
}

func newTerminalExplainer(
	terminal domain.Terminal,
	facility *domain.Facility,
	deviceStates map[string]projectedDeviceState,
) *terminalExplainer {
	diagnostics := evaluateDiagnostics(terminal.Root, facility, terminal.ID)
	markDiagnosticPathAncestors(terminal.Root, diagnostics.forcedVisible)
	return &terminalExplainer{
		terminal:        terminal,
		facility:        facility,
		deviceStates:    deviceStates,
		completedBlocks: completedBlockTexts(terminal.CommandStates),
		diagnostics:     diagnostics,
		faults:          indexTerminalFaults(terminal.Root, facility, terminal.ID),
		rewrites:        blockRewriteRules(terminal),
	}
}

func (explainer *terminalExplainer) explain() TerminalExplanation {
	explanation := TerminalExplanation{TerminalID: explainer.terminal.ID}
	var visit func(node domain.ContentNode, root bool)
	visit = func(node domain.ContentNode, root bool) {
		explanation.Nodes = append(explanation.Nodes, explainer.explainNode(node, root))
		for _, child := range node.Children {
			visit(child, false)
		}
	}
	visit(explainer.terminal.Root, true)
	return explanation
}

func (explainer *terminalExplainer) explainNode(node domain.ContentNode, root bool) NodeExplanation {
	explanation := NodeExplanation{
		NodeID:     node.ID,
		PlayerName: resolveNodeName(node, explainer.terminal.CommandStates, explainer.deviceStates),
		Visible:    resolveNodeVisible(node, explainer.deviceStates, explainer.diagnostics, root),
		NameRules:  explainer.nameRules(node),
	}
	if !root {
		explanation.VisibilityRules = explainer.visibilityRules(node)
	}
	if node.Type == domain.NodeCommand {
		explanation.Available, explanation.AvailabilityRules = explainer.availability(node)
	}
	if node.Type == domain.NodeEntry {
		for _, block := range node.Blocks {
			explanation.Blocks = append(explanation.Blocks, explainer.explainBlock(block))
		}
	}
	return explanation
}

// nameRules mirrors resolveNodeName: a single matching state name outranks
// the frozen name of an executed command, which outranks the authored name.
func (explainer *terminalExplainer) nameRules(node domain.ContentNode) []PresentationRule {
	rules := []PresentationRule{textRule(PresentationRuleKindAuthoredName, PresentationCondition{}, node.Name, true)}
	deciding := PresentationRuleKindAuthoredName
	if node.Type == domain.NodeCommand {
		if state, executed := explainer.terminal.CommandStates[node.ID]; executed {
			rules = append(rules, explainer.commandRule(PresentationRuleKindNameAfterExecution, node.ID, state.CompletedName, true))
			deciding = PresentationRuleKindNameAfterExecution
		} else if node.StateChange != nil {
			rules = append(rules, explainer.commandRule(PresentationRuleKindNameAfterExecution, node.ID, node.StateChange.CompletedName, false))
		}
	}
	rules = append(rules, explainer.variantRules(PresentationRuleKindStateName, node.FacilityNameVariants)...)
	if _, ok := matchingFacilityText(node.FacilityNameVariants, explainer.deviceStates); ok {
		deciding = PresentationRuleKindStateName
	}
	markPlayersSee(rules, deciding)
	return rules
}

// visibilityRules mirrors resolveNodeVisible for a node below the root: a
// fault that exposes the node or one of its descendants outranks the node's
// state visibility.
func (explainer *terminalExplainer) visibilityRules(node domain.ContentNode) []PresentationRule {
	var rules []PresentationRule
	if node.VisibleWhen != nil {
		rules = append(rules, effectRule(PresentationRuleKindStateVisibility, deviceCondition(*node.VisibleWhen),
			PresentationRuleEffectVisible, facilityEqualityMatches(*node.VisibleWhen, explainer.deviceStates)))
	}
	for _, index := range explainer.faults.exposing[node.ID] {
		rules = append(rules, explainer.faultRule(PresentationRuleKindDiagnosticForcedVisibility, index, PresentationRuleEffectVisible))
	}
	switch {
	case explainer.diagnostics.forcedVisible[node.ID]:
		markPlayersSee(rules, PresentationRuleKindDiagnosticForcedVisibility)
	case node.VisibleWhen != nil:
		markPlayersSee(rules, PresentationRuleKindStateVisibility)
	}
	return rules
}

// availability combines the projected availability of a command with the
// active faults that block the capability running it needs. A command
// without an availability rule reports no availability.
func (explainer *terminalExplainer) availability(command domain.ContentNode) (*bool, []PresentationRule) {
	var rules []PresentationRule
	if command.AvailableWhen != nil {
		rules = append(rules, effectRule(PresentationRuleKindStateAvailability, deviceCondition(*command.AvailableWhen),
			PresentationRuleEffectAvailable, facilityEqualityMatches(*command.AvailableWhen, explainer.deviceStates)))
	}
	capability := domain.CommandFacilityCapability(command)
	for _, index := range explainer.faults.blocking[capability] {
		rules = append(rules, explainer.faultRule(PresentationRuleKindDiagnosticCapabilityBlock, index, PresentationRuleEffectUnavailable))
	}
	if len(rules) == 0 {
		return nil, nil
	}

	projected := resolveNodeAvailable(command, explainer.deviceStates)
	blocked := explainer.diagnostics.blockedCapabilities[capability]
	switch {
	case blocked:
		markPlayersSee(rules, PresentationRuleKindDiagnosticCapabilityBlock)
	case command.AvailableWhen != nil:
		markPlayersSee(rules, PresentationRuleKindStateAvailability)
	}
	return new((projected == nil || *projected) && !blocked), rules
}

// explainBlock mirrors resolveBlockText: a single fault substitution outranks
// a single matching state text, which outranks a single executed rewrite,
// which outranks the initial text.
func (explainer *terminalExplainer) explainBlock(block domain.EntryContentBlock) BlockExplanation {
	rules := []PresentationRule{textRule(PresentationRuleKindInitialText, PresentationCondition{}, block.InitialText, true)}
	deciding := PresentationRuleKindInitialText
	rules = append(rules, explainer.rewrites[block.ID]...)
	if _, ok := explainer.completedBlocks[block.ID]; ok {
		deciding = PresentationRuleKindTextAfterExecution
	}
	rules = append(rules, explainer.variantRules(PresentationRuleKindStateText, block.FacilityTextVariants)...)
	if _, ok := matchingFacilityText(block.FacilityTextVariants, explainer.deviceStates); ok {
		deciding = PresentationRuleKindStateText
	}
	for _, substitution := range explainer.faults.substitutions[block.ID] {
		condition := explainer.facility.Conditions[substitution.condition]
		rules = append(rules, textRule(PresentationRuleKindDiagnosticSubstitution, diagnosticCondition(condition.ID),
			substitution.text, condition.CurrentActive))
	}
	if _, ok := explainer.diagnostics.recordSubstitutions[block.ID]; ok {
		deciding = PresentationRuleKindDiagnosticSubstitution
	}
	markPlayersSee(rules, deciding)
	return BlockExplanation{
		BlockID:    block.ID,
		PlayerText: resolveBlockText(block, explainer.completedBlocks, explainer.deviceStates, explainer.diagnostics),
		TextRules:  rules,
	}
}

func (explainer *terminalExplainer) variantRules(
	kind PresentationRuleKind,
	variants []domain.FacilityTextVariant,
) []PresentationRule {
	rules := make([]PresentationRule, 0, len(variants))
	for _, variant := range variants {
		rules = append(rules, textRule(kind, deviceCondition(variant.When), variant.Text,
			facilityEqualityMatches(variant.When, explainer.deviceStates)))
	}
	return rules
}

func (explainer *terminalExplainer) commandRule(
	kind PresentationRuleKind,
	commandID string,
	text string,
	executed bool,
) PresentationRule {
	return textRule(kind, commandCondition(explainer.terminal.ID, commandID), text, executed)
}

func (explainer *terminalExplainer) faultRule(
	kind PresentationRuleKind,
	conditionIndex int,
	effect PresentationRuleEffect,
) PresentationRule {
	condition := explainer.facility.Conditions[conditionIndex]
	return effectRule(kind, diagnosticCondition(condition.ID), effect, condition.CurrentActive)
}

// terminalFaults indexes, for one terminal, the faults whose effects can
// reach each target, whether or not they are active. Faults are facility
// condition indices in facility order. The filters are those of
// evaluateDiagnostics.
type terminalFaults struct {
	// exposing maps a node ID to the faults exposing the node or one of its
	// descendants, because exposing a node also exposes its folders
	// (markDiagnosticPathAncestors).
	exposing      map[string][]int
	substitutions map[string][]faultSubstitution
	blocking      map[domain.FacilityCapability][]int
}

// faultSubstitution is one fault's replacement text for an entry block.
type faultSubstitution struct {
	condition int
	text      string
}

func indexTerminalFaults(authored domain.ContentNode, facility *domain.Facility, terminalID string) terminalFaults {
	faults := terminalFaults{
		exposing:      make(map[string][]int),
		substitutions: make(map[string][]faultSubstitution),
		blocking:      make(map[domain.FacilityCapability][]int),
	}
	if facility == nil {
		return faults
	}
	referencedDevices := terminalReferencedDevices(authored, facility)
	paths := make(map[string][]int)
	for index, condition := range facility.Conditions {
		if (condition.Device == nil) == (condition.Terminal == nil) {
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
				paths[nodeID] = appendCondition(paths[nodeID], index)
			case effect.RecordSubstitution != nil && diagnosticTargetApplies(condition, conditionApplies, effect.RecordSubstitution.TerminalID, terminalID):
				blockID := effect.RecordSubstitution.BlockID
				faults.substitutions[blockID] = append(faults.substitutions[blockID],
					faultSubstitution{condition: index, text: effect.RecordSubstitution.ReplacementText})
			case effect.CapabilityBlock != nil && conditionApplies && validProjectedCapability(effect.CapabilityBlock.Capability):
				capability := effect.CapabilityBlock.Capability
				faults.blocking[capability] = appendCondition(faults.blocking[capability], index)
			}
		}
	}
	collectExposingFaults(authored, paths, faults.exposing)
	return faults
}

// appendCondition appends a condition index unless it is already last, which
// keeps one rule per fault when a fault repeats an effect.
func appendCondition(conditions []int, index int) []int {
	if last := len(conditions) - 1; last >= 0 && conditions[last] == index {
		return conditions
	}
	return append(conditions, index)
}

// collectExposingFaults records, for the node and each descendant, the sorted
// faults exposing it or a node below it, and returns the node's.
func collectExposingFaults(node domain.ContentNode, paths, exposing map[string][]int) []int {
	conditions := slices.Clone(paths[node.ID])
	for _, child := range node.Children {
		conditions = append(conditions, collectExposingFaults(child, paths, exposing)...)
	}
	slices.Sort(conditions)
	conditions = slices.Compact(conditions)
	if len(conditions) != 0 {
		exposing[node.ID] = conditions
	}
	return conditions
}

// blockRewriteRules lists, for each block ID, the commands of the terminal
// that rewrite it: executed commands, by command ID, with the text frozen at
// execution, then the other commands, in tree order, with the text they
// would write.
func blockRewriteRules(terminal domain.Terminal) map[string][]PresentationRule {
	rewrites := make(map[string][]PresentationRule)
	add := func(commandID string, change domain.EntryContentChange, executed bool) {
		rewrites[change.BlockID] = append(rewrites[change.BlockID], textRule(PresentationRuleKindTextAfterExecution,
			commandCondition(terminal.ID, commandID), change.CompletedText, executed))
	}
	for _, commandID := range slices.Sorted(maps.Keys(terminal.CommandStates)) {
		if change := terminal.CommandStates[commandID].EntryContentChange; change != nil {
			add(commandID, *change, true)
		}
	}
	var visit func(domain.ContentNode)
	visit = func(node domain.ContentNode) {
		if _, executed := terminal.CommandStates[node.ID]; !executed && node.Type == domain.NodeCommand &&
			node.StateChange != nil && node.StateChange.EntryContentChange != nil {
			add(node.ID, *node.StateChange.EntryContentChange, false)
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(terminal.Root)
	return rewrites
}

// markPlayersSee marks the first rule of the deciding kind. A state
// visibility or availability rule decides whether or not its condition
// holds; any other rule decides only while it holds.
func markPlayersSee(rules []PresentationRule, deciding PresentationRuleKind) {
	decidesByAbsence := deciding == PresentationRuleKindStateVisibility ||
		deciding == PresentationRuleKindStateAvailability
	for index := range rules {
		if rule := &rules[index]; rule.Kind == deciding && (rule.ConditionHolds || decidesByAbsence) {
			rule.PlayersSee = true
			return
		}
	}
}

func textRule(kind PresentationRuleKind, condition PresentationCondition, text string, holds bool) PresentationRule {
	return PresentationRule{
		Kind: kind, Precedence: rulePrecedence(kind), Condition: condition, Text: text, ConditionHolds: holds,
	}
}

func effectRule(
	kind PresentationRuleKind,
	condition PresentationCondition,
	effect PresentationRuleEffect,
	holds bool,
) PresentationRule {
	return PresentationRule{
		Kind: kind, Precedence: rulePrecedence(kind), Condition: condition, Effect: effect, ConditionHolds: holds,
	}
}

// rulePrecedence ranks a kind within its target, from 1 for the lowest.
func rulePrecedence(kind PresentationRuleKind) uint32 {
	switch kind {
	case PresentationRuleKindAuthoredName, PresentationRuleKindStateVisibility,
		PresentationRuleKindStateAvailability, PresentationRuleKindInitialText:
		return 1
	case PresentationRuleKindNameAfterExecution, PresentationRuleKindDiagnosticForcedVisibility,
		PresentationRuleKindDiagnosticCapabilityBlock, PresentationRuleKindTextAfterExecution:
		return 2
	case PresentationRuleKindStateName, PresentationRuleKindStateText:
		return 3
	case PresentationRuleKindDiagnosticSubstitution:
		return 4
	default:
		return 0
	}
}

func deviceCondition(equality domain.FacilityStateEquality) PresentationCondition {
	return PresentationCondition{
		DeviceState: &domain.FacilityStateEquality{DeviceID: equality.DeviceID, StateID: equality.StateID},
	}
}

func commandCondition(terminalID, commandID string) PresentationCondition {
	return PresentationCondition{CommandExecuted: &CommandExecutedCondition{TerminalID: terminalID, CommandID: commandID}}
}

func diagnosticCondition(conditionID string) PresentationCondition {
	return PresentationCondition{DiagnosticActive: &DiagnosticActiveCondition{ConditionID: conditionID}}
}
