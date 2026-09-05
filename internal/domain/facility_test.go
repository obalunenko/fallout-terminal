package domain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneFacilityDeeplyDetachesDefinitionsAndProtectedValues(t *testing.T) {
	t.Parallel()

	original := &Facility{
		Revision: 7,
		Extra:    map[string]json.RawMessage{"futureFacility": json.RawMessage(`{"keep":true}`)},
		Devices: []FacilityDevice{
			{
				ID: "power-grid", Name: "Main grid", Kind: FacilityDeviceKind("power-grid"),
				InitialStateID: "offline", CurrentStateID: "online",
				Extra: map[string]json.RawMessage{"futureDevice": json.RawMessage(`17`)},
				States: []FacilityDeviceState{
					{ID: "offline", Name: "Offline"},
					{ID: "online", Name: "Online", Extra: map[string]json.RawMessage{"futureState": json.RawMessage(`[]`)}},
				},
				Transitions: []FacilityDeviceTransition{{
					ID: "restore", Name: "Restore power", SourceStateID: "offline", DestinationStateID: "online",
					Preconditions:    []FacilityStateEquality{{DeviceID: "cooling", StateID: "online"}},
					ConditionEffects: []FacilityConditionEffect{{ConditionID: "unpowered", Active: false}},
					Recovery:         true,
					Extra:            map[string]json.RawMessage{"futureTransition": json.RawMessage(`null`)},
				}},
			},
			{
				ID: "cooling", Name: "Cooling loop", Kind: FacilityDeviceKind("custom"), CustomKind: "cooling-loop",
				InitialStateID: "online", CurrentStateID: "online",
				States: []FacilityDeviceState{{ID: "online", Name: "Online"}},
			},
		},
		Conditions: []DiagnosticCondition{{
			ID: "unpowered", Name: "Grid unpowered", Category: DiagnosticConditionCategory("unpowered"),
			Device: &DiagnosticDeviceScope{DeviceID: "power-grid"}, InitialActive: true, CurrentActive: false,
			Effects: []DiagnosticEffect{{
				CapabilityBlock: &CapabilityBlockEffect{Capability: FacilityCapability("execute-command")},
			}},
			Recovery: []DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
			Extra:    map[string]json.RawMessage{"futureCondition": json.RawMessage(`"keep"`)},
		}},
		RecoveryPrograms: []RecoveryProgram{{
			ID: "restore-grid", Name: "Restore grid",
			Transitions: []FacilityTransitionRequest{{DeviceID: "power-grid", TransitionID: "restore"}},
			Extra:       map[string]json.RawMessage{"futureProgram": json.RawMessage(`{}`)},
		}},
	}

	clone := CloneFacility(original)
	require.NotNil(t, clone)

	clone.Revision = 8
	clone.Extra["futureFacility"][0] = '['
	clone.Devices[0].CurrentStateID = "offline"
	clone.Devices[0].Extra["futureDevice"][0] = '8'
	clone.Devices[0].States[1].Name = "Changed"
	clone.Devices[0].States[1].Extra["futureState"][0] = '{'
	clone.Devices[0].Transitions[0].Preconditions[0].StateID = "offline"
	clone.Devices[0].Transitions[0].ConditionEffects[0].Active = true
	clone.Devices[0].Transitions[0].Extra["futureTransition"][0] = '0'
	clone.Conditions[0].CurrentActive = true
	clone.Conditions[0].Device.DeviceID = "cooling"
	clone.Conditions[0].Effects[0].CapabilityBlock.Capability = FacilityCapability("hack")
	*clone.Conditions[0].Recovery[0].PrivateOverseerAction = false
	clone.Conditions[0].Extra["futureCondition"][0] = '['
	clone.RecoveryPrograms[0].Transitions[0].TransitionID = "shutdown"
	clone.RecoveryPrograms[0].Extra["futureProgram"][0] = '['

	assert.Equal(t, uint64(7), original.Revision)
	assert.Equal(t, json.RawMessage(`{"keep":true}`), original.Extra["futureFacility"])
	assert.Equal(t, "online", original.Devices[0].CurrentStateID)
	assert.Equal(t, json.RawMessage(`17`), original.Devices[0].Extra["futureDevice"])
	assert.Equal(t, "Online", original.Devices[0].States[1].Name)
	assert.Equal(t, json.RawMessage(`[]`), original.Devices[0].States[1].Extra["futureState"])
	assert.Equal(t, "online", original.Devices[0].Transitions[0].Preconditions[0].StateID)
	assert.False(t, original.Devices[0].Transitions[0].ConditionEffects[0].Active)
	assert.Equal(t, json.RawMessage(`null`), original.Devices[0].Transitions[0].Extra["futureTransition"])
	assert.False(t, original.Conditions[0].CurrentActive)
	assert.Equal(t, "power-grid", original.Conditions[0].Device.DeviceID)
	assert.Equal(t, FacilityCapability("execute-command"), original.Conditions[0].Effects[0].CapabilityBlock.Capability)
	assert.True(t, *original.Conditions[0].Recovery[0].PrivateOverseerAction)
	assert.Equal(t, json.RawMessage(`"keep"`), original.Conditions[0].Extra["futureCondition"])
	assert.Equal(t, "restore", original.RecoveryPrograms[0].Transitions[0].TransitionID)
	assert.Equal(t, json.RawMessage(`{}`), original.RecoveryPrograms[0].Extra["futureProgram"])
	assert.Nil(t, CloneFacility(nil))
}

func TestValidateSessionAcceptsFiniteFacilityStateGraph(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateSession(validFacilitySessionForTest()))
}

