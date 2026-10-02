package player

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/control"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1/playerv1connect"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

const (
	edgeTestUsername = "players"
	edgeTestPassword = "synthetic-player-input"
)

func TestFirstPublicSnapshotRestoresControllerPresentation(t *testing.T) {
	domainSnapshot := &domain.PersonalizedSnapshot{
		RecognitionHandle: "recognition-reconnect", Revision: 42,
		PlayerState: &domain.PlayerState{
			SessionID: "session-observer", FallbackName: "PLAYER 2",
			Role: domain.PlayerRoleObserver, Phase: domain.PlayerPhaseObserving,
			BroadcastID: "broadcast-1", ActiveTerminalID: "terminal-1",
		},
		Terminal: domain.TerminalPresentation{Live: &domain.PublicLiveState{
			TerminalID: "terminal-1", TerminalName: "Overseer",
			Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT"},
			Nav:  domain.NavState{Path: []string{"root"}, Mode: "list"},
			Presentation: domain.ControllerTerminalPresentation{
				Kind: domain.ControllerTerminalPresentationMenu, ContextKey: "menu:root", TargetID: "status",
			},
		}},
	}
	generated, err := SnapshotToProto(domainSnapshot)
	require.NoError(t, err)
	got := generated.GetTerminalPresentation().GetLiveTerminal().GetControllerPresentation()
	require.Equal(t, "menu:root", got.GetContextKey())
	require.Equal(t, "status", got.GetMenu().GetTargetId())

	stream := NewSubscription(t.Context(), "physical-reconnect", "session-observer", &playerv1.SubscriptionMessage{
		Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: generated},
	}, 2)
	t.Cleanup(stream.Close)
	cloned := stream.Snapshot().GetSnapshot().GetTerminalPresentation().GetLiveTerminal().GetControllerPresentation()
	require.Equal(t, "status", cloned.GetMenu().GetTargetId())
}

type publicIngressTransport struct {
	base     http.RoundTripper
	target   *url.URL
	host     string
	username string
	password string
}

func TestFirstPublicSnapshotRestoresPendingRejectedAndCompletedCommandState(t *testing.T) {
	t.Parallel()

	const commandID = "command-open-doors"
	for _, test := range []struct {
		name          string
		phase         domain.CommandExecutionPhase
		completed     bool
		wantPhase     playerv1.CommandExecutionPhase
		wantName      string
		wantResult    string
		wantCommandID bool
	}{
		{name: "ordinary pending", phase: domain.CommandExecutionPhasePending, wantPhase: playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_PENDING, wantName: "Open doors", wantResult: "Doors opened"},
		{name: "initial state-changing pending", phase: domain.CommandExecutionPhasePending, wantPhase: playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_PENDING, wantName: "Open doors", wantResult: "Doors opened"},
		{name: "completed state-changing pending", phase: domain.CommandExecutionPhasePending, completed: true, wantPhase: playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_PENDING, wantName: "Doors open", wantResult: "Doors opened"},
		{name: "rejected", phase: domain.CommandExecutionPhaseRejected, wantPhase: playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_REJECTED, wantName: "Open doors", wantResult: "Doors opened"},
		{name: "completed state-changing rejected", phase: domain.CommandExecutionPhaseRejected, completed: true, wantPhase: playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_REJECTED, wantName: "Doors open", wantResult: "Doors opened"},
		{name: "completed", completed: true, wantName: "Doors open", wantResult: "Doors opened", wantCommandID: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			live := commandLifecycleLiveState(test.phase, test.completed)
			domainSnapshot := &domain.PersonalizedSnapshot{
				RecognitionHandle: "recognition-reconnect",
				Revision:          40,
				PlayerState: &domain.PlayerState{
					SessionID: "session-controller", FallbackName: "PLAYER 1",
					Role: domain.PlayerRoleActive, Phase: domain.PlayerPhaseControlling,
					BroadcastID: "broadcast-1", ActiveTerminalID: "terminal-1",
				},
				Terminal: domain.TerminalPresentation{Live: live},
			}
			generated, err := SnapshotToProto(domainSnapshot)
			require.NoError(t, err)
			stream := NewSubscription(t.Context(), "physical-reconnect", "session-controller", &playerv1.SubscriptionMessage{
				Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: generated},
			}, 2)
			t.Cleanup(stream.Close)

			first := stream.Snapshot().GetSnapshot()
			require.NotNil(t, first)
			require.Equal(t, uint64(40), first.GetRevision())
			require.Equal(t, "recognition-reconnect", first.GetRecognitionHandle())
			require.Equal(t, playerv1.PlayerRole_PLAYER_ROLE_ACTIVE, first.GetPlayerState().GetRole())
			terminal := first.GetTerminalPresentation().GetLiveTerminal()
			require.NotNil(t, terminal)
			command := terminal.GetTree().GetFolder().GetChildren()[0]
			require.Equal(t, commandID, command.GetId())
			require.Equal(t, test.wantName, command.GetName())
			require.Equal(t, test.wantResult, command.GetCommand().GetText())
			if test.phase == "" && test.completed {
				require.Nil(t, terminal.GetCommandExecution())
				if test.wantCommandID {
					require.Equal(t, commandID, terminal.GetNavigation().GetCommandNodeId())
				}
			} else {
				require.Equal(t, test.wantPhase, terminal.GetCommandExecution().GetPhase())
				require.Equal(t, commandID, terminal.GetCommandExecution().GetCommandNodeId())
				require.Empty(t, terminal.GetNavigation().GetCommandNodeId())
				require.Nil(t, terminal.GetCommandExecution().ProtoReflect().Descriptor().Fields().ByName("request_id"))
			}
		})
	}
}

