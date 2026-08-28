package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/hack"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/live"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/nav"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControllerPresentationCommandCarriesSemanticContext(t *testing.T) {
	commandType := reflect.TypeFor[domain.RuntimeCommand]()
	field, ok := commandType.FieldByName("Presentation")
	require.True(t, ok, "RuntimeCommand must expose Presentation")
	require.NotEqual(t, reflect.Invalid, field.Type.Kind())
	_, ok = field.Type.FieldByName("ContextKey")
	require.True(t, ok, "presentation command must carry the semantic context precondition")

	runtimeType := reflect.TypeFor[domain.TerminalRuntime]()
	_, ok = runtimeType.FieldByName("Presentation")
	require.True(t, ok, "coordinator-owned runtime must retain presentation across reassignment")
}

func TestControllerPresentationAuthorizationReassignmentAndProjection(t *testing.T) {
	engine := live.New(nil, nil)
	fixture := newUS2Fixture(t, engine)
	projection, _, ok := fixture.service.CurrentLiveForSession(fixture.controllerSession)
	require.True(t, ok)
	require.NotEmpty(t, projection.Presentation.ContextKey)

	first := domain.RuntimeCommand{
		RequestID: "presentation-controller", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandPresentation,
		Presentation: domain.ControllerTerminalPresentation{
			Kind: domain.ControllerTerminalPresentationHacking, ContextKey: projection.Presentation.ContextKey,
			TargetID: "candidate-wrong",
		},
	}
	result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, first)
	require.True(t, result.Accepted)
	require.Equal(t, "candidate-wrong", canonicalTerminal(t, fixture.service, fixture.terminalID).Presentation.TargetID)

	beforeObserver := fixture.service.Revision()
	observer := first
	observer.RequestID = "presentation-observer"
	observer.Presentation.TargetID = "candidate-secret"
	result = fixture.service.DispatchPlayerAction(fixture.observerConnection, observer)
	require.False(t, result.Accepted)
	require.Equal(t, domain.ActionReasonNotController, result.Reason)
	require.Equal(t, beforeObserver, fixture.service.Revision())
	require.Equal(t, "candidate-wrong", canonicalTerminal(t, fixture.service, fixture.terminalID).Presentation.TargetID)

	_, err := fixture.service.SetActiveController(fixture.observerSession)
	require.NoError(t, err)
	require.Equal(t, "candidate-wrong", canonicalTerminal(t, fixture.service, fixture.terminalID).Presentation.TargetID,
		"reassignment must preserve the shared presentation")

	former := first
	former.RequestID = "presentation-former-controller"
	former.Presentation.TargetID = "candidate-secret"
	result = fixture.service.DispatchPlayerAction(fixture.controllerConnection, former)
	require.False(t, result.Accepted)
	require.Equal(t, domain.ActionReasonNotController, result.Reason)

	newController := observer
	newController.RequestID = "presentation-new-controller"
	result = fixture.service.DispatchPlayerAction(fixture.observerConnection, newController)
	require.True(t, result.Accepted)
	require.Equal(t, "candidate-secret", canonicalTerminal(t, fixture.service, fixture.terminalID).Presentation.TargetID)
}

func TestControllerPresentationAndReassignmentShareCommitOrderAcross100Interleavings(t *testing.T) {
	for trial := range 100 {
		t.Run(fmt.Sprintf("trial-%03d", trial), func(t *testing.T) {
			fixture := newUS2Fixture(t, live.New(nil, nil))
			projection, _, ok := fixture.service.CurrentLiveForSession(fixture.controllerSession)
			require.True(t, ok)
			before := projection.Presentation
			require.NotEmpty(t, before.ContextKey)

			command := domain.RuntimeCommand{
				RequestID:   domain.RequestID(fmt.Sprintf("presentation-reassign-race-%03d", trial)),
				BroadcastID: fixture.broadcastID,
				TerminalID:  fixture.terminalID,
				Kind:        domain.RuntimeCommandPresentation,
				Presentation: domain.ControllerTerminalPresentation{
					Kind:       domain.ControllerTerminalPresentationHacking,
					ContextKey: before.ContextKey,
					TargetID:   "candidate-wrong",
				},
			}

			start := make(chan struct{})
			actionDone := make(chan domain.ActionResult, 1)
			reassignmentDone := make(chan error, 1)
			go func() {
				<-start
				actionDone <- fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
			}()
			go func() {
				<-start
				_, err := fixture.service.SetActiveController(fixture.observerSession)
				reassignmentDone <- err
			}()
			close(start)

			result := <-actionDone
			require.NoError(t, <-reassignmentDone)
			final := fixture.service.Snapshot()
			assertExactlyOneController(t, final, fixture.observerSession)
			presentation := canonicalTerminal(t, fixture.service, fixture.terminalID).Presentation
			if result.Accepted {
				require.Equal(t, domain.ActionReasonAccepted, result.Reason)
				require.Equal(t, domain.ControllerTerminalPresentationHacking, presentation.Kind)
				require.Equal(t, before.ContextKey, presentation.ContextKey)
				require.Equal(t, "candidate-wrong", presentation.TargetID)
			} else {
				require.Equal(t, domain.ActionReasonNotController, result.Reason)
				require.Equal(t, before, presentation)
			}
		})
	}
}

// These tests intentionally exercise the package-private transaction seam.
// Story commands build on this seam, while commit remains the single place
// that assigns revisions, detaches effects, and orders their publication.

func TestAttachSubscriptionCreatesCompleteSnapshotAndSelectionCommitsOnce(t *testing.T) {
	ids := testutil.NewFakeOpaqueIDSource("broadcast-1", "session-1", "recognition-1")
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: ids, Enqueue: effects.Enqueue})
	_, err := service.InstallPlayerConfig(domain.PlayerConfigHandle{Path: "/private/players.json", Version: 1, Name: "Vault 33"}, []domain.CharacterRosterEntry{{ID: "character-1", Name: "Lucy", Intelligence: 1}})
	require.NoError(t, err)
	_, err = service.StartBroadcast()
	require.NoError(t, err)

	snapshot, err := service.AttachSubscription("stream-1", nil)
	require.NoError(t, err)
	require.Equal(t, domain.RecognitionHandle("recognition-1"), snapshot.RecognitionHandle)
	require.Equal(t, domain.LogicalSessionID("session-1"), snapshot.PlayerState.SessionID)
	require.True(t, snapshot.Terminal.NoLiveTerminal)
	require.Nil(t, snapshot.Terminal.Live)
	require.Equal(t, snapshot.Revision, snapshot.PlayerState.Revision)
	require.Equal(t, 1, service.ActiveStreamCount())

	beforeSelection := service.Revision()
	result := service.SelectCharacterForRecognition(snapshot.RecognitionHandle, domain.RequestID("request-1"), domain.BroadcastID("broadcast-1"), domain.CharacterID("character-1"))
	require.True(t, result.Accepted)
	require.Equal(t, domain.ActionReasonAccepted, result.Reason)
	require.Equal(t, beforeSelection+1, result.Revision)
	require.Equal(t, result.Revision, service.Revision())
}

func TestAttachSubscriptionRegistrationIsOrderedBeforeConcurrentMutation(t *testing.T) {
	ids := testutil.NewFakeOpaqueIDSource("session-1", "recognition-1", "character-1")
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: ids, Enqueue: effects.Enqueue})

	registrationEntered := make(chan struct{})
	releaseRegistration := make(chan struct{})
	attachDone := make(chan *domain.PersonalizedSnapshot, 1)
	go func() {
		snapshot, err := service.AttachSubscriptionAndRegister("stream-1", nil, func(registered *domain.PersonalizedSnapshot) {
			close(registrationEntered)
			<-releaseRegistration
			require.NotNil(t, registered)
		})
		require.NoError(t, err)
		attachDone <- snapshot
	}()

	<-registrationEntered
	mutationDone := make(chan struct{})
	go func() {
		_, err := addCharacter(service, "Lucy")
		require.NoError(t, err)
		close(mutationDone)
	}()
	select {
	case <-mutationDone:
		assert.FailNow(t, "mutation crossed the subscription registration boundary")
	case <-time.After(20 * time.Millisecond):
	}

	close(releaseRegistration)
	snapshot := <-attachDone
	<-mutationDone
	require.Equal(t, uint64(1), snapshot.Revision)
	require.Equal(t, uint64(2), service.Revision())

	var revisionTwoUpdates int
	for _, effect := range effects.Values() {
		if effect.Update != nil && effect.Update.Revision == 2 && effect.SessionID == snapshot.PlayerState.SessionID {
			revisionTwoUpdates++
		}
	}
	require.Equal(t, 1, revisionTwoUpdates, "the first post-snapshot revision must be offered exactly once")
}

func TestAcceptedMutationPublishesOnePreassembledUpdatePerSessionBeforeReturn(t *testing.T) {
	ids := testutil.NewFakeOpaqueIDSource("session-1", "recognition-1", "session-2", "recognition-2", "character-1")
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: ids, Enqueue: effects.Enqueue})
	first, err := service.AttachSubscription("stream-1", nil)
	require.NoError(t, err)
	second, err := service.AttachSubscription("stream-2", nil)
	require.NoError(t, err)
	baseline := effects.Calls()

	state, err := addCharacter(service, "Lucy")
	require.NoError(t, err)
	require.Equal(t, service.Revision(), state.Revision)

	updates := make(map[domain.LogicalSessionID]int)
	for _, effect := range effects.Values()[baseline:] {
		if effect.Update == nil {
			continue
		}
		require.Equal(t, state.Revision, effect.Update.Revision)
		updates[effect.SessionID]++
	}
	require.Equal(t, map[domain.LogicalSessionID]int{
		first.PlayerState.SessionID:  1,
		second.PlayerState.SessionID: 1,
	}, updates)
}

func TestUnassignedSubscriptionCannotMutateSharedTerminalState(t *testing.T) {
	ids := testutil.NewFakeOpaqueIDSource("broadcast-1", "session-1", "recognition-1")
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: ids, Enqueue: effects.Enqueue})
	_, err := service.InstallPlayerConfig(domain.PlayerConfigHandle{Path: "/private/players.json", Version: 1, Name: "Vault 33"}, []domain.CharacterRosterEntry{{ID: "character-1", Name: "Lucy", Intelligence: 1}})
	require.NoError(t, err)
	_, err = service.StartBroadcast()
	require.NoError(t, err)
	snapshot, err := service.AttachSubscription("stream-1", nil)
	require.NoError(t, err)

	before := service.Revision()
	service.DispatchPlayerAction("stream-1", domain.RuntimeCommand{
		RequestID: "request-unauthorized", BroadcastID: "broadcast-1", TerminalID: "terminal-1",
		Kind: domain.RuntimeCommandNavigate, Action: "back",
	})
	require.Equal(t, before, service.Revision())
	values := effects.Values()
	require.NotEmpty(t, values)
	last := values[len(values)-1]
	require.NotNil(t, last.Result)
	require.Equal(t, domain.ActionReasonUnassigned, last.Result.Reason)
	require.False(t, last.Result.Accepted)
	require.Equal(t, snapshot.PlayerState.SessionID, last.SessionID)
}

func TestServiceUsesInjectedOpaqueIDSourceDeterministically(t *testing.T) {
	ids := &sequenceIDSource{values: []string{
		"opaque-session-id",
		"opaque-browser-token",
		"opaque-character-id",
	}}
	service := New(Config{IDs: ids})

	for index, want := range ids.values {
		{
			got := service.nextID()
			require.Falsef(t, got != want,
				"nextID() call %d = %q, want injected opaque value %q", index+1, got, want)
		}

	}
}

func TestCommitAdvancesRevisionOnlyForAcceptedTransitions(t *testing.T) {
	var effects []Effect
	service := New(Config{Enqueue: func(effect Effect) {
		effects = append(effects, effect)
	}})

	first := service.commit(func(*domain.ProcessRuntime) transition {
		return transition{accepted: true, effects: []Effect{{Live: testLiveState("first")}}}
	})
	rejected := service.commit(func(*domain.ProcessRuntime) transition {
		return transition{accepted: false, effects: []Effect{{Live: testLiveState("rejected")}}}
	})
	second := service.commit(func(*domain.ProcessRuntime) transition {
		return transition{accepted: true, effects: []Effect{{Live: testLiveState("second")}}}
	})
	require.Falsef(t, !first.accepted || first.revision != 1,
		"first commit = %#v, want accepted revision 1", first)
	require.Falsef(t, rejected.accepted || rejected.revision != 1,
		"rejected commit = %#v, want rejected at unchanged revision 1", rejected)
	require.Falsef(t, !second.accepted || second.revision != 2,
		"second commit = %#v, want accepted revision 2", second)

	wantRevisions := []uint64{1, 1, 2}
	gotRevisions := make([]uint64, 0, len(effects))
	for _, effect := range effects {
		gotRevisions = append(gotRevisions, effect.Revision)
	}
	require.Falsef(t, !cmp.Equal(gotRevisions, wantRevisions),
		"effect revisions = %v, want %v", gotRevisions, wantRevisions)

}

func TestCommitDetachesEffectsBeforeEnqueue(t *testing.T) {
	var enqueued Effect
	service := New(Config{Enqueue: func(effect Effect) {
		enqueued = effect
	}})
	produced := Effect{Live: testLiveState("canonical")}

	result := service.commit(func(*domain.ProcessRuntime) transition {
		return transition{accepted: true, effects: []Effect{produced}}
	})
	require.Falsef(t, !result.accepted || result.revision != 1,
		"commit() = %#v, want accepted revision 1", result)

	produced.Live.TerminalName = "mutated producer"
	produced.Live.Tree.Children[0].Name = "mutated child"
	produced.Live.Nav.Path[0] = "mutated path"
	produced.Live.Hack.Log[0] = "mutated log"
	produced.Live.Hack.Columns[0].Words[0].ID = "mutated word"
	require.Falsef(t, enqueued.Revision != 1,
		"enqueued revision = %d, want 1", enqueued.Revision)

	want := testLiveState("canonical")
	require.Falsef(t, !cmp.Equal(enqueued.Live, want),
		"enqueued effect aliases its producer\ngot:  %#v\nwant: %#v", enqueued.Live, want)

}

func TestCommitEnqueuesBeforeUnlocking(t *testing.T) {
	firstEnqueueStarted := make(chan struct{})
	releaseFirstEnqueue := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	var revisions []uint64

	service := New(Config{Enqueue: func(effect Effect) {
		mu.Lock()
		revisions = append(revisions, effect.Revision)
		mu.Unlock()

		if effect.Live != nil && effect.Live.TerminalID == "first" {
			once.Do(func() { close(firstEnqueueStarted) })
			<-releaseFirstEnqueue
		}
	}})

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		service.commit(func(*domain.ProcessRuntime) transition {
			return transition{accepted: true, effects: []Effect{{Live: testLiveState("first")}}}
		})
	}()

	select {
	case <-firstEnqueueStarted:
	case <-time.After(time.Second):
		assert.FailNow(t, "first effect was not enqueued")
	}

	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		service.commit(func(*domain.ProcessRuntime) transition {
			return transition{accepted: true, effects: []Effect{{Live: testLiveState("second")}}}
		})
	}()

	select {
	case <-secondDone:
		assert.FailNow(t, "second transition committed while the first effect enqueue still held the transaction boundary")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirstEnqueue)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		assert.FailNow(t, "first transition did not finish after its enqueue was released")
	}
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		assert.FailNow(t, "second transition did not finish after the first transaction unlocked")
	}

	mu.Lock()
	got := append([]uint64(nil), revisions...)
	mu.Unlock()
	{
		want := []uint64{1, 2}
		require.Falsef(t, !cmp.Equal(got, want),
			"enqueued revisions = %v, want %v", got, want)
	}

}

func TestRosterCreationAndFreshBroadcastSelection(t *testing.T) {
	service := newUS1Service()

	_, err := addCharacter(service, "Mara")
	require.Falsef(t, err != nil,
		"AddCharacter(Mara) error = %v", err)

	state, err := addCharacter(service, "Boone")
	require.Falsef(t, err != nil,
		"AddCharacter(Boone) error = %v", err)
	require.Falsef(t, len(state.Roster) != 2 || state.Roster[0].Name != "Mara" || state.Roster[1].Name != "Boone",
		"roster after creation = %#v, want Mara then Boone", state.Roster)
	require.Falsef(t, state.Roster[0].ID == "" || state.Roster[1].ID == "" || state.Roster[0].ID == state.Roster[1].ID,
		"roster IDs are not distinct opaque values: %#v", state.Roster)

	state, err = service.StartBroadcast()
	require.Falsef(t, err != nil,
		"StartBroadcast() error = %v", err)
	require.Falsef(t, state.Broadcast == nil || state.Broadcast.ID == "" || state.Broadcast.ControllerSessionID != nil,
		"fresh broadcast = %#v, want new ID with no controller", state.Broadcast)

	for _, character := range state.Roster {
		require.Falsef(t, character.ClaimedBySessionID != nil,
			"fresh broadcast retained claim %#v", character)

	}

	identity := service.CreateSession(domain.ConnectionID("connection-1"))
	require.Falsef(t, identity.SessionID == "" || identity.BrowserToken == "" || identity.State == nil || identity.State.FallbackName == "",
		"CreateSession() = %#v, want opaque identity, token, and fallback state", identity)

	result := service.SelectCharacter(CharacterSelection{
		SessionID:   identity.SessionID,
		RequestID:   "select-1",
		BroadcastID: state.Broadcast.ID,
		CharacterID: state.Roster[0].ID,
	})
	require.Falsef(t, !result.Accepted,
		"fresh SelectCharacter() = %#v, want accepted", result)

	selected := service.Snapshot()
	require.Falsef(t, selected.Broadcast == nil || selected.Broadcast.ControllerSessionID == nil || *selected.Broadcast.ControllerSessionID != identity.SessionID,
		"initial controller = %#v, want %q", selected.Broadcast, identity.SessionID)

	assertExclusiveClaimInvariants(t, selected)
	{
		got := masterSession(t, selected, identity.SessionID)
		require.Falsef(t, got.Role != domain.PlayerRoleActive || got.Character == nil || got.Character.ID != state.Roster[0].ID,
			"selected session = %#v, want active Mara assignment", got)
	}

}

func TestConcurrentSameCharacterClaimHasExactlyOneWinnerAcross100Trials(t *testing.T) {
	for trial := range 100 {
		service := newUS1Service()
		_, err := addCharacter(service, "Mara")
		require.Falsef(t, err != nil,
			"trial %d AddCharacter() error = %v", trial, err)

		state, err := service.StartBroadcast()
		require.Falsef(t, err != nil,
			"trial %d StartBroadcast() error = %v", trial, err)

		first := service.CreateSession(domain.ConnectionID(fmt.Sprintf("trial-%d-first", trial)))
		second := service.CreateSession(domain.ConnectionID(fmt.Sprintf("trial-%d-second", trial)))
		selection := func(identity SessionIdentity, requestID string) domain.ActionResult {
			return service.SelectCharacter(CharacterSelection{
				SessionID: identity.SessionID, RequestID: requestID,
				BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
			})
		}

		start := make(chan struct{})
		results := make(chan domain.ActionResult, 2)
		var workers sync.WaitGroup
		for index, candidate := range []SessionIdentity{first, second} {
			workers.Add(1)
			go func(index int, candidate SessionIdentity) {
				defer workers.Done()
				<-start
				results <- selection(candidate, fmt.Sprintf("trial-%d-request-%d", trial, index))
			}(index, candidate)
		}
		close(start)
		workers.Wait()
		close(results)

		accepted := 0
		for result := range results {
			if result.Accepted {
				accepted++
			}
		}
		require.Falsef(t, accepted != 1,
			"trial %d accepted claims = %d, want exactly 1", trial, accepted)

		snapshot := service.Snapshot()
		assertExclusiveClaimInvariants(t, snapshot)
		require.Falsef(t, claimedRosterCount(snapshot) != 1 || activeSessionCount(snapshot) != 1,
			"trial %d state = %#v, want one claim and one controller", trial, snapshot)

	}
}

func TestConcurrentDifferentFirstAssignmentsChooseExactlyOneControllerAcross100Trials(t *testing.T) {
	for trial := range 100 {
		service := newUS1Service()
		_, err := addCharacter(service, "Mara")
		require.Falsef(t, err != nil,
			"trial %d AddCharacter(Mara) error = %v", trial, err)

		_, err = addCharacter(service, "Boone")
		require.Falsef(t, err != nil,
			"trial %d AddCharacter(Boone) error = %v", trial, err)

		state, err := service.StartBroadcast()
		require.Falsef(t, err != nil,
			"trial %d StartBroadcast() error = %v", trial, err)

		first := service.CreateSession(domain.ConnectionID(fmt.Sprintf("trial-%d-first", trial)))
		second := service.CreateSession(domain.ConnectionID(fmt.Sprintf("trial-%d-second", trial)))

		start := make(chan struct{})
		results := make(chan domain.ActionResult, 2)
		var workers sync.WaitGroup
		for index, candidate := range []struct {
			identity    SessionIdentity
			characterID domain.CharacterID
		}{{first, state.Roster[0].ID}, {second, state.Roster[1].ID}} {
			workers.Add(1)
			go func(index int, candidate SessionIdentity, characterID domain.CharacterID) {
				defer workers.Done()
				<-start
				results <- service.SelectCharacter(CharacterSelection{
					SessionID: candidate.SessionID, RequestID: fmt.Sprintf("trial-%d-request-%d", trial, index),
					BroadcastID: state.Broadcast.ID, CharacterID: characterID,
				})
			}(index, candidate.identity, candidate.characterID)
		}
		close(start)
		workers.Wait()
		close(results)

		for result := range results {
			require.Falsef(t, !result.Accepted,
				"trial %d different-character selection rejected: %#v", trial, result)

		}
		snapshot := service.Snapshot()
		assertExclusiveClaimInvariants(t, snapshot)
		require.Falsef(t, claimedRosterCount(snapshot) != 2 || activeSessionCount(snapshot) != 1 || observerSessionCount(snapshot) != 1,
			"trial %d roles/claims = %#v, want two claims with one active and one observer", trial, snapshot)

	}
}

func TestSessionCannotClaimTwoCharactersAndCharacterCannotHaveTwoSessions(t *testing.T) {
	service := newUS1Service()
	_, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	_, err = addCharacter(service, "Boone")
	if err != nil {
		require.NoError(t, err)
	}
	state, err := service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	first := service.CreateSession("connection-first")
	second := service.CreateSession("connection-second")
	{

		result := service.SelectCharacter(CharacterSelection{
			SessionID: first.SessionID, RequestID: "first-mara",
			BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
		})
		require.Falsef(t, !result.Accepted,
			"first claim = %#v, want accepted", result)
	}
	{

		result := service.SelectCharacter(CharacterSelection{
			SessionID: first.SessionID, RequestID: "first-boone",
			BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[1].ID,
		})
		require.Falsef(t, result.Accepted,
			"same session second claim = %#v, want rejected", result)
	}
	{

		result := service.SelectCharacter(CharacterSelection{
			SessionID: second.SessionID, RequestID: "second-mara",
			BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
		})
		require.Falsef(t, result.Accepted,
			"same character second session claim = %#v, want rejected", result)
	}

	snapshot := service.Snapshot()
	assertExclusiveClaimInvariants(t, snapshot)
	require.Falsef(t, claimedRosterCount(snapshot) != 1 || masterSession(t, snapshot, first.SessionID).Character.ID != state.Roster[0].ID || masterSession(t, snapshot, second.SessionID).Character != nil,
		"rejected claims changed assignments: %#v", snapshot)

}

func TestPlayerActionAuthorizationRejectsWithoutTerminalMutationOrRandomness(t *testing.T) {
	tests := []struct {
		name       string
		connection func(us2Fixture) domain.ConnectionID
		mutate     func(*testing.T, us2Fixture)
		terminalID string
		wantReason domain.ActionReason
	}{
		{
			name: "observer", connection: func(fixture us2Fixture) domain.ConnectionID { return fixture.observerConnection },
			wantReason: domain.ActionReasonNotController,
		},
		{
			name: "unassigned", connection: func(fixture us2Fixture) domain.ConnectionID { return fixture.unassignedConnection },
			wantReason: domain.ActionReasonUnassigned,
		},
		{
			name: "unknown", connection: func(us2Fixture) domain.ConnectionID { return "connection-unknown" },
			wantReason: domain.ActionReasonInvalidSession,
		},
		{
			name: "stale terminal", connection: func(fixture us2Fixture) domain.ConnectionID { return fixture.controllerConnection },
			terminalID: "terminal-stale", wantReason: domain.ActionReasonStaleTerminal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := &recordingTerminalRuntime{}
			fixture := newUS2Fixture(t, runtime)
			if test.mutate != nil {
				test.mutate(t, fixture)
			}
			terminalID := test.terminalID
			if terminalID == "" {
				terminalID = fixture.terminalID
			}
			requestID := "reject-" + test.name
			beforeTerminal := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
			beforeRevision := fixture.service.Revision()
			beforeCalls := runtime.Calls()
			beforeRandom := runtime.RandomCalls()

			fixture.service.DispatchPlayerAction(test.connection(fixture), domain.RuntimeCommand{
				RequestID: requestID, BroadcastID: fixture.broadcastID, TerminalID: terminalID,
				Kind: domain.RuntimeCommandActivatePattern, PatternID: "opaque-current-pattern",
			})

			result := actionResultForRequest(t, fixture.effects, requestID)
			require.Falsef(t, result.Accepted || result.Reason != test.wantReason || result.Revision != beforeRevision,
				"DispatchPlayerAction() result = %#v, want rejected %q at revision %d", result, test.wantReason, beforeRevision)
			{

				got := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
				require.Falsef(t, !cmp.Equal(got, beforeTerminal),
					"rejected action changed canonical terminal bytes\nbefore: %s\nafter:  %s", beforeTerminal, got)
			}
			require.Falsef(t, fixture.service.Revision() != beforeRevision || runtime.Calls() != beforeCalls || runtime.RandomCalls() != beforeRandom,
				"rejected action changed revision/runtime/RNG: revision %d->%d calls %d->%d RNG %d->%d",
				beforeRevision, fixture.service.Revision(), beforeCalls, runtime.Calls(), beforeRandom, runtime.RandomCalls())

		})
	}
}

func TestRecognitionAuthorityMatrixRejectsWithoutCanonicalReplayOrRandomEffects(t *testing.T) {
	tests := []struct {
		name       string
		handle     func(us2Fixture) domain.RecognitionHandle
		mutate     func(us2Fixture)
		broadcast  func(us2Fixture) domain.BroadcastID
		terminalID func(us2Fixture) string
		wantReason domain.ActionReason
	}{
		{
			name: "unknown handle", handle: func(us2Fixture) domain.RecognitionHandle { return "unknown-handle" },
			wantReason: domain.ActionReasonInvalidSession,
		},
		{
			name: "observer", handle: func(fixture us2Fixture) domain.RecognitionHandle {
				return domain.RecognitionHandle(fixture.observerToken)
			},
			wantReason: domain.ActionReasonNotController,
		},
		{
			name: "unassigned", handle: func(fixture us2Fixture) domain.RecognitionHandle {
				return domain.RecognitionHandle(fixture.unassignedToken)
			},
			wantReason: domain.ActionReasonUnassigned,
		},
		{
			name: "disconnected controller",
			handle: func(fixture us2Fixture) domain.RecognitionHandle {
				return domain.RecognitionHandle(fixture.controllerToken)
			},
			mutate:     func(fixture us2Fixture) { fixture.service.DetachConnection(fixture.controllerConnection) },
			wantReason: domain.ActionReasonControllerDisconnected,
		},
		{
			name: "stale broadcast", handle: func(fixture us2Fixture) domain.RecognitionHandle {
				return domain.RecognitionHandle(fixture.controllerToken)
			},
			broadcast:  func(us2Fixture) domain.BroadcastID { return "stale-broadcast" },
			wantReason: domain.ActionReasonStaleBroadcast,
		},
		{
			name: "stale terminal", handle: func(fixture us2Fixture) domain.RecognitionHandle {
				return domain.RecognitionHandle(fixture.controllerToken)
			},
			terminalID: func(us2Fixture) string { return "stale-terminal" },
			wantReason: domain.ActionReasonStaleTerminal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := &recordingTerminalRuntime{}
			fixture := newUS2Fixture(t, runtime)
			if test.mutate != nil {
				test.mutate(fixture)
			}
			broadcastID := fixture.broadcastID
			if test.broadcast != nil {
				broadcastID = test.broadcast(fixture)
			}
			terminalID := fixture.terminalID
			if test.terminalID != nil {
				terminalID = test.terminalID(fixture)
			}
			beforeTerminal := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
			beforeRevision := fixture.service.Revision()
			beforeCalls := runtime.Calls()
			beforeRandom := runtime.RandomCalls()

			result := fixture.service.DispatchPlayerActionForRecognition(test.handle(fixture), domain.RuntimeCommand{
				RequestID: "authority-" + domain.RequestID(test.name), BroadcastID: broadcastID, TerminalID: terminalID,
				Kind: domain.RuntimeCommandActivatePattern, PatternID: "current-pattern",
			})
			require.Falsef(t, result.Accepted || result.Reason != test.wantReason || result.Revision != beforeRevision,
				"result = %#v, want %q at revision %d", result, test.wantReason, beforeRevision)
			require.Falsef(t, fixture.service.Revision() != beforeRevision || runtime.Calls() != beforeCalls || runtime.RandomCalls() != beforeRandom,
				"rejection changed canonical counters: revision=%d calls=%d random=%d", fixture.service.Revision(), runtime.Calls(), runtime.RandomCalls())
			{

				got := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
				require.Falsef(t, !cmp.Equal(got, beforeTerminal),
					"rejection changed canonical terminal\nbefore: %s\nafter: %s", beforeTerminal, got)
			}

		})
	}
}

func TestControllerActionIsAuthorizedAndObserverStateRemainsCanonical(t *testing.T) {
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	beforeRevision := fixture.service.Revision()

	fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "controller-nav", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs",
	})

	result := actionResultForRequest(t, fixture.effects, "controller-nav")
	require.Falsef(t, !result.Accepted || result.Reason != domain.ActionReasonAccepted || result.Revision != beforeRevision+1,
		"controller action result = %#v, want accepted revision %d", result, beforeRevision+1)

	terminal := canonicalTerminal(t, fixture.service, fixture.terminalID)
	require.Falsef(t, !cmp.Equal(terminal.Nav.Path, []string{"root", "docs"}) || runtime.Calls() != 1,
		"controller navigation = %#v, runtime calls = %d", terminal.Nav, runtime.Calls())

	observer, ok := fixture.service.PlayerSnapshot(fixture.observerSession)
	require.Falsef(t, !ok || observer.Role != domain.PlayerRoleObserver,
		"observer state changed after controller action: %#v, ok=%t", observer, ok)

}

