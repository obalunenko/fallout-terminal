package live

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/stretchr/testify/require"
)

// explainDemoBudget is the Go-time budget for explaining a demo-sized session
// (plan.md, Performance Goals).
const explainDemoBudget = 50 * time.Millisecond

// Precedence ranks per target, lowest first (data-model.md, FR-010).
var (
	nameRuleRanks = map[PresentationRuleKind]int{
		PresentationRuleKindAuthoredName:       0,
		PresentationRuleKindNameAfterExecution: 1,
		PresentationRuleKindStateName:          2,
	}
	textRuleRanks = map[PresentationRuleKind]int{
		PresentationRuleKindInitialText:            0,
		PresentationRuleKindTextAfterExecution:     1,
		PresentationRuleKindStateText:              2,
		PresentationRuleKindDiagnosticSubstitution: 3,
	}
	visibilityRuleRanks = map[PresentationRuleKind]int{
		PresentationRuleKindStateVisibility:            0,
		PresentationRuleKindDiagnosticForcedVisibility: 1,
	}
	availabilityRuleRanks = map[PresentationRuleKind]int{
		PresentationRuleKindStateAvailability:         0,
		PresentationRuleKindDiagnosticCapabilityBlock: 1,
	}
)

type explanationCase struct {
	name    string
	session domain.Session
}

// TestExplainSessionMatchesPlayerProjection checks that, for every terminal of
// every shipped and browser fixture session, the rule marked as what players
// see produces exactly the projected name, visibility, availability, and block
// text. Each session is also explained under every single device-state and
// fault change, with every command executed, with none executed, and without
// its facility, so every rule kind gets to win and to lose.
func TestExplainSessionMatchesPlayerProjection(t *testing.T) {
	t.Parallel()

	t.Run("sessions", func(t *testing.T) {
		t.Parallel()
		requireExplanationParity(t, repositoryExplanationSessions(t))
	})
	t.Run("browser fixtures", func(t *testing.T) {
		t.Parallel()
		requireExplanationParity(t, browserFixtureExplanationSessions(t))
	})
}

func TestExplainSessionRanksDemoEvacuationCommandRules(t *testing.T) {
	t.Parallel()

	const (
		commandName  = "Открыть выход и подать сигнал"
		doorID       = "vault-door-main"
		gridID       = "power-grid-main"
		gridFaultID  = "grid-unpowered"
		doorFaultID  = "door-offline"
		executedName = "Выход открыт"
		openName     = "Выход уже открыт"
	)
	demo := explanationDemoSession(t)
	terminalID, commandID := explanationCommandByName(t, demo, commandName)
	execute := func(t *testing.T, session *domain.Session) {
		terminal := explanationTerminal(t, session, terminalID)
		node := explanationNode(t, terminal.Root, commandID)
		if terminal.CommandStates == nil {
			terminal.CommandStates = make(map[string]domain.CommandExecutionState)
		}
		terminal.CommandStates[commandID] = domain.CommandExecutionState{
			CompletedName: node.StateChange.CompletedName, ResultText: node.Text,
			EntryContentChange: node.StateChange.EntryContentChange,
		}
	}

	tests := []struct {
		name                 string
		mutate               func(*testing.T, *domain.Session)
		wantName             string
		wantNameRule         PresentationRuleKind
		wantAvailable        bool
		wantAvailabilityRule PresentationRuleKind
	}{
		{
			name:     "as shipped",
			mutate:   func(*testing.T, *domain.Session) {},
			wantName: commandName, wantNameRule: PresentationRuleKindAuthoredName,
			wantAvailable: true, wantAvailabilityRule: PresentationRuleKindStateAvailability,
		},
		{
			name:     "executed with the door still sealed",
			mutate:   execute,
			wantName: executedName, wantNameRule: PresentationRuleKindNameAfterExecution,
			wantAvailable: true, wantAvailabilityRule: PresentationRuleKindStateAvailability,
		},
		{
			name: "executed with the door open",
			mutate: func(t *testing.T, session *domain.Session) {
				execute(t, session)
				explanationDevice(t, session, doorID).CurrentStateID = "open"
			},
			wantName: openName, wantNameRule: PresentationRuleKindStateName,
			wantAvailable: true, wantAvailabilityRule: PresentationRuleKindStateAvailability,
		},
		{
			name: "power grid offline",
			mutate: func(t *testing.T, session *domain.Session) {
				explanationDevice(t, session, gridID).CurrentStateID = "offline"
			},
			wantName: commandName, wantNameRule: PresentationRuleKindAuthoredName,
			wantAvailable: false, wantAvailabilityRule: PresentationRuleKindStateAvailability,
		},
		{
			name: "power grid fault blocks execution",
			mutate: func(t *testing.T, session *domain.Session) {
				explanationCondition(t, session, gridFaultID).CurrentActive = true
			},
			wantName: commandName, wantNameRule: PresentationRuleKindAuthoredName,
			wantAvailable: false, wantAvailabilityRule: PresentationRuleKindDiagnosticCapabilityBlock,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := domain.CloneSession(demo)
			test.mutate(t, &session)
			terminal := explanationTerminal(t, &session, terminalID)
			terminalExplanation := explanationForTerminal(t, ExplainSession(session), terminalID)
			requireTerminalExplanation(t, session, *terminal, terminalExplanation)
			node := explanationForNode(t, terminalExplanation, commandID)

			require.Equal(t, []PresentationRuleKind{
				PresentationRuleKindAuthoredName,
				PresentationRuleKindNameAfterExecution,
				PresentationRuleKindStateName,
			}, explanationRuleKinds(node.NameRules))
			afterExecution, stateName := node.NameRules[1], node.NameRules[2]
			require.Greater(t, stateName.Precedence, afterExecution.Precedence,
				"the state name must outrank the name after execution")
			require.Equal(t, executedName, afterExecution.Text)
			require.Equal(t, &CommandExecutedCondition{TerminalID: terminalID, CommandID: commandID},
				afterExecution.Condition.CommandExecuted)
			require.Equal(t, openName, stateName.Text)
			requireDeviceCondition(t, stateName.Condition, doorID, "open")

			require.Equal(t, test.wantName, node.PlayerName)
			require.Equal(t, test.wantNameRule, explanationMarked(t, node.NameRules).Kind)

			gridRule := explanationRuleWithKind(t, node.AvailabilityRules, PresentationRuleKindStateAvailability)
			requireDeviceCondition(t, gridRule.Condition, gridID, "online")
			require.Equal(t, PresentationRuleEffectAvailable, gridRule.Effect)
			require.Subset(t, explanationRuleSummaries(node.AvailabilityRules), []string{
				"diagnostic:" + doorFaultID + " → ",
				"diagnostic:" + gridFaultID + " → ",
			}, "faults that block command execution on this terminal must be listed")
			require.NotNil(t, node.Available)
			require.Equal(t, test.wantAvailable, *node.Available)
			require.Equal(t, test.wantAvailabilityRule, explanationMarked(t, node.AvailabilityRules).Kind)
		})
	}
}

