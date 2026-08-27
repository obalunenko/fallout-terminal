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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	artifactManifestFilename = "artifact-manifest.json"
	artifactManifestVersion  = 2
)

var normalizedArchiveTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// ArchiveFile maps one archive-relative payload path to a staged regular file.
type ArchiveFile struct {
	Path       string
	SourcePath string
}

// ArchiveResult identifies the atomically published archive and checksum.
type ArchiveResult struct {
	ArchivePath  string
	ChecksumPath string
	SHA256       string
}

// ArtifactManifest is the stable schema-v2 identity and file inventory stored
// inside every portable archive.
type ArtifactManifest struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	Product        string                 `json:"product"`
	Version        string                 `json:"version"`
	SourceRevision string                 `json:"sourceRevision"`
	Target         ArtifactTarget         `json:"target"`
	Runtime        string                 `json:"runtime"`
	Files          []ArtifactFileManifest `json:"files"`
}

// ArtifactTarget is the canonical operating system and architecture recorded
// in an artifact manifest.
type ArtifactTarget struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// ArtifactFileManifest records deterministic evidence for one payload file.
type ArtifactFileManifest struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

type preparedArchiveFile struct {
	path     string
	contents []byte
	mode     fs.FileMode
}

// WritePortableArchive validates and publishes one deterministic target
// archive and its SHA-256 sidecar. Failed attempts expose neither output.
func WritePortableArchive(
	ctx context.Context,
	outputDirectory string,
	target Target,
	version ReleaseVersion,
	sourceRevision string,
	files []ArchiveFile,
) (ArchiveResult, error) {
	if ctx == nil {
		return ArchiveResult{}, errors.New("archive context is required")
	}
	if err := ctx.Err(); err != nil {
		return ArchiveResult{}, err
	}
	if !target.Portable() {
		return ArchiveResult{}, fmt.Errorf("portable archive requires a supported release target, got %s", target)
	}
	if outputDirectory == "" {
		return ArchiveResult{}, errors.New("archive output directory is empty")
	}
	if err := validatePackageVersion(version); err != nil {
		return ArchiveResult{}, err
	}
	if err := validateSourceRevision(sourceRevision); err != nil {
		return ArchiveResult{}, err
	}

	prepared, manifest, err := prepareArchiveFiles(ctx, target, version, sourceRevision, files)
	if err != nil {
		return ArchiveResult{}, err
	}
	archiveContents, err := encodePortableArchive(ctx, target.ArchiveFormat(), prepared, manifest)
	if err != nil {
		return ArchiveResult{}, err
	}

	digest := sha256.Sum256(archiveContents)
	digestText := hex.EncodeToString(digest[:])
	archivePath := filepath.Join(filepath.Clean(outputDirectory), target.ArchiveName())
	checksumPath := archivePath + ".sha256"
	checksumContents := []byte(digestText + "  " + filepath.Base(archivePath) + "\n")
	if err := ctx.Err(); err != nil {
		return ArchiveResult{}, err
	}
	if err := publishArchivePair(archivePath, archiveContents, checksumPath, checksumContents); err != nil {
		return ArchiveResult{}, err
	}
	return ArchiveResult{ArchivePath: archivePath, ChecksumPath: checksumPath, SHA256: digestText}, nil
}

