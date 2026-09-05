package session

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyWorldActionCommitsMultiDeviceCandidateAtomically(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-world-action.json")
	initial := facilityWorldActionSession()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID:            "request-atomic",
		TerminalID:               "terminal",
		CommandID:                "restore-and-open",
		ExpectedFacilityRevision: 9,
		Transitions: []domain.FacilityTransitionRequest{
			{DeviceID: "power", TransitionID: "restore"},
			{DeviceID: "door", TransitionID: "open"},
		},
	})

	require.True(t, result.OK, "ApplyWorldAction() = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, "request-atomic", result.CorrelationID)
	assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
	assert.Equal(t, uint64(1), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(10), result.ResultingFacilityRevision)
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))
	require.NotNil(t, result.Session)
	require.NotNil(t, result.Session.Facility)
	assert.Equal(t, uint64(10), result.Session.Facility.Revision)
	assert.Equal(t, "online", facilityDeviceByID(t, result.Session.Facility, "power").CurrentStateID)
	assert.Equal(t, "open", facilityDeviceByID(t, result.Session.Facility, "door").CurrentStateID)
	assert.False(t, facilityConditionByID(t, result.Session.Facility, "door-offline").CurrentActive)
	assert.Contains(t, result.Session.Terminals[0].CommandStates, "restore-and-open")

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	require.NotNil(t, persisted.Facility)
	assert.Equal(t, *result.Session.Facility, *persisted.Facility)
	assert.Equal(t, result.Session.Terminals[0].CommandStates, persisted.Terminals[0].CommandStates)

	writesAfterCommit := len(fileSystem.WriteCalls())
	repeated := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID:            "request-replay",
		TerminalID:               "terminal",
		CommandID:                "restore-and-open",
		ExpectedFacilityRevision: 9,
		Transitions: []domain.FacilityTransitionRequest{
			{DeviceID: "power", TransitionID: "restore"},
			{DeviceID: "door", TransitionID: "open"},
		},
	})
	require.True(t, repeated.OK, "repeated ApplyWorldAction() = %#v", repeated)
	assert.False(t, repeated.Changed)
	assert.Equal(t, uint64(1), repeated.SessionRevision)
	assert.Equal(t, uint64(10), repeated.PreviousFacilityRevision)
	assert.Equal(t, uint64(10), repeated.ResultingFacilityRevision)
	assert.Equal(t, writesAfterCommit, len(fileSystem.WriteCalls()))
	require.NotNil(t, repeated.Session)
	assert.Equal(t, result.Session, repeated.Session)
}

func TestApplyWorldActionCommits100ConsecutiveMultiDeviceCandidatesAtomically(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-world-action-100.json")
	initial := facilityConsecutiveWorldActionSession(100)
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	for attempt := range 100 {
		opening := attempt%2 == 0
		transitions := []domain.FacilityTransitionRequest{
			{DeviceID: "power", TransitionID: "restore"},
			{DeviceID: "door", TransitionID: "open"},
		}
		wantPower, wantDoor := "online", "open"
		if !opening {
			transitions = []domain.FacilityTransitionRequest{
				{DeviceID: "power", TransitionID: "cut"},
				{DeviceID: "door", TransitionID: "close"},
			}
			wantPower, wantDoor = "offline", "closed"
		}

		result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
			CorrelationID:            fmt.Sprintf("request-atomic-%03d", attempt),
			TerminalID:               "terminal",
			CommandID:                fmt.Sprintf("facility-cycle-%03d", attempt),
			ExpectedFacilityRevision: uint64(attempt),
			Transitions:              transitions,
		})

		require.True(t, result.OK, "ApplyWorldAction(attempt %d) = %#v", attempt, result)
		require.True(t, result.Changed, "attempt %d did not commit", attempt)
		require.NotNil(t, result.Session)
		require.NotNil(t, result.Session.Facility)
		assert.Equal(t, uint64(attempt), result.PreviousFacilityRevision)
		assert.Equal(t, uint64(attempt+1), result.ResultingFacilityRevision)
		assert.Equal(t, uint64(attempt+1), result.Session.Facility.Revision)
		assert.Equal(t, writesBefore+attempt+1, len(fileSystem.WriteCalls()))
		assert.Equal(t, wantPower, facilityDeviceByID(t, result.Session.Facility, "power").CurrentStateID)
		assert.Equal(t, wantDoor, facilityDeviceByID(t, result.Session.Facility, "door").CurrentStateID)
	}

	snapshot := service.Snapshot()
	require.NotNil(t, snapshot.Session)
	require.NotNil(t, snapshot.Session.Facility)
	assert.Equal(t, uint64(100), snapshot.Session.Facility.Revision)
	assert.Len(t, snapshot.Session.Terminals[0].CommandStates, 100)
	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, *snapshot.Session, persisted)
}

func TestApplyWorldActionRejectsMoreThanOneTransitionPerDeviceWithoutMutation(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-duplicate-device-action.json")
	initial := facilityWorldActionSession()
	initialData := mustEncodeSession(t, initial)
	fileSystem.SeedFile(target, initialData)
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID:            "request-duplicate",
		TerminalID:               "terminal",
		CommandID:                "restore-and-open",
		ExpectedFacilityRevision: 9,
		Transitions: []domain.FacilityTransitionRequest{
			{DeviceID: "power", TransitionID: "restore"},
			{DeviceID: "power", TransitionID: "restore"},
		},
	})

	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailureDuplicate, result.Failure)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(9), result.ResultingFacilityRevision)
	assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
	assert.Equal(t, initialData, fileSystemFileData(t, fileSystem, target))
	assertWorldActionSessionUnchanged(t, service, initial)
}

