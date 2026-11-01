package session

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	persistencev1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/persistence/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestCommandContentUsesOneRealBehaviorOneofWithStableFieldNumbers(t *testing.T) {
	descriptor := (&persistencev1.CommandContent{}).ProtoReflect().Descriptor()
	behavior := descriptor.Oneofs().ByName("behavior")
	require.NotNil(t, behavior)
	require.False(t, behavior.IsSynthetic())

	stateChange := descriptor.Fields().ByName("state_change")
	require.NotNil(t, stateChange)
	require.Equal(t, int32(2), int32(stateChange.Number()))
	require.Equal(t, behavior, stateChange.ContainingOneof())

	terminalTransition := descriptor.Fields().ByName("terminal_transition")
	require.NotNil(t, terminalTransition)
	require.Equal(t, int32(3), int32(terminalTransition.Number()))
	require.Equal(t, behavior, terminalTransition.ContainingOneof())
}

func TestSessionContractAddsTerminalGroupsWithStableFieldNumbers(t *testing.T) {
	t.Parallel()

	sessionDescriptor := (&persistencev1.Session{}).ProtoReflect().Descriptor()
	groups := sessionDescriptor.Fields().ByName("terminal_groups")
	require.NotNil(t, groups)
	require.Equal(t, int32(5), int32(groups.Number()))

	groupDescriptor := (&persistencev1.TerminalGroup{}).ProtoReflect().Descriptor()
	require.Equal(t, int32(1), int32(groupDescriptor.Fields().ByName("id").Number()))
	require.Equal(t, int32(2), int32(groupDescriptor.Fields().ByName("name").Number()))
	require.Equal(t, int32(3), int32(groupDescriptor.Fields().ByName("terminal_ids").Number()))
}

func TestSessionContractRoundTripsOrderedTerminalGroups(t *testing.T) {
	t.Parallel()

	value := linkedSessionForTest()
	value.TerminalGroups = []domain.TerminalGroup{{ID: "vault", Name: "Vault", TerminalIDs: []string{"b", "a"}}}
	semantic, err := SessionToProto(value)
	require.NoError(t, err)
	require.Len(t, semantic.GetTerminalGroups(), 1)
	require.Equal(t, []string{"b", "a"}, semantic.GetTerminalGroups()[0].GetTerminalIds())

	roundTrip, err := SessionFromProto(semantic, value)
	require.NoError(t, err)
	require.Equal(t, value, roundTrip)
	roundTrip.TerminalGroups[0].TerminalIDs[0] = "changed"
	require.Equal(t, "b", value.TerminalGroups[0].TerminalIDs[0])
}

func TestSessionContractMapsEveryKnownFieldAndPreservesRecursiveJSONExtras(t *testing.T) {
	value := domain.Session{
		Version: 1, Name: "Vault", PlayerConfig: "players/roster.json",
		Extra: map[string]json.RawMessage{"futureSession": json.RawMessage(`{"enabled":true}`)},
		Terminals: []domain.Terminal{{
			ID: "terminal-1", Name: "Overseer", HackLevel: 2, IntroText: "WELCOME",
			Extra: map[string]json.RawMessage{"futureTerminal": json.RawMessage(`7`)},
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT", Extra: map[string]json.RawMessage{"futureRoot": json.RawMessage(`"x"`)},
				Children: []domain.ContentNode{
					{ID: "command", Type: domain.NodeCommand, Name: "RUN", Text: "OK", Extra: map[string]json.RawMessage{"futureCommand": json.RawMessage(`[]`)}},
					{ID: "entry", Type: domain.NodeEntry, Name: "READ", Description: "BODY", Extra: map[string]json.RawMessage{"futureEntry": json.RawMessage(`null`)}},
				},
			},
		}},
	}

	semantic, err := SessionToProto(value)
	require.NoError(t, err)
	require.Equal(t, int32(1), semantic.GetVersion())
	require.Equal(t, "players/roster.json", semantic.GetPlayerConfig())
	require.Equal(t, int32(2), semantic.GetTerminals()[0].GetHackLevel())
	require.Equal(t, "OK", semantic.GetTerminals()[0].GetRoot().GetFolder().GetChildren()[0].GetCommand().GetText())
	require.Equal(t, "BODY", semantic.GetTerminals()[0].GetRoot().GetFolder().GetChildren()[1].GetEntry().GetDescription())

	roundTrip, err := SessionFromProto(semantic, value)
	require.NoError(t, err)
	require.Equal(t, value, roundTrip)
	roundTripSemantic, err := SessionToProto(roundTrip)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(semantic, roundTripSemantic, protocmp.Transform()))
}

