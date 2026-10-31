package control

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/require"
)

// The authoring fixture extends the facility command fixture. Terminal-1 is
// the active broadcast terminal and holds the door command with a device
// action; terminal-2 is a suspended broadcast terminal whose players are in
// its LOGS folder. The durable owner is an in-memory session store that
// checks only the authoring revision and keeps live values canonical.
const (
	authoringRelayCommandID   = "command-call-relay" // state change without a device action
	authoringFlagCommandID    = "command-raise-flag" // on terminal-2
	authoringSecondTerminalID = "terminal-2"
	authoringSessionRevision  = uint64(40)
)

func TestSaveAuthoringRefreshesEveryBroadcastTerminal(t *testing.T) {
	t.Parallel()

	fixture := newAuthoringSaveFixture(t)
	before := fixture.service.Snapshot()
	terminalsBefore := canonicalTerminalSlots(t, fixture.service)
	createsBefore, updatesBefore, _ := fixture.lifecycle.Calls()
	effectsBefore := fixture.effects.Calls()
	candidate := fixture.durable.Session()
	authoringNode(t, &candidate, fixture.terminalID, "docs").Name = "ARCHIVE"
	authoringNode(t, &candidate, authoringSecondTerminalID, authoringFlagCommandID).Name = "Hoist flag"
	authoringRenamePower(t, &candidate)

	state, result, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
		Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-refresh",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	require.Equal(t, "authoring-refresh", result.CorrelationID)
	require.Equal(t, uint64(2), result.AuthoringRevision)
	require.Equal(t, authoringSessionRevision+1, result.SessionRevision)
	require.Equal(t, fixture.initialFacility.Revision, result.PreviousFacilityRevision)
	require.Equal(t, fixture.initialFacility.Revision+1, result.ResultingFacilityRevision)
	require.Equal(t, before.Revision+1, state.Revision)
	require.NotNil(t, state.FacilityRevision)
	require.Equal(t, result.ResultingFacilityRevision, *state.FacilityRevision)
	require.Empty(t, cmp.Diff(result.Session.Facility, coordinatorFacilitySnapshot(t, fixture.service)),
		"the durable facility must be installed")

	durable := fixture.durable.Session()
	for _, terminalID := range []string{fixture.terminalID, authoringSecondTerminalID} {
		previous := terminalsBefore[terminalID]
		after := canonicalTerminal(t, fixture.service, terminalID)
		require.Emptyf(t, cmp.Diff(terminalByStableID(&durable, terminalID).Root, after.AuthoredTree),
			"%s was not refreshed from the durable session", terminalID)
		require.Equalf(t, previous.Hack, after.Hack, "%s must keep its hacking progress", terminalID)
		require.Equalf(t, previous.Nav, after.Nav, "%s must keep its navigation", terminalID)
		require.Equalf(t, previous.Lifecycle, after.Lifecycle, "%s changed lifecycle", terminalID)
	}
	createsAfter, updatesAfter, _ := fixture.lifecycle.Calls()
	require.Equal(t, createsBefore, createsAfter, "a save must update runtimes, never recreate them")
	require.Equal(t, updatesBefore+2, updatesAfter, "every broadcast terminal must be refreshed once")

	effects := fixture.effects.Values()[effectsBefore:]
	require.Equal(t, 1, masterEffectCount(effects))
	live := authoringLiveEffects(effects)
	require.Len(t, live, 1)
	require.Equal(t, fixture.terminalID, live[0].TerminalID)
	published := contentNodeByIDForControlTest(live[0].Tree, "docs")
	require.NotNil(t, published)
	require.Equal(t, "ARCHIVE", published.Name, "a saved edit must reach players at once")
	authoringRequireNoAuthoredText(t, auditEvents(effects))

	result.Session.Facility.Devices[0].Name = "tampered"
	require.Equal(t, "Main power", coordinatorFacilitySnapshot(t, fixture.service).Devices[0].Name)
}

