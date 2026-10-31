package control

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/live"
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

func TestPreviewFacilityReturnsCurrentViewAndDifferencesFromOneSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newFacilityCommandFixture(t, &recordingFacilityStore{})
	fixture.service.terminals = &recordingFacilityPreviewLifecycle{}
	terminal := canonicalTerminal(t, fixture.service, fixture.terminalID)
	current := facilityDeviceState(t, fixture.initialFacility, "door")
	preview := domain.FacilityPreview{
		ExpectedFacilityRevision: fixture.initialFacility.Revision,
		TerminalID:               fixture.terminalID,
		DeviceState:              &domain.FacilityDeviceStatePreview{DeviceID: "door", StateID: "open"},
	}
	wantDifferences, issues := live.PreviewDifferences(terminal, fixture.initialFacility, preview)
	require.Empty(t, issues)

	result := fixture.service.PreviewFacility(t.Context(), preview)

	require.True(t, result.OK, "PreviewFacility() = %#v", result)
	require.NotNil(t, result.Current)
	require.Equal(t, "PREVIEW:"+strings.ToUpper(current), result.Current.Tree.Children[0].Name,
		"the current view must use the installed device state")
	require.Equal(t, "PREVIEW:OPEN", result.Terminal.Tree.Children[0].Name)
	require.Empty(t, cmp.Diff(wantDifferences, result.Differences))
	require.Equal(t, "open", preview.DeviceState.StateID, "the caller's preview must stay unchanged")
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

func (store *recordingFacilityStore) DeviceResetCallCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.deviceResetCalls)
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

// The play fixture extends the facility command fixture for the Overseer
// play operations. The door gains a third state that no transition reaches,
// the ventilation has a fault without operator recovery, terminal-1 has a
// terminal-scoped fault, and terminal-2 is a suspended broadcast terminal.
const (
	facilityPlaySuspendedTerminalID = "terminal-2"
	facilityPlayTerminalFaultID     = "terminal-fault"
	facilityPlayRevision            = uint64(7)
)

func TestFacilityPlayChangeCommitsReprojectsAuditsAndFeedsOneChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		prepare    func(*testing.T, *domain.Facility)
		call       facilityPlayCall
		mutate     func(*testing.T, *domain.Facility)
		devices    []string
		conditions []string
		wantAudit  facilityPlayAudit
		wantFeed   []domain.FacilityChange
	}{
		{
			name: "card transition",
			call: facilityPlayTransition("door", "open", facilityPlayRevision, "play-transition"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "open"
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			devices: []string{"door"}, conditions: []string{"door-alarm"},
			wantAudit: facilityPlayAudit{
				DeviceIDs: []string{"door"}, ConditionIDs: []string{"door-alarm"},
				DeviceTransitions: []FacilityDeviceAuditTransition{{
					DeviceID: "door", PreviousStateID: "sealed", ResultingStateID: "open",
				}},
				ConditionTransitions: []FacilityConditionAuditTransition{{
					ConditionID: "door-alarm", PreviousActive: false, ResultingActive: true,
				}},
			},
			wantFeed: []domain.FacilityChange{{DeviceID: "door", Transition: &domain.FacilityChangeTransition{
				TransitionID: "open", FromStateID: "sealed", ToStateID: "open",
			}}},
		},
		{
			name: "forced change to a state no transition reaches",
			call: facilityPlayForce("door", "jammed", facilityPlayRevision, "play-force"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "jammed"
			},
			devices: []string{"door"},
			wantAudit: facilityPlayAudit{
				DeviceIDs: []string{"door"},
				DeviceTransitions: []FacilityDeviceAuditTransition{{
					DeviceID: "door", PreviousStateID: "sealed", ResultingStateID: "jammed",
				}},
			},
			wantFeed: []domain.FacilityChange{{DeviceID: "door", Forced: &domain.FacilityChangeForced{
				FromStateID: "sealed", ToStateID: "jammed",
			}}},
		},
		{
			name: "fault switched on",
			call: facilityPlayFault("door-alarm", true, facilityPlayRevision, "play-fault-on"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			conditions: []string{"door-alarm"},
			wantAudit: facilityPlayAudit{
				DeviceIDs: []string{"door"}, ConditionIDs: []string{"door-alarm"},
				ConditionTransitions: []FacilityConditionAuditTransition{{
					ConditionID: "door-alarm", PreviousActive: false, ResultingActive: true,
				}},
			},
			wantFeed: []domain.FacilityChange{{DeviceID: "door", Fault: &domain.FacilityChangeFault{
				ConditionID: "door-alarm", Active: true,
			}}},
		},
		{
			name: "fault switched off",
			prepare: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			call: facilityPlayFault("door-alarm", false, facilityPlayRevision, "play-fault-off"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = false
			},
			conditions: []string{"door-alarm"},
			wantAudit: facilityPlayAudit{
				DeviceIDs: []string{"door"}, ConditionIDs: []string{"door-alarm"},
				ConditionTransitions: []FacilityConditionAuditTransition{{
					ConditionID: "door-alarm", PreviousActive: true, ResultingActive: false,
				}},
			},
			wantFeed: []domain.FacilityChange{{DeviceID: "door", Fault: &domain.FacilityChangeFault{
				ConditionID: "door-alarm", Active: false,
			}}},
		},
		{
			name: "fault without operator recovery",
			call: facilityPlayFault("vent-clog", true, facilityPlayRevision, "play-fault-no-recovery"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "vent-clog").CurrentActive = true
			},
			conditions: []string{"vent-clog"},
			wantAudit: facilityPlayAudit{
				DeviceIDs: []string{"vent"}, ConditionIDs: []string{"vent-clog"},
				ConditionTransitions: []FacilityConditionAuditTransition{{
					ConditionID: "vent-clog", PreviousActive: false, ResultingActive: true,
				}},
			},
			wantFeed: []domain.FacilityChange{{DeviceID: "vent", Fault: &domain.FacilityChangeFault{
				ConditionID: "vent-clog", Active: true,
			}}},
		},
		{
			name: "terminal-scoped fault feeds no device",
			call: facilityPlayFault(facilityPlayTerminalFaultID, true, facilityPlayRevision, "play-fault-terminal"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, facilityPlayTerminalFaultID).CurrentActive = true
			},
			conditions: []string{facilityPlayTerminalFaultID},
			wantAudit: facilityPlayAudit{
				ConditionIDs: []string{facilityPlayTerminalFaultID},
				ConditionTransitions: []FacilityConditionAuditTransition{{
					ConditionID: facilityPlayTerminalFaultID, PreviousActive: false, ResultingActive: true,
				}},
			},
		},
		{
			name: "device reset",
			prepare: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "open"
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			call: facilityPlayReset("door", facilityPlayRevision, "play-reset"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "sealed"
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = false
			},
			devices: []string{"door"}, conditions: []string{"door-alarm"},
			wantAudit: facilityPlayAudit{
				DeviceIDs: []string{"door"}, ConditionIDs: []string{"door-alarm"},
				DeviceTransitions: []FacilityDeviceAuditTransition{{
					DeviceID: "door", PreviousStateID: "open", ResultingStateID: "sealed",
				}},
				ConditionTransitions: []FacilityConditionAuditTransition{{
					ConditionID: "door-alarm", PreviousActive: true, ResultingActive: false,
				}},
			},
			wantFeed: []domain.FacilityChange{{DeviceID: "door", Reset: &domain.FacilityChangeReset{
				FromStateID: "open", ToStateID: "sealed", ConditionIDs: []string{"door-alarm"},
			}}},
		},
		{
			name: "device reset of faults only",
			prepare: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			call: facilityPlayReset("door", facilityPlayRevision, "play-reset-faults"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = false
			},
			conditions: []string{"door-alarm"},
			wantAudit: facilityPlayAudit{
				DeviceIDs: []string{"door"}, ConditionIDs: []string{"door-alarm"},
				ConditionTransitions: []FacilityConditionAuditTransition{{
					ConditionID: "door-alarm", PreviousActive: true, ResultingActive: false,
				}},
			},
			wantFeed: []domain.FacilityChange{{DeviceID: "door", Reset: &domain.FacilityChangeReset{
				FromStateID: "sealed", ToStateID: "sealed", ConditionIDs: []string{"door-alarm"},
			}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityPlayFixture(t, test.prepare)
			durable := facilityPlayDurable(t, fixture, test.mutate, test.devices, test.conditions)
			fixture.play.setResult(durable)
			terminalBefore := canonicalTerminal(t, fixture.service, fixture.terminalID)
			revisionBefore := fixture.service.Revision()
			effectsBefore := fixture.effects.Calls()
			projectionsBefore := fixture.lifecycle.Projections()

			state, result, err := test.call.run(t.Context(), fixture.service)

			require.NoError(t, err)
			require.NotNil(t, state)
			require.NotNil(t, result)
			durable.CorrelationID = test.call.correlationID
			require.Empty(t, cmp.Diff(durable, *result), "the result must be the verified durable result")
			require.Equal(t, []any{test.call.request}, fixture.play.Requests())
			require.Equal(t, revisionBefore+1, state.Revision)
			require.NotNil(t, state.FacilityRevision)
			require.Equal(t, facilityPlayRevision+1, *state.FacilityRevision)
			require.Empty(t, cmp.Diff(durable.Session.Facility, coordinatorFacilitySnapshot(t, fixture.service)),
				"the durable facility must be installed")

			terminalAfter := canonicalTerminal(t, fixture.service, fixture.terminalID)
			require.Equal(t, terminalBefore.CommandStates, terminalAfter.CommandStates,
				"a play change must not mark any command executed")
			require.Zero(t, fixture.commandExecutionFixture.store.ExecuteCalls())

			effects := fixture.effects.Values()[effectsBefore:]
			require.Equal(t, 1, masterEffectCount(effects), "one commit publishes one master snapshot")
			live := facilityPlayLiveEffects(effects)
			require.Len(t, live, 1, "the active terminal gets one reprojected live state")
			require.Equal(t, fixture.terminalID, live[0].TerminalID)
			require.Equal(t, []facilityPlayProjection{
				{TerminalID: fixture.terminalID, FacilityRevision: facilityPlayRevision + 1},
				{TerminalID: facilityPlaySuspendedTerminalID, FacilityRevision: facilityPlayRevision + 1},
			}, fixture.lifecycle.Projections()[len(projectionsBefore):],
				"every broadcast terminal must be reprojected with the committed facility")

			event := requireFacilityAuditEvent(t, effects, test.call.event, test.call.correlationID)
			want := test.wantAudit
			want.Outcome = "succeeded"
			want.PreviousFacilityRevision = facilityPlayRevision
			want.ResultingFacilityRevision = facilityPlayRevision + 1
			requireFacilityPlayAudit(t, test.call, want, event)

			require.Empty(t, cmp.Diff(test.wantFeed, facilityPlayFeed(state.FacilityChanges), cmpopts.EquateEmpty()))
			requireFacilityPlayFeedOrder(t, state.FacilityChanges)
			require.Empty(t, cmp.Diff(state.FacilityChanges, fixture.service.Snapshot().FacilityChanges, cmpopts.EquateEmpty()))
			require.Empty(t, cmp.Diff(state.FacilityChanges, facilityPlayMasterFeed(t, effects), cmpopts.EquateEmpty()),
				"the published master snapshot must carry the feed")
		})
	}
}

