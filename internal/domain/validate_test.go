package domain

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSessionRejectsInvalidKnownShape(t *testing.T) {
	t.Parallel()

	validRoot := ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}}
	valid := Session{
		Version: 1,
		Name:    "Campaign",
		Terminals: []Terminal{{
			ID: "t1", Name: "Terminal", HackLevel: 0, Root: validRoot,
		}},
	}

	tests := []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "unsupported version", mutate: func(s *Session) { s.Version = 2 }},
		{name: "blank session name", mutate: func(s *Session) { s.Name = "  " }},
		{name: "session name bytes", mutate: func(s *Session) { s.Name = strings.Repeat("é", 129) }},
		{name: "blank terminal id", mutate: func(s *Session) { s.Terminals[0].ID = "" }},
		{name: "hack level", mutate: func(s *Session) { s.Terminals[0].HackLevel = 6 }},
		{name: "invalid root id", mutate: func(s *Session) { s.Terminals[0].Root.ID = "not-root" }},
		{name: "invalid root type", mutate: func(s *Session) { s.Terminals[0].Root.Type = NodeEntry }},
		{name: "unknown node type", mutate: func(s *Session) {
			s.Terminals[0].Root.Children = []ContentNode{{ID: "mystery", Type: "mystery", Name: "?"}}
		}},
		{name: "duplicate terminal id", mutate: func(s *Session) { s.Terminals = append(s.Terminals, s.Terminals[0]) }},
		{name: "duplicate node id", mutate: func(s *Session) {
			s.Terminals[0].Root.Children = []ContentNode{
				{ID: "n1", Type: NodeEntry, Name: "A", Description: "a"},
				{ID: "n1", Type: NodeCommand, Name: "B", Text: "b"},
			}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := cloneSession(valid)
			test.mutate(&candidate)
			require.Error(t, ValidateSession(candidate))
		})
	}
}

func TestValidateSessionRejectsDocumentLimits(t *testing.T) {
	t.Parallel()

	terminals := make([]Terminal, MaxTerminals+1)
	for i := range terminals {
		terminals[i] = Terminal{
			ID:   fmt.Sprintf("t%d", i),
			Name: "Terminal",
			Root: ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}},
		}
	}
	assert.Error(t, ValidateSession(Session{Version: 1, Name: "Too many", Terminals: terminals}))

	root := ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}}
	cursor := &root
	for depth := 1; depth <= MaxNodeDepth; depth++ {
		cursor.Children = []ContentNode{{ID: fmt.Sprintf("f%d", depth), Type: NodeFolder, Name: "Folder", Children: []ContentNode{}}}
		cursor = &cursor.Children[0]
	}
	tooDeep := Session{Version: 1, Name: "Deep", Terminals: []Terminal{{ID: "t1", Name: "T", Root: root}}}
	assert.Error(t, ValidateSession(tooDeep))
}

func TestValidatePlayerConfigAcceptsIntelligenceBoundaries(t *testing.T) {
	t.Parallel()

	for _, intelligence := range []int{1, 10} {
		t.Run(fmt.Sprintf("intelligence %d", intelligence), func(t *testing.T) {
			t.Parallel()
			config := PlayerConfig{
				Version: 1,
				Name:    "Players",
				Roster: []CharacterRosterEntry{{
					ID: "mara", Name: "Mara", Intelligence: intelligence, HackerPerkAvailable: true,
				}},
			}
			require.NoError(t, ValidatePlayerConfig(config))
		})
	}
}

func TestValidatePlayerConfigRejectsOutOfRangeIntelligence(t *testing.T) {
	t.Parallel()

	for _, intelligence := range []int{0, 11} {
		t.Run(fmt.Sprintf("intelligence %d", intelligence), func(t *testing.T) {
			t.Parallel()
			config := PlayerConfig{
				Version: 1,
				Name:    "Players",
				Roster: []CharacterRosterEntry{{
					ID: "mara", Name: "Mara", Intelligence: intelligence, HackerPerkAvailable: false,
				}},
			}
			require.ErrorContains(t, ValidatePlayerConfig(config), "intelligence")
		})
	}
}

func TestValidateSessionAcceptsStateChangingCommandStateByStableID(t *testing.T) {
	t.Parallel()

	session := validStateChangingSessionForTest()
	command := session.Terminals[0].Root.Children[0].Children[0]
	command.Name = "Renamed authored command"
	command.StateChange.CompletedName = "Renamed authored completed title"
	session.Terminals[0].Root.Children = []ContentNode{
		{ID: "other", Type: NodeFolder, Name: "OTHER", Children: []ContentNode{}},
		{ID: "moved", Type: NodeFolder, Name: "MOVED", Children: []ContentNode{command}},
	}

	require.NoError(t, ValidateSession(session))
	assert.Contains(t, session.Terminals[0].CommandStates, command.ID)
}