// TestExplainSessionListsLegacyStateChangesNextToDeviceRules covers FR-012:
// legacy command state changes appear as ordinary rules beside device rules.
func TestExplainSessionListsLegacyStateChangesNextToDeviceRules(t *testing.T) {
	t.Parallel()

	demo := explanationDemoSession(t)
	terminalID, routeCommandID := explanationCommandByName(t, demo, "Активировать маршрут выхода")
	_, evacuationCommandID := explanationCommandByName(t, demo, "Открыть выход и подать сигнал")
	terminal := explanationTerminal(t, &demo, terminalID)
	require.Contains(t, terminal.CommandStates, routeCommandID, "the demo ships with the route command executed")
	entryID := explanationNodeIDByName(t, terminal.Root, "СВОДКА ЭВАКУАЦИИ")
	entry := explanationForNode(t, explanationForTerminal(t, ExplainSession(demo), terminalID), entryID)
	blocks := make(map[string]BlockExplanation, len(entry.Blocks))
	for _, block := range entry.Blocks {
		blocks[block.BlockID] = block
	}

	heading := blocks["b_vault_shutdown_heading"]
	require.Equal(t, "МАРШРУТ ВЫХОДА: ОСВЕЩЁН // УКАЗАТЕЛИ АКТИВНЫ", heading.PlayerText)
	marked := explanationMarked(t, heading.TextRules)
	require.Equal(t, PresentationRuleKindTextAfterExecution, marked.Kind)
	require.Equal(t, &CommandExecutedCondition{TerminalID: terminalID, CommandID: routeCommandID},
		marked.Condition.CommandExecuted)

	body := blocks["b_vault_shutdown_body"]
	pending := explanationRuleWithKind(t, body.TextRules, PresentationRuleKindTextAfterExecution)
	require.Equal(t, &CommandExecutedCondition{TerminalID: terminalID, CommandID: evacuationCommandID},
		pending.Condition.CommandExecuted)
	require.False(t, pending.ConditionHolds)
	require.Equal(t, PresentationRuleKindInitialText, explanationMarked(t, body.TextRules).Kind)

	door := blocks["b_vault_door_status"]
	require.Equal(t, []PresentationRuleKind{
		PresentationRuleKindInitialText,
		PresentationRuleKindStateText,
	}, explanationRuleKinds(door.TextRules))
	requireDeviceCondition(t, door.TextRules[1].Condition, "vault-door-main", "open")
	require.Equal(t, "ГЕРМОДВЕРЬ: ОТКРЫТА // МАРШРУТ СВОБОДЕН", door.TextRules[1].Text)
}

// TestExplainSessionResolvesCancellationsAndForcedAncestors covers the
// precedence edge cases the corpus may not exercise: two executed commands
// rewriting one block cancel each other, a fault that exposes a diagnostic
// node also exposes its hidden folder, and an active capability block makes a
// command without an availability binding unavailable.
func TestExplainSessionResolvesCancellationsAndForcedAncestors(t *testing.T) {
	t.Parallel()

	session := explanationEdgeCaseSession()
	terminal := session.Terminals[0]
	explained := explanationForTerminal(t, ExplainSession(session), terminal.ID)
	requireTerminalExplanation(t, session, terminal, explained)

	folder := explanationForNode(t, explained, "maintenance")
	require.True(t, folder.Visible, "a fault exposing a child must expose its hidden folder")
	forced := explanationMarked(t, folder.VisibilityRules)
	require.Equal(t, PresentationRuleKindDiagnosticForcedVisibility, forced.Kind)
	require.Equal(t, &DiagnosticActiveCondition{ConditionID: "door-fault"}, forced.Condition.DiagnosticActive)
	state := explanationRuleWithKind(t, folder.VisibilityRules, PresentationRuleKindStateVisibility)
	require.False(t, state.ConditionHolds)
	require.False(t, state.PlayersSee)

	status := explanationForNode(t, explained, "status")
	require.Len(t, status.Blocks, 1)
	summary := status.Blocks[0]
	require.Equal(t, "SUMMARY: IDLE", summary.PlayerText, "conflicting executed rewrites cancel each other")
	require.Equal(t, PresentationRuleKindInitialText, explanationMarked(t, summary.TextRules).Kind)
	for _, rule := range summary.TextRules {
		if rule.Kind == PresentationRuleKindTextAfterExecution {
			require.True(t, rule.ConditionHolds)
			require.False(t, rule.PlayersSee)
		}
	}

	first := explanationForNode(t, explained, "first")
	require.NotNil(t, first.Available)
	require.False(t, *first.Available)
	blocked := explanationMarked(t, first.AvailabilityRules)
	require.Equal(t, PresentationRuleKindDiagnosticCapabilityBlock, blocked.Kind)
	require.Equal(t, PresentationRuleEffectUnavailable, blocked.Effect)
}

