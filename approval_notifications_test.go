package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	controlservice "github.com/obalunenko/Fallout-Terminal/v2/internal/control"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	liveservice "github.com/obalunenko/Fallout-Terminal/v2/internal/live"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v3/pkg/application"
	wailsnotifications "github.com/wailsapp/wails/v3/pkg/services/notifications"
)

const approvalNotificationTestLaunchID = "test-launch"

type approvalTerminalNavigationCatalog struct {
	source    domain.TerminalTarget
	target    domain.TerminalTarget
	commandID string
}

func (catalog approvalTerminalNavigationCatalog) LookupTerminal(terminalID string) (domain.TerminalTarget, bool) {
	switch terminalID {
	case catalog.source.TerminalID:
		return catalog.source, true
	case catalog.target.TerminalID:
		return catalog.target, true
	default:
		return domain.TerminalTarget{}, false
	}
}

func (catalog approvalTerminalNavigationCatalog) LookupTerminalTransition(sourceTerminalID, commandID string) (domain.TerminalTransitionTarget, bool) {
	if sourceTerminalID != catalog.source.TerminalID || commandID != catalog.commandID {
		return domain.TerminalTransitionTarget{}, false
	}
	return domain.TerminalTransitionTarget{
		SourceTerminalID:   catalog.source.TerminalID,
		SourceTerminalName: catalog.source.TerminalName,
		CommandID:          catalog.commandID,
		CommandName:        "OPEN TARGET",
		Target:             catalog.target,
	}, true
}

type fakeApprovalNativeNotifier struct {
	mu sync.Mutex

	startupErr, categoryErr, checkErr, requestErr, sendErr error
	pendingRemoveErr, deliveredRemoveErr, shutdownErr      error
	checkAuthorized, requestAuthorized                     bool
	startupCalls, checkCalls, requestCalls, shutdownCalls  int
	categories                                             []wailsnotifications.NotificationCategory
	notifications                                          []wailsnotifications.NotificationOptions
	pendingRemovals, deliveredRemovals                     []string
	callback                                               func(wailsnotifications.NotificationResult)
	checkEntered, requestEntered                           chan<- struct{}
	checkRelease, requestRelease                           <-chan struct{}
}

func (fake *fakeApprovalNativeNotifier) ServiceStartup(context.Context, application.ServiceOptions) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.startupCalls++
	return fake.startupErr
}

func (fake *fakeApprovalNativeNotifier) ServiceShutdown() error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.shutdownCalls++
	return fake.shutdownErr
}

func (fake *fakeApprovalNativeNotifier) CheckNotificationAuthorization() (bool, error) {
	fake.mu.Lock()
	fake.checkCalls++
	authorized, err := fake.checkAuthorized, fake.checkErr
	entered, release := fake.checkEntered, fake.checkRelease
	fake.mu.Unlock()
	waitForFakeApprovalNativeOperation(entered, release)
	return authorized, err
}

func (fake *fakeApprovalNativeNotifier) RequestNotificationAuthorization() (bool, error) {
	fake.mu.Lock()
	fake.requestCalls++
	authorized, err := fake.requestAuthorized, fake.requestErr
	entered, release := fake.requestEntered, fake.requestRelease
	fake.mu.Unlock()
	waitForFakeApprovalNativeOperation(entered, release)
	return authorized, err
}

func (fake *fakeApprovalNativeNotifier) RegisterNotificationCategory(category wailsnotifications.NotificationCategory) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.categories = append(fake.categories, category)
	return fake.categoryErr
}

func (fake *fakeApprovalNativeNotifier) SendNotificationWithActions(options wailsnotifications.NotificationOptions) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.notifications = append(fake.notifications, options)
	return fake.sendErr
}

func (fake *fakeApprovalNativeNotifier) RemovePendingNotification(id string) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.pendingRemovals = append(fake.pendingRemovals, id)
	return fake.pendingRemoveErr
}

func (fake *fakeApprovalNativeNotifier) RemoveDeliveredNotification(id string) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.deliveredRemovals = append(fake.deliveredRemovals, id)
	return fake.deliveredRemoveErr
}

func (fake *fakeApprovalNativeNotifier) OnNotificationResponse(callback func(wailsnotifications.NotificationResult)) {
	fake.mu.Lock()
	fake.callback = callback
	fake.mu.Unlock()
}

func (fake *fakeApprovalNativeNotifier) respond(result wailsnotifications.NotificationResult) {
	fake.mu.Lock()
	callback := fake.callback
	fake.mu.Unlock()
	if callback != nil {
		callback(result)
	}
}

func (fake *fakeApprovalNativeNotifier) capturedCallback() func(wailsnotifications.NotificationResult) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.callback
}

func (fake *fakeApprovalNativeNotifier) setSendError(err error) {
	fake.mu.Lock()
	fake.sendErr = err
	fake.mu.Unlock()
}

func (fake *fakeApprovalNativeNotifier) setAuthorization(authorized bool, err error) {
	fake.mu.Lock()
	fake.checkAuthorized = authorized
	fake.checkErr = err
	fake.mu.Unlock()
}

func waitForFakeApprovalNativeOperation(entered chan<- struct{}, release <-chan struct{}) {
	if entered != nil {
		entered <- struct{}{}
	}
	if release != nil {
		<-release
	}
}