func TestFacilityPlayChangeWithdrawsThePendingRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		prepare    func(*testing.T, *domain.Facility)
		call       facilityPlayCall
		mutate     func(*testing.T, *domain.Facility)
		devices    []string
		conditions []string
		wantOwn    []domain.FacilityChange
	}{
		{
			name: "card transition",
			call: facilityPlayTransition("door", "open", facilityPlayRevision, "withdraw-transition"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "open"
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			devices: []string{"door"}, conditions: []string{"door-alarm"},
			wantOwn: []domain.FacilityChange{{DeviceID: "door", Transition: &domain.FacilityChangeTransition{
				TransitionID: "open", FromStateID: "sealed", ToStateID: "open",
			}}},
		},
		{
			name: "forced change",
			call: facilityPlayForce("door", "jammed", facilityPlayRevision, "withdraw-force"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "jammed"
			},
			devices: []string{"door"},
			wantOwn: []domain.FacilityChange{{DeviceID: "door", Forced: &domain.FacilityChangeForced{
				FromStateID: "sealed", ToStateID: "jammed",
			}}},
		},
		{
			name: "fault switch",
			call: facilityPlayFault("door-alarm", true, facilityPlayRevision, "withdraw-fault"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			conditions: []string{"door-alarm"},
			wantOwn: []domain.FacilityChange{{DeviceID: "door", Fault: &domain.FacilityChangeFault{
				ConditionID: "door-alarm", Active: true,
			}}},
		},
		{
			name: "terminal-scoped fault switch",
			call: facilityPlayFault(facilityPlayTerminalFaultID, true, facilityPlayRevision, "withdraw-terminal-fault"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, facilityPlayTerminalFaultID).CurrentActive = true
			},
			conditions: []string{facilityPlayTerminalFaultID},
		},
		{
			name: "device reset of faults only",
			prepare: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			call: facilityPlayReset("door", facilityPlayRevision, "withdraw-reset"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = false
			},
			conditions: []string{"door-alarm"},
			wantOwn: []domain.FacilityChange{{DeviceID: "door", Reset: &domain.FacilityChangeReset{
				FromStateID: "sealed", ToStateID: "sealed", ConditionIDs: []string{"door-alarm"},
			}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityPlayFixture(t, test.prepare)
			pending := fixture.requirePendingRequest(t, "withdraw-pending")
			fixture.play.setResult(facilityPlayDurable(t, fixture, test.mutate, test.devices, test.conditions))
			effectsBefore := fixture.effects.Calls()

			state, result, err := test.call.run(t.Context(), fixture.service)

			require.NoError(t, err)
			require.True(t, result.OK)
			require.True(t, result.Changed)
			require.Nil(t, state.PendingCommandExecution)
			require.Nil(t, fixture.service.Snapshot().PendingCommandExecution)
			require.Equal(t, &domain.CommandExecutionPresentation{
				Phase: domain.CommandExecutionPhaseRejected, CommandID: fixture.commandID,
			}, canonicalTerminal(t, fixture.service, fixture.terminalID).CommandExecution,
				"the player's terminal must show the withdrawn request as rejected")

			effects := fixture.effects.Values()[effectsBefore:]
			require.Equal(t, 1, masterEffectCount(effects))
			records := facilityPlayRequestRecords(effects, pending.RequestID)
			require.Len(t, records, 1, "the withdrawn request gets exactly one outcome record: %#v", records)
			record := records[0]
			require.Equal(t, "command.request_outcome", record.Name)
			require.Equal(t, "superseded", record.Outcome)
			require.Equal(t, fixture.terminalID, record.TerminalID)
			require.Equal(t, fixture.commandID, record.CommandID)
			require.NotNil(t, record.Facility, "the withdrawal record must carry revisions")
			require.Equal(t, pending.RequestID, record.Facility.CorrelationID)
			require.Equal(t, facilityPlayRevision, record.Facility.PreviousFacilityRevision)
			require.Equal(t, facilityPlayRevision+1, record.Facility.ResultingFacilityRevision)
			requireFacilityAuditEvent(t, effects, test.call.event, test.call.correlationID)

			wantFeed := append([]domain.FacilityChange{
				facilityPlayWithdrawn("door", fixture.terminalID, fixture.commandID),
			}, test.wantOwn...)
			require.Empty(t, cmp.Diff(wantFeed, facilityPlayFeed(state.FacilityChanges), cmpopts.EquateEmpty()),
				"the withdrawal entry follows the play change's own entry")
			requireFacilityPlayFeedOrder(t, state.FacilityChanges)
			requireFacilityPlayOneCommitTime(t, state.FacilityChanges)

			_, mutation, stale, resolveErr := fixture.service.ResolveCommandExecution(
				t.Context(), pending.RequestID, domain.CommandExecutionApprove,
			)
			require.ErrorIs(t, resolveErr, ErrFacilityStaleRevision)
			require.Nil(t, mutation)
			require.NotNil(t, stale)
			require.Zero(t, fixture.store.CallCount(), "a withdrawn request must never reach the world")
		})
	}
}

func TestFacilityPlayWithdrawalFeedsEachTargetedDeviceInIDOrder(t *testing.T) {
	t.Parallel()

	fixture := newFacilityPlayFixture(t, nil)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		facilityCommand(runtime, fixture.terminalID, fixture.commandID).StateChange.FacilityAction =
			&domain.FacilityActionConfig{Transitions: &domain.FacilityTransitionList{
				Transitions: []domain.FacilityTransitionRequest{
					{DeviceID: "vent", TransitionID: "start"},
					{DeviceID: "door", TransitionID: "open"},
				},
			}}
		return transition{accepted: true}
	})
	fixture.requirePendingRequest(t, "withdraw-two-devices")
	fixture.play.setResult(facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
		facilityPlayDevice(t, facility, "door").CurrentStateID = "jammed"
	}, []string{"door"}, nil))

	state, result, err := fixture.service.ForceFacilityDeviceState(t.Context(), FacilityDeviceForceRequest{
		DeviceID: "door", StateID: "jammed",
		ExpectedFacilityRevision: facilityPlayRevision, CorrelationID: "withdraw-two-devices-force",
	})

	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Empty(t, cmp.Diff([]domain.FacilityChange{
		facilityPlayWithdrawn("vent", fixture.terminalID, fixture.commandID),
		facilityPlayWithdrawn("door", fixture.terminalID, fixture.commandID),
		{DeviceID: "door", Forced: &domain.FacilityChangeForced{FromStateID: "sealed", ToStateID: "jammed"}},
	}, facilityPlayFeed(state.FacilityChanges), cmpopts.EquateEmpty()),
		"withdrawal entries follow the play change's entry, in device ID order")
	requireFacilityPlayFeedOrder(t, state.FacilityChanges)
	requireFacilityPlayOneCommitTime(t, state.FacilityChanges)
	sequences := make([]uint64, len(state.FacilityChanges))
	for index, change := range state.FacilityChanges {
		sequences[index] = change.Sequence
	}
	first := sequences[len(sequences)-1]
	require.Equal(t, []uint64{first + 2, first + 1, first}, sequences,
		"one commit numbers its entries consecutively")
}

