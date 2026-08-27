package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
	wailsnotifications "github.com/wailsapp/wails/v3/pkg/services/notifications"
)

const (
	approvalNotificationCategoryID = "fallout-terminal.command-approval"
	approvalNotificationApproveID  = "APPROVE"
	approvalNotificationRejectID   = "REJECT"
	approvalNotificationIDPrefix   = "fallout-terminal.command-approval:"
	approvalNotificationTitle      = "ТРЕБУЕТСЯ РЕШЕНИЕ СМОТРИТЕЛЯ"
	approvalNotificationBodyLimit  = 512
)

type approvalRequestKind string

const (
	approvalRequestCommandExecution   approvalRequestKind = "command-execution"
	approvalRequestTerminalNavigation approvalRequestKind = "terminal-navigation"
)

type approvalDeliveryState uint8

const (
	approvalDeliveryWaiting approvalDeliveryState = iota
	approvalDeliveryAttempting
	approvalDeliveryDelivered
	approvalDeliveryFailed
)

type approvalNotificationAvailability uint8

const (
	approvalNotificationsStarting approvalNotificationAvailability = iota
	approvalNotificationsAuthorizing
	approvalNotificationsReady
	approvalNotificationsUnavailable
	approvalNotificationsStopped
)

type approvalNotificationRequest struct {
	kind           approvalRequestKind
	requestID      string
	notificationID string
	body           string
}

type currentApprovalNotification struct {
	request         approvalNotificationRequest
	delivery        approvalDeliveryState
	decisionPending bool
	invalidated     bool
}

type approvalNotificationTarget interface {
	ResolveCommandExecution(CommandExecutionDecisionPayload) ResolveCommandExecutionResult
	ResolveTerminalNavigation(TerminalNavigationDecisionPayload) ResolveTerminalNavigationResult
}

type approvalNativeNotifier interface {
	ServiceStartup(context.Context, application.ServiceOptions) error
	ServiceShutdown() error
	CheckNotificationAuthorization() (bool, error)
	RequestNotificationAuthorization() (bool, error)
	RegisterNotificationCategory(wailsnotifications.NotificationCategory) error
	SendNotificationWithActions(wailsnotifications.NotificationOptions) error
	RemovePendingNotification(string) error
	RemoveDeliveredNotification(string) error
	OnNotificationResponse(func(wailsnotifications.NotificationResult))
}

// approvalNotificationService keeps the native notification API behind a
// root-only lifecycle boundary. The in-app decision flow remains authoritative.
type approvalNotificationService struct {
	mu       sync.Mutex
	nativeMu sync.Mutex

	log          logger.Logger
	native       approvalNativeNotifier
	target       approvalNotificationTarget
	availability approvalNotificationAvailability
	generation   uint64
	launchID     string
	launchIDErr  error
	current      *currentApprovalNotification
}

func newApprovalNotificationService(
	ctx context.Context,
	native approvalNativeNotifier,
	logs ...logger.Logger,
) *approvalNotificationService {
	if ctx == nil {
		panic(errWailsContextRequired)
	}
	applicationLogger := logger.FromContext(ctx)
	if len(logs) > 0 && logs[0] != nil {
		applicationLogger = logs[0]
	}
	launchID, launchIDErr := newApprovalNotificationLaunchID()
	return &approvalNotificationService{
		log: applicationLogger, native: native, availability: approvalNotificationsStarting,
		launchID: launchID, launchIDErr: launchIDErr,
	}
}

func (service *approvalNotificationService) bind(target approvalNotificationTarget) {
	service.mu.Lock()
	service.target = target
	service.mu.Unlock()
}

func (*approvalNotificationService) ServiceName() string {
	return "fallout-terminal-approval-notifications"
}

