package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	sessionservice "github.com/obalunenko/Fallout-Terminal/v2/internal/session"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	tunnelservice "github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	updateservice "github.com/obalunenko/Fallout-Terminal/v2/internal/update"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func TestApplicationUpdateRecoveryFailureDoesNotInitializeUpdateInfrastructure(t *testing.T) {
	t.Parallel()

	failure := updateservice.UpdateFailure{
		Stage:          updateservice.FailureStageRecovery,
		Message:        "The previous application update did not finish applying.",
		RecoveryAction: "Continue using the installed application and try again later.",
	}
	manager, err := newApplicationUpdateManager(
		nil,
		true,
		"2.4.0",
		"/private/application-update-recovery.json",
		&failure,
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, updateservice.UpdateSnapshot{
		Revision:         1,
		State:            updateservice.UpdateStateFailed,
		InstalledVersion: "2.4.0",
		Failure:          failure,
	}, manager.Status(t.Context()))
}

func TestWailsV3ApplicationOptionsServeOnlyOverseerAssetsAndQuitWithLastWindow(t *testing.T) {
	t.Parallel()

	assets := fstest.MapFS{
		"index.html":  {Data: []byte("<!doctype html><title>OVERSEER</title>")},
		"overseer.js": {Data: []byte("export const overseer = true;")},
	}
	options := wailsApplicationOptions(assets, func(application.SecondInstanceData) {})
	require.Equal(t, "Fallout Terminal", options.Name)
	require.Equal(t, "Fallout Terminal — Overseer Control", options.Description)
	require.Equal(t, "FalloutTerminalWindow", options.Windows.WndClass)
	require.Equal(t, "fallout-terminal", options.Linux.ProgramName)
	require.False(t, options.Windows.DisableQuitOnLastWindowClosed)
	require.False(t, options.Linux.DisableQuitOnLastWindowClosed)
	require.True(t, options.DisableDefaultSignalHandler, "signal.NotifyContext is the sole production signal owner")
	require.True(t, options.Mac.ApplicationShouldTerminateAfterLastWindowClosed)
	requirePNGIcon(t, options.Icon)
	require.Equal(t, options.Icon, overseerWindowOptions().Linux.Icon, "Linux window and application identity must use the same icon")
	require.NotNil(t, options.Assets.Handler)
	require.Empty(t, options.Services, "services are registered only after core composition")

	request := httptest.NewRequest("GET", "http://wails.localhost/", nil)
	response := httptest.NewRecorder()
	options.Assets.Handler.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	result := response.Result()
	t.Cleanup(func() { require.NoError(t, result.Body.Close()) })
	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "OVERSEER")
	assert.NotContains(t, string(body), "characterSelect", "the private Wails asset host must not serve the public player application")
}

func TestWailsApplicationOptionsEnableSingleInstanceLaunches(t *testing.T) {
	t.Parallel()

	var received application.SecondInstanceData
	options := wailsApplicationOptions(fstest.MapFS{}, func(data application.SecondInstanceData) {
		received = data
	})
	require.NotNil(t, options.SingleInstance)
	require.Equal(t, "com.vaulttec.fallout-terminal", options.SingleInstance.UniqueID)
	require.Zero(t, options.SingleInstance.ExitCode)
	require.Nil(t, options.SingleInstance.AdditionalData)
	require.NotZero(t, options.SingleInstance.EncryptionKey)
	require.NotNil(t, options.SingleInstance.OnSecondInstanceLaunch)

	launch := application.SecondInstanceData{
		Args:           []string{"Fallout Terminal", "--untrusted"},
		WorkingDir:     "/untrusted",
		AdditionalData: map[string]string{"untrusted": "value"},
	}
	options.SingleInstance.OnSecondInstanceLaunch(launch)
	require.Equal(t, launch, received)
}

