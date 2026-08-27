package buildtool

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const releaseManifestTestVersion = "2.7.1"

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

func TestParseReleaseTagDerivesCanonicalRepresentations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		tag             string
		canonical       string
		numericCore     string
		numericFourPart string
		prerelease      bool
		wantErr         bool
	}{
		{
			name:            "stable",
			tag:             "v2.0.0",
			canonical:       "2.0.0",
			numericCore:     "2.0.0",
			numericFourPart: "2.0.0.0",
		},
		{
			name:            "prerelease",
			tag:             "v2.1.3-beta.1",
			canonical:       "2.1.3-beta.1",
			numericCore:     "2.1.3",
			numericFourPart: "2.1.3.0",
			prerelease:      true,
		},
		{name: "non-v2 major zero", tag: "v0.0.0-rc.1", wantErr: true},
		{name: "non-v2 major one", tag: "v1.2.3", wantErr: true},
		{name: "non-v2 major three", tag: "v3.0.0", wantErr: true},
		{name: "missing prefix", tag: "2.0.0", wantErr: true},
		{name: "leading-zero major", tag: "v02.0.0", wantErr: true},
		{name: "leading-zero minor", tag: "v2.00.0", wantErr: true},
		{name: "leading-zero patch", tag: "v2.0.00", wantErr: true},
		{name: "leading-zero numeric prerelease", tag: "v2.0.0-rc.01", wantErr: true},
		{name: "build metadata", tag: "v2.0.0+build.1", wantErr: true},
		{name: "empty prerelease", tag: "v2.0.0-", wantErr: true},
		{name: "surrounding whitespace", tag: " v2.0.0 ", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			version, err := ParseReleaseTag(test.tag)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.canonical, version.Canonical)
			assert.Equal(t, test.numericCore, version.NumericCore)
			assert.Equal(t, test.numericFourPart, version.NumericFourPart)
			assert.Equal(t, test.prerelease, version.Prerelease)
			assert.True(t, version.IsRelease)
		})
	}
}

func TestResolveBuildVersionSelectsStrictReleaseOrLocalMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		canonical       string
		numericCore     string
		numericFourPart string
		prerelease      bool
		isRelease       bool
		wantErr         bool
	}{
		{
			name:            "empty local version",
			canonical:       "development",
			numericCore:     "0.0.0",
			numericFourPart: "0.0.0.0",
		},
		{
			name:            "stable release version",
			input:           "2.0.0",
			canonical:       "2.0.0",
			numericCore:     "2.0.0",
			numericFourPart: "2.0.0.0",
			isRelease:       true,
		},
		{
			name:            "prerelease version",
			input:           "2.0.0-rc.1",
			canonical:       "2.0.0-rc.1",
			numericCore:     "2.0.0",
			numericFourPart: "2.0.0.0",
			prerelease:      true,
			isRelease:       true,
		},
		{name: "explicit development is not release input", input: "development", wantErr: true},
		{name: "tag prefix is forbidden", input: "v2.0.0", wantErr: true},
		{name: "non-v2 major", input: "1.2.3", wantErr: true},
		{name: "future major", input: "3.0.0", wantErr: true},
		{name: "leading-zero major", input: "02.0.0", wantErr: true},
		{name: "leading-zero minor", input: "2.00.0", wantErr: true},
		{name: "leading-zero patch", input: "2.0.00", wantErr: true},
		{name: "leading-zero numeric prerelease", input: "2.0.0-rc.01", wantErr: true},
		{name: "build metadata", input: "2.0.0+build.1", wantErr: true},
		{name: "missing patch", input: "2.0", wantErr: true},
		{name: "empty prerelease", input: "2.0.0-", wantErr: true},
		{name: "surrounding whitespace", input: " 2.0.0 ", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			version, err := ResolveBuildVersion(test.input)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.canonical, version.Canonical)
			assert.Equal(t, test.numericCore, version.NumericCore)
			assert.Equal(t, test.numericFourPart, version.NumericFourPart)
			assert.Equal(t, test.prerelease, version.Prerelease)
			assert.Equal(t, test.isRelease, version.IsRelease)
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

func TestInspectReleaseArchiveVersionAcceptsMatchingManifestV2AcrossPortableTargets(t *testing.T) {
	t.Parallel()

	expected, err := ResolveBuildVersion(releaseManifestTestVersion)
	require.NoError(t, err)
	for _, target := range PortableTargets() {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()

			archivePath := writeManifestV2ReleaseFixture(t, target, expected, nil, nil)
			probe := func(
				ctx context.Context,
				gotTarget Target,
				_ string,
				arguments []string,
			) (NativeVersionEvidence, error) {
				require.NoError(t, ctx.Err())
				assert.Equal(t, target, gotTarget)
				assert.Equal(t, []string{"--version"}, arguments)
				return matchingNativeVersionEvidence(target, expected), nil
			}

			require.NoError(t, inspectReleaseArchiveVersion(t.Context(), target, archivePath, expected, probe))
		})
	}
}

