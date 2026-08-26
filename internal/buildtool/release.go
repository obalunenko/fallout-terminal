package buildtool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const darwinReleaseArchiveName = "Fallout-Terminal-arm64.dmg"

// ReleaseCandidateResult describes the exact files eligible for publication.
type ReleaseCandidateResult struct {
	SourceSHA       string
	OutputDirectory string
	Files           []string
}

// BuildReleaseCandidate builds the complete unsigned release inventory from
// the current checkout. The Darwin application is built natively; portable
// targets use the same Docker packaging path as PackageAllDocker.
func BuildReleaseCandidate(
	ctx context.Context,
	root string,
	outputDirectory string,
	report func(AggregateTargetRecord),
) (ReleaseCandidateResult, error) {
	result := ReleaseCandidateResult{}
	if err := validateRoot(root); err != nil {
		return result, err
	}
	if ctx == nil {
		return result, errors.New("release candidate context is required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if outputDirectory == "" {
		return result, errors.New("release candidate output directory must not be empty")
	}

	output := outputDirectory
	if !filepath.IsAbs(output) {
		output = filepath.Join(root, output)
	}
	result.OutputDirectory = filepath.Clean(output)
	if err := validateDockerAggregateOutput(root, result.OutputDirectory); err != nil {
		return result, err
	}
	if err := validateLocalPackageAllHost(RuntimeHost()); err != nil {
		return result, err
	}

	workRoot, err := createDockerPackageWorkRoot(result.OutputDirectory)
	if err != nil {
		return result, err
	}
	cleanupWorkRoot := true
	defer func() {
		if cleanupWorkRoot {
			_ = os.RemoveAll(workRoot)
		}
	}()

	matrixDirectory := filepath.Join(workRoot, "matrix")
	matrix, err := PackageAllDocker(ctx, root, matrixDirectory, report)
	if err != nil {
		return result, err
	}
	result.SourceSHA = matrix.SourceSHA

	candidateDirectory := filepath.Join(workRoot, "candidate")
	if err := os.Mkdir(candidateDirectory, 0o755); err != nil {
		return result, fmt.Errorf("create release candidate quarantine: %w", err)
	}
	if err := assembleReleaseCandidate(ctx, root, matrix, candidateDirectory); err != nil {
		return result, err
	}
	files, err := verifyReleaseCandidate(ctx, candidateDirectory, matrix.SourceSHA)
	if err != nil {
		return result, err
	}

	if err := validateDockerAggregateOutput(root, result.OutputDirectory); err != nil {
		return result, err
	}
	if err := publishDockerAggregateOutput(candidateDirectory, result.OutputDirectory, workRoot); err != nil {
		previousDirectory := filepath.Join(workRoot, "previous-output")
		if _, recoveryErr := os.Lstat(previousDirectory); !errors.Is(recoveryErr, os.ErrNotExist) {
			cleanupWorkRoot = false
			if recoveryErr != nil {
				return result, errors.Join(
					err,
					fmt.Errorf("inspect preserved release candidate %q: %w", previousDirectory, recoveryErr),
				)
			}
			return result, fmt.Errorf("%w; previous output retained at %q", err, previousDirectory)
		}
		return result, err
	}
	result.Files = files
	return result, nil
}

func assembleReleaseCandidate(
	ctx context.Context,
	root string,
	matrix AggregateResult,
	candidateDirectory string,
) error {
	for _, name := range portableReleaseFileNames() {
		if err := os.Rename(
			filepath.Join(matrix.OutputDirectory, name),
			filepath.Join(candidateDirectory, name),
		); err != nil {
			return fmt.Errorf("stage release file %q: %w", name, err)
		}
	}

	dmgPath := filepath.Join(candidateDirectory, darwinReleaseArchiveName)
	darwinDirectory := filepath.Dir(matrix.DarwinBundlePath)
	if err := createUnsignedDMG(ctx, root, darwinDirectory, dmgPath); err != nil {
		return err
	}
	digest, err := hashFile(ctx, dmgPath)
	if err != nil {
		return fmt.Errorf("hash Darwin release archive: %w", err)
	}
	sidecar := digest + "  " + darwinReleaseArchiveName + "\n"
	if err := os.WriteFile(dmgPath+".sha256", []byte(sidecar), 0o644); err != nil {
		return fmt.Errorf("write Darwin release checksum: %w", err)
	}
	return nil
}

func createUnsignedDMG(ctx context.Context, root, sourceDirectory, destination string) error {
	command := exec.CommandContext(
		ctx,
		"/usr/bin/hdiutil",
		"create",
		"-volname", applicationName,
		"-srcfolder", sourceDirectory,
		"-ov",
		"-format", "UDZO",
		destination,
	)
	command.Dir = root
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		detail := strings.TrimSpace(output.String())
		if detail == "" {
			return fmt.Errorf("create unsigned Darwin DMG: %w", err)
		}
		return fmt.Errorf("create unsigned Darwin DMG: %s: %w", detail, err)
	}
	return nil
}

func verifyReleaseCandidate(ctx context.Context, directory, sourceSHA string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read release candidate: %w", err)
	}
	want := releaseCandidateFileNames()
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect release candidate file %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return nil, fmt.Errorf("release candidate entry %q must be a non-empty regular file", entry.Name())
		}
		actual = append(actual, entry.Name())
	}
	slices.Sort(actual)
	if !slices.Equal(actual, want) {
		return nil, fmt.Errorf("release candidate inventory mismatch: got %q, want %q", actual, want)
	}

	artifacts := make([]VerifiedArtifact, 0, len(portableMatrixTargets()))
	for _, target := range portableMatrixTargets() {
		archivePath := filepath.Join(directory, target.ArchiveName())
		verified, err := VerifyArtifact(ctx, archivePath, archivePath+".sha256", target)
		if err != nil {
			return nil, fmt.Errorf("verify release candidate %s: %w", target, err)
		}
		artifacts = append(artifacts, verified)
	}
	if err := verifyAggregateIndex(
		ctx,
		filepath.Join(directory, portableAggregateIndexName),
		"",
		sourceSHA,
		artifacts,
	); err != nil {
		return nil, fmt.Errorf("verify release candidate aggregate index: %w", err)
	}
	if _, err := verifyArtifactChecksum(
		ctx,
		filepath.Join(directory, darwinReleaseArchiveName),
		filepath.Join(directory, darwinReleaseArchiveName+".sha256"),
		darwinReleaseArchiveName,
	); err != nil {
		return nil, fmt.Errorf("verify Darwin release archive: %w", err)
	}
	return actual, nil
}

func portableReleaseFileNames() []string {
	files := []string{portableAggregateIndexName}
	for _, target := range portableMatrixTargets() {
		files = append(files, target.ArchiveName(), target.ArchiveName()+".sha256")
	}
	return files
}

func releaseCandidateFileNames() []string {
	files := append([]string(nil), portableReleaseFileNames()...)
	files = append(files, darwinReleaseArchiveName, darwinReleaseArchiveName+".sha256")
	slices.Sort(files)
	return files
}