func TestFirstPublicSnapshotRestoresTerminalRouteAndPendingWithoutPrivateDecisionIdentity(t *testing.T) {
	t.Parallel()
	live := commandLifecycleLiveState("", true)
	live.TerminalNavigation = &domain.TerminalNavigationPresentation{
		RouteDepth:   2,
		ReturnTarget: &domain.TerminalReturnTarget{TerminalID: "terminal-a", TerminalName: "Terminal A"},
		Pending: &domain.PendingTerminalNavigationPresentation{
			Direction: domain.TerminalNavigationReturn, TargetTerminalID: "terminal-a", TargetTerminalName: "Terminal A",
		},
	}
	domainSnapshot := &domain.PersonalizedSnapshot{
		RecognitionHandle: "recognition-route", Revision: 70,
		PlayerState: &domain.PlayerState{
			SessionID: "session-observer", FallbackName: "PLAYER 2", Role: domain.PlayerRoleObserver,
			Phase: domain.PlayerPhaseObserving, BroadcastID: "broadcast-1", ActiveTerminalID: "terminal-b",
		},
		Terminal: domain.TerminalPresentation{Live: live},
	}
	generated, err := SnapshotToProto(domainSnapshot)
	require.NoError(t, err)
	stream := NewSubscription(t.Context(), "physical-route-reconnect", "session-observer", &playerv1.SubscriptionMessage{
		Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: generated},
	}, 2)
	t.Cleanup(stream.Close)

	snapshot := stream.Snapshot().GetSnapshot()
	require.Equal(t, uint64(70), snapshot.GetRevision())
	navigation := snapshot.GetTerminalPresentation().GetLiveTerminal().GetTerminalNavigation()
	require.NotNil(t, navigation)
	require.Equal(t, uint32(2), navigation.GetRouteDepth())
	require.Equal(t, "terminal-a", navigation.GetReturnTarget().GetTerminalId())
	require.Equal(t, playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_RETURN, navigation.GetPending().GetDirection())
	require.Nil(t, navigation.ProtoReflect().Descriptor().Fields().ByName("request_id"))
}

func commandLifecycleLiveState(phase domain.CommandExecutionPhase, completed bool) *domain.PublicLiveState {
	const commandID = "command-open-doors"
	name := "Open doors"
	if completed {
		name = "Doors open"
	}
	live := &domain.PublicLiveState{
		TerminalID: "terminal-1", TerminalName: "Overseer",
		Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{{
			ID: commandID, Type: domain.NodeCommand, Name: name, Text: "Doors opened",
		}}},
		Nav: domain.NavState{Path: []string{"root"}, Mode: "list"},
	}
	if phase != "" {
		live.CommandExecution = &domain.CommandExecutionPresentation{Phase: phase, CommandID: commandID}
	} else if completed {
		commandNodeID := commandID
		live.Nav.CommandNodeID = &commandNodeID
	}
	return live
}

