package buildtool

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const (
	maxVerificationEntrySize = 512 << 20
	maxVerificationTotalSize = 1 << 30
	maxManifestSize          = 1 << 20
)

// ArtifactVerification is immutable evidence that an archive and its checksum
// satisfy the complete contract for one target and source revision.
type ArtifactVerification struct {
	target         Target
	sourceRevision string
	archiveName    string
	checksum       string
}

// Target returns the verified portable target.
func (verification ArtifactVerification) Target() Target { return verification.target }

// SourceRevision returns the verified full source revision from the manifest.
func (verification ArtifactVerification) SourceRevision() string { return verification.sourceRevision }

// ArchiveName returns the verified stable archive filename.
func (verification ArtifactVerification) ArchiveName() string { return verification.archiveName }

// Checksum returns the verified lowercase archive SHA-256.
func (verification ArtifactVerification) Checksum() string { return verification.checksum }

type inspectedArtifactFile struct {
	size     int64
	mode     fs.FileMode
	sha256   string
	contents []byte
}

// VerifyArtifact validates an archive without extracting it to the filesystem.
func VerifyArtifact(
	ctx context.Context,
	archivePath string,
	checksumPath string,
	expected Target,
) (ArtifactVerification, error) {
	if ctx == nil {
		return ArtifactVerification{}, errors.New("artifact verification context is required")
	}
	if err := ctx.Err(); err != nil {
		return ArtifactVerification{}, err
	}
	if !expected.Portable() {
		return ArtifactVerification{}, fmt.Errorf("artifact verification requires a portable target, got %s", expected)
	}
	expectedArchiveName := expected.ArchiveName()
	archiveName := filepath.Base(archivePath)
	if archiveName != expectedArchiveName {
		return ArtifactVerification{}, fmt.Errorf("archive name %q does not match stable target name %q", archiveName, expectedArchiveName)
	}
	if filepath.Clean(checksumPath) != filepath.Clean(archivePath)+".sha256" {
		return ArtifactVerification{}, fmt.Errorf("checksum sidecar must be %q", archivePath+".sha256")
	}
	if err := requireRegularVerificationFile("archive", archivePath, maxVerificationTotalSize); err != nil {
		return ArtifactVerification{}, err
	}
	if err := requireRegularVerificationFile("checksum sidecar", checksumPath, 1024); err != nil {
		return ArtifactVerification{}, err
	}

	checksum, err := verifyArtifactChecksum(ctx, archivePath, checksumPath, archiveName)
	if err != nil {
		return ArtifactVerification{}, err
	}
	files, err := inspectArtifactArchive(ctx, archivePath, expected)
	if err != nil {
		return ArtifactVerification{}, err
	}
	manifest, err := verifyArtifactManifest(expected, files)
	if err != nil {
		return ArtifactVerification{}, err
	}
	if err := verifyArtifactExecutable(expected, files[expected.ExecutableName()].contents); err != nil {
		return ArtifactVerification{}, err
	}
	return ArtifactVerification{
		target:         expected,
		sourceRevision: manifest.SourceRevision,
		archiveName:    archiveName,
		checksum:       checksum,
	}, nil
}

func verifyArtifactChecksum(ctx context.Context, archivePath, checksumPath, archiveName string) (string, error) {
	contents, err := readLimitedFile(ctx, checksumPath, 1024)
	if err != nil {
		return "", fmt.Errorf("read checksum sidecar: %w", err)
	}
	line, ok := strings.CutSuffix(string(contents), "\n")
	if !ok || strings.Contains(line, "\n") {
		return "", errors.New("checksum sidecar must contain exactly one newline-terminated record")
	}
	digest, sidecarName, ok := strings.Cut(line, "  ")
	if !ok || sidecarName == "" || strings.Contains(sidecarName, "  ") {
		return "", errors.New("checksum sidecar must contain lowercase SHA-256, two spaces, and archive name")
	}
	if sidecarName != archiveName {
		return "", fmt.Errorf("checksum sidecar names %q instead of %q", sidecarName, archiveName)
	}
	if !validLowerHex(digest, sha256.Size) {
		return "", errors.New("checksum sidecar does not contain a lowercase SHA-256")
	}
	actual, err := hashFile(ctx, archivePath)
	if err != nil {
		return "", fmt.Errorf("hash archive for checksum verification: %w", err)
	}
	if actual != digest {
		return "", fmt.Errorf("archive checksum mismatch: got %s, want %s", actual, digest)
	}
	return actual, nil
}

