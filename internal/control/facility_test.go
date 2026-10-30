package control

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFacilityAuditCommandLifecycleIsCorrelatedSortedAndCommittedExactlyOnce(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	prepareMultiDeviceFacilityAuditFixture(t, &fixture)
	baseline := fixture.effects.Calls()
	playerRequestID := "facility-audit-atomic-command"

	selected := fixture.service.DispatchPlayerAction(fixture.controllerConnection, fixture.command(playerRequestID))
	require.True(t, selected.Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	requestID := pending.RequestID

	request := requireFacilityAuditEvent(
		t, fixture.effects.Values()[baseline:], "facility.request_received", requestID,
	)
	require.Equal(t, "pending", request.Outcome)
	require.Equal(t, requestID, request.Facility.CorrelationID)
	require.Equal(t, FacilityAuditActionCommand, request.Facility.Action)
	require.Equal(t, fixture.broadcastID, request.Facility.BroadcastID)
	require.Equal(t, fixture.terminalID, request.Facility.TerminalID)
	require.Equal(t, fixture.commandID, request.Facility.CommandID)
	require.Equal(t, []string{"door", "vent"}, request.Facility.DeviceIDs)
	require.Equal(t, []string{"alpha-fault", "door-alarm", "zeta-fault"}, request.Facility.ConditionIDs)
	require.Equal(t, uint64(7), request.Facility.PreviousFacilityRevision)
	require.Equal(t, uint64(7), request.Facility.ResultingFacilityRevision)

	_, _, result, err := fixture.service.ResolveCommandExecution(
		t.Context(), requestID, domain.CommandExecutionApprove,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.Equal(t, 1, fixture.store.CallCount())

	events := fixture.effects.Values()[baseline:]
	decision := requireFacilityAuditEvent(t, events, "facility.decision", requestID)
	require.Equal(t, string(domain.CommandExecutionApprove), decision.Decision)
	require.Equal(t, "succeeded", decision.Outcome)
	requireFacilityAuditCommitFacts(t, decision.Facility)
	committed := requireFacilityAuditEvent(t, events, "facility.transition", requestID)
	require.Equal(t, "committed", committed.Outcome)
	requireFacilityAuditCommitFacts(t, committed.Facility)

	_, _, duplicate, duplicateErr := fixture.service.ResolveCommandExecution(
		t.Context(), requestID, domain.CommandExecutionApprove,
	)
	require.Error(t, duplicateErr)
	require.NotNil(t, duplicate)
	require.Equal(t, domain.FacilityFailureDuplicate, duplicate.Failure)
	require.Equal(t, 1, fixture.store.CallCount(), "duplicate decision repeated durable mutation")
	require.Len(t, matchingFacilityAuditEvents(
		fixture.effects.Values()[baseline:], "facility.transition", requestID,
	), 1, "one committed world action must have exactly one transition record")
}

func TestFacilityAuditFailureCarriesTypedOutcomeWithoutCommitEvidence(t *testing.T) {
	t.Parallel()

	store := &recordingFacilityStore{}
	fixture := newFacilityCommandFixture(t, store)
	store.result = domain.FacilityOperationResult{
		Failure:                   domain.FacilityFailurePersistenceFailed,
		PreviousFacilityRevision:  fixture.initialFacility.Revision,
		ResultingFacilityRevision: fixture.initialFacility.Revision,
		AffectedDeviceIDs:         []string{"door"},
		AffectedConditionIDs:      []string{"door-alarm"},
	}
	baseline := fixture.effects.Calls()
	playerRequestID := "facility-audit-persistence-failure"
	require.True(t, fixture.service.DispatchPlayerAction(
		fixture.controllerConnection, fixture.command(playerRequestID),
	).Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	requestID := pending.RequestID

	_, _, result, err := fixture.service.ResolveCommandExecution(
		t.Context(), requestID, domain.CommandExecutionApprove,
	)
	require.Error(t, err)
	require.NotNil(t, result)
	require.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)

	events := fixture.effects.Values()[baseline:]
	decision := requireFacilityAuditEvent(t, events, "facility.decision", requestID)
	require.Equal(t, "failed", decision.Outcome)
	require.Equal(t, domain.FacilityFailurePersistenceFailed, decision.Facility.Failure)
	failure := requireFacilityAuditEvent(t, events, "facility.failure", requestID)
	require.Equal(t, "persistence-failed", failure.Outcome)
	require.Equal(t, domain.FacilityFailurePersistenceFailed, failure.Facility.Failure)
	require.Equal(t, fixture.initialFacility.Revision, failure.Facility.PreviousFacilityRevision)
	require.Equal(t, fixture.initialFacility.Revision, failure.Facility.ResultingFacilityRevision)
	require.Empty(t, matchingFacilityAuditEvents(events, "facility.transition", requestID))

	encoded, marshalErr := json.Marshal(auditEvents(events))
	require.NoError(t, marshalErr)
	for _, forbidden := range []string{
		"Vault door", "Open door", "Power grid", "Authorize this command?", "Command completed.",
	} {
		require.NotContains(t, string(encoded), forbidden)
	}
}

func TestFacilityAuditRecoveryAndResetRecordSafeStateAndRevisionTransitions(t *testing.T) {
	t.Parallel()

	t.Run("recovery", func(t *testing.T) {
		t.Parallel()
		store := &recordingFacilityStore{}
		fixture := newFacilityCommandFixture(t, store)
		facility := recoveryFacilityState()
		fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
			runtime.Facility = domain.CloneFacility(facility)
			return transition{accepted: true}
		})
		canonical := facilityAuthoringSession(facility)
		canonical.Facility.Revision++
		canonical.Facility.Conditions[0].CurrentActive = false
		store.result = domain.FacilityOperationResult{
			OK: true, Changed: true, SessionRevision: 52,
			PreviousFacilityRevision:  facility.Revision,
			ResultingFacilityRevision: canonical.Facility.Revision,
			AffectedConditionIDs:      []string{"door-alarm"}, Session: &canonical,
		}
		recovery := domain.DiagnosticRecoveryReference{PrivateOverseerAction: new(true)}
		baseline := fixture.effects.Calls()
		correlationID := "facility-audit-recovery"

		_, result, err := fixture.service.RecoverFacilityCondition(t.Context(), FacilityRecoveryRequest{
			ConditionID: "door-alarm", ExpectedFacilityRevision: facility.Revision,
			CorrelationID: correlationID, Recovery: &recovery,
		})
		require.NoError(t, err)
		require.True(t, result.OK)

		event := requireFacilityAuditEvent(t, fixture.effects.Values()[baseline:], "facility.recovery", correlationID)
		require.Equal(t, "succeeded", event.Outcome)
		require.Equal(t, FacilityAuditActionRecover, event.Facility.Action)
		require.Equal(t, []string{"door-alarm"}, event.Facility.ConditionIDs)
		require.Equal(t, facility.Revision, event.Facility.PreviousFacilityRevision)
		require.Equal(t, canonical.Facility.Revision, event.Facility.ResultingFacilityRevision)
		require.Equal(t, []FacilityConditionAuditTransition{{
			ConditionID: "door-alarm", PreviousActive: true, ResultingActive: false,
		}}, event.Facility.ConditionTransitions)
	})

	t.Run("whole facility reset", func(t *testing.T) {
		t.Parallel()
		store := &recordingFacilityStore{}
		fixture := newFacilityCommandFixture(t, store)
		current := facilityApprovalState(true)
		fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
			runtime.Facility = domain.CloneFacility(current)
			return transition{accepted: true}
		})
		canonical := facilityAuthoringSession(current)
		canonical.Facility.Revision++
		canonical.Facility.Devices[1].CurrentStateID = "sealed"
		canonical.Facility.Conditions[0].CurrentActive = false
		store.result = domain.FacilityOperationResult{
			OK: true, Changed: true, SessionRevision: 53,
			PreviousFacilityRevision:  current.Revision,
			ResultingFacilityRevision: canonical.Facility.Revision,
			AffectedDeviceIDs:         []string{"door"},
			AffectedConditionIDs:      []string{"door-alarm"}, Session: &canonical,
		}
		baseline := fixture.effects.Calls()
		correlationID := "facility-audit-reset"

		_, result, err := fixture.service.ResetFacility(t.Context(), FacilityResetRequest{
			ExpectedFacilityRevision: current.Revision, CorrelationID: correlationID,
		})
		require.NoError(t, err)
		require.True(t, result.OK)

		event := requireFacilityAuditEvent(t, fixture.effects.Values()[baseline:], "facility.reset", correlationID)
		require.Equal(t, "succeeded", event.Outcome)
		require.Equal(t, FacilityAuditActionReset, event.Facility.Action)
		require.Equal(t, "facility", event.Facility.ResetScope)
		require.Equal(t, current.Revision, event.Facility.PreviousFacilityRevision)
		require.Equal(t, canonical.Facility.Revision, event.Facility.ResultingFacilityRevision)
		require.Equal(t, []FacilityDeviceAuditTransition{{
			DeviceID: "door", PreviousStateID: "open", ResultingStateID: "sealed",
		}}, event.Facility.DeviceTransitions)
		require.Equal(t, []FacilityConditionAuditTransition{{
			ConditionID: "door-alarm", PreviousActive: true, ResultingActive: false,
		}}, event.Facility.ConditionTransitions)
	})
}

