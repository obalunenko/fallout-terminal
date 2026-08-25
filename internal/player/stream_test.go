package player

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/obalunenko/Fallout-Terminal/internal/domain"
	playerv1 "github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/player/v1"
	"github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/player/v1/playerv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionQueueIsBoundedNonblockingAndRecoversOnlyFromANewSnapshot(t *testing.T) {
	snapshot := subscriptionSnapshot(1)
	stream := NewSubscription(t.Context(), "physical-1", "logical-1", snapshot, 0)
	require.Equal(t, defaultSubscriptionQueueSize, cap(stream.updates))
	for revision := uint64(2); revision <= defaultSubscriptionQueueSize+1; revision++ {
		require.True(t, stream.Offer(subscriptionUpdate(revision)), "revision %d", revision)
	}

	started := time.Now()
	require.False(t, stream.Offer(subscriptionUpdate(defaultSubscriptionQueueSize+2)))
	require.Less(t, time.Since(started), 50*time.Millisecond)
	require.Eventually(t, func() bool {
		select {
		case <-stream.Done():
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
	require.ErrorIs(t, context.Cause(stream.context), errSubscriptionQueueOverflow)
	require.False(t, stream.Offer(subscriptionUpdate(defaultSubscriptionQueueSize+3)))

	recovered := NewSubscription(t.Context(), "physical-2", "logical-1", subscriptionSnapshot(50), 2)
	require.False(t, recovered.Offer(subscriptionUpdate(49)))
	select {
	case <-recovered.Done():
	case <-time.After(time.Second):
		assert.FailNow(t, "stale increment did not terminate the physical recovery stream")
	}
	require.ErrorIs(t, context.Cause(recovered.context), errSubscriptionRevisionRegressed)
	fresh := NewSubscription(t.Context(), "physical-3", "logical-1", subscriptionSnapshot(50), 2)
	require.True(t, fresh.Offer(subscriptionUpdate(51)))
	require.Equal(t, uint64(51), (<-fresh.Updates()).GetUpdate().GetRevision())
}

func TestCommandExecutionStreamIsMonotonicAndReconnectsFromCompleteStateAfterOverflow(t *testing.T) {
	t.Parallel()

	stream := NewSubscription(t.Context(), "physical-monotonic", "logical-1", commandExecutionSnapshot(20, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_PENDING, false), 3)
	require.True(t, stream.Offer(commandExecutionUpdate(21, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_REJECTED, false)))
	require.True(t, stream.Offer(commandExecutionUpdate(22, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_UNSPECIFIED, true)))

	first := stream.Snapshot().GetSnapshot()
	require.Equal(t, uint64(20), first.GetRevision())
	require.Equal(t, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_PENDING,
		first.GetTerminalPresentation().GetLiveTerminal().GetCommandExecution().GetPhase())
	rejected := (<-stream.Updates()).GetUpdate()
	require.Equal(t, uint64(21), rejected.GetRevision())
	require.Equal(t, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_REJECTED,
		rejected.GetTerminalPresentation().GetLiveTerminal().GetCommandExecution().GetPhase())
	completed := (<-stream.Updates()).GetUpdate()
	assertCompletedCommandStreamState(t, 22, completed.GetRevision(), completed.GetTerminalPresentation().GetLiveTerminal())
	require.True(t, stream.Offer(commandExecutionUpdate(22, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_UNSPECIFIED, true)), "same revision is an idempotent no-op")
	select {
	case unexpected := <-stream.Updates():
		assert.Failf(t, "duplicate revision delivered", "update = %#v", unexpected)
	default:
	}
	require.False(t, stream.Offer(commandExecutionUpdate(21, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_REJECTED, false)), "older command state must terminate the physical stream")
	select {
	case <-stream.Done():
	case <-time.After(time.Second):
		assert.FailNow(t, "stream accepted a regressing command revision")
	}

	overflowed := NewSubscription(t.Context(), "physical-overflow", "logical-1", commandExecutionSnapshot(30, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_PENDING, false), 1)
	require.True(t, overflowed.Offer(commandExecutionUpdate(31, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_REJECTED, false)))
	require.False(t, overflowed.Offer(commandExecutionUpdate(32, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_UNSPECIFIED, true)))
	select {
	case <-overflowed.Done():
	case <-time.After(time.Second):
		assert.FailNow(t, "overflowed command stream remained open")
	}

	// Recovery is entirely server-owned: the new physical stream starts from
	// the current complete snapshot and never needs the dropped increments.
	reconnected := NewSubscription(t.Context(), "physical-reconnected", "logical-1", commandExecutionSnapshot(32, playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_UNSPECIFIED, true), 1)
	reconnectedFirst := reconnected.Snapshot().GetSnapshot()
	assertCompletedCommandStreamState(t, 32, reconnectedFirst.GetRevision(), reconnectedFirst.GetTerminalPresentation().GetLiveTerminal())
	require.True(t, reconnected.Offer(subscriptionUpdate(33)))
	require.Equal(t, uint64(33), (<-reconnected.Updates()).GetUpdate().GetRevision())
}

func commandExecutionSnapshot(revision uint64, phase playerv1.CommandExecutionPhase, completed bool) *playerv1.SubscriptionMessage {
	return &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: &playerv1.PersonalizedSnapshot{
		RecognitionHandle: "recognition-logical-1", Revision: revision,
		PlayerState: &playerv1.PlayerState{
			LogicalSessionId: "logical-1", Role: playerv1.PlayerRole_PLAYER_ROLE_ACTIVE,
			Phase: playerv1.PlayerPhase_PLAYER_PHASE_CONTROLLING,
		},
		TerminalPresentation: commandExecutionTerminalPresentation(phase, completed),
	}}}
}