func (service *approvalNotificationService) ServiceStartup(
	ctx context.Context,
	options application.ServiceOptions,
) error {
	if service == nil || service.native == nil {
		return nil
	}
	service.mu.Lock()
	service.generation++
	generation := service.generation
	service.availability = approvalNotificationsStarting
	service.mu.Unlock()
	if service.launchIDErr != nil {
		service.disable("notification.launch-id", service.launchIDErr)
		return nil
	}

	if err := service.native.ServiceStartup(ctx, options); err != nil {
		service.disable("notification.start", err)
		return nil
	}
	service.native.OnNotificationResponse(service.handleResponse)
	if err := service.native.RegisterNotificationCategory(approvalNotificationCategory()); err != nil {
		service.disable("notification.category.register", err)
		return nil
	}

	service.mu.Lock()
	if service.generation == generation && service.availability != approvalNotificationsStopped {
		service.availability = approvalNotificationsAuthorizing
	}
	service.mu.Unlock()
	go service.authorize(generation)
	return nil
}

func (service *approvalNotificationService) ServiceShutdown() error {
	if service == nil || service.native == nil {
		return nil
	}
	service.mu.Lock()
	if service.availability == approvalNotificationsStopped {
		service.mu.Unlock()
		return nil
	}
	service.generation++
	generation := service.generation
	service.availability = approvalNotificationsStopped
	old := service.invalidateCurrentLocked()
	service.mu.Unlock()
	service.native.OnNotificationResponse(nil)
	go func() {
		service.cleanup(old, generation, true)
		service.nativeMu.Lock()
		err := service.native.ServiceShutdown()
		service.nativeMu.Unlock()
		if err != nil {
			service.logFailure("notification.shutdown", err)
		}
	}()
	return nil
}

func (service *approvalNotificationService) observeCoordinationState(state *domain.MasterCoordinationState) {
	if service == nil {
		return
	}
	request, valid := approvalRequestFromState(state)
	if !valid {
		service.log.WithField("operation", "notification.state.reduce").Warn("approval notification state was ambiguous")
	}

	service.mu.Lock()
	if service.availability == approvalNotificationsStopped {
		service.mu.Unlock()
		return
	}
	if request != nil && service.current != nil && !service.current.invalidated &&
		request.kind == service.current.request.kind && request.requestID == service.current.request.requestID {
		service.mu.Unlock()
		return
	}
	old := service.invalidateCurrentLocked()
	if request != nil {
		request.notificationID = approvalNotificationID(service.launchID, request.kind, request.requestID)
		service.current = &currentApprovalNotification{request: *request, delivery: approvalDeliveryWaiting}
	}
	readyRequest, generation := service.claimDeliveryLocked()
	service.mu.Unlock()

	if old != "" || readyRequest != nil {
		go service.applyTransition(old, readyRequest, generation)
	}
}

func (service *approvalNotificationService) authorize(generation uint64) {
	service.nativeMu.Lock()
	authorized, err := service.native.CheckNotificationAuthorization()
	if err == nil && !authorized {
		authorized, err = service.native.RequestNotificationAuthorization()
	}
	service.nativeMu.Unlock()
	if err != nil {
		service.logFailure("notification.authorization", err)
	}

	service.mu.Lock()
	if generation != service.generation || service.availability == approvalNotificationsStopped {
		service.mu.Unlock()
		return
	}
	if err != nil || !authorized {
		service.availability = approvalNotificationsUnavailable
		service.mu.Unlock()
		return
	}
	service.availability = approvalNotificationsReady
	request, currentGeneration := service.claimDeliveryLocked()
	service.mu.Unlock()
	if request != nil {
		go service.applyTransition("", request, currentGeneration)
	}
}