type fakeApprovalNativeSnapshot struct {
	startupCalls, checkCalls, requestCalls, shutdownCalls int
	categories                                            []wailsnotifications.NotificationCategory
	notifications                                         []wailsnotifications.NotificationOptions
	pendingRemovals, deliveredRemovals                    []string
}

func (fake *fakeApprovalNativeNotifier) snapshot() fakeApprovalNativeSnapshot {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fakeApprovalNativeSnapshot{
		startupCalls: fake.startupCalls, checkCalls: fake.checkCalls, requestCalls: fake.requestCalls,
		shutdownCalls:     fake.shutdownCalls,
		categories:        append([]wailsnotifications.NotificationCategory(nil), fake.categories...),
		notifications:     append([]wailsnotifications.NotificationOptions(nil), fake.notifications...),
		pendingRemovals:   append([]string(nil), fake.pendingRemovals...),
		deliveredRemovals: append([]string(nil), fake.deliveredRemovals...),
	}
}

type recordedApprovalDecision struct {
	kind      approvalRequestKind
	requestID string
	decision  string
}

type fakeApprovalTarget struct {
	mu        sync.Mutex
	succeed   bool
	decisions []recordedApprovalDecision
	entered   chan<- struct{}
	release   <-chan struct{}
}

func (target *fakeApprovalTarget) ResolveCommandExecution(payload CommandExecutionDecisionPayload) ResolveCommandExecutionResult {
	target.mu.Lock()
	target.decisions = append(target.decisions, recordedApprovalDecision{
		kind: approvalRequestCommandExecution, requestID: payload.RequestID, decision: string(payload.Decision),
	})
	succeed, entered, release := target.succeed, target.entered, target.release
	target.mu.Unlock()
	target.wait(entered, release)
	return ResolveCommandExecutionResult{OK: succeed}
}

func (target *fakeApprovalTarget) ResolveTerminalNavigation(payload TerminalNavigationDecisionPayload) ResolveTerminalNavigationResult {
	target.mu.Lock()
	target.decisions = append(target.decisions, recordedApprovalDecision{
		kind: approvalRequestTerminalNavigation, requestID: payload.RequestID, decision: string(payload.Decision),
	})
	succeed, entered, release := target.succeed, target.entered, target.release
	target.mu.Unlock()
	target.wait(entered, release)
	return ResolveTerminalNavigationResult{OK: succeed}
}

func (target *fakeApprovalTarget) snapshot() []recordedApprovalDecision {
	target.mu.Lock()
	defer target.mu.Unlock()
	return append([]recordedApprovalDecision(nil), target.decisions...)
}

func (target *fakeApprovalTarget) setSucceed(succeed bool) {
	target.mu.Lock()
	target.succeed = succeed
	target.mu.Unlock()
}

func (*fakeApprovalTarget) wait(entered chan<- struct{}, release <-chan struct{}) {
	if entered != nil {
		entered <- struct{}{}
	}
	if release != nil {
		<-release
	}
}

func commandApprovalState(revision uint64, requestID string, mode domain.CommandApprovalMode) *domain.MasterCoordinationState {
	return &domain.MasterCoordinationState{
		Revision: revision,
		PendingCommandExecution: &domain.MasterPendingCommandExecution{
			RequestID: requestID, TerminalID: "terminal-1", CommandID: "unlock",
			CommandName: "ОТКРЫТЬ УБЕЖИЩЕ", Mode: mode, ConfirmationText: "Подтвердите выполнение.",
		},
	}
}

func navigationApprovalState(revision uint64, requestID string) *domain.MasterCoordinationState {
	return &domain.MasterCoordinationState{
		Revision: revision,
		PendingTerminalNavigation: &domain.MasterPendingTerminalNavigation{
			RequestID: requestID, SourceTerminalID: "source", SourceTerminalName: "ВХОД",
			CommandID: "travel", CommandName: "ПЕРЕЙТИ", TargetTerminalID: "target", TargetTerminalName: "РЕАКТОР",
		},
	}
}

func startReadyApprovalNotifications(
	t *testing.T,
	fake *fakeApprovalNativeNotifier,
	launchIDs ...string,
) *approvalNotificationService {
	t.Helper()
	fake.checkAuthorized = true
	service := newApprovalNotificationService(t.Context(), fake)
	service.launchID = approvalNotificationTestLaunchID
	service.launchIDErr = nil
	if len(launchIDs) > 0 {
		service.launchID = launchIDs[0]
	}
	require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))
	t.Cleanup(func() { require.NoError(t, service.ServiceShutdown()) })
	require.Eventually(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return fake.checkCalls == 1
	}, time.Second, time.Millisecond)
	return service
}