func TestFacilityEffectiveStreamsConvergeForControllerAndObserverAtMonotonicRevisions(t *testing.T) {
	t.Parallel()

	controllerSnapshot, err := SnapshotToProto(facilityEffectiveSnapshot(domain.PlayerRoleActive, 80, false))
	require.NoError(t, err)
	observerSnapshot, err := SnapshotToProto(facilityEffectiveSnapshot(domain.PlayerRoleObserver, 80, false))
	require.NoError(t, err)
	controller := NewSubscription(t.Context(), "facility-controller", "session-controller", &playerv1.SubscriptionMessage{
		Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: controllerSnapshot},
	}, 2)
	t.Cleanup(controller.Close)
	observer := NewSubscription(t.Context(), "facility-observer", "session-observer", &playerv1.SubscriptionMessage{
		Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: observerSnapshot},
	}, 2)
	t.Cleanup(observer.Close)
	legacyDomain := facilityEffectiveSnapshot(domain.PlayerRoleActive, 80, false)
	legacyDomain.Terminal.Live.Tree.Children[0].Available = nil
	legacySnapshot, err := SnapshotToProto(legacyDomain)
	require.NoError(t, err)
	legacy := NewSubscription(t.Context(), "facility-legacy", "session-legacy", &playerv1.SubscriptionMessage{
		Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: legacySnapshot},
	}, 1)
	t.Cleanup(legacy.Close)

	assertFacilityEffectiveTerminal(t, controller.Snapshot().GetSnapshot().GetTerminalPresentation().GetLiveTerminal(), false)
	assertFacilityEffectiveTerminal(t, observer.Snapshot().GetSnapshot().GetTerminalPresentation().GetLiveTerminal(), false)
	legacyCommand := legacy.Snapshot().GetSnapshot().GetTerminalPresentation().GetLiveTerminal().GetTree().GetFolder().GetChildren()[0].GetCommand()
	require.Nil(t, legacyCommand.Available, "absence must keep legacy commands available")

	openPresentation := facilityEffectiveSnapshot(domain.PlayerRoleActive, 81, true).Terminal
	openUpdate, err := CompoundUpdateToProto(&domain.CompoundUpdate{Revision: 81, Terminal: &openPresentation})
	require.NoError(t, err)
	message := &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Update{Update: openUpdate}}
	require.True(t, controller.Offer(message))
	require.True(t, observer.Offer(message))

	controllerUpdate := (<-controller.Updates()).GetUpdate()
	observerUpdate := (<-observer.Updates()).GetUpdate()
	require.Equal(t, uint64(81), controllerUpdate.GetRevision())
	require.Equal(t, controllerUpdate.GetRevision(), observerUpdate.GetRevision())
	controllerTerminal := controllerUpdate.GetTerminalPresentation().GetLiveTerminal()
	observerTerminal := observerUpdate.GetTerminalPresentation().GetLiveTerminal()
	assertFacilityEffectiveTerminal(t, controllerTerminal, true)
	assertFacilityEffectiveTerminal(t, observerTerminal, true)
	require.True(t, proto.Equal(controllerTerminal, observerTerminal), "controller and observer must receive the same effective facility presentation")

	require.True(t, controller.Offer(message), "same revision must remain an idempotent no-op")
	select {
	case unexpected := <-controller.Updates():
		assert.Failf(t, "duplicate facility revision delivered", "update = %#v", unexpected)
	default:
	}
	require.False(t, controller.Offer(facilityStreamUpdate(t, 80, false)), "a facility projection must never regress its stream revision")
	require.Eventually(t, func() bool {
		return errors.Is(context.Cause(controller.context), errSubscriptionRevisionRegressed)
	}, time.Second, time.Millisecond)
}

func TestFacilityReconnectStartsFromOneCompleteCurrentSnapshot(t *testing.T) {
	t.Parallel()

	initial, err := SnapshotToProto(facilityEffectiveSnapshot(domain.PlayerRoleActive, 120, false))
	require.NoError(t, err)
	before := NewSubscription(t.Context(), "facility-before", "session-controller", &playerv1.SubscriptionMessage{
		Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: initial},
	}, 1)
	t.Cleanup(before.Close)
	require.True(t, before.Offer(facilityStreamUpdate(t, 121, true)))
	changed := (<-before.Updates()).GetUpdate()
	assertFacilityEffectiveTerminal(t, changed.GetTerminalPresentation().GetLiveTerminal(), true)
	before.Close()

	current, err := SnapshotToProto(facilityEffectiveSnapshot(domain.PlayerRoleActive, 121, true))
	require.NoError(t, err)
	reconnected := NewSubscription(t.Context(), "facility-reconnected", "session-controller", &playerv1.SubscriptionMessage{
		Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: current},
	}, 1)
	t.Cleanup(reconnected.Close)

	restored := reconnected.Snapshot().GetSnapshot()
	require.Equal(t, uint64(121), restored.GetRevision())
	require.Equal(t, playerv1.PlayerRole_PLAYER_ROLE_ACTIVE, restored.GetPlayerState().GetRole())
	assertFacilityEffectiveTerminal(t, restored.GetTerminalPresentation().GetLiveTerminal(), true)
	select {
	case unexpected := <-reconnected.Updates():
		assert.Failf(t, "reconnect replayed a partial facility update", "update = %#v", unexpected)
	default:
	}
}

