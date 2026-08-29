package buildtool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRootRequiresExactV2ApplicationModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		module  string
		wantErr bool
	}{
		{
			name:   "exact v2 module",
			module: "module github.com/obalunenko/Fallout-Terminal/v2\n\ngo 1.27.0\n",
		},
		{
			name:    "unsuffixed module",
			module:  "module github.com/obalunenko/Fallout-Terminal\n\ngo 1.27.0\n",
			wantErr: true,
		},
		{
			name:    "substring major",
			module:  "module github.com/obalunenko/Fallout-Terminal/v20\n\ngo 1.27.0\n",
			wantErr: true,
		},
		{
			name:    "identity only in comment",
			module:  "module example.com/application\n\ngo 1.27.0\n\n// module github.com/obalunenko/Fallout-Terminal/v2\n",
			wantErr: true,
		},
		{
			name:    "v1 module",
			module:  "module github.com/obalunenko/Fallout-Terminal/v1\n\ngo 1.27.0\n",
			wantErr: true,
		},
		{
			name:    "v3 module",
			module:  "module github.com/obalunenko/Fallout-Terminal/v3\n\ngo 1.27.0\n",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(test.module), 0o600))

			err := validateRoot(root)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestBuildPlanHasOneOrderedOwnerAndPortableToolInvocation(t *testing.T) {
	steps, err := Plan("build", nil)
	require.NoError(t, err)

	wantNames := []string{
		"install locked frontend dependencies",
		"verify protobuf and generated clients",
		"build client frontend",
		"generate Wails bindings",
		"build Overseer frontend",
		"create binary output directory",
		"compile macOS arm64 application",
	}
	require.Len(t, steps, len(wantNames))
	for index, want := range wantNames {
		assert.Equal(t, want, steps[index].Name)
		command := strings.Join(append([]string{steps[index].Program}, steps[index].Arguments...), " ")
		assert.Falsef(t, strings.Contains(strings.ToLower(command), "taskfile") || strings.HasPrefix(command, "task ") || strings.Contains(command, " wails3 task"),
			"step %q invokes a Taskfile tool: %q", want, command)
	}
	bindings := steps[3]
	assert.Equal(t, "go", bindings.Program)
	assert.Equal(t, []string{
		"tool",
		"-modfile=tools/wails/go.mod",
		"wails3",
		"generate",
		"bindings",
		"-clean",
		"-d",
		"frontend/overseer/bindings",
		"./...",
	}, bindings.Arguments, "the Wails generator must resolve through its portable repository-owned tool module")
	got := steps[len(steps)-1].Arguments
	assert.Equal(t, filepath.Join("build", "bin", applicationName), got[len(got)-2])
}

func TestPreparationOrderIsLockedForEveryActionAndTarget(t *testing.T) {
	wantOrder := []string{
		"install locked frontend dependencies",
		"verify protobuf and generated clients",
		"build client frontend",
		"generate Wails bindings",
		"build Overseer frontend",
	}

	for _, action := range []string{"prepare", "build", "dev", "package"} {
		t.Run(action, func(t *testing.T) {
			steps, err := Plan(action, nil)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(steps), len(wantOrder))
			assert.Equal(t, wantOrder, stepNames(steps[:len(wantOrder)]))
			assertPortablePreparation(t, steps[:len(wantOrder)])

			assert.Equal(t, "npm", steps[0].Program)
			assert.Equal(t, []string{"ci", "--prefix", "frontend"}, steps[0].Arguments)
		})
	}

	for _, target := range []Target{
		mustParseTarget(t, "windows", "arm64"),
		mustParseTarget(t, "windows", "amd64"),
		mustParseTarget(t, "linux", "arm64"),
		mustParseTarget(t, "linux", "amd64"),
	} {
		t.Run(target.String(), func(t *testing.T) {
			steps, err := PlanForTarget("build", target, nil)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(steps), len(wantOrder))
			assert.Equal(t, wantOrder, stepNames(steps[:len(wantOrder)]))
			assertPortablePreparation(t, steps[:len(wantOrder)])
			for _, step := range steps[:len(wantOrder)] {
				assert.Empty(t, step.Environment, "target environment must not leak into preparation step %q", step.Name)
			}
		})
	}
}

