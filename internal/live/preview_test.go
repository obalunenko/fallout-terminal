package live

import (
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/stretchr/testify/require"
)

// previewDiffBudget is the Go time one demo-sized preview may spend (plan,
// Performance Goals).
const previewDiffBudget = 50 * time.Millisecond

func TestPreviewDifferencesNameEveryPlayerChangeWhenTheDemoDoorOpens(t *testing.T) {
	t.Parallel()

	session := previewDiffDemoSession(t)
	state := previewDiffTerminalRuntime(t, session, "t_demo1")
	summary := findContentNode(state.AuthoredTree, "n_entry_vault_shutdown")
	require.NotNil(t, summary)
	require.Equal(t, "СВОДКА ЭВАКУАЦИИ", summary.Name)
	require.Equal(t, "b_vault_door_status", summary.Blocks[2].ID, "block 3 of the evacuation summary")
	require.Equal(t, "sealed", previewDiffDeviceState(t, session.Facility, "vault-door-main"))

	differences, issues := PreviewDifferences(state, session.Facility, previewDiffDeviceOverride(
		session.Facility, "t_demo1", "vault-door-main", "open",
	))

	require.Empty(t, issues)
	want := []PreviewDifference{
		{
			Kind: PreviewDifferenceTextChanged, NodeID: "n_entry_vault_shutdown", BlockID: new("b_vault_door_status"),
			Before: new("ГЕРМОДВЕРЬ: ЗАПЕРТА"), After: new("ГЕРМОДВЕРЬ: ОТКРЫТА // МАРШРУТ СВОБОДЕН"),
		},
		{Kind: PreviewDifferenceAppears, NodeID: "n_entry_open_route", After: new("НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН")},
		{
			Kind: PreviewDifferenceNameChanged, NodeID: "n_cmd_open_evacuation",
			Before: new("Открыть выход и подать сигнал"), After: new("Выход уже открыт"),
		},
	}
	require.Empty(t, cmp.Diff(want, differences))
}

func TestPreviewDifferencesReportAvailabilityChangesOnTheDemo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		currentState string
		previewState string
		want         []PreviewDifference
	}{
		{
			name: "power lost", currentState: "online", previewState: "offline",
			want: []PreviewDifference{
				{
					Kind: PreviewDifferenceTextChanged, NodeID: "n_entry_facility_status", BlockID: new("b_grid_status"),
					Before: new("РЕЗЕРВНАЯ СЕТЬ: В НОРМЕ // 88% НАГРУЗКИ"), After: new("РЕЗЕРВНАЯ СЕТЬ: НЕТ ПИТАНИЯ"),
				},
				{Kind: PreviewDifferenceAppears, NodeID: "n_folder_diagnostics", After: new("АВАРИЙНАЯ ДИАГНОСТИКА")},
				{
					Kind: PreviewDifferenceNameChanged, NodeID: "n_cmd_prime_grid",
					Before: new("Резервная сеть работает"), After: new("Резервная сеть запущена"),
				},
				{Kind: PreviewDifferenceBecomesUnavailable, NodeID: "n_cmd_open_evacuation"},
			},
		},
		{
			name: "power restored", currentState: "offline", previewState: "online",
			want: []PreviewDifference{
				{
					Kind: PreviewDifferenceTextChanged, NodeID: "n_entry_facility_status", BlockID: new("b_grid_status"),
					Before: new("РЕЗЕРВНАЯ СЕТЬ: НЕТ ПИТАНИЯ"), After: new("РЕЗЕРВНАЯ СЕТЬ: В НОРМЕ // 88% НАГРУЗКИ"),
				},
				{Kind: PreviewDifferenceDisappears, NodeID: "n_folder_diagnostics", Before: new("АВАРИЙНАЯ ДИАГНОСТИКА")},
				{
					Kind: PreviewDifferenceNameChanged, NodeID: "n_cmd_prime_grid",
					Before: new("Резервная сеть запущена"), After: new("Резервная сеть работает"),
				},
				{Kind: PreviewDifferenceBecomesAvailable, NodeID: "n_cmd_open_evacuation"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := previewDiffDemoSession(t)
			previewDiffSetDeviceState(t, session.Facility, "power-grid-main", test.currentState)
			state := previewDiffTerminalRuntime(t, session, "t_demo1")

			differences, issues := PreviewDifferences(state, session.Facility, previewDiffDeviceOverride(
				session.Facility, "t_demo1", "power-grid-main", test.previewState,
			))

			require.Empty(t, issues)
			require.Empty(t, cmp.Diff(test.want, differences))
		})
	}
}

