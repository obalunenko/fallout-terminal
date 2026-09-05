package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailsnotifications "github.com/wailsapp/wails/v3/pkg/services/notifications"

	controlservice "github.com/obalunenko/Fallout-Terminal/v2/internal/control"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/diagnostics"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	configv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/config/v1"
	liveservice "github.com/obalunenko/Fallout-Terminal/v2/internal/live"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/platform"
	playerserver "github.com/obalunenko/Fallout-Terminal/v2/internal/player"
	playerconfigservice "github.com/obalunenko/Fallout-Terminal/v2/internal/playerconfig"
	sessionservice "github.com/obalunenko/Fallout-Terminal/v2/internal/session"
	tunnelservice "github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	updateservice "github.com/obalunenko/Fallout-Terminal/v2/internal/update"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/version"
	"github.com/obalunenko/logger"
)

// The repository-owned Go build command prepares the frontend before production
// compilation. The checked-in .keep keeps ordinary Go tooling compile-safe on a
// clean checkout.
//
//go:embed all:frontend/overseer/dist
var overseerSource embed.FS

// The player remains a distinct browser application but is owned and served
// by the same Go process.
//
//go:embed all:frontend/client/dist
var clientSource embed.FS

var errApplicationProcessComplete = errors.New("application process complete")

