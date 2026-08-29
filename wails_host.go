package main

import (
	"context"
	cryptorand "crypto/rand"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/platform"
	updateservice "github.com/obalunenko/Fallout-Terminal/v2/internal/update"
	"github.com/obalunenko/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

const wailsShutdownTimeout = 5 * time.Second

const (
	wailsApplicationName        = "Fallout Terminal"
	wailsApplicationDescription = "Fallout Terminal — Overseer Control"
	wailsSingleInstanceID       = "com.vaulttec.fallout-terminal"
	wailsWindowsWindowClass     = "FalloutTerminalWindow"
	wailsLinuxProgramName       = "fallout-terminal"
)

//go:embed build/appicon.png
var embeddedWailsApplicationIcon []byte

var (
	errWailsContextRequired  = errors.New("wails lifecycle context is required")
	errWailsShutdownComplete = errors.New("wails lifecycle shutdown complete")
	errWailsShutdownTimeout  = errors.New("wails lifecycle shutdown timed out")
)

func init() {
	application.RegisterEvent[domain.ServerInfo](serverInfoEvent)
	application.RegisterEvent[int](clientCountEvent)
	application.RegisterEvent[*domain.PublicHackState](hackStateEvent)
	application.RegisterEvent[*domain.MasterCoordinationState](coordinationStateEvent)
	application.RegisterEvent[SessionStateEvent](sessionStateEvent)
	application.RegisterEvent[PublicAccessSnapshot](publicAccessStatusEvent)
	application.RegisterEvent[ApplicationUpdateSnapshot](applicationUpdateStatusEvent)
}

func newWailsApplication(
	overseerAssets fs.FS,
	onSecondInstanceLaunch func(application.SecondInstanceData),
) *application.App {
	return application.New(wailsApplicationOptions(overseerAssets, onSecondInstanceLaunch))
}

// newApplicationUpdateManager composes discovery only for a versioned,
// packaged release. Development and unpackaged builds never construct the
// provider or initialise Wails' updater, so their status handshake cannot
// reach the network.
func newApplicationUpdateManager(
	host *application.App,
	packaged bool,
	installedVersion string,
	recoveryPath string,
	initialFailure *updateservice.UpdateFailure,
	publish updateservice.PublishFunc,
) (*updateservice.Manager, error) {
	config := updateservice.ManagerConfig{
		InstalledVersion: installedVersion,
		Packaged:         packaged,
		Publish:          publish,
		InitialFailure:   initialFailure,
	}
	if !packaged || installedVersion == "" || installedVersion == "development" {
		return updateservice.NewManager(config)
	}
	if initialFailure != nil {
		// Recovery status must remain visible even when update infrastructure is
		// unavailable. A previous apply outcome never requires provider or
		// installation discovery during this launch.
		return updateservice.NewManager(config)
	}
	if host == nil || host.Updater == nil {
		return nil, errors.New("application update host is unavailable")
	}
	provider, err := newApplicationGitHubProvider(applicationGitHubProviderConfig{})
	if err != nil {
		return nil, err
	}
	updaterEvents := newApplicationUpdaterHost(host)
	host.Updater = updater.New(updaterEvents)
	adapter, err := newHeadlessWailsUpdater(host, installedVersion, provider)
	if err != nil {
		return nil, fmt.Errorf("initialize headless updater: %w", err)
	}
	adapter.events = updaterEvents
	installedUnit, installedLaunchRelativePath, err := platform.InstalledApplicationUnit()
	if err != nil {
		return nil, fmt.Errorf("resolve installed application unit: %w", err)
	}
	attemptID := newApplicationUpdateAttemptID()
	config.Check = func(ctx context.Context) (*updateservice.UpdateCandidate, error) {
		release, checkErr := adapter.Check(ctx)
		if checkErr != nil {
			return nil, checkErr
		}
		return applicationUpdateCandidateFromWails(release)
	}
	config.Prepare = func(
		ctx context.Context,
		candidate updateservice.UpdateCandidate,
		report func(updateservice.UpdateState, updateservice.UpdateProgress),
	) (updateservice.PreparedApplicationUnit, error) {
		return adapter.PrepareApplicationUpdate(
			ctx, candidate, attemptID, installedUnit, installedLaunchRelativePath, report,
		)
	}
	config.Restart = func(ctx context.Context, prepared updateservice.PreparedApplicationUnit) error {
		request := updateservice.HelperRequestForPrepared(prepared, os.Getpid(), 0, recoveryPath)
		if err := updateservice.LaunchCopiedReplacementHelper(ctx, request); err != nil {
			return err
		}
		host.Quit()
		return nil
	}
	config.IDs = func() string { return attemptID }
	return updateservice.NewManager(config)
}

func applicationUpdateCandidateFromWails(release *updater.Release) (*updateservice.UpdateCandidate, error) {
	if release == nil {
		return nil, nil
	}
	channel := updateservice.Channel(release.Channel)
	if !channel.Valid() || release.Version == "" || release.Verification == nil ||
		release.Verification.DigestAlgo != "sha256" || len(release.Verification.Digest) != 32 {
		return nil, errors.New("application update release metadata is invalid")
	}
	assetID, idOK := release.Metadata["github.asset.id"].(int64)
	downloadURL, urlOK := release.Metadata["github.asset.url"].(string)
	if !idOK || assetID <= 0 || !urlOK || downloadURL == "" || release.Artifact.Filename == "" ||
		release.Artifact.Size <= 0 || release.Artifact.Platform == "" || release.Artifact.Arch == "" {
		return nil, errors.New("application update release metadata is invalid")
	}
	var digest [32]byte
	copy(digest[:], release.Verification.Digest)
	return &updateservice.UpdateCandidate{
		Version:      release.Version,
		Channel:      channel,
		Name:         release.Name,
		ReleaseNotes: release.Notes,
		PublishedAt:  release.PublishedAt,
		Artifact: updateservice.ReleaseAsset{
			ID: assetID, Name: release.Artifact.Filename, State: "uploaded", Size: release.Artifact.Size,
			DigestAlgorithm: "sha256", Digest: digest, DownloadURL: downloadURL,
			Target: updateservice.Target{OS: release.Artifact.Platform, Arch: release.Artifact.Arch},
		},
	}, nil
}

func newApplicationUpdateAttemptID() string {
	var value [16]byte
	if _, err := cryptorand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	// Discovery is launch scoped and asks for one identifier. A time-derived
	// fallback preserves an opaque nonempty correlation value if system entropy
	// is temporarily unavailable without preventing the local app from starting.
	return fmt.Sprintf("attempt-%x", time.Now().UnixNano())
}

func wailsApplicationOptions(
	overseerAssets fs.FS,
	onSecondInstanceLaunch func(application.SecondInstanceData),
) application.Options {
	return application.Options{
		Name:                        wailsApplicationName,
		Description:                 wailsApplicationDescription,
		Icon:                        wailsApplicationIcon(),
		DisableDefaultSignalHandler: true,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               wailsSingleInstanceID,
			OnSecondInstanceLaunch: onSecondInstanceLaunch,
			ExitCode:               0,
			EncryptionKey:          wailsSingleInstanceEncryptionKey(),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(overseerAssets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			WndClass:                      wailsWindowsWindowClass,
			DisableQuitOnLastWindowClosed: false,
		},
		Linux: application.LinuxOptions{
			DisableQuitOnLastWindowClosed: false,
			ProgramName:                   wailsLinuxProgramName,
		},
	}
}

func wailsApplicationIcon() []byte {
	return append([]byte(nil), embeddedWailsApplicationIcon...)
}

// wailsSingleInstanceEncryptionKey returns a copy of the embedded application
// key so every process can authenticate activation messages before configuration
// or credential stores are available. The callback still ignores payload data.
func wailsSingleInstanceEncryptionKey() [32]byte {
	return [32]byte{
		0x9e, 0x56, 0x30, 0xdc, 0x83, 0x95, 0x19, 0x83,
		0x1d, 0x9a, 0xbb, 0x2f, 0xe7, 0x99, 0x7a, 0x01,
		0x82, 0x59, 0xf2, 0x07, 0x87, 0x4e, 0x42, 0x13,
		0xda, 0x93, 0x12, 0xc1, 0x53, 0x14, 0xd7, 0x82,
	}
}

type overseerWindowActivator interface {
	Restore()
	Focus()
}

type overseerWindowActivation struct {
	mu      sync.Mutex
	window  overseerWindowActivator
	pending bool
}

func (activation *overseerWindowActivation) bind(window overseerWindowActivator) {
	activation.mu.Lock()
	activation.window = window
	pending := activation.pending
	activation.pending = false
	activation.mu.Unlock()
	if pending {
		restoreAndFocusOverseerWindow(window)
	}
}

func (activation *overseerWindowActivation) handleSecondInstanceLaunch(_ application.SecondInstanceData) {
	activation.mu.Lock()
	window := activation.window
	if window == nil {
		activation.pending = true
		activation.mu.Unlock()
		return
	}
	activation.mu.Unlock()
	restoreAndFocusOverseerWindow(window)
}

func restoreAndFocusOverseerWindow(window overseerWindowActivator) {
	window.Restore()
	window.Focus()
}

func newOverseerWindow(host *application.App) *application.WebviewWindow {
	window := host.Window.NewWithOptions(overseerWindowOptions())
	registerOverseerWindowQuitOnClose(window, host)
	return window
}

type overseerWindowCloseRegistrar interface {
	RegisterHook(events.WindowEventType, func(*application.WindowEvent)) func()
}

type applicationQuitter interface {
	Quit()
}

func registerOverseerWindowQuitOnClose(window overseerWindowCloseRegistrar, host applicationQuitter) {
	var quitOnce sync.Once
	requestQuit := func(*application.WindowEvent) {
		quitOnce.Do(func() {
			host.Quit()
		})
	}
	window.RegisterHook(events.Common.WindowClosing, requestQuit)
	registerNativeWindowCloseFallback(window, requestQuit)
}

func overseerWindowOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Title:            wailsApplicationDescription,
		Width:            1200,
		Height:           780,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(11, 13, 10),
		URL:              "/",
		Windows: application.WindowsWindow{
			DisableIcon: false,
		},
		Linux: application.LinuxWindow{
			Icon: wailsApplicationIcon(),
		},
	}
}

