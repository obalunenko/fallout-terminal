package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/live"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAuthoringAcceptsEditAfterLiveChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		live func(t *testing.T, service *Service)
	}{
		{name: "legacy command execution", live: executeAuthoringLegacyCommand},
		{name: "approved facility command", live: approveAuthoringFacilityCommand},
		{
			name: "private fault recovery",
			live: func(t *testing.T, service *Service) {
				t.Helper()
				result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
					CorrelationID:            "authoring-live-recovery",
					ExpectedFacilityRevision: facilityRevision(service.Snapshot().Session),
					RecoveryConditionID:      "door-offline",
					Recovery:                 &domain.DiagnosticRecoveryReference{PrivateOverseerAction: new(true)},
				})
				require.True(t, result.OK, "ApplyWorldAction(recovery) = %#v", result)
				require.True(t, result.Changed)
			},
		},
		{
			name: "device reset after approval",
			live: func(t *testing.T, service *Service) {
				t.Helper()
				approveAuthoringFacilityCommand(t, service)
				result := service.ResetFacilityDevice(t.Context(), FacilityDeviceResetRequest{
					DeviceID:                 "door",
					ExpectedFacilityRevision: facilityRevision(service.Snapshot().Session),
					CorrelationID:            "authoring-live-device-reset",
				})
				require.True(t, result.OK, "ResetFacilityDevice() = %#v", result)
				require.True(t, result.Changed)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			initial := authoringTestSession()
			service, fileSystem, target := openAuthoringTestService(t, initial)
			// The Overseer edits the copy it had before the live change landed.
			candidate := domain.CloneSession(initial)
			test.live(t, service)
			liveState := service.Snapshot()
			require.NotNil(t, liveState.Session)
			require.Equal(t, uint64(1), liveState.AuthoringRevision, "live changes must not advance the authoring revision")
			writesBefore := len(fileSystem.WriteCalls())

			candidate.Terminals[0].Name = "Reauthored facility terminal"
			findNodeByID(&candidate.Terminals[0].Root, "archive-status").Name = "ARCHIVE LEDGER"
			result := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-after-live",
			})

			require.True(t, result.OK, "SaveAuthoring() = %#v", result)
			assert.True(t, result.Changed)
			assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
			assert.Empty(t, result.Issues)
			assert.Equal(t, "authoring-after-live", result.CorrelationID)
			assert.Equal(t, uint64(2), result.AuthoringRevision)
			assert.Equal(t, liveState.SavedRevision+1, result.SessionRevision)
			liveFacilityRevision := facilityRevision(liveState.Session)
			assert.Equal(t, liveFacilityRevision, result.PreviousFacilityRevision)
			assert.Equal(t, liveFacilityRevision, result.ResultingFacilityRevision,
				"a terminal-only edit must not step the facility revision")
			assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()), "an authoring save is one durable write")

			want := domain.CloneSession(*liveState.Session)
			want.Terminals[0].Name = "Reauthored facility terminal"
			findNodeByID(&want.Terminals[0].Root, "archive-status").Name = "ARCHIVE LEDGER"
			require.NotNil(t, result.Session)
			assert.Equal(t, want, *result.Session, "the edit must apply on top of the current live values")
			assertAuthoringCanonical(t, service, want, 2)
			persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
			require.NoError(t, err)
			assert.Equal(t, want, persisted)
		})
	}
}