func TestWailsSaveSessionBindingRetainsBothRealDemoTerminals(t *testing.T) {
	raw, err := os.ReadFile("sessions/demo.json")
	require.NoError(t, err)
	target := "/Campaigns/wails-demo-transition.json"
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, raw)
	sessions := sessionservice.NewService(
		sessionservice.NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		sessionservice.Locations{
			DocumentsDefault: "/Campaigns",
			BundledDemo:      "/Applications/Fallout Terminal.app/Contents/Resources/sessions/demo.json",
		},
	)
	t.Cleanup(func() { require.NoError(t, sessions.Shutdown(context.WithoutCancel(t.Context()))) })
	core := NewAppWithDependencies(t.Context(), AppDependencies{Sessions: sessions})
	require.True(t, core.OpenSession().OK)

	_ = application.New(application.Options{})
	bindings := application.NewBindings(nil, nil)
	require.NoError(t, bindings.Add(application.NewService(newDesktopService(core))))
	method := bindings.Get(&application.CallOptions{
		MethodName: "github.com/obalunenko/Fallout-Terminal/v2.desktopService.SaveSession",
	})
	require.NotNil(t, method, "SaveSession binding must resolve")
	result, err := method.Call(t.Context(), []json.RawMessage{json.RawMessage(raw)})
	require.NoError(t, err)
	saved, ok := result.(sessionservice.SaveResult)
	require.True(t, ok, "SaveSession result type = %T", result)
	require.True(t, saved.OK, "SaveSession binding result = %#v", saved)

	reopened := core.OpenSession()
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.Len(t, reopened.Session.Terminals, 2)
	require.Equal(t, "t_demo2", reopened.Session.Terminals[0].Root.Children[4].TerminalTransition.TargetTerminalID)
}

func TestOverseerWindowOptionsPreserveAcceptedSingleWindowContract(t *testing.T) {
	t.Parallel()

	options := overseerWindowOptions()
	require.Equal(t, "Fallout Terminal — Overseer Control", options.Title)
	require.Equal(t, 1200, options.Width)
	require.Equal(t, 780, options.Height)
	require.Equal(t, 900, options.MinWidth)
	require.Equal(t, 600, options.MinHeight)
	require.Equal(t, "/", options.URL)
	require.Equal(t, application.NewRGB(11, 13, 10), options.BackgroundColour)
	require.False(t, options.AllowSimpleEventEmit)
	require.False(t, options.Windows.DisableIcon)
	requirePNGIcon(t, options.Linux.Icon)
}

func TestOverseerWindowActivationRestoresAndFocusesReadyWindow(t *testing.T) {
	t.Parallel()

	target := &recordingOverseerWindowActivator{}
	activation := &overseerWindowActivation{}
	activation.bind(target)

	launch := application.SecondInstanceData{
		Args:           []string{"Fallout Terminal", "--untrusted"},
		WorkingDir:     "/untrusted",
		AdditionalData: map[string]string{"untrusted": "value"},
	}
	for range 3 {
		activation.handleSecondInstanceLaunch(launch)
	}

	require.Equal(t, []string{"restore", "focus", "restore", "focus", "restore", "focus"}, target.callsSnapshot())
}

func TestOverseerWindowActivationCoalescesLaunchesUntilWindowBinding(t *testing.T) {
	t.Parallel()

	activation := &overseerWindowActivation{}
	var launches sync.WaitGroup
	for range 64 {
		launches.Add(1)
		go func() {
			defer launches.Done()
			activation.handleSecondInstanceLaunch(application.SecondInstanceData{})
		}()
	}
	launches.Wait()

	target := &recordingOverseerWindowActivator{}
	activation.bind(target)
	require.Equal(t, []string{"restore", "focus"}, target.callsSnapshot())
}

