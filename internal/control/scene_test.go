package control

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/require"
)

// The scene fixture extends the facility command fixture. Terminal-1 is the
// active broadcast terminal, terminal-2 is a suspended broadcast terminal,
// and terminal-3 exists only in the durable session.
const (
	sceneDoorCommandID     = "command-open-doors" // device action, never executed
	sceneLogCommandID      = "command-log-entry"  // executed on terminal-1
	sceneRelayCommandID    = "command-call-relay" // never executed, no device action
	sceneFlagCommandID     = "command-raise-flag" // executed on terminal-2
	sceneArchiveCommandID  = "command-seal-archive"
	sceneSecondTerminalID  = "terminal-2"
	sceneArchiveTerminalID = "terminal-3"
	sceneSessionRevision   = uint64(60)
)

func TestResetSceneInstallsTheStartingSceneOnEveryBroadcastTerminal(t *testing.T) {
	t.Parallel()

	fixture := newSceneResetFixture(t)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		first := runtime.Broadcast.TerminalRuntimes[fixture.terminalID]
		first.Nav.CommandNodeID = new(sceneLogCommandID)
		second := runtime.Broadcast.TerminalRuntimes[sceneSecondTerminalID]
		second.Nav.CommandNodeID = new(sceneFlagCommandID)
		second.CommandExecution = &domain.CommandExecutionPresentation{
			Phase: domain.CommandExecutionPhaseRejected, CommandID: sceneFlagCommandID,
		}
		return transition{accepted: true}
	})
	fixture.scene.setResult(sceneResetSuccess(fixture))
	before := fixture.service.Snapshot()
	terminalsBefore := canonicalTerminalSlots(t, fixture.service)
	effectsBefore := fixture.effects.Calls()
	request := sceneResetRequest(fixture, "scene-reset")

	state, result, err := fixture.service.ResetScene(t.Context(), request)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	require.Equal(t, sceneResetCommands(), result.ResetCommands)
	require.Equal(t, []SceneResetRequest{request}, fixture.scene.SceneResets())
	require.Equal(t, before.Revision+1, state.Revision)
	require.NotNil(t, state.FacilityRevision)
	require.Equal(t, fixture.current.Revision+1, *state.FacilityRevision)
	require.Empty(t, cmp.Diff(result.Session.Facility, coordinatorFacilitySnapshot(t, fixture.service)),
		"the durable facility must be installed")

	for _, terminal := range []struct {
		terminalID   string
		commandID    string
		authoredName string
	}{
		{terminalID: fixture.terminalID, commandID: sceneLogCommandID, authoredName: "Log the entry"},
		{terminalID: sceneSecondTerminalID, commandID: sceneFlagCommandID, authoredName: "Raise flag"},
	} {
		previous := terminalsBefore[terminal.terminalID]
		after := canonicalTerminal(t, fixture.service, terminal.terminalID)
		require.Emptyf(t, after.CommandStates, "%s keeps executed commands", terminal.terminalID)
		require.Nilf(t, after.Nav.CommandNodeID, "%s keeps a reset command open", terminal.terminalID)
		require.Nilf(t, after.CommandExecution, "%s keeps reset command presentation", terminal.terminalID)
		command := contentNodeByIDForControlTest(after.Tree, terminal.commandID)
		require.NotNil(t, command)
		require.Equalf(t, terminal.authoredName, command.Name, "%s was not reprojected", terminal.terminalID)
		require.Equal(t, previous.Nav.Path, after.Nav.Path)
		require.Equal(t, previous.Lifecycle, after.Lifecycle)
		require.Equal(t, previous.Hack, after.Hack, "scene reset must keep hacking progress")
	}
	require.Equal(t, before.Broadcast.ActiveTerminalID, state.Broadcast.ActiveTerminalID)
	require.Equal(t, sceneReturnRoute(), sceneBroadcastRoute(t, fixture.service), "scene reset must keep the return route")

	effects := fixture.effects.Values()[effectsBefore:]
	require.Equal(t, 1, masterEffectCount(effects))
	live := sceneLiveEffects(effects)
	require.Len(t, live, 1)
	require.Equal(t, fixture.terminalID, live[0].TerminalID)
	published := contentNodeByIDForControlTest(live[0].Tree, sceneLogCommandID)
	require.NotNil(t, published)
	require.Equal(t, "Log the entry", published.Name)

	result.Session.Facility.Devices[1].CurrentStateID = "tampered"
	require.Equal(t, "sealed", facilityDeviceState(t, coordinatorFacilitySnapshot(t, fixture.service), "door"))
}