func TestFacilityCommandSelectionSnapshotsDetachedIntentWithoutMutation(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	before := domain.CloneFacility(fixture.initialFacility)

	selected := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("facility-pending"),
	)
	require.True(t, selected.Accepted)
	require.Zero(t, fixture.store.CallCount())
	require.Empty(t, cmp.Diff(before, coordinatorFacilitySnapshot(t, fixture.service)))

	pending := pendingFacilityCommandSnapshot(t, fixture.service)
	require.NotNil(t, pending)
	require.NotNil(t, pending.FacilityAction)
	require.Equal(t, before.Revision, pending.FacilityAction.ExpectedFacilityRevision)
	require.NotEmpty(t, pending.FacilityAction.ActionFingerprint)
	require.Equal(t, []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "open"}}, pending.FacilityAction.TransitionRequests)
	require.Equal(t, []domain.FacilityStateEquality{{DeviceID: "door", StateID: "sealed"}}, pending.FacilityAction.ExpectedSourceStates)
	require.Equal(t, []string{"door-alarm"}, pending.FacilityAction.AffectedConditionIDs)

	pending.FacilityAction.TransitionRequests[0].TransitionID = "tampered"
	pending.FacilityAction.ExpectedSourceStates[0].StateID = "open"
	pending.FacilityAction.AffectedConditionIDs[0] = "tampered"
	authoritative := pendingFacilityCommandSnapshot(t, fixture.service).FacilityAction
	require.Equal(t, "open", authoritative.TransitionRequests[0].TransitionID)
	require.Equal(t, "sealed", authoritative.ExpectedSourceStates[0].StateID)
	require.Equal(t, "door-alarm", authoritative.AffectedConditionIDs[0])
}

func TestFacilityCommandRejectionReturnsTypedResultAndPreservesWorld(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	before := domain.CloneFacility(fixture.initialFacility)
	require.True(t, fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("facility-reject"),
	).Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)

	state, mutation, result, err := fixture.service.ResolveCommandExecution(
		t.Context(), pending.RequestID, domain.CommandExecutionReject,
	)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Nil(t, mutation)
	require.Nil(t, state.PendingCommandExecution)
	require.Zero(t, fixture.store.CallCount())
	require.Empty(t, cmp.Diff(before, coordinatorFacilitySnapshot(t, fixture.service)))

	require.NotNil(t, result)
	require.False(t, result.OK)
	require.False(t, result.Changed)
	require.Equal(t, pending.RequestID, result.CorrelationID)
	require.Equal(t, domain.FacilityFailureRejected, result.Failure)
	require.Equal(t, before.Revision, result.PreviousFacilityRevision)
	require.Equal(t, before.Revision, result.ResultingFacilityRevision)
}

func TestFacilityCommandApprovalRevalidatesRevisionAndAuthoredFingerprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mutate      func(*domain.ProcessRuntime, string)
		wantFailure domain.FacilityFailureCode
	}{
		{
			name: "facility revision changed",
			mutate: func(runtime *domain.ProcessRuntime, _ string) {
				runtime.Facility.Revision++
			},
			wantFailure: domain.FacilityFailureStaleRevision,
		},
		{
			name: "authored action changed",
			mutate: func(runtime *domain.ProcessRuntime, commandID string) {
				command := facilityCommand(runtime, "terminal-1", commandID)
				command.StateChange.FacilityAction.Transitions.Transitions[0].TransitionID = "changed"
			},
			wantFailure: domain.FacilityFailureConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
			require.True(t, fixture.service.DispatchPlayerAction(
				fixture.controllerConnection,
				fixture.command("facility-revalidate-"+test.name),
			).Accepted)
			pending := fixture.service.Snapshot().PendingCommandExecution
			require.NotNil(t, pending)

			fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
				test.mutate(runtime, fixture.commandID)
				return transition{accepted: true}
			})
			beforeApproval := coordinatorFacilitySnapshot(t, fixture.service)

			state, mutation, result, err := fixture.service.ResolveCommandExecution(
				t.Context(), pending.RequestID, domain.CommandExecutionApprove,
			)
			require.Error(t, err)
			require.NotNil(t, state)
			require.Nil(t, mutation)
			require.Zero(t, fixture.store.CallCount(), "failed revalidation reached durability")
			require.Empty(t, cmp.Diff(beforeApproval, coordinatorFacilitySnapshot(t, fixture.service)))
			require.NotNil(t, result)
			require.Equal(t, test.wantFailure, result.Failure)
			require.False(t, result.Changed)
		})
	}
}