func TestApplyWorldActionValidatesEveryTransitionAgainstOnePreState(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-precondition-failure.json")
	initial := facilityWorldActionSession()
	initial.Facility.Devices[0].CurrentStateID = "online"
	initialData := mustEncodeSession(t, initial)
	fileSystem.SeedFile(target, initialData)
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID:            "request-precondition",
		TerminalID:               "terminal",
		CommandID:                "restore-and-open",
		ExpectedFacilityRevision: 9,
		Transitions: []domain.FacilityTransitionRequest{
			{DeviceID: "door", TransitionID: "open"},
			{DeviceID: "power", TransitionID: "restore"},
		},
	})

	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailurePreconditionFailed, result.Failure)
	assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
	assert.Equal(t, initialData, fileSystemFileData(t, fileSystem, target))
	assertWorldActionSessionUnchanged(t, service, initial)
}

func TestApplyWorldActionStorageFailureRollsBackWholeCandidate(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "facility-world-action-write-failure.json")
	initial := facilityWorldActionSession()
	initialData := mustEncodeSession(t, initial)
	store := &failingMutationStore{
		path: target,
		data: initialData,
		err:  fmt.Errorf("injected facility replacement failure"),
	}
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)

	result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID:            "request-storage-failure",
		TerminalID:               "terminal",
		CommandID:                "restore-and-open",
		ExpectedFacilityRevision: 9,
		Transitions: []domain.FacilityTransitionRequest{
			{DeviceID: "power", TransitionID: "restore"},
			{DeviceID: "door", TransitionID: "open"},
		},
	})

	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(9), result.ResultingFacilityRevision)
	assert.Equal(t, 1, store.writes)
	assert.Equal(t, initialData, store.data)
	assertWorldActionSessionUnchanged(t, service, initial)
}

func TestApplyWorldActionPrivateRecoveryCommitsOnceAndThenNoOps(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-private-recovery.json")
	initial := facilityRecoverySession()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())
	recovery := domain.CloneDiagnosticRecoveryReference(initial.Facility.Conditions[0].Recovery[2])

	result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID: "private-recovery", ExpectedFacilityRevision: initial.Facility.Revision,
		RecoveryConditionID: "door-offline", Recovery: &recovery,
	})

	require.True(t, result.OK, "ApplyWorldAction(private recovery) = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
	assert.Equal(t, uint64(1), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(10), result.ResultingFacilityRevision)
	assert.Empty(t, result.AffectedDeviceIDs)
	assert.Equal(t, []string{"door-offline"}, result.AffectedConditionIDs)
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))
	require.NotNil(t, result.Session)
	assert.False(t, facilityConditionByID(t, result.Session.Facility, "door-offline").CurrentActive)
	assert.Equal(t, "closed", facilityDeviceByID(t, result.Session.Facility, "door").CurrentStateID)
	assert.NotContains(t, result.Session.Terminals[0].CommandStates, "restore-and-open")

	result.Session.Facility.Conditions[0].CurrentActive = true
	snapshot := service.Snapshot()
	require.NotNil(t, snapshot.Session)
	assert.False(t, facilityConditionByID(t, snapshot.Session.Facility, "door-offline").CurrentActive)

	writesAfterRecovery := len(fileSystem.WriteCalls())
	repeated := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID: "private-recovery-noop", ExpectedFacilityRevision: 10,
		RecoveryConditionID: "door-offline", Recovery: &recovery,
	})
	require.True(t, repeated.OK, "ApplyWorldAction(private recovery no-op) = %#v", repeated)
	assert.False(t, repeated.Changed)
	assert.Equal(t, uint64(1), repeated.SessionRevision)
	assert.Equal(t, uint64(10), repeated.PreviousFacilityRevision)
	assert.Equal(t, uint64(10), repeated.ResultingFacilityRevision)
	assert.Empty(t, repeated.AffectedDeviceIDs)
	assert.Empty(t, repeated.AffectedConditionIDs)
	assert.Equal(t, writesAfterRecovery, len(fileSystem.WriteCalls()))
}

func TestApplyWorldActionRecoveryProgramExpandsAndCommitsConditionEffectsAtomically(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-program-recovery.json")
	initial := facilityRecoverySession()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())
	recovery := domain.CloneDiagnosticRecoveryReference(initial.Facility.Conditions[0].Recovery[1])

	result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID: "program-recovery", ExpectedFacilityRevision: initial.Facility.Revision,
		RecoveryConditionID: "door-offline", Recovery: &recovery,
	})

	require.True(t, result.OK, "ApplyWorldAction(program recovery) = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, uint64(10), result.ResultingFacilityRevision)
	assert.Equal(t, []string{"door"}, result.AffectedDeviceIDs)
	assert.Equal(t, []string{"door-offline"}, result.AffectedConditionIDs)
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))
	require.NotNil(t, result.Session)
	assert.Equal(t, "open", facilityDeviceByID(t, result.Session.Facility, "door").CurrentStateID)
	assert.False(t, facilityConditionByID(t, result.Session.Facility, "door-offline").CurrentActive)
	assert.NotContains(t, result.Session.Terminals[0].CommandStates, "restore-and-open")

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, result.Session, &persisted)
}