func TestSessionContractRejectsMissingOneofAndInvalidReference(t *testing.T) {
	value := domain.Session{Version: 1, Name: "Vault", PlayerConfig: testAbsolutePath("absolute", "players.json"), Terminals: []domain.Terminal{}}
	_, err := SessionToProto(value)
	require.Error(t, err)
}

func TestSessionContractKeepsLegacyOptionalFieldsAbsentAndOrdinaryContentUnchanged(t *testing.T) {
	value := domain.Session{
		Version: 1,
		Name:    "Legacy ordinary",
		Extra:   map[string]json.RawMessage{"futureSession": json.RawMessage(`{"keep":true}`)},
		Terminals: []domain.Terminal{{
			ID: "terminal-1", Name: "Overseer",
			Extra: map[string]json.RawMessage{"futureTerminal": json.RawMessage(`17`)},
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{{
					ID: "status", Type: domain.NodeCommand, Name: "Read status", Text: "All systems nominal.",
					Extra: map[string]json.RawMessage{"futureCommand": json.RawMessage(`"keep"`)},
				}},
			},
		}},
	}

	semantic, err := SessionToProto(value)
	require.NoError(t, err)
	require.Equal(t, int32(1), semantic.GetVersion())
	require.Nil(t, semantic.PlayerConfig)
	require.Nil(t, semantic.GetTerminals()[0].CommandStates)
	ordinary := semantic.GetTerminals()[0].GetRoot().GetFolder().GetChildren()[0].GetCommand()
	require.NotNil(t, ordinary)
	require.Equal(t, "All systems nominal.", ordinary.GetText())
	require.Nil(t, ordinary.GetBehavior())

	roundTrip, err := SessionFromProto(semantic, value)
	require.NoError(t, err)
	require.Equal(t, value, roundTrip)
	require.Nil(t, roundTrip.Terminals[0].Root.Children[0].StateChange)
	require.Empty(t, roundTrip.Terminals[0].CommandStates)
}

func TestSessionContractKeepsFacilityPresenceAdditive(t *testing.T) {
	t.Parallel()

	sessionDescriptor := (&persistencev1.Session{}).ProtoReflect().Descriptor()
	facilityField := sessionDescriptor.Fields().ByName("facility")
	require.NotNil(t, facilityField)
	require.Equal(t, int32(6), int32(facilityField.Number()))
	require.True(t, facilityField.HasOptionalKeyword())

	legacy := linkedSessionForTest()
	semantic, err := SessionToProto(legacy)
	require.NoError(t, err)
	require.False(t, semantic.ProtoReflect().Has(facilityField))
	require.Nil(t, semantic.GetFacility())

	roundTrip, err := SessionFromProto(semantic, legacy)
	require.NoError(t, err)
	require.Nil(t, roundTrip.Facility)
}

func TestSessionContractRoundTripsFullFacilityPersistenceGraph(t *testing.T) {
	t.Parallel()

	semantic := fullFacilitySessionProtoForTest()
	facilityField := semantic.ProtoReflect().Descriptor().Fields().ByName("facility")
	require.True(t, semantic.ProtoReflect().Has(facilityField))
	require.NotNil(t, semantic.GetFacility())

	roundTrip, err := SessionFromProto(semantic, domain.Session{})
	require.NoError(t, err)
	require.NotNil(t, roundTrip.Facility)

	remapped, err := SessionToProto(roundTrip)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(semantic, remapped, protocmp.Transform()))
}