func TestConcurrentFacilityApprovalPersistsAndPublishesAtMostOnce(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	store := &recordingFacilityStore{started: started, release: release}
	fixture := newFacilityCommandFixture(t, store)
	store.result = successfulFacilityResult(fixture)
	require.True(t, fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("facility-concurrent"),
	).Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()

	type resolution struct {
		state          *domain.MasterCoordinationState
		mutation       *CommandStateMutation
		facilityResult *domain.FacilityOperationResult
		err            error
	}
	resolved := make(chan resolution, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Go(func() {
			state, mutation, facilityResult, err := fixture.service.ResolveCommandExecution(
				t.Context(), pending.RequestID, domain.CommandExecutionApprove,
			)
			resolved <- resolution{state: state, mutation: mutation, facilityResult: facilityResult, err: err}
		})
	}
	done := make(chan struct{})
	go func() {
		group.Wait()
		close(done)
	}()

	<-started
	require.Equal(t, effectsBefore, fixture.effects.Calls(), "facility state published before durability")
	select {
	case premature := <-resolved:
		assert.FailNow(t, "approval returned before durability", "%#v", premature)
	default:
	}
	close(release)
	<-done
	close(resolved)

	var succeeded, deduplicated int
	for resolution := range resolved {
		require.NotNil(t, resolution.facilityResult)
		result := resolution.facilityResult
		switch result.Failure {
		case domain.FacilityFailureUnspecified:
			require.NoError(t, resolution.err)
			require.True(t, result.OK)
			require.True(t, result.Changed)
			require.NotNil(t, resolution.mutation)
			succeeded++
		case domain.FacilityFailureDuplicate:
			require.Error(t, resolution.err)
			require.False(t, result.Changed)
			require.Nil(t, resolution.mutation)
			deduplicated++
		default:
			t.Fatalf("unexpected concurrent facility outcome: %#v", result)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, deduplicated)
	require.Equal(t, 1, store.CallCount())
	require.Equal(t, revisionBefore+1, fixture.service.Revision())
	require.Greater(t, fixture.effects.Calls(), effectsBefore)

	installed := coordinatorFacilitySnapshot(t, fixture.service)
	require.Equal(t, uint64(8), installed.Revision)
	require.Equal(t, "open", installed.Devices[1].CurrentStateID)
	require.True(t, installed.Conditions[0].CurrentActive)

	_, duplicateMutation, duplicateResult, duplicateErr := fixture.service.ResolveCommandExecution(
		t.Context(), pending.RequestID, domain.CommandExecutionApprove,
	)
	require.Error(t, duplicateErr)
	require.Nil(t, duplicateMutation)
	require.NotNil(t, duplicateResult)
	require.Equal(t, domain.FacilityFailureDuplicate, duplicateResult.Failure)
	require.Equal(t, 1, store.CallCount())
}

func TestFacilityActionStress1000AttemptsCommitAtMostOnceWithoutSplitState(t *testing.T) {
	t.Parallel()

	const attemptsPerCase = 250

	t.Run("duplicate request identity after commit", func(t *testing.T) {
		t.Parallel()

		store := &recordingFacilityStore{}
		fixture := newFacilityCommandFixture(t, store)
		store.result = successfulFacilityResult(fixture)
		require.True(t, fixture.service.DispatchPlayerAction(
			fixture.controllerConnection,
			fixture.command("facility-stress-duplicate"),
		).Accepted)
		pending := pendingFacilityCommandSnapshot(t, fixture.service)
		require.NotNil(t, pending)

		_, _, committed, err := fixture.service.ResolveCommandExecution(
			t.Context(), pending.RequestID, domain.CommandExecutionApprove,
		)
		require.NoError(t, err)
		require.True(t, committed.OK)
		results := resolveFacilityApprovalsConcurrently(
			t, fixture.service, repeatedRequestIDs(pending.RequestID, attemptsPerCase-1),
		)
		for _, result := range results {
			require.Error(t, result.err)
			require.NotNil(t, result.facility)
			require.Equal(t, domain.FacilityFailureDuplicate, result.facility.Failure)
		}

		require.Equal(t, 1, store.CallCount())
		installed := coordinatorFacilitySnapshot(t, fixture.service)
		require.Equal(t, uint64(8), installed.Revision)
		require.Equal(t, "open", facilityDeviceState(t, installed, "door"))
		require.True(t, facilityConditionActive(t, installed, "door-alarm"))
	})

	t.Run("stale request identities", func(t *testing.T) {
		t.Parallel()

		store := &recordingFacilityStore{}
		fixture := newFacilityCommandFixture(t, store)
		before := coordinatorFacilitySnapshot(t, fixture.service)
		require.True(t, fixture.service.DispatchPlayerAction(
			fixture.controllerConnection,
			fixture.command("facility-stress-stale"),
		).Accepted)
		requestIDs := make([]string, attemptsPerCase)
		for attempt := range requestIDs {
			requestIDs[attempt] = fmt.Sprintf("stale-facility-request-%03d", attempt)
		}

		results := resolveFacilityApprovalsConcurrently(t, fixture.service, requestIDs)
		for _, result := range results {
			require.ErrorIs(t, result.err, ErrCommandExecutionStale)
			require.Nil(t, result.facility)
		}

		require.Zero(t, store.CallCount())
		require.Empty(t, cmp.Diff(before, coordinatorFacilitySnapshot(t, fixture.service)))
	})

	t.Run("disjoint devices in one atomic action", func(t *testing.T) {
		t.Parallel()

		store := &recordingFacilityStore{}
		fixture := newFacilityCommandFixture(t, store)
		prepareMultiDeviceFacilityAuditFixture(t, &fixture)
		require.True(t, fixture.service.DispatchPlayerAction(
			fixture.controllerConnection,
			fixture.command("facility-stress-disjoint"),
		).Accepted)
		pending := pendingFacilityCommandSnapshot(t, fixture.service)
		require.NotNil(t, pending)

		results := resolveFacilityApprovalsConcurrently(
			t, fixture.service, repeatedRequestIDs(pending.RequestID, attemptsPerCase),
		)
		requireFacilityStressOutcomes(t, results, 1, attemptsPerCase-1)
		require.Equal(t, 1, store.CallCount())
		installed := coordinatorFacilitySnapshot(t, fixture.service)
		require.Equal(t, uint64(8), installed.Revision)
		require.Equal(t, "open", facilityDeviceState(t, installed, "door"))
		require.Equal(t, "running", facilityDeviceState(t, installed, "vent"))
		require.True(t, facilityConditionActive(t, installed, "door-alarm"))
		require.True(t, facilityConditionActive(t, installed, "alpha-fault"))
		require.True(t, facilityConditionActive(t, installed, "zeta-fault"))
	})

	t.Run("overlapping concurrent approvals", func(t *testing.T) {
		t.Parallel()

		store := &recordingFacilityStore{}
		fixture := newFacilityCommandFixture(t, store)
		store.result = successfulFacilityResult(fixture)
		require.True(t, fixture.service.DispatchPlayerAction(
			fixture.controllerConnection,
			fixture.command("facility-stress-overlap"),
		).Accepted)
		pending := pendingFacilityCommandSnapshot(t, fixture.service)
		require.NotNil(t, pending)

		results := resolveFacilityApprovalsConcurrently(
			t, fixture.service, repeatedRequestIDs(pending.RequestID, attemptsPerCase),
		)
		requireFacilityStressOutcomes(t, results, 1, attemptsPerCase-1)
		require.Equal(t, 1, store.CallCount())
		installed := coordinatorFacilitySnapshot(t, fixture.service)
		require.Equal(t, uint64(8), installed.Revision)
		require.Equal(t, "open", facilityDeviceState(t, installed, "door"))
		require.True(t, facilityConditionActive(t, installed, "door-alarm"))
	})
}

func TestSaveFacilityAuthoringInstallsCanonicalStateAndInvalidatesPendingAction(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	fixture.service.terminals = &recordingTerminalLifecycle{}
	require.True(t, fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("facility-authoring-pending"),
	).Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)

	candidate := facilityAuthoringSession(fixture.initialFacility)
	candidate.Facility.Devices[1].Name = "Vault door MK II"
	canonical := domain.CloneSession(candidate)
	canonical.Facility.Revision++
	store := &recordingFacilityAuthoringStore{result: domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: 42,
		PreviousFacilityRevision:  fixture.initialFacility.Revision,
		ResultingFacilityRevision: canonical.Facility.Revision,
		AffectedDeviceIDs:         []string{"door"},
		Session:                   &canonical,
	}}
	fixture.service.facilityAuthoringStore = store
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()

	state, result, err := fixture.service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: candidate, ExpectedSessionRevision: 41,
		ExpectedFacilityRevision: fixture.initialFacility.Revision, CorrelationID: "authoring-1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	require.Equal(t, revisionBefore+1, state.Revision)
	require.Equal(t, canonical.Facility.Revision, *state.FacilityRevision)
	require.Nil(t, state.PendingCommandExecution)
	require.Equal(t, 1, store.CallCount())
	require.Greater(t, fixture.effects.Calls(), effectsBefore)

	installed := coordinatorFacilitySnapshot(t, fixture.service)
	require.Equal(t, "Vault door MK II", installed.Devices[1].Name)
	result.Session.Facility.Devices[1].Name = "tampered"
	require.Equal(t, "Vault door MK II", coordinatorFacilitySnapshot(t, fixture.service).Devices[1].Name)
	terminal := canonicalTerminal(t, fixture.service, fixture.terminalID)
	require.Equal(t, domain.CommandExecutionPhaseRejected, terminal.CommandExecution.Phase)

	_, mutation, stale, resolveErr := fixture.service.ResolveCommandExecution(
		t.Context(), pending.RequestID, domain.CommandExecutionApprove,
	)
	require.ErrorIs(t, resolveErr, ErrFacilityStaleRevision)
	require.Nil(t, mutation)
	require.NotNil(t, stale)
	require.Equal(t, domain.FacilityFailureStaleRevision, stale.Failure)
}

func TestSaveFacilityAuthoringRejectsStaleFacilityBeforeDurability(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	store := &recordingFacilityAuthoringStore{}
	fixture.service.facilityAuthoringStore = store
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()

	state, result, err := fixture.service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: facilityAuthoringSession(fixture.initialFacility), ExpectedSessionRevision: 41,
		ExpectedFacilityRevision: fixture.initialFacility.Revision - 1, CorrelationID: "authoring-stale",
	})
	require.ErrorIs(t, err, ErrFacilityStaleRevision)
	require.NotNil(t, result)
	require.Equal(t, domain.FacilityFailureStaleRevision, result.Failure)
	require.Zero(t, result.SessionRevision)
	require.Equal(t, revisionBefore, state.Revision)
	require.Zero(t, store.CallCount())
	require.Equal(t, effectsBefore, fixture.effects.Calls())
}

func TestSaveFacilityAuthoringRejectsCanonicalSessionDifferentFromNormalizedCandidate(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	fixture.service.terminals = &recordingTerminalLifecycle{}
	candidate := facilityAuthoringSession(fixture.initialFacility)
	candidate.Facility.Devices[1].Name = "Vault door MK II"
	unexpected := domain.CloneSession(candidate)
	unexpected.Name = "different valid session"
	unexpected.Facility.Revision++
	fixture.service.facilityAuthoringStore = &recordingFacilityAuthoringStore{result: domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: 42,
		PreviousFacilityRevision:  fixture.initialFacility.Revision,
		ResultingFacilityRevision: unexpected.Facility.Revision,
		AffectedDeviceIDs:         []string{"door"}, Session: &unexpected,
	}}

	state, result, err := fixture.service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: candidate, ExpectedSessionRevision: 41,
		ExpectedFacilityRevision: fixture.initialFacility.Revision, CorrelationID: "authoring-wrong-session",
	})
	require.ErrorIs(t, err, ErrFacilityPersistence)
	require.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
	require.Equal(t, fixture.initialFacility.Revision, *state.FacilityRevision)
	require.Equal(t, "Vault door", coordinatorFacilitySnapshot(t, fixture.service).Devices[1].Name)
}