func TestResetSceneHandlesEachPendingRequestWithOneCorrelatedOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		commandID    string
		cleared      bool
		withdrawn    bool
		presentation *domain.CommandExecutionPresentation
	}{
		{
			name: "device action is invalidated", commandID: sceneDoorCommandID, cleared: true, withdrawn: true,
			presentation: &domain.CommandExecutionPresentation{
				Phase: domain.CommandExecutionPhaseRejected, CommandID: sceneDoorCommandID,
			},
		},
		{name: "request for a reset command is cleared", commandID: sceneLogCommandID, cleared: true},
		{
			name: "other request stays pending", commandID: sceneRelayCommandID,
			presentation: &domain.CommandExecutionPresentation{
				Phase: domain.CommandExecutionPhasePending, CommandID: sceneRelayCommandID,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newSceneResetFixture(t)
			selected := fixture.service.DispatchPlayerAction(
				fixture.controllerConnection,
				fixture.sceneCommand(domain.RequestID("scene-pending-"+test.commandID), test.commandID),
			)
			require.True(t, selected.Accepted)
			pending := fixture.service.Snapshot().PendingCommandExecution
			require.NotNil(t, pending)
			require.Equal(t, test.commandID, pending.CommandID)
			fixture.scene.setResult(sceneResetSuccess(fixture))
			effectsBefore := fixture.effects.Calls()

			_, result, err := fixture.service.ResetScene(t.Context(), sceneResetRequest(fixture, "scene-reset-pending"))

			require.NoError(t, err)
			require.True(t, result.OK)
			terminal := canonicalTerminal(t, fixture.service, fixture.terminalID)
			require.Equal(t, test.presentation, terminal.CommandExecution)
			feed := fixture.service.Snapshot().FacilityChanges
			wantFeed := []domain.FacilityChange{sceneDoorResetEntry()}
			if test.withdrawn {
				wantFeed = slices.Insert(wantFeed, 0, domain.FacilityChange{
					DeviceID: "door", PlayerRefusal: &domain.FacilityChangePlayerRefusal{
						TerminalID: fixture.terminalID, CommandID: test.commandID,
						Reason: domain.FacilityChangeRefusalWithdrawn,
					},
				})
			}
			require.Empty(t, cmp.Diff(wantFeed, feed, sceneFeedContent),
				"only a withdrawn device action adds a refusal entry for each of its devices")
			if test.withdrawn {
				require.Greater(t, feed[0].Sequence, feed[1].Sequence, "the withdrawal follows the reset entry")
				require.Equal(t, feed[1].OccurredAtUnixMs, feed[0].OccurredAtUnixMs, "entries of one commit share one time")
			}
			records := sceneRequestRecords(fixture.effects.Values()[effectsBefore:], pending.RequestID)
			if !test.cleared {
				after := fixture.service.Snapshot().PendingCommandExecution
				require.NotNil(t, after, "a request outside the reset must stay pending")
				require.Equal(t, pending.RequestID, after.RequestID)
				require.Empty(t, records, "a request that stays pending gets no outcome record")
				return
			}
			require.Nil(t, fixture.service.Snapshot().PendingCommandExecution)
			require.Len(t, records, 1, "audit events = %#v", records)
			record := records[0]
			require.Equal(t, "command.request_outcome", record.Name)
			require.Equal(t, "superseded", record.Outcome)
			require.Equal(t, fixture.terminalID, record.TerminalID)
			require.Equal(t, test.commandID, record.CommandID)
			require.NotNil(t, record.Facility, "the outcome record must carry revisions")
			require.Equal(t, pending.RequestID, record.Facility.CorrelationID)
			require.Equal(t, fixture.current.Revision, record.Facility.PreviousFacilityRevision)
			require.Equal(t, result.ResultingFacilityRevision, record.Facility.ResultingFacilityRevision)
			require.Equal(t, sceneSessionRevision, record.Facility.PreviousSessionRevision)
			require.Equal(t, result.SessionRevision, record.Facility.ResultingSessionRevision)
			sceneRequireNoAuthoredText(t, records)

			fixture.scene.setResult(sceneResetUnchanged(*result.Session, result.SessionRevision))
			retryBefore := fixture.effects.Calls()
			_, retry, retryErr := fixture.service.ResetScene(t.Context(), SceneResetRequest{
				ExpectedSessionRevision:  result.SessionRevision,
				ExpectedFacilityRevision: result.ResultingFacilityRevision,
				CorrelationID:            "scene-reset-retry",
			})
			require.NoError(t, retryErr)
			require.True(t, retry.OK)
			require.False(t, retry.Changed)
			require.Empty(t, sceneRequestRecords(fixture.effects.Values()[retryBefore:], pending.RequestID),
				"a retried reset must not record the request again")
			require.Empty(t, cmp.Diff(feed, fixture.service.Snapshot().FacilityChanges),
				"an unchanged reset must not add feed entries")
		})
	}
}

