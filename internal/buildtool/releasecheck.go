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

const releaseMajorVersion = "2"

var releaseTagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// ValidateReleaseTag accepts the release workflow's strict v2 SemVer subset
// and reports whether the accepted version is a prerelease. Build metadata is
// not accepted because a tag maps to one create-only GitHub Release identity.
func ValidateReleaseTag(tag string) (bool, error) {
	matches := releaseTagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return false, fmt.Errorf("invalid release tag %q (want v2.MINOR.PATCH with an optional SemVer prerelease suffix)", tag)
	}
	if matches[1] != releaseMajorVersion {
		return false, fmt.Errorf("invalid release tag %q: release major must be v%s", tag, releaseMajorVersion)
	}
	prerelease := matches[4]
	for _, identifier := range strings.Split(prerelease, ".") {
		if len(identifier) > 1 && identifier[0] == '0' && numericIdentifier(identifier) {
			return false, fmt.Errorf("invalid release tag %q: numeric prerelease identifiers must not contain leading zeroes", tag)
		}
	}
	return prerelease != "", nil
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
