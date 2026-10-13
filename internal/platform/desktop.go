package platform

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	errDesktopNotReady        = errors.New("desktop runtime is not ready")
	errDesktopContextRequired = errors.New("desktop context is required")
	errExternalURLUnsupported = errors.New("external URL must be an absolute HTTP or HTTPS URL")
	errDesktopPathUnsupported = errors.New("desktop path must be an absolute existing directory")
)

// Desktop is the narrow Wails-backed implementation used for native session
// dialogs and external HTTP(S) links. It retains only the Wails application
// context and exposes no general filesystem or process surface.
type Desktop struct {
	mu      sync.RWMutex
	ctx     context.Context
	dialogs DialogManager
	browser BrowserManager
}

type FileFilter struct {
	DisplayName string
	Pattern     string
}

type OpenFileOptions struct {
	Title            string
	DefaultDirectory string
	DefaultFilename  string
	Filters          []FileFilter
	ResolvesAliases  bool
}

type SaveFileOptions struct {
	Title                string
	DefaultDirectory     string
	DefaultFilename      string
	Filters              []FileFilter
	CanCreateDirectories bool
}

type DialogManager interface {
	OpenFile(context.Context, OpenFileOptions) (string, error)
	SaveFile(context.Context, SaveFileOptions) (string, error)
}

type BrowserManager interface {
	OpenURL(context.Context, string) error
	OpenFile(context.Context, string) error
}

type wailsV3DialogManager struct {
	manager *application.DialogManager
}

func (manager wailsV3DialogManager) OpenFile(_ context.Context, options OpenFileOptions) (string, error) {
	if manager.manager == nil {
		return "", errors.New("native dialog manager is unavailable")
	}
	dialog := manager.manager.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:           options.Title,
		Directory:       options.DefaultDirectory,
		Filters:         wailsFileFilters(runtime.GOOS, options.Filters),
		ResolvesAliases: options.ResolvesAliases,
	})
	return dialog.PromptForSingleSelection()
}

func (manager wailsV3DialogManager) SaveFile(_ context.Context, options SaveFileOptions) (string, error) {
	if manager.manager == nil {
		return "", errors.New("native dialog manager is unavailable")
	}
	dialog := manager.manager.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:                options.Title,
		Directory:            options.DefaultDirectory,
		Filename:             options.DefaultFilename,
		Filters:              wailsFileFilters(runtime.GOOS, options.Filters),
		CanCreateDirectories: options.CanCreateDirectories,
	})
	return dialog.PromptForSingleSelection()
}

func wailsFileFilters(goos string, filters []FileFilter) []application.FileFilter {
	nativeFilters := make([]application.FileFilter, 0, len(filters))
	for _, filter := range filters {
		nativeFilters = append(nativeFilters, application.FileFilter{
			DisplayName: filter.DisplayName,
			Pattern:     wailsFileFilterPattern(goos, filter.Pattern),
		})
	}
	return nativeFilters
}

// Wails v3 beta passes filter patterns directly to
// UTType.typeWithFilenameExtension on Darwin. That API expects bare
// extensions, while the cross-platform Wails contract documents glob patterns.
// Normalize only the Darwin boundary so the native panel can select JSON files
// without changing the platform-independent dialog contract.
func wailsFileFilterPattern(goos, pattern string) string {
	if goos != "darwin" {
		return pattern
	}
	parts := strings.Split(pattern, ";")
	for index, part := range parts {
		parts[index] = strings.TrimPrefix(part, "*.")
	}
	return strings.Join(parts, ";")
}

type wailsV3BrowserManager struct {
	manager *application.BrowserManager
}

func (manager wailsV3BrowserManager) OpenURL(_ context.Context, rawURL string) error {
	if manager.manager == nil {
		return errors.New("external browser manager is unavailable")
	}
	return manager.manager.OpenURL(rawURL)
}

func (manager wailsV3BrowserManager) OpenFile(_ context.Context, path string) error {
	if manager.manager == nil {
		return errors.New("external browser manager is unavailable")
	}
	return manager.manager.OpenFile(path)
}