func TestFacilityPlayChangeThatChangesNothingLeavesTheRuntimeAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call facilityPlayCall
	}{
		{name: "forced change to the current state", call: facilityPlayForce("door", "sealed", facilityPlayRevision, "noop-force")},
		{name: "fault already in position", call: facilityPlayFault("door-alarm", false, facilityPlayRevision, "noop-fault")},
		{name: "device reset at its starting values", call: facilityPlayReset("door", facilityPlayRevision, "noop-reset")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityPlayFixture(t, nil)
			pending := fixture.requirePendingRequest(t, "noop-pending")
			unchanged := facilityPlayUnchanged(t, fixture)
			fixture.play.setResult(unchanged)
			before := fixture.service.Snapshot()
			facilityBefore := coordinatorFacilitySnapshot(t, fixture.service)
			effectsBefore := fixture.effects.Calls()
			projectionsBefore := len(fixture.lifecycle.Projections())

			state, result, err := test.call.run(t.Context(), fixture.service)

			require.NoError(t, err)
			require.True(t, result.OK)
			require.False(t, result.Changed)
			unchanged.CorrelationID = test.call.correlationID
			require.Empty(t, cmp.Diff(unchanged, *result))
			require.Equal(t, before.Revision, state.Revision)
			require.Empty(t, cmp.Diff(facilityBefore, coordinatorFacilitySnapshot(t, fixture.service)))
			after := fixture.service.Snapshot().PendingCommandExecution
			require.NotNil(t, after, "an unchanged play change must not withdraw the pending request")
			require.Equal(t, pending.RequestID, after.RequestID)
			require.Empty(t, state.FacilityChanges, "an unchanged play change adds no feed entry")
			require.Empty(t, fixture.service.Snapshot().FacilityChanges)

			effects := fixture.effects.Values()[effectsBefore:]
			require.Zero(t, masterEffectCount(effects))
			require.Empty(t, facilityPlayLiveEffects(effects))
			require.Len(t, fixture.lifecycle.Projections(), projectionsBefore, "nothing is reprojected")
			require.Empty(t, facilityPlayRequestRecords(effects, pending.RequestID))
			event := requireFacilityAuditEvent(t, effects, test.call.event, test.call.correlationID)
			requireFacilityPlayAudit(t, test.call, facilityPlayAudit{
				Outcome: "unchanged", DeviceIDs: test.call.auditDeviceIDs, ConditionIDs: test.call.auditConditionIDs,
				PreviousFacilityRevision: facilityPlayRevision, ResultingFacilityRevision: facilityPlayRevision,
			}, event)
		})
	}
}

