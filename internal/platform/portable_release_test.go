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
	for _, pin := range []string{"v3.0.0-beta.15", "3.0.0-beta.15"} {
		assert.Contains(t, composition, pin)
	}
	assert.Contains(t, bindingsCheck, `GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/fallout-terminal-go-cache}`)
	assert.NotContains(t, bindingsCheck, "/private/tmp")
}

func TestNodeRuntimePolicyIsAlignedAcrossActiveSurfaces(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	qualityWorkflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-cross-platform.yml"))
	portableWorkflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-portable.yml"))
	frontendLock := readAcceptanceDocument(t, filepath.Join(root, "frontend", "package-lock.json"))
	browserLock := readAcceptanceDocument(t, filepath.Join(root, "tests", "browser", "package-lock.json"))
	readme := readAcceptanceDocument(t, filepath.Join(root, "README.md"))
	contributing := readAcceptanceDocument(t, filepath.Join(root, "CONTRIBUTING.md"))
	taskfile := readAcceptanceDocument(t, filepath.Join(root, "Taskfile.yml"))
	nvmVersion := strings.TrimSpace(readAcceptanceDocument(t, filepath.Join(root, ".nvmrc")))

	for name, workflow := range map[string]string{
		"quality":  qualityWorkflow,
		"portable": portableWorkflow,
	} {
		assert.Contains(t, workflow, "NODE_VERSION: 26.8.1", name)
		assert.NotContains(t, workflow, "NODE_VERSION: 20.19.0", name)
	}

	for action, expectedCount := range map[string]int{
		"actions/checkout@v7.0.1":          4,
		"actions/setup-go@v7.0.0":          4,
		"actions/setup-node@v7.0.0":        2,
		"actions/upload-artifact@v7.0.1":   1,
		"actions/download-artifact@v8.0.1": 1,
	} {
		assert.Equal(t, expectedCount, strings.Count(qualityWorkflow+portableWorkflow, action), action)
	}

	for _, relativePath := range []string{
		"frontend/package.json",
		"frontend/client/package.json",
		"frontend/overseer/package.json",
	} {
		manifest := readAcceptanceDocument(t, filepath.Join(root, filepath.FromSlash(relativePath)))
		assert.Contains(t, manifest, `"node": "26.8.1"`, relativePath)
		assert.NotContains(t, manifest, `"node": ">=20.19.0"`, relativePath)
	}
	browserManifest := readAcceptanceDocument(t, filepath.Join(root, "tests", "browser", "package.json"))
	assert.Contains(t, browserManifest, `"node": ">=26.8.1"`)
	assert.NotContains(t, browserManifest, `"node": ">=20.19.0"`)

	assert.Equal(t, 3, strings.Count(frontendLock, `"node": "26.8.1"`))
	assert.NotContains(t, frontendLock, `"node": ">=20.19.0"`)
	assert.Equal(t, 1, strings.Count(browserLock, `"node": ">=26.8.1"`))
	assert.NotContains(t, browserLock, `"node": ">=20.19.0"`)
	assert.Contains(t, readme, "ровно Node.js 26.8.1 и npm")
	assert.NotContains(t, readme, "Node.js 20.19+ и npm;")
	assert.Equal(t, "26.8.1", nvmVersion)
	assert.Contains(t, contributing, "exactly Node.js 26.8.1 and npm")
	assert.Contains(t, contributing, "nvm use")
	assert.NotContains(t, contributing, "Node.js 20.19+ and npm")
	for _, required := range []string{
		`NODE: '{{default "node" .NODE}}'`,
		"NODE_VERSION: '26.8.1'",
		"node:check:",
		"task: node:check",
		`Run "nvm use" from the repository root.`,
	} {
		assert.Contains(t, taskfile, required)
	}
}

func TestTaskfileAlignsDarwinCGOQualityDeploymentTarget(t *testing.T) {
	t.Parallel()

	taskfile := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), "Taskfile.yml"))
	for _, required := range []string{
		"MACOS_MINIMUM_VERSION: '13.0'",
		`DARWIN_CGO_ENV: '{{if eq OS "darwin"}}env`,
		"MACOSX_DEPLOYMENT_TARGET={{.MACOS_MINIMUM_VERSION}}",
		"CGO_CFLAGS=-mmacosx-version-min={{.MACOS_MINIMUM_VERSION}}",
		`CGO_LDFLAGS="-mmacosx-version-min={{.MACOS_MINIMUM_VERSION}} -Wl,-no_warn_duplicate_libraries"`,
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

func TestPortableReleaseBrowserFixtureUsesPortableGoCache(t *testing.T) {
	t.Parallel()

	config := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), "tests", "browser", "playwright.config.mjs"))
	for _, required := range []string{
		"import { tmpdir } from 'node:os';",
		"import { join } from 'node:path';",
		"process.env.GOCACHE || join(tmpdir(), 'fallout-browser-fixture-cache')",
		"GOCACHE: browserFixtureGoCache",
	} {
		assert.Contains(t, config, required)
	}
	assert.NotContains(t, config, "/private/tmp")
	assert.NotContains(t, config, "GOCACHE=")
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

