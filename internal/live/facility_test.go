package live

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFacilityProjectionAppliesPresentationPrecedencePerProperty(t *testing.T) {
	t.Parallel()

	authored := facilityProjectionTreeForTest()
	completed := facilityCompletedStatesForTest()

	tests := []struct {
		name             string
		completed        map[string]domain.CommandExecutionState
		deviceState      string
		diagnosticActive bool
		wantCommandName  string
		wantCommandText  string
		wantEntryText    string
	}{
		{
			name:            "authored base",
			deviceState:     "maintenance",
			wantCommandName: "OPEN DOOR",
			wantCommandText: "Base command result.",
			wantEntryText:   facilityEntryText("BASE STATUS"),
		},
		{
			name:            "completed command",
			completed:       completed,
			deviceState:     "maintenance",
			wantCommandName: "OPEN DOOR COMPLETE",
			wantCommandText: "Completed command result.",
			wantEntryText:   facilityEntryText("COMPLETED STATUS"),
		},
		{
			name:            "matching device binding",
			completed:       completed,
			deviceState:     "open",
			wantCommandName: "DOOR OPEN",
			wantCommandText: "Completed command result.",
			wantEntryText:   facilityEntryText("DEVICE STATUS: OPEN"),
		},
		{
			name:             "active diagnostic override",
			completed:        completed,
			deviceState:      "open",
			diagnosticActive: true,
			wantCommandName:  "DOOR OPEN",
			wantCommandText:  "Completed command result.",
			wantEntryText:    facilityEntryText("DIAGNOSTIC STATUS: CORRUPTED"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			facility := facilityProjectionStateForTest(test.deviceState, test.diagnosticActive)
			projection := projectFacility(authored, test.completed, facility, "terminal-a")

			command := findContentNode(projection.Tree, "open-door")
			require.NotNil(t, command)
			assert.Equal(t, test.wantCommandName, command.Name)
			assert.Equal(t, test.wantCommandText, command.Text)

			entry := findContentNode(projection.Tree, "door-status")
			require.NotNil(t, entry)
			assert.Equal(t, test.wantEntryText, entry.Description)
		})
	}
}

func TestFacilityProjectionRefreshesOpenEntryContentFromCurrentSharedState(t *testing.T) {
	t.Parallel()

	authored := facilityProjectionTreeForTest()
	completed := facilityCompletedStatesForTest()
	sealed := facilityProjectionStateForTest("sealed", false)
	open := domain.CloneFacility(sealed)
	open.Devices[0].CurrentStateID = "open"
	sealedBefore := domain.CloneFacility(sealed)
	openBefore := domain.CloneFacility(open)

	before := projectFacility(authored, completed, sealed, "terminal-a")
	after := projectFacility(authored, completed, open, "terminal-a")
	beforeEntry := findContentNode(before.Tree, "door-status")
	afterEntry := findContentNode(after.Tree, "door-status")
	require.NotNil(t, beforeEntry)
	require.NotNil(t, afterEntry)
	require.Equal(t, beforeEntry.ID, afterEntry.ID, "the open entry must remain the same stable authored node")
	assert.Equal(t, facilityEntryText("COMPLETED STATUS"), beforeEntry.Description)
	assert.Equal(t, facilityEntryText("DEVICE STATUS: OPEN"), afterEntry.Description)
	assert.Equal(t, sealedBefore, sealed, "projection mutated the sealed facility snapshot")
	assert.Equal(t, openBefore, open, "projection mutated the open facility snapshot")
}