func TestApplyWorldActionRecoveryRejectsStaleOrUnauthorisedRequestsWithoutMutation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name            string
		mutateSession   func(*testing.T, *domain.Session)
		mutateCanonical func(*testing.T, *domain.Session)
		mutateRequest   func(*WorldActionRequest)
		recoveryIndex   int
		expectedFailure domain.FacilityFailureCode
	}{
		{
			name: "stale revision",
			mutateRequest: func(request *WorldActionRequest) {
				request.ExpectedFacilityRevision--
			},
			expectedFailure: domain.FacilityFailureStaleRevision,
		},
		{
			name: "recovery is not allowlisted",
			mutateSession: func(_ *testing.T, session *domain.Session) {
				session.Facility.Conditions[0].Recovery = session.Facility.Conditions[0].Recovery[2:]
			},
			expectedFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name:          "program transition is not marked for recovery",
			recoveryIndex: 1,
			mutateCanonical: func(t *testing.T, session *domain.Session) {
				facilityDeviceByID(t, session.Facility, "door").Transitions[0].Recovery = false
			},
			expectedFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name:          "program does not clear selected condition",
			recoveryIndex: 1,
			mutateSession: func(t *testing.T, session *domain.Session) {
				facilityDeviceByID(t, session.Facility, "door").Transitions[0].ConditionEffects = nil
			},
			expectedFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name: "mixed command and recovery authority",
			mutateRequest: func(request *WorldActionRequest) {
				request.TerminalID = "terminal"
				request.CommandID = "restore-and-open"
			},
			expectedFailure: domain.FacilityFailureInvalidConfiguration,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fileSystem := testutil.NewFakeFileSystem()
			target := filepath.Join(testCampaignsRoot, "facility-recovery-rejected-"+test.name+".json")
			initial := facilityRecoverySession()
			recovery := domain.CloneDiagnosticRecoveryReference(
				initial.Facility.Conditions[0].Recovery[test.recoveryIndex],
			)
			if test.mutateSession != nil {
				test.mutateSession(t, &initial)
			}
			initialData := mustEncodeSession(t, initial)
			fileSystem.SeedFile(target, initialData)
			service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
			require.True(t, service.Open(t.Context()).OK)
			canonical := domain.CloneSession(initial)
			if test.mutateCanonical != nil {
				func() {
					service.mu.Lock()
					defer service.mu.Unlock()
					test.mutateCanonical(t, service.active.Session)
					test.mutateCanonical(t, &canonical)
				}()
			}
			writesBefore := len(fileSystem.WriteCalls())
			request := WorldActionRequest{
				CorrelationID: "rejected-recovery", ExpectedFacilityRevision: initial.Facility.Revision,
				RecoveryConditionID: "door-offline", Recovery: &recovery,
			}
			if test.mutateRequest != nil {
				test.mutateRequest(&request)
			}

			result := service.ApplyWorldAction(t.Context(), request)

			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.Equal(t, test.expectedFailure, result.Failure)
			assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
			assert.Equal(t, uint64(9), result.ResultingFacilityRevision)
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
			assert.Equal(t, initialData, fileSystemFileData(t, fileSystem, target))
			assertWorldActionSessionUnchanged(t, service, canonical)
		})
	}
}

func TestApplyWorldActionRecoveryStorageFailureRollsBackWholeCandidate(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "facility-recovery-write-failure.json")
	initial := facilityRecoverySession()
	initialData := mustEncodeSession(t, initial)
	store := &failingMutationStore{
		path: target, data: initialData, err: fmt.Errorf("injected recovery failure"),
	}
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	recovery := domain.CloneDiagnosticRecoveryReference(initial.Facility.Conditions[0].Recovery[1])

	result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID: "program-recovery-failure", ExpectedFacilityRevision: initial.Facility.Revision,
		RecoveryConditionID: "door-offline", Recovery: &recovery,
	})

	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(9), result.ResultingFacilityRevision)
	assert.Equal(t, 1, store.writes)
	assert.Equal(t, initialData, store.data)
	assertWorldActionSessionUnchanged(t, service, initial)
}

func TestResetFacilityDeviceRestoresOnlyDeviceAndDirectConditions(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-device-reset.json")
	initial := facilityResetSession()
	initialData := mustEncodeSession(t, initial)
	fileSystem.SeedFile(target, initialData)
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.ResetFacilityDevice(t.Context(), FacilityDeviceResetRequest{
		DeviceID: "door", ExpectedFacilityRevision: initial.Facility.Revision,
		CorrelationID: "reset-door",
	})

	require.True(t, result.OK, "ResetFacilityDevice() = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
	assert.Equal(t, uint64(1), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(10), result.ResultingFacilityRevision)
	assert.Equal(t, []string{"door"}, result.AffectedDeviceIDs)
	assert.Equal(t, []string{"door-alarm", "door-offline"}, result.AffectedConditionIDs)
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))
	require.NotNil(t, result.Session)
	require.NotNil(t, result.Session.Facility)
	assert.Equal(t, "closed", facilityDeviceByID(t, result.Session.Facility, "door").CurrentStateID)
	assert.True(t, facilityConditionByID(t, result.Session.Facility, "door-offline").CurrentActive)
	assert.False(t, facilityConditionByID(t, result.Session.Facility, "door-alarm").CurrentActive)
	assert.Equal(t, "online", facilityDeviceByID(t, result.Session.Facility, "power").CurrentStateID)
	assert.False(t, facilityConditionByID(t, result.Session.Facility, "power-fault").CurrentActive)
	assert.True(t, facilityConditionByID(t, result.Session.Facility, "terminal-fault").CurrentActive)

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, result.Session, &persisted)
}