func TestValidateSessionRejectsInvalidStateChangingCommandShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "state change on folder", mutate: func(s *Session) {
			s.Terminals[0].Root.StateChange = &StateChangeConfig{CompletedName: "Done", ConfirmationText: "Proceed?"}
		}},
		{name: "state change on entry", mutate: func(s *Session) {
			node := &s.Terminals[0].Root.Children[0].Children[0]
			node.Type, node.Text, node.Description = NodeEntry, "", "Description"
		}},
		{name: "blank authored result", mutate: func(s *Session) {
			s.Terminals[0].Root.Children[0].Children[0].Text = " \t"
		}},
		{name: "blank completed name", mutate: func(s *Session) {
			s.Terminals[0].Root.Children[0].Children[0].StateChange.CompletedName = " \t"
		}},
		{name: "blank confirmation text", mutate: func(s *Session) {
			s.Terminals[0].Root.Children[0].Children[0].StateChange.ConfirmationText = "\n"
		}},
		{name: "completed name exceeds name limit", mutate: func(s *Session) {
			s.Terminals[0].Root.Children[0].Children[0].StateChange.CompletedName = strings.Repeat("x", maxNameBytes+1)
		}},
		{name: "confirmation exceeds command body limit", mutate: func(s *Session) {
			s.Terminals[0].Root.Children[0].Children[0].StateChange.ConfirmationText = strings.Repeat("x", maxBodyBytes+1)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := cloneSession(validStateChangingSessionForTest())
			test.mutate(&candidate)
			require.Error(t, ValidateSession(candidate))
		})
	}
}

func TestValidateSessionRejectsInvalidCommandExecutionStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "orphaned command ID", mutate: func(s *Session) {
			s.Terminals[0].CommandStates["missing-command"] = s.Terminals[0].CommandStates["doors"]
			delete(s.Terminals[0].CommandStates, "doors")
		}},
		{name: "new command ID does not inherit old state", mutate: func(s *Session) {
			s.Terminals[0].Root.Children[0].Children[0].ID = "replacement-doors"
		}},
		{name: "state points to folder", mutate: func(s *Session) {
			s.Terminals[0].CommandStates["section"] = s.Terminals[0].CommandStates["doors"]
			delete(s.Terminals[0].CommandStates, "doors")
		}},
		{name: "state points to ordinary command", mutate: func(s *Session) {
			node := &s.Terminals[0].Root.Children[0].Children[0]
			node.StateChange = nil
		}},
		{name: "state belongs to another terminal", mutate: func(s *Session) {
			command := s.Terminals[0].Root.Children[0].Children[0]
			s.Terminals[0].Root.Children[0].Children = nil
			s.Terminals = append(s.Terminals, Terminal{
				ID: "t2", Name: "Second terminal",
				Root: ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{command}},
			})
		}},
		{name: "blank snapshot completed name", mutate: func(s *Session) {
			state := s.Terminals[0].CommandStates["doors"]
			state.CompletedName = " "
			s.Terminals[0].CommandStates["doors"] = state
		}},
		{name: "blank snapshot result", mutate: func(s *Session) {
			state := s.Terminals[0].CommandStates["doors"]
			state.ResultText = "\n"
			s.Terminals[0].CommandStates["doors"] = state
		}},
		{name: "snapshot completed name exceeds name limit", mutate: func(s *Session) {
			state := s.Terminals[0].CommandStates["doors"]
			state.CompletedName = strings.Repeat("x", maxNameBytes+1)
			s.Terminals[0].CommandStates["doors"] = state
		}},
		{name: "snapshot result exceeds command body limit", mutate: func(s *Session) {
			state := s.Terminals[0].CommandStates["doors"]
			state.ResultText = strings.Repeat("x", maxBodyBytes+1)
			s.Terminals[0].CommandStates["doors"] = state
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := cloneSession(validStateChangingSessionForTest())
			test.mutate(&candidate)
			require.Error(t, ValidateSession(candidate))
		})
	}
}