func TestSaveAuthoringRebasesOrRejectsPendingDeviceAction(t *testing.T) {
	t.Parallel()

	for _, test := range authoringPendingCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newAuthoringSaveFixture(t)
			pending := fixture.selectDoorCommand(t, "authoring-pending")
			candidate := fixture.durable.Session()
			test.edit(t, &candidate)
			wantRevision := fixture.initialFacility.Revision + test.facilityStep
			effectsBefore := fixture.effects.Calls()

			_, result, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-pending-save",
			})

			require.NoError(t, err)
			require.True(t, result.OK)
			require.True(t, result.Changed)
			require.Equal(t, wantRevision, result.ResultingFacilityRevision)
			terminal := canonicalTerminal(t, fixture.service, fixture.terminalID)
			if test.keepsPending {
				after := pendingFacilityCommandSnapshot(t, fixture.service)
				require.NotNil(t, after, "the pending request must stay approvable")
				require.Equal(t, pending.RequestID, after.RequestID)
				require.NotNil(t, after.FacilityAction)
				require.Equal(t, wantRevision, after.FacilityAction.ExpectedFacilityRevision)
				require.Equal(t, pending.FacilityAction.ActionFingerprint, after.FacilityAction.ActionFingerprint)
				require.Equal(t, pending.FacilityAction.ExpectedSourceStates, after.FacilityAction.ExpectedSourceStates)
				require.NotNil(t, terminal.CommandExecution)
				require.Equal(t, domain.CommandExecutionPhasePending, terminal.CommandExecution.Phase)
			} else {
				require.Nil(t, fixture.service.Snapshot().PendingCommandExecution)
				require.Equal(t, &domain.CommandExecutionPresentation{
					Phase: domain.CommandExecutionPhaseRejected, CommandID: fixture.commandID,
				}, terminal.CommandExecution)
			}

			records := authoringRequestRecords(fixture.effects.Values()[effectsBefore:], pending.RequestID)
			if test.wantOutcome == "" {
				require.Empty(t, records, "a save that leaves the device action alone records nothing for it")
			} else {
				require.Len(t, records, 1, "audit events = %#v", records)
				record := records[0]
				require.Equal(t, "command.request_outcome", record.Name)
				require.Equal(t, test.wantOutcome, record.Outcome)
				require.Equal(t, fixture.terminalID, record.TerminalID)
				require.Equal(t, fixture.commandID, record.CommandID)
				require.NotNil(t, record.Facility, "the outcome record must carry revisions")
				require.Equal(t, pending.RequestID, record.Facility.CorrelationID)
				require.Equal(t, fixture.initialFacility.Revision, record.Facility.PreviousFacilityRevision)
				require.Equal(t, wantRevision, record.Facility.ResultingFacilityRevision)
				authoringRequireNoAuthoredText(t, records)
			}

			retryBefore := fixture.effects.Calls()
			_, retry, retryErr := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: result.AuthoringRevision,
				CorrelationID: "authoring-pending-retry",
			})
			require.NoError(t, retryErr)
			require.True(t, retry.OK)
			require.False(t, retry.Changed)
			require.Empty(t, authoringRequestRecords(fixture.effects.Values()[retryBefore:], pending.RequestID),
				"a retried save must not record the request again")

			_, _, approval, approveErr := fixture.service.ResolveCommandExecution(
				t.Context(), pending.RequestID, domain.CommandExecutionApprove,
			)
			require.NotNil(t, approval)
			if test.keepsPending {
				require.NoError(t, approveErr)
				require.True(t, approval.OK)
				actions := fixture.durable.WorldActions()
				require.Len(t, actions, 1)
				require.Equal(t, wantRevision, actions[0].ExpectedFacilityRevision)
			} else {
				require.ErrorIs(t, approveErr, ErrFacilityStaleRevision)
				require.Equal(t, domain.FacilityFailureStaleRevision, approval.Failure)
				require.Empty(t, fixture.durable.WorldActions(), "a rejected device action must never reach the world")
			}
		})
	}
}

func TestSaveAuthoringKeepsRequestsWithoutDeviceActionPending(t *testing.T) {
	t.Parallel()

	fixture := newAuthoringSaveFixture(t)
	command := fixture.command("authoring-relay-request")
	command.NodeID = authoringRelayCommandID
	require.True(t, fixture.service.DispatchPlayerAction(fixture.controllerConnection, command).Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	require.Equal(t, authoringRelayCommandID, pending.CommandID)
	candidate := fixture.durable.Session()
	authoringRenamePower(t, &candidate)
	effectsBefore := fixture.effects.Calls()

	_, result, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
		Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-relay-save",
	})

	require.NoError(t, err)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	after := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, after)
	require.Equal(t, pending.RequestID, after.RequestID)
	require.Empty(t, authoringRequestRecords(fixture.effects.Values()[effectsBefore:], pending.RequestID))
}

func TestSaveAuthoringFeedsAStaleRefusalWhenAPendingRequestCannotRebase(t *testing.T) {
	t.Parallel()

	for _, test := range authoringPendingCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newAuthoringSaveFixture(t)
			fixture.selectDoorCommand(t, "authoring-feed-pending")
			candidate := fixture.durable.Session()
			test.edit(t, &candidate)
			effectsBefore := fixture.effects.Calls()

			state, result, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-feed-save",
			})

			require.NoError(t, err)
			require.True(t, result.OK)
			require.True(t, result.Changed)
			var want []domain.FacilityChange
			if test.staleRefusal {
				want = []domain.FacilityChange{{DeviceID: "door", PlayerRefusal: &domain.FacilityChangePlayerRefusal{
					TerminalID: fixture.terminalID, CommandID: fixture.commandID,
					Reason: domain.FacilityChangeRefusalStale,
				}}}
			}
			authoringRequireFeed(t, want, state.FacilityChanges)
			authoringRequireFeed(t, want, fixture.service.Snapshot().FacilityChanges)
			published := authoringLastMasterEffect(t, fixture.effects.Values()[effectsBefore:])
			authoringRequireFeed(t, want, published.FacilityChanges)

			_, retry, retryErr := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: result.AuthoringRevision,
				CorrelationID: "authoring-feed-retry",
			})
			require.NoError(t, retryErr)
			require.True(t, retry.OK)
			require.False(t, retry.Changed)
			authoringRequireFeed(t, want, fixture.service.Snapshot().FacilityChanges)
		})
	}
}

func TestSaveAuthoringDropsTheFeedOfDeletedDevices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		edit        func(*testing.T, *domain.Session)
		stale       bool
		wantDevices []string
	}{
		{
			name:        "a deleted device loses its feed",
			edit:        authoringDeleteLamp,
			wantDevices: []string{"door"},
		},
		{
			name:        "a save that keeps every device keeps every feed",
			edit:        authoringRenamePower,
			wantDevices: []string{"door", "lamp"},
		},
		{
			name:        "a rejected save keeps every feed",
			edit:        authoringDeleteLamp,
			stale:       true,
			wantDevices: []string{"door", "lamp"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newAuthoringSaveFixture(t)
			withLamp := fixture.durable.Session()
			withLamp.Facility.Devices = append(withLamp.Facility.Devices, authoringLampDevice())
			_, added, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: withLamp, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-add-lamp",
			})
			require.NoError(t, err)
			require.True(t, added.Changed)
			seeded := authoringSeedFeed(fixture.service)
			candidate := fixture.durable.Session()
			test.edit(t, &candidate)
			expectedAuthoringRevision := added.AuthoringRevision
			if test.stale {
				expectedAuthoringRevision++
			}
			effectsBefore := fixture.effects.Calls()

			state, result, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: expectedAuthoringRevision,
				CorrelationID: "authoring-feed-prune",
			})

			require.NotNil(t, result)
			if test.stale {
				require.ErrorIs(t, err, ErrFacilityStaleRevision)
				require.False(t, result.OK)
			} else {
				require.NoError(t, err)
				require.True(t, result.OK)
				require.True(t, result.Changed)
			}
			var want []domain.FacilityChange
			for _, change := range seeded {
				if slices.Contains(test.wantDevices, change.DeviceID) {
					want = append(want, change)
				}
			}
			require.Empty(t, cmp.Diff(want, state.FacilityChanges, cmpopts.EquateEmpty()))
			require.Empty(t, cmp.Diff(want, fixture.service.Snapshot().FacilityChanges, cmpopts.EquateEmpty()))
			if !test.stale {
				published := authoringLastMasterEffect(t, fixture.effects.Values()[effectsBefore:])
				require.Empty(t, cmp.Diff(want, published.FacilityChanges, cmpopts.EquateEmpty()),
					"the published snapshot must carry the pruned feed")
			}
			fixture.service.mu.RLock()
			devices := slices.Sorted(maps.Keys(fixture.service.runtime.FacilityChanges))
			sequence := fixture.service.runtime.FacilityChangeSequence
			fixture.service.mu.RUnlock()
			require.Equal(t, test.wantDevices, devices)
			require.Equal(t, seeded[0].Sequence, sequence, "pruning must keep the run's sequence")
		})
	}
}