func TestDuplicatePlayerActionFingerprintNeverMutatesTwice(t *testing.T) {
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	command := domain.RuntimeCommand{
		RequestID: "duplicate-nav", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs",
	}

	fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
	first := actionResultForRequest(t, fixture.effects, command.RequestID)
	require.Falsef(t, !first.Accepted,
		"first action = %#v, want accepted", first)

	afterFirst := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
	afterFirstRevision := fixture.service.Revision()
	afterFirstCalls := runtime.Calls()

	fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
	replayed := actionResultForRequest(t, fixture.effects, command.RequestID)
	require.Falsef(t, !cmp.Equal(replayed, first),
		"exact duplicate result = %#v, want cached %#v", replayed, first)
	require.False(t, runtime.Calls() != afterFirstCalls || fixture.service.Revision() != afterFirstRevision || !cmp.Equal(canonicalTerminalBytes(t, fixture.service, fixture.terminalID), afterFirst),
		"exact duplicate repeated canonical mutation")

	different := command
	different.NodeID = "other-node"
	fixture.service.DispatchPlayerAction(fixture.controllerConnection, different)
	conflict := actionResultForRequest(t, fixture.effects, command.RequestID)
	require.Falsef(t, conflict.Accepted || conflict.Reason != domain.ActionReasonDuplicate || conflict.Revision != afterFirstRevision,
		"different duplicate fingerprint = %#v, want duplicate rejection", conflict)
	require.False(t, runtime.Calls() != afterFirstCalls || fixture.service.Revision() != afterFirstRevision || !cmp.Equal(canonicalTerminalBytes(t, fixture.service, fixture.terminalID), afterFirst),
		"different duplicate fingerprint changed canonical state")

}

func TestRequestReplayCacheBoundsDeterministicallyAt256AndClearsWithBroadcastEpoch(t *testing.T) {
	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.SessionsByID[fixture.controllerSession].RequestResults = make(map[domain.RequestID]domain.RequestResultRecord)
		return transition{persist: true}
	})
	beforeRevision := fixture.service.Revision()

	for index := range 257 {
		requestID := domain.RequestID(fmt.Sprintf("request-%03d", index))
		result := fixture.service.SelectCharacter(CharacterSelection{
			ConnectionID: fixture.controllerConnection, SessionID: fixture.controllerSession, RequestID: requestID,
			BroadcastID: fixture.broadcastID, CharacterID: "character-does-not-matter",
		})
		require.Falsef(t, result.Accepted || result.Reason != domain.ActionReasonConflict,
			"fill request %q = %#v, want conflict", requestID, result)

	}

	fixture.service.mu.RLock()
	cache := fixture.service.runtime.SessionsByID[fixture.controllerSession].RequestResults
	_, oldestRetained := cache["request-000"]
	_, newestRetained := cache["request-256"]
	cacheSize := len(cache)
	fixture.service.mu.RUnlock()
	require.Falsef(t, cacheSize != defaultRequestResultLimit || oldestRetained || !newestRetained,
		"bounded replay cache size=%d oldest=%t newest=%t", cacheSize, oldestRetained, newestRetained)

	exact := fixture.service.SelectCharacter(CharacterSelection{
		ConnectionID: fixture.controllerConnection, SessionID: fixture.controllerSession, RequestID: "request-256",
		BroadcastID: fixture.broadcastID, CharacterID: "character-does-not-matter",
	})
	require.Falsef(t, exact.Accepted || exact.Reason != domain.ActionReasonConflict || exact.Revision != beforeRevision,
		"retained exact replay = %#v", exact)

	changed := fixture.service.SelectCharacter(CharacterSelection{
		ConnectionID: fixture.controllerConnection, SessionID: fixture.controllerSession, RequestID: "request-256",
		BroadcastID: fixture.broadcastID, CharacterID: "different-payload",
	})
	require.Falsef(t, changed.Accepted || changed.Reason != domain.ActionReasonDuplicate || changed.Revision != beforeRevision,
		"retained changed replay = %#v", changed)

	evicted := fixture.service.SelectCharacter(CharacterSelection{
		ConnectionID: fixture.controllerConnection, SessionID: fixture.controllerSession, RequestID: "request-000",
		BroadcastID: fixture.broadcastID, CharacterID: "different-payload",
	})
	require.Falsef(t, evicted.Accepted || evicted.Reason != domain.ActionReasonConflict || evicted.Revision != beforeRevision,
		"evicted request was not evaluated anew: %#v", evicted)
	require.Falsef(t, fixture.service.Revision() != beforeRevision,
		"replay stress changed revision: got %d want %d", fixture.service.Revision(), beforeRevision)

	if _, err := fixture.service.EndBroadcast(); err != nil {
		require.NoError(t, err)
	}
	if _, err := fixture.service.StartBroadcast(); err != nil {
		require.NoError(t, err)
	}
	fixture.service.mu.RLock()
	defer fixture.service.mu.RUnlock()
	{
		got := len(fixture.service.runtime.SessionsByID[fixture.controllerSession].RequestResults)
		require.Falsef(t, got != 0,
			"broadcast restart retained %d replay records", got)
	}

}

func TestConcurrentPatternActivationHasOneCoordinatorWinnerAndOneOutcomeSequenceAcross100Races(t *testing.T) {
	for trial := range 100 {
		random := &controlCountingRandom{}
		liveRuntime := live.New(random, controlFixedWords{})
		effects := testutil.NewFakeOrderedEffectSink[Effect]()
		service := New(Config{
			IDs: &counterIDSource{}, Enqueue: effects.Enqueue,
			Runtime: liveRuntime, Terminals: liveRuntime, TrustedHack: liveRuntime,
		})
		state, err := addCharacter(service, "Mara")
		if err != nil {
			require.NoError(t, err)
		}
		state, err = service.StartBroadcast()
		if err != nil {
			require.NoError(t, err)
		}
		connectionID := domain.ConnectionID(fmt.Sprintf("pattern-controller-%d", trial))
		controller := service.CreateSession(connectionID)
		{
			result := service.SelectCharacter(CharacterSelection{
				ConnectionID: connectionID, SessionID: controller.SessionID, RequestID: "select-controller",
				BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
			})
			require.Falsef(t, !result.Accepted,
				"trial %d selection = %#v", trial, result)
		}
		{

			_, err = service.RequestTerminalActivation(domain.TerminalTarget{
				TerminalID: "terminal-pattern", TerminalName: "Pattern", HackLevel: 1, Tree: testPatternTree(),
			})
			require.Falsef(t, err != nil,
				"trial %d activation: %v", trial, err)
		}

		projection, _, ok := service.CurrentLiveForSession(controller.SessionID)
		require.Falsef(t, !ok || projection == nil || projection.Hack == nil || len(projection.Hack.Patterns) == 0,
			"trial %d has no generated pattern: %#v", trial, projection)

		patternID := projection.Hack.Patterns[0].ID
		beforeRandom := random.Calls()
		beforeRevision := service.Revision()

		start := make(chan struct{})
		results := make(chan domain.ActionResult, 2)
		for contender := range 2 {
			go func() {
				<-start
				results <- service.DispatchPlayerAction(connectionID, domain.RuntimeCommand{
					RequestID:   domain.RequestID(fmt.Sprintf("pattern-%d-%d", trial, contender)),
					BroadcastID: state.Broadcast.ID, TerminalID: "terminal-pattern",
					Kind: domain.RuntimeCommandActivatePattern, PatternID: patternID,
				})
			}()
		}
		close(start)
		first, second := <-results, <-results
		accepted := 0
		for _, result := range []domain.ActionResult{first, second} {
			if result.Accepted {
				accepted++
			} else {
				require.Falsef(t, result.Reason != domain.ActionReasonInvalidAction,
					"trial %d losing result = %#v", trial, result)
			}

		}
		require.Falsef(t, accepted != 1 || service.Revision() != beforeRevision+1,
			"trial %d accepted=%d revision=%d want %d", trial, accepted, service.Revision(), beforeRevision+1)
		{

			draws := random.Calls() - beforeRandom
			require.Falsef(t, draws < 1 || draws > 2,
				"trial %d action random draws=%d, want one outcome plus at most one dud draw", trial, draws)
		}

	}
}

func TestPlayerActionAndControllerReassignmentFollowCoordinatorOrder(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	runtime := &recordingTerminalRuntime{started: started, release: release}
	fixture := newUS2Fixture(t, runtime)

	actionDone := make(chan struct{})
	go func() {
		defer close(actionDone)
		fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
			RequestID: "before-reassign", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
			Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs",
		})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		assert.FailNow(t, "controller action did not enter runtime boundary")
	}

	reassigned := make(chan struct{})
	go func() {
		defer close(reassigned)
		setControllerForTest(fixture.service, fixture.observerSession)
	}()
	select {
	case <-reassigned:
		assert.FailNow(t, "controller reassignment overtook an action already inside the coordinator transaction")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-actionDone:
	case <-time.After(time.Second):
		assert.FailNow(t, "ordered controller action did not finish")
	}
	select {
	case <-reassigned:
	case <-time.After(time.Second):
		assert.FailNow(t, "controller reassignment did not follow the completed action")
	}
	{
		result := actionResultForRequest(t, fixture.effects, "before-reassign")
		require.Falsef(t, !result.Accepted,
			"action ordered before reassignment = %#v, want accepted", result)
	}

	before := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
	beforeCalls := runtime.Calls()
	beforeRevision := fixture.service.Revision()
	fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "after-reassign", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "back",
	})
	result := actionResultForRequest(t, fixture.effects, "after-reassign")
	require.Falsef(t, result.Accepted || result.Reason != domain.ActionReasonNotController || result.Revision != beforeRevision,
		"former controller action after reassignment = %#v, want not-controller rejection", result)
	require.False(t, runtime.Calls() != beforeCalls || !cmp.Equal(canonicalTerminalBytes(t, fixture.service, fixture.terminalID), before),
		"action ordered after reassignment mutated canonical terminal")

}

func TestSetActiveControllerRequiresConnectedAssignedObserverAndPreservesCanonicalRuntime(t *testing.T) {
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	beforeState := fixture.service.Snapshot()
	beforeTerminal := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
	beforeAssignments := masterAssignments(beforeState)
	beforeRevision := fixture.service.Revision()

	state, err := fixture.service.SetActiveController(fixture.observerSession)
	require.Falsef(t, err != nil,
		"SetActiveController(eligible observer) error = %v", err)
	require.Falsef(t, state == nil || state.Revision != beforeRevision+1,
		"SetActiveController() state = %#v, want revision %d", state, beforeRevision+1)

	assertExactlyOneController(t, state, fixture.observerSession)
	{
		got := masterSession(t, state, fixture.controllerSession).Role
		require.Falsef(t, got != domain.PlayerRoleObserver,
			"former controller role = %q, want observer", got)
	}
	require.Falsef(t, !cmp.Equal(masterAssignments(state), beforeAssignments),
		"reassignment changed character assignments\nbefore: %#v\nafter:  %#v", beforeAssignments, masterAssignments(state))
	{

		got := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
		require.Falsef(t, !cmp.Equal(got, beforeTerminal),
			"reassignment changed terminal/private puzzle bytes\nbefore: %s\nafter:  %s", beforeTerminal, got)
	}
	require.Falsef(t, runtime.Calls() != 0 || runtime.RandomCalls() != 0,
		"reassignment entered gameplay runtime: calls=%d RNG=%d", runtime.Calls(), runtime.RandomCalls())

}

func TestSetActiveControllerRejectsEveryIneligibleTargetWithoutMutation(t *testing.T) {
	t.Run("unknown logical session", func(t *testing.T) {
		fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
		assertControllerReassignmentRejected(t, fixture.service, "unknown-session", fixture.terminalID)
	})

	t.Run("unassigned logical session", func(t *testing.T) {
		fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
		assertControllerReassignmentRejected(t, fixture.service, fixture.unassignedSession, fixture.terminalID)
	})

	t.Run("disconnected assigned observer", func(t *testing.T) {
		fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
		fixture.service.DetachConnection(fixture.observerConnection)
		assertControllerReassignmentRejected(t, fixture.service, fixture.observerSession, fixture.terminalID)
	})

	t.Run("no current broadcast", func(t *testing.T) {
		service := New(Config{IDs: &counterIDSource{}})
		session := service.CreateSession("no-broadcast-connection")
		assertControllerReassignmentRejected(t, service, session.SessionID, "")
	})
}

func TestActionAndSetActiveControllerHaveOneCoordinatorOrderAcross100Interleavings(t *testing.T) {
	for trial := range 100 {
		t.Run(fmt.Sprintf("trial-%03d", trial), func(t *testing.T) {
			actionFirst := trial%2 == 0
			runtime := &recordingTerminalRuntime{}
			if actionFirst {
				runtime.started = make(chan struct{})
				runtime.release = make(chan struct{})
			}
			fixture := newUS2Fixture(t, runtime)
			beforeState := fixture.service.Snapshot()
			beforeAssignments := masterAssignments(beforeState)
			beforeTerminal := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
			requestID := domain.RequestID(fmt.Sprintf("reassign-race-%03d", trial))
			command := domain.RuntimeCommand{
				RequestID: requestID, BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
				Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs",
			}

			type reassignmentResult struct {
				state *domain.MasterCoordinationState
				err   error
			}
			reassigned := make(chan reassignmentResult, 1)
			actionDone := make(chan struct{})
			if actionFirst {
				go func() {
					fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
					close(actionDone)
				}()
				waitForSignal(t, runtime.started, "former-controller action to enter runtime")
				go func() {
					state, err := fixture.service.SetActiveController(fixture.observerSession)
					reassigned <- reassignmentResult{state: state, err: err}
				}()
				close(runtime.release)
			} else {
				gateStarted := make(chan struct{})
				gateRelease := make(chan struct{})
				originalEnqueue := fixture.service.enqueue
				var gateOnce sync.Once
				fixture.service.enqueue = func(effect Effect) {
					if effect.Master != nil && effect.Master.Broadcast != nil && effect.Master.Broadcast.ControllerSessionID != nil && *effect.Master.Broadcast.ControllerSessionID == fixture.observerSession {
						gateOnce.Do(func() {
							close(gateStarted)
							<-gateRelease
						})
					}
					originalEnqueue(effect)
				}
				go func() {
					state, err := fixture.service.SetActiveController(fixture.observerSession)
					reassigned <- reassignmentResult{state: state, err: err}
				}()
				waitForSignal(t, gateStarted, "controller reassignment to enter ordered publication")
				go func() {
					fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
					close(actionDone)
				}()
				close(gateRelease)
			}

			waitForSignal(t, actionDone, "interleaved former-controller action")
			var reassignment reassignmentResult
			select {
			case reassignment = <-reassigned:
			case <-time.After(time.Second):
				assert.FailNow(t, "interleaved controller reassignment did not finish")
			}
			require.Falsef(t, reassignment.err != nil || reassignment.state == nil,
				"SetActiveController() = state %#v error %v", reassignment.state, reassignment.err)

			assertExactlyOneController(t, reassignment.state, fixture.observerSession)
			require.Falsef(t, !cmp.Equal(masterAssignments(reassignment.state), beforeAssignments),
				"trial %d changed assignments: got %#v want %#v", trial, masterAssignments(reassignment.state), beforeAssignments)

			result := actionResultForRequest(t, fixture.effects, string(requestID))
			if actionFirst {
				require.Falsef(t, !result.Accepted || result.Reason != domain.ActionReasonAccepted || runtime.Calls() != 1,
					"action-before-reassignment result = %#v runtime calls=%d", result, runtime.Calls())
				{

					got := canonicalTerminal(t, fixture.service, fixture.terminalID).Nav.Path
					require.Falsef(t, !cmp.Equal(got, []string{"root", "docs"}),
						"accepted ordered action navigation = %#v", got)
				}

			} else {
				require.Falsef(t, result.Accepted || result.Reason != domain.ActionReasonNotController || runtime.Calls() != 0,
					"reassignment-before-action result = %#v runtime calls=%d", result, runtime.Calls())
				{

					got := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
					require.Falsef(t, !cmp.Equal(got, beforeTerminal),
						"rejected former-controller action changed terminal\nbefore: %s\nafter:  %s", beforeTerminal, got)
				}

			}

			beforeRejected := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
			beforeRejectedCalls := runtime.Calls()
			beforeRejectedRevision := fixture.service.Revision()
			fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
				RequestID: domain.RequestID(fmt.Sprintf("former-controller-%03d", trial)), BroadcastID: fixture.broadcastID,
				TerminalID: fixture.terminalID, Kind: domain.RuntimeCommandNavigate, Action: "back",
			})
			formerResult := actionResultForRequest(t, fixture.effects, fmt.Sprintf("former-controller-%03d", trial))
			require.Falsef(t, formerResult.Accepted || formerResult.Reason != domain.ActionReasonNotController || formerResult.Revision != beforeRejectedRevision,
				"former controller retry = %#v, want not-controller at revision %d", formerResult, beforeRejectedRevision)
			require.False(t, runtime.Calls() != beforeRejectedCalls || !cmp.Equal(canonicalTerminalBytes(t, fixture.service, fixture.terminalID), beforeRejected),
				"former-controller rejection changed canonical terminal/private puzzle")

		})
	}
}

func assertControllerReassignmentRejected(t *testing.T, service *Service, sessionID domain.LogicalSessionID, terminalID string) {
	t.Helper()
	before := service.Snapshot()
	beforeRevision := service.Revision()
	var beforeTerminal []byte
	if terminalID != "" {
		beforeTerminal = canonicalTerminalBytes(t, service, terminalID)
	}
	state, err := service.SetActiveController(sessionID)
	require.Falsef(t, err == nil,
		"SetActiveController(%q) unexpectedly succeeded with %#v", sessionID, state)
	require.Falsef(t, !cmp.Equal(state, before) || !cmp.Equal(service.Snapshot(), before) || service.Revision() != beforeRevision,
		"rejected reassignment changed authoritative state\nbefore: %#v\nresult: %#v\nafter:  %#v", before, state, service.Snapshot())
	require.False(t, terminalID != "" && !cmp.Equal(canonicalTerminalBytes(t, service, terminalID), beforeTerminal),
		"rejected reassignment changed terminal/private puzzle")

}

func assertExactlyOneController(t *testing.T, state *domain.MasterCoordinationState, want domain.LogicalSessionID) {
	t.Helper()
	require.Falsef(t, state == nil || state.Broadcast == nil || state.Broadcast.ControllerSessionID == nil || *state.Broadcast.ControllerSessionID != want,
		"controller state = %#v, want %q", state, want)

	active := 0
	for _, session := range state.Sessions {
		if session.Role == domain.PlayerRoleActive {
			active++
			require.Falsef(t, session.ID != want,
				"unexpected active session %q, want %q", session.ID, want)

		}
	}
	require.Falsef(t, active != 1,
		"active controller count = %d, want exactly 1 in %#v", active, state.Sessions)

}

func masterAssignments(state *domain.MasterCoordinationState) map[domain.LogicalSessionID]domain.CharacterID {
	assignments := make(map[domain.LogicalSessionID]domain.CharacterID)
	if state == nil {
		return assignments
	}
	for _, session := range state.Sessions {
		if session.Character != nil {
			assignments[session.ID] = session.Character.ID
		}
	}
	return assignments
}

func waitForSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		assert.FailNowf(t, "assertion failed", "timed out waiting for %s", description)
	}
}

func TestAcceptedPlayerActionsPreserveNavigationAndHackingOutcomes(t *testing.T) {
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	beforeNav := canonicalTerminal(t, fixture.service, fixture.terminalID)
	wantNav := nav.ApplyAction(beforeNav.Nav, beforeNav.Tree, "enter", "docs")

	fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "outcome-nav", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs",
	})
	{
		result := actionResultForRequest(t, fixture.effects, "outcome-nav")
		require.Falsef(t, !result.Accepted,
			"navigation result = %#v, want accepted", result)
	}
	{

		got := canonicalTerminal(t, fixture.service, fixture.terminalID).Nav
		require.Falsef(t, !cmp.Equal(got, wantNav),
			"coordinated navigation = %#v, want unchanged gameplay result %#v", got, wantNav)
	}

	beforeHack := cloneHackState(canonicalTerminal(t, fixture.service, fixture.terminalID).Hack)
	wantHack := cloneHackState(beforeHack)
	hack.ApplyGuess(wantHack, "candidate-wrong")
	fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "outcome-guess", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandGuess, TargetID: "candidate-wrong",
	})
	{
		result := actionResultForRequest(t, fixture.effects, "outcome-guess")
		require.Falsef(t, !result.Accepted,
			"hacking result = %#v, want accepted", result)
	}
	{

		got := canonicalTerminal(t, fixture.service, fixture.terminalID).Hack
		require.Falsef(t, !cmp.Equal(got, wantHack),
			"coordinated hacking outcome changed\ngot:  %#v\nwant: %#v", got, wantHack)
	}

}

func TestUniversalCommandApprovalMatrixCreatesOnePendingBeforeAnyModeEffect(t *testing.T) {
	for _, test := range []struct {
		name        string
		commandName string
		mode        domain.CommandApprovalMode
		configure   func(*commandExecutionFixture)
		transition  bool
	}{
		{
			name: "ordinary", commandName: "Open doors", mode: domain.CommandApprovalModeOrdinary,
			configure: func(fixture *commandExecutionFixture) {
				fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
					children := runtime.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children
					for index := range children {
						if children[index].ID == fixture.commandID {
							children[index].StateChange = nil
						}
					}
					return transition{accepted: true}
				})
			},
		},
		{name: "initial state-changing", commandName: "Open doors", mode: domain.CommandApprovalModeStateChange},
		{
			name: "completed state-changing", commandName: "Doors open", mode: domain.CommandApprovalModeCompletedStateChange,
			configure: func(fixture *commandExecutionFixture) {
				fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
					terminal := runtime.Broadcast.TerminalRuntimes[fixture.terminalID]
					terminal.CommandStates = map[string]domain.CommandExecutionState{
						fixture.commandID: {CompletedName: "Doors open", ResultText: "Doors opened"},
					}
					return transition{accepted: true}
				})
			},
		},
		{
			name: "terminal transition", commandName: "Open doors",
			configure: func(fixture *commandExecutionFixture) {
				fixture.service.terminalCatalog = &recordingTerminalCatalog{transitions: map[string]domain.TerminalTransitionTarget{
					fixture.terminalID + "/" + fixture.commandID: {
						SourceTerminalID: fixture.terminalID, SourceTerminalName: "Terminal 1",
						CommandID: fixture.commandID, CommandName: "Open doors",
						Target: terminalTarget("terminal-b", "Terminal B"),
					},
				}}
				fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
					children := runtime.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children
					for index := range children {
						if children[index].ID == fixture.commandID {
							children[index].StateChange = nil
							children[index].TerminalTransition = &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-b"}
						}
					}
					return transition{accepted: true}
				})
			},
			transition: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &recordingCommandStateStore{}
			fixture := newCommandExecutionFixture(t, store)
			if test.configure != nil {
				test.configure(&fixture)
			}

			before := canonicalTerminal(t, fixture.service, fixture.terminalID)
			activeBefore := *fixture.service.Snapshot().Broadcast.ActiveTerminalID
			command := fixture.command(domain.RequestID("universal-approval-" + test.name))
			result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
			require.True(t, result.Accepted)

			state := fixture.service.Snapshot()
			var pendingID string
			if test.transition {
				require.NotNil(t, state.PendingTerminalNavigation, "%s bypassed master approval", test.name)
				require.Nil(t, state.PendingCommandExecution)
				require.Equal(t, fixture.commandID, state.PendingTerminalNavigation.CommandID)
				require.Equal(t, test.commandName, state.PendingTerminalNavigation.CommandName)
				pendingID = state.PendingTerminalNavigation.RequestID
			} else {
				require.NotNil(t, state.PendingCommandExecution, "%s bypassed master approval", test.name)
				require.Nil(t, state.PendingTerminalNavigation)
				require.Equal(t, fixture.commandID, state.PendingCommandExecution.CommandID)
				require.Equal(t, test.commandName, state.PendingCommandExecution.CommandName)
				require.Equal(t, test.mode, state.PendingCommandExecution.Mode)
				pendingID = state.PendingCommandExecution.RequestID
			}
			require.NotEmpty(t, pendingID)
			require.Equal(t, activeBefore, *state.Broadcast.ActiveTerminalID)
			after := canonicalTerminal(t, fixture.service, fixture.terminalID)
			require.Equal(t, before.Nav, after.Nav)
			require.Equal(t, before.CommandStates, after.CommandStates)
			require.Zero(t, fixture.runtime.Calls(), "result became visible before approval")
			require.Zero(t, store.ExecuteCalls(), "durable state changed before approval")

			for attempt := range 20 {
				replayed := fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
				require.Equal(t, result, replayed, "exact replay %d", attempt)
				current := fixture.service.Snapshot()
				if test.transition {
					require.Equal(t, pendingID, current.PendingTerminalNavigation.RequestID)
				} else {
					require.Equal(t, pendingID, current.PendingCommandExecution.RequestID)
				}
			}
			competing := fixture.command(domain.RequestID("competing-" + test.name))
			competing.Action, competing.NodeID = "back", ""
			require.Equal(t, domain.ActionReasonConflict,
				fixture.service.DispatchPlayerAction(fixture.controllerConnection, competing).Reason)
			require.Zero(t, fixture.runtime.Calls())
			require.Zero(t, store.ExecuteCalls())
		})
	}
}

func TestOrdinaryAndCompletedCommandDecisionsPreserveModeSpecificEffects(t *testing.T) {
	for _, test := range []struct {
		name      string
		completed bool
		decision  domain.CommandExecutionDecision
	}{
		{name: "ordinary approve", decision: domain.CommandExecutionApprove},
		{name: "ordinary reject", decision: domain.CommandExecutionReject},
		{name: "completed approve", completed: true, decision: domain.CommandExecutionApprove},
		{name: "completed reject", completed: true, decision: domain.CommandExecutionReject},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &recordingCommandStateStore{}
			fixture := newCommandExecutionFixture(t, store)
			fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
				terminal := runtime.Broadcast.TerminalRuntimes[fixture.terminalID]
				if test.completed {
					terminal.CommandStates = map[string]domain.CommandExecutionState{
						fixture.commandID: {CompletedName: "Doors open", ResultText: "Doors opened"},
					}
				} else {
					for index := range terminal.Tree.Children {
						if terminal.Tree.Children[index].ID == fixture.commandID {
							terminal.Tree.Children[index].StateChange = nil
						}
					}
				}
				return transition{accepted: true}
			})

			before := canonicalTerminal(t, fixture.service, fixture.terminalID)
			selected := fixture.service.DispatchPlayerAction(
				fixture.controllerConnection,
				fixture.command(domain.RequestID("decision-"+test.name)),
			)
			require.True(t, selected.Accepted)
			pending := fixture.service.Snapshot().PendingCommandExecution
			require.NotNil(t, pending)

			state, mutation, err := fixture.service.ResolveCommandExecution(t.Context(), pending.RequestID, test.decision)
			require.NoError(t, err)
			require.Nil(t, mutation)
			require.Nil(t, state.PendingCommandExecution)
			require.Zero(t, store.ExecuteCalls())
			after := canonicalTerminal(t, fixture.service, fixture.terminalID)
			if !test.completed && test.decision == domain.CommandExecutionReject {
				require.Equal(t, &domain.CommandExecutionPresentation{
					Phase:     domain.CommandExecutionPhaseRejected,
					CommandID: fixture.commandID,
				}, after.CommandExecution)
			} else {
				require.Nil(t, after.CommandExecution)
			}
			require.Equal(t, before.CommandStates, after.CommandStates)
			if test.decision == domain.CommandExecutionApprove {
				require.NotNil(t, after.Nav.CommandNodeID)
				require.Equal(t, fixture.commandID, *after.Nav.CommandNodeID)
			} else {
				require.Equal(t, before.Nav, after.Nav)
			}
		})
	}
}

func TestStateChangingCommandSelectionCreatesOneServerOwnedPendingRequest(t *testing.T) {
	fixture := newCommandExecutionFixture(t, &recordingCommandStateStore{})
	revisionBefore := fixture.service.Revision()

	result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, fixture.command("player-request-1"))
	require.True(t, result.Accepted)
	require.Equal(t, domain.ActionReasonAccepted, result.Reason)
	require.Equal(t, revisionBefore+1, result.Revision)

	state := fixture.service.Snapshot()
	pending := state.PendingCommandExecution
	require.NotNil(t, pending)
	require.NotEmpty(t, pending.RequestID)
	require.NotEqual(t, "player-request-1", pending.RequestID, "master decision IDs are server-owned")
	require.Equal(t, fixture.broadcastID, pending.BroadcastID)
	require.Equal(t, fixture.terminalID, pending.TerminalID)
	require.Equal(t, fixture.commandID, pending.CommandID)
	require.Equal(t, "Open doors", pending.CommandName)
	require.Equal(t, domain.CommandApprovalModeStateChange, pending.Mode)
	require.Equal(t, "Open the doors?", pending.ConfirmationText)
	require.Equal(t, 0, fixture.runtime.Calls(), "pending selection must not execute ordinary navigation")
	require.Equal(t, 0, fixture.store.ExecuteCalls())

	firstRequestID := pending.RequestID
	revisionAfterFirst := fixture.service.Revision()
	for attempt := range 100 {
		blocked := fixture.service.DispatchPlayerAction(
			fixture.controllerConnection,
			fixture.command(domain.RequestID(fmt.Sprintf("pending-check-%03d", attempt))),
		)
		require.False(t, blocked.Accepted, "pending check %d", attempt)
		require.Equal(t, domain.ActionReasonConflict, blocked.Reason, "pending check %d", attempt)
		require.Equal(t, revisionAfterFirst, blocked.Revision, "pending check %d", attempt)
		require.Equal(t, firstRequestID, fixture.service.Snapshot().PendingCommandExecution.RequestID, "pending check %d", attempt)
		require.Equal(t, 0, fixture.runtime.Calls(), "pending check %d entered terminal runtime", attempt)
		require.Equal(t, 0, fixture.store.ExecuteCalls(), "pending check %d entered durable store", attempt)
	}
}

