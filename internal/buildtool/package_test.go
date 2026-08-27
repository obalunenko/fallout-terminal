package buildtool

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortablePackagePlanSelectsReleaseAndLocalVersionRepresentations(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		canonical       string
		numericCore     string
		numericFourPart string
		release         bool
	}{
		{
			name:            "stable release",
			input:           "2.4.6",
			canonical:       "2.4.6",
			numericCore:     "2.4.6",
			numericFourPart: "2.4.6.0",
			release:         true,
		},
		{
			name:            "prerelease",
			input:           "2.4.6-rc.2",
			canonical:       "2.4.6-rc.2",
			numericCore:     "2.4.6",
			numericFourPart: "2.4.6.0",
			release:         true,
		},
		{
			name:            "local development",
			canonical:       "development",
			numericCore:     "0.0.0",
			numericFourPart: "0.0.0.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("VERSION", test.input)

			target := mustParseTarget(t, goosWindows, goarchAMD64)
			plan := mustPackagePlan(t, target)
			actions := plan.Actions()
			compile, _ := packageCompileStep(t, actions)
			compileInvocation := strings.Join(compile.Arguments, " ")
			allActions := packageActionText(actions)

			assert.NotContains(t, compileInvocation, "-buildvcs=false", "package builds must retain Go VCS metadata")
			if test.release {
				assert.Contains(
					t,
					compileInvocation,
					"-X "+applicationModule+"/internal/version.value="+test.canonical,
					"release packages must link the canonical application version",
				)
			} else {
				assert.NotContains(
					t,
					compileInvocation,
					applicationModule+"/internal/version.value=",
					"local packages must use the version owner's development default",
				)
			}

			for _, representation := range []string{test.canonical, test.numericCore, test.numericFourPart} {
				assert.Containsf(t, allActions, representation, "package actions do not carry representation %q", representation)
			}
		})
	}
}

func TestPortablePackagePlanRejectsMalformedExplicitReleaseVersion(t *testing.T) {
	t.Setenv("VERSION", "v2.4.6")

	target := mustParseTarget(t, goosLinux, goarchAMD64)
	_, err := NewPackagePlan(target, NewHost(target.OS(), target.Arch()))
	require.ErrorContains(t, err, "VERSION")
}

func TestRenderVersionTemplateIsDeterministicAndPreservesInput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join("build", "windows", "info.json.tmpl")
	destination := filepath.Join("build", "bin", "windows-amd64", "metadata", "info.json")
	template := []byte("human={{VERSION}}\nnumeric={{NUMERIC_CORE}}\nfour={{NUMERIC_FOUR_PART}}\n")
	writePackageFixtureContents(t, root, source, template)
	version, err := ResolveBuildVersion("2.4.6-rc.2")
	require.NoError(t, err)
	step := versionTemplateStep("render fixture", source, destination, version)

	require.NoError(t, execute(t.Context(), root, step))
	first, err := os.ReadFile(filepath.Join(root, destination))
	require.NoError(t, err)
	require.Equal(t, "human=2.4.6-rc.2\nnumeric=2.4.6\nfour=2.4.6.0\n", string(first))

	require.NoError(t, execute(t.Context(), root, step))
	second, err := os.ReadFile(filepath.Join(root, destination))
	require.NoError(t, err)
	assert.Equal(t, first, second)

	preserved, err := os.ReadFile(filepath.Join(root, source))
	require.NoError(t, err)
	assert.Equal(t, template, preserved)
}