// wailsServiceRegistrar is the narrow host seam needed after core composition.
// The concrete Wails application stays in root composition and the core App is
// never registered as a frontend service.
type wailsServiceRegistrar interface {
	RegisterService(application.Service)
	Quit()
}

type coreLifecycle interface {
	Start(context.Context) error
	Shutdown(context.Context) error
}

type wailsEventEmitter interface {
	Emit(string, ...any) bool
}

type wailsEventSink struct {
	events wailsEventEmitter
}

func newWailsEventSink(events wailsEventEmitter) *wailsEventSink {
	return &wailsEventSink{events: events}
}

func (sink *wailsEventSink) Emit(name string, payload any) error {
	if sink == nil || sink.events == nil {
		return errors.New("wails event manager is unavailable")
	}
	sink.events.Emit(name, payload)
	return nil
}

// applicationUpdaterHost keeps Wails updater lifecycle payloads inside Go.
// Release objects contain backend-only verification and download metadata, so
// they must not be forwarded through the general Wails event bridge. Authored
// update snapshots are published separately by App.
type applicationUpdaterHost struct {
	application *application.App

	mu        sync.RWMutex
	nextID    uint64
	listeners map[string]map[uint64]func(any)
}

func newApplicationUpdaterHost(host *application.App) *applicationUpdaterHost {
	return &applicationUpdaterHost{
		application: host,
		listeners:   make(map[string]map[uint64]func(any)),
	}
}

