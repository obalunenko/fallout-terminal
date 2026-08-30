package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWailsV3GoToolsAreIsolatedFromApplicationModule(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	tests := []struct {
		name          string
		directory     string
		tool          string
		parentRequire string
	}{
		{
			name:          "Wails CLI",
			directory:     "wails",
			tool:          "github.com/wailsapp/wails/v3/cmd/wails3",
			parentRequire: "github.com/wailsapp/wails/v3 v3.0.0-beta.15",
		},
		{
			name:          "Go Task",
			directory:     "task",
			tool:          "github.com/go-task/task/v3/cmd/task",
			parentRequire: "github.com/go-task/task/v3 v3.53.1",
		},
		{
			name:          "Buf CLI",
			directory:     "buf",
			tool:          "github.com/bufbuild/buf/cmd/buf",
			parentRequire: "github.com/bufbuild/buf v1.72.0",
		},
		{
			name:          "golangci-lint",
			directory:     "golangci-lint",
			tool:          "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
			parentRequire: "github.com/golangci/golangci-lint/v2 v2.13.1",
		},
		{
			name:          "protoc-gen-go",
			directory:     "protoc-gen-go",
			tool:          "google.golang.org/protobuf/cmd/protoc-gen-go",
			parentRequire: "google.golang.org/protobuf v1.36.11",
		},
		{
			name:          "protoc-gen-connect-go",
			directory:     "protoc-gen-connect-go",
			tool:          "connectrpc.com/connect/cmd/protoc-gen-connect-go",
			parentRequire: "connectrpc.com/connect v1.20.0",
		},
		{
			name:          "GoReleaser",
			directory:     "goreleaser",
			tool:          "github.com/goreleaser/goreleaser/v2",
			parentRequire: "github.com/goreleaser/goreleaser/v2 v2.18.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			module := readAcceptanceDocument(t, filepath.Join(root, "tools", test.directory, "go.mod"))
			assert.Equal(t, 1, strings.Count(module, "\ntool "))
			assert.Contains(t, module, "\ntool "+test.tool+"\n")
			parentPattern := "(?m)^[[:space:]]*(require[[:space:]]+)?" +
				regexp.QuoteMeta(test.parentRequire) + "([[:space:]]+// indirect)?$"
			assert.Regexp(t, parentPattern, module)
			assert.Contains(t, module, "\ngo 1.27.0\n")

			sum, err := os.ReadFile(filepath.Join(root, "tools", test.directory, "go.sum"))
			require.NoError(t, err)
			require.NotEmpty(t, sum)
		})
	}

	applicationModule := readAcceptanceDocument(t, filepath.Join(root, "go.mod"))
	assert.NotContains(t, applicationModule, "\ntool ")
	assert.NotContains(t, applicationModule, "github.com/bufbuild/buf")
	assert.NotContains(t, applicationModule, "github.com/golangci/golangci-lint")
	assert.NotContains(t, applicationModule, "/cmd/protoc-gen-go")
	assert.NotContains(t, applicationModule, "/cmd/protoc-gen-connect-go")
	assert.NotContains(t, applicationModule, "/v3/cmd/wails3")
	assert.NotContains(t, applicationModule, "github.com/go-task/task")
	assert.NotContains(t, applicationModule, "github.com/goreleaser/goreleaser")
	assert.NotContains(t, applicationModule, "oras.land/oras")
}