func TestSessionContractPreservesFacilityVariantPresence(t *testing.T) {
	t.Parallel()

	semantic := fullFacilitySessionProtoForTest()
	facility := semantic.GetFacility()
	require.NotNil(t, facility)
	require.Len(t, facility.GetDevices(), 2)
	require.Len(t, facility.GetConditions(), 2)

	customDevice := facility.GetDevices()[1]
	customKindField := customDevice.ProtoReflect().Descriptor().Fields().ByName("custom_kind")
	require.True(t, customKindField.HasOptionalKeyword())
	require.True(t, customDevice.ProtoReflect().Has(customKindField))
	require.Equal(t, "coolant-manifold", customDevice.GetCustomKind())

	reactor := facility.GetDevices()[0]
	reactorCustomKind := reactor.ProtoReflect().Descriptor().Fields().ByName("custom_kind")
	require.False(t, reactor.ProtoReflect().Has(reactorCustomKind))

	deviceCondition := facility.GetConditions()[0]
	customCategoryField := deviceCondition.ProtoReflect().Descriptor().Fields().ByName("custom_category")
	require.True(t, customCategoryField.HasOptionalKeyword())
	require.True(t, deviceCondition.ProtoReflect().Has(customCategoryField))
	require.IsType(t, &persistencev1.DiagnosticCondition_Device{}, deviceCondition.GetScope())
	require.IsType(t, &persistencev1.DiagnosticEffect_CapabilityBlock{}, deviceCondition.GetEffects()[0].GetEffect())
	require.IsType(t, &persistencev1.DiagnosticEffect_DiagnosticPath{}, deviceCondition.GetEffects()[1].GetEffect())
	require.IsType(t, &persistencev1.DiagnosticEffect_RecordSubstitution{}, deviceCondition.GetEffects()[2].GetEffect())
	require.IsType(t, &persistencev1.DiagnosticEffect_DisplayInstability{}, deviceCondition.GetEffects()[3].GetEffect())
	require.IsType(t, &persistencev1.DiagnosticRecoveryReference_Transition{}, deviceCondition.GetRecovery()[0].GetRecovery())
	require.IsType(t, &persistencev1.DiagnosticRecoveryReference_RecoveryProgramId{}, deviceCondition.GetRecovery()[1].GetRecovery())
	require.IsType(t, &persistencev1.DiagnosticRecoveryReference_PrivateOverseerAction{}, deviceCondition.GetRecovery()[2].GetRecovery())

	terminalCondition := facility.GetConditions()[1]
	require.False(t, terminalCondition.ProtoReflect().Has(customCategoryField))
	require.IsType(t, &persistencev1.DiagnosticCondition_Terminal{}, terminalCondition.GetScope())

	children := semantic.GetTerminals()[0].GetRoot().GetFolder().GetChildren()
	visibleWhenField := children[0].ProtoReflect().Descriptor().Fields().ByName("visible_when")
	require.True(t, children[0].ProtoReflect().Has(visibleWhenField))
	availableWhenField := children[1].ProtoReflect().Descriptor().Fields().ByName("available_when")
	require.True(t, children[1].ProtoReflect().Has(availableWhenField))
	directAction := children[1].GetCommand().GetStateChange().GetFacilityAction()
	facilityActionField := children[1].GetCommand().GetStateChange().ProtoReflect().Descriptor().Fields().ByName("facility_action")
	require.True(t, children[1].GetCommand().GetStateChange().ProtoReflect().Has(facilityActionField))
	require.IsType(t, &persistencev1.FacilityActionConfig_Transitions{}, directAction.GetAction())
	programAction := children[2].GetCommand().GetStateChange().GetFacilityAction()
	require.IsType(t, &persistencev1.FacilityActionConfig_RecoveryProgramId{}, programAction.GetAction())

	roundTrip, err := SessionFromProto(semantic, domain.Session{})
	require.NoError(t, err)
	remapped, err := SessionToProto(roundTrip)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(semantic, remapped, protocmp.Transform()))
}

func TestSessionContractRoundTripsStateChangeConfigAndFrozenCommandStates(t *testing.T) {
	value := stateChangingSession("Vault")
	value.Terminals[0].CommandStates = map[string]domain.CommandExecutionState{
		"doors": {
			CompletedName: "Двери открыты",
			ResultText:    "Доступ в сектор разрешён.",
		},
	}

	semantic, err := SessionToProto(value)
	require.NoError(t, err)
	require.Equal(t, int32(1), semantic.GetVersion())
	require.Equal(t, "Двери открыты", semantic.GetTerminals()[0].GetCommandStates()["doors"].GetCompletedName())
	require.Equal(t, "Доступ в сектор разрешён.", semantic.GetTerminals()[0].GetCommandStates()["doors"].GetResultText())
	command := semantic.GetTerminals()[0].GetRoot().GetFolder().GetChildren()[0].GetCommand()
	require.IsType(t, &persistencev1.CommandContent_StateChange{}, command.GetBehavior())
	require.Equal(t, "Двери открыты", command.GetStateChange().GetCompletedName())
	require.Equal(t, "Открыть двери?", command.GetStateChange().GetConfirmationText())

	roundTrip, err := SessionFromProto(semantic, value)
	require.NoError(t, err)
	require.Equal(t, value, roundTrip)
	roundTripSemantic, err := SessionToProto(roundTrip)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(semantic, roundTripSemantic, protocmp.Transform()))
}

