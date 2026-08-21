package player

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/internal/control"
	"github.com/obalunenko/Fallout-Terminal/internal/domain"
	playerv1 "github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/player/v1"
	"github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/player/v1/playerv1connect"
	"github.com/obalunenko/Fallout-Terminal/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestConnectSubscribeBeginsWithCompleteSnapshotAndSelectsCharacter(t *testing.T) {
	var service *ConnectService
	coordinator := newConnectTestCoordinator(t, func(effect control.Effect) {
		if service != nil {
			service.PublishEffect(effect)
		}
	})
	var err error
	service, err = NewConnectService(ConnectServiceConfig{Coordinator: coordinator})
	require.NoError(t, err)

	path, handler := playerv1connect.NewPlayerServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := playerv1connect.NewPlayerServiceClient(server.Client(), server.URL)
	streamContext, cancelStream := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancelStream(errors.New("test subscription closed")) })
	stream, err := client.Subscribe(streamContext, connect.NewRequest(&playerv1.SubscribeRequest{}))
	require.NoError(t, err)
	require.True(t, stream.Receive())
	first := stream.Msg()
	snapshot := first.GetSnapshot()
	require.NotNil(t, snapshot)
	require.NotEmpty(t, snapshot.GetRecognitionHandle())
	require.NotNil(t, snapshot.GetPlayerState())
	require.NotNil(t, snapshot.GetTerminalPresentation().GetNoLiveTerminal())
	require.Nil(t, first.GetUpdate())

	wantPlayer := &playerv1.PlayerState{
		LogicalSessionId: snapshot.GetPlayerState().GetLogicalSessionId(),
		FallbackName:     snapshot.GetPlayerState().GetFallbackName(),
		Role:             playerv1.PlayerRole_PLAYER_ROLE_UNASSIGNED,
		Phase:            playerv1.PlayerPhase_PLAYER_PHASE_SELECTING,
		BroadcastId:      new("broadcast-1"),
		Roster: []*playerv1.RosterEntry{{
			CharacterId:  "character-1",
			DisplayName:  "Lucy",
			Availability: playerv1.RosterAvailability_ROSTER_AVAILABILITY_AVAILABLE,
		}},
	}
	require.Empty(t, cmp.Diff(wantPlayer, snapshot.GetPlayerState(), protocmp.Transform()))

	response, err := client.SelectCharacter(t.Context(), connect.NewRequest(&playerv1.SelectCharacterRequest{
		RecognitionHandle: snapshot.GetRecognitionHandle(),
		RequestId:         "request-1",
		BroadcastId:       "broadcast-1",
		CharacterId:       "character-1",
	}))
	require.NoError(t, err)
	require.True(t, response.Msg.GetAccepted())
	require.Equal(t, playerv1.ActionReason_ACTION_REASON_ACCEPTED, response.Msg.GetReason())
	require.Greater(t, response.Msg.GetRevision(), snapshot.GetRevision())
	require.True(t, stream.Receive())
	update := stream.Msg().GetUpdate()
	require.NotNil(t, update)
	require.Equal(t, response.Msg.GetRevision(), update.GetRevision())
	require.NotNil(t, update.GetPlayerState())
}

func TestConnectSubscribeHandleMatrixRejectsInvalidWithoutCanonicalEffects(t *testing.T) {
	tests := []struct {
		name   string
		handle string
	}{
		{name: "blank", handle: ""},
		{name: "whitespace", handle: "not valid"},
		{name: "oversized", handle: strings.Repeat("a", domain.MaxRecognitionHandleBytes+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			coordinator := newConnectTestCoordinator(t)
			beforeRevision := coordinator.Revision()
			service, err := NewConnectService(ConnectServiceConfig{Coordinator: coordinator})
			require.NoError(t, err)
			path, handler := playerv1connect.NewPlayerServiceHandler(service)
			mux := http.NewServeMux()
			mux.Handle(path, handler)
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			client := playerv1connect.NewPlayerServiceClient(server.Client(), server.URL)

			stream, err := client.Subscribe(t.Context(), connect.NewRequest(&playerv1.SubscribeRequest{RecognitionHandle: &test.handle}))
			if err == nil {
				require.False(t, stream.Receive())
				err = stream.Err()
			}
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			require.Equal(t, beforeRevision, coordinator.Revision())
			require.Zero(t, coordinator.ActiveStreamCount())
		})
	}
}