func TestValidateSessionRejectsStateChangeKnownFieldsInExtras(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "node stateChange", mutate: func(s *Session) {
			node := &s.Terminals[0].Root.Children[0].Children[0]
			node.Extra = map[string]json.RawMessage{"stateChange": json.RawMessage(`{"future":true}`)}
		}},
		{name: "terminal commandStates", mutate: func(s *Session) {
			s.Terminals[0].Extra = map[string]json.RawMessage{"commandStates": json.RawMessage(`{}`)}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := cloneSession(validStateChangingSessionForTest())
			test.mutate(&candidate)
			require.Error(t, ValidateSession(candidate))
		})
	}
}

func TestValidateSessionResolvesTerminalTransitionsInTwoPasses(t *testing.T) {
	t.Parallel()

	linked := Session{Version: 1, Name: "Links", Terminals: []Terminal{
		{
			ID: "a", Name: "A",
			Root: ContentNode{
				ID: "root", Type: NodeFolder, Name: "ROOT",
				Children: []ContentNode{{
					ID: "go", Type: NodeCommand, Name: "GO",
					TerminalTransition: &TerminalTransitionConfig{TargetTerminalID: "b"},
				}},
			},
		},
		{
			ID: "b", Name: "B",
			Root: ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}},
		},
	}}
	require.NoError(t, ValidateSession(linked), "a forward reference must not depend on terminal ordering")

	for _, test := range []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "missing target", mutate: func(s *Session) { s.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID = "missing" }},
		{name: "self target", mutate: func(s *Session) { s.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID = "a" }},
		{name: "blank target", mutate: func(s *Session) { s.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID = " " }},
		{name: "state change conflict", mutate: func(s *Session) {
			s.Terminals[0].Root.Children[0].StateChange = &StateChangeConfig{CompletedName: "Done", ConfirmationText: "Proceed?"}
			s.Terminals[0].Root.Children[0].Text = "Done"
		}},
		{name: "config on folder", mutate: func(s *Session) {
			s.Terminals[0].Root.TerminalTransition = &TerminalTransitionConfig{TargetTerminalID: "b"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := CloneSession(linked)
			test.mutate(&candidate)
			require.Error(t, ValidateSession(candidate))
		})
	}
}

