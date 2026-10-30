package session

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetSceneRestoresWholeSceneInOneWrite(t *testing.T) {
	t.Parallel()

	initial := sceneResetSession()
	service, fileSystem, target := openSceneResetService(t, "scene-reset-whole.json", initial)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.ResetScene(t.Context(), SceneResetRequest{
		ExpectedSessionRevision:  0,
		ExpectedFacilityRevision: initial.Facility.Revision,
		CorrelationID:            "reset-scene",
	})

	require.True(t, result.OK, "ResetScene() = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, "reset-scene", result.CorrelationID)
	assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
	assert.Equal(t, uint64(1), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(10), result.ResultingFacilityRevision)
	assert.Equal(t, []string{"door", "power"}, result.AffectedDeviceIDs)
	assert.Equal(t,
		[]string{"door-alarm", "door-offline", "power-fault", "terminal-fault"},
		result.AffectedConditionIDs,
	)
	assert.Equal(t, []CommandReference{
		{TerminalID: "terminal", CommandID: "restore-and-open"},
		{TerminalID: "terminal", CommandID: "seal-archive"},
		{TerminalID: "terminal-b", CommandID: "vent-reactor"},
	}, result.ResetCommands)
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()), "scene reset must be one durable write")

	want := sceneAtStartingValues(initial)
	want.Facility.Revision = 10
	require.NotNil(t, result.Session)
	assert.Equal(t, want, *result.Session)

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, want, persisted)

	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Equal(t, want, *active.Session)
	assert.Equal(t, uint64(1), active.SavedRevision)
	assert.Equal(t, uint64(1), active.RequestedRevision)
	assert.Equal(t, SaveStateSaved, active.SaveState)
}

func TestResetSceneStepsFacilityRevisionOnlyWhenFacilityChanged(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                 string
		session              func() domain.Session
		wantFacilityRevision uint64
		wantAffectedDevices  []string
		wantAffectedFaults   []string
		wantResetCommands    []CommandReference
	}{
		{
			name: "command states only",
			session: func() domain.Session {
				session := sceneResetSession()
				*session.Facility = *sceneAtStartingValues(session).Facility
				return session
			},
			wantFacilityRevision: 9,
			wantResetCommands: []CommandReference{
				{TerminalID: "terminal", CommandID: "restore-and-open"},
				{TerminalID: "terminal", CommandID: "seal-archive"},
				{TerminalID: "terminal-b", CommandID: "vent-reactor"},
			},
		},
		{
			name: "facility only",
			session: func() domain.Session {
				session := sceneResetSession()
				for index := range session.Terminals {
					session.Terminals[index].CommandStates = nil
				}
				return session
			},
			wantFacilityRevision: 10,
			wantAffectedDevices:  []string{"door", "power"},
			wantAffectedFaults:   []string{"door-alarm", "door-offline", "power-fault", "terminal-fault"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			initial := test.session()
			service, fileSystem, target := openSceneResetService(t, "scene-reset-partial-"+test.name+".json", initial)
			writesBefore := len(fileSystem.WriteCalls())

			result := service.ResetScene(t.Context(), SceneResetRequest{
				ExpectedSessionRevision:  0,
				ExpectedFacilityRevision: initial.Facility.Revision,
				CorrelationID:            "reset-scene-partial",
			})

			require.True(t, result.OK, "ResetScene() = %#v", result)
			assert.True(t, result.Changed)
			assert.Equal(t, uint64(1), result.SessionRevision)
			assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
			assert.Equal(t, test.wantFacilityRevision, result.ResultingFacilityRevision)
			assert.Equal(t, test.wantAffectedDevices, nilIfEmpty(result.AffectedDeviceIDs))
			assert.Equal(t, test.wantAffectedFaults, nilIfEmpty(result.AffectedConditionIDs))
			assert.Equal(t, test.wantResetCommands, nilIfEmpty(result.ResetCommands))
			assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))

			want := sceneAtStartingValues(initial)
			want.Facility.Revision = test.wantFacilityRevision
			require.NotNil(t, result.Session)
			assert.Equal(t, want, *result.Session)

			persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
			require.NoError(t, err)
			assert.Equal(t, want, persisted)
		})
	}
}