func TestSaveAuthoringAcceptsSavesInterleavedWithWorldActions(t *testing.T) {
	t.Parallel()

	fixture := newAuthoringSaveFixture(t)
	first := fixture.durable.Session()
	authoringNode(t, &first, fixture.terminalID, "docs").Name = "ARCHIVE"
	_, saved, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
		Candidate: first, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-before-world",
	})
	require.NoError(t, err)
	require.True(t, saved.OK)

	pending := fixture.selectDoorCommand(t, "authoring-world-action")
	_, _, world, worldErr := fixture.service.ResolveCommandExecution(
		t.Context(), pending.RequestID, domain.CommandExecutionApprove,
	)
	require.NoError(t, worldErr)
	require.True(t, world.OK)
	require.Greater(t, world.SessionRevision, saved.SessionRevision)
	require.Greater(t, world.ResultingFacilityRevision, saved.ResultingFacilityRevision)

	// The author's next edit was made on a copy taken before the world
	// action: it still shows the door sealed and the command not executed.
	second := domain.CloneSession(first)
	authoringNode(t, &second, fixture.terminalID, authoringRelayCommandID).Name = "Hail the relay"
	_, result, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
		Candidate: second, ExpectedAuthoringRevision: saved.AuthoringRevision, CorrelationID: "authoring-after-world",
	})

	require.NoError(t, err, "a world action must never reject an authoring save")
	require.True(t, result.OK)
	require.True(t, result.Changed)
	require.Equal(t, saved.AuthoringRevision+1, result.AuthoringRevision)
	require.Equal(t, world.ResultingFacilityRevision, result.ResultingFacilityRevision,
		"a terminal edit must not step the facility revision")
	facility := coordinatorFacilitySnapshot(t, fixture.service)
	require.Equal(t, "open", facilityDeviceState(t, facility, "door"), "the save must keep live device states")
	require.True(t, facilityConditionActive(t, facility, "door-alarm"), "the save must keep live fault activity")
	terminal := canonicalTerminal(t, fixture.service, fixture.terminalID)
	require.Contains(t, terminal.CommandStates, fixture.commandID, "the save must keep executed commands")
	relay := contentNodeByIDForControlTest(terminal.AuthoredTree, authoringRelayCommandID)
	require.NotNil(t, relay)
	require.Equal(t, "Hail the relay", relay.Name)
}