func TestResetSceneAddsOneFeedEntryPerChangedDevice(t *testing.T) {
	t.Parallel()

	fixture := newSceneResetFixture(t)
	addSceneFeedEntities(t, fixture)
	durable := sceneResetSuccess(fixture)
	durable.AffectedConditionIDs = []string{"door-alarm", "terminal-fault", "vent-fault"}
	fixture.scene.setResult(durable)
	effectsBefore := fixture.effects.Calls()

	state, result, err := fixture.service.ResetScene(t.Context(), sceneResetRequest(fixture, "scene-reset-feed"))

	require.NoError(t, err)
	require.True(t, result.OK)
	require.True(t, result.Changed)
	feed := fixture.service.Snapshot().FacilityChanges
	require.Empty(t, cmp.Diff([]domain.FacilityChange{
		{DeviceID: "vent", SceneReset: &domain.FacilityChangeReset{
			FromStateID: "idle", ToStateID: "idle", ConditionIDs: []string{"vent-fault"},
		}},
		sceneDoorResetEntry(),
	}, feed, sceneFeedContent), "the unchanged grid and the terminal fault must add no entry")
	require.Greater(t, feed[0].Sequence, feed[1].Sequence,
		"entries are appended in device ID order and listed newest first")
	require.Equal(t, feed[1].OccurredAtUnixMs, feed[0].OccurredAtUnixMs, "entries of one commit share one time")
	require.Empty(t, cmp.Diff(feed, state.FacilityChanges), "the returned state must carry the feed")
	var published []*domain.MasterCoordinationState
	for _, effect := range fixture.effects.Values()[effectsBefore:] {
		if effect.Master != nil {
			published = append(published, effect.Master)
		}
	}
	require.Len(t, published, 1)
	require.Empty(t, cmp.Diff(feed, published[0].FacilityChanges), "the published snapshot must carry the feed")
}

