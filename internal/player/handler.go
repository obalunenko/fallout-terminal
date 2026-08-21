package player

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync/atomic"

	"connectrpc.com/connect"
	"github.com/obalunenko/Fallout-Terminal/internal/control"
	"github.com/obalunenko/Fallout-Terminal/internal/domain"
	playerv1 "github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/player/v1"
	"github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/player/v1/playerv1connect"
)

// ConnectCoordinator is the narrow canonical seam used by the generated
// public service. Recognition never authorizes a mutation by itself.
type ConnectCoordinator interface {
	AttachSubscriptionAndRegister(domain.ConnectionID, *domain.RecognitionHandle, func(*domain.PersonalizedSnapshot)) (*domain.PersonalizedSnapshot, error)
	DetachConnection(domain.ConnectionID)
	SelectCharacterForRecognition(domain.RecognitionHandle, domain.RequestID, domain.BroadcastID, domain.CharacterID) domain.ActionResult
	DispatchPlayerActionForRecognition(domain.RecognitionHandle, domain.RuntimeCommand) domain.ActionResult
}

// ConnectServiceConfig supplies only transport-independent application
// dependencies. Generated requests and streams remain detached boundary values.
type ConnectServiceConfig struct {
	Coordinator   ConnectCoordinator
	QueueSize     int
	Hub           *SubscriptionHub
	Assets        fs.FS
	OnClientCount func(int)
}

// ConnectService implements the generated public handler with one server
// stream and separately typed unary responsibilities.
type ConnectService struct {
	playerv1connect.UnimplementedPlayerServiceHandler

	coordinator   ConnectCoordinator
	queueSize     int
	hub           *SubscriptionHub
	assets        fs.FS
	onClientCount func(int)
	sequence      atomic.Uint64
}

// NewConnectService validates the public service's narrow dependencies.
func NewConnectService(config ConnectServiceConfig) (*ConnectService, error) {
	if config.Coordinator == nil {
		return nil, fmt.Errorf("connect player coordinator is not configured")
	}
	if config.QueueSize <= 0 {
		config.QueueSize = defaultSubscriptionQueueSize
	}
	if config.Hub == nil {
		config.Hub = NewSubscriptionHub()
	}
	return &ConnectService{
		coordinator: config.Coordinator, queueSize: config.QueueSize, hub: config.Hub,
		assets: config.Assets, onClientCount: config.OnClientCount,
	}, nil
}

// NewConnectHandler builds the sole generated public handler with the common
// decoded protobuf limit applied to every procedure, including unknown fields
// and decompressed messages.
func NewConnectHandler(service *ConnectService) (string, http.Handler) {
	return playerv1connect.NewPlayerServiceHandler(service, connect.WithReadMaxBytes(MaxUncompressedMessageBytes))
}

// CloseSubscriptions terminates all active physical streams during ordered
// server shutdown. Stream defers still detach canonical connection presence.
func (service *ConnectService) CloseSubscriptions() {
	if service == nil {
		return
	}
	service.hub.CloseAll()
}

// Subscribe attaches before capturing and sending exactly one complete first
// snapshot, then drains only strictly newer queued compound updates.
func (service *ConnectService) Subscribe(ctx context.Context, request *connect.Request[playerv1.SubscribeRequest], stream *connect.ServerStream[playerv1.SubscriptionMessage]) error {
	if ctx == nil || service == nil || request == nil || stream == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("subscribe request is required"))
	}
	handle, err := SubscribeRecognition(request.Msg)
	if err != nil {
		return publicConnectError(err)
	}
	connectionID := domain.ConnectionID(fmt.Sprintf("connect-%d", service.sequence.Add(1)))
	var physical *Subscription
	var conversionErr error
	snapshot, err := service.coordinator.AttachSubscriptionAndRegister(connectionID, handle, func(attached *domain.PersonalizedSnapshot) {
		generatedSnapshot, adapterErr := SnapshotToProto(attached)
		if adapterErr != nil {
			conversionErr = adapterErr
			return
		}
		first := &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Snapshot{Snapshot: generatedSnapshot}}
		physical = NewSubscription(ctx, connectionID, attached.PlayerState.SessionID, first, service.queueSize)
		service.hub.Register(physical)
	})
	if err != nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("player subscription is temporarily unavailable"))
	}
	if conversionErr != nil || snapshot == nil || physical == nil {
		service.coordinator.DetachConnection(connectionID)
		return connect.NewError(connect.CodeInternal, errors.New("could not build player snapshot"))
	}
	service.emitClientCount()
	defer func() {
		service.hub.Unregister(connectionID)
		service.emitClientCount()
		service.coordinator.DetachConnection(connectionID)
	}()

	if err := stream.Send(physical.Snapshot()); err != nil {
		return mapStreamError(err)
	}
	for {
		select {
		case <-ctx.Done():
			return connect.NewError(connect.CodeCanceled, errors.New("player subscription canceled"))
		case <-physical.Done():
			return nil
		case update := <-physical.Updates():
			if err := stream.Send(update); err != nil {
				return mapStreamError(err)
			}
		}
	}
}

// PublishEffect offers the one preassembled complete generated update carried
// by an ordered coordinator effect. Legacy component envelopes are ignored.
func (service *ConnectService) PublishEffect(effect control.Effect) {
	if service == nil || effect.Update == nil || effect.SessionID == "" {
		return
	}
	update := effect.Update
	if update.Player != nil && update.Player.Notice != nil &&
		(update.Player.SessionID != effect.SessionID || update.Player.Role != domain.PlayerRoleActive) {
		update = domain.CloneCompoundUpdate(update)
		update.Player.Notice = nil
	}
	generated, err := CompoundUpdateToProto(update)
	if err != nil {
		return
	}
	service.hub.Offer(effect.SessionID, &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Update{Update: generated}})
}

