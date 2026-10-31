package main

import (
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/live"
	"github.com/stretchr/testify/require"
)

const lifecycleTestTerminalID = "terminal-lifecycle"

var lifecycleTestUnstable = []domain.TerminalPresentationEffect{domain.TerminalPresentationEffectDisplayUnstable}

func newLifecycleTestLifecycle() *fixtureFacilityLifecycle {
	return &fixtureFacilityLifecycle{
		base:        live.New(nil, nil),
		state:       &fixtureFacilityPlayerState{},
		diagnostics: &fixtureFacilityDiagnosticState{},
	}
}

// lifecycleTestTarget is a terminal whose entry depends on the door, so a
// fault scoped to the door applies to it.
func lifecycleTestTarget(effects ...domain.TerminalPresentationEffect) domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: lifecycleTestTerminalID, TerminalName: "LIFECYCLE",
		Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{{
			ID: "door-status", Type: domain.NodeEntry, Name: "DOOR STATUS",
			VisibleWhen: &domain.FacilityStateEquality{DeviceID: "door", StateID: "sealed"},
		}}},
		Effects: effects,
	}
}

// lifecycleTestFacility has one display fault, scoped to the test terminal
// or to the door it depends on, in the given position.
func lifecycleTestFacility(scope string, active bool) *domain.Facility {
	condition := domain.DiagnosticCondition{
		ID: "flicker", Name: "Flicker", Category: domain.DiagnosticConditionCategoryDisplayUnstable,
		InitialActive: false, CurrentActive: active,
		Effects: []domain.DiagnosticEffect{{DisplayInstability: &domain.DisplayInstabilityEffect{}}},
	}
	switch scope {
	case "terminal":
		condition.Terminal = &domain.DiagnosticTerminalScope{TerminalID: lifecycleTestTerminalID}
	case "device":
		condition.Device = &domain.DiagnosticDeviceScope{DeviceID: "door"}
	}
	return &domain.Facility{
		Revision: 1,
		Devices: []domain.FacilityDevice{{
			ID: "door", Name: "Door", Kind: domain.FacilityDeviceKindDoor,
			InitialStateID: "sealed", CurrentStateID: "sealed",
			States: []domain.FacilityDeviceState{{ID: "sealed", Name: "Sealed"}, {ID: "open", Name: "Open"}},
		}},
		Conditions: []domain.DiagnosticCondition{condition},
	}
}

func TestFixtureFacilityLifecycleProjectsAFaultSwitchedOff(t *testing.T) {
	t.Parallel()

	for _, scope := range []string{"terminal", "device"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()
			lifecycle := newLifecycleTestLifecycle()
			runtime, _ := lifecycle.CreateRuntime(lifecycleTestTarget())
			require.NotNil(t, runtime)

			for _, step := range []struct {
				active bool
				want   []domain.TerminalPresentationEffect
			}{
				{active: false, want: nil},
				{active: true, want: lifecycleTestUnstable},
				{active: false, want: nil},
				{active: true, want: lifecycleTestUnstable},
			} {
				projection := lifecycle.ProjectFacility(runtime, lifecycleTestFacility(scope, step.active))
				require.NotNil(t, projection)
				require.Equal(t, step.want, projection.Effects, "the projection with the fault active=%t", step.active)
				require.Equal(t, step.want, lifecycle.ProjectRuntime(runtime).Effects,
					"the checkpoint every later player update carries, with the fault active=%t", step.active)
			}
		})
	}
}

func TestFixtureFacilityLifecycleKeepsTargetEffectsOverTheFacility(t *testing.T) {
	t.Parallel()

	lifecycle := newLifecycleTestLifecycle()
	runtime, _ := lifecycle.CreateRuntime(lifecycleTestTarget(lifecycleTestUnstable...))
	require.NotNil(t, runtime)

	for _, active := range []bool{false, true, false} {
		projection := lifecycle.ProjectFacility(runtime, lifecycleTestFacility("terminal", active))
		require.Equal(t, lifecycleTestUnstable, projection.Effects,
			"a target's own effect stays whatever the fault, with the fault active=%t", active)
	}
	require.Equal(t, lifecycleTestUnstable, lifecycle.ProjectFacility(runtime, nil).Effects,
		"a target's own effect stays without a facility")

	require.NotNil(t, lifecycle.UpdateRuntime(runtime, lifecycleTestTarget()))
	require.Empty(t, lifecycle.ProjectFacility(runtime, lifecycleTestFacility("terminal", false)).Effects,
		"a target without effects clears the earlier target's effect")
}