func TestResetSceneInvalidatedDeviceActionCannotBeApproved(t *testing.T) {
	t.Parallel()

	fixture := newSceneResetFixture(t)
	require.True(t, fixture.service.DispatchPlayerAction(
		fixture.controllerConnection, fixture.sceneCommand("scene-device-action", sceneDoorCommandID),
	).Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	fixture.scene.setResult(sceneResetSuccess(fixture))
	_, reset, resetErr := fixture.service.ResetScene(t.Context(), sceneResetRequest(fixture, "scene-reset-device"))
	require.NoError(t, resetErr)
	require.True(t, reset.OK)

	_, mutation, stale, err := fixture.service.ResolveCommandExecution(
		t.Context(), pending.RequestID, domain.CommandExecutionApprove,
	)

	require.ErrorIs(t, err, ErrFacilityStaleRevision)
	require.Nil(t, mutation)
	require.NotNil(t, stale)
	require.Equal(t, domain.FacilityFailureStaleRevision, stale.Failure)
	require.Zero(t, fixture.store.CallCount(), "an invalidated device action must never reach the world")
}

func TestResetSceneFailureLeavesTheRuntimeUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		request     func(sceneResetFixture) SceneResetRequest
		result      func(sceneResetFixture) domain.FacilityOperationResult
		canceled    bool
		wantFailure domain.FacilityFailureCode
		wantOutcome string
		wantStore   int
	}{
		{
			name: "stale facility revision",
			request: func(fixture sceneResetFixture) SceneResetRequest {
				request := sceneResetRequest(fixture, "scene-reset-stale-facility")
				request.ExpectedFacilityRevision--
				return request
			},
			wantFailure: domain.FacilityFailureStaleRevision, wantOutcome: "stale",
		},
		{
			name: "store reports a stale session",
			result: func(fixture sceneResetFixture) domain.FacilityOperationResult {
				return sceneResetFailure(fixture, domain.FacilityFailureStaleRevision, sceneSessionRevision+2)
			},
			wantFailure: domain.FacilityFailureStaleRevision, wantOutcome: "stale", wantStore: 1,
		},
		{
			name: "write failure",
			result: func(fixture sceneResetFixture) domain.FacilityOperationResult {
				return sceneResetFailure(fixture, domain.FacilityFailurePersistenceFailed, sceneSessionRevision)
			},
			wantFailure: domain.FacilityFailurePersistenceFailed, wantOutcome: "failed", wantStore: 1,
		},
		{
			name: "durable session keeps executed commands",
			result: func(fixture sceneResetFixture) domain.FacilityOperationResult {
				result := sceneResetSuccess(fixture)
				result.Session.Terminals[0].CommandStates = sceneExecutedCommandStates(sceneLogCommandID)
				return result
			},
			wantFailure: domain.FacilityFailurePersistenceFailed, wantOutcome: "failed", wantStore: 1,
		},
		{
			name: "durable facility keeps live values",
			result: func(fixture sceneResetFixture) domain.FacilityOperationResult {
				result := sceneResetSuccess(fixture)
				result.Session.Facility.Devices[1].CurrentStateID = "open"
				return result
			},
			wantFailure: domain.FacilityFailurePersistenceFailed, wantOutcome: "failed", wantStore: 1,
		},
		{
			name: "blank correlation ID",
			request: func(fixture sceneResetFixture) SceneResetRequest {
				return sceneResetRequest(fixture, " ")
			},
			wantFailure: domain.FacilityFailureInvalidConfiguration, wantOutcome: "invalid",
		},
		{
			name: "canceled request", canceled: true,
			wantFailure: domain.FacilityFailureRuntimeContextEnded, wantOutcome: "failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newSceneResetFixture(t)
			require.True(t, fixture.service.DispatchPlayerAction(
				fixture.controllerConnection, fixture.sceneCommand("scene-failure-pending", sceneDoorCommandID),
			).Accepted)
			request := sceneResetRequest(fixture, "scene-reset-failure")
			if test.request != nil {
				request = test.request(fixture)
			}
			if test.result != nil {
				fixture.scene.setResult(test.result(fixture))
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
			effectsBefore := fixture.effects.Calls()

			state, result, err := fixture.service.ResetScene(ctx, request)

			require.ErrorIs(t, err, facilityFailureError(test.wantFailure))
			require.NotNil(t, state)
			require.NotNil(t, result)
			require.False(t, result.OK)
			require.False(t, result.Changed)
			require.Equal(t, test.wantFailure, result.Failure)
			require.Empty(t, result.ResetCommands)
			require.Len(t, fixture.scene.SceneResets(), test.wantStore)
			require.Equal(t, before.Revision, state.Revision)
			require.Empty(t, cmp.Diff(before, fixture.service.Snapshot()), "a failed reset changed coordination state")
			require.Equal(t, terminalsBefore, canonicalTerminalSlots(t, fixture.service), "a failed reset changed a terminal")
			require.Empty(t, cmp.Diff(facilityBefore, coordinatorFacilitySnapshot(t, fixture.service)),
				"a failed reset installed a facility")

			effects := fixture.effects.Values()[effectsBefore:]
			require.Zero(t, masterEffectCount(effects))
			require.Empty(t, sceneLiveEffects(effects))
			audit := requireFacilityAuditEvent(t, effects, facilityAuditEventReset, request.CorrelationID)
			require.Equal(t, test.wantOutcome, audit.Outcome)
			require.Equal(t, "scene", audit.Facility.ResetScope)
			require.Equal(t, test.wantFailure, audit.Facility.Failure)
			require.Equal(t, fixture.current.Revision, audit.Facility.PreviousFacilityRevision)
			require.Equal(t, fixture.current.Revision, audit.Facility.ResultingFacilityRevision)
			require.Len(t, auditEvents(effects), 1, "a failed reset records exactly one outcome")
		})
	}
}