func TestWailsV3PinsAndGoBuildToolAreOwnedAndExact(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	applicationModule := readAcceptanceDocument(t, filepath.Join(root, "go.mod"))
	assert.Equal(t, 1, strings.Count(applicationModule, "github.com/wailsapp/wails/v3 v3.0.0-beta.15"))

	packageRaw, err := os.ReadFile(filepath.Join(root, "frontend", "overseer", "package.json"))
	require.NoError(t, err)
	var packageConfig struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	require.NoError(t, json.Unmarshal(packageRaw, &packageConfig))
	assert.Equal(t, "3.0.0-beta.15", packageConfig.Dependencies["@wailsio/runtime"])

	lock := readAcceptanceDocument(t, filepath.Join(root, "frontend", "package-lock.json"))
	assert.Contains(t, lock, `"@wailsio/runtime": "3.0.0-beta.15"`)
	assert.Contains(t, lock, `runtime-3.0.0-beta.15.tgz`)

	files := []struct {
		path   string
		tokens []string
	}{
		{"cmd/build/main.go", []string{"buildtool.Run", "package-all-docker", "validate-release-tag", "inspect-release-archive", "inspect-release-inventory"}},
		{"internal/buildtool/buildtool.go", []string{"scripts", "verifyProtobufAndGeneratedClients", "tools/wails/go.mod", "frontend/overseer/bindings", "GOARCH", "arm64", "13.0", `applicationName+".app"`}},
		{"internal/buildtool/preflight.go", []string{"verifyGeneratedContracts", "tools/buf/go.mod", "generate Go protobuf contracts", "verifyPlayerFrontend", "verifyOverseerFrontend"}},
		{"internal/buildtool/docker.go", []string{"PackageAllDocker", "packageDarwinAggregateBundle", "darwin-arm64", "linux/", "SOURCE_REVISION", "atomically publish Docker package matrix"}},
		{
			"build/docker/Dockerfile.package",
			[]string{
				"golang:1.27-trixie",
				"node:24-trixie",
				"libgtk-4-dev",
				"libwebkitgtk-6.0-dev",
				"package-container",
				"/export/bin/",
			},
		},
		{".dockerignore", []string{".git", "**/node_modules", "build/dist", "**/.env*"}},
		{"build/darwin/Info.plist.tmpl", []string{"com.vaulttec.fallout-terminal", "13.0", "icon.icns", "{{VERSION}}", "{{NUMERIC_CORE}}"}},
		{"build/darwin/Info.dev.plist", []string{"com.vaulttec.fallout-terminal", "13.0", "icon.icns"}},
		{"build/darwin/entitlements.plist", []string{"com.apple.security.network.server"}},
	}

	icon, err := os.Stat(filepath.Join(root, "build", "appicon.png"))
	require.NoError(t, err)
	assert.True(t, icon.Mode().IsRegular())
	assert.Positive(t, icon.Size(), "the development application icon source must not be empty")

	buildSource := readAcceptanceDocument(t, filepath.Join(root, "internal", "buildtool", "buildtool.go"))
	for _, required := range []string{
		`filepath.Join("build", "dev", applicationName+".app")`,
		`filepath.Join("build", "darwin", "Info.dev.plist")`,
		`Name: "install development application metadata"`,
		`commandStep("install development application icon"`,
		`commandStep("run development application"`,
	} {
		assert.Contains(t, buildSource, required)
	}
	for _, file := range files {
		t.Run(file.path, func(t *testing.T) {
			t.Parallel()
			contents := readAcceptanceDocument(t, filepath.Join(root, filepath.FromSlash(file.path)))
			for _, token := range file.tokens {
				assert.Contains(t, contents, token)
			}
		})
	}
	assert.NoFileExists(t, filepath.Join(root, "build", "darwin", "Info.plist"))
	assert.FileExists(t, filepath.Join(root, "build", "darwin", "Info.dev.plist"))

	for _, path := range []string{
		"Taskfile.yaml",
		"build/Taskfile.yml",
		"build/common/Taskfile.yml",
		"build/darwin/Taskfile.yml",
	} {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		assert.ErrorIs(t, err, os.ErrNotExist, "%s must not exist", path)
	}
}