func TestResetSceneRejectsStaleRevisionWithoutMutation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		// advance moves the canonical session past the revisions the caller
		// counted against and returns those now-stale revisions.
		advance func(*testing.T, *Service, domain.Session) (sessionRevision, facilityRevision uint64)
	}{
		{
			name: "session revision",
			advance: func(t *testing.T, service *Service, initial domain.Session) (uint64, uint64) {
				t.Helper()
				reset := service.ResetCommandState(t.Context(), "terminal-b", "vent-reactor")
				require.True(t, reset.OK, "ResetCommandState() = %#v", reset)
				require.True(t, reset.Changed)
				require.Equal(t, uint64(1), reset.Revision)
				return 0, initial.Facility.Revision
			},
		},
		{
			name: "facility revision",
			advance: func(t *testing.T, _ *Service, initial domain.Session) (uint64, uint64) {
				t.Helper()
				return 0, initial.Facility.Revision - 1
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			initial := sceneResetSession()
			service, fileSystem, target := openSceneResetService(t, "scene-reset-stale-"+test.name+".json", initial)
			sessionRevision, facilityRevision := test.advance(t, service, initial)
			before := service.Snapshot()
			require.NotNil(t, before.Session)
			beforeData := fileSystemFileData(t, fileSystem, target)
			writesBefore := len(fileSystem.WriteCalls())

			result := service.ResetScene(t.Context(), SceneResetRequest{
				ExpectedSessionRevision:  sessionRevision,
				ExpectedFacilityRevision: facilityRevision,
				CorrelationID:            "reset-scene-stale",
			})

			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.Equal(t, domain.FacilityFailureStaleRevision, result.Failure)
			assert.Equal(t, before.SavedRevision, result.SessionRevision)
			assert.Equal(t, initial.Facility.Revision, result.PreviousFacilityRevision)
			assert.Equal(t, initial.Facility.Revision, result.ResultingFacilityRevision)
			assert.Empty(t, result.AffectedDeviceIDs)
			assert.Empty(t, result.AffectedConditionIDs)
			assert.Empty(t, result.ResetCommands)
			require.NotNil(t, result.Session, "a stale reset returns the canonical session for fresh counts")
			assert.Equal(t, *before.Session, *result.Session)

			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
			assert.Equal(t, beforeData, fileSystemFileData(t, fileSystem, target))
			assert.Equal(t, before, service.Snapshot())
		})
	}
}

func TestResetSceneStorageFailureRollsBackCompleteCandidate(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "scene-reset-write-failure.json")
	initial := sceneResetSession()
	initialData := mustEncodeSession(t, initial)
	store := &failingMutationStore{
		path: target, data: initialData, err: fmt.Errorf("injected scene reset failure"),
	}
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	request := SceneResetRequest{
		ExpectedSessionRevision:  0,
		ExpectedFacilityRevision: initial.Facility.Revision,
		CorrelationID:            "reset-scene-failure",
	}

	result := service.ResetScene(t.Context(), request)

	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
	assert.Equal(t, uint64(0), result.SessionRevision)
	assert.Equal(t, initial.Facility.Revision, result.PreviousFacilityRevision)
	assert.Equal(t, initial.Facility.Revision, result.ResultingFacilityRevision)
	assert.Empty(t, result.ResetCommands, "a failed reset must not report commands as reset")
	assert.Equal(t, 1, store.writes, "the whole scene must be attempted as one write")
	assert.Equal(t, initialData, store.data)

	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Equal(t, initial, *active.Session, "devices, faults, and command states must all be rolled back")
	assert.Zero(t, active.RequestedRevision)
	assert.Zero(t, active.SavedRevision)
	assert.Equal(t, uint64(1), active.AuthoringRevision)

	// The rollback restores the exact revisions the caller counted against,
	// so the same confirmation can be retried once storage recovers.
	store.err = nil
	retried := service.ResetScene(t.Context(), request)

	require.True(t, retried.OK, "retried ResetScene() = %#v", retried)
	assert.True(t, retried.Changed)
	assert.Equal(t, uint64(1), retried.SessionRevision)
	assert.Equal(t, uint64(10), retried.ResultingFacilityRevision)
	assert.Equal(t, 2, store.writes)
	want := sceneAtStartingValues(initial)
	want.Facility.Revision = 10
	persisted, err := domain.DecodeSession(store.data)
	require.NoError(t, err)
	assert.Equal(t, want, persisted)
}