func TestTypedSharedActionHandlersRejectUnassignedSessionWithoutMutation(t *testing.T) {
	coordinator := newConnectTestCoordinator(t)
	snapshot, err := coordinator.AttachSubscription("connect-test-stream", nil)
	require.NoError(t, err)
	service, err := NewConnectService(ConnectServiceConfig{Coordinator: coordinator})
	require.NoError(t, err)
	beforeRevision := coordinator.Revision()

	tests := []struct {
		name string
		call func() (*connect.Response[playerv1.ActionResult], error)
	}{
		{
			name: "navigate",
			call: func() (*connect.Response[playerv1.ActionResult], error) {
				return service.Navigate(t.Context(), connect.NewRequest(&playerv1.NavigateRequest{
					RecognitionHandle: string(snapshot.RecognitionHandle), RequestId: "navigate-1",
					BroadcastId: "broadcast-1", TerminalId: "terminal-1",
					Action: &playerv1.NavigateRequest_Back{Back: &playerv1.NavigateBack{}},
				}))
			},
		},
		{
			name: "guess",
			call: func() (*connect.Response[playerv1.ActionResult], error) {
				return service.Guess(t.Context(), connect.NewRequest(&playerv1.GuessRequest{
					RecognitionHandle: string(snapshot.RecognitionHandle), RequestId: "guess-1",
					BroadcastId: "broadcast-1", TerminalId: "terminal-1",
					Target: &playerv1.GuessRequest_WordId{WordId: "word-1"},
				}))
			},
		},
		{
			name: "activate pattern",
			call: func() (*connect.Response[playerv1.ActionResult], error) {
				return service.ActivatePattern(t.Context(), connect.NewRequest(&playerv1.ActivatePatternRequest{
					RecognitionHandle: string(snapshot.RecognitionHandle), RequestId: "pattern-1",
					BroadcastId: "broadcast-1", TerminalId: "terminal-1", PatternId: "opaque-pattern-1",
				}))
			},
		},
		{
			name: "set presentation",
			call: func() (*connect.Response[playerv1.ActionResult], error) {
				return service.SetPresentation(t.Context(), connect.NewRequest(&playerv1.SetPresentationRequest{
					RecognitionHandle: string(snapshot.RecognitionHandle), RequestId: "presentation-1",
					BroadcastId: "broadcast-1", TerminalId: "terminal-1", ContextKey: "menu:root",
					Presentation: &playerv1.ControllerTerminalPresentation{
						ContextKey:   "menu:root",
						Presentation: &playerv1.ControllerTerminalPresentation_Menu{Menu: &playerv1.MenuSelection{TargetId: "docs"}},
					},
				}))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := test.call()
			require.NoError(t, err)
			require.False(t, response.Msg.GetAccepted())
			require.Equal(t, playerv1.ActionReason_ACTION_REASON_UNASSIGNED, response.Msg.GetReason())
			require.Equal(t, beforeRevision, response.Msg.GetRevision())
		})
	}
	require.Equal(t, beforeRevision, coordinator.Revision())
}