func TestOverseerWindowCloseRegistersOnlyTheCurrentPlatformFallbackAndQuitsExactlyOnce(t *testing.T) {
	t.Parallel()

	window := &recordingOverseerWindowCloseRegistrar{}
	quit := &blockingApplicationQuitter{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	t.Cleanup(quit.unblock)
	registerOverseerWindowQuitOnClose(window, quit)

	require.Equal(t, []events.WindowEventType{events.Common.WindowClosing}, window.hookEventTypes)
	require.Len(t, window.hookCallbacks, 1)
	require.NotNil(t, window.hookCallbacks[0])

	firstCallback := window.hookCallbacks[0]
	if runtime.GOOS == "darwin" {
		require.Equal(t, []events.WindowEventType{events.Mac.WindowWillClose}, window.nativeEventTypes)
		require.Len(t, window.nativeCallbacks, 1)
		require.NotNil(t, window.nativeCallbacks[0])
		firstCallback = window.nativeCallbacks[0]
	} else {
		require.Empty(t, window.nativeEventTypes, "Windows and Linux must not register a Darwin event fallback")
		require.Empty(t, window.nativeCallbacks, "Windows and Linux use the portable closing hook only")
	}

	callbackDone := make(chan struct{})
	go func() {
		firstCallback(&application.WindowEvent{})
		close(callbackDone)
	}()
	require.Eventually(t, func() bool { return len(quit.entered) == 1 }, time.Second, time.Millisecond)
	select {
	case <-callbackDone:
		require.Fail(t, "native close returned before application termination completed")
	default:
	}
	quit.unblock()
	require.Eventually(t, func() bool {
		select {
		case <-callbackDone:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
	window.hookCallbacks[0](&application.WindowEvent{})
	if runtime.GOOS == "darwin" {
		window.nativeCallbacks[0](&application.WindowEvent{})
	}
	require.Len(t, quit.entered, 1)
}

func TestWailsLifecycleStartupClassifiesApplicationFailuresAsStatusVisible(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		startErr error
	}{
		{name: "success"},
		{name: "application failure remains presentable", startErr: errors.New("listener occupied")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			core := &recordingWailsLifecycleCore{startErr: test.startErr}
			service := newWailsLifecycleService(t.Context(), core, nil)
			ctx := context.WithValue(t.Context(), lifecycleContextKey{}, "application-lifetime")

			require.NoError(t, service.ServiceStartup(ctx, application.ServiceOptions{}))
			require.Equal(t, 1, core.startCalls)
			require.NotSame(t, ctx, core.startContext)
			require.Equal(t, "application-lifetime", core.startContext.Value(lifecycleContextKey{}))
			require.NoError(t, core.startContext.Err())
		})
	}
}

func TestWailsLifecycleUpdateCheckFailureNeverChangesLocalStartupStatus(t *testing.T) {
	recorder := &callRecorder{}
	checker := newApplicationUpdateCheckProbe(t, recorder, errors.New("release endpoint unavailable"))
	manager, err := updateservice.NewManager(updateservice.ManagerConfig{
		InstalledVersion: "2.4.0",
		Packaged:         true,
		Check:            checker.Check,
		IDs:              func() string { return "wails-startup-check" },
	})
	require.NoError(t, err)
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Player: &recordingPlayerServer{recorder: recorder, info: domain.ServerInfo{
			IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690",
		}},
		Events:  &recordingEventSink{recorder: recorder},
		Desktop: &recordingDesktop{recorder: recorder},
		Updates: manager,
	})
	service := newWailsLifecycleService(t.Context(), app, nil)

	require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))
	t.Cleanup(func() {
		require.NoError(t, service.ServiceShutdown())
	})
	assert.Never(t, checker.Started, 25*time.Millisecond, time.Millisecond,
		"Wails startup itself must not arm an external update check")
	require.Equal(t, "ready-local", app.lifecyclePhase())
	require.Empty(t, app.GetRuntimeStatus().StartupError)

	started := time.Now()
	newDesktopService(app).GetApplicationUpdateStatus()
	assert.Less(t, time.Since(started), 100*time.Millisecond)
	checker.WaitForStart(t)
	checker.Release()
	require.Eventually(t, func() bool {
		return manager.Snapshot().State == updateservice.UpdateStateFailed
	}, time.Second, time.Millisecond)

	status := app.GetRuntimeStatus()
	assert.Empty(t, status.StartupError)
	assert.NotNil(t, status.ServerInfo)
	assert.Equal(t, "ready-local", app.lifecyclePhase())
	assert.Equal(t, 0, countRecordedCall(recorder.Calls(), "player:stop"))
}