func TestCompletedStateChangingCommandRequiresFreshApprovalWithoutSecondDurableExecution(t *testing.T) {
	store := &recordingCommandStateStore{
		mutation: CommandStateMutation{Changed: true, Revision: 72, Session: commandExecutionSession(true)},
	}
	fixture := newCommandExecutionFixture(t, store)
	selected := fixture.service.DispatchPlayerAction(fixture.controllerConnection, fixture.command("complete-once"))
	require.True(t, selected.Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)

	operationContext := context.WithValue(t.Context(), commandStateContextKey{}, "durable-operation")
	approved, mutation, err := fixture.service.ResolveCommandExecution(operationContext, pending.RequestID, domain.CommandExecutionApprove)
	require.NoError(t, err)
	require.NotNil(t, mutation)
	require.Nil(t, approved.PendingCommandExecution)
	require.Equal(t, 1, store.ExecuteCalls())
	require.Equal(t, "durable-operation", store.ExecuteContexts()[0].Value(commandStateContextKey{}))
	require.Equal(t, 0, fixture.runtime.Calls())

	beforeRepeat := canonicalTerminal(t, fixture.service, fixture.terminalID)
	repeated := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		fixture.command("completed-requires-approval"),
	)
	require.True(t, repeated.Accepted)
	pending = fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending, "completed command bypassed the universal approval boundary")
	require.Equal(t, fixture.commandID, pending.CommandID)
	require.Equal(t, "Doors open", pending.CommandName)
	require.Equal(t, domain.CommandApprovalModeCompletedStateChange, pending.Mode)
	require.Equal(t, 1, store.ExecuteCalls(), "completed repeat performed a second durable execution")
	require.Equal(t, 0, fixture.runtime.Calls(), "completed result became visible before approval")
	afterRepeat := canonicalTerminal(t, fixture.service, fixture.terminalID)
	require.Equal(t, beforeRepeat.Nav, afterRepeat.Nav)
	require.Equal(t, beforeRepeat.CommandStates, afterRepeat.CommandStates)
}

func TestApproveCommandExecutionWaitsForDurabilityBeforePublishingSuccess(t *testing.T) {
	store := &recordingCommandStateStore{
		mutation: CommandStateMutation{Changed: true, Revision: 71, Session: commandExecutionSession(true)},
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	fixture := newCommandExecutionFixture(t, store)
	selected := fixture.service.DispatchPlayerAction(fixture.controllerConnection, fixture.command("approve-player-request"))
	require.True(t, selected.Accepted)
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()

	type resolution struct {
		state    *domain.MasterCoordinationState
		mutation *CommandStateMutation
		err      error
	}
	resolved := make(chan resolution, 1)
	go func() {
		state, mutation, err := fixture.service.ResolveCommandExecution(t.Context(), pending.RequestID, domain.CommandExecutionApprove)
		resolved <- resolution{state: state, mutation: mutation, err: err}
	}()

	<-store.started
	require.Equal(t, effectsBefore, fixture.effects.Calls(), "completed state published before durable store returned")
	select {
	case premature := <-resolved:
		assert.FailNow(t, "approve returned before durability", "%#v", premature)
	default:
	}
	close(store.release)
	approved := <-resolved

	require.NoError(t, approved.err)
	require.NotNil(t, approved.state)
	require.Nil(t, approved.state.PendingCommandExecution)
	require.Equal(t, revisionBefore+1, approved.state.Revision)
	require.NotNil(t, approved.mutation)
	require.Equal(t, uint64(71), approved.mutation.Revision)
	require.Equal(t, commandExecutionSession(true), approved.mutation.Session)
	require.Equal(t, [][2]string{{fixture.terminalID, fixture.commandID}}, store.ExecuteArguments())
	require.Greater(t, fixture.effects.Calls(), effectsBefore, "durable approve did not publish its accepted revision")
}

func TestApproveCommandExecutionPersistenceFailureClearsPendingWithoutSuccess(t *testing.T) {
	for attempt := range 100 {
		store := &recordingCommandStateStore{err: errors.New("private disk path: write failed")}
		fixture := newCommandExecutionFixture(t, store)
		fixture.service.DispatchPlayerAction(
			fixture.controllerConnection,
			fixture.command(domain.RequestID(fmt.Sprintf("failure-player-request-%03d", attempt))),
		)
		pending := fixture.service.Snapshot().PendingCommandExecution
		require.NotNil(t, pending, "failure attempt %d", attempt)
		revisionBefore := fixture.service.Revision()
		effectsBefore := len(fixture.effects.Values())

		state, mutation, err := fixture.service.ResolveCommandExecution(t.Context(), pending.RequestID, domain.CommandExecutionApprove)
		require.Error(t, err, "failure attempt %d", attempt)
		require.ErrorIs(t, err, ErrCommandExecutionPersistence, "failure attempt %d", attempt)
		require.NotContains(t, err.Error(), "private disk path", "failure attempt %d", attempt)
		require.Nil(t, mutation, "failure attempt %d", attempt)
		require.NotNil(t, state, "failure attempt %d", attempt)
		require.Nil(t, state.PendingCommandExecution, "failure attempt %d", attempt)
		require.Equal(t, revisionBefore+1, state.Revision, "failure attempt %d", attempt)
		require.Equal(t, 1, store.ExecuteCalls(), "failure attempt %d", attempt)
		require.Equal(t, 0, fixture.runtime.Calls(), "failure attempt %d entered terminal runtime", attempt)

		terminal := canonicalTerminal(t, fixture.service, fixture.terminalID)
		require.Empty(t, terminal.CommandStates, "failure attempt %d installed a completed snapshot", attempt)
		command := contentNodeByIDForControlTest(terminal.Tree, fixture.commandID)
		require.NotNil(t, command, "failure attempt %d", attempt)
		require.Equal(t, "Open doors", command.Name, "failure attempt %d published a completed title", attempt)
		for _, effect := range fixture.effects.Values()[effectsBefore:] {
			if effect.Live == nil {
				continue
			}
			published := contentNodeByIDForControlTest(effect.Live.Tree, fixture.commandID)
			require.NotNil(t, published, "failure attempt %d", attempt)
			require.Equal(t, "Open doors", published.Name, "failure attempt %d emitted a completed title", attempt)
		}
	}
}

func TestRejectAndDialogCloseResolvePendingWithoutPersistence(t *testing.T) {
	for trial := range 100 {
		for _, name := range []string{"explicit reject", "dialog close maps to reject"} {
			t.Run(fmt.Sprintf("%03d/%s", trial, name), func(t *testing.T) {
				store := &recordingCommandStateStore{}
				fixture := newCommandExecutionFixture(t, store)
				selected := fixture.service.DispatchPlayerAction(
					fixture.controllerConnection,
					fixture.command(domain.RequestID(fmt.Sprintf("reject-player-request-%03d-%s", trial, name))),
				)
				require.True(t, selected.Accepted)
				pending := fixture.service.Snapshot().PendingCommandExecution
				require.NotNil(t, pending)
				revisionBefore := fixture.service.Revision()

				state, mutation, err := fixture.service.ResolveCommandExecution(t.Context(), pending.RequestID, domain.CommandExecutionReject)
				require.NoError(t, err)
				require.Nil(t, mutation)
				require.NotNil(t, state)
				require.Nil(t, state.PendingCommandExecution)
				require.Equal(t, revisionBefore+1, state.Revision)
				require.Equal(t, 0, store.ExecuteCalls())
			})
		}
	}
}

func TestCommandExecutionResolutionRejectsStaleAndDuplicateRequestIDs(t *testing.T) {
	store := &recordingCommandStateStore{
		mutation: CommandStateMutation{Changed: true, Revision: 91, Session: commandExecutionSession(true)},
	}
	fixture := newCommandExecutionFixture(t, store)
	fixture.service.DispatchPlayerAction(fixture.controllerConnection, fixture.command("exact-player-request"))
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	before := fixture.service.Snapshot()

	staleState, staleMutation, staleErr := fixture.service.ResolveCommandExecution(t.Context(), "stale-server-request", domain.CommandExecutionApprove)
	require.Error(t, staleErr)
	require.ErrorIs(t, staleErr, ErrCommandExecutionStale)
	require.ErrorIs(t, fmt.Errorf("application boundary: %w", staleErr), ErrCommandExecutionStale)
	require.Nil(t, staleMutation)
	require.Equal(t, before, staleState)
	require.Equal(t, before, fixture.service.Snapshot())
	require.Equal(t, 0, store.ExecuteCalls())

	acceptedState, acceptedMutation, acceptedErr := fixture.service.ResolveCommandExecution(t.Context(), pending.RequestID, domain.CommandExecutionApprove)
	require.NoError(t, acceptedErr)
	require.NotNil(t, acceptedMutation)
	require.Nil(t, acceptedState.PendingCommandExecution)
	revisionAfterApprove := fixture.service.Revision()

	duplicateState, duplicateMutation, duplicateErr := fixture.service.ResolveCommandExecution(t.Context(), pending.RequestID, domain.CommandExecutionApprove)
	require.Error(t, duplicateErr)
	require.Nil(t, duplicateMutation)
	require.Equal(t, revisionAfterApprove, duplicateState.Revision)
	require.Nil(t, duplicateState.PendingCommandExecution)
	require.Equal(t, 1, store.ExecuteCalls())
}

func TestConcurrentStateChangingCommandSelectionsCreateExactlyOnePendingAcross100Races(t *testing.T) {
	store := &recordingCommandStateStore{}
	fixture := newCommandExecutionFixture(t, store)
	revisionBefore := fixture.service.Revision()
	effectsBefore := fixture.effects.Calls()

	const callers = 100
	results := make(chan domain.ActionResult, callers)
	var group sync.WaitGroup
	group.Add(callers)
	for index := range callers {
		go func(index int) {
			defer group.Done()
			results <- fixture.service.DispatchPlayerAction(
				fixture.controllerConnection,
				fixture.command(domain.RequestID(fmt.Sprintf("concurrent-command-%03d", index))),
			)
		}(index)
	}
	group.Wait()
	close(results)

	accepted := 0
	for result := range results {
		if result.Accepted {
			accepted++
			continue
		}
		require.Equal(t, domain.ActionReasonConflict, result.Reason)
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, revisionBefore+1, fixture.service.Revision())
	pending := fixture.service.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	require.NotEmpty(t, pending.RequestID)
	require.Equal(t, fixture.commandID, pending.CommandID)
	require.Equal(t, 0, fixture.runtime.Calls())
	require.Equal(t, 0, store.ExecuteCalls())
	require.Equal(t, 1, masterEffectCount(fixture.effects.Values()[effectsBefore:]))
}

func TestControllerDisconnectRetainsPendingCommandExecutionForMasterResolution(t *testing.T) {
	store := &recordingCommandStateStore{
		mutation: CommandStateMutation{Changed: true, Revision: 101, Session: commandExecutionSession(true)},
	}
	fixture := newCommandExecutionFixture(t, store)
	selected := fixture.service.DispatchPlayerAction(fixture.controllerConnection, fixture.command("disconnect-pending"))
	require.True(t, selected.Accepted)

	beforeDisconnect := fixture.service.Snapshot()
	pending := beforeDisconnect.PendingCommandExecution
	require.NotNil(t, pending)
	require.NotNil(t, beforeDisconnect.Broadcast)
	require.NotNil(t, beforeDisconnect.Broadcast.ControllerSessionID)
	controllerSessionID := *beforeDisconnect.Broadcast.ControllerSessionID

	fixture.service.DetachConnection(fixture.controllerConnection)
	disconnected := fixture.service.Snapshot()
	require.Equal(t, beforeDisconnect.Revision+1, disconnected.Revision)
	require.Equal(t, pending, disconnected.PendingCommandExecution)
	require.False(t, masterSession(t, disconnected, controllerSessionID).Connected)
	require.Equal(t, 0, store.ExecuteCalls())

	resolved, mutation, err := fixture.service.ResolveCommandExecution(t.Context(), pending.RequestID, domain.CommandExecutionApprove)
	require.NoError(t, err)
	require.NotNil(t, mutation)
	require.Nil(t, resolved.PendingCommandExecution)
	require.Equal(t, 1, store.ExecuteCalls())
	require.Equal(t, [][2]string{{fixture.terminalID, fixture.commandID}}, store.ExecuteArguments())
}

func TestControllerDisconnectAndCommandApprovalRaceSerializesExactlyOnce(t *testing.T) {
	const trials = 100
	for trial := range trials {
		store := &recordingCommandStateStore{
			mutation: CommandStateMutation{Changed: true, Revision: uint64(200 + trial), Session: commandExecutionSession(true)},
		}
		fixture := newCommandExecutionFixture(t, store)
		selected := fixture.service.DispatchPlayerAction(
			fixture.controllerConnection,
			fixture.command(domain.RequestID(fmt.Sprintf("disconnect-approve-race-%02d", trial))),
		)
		require.True(t, selected.Accepted)
		before := fixture.service.Snapshot()
		require.NotNil(t, before.PendingCommandExecution)
		require.NotNil(t, before.Broadcast)
		require.NotNil(t, before.Broadcast.ControllerSessionID)
		requestID := before.PendingCommandExecution.RequestID
		controllerSessionID := *before.Broadcast.ControllerSessionID

		start := make(chan struct{})
		approved := make(chan error, 1)
		var group sync.WaitGroup
		group.Add(2)
		go func() {
			defer group.Done()
			<-start
			fixture.service.DetachConnection(fixture.controllerConnection)
		}()
		go func() {
			defer group.Done()
			<-start
			state, mutation, err := fixture.service.ResolveCommandExecution(t.Context(), requestID, domain.CommandExecutionApprove)
			if err == nil && (state == nil || state.PendingCommandExecution != nil || mutation == nil) {
				err = fmt.Errorf("incomplete approval result: state=%#v mutation=%#v", state, mutation)
			}
			approved <- err
		}()
		close(start)
		group.Wait()

		require.NoError(t, <-approved, "trial %d", trial)
		final := fixture.service.Snapshot()
		require.Nil(t, final.PendingCommandExecution, "trial %d", trial)
		require.False(t, masterSession(t, final, controllerSessionID).Connected, "trial %d", trial)
		require.Equal(t, 1, store.ExecuteCalls(), "trial %d", trial)
		require.Equal(t, [][2]string{{fixture.terminalID, fixture.commandID}}, store.ExecuteArguments(), "trial %d", trial)
	}
}

func TestCommandExecutionLifecycleBoundariesClearPendingAndRejectedWithoutPersistence(t *testing.T) {
	boundaries := []string{"end broadcast", "terminal switch", "shutdown"}
	phases := []domain.CommandExecutionPhase{
		domain.CommandExecutionPhasePending,
		domain.CommandExecutionPhaseRejected,
	}

	for _, boundary := range boundaries {
		for _, phase := range phases {
			t.Run(boundary+"/"+string(phase), func(t *testing.T) {
				store := &recordingCommandStateStore{}
				fixture := newCommandExecutionFixture(t, store)
				if boundary == "terminal switch" {
					fixture.service.terminals = &recordingTerminalLifecycle{}
					fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
						active := activeTerminalRuntime(runtime.Broadcast)
						require.NotNil(t, active)
						require.NotNil(t, active.Hack)
						active.Hack.Solved = true
						return transition{accepted: true}
					})
				}

				selected := fixture.service.DispatchPlayerAction(
					fixture.controllerConnection,
					fixture.command(domain.RequestID("lifecycle-"+boundary+"-"+string(phase))),
				)
				require.True(t, selected.Accepted)
				pending := fixture.service.Snapshot().PendingCommandExecution
				require.NotNil(t, pending)
				requestID := pending.RequestID

				if phase == domain.CommandExecutionPhaseRejected {
					rejected, mutation, err := fixture.service.ResolveCommandExecution(t.Context(), requestID, domain.CommandExecutionReject)
					require.NoError(t, err)
					require.Nil(t, mutation)
					require.Nil(t, rejected.PendingCommandExecution)
				}
				presentation := canonicalTerminal(t, fixture.service, fixture.terminalID).CommandExecution
				require.NotNil(t, presentation)
				require.Equal(t, phase, presentation.Phase)
				require.Equal(t, fixture.commandID, presentation.CommandID)

				switch boundary {
				case "end broadcast":
					ended, err := fixture.service.EndBroadcast()
					require.NoError(t, err)
					require.Nil(t, ended.Broadcast)
					require.Nil(t, ended.PendingCommandExecution)
				case "terminal switch":
					targetID := "terminal-lifecycle-target"
					switched, err := fixture.service.RequestTerminalActivation(terminalTarget(targetID, "Lifecycle target"))
					require.NoError(t, err)
					require.Nil(t, switched.PendingCommandExecution)
					require.NotNil(t, switched.Broadcast)
					require.NotNil(t, switched.Broadcast.ActiveTerminalID)
					require.Equal(t, targetID, *switched.Broadcast.ActiveTerminalID)
					require.Nil(t, canonicalTerminal(t, fixture.service, fixture.terminalID).CommandExecution)
					require.Nil(t, canonicalTerminal(t, fixture.service, targetID).CommandExecution)
				case "shutdown":
					fixture.service.Shutdown()
					shutdown := fixture.service.Snapshot()
					require.Nil(t, shutdown.Broadcast)
					require.Nil(t, shutdown.PendingCommandExecution)
				default:
					assert.FailNow(t, "unknown lifecycle boundary", boundary)
				}

				require.Equal(t, 0, store.ExecuteCalls(), "lifecycle cancellation must not persist command state")
				beforeLateCallback := fixture.service.Snapshot()
				stale, mutation, err := fixture.service.ResolveCommandExecution(t.Context(), requestID, domain.CommandExecutionApprove)
				require.Error(t, err)
				require.Nil(t, mutation)
				require.Equal(t, beforeLateCallback, stale)
				require.Equal(t, beforeLateCallback, fixture.service.Snapshot())
				require.Equal(t, 0, store.ExecuteCalls(), "stale dialog callback reached persistence")
			})
		}
	}
}

func TestAttachConnectionAbsentAndUnknownTokensIssueUniqueReplacementIdentities(t *testing.T) {
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue})

	firstToken, first := service.AttachConnection("connection-first", "")
	secondToken, second := service.AttachConnection("connection-second", "")
	unknownToken, unknown := service.AttachConnection("connection-unknown", "client-stale-token")
	repeatedUnknownToken, repeatedUnknown := service.AttachConnection("connection-repeated-unknown", "client-stale-token")

	states := []*domain.PlayerState{first, second, unknown, repeatedUnknown}
	tokens := []domain.BrowserToken{firstToken, secondToken, unknownToken, repeatedUnknownToken}
	seenTokens := make(map[domain.BrowserToken]struct{}, len(tokens))
	seenSessions := make(map[domain.LogicalSessionID]struct{}, len(states))
	seenFallbacks := make(map[string]struct{}, len(states))
	for index, state := range states {
		require.Falsef(t, state == nil || state.SessionID == "" || state.FallbackName == "",
			"AttachConnection() state %d = %#v, want fresh logical identity", index, state)
		require.Falsef(t, tokens[index] == "" || tokens[index] == "client-stale-token",
			"AttachConnection() token %d = %q, want server-issued replacement", index, tokens[index])
		{

			_, duplicate := seenTokens[tokens[index]]
			require.Falsef(t, duplicate,
				"replacement browser token %q was reused", tokens[index])
		}
		{

			_, duplicate := seenSessions[state.SessionID]
			require.Falsef(t, duplicate,
				"unrecognized attachment reused logical session %q", state.SessionID)
		}
		{

			_, duplicate := seenFallbacks[state.FallbackName]
			require.Falsef(t, duplicate,
				"fresh logical sessions reused fallback name %q", state.FallbackName)
		}

		seenTokens[tokens[index]] = struct{}{}
		seenSessions[state.SessionID] = struct{}{}
		seenFallbacks[state.FallbackName] = struct{}{}
	}
	require.Falsef(t, masterEffectCount(effects.Values()) != 4,
		"fresh first-connection master effects = %d, want 4", masterEffectCount(effects.Values()))

}

func TestKnownTokenReusesStableSessionAcrossFirstAndLastPresenceTransitions(t *testing.T) {
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue})
	firstConnection := domain.ConnectionID("connection-first")
	token, initial := service.AttachConnection(firstConnection, "")
	_, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	state, err := service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	selection := service.SelectCharacter(CharacterSelection{
		ConnectionID: firstConnection, SessionID: initial.SessionID, RequestID: "select-mara",
		BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
	})
	require.Falsef(t, !selection.Accepted,
		"initial claim = %#v", selection)

	claimed := service.Snapshot()
	fallback := masterSession(t, claimed, initial.SessionID).FallbackName

	service.DetachConnection(firstConnection)
	disconnected := service.Snapshot()
	disconnectedSession := masterSession(t, disconnected, initial.SessionID)
	require.Falsef(t, disconnectedSession.Connected || disconnectedSession.Character == nil || disconnectedSession.Role != domain.PlayerRoleActive,
		"last detach changed stable claim/role: %#v", disconnectedSession)

	baseline := effects.Calls()

	secondConnection := domain.ConnectionID("connection-second")
	returnedToken, reconnected := service.AttachConnection(secondConnection, token)
	require.Falsef(t, returnedToken != token || reconnected == nil || reconnected.SessionID != initial.SessionID || reconnected.FallbackName != fallback,
		"known-token reconnect = token %q state %#v, want stable %q/%q", returnedToken, reconnected, token, initial.SessionID)
	require.Falsef(t, reconnected.Character == nil || reconnected.Character.ID != state.Roster[0].ID || reconnected.Role != domain.PlayerRoleActive,
		"known-token reconnect lost assignment/role: %#v", reconnected)
	{

		got := masterEffectCount(effects.Values()[baseline:])
		require.Falsef(t, got != 1,
			"disconnected-to-connected transition emitted %d master effects, want 1", got)
	}

	baseline = effects.Calls()
	thirdConnection := domain.ConnectionID("connection-third")
	thirdToken, third := service.AttachConnection(thirdConnection, token)
	require.Falsef(t, thirdToken != token || third == nil || third.SessionID != initial.SessionID,
		"second tab = token %q state %#v, want same logical session", thirdToken, third)
	{

		got := masterEffectCount(effects.Values()[baseline:])
		require.Falsef(t, got != 0,
			"additional connection emitted %d presence effects, want 0", got)
	}

	baseline = effects.Calls()
	service.DetachConnection(secondConnection)
	require.False(t, !masterSession(t, service.Snapshot(), initial.SessionID).Connected,
		"closing one of two connections marked the logical session disconnected")
	{

		got := masterEffectCount(effects.Values()[baseline:])
		require.Falsef(t, got != 0,
			"non-final detach emitted %d presence effects, want 0", got)
	}

	baseline = effects.Calls()
	service.DetachConnection(thirdConnection)
	final := masterSession(t, service.Snapshot(), initial.SessionID)
	require.Falsef(t, final.Connected || final.Character == nil || final.Character.ID != state.Roster[0].ID || final.Role != domain.PlayerRoleActive,
		"final detach changed stable logical-session state: %#v", final)
	{

		got := masterEffectCount(effects.Values()[baseline:])
		require.Falsef(t, got != 1,
			"final detach emitted %d presence effects, want 1", got)
	}

}

func TestUnrecognizedSessionAttachmentDoesNotReleaseExistingClaim(t *testing.T) {
	service := New(Config{IDs: &counterIDSource{}})
	ownerConnection := domain.ConnectionID("connection-owner")
	ownerToken, owner := service.AttachConnection(ownerConnection, "")
	_, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	state, err := service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	{
		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: ownerConnection, SessionID: owner.SessionID, RequestID: "select-owner",
			BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
		})
		require.Falsef(t, !result.Accepted,
			"owner selection = %#v", result)
	}

	service.DetachConnection(ownerConnection)

	replacementToken, newcomer := service.AttachConnection("connection-newcomer", "unknown-after-restart")
	require.Falsef(t, replacementToken == "" || replacementToken == "unknown-after-restart" || replacementToken == ownerToken || newcomer == nil || newcomer.SessionID == owner.SessionID,
		"unrecognized attachment = token %q state %#v", replacementToken, newcomer)

	snapshot := service.Snapshot()
	ownerState := masterSession(t, snapshot, owner.SessionID)
	newcomerState := masterSession(t, snapshot, newcomer.SessionID)
	require.Falsef(t, ownerState.Character == nil || ownerState.Character.ID != state.Roster[0].ID || ownerState.Role != domain.PlayerRoleActive,
		"unrecognized session released or demoted existing owner: %#v", ownerState)
	require.Falsef(t, newcomerState.Character != nil || newcomerState.Role != domain.PlayerRoleUnassigned,
		"unrecognized newcomer inherited claim: %#v", newcomerState)
	require.Falsef(t, snapshot.Roster[0].ClaimedBySessionID == nil || *snapshot.Roster[0].ClaimedBySessionID != owner.SessionID,
		"roster claim moved after unrecognized attachment: %#v", snapshot.Roster)

}

func TestFinalDetachRetainsObserverAndControllerClaimsWithoutPromotionOrRuntimeMutation(t *testing.T) {
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	terminalBefore := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
	assignmentsBefore := masterAssignments(fixture.service.Snapshot())

	baseline := fixture.effects.Calls()
	observerRevision := fixture.service.Revision()
	fixture.service.DetachConnection(fixture.observerConnection)
	observerDetached := fixture.service.Snapshot()
	require.Falsef(t, observerDetached.Revision != observerRevision+1,
		"observer final detach revision = %d, want %d", observerDetached.Revision, observerRevision+1)

	observer := masterSession(t, observerDetached, fixture.observerSession)
	require.Falsef(t, observer.Connected || observer.Character == nil || observer.Role != domain.PlayerRoleObserver,
		"detached observer lost stable claim/role: %#v", observer)

	assertExactlyOneController(t, observerDetached, fixture.controllerSession)
	assertPresenceOnlyEffects(t, fixture.effects.Values()[baseline:], observerDetached.Revision)

	baseline = fixture.effects.Calls()
	controllerRevision := fixture.service.Revision()
	fixture.service.DetachConnection(fixture.controllerConnection)
	controllerDetached := fixture.service.Snapshot()
	require.Falsef(t, controllerDetached.Revision != controllerRevision+1,
		"controller final detach revision = %d, want %d", controllerDetached.Revision, controllerRevision+1)

	controller := masterSession(t, controllerDetached, fixture.controllerSession)
	require.Falsef(t, controller.Connected || controller.Character == nil || controller.Role != domain.PlayerRoleActive,
		"detached controller lost stable claim/designation: %#v", controller)
	{

		got := masterSession(t, controllerDetached, fixture.observerSession).Role
		require.Falsef(t, got != domain.PlayerRoleObserver,
			"observer role after controller disconnect = %q, want no automatic promotion", got)
	}

	assertExactlyOneController(t, controllerDetached, fixture.controllerSession)
	assertPresenceOnlyEffects(t, fixture.effects.Values()[baseline:], controllerDetached.Revision)
	require.Falsef(t, !cmp.Equal(masterAssignments(controllerDetached), assignmentsBefore),
		"final detaches changed claims\nbefore: %#v\nafter:  %#v", assignmentsBefore, masterAssignments(controllerDetached))
	{

		got := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
		require.Falsef(t, !cmp.Equal(got, terminalBefore),
			"final detaches changed terminal/private puzzle\nbefore: %s\nafter:  %s", terminalBefore, got)
	}
	require.Falsef(t, runtime.Calls() != 0 || runtime.RandomCalls() != 0,
		"presence transitions entered gameplay runtime: calls=%d RNG=%d", runtime.Calls(), runtime.RandomCalls())

}

func TestControllerReconnectBeforeAndAfterReassignmentRestoresAuthoritativeRoleOnly(t *testing.T) {
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	terminalBefore := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
	assignmentsBefore := masterAssignments(fixture.service.Snapshot())

	fixture.service.DetachConnection(fixture.controllerConnection)
	reconnectConnection := domain.ConnectionID("connection-controller-reconnect")
	returnedToken, reconnected := fixture.service.AttachConnection(reconnectConnection, fixture.controllerToken)
	require.Falsef(t, returnedToken != fixture.controllerToken || reconnected == nil || reconnected.SessionID != fixture.controllerSession || reconnected.Character == nil || reconnected.Role != domain.PlayerRoleActive,
		"unchanged controller reconnect = token %q state %#v", returnedToken, reconnected)

	assertExactlyOneController(t, fixture.service.Snapshot(), fixture.controllerSession)

	fixture.service.DetachConnection(reconnectConnection)
	state, err := fixture.service.SetActiveController(fixture.observerSession)
	require.Falsef(t, err != nil,
		"reassign while former controller disconnected: %v", err)

	assertExactlyOneController(t, state, fixture.observerSession)

	secondReconnect := domain.ConnectionID("connection-former-controller-return")
	returnedToken, formerController := fixture.service.AttachConnection(secondReconnect, fixture.controllerToken)
	require.Falsef(t, returnedToken != fixture.controllerToken || formerController == nil || formerController.SessionID != fixture.controllerSession,
		"former-controller reconnect identity = token %q state %#v", returnedToken, formerController)
	require.Falsef(t, formerController.Character == nil || formerController.Role != domain.PlayerRoleObserver || formerController.Phase != domain.PlayerPhaseObserving,
		"reassigned former-controller reconnect = %#v, want assigned observer", formerController)

	final := fixture.service.Snapshot()
	assertExactlyOneController(t, final, fixture.observerSession)
	require.Falsef(t, !cmp.Equal(masterAssignments(final), assignmentsBefore),
		"disconnect/reconnect/reassignment changed claims\nbefore: %#v\nafter:  %#v", assignmentsBefore, masterAssignments(final))
	{

		got := canonicalTerminalBytes(t, fixture.service, fixture.terminalID)
		require.Falsef(t, !cmp.Equal(got, terminalBefore),
			"disconnect/reconnect/reassignment changed terminal/private puzzle\nbefore: %s\nafter:  %s", terminalBefore, got)
	}
	require.Falsef(t, runtime.Calls() != 0 || runtime.RandomCalls() != 0,
		"disconnect lifecycle entered gameplay runtime: calls=%d RNG=%d", runtime.Calls(), runtime.RandomCalls())

}