func TestApprovalNotificationDeliveryContract(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		state *domain.MasterCoordinationState
		body  string
		kind  approvalRequestKind
	}{
		{name: "ordinary command", state: commandApprovalState(1, "request-ordinary", domain.CommandApprovalModeOrdinary), body: "КОМАНДА: ОТКРЫТЬ УБЕЖИЩЕ\nПодтвердите выполнение.", kind: approvalRequestCommandExecution},
		{name: "state changing command", state: commandApprovalState(1, "request-state", domain.CommandApprovalModeStateChange), body: "КОМАНДА: ОТКРЫТЬ УБЕЖИЩЕ\nПодтвердите выполнение.", kind: approvalRequestCommandExecution},
		{name: "completed state changing command", state: commandApprovalState(1, "request-completed", domain.CommandApprovalModeCompletedStateChange), body: "КОМАНДА: ОТКРЫТЬ УБЕЖИЩЕ\nПодтвердите выполнение.", kind: approvalRequestCommandExecution},
		{name: "terminal navigation", state: navigationApprovalState(1, "request-navigation"), body: "КОМАНДА: ПЕРЕЙТИ\nВХОД → РЕАКТОР", kind: approvalRequestTerminalNavigation},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fake := &fakeApprovalNativeNotifier{}
			service := startReadyApprovalNotifications(t, fake)
			service.observeCoordinationState(test.state)
			service.observeCoordinationState(domain.CloneMasterCoordinationState(test.state))
			require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
			snapshot := fake.snapshot()
			require.Equal(t, approvalNotificationCategory(), snapshot.categories[0])
			require.Equal(t, wailsnotifications.NotificationOptions{
				ID: approvalNotificationID(approvalNotificationTestLaunchID, test.kind, func() string {
					if test.state.PendingCommandExecution != nil {
						return test.state.PendingCommandExecution.RequestID
					}
					return test.state.PendingTerminalNavigation.RequestID
				}()),
				Title: approvalNotificationTitle, Body: test.body, CategoryID: approvalNotificationCategoryID,
				InterruptionLevel: wailsnotifications.InterruptionLevelActive,
			}, snapshot.notifications[0])
			assert.Nil(t, snapshot.notifications[0].Data)
		})
	}
}

func TestApprovalNotificationReplacementAndClearCleanExactIDs(t *testing.T) {
	t.Parallel()
	fake := &fakeApprovalNativeNotifier{}
	service := startReadyApprovalNotifications(t, fake)
	service.observeCoordinationState(commandApprovalState(1, "one", domain.CommandApprovalModeOrdinary))
	require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	service.observeCoordinationState(navigationApprovalState(2, "two"))
	require.Eventually(t, func() bool {
		snapshot := fake.snapshot()
		return len(snapshot.notifications) == 2 && len(snapshot.pendingRemovals) == 1 && len(snapshot.deliveredRemovals) == 1
	}, time.Second, time.Millisecond)
	service.observeCoordinationState(&domain.MasterCoordinationState{Revision: 3})
	require.Eventually(t, func() bool {
		snapshot := fake.snapshot()
		return len(snapshot.pendingRemovals) == 2 && len(snapshot.deliveredRemovals) == 2
	}, time.Second, time.Millisecond)
	require.Equal(t, []string{
		approvalNotificationID(approvalNotificationTestLaunchID, approvalRequestCommandExecution, "one"),
		approvalNotificationID(approvalNotificationTestLaunchID, approvalRequestTerminalNavigation, "two"),
	}, fake.snapshot().pendingRemovals)
}

func TestApprovalNotificationRejectsOldLaunchResponseForRestoredRequest(t *testing.T) {
	t.Parallel()
	const requestID = "restored-request"

	oldFake := &fakeApprovalNativeNotifier{}
	oldService := startReadyApprovalNotifications(t, oldFake, "old-launch")
	oldService.observeCoordinationState(commandApprovalState(1, requestID, domain.CommandApprovalModeOrdinary))
	require.Eventually(t, func() bool { return len(oldFake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	oldOption := oldFake.snapshot().notifications[0]
	require.NoError(t, oldService.ServiceShutdown())

	currentFake := &fakeApprovalNativeNotifier{}
	target := &fakeApprovalTarget{succeed: true}
	currentService := startReadyApprovalNotifications(t, currentFake, "current-launch")
	currentService.bind(target)
	currentService.observeCoordinationState(commandApprovalState(1, requestID, domain.CommandApprovalModeOrdinary))
	require.Eventually(t, func() bool { return len(currentFake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	currentOption := currentFake.snapshot().notifications[0]
	require.NotEqual(t, oldOption.ID, currentOption.ID)

	currentService.handleResponse(wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
		ID: oldOption.ID, CategoryID: oldOption.CategoryID, ActionIdentifier: approvalNotificationApproveID,
	}})
	require.Empty(t, target.snapshot())
	currentService.handleResponse(wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
		ID: currentOption.ID, CategoryID: currentOption.CategoryID, ActionIdentifier: approvalNotificationApproveID,
	}})
	require.Len(t, target.snapshot(), 1)
}

func TestApprovalNotificationResponsesRouteOnlyCurrentTrustedAction(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, action, decision string
		state                  *domain.MasterCoordinationState
		kind                   approvalRequestKind
	}{
		{name: "approve command", action: approvalNotificationApproveID, decision: "approve", state: commandApprovalState(1, "command", domain.CommandApprovalModeOrdinary), kind: approvalRequestCommandExecution},
		{name: "reject command", action: approvalNotificationRejectID, decision: "reject", state: commandApprovalState(1, "command", domain.CommandApprovalModeOrdinary), kind: approvalRequestCommandExecution},
		{name: "reject navigation", action: approvalNotificationRejectID, decision: "reject", state: navigationApprovalState(1, "navigation"), kind: approvalRequestTerminalNavigation},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fake := &fakeApprovalNativeNotifier{}
			target := &fakeApprovalTarget{succeed: true}
			service := startReadyApprovalNotifications(t, fake)
			service.bind(target)
			service.observeCoordinationState(test.state)
			require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
			option := fake.snapshot().notifications[0]
			fake.respond(wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
				ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: test.action,
				UserInfo: map[string]any{"requestID": "attacker-controlled"},
			}})
			require.Equal(t, []recordedApprovalDecision{{
				kind: test.kind, requestID: func() string {
					if test.state.PendingCommandExecution != nil {
						return test.state.PendingCommandExecution.RequestID
					}
					return test.state.PendingTerminalNavigation.RequestID
				}(), decision: test.decision,
			}}, target.snapshot())
		})
	}
}

