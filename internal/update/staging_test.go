package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareApplicationUnitValidatesManifestBeforeAdjacentStaging(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	extractedRoot := filepath.Join(root, "download", "Fallout Terminal")
	installedUnit := filepath.Join(root, "installed", "Fallout Terminal")
	writeStagingFile(t, filepath.Join(installedUnit, "Fallout Terminal"), "old executable")
	writeExtractedApplicationFixture(t, extractedRoot, "2.5.0")

	candidate := UpdateCandidate{
		Version:  "2.5.0",
		Artifact: ReleaseAsset{Target: Target{OS: "linux", Arch: "amd64"}},
	}
	prepared, err := PrepareApplicationUnit(t.Context(), PrepareApplicationUnitRequest{
		Candidate: candidate, AttemptID: "attempt-42", ExtractedRoot: extractedRoot,
		InstalledUnit: installedUnit, InstalledLaunchRelativePath: "Fallout Terminal",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(prepared.StagedUnit)) })
	assert.Equal(t, "2.5.0", prepared.Version)
	assert.Equal(t, installedUnit, prepared.InstalledUnit)
	assert.Equal(t, filepath.Dir(installedUnit), filepath.Dir(prepared.StagedUnit))
	assert.FileExists(t, filepath.Join(prepared.StagedUnit, "Fallout Terminal"))
}

func TestPrepareApplicationUnitRejectsManifestIdentityBeforeStaging(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	extractedRoot := filepath.Join(root, "download", "Fallout Terminal")
	installedUnit := filepath.Join(root, "installed", "Fallout Terminal")
	writeStagingFile(t, filepath.Join(installedUnit, "Fallout Terminal"), "old executable")
	writeExtractedApplicationFixture(t, extractedRoot, "2.4.9")

	_, err := PrepareApplicationUnit(t.Context(), PrepareApplicationUnitRequest{
		Candidate: UpdateCandidate{
			Version:  "2.5.0",
			Artifact: ReleaseAsset{Target: Target{OS: "linux", Arch: "amd64"}},
		},
		AttemptID: "attempt-42", ExtractedRoot: extractedRoot,
		InstalledUnit: installedUnit, InstalledLaunchRelativePath: "Fallout Terminal",
	})
	require.ErrorContains(t, err, "manifest identity")
	entries, readErr := os.ReadDir(filepath.Dir(installedUnit))
	require.NoError(t, readErr)
	require.Len(t, entries, 1)
	assert.Equal(t, filepath.Base(installedUnit), entries[0].Name())
}

func TestValidateExtractedManifestRejectsEachValidationStage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*extractedArtifactManifest)
		wantError string
	}{
		{
			name: "identity",
			mutate: func(manifest *extractedArtifactManifest) {
				manifest.Product = "Other Product"
			},
			wantError: "manifest identity",
		},
		{
			name: "inventory shape",
			mutate: func(manifest *extractedArtifactManifest) {
				manifest.Files = manifest.Files[:len(manifest.Files)-1]
			},
			wantError: "manifest inventory",
		},
		{
			name: "invalid record",
			mutate: func(manifest *extractedArtifactManifest) {
				manifest.Files[0].Path = "../escape"
			},
			wantError: "invalid file record",
		},
		{
			name: "unsorted records",
			mutate: func(manifest *extractedArtifactManifest) {
				first := manifest.Files[0]
				manifest.Files[0] = manifest.Files[len(manifest.Files)-1]
				manifest.Files[len(manifest.Files)-1] = first
			},
			wantError: "duplicated or unsorted",
		},
		{
			name: "file evidence",
			mutate: func(manifest *extractedArtifactManifest) {
				manifest.Files[0].SHA256 = strings.Repeat("0", sha256.Size*2)
			},
			wantError: "file evidence",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := filepath.Join(t.TempDir(), "Fallout Terminal")
			writeExtractedApplicationFixture(t, root, "2.5.0")
			rewriteExtractedManifest(t, root, test.mutate)
			err := validateExtractedManifest(t.Context(), root, UpdateCandidate{
				Version: "2.5.0", Artifact: ReleaseAsset{Target: Target{OS: "linux", Arch: "amd64"}},
			})
			require.ErrorContains(t, err, test.wantError)
		})
	}
}