func TestResetFacilityRestoresEveryAuthoredInitialValueOnce(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-whole-reset.json")
	initial := facilityResetSession()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.ResetFacility(t.Context(), FacilityResetRequest{
		ExpectedFacilityRevision: initial.Facility.Revision,
		CorrelationID:            "reset-facility",
	})

	require.True(t, result.OK, "ResetFacility() = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, uint64(1), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(10), result.ResultingFacilityRevision)
	assert.Equal(t, []string{"door", "power"}, result.AffectedDeviceIDs)
	assert.Equal(t, []string{"door-alarm", "door-offline", "power-fault", "terminal-fault"}, result.AffectedConditionIDs)
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))
	require.NotNil(t, result.Session)
	for _, device := range result.Session.Facility.Devices {
		assert.Equal(t, device.InitialStateID, device.CurrentStateID, "device %q", device.ID)
	}
	for _, condition := range result.Session.Facility.Conditions {
		assert.Equal(t, condition.InitialActive, condition.CurrentActive, "condition %q", condition.ID)
	}

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, result.Session, &persisted)
}

func TestFacilityResetNoOpDoesNotWriteOrAdvanceRevisions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		reset func(context.Context, *Service, uint64) domain.FacilityOperationResult
	}{
		{
			name: "device",
			reset: func(ctx context.Context, service *Service, revision uint64) domain.FacilityOperationResult {
				return service.ResetFacilityDevice(ctx, FacilityDeviceResetRequest{
					DeviceID: "door", ExpectedFacilityRevision: revision, CorrelationID: "reset-device-no-op",
				})
			},
		},
		{
			name: "whole facility",
			reset: func(ctx context.Context, service *Service, revision uint64) domain.FacilityOperationResult {
				return service.ResetFacility(ctx, FacilityResetRequest{
					ExpectedFacilityRevision: revision, CorrelationID: "reset-facility-no-op",
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fileSystem := testutil.NewFakeFileSystem()
			target := filepath.Join(testCampaignsRoot, "facility-reset-no-op-"+test.name+".json")
			initial := facilityResetSessionAtInitialValues()
			initialData := mustEncodeSession(t, initial)
			fileSystem.SeedFile(target, initialData)
			service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
			require.True(t, service.Open(t.Context()).OK)
			writesBefore := len(fileSystem.WriteCalls())

			result := test.reset(t.Context(), service, initial.Facility.Revision)

			require.True(t, result.OK, "reset() = %#v", result)
			assert.False(t, result.Changed)
			assert.Equal(t, uint64(0), result.SessionRevision)
			assert.Equal(t, initial.Facility.Revision, result.PreviousFacilityRevision)
			assert.Equal(t, initial.Facility.Revision, result.ResultingFacilityRevision)
			assert.Empty(t, result.AffectedDeviceIDs)
			assert.Empty(t, result.AffectedConditionIDs)
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
			assert.Equal(t, initialData, fileSystemFileData(t, fileSystem, target))
			assertWorldActionSessionUnchanged(t, service, initial)
		})
	}
}

func TestFacilityResetRejectsStaleRevisionWithoutMutation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		reset func(context.Context, *Service, uint64) domain.FacilityOperationResult
	}{
		{
			name: "device",
			reset: func(ctx context.Context, service *Service, revision uint64) domain.FacilityOperationResult {
				return service.ResetFacilityDevice(ctx, FacilityDeviceResetRequest{
					DeviceID: "door", ExpectedFacilityRevision: revision, CorrelationID: "reset-device-stale",
				})
			},
		},
		{
			name: "whole facility",
			reset: func(ctx context.Context, service *Service, revision uint64) domain.FacilityOperationResult {
				return service.ResetFacility(ctx, FacilityResetRequest{
					ExpectedFacilityRevision: revision, CorrelationID: "reset-facility-stale",
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fileSystem := testutil.NewFakeFileSystem()
			target := filepath.Join(testCampaignsRoot, "facility-reset-stale-"+test.name+".json")
			initial := facilityResetSession()
			initialData := mustEncodeSession(t, initial)
			fileSystem.SeedFile(target, initialData)
			service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
			require.True(t, service.Open(t.Context()).OK)
			writesBefore := len(fileSystem.WriteCalls())

			result := test.reset(t.Context(), service, initial.Facility.Revision-1)

			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.Equal(t, domain.FacilityFailureStaleRevision, result.Failure)
			assert.Equal(t, initial.Facility.Revision, result.PreviousFacilityRevision)
			assert.Equal(t, initial.Facility.Revision, result.ResultingFacilityRevision)
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
			assert.Equal(t, initialData, fileSystemFileData(t, fileSystem, target))
			assertWorldActionSessionUnchanged(t, service, initial)
		})
	}
}

func TestFacilityResetStorageFailureRollsBackCompleteCandidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		reset func(context.Context, *Service, uint64) domain.FacilityOperationResult
	}{
		{
			name: "device",
			reset: func(ctx context.Context, service *Service, revision uint64) domain.FacilityOperationResult {
				return service.ResetFacilityDevice(ctx, FacilityDeviceResetRequest{
					DeviceID: "door", ExpectedFacilityRevision: revision, CorrelationID: "reset-device-failure",
				})
			},
		},
		{
			name: "whole facility",
			reset: func(ctx context.Context, service *Service, revision uint64) domain.FacilityOperationResult {
				return service.ResetFacility(ctx, FacilityResetRequest{
					ExpectedFacilityRevision: revision, CorrelationID: "reset-facility-failure",
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			target := filepath.Join(testCampaignsRoot, "facility-reset-write-failure-"+test.name+".json")
			initial := facilityResetSession()
			initialData := mustEncodeSession(t, initial)
			store := &failingMutationStore{
				path: target, data: initialData, err: fmt.Errorf("injected facility reset failure"),
			}
			service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
			require.True(t, service.Open(t.Context()).OK)

			result := test.reset(t.Context(), service, initial.Facility.Revision)

			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
			assert.Equal(t, initial.Facility.Revision, result.PreviousFacilityRevision)
			assert.Equal(t, initial.Facility.Revision, result.ResultingFacilityRevision)
			assert.Equal(t, 1, store.writes)
			assert.Equal(t, initialData, store.data)
			assertWorldActionSessionUnchanged(t, service, initial)
		})
	}
}

func TestSaveFacilityAuthoringPreservesCurrentValuesAndInitializesNewEntities(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-authoring-values.json")
	initial := facilityWorldActionSession()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	candidate := domain.CloneSession(initial)
	candidate.Name = "Reauthored facility"
	candidate.Facility.Revision = 999
	facilityDeviceByID(t, candidate.Facility, "power").Name = "Primary power grid"
	facilityDeviceByID(t, candidate.Facility, "power").CurrentStateID = "online"
	facilityDeviceByID(t, candidate.Facility, "door").CurrentStateID = "open"
	facilityConditionByID(t, candidate.Facility, "door-offline").CurrentActive = false
	candidate.Facility.Devices = append(candidate.Facility.Devices, domain.FacilityDevice{
		ID: "ventilation", Name: "Ventilation", Kind: domain.FacilityDeviceKindVentilation,
		InitialStateID: "stopped", CurrentStateID: "running",
		States: []domain.FacilityDeviceState{{ID: "stopped", Name: "Stopped"}, {ID: "running", Name: "Running"}},
	})
	candidate.Facility.Conditions = append(candidate.Facility.Conditions, domain.DiagnosticCondition{
		ID: "ventilation-unpowered", Name: "Ventilation unpowered", Category: domain.DiagnosticConditionCategoryUnpowered,
		Device: &domain.DiagnosticDeviceScope{DeviceID: "ventilation"}, InitialActive: true, CurrentActive: false,
		Effects: []domain.DiagnosticEffect{{
			CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityExecuteCommand},
		}},
		Recovery: []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
	})

	result := service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: candidate, ExpectedSessionRevision: 0, ExpectedFacilityRevision: 9,
		CorrelationID: "authoring-values",
	})

	require.True(t, result.OK, "SaveFacilityAuthoring() = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
	assert.Equal(t, "authoring-values", result.CorrelationID)
	assert.Equal(t, uint64(1), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(10), result.ResultingFacilityRevision)
	assert.Equal(t, []string{"power", "ventilation"}, result.AffectedDeviceIDs)
	assert.Equal(t, []string{"ventilation-unpowered"}, result.AffectedConditionIDs)
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))
	require.NotNil(t, result.Session)
	require.NotNil(t, result.Session.Facility)
	assert.Equal(t, uint64(10), result.Session.Facility.Revision)
	assert.Equal(t, "offline", facilityDeviceByID(t, result.Session.Facility, "power").CurrentStateID)
	assert.Equal(t, "closed", facilityDeviceByID(t, result.Session.Facility, "door").CurrentStateID)
	assert.True(t, facilityConditionByID(t, result.Session.Facility, "door-offline").CurrentActive)
	assert.Equal(t, "stopped", facilityDeviceByID(t, result.Session.Facility, "ventilation").CurrentStateID)
	assert.True(t, facilityConditionByID(t, result.Session.Facility, "ventilation-unpowered").CurrentActive)
	assert.Equal(t, "Primary power grid", facilityDeviceByID(t, result.Session.Facility, "power").Name)

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, result.Session, &persisted)
}