func TestResetSceneRecordsOneCorrelatedAuditWithRevisionsAndCounts(t *testing.T) {
	t.Parallel()

	fixture := newSceneResetFixture(t)
	fixture.scene.setResult(sceneResetSuccess(fixture))
	effectsBefore := fixture.effects.Calls()
	correlationID := "scene-reset-audit"

	_, result, err := fixture.service.ResetScene(t.Context(), sceneResetRequest(fixture, correlationID))

	require.NoError(t, err)
	require.True(t, result.OK)
	effects := fixture.effects.Values()[effectsBefore:]
	audit := requireFacilityAuditEvent(t, effects, facilityAuditEventReset, correlationID)
	require.Equal(t, "succeeded", audit.Outcome)
	facts := audit.Facility
	require.Equal(t, correlationID, facts.CorrelationID)
	require.Equal(t, FacilityAuditActionReset, facts.Action)
	require.Equal(t, "scene", facts.ResetScope)
	require.Equal(t, fixture.current.Revision, facts.PreviousFacilityRevision)
	require.Equal(t, fixture.current.Revision+1, facts.ResultingFacilityRevision)
	require.Equal(t, sceneSessionRevision, facts.PreviousSessionRevision)
	require.Equal(t, sceneSessionRevision+1, facts.ResultingSessionRevision)
	require.Equal(t, []string{"door"}, facts.DeviceIDs)
	require.Equal(t, []string{"door-alarm"}, facts.ConditionIDs)
	require.Equal(t, sceneResetCommands(), facts.ResetCommands)
	require.Equal(t, []FacilityDeviceAuditTransition{{
		DeviceID: "door", PreviousStateID: "open", ResultingStateID: "sealed",
	}}, facts.DeviceTransitions)
	require.Equal(t, []FacilityConditionAuditTransition{{
		ConditionID: "door-alarm", PreviousActive: true, ResultingActive: false,
	}}, facts.ConditionTransitions)
	sceneRequireNoAuthoredText(t, auditEvents(effects))
}

func TestResetSceneOfTheStartingSceneReportsUnchanged(t *testing.T) {
	t.Parallel()

	fixture := newSceneResetFixture(t)
	starting := facilityApprovalState(false)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(starting)
		for _, terminal := range runtime.Broadcast.TerminalRuntimes {
			terminal.CommandStates = nil
		}
		return transition{accepted: true}
	})
	fixture.scene.setResult(sceneResetUnchanged(sceneResetSession(starting, false), sceneSessionRevision))
	before := fixture.service.Snapshot()
	effectsBefore := fixture.effects.Calls()
	correlationID := "scene-reset-unchanged"

	state, result, err := fixture.service.ResetScene(t.Context(), SceneResetRequest{
		ExpectedSessionRevision:  sceneSessionRevision,
		ExpectedFacilityRevision: starting.Revision,
		CorrelationID:            correlationID,
	})

	require.NoError(t, err)
	require.True(t, result.OK)
	require.False(t, result.Changed)
	require.Empty(t, result.ResetCommands)
	require.Equal(t, before.Revision, state.Revision)
	require.Empty(t, fixture.service.Snapshot().FacilityChanges, "an unchanged reset must not add feed entries")
	effects := fixture.effects.Values()[effectsBefore:]
	require.Zero(t, masterEffectCount(effects))
	audit := requireFacilityAuditEvent(t, effects, facilityAuditEventReset, correlationID)
	require.Equal(t, "unchanged", audit.Outcome)
	require.Equal(t, starting.Revision, audit.Facility.PreviousFacilityRevision)
	require.Equal(t, starting.Revision, audit.Facility.ResultingFacilityRevision)
	require.Equal(t, sceneSessionRevision, audit.Facility.PreviousSessionRevision)
	require.Equal(t, sceneSessionRevision, audit.Facility.ResultingSessionRevision)
}

