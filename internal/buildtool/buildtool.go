// Package buildtool owns the repository's dependency-free development, native
// build, and packaging graph. It deliberately uses only the Go standard library;
// versioned development tools continue to run through their isolated Go modules.
package buildtool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	applicationName = "Fallout Terminal"
	minimumMacOS    = "13.0"
)

const (
	applicationModule                 = "github.com/obalunenko/Fallout-Terminal/v2"
	macOSNoWarnDuplicateLibrariesFlag = "-Wl,-no_warn_duplicate_libraries"
)

// macOSCGOLinkerFlags keeps the supported deployment target while silencing
// ld's harmless duplicate-library warning when Wails CGO packages each link Objective-C.
func macOSCGOLinkerFlags(deploymentTarget string) string {
	return "-mmacosx-version-min=" + deploymentTarget + " " + macOSNoWarnDuplicateLibrariesFlag
}

type operation uint8

const (
	runCommand operation = iota
	removeTree
	makeDirectory
	copyFile
	changeMode
	runPreflight
	renderTemplate
)

// Step is one deterministic node in the repository build graph.
type Step struct {
	Name        string
	Operation   operation
	Program     string
	Arguments   []string
	Environment map[string]string
	Path        string
	Source      string
	Destination string
	Mode        os.FileMode
	preflight   preflightKind
	target      Target
}

// Plan returns the ordered, nonrecursive graph for an action.
func Plan(action string, applicationArguments []string) ([]Step, error) {
	target := DefaultTarget()
	switch action {
	case "prepare":
		return preparePlan(), nil
	case "build":
		return append(preparePlan(), implicitBuildSteps(target)...), nil
	case "dev", "run":
		steps := append(preparePlan(), developmentSteps()...)
		return append(steps, commandStep("run development application", developmentExecutable(), applicationArguments...)), nil
	case "package":
		return implicitPackagePlan()
	default:
		return nil, fmt.Errorf("unknown action %q (want dev, build, package, run, or prepare)", action)
	}
}

// PlanForTarget returns the ordered, nonrecursive graph for an action and
// immutable build target. Portable package assembly is added by the package
// planner; this shared graph owns preparation and native compilation only.
func PlanForTarget(action string, target Target, applicationArguments []string) ([]Step, error) {
	if !target.valid() {
		return nil, fmt.Errorf("invalid build target %s", target.String())
	}

	switch action {
	case "prepare":
		return preparePlan(), nil
	case "build":
		return append(preparePlan(), buildSteps(target)...), nil
	case "dev", "run":
		if target != DefaultTarget() {
			return nil, fmt.Errorf("action %q is available only for %s, got %s", action, DefaultTarget(), target)
		}
		steps := append(preparePlan(), developmentSteps()...)
		return append(steps, commandStep("run development application", developmentExecutable(), applicationArguments...)), nil
	case "package":
		if target.Portable() {
			plan, err := NewPackagePlan(target, NewHost(target.OS(), target.Arch()))
			if err != nil {
				return nil, err
			}
			return plan.Actions(), nil
		}
		return implicitPackagePlan()
	default:
		return nil, fmt.Errorf("unknown action %q (want dev, build, package, run, or prepare)", action)
	}
}

func preparePlan() []Step {
	return []Step{
		commandStep("install locked frontend dependencies", "npm", "ci", "--prefix", "frontend"),
		preflightStep("verify protobuf and generated clients", verifyProtobufAndGeneratedClients, Target{}),
		commandStep("build client frontend", "npm", "run", "build:client", "--prefix", "frontend"),
		commandStep("generate Wails bindings", "go", "tool", "-modfile=tools/wails/go.mod", "wails3", "generate", "bindings", "-clean", "-d", "frontend/overseer/bindings", "./..."),
		commandStep("build Overseer frontend", "npm", "run", "build:overseer", "--prefix", "frontend"),
	}
}

func buildSteps(target Target) []Step {
	outputDirectory := filepath.Join("build", "bin")
	if target.Portable() {
		outputDirectory = filepath.Join(outputDirectory, target.OS()+"-"+target.Arch())
	}
	steps := make([]Step, 0, 3)
	if target.Portable() {
		steps = append(steps, preflightStep("verify "+target.String()+" native build prerequisites", verifyNativeBuildPrerequisites, target))
	}
	steps = append(steps,
		Step{Name: "create binary output directory", Operation: makeDirectory, Path: outputDirectory, Mode: 0o755},
		compileStep(target, filepath.Join(outputDirectory, target.ExecutableName())),
	)
	return steps
}

func implicitBuildSteps(target Target) []Step {
	return []Step{
		{Name: "create binary output directory", Operation: makeDirectory, Path: filepath.Join("build", "bin"), Mode: 0o755},
		compileStep(target, filepath.Join("build", "bin", target.ExecutableName())),
	}
}