func TestValidateSessionRejectsInvalidFacilityStateGraphs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Facility)
	}{
		{name: "duplicate device ID", mutate: func(facility *Facility) {
			facility.Devices = append(facility.Devices, facility.Devices[0])
		}},
		{name: "empty state set", mutate: func(facility *Facility) {
			facility.Devices[0].States = nil
		}},
		{name: "duplicate state ID", mutate: func(facility *Facility) {
			facility.Devices[0].States[1].ID = facility.Devices[0].States[0].ID
		}},
		{name: "duplicate normalized state name", mutate: func(facility *Facility) {
			facility.Devices[0].States[1].Name = " offline "
		}},
		{name: "unknown initial state", mutate: func(facility *Facility) {
			facility.Devices[0].InitialStateID = "missing"
		}},
		{name: "unknown protected current state", mutate: func(facility *Facility) {
			facility.Devices[0].CurrentStateID = "missing"
		}},
		{name: "duplicate transition ID", mutate: func(facility *Facility) {
			facility.Devices[0].Transitions = append(facility.Devices[0].Transitions, facility.Devices[0].Transitions[0])
		}},
		{name: "unknown transition source", mutate: func(facility *Facility) {
			facility.Devices[0].Transitions[0].SourceStateID = "missing"
		}},
		{name: "unknown transition destination", mutate: func(facility *Facility) {
			facility.Devices[0].Transitions[0].DestinationStateID = "missing"
		}},
		{name: "transition to same state", mutate: func(facility *Facility) {
			facility.Devices[0].Transitions[0].DestinationStateID = facility.Devices[0].Transitions[0].SourceStateID
		}},
		{name: "unknown precondition device", mutate: func(facility *Facility) {
			facility.Devices[1].Transitions[0].Preconditions[0].DeviceID = "missing"
		}},
		{name: "precondition state owned by another device", mutate: func(facility *Facility) {
			facility.Devices[1].Transitions[0].Preconditions[0].StateID = "sealed"
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := validFacilitySessionForTest()
			test.mutate(candidate.Facility)
			require.Error(t, ValidateSession(candidate))
		})
	}
}

func TestValidateSessionAcceptsSingleDeviceEqualityBindings(t *testing.T) {
	t.Parallel()

	session := validFacilitySessionForTest()
	command := &session.Terminals[0].Root.Children[0]
	command.FacilityNameVariants = []FacilityTextVariant{
		{When: FacilityStateEquality{DeviceID: "door", StateID: "sealed"}, Text: "OPEN DOOR"},
		{When: FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "DOOR OPEN"},
	}
	command.VisibleWhen = &FacilityStateEquality{DeviceID: "power", StateID: "online"}
	command.AvailableWhen = &FacilityStateEquality{DeviceID: "door", StateID: "sealed"}
	session.Terminals[0].Root.Children = append(session.Terminals[0].Root.Children, ContentNode{
		ID: "status", Type: NodeEntry, Name: "STATUS",
		Blocks: []EntryContentBlock{{
			ID: "door-status", InitialText: "SEALED",
			FacilityTextVariants: []FacilityTextVariant{
				{When: FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "OPEN"},
			},
		}},
	})

	require.NoError(t, ValidateSession(session))
}

func TestValidateSessionRejectsInvalidFacilityEqualityBindings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "unknown binding device", mutate: func(session *Session) {
			session.Terminals[0].Root.Children[0].VisibleWhen = &FacilityStateEquality{DeviceID: "missing", StateID: "online"}
		}},
		{name: "state owned by another device", mutate: func(session *Session) {
			session.Terminals[0].Root.Children[0].VisibleWhen = &FacilityStateEquality{DeviceID: "door", StateID: "online"}
		}},
		{name: "name variants use multiple devices", mutate: func(session *Session) {
			session.Terminals[0].Root.Children[0].FacilityNameVariants = []FacilityTextVariant{
				{When: FacilityStateEquality{DeviceID: "door", StateID: "sealed"}, Text: "SEALED"},
				{When: FacilityStateEquality{DeviceID: "power", StateID: "online"}, Text: "ONLINE"},
			}
		}},
		{name: "name variants repeat one state", mutate: func(session *Session) {
			session.Terminals[0].Root.Children[0].FacilityNameVariants = []FacilityTextVariant{
				{When: FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "OPEN"},
				{When: FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "STILL OPEN"},
			}
		}},
		{name: "availability on entry", mutate: func(session *Session) {
			session.Terminals[0].Root.Children = append(session.Terminals[0].Root.Children, ContentNode{
				ID: "entry", Type: NodeEntry, Name: "ENTRY", Description: "Text",
				AvailableWhen: &FacilityStateEquality{DeviceID: "power", StateID: "online"},
			})
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := validFacilitySessionForTest()
			test.mutate(&candidate)
			require.Error(t, ValidateSession(candidate))
		})
	}
}

func TestValidateSessionAcceptsDiagnosticConditionCategoriesAndExactScopes(t *testing.T) {
	t.Parallel()

	categories := []struct {
		name           string
		category       DiagnosticConditionCategory
		customCategory string
	}{
		{name: "offline", category: DiagnosticConditionCategoryOffline},
		{name: "unpowered", category: DiagnosticConditionCategoryUnpowered},
		{name: "network isolated", category: DiagnosticConditionCategoryNetworkIsolated},
		{name: "storage damaged", category: DiagnosticConditionCategoryStorageDamaged},
		{name: "authorization corrupted", category: DiagnosticConditionCategoryAuthorizationCorrupted},
		{name: "display unstable", category: DiagnosticConditionCategoryDisplayUnstable},
		{name: "custom", category: DiagnosticConditionCategoryCustom, customCategory: "coolant-contaminated"},
	}
	scopes := []struct {
		name     string
		device   *DiagnosticDeviceScope
		terminal *DiagnosticTerminalScope
	}{
		{name: "device", device: &DiagnosticDeviceScope{DeviceID: "door"}},
		{name: "terminal", terminal: &DiagnosticTerminalScope{TerminalID: "terminal"}},
	}

	for _, category := range categories {
		for _, scope := range scopes {
			t.Run(category.name+"/"+scope.name, func(t *testing.T) {
				t.Parallel()
				session := diagnosticFacilitySessionForTest()
				condition := &session.Facility.Conditions[0]
				condition.Category = category.category
				condition.CustomCategory = category.customCategory
				condition.Device = scope.device
				condition.Terminal = scope.terminal

				require.NoError(t, ValidateSession(session))
			})
		}
	}
}