func TestSaveAuthoringRejectsStaleAuthoringRevisionWithCanonicalSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		authored func(t *testing.T, service *Service)
		expected uint64
		current  uint64
	}{
		{
			name: "earlier authoring save",
			authored: func(t *testing.T, service *Service) {
				t.Helper()
				candidate := domain.CloneSession(*service.Snapshot().Session)
				candidate.Name = "First authored edit"
				result := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
					Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-first",
				})
				require.True(t, result.OK, "SaveAuthoring(first) = %#v", result)
				require.True(t, result.Changed)
			},
			expected: 1,
			current:  2,
		},
		{
			name: "terminal group replacement",
			authored: func(t *testing.T, service *Service) {
				t.Helper()
				active := service.Snapshot()
				groups := domain.CloneTerminalGroups(active.Session.TerminalGroups)
				groups[0].Name = "Facility wing"
				result := service.ReplaceTerminalGroups(t.Context(), groups, active.SavedRevision)
				require.True(t, result.OK, "ReplaceTerminalGroups() = %#v", result)
				require.True(t, result.Changed)
			},
			expected: 1,
			current:  2,
		},
		{
			name: "player config association",
			authored: func(t *testing.T, service *Service) {
				t.Helper()
				result := service.AssociatePlayerConfig(t.Context(), filepath.Join(testCampaignsRoot, "authoring-players.json"))
				require.True(t, result.OK, "AssociatePlayerConfig() = %#v", result)
			},
			expected: 1,
			current:  2,
		},
		{
			name:     "revision never issued",
			authored: func(*testing.T, *Service) {},
			expected: 2,
			current:  1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			initial := authoringTestSession()
			service, fileSystem, target := openAuthoringTestService(t, initial)
			test.authored(t, service)
			canonical := service.Snapshot()
			require.NotNil(t, canonical.Session)
			require.Equal(t, test.current, canonical.AuthoringRevision)
			canonicalData := fileSystemFileData(t, fileSystem, target)
			writesBefore := len(fileSystem.WriteCalls())

			candidate := domain.CloneSession(initial)
			candidate.Terminals[0].Name = "Edit based on an older authored configuration"
			result := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: test.expected, CorrelationID: "authoring-stale",
			})

			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.Equal(t, domain.FacilityFailureStaleRevision, result.Failure)
			assert.Equal(t, "authoring-stale", result.CorrelationID)
			assert.Equal(t, test.current, result.AuthoringRevision, "a stale result must carry the current authoring revision")
			assert.Equal(t, canonical.SavedRevision, result.SessionRevision)
			assert.Equal(t, facilityRevision(canonical.Session), result.PreviousFacilityRevision)
			assert.Equal(t, facilityRevision(canonical.Session), result.ResultingFacilityRevision)
			require.NotNil(t, result.Session, "a stale result must return the canonical session")
			assert.Equal(t, *canonical.Session, *result.Session)
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
			assert.Equal(t, canonicalData, fileSystemFileData(t, fileSystem, target))
			assertAuthoringCanonical(t, service, *canonical.Session, test.current)
		})
	}
}

func TestSaveAuthoringIgnoresLiveValuesInCandidate(t *testing.T) {
	t.Parallel()

	initial := authoringTestSession()
	service, fileSystem, target := openAuthoringTestService(t, initial)
	approveAuthoringFacilityCommand(t, service)
	executeAuthoringLegacyCommand(t, service)
	liveState := service.Snapshot()
	require.NotNil(t, liveState.Session)
	require.Equal(t, uint64(10), facilityRevision(liveState.Session))

	candidate := domain.CloneSession(*liveState.Session)
	// Live values the Overseer may hold stale or forged copies of.
	candidate.Facility.Revision = 999
	facilityDeviceByID(t, candidate.Facility, "power").CurrentStateID = "offline"
	facilityDeviceByID(t, candidate.Facility, "door").CurrentStateID = "closed"
	facilityConditionByID(t, candidate.Facility, "door-offline").CurrentActive = true
	candidate.Terminals[0].CommandStates = map[string]domain.CommandExecutionState{
		"restore-and-open": {CompletedName: "FORGED COMPLETION", ResultText: "Forged."},
		"archive-status":   {CompletedName: "NOT A COMMAND", ResultText: "Forged."},
	}
	candidate.Terminals[1].CommandStates = map[string]domain.CommandExecutionState{
		"root": {CompletedName: "FORGED ROOT", ResultText: "Forged."},
	}
	candidate.TerminalGroups = []domain.TerminalGroup{{
		ID: "merged", Name: "Merged", TerminalIDs: []string{"terminal", "annex"},
	}}
	// The authored edit: a renamed device plus new entities with non-initial
	// current values, which must start at their initial values.
	facilityDeviceByID(t, candidate.Facility, "power").Name = "Primary power grid"
	candidate.Facility.Devices = append(candidate.Facility.Devices, authoringVentilationDevice("running"))
	candidate.Facility.Conditions = append(candidate.Facility.Conditions, authoringVentilationCondition(false))
	writesBefore := len(fileSystem.WriteCalls())

	result := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
		Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-live-values",
	})

	require.True(t, result.OK, "SaveAuthoring() = %#v", result)
	assert.True(t, result.Changed)
	assert.Equal(t, uint64(2), result.AuthoringRevision)
	assert.Equal(t, liveState.SavedRevision+1, result.SessionRevision)
	assert.Equal(t, uint64(10), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(11), result.ResultingFacilityRevision, "a facility edit steps the canonical facility revision once")
	assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()))

	want := domain.CloneSession(*liveState.Session)
	want.Facility.Revision = 11
	facilityDeviceByID(t, want.Facility, "power").Name = "Primary power grid"
	want.Facility.Devices = append(want.Facility.Devices, authoringVentilationDevice("stopped"))
	want.Facility.Conditions = append(want.Facility.Conditions, authoringVentilationCondition(true))
	require.NotNil(t, result.Session)
	assert.Equal(t, want, *result.Session,
		"command states, device states, fault activity, terminal groups, and the facility revision stay canonical")
	assertAuthoringCanonical(t, service, want, 2)
	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, want, persisted)
}

