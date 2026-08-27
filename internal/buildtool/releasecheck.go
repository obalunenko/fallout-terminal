package buildtool

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	releaseMajorVersion     = "2"
	developmentBuildVersion = "development"
	maxReleaseExecutable    = 512 << 20
	maxReleaseMetadata      = 1 << 20
)

var (
	releaseTagPattern     = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	releaseVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
)

// ReleaseVersion is the single validated identity used by executable and
// platform metadata. Numeric fields are derived from Canonical and never act
// as independent version inputs.
type ReleaseVersion struct {
	Canonical       string
	NumericCore     string
	NumericFourPart string
	Prerelease      bool
	IsRelease       bool
}

// ParseReleaseTag validates one strict v2 release tag and derives every
// representation consumed by the package matrix.
func ParseReleaseTag(tag string) (ReleaseVersion, error) {
	matches := releaseTagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return ReleaseVersion{}, fmt.Errorf("invalid release tag %q (want v2.MINOR.PATCH with an optional SemVer prerelease suffix)", tag)
	}
	return releaseVersionFromMatches("release tag", tag, matches)
}

// ResolveBuildVersion validates a non-empty canonical release VERSION. Empty
// input selects the explicit local development identity and zero-valued native
// numeric representations.
func ResolveBuildVersion(value string) (ReleaseVersion, error) {
	if value == "" {
		return ReleaseVersion{
			Canonical:       developmentBuildVersion,
			NumericCore:     "0.0.0",
			NumericFourPart: "0.0.0.0",
		}, nil
	}
	matches := releaseVersionPattern.FindStringSubmatch(value)
	if matches == nil {
		return ReleaseVersion{}, fmt.Errorf("invalid build VERSION %q (want 2.MINOR.PATCH with an optional SemVer prerelease suffix)", value)
	}
	return releaseVersionFromMatches("build VERSION", value, matches)
}

// ValidateReleaseTag accepts the release workflow's strict v2 SemVer subset
// and reports whether the accepted version is a prerelease. Build metadata is
// not accepted because a tag maps to one create-only GitHub Release identity.
func ValidateReleaseTag(tag string) (bool, error) {
	version, err := ParseReleaseTag(tag)
	if err != nil {
		return false, err
	}
	return version.Prerelease, nil
}

func releaseVersionFromMatches(kind, value string, matches []string) (ReleaseVersion, error) {
	if matches[1] != releaseMajorVersion {
		return ReleaseVersion{}, fmt.Errorf("invalid %s %q: release major must be %s", kind, value, releaseMajorVersion)
	}
	prerelease := matches[4]
	for _, identifier := range strings.Split(prerelease, ".") {
		if len(identifier) > 1 && identifier[0] == '0' && numericIdentifier(identifier) {
			return ReleaseVersion{}, fmt.Errorf("invalid %s %q: numeric prerelease identifiers must not contain leading zeroes", kind, value)
		}
	}

	numericCore := strings.Join(matches[1:4], ".")
	canonical := numericCore
	if prerelease != "" {
		canonical += "-" + prerelease
	}
	return ReleaseVersion{
		Canonical:       canonical,
		NumericCore:     numericCore,
		NumericFourPart: numericCore + ".0",
		Prerelease:      prerelease != "",
		IsRelease:       true,
	}, nil
}

func numericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// InspectReleaseArchive validates only release eligibility: stable filename,
// non-empty archive, expected executable, and required resources. Stronger
// writer invariants remain intentionally outside this tagged-release gate.
func InspectReleaseArchive(ctx context.Context, target Target, archivePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !target.Portable() {
		return fmt.Errorf("unsupported release target %s", target)
	}
	if filepath.Base(archivePath) != target.ArchiveName() {
		return fmt.Errorf("archive filename mismatch for %s: want %q, got %q", target, target.ArchiveName(), filepath.Base(archivePath))
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("stat release archive for %s: %w", target, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("release archive for %s is not a regular file", target)
	}
	if info.Size() == 0 {
		return fmt.Errorf("release archive for %s is empty", target)
	}

	entries, err := inspectReleaseEntries(ctx, target.ArchiveFormat(), archivePath)
	if err != nil {
		return fmt.Errorf("inspect %s release archive: %w", target, err)
	}
	executable := path.Join(applicationName, target.ExecutablePath())
	if size, found := entries[executable]; !found {
		return fmt.Errorf("release archive for %s is missing executable %q", target, executable)
	} else if size == 0 {
		return fmt.Errorf("release archive for %s contains empty executable %q", target, executable)
	}
	for _, resource := range target.RequiredResourcePaths() {
		required := path.Join(applicationName, resource)
		if _, found := entries[required]; !found {
			return fmt.Errorf("release archive for %s is missing required resource %q", target, required)
		}
	}
	return ctx.Err()
}

// InspectReleaseArchiveVersion verifies that one structurally eligible archive
// reports the expected canonical release identity. Executables are run only on
// an exact matching native host.
func InspectReleaseArchiveVersion(
	ctx context.Context,
	target Target,
	archivePath string,
	expectedVersion string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	expected, err := ResolveBuildVersion(expectedVersion)
	if err != nil {
		return fmt.Errorf("validate expected release version: %w", err)
	}
	if !expected.IsRelease {
		return errors.New("expected release version is required")
	}
	if err := ValidateHost(target, RuntimeHost()); err != nil {
		return fmt.Errorf("inspect packaged version on matching native host: %w", err)
	}
	return inspectReleaseArchiveVersion(ctx, target, archivePath, expected, probeNativeVersion)
}

func inspectReleaseArchiveVersion(
	ctx context.Context,
	target Target,
	archivePath string,
	expected ReleaseVersion,
	probe nativeVersionProbe,
) error {
	if err := InspectReleaseArchive(ctx, target, archivePath); err != nil {
		return err
	}
	if !expected.IsRelease {
		return errors.New("archive inspection requires an expected release version")
	}
	if err := validatePackageVersion(expected); err != nil {
		return fmt.Errorf("validate expected release version: %w", err)
	}
	if probe == nil {
		return errors.New("native version probe is required")
	}

	executableEntry := path.Join(applicationName, target.ExecutablePath())
	executable, err := readReleaseArchiveEntry(ctx, target.ArchiveFormat(), archivePath, executableEntry, maxReleaseExecutable)
	if err != nil {
		return fmt.Errorf("extract packaged executable version target: %w", err)
	}
	temporaryRoot, err := os.MkdirTemp("", "fallout-terminal-release-version-")
	if err != nil {
		return fmt.Errorf("create packaged version inspection directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(temporaryRoot) }()
	executablePath := filepath.Join(temporaryRoot, target.ExecutableName())
	if err := os.WriteFile(executablePath, executable, 0o700); err != nil {
		return fmt.Errorf("write packaged executable version target: %w", err)
	}
	clear(executable)
	if err := os.Chmod(executablePath, 0o700); err != nil {
		return fmt.Errorf("make packaged executable version target runnable: %w", err)
	}

	evidence, err := probe(ctx, target, executablePath, []string{"--version"})
	if err != nil {
		return fmt.Errorf("probe packaged executable version: %w", err)
	}
	if err := validateNativeVersionEvidence(target, expected, evidence); err != nil {
		return err
	}
	if target.OS() != goosDarwin {
		return ctx.Err()
	}

	plistEntry := path.Join(applicationName, applicationName+".app", "Contents", "Info.plist")
	plist, err := readReleaseArchiveEntry(ctx, target.ArchiveFormat(), archivePath, plistEntry, maxReleaseMetadata)
	if err != nil {
		return fmt.Errorf("read packaged Darwin metadata: %w", err)
	}
	if err := validateDarwinVersionMetadata(plist, expected); err != nil {
		return err
	}
	return ctx.Err()
}

func readReleaseArchiveEntry(
	ctx context.Context,
	format ArchiveFormat,
	archivePath string,
	entryName string,
	maximumSize int64,
) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch format {
	case ArchiveFormatZIP:
		return readZIPReleaseArchiveEntry(ctx, archivePath, entryName, maximumSize)
	case ArchiveFormatTarGzip:
		return readTarGzipReleaseArchiveEntry(ctx, archivePath, entryName, maximumSize)
	default:
		return nil, fmt.Errorf("unsupported release archive format %q", format)
	}
}

func readZIPReleaseArchiveEntry(
	ctx context.Context,
	archivePath string,
	entryName string,
	maximumSize int64,
) ([]byte, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	for _, file := range reader.File {
		if file.Name != entryName {
			continue
		}
		if file.FileInfo().IsDir() || !file.Mode().IsRegular() {
			_ = reader.Close()
			return nil, fmt.Errorf("archive entry %q is not a regular file", entryName)
		}
		if file.UncompressedSize64 > uint64(maximumSize) {
			_ = reader.Close()
			return nil, fmt.Errorf("archive entry %q exceeds %d bytes", entryName, maximumSize)
		}
		entry, err := file.Open()
		if err != nil {
			_ = reader.Close()
			return nil, err
		}
		contents, readErr := readBoundedReleaseEntry(ctx, entry, maximumSize)
		entryCloseErr := entry.Close()
		archiveCloseErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if entryCloseErr != nil {
			return nil, entryCloseErr
		}
		if archiveCloseErr != nil {
			return nil, archiveCloseErr
		}
		return contents, nil
	}
	if err := reader.Close(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("archive entry %q is missing", entryName)
}

func readTarGzipReleaseArchiveEntry(
	ctx context.Context,
	archivePath string,
	entryName string,
	maximumSize int64,
) ([]byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	tarReader := tar.NewReader(gzipReader)
	for {
		if err := ctx.Err(); err != nil {
			_ = gzipReader.Close()
			_ = file.Close()
			return nil, err
		}
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = gzipReader.Close()
			_ = file.Close()
			return nil, err
		}
		if header.Name != entryName {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			_ = gzipReader.Close()
			_ = file.Close()
			return nil, fmt.Errorf("archive entry %q is not a regular file", entryName)
		}
		if header.Size > maximumSize {
			_ = gzipReader.Close()
			_ = file.Close()
			return nil, fmt.Errorf("archive entry %q exceeds %d bytes", entryName, maximumSize)
		}
		contents, readErr := readBoundedReleaseEntry(ctx, tarReader, maximumSize)
		gzipCloseErr := gzipReader.Close()
		fileCloseErr := file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if gzipCloseErr != nil {
			return nil, gzipCloseErr
		}
		if fileCloseErr != nil {
			return nil, fileCloseErr
		}
		return contents, nil
	}
	gzipCloseErr := gzipReader.Close()
	fileCloseErr := file.Close()
	if gzipCloseErr != nil {
		return nil, gzipCloseErr
	}
	if fileCloseErr != nil {
		return nil, fileCloseErr
	}
	return nil, fmt.Errorf("archive entry %q is missing", entryName)
}

func readBoundedReleaseEntry(ctx context.Context, reader io.Reader, maximumSize int64) ([]byte, error) {
	contents, err := io.ReadAll(io.LimitReader(reader, maximumSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > maximumSize {
		clear(contents)
		return nil, fmt.Errorf("archive entry exceeds %d bytes", maximumSize)
	}
	if err := ctx.Err(); err != nil {
		clear(contents)
		return nil, err
	}
	return contents, nil
}

func inspectReleaseEntries(ctx context.Context, format ArchiveFormat, archivePath string) (map[string]int64, error) {
	switch format {
	case ArchiveFormatZIP:
		reader, err := zip.OpenReader(archivePath)
		if err != nil {
			return nil, err
		}
		entries := make(map[string]int64, len(reader.File))
		for _, file := range reader.File {
			if err := ctx.Err(); err != nil {
				_ = reader.Close()
				return nil, err
			}
			if !file.FileInfo().IsDir() {
				entries[file.Name] = int64(file.UncompressedSize64)
			}
		}
		if err := reader.Close(); err != nil {
			return nil, err
		}
		return entries, nil
	case ArchiveFormatTarGzip:
		file, err := os.Open(archivePath)
		if err != nil {
			return nil, err
		}
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		entries := make(map[string]int64)
		tarReader := tar.NewReader(gzipReader)
		for {
			if err := ctx.Err(); err != nil {
				_ = gzipReader.Close()
				_ = file.Close()
				return nil, err
			}
			header, err := tarReader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				_ = gzipReader.Close()
				_ = file.Close()
				return nil, err
			}
			if header.Typeflag == tar.TypeReg {
				entries[header.Name] = header.Size
			}
		}
		gzipErr := gzipReader.Close()
		fileErr := file.Close()
		if gzipErr != nil {
			return nil, gzipErr
		}
		if fileErr != nil {
			return nil, fileErr
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("unsupported release archive format %q", format)
	}
}

// InspectReleaseInventory requires the flat directory passed to GoReleaser to
// contain exactly one non-empty stable archive for every supported target.
func InspectReleaseInventory(ctx context.Context, directory string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read release inventory: %w", err)
	}
	want := make([]string, 0, len(PortableTargets()))
	for _, target := range PortableTargets() {
		want = append(want, target.ArchiveName())
	}
	sort.Strings(want)
	if len(entries) != len(want) {
		return fmt.Errorf("release inventory requires exactly %d archives, got %d entries", len(want), len(entries))
	}
	for index, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Name() != want[index] {
			return fmt.Errorf("release inventory mismatch at %d: want %q, got %q", index, want[index], entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect release asset %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("release asset %q is not a regular file", entry.Name())
		}
		if info.Size() == 0 {
			return fmt.Errorf("release asset %q is empty", entry.Name())
		}
	}
	return ctx.Err()
}