func TestSaveAuthoringFailureLeavesTheRuntimeUnchanged(t *testing.T) {
	t.Parallel()

	invalid := []domain.FacilityIssue{{
		Code: domain.FacilityFailureInvalidConfiguration, EntityKind: "terminal", EntityID: new("terminal-1"),
	}}
	tests := []struct {
		name          string
		request       func(AuthoringSaveRequest) AuthoringSaveRequest
		failure       domain.FacilityFailureCode
		issues        []domain.FacilityIssue
		canceled      bool
		wantFailure   domain.FacilityFailureCode
		wantSaves     int
		wantCanonical bool
	}{
		{
			name: "authored configuration changed meanwhile",
			request: func(request AuthoringSaveRequest) AuthoringSaveRequest {
				request.ExpectedAuthoringRevision = 7
				return request
			},
			wantFailure: domain.FacilityFailureStaleRevision, wantSaves: 1, wantCanonical: true,
		},
		{
			name:    "invalid candidate",
			failure: domain.FacilityFailureInvalidConfiguration, issues: invalid,
			wantFailure: domain.FacilityFailureInvalidConfiguration, wantSaves: 1,
		},
		{
			name:        "write failure",
			failure:     domain.FacilityFailurePersistenceFailed,
			wantFailure: domain.FacilityFailurePersistenceFailed, wantSaves: 1,
		},
		{
			name: "blank correlation ID",
			request: func(request AuthoringSaveRequest) AuthoringSaveRequest {
				request.CorrelationID = " "
				return request
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration,
		},
		{name: "canceled request", canceled: true, wantFailure: domain.FacilityFailureRuntimeContextEnded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newAuthoringSaveFixture(t)
			pending := fixture.selectDoorCommand(t, "authoring-failure-pending")
			fixture.durable.setFailure(test.failure, test.issues)
			candidate := fixture.durable.Session()
			authoringRenamePower(t, &candidate)
			request := AuthoringSaveRequest{Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-failure"}
			if test.request != nil {
				request = test.request(request)
			}
			ctx := t.Context()
			if test.canceled {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			before := fixture.service.Snapshot()
			terminalsBefore := canonicalTerminalSlots(t, fixture.service)
			facilityBefore := coordinatorFacilitySnapshot(t, fixture.service)
			durableBefore := fixture.durable.Session()
			effectsBefore := fixture.effects.Calls()

			state, result, err := fixture.service.SaveAuthoring(ctx, request)

			require.ErrorIs(t, err, facilityFailureError(test.wantFailure))
			require.NotNil(t, state)
			require.NotNil(t, result)
			require.False(t, result.OK)
			require.False(t, result.Changed)
			require.Equal(t, test.wantFailure, result.Failure)
			require.Equal(t, test.issues, result.Issues)
			if test.wantCanonical {
				require.NotNil(t, result.Session, "a stale save must return the canonical session")
				require.Empty(t, cmp.Diff(durableBefore, *result.Session))
				require.Equal(t, uint64(1), result.AuthoringRevision, "a stale save must return the current authoring revision")
			}
			require.Equal(t, test.wantSaves, fixture.durable.SaveCount(), "durable save calls")
			require.Empty(t, cmp.Diff(durableBefore, fixture.durable.Session()), "a failed save wrote the session")
			require.Equal(t, before.Revision, state.Revision)
			require.Empty(t, cmp.Diff(before, fixture.service.Snapshot()), "a failed save changed coordination state")
			require.Equal(t, terminalsBefore, canonicalTerminalSlots(t, fixture.service), "a failed save changed a terminal")
			require.Empty(t, cmp.Diff(facilityBefore, coordinatorFacilitySnapshot(t, fixture.service)),
				"a failed save installed a facility")
			after := pendingFacilityCommandSnapshot(t, fixture.service)
			require.NotNil(t, after, "a failed save must leave the pending request alone")
			require.Equal(t, pending.FacilityAction.ExpectedFacilityRevision, after.FacilityAction.ExpectedFacilityRevision)

			effects := fixture.effects.Values()[effectsBefore:]
			require.Zero(t, masterEffectCount(effects))
			require.Empty(t, authoringLiveEffects(effects))
			require.Empty(t, authoringRequestRecords(effects, pending.RequestID))
			authoringRequireNoAuthoredText(t, auditEvents(effects))
		})
	}
}

func TestSaveAuthoringDecisionAndPublicationIgnoreAFailingLogSink(t *testing.T) {
	t.Parallel()

	type outcome struct {
		state       *domain.MasterCoordinationState
		result      *domain.FacilityOperationResult
		err         error
		snapshot    *domain.MasterCoordinationState
		terminals   map[string]*domain.TerminalRuntime
		publication []Effect
		audits      int
	}
	run := func(t *testing.T, edit func(*testing.T, *domain.Session), failingLog bool) outcome {
		t.Helper()
		fixture := newAuthoringSaveFixture(t)
		fixture.selectDoorCommand(t, "authoring-log-sink")
		candidate := fixture.durable.Session()
		edit(t, &candidate)
		delivered := testutil.NewFakeOrderedEffectSink[Effect]()
		fixture.service.enqueue = func(effect Effect) {
			if failingLog && len(effect.Audit) != 0 {
				return // The log sink fails: no audit record is retained.
			}
			delivered.Enqueue(effect)
		}

		state, result, err := fixture.service.SaveAuthoring(t.Context(), AuthoringSaveRequest{
			Candidate: candidate, ExpectedAuthoringRevision: 1, CorrelationID: "authoring-log-sink-save",
		})

		values := delivered.Values()
		var publication []Effect
		for _, effect := range values {
			if len(effect.Audit) == 0 {
				publication = append(publication, effect)
			}
		}
		return outcome{
			state: state, result: result, err: err, snapshot: fixture.service.Snapshot(),
			terminals: canonicalTerminalSlots(t, fixture.service), publication: publication,
			audits: len(auditEvents(values)),
		}
	}

	for _, test := range authoringPendingCases() {
		if test.wantOutcome == "" {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			working := run(t, test.edit, false)
			failing := run(t, test.edit, true)

			require.NoError(t, working.err)
			require.True(t, working.result.OK)
			require.NotZero(t, working.audits)
			require.Zero(t, failing.audits)
			require.Equal(t, working.err, failing.err)
			require.Empty(t, cmp.Diff(working.result, failing.result), "the save decision depends on logging")
			// The two runs read the clock at different moments.
			feedTime := cmpopts.IgnoreFields(domain.FacilityChange{}, "OccurredAtUnixMs")
			require.Empty(t, cmp.Diff(working.state, failing.state, feedTime))
			require.Empty(t, cmp.Diff(working.snapshot, failing.snapshot, feedTime),
				"the pending decision depends on logging")
			require.Equal(t, working.terminals, failing.terminals)
			require.NotEmpty(t, working.publication)
			require.Empty(t, cmp.Diff(working.publication, failing.publication, feedTime),
				"publication depends on logging")
		})
	}
}

// TestSaveAuthoringRacesWorldActionsAndSceneResets runs one author saving in
// order against a player executing the door command and the Overseer
// resetting the scene. Run it with `task test:race`.
func TestSaveAuthoringRacesWorldActionsAndSceneResets(t *testing.T) {
	t.Parallel()

	const rounds = 25
	fixture := newAuthoringSaveFixture(t)
	ctx := t.Context()
	saves := make([]*domain.FacilityOperationResult, 0, rounds)
	saveErrs := make([]error, 0, rounds)
	var wg sync.WaitGroup
	wg.Go(func() {
		expected := uint64(1)
		for round := range rounds {
			candidate := fixture.durable.Session()
			candidate.Facility.Devices[0].Name = fmt.Sprintf("Power grid %d", round)
			candidate.Terminals[0].Root.Children[0].Name = fmt.Sprintf("DOCS %d", round)
			_, result, err := fixture.service.SaveAuthoring(ctx, AuthoringSaveRequest{
				Candidate: candidate, ExpectedAuthoringRevision: expected,
				CorrelationID: fmt.Sprintf("authoring-race-%d", round),
			})
			saves, saveErrs = append(saves, result), append(saveErrs, err)
			if result != nil && result.OK {
				expected = result.AuthoringRevision
			}
		}
	})
	wg.Go(func() {
		for round := range rounds {
			fixture.service.DispatchPlayerAction(fixture.controllerConnection,
				fixture.command(domain.RequestID(fmt.Sprintf("race-command-%d", round))))
			if pending := fixture.service.Snapshot().PendingCommandExecution; pending != nil {
				_, _, _, _ = fixture.service.ResolveCommandExecution(ctx, pending.RequestID, domain.CommandExecutionApprove)
			}
			fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
				RequestID: domain.RequestID(fmt.Sprintf("race-back-%d", round)), BroadcastID: fixture.broadcastID,
				TerminalID: fixture.terminalID, Kind: domain.RuntimeCommandNavigate, Action: "back",
			})
		}
	})
	wg.Go(func() {
		for round := range rounds {
			// Saves and world actions move the revisions underneath; retry a
			// stale reset a few times so that resets do commit.
			for attempt := range 3 {
				facilityRevision := uint64(0)
				if revision := fixture.service.Snapshot().FacilityRevision; revision != nil {
					facilityRevision = *revision
				}
				_, result, _ := fixture.service.ResetScene(ctx, SceneResetRequest{
					ExpectedSessionRevision: fixture.durable.Revision(), ExpectedFacilityRevision: facilityRevision,
					CorrelationID: fmt.Sprintf("race-reset-%d-%d", round, attempt),
				})
				if result == nil || result.Failure != domain.FacilityFailureStaleRevision {
					break
				}
			}
		}
	})
	wg.Wait()

	require.Len(t, saves, rounds)
	for round, result := range saves {
		require.NoErrorf(t, saveErrs[round], "save %d: live play must never reject an authoring save", round)
		require.NotNilf(t, result, "save %d", round)
		require.Truef(t, result.OK, "save %d = %#v", round, result)
	}
	durable := fixture.durable.Session()
	require.Empty(t, cmp.Diff(durable.Facility, coordinatorFacilitySnapshot(t, fixture.service)),
		"the installed facility diverged from the durable one")
	for _, terminal := range durable.Terminals {
		slot := canonicalTerminal(t, fixture.service, terminal.ID)
		require.Emptyf(t, cmp.Diff(terminal.Root, slot.AuthoredTree), "%s tree diverged", terminal.ID)
		require.Emptyf(t, cmp.Diff(terminal.CommandStates, slot.CommandStates, cmpopts.EquateEmpty()),
			"%s command states diverged", terminal.ID)
	}
}