func TestTypedHandlersClassifyStructuralDomainAndBoundaryErrorsSafely(t *testing.T) {
	coordinator := newConnectTestCoordinator(t)
	service, err := NewConnectService(ConnectServiceConfig{Coordinator: coordinator})
	require.NoError(t, err)
	beforeRevision := coordinator.Revision()
	secret := "private-recognition-handle"

	structural := []struct {
		name string
		call func() error
	}{
		{
			name: "blank recognition",
			call: func() error {
				_, err := service.Navigate(t.Context(), connect.NewRequest(&playerv1.NavigateRequest{
					RequestId: "request-1", BroadcastId: "broadcast-1", TerminalId: "terminal-1",
					Action: &playerv1.NavigateRequest_Back{Back: &playerv1.NavigateBack{}},
				}))
				return err
			},
		},
		{
			name: "oversized request identity",
			call: func() error {
				_, err := service.SelectCharacter(t.Context(), connect.NewRequest(&playerv1.SelectCharacterRequest{
					RecognitionHandle: secret, RequestId: strings.Repeat("r", domain.MaxRequestIDBytes+1),
					BroadcastId: "broadcast-1", CharacterId: "character-1",
				}))
				return err
			},
		},
		{
			name: "missing navigation variant",
			call: func() error {
				_, err := service.Navigate(t.Context(), connect.NewRequest(&playerv1.NavigateRequest{
					RecognitionHandle: secret, RequestId: "request-2", BroadcastId: "broadcast-1", TerminalId: "terminal-1",
				}))
				return err
			},
		},
		{
			name: "illegal filler coordinates",
			call: func() error {
				_, err := service.Guess(t.Context(), connect.NewRequest(&playerv1.GuessRequest{
					RecognitionHandle: secret, RequestId: "request-3", BroadcastId: "broadcast-1", TerminalId: "terminal-1",
					Target: &playerv1.GuessRequest_Filler{Filler: &playerv1.FillerTarget{Column: 2}},
				}))
				return err
			},
		},
	}
	for _, test := range structural {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			require.NotContains(t, err.Error(), secret)
			require.Equal(t, beforeRevision, coordinator.Revision())
		})
	}

	unknown, err := service.ActivatePattern(t.Context(), connect.NewRequest(&playerv1.ActivatePatternRequest{
		RecognitionHandle: "unknown-but-well-formed", RequestId: "request-4", BroadcastId: "broadcast-1",
		TerminalId: "terminal-1", PatternId: "pattern-1",
	}))
	require.NoError(t, err)
	require.False(t, unknown.Msg.GetAccepted())
	require.Equal(t, playerv1.ActionReason_ACTION_REASON_INVALID_SESSION, unknown.Msg.GetReason())
	require.Equal(t, beforeRevision, coordinator.Revision())

	require.Equal(t, playerv1.ActionReason_ACTION_REASON_UNSPECIFIED, ActionResultToProto(domain.ActionResult{Reason: "private-reason"}).GetReason())
	require.Equal(t, connect.CodeCanceled, connect.CodeOf(mapStreamError(context.Canceled)))
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(mapStreamError(errors.New("private dependency detail"))))
	require.NotContains(t, mapStreamError(errors.New("private dependency detail")).Error(), "private dependency detail")
	require.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(publicConnectError(ErrResourceExhausted)))
}

