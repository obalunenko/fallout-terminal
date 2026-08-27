package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/buildtool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunExposesNetworkFreeReleaseCheckActions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, run(t.Context(), root, []string{"validate-release-tag", "--tag", "v2.0.0-rc.1"}))

	target, err := buildtool.ParseTarget("windows", "amd64")
	require.NoError(t, err)
	archivePath := filepath.Join(root, target.ArchiveName())
	writeCLIArchiveFixture(t, archivePath, target)
	require.NoError(t, run(t.Context(), root, []string{
		"inspect-release-archive", "--target", target.String(), "--archive", archivePath,
	}))

	inventory := t.TempDir()
	for _, pair := range [][2]string{{"windows", "amd64"}, {"windows", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "arm64"}} {
		matrixTarget, parseErr := buildtool.ParseTarget(pair[0], pair[1])
		require.NoError(t, parseErr)
		require.NoError(t, os.WriteFile(filepath.Join(inventory, matrixTarget.ArchiveName()), []byte("archive"), 0o600))
	}
	require.NoError(t, run(t.Context(), root, []string{"inspect-release-inventory", "--directory", inventory}))
}

func TestRunRetainsLocalDockerActionAndRejectsObsoleteRemoteActions(t *testing.T) {
	t.Parallel()

	err := run(t.Context(), t.TempDir(), []string{"package-all-docker", "--unsupported"})
	var commandUsageError *usageError
	require.ErrorAs(t, err, &commandUsageError)
	assert.NotContains(t, err.Error(), "unknown action")

	for _, action := range []string{"package-all", "release-candidate"} {
		err := run(t.Context(), t.TempDir(), []string{action})
		require.ErrorAs(t, err, &commandUsageError)
		assert.Contains(t, err.Error(), "unknown action")
	}
}

func TestReleaseCheckActionsReportUsageAndCancellation(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"validate-release-tag"},
		{"validate-release-tag", "--tag", "v2.0.0", "extra"},
		{"inspect-release-archive", "--target", "windows/amd64"},
		{"inspect-release-inventory"},
	}
	for _, arguments := range tests {
		err := run(t.Context(), t.TempDir(), arguments)
		var commandUsageError *usageError
		require.ErrorAs(t, err, &commandUsageError, arguments)
	}

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	cancel()
	err := run(ctx, t.TempDir(), []string{
		"inspect-release-archive", "--target", "windows/amd64", "--archive", "Fallout-Terminal-windows-amd64.zip",
	})
	assert.True(t, errors.Is(err, context.Canceled))
}

func writeCLIArchiveFixture(t *testing.T, archivePath string, target buildtool.Target) {
	t.Helper()
	entries := append([]string{target.ExecutablePath()}, target.RequiredResourcePaths()...)
	var contents bytes.Buffer
	writer := zip.NewWriter(&contents)
	for _, name := range entries {
		entry, err := writer.Create(path.Join("Fallout Terminal", name))
		require.NoError(t, err)
		_, err = entry.Write([]byte("fixture"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, os.WriteFile(archivePath, contents.Bytes(), 0o600))
}