func TestDirectTerminalActivationClearAndLateAssignmentPreserveBroadcastIdentity(t *testing.T) {
	actions := &recordingTerminalRuntime{}
	terminals := &recordingTerminalLifecycle{}
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, Runtime: actions, Terminals: terminals})

	state, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	maraID := state.Roster[0].ID
	state, err = addCharacter(service, "Boone")
	if err != nil {
		require.NoError(t, err)
	}
	booneID := state.Roster[1].ID
	state, err = addCharacter(service, "Arcade")
	if err != nil {
		require.NoError(t, err)
	}
	arcadeID := state.Roster[2].ID
	state, err = service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	broadcastID := state.Broadcast.ID
	controller := service.CreateSession("switch-controller")
	observer := service.CreateSession("switch-observer")
	late := service.CreateSession("switch-late")
	{
		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: "switch-controller", SessionID: controller.SessionID, RequestID: "switch-select-controller",
			BroadcastID: broadcastID, CharacterID: maraID,
		})
		require.Falsef(t, !result.Accepted,
			"controller selection = %#v", result)
	}
	{

		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: "switch-observer", SessionID: observer.SessionID, RequestID: "switch-select-observer",
			BroadcastID: broadcastID, CharacterID: booneID,
		})
		require.Falsef(t, !result.Accepted,
			"observer selection = %#v", result)
	}

	identityBefore := service.Snapshot()
	assignmentsBefore := masterAssignments(identityBefore)

	terminalA := terminalTarget("terminal-a", "Overseer A")
	state, err = service.RequestTerminalActivation(terminalA)
	require.Falsef(t, err != nil,
		"RequestTerminalActivation(A) error = %v", err)

	assertActiveTerminalAndIdentity(t, state, "terminal-a", broadcastID, controller.SessionID, assignmentsBefore)
	controllerState, ok := service.PlayerSnapshot(controller.SessionID)
	require.Falsef(t, !ok || controllerState.ActiveTerminalID != "terminal-a" || controllerState.Phase != domain.PlayerPhaseControlling,
		"controller after activation A = %#v, ok=%t", controllerState, ok)

	terminalARuntime := canonicalTerminal(t, service, "terminal-a")

	terminalB := terminalTarget("terminal-b", "Overseer B")
	state, err = service.RequestTerminalActivation(terminalB)
	require.Falsef(t, err != nil,
		"direct completed-puzzle activation B error = %v", err)

	assertActiveTerminalAndIdentity(t, state, "terminal-b", broadcastID, controller.SessionID, assignmentsBefore)
	slots := canonicalTerminalSlots(t, service)
	require.Falsef(t, len(slots) != 2 || slots["terminal-a"] == nil || slots["terminal-b"] == nil,
		"direct switch runtime slots = %#v, want owned A and B checkpoints", slots)
	require.Falsef(t, slots["terminal-a"].Lifecycle != domain.TerminalLifecycleSuspended || slots["terminal-b"].Lifecycle != domain.TerminalLifecycleActive,
		"direct switch lifecycle = A %q B %q", slots["terminal-a"].Lifecycle, slots["terminal-b"].Lifecycle)

	suspendedA := canonicalTerminal(t, service, "terminal-a")
	terminalARuntime.Lifecycle = ""
	suspendedA.Lifecycle = ""
	require.Falsef(t, !cmp.Equal(suspendedA, terminalARuntime),
		"switching away changed completed source gameplay runtime\nbefore: %#v\nafter:  %#v", terminalARuntime, suspendedA)

	restoredA := terminalTarget("terminal-a", "Overseer A Updated")
	state, err = service.RequestTerminalActivation(restoredA)
	require.Falsef(t, err != nil,
		"RequestTerminalActivation(restored A) error = %v", err)

	assertActiveTerminalAndIdentity(t, state, "terminal-a", broadcastID, controller.SessionID, assignmentsBefore)
	{
		creates, updates, _ := terminals.Calls()
		require.Falsef(t, creates != 2 || updates != 1,
			"runtime lifecycle calls = create/update %d/%d, want 2/1", creates, updates)
	}

	restored := canonicalTerminal(t, service, "terminal-a")
	require.Falsef(t, restored.TerminalName != "Overseer A Updated" || restored.Hack == nil || !restored.Hack.Solved || restored.Hack.GenerationID != "generation-terminal-a",
		"restored checkpoint = %#v, want updated authored metadata and preserved completed puzzle", restored)
	{

		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: "switch-late", SessionID: late.SessionID, RequestID: "switch-select-late",
			BroadcastID: broadcastID, CharacterID: arcadeID,
		})
		require.Falsef(t, !result.Accepted,
			"late assignment = %#v", result)
	}

	lateState, ok := service.PlayerSnapshot(late.SessionID)
	require.Falsef(t, !ok || lateState.Character == nil || lateState.ActiveTerminalID != "terminal-a" || lateState.Phase != domain.PlayerPhaseObserving,
		"late assignee current-terminal snapshot = %#v, ok=%t", lateState, ok)

	assignmentsWithLate := masterAssignments(service.Snapshot())

	state, err = service.RequestTerminalClear()
	require.Falsef(t, err != nil,
		"direct completed-puzzle clear error = %v", err)
	require.Falsef(t, state.Broadcast == nil || state.Broadcast.ActiveTerminalID != nil || state.Broadcast.ID != broadcastID,
		"cleared terminal state = %#v, want active broadcast with nil terminal", state.Broadcast)

	assertExactlyOneController(t, state, controller.SessionID)
	require.Falsef(t, !cmp.Equal(masterAssignments(state), assignmentsWithLate),
		"terminal clear changed claims: got %#v want %#v", masterAssignments(state), assignmentsWithLate)

	for _, sessionID := range []domain.LogicalSessionID{controller.SessionID, observer.SessionID, late.SessionID} {
		player, exists := service.PlayerSnapshot(sessionID)
		require.Falsef(t, !exists || player.Character == nil || player.ActiveTerminalID != "" || player.Phase != domain.PlayerPhaseWaiting,
			"assigned session %q after terminal clear = %#v, exists=%t", sessionID, player, exists)

	}
	for terminalID, slot := range canonicalTerminalSlots(t, service) {
		require.Falsef(t, slot.Lifecycle != domain.TerminalLifecycleSuspended,
			"cleared runtime slot %q lifecycle = %q, want suspended", terminalID, slot.Lifecycle)

	}
	require.Falsef(t, !hasClearEffectAtRevision(effects.Values(), state.Revision),
		"terminal clear emitted no revision-%d canonical clear effect", state.Revision)

}

func TestInactiveAndClearedTerminalActionsAreRejectedWithoutTouchingRuntimeSlots(t *testing.T) {
	actions := &recordingTerminalRuntime{}
	terminals := &recordingTerminalLifecycle{}
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, Runtime: actions, Terminals: terminals})
	state, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	characterID := state.Roster[0].ID
	state, err = service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	connectionID := domain.ConnectionID("inactive-controller")
	controller := service.CreateSession(connectionID)
	{
		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: connectionID, SessionID: controller.SessionID, RequestID: "inactive-select",
			BroadcastID: state.Broadcast.ID, CharacterID: characterID,
		})
		require.Falsef(t, !result.Accepted,
			"controller selection = %#v", result)
	}

	if _, err = service.RequestTerminalActivation(terminalTarget("terminal-a", "A")); err != nil {
		require.NoError(t, err)
	}
	if _, err = service.RequestTerminalActivation(terminalTarget("terminal-b", "B")); err != nil {
		require.NoError(t, err)
	}

	assertRejectedTerminalAction := func(requestID string, terminalID string) {
		t.Helper()
		beforeSlots := canonicalTerminalSlotBytes(t, service)
		beforeRevision := service.Revision()
		beforeCalls := actions.Calls()
		service.DispatchPlayerAction(connectionID, domain.RuntimeCommand{
			RequestID: domain.RequestID(requestID), BroadcastID: state.Broadcast.ID, TerminalID: terminalID,
			Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs",
		})
		result := actionResultForRequest(t, effects, requestID)
		require.Falsef(t, result.Accepted || result.Reason != domain.ActionReasonStaleTerminal || result.Revision != beforeRevision,
			"inactive action %q result = %#v, want stale-terminal revision %d", terminalID, result, beforeRevision)
		require.Falsef(t, actions.Calls() != beforeCalls || !cmp.Equal(canonicalTerminalSlotBytes(t, service), beforeSlots),
			"inactive action %q touched gameplay/runtime slots", terminalID)

	}

	assertRejectedTerminalAction("inactive-a", "terminal-a")
	if _, err = service.RequestTerminalClear(); err != nil {
		require.NoError(t, err)
	}
	assertRejectedTerminalAction("cleared-b", "terminal-b")
}

func TestUnfinishedTerminalSwitchPreserveKeepsSourceActionableAndRestoresExactCheckpoint(t *testing.T) {
	fixture := newUS8SwitchFixture(t)
	sourceBefore := canonicalTerminalBytes(t, fixture.service, "terminal-a")
	revisionBefore := fixture.service.Revision()

	state, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-b", "Terminal B"))
	require.Falsef(t, err != nil,
		"unfinished switch request error = %v", err)

	assertPendingSwitch(t, state, fixture.broadcastID, "terminal-a", "terminal-b")
	require.Falsef(t, state.Revision != revisionBefore+1 || state.PendingSwitch.SwitchID == "" || state.PendingSwitch.SwitchID == domain.SwitchID("terminal-a") || state.PendingSwitch.SwitchID == domain.SwitchID("terminal-b"),
		"pending switch identity/revision = %#v, want opaque token at %d", state.PendingSwitch, revisionBefore+1)

	switchID := state.PendingSwitch.SwitchID
	{
		got := canonicalTerminalBytes(t, fixture.service, "terminal-a")
		require.Falsef(t, !cmp.Equal(got, sourceBefore),
			"decision-required request changed source checkpoint\nbefore: %s\nafter:  %s", sourceBefore, got)
	}
	{

		slots := canonicalTerminalSlots(t, fixture.service)
		require.Falsef(t, len(slots) != 1 || slots["terminal-b"] != nil,
			"decision-required request created target prematurely: %#v", slots)
	}

	fixture.service.DispatchPlayerAction(fixture.connectionID, domain.RuntimeCommand{
		RequestID: "pending-source-action", BroadcastID: fixture.broadcastID, TerminalID: "terminal-a",
		Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs",
	})
	{
		result := actionResultForRequest(t, fixture.effects, "pending-source-action")
		require.Falsef(t, !result.Accepted,
			"source action while switch pending = %#v, want accepted", result)
	}

	sourceAfterAction := canonicalTerminalBytes(t, fixture.service, "terminal-a")
	require.False(t, cmp.Equal(sourceAfterAction, sourceBefore),
		"accepted source action while pending did not update canonical checkpoint")

	state, err = fixture.service.ResolveTerminalSwitch(switchID, domain.TerminalSwitchPreserve)
	require.Falsef(t, err != nil,
		"ResolveTerminalSwitch(preserve) error = %v", err)
	require.Falsef(t, state.PendingSwitch != nil || state.Broadcast.ActiveTerminalID == nil || *state.Broadcast.ActiveTerminalID != "terminal-b",
		"preserve resolution state = %#v", state)

	slots := canonicalTerminalSlots(t, fixture.service)
	require.Falsef(t, slots["terminal-a"] == nil || slots["terminal-a"].Lifecycle != domain.TerminalLifecycleSuspended || slots["terminal-b"] == nil || slots["terminal-b"].Lifecycle != domain.TerminalLifecycleActive,
		"preserve runtime slots = %#v", slots)

	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		runtime.Broadcast.TerminalRuntimes["terminal-b"].Hack.Solved = true
		return transition{accepted: true}
	})

	state, err = fixture.service.RequestTerminalActivation(terminalTarget("terminal-a", "Terminal A Updated"))
	require.Falsef(t, err != nil,
		"reactivate preserved A error = %v", err)
	require.Falsef(t, state.Broadcast.ActiveTerminalID == nil || *state.Broadcast.ActiveTerminalID != "terminal-a",
		"reactivated preserved source state = %#v", state.Broadcast)

	restored := canonicalTerminal(t, fixture.service, "terminal-a")
	require.Falsef(t, restored.TerminalName != "Terminal A Updated" || restored.Nav.Path[len(restored.Nav.Path)-1] != "docs" || restored.Hack == nil || restored.Hack.GenerationID != "generation-terminal-a-1",
		"reactivated preserved checkpoint = %#v", restored)
	{

		suspends, reactivates, discards := fixture.terminals.DecisionCalls()
		require.Falsef(t, suspends != 1 || reactivates != 1 || discards != 0,
			"preserve lifecycle calls suspend/reactivate/discard = %d/%d/%d", suspends, reactivates, discards)
	}

}

func TestUnfinishedTerminalSwitchCancelDiscardStaleAndDeletionGuards(t *testing.T) {
	t.Run("cancel", func(t *testing.T) {
		fixture := newUS8SwitchFixture(t)
		sourceBefore := canonicalTerminalBytes(t, fixture.service, "terminal-a")
		state, err := fixture.service.RequestTerminalClear()
		require.Falsef(t, err != nil,
			"unfinished clear request error = %v", err)

		assertPendingSwitch(t, state, fixture.broadcastID, "terminal-a", "")
		state, err = fixture.service.ResolveTerminalSwitch(state.PendingSwitch.SwitchID, domain.TerminalSwitchCancel)
		require.Falsef(t, err != nil,
			"ResolveTerminalSwitch(cancel) error = %v", err)
		require.Falsef(t, state.PendingSwitch != nil || state.Broadcast.ActiveTerminalID == nil || *state.Broadcast.ActiveTerminalID != "terminal-a",
			"cancel resolution state = %#v", state)
		{

			got := canonicalTerminalBytes(t, fixture.service, "terminal-a")
			require.Falsef(t, !cmp.Equal(got, sourceBefore),
				"cancel changed source checkpoint\nbefore: %s\nafter:  %s", sourceBefore, got)
		}

	})

	t.Run("discard and inactive rejection", func(t *testing.T) {
		fixture := newUS8SwitchFixture(t)
		firstGeneration := canonicalTerminal(t, fixture.service, "terminal-a").Hack.GenerationID
		state, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-b", "Terminal B"))
		if err != nil {
			require.NoError(t, err)
		}
		state, err = fixture.service.ResolveTerminalSwitch(state.PendingSwitch.SwitchID, domain.TerminalSwitchDiscard)
		require.Falsef(t, err != nil,
			"ResolveTerminalSwitch(discard) error = %v", err)
		require.Falsef(t, state.PendingSwitch != nil || state.Broadcast.ActiveTerminalID == nil || *state.Broadcast.ActiveTerminalID != "terminal-b",
			"discard resolution state = %#v", state)
		{

			slots := canonicalTerminalSlots(t, fixture.service)
			require.Falsef(t, slots["terminal-a"] != nil,
				"discard retained source runtime: %#v", slots)
		}

		beforeCalls := fixture.actions.Calls()
		beforeSlots := canonicalTerminalSlotBytes(t, fixture.service)
		fixture.service.DispatchPlayerAction(fixture.connectionID, domain.RuntimeCommand{
			RequestID: "discarded-source-action", BroadcastID: fixture.broadcastID, TerminalID: "terminal-a",
			Kind: domain.RuntimeCommandNavigate, Action: "back",
		})
		result := actionResultForRequest(t, fixture.effects, "discarded-source-action")
		require.Falsef(t, result.Accepted || result.Reason != domain.ActionReasonStaleTerminal || fixture.actions.Calls() != beforeCalls || !cmp.Equal(canonicalTerminalSlotBytes(t, fixture.service), beforeSlots),
			"discarded source action = %#v calls=%d", result, fixture.actions.Calls())

		fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
			runtime.Broadcast.TerminalRuntimes["terminal-b"].Hack.Solved = true
			return transition{accepted: true}
		})
		if _, err = fixture.service.RequestTerminalActivation(terminalTarget("terminal-a", "Terminal A Fresh")); err != nil {
			require.NoError(t, err)
		}
		fresh := canonicalTerminal(t, fixture.service, "terminal-a")
		require.Falsef(t, fresh.Hack == nil || fresh.Hack.GenerationID == firstGeneration,
			"discarded source was not freshly generated: old %q new %#v", firstGeneration, fresh.Hack)

	})

	t.Run("stale and deletion guards", func(t *testing.T) {
		fixture := newUS8SwitchFixture(t)
		state, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-b", "Terminal B"))
		if err != nil {
			require.NoError(t, err)
		}
		switchID := state.PendingSwitch.SwitchID
		before := fixture.service.Snapshot()
		beforeSlots := canonicalTerminalSlotBytes(t, fixture.service)
		{
			_, err = fixture.service.ResolveTerminalSwitch("unknown-switch", domain.TerminalSwitchPreserve)
			require.False(t, err == nil,
				"unknown switch decision unexpectedly resolved")
		}
		require.False(t, !cmp.Equal(fixture.service.Snapshot(), before) || !cmp.Equal(canonicalTerminalSlotBytes(t, fixture.service), beforeSlots),
			"stale decision refusal changed canonical state")
		{

			err = fixture.service.CanDeleteTerminal("terminal-a")
			require.False(t, err == nil,
				"active source terminal deletion was allowed")
		}

		if _, err = fixture.service.ResolveTerminalSwitch(switchID, domain.TerminalSwitchPreserve); err != nil {
			require.NoError(t, err)
		}
		{
			err = fixture.service.CanDeleteTerminal("terminal-a")
			require.False(t, err == nil,
				"preserved suspended terminal deletion was allowed")
		}
		{

			err = fixture.service.CanDeleteTerminal("terminal-unowned")
			require.Falsef(t, err != nil,
				"unowned terminal deletion guard error = %v", err)
		}

	})
}

func TestResetFailedHackAtomicallyReplacesOnlyActiveSlotAndSerializesDuplicates(t *testing.T) {
	fixture := newUS8SwitchFixture(t)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		active := runtime.Broadcast.TerminalRuntimes["terminal-a"]
		active.Hack.AttemptsLeft = 0
		active.Hack.Failed = true
		active.Hack.Log = []string{"TERMINAL LOCKED"}
		runtime.Broadcast.TerminalRuntimes["terminal-observer"] = testTerminalRuntime("terminal-observer")
		runtime.Broadcast.TerminalRuntimes["terminal-observer"].Lifecycle = domain.TerminalLifecycleSuspended
		return transition{accepted: true}
	})

	before := fixture.service.Snapshot()
	assignmentsBefore := masterAssignments(before)
	otherBefore := canonicalTerminalBytes(t, fixture.service, "terminal-observer")
	oldGeneration := canonicalTerminal(t, fixture.service, "terminal-a").Hack.GenerationID
	latest := terminalTarget("terminal-a", "Terminal A Latest")
	latest.HackLevel = 2
	latest.IntroText = "LATEST INTRO"
	revisionBefore := fixture.service.Revision()

	state, err := fixture.service.ResetFailedHack(latest)
	require.Falsef(t, err != nil,
		"ResetFailedHack() error = %v", err)
	require.Falsef(t, state.Revision != revisionBefore+1 || state.Broadcast == nil || state.Broadcast.ID != fixture.broadcastID || state.Broadcast.ActiveTerminalID == nil || *state.Broadcast.ActiveTerminalID != "terminal-a",
		"reset state = %#v", state)
	require.Falsef(t, !cmp.Equal(masterAssignments(state), assignmentsBefore) || state.Broadcast.ControllerSessionID == nil || before.Broadcast.ControllerSessionID == nil || *state.Broadcast.ControllerSessionID != *before.Broadcast.ControllerSessionID || !cmp.Equal(state.Sessions, before.Sessions) || !cmp.Equal(state.Roster, before.Roster),
		"reset changed identity/ownership state\nbefore=%#v\nafter=%#v", before, state)
	{

		got := canonicalTerminalBytes(t, fixture.service, "terminal-observer")
		require.Falsef(t, !cmp.Equal(got, otherBefore),
			"reset changed unrelated runtime\nbefore=%s\nafter=%s", otherBefore, got)
	}

	fresh := canonicalTerminal(t, fixture.service, "terminal-a")
	require.Falsef(t, fresh.Hack == nil || fresh.Hack.GenerationID == oldGeneration || fresh.Hack.Level != 2 || fresh.Hack.AttemptsLeft != fresh.Hack.AttemptsMax || fresh.Hack.Failed || fresh.Hack.Solved || len(fresh.Hack.Log) != 0 || fresh.TerminalName != latest.TerminalName || fresh.IntroText != latest.IntroText,
		"fresh active runtime = %#v", fresh)
	{

		_, _, resets := fixture.terminals.DecisionCalls()
		require.Falsef(t, resets != 1,
			"reset lifecycle calls = %d, want 1", resets)
	}
	require.Falsef(t, !hasLiveEffectAtRevision(fixture.effects.Values(), state.Revision, "terminal-a"),
		"reset emitted no live effect at revision %d", state.Revision)

	afterFirst := canonicalTerminalSlotBytes(t, fixture.service)
	{
		duplicate, duplicateErr := fixture.service.ResetFailedHack(latest)
		require.Falsef(t, duplicateErr == nil || duplicate.Revision != state.Revision,
			"duplicate reset = state %#v error %v", duplicate, duplicateErr)
	}
	require.False(t, fixture.service.Revision() != state.Revision || !cmp.Equal(canonicalTerminalSlotBytes(t, fixture.service), afterFirst),
		"duplicate reset mutated canonical state")

	wrong := latest
	wrong.TerminalID = "terminal-observer"
	{
		stale, staleErr := fixture.service.ResetFailedHack(wrong)
		require.Falsef(t, staleErr == nil || stale.Revision != state.Revision,
			"stale reset = state %#v error %v", stale, staleErr)
	}

}

func TestResetFailedHackSerializesConcurrentDuplicateRequests(t *testing.T) {
	fixture := newUS8SwitchFixture(t)
	fixture.service.commit(func(runtime *domain.ProcessRuntime) transition {
		active := runtime.Broadcast.TerminalRuntimes["terminal-a"]
		active.Hack.AttemptsLeft = 0
		active.Hack.Failed = true
		return transition{accepted: true}
	})
	target := terminalTarget("terminal-a", "Terminal A Retry")
	revisionBefore := fixture.service.Revision()

	const callers = 32
	results := make(chan error, callers)
	var group sync.WaitGroup
	group.Add(callers)
	for range callers {
		go func() {
			defer group.Done()
			_, err := fixture.service.ResetFailedHack(target)
			results <- err
		}()
	}
	group.Wait()
	close(results)

	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		}
	}
	require.Falsef(t, accepted != 1 || fixture.service.Revision() != revisionBefore+1,
		"concurrent resets accepted=%d revision=%d, want 1/%d", accepted, fixture.service.Revision(), revisionBefore+1)

	fresh := canonicalTerminal(t, fixture.service, "terminal-a")
	require.Falsef(t, fresh.Hack == nil || fresh.Hack.Failed || fresh.Hack.Solved || fresh.Hack.AttemptsLeft != fresh.Hack.AttemptsMax,
		"serialized reset result = %#v", fresh)
	{

		_, _, resets := fixture.terminals.DecisionCalls()
		require.Falsef(t, resets != 1,
			"concurrent reset lifecycle calls = %d, want 1", resets)
	}

}

func TestCurrentLiveForSessionResumesCoordinatorOwnedRuntimeWithoutRegeneration(t *testing.T) {
	liveService := live.New(nil, nil)
	service := New(Config{IDs: &counterIDSource{}, Runtime: liveService, Terminals: liveService})
	_, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	state, err := service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	controller := service.CreateSession("resume-controller")
	{
		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: "resume-controller", SessionID: controller.SessionID, RequestID: "resume-select",
			BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
		})
		require.Falsef(t, !result.Accepted,
			"SelectCharacter() = %#v", result)
	}

	if _, err = service.RequestTerminalActivation(terminalTarget("terminal-resume", "Resume Terminal")); err != nil {
		require.NoError(t, err)
	}

	want, revision, ok := service.CurrentLiveForSession(controller.SessionID)
	require.Falsef(t, !ok || want == nil || want.TerminalID != "terminal-resume" || want.Hack == nil,
		"CurrentLiveForSession() = %#v, %d, %v", want, revision, ok)

	want.TerminalName = "mutated detached projection"
	want.Hack.AttemptsLeft = -1
	canonical, canonicalRevision, ok := service.CurrentLiveForSession(controller.SessionID)
	require.Falsef(t, !ok || canonical == nil || canonical.TerminalName != "Resume Terminal" || canonical.Hack.AttemptsLeft < 0 || canonicalRevision != revision,
		"detached current live = %#v revision=%d ok=%v", canonical, canonicalRevision, ok)

	returnedToken, resumed := service.AttachConnection("resume-new-tab", controller.BrowserToken)
	require.Falsef(t, returnedToken != controller.BrowserToken || resumed == nil || resumed.SessionID != controller.SessionID || resumed.Role != domain.PlayerRoleActive || resumed.ActiveTerminalID != "terminal-resume",
		"recognized resume = token %q state %#v", returnedToken, resumed)

	resumedLive, _, ok := service.CurrentLiveForSession(resumed.SessionID)
	require.Falsef(t, !ok || !cmp.Equal(resumedLive, canonical),
		"resumed live regenerated or drifted\nwant=%#v\ngot=%#v", canonical, resumedLive)

	unassigned := service.CreateSession("resume-unassigned")
	{
		leaked, _, available := service.CurrentLiveForSession(unassigned.SessionID)
		require.Falsef(t, available || leaked != nil,
			"unassigned session received coordinator runtime: %#v", leaked)
	}

}

func TestForceHackSuccessMutatesOnlyCoordinatorOwnedActiveRuntime(t *testing.T) {
	liveService := live.New(nil, nil)
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{
		IDs: &counterIDSource{}, Enqueue: effects.Enqueue, Runtime: liveService,
		Terminals: liveService, TrustedHack: liveService,
	})
	_, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	state, err := service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	controller := service.CreateSession("force-controller")
	{
		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: "force-controller", SessionID: controller.SessionID, RequestID: "force-select",
			BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
		})
		require.Falsef(t, !result.Accepted,
			"SelectCharacter() = %#v", result)
	}

	if _, err = service.RequestTerminalActivation(terminalTarget("terminal-force", "Force Terminal")); err != nil {
		require.NoError(t, err)
	}
	before, beforeRevision, ok := service.CurrentLiveForSession(controller.SessionID)
	require.Falsef(t, !ok || before == nil || before.Hack == nil || before.Hack.Solved || before.Hack.Failed,
		"force precondition = %#v", before)
	{

		legacy := liveService.Snapshot()
		require.Falsef(t, legacy != nil,
			"legacy live slot unexpectedly owns coordinator runtime: %#v", legacy)
	}

	forced, accepted := service.ForceHackSuccess()
	require.Falsef(t, !accepted || forced == nil || !forced.Solved || forced.Failed || forced.AttemptsLeft != before.Hack.AttemptsLeft,
		"ForceHackSuccess() = %#v, %v", forced, accepted)

	after, afterRevision, ok := service.CurrentLiveForSession(controller.SessionID)
	require.Falsef(t, !ok || after == nil || after.Hack == nil || !after.Hack.Solved || after.Hack.Failed || afterRevision != beforeRevision+1,
		"forced canonical runtime = %#v revision=%d", after, afterRevision)
	{

		legacy := liveService.Snapshot()
		require.Falsef(t, legacy != nil,
			"trusted force populated legacy live slot: %#v", legacy)
	}
	require.Falsef(t, !hasLiveEffectAtRevision(effects.Values(), afterRevision, "terminal-force"),
		"trusted force emitted no complete live projection at revision %d", afterRevision)

	liveEffects := 0
	for _, effect := range effects.Values() {
		if effect.Revision == afterRevision && effect.Live != nil {
			liveEffects++
		}
	}
	require.Falsef(t, liveEffects != 1,
		"trusted force emitted %d live projections at revision %d, want 1", liveEffects, afterRevision)
	{

		duplicate, duplicateOK := service.ForceHackSuccess()
		require.Falsef(t, duplicateOK || duplicate != nil || service.Revision() != afterRevision,
			"duplicate force = %#v, %v revision=%d", duplicate, duplicateOK, service.Revision())
	}

	service.commit(func(runtime *domain.ProcessRuntime) transition {
		terminal := activeTerminalRuntime(runtime.Broadcast)
		terminal.Hack.Solved = false
		terminal.Hack.Failed = true
		terminal.Hack.AttemptsLeft = 0
		return transition{accepted: true}
	})
	failedRevision := service.Revision()
	{
		failed, failedOK := service.ForceHackSuccess()
		require.Falsef(t, failedOK || failed != nil || service.Revision() != failedRevision,
			"failed-puzzle force = %#v, %v revision=%d", failed, failedOK, service.Revision())
	}

}

func hasLiveEffectAtRevision(effects []Effect, revision uint64, terminalID string) bool {
	for _, effect := range effects {
		if effect.Revision == revision && effect.Live != nil && effect.Live.TerminalID == terminalID {
			return true
		}
	}
	return false
}