func TestSelectReplacementUnitForPortableTargets(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name           string
		target         Target
		executableName string
		wantLaunch     string
	}{
		{
			name:           "windows portable directory",
			target:         Target{OS: "windows", Arch: "amd64"},
			executableName: "Fallout Terminal.exe",
			wantLaunch:     "Fallout Terminal.exe",
		},
		{
			name:           "linux portable directory",
			target:         Target{OS: "linux", Arch: "arm64"},
			executableName: "Fallout Terminal",
			wantLaunch:     "Fallout Terminal",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			extractedRoot := filepath.Join(t.TempDir(), "Fallout Terminal")
			writeStagingFile(t, filepath.Join(extractedRoot, test.executableName), "executable")
			writeStagingFile(t, filepath.Join(extractedRoot, "artifact-manifest.json"), "{}")

			unit, launchRelativePath, err := selectReplacementUnit(extractedRoot, test.target)
			require.NoError(t, err)
			assert.Equal(t, extractedRoot, unit)
			assert.Equal(t, test.wantLaunch, launchRelativePath)
		})
	}
}

func TestSelectReplacementUnitForDarwinUsesNestedBundle(t *testing.T) {
	t.Parallel()

	extractedRoot := filepath.Join(t.TempDir(), "Fallout Terminal")
	bundle := filepath.Join(extractedRoot, "Fallout Terminal.app")
	writeStagingFile(t, filepath.Join(extractedRoot, "artifact-manifest.json"), "{}")
	writeStagingFile(t, filepath.Join(bundle, "Contents", "MacOS", "Fallout Terminal"), "executable")

	unit, launchRelativePath, err := selectReplacementUnit(
		extractedRoot,
		Target{OS: "darwin", Arch: "arm64"},
	)
	require.NoError(t, err)
	assert.Equal(t, bundle, unit)
	assert.Equal(t, filepath.Join("Contents", "MacOS", "Fallout Terminal"), launchRelativePath)
}

func TestSelectReplacementUnitRejectsUnsafeOrIncompatibleTrees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target Target
		build  func(*testing.T, string) string
	}{
		{
			name:   "empty root",
			target: Target{OS: "linux", Arch: "amd64"},
			build: func(*testing.T, string) string {
				return ""
			},
		},
		{
			name:   "filesystem root",
			target: Target{OS: "windows", Arch: "amd64"},
			build: func(*testing.T, string) string {
				return string(filepath.Separator)
			},
		},
		{
			name:   "unsupported target",
			target: Target{OS: "freebsd", Arch: "amd64"},
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "Fallout Terminal")
				writeStagingFile(t, filepath.Join(path, "Fallout Terminal"), "executable")
				return path
			},
		},
		{
			name:   "missing portable executable",
			target: Target{OS: "linux", Arch: "amd64"},
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "Fallout Terminal")
				require.NoError(t, os.MkdirAll(path, 0o755))
				return path
			},
		},
		{
			name:   "missing darwin bundle executable",
			target: Target{OS: "darwin", Arch: "arm64"},
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "Fallout Terminal")
				require.NoError(t, os.MkdirAll(filepath.Join(path, "Fallout Terminal.app"), 0o755))
				return path
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			extractedRoot := test.build(t, t.TempDir())
			_, _, err := selectReplacementUnit(extractedRoot, test.target)
			require.Error(t, err)
		})
	}
}