func TestSaveFacilityAuthoringRejectsStaleDocumentOrFacilityRevision(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                     string
		expectedSessionRevision  uint64
		expectedFacilityRevision uint64
	}{
		{name: "document revision", expectedSessionRevision: 1, expectedFacilityRevision: 9},
		{name: "facility revision", expectedSessionRevision: 0, expectedFacilityRevision: 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fileSystem := testutil.NewFakeFileSystem()
			target := filepath.Join(testCampaignsRoot, "facility-authoring-stale-"+test.name+".json")
			initial := facilityWorldActionSession()
			initialData := mustEncodeSession(t, initial)
			fileSystem.SeedFile(target, initialData)
			service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
			require.True(t, service.Open(t.Context()).OK)
			writesBefore := len(fileSystem.WriteCalls())

			candidate := domain.CloneSession(initial)
			candidate.Name = "Stale facility draft"
			result := service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
				Candidate: candidate, ExpectedSessionRevision: test.expectedSessionRevision,
				ExpectedFacilityRevision: test.expectedFacilityRevision, CorrelationID: "authoring-stale",
			})

			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.Equal(t, domain.FacilityFailureStaleRevision, result.Failure)
			assert.Equal(t, uint64(0), result.SessionRevision)
			assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
			assert.Equal(t, uint64(9), result.ResultingFacilityRevision)
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
			assert.Equal(t, initialData, fileSystemFileData(t, fileSystem, target))
			assertWorldActionSessionUnchanged(t, service, initial)
		})
	}
}