func developmentSteps() []Step {
	app := developmentBundle()
	contents := filepath.Join(app, "Contents")
	macOS := filepath.Join(contents, "MacOS")
	resources := filepath.Join(contents, "Resources")
	executable := filepath.Join(macOS, applicationName)

	return []Step{
		{Name: "remove previous development application bundle", Operation: removeTree, Path: app},
		{Name: "create development application executable directory", Operation: makeDirectory, Path: macOS, Mode: 0o755},
		{Name: "create development bundled session directory", Operation: makeDirectory, Path: filepath.Join(resources, "sessions"), Mode: 0o755},
		{Name: "install development application metadata", Operation: copyFile, Source: filepath.Join("build", "darwin", "Info.dev.plist"), Destination: filepath.Join(contents, "Info.plist"), Mode: 0o644},
		commandStep("install development application icon", "go", "tool", "-modfile=tools/wails/go.mod", "wails3", "generate", "icons", "-input", filepath.Join("build", "appicon.png"), "-macfilename", filepath.Join(resources, "icon.icns"), "-windowsfilename", filepath.Join(resources, "icon.ico")),
		{Name: "install development bundled demo player config", Operation: copyFile, Source: filepath.Join("sessions", "demo-players.json"), Destination: filepath.Join(resources, "sessions", "demo-players.json"), Mode: 0o444},
		{Name: "install development bundled demo", Operation: copyFile, Source: filepath.Join("sessions", "demo.json"), Destination: filepath.Join(resources, "sessions", "demo.json"), Mode: 0o444},
		compileStep(DefaultTarget(), executable),
		{Name: "make development application executable", Operation: changeMode, Path: executable, Mode: 0o755},
	}
}

func developmentBundle() string {
	return filepath.Join("build", "dev", applicationName+".app")
}

func developmentExecutable() string {
	return filepath.Join(developmentBundle(), "Contents", "MacOS", applicationName)
}

func implicitPackagePlan() ([]Step, error) {
	version, err := ResolveBuildVersion(os.Getenv(packageVersionEnvironmentCanonical))
	if err != nil {
		return nil, fmt.Errorf("resolve package VERSION: %w", err)
	}
	return append(preparePlan(), packageSteps(version)...), nil
}

func packageSteps(version ReleaseVersion) []Step {
	app := filepath.Join("build", "bin", applicationName+".app")
	contents := filepath.Join(app, "Contents")
	macOS := filepath.Join(contents, "MacOS")
	resources := filepath.Join(contents, "Resources")
	executable := filepath.Join(macOS, applicationName)

	return []Step{
		commandStep("verify embedded dependency and license inventory", filepath.Join("scripts", "dependency-license-check.sh")),
		{Name: "remove previous application bundle", Operation: removeTree, Path: app},
		{Name: "create application executable directory", Operation: makeDirectory, Path: macOS, Mode: 0o755},
		{Name: "create bundled session directory", Operation: makeDirectory, Path: filepath.Join(resources, "sessions"), Mode: 0o755},
		versionTemplateStep(
			"install application metadata",
			filepath.Join("build", "darwin", "Info.plist.tmpl"),
			filepath.Join(contents, "Info.plist"),
			version,
		),
		commandStep("install application icon", "go", "tool", "-modfile=tools/wails/go.mod", "wails3", "generate", "icons", "-input", filepath.Join("build", "appicon.png"), "-macfilename", filepath.Join(resources, "icon.icns"), "-windowsfilename", filepath.Join(resources, "icon.ico")),
		{Name: "install bundled demo player config", Operation: copyFile, Source: filepath.Join("sessions", "demo-players.json"), Destination: filepath.Join(resources, "sessions", "demo-players.json"), Mode: 0o444},
		{Name: "install bundled demo", Operation: copyFile, Source: filepath.Join("sessions", "demo.json"), Destination: filepath.Join(resources, "sessions", "demo.json"), Mode: 0o444},
		{Name: "install third-party notices", Operation: copyFile, Source: "THIRD_PARTY_NOTICES.md", Destination: filepath.Join(resources, "THIRD_PARTY_NOTICES.md"), Mode: 0o444},
		versionedCompileStep(DefaultTarget(), executable, version),
		{Name: "make application executable", Operation: changeMode, Path: executable, Mode: 0o755},
		commandStep("sign completed application bundle", "/usr/bin/codesign", "--force", "--deep", "--options", "runtime", "--entitlements", filepath.Join("build", "darwin", "entitlements.plist"), "--sign", "-", app),
	}
}

func compileStep(target Target, output string) Step {
	return compileStepWithVersion(target, output, "")
}

func versionedCompileStep(target Target, output string, version ReleaseVersion) Step {
	linkerVersion := ""
	if version.IsRelease {
		linkerVersion = version.Canonical
	}
	return compileStepWithVersion(target, output, linkerVersion)
}