func TestFacilityProjectionOmitsInvisibleNodesAndMarksUnavailableCommands(t *testing.T) {
	t.Parallel()

	authored := domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{
			{
				ID: "secured-record", Type: domain.NodeEntry, Name: "SECURED RECORD", Description: "VISIBLE",
				VisibleWhen: &domain.FacilityStateEquality{DeviceID: "door", StateID: "open"},
			},
			{
				ID: "start-reactor", Type: domain.NodeCommand, Name: "START REACTOR", Text: "STARTED",
				AvailableWhen: &domain.FacilityStateEquality{DeviceID: "power", StateID: "online"},
			},
			{ID: "status", Type: domain.NodeCommand, Name: "STATUS", Text: "NOMINAL"},
		},
	}
	facility := &domain.Facility{Devices: []domain.FacilityDevice{
		facilityDeviceForProjectionTest("door", "sealed", "sealed", "open"),
		facilityDeviceForProjectionTest("power", "offline", "offline", "online"),
	}}

	blocked := projectFacility(authored, nil, facility, "terminal-a")
	assert.Nil(t, findContentNode(blocked.Tree, "secured-record"), "false visibility equality must omit the node")
	unavailable := findContentNode(blocked.Tree, "start-reactor")
	require.NotNil(t, unavailable, "unavailable commands remain visible")
	require.NotNil(t, unavailable.Available)
	assert.False(t, *unavailable.Available)
	ordinary := findContentNode(blocked.Tree, "status")
	require.NotNil(t, ordinary)
	assert.Nil(t, ordinary.Available, "an unbound command keeps the backward-compatible absent marker")

	availableFacility := domain.CloneFacility(facility)
	availableFacility.Devices[0].CurrentStateID = "open"
	availableFacility.Devices[1].CurrentStateID = "online"
	available := projectFacility(authored, nil, availableFacility, "terminal-a")
	assert.NotNil(t, findContentNode(available.Tree, "secured-record"), "matching equality must retain the node")
	command := findContentNode(available.Tree, "start-reactor")
	require.NotNil(t, command)
	require.NotNil(t, command.Available)
	assert.True(t, *command.Available)
}

func TestFacilityProjectionIsDetachedAndRepeatableWithActiveBindings(t *testing.T) {
	t.Parallel()

	authored := facilityProjectionTreeForTest()
	completed := facilityCompletedStatesForTest()
	facility := facilityProjectionStateForTest("open", true)
	authoredBefore := domain.CloneContentNode(authored)
	completedBefore := domain.CloneCommandExecutionStates(completed)
	facilityBefore := domain.CloneFacility(facility)

	first := projectFacility(authored, completed, facility, "terminal-a")
	second := projectFacility(authored, completed, facility, "terminal-a")
	require.Empty(t, cmp.Diff(first, second), "identical inputs must produce identical projections")

	first.Tree.Children[0].Name = "MUTATED OUTPUT"
	first.Tree.Children[1].Blocks[1].InitialText = "MUTATED BLOCK"
	first.Tree.Children[1].Blocks[1].FacilityTextVariants[0].Text = "MUTATED VARIANT"
	assert.Empty(t, cmp.Diff(authoredBefore, authored), "projection retained an authored tree alias")
	assert.Empty(t, cmp.Diff(completedBefore, completed), "projection retained a completed-state alias")
	assert.Empty(t, cmp.Diff(facilityBefore, facility), "projection retained a facility alias")
	assert.Equal(t, "DOOR OPEN", second.Tree.Children[0].Name)
	assert.Equal(t, facilityEntryText("DIAGNOSTIC STATUS: CORRUPTED"), second.Tree.Children[1].Description)
}

func TestFacilityDiagnosticProjectionAppliesOnlySelectedCapabilitiesAndScopes(t *testing.T) {
	t.Parallel()

	authored := domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{
			{
				ID: "diagnostics", Type: domain.NodeFolder, Name: "DIAGNOSTICS",
				VisibleWhen: &domain.FacilityStateEquality{DeviceID: "power", StateID: "online"},
				Children:    []domain.ContentNode{{ID: "power-report", Type: domain.NodeEntry, Name: "POWER REPORT", Description: "REPORT"}},
			},
			{
				ID: "restore-power", Type: domain.NodeCommand, Name: "RESTORE POWER", Text: "RESTORED",
				AvailableWhen: &domain.FacilityStateEquality{DeviceID: "power", StateID: "online"},
			},
		},
	}
	facility := &domain.Facility{
		Devices: []domain.FacilityDevice{facilityDeviceForProjectionTest("power", "offline", "offline", "online")},
		Conditions: []domain.DiagnosticCondition{
			{
				ID: "terminal-offline", Name: "Terminal offline", CurrentActive: true,
				Terminal: &domain.DiagnosticTerminalScope{TerminalID: "terminal-a"},
				Effects: []domain.DiagnosticEffect{
					{CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityExecuteCommand}},
					{DiagnosticPath: &domain.DiagnosticPathEffect{TerminalID: "terminal-a", NodeID: "power-report"}},
				},
			},
			{
				ID: "power-isolated", Name: "Power isolated", CurrentActive: true,
				Device: &domain.DiagnosticDeviceScope{DeviceID: "power"},
				Effects: []domain.DiagnosticEffect{
					{CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityHack}},
				},
			},
			{
				ID: "other-terminal", Name: "Other terminal", CurrentActive: true,
				Terminal: &domain.DiagnosticTerminalScope{TerminalID: "terminal-b"},
				Effects: []domain.DiagnosticEffect{
					{CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityTerminalTransition}},
				},
			},
		},
	}

	projection := projectFacility(authored, nil, facility, "terminal-a")
	assert.True(t, projection.BlockedCapabilities[domain.FacilityCapabilityExecuteCommand])
	assert.True(t, projection.BlockedCapabilities[domain.FacilityCapabilityHack])
	assert.False(t, projection.BlockedCapabilities[domain.FacilityCapabilityTerminalTransition])
	assert.NotNil(t, findContentNode(projection.Tree, "diagnostics"), "diagnostic path must expose every hidden ancestor")
	assert.NotNil(t, findContentNode(projection.Tree, "power-report"))
}