func TestStageApplicationUnitCopiesToSameVolumeSiblingAndSyncs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	installed := filepath.Join(root, "Fallout Terminal")
	source := filepath.Join(root, "extracted", "Fallout Terminal")
	writeStagingFile(t, filepath.Join(installed, "Fallout Terminal"), "old")
	writeStagingFile(t, filepath.Join(source, "Fallout Terminal"), "new")

	var copiedSource string
	var copiedDestination string
	var syncedPath string
	dependencies := stagingDependencies{
		copyTree: func(ctx context.Context, from, to string) error {
			require.NoError(t, ctx.Err())
			copiedSource = from
			copiedDestination = to
			writeStagingFile(t, filepath.Join(to, "Fallout Terminal"), "new")
			return nil
		},
		syncTree: func(ctx context.Context, path string) error {
			require.NoError(t, ctx.Err())
			syncedPath = path
			return nil
		},
		removeAll: os.RemoveAll,
	}

	prepared, err := stageApplicationUnit(t.Context(), stageRequest{
		AttemptID:          "attempt-42",
		Version:            "2.5.0",
		Target:             Target{OS: "linux", Arch: "amd64"},
		SourceUnit:         source,
		InstalledUnit:      installed,
		LaunchRelativePath: "Fallout Terminal",
	}, dependencies)
	require.NoError(t, err)

	assert.Equal(t, source, copiedSource)
	assert.Equal(t, prepared.StagedUnit, copiedDestination)
	assert.Equal(t, prepared.StagedUnit, syncedPath)
	assert.Equal(t, filepath.Dir(installed), filepath.Dir(prepared.StagedUnit))
	assert.NotEqual(t, installed, prepared.StagedUnit)
	assert.Contains(t, filepath.Base(prepared.StagedUnit), "attempt-42")
	assert.Equal(t, PreparedApplicationUnit{
		AttemptID:          "attempt-42",
		Version:            "2.5.0",
		Target:             Target{OS: "linux", Arch: "amd64"},
		InstalledUnit:      installed,
		StagedUnit:         prepared.StagedUnit,
		LaunchRelativePath: "Fallout Terminal",
	}, prepared)
	assert.FileExists(t, filepath.Join(prepared.StagedUnit, "Fallout Terminal"))
	assert.FileExists(t, filepath.Join(installed, "Fallout Terminal"))
}

func TestStageApplicationUnitCleansOnlyAttemptOwnedSiblingOnFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	installed := filepath.Join(root, "Fallout Terminal")
	source := filepath.Join(root, "extracted", "Fallout Terminal")
	writeStagingFile(t, filepath.Join(installed, "Fallout Terminal"), "old")
	writeStagingFile(t, filepath.Join(source, "Fallout Terminal"), "new")

	var removed []string
	dependencies := stagingDependencies{
		copyTree: func(_ context.Context, _, to string) error {
			writeStagingFile(t, filepath.Join(to, "Fallout Terminal"), "new")
			return nil
		},
		syncTree: func(context.Context, string) error {
			return errors.New("injected sync failure")
		},
		removeAll: func(path string) error {
			removed = append(removed, path)
			return os.RemoveAll(path)
		},
	}

	_, err := stageApplicationUnit(t.Context(), stageRequest{
		AttemptID:          "attempt-cleanup",
		Version:            "2.5.0",
		Target:             Target{OS: "linux", Arch: "amd64"},
		SourceUnit:         source,
		InstalledUnit:      installed,
		LaunchRelativePath: "Fallout Terminal",
	}, dependencies)
	require.ErrorContains(t, err, "sync")
	require.Len(t, removed, 1)
	assert.Equal(t, filepath.Dir(installed), filepath.Dir(removed[0]))
	assert.Contains(t, filepath.Base(removed[0]), "attempt-cleanup")
	assert.NoDirExists(t, removed[0])
	assert.FileExists(t, filepath.Join(installed, "Fallout Terminal"))
	assert.FileExists(t, filepath.Join(source, "Fallout Terminal"))
}