func TestValidateSessionAcceptsEveryDiagnosticEffect(t *testing.T) {
	t.Parallel()

	for _, capability := range []FacilityCapability{
		FacilityCapabilityExecuteCommand,
		FacilityCapabilityViewEntry,
		FacilityCapabilityHack,
		FacilityCapabilityTerminalTransition,
		FacilityCapabilityRunRecoveryProgram,
	} {
		t.Run("capability/"+string(capability), func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			session.Facility.Conditions[0].Effects = []DiagnosticEffect{{
				CapabilityBlock: &CapabilityBlockEffect{Capability: capability},
			}}

			require.NoError(t, ValidateSession(session))
		})
	}

	for _, test := range []struct {
		name   string
		effect DiagnosticEffect
	}{
		{name: "diagnostic path", effect: DiagnosticEffect{DiagnosticPath: &DiagnosticPathEffect{
			TerminalID: "terminal", NodeID: "diagnostics",
		}}},
		{name: "record substitution", effect: DiagnosticEffect{RecordSubstitution: &RecordSubstitutionEffect{
			TerminalID: "terminal", BlockID: "reactor-status", ReplacementText: "[CRC FAILURE]",
		}}},
		{name: "display instability", effect: DiagnosticEffect{DisplayInstability: &DisplayInstabilityEffect{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			session.Facility.Conditions[0].Effects = []DiagnosticEffect{test.effect}

			require.NoError(t, ValidateSession(session))
		})
	}
}

func TestValidateSessionAcceptsEveryDiagnosticRecoveryReference(t *testing.T) {
	t.Parallel()

	programID := "open-door"
	for _, test := range []struct {
		name      string
		reference DiagnosticRecoveryReference
	}{
		{name: "authored transition", reference: DiagnosticRecoveryReference{
			Transition: &FacilityTransitionRequest{DeviceID: "door", TransitionID: "open"},
		}},
		{name: "recovery program", reference: DiagnosticRecoveryReference{RecoveryProgramID: &programID}},
		{name: "private overseer action", reference: DiagnosticRecoveryReference{PrivateOverseerAction: new(true)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			session.Facility.Conditions[0].Recovery = []DiagnosticRecoveryReference{test.reference}

			require.NoError(t, ValidateSession(session))
		})
	}
}

func TestValidateSessionRequiresRecoveryMarkedViableConditionRecovery(t *testing.T) {
	t.Parallel()

	programID := "open-door"
	for _, test := range []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "direct transition is not recovery marked", mutate: func(session *Session) {
			session.Facility.Devices[1].Transitions[0].Recovery = false
			session.Facility.Conditions[0].Recovery = []DiagnosticRecoveryReference{{
				Transition: &FacilityTransitionRequest{DeviceID: "door", TransitionID: "open"},
			}}
		}},
		{name: "direct transition does not clear condition", mutate: func(session *Session) {
			session.Facility.Devices[1].Transitions[0].ConditionEffects = nil
			session.Facility.Conditions[0].Recovery = []DiagnosticRecoveryReference{{
				Transition: &FacilityTransitionRequest{DeviceID: "door", TransitionID: "open"},
			}}
		}},
		{name: "program contains non-recovery transition", mutate: func(session *Session) {
			session.Facility.Devices[1].Transitions[0].Recovery = false
			session.Facility.Conditions[0].Recovery = []DiagnosticRecoveryReference{{RecoveryProgramID: &programID}}
		}},
		{name: "program does not clear condition", mutate: func(session *Session) {
			session.Facility.Devices[1].Transitions[0].ConditionEffects = nil
			session.Facility.Conditions[0].Recovery = []DiagnosticRecoveryReference{{RecoveryProgramID: &programID}}
		}},
		{name: "non-recovery reference remains invalid beside private recovery", mutate: func(session *Session) {
			session.Facility.Devices[1].Transitions[0].Recovery = false
			session.Facility.Conditions[0].Recovery = []DiagnosticRecoveryReference{
				{Transition: &FacilityTransitionRequest{DeviceID: "door", TransitionID: "open"}},
				{PrivateOverseerAction: new(true)},
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			test.mutate(&session)

			require.Error(t, ValidateSession(session))
		})
	}

	session := diagnosticFacilitySessionForTest()
	session.Facility.Devices[1].Transitions[0].ConditionEffects = nil
	session.Facility.Conditions[0].Recovery = []DiagnosticRecoveryReference{
		{Transition: &FacilityTransitionRequest{DeviceID: "door", TransitionID: "open"}},
		{PrivateOverseerAction: new(true)},
	}
	require.NoError(t, ValidateSession(session), "one viable private recovery may accompany a recovery-marked transition")
}

func TestValidateSessionRejectsTerminalScopedDiagnosticTargetOnAnotherTerminal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		effect func() DiagnosticEffect
	}{
		{name: "diagnostic path", effect: func() DiagnosticEffect {
			return DiagnosticEffect{DiagnosticPath: &DiagnosticPathEffect{
				TerminalID: "terminal-b", NodeID: "diagnostics",
			}}
		}},
		{name: "record substitution", effect: func() DiagnosticEffect {
			return DiagnosticEffect{RecordSubstitution: &RecordSubstitutionEffect{
				TerminalID: "terminal-b", BlockID: "reactor-status", ReplacementText: "damaged",
			}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			other := session.Terminals[0]
			other.ID = "terminal-b"
			other.Name = "Terminal B"
			session.Terminals = append(session.Terminals, other)
			condition := &session.Facility.Conditions[0]
			condition.Device = nil
			condition.Terminal = &DiagnosticTerminalScope{TerminalID: "terminal"}
			condition.Effects = []DiagnosticEffect{test.effect()}

			require.Error(t, ValidateSession(session))
		})
	}
}