func TestApprovalNotificationCommandRejectUsesAppDecisionPathAndPublishesAccessError(t *testing.T) {
	const commandID = "diagnostics"
	completedState := domain.CommandExecutionState{CompletedName: "DIAGNOSTICS COMPLETE", ResultText: "DONE"}
	for _, test := range []struct {
		name          string
		stateChange   *domain.StateChangeConfig
		commandStates map[string]domain.CommandExecutionState
		wantMode      domain.CommandApprovalMode
	}{
		{name: "ordinary", wantMode: domain.CommandApprovalModeOrdinary},
		{
			name: "completed state changing",
			stateChange: &domain.StateChangeConfig{
				CompletedName: "DIAGNOSTICS COMPLETE", ConfirmationText: "Run diagnostics again?",
			},
			commandStates: map[string]domain.CommandExecutionState{commandID: completedState},
			wantMode:      domain.CommandApprovalModeCompletedStateChange,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			live := liveservice.New(nil, nil)
			rejectedEffects := make(chan *domain.PublicLiveState, 1)
			coordination := controlservice.New(controlservice.Config{
				Runtime: live, Terminals: live, TrustedHack: live,
				Enqueue: func(effect controlservice.Effect) {
					if effect.Live != nil && effect.Live.CommandExecution != nil &&
						effect.Live.CommandExecution.Phase == domain.CommandExecutionPhaseRejected {
						rejectedEffects <- effect.Live
					}
				},
			})
			state, err := coordination.AddCharacter(domain.CharacterCreatePayload{
				Name: "Mara", Intelligence: 1, HackerPerkAvailable: false,
				ExpectedRevision: coordination.Revision(),
			})
			require.NoError(t, err)
			require.Len(t, state.Roster, 1)
			state, err = coordination.StartBroadcast()
			require.NoError(t, err)
			require.NotNil(t, state.Broadcast)

			connectionID := domain.ConnectionID("notification-controller-" + test.name)
			session := coordination.CreateSession(connectionID)
			selected := coordination.SelectCharacter(controlservice.CharacterSelection{
				ConnectionID: connectionID, SessionID: session.SessionID,
				RequestID: "notification-select-" + test.name, BroadcastID: state.Broadcast.ID,
				CharacterID: state.Roster[0].ID,
			})
			require.True(t, selected.Accepted)
			_, err = coordination.RequestTerminalActivation(domain.TerminalTarget{
				TerminalID: "terminal-1", TerminalName: "Diagnostics", HackLevel: 0,
				Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{{
					ID: commandID, Type: domain.NodeCommand, Name: "RUN", Text: "DONE", StateChange: test.stateChange,
				}}},
				CommandStates: test.commandStates,
			})
			require.NoError(t, err)
			selected = coordination.DispatchPlayerAction(connectionID, domain.RuntimeCommand{
				RequestID: "notification-command-" + test.name, BroadcastID: state.Broadcast.ID,
				TerminalID: "terminal-1", Kind: domain.RuntimeCommandNavigate,
				Action: "command", NodeID: commandID,
			})
			require.True(t, selected.Accepted)
			pending := coordination.Snapshot().PendingCommandExecution
			require.NotNil(t, pending)
			require.Equal(t, test.wantMode, pending.Mode)

			app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: coordination})
			fake := &fakeApprovalNativeNotifier{}
			service := startReadyApprovalNotifications(t, fake)
			service.bind(app)
			service.observeCoordinationState(coordination.Snapshot())
			require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
			option := fake.snapshot().notifications[0]
			fake.respond(wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
				ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: approvalNotificationRejectID,
			}})

			require.Eventually(t, func() bool {
				return coordination.Snapshot().PendingCommandExecution == nil
			}, time.Second, time.Millisecond)
			var rejected *domain.PublicLiveState
			select {
			case rejected = <-rejectedEffects:
			case <-time.After(time.Second):
				t.Fatal("notification rejection did not publish a rejected live state")
			}
			require.NotNil(t, rejected)
			require.Equal(t, &domain.CommandExecutionPresentation{
				Phase: domain.CommandExecutionPhaseRejected, CommandID: commandID,
			}, rejected.CommandExecution)
			require.Equal(t, domain.NavState{Path: []string{"root"}, Mode: "list"}, rejected.Nav)
		})
	}
}