func inspectArtifactArchive(ctx context.Context, archivePath string, target Target) (map[string]inspectedArtifactFile, error) {
	switch target.ArchiveFormat() {
	case ArchiveFormatZIP:
		return inspectZIPArtifact(ctx, archivePath, target)
	case ArchiveFormatTarGzip:
		return inspectTarGzipArtifact(ctx, archivePath)
	default:
		return nil, fmt.Errorf("unsupported artifact archive format %q", target.ArchiveFormat())
	}
}

func inspectZIPArtifact(
	ctx context.Context,
	archivePath string,
	target Target,
) (files map[string]inspectedArtifactFile, resultErr error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("inspect ZIP archive: %w", err)
	}
	defer closeVerificationResource(&resultErr, "close ZIP archive", reader)

	files = make(map[string]inspectedArtifactFile, len(reader.File))
	seen := make(map[string]struct{}, len(reader.File))
	previousName := ""
	var totalSize int64
	for _, entry := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := verifyArchiveOrder(previousName, entry.Name); err != nil {
			return nil, err
		}
		previousName = entry.Name
		relative, err := verifiedArchiveRelativePath(entry.Name)
		if err != nil {
			return nil, err
		}
		key := relative
		if target.OS() == goosWindows {
			key = strings.ToLower(key)
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("archive contains duplicate normalized entry %q", entry.Name)
		}
		seen[key] = struct{}{}
		if !entry.Mode().IsRegular() {
			return nil, fmt.Errorf("archive entry %q is not a regular file", entry.Name)
		}
		declaredSize := int64(entry.UncompressedSize64)
		if err := verifyArtifactSize(relative, declaredSize, &totalSize); err != nil {
			return nil, err
		}
		entryReader, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("open ZIP archive entry %q: %w", entry.Name, err)
		}
		inspected, readErr := inspectArtifactEntry(
			ctx,
			io.LimitReader(entryReader, declaredSize+1),
			declaredSize,
			retainArtifactContents(relative),
		)
		closeErr := entryReader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read ZIP archive entry %q: %w", entry.Name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close ZIP archive entry %q: %w", entry.Name, closeErr)
		}
		inspected.mode = entry.Mode().Perm()
		files[relative] = inspected
	}
	return files, nil
}

func inspectTarGzipArtifact(
	ctx context.Context,
	archivePath string,
) (files map[string]inspectedArtifactFile, resultErr error) {
	archive, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open TAR.GZ archive: %w", err)
	}
	defer closeVerificationResource(&resultErr, "close TAR.GZ archive", archive)
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return nil, fmt.Errorf("inspect TAR.GZ archive: %w", err)
	}
	defer closeVerificationResource(&resultErr, "close gzip reader", gzipReader)

	files = make(map[string]inspectedArtifactFile)
	tarReader := tar.NewReader(gzipReader)
	previousName := ""
	var totalSize int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("inspect TAR.GZ archive: %w", err)
		}
		if err := verifyArchiveOrder(previousName, header.Name); err != nil {
			return nil, err
		}
		previousName = header.Name
		relative, err := verifiedArchiveRelativePath(header.Name)
		if err != nil {
			return nil, err
		}
		// A zero type flag is the historical regular-file encoding accepted by tar readers.
		if header.Typeflag != tar.TypeReg && header.Typeflag != 0 {
			return nil, fmt.Errorf("archive entry %q is not a regular file", header.Name)
		}
		if err := verifyArtifactSize(relative, header.Size, &totalSize); err != nil {
			return nil, err
		}
		inspected, err := inspectArtifactEntry(ctx, tarReader, header.Size, retainArtifactContents(relative))
		if err != nil {
			return nil, fmt.Errorf("read TAR.GZ archive entry %q: %w", header.Name, err)
		}
		inspected.mode = fs.FileMode(header.Mode).Perm()
		files[relative] = inspected
	}
	return files, nil
}