func TestValidateSessionRejectsInvalidDiagnosticConditionShapes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		mutate func(*DiagnosticCondition)
	}{
		{name: "unknown category", mutate: func(condition *DiagnosticCondition) {
			condition.Category = DiagnosticConditionCategory("radiation-randomizer")
		}},
		{name: "standard category with custom label", mutate: func(condition *DiagnosticCondition) {
			condition.CustomCategory = "unexpected"
		}},
		{name: "custom category without label", mutate: func(condition *DiagnosticCondition) {
			condition.Category = DiagnosticConditionCategoryCustom
			condition.CustomCategory = ""
		}},
		{name: "missing scope", mutate: func(condition *DiagnosticCondition) {
			condition.Device = nil
			condition.Terminal = nil
		}},
		{name: "two scopes", mutate: func(condition *DiagnosticCondition) {
			condition.Terminal = &DiagnosticTerminalScope{TerminalID: "terminal"}
		}},
		{name: "unknown device scope", mutate: func(condition *DiagnosticCondition) {
			condition.Device = &DiagnosticDeviceScope{DeviceID: "missing"}
		}},
		{name: "terminal ID in device scope", mutate: func(condition *DiagnosticCondition) {
			condition.Device = &DiagnosticDeviceScope{DeviceID: "terminal"}
		}},
		{name: "unknown terminal scope", mutate: func(condition *DiagnosticCondition) {
			condition.Device = nil
			condition.Terminal = &DiagnosticTerminalScope{TerminalID: "missing"}
		}},
		{name: "device ID in terminal scope", mutate: func(condition *DiagnosticCondition) {
			condition.Device = nil
			condition.Terminal = &DiagnosticTerminalScope{TerminalID: "door"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			test.mutate(&session.Facility.Conditions[0])

			require.Error(t, ValidateSession(session))
		})
	}
}

func TestValidateSessionRejectsInvalidDiagnosticEffectsAndRecoveryReferences(t *testing.T) {
	t.Parallel()

	programID := "open-door"
	for _, test := range []struct {
		name   string
		mutate func(*DiagnosticCondition)
	}{
		{name: "missing effect variant", mutate: func(condition *DiagnosticCondition) {
			condition.Effects = []DiagnosticEffect{{}}
		}},
		{name: "multiple effect variants", mutate: func(condition *DiagnosticCondition) {
			condition.Effects = []DiagnosticEffect{{
				CapabilityBlock:    &CapabilityBlockEffect{Capability: FacilityCapabilityExecuteCommand},
				DisplayInstability: &DisplayInstabilityEffect{},
			}}
		}},
		{name: "unknown capability", mutate: func(condition *DiagnosticCondition) {
			condition.Effects = []DiagnosticEffect{{
				CapabilityBlock: &CapabilityBlockEffect{Capability: FacilityCapability("back")},
			}}
		}},
		{name: "diagnostic path uses unknown terminal", mutate: func(condition *DiagnosticCondition) {
			condition.Effects = []DiagnosticEffect{{DiagnosticPath: &DiagnosticPathEffect{
				TerminalID: "missing", NodeID: "diagnostics",
			}}}
		}},
		{name: "diagnostic path uses node from another terminal", mutate: func(condition *DiagnosticCondition) {
			condition.Effects = []DiagnosticEffect{{DiagnosticPath: &DiagnosticPathEffect{
				TerminalID: "terminal", NodeID: "other-diagnostics",
			}}}
		}},
		{name: "record substitution uses unknown block", mutate: func(condition *DiagnosticCondition) {
			condition.Effects = []DiagnosticEffect{{RecordSubstitution: &RecordSubstitutionEffect{
				TerminalID: "terminal", BlockID: "missing", ReplacementText: "damaged",
			}}}
		}},
		{name: "missing recovery variant", mutate: func(condition *DiagnosticCondition) {
			condition.Recovery = []DiagnosticRecoveryReference{{}}
		}},
		{name: "multiple recovery variants", mutate: func(condition *DiagnosticCondition) {
			condition.Recovery = []DiagnosticRecoveryReference{{
				RecoveryProgramID: &programID, PrivateOverseerAction: new(true),
			}}
		}},
		{name: "false private recovery", mutate: func(condition *DiagnosticCondition) {
			condition.Recovery = []DiagnosticRecoveryReference{{PrivateOverseerAction: new(false)}}
		}},
		{name: "unknown recovery transition", mutate: func(condition *DiagnosticCondition) {
			condition.Recovery = []DiagnosticRecoveryReference{{
				Transition: &FacilityTransitionRequest{DeviceID: "door", TransitionID: "missing"},
			}}
		}},
		{name: "unknown recovery program", mutate: func(condition *DiagnosticCondition) {
			missing := "missing"
			condition.Recovery = []DiagnosticRecoveryReference{{RecoveryProgramID: &missing}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			test.mutate(&session.Facility.Conditions[0])

			require.Error(t, ValidateSession(session))
		})
	}
}

func TestValidateSessionRejectsOverlappingDiagnosticPresentationTargets(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		effect DiagnosticEffect
	}{
		{name: "diagnostic path", effect: DiagnosticEffect{DiagnosticPath: &DiagnosticPathEffect{
			TerminalID: "terminal", NodeID: "diagnostics",
		}}},
		{name: "record substitution", effect: DiagnosticEffect{RecordSubstitution: &RecordSubstitutionEffect{
			TerminalID: "terminal", BlockID: "reactor-status", ReplacementText: "second replacement",
		}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			first := &session.Facility.Conditions[0]
			first.Effects = []DiagnosticEffect{test.effect}
			second := *first
			second.ID = "second-condition"
			second.Name = "Second condition"
			second.Effects = []DiagnosticEffect{test.effect}
			session.Facility.Conditions = append(session.Facility.Conditions, second)

			require.ErrorContains(t, ValidateSession(session), "overlaps")
		})
	}
}

func TestExpandRecoveryProgramReturnsFiniteDetachedWorldAction(t *testing.T) {
	t.Parallel()

	session := diagnosticFacilitySessionForTest()
	// Both requests are evaluated from one pre-state, so the door precondition
	// must match the power transition's authored source state.
	session.Facility.Devices[1].Transitions[0].Preconditions[0].StateID = "offline"
	program := &session.Facility.RecoveryPrograms[0]
	program.Transitions = []FacilityTransitionRequest{
		{DeviceID: "power", TransitionID: "restore", Extra: map[string]json.RawMessage{"future": json.RawMessage(`true`)}},
		{DeviceID: "door", TransitionID: "open"},
	}

	want := CloneFacilityTransitionRequests(program.Transitions)
	got, issues := ExpandRecoveryProgram(session.Facility, program.ID)
	require.Empty(t, issues)
	assert.Equal(t, want, got)

	repeated, repeatedIssues := ExpandRecoveryProgram(session.Facility, program.ID)
	require.Empty(t, repeatedIssues)
	assert.Equal(t, got, repeated, "program expansion must be deterministic")

	got[0].TransitionID = "mutated"
	got[0].Extra["future"][0] = 'f'
	fresh, freshIssues := ExpandRecoveryProgram(session.Facility, program.ID)
	require.Empty(t, freshIssues)
	assert.Equal(t, want, fresh, "expanded requests must not alias authored recovery data")
}