func TestResetSceneDecisionAndPublicationIgnoreAFailingLogSink(t *testing.T) {
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
	run := func(t *testing.T, failingLog bool) outcome {
		t.Helper()
		fixture := newSceneResetFixture(t)
		require.True(t, fixture.service.DispatchPlayerAction(
			fixture.controllerConnection, fixture.sceneCommand("scene-log-sink", sceneDoorCommandID),
		).Accepted)
		fixture.scene.setResult(sceneResetSuccess(fixture))
		delivered := testutil.NewFakeOrderedEffectSink[Effect]()
		fixture.service.enqueue = func(effect Effect) {
			if failingLog && len(effect.Audit) != 0 {
				return // The log sink fails: no audit record is retained.
			}
			delivered.Enqueue(effect)
		}

		state, result, err := fixture.service.ResetScene(t.Context(), sceneResetRequest(fixture, "scene-reset-log-sink"))

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

	working := run(t, false)
	failing := run(t, true)

	require.NoError(t, working.err)
	require.True(t, working.result.OK)
	require.NotZero(t, working.audits)
	require.Zero(t, failing.audits)
	require.Equal(t, working.err, failing.err)
	require.Empty(t, cmp.Diff(working.result, failing.result), "the reset decision depends on logging")
	// The two runs read the clock separately, so feed times may differ.
	feedTime := cmpopts.IgnoreFields(domain.FacilityChange{}, "OccurredAtUnixMs")
	require.Empty(t, cmp.Diff(working.state, failing.state, feedTime))
	require.Empty(t, cmp.Diff(working.snapshot, failing.snapshot, feedTime))
	require.Equal(t, working.terminals, failing.terminals)
	require.NotEmpty(t, working.publication)
	require.Empty(t, cmp.Diff(working.publication, failing.publication, feedTime), "publication depends on logging")
}

type sceneResetFixture struct {
	facilityCommandFixture
	scene   *recordingSceneResetStore
	current *domain.Facility
}

// newSceneResetFixture builds a broadcast whose scene differs from its
// starting values: the door is open, its alarm is active, and terminal-1 and
// terminal-2 each have one executed command. Players returned to terminal-1
// from terminal-2, and terminal-1 keeps a solved hacking state.
func newSceneResetFixture(t *testing.T) sceneResetFixture {
	t.Helper()
	base := newFacilityCommandFixture(t, &recordingFacilityStore{})
	scene := &recordingSceneResetStore{recordingFacilityStore: base.store}
	base.service.facilityStore = scene
	base.service.terminals = &recordingTerminalLifecycle{}
	current := facilityApprovalState(true)
	session := sceneResetSession(current, true)
	base.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(current)
		for _, terminal := range session.Terminals {
			if terminal.ID == sceneArchiveTerminalID {
				continue
			}
			slot := runtime.Broadcast.TerminalRuntimes[terminal.ID]
			if slot == nil {
				slot = testTerminalRuntime(terminal.ID)
				slot.TerminalName = terminal.Name
				slot.Lifecycle = domain.TerminalLifecycleSuspended
				runtime.Broadcast.TerminalRuntimes[terminal.ID] = slot
			}
			slot.AuthoredTree = domain.CloneContentNode(terminal.Root)
			slot.CommandStates = cloneCommandStates(terminal.CommandStates)
			slot.Tree = domain.CloneContentNode(terminal.Root)
			applyRecordedCommandStates(&slot.Tree, slot.CommandStates)
		}
		runtime.Broadcast.Route = sceneReturnRoute()
		return transition{accepted: true}
	})
	return sceneResetFixture{facilityCommandFixture: base, scene: scene, current: current}
}

func (fixture sceneResetFixture) sceneCommand(requestID domain.RequestID, commandID string) domain.RuntimeCommand {
	command := fixture.command(requestID)
	command.NodeID = commandID
	return command
}

// sceneResetSession is the durable session behind the fixture. With executed
// set, each terminal keeps one executed command.
func sceneResetSession(facility *domain.Facility, executed bool) domain.Session {
	command := func(id, name, text, completedName, confirmation string) domain.ContentNode {
		return domain.ContentNode{
			ID: id, Type: domain.NodeCommand, Name: name, Text: text,
			StateChange: &domain.StateChangeConfig{CompletedName: completedName, ConfirmationText: confirmation},
		}
	}
	door := command(sceneDoorCommandID, "Open doors", "Doors opened", "Doors open", "Open the doors?")
	door.StateChange.FacilityAction = &domain.FacilityActionConfig{
		Transitions: &domain.FacilityTransitionList{Transitions: []domain.FacilityTransitionRequest{{
			DeviceID: "door", TransitionID: "open",
		}}},
	}
	terminals := []domain.Terminal{
		{
			ID: "terminal-1", Name: "Overseer", HackLevel: 1,
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{
					{ID: "docs", Type: domain.NodeFolder, Name: "DOCS", Children: []domain.ContentNode{}},
					door,
					command(sceneLogCommandID, "Log the entry", "Entry recorded.", "Entry logged", "Log this entry?"),
					command(sceneRelayCommandID, "Call the relay", "Relay answered.", "Relay called", "Call the relay?"),
				},
			},
		},
		{
			ID: sceneSecondTerminalID, Name: "Relay", HackLevel: 1,
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{
					command(sceneFlagCommandID, "Raise flag", "Flag is up.", "Flag raised", "Raise the flag?"),
				},
			},
		},
		{
			ID: sceneArchiveTerminalID, Name: "Archive", HackLevel: 1,
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{
					command(sceneArchiveCommandID, "Seal archive", "Archive sealed.", "Archive closed", "Seal the archive?"),
				},
			},
		},
	}
	if executed {
		terminals[0].CommandStates = sceneExecutedCommandStates(sceneLogCommandID)
		terminals[1].CommandStates = sceneExecutedCommandStates(sceneFlagCommandID)
		terminals[2].CommandStates = sceneExecutedCommandStates(sceneArchiveCommandID)
	}
	return domain.Session{
		Version: 1, Name: "Scene reset fixture", Terminals: terminals, Facility: domain.CloneFacility(facility),
	}
}

