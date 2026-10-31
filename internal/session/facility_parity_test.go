package session

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
)

var updateDemoVerdicts = flag.Bool("update-demo-verdicts", false, "rewrite testdata/demo-transition-verdicts.json")

const demoVerdictsPath = "testdata/demo-transition-verdicts.json"

// demoTransitionVerdict is one row of the golden table that the Overseer's
// card verdict is checked against in the browser (SC-002).
type demoTransitionVerdict struct {
	DeviceID     string `json:"deviceId"`
	TransitionID string `json:"transitionId"`
	// Case is "met", or "unmet:<precondition device ID>" when that one
	// precondition's device is in another state.
	Case string `json:"case"`
	// States holds every device's current state for this case.
	States    map[string]string `json:"states"`
	Available bool              `json:"available"`
}

// TestDemoTransitionVerdictsMatchThePlayerCommandPath runs every demo
// transition through the card operation and through a player command from the
// same facility state, with no fault blocking commands, and requires the same
// outcome (SC-002). The outcomes are the golden table the browser journey
// compares the card against.
func TestDemoTransitionVerdictsMatchThePlayerCommandPath(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "sessions", "demo.json"))
	require.NoError(t, err)
	demo, err := domain.DecodeSession(raw)
	require.NoError(t, err)
	require.NotNil(t, demo.Facility)

	var verdicts []demoTransitionVerdict
	for _, device := range demo.Facility.Devices {
		for _, transition := range device.Transitions {
			for _, parity := range demoTransitionCases(demo, device, transition) {
				card := runDemoParityCase(t, parity.session, func(ctx context.Context, service *Service, revision uint64) domain.FacilityOperationResult {
					return service.ApplyFacilityDeviceTransition(ctx, FacilityDeviceTransitionRequest{
						DeviceID: device.ID, TransitionID: transition.ID,
						ExpectedFacilityRevision: revision, CorrelationID: "parity-card",
					})
				})
				player := runDemoParityCase(t, parity.session, func(ctx context.Context, service *Service, revision uint64) domain.FacilityOperationResult {
					return service.ApplyWorldAction(ctx, WorldActionRequest{
						CorrelationID: "parity-player", TerminalID: demoParityTerminalID, CommandID: demoParityCommandID,
						ExpectedFacilityRevision: revision,
						Transitions:              []domain.FacilityTransitionRequest{{DeviceID: device.ID, TransitionID: transition.ID}},
					})
				})
				name := fmt.Sprintf("%s/%s/%s", device.ID, transition.ID, parity.name)
				assert.Equal(t, player.OK, card.OK, "%s: card %#v, player %#v", name, card.Failure, player.Failure)
				assert.Equal(t, player.Failure, card.Failure, name)
				verdicts = append(verdicts, demoTransitionVerdict{
					DeviceID: device.ID, TransitionID: transition.ID, Case: parity.name,
					States: parity.states, Available: card.OK,
				})
			}
		}
	}

	encoded, err := json.MarshalIndent(verdicts, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')
	if *updateDemoVerdicts {
		require.NoError(t, os.WriteFile(demoVerdictsPath, encoded, 0o644))
	}
	golden, err := os.ReadFile(demoVerdictsPath)
	require.NoError(t, err)
	assert.JSONEq(t, string(golden), string(encoded),
		"demo verdicts changed; rerun with -update-demo-verdicts and review the diff")
}

const (
	demoParityTerminalID = "t_demo1"
	demoParityCommandID  = "parity-command"
)

type demoParityCase struct {
	name    string
	session domain.Session
	states  map[string]string
}

// demoTransitionCases builds the states where every precondition holds and
// where each precondition in turn fails. Every fault is cleared so no
// capability block decides the outcome.
func demoTransitionCases(
	demo domain.Session,
	device domain.FacilityDevice,
	transition domain.FacilityDeviceTransition,
) []demoParityCase {
	build := func(name string, unmet int) demoParityCase {
		session := domain.CloneSession(demo)
		facility := session.Facility
		for index := range facility.Conditions {
			facility.Conditions[index].CurrentActive = false
		}
		facilityDevice(facility, device.ID).CurrentStateID = transition.SourceStateID
		for index, precondition := range transition.Preconditions {
			target := facilityDevice(facility, precondition.DeviceID)
			target.CurrentStateID = precondition.StateID
			if index == unmet {
				target.CurrentStateID = otherDeviceState(*target, precondition.StateID)
			}
		}
		terminal := &session.Terminals[0]
		for index := range session.Terminals {
			if session.Terminals[index].ID == demoParityTerminalID {
				terminal = &session.Terminals[index]
			}
		}
		terminal.Root.Children = append(terminal.Root.Children, domain.ContentNode{
			ID: demoParityCommandID, Type: domain.NodeCommand, Name: "PARITY", Text: "Parity.",
			StateChange: &domain.StateChangeConfig{
				CompletedName: "PARITY COMPLETE", ConfirmationText: "Authorize parity?",
				FacilityAction: &domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
					Transitions: []domain.FacilityTransitionRequest{{DeviceID: device.ID, TransitionID: transition.ID}},
				}},
			},
		})
		states := make(map[string]string, len(facility.Devices))
		for _, value := range facility.Devices {
			states[value.ID] = value.CurrentStateID
		}
		return demoParityCase{name: name, session: session, states: states}
	}
	cases := []demoParityCase{build("met", -1)}
	for index, precondition := range transition.Preconditions {
		cases = append(cases, build("unmet:"+precondition.DeviceID, index))
	}
	return cases
}

func otherDeviceState(device domain.FacilityDevice, stateID string) string {
	for _, state := range device.States {
		if state.ID != stateID {
			return state.ID
		}
	}
	return stateID
}

// runDemoParityCase opens session in a fresh service and runs call at its
// facility revision.
func runDemoParityCase(
	t *testing.T,
	session domain.Session,
	call func(context.Context, *Service, uint64) domain.FacilityOperationResult,
) domain.FacilityOperationResult {
	t.Helper()
	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "demo-parity.json")
	fileSystem.SeedFile(target, mustEncodeSession(t, session))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	return call(t.Context(), service, session.Facility.Revision)
}