func TestWailsLifecycleRecordsAbsorbedStartupFailureExactlyOnce(t *testing.T) {
	t.Parallel()

	startErr := errors.New("PROVIDER-SECRET-CANARY")
	core := &recordingWailsLifecycleCore{startErr: startErr}
	logs := testutil.NewRecordingLogger()
	service := newWailsLifecycleService(t.Context(), core, nil, logs)

	require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))

	records := logs.Records()
	require.Len(t, records, 1)
	record := requireLogRecord(t, records, "application startup failed")
	require.Equal(t, "error", record.Level)
	require.Equal(t, "application.start", record.Fields["operation"])
	require.Equal(t, "failed", record.Fields["outcome"])
	require.NotContains(t, fmt.Sprintf("%#v", records), startErr.Error())
}

func TestWailsLifecycleShutdownUsesFreshBoundedContext(t *testing.T) {
	t.Parallel()

	root := context.WithValue(t.Context(), lifecycleContextKey{}, "process-root")
	startupContext, cancelStartup := context.WithCancelCause(t.Context())
	core := &recordingWailsLifecycleCore{}
	service := newWailsLifecycleService(root, core, nil)
	require.NoError(t, service.ServiceStartup(startupContext, application.ServiceOptions{}))
	cancelStartup(errors.New("test startup context closed"))

	started := time.Now()
	require.NoError(t, service.ServiceShutdown())
	require.Equal(t, 1, core.shutdownCalls)
	require.NoError(t, core.shutdownContextErr)
	require.True(t, core.shutdownHasDeadline)
	require.WithinDuration(t, started.Add(wailsShutdownTimeout), core.shutdownDeadline, time.Second)
	require.Equal(t, "process-root", core.shutdownContext.Value(lifecycleContextKey{}))
	require.ErrorIs(t, context.Cause(core.shutdownContext), errWailsShutdownComplete)
}

func TestWailsLifecycleRootCancellationStopsRuntimeAndRequestsHostQuit(t *testing.T) {
	root, cancelRoot := context.WithCancelCause(t.Context())
	core := &recordingWailsLifecycleCore{}
	quit := make(chan struct{})
	var quitOnce sync.Once
	service := newWailsLifecycleService(root, core, func() {
		quitOnce.Do(func() { close(quit) })
	})
	startupContext := context.WithValue(t.Context(), lifecycleContextKey{}, "wails-runtime")
	require.NoError(t, service.ServiceStartup(startupContext, application.ServiceOptions{}))

	shutdownCause := errors.New("test process signal received")
	cancelRoot(shutdownCause)
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("root cancellation did not request Wails host shutdown")
	}
	require.Equal(t, "wails-runtime", core.startContext.Value(lifecycleContextKey{}))
	require.ErrorIs(t, context.Cause(core.startContext), shutdownCause)
	require.NoError(t, service.ServiceShutdown())
}

func TestWailsLifecyclePartialStartupUnwindsOnceAndRepeatedShutdownIsSafe(t *testing.T) {
	t.Parallel()

	recorder := &callRecorder{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Player: &recordingPlayerServer{recorder: recorder, info: domain.ServerInfo{
			IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690",
		}},
		Events: &recordingEventSink{recorder: recorder, err: errors.New("desktop event bridge unavailable")},
	})
	service := newWailsLifecycleService(t.Context(), app, nil)

	require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))
	require.Equal(t, "failed", app.lifecyclePhase())
	require.Contains(t, app.GetRuntimeStatus().StartupError, "desktop event bridge unavailable")
	require.Equal(t, []string{"player:start", "event:server-info", "player:stop"}, recorder.Calls())

	require.NoError(t, service.ServiceShutdown())
	require.NoError(t, service.ServiceShutdown())
	require.Equal(t, []string{"player:start", "event:server-info", "player:stop"}, recorder.Calls())
}