func TestValidateSessionAcceptsCanonicalTerminalGroupsInDeterministicOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		groups []TerminalGroup
	}{
		{
			name: "singleton groups preserve group order",
			groups: []TerminalGroup{
				{ID: "group-c", Name: "Charlie", TerminalIDs: []string{"c"}},
				{ID: "group-a", Name: "Alpha", TerminalIDs: []string{"a"}},
				{ID: "group-b", Name: "Bravo", TerminalIDs: []string{"b"}},
			},
		},
		{
			name: "multi-terminal group preserves member order",
			groups: []TerminalGroup{
				{ID: "group-route", Name: "Route", TerminalIDs: []string{"c", "a", "b"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session := validTerminalGroupValidationSession()
			session.TerminalGroups = test.groups
			want := CloneSession(session)

			require.NoError(t, ValidateSession(session))
			assert.Equal(t, want.TerminalGroups, session.TerminalGroups, "validation must not reorder groups or members")
		})
	}
}

func TestValidateSessionRejectsInvalidTerminalGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		groups []TerminalGroup
	}{
		{
			name: "empty group",
			groups: []TerminalGroup{
				{ID: "group-empty", Name: "Empty", TerminalIDs: []string{}},
				{ID: "group-rest", Name: "Rest", TerminalIDs: []string{"a", "b", "c"}},
			},
		},
		{
			name: "duplicate normalized name",
			groups: []TerminalGroup{
				{ID: "group-a", Name: "  Main Route  ", TerminalIDs: []string{"a"}},
				{ID: "group-b", Name: "mAiN rOuTe", TerminalIDs: []string{"b", "c"}},
			},
		},
		{
			name: "duplicate member in one group",
			groups: []TerminalGroup{
				{ID: "group-route", Name: "Route", TerminalIDs: []string{"a", "a", "b", "c"}},
			},
		},
		{
			name: "terminal assigned to multiple groups",
			groups: []TerminalGroup{
				{ID: "group-one", Name: "One", TerminalIDs: []string{"a", "b"}},
				{ID: "group-two", Name: "Two", TerminalIDs: []string{"b", "c"}},
			},
		},
		{
			name: "terminal missing from all groups",
			groups: []TerminalGroup{
				{ID: "group-route", Name: "Route", TerminalIDs: []string{"a", "b"}},
			},
		},
		{
			name: "group references missing terminal",
			groups: []TerminalGroup{
				{ID: "group-route", Name: "Route", TerminalIDs: []string{"a", "b", "c", "missing"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session := validTerminalGroupValidationSession()
			session.TerminalGroups = test.groups

			require.Error(t, ValidateSession(session))
		})
	}
}

func TestLegacyCrossTerminalLinkRemainsCompatibleAfterSingletonNormalization(t *testing.T) {
	t.Parallel()

	legacy := legacyTerminalGroupCompatibilitySession()
	require.Empty(t, legacy.TerminalGroups)
	require.NoError(t, ValidateSession(legacy), "legacy absence is a compatible document shape")

	normalized := NormalizeTerminalGroups(legacy)
	require.Len(t, normalized.TerminalGroups, 2)
	assert.Equal(t, []string{"a"}, normalized.TerminalGroups[0].TerminalIDs)
	assert.Equal(t, []string{"b"}, normalized.TerminalGroups[1].TerminalIDs)
	assert.NotEqual(t, normalized.TerminalGroups[0].ID, normalized.TerminalGroups[1].ID)
	require.NoError(t, ValidateSession(normalized), "legacy cross-singleton links remain authored and dormant")
	transition := normalized.Terminals[0].Root.Children[0].TerminalTransition
	require.NotNil(t, transition)
	assert.Equal(t, "b", transition.TargetTerminalID)

	_, err := ValidateTerminalGroupReplacement(normalized, normalized.TerminalGroups)
	require.ErrorContains(t, err, "crosses groups")
	require.ErrorContains(t, err, `command "go"`)
	coupled := []TerminalGroup{
		{ID: "coupled", Name: "Coupled", TerminalIDs: []string{"a", "b"}},
	}
	diff, err := ValidateTerminalGroupReplacement(normalized, coupled)
	require.NoError(t, err)
	assert.True(t, diff.Changed)
	assert.True(t, diff.MembershipOrOrderChanged)
}

func TestTerminalGroupReplacementIdentifiesEveryCrossGroupCommandDeterministically(t *testing.T) {
	t.Parallel()

	session := legacyTerminalGroupCompatibilitySession()
	session.Terminals[0].Root.Children = append(session.Terminals[0].Root.Children, ContentNode{
		ID: "go-backup", Type: NodeCommand, Name: "GO BACKUP",
		TerminalTransition: &TerminalTransitionConfig{TargetTerminalID: "b"},
	})
	session.TerminalGroups = []TerminalGroup{
		{ID: "coupled", Name: "Coupled", TerminalIDs: []string{"a", "b"}},
	}
	candidate := []TerminalGroup{
		{ID: "left", Name: "Left", TerminalIDs: []string{"a"}},
		{ID: "right", Name: "Right", TerminalIDs: []string{"b"}},
	}

	_, err := ValidateTerminalGroupReplacement(session, candidate)

	require.EqualError(t, err,
		`terminal group candidate invalidates authored transitions: `+
			`terminal transition command "go" in terminal "a" targets terminal "b" and crosses groups "left" and "right"; `+
			`terminal transition command "go-backup" in terminal "a" targets terminal "b" and crosses groups "left" and "right"`,
	)
}

func TestNormalizeTerminalGroupsDoesNotRepairMalformedExplicitGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		groups []TerminalGroup
	}{
		{
			name: "partial membership",
			groups: []TerminalGroup{
				{ID: "partial", Name: "Partial", TerminalIDs: []string{"a"}},
			},
		},
		{
			name: "empty explicit group",
			groups: []TerminalGroup{
				{ID: "empty", Name: "Empty", TerminalIDs: []string{}},
				{ID: "rest", Name: "Rest", TerminalIDs: []string{"a", "b"}},
			},
		},
		{
			name: "unknown explicit member",
			groups: []TerminalGroup{
				{ID: "explicit", Name: "Explicit", TerminalIDs: []string{"a", "b", "missing"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session := legacyTerminalGroupCompatibilitySession()
			session.TerminalGroups = test.groups
			normalized := NormalizeTerminalGroups(session)
			assert.Equal(t, test.groups, normalized.TerminalGroups,
				"non-empty explicit groups must not be silently repaired as legacy")
			require.Error(t, ValidateSession(normalized))
		})
	}
}

func TestCompatibleSingletonNormalizationPreservesUnknownExtras(t *testing.T) {
	t.Parallel()

	legacy := legacyTerminalGroupCompatibilitySession()
	legacy.Extra = map[string]json.RawMessage{
		"futureSession": json.RawMessage(`{"keep":true}`),
	}
	legacy.Terminals[0].Extra = map[string]json.RawMessage{
		"futureTerminal": json.RawMessage(`17`),
	}
	legacy.Terminals[0].Root.Children[0].Extra = map[string]json.RawMessage{
		"futureCommand": json.RawMessage(`{"mode":"compatibility"}`),
	}

	normalized := NormalizeTerminalGroups(legacy)
	require.NoError(t, ValidateSession(normalized))
	assert.Equal(t, legacy.Extra, normalized.Extra)
	assert.Equal(t, legacy.Terminals[0].Extra, normalized.Terminals[0].Extra)
	assert.Equal(t, legacy.Terminals[0].Root.Children[0].Extra, normalized.Terminals[0].Root.Children[0].Extra)

	encoded, err := EncodeSession(normalized)
	require.NoError(t, err)
	for _, field := range []string{"futureSession", "futureTerminal", "futureCommand", "terminalGroups"} {
		assert.Contains(t, string(encoded), `"`+field+`"`)
	}
	roundTrip, err := DecodeSession(encoded)
	require.NoError(t, err)
	assert.JSONEq(t, string(normalized.Extra["futureSession"]), string(roundTrip.Extra["futureSession"]))
	assert.JSONEq(t, string(normalized.Terminals[0].Extra["futureTerminal"]),
		string(roundTrip.Terminals[0].Extra["futureTerminal"]))
	assert.JSONEq(t, string(normalized.Terminals[0].Root.Children[0].Extra["futureCommand"]),
		string(roundTrip.Terminals[0].Root.Children[0].Extra["futureCommand"]))
}

func validTerminalGroupValidationSession() Session {
	root := ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}}
	return Session{
		Version: 1,
		Name:    "Grouped campaign",
		Terminals: []Terminal{
			{ID: "a", Name: "Alpha", Root: root},
			{ID: "b", Name: "Bravo", Root: root},
			{ID: "c", Name: "Charlie", Root: root},
		},
		TerminalGroups: []TerminalGroup{
			{ID: "group-route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
		},
	}
}