func (host *applicationUpdaterHost) Emit(name string, data ...any) bool {
	host.mu.RLock()
	callbacks := make([]func(any), 0, len(host.listeners[name]))
	for _, callback := range host.listeners[name] {
		callbacks = append(callbacks, callback)
	}
	host.mu.RUnlock()
	var payload any
	if len(data) == 1 {
		payload = data[0]
	} else if len(data) > 1 {
		payload = append([]any(nil), data...)
	}
	for _, callback := range callbacks {
		callback(payload)
	}
	return len(callbacks) > 0
}

func (host *applicationUpdaterHost) OnEvent(name string, callback func(any)) func() {
	if callback == nil {
		return func() {}
	}
	host.mu.Lock()
	host.nextID++
	id := host.nextID
	if host.listeners[name] == nil {
		host.listeners[name] = make(map[uint64]func(any))
	}
	host.listeners[name][id] = callback
	host.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			host.mu.Lock()
			delete(host.listeners[name], id)
			host.mu.Unlock()
		})
	}
}

func (*applicationUpdaterHost) OpenWindow(updater.WindowOptions) updater.WindowHandle { return nil }

func (host *applicationUpdaterHost) Quit() {
	if host.application != nil {
		host.application.Quit()
	}
}

// wailsLifecycleService adapts framework lifecycle callbacks to the unbound
// core. Its method names are Wails lifecycle hooks, not authored bridge calls.
type wailsLifecycleService struct {
	root                context.Context
	core                coreLifecycle
	quit                func()
	runtimeCancel       context.CancelCauseFunc
	stopRootPropagation func() bool
	log                 logger.Logger
}

