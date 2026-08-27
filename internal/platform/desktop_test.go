package platform

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingDialogs struct {
	openContexts []context.Context
	openOptions  []OpenFileOptions
	openResult   string
	openErr      error
	saveContexts []context.Context
	saveOptions  []SaveFileOptions
	saveResult   string
	saveErr      error
}

func (manager *recordingDialogs) OpenFile(ctx context.Context, options OpenFileOptions) (string, error) {
	manager.openContexts = append(manager.openContexts, ctx)
	manager.openOptions = append(manager.openOptions, options)
	return manager.openResult, manager.openErr
}

func (manager *recordingDialogs) SaveFile(ctx context.Context, options SaveFileOptions) (string, error) {
	manager.saveContexts = append(manager.saveContexts, ctx)
	manager.saveOptions = append(manager.saveOptions, options)
	return manager.saveResult, manager.saveErr
}

type recordingBrowser struct {
	contexts []context.Context
	urls     []string
	err      error
}

func (manager *recordingBrowser) OpenURL(ctx context.Context, rawURL string) error {
	manager.contexts = append(manager.contexts, ctx)
	manager.urls = append(manager.urls, rawURL)
	return manager.err
}

func TestDesktopDialogAdaptersPreservePortableJSONOptions(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	missing := filepath.Join(existing, "Campaigns", "Убежище 33")
	tests := []struct {
		name        string
		run         func(*Desktop) (string, error)
		result      string
		wantOpen    *OpenFileOptions
		wantSave    *SaveFileOptions
		wantContext string
	}{
		{
			name:   "open uses nearest existing ancestor and resolves aliases",
			run:    func(desktop *Desktop) (string, error) { return desktop.OpenFile(missing) },
			result: filepath.Join(existing, "campaign.json"),
			wantOpen: &OpenFileOptions{
				Title:            "Open Fallout Terminal Session",
				DefaultDirectory: existing,
				Filters:          []FileFilter{{DisplayName: "Fallout Terminal sessions (*.json)", Pattern: "*.json"}},
				ResolvesAliases:  true,
			},
			wantContext: "open",
		},
		{
			name: "save keeps filename and permits directory creation",
			run: func(desktop *Desktop) (string, error) {
				return desktop.SaveFile(filepath.Join(missing, "смотритель.json"))
			},
			result: filepath.Join(existing, "смотритель.json"),
			wantSave: &SaveFileOptions{
				Title:                "Save Fallout Terminal Session",
				DefaultDirectory:     existing,
				DefaultFilename:      "смотритель.json",
				Filters:              []FileFilter{{DisplayName: "Fallout Terminal sessions (*.json)", Pattern: "*.json"}},
				CanCreateDirectories: true,
			},
			wantContext: "save",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			type contextKey struct{}
			ctx := context.WithValue(t.Context(), contextKey{}, test.wantContext)
			dialogs := &recordingDialogs{openResult: test.result, saveResult: test.result}
			desktop := NewDesktopWithManagers(ctx, dialogs, &recordingBrowser{})
			got, err := test.run(desktop)
			require.NoError(t, err)
			assert.Equal(t, test.result, got)
			if test.wantOpen != nil {
				require.Equal(t, []OpenFileOptions{*test.wantOpen}, dialogs.openOptions)
				require.Len(t, dialogs.openContexts, 1)
				assert.Equal(t, test.wantContext, dialogs.openContexts[0].Value(contextKey{}))
			}
			if test.wantSave != nil {
				require.Equal(t, []SaveFileOptions{*test.wantSave}, dialogs.saveOptions)
				require.Len(t, dialogs.saveContexts, 1)
				assert.Equal(t, test.wantContext, dialogs.saveContexts[0].Value(contextKey{}))
			}
		})
	}
}

func TestDesktopDialogAdaptersPreserveCancelAndNativeErrorOutcomes(t *testing.T) {
	t.Parallel()

	nativeErr := errors.New("dialog unavailable")
	tests := []struct {
		name    string
		run     func(*Desktop) (string, error)
		setup   func(*recordingDialogs)
		wantErr error
	}{
		{
			name:  "open cancel remains empty without error",
			run:   func(desktop *Desktop) (string, error) { return desktop.OpenFile("") },
			setup: func(*recordingDialogs) {},
		},
		{
			name:  "save cancel remains empty without error",
			run:   func(desktop *Desktop) (string, error) { return desktop.SaveFile("session.json") },
			setup: func(*recordingDialogs) {},
		},
		{
			name: "open native error is preserved",
			run:  func(desktop *Desktop) (string, error) { return desktop.OpenFile("") },
			setup: func(dialogs *recordingDialogs) {
				dialogs.openErr = nativeErr
			},
			wantErr: nativeErr,
		},
		{
			name: "save native error is preserved",
			run:  func(desktop *Desktop) (string, error) { return desktop.SaveFile("session.json") },
			setup: func(dialogs *recordingDialogs) {
				dialogs.saveErr = nativeErr
			},
			wantErr: nativeErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dialogs := &recordingDialogs{}
			test.setup(dialogs)
			desktop := NewDesktopWithManagers(t.Context(), dialogs, &recordingBrowser{})
			got, err := test.run(desktop)
			assert.Empty(t, got)
			require.Equal(t, 1, len(dialogs.openOptions)+len(dialogs.saveOptions))
			if test.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestWailsFileFilterPatternTranslatesForEachDesktopPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos string
		want string
	}{
		{goos: "darwin", want: "json;yaml"},
		{goos: "windows", want: "*.json;*.yaml"},
		{goos: "linux", want: "*.json;*.yaml"},
	}

	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.want, wailsFileFilterPattern(test.goos, "*.json;*.yaml"))
		})
	}
}