func TestPortablePackagePlanUsesTargetNativeCompilationAndIsolatedStaging(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		goos           string
		goarch         string
		executableName string
		cgoEnabled     string
		windowsGUI     bool
	}{
		{name: "Windows ARM64", goos: "windows", goarch: "arm64", executableName: applicationName + ".exe", cgoEnabled: "0", windowsGUI: true},
		{name: "Windows AMD64", goos: "windows", goarch: "amd64", executableName: applicationName + ".exe", cgoEnabled: "0", windowsGUI: true},
		{name: "Linux ARM64", goos: "linux", goarch: "arm64", executableName: applicationName, cgoEnabled: "1"},
		{name: "Linux AMD64", goos: "linux", goarch: "amd64", executableName: applicationName, cgoEnabled: "1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target := mustParseTarget(t, test.goos, test.goarch)
			plan := mustPackagePlan(t, target)

			wantTargetRoot := filepath.Join("build", "bin", test.goos+"-"+test.goarch)
			assert.Equal(t, filepath.Join(wantTargetRoot, "stage"), plan.StageRoot())
			assert.Equal(t, filepath.Join(wantTargetRoot, "stage", applicationName), plan.PayloadRoot())
			assert.Equal(t, filepath.Join("build", "dist", target.ArchiveName()), plan.OutputPath())
			assert.Equal(t, plan.OutputPath()+".sha256", plan.ChecksumPath())
			assert.Equal(t, productionProfile, plan.ProductionProfile())

			compile, _ := packageCompileStep(t, plan.Actions())
			assert.Equal(t, test.goos, compile.Environment["GOOS"])
			assert.Equal(t, test.goarch, compile.Environment["GOARCH"])
			assert.Equal(t, test.cgoEnabled, compile.Environment["CGO_ENABLED"])
			assert.Contains(t, compile.Arguments, filepath.Join(plan.PayloadRoot(), test.executableName))

			invocation := strings.Join(compile.Arguments, " ")
			if test.windowsGUI {
				assert.Contains(t, invocation, "-H windowsgui")
			} else {
				assert.NotContains(t, invocation, "windowsgui")
			}

			if test.goos == goosLinux {
				executable := filepath.Join(plan.PayloadRoot(), test.executableName)
				modeStep, found := findPackageStep(plan.Actions(), func(step Step) bool {
					return step.Operation == changeMode && step.Path == executable
				})
				require.True(t, found, "Linux package plan must make the native executable runnable")
				assert.Equal(t, os.FileMode(0o755), modeStep.Mode)
			}
		})
	}
}

func TestPortablePackagePlanIncludesExactRuntimeResourceInventory(t *testing.T) {
	t.Parallel()

	for _, target := range portableTestTargets(t) {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()

			plan := mustPackagePlan(t, target)
			actions := plan.Actions()
			_, compileIndex := packageCompileStep(t, actions)

			type resource struct {
				source string
				mode   os.FileMode
				index  int
			}
			got := make(map[string]resource)
			for index, action := range actions {
				if action.Operation != copyFile {
					continue
				}
				relative, err := filepath.Rel(plan.PayloadRoot(), action.Destination)
				if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					continue
				}
				got[relative] = resource{source: action.Source, mode: action.Mode, index: index}
			}

			want := map[string]string{
				filepath.Join("resources", "appicon.png"):                   filepath.Join("build", "appicon.png"),
				filepath.Join("resources", "THIRD_PARTY_NOTICES.md"):        "THIRD_PARTY_NOTICES.md",
				filepath.Join("resources", "sessions", "demo.json"):         filepath.Join("sessions", "demo.json"),
				filepath.Join("resources", "sessions", "demo-players.json"): filepath.Join("sessions", "demo-players.json"),
			}
			require.Len(t, got, len(want), "portable payload must contain exactly the reviewed runtime resources")
			for destination, source := range want {
				copied, exists := got[destination]
				require.Truef(t, exists, "missing portable resource %s", destination)
				assert.Equal(t, source, copied.source)
				assert.Equal(t, os.FileMode(0o444), copied.mode)
				assert.Lessf(t, copied.index, compileIndex, "%s must be staged before compilation", destination)
			}
		})
	}
}

