package buildtool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const portableDockerfilePath = "build/docker/Dockerfile.package"

// PackageAllDocker builds and atomically publishes the complete portable
// matrix from the current checkout using architecture-matched Linux
// containers. The checkout does not need to be clean or pushed.
func PackageAllDocker(
	ctx context.Context,
	root string,
	outputDirectory string,
	report func(AggregateTargetRecord),
) (AggregateResult, error) {
	result := AggregateResult{}
	if err := validateRoot(root); err != nil {
		return result, err
	}
	if ctx == nil {
		return result, errors.New("Docker package context is required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if outputDirectory == "" {
		return result, errors.New("Docker package output directory must not be empty")
	}
	output := outputDirectory
	if !filepath.IsAbs(output) {
		output = filepath.Join(root, output)
	}
	result.OutputDirectory = filepath.Clean(output)
	if err := ensureAggregateOutputAbsent(result.OutputDirectory); err != nil {
		return result, err
	}
	if err := requirePortableDockerfile(root); err != nil {
		return result, err
	}
	if err := requireDocker(ctx, root); err != nil {
		return result, err
	}

	revision, err := resolveSourceRevision(ctx, root)
	if err != nil {
		return result, err
	}
	result.SourceSHA = revision
	result.CorrelationID = "local-docker-" + revision[:12]

	workRoot, err := createDockerPackageWorkRoot(result.OutputDirectory)
	if err != nil {
		return result, err
	}
	defer func() {
		_ = os.RemoveAll(workRoot)
	}()
	publishDirectory := filepath.Join(workRoot, "publish")
	if err := os.Mkdir(publishDirectory, 0o755); err != nil {
		return result, fmt.Errorf("create Docker package quarantine: %w", err)
	}

	records := make([]AggregateTargetRecord, 0, len(portableMatrixTargets()))
	for _, target := range portableMatrixTargets() {
		targetDirectory := filepath.Join(workRoot, target.OS()+"-"+target.Arch())
		record, err := packageDockerTarget(
			ctx,
			root,
			targetDirectory,
			publishDirectory,
			target,
			revision,
			report,
		)
		records = append(records, record)
		if err != nil {
			result.Records = cloneAggregateRecords(records)
			return result, err
		}
	}
	result.Records = cloneAggregateRecords(records)

	if err := writeLocalAggregateIndex(publishDirectory, result.CorrelationID, revision, records); err != nil {
		return result, err
	}
	artifacts, err := (directoryAggregateVerifier{}).Verify(ctx, publishDirectory)
	if err != nil {
		return result, fmt.Errorf("verify complete Docker package matrix: %w", err)
	}
	if err := validateAggregateRecords(records, revision); err != nil {
		return result, err
	}
	if err := validateAggregateArtifacts(artifacts, records, revision); err != nil {
		return result, err
	}
	result.Artifacts = append([]VerifiedArtifact(nil), artifacts...)

	if err := ensureAggregateOutputAbsent(result.OutputDirectory); err != nil {
		return result, err
	}
	if err := os.Rename(publishDirectory, result.OutputDirectory); err != nil {
		return result, fmt.Errorf("atomically publish Docker package matrix: %w", err)
	}
	return result, nil
}

func packageDockerTarget(
	ctx context.Context,
	root string,
	targetDirectory string,
	publishDirectory string,
	target Target,
	revision string,
	report func(AggregateTargetRecord),
) (AggregateTargetRecord, error) {
	record := AggregateTargetRecord{
		Target:      target,
		SourceSHA:   revision,
		Status:      AggregateTargetBuilding,
		ArchiveName: target.ArchiveName(),
	}
	reportAggregateTarget(report, record)

	if err := buildPortableDockerTarget(ctx, root, targetDirectory, target, revision); err != nil {
		return failDockerTarget(record, report, fmt.Errorf("package %s with Docker: %w", target, err))
	}
	if err := collectDockerTarget(targetDirectory, publishDirectory, target); err != nil {
		return failDockerTarget(record, report, fmt.Errorf("collect %s Docker package: %w", target, err))
	}
	verified, err := VerifyArtifact(
		ctx,
		filepath.Join(publishDirectory, target.ArchiveName()),
		filepath.Join(publishDirectory, target.ArchiveName()+".sha256"),
		target,
	)
	if err != nil {
		return failDockerTarget(record, report, fmt.Errorf("verify %s Docker package: %w", target, err))
	}
	record.Status = AggregateTargetEligible
	record.Checksum = verified.Checksum()
	reportAggregateTarget(report, record)
	return record, nil
}

func failDockerTarget(
	record AggregateTargetRecord,
	report func(AggregateTargetRecord),
	err error,
) (AggregateTargetRecord, error) {
	record.Status = AggregateTargetFailed
	record.Failure = err
	reportAggregateTarget(report, record)
	return record, err
}

func requirePortableDockerfile(root string) error {
	path := filepath.Join(root, portableDockerfilePath)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("portable Dockerfile %q is unavailable: %w", portableDockerfilePath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("portable Dockerfile %q is not a regular file", portableDockerfilePath)
	}
	return nil
}

func requireDocker(ctx context.Context, root string) error {
	command := exec.CommandContext(ctx, "docker", "info")
	command.Dir = root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("Docker is required; install Docker and start its daemon")
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("Docker daemon is unavailable: %s", detail)
	}
	return nil
}