func TestExpandRecoveryProgramRejectsMissingConflictingAndUnboundedPrograms(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		programID   string
		mutate      func(*Facility)
		failureCode FacilityFailureCode
		entityKind  string
		entityID    string
	}{
		{
			name: "missing program", programID: "missing", mutate: func(*Facility) {},
			failureCode: FacilityFailureMissingReference,
			entityKind:  string(FacilityEntityKindRecoveryProgram), entityID: "missing",
		},
		{name: "unknown transition", programID: "open-door", mutate: func(facility *Facility) {
			facility.RecoveryPrograms[0].Transitions[0].TransitionID = "missing"
		}, failureCode: FacilityFailureMissingReference,
			entityKind: string(FacilityEntityKindDeviceTransition), entityID: "missing"},
		{name: "multiple transitions for one device", programID: "open-door", mutate: func(facility *Facility) {
			facility.RecoveryPrograms[0].Transitions = append(
				facility.RecoveryPrograms[0].Transitions,
				facility.RecoveryPrograms[0].Transitions[0],
			)
		}, failureCode: FacilityFailureConflict,
			entityKind: string(FacilityEntityKindRecoveryProgram), entityID: "open-door"},
		{name: "program exceeds finite expansion limit", programID: "open-door", mutate: func(facility *Facility) {
			facility.RecoveryPrograms[0].Transitions = make([]FacilityTransitionRequest, maxFacilityItemsPerList+1)
			for index := range facility.RecoveryPrograms[0].Transitions {
				facility.RecoveryPrograms[0].Transitions[index] = FacilityTransitionRequest{
					DeviceID: "door", TransitionID: "open",
				}
			}
		}, failureCode: FacilityFailureInvalidConfiguration,
			entityKind: string(FacilityEntityKindRecoveryProgram), entityID: "open-door"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := diagnosticFacilitySessionForTest()
			test.mutate(session.Facility)

			transitions, issues := ExpandRecoveryProgram(session.Facility, test.programID)
			assert.Nil(t, transitions)
			require.NotEmpty(t, issues)
			assert.Equal(t, test.failureCode, issues[0].Code)
			assert.Equal(t, test.entityKind, issues[0].EntityKind)
			require.NotNil(t, issues[0].EntityID)
			assert.Equal(t, test.entityID, *issues[0].EntityID)
		})
	}
}

func TestFacilityFailureCodesRemainStableAndDistinct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code FacilityFailureCode
		want string
	}{
		{code: FacilityFailureUnspecified, want: "unspecified"},
		{code: FacilityFailureRejected, want: "rejected"},
		{code: FacilityFailureMissingReference, want: "missing-reference"},
		{code: FacilityFailureInvalidTransition, want: "invalid-transition"},
		{code: FacilityFailurePreconditionFailed, want: "precondition-failed"},
		{code: FacilityFailureStaleRevision, want: "stale-revision"},
		{code: FacilityFailureConflict, want: "conflict"},
		{code: FacilityFailureDuplicate, want: "duplicate"},
		{code: FacilityFailureInvalidConfiguration, want: "invalid-configuration"},
		{code: FacilityFailurePersistenceFailed, want: "persistence-failed"},
		{code: FacilityFailureRuntimeContextEnded, want: "runtime-context-ended"},
	}

	seen := make(map[FacilityFailureCode]struct{}, len(tests))
	for _, test := range tests {
		assert.Equal(t, test.want, string(test.code))
		_, duplicate := seen[test.code]
		assert.False(t, duplicate, "failure code %q is duplicated", test.code)
		seen[test.code] = struct{}{}
	}
}

func TestFacilityStableIdentitiesAreScopedAndFinite(t *testing.T) {
	t.Parallel()

	valid := dependencyFacilitySessionForTest()
	valid.Facility.Devices[1].States[0].ID = valid.Facility.Devices[0].States[0].ID
	valid.Facility.Devices[1].InitialStateID = valid.Facility.Devices[0].States[0].ID
	valid.Facility.Devices[1].CurrentStateID = valid.Facility.Devices[0].States[0].ID
	valid.Facility.Devices[1].Transitions[0].SourceStateID = valid.Facility.Devices[0].States[0].ID
	valid.Terminals[0].Root.Children[0].AvailableWhen.StateID = valid.Facility.Devices[0].States[0].ID
	require.NoError(t, ValidateSession(valid), "state IDs are scoped by their owning device")

	scopedTransitions := dependencyFacilitySessionForTest()
	scopedTransitions.Facility.Devices[1].Transitions[0].ID = "restore"
	scopedTransitions.Terminals[0].Root.Children[0].StateChange.FacilityAction.Transitions.Transitions[0].TransitionID = "restore"
	scopedTransitions.Facility.Conditions[0].Recovery[0].Transition.TransitionID = "restore"
	scopedTransitions.Facility.RecoveryPrograms[0].Transitions[0].TransitionID = "restore"
	require.NoError(t, ValidateSession(scopedTransitions), "transition IDs are scoped by their owning device")

	for _, test := range []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "blank device identity", mutate: func(session *Session) {
			session.Facility.Devices[0].ID = " "
		}},
		{name: "invalid UTF-8 state identity", mutate: func(session *Session) {
			session.Facility.Devices[0].States[0].ID = string([]byte{0xff})
		}},
		{name: "untrimmed condition identity", mutate: func(session *Session) {
			session.Facility.Conditions[0].ID = " door-alarm"
		}},
		{name: "overlong program identity", mutate: func(session *Session) {
			session.Facility.RecoveryPrograms[0].ID = strings.Repeat("x", maxNameBytes+1)
		}},
		{name: "duplicate state identity in one device", mutate: func(session *Session) {
			session.Facility.Devices[0].States[1].ID = session.Facility.Devices[0].States[0].ID
		}},
		{name: "duplicate transition identity in one device", mutate: func(session *Session) {
			session.Facility.Devices[1].Transitions = append(
				session.Facility.Devices[1].Transitions,
				session.Facility.Devices[1].Transitions[0],
			)
		}},
		{name: "transition endpoint outside owner", mutate: func(session *Session) {
			session.Facility.Devices[1].Transitions[0].DestinationStateID = "online"
		}},
		{name: "implicit self transition", mutate: func(session *Session) {
			transition := &session.Facility.Devices[1].Transitions[0]
			transition.DestinationStateID = transition.SourceStateID
		}},
		{name: "cyclic multi-device preconditions", mutate: func(session *Session) {
			session.Facility.Devices[0].Transitions[0].Preconditions = []FacilityStateEquality{{
				DeviceID: "door", StateID: "sealed",
			}}
			session.Facility.Devices[1].Transitions[0].Preconditions[0].StateID = "offline"
			session.Terminals[0].Root.Children[1].StateChange.FacilityAction = &FacilityActionConfig{
				Transitions: &FacilityTransitionList{Transitions: []FacilityTransitionRequest{
					{DeviceID: "power", TransitionID: "restore"},
					{DeviceID: "door", TransitionID: "open"},
				}},
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := dependencyFacilitySessionForTest()
			test.mutate(&candidate)
			require.Error(t, ValidateSession(candidate))
		})
	}
}