func TestConnectPublishesPersistenceFailureNoticeOnlyToControllerAndClearsIt(t *testing.T) {
	service, err := NewConnectService(ConnectServiceConfig{Coordinator: newConnectTestCoordinator(t)})
	require.NoError(t, err)

	controller := NewSubscription(t.Context(), "controller-stream", "controller-session", subscriptionSnapshot(1), 4)
	observer := NewSubscription(t.Context(), "observer-stream", "observer-session", subscriptionSnapshot(1), 4)
	service.hub.Register(controller)
	service.hub.Register(observer)
	t.Cleanup(service.CloseSubscriptions)

	controllerState := &domain.PlayerState{
		SessionID: "controller-session", FallbackName: "Игрок 1",
		Role: domain.PlayerRoleActive, Phase: domain.PlayerPhaseControlling,
	}
	setDomainPlayerNotice(t, controllerState, "command-persistence-failed")
	service.PublishEffect(control.Effect{
		SessionID: "controller-session",
		Update:    &domain.CompoundUpdate{Revision: 2, Player: controllerState},
	})

	controllerUpdate := (<-controller.Updates()).GetUpdate()
	require.NotNil(t, controllerUpdate)
	require.Equal(t,
		playerv1.PlayerNoticeKind_PLAYER_NOTICE_KIND_COMMAND_PERSISTENCE_FAILED,
		controllerUpdate.GetPlayerState().GetNotice().GetKind(),
	)
	select {
	case unexpected := <-observer.Updates():
		assert.Failf(t, "controller notice leaked", "observer received %#v", unexpected)
	default:
	}

	observerState := &domain.PlayerState{
		SessionID: "observer-session", FallbackName: "Игрок 2",
		Role: domain.PlayerRoleObserver, Phase: domain.PlayerPhaseObserving,
	}
	service.PublishEffect(control.Effect{
		SessionID: "observer-session",
		Update:    &domain.CompoundUpdate{Revision: 2, Player: observerState},
	})
	observerUpdate := (<-observer.Updates()).GetUpdate()
	require.NotNil(t, observerUpdate)
	require.Nil(t, observerUpdate.GetPlayerState().GetNotice())

	clearDomainOptionalField(t, controllerState, "Notice")
	service.PublishEffect(control.Effect{
		SessionID: "controller-session",
		Update:    &domain.CompoundUpdate{Revision: 3, Player: controllerState},
	})
	cleared := (<-controller.Updates()).GetUpdate()
	require.NotNil(t, cleared)
	require.Nil(t, cleared.GetPlayerState().GetNotice())
}

func TestNavigateHandlerKeepsOrdinaryCommandOnExistingTypedPath(t *testing.T) {
	base := newConnectTestCoordinator(t)
	coordinator := &recordingActionCoordinator{ConnectCoordinator: base}
	service, err := NewConnectService(ConnectServiceConfig{Coordinator: coordinator})
	require.NoError(t, err)

	request := &playerv1.NavigateRequest{
		RecognitionHandle: "recognition-ordinary", RequestId: "ordinary-request",
		BroadcastId: "broadcast-ordinary", TerminalId: "terminal-ordinary",
		Action: &playerv1.NavigateRequest_Command{Command: &playerv1.NavigateCommand{NodeId: "diagnostic"}},
	}
	response, err := service.Navigate(t.Context(), connect.NewRequest(request))
	require.NoError(t, err)
	require.True(t, response.Msg.GetAccepted())
	require.Equal(t, playerv1.ActionReason_ACTION_REASON_ACCEPTED, response.Msg.GetReason())
	require.Equal(t, domain.RecognitionHandle("recognition-ordinary"), coordinator.handle)
	require.Equal(t, domain.RuntimeCommandNavigate, coordinator.command.Kind)
	require.Equal(t, "command", coordinator.command.Action)
	require.Equal(t, "diagnostic", coordinator.command.NodeID)
	require.NotEmpty(t, coordinator.command.PayloadFingerprint)

	firstFingerprint := coordinator.command.PayloadFingerprint
	replayed, err := service.Navigate(t.Context(), connect.NewRequest(request))
	require.NoError(t, err)
	require.Equal(t, response.Msg, replayed.Msg)
	require.Equal(t, 2, coordinator.calls)
	require.Equal(t, firstFingerprint, coordinator.command.PayloadFingerprint,
		"the generated ordinary request must retain exact replay identity")
}

