package buildtool

import (
	"bytes"
	"context"
	"debug/macho"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const portableDockerfilePath = "build/docker/Dockerfile.package"

// PackageAllDocker builds and atomically publishes the native Darwin bundle
// and complete portable matrix from the current checkout. Darwin uses the
// canonical native package plan; portable targets use architecture-matched
// Linux containers. The checkout does not need to be clean or pushed.
func PackageAllDocker(
	ctx context.Context,
	root string,
	outputDirectory string,
	report func(LocalPackageTargetRecord),
) (LocalPackageResult, error) {
	result := LocalPackageResult{}
	if err := validateRoot(root); err != nil {
		return result, err
	}
	if ctx == nil {
		return result, errors.New("docker package context is required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if outputDirectory == "" {
		return result, errors.New("docker package output directory must not be empty")
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
	cleanupWorkRoot := true
	defer func() {
		if cleanupWorkRoot {
			_ = os.RemoveAll(workRoot)
		}
	}()
	publishDirectory := filepath.Join(workRoot, "publish")
	if err := os.Mkdir(publishDirectory, 0o755); err != nil {
		return result, fmt.Errorf("create Docker package quarantine: %w", err)
	}
	executableDirectory := filepath.Join(workRoot, "bin")
	if err := os.Mkdir(executableDirectory, 0o755); err != nil {
		return result, fmt.Errorf("create Docker executable quarantine: %w", err)
	}
	darwinBundleDirectory := filepath.Join(executableDirectory, "darwin-arm64", applicationName+".app")
	if err := packageDarwinAggregateBundle(ctx, root, darwinBundleDirectory); err != nil {
		return result, err
	}

	records := make([]LocalPackageTargetRecord, 0, len(localDockerTargets()))
	for _, target := range localDockerTargets() {
		targetDirectory := filepath.Join(workRoot, target.OS()+"-"+target.Arch())
		record, err := packageDockerTarget(
			ctx,
			root,
			targetDirectory,
			publishDirectory,
			executableDirectory,
			target,
			revision,
			report,
		)
		records = append(records, record)
		if err != nil {
			result.Records = cloneLocalPackageRecords(records)
			return result, err
		}
	}
	result.Records = cloneLocalPackageRecords(records)

	if err := writeLocalAggregateIndex(publishDirectory, result.CorrelationID, revision, records); err != nil {
		return result, err
	}
	artifacts, err := (directoryLocalAggregateVerifier{}).Verify(ctx, publishDirectory)
	if err != nil {
		return result, fmt.Errorf("verify complete Docker package matrix: %w", err)
	}
	if err := validateLocalPackageRecords(records, revision); err != nil {
		return result, err
	}
	if err := validateLocalPackageArtifacts(artifacts, records, revision); err != nil {
		return result, err
	}
	result.Artifacts = append([]LocalVerifiedArtifact(nil), artifacts...)
	if err := os.Rename(executableDirectory, filepath.Join(publishDirectory, "bin")); err != nil {
		return result, fmt.Errorf("publish verified Docker executables: %w", err)
	}

	if err := validateDockerAggregateOutput(root, result.OutputDirectory); err != nil {
		return result, err
	}
	if err := publishDockerAggregateOutput(publishDirectory, result.OutputDirectory, workRoot); err != nil {
		previousDirectory := filepath.Join(workRoot, "previous-output")
		if _, recoveryErr := os.Lstat(previousDirectory); !errors.Is(recoveryErr, os.ErrNotExist) {
			cleanupWorkRoot = false
			if recoveryErr != nil {
				return result, errors.Join(
					err,
					fmt.Errorf("inspect preserved Docker aggregate output %q: %w", previousDirectory, recoveryErr),
				)
			}
			return result, fmt.Errorf("%w; previous output retained at %q", err, previousDirectory)
		}
		return result, err
	}
	result.DarwinBundlePath = filepath.Join(
		result.OutputDirectory,
		"bin",
		"darwin-arm64",
		applicationName+".app",
	)
	return result, nil
}

func validateLocalPackageAllHost(host Host) error {
	if err := ValidateHost(DefaultTarget(), host); err != nil {
		return fmt.Errorf("local package-all requires the supported darwin/arm64 host: %w", err)
	}
	return nil
}

func packageDarwinAggregateBundle(ctx context.Context, root string, destination string) error {
	if err := Run(ctx, root, "package", nil); err != nil {
		return fmt.Errorf("package darwin/arm64 application: %w", err)
	}
	if err := os.Mkdir(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create darwin/arm64 aggregate directory: %w", err)
	}
	source := filepath.Join(root, "build", "bin", applicationName+".app")
	if err := copyDarwinBundle(ctx, source, destination); err != nil {
		return fmt.Errorf("copy darwin/arm64 application into aggregate: %w", err)
	}
	if err := verifyDarwinBundleInventory(destination); err != nil {
		return fmt.Errorf("verify darwin/arm64 application inventory: %w", err)
	}
	if err := verifyDarwinBundleExecutable(destination); err != nil {
		return fmt.Errorf("verify darwin/arm64 application executable: %w", err)
	}
	if err := verifyDarwinBundleSignature(ctx, root, destination); err != nil {
		return fmt.Errorf("verify darwin/arm64 application signature: %w", err)
	}
	return nil
}

func copyDarwinBundle(ctx context.Context, source string, destination string) error {
	if ctx == nil {
		return errors.New("darwin bundle copy context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect source application bundle %q: %w", source, err)
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.IsDir() {
		return fmt.Errorf("source application bundle must be a directory and not a symlink: %q", source)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return fmt.Errorf("inspect destination application bundle %q: %w", destination, err)
		}
		return fmt.Errorf("destination application bundle already exists: %q", destination)
	}

	type directoryMode struct {
		path string
		mode os.FileMode
	}
	directories := make([]directoryMode, 0, 8)
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect application bundle entry %q: %w", path, err)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("resolve application bundle entry %q: %w", path, err)
		}
		targetPath := destination
		if relative != "." {
			targetPath = filepath.Join(destination, relative)
		}
		switch {
		case info.IsDir():
			if err := os.Mkdir(targetPath, 0o755); err != nil {
				return fmt.Errorf("create application bundle directory %q: %w", targetPath, err)
			}
			directories = append(directories, directoryMode{path: targetPath, mode: info.Mode().Perm()})
			return nil
		case info.Mode().IsRegular():
			return copyDarwinBundleFile(ctx, path, targetPath, info.Mode().Perm())
		default:
			return fmt.Errorf("application bundle entry %q is not a regular file or directory", path)
		}
	})
	if err != nil {
		return err
	}
	for _, directory := range slices.Backward(directories) {
		if err := os.Chmod(directory.path, directory.mode); err != nil {
			return fmt.Errorf("preserve application bundle directory mode %q: %w", directory.path, err)
		}
	}
	return nil
}