func TestTaskfileOwnsWailsCompatibleWorkflowsAndMakeOnlyBootstrapsTools(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	taskfile := readAcceptanceDocument(t, filepath.Join(root, "Taskfile.yml"))
	assert.Contains(t, taskfile, "version: '3'")

	for _, taskName := range []string{
		"dev",
		"prepare",
		"build",
		"package",
		"package:all",
		"ci:quality",
		"deps",
		"deps:frontend",
		"deps:browser",
		"speckit:install",
		"speckit:update:check",
		"speckit:update:test",
		"fmt",
		"fmt:check",
		"vet",
		"lint",
		"test",
		"test:race",
		"proto:generate",
		"proto:check",
		"proto:breaking",
		"bindings:check",
		"browser:test",
		"check",
		"release:macos:preflight",
		"release:tag",
		"release:publish",
		"release:macos:signed",
	} {
		t.Run("task "+taskName, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, taskfileTask(t, taskfile, taskName))
		})
	}

	for _, action := range []string{"dev", "prepare", "build", "package"} {
		body := taskfileTask(t, taskfile, action)
		assert.Contains(t, body, "run ./cmd/build "+action)
		assert.NotContains(t, body, "wails3", "%s must not recurse through a high-level Wails wrapper", action)
	}

	for _, taskName := range []string{"build", "package"} {
		body := taskfileTask(t, taskfile, taskName)
		assert.Contains(t, body, "{{.GOOS}}")
		assert.Contains(t, body, "{{.GOARCH}}")
		assert.Contains(t, body, `--target "{{.GOOS}}/{{.GOARCH}}"`)
	}

	packageAll := taskfileTask(t, taskfile, "package:all")
	assert.Contains(t, packageAll, "run ./cmd/build package-all-docker")
	assert.Contains(t, packageAll, `--output "{{.OUTPUT}}"`)
	assert.NotContains(t, packageAll, "REF")
	assert.NotContains(t, packageAll, "--ref")
	releasePublish := taskfileTask(t, taskfile, "release:publish")
	assert.Contains(t, releasePublish, "tools/goreleaser/go.mod")
	assert.Contains(t, releasePublish, "goreleaser release")
	releaseTag := taskfileTask(t, taskfile, "release:tag")
	for _, required := range []string{
		"tools/svu/go.mod",
		"svu current",
		"svu_command=next",
		"svu_command=major",
		"set -- next --always",
		"set -- major",
		`--prerelease "$PRERELEASE"`,
		`svu "$@"`,
		"validate-release-tag",
		"git status --porcelain",
		`git tag "{{.RELEASE_TAG}}"`,
		`git push origin "refs/tags/{{.RELEASE_TAG}}"`,
		`git tag --delete "{{.RELEASE_TAG}}"`,
	} {
		assert.Contains(t, releaseTag, required)
	}
	assert.Contains(t, releaseTag, "prompt:")

	dev := taskfileTask(t, taskfile, "dev")
	assert.Contains(t, dev, "{{.APP_ARGS}}")

	deps := taskfileTask(t, taskfile, "deps")
	assert.Contains(t, deps, "task: deps:frontend")
	assert.Contains(t, deps, "task: deps:browser")
	protoGenerate := taskfileTask(t, taskfile, "proto:generate")
	protoCheck := taskfileTask(t, taskfile, "proto:check")
	assert.Contains(t, protoGenerate, "task: deps:frontend")
	assert.Contains(t, protoCheck, "task: deps:frontend")
	browserTest := taskfileTask(t, taskfile, "browser:test")
	assert.Contains(t, browserTest, "task: deps:frontend")
	assert.Contains(t, browserTest, "task: deps:browser")

	check := taskfileTask(t, taskfile, "check")
	for _, dependency := range []string{
		"fmt:check",
		"vet",
		"lint",
		"test:race",
		"proto:check",
		"proto:breaking",
		"bindings:check",
		"speckit:update:test",
	} {
		assert.Contains(t, check, "task: "+dependency)
	}

	for _, forbidden := range []string{
		"wails3 dev",
		"wails3 run",
		"wails3 build",
		"wails3 package",
		"wails3 task build",
		"wails3 task package",
	} {
		assert.NotContains(t, taskfile, forbidden)
	}

	makefile := readAcceptanceDocument(t, filepath.Join(root, "Makefile"))
	assert.Contains(t, makefile, ".DEFAULT_GOAL := tools")
	assert.Contains(t, makefile, "$(sort $(wildcard tools/*/go.mod))")
	assert.Contains(t, makefile, "$(GO) install tool")
	assert.Contains(t, makefile, `module_dir="$${module_file%/go.mod}"`)
	assert.Contains(t, makefile, "task --list")

	targetPattern := regexp.MustCompile(`(?m)^([[:alnum:]_-]+):[^=\n]*$`)
	matches := targetPattern.FindAllStringSubmatch(makefile, -1)
	require.Len(t, matches, 2, "Make must expose only the tools bootstrap and non-mutating help targets")
	assert.Equal(t, "tools", matches[0][1])
	assert.Equal(t, "help", matches[1][1])
}