func TestExplainSessionIsDeterministicAndDetached(t *testing.T) {
	t.Parallel()

	demo := explanationDemoSession(t)
	original := domain.CloneSession(demo)
	want := ExplainSession(demo)

	scribbled := ExplainSession(demo)
	for terminalIndex := range scribbled {
		for nodeIndex := range scribbled[terminalIndex].Nodes {
			node := &scribbled[terminalIndex].Nodes[nodeIndex]
			if node.Available != nil {
				*node.Available = !*node.Available
			}
			for _, rules := range [][]PresentationRule{node.NameRules, node.VisibilityRules, node.AvailabilityRules} {
				scribbleExplanationRules(rules)
			}
			for blockIndex := range node.Blocks {
				scribbleExplanationRules(node.Blocks[blockIndex].TextRules)
			}
		}
	}

	if diff := cmp.Diff(want, ExplainSession(demo)); diff != "" {
		t.Fatalf("ExplainSession(demo) changed after its previous result was modified (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(original, demo, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("ExplainSession(demo) result shares memory with its input (-want +got):\n%s", diff)
	}
}

func BenchmarkExplainDemo(b *testing.B) {
	demo := explanationDemoSession(b)
	for b.Loop() {
		ExplainSession(demo)
	}
	perOp := b.Elapsed() / time.Duration(b.N)
	b.ReportMetric(float64(perOp)/float64(time.Millisecond), "ms/op")
	if perOp > explainDemoBudget {
		b.Errorf("ExplainSession(demo) took %v per call, want at most %v", perOp, explainDemoBudget)
	}
}

func requireExplanationParity(t *testing.T, cases []explanationCase) {
	t.Helper()
	require.NotEmpty(t, cases)
	for _, base := range cases {
		for _, scenario := range explanationScenarios(base) {
			t.Run(scenario.name, func(t *testing.T) {
				t.Parallel()
				before := domain.CloneSession(scenario.session)
				got := ExplainSession(scenario.session)
				if diff := cmp.Diff(before, scenario.session, cmpopts.EquateEmpty()); diff != "" {
					t.Fatalf("ExplainSession() modified its input (-before +after):\n%s", diff)
				}
				require.Len(t, got, len(scenario.session.Terminals), "one explanation per terminal, in session order")
				for index, terminal := range scenario.session.Terminals {
					requireTerminalExplanation(t, scenario.session, terminal, got[index])
				}
			})
		}
	}
}

// requireTerminalExplanation compares one terminal explanation with the
// player projection. Visible is the node's own decision, so a node reaches
// players exactly when it and all its ancestors are visible.
func requireTerminalExplanation(
	t *testing.T,
	session domain.Session,
	terminal domain.Terminal,
	got TerminalExplanation,
) {
	t.Helper()
	require.Equal(t, terminal.ID, got.TerminalID)
	projection := projectFacility(terminal.Root, terminal.CommandStates, session.Facility, terminal.ID)
	present := indexExplanationTree(projection.Tree)
	unhidden := indexExplanationTree(projectFacility(
		explanationTreeWithoutVisibility(terminal.Root), terminal.CommandStates, session.Facility, terminal.ID,
	).Tree)

	position := 0
	var visit func(node domain.ContentNode, ancestorsVisible bool)
	visit = func(node domain.ContentNode, ancestorsVisible bool) {
		require.Lessf(t, position, len(got.Nodes), "terminal %q: explanation ends before node %q", terminal.ID, node.ID)
		explained := got.Nodes[position]
		position++
		require.Equalf(t, node.ID, explained.NodeID, "terminal %q: nodes must follow tree order", terminal.ID)
		projected, ok := unhidden[node.ID]
		require.Truef(t, ok, "node %q missing from the projection", node.ID)
		requireNodeExplanation(t, session, terminal, node, projected, projection.BlockedCapabilities, explained)

		visible := ancestorsVisible && explained.Visible
		_, reachesPlayers := present[node.ID]
		require.Equalf(t, reachesPlayers, visible,
			"terminal %q node %q: in the player projection = %t, explained visible through its ancestors = %t",
			terminal.ID, node.ID, reachesPlayers, visible)
		for _, child := range node.Children {
			visit(child, visible)
		}
	}
	visit(terminal.Root, true)
	require.Equalf(t, len(got.Nodes), position, "terminal %q: explanation lists unknown nodes", terminal.ID)
}

// explanationTarget is the node under check, with the session it belongs to.
type explanationTarget struct {
	session  domain.Session
	terminal domain.Terminal
	authored domain.ContentNode
	// projected is the node as the player projection renders it when no
	// visibility rule hides it.
	projected domain.ContentNode
	label     string
}

func requireNodeExplanation(
	t *testing.T,
	session domain.Session,
	terminal domain.Terminal,
	authored domain.ContentNode,
	projected domain.ContentNode,
	blockedCapabilities map[domain.FacilityCapability]bool,
	got NodeExplanation,
) {
	t.Helper()
	target := explanationTarget{
		session: session, terminal: terminal, authored: authored, projected: projected,
		label: fmt.Sprintf("terminal %q node %q", terminal.ID, authored.ID),
	}
	requireNameExplanation(t, target, got)
	requireVisibilityExplanation(t, target, got)
	requireAvailabilityExplanation(t, target, blockedCapabilities[explanationCommandCapability(authored)], got)
	requireBlockExplanations(t, target, got.Blocks)
}

func requireNameExplanation(t *testing.T, target explanationTarget, got NodeExplanation) {
	t.Helper()
	authored, label := target.authored, target.label
	marked := requireExplanationRules(t, target.session, target.terminal, label+" name rules", got.NameRules, nameRuleRanks)
	require.Equalf(t, []string{"always → " + authored.Name},
		explanationRuleSummariesOfKind(got.NameRules, PresentationRuleKindAuthoredName), "%s: authored name", label)
	var wantAfterExecution []string
	commandKey := explanationCommandKey(target.terminal.ID, authored.ID)
	if completed, executed := target.terminal.CommandStates[authored.ID]; executed && authored.Type == domain.NodeCommand {
		wantAfterExecution = []string{commandKey + " → " + completed.CompletedName}
	} else if authored.Type == domain.NodeCommand && authored.StateChange != nil {
		wantAfterExecution = []string{commandKey + " → " + authored.StateChange.CompletedName}
	}
	require.Equalf(t, wantAfterExecution,
		explanationRuleSummariesOfKind(got.NameRules, PresentationRuleKindNameAfterExecution), "%s: name after execution", label)
	require.Equalf(t, explanationVariantSummaries(authored.FacilityNameVariants),
		explanationRuleSummariesOfKind(got.NameRules, PresentationRuleKindStateName), "%s: state names", label)
	require.NotNilf(t, marked, "%s: no name rule is marked", label)
	require.Equalf(t, marked.Text, got.PlayerName, "%s: marked name rule", label)
	require.Equalf(t, target.projected.Name, got.PlayerName, "%s: player name", label)
}

func requireVisibilityExplanation(t *testing.T, target explanationTarget, got NodeExplanation) {
	t.Helper()
	authored, label := target.authored, target.label
	root := authored.ID == target.terminal.Root.ID
	marked := requireExplanationRules(t, target.session, target.terminal, label+" visibility rules",
		got.VisibilityRules, visibilityRuleRanks)
	var wantStateVisibility []string
	if authored.VisibleWhen != nil && !root {
		wantStateVisibility = []string{explanationDeviceKey(*authored.VisibleWhen) + " → "}
	}
	require.Equalf(t, wantStateVisibility,
		explanationRuleSummariesOfKind(got.VisibilityRules, PresentationRuleKindStateVisibility), "%s: state visibility", label)
	require.Subsetf(t,
		explanationRuleSummariesOfKind(got.VisibilityRules, PresentationRuleKindDiagnosticForcedVisibility),
		explanationDiagnosticPaths(target.session.Facility, target.terminal.ID, authored.ID),
		"%s: faults exposing the node", label)
	wantVisible := true
	if marked != nil && marked.Kind == PresentationRuleKindStateVisibility {
		wantVisible = marked.ConditionHolds
	}
	require.Equalf(t, wantVisible, got.Visible, "%s: visible", label)
	if root {
		require.Truef(t, got.Visible, "%s: the root is always visible", label)
	}
}

// requireAvailabilityExplanation checks availability, which only commands
// have: the projected availability, made false by an active block of the
// capability running the command needs.
func requireAvailabilityExplanation(t *testing.T, target explanationTarget, blocked bool, got NodeExplanation) {
	t.Helper()
	authored, label := target.authored, target.label
	if authored.Type != domain.NodeCommand {
		require.Emptyf(t, got.AvailabilityRules, "%s: only commands have availability rules", label)
		require.Nilf(t, got.Available, "%s: only commands report availability", label)
		return
	}
	marked := requireExplanationRules(t, target.session, target.terminal, label+" availability rules",
		got.AvailabilityRules, availabilityRuleRanks)
	var wantStateAvailability []string
	if authored.AvailableWhen != nil {
		wantStateAvailability = []string{explanationDeviceKey(*authored.AvailableWhen) + " → "}
	}
	require.Equalf(t, wantStateAvailability,
		explanationRuleSummariesOfKind(got.AvailabilityRules, PresentationRuleKindStateAvailability),
		"%s: state availability", label)
	if len(got.AvailabilityRules) == 0 {
		require.Nilf(t, got.Available, "%s: availability without an availability rule", label)
		require.Falsef(t, blocked, "%s: an active capability block must be listed", label)
		return
	}
	require.NotNilf(t, got.Available, "%s: availability rules without availability", label)
	projected := target.projected.Available
	require.Equalf(t, (projected == nil || *projected) && !blocked, *got.Available, "%s: available", label)
	markedAvailable := true
	if marked != nil {
		markedAvailable = marked.Kind == PresentationRuleKindStateAvailability && marked.ConditionHolds
	}
	require.Equalf(t, markedAvailable, *got.Available, "%s: marked availability rule", label)
}

// requireBlockExplanations checks block texts, which only entries with
// blocks have.
func requireBlockExplanations(t *testing.T, target explanationTarget, got []BlockExplanation) {
	t.Helper()
	authored, label := target.authored, target.label
	if authored.Type != domain.NodeEntry || len(authored.Blocks) == 0 {
		require.Emptyf(t, got, "%s: only entries with blocks have block texts", label)
		return
	}
	require.Lenf(t, got, len(authored.Blocks), "%s: blocks", label)
	texts := make([]string, len(authored.Blocks))
	for index, block := range authored.Blocks {
		explained := got[index]
		blockLabel := fmt.Sprintf("%s block %q", label, block.ID)
		require.Equalf(t, block.ID, explained.BlockID, "%s: blocks must follow authored order", label)
		marked := requireExplanationRules(t, target.session, target.terminal, blockLabel+" text rules",
			explained.TextRules, textRuleRanks)
		require.Equalf(t, []string{"always → " + block.InitialText},
			explanationRuleSummariesOfKind(explained.TextRules, PresentationRuleKindInitialText), "%s: initial text", blockLabel)
		require.ElementsMatchf(t, explanationBlockRewrites(target.terminal, block.ID),
			explanationRuleSummariesOfKind(explained.TextRules, PresentationRuleKindTextAfterExecution),
			"%s: text after execution", blockLabel)
		require.Equalf(t, explanationVariantSummaries(block.FacilityTextVariants),
			explanationRuleSummariesOfKind(explained.TextRules, PresentationRuleKindStateText), "%s: state texts", blockLabel)
		require.ElementsMatchf(t, explanationSubstitutions(target.session.Facility, target.terminal.ID, block.ID),
			explanationRuleSummariesOfKind(explained.TextRules, PresentationRuleKindDiagnosticSubstitution),
			"%s: fault substitutions", blockLabel)
		require.NotNilf(t, marked, "%s: no text rule is marked", blockLabel)
		require.Equalf(t, marked.Text, explained.PlayerText, "%s: marked text rule", blockLabel)
		texts[index] = explained.PlayerText
	}
	require.Equalf(t, target.projected.Description, strings.Join(texts, "\n\n"), "%s: block texts players see", label)
}

// requireExplanationRules checks one target's rule list and returns the rule
// marked as what players see, if any. A rule applies when its condition holds
// and no other rule of its kind also holds: competing state variants, executed
// rewrites, and fault substitutions cancel each other. Capability blocks and
// forced visibility never cancel, because a folder is exposed by every fault
// that exposes one of its descendants. The strongest applying rule decides. When none applies, a state
// visibility or availability rule still decides by its condition not holding;
// otherwise the default (visible, available) holds and nothing is marked.
func requireExplanationRules(
	t *testing.T,
	session domain.Session,
	terminal domain.Terminal,
	label string,
	rules []PresentationRule,
	ranks map[PresentationRuleKind]int,
) *PresentationRule {
	t.Helper()
	seen := make(map[string]bool, len(rules))
	holding := make(map[PresentationRuleKind]int)
	var marked *PresentationRule
	for index := range rules {
		rule := &rules[index]
		rank, ok := ranks[rule.Kind]
		require.Truef(t, ok, "%s[%d]: kind %v does not belong to this target", label, index, rule.Kind)
		requireExplanationRuleShape(t, terminal, fmt.Sprintf("%s[%d]", label, index), *rule)
		require.Equalf(t, explanationConditionHolds(session, terminal, rule.Condition), rule.ConditionHolds,
			"%s[%d] (%v): conditionHolds", label, index, rule.Kind)
		key := fmt.Sprintf("%v|%s", rule.Kind, explanationConditionKey(rule.Condition))
		require.Falsef(t, seen[key], "%s[%d]: duplicate rule %s", label, index, key)
		seen[key] = true
		if index > 0 {
			previous := rules[index-1]
			previousRank := ranks[previous.Kind]
			require.LessOrEqualf(t, previousRank, rank, "%s: rules must be ordered by precedence, lowest first", label)
			if previousRank == rank {
				require.Equalf(t, previous.Precedence, rule.Precedence, "%s: one precedence per kind", label)
			} else {
				require.Lessf(t, previous.Precedence, rule.Precedence, "%s: a higher kind needs a higher precedence", label)
			}
		}
		if rule.ConditionHolds {
			holding[rule.Kind]++
		}
		if rule.PlayersSee {
			require.Nilf(t, marked, "%s: more than one rule is marked as what players see", label)
			marked = rule
		}
	}

	applies := func(rule PresentationRule) bool {
		switch rule.Kind {
		case PresentationRuleKindDiagnosticCapabilityBlock, PresentationRuleKindDiagnosticForcedVisibility:
			return rule.ConditionHolds
		default:
			return rule.ConditionHolds && holding[rule.Kind] == 1
		}
	}
	var strongest *PresentationRule
	decidesByAbsence := false
	for index := range rules {
		if applies(rules[index]) {
			strongest = &rules[index]
		}
		switch rules[index].Kind {
		case PresentationRuleKindStateVisibility, PresentationRuleKindStateAvailability:
			decidesByAbsence = true
		}
	}
	switch {
	case strongest != nil:
		require.NotNilf(t, marked, "%s: %v decides what players see but is not marked", label, strongest.Kind)
		require.Truef(t, applies(*marked), "%s: marked %v does not apply", label, marked.Kind)
		require.Equalf(t, strongest.Precedence, marked.Precedence,
			"%s: marked %v is outranked by %v", label, marked.Kind, strongest.Kind)
	case decidesByAbsence:
		require.NotNilf(t, marked, "%s: the state rule decides by not holding but is not marked", label)
		require.Containsf(t,
			[]PresentationRuleKind{PresentationRuleKindStateVisibility, PresentationRuleKindStateAvailability},
			marked.Kind, "%s: only a state rule decides when nothing applies", label)
	default:
		require.Nilf(t, marked, "%s: no rule decides, yet one is marked", label)
	}
	return marked
}

func requireExplanationRuleShape(t *testing.T, terminal domain.Terminal, label string, rule PresentationRule) {
	t.Helper()
	condition := rule.Condition
	conditions := 0
	for _, set := range []bool{condition.DeviceState != nil, condition.CommandExecuted != nil, condition.DiagnosticActive != nil} {
		if set {
			conditions++
		}
	}
	require.LessOrEqualf(t, conditions, 1, "%s: a rule has at most one condition", label)

	switch rule.Kind {
	case PresentationRuleKindAuthoredName, PresentationRuleKindInitialText:
		require.Zerof(t, conditions, "%s: %v always applies", label, rule.Kind)
	case PresentationRuleKindNameAfterExecution, PresentationRuleKindTextAfterExecution:
		require.NotNilf(t, condition.CommandExecuted, "%s: %v needs a command condition", label, rule.Kind)
		require.Equalf(t, terminal.ID, condition.CommandExecuted.TerminalID, "%s: command states are terminal-scoped", label)
	case PresentationRuleKindStateName, PresentationRuleKindStateText,
		PresentationRuleKindStateVisibility, PresentationRuleKindStateAvailability:
		require.NotNilf(t, condition.DeviceState, "%s: %v needs a device condition", label, rule.Kind)
	default:
		require.NotNilf(t, condition.DiagnosticActive, "%s: %v needs a fault condition", label, rule.Kind)
	}

	switch rule.Kind {
	case PresentationRuleKindStateVisibility, PresentationRuleKindDiagnosticForcedVisibility:
		require.Equalf(t, PresentationRuleEffectVisible, rule.Effect, "%s: effect", label)
		require.Emptyf(t, rule.Text, "%s: visibility rules carry no text", label)
	case PresentationRuleKindStateAvailability:
		require.Equalf(t, PresentationRuleEffectAvailable, rule.Effect, "%s: effect", label)
		require.Emptyf(t, rule.Text, "%s: availability rules carry no text", label)
	case PresentationRuleKindDiagnosticCapabilityBlock:
		require.Equalf(t, PresentationRuleEffectUnavailable, rule.Effect, "%s: effect", label)
		require.Emptyf(t, rule.Text, "%s: availability rules carry no text", label)
	default:
		require.Equalf(t, PresentationRuleEffectUnspecified, rule.Effect, "%s: name and text rules carry text only", label)
	}
}

// explanationConditionHolds evaluates a rule condition against the session's
// live values, independently of the code under test.
func explanationConditionHolds(session domain.Session, terminal domain.Terminal, condition PresentationCondition) bool {
	switch {
	case condition.DeviceState != nil:
		if session.Facility == nil {
			return false
		}
		for _, device := range session.Facility.Devices {
			if device.ID == condition.DeviceState.DeviceID {
				return device.CurrentStateID == condition.DeviceState.StateID
			}
		}
		return false
	case condition.CommandExecuted != nil:
		_, ok := terminal.CommandStates[condition.CommandExecuted.CommandID]
		return ok
	case condition.DiagnosticActive != nil:
		if session.Facility == nil {
			return false
		}
		for _, fault := range session.Facility.Conditions {
			if fault.ID == condition.DiagnosticActive.ConditionID {
				return fault.CurrentActive
			}
		}
		return false
	default:
		return true
	}
}

func explanationConditionKey(condition PresentationCondition) string {
	switch {
	case condition.DeviceState != nil:
		return explanationDeviceKey(*condition.DeviceState)
	case condition.CommandExecuted != nil:
		return explanationCommandKey(condition.CommandExecuted.TerminalID, condition.CommandExecuted.CommandID)
	case condition.DiagnosticActive != nil:
		return "diagnostic:" + condition.DiagnosticActive.ConditionID
	default:
		return "always"
	}
}

func explanationDeviceKey(equality domain.FacilityStateEquality) string {
	return "device:" + equality.DeviceID + "=" + equality.StateID
}

func explanationCommandKey(terminalID, commandID string) string {
	return "command:" + terminalID + "/" + commandID
}

func explanationRuleSummaries(rules []PresentationRule) []string {
	summaries := make([]string, 0, len(rules))
	for _, rule := range rules {
		summaries = append(summaries, explanationConditionKey(rule.Condition)+" → "+rule.Text)
	}
	return summaries
}

func explanationRuleSummariesOfKind(rules []PresentationRule, kind PresentationRuleKind) []string {
	var summaries []string
	for _, rule := range rules {
		if rule.Kind == kind {
			summaries = append(summaries, explanationConditionKey(rule.Condition)+" → "+rule.Text)
		}
	}
	return summaries
}

func explanationVariantSummaries(variants []domain.FacilityTextVariant) []string {
	var summaries []string
	for _, variant := range variants {
		summaries = append(summaries, explanationDeviceKey(variant.When)+" → "+variant.Text)
	}
	return summaries
}

// explanationBlockRewrites lists every command of the terminal that rewrites
// the block: executed commands with their frozen text, others with the text
// they would write.
func explanationBlockRewrites(terminal domain.Terminal, blockID string) []string {
	var rewrites []string
	for _, commandID := range slices.Sorted(maps.Keys(terminal.CommandStates)) {
		if change := terminal.CommandStates[commandID].EntryContentChange; change != nil && change.BlockID == blockID {
			rewrites = append(rewrites, explanationCommandKey(terminal.ID, commandID)+" → "+change.CompletedText)
		}
	}
	var visit func(domain.ContentNode)
	visit = func(node domain.ContentNode) {
		if _, executed := terminal.CommandStates[node.ID]; !executed && node.Type == domain.NodeCommand &&
			node.StateChange != nil && node.StateChange.EntryContentChange != nil &&
			node.StateChange.EntryContentChange.BlockID == blockID {
			rewrites = append(rewrites,
				explanationCommandKey(terminal.ID, node.ID)+" → "+node.StateChange.EntryContentChange.CompletedText)
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(terminal.Root)
	return rewrites
}

func explanationFaultReaches(fault domain.DiagnosticCondition, targetTerminalID, terminalID string) bool {
	if targetTerminalID != terminalID {
		return false
	}
	return fault.Device != nil || (fault.Terminal != nil && fault.Terminal.TerminalID == terminalID)
}

func explanationDiagnosticPaths(facility *domain.Facility, terminalID, nodeID string) []string {
	var paths []string
	if facility == nil {
		return paths
	}
	for _, fault := range facility.Conditions {
		for _, effect := range fault.Effects {
			if path := effect.DiagnosticPath; path != nil && path.NodeID == nodeID &&
				explanationFaultReaches(fault, path.TerminalID, terminalID) {
				paths = append(paths, "diagnostic:"+fault.ID+" → ")
			}
		}
	}
	return paths
}

func explanationSubstitutions(facility *domain.Facility, terminalID, blockID string) []string {
	var substitutions []string
	if facility == nil {
		return substitutions
	}
	for _, fault := range facility.Conditions {
		for _, effect := range fault.Effects {
			if substitution := effect.RecordSubstitution; substitution != nil && substitution.BlockID == blockID &&
				explanationFaultReaches(fault, substitution.TerminalID, terminalID) {
				substitutions = append(substitutions, "diagnostic:"+fault.ID+" → "+substitution.ReplacementText)
			}
		}
	}
	return substitutions
}

// explanationCommandCapability is the capability a player request to run the
// command needs (data-model.md, available).
func explanationCommandCapability(command domain.ContentNode) domain.FacilityCapability {
	switch {
	case command.Behavior() == domain.CommandBehaviorTerminalTransition:
		return domain.FacilityCapabilityTerminalTransition
	case command.StateChange != nil && command.StateChange.FacilityAction != nil &&
		command.StateChange.FacilityAction.RecoveryProgramID != nil:
		return domain.FacilityCapabilityRunRecoveryProgram
	default:
		return domain.FacilityCapabilityExecuteCommand
	}
}

func explanationScenarios(base explanationCase) []explanationCase {
	scenarios := []explanationCase{base}
	add := func(name string, mutate func(*domain.Session)) {
		session := domain.CloneSession(base.session)
		mutate(&session)
		scenarios = append(scenarios, explanationCase{name: base.name + "/" + name, session: session})
	}
	if facility := base.session.Facility; facility != nil {
		for deviceIndex, device := range facility.Devices {
			for _, state := range device.States {
				if state.ID != device.CurrentStateID {
					add("device "+device.ID+"="+state.ID, func(session *domain.Session) {
						session.Facility.Devices[deviceIndex].CurrentStateID = state.ID
					})
				}
			}
		}
		for faultIndex, fault := range facility.Conditions {
			add(fmt.Sprintf("fault %s active=%t", fault.ID, !fault.CurrentActive), func(session *domain.Session) {
				session.Facility.Conditions[faultIndex].CurrentActive = !fault.CurrentActive
			})
		}
		for _, active := range []bool{true, false} {
			add(fmt.Sprintf("every fault active=%t", active), func(session *domain.Session) {
				for index := range session.Facility.Conditions {
					session.Facility.Conditions[index].CurrentActive = active
				}
			})
		}
		add("without facility", explanationWithoutFacility)
	}
	add("every command executed", explanationExecuteEveryCommand)
	add("no command executed", func(session *domain.Session) {
		for index := range session.Terminals {
			session.Terminals[index].CommandStates = nil
		}
	})
	return scenarios
}

func explanationExecuteEveryCommand(session *domain.Session) {
	for index := range session.Terminals {
		terminal := &session.Terminals[index]
		var visit func(domain.ContentNode)
		visit = func(node domain.ContentNode) {
			if _, executed := terminal.CommandStates[node.ID]; !executed &&
				node.Type == domain.NodeCommand && node.StateChange != nil {
				if terminal.CommandStates == nil {
					terminal.CommandStates = make(map[string]domain.CommandExecutionState)
				}
				state := domain.CommandExecutionState{CompletedName: node.StateChange.CompletedName, ResultText: node.Text}
				if change := node.StateChange.EntryContentChange; change != nil {
					state.EntryContentChange = &domain.EntryContentChange{BlockID: change.BlockID, CompletedText: change.CompletedText}
				}
				terminal.CommandStates[node.ID] = state
			}
			for _, child := range node.Children {
				visit(child)
			}
		}
		visit(terminal.Root)
	}
}

// explanationWithoutFacility turns a session into a legacy one: no facility
// and no bindings, only authored names, texts, and command state changes.
func explanationWithoutFacility(session *domain.Session) {
	session.Facility = nil
	var strip func(*domain.ContentNode)
	strip = func(node *domain.ContentNode) {
		node.VisibleWhen, node.AvailableWhen, node.FacilityNameVariants = nil, nil, nil
		for index := range node.Blocks {
			node.Blocks[index].FacilityTextVariants = nil
		}
		if node.StateChange != nil {
			node.StateChange.FacilityAction = nil
		}
		for index := range node.Children {
			strip(&node.Children[index])
		}
	}
	for index := range session.Terminals {
		strip(&session.Terminals[index].Root)
	}
}

func explanationTreeWithoutVisibility(node domain.ContentNode) domain.ContentNode {
	clone := domain.CloneContentNode(node)
	var strip func(*domain.ContentNode)
	strip = func(node *domain.ContentNode) {
		node.VisibleWhen = nil
		for index := range node.Children {
			strip(&node.Children[index])
		}
	}
	strip(&clone)
	return clone
}

func indexExplanationTree(root domain.ContentNode) map[string]domain.ContentNode {
	nodes := make(map[string]domain.ContentNode)
	var visit func(domain.ContentNode)
	visit = func(node domain.ContentNode) {
		nodes[node.ID] = node
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(root)
	return nodes
}

func repositoryExplanationSessions(t testing.TB) []explanationCase {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "sessions", "*.json"))
	require.NoError(t, err)
	var cases []explanationCase
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		if _, err := domain.DecodePlayerConfig(data); err == nil {
			continue
		}
		session, err := domain.DecodeSession(data)
		require.NoErrorf(t, err, "DecodeSession(%s)", path)
		cases = append(cases, explanationCase{name: "sessions/" + filepath.Base(path), session: session})
	}
	require.NotEmpty(t, cases, "no session files under sessions/")
	return cases
}

// browserFixtureExplanationSessions builds the browser suite's facility
// fixture sessions with the Node.js runtime that suite already requires, so
// the corpus follows the fixtures without a Go copy of them. The sessions are
// decoded without validation: the module predates the rule that condition
// recovery must name a recovery transition, which no presentation rule reads.
// The fixture server's own sessions live in package main and are reached by
// the browser journeys instead.
func browserFixtureExplanationSessions(t testing.TB) []explanationCase {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH; the browser fixture sessions need Node.js (see .nvmrc)")
	}
	module, err := filepath.Abs(filepath.Join("..", "..", "tests", "browser", "fixtures", "facility-session.mjs"))
	require.NoError(t, err)
	const script = `
const fixtures = await import(process.argv[1]);
process.stdout.write(JSON.stringify({
  'authoring': fixtures.buildFacilityAuthoringSession(),
  'multi-terminal': fixtures.buildMultiTerminalFacilitySession(),
  'diagnostics-all-active': fixtures.buildDiagnosticFacilitySession(),
  'diagnostics-none-active': fixtures.buildDiagnosticFacilitySession({ activeConditions: [] }),
  'lifecycle': fixtures.buildFacilityLifecycleFixture().session,
}));`
	command := exec.CommandContext(t.Context(), node, "--input-type=module", "--eval", script,
		(&url.URL{Scheme: "file", Path: filepath.ToSlash(module)}).String())
	output, err := command.Output()
	require.NoErrorf(t, err, "building browser fixture sessions with node")

	var documents map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(output, &documents))
	cases := make([]explanationCase, 0, len(documents))
	for _, name := range slices.Sorted(maps.Keys(documents)) {
		var session domain.Session
		require.NoErrorf(t, json.Unmarshal(documents[name], &session), "decoding browser fixture %s", name)
		cases = append(cases, explanationCase{name: "browser/" + name, session: domain.NormalizeTerminalGroups(session)})
	}
	return cases
}