func TestWindowsPackagePlanGeneratesPinnedBuildScopedMetadata(t *testing.T) {
	t.Parallel()

	for _, goarch := range []string{goarchARM64, goarchAMD64} {
		t.Run(goarch, func(t *testing.T) {
			t.Parallel()

			target := mustParseTarget(t, goosWindows, goarch)
			plan := mustPackagePlan(t, target)
			actions := plan.Actions()
			_, compileIndex := packageCompileStep(t, actions)

			iconStep, iconIndex := requirePinnedWailsGenerateStep(t, actions, "icons")
			assert.Contains(t, iconStep.Arguments, filepath.Join("build", "appicon.png"))

			sysoStep, sysoIndex := requirePinnedWailsGenerateStep(t, actions, "syso")
			assert.Less(t, iconIndex, sysoIndex)
			assert.Less(t, sysoIndex, compileIndex)
			assert.Equal(t, goarch, requiredFlagValue(t, sysoStep.Arguments, "-arch"))
			metadataRoot := filepath.Join("build", "bin", "windows-"+goarch, "metadata")
			renderedManifest := filepath.Join(metadataRoot, "app.manifest")
			renderedInfo := filepath.Join(metadataRoot, "info.json")
			assert.Equal(t, renderedManifest, requiredFlagValue(t, sysoStep.Arguments, "-manifest"))
			assert.Equal(t, renderedInfo, requiredFlagValue(t, sysoStep.Arguments, "-info"))

			manifestRender, manifestRenderIndex := requirePackageStep(t, actions, func(step Step) bool {
				return step.Source == filepath.Join("build", "windows", "app.manifest.tmpl") &&
					step.Destination == renderedManifest
			}, "render isolated Windows application manifest")
			infoRender, infoRenderIndex := requirePackageStep(t, actions, func(step Step) bool {
				return step.Source == filepath.Join("build", "windows", "info.json.tmpl") &&
					step.Destination == renderedInfo
			}, "render isolated Windows version information")
			assert.Less(t, manifestRenderIndex, sysoIndex)
			assert.Less(t, infoRenderIndex, sysoIndex)
			assert.NotEqual(t, manifestRender.Source, manifestRender.Destination)
			assert.NotEqual(t, infoRender.Source, infoRender.Destination)
			assert.Contains(t, plan.FailureCleanupPaths(), metadataRoot)
			assertPackageTemplatesAreReadOnlyInputs(t, actions)

			iconPath := requiredFlagValue(t, sysoStep.Arguments, "-icon")
			assert.Contains(t, iconStep.Arguments, iconPath)
			assert.Equal(t, ".ico", filepath.Ext(iconPath))

			sysoPath := requiredFlagValue(t, sysoStep.Arguments, "-out")
			assert.Equal(t, ".syso", filepath.Ext(sysoPath))
			assert.Contains(t, filepath.Base(sysoPath), goarch)
			assert.Contains(t, plan.FailureCleanupPaths(), sysoPath)

			_, cleanupIndex := requirePackageStep(t, actions, func(step Step) bool {
				return step.Operation == removeTree && step.Path == sysoPath
			}, "remove generated Windows metadata")
			assert.Greater(t, cleanupIndex, compileIndex, "generated target metadata must not survive a successful build")
		})
	}
}

func TestPortablePackagePlanOutputsAreStableAndCollisionFree(t *testing.T) {
	t.Parallel()

	stageRoots := make(map[string]string)
	outputs := make(map[string]string)
	checksums := make(map[string]string)
	for _, target := range portableTestTargets(t) {
		plan := mustPackagePlan(t, target)
		repeated := mustPackagePlan(t, target)

		assert.Equal(t, plan.StageRoot(), repeated.StageRoot())
		assert.Equal(t, plan.OutputPath(), repeated.OutputPath())
		assert.Equal(t, plan.ChecksumPath(), repeated.ChecksumPath())

		assert.NotContains(t, stageRoots, plan.StageRoot())
		assert.NotContains(t, outputs, plan.OutputPath())
		assert.NotContains(t, checksums, plan.ChecksumPath())
		stageRoots[plan.StageRoot()] = target.String()
		outputs[plan.OutputPath()] = target.String()
		checksums[plan.ChecksumPath()] = target.String()
	}

	assert.Len(t, stageRoots, 4)
	assert.Len(t, outputs, 4)
	assert.Len(t, checksums, 4)
}

func TestDarwinPortablePackagePlanStagesCompleteUnsignedApplicationBundle(t *testing.T) {
	t.Parallel()

	target := mustParseTarget(t, goosDarwin, goarchARM64)
	plan := mustPackagePlan(t, target)
	root := t.TempDir()
	t.Cleanup(func() { require.NoError(t, plan.CleanupFailure(root)) })

	assert.Equal(t, filepath.Join("build", "bin", "darwin-arm64", "stage"), plan.StageRoot())
	assert.Equal(t, filepath.Join(plan.StageRoot(), applicationName), plan.PayloadRoot())
	assert.Equal(t, filepath.Join("build", "dist", "Fallout-Terminal-darwin-arm64.zip"), plan.OutputPath())

	actions := plan.Actions()
	compile, compileIndex := packageCompileStep(t, actions)
	assert.Equal(t, goosDarwin, compile.Environment["GOOS"])
	assert.Equal(t, goarchARM64, compile.Environment["GOARCH"])
	assert.Equal(t, "1", compile.Environment["CGO_ENABLED"])
	assert.Contains(t, compile.Arguments, filepath.Join(plan.PayloadRoot(), "Fallout Terminal.app", "Contents", "MacOS", applicationName))

	renderedInfoPlist := filepath.Join(plan.PayloadRoot(), "Fallout Terminal.app", "Contents", "Info.plist")
	metadata, metadataIndex := requirePackageStep(t, actions, func(step Step) bool {
		return step.Source == filepath.Join("build", "darwin", "Info.plist.tmpl") &&
			step.Destination == renderedInfoPlist
	}, "render isolated Darwin application metadata")
	assert.Less(t, metadataIndex, compileIndex)
	assert.NotEqual(t, metadata.Source, metadata.Destination)
	assertPackageTemplatesAreReadOnlyInputs(t, actions)

	wantCopies := map[string]string{
		filepath.Join(plan.PayloadRoot(), "Fallout Terminal.app", "Contents", "Resources", "THIRD_PARTY_NOTICES.md"):        "THIRD_PARTY_NOTICES.md",
		filepath.Join(plan.PayloadRoot(), "Fallout Terminal.app", "Contents", "Resources", "sessions", "demo.json"):         filepath.Join("sessions", "demo.json"),
		filepath.Join(plan.PayloadRoot(), "Fallout Terminal.app", "Contents", "Resources", "sessions", "demo-players.json"): filepath.Join("sessions", "demo-players.json"),
	}
	for destination, source := range wantCopies {
		step, index := requirePackageStep(t, actions, func(step Step) bool {
			return step.Operation == copyFile && step.Destination == destination
		}, "stage Darwin bundle resource")
		assert.Equal(t, source, step.Source)
		assert.Less(t, index, compileIndex)
	}
	icon, iconIndex := requirePinnedWailsGenerateStep(t, actions, "icons")
	assert.Equal(t,
		filepath.Join(plan.PayloadRoot(), "Fallout Terminal.app", "Contents", "Resources", "icon.icns"),
		requiredFlagValue(t, icon.Arguments, "-macfilename"),
	)
	assert.Less(t, iconIndex, compileIndex)

	joined := strings.ToLower(packageActionText(actions))
	for _, forbidden := range []string{
		"codesign", "notar", "staple", ".dmg", "credentials", "settings.json", "sessions/active", "plaintext", "secret",
	} {
		assert.NotContains(t, joined, forbidden)
	}
}