func legacyTerminalGroupCompatibilitySession() Session {
	return Session{
		Version: 1,
		Name:    "Legacy linked campaign",
		Terminals: []Terminal{
			{
				ID: "a", Name: "Alpha",
				Root: ContentNode{
					ID: "root", Type: NodeFolder, Name: "ROOT",
					Children: []ContentNode{{
						ID: "go", Type: NodeCommand, Name: "GO",
						TerminalTransition: &TerminalTransitionConfig{TargetTerminalID: "b"},
					}},
				},
			},
			{
				ID: "b", Name: "Beta",
				Root: ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}},
			},
		},
	}
}

func validStateChangingSessionForTest() Session {
	return Session{
		Version: 1,
		Name:    "Stateful campaign",
		Terminals: []Terminal{{
			ID: "t1", Name: "Terminal", HackLevel: 0,
			Root: ContentNode{
				ID: "root", Type: NodeFolder, Name: "ROOT",
				Children: []ContentNode{{
					ID: "section", Type: NodeFolder, Name: "SECTION",
					Children: []ContentNode{{
						ID: "doors", Type: NodeCommand, Name: "Open doors", Text: "Doors opened.",
						StateChange: &StateChangeConfig{
							CompletedName:    "Doors open",
							ConfirmationText: "Open the doors?",
						},
					}},
				}},
			},
			CommandStates: map[string]CommandExecutionState{
				"doors": {CompletedName: "Doors were opened", ResultText: "Access granted."},
			},
		}},
	}
}

func cloneSession(session Session) Session {
	clone := session
	clone.Terminals = append([]Terminal(nil), session.Terminals...)
	for i := range clone.Terminals {
		clone.Terminals[i].Root = cloneNode(session.Terminals[i].Root)
		if session.Terminals[i].CommandStates != nil {
			clone.Terminals[i].CommandStates = make(map[string]CommandExecutionState, len(session.Terminals[i].CommandStates))
			maps.Copy(clone.Terminals[i].CommandStates, session.Terminals[i].CommandStates)
		}
	}
	return clone
}

func cloneNode(node ContentNode) ContentNode {
	clone := node
	if node.StateChange != nil {
		stateChange := *node.StateChange
		clone.StateChange = &stateChange
	}
	clone.Children = make([]ContentNode, len(node.Children))
	for i := range node.Children {
		clone.Children[i] = cloneNode(node.Children[i])
	}
	return clone
}
