package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubDirectoryProvider struct {
	home               string
	homeErr            error
	documents          string
	documentsErr       error
	applicationData    string
	applicationDataErr error
}

func (p stubDirectoryProvider) HomeDirectory() (string, error) {
	return p.home, p.homeErr
}

func (p stubDirectoryProvider) DocumentsDirectory() (string, error) {
	return p.documents, p.documentsErr
}

func (p stubDirectoryProvider) ApplicationDataDirectory() (string, error) {
	return p.applicationData, p.applicationDataErr
}

func TestPublicAccessSettingsPathUsesApplicationSupportWithoutSideEffects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	home := filepath.Join(root, "Users", "player")
	resourceRoot := filepath.Join(root, "Applications", "Fallout Terminal.app", "Contents", "Resources")
	locations, err := NewSessionLocations(home, resourceRoot)
	require.NoError(t, err)
	path, err := PublicAccessSettingsPath(locations.ApplicationSupport)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(
		home, "Library", "Application Support", "com.vaulttec.fallout-terminal", "public-access.json",
	), path)

	_, err = PublicAccessSettingsPath("relative/application-support")
	require.Error(t, err)
}

func TestApplicationLogDirectoryUsesApplicationSupportWithoutSideEffects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	applicationSupport := filepath.Join(root, "Application Support")
	path, err := ApplicationLogDirectory(applicationSupport)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(applicationSupport, "logs"), path)
	_, err = os.Stat(path)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = ApplicationLogDirectory("relative")
	require.Error(t, err)
}

func TestApplicationUpdateRecoveryPathUsesApplicationSupportWithoutSideEffects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	applicationSupport := filepath.Join(root, "Application Support")
	path, err := ApplicationUpdateRecoveryPath(applicationSupport)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(applicationSupport, applicationUpdateRecoveryFilename), path)
	_, err = os.Stat(applicationSupport)
	assert.ErrorIs(t, err, os.ErrNotExist)

	_, err = ApplicationUpdateRecoveryPath("relative/application-support")
	require.Error(t, err)
}