func TestPreviewDifferencesReportAFaultForcingADiagnosticPathOnTheDemo(t *testing.T) {
	t.Parallel()

	session := previewDiffDemoSession(t)
	state := previewDiffTerminalRuntime(t, session, "t_demo1")

	differences, issues := PreviewDifferences(state, session.Facility, domain.FacilityPreview{
		ExpectedFacilityRevision: session.Facility.Revision,
		TerminalID:               "t_demo1",
		Condition:                &domain.FacilityConditionPreview{ConditionID: "door-offline", Active: true},
	})

	require.Empty(t, issues)
	require.Empty(t, cmp.Diff([]PreviewDifference{
		{Kind: PreviewDifferenceAppears, NodeID: "n_folder_diagnostics", After: new("АВАРИЙНАЯ ДИАГНОСТИКА")},
	}, differences), "a forced diagnostic path appears once, at its highest newly visible node")
}

func TestPreviewDifferencesFollowTreeOrderThenBlockOrder(t *testing.T) {
	t.Parallel()

	state := &domain.TerminalRuntime{TerminalID: "terminal-a", AuthoredTree: previewDiffOrderingTree()}
	facility := &domain.Facility{
		Revision: 3,
		Devices:  []domain.FacilityDevice{facilityDeviceForProjectionTest("door", "sealed", "sealed", "maintenance", "open")},
	}

	differences, issues := PreviewDifferences(state, facility, previewDiffDeviceOverride(facility, "terminal-a", "door", "open"))

	require.Empty(t, issues)
	want := []PreviewDifference{
		{Kind: PreviewDifferenceAppears, NodeID: "route", After: new("ROUTE")},
		{Kind: PreviewDifferenceNameChanged, NodeID: "seal-door", Before: new("SEAL DOOR"), After: new("DOOR IS OPEN")},
		{Kind: PreviewDifferenceBecomesUnavailable, NodeID: "seal-door"},
		{
			Kind: PreviewDifferenceTextChanged, NodeID: "door-log", BlockID: new("log-first"),
			Before: new("FIRST SEALED"), After: new("FIRST OPEN"),
		},
		{
			Kind: PreviewDifferenceTextChanged, NodeID: "door-log", BlockID: new("log-third"),
			Before: new("THIRD SEALED"), After: new("THIRD OPEN"),
		},
		{Kind: PreviewDifferenceDisappears, NodeID: "sealed-wing", Before: new("SEALED WING")},
		{Kind: PreviewDifferenceBecomesAvailable, NodeID: "leave-vault"},
	}
	require.Empty(t, cmp.Diff(want, differences),
		"node differences precede their blocks, hidden nodes and disappearing subtrees add nothing")
}

func TestPreviewDifferencesApplyCommandStatesAndDiagnosticsToBothSides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		preview func(*domain.Facility) domain.FacilityPreview
		want    []PreviewDifference
	}{
		{
			name: "device state outranks the executed command",
			preview: func(facility *domain.Facility) domain.FacilityPreview {
				return previewDiffDeviceOverride(facility, "terminal-a", "door", "open")
			},
			want: []PreviewDifference{
				{Kind: PreviewDifferenceNameChanged, NodeID: "open-door", Before: new("OPEN DOOR COMPLETE"), After: new("DOOR OPEN")},
				{
					Kind: PreviewDifferenceTextChanged, NodeID: "door-status", BlockID: new("door-state"),
					Before: new("COMPLETED STATUS"), After: new("DEVICE STATUS: OPEN"),
				},
			},
		},
		{
			name: "diagnostic substitution outranks the executed command",
			preview: func(facility *domain.Facility) domain.FacilityPreview {
				return domain.FacilityPreview{
					ExpectedFacilityRevision: facility.Revision, TerminalID: "terminal-a",
					Condition: &domain.FacilityConditionPreview{ConditionID: "damaged-door-record", Active: true},
				}
			},
			want: []PreviewDifference{{
				Kind: PreviewDifferenceTextChanged, NodeID: "door-status", BlockID: new("door-state"),
				Before: new("COMPLETED STATUS"), After: new("DIAGNOSTIC STATUS: CORRUPTED"),
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			state := &domain.TerminalRuntime{
				TerminalID: "terminal-a", AuthoredTree: facilityProjectionTreeForTest(),
				CommandStates: facilityCompletedStatesForTest(),
			}
			facility := facilityProjectionStateForTest("sealed", false)

			differences, issues := PreviewDifferences(state, facility, test.preview(facility))

			require.Empty(t, issues)
			require.Empty(t, cmp.Diff(test.want, differences))
		})
	}
}