func TestReleaseToolSurfaceUsesTaskAndPinnedReleaseTools(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	taskfile := readAcceptanceDocument(t, filepath.Join(root, "Taskfile.yml"))
	mainSource := readAcceptanceDocument(t, filepath.Join(root, "cmd", "build", "main.go"))
	releaseWorkflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-portable.yml"))
	qualityWorkflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-cross-platform.yml"))

	for _, taskName := range []string{"package", "package:all", "release:tag", "release:publish"} {
		require.NotEmpty(t, taskfileTask(t, taskfile, taskName))
	}
	for _, removed := range []string{"package:all:remote", "release:local"} {
		assert.NotContains(t, taskfile, "\n  "+removed+":\n")
	}
	assert.Contains(t, taskfileTask(t, taskfile, "package:all"), "package-all-docker")
	assert.Contains(t, taskfileTask(t, taskfile, "release:publish"), "tools/goreleaser/go.mod")
	assert.Contains(t, taskfileTask(t, taskfile, "release:publish"), "goreleaser release --clean --config .goreleaser.yaml")
	assert.Contains(t, taskfileTask(t, taskfile, "release:tag"), "tools/svu/go.mod")

	for _, removedAction := range []string{`case "package-all":`, `case "release-candidate":`} {
		assert.NotContains(t, mainSource, removedAction)
	}
	assert.Contains(t, mainSource, `case "package-all-docker":`)

	assert.FileExists(t, filepath.Join(root, "tools", "goreleaser", "go.mod"))
	assert.FileExists(t, filepath.Join(root, "tools", "svu", "go.mod"))
	assert.NoFileExists(t, filepath.Join(root, "tools", "oras", "go.mod"))
	for _, workflow := range []string{releaseWorkflow, qualityWorkflow} {
		assert.NotContains(t, strings.ToLower(workflow), "package-all-docker")
		assert.NotContains(t, strings.ToLower(workflow), "release:macos:signed")
		assert.NotContains(t, strings.ToLower(workflow), "oras")
	}

	makefile := readAcceptanceDocument(t, filepath.Join(root, "Makefile"))
	targetPattern := regexp.MustCompile(`(?m)^([[:alnum:]_-]+):[^=\n]*$`)
	require.Len(t, targetPattern.FindAllStringSubmatch(makefile, -1), 2)
}