type authoringPendingCase struct {
	name         string
	edit         func(*testing.T, *domain.Session)
	facilityStep uint64
	keepsPending bool
	wantOutcome  string
	staleRefusal bool // the door's feed gains a STALE player refusal
}

func authoringPendingCases() []authoringPendingCase {
	return []authoringPendingCase{
		{
			name: "facility edit keeps the device action", edit: authoringRenamePower,
			facilityStep: 1, keepsPending: true, wantOutcome: "rebased",
		},
		{
			name: "facility edit changes the device action", edit: authoringChangeDoorAction,
			facilityStep: 1, wantOutcome: "superseded", staleRefusal: true,
		},
		{
			name: "terminal edit leaves the device action alone",
			edit: func(t *testing.T, session *domain.Session) {
				authoringNode(t, session, "terminal-1", "docs").Name = "ARCHIVE"
			},
			keepsPending: true,
		},
	}
}

func authoringRenamePower(t *testing.T, session *domain.Session) {
	t.Helper()
	require.NotNil(t, session.Facility)
	require.Equal(t, "power", session.Facility.Devices[0].ID)
	session.Facility.Devices[0].Name = "Main power"
}

// authoringChangeDoorAction makes the door command use a new transition, so
// its device action and fingerprint change.
func authoringChangeDoorAction(t *testing.T, session *domain.Session) {
	t.Helper()
	require.Equal(t, "door", session.Facility.Devices[1].ID)
	door := &session.Facility.Devices[1]
	door.Transitions = append(door.Transitions, domain.FacilityDeviceTransition{
		ID: "force-open", Name: "Force door", SourceStateID: "sealed", DestinationStateID: "open",
		Preconditions:    []domain.FacilityStateEquality{{DeviceID: "power", StateID: "online"}},
		ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: "door-alarm", Active: true}},
	})
	command := authoringNode(t, session, "terminal-1", "command-open-doors")
	command.StateChange.FacilityAction = &domain.FacilityActionConfig{
		Transitions: &domain.FacilityTransitionList{Transitions: []domain.FacilityTransitionRequest{{
			DeviceID: "door", TransitionID: "force-open",
		}}},
	}
}

func authoringLampDevice() domain.FacilityDevice {
	return domain.FacilityDevice{
		ID: "lamp", Name: "Corridor lamp", Kind: domain.FacilityDeviceKindAlarm,
		InitialStateID: "off", CurrentStateID: "off",
		States: []domain.FacilityDeviceState{{ID: "off", Name: "Lamp off"}, {ID: "on", Name: "Lamp on"}},
	}
}

func authoringDeleteLamp(t *testing.T, session *domain.Session) {
	t.Helper()
	require.NotNil(t, session.Facility)
	devices := slices.DeleteFunc(slices.Clone(session.Facility.Devices), func(device domain.FacilityDevice) bool {
		return device.ID == "lamp"
	})
	require.Len(t, devices, len(session.Facility.Devices)-1, "the lamp must exist before it is deleted")
	session.Facility.Devices = devices
}

