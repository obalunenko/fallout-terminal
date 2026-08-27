package main

import (
	"context"
	_ "embed"
	"errors"
	"io/fs"
	"sync"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const wailsShutdownTimeout = 5 * time.Second

const (
	wailsApplicationName        = "Fallout Terminal"
	wailsApplicationDescription = "Fallout Terminal — Overseer Control"
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
}

func newWailsApplication(overseerAssets fs.FS) *application.App {
	return application.New(wailsApplicationOptions(overseerAssets))
}

func wailsApplicationOptions(overseerAssets fs.FS) application.Options {
	return application.Options{
		Name:                        wailsApplicationName,
		Description:                 wailsApplicationDescription,
		Icon:                        wailsApplicationIcon(),
		DisableDefaultSignalHandler: true,
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
	host.RegisterService(application.NewService(newWailsLifecycleService(ctx, core, host.Quit)))
	host.RegisterService(application.NewService(newDesktopService(core)))
}