func TestGoPackageOutputDeploymentTargetAndFinalSignOrderAreExplicit(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	buildSource := readAcceptanceDocument(t, filepath.Join(root, "internal", "buildtool", "buildtool.go"))
	for _, required := range []string{
		`minimumMacOS    = "13.0"`,
		`filepath.Join("build", "bin", applicationName+".app")`,
		`"GOARCH":      target.Arch()`,
		`"GOOS":        target.OS()`,
		`environment["MACOSX_DEPLOYMENT_TARGET"] = minimumMacOS`,
		`commandStep("sign completed application bundle"`,
	} {
		assert.Contains(t, buildSource, required)
	}

	metadata := readAcceptanceDocument(t, filepath.Join(root, "build", "darwin", "Info.plist.tmpl"))
	assert.Contains(t, metadata, "<key>LSMinimumSystemVersion</key>")
	assert.Contains(t, metadata, "<string>13.0</string>")
	assert.Contains(t, metadata, "<string>icon.icns</string>")
	assert.Contains(t, metadata, "<string>{{VERSION}}</string>")
	assert.Contains(t, metadata, "<string>{{NUMERIC_CORE}}</string>")

	packageStart := strings.Index(buildSource, "func packageSteps(version ReleaseVersion) []Step {")
	require.NotEqual(t, -1, packageStart)
	packageEnd := strings.Index(buildSource[packageStart:], "\nfunc compileStep(")
	require.NotEqual(t, -1, packageEnd)
	packageSource := buildSource[packageStart : packageStart+packageEnd]
	installDemo := strings.Index(packageSource, `Name: "install bundled demo"`)
	compile := strings.Index(packageSource, `versionedCompileStep(DefaultTarget(), executable, version)`)
	sign := strings.Index(packageSource, `commandStep("sign completed application bundle"`)
	require.NotEqual(t, -1, installDemo)
	require.NotEqual(t, -1, compile)
	require.NotEqual(t, -1, sign)
	assert.Less(t, installDemo, compile)
	assert.Less(t, compile, sign)
}

func TestReproducibleBuildHashesPackagedExecutableAndUsesQuietToolEnvironments(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	reproducible := readAcceptanceDocument(t, filepath.Join(root, "scripts", "reproducible-build-check.sh"))
	assert.Contains(t, reproducible, `application_executable="${application_bundle}/Contents/MacOS/Fallout Terminal"`)
	assert.NotContains(t, reproducible, `shasum -a 256 "build/bin/Fallout Terminal"`)

	protoCheck := readAcceptanceDocument(t, filepath.Join(root, "scripts", "proto-check.sh"))
	assert.Contains(t, protoCheck, `if [[ "$(go env GOOS)" == darwin ]]; then
  export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
  export CGO_CFLAGS="${CGO_CFLAGS:--mmacosx-version-min=${MACOSX_DEPLOYMENT_TARGET}}"
  export CGO_LDFLAGS="${CGO_LDFLAGS:--mmacosx-version-min=${MACOSX_DEPLOYMENT_TARGET} -Wl,-no_warn_duplicate_libraries}"
fi`)

	protoGenerate := readAcceptanceDocument(t, filepath.Join(root, "scripts", "proto-generate.sh"))
	assert.Contains(t, protoGenerate, `node_major >= 22`)
	assert.Contains(t, protoGenerate, `--no-experimental-webstorage`)
}

func TestStandaloneMacOSWorkflowIsRemovedFromActiveAutomation(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	assert.NoFileExists(t, filepath.Join(root, ".github", "workflows", "wails-macos.yml"))
	portable := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-portable.yml"))
	protoCheck := readAcceptanceDocument(t, filepath.Join(root, "scripts", "proto-check.sh"))
	assert.NotContains(t, portable, "wails-macos.yml")
	assert.NotContains(t, protoCheck, "wails-macos.yml")
}