func TestBuildPlanTargetEnvironmentIsExplicitAndIsolated(t *testing.T) {
	tests := []struct {
		name        string
		target      Target
		environment map[string]string
	}{
		{
			name:   "macOS compatibility",
			target: DefaultTarget(),
			environment: map[string]string{
				"CGO_ENABLED":              "1",
				"CGO_CFLAGS":               "-mmacosx-version-min=" + minimumMacOS,
				"CGO_LDFLAGS":              macOSCGOLinkerFlags(minimumMacOS),
				"GOARCH":                   "arm64",
				"GOOS":                     "darwin",
				"MACOSX_DEPLOYMENT_TARGET": minimumMacOS,
			},
		},
		{name: "Windows ARM64", target: mustParseTarget(t, "windows", "arm64"), environment: map[string]string{"CGO_ENABLED": "0", "GOARCH": "arm64", "GOOS": "windows"}},
		{name: "Windows AMD64", target: mustParseTarget(t, "windows", "amd64"), environment: map[string]string{"CGO_ENABLED": "0", "GOARCH": "amd64", "GOOS": "windows"}},
		{name: "Linux ARM64", target: mustParseTarget(t, "linux", "arm64"), environment: map[string]string{"CGO_ENABLED": "1", "GOARCH": "arm64", "GOOS": "linux"}},
		{name: "Linux AMD64", target: mustParseTarget(t, "linux", "amd64"), environment: map[string]string{"CGO_ENABLED": "1", "GOARCH": "amd64", "GOOS": "linux"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			steps, err := PlanForTarget("build", test.target, nil)
			require.NoError(t, err)

			compile := requireCompileStep(t, steps)
			assert.Equal(t, test.environment, compile.Environment)

			compile.Environment["GOOS"] = "mutated"
			freshSteps, err := PlanForTarget("build", test.target, nil)
			require.NoError(t, err)
			assert.Equal(t, test.environment, requireCompileStep(t, freshSteps).Environment, "plans must not share mutable target environments")
		})
	}
}

func TestImplicitAndExplicitDarwinBuildPlansRemainDistinct(t *testing.T) {
	implicit, err := Plan("build", nil)
	require.NoError(t, err)
	explicit, err := PlanForTarget("build", DefaultTarget(), nil)
	require.NoError(t, err)

	assert.NotEqual(t, implicit, explicit)
	assert.Contains(t, requireCompileStep(t, implicit).Arguments, filepath.Join("build", "bin", applicationName))
	assert.Contains(t, requireCompileStep(t, explicit).Arguments, filepath.Join("build", "bin", "darwin-arm64", applicationName))
}

func TestDevelopmentPlansAssembleAndLaunchOwnedApplicationIdentity(t *testing.T) {
	steps, err := Plan("dev", []string{"--fixture"})
	require.NoError(t, err)

	positions := make(map[string]int, len(steps))
	for index, step := range steps {
		positions[step.Name] = index
	}

	for _, name := range []string{
		"remove previous development application bundle",
		"install development application metadata",
		"install development application icon",
		"compile macOS arm64 application",
		"run development application",
	} {
		assert.Contains(t, positions, name)
	}
	assert.Less(t, positions["install development application metadata"], positions["compile macOS arm64 application"])
	assert.Less(t, positions["install development application icon"], positions["compile macOS arm64 application"])
	assert.Less(t, positions["compile macOS arm64 application"], positions["run development application"])

	metadata := steps[positions["install development application metadata"]]
	assert.Equal(t, filepath.Join("build", "darwin", "Info.dev.plist"), metadata.Source)
	assert.Equal(t, filepath.Join("build", "dev", applicationName+".app", "Contents", "Info.plist"), metadata.Destination)

	icon := steps[positions["install development application icon"]]
	assert.Contains(t, icon.Arguments, filepath.Join("build", "appicon.png"))
	assert.Contains(t, icon.Arguments, filepath.Join("build", "dev", applicationName+".app", "Contents", "Resources", "icon.icns"))

	launch := steps[positions["run development application"]]
	assert.Equal(t, filepath.Join("build", "dev", applicationName+".app", "Contents", "MacOS", applicationName), launch.Program)
	assert.Equal(t, []string{"--fixture"}, launch.Arguments)

	productionBundle := filepath.Join("build", "bin", applicationName+".app")
	for _, step := range steps {
		for _, value := range append([]string{step.Program, step.Path, step.Source, step.Destination}, step.Arguments...) {
			assert.NotEqual(t, productionBundle, value, "development action must not mutate or launch the production package")
		}
	}
}