func commandExecutionUpdate(revision uint64, phase playerv1.CommandExecutionPhase, completed bool) *playerv1.SubscriptionMessage {
	return &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Update{Update: &playerv1.CompoundUpdate{
		Revision: revision, TerminalPresentation: commandExecutionTerminalPresentation(phase, completed),
	}}}
}

func commandExecutionTerminalPresentation(phase playerv1.CommandExecutionPhase, completed bool) *playerv1.TerminalPresentation {
	const commandID = "command-open-doors"
	name := "Open doors"
	if completed {
		name = "Doors open"
	}
	live := &playerv1.LiveTerminal{
		TerminalId: "terminal-1", TerminalName: "Overseer",
		Tree: &playerv1.ContentNode{Id: "root", Name: "ROOT", Content: &playerv1.ContentNode_Folder{Folder: &playerv1.ContentFolder{Children: []*playerv1.ContentNode{{
			Id: commandID, Name: name, Content: &playerv1.ContentNode_Command{Command: &playerv1.ContentCommand{Text: "Doors opened"}},
		}}}}},
		Navigation: &playerv1.NavigationState{Path: []string{"root"}, Mode: playerv1.NavigationMode_NAVIGATION_MODE_LIST},
	}
	if completed {
		commandNodeID := commandID
		live.Navigation.CommandNodeId = &commandNodeID
	} else {
		live.CommandExecution = &playerv1.CommandExecutionPresentation{Phase: phase, CommandNodeId: commandID}
	}
	return &playerv1.TerminalPresentation{Presentation: &playerv1.TerminalPresentation_LiveTerminal{LiveTerminal: live}}
}

func assertCompletedCommandStreamState(t *testing.T, wantRevision, revision uint64, terminal *playerv1.LiveTerminal) {
	t.Helper()
	require.Equal(t, wantRevision, revision)
	require.NotNil(t, terminal)
	require.Nil(t, terminal.GetCommandExecution())
	command := terminal.GetTree().GetFolder().GetChildren()[0]
	require.Equal(t, "Doors open", command.GetName())
	require.Equal(t, "Doors opened", command.GetCommand().GetText())
	require.Equal(t, "command-open-doors", terminal.GetNavigation().GetCommandNodeId())
}