func TestPortableReleaseWorkflowPropagatesOneCanonicalVersionBeforeUpload(t *testing.T) {
	t.Parallel()

	workflow := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), ".github", "workflows", "wails-portable.yml"))
	preflightStart := strings.Index(workflow, "\n  preflight:")
	packageStart := strings.Index(workflow, "\n  package:")
	publishStart := strings.Index(workflow, "\n  publish:")
	require.GreaterOrEqual(t, preflightStart, 0)
	require.Greater(t, packageStart, preflightStart)
	require.Greater(t, publishStart, packageStart)

	preflight := workflow[preflightStart:packageStart]
	packageJob := workflow[packageStart:publishStart]
	for _, required := range []string{
		"outputs:",
		"version: ${{ steps.release-version.outputs.version }}",
		"id: release-version",
		`test -n "$VERSION"`,
		`echo "version=$VERSION" >> "$GITHUB_OUTPUT"`,
	} {
		assert.Contains(t, preflight, required)
	}
	assert.Equal(t, 1, strings.Count(preflight, `>> "$GITHUB_OUTPUT"`), "preflight must export VERSION exactly once")
	assert.Equal(t, 1, strings.Count(workflow, "validate-release-tag"), "only preflight may derive VERSION from the tag")

	assert.Contains(t, packageJob, "VERSION: ${{ needs.preflight.outputs.version }}")
	assert.Equal(t, 5, strings.Count(packageJob, "target: "), "the shared VERSION must cover every native matrix target")
	assert.Contains(t, packageJob, `task package GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} VERSION="$VERSION"`)
	assert.Contains(t, packageJob, `inspect-release-archive --target "${{ matrix.target }}" --archive "build/dist/${{ matrix.archive }}" --version "$VERSION"`)
	for _, forbidden := range []string{"GITHUB_REF_NAME", "github.ref_name", "validate-release-tag"} {
		assert.NotContains(t, packageJob, forbidden, "package targets must consume preflight VERSION without re-deriving it")
	}

	verification := strings.Index(packageJob, "inspect-release-archive")
	upload := strings.Index(packageJob, "actions/upload-artifact@v7.0.1")
	require.GreaterOrEqual(t, verification, 0)
	require.Greater(t, upload, verification, "packaged version verification must succeed before upload")
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

func TestPortablePublicationInventoryIsExactlyTheGovernedFiveArchives(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-portable.yml"))
	config := readAcceptanceDocument(t, filepath.Join(root, ".goreleaser.yaml"))
	want := []string{
		"Fallout-Terminal-windows-amd64.zip",
		"Fallout-Terminal-windows-arm64.zip",
		"Fallout-Terminal-linux-amd64.tar.gz",
		"Fallout-Terminal-linux-arm64.tar.gz",
		"Fallout-Terminal-darwin-arm64.zip",
	}

	workflowArchives := acceptanceValuesWithPrefix(workflow, "archive:")
	publishedGlobs := acceptanceValuesWithPrefix(config, "- glob: ./combined/")
	assert.ElementsMatch(t, want, workflowArchives)
	assert.ElementsMatch(t, want, publishedGlobs)
	assert.Len(t, workflowArchives, len(want), "the package matrix must contain no duplicate or extra archive")
	assert.Len(t, publishedGlobs, len(want), "GoReleaser must publish no duplicate or extra archive")

	assert.Equal(t, 1, strings.Count(workflow, "inspect-release-inventory --directory combined"))
	assert.Equal(t, 1, strings.Count(workflow, "go tool -modfile=tools/task/go.mod task release:publish"))
	assert.Equal(t, 1, strings.Count(workflow, "uses: actions/download-artifact@v8.0.1"))
	for _, forbidden := range []string{
		".sha256", "SHA256SUMS", "aggregate-index", "verification.json", ".dmg",
	} {
		assert.NotContains(t, strings.ToLower(workflow+"\n"+config), strings.ToLower(forbidden))
	}
}

func acceptanceValuesWithPrefix(document, prefix string) []string {
	values := make([]string, 0)
	for line := range strings.SplitSeq(document, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, prefix); ok {
			values = append(values, strings.TrimSpace(after))
		}
	}
	return values
}