func TestPreviewFacilityProjectsDetachedDeviceStateWithoutInstallingIt(t *testing.T) {
	t.Parallel()

	service := New(nil, nil)
	authored := facilityProjectionTreeForTest()
	runtime := &domain.TerminalRuntime{
		TerminalID: "terminal-a", TerminalName: "Terminal A",
		AuthoredTree: authored, Tree: domain.CloneContentNode(authored),
		CommandStates: facilityCompletedStatesForTest(), Nav: domain.NavState{Path: []string{"root"}, Mode: "list"},
	}
	facility := facilityProjectionStateForTest("sealed", false)
	runtimeBefore := cloneTerminalRuntimeForPreviewTest(runtime)
	facilityBefore := domain.CloneFacility(facility)
	installed := service.Set("installed", "Installed", domain.ContentNode{ID: "installed", Type: domain.NodeFolder, Name: "INSTALLED"}, 0, "")

	preview, issues := service.PreviewFacility(runtime, facility, domain.FacilityPreview{
		ExpectedFacilityRevision: facility.Revision,
		TerminalID:               runtime.TerminalID,
		DeviceState: &domain.FacilityDeviceStatePreview{
			DeviceID: "door", StateID: "open",
		},
	})

	require.Empty(t, issues)
	require.NotNil(t, preview)
	command := findContentNode(preview.Tree, "open-door")
	require.NotNil(t, command)
	require.Equal(t, "DOOR OPEN", command.Name)
	require.Empty(t, cmp.Diff(runtimeBefore, runtime), "preview mutated the terminal checkpoint")
	require.Empty(t, cmp.Diff(facilityBefore, facility), "preview mutated the canonical facility")
	require.Empty(t, cmp.Diff(installed, service.Snapshot()), "preview changed the installed live state")

	command.Name = "TAMPERED"
	repeated, repeatedIssues := service.PreviewFacility(runtime, facility, domain.FacilityPreview{
		ExpectedFacilityRevision: facility.Revision,
		TerminalID:               runtime.TerminalID,
		DeviceState: &domain.FacilityDeviceStatePreview{
			DeviceID: "door", StateID: "open",
		},
	})
	require.Empty(t, repeatedIssues)
	require.Equal(t, "DOOR OPEN", findContentNode(repeated.Tree, "open-door").Name)
}

func TestPreviewFacilityProjectsDetachedDiagnosticCondition(t *testing.T) {
	t.Parallel()

	service := New(nil, nil)
	authored := facilityProjectionTreeForTest()
	runtime := &domain.TerminalRuntime{
		TerminalID: "terminal-a", TerminalName: "Terminal A",
		AuthoredTree: authored, Tree: domain.CloneContentNode(authored),
		CommandStates: facilityCompletedStatesForTest(), Nav: domain.NavState{Path: []string{"root"}, Mode: "list"},
	}
	facility := facilityProjectionStateForTest("open", false)
	facility.Conditions[0].Effects = append(facility.Conditions[0].Effects,
		domain.DiagnosticEffect{DisplayInstability: &domain.DisplayInstabilityEffect{}},
	)
	runtimeBefore := cloneTerminalRuntimeForPreviewTest(runtime)
	facilityBefore := domain.CloneFacility(facility)

	preview, issues := service.PreviewFacility(runtime, facility, domain.FacilityPreview{
		ExpectedFacilityRevision: facility.Revision,
		TerminalID:               runtime.TerminalID,
		Condition: &domain.FacilityConditionPreview{
			ConditionID: "damaged-door-record", Active: true,
		},
	})

	require.Empty(t, issues)
	require.NotNil(t, preview)
	require.Equal(t, []domain.TerminalPresentationEffect{domain.TerminalPresentationEffectDisplayUnstable}, preview.Effects)
	entry := findContentNode(preview.Tree, "door-status")
	require.NotNil(t, entry)
	require.Equal(t, facilityEntryText("DIAGNOSTIC STATUS: CORRUPTED"), entry.Description)
	require.Empty(t, cmp.Diff(runtimeBefore, runtime))
	require.Empty(t, cmp.Diff(facilityBefore, facility))
}