func TestSaveAuthoringAcceptsFacilityAndBindingEditsWithTerminalEdits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		edit                   func(t *testing.T, session *domain.Session)
		wantFacilityRevision   uint64
		facilityRevisionReason string
	}{
		{
			name: "terminal edit only",
			edit: func(t *testing.T, session *domain.Session) {
				t.Helper()
				session.Terminals[0].Name = "Reauthored facility terminal"
				findNodeByID(&session.Terminals[0].Root, "seal-archive").Text = "Archive sealed by the Overseer."
			},
			wantFacilityRevision:   9,
			facilityRevisionReason: "a terminal edit must not step the facility revision",
		},
		{
			name: "binding edit only",
			edit: func(t *testing.T, session *domain.Session) {
				t.Helper()
				addAuthoringBindings(t, session)
			},
			wantFacilityRevision:   9,
			facilityRevisionReason: "a binding lives in the terminal tree and must not step the facility revision",
		},
		{
			name: "terminal, facility, and binding edits",
			edit: func(t *testing.T, session *domain.Session) {
				t.Helper()
				session.Terminals[0].Name = "Reauthored facility terminal"
				facilityDeviceByID(t, session.Facility, "power").Name = "Primary power grid"
				facilityDeviceByID(t, session.Facility, "door").States[1].Name = "Wide open"
				addAuthoringBindings(t, session)
			},
			wantFacilityRevision:   10,
			facilityRevisionReason: "a facility edit steps the facility revision once",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			initial := authoringTestSession()
			service, fileSystem, target := openAuthoringTestService(t, initial)
			writesBefore := len(fileSystem.WriteCalls())
			candidate := domain.CloneSession(initial)
			test.edit(t, &candidate)

			result := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-combined",
			})

			require.True(t, result.OK, "SaveAuthoring() = %#v", result)
			assert.True(t, result.Changed)
			assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
			assert.Equal(t, uint64(2), result.AuthoringRevision)
			assert.Equal(t, uint64(1), result.SessionRevision)
			assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
			assert.Equal(t, test.wantFacilityRevision, result.ResultingFacilityRevision, test.facilityRevisionReason)
			assert.Equal(t, writesBefore+1, len(fileSystem.WriteCalls()), "every edit in one save is one durable write")

			want := domain.CloneSession(initial)
			test.edit(t, &want)
			want.Facility.Revision = test.wantFacilityRevision
			require.NotNil(t, result.Session)
			assert.Equal(t, want, *result.Session)
			assertAuthoringCanonical(t, service, want, 2)
			persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
			require.NoError(t, err)
			assert.Equal(t, want, persisted)
		})
	}
}

func TestSaveAuthoringNoOpLeavesDocumentAndRevisionsUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		live      func(t *testing.T, service *Service)
		candidate func(initial domain.Session) domain.Session
	}{
		{
			name:      "identical candidate",
			live:      func(*testing.T, *Service) {},
			candidate: domain.CloneSession,
		},
		{
			name: "candidate with stale live values",
			live: func(t *testing.T, service *Service) {
				t.Helper()
				approveAuthoringFacilityCommand(t, service)
				executeAuthoringLegacyCommand(t, service)
			},
			candidate: domain.CloneSession,
		},
		{
			name: "candidate with different terminal groups",
			live: func(*testing.T, *Service) {},
			candidate: func(initial domain.Session) domain.Session {
				candidate := domain.CloneSession(initial)
				candidate.TerminalGroups = []domain.TerminalGroup{{
					ID: "merged", Name: "Merged", TerminalIDs: []string{"terminal", "annex"},
				}}
				return candidate
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			initial := authoringTestSession()
			service, fileSystem, target := openAuthoringTestService(t, initial)
			test.live(t, service)
			canonical := service.Snapshot()
			require.NotNil(t, canonical.Session)
			canonicalData := fileSystemFileData(t, fileSystem, target)
			writesBefore := len(fileSystem.WriteCalls())

			result := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: test.candidate(initial), ExpectedAuthoringRevision: 1, CorrelationID: "authoring-no-op",
			})

			require.True(t, result.OK, "SaveAuthoring(no-op) = %#v", result)
			assert.False(t, result.Changed)
			assert.Equal(t, domain.FacilityFailureUnspecified, result.Failure)
			assert.Equal(t, "authoring-no-op", result.CorrelationID)
			assert.Equal(t, uint64(1), result.AuthoringRevision)
			assert.Equal(t, canonical.SavedRevision, result.SessionRevision)
			assert.Equal(t, facilityRevision(canonical.Session), result.PreviousFacilityRevision)
			assert.Equal(t, facilityRevision(canonical.Session), result.ResultingFacilityRevision)
			require.NotNil(t, result.Session)
			assert.Equal(t, *canonical.Session, *result.Session)
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()), "a no-op save must not write")
			assert.Equal(t, canonicalData, fileSystemFileData(t, fileSystem, target))
			assertAuthoringCanonical(t, service, *canonical.Session, 1)
		})
	}
}