func TestSubscriptionHubIsolatesOverflowingAndCanceledPhysicalSiblings(t *testing.T) {
	hub := NewSubscriptionHub()
	blocked := NewSubscription(t.Context(), "blocked", "logical-1", subscriptionSnapshot(1), 1)
	healthy := NewSubscription(t.Context(), "healthy", "logical-1", subscriptionSnapshot(1), 2)
	canceledContext, cancel := context.WithCancelCause(t.Context())
	canceled := NewSubscription(canceledContext, "canceled", "logical-1", subscriptionSnapshot(1), 1)
	hub.Register(blocked)
	hub.Register(healthy)
	hub.Register(canceled)
	cancel(errors.New("test sibling stream canceled"))
	select {
	case <-canceled.Done():
	case <-time.After(time.Second):
		assert.FailNow(t, "canceled physical stream remained active")
	}

	hub.Offer("logical-1", subscriptionUpdate(2))
	require.Equal(t, uint64(2), (<-healthy.Updates()).GetUpdate().GetRevision())
	hub.Offer("logical-1", subscriptionUpdate(3))
	require.Equal(t, uint64(3), (<-healthy.Updates()).GetUpdate().GetRevision())
	select {
	case <-blocked.Done():
	case <-time.After(time.Second):
		assert.FailNow(t, "overflowing sibling was not isolated")
	}

	hub.mu.Lock()
	_, blockedRegistered := hub.byID["blocked"]
	_, canceledRegistered := hub.byID["canceled"]
	_, healthyRegistered := hub.byID["healthy"]
	hub.mu.Unlock()
	require.False(t, blockedRegistered)
	require.False(t, canceledRegistered)
	require.True(t, healthyRegistered)

	hub.Offer("logical-1", subscriptionUpdate(4))
	require.Equal(t, uint64(4), (<-healthy.Updates()).GetUpdate().GetRevision())
	hub.Unregister("healthy")
	hub.Unregister("healthy")
}

func TestRepresentativeThreeHourStreamReconnectSoak(t *testing.T) {
	t.Parallel()

	const simulatedSeconds = 3 * 60 * 60
	hub := NewSubscriptionHub()
	stream := NewSubscription(t.Context(), "physical-0", "logical-1", subscriptionSnapshot(1), 2)
	hub.Register(stream)
	currentRevision := uint64(1)
	reconnects := 0
	for second := 1; second <= simulatedSeconds; second++ {
		// One authoritative heartbeat-equivalent projection per simulated second
		// exercises long-run ordering without making the suite wait three hours.
		currentRevision++
		hub.Offer("logical-1", subscriptionUpdate(currentRevision))
		select {
		case update := <-stream.Updates():
			require.Equal(t, currentRevision, update.GetUpdate().GetRevision())
		case <-time.After(time.Second):
			assert.FailNowf(t, "assertion failed", "simulated second %d did not deliver revision %d", second, currentRevision)
		}

		// Interrupt and recover every five simulated minutes. Recovery begins
		// from one complete current snapshot and never retries old increments.
		if second%(5*60) == 0 {
			hub.Unregister(stream.ID())
			reconnects++
			stream = NewSubscription(t.Context(), domain.ConnectionID(fmt.Sprintf("physical-%d", reconnects)), "logical-1", subscriptionSnapshot(currentRevision), 2)
			hub.Register(stream)
			require.Equal(t, currentRevision, stream.Snapshot().GetSnapshot().GetRevision())
		}
	}
	require.Equal(t, 36, reconnects)
	require.Equal(t, 1, hub.Count())
	hub.CloseAll()
	require.Equal(t, 0, hub.Count())
}

func TestTargetedMailboxIsNonLossyAndIndependentFromCanonicalQueue(t *testing.T) {
	stream := NewSubscription(t.Context(), "physical-targeted", "logical-1", subscriptionSnapshot(1), 1, "tab-1")
	first := targetedResult("tab-1", 1, "request-1")
	second := targetedResult("tab-1", 1, "request-2")
	require.True(t, stream.PublishTargeted(t.Context(), first))

	published := make(chan bool, 1)
	go func() { published <- stream.PublishTargeted(t.Context(), second) }()
	require.Never(t, func() bool { return len(published) != 0 }, 20*time.Millisecond, time.Millisecond)
	require.True(t, stream.Offer(subscriptionUpdate(2)), "targeted pressure must not consume canonical capacity")
	require.Equal(t, uint64(2), (<-stream.Updates()).GetUpdate().GetRevision())
	require.Equal(t, "request-1", (<-stream.Targeted()).GetPresentationUplinkResult().GetAction().GetRequestId())
	require.True(t, <-published)
	require.Equal(t, "request-2", (<-stream.Targeted()).GetPresentationUplinkResult().GetAction().GetRequestId())

	stream.Close()
	require.False(t, stream.PublishTargeted(t.Context(), targetedResult("tab-1", 1, "request-3")))
}