func createDockerPackageWorkRoot(outputDirectory string) (string, error) {
	parent := filepath.Dir(outputDirectory)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create Docker package output parent: %w", err)
	}
	workRoot, err := os.MkdirTemp(parent, ".fallout-terminal-package-all-")
	if err != nil {
		return "", fmt.Errorf("create Docker package work directory: %w", err)
	}
	return workRoot, nil
}

func buildPortableDockerTarget(
	ctx context.Context,
	root string,
	outputDirectory string,
	target Target,
	revision string,
) error {
	arguments := []string{
		"build",
		"--file", portableDockerfilePath,
		"--platform", "linux/" + target.Arch(),
		"--target", "artifact",
		"--build-arg", "TARGETOS=" + target.OS(),
		"--build-arg", "TARGETARCH=" + target.Arch(),
		"--build-arg", "SOURCE_REVISION=" + revision,
		"--output", "type=local,dest=" + outputDirectory,
		root,
	}
	command := exec.CommandContext(ctx, "docker", arguments...)
	command.Dir = root
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("Docker build failed: %w", err)
	}
	return nil
}

func collectDockerTarget(sourceDirectory, publishDirectory string, target Target) error {
	entries, err := os.ReadDir(sourceDirectory)
	if err != nil {
		return fmt.Errorf("read Docker output: %w", err)
	}
	expected := map[string]struct{}{
		target.ArchiveName():             {},
		target.ArchiveName() + ".sha256": {},
	}
	if len(entries) != len(expected) {
		return fmt.Errorf("Docker output requires exactly two files, got %d", len(entries))
	}
	for _, entry := range entries {
		if _, ok := expected[entry.Name()]; !ok {
			return fmt.Errorf("Docker output contains unexpected entry %q", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect Docker output %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Docker output %q is not a regular file", entry.Name())
		}
		if err := os.Rename(
			filepath.Join(sourceDirectory, entry.Name()),
			filepath.Join(publishDirectory, entry.Name()),
		); err != nil {
			return fmt.Errorf("quarantine Docker output %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func writeLocalAggregateIndex(
	directory string,
	correlationID string,
	revision string,
	records []AggregateTargetRecord,
) error {
	index := aggregateIndexDocument{
		SchemaVersion:  1,
		CorrelationID:  correlationID,
		SourceRevision: revision,
		Artifacts:      make([]aggregateIndexArtifact, 0, len(records)),
	}
	for _, record := range records {
		index.Artifacts = append(index.Artifacts, aggregateIndexArtifact{
			Target:      record.Target.String(),
			ArchiveName: record.ArchiveName,
			Checksum:    record.Checksum,
		})
	}
	contents, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("encode local aggregate index: %w", err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(filepath.Join(directory, portableAggregateIndexName), contents, 0o644); err != nil {
		return fmt.Errorf("write local aggregate index: %w", err)
	}
	return nil
}

func reportAggregateTarget(report func(AggregateTargetRecord), record AggregateTargetRecord) {
	if report != nil {
		report(record)
	}
}
