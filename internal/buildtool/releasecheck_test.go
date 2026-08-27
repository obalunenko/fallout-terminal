package buildtool

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateReleaseTagUsesStrictV2SemVer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag        string
		prerelease bool
		valid      bool
	}{
		{tag: "v2.0.0", valid: true},
		{tag: "v2.1.3-beta.1", prerelease: true, valid: true},
		{tag: "v0.0.0-rc.1"},
		{tag: "v1.2.3"},
		{tag: "v3.0.0"},
		{tag: "1.2.3"},
		{tag: "v01.2.3"},
		{tag: "v2.02.3"},
		{tag: "v2.2.03"},
		{tag: "v2.2.3-01"},
		{tag: "v2.2.3-"},
		{tag: "v2.2.3+build.1"},
		{tag: "vnext"},
		{tag: "V2.2.3"},
		{tag: " v2.2.3"},
	}

	for _, test := range tests {
		t.Run(test.tag, func(t *testing.T) {
			t.Parallel()

			prerelease, err := ValidateReleaseTag(test.tag)
			if !test.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.prerelease, prerelease)
		})
	}
}

func TestInspectReleaseArchiveChecksOnlyMinimalEligibility(t *testing.T) {
	t.Parallel()

	for _, target := range PortableTargets() {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			archivePath := filepath.Join(root, target.ArchiveName())
			entries := releaseArchiveEntries(target)
			entries[path.Join(applicationName, "optional-local-metadata.txt")] = []byte("allowed\n")
			writeReleaseArchiveFixture(t, archivePath, target.ArchiveFormat(), entries)

			require.NoError(t, InspectReleaseArchive(t.Context(), target, archivePath))
		})
	}
}

func TestInspectReleaseArchiveRejectsIneligibleArchives(t *testing.T) {
	t.Parallel()

	target := mustParseTarget(t, goosWindows, goarchAMD64)
	tests := []struct {
		name   string
		mutate func(map[string][]byte)
		empty  bool
		wrong  bool
	}{
		{name: "empty archive", empty: true},
		{name: "wrong stable filename", wrong: true},
		{name: "missing executable", mutate: func(entries map[string][]byte) {
			delete(entries, path.Join(applicationName, target.ExecutablePath()))
		}},
		{name: "empty executable", mutate: func(entries map[string][]byte) {
			entries[path.Join(applicationName, target.ExecutablePath())] = nil
		}},
		{name: "missing required resource", mutate: func(entries map[string][]byte) {
			delete(entries, path.Join(applicationName, target.RequiredResourcePaths()[0]))
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			name := target.ArchiveName()
			if test.wrong {
				name = "renamed.zip"
			}
			archivePath := filepath.Join(root, name)
			if test.empty {
				require.NoError(t, os.WriteFile(archivePath, nil, 0o600))
			} else {
				entries := releaseArchiveEntries(target)
				if test.mutate != nil {
					test.mutate(entries)
				}
				writeReleaseArchiveFixture(t, archivePath, target.ArchiveFormat(), entries)
			}

			require.Error(t, InspectReleaseArchive(t.Context(), target, archivePath))
		})
	}
}

func TestReleaseChecksHonorCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	cancel()
	target := mustParseTarget(t, goosLinux, goarchAMD64)
	root := t.TempDir()
	archivePath := filepath.Join(root, target.ArchiveName())
	writeReleaseArchiveFixture(t, archivePath, target.ArchiveFormat(), releaseArchiveEntries(target))

	assert.ErrorIs(t, InspectReleaseArchive(ctx, target, archivePath), context.Canceled)
	assert.ErrorIs(t, InspectReleaseInventory(ctx, root), context.Canceled)
}

func TestInspectReleaseInventoryRequiresExactlyFiveNonEmptyArchives(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, target := range PortableTargets() {
		require.NoError(t, os.WriteFile(filepath.Join(root, target.ArchiveName()), []byte("archive"), 0o600))
	}
	require.NoError(t, InspectReleaseInventory(t.Context(), root))
}

func TestInspectReleaseInventoryRejectsMissingEmptyNestedAndExtraAssets(t *testing.T) {
	t.Parallel()

	extras := []string{
		"Fallout-Terminal-windows-amd64.zip.sha256",
		"aggregate-index.json",
		"Fallout Terminal.exe",
		"Fallout-Terminal-darwin-arm64.dmg",
		"release-verification.json",
		"unexpected.txt",
	}

	for _, extra := range extras {
		t.Run("extra "+extra, func(t *testing.T) {
			t.Parallel()

			root := validReleaseInventoryFixture(t)
			require.NoError(t, os.WriteFile(filepath.Join(root, extra), []byte("extra"), 0o600))
			require.Error(t, InspectReleaseInventory(t.Context(), root))
		})
	}

	t.Run("missing archive", func(t *testing.T) {
		t.Parallel()
		root := validReleaseInventoryFixture(t)
		require.NoError(t, os.Remove(filepath.Join(root, "Fallout-Terminal-linux-arm64.tar.gz")))
		require.Error(t, InspectReleaseInventory(t.Context(), root))
	})
	t.Run("empty archive", func(t *testing.T) {
		t.Parallel()
		root := validReleaseInventoryFixture(t)
		require.NoError(t, os.WriteFile(filepath.Join(root, "Fallout-Terminal-linux-arm64.tar.gz"), nil, 0o600))
		require.Error(t, InspectReleaseInventory(t.Context(), root))
	})
	t.Run("nested archive", func(t *testing.T) {
		t.Parallel()
		root := validReleaseInventoryFixture(t)
		missing := filepath.Join(root, "Fallout-Terminal-linux-arm64.tar.gz")
		require.NoError(t, os.Remove(missing))
		nested := filepath.Join(root, "nested")
		require.NoError(t, os.Mkdir(nested, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(nested, filepath.Base(missing)), []byte("archive"), 0o600))
		require.Error(t, InspectReleaseInventory(t.Context(), root))
	})
}

func validReleaseInventoryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, target := range PortableTargets() {
		require.NoError(t, os.WriteFile(filepath.Join(root, target.ArchiveName()), []byte("archive"), 0o600))
	}
	return root
}

func releaseArchiveEntries(target Target) map[string][]byte {
	entries := map[string][]byte{
		path.Join(applicationName, target.ExecutablePath()): []byte("executable"),
	}
	for _, resource := range target.RequiredResourcePaths() {
		entries[path.Join(applicationName, resource)] = []byte("resource")
	}
	return entries
}

func writeReleaseArchiveFixture(t *testing.T, archivePath string, format ArchiveFormat, entries map[string][]byte) {
	t.Helper()
	var contents bytes.Buffer
	switch format {
	case ArchiveFormatZIP:
		writer := zip.NewWriter(&contents)
		for name, value := range entries {
			entry, err := writer.Create(name)
			require.NoError(t, err)
			_, err = entry.Write(value)
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())
	case ArchiveFormatTarGzip:
		gzipWriter := gzip.NewWriter(&contents)
		tarWriter := tar.NewWriter(gzipWriter)
		for name, value := range entries {
			require.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(value))}))
			_, err := io.Copy(tarWriter, bytes.NewReader(value))
			require.NoError(t, err)
		}
		require.NoError(t, tarWriter.Close())
		require.NoError(t, gzipWriter.Close())
	default:
		require.FailNow(t, "unsupported archive fixture format", string(format))
	}
	require.NoError(t, os.WriteFile(archivePath, contents.Bytes(), 0o600))
}