func copyDarwinBundleFile(ctx context.Context, source string, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open application bundle file %q: %w", source, err)
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = input.Close()
		return fmt.Errorf("create application bundle file %q: %w", destination, err)
	}
	_, copyErr := copyWithContext(ctx, output, input)
	outputCloseErr := output.Close()
	inputCloseErr := input.Close()
	if copyErr != nil {
		return fmt.Errorf("copy application bundle file %q: %w", source, copyErr)
	}
	if outputCloseErr != nil {
		return fmt.Errorf("close copied application bundle file %q: %w", destination, outputCloseErr)
	}
	if inputCloseErr != nil {
		return fmt.Errorf("close source application bundle file %q: %w", source, inputCloseErr)
	}
	if err := os.Chmod(destination, mode); err != nil {
		return fmt.Errorf("preserve application bundle file mode %q: %w", destination, err)
	}
	return nil
}

func verifyDarwinBundleInventory(bundle string) error {
	required := []string{
		filepath.Join("Contents", "Info.plist"),
		filepath.Join("Contents", "MacOS", applicationName),
		filepath.Join("Contents", "Resources", "icon.icns"),
		filepath.Join("Contents", "Resources", "THIRD_PARTY_NOTICES.md"),
		filepath.Join("Contents", "Resources", "sessions", "demo.json"),
		filepath.Join("Contents", "Resources", "sessions", "demo-players.json"),
		filepath.Join("Contents", "_CodeSignature", "CodeResources"),
	}
	for _, relative := range required {
		if err := requireRegularVerificationFile(
			"Darwin application bundle",
			filepath.Join(bundle, relative),
			maxVerificationEntrySize,
		); err != nil {
			return err
		}
	}
	executable := filepath.Join(bundle, "Contents", "MacOS", applicationName)
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("inspect Darwin application executable: %w", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("darwin application executable is not executable: %q", executable)
	}
	return nil
}