func TestPreviewFacilityRejectsInvalidOverridesWithoutMutation(t *testing.T) {
	t.Parallel()

	authored := facilityProjectionTreeForTest()
	newRuntime := func() *domain.TerminalRuntime {
		return &domain.TerminalRuntime{
			TerminalID: "terminal-a", AuthoredTree: authored, Tree: domain.CloneContentNode(authored),
			CommandStates: facilityCompletedStatesForTest(), Nav: domain.NavState{Path: []string{"root"}, Mode: "list"},
		}
	}
	tests := []struct {
		name        string
		facility    *domain.Facility
		preview     domain.FacilityPreview
		wantFailure domain.FacilityFailureCode
	}{
		{
			name: "stale revision", facility: facilityProjectionStateForTest("sealed", false),
			preview: domain.FacilityPreview{
				ExpectedFacilityRevision: 99, TerminalID: "terminal-a",
				DeviceState: &domain.FacilityDeviceStatePreview{DeviceID: "door", StateID: "open"},
			},
			wantFailure: domain.FacilityFailureStaleRevision,
		},
		{
			name: "unknown terminal", facility: facilityProjectionStateForTest("sealed", false),
			preview: domain.FacilityPreview{
				ExpectedFacilityRevision: 0, TerminalID: "terminal-b",
				DeviceState: &domain.FacilityDeviceStatePreview{DeviceID: "door", StateID: "open"},
			},
			wantFailure: domain.FacilityFailureMissingReference,
		},
		{
			name: "missing override", facility: facilityProjectionStateForTest("sealed", false),
			preview:     domain.FacilityPreview{ExpectedFacilityRevision: 0, TerminalID: "terminal-a"},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name: "ambiguous override", facility: facilityProjectionStateForTest("sealed", false),
			preview: domain.FacilityPreview{
				ExpectedFacilityRevision: 0, TerminalID: "terminal-a",
				DeviceState: &domain.FacilityDeviceStatePreview{DeviceID: "door", StateID: "open"},
				Condition:   &domain.FacilityConditionPreview{ConditionID: "damaged-door-record", Active: true},
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name: "unknown device", facility: facilityProjectionStateForTest("sealed", false),
			preview: domain.FacilityPreview{
				ExpectedFacilityRevision: 0, TerminalID: "terminal-a",
				DeviceState: &domain.FacilityDeviceStatePreview{DeviceID: "missing", StateID: "open"},
			},
			wantFailure: domain.FacilityFailureMissingReference,
		},
		{
			name: "unknown state", facility: facilityProjectionStateForTest("sealed", false),
			preview: domain.FacilityPreview{
				ExpectedFacilityRevision: 0, TerminalID: "terminal-a",
				DeviceState: &domain.FacilityDeviceStatePreview{DeviceID: "door", StateID: "missing"},
			},
			wantFailure: domain.FacilityFailureMissingReference,
		},
		{
			name: "unknown condition", facility: facilityProjectionStateForTest("sealed", false),
			preview: domain.FacilityPreview{
				ExpectedFacilityRevision: 0, TerminalID: "terminal-a",
				Condition: &domain.FacilityConditionPreview{ConditionID: "missing", Active: true},
			},
			wantFailure: domain.FacilityFailureMissingReference,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := New(nil, nil)
			runtime := newRuntime()
			runtimeBefore := cloneTerminalRuntimeForPreviewTest(runtime)
			facilityBefore := domain.CloneFacility(test.facility)

			preview, issues := service.PreviewFacility(runtime, test.facility, test.preview)

			require.Nil(t, preview)
			require.Len(t, issues, 1)
			require.Equal(t, test.wantFailure, issues[0].Code)
			require.Empty(t, cmp.Diff(runtimeBefore, runtime))
			require.Empty(t, cmp.Diff(facilityBefore, test.facility))
		})
	}
}