func compileStepWithVersion(target Target, output, version string) Step {
	name := "compile " + target.String() + " application"
	if target == DefaultTarget() {
		name = "compile macOS arm64 application"
	}
	linkerFlags := "-w -s"
	if target.OS() == goosWindows {
		linkerFlags += " -H windowsgui"
	}
	if version != "" {
		linkerFlags += " -X " + applicationModule + "/internal/version.value=" + version
	}
	step := commandStep(name, "go", "build", "-tags", strings.Join(target.BuildTags(), ","), "-trimpath", "-ldflags="+linkerFlags, "-o", output, ".")
	step.Environment = compileEnvironment(target)
	return step
}

func compileEnvironment(target Target) map[string]string {
	environment := map[string]string{
		"CGO_ENABLED": "1",
		"GOARCH":      target.Arch(),
		"GOOS":        target.OS(),
	}
	if target.OS() == goosWindows {
		environment["CGO_ENABLED"] = "0"
	}
	if target == DefaultTarget() {
		environment["CGO_CFLAGS"] = "-mmacosx-version-min=" + minimumMacOS
		environment["CGO_LDFLAGS"] = macOSCGOLinkerFlags(minimumMacOS)
		environment["MACOSX_DEPLOYMENT_TARGET"] = minimumMacOS
	}
	return environment
}

func commandStep(name, program string, arguments ...string) Step {
	return Step{Name: name, Operation: runCommand, Program: program, Arguments: append([]string(nil), arguments...)}
}

func preflightStep(name string, preflight preflightKind, target Target) Step {
	return Step{Name: name, Operation: runPreflight, preflight: preflight, target: target}
}