func TestSessionContractRoundTripsEntryBlocksAndExplicitEmptyChanges(t *testing.T) {
	t.Parallel()

	value := domain.Session{
		Version: 1,
		Name:    "Reactor",
		Extra:   map[string]json.RawMessage{"futureSession": json.RawMessage(`{"keep":true}`)},
		Terminals: []domain.Terminal{{
			ID:    "terminal-reactor",
			Name:  "Reactor control",
			Extra: map[string]json.RawMessage{"futureTerminal": json.RawMessage(`17`)},
			Root: domain.ContentNode{
				ID:   "root",
				Type: domain.NodeFolder,
				Name: "ROOT",
				Children: []domain.ContentNode{
					{
						ID:   "reactor-status",
						Type: domain.NodeEntry,
						Name: "REACTOR STATUS",
						Blocks: []domain.EntryContentBlock{
							{ID: "reactor-power", InitialText: "POWER: OFFLINE"},
							{ID: "reactor-cooling", InitialText: "COOLING: OFFLINE"},
						},
						Extra: map[string]json.RawMessage{"futureEntry": json.RawMessage(`{"layout":"dense"}`)},
					},
					{
						ID:   "restore-power",
						Type: domain.NodeCommand,
						Name: "RESTORE POWER",
						Text: "Primary power restored.",
						StateChange: &domain.StateChangeConfig{
							CompletedName:    "POWER RESTORED",
							ConfirmationText: "Authorize reactor power restoration?",
							EntryContentChange: &domain.EntryContentChange{
								BlockID: "reactor-power",
							},
						},
						Extra: map[string]json.RawMessage{"futureCommand": json.RawMessage(`"keep"`)},
					},
				},
			},
			CommandStates: map[string]domain.CommandExecutionState{
				"restore-power": {
					CompletedName: "POWER RESTORED",
					ResultText:    "Primary power restored.",
					EntryContentChange: &domain.EntryContentChange{
						BlockID: "reactor-power",
					},
				},
			},
		}},
	}

	semantic, err := SessionToProto(value)
	require.NoError(t, err)
	children := semantic.GetTerminals()[0].GetRoot().GetFolder().GetChildren()
	entry := children[0].GetEntry()
	require.Empty(t, cmp.Diff(&persistencev1.EntryContent{
		Blocks: []*persistencev1.EntryContentBlock{
			{Id: "reactor-power", InitialText: "POWER: OFFLINE"},
			{Id: "reactor-cooling", InitialText: "COOLING: OFFLINE"},
		},
	}, entry, protocmp.Transform()))

	configured := children[1].GetCommand().GetStateChange()
	require.NotNil(t, configured)
	configuredField := configured.ProtoReflect().Descriptor().Fields().ByName("entry_content_change")
	require.True(t, configured.ProtoReflect().Has(configuredField))
	require.Equal(t, "reactor-power", configured.GetEntryContentChange().GetBlockId())
	require.Empty(t, configured.GetEntryContentChange().GetCompletedText())

	frozen := semantic.GetTerminals()[0].GetCommandStates()["restore-power"]
	require.NotNil(t, frozen)
	frozenField := frozen.ProtoReflect().Descriptor().Fields().ByName("entry_content_change")
	require.True(t, frozen.ProtoReflect().Has(frozenField))
	require.Equal(t, "reactor-power", frozen.GetEntryContentChange().GetBlockId())
	require.Empty(t, frozen.GetEntryContentChange().GetCompletedText())

	roundTrip, err := SessionFromProto(semantic, value)
	require.NoError(t, err)
	require.Equal(t, value, roundTrip)
	require.Equal(t, value.Extra, roundTrip.Extra)
	require.Equal(t, value.Terminals[0].Extra, roundTrip.Terminals[0].Extra)
	require.Equal(t, value.Terminals[0].Root.Children[0].Extra, roundTrip.Terminals[0].Root.Children[0].Extra)
	require.Equal(t, value.Terminals[0].Root.Children[1].Extra, roundTrip.Terminals[0].Root.Children[1].Extra)

	roundTripSemantic, err := SessionToProto(roundTrip)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(semantic, roundTripSemantic, protocmp.Transform()))
}