func TestWailsV3RollbackRecordHasIdentitySafetyTriggersAndHonestEvidenceFields(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	rollback := readAcceptanceDocument(t, filepath.Join(root, "docs", "wails-v3-migration-rollback.md"))
	for _, required := range []string{
		"f1084b3df8b5630862bdf7a0f347b599156653ef",
		"Source verification | `PASS`",
		"Artifact status | `BUILT FOR DRILL — ACCEPTED FOR THIS DRILL`",
		"Executable SHA-256 | `c1faf7fe4f2ed0abc5c4814b8e71805f5b57a65b817fd3a45bbcc90bdaf29530`",
		"invent or prefill an artifact digest",
		"bcb207704657a92f9902f4ac04ef11765b18f031",
		"provenance only—not the build candidate",
		"## Rollback Triggers",
		"| Trigger | Required action | Decision owner |",
		"## Data-Safe Rollback Procedure",
		"safety copies",
		"Record SHA-256 values for the originals and safety copies",
		"separate maintenance worktree or clone",
		"Open only the safety-copy version-1 data without migration or conversion",
		"## Rollback Drill Evidence",
		"Overall drill result | `PASS`",
	} {
		assert.Contains(t, rollback, required)
	}
	assert.NotContains(t, rollback, "Artifact status | `PASS`")
	assert.Contains(t, rollback, "immutable source commit remains canonical rollback authority")
}

func TestAcceptanceEvidenceUsesOneCanonicalPostElectronCandidate(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	quickstart := readAcceptanceDocument(t, filepath.Join(root, "specs", "001-wails-v2-migration", "quickstart.md"))
	rollback := readAcceptanceDocument(t, filepath.Join(root, "docs", "wails-migration-rollback.md"))

	quickstartCommit, quickstartDigest := canonicalCandidate(t, "quickstart", quickstart)
	rollbackCommit, rollbackDigest := canonicalCandidate(t, "rollback guide", rollback)
	assert.Falsef(t, quickstartCommit != rollbackCommit,
		"canonical candidate commit conflicts: quickstart=%s rollback=%s", quickstartCommit, rollbackCommit)
	assert.Falsef(t, quickstartDigest != rollbackDigest,
		"canonical executable SHA-256 conflicts: quickstart=%s rollback=%s", quickstartDigest, rollbackDigest)

}

func TestDistributionGuidanceDocumentsPortablePlatformsAndPackaging(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	readme := readAcceptanceDocument(t, filepath.Join(root, "README.md"))
	support := readAcceptanceDocument(t, filepath.Join(root, "docs", "platform-support.md"))
	packaging := readAcceptanceDocument(t, filepath.Join(root, "docs", "platform-packaging.md"))

	for _, document := range []string{readme, packaging, support} {
		for _, required := range []string{
			"darwin/arm64", "Fallout-Terminal-darwin-arm64.zip",
			"windows/amd64", "Fallout-Terminal-windows-amd64.zip",
			"windows/arm64", "Fallout-Terminal-windows-arm64.zip",
			"linux/amd64", "Fallout-Terminal-linux-amd64.tar.gz",
			"linux/arm64", "Fallout-Terminal-linux-arm64.tar.gz",
		} {
			assert.Contains(t, document, required)
		}
	}

	normalizedPackaging := strings.Join(strings.Fields(packaging), " ")
	for _, required := range []string{
		"v2.0.0",
		"v2.0.0-rc.1",
		"github.com/obalunenko/Fallout-Terminal/v2",
		"VERSION=2.0.0",
		"VERSION=2.0.0-rc.1",
		"development",
		"The tag major must match the root Go module major",
	} {
		assert.Contains(t, normalizedPackaging, required)
	}
	assert.NotContains(t, normalizedPackaging, "VERSION=v2.", "canonical VERSION must omit the tag's leading v")
	assert.Contains(t, strings.ToLower(normalizedPackaging), "local", "the packaging guide must identify development as local non-release behavior")

	for _, required := range []string{
		"make tools",
		"make help",
		"task dev",
		"task build",
		"task package",
		"task package:all",
		"pull requests", "main", "tag",
		"docs/platform-support.md",
		"docs/platform-packaging.md",
	} {
		assert.Contains(t, readme, required)
	}

	for _, required := range []string{
		"Windows 10", "Windows 11", "WebView2",
		"GTK4", "WebKitGTK 6.0", "Secret Service",
		"macOS 13", "Apple Silicon", "unsigned", "Keychain",
		"Windows Credential Manager", "%APPDATA%", "~/.config", "~/Library/Application Support",
		"Fallout Terminal.exe", "./Fallout Terminal", "Fallout Terminal.app",
		"archive availability", "optional", "NOT RUN",
		"локальн", "устранение неполадок",
	} {
		assert.Contains(t, support, required)
	}

	for _, required := range []string{
		"make tools",
		"make help",
		"task dev", "task prepare", "task build", "task package",
		"task deps", "task fmt", "task vet", "task lint", "task test",
		"task proto:generate", "task proto:check", "task proto:breaking",
		"task bindings:check", "task browser:test", "task check",
		"task release:tag", "PRERELEASE=rc.1",
		"task release:macos:preflight", "task release:publish", "task release:macos:signed",
		"GOOS", "GOARCH",
		"task package GOOS=windows GOARCH=amd64",
		"task package GOOS=windows GOARCH=arm64",
		"task package GOOS=linux GOARCH=amd64",
		"task package GOOS=linux GOARCH=arm64",
		"task package GOOS=darwin GOARCH=arm64",
		"task package:all", "OUTPUT=", "local", "Docker", "never runs in CI",
		"exactly five", "pinned GoReleaser", "create-only",
		"rerun the same tag immediately", "Delete the partial release manually",
		"не публикуется", "код завершения",
	} {
		assert.Contains(t, packaging, required)
	}

	for _, document := range []string{readme, packaging, support} {
		for _, obsolete := range []string{
			"package:all:remote", "release:local", "aggregate-index.json", "GitHub Packages", "tools/oras", ".dmg.sha256",
		} {
			assert.NotContains(t, document, obsolete)
		}
	}
}