func TestSaveFacilityAuthoringSanitizesMalformedStoreFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*domain.FacilityOperationResult)
	}{
		{name: "correlation", mutate: func(result *domain.FacilityOperationResult) { result.CorrelationID = "wrong" }},
		{name: "changed", mutate: func(result *domain.FacilityOperationResult) { result.Changed = true }},
		{name: "revision", mutate: func(result *domain.FacilityOperationResult) { result.ResultingFacilityRevision++ }},
		{name: "affected ids", mutate: func(result *domain.FacilityOperationResult) { result.AffectedDeviceIDs = []string{"door"} }},
		{name: "invalid session", mutate: func(result *domain.FacilityOperationResult) { result.Session = &domain.Session{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
			fixture.service.terminals = &recordingTerminalLifecycle{}
			returned := domain.FacilityOperationResult{
				CorrelationID: "authoring-malformed", Failure: domain.FacilityFailureInvalidConfiguration,
				SessionRevision: 41, PreviousFacilityRevision: fixture.initialFacility.Revision,
				ResultingFacilityRevision: fixture.initialFacility.Revision,
			}
			test.mutate(&returned)
			fixture.service.facilityAuthoringStore = &recordingFacilityAuthoringStore{
				result: returned, preserveResult: true,
			}

			_, result, err := fixture.service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
				Candidate: facilityAuthoringSession(fixture.initialFacility), ExpectedSessionRevision: 41,
				ExpectedFacilityRevision: fixture.initialFacility.Revision, CorrelationID: "authoring-malformed",
			})
			require.ErrorIs(t, err, ErrFacilityPersistence)
			require.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
			require.Equal(t, "authoring-malformed", result.CorrelationID)
			require.Zero(t, result.SessionRevision)
			require.Nil(t, result.Session)
			require.Empty(t, result.AffectedDeviceIDs)
			require.Equal(t, fixture.initialFacility.Revision, result.PreviousFacilityRevision)
			require.Equal(t, result.PreviousFacilityRevision, result.ResultingFacilityRevision)
		})
	}
}

func TestSaveFacilityAuthoringStripsStateFromValidStoreFailure(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	fixture.service.terminals = &recordingTerminalLifecycle{}
	returnedSession := facilityAuthoringSession(fixture.initialFacility)
	fixture.service.facilityAuthoringStore = &recordingFacilityAuthoringStore{
		preserveResult: true,
		result: domain.FacilityOperationResult{
			CorrelationID:             "authoring-failure",
			Failure:                   domain.FacilityFailureInvalidConfiguration,
			SessionRevision:           41,
			PreviousFacilityRevision:  fixture.initialFacility.Revision,
			ResultingFacilityRevision: fixture.initialFacility.Revision,
			Session:                   &returnedSession,
		},
	}

	_, result, err := fixture.service.SaveFacilityAuthoring(t.Context(), FacilityAuthoringRequest{
		Candidate: facilityAuthoringSession(fixture.initialFacility), ExpectedSessionRevision: 41,
		ExpectedFacilityRevision: fixture.initialFacility.Revision, CorrelationID: "authoring-failure",
	})
	require.ErrorIs(t, err, ErrFacilityInvalidConfiguration)
	require.Equal(t, domain.FacilityFailureInvalidConfiguration, result.Failure)
	require.False(t, result.Changed)
	require.Nil(t, result.Session)
	require.Empty(t, result.AffectedDeviceIDs)
	require.Empty(t, result.AffectedConditionIDs)
}

func TestInspectFacilityDependenciesReturnsDetachedReportWithoutRuntimeEffects(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	session := facilityAuthoringSession(fixture.initialFacility)
	target := domain.FacilityEntityReference{Kind: domain.FacilityEntityKindDevice, EntityID: "door"}
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()
	request := FacilityDependencyInspectionRequest{
		Session: session, Target: target, ExpectedFacilityRevision: fixture.initialFacility.Revision,
	}

	result := fixture.service.InspectFacilityDependencies(t.Context(), request)

	require.True(t, result.OK, "InspectFacilityDependencies() = %#v", result)
	require.Empty(t, result.Failure)
	require.Equal(t, fixture.initialFacility.Revision, result.FacilityRevision)
	require.NotNil(t, result.Report)
	require.NotEmpty(t, result.Report.Dependencies)
	require.Equal(t, revisionBefore, fixture.service.Revision())
	require.Equal(t, effectsBefore, fixture.effects.Calls())
	require.Empty(t, cmp.Diff(fixture.initialFacility, coordinatorFacilitySnapshot(t, fixture.service)))

	result.Report.Target.EntityID = "tampered"
	result.Report.Dependencies[0].SourceID = "tampered"
	request.Session.Facility.Devices[1].Name = "tampered"
	repeated := fixture.service.InspectFacilityDependencies(t.Context(), FacilityDependencyInspectionRequest{
		Session: facilityAuthoringSession(fixture.initialFacility),
		Target:  target, ExpectedFacilityRevision: fixture.initialFacility.Revision,
	})
	require.True(t, repeated.OK)
	require.Equal(t, "door", repeated.Report.Target.EntityID)
	require.NotEqual(t, "tampered", repeated.Report.Dependencies[0].SourceID)
	require.Equal(t, revisionBefore, fixture.service.Revision())
	require.Equal(t, effectsBefore, fixture.effects.Calls())
}

func TestPreviewFacilityIsDetachedAndLeavesPendingRuntimeUnchanged(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	fixture.service.terminals = &recordingFacilityPreviewLifecycle{}
	require.True(t, fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("preview-pending"),
	).Accepted)
	pendingBefore := pendingFacilityCommandSnapshot(t, fixture.service)
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()
	preview := domain.FacilityPreview{
		ExpectedFacilityRevision: fixture.initialFacility.Revision,
		TerminalID:               fixture.terminalID,
		DeviceState: &domain.FacilityDeviceStatePreview{
			DeviceID: "door", StateID: "open",
		},
	}

	result := fixture.service.PreviewFacility(t.Context(), preview)

	require.True(t, result.OK, "PreviewFacility() = %#v", result)
	require.Empty(t, result.Failure)
	require.Equal(t, fixture.initialFacility.Revision, result.FacilityRevision)
	require.NotNil(t, result.Terminal)
	require.Equal(t, "PREVIEW:OPEN", result.Terminal.Tree.Children[0].Name)
	require.Equal(t, revisionBefore, fixture.service.Revision())
	require.Equal(t, effectsBefore, fixture.effects.Calls())
	require.Empty(t, cmp.Diff(fixture.initialFacility, coordinatorFacilitySnapshot(t, fixture.service)))
	require.Empty(t, cmp.Diff(pendingBefore, pendingFacilityCommandSnapshot(t, fixture.service)))

	result.Terminal.Tree.Children[0].Name = "tampered"
	repeated := fixture.service.PreviewFacility(t.Context(), preview)
	require.True(t, repeated.OK)
	require.Equal(t, "PREVIEW:OPEN", repeated.Terminal.Tree.Children[0].Name)
	require.Equal(t, revisionBefore, fixture.service.Revision())
	require.Equal(t, effectsBefore, fixture.effects.Calls())
}

func TestPreviewFacilityRejectsStaleRevisionWithoutEffects(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	fixture.service.terminals = &recordingFacilityPreviewLifecycle{}
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()

	result := fixture.service.PreviewFacility(t.Context(), domain.FacilityPreview{
		ExpectedFacilityRevision: fixture.initialFacility.Revision - 1,
		TerminalID:               fixture.terminalID,
		Condition: &domain.FacilityConditionPreview{
			ConditionID: "door-alarm", Active: true,
		},
	})

	require.False(t, result.OK)
	require.Equal(t, domain.FacilityFailureStaleRevision, result.Failure)
	require.Equal(t, fixture.initialFacility.Revision, result.FacilityRevision)
	require.Nil(t, result.Terminal)
	require.Equal(t, revisionBefore, fixture.service.Revision())
	require.Equal(t, effectsBefore, fixture.effects.Calls())
}