func explanationDemoSession(t testing.TB) domain.Session {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "sessions", "demo.json"))
	require.NoError(t, err)
	session, err := domain.DecodeSession(data)
	require.NoError(t, err)
	return session
}

// explanationEdgeCaseSession has a folder hidden by a sealed door that holds a
// diagnostic entry exposed by an active door fault, an entry block rewritten
// by two executed commands, and a fault that blocks command execution.
func explanationEdgeCaseSession() domain.Session {
	whenOpen := domain.FacilityStateEquality{DeviceID: "door", StateID: "open"}
	return domain.Session{
		Version: 1, Name: "Explanation edge cases",
		Terminals: []domain.Terminal{{
			ID: "terminal", Name: "Terminal",
			Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
				{
					ID: "maintenance", Type: domain.NodeFolder, Name: "MAINTENANCE", VisibleWhen: &whenOpen,
					Children: []domain.ContentNode{{
						ID: "fault-report", Type: domain.NodeEntry, Name: "FAULT REPORT",
						Blocks: []domain.EntryContentBlock{{ID: "fault-body", InitialText: "DOOR DRIVE FAULT"}},
					}},
				},
				{
					ID: "status", Type: domain.NodeEntry, Name: "STATUS",
					Blocks: []domain.EntryContentBlock{{ID: "summary", InitialText: "SUMMARY: IDLE"}},
				},
				{
					ID: "first", Type: domain.NodeCommand, Name: "FIRST", Text: "First done.",
					StateChange: &domain.StateChangeConfig{
						CompletedName: "FIRST DONE", ConfirmationText: "Run first?",
						EntryContentChange: &domain.EntryContentChange{BlockID: "summary", CompletedText: "SUMMARY: FIRST"},
					},
				},
				{
					ID: "second", Type: domain.NodeCommand, Name: "SECOND", Text: "Second done.",
					StateChange: &domain.StateChangeConfig{
						CompletedName: "SECOND DONE", ConfirmationText: "Run second?",
						EntryContentChange: &domain.EntryContentChange{BlockID: "summary", CompletedText: "SUMMARY: SECOND"},
					},
				},
			}},
			CommandStates: map[string]domain.CommandExecutionState{
				"first": {
					CompletedName: "FIRST DONE", ResultText: "First done.",
					EntryContentChange: &domain.EntryContentChange{BlockID: "summary", CompletedText: "SUMMARY: FIRST"},
				},
				"second": {
					CompletedName: "SECOND DONE", ResultText: "Second done.",
					EntryContentChange: &domain.EntryContentChange{BlockID: "summary", CompletedText: "SUMMARY: SECOND"},
				},
			},
		}},
		Facility: &domain.Facility{
			Revision: 3,
			Devices: []domain.FacilityDevice{{
				ID: "door", Name: "Door", Kind: domain.FacilityDeviceKindDoor,
				InitialStateID: "sealed", CurrentStateID: "sealed",
				States: []domain.FacilityDeviceState{{ID: "sealed", Name: "Sealed"}, {ID: "open", Name: "Open"}},
			}},
			Conditions: []domain.DiagnosticCondition{{
				ID: "door-fault", Name: "Door fault", Category: domain.DiagnosticConditionCategoryOffline,
				Device: &domain.DiagnosticDeviceScope{DeviceID: "door"}, InitialActive: true, CurrentActive: true,
				Effects: []domain.DiagnosticEffect{
					{DiagnosticPath: &domain.DiagnosticPathEffect{TerminalID: "terminal", NodeID: "fault-report"}},
					{CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityExecuteCommand}},
				},
				Recovery: []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
			}},
			RecoveryPrograms: []domain.RecoveryProgram{},
		},
	}
}