func TestDarwinPortableAndImplicitDeveloperPackagePlansRemainDistinct(t *testing.T) {
	t.Parallel()

	explicit, err := PlanForTarget("package", mustParseTarget(t, goosDarwin, goarchARM64), nil)
	require.NoError(t, err)
	implicit, err := Plan("package", nil)
	require.NoError(t, err)

	assert.NotEqual(t, packageActionText(implicit), packageActionText(explicit))
	assert.NotContains(t, strings.ToLower(packageActionText(explicit)), "codesign")
	assert.Contains(t, strings.ToLower(packageActionText(implicit)), "codesign")
}

func TestNativePrerequisitesDoNotApplyLinuxDesktopChecksToWindowsOrDarwin(t *testing.T) {
	t.Setenv("PATH", "")

	for _, target := range []Target{
		mustParseTarget(t, goosWindows, goarchAMD64),
		mustParseTarget(t, goosDarwin, goarchARM64),
	} {
		require.NoError(t, verifyNativePrerequisites(t.Context(), t.TempDir(), target))
	}
}

func TestPortablePackagePlanAccessorsDoNotExposeMutableState(t *testing.T) {
	t.Parallel()

	plan := mustPackagePlan(t, mustParseTarget(t, goosWindows, goarchAMD64))

	environment := plan.Environment()
	environment["GOOS"] = "mutated"
	assert.Equal(t, goosWindows, plan.Environment()["GOOS"])

	actions := plan.Actions()
	require.NotEmpty(t, actions)
	actions[0].Name = "mutated"
	actions[0].Arguments = append(actions[0].Arguments, "--mutated")
	actions[0].Environment = map[string]string{"MUTATED": "1"}
	assert.NotEqual(t, "mutated", plan.Actions()[0].Name)
	assert.NotContains(t, plan.Actions()[0].Arguments, "--mutated")
	assert.NotContains(t, plan.Actions()[0].Environment, "MUTATED")

	temporaryPaths := plan.TemporaryPaths()
	require.NotEmpty(t, temporaryPaths)
	temporaryPaths[0] = "mutated"
	assert.NotEqual(t, "mutated", plan.TemporaryPaths()[0])

	cleanupPaths := plan.FailureCleanupPaths()
	require.NotEmpty(t, cleanupPaths)
	cleanupPaths[0] = "mutated"
	assert.NotEqual(t, "mutated", plan.FailureCleanupPaths()[0])
}