func TestPackagePlanCompletesResourcesBeforeFinalSignature(t *testing.T) {
	steps, err := Plan("package", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"install locked frontend dependencies",
		"verify protobuf and generated clients",
		"build client frontend",
		"generate Wails bindings",
		"build Overseer frontend",
		"verify embedded dependency and license inventory",
		"remove previous application bundle",
		"create application executable directory",
		"create bundled session directory",
		"install application metadata",
		"install application icon",
		"install bundled demo player config",
		"install bundled demo",
		"install third-party notices",
		"compile macOS arm64 application",
		"make application executable",
		"sign completed application bundle",
	}, stepNames(steps), "the existing macOS package resource and signature order is a compatibility contract")

	positions := make(map[string]int, len(steps))
	for index, step := range steps {
		positions[step.Name] = index
	}
	for _, resource := range []string{"install application metadata", "install application icon", "install bundled demo"} {
		assert.Lessf(t, positions[resource], positions["sign completed application bundle"], "%s must precede final signature", resource)
	}
	assert.Less(t, positions["compile macOS arm64 application"], positions["sign completed application bundle"], "application compilation must precede final signature")

	signature := steps[positions["sign completed application bundle"]]
	assert.Equal(t, "/usr/bin/codesign", signature.Program)
	assert.Equal(t, []string{
		"--force",
		"--deep",
		"--options",
		"runtime",
		"--entitlements",
		filepath.Join("build", "darwin", "entitlements.plist"),
		"--sign",
		"-",
		filepath.Join("build", "bin", applicationName+".app"),
	}, signature.Arguments)
}

func TestImplicitPackagePlanUsesOneVersionForMetadataAndExecutable(t *testing.T) {
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
			t.Setenv(packageVersionEnvironmentCanonical, test.input)

			steps, err := Plan("package", nil)
			require.NoError(t, err)

			positions := make(map[string]int, len(steps))
			for index, step := range steps {
				positions[step.Name] = index
			}
			metadataIndex, found := positions["install application metadata"]
			require.True(t, found)
			metadata := steps[metadataIndex]
			assert.Equal(t, renderTemplate, metadata.Operation)
			assert.Equal(t, filepath.Join("build", "darwin", "Info.plist.tmpl"), metadata.Source)
			assert.Equal(t,
				filepath.Join("build", "bin", applicationName+".app", "Contents", "Info.plist"),
				metadata.Destination,
			)
			assert.Equal(t, map[string]string{
				packageVersionEnvironmentCanonical:       test.canonical,
				packageVersionEnvironmentNumericCore:     test.numericCore,
				packageVersionEnvironmentNumericFourPart: test.numericFourPart,
			}, metadata.Environment)

			compileIndex, found := positions["compile macOS arm64 application"]
			require.True(t, found)
			assert.Less(t, metadataIndex, compileIndex)
			compile := steps[compileIndex]
			arguments := strings.Join(compile.Arguments, " ")
			linkerVersion := applicationModule + "/internal/version.value=" + test.canonical
			if test.release {
				assert.Contains(t, arguments, linkerVersion)
			} else {
				assert.NotContains(t, arguments, applicationModule+"/internal/version.value=")
			}
		})
	}

	t.Run("malformed release version", func(t *testing.T) {
		t.Setenv(packageVersionEnvironmentCanonical, "v2.4.6")
		_, err := Plan("package", nil)
		require.ErrorContains(t, err, "resolve package VERSION")
	})
}