func (service *approvalNotificationService) applyTransition(
	oldNotificationID string,
	request *approvalNotificationRequest,
	generation uint64,
) {
	service.cleanup(oldNotificationID, generation, false)
	if request == nil {
		return
	}
	service.nativeMu.Lock()
	service.mu.Lock()
	current := service.current
	canSend := generation == service.generation && service.availability == approvalNotificationsReady &&
		current != nil && !current.invalidated && current.request.notificationID == request.notificationID &&
		current.delivery == approvalDeliveryAttempting
	service.mu.Unlock()
	if !canSend {
		service.nativeMu.Unlock()
		return
	}
	err := service.native.SendNotificationWithActions(approvalNotificationOptions(*request))
	authorizationUnavailable := false
	var authorizationErr error
	if err != nil {
		service.mu.Lock()
		canRecheckAuthorization := generation == service.generation &&
			service.availability == approvalNotificationsReady
		service.mu.Unlock()
		if canRecheckAuthorization {
			var authorized bool
			authorized, authorizationErr = service.native.CheckNotificationAuthorization()
			authorizationUnavailable = authorizationErr != nil || !authorized
		}
	}
	service.nativeMu.Unlock()

	service.mu.Lock()
	current = service.current
	if generation == service.generation {
		if current != nil && current.request.notificationID == request.notificationID &&
			current.delivery == approvalDeliveryAttempting {
			if err != nil {
				current.delivery = approvalDeliveryFailed
			} else {
				current.delivery = approvalDeliveryDelivered
			}
		}
		if authorizationUnavailable && service.availability != approvalNotificationsStopped {
			service.availability = approvalNotificationsUnavailable
		}
	}
	service.mu.Unlock()
	if err != nil {
		service.logFailure("notification.send", err)
	}
	if authorizationErr != nil {
		service.logFailure("notification.authorization.recheck", authorizationErr)
	}
}

func (service *approvalNotificationService) handleResponse(result wailsnotifications.NotificationResult) {
	if result.Error != nil {
		service.logFailure("notification.response", result.Error)
		return
	}
	action, ok := approvalDecisionForAction(result.Response.ActionIdentifier)
	if !ok || result.Response.CategoryID != approvalNotificationCategoryID {
		return
	}

	service.mu.Lock()
	current := service.current
	if service.availability == approvalNotificationsStopped || current == nil || current.invalidated ||
		current.decisionPending || result.Response.ID != current.request.notificationID {
		service.mu.Unlock()
		return
	}
	current.decisionPending = true
	request := current.request
	target := service.target
	generation := service.generation
	service.mu.Unlock()
	if target == nil {
		service.releaseDecision(request.notificationID, generation)
		return
	}

	succeeded := false
	switch request.kind {
	case approvalRequestCommandExecution:
		result := target.ResolveCommandExecution(CommandExecutionDecisionPayload{
			RequestID: request.requestID,
			Decision:  domain.CommandExecutionDecision(action),
		})
		succeeded = result.OK
	case approvalRequestTerminalNavigation:
		result := target.ResolveTerminalNavigation(TerminalNavigationDecisionPayload{
			RequestID: request.requestID,
			Decision:  domain.TerminalNavigationDecision(action),
		})
		succeeded = result.OK
	}
	if !succeeded {
		service.releaseDecision(request.notificationID, generation)
		return
	}
	service.mu.Lock()
	if generation == service.generation && service.current != nil &&
		service.current.request.notificationID == request.notificationID {
		old := service.invalidateCurrentLocked()
		service.mu.Unlock()
		go service.cleanup(old, generation, false)
		return
	}
	service.mu.Unlock()
}

func (service *approvalNotificationService) releaseDecision(notificationID string, generation uint64) {
	service.mu.Lock()
	if generation == service.generation && service.current != nil && !service.current.invalidated &&
		service.current.request.notificationID == notificationID {
		service.current.decisionPending = false
	}
	service.mu.Unlock()
}

func (service *approvalNotificationService) claimDeliveryLocked() (*approvalNotificationRequest, uint64) {
	if service.availability != approvalNotificationsReady || service.current == nil ||
		service.current.invalidated || service.current.delivery != approvalDeliveryWaiting {
		return nil, service.generation
	}
	service.current.delivery = approvalDeliveryAttempting
	request := service.current.request
	return &request, service.generation
}

func (service *approvalNotificationService) invalidateCurrentLocked() string {
	if service.current == nil {
		return ""
	}
	service.current.invalidated = true
	notificationID := service.current.request.notificationID
	service.current = nil
	return notificationID
}