func TestPreviewDifferencesAreEmptyWhenTheOverrideKeepsTheCurrentState(t *testing.T) {
	t.Parallel()

	session := previewDiffDemoSession(t)
	state := previewDiffTerminalRuntime(t, session, "t_demo1")
	tests := []struct {
		name    string
		preview domain.FacilityPreview
	}{
		{
			name:    "device",
			preview: previewDiffDeviceOverride(session.Facility, "t_demo1", "vault-door-main", "sealed"),
		},
		{
			name: "fault",
			preview: domain.FacilityPreview{
				ExpectedFacilityRevision: session.Facility.Revision, TerminalID: "t_demo1",
				Condition: &domain.FacilityConditionPreview{ConditionID: "door-offline", Active: false},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			differences, issues := PreviewDifferences(state, session.Facility, test.preview)

			require.Empty(t, issues)
			require.Empty(t, differences, "an identical state changes nothing players see")
		})
	}
}

func TestPreviewDifferencesNeverMutateInputs(t *testing.T) {
	t.Parallel()

	session := previewDiffDemoSession(t)
	state := previewDiffTerminalRuntime(t, session, "t_demo1")
	stateBefore := cloneTerminalRuntimeForPreviewTest(state)
	facilityBefore := domain.CloneFacility(session.Facility)
	preview := previewDiffDeviceOverride(session.Facility, "t_demo1", "vault-door-main", "open")
	previewBefore := preview
	previewBefore.DeviceState = new(*preview.DeviceState)

	first, firstIssues := PreviewDifferences(state, session.Facility, preview)
	require.Empty(t, firstIssues)
	require.NotEmpty(t, first)
	*first[0].Before = "TAMPERED"
	first[0].NodeID = "tampered"
	second, secondIssues := PreviewDifferences(state, session.Facility, preview)

	require.Empty(t, secondIssues)
	require.Equal(t, "n_entry_vault_shutdown", second[0].NodeID)
	require.Equal(t, "ГЕРМОДВЕРЬ: ЗАПЕРТА", *second[0].Before)
	require.Empty(t, cmp.Diff(stateBefore, state), "preview differences mutated the terminal checkpoint")
	require.Empty(t, cmp.Diff(facilityBefore, session.Facility), "preview differences mutated the facility")
	require.Empty(t, cmp.Diff(previewBefore, preview), "preview differences mutated the override")
}

func TestPreviewDifferencesRejectInvalidOverridesLikePreviewFacility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		preview     func(*domain.Facility) domain.FacilityPreview
		wantFailure domain.FacilityFailureCode
	}{
		{
			name: "stale revision",
			preview: func(facility *domain.Facility) domain.FacilityPreview {
				preview := previewDiffDeviceOverride(facility, "t_demo1", "vault-door-main", "open")
				preview.ExpectedFacilityRevision++
				return preview
			},
			wantFailure: domain.FacilityFailureStaleRevision,
		},
		{
			name: "other terminal",
			preview: func(facility *domain.Facility) domain.FacilityPreview {
				return previewDiffDeviceOverride(facility, "t_demo2", "vault-door-main", "open")
			},
			wantFailure: domain.FacilityFailureMissingReference,
		},
		{
			name: "unknown device",
			preview: func(facility *domain.Facility) domain.FacilityPreview {
				return previewDiffDeviceOverride(facility, "t_demo1", "missing-device", "open")
			},
			wantFailure: domain.FacilityFailureMissingReference,
		},
		{
			name: "unknown state",
			preview: func(facility *domain.Facility) domain.FacilityPreview {
				return previewDiffDeviceOverride(facility, "t_demo1", "vault-door-main", "missing-state")
			},
			wantFailure: domain.FacilityFailureMissingReference,
		},
		{
			name: "missing override",
			preview: func(facility *domain.Facility) domain.FacilityPreview {
				return domain.FacilityPreview{ExpectedFacilityRevision: facility.Revision, TerminalID: "t_demo1"}
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			session := previewDiffDemoSession(t)
			state := previewDiffTerminalRuntime(t, session, "t_demo1")
			facilityBefore := domain.CloneFacility(session.Facility)

			differences, issues := PreviewDifferences(state, session.Facility, test.preview(session.Facility))

			require.Nil(t, differences)
			require.Len(t, issues, 1)
			require.Equal(t, test.wantFailure, issues[0].Code)
			require.Empty(t, cmp.Diff(facilityBefore, session.Facility))
		})
	}
}

func BenchmarkPreviewDifferencesDemo(b *testing.B) {
	session := previewDiffDemoSession(b)
	state := previewDiffTerminalRuntime(b, session, "t_demo1")
	preview := previewDiffDeviceOverride(session.Facility, "t_demo1", "vault-door-main", "open")

	for b.Loop() {
		if _, issues := PreviewDifferences(state, session.Facility, preview); len(issues) != 0 {
			b.Fatalf("PreviewDifferences() issues = %#v", issues)
		}
	}

	if perPreview := b.Elapsed() / time.Duration(b.N); perPreview > previewDiffBudget {
		b.Fatalf("PreviewDifferences() took %s per demo preview, budget %s", perPreview, previewDiffBudget)
	}
}