func TestApprovalNotificationForwardTransitionRejectUsesAppDecisionPathAndPublishesAccessError(t *testing.T) {
	const (
		sourceID  = "terminal-source"
		targetID  = "terminal-target"
		commandID = "open-target"
	)
	source := domain.TerminalTarget{
		TerminalID: sourceID, TerminalName: "Source", HackLevel: 0,
		Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{{
			ID: commandID, Type: domain.NodeCommand, Name: "OPEN TARGET",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: targetID},
		}}},
	}
	target := domain.TerminalTarget{
		TerminalID: targetID, TerminalName: "Target", HackLevel: 0,
		Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT"},
	}
	live := liveservice.New(nil, nil)
	rejectedEffects := make(chan *domain.PublicLiveState, 1)
	coordination := controlservice.New(controlservice.Config{
		Runtime: live, Terminals: live, TrustedHack: live,
		TerminalCatalog: approvalTerminalNavigationCatalog{source: source, target: target, commandID: commandID},
		Enqueue: func(effect controlservice.Effect) {
			if effect.Live != nil && effect.Live.CommandExecution != nil &&
				effect.Live.CommandExecution.Phase == domain.CommandExecutionPhaseRejected {
				rejectedEffects <- effect.Live
			}
		},
	})
	state, err := coordination.AddCharacter(domain.CharacterCreatePayload{
		Name: "Mara", Intelligence: 1, HackerPerkAvailable: false,
		ExpectedRevision: coordination.Revision(),
	})
	require.NoError(t, err)
	require.Len(t, state.Roster, 1)
	state, err = coordination.StartBroadcast()
	require.NoError(t, err)
	require.NotNil(t, state.Broadcast)

	connectionID := domain.ConnectionID("notification-transition-controller")
	session := coordination.CreateSession(connectionID)
	selected := coordination.SelectCharacter(controlservice.CharacterSelection{
		ConnectionID: connectionID, SessionID: session.SessionID,
		RequestID: "notification-transition-select", BroadcastID: state.Broadcast.ID,
		CharacterID: state.Roster[0].ID,
	})
	require.True(t, selected.Accepted)
	_, err = coordination.RequestTerminalActivation(source)
	require.NoError(t, err)
	selected = coordination.DispatchPlayerAction(connectionID, domain.RuntimeCommand{
		RequestID: "notification-transition", BroadcastID: state.Broadcast.ID,
		TerminalID: sourceID, Kind: domain.RuntimeCommandNavigate,
		Action: "command", NodeID: commandID,
	})
	require.True(t, selected.Accepted)
	pending := coordination.Snapshot().PendingTerminalNavigation
	require.NotNil(t, pending)
	require.Equal(t, domain.TerminalNavigationForward, pending.Direction)

	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: coordination})
	fake := &fakeApprovalNativeNotifier{}
	service := startReadyApprovalNotifications(t, fake)
	service.bind(app)
	service.observeCoordinationState(coordination.Snapshot())
	require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	option := fake.snapshot().notifications[0]
	fake.respond(wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
		ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: approvalNotificationRejectID,
	}})

	require.Eventually(t, func() bool {
		return coordination.Snapshot().PendingTerminalNavigation == nil
	}, time.Second, time.Millisecond)
	var rejected *domain.PublicLiveState
	select {
	case rejected = <-rejectedEffects:
	case <-time.After(time.Second):
		t.Fatal("forward-transition notification rejection did not publish a rejected live state")
	}
	require.Equal(t, sourceID, rejected.TerminalID)
	require.Equal(t, domain.NavState{Path: []string{"root"}, Mode: "list"}, rejected.Nav)
	require.Equal(t, &domain.CommandExecutionPresentation{
		Phase: domain.CommandExecutionPhaseRejected, CommandID: commandID,
	}, rejected.CommandExecution)
	require.Nil(t, rejected.TerminalNavigation)
	final := coordination.Snapshot()
	require.Equal(t, sourceID, *final.Broadcast.ActiveTerminalID)
}

func TestApprovalNotificationIgnoresMalformedStaleAndRepeatedResponses(t *testing.T) {
	t.Parallel()
	fake := &fakeApprovalNativeNotifier{}
	target := &fakeApprovalTarget{succeed: true}
	service := startReadyApprovalNotifications(t, fake)
	service.bind(target)
	service.observeCoordinationState(commandApprovalState(1, "current", domain.CommandApprovalModeOrdinary))
	require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	option := fake.snapshot().notifications[0]
	invalid := []wailsnotifications.NotificationResult{
		{Error: errors.New("callback failed")},
		{Response: wailsnotifications.NotificationResponse{ID: "old-process", CategoryID: option.CategoryID, ActionIdentifier: approvalNotificationApproveID}},
		{Response: wailsnotifications.NotificationResponse{ID: option.ID, CategoryID: "wrong", ActionIdentifier: approvalNotificationApproveID}},
		{Response: wailsnotifications.NotificationResponse{ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: wailsnotifications.DefaultActionIdentifier}},
		{Response: wailsnotifications.NotificationResponse{ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: "UNKNOWN"}},
	}
	for _, result := range invalid {
		fake.respond(result)
	}
	require.Empty(t, target.snapshot())

	response := wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
		ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: approvalNotificationApproveID,
	}}
	var responses sync.WaitGroup
	for range 100 {
		responses.Go(func() {
			fake.respond(response)
		})
	}
	responses.Wait()
	require.Len(t, target.snapshot(), 1)
}