func explanationCommandByName(t *testing.T, session domain.Session, name string) (string, string) {
	t.Helper()
	for _, terminal := range session.Terminals {
		var found string
		var visit func(domain.ContentNode)
		visit = func(node domain.ContentNode) {
			if node.Type == domain.NodeCommand && node.Name == name {
				found = node.ID
			}
			for _, child := range node.Children {
				visit(child)
			}
		}
		visit(terminal.Root)
		if found != "" {
			return terminal.ID, found
		}
	}
	t.Fatalf("no command named %q", name)
	return "", ""
}

func explanationNodeIDByName(t *testing.T, root domain.ContentNode, name string) string {
	t.Helper()
	for id, node := range indexExplanationTree(root) {
		if node.Name == name {
			return id
		}
	}
	t.Fatalf("no node named %q", name)
	return ""
}

func explanationTerminal(t *testing.T, session *domain.Session, terminalID string) *domain.Terminal {
	t.Helper()
	for index := range session.Terminals {
		if session.Terminals[index].ID == terminalID {
			return &session.Terminals[index]
		}
	}
	t.Fatalf("no terminal %q", terminalID)
	return nil
}

func explanationNode(t *testing.T, root domain.ContentNode, nodeID string) domain.ContentNode {
	t.Helper()
	node, ok := indexExplanationTree(root)[nodeID]
	require.Truef(t, ok, "no node %q", nodeID)
	return node
}