func (service *approvalNotificationService) cleanup(
	notificationID string,
	generation uint64,
	allowStopped bool,
) {
	if notificationID == "" || service.native == nil {
		return
	}
	service.nativeMu.Lock()
	service.mu.Lock()
	canRemove := generation == service.generation &&
		(service.availability != approvalNotificationsStopped || allowStopped)
	service.mu.Unlock()
	if !canRemove {
		service.nativeMu.Unlock()
		return
	}
	pendingErr := service.native.RemovePendingNotification(notificationID)
	deliveredErr := service.native.RemoveDeliveredNotification(notificationID)
	service.nativeMu.Unlock()
	if pendingErr != nil {
		service.logFailure("notification.remove.pending", pendingErr)
	}
	if deliveredErr != nil {
		service.logFailure("notification.remove.delivered", deliveredErr)
	}
}

func (service *approvalNotificationService) disable(operation string, err error) {
	service.mu.Lock()
	if service.availability != approvalNotificationsStopped {
		service.availability = approvalNotificationsUnavailable
	}
	service.mu.Unlock()
	service.logFailure(operation, err)
}

func (service *approvalNotificationService) logFailure(operation string, err error) {
	service.log.WithError(err).WithField("operation", operation).Warn("native approval notification unavailable")
}

func approvalNotificationCategory() wailsnotifications.NotificationCategory {
	return wailsnotifications.NotificationCategory{
		ID: approvalNotificationCategoryID,
		Actions: []wailsnotifications.NotificationAction{
			{ID: approvalNotificationApproveID, Title: "ОДОБРИТЬ"},
			{ID: approvalNotificationRejectID, Title: "ОТКЛОНИТЬ", Destructive: true},
		},
	}
}

func approvalNotificationOptions(request approvalNotificationRequest) wailsnotifications.NotificationOptions {
	return wailsnotifications.NotificationOptions{
		ID:                request.notificationID,
		Title:             approvalNotificationTitle,
		Body:              request.body,
		CategoryID:        approvalNotificationCategoryID,
		InterruptionLevel: wailsnotifications.InterruptionLevelActive,
	}
}

func approvalRequestFromState(state *domain.MasterCoordinationState) (*approvalNotificationRequest, bool) {
	if state == nil || (state.PendingCommandExecution == nil && state.PendingTerminalNavigation == nil) {
		return nil, true
	}
	if state.PendingCommandExecution != nil && state.PendingTerminalNavigation != nil {
		return nil, false
	}
	if pending := state.PendingCommandExecution; pending != nil {
		requestID := strings.TrimSpace(pending.RequestID)
		if requestID == "" {
			return nil, false
		}
		command := firstNonBlank(pending.CommandName, pending.CommandID)
		body := "КОМАНДА: " + command
		if confirmation := strings.TrimSpace(pending.ConfirmationText); confirmation != "" {
			body += "\n" + confirmation
		}
		return newApprovalNotificationRequest(approvalRequestCommandExecution, requestID, body), true
	}
	pending := state.PendingTerminalNavigation
	requestID := strings.TrimSpace(pending.RequestID)
	if requestID == "" {
		return nil, false
	}
	body := "КОМАНДА: " + firstNonBlank(pending.CommandName, pending.CommandID) + "\n" +
		firstNonBlank(pending.SourceTerminalName, pending.SourceTerminalID) + " → " +
		firstNonBlank(pending.TargetTerminalName, pending.TargetTerminalID)
	return newApprovalNotificationRequest(approvalRequestTerminalNavigation, requestID, body), true
}

func newApprovalNotificationRequest(kind approvalRequestKind, requestID, body string) *approvalNotificationRequest {
	return &approvalNotificationRequest{
		kind:      kind,
		requestID: requestID,
		body:      truncateRunes(body, approvalNotificationBodyLimit),
	}
}

func newApprovalNotificationLaunchID() (string, error) {
	var value [16]byte
	if _, err := cryptorand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func approvalNotificationID(launchID string, kind approvalRequestKind, requestID string) string {
	return approvalNotificationIDPrefix + launchID + ":" + string(kind) + ":" + requestID
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func approvalDecisionForAction(action string) (string, bool) {
	switch action {
	case approvalNotificationApproveID:
		return string(domain.CommandExecutionApprove), true
	case approvalNotificationRejectID:
		return string(domain.CommandExecutionReject), true
	default:
		return "", false
	}
}