func TestPortablePackageFailureCleanupRemovesOnlyOwnedTargetOutputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := mustParseTarget(t, goosWindows, goarchAMD64)
	plan := mustPackagePlan(t, target)

	for _, relative := range plan.FailureCleanupPaths() {
		if relative == plan.StageRoot() {
			writePackageFixture(t, root, filepath.Join(relative, "owned.txt"))
			continue
		}
		writePackageFixture(t, root, relative)
	}

	otherTarget := mustPackagePlan(t, mustParseTarget(t, goosLinux, goarchAMD64))
	otherSentinel := filepath.Join(otherTarget.StageRoot(), "keep.txt")
	writePackageFixture(t, root, otherSentinel)
	sharedSentinel := filepath.Join("build", "dist", "keep.txt")
	writePackageFixture(t, root, sharedSentinel)

	require.NoError(t, plan.CleanupFailure(root))

	for _, relative := range plan.FailureCleanupPaths() {
		_, err := os.Stat(filepath.Join(root, relative))
		assert.ErrorIsf(t, err, os.ErrNotExist, "owned package path still exists: %s", relative)
	}
	assert.FileExists(t, filepath.Join(root, otherSentinel))
	assert.FileExists(t, filepath.Join(root, sharedSentinel))
	assert.NotContains(t, plan.FailureCleanupPaths(), filepath.Join("build", "bin"))
	assert.NotContains(t, plan.FailureCleanupPaths(), filepath.Join("build", "dist"))
}

func portableTestTargets(t *testing.T) []Target {
	t.Helper()

	return []Target{
		mustParseTarget(t, goosWindows, goarchARM64),
		mustParseTarget(t, goosWindows, goarchAMD64),
		mustParseTarget(t, goosLinux, goarchARM64),
		mustParseTarget(t, goosLinux, goarchAMD64),
	}
}

func mustPackagePlan(t *testing.T, target Target) PackagePlan {
	t.Helper()

	plan, err := NewPackagePlan(target, NewHost(target.OS(), target.Arch()))
	require.NoError(t, err)
	return plan
}

func packageCompileStep(t *testing.T, actions []Step) (Step, int) {
	t.Helper()

	return requirePackageStep(t, actions, func(step Step) bool {
		return step.Program == "go" && len(step.Arguments) > 0 && step.Arguments[0] == "build"
	}, "compile portable application")
}

func findPackageStep(actions []Step, matches func(Step) bool) (Step, bool) {
	for _, action := range actions {
		if matches(action) {
			return action, true
		}
	}
	return Step{}, false
}

func requirePackageStep(t *testing.T, actions []Step, matches func(Step) bool, description string) (Step, int) {
	t.Helper()

	for index, action := range actions {
		if matches(action) {
			return action, index
		}
	}
	require.FailNow(t, "package plan step is missing", description)
	return Step{}, -1
}

func requirePinnedWailsGenerateStep(t *testing.T, actions []Step, generator string) (Step, int) {
	t.Helper()

	prefix := []string{"tool", "-modfile=tools/wails/go.mod", "wails3", "generate", generator}
	return requirePackageStep(t, actions, func(step Step) bool {
		return step.Program == "go" && len(step.Arguments) >= len(prefix) && slices.Equal(step.Arguments[:len(prefix)], prefix)
	}, "pinned Wails "+generator+" generation")
}

func requiredFlagValue(t *testing.T, arguments []string, flag string) string {
	t.Helper()

	for index, argument := range arguments {
		if argument == flag {
			require.Less(t, index+1, len(arguments), "%s has no value", flag)
			return arguments[index+1]
		}
	}
	require.FailNow(t, "required flag is missing", flag)
	return ""
}

func writePackageFixture(t *testing.T, root, relative string) {
	t.Helper()

	writePackageFixtureContents(t, root, relative, []byte("fixture"))
}

func writePackageFixtureContents(t *testing.T, root, relative string, contents []byte) {
	t.Helper()

	path := filepath.Join(root, relative)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, contents, 0o600))
}

func packageActionText(actions []Step) string {
	var parts []string
	for _, action := range actions {
		parts = append(parts, action.Name, action.Program, strings.Join(action.Arguments, " "), action.Source, action.Destination, action.Path)
		environmentKeys := make([]string, 0, len(action.Environment))
		for key := range action.Environment {
			environmentKeys = append(environmentKeys, key)
		}
		sort.Strings(environmentKeys)
		for _, key := range environmentKeys {
			parts = append(parts, key+"="+action.Environment[key])
		}
	}
	return strings.Join(parts, "\n")
}

func assertPackageTemplatesAreReadOnlyInputs(t *testing.T, actions []Step) {
	t.Helper()

	templates := []string{
		filepath.Join("build", "darwin", "Info.plist.tmpl"),
		filepath.Join("build", "windows", "info.json.tmpl"),
		filepath.Join("build", "windows", "app.manifest.tmpl"),
	}
	for _, action := range actions {
		for _, template := range templates {
			assert.NotEqualf(t, template, action.Destination, "package action %q overwrites a checked-in template", action.Name)
			assert.NotEqualf(t, template, action.Path, "package action %q mutates a checked-in template", action.Name)
		}
	}
}