func prepareArchiveFiles(
	ctx context.Context,
	target Target,
	version ReleaseVersion,
	sourceRevision string,
	files []ArchiveFile,
) ([]preparedArchiveFile, ArtifactManifest, error) {
	prepared := make([]preparedArchiveFile, 0, len(files)+1)
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, ArtifactManifest{}, err
		}
		normalized, err := normalizeArchivePath(target, file.Path)
		if err != nil {
			return nil, ArtifactManifest{}, err
		}
		key := normalized
		if target.OS() == goosWindows {
			key = strings.ToLower(key)
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, ArtifactManifest{}, fmt.Errorf("duplicate normalized archive path %q", normalized)
		}
		seen[key] = struct{}{}

		contents, err := readArchiveSource(ctx, file.SourcePath)
		if err != nil {
			return nil, ArtifactManifest{}, fmt.Errorf("read archive source for %q: %w", normalized, err)
		}
		prepared = append(prepared, preparedArchiveFile{
			path: normalized, contents: contents, mode: archiveFileMode(target, normalized),
		})
	}
	sort.Slice(prepared, func(i, j int) bool { return prepared[i].path < prepared[j].path })
	if err := validateExactArchiveInventory(target, prepared); err != nil {
		return nil, ArtifactManifest{}, err
	}

	manifest := ArtifactManifest{
		SchemaVersion:  artifactManifestVersion,
		Product:        applicationName,
		Version:        version.Canonical,
		SourceRevision: sourceRevision,
		Target:         ArtifactTarget{OS: target.OS(), Arch: target.Arch()},
		Runtime:        target.NativeRuntime(),
		Files:          make([]ArtifactFileManifest, 0, len(prepared)),
	}
	for _, file := range prepared {
		digest := sha256.Sum256(file.contents)
		manifest.Files = append(manifest.Files, ArtifactFileManifest{
			Path: file.path, Size: int64(len(file.contents)), Mode: fmt.Sprintf("%04o", file.mode.Perm()),
			SHA256: hex.EncodeToString(digest[:]),
		})
	}
	return prepared, manifest, nil
}