func TestFacilityPlayChangeRejectedBeforeTheStoreChangesNothing(t *testing.T) {
	t.Parallel()

	type rejection struct {
		name         string
		call         facilityPlayCall
		setup        func(*facilityPlayFixture)
		canceled     bool
		wantFailure  domain.FacilityFailureCode
		wantOutcome  string
		wantRevision uint64
	}
	var tests []rejection
	for _, call := range []facilityPlayCall{
		facilityPlayTransition("door", "open", facilityPlayRevision, "reject-transition"),
		facilityPlayForce("door", "jammed", facilityPlayRevision, "reject-force"),
		facilityPlayFault("door-alarm", true, facilityPlayRevision, "reject-fault"),
	} {
		tests = append(tests,
			rejection{
				name: call.event + "/stale revision", call: call.withRevision(facilityPlayRevision - 1),
				wantFailure: domain.FacilityFailureStaleRevision, wantOutcome: "stale", wantRevision: facilityPlayRevision,
			},
			rejection{
				name: call.event + "/blank correlation", call: call.withCorrelationID(" "),
				wantFailure: domain.FacilityFailureInvalidConfiguration, wantOutcome: "invalid", wantRevision: facilityPlayRevision,
			},
			rejection{
				name: call.event + "/blank target", call: call.withTargetID(" "),
				wantFailure: domain.FacilityFailureInvalidConfiguration, wantOutcome: "invalid", wantRevision: facilityPlayRevision,
			},
			rejection{
				name: call.event + "/canceled context", call: call, canceled: true,
				wantFailure: domain.FacilityFailureRuntimeContextEnded, wantOutcome: "failed", wantRevision: facilityPlayRevision,
			},
			rejection{
				name: call.event + "/no facility", call: call,
				setup: func(fixture *facilityPlayFixture) {
					fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
						runtime.Facility = nil
						return transition{accepted: true}
					})
				},
				wantFailure: domain.FacilityFailureMissingReference, wantOutcome: "failed",
			},
			rejection{
				name: call.event + "/store without play operations", call: call,
				setup: func(fixture *facilityPlayFixture) {
					fixture.service.facilityStore = fixture.store
				},
				wantFailure: domain.FacilityFailurePersistenceFailed, wantOutcome: "failed", wantRevision: facilityPlayRevision,
			},
		)
	}
	for _, call := range []facilityPlayCall{
		facilityPlayTransition("door", " ", facilityPlayRevision, "reject-blank-transition"),
		facilityPlayForce("door", " ", facilityPlayRevision, "reject-blank-state"),
	} {
		tests = append(tests, rejection{
			name: call.event + "/blank value", call: call,
			wantFailure: domain.FacilityFailureInvalidConfiguration, wantOutcome: "invalid", wantRevision: facilityPlayRevision,
		})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityPlayFixture(t, nil)
			pending := fixture.requirePendingRequest(t, "reject-pending")
			fixture.play.setResult(facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "jammed"
			}, []string{"door"}, nil))
			if test.setup != nil {
				test.setup(&fixture)
			}
			facilityBefore := coordinatorFacilitySnapshot(t, fixture.service)
			revisionBefore := fixture.service.Revision()
			effectsBefore := fixture.effects.Calls()
			projectionsBefore := len(fixture.lifecycle.Projections())
			ctx := t.Context()
			if test.canceled {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}

			state, result, err := test.call.run(ctx, fixture.service)

			require.ErrorIs(t, err, facilityFailureError(test.wantFailure))
			require.NotNil(t, state)
			require.NotNil(t, result)
			require.False(t, result.OK)
			require.False(t, result.Changed)
			require.Equal(t, test.wantFailure, result.Failure)
			require.Equal(t, test.call.correlationID, result.CorrelationID)
			require.Equal(t, test.wantRevision, result.PreviousFacilityRevision)
			require.Equal(t, test.wantRevision, result.ResultingFacilityRevision)
			require.Empty(t, result.AffectedDeviceIDs)
			require.Empty(t, result.AffectedConditionIDs)
			require.Nil(t, result.Session)
			require.Empty(t, fixture.play.Requests(), "a rejected play change must not reach the store")
			require.Equal(t, revisionBefore, fixture.service.Revision())
			require.Empty(t, cmp.Diff(facilityBefore, coordinatorFacilitySnapshot(t, fixture.service)))
			after := fixture.service.Snapshot().PendingCommandExecution
			require.NotNil(t, after, "a rejected play change must not withdraw the pending request")
			require.Equal(t, pending.RequestID, after.RequestID)
			require.Empty(t, fixture.service.Snapshot().FacilityChanges, "a rejected play change adds no feed entry")

			effects := fixture.effects.Values()[effectsBefore:]
			require.Zero(t, masterEffectCount(effects))
			require.Empty(t, facilityPlayLiveEffects(effects))
			require.Len(t, fixture.lifecycle.Projections(), projectionsBefore)
			event := requireFacilityAuditEvent(t, effects, test.call.event, test.call.correlationID)
			requireFacilityPlayRejectionAudit(t, test.call, test.wantOutcome, test.wantFailure, test.wantRevision, event)
		})
	}
}

func TestFacilityPlayChangeKeepsAValidStoreFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		call    facilityPlayCall
		failure domain.FacilityFailureCode
		issue   domain.FacilityIssue
	}{
		{
			name: "unmet precondition", call: facilityPlayTransition("door", "open", facilityPlayRevision, "store-precondition"),
			failure: domain.FacilityFailurePreconditionFailed,
			issue:   domain.FacilityIssue{Code: domain.FacilityFailurePreconditionFailed, EntityKind: "device", EntityID: new("power")},
		},
		{
			name: "device not in the source state", call: facilityPlayTransition("door", "open", facilityPlayRevision, "store-source"),
			failure: domain.FacilityFailureInvalidTransition,
			issue:   domain.FacilityIssue{Code: domain.FacilityFailureInvalidTransition, EntityKind: "device", EntityID: new("door")},
		},
		{
			name: "missing state", call: facilityPlayForce("door", "melted", facilityPlayRevision, "store-missing-state"),
			failure: domain.FacilityFailureMissingReference,
			issue:   domain.FacilityIssue{Code: domain.FacilityFailureMissingReference, EntityKind: "state", EntityID: new("melted")},
		},
		{
			name: "missing condition", call: facilityPlayFault("lamp-flicker", true, facilityPlayRevision, "store-missing-condition"),
			failure: domain.FacilityFailureMissingReference,
			issue:   domain.FacilityIssue{Code: domain.FacilityFailureMissingReference, EntityKind: "condition", EntityID: new("lamp-flicker")},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityPlayFixture(t, nil)
			pending := fixture.requirePendingRequest(t, "store-failure-pending")
			fixture.play.setResult(domain.FacilityOperationResult{
				Failure: test.failure, Issues: []domain.FacilityIssue{test.issue},
				PreviousFacilityRevision: facilityPlayRevision, ResultingFacilityRevision: facilityPlayRevision,
			})
			facilityBefore := coordinatorFacilitySnapshot(t, fixture.service)
			effectsBefore := fixture.effects.Calls()

			_, result, err := test.call.run(t.Context(), fixture.service)

			require.ErrorIs(t, err, facilityFailureError(test.failure))
			require.False(t, result.OK)
			require.False(t, result.Changed)
			require.Equal(t, test.failure, result.Failure)
			require.Equal(t, []domain.FacilityIssue{test.issue}, result.Issues)
			require.Equal(t, facilityPlayRevision, result.PreviousFacilityRevision)
			require.Equal(t, facilityPlayRevision, result.ResultingFacilityRevision)
			require.Nil(t, result.Session)
			require.Len(t, fixture.play.Requests(), 1)
			require.Empty(t, cmp.Diff(facilityBefore, coordinatorFacilitySnapshot(t, fixture.service)))
			require.Equal(t, pending.RequestID, fixture.service.Snapshot().PendingCommandExecution.RequestID)
			require.Empty(t, fixture.service.Snapshot().FacilityChanges)

			effects := fixture.effects.Values()[effectsBefore:]
			require.Zero(t, masterEffectCount(effects))
			event := requireFacilityAuditEvent(t, effects, test.call.event, test.call.correlationID)
			requireFacilityPlayRejectionAudit(t, test.call, "failed", test.failure, facilityPlayRevision, event)
		})
	}
}