func verifiedArchiveRelativePath(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, '\x00') || strings.Contains(name, `\`) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	if path.Clean(name) != name {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	prefix := applicationName + "/"
	if !strings.HasPrefix(name, prefix) {
		return "", fmt.Errorf("unsafe archive path %q is outside the %s root", name, applicationName)
	}
	relative := strings.TrimPrefix(name, prefix)
	if relative == "" || path.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, "../") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return relative, nil
}

func verifyArchiveOrder(previous, current string) error {
	if previous == "" {
		return nil
	}
	if current == previous {
		return fmt.Errorf("archive contains duplicate entry %q", current)
	}
	if current < previous {
		return fmt.Errorf("archive entries are not sorted: %q appears after %q", current, previous)
	}
	return nil
}

func verifyArtifactSize(path string, size int64, total *int64) error {
	if size < 0 || size > maxVerificationEntrySize {
		return fmt.Errorf("archive entry %q has unsafe size %d", path, size)
	}
	if path == artifactManifestFilename && size > maxManifestSize {
		return fmt.Errorf("%s exceeds %d bytes", artifactManifestFilename, maxManifestSize)
	}
	if *total > maxVerificationTotalSize-size {
		return fmt.Errorf("archive uncompressed size exceeds %d bytes", maxVerificationTotalSize)
	}
	*total += size
	return nil
}

func inspectArtifactEntry(ctx context.Context, reader io.Reader, declaredSize int64, retain bool) (inspectedArtifactFile, error) {
	digest := sha256.New()
	var contents bytes.Buffer
	writers := []io.Writer{digest}
	if retain {
		contents.Grow(int(declaredSize))
		writers = append(writers, &contents)
	}
	written, err := copyWithContext(ctx, io.MultiWriter(writers...), reader)
	if err != nil {
		return inspectedArtifactFile{}, err
	}
	if written != declaredSize {
		return inspectedArtifactFile{}, fmt.Errorf("size is %d bytes, archive declares %d", written, declaredSize)
	}
	return inspectedArtifactFile{
		size:     written,
		sha256:   hex.EncodeToString(digest.Sum(nil)),
		contents: contents.Bytes(),
	}, nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			count, writeErr := destination.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}

func retainArtifactContents(relative string) bool {
	return relative == artifactManifestFilename || relative == applicationName || relative == applicationName+".exe"
}

func verifyArtifactManifest(expected Target, files map[string]inspectedArtifactFile) (ArtifactManifest, error) {
	manifestEntry, exists := files[artifactManifestFilename]
	if !exists {
		return ArtifactManifest{}, fmt.Errorf("required archive file %q is missing", artifactManifestFilename)
	}
	if manifestEntry.size > maxManifestSize {
		return ArtifactManifest{}, fmt.Errorf("%s exceeds %d bytes", artifactManifestFilename, maxManifestSize)
	}
	if manifestEntry.mode.Perm() != 0o444 {
		return ArtifactManifest{}, fmt.Errorf("%s mode is %04o, want 0444", artifactManifestFilename, manifestEntry.mode.Perm())
	}
	manifest, err := decodeArtifactManifest(manifestEntry.contents)
	if err != nil {
		return ArtifactManifest{}, fmt.Errorf("decode %s: %w", artifactManifestFilename, err)
	}
	if manifest.SchemaVersion != artifactManifestVersion {
		return ArtifactManifest{}, fmt.Errorf(
			"%s schemaVersion is %d, want %d",
			artifactManifestFilename,
			manifest.SchemaVersion,
			artifactManifestVersion,
		)
	}
	if manifest.Product != applicationName {
		return ArtifactManifest{}, fmt.Errorf("manifest product is %q, want %q", manifest.Product, applicationName)
	}
	version, err := resolveManifestVersion(manifest.Version)
	if err != nil {
		return ArtifactManifest{}, fmt.Errorf("manifest version: %w", err)
	}
	if err := validatePackageVersion(version); err != nil {
		return ArtifactManifest{}, fmt.Errorf("manifest version: %w", err)
	}
	if err := validateSourceRevision(manifest.SourceRevision); err != nil {
		return ArtifactManifest{}, fmt.Errorf("manifest sourceRevision: %w", err)
	}
	if manifest.Target.OS != expected.OS() || manifest.Target.Arch != expected.Arch() {
		return ArtifactManifest{}, fmt.Errorf(
			"manifest target %s/%s does not match expected %s",
			manifest.Target.OS,
			manifest.Target.Arch,
			expected,
		)
	}
	if manifest.Runtime != expected.NativeRuntime() {
		return ArtifactManifest{}, fmt.Errorf("manifest runtime is %q, want %q", manifest.Runtime, expected.NativeRuntime())
	}
	if err := verifyManifestInventory(expected, manifest.Files, files); err != nil {
		return ArtifactManifest{}, err
	}
	return manifest, nil
}

func decodeArtifactManifest(contents []byte) (ArtifactManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var manifest ArtifactManifest
	if err := decoder.Decode(&manifest); err != nil {
		return ArtifactManifest{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ArtifactManifest{}, errors.New("manifest contains multiple JSON values")
		}
		return ArtifactManifest{}, err
	}
	return manifest, nil
}

func verifyManifestInventory(expected Target, manifestFiles []ArtifactFileManifest, files map[string]inspectedArtifactFile) error {
	required := requiredArchivePaths(expected)
	manifestByPath := make(map[string]ArtifactFileManifest, len(manifestFiles))
	seen := make(map[string]struct{}, len(manifestFiles))
	previousPath := ""
	for _, record := range manifestFiles {
		key := record.Path
		if expected.OS() == goosWindows {
			key = strings.ToLower(key)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("manifest contains duplicate path %q", record.Path)
		}
		seen[key] = struct{}{}
		if previousPath != "" && record.Path < previousPath {
			return fmt.Errorf("manifest files are not sorted: %q appears after %q", record.Path, previousPath)
		}
		previousPath = record.Path
		if _, err := verifiedManifestPath(record.Path); err != nil {
			return err
		}
		manifestByPath[record.Path] = record
	}

	for _, requiredPath := range required {
		record, listed := manifestByPath[requiredPath]
		if !listed {
			return fmt.Errorf("required archive file %q is missing from manifest", requiredPath)
		}
		entry, archived := files[requiredPath]
		if !archived {
			return fmt.Errorf("required archive file %q is missing", requiredPath)
		}
		if entry.size == 0 {
			return fmt.Errorf("required archive file %q is empty", requiredPath)
		}
		requiredMode := archiveFileMode(expected, requiredPath)
		wantMode := fmt.Sprintf("%04o", requiredMode.Perm())
		if record.Mode != wantMode || entry.mode.Perm() != requiredMode.Perm() {
			return fmt.Errorf("archive file %q mode is manifest=%s archive=%04o, want %s", requiredPath, record.Mode, entry.mode.Perm(), wantMode)
		}
	}
	if len(manifestByPath) != len(required) {
		for manifestPath := range manifestByPath {
			if !slices.Contains(required, manifestPath) {
				return fmt.Errorf("manifest contains unexpected file %q", manifestPath)
			}
		}
		return fmt.Errorf("manifest contains %d files, want %d", len(manifestByPath), len(required))
	}
	if len(files) != len(required)+1 {
		for archivePath := range files {
			if archivePath == artifactManifestFilename {
				continue
			}
			if !slices.Contains(required, archivePath) {
				return fmt.Errorf("archive contains unlisted file %q", archivePath)
			}
		}
		return fmt.Errorf("archive contains %d files, want %d", len(files), len(required)+1)
	}

	for manifestPath, record := range manifestByPath {
		entry := files[manifestPath]
		if record.Size != entry.size {
			return fmt.Errorf("archive file %q size is %d, manifest declares %d", manifestPath, entry.size, record.Size)
		}
		if !validLowerHex(record.SHA256, sha256.Size) {
			return fmt.Errorf("archive file %q manifest sha256 is invalid", manifestPath)
		}
		if record.SHA256 != entry.sha256 {
			return fmt.Errorf("archive file %q sha256 is %s, manifest declares %s", manifestPath, entry.sha256, record.SHA256)
		}
		archiveMode := fmt.Sprintf("%04o", entry.mode.Perm())
		if record.Mode != archiveMode {
			return fmt.Errorf("archive file %q mode is %s, manifest declares %s", manifestPath, archiveMode, record.Mode)
		}
	}
	return nil
}

func verifiedManifestPath(manifestPath string) (string, error) {
	if manifestPath == "" || strings.ContainsRune(manifestPath, '\x00') || strings.Contains(manifestPath, `\`) || path.IsAbs(manifestPath) {
		return "", fmt.Errorf("manifest contains unsafe path %q", manifestPath)
	}
	if path.Clean(manifestPath) != manifestPath || manifestPath == ".." || strings.HasPrefix(manifestPath, "../") {
		return "", fmt.Errorf("manifest contains unsafe path %q", manifestPath)
	}
	return manifestPath, nil
}

func verifyArtifactExecutable(target Target, contents []byte) error {
	switch target.OS() {
	case goosWindows:
		return verifyPEExecutable(target, contents)
	case goosLinux:
		return verifyELFExecutable(target, contents)
	default:
		return fmt.Errorf("unsupported executable target %s", target)
	}
}

func verifyPEExecutable(target Target, contents []byte) error {
	if len(contents) < 0x40 || !bytes.Equal(contents[:2], []byte("MZ")) {
		return fmt.Errorf("verify PE executable %q: DOS header is missing", target.ExecutableName())
	}
	peOffset := uint64(binary.LittleEndian.Uint32(contents[0x3c:]))
	if peOffset > uint64(len(contents)-24) || !bytes.Equal(contents[peOffset:peOffset+4], []byte("PE\x00\x00")) {
		return fmt.Errorf("verify PE executable %q: PE signature is missing or invalid", target.ExecutableName())
	}
	fileHeader := contents[peOffset+4 : peOffset+24]
	wantMachine := uint16(0x8664)
	if target.Arch() == goarchARM64 {
		wantMachine = 0xaa64
	}
	machine := binary.LittleEndian.Uint16(fileHeader)
	if machine != wantMachine {
		return fmt.Errorf("PE executable %q machine %#x does not match %s", target.ExecutableName(), machine, target.Arch())
	}
	const imageFileExecutable = 0x0002
	if binary.LittleEndian.Uint16(fileHeader[18:])&imageFileExecutable == 0 {
		return fmt.Errorf("PE executable %q is not marked executable", target.ExecutableName())
	}
	return nil
}

func verifyELFExecutable(target Target, contents []byte) error {
	file, err := elf.NewFile(bytes.NewReader(contents))
	if err != nil {
		return fmt.Errorf("verify ELF executable %q: %w", target.ExecutableName(), err)
	}
	wantMachine := elf.EM_X86_64
	if target.Arch() == goarchARM64 {
		wantMachine = elf.EM_AARCH64
	}
	if file.Machine != wantMachine {
		return fmt.Errorf("ELF executable %q machine %s does not match %s", target.ExecutableName(), file.Machine, target.Arch())
	}
	if file.Type != elf.ET_EXEC && file.Type != elf.ET_DYN {
		return fmt.Errorf("ELF executable %q has non-executable type %s", target.ExecutableName(), file.Type)
	}
	return nil
}

func closeVerificationResource(resultErr *error, operation string, closer io.Closer) {
	if err := closer.Close(); err != nil {
		*resultErr = errors.Join(*resultErr, fmt.Errorf("%s: %w", operation, err))
	}
}

func readLimitedFile(ctx context.Context, filePath string, limit int64) (_ []byte, resultErr error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer closeVerificationResource(&resultErr, "close limited verification file", file)
	var contents bytes.Buffer
	written, err := copyWithContext(ctx, &contents, io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if written > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return contents.Bytes(), nil
}

func requireRegularVerificationFile(name, filePath string, maximumSize int64) error {
	info, err := os.Lstat(filePath)
	if err != nil {
		return fmt.Errorf("inspect %s %q: %w", name, filePath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s %q is not a regular file", name, filePath)
	}
	if info.Size() > maximumSize {
		return fmt.Errorf("%s %q exceeds %d bytes", name, filePath, maximumSize)
	}
	return nil
}

func hashFile(ctx context.Context, filePath string) (_ string, resultErr error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer closeVerificationResource(&resultErr, "close verification hash input", file)
	digest := sha256.New()
	if _, err := copyWithContext(ctx, digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func validLowerHex(value string, size int) bool {
	if len(value) != size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