func TestFacilityEqualityReferencesResolveWithinOwningDevice(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		mutate func(*Session)
	}{
		{name: "transition precondition", mutate: func(session *Session) {
			session.Facility.Devices[1].Transitions[0].Preconditions[0].StateID = "sealed"
		}},
		{name: "menu name variant", mutate: func(session *Session) {
			session.Terminals[0].Root.Children[0].FacilityNameVariants[0].When.StateID = "sealed"
		}},
		{name: "visibility", mutate: func(session *Session) {
			session.Terminals[0].Root.Children[2].VisibleWhen.StateID = "sealed"
		}},
		{name: "availability", mutate: func(session *Session) {
			session.Terminals[0].Root.Children[0].AvailableWhen.StateID = "online"
		}},
		{name: "entry content variant", mutate: func(session *Session) {
			session.Terminals[0].Root.Children[2].Blocks[0].FacilityTextVariants[0].When.StateID = "sealed"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := dependencyFacilitySessionForTest()
			test.mutate(&candidate)
			require.ErrorContains(t, ValidateSession(candidate), "unknown state")
		})
	}
}

func TestBuildFacilityDependencyReportIndexesEveryReferenceKindDeterministically(t *testing.T) {
	t.Parallel()

	session := dependencyFacilitySessionForTest()
	require.NoError(t, ValidateSession(session))
	targets := []FacilityEntityReference{
		{Kind: FacilityEntityKindDevice, EntityID: "power"},
		{Kind: FacilityEntityKindDevice, EntityID: "door"},
		{Kind: FacilityEntityKindDeviceState, EntityID: "online", OwnerID: new("power")},
		{Kind: FacilityEntityKindDeviceTransition, EntityID: "open", OwnerID: new("door")},
		{Kind: FacilityEntityKindCondition, EntityID: "door-alarm"},
		{Kind: FacilityEntityKindRecoveryProgram, EntityID: "open-door"},
	}

	kinds := make(map[FacilityDependencyKind]bool)
	for _, target := range targets {
		report, issues := BuildFacilityDependencyReport(session, target)
		require.Empty(t, issues, "target %#v", target)
		assert.Equal(t, target, report.Target)
		for _, dependency := range report.Dependencies {
			kinds[dependency.Kind] = true
			assert.NotEmpty(t, dependency.SourceID)
			assert.NotEmpty(t, dependency.TargetID)
			assert.NotEmpty(t, dependency.Property)
		}

		repeated, repeatedIssues := BuildFacilityDependencyReport(session, target)
		assert.Empty(t, repeatedIssues)
		assert.Equal(t, report, repeated, "dependency order must be deterministic")
		if len(report.Dependencies) != 0 {
			report.Dependencies[0].Property = "mutated"
			if report.Dependencies[0].TerminalID != nil {
				*report.Dependencies[0].TerminalID = "mutated"
			}
			fresh, freshIssues := BuildFacilityDependencyReport(session, target)
			assert.Empty(t, freshIssues)
			assert.NotEqual(t, report, fresh, "dependency output aliases retained index state")
		}
	}

	assert.Equal(t, map[FacilityDependencyKind]bool{
		FacilityDependencyKindTransitionPrecondition:    true,
		FacilityDependencyKindTransitionConditionEffect: true,
		FacilityDependencyKindRecoveryReference:         true,
		FacilityDependencyKindRecoveryProgramTransition: true,
		FacilityDependencyKindCommandAction:             true,
		FacilityDependencyKindNameVariant:               true,
		FacilityDependencyKindEntryContentVariant:       true,
		FacilityDependencyKindVisibility:                true,
		FacilityDependencyKindAvailability:              true,
		FacilityDependencyKindDiagnosticScope:           true,
		FacilityDependencyKindDiagnosticEffect:          true,
	}, kinds)
}

func TestBuildFacilityDependencyReportRequiresExactScopedTarget(t *testing.T) {
	t.Parallel()

	session := dependencyFacilitySessionForTest()
	for _, target := range []FacilityEntityReference{
		{Kind: FacilityEntityKindDeviceState, EntityID: "online"},
		{Kind: FacilityEntityKindDeviceState, EntityID: "online", OwnerID: new("missing")},
		{Kind: FacilityEntityKindDeviceTransition, EntityID: "open", OwnerID: new("power")},
		{Kind: FacilityEntityKindCondition, EntityID: "missing"},
		{Kind: FacilityEntityKindUnspecified, EntityID: "power"},
	} {
		report, issues := BuildFacilityDependencyReport(session, target)
		assert.Equal(t, target, report.Target)
		assert.Empty(t, report.Dependencies)
		require.NotEmpty(t, issues)
		assert.Equal(t, FacilityFailureMissingReference, issues[0].Code)
	}
}