func explanationDevice(t *testing.T, session *domain.Session, deviceID string) *domain.FacilityDevice {
	t.Helper()
	for index := range session.Facility.Devices {
		if session.Facility.Devices[index].ID == deviceID {
			return &session.Facility.Devices[index]
		}
	}
	t.Fatalf("no device %q", deviceID)
	return nil
}

func explanationCondition(t *testing.T, session *domain.Session, conditionID string) *domain.DiagnosticCondition {
	t.Helper()
	for index := range session.Facility.Conditions {
		if session.Facility.Conditions[index].ID == conditionID {
			return &session.Facility.Conditions[index]
		}
	}
	t.Fatalf("no fault %q", conditionID)
	return nil
}

func explanationForTerminal(t *testing.T, explanations []TerminalExplanation, terminalID string) TerminalExplanation {
	t.Helper()
	for _, explanation := range explanations {
		if explanation.TerminalID == terminalID {
			return explanation
		}
	}
	t.Fatalf("no explanation for terminal %q", terminalID)
	return TerminalExplanation{}
}

func explanationForNode(t *testing.T, terminal TerminalExplanation, nodeID string) NodeExplanation {
	t.Helper()
	for _, node := range terminal.Nodes {
		if node.NodeID == nodeID {
			return node
		}
	}
	t.Fatalf("no explanation for node %q", nodeID)
	return NodeExplanation{}
}