func TestSessionLocationsForNativeProfiles(t *testing.T) {
	t.Parallel()

	testAbsoluteRoot := t.TempDir()
	tests := map[string]struct {
		goos         string
		provider     stubDirectoryProvider
		resourceRoot string
		want         SessionLocations
	}{
		"darwin preserves documents and application support": {
			goos: "darwin",
			provider: stubDirectoryProvider{
				home:            filepath.Join(testAbsoluteRoot, "Users", "overseer"),
				documents:       filepath.Join(testAbsoluteRoot, "ignored", "redirected documents"),
				applicationData: filepath.Join(testAbsoluteRoot, "ignored", "app data"),
			},
			resourceRoot: filepath.Join(testAbsoluteRoot, "Applications", "Fallout Terminal.app", "Contents", "Resources"),
			want: SessionLocations{
				DocumentsDefault: filepath.Join(testAbsoluteRoot, "Users", "overseer", "Documents", "Fallout Terminal", "Sessions"),
				BundledDemo: filepath.Join(
					testAbsoluteRoot, "Applications", "Fallout Terminal.app", "Contents", "Resources", "sessions", "demo.json",
				),
				ApplicationSupport: filepath.Join(
					testAbsoluteRoot, "Users", "overseer", "Library", "Application Support", "com.vaulttec.fallout-terminal",
				),
			},
		},
		"windows uses redirected known folders": {
			goos: "windows",
			provider: stubDirectoryProvider{
				home:            filepath.Join(testAbsoluteRoot, "Users", "ignored"),
				documents:       filepath.Join(testAbsoluteRoot, "redirected volume", "Overseer Documents"),
				applicationData: filepath.Join(testAbsoluteRoot, "redirected volume", "Overseer AppData"),
			},
			resourceRoot: filepath.Join(testAbsoluteRoot, "portable", "Fallout Terminal", "resources"),
			want: SessionLocations{
				DocumentsDefault: filepath.Join(
					testAbsoluteRoot, "redirected volume", "Overseer Documents", "Fallout Terminal", "Sessions",
				),
				BundledDemo: filepath.Join(
					testAbsoluteRoot, "portable", "Fallout Terminal", "resources", "sessions", "demo.json",
				),
				ApplicationSupport: filepath.Join(
					testAbsoluteRoot, "redirected volume", "Overseer AppData", "com.vaulttec.fallout-terminal",
				),
			},
		},
		"linux uses XDG roots with spaces and Unicode": {
			goos: "linux",
			provider: stubDirectoryProvider{
				home:            filepath.Join(testAbsoluteRoot, "home", "ignored"),
				documents:       filepath.Join(testAbsoluteRoot, "media", "Смотритель", "Мои документы"),
				applicationData: filepath.Join(testAbsoluteRoot, "media", "Смотритель", "Настройки XDG"),
			},
			resourceRoot: filepath.Join(testAbsoluteRoot, "opt", "Fallout Terminal", "resources"),
			want: SessionLocations{
				DocumentsDefault: filepath.Join(
					testAbsoluteRoot, "media", "Смотритель", "Мои документы", "Fallout Terminal", "Sessions",
				),
				BundledDemo: filepath.Join(
					testAbsoluteRoot, "opt", "Fallout Terminal", "resources", "sessions", "demo.json",
				),
				ApplicationSupport: filepath.Join(
					testAbsoluteRoot, "media", "Смотритель", "Настройки XDG", "com.vaulttec.fallout-terminal",
				),
			},
		},
		"linux falls back to home when XDG roots are unset": {
			goos: "linux",
			provider: stubDirectoryProvider{
				home: filepath.Join(testAbsoluteRoot, "home", "lone wanderer"),
			},
			resourceRoot: filepath.Join(testAbsoluteRoot, "srv", "fallout-terminal", "resources"),
			want: SessionLocations{
				DocumentsDefault: filepath.Join(
					testAbsoluteRoot, "home", "lone wanderer", "Documents", "Fallout Terminal", "Sessions",
				),
				BundledDemo: filepath.Join(
					testAbsoluteRoot, "srv", "fallout-terminal", "resources", "sessions", "demo.json",
				),
				ApplicationSupport: filepath.Join(
					testAbsoluteRoot, "home", "lone wanderer", ".config", "com.vaulttec.fallout-terminal",
				),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := sessionLocationsFor(test.goos, test.provider, test.resourceRoot)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
			logDirectory, err := ApplicationLogDirectory(got.ApplicationSupport)
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(got.ApplicationSupport, applicationLogsDirectoryName), logDirectory)
			assert.False(t, pathsOverlap(test.goos, test.resourceRoot, logDirectory),
				"packaged resources and retained logs overlap: resource=%q logs=%q", test.resourceRoot, logDirectory)
		})
	}
}

func TestSessionLocationsForRejectsUnavailableOrUnsafeNativeRoots(t *testing.T) {
	t.Parallel()

	testAbsoluteRoot := t.TempDir()
	errUnavailable := errors.New("native directory service unavailable")
	tests := map[string]struct {
		goos      string
		provider  stubDirectoryProvider
		resource  string
		wantError string
	}{
		"unsupported operating system": {
			goos: "plan9",
			provider: stubDirectoryProvider{
				home: filepath.Join(testAbsoluteRoot, "home", "overseer"),
			},
			resource:  filepath.Join(testAbsoluteRoot, "opt", "fallout", "resources"),
			wantError: "operating system",
		},
		"Windows Known Documents lookup unavailable": {
			goos: "windows",
			provider: stubDirectoryProvider{
				documentsErr:    errUnavailable,
				applicationData: filepath.Join(testAbsoluteRoot, "appdata"),
			},
			resource:  filepath.Join(testAbsoluteRoot, "opt", "fallout", "resources"),
			wantError: "documents directory",
		},
		"Windows application data is read only": {
			goos: "windows",
			provider: stubDirectoryProvider{
				documents:          filepath.Join(testAbsoluteRoot, "documents"),
				applicationDataErr: fs.ErrPermission,
			},
			resource:  filepath.Join(testAbsoluteRoot, "opt", "fallout", "resources"),
			wantError: "application data directory",
		},
		"Windows Known Documents root is empty": {
			goos: "windows",
			provider: stubDirectoryProvider{
				applicationData: filepath.Join(testAbsoluteRoot, "appdata"),
			},
			resource:  filepath.Join(testAbsoluteRoot, "opt", "fallout", "resources"),
			wantError: "documents directory is empty",
		},
		"Windows application data root is relative": {
			goos: "windows",
			provider: stubDirectoryProvider{
				documents:       filepath.Join(testAbsoluteRoot, "documents"),
				applicationData: filepath.Join("relative", "appdata"),
			},
			resource:  filepath.Join(testAbsoluteRoot, "opt", "fallout", "resources"),
			wantError: "application data directory must be absolute",
		},
		"Linux XDG documents lookup fails": {
			goos: "linux",
			provider: stubDirectoryProvider{
				home:         filepath.Join(testAbsoluteRoot, "home", "overseer"),
				documentsErr: errUnavailable,
			},
			resource:  filepath.Join(testAbsoluteRoot, "opt", "fallout", "resources"),
			wantError: "documents directory",
		},
		"Linux XDG config root is relative": {
			goos: "linux",
			provider: stubDirectoryProvider{
				home:            filepath.Join(testAbsoluteRoot, "home", "overseer"),
				documents:       filepath.Join(testAbsoluteRoot, "documents"),
				applicationData: filepath.Join("relative", "config"),
			},
			resource:  filepath.Join(testAbsoluteRoot, "opt", "fallout", "resources"),
			wantError: "application data directory must be absolute",
		},
		"Linux fallback home lookup fails": {
			goos: "linux",
			provider: stubDirectoryProvider{
				homeErr: errUnavailable,
			},
			resource:  filepath.Join(testAbsoluteRoot, "opt", "fallout", "resources"),
			wantError: "home directory",
		},
		"resource root is relative": {
			goos: "linux",
			provider: stubDirectoryProvider{
				home: filepath.Join(testAbsoluteRoot, "home", "overseer"),
			},
			resource:  filepath.Join("relative", "resources"),
			wantError: "resource root must be absolute",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := sessionLocationsFor(test.goos, test.provider, test.resource)
			require.ErrorContains(t, err, test.wantError)
		})
	}
}

