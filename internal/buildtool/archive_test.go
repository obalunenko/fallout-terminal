package buildtool

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	archiveTestRevision = "0123456789abcdef0123456789abcdef01234567"
	archiveTestVersion  = "2.4.6-rc.2"
)

var archiveTestTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

type archiveTestFixture struct {
	files    []ArchiveFile
	contents map[string][]byte
}

type testObservedArchive struct {
	entries     []testObservedArchiveEntry
	gzipModTime time.Time
}

type testObservedArchiveEntry struct {
	name       string
	mode       fs.FileMode
	modTime    time.Time
	accessTime time.Time
	changeTime time.Time
	contents   []byte
}

type testManifestDocument struct {
	SchemaVersion  int                      `json:"schemaVersion"`
	Product        string                   `json:"product"`
	Version        string                   `json:"version"`
	SourceRevision string                   `json:"sourceRevision"`
	Target         testManifestTarget       `json:"target"`
	Runtime        string                   `json:"runtime"`
	Files          []testManifestFileRecord `json:"files"`
}

type testManifestTarget struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type testManifestFileRecord struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

func TestWritePortableArchiveIsDeterministic(t *testing.T) {
	t.Parallel()

	for _, target := range PortableTargets() {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			fixture := newArchiveTestFixture(t, root, target)
			firstOutput := filepath.Join(root, "first output")
			secondOutput := filepath.Join(root, "second output")
			version := archiveTestReleaseVersion(t)

			first, err := WritePortableArchive(t.Context(), firstOutput, target, version, archiveTestRevision, fixture.files)
			require.NoError(t, err)
			second, err := WritePortableArchive(t.Context(), secondOutput, target, version, archiveTestRevision, fixture.files)
			require.NoError(t, err)

			assert.Equal(t, filepath.Join(firstOutput, target.ArchiveName()), first.ArchivePath)
			assert.Equal(t, first.ArchivePath+".sha256", first.ChecksumPath)
			assert.Equal(t, filepath.Join(secondOutput, target.ArchiveName()), second.ArchivePath)
			assert.Equal(t, second.ArchivePath+".sha256", second.ChecksumPath)

			firstBytes := readArchiveTestFile(t, first.ArchivePath)
			secondBytes := readArchiveTestFile(t, second.ArchivePath)
			assert.Equal(t, firstBytes, secondBytes)
			assert.Equal(t, testArchiveSHA256Hex(firstBytes), first.SHA256)
			assert.Equal(t, first.SHA256, second.SHA256)
			assertArchiveChecksumSidecar(t, first)
			assertArchiveChecksumSidecar(t, second)

			observed := readObservedArchive(t, target.ArchiveFormat(), firstBytes)
			assertDeterministicArchiveEntries(t, target, fixture, observed)
		})
	}
}

func TestWritePortableArchiveRejectsCanceledContextBeforePublication(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := mustParseTarget(t, goosLinux, goarchAMD64)
	fixture := newArchiveTestFixture(t, root, target)
	output := filepath.Join(root, "canceled output")
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	cancel()

	_, err := WritePortableArchive(ctx, output, target, archiveTestReleaseVersion(t), archiveTestRevision, fixture.files)
	require.ErrorIs(t, err, context.Canceled)
	assert.NoFileExists(t, filepath.Join(output, target.ArchiveName()))
	assert.NoFileExists(t, filepath.Join(output, target.ArchiveName()+".sha256"))
}

func TestWritePortableArchiveRejectsInconsistentCanonicalVersion(t *testing.T) {
	t.Parallel()

	target := mustParseTarget(t, goosLinux, goarchAMD64)
	root := t.TempDir()
	fixture := newArchiveTestFixture(t, root, target)
	version := archiveTestReleaseVersion(t)
	version.Canonical = "2.4.7"

	_, err := WritePortableArchive(
		t.Context(), filepath.Join(root, "invalid-version"), target,
		version, archiveTestRevision, fixture.files,
	)
	require.ErrorContains(t, err, "inconsistent package version")
}