func cloneTerminalRuntimeForPreviewTest(runtime *domain.TerminalRuntime) *domain.TerminalRuntime {
	if runtime == nil {
		return nil
	}
	clone := *runtime
	clone.AuthoredTree = domain.CloneContentNode(runtime.AuthoredTree)
	clone.Tree = domain.CloneContentNode(runtime.Tree)
	clone.CommandStates = domain.CloneCommandExecutionStates(runtime.CommandStates)
	clone.Effects = slices.Clone(runtime.Effects)
	clone.Nav.Path = slices.Clone(runtime.Nav.Path)
	return &clone
}

func TestTerminalReferencedDevicesCollectsBindingsAndActionsOnce(t *testing.T) {
	t.Parallel()

	programID := "start-ventilation"
	authored := domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{
			{
				ID: "operate", Type: domain.NodeCommand, Name: "OPERATE", Text: "DONE",
				AvailableWhen: &domain.FacilityStateEquality{DeviceID: "power", StateID: "online"},
				FacilityNameVariants: []domain.FacilityTextVariant{{
					When: domain.FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "OPERATE OPEN DOOR",
				}},
				StateChange: &domain.StateChangeConfig{
					CompletedName: "OPERATED", ConfirmationText: "Operate?",
					FacilityAction: &domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
						Transitions: []domain.FacilityTransitionRequest{{DeviceID: "alarm", TransitionID: "silence"}},
					}},
				},
			},
			{
				ID: "status", Type: domain.NodeEntry, Name: "STATUS",
				VisibleWhen: &domain.FacilityStateEquality{DeviceID: "storage", StateID: "healthy"},
				Blocks: []domain.EntryContentBlock{{
					ID: "status-block", InitialText: "STATUS",
					FacilityTextVariants: []domain.FacilityTextVariant{{
						When: domain.FacilityStateEquality{DeviceID: "reactor", StateID: "online"}, Text: "ONLINE",
					}},
				}},
			},
			{
				ID: "run-program", Type: domain.NodeCommand, Name: "RUN PROGRAM", Text: "STARTED",
				StateChange: &domain.StateChangeConfig{
					CompletedName: "PROGRAM COMPLETE", ConfirmationText: "Run?",
					FacilityAction: &domain.FacilityActionConfig{RecoveryProgramID: &programID},
				},
			},
		},
	}
	facility := &domain.Facility{
		Devices: []domain.FacilityDevice{
			facilityDeviceForProjectionTest("power", "online", "offline", "online"),
			facilityDeviceForProjectionTest("door", "open", "sealed", "open"),
			facilityDeviceForProjectionTest("storage", "healthy", "damaged", "healthy"),
			facilityDeviceForProjectionTest("reactor", "online", "offline", "online"),
			{
				ID: "alarm", Name: "alarm", Kind: domain.FacilityDeviceKindAlarm,
				InitialStateID: "ringing", CurrentStateID: "ringing",
				States: []domain.FacilityDeviceState{{ID: "ringing", Name: "ringing"}, {ID: "silent", Name: "silent"}},
				Transitions: []domain.FacilityDeviceTransition{{
					ID: "silence", Name: "silence", SourceStateID: "ringing", DestinationStateID: "silent",
				}},
			},
			{
				ID: "ventilation", Name: "ventilation", Kind: domain.FacilityDeviceKindVentilation,
				InitialStateID: "stopped", CurrentStateID: "stopped",
				States: []domain.FacilityDeviceState{{ID: "stopped", Name: "stopped"}, {ID: "running", Name: "running"}},
				Transitions: []domain.FacilityDeviceTransition{{
					ID: "start", Name: "start", SourceStateID: "stopped", DestinationStateID: "running",
				}},
			},
		},
		RecoveryPrograms: []domain.RecoveryProgram{{
			ID: programID, Name: "Start ventilation",
			Transitions: []domain.FacilityTransitionRequest{{DeviceID: "ventilation", TransitionID: "start"}},
		}},
	}
	facilityBefore := domain.CloneFacility(facility)

	want := map[string]bool{
		"alarm": true, "door": true, "power": true, "reactor": true, "storage": true, "ventilation": true,
	}
	assert.Equal(t, want, terminalReferencedDevices(authored, facility))
	assert.Equal(t, want, terminalReferencedDevices(authored, facility))
	assert.Empty(t, cmp.Diff(facilityBefore, facility))
}