func TestPackagePlanOwnsEmbeddedDependencyNoticesAndNoProviderExecutable(t *testing.T) {
	t.Parallel()

	steps, err := Plan("package", nil)
	require.NoError(t, err)

	positions := make(map[string]int, len(steps))
	for index, step := range steps {
		positions[step.Name] = index
		values := append([]string{step.Program, step.Path, step.Source, step.Destination}, step.Arguments...)
		joined := strings.ToLower(strings.Join(values, " "))
		assert.NotContains(t, joined, "ngrok", "package plan must not copy or execute a provider binary")
		assert.NotContains(t, joined, "curl", "package plan must not download a provider runtime")
	}

	dependencyIndex, exists := positions["verify embedded dependency and license inventory"]
	require.True(t, exists, "package must run the exact SDK/license inventory gate")
	assert.Equal(t, filepath.Join("scripts", "dependency-license-check.sh"), steps[dependencyIndex].Program)

	noticeIndex, exists := positions["install third-party notices"]
	require.True(t, exists, "package must include reviewed third-party notices")
	notice := steps[noticeIndex]
	assert.Equal(t, "THIRD_PARTY_NOTICES.md", notice.Source)
	assert.Equal(t, filepath.Join("build", "bin", applicationName+".app", "Contents", "Resources", "THIRD_PARTY_NOTICES.md"), notice.Destination)
	assert.Equal(t, 0o444, int(notice.Mode.Perm()))
	assert.Less(t, noticeIndex, positions["compile macOS arm64 application"])
	assert.Less(t, noticeIndex, positions["sign completed application bundle"])

	compile := steps[positions["compile macOS arm64 application"]]
	assert.Equal(t, "darwin", compile.Environment["GOOS"])
	assert.Equal(t, "arm64", compile.Environment["GOARCH"])
	assert.Equal(t, "1", compile.Environment["CGO_ENABLED"])
	assert.Equal(t, minimumMacOS, compile.Environment["MACOSX_DEPLOYMENT_TARGET"])
	assert.Equal(t, "-mmacosx-version-min="+minimumMacOS, compile.Environment["CGO_CFLAGS"])
	assert.Equal(t, macOSCGOLinkerFlags(minimumMacOS), compile.Environment["CGO_LDFLAGS"])
}

func TestPackagePlanPreservesCanonicalFrontendAndOfflineResourceOwnership(t *testing.T) {
	t.Parallel()

	steps, err := Plan("package", nil)
	require.NoError(t, err)

	positions := make(map[string]int, len(steps))
	for index, step := range steps {
		positions[step.Name] = index
	}
	ordered := []string{
		"install locked frontend dependencies",
		"verify protobuf and generated clients",
		"build client frontend",
		"generate Wails bindings",
		"build Overseer frontend",
		"install application metadata",
		"install application icon",
		"install bundled demo player config",
		"install bundled demo",
		"compile macOS arm64 application",
		"sign completed application bundle",
	}
	for index := 1; index < len(ordered); index++ {
		assert.Lessf(t, positions[ordered[index-1]], positions[ordered[index]], "%s must precede %s", ordered[index-1], ordered[index])
	}

	demo := steps[positions["install bundled demo"]]
	assert.Equal(t, filepath.Join("sessions", "demo.json"), demo.Source)
	assert.Equal(t, filepath.Join("build", "bin", applicationName+".app", "Contents", "Resources", "sessions", "demo.json"), demo.Destination)
	assert.Equal(t, 0o444, int(demo.Mode.Perm()))

	players := steps[positions["install bundled demo player config"]]
	assert.Equal(t, filepath.Join("sessions", "demo-players.json"), players.Source)
	assert.Equal(t, filepath.Join("build", "bin", applicationName+".app", "Contents", "Resources", "sessions", "demo-players.json"), players.Destination)
	assert.Equal(t, 0o444, int(players.Mode.Perm()))
}

func TestUnknownActionIsRejected(t *testing.T) {
	for _, action := range []string{"run", "task"} {
		t.Run(action, func(t *testing.T) {
			_, err := Plan(action, nil)
			require.Error(t, err)
		})
	}
}

func mustParseTarget(t *testing.T, goos, goarch string) Target {
	t.Helper()

	target, err := ParseTarget(goos, goarch)
	require.NoError(t, err)
	return target
}

func stepNames(steps []Step) []string {
	names := make([]string, len(steps))
	for index, step := range steps {
		names[index] = step.Name
	}
	return names
}

func assertPortablePreparation(t *testing.T, steps []Step) {
	t.Helper()

	for _, step := range steps {
		invocation := strings.Join(append([]string{step.Program}, step.Arguments...), " ")
		assert.Falsef(t, filepath.IsAbs(step.Program), "preparation step %q uses an absolute host-specific program: %q", step.Name, invocation)
		assert.NotContainsf(t, strings.ToLower(invocation), ".sh", "preparation step %q relies on a Unix shell script", step.Name)
		assert.NotContainsf(t, strings.ToLower(invocation), ".ps1", "preparation step %q relies on a Windows shell script", step.Name)
	}
}

func requireCompileStep(t *testing.T, steps []Step) Step {
	t.Helper()

	for _, step := range steps {
		if step.Program == "go" && len(step.Arguments) > 0 && step.Arguments[0] == "build" {
			return step
		}
	}
	require.FailNow(t, "build plan has no Go compile step")
	return Step{}
}