func TestSaveAuthoringRejectsInvalidCandidateWithoutWriting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		live        func(t *testing.T, service *Service)
		edit        func(t *testing.T, candidate *domain.Session)
		wantFailure domain.FacilityFailureCode
	}{
		{
			name: "removed device that is still referenced",
			edit: func(_ *testing.T, candidate *domain.Session) {
				candidate.Facility.Devices = candidate.Facility.Devices[1:]
			},
			wantFailure: domain.FacilityFailureConflict,
		},
		{
			name: "command action naming an unknown device",
			edit: func(t *testing.T, candidate *domain.Session) {
				t.Helper()
				action := findNodeByID(&candidate.Terminals[0].Root, "restore-and-open").StateChange.FacilityAction
				action.Transitions.Transitions = append(action.Transitions.Transitions,
					domain.FacilityTransitionRequest{DeviceID: "pump", TransitionID: "start"})
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name: "removed facility",
			edit: func(_ *testing.T, candidate *domain.Session) {
				candidate.Facility = nil
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name: "duplicate node ID",
			edit: func(_ *testing.T, candidate *domain.Session) {
				candidate.Terminals[0].Root.Children = append(candidate.Terminals[0].Root.Children, domain.ContentNode{
					ID: "seal-archive", Type: domain.NodeEntry, Name: "DUPLICATE",
				})
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name: "new cross-group terminal transition",
			edit: func(_ *testing.T, candidate *domain.Session) {
				candidate.Terminals[0].Root.Children = append(candidate.Terminals[0].Root.Children, domain.ContentNode{
					ID: "go-annex", Type: domain.NodeCommand, Name: "GO TO ANNEX",
					TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "annex"},
				})
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{
			name: "executed command loses its state change",
			live: executeAuthoringLegacyCommand,
			edit: func(t *testing.T, candidate *domain.Session) {
				t.Helper()
				findNodeByID(&candidate.Terminals[0].Root, "seal-archive").StateChange = nil
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			initial := authoringTestSession()
			service, fileSystem, target := openAuthoringTestService(t, initial)
			if test.live != nil {
				test.live(t, service)
			}
			canonical := service.Snapshot()
			require.NotNil(t, canonical.Session)
			canonicalData := fileSystemFileData(t, fileSystem, target)
			writesBefore := len(fileSystem.WriteCalls())
			candidate := domain.CloneSession(initial)
			test.edit(t, &candidate)

			result := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-invalid",
			})

			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.Equal(t, test.wantFailure, result.Failure)
			assert.Equal(t, "authoring-invalid", result.CorrelationID)
			require.NotEmpty(t, result.Issues, "an invalid candidate must explain itself with issues")
			assert.Equal(t, test.wantFailure, result.Issues[0].Code)
			assert.NotEmpty(t, result.Issues[0].EntityKind)
			assert.Equal(t, uint64(1), result.AuthoringRevision)
			assert.Equal(t, canonical.SavedRevision, result.SessionRevision)
			assert.Equal(t, facilityRevision(canonical.Session), result.ResultingFacilityRevision)
			require.NotNil(t, result.Session)
			assert.Equal(t, *canonical.Session, *result.Session)
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()), "an invalid candidate must not be written")
			assert.Equal(t, canonicalData, fileSystemFileData(t, fileSystem, target))
			assertAuthoringCanonical(t, service, *canonical.Session, 1)
		})
	}
}