func TestStageApplicationUnitRejectsUnsafePathsBeforeCopy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	validSource := filepath.Join(root, "extracted", "Fallout Terminal")
	validInstalled := filepath.Join(root, "installed", "Fallout Terminal")
	writeStagingFile(t, filepath.Join(validSource, "Fallout Terminal"), "new")
	writeStagingFile(t, filepath.Join(validInstalled, "Fallout Terminal"), "old")

	tests := []struct {
		name    string
		request stageRequest
	}{
		{
			name: "empty attempt",
			request: stageRequest{
				SourceUnit: validSource, InstalledUnit: validInstalled, LaunchRelativePath: "Fallout Terminal",
			},
		},
		{
			name: "root source",
			request: stageRequest{
				AttemptID: "attempt", SourceUnit: string(filepath.Separator), InstalledUnit: validInstalled,
				LaunchRelativePath: "Fallout Terminal",
			},
		},
		{
			name: "root installed unit",
			request: stageRequest{
				AttemptID: "attempt", SourceUnit: validSource, InstalledUnit: string(filepath.Separator),
				LaunchRelativePath: "Fallout Terminal",
			},
		},
		{
			name: "traversing launch path",
			request: stageRequest{
				AttemptID: "attempt", SourceUnit: validSource, InstalledUnit: validInstalled,
				LaunchRelativePath: filepath.Join("..", "Fallout Terminal"),
			},
		},
		{
			name: "absolute launch path",
			request: stageRequest{
				AttemptID: "attempt", SourceUnit: validSource, InstalledUnit: validInstalled,
				LaunchRelativePath: filepath.Join(root, "Fallout Terminal"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			copies := 0
			dependencies := stagingDependencies{
				copyTree: func(context.Context, string, string) error {
					copies++
					return nil
				},
				syncTree:  func(context.Context, string) error { return nil },
				removeAll: os.RemoveAll,
			}

			_, err := stageApplicationUnit(t.Context(), test.request, dependencies)
			require.Error(t, err)
			assert.Zero(t, copies)
		})
	}
}

func writeStagingFile(t *testing.T, path, contents string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o755))
}

func writeExtractedApplicationFixture(t *testing.T, root, version string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(root, 0o755))
	required := requiredExtractedApplicationFiles(Target{OS: "linux", Arch: "amd64"})
	paths := make([]string, 0, len(required))
	for path := range required {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	files := make([]extractedArtifactManifestFile, 0, len(paths))
	for _, path := range paths {
		contents := []byte("fixture for " + path)
		mode := os.FileMode(0o444)
		if path == "Fallout Terminal" {
			mode = 0o755
		}
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, contents, mode))
		digest := sha256.Sum256(contents)
		files = append(files, extractedArtifactManifestFile{
			Path: path, Size: int64(len(contents)), Mode: fmt.Sprintf("%04o", mode),
			SHA256: hex.EncodeToString(digest[:]),
		})
	}
	manifest := extractedArtifactManifest{
		SchemaVersion:  2,
		Product:        "Fallout Terminal",
		Version:        version,
		SourceRevision: strings.Repeat("a", 40),
		Target:         extractedArtifactManifestTarget{OS: "linux", Arch: "amd64"},
		Runtime:        "GTK4/WebKitGTK 6.0 and Secret Service",
		Files:          files,
	}
	contents, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, artifactManifest), contents, 0o444))
}

func rewriteExtractedManifest(t *testing.T, root string, mutate func(*extractedArtifactManifest)) {
	t.Helper()

	path := filepath.Join(root, artifactManifest)
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	var manifest extractedArtifactManifest
	require.NoError(t, json.Unmarshal(contents, &manifest))
	mutate(&manifest)
	contents, err = json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(path, 0o644))
	require.NoError(t, os.WriteFile(path, contents, 0o444))
	require.NoError(t, os.Chmod(path, 0o444))
}

func assertSafeSiblingPath(t *testing.T, path, installed, attemptID string) {
	t.Helper()

	assert.Equal(t, filepath.Dir(installed), filepath.Dir(path))
	assert.NotEqual(t, installed, path)
	assert.True(t, strings.Contains(filepath.Base(path), attemptID), "path %q does not identify its attempt", path)
}