func verifyDarwinBundleExecutable(bundle string) error {
	executable := filepath.Join(bundle, "Contents", "MacOS", applicationName)
	file, err := macho.Open(executable)
	if err != nil {
		return fmt.Errorf("inspect Mach-O executable %q: %w", executable, err)
	}
	if file.Cpu != macho.CpuArm64 {
		_ = file.Close()
		return fmt.Errorf("darwin application executable CPU is %s, want %s", file.Cpu, macho.CpuArm64)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Mach-O executable %q: %w", executable, err)
	}
	return nil
}

func verifyDarwinBundleSignature(ctx context.Context, root string, bundle string) error {
	command := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", bundle)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("codesign verification failed: %s", detail)
	}
	return nil
}

func validateDockerAggregateOutput(root string, outputDirectory string) error {
	root = filepath.Clean(root)
	outputDirectory = filepath.Clean(outputDirectory)
	if outputDirectory == filepath.VolumeName(outputDirectory)+string(filepath.Separator) {
		return fmt.Errorf("docker aggregate output must not be a filesystem root: %q", outputDirectory)
	}
	relativeRoot, err := filepath.Rel(outputDirectory, root)
	if err != nil {
		return fmt.Errorf("compare Docker aggregate output with repository root: %w", err)
	}
	outputContainsRoot := relativeRoot == "." ||
		(relativeRoot != ".." && !strings.HasPrefix(relativeRoot, ".."+string(filepath.Separator)))
	if outputContainsRoot {
		return fmt.Errorf("docker aggregate output must not contain the repository root: %q", outputDirectory)
	}

	info, err := os.Lstat(outputDirectory)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("inspect Docker aggregate output %q: %w", outputDirectory, err)
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("docker aggregate output must not be a symlink: %q", outputDirectory)
	case !info.IsDir():
		return fmt.Errorf("docker aggregate output must be a directory: %q", outputDirectory)
	}

	defaultOutput := filepath.Join(root, "build", "dist")
	if outputDirectory == defaultOutput {
		return nil
	}
	indexPath := filepath.Join(outputDirectory, localAggregateIndexName)
	indexInfo, err := os.Lstat(indexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(
				"refusing to replace unrecognized Docker aggregate output %q: %s is missing",
				outputDirectory,
				localAggregateIndexName,
			)
		}
		return fmt.Errorf("inspect Docker aggregate marker %q: %w", indexPath, err)
	}
	if indexInfo.Mode()&os.ModeSymlink != 0 || !indexInfo.Mode().IsRegular() {
		return fmt.Errorf("docker aggregate marker must be a regular file: %q", indexPath)
	}
	return nil
}

func publishDockerAggregateOutput(publishDirectory string, outputDirectory string, workRoot string) error {
	_, err := os.Lstat(outputDirectory)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Rename(publishDirectory, outputDirectory); err != nil {
			return fmt.Errorf("atomically publish Docker package matrix: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect previous Docker aggregate output %q: %w", outputDirectory, err)
	}

	previousDirectory := filepath.Join(workRoot, "previous-output")
	if err := os.Rename(outputDirectory, previousDirectory); err != nil {
		return fmt.Errorf("preserve previous Docker aggregate output: %w", err)
	}
	if err := os.Rename(publishDirectory, outputDirectory); err != nil {
		publishErr := fmt.Errorf("publish replacement Docker package matrix: %w", err)
		if rollbackErr := os.Rename(previousDirectory, outputDirectory); rollbackErr != nil {
			return errors.Join(
				publishErr,
				fmt.Errorf("restore previous Docker aggregate output: %w", rollbackErr),
			)
		}
		return publishErr
	}
	return nil
}