func TestSaveAuthoringRefusesRequestWithoutWriting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		prepare        func(t *testing.T, service *Service, fileSystem *testutil.FakeFileSystem)
		ctx            func(t *testing.T) context.Context
		correlationID  string
		wantFailure    domain.FacilityFailureCode
		wantEntityKind string
	}{
		{
			name: "bundled demo",
			prepare: func(t *testing.T, service *Service, _ *testutil.FakeFileSystem) {
				t.Helper()
				// Open refuses the bundled demo, so activate it directly to prove
				// the store refuses to write it in place.
				require.True(t, service.activate(testLocations.BundledDemo, authoringTestSession()).OK)
			},
			wantFailure:    domain.FacilityFailureInvalidConfiguration,
			wantEntityKind: "session",
		},
		{
			name:          "blank correlation ID",
			prepare:       openAuthoringFileSystemSession,
			correlationID: " \t",
			wantFailure:   domain.FacilityFailureInvalidConfiguration,
		},
		{
			name:        "no active session",
			prepare:     func(*testing.T, *Service, *testutil.FakeFileSystem) {},
			wantFailure: domain.FacilityFailureRuntimeContextEnded,
		},
		{
			name: "shut down service",
			prepare: func(t *testing.T, service *Service, fileSystem *testutil.FakeFileSystem) {
				t.Helper()
				openAuthoringFileSystemSession(t, service, fileSystem)
				require.NoError(t, service.Shutdown(t.Context()))
			},
			wantFailure: domain.FacilityFailureRuntimeContextEnded,
		},
		{
			name:    "canceled request",
			prepare: openAuthoringFileSystemSession,
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			wantFailure: domain.FacilityFailureRuntimeContextEnded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fileSystem := testutil.NewFakeFileSystem()
			service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: authoringTestPath(t)}, testLocations)
			t.Cleanup(func() { shutdownAuthoringTestService(t, service) })
			test.prepare(t, service, fileSystem)
			before := service.Snapshot()
			writesBefore := len(fileSystem.WriteCalls())
			ctx := t.Context()
			if test.ctx != nil {
				ctx = test.ctx(t)
			}
			correlationID := test.correlationID
			if correlationID == "" {
				correlationID = "authoring-refused"
			}
			candidate := authoringTestSession()
			candidate.Name = "Edit that has nowhere to go"

			result := service.SaveAuthoring(ctx, AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: correlationID,
			})

			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.Equal(t, test.wantFailure, result.Failure)
			if test.wantEntityKind != "" {
				require.NotEmpty(t, result.Issues)
				assert.Equal(t, test.wantEntityKind, result.Issues[0].EntityKind)
			}
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()), "a refused save must not write")
			after := service.Snapshot()
			assert.Equal(t, before.Session, after.Session)
			assert.Equal(t, before.AuthoringRevision, after.AuthoringRevision)
			assert.Equal(t, before.SavedRevision, after.SavedRevision)
		})
	}
}

func TestSaveAuthoringWriteFailureLeavesCanonicalStateUnchanged(t *testing.T) {
	t.Parallel()

	target := authoringTestPath(t)
	initial := authoringTestSession()
	initialData := mustEncodeSession(t, initial)
	store := &failingMutationStore{path: target, data: initialData, err: errors.New("injected authoring save failure")}
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { shutdownAuthoringTestService(t, service) })
	require.True(t, service.Open(t.Context()).OK)

	candidate := domain.CloneSession(initial)
	candidate.Terminals[0].Name = "Reauthored facility terminal"
	facilityDeviceByID(t, candidate.Facility, "power").Name = "Primary power grid"
	result := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
		Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-write-failure",
	})

	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
	assert.Equal(t, uint64(1), result.AuthoringRevision)
	assert.Equal(t, uint64(0), result.SessionRevision)
	assert.Equal(t, uint64(9), result.PreviousFacilityRevision)
	assert.Equal(t, uint64(9), result.ResultingFacilityRevision)
	assert.Equal(t, 1, store.writes)
	assert.Equal(t, initialData, store.data)
	assertAuthoringCanonical(t, service, initial, 1)
	snapshot := service.Snapshot()
	assert.Equal(t, uint64(0), snapshot.SavedRevision)
	assert.Equal(t, uint64(0), snapshot.RequestedRevision, "a failed write must roll the requested revision back")

	// The same edit applies once storage recovers, against the unchanged
	// authoring revision.
	store.err = nil
	retried := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
		Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-write-retry",
	})
	require.True(t, retried.OK, "SaveAuthoring(retry) = %#v", retried)
	assert.True(t, retried.Changed)
	assert.Equal(t, uint64(2), retried.AuthoringRevision)
	assert.Equal(t, uint64(10), retried.ResultingFacilityRevision)
	persisted, err := domain.DecodeSession(store.data)
	require.NoError(t, err)
	require.NotNil(t, retried.Session)
	assert.Equal(t, *retried.Session, persisted)
}