func TestFacilityPlayChangeRejectsADurableResultThatDoesNotVerify(t *testing.T) {
	t.Parallel()

	moveDoor := func(stateID string) func(*testing.T, *domain.Facility) {
		return func(t *testing.T, facility *domain.Facility) {
			facilityPlayDevice(t, facility, "door").CurrentStateID = stateID
		}
	}
	tests := []struct {
		name   string
		call   facilityPlayCall
		result func(*testing.T, facilityPlayFixture) domain.FacilityOperationResult
	}{
		{
			name: "transition reported as unchanged",
			call: facilityPlayTransition("door", "open", facilityPlayRevision, "verify-transition-unchanged"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayUnchanged(t, fixture)
			},
		},
		{
			name: "transition moved the device elsewhere",
			call: facilityPlayTransition("door", "open", facilityPlayRevision, "verify-transition-elsewhere"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
					facilityPlayDevice(t, facility, "door").CurrentStateID = "jammed"
					facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
				}, []string{"door"}, []string{"door-alarm"})
			},
		},
		{
			name: "transition skipped its fault effect",
			call: facilityPlayTransition("door", "open", facilityPlayRevision, "verify-transition-effect"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayDurable(t, fixture, moveDoor("open"), []string{"door"}, []string{"door-alarm"})
			},
		},
		{
			name: "transition named no device",
			call: facilityPlayTransition("door", "open", facilityPlayRevision, "verify-transition-devices"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
					facilityPlayDevice(t, facility, "door").CurrentStateID = "open"
					facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
				}, nil, []string{"door-alarm"})
			},
		},
		{
			name: "transition skipped a revision",
			call: facilityPlayTransition("door", "open", facilityPlayRevision, "verify-transition-revision"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				result := facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
					facilityPlayDevice(t, facility, "door").CurrentStateID = "open"
					facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
					facility.Revision++
				}, []string{"door"}, []string{"door-alarm"})
				return result
			},
		},
		{
			name: "transition without a session",
			call: facilityPlayTransition("door", "open", facilityPlayRevision, "verify-transition-session"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				result := facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
					facilityPlayDevice(t, facility, "door").CurrentStateID = "open"
					facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
				}, []string{"door"}, []string{"door-alarm"})
				result.Session = nil
				return result
			},
		},
		{
			name: "forced change from another revision",
			call: facilityPlayForce("door", "jammed", facilityPlayRevision, "verify-force-previous"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				result := facilityPlayDurable(t, fixture, moveDoor("jammed"), []string{"door"}, nil)
				result.PreviousFacilityRevision--
				return result
			},
		},
		{
			name: "forced change ran a fault effect",
			call: facilityPlayForce("door", "open", facilityPlayRevision, "verify-force-effect"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
					facilityPlayDevice(t, facility, "door").CurrentStateID = "open"
					facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
				}, []string{"door"}, []string{"door-alarm"})
			},
		},
		{
			name: "forced change reached another state",
			call: facilityPlayForce("door", "jammed", facilityPlayRevision, "verify-force-state"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayDurable(t, fixture, moveDoor("open"), []string{"door"}, nil)
			},
		},
		{
			name: "fault switch changed the starting position",
			call: facilityPlayFault("door-alarm", true, facilityPlayRevision, "verify-fault-initial"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
					condition := facilityPlayCondition(t, facility, "door-alarm")
					condition.CurrentActive = true
					condition.InitialActive = true
				}, nil, []string{"door-alarm"})
			},
		},
		{
			name: "fault switch named a device",
			call: facilityPlayFault("door-alarm", true, facilityPlayRevision, "verify-fault-devices"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
					facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
				}, []string{"door"}, []string{"door-alarm"})
			},
		},
		{
			name: "fault switch left the fault in place",
			call: facilityPlayFault("door-alarm", true, facilityPlayRevision, "verify-fault-position"),
			result: func(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
				return facilityPlayDurable(t, fixture, func(*testing.T, *domain.Facility) {}, nil, []string{"door-alarm"})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFacilityPlayFixture(t, nil)
			pending := fixture.requirePendingRequest(t, "verify-pending")
			fixture.play.setResult(test.result(t, fixture))
			facilityBefore := coordinatorFacilitySnapshot(t, fixture.service)
			revisionBefore := fixture.service.Revision()
			effectsBefore := fixture.effects.Calls()
			projectionsBefore := len(fixture.lifecycle.Projections())

			_, result, err := test.call.run(t.Context(), fixture.service)

			require.ErrorIs(t, err, ErrFacilityPersistence)
			require.False(t, result.OK)
			require.False(t, result.Changed)
			require.Equal(t, domain.FacilityFailurePersistenceFailed, result.Failure)
			require.Nil(t, result.Session)
			require.Equal(t, revisionBefore, fixture.service.Revision())
			require.Empty(t, cmp.Diff(facilityBefore, coordinatorFacilitySnapshot(t, fixture.service)),
				"a result that does not verify must not be installed")
			require.Equal(t, pending.RequestID, fixture.service.Snapshot().PendingCommandExecution.RequestID)
			require.Empty(t, fixture.service.Snapshot().FacilityChanges)

			effects := fixture.effects.Values()[effectsBefore:]
			require.Zero(t, masterEffectCount(effects))
			require.Len(t, fixture.lifecycle.Projections(), projectionsBefore)
			event := requireFacilityAuditEvent(t, effects, test.call.event, test.call.correlationID)
			requireFacilityPlayRejectionAudit(t, test.call, "failed", domain.FacilityFailurePersistenceFailed,
				facilityPlayRevision, event)
		})
	}
}

func TestConcurrentFacilityPlayChangesApplyAtMostOnce(t *testing.T) {
	t.Parallel()

	fixture := newFacilityPlayFixture(t, nil)
	fixture.play.setResult(facilityPlayDurable(t, fixture, func(t *testing.T, facility *domain.Facility) {
		facilityPlayDevice(t, facility, "door").CurrentStateID = "jammed"
	}, []string{"door"}, nil))

	const attempts = 8
	results := make(chan *domain.FacilityOperationResult, attempts)
	var group sync.WaitGroup
	for attempt := range attempts {
		group.Go(func() {
			_, result, _ := fixture.service.ForceFacilityDeviceState(t.Context(), FacilityDeviceForceRequest{
				DeviceID: "door", StateID: "jammed", ExpectedFacilityRevision: facilityPlayRevision,
				CorrelationID: fmt.Sprintf("concurrent-force-%d", attempt),
			})
			results <- result
		})
	}
	group.Wait()
	close(results)

	var applied, stale int
	for result := range results {
		require.NotNil(t, result)
		switch {
		case result.OK && result.Changed:
			applied++
		case result.Failure == domain.FacilityFailureStaleRevision:
			stale++
		default:
			t.Fatalf("unexpected concurrent play outcome: %#v", result)
		}
	}
	require.Equal(t, 1, applied, "two play changes from one revision must never both apply")
	require.Equal(t, attempts-1, stale)
	require.Len(t, fixture.play.Requests(), 1, "a change against an older revision must not reach the store")
	require.Equal(t, facilityPlayRevision+1, coordinatorFacilitySnapshot(t, fixture.service).Revision)
	require.Len(t, fixture.service.Snapshot().FacilityChanges, 1)
}

