package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/buildtool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunExposesNetworkFreeReleaseCheckActions(t *testing.T) {
	originalInspector := inspectReleaseArchiveVersion
	t.Cleanup(func() {
		inspectReleaseArchiveVersion = originalInspector
	})
	inspectReleaseArchiveVersion = func(
		ctx context.Context,
		target buildtool.Target,
		archivePath string,
		expectedVersion string,
	) error {
		require.NoError(t, ctx.Err())
		assert.Equal(t, "windows/amd64", target.String())
		assert.Equal(t, "Fallout-Terminal-windows-amd64.zip", filepath.Base(archivePath))
		assert.Equal(t, "2.0.0", expectedVersion)
		return nil
	}

	root := t.TempDir()
	require.NoError(t, run(t.Context(), root, []string{"validate-release-tag", "--tag", "v2.0.0-rc.1"}))

	target, err := buildtool.ParseTarget("windows", "amd64")
	require.NoError(t, err)
	archivePath := filepath.Join(root, target.ArchiveName())
	writeCLIArchiveFixture(t, archivePath, target)
	require.NoError(t, run(t.Context(), root, []string{
		"inspect-release-archive", "--target", target.String(), "--archive", archivePath, "--version", "2.0.0",
	}))

	inventory := t.TempDir()
	for _, pair := range [][2]string{{"windows", "amd64"}, {"windows", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "arm64"}} {
		matrixTarget, parseErr := buildtool.ParseTarget(pair[0], pair[1])
		require.NoError(t, parseErr)
		require.NoError(t, os.WriteFile(filepath.Join(inventory, matrixTarget.ArchiveName()), []byte("archive"), 0o600))
	}
	require.NoError(t, run(t.Context(), root, []string{"inspect-release-inventory", "--directory", inventory}))
}

func TestValidateReleaseTagWritesOnlyCanonicalVersion(t *testing.T) {
	output, err := captureStandardOutput(t, func() error {
		return run(t.Context(), t.TempDir(), []string{"validate-release-tag", "--tag", "v2.0.0-rc.1"})
	})
	require.NoError(t, err)
	assert.Equal(t, "2.0.0-rc.1\n", output)
}

func TestInspectReleaseArchiveRequiresCanonicalExpectedVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		version        string
		includeVersion bool
		diagnostic     string
	}{
		{name: "missing version", diagnostic: "--version"},
		{name: "empty version", includeVersion: true, diagnostic: "must not be empty"},
		{name: "development identity", version: "development", includeVersion: true, diagnostic: "development"},
		{name: "tag prefix", version: "v2.0.0", includeVersion: true, diagnostic: "v2.0.0"},
		{name: "wrong major", version: "1.0.0", includeVersion: true, diagnostic: "1.0.0"},
		{name: "build metadata", version: "2.0.0+build.1", includeVersion: true, diagnostic: "2.0.0+build.1"},
		{name: "incomplete version", version: "2.0", includeVersion: true, diagnostic: "2.0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			arguments := []string{
				"inspect-release-archive",
				"--target", "windows/amd64",
				"--archive", filepath.Join(t.TempDir(), "Fallout-Terminal-windows-amd64.zip"),
			}
			if test.includeVersion {
				arguments = append(arguments, "--version", test.version)
			}

			err := run(t.Context(), t.TempDir(), arguments)
			var commandUsageError *usageError
			require.ErrorAs(t, err, &commandUsageError)
			assert.Contains(t, err.Error(), "version")
			assert.Contains(t, err.Error(), test.diagnostic)
		})
	}
}

func TestRunRetainsLocalDockerActionAndRejectsObsoleteActions(t *testing.T) {
	t.Parallel()

	err := run(t.Context(), t.TempDir(), []string{"package-all-docker", "--unsupported"})
	var commandUsageError *usageError
	require.ErrorAs(t, err, &commandUsageError)
	assert.NotContains(t, err.Error(), "unknown action")

	for _, action := range []string{"run", "package-all", "release-candidate"} {
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
		"inspect-release-archive", "--target", "windows/amd64", "--archive", "Fallout-Terminal-windows-amd64.zip", "--version", "2.0.0",
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

func captureStandardOutput(t *testing.T, action func() error) (string, error) {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })
	t.Cleanup(func() { _ = writer.Close() })

	original := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = original })

	actionErr := action()
	os.Stdout = original
	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(output), actionErr
}