func TestSaveAuthoringRevisionAdvancesOnlyOnAuthoredChange(t *testing.T) {
	t.Parallel()

	initial := authoringTestSession()
	service, _, _ := openAuthoringTestService(t, initial)
	save := func(t *testing.T, expected uint64, edit func(*domain.Session)) AuthoringSaveResult {
		t.Helper()
		candidate := domain.CloneSession(*service.Snapshot().Session)
		edit(&candidate)
		return service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
			Candidate: candidate, ExpectedAuthoringRevision: expected, CorrelationID: "authoring-sequence",
		})
	}
	steps := []struct {
		name string
		run  func(t *testing.T)
		want uint64
	}{
		{
			name: "no-op save",
			run: func(t *testing.T) {
				t.Helper()
				result := save(t, 1, func(*domain.Session) {})
				require.True(t, result.OK, "SaveAuthoring(no-op) = %#v", result)
				require.False(t, result.Changed)
				assert.Equal(t, uint64(1), result.AuthoringRevision)
			},
			want: 1,
		},
		{
			name: "changed save",
			run: func(t *testing.T) {
				t.Helper()
				result := save(t, 1, func(candidate *domain.Session) { candidate.Name = "First edit" })
				require.True(t, result.OK, "SaveAuthoring(first) = %#v", result)
				require.True(t, result.Changed)
				assert.Equal(t, uint64(2), result.AuthoringRevision)
			},
			want: 2,
		},
		{
			name: "legacy command execution",
			run:  func(t *testing.T) { executeAuthoringLegacyCommand(t, service) },
			want: 2,
		},
		{
			name: "approved facility command",
			run:  func(t *testing.T) { approveAuthoringFacilityCommand(t, service) },
			want: 2,
		},
		{
			name: "command reset",
			run: func(t *testing.T) {
				t.Helper()
				result := service.ResetCommandState(t.Context(), "terminal", "seal-archive")
				require.True(t, result.OK, "ResetCommandState() = %#v", result)
				require.True(t, result.Changed)
			},
			want: 2,
		},
		{
			name: "device reset",
			run: func(t *testing.T) {
				t.Helper()
				result := service.ResetFacilityDevice(t.Context(), FacilityDeviceResetRequest{
					DeviceID: "door", ExpectedFacilityRevision: facilityRevision(service.Snapshot().Session),
					CorrelationID: "authoring-sequence-device-reset",
				})
				require.True(t, result.OK, "ResetFacilityDevice() = %#v", result)
				require.True(t, result.Changed)
			},
			want: 2,
		},
		{
			name: "rejected invalid save",
			run: func(t *testing.T) {
				t.Helper()
				result := save(t, 2, func(candidate *domain.Session) { candidate.Facility = nil })
				require.False(t, result.OK)
				assert.Equal(t, uint64(2), result.AuthoringRevision)
			},
			want: 2,
		},
		{
			name: "rejected stale save",
			run: func(t *testing.T) {
				t.Helper()
				result := save(t, 1, func(candidate *domain.Session) { candidate.Name = "Stale edit" })
				require.Equal(t, domain.FacilityFailureStaleRevision, result.Failure)
				assert.Equal(t, uint64(2), result.AuthoringRevision)
			},
			want: 2,
		},
		{
			name: "second changed save",
			run: func(t *testing.T) {
				t.Helper()
				result := save(t, 2, func(candidate *domain.Session) { candidate.Name = "Second edit" })
				require.True(t, result.OK, "SaveAuthoring(second) = %#v", result)
				require.True(t, result.Changed)
				assert.Equal(t, uint64(3), result.AuthoringRevision)
			},
			want: 3,
		},
		{
			name: "reopen",
			run: func(t *testing.T) {
				t.Helper()
				opened := service.Open(t.Context())
				require.True(t, opened.OK, "Open() = %#v", opened)
				assert.Equal(t, uint64(1), opened.AuthoringRevision)
			},
			want: 1,
		},
	}

	// The steps share one session, so they run in order and stop at the
	// first failure.
	for _, step := range steps {
		if !t.Run(step.name, func(t *testing.T) {
			step.run(t)
			assert.Equal(t, step.want, service.Snapshot().AuthoringRevision)
		}) {
			return
		}
	}
}