func TestCloseUplinksRetainsAuthoritativeSubscriptions(t *testing.T) {
	hub := NewSubscriptionHub()
	snapshot := subscriptionSnapshot(1)
	snapshot.GetSnapshot().RecognitionHandle = "recognition-logical-1"
	stream := NewSubscription(t.Context(), "physical-1", "logical-1", snapshot, 1, "tab-1")
	hub.Register(stream)
	uplink, err := hub.BindUplink(t.Context(), PresentationUplinkBinding{
		RecognitionHandle: domain.RecognitionHandle("recognition-logical-1"),
		ClientInstanceID:  "tab-1",
		Generation:        1,
	})
	require.NoError(t, err)

	cause := errors.New("test uplink rotation")
	hub.CloseUplinks(cause)
	require.ErrorIs(t, context.Cause(uplink.Context), cause)
	require.False(t, hub.Current(uplink))
	require.Equal(t, 1, hub.Count())
	select {
	case <-stream.Done():
		assert.FailNow(t, "uplink rotation closed the authoritative subscription")
	default:
	}
}

func TestLatestIntentMailboxReplacesOnlyUnprocessedValue(t *testing.T) {
	mailbox := NewLatestIntentMailbox()
	require.True(t, mailbox.Offer(&playerv1.PresentationIntent{RequestId: "request-1"}))
	require.True(t, mailbox.Offer(&playerv1.PresentationIntent{RequestId: "request-2"}))
	intent, ok := mailbox.Take(t.Context())
	require.True(t, ok)
	require.Equal(t, "request-2", intent.GetRequestId())
	mailbox.Close(errors.New("test mailbox closed"))
	_, ok = mailbox.Take(t.Context())
	require.False(t, ok)
}

func TestLatestIntentMailboxGracefulFinishRetainsFinalPendingValue(t *testing.T) {
	mailbox := NewLatestIntentMailbox()
	result := make(chan *playerv1.PresentationIntent, 1)
	go func() {
		intent, ok := mailbox.Take(t.Context())
		if ok {
			result <- intent
			return
		}
		result <- nil
	}()
	require.True(t, mailbox.Offer(&playerv1.PresentationIntent{RequestId: "final-request"}))
	mailbox.Finish()
	require.Equal(t, "final-request", (<-result).GetRequestId())
	_, ok := mailbox.Take(t.Context())
	require.False(t, ok)
}

func targetedResult(clientID string, generation uint64, requestID string) *playerv1.SubscriptionMessage {
	return &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_PresentationUplinkResult{
		PresentationUplinkResult: &playerv1.PresentationUplinkResult{
			ClientInstanceId: clientID, UplinkGeneration: generation,
			Payload: &playerv1.PresentationUplinkResult_Action{Action: &playerv1.ActionResult{RequestId: requestID}},
		},
	}}
}