func TestResetFacilityDeviceInvalidatesPendingAndPublishesOneSnapshot(t *testing.T) {
	t.Parallel()

	store := &recordingFacilityStore{}
	fixture := newFacilityCommandFixture(t, store)
	current := facilityApprovalState(true)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(current)
		return transition{accepted: true}
	})
	require.True(t, fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("device-reset-pending"),
	).Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	canonical := facilityAuthoringSession(current)
	canonical.Facility.Revision++
	canonical.Facility.Devices[1].CurrentStateID = canonical.Facility.Devices[1].InitialStateID
	canonical.Facility.Conditions[0].CurrentActive = canonical.Facility.Conditions[0].InitialActive
	store.result = domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: 42,
		PreviousFacilityRevision: current.Revision, ResultingFacilityRevision: canonical.Facility.Revision,
		AffectedDeviceIDs: []string{"door"}, AffectedConditionIDs: []string{"door-alarm"}, Session: &canonical,
	}
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()

	state, result, err := fixture.service.ResetFacilityDevice(t.Context(), FacilityDeviceResetRequest{
		DeviceID: "door", ExpectedFacilityRevision: current.Revision, CorrelationID: "reset-door",
	})

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	require.Equal(t, revisionBefore+1, state.Revision)
	require.Equal(t, canonical.Facility.Revision, *state.FacilityRevision)
	require.Nil(t, state.PendingCommandExecution)
	require.Equal(t, 1, store.DeviceResetCallCount())
	require.Equal(t, 1, masterEffectCount(fixture.effects.Values()[effectsBefore:]))
	require.Equal(t, "sealed", facilityDeviceState(t, coordinatorFacilitySnapshot(t, fixture.service), "door"))
	require.False(t, facilityConditionActive(t, coordinatorFacilitySnapshot(t, fixture.service), "door-alarm"))

	_, mutation, stale, resolveErr := fixture.service.ResolveCommandExecution(
		t.Context(), pending.RequestID, domain.CommandExecutionApprove,
	)
	require.ErrorIs(t, resolveErr, ErrFacilityStaleRevision)
	require.Nil(t, mutation)
	require.NotNil(t, stale)
	require.Equal(t, domain.FacilityFailureStaleRevision, stale.Failure)
}

func TestResetFacilityPublishesOneSnapshotAndDetachedCanonicalResult(t *testing.T) {
	t.Parallel()

	store := &recordingFacilityStore{}
	fixture := newFacilityCommandFixture(t, store)
	current := facilityApprovalState(true)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(current)
		return transition{accepted: true}
	})
	canonical := facilityAuthoringSession(current)
	canonical.Facility.Revision++
	for index := range canonical.Facility.Devices {
		canonical.Facility.Devices[index].CurrentStateID = canonical.Facility.Devices[index].InitialStateID
	}
	for index := range canonical.Facility.Conditions {
		canonical.Facility.Conditions[index].CurrentActive = canonical.Facility.Conditions[index].InitialActive
	}
	store.result = domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: 43,
		PreviousFacilityRevision: current.Revision, ResultingFacilityRevision: canonical.Facility.Revision,
		AffectedDeviceIDs: []string{"door"}, AffectedConditionIDs: []string{"door-alarm"}, Session: &canonical,
	}
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()

	state, result, err := fixture.service.ResetFacility(t.Context(), FacilityResetRequest{
		ExpectedFacilityRevision: current.Revision, CorrelationID: "reset-facility",
	})

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	require.Equal(t, revisionBefore+1, state.Revision)
	require.Equal(t, canonical.Facility.Revision, *state.FacilityRevision)
	require.Equal(t, 1, store.FacilityResetCallCount())
	require.Equal(t, 1, masterEffectCount(fixture.effects.Values()[effectsBefore:]))

	result.Session.Facility.Devices[1].CurrentStateID = "tampered"
	require.Equal(t, "sealed", facilityDeviceState(t, coordinatorFacilitySnapshot(t, fixture.service), "door"))
}

func TestApprovedRecoveryTransitionClearsConditionThroughFacilityTransaction(t *testing.T) {
	t.Parallel()

	store := &recordingFacilityStore{}
	fixture := newFacilityCommandFixture(t, store)
	before := recoveryFacilityState()
	action := &domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
		Transitions: []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "seal"}},
	}}
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(before)
		facilityCommand(runtime, fixture.terminalID, fixture.commandID).StateChange.FacilityAction = action
		return transition{accepted: true}
	})
	store.result = successfulRecoveryCommandResult(action, false)

	selected := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("approved-transition-recovery"),
	)
	require.True(t, selected.Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)

	_, mutation, result, err := fixture.service.ResolveCommandExecution(
		t.Context(), pending.RequestID, domain.CommandExecutionApprove,
	)
	require.NoError(t, err)
	require.NotNil(t, mutation)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	require.Equal(t, []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "seal"}},
		fixture.store.Requests()[0].Transitions)

	installed := coordinatorFacilitySnapshot(t, fixture.service)
	require.Equal(t, before.Revision+1, installed.Revision)
	require.Equal(t, "sealed", facilityDeviceState(t, installed, "door"))
	require.False(t, facilityConditionActive(t, installed, "door-alarm"))
}

func TestRecoveryProgramCommandExpandsOnlyItsAllowlistedTransitions(t *testing.T) {
	t.Parallel()

	store := &recordingFacilityStore{}
	fixture := newFacilityCommandFixture(t, store)
	programID := "secure-vault"
	action := &domain.FacilityActionConfig{RecoveryProgramID: &programID}
	facility := recoveryFacilityState()
	facility.RecoveryPrograms = []domain.RecoveryProgram{{
		ID: programID, Name: "Secure vault",
		Transitions: []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "seal"}},
	}}
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(facility)
		facilityCommand(runtime, fixture.terminalID, fixture.commandID).StateChange.FacilityAction = action
		return transition{accepted: true}
	})
	store.result = successfulRecoveryCommandResult(action, true)

	require.True(t, fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("recovery-program"),
	).Accepted)
	pending := pendingFacilityCommandSnapshot(t, fixture.service)
	require.NotNil(t, pending)
	require.NotNil(t, pending.FacilityAction)
	require.Equal(t, &programID, pending.FacilityAction.RecoveryProgramID)
	require.Equal(t, facility.RecoveryPrograms[0].Transitions, pending.FacilityAction.TransitionRequests)

	_, _, result, err := fixture.service.ResolveCommandExecution(
		t.Context(), pending.RequestID, domain.CommandExecutionApprove,
	)
	require.NoError(t, err)
	require.True(t, result.OK)
	require.Equal(t, facility.RecoveryPrograms[0].Transitions, fixture.store.Requests()[0].Transitions)
}

func TestRecoverFacilityConditionUsesExplicitAuthoredPrivateRecovery(t *testing.T) {
	t.Parallel()

	store := &recordingFacilityStore{}
	fixture := newFacilityCommandFixture(t, store)
	facility := recoveryFacilityState()
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(facility)
		return transition{accepted: true}
	})
	recovered := facilityAuthoringSession(facility)
	recovered.Facility.Revision++
	recovered.Facility.Conditions[0].CurrentActive = false
	store.result = domain.FacilityOperationResult{
		OK: true, Changed: true,
		PreviousFacilityRevision: facility.Revision, ResultingFacilityRevision: recovered.Facility.Revision,
		AffectedConditionIDs: []string{"door-alarm"}, Session: &recovered,
	}
	recovery := domain.DiagnosticRecoveryReference{PrivateOverseerAction: new(true)}

	staleState, stale, err := fixture.service.RecoverFacilityCondition(t.Context(), FacilityRecoveryRequest{
		ConditionID: "door-alarm", ExpectedFacilityRevision: facility.Revision - 1,
		CorrelationID: "private-recovery-stale", Recovery: &recovery,
	})
	require.ErrorIs(t, err, ErrFacilityStaleRevision)
	require.NotNil(t, staleState)
	require.NotNil(t, stale)
	require.Equal(t, domain.FacilityFailureStaleRevision, stale.Failure)
	require.Zero(t, store.CallCount())
	require.True(t, facilityConditionActive(t, coordinatorFacilitySnapshot(t, fixture.service), "door-alarm"))

	state, result, err := fixture.service.RecoverFacilityCondition(t.Context(), FacilityRecoveryRequest{
		ConditionID: "door-alarm", ExpectedFacilityRevision: facility.Revision,
		CorrelationID: "private-recovery", Recovery: &recovery,
	})
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	require.Equal(t, recovered.Facility.Revision, *state.FacilityRevision)
	require.False(t, facilityConditionActive(t, coordinatorFacilitySnapshot(t, fixture.service), "door-alarm"))
	require.Equal(t, 1, store.CallCount())
	request := store.Requests()[0]
	require.Equal(t, "door-alarm", request.RecoveryConditionID)
	require.Equal(t, &recovery, request.Recovery)
	require.Empty(t, request.TerminalID)
	require.Empty(t, request.CommandID)
}