func TestWailsLifecycleRepeatedQuitRetriesFailedPublicCleanupWithFreshFiveSecondContexts(t *testing.T) {
	recorder := &callRecorder{}
	preferences := tunnelservice.DefaultPublicAccessPreferences()
	core := &recordingPublicAccessCore{
		recorder: recorder,
		snapshot: tunnelservice.PublicAccessSnapshot{
			Preferences: preferences,
			Status:      tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled},
		},
		shutdownErrors: []error{errors.New("synthetic first cleanup failure"), nil},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		PublicAccess: core,
		Player: &recordingPlayerServer{recorder: recorder, info: domain.ServerInfo{
			URL: "http://127.0.0.1:3690", LocalURL: "http://127.0.0.1:3690", Port: 3690,
		}},
		Events: &recordingEventSink{recorder: recorder},
	})
	service := newWailsLifecycleService(t.Context(), app, nil)
	require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))

	started := time.Now()
	require.Error(t, service.ServiceShutdown())
	require.NoError(t, service.ServiceShutdown())
	assert.Equal(t, 2, core.shutdowns)
	assert.WithinDuration(t, started, time.Now(), wailsShutdownTimeout)
	assert.Equal(t, 1, countRecordedCall(recorder.Calls(), "player:stop"))
}

func TestWailsEventSinkUsesInjectedManagerForExactTypedEventNames(t *testing.T) {
	t.Parallel()

	emitter := &recordingWailsEventEmitter{}
	sink := newWailsEventSink(emitter)
	payloads := []struct {
		name    string
		payload any
	}{
		{serverInfoEvent, domain.ServerInfo{URL: "http://127.0.0.1:3690"}},
		{clientCountEvent, 4},
		{hackStateEvent, &domain.PublicHackState{AttemptsLeft: 2}},
		{coordinationStateEvent, &domain.MasterCoordinationState{Revision: 7}},
		{sessionStateEvent, SessionStateEvent{Revision: 8}},
		{publicAccessStatusEvent, PublicAccessSnapshot{Status: PublicAccessStatus{State: "disabled"}}},
		{applicationUpdateStatusEvent, ApplicationUpdateSnapshot{Revision: 9, State: "checking"}},
	}
	for _, payload := range payloads {
		require.NoError(t, sink.Emit(payload.name, payload.payload))
	}
	require.Equal(t, []string{
		"server-info", "client-count", "hack-state", "coordination-state", "session-state",
		"public-access-status", "application-update-status",
	}, emitter.names)
	for index := range payloads {
		require.Equal(t, payloads[index].payload, emitter.payloads[index])
	}
	require.Error(t, newWailsEventSink(nil).Emit(serverInfoEvent, domain.ServerInfo{}))
}

func TestWailsServicesRegisterApprovalNotificationsBeforeCoreLifecycle(t *testing.T) {
	t.Parallel()
	host := &recordingWailsServiceRegistrar{}
	notifications := newApprovalNotificationService(t.Context(), &fakeApprovalNativeNotifier{})
	core := NewAppWithDependencies(t.Context(), AppDependencies{CoordinationObserver: notifications})

	registerWailsServices(t.Context(), host, core)

	require.Len(t, host.services, 3)
	_, notificationOK := host.services[0].Instance().(*approvalNotificationService)
	_, lifecycleOK := host.services[1].Instance().(*wailsLifecycleService)
	_, desktopOK := host.services[2].Instance().(*desktopService)
	require.True(t, notificationOK)
	require.True(t, lifecycleOK)
	require.True(t, desktopOK)
}

func TestApprovalNotificationLifecycleFailuresDoNotAbortWailsStartup(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		fake *fakeApprovalNativeNotifier
	}{
		{name: "startup failure", fake: &fakeApprovalNativeNotifier{startupErr: errors.New("unavailable")}},
		{name: "category failure", fake: &fakeApprovalNativeNotifier{categoryErr: errors.New("unsupported")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := newApprovalNotificationService(t.Context(), test.fake)
			require.NoError(t, service.ServiceStartup(t.Context(), application.ServiceOptions{}))
			require.NoError(t, service.ServiceShutdown())
			require.Eventually(t, func() bool { return test.fake.snapshot().shutdownCalls == 1 }, time.Second, time.Millisecond)
		})
	}
}