func facilityEffectiveSnapshot(role domain.PlayerRole, revision uint64, open bool) *domain.PersonalizedSnapshot {
	available := open
	phase := domain.PlayerPhaseObserving
	if role == domain.PlayerRoleActive {
		phase = domain.PlayerPhaseControlling
	}
	commandName := "OPEN SECURITY DOOR"
	commandText := "Door controls unavailable"
	entryText := "SECURITY DOOR: SEALED"
	effects := []domain.TerminalPresentationEffect{
		domain.TerminalPresentationEffectDisplayUnstable,
		domain.TerminalPresentationEffect("unbounded-content-destruction"),
	}
	if open {
		commandName = "SEAL SECURITY DOOR"
		commandText = "Door controls ready"
		entryText = "SECURITY DOOR: OPEN"
		effects = nil
	}
	return &domain.PersonalizedSnapshot{
		RecognitionHandle: domain.RecognitionHandle("facility-recognition-" + string(role)),
		Revision:          revision,
		PlayerState: &domain.PlayerState{
			SessionID: "session-" + domain.LogicalSessionID(role), FallbackName: "PLAYER",
			Role: role, Phase: phase, BroadcastID: "broadcast-facility", ActiveTerminalID: "terminal-security",
		},
		Terminal: domain.TerminalPresentation{Live: &domain.PublicLiveState{
			TerminalID: "terminal-security", TerminalName: "Security",
			Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
				{ID: "door-command", Type: domain.NodeCommand, Name: commandName, Text: commandText, Available: new(available)},
				{ID: "door-status", Type: domain.NodeEntry, Name: "DOOR STATUS", Description: entryText},
			}},
			Nav: domain.NavState{Path: []string{"root"}, Mode: "list"},
			Presentation: domain.ControllerTerminalPresentation{
				Kind: domain.ControllerTerminalPresentationMenu, ContextKey: "menu:root", TargetID: "door-command",
			},
			Effects: effects,
		}},
	}
}

func facilityStreamUpdate(t *testing.T, revision uint64, open bool) *playerv1.SubscriptionMessage {
	t.Helper()
	presentation := facilityEffectiveSnapshot(domain.PlayerRoleActive, revision, open).Terminal
	update, err := CompoundUpdateToProto(&domain.CompoundUpdate{Revision: revision, Terminal: &presentation})
	require.NoError(t, err)
	return &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Update{Update: update}}
}

func assertFacilityEffectiveTerminal(t *testing.T, terminal *playerv1.LiveTerminal, open bool) {
	t.Helper()
	require.NotNil(t, terminal)
	require.Equal(t, "terminal-security", terminal.GetTerminalId())
	require.NotNil(t, terminal.GetNavigation())
	require.Equal(t, []string{"root"}, terminal.GetNavigation().GetPath())
	require.NotNil(t, terminal.GetControllerPresentation())
	require.Equal(t, "door-command", terminal.GetControllerPresentation().GetMenu().GetTargetId())
	children := terminal.GetTree().GetFolder().GetChildren()
	require.Len(t, children, 2, "every facility publication must carry the complete effective tree")
	command := children[0]
	require.Equal(t, "door-command", command.GetId())
	require.NotNil(t, command.GetCommand().Available, "effective availability must retain optional presence")
	require.Equal(t, open, command.GetCommand().GetAvailable())
	entry := children[1]
	require.Equal(t, "door-status", entry.GetId())
	if open {
		require.Equal(t, "SEAL SECURITY DOOR", command.GetName())
		require.Equal(t, "Door controls ready", command.GetCommand().GetText())
		require.Equal(t, "SECURITY DOOR: OPEN", entry.GetEntry().GetDescription())
		require.Empty(t, terminal.GetEffects())
		return
	}
	require.Equal(t, "OPEN SECURITY DOOR", command.GetName())
	require.Equal(t, "Door controls unavailable", command.GetCommand().GetText())
	require.Equal(t, "SECURITY DOOR: SEALED", entry.GetEntry().GetDescription())
	require.Equal(t, []playerv1.TerminalPresentationEffect{
		playerv1.TerminalPresentationEffect_TERMINAL_PRESENTATION_EFFECT_DISPLAY_UNSTABLE,
	}, terminal.GetEffects(), "only bounded player-safe effects may cross the stream")
}