func TestRecoverFacilityConditionStripsStateFromValidStoreFailure(t *testing.T) {
	t.Parallel()

	store := &recordingFacilityStore{}
	fixture := newFacilityCommandFixture(t, store)
	facility := recoveryFacilityState()
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(facility)
		return transition{accepted: true}
	})
	returnedSession := facilityAuthoringSession(facility)
	store.result = domain.FacilityOperationResult{
		Failure:                   domain.FacilityFailureConflict,
		PreviousFacilityRevision:  facility.Revision,
		ResultingFacilityRevision: facility.Revision,
		Session:                   &returnedSession,
	}
	recovery := domain.DiagnosticRecoveryReference{PrivateOverseerAction: new(true)}

	_, result, err := fixture.service.RecoverFacilityCondition(t.Context(), FacilityRecoveryRequest{
		ConditionID: "door-alarm", ExpectedFacilityRevision: facility.Revision,
		CorrelationID: "recovery-failure", Recovery: &recovery,
	})
	require.ErrorIs(t, err, ErrFacilityConflict)
	require.Equal(t, domain.FacilityFailureConflict, result.Failure)
	require.False(t, result.Changed)
	require.Nil(t, result.Session)
	require.Empty(t, result.AffectedDeviceIDs)
	require.Empty(t, result.AffectedConditionIDs)
}

func TestActiveDiagnosticConditionsBlockSelectedCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		capability domain.FacilityCapability
		prepare    func(*testing.T, *facilityCommandFixture) domain.RuntimeCommand
	}{
		{
			name: "execute command", capability: domain.FacilityCapabilityExecuteCommand,
			prepare: func(_ *testing.T, fixture *facilityCommandFixture) domain.RuntimeCommand {
				return fixture.command("blocked-command")
			},
		},
		{
			name: "view entry", capability: domain.FacilityCapabilityViewEntry,
			prepare: func(_ *testing.T, fixture *facilityCommandFixture) domain.RuntimeCommand {
				fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
					terminal := runtime.Broadcast.TerminalRuntimes[fixture.terminalID]
					entry := domain.ContentNode{ID: "diagnostic-record", Type: domain.NodeEntry, Name: "Record"}
					terminal.AuthoredTree.Children = append(terminal.AuthoredTree.Children, domain.CloneContentNode(entry))
					terminal.Tree.Children = append(terminal.Tree.Children, entry)
					return transition{accepted: true}
				})
				return domain.RuntimeCommand{
					RequestID: "blocked-entry", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
					Kind: domain.RuntimeCommandNavigate, Action: "entry", NodeID: "diagnostic-record",
				}
			},
		},
		{
			name: "hack", capability: domain.FacilityCapabilityHack,
			prepare: func(_ *testing.T, fixture *facilityCommandFixture) domain.RuntimeCommand {
				return domain.RuntimeCommand{
					RequestID: "blocked-hack", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
					Kind: domain.RuntimeCommandGuess, TargetID: "candidate-wrong",
				}
			},
		},
		{
			name: "terminal transition", capability: domain.FacilityCapabilityTerminalTransition,
			prepare: func(_ *testing.T, fixture *facilityCommandFixture) domain.RuntimeCommand {
				fixture.service.terminalCatalog = &recordingTerminalCatalog{transitions: map[string]domain.TerminalTransitionTarget{
					fixture.terminalID + "/" + fixture.commandID: {
						SourceTerminalID: fixture.terminalID, CommandID: fixture.commandID,
						Target: domain.TerminalTarget{TerminalID: "terminal-2", TerminalName: "Terminal 2"},
					},
				}}
				fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
					command := facilityCommand(runtime, fixture.terminalID, fixture.commandID)
					command.StateChange = nil
					command.TerminalTransition = &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-2"}
					return transition{accepted: true}
				})
				return fixture.command("blocked-terminal-transition")
			},
		},
		{
			name: "recovery program", capability: domain.FacilityCapabilityRunRecoveryProgram,
			prepare: func(_ *testing.T, fixture *facilityCommandFixture) domain.RuntimeCommand {
				programID := "secure-vault"
				fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
					runtime.Facility.RecoveryPrograms = []domain.RecoveryProgram{{
						ID: programID, Name: "Secure vault",
						Transitions: []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "open"}},
					}}
					facilityCommand(runtime, fixture.terminalID, fixture.commandID).StateChange.FacilityAction =
						&domain.FacilityActionConfig{RecoveryProgramID: &programID}
					return transition{accepted: true}
				})
				return fixture.command("blocked-recovery-program")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
			command := test.prepare(t, &fixture)
			addActiveTerminalCapabilityBlock(fixture.service, fixture.terminalID, test.capability)
			revision := fixture.service.Revision()

			result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
			require.False(t, result.Accepted)
			require.Equal(t, domain.ActionReasonInvalidAction, result.Reason)
			require.Equal(t, revision, result.Revision)
			require.Nil(t, fixture.service.Snapshot().PendingCommandExecution)
			require.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)
			require.Zero(t, fixture.store.CallCount())
		})
	}
}

func TestBackAndRejectedCommandAcknowledgementRemainAuthoritativeEscapeActions(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	addActiveTerminalCapabilityBlock(fixture.service, fixture.terminalID,
		domain.FacilityCapabilityExecuteCommand,
		domain.FacilityCapabilityViewEntry,
		domain.FacilityCapabilityHack,
		domain.FacilityCapabilityTerminalTransition,
		domain.FacilityCapabilityRunRecoveryProgram,
	)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		terminal := runtime.Broadcast.TerminalRuntimes[fixture.terminalID]
		terminal.Nav = domain.NavState{Path: []string{"root", "docs"}, Mode: "list"}
		return transition{accepted: true}
	})

	back := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "fault-back", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "back",
	})
	require.True(t, back.Accepted)
	require.Equal(t, []string{"root"}, canonicalTerminal(t, fixture.service, fixture.terminalID).Nav.Path)

	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		terminal := runtime.Broadcast.TerminalRuntimes[fixture.terminalID]
		terminal.CommandExecution = &domain.CommandExecutionPresentation{
			Phase: domain.CommandExecutionPhaseRejected, CommandID: fixture.commandID,
		}
		return transition{accepted: true}
	})
	acknowledged := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "fault-acknowledge", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "back",
	})
	require.True(t, acknowledged.Accepted)
	require.Nil(t, canonicalTerminal(t, fixture.service, fixture.terminalID).CommandExecution)
}

func recoveryFacilityState() *domain.Facility {
	facility := facilityApprovalState(true)
	facility.Devices[1].Transitions = append(facility.Devices[1].Transitions, domain.FacilityDeviceTransition{
		ID: "seal", Name: "Seal door", SourceStateID: "open", DestinationStateID: "sealed", Recovery: true,
		ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: "door-alarm", Active: false}},
	})
	facility.Conditions[0].Effects = []domain.DiagnosticEffect{{
		CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityHack},
	}}
	facility.Conditions[0].Recovery = []domain.DiagnosticRecoveryReference{
		{Transition: &domain.FacilityTransitionRequest{DeviceID: "door", TransitionID: "seal"}},
		{PrivateOverseerAction: new(true)},
	}
	return facility
}

func successfulRecoveryCommandResult(
	action *domain.FacilityActionConfig,
	includeProgram bool,
) domain.FacilityOperationResult {
	before := recoveryFacilityState()
	if includeProgram {
		before.RecoveryPrograms = []domain.RecoveryProgram{{
			ID: *action.RecoveryProgramID, Name: "Secure vault",
			Transitions: []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "seal"}},
		}}
	}
	after := domain.CloneFacility(before)
	after.Revision++
	after.Devices[1].CurrentStateID = "sealed"
	after.Conditions[0].CurrentActive = false
	session := commandExecutionSession(true)
	session.Facility = after
	session.Terminals[0].Root.Children[0].StateChange.FacilityAction = action
	return domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: 41,
		PreviousFacilityRevision: before.Revision, ResultingFacilityRevision: after.Revision,
		AffectedDeviceIDs: []string{"door"}, AffectedConditionIDs: []string{"door-alarm"},
		Session: &session,
	}
}