type recordingWailsServiceRegistrar struct {
	services []application.Service
}

func (host *recordingWailsServiceRegistrar) RegisterService(service application.Service) {
	host.services = append(host.services, service)
}

func (*recordingWailsServiceRegistrar) Quit() {}

type lifecycleContextKey struct{}

func requirePNGIcon(t *testing.T, icon []byte) {
	t.Helper()

	pngSignature := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	require.GreaterOrEqual(t, len(icon), len(pngSignature), "application icon must contain a complete PNG signature")
	require.Equal(t, pngSignature, icon[:len(pngSignature)])
}

type recordingOverseerWindowCloseRegistrar struct {
	hookEventTypes   []events.WindowEventType
	hookCallbacks    []func(*application.WindowEvent)
	nativeEventTypes []events.WindowEventType
	nativeCallbacks  []func(*application.WindowEvent)
}

type recordingOverseerWindowActivator struct {
	mu    sync.Mutex
	calls []string
}

func (window *recordingOverseerWindowActivator) Restore() {
	window.mu.Lock()
	defer window.mu.Unlock()
	window.calls = append(window.calls, "restore")
}

func (window *recordingOverseerWindowActivator) Focus() {
	window.mu.Lock()
	defer window.mu.Unlock()
	window.calls = append(window.calls, "focus")
}

func (window *recordingOverseerWindowActivator) callsSnapshot() []string {
	window.mu.Lock()
	defer window.mu.Unlock()
	return append([]string(nil), window.calls...)
}

func (registrar *recordingOverseerWindowCloseRegistrar) RegisterHook(
	eventType events.WindowEventType,
	callback func(*application.WindowEvent),
) func() {
	registrar.hookEventTypes = append(registrar.hookEventTypes, eventType)
	registrar.hookCallbacks = append(registrar.hookCallbacks, callback)
	return func() {}
}

func (registrar *recordingOverseerWindowCloseRegistrar) OnWindowEvent(
	eventType events.WindowEventType,
	callback func(*application.WindowEvent),
) func() {
	registrar.nativeEventTypes = append(registrar.nativeEventTypes, eventType)
	registrar.nativeCallbacks = append(registrar.nativeCallbacks, callback)
	return func() {}
}

type blockingApplicationQuitter struct {
	entered     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
}

func (quitter *blockingApplicationQuitter) Quit() {
	quitter.entered <- struct{}{}
	<-quitter.release
}

func (quitter *blockingApplicationQuitter) unblock() {
	quitter.releaseOnce.Do(func() { close(quitter.release) })
}

type recordingWailsEventEmitter struct {
	names    []string
	payloads []any
}

func (emitter *recordingWailsEventEmitter) Emit(name string, payloads ...any) bool {
	emitter.names = append(emitter.names, name)
	if len(payloads) == 1 {
		emitter.payloads = append(emitter.payloads, payloads[0])
	} else {
		emitter.payloads = append(emitter.payloads, payloads)
	}
	return false
}

type recordingWailsLifecycleCore struct {
	startErr            error
	shutdownErr         error
	startCalls          int
	shutdownCalls       int
	startContext        context.Context
	shutdownContext     context.Context
	shutdownContextErr  error
	shutdownDeadline    time.Time
	shutdownHasDeadline bool
}

func (core *recordingWailsLifecycleCore) Start(ctx context.Context) error {
	core.startCalls++
	core.startContext = ctx
	return core.startErr
}

func (core *recordingWailsLifecycleCore) Shutdown(ctx context.Context) error {
	core.shutdownCalls++
	core.shutdownContext = ctx
	core.shutdownContextErr = ctx.Err()
	core.shutdownDeadline, core.shutdownHasDeadline = ctx.Deadline()
	return core.shutdownErr
}