func TestConnectServerShutdownIsBoundedWithBlockedAndCanceledPhysicalStreams(t *testing.T) {
	coordinator := newConnectTestCoordinator(t)
	hub := NewSubscriptionHub()
	connectPlayer, err := NewConnectService(ConnectServiceConfig{Coordinator: coordinator, QueueSize: 1, Hub: hub})
	require.NoError(t, err)
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>Player</title>")}}
	serverRoot := context.WithValue(t.Context(), playerContextKey{}, "player-server")
	server, err := NewServer(serverRoot, Config{
		Address: "127.0.0.1:0", Assets: fs.FS(assets), Connect: connectPlayer,
	})
	require.NoError(t, err)
	_, err = server.Start(t.Context())
	require.NoError(t, err)

	client := playerv1connect.NewPlayerServiceClient(http.DefaultClient, server.Info().LocalURL)
	blockedContext, cancelBlocked := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancelBlocked(errors.New("test blocked stream closed")) })
	blocked, err := client.Subscribe(blockedContext, connect.NewRequest(&playerv1.SubscribeRequest{}))
	require.NoError(t, err)
	require.True(t, blocked.Receive(), "stream error: %v", blocked.Err())
	snapshot := blocked.Msg().GetSnapshot()
	require.NotNil(t, snapshot)

	healthyContext, cancelHealthy := context.WithCancelCause(t.Context())
	healthy, err := client.Subscribe(healthyContext, connect.NewRequest(&playerv1.SubscribeRequest{RecognitionHandle: &snapshot.RecognitionHandle}))
	require.NoError(t, err)
	require.True(t, healthy.Receive())
	healthySnapshot := healthy.Msg().GetSnapshot()
	sessionID := healthySnapshot.GetPlayerState().GetLogicalSessionId()
	require.NotEmpty(t, sessionID)

	hub.Offer(domain.LogicalSessionID(sessionID), subscriptionUpdate(healthySnapshot.GetRevision()+1))
	require.True(t, healthy.Receive())

	cancelHealthy(errors.New("test healthy stream closed"))
	require.Eventually(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return len(hub.byID) == 1
	}, time.Second, time.Millisecond)

	shutdownDeadline, stopShutdownDeadline := context.WithTimeoutCause(t.Context(), 5*time.Second, errors.New("test player shutdown timed out"))
	shutdownContext, cancelShutdown := context.WithCancelCause(shutdownDeadline)
	t.Cleanup(func() {
		cancelShutdown(errors.New("test player shutdown completed"))
		stopShutdownDeadline()
	})
	started := time.Now()
	require.NoError(t, server.Stop(shutdownContext))
	require.Equal(t, "player-server", server.context.Value(playerContextKey{}))
	require.ErrorIs(t, context.Cause(server.context), errPlayerServerStopped)
	require.Less(t, time.Since(started), 5*time.Second)
	require.NoError(t, server.Stop(shutdownContext))
	require.Eventually(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return len(hub.byID) == 0
	}, time.Second, time.Millisecond)
}

func TestConnectServerLifetimeSurvivesCompletedStartupContext(t *testing.T) {
	coordinator := newConnectTestCoordinator(t)
	hub := NewSubscriptionHub()
	connectPlayer, err := NewConnectService(ConnectServiceConfig{Coordinator: coordinator, QueueSize: 1, Hub: hub})
	require.NoError(t, err)
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>Player</title>")}}
	serverRoot := context.WithValue(t.Context(), playerContextKey{}, "player-server")
	server, err := NewServer(serverRoot, Config{
		Address: "127.0.0.1:0", Assets: fs.FS(assets), Connect: connectPlayer,
	})
	require.NoError(t, err)
	startupContext, completeStartup := context.WithCancelCause(t.Context())
	_, err = server.Start(startupContext)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupContext, cancelCleanup := context.WithTimeoutCause(
			context.WithoutCancel(t.Context()), time.Second, errors.New("test player cleanup timed out"),
		)
		defer cancelCleanup()
		require.NoError(t, server.Stop(cleanupContext))
	})

	completeStartup(errors.New("test player startup completed"))
	require.Never(t, func() bool { return server.context.Err() != nil }, 50*time.Millisecond, time.Millisecond,
		"completed startup context canceled the committed player server")

	client := playerv1connect.NewPlayerServiceClient(http.DefaultClient, server.Info().LocalURL)
	streamContext, cancelStream := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancelStream(errors.New("test post-start stream closed")) })
	stream, err := client.Subscribe(streamContext, connect.NewRequest(&playerv1.SubscribeRequest{}))
	require.NoError(t, err)
	require.True(t, stream.Receive(), "initial snapshot error: %v", stream.Err())
	snapshot := stream.Msg().GetSnapshot()
	require.NotNil(t, snapshot)
	sessionID := snapshot.GetPlayerState().GetLogicalSessionId()
	require.NotEmpty(t, sessionID)

	hub.Offer(domain.LogicalSessionID(sessionID), subscriptionUpdate(snapshot.GetRevision()+1))
	require.True(t, stream.Receive(), "post-start update error: %v", stream.Err())
	require.Equal(t, snapshot.GetRevision()+1, stream.Msg().GetUpdate().GetRevision())
	require.NoError(t, server.context.Err())
}

type playerContextKey struct{}

func subscriptionSnapshot(revision uint64) *playerv1.SubscriptionMessage {
	return &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: &playerv1.PersonalizedSnapshot{Revision: revision}}}
}

func subscriptionUpdate(revision uint64) *playerv1.SubscriptionMessage {
	return &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Update{Update: &playerv1.CompoundUpdate{Revision: revision}}}
}