func addActiveTerminalCapabilityBlock(
	service *Service,
	terminalID string,
	capabilities ...domain.FacilityCapability,
) {
	service.commit(func(runtime *domain.ProcessRuntime) transition {
		condition := domain.DiagnosticCondition{
			ID: "terminal-block", Name: "Terminal blocked",
			Category:      domain.DiagnosticConditionCategoryOffline,
			Terminal:      &domain.DiagnosticTerminalScope{TerminalID: terminalID},
			InitialActive: true, CurrentActive: true,
			Recovery: []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
		}
		for _, capability := range capabilities {
			condition.Effects = append(condition.Effects, domain.DiagnosticEffect{
				CapabilityBlock: &domain.CapabilityBlockEffect{Capability: capability},
			})
		}
		runtime.Facility.Conditions = append(runtime.Facility.Conditions, condition)
		return transition{accepted: true}
	})
}

func facilityDeviceState(t *testing.T, facility *domain.Facility, deviceID string) string {
	t.Helper()
	for _, device := range facility.Devices {
		if device.ID == deviceID {
			return device.CurrentStateID
		}
	}
	t.Fatalf("facility device %q not found", deviceID)
	return ""
}

func facilityConditionActive(t *testing.T, facility *domain.Facility, conditionID string) bool {
	t.Helper()
	for _, condition := range facility.Conditions {
		if condition.ID == conditionID {
			return condition.CurrentActive
		}
	}
	t.Fatalf("facility condition %q not found", conditionID)
	return false
}

func facilityAuthoringSession(facility *domain.Facility) domain.Session {
	session := commandExecutionSession(false)
	session.Facility = domain.CloneFacility(facility)
	command := &session.Terminals[0].Root.Children[0]
	command.StateChange.FacilityAction = &domain.FacilityActionConfig{
		Transitions: &domain.FacilityTransitionList{Transitions: []domain.FacilityTransitionRequest{{
			DeviceID: "door", TransitionID: "open",
		}}},
	}
	return session
}

type recordingFacilityPreviewLifecycle struct {
	recordingTerminalLifecycle
}

func (lifecycle *recordingFacilityPreviewLifecycle) PreviewFacility(
	runtime *domain.TerminalRuntime,
	_ *domain.Facility,
	preview domain.FacilityPreview,
) (*domain.PublicLiveState, []domain.FacilityIssue) {
	clone := cloneTerminalRuntime(runtime)
	if preview.DeviceState != nil && len(clone.Tree.Children) != 0 {
		clone.Tree.Children[0].Name = "PREVIEW:" + strings.ToUpper(preview.DeviceState.StateID)
	}
	return publicTerminalRuntime(clone), nil
}

type recordingFacilityAuthoringStore struct {
	mu             sync.Mutex
	result         domain.FacilityOperationResult
	preserveResult bool
	calls          []FacilityAuthoringRequest
}

func (store *recordingFacilityAuthoringStore) SaveFacilityAuthoring(
	_ context.Context,
	request FacilityAuthoringRequest,
) domain.FacilityOperationResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls = append(store.calls, cloneFacilityAuthoringRequest(request))
	result := cloneFacilityOperationResult(store.result)
	if !store.preserveResult {
		result.CorrelationID = request.CorrelationID
	}
	return result
}

func (store *recordingFacilityAuthoringStore) CallCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.calls)
}

type facilityCommandFixture struct {
	commandExecutionFixture
	store           *recordingFacilityStore
	initialFacility *domain.Facility
}

type facilityStressResolution struct {
	facility *domain.FacilityOperationResult
	err      error
}

func repeatedRequestIDs(requestID string, count int) []string {
	requestIDs := make([]string, count)
	for index := range requestIDs {
		requestIDs[index] = requestID
	}
	return requestIDs
}

func resolveFacilityApprovalsConcurrently(
	t *testing.T,
	service *Service,
	requestIDs []string,
) []facilityStressResolution {
	t.Helper()

	results := make(chan facilityStressResolution, len(requestIDs))
	var group sync.WaitGroup
	for _, requestID := range requestIDs {
		group.Go(func() {
			_, _, facility, err := service.ResolveCommandExecution(
				t.Context(), requestID, domain.CommandExecutionApprove,
			)
			results <- facilityStressResolution{facility: facility, err: err}
		})
	}
	group.Wait()
	close(results)

	resolutions := make([]facilityStressResolution, 0, len(requestIDs))
	for result := range results {
		resolutions = append(resolutions, result)
	}
	return resolutions
}

func requireFacilityStressOutcomes(
	t *testing.T,
	results []facilityStressResolution,
	wantCommitted int,
	wantDuplicate int,
) {
	t.Helper()

	var committed int
	var duplicate int
	for _, result := range results {
		require.NotNil(t, result.facility)
		switch result.facility.Failure {
		case domain.FacilityFailureUnspecified:
			require.NoError(t, result.err)
			require.True(t, result.facility.OK)
			require.True(t, result.facility.Changed)
			committed++
		case domain.FacilityFailureDuplicate:
			require.Error(t, result.err)
			require.False(t, result.facility.Changed)
			duplicate++
		default:
			t.Fatalf("unexpected facility stress outcome: %#v", result.facility)
		}
	}
	require.Equal(t, wantCommitted, committed)
	require.Equal(t, wantDuplicate, duplicate)
}

func prepareMultiDeviceFacilityAuditFixture(t *testing.T, fixture *facilityCommandFixture) {
	t.Helper()
	requests := []domain.FacilityTransitionRequest{
		{DeviceID: "vent", TransitionID: "start"},
		{DeviceID: "door", TransitionID: "open"},
	}
	initial := facilityApprovalState(false)
	addFacilityAuditEntities(initial, false)
	fixture.initialFacility = domain.CloneFacility(initial)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(initial)
		facilityCommand(runtime, fixture.terminalID, fixture.commandID).StateChange.FacilityAction =
			&domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
				Transitions: domain.CloneFacilityTransitionRequests(requests),
			}}
		return transition{accepted: true}
	})

	canonical := commandExecutionSession(true)
	canonical.Facility = facilityApprovalState(true)
	addFacilityAuditEntities(canonical.Facility, true)
	canonical.Terminals[0].Root.Children[0].StateChange.FacilityAction =
		&domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
			Transitions: domain.CloneFacilityTransitionRequests(requests),
		}}
	fixture.store.result = domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: 51,
		PreviousFacilityRevision:  initial.Revision,
		ResultingFacilityRevision: canonical.Facility.Revision,
		AffectedDeviceIDs:         []string{"vent", "door"},
		AffectedConditionIDs:      []string{"zeta-fault", "door-alarm", "alpha-fault"},
		Session:                   &canonical,
	}
}

func addFacilityAuditEntities(facility *domain.Facility, committed bool) {
	ventState := "idle"
	if committed {
		ventState = "running"
	}
	facility.Devices = append(facility.Devices, domain.FacilityDevice{
		ID: "vent", Name: "Ventilation", Kind: domain.FacilityDeviceKindVentilation,
		InitialStateID: "idle", CurrentStateID: ventState,
		States: []domain.FacilityDeviceState{{ID: "idle", Name: "Idle"}, {ID: "running", Name: "Running"}},
		Transitions: []domain.FacilityDeviceTransition{{
			ID: "start", Name: "Start ventilation", SourceStateID: "idle", DestinationStateID: "running",
		}},
	})
	facility.Conditions = append(facility.Conditions,
		domain.DiagnosticCondition{
			ID: "zeta-fault", Name: "Zeta fault", Category: domain.DiagnosticConditionCategoryCustom,
			CustomCategory: "zeta", Device: &domain.DiagnosticDeviceScope{DeviceID: "door"},
			CurrentActive: committed,
			Effects: []domain.DiagnosticEffect{{
				CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityHack},
			}},
			Recovery: []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
		},
		domain.DiagnosticCondition{
			ID: "alpha-fault", Name: "Alpha fault", Category: domain.DiagnosticConditionCategoryCustom,
			CustomCategory: "alpha", Device: &domain.DiagnosticDeviceScope{DeviceID: "door"},
			CurrentActive: committed,
			Effects: []domain.DiagnosticEffect{{
				CapabilityBlock: &domain.CapabilityBlockEffect{Capability: domain.FacilityCapabilityViewEntry},
			}},
			Recovery: []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
		},
	)
	door := &facility.Devices[1]
	door.Transitions[0].ConditionEffects = append(door.Transitions[0].ConditionEffects,
		domain.FacilityConditionEffect{ConditionID: "zeta-fault", Active: true},
		domain.FacilityConditionEffect{ConditionID: "alpha-fault", Active: true},
	)
}