// authoringSeedFeed installs a feed for the door and the lamp and returns it
// as the snapshot lists it, newest first.
func authoringSeedFeed(service *Service) []domain.FacilityChange {
	const at = int64(1793390820000)
	seeded := []domain.FacilityChange{
		{Sequence: 3, OccurredAtUnixMs: at, DeviceID: "door", PlayerRefusal: &domain.FacilityChangePlayerRefusal{
			TerminalID: "terminal-1", CommandID: "command-open-doors", Reason: domain.FacilityChangeRefusalDeclined,
		}},
		{Sequence: 2, OccurredAtUnixMs: at, DeviceID: "lamp", Forced: &domain.FacilityChangeForced{
			FromStateID: "off", ToStateID: "on",
		}},
		{Sequence: 1, OccurredAtUnixMs: at, DeviceID: "door", Forced: &domain.FacilityChangeForced{
			FromStateID: "sealed", ToStateID: "open",
		}},
	}
	service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.FacilityChanges = make(map[string][]domain.FacilityChange)
		for _, change := range domain.CloneFacilityChanges(seeded) {
			runtime.FacilityChanges[change.DeviceID] = append(runtime.FacilityChanges[change.DeviceID], change)
		}
		runtime.FacilityChangeSequence = seeded[0].Sequence
		return transition{accepted: true}
	})
	return seeded
}

// authoringRequireFeed compares a projected feed with want. The coordinator
// numbers and stamps each entry, so only their presence is checked.
func authoringRequireFeed(t *testing.T, want, got []domain.FacilityChange) {
	t.Helper()
	require.Empty(t, cmp.Diff(want, got, cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(domain.FacilityChange{}, "Sequence", "OccurredAtUnixMs")))
	for _, change := range got {
		require.NotZero(t, change.Sequence, "every entry must be numbered")
		require.NotZero(t, change.OccurredAtUnixMs, "every entry must carry the commit time")
	}
}

func authoringLastMasterEffect(t *testing.T, effects []Effect) *domain.MasterCoordinationState {
	t.Helper()
	for _, effect := range slices.Backward(effects) {
		if effect.Master != nil {
			return effect.Master
		}
	}
	require.FailNow(t, "the save published no master snapshot")
	return nil
}

type authoringSaveFixture struct {
	facilityCommandFixture
	durable   *authoringSessionStore
	lifecycle *recordingTerminalLifecycle
}

func newAuthoringSaveFixture(t *testing.T) authoringSaveFixture {
	t.Helper()
	base := newFacilityCommandFixture(t, &recordingFacilityStore{})
	session := authoringSaveSession(base.terminalID, base.initialFacility)
	durable := &authoringSessionStore{
		session: domain.CloneSession(session), revision: authoringSessionRevision, authoringRevision: 1,
	}
	lifecycle := &recordingTerminalLifecycle{}
	base.service.terminals = lifecycle
	base.service.facilityStore = durable
	base.service.authoringStore = durable
	base.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(session.Facility)
		for _, terminal := range session.Terminals {
			slot := runtime.Broadcast.TerminalRuntimes[terminal.ID]
			if slot == nil {
				slot = testTerminalRuntime(terminal.ID)
				slot.TerminalName = terminal.Name
				slot.Lifecycle = domain.TerminalLifecycleSuspended
				slot.Nav = domain.NavState{Path: []string{"root", "logs"}, Mode: "list"}
				runtime.Broadcast.TerminalRuntimes[terminal.ID] = slot
			}
			slot.AuthoredTree = domain.CloneContentNode(terminal.Root)
			slot.CommandStates = cloneCommandStates(terminal.CommandStates)
			slot.Tree = domain.CloneContentNode(terminal.Root)
			applyRecordedCommandStates(&slot.Tree, slot.CommandStates)
		}
		return transition{accepted: true}
	})
	return authoringSaveFixture{facilityCommandFixture: base, durable: durable, lifecycle: lifecycle}
}

// selectDoorCommand has the controller request the door command and returns
// the resulting pending request with its device action.
func (fixture authoringSaveFixture) selectDoorCommand(t *testing.T, requestID domain.RequestID) *domain.PendingCommandExecution {
	t.Helper()
	require.True(t, fixture.service.DispatchPlayerAction(fixture.controllerConnection, fixture.command(requestID)).Accepted)
	pending := pendingFacilityCommandSnapshot(t, fixture.service)
	require.NotNil(t, pending)
	require.Equal(t, fixture.commandID, pending.CommandID)
	require.NotNil(t, pending.FacilityAction)
	require.Equal(t, fixture.initialFacility.Revision, pending.FacilityAction.ExpectedFacilityRevision)
	return pending
}

func authoringSaveSession(terminalID string, facility *domain.Facility) domain.Session {
	command := func(id, name, text, completedName, confirmation string) domain.ContentNode {
		return domain.ContentNode{
			ID: id, Type: domain.NodeCommand, Name: name, Text: text,
			StateChange: &domain.StateChangeConfig{CompletedName: completedName, ConfirmationText: confirmation},
		}
	}
	door := command("command-open-doors", "Open doors", "Doors opened", "Doors open", "Open the doors?")
	door.StateChange.FacilityAction = &domain.FacilityActionConfig{
		Transitions: &domain.FacilityTransitionList{Transitions: []domain.FacilityTransitionRequest{{
			DeviceID: "door", TransitionID: "open",
		}}},
	}
	return domain.Session{
		Version: 1, Name: "Authoring save fixture", Facility: domain.CloneFacility(facility),
		Terminals: []domain.Terminal{
			{
				ID: terminalID, Name: "Overseer", HackLevel: 1,
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
					{ID: "docs", Type: domain.NodeFolder, Name: "DOCS", Children: []domain.ContentNode{}},
					door,
					command(authoringRelayCommandID, "Call the relay", "Relay answered.", "Relay called", "Call the relay?"),
				}},
			},
			{
				ID: authoringSecondTerminalID, Name: "Relay", HackLevel: 1,
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
					{ID: "logs", Type: domain.NodeFolder, Name: "LOGS", Children: []domain.ContentNode{{
						ID: "log-entry", Type: domain.NodeEntry, Name: "Shift log", Description: "All quiet.",
					}}},
					command(authoringFlagCommandID, "Raise flag", "Flag is up.", "Flag raised", "Raise the flag?"),
				}},
			},
		},
	}
}