func TestSaveAuthoringRoundTripsEverySessionFileWithIdenticalProjections(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("..", "..", "sessions", "*.json"))
	require.NoError(t, err)
	sessionFiles := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		if _, sessionErr := domain.DecodeSession(raw); sessionErr != nil {
			_, configErr := domain.DecodePlayerConfig(raw)
			require.NoError(t, configErr, "%s is neither a session (%v) nor a player config", path, sessionErr)
			continue
		}
		sessionFiles++

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			fileSystem := testutil.NewFakeFileSystem()
			target := filepath.Join(testCampaignsRoot, "round-trip", filepath.Base(path))
			fileSystem.SeedFile(target, raw)
			service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
			t.Cleanup(func() { shutdownAuthoringTestService(t, service) })
			opened := service.Open(t.Context())
			require.True(t, opened.OK, "Open(%s) = %#v", path, opened)
			require.NotNil(t, opened.Session)
			original := domain.CloneSession(*opened.Session)
			wantProjections := authoringPlayerProjections(t, original)
			writesBefore := len(fileSystem.WriteCalls())

			noOp := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: domain.CloneSession(original), ExpectedAuthoringRevision: 1, CorrelationID: "round-trip-no-op",
			})
			require.True(t, noOp.OK, "SaveAuthoring(no-op) = %#v", noOp)
			assert.False(t, noOp.Changed, "saving an unedited session file must be a no-op")
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
			assert.Equal(t, raw, fileSystemFileData(t, fileSystem, target))
			require.NotNil(t, noOp.Session)
			assert.Equal(t, wantProjections, authoringPlayerProjections(t, *noOp.Session))

			edited := domain.CloneSession(original)
			edited.Name += " (edited)"
			edit := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: edited, ExpectedAuthoringRevision: 1, CorrelationID: "round-trip-edit",
			})
			require.True(t, edit.OK, "SaveAuthoring(edit) = %#v", edit)
			require.True(t, edit.Changed)
			undo := service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: domain.CloneSession(original), ExpectedAuthoringRevision: edit.AuthoringRevision,
				CorrelationID: "round-trip-undo",
			})
			require.True(t, undo.OK, "SaveAuthoring(undo) = %#v", undo)
			require.True(t, undo.Changed)
			assert.Equal(t, writesBefore+2, len(fileSystem.WriteCalls()))

			written := fileSystemFileData(t, fileSystem, target)
			assert.Equal(t, mustEncodeSession(t, original), written,
				"reverting the edit must write the original document byte for byte")
			reopened, err := domain.DecodeSession(written)
			require.NoError(t, err)
			assert.Equal(t, original, reopened)
			assert.Equal(t, wantProjections, authoringPlayerProjections(t, reopened),
				"players must see exactly what they saw before the save")
		})
	}
	require.NotZero(t, sessionFiles, "sessions/ must contain at least one session file")
}

// authoringTestSession extends the facility world-action fixture with a
// legacy state-change command that rewrites an entry block, and with a second
// terminal in its own group.
func authoringTestSession() domain.Session {
	session := facilityWorldActionSession()
	root := &session.Terminals[0].Root
	root.Children = append(root.Children,
		domain.ContentNode{
			ID: "archive-status", Type: domain.NodeEntry, Name: "ARCHIVE STATUS",
			Blocks: []domain.EntryContentBlock{{ID: "archive-seal", InitialText: "ARCHIVE: OPEN"}},
		},
		domain.ContentNode{
			ID: "seal-archive", Type: domain.NodeCommand, Name: "SEAL ARCHIVE", Text: "Archive sealed.",
			StateChange: &domain.StateChangeConfig{
				CompletedName: "ARCHIVE SEALED", ConfirmationText: "Seal the archive?",
				EntryContentChange: &domain.EntryContentChange{
					BlockID: "archive-seal", CompletedText: "ARCHIVE: SEALED",
				},
			},
		},
	)
	session.Terminals = append(session.Terminals, terminalForGroupTest("annex", "Annex terminal"))
	session.TerminalGroups = append(session.TerminalGroups, domain.TerminalGroup{
		ID: "annex-group", Name: "Annex", TerminalIDs: []string{"annex"},
	})
	return session
}

// addAuthoringBindings adds facility bindings in the terminal tree: a
// visibility rule, a state-based name, and a state-based block text.
func addAuthoringBindings(t *testing.T, session *domain.Session) {
	t.Helper()
	status := findNodeByID(&session.Terminals[0].Root, "archive-status")
	require.NotNil(t, status)
	status.VisibleWhen = &domain.FacilityStateEquality{DeviceID: "power", StateID: "online"}
	status.Blocks[0].FacilityTextVariants = []domain.FacilityTextVariant{{
		When: domain.FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "ARCHIVE: EXPOSED",
	}}
	seal := findNodeByID(&session.Terminals[0].Root, "seal-archive")
	require.NotNil(t, seal)
	seal.FacilityNameVariants = []domain.FacilityTextVariant{{
		When: domain.FacilityStateEquality{DeviceID: "door", StateID: "open"}, Text: "SEAL ARCHIVE NOW",
	}}
}

