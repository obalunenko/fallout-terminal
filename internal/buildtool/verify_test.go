package buildtool

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const verificationSourceRevision = "0123456789abcdef0123456789abcdef01234567"

type verificationFile struct {
	contents []byte
	mode     os.FileMode
}

type verificationManifest struct {
	SchemaVersion  int                        `json:"schemaVersion"`
	Product        string                     `json:"product"`
	SourceRevision string                     `json:"sourceRevision"`
	Target         verificationManifestTarget `json:"target"`
	Runtime        string                     `json:"runtime"`
	Files          []verificationManifestFile `json:"files"`
}

type verificationManifestTarget struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type verificationManifestFile struct {
	Path   string `json:"path"`
	Size   int    `json:"size"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

type verificationFixture struct {
	target        Target
	archiveName   string
	files         map[string]verificationFile
	manifest      verificationManifest
	manifestBytes []byte
	omitManifest  bool
	archiveBytes  []byte
	checksumText  *string
	checksumName  string
	archivePath   string
	checksumPath  string
	archiveDigest string
}

func TestVerifyArtifactAcceptsEachPortableTargetAndReturnsImmutableIdentity(t *testing.T) {
	t.Parallel()

	for _, target := range portableTestTargets(t) {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()

			fixture := newVerificationFixture(t, target)
			fixture.write(t)

			verified, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
			require.NoError(t, err)
			assert.Equal(t, target, verified.Target())
			assert.Equal(t, verificationSourceRevision, verified.SourceRevision())
			assert.Equal(t, target.ArchiveName(), verified.ArchiveName())
			assert.Equal(t, fixture.archiveDigest, verified.Checksum())
		})
	}
}

func TestVerifyArtifactRejectsPEAndELFIdentityMismatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		targetOS   string
		targetArch string
		binary     []byte
		want       string
	}{
		{name: "Windows AMD64 carrying ARM64 PE", targetOS: goosWindows, targetArch: goarchAMD64, binary: verificationPE(goarchARM64), want: "amd64"},
		{name: "Windows ARM64 carrying AMD64 PE", targetOS: goosWindows, targetArch: goarchARM64, binary: verificationPE(goarchAMD64), want: "arm64"},
		{name: "Linux AMD64 carrying ARM64 ELF", targetOS: goosLinux, targetArch: goarchAMD64, binary: verificationELF(goarchARM64), want: "amd64"},
		{name: "Linux ARM64 carrying AMD64 ELF", targetOS: goosLinux, targetArch: goarchARM64, binary: verificationELF(goarchAMD64), want: "arm64"},
		{name: "Windows carrying non-PE executable", targetOS: goosWindows, targetArch: goarchAMD64, binary: verificationELF(goarchAMD64), want: "PE"},
		{name: "Linux carrying non-ELF executable", targetOS: goosLinux, targetArch: goarchAMD64, binary: verificationPE(goarchAMD64), want: "ELF"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target := mustParseTarget(t, test.targetOS, test.targetArch)
			fixture := newVerificationFixture(t, target)
			fixture.replaceFile(target.ExecutableName(), test.binary, 0o755)
			fixture.write(t)

			_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
			require.Error(t, err)
			assert.ErrorContains(t, err, target.ExecutableName())
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestVerifyArtifactRejectsProductTargetRuntimeAndArchiveNameMismatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*verificationFixture)
		want   string
	}{
		{
			name: "wrong product",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Product = "Another Product"
			},
			want: applicationName,
		},
		{
			name: "wrong target operating system",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Target.OS = goosLinux
			},
			want: "windows/amd64",
		},
		{
			name: "wrong target architecture",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Target.Arch = goarchARM64
			},
			want: "windows/amd64",
		},
		{
			name: "wrong native runtime",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Runtime = "GTK3"
			},
			want: "WebView2",
		},
		{
			name: "invalid source revision",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.SourceRevision = "not-a-full-source-revision"
			},
			want: "sourceRevision",
		},
		{
			name: "unstable archive name",
			mutate: func(fixture *verificationFixture) {
				fixture.archiveName = "fallout-windows-amd64.zip"
			},
			want: "Fallout-Terminal-windows-amd64.zip",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target := mustParseTarget(t, goosWindows, goarchAMD64)
			fixture := newVerificationFixture(t, target)
			test.mutate(fixture)
			fixture.write(t)

			_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
			require.Error(t, err)
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestVerifyArtifactRejectsMissingRequiredPayloadAndExecutableMode(t *testing.T) {
	t.Parallel()

	required := []string{
		filepath.ToSlash(filepath.Join("resources", "appicon.png")),
		filepath.ToSlash(filepath.Join("resources", "THIRD_PARTY_NOTICES.md")),
		filepath.ToSlash(filepath.Join("resources", "sessions", "demo.json")),
		filepath.ToSlash(filepath.Join("resources", "sessions", "demo-players.json")),
	}
	for _, path := range required {
		t.Run("missing "+path, func(t *testing.T) {
			t.Parallel()

			target := mustParseTarget(t, goosLinux, goarchAMD64)
			fixture := newVerificationFixture(t, target)
			fixture.removeFile(path)
			fixture.write(t)

			_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
			require.Error(t, err)
			assert.ErrorContains(t, err, path)
		})
	}

	t.Run("missing executable", func(t *testing.T) {
		t.Parallel()

		target := mustParseTarget(t, goosLinux, goarchAMD64)
		fixture := newVerificationFixture(t, target)
		fixture.removeFile(target.ExecutableName())
		fixture.write(t)

		_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
		require.Error(t, err)
		assert.ErrorContains(t, err, target.ExecutableName())
	})

	t.Run("Linux executable is not runnable", func(t *testing.T) {
		t.Parallel()

		target := mustParseTarget(t, goosLinux, goarchARM64)
		fixture := newVerificationFixture(t, target)
		fixture.replaceFile(target.ExecutableName(), verificationELF(goarchARM64), 0o644)
		fixture.write(t)

		_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
		require.Error(t, err)
		assert.ErrorContains(t, err, "0755")
	})

	t.Run("missing artifact manifest", func(t *testing.T) {
		t.Parallel()

		target := mustParseTarget(t, goosWindows, goarchARM64)
		fixture := newVerificationFixture(t, target)
		fixture.omitManifest = true
		fixture.write(t)

		_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
		require.Error(t, err)
		assert.ErrorContains(t, err, "artifact-manifest.json")
	})
}

func TestVerifyArtifactRejectsChecksumAndArchiveCorruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*verificationFixture)
		want   string
	}{
		{
			name: "checksum digest mismatch",
			mutate: func(fixture *verificationFixture) {
				text := strings.Repeat("0", sha256.Size*2) + "  " + fixture.archiveName + "\n"
				fixture.checksumText = &text
			},
			want: "checksum",
		},
		{
			name: "checksum names another archive",
			mutate: func(fixture *verificationFixture) {
				fixture.checksumName = "another.zip"
			},
			want: "another.zip",
		},
		{
			name: "malformed checksum",
			mutate: func(fixture *verificationFixture) {
				text := "not-a-sha256\n"
				fixture.checksumText = &text
			},
			want: "checksum",
		},
		{
			name: "corrupt ZIP with matching checksum",
			mutate: func(fixture *verificationFixture) {
				fixture.archiveBytes = []byte("not a ZIP archive")
			},
			want: "ZIP",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target := mustParseTarget(t, goosWindows, goarchAMD64)
			fixture := newVerificationFixture(t, target)
			test.mutate(fixture)
			fixture.write(t)

			_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
			require.Error(t, err)
			assert.ErrorContains(t, err, test.want)
		})
	}

	t.Run("corrupt TAR.GZ with matching checksum", func(t *testing.T) {
		t.Parallel()

		target := mustParseTarget(t, goosLinux, goarchARM64)
		fixture := newVerificationFixture(t, target)
		fixture.archiveBytes = []byte("not a TAR.GZ archive")
		fixture.write(t)

		_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
		require.Error(t, err)
		assert.ErrorContains(t, err, "TAR.GZ")
	})
}

func TestVerifyArtifactRejectsCorruptOrMismatchedManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*verificationFixture)
		want   string
	}{
		{
			name: "invalid JSON",
			mutate: func(fixture *verificationFixture) {
				fixture.manifestBytes = []byte("{not-json")
			},
			want: "artifact-manifest.json",
		},
		{
			name: "wrong file hash",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Files[0].SHA256 = strings.Repeat("0", sha256.Size*2)
			},
			want: "sha256",
		},
		{
			name: "wrong file size",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Files[0].Size++
			},
			want: "size",
		},
		{
			name: "wrong file mode",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Files[0].Mode = "0600"
			},
			want: "mode",
		},
		{
			name: "unsorted file manifest",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Files[0], fixture.manifest.Files[1] = fixture.manifest.Files[1], fixture.manifest.Files[0]
			},
			want: "sorted",
		},
		{
			name: "duplicate manifest path",
			mutate: func(fixture *verificationFixture) {
				fixture.manifest.Files[1].Path = fixture.manifest.Files[0].Path
			},
			want: "duplicate",
		},
		{
			name: "archive file absent from manifest",
			mutate: func(fixture *verificationFixture) {
				fixture.files["resources/unlisted.txt"] = verificationFile{contents: []byte("unlisted"), mode: 0o444}
			},
			want: "resources/unlisted.txt",
		},
		{
			name: "unsafe archive path",
			mutate: func(fixture *verificationFixture) {
				fixture.files["../escape.txt"] = verificationFile{contents: []byte("escape"), mode: 0o444}
			},
			want: "unsafe",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target := mustParseTarget(t, goosLinux, goarchAMD64)
			fixture := newVerificationFixture(t, target)
			test.mutate(fixture)
			fixture.write(t)

			_, err := VerifyArtifact(t.Context(), fixture.archivePath, fixture.checksumPath, target)
			require.Error(t, err)
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestVerifyArtifactStopsOnCanceledContextBeforeReadingFiles(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	cancel()
	target := mustParseTarget(t, goosLinux, goarchAMD64)

	_, err := VerifyArtifact(ctx, filepath.Join(t.TempDir(), target.ArchiveName()), "missing.sha256", target)
	require.ErrorIs(t, err, context.Canceled)
}

func newVerificationFixture(t *testing.T, target Target) *verificationFixture {
	t.Helper()

	executable := verificationELF(target.Arch())
	if target.OS() == goosWindows {
		executable = verificationPE(target.Arch())
	}
	fixture := &verificationFixture{
		target:      target,
		archiveName: target.ArchiveName(),
		files: map[string]verificationFile{
			target.ExecutableName():                {contents: executable, mode: 0o755},
			"resources/appicon.png":                {contents: []byte("portable-icon"), mode: 0o444},
			"resources/THIRD_PARTY_NOTICES.md":     {contents: []byte("portable notices\n"), mode: 0o444},
			"resources/sessions/demo.json":         {contents: []byte(`{"version":1,"name":"demo"}`), mode: 0o444},
			"resources/sessions/demo-players.json": {contents: []byte(`{"version":1,"name":"players"}`), mode: 0o444},
		},
		manifest: verificationManifest{
			SchemaVersion:  1,
			Product:        applicationName,
			SourceRevision: verificationSourceRevision,
			Target:         verificationManifestTarget{OS: target.OS(), Arch: target.Arch()},
			Runtime:        target.NativeRuntime(),
		},
	}
	fixture.refreshManifest()
	return fixture
}

func (fixture *verificationFixture) replaceFile(path string, contents []byte, mode os.FileMode) {
	fixture.files[path] = verificationFile{contents: append([]byte(nil), contents...), mode: mode}
	fixture.refreshManifest()
}

func (fixture *verificationFixture) removeFile(path string) {
	delete(fixture.files, path)
	fixture.refreshManifest()
}

func (fixture *verificationFixture) refreshManifest() {
	paths := make([]string, 0, len(fixture.files))
	for path := range fixture.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	fixture.manifest.Files = make([]verificationManifestFile, 0, len(paths))
	for _, path := range paths {
		file := fixture.files[path]
		digest := sha256.Sum256(file.contents)
		fixture.manifest.Files = append(fixture.manifest.Files, verificationManifestFile{
			Path:   path,
			Size:   len(file.contents),
			Mode:   fmt.Sprintf("%04o", file.mode.Perm()),
			SHA256: hex.EncodeToString(digest[:]),
		})
	}
}

func (fixture *verificationFixture) write(t *testing.T) {
	t.Helper()

	directory := t.TempDir()
	fixture.archivePath = filepath.Join(directory, fixture.archiveName)
	fixture.checksumPath = fixture.archivePath + ".sha256"
	archiveContents := fixture.archiveBytes
	if archiveContents == nil {
		manifestContents := fixture.manifestBytes
		if manifestContents == nil {
			var err error
			manifestContents, err = json.Marshal(fixture.manifest)
			require.NoError(t, err)
			manifestContents = append(manifestContents, '\n')
		}
		archiveFiles := make(map[string]verificationFile, len(fixture.files)+1)
		for path, file := range fixture.files {
			archiveFiles[path] = file
		}
		if !fixture.omitManifest {
			archiveFiles["artifact-manifest.json"] = verificationFile{contents: manifestContents, mode: 0o444}
		}
		archiveContents = verificationArchive(t, fixture.target.ArchiveFormat(), archiveFiles)
	}
	require.NoError(t, os.WriteFile(fixture.archivePath, archiveContents, 0o600))
	digest := sha256.Sum256(archiveContents)
	fixture.archiveDigest = hex.EncodeToString(digest[:])
	checksumName := fixture.archiveName
	if fixture.checksumName != "" {
		checksumName = fixture.checksumName
	}
	checksumText := fixture.archiveDigest + "  " + checksumName + "\n"
	if fixture.checksumText != nil {
		checksumText = *fixture.checksumText
	}
	require.NoError(t, os.WriteFile(fixture.checksumPath, []byte(checksumText), 0o600))
}

func verificationArchive(t *testing.T, format ArchiveFormat, files map[string]verificationFile) []byte {
	t.Helper()

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var buffer bytes.Buffer
	switch format {
	case ArchiveFormatZIP:
		writer := zip.NewWriter(&buffer)
		for _, path := range paths {
			header := &zip.FileHeader{Name: applicationName + "/" + path, Method: zip.Store}
			header.SetMode(files[path].mode)
			entry, err := writer.CreateHeader(header)
			require.NoError(t, err)
			_, err = entry.Write(files[path].contents)
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())
	case ArchiveFormatTarGzip:
		gzipWriter := gzip.NewWriter(&buffer)
		tarWriter := tar.NewWriter(gzipWriter)
		for _, path := range paths {
			file := files[path]
			header := &tar.Header{
				Name:     applicationName + "/" + path,
				Mode:     int64(file.mode.Perm()),
				Size:     int64(len(file.contents)),
				Typeflag: tar.TypeReg,
			}
			require.NoError(t, tarWriter.WriteHeader(header))
			_, err := tarWriter.Write(file.contents)
			require.NoError(t, err)
		}
		require.NoError(t, tarWriter.Close())
		require.NoError(t, gzipWriter.Close())
	default:
		require.FailNow(t, "unsupported verification fixture archive format", string(format))
	}
	return buffer.Bytes()
}

func verificationPE(goarch string) []byte {
	const peOffset = 0x80
	const optionalHeaderSize = 0xf0
	contents := make([]byte, peOffset+4+20+optionalHeaderSize)
	copy(contents, "MZ")
	binary.LittleEndian.PutUint32(contents[0x3c:], peOffset)
	copy(contents[peOffset:], "PE\x00\x00")
	machine := uint16(0x8664)
	if goarch == goarchARM64 {
		machine = 0xaa64
	}
	fileHeader := contents[peOffset+4:]
	binary.LittleEndian.PutUint16(fileHeader, machine)
	binary.LittleEndian.PutUint16(fileHeader[16:], optionalHeaderSize)
	binary.LittleEndian.PutUint16(fileHeader[18:], 0x0002)
	optionalHeader := fileHeader[20:]
	binary.LittleEndian.PutUint16(optionalHeader, 0x020b)
	binary.LittleEndian.PutUint16(optionalHeader[68:], 2)
	return contents
}

func verificationELF(goarch string) []byte {
	contents := make([]byte, 64)
	copy(contents, []byte{0x7f, 'E', 'L', 'F'})
	contents[4] = 2
	contents[5] = 1
	contents[6] = 1
	binary.LittleEndian.PutUint16(contents[16:], 2)
	machine := uint16(62)
	if goarch == goarchARM64 {
		machine = 183
	}
	binary.LittleEndian.PutUint16(contents[18:], machine)
	binary.LittleEndian.PutUint32(contents[20:], 1)
	binary.LittleEndian.PutUint16(contents[52:], 64)
	binary.LittleEndian.PutUint16(contents[54:], 56)
	binary.LittleEndian.PutUint16(contents[58:], 64)
	return contents
}