func TestEndAndRestartBroadcastClearsEpochStateWhileRetainingProcessIdentity(t *testing.T) {
	actions := &recordingTerminalRuntime{}
	terminals := newRecordingDecisionTerminalLifecycle()
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, Runtime: actions, Terminals: terminals})

	state, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	maraID := state.Roster[0].ID
	state, err = addCharacter(service, "Boone")
	if err != nil {
		require.NoError(t, err)
	}
	booneID := state.Roster[1].ID
	rosterBefore := append([]domain.MasterRosterEntry(nil), state.Roster...)
	state, err = service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	firstBroadcastID := state.Broadcast.ID
	firstConnection := domain.ConnectionID("lifetime-first")
	secondConnection := domain.ConnectionID("lifetime-second")
	first := service.CreateSession(firstConnection)
	second := service.CreateSession(secondConnection)
	if _, err = service.RenameLogicalSession(first.SessionID, "TABLET LEFT"); err != nil {
		require.NoError(t, err)
	}
	if _, err = service.RenameLogicalSession(second.SessionID, "TABLET RIGHT"); err != nil {
		require.NoError(t, err)
	}
	{
		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: firstConnection, SessionID: first.SessionID, RequestID: "lifetime-first-request",
			BroadcastID: firstBroadcastID, CharacterID: maraID,
		})
		require.Falsef(t, !result.Accepted,
			"first-broadcast controller selection = %#v", result)
	}
	{

		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: secondConnection, SessionID: second.SessionID, RequestID: "lifetime-second-request",
			BroadcastID: firstBroadcastID, CharacterID: booneID,
		})
		require.Falsef(t, !result.Accepted,
			"first-broadcast observer selection = %#v", result)
	}

	if _, err = service.RequestTerminalActivation(terminalTarget("lifetime-terminal-a", "Terminal A")); err != nil {
		require.NoError(t, err)
	}
	state, err = service.RequestTerminalActivation(terminalTarget("lifetime-terminal-b", "Terminal B"))
	if err != nil {
		require.NoError(t, err)
	}
	assertPendingSwitch(t, state, firstBroadcastID, "lifetime-terminal-a", "lifetime-terminal-b")
	{
		cacheCount := requestCacheCount(t, service)
		require.Falsef(t, cacheCount < 2,
			"populated first-broadcast request cache count = %d, want at least 2", cacheCount)
	}
	{

		slots := canonicalTerminalSlots(t, service)
		require.False(t, len(slots) == 0,
			"populated first broadcast has no coordinator-owned terminal runtime")
	}

	baseline := effects.Calls()
	beforeEndRevision := service.Revision()
	ended, err := service.EndBroadcast()
	require.Falsef(t, err != nil,
		"EndBroadcast() error = %v", err)
	require.Falsef(t, ended == nil || ended.Revision != beforeEndRevision+1 || ended.Broadcast != nil || ended.PendingSwitch != nil,
		"EndBroadcast() state = %#v, want no broadcast at revision %d", ended, beforeEndRevision+1)
	require.Falsef(t, !cmp.Equal(masterRosterIdentities(ended), masterRosterIdentities(&domain.MasterCoordinationState{Roster: rosterBefore})),
		"broadcast end changed retained roster\nbefore: %#v\nafter:  %#v", rosterBefore, ended.Roster)

	firstEnded := masterSession(t, ended, first.SessionID)
	secondEnded := masterSession(t, ended, second.SessionID)
	require.Falsef(t, firstEnded.FallbackName != "TABLET LEFT" || secondEnded.FallbackName != "TABLET RIGHT" || !firstEnded.Connected || !secondEnded.Connected,
		"broadcast end changed retained session identity/presence: first %#v second %#v", firstEnded, secondEnded)

	for _, session := range []domain.MasterSessionEntry{firstEnded, secondEnded} {
		require.Falsef(t, session.Character != nil || session.Role != domain.PlayerRoleUnassigned,
			"broadcast end retained assignment/controller role: %#v", session)

	}
	for _, character := range ended.Roster {
		require.Falsef(t, character.ClaimedBySessionID != nil,
			"broadcast end retained roster claim: %#v", character)

	}
	assertEndedRuntimeRoot(t, service)
	assertBroadcastEndEffects(t, effects.Values()[baseline:], ended.Revision, first.SessionID, second.SessionID)

	returnedToken, reattached := service.AttachConnection("lifetime-first-reopen", first.BrowserToken)
	require.Falsef(t, returnedToken != first.BrowserToken || reattached == nil || reattached.SessionID != first.SessionID || reattached.FallbackName != "TABLET LEFT" || reattached.Character != nil || reattached.Phase != domain.PlayerPhaseNoBroadcast,
		"same-process post-end token/session = token %q state %#v", returnedToken, reattached)

	secondBroadcast, err := service.StartBroadcast()
	require.Falsef(t, err != nil,
		"second StartBroadcast() error = %v", err)
	require.Falsef(t, secondBroadcast.Broadcast == nil || secondBroadcast.Broadcast.ID == "" || secondBroadcast.Broadcast.ID == firstBroadcastID,
		"second broadcast ID = %#v, want fresh from %q", secondBroadcast.Broadcast, firstBroadcastID)
	require.Falsef(t, secondBroadcast.Broadcast.ControllerSessionID != nil || secondBroadcast.Broadcast.ActiveTerminalID != nil || secondBroadcast.PendingSwitch != nil,
		"fresh broadcast retained controller/terminal/pending state: %#v", secondBroadcast)
	require.Falsef(t, !cmp.Equal(masterRosterIdentities(secondBroadcast), masterRosterIdentities(ended)),
		"second broadcast changed roster identities: %#v vs %#v", secondBroadcast.Roster, ended.Roster)

	for _, sessionID := range []domain.LogicalSessionID{first.SessionID, second.SessionID} {
		player, ok := service.PlayerSnapshot(sessionID)
		require.Falsef(t, !ok || player.Character != nil || player.Role != domain.PlayerRoleUnassigned || player.Phase != domain.PlayerPhaseSelecting,
			"fresh broadcast session %q = %#v, ok=%t", sessionID, player, ok)

	}
	{
		cacheCount := requestCacheCount(t, service)
		require.Falsef(t, cacheCount != 0,
			"fresh broadcast retained %d prior request results", cacheCount)
	}

	// Reusing an old request ID with a new-broadcast payload proves that the
	// per-broadcast cache was discarded instead of replaying the old result.
	newController := service.SelectCharacter(CharacterSelection{
		ConnectionID: secondConnection, SessionID: second.SessionID, RequestID: "lifetime-second-request",
		BroadcastID: secondBroadcast.Broadcast.ID, CharacterID: maraID,
	})
	require.Falsef(t, !newController.Accepted,
		"fresh-broadcast reused request ID = %#v, want new accepted selection", newController)

	final := service.Snapshot()
	assertExactlyOneController(t, final, second.SessionID)
	{
		got := masterSession(t, final, first.SessionID)
		require.Falsef(t, got.Character != nil || got.Role != domain.PlayerRoleUnassigned,
			"old controller retained ownership in fresh broadcast: %#v", got)
	}

}

func requestCacheCount(t *testing.T, service *Service) int {
	t.Helper()
	service.mu.RLock()
	defer service.mu.RUnlock()
	count := 0
	for _, session := range service.runtime.SessionsByID {
		if session != nil {
			count += len(session.RequestResults)
		}
	}
	return count
}

func masterRosterIdentities(state *domain.MasterCoordinationState) []domain.PlayerCharacter {
	identities := make([]domain.PlayerCharacter, 0)
	if state == nil {
		return identities
	}
	for _, character := range state.Roster {
		identities = append(identities, domain.PlayerCharacter{ID: character.ID, Name: character.Name})
	}
	return identities
}

func assertEndedRuntimeRoot(t *testing.T, service *Service) {
	t.Helper()
	service.mu.RLock()
	defer service.mu.RUnlock()
	require.Falsef(t, service.runtime.Broadcast != nil || service.runtime.PendingSwitch != nil,
		"ended runtime retained broadcast/pending switch: %#v", service.runtime)

	for sessionID, session := range service.runtime.SessionsByID {
		require.Falsef(t, session == nil || len(session.RequestResults) != 0,
			"ended runtime session %q request cache = %#v", sessionID, session)

	}
}

func assertBroadcastEndEffects(t *testing.T, effects []Effect, revision uint64, sessionIDs ...domain.LogicalSessionID) {
	t.Helper()
	require.Falsef(t, masterEffectCount(effects) != 1 || !hasClearEffectAtRevision(effects, revision),
		"broadcast end effects = %#v, want one master and terminal clear at revision %d", effects, revision)

	seenPlayers := make(map[domain.LogicalSessionID]bool)
	for _, effect := range effects {
		require.Falsef(t, effect.Revision != revision,
			"broadcast end effect revision = %d, want %d", effect.Revision, revision)

		if effect.Player != nil {
			require.Falsef(t, effect.Player.Character != nil || effect.Player.Role != domain.PlayerRoleUnassigned || effect.Player.Phase != domain.PlayerPhaseNoBroadcast,
				"broadcast end player effect = %#v", effect.Player)

			seenPlayers[effect.SessionID] = true
		}
	}
	for _, sessionID := range sessionIDs {
		require.Falsef(t, !seenPlayers[sessionID],
			"broadcast end omitted player context for session %q", sessionID)

	}
}

type us8SwitchFixture struct {
	service      *Service
	effects      *testutil.FakeOrderedEffectSink[Effect]
	actions      *recordingTerminalRuntime
	terminals    *recordingDecisionTerminalLifecycle
	broadcastID  domain.BroadcastID
	connectionID domain.ConnectionID
}

func newUS8SwitchFixture(t *testing.T) us8SwitchFixture {
	t.Helper()
	actions := &recordingTerminalRuntime{}
	terminals := newRecordingDecisionTerminalLifecycle()
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, Runtime: actions, Terminals: terminals})
	state, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	characterID := state.Roster[0].ID
	state, err = service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	connectionID := domain.ConnectionID("decision-controller")
	controller := service.CreateSession(connectionID)
	{
		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: connectionID, SessionID: controller.SessionID, RequestID: "decision-select",
			BroadcastID: state.Broadcast.ID, CharacterID: characterID,
		})
		require.Falsef(t, !result.Accepted,
			"controller selection = %#v", result)
	}

	if _, err = service.RequestTerminalActivation(terminalTarget("terminal-a", "Terminal A")); err != nil {
		require.NoError(t, err)
	}
	return us8SwitchFixture{service: service, effects: effects, actions: actions, terminals: terminals, broadcastID: state.Broadcast.ID, connectionID: connectionID}
}

func assertPendingSwitch(t *testing.T, state *domain.MasterCoordinationState, broadcastID domain.BroadcastID, sourceTerminalID string, targetTerminalID string) {
	t.Helper()
	require.Falsef(t, state == nil || state.Broadcast == nil || state.Broadcast.ID != broadcastID || state.Broadcast.ActiveTerminalID == nil || *state.Broadcast.ActiveTerminalID != sourceTerminalID || state.PendingSwitch == nil,
		"pending switch state = %#v", state)
	require.Falsef(t, state.PendingSwitch.BroadcastID != broadcastID || state.PendingSwitch.SourceTerminalID != sourceTerminalID,
		"pending switch source/broadcast = %#v", state.PendingSwitch)

	if targetTerminalID == "" {
		require.Falsef(t, state.PendingSwitch.TargetTerminalID != nil,
			"pending clear target = %#v, want nil", state.PendingSwitch.TargetTerminalID)

	} else {
		require.Falsef(t, state.PendingSwitch.TargetTerminalID == nil || *state.PendingSwitch.TargetTerminalID != targetTerminalID,
			"pending activation target = %#v, want %q", state.PendingSwitch.TargetTerminalID, targetTerminalID)
	}

}

type recordingDecisionTerminalLifecycle struct {
	recordingTerminalLifecycle
	muDecision  sync.Mutex
	generations map[string]int
	suspends    int
	reactivates int
	discards    int
}

type terminalDecisionLifecycleContract interface {
	TerminalRuntimeLifecycle
	SuspendRuntime(*domain.TerminalRuntime)
	ReactivateRuntime(*domain.TerminalRuntime, domain.TerminalTarget) *domain.PublicLiveState
	DiscardRuntime(domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState)
	ResetFailedHack(*domain.TerminalRuntime, domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState)
}

var _ terminalDecisionLifecycleContract = (*recordingDecisionTerminalLifecycle)(nil)

func newRecordingDecisionTerminalLifecycle() *recordingDecisionTerminalLifecycle {
	return &recordingDecisionTerminalLifecycle{generations: make(map[string]int)}
}

func (lifecycle *recordingDecisionTerminalLifecycle) CreateRuntime(target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	lifecycle.muDecision.Lock()
	lifecycle.generations[target.TerminalID]++
	generation := lifecycle.generations[target.TerminalID]
	lifecycle.muDecision.Unlock()
	runtime, _ := lifecycle.recordingTerminalLifecycle.CreateRuntime(target)
	runtime.Hack.Solved = false
	runtime.Hack.Failed = false
	runtime.Hack.Level = target.HackLevel
	runtime.Hack.AttemptsLeft = runtime.Hack.AttemptsMax
	runtime.Hack.Log = []string{}
	runtime.Hack.GenerationID = fmt.Sprintf("generation-%s-%d", target.TerminalID, generation)
	return runtime, publicTerminalRuntime(runtime)
}

func (lifecycle *recordingDecisionTerminalLifecycle) SuspendRuntime(runtime *domain.TerminalRuntime) {
	lifecycle.muDecision.Lock()
	lifecycle.suspends++
	lifecycle.muDecision.Unlock()
	runtime.Lifecycle = domain.TerminalLifecycleSuspended
}

func (lifecycle *recordingDecisionTerminalLifecycle) ReactivateRuntime(runtime *domain.TerminalRuntime, target domain.TerminalTarget) *domain.PublicLiveState {
	lifecycle.muDecision.Lock()
	lifecycle.reactivates++
	lifecycle.muDecision.Unlock()
	runtime.Lifecycle = domain.TerminalLifecycleActive
	return lifecycle.UpdateRuntime(runtime, target)
}

func (lifecycle *recordingDecisionTerminalLifecycle) DiscardRuntime(target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	lifecycle.muDecision.Lock()
	lifecycle.discards++
	lifecycle.muDecision.Unlock()
	return lifecycle.CreateRuntime(target)
}

func (lifecycle *recordingDecisionTerminalLifecycle) ResetFailedHack(runtime *domain.TerminalRuntime, target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	if runtime == nil || runtime.TerminalID != target.TerminalID || runtime.Hack == nil || !runtime.Hack.Failed || runtime.Hack.Solved {
		return nil, nil
	}
	lifecycle.muDecision.Lock()
	lifecycle.discards++
	lifecycle.muDecision.Unlock()
	return lifecycle.CreateRuntime(target)
}

func (lifecycle *recordingDecisionTerminalLifecycle) DecisionCalls() (int, int, int) {
	lifecycle.muDecision.Lock()
	defer lifecycle.muDecision.Unlock()
	return lifecycle.suspends, lifecycle.reactivates, lifecycle.discards
}

func terminalTarget(id string, name string) domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: id, TerminalName: name, HackLevel: 1, IntroText: "WELCOME " + id,
		Tree: testTerminalRuntime(id).Tree,
	}
}

func assertActiveTerminalAndIdentity(t *testing.T, state *domain.MasterCoordinationState, terminalID string, broadcastID domain.BroadcastID, controllerID domain.LogicalSessionID, assignments map[domain.LogicalSessionID]domain.CharacterID) {
	t.Helper()
	require.Falsef(t, state == nil || state.Broadcast == nil || state.Broadcast.ID != broadcastID || state.Broadcast.ActiveTerminalID == nil || *state.Broadcast.ActiveTerminalID != terminalID,
		"active terminal state = %#v, want broadcast %q terminal %q", state, broadcastID, terminalID)

	assertExactlyOneController(t, state, controllerID)
	require.Falsef(t, !cmp.Equal(masterAssignments(state), assignments),
		"terminal transition changed assignments: got %#v want %#v", masterAssignments(state), assignments)

}

func canonicalTerminalSlots(t *testing.T, service *Service) map[string]*domain.TerminalRuntime {
	t.Helper()
	service.mu.RLock()
	defer service.mu.RUnlock()
	result := make(map[string]*domain.TerminalRuntime)
	if service.runtime.Broadcast == nil {
		return result
	}
	for terminalID, runtime := range service.runtime.Broadcast.TerminalRuntimes {
		result[terminalID] = cloneTerminalRuntime(runtime)
	}
	return result
}

func canonicalTerminalSlotBytes(t *testing.T, service *Service) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	for terminalID := range canonicalTerminalSlots(t, service) {
		result[terminalID] = canonicalTerminalBytes(t, service, terminalID)
	}
	return result
}

func hasClearEffectAtRevision(effects []Effect, revision uint64) bool {
	for _, effect := range effects {
		if effect.Revision == revision && effect.ClearLiveTerminal && effect.Live == nil {
			return true
		}
	}
	return false
}

type recordingTerminalLifecycle struct {
	mu       sync.Mutex
	creates  int
	updates  int
	projects int
}

func (lifecycle *recordingTerminalLifecycle) CreateRuntime(target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	lifecycle.mu.Lock()
	lifecycle.creates++
	lifecycle.mu.Unlock()
	runtime := testTerminalRuntime(target.TerminalID)
	runtime.TerminalName = target.TerminalName
	runtime.Tree = domain.CloneContentNode(target.Tree)
	runtime.HackLevel = target.HackLevel
	runtime.IntroText = target.IntroText
	runtime.Hack.GenerationID = "generation-" + target.TerminalID
	runtime.Hack.Solved = true
	runtime.Hack.Log = []string{"ACCESS GRANTED"}
	return runtime, publicTerminalRuntime(runtime)
}

func (lifecycle *recordingTerminalLifecycle) UpdateRuntime(runtime *domain.TerminalRuntime, target domain.TerminalTarget) *domain.PublicLiveState {
	lifecycle.mu.Lock()
	lifecycle.updates++
	lifecycle.mu.Unlock()
	runtime.TerminalName = target.TerminalName
	runtime.Tree = domain.CloneContentNode(target.Tree)
	runtime.HackLevel = target.HackLevel
	runtime.IntroText = target.IntroText
	runtime.Nav = nav.Revalidate(runtime.Nav, runtime.Tree)
	return publicTerminalRuntime(runtime)
}

func (lifecycle *recordingTerminalLifecycle) ProjectRuntime(runtime *domain.TerminalRuntime) *domain.PublicLiveState {
	lifecycle.mu.Lock()
	lifecycle.projects++
	lifecycle.mu.Unlock()
	return publicTerminalRuntime(runtime)
}

func (lifecycle *recordingTerminalLifecycle) Calls() (int, int, int) {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.creates, lifecycle.updates, lifecycle.projects
}

func assertPresenceOnlyEffects(t *testing.T, effects []Effect, revision uint64) {
	t.Helper()
	require.Falsef(t, masterEffectCount(effects) != 1,
		"presence transition master effects = %d, want exactly 1 in %#v", masterEffectCount(effects), effects)

	for _, effect := range effects {
		require.Falsef(t, effect.Revision != revision,
			"presence effect revision = %d, want %d", effect.Revision, revision)
		require.Falsef(t, effect.Live != nil || effect.Hack != nil || effect.Result != nil || effect.ClearLiveTerminal || effect.TerminalID != "" || effect.ConnectionID != "",
			"presence transition emitted gameplay/result payload: %#v", effect)
		require.Falsef(t, effect.Master == nil && effect.Player == nil && effect.Update == nil,
			"presence transition emitted empty effect: %#v", effect)

	}
}

func TestOverseerRosterAndAssignmentCorrectionsPreserveRuntime(t *testing.T) {
	runtime := &recordingTerminalRuntime{}
	service := New(Config{IDs: &counterIDSource{}, Runtime: runtime})
	state, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	maraID := state.Roster[0].ID
	state, err = addCharacter(service, "Boone")
	if err != nil {
		require.NoError(t, err)
	}
	booneID := state.Roster[1].ID
	state, err = addCharacter(service, "Mara")
	require.Falsef(t, err != nil,
		"duplicate character names must remain valid: %v", err)
	require.Falsef(t, len(state.Roster) != 3 || state.Roster[2].Name != "Mara" || state.Roster[2].ID == maraID,
		"duplicate-name roster entry = %#v, want a distinct stable identity", state.Roster)

	duplicateMaraID := state.Roster[2].ID
	state, err = service.UpdateCharacter(domain.CharacterUpdatePayload{
		CharacterID: maraID, Name: "  Mara Voss  ", Intelligence: 9,
		HackerPerkAvailable: true, ExpectedRevision: service.Revision(),
	})
	require.Falsef(t, err != nil,
		"UpdateCharacter() error = %v", err)
	require.Falsef(t, state.Roster[0].ID != maraID || state.Roster[0].Name != "Mara Voss" || state.Roster[0].Intelligence != 9 || !state.Roster[0].HackerPerkAvailable,
		"updated roster entry = %#v, want stable ID and complete trimmed profile", state.Roster[0])
	assertRejectedCoordinationMutation(t, service, func() error {
		_, updateErr := service.UpdateCharacter(domain.CharacterUpdatePayload{
			CharacterID: maraID, Name: strings.Repeat("x", 81), Intelligence: 9,
			HackerPerkAvailable: true, ExpectedRevision: service.Revision(),
		})
		return updateErr
	})

	_, err = service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	first := service.CreateSession("gm-first")
	second := service.CreateSession("gm-second")
	third := service.CreateSession("gm-third")
	terminalID := "terminal-roster-neutral"
	service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.ActiveTerminalID = &terminalID
		root.Broadcast.TerminalRuntimes[terminalID] = testTerminalRuntime(terminalID)
		return transition{accepted: true}
	})
	runtimeBefore := canonicalTerminalBytes(t, service, terminalID)

	state, err = service.RenameLogicalSession(first.SessionID, "  TABLET LEFT  ")
	require.Falsef(t, err != nil,
		"RenameLogicalSession() error = %v", err)
	{

		got := masterSession(t, state, first.SessionID).FallbackName
		require.Falsef(t, got != "TABLET LEFT",
			"renamed fallback = %q, want TABLET LEFT", got)
	}

	assertRejectedCoordinationMutation(t, service, func() error {
		_, renameErr := service.RenameLogicalSession(second.SessionID, "TABLET LEFT")
		return renameErr
	})
	assertRejectedCoordinationMutation(t, service, func() error {
		_, renameErr := service.RenameLogicalSession(second.SessionID, "   ")
		return renameErr
	})

	state, err = service.AssignCharacter(first.SessionID, maraID)
	require.Falsef(t, err != nil,
		"AssignCharacter(first) error = %v", err)
	require.Falsef(t, state.Broadcast.ControllerSessionID == nil || *state.Broadcast.ControllerSessionID != first.SessionID,
		"first GM assignment controller = %#v, want %q", state.Broadcast, first.SessionID)

	state, err = service.AssignCharacter(second.SessionID, booneID)
	require.Falsef(t, err != nil,
		"AssignCharacter(second) error = %v", err)

	assertExclusiveClaimInvariants(t, state)
	{
		got := masterSession(t, state, second.SessionID).Role
		require.Falsef(t, got != domain.PlayerRoleObserver,
			"second GM assignment role = %q, want observer", got)
	}

	assertRejectedCoordinationMutation(t, service, func() error {
		_, assignErr := service.AssignCharacter(third.SessionID, booneID)
		return assignErr
	})
	assertRejectedCoordinationMutation(t, service, func() error {
		_, deleteErr := service.DeleteCharacter(domain.CharacterDeletePayload{
			CharacterID: maraID, ExpectedRevision: service.Revision(),
		})
		return deleteErr
	})

	state, err = service.MoveCharacter(maraID, third.SessionID)
	require.Falsef(t, err != nil,
		"MoveCharacter() error = %v", err)

	assertExclusiveClaimInvariants(t, state)
	require.Falsef(t, state.Broadcast.ControllerSessionID != nil,
		"moving the controller character retained or promoted control: %#v", state.Broadcast)
	{

		got := masterSession(t, state, first.SessionID)
		require.Falsef(t, got.Character != nil || got.Role != domain.PlayerRoleUnassigned,
			"former owner after move = %#v, want unassigned", got)
	}
	{

		got := masterSession(t, state, third.SessionID)
		require.Falsef(t, got.Character == nil || got.Character.ID != maraID || got.Role != domain.PlayerRoleObserver,
			"move destination = %#v, want observer owning stable Mara ID", got)
	}

	state, err = service.ReleaseCharacter(second.SessionID)
	require.Falsef(t, err != nil,
		"ReleaseCharacter() error = %v", err)

	assertExclusiveClaimInvariants(t, state)
	{
		got := masterSession(t, state, second.SessionID)
		require.Falsef(t, got.Character != nil || got.Role != domain.PlayerRoleUnassigned,
			"released session = %#v, want unassigned", got)
	}

	assertRejectedCoordinationMutation(t, service, func() error {
		_, deleteErr := service.DeleteCharacter(domain.CharacterDeletePayload{
			CharacterID: booneID, ExpectedRevision: service.Revision(),
		})
		return deleteErr
	})
	require.Equal(t, duplicateMaraID, state.Roster[2].ID)
	require.Falsef(t, runtime.RandomCalls() != 0,
		"roster/assignment commands consumed runtime randomness %d times", runtime.RandomCalls())
	{

		got := canonicalTerminalBytes(t, service, terminalID)
		require.Falsef(t, !cmp.Equal(got, runtimeBefore),
			"roster/assignment commands mutated canonical terminal/puzzle\nbefore: %s\nafter:  %s", runtimeBefore, got)
	}

}

func assertRejectedCoordinationMutation(t *testing.T, service *Service, command func() error) {
	t.Helper()
	before := service.Snapshot()
	{
		err := command()
		require.False(t, err == nil,
			"coordination command unexpectedly succeeded")
	}

	after := service.Snapshot()
	require.Falsef(t, !cmp.Equal(after, before),
		"rejected coordination command changed state\nbefore: %#v\nafter:  %#v", before, after)

}

func masterEffectCount(effects []Effect) int {
	count := 0
	for _, effect := range effects {
		if effect.Master != nil {
			count++
		}
	}
	return count
}

func contentNodeByIDForControlTest(root domain.ContentNode, nodeID string) *domain.ContentNode {
	if root.ID == nodeID {
		return &root
	}
	for _, child := range root.Children {
		if found := contentNodeByIDForControlTest(child, nodeID); found != nil {
			return found
		}
	}
	return nil
}

type us2Fixture struct {
	service              *Service
	effects              *testutil.FakeOrderedEffectSink[Effect]
	broadcastID          domain.BroadcastID
	terminalID           string
	controllerConnection domain.ConnectionID
	observerConnection   domain.ConnectionID
	unassignedConnection domain.ConnectionID
	controllerSession    domain.LogicalSessionID
	observerSession      domain.LogicalSessionID
	unassignedSession    domain.LogicalSessionID
	controllerToken      domain.BrowserToken
	observerToken        domain.BrowserToken
	unassignedToken      domain.BrowserToken
}

type commandExecutionFixture struct {
	service              *Service
	effects              *testutil.FakeOrderedEffectSink[Effect]
	runtime              *recordingTerminalRuntime
	store                *recordingCommandStateStore
	broadcastID          domain.BroadcastID
	terminalID           string
	commandID            string
	controllerConnection domain.ConnectionID
}

func newCommandExecutionFixture(t *testing.T, store *recordingCommandStateStore) commandExecutionFixture {
	t.Helper()
	runtime := &recordingTerminalRuntime{}
	base := newUS2Fixture(t, runtime)
	base.service.commandStateStore = store
	commandID := "command-open-doors"
	base.service.commit(func(root *domain.ProcessRuntime) transition {
		terminal := root.Broadcast.TerminalRuntimes[base.terminalID]
		terminal.Tree.Children = append(terminal.Tree.Children, domain.ContentNode{
			ID: commandID, Type: domain.NodeCommand, Name: "Open doors", Text: "Doors opened",
			StateChange: &domain.StateChangeConfig{
				CompletedName: "Doors open", ConfirmationText: "Open the doors?",
			},
		})
		return transition{accepted: true}
	})
	return commandExecutionFixture{
		service: base.service, effects: base.effects, runtime: runtime, store: store,
		broadcastID: base.broadcastID, terminalID: base.terminalID, commandID: commandID,
		controllerConnection: base.controllerConnection,
	}
}

func TestLinkedCommandCreatesOneReplaySafePendingAndResolvesAtomically(t *testing.T) {
	t.Parallel()

	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	lifecycle := newRecordingDecisionTerminalLifecycle()
	fixture.service.terminals = lifecycle
	catalog := &recordingTerminalCatalog{transitions: map[string]domain.TerminalTransitionTarget{
		fixture.terminalID + "/linked-command": {
			SourceTerminalID: fixture.terminalID, SourceTerminalName: "Terminal 1",
			CommandID: "linked-command", CommandName: "Open B", Target: terminalTarget("terminal-b", "Terminal B"),
		},
	}}
	fixture.service.terminalCatalog = catalog
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children = append(root.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children, domain.ContentNode{
			ID: "linked-command", Type: domain.NodeCommand, Name: "Open B",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-b"},
		})
		return transition{accepted: true}
	})
	command := domain.RuntimeCommand{
		RequestID: "linked-request", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "linked-command", PayloadFingerprint: "linked-fingerprint",
	}
	observerCommand := command
	observerCommand.RequestID, observerCommand.PayloadFingerprint = "observer-linked", "observer-linked"
	beforeObserver := fixture.service.Snapshot()
	assert.Equal(t, domain.ActionReasonNotController, fixture.service.DispatchPlayerAction(fixture.observerConnection, observerCommand).Reason)
	require.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)
	assert.Equal(t, beforeObserver.Broadcast, fixture.service.Snapshot().Broadcast)
	unassignedCommand := command
	unassignedCommand.RequestID, unassignedCommand.PayloadFingerprint = "unassigned-linked", "unassigned-linked"
	assert.Equal(t, domain.ActionReasonUnassigned, fixture.service.DispatchPlayerAction(fixture.unassignedConnection, unassignedCommand).Reason)
	require.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)

	result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
	require.True(t, result.Accepted)
	first := fixture.service.Snapshot().PendingTerminalNavigation
	require.NotNil(t, first)
	assert.Equal(t, domain.TerminalNavigationForward, first.Direction)
	assert.Equal(t, "terminal-b", first.TargetTerminalID)
	assert.Zero(t, first.RouteDepth)
	assert.Equal(t, 0, runtime.Calls(), "linked command must not reach ordinary gameplay")
	handle := domain.RecognitionHandle(fixture.controllerToken)
	reconnected, err := fixture.service.AttachSubscription("linked-reconnect", &handle)
	require.NoError(t, err)
	require.GreaterOrEqual(t, reconnected.Revision, result.Revision)
	require.NotNil(t, reconnected.Terminal.Live)
	require.NotNil(t, reconnected.Terminal.Live.TerminalNavigation)
	require.NotNil(t, reconnected.Terminal.Live.TerminalNavigation.Pending)
	assert.Equal(t, domain.TerminalNavigationForward, reconnected.Terminal.Live.TerminalNavigation.Pending.Direction)
	assert.Equal(t, "terminal-b", reconnected.Terminal.Live.TerminalNavigation.Pending.TargetTerminalID)
	fixture.service.DetachConnection("linked-reconnect")

	for range 20 {
		replayed := fixture.service.DispatchPlayerAction(fixture.controllerConnection, command)
		assert.Equal(t, result, replayed)
		assert.Equal(t, first.RequestID, fixture.service.Snapshot().PendingTerminalNavigation.RequestID)
	}
	competing := command
	competing.RequestID, competing.PayloadFingerprint = "competing", "competing-fingerprint"
	competing.Action, competing.NodeID = "back", ""
	assert.Equal(t, domain.ActionReasonConflict, fixture.service.DispatchPlayerAction(fixture.controllerConnection, competing).Reason)

	rejected, err := fixture.service.ResolveTerminalNavigation(first.RequestID, domain.TerminalNavigationReject)
	require.NoError(t, err)
	require.Nil(t, rejected.PendingTerminalNavigation)
	require.Equal(t, fixture.terminalID, *rejected.Broadcast.ActiveTerminalID)

	command.RequestID, command.PayloadFingerprint = "linked-request-approve", "linked-fingerprint-approve"
	require.True(t, fixture.service.DispatchPlayerAction(fixture.controllerConnection, command).Accepted)
	pending := fixture.service.Snapshot().PendingTerminalNavigation
	require.NotNil(t, pending)
	approved, err := fixture.service.ResolveTerminalNavigation(pending.RequestID, domain.TerminalNavigationApprove)
	require.NoError(t, err)
	require.Equal(t, "terminal-b", *approved.Broadcast.ActiveTerminalID)
	require.Nil(t, approved.PendingTerminalNavigation)
	fixture.service.mu.RLock()
	assert.Len(t, fixture.service.runtime.Broadcast.Route, 1)
	assert.Equal(t, domain.TerminalLifecycleSuspended, fixture.service.runtime.Broadcast.TerminalRuntimes[fixture.terminalID].Lifecycle)
	assert.Nil(t, fixture.service.runtime.PendingSwitch)
	assert.Equal(t, domain.NavState{Path: []string{"root"}, Mode: "list"}, fixture.service.runtime.Broadcast.TerminalRuntimes["terminal-b"].Nav)
	fixture.service.mu.RUnlock()
}