func authoringVentilationDevice(currentStateID string) domain.FacilityDevice {
	return domain.FacilityDevice{
		ID: "ventilation", Name: "Ventilation", Kind: domain.FacilityDeviceKindVentilation,
		InitialStateID: "stopped", CurrentStateID: currentStateID,
		States: []domain.FacilityDeviceState{{ID: "stopped", Name: "Stopped"}, {ID: "running", Name: "Running"}},
	}
}

func authoringVentilationCondition(currentActive bool) domain.DiagnosticCondition {
	return domain.DiagnosticCondition{
		ID: "ventilation-unpowered", Name: "Ventilation unpowered", Category: domain.DiagnosticConditionCategoryUnpowered,
		Device: &domain.DiagnosticDeviceScope{DeviceID: "ventilation"}, InitialActive: true, CurrentActive: currentActive,
		Effects: []domain.DiagnosticEffect{{
			CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityExecuteCommand},
		}},
		Recovery: []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
	}
}

func authoringTestPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(testCampaignsRoot, "authoring", filepath.FromSlash(t.Name())+".json")
}

// openAuthoringTestService opens initial from a fresh fake file system and
// checks that opening starts the authoring revision at 1.
func openAuthoringTestService(t *testing.T, initial domain.Session) (*Service, *testutil.FakeFileSystem, string) {
	t.Helper()
	fileSystem := testutil.NewFakeFileSystem()
	target := authoringTestPath(t)
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { shutdownAuthoringTestService(t, service) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.Equal(t, uint64(1), opened.AuthoringRevision)
	require.NotNil(t, opened.Session)
	require.Equal(t, initial, *opened.Session)
	return service, fileSystem, target
}

// openAuthoringFileSystemSession seeds the authoring fixture at the dialog's
// path and opens it.
func openAuthoringFileSystemSession(t *testing.T, service *Service, fileSystem *testutil.FakeFileSystem) {
	t.Helper()
	fileSystem.SeedFile(authoringTestPath(t), mustEncodeSession(t, authoringTestSession()))
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
}

func shutdownAuthoringTestService(t *testing.T, service *Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
	defer cancel()
	assert.NoError(t, service.Shutdown(ctx))
}

func approveAuthoringFacilityCommand(t *testing.T, service *Service) {
	t.Helper()
	result := service.ApplyWorldAction(t.Context(), WorldActionRequest{
		CorrelationID:            "authoring-live-approval",
		TerminalID:               "terminal",
		CommandID:                "restore-and-open",
		ExpectedFacilityRevision: facilityRevision(service.Snapshot().Session),
		Transitions: []domain.FacilityTransitionRequest{
			{DeviceID: "power", TransitionID: "restore"},
			{DeviceID: "door", TransitionID: "open"},
		},
	})
	require.True(t, result.OK, "ApplyWorldAction() = %#v", result)
	require.True(t, result.Changed)
}

func executeAuthoringLegacyCommand(t *testing.T, service *Service) {
	t.Helper()
	result := service.ExecuteCommandState(t.Context(), "terminal", "seal-archive")
	require.True(t, result.OK, "ExecuteCommandState() = %#v", result)
	require.True(t, result.Changed)
}

func assertAuthoringCanonical(t *testing.T, service *Service, want domain.Session, wantAuthoringRevision uint64) {
	t.Helper()
	snapshot := service.Snapshot()
	require.NotNil(t, snapshot.Session)
	assert.Equal(t, want, *snapshot.Session)
	assert.Equal(t, wantAuthoringRevision, snapshot.AuthoringRevision)
}

// authoringPlayerProjections returns what players see on every terminal of
// session, keyed by terminal ID. The puzzle board and the presentation key
// derived from it are generated per runtime rather than authored, so they are
// left out.
func authoringPlayerProjections(t *testing.T, session domain.Session) map[string]domain.PublicLiveState {
	t.Helper()
	engine := live.New(nil, nil)
	projections := make(map[string]domain.PublicLiveState, len(session.Terminals))
	for _, terminal := range session.Terminals {
		checkpoint, _ := engine.CreateRuntime(terminalTarget(terminal))
		require.NotNil(t, checkpoint, "terminal %q", terminal.ID)
		projection := engine.ProjectFacility(checkpoint, session.Facility)
		require.NotNil(t, projection, "terminal %q", terminal.ID)
		if projection.Hack != nil {
			projection.Hack = nil
			projection.Presentation.ContextKey = ""
		}
		projections[terminal.ID] = *projection
	}
	return projections
}