func TestSaveFacilityAuthoringAppliesCompleteReferenceRepairAtomically(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-authoring-reference-repair.json")
	initial := facilityWorldActionSession()
	initialData := mustEncodeSession(t, initial)
	fileSystem.SeedFile(target, initialData)
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	partial := domain.CloneSession(initial)
	partial.Facility.Devices = append([]domain.FacilityDevice(nil), partial.Facility.Devices[1:]...)
	rejected := service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: partial, ExpectedSessionRevision: 0, ExpectedFacilityRevision: 9,
		CorrelationID: "authoring-partial-repair",
	})
	assert.False(t, rejected.OK)
	assert.False(t, rejected.Changed)
	assert.Equal(t, domain.FacilityFailureConflict, rejected.Failure)
	assert.NotEmpty(t, rejected.Issues)
	assert.Equal(t, domain.FacilityFailureConflict, rejected.Issues[0].Code)
	assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
	assert.Equal(t, initialData, fileSystemFileData(t, fileSystem, target))
	assertWorldActionSessionUnchanged(t, service, initial)

	repaired := domain.CloneSession(partial)
	command := &repaired.Terminals[0].Root.Children[0]
	command.StateChange.FacilityAction.Transitions.Transitions = []domain.FacilityTransitionRequest{{
		DeviceID: "door", TransitionID: "open",
	}}
	facilityDeviceByID(t, repaired.Facility, "door").Transitions[0].Preconditions = nil
	accepted := service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: repaired, ExpectedSessionRevision: 0, ExpectedFacilityRevision: 9,
		CorrelationID: "authoring-complete-repair",
	})
	require.True(t, accepted.OK, "SaveFacilityAuthoring(repaired) = %#v", accepted)
	assert.True(t, accepted.Changed)
	assert.Equal(t, uint64(1), accepted.SessionRevision)
	assert.Equal(t, uint64(10), accepted.ResultingFacilityRevision)
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))
	require.NotNil(t, accepted.Session)
	assert.Nil(t, findFacilityDeviceByID(accepted.Session.Facility, "power"))
	assert.Equal(t, []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "open"}},
		accepted.Session.Terminals[0].Root.Children[0].StateChange.FacilityAction.Transitions.Transitions)
}

func TestSaveFacilityAuthoringPersistenceFailureRollsBackCompleteCandidate(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "facility-authoring-write-failure.json")
	initial := facilityWorldActionSession()
	initialData := mustEncodeSession(t, initial)
	store := &failingMutationStore{
		path: target, data: initialData, err: fmt.Errorf("injected facility authoring failure"),
	}
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)

	candidate := domain.CloneSession(initial)
	candidate.Facility.Devices[0].Name = "Reauthored power grid"
	result := service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: candidate, ExpectedSessionRevision: 0, ExpectedFacilityRevision: 9,
		CorrelationID: "authoring-write-failure",
	})

	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
	assert.Equal(t, uint64(0), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(9), result.ResultingFacilityRevision)
	assert.Equal(t, 1, store.writes)
	assert.Equal(t, initialData, store.data)
	assertWorldActionSessionUnchanged(t, service, initial)
}

func TestSaveFacilityAuthoringNoOpDoesNotWriteOrAdvanceRevisions(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "facility-authoring-no-op.json")
	initial := facilityWorldActionSession()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: domain.CloneSession(initial), ExpectedSessionRevision: 0, ExpectedFacilityRevision: 9,
		CorrelationID: "authoring-no-op",
	})

	require.True(t, result.OK, "SaveFacilityAuthoring(no-op) = %#v", result)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
	assert.Equal(t, uint64(0), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(9), result.ResultingFacilityRevision)
	assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
	require.NotNil(t, result.Session)
	assert.Equal(t, initial, *result.Session)
	assertWorldActionSessionUnchanged(t, service, initial)
}