func TestValidateFacilityAuthoringCandidateReportsIdentityImpact(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		entityKind FacilityEntityKind
		entityID   string
		mutate     func(*Session)
	}{
		{name: "device renamed", entityKind: FacilityEntityKindDevice, entityID: "power", mutate: func(candidate *Session) {
			candidate.Facility.Devices[0].ID = "grid"
		}},
		{name: "state renamed", entityKind: FacilityEntityKindDeviceState, entityID: "online", mutate: func(candidate *Session) {
			candidate.Facility.Devices[0].States[1].ID = "energized"
		}},
		{name: "transition renamed", entityKind: FacilityEntityKindDeviceTransition, entityID: "open", mutate: func(candidate *Session) {
			candidate.Facility.Devices[1].Transitions[0].ID = "unlock"
		}},
		{name: "condition deleted", entityKind: FacilityEntityKindCondition, entityID: "door-alarm", mutate: func(candidate *Session) {
			candidate.Facility.Conditions = []DiagnosticCondition{}
		}},
		{name: "recovery program deleted", entityKind: FacilityEntityKindRecoveryProgram, entityID: "open-door", mutate: func(candidate *Session) {
			candidate.Facility.RecoveryPrograms = []RecoveryProgram{}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			current := dependencyFacilitySessionForTest()
			candidate := CloneSession(current)
			test.mutate(&candidate)

			issues := ValidateFacilityAuthoringCandidate(current, candidate)
			require.NotEmpty(t, issues)
			assertFacilityIdentityIssue(t, issues, test.entityKind, test.entityID)
		})
	}
}

func TestValidateFacilityAuthoringCandidateAcceptsDisplayRenameAndCompleteRepair(t *testing.T) {
	t.Parallel()

	current := dependencyFacilitySessionForTest()
	displayRename := CloneSession(current)
	displayRename.Facility.Devices[0].Name = "Emergency power grid"
	displayRename.Facility.Devices[0].States[1].Name = "Energized"
	displayRename.Facility.Devices[1].Transitions[0].Name = "Release door"
	displayRename.Facility.Conditions[0].Name = "Door security alarm"
	displayRename.Facility.RecoveryPrograms[0].Name = "Release door program"
	assert.Empty(t, ValidateFacilityAuthoringCandidate(current, displayRename))

	completeRepair := CloneSession(current)
	reassignDeviceIdentityForTest(&completeRepair, "power", "grid")
	require.NoError(t, ValidateSession(completeRepair))
	assert.Empty(t, ValidateFacilityAuthoringCandidate(current, completeRepair))

	incompleteRepair := CloneSession(current)
	incompleteRepair.Facility.Devices[0].ID = "grid"
	issues := ValidateFacilityAuthoringCandidate(current, incompleteRepair)
	require.NotEmpty(t, issues)
	assertFacilityIdentityIssue(t, issues, FacilityEntityKindDevice, "power")
}

func assertFacilityIdentityIssue(t *testing.T, issues []FacilityIssue, kind FacilityEntityKind, entityID string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == FacilityFailureConflict && issue.EntityKind == string(kind) &&
			issue.EntityID != nil && *issue.EntityID == entityID {
			return
		}
	}
	assert.Fail(t, "missing facility identity impact", "kind=%q entity=%q issues=%#v", kind, entityID, issues)
}

func dependencyFacilitySessionForTest() Session {
	session := validFacilitySessionForTest()
	command := &session.Terminals[0].Root.Children[0]
	command.StateChange = &StateChangeConfig{
		CompletedName: "DOOR OPEN", ConfirmationText: "Open the door?",
		FacilityAction: &FacilityActionConfig{Transitions: &FacilityTransitionList{
			Transitions: []FacilityTransitionRequest{{DeviceID: "door", TransitionID: "open"}},
		}},
	}
	command.FacilityNameVariants = []FacilityTextVariant{{
		When: FacilityStateEquality{DeviceID: "power", StateID: "online"}, Text: "OPEN POWERED DOOR",
	}}
	command.AvailableWhen = &FacilityStateEquality{DeviceID: "door", StateID: "sealed"}
	programID := "open-door"
	session.Terminals[0].Root.Children = append(session.Terminals[0].Root.Children,
		ContentNode{
			ID: "run-recovery", Type: NodeCommand, Name: "RUN RECOVERY", Text: "Recovery started.",
			StateChange: &StateChangeConfig{
				CompletedName: "RECOVERY STARTED", ConfirmationText: "Run recovery?",
				FacilityAction: &FacilityActionConfig{RecoveryProgramID: &programID},
			},
		},
		ContentNode{
			ID: "status", Type: NodeEntry, Name: "STATUS",
			VisibleWhen: &FacilityStateEquality{DeviceID: "power", StateID: "online"},
			Blocks: []EntryContentBlock{{
				ID: "power-status", InitialText: "POWER: OFFLINE",
				FacilityTextVariants: []FacilityTextVariant{{
					When: FacilityStateEquality{DeviceID: "power", StateID: "online"}, Text: "POWER: ONLINE",
				}},
			}},
		},
	)
	session.Facility.Devices[1].Transitions[0].ConditionEffects = []FacilityConditionEffect{{
		ConditionID: "door-alarm", Active: false,
	}}
	session.Facility.Devices[1].Transitions[0].Recovery = true
	session.Facility.Conditions = []DiagnosticCondition{{
		ID: "door-alarm", Name: "Door alarm", Category: DiagnosticConditionCategoryOffline,
		Device: &DiagnosticDeviceScope{DeviceID: "door"},
		Effects: []DiagnosticEffect{
			{CapabilityBlock: &CapabilityBlockEffect{Capability: FacilityCapabilityExecuteCommand}},
			{DiagnosticPath: &DiagnosticPathEffect{TerminalID: "terminal", NodeID: "status"}},
			{RecordSubstitution: &RecordSubstitutionEffect{
				TerminalID: "terminal", BlockID: "power-status", ReplacementText: "DATA CORRUPTED",
			}},
		},
		Recovery: []DiagnosticRecoveryReference{
			{Transition: &FacilityTransitionRequest{DeviceID: "door", TransitionID: "open"}},
			{RecoveryProgramID: &programID},
			{PrivateOverseerAction: new(true)},
		},
	}}
	session.Facility.RecoveryPrograms = []RecoveryProgram{{
		ID: programID, Name: "Open door", Transitions: []FacilityTransitionRequest{{
			DeviceID: "door", TransitionID: "open",
		}},
	}}
	return session
}