func TestWritePortableArchiveRejectsUnsafePathsAndTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		wantError string
		mutate    func(*testing.T, string, *archiveTestFixture)
	}{
		{
			name:      "absolute path",
			wantError: "absolute",
			mutate: func(_ *testing.T, _ string, fixture *archiveTestFixture) {
				fixture.files[0].Path = "/Fallout Terminal.exe"
			},
		},
		{
			name:      "Windows drive path",
			wantError: "drive",
			mutate: func(_ *testing.T, _ string, fixture *archiveTestFixture) {
				fixture.files[0].Path = `C:\Fallout Terminal.exe`
			},
		},
		{
			name:      "parent traversal",
			wantError: "parent traversal",
			mutate: func(_ *testing.T, _ string, fixture *archiveTestFixture) {
				fixture.files[0].Path = "../Fallout Terminal.exe"
			},
		},
		{
			name:      "duplicate path",
			wantError: "duplicate",
			mutate: func(_ *testing.T, _ string, fixture *archiveTestFixture) {
				fixture.files = append(fixture.files, fixture.files[0])
			},
		},
		{
			name:      "duplicate normalized slash path",
			wantError: "duplicate",
			mutate: func(_ *testing.T, _ string, fixture *archiveTestFixture) {
				duplicate := fixture.files[1]
				duplicate.Path = strings.ReplaceAll(duplicate.Path, "/", `\`)
				fixture.files = append(fixture.files, duplicate)
			},
		},
		{
			name:      "symbolic link source",
			wantError: "symbolic link",
			mutate: func(t *testing.T, root string, fixture *archiveTestFixture) {
				link := filepath.Join(root, "linked executable")
				if err := os.Symlink(fixture.files[0].SourcePath, link); err != nil {
					t.Skipf("symbolic links unavailable on this host: %v", err)
				}
				fixture.files[0].SourcePath = link
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			target := mustParseTarget(t, goosLinux, goarchAMD64)
			fixture := newArchiveTestFixture(t, root, target)
			test.mutate(t, root, &fixture)
			output := filepath.Join(root, "rejected output")

			_, err := WritePortableArchive(
				t.Context(), output, target, archiveTestReleaseVersion(t), archiveTestRevision, fixture.files,
			)
			require.ErrorContains(t, err, test.wantError)
			assert.NoFileExists(t, filepath.Join(output, target.ArchiveName()))
			assert.NoFileExists(t, filepath.Join(output, target.ArchiveName()+".sha256"))
		})
	}
}

func TestWritePortableArchiveRequiresExactInventory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*archiveTestFixture)
	}{
		{
			name: "missing bundled demo",
			mutate: func(fixture *archiveTestFixture) {
				fixture.files = fixture.files[:len(fixture.files)-1]
			},
		},
		{
			name: "extra provider executable",
			mutate: func(fixture *archiveTestFixture) {
				fixture.files = append(fixture.files, ArchiveFile{
					Path:       "resources/tools/go",
					SourcePath: fixture.files[0].SourcePath,
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			target := mustParseTarget(t, goosWindows, goarchARM64)
			fixture := newArchiveTestFixture(t, root, target)
			test.mutate(&fixture)

			_, err := WritePortableArchive(
				t.Context(), filepath.Join(root, "output"), target,
				archiveTestReleaseVersion(t), archiveTestRevision, fixture.files,
			)
			require.ErrorContains(t, err, "exact payload inventory")
		})
	}
}

func TestWritePortableArchiveExcludesUserOwnedAndSecretBearingFiles(t *testing.T) {
	t.Parallel()

	for _, target := range PortableTargets() {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			fixture := newArchiveTestFixture(t, root, target)
			result, err := WritePortableArchive(
				t.Context(), filepath.Join(root, "output"), target,
				archiveTestReleaseVersion(t), archiveTestRevision, fixture.files,
			)
			require.NoError(t, err)

			observed := readObservedArchive(t, target.ArchiveFormat(), readArchiveTestFile(t, result.ArchivePath))
			for _, entry := range observed.entries {
				lower := strings.ToLower(entry.name)
				for _, forbidden := range []string{
					"credentials", "private-settings", "plaintext", "secret", "verification-record", "user-sessions",
				} {
					assert.NotContains(t, lower, forbidden)
				}
			}
		})
	}
}

func TestWritePortableDarwinArchiveRejectsUnsafePathsAndCancellation(t *testing.T) {
	t.Parallel()

	target := mustParseTarget(t, goosDarwin, goarchARM64)
	root := t.TempDir()
	fixture := newArchiveTestFixture(t, root, target)
	fixture.files[0].Path = "../Fallout Terminal.app/Contents/MacOS/Fallout Terminal"
	_, err := WritePortableArchive(
		t.Context(), filepath.Join(root, "unsafe"), target,
		archiveTestReleaseVersion(t), archiveTestRevision, fixture.files,
	)
	require.ErrorContains(t, err, "parent traversal")

	fixture = newArchiveTestFixture(t, root, target)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	cancel()
	_, err = WritePortableArchive(
		ctx, filepath.Join(root, "canceled"), target,
		archiveTestReleaseVersion(t), archiveTestRevision, fixture.files,
	)
	require.ErrorIs(t, err, context.Canceled)
}

func assertDeterministicArchiveEntries(
	t *testing.T,
	target Target,
	fixture archiveTestFixture,
	observed testObservedArchive,
) {
	t.Helper()

	names := make([]string, len(observed.entries))
	entries := make(map[string]testObservedArchiveEntry, len(observed.entries))
	for index, entry := range observed.entries {
		names[index] = entry.name
		entries[entry.name] = entry
		assert.True(t, entry.modTime.Equal(archiveTestTime), "%s timestamp is %s", entry.name, entry.modTime)
		assert.True(t, entry.accessTime.IsZero(), "%s has an access timestamp", entry.name)
		assert.True(t, entry.changeTime.IsZero(), "%s has a change timestamp", entry.name)
	}
	assert.True(t, sort.StringsAreSorted(names))
	assert.Equal(t, expectedArchivePaths(target), names)
	if target.ArchiveFormat() == ArchiveFormatTarGzip {
		assert.True(t, observed.gzipModTime.Equal(archiveTestTime))
	}

	manifestPath := path.Join(applicationName, "artifact-manifest.json")
	manifestEntry := entries[manifestPath]
	assert.Equal(t, fs.FileMode(0o444), manifestEntry.mode.Perm())
	manifest := decodeArchiveManifest(t, manifestEntry.contents)
	assert.Equal(t, 2, manifest.SchemaVersion)
	assert.Equal(t, applicationName, manifest.Product)
	assert.Equal(t, archiveTestVersion, manifest.Version)
	assert.Equal(t, archiveTestRevision, manifest.SourceRevision)
	assert.Equal(t, testManifestTarget{OS: target.OS(), Arch: target.Arch()}, manifest.Target)
	assert.Equal(t, target.NativeRuntime(), manifest.Runtime)

	wantManifestFiles := expectedManifestFiles(target, fixture.contents)
	assert.Equal(t, wantManifestFiles, manifest.Files)
	for _, file := range manifest.Files {
		assert.NotEqual(t, "artifact-manifest.json", file.Path)
		assert.False(t, filepath.IsAbs(file.Path))
		assert.NotContains(t, file.Path, `\`)
	}

	for relative, contents := range fixture.contents {
		entry := entries[path.Join(applicationName, relative)]
		assert.Equal(t, contents, entry.contents)
		assert.Equal(t, testNormalizedArchiveMode(target, relative), entry.mode.Perm())
	}
}

func newArchiveTestFixture(t *testing.T, root string, target Target) archiveTestFixture {
	t.Helper()

	contents := map[string][]byte{
		target.ExecutablePath(): []byte("target executable for " + target.String()),
	}
	for _, resource := range target.RequiredResourcePaths() {
		if resource != artifactManifestFilename {
			contents[resource] = []byte("reviewed resource for " + resource)
		}
	}
	paths := make([]string, 0, len(contents))
	for archivePath := range contents {
		paths = append(paths, archivePath)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))

	files := make([]ArchiveFile, 0, len(paths))
	for index, archivePath := range paths {
		source := filepath.Join(root, "sources", fmt.Sprintf("%02d", index))
		require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
		require.NoError(t, os.WriteFile(source, contents[archivePath], 0o600))
		files = append(files, ArchiveFile{Path: archivePath, SourcePath: source})
	}
	return archiveTestFixture{files: files, contents: contents}
}

func archiveTestReleaseVersion(t *testing.T) ReleaseVersion {
	t.Helper()

	version, err := ResolveBuildVersion(archiveTestVersion)
	require.NoError(t, err)
	return version
}

func expectedArchivePaths(target Target) []string {
	paths := []string{path.Join(applicationName, target.ExecutablePath())}
	for _, resource := range target.RequiredResourcePaths() {
		paths = append(paths, path.Join(applicationName, resource))
	}
	sort.Strings(paths)
	return paths
}

func TestSchemaV2ArchiveInventoryRemainsCompatibleWithV2(t *testing.T) {
	t.Parallel()

	for _, target := range PortableTargets() {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()

			want := []string{
				artifactManifestFilename,
				"resources/THIRD_PARTY_NOTICES.md",
				"resources/appicon.png",
				"resources/sessions/demo-players.json",
				"resources/sessions/demo.json",
			}
			if target.OS() == goosDarwin {
				want = []string{
					artifactManifestFilename,
					"Fallout Terminal.app/Contents/Info.plist",
					"Fallout Terminal.app/Contents/Resources/THIRD_PARTY_NOTICES.md",
					"Fallout Terminal.app/Contents/Resources/icon.icns",
					"Fallout Terminal.app/Contents/Resources/sessions/demo-players.json",
					"Fallout Terminal.app/Contents/Resources/sessions/demo.json",
				}
			}

			assert.Equal(t, want, target.RequiredResourcePaths())
		})
	}
}

func expectedManifestFiles(target Target, contents map[string][]byte) []testManifestFileRecord {
	paths := make([]string, 0, len(contents))
	for filePath := range contents {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)

	files := make([]testManifestFileRecord, 0, len(paths))
	for _, filePath := range paths {
		mode := testNormalizedArchiveMode(target, filePath)
		files = append(files, testManifestFileRecord{
			Path:   filePath,
			Size:   int64(len(contents[filePath])),
			Mode:   fmt.Sprintf("%04o", mode.Perm()),
			SHA256: testArchiveSHA256Hex(contents[filePath]),
		})
	}
	return files
}

func testNormalizedArchiveMode(target Target, filePath string) fs.FileMode {
	if filePath == target.ExecutablePath() {
		return 0o755
	}
	return 0o444
}

func readObservedArchive(t *testing.T, format ArchiveFormat, contents []byte) testObservedArchive {
	t.Helper()

	switch format {
	case ArchiveFormatZIP:
		reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
		require.NoError(t, err)
		observed := testObservedArchive{entries: make([]testObservedArchiveEntry, 0, len(reader.File))}
		for _, file := range reader.File {
			entryReader, err := file.Open()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, entryReader.Close()) })
			entryContents, err := io.ReadAll(entryReader)
			require.NoError(t, err)
			observed.entries = append(observed.entries, testObservedArchiveEntry{
				name: file.Name, mode: file.Mode(), modTime: file.Modified.UTC(), contents: entryContents,
			})
		}
		return observed
	case ArchiveFormatTarGzip:
		gzipReader, err := gzip.NewReader(bytes.NewReader(contents))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, gzipReader.Close()) })
		observed := testObservedArchive{gzipModTime: gzipReader.ModTime.UTC()}
		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			entryContents, err := io.ReadAll(tarReader)
			require.NoError(t, err)
			observed.entries = append(observed.entries, testObservedArchiveEntry{
				name: header.Name, mode: fs.FileMode(header.Mode), modTime: header.ModTime.UTC(),
				accessTime: header.AccessTime, changeTime: header.ChangeTime, contents: entryContents,
			})
		}
		return observed
	default:
		t.Fatalf("unsupported archive test format %q", format)
		return testObservedArchive{}
	}
}

func decodeArchiveManifest(t *testing.T, contents []byte) testManifestDocument {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var manifest testManifestDocument
	require.NoError(t, decoder.Decode(&manifest))
	var trailing any
	require.ErrorIs(t, decoder.Decode(&trailing), io.EOF)
	return manifest
}

func assertArchiveChecksumSidecar(t *testing.T, result ArchiveResult) {
	t.Helper()

	archiveContents := readArchiveTestFile(t, result.ArchivePath)
	wantHash := testArchiveSHA256Hex(archiveContents)
	assert.Equal(t, wantHash, result.SHA256)
	assert.Equal(t,
		wantHash+"  "+filepath.Base(result.ArchivePath)+"\n",
		string(readArchiveTestFile(t, result.ChecksumPath)),
	)
}

func readArchiveTestFile(t *testing.T, filePath string) []byte {
	t.Helper()

	contents, err := os.ReadFile(filePath)
	require.NoError(t, err)
	return contents
}

func testArchiveSHA256Hex(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}
