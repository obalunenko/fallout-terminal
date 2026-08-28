package buildtool

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
)

const productionProfile = "production"

// PackagePlan is an immutable description of one native portable package
// attempt. Slice and map accessors return copies so callers cannot mutate the
// plan after validation.
type PackagePlan struct {
	target             Target
	host               Host
	stageRoot          string
	payloadRoot        string
	outputPath         string
	checksumPath       string
	version            ReleaseVersion
	temporaryPaths     []string
	actions            []Step
	failureCleanupPath []string
}

// NewPackagePlan validates a portable target against its native build host and
// constructs the target-isolated paths used by later resource, archive, and
// verification stages.
func NewPackagePlan(target Target, host Host) (PackagePlan, error) {
	version, err := ResolveBuildVersion(os.Getenv(packageVersionEnvironmentCanonical))
	if err != nil {
		return PackagePlan{}, fmt.Errorf("resolve package VERSION: %w", err)
	}
	return newPackagePlan(target, host, version)
}

func newPackagePlan(target Target, host Host, version ReleaseVersion) (PackagePlan, error) {
	if !target.Portable() {
		return PackagePlan{}, fmt.Errorf("portable package plan requires a supported release target, got %s", target)
	}
	if err := ValidateHost(target, host); err != nil {
		return PackagePlan{}, err
	}
	if err := validatePackageVersion(version); err != nil {
		return PackagePlan{}, err
	}

	targetRoot := filepath.Join("build", "bin", target.OS()+"-"+target.Arch())
	stageRoot := filepath.Join(targetRoot, "stage")
	payloadRoot := filepath.Join(stageRoot, applicationName)
	outputPath := filepath.Join("build", "dist", target.ArchiveName())
	checksumPath := outputPath + ".sha256"
	temporaryPaths := []string{outputPath + ".partial", checksumPath + ".partial"}
	if target.OS() == goosWindows {
		temporaryPaths = append(
			temporaryPaths,
			windowsIconPath(target),
			windowsSysoPath(target),
			windowsMetadataRoot(target),
		)
	}

	plan := PackagePlan{
		target:         target,
		host:           host,
		stageRoot:      stageRoot,
		payloadRoot:    payloadRoot,
		outputPath:     outputPath,
		checksumPath:   checksumPath,
		version:        version,
		temporaryPaths: append([]string(nil), temporaryPaths...),
		failureCleanupPath: append(
			[]string{stageRoot, outputPath, checksumPath},
			temporaryPaths...,
		),
	}
	plan.actions = portablePackageActions(plan)
	return plan, nil
}

// Target returns the canonical package destination.
func (p PackagePlan) Target() Target {
	return p.target
}

// Host returns the validated native builder.
func (p PackagePlan) Host() Host {
	return p.host
}

// StageRoot returns the isolated, repository-relative working tree.
func (p PackagePlan) StageRoot() string {
	return p.stageRoot
}

// PayloadRoot returns the single archive root directory inside StageRoot.
func (p PackagePlan) PayloadRoot() string {
	return p.payloadRoot
}

// OutputPath returns the stable repository-relative archive destination.
func (p PackagePlan) OutputPath() string {
	return p.outputPath
}

// ChecksumPath returns the archive's stable SHA-256 sidecar destination.
func (p PackagePlan) ChecksumPath() string {
	return p.checksumPath
}

// Version returns the validated canonical package identity shared by the
// executable, native metadata, and artifact manifest.
func (p PackagePlan) Version() ReleaseVersion {
	return p.version
}

// TemporaryPaths returns the owned unpublished outputs for this attempt.
func (p PackagePlan) TemporaryPaths() []string {
	return append([]string(nil), p.temporaryPaths...)
}

// ProductionProfile returns the immutable package build profile.
func (p PackagePlan) ProductionProfile() string {
	return productionProfile
}

// Environment returns a fresh copy of the explicit target compile environment.
func (p PackagePlan) Environment() map[string]string {
	return compileEnvironment(p.target)
}

// Actions returns a deep copy of the ordered package actions.
func (p PackagePlan) Actions() []Step {
	actions := make([]Step, len(p.actions))
	for index, action := range p.actions {
		actions[index] = cloneStep(action)
	}
	return actions
}

// FailureCleanupPaths returns the exact allowlist that may be removed after a
// failed attempt. It includes the isolated stage and this target's stable and
// unpublished outputs, never a shared directory.
func (p PackagePlan) FailureCleanupPaths() []string {
	return append([]string(nil), p.failureCleanupPath...)
}