func TestFacilityDiagnosticProjectionPreservesRecordPagesAndUsesDiagnosticPrecedence(t *testing.T) {
	t.Parallel()

	authored := domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{{
			ID: "damaged-record", Type: domain.NodeEntry, Name: "DAMAGE REPORT",
			Blocks: []domain.EntryContentBlock{
				{ID: "page-one", InitialText: "PAGE ONE", FacilityTextVariants: []domain.FacilityTextVariant{{
					When: domain.FacilityStateEquality{DeviceID: "storage", StateID: "damaged"}, Text: "DEVICE PAGE ONE",
				}}},
				{ID: "page-two", InitialText: "PAGE TWO"},
				{ID: "page-three", InitialText: "PAGE THREE"},
			},
		}},
	}
	completed := map[string]domain.CommandExecutionState{
		"repair-index": {EntryContentChange: &domain.EntryContentChange{BlockID: "page-two", CompletedText: "COMPLETED PAGE TWO"}},
	}
	facility := &domain.Facility{
		Devices: []domain.FacilityDevice{facilityDeviceForProjectionTest("storage", "damaged", "healthy", "damaged")},
		Conditions: []domain.DiagnosticCondition{{
			ID: "storage-damaged", Name: "Storage damaged", CurrentActive: true,
			Terminal: &domain.DiagnosticTerminalScope{TerminalID: "terminal-a"},
			Effects: []domain.DiagnosticEffect{
				{RecordSubstitution: &domain.RecordSubstitutionEffect{TerminalID: "terminal-a", BlockID: "page-one", ReplacementText: "CORRUPTED PAGE ONE"}},
				{RecordSubstitution: &domain.RecordSubstitutionEffect{TerminalID: "terminal-a", BlockID: "page-three", ReplacementText: "CORRUPTED PAGE THREE"}},
			},
		}},
	}

	projection := projectFacility(authored, completed, facility, "terminal-a")
	record := findContentNode(projection.Tree, "damaged-record")
	require.NotNil(t, record)
	assert.Equal(t, "CORRUPTED PAGE ONE\n\nCOMPLETED PAGE TWO\n\nCORRUPTED PAGE THREE", record.Description)
	assert.Len(t, record.Blocks, 3, "diagnostics select authored variants without deleting source pages")
}

func TestFacilityDiagnosticProjectionIsStableAcrossRepeatedDisplayEffects(t *testing.T) {
	t.Parallel()

	authored := facilityProjectionTreeForTest()
	completed := facilityCompletedStatesForTest()
	facility := facilityProjectionStateForTest("open", true)
	facility.Conditions[0].Effects = append(facility.Conditions[0].Effects,
		domain.DiagnosticEffect{DisplayInstability: &domain.DisplayInstabilityEffect{}},
		domain.DiagnosticEffect{CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityViewEntry}},
	)
	authoredBefore := domain.CloneContentNode(authored)
	completedBefore := domain.CloneCommandExecutionStates(completed)
	facilityBefore := domain.CloneFacility(facility)
	want := projectFacility(authored, completed, facility, "terminal-a")

	for range 100 {
		got := projectFacility(authored, completed, facility, "terminal-a")
		assert.Empty(t, cmp.Diff(want, got))
	}
	assert.Equal(t, []domain.TerminalPresentationEffect{domain.TerminalPresentationEffectDisplayUnstable}, want.Effects)
	assert.True(t, want.BlockedCapabilities[domain.FacilityCapabilityViewEntry])
	assert.Empty(t, cmp.Diff(authoredBefore, authored))
	assert.Empty(t, cmp.Diff(completedBefore, completed))
	assert.Empty(t, cmp.Diff(facilityBefore, facility))
}