func TestApprovalNotificationDecisionFailurePreservesCurrentRequestForRetry(t *testing.T) {
	t.Parallel()
	fake := &fakeApprovalNativeNotifier{}
	target := &fakeApprovalTarget{}
	service := startReadyApprovalNotifications(t, fake)
	service.bind(target)
	service.observeCoordinationState(commandApprovalState(1, "retry-current", domain.CommandApprovalModeStateChange))
	require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	option := fake.snapshot().notifications[0]
	response := wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
		ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: approvalNotificationApproveID,
	}}

	fake.respond(response)
	require.Len(t, target.snapshot(), 1)
	require.Empty(t, fake.snapshot().pendingRemovals)
	service.mu.Lock()
	require.NotNil(t, service.current)
	require.False(t, service.current.decisionPending)
	require.False(t, service.current.invalidated)
	service.mu.Unlock()

	target.setSucceed(true)
	fake.respond(response)
	require.Len(t, target.snapshot(), 2)
	require.Eventually(t, func() bool {
		return len(fake.snapshot().pendingRemovals) == 1 && len(fake.snapshot().deliveredRemovals) == 1
	}, time.Second, time.Millisecond)
}

func TestApprovalNotificationInAppResolutionWinsWhileNativeDecisionIsPending(t *testing.T) {
	t.Parallel()
	fake := &fakeApprovalNativeNotifier{}
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	target := &fakeApprovalTarget{entered: entered, release: release}
	service := startReadyApprovalNotifications(t, fake)
	service.bind(target)
	service.observeCoordinationState(commandApprovalState(1, "in-app-wins", domain.CommandApprovalModeOrdinary))
	require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	option := fake.snapshot().notifications[0]
	response := wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
		ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: approvalNotificationApproveID,
	}}
	responseDone := make(chan struct{})
	go func() {
		fake.respond(response)
		close(responseDone)
	}()

	require.Eventually(t, func() bool { return len(entered) == 1 }, time.Second, time.Millisecond)
	<-entered
	// This authoritative clear represents the in-app path winning while the
	// native callback is waiting in the same App decision boundary.
	service.observeCoordinationState(&domain.MasterCoordinationState{Revision: 2})
	releaseOnce.Do(func() { close(release) })
	require.Eventually(t, func() bool {
		select {
		case <-responseDone:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
	require.Len(t, target.snapshot(), 1)
	fake.respond(response)
	require.Len(t, target.snapshot(), 1, "the stale native response reached App after the in-app decision")
	require.Eventually(t, func() bool {
		return len(fake.snapshot().pendingRemovals) == 1 && len(fake.snapshot().deliveredRemovals) == 1
	}, time.Second, time.Millisecond)
}

func TestApprovalNotificationFailuresRemainNonfatal(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		configure func(*fakeApprovalNativeNotifier)
	}{
		{name: "native startup", configure: func(fake *fakeApprovalNativeNotifier) { fake.startupErr = errors.New("unavailable") }},
		{name: "category", configure: func(fake *fakeApprovalNativeNotifier) { fake.categoryErr = errors.New("unsupported") }},
		{name: "authorization check", configure: func(fake *fakeApprovalNativeNotifier) { fake.checkErr = errors.New("permission failed") }},
		{name: "authorization denied", configure: func(fake *fakeApprovalNativeNotifier) { fake.checkAuthorized = false; fake.requestAuthorized = false }},
		{name: "authorization request", configure: func(fake *fakeApprovalNativeNotifier) { fake.requestErr = errors.New("permission timeout") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fake := &fakeApprovalNativeNotifier{}
			test.configure(fake)
			service := newApprovalNotificationService(t.Context(), fake)
			require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))
			t.Cleanup(func() { require.NoError(t, service.ServiceShutdown()) })
			service.observeCoordinationState(commandApprovalState(1, "fallback", domain.CommandApprovalModeOrdinary))
			require.Never(t, func() bool { return len(fake.snapshot().notifications) != 0 }, 20*time.Millisecond, time.Millisecond)
		})
	}
}

func TestApprovalNotificationSendCleanupAndShutdownFailuresRemainNonfatal(t *testing.T) {
	t.Parallel()
	fake := &fakeApprovalNativeNotifier{
		sendErr: errors.New("missing notification daemon"), pendingRemoveErr: errors.New("pending cleanup"),
		deliveredRemoveErr: errors.New("delivered cleanup"), shutdownErr: errors.New("shutdown"),
	}
	service := startReadyApprovalNotifications(t, fake)
	service.observeCoordinationState(commandApprovalState(1, "fallback", domain.CommandApprovalModeOrdinary))
	require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	service.observeCoordinationState(&domain.MasterCoordinationState{Revision: 2})
	require.Eventually(t, func() bool { return len(fake.snapshot().pendingRemovals) == 1 }, time.Second, time.Millisecond)
	require.NoError(t, service.ServiceShutdown())
	require.Eventually(t, func() bool { return fake.snapshot().shutdownCalls >= 1 }, time.Second, time.Millisecond)
}

func TestApprovalNotificationRequestsAuthorizationAtMostOnce(t *testing.T) {
	t.Parallel()
	fake := &fakeApprovalNativeNotifier{requestAuthorized: true}
	service := newApprovalNotificationService(t.Context(), fake)
	service.launchID = approvalNotificationTestLaunchID
	service.launchIDErr = nil
	require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))
	t.Cleanup(func() { require.NoError(t, service.ServiceShutdown()) })
	for revision := uint64(1); revision <= 20; revision++ {
		service.observeCoordinationState(commandApprovalState(revision, fmt.Sprintf("request-%d", revision), domain.CommandApprovalModeOrdinary))
	}
	require.Eventually(t, func() bool { return fake.snapshot().requestCalls == 1 }, time.Second, time.Millisecond)
	assert.Equal(t, 1, fake.snapshot().checkCalls)
}