func authoringNode(t *testing.T, session *domain.Session, terminalID, nodeID string) *domain.ContentNode {
	t.Helper()
	terminal := terminalByStableID(session, terminalID)
	require.NotNilf(t, terminal, "no terminal %q", terminalID)
	node, ok := findContentNode(&terminal.Root, nodeID)
	require.Truef(t, ok, "no node %q on %q", nodeID, terminalID)
	return node
}

func authoringLiveEffects(effects []Effect) []*domain.PublicLiveState {
	var live []*domain.PublicLiveState
	for _, effect := range effects {
		if effect.Live != nil {
			live = append(live, effect.Live)
		}
	}
	return live
}

func authoringRequestRecords(effects []Effect, requestID string) []AuditEvent {
	var records []AuditEvent
	for _, event := range auditEvents(effects) {
		if event.RequestID == requestID {
			records = append(records, event)
		}
	}
	return records
}

func authoringRequireNoAuthoredText(t *testing.T, events []AuditEvent) {
	t.Helper()
	encoded, err := json.Marshal(events)
	require.NoError(t, err)
	for _, authored := range []string{
		"Open door", "Doors open", "Open the doors?", "Call the relay", "Relay called", "Relay answered.",
		"Hail the relay", "Raise flag", "Hoist flag", "Flag raised", "Flag is up.", "Shift log", "All quiet.",
		"DOCS", "ARCHIVE", "LOGS", "Power grid", "Main power", "Vault door", "Door alarm", "Force door",
	} {
		require.NotContains(t, string(encoded), authored)
	}
}

// authoringSessionStore is an in-memory durable session owner. Authoring
// saves are checked against the authoring revision only and always keep the
// canonical live values; world actions and scene resets change live values
// and never the authoring revision.
type authoringSessionStore struct {
	mu                sync.Mutex
	session           domain.Session
	revision          uint64
	authoringRevision uint64
	failure           domain.FacilityFailureCode
	issues            []domain.FacilityIssue
	saves             []AuthoringSaveRequest
	worldActions      []FacilityMutationRequest
}

func (store *authoringSessionStore) SaveAuthoring(
	_ context.Context,
	request AuthoringSaveRequest,
) domain.FacilityOperationResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	request.Candidate = domain.CloneSession(request.Candidate)
	store.saves = append(store.saves, request)
	result := store.resultLocked(request.CorrelationID)
	result.AuthoringRevision = store.authoringRevision
	switch {
	case store.failure != "":
		result.Failure = store.failure
		result.Issues = cloneFacilityIssues(store.issues)
		return result
	case request.ExpectedAuthoringRevision != store.authoringRevision:
		result.Failure = domain.FacilityFailureStaleRevision
		result.Session = new(domain.CloneSession(store.session))
		return result
	}

	result.OK = true
	result.Failure = domain.FacilityFailureUnspecified
	merged := authoringMergeLiveValues(store.session, request.Candidate)
	if cmp.Equal(merged, store.session, cmpopts.EquateEmpty()) {
		result.Session = new(domain.CloneSession(store.session))
		return result
	}
	if merged.Facility != nil && authoringFacilityEdited(store.session.Facility, merged.Facility) {
		merged.Facility.Revision++
	}
	store.session = merged
	store.revision++
	store.authoringRevision++
	result.Changed = true
	result.SessionRevision = store.revision
	result.AuthoringRevision = store.authoringRevision
	result.ResultingFacilityRevision = facilityRevision(merged.Facility)
	result.Session = new(domain.CloneSession(merged))
	return result
}

func (store *authoringSessionStore) ApplyWorldAction(
	_ context.Context,
	request FacilityMutationRequest,
) domain.FacilityOperationResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.worldActions = append(store.worldActions, cloneFacilityMutationRequest(request))
	result := store.resultLocked(request.CorrelationID)
	if store.session.Facility == nil || request.ExpectedFacilityRevision != store.session.Facility.Revision {
		result.Failure = domain.FacilityFailureStaleRevision
		return result
	}

	candidate := domain.CloneSession(store.session)
	var devices, conditions []string
	for _, transitionRequest := range request.Transitions {
		device := authoringStoreDevice(candidate.Facility, transitionRequest.DeviceID)
		if device == nil {
			result.Failure = domain.FacilityFailureMissingReference
			return result
		}
		transition := facilityTransition(device, transitionRequest.TransitionID)
		if transition == nil || device.CurrentStateID != transition.SourceStateID {
			result.Failure = domain.FacilityFailureInvalidTransition
			return result
		}
		for _, precondition := range transition.Preconditions {
			if required := authoringStoreDevice(candidate.Facility, precondition.DeviceID); required == nil ||
				required.CurrentStateID != precondition.StateID {
				result.Failure = domain.FacilityFailurePreconditionFailed
				return result
			}
		}
		device.CurrentStateID = transition.DestinationStateID
		devices = append(devices, device.ID)
		for _, effect := range transition.ConditionEffects {
			if condition := facilityCondition(candidate.Facility, effect.ConditionID); condition != nil {
				condition.CurrentActive = effect.Active
				conditions = append(conditions, condition.ID)
			}
		}
	}
	terminal := terminalByStableID(&candidate, request.TerminalID)
	if terminal == nil {
		result.Failure = domain.FacilityFailureMissingReference
		return result
	}
	command, ok := findContentNode(&terminal.Root, request.CommandID)
	if !ok || command.StateChange == nil {
		result.Failure = domain.FacilityFailureMissingReference
		return result
	}
	state := domain.CommandExecutionState{CompletedName: command.StateChange.CompletedName, ResultText: command.Text}
	if change := command.StateChange.EntryContentChange; change != nil {
		state.EntryContentChange = &domain.EntryContentChange{BlockID: change.BlockID, CompletedText: change.CompletedText}
	}
	if terminal.CommandStates == nil {
		terminal.CommandStates = make(map[string]domain.CommandExecutionState)
	}
	terminal.CommandStates[request.CommandID] = state
	candidate.Facility.Revision++
	store.session = candidate
	store.revision++

	result.OK, result.Changed = true, true
	result.SessionRevision = store.revision
	result.ResultingFacilityRevision = candidate.Facility.Revision
	result.AffectedDeviceIDs = sortedUniqueStrings(devices)
	result.AffectedConditionIDs = sortedUniqueStrings(conditions)
	result.Session = new(domain.CloneSession(candidate))
	return result
}