func packageDockerTarget(
	ctx context.Context,
	root string,
	targetDirectory string,
	publishDirectory string,
	executableDirectory string,
	target Target,
	revision string,
	report func(LocalPackageTargetRecord),
) (LocalPackageTargetRecord, error) {
	record := LocalPackageTargetRecord{
		Target:      target,
		SourceSHA:   revision,
		Status:      LocalPackageTargetBuilding,
		ArchiveName: target.ArchiveName(),
	}
	reportLocalPackageTarget(report, record)

	if err := buildPortableDockerTarget(ctx, root, targetDirectory, target, revision); err != nil {
		return failDockerTarget(record, report, fmt.Errorf("package %s with Docker: %w", target, err))
	}
	payloadDirectory, err := collectDockerTarget(
		targetDirectory,
		publishDirectory,
		executableDirectory,
		target,
	)
	if err != nil {
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
	if err := verifyDockerExecutablePayload(
		ctx,
		payloadDirectory,
		filepath.Join(publishDirectory, target.ArchiveName()),
		target,
	); err != nil {
		return failDockerTarget(record, report, fmt.Errorf("verify %s Docker executable: %w", target, err))
	}
	record.Status = LocalPackageTargetEligible
	record.Checksum = verified.Checksum()
	reportLocalPackageTarget(report, record)
	return record, nil
}

func failDockerTarget(
	record LocalPackageTargetRecord,
	report func(LocalPackageTargetRecord),
	err error,
) (LocalPackageTargetRecord, error) {
	record.Status = LocalPackageTargetFailed
	record.Failure = err
	reportLocalPackageTarget(report, record)
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
	return requireDockerWith(ctx, root, runDockerInfo)
}

type dockerInfoRunner func(context.Context, string) (string, error)

func requireDockerWith(ctx context.Context, root string, run dockerInfoRunner) error {
	if run == nil {
		return errors.New("docker prerequisite probe is unavailable; install Docker and start its daemon")
	}
	detail, err := run(ctx, root)
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, exec.ErrNotFound) {
		return errors.New("docker is required; install Docker and start its daemon")
	}
	detail = strings.TrimSpace(detail)
	if detail == "" {
		detail = "Docker returned no diagnostic"
	}
	return fmt.Errorf(
		"docker daemon is unavailable: %s; start Docker and verify `docker info` succeeds: %w",
		detail,
		err,
	)
}

func runDockerInfo(ctx context.Context, root string) (string, error) {
	command := exec.CommandContext(ctx, "docker", "info")
	command.Dir = root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	return stderr.String(), err
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
	var stderr bytes.Buffer
	command.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return dockerBuildFailure(target, stderr.String(), err)
	}
	return nil
}

func dockerBuildFailure(target Target, detail string, err error) error {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		detail = "Docker returned no diagnostic"
	}
	return fmt.Errorf(
		"docker build for %s failed: %s; ensure Docker BuildKit can execute linux/%s (enable containerd image storage or install the matching binfmt handler): %w",
		target,
		detail,
		target.Arch(),
		err,
	)
}