func TestResetSceneKeepsAuthoringRevision(t *testing.T) {
	t.Parallel()

	initial := sceneResetSession()
	service, _, _ := openSceneResetService(t, "scene-reset-authoring-revision.json", initial)
	regrouped := service.ReplaceTerminalGroups(t.Context(), []domain.TerminalGroup{
		{ID: "facility-group", Name: "Facility", TerminalIDs: []string{"terminal"}},
		{ID: "reactor-group", Name: "Reactor", TerminalIDs: []string{"terminal-b"}},
	}, 0)
	require.True(t, regrouped.OK, "ReplaceTerminalGroups() = %#v", regrouped)
	require.True(t, regrouped.Changed)
	require.Equal(t, uint64(2), regrouped.AuthoringRevision)

	result := service.ResetScene(t.Context(), SceneResetRequest{
		ExpectedSessionRevision:  regrouped.Revision,
		ExpectedFacilityRevision: initial.Facility.Revision,
		CorrelationID:            "reset-scene-authoring-revision",
	})

	require.True(t, result.OK, "ResetScene() = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, regrouped.Revision+1, result.SessionRevision)
	assert.Equal(t, uint64(2), result.AuthoringRevision, "a scene reset changes live values only")
	assert.Equal(t, uint64(2), service.Snapshot().AuthoringRevision)

	repeated := service.ResetScene(t.Context(), SceneResetRequest{
		ExpectedSessionRevision:  result.SessionRevision,
		ExpectedFacilityRevision: result.ResultingFacilityRevision,
		CorrelationID:            "reset-scene-authoring-revision-repeat",
	})

	require.True(t, repeated.OK, "repeated ResetScene() = %#v", repeated)
	assert.False(t, repeated.Changed)
	assert.Equal(t, uint64(2), repeated.AuthoringRevision)
	assert.Equal(t, uint64(2), service.Snapshot().AuthoringRevision)
}

func TestResetSceneAtStartingValuesIsNoOp(t *testing.T) {
	t.Parallel()

	initial := sceneAtStartingValues(sceneResetSession())
	service, fileSystem, target := openSceneResetService(t, "scene-reset-no-op.json", initial)
	initialData := fileSystemFileData(t, fileSystem, target)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.ResetScene(t.Context(), SceneResetRequest{
		ExpectedSessionRevision:  0,
		ExpectedFacilityRevision: initial.Facility.Revision,
		CorrelationID:            "reset-scene-no-op",
	})

	require.True(t, result.OK, "ResetScene() = %#v", result)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
	assert.Equal(t, uint64(0), result.SessionRevision)
	assert.Equal(t, initial.Facility.Revision, result.PreviousFacilityRevision)
	assert.Equal(t, initial.Facility.Revision, result.ResultingFacilityRevision)
	assert.Empty(t, result.AffectedDeviceIDs)
	assert.Empty(t, result.AffectedConditionIDs)
	assert.Empty(t, result.ResetCommands)
	assert.Equal(t, uint64(1), result.AuthoringRevision)
	require.NotNil(t, result.Session)
	assert.Equal(t, initial, *result.Session)

	assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
	assert.Equal(t, initialData, fileSystemFileData(t, fileSystem, target))
	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Equal(t, initial, *active.Session)
	assert.Zero(t, active.SavedRevision)
	assert.Equal(t, uint64(1), active.AuthoringRevision)
}

// openSceneResetService opens session from a fake file system and confirms
// that the fixture round-trips unchanged, so later equality checks compare
// only what the reset changed.
func openSceneResetService(
	t *testing.T,
	name string,
	session domain.Session,
) (*Service, *testutil.FakeFileSystem, string) {
	t.Helper()
	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, name)
	fileSystem.SeedFile(target, mustEncodeSession(t, session))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	require.Equal(t, session, *opened.Session)
	require.Equal(t, uint64(1), opened.AuthoringRevision)
	return service, fileSystem, target
}

// sceneResetSession is a facility away from its starting values with
// executed commands on two terminals.
func sceneResetSession() domain.Session {
	session := facilityResetSession()
	terminal := &session.Terminals[0]
	terminal.Root.Children = append(terminal.Root.Children, domain.ContentNode{
		ID: "seal-archive", Type: domain.NodeCommand, Name: "SEAL ARCHIVE",
		Text: "Archive sealed.",
		StateChange: &domain.StateChangeConfig{
			CompletedName: "ARCHIVE SEALED", ConfirmationText: "Seal the archive?",
		},
	})
	terminal.CommandStates = map[string]domain.CommandExecutionState{
		"restore-and-open": {CompletedName: "POWER RESTORED; DOOR OPEN", ResultText: "Power restored and door opened."},
		"seal-archive":     {CompletedName: "ARCHIVE SEALED", ResultText: "Archive sealed."},
	}
	session.Terminals = append(session.Terminals, domain.Terminal{
		ID: "terminal-b", Name: "Reactor terminal",
		Root: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: "vent-reactor", Type: domain.NodeCommand, Name: "VENT REACTOR",
				Text: "Reactor vented.",
				StateChange: &domain.StateChangeConfig{
					CompletedName: "REACTOR VENTED", ConfirmationText: "Vent the reactor?",
				},
			}},
		},
		CommandStates: map[string]domain.CommandExecutionState{
			"vent-reactor": {CompletedName: "REACTOR VENTED", ResultText: "Reactor vented."},
		},
	})
	session.TerminalGroups[0].TerminalIDs = append(session.TerminalGroups[0].TerminalIDs, "terminal-b")
	return session
}

// sceneAtStartingValues returns a copy of session with every device and fault
// at its starting value and no executed commands. The facility revision is
// left as is.
func sceneAtStartingValues(session domain.Session) domain.Session {
	reset := domain.CloneSession(session)
	for index := range reset.Facility.Devices {
		device := &reset.Facility.Devices[index]
		device.CurrentStateID = device.InitialStateID
	}
	for index := range reset.Facility.Conditions {
		condition := &reset.Facility.Conditions[index]
		condition.CurrentActive = condition.InitialActive
	}
	for index := range reset.Terminals {
		reset.Terminals[index].CommandStates = nil
	}
	return reset
}

// nilIfEmpty lets table expectations use nil for "none" whether the result
// carries a nil or an empty slice.
func nilIfEmpty[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	return values
}