func normalizeArchivePath(target Target, candidate string) (string, error) {
	if candidate == "" {
		return "", errors.New("archive path is empty")
	}
	if strings.ContainsRune(candidate, 0) {
		return "", errors.New("archive path contains a NUL byte")
	}
	normalizedSlashes := strings.ReplaceAll(candidate, `\`, "/")
	if strings.HasPrefix(normalizedSlashes, "/") || filepath.IsAbs(candidate) {
		return "", fmt.Errorf("absolute archive path is forbidden: %q", candidate)
	}
	if len(normalizedSlashes) >= 2 && normalizedSlashes[1] == ':' {
		return "", fmt.Errorf("drive archive path is forbidden: %q", candidate)
	}
	normalized := path.Clean(normalizedSlashes)
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("parent traversal archive path is forbidden: %q", candidate)
	}
	if normalized == artifactManifestFilename {
		return "", fmt.Errorf("archive manifest path is reserved: %q", candidate)
	}
	if target.OS() == goosWindows && strings.Contains(normalized, ":") {
		return "", fmt.Errorf("drive archive path is forbidden: %q", candidate)
	}
	return normalized, nil
}

func readArchiveSource(ctx context.Context, sourcePath string) ([]byte, error) {
	if sourcePath == "" {
		return nil, errors.New("source path is empty")
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("symbolic link sources are forbidden")
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("source is not a regular file (mode %s)", info.Mode())
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return nil, errors.New("source changed while preparing the archive")
	}
	contents, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := ctx.Err(); err != nil {
		clear(contents)
		return nil, err
	}
	return contents, nil
}

func validateExactArchiveInventory(target Target, prepared []preparedArchiveFile) error {
	want := requiredArchivePaths(target)
	if len(prepared) != len(want) {
		return fmt.Errorf("exact payload inventory requires %d files, got %d", len(want), len(prepared))
	}
	for index, file := range prepared {
		if file.path != want[index] {
			return fmt.Errorf("exact payload inventory mismatch at %d: want %q, got %q", index, want[index], file.path)
		}
	}
	return nil
}

func requiredArchivePaths(target Target) []string {
	paths := []string{target.ExecutablePath()}
	for _, resource := range target.RequiredResourcePaths() {
		if resource != artifactManifestFilename {
			paths = append(paths, resource)
		}
	}
	sort.Strings(paths)
	return paths
}

func archiveFileMode(target Target, archivePath string) fs.FileMode {
	if archivePath == target.ExecutablePath() {
		return 0o755
	}
	return 0o444
}

func encodePortableArchive(
	ctx context.Context,
	format ArchiveFormat,
	files []preparedArchiveFile,
	manifest ArtifactManifest,
) ([]byte, error) {
	manifestContents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode artifact manifest: %w", err)
	}
	manifestContents = append(manifestContents, '\n')

	entries := make([]preparedArchiveFile, 0, len(files)+1)
	for _, file := range files {
		entries = append(entries, preparedArchiveFile{
			path: path.Join(applicationName, file.path), contents: file.contents, mode: file.mode,
		})
	}
	entries = append(entries, preparedArchiveFile{
		path: path.Join(applicationName, artifactManifestFilename), contents: manifestContents, mode: 0o444,
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	switch format {
	case ArchiveFormatZIP:
		return encodeZIP(ctx, entries)
	case ArchiveFormatTarGzip:
		return encodeTarGzip(ctx, entries)
	default:
		return nil, fmt.Errorf("unsupported portable archive format %q", format)
	}
}

func encodeZIP(ctx context.Context, entries []preparedArchiveFile) ([]byte, error) {
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			_ = writer.Close()
			return nil, err
		}
		header := &zip.FileHeader{Name: entry.path, Method: zip.Deflate}
		header.SetMode(entry.mode)
		header.Modified = normalizedArchiveTime
		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("create ZIP entry %q: %w", entry.path, err)
		}
		if _, err := entryWriter.Write(entry.contents); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("write ZIP entry %q: %w", entry.path, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close ZIP archive: %w", err)
	}
	return output.Bytes(), nil
}

func encodeTarGzip(ctx context.Context, entries []preparedArchiveFile) ([]byte, error) {
	var output bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&output, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create gzip writer: %w", err)
	}
	gzipWriter.ModTime = normalizedArchiveTime
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, err
		}
		header := &tar.Header{
			Name: entry.path, Mode: int64(entry.mode.Perm()), Size: int64(len(entry.contents)),
			ModTime: normalizedArchiveTime, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("create TAR entry %q: %w", entry.path, err)
		}
		if _, err := tarWriter.Write(entry.contents); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("write TAR entry %q: %w", entry.path, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("close TAR archive: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close gzip archive: %w", err)
	}
	return output.Bytes(), nil
}

func publishArchivePair(archivePath string, archiveContents []byte, checksumPath string, checksumContents []byte) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return fmt.Errorf("create archive output directory: %w", err)
	}
	archiveTemporary := archivePath + ".partial"
	checksumTemporary := checksumPath + ".partial"
	if err := removeArchiveOutputs(archiveTemporary, checksumTemporary); err != nil {
		return err
	}
	if err := writeSyncedFile(archiveTemporary, archiveContents, 0o644); err != nil {
		return cleanupArchiveFailure(
			fmt.Errorf("write unpublished archive: %w", err), archiveTemporary, checksumTemporary,
		)
	}
	if err := writeSyncedFile(checksumTemporary, checksumContents, 0o644); err != nil {
		return cleanupArchiveFailure(
			fmt.Errorf("write unpublished archive checksum: %w", err), archiveTemporary, checksumTemporary,
		)
	}
	if err := removeArchiveOutputs(checksumPath, archivePath); err != nil {
		return cleanupArchiveFailure(err, archiveTemporary, checksumTemporary)
	}
	if err := os.Rename(archiveTemporary, archivePath); err != nil {
		return cleanupArchiveFailure(
			fmt.Errorf("publish archive: %w", err), archiveTemporary, checksumTemporary,
		)
	}
	if err := os.Rename(checksumTemporary, checksumPath); err != nil {
		_ = os.Rename(archivePath, archiveTemporary)
		return cleanupArchiveFailure(
			fmt.Errorf("publish archive checksum: %w", err), archivePath, archiveTemporary, checksumTemporary,
		)
	}
	return nil
}

func writeSyncedFile(path string, contents []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func removeArchiveOutputs(paths ...string) error {
	var cleanupErrors []error
	for _, output := range paths {
		if err := os.Remove(output); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove archive output %q: %w", output, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func cleanupArchiveFailure(cause error, paths ...string) error {
	if cleanupErr := removeArchiveOutputs(paths...); cleanupErr != nil {
		return fmt.Errorf("%w (cleanup failed: %v)", cause, cleanupErr)
	}
	return cause
}

func validateSourceRevision(revision string) error {
	if len(revision) != 40 && len(revision) != 64 {
		return fmt.Errorf("source revision must be a full 40- or 64-character commit SHA")
	}
	for _, character := range revision {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return fmt.Errorf("source revision must be lowercase hexadecimal")
		}
	}
	return nil
}