func collectDockerTarget(
	sourceDirectory string,
	publishDirectory string,
	executableDirectory string,
	target Target,
) (string, error) {
	entries, err := os.ReadDir(sourceDirectory)
	if err != nil {
		return "", fmt.Errorf("read Docker output: %w", err)
	}
	if len(entries) != 2 || entries[0].Name() != "bin" || entries[1].Name() != "dist" ||
		!entries[0].IsDir() || !entries[1].IsDir() {
		return "", errors.New("docker output requires exactly the bin and dist directories")
	}
	distDirectory := filepath.Join(sourceDirectory, "dist")
	distEntries, err := os.ReadDir(distDirectory)
	if err != nil {
		return "", fmt.Errorf("read Docker archive output: %w", err)
	}
	expected := map[string]struct{}{
		target.ArchiveName():             {},
		target.ArchiveName() + ".sha256": {},
	}
	if len(distEntries) != len(expected) {
		return "", fmt.Errorf("docker archive output requires exactly two files, got %d", len(distEntries))
	}
	for _, entry := range distEntries {
		if _, ok := expected[entry.Name()]; !ok {
			return "", fmt.Errorf("docker archive output contains unexpected entry %q", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return "", fmt.Errorf("inspect Docker archive output %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("docker archive output %q is not a regular file", entry.Name())
		}
		if err := os.Rename(
			filepath.Join(distDirectory, entry.Name()),
			filepath.Join(publishDirectory, entry.Name()),
		); err != nil {
			return "", fmt.Errorf("quarantine Docker archive output %q: %w", entry.Name(), err)
		}
	}

	targetName := target.OS() + "-" + target.Arch()
	dockerBinDirectory := filepath.Join(sourceDirectory, "bin")
	binEntries, err := os.ReadDir(dockerBinDirectory)
	if err != nil {
		return "", fmt.Errorf("read Docker executable output: %w", err)
	}
	if len(binEntries) != 1 || binEntries[0].Name() != targetName || !binEntries[0].IsDir() {
		return "", fmt.Errorf("docker executable output requires exactly the %s directory", targetName)
	}
	dockerPayload := filepath.Join(dockerBinDirectory, targetName)
	payloadDirectory := filepath.Join(executableDirectory, targetName)
	if err := os.Rename(dockerPayload, payloadDirectory); err != nil {
		return "", fmt.Errorf("quarantine Docker executable payload: %w", err)
	}
	return payloadDirectory, nil
}

func verifyDockerExecutablePayload(
	ctx context.Context,
	directory string,
	archivePath string,
	target Target,
) error {
	files, err := inspectArtifactArchive(ctx, archivePath, target)
	if err != nil {
		return fmt.Errorf("inspect verified archive: %w", err)
	}

	expected := requiredArchivePaths(target)
	actual := make([]string, 0, len(expected))
	err = filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == directory || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("payload entry %q is not a regular file", path)
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		actual = append(actual, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return fmt.Errorf("inspect executable payload: %w", err)
	}
	slices.Sort(actual)
	if !slices.Equal(actual, expected) {
		return fmt.Errorf("executable payload inventory mismatch: got %q, want %q", actual, expected)
	}
	for _, relative := range expected {
		path := filepath.Join(directory, filepath.FromSlash(relative))
		if err := requireRegularVerificationFile("executable payload", path, maxVerificationEntrySize); err != nil {
			return err
		}
		checksum, err := hashFile(ctx, path)
		if err != nil {
			return fmt.Errorf("hash executable payload %q: %w", relative, err)
		}
		if checksum != files[relative].sha256 {
			return fmt.Errorf("executable payload %q does not match its verified archive", relative)
		}
	}
	return nil
}

func writeLocalAggregateIndex(
	directory string,
	correlationID string,
	revision string,
	records []LocalPackageTargetRecord,
) error {
	index := localAggregateIndexDocument{
		SchemaVersion:  1,
		CorrelationID:  correlationID,
		SourceRevision: revision,
		Artifacts:      make([]localAggregateIndexArtifact, 0, len(records)),
	}
	for _, record := range records {
		index.Artifacts = append(index.Artifacts, localAggregateIndexArtifact{
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
	if err := os.WriteFile(filepath.Join(directory, localAggregateIndexName), contents, 0o644); err != nil {
		return fmt.Errorf("write local aggregate index: %w", err)
	}
	return nil
}

func reportLocalPackageTarget(report func(LocalPackageTargetRecord), record LocalPackageTargetRecord) {
	if report != nil {
		report(record)
	}
}