// NewDesktop constructs a wrapper over the application-owned Wails v3
// managers. A non-nil context may be supplied by tests or composition code
// that already started.
func NewDesktop(ctx context.Context, dialogs *application.DialogManager, browser *application.BrowserManager) *Desktop {
	return NewDesktopWithManagers(ctx, wailsV3DialogManager{manager: dialogs}, wailsV3BrowserManager{manager: browser})
}

func NewDesktopWithManagers(ctx context.Context, dialogs DialogManager, browser BrowserManager) *Desktop {
	if ctx == nil {
		panic(errDesktopContextRequired)
	}
	return &Desktop{ctx: ctx, dialogs: dialogs, browser: browser}
}

// Ready installs the Wails application context.
func (desktop *Desktop) Ready(ctx context.Context) error {
	if ctx == nil {
		return errDesktopContextRequired
	}
	desktop.mu.Lock()
	desktop.ctx = ctx
	desktop.mu.Unlock()
	return nil
}

// Close releases the retained desktop context. It is idempotent.
func (desktop *Desktop) Close(ctx context.Context) error {
	if ctx == nil {
		return errDesktopContextRequired
	}
	desktop.mu.Lock()
	desktop.ctx = nil
	desktop.mu.Unlock()
	return nil
}

// OpenFile shows a native JSON session picker. An absent suggested directory
// is never created merely to display the dialog; the nearest existing ancestor
// is used as the native starting point.
func (desktop *Desktop) OpenFile(defaultPath string) (string, error) {
	ctx, err := desktop.context()
	if err != nil {
		return "", err
	}
	directory, filename := dialogLocation(defaultPath, false)
	if desktop.dialogs == nil {
		return "", errors.New("native dialog manager is unavailable")
	}
	return desktop.dialogs.OpenFile(ctx, OpenFileOptions{
		Title:            "Open Fallout Terminal Session",
		DefaultDirectory: directory,
		DefaultFilename:  filename,
		Filters: []FileFilter{
			{DisplayName: "Fallout Terminal sessions (*.json)", Pattern: "*.json"},
		},
		ResolvesAliases: true,
	})
}

// SaveFile shows a native JSON session destination picker.
func (desktop *Desktop) SaveFile(defaultPath string) (string, error) {
	ctx, err := desktop.context()
	if err != nil {
		return "", err
	}
	directory, filename := dialogLocation(defaultPath, true)
	if desktop.dialogs == nil {
		return "", errors.New("native dialog manager is unavailable")
	}
	return desktop.dialogs.SaveFile(ctx, SaveFileOptions{
		Title:                "Save Fallout Terminal Session",
		DefaultDirectory:     directory,
		DefaultFilename:      filename,
		CanCreateDirectories: true,
		Filters: []FileFilter{
			{DisplayName: "Fallout Terminal sessions (*.json)", Pattern: "*.json"},
		},
	})
}

// OpenURL opens only absolute HTTP(S) URLs in the system browser.
func (desktop *Desktop) OpenURL(rawURL string) error {
	ctx, err := desktop.context()
	if err != nil {
		return err
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return errExternalURLUnsupported
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errExternalURLUnsupported
	}
	if desktop.browser == nil {
		return errors.New("external browser manager is unavailable")
	}
	return desktop.browser.OpenURL(ctx, parsed.String())
}

// OpenDirectory opens one already-resolved, existing directory. It is kept
// separate from OpenURL so filesystem paths can never enter URL handling.
func (desktop *Desktop) OpenDirectory(path string) error {
	ctx, err := desktop.context()
	if err != nil {
		return err
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) || cleaned != path {
		return errDesktopPathUnsupported
	}
	info, err := os.Lstat(cleaned)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errDesktopPathUnsupported
	}
	if desktop.browser == nil {
		return errors.New("external browser manager is unavailable")
	}
	return desktop.browser.OpenFile(ctx, cleaned)
}

func (desktop *Desktop) context() (context.Context, error) {
	desktop.mu.RLock()
	defer desktop.mu.RUnlock()
	if desktop.ctx == nil {
		return nil, errDesktopNotReady
	}
	if err := desktop.ctx.Err(); err != nil {
		return nil, err
	}
	return desktop.ctx, nil
}