func (service *ConnectService) emitClientCount() {
	if service != nil && service.onClientCount != nil {
		service.onClientCount(service.hub.Count())
	}
}

// SelectCharacter resolves the opaque handle without creating state, executes
// the coordinator transaction once, and offers its authoritative update before
// completing the unary response.
func (service *ConnectService) SelectCharacter(_ context.Context, request *connect.Request[playerv1.SelectCharacterRequest]) (*connect.Response[playerv1.ActionResult], error) {
	if service == nil || request == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("select character request is required"))
	}
	mutation, err := SelectionFromProto(request.Msg, "")
	if err != nil {
		return nil, publicConnectError(err)
	}
	result := service.coordinator.SelectCharacterForRecognition(
		mutation.RecognitionHandle,
		mutation.Selection.RequestID,
		mutation.Selection.BroadcastID,
		mutation.Selection.CharacterID,
	)
	return connect.NewResponse(ActionResultToProto(result)), nil
}

// Navigate validates the exact action variant and invokes the canonical action once.
func (service *ConnectService) Navigate(_ context.Context, request *connect.Request[playerv1.NavigateRequest]) (*connect.Response[playerv1.ActionResult], error) {
	if service == nil || request == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("navigate request is required"))
	}
	mutation, err := NavigateFromProto(request.Msg)
	if err != nil {
		return nil, publicConnectError(err)
	}
	return service.dispatchRuntimeMutation(mutation), nil
}

// Guess validates the exact word/filler variant and invokes the canonical action once.
func (service *ConnectService) Guess(_ context.Context, request *connect.Request[playerv1.GuessRequest]) (*connect.Response[playerv1.ActionResult], error) {
	if service == nil || request == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("guess request is required"))
	}
	mutation, err := GuessFromProto(request.Msg)
	if err != nil {
		return nil, publicConnectError(err)
	}
	return service.dispatchRuntimeMutation(mutation), nil
}

// ActivatePattern validates the opaque pattern target and invokes the canonical action once.
func (service *ConnectService) ActivatePattern(_ context.Context, request *connect.Request[playerv1.ActivatePatternRequest]) (*connect.Response[playerv1.ActionResult], error) {
	if service == nil || request == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("activate pattern request is required"))
	}
	mutation, err := ActivatePatternFromProto(request.Msg)
	if err != nil {
		return nil, publicConnectError(err)
	}
	return service.dispatchRuntimeMutation(mutation), nil
}

// SetPresentation validates one semantic controller-owned view mutation and
// invokes the same ordered coordinator boundary as navigation and hacking.
func (service *ConnectService) SetPresentation(_ context.Context, request *connect.Request[playerv1.SetPresentationRequest]) (*connect.Response[playerv1.ActionResult], error) {
	if service == nil || request == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("presentation request is required"))
	}
	mutation, err := PresentationFromProto(request.Msg)
	if err != nil {
		return nil, publicConnectError(err)
	}
	return service.dispatchRuntimeMutation(mutation), nil
}

// SoundManifest returns only allowlisted files from the embedded player asset
// filesystem. The typed category is validated before the filesystem is read.
func (service *ConnectService) SoundManifest(ctx context.Context, request *connect.Request[playerv1.SoundManifestRequest]) (*connect.Response[playerv1.SoundManifestResponse], error) {
	if service == nil || request == nil || request.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sound manifest request is required"))
	}
	if err := ctx.Err(); err != nil {
		return nil, connect.NewError(connect.CodeCanceled, errors.New("sound manifest request canceled"))
	}
	if err := ValidateMessageSize(request.Msg); err != nil {
		return nil, publicConnectError(err)
	}
	category, err := SoundCategoryFromProto(request.Msg.Category)
	if err != nil {
		return nil, publicConnectError(err)
	}

	assets := make([]string, 0)
	directory := "sounds/" + string(category)
	if service.assets != nil {
		entries, readErr := fs.ReadDir(service.assets, directory)
		if readErr == nil {
			for _, entry := range entries {
				info, infoErr := entry.Info()
				if infoErr != nil || !info.Mode().IsRegular() || path.Base(entry.Name()) != entry.Name() || strings.ContainsAny(entry.Name(), `/\\`) {
					continue
				}
				if _, allowed := allowedSoundExtensions[strings.ToLower(path.Ext(entry.Name()))]; !allowed {
					continue
				}
				assets = append(assets, directory+"/"+entry.Name())
			}
		}
	}
	sort.Strings(assets)
	return connect.NewResponse(SoundManifestToProto(domain.SoundManifest{Category: category, Assets: assets})), nil
}

func (service *ConnectService) dispatchRuntimeMutation(mutation RuntimeMutation) *connect.Response[playerv1.ActionResult] {
	result := service.coordinator.DispatchPlayerActionForRecognition(mutation.RecognitionHandle, mutation.Command)
	return connect.NewResponse(ActionResultToProto(result))
}

func publicConnectError(err error) error {
	if errors.Is(err, ErrResourceExhausted) {
		return connect.NewError(connect.CodeResourceExhausted, errors.New("public player request exceeds the configured limit"))
	}
	return connect.NewError(connect.CodeInvalidArgument, errors.New("public player request is invalid"))
}

func mapStreamError(err error) error {
	if errors.Is(err, context.Canceled) {
		return connect.NewError(connect.CodeCanceled, errors.New("player subscription canceled"))
	}
	return connect.NewError(connect.CodeUnavailable, errors.New("player subscription ended"))
}