func TestFacilityDiagnosticCategoriesAreStableAcross100Replays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		category       domain.DiagnosticConditionCategory
		customCategory string
	}{
		{name: "offline", category: domain.DiagnosticConditionCategoryOffline},
		{name: "unpowered", category: domain.DiagnosticConditionCategoryUnpowered},
		{name: "network isolated", category: domain.DiagnosticConditionCategoryNetworkIsolated},
		{name: "storage damaged", category: domain.DiagnosticConditionCategoryStorageDamaged},
		{name: "authorization corrupted", category: domain.DiagnosticConditionCategoryAuthorizationCorrupted},
		{name: "display unstable", category: domain.DiagnosticConditionCategoryDisplayUnstable},
		{name: "authored custom fault", category: domain.DiagnosticConditionCategoryCustom, customCategory: "coolant-contaminated"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			authored := facilityProjectionTreeForTest()
			authored.Children = append(authored.Children, domain.ContentNode{
				ID: "diagnostics", Type: domain.NodeFolder, Name: "DIAGNOSTICS",
				VisibleWhen: &domain.FacilityStateEquality{DeviceID: "door", StateID: "maintenance"},
				Children: []domain.ContentNode{{
					ID: "fault-report", Type: domain.NodeEntry, Name: "FAULT REPORT", Description: "AUTHORED REPORT",
				}},
			})
			completed := facilityCompletedStatesForTest()
			facility := facilityProjectionStateForTest("open", false)
			facility.Conditions = []domain.DiagnosticCondition{{
				ID: "replay-fault", Name: "Replay fault", Category: test.category,
				CustomCategory: test.customCategory,
				Terminal:       &domain.DiagnosticTerminalScope{TerminalID: "terminal-a"},
				Effects: []domain.DiagnosticEffect{
					{CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityViewEntry}},
					{DiagnosticPath: &domain.DiagnosticPathEffect{TerminalID: "terminal-a", NodeID: "fault-report"}},
					{RecordSubstitution: &domain.RecordSubstitutionEffect{
						TerminalID: "terminal-a", BlockID: "door-state", ReplacementText: "DIAGNOSTIC REPLAY",
					}},
					{DisplayInstability: &domain.DisplayInstabilityEffect{}},
				},
			}}
			runtime := &domain.TerminalRuntime{
				TerminalID: "terminal-a", TerminalName: "Terminal A",
				AuthoredTree: authored, Tree: domain.CloneContentNode(authored),
				CommandStates: completed,
				Nav:           domain.NavState{Path: []string{"root", "diagnostics"}, Mode: "list"},
			}
			authoredBefore := domain.CloneContentNode(authored)
			completedBefore := domain.CloneCommandExecutionStates(completed)
			facilityBefore := domain.CloneFacility(facility)
			runtimeBefore := cloneTerminalRuntimeForPreviewTest(runtime)

			activeFacility := domain.CloneFacility(facility)
			activeFacility.Conditions[0].CurrentActive = true
			wantProjection := projectFacility(authored, completed, activeFacility, "terminal-a")
			service := New(nil, nil)
			wantPreview, wantIssues := service.PreviewFacility(runtime, facility, domain.FacilityPreview{
				ExpectedFacilityRevision: facility.Revision,
				TerminalID:               runtime.TerminalID,
				Condition:                &domain.FacilityConditionPreview{ConditionID: "replay-fault", Active: true},
			})
			require.Empty(t, wantIssues)
			require.NotNil(t, wantPreview)
			require.True(t, wantProjection.BlockedCapabilities[domain.FacilityCapabilityViewEntry])
			require.Equal(t, []domain.TerminalPresentationEffect{domain.TerminalPresentationEffectDisplayUnstable}, wantProjection.Effects)
			require.Equal(t, []string{"root", "diagnostics"}, wantPreview.Nav.Path)
			require.NotNil(t, findContentNode(wantProjection.Tree, "fault-report"))
			entry := findContentNode(wantProjection.Tree, "door-status")
			require.NotNil(t, entry)
			require.Equal(t, facilityEntryText("DIAGNOSTIC REPLAY"), entry.Description)

			for range 100 {
				gotProjection := projectFacility(authored, completed, activeFacility, "terminal-a")
				assert.Empty(t, cmp.Diff(wantProjection, gotProjection))
				gotPreview, issues := service.PreviewFacility(runtime, facility, domain.FacilityPreview{
					ExpectedFacilityRevision: facility.Revision,
					TerminalID:               runtime.TerminalID,
					Condition:                &domain.FacilityConditionPreview{ConditionID: "replay-fault", Active: true},
				})
				require.Empty(t, issues)
				assert.Empty(t, cmp.Diff(wantPreview, gotPreview))
			}

			assert.Empty(t, cmp.Diff(authoredBefore, authored), "diagnostic replay mutated authored content")
			assert.Empty(t, cmp.Diff(completedBefore, completed), "diagnostic replay mutated completed command state")
			assert.Empty(t, cmp.Diff(facilityBefore, facility), "diagnostic replay mutated persistent facility state")
			assert.Empty(t, cmp.Diff(runtimeBefore, runtime), "diagnostic replay mutated terminal navigation or content")
		})
	}
}