// previewDiffOrderingTree has one device, door, driving every difference
// kind, a node hidden on both sides, and a subtree that disappears whole.
func previewDiffOrderingTree() domain.ContentNode {
	whenOpen := domain.FacilityStateEquality{DeviceID: "door", StateID: "open"}
	whenSealed := domain.FacilityStateEquality{DeviceID: "door", StateID: "sealed"}
	whenMaintenance := domain.FacilityStateEquality{DeviceID: "door", StateID: "maintenance"}
	openText := func(text string) []domain.FacilityTextVariant {
		return []domain.FacilityTextVariant{{When: whenOpen, Text: text}}
	}
	return domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{
			{ID: "route", Type: domain.NodeEntry, Name: "ROUTE", Description: "Route open.", VisibleWhen: &whenOpen},
			{
				ID: "door-controls", Type: domain.NodeFolder, Name: "DOOR CONTROLS",
				Children: []domain.ContentNode{
					{
						ID: "seal-door", Type: domain.NodeCommand, Name: "SEAL DOOR", Text: "Sealed.",
						FacilityNameVariants: openText("DOOR IS OPEN"), AvailableWhen: &whenSealed,
					},
					{
						ID: "door-log", Type: domain.NodeEntry, Name: "DOOR LOG",
						Blocks: []domain.EntryContentBlock{
							{ID: "log-first", InitialText: "FIRST SEALED", FacilityTextVariants: openText("FIRST OPEN")},
							{ID: "log-second", InitialText: "SECOND"},
							{ID: "log-third", InitialText: "THIRD SEALED", FacilityTextVariants: openText("THIRD OPEN")},
						},
					},
				},
			},
			{
				ID: "maintenance-log", Type: domain.NodeEntry, Name: "MAINTENANCE LOG", VisibleWhen: &whenMaintenance,
				FacilityNameVariants: openText("MAINTENANCE LOG OPEN"),
			},
			{
				ID: "sealed-wing", Type: domain.NodeFolder, Name: "SEALED WING", VisibleWhen: &whenSealed,
				Children: []domain.ContentNode{{
					ID: "wing-log", Type: domain.NodeEntry, Name: "WING LOG",
					Blocks: []domain.EntryContentBlock{
						{ID: "wing-status", InitialText: "WING SEALED", FacilityTextVariants: openText("WING OPEN")},
					},
				}},
			},
			{ID: "leave-vault", Type: domain.NodeCommand, Name: "LEAVE VAULT", Text: "Left.", AvailableWhen: &whenOpen},
		},
	}
}

func previewDiffDemoSession(tb testing.TB) domain.Session {
	tb.Helper()
	raw, err := os.ReadFile("../../sessions/demo.json")
	require.NoError(tb, err)
	session, err := domain.DecodeSession(raw)
	require.NoError(tb, err)
	require.NotNil(tb, session.Facility)
	return session
}

func previewDiffTerminalRuntime(tb testing.TB, session domain.Session, terminalID string) *domain.TerminalRuntime {
	tb.Helper()
	for _, terminal := range session.Terminals {
		if terminal.ID == terminalID {
			return &domain.TerminalRuntime{
				TerminalID: terminal.ID, TerminalName: terminal.Name,
				AuthoredTree: domain.CloneContentNode(terminal.Root), Tree: domain.CloneContentNode(terminal.Root),
				CommandStates: domain.CloneCommandExecutionStates(terminal.CommandStates),
			}
		}
	}
	require.FailNow(tb, "demo terminal not found", terminalID)
	return nil
}

func previewDiffDeviceOverride(facility *domain.Facility, terminalID, deviceID, stateID string) domain.FacilityPreview {
	return domain.FacilityPreview{
		ExpectedFacilityRevision: facility.Revision,
		TerminalID:               terminalID,
		DeviceState:              &domain.FacilityDeviceStatePreview{DeviceID: deviceID, StateID: stateID},
	}
}

func previewDiffDeviceState(tb testing.TB, facility *domain.Facility, deviceID string) string {
	tb.Helper()
	for _, device := range facility.Devices {
		if device.ID == deviceID {
			return device.CurrentStateID
		}
	}
	require.FailNow(tb, "device not found", deviceID)
	return ""
}

func previewDiffSetDeviceState(tb testing.TB, facility *domain.Facility, deviceID, stateID string) {
	tb.Helper()
	for index := range facility.Devices {
		if facility.Devices[index].ID == deviceID {
			facility.Devices[index].CurrentStateID = stateID
			return
		}
	}
	require.FailNow(tb, "device not found", deviceID)
}