func newWailsLifecycleService(ctx context.Context, core coreLifecycle, quit func(), logs ...logger.Logger) *wailsLifecycleService {
	if ctx == nil {
		panic(errWailsContextRequired)
	}
	applicationLogger := logger.FromContext(ctx)
	if len(logs) > 0 && logs[0] != nil {
		applicationLogger = logs[0]
	}
	return &wailsLifecycleService{root: ctx, core: core, quit: quit, log: applicationLogger}
}

func (service *wailsLifecycleService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if ctx == nil {
		return errWailsContextRequired
	}
	runtimeContext, cancel := context.WithCancelCause(ctx)
	service.runtimeCancel = cancel
	service.stopRootPropagation = context.AfterFunc(service.root, func() {
		cancel(context.Cause(service.root))
		if service.quit != nil {
			service.quit()
		}
	})
	// Application-owned failures are recorded by the core in RuntimeStatus and
	// leave the Overseer window available to explain the failure. Returning them
	// here would make Wails abort before that status can be presented.
	if err := service.core.Start(runtimeContext); err != nil {
		service.log.WithFields(logger.Fields{
			"operation": "application.start",
			"outcome":   "failed",
		}).Error("application startup failed")
	}
	return nil
}

func (service *wailsLifecycleService) ServiceShutdown() error {
	if service.stopRootPropagation != nil {
		service.stopRootPropagation()
	}
	if service.runtimeCancel != nil {
		defer service.runtimeCancel(errWailsShutdownComplete)
	}
	ctx, cancel := boundedCleanupContext(service.root, wailsShutdownTimeout, errWailsShutdownTimeout)
	defer cancel(errWailsShutdownComplete)
	return service.core.Shutdown(ctx)
}

func registerWailsServices(ctx context.Context, host wailsServiceRegistrar, core *App) {
	if core != nil {
		if notifications, ok := core.deps.CoordinationObserver.(*approvalNotificationService); ok {
			host.RegisterService(application.NewService(notifications))
		}
	}
	host.RegisterService(application.NewService(newWailsLifecycleService(ctx, core, host.Quit)))
	host.RegisterService(application.NewService(newDesktopService(core)))
}