// CleanupFailure removes only paths owned by this package attempt. Stable
// outputs are included so a failed rebuild can never leave a previous archive
// looking like the result of the current command.
func (p PackagePlan) CleanupFailure(root string) error {
	var cleanupErrors []error
	for _, path := range p.failureCleanupPath {
		resolved, err := resolvePackageCleanupPath(root, p, path)
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
			continue
		}
		if err := os.RemoveAll(resolved); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove package path %q: %w", path, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func (p PackagePlan) archiveFiles(root string) ([]ArchiveFile, error) {
	payloadRoot, err := resolvePath(root, p.payloadRoot)
	if err != nil {
		return nil, err
	}
	paths := requiredArchivePaths(p.target)
	files := make([]ArchiveFile, 0, len(paths))
	for _, archivePath := range paths {
		files = append(files, ArchiveFile{
			Path:       archivePath,
			SourcePath: filepath.Join(payloadRoot, filepath.FromSlash(archivePath)),
		})
	}
	return files, nil
}

func portablePackageActions(plan PackagePlan) []Step {
	if plan.target.OS() == goosDarwin {
		return darwinPortablePackageActions(plan)
	}
	resources := filepath.Join(plan.payloadRoot, "resources")
	actions := append([]Step(nil), preparePlan()...)
	actions = append(actions,
		preflightStep("verify "+plan.target.String()+" native build prerequisites", verifyNativeBuildPrerequisites, plan.target),
		Step{Name: "remove previous " + plan.target.String() + " staging directory", Operation: removeTree, Path: plan.stageRoot},
		Step{Name: "create portable application root", Operation: makeDirectory, Path: plan.payloadRoot, Mode: 0o755},
		Step{Name: "install portable launch guide", Operation: copyFile, Source: portableLaunchGuideFilename, Destination: filepath.Join(plan.payloadRoot, portableLaunchGuideFilename), Mode: 0o444},
		Step{Name: "create portable resource directory", Operation: makeDirectory, Path: filepath.Join(resources, "sessions"), Mode: 0o755},
		Step{Name: "install portable application icon", Operation: copyFile, Source: filepath.Join("build", "appicon.png"), Destination: filepath.Join(resources, "appicon.png"), Mode: 0o444},
		Step{Name: "install portable third-party notices", Operation: copyFile, Source: "THIRD_PARTY_NOTICES.md", Destination: filepath.Join(resources, "THIRD_PARTY_NOTICES.md"), Mode: 0o444},
		Step{Name: "install portable bundled demo player config", Operation: copyFile, Source: filepath.Join("sessions", "demo-players.json"), Destination: filepath.Join(resources, "sessions", "demo-players.json"), Mode: 0o444},
		Step{Name: "install portable bundled demo", Operation: copyFile, Source: filepath.Join("sessions", "demo.json"), Destination: filepath.Join(resources, "sessions", "demo.json"), Mode: 0o444},
	)
	if plan.target.OS() == goosWindows {
		icon := windowsIconPath(plan.target)
		syso := windowsSysoPath(plan.target)
		manifest := windowsManifestPath(plan.target)
		info := windowsInfoPath(plan.target)
		actions = append(actions,
			versionTemplateStep(
				"render "+plan.target.String()+" application manifest",
				filepath.Join("build", "windows", "app.manifest.tmpl"),
				manifest,
				plan.version,
			),
			versionTemplateStep(
				"render "+plan.target.String()+" version information",
				filepath.Join("build", "windows", "info.json.tmpl"),
				info,
				plan.version,
			),
			commandStep(
				"generate "+plan.target.String()+" application icon",
				"go", "tool", "-modfile=tools/wails/go.mod", "wails3", "generate", "icons",
				"-input", filepath.Join("build", "appicon.png"),
				"-windowsfilename", icon,
				"-macfilename", "",
			),
			commandStep(
				"generate "+plan.target.String()+" application metadata",
				"go", "tool", "-modfile=tools/wails/go.mod", "wails3", "generate", "syso",
				"-arch", plan.target.Arch(),
				"-icon", icon,
				"-manifest", manifest,
				"-info", info,
				"-out", syso,
			),
		)
	}
	executable := filepath.Join(plan.payloadRoot, plan.target.ExecutableName())
	actions = append(actions, versionedCompileStep(plan.target, executable, plan.version))
	if plan.target.OS() == goosLinux {
		actions = append(actions, Step{Name: "make Linux application executable", Operation: changeMode, Path: executable, Mode: 0o755})
	}
	if plan.target.OS() == goosWindows {
		actions = append(actions,
			Step{Name: "remove generated Windows metadata", Operation: removeTree, Path: windowsSysoPath(plan.target)},
			Step{Name: "remove generated Windows icon", Operation: removeTree, Path: windowsIconPath(plan.target)},
		)
	}
	return actions
}