func sceneExecutedCommandStates(commandID string) map[string]domain.CommandExecutionState {
	completed := map[string]domain.CommandExecutionState{
		sceneLogCommandID:     {CompletedName: "Entry logged", ResultText: "Entry recorded."},
		sceneFlagCommandID:    {CompletedName: "Flag raised", ResultText: "Flag is up."},
		sceneArchiveCommandID: {CompletedName: "Archive closed", ResultText: "Archive sealed."},
	}
	return map[string]domain.CommandExecutionState{commandID: completed[commandID]}
}

func sceneResetCommands() []domain.CommandReference {
	return []domain.CommandReference{
		{TerminalID: "terminal-1", CommandID: sceneLogCommandID},
		{TerminalID: sceneSecondTerminalID, CommandID: sceneFlagCommandID},
		{TerminalID: sceneArchiveTerminalID, CommandID: sceneArchiveCommandID},
	}
}

func sceneReturnRoute() []domain.TerminalReturnPoint {
	return []domain.TerminalReturnPoint{{
		TerminalID: sceneSecondTerminalID, TerminalName: "Relay", FolderID: "root",
		CommandID: sceneFlagCommandID, CommandName: "Raise flag",
	}}
}

func sceneResetRequest(fixture sceneResetFixture, correlationID string) SceneResetRequest {
	return SceneResetRequest{
		ExpectedSessionRevision:  sceneSessionRevision,
		ExpectedFacilityRevision: fixture.current.Revision,
		CorrelationID:            correlationID,
	}
}

// sceneResetSuccess is the durable result of resetting the fixture scene: one
// write that restores every device and fault and clears every terminal.
func sceneResetSuccess(fixture sceneResetFixture) domain.FacilityOperationResult {
	starting := domain.CloneFacility(fixture.current)
	starting.Revision++
	for index := range starting.Devices {
		starting.Devices[index].CurrentStateID = starting.Devices[index].InitialStateID
	}
	for index := range starting.Conditions {
		starting.Conditions[index].CurrentActive = starting.Conditions[index].InitialActive
	}
	canonical := sceneResetSession(starting, false)
	return domain.FacilityOperationResult{
		OK: true, Changed: true, SessionRevision: sceneSessionRevision + 1,
		PreviousFacilityRevision: fixture.current.Revision, ResultingFacilityRevision: starting.Revision,
		AffectedDeviceIDs: []string{"door"}, AffectedConditionIDs: []string{"door-alarm"},
		ResetCommands: sceneResetCommands(), Session: &canonical,
	}
}

func sceneResetUnchanged(session domain.Session, sessionRevision uint64) domain.FacilityOperationResult {
	return domain.FacilityOperationResult{
		OK: true, SessionRevision: sessionRevision,
		PreviousFacilityRevision: session.Facility.Revision, ResultingFacilityRevision: session.Facility.Revision,
		Session: &session,
	}
}

func sceneResetFailure(
	fixture sceneResetFixture,
	failure domain.FacilityFailureCode,
	sessionRevision uint64,
) domain.FacilityOperationResult {
	return domain.FacilityOperationResult{
		Failure: failure, SessionRevision: sessionRevision,
		PreviousFacilityRevision: fixture.current.Revision, ResultingFacilityRevision: fixture.current.Revision,
	}
}

// sceneFeedContent compares feed entries by what they record, leaving out the
// sequence and time the run assigns.
var sceneFeedContent = cmpopts.IgnoreFields(domain.FacilityChange{}, "Sequence", "OccurredAtUnixMs")