// Run executes one action from the repository root.
func Run(ctx context.Context, root, action string, applicationArguments []string) error {
	if err := validateRoot(root); err != nil {
		return err
	}
	if action != "prepare" {
		if err := ValidateHost(DefaultTarget(), RuntimeHost()); err != nil {
			return err
		}
	}
	steps, err := Plan(action, applicationArguments)
	if err != nil {
		return err
	}
	for _, step := range steps {
		fmt.Printf("==> %s\n", step.Name)
		if err := execute(ctx, root, step); err != nil {
			return fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	return nil
}

// RunForTarget validates the repository, pure plan, and native host before it
// executes the first potentially mutating step.
func RunForTarget(ctx context.Context, root, action string, target Target, applicationArguments []string) error {
	return runForTargetOnHost(ctx, root, action, target, RuntimeHost(), applicationArguments)
}

// RunPortablePackageInContainer executes one portable package inside the
// architecture-matched Linux container created by PackageAllDocker. Windows
// uses Go's CGO-free cross-compilation; Linux remains a native CGO build.
func RunPortablePackageInContainer(ctx context.Context, root string, target Target) error {
	if !target.Portable() {
		return fmt.Errorf("container package requires a portable target, got %s", target)
	}
	if runtimeTarget := os.Getenv("FALLOUT_TERMINAL_CONTAINER_TARGET"); runtimeTarget != target.String() {
		return fmt.Errorf(
			"container package target marker mismatch: expected %q, got %q",
			target.String(),
			runtimeTarget,
		)
	}
	runtimeHost := RuntimeHost()
	if runtimeHost.OS() != goosLinux || runtimeHost.Arch() != target.Arch() {
		return fmt.Errorf(
			"container package for %s requires a linux/%s runtime, got %s",
			target,
			target.Arch(),
			runtimeHost,
		)
	}
	return runForTargetOnHost(ctx, root, "package", target, NewHost(target.OS(), target.Arch()), nil)
}

func runForTargetOnHost(
	ctx context.Context,
	root string,
	action string,
	target Target,
	host Host,
	applicationArguments []string,
) error {
	if err := validateRoot(root); err != nil {
		return err
	}
	if action != "prepare" {
		if err := ValidateHost(target, host); err != nil {
			return err
		}
	}
	steps, err := PlanForTarget(action, target, applicationArguments)
	if err != nil {
		return err
	}
	var packagePlan PackagePlan
	if action == "package" && target.Portable() {
		packagePlan, err = NewPackagePlan(target, host)
		if err != nil {
			return err
		}
		if err := packagePlan.CleanupFailure(root); err != nil {
			return fmt.Errorf("clean previous %s package attempt: %w", target, err)
		}
		steps = packagePlan.Actions()
	}
	for _, step := range steps {
		fmt.Printf("==> %s\n", step.Name)
		if err := execute(ctx, root, step); err != nil {
			if target.Portable() && action == "package" {
				return cleanupPortablePackageFailure(root, packagePlan, fmt.Errorf("%s: %w", step.Name, err))
			}
			return fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	if action == "package" && target.Portable() {
		if err := writePlannedPortableArchive(ctx, root, packagePlan); err != nil {
			return cleanupPortablePackageFailure(root, packagePlan, err)
		}
	}
	return nil
}

func writePlannedPortableArchive(ctx context.Context, root string, plan PackagePlan) error {
	sourceRevision, err := resolveSourceRevision(ctx, root)
	if err != nil {
		return err
	}
	files, err := plan.archiveFiles(root)
	if err != nil {
		return fmt.Errorf("resolve %s archive inventory: %w", plan.target, err)
	}
	archivePath, err := resolvePath(root, plan.outputPath)
	if err != nil {
		return fmt.Errorf("resolve %s archive output: %w", plan.target, err)
	}
	result, err := WritePortableArchive(
		ctx,
		filepath.Dir(archivePath),
		plan.target,
		plan.Version(),
		sourceRevision,
		files,
	)
	if err != nil {
		return fmt.Errorf("write %s portable archive: %w", plan.target, err)
	}
	if result.ArchivePath != archivePath || result.ChecksumPath != archivePath+".sha256" {
		return fmt.Errorf("write %s portable archive: output paths do not match the package plan", plan.target)
	}
	fmt.Printf(
		"==> packaged %s archive=%s revision=%s checksum=%s\n",
		plan.target,
		plan.outputPath,
		sourceRevision,
		plan.checksumPath,
	)
	return nil
}

func resolveSourceRevision(ctx context.Context, root string) (string, error) {
	if revision := os.Getenv("FALLOUT_TERMINAL_SOURCE_REVISION"); revision != "" &&
		os.Getenv("FALLOUT_TERMINAL_CONTAINER_TARGET") != "" {
		if err := validateSourceRevision(revision); err != nil {
			return "", fmt.Errorf("resolve package source revision from environment: %w", err)
		}
		return revision, nil
	}
	command := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD^{commit}")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolve package source revision: %w", err)
	}
	revision := strings.TrimSpace(string(output))
	if err := validateSourceRevision(revision); err != nil {
		return "", fmt.Errorf("resolve package source revision: %w", err)
	}
	return revision, nil
}

func cleanupPortablePackageFailure(root string, plan PackagePlan, cause error) error {
	if cleanupErr := plan.CleanupFailure(root); cleanupErr != nil {
		return fmt.Errorf("%w (cleanup failed: %v)", cause, cleanupErr)
	}
	return cause
}

func validateRoot(root string) error {
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return fmt.Errorf("run from the repository root: %w", err)
	}
	for line := range strings.SplitSeq(string(module), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "module" {
			continue
		}
		if len(fields) == 2 && fields[1] == applicationModule {
			return nil
		}
		break
	}
	return errors.New("run from the Fallout-Terminal repository root")
}

func execute(ctx context.Context, root string, step Step) error {
	switch step.Operation {
	case runCommand:
		cmd := exec.CommandContext(ctx, step.Program, step.Arguments...)
		cmd.Dir = root
		cmd.Env = mergeEnvironment(os.Environ(), step.Environment)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case removeTree:
		target, err := resolvePath(root, step.Path)
		if err != nil {
			return err
		}
		if !ownedRemovalPath(root, target) {
			return fmt.Errorf("refusing to remove unexpected path %q", target)
		}
		return os.RemoveAll(target)
	case makeDirectory:
		target, err := resolvePath(root, step.Path)
		if err != nil {
			return err
		}
		return os.MkdirAll(target, step.Mode)
	case copyFile:
		return copyRegularFile(root, step.Source, step.Destination, step.Mode)
	case changeMode:
		target, err := resolvePath(root, step.Path)
		if err != nil {
			return err
		}
		return os.Chmod(target, step.Mode)
	case runPreflight:
		return executePreflight(ctx, root, step.preflight, step.target)
	case renderTemplate:
		return renderVersionTemplate(ctx, root, step)
	default:
		return fmt.Errorf("unsupported build operation %d", step.Operation)
	}
}

func resolvePath(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("build path must be repository-relative: %q", path)
	}
	cleanRoot := filepath.Clean(root)
	resolved := filepath.Join(cleanRoot, filepath.Clean(path))
	if resolved != cleanRoot && !strings.HasPrefix(resolved, cleanRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("build path escapes repository root: %q", path)
	}
	return resolved, nil
}

func copyRegularFile(root, source, destination string, mode os.FileMode) error {
	sourcePath, err := resolvePath(root, source)
	if err != nil {
		return err
	}
	destinationPath, err := resolvePath(root, destination)
	if err != nil {
		return err
	}
	input, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", source)
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(destinationPath, mode)
}

func mergeEnvironment(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	values := make(map[string]string, len(base)+len(overrides))
	for _, item := range base {
		key, value, found := strings.Cut(item, "=")
		if found {
			values[key] = value
		}
	}
	maps.Copy(values, overrides)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}