func readAcceptanceDocument(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		require.NoError(t, err)
	}
	return string(raw)
}

func canonicalCandidate(t *testing.T, name, document string) (string, string) {
	t.Helper()
	commit := canonicalValue(t, name, document, "Canonical candidate commit: `", 40)
	digest := canonicalValue(t, name, document, "Canonical executable SHA-256: `", 64)
	return commit, digest
}

func canonicalValue(t *testing.T, name, document, prefix string, length int) string {
	t.Helper()
	{
		count := strings.Count(document, prefix)
		require.Falsef(t, count != 1,
			"%s contains %d %q records, want exactly one", name, count, strings.TrimSuffix(prefix, "`"))
	}

	start := strings.Index(document, prefix) + len(prefix)
	rest := document[start:]
	end := strings.IndexByte(rest, '`')
	require.Falsef(t, end < 0,
		"%s canonical record %q has no closing backtick", name, strings.TrimSuffix(prefix, "`"))

	value := rest[:end]
	require.Falsef(t, len(value) != length,
		"%s canonical record %q has %d characters, want %d", name, strings.TrimSuffix(prefix, "`"), len(value), length)

	for _, character := range value {
		require.Falsef(t, !strings.ContainsRune("0123456789abcdef", character),
			"%s canonical record %q is not lowercase hexadecimal: %q", name, strings.TrimSuffix(prefix, "`"), value)

	}
	return value
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.False(t, !ok,
		"cannot resolve startup test location")

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func taskfileTask(t *testing.T, document, name string) string {
	t.Helper()

	_, tasks, ok := strings.Cut(document, "\ntasks:\n")
	require.True(t, ok, "Taskfile has no tasks mapping")

	marker := "  " + name + ":\n"
	start := strings.Index(tasks, marker)
	require.NotEqualf(t, -1, start, "Taskfile has no %q task", name)
	rest := tasks[start+len(marker):]

	nextTask := regexp.MustCompile(`(?m)^  [[:alnum:]_-]+(?::[[:alnum:]_-]+)*:\n`).FindStringIndex(rest)
	if nextTask == nil {
		return rest
	}
	return rest[:nextTask[0]]
}