func requireFacilityAuditCommitFacts(t *testing.T, facts *FacilityAuditFacts) {
	t.Helper()
	require.NotNil(t, facts)
	require.Equal(t, FacilityAuditActionCommand, facts.Action)
	require.Equal(t, []string{"door", "vent"}, facts.DeviceIDs)
	require.Equal(t, []string{"alpha-fault", "door-alarm", "zeta-fault"}, facts.ConditionIDs)
	require.Equal(t, uint64(7), facts.PreviousFacilityRevision)
	require.Equal(t, uint64(8), facts.ResultingFacilityRevision)
	require.Equal(t, []FacilityDeviceAuditTransition{
		{DeviceID: "door", PreviousStateID: "sealed", ResultingStateID: "open"},
		{DeviceID: "vent", PreviousStateID: "idle", ResultingStateID: "running"},
	}, facts.DeviceTransitions)
	require.Equal(t, []FacilityConditionAuditTransition{
		{ConditionID: "alpha-fault", PreviousActive: false, ResultingActive: true},
		{ConditionID: "door-alarm", PreviousActive: false, ResultingActive: true},
		{ConditionID: "zeta-fault", PreviousActive: false, ResultingActive: true},
	}, facts.ConditionTransitions)
}

func requireFacilityAuditEvent(t *testing.T, effects []Effect, name, correlationID string) AuditEvent {
	t.Helper()
	matches := matchingFacilityAuditEvents(effects, name, correlationID)
	require.Len(t, matches, 1, "facility audit events = %#v", auditEvents(effects))
	require.NotNil(t, matches[0].Facility)
	require.Equal(t, correlationID, matches[0].RequestID)
	require.Equal(t, correlationID, matches[0].Facility.CorrelationID)
	return matches[0]
}

func matchingFacilityAuditEvents(effects []Effect, name, correlationID string) []AuditEvent {
	var matches []AuditEvent
	for _, event := range auditEvents(effects) {
		if event.Name == name && event.RequestID == correlationID {
			matches = append(matches, event)
		}
	}
	return matches
}

func newFacilityCommandFixture(t *testing.T, store *recordingFacilityStore) facilityCommandFixture {
	t.Helper()
	base := newCommandExecutionFixture(t, &recordingCommandStateStore{})
	initial := facilityApprovalState(false)
	base.service.facilityStore = store
	base.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(initial)
		command := facilityCommand(runtime, base.terminalID, base.commandID)
		command.StateChange.FacilityAction = &domain.FacilityActionConfig{
			Transitions: &domain.FacilityTransitionList{Transitions: []domain.FacilityTransitionRequest{{
				DeviceID: "door", TransitionID: "open",
			}}},
		}
		return transition{accepted: true}
	})
	return facilityCommandFixture{
		commandExecutionFixture: base,
		store:                   store,
		initialFacility:         initial,
	}
}

func facilityCommand(runtime *domain.ProcessRuntime, terminalID, commandID string) *domain.ContentNode {
	for index := range runtime.Broadcast.TerminalRuntimes[terminalID].AuthoredTree.Children {
		command := &runtime.Broadcast.TerminalRuntimes[terminalID].AuthoredTree.Children[index]
		if command.ID == commandID {
			return command
		}
	}
	return nil
}

func pendingFacilityCommandSnapshot(t *testing.T, service *Service) *domain.PendingCommandExecution {
	t.Helper()
	service.mu.RLock()
	defer service.mu.RUnlock()
	if service.runtime.PendingCommandExecution == nil {
		return nil
	}
	clone := *service.runtime.PendingCommandExecution
	clone.FacilityAction = domain.ClonePendingFacilityAction(service.runtime.PendingCommandExecution.FacilityAction)
	return &clone
}

func facilityApprovalState(open bool) *domain.Facility {
	doorState := "sealed"
	alarmActive := false
	revision := uint64(7)
	if open {
		doorState = "open"
		alarmActive = true
		revision++
	}
	return &domain.Facility{
		Revision: revision,
		Devices: []domain.FacilityDevice{
			{
				ID: "power", Name: "Power grid", Kind: domain.FacilityDeviceKindPowerGrid,
				InitialStateID: "online", CurrentStateID: "online",
				States: []domain.FacilityDeviceState{{ID: "online", Name: "Online"}},
			},
			{
				ID: "door", Name: "Vault door", Kind: domain.FacilityDeviceKindDoor,
				InitialStateID: "sealed", CurrentStateID: doorState,
				States: []domain.FacilityDeviceState{{ID: "sealed", Name: "Sealed"}, {ID: "open", Name: "Open"}},
				Transitions: []domain.FacilityDeviceTransition{{
					ID: "open", Name: "Open door", SourceStateID: "sealed", DestinationStateID: "open",
					Preconditions:    []domain.FacilityStateEquality{{DeviceID: "power", StateID: "online"}},
					ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: "door-alarm", Active: true}},
				}},
			},
		},
		Conditions: []domain.DiagnosticCondition{{
			ID: "door-alarm", Name: "Door alarm", Category: domain.DiagnosticConditionCategoryOffline,
			Device: &domain.DiagnosticDeviceScope{DeviceID: "door"}, InitialActive: false, CurrentActive: alarmActive,
			Effects:  []domain.DiagnosticEffect{{DisplayInstability: &domain.DisplayInstabilityEffect{}}},
			Recovery: []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
		}},
		RecoveryPrograms: []domain.RecoveryProgram{},
	}
}

func successfulFacilityResult(fixture facilityCommandFixture) domain.FacilityOperationResult {
	session := commandExecutionSession(true)
	session.Facility = facilityApprovalState(true)
	command := &session.Terminals[0].Root.Children[0]
	command.StateChange.FacilityAction = &domain.FacilityActionConfig{
		Transitions: &domain.FacilityTransitionList{Transitions: []domain.FacilityTransitionRequest{{
			DeviceID: "door", TransitionID: "open",
		}}},
	}
	return domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: 41,
		PreviousFacilityRevision:  fixture.initialFacility.Revision,
		ResultingFacilityRevision: session.Facility.Revision,
		AffectedDeviceIDs:         []string{"door"},
		AffectedConditionIDs:      []string{"door-alarm"},
		Session:                   &session,
	}
}

type recordingFacilityStore struct {
	mu               sync.Mutex
	result           domain.FacilityOperationResult
	calls            []FacilityMutationRequest
	deviceResetCalls []FacilityDeviceResetRequest
	facilityResets   []FacilityResetRequest
	started          chan struct{}
	release          chan struct{}
	once             sync.Once
}

func (store *recordingFacilityStore) ApplyWorldAction(ctx context.Context, request FacilityMutationRequest) domain.FacilityOperationResult {
	store.mu.Lock()
	request.Transitions = domain.CloneFacilityTransitionRequests(request.Transitions)
	store.calls = append(store.calls, request)
	result := store.result
	started := store.started
	release := store.release
	store.mu.Unlock()
	if started != nil {
		store.once.Do(func() { close(started) })
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return domain.FacilityOperationResult{
				CorrelationID: request.CorrelationID,
				Failure:       domain.FacilityFailureRuntimeContextEnded,
			}
		}
	}
	result.CorrelationID = request.CorrelationID
	if result.Session != nil {
		cloned := domain.CloneSession(*result.Session)
		result.Session = &cloned
	}
	return result
}

func (store *recordingFacilityStore) CallCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.calls)
}

func (store *recordingFacilityStore) ResetFacilityDevice(
	_ context.Context,
	request FacilityDeviceResetRequest,
) domain.FacilityOperationResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.deviceResetCalls = append(store.deviceResetCalls, request)
	result := cloneFacilityOperationResult(store.result)
	result.CorrelationID = request.CorrelationID
	return result
}

func (store *recordingFacilityStore) ResetFacility(
	_ context.Context,
	request FacilityResetRequest,
) domain.FacilityOperationResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.facilityResets = append(store.facilityResets, request)
	result := cloneFacilityOperationResult(store.result)
	result.CorrelationID = request.CorrelationID
	return result
}

func (store *recordingFacilityStore) DeviceResetCallCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.deviceResetCalls)
}

func (store *recordingFacilityStore) FacilityResetCallCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.facilityResets)
}

func (store *recordingFacilityStore) Requests() []FacilityMutationRequest {
	store.mu.Lock()
	defer store.mu.Unlock()
	requests := make([]FacilityMutationRequest, len(store.calls))
	for index, request := range store.calls {
		requests[index] = cloneFacilityMutationRequest(request)
	}
	return requests
}