func (transport publicIngressTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	forwarded := request.Clone(request.Context())
	targetURL := *request.URL
	targetURL.Scheme = transport.target.Scheme
	targetURL.Host = transport.target.Host
	forwarded.URL = &targetURL
	forwarded.Host = transport.host
	forwarded.SetBasicAuth(transport.username, transport.password)
	return transport.base.RoundTrip(forwarded)
}

func endpointAuthSeam(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != edgeTestUsername || password != edgeTestPassword {
			response.Header().Set("WWW-Authenticate", `Basic realm="Fallout Terminal Players"`)
			http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		forwarded := request.Clone(request.Context())
		forwarded.Header.Del("Authorization")
		next.ServeHTTP(response, forwarded)
	})
}

func TestPublicIngressProtectsStaticUnaryAndStreamingBeforeUnchangedPlayerBoundary(t *testing.T) {
	var service *ConnectService
	coordinator := newConnectTestCoordinator(t, func(effect control.Effect) {
		if service != nil {
			service.PublishEffect(effect)
		}
	})
	var err error
	service, err = NewConnectService(ConnectServiceConfig{Coordinator: coordinator, Assets: playerAssets()})
	require.NoError(t, err)
	rpcPath, rpcHandler := NewConnectHandler(service)
	application := NewApplicationHandler(playerAssets(), rpcPath, rpcHandler)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	var upstreamProtocol atomic.Int64
	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			upstreamProtocol.Store(int64(request.ProtoMajor))
			application.ServeHTTP(response, request)
		}),
		Protocols: protocols,
	}
	go func() { _ = httpServer.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		require.NoError(t, httpServer.Shutdown(ctx))
	})
	playerURL := "http://" + listener.Addr().String()
	ingress, err := tunnel.NewPublicIngressFactory().Start(t.Context(), playerURL)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ingress.Close(context.WithoutCancel(t.Context()))) })
	require.NoError(t, ingress.Activate("public.example", edgeTestUsername, []byte(edgeTestPassword)))

	for _, path := range []string{
		"/", "/client.js",
		"/fallout.terminal.player.v1.PlayerService/SoundManifest",
		"/fallout.terminal.player.v1.PlayerService/Subscribe",
	} {
		for _, credentials := range []struct{ username, password string }{{}, {username: edgeTestUsername, password: "synthetic-wrong-input"}} {
			request, requestErr := http.NewRequestWithContext(t.Context(), http.MethodPost, ingress.URL().String()+path, bytes.NewReader([]byte{0, 0, 0, 0, 0}))
			require.NoError(t, requestErr)
			request.Host = "public.example"
			request.Header.Set("Content-Type", "application/connect+proto")
			if credentials.username != "" {
				request.SetBasicAuth(credentials.username, credentials.password)
			}
			response, requestErr := http.DefaultClient.Do(request)
			require.NoError(t, requestErr)
			_ = response.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, response.StatusCode, path)
			assert.NotEmpty(t, response.Header.Get("WWW-Authenticate"), path)
		}
	}

	authenticatedClient := &http.Client{Transport: publicIngressTransport{
		base: http.DefaultTransport, target: ingress.URL(), host: "public.example",
		username: edgeTestUsername, password: edgeTestPassword,
	}}
	staticRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://public.example/", nil)
	require.NoError(t, err)
	staticResponse, err := authenticatedClient.Do(staticRequest)
	require.NoError(t, err)
	_ = staticResponse.Body.Close()
	assert.Equal(t, http.StatusOK, staticResponse.StatusCode)
	assert.Empty(t, staticResponse.Header.Get("WWW-Authenticate"))

	client := playerv1connect.NewPlayerServiceClient(authenticatedClient, "http://public.example")
	manifest, err := client.SoundManifest(t.Context(), connect.NewRequest(&playerv1.SoundManifestRequest{
		Category: playerv1.SoundCategory_SOUND_CATEGORY_AMBIENT,
	}))
	require.NoError(t, err)
	require.NotNil(t, manifest.Msg)

	streamContext, cancelStream := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancelStream(errors.New("test public stream completed")) })
	clientInstanceID := "public-tab-1"
	stream, err := client.Subscribe(streamContext, connect.NewRequest(&playerv1.SubscribeRequest{ClientInstanceId: &clientInstanceID}))
	require.NoError(t, err)
	require.True(t, stream.Receive(), "stream error: %v", stream.Err())
	snapshot := stream.Msg().GetSnapshot()
	require.NotNil(t, snapshot)
	clonedSnapshot := proto.Clone(snapshot).(*playerv1.PersonalizedSnapshot)
	require.Empty(t, cmp.Diff(snapshot, clonedSnapshot, protocmp.Transform()))

	uplink := client.PresentationUplink(t.Context())
	require.NoError(t, uplink.Send(&playerv1.PresentationUplinkRequest{Payload: &playerv1.PresentationUplinkRequest_Open{
		Open: &playerv1.PresentationUplinkOpen{
			ClientInstanceId: clientInstanceID, UplinkGeneration: 1,
			RecognitionHandle: snapshot.GetRecognitionHandle(),
		},
	}}))
	require.True(t, stream.Receive(), "uplink ready stream error: %v", stream.Err())
	ready := stream.Msg().GetPresentationUplinkResult()
	require.NotNil(t, ready)
	require.Equal(t, clientInstanceID, ready.GetClientInstanceId())
	require.NotNil(t, ready.GetReady())
	_, err = uplink.CloseAndReceive()
	require.NoError(t, err)

	selected, err := client.SelectCharacter(t.Context(), connect.NewRequest(&playerv1.SelectCharacterRequest{
		RecognitionHandle: snapshot.GetRecognitionHandle(), RequestId: "edge-request-1",
		BroadcastId: "broadcast-1", CharacterId: "character-1",
	}))
	require.NoError(t, err)
	require.True(t, selected.Msg.GetAccepted())
	require.True(t, stream.Receive(), "stream error: %v", stream.Err())
	update := stream.Msg().GetUpdate()
	require.NotNil(t, update)
	assert.Equal(t, selected.Msg.GetRevision(), update.GetRevision())

	onAirRevision := update.GetRevision() + 1
	service.PublishEffect(control.Effect{
		SessionID: domain.LogicalSessionID(snapshot.GetPlayerState().GetLogicalSessionId()),
		Update: &domain.CompoundUpdate{
			Revision: onAirRevision,
			Terminal: &domain.TerminalPresentation{Live: commandLifecycleLiveState("", false)},
		},
	})
	require.True(t, stream.Receive(), "post-selection on-air stream error: %v", stream.Err())
	onAir := stream.Msg().GetUpdate()
	require.NotNil(t, onAir)
	assert.Equal(t, onAirRevision, onAir.GetRevision())
	assert.Equal(t, "terminal-1", onAir.GetTerminalPresentation().GetLiveTerminal().GetTerminalId())
	assert.Equal(t, int64(2), upstreamProtocol.Load(), "ingress-to-player requests must use h2c")
}