func TestInspectReleaseArchiveVersionRejectsManifestOrPackageDisagreement(t *testing.T) {
	t.Parallel()

	target := mustParseTarget(t, goosLinux, goarchAMD64)
	expected, err := ResolveBuildVersion(releaseManifestTestVersion)
	require.NoError(t, err)
	tests := []struct {
		name           string
		mutateManifest func(*testManifestDocument)
		mutatePackage  func(map[string][]byte)
	}{
		{
			name: "schema v1",
			mutateManifest: func(manifest *testManifestDocument) {
				manifest.SchemaVersion = 1
			},
		},
		{
			name: "canonical version mismatch",
			mutateManifest: func(manifest *testManifestDocument) {
				manifest.Version = "2.7.2"
			},
		},
		{
			name: "development version",
			mutateManifest: func(manifest *testManifestDocument) {
				manifest.Version = developmentBuildVersion
			},
		},
		{
			name: "target mismatch",
			mutateManifest: func(manifest *testManifestDocument) {
				manifest.Target.Arch = goarchARM64
			},
		},
		{
			name: "manifest inventory digest mismatch",
			mutateManifest: func(manifest *testManifestDocument) {
				manifest.Files[0].SHA256 = strings.Repeat("0", 64)
			},
		},
		{
			name: "unlisted package entry",
			mutatePackage: func(entries map[string][]byte) {
				entries["unexpected-provider-binary"] = []byte("must not ship")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			archivePath := writeManifestV2ReleaseFixture(
				t, target, expected, test.mutateManifest, test.mutatePackage,
			)
			probe := func(_ context.Context, _ Target, _ string, _ []string) (NativeVersionEvidence, error) {
				return matchingNativeVersionEvidence(target, expected), nil
			}

			require.Error(t, inspectReleaseArchiveVersion(t.Context(), target, archivePath, expected, probe))
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

func writeManifestV2ReleaseFixture(
	t *testing.T,
	target Target,
	version ReleaseVersion,
	mutateManifest func(*testManifestDocument),
	mutatePackage func(map[string][]byte),
) string {
	t.Helper()

	payload := map[string][]byte{
		target.ExecutablePath(): []byte("native executable fixture for " + target.String()),
	}
	for _, resource := range target.RequiredResourcePaths() {
		if resource == artifactManifestFilename {
			continue
		}
		payload[resource] = []byte("resource fixture for " + resource)
	}
	if target.OS() == goosDarwin {
		payload["Fallout Terminal.app/Contents/Info.plist"] = darwinVersionPlist(
			version.Canonical, version.NumericCore,
		)
	}

	manifest := testManifestDocument{
		SchemaVersion:  2,
		Product:        applicationName,
		Version:        version.Canonical,
		SourceRevision: archiveTestRevision,
		Target:         testManifestTarget{OS: target.OS(), Arch: target.Arch()},
		Runtime:        target.NativeRuntime(),
		Files:          expectedManifestFiles(target, payload),
	}
	if mutateManifest != nil {
		mutateManifest(&manifest)
	}
	manifestContents, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	manifestContents = append(manifestContents, '\n')
	if mutatePackage != nil {
		mutatePackage(payload)
	}

	entries := make(map[string][]byte, len(payload)+1)
	for relative, contents := range payload {
		entries[path.Join(applicationName, relative)] = contents
	}
	entries[path.Join(applicationName, artifactManifestFilename)] = manifestContents
	root := t.TempDir()
	archivePath := filepath.Join(root, target.ArchiveName())
	writeReleaseArchiveFixtureWithTargetModes(t, archivePath, target, entries)
	return archivePath
}

func matchingNativeVersionEvidence(target Target, version ReleaseVersion) NativeVersionEvidence {
	evidence := NativeVersionEvidence{ExecutableOutput: version.Canonical + "\n"}
	if target.OS() == goosWindows {
		evidence.FileVersion = version.Canonical
		evidence.ProductVersion = version.Canonical
		evidence.FixedFileVersion = version.NumericFourPart
		evidence.FixedProductVersion = version.NumericFourPart
		evidence.AssemblyVersion = version.NumericFourPart
	}
	return evidence
}

func writeReleaseArchiveFixtureWithTargetModes(
	t *testing.T,
	archivePath string,
	target Target,
	entries map[string][]byte,
) {
	t.Helper()

	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	var contents bytes.Buffer
	switch target.ArchiveFormat() {
	case ArchiveFormatZIP:
		writer := zip.NewWriter(&contents)
		for _, name := range names {
			value := entries[name]
			header := &zip.FileHeader{Name: name, Method: zip.Deflate}
			header.SetMode(releaseArchiveFixtureMode(target, name))
			entry, err := writer.CreateHeader(header)
			require.NoError(t, err)
			_, err = entry.Write(value)
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())
	case ArchiveFormatTarGzip:
		gzipWriter := gzip.NewWriter(&contents)
		tarWriter := tar.NewWriter(gzipWriter)
		for _, name := range names {
			value := entries[name]
			header := &tar.Header{
				Name: name,
				Mode: int64(releaseArchiveFixtureMode(target, name).Perm()),
				Size: int64(len(value)),
			}
			require.NoError(t, tarWriter.WriteHeader(header))
			_, err := io.Copy(tarWriter, bytes.NewReader(value))
			require.NoError(t, err)
		}
		require.NoError(t, tarWriter.Close())
		require.NoError(t, gzipWriter.Close())
	default:
		require.FailNow(t, "unsupported archive fixture format", string(target.ArchiveFormat()))
	}
	require.NoError(t, os.WriteFile(archivePath, contents.Bytes(), 0o600))
}

func releaseArchiveFixtureMode(target Target, name string) fs.FileMode {
	relative := strings.TrimPrefix(name, applicationName+"/")
	if relative == target.ExecutablePath() {
		return 0o755
	}
	return 0o444
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