func TestConcurrentLinkedRequestsCreateExactlyOnePendingDecision(t *testing.T) {
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	fixture.service.terminals = newRecordingDecisionTerminalLifecycle()
	fixture.service.terminalCatalog = &recordingTerminalCatalog{transitions: map[string]domain.TerminalTransitionTarget{
		fixture.terminalID + "/linked-command": {
			SourceTerminalID: fixture.terminalID, SourceTerminalName: "Terminal 1",
			CommandID: "linked-command", CommandName: "Open B", Target: terminalTarget("terminal-b", "Terminal B"),
		},
	}}
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children = append(root.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children, domain.ContentNode{
			ID: "linked-command", Type: domain.NodeCommand, Name: "Open B",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-b"},
		})
		return transition{accepted: true}
	})

	start := make(chan struct{})
	results := make(chan domain.ActionResult, 2)
	for _, requestID := range []string{"concurrent-a", "concurrent-b"} {
		go func() {
			<-start
			results <- fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
				RequestID: requestID, BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
				Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "linked-command",
			})
		}()
	}
	close(start)
	first, second := <-results, <-results
	reasons := []domain.ActionReason{first.Reason, second.Reason}
	assert.ElementsMatch(t, []domain.ActionReason{domain.ActionReasonAccepted, domain.ActionReasonConflict}, reasons)
	require.NotNil(t, fixture.service.Snapshot().PendingTerminalNavigation)
	assert.Equal(t, 0, runtime.Calls())
}

func TestLinkedCommandWithStaleCatalogTargetFailsWithoutPending(t *testing.T) {
	t.Parallel()
	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.terminalCatalog = &recordingTerminalCatalog{}
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children = append(root.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children, domain.ContentNode{
			ID: "stale-link", Type: domain.NodeCommand, Name: "Missing", TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "missing"},
		})
		return transition{accepted: true}
	})
	result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "stale-link", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "stale-link", PayloadFingerprint: "stale-link",
	})
	assert.Equal(t, domain.ActionReasonInvalidAction, result.Reason)
	require.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)
}

func TestStaleForwardApprovalClearsOnlyPendingAndPublishesTypedNotice(t *testing.T) {
	t.Parallel()
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	fixture.service.terminals = newRecordingDecisionTerminalLifecycle()
	catalog := &recordingTerminalCatalog{transitions: map[string]domain.TerminalTransitionTarget{
		fixture.terminalID + "/linked-command": {
			SourceTerminalID: fixture.terminalID, SourceTerminalName: "Terminal 1",
			CommandID: "linked-command", CommandName: "Open B", Target: terminalTarget("terminal-b", "Terminal B"),
		},
	}}
	fixture.service.terminalCatalog = catalog
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children = append(root.Broadcast.TerminalRuntimes[fixture.terminalID].Tree.Children, domain.ContentNode{
			ID: "linked-command", Type: domain.NodeCommand, Name: "Open B",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-b"},
		})
		return transition{accepted: true}
	})
	result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "stale-approval", BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "linked-command",
	})
	require.True(t, result.Accepted)
	pending := fixture.service.Snapshot().PendingTerminalNavigation
	require.NotNil(t, pending)
	catalog.transitions[fixture.terminalID+"/linked-command"] = domain.TerminalTransitionTarget{
		SourceTerminalID: fixture.terminalID, SourceTerminalName: "Terminal 1",
		CommandID: "linked-command", CommandName: "Open C", Target: terminalTarget("terminal-c", "Terminal C"),
	}
	state, err := fixture.service.ResolveTerminalNavigation(pending.RequestID, domain.TerminalNavigationApprove)
	require.Error(t, err)
	require.Nil(t, state.PendingTerminalNavigation)
	require.NotNil(t, state.TerminalNavigationNotice)
	assert.Equal(t, domain.TerminalNavigationNoticeTargetChanged, state.TerminalNavigationNotice.Reason)
	require.Equal(t, fixture.terminalID, *state.Broadcast.ActiveTerminalID)
	fixture.service.mu.RLock()
	require.Empty(t, fixture.service.runtime.Broadcast.Route)
	fixture.service.mu.RUnlock()
}

func TestSameGroupForwardRequestRequiresControllerAndCreatesOnePending(t *testing.T) {
	t.Parallel()

	fixture, catalog := newGroupAwareForwardFixture(t)
	before := fixture.service.Snapshot()

	observer := fixture.service.DispatchPlayerAction(
		fixture.observerConnection,
		groupAwareForwardCommand(fixture, "same-group-observer"),
	)
	assert.Equal(t, domain.ActionReasonNotController, observer.Reason)
	assert.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)
	assert.Equal(t, fixture.terminalID, *fixture.service.Snapshot().Broadcast.ActiveTerminalID)

	catalog.groups = []domain.TerminalGroup{
		{ID: "source", Name: "Source", TerminalIDs: []string{fixture.terminalID}},
		{ID: "target", Name: "Target", TerminalIDs: []string{"terminal-b"}},
	}
	crossGroup := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		groupAwareForwardCommand(fixture, "cross-group-controller"),
	)
	assert.Equal(t, domain.ActionReasonInvalidAction, crossGroup.Reason)
	assert.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)
	assert.Equal(t, fixture.terminalID, *fixture.service.Snapshot().Broadcast.ActiveTerminalID)
	fixture.service.mu.RLock()
	assert.Empty(t, fixture.service.runtime.Broadcast.Route)
	fixture.service.mu.RUnlock()

	catalog.groups = []domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{fixture.terminalID, "terminal-b"}},
	}
	requested := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		groupAwareForwardCommand(fixture, "same-group-controller"),
	)
	require.True(t, requested.Accepted)
	pending := fixture.service.Snapshot().PendingTerminalNavigation
	require.NotNil(t, pending)
	assert.Equal(t, domain.TerminalNavigationForward, pending.Direction)
	assert.Equal(t, fixture.terminalID, pending.SourceTerminalID)
	assert.Equal(t, "terminal-b", pending.TargetTerminalID)
	assert.Equal(t, fixture.terminalID, *fixture.service.Snapshot().Broadcast.ActiveTerminalID)
	fixture.service.mu.RLock()
	assert.Empty(t, fixture.service.runtime.Broadcast.Route)
	fixture.service.mu.RUnlock()

	competing := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		groupAwareForwardCommand(fixture, "same-group-competing"),
	)
	assert.Equal(t, domain.ActionReasonConflict, competing.Reason)
	assert.Equal(t, pending.RequestID, fixture.service.Snapshot().PendingTerminalNavigation.RequestID)
	assert.Greater(t, fixture.service.Revision(), before.Revision)
}

func TestSameGroupForwardRejectAndCloseLeaveActiveTerminalAndRouteUnchanged(t *testing.T) {
	t.Parallel()

	for _, resolution := range []string{"explicit reject", "dialog close maps to reject"} {
		t.Run(resolution, func(t *testing.T) {
			t.Parallel()

			fixture, _ := newGroupAwareForwardFixture(t)
			selected := fixture.service.DispatchPlayerAction(
				fixture.controllerConnection,
				groupAwareForwardCommand(fixture, "same-group-"+strings.ReplaceAll(resolution, " ", "-")),
			)
			require.True(t, selected.Accepted)
			pending := fixture.service.Snapshot().PendingTerminalNavigation
			require.NotNil(t, pending)

			state, err := fixture.service.ResolveTerminalNavigation(
				pending.RequestID,
				domain.TerminalNavigationReject,
			)
			require.NoError(t, err)
			assert.Nil(t, state.PendingTerminalNavigation)
			assert.Equal(t, fixture.terminalID, *state.Broadcast.ActiveTerminalID)
			fixture.service.mu.RLock()
			assert.Empty(t, fixture.service.runtime.Broadcast.Route)
			fixture.service.mu.RUnlock()
		})
	}
}

func TestSameGroupForwardApprovalAddsExactlyOneRoutePoint(t *testing.T) {
	t.Parallel()

	fixture, _ := newGroupAwareForwardFixture(t)
	selected := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		groupAwareForwardCommand(fixture, "same-group-approve"),
	)
	require.True(t, selected.Accepted)
	pending := fixture.service.Snapshot().PendingTerminalNavigation
	require.NotNil(t, pending)

	approved, err := fixture.service.ResolveTerminalNavigation(
		pending.RequestID,
		domain.TerminalNavigationApprove,
	)
	require.NoError(t, err)
	assert.Nil(t, approved.PendingTerminalNavigation)
	assert.Equal(t, "terminal-b", *approved.Broadcast.ActiveTerminalID)
	fixture.service.mu.RLock()
	require.Len(t, fixture.service.runtime.Broadcast.Route, 1)
	point := fixture.service.runtime.Broadcast.Route[0]
	assert.Equal(t, fixture.terminalID, point.TerminalID)
	assert.Equal(t, "linked-command", point.CommandID)
	fixture.service.mu.RUnlock()

	repeated, repeatedErr := fixture.service.ResolveTerminalNavigation(
		pending.RequestID,
		domain.TerminalNavigationApprove,
	)
	require.ErrorContains(t, repeatedErr, "stale")
	assert.Equal(t, "terminal-b", *repeated.Broadcast.ActiveTerminalID)
	fixture.service.mu.RLock()
	assert.Len(t, fixture.service.runtime.Broadcast.Route, 1)
	fixture.service.mu.RUnlock()
}

func TestSameGroupForwardApprovalRejectsStaleMembershipBeforeActivation(t *testing.T) {
	t.Parallel()

	fixture, catalog := newGroupAwareForwardFixture(t)
	selected := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		groupAwareForwardCommand(fixture, "same-group-stale-membership"),
	)
	require.True(t, selected.Accepted)
	pending := fixture.service.Snapshot().PendingTerminalNavigation
	require.NotNil(t, pending)

	catalog.groups = []domain.TerminalGroup{
		{ID: "source", Name: "Source", TerminalIDs: []string{fixture.terminalID}},
		{ID: "target", Name: "Target", TerminalIDs: []string{"terminal-b"}},
	}
	state, err := fixture.service.ResolveTerminalNavigation(
		pending.RequestID,
		domain.TerminalNavigationApprove,
	)
	require.Error(t, err)
	assert.Nil(t, state.PendingTerminalNavigation)
	assert.Equal(t, fixture.terminalID, *state.Broadcast.ActiveTerminalID)
	require.NotNil(t, state.TerminalNavigationNotice)
	assert.Equal(t, domain.TerminalNavigationNoticeTargetChanged, state.TerminalNavigationNotice.Reason)
	fixture.service.mu.RLock()
	assert.Empty(t, fixture.service.runtime.Broadcast.Route)
	assert.NotContains(t, fixture.service.runtime.Broadcast.TerminalRuntimes, "terminal-b")
	fixture.service.mu.RUnlock()
}

func TestFreshBroadcastStartSeedsOrderedPrefixForFirstMiddleAndLastMembers(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		terminal  string
		wantRoute []string
	}{
		{name: "first", terminal: "terminal-a", wantRoute: []string{}},
		{name: "middle", terminal: "terminal-c", wantRoute: []string{"terminal-a", "terminal-b"}},
		{name: "last", terminal: "terminal-d", wantRoute: []string{"terminal-a", "terminal-b", "terminal-c"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture, _ := newOrderedGroupStartFixture(t)
			state, err := fixture.service.RequestTerminalActivation(terminalTarget(test.terminal, test.terminal))
			require.NoError(t, err)
			require.Equal(t, test.terminal, *state.Broadcast.ActiveTerminalID)

			fixture.service.mu.RLock()
			route := append([]domain.TerminalReturnPoint(nil), fixture.service.runtime.Broadcast.Route...)
			initialTerminalID := fixture.service.runtime.Broadcast.InitialTerminalID
			initialGroupID := fixture.service.runtime.Broadcast.InitialTerminalGroupID
			initialGroupPosition := fixture.service.runtime.Broadcast.InitialTerminalGroupPosition
			fixture.service.mu.RUnlock()
			assert.Equal(t, test.terminal, initialTerminalID)
			assert.Equal(t, "ordered-route", initialGroupID)
			assert.Equal(t, slices.Index([]string{"terminal-a", "terminal-b", "terminal-c", "terminal-d"}, test.terminal), initialGroupPosition)
			require.Len(t, route, len(test.wantRoute))
			for index, terminalID := range test.wantRoute {
				point := route[index]
				assert.Equal(t, terminalID, point.TerminalID)
				assert.Equal(t, domain.TerminalReturnInitialPrefix, point.Origin)
				assert.Equal(t, "ordered-route", point.GroupID)
				assert.Equal(t, index, point.GroupPosition)
				assert.Equal(t, "root", point.FolderID)
			}
		})
	}
}

func TestSeededReturnRejectAndClosePreserveLIFOThenApprovalsReachFirstMember(t *testing.T) {
	t.Parallel()

	for _, resolution := range []string{"explicit reject", "dialog close maps to reject"} {
		t.Run(resolution, func(t *testing.T) {
			t.Parallel()

			fixture, _ := newOrderedGroupStartFixture(t)
			_, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-c", "Terminal C"))
			require.NoError(t, err)
			pending := requestOrderedGroupReturn(t, fixture, "terminal-c", "seeded-"+strings.ReplaceAll(resolution, " ", "-"))
			assert.Equal(t, "terminal-b", pending.TargetTerminalID)
			assert.Equal(t, domain.TerminalReturnInitialPrefix, pending.ReturnPoint.Origin)

			state, resolveErr := fixture.service.ResolveTerminalNavigation(
				pending.RequestID,
				domain.TerminalNavigationReject,
			)
			require.NoError(t, resolveErr)
			assert.Nil(t, state.PendingTerminalNavigation)
			assert.Equal(t, "terminal-c", *state.Broadcast.ActiveTerminalID)
			assertOrderedRouteIDs(t, fixture.service, []string{"terminal-a", "terminal-b"})
		})
	}

	fixture, _ := newOrderedGroupStartFixture(t)
	_, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-c", "Terminal C"))
	require.NoError(t, err)
	for index, step := range []struct {
		from      string
		to        string
		wantRoute []string
	}{
		{from: "terminal-c", to: "terminal-b", wantRoute: []string{"terminal-a"}},
		{from: "terminal-b", to: "terminal-a", wantRoute: []string{}},
	} {
		pending := requestOrderedGroupReturn(t, fixture, step.from, fmt.Sprintf("seeded-approve-%d", index))
		before := fixture.service.Snapshot()
		assert.Equal(t, step.from, *before.Broadcast.ActiveTerminalID)
		state, approveErr := fixture.service.ResolveTerminalNavigation(
			pending.RequestID,
			domain.TerminalNavigationApprove,
		)
		require.NoError(t, approveErr)
		assert.Equal(t, step.to, *state.Broadcast.ActiveTerminalID)
		assert.Nil(t, state.PendingTerminalNavigation)
		assertOrderedRouteIDs(t, fixture.service, step.wantRoute)
	}
}

func TestSeededReturnApprovalRejectsStaleGroupOrderWithoutNavigation(t *testing.T) {
	t.Parallel()

	fixture, catalog := newOrderedGroupStartFixture(t)
	_, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-c", "Terminal C"))
	require.NoError(t, err)
	pending := requestOrderedGroupReturn(t, fixture, "terminal-c", "seeded-stale-order")
	catalog.groups = []domain.TerminalGroup{
		{ID: "ordered-route", Name: "Ordered route", TerminalIDs: []string{"terminal-b", "terminal-a", "terminal-c", "terminal-d"}},
	}

	state, resolveErr := fixture.service.ResolveTerminalNavigation(
		pending.RequestID,
		domain.TerminalNavigationApprove,
	)
	require.Error(t, resolveErr)
	assert.Equal(t, "terminal-c", *state.Broadcast.ActiveTerminalID)
	assertOrderedRouteIDs(t, fixture.service, []string{"terminal-a", "terminal-b"})
	fixture.service.mu.RLock()
	assert.NotContains(t, fixture.service.runtime.Broadcast.TerminalRuntimes, "terminal-b")
	fixture.service.mu.RUnlock()
}

func TestSeededReturnRejectsCrossGroupTargetBeforePending(t *testing.T) {
	t.Parallel()

	fixture, catalog := newOrderedGroupStartFixture(t)
	_, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-c", "Terminal C"))
	require.NoError(t, err)
	catalog.groups = []domain.TerminalGroup{
		{ID: "prefix", Name: "Prefix", TerminalIDs: []string{"terminal-a", "terminal-b"}},
		{ID: "active", Name: "Active", TerminalIDs: []string{"terminal-c", "terminal-d"}},
	}

	result := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		orderedGroupBackCommand(fixture, "terminal-c", "seeded-cross-group"),
	)
	assert.Equal(t, domain.ActionReasonInvalidAction, result.Reason)
	assert.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)
	assert.Equal(t, "terminal-c", *fixture.service.Snapshot().Broadcast.ActiveTerminalID)
	assertOrderedRouteIDs(t, fixture.service, []string{"terminal-a", "terminal-b"})
}

func TestLaterManualActivationClearsSeededRouteWithoutReseeding(t *testing.T) {
	t.Parallel()

	fixture, _ := newOrderedGroupStartFixture(t)
	_, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-c", "Terminal C"))
	require.NoError(t, err)
	assertOrderedRouteIDs(t, fixture.service, []string{"terminal-a", "terminal-b"})
	requestOrderedGroupReturn(t, fixture, "terminal-c", "seeded-before-manual")
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.TerminalRuntimes["terminal-c"].Hack.Solved = true
		return transition{accepted: true}
	})

	state, activationErr := fixture.service.RequestTerminalActivation(terminalTarget("terminal-d", "Terminal D"))
	require.NoError(t, activationErr)
	assert.Equal(t, "terminal-d", *state.Broadcast.ActiveTerminalID)
	assert.Nil(t, state.PendingTerminalNavigation)
	assertOrderedRouteIDs(t, fixture.service, []string{})
}

func TestManualAndBroadcastLifecycleClearTerminalNavigationState(t *testing.T) {
	for _, boundary := range []string{"manual activation", "manual clear", "end broadcast", "shutdown"} {
		t.Run(boundary, func(t *testing.T) {
			fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
			fixture.service.terminals = newRecordingDecisionTerminalLifecycle()
			fixture.service.commit(func(root *domain.ProcessRuntime) transition {
				terminal := root.Broadcast.TerminalRuntimes[fixture.terminalID]
				terminal.Hack.Solved = true
				root.Broadcast.Route = []domain.TerminalReturnPoint{{TerminalID: "terminal-old", FolderID: "root", CommandID: "old-link"}}
				root.PendingTerminalNavigation = &domain.PendingTerminalNavigation{
					RequestID: "navigation-old", BroadcastID: root.Broadcast.ID, Direction: domain.TerminalNavigationForward,
					SourceTerminalID: fixture.terminalID, TargetTerminalID: "terminal-old",
				}
				targetID := "terminal-old"
				root.TerminalNavigationNotice = &domain.TerminalNavigationNotice{
					Reason: domain.TerminalNavigationNoticeTargetChanged, SourceTerminalID: fixture.terminalID,
					CommandID: "old-link", TargetTerminalID: &targetID,
				}
				return transition{accepted: true}
			})

			switch boundary {
			case "manual activation":
				_, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-manual", "Manual"))
				require.NoError(t, err)
			case "manual clear":
				_, err := fixture.service.RequestTerminalClear()
				require.NoError(t, err)
			case "end broadcast":
				_, err := fixture.service.EndBroadcast()
				require.NoError(t, err)
			case "shutdown":
				fixture.service.Shutdown()
			}

			state := fixture.service.Snapshot()
			require.Nil(t, state.PendingTerminalNavigation)
			require.Nil(t, state.TerminalNavigationNotice)
			fixture.service.mu.RLock()
			if fixture.service.runtime.Broadcast != nil {
				require.Empty(t, fixture.service.runtime.Broadcast.Route)
			}
			fixture.service.mu.RUnlock()
			if boundary == "end broadcast" {
				started, err := fixture.service.StartBroadcast()
				require.NoError(t, err)
				require.Nil(t, started.PendingTerminalNavigation)
				require.Nil(t, started.TerminalNavigationNotice)
			}
		})
	}
}

func TestRootBackReturnRejectsWithoutMutationThenApprovesOneLIFOPoint(t *testing.T) {
	t.Parallel()
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	lifecycle := newRecordingDecisionTerminalLifecycle()
	fixture.service.terminals = lifecycle

	targetA := terminalTarget(fixture.terminalID, "Terminal A")
	targetA.Tree = domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
		{ID: "archive", Type: domain.NodeFolder, Name: "ARCHIVE", Children: []domain.ContentNode{
			{ID: "docs", Type: domain.NodeFolder, Name: "DOCS"},
		}},
	}}
	fixture.service.terminalCatalog = &recordingTerminalCatalog{terminals: map[string]domain.TerminalTarget{
		fixture.terminalID: targetA,
	}}
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		source := root.Broadcast.TerminalRuntimes[fixture.terminalID]
		source.Lifecycle = domain.TerminalLifecycleSuspended
		destination := testTerminalRuntime("terminal-b")
		destination.TerminalName = "Terminal B"
		destination.Nav = domain.NavState{Path: []string{"root", "docs"}, Mode: "list"}
		root.Broadcast.TerminalRuntimes["terminal-b"] = destination
		active := "terminal-b"
		root.Broadcast.ActiveTerminalID = &active
		root.Broadcast.Route = []domain.TerminalReturnPoint{{
			TerminalID: fixture.terminalID, TerminalName: "Terminal A", FolderID: "docs",
			AncestorFolderIDs: []string{"root"}, CommandID: "open-b", CommandName: "Open B",
		}}
		return transition{accepted: true}
	})

	// Back inside a terminal folder keeps its legacy intra-terminal meaning.
	inside := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "return-inside", BroadcastID: fixture.broadcastID, TerminalID: "terminal-b",
		Kind: domain.RuntimeCommandNavigate, Action: "back",
	})
	require.True(t, inside.Accepted)
	require.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)
	assert.Equal(t, domain.NavState{Path: []string{"root"}, Mode: "list"}, canonicalTerminal(t, fixture.service, "terminal-b").Nav)

	requestReturn := func(requestID string) *domain.MasterPendingTerminalNavigation {
		result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
			RequestID: requestID, BroadcastID: fixture.broadcastID, TerminalID: "terminal-b",
			Kind: domain.RuntimeCommandNavigate, Action: "back",
		})
		require.True(t, result.Accepted)
		pending := fixture.service.Snapshot().PendingTerminalNavigation
		require.NotNil(t, pending)
		assert.Equal(t, domain.TerminalNavigationReturn, pending.Direction)
		assert.Equal(t, fixture.terminalID, pending.TargetTerminalID)
		assert.Equal(t, uint32(1), pending.RouteDepth)
		return pending
	}

	pending := requestReturn("return-reject")
	beforeReject := fixture.service.Snapshot()
	rejected, err := fixture.service.ResolveTerminalNavigation(pending.RequestID, domain.TerminalNavigationReject)
	require.NoError(t, err)
	require.Nil(t, rejected.PendingTerminalNavigation)
	require.Equal(t, "terminal-b", *rejected.Broadcast.ActiveTerminalID)
	fixture.service.mu.RLock()
	require.Len(t, fixture.service.runtime.Broadcast.Route, 1)
	fixture.service.mu.RUnlock()
	assert.Greater(t, rejected.Revision, beforeReject.Revision)

	pending = requestReturn("return-approve")
	approved, err := fixture.service.ResolveTerminalNavigation(pending.RequestID, domain.TerminalNavigationApprove)
	require.NoError(t, err)
	require.Equal(t, fixture.terminalID, *approved.Broadcast.ActiveTerminalID)
	require.Nil(t, approved.PendingTerminalNavigation)
	fixture.service.mu.RLock()
	require.Empty(t, fixture.service.runtime.Broadcast.Route)
	assert.Equal(t, domain.NavState{Path: []string{"root", "archive", "docs"}, Mode: "list"}, fixture.service.runtime.Broadcast.TerminalRuntimes[fixture.terminalID].Nav)
	fixture.service.mu.RUnlock()
}

func TestReturnApprovalRequiresUnchangedTopAndUnwindsCyclesOnePointAtATime(t *testing.T) {
	t.Parallel()
	runtime := &recordingTerminalRuntime{}
	fixture := newUS2Fixture(t, runtime)
	lifecycle := newRecordingDecisionTerminalLifecycle()
	fixture.service.terminals = lifecycle
	fixture.service.terminalCatalog = &recordingTerminalCatalog{terminals: map[string]domain.TerminalTarget{
		"terminal-a": terminalTarget("terminal-a", "Terminal A"),
		"terminal-b": terminalTarget("terminal-b", "Terminal B"),
	}}
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		delete(root.Broadcast.TerminalRuntimes, fixture.terminalID)
		for _, id := range []string{"terminal-a", "terminal-b", "terminal-c"} {
			terminal := testTerminalRuntime(id)
			terminal.Lifecycle = domain.TerminalLifecycleSuspended
			root.Broadcast.TerminalRuntimes[id] = terminal
		}
		root.Broadcast.TerminalRuntimes["terminal-c"].Lifecycle = domain.TerminalLifecycleActive
		active := "terminal-c"
		root.Broadcast.ActiveTerminalID = &active
		root.Broadcast.Route = []domain.TerminalReturnPoint{
			{TerminalID: "terminal-a", TerminalName: "Terminal A", FolderID: "root", CommandID: "to-b", CommandName: "To B"},
			{TerminalID: "terminal-b", TerminalName: "Terminal B", FolderID: "root", CommandID: "to-c", CommandName: "To C"},
		}
		return transition{accepted: true}
	})

	back := func(requestID, terminalID string) *domain.MasterPendingTerminalNavigation {
		result := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
			RequestID: requestID, BroadcastID: fixture.broadcastID, TerminalID: terminalID,
			Kind: domain.RuntimeCommandNavigate, Action: "back",
		})
		require.True(t, result.Accepted)
		pending := fixture.service.Snapshot().PendingTerminalNavigation
		require.NotNil(t, pending)
		return pending
	}

	stale := back("return-stale", "terminal-c")
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.Route[len(root.Broadcast.Route)-1].CommandID = "changed-command"
		return transition{accepted: true}
	})
	beforeStale := fixture.service.Snapshot()
	got, err := fixture.service.ResolveTerminalNavigation(stale.RequestID, domain.TerminalNavigationApprove)
	require.Error(t, err)
	assert.Equal(t, beforeStale, got)
	assert.Equal(t, beforeStale, fixture.service.Snapshot())

	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		root.PendingTerminalNavigation = nil
		root.Broadcast.Route[len(root.Broadcast.Route)-1].CommandID = "to-c"
		return transition{accepted: true}
	})
	first := back("return-c-b", "terminal-c")
	state, err := fixture.service.ResolveTerminalNavigation(first.RequestID, domain.TerminalNavigationApprove)
	require.NoError(t, err)
	require.Equal(t, "terminal-b", *state.Broadcast.ActiveTerminalID)
	fixture.service.mu.RLock()
	require.Len(t, fixture.service.runtime.Broadcast.Route, 1)
	fixture.service.mu.RUnlock()

	second := back("return-b-a", "terminal-b")
	state, err = fixture.service.ResolveTerminalNavigation(second.RequestID, domain.TerminalNavigationApprove)
	require.NoError(t, err)
	require.Equal(t, "terminal-a", *state.Broadcast.ActiveTerminalID)
	fixture.service.mu.RLock()
	require.Empty(t, fixture.service.runtime.Broadcast.Route)
	fixture.service.mu.RUnlock()

	ordinary := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
		RequestID: "no-route-back", BroadcastID: fixture.broadcastID, TerminalID: "terminal-a",
		Kind: domain.RuntimeCommandNavigate, Action: "back",
	})
	require.True(t, ordinary.Accepted)
	require.Nil(t, fixture.service.Snapshot().PendingTerminalNavigation)
}

type groupAwareNavigationCatalog struct {
	groups      []domain.TerminalGroup
	transitions map[string]domain.TerminalTransitionTarget
	terminals   map[string]domain.TerminalTarget
}

func (catalog *groupAwareNavigationCatalog) LookupTerminal(id string) (domain.TerminalTarget, bool) {
	target, ok := catalog.terminals[id]
	if !ok {
		return domain.TerminalTarget{}, false
	}
	return *cloneTerminalTarget(&target), true
}

func (catalog *groupAwareNavigationCatalog) LookupTerminalGroup(
	terminalID string,
) (domain.TerminalGroupSnapshot, bool) {
	var found *domain.TerminalGroup
	for index := range catalog.groups {
		group := &catalog.groups[index]
		for _, memberID := range group.TerminalIDs {
			if memberID != terminalID {
				continue
			}
			if found != nil {
				return domain.TerminalGroupSnapshot{}, false
			}
			found = group
		}
	}
	if found == nil {
		return domain.TerminalGroupSnapshot{}, false
	}
	return domain.TerminalGroupSnapshot{
		ID: found.ID, Name: found.Name, TerminalIDs: append([]string(nil), found.TerminalIDs...),
	}, true
}

func (catalog *groupAwareNavigationCatalog) LookupTerminalTransition(
	sourceID string,
	commandID string,
) (domain.TerminalTransitionTarget, bool) {
	transition, ok := catalog.transitions[sourceID+"/"+commandID]
	if !ok || !sameExactTerminalGroup(catalog.groups, sourceID, transition.Target.TerminalID) {
		return domain.TerminalTransitionTarget{}, false
	}
	transition.Target = *cloneTerminalTarget(&transition.Target)
	return transition, true
}

func sameExactTerminalGroup(groups []domain.TerminalGroup, firstID, secondID string) bool {
	groupFor := func(terminalID string) (string, bool) {
		groupID := ""
		memberships := 0
		for _, group := range groups {
			for _, memberID := range group.TerminalIDs {
				if memberID != terminalID {
					continue
				}
				groupID = group.ID
				memberships++
			}
		}
		return groupID, memberships == 1
	}
	firstGroup, firstOK := groupFor(firstID)
	secondGroup, secondOK := groupFor(secondID)
	return firstOK && secondOK && firstGroup == secondGroup
}

func newGroupAwareForwardFixture(t *testing.T) (us2Fixture, *groupAwareNavigationCatalog) {
	t.Helper()

	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.terminals = newRecordingDecisionTerminalLifecycle()
	target := terminalTarget("terminal-b", "Terminal B")
	catalog := &groupAwareNavigationCatalog{
		groups: []domain.TerminalGroup{
			{ID: "route", Name: "Route", TerminalIDs: []string{fixture.terminalID, target.TerminalID}},
		},
		transitions: map[string]domain.TerminalTransitionTarget{
			fixture.terminalID + "/linked-command": {
				SourceTerminalID: fixture.terminalID, SourceTerminalName: "Terminal 1",
				CommandID: "linked-command", CommandName: "Open B", Target: target,
			},
		},
		terminals: map[string]domain.TerminalTarget{target.TerminalID: target},
	}
	fixture.service.terminalCatalog = catalog
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		terminal := root.Broadcast.TerminalRuntimes[fixture.terminalID]
		terminal.Tree.Children = append(terminal.Tree.Children, domain.ContentNode{
			ID: "linked-command", Type: domain.NodeCommand, Name: "Open B",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: target.TerminalID},
		})
		return transition{accepted: true}
	})
	return fixture, catalog
}