func TestApprovalNotificationRetainsLatestPendingRequestDuringAuthorization(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	fake := &fakeApprovalNativeNotifier{
		requestAuthorized: true, requestEntered: entered, requestRelease: release,
	}
	service := newApprovalNotificationService(t.Context(), fake)
	service.launchID = approvalNotificationTestLaunchID
	service.launchIDErr = nil
	require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))
	t.Cleanup(func() { require.NoError(t, service.ServiceShutdown()) })
	require.Eventually(t, func() bool { return len(entered) == 1 }, time.Second, time.Millisecond)
	<-entered

	service.observeCoordinationState(commandApprovalState(1, "during-consent-old", domain.CommandApprovalModeOrdinary))
	service.observeCoordinationState(navigationApprovalState(2, "during-consent-latest"))
	require.Empty(t, fake.snapshot().notifications)
	require.Equal(t, 1, fake.snapshot().requestCalls)
	releaseOnce.Do(func() { close(release) })
	require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	require.Equal(t,
		approvalNotificationID(approvalNotificationTestLaunchID, approvalRequestTerminalNavigation, "during-consent-latest"),
		fake.snapshot().notifications[0].ID,
	)
}

func TestApprovalNotificationShutdownIgnoresLateAuthorizationAndCapturedResponse(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	fake := &fakeApprovalNativeNotifier{
		requestAuthorized: true, requestEntered: entered, requestRelease: release,
	}
	target := &fakeApprovalTarget{succeed: true}
	service := newApprovalNotificationService(t.Context(), fake)
	service.launchID = approvalNotificationTestLaunchID
	service.launchIDErr = nil
	service.bind(target)
	require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))
	t.Cleanup(func() { require.NoError(t, service.ServiceShutdown()) })
	service.observeCoordinationState(commandApprovalState(1, "shutdown-pending", domain.CommandApprovalModeOrdinary))
	require.Eventually(t, func() bool { return len(entered) == 1 }, time.Second, time.Millisecond)
	<-entered
	callback := fake.capturedCallback()
	require.NotNil(t, callback)

	started := time.Now()
	require.NoError(t, service.ServiceShutdown())
	require.Less(t, time.Since(started), 100*time.Millisecond, "shutdown waited for the authorization dialog")
	callback(wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
		ID:         approvalNotificationID(approvalNotificationTestLaunchID, approvalRequestCommandExecution, "shutdown-pending"),
		CategoryID: approvalNotificationCategoryID, ActionIdentifier: approvalNotificationApproveID,
	}})
	require.Empty(t, target.snapshot())
	releaseOnce.Do(func() { close(release) })
	require.Eventually(t, func() bool {
		snapshot := fake.snapshot()
		return snapshot.shutdownCalls == 1 && len(snapshot.pendingRemovals) == 1 && len(snapshot.deliveredRemovals) == 1
	}, time.Second, time.Millisecond)
	require.Empty(t, fake.snapshot().notifications, "late authorization delivered after shutdown")
}

func TestApprovalNotificationShutdownDiscardsQueuedNativeWork(t *testing.T) {
	t.Parallel()

	t.Run("cleanup", func(t *testing.T) {
		fake := &fakeApprovalNativeNotifier{}
		service := startReadyApprovalNotifications(t, fake)
		service.mu.Lock()
		generation := service.generation
		service.mu.Unlock()

		service.nativeMu.Lock()
		started := make(chan struct{})
		done := make(chan struct{})
		go func() {
			close(started)
			service.cleanup("queued-cleanup", generation, false)
			close(done)
		}()
		<-started
		require.NoError(t, service.ServiceShutdown())
		service.nativeMu.Unlock()

		require.Eventually(t, func() bool {
			select {
			case <-done:
				return fake.snapshot().shutdownCalls == 1
			default:
				return false
			}
		}, time.Second, time.Millisecond)
		require.Empty(t, fake.snapshot().pendingRemovals)
		require.Empty(t, fake.snapshot().deliveredRemovals)
	})

	t.Run("send", func(t *testing.T) {
		fake := &fakeApprovalNativeNotifier{}
		service := startReadyApprovalNotifications(t, fake)
		request := approvalNotificationRequest{
			kind: approvalRequestCommandExecution, requestID: "queued-send",
			notificationID: approvalNotificationID(approvalNotificationTestLaunchID, approvalRequestCommandExecution, "queued-send"),
			body:           "queued send",
		}
		service.mu.Lock()
		generation := service.generation
		service.current = &currentApprovalNotification{request: request, delivery: approvalDeliveryAttempting}
		service.mu.Unlock()

		service.nativeMu.Lock()
		started := make(chan struct{})
		done := make(chan struct{})
		go func() {
			close(started)
			service.applyTransition("", &request, generation)
			close(done)
		}()
		<-started
		require.NoError(t, service.ServiceShutdown())
		service.nativeMu.Unlock()

		require.Eventually(t, func() bool {
			select {
			case <-done:
				return fake.snapshot().shutdownCalls == 1
			default:
				return false
			}
		}, time.Second, time.Millisecond)
		require.Empty(t, fake.snapshot().notifications)
	})
}