func TestPublicDescriptorAndProceduresExcludeEveryPrivateDesktopCapability(t *testing.T) {
	var symbols []string
	var collectMessages func(protoreflect.MessageDescriptors)
	collectMessages = func(messages protoreflect.MessageDescriptors) {
		for index := 0; index < messages.Len(); index++ {
			message := messages.Get(index)
			symbols = append(symbols, string(message.FullName()))
			for fieldIndex := 0; fieldIndex < message.Fields().Len(); fieldIndex++ {
				symbols = append(symbols, string(message.Fields().Get(fieldIndex).FullName()))
			}
			collectMessages(message.Messages())
		}
	}
	for _, file := range []protoreflect.FileDescriptor{
		playerv1.File_fallout_terminal_player_v1_player_proto,
		playerv1.File_fallout_terminal_player_v1_terminal_proto,
		playerv1.File_fallout_terminal_player_v1_navigation_proto,
	} {
		collectMessages(file.Messages())
		for enumIndex := 0; enumIndex < file.Enums().Len(); enumIndex++ {
			enum := file.Enums().Get(enumIndex)
			symbols = append(symbols, string(enum.FullName()))
			for valueIndex := 0; valueIndex < enum.Values().Len(); valueIndex++ {
				symbols = append(symbols, string(enum.Values().Get(valueIndex).FullName()))
			}
		}
		for serviceIndex := 0; serviceIndex < file.Services().Len(); serviceIndex++ {
			service := file.Services().Get(serviceIndex)
			symbols = append(symbols, string(service.FullName()))
			for methodIndex := 0; methodIndex < service.Methods().Len(); methodIndex++ {
				symbols = append(symbols, string(service.Methods().Get(methodIndex).FullName()))
			}
		}
	}
	publicSurface := strings.ToLower(strings.Join(symbols, "\n") + playerv1connect.PlayerServiceName +
		playerv1connect.PlayerServiceSubscribeProcedure + playerv1connect.PlayerServiceSelectCharacterProcedure +
		playerv1connect.PlayerServiceNavigateProcedure + playerv1connect.PlayerServiceGuessProcedure +
		playerv1connect.PlayerServiceActivatePatternProcedure + playerv1connect.PlayerServiceSetPresentationProcedure +
		playerv1connect.PlayerServiceSoundManifestProcedure)
	for _, forbidden := range []string{
		"desktop", "dialog", "openurl", "forcehacksuccess", "resetfailedhack", "runtimestatus",
		"serverinformation", "credential", "secretword", "logicalsessionstate", "coordinationstate",
		"pendingcommandexecution", "resolvecommandexecution", "commandexecutiondecision", "confirmationtext",
		"commandstates", "resetcommandstate", "resetterminalcommandstates", "sessionstateevent",
		"sessionstateresult", "filepath", "approve",
	} {
		require.NotContains(t, publicSurface, forbidden)
	}
}

type recordingActionCoordinator struct {
	ConnectCoordinator
	handle  domain.RecognitionHandle
	command domain.RuntimeCommand
	calls   int
}

func (coordinator *recordingActionCoordinator) DispatchPlayerActionForRecognition(handle domain.RecognitionHandle, command domain.RuntimeCommand) domain.ActionResult {
	coordinator.calls++
	coordinator.handle = handle
	coordinator.command = command
	return domain.ActionResult{
		RequestID: command.RequestID, Accepted: true,
		Reason: domain.ActionReasonAccepted, Revision: 42,
	}
}

func newConnectTestCoordinator(t *testing.T, publish ...func(control.Effect)) *control.Service {
	t.Helper()
	ids := testutil.NewFakeOpaqueIDSource("broadcast-1", "session-1", "recognition-1")
	var enqueue func(control.Effect)
	if len(publish) > 0 {
		enqueue = publish[0]
	}
	coordinator := control.New(control.Config{IDs: ids, Enqueue: enqueue})
	_, err := coordinator.InstallPlayerConfig(domain.PlayerConfigHandle{Path: "/private/player.json", Version: 1, Name: "Vault 33"}, []domain.CharacterRosterEntry{{ID: "character-1", Name: "Lucy", Intelligence: 1}})
	require.NoError(t, err)
	_, err = coordinator.StartBroadcast()
	require.NoError(t, err)
	return coordinator
}