func groupAwareForwardCommand(fixture us2Fixture, requestID string) domain.RuntimeCommand {
	return domain.RuntimeCommand{
		RequestID: domain.RequestID(requestID), BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "linked-command",
		PayloadFingerprint: requestID,
	}
}

func newOrderedGroupStartFixture(t *testing.T) (us2Fixture, *groupAwareNavigationCatalog) {
	t.Helper()

	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.terminals = newRecordingDecisionTerminalLifecycle()
	terminalIDs := []string{"terminal-a", "terminal-b", "terminal-c", "terminal-d"}
	targets := make(map[string]domain.TerminalTarget, len(terminalIDs))
	for _, terminalID := range terminalIDs {
		targets[terminalID] = terminalTarget(terminalID, strings.ToUpper(strings.TrimPrefix(terminalID, "terminal-")))
	}
	catalog := &groupAwareNavigationCatalog{
		groups: []domain.TerminalGroup{
			{ID: "ordered-route", Name: "Ordered route", TerminalIDs: terminalIDs},
		},
		transitions: map[string]domain.TerminalTransitionTarget{},
		terminals:   targets,
	}
	fixture.service.terminalCatalog = catalog
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.ActiveTerminalID = nil
		root.Broadcast.TerminalRuntimes = make(map[string]*domain.TerminalRuntime)
		root.Broadcast.Route = nil
		root.PendingSwitch = nil
		root.PendingCommandExecution = nil
		root.PendingTerminalNavigation = nil
		root.TerminalNavigationNotice = nil
		return transition{accepted: true}
	})
	return fixture, catalog
}

func orderedGroupBackCommand(fixture us2Fixture, terminalID, requestID string) domain.RuntimeCommand {
	return domain.RuntimeCommand{
		RequestID: domain.RequestID(requestID), BroadcastID: fixture.broadcastID, TerminalID: terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "back", PayloadFingerprint: requestID,
	}
}

func requestOrderedGroupReturn(
	t *testing.T,
	fixture us2Fixture,
	terminalID string,
	requestID string,
) domain.PendingTerminalNavigation {
	t.Helper()
	result := fixture.service.DispatchPlayerAction(
		fixture.controllerConnection,
		orderedGroupBackCommand(fixture, terminalID, requestID),
	)
	require.True(t, result.Accepted)
	fixture.service.mu.RLock()
	runtimePending := fixture.service.runtime.PendingTerminalNavigation
	var pending *domain.PendingTerminalNavigation
	if runtimePending != nil {
		value := *runtimePending
		value.ReturnPoint = cloneTerminalReturnPoint(value.ReturnPoint)
		pending = &value
	}
	fixture.service.mu.RUnlock()
	require.NotNil(t, pending)
	require.Equal(t, domain.TerminalNavigationReturn, pending.Direction)
	return *pending
}

func assertOrderedRouteIDs(t *testing.T, service *Service, want []string) {
	t.Helper()
	service.mu.RLock()
	got := make([]string, len(service.runtime.Broadcast.Route))
	for index, point := range service.runtime.Broadcast.Route {
		got[index] = point.TerminalID
	}
	service.mu.RUnlock()
	assert.Equal(t, want, got)
}

type recordingTerminalCatalog struct {
	transitions map[string]domain.TerminalTransitionTarget
	terminals   map[string]domain.TerminalTarget
}

func (catalog *recordingTerminalCatalog) LookupTerminal(id string) (domain.TerminalTarget, bool) {
	target, ok := catalog.terminals[id]
	return *cloneTerminalTarget(&target), ok
}

func (catalog *recordingTerminalCatalog) LookupTerminalTransition(sourceID, commandID string) (domain.TerminalTransitionTarget, bool) {
	transition, ok := catalog.transitions[sourceID+"/"+commandID]
	if !ok {
		return domain.TerminalTransitionTarget{}, false
	}
	transition.Target = *cloneTerminalTarget(&transition.Target)
	return transition, true
}

func (fixture commandExecutionFixture) command(requestID domain.RequestID) domain.RuntimeCommand {
	return domain.RuntimeCommand{
		RequestID: requestID, BroadcastID: fixture.broadcastID, TerminalID: fixture.terminalID,
		Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: fixture.commandID,
	}
}

type recordingCommandStateStore struct {
	mu        sync.Mutex
	mutation  CommandStateMutation
	err       error
	executes  [][2]string
	contexts  []context.Context
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
}

type commandStateContextKey struct{}

func (store *recordingCommandStateStore) ExecuteCommandState(ctx context.Context, terminalID, commandID string) (CommandStateMutation, error) {
	store.mu.Lock()
	store.executes = append(store.executes, [2]string{terminalID, commandID})
	store.contexts = append(store.contexts, ctx)
	started := store.started
	release := store.release
	mutation := store.mutation
	err := store.err
	store.mu.Unlock()
	if started != nil {
		store.startOnce.Do(func() { close(started) })
	}
	if release != nil {
		<-release
	}
	return mutation, err
}

func (store *recordingCommandStateStore) ResetCommandState(context.Context, string, string) (CommandStateMutation, error) {
	return CommandStateMutation{}, errors.New("unexpected reset-one call")
}

func (store *recordingCommandStateStore) ResetTerminalCommandStates(context.Context, string) (CommandStateMutation, error) {
	return CommandStateMutation{}, errors.New("unexpected reset-terminal call")
}

func (store *recordingCommandStateStore) ExecuteCalls() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.executes)
}

func (store *recordingCommandStateStore) ExecuteArguments() [][2]string {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([][2]string(nil), store.executes...)
}

func (store *recordingCommandStateStore) ExecuteContexts() []context.Context {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]context.Context(nil), store.contexts...)
}

func commandExecutionSession(completed bool) domain.Session {
	commandID := "command-open-doors"
	terminal := domain.Terminal{
		ID: "terminal-1", Name: "Overseer", HackLevel: 1,
		Root: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: commandID, Type: domain.NodeCommand, Name: "Open doors", Text: "Doors opened",
				StateChange: &domain.StateChangeConfig{
					CompletedName: "Doors open", ConfirmationText: "Open the doors?",
				},
			}},
		},
	}
	if completed {
		terminal.CommandStates = map[string]domain.CommandExecutionState{
			commandID: {CompletedName: "Doors open", ResultText: "Doors opened"},
		}
	}
	return domain.Session{Version: 1, Name: "Command execution fixture", Terminals: []domain.Terminal{terminal}}
}

func newUS2Fixture(t *testing.T, runtime RuntimeActions) us2Fixture {
	t.Helper()
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	config := Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, Runtime: runtime}
	if terminals, ok := runtime.(TerminalRuntimeLifecycle); ok {
		config.Terminals = terminals
	}
	service := New(config)
	_, err := addCharacter(service, "Mara")
	if err != nil {
		require.NoError(t, err)
	}
	_, err = addCharacter(service, "Boone")
	if err != nil {
		require.NoError(t, err)
	}
	state, err := service.StartBroadcast()
	if err != nil {
		require.NoError(t, err)
	}
	controllerConnection := domain.ConnectionID("connection-controller")
	observerConnection := domain.ConnectionID("connection-observer")
	unassignedConnection := domain.ConnectionID("connection-unassigned")
	controller := service.CreateSession(controllerConnection)
	observer := service.CreateSession(observerConnection)
	unassigned := service.CreateSession(unassignedConnection)
	{
		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: controllerConnection, SessionID: controller.SessionID, RequestID: "select-controller",
			BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[0].ID,
		})
		require.Falsef(t, !result.Accepted,
			"controller selection = %#v", result)
	}
	{

		result := service.SelectCharacter(CharacterSelection{
			ConnectionID: observerConnection, SessionID: observer.SessionID, RequestID: "select-observer",
			BroadcastID: state.Broadcast.ID, CharacterID: state.Roster[1].ID,
		})
		require.Falsef(t, !result.Accepted,
			"observer selection = %#v", result)
	}

	terminalID := "terminal-1"
	service.commit(func(root *domain.ProcessRuntime) transition {
		root.Broadcast.ActiveTerminalID = &terminalID
		root.Broadcast.TerminalRuntimes[terminalID] = testTerminalRuntime(terminalID)
		return transition{accepted: true}
	})
	return us2Fixture{
		service: service, effects: effects, broadcastID: state.Broadcast.ID, terminalID: terminalID,
		controllerConnection: controllerConnection, observerConnection: observerConnection, unassignedConnection: unassignedConnection,
		controllerSession: controller.SessionID, observerSession: observer.SessionID, unassignedSession: unassigned.SessionID,
		controllerToken: controller.BrowserToken, observerToken: observer.BrowserToken, unassignedToken: unassigned.BrowserToken,
	}
}

type recordingTerminalRuntime struct {
	mu sync.Mutex

	calls       []domain.RuntimeCommand
	randomCalls int
	started     chan struct{}
	release     chan struct{}
	startOnce   sync.Once
}

func (runtime *recordingTerminalRuntime) Apply(state *domain.TerminalRuntime, command domain.RuntimeCommand) (*domain.PublicLiveState, bool) {
	runtime.mu.Lock()
	runtime.calls = append(runtime.calls, command)
	if command.Kind == domain.RuntimeCommandActivatePattern {
		runtime.randomCalls++
	}
	started := runtime.started
	release := runtime.release
	runtime.mu.Unlock()
	if started != nil {
		runtime.startOnce.Do(func() {
			close(started)
			<-release
		})
	}

	switch command.Kind {
	case domain.RuntimeCommandNavigate:
		state.Nav = nav.ApplyAction(state.Nav, state.Tree, command.Action, command.NodeID)
	case domain.RuntimeCommandGuess:
		if state.Hack == nil || state.Hack.Solved || state.Hack.Failed {
			return nil, false
		}
		hack.ApplyGuess(state.Hack, command.TargetID)
	case domain.RuntimeCommandActivatePattern:
		return nil, false
	default:
		return nil, false
	}
	return publicTerminalRuntime(state), true
}

func (runtime *recordingTerminalRuntime) Calls() int {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return len(runtime.calls)
}

func (runtime *recordingTerminalRuntime) RandomCalls() int {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.randomCalls
}

func actionResultForRequest(t *testing.T, effects *testutil.FakeOrderedEffectSink[Effect], requestID string) domain.ActionResult {
	t.Helper()
	recorded := effects.Values()
	for _, r := range slices.Backward(recorded) {
		if r.Result != nil && r.Result.RequestID == requestID {
			return *r.Result
		}
	}
	assert.FailNowf(t, "assertion failed", "no action result effect for request %q", requestID)
	return domain.ActionResult{}
}

func canonicalTerminalBytes(t *testing.T, service *Service, terminalID string) []byte {
	t.Helper()
	terminal := canonicalTerminal(t, service, terminalID)
	var usedPatterns []domain.HackPatternIdentity
	var privateHack *canonicalHackSnapshot
	if terminal.Hack != nil {
		hackState := terminal.Hack
		for identity := range hackState.UsedPatterns {
			usedPatterns = append(usedPatterns, identity)
		}
		sort.Slice(usedPatterns, func(left, right int) bool {
			if usedPatterns[left].GenerationID != usedPatterns[right].GenerationID {
				return usedPatterns[left].GenerationID < usedPatterns[right].GenerationID
			}
			if usedPatterns[left].Row != usedPatterns[right].Row {
				return usedPatterns[left].Row < usedPatterns[right].Row
			}
			if usedPatterns[left].Start != usedPatterns[right].Start {
				return usedPatterns[left].Start < usedPatterns[right].Start
			}
			return usedPatterns[left].End < usedPatterns[right].End
		})
		privateHack = &canonicalHackSnapshot{
			GenerationID: hackState.GenerationID, Level: hackState.Level, WordLength: hackState.WordLength,
			AttemptsMax: hackState.AttemptsMax, AttemptsLeft: hackState.AttemptsLeft,
			SecretWord: hackState.SecretWord, WordsByID: hackState.WordsByID, UsedPatterns: usedPatterns,
			Solved: hackState.Solved, Failed: hackState.Failed, Log: hackState.Log, Columns: hackState.Columns,
		}
	}
	encoded, err := json.Marshal(canonicalTerminalSnapshot{
		TerminalID: terminal.TerminalID, TerminalName: terminal.TerminalName,
		Tree: terminal.Tree, CommandStates: terminal.CommandStates, CommandExecution: terminal.CommandExecution,
		HackLevel: terminal.HackLevel, IntroText: terminal.IntroText, Nav: terminal.Nav,
		Hack: privateHack, Lifecycle: terminal.Lifecycle,
	})
	if err != nil {
		require.NoError(t, err)
	}
	return encoded
}

type canonicalTerminalSnapshot struct {
	TerminalID       string
	TerminalName     string
	Tree             domain.ContentNode
	CommandStates    map[string]domain.CommandExecutionState
	CommandExecution *domain.CommandExecutionPresentation
	HackLevel        int
	IntroText        string
	Nav              domain.NavState
	Hack             *canonicalHackSnapshot
	Lifecycle        domain.TerminalLifecycle
}

type canonicalHackSnapshot struct {
	GenerationID string
	Level        int
	WordLength   int
	AttemptsMax  int
	AttemptsLeft int
	SecretWord   string
	WordsByID    map[string]domain.HackCandidate
	UsedPatterns []domain.HackPatternIdentity
	Solved       bool
	Failed       bool
	Log          []string
	Columns      []domain.HackColumn
}

func canonicalTerminal(t *testing.T, service *Service, terminalID string) *domain.TerminalRuntime {
	t.Helper()
	service.mu.RLock()
	defer service.mu.RUnlock()
	require.Falsef(t, service.runtime.Broadcast == nil || service.runtime.Broadcast.TerminalRuntimes[terminalID] == nil,
		"canonical terminal %q is absent", terminalID)

	return cloneTerminalRuntime(service.runtime.Broadcast.TerminalRuntimes[terminalID])
}

func setControllerForTest(service *Service, sessionID domain.LogicalSessionID) {
	service.commit(func(runtime *domain.ProcessRuntime) transition {
		controller := sessionID
		runtime.Broadcast.ControllerSessionID = &controller
		return transition{accepted: true}
	})
}

func testTerminalRuntime(terminalID string) *domain.TerminalRuntime {
	return &domain.TerminalRuntime{
		TerminalID: terminalID, TerminalName: "Overseer", Lifecycle: domain.TerminalLifecycleActive,
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: "docs", Type: domain.NodeFolder, Name: "DOCS",
			}},
		},
		Nav:       domain.NavState{Path: []string{"root"}, Mode: "list"},
		HackLevel: 1,
		Hack: &domain.HackState{
			GenerationID: "generation-1", Level: 1, WordLength: 5,
			AttemptsMax: 4, AttemptsLeft: 4, SecretWord: "ALPHA",
			WordsByID: map[string]domain.HackCandidate{
				"candidate-secret": {Text: "ALPHA"},
				"candidate-wrong":  {Text: "BRAVO"},
			},
			UsedPatterns: make(map[domain.HackPatternIdentity]struct{}),
			Log:          []string{},
		},
	}
}

func publicTerminalRuntime(state *domain.TerminalRuntime) *domain.PublicLiveState {
	return &domain.PublicLiveState{
		TerminalID: state.TerminalID, TerminalName: state.TerminalName,
		Tree: state.Tree, HackLevel: state.HackLevel, IntroText: state.IntroText,
		Nav: state.Nav, Hack: hack.PublicState(state.Hack),
	}
}

func newUS1Service() *Service {
	return New(Config{IDs: &counterIDSource{}})
}

func assertExclusiveClaimInvariants(t *testing.T, state *domain.MasterCoordinationState) {
	t.Helper()
	claimBySession := make(map[domain.LogicalSessionID]domain.CharacterID)
	for _, character := range state.Roster {
		if character.ClaimedBySessionID == nil {
			continue
		}
		{
			previous, duplicate := claimBySession[*character.ClaimedBySessionID]
			require.Falsef(t, duplicate,
				"session %q claims both %q and %q", *character.ClaimedBySessionID, previous, character.ID)
		}

		claimBySession[*character.ClaimedBySessionID] = character.ID
	}
	for _, session := range state.Sessions {
		claimed, hasClaim := claimBySession[session.ID]
		require.Falsef(t, session.Character == nil && hasClaim,
			"roster claim for %q is missing from session projection", session.ID)
		require.Falsef(t, session.Character != nil && (!hasClaim || claimed != session.Character.ID),
			"session claim for %q disagrees with roster: session=%#v roster=%#v", session.ID, session.Character, claimBySession)

	}
	if state.Broadcast != nil && state.Broadcast.ControllerSessionID != nil {
		controller := masterSession(t, state, *state.Broadcast.ControllerSessionID)
		require.Falsef(t, controller.Character == nil || controller.Role != domain.PlayerRoleActive,
			"controller is not an active assigned session: %#v", controller)

	}
}

func masterSession(t *testing.T, state *domain.MasterCoordinationState, sessionID domain.LogicalSessionID) domain.MasterSessionEntry {
	t.Helper()
	for _, session := range state.Sessions {
		if session.ID == sessionID {
			return session
		}
	}
	assert.FailNowf(t, "assertion failed", "master state has no logical session %q", sessionID)
	return domain.MasterSessionEntry{}
}

func claimedRosterCount(state *domain.MasterCoordinationState) int {
	count := 0
	for _, character := range state.Roster {
		if character.ClaimedBySessionID != nil {
			count++
		}
	}
	return count
}

func activeSessionCount(state *domain.MasterCoordinationState) int {
	return sessionRoleCount(state, domain.PlayerRoleActive)
}

func observerSessionCount(state *domain.MasterCoordinationState) int {
	return sessionRoleCount(state, domain.PlayerRoleObserver)
}

func sessionRoleCount(state *domain.MasterCoordinationState, role domain.PlayerRole) int {
	count := 0
	for _, session := range state.Sessions {
		if session.Role == role {
			count++
		}
	}
	return count
}

type counterIDSource struct {
	next atomic.Uint64
}

type controlFixedWords struct{}

type controlCountingRandom struct{ calls atomic.Int64 }

func (random *controlCountingRandom) Intn(limit int) int {
	random.calls.Add(1)
	if limit <= 1 {
		return 0
	}
	return 1
}

func (random *controlCountingRandom) Calls() int {
	return int(random.calls.Load())
}

func (controlFixedWords) PickWords(length, count int) []string {
	pools := map[int][]string{
		4: {"CODE", "CAVE", "DUST", "IRON", "GATE", "BOLT", "RAMP", "CORE", "FUSE", "GRID", "LAMP", "MASK", "NODE", "PIPE", "RING", "RUST"},
		5: {"ALLOY", "ARMOR", "ATLAS", "BASIN", "BLAST", "BRICK", "CABLE", "CACHE", "CARGO", "CLIFF", "CLOCK", "CRANE", "CRATE", "CREEK", "DRAIN", "DRONE"},
	}
	return append([]string(nil), pools[length][:count]...)
}

func testPatternTree() domain.ContentNode {
	return testTerminalRuntime("pattern-tree").Tree
}

func (source *counterIDSource) Next() string {
	return fmt.Sprintf("opaque-%d", source.next.Add(1))
}

func addCharacter(service *Service, name string) (*domain.MasterCoordinationState, error) {
	return service.AddCharacter(domain.CharacterCreatePayload{
		Name:                name,
		Intelligence:        1,
		HackerPerkAvailable: false,
		ExpectedRevision:    service.Revision(),
	})
}

type sequenceIDSource struct {
	mu     sync.Mutex
	values []string
	next   int
}

func (source *sequenceIDSource) Next() string {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.next >= len(source.values) {
		panic("sequence ID source exhausted")
	}
	value := source.values[source.next]
	source.next++
	return value
}

func testLiveState(id string) *domain.PublicLiveState {
	return &domain.PublicLiveState{
		TerminalID:   id,
		TerminalName: "canonical",
		Tree: domain.ContentNode{
			ID:   "root",
			Type: domain.NodeFolder,
			Name: "ROOT",
			Children: []domain.ContentNode{{
				ID:   "docs",
				Type: domain.NodeFolder,
				Name: "DOCS",
			}},
		},
		Nav: domain.NavState{Path: []string{"root"}, Mode: "list"},
		Hack: &domain.PublicHackState{
			Log: []string{"ENTRY DENIED"},
			Columns: []domain.HackColumn{{
				Words: []domain.HackWord{{ID: "candidate-1", Start: 0, Length: 5}},
			}},
		},
	}
}

func TestPlayerConfigRosterInstallAndSaveBeforePublication(t *testing.T) {
	t.Parallel()

	store := &fakeRosterStore{}
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, RosterStore: store})
	{

		_, err := addCharacter(service, "No Store Yet")
		require.False(t, err == nil,
			"AddCharacter() without an active player config succeeded")
	}
	{

		got := service.Snapshot()
		require.Falsef(t, len(got.Roster) != 0 || got.Revision != 0,
			"failed add changed state: %#v", got)
	}

	handle := domain.PlayerConfigHandle{Path: "/Campaigns/players.json", Version: 1, Name: "Players", ContentDigest: "digest-0"}
	emptyInstalled, err := service.InstallPlayerConfig(handle, []domain.CharacterRosterEntry{})
	require.Falsef(t, err != nil,
		"InstallPlayerConfig() empty roster error = %v", err)
	require.Falsef(t, emptyInstalled.PlayerConfig == nil || len(emptyInstalled.Roster) != 0,
		"installed empty player config = %#v", emptyInstalled)

	beforeNil := service.Snapshot()
	{
		_, err := service.InstallPlayerConfig(handle, nil)
		require.Falsef(t, err == nil || err.Error() != "roster must be an array",
			"InstallPlayerConfig() nil roster error = %v, want roster array validation", err)
	}
	{

		afterNil := service.Snapshot()
		require.Falsef(t, !cmp.Equal(afterNil, beforeNil),
			"nil roster changed coordinator\nbefore=%#v\nafter=%#v", beforeNil, afterNil)
	}

	roster := []domain.CharacterRosterEntry{
		{ID: "mara", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true},
		{ID: "boone", Name: "Boone", Intelligence: 3},
	}
	installed, err := service.InstallPlayerConfig(handle, roster)
	require.Falsef(t, err != nil,
		"InstallPlayerConfig() error = %v", err)
	require.Falsef(t, installed.PlayerConfig == nil || installed.PlayerConfig.Name != "Players" || len(installed.Roster) != 2,
		"installed state = %#v", installed)
	require.Equal(t, 8, installed.Roster[0].Intelligence)
	require.True(t, installed.Roster[0].HackerPerkAvailable)
	require.Equal(t, 3, installed.Roster[1].Intelligence)
	require.False(t, installed.Roster[1].HackerPerkAvailable)

	store.fail = true
	before := service.Snapshot()
	effectsBefore := effects.Calls()
	{
		_, err := service.UpdateCharacter(domain.CharacterUpdatePayload{
			CharacterID: "mara", Name: "Mara Voss", Intelligence: 10,
			HackerPerkAvailable: false, ExpectedRevision: service.Revision(),
		})
		require.False(t, err == nil,
			"UpdateCharacter() with failed persistence succeeded")
	}
	{

		after := service.Snapshot()
		require.Falsef(t, !cmp.Equal(after, before),
			"failed persistence changed coordinator\nbefore=%#v\nafter=%#v", before, after)
	}
	require.Falsef(t, effects.Calls() != effectsBefore,
		"failed persistence published %d effects", effects.Calls()-effectsBefore)

	store.fail = false
	renamed, err := service.UpdateCharacter(domain.CharacterUpdatePayload{
		CharacterID: "mara", Name: "Mara Voss", Intelligence: 10,
		HackerPerkAvailable: false, ExpectedRevision: service.Revision(),
	})
	require.Falsef(t, err != nil || renamed.Roster[0].Name != "Mara Voss",
		"UpdateCharacter() = state %#v, error %v", renamed, err)
	require.Equal(t, 10, renamed.Roster[0].Intelligence)
	require.False(t, renamed.Roster[0].HackerPerkAvailable)
	require.Falsef(t, len(store.saves) != 1 || store.saves[0].Roster[0].Name != "Mara Voss",
		"persisted candidates = %#v", store.saves)

	added, err := addCharacter(service, "Arcade")
	require.Falsef(t, err != nil || len(added.Roster) != 3,
		"AddCharacter() = state %#v, error %v", added, err)
	require.Equal(t, 1, added.Roster[2].Intelligence)
	require.False(t, added.Roster[2].HackerPerkAvailable)

	deleted, err := service.DeleteCharacter(domain.CharacterDeletePayload{
		CharacterID: "boone", ExpectedRevision: service.Revision(),
	})
	require.Falsef(t, err != nil || len(deleted.Roster) != 2,
		"DeleteCharacter() = state %#v, error %v", deleted, err)
	require.Falsef(t, len(store.saves) != 3,
		"successful roster mutations saved %d candidates, want 3", len(store.saves))
	require.Equal(t, []string{"digest-0", "digest-1", "digest-2"}, []string{
		store.handles[0].ContentDigest,
		store.handles[1].ContentDigest,
		store.handles[2].ContentDigest,
	})
	{

		got := store.saves[2].Roster
		require.Falsef(t, len(got) != 2 || got[0].ID != "mara" || got[1].Name != "Arcade",
			"final persisted ordered roster = %#v", got)
	}

}

func TestAddCharacterPersistsCompleteProfileOnceAndRejectsDuplicateRevision(t *testing.T) {
	t.Parallel()

	ids := &counterIDSource{}
	store := &fakeRosterStore{}
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: ids, Enqueue: effects.Enqueue, RosterStore: store})
	handle := domain.PlayerConfigHandle{
		Path: "/Campaigns/players.json", Version: 1, Name: "Players", ContentDigest: "digest-0",
	}
	installed, err := service.InstallPlayerConfig(handle, []domain.CharacterRosterEntry{})
	require.NoError(t, err)
	baselineEffects := effects.Calls()

	request := domain.CharacterCreatePayload{
		Name:                "  Mara Voss  ",
		Intelligence:        10,
		HackerPerkAvailable: false,
		ExpectedRevision:    installed.Revision,
	}
	added, err := service.AddCharacter(request)
	require.NoError(t, err)
	require.Equal(t, installed.Revision+1, added.Revision)
	require.Equal(t, uint64(1), ids.next.Load(), "accepted add must allocate exactly one stable ID")
	require.Len(t, added.Roster, 1)
	require.Equal(t, domain.MasterRosterEntry{
		ID: "opaque-1", Name: "Mara Voss", Intelligence: 10, HackerPerkAvailable: false,
	}, added.Roster[0])
	require.Len(t, store.saves, 1)
	require.Equal(t, []domain.CharacterRosterEntry{{
		ID: "opaque-1", Name: "Mara Voss", Intelligence: 10, HackerPerkAvailable: false,
	}}, store.saves[0].Roster)
	require.Equal(t, "digest-0", store.handles[0].ContentDigest)
	require.NotNil(t, service.runtime.ActivePlayerConfig)
	require.Equal(t, "digest-1", service.runtime.ActivePlayerConfig.ContentDigest)
	require.Equal(t, baselineEffects+1, effects.Calls(), "accepted add must publish one master effect")
	require.Equal(t, added.Revision, effects.Values()[baselineEffects].Revision)
	require.NotNil(t, effects.Values()[baselineEffects].Master)

	beforeRetry := service.Snapshot()
	retry, retryErr := service.AddCharacter(request)
	require.ErrorContains(t, retryErr, "revision")
	require.Equal(t, beforeRetry, retry)
	require.Equal(t, uint64(1), ids.next.Load(), "stale retry must not allocate another ID")
	require.Len(t, store.saves, 1, "stale retry must not repeat persistence")
	require.Equal(t, baselineEffects+1, effects.Calls(), "stale retry must not publish")

	reopened := New(Config{IDs: &counterIDSource{}, RosterStore: &fakeRosterStore{}})
	reopenedState, reopenErr := reopened.InstallPlayerConfig(*service.runtime.ActivePlayerConfig, store.saves[0].Roster)
	require.NoError(t, reopenErr)
	require.Equal(t, added.Roster, reopenedState.Roster, "canonical reopen must preserve the exact profile")
}

func TestAddCharacterGuardsRunBeforeIDAllocationOrPersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prepare func(t *testing.T, service *Service) uint64
		request func(current uint64) domain.CharacterCreatePayload
		wantErr string
	}{
		{
			name: "missing active config",
			prepare: func(t *testing.T, service *Service) uint64 {
				t.Helper()
				return service.Revision()
			},
			request: validCharacterCreatePayload,
			wantErr: "player config",
		},
		{
			name:    "stale coordination revision",
			prepare: installEmptyPlayerConfig,
			request: func(current uint64) domain.CharacterCreatePayload {
				payload := validCharacterCreatePayload(current)
				payload.ExpectedRevision = current - 1
				return payload
			},
			wantErr: "revision",
		},
		{
			name: "active broadcast",
			prepare: func(t *testing.T, service *Service) uint64 {
				t.Helper()
				installEmptyPlayerConfig(t, service)
				state, err := service.StartBroadcast()
				require.NoError(t, err)
				return state.Revision
			},
			request: validCharacterCreatePayload,
			wantErr: "broadcast",
		},
		{
			name:    "blank name",
			prepare: installEmptyPlayerConfig,
			request: func(current uint64) domain.CharacterCreatePayload {
				payload := validCharacterCreatePayload(current)
				payload.Name = "   "
				return payload
			},
			wantErr: "blank",
		},
		{
			name:    "intelligence below range",
			prepare: installEmptyPlayerConfig,
			request: func(current uint64) domain.CharacterCreatePayload {
				payload := validCharacterCreatePayload(current)
				payload.Intelligence = 0
				return payload
			},
			wantErr: "intelligence",
		},
		{
			name:    "intelligence above range",
			prepare: installEmptyPlayerConfig,
			request: func(current uint64) domain.CharacterCreatePayload {
				payload := validCharacterCreatePayload(current)
				payload.Intelligence = 11
				return payload
			},
			wantErr: "intelligence",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ids := &counterIDSource{}
			store := &fakeRosterStore{}
			effects := testutil.NewFakeOrderedEffectSink[Effect]()
			service := New(Config{IDs: ids, Enqueue: effects.Enqueue, RosterStore: store})
			current := test.prepare(t, service)
			before := service.Snapshot()
			beforeIDs := ids.next.Load()
			beforeEffects := effects.Calls()

			state, err := service.AddCharacter(test.request(current))
			require.ErrorContains(t, err, test.wantErr)
			require.Equal(t, before, state)
			require.Equal(t, before, service.Snapshot())
			require.Equal(t, beforeIDs, ids.next.Load(), "guard rejection must occur before stable ID allocation")
			require.Empty(t, store.saves, "guard rejection must not persist")
			require.Equal(t, beforeEffects, effects.Calls(), "guard rejection must not publish")
		})
	}
}