func TestApprovalNotificationRevokedDeliveryFailsClosed(t *testing.T) {
	t.Parallel()
	fake := &fakeApprovalNativeNotifier{}
	target := &fakeApprovalTarget{succeed: true}
	service := startReadyApprovalNotifications(t, fake)
	service.bind(target)
	fake.setAuthorization(false, nil)
	fake.setSendError(errors.New("notification permission revoked"))
	service.observeCoordinationState(commandApprovalState(1, "revoked", domain.CommandApprovalModeOrdinary))
	require.Eventually(t, func() bool {
		snapshot := fake.snapshot()
		return len(snapshot.notifications) == 1 && snapshot.checkCalls == 2
	}, time.Second, time.Millisecond)
	require.Empty(t, target.snapshot())
	service.mu.Lock()
	require.NotNil(t, service.current)
	require.Equal(t, approvalDeliveryFailed, service.current.delivery)
	require.False(t, service.current.decisionPending)
	require.Equal(t, approvalNotificationsUnavailable, service.availability)
	service.mu.Unlock()

	service.observeCoordinationState(commandApprovalState(2, "after-revocation", domain.CommandApprovalModeOrdinary))
	require.Never(t, func() bool {
		snapshot := fake.snapshot()
		return len(snapshot.notifications) != 1 || snapshot.checkCalls != 2 || snapshot.requestCalls != 0
	}, 20*time.Millisecond, time.Millisecond)
	require.Empty(t, target.snapshot())
}

func TestApprovalNotificationContentEdgeCases(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		state    *domain.MasterCoordinationState
		wantBody string
	}{
		{
			name: "blank command name falls back to command ID",
			state: &domain.MasterCoordinationState{PendingCommandExecution: &domain.MasterPendingCommandExecution{
				RequestID: "blank-command-name", CommandID: "command-fallback", ConfirmationText: "  ПЕРВАЯ СТРОКА\nВТОРАЯ СТРОКА  ",
			}},
			wantBody: "КОМАНДА: command-fallback\nПЕРВАЯ СТРОКА\nВТОРАЯ СТРОКА",
		},
		{
			name: "blank navigation names fall back to stable IDs",
			state: &domain.MasterCoordinationState{PendingTerminalNavigation: &domain.MasterPendingTerminalNavigation{
				RequestID: "blank-navigation-names", CommandID: "navigate-fallback",
				SourceTerminalID: "source-fallback", TargetTerminalID: "target-fallback",
			}},
			wantBody: "КОМАНДА: navigate-fallback\nsource-fallback → target-fallback",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request, valid := approvalRequestFromState(test.state)
			require.True(t, valid)
			require.NotNil(t, request)
			require.Equal(t, test.wantBody, request.body)
		})
	}

	longState := commandApprovalState(1, "unicode-long", domain.CommandApprovalModeOrdinary)
	longState.PendingCommandExecution.ConfirmationText = "МНОГОСТРОЧНОЕ ПОДТВЕРЖДЕНИЕ\n" + strings.Repeat("Ж", approvalNotificationBodyLimit)
	longState.Sessions = []domain.MasterSessionEntry{{FallbackName: "PRIVATE PLAYER NAME"}}
	request, valid := approvalRequestFromState(longState)
	require.True(t, valid)
	require.NotNil(t, request)
	require.Len(t, []rune(request.body), approvalNotificationBodyLimit)
	require.True(t, strings.HasPrefix(request.body, "КОМАНДА: ОТКРЫТЬ УБЕЖИЩЕ\nМНОГОСТРОЧНОЕ ПОДТВЕРЖДЕНИЕ\n"))
	require.NotContains(t, request.body, "PRIVATE PLAYER NAME")
	options := approvalNotificationOptions(*request)
	require.Nil(t, options.Data)
	require.Nil(t, options.Attachments)
	require.Nil(t, options.Schedule)
	require.Empty(t, options.Subtitle)
	require.Empty(t, options.ThreadID)
}

func TestApprovalNotificationAmbiguousPendingStateFailsClosed(t *testing.T) {
	t.Parallel()
	fake := &fakeApprovalNativeNotifier{}
	target := &fakeApprovalTarget{succeed: true}
	service := startReadyApprovalNotifications(t, fake)
	service.bind(target)
	state := commandApprovalState(1, "ambiguous-command", domain.CommandApprovalModeOrdinary)
	service.observeCoordinationState(state)
	require.Eventually(t, func() bool { return len(fake.snapshot().notifications) == 1 }, time.Second, time.Millisecond)
	option := fake.snapshot().notifications[0]
	state.PendingTerminalNavigation = navigationApprovalState(2, "ambiguous-navigation").PendingTerminalNavigation

	service.observeCoordinationState(state)
	require.Eventually(t, func() bool {
		return len(fake.snapshot().pendingRemovals) == 1 && len(fake.snapshot().deliveredRemovals) == 1
	}, time.Second, time.Millisecond)
	require.Len(t, fake.snapshot().notifications, 1, "ambiguous state delivered another notification")
	service.mu.Lock()
	require.Nil(t, service.current)
	service.mu.Unlock()
	fake.respond(wailsnotifications.NotificationResult{Response: wailsnotifications.NotificationResponse{
		ID: option.ID, CategoryID: option.CategoryID, ActionIdentifier: approvalNotificationApproveID,
	}})
	require.Empty(t, target.snapshot())
}