func TestSessionLocationsKeepBundledResourcesSeparateFromWritableData(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	resourceRoot := filepath.Join(root, "read only packaged resources")
	require.NoError(t, os.Mkdir(resourceRoot, 0o755))
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Chmod(resourceRoot, 0o555))
		t.Cleanup(func() {
			require.NoError(t, os.Chmod(resourceRoot, 0o755))
		})
	}

	documentsRoot := filepath.Join(root, "redirected writable data", "Documents")
	applicationDataRoot := filepath.Join(root, "redirected writable data", "Settings")
	locations, err := sessionLocationsFor("windows", stubDirectoryProvider{
		documents:       documentsRoot,
		applicationData: applicationDataRoot,
	}, resourceRoot)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(resourceRoot, "sessions", "demo.json"), locations.BundledDemo)
	assert.Equal(t, filepath.Join(documentsRoot, "Fallout Terminal", "Sessions"), locations.DocumentsDefault)
	assert.Equal(t, filepath.Join(applicationDataRoot, "com.vaulttec.fallout-terminal"), locations.ApplicationSupport)
	assert.False(t, testPathContains(resourceRoot, locations.DocumentsDefault))
	assert.False(t, testPathContains(resourceRoot, locations.ApplicationSupport))
	assert.False(t, testPathContains(filepath.Dir(locations.DocumentsDefault), locations.BundledDemo))
	assert.False(t, testPathContains(locations.ApplicationSupport, locations.BundledDemo))
	_, err = os.Stat(locations.DocumentsDefault)
	assert.ErrorIs(t, err, fs.ErrNotExist)
	_, err = os.Stat(locations.ApplicationSupport)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestSessionLocationsRejectWritableDataInsideBundledResources(t *testing.T) {
	t.Parallel()

	testAbsoluteRoot := t.TempDir()
	resourceRoot := filepath.Join(testAbsoluteRoot, "opt", "Fallout Terminal", "resources")
	tests := map[string]stubDirectoryProvider{
		"documents overlap": {
			documents:       filepath.Join(resourceRoot, "user data"),
			applicationData: filepath.Join(testAbsoluteRoot, "home", "overseer", ".config"),
		},
		"application data overlap": {
			documents:       filepath.Join(testAbsoluteRoot, "home", "overseer", "Documents"),
			applicationData: filepath.Join(resourceRoot, "settings"),
		},
	}

	for name, provider := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := sessionLocationsFor("windows", provider, resourceRoot)
			require.ErrorContains(t, err, "bundled resource root")
		})
	}
}

func testPathContains(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || relative != ".." && !filepath.IsAbs(relative) &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