func TestFacilityPlayChangesReachTheFeedAndTheLogWithoutSecrets(t *testing.T) {
	t.Parallel()

	const (
		passwordCanary   = "PASSWORD-CANARY-ALPHA"
		hackWordCanary   = "HACK-WORD-CANARY"
		hackLogCanary    = "HACK-LOG-CANARY"
		credentialCanary = "CREDENTIAL-CANARY"
	)
	fixture := newFacilityPlayFixture(t, nil)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		for _, terminal := range runtime.Broadcast.TerminalRuntimes {
			terminal.IntroText = "LOGIN " + credentialCanary
			terminal.Hack.SecretWord = passwordCanary
			terminal.Hack.WordsByID = map[string]domain.HackCandidate{
				"candidate-secret": {Text: passwordCanary},
				"candidate-wrong":  {Text: hackWordCanary},
			}
			terminal.Hack.Log = []string{hackLogCanary}
		}
		return transition{accepted: true}
	})
	fixture.requirePendingRequest(t, "secrets-pending")
	baseline := fixture.effects.Calls()

	type playStep struct {
		call    facilityPlayCall
		mutate  func(*testing.T, *domain.Facility)
		devices []string
		faults  []string
	}
	changed := []playStep{
		{
			call: facilityPlayTransition("door", "open", 7, "secrets-transition"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "open"
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = true
			},
			devices: []string{"door"}, faults: []string{"door-alarm"},
		},
		{
			call: facilityPlayFault("door-alarm", false, 8, "secrets-fault"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, "door-alarm").CurrentActive = false
			},
			faults: []string{"door-alarm"},
		},
		{
			call: facilityPlayForce("door", "jammed", 9, "secrets-force"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "jammed"
			},
			devices: []string{"door"},
		},
		{
			call: facilityPlayFault(facilityPlayTerminalFaultID, true, 10, "secrets-terminal-fault"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayCondition(t, facility, facilityPlayTerminalFaultID).CurrentActive = true
			},
			faults: []string{facilityPlayTerminalFaultID},
		},
		{
			call: facilityPlayReset("door", 11, "secrets-reset"),
			mutate: func(t *testing.T, facility *domain.Facility) {
				facilityPlayDevice(t, facility, "door").CurrentStateID = "sealed"
			},
			devices: []string{"door"},
		},
	}
	for _, step := range changed {
		fixture.play.setResult(facilityPlayDurable(t, fixture, step.mutate, step.devices, step.faults))
		_, result, err := step.call.run(t.Context(), fixture.service)
		require.NoError(t, err, step.call.correlationID)
		require.True(t, result.Changed, step.call.correlationID)
	}
	rejected := []facilityPlayCall{
		facilityPlayForce("door", "open", 3, "secrets-stale"),
		facilityPlayTransition("door", "open", 12, " "),
	}
	for _, call := range rejected {
		_, result, err := call.run(t.Context(), fixture.service)
		require.Error(t, err)
		require.False(t, result.OK)
	}

	effects := fixture.effects.Values()[baseline:]
	calls := make([]facilityPlayCall, 0, len(changed)+len(rejected))
	for _, step := range changed {
		calls = append(calls, step.call)
	}
	for _, call := range append(calls, rejected...) {
		requireFacilityAuditEvent(t, effects, call.event, call.correlationID)
	}
	feed := fixture.service.Snapshot().FacilityChanges
	require.Empty(t, cmp.Diff([]domain.FacilityChange{
		{DeviceID: "door", Reset: &domain.FacilityChangeReset{FromStateID: "jammed", ToStateID: "sealed"}},
		{DeviceID: "door", Forced: &domain.FacilityChangeForced{FromStateID: "open", ToStateID: "jammed"}},
		{DeviceID: "door", Fault: &domain.FacilityChangeFault{ConditionID: "door-alarm", Active: false}},
		facilityPlayWithdrawn("door", fixture.terminalID, fixture.commandID),
		{DeviceID: "door", Transition: &domain.FacilityChangeTransition{
			TransitionID: "open", FromStateID: "sealed", ToStateID: "open",
		}},
	}, facilityPlayFeed(feed), cmpopts.EquateEmpty()), "every changed play change on a device is in its feed")
	requireFacilityPlayFeedOrder(t, feed)

	encodedLog, err := json.Marshal(auditEvents(effects))
	require.NoError(t, err)
	encodedFeed, err := json.Marshal(feed)
	require.NoError(t, err)
	for _, forbidden := range []string{
		passwordCanary, hackWordCanary, hackLogCanary, credentialCanary,
		"Vault door", "Open door", "Power grid", "Door alarm", "Ventilation", "Vent clogged", "Terminal flicker",
		"Jammed", "Sealed", "Open doors", "Doors open", "Doors opened", "Open the doors?",
	} {
		require.NotContains(t, string(encodedLog), forbidden, "the retained log must carry IDs only")
		require.NotContains(t, string(encodedFeed), forbidden, "the feed must carry IDs only")
	}
}

// facilityPlayCall is one Overseer play change sent to the coordinator, with
// the audit identity and requested target its record must carry.
type facilityPlayCall struct {
	event             string
	action            FacilityAuditAction
	correlationID     string
	request           any
	auditDeviceIDs    []string
	auditConditionIDs []string
	transitionID      string
	targetStateID     string
	targetActive      *bool
	run               func(context.Context, *Service) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error)

	// targetID, valueID, and revision are the request's inputs; build sends
	// the same kind of play change with other inputs.
	targetID string
	valueID  string
	revision uint64
	build    func(targetID, valueID string, revision uint64, correlationID string) facilityPlayCall
}

func facilityPlayTransition(deviceID, transitionID string, revision uint64, correlationID string) facilityPlayCall {
	request := FacilityDeviceTransitionRequest{
		DeviceID: deviceID, TransitionID: transitionID,
		ExpectedFacilityRevision: revision, CorrelationID: correlationID,
	}
	return facilityPlayCall{
		event: "facility.overseer_transition", action: FacilityAuditActionTransition,
		correlationID: correlationID, request: request,
		auditDeviceIDs: []string{deviceID}, transitionID: transitionID,
		run: func(ctx context.Context, service *Service) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
			return service.ApplyFacilityDeviceTransition(ctx, request)
		},
		targetID: deviceID, valueID: transitionID, revision: revision, build: facilityPlayTransition,
	}
}

func facilityPlayForce(deviceID, stateID string, revision uint64, correlationID string) facilityPlayCall {
	request := FacilityDeviceForceRequest{
		DeviceID: deviceID, StateID: stateID,
		ExpectedFacilityRevision: revision, CorrelationID: correlationID,
	}
	return facilityPlayCall{
		event: "facility.force", action: FacilityAuditActionForce,
		correlationID: correlationID, request: request,
		auditDeviceIDs: []string{deviceID}, targetStateID: stateID,
		run: func(ctx context.Context, service *Service) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
			return service.ForceFacilityDeviceState(ctx, request)
		},
		targetID: deviceID, valueID: stateID, revision: revision, build: facilityPlayForce,
	}
}

// facilityPlayFault switches one fault. Its record names the fault and, for
// a device-scoped fault of the play fixture, that device.
func facilityPlayFault(conditionID string, active bool, revision uint64, correlationID string) facilityPlayCall {
	request := FacilityConditionActiveRequest{
		ConditionID: conditionID, Active: active,
		ExpectedFacilityRevision: revision, CorrelationID: correlationID,
	}
	var deviceIDs []string
	switch conditionID {
	case "door-alarm":
		deviceIDs = []string{"door"}
	case "vent-clog":
		deviceIDs = []string{"vent"}
	}
	return facilityPlayCall{
		event: "facility.fault", action: FacilityAuditActionFault,
		correlationID: correlationID, request: request,
		auditDeviceIDs: deviceIDs, auditConditionIDs: []string{conditionID}, targetActive: new(active),
		run: func(ctx context.Context, service *Service) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
			return service.SetFacilityConditionActive(ctx, request)
		},
		targetID: conditionID, revision: revision,
		build: func(targetID, _ string, revision uint64, correlationID string) facilityPlayCall {
			return facilityPlayFault(targetID, active, revision, correlationID)
		},
	}
}

func facilityPlayReset(deviceID string, revision uint64, correlationID string) facilityPlayCall {
	request := FacilityDeviceResetRequest{
		DeviceID: deviceID, ExpectedFacilityRevision: revision, CorrelationID: correlationID,
	}
	return facilityPlayCall{
		event: facilityAuditEventReset, action: FacilityAuditActionReset,
		correlationID: correlationID, request: request, auditDeviceIDs: []string{deviceID},
		run: func(ctx context.Context, service *Service) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error) {
			return service.ResetFacilityDevice(ctx, request)
		},
		targetID: deviceID, revision: revision,
		build: func(targetID, _ string, revision uint64, correlationID string) facilityPlayCall {
			return facilityPlayReset(targetID, revision, correlationID)
		},
	}
}

