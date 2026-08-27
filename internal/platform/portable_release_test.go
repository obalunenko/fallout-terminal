package platform

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityWorkflowIsReadOnlyAndSeparatedFromReleaseAutomation(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-cross-platform.yml"))
	taskfile := readAcceptanceDocument(t, filepath.Join(root, "Taskfile.yml"))

	for _, required := range []string{
		"pull_request:",
		"types: [opened, synchronize, reopened]",
		"push:",
		"- main",
		"permissions:",
		"contents: read",
		"go tool -modfile=tools/task/go.mod task ci:quality",
	} {
		assert.Contains(t, workflow, required)
	}
	for _, forbidden := range []string{
		"workflow_dispatch:",
		"contents: write",
		"packages: write",
		"strategy:",
		"matrix:",
		"windows/amd64",
		"linux/arm64",
		"darwin/arm64",
		"actions/upload-artifact",
		"goreleaser",
		"release:publish",
		"task package",
		"package:all",
	} {
		assert.NotContains(t, workflow, forbidden)
	}

	quality := taskfileTask(t, taskfile, "ci:quality")
	require.NotEmpty(t, quality)
	for _, dependency := range []string{
		"test",
		"vet",
		"frontend:build",
		"startup:check",
		"wails:pins:check",
		"bindings:check",
		"proto:format:check",
		"proto:lint",
		"proto:drift:check",
		"proto:breaking",
		"proto:generated:check",
	} {
		assert.Contains(t, quality, "task: "+dependency)
	}
	for _, forbidden := range []string{"package", "release", "docker", "sign", "native-ui"} {
		assert.NotContains(t, strings.ToLower(quality), forbidden)
	}
}

func TestQualityWorkflowUsesLockedFrontendAndExactWailsContracts(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-cross-platform.yml"))
	taskfile := readAcceptanceDocument(t, filepath.Join(root, "Taskfile.yml"))
	bindingsCheck := readAcceptanceDocument(t, filepath.Join(root, "scripts", "wails-bindings-check.sh"))
	composition := workflow + "\n" + taskfile
	for _, required := range []string{
		"ci --prefix frontend",
		"run build:overseer --prefix frontend",
		"run build:client --prefix frontend",
		"scripts/wails-v3-contract-check.sh",
		"scripts/wails-bindings-check.sh",
		"scripts/proto-drift-test.sh",
		"scripts/proto-breaking.sh",
	} {
		assert.Contains(t, composition, required)
	}
	for _, pin := range []string{"v3.0.0-beta.13", "3.0.0-beta.13"} {
		assert.Contains(t, composition, pin)
	}
	assert.Contains(t, bindingsCheck, `GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/fallout-terminal-go-cache}`)
	assert.NotContains(t, bindingsCheck, "/private/tmp")
}

func TestTaskfileAlignsDarwinCGOQualityDeploymentTarget(t *testing.T) {
	t.Parallel()

	taskfile := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), "Taskfile.yml"))
	for _, required := range []string{
		"MACOS_MINIMUM_VERSION: '13.0'",
		`DARWIN_CGO_ENV: '{{if eq OS "darwin"}}env`,
		"MACOSX_DEPLOYMENT_TARGET={{.MACOS_MINIMUM_VERSION}}",
		"CGO_CFLAGS=-mmacosx-version-min={{.MACOS_MINIMUM_VERSION}}",
		"CGO_LDFLAGS=-mmacosx-version-min={{.MACOS_MINIMUM_VERSION}}",
	} {
		assert.Contains(t, taskfile, required)
	}

	for taskName, command := range map[string]string{
		"startup:check": "test ./internal/platform",
		"test":          "test ./...",
		"test:race":     "test -race ./...",
		"vet":           "vet ./...",
	} {
		assert.Contains(t, taskfileTask(t, taskfile, taskName), "{{.DARWIN_CGO_ENV}} {{.GO}} "+command)
	}
}

func TestPortableReleaseWorkflowIsTagOnlyCreateOnlyFiveTargetCoordination(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-portable.yml"))
	for _, required := range []string{
		"tags:",
		"- 'v*'",
		"preflight:",
		"needs: [preflight]",
		"fail-fast: false",
		"runs-on: ${{ matrix.runner }}",
		"windows-2025",
		"windows-11-arm",
		"ubuntu-24.04",
		"ubuntu-24.04-arm",
		"macos-15",
		"task package GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }}",
		"inspect-release-archive",
		"inspect-release-inventory",
		"needs: [package]",
		"task release:publish",
		"contents: write",
		"--paginate",
		"include drafts",
		"No release exists; rerun the same tag immediately.",
		"Delete the partial release manually, then rerun the same tag.",
	} {
		assert.Contains(t, workflow, required)
	}
	for _, target := range []string{"windows/amd64", "windows/arm64", "linux/amd64", "linux/arm64", "darwin/arm64"} {
		assert.Equal(t, 1, strings.Count(workflow, "target: "+target), target)
	}
	for _, archive := range []string{
		"Fallout-Terminal-windows-amd64.zip",
		"Fallout-Terminal-windows-arm64.zip",
		"Fallout-Terminal-linux-amd64.tar.gz",
		"Fallout-Terminal-linux-arm64.tar.gz",
		"Fallout-Terminal-darwin-arm64.zip",
	} {
		assert.Contains(t, workflow, archive)
	}
	for _, forbidden := range []string{
		"workflow_dispatch:",
		"pull_request:",
		"branches:",
		"packages: write",
		".sha256",
		"aggregate-index.json",
		"verification.json",
		".dmg",
		"wails-macos.yml",
		"gh release create",
		"gh release delete",
		"oras",
		"rollback",
		"replace",
	} {
		assert.NotContains(t, strings.ToLower(workflow), strings.ToLower(forbidden))
	}
	assert.GreaterOrEqual(t, strings.Count(workflow, "gh api --paginate"), 3, "preflight, pre-publish, and failure diagnosis must inspect all release states")
}

func TestGoReleaserPublishesOnlyFivePrebuiltArchives(t *testing.T) {
	t.Parallel()

	config := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), ".goreleaser.yaml"))
	for _, required := range []string{
		"version: 2",
		"skip: true",
		"disable: true",
		"draft: false",
		"prerelease: auto",
		"./combined/Fallout-Terminal-windows-amd64.zip",
		"./combined/Fallout-Terminal-windows-arm64.zip",
		"./combined/Fallout-Terminal-linux-amd64.tar.gz",
		"./combined/Fallout-Terminal-linux-arm64.tar.gz",
		"./combined/Fallout-Terminal-darwin-arm64.zip",
	} {
		assert.Contains(t, config, required)
	}
	assert.Equal(t, 5, strings.Count(config, "- glob: ./combined/"))
	for _, forbidden := range []string{"keep-existing", "replace_existing_artifacts", ".sha256", ".dmg", "aggregate-index", "publisher"} {
		assert.NotContains(t, strings.ToLower(config), strings.ToLower(forbidden))
	}
}