func TestSessionContractKeepsLegacyEntryDescriptionWithoutImplicitBlocks(t *testing.T) {
	t.Parallel()

	value := domain.Session{
		Version: 1,
		Name:    "Legacy entry",
		Extra:   map[string]json.RawMessage{"futureSession": json.RawMessage(`{"keep":true}`)},
		Terminals: []domain.Terminal{{
			ID: "terminal-legacy", Name: "Legacy terminal",
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{{
					ID: "legacy-log", Type: domain.NodeEntry, Name: "LOG",
					Description: "One uninterrupted legacy description.",
					Extra:       map[string]json.RawMessage{"futureEntry": json.RawMessage(`{"style":"amber"}`)},
				}},
			},
		}},
	}

	semantic, err := SessionToProto(value)
	require.NoError(t, err)
	entry := semantic.GetTerminals()[0].GetRoot().GetFolder().GetChildren()[0].GetEntry()
	require.NotNil(t, entry)
	assert.Equal(t, value.Terminals[0].Root.Children[0].Description, entry.GetDescription())
	assert.Empty(t, entry.GetBlocks())

	roundTrip, err := SessionFromProto(semantic, value)
	require.NoError(t, err)
	require.Equal(t, value, roundTrip)
	assert.Nil(t, roundTrip.Terminals[0].Root.Children[0].Blocks)
	assert.JSONEq(t, `{"style":"amber"}`, string(roundTrip.Terminals[0].Root.Children[0].Extra["futureEntry"]))
}

func TestSessionContractRoundTripsTerminalTransitionAndKeepsLegacyAbsent(t *testing.T) {
	t.Parallel()

	value := linkedSessionForTest()
	semantic, err := SessionToProto(value)
	require.NoError(t, err)
	command := semantic.GetTerminals()[0].GetRoot().GetFolder().GetChildren()[0].GetCommand()
	require.IsType(t, &persistencev1.CommandContent_TerminalTransition{}, command.GetBehavior())
	require.NotNil(t, command.GetTerminalTransition())
	require.Equal(t, "b", command.GetTerminalTransition().GetTargetTerminalId())

	roundTrip, err := SessionFromProto(semantic, value)
	require.NoError(t, err)
	require.Equal(t, value, roundTrip)

	legacy := value
	legacy.Terminals = append([]domain.Terminal(nil), value.Terminals...)
	legacy.Terminals[0].Root = value.Terminals[0].Root
	legacy.Terminals[0].Root.Children = append([]domain.ContentNode(nil), value.Terminals[0].Root.Children...)
	legacy.Terminals[0].Root.Children[0].TerminalTransition = nil
	legacyProto, err := SessionToProto(legacy)
	require.NoError(t, err)
	require.Nil(t, legacyProto.GetTerminals()[0].GetRoot().GetFolder().GetChildren()[0].GetCommand().GetBehavior())
}

func TestSessionContractRejectsDualCommandBehaviorAtBoundary(t *testing.T) {
	t.Parallel()

	value := linkedSessionForTest()
	command := &value.Terminals[0].Root.Children[0]
	command.Text = "Done"
	command.StateChange = &domain.StateChangeConfig{CompletedName: "Done", ConfirmationText: "Proceed?"}

	_, err := SessionToProto(value)
	require.ErrorContains(t, err, "cannot contain both stateChange and terminalTransition")
}

func linkedSessionForTest() domain.Session {
	return domain.Session{Version: 1, Name: "Links", Terminals: []domain.Terminal{
		{
			ID: "a", Name: "Alpha",
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{{
					ID: "go", Type: domain.NodeCommand, Name: "GO",
					TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "b"},
				}},
			},
		},
		{
			ID: "b", Name: "Beta",
			Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}},
		},
	}}
}