func (call facilityPlayCall) withRevision(revision uint64) facilityPlayCall {
	return call.build(call.targetID, call.valueID, revision, call.correlationID)
}

func (call facilityPlayCall) withCorrelationID(correlationID string) facilityPlayCall {
	return call.build(call.targetID, call.valueID, call.revision, correlationID)
}

func (call facilityPlayCall) withTargetID(targetID string) facilityPlayCall {
	return call.build(targetID, call.valueID, call.revision, call.correlationID)
}

// facilityPlayAudit is the part of a play-change record the tests pin.
type facilityPlayAudit struct {
	Outcome                   string
	Action                    FacilityAuditAction
	DeviceIDs                 []string
	ConditionIDs              []string
	DeviceTransitions         []FacilityDeviceAuditTransition
	ConditionTransitions      []FacilityConditionAuditTransition
	Failure                   domain.FacilityFailureCode
	PreviousFacilityRevision  uint64
	ResultingFacilityRevision uint64
	TransitionID              string
	TargetStateID             string
	TargetActive              *bool
}

// requireFacilityPlayAudit checks a play-change record against want. The
// action and the requested target always come from the call.
func requireFacilityPlayAudit(t *testing.T, call facilityPlayCall, want facilityPlayAudit, event AuditEvent) {
	t.Helper()
	require.Equal(t, want.Outcome, event.Outcome)
	require.NotNil(t, event.Facility)
	facts := event.Facility
	want.Action = call.action
	want.TransitionID = call.transitionID
	want.TargetStateID = call.targetStateID
	want.TargetActive = call.targetActive
	got := facilityPlayAudit{
		Outcome: facts.Outcome, Action: facts.Action,
		DeviceIDs: facts.DeviceIDs, ConditionIDs: facts.ConditionIDs,
		DeviceTransitions: facts.DeviceTransitions, ConditionTransitions: facts.ConditionTransitions,
		Failure:                  facts.Failure,
		PreviousFacilityRevision: facts.PreviousFacilityRevision, ResultingFacilityRevision: facts.ResultingFacilityRevision,
		TransitionID: facts.TransitionID, TargetStateID: facts.TargetStateID, TargetActive: facts.TargetActive,
	}
	require.Empty(t, cmp.Diff(want, got, cmpopts.EquateEmpty()), "play-change audit record")
}

// requireFacilityPlayRejectionAudit checks the record of a rejected play
// change: its outcome, failure, and unchanged revision, the requested target,
// and no before-and-after values.
func requireFacilityPlayRejectionAudit(
	t *testing.T,
	call facilityPlayCall,
	outcome string,
	failure domain.FacilityFailureCode,
	revision uint64,
	event AuditEvent,
) {
	t.Helper()
	require.Equal(t, outcome, event.Outcome)
	require.NotNil(t, event.Facility)
	facts := event.Facility
	require.Equal(t, outcome, facts.Outcome)
	require.Equal(t, call.action, facts.Action)
	require.Equal(t, failure, facts.Failure)
	require.Equal(t, revision, facts.PreviousFacilityRevision)
	require.Equal(t, revision, facts.ResultingFacilityRevision)
	require.Empty(t, facts.DeviceTransitions, "a rejected play change records no before-and-after values")
	require.Empty(t, facts.ConditionTransitions, "a rejected play change records no before-and-after values")
	require.Equal(t, call.transitionID, facts.TransitionID)
	require.Equal(t, call.targetStateID, facts.TargetStateID)
	require.Equal(t, call.targetActive, facts.TargetActive)
}

type facilityPlayFixture struct {
	facilityCommandFixture
	play      *recordingFacilityPlayStore
	lifecycle *facilityPlayProjectionLifecycle
}

// newFacilityPlayFixture installs facilityPlayState, after prepare edits it,
// with the play store and a lifecycle that records every reprojection.
func newFacilityPlayFixture(t *testing.T, prepare func(*testing.T, *domain.Facility)) facilityPlayFixture {
	t.Helper()
	base := newFacilityCommandFixture(t, &recordingFacilityStore{})
	play := &recordingFacilityPlayStore{recordingFacilityStore: base.store}
	lifecycle := &facilityPlayProjectionLifecycle{}
	base.service.facilityStore = play
	base.service.terminals = lifecycle
	facility := facilityPlayState()
	if prepare != nil {
		prepare(t, facility)
	}
	require.NoError(t, domain.ValidateSession(facilityAuthoringSession(facility)), "play fixture session")
	base.initialFacility = domain.CloneFacility(facility)
	base.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(facility)
		suspended := testTerminalRuntime(facilityPlaySuspendedTerminalID)
		suspended.Lifecycle = domain.TerminalLifecycleSuspended
		runtime.Broadcast.TerminalRuntimes[facilityPlaySuspendedTerminalID] = suspended
		return transition{accepted: true}
	})
	return facilityPlayFixture{facilityCommandFixture: base, play: play, lifecycle: lifecycle}
}

// requirePendingRequest selects the fixture's device command so that its
// request waits for the Overseer.
func (fixture facilityPlayFixture) requirePendingRequest(t *testing.T, requestID domain.RequestID) *domain.MasterPendingCommandExecution {
	t.Helper()
	selected := fixture.service.DispatchPlayerAction(fixture.controllerConnection, fixture.command(requestID))
	require.True(t, selected.Accepted, "select the device command: %#v", selected)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	return pending
}

// facilityPlayState is the play fixture's facility at revision 7: the power
// grid online, the door sealed with a third state no transition reaches, the
// ventilation idle, and every fault off.
func facilityPlayState() *domain.Facility {
	facility := facilityApprovalState(false)
	door := &facility.Devices[1]
	door.States = append(door.States, domain.FacilityDeviceState{ID: "jammed", Name: "Jammed"})
	facility.Devices = append(facility.Devices, domain.FacilityDevice{
		ID: "vent", Name: "Ventilation", Kind: domain.FacilityDeviceKindVentilation,
		InitialStateID: "idle", CurrentStateID: "idle",
		States: []domain.FacilityDeviceState{{ID: "idle", Name: "Idle"}, {ID: "running", Name: "Running"}},
		Transitions: []domain.FacilityDeviceTransition{{
			ID: "start", Name: "Start ventilation", SourceStateID: "idle", DestinationStateID: "running",
		}},
	})
	facility.Conditions = append(facility.Conditions,
		domain.DiagnosticCondition{
			ID: "vent-clog", Name: "Vent clogged", Category: domain.DiagnosticConditionCategoryOffline,
			Device:  &domain.DiagnosticDeviceScope{DeviceID: "vent"},
			Effects: []domain.DiagnosticEffect{{DisplayInstability: &domain.DisplayInstabilityEffect{}}},
		},
		domain.DiagnosticCondition{
			ID: facilityPlayTerminalFaultID, Name: "Terminal flicker", Category: domain.DiagnosticConditionCategoryOffline,
			Terminal: &domain.DiagnosticTerminalScope{TerminalID: "terminal-1"},
			Effects:  []domain.DiagnosticEffect{{DisplayInstability: &domain.DisplayInstabilityEffect{}}},
			Recovery: []domain.DiagnosticRecoveryReference{{PrivateOverseerAction: new(true)}},
		},
	)
	return facility
}

// facilityPlayDurable returns the durable result of one accepted play change
// on the installed facility: mutate applied, the revision raised by one, and
// the given affected IDs.
func facilityPlayDurable(
	t *testing.T,
	fixture facilityPlayFixture,
	mutate func(*testing.T, *domain.Facility),
	deviceIDs []string,
	conditionIDs []string,
) domain.FacilityOperationResult {
	t.Helper()
	before := coordinatorFacilitySnapshot(t, fixture.service)
	after := domain.CloneFacility(before)
	mutate(t, after)
	after.Revision++
	session := facilityAuthoringSession(after)
	return domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: 70 + after.Revision,
		PreviousFacilityRevision: before.Revision, ResultingFacilityRevision: after.Revision,
		AffectedDeviceIDs: deviceIDs, AffectedConditionIDs: conditionIDs, Session: &session,
	}
}