func TestAddCharacterPersistenceFailureKeepsCanonicalStateAndRevision(t *testing.T) {
	t.Parallel()

	ids := &counterIDSource{}
	store := &fakeRosterStore{fail: true}
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: ids, Enqueue: effects.Enqueue, RosterStore: store})
	current := installEmptyPlayerConfig(t, service)
	before := service.Snapshot()
	beforeEffects := effects.Calls()

	state, err := service.AddCharacter(validCharacterCreatePayload(current))
	require.ErrorContains(t, err, "save")
	require.Equal(t, before, state)
	require.Equal(t, before, service.Snapshot())
	require.Empty(t, store.saves)
	require.Equal(t, beforeEffects, effects.Calls())
	require.Equal(t, "digest-0", service.runtime.ActivePlayerConfig.ContentDigest)
}

func TestUpdateCharacterPersistsCompleteProfilePreservesIdentityOrderAndNoop(t *testing.T) {
	t.Parallel()

	store := &fakeRosterStore{}
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, RosterStore: store})
	installed, err := service.InstallPlayerConfig(domain.PlayerConfigHandle{
		Path: "/Campaigns/players.json", Version: 1, Name: "Players", ContentDigest: "digest-0",
	}, []domain.CharacterRosterEntry{
		{ID: "mara", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true},
		{ID: "boone", Name: "Boone", Intelligence: 3, HackerPerkAvailable: false},
		{ID: "arcade", Name: "Arcade", Intelligence: 9, HackerPerkAvailable: true},
	})
	require.NoError(t, err)
	baselineEffects := effects.Calls()

	request := domain.CharacterUpdatePayload{
		CharacterID: "boone", Name: "  Craig Boone  ", Intelligence: 10,
		HackerPerkAvailable: true, ExpectedRevision: installed.Revision,
	}
	updated, err := service.UpdateCharacter(request)
	require.NoError(t, err)
	require.Equal(t, installed.Revision+1, updated.Revision)
	require.Equal(t, []domain.CharacterID{"mara", "boone", "arcade"}, []domain.CharacterID{
		updated.Roster[0].ID, updated.Roster[1].ID, updated.Roster[2].ID,
	})
	require.Equal(t, domain.MasterRosterEntry{
		ID: "boone", Name: "Craig Boone", Intelligence: 10, HackerPerkAvailable: true,
	}, updated.Roster[1])
	require.Len(t, store.saves, 1)
	require.Equal(t, []domain.CharacterID{"mara", "boone", "arcade"}, []domain.CharacterID{
		store.saves[0].Roster[0].ID, store.saves[0].Roster[1].ID, store.saves[0].Roster[2].ID,
	})
	require.Equal(t, domain.CharacterRosterEntry{
		ID: "boone", Name: "Craig Boone", Intelligence: 10, HackerPerkAvailable: true,
	}, store.saves[0].Roster[1])
	require.Equal(t, baselineEffects+1, effects.Calls())
	require.Equal(t, "digest-1", service.runtime.ActivePlayerConfig.ContentDigest)

	noOpRequest := request
	noOpRequest.Name = "Craig Boone"
	noOpRequest.ExpectedRevision = updated.Revision
	beforeNoOp := service.Snapshot()
	noOp, noOpErr := service.UpdateCharacter(noOpRequest)
	require.NoError(t, noOpErr)
	require.Equal(t, beforeNoOp, noOp)
	require.Equal(t, beforeNoOp.Revision, service.Revision())
	require.Len(t, store.saves, 1, "no-op update must not write player config")
	require.Equal(t, baselineEffects+1, effects.Calls(), "no-op update must not publish")

	replayed, replayErr := service.UpdateCharacter(request)
	require.ErrorContains(t, replayErr, "revision")
	require.Equal(t, beforeNoOp, replayed, "stale replay must return the authoritative state")
	require.Len(t, store.saves, 1, "stale replay must not repeat persistence")
	require.Equal(t, baselineEffects+1, effects.Calls(), "stale replay must not publish")
}

func TestDeleteCharacterPreservesSurvivorOrderAndRejectsDuplicateRevision(t *testing.T) {
	t.Parallel()

	store := &fakeRosterStore{}
	effects := testutil.NewFakeOrderedEffectSink[Effect]()
	service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, RosterStore: store})
	installed, err := service.InstallPlayerConfig(domain.PlayerConfigHandle{
		Path: "/Campaigns/players.json", Version: 1, Name: "Players", ContentDigest: "digest-0",
	}, []domain.CharacterRosterEntry{
		{ID: "mara", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true},
		{ID: "boone", Name: "Boone", Intelligence: 3},
		{ID: "arcade", Name: "Arcade", Intelligence: 9, HackerPerkAvailable: true},
	})
	require.NoError(t, err)
	baselineEffects := effects.Calls()

	request := domain.CharacterDeletePayload{CharacterID: "boone", ExpectedRevision: installed.Revision}
	deleted, err := service.DeleteCharacter(request)
	require.NoError(t, err)
	require.Equal(t, installed.Revision+1, deleted.Revision)
	require.Equal(t, []domain.CharacterID{"mara", "arcade"}, []domain.CharacterID{
		deleted.Roster[0].ID, deleted.Roster[1].ID,
	})
	require.Len(t, store.saves, 1)
	require.Equal(t, []domain.CharacterID{"mara", "arcade"}, []domain.CharacterID{
		store.saves[0].Roster[0].ID, store.saves[0].Roster[1].ID,
	})
	require.Equal(t, baselineEffects+1, effects.Calls())

	replayed, replayErr := service.DeleteCharacter(request)
	require.ErrorContains(t, replayErr, "revision")
	require.Equal(t, deleted, replayed)
	require.Len(t, store.saves, 1)
	require.Equal(t, baselineEffects+1, effects.Calls())
}

func TestRosterMutationsRejectActiveBroadcastWithoutPersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command func(service *Service) (*domain.MasterCoordinationState, error)
	}{
		{
			name: "add",
			command: func(service *Service) (*domain.MasterCoordinationState, error) {
				return service.AddCharacter(domain.CharacterCreatePayload{
					Name: "Arcade", Intelligence: 9, HackerPerkAvailable: true,
					ExpectedRevision: service.Revision(),
				})
			},
		},
		{
			name: "update",
			command: func(service *Service) (*domain.MasterCoordinationState, error) {
				return service.UpdateCharacter(domain.CharacterUpdatePayload{
					CharacterID: "mara", Name: "Mara Voss", Intelligence: 10,
					HackerPerkAvailable: false, ExpectedRevision: service.Revision(),
				})
			},
		},
		{
			name: "delete",
			command: func(service *Service) (*domain.MasterCoordinationState, error) {
				return service.DeleteCharacter(domain.CharacterDeletePayload{
					CharacterID: "mara", ExpectedRevision: service.Revision(),
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeRosterStore{}
			effects := testutil.NewFakeOrderedEffectSink[Effect]()
			service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, RosterStore: store})
			_, err := service.InstallPlayerConfig(domain.PlayerConfigHandle{
				Path: "/Campaigns/players.json", Version: 1, Name: "Players", ContentDigest: "digest-0",
			}, []domain.CharacterRosterEntry{{ID: "mara", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true}})
			require.NoError(t, err)
			_, err = service.StartBroadcast()
			require.NoError(t, err)
			before := service.Snapshot()
			beforeEffects := effects.Calls()

			state, commandErr := test.command(service)
			require.ErrorContains(t, commandErr, "broadcast")
			require.Equal(t, before, state)
			require.Equal(t, before, service.Snapshot())
			require.Empty(t, store.saves)
			require.Equal(t, beforeEffects, effects.Calls())
		})
	}
}

func TestUpdateAndDeletePersistenceConflictsKeepAuthoritativeState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		saveErr      error
		wantGuidance bool
		command      func(service *Service) (*domain.MasterCoordinationState, error)
	}{
		{
			name: "update stale content digest", saveErr: errors.New("active player configuration is missing, unreadable, or changed; reopen or reselect it"), wantGuidance: true,
			command: func(service *Service) (*domain.MasterCoordinationState, error) {
				return service.UpdateCharacter(domain.CharacterUpdatePayload{
					CharacterID: "mara", Name: "Mara Voss", Intelligence: 10,
					HackerPerkAvailable: false, ExpectedRevision: service.Revision(),
				})
			},
		},
		{
			name: "update atomic save failure", saveErr: errors.New("injected atomic replacement failure"),
			command: func(service *Service) (*domain.MasterCoordinationState, error) {
				return service.UpdateCharacter(domain.CharacterUpdatePayload{
					CharacterID: "mara", Name: "Mara Voss", Intelligence: 10,
					HackerPerkAvailable: false, ExpectedRevision: service.Revision(),
				})
			},
		},
		{
			name: "delete stale content digest", saveErr: errors.New("active player configuration is missing, unreadable, or changed; reopen or reselect it"), wantGuidance: true,
			command: func(service *Service) (*domain.MasterCoordinationState, error) {
				return service.DeleteCharacter(domain.CharacterDeletePayload{
					CharacterID: "mara", ExpectedRevision: service.Revision(),
				})
			},
		},
		{
			name: "delete atomic save failure", saveErr: errors.New("injected atomic replacement failure"),
			command: func(service *Service) (*domain.MasterCoordinationState, error) {
				return service.DeleteCharacter(domain.CharacterDeletePayload{
					CharacterID: "mara", ExpectedRevision: service.Revision(),
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeRosterStore{saveErr: test.saveErr}
			effects := testutil.NewFakeOrderedEffectSink[Effect]()
			service := New(Config{IDs: &counterIDSource{}, Enqueue: effects.Enqueue, RosterStore: store})
			_, err := service.InstallPlayerConfig(domain.PlayerConfigHandle{
				Path: "/Campaigns/players.json", Version: 1, Name: "Players", ContentDigest: "digest-0",
			}, []domain.CharacterRosterEntry{{ID: "mara", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true}})
			require.NoError(t, err)
			before := service.Snapshot()
			beforeEffects := effects.Calls()

			state, commandErr := test.command(service)
			require.ErrorContains(t, commandErr, test.saveErr.Error())
			if test.wantGuidance {
				require.ErrorContains(t, commandErr, "reopen or reselect it")
			}
			require.Equal(t, before, state, "failure result must carry authoritative state")
			require.Equal(t, before, service.Snapshot())
			require.Empty(t, store.saves)
			require.Equal(t, beforeEffects, effects.Calls())
			require.NotNil(t, service.runtime.ActivePlayerConfig)
			require.Equal(t, "digest-0", service.runtime.ActivePlayerConfig.ContentDigest)
		})
	}
}

func TestReplaceTerminalGroupsRequiresCurrentCoordinationRevisionAndRejectsReplay(t *testing.T) {
	t.Parallel()

	groups := []domain.TerminalGroup{
		{ID: "route", Name: "Renamed route", TerminalIDs: []string{"terminal-1", "terminal-b"}},
	}
	store := &recordingTerminalGroupStore{mutation: TerminalGroupMutation{
		Changed:  true,
		Revision: 42,
		Session: domain.Session{
			Version: 1, Name: "Grouped session",
			Terminals: []domain.Terminal{
				{
					ID: "terminal-1", Name: "Terminal 1",
					Root: domain.ContentNode{
						ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{},
					},
				},
				{
					ID: "terminal-b", Name: "Terminal B",
					Root: domain.ContentNode{
						ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{},
					},
				},
			},
			TerminalGroups: groups,
		},
	}}
	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.terminalGroupStore = store
	before := fixture.service.Snapshot()
	beforeEffects := fixture.effects.Calls()
	candidate := domain.TerminalGroupCandidate{
		TerminalGroups:               groups,
		ExpectedSessionRevision:      41,
		ExpectedCoordinationRevision: before.Revision,
	}
	staleCandidate := candidate
	staleCandidate.ExpectedCoordinationRevision--
	staleState, staleMutation, staleErr := fixture.service.ReplaceTerminalGroups(t.Context(), staleCandidate)
	require.ErrorContains(t, staleErr, "coordination revision")
	assert.Nil(t, staleMutation)
	assert.Equal(t, before, staleState)
	assert.Equal(t, before, fixture.service.Snapshot())
	assert.Empty(t, store.calls)
	assert.Equal(t, beforeEffects, fixture.effects.Calls())

	operationContext := context.WithValue(t.Context(), terminalGroupContextKey{}, "group-mutation")
	state, mutation, err := fixture.service.ReplaceTerminalGroups(operationContext, candidate)
	require.NoError(t, err)
	require.NotNil(t, mutation)
	assert.Equal(t, uint64(42), mutation.Revision)
	assert.Equal(t, before.Revision+1, state.Revision)
	require.Len(t, store.calls, 1)
	assert.Equal(t, uint64(41), store.calls[0].expectedRevision)
	assert.Equal(t, groups, store.calls[0].groups)
	assert.Equal(t, "group-mutation", store.calls[0].ctx.Value(terminalGroupContextKey{}))
	assert.Greater(t, fixture.effects.Calls(), beforeEffects)

	accepted := fixture.service.Snapshot()
	acceptedEffects := fixture.effects.Calls()
	replayed, repeatedMutation, repeatedErr := fixture.service.ReplaceTerminalGroups(t.Context(), candidate)
	require.ErrorContains(t, repeatedErr, "coordination revision")
	assert.Nil(t, repeatedMutation)
	assert.Equal(t, accepted, replayed)
	assert.Equal(t, accepted, fixture.service.Snapshot())
	assert.Len(t, store.calls, 1, "stale replay reached durable group storage")
	assert.Equal(t, acceptedEffects, fixture.effects.Calls())
}

func TestReplaceTerminalGroupsRejectsCandidatesThatInvalidatePendingNavigation(t *testing.T) {
	t.Parallel()

	for _, direction := range []domain.TerminalNavigationDirection{
		domain.TerminalNavigationForward,
		domain.TerminalNavigationReturn,
	} {
		t.Run(string(direction), func(t *testing.T) {
			t.Parallel()

			store := &recordingTerminalGroupStore{}
			fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
			fixture.service.terminalGroupStore = store
			fixture.service.commit(func(root *domain.ProcessRuntime) transition {
				root.PendingTerminalNavigation = &domain.PendingTerminalNavigation{
					RequestID: "pending-group-change", BroadcastID: root.Broadcast.ID,
					Direction: direction, SourceTerminalID: fixture.terminalID,
					TargetTerminalID: "terminal-b",
					ReturnPoint:      domain.TerminalReturnPoint{TerminalID: "terminal-b"},
				}
				return transition{accepted: true}
			})
			before := fixture.service.Snapshot()
			beforeEffects := fixture.effects.Calls()
			candidate := domain.TerminalGroupCandidate{
				TerminalGroups: []domain.TerminalGroup{
					{ID: "source", Name: "Source", TerminalIDs: []string{fixture.terminalID}},
					{ID: "target", Name: "Target", TerminalIDs: []string{"terminal-b"}},
				},
				ExpectedSessionRevision: 17, ExpectedCoordinationRevision: before.Revision,
			}

			state, mutation, err := fixture.service.ReplaceTerminalGroups(t.Context(), candidate)
			require.ErrorContains(t, err, "pending")
			require.ErrorContains(t, err, string(direction))
			require.ErrorContains(t, err, fixture.terminalID)
			require.ErrorContains(t, err, "terminal-b")
			assert.Nil(t, mutation)
			assert.Equal(t, before, state)
			assert.Equal(t, before, fixture.service.Snapshot())
			assert.Empty(t, store.calls)
			assert.Equal(t, beforeEffects, fixture.effects.Calls())
		})
	}
}

func TestReplaceTerminalGroupsPreservesSanitizedStoreRejectionAndCanonicalRevision(t *testing.T) {
	t.Parallel()

	canonical := domain.Session{
		Version: 1,
		Name:    "Canonical rejection",
		Terminals: []domain.Terminal{
			{ID: "terminal-1", Name: "Terminal 1", Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}}},
			{ID: "terminal-b", Name: "Terminal B", Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}}},
		},
		TerminalGroups: []domain.TerminalGroup{
			{ID: "left", Name: "Left", TerminalIDs: []string{"terminal-1"}},
			{ID: "right", Name: "Right", TerminalIDs: []string{"terminal-b"}},
		},
	}
	const feedback = `terminal group candidate invalidates authored transitions: ` +
		`terminal transition command "open-b" in terminal "terminal-1" targets terminal "terminal-b" and crosses groups "left" and "right"; ` +
		`terminal transition command "open-b-backup" in terminal "terminal-1" targets terminal "terminal-b" and crosses groups "left" and "right"`
	store := &recordingTerminalGroupStore{
		mutation: TerminalGroupMutation{Revision: 43, Session: canonical},
		err:      &TerminalGroupStoreRejection{Message: feedback},
	}
	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.terminalGroupStore = store
	before := fixture.service.Snapshot()
	beforeEffects := fixture.effects.Calls()
	candidate := domain.TerminalGroupCandidate{
		TerminalGroups: []domain.TerminalGroup{
			{ID: "route", Name: "Route", TerminalIDs: []string{fixture.terminalID}},
		},
		ExpectedSessionRevision: 42, ExpectedCoordinationRevision: before.Revision,
	}

	state, mutation, err := fixture.service.ReplaceTerminalGroups(t.Context(), candidate)

	require.EqualError(t, err, feedback)
	require.ErrorContains(t, err, `command "open-b"`)
	require.ErrorContains(t, err, `command "open-b-backup"`)
	require.NotNil(t, mutation)
	assert.False(t, mutation.Changed)
	assert.Equal(t, uint64(43), mutation.Revision)
	assert.Equal(t, canonical, mutation.Session)
	assert.Equal(t, before, state)
	assert.Equal(t, before, fixture.service.Snapshot())
	require.Len(t, store.calls, 1)
	assert.Equal(t, beforeEffects, fixture.effects.Calls())
}

func TestReplaceTerminalGroupsRejectsActiveRouteSplitAtomically(t *testing.T) {
	t.Parallel()

	store := &recordingTerminalGroupStore{}
	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.terminalGroupStore = store
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		active := "terminal-c"
		root.Broadcast.ActiveTerminalID = &active
		root.Broadcast.Route = []domain.TerminalReturnPoint{
			{TerminalID: "terminal-a", TerminalName: "A"},
			{TerminalID: "terminal-b", TerminalName: "B"},
		}
		return transition{accepted: true}
	})
	before := fixture.service.Snapshot()
	beforeEffects := fixture.effects.Calls()
	candidate := domain.TerminalGroupCandidate{
		TerminalGroups: []domain.TerminalGroup{
			{ID: "route-prefix", Name: "Route prefix", TerminalIDs: []string{"terminal-a", "terminal-b"}},
			{ID: "active", Name: "Active", TerminalIDs: []string{"terminal-c"}},
		},
		ExpectedSessionRevision: 23, ExpectedCoordinationRevision: before.Revision,
	}

	state, mutation, err := fixture.service.ReplaceTerminalGroups(t.Context(), candidate)
	require.ErrorContains(t, err, "route")
	assert.Nil(t, mutation)
	assert.Equal(t, before, state)
	assert.Equal(t, before, fixture.service.Snapshot())
	assert.Empty(t, store.calls)
	assert.Equal(t, beforeEffects, fixture.effects.Calls())
}

func TestReplaceTerminalGroupsRejectsSeededPrefixReorder(t *testing.T) {
	t.Parallel()

	store := &recordingTerminalGroupStore{}
	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.terminalGroupStore = store
	fixture.service.commit(func(root *domain.ProcessRuntime) transition {
		active := "terminal-c"
		root.Broadcast.ActiveTerminalID = &active
		root.Broadcast.Route = []domain.TerminalReturnPoint{
			{
				TerminalID: "terminal-a", TerminalName: "A", Origin: domain.TerminalReturnInitialPrefix,
				GroupID: "route", GroupPosition: 0,
			},
			{
				TerminalID: "terminal-b", TerminalName: "B", Origin: domain.TerminalReturnInitialPrefix,
				GroupID: "route", GroupPosition: 1,
			},
		}
		return transition{accepted: true}
	})
	before := fixture.service.Snapshot()
	beforeEffects := fixture.effects.Calls()
	candidate := domain.TerminalGroupCandidate{
		TerminalGroups: []domain.TerminalGroup{
			{ID: "route", Name: "Route", TerminalIDs: []string{"terminal-b", "terminal-a", "terminal-c", "terminal-d"}},
		},
		ExpectedSessionRevision: 29, ExpectedCoordinationRevision: before.Revision,
	}

	state, mutation, err := fixture.service.ReplaceTerminalGroups(t.Context(), candidate)
	require.ErrorContains(t, err, "seeded")
	assert.Nil(t, mutation)
	assert.Equal(t, before, state)
	assert.Equal(t, before, fixture.service.Snapshot())
	assert.Empty(t, store.calls)
	assert.Equal(t, beforeEffects, fixture.effects.Calls())
}

func TestReplaceTerminalGroupsProtectsInitializedSeedChainAndPendingReturnAdjacency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		prepare   func(*testing.T, us2Fixture, *groupAwareNavigationCatalog)
		candidate []string
		wantError string
	}{
		{
			name:      "initialized terminal position",
			candidate: []string{"terminal-a", "terminal-b", "terminal-d", "terminal-c"},
			wantError: "initialized terminal",
		},
		{
			name: "seeded successor chain",
			prepare: func(t *testing.T, fixture us2Fixture, _ *groupAwareNavigationCatalog) {
				t.Helper()
				fixture.service.commit(func(root *domain.ProcessRuntime) transition {
					root.Broadcast.Route = append([]domain.TerminalReturnPoint(nil), root.Broadcast.Route[1:]...)
					return transition{accepted: true}
				})
			},
			candidate: []string{"terminal-a", "terminal-b", "terminal-c", "terminal-d"},
			wantError: "successor chain",
		},
		{
			name: "pending seeded return adjacency",
			prepare: func(t *testing.T, fixture us2Fixture, _ *groupAwareNavigationCatalog) {
				t.Helper()
				requestOrderedGroupReturn(t, fixture, "terminal-c", "pending-adjacency")
			},
			candidate: []string{"terminal-b", "terminal-a", "terminal-c", "terminal-d"},
			wantError: "pending return adjacency",
		},
		{
			name: "active seeded return adjacency after consuming prefix",
			prepare: func(t *testing.T, fixture us2Fixture, _ *groupAwareNavigationCatalog) {
				t.Helper()
				pending := requestOrderedGroupReturn(t, fixture, "terminal-c", "consume-prefix")
				_, err := fixture.service.ResolveTerminalNavigation(
					pending.RequestID,
					domain.TerminalNavigationApprove,
				)
				require.NoError(t, err)
			},
			candidate: []string{"terminal-a", "terminal-d", "terminal-c", "terminal-b"},
			wantError: "active seeded return adjacency",
		},
		{
			name: "mixed route preserves current return but breaks deeper seeded adjacency",
			prepare: func(t *testing.T, fixture us2Fixture, catalog *groupAwareNavigationCatalog) {
				t.Helper()
				pending := requestOrderedGroupReturn(t, fixture, "terminal-c", "mixed-route-return")
				_, err := fixture.service.ResolveTerminalNavigation(
					pending.RequestID,
					domain.TerminalNavigationApprove,
				)
				require.NoError(t, err)

				catalog.transitions["terminal-b/forward-to-d"] = domain.TerminalTransitionTarget{
					SourceTerminalID: "terminal-b", SourceTerminalName: "B",
					CommandID: "forward-to-d", CommandName: "Open D", Target: catalog.terminals["terminal-d"],
				}
				fixture.service.commit(func(root *domain.ProcessRuntime) transition {
					root.Broadcast.TerminalRuntimes["terminal-b"].Tree.Children = append(
						root.Broadcast.TerminalRuntimes["terminal-b"].Tree.Children,
						domain.ContentNode{
							ID: "forward-to-d", Type: domain.NodeCommand, Name: "Open D",
							TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-d"},
						},
					)
					return transition{accepted: true}
				})
				selected := fixture.service.DispatchPlayerAction(fixture.controllerConnection, domain.RuntimeCommand{
					RequestID: "mixed-route-forward", BroadcastID: fixture.broadcastID, TerminalID: "terminal-b",
					Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "forward-to-d",
					PayloadFingerprint: "mixed-route-forward",
				})
				require.True(t, selected.Accepted)
				forward := fixture.service.Snapshot().PendingTerminalNavigation
				require.NotNil(t, forward)
				_, err = fixture.service.ResolveTerminalNavigation(
					forward.RequestID,
					domain.TerminalNavigationApprove,
				)
				require.NoError(t, err)
				assertOrderedRouteIDs(t, fixture.service, []string{"terminal-a", "terminal-b"})
			},
			candidate: []string{"terminal-a", "terminal-d", "terminal-c", "terminal-b"},
			wantError: "seeded return successor adjacency",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture, catalog := newOrderedGroupStartFixture(t)
			store := &recordingTerminalGroupStore{}
			fixture.service.terminalGroupStore = store
			_, err := fixture.service.RequestTerminalActivation(terminalTarget("terminal-c", "Terminal C"))
			require.NoError(t, err)
			if test.prepare != nil {
				test.prepare(t, fixture, catalog)
			}
			before := fixture.service.Snapshot()
			beforeEffects := fixture.effects.Calls()
			candidate := domain.TerminalGroupCandidate{
				TerminalGroups: []domain.TerminalGroup{{
					ID: "ordered-route", Name: "Ordered route", TerminalIDs: test.candidate,
				}},
				ExpectedSessionRevision:      30,
				ExpectedCoordinationRevision: before.Revision,
			}

			state, mutation, replaceErr := fixture.service.ReplaceTerminalGroups(t.Context(), candidate)

			require.ErrorContains(t, replaceErr, test.wantError)
			assert.Nil(t, mutation)
			assert.Equal(t, before, state)
			assert.Equal(t, before, fixture.service.Snapshot())
			assert.Empty(t, store.calls)
			assert.Equal(t, beforeEffects, fixture.effects.Calls())
		})
	}
}

func TestReplaceTerminalGroupsPersistenceFailureHasNoRuntimeFragment(t *testing.T) {
	t.Parallel()

	store := &recordingTerminalGroupStore{err: errors.New("private campaign path: atomic replacement failed")}
	fixture := newUS2Fixture(t, &recordingTerminalRuntime{})
	fixture.service.terminalGroupStore = store
	before := fixture.service.Snapshot()
	beforeEffects := fixture.effects.Calls()
	candidate := domain.TerminalGroupCandidate{
		TerminalGroups: []domain.TerminalGroup{
			{ID: "route", Name: "Renamed route", TerminalIDs: []string{fixture.terminalID}},
		},
		ExpectedSessionRevision: 31, ExpectedCoordinationRevision: before.Revision,
	}

	state, mutation, err := fixture.service.ReplaceTerminalGroups(t.Context(), candidate)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "private campaign path")
	assert.Nil(t, mutation)
	assert.Equal(t, before, state)
	assert.Equal(t, before, fixture.service.Snapshot())
	require.Len(t, store.calls, 1)
	assert.Equal(t, beforeEffects, fixture.effects.Calls())
}

type terminalGroupStoreCall struct {
	ctx              context.Context
	groups           []domain.TerminalGroup
	expectedRevision uint64
}

type terminalGroupContextKey struct{}

type recordingTerminalGroupStore struct {
	mutation TerminalGroupMutation
	err      error
	calls    []terminalGroupStoreCall
}

func (store *recordingTerminalGroupStore) ReplaceTerminalGroups(
	ctx context.Context,
	groups []domain.TerminalGroup,
	expectedRevision uint64,
) (TerminalGroupMutation, error) {
	cloned := make([]domain.TerminalGroup, len(groups))
	for index, group := range groups {
		cloned[index] = group
		cloned[index].TerminalIDs = append([]string(nil), group.TerminalIDs...)
	}
	store.calls = append(store.calls, terminalGroupStoreCall{
		ctx: ctx, groups: cloned, expectedRevision: expectedRevision,
	})
	return store.mutation, store.err
}

func validCharacterCreatePayload(revision uint64) domain.CharacterCreatePayload {
	return domain.CharacterCreatePayload{
		Name:                "Mara",
		Intelligence:        1,
		HackerPerkAvailable: true,
		ExpectedRevision:    revision,
	}
}

func installEmptyPlayerConfig(t *testing.T, service *Service) uint64 {
	t.Helper()
	state, err := service.InstallPlayerConfig(domain.PlayerConfigHandle{
		Path: "/Campaigns/players.json", Version: 1, Name: "Players", ContentDigest: "digest-0",
	}, []domain.CharacterRosterEntry{})
	require.NoError(t, err)
	return state.Revision
}

func TestPlayerConfigReplacementRequiresNoBroadcastAndPreservesRuntimeOnFailure(t *testing.T) {
	t.Parallel()

	store := &fakeRosterStore{}
	service := New(Config{IDs: &counterIDSource{}, RosterStore: store})
	handle := domain.PlayerConfigHandle{Path: "/Campaigns/players.json", Version: 1, Name: "Players"}
	if _, err := service.InstallPlayerConfig(handle, []domain.CharacterRosterEntry{{ID: "mara", Name: "Mara", Intelligence: 1}}); err != nil {
		require.NoError(t, err)
	}
	if _, err := service.StartBroadcast(); err != nil {
		require.NoError(t, err)
	}
	before := service.Snapshot()
	{
		_, err := service.InstallPlayerConfig(domain.PlayerConfigHandle{Path: "/Campaigns/other.json", Version: 1, Name: "Other"}, nil)
		require.False(t, err == nil,
			"InstallPlayerConfig() during broadcast succeeded")
	}
	{

		after := service.Snapshot()
		require.Falsef(t, !cmp.Equal(after, before),
			"rejected replacement changed state\nbefore=%#v\nafter=%#v", before, after)
	}

}

type fakeRosterStore struct {
	fail    bool
	saveErr error
	handles []domain.PlayerConfigHandle
	saves   []domain.PlayerConfig
}

func (store *fakeRosterStore) Save(handle domain.PlayerConfigHandle, roster []domain.CharacterRosterEntry) (domain.PlayerConfigHandle, error) {
	if store.saveErr != nil {
		return domain.PlayerConfigHandle{}, store.saveErr
	}
	if store.fail {
		return domain.PlayerConfigHandle{}, errors.New("injected player-config write failure")
	}
	store.saves = append(store.saves, domain.PlayerConfig{
		Version: handle.Version,
		Name:    handle.Name,
		Roster:  append([]domain.CharacterRosterEntry(nil), roster...),
	})
	store.handles = append(store.handles, handle)
	handle.ContentDigest = fmt.Sprintf("digest-%d", len(store.saves))
	return handle, nil
}