func fullFacilitySessionProtoForTest() *persistencev1.Session {
	stateEquality := func(deviceID, stateID string) *persistencev1.FacilityStateEquality {
		return &persistencev1.FacilityStateEquality{DeviceId: deviceID, StateId: stateID}
	}
	transitionRequest := func(deviceID, transitionID string) *persistencev1.FacilityTransitionRequest {
		return &persistencev1.FacilityTransitionRequest{DeviceId: deviceID, TransitionId: transitionID}
	}
	textVariant := func(deviceID, stateID, text string) *persistencev1.FacilityTextVariant {
		return &persistencev1.FacilityTextVariant{When: stateEquality(deviceID, stateID), Text: text}
	}

	reactor := &persistencev1.FacilityDevice{
		Id: "reactor-main", Name: "Main Reactor",
		Kind:           persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_REACTOR,
		InitialStateId: "offline", CurrentStateId: "online",
		States: []*persistencev1.FacilityDeviceState{
			{Id: "offline", Name: "Offline"},
			{Id: "online", Name: "Online"},
		},
		Transitions: []*persistencev1.FacilityDeviceTransition{{
			Id: "scram", Name: "Scram reactor", SourceStateId: "online", DestinationStateId: "offline",
			Preconditions:    []*persistencev1.FacilityStateEquality{stateEquality("cooling-loop", "online")},
			ConditionEffects: []*persistencev1.FacilityConditionEffect{{ConditionId: "reactor-fault", Active: true}},
			Recovery:         true,
		}},
	}
	cooling := &persistencev1.FacilityDevice{
		Id: "cooling-loop", Name: "Primary Cooling",
		Kind:           persistencev1.FacilityDeviceKind_FACILITY_DEVICE_KIND_CUSTOM,
		CustomKind:     new("coolant-manifold"),
		InitialStateId: "offline",
		CurrentStateId: "online",
		States: []*persistencev1.FacilityDeviceState{
			{Id: "offline", Name: "Offline"},
			{Id: "online", Name: "Online"},
		},
		Transitions: []*persistencev1.FacilityDeviceTransition{{
			Id: "start", Name: "Start cooling", SourceStateId: "offline", DestinationStateId: "online",
		}},
	}

	deviceCondition := &persistencev1.DiagnosticCondition{
		Id: "reactor-fault", Name: "Reactor control fault",
		Category:       persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_CUSTOM,
		CustomCategory: new("flux-instability"),
		Scope: &persistencev1.DiagnosticCondition_Device{
			Device: &persistencev1.DiagnosticDeviceScope{DeviceId: "reactor-main"},
		},
		InitialActive: false,
		CurrentActive: true,
		Effects: []*persistencev1.DiagnosticEffect{
			{Effect: &persistencev1.DiagnosticEffect_CapabilityBlock{CapabilityBlock: &persistencev1.CapabilityBlockEffect{
				Capability: persistencev1.FacilityCapability_FACILITY_CAPABILITY_EXECUTE_COMMAND,
			}}},
			{Effect: &persistencev1.DiagnosticEffect_DiagnosticPath{DiagnosticPath: &persistencev1.DiagnosticPathEffect{
				TerminalId: "terminal-reactor", NodeId: "reactor-record",
			}}},
			{Effect: &persistencev1.DiagnosticEffect_RecordSubstitution{RecordSubstitution: &persistencev1.RecordSubstitutionEffect{
				TerminalId: "terminal-reactor", BlockId: "reactor-status", ReplacementText: "R3ACT0R: ####",
			}}},
			{Effect: &persistencev1.DiagnosticEffect_DisplayInstability{DisplayInstability: &persistencev1.DisplayInstabilityEffect{}}},
		},
		Recovery: []*persistencev1.DiagnosticRecoveryReference{
			{Recovery: &persistencev1.DiagnosticRecoveryReference_Transition{Transition: transitionRequest("reactor-main", "scram")}},
			{Recovery: &persistencev1.DiagnosticRecoveryReference_RecoveryProgramId{RecoveryProgramId: "safe-shutdown"}},
			{Recovery: &persistencev1.DiagnosticRecoveryReference_PrivateOverseerAction{PrivateOverseerAction: true}},
		},
	}
	terminalCondition := &persistencev1.DiagnosticCondition{
		Id: "terminal-isolated", Name: "Terminal isolated",
		Category: persistencev1.DiagnosticConditionCategory_DIAGNOSTIC_CONDITION_CATEGORY_NETWORK_ISOLATED,
		Scope: &persistencev1.DiagnosticCondition_Terminal{
			Terminal: &persistencev1.DiagnosticTerminalScope{TerminalId: "terminal-reactor"},
		},
		Effects: []*persistencev1.DiagnosticEffect{{
			Effect: &persistencev1.DiagnosticEffect_CapabilityBlock{CapabilityBlock: &persistencev1.CapabilityBlockEffect{
				Capability: persistencev1.FacilityCapability_FACILITY_CAPABILITY_TERMINAL_TRANSITION,
			}},
		}},
		Recovery: []*persistencev1.DiagnosticRecoveryReference{{
			Recovery: &persistencev1.DiagnosticRecoveryReference_PrivateOverseerAction{PrivateOverseerAction: true},
		}},
	}

	directAction := &persistencev1.FacilityActionConfig{Action: &persistencev1.FacilityActionConfig_Transitions{
		Transitions: &persistencev1.FacilityTransitionList{Transitions: []*persistencev1.FacilityTransitionRequest{
			transitionRequest("reactor-main", "scram"),
		}},
	}}
	programAction := &persistencev1.FacilityActionConfig{Action: &persistencev1.FacilityActionConfig_RecoveryProgramId{
		RecoveryProgramId: "safe-shutdown",
	}}

	return &persistencev1.Session{
		Version: 1,
		Name:    "Facility contract",
		Facility: &persistencev1.Facility{
			Revision: 12,
			Devices:  []*persistencev1.FacilityDevice{reactor, cooling},
			Conditions: []*persistencev1.DiagnosticCondition{
				deviceCondition,
				terminalCondition,
			},
			RecoveryPrograms: []*persistencev1.RecoveryProgram{{
				Id: "safe-shutdown", Name: "Safe shutdown",
				Transitions: []*persistencev1.FacilityTransitionRequest{transitionRequest("reactor-main", "scram")},
			}},
		},
		Terminals: []*persistencev1.Terminal{{
			Id: "terminal-reactor", Name: "Reactor Control",
			Root: &persistencev1.ContentNode{
				Id: "root", Name: "ROOT",
				Content: &persistencev1.ContentNode_Folder{Folder: &persistencev1.FolderContent{Children: []*persistencev1.ContentNode{
					{
						Id: "reactor-record", Name: "REACTOR STATUS",
						Content: &persistencev1.ContentNode_Entry{Entry: &persistencev1.EntryContent{Blocks: []*persistencev1.EntryContentBlock{{
							Id: "reactor-status", InitialText: "REACTOR: OFFLINE",
							FacilityTextVariants: []*persistencev1.FacilityTextVariant{
								textVariant("reactor-main", "online", "REACTOR: ONLINE"),
							},
						}}}},
						FacilityNameVariants: []*persistencev1.FacilityTextVariant{
							textVariant("reactor-main", "online", "REACTOR STATUS: ONLINE"),
						},
						VisibleWhen: stateEquality("reactor-main", "online"),
					},
					{
						Id: "scram-reactor", Name: "SCRAM REACTOR",
						Content: &persistencev1.ContentNode_Command{Command: &persistencev1.CommandContent{
							Text: "Reactor scrammed.",
							Behavior: &persistencev1.CommandContent_StateChange{StateChange: &persistencev1.StateChangeConfig{
								CompletedName: "REACTOR SCRAMMED", ConfirmationText: "Scram reactor?", FacilityAction: directAction,
							}},
						}},
						AvailableWhen: stateEquality("reactor-main", "online"),
					},
					{
						Id: "run-safe-shutdown", Name: "RUN SAFE SHUTDOWN",
						Content: &persistencev1.ContentNode_Command{Command: &persistencev1.CommandContent{
							Text: "Safe shutdown complete.",
							Behavior: &persistencev1.CommandContent_StateChange{StateChange: &persistencev1.StateChangeConfig{
								CompletedName: "SAFE SHUTDOWN COMPLETE", ConfirmationText: "Run safe shutdown?", FacilityAction: programAction,
							}},
						}},
					},
				}}},
			},
		}},
		TerminalGroups: []*persistencev1.TerminalGroup{{
			Id: "reactor-group", Name: "Reactor", TerminalIds: []string{"terminal-reactor"},
		}},
	}
}