// facilityPlayUnchanged returns the durable result of a play change that
// found nothing to change.
func facilityPlayUnchanged(t *testing.T, fixture facilityPlayFixture) domain.FacilityOperationResult {
	t.Helper()
	current := coordinatorFacilitySnapshot(t, fixture.service)
	session := facilityAuthoringSession(current)
	return domain.FacilityOperationResult{
		OK: true, SessionRevision: 70 + current.Revision,
		PreviousFacilityRevision: current.Revision, ResultingFacilityRevision: current.Revision,
		Session: &session,
	}
}

func facilityPlayDevice(t *testing.T, facility *domain.Facility, deviceID string) *domain.FacilityDevice {
	t.Helper()
	for index := range facility.Devices {
		if facility.Devices[index].ID == deviceID {
			return &facility.Devices[index]
		}
	}
	t.Fatalf("facility device %q not found", deviceID)
	return nil
}

func facilityPlayCondition(t *testing.T, facility *domain.Facility, conditionID string) *domain.DiagnosticCondition {
	t.Helper()
	condition := facilityCondition(facility, conditionID)
	require.NotNil(t, condition, "facility condition %q not found", conditionID)
	return condition
}

func facilityPlayWithdrawn(deviceID, terminalID, commandID string) domain.FacilityChange {
	return domain.FacilityChange{DeviceID: deviceID, PlayerRefusal: &domain.FacilityChangePlayerRefusal{
		TerminalID: terminalID, CommandID: commandID, Reason: domain.FacilityChangeRefusalWithdrawn,
	}}
}

// facilityPlayFeed returns the feed without its sequence numbers and times,
// which requireFacilityPlayFeedOrder and requireFacilityPlayOneCommitTime
// check on their own.
func facilityPlayFeed(changes []domain.FacilityChange) []domain.FacilityChange {
	feed := domain.CloneFacilityChanges(changes)
	for index := range feed {
		feed[index].Sequence = 0
		feed[index].OccurredAtUnixMs = 0
	}
	return feed
}

// requireFacilityPlayFeedOrder checks that the feed is newest first with
// unique sequence numbers, and that every entry has a time.
func requireFacilityPlayFeedOrder(t *testing.T, changes []domain.FacilityChange) {
	t.Helper()
	for index, change := range changes {
		require.NotZero(t, change.Sequence, "feed entry %d has no sequence", index)
		require.NotZero(t, change.OccurredAtUnixMs, "feed entry %d has no time", index)
		if index > 0 {
			require.Less(t, change.Sequence, changes[index-1].Sequence, "the feed must be newest first")
		}
	}
}

// requireFacilityPlayOneCommitTime checks that the entries of one commit
// share one time.
func requireFacilityPlayOneCommitTime(t *testing.T, changes []domain.FacilityChange) {
	t.Helper()
	for _, change := range changes {
		require.Equal(t, changes[0].OccurredAtUnixMs, change.OccurredAtUnixMs, "entries of one commit share one time")
	}
}

// facilityPlayMasterFeed returns the feed of the one master snapshot among
// effects.
func facilityPlayMasterFeed(t *testing.T, effects []Effect) []domain.FacilityChange {
	t.Helper()
	var feeds [][]domain.FacilityChange
	for _, effect := range effects {
		if effect.Master != nil {
			feeds = append(feeds, effect.Master.FacilityChanges)
		}
	}
	require.Len(t, feeds, 1)
	return feeds[0]
}

func facilityPlayLiveEffects(effects []Effect) []*domain.PublicLiveState {
	var live []*domain.PublicLiveState
	for _, effect := range effects {
		if effect.Live != nil {
			live = append(live, effect.Live)
		}
	}
	return live
}

func facilityPlayRequestRecords(effects []Effect, requestID string) []AuditEvent {
	var records []AuditEvent
	for _, event := range auditEvents(effects) {
		if event.RequestID == requestID {
			records = append(records, event)
		}
	}
	return records
}

// recordingFacilityPlayStore adds the play operations to the fixture's
// facility store. Every play operation, device reset included, returns the
// result set with setResult under the caller's correlation ID.
type recordingFacilityPlayStore struct {
	*recordingFacilityStore
	playMu     sync.Mutex
	playResult domain.FacilityOperationResult
	requests   []any
}

var (
	_ FacilityPlayStore  = (*recordingFacilityPlayStore)(nil)
	_ FacilityResetStore = (*recordingFacilityPlayStore)(nil)
)

func (store *recordingFacilityPlayStore) ApplyFacilityDeviceTransition(
	_ context.Context,
	request FacilityDeviceTransitionRequest,
) domain.FacilityOperationResult {
	return store.record(request, request.CorrelationID)
}

func (store *recordingFacilityPlayStore) ForceFacilityDeviceState(
	_ context.Context,
	request FacilityDeviceForceRequest,
) domain.FacilityOperationResult {
	return store.record(request, request.CorrelationID)
}

func (store *recordingFacilityPlayStore) SetFacilityConditionActive(
	_ context.Context,
	request FacilityConditionActiveRequest,
) domain.FacilityOperationResult {
	return store.record(request, request.CorrelationID)
}

func (store *recordingFacilityPlayStore) ResetFacilityDevice(
	_ context.Context,
	request FacilityDeviceResetRequest,
) domain.FacilityOperationResult {
	return store.record(request, request.CorrelationID)
}

func (store *recordingFacilityPlayStore) record(request any, correlationID string) domain.FacilityOperationResult {
	store.playMu.Lock()
	defer store.playMu.Unlock()
	store.requests = append(store.requests, request)
	result := cloneFacilityOperationResult(store.playResult)
	result.CorrelationID = correlationID
	return result
}

func (store *recordingFacilityPlayStore) setResult(result domain.FacilityOperationResult) {
	store.playMu.Lock()
	defer store.playMu.Unlock()
	store.playResult = cloneFacilityOperationResult(result)
}

// Requests returns every play request the store received, in order.
func (store *recordingFacilityPlayStore) Requests() []any {
	store.playMu.Lock()
	defer store.playMu.Unlock()
	return append([]any(nil), store.requests...)
}

// facilityPlayProjectionLifecycle records the facility revision each
// broadcast terminal is reprojected with.
type facilityPlayProjectionLifecycle struct {
	recordingTerminalLifecycle
	projectionMu sync.Mutex
	projections  []facilityPlayProjection
}

type facilityPlayProjection struct {
	TerminalID       string
	FacilityRevision uint64
}

func (lifecycle *facilityPlayProjectionLifecycle) ProjectFacility(
	runtime *domain.TerminalRuntime,
	facility *domain.Facility,
) *domain.PublicLiveState {
	lifecycle.projectionMu.Lock()
	lifecycle.projections = append(lifecycle.projections, facilityPlayProjection{
		TerminalID: runtime.TerminalID, FacilityRevision: facilityRevision(facility),
	})
	lifecycle.projectionMu.Unlock()
	return lifecycle.recordingTerminalLifecycle.ProjectFacility(runtime, facility)
}

func (lifecycle *facilityPlayProjectionLifecycle) Projections() []facilityPlayProjection {
	lifecycle.projectionMu.Lock()
	defer lifecycle.projectionMu.Unlock()
	return append([]facilityPlayProjection(nil), lifecycle.projections...)
}