func reassignDeviceIdentityForTest(session *Session, oldID, newID string) {
	for deviceIndex := range session.Facility.Devices {
		device := &session.Facility.Devices[deviceIndex]
		if device.ID == oldID {
			device.ID = newID
		}
		for transitionIndex := range device.Transitions {
			transition := &device.Transitions[transitionIndex]
			for preconditionIndex := range transition.Preconditions {
				if transition.Preconditions[preconditionIndex].DeviceID == oldID {
					transition.Preconditions[preconditionIndex].DeviceID = newID
				}
			}
		}
	}
	for conditionIndex := range session.Facility.Conditions {
		condition := &session.Facility.Conditions[conditionIndex]
		if condition.Device != nil && condition.Device.DeviceID == oldID {
			condition.Device.DeviceID = newID
		}
		for recoveryIndex := range condition.Recovery {
			if transition := condition.Recovery[recoveryIndex].Transition; transition != nil && transition.DeviceID == oldID {
				transition.DeviceID = newID
			}
		}
	}
	for programIndex := range session.Facility.RecoveryPrograms {
		for transitionIndex := range session.Facility.RecoveryPrograms[programIndex].Transitions {
			request := &session.Facility.RecoveryPrograms[programIndex].Transitions[transitionIndex]
			if request.DeviceID == oldID {
				request.DeviceID = newID
			}
		}
	}
	for terminalIndex := range session.Terminals {
		reassignDeviceInContentForTest(&session.Terminals[terminalIndex].Root, oldID, newID)
	}
}

func reassignDeviceInContentForTest(node *ContentNode, oldID, newID string) {
	if node.VisibleWhen != nil && node.VisibleWhen.DeviceID == oldID {
		node.VisibleWhen.DeviceID = newID
	}
	if node.AvailableWhen != nil && node.AvailableWhen.DeviceID == oldID {
		node.AvailableWhen.DeviceID = newID
	}
	for variantIndex := range node.FacilityNameVariants {
		if node.FacilityNameVariants[variantIndex].When.DeviceID == oldID {
			node.FacilityNameVariants[variantIndex].When.DeviceID = newID
		}
	}
	for blockIndex := range node.Blocks {
		for variantIndex := range node.Blocks[blockIndex].FacilityTextVariants {
			if node.Blocks[blockIndex].FacilityTextVariants[variantIndex].When.DeviceID == oldID {
				node.Blocks[blockIndex].FacilityTextVariants[variantIndex].When.DeviceID = newID
			}
		}
	}
	if node.StateChange != nil && node.StateChange.FacilityAction != nil && node.StateChange.FacilityAction.Transitions != nil {
		for requestIndex := range node.StateChange.FacilityAction.Transitions.Transitions {
			request := &node.StateChange.FacilityAction.Transitions.Transitions[requestIndex]
			if request.DeviceID == oldID {
				request.DeviceID = newID
			}
		}
	}
	for childIndex := range node.Children {
		reassignDeviceInContentForTest(&node.Children[childIndex], oldID, newID)
	}
}

func diagnosticFacilitySessionForTest() Session {
	session := validFacilitySessionForTest()
	session.Terminals[0].Root.Children = append(session.Terminals[0].Root.Children,
		ContentNode{ID: "diagnostics", Type: NodeFolder, Name: "DIAGNOSTICS", Children: []ContentNode{}},
		ContentNode{
			ID: "reactor-record", Type: NodeEntry, Name: "REACTOR STATUS",
			Blocks: []EntryContentBlock{{ID: "reactor-status", InitialText: "REACTOR NOMINAL"}},
		},
	)
	programID := "open-door"
	session.Facility.RecoveryPrograms = []RecoveryProgram{{
		ID: programID, Name: "Open door",
		Transitions: []FacilityTransitionRequest{{DeviceID: "door", TransitionID: "open"}},
	}}
	session.Facility.Conditions = []DiagnosticCondition{{
		ID: "door-offline", Name: "Door offline", Category: DiagnosticConditionCategoryOffline,
		Device: &DiagnosticDeviceScope{DeviceID: "door"},
		Effects: []DiagnosticEffect{{
			CapabilityBlock: &CapabilityBlockEffect{Capability: FacilityCapabilityExecuteCommand},
		}},
		Recovery: []DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
	}}
	session.Facility.Devices[1].Transitions[0].Recovery = true
	session.Facility.Devices[1].Transitions[0].ConditionEffects = []FacilityConditionEffect{{
		ConditionID: "door-offline", Active: false,
	}}
	return session
}

func validFacilitySessionForTest() Session {
	return Session{
		Version: 1,
		Name:    "Facility",
		Terminals: []Terminal{{
			ID: "terminal", Name: "Terminal",
			Root: ContentNode{
				ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{{
					ID: "open-door", Type: NodeCommand, Name: "OPEN DOOR", Text: "Door opened.",
				}},
			},
		}},
		Facility: &Facility{
			Revision: 3,
			Devices: []FacilityDevice{
				{
					ID: "power", Name: "Power grid", Kind: FacilityDeviceKind("power-grid"),
					InitialStateID: "offline", CurrentStateID: "online",
					States: []FacilityDeviceState{
						{ID: "offline", Name: "Offline"},
						{ID: "online", Name: "Online"},
					},
					Transitions: []FacilityDeviceTransition{{
						ID: "restore", Name: "Restore", SourceStateID: "offline", DestinationStateID: "online",
					}},
				},
				{
					ID: "door", Name: "Vault door", Kind: FacilityDeviceKind("door"),
					InitialStateID: "sealed", CurrentStateID: "sealed",
					States: []FacilityDeviceState{
						{ID: "sealed", Name: "Sealed"},
						{ID: "open", Name: "Open"},
					},
					Transitions: []FacilityDeviceTransition{{
						ID: "open", Name: "Open", SourceStateID: "sealed", DestinationStateID: "open",
						Preconditions: []FacilityStateEquality{{DeviceID: "power", StateID: "online"}},
					}},
				},
			},
			Conditions:       []DiagnosticCondition{},
			RecoveryPrograms: []RecoveryProgram{},
		},
	}
}