func darwinPortablePackageActions(plan PackagePlan) []Step {
	bundle := filepath.Join(plan.payloadRoot, applicationName+".app")
	contents := filepath.Join(bundle, "Contents")
	macOS := filepath.Join(contents, "MacOS")
	resources := filepath.Join(contents, "Resources")
	executable := filepath.Join(macOS, applicationName)

	actions := append([]Step(nil), preparePlan()...)
	actions = append(actions,
		preflightStep("verify "+plan.target.String()+" native build prerequisites", verifyNativeBuildPrerequisites, plan.target),
		Step{Name: "remove previous " + plan.target.String() + " staging directory", Operation: removeTree, Path: plan.stageRoot},
		Step{Name: "create portable application root", Operation: makeDirectory, Path: plan.payloadRoot, Mode: 0o755},
		Step{Name: "install portable launch guide", Operation: copyFile, Source: portableLaunchGuideFilename, Destination: filepath.Join(plan.payloadRoot, portableLaunchGuideFilename), Mode: 0o444},
		Step{Name: "create Darwin application executable directory", Operation: makeDirectory, Path: macOS, Mode: 0o755},
		Step{Name: "create Darwin bundled session directory", Operation: makeDirectory, Path: filepath.Join(resources, "sessions"), Mode: 0o755},
		versionTemplateStep(
			"render Darwin application metadata",
			filepath.Join("build", "darwin", "Info.plist.tmpl"),
			filepath.Join(contents, "Info.plist"),
			plan.version,
		),
		commandStep(
			"install Darwin application icon",
			"go", "tool", "-modfile=tools/wails/go.mod", "wails3", "generate", "icons",
			"-input", filepath.Join("build", "appicon.png"),
			"-macfilename", filepath.Join(resources, "icon.icns"),
			"-windowsfilename", "",
		),
		Step{Name: "install Darwin third-party notices", Operation: copyFile, Source: "THIRD_PARTY_NOTICES.md", Destination: filepath.Join(resources, "THIRD_PARTY_NOTICES.md"), Mode: 0o444},
		Step{Name: "install Darwin bundled demo player config", Operation: copyFile, Source: filepath.Join("sessions", "demo-players.json"), Destination: filepath.Join(resources, "sessions", "demo-players.json"), Mode: 0o444},
		Step{Name: "install Darwin bundled demo", Operation: copyFile, Source: filepath.Join("sessions", "demo.json"), Destination: filepath.Join(resources, "sessions", "demo.json"), Mode: 0o444},
		versionedCompileStep(plan.target, executable, plan.version),
		Step{Name: "make Darwin application executable", Operation: changeMode, Path: executable, Mode: 0o755},
	)
	return actions
}

func windowsIconPath(target Target) string {
	return filepath.Join("build", "bin", target.OS()+"-"+target.Arch(), "appicon-"+target.Arch()+".ico")
}

func windowsSysoPath(target Target) string {
	return "rsrc_windows_" + target.Arch() + ".syso"
}

func cloneStep(step Step) Step {
	step.Arguments = append([]string(nil), step.Arguments...)
	if step.Environment != nil {
		environment := make(map[string]string, len(step.Environment))
		maps.Copy(environment, step.Environment)
		step.Environment = environment
	}
	return step
}

func resolvePackageCleanupPath(root string, plan PackagePlan, path string) (string, error) {
	owned := slices.Contains(plan.failureCleanupPath, path)
	if !owned {
		return "", fmt.Errorf("refusing to remove package path outside the %s cleanup allowlist: %q", plan.target, path)
	}
	return resolvePath(root, path)
}

func ownedRemovalPath(root, resolved string) bool {
	cleanRoot := filepath.Clean(root)
	if slices.Contains([]string{
		filepath.Join(cleanRoot, "build", "bin", applicationName+".app"),
		filepath.Join(cleanRoot, developmentBundle()),
	}, resolved) {
		return true
	}
	for _, target := range PortableTargets() {
		stage := filepath.Join(cleanRoot, "build", "bin", target.OS()+"-"+target.Arch(), "stage")
		metadata := filepath.Join(cleanRoot, windowsMetadataRoot(target))
		if resolved == stage || (target.OS() == goosWindows && resolved == metadata) {
			return true
		}
	}
	for _, goarch := range []string{goarchARM64, goarchAMD64} {
		if resolved == filepath.Join(cleanRoot, windowsSysoPath(Target{goos: goosWindows, goarch: goarch})) ||
			resolved == filepath.Join(cleanRoot, windowsIconPath(Target{goos: goosWindows, goarch: goarch})) {
			return true
		}
	}
	return false
}
