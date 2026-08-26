package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestValidateProductionResources(t *testing.T) {
	demo := filepath.Join(t.TempDir(), "demo.json")
	require.NoError(t, os.WriteFile(demo, []byte("{}"), 0o600))

	completePlayer := fs.FS(fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html>")},
	})

	t.Run("complete package", func(t *testing.T) {
		require.NoError(t, validateProductionResources(completePlayer, demo))
	})

	t.Run("missing player index", func(t *testing.T) {
		err := validateProductionResources(fstest.MapFS{".keep": {}}, demo)
		require.EqualError(t, err, "player assets are incomplete: index.html is unavailable")
	})

	t.Run("missing bundled demo", func(t *testing.T) {
		err := validateProductionResources(completePlayer, filepath.Join(t.TempDir(), "missing.json"))
		require.ErrorContains(t, err, "bundled demo is unavailable")
	})

	t.Run("empty bundled demo", func(t *testing.T) {
		emptyDemo := filepath.Join(t.TempDir(), "demo.json")
		require.NoError(t, os.WriteFile(emptyDemo, nil, 0o600))
		err := validateProductionResources(completePlayer, emptyDemo)
		require.ErrorContains(t, err, "bundled demo is unavailable")
	})

	t.Run("player index must be a regular file", func(t *testing.T) {
		err := validateProductionResources(fstest.MapFS{"index.html": {Mode: fs.ModeDir}}, demo)
		require.ErrorContains(t, err, "player assets are incomplete")
	})
}

func TestWailsV3HostKeepsOverseerAndClientResourceBoundariesSeparate(t *testing.T) {
	root, err := os.Getwd()
	require.NoError(t, err)

	mainSource, err := os.ReadFile(filepath.Join(root, "main.go"))
	require.NoError(t, err)
	hostSource, err := os.ReadFile(filepath.Join(root, "wails_host.go"))
	require.NoError(t, err)

	mainText := string(mainSource)
	hostText := string(hostSource)
	require.Contains(t, mainText, `fs.Sub(overseerSource, "frontend/overseer/dist")`)
	require.Contains(t, mainText, `fs.Sub(clientSource, "frontend/client/dist")`)
	require.Contains(t, mainText, "composeApplication(rootContext, host, clientAssets)")
	require.Contains(t, mainText, "signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)")
	require.Contains(t, hostText, "application.AssetFileServerFS(overseerAssets)")
	require.Contains(t, hostText, "newDesktopService(core)")
	require.NotContains(t, hostText, "clientAssets")
	require.Equal(t, 1, strings.Count(hostText, "host.Window.NewWithOptions("))
}

func TestApplicationPackageIdentityComesFromCompileTimeProfile(t *testing.T) {
	require.Equal(t, packagedBuild, isPackagedApplication(), "package identity must not depend on executable path or process state")

	root, err := os.Getwd()
	require.NoError(t, err)
	profiles := []struct {
		filename   string
		constraint string
		identity   string
	}{
		{
			filename:   "build_profile_development.go",
			constraint: "//go:build !production",
			identity:   "const packagedBuild = false",
		},
		{
			filename:   "build_profile_production.go",
			constraint: "//go:build production",
			identity:   "const packagedBuild = true",
		},
	}
	for _, profile := range profiles {
		t.Run(profile.filename, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(root, profile.filename))
			require.NoError(t, err)
			text := string(source)
			require.Contains(t, text, profile.constraint)
			require.Contains(t, text, profile.identity)
		})
	}
}

func TestApplicationResourceRootUsesTargetPackageLayout(t *testing.T) {
	root := t.TempDir()
	unrelatedWorkingDirectory := filepath.Join(root, "launcher", "unrelated working directory")
	require.NoError(t, os.MkdirAll(unrelatedWorkingDirectory, 0o755))

	tests := []struct {
		name       string
		goos       string
		executable string
		want       string
	}{
		{
			name:       "macOS bundle",
			goos:       "darwin",
			executable: filepath.Join(root, "Fallout Terminal.app", "Contents", "MacOS", "Fallout Terminal"),
			want:       filepath.Join(root, "Fallout Terminal.app", "Contents", "Resources"),
		},
		{
			name:       "Windows portable archive",
			goos:       "windows",
			executable: filepath.Join(root, "windows package", "Fallout Terminal.exe"),
			want:       filepath.Join(root, "windows package", "resources"),
		},
		{
			name:       "Linux portable archive",
			goos:       "linux",
			executable: filepath.Join(root, "linux package", "Fallout Terminal"),
			want:       filepath.Join(root, "linux package", "resources"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, os.MkdirAll(test.want, 0o755))
			require.Equal(t, test.want, applicationResourceRootFor(true, test.goos, test.executable, unrelatedWorkingDirectory))
		})
	}
}

func TestDevelopmentResourceRootUsesCheckoutWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	checkout := filepath.Join(root, "checkout", "Fallout-Terminal")
	require.NoError(t, os.MkdirAll(checkout, 0o755))
	executable := filepath.Join(root, "somewhere else", "Fallout Terminal.exe")

	for _, goos := range []string{"darwin", "windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			require.Equal(t, checkout, applicationResourceRootFor(false, goos, executable, checkout))
		})
	}
}

func TestPackagedResourceRootDoesNotFallBackWhenResourcesAreMissing(t *testing.T) {
	root := t.TempDir()
	unrelatedWorkingDirectory := filepath.Join(root, "checkout with resources")
	require.NoError(t, os.MkdirAll(filepath.Join(unrelatedWorkingDirectory, "sessions"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(unrelatedWorkingDirectory, "sessions", "demo.json"), []byte("{}"), 0o600))
	completePlayer := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}}

	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			executableName := "Fallout Terminal"
			if goos == "windows" {
				executableName += ".exe"
			}
			executable := filepath.Join(root, goos+" package", executableName)
			resourceRoot := applicationResourceRootFor(true, goos, executable, unrelatedWorkingDirectory)
			require.Equal(t, filepath.Join(filepath.Dir(executable), "resources"), resourceRoot)

			err := validateProductionResources(completePlayer, filepath.Join(resourceRoot, "sessions", "demo.json"))
			require.ErrorContains(t, err, "bundled demo is unavailable")
		})
	}
}