func (store *authoringSessionStore) ResetScene(
	_ context.Context,
	request SceneResetRequest,
) domain.FacilityOperationResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := store.resultLocked(request.CorrelationID)
	result.AuthoringRevision = store.authoringRevision
	if store.session.Facility == nil || request.ExpectedSessionRevision != store.revision ||
		request.ExpectedFacilityRevision != store.session.Facility.Revision {
		result.Failure = domain.FacilityFailureStaleRevision
		return result
	}

	candidate := domain.CloneSession(store.session)
	var devices, conditions []string
	for index := range candidate.Facility.Devices {
		device := &candidate.Facility.Devices[index]
		if device.CurrentStateID != device.InitialStateID {
			device.CurrentStateID = device.InitialStateID
			devices = append(devices, device.ID)
		}
	}
	for index := range candidate.Facility.Conditions {
		condition := &candidate.Facility.Conditions[index]
		if condition.CurrentActive != condition.InitialActive {
			condition.CurrentActive = condition.InitialActive
			conditions = append(conditions, condition.ID)
		}
	}
	var commands []domain.CommandReference
	for index := range candidate.Terminals {
		terminal := &candidate.Terminals[index]
		for _, commandID := range slices.Sorted(maps.Keys(terminal.CommandStates)) {
			commands = append(commands, domain.CommandReference{TerminalID: terminal.ID, CommandID: commandID})
		}
		terminal.CommandStates = nil
	}

	result.OK = true
	result.Failure = domain.FacilityFailureUnspecified
	if len(devices) == 0 && len(conditions) == 0 && len(commands) == 0 {
		result.Session = new(domain.CloneSession(store.session))
		return result
	}
	if len(devices) != 0 || len(conditions) != 0 {
		candidate.Facility.Revision++
	}
	store.session = candidate
	store.revision++
	result.Changed = true
	result.SessionRevision = store.revision
	result.ResultingFacilityRevision = candidate.Facility.Revision
	result.AffectedDeviceIDs = sortedUniqueStrings(devices)
	result.AffectedConditionIDs = sortedUniqueStrings(conditions)
	result.ResetCommands = commands
	result.Session = new(domain.CloneSession(candidate))
	return result
}

func (store *authoringSessionStore) resultLocked(correlationID string) domain.FacilityOperationResult {
	revision := facilityRevision(store.session.Facility)
	return domain.FacilityOperationResult{
		CorrelationID: correlationID, SessionRevision: store.revision,
		PreviousFacilityRevision: revision, ResultingFacilityRevision: revision,
	}
}

func (store *authoringSessionStore) setFailure(failure domain.FacilityFailureCode, issues []domain.FacilityIssue) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failure = failure
	store.issues = cloneFacilityIssues(issues)
}

func (store *authoringSessionStore) Session() domain.Session {
	store.mu.Lock()
	defer store.mu.Unlock()
	return domain.CloneSession(store.session)
}

func (store *authoringSessionStore) Revision() uint64 {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.revision
}

func (store *authoringSessionStore) SaveCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.saves)
}

func (store *authoringSessionStore) WorldActions() []FacilityMutationRequest {
	store.mu.Lock()
	defer store.mu.Unlock()
	actions := make([]FacilityMutationRequest, len(store.worldActions))
	for index, action := range store.worldActions {
		actions[index] = cloneFacilityMutationRequest(action)
	}
	return actions
}

// authoringMergeLiveValues applies a candidate on top of the canonical live
// values: command states, device states, fault activity, terminal groups, and
// the facility revision always come from the canonical document.
func authoringMergeLiveValues(canonical, candidate domain.Session) domain.Session {
	merged := domain.CloneSession(candidate)
	merged.TerminalGroups = domain.CloneTerminalGroups(canonical.TerminalGroups)
	for index := range merged.Terminals {
		terminal := &merged.Terminals[index]
		terminal.CommandStates = nil
		if current := terminalByStableID(&canonical, terminal.ID); current != nil {
			terminal.CommandStates = cloneCommandStates(current.CommandStates)
		}
	}
	if merged.Facility == nil {
		return merged
	}
	merged.Facility.Revision = facilityRevision(canonical.Facility)
	for index := range merged.Facility.Devices {
		device := &merged.Facility.Devices[index]
		device.CurrentStateID = device.InitialStateID
		if current := authoringStoreDevice(canonical.Facility, device.ID); current != nil {
			device.CurrentStateID = current.CurrentStateID
		}
	}
	for index := range merged.Facility.Conditions {
		condition := &merged.Facility.Conditions[index]
		condition.CurrentActive = condition.InitialActive
		if current := facilityCondition(canonical.Facility, condition.ID); current != nil {
			condition.CurrentActive = current.CurrentActive
		}
	}
	return merged
}

func authoringFacilityEdited(before, after *domain.Facility) bool {
	if before == nil || after == nil {
		return before != after
	}
	return !cmp.Equal(before, after, cmpopts.EquateEmpty(), cmpopts.IgnoreFields(domain.Facility{}, "Revision"))
}

func authoringStoreDevice(facility *domain.Facility, deviceID string) *domain.FacilityDevice {
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