func main() {
	if handled, err := updateservice.RunReplacementHelperFromEnvironment(context.Background(), os.LookupEnv); handled {
		if err != nil {
			os.Exit(1)
		}
		return
	}
	exitCode := runMain(os.Args[1:], os.Stdout, os.Stderr, runApplication)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func runMain(arguments []string, stdout, stderr io.Writer, startApplication func()) int {
	if len(arguments) == 0 || arguments[0] != "--version" {
		startApplication()
		return 0
	}
	if len(arguments) != 1 {
		if _, err := fmt.Fprintln(stderr, "usage: Fallout Terminal --version"); err != nil {
			return 1
		}
		return 2
	}
	if _, err := fmt.Fprintln(stdout, version.Current()); err != nil {
		return 1
	}
	return 0
}

// newApplicationLogger composes the retained writer with the one production
// logger. The writer owns reporting storage degradation so initialization
// failures cannot produce duplicate warnings through the fallback.
func newApplicationLogger(ctx context.Context, logDirectory string, fallback io.Writer) (*diagnostics.RunLog, logger.Logger) {
	retainedLog, _ := diagnostics.Open(diagnostics.Options{Directory: logDirectory, Fallback: fallback})
	applicationLogger := logger.Init(ctx, logger.Params{Level: "info", Format: "text", Writer: retainedLog}).WithField("run_id", retainedLog.RunID())
	return retainedLog, applicationLogger
}

func runApplication() {
	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	rootContext, cancelRoot := context.WithCancelCause(signalContext)
	defer stopSignals()
	defer cancelRoot(errApplicationProcessComplete)
	stopSignalDiversion := context.AfterFunc(rootContext, stopSignals)
	defer stopSignalDiversion()
	locations, locationsErr := platform.DefaultSessionLocations(applicationResourceRoot())
	logDirectory := ""
	if locationsErr == nil {
		logDirectory, locationsErr = platform.ApplicationLogDirectory(locations.ApplicationSupport)
	}
	retainedLog, applicationLogger := newApplicationLogger(rootContext, logDirectory, os.Stderr)
	defer func() { _ = retainedLog.Close() }()
	rootContext = logger.ContextWithLogger(rootContext, applicationLogger)
	if locationsErr != nil {
		logger.WithField(rootContext, "error_category", "location_unavailable").Fatal("resolve application locations")
	}

	overseerAssets, err := fs.Sub(overseerSource, "frontend/overseer/dist")
	if err != nil {
		logger.WithField(rootContext, "error_category", "asset_unavailable").Fatal("prepare Overseer assets")
	}
	clientAssets, err := fs.Sub(clientSource, "frontend/client/dist")
	if err != nil {
		logger.WithField(rootContext, "error_category", "asset_unavailable").Fatal("prepare player assets")
	}

	windowActivation := &overseerWindowActivation{}
	host := newWailsApplication(overseerAssets, windowActivation.handleSecondInstanceLaunch)
	core, err := composeApplication(rootContext, host, clientAssets, locations, retainedLog)
	if err != nil {
		logger.WithField(rootContext, "error_category", "composition_failed").Fatal("compose application")
	}
	registerWailsServices(rootContext, host, core)
	windowActivation.bind(newOverseerWindow(host))
	if err := host.Run(); err != nil {
		logger.WithField(rootContext, "operation", "application.run").Fatal("application host stopped with an error")
	}
}

func composeApplication(ctx context.Context, host *application.App, clientAssets fs.FS, locations platform.SessionLocations, retainedLog *diagnostics.RunLog) (*App, error) {
	if ctx == nil {
		return nil, errors.New("application composition context is required")
	}
	if err := validateProductionResources(clientAssets, locations.BundledDemo); err != nil {
		return nil, err
	}
	publicAccessSettingsPath, err := platform.PublicAccessSettingsPath(locations.ApplicationSupport)
	if err != nil {
		return nil, fmt.Errorf("resolve public-access settings path: %w", err)
	}
	runtimeConfig := defaultApplicationConfig(locations)
	desktop := platform.NewDesktop(ctx, host.Dialog, host.Browser)
	events := newWailsEventSink(host.Event)
	live := liveservice.New(nil, nil)
	playerConfigs := playerconfigservice.NewService(
		playerconfigservice.NewStorage(nil), desktop, locations.DocumentsDefault,
	)
	sessions := sessionservice.NewService(
		sessionservice.NewStorage(nil),
		desktop,
		sessionservice.Locations{
			DocumentsDefault:   locations.DocumentsDefault,
			BundledDemo:        locations.BundledDemo,
			ApplicationSupport: locations.ApplicationSupport,
		},
	)
	effectRouter := &coordinationEffectRouter{}
	sessionStore := &sessionCommandStateStore{service: sessions}
	coordination := controlservice.New(controlservice.Config{
		Enqueue:                effectRouter.Enqueue,
		Runtime:                live,
		Terminals:              live,
		TrustedHack:            live,
		RosterStore:            playerConfigs,
		CommandStateStore:      sessionStore,
		FacilityStore:          sessionStore,
		FacilityAuthoringStore: sessionStore,
		TerminalGroupStore:     sessionStore,
		TerminalCatalog:        sessions,
		RequestResultLimit:     int(runtimeConfig.Coordination.RequestResultLimit),
	})
	playerConfig := playerserver.Config{
		Address: runtimeConfig.PlayerServer.Address,
		Assets:  clientAssets,
		Logger:  logger.FromContext(ctx),
	}
	connectPlayer, err := playerserver.NewConnectService(playerserver.ConnectServiceConfig{
		Coordinator: coordination,
		Assets:      clientAssets,
		QueueSize:   int(runtimeConfig.PlayerServer.DeliveryQueueSize),
		OnClientCount: func(count int) {
			if app := effectRouter.App(); app != nil {
				app.updateClientCount(count)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("construct Connect player service: %w", err)
	}
	playerConfig.Connect = connectPlayer
	player, err := playerserver.NewServer(ctx, playerConfig)
	if err != nil {
		return nil, fmt.Errorf("construct player server: %w", err)
	}
	packaged := isPackagedApplication()
	applicationUpdateRecoveryPath, err := platform.ApplicationUpdateRecoveryPath(locations.ApplicationSupport)
	if err != nil {
		return nil, fmt.Errorf("resolve application update recovery path: %w", err)
	}
	var applicationUpdateRecovery updateservice.RecoveryOutcome
	if packaged {
		applicationUpdateRecovery = updateservice.ConsumeApplicationUpdateRecovery(
			ctx,
			applicationUpdateRecoveryPath,
			version.Current(),
		)
	}
	publicSettings := tunnelservice.NewPublicAccessSettingsStore(publicAccessSettingsPath, nil, nil)
	publicSecrets := platform.NewPlatformSecureCredentialStore(packaged)
	effectivePublicSettings, effectivePublicSecrets := publicAccessStoresForProfile(publicSettings, publicSecrets, packaged, os.LookupEnv)
	var app *App
	publicAccess, err := tunnelservice.NewPublicAccessManager(tunnelservice.ManagerConfig{
		Settings:    effectivePublicSettings,
		Secrets:     effectivePublicSecrets,
		Tunnel:      tunnelservice.NewEmbeddedNgrokService(),
		Ingress:     tunnelservice.NewPublicIngressFactory(),
		UpstreamURL: publicAccessCompositionRoute().UpstreamURL,
		Publish: func(snapshot tunnelservice.PublicAccessSnapshot) {
			if app != nil {
				app.acceptPublicAccessSnapshot(snapshot, true)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("construct embedded public access: %w", err)
	}
	updates, err := newApplicationUpdateManager(
		host,
		packaged,
		version.Current(),
		applicationUpdateRecoveryPath,
		applicationUpdateRecovery.Failure,
		func(snapshot updateservice.UpdateSnapshot) {
			if app != nil {
				app.publishApplicationUpdateSnapshot(snapshot)
			}
		},
	)
	if err != nil {
		return nil, fmt.Errorf("construct application update service: %w", err)
	}
	approvalNotifications := newApprovalNotificationService(ctx, wailsnotifications.New())
	app = NewAppWithDependencies(ctx, AppDependencies{
		Sessions:             sessions,
		PlayerConfigs:        playerConfigs,
		Live:                 live,
		Coordination:         coordination,
		CoordinationEvents:   effectRouter,
		Player:               player,
		Desktop:              desktop,
		Browser:              desktop,
		LogDirectoryOpener:   desktop,
		LogDirectory:         retainedLog.Directory(),
		ActiveLogPath:        retainedLog.CurrentPath,
		Clipboard:            host.Clipboard,
		Events:               events,
		PublicSettings:       effectivePublicSettings,
		PublicSecrets:        effectivePublicSecrets,
		PublicAccess:         publicAccess,
		Updates:              updates,
		CoordinationObserver: approvalNotifications,
		Logger:               logger.FromContext(ctx),
		StartupTimeout:       time.Duration(runtimeConfig.Startup.TimeoutMilliseconds) * time.Millisecond,
		ShutdownTimeout:      time.Duration(runtimeConfig.Shutdown.TimeoutMilliseconds) * time.Millisecond,
	})
	approvalNotifications.bind(app)
	effectRouter.Bind(player, app)
	return app, nil
}

// sessionCommandStateStore adapts the session owner's context-aware mutation
// API to the coordinator's callback-free synchronous durability seam.
type sessionCommandStateStore struct {
	service *sessionservice.Service
}

var _ sessionservice.TerminalCatalog = (*sessionservice.Service)(nil)
var _ coordinationFacilityLifecycleService = (*controlservice.Service)(nil)
var _ controlservice.CommandStateStore = (*sessionCommandStateStore)(nil)
var _ controlservice.FacilityStore = (*sessionCommandStateStore)(nil)
var _ controlservice.FacilityResetStore = (*sessionCommandStateStore)(nil)
var _ controlservice.FacilityAuthoringStore = (*sessionCommandStateStore)(nil)
var _ controlservice.TerminalGroupStore = (*sessionCommandStateStore)(nil)

func (store *sessionCommandStateStore) ApplyWorldAction(
	ctx context.Context,
	request controlservice.FacilityMutationRequest,
) domain.FacilityOperationResult {
	if store == nil || store.service == nil {
		return domain.FacilityOperationResult{
			CorrelationID: request.CorrelationID,
			Failure:       domain.FacilityFailurePersistenceFailed,
		}
	}
	return store.service.ApplyWorldAction(ctx, sessionservice.WorldActionRequest{
		CorrelationID:            request.CorrelationID,
		TerminalID:               request.TerminalID,
		CommandID:                request.CommandID,
		ExpectedFacilityRevision: request.ExpectedFacilityRevision,
		Transitions:              domain.CloneFacilityTransitionRequests(request.Transitions),
		RecoveryConditionID:      request.RecoveryConditionID,
		Recovery:                 cloneDiagnosticRecoveryReference(request.Recovery),
	})
}

func cloneDiagnosticRecoveryReference(
	recovery *domain.DiagnosticRecoveryReference,
) *domain.DiagnosticRecoveryReference {
	if recovery == nil {
		return nil
	}
	clone := domain.CloneDiagnosticRecoveryReference(*recovery)
	return &clone
}

func (store *sessionCommandStateStore) SaveFacilityAuthoring(
	ctx context.Context,
	request controlservice.FacilityAuthoringRequest,
) domain.FacilityOperationResult {
	if store == nil || store.service == nil {
		return domain.FacilityOperationResult{
			CorrelationID: request.CorrelationID,
			Failure:       domain.FacilityFailurePersistenceFailed,
		}
	}
	return store.service.SaveFacilityAuthoring(ctx, sessionservice.FacilityAuthoringRequest{
		Candidate:                domain.CloneSession(request.Candidate),
		ExpectedSessionRevision:  request.ExpectedSessionRevision,
		ExpectedFacilityRevision: request.ExpectedFacilityRevision,
		CorrelationID:            request.CorrelationID,
	})
}

func (store *sessionCommandStateStore) ResetFacilityDevice(
	ctx context.Context,
	request controlservice.FacilityDeviceResetRequest,
) domain.FacilityOperationResult {
	if store == nil || store.service == nil {
		return domain.FacilityOperationResult{
			CorrelationID: request.CorrelationID,
			Failure:       domain.FacilityFailurePersistenceFailed,
		}
	}
	return store.service.ResetFacilityDevice(ctx, sessionservice.FacilityDeviceResetRequest{
		DeviceID:                 request.DeviceID,
		ExpectedFacilityRevision: request.ExpectedFacilityRevision,
		CorrelationID:            request.CorrelationID,
	})
}

func (store *sessionCommandStateStore) ResetFacility(
	ctx context.Context,
	request controlservice.FacilityResetRequest,
) domain.FacilityOperationResult {
	if store == nil || store.service == nil {
		return domain.FacilityOperationResult{
			CorrelationID: request.CorrelationID,
			Failure:       domain.FacilityFailurePersistenceFailed,
		}
	}
	return store.service.ResetFacility(ctx, sessionservice.FacilityResetRequest{
		ExpectedFacilityRevision: request.ExpectedFacilityRevision,
		CorrelationID:            request.CorrelationID,
	})
}

func (store *sessionCommandStateStore) ReplaceTerminalGroups(
	ctx context.Context,
	groups []domain.TerminalGroup,
	expectedRevision uint64,
) (controlservice.TerminalGroupMutation, error) {
	if store == nil || store.service == nil {
		return controlservice.TerminalGroupMutation{}, errors.New("session terminal-group store is unavailable")
	}
	result := store.service.ReplaceTerminalGroups(ctx, groups, expectedRevision)
	if !result.OK {
		message := result.Error
		if message == "" {
			message = "session terminal-group mutation failed"
		}
		mutation := controlservice.TerminalGroupMutation{Revision: result.Revision}
		if result.Session != nil {
			mutation.Session = *result.Session
		}
		return mutation, &controlservice.TerminalGroupStoreRejection{Message: message}
	}
	if result.Session == nil {
		return controlservice.TerminalGroupMutation{}, errors.New("session terminal-group mutation returned no document")
	}
	return controlservice.TerminalGroupMutation{
		Changed: result.Changed, Revision: result.Revision, Session: *result.Session,
	}, nil
}

func (store *sessionCommandStateStore) ExecuteCommandState(ctx context.Context, terminalID, commandID string) (controlservice.CommandStateMutation, error) {
	if store == nil || store.service == nil {
		return controlservice.CommandStateMutation{}, errors.New("session command-state store is unavailable")
	}
	return commandStateMutation(store.service.ExecuteCommandState(ctx, terminalID, commandID))
}

func (store *sessionCommandStateStore) ResetCommandState(ctx context.Context, terminalID, commandID string) (controlservice.CommandStateMutation, error) {
	if store == nil || store.service == nil {
		return controlservice.CommandStateMutation{}, errors.New("session command-state store is unavailable")
	}
	return commandStateMutation(store.service.ResetCommandState(ctx, terminalID, commandID))
}

func (store *sessionCommandStateStore) ResetTerminalCommandStates(ctx context.Context, terminalID string) (controlservice.CommandStateMutation, error) {
	if store == nil || store.service == nil {
		return controlservice.CommandStateMutation{}, errors.New("session command-state store is unavailable")
	}
	return commandStateMutation(store.service.ResetTerminalCommandStates(ctx, terminalID))
}

func commandStateMutation(result sessionservice.CommandStateResult) (controlservice.CommandStateMutation, error) {
	if !result.OK {
		message := result.Error
		if message == "" {
			message = "session command-state mutation failed"
		}
		return controlservice.CommandStateMutation{}, errors.New(message)
	}
	if result.Session == nil {
		return controlservice.CommandStateMutation{}, errors.New("session command-state mutation returned no document")
	}
	return controlservice.CommandStateMutation{
		Changed: result.Changed, Revision: result.Revision, Session: *result.Session,
	}, nil
}

func publicAccessStoresForProfile(
	settings tunnelservice.PublicAccessSettings,
	secrets tunnelservice.SecretStore,
	packaged bool,
	lookup tunnelservice.EnvironmentLookup,
) (tunnelservice.PublicAccessSettings, tunnelservice.SecretStore) {
	if packaged || lookup == nil {
		return settings, secrets
	}
	override := tunnelservice.NewDevelopmentTestPublicAccessOverride(settings, secrets, lookup)
	return override, override
}

type publicAccessRoute struct {
	PlayerTarget string
	UpstreamURL  string
}

func publicAccessCompositionRoute() publicAccessRoute {
	return publicAccessRoute{
		PlayerTarget: tunnelservice.PlayerUpstreamAddress,
		UpstreamURL:  "http://" + tunnelservice.PlayerUpstreamAddress,
	}
}

func defaultApplicationConfig(locations platform.SessionLocations) *configv1.ApplicationConfig {
	return &configv1.ApplicationConfig{
		PlayerServer: &configv1.PlayerServerConfig{
			Address: "0.0.0.0:3690", DeliveryQueueSize: 32, StartupTimeoutMilliseconds: 20_000,
			RequestLimits: &configv1.PublicRequestLimits{
				UncompressedMessageBytes: playerserver.MaxUncompressedMessageBytes,
				EncodedBodyBytes:         playerserver.MaxEncodedBodyBytes,
				DecompressedMessageBytes: playerserver.MaxDecompressedMessageBytes,
				RecognitionHandleBytes:   domain.MaxRecognitionHandleBytes,
				RequestIdBytes:           domain.MaxRequestIDBytes,
				BroadcastIdBytes:         domain.MaxBroadcastIDBytes,
				GenerationIdBytes:        domain.MaxGenerationIDBytes,
				TerminalIdBytes:          domain.MaxTerminalIDBytes,
				CharacterIdBytes:         domain.MaxCharacterIDBytes,
				ActionTargetBytes:        domain.MaxActionTargetBytes,
				SoundCategoryBytes:       domain.MaxSoundCategoryBytes,
			},
		},
		Coordination: &configv1.CoordinationConfig{RequestResultLimit: 256},
		Browser: &configv1.BrowserClientConfig{
			RecognitionStorageKey:      "fallout-terminal.player-token",
			ReconnectDelayMilliseconds: 3_000,
			ElectionLeaseMilliseconds:  5_000,
		},
		Paths: &configv1.PathConfig{
			DocumentsDirectory:          locations.DocumentsDefault,
			BundledDemoPath:             locations.BundledDemo,
			ApplicationSupportDirectory: locations.ApplicationSupport,
		},
		Startup:  &configv1.StartupConfig{TimeoutMilliseconds: 30_000},
		Shutdown: &configv1.ShutdownConfig{GracePeriodMilliseconds: 2_000, TimeoutMilliseconds: 5_000},
	}
}

// coordinationEffectRouter closes the construction cycle without letting the
// coordinator know about Connect streams or Wails. Enqueue snapshots its targets
// under a short lock and releases that lock before dispatch, so coordinator
// publication cannot re-enter the router or invert lock ownership.
type coordinationEffectRouter struct {
	mu     sync.RWMutex
	player *playerserver.Server
	app    *App

	masterMu        sync.Mutex
	masterDeferrals int
	masterDraining  bool
	masterQueue     []deferredMasterEvent
}

type deferredMasterEvent struct {
	app   *App
	state *domain.MasterCoordinationState
}

type routedMasterEventDeferral struct {
	router *coordinationEffectRouter
	app    *App
	once   sync.Once
}

func (router *coordinationEffectRouter) Bind(player *playerserver.Server, app *App) {
	router.mu.Lock()
	router.player = player
	router.app = app
	router.mu.Unlock()
}

func (router *coordinationEffectRouter) App() *App {
	router.mu.RLock()
	defer router.mu.RUnlock()
	return router.app
}

func (router *coordinationEffectRouter) Enqueue(effect controlservice.Effect) {
	router.mu.RLock()
	player := router.player
	app := router.app
	router.mu.RUnlock()

	if player != nil {
		player.PublishCoordinationEffect(effect)
	}
	if app != nil && effect.Master != nil {
		router.enqueueMasterEvent(app, effect.Master)
	}
	if app != nil {
		app.recordAuditEvents(effect.Audit, effect.Revision)
		switch {
		case effect.Hack != nil:
			app.updateHackState(effect.Hack)
		case effect.Live != nil:
			app.updateHackState(effect.Live.Hack)
		}
	}
}

// DeferMasterEvents preserves coordinator order while allowing an App command
// to publish its matching durable session before the resulting private state.
func (router *coordinationEffectRouter) DeferMasterEvents() coordinationMasterEventDeferral {
	router.mu.RLock()
	app := router.app
	router.mu.RUnlock()
	router.masterMu.Lock()
	router.masterDeferrals++
	router.masterMu.Unlock()
	return &routedMasterEventDeferral{router: router, app: app}
}

func (deferral *routedMasterEventDeferral) Commit() {
	deferral.once.Do(func() { deferral.router.finishMasterEventDeferral(nil, nil) })
}

// Discard removes only the snapshot returned by the failed operation. Other
// coordinator work may enqueue while the App waits for durability.
func (deferral *routedMasterEventDeferral) Discard(state *domain.MasterCoordinationState) {
	deferral.once.Do(func() { deferral.router.finishMasterEventDeferral(deferral.app, state) })
}

func (router *coordinationEffectRouter) finishMasterEventDeferral(
	app *App,
	discarded *domain.MasterCoordinationState,
) {
	router.masterMu.Lock()
	if app != nil && discarded != nil {
		router.masterQueue = slices.DeleteFunc(router.masterQueue, func(event deferredMasterEvent) bool {
			return event.app == app && event.state != nil && event.state.Revision == discarded.Revision
		})
	}
	if router.masterDeferrals > 0 {
		router.masterDeferrals--
	}
	shouldDrain := router.masterDeferrals == 0 && !router.masterDraining && len(router.masterQueue) != 0
	if shouldDrain {
		router.masterDraining = true
	}
	router.masterMu.Unlock()
	if shouldDrain {
		router.drainMasterEvents()
	}
}

func (router *coordinationEffectRouter) enqueueMasterEvent(app *App, state *domain.MasterCoordinationState) {
	router.masterMu.Lock()
	router.masterQueue = append(router.masterQueue, deferredMasterEvent{
		app: app, state: domain.CloneMasterCoordinationState(state),
	})
	shouldDrain := router.masterDeferrals == 0 && !router.masterDraining
	if shouldDrain {
		router.masterDraining = true
	}
	router.masterMu.Unlock()
	if shouldDrain {
		router.drainMasterEvents()
	}
}

func (router *coordinationEffectRouter) drainMasterEvents() {
	for {
		router.masterMu.Lock()
		if router.masterDeferrals != 0 || len(router.masterQueue) == 0 {
			router.masterDraining = false
			router.masterMu.Unlock()
			return
		}
		next := router.masterQueue[0]
		router.masterQueue = router.masterQueue[1:]
		router.masterMu.Unlock()
		next.app.publishCoordinationState(next.state)
	}
}