func explanationMarked(t *testing.T, rules []PresentationRule) PresentationRule {
	t.Helper()
	for _, rule := range rules {
		if rule.PlayersSee {
			return rule
		}
	}
	t.Fatalf("no rule is marked as what players see in %v", explanationRuleKinds(rules))
	return PresentationRule{}
}

func explanationRuleWithKind(t *testing.T, rules []PresentationRule, kind PresentationRuleKind) PresentationRule {
	t.Helper()
	for _, rule := range rules {
		if rule.Kind == kind {
			return rule
		}
	}
	t.Fatalf("no %v rule in %v", kind, explanationRuleKinds(rules))
	return PresentationRule{}
}

func explanationRuleKinds(rules []PresentationRule) []PresentationRuleKind {
	kinds := make([]PresentationRuleKind, len(rules))
	for index, rule := range rules {
		kinds[index] = rule.Kind
	}
	return kinds
}

func requireDeviceCondition(t *testing.T, condition PresentationCondition, deviceID, stateID string) {
	t.Helper()
	require.NotNil(t, condition.DeviceState)
	require.Equal(t, deviceID, condition.DeviceState.DeviceID)
	require.Equal(t, stateID, condition.DeviceState.StateID)
}

func scribbleExplanationRules(rules []PresentationRule) {
	for index := range rules {
		condition := rules[index].Condition
		if condition.DeviceState != nil {
			condition.DeviceState.StateID = "scribbled"
		}
		if condition.CommandExecuted != nil {
			condition.CommandExecuted.CommandID = "scribbled"
		}
		if condition.DiagnosticActive != nil {
			condition.DiagnosticActive.ConditionID = "scribbled"
		}
	}
}