func TestSaveFacilityAuthoringDoesNotExposePendingOrdinarySaveAsDurable(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "facility-authoring-pending-save.json")
	initial := facilityWorldActionSession()
	initialData := mustEncodeSession(t, initial)
	store := newBlockingStore()
	store.seed(target, initialData)
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	t.Cleanup(store.release)
	require.True(t, service.Open(t.Context()).OK)

	pending := domain.CloneSession(initial)
	pending.Name = "Pending ordinary save"
	saveDone := make(chan SaveResult, 1)
	go func() {
		saveDone <- service.Save(t.Context(), pending, 1)
	}()
	select {
	case <-store.firstWriteStarted:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "ordinary save did not begin writing")
	}

	authoring := domain.CloneSession(pending)
	authoring.Facility.Devices[0].Name = "Facility draft during save"
	result := service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: authoring, ExpectedSessionRevision: 0, ExpectedFacilityRevision: 9,
		CorrelationID: "authoring-during-save",
	})

	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailureStaleRevision, result.Failure)
	assert.Equal(t, uint64(0), result.SessionRevision)
	assert.Nil(t, result.Session, "an uncommitted active draft must not be returned as durable canonical state")
	assert.Equal(t, initialData, store.file(target))

	store.release()
	select {
	case saved := <-saveDone:
		require.True(t, saved.OK, "Save() = %#v", saved)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "ordinary save did not finish")
	}
}

func facilityRecoverySession() domain.Session {
	session := facilityWorldActionSession()
	door := &session.Facility.Devices[1]
	door.Transitions[0].Recovery = true
	programID := "open-door"
	session.Facility.RecoveryPrograms = []domain.RecoveryProgram{{
		ID: programID, Name: "Open door",
		Transitions: []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "open"}},
	}}
	condition := &session.Facility.Conditions[0]
	condition.Recovery = []domain.DiagnosticRecoveryReference{
		{Transition: &domain.FacilityTransitionRequest{DeviceID: "door", TransitionID: "open"}},
		{RecoveryProgramID: &programID},
		{PrivateOverseerAction: new(true)},
	}
	return session
}

func facilityResetSession() domain.Session {
	session := facilityWorldActionSession()
	session.Facility.Devices[0].CurrentStateID = "online"
	session.Facility.Devices[1].CurrentStateID = "open"
	session.Facility.Conditions[0].CurrentActive = false
	session.Facility.Conditions = append(session.Facility.Conditions,
		domain.DiagnosticCondition{
			ID: "door-alarm", Name: "Door alarm", Category: domain.DiagnosticConditionCategoryCustom,
			CustomCategory: "door-alarm", Device: &domain.DiagnosticDeviceScope{DeviceID: "door"},
			InitialActive: false, CurrentActive: true,
			Effects: []domain.DiagnosticEffect{{DisplayInstability: &domain.DisplayInstabilityEffect{}}},
		},
		domain.DiagnosticCondition{
			ID: "power-fault", Name: "Power fault", Category: domain.DiagnosticConditionCategoryCustom,
			CustomCategory: "power-fault", Device: &domain.DiagnosticDeviceScope{DeviceID: "power"},
			InitialActive: true, CurrentActive: false,
			Effects: []domain.DiagnosticEffect{{DisplayInstability: &domain.DisplayInstabilityEffect{}}},
		},
		domain.DiagnosticCondition{
			ID: "terminal-fault", Name: "Terminal fault", Category: domain.DiagnosticConditionCategoryDisplayUnstable,
			Terminal:      &domain.DiagnosticTerminalScope{TerminalID: "terminal"},
			InitialActive: false, CurrentActive: true,
			Effects: []domain.DiagnosticEffect{{DisplayInstability: &domain.DisplayInstabilityEffect{}}},
		},
	)
	return session
}

func facilityResetSessionAtInitialValues() domain.Session {
	session := facilityResetSession()
	for index := range session.Facility.Devices {
		device := &session.Facility.Devices[index]
		device.CurrentStateID = device.InitialStateID
	}
	for index := range session.Facility.Conditions {
		condition := &session.Facility.Conditions[index]
		condition.CurrentActive = condition.InitialActive
	}
	return session
}

func facilityWorldActionSession() domain.Session {
	return domain.Session{
		Version: 1,
		Name:    "Facility world action",
		Terminals: []domain.Terminal{{
			ID: "terminal", Name: "Facility terminal",
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{{
					ID: "restore-and-open", Type: domain.NodeCommand, Name: "RESTORE POWER AND OPEN DOOR",
					Text: "Power restored and door opened.",
					StateChange: &domain.StateChangeConfig{
						CompletedName:    "POWER RESTORED; DOOR OPEN",
						ConfirmationText: "Authorize facility action?",
						FacilityAction: &domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
							Transitions: []domain.FacilityTransitionRequest{
								{DeviceID: "power", TransitionID: "restore"},
								{DeviceID: "door", TransitionID: "open"},
							},
						}},
					},
				}},
			},
		}},
		TerminalGroups: []domain.TerminalGroup{{
			ID: "facility-group", Name: "Facility", TerminalIDs: []string{"terminal"},
		}},
		Facility: &domain.Facility{
			Revision: 9,
			Devices: []domain.FacilityDevice{
				{
					ID: "power", Name: "Main power", Kind: domain.FacilityDeviceKindPowerGrid,
					InitialStateID: "offline", CurrentStateID: "offline",
					States: []domain.FacilityDeviceState{
						{ID: "offline", Name: "Offline"},
						{ID: "online", Name: "Online"},
					},
					Transitions: []domain.FacilityDeviceTransition{{
						ID: "restore", Name: "Restore", SourceStateID: "offline", DestinationStateID: "online",
					}},
				},
				{
					ID: "door", Name: "Security door", Kind: domain.FacilityDeviceKindDoor,
					InitialStateID: "closed", CurrentStateID: "closed",
					States: []domain.FacilityDeviceState{
						{ID: "closed", Name: "Closed"},
						{ID: "open", Name: "Open"},
					},
					Transitions: []domain.FacilityDeviceTransition{{
						ID: "open", Name: "Open", SourceStateID: "closed", DestinationStateID: "open",
						Preconditions:    []domain.FacilityStateEquality{{DeviceID: "power", StateID: "offline"}},
						ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: "door-offline", Active: false}},
					}},
				},
			},
			Conditions: []domain.DiagnosticCondition{{
				ID: "door-offline", Name: "Door controller offline", Category: domain.DiagnosticConditionCategoryOffline,
				Device:        &domain.DiagnosticDeviceScope{DeviceID: "door"},
				InitialActive: true,
				CurrentActive: true,
				Effects: []domain.DiagnosticEffect{{
					CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityExecuteCommand},
				}},
				Recovery: []domain.DiagnosticRecoveryReference{{
					PrivateOverseerAction: new(true),
				}},
			}},
			RecoveryPrograms: []domain.RecoveryProgram{},
		},
	}
}

