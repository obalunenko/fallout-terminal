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