// sceneDoorResetEntry is the feed entry the fixture's scene reset adds for the
// door: it closes and its alarm clears.
func sceneDoorResetEntry() domain.FacilityChange {
	return domain.FacilityChange{DeviceID: "door", SceneReset: &domain.FacilityChangeReset{
		FromStateID: "open", ToStateID: "sealed", ConditionIDs: []string{"door-alarm"},
	}}
}

// addSceneFeedEntities adds two faults the scene reset clears: one on a
// ventilation unit that stays in its starting state, and one scoped to a
// terminal rather than a device.
func addSceneFeedEntities(t *testing.T, fixture sceneResetFixture) {
	t.Helper()
	displayInstability := []domain.DiagnosticEffect{{DisplayInstability: &domain.DisplayInstabilityEffect{}}}
	fixture.current.Devices = append(fixture.current.Devices, domain.FacilityDevice{
		ID: "vent", Name: "Ventilation", Kind: domain.FacilityDeviceKindVentilation,
		InitialStateID: "idle", CurrentStateID: "idle",
		States: []domain.FacilityDeviceState{{ID: "idle", Name: "Idle"}, {ID: "running", Name: "Running"}},
	})
	fixture.current.Conditions = append(fixture.current.Conditions,
		domain.DiagnosticCondition{
			ID: "vent-fault", Name: "Vent fault", Category: domain.DiagnosticConditionCategoryCustom,
			CustomCategory: "vent-fault", Device: &domain.DiagnosticDeviceScope{DeviceID: "vent"},
			CurrentActive: true, Effects: displayInstability,
		},
		domain.DiagnosticCondition{
			ID: "terminal-fault", Name: "Terminal fault", Category: domain.DiagnosticConditionCategoryDisplayUnstable,
			Terminal:      &domain.DiagnosticTerminalScope{TerminalID: fixture.terminalID},
			CurrentActive: true, Effects: displayInstability,
		},
	)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Facility = domain.CloneFacility(fixture.current)
		return transition{accepted: true}
	})
}

func sceneBroadcastRoute(t *testing.T, service *Service) []domain.TerminalReturnPoint {
	t.Helper()
	service.mu.RLock()
	defer service.mu.RUnlock()
	require.NotNil(t, service.runtime.Broadcast)
	return slices.Clone(service.runtime.Broadcast.Route)
}

func sceneLiveEffects(effects []Effect) []*domain.PublicLiveState {
	var live []*domain.PublicLiveState
	for _, effect := range effects {
		if effect.Live != nil {
			live = append(live, effect.Live)
		}
	}
	return live
}

func sceneRequestRecords(effects []Effect, requestID string) []AuditEvent {
	var records []AuditEvent
	for _, event := range auditEvents(effects) {
		if event.RequestID == requestID {
			records = append(records, event)
		}
	}
	return records
}

func sceneRequireNoAuthoredText(t *testing.T, events []AuditEvent) {
	t.Helper()
	encoded, err := json.Marshal(events)
	require.NoError(t, err)
	for _, authored := range []string{
		"Open doors", "Doors open", "Open the doors?", "Log the entry", "Entry logged", "Entry recorded.",
		"Call the relay", "Raise flag", "Flag raised", "Seal archive", "Archive closed",
		"Vault door", "Power grid", "Door alarm",
	} {
		require.NotContains(t, string(encoded), authored)
	}
}

// recordingSceneResetStore is the fixture's facility store with the scene
// reset capability added.
type recordingSceneResetStore struct {
	*recordingFacilityStore
	resetMu     sync.Mutex
	resetResult domain.FacilityOperationResult
	resets      []SceneResetRequest
}

func (store *recordingSceneResetStore) ResetScene(
	_ context.Context,
	request SceneResetRequest,
) domain.FacilityOperationResult {
	store.resetMu.Lock()
	defer store.resetMu.Unlock()
	store.resets = append(store.resets, request)
	result := cloneFacilityOperationResult(store.resetResult)
	result.ResetCommands = slices.Clone(store.resetResult.ResetCommands)
	result.CorrelationID = request.CorrelationID
	return result
}

func (store *recordingSceneResetStore) setResult(result domain.FacilityOperationResult) {
	store.resetMu.Lock()
	defer store.resetMu.Unlock()
	store.resetResult = result
}

func (store *recordingSceneResetStore) SceneResets() []SceneResetRequest {
	store.resetMu.Lock()
	defer store.resetMu.Unlock()
	return slices.Clone(store.resets)
}