func TestFacilityDiagnosticProjectionSuppressesConflictingRecordSubstitutions(t *testing.T) {
	t.Parallel()

	authored := facilityProjectionTreeForTest()
	facility := facilityProjectionStateForTest("sealed", true)
	facility.Conditions = append(facility.Conditions, domain.DiagnosticCondition{
		ID: "second-damaged-record", Name: "Second damaged record", CurrentActive: true,
		Terminal: &domain.DiagnosticTerminalScope{TerminalID: "terminal-a"},
		Effects: []domain.DiagnosticEffect{{RecordSubstitution: &domain.RecordSubstitutionEffect{
			TerminalID: "terminal-a", BlockID: "door-state", ReplacementText: "OTHER CORRUPTION",
		}}},
	})

	projection := projectFacility(authored, nil, facility, "terminal-a")
	entry := findContentNode(projection.Tree, "door-status")
	require.NotNil(t, entry)
	assert.Equal(t, facilityEntryText("BASE STATUS"), entry.Description,
		"an invalid overlap must not select either authored corruption arbitrarily")
}

func facilityProjectionTreeForTest() domain.ContentNode {
	return domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{
			{
				ID: "open-door", Type: domain.NodeCommand, Name: "OPEN DOOR", Text: "Base command result.",
				FacilityNameVariants: []domain.FacilityTextVariant{{
					When: domain.FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "DOOR OPEN",
				}},
			},
			{
				ID: "door-status", Type: domain.NodeEntry, Name: "DOOR STATUS",
				Blocks: []domain.EntryContentBlock{
					{ID: "header", InitialText: "HEADER"},
					{
						ID: "door-state", InitialText: "BASE STATUS",
						FacilityTextVariants: []domain.FacilityTextVariant{{
							When: domain.FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "DEVICE STATUS: OPEN",
						}},
					},
					{ID: "footer", InitialText: "FOOTER"},
				},
			},
		},
	}
}

func facilityCompletedStatesForTest() map[string]domain.CommandExecutionState {
	return map[string]domain.CommandExecutionState{
		"open-door": {
			CompletedName: "OPEN DOOR COMPLETE", ResultText: "Completed command result.",
			EntryContentChange: &domain.EntryContentChange{BlockID: "door-state", CompletedText: "COMPLETED STATUS"},
		},
	}
}

func facilityProjectionStateForTest(deviceState string, diagnosticActive bool) *domain.Facility {
	return &domain.Facility{
		Devices: []domain.FacilityDevice{
			facilityDeviceForProjectionTest("door", deviceState, "sealed", "maintenance", "open"),
		},
		Conditions: []domain.DiagnosticCondition{{
			ID: "damaged-door-record", Name: "Damaged door record",
			Category:      domain.DiagnosticConditionCategoryStorageDamaged,
			Terminal:      &domain.DiagnosticTerminalScope{TerminalID: "terminal-a"},
			CurrentActive: diagnosticActive,
			Effects: []domain.DiagnosticEffect{{RecordSubstitution: &domain.RecordSubstitutionEffect{
				TerminalID: "terminal-a", BlockID: "door-state", ReplacementText: "DIAGNOSTIC STATUS: CORRUPTED",
			}}},
		}},
	}
}

func facilityDeviceForProjectionTest(id, currentState string, states ...string) domain.FacilityDevice {
	device := domain.FacilityDevice{
		ID: id, Name: id, Kind: domain.FacilityDeviceKindCustom,
		InitialStateID: states[0], CurrentStateID: currentState,
		States: make([]domain.FacilityDeviceState, len(states)),
	}
	for index, stateID := range states {
		device.States[index] = domain.FacilityDeviceState{ID: stateID, Name: stateID}
	}
	return device
}

func facilityEntryText(status string) string {
	return "HEADER\n\n" + status + "\n\nFOOTER"
}