func TestDesktopBrowserAdapterAllowsOnlyAbsoluteHTTPAndHTTPS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawURL  string
		wantURL string
		wantErr string
	}{
		{name: "http", rawURL: "http://127.0.0.1:3690/player", wantURL: "http://127.0.0.1:3690/player"},
		{name: "https", rawURL: "https://players.example.test/session", wantURL: "https://players.example.test/session"},
		{name: "file", rawURL: "file:///etc/passwd", wantErr: "external URL must be an absolute HTTP or HTTPS URL"},
		{name: "javascript", rawURL: "javascript:alert(1)", wantErr: "external URL must be an absolute HTTP or HTTPS URL"},
		{name: "data", rawURL: "data:text/plain,secret", wantErr: "external URL must be an absolute HTTP or HTTPS URL"},
		{name: "relative", rawURL: "/player", wantErr: "external URL must be an absolute HTTP or HTTPS URL"},
		{name: "scheme relative", rawURL: "//players.example.test/session", wantErr: "external URL must be an absolute HTTP or HTTPS URL"},
		{name: "https without host", rawURL: "https:///session", wantErr: "external URL must be an absolute HTTP or HTTPS URL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			browser := &recordingBrowser{}
			desktop := NewDesktopWithManagers(t.Context(), &recordingDialogs{}, browser)
			err := desktop.OpenURL(test.rawURL)
			if test.wantErr != "" {
				require.EqualError(t, err, test.wantErr)
				require.Empty(t, browser.urls)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{test.wantURL}, browser.urls)
		})
	}
}

func TestDesktopBrowserAdapterPreservesNativeErrors(t *testing.T) {
	t.Parallel()

	nativeErr := errors.New("browser unavailable")
	browser := &recordingBrowser{err: nativeErr}
	desktop := NewDesktopWithManagers(t.Context(), &recordingDialogs{}, browser)
	require.ErrorIs(t, desktop.OpenURL("https://example.test"), nativeErr)
	require.Equal(t, []string{"https://example.test"}, browser.urls)
}

func TestDesktopAdaptersUseReadyContextUntilClosed(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	initialContext := context.WithValue(t.Context(), contextKey{}, "initial")
	readyContext := context.WithValue(t.Context(), contextKey{}, "ready")
	dialogs := &recordingDialogs{}
	browser := &recordingBrowser{}
	desktop := NewDesktopWithManagers(initialContext, dialogs, browser)

	require.NoError(t, desktop.Ready(readyContext))
	_, err := desktop.OpenFile("")
	require.NoError(t, err)
	_, err = desktop.SaveFile("session.json")
	require.NoError(t, err)
	require.NoError(t, desktop.OpenURL("https://example.test"))
	require.Len(t, dialogs.openContexts, 1)
	require.Len(t, dialogs.saveContexts, 1)
	require.Len(t, browser.contexts, 1)
	require.Equal(t, "ready", dialogs.openContexts[0].Value(contextKey{}))
	require.Equal(t, "ready", dialogs.saveContexts[0].Value(contextKey{}))
	require.Equal(t, "ready", browser.contexts[0].Value(contextKey{}))

	require.NoError(t, desktop.Close(t.Context()))
	require.NoError(t, desktop.Close(t.Context()), "Close must be idempotent")
	_, err = desktop.OpenFile("")
	require.ErrorIs(t, err, errDesktopNotReady)
	_, err = desktop.SaveFile("")
	require.ErrorIs(t, err, errDesktopNotReady)
	require.ErrorIs(t, desktop.OpenURL("https://example.test"), errDesktopNotReady)
	require.Len(t, dialogs.openContexts, 1)
	require.Len(t, dialogs.saveContexts, 1)
	require.Len(t, browser.contexts, 1)
}

func TestDesktopAdaptersRejectNilLifecycleContextsWithoutChangingState(t *testing.T) {
	t.Parallel()

	var nilContext context.Context
	desktop := NewDesktopWithManagers(t.Context(), &recordingDialogs{}, &recordingBrowser{})
	require.ErrorIs(t, desktop.Ready(nilContext), errDesktopContextRequired)
	require.ErrorIs(t, desktop.Close(nilContext), errDesktopContextRequired)
	_, err := desktop.OpenFile("")
	require.NoError(t, err)
	require.PanicsWithValue(t, errDesktopContextRequired, func() {
		NewDesktopWithManagers(nilContext, &recordingDialogs{}, &recordingBrowser{})
	})
}

func TestDesktopAdaptersRejectCanceledApplicationContextBeforeNativeCalls(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	cancel()
	dialogs := &recordingDialogs{}
	browser := &recordingBrowser{}
	desktop := NewDesktopWithManagers(ctx, dialogs, browser)

	_, err := desktop.OpenFile("")
	require.ErrorIs(t, err, context.Canceled)
	_, err = desktop.SaveFile("session.json")
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, desktop.OpenURL("https://example.test"), context.Canceled)
	require.Empty(t, dialogs.openContexts)
	require.Empty(t, dialogs.saveContexts)
	require.Empty(t, browser.contexts)
}