func facilityConsecutiveWorldActionSession(actionCount int) domain.Session {
	session := facilityWorldActionSession()
	session.Facility.Revision = 0
	session.Facility.Devices[0].Transitions = append(
		session.Facility.Devices[0].Transitions,
		domain.FacilityDeviceTransition{
			ID: "cut", Name: "Cut", SourceStateID: "online", DestinationStateID: "offline",
		},
	)
	session.Facility.Devices[1].Transitions[0].Preconditions = nil
	session.Facility.Devices[1].Transitions[0].ConditionEffects = nil
	session.Facility.Devices[1].Transitions = append(
		session.Facility.Devices[1].Transitions,
		domain.FacilityDeviceTransition{
			ID: "close", Name: "Close", SourceStateID: "open", DestinationStateID: "closed",
		},
	)
	session.Terminals[0].Root.Children = make([]domain.ContentNode, 0, actionCount)
	for action := range actionCount {
		transitions := []domain.FacilityTransitionRequest{
			{DeviceID: "power", TransitionID: "restore"},
			{DeviceID: "door", TransitionID: "open"},
		}
		if action%2 != 0 {
			transitions = []domain.FacilityTransitionRequest{
				{DeviceID: "power", TransitionID: "cut"},
				{DeviceID: "door", TransitionID: "close"},
			}
		}
		session.Terminals[0].Root.Children = append(session.Terminals[0].Root.Children, domain.ContentNode{
			ID:   fmt.Sprintf("facility-cycle-%03d", action),
			Type: domain.NodeCommand,
			Name: fmt.Sprintf("FACILITY CYCLE %03d", action),
			Text: "Facility cycle complete.",
			StateChange: &domain.StateChangeConfig{
				CompletedName:    fmt.Sprintf("FACILITY CYCLE %03d COMPLETE", action),
				ConfirmationText: "Authorize facility cycle?",
				FacilityAction: &domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
					Transitions: transitions,
				}},
			},
		})
	}
	return session
}

func facilityDeviceByID(t *testing.T, facility *domain.Facility, deviceID string) *domain.FacilityDevice {
	t.Helper()
	device := findFacilityDeviceByID(facility, deviceID)
	if device != nil {
		return device
	}
	t.Fatalf("device %q not found", deviceID)
	return nil
}

func findFacilityDeviceByID(facility *domain.Facility, deviceID string) *domain.FacilityDevice {
	if facility == nil {
		return nil
	}
	for index := range facility.Devices {
		if facility.Devices[index].ID == deviceID {
			return &facility.Devices[index]
		}
	}
	return nil
}

func facilityConditionByID(t *testing.T, facility *domain.Facility, conditionID string) *domain.DiagnosticCondition {
	t.Helper()
	for index := range facility.Conditions {
		if facility.Conditions[index].ID == conditionID {
			return &facility.Conditions[index]
		}
	}
	t.Fatalf("condition %q not found", conditionID)
	return nil
}

func assertWorldActionSessionUnchanged(t *testing.T, service *Service, want domain.Session) {
	t.Helper()
	snapshot := service.Snapshot()
	require.NotNil(t, snapshot.Session)
	assert.Equal(t, want, *snapshot.Session)
	require.NotNil(t, snapshot.Session.Facility)
	require.NotNil(t, want.Facility)
	assert.Equal(t, want.Facility.Revision, snapshot.Session.Facility.Revision)
	assert.Equal(t,
		facilityDeviceByID(t, want.Facility, "power").CurrentStateID,
		facilityDeviceByID(t, snapshot.Session.Facility, "power").CurrentStateID,
	)
	assert.Equal(t,
		facilityDeviceByID(t, want.Facility, "door").CurrentStateID,
		facilityDeviceByID(t, snapshot.Session.Facility, "door").CurrentStateID,
	)
	assert.Equal(t,
		facilityConditionByID(t, want.Facility, "door-offline").CurrentActive,
		facilityConditionByID(t, snapshot.Session.Facility, "door-offline").CurrentActive,
	)
	assert.NotContains(t, snapshot.Session.Terminals[0].CommandStates, "restore-and-open")
}