func TestAuthenticatedForwardingStillAppliesOriginAndBodyLimitsInsidePlayer(t *testing.T) {
	var calls int
	application := NewApplicationHandler(playerAssets(), "/fallout.terminal.player.v1.PlayerService/", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		calls++
		response.WriteHeader(http.StatusNoContent)
	}))
	handler := endpointAuthSeam(application)

	for _, test := range []struct {
		name   string
		origin string
		body   []byte
		want   int
	}{
		{name: "same origin reaches RPC", origin: "https://public.example", want: http.StatusNoContent},
		{name: "foreign origin rejected after edge", origin: "https://other.example", want: http.StatusForbidden},
		{name: "encoded limit rejected after edge", origin: "https://public.example", body: bytes.Repeat([]byte{'x'}, MaxEncodedBodyBytes+1), want: http.StatusTooManyRequests},
	} {
		request := httptest.NewRequest(http.MethodPost, "https://public.example/fallout.terminal.player.v1.PlayerService/Navigate", bytes.NewReader(test.body))
		request.Host = "public.example"
		request.Header.Set("Origin", test.origin)
		request.Header.Set("Content-Type", "application/proto")
		request.SetBasicAuth(edgeTestUsername, edgeTestPassword)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		assert.Equal(t, test.want, recorder.Code, test.name)
	}
	assert.Equal(t, 1, calls)
}
