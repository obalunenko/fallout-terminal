package platform

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortableReleaseWorkflowUsesExplicitNativeRunnerMatrix(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-portable.yml"))

	for _, required := range []string{
		"fail-fast: false",
		"runs-on: ${{ matrix.runner }}",
		"windows-2025",
		"windows-11-arm",
		"ubuntu-24.04",
		"ubuntu-24.04-arm",
		"windows/amd64",
		"windows/arm64",
		"linux/amd64",
		"linux/arm64",
	} {
		assert.Contains(t, workflow, required)
	}
	for _, runner := range []string{"windows-2025", "windows-11-arm", "ubuntu-24.04", "ubuntu-24.04-arm"} {
		pattern := regexp.MustCompile(`(?m)^[[:space:]]+runner: ` + regexp.QuoteMeta(runner) + `$`)
		assert.Len(t, pattern.FindAllString(workflow, -1), 1, "%s must identify exactly one native target", runner)
	}
	assert.NotContains(t, workflow, "runs-on: ubuntu-latest")
	assert.NotContains(t, workflow, "runs-on: windows-latest")
}

func TestPortableReleaseUploadsOnlyAfterNativeVerification(t *testing.T) {
	t.Parallel()

	workflow := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), ".github", "workflows", "wails-portable.yml"))
	packageJob := strings.Index(workflow, "  package:")
	aggregateJob := strings.Index(workflow, "  aggregate:")
	require.NotEqual(t, -1, packageJob)
	require.Greater(t, aggregateJob, packageJob)
	packageSource := workflow[packageJob:aggregateJob]

	for _, required := range []string{
		"go tool -modfile=tools/task/go.mod task package",
		"scripts/verify-windows-package.ps1",
		"scripts/verify-linux-package.sh",
		"actions/upload-artifact@v4",
		"if: ${{ success() }}",
	} {
		assert.Contains(t, packageSource, required)
	}
	upload := strings.Index(packageSource, "actions/upload-artifact@v4")
	require.NotEqual(t, -1, upload)
	assert.Less(t, strings.Index(packageSource, "scripts/verify-windows-package.ps1"), upload)
	assert.Less(t, strings.Index(packageSource, "scripts/verify-linux-package.sh"), upload)
	assert.NotContains(t, packageSource, "make ")
	assert.NotContains(t, packageSource, "\n          task package")
}

func TestPortableNativeSmokeCoversSessionPlayerURLAndCredentialParity(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	windows := readAcceptanceDocument(t, filepath.Join(root, "scripts", "verify-windows-package.ps1"))
	linux := readAcceptanceDocument(t, filepath.Join(root, "scripts", "verify-linux-package.sh"))
	workflow := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-portable.yml"))
	secretCheck := readAcceptanceDocument(t, filepath.Join(root, "scripts", "secret-leak-check.sh"))

	for _, script := range []string{windows, linux} {
		for _, required := range []string{
			"go run ./cmd/native-credential-smoke",
			"session.save",
			"application-reopen",
			"control-accepted",
			"synchronized",
			"http://127.0.0.1:",
		} {
			assert.Contains(t, script, required)
		}
	}
	assert.Contains(t, linux, "native-ui-smoke.py")
	assert.Contains(t, linux, "--scan-root")
	assert.Contains(t, windows, "Assert-NoCredentialCanaryLeak")
	assert.Contains(t, workflow, "python3-pyatspi")
	assert.Contains(t, workflow, "--expect-unavailable")
	assert.Contains(t, secretCheck, "--scan-root")
}

func TestPortableReleaseAggregateAlwaysGatesCompleteVerifiedMatrix(t *testing.T) {
	t.Parallel()

	workflow := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), ".github", "workflows", "wails-portable.yml"))
	aggregateJob := strings.Index(workflow, "  aggregate:")
	require.NotEqual(t, -1, aggregateJob)
	aggregateSource := workflow[aggregateJob:]
	for _, required := range []string{
		"if: ${{ always() }}",
		"needs: [package]",
		"actions/download-artifact@v4",
		"actions/upload-artifact@v4",
		"fallout-terminal-portable",
		"SOURCE_SHA",
	} {
		assert.Contains(t, aggregateSource, required)
	}
	assert.Contains(t, aggregateSource, "needs.package.result")
	assert.Contains(t, aggregateSource, "success")
	assert.Contains(t, aggregateSource, "windows-amd64")
	assert.Contains(t, aggregateSource, "windows-arm64")
	assert.Contains(t, aggregateSource, "linux-amd64")
	assert.Contains(t, aggregateSource, "linux-arm64")
}

func TestPortableReleasePublishesCompleteFiveTargetInventoryForVersionTags(t *testing.T) {
	t.Parallel()

	workflow := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), ".github", "workflows", "wails-portable.yml"))
	publishJob := strings.Index(workflow, "  publish:")
	require.NotEqual(t, -1, publishJob)
	publishSource := workflow[publishJob:]

	for _, required := range []string{
		"tags:",
		"- 'v*'",
		"validate_release:",
		"uses: ./.github/workflows/wails-macos.yml",
		"needs: [aggregate, macos-trust]",
		"fallout-terminal-release-candidate",
		"github.ref_type == 'tag'",
		"contents: write",
		"packages: write",
		"go tool -modfile=tools/oras/go.mod oras push",
		"oras manifest delete --force",
		"ghcr.io/${package_repository}:${GITHUB_REF_NAME}",
		"go tool -modfile=tools/goreleaser/go.mod goreleaser release --clean --config .goreleaser.yaml",
		"Roll back incomplete cross-destination publication",
		"Fallout-Terminal-arm64.dmg",
		"Fallout-Terminal-arm64.dmg.sha256",
		"combined/*",
	} {
		assert.Contains(t, workflow, required)
	}

	assert.Contains(t, publishSource, "fallout-terminal-release-candidate")
	assert.Contains(t, publishSource, "application/vnd.fallout-terminal.release.v1")
	assert.Contains(t, publishSource, "aggregate-index.json")
	assert.NotContains(t, publishSource, "gh release")
	assert.NotContains(t, publishSource, "make ")
}

func TestGoReleaserV2PublishesOnlyPreverifiedPortableFiles(t *testing.T) {
	t.Parallel()

	config := readAcceptanceDocument(t, filepath.Join(repositoryRoot(t), ".goreleaser.yaml"))
	for _, required := range []string{
		"version: 2",
		"skip: true",
		"disable: true",
		"prerelease: auto",
		"draft: true",
		"replace_existing_artifacts: true",
		"./combined/aggregate-index.json",
		"./combined/Fallout-Terminal-arm64.dmg",
		"./combined/Fallout-Terminal-arm64.dmg.sha256",
		"./combined/Fallout-Terminal-windows-amd64.zip",
		"./combined/Fallout-Terminal-windows-arm64.zip",
		"./combined/Fallout-Terminal-linux-amd64.tar.gz",
		"./combined/Fallout-Terminal-linux-arm64.tar.gz",
	} {
		assert.Contains(t, config, required)
	}
	assert.Equal(t, 11, strings.Count(config, "- glob: ./combined/"))
}

func TestPortableReleaseRemainsSeparateFromMacOSTrustWorkflow(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	portable := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-portable.yml"))
	macOS := readAcceptanceDocument(t, filepath.Join(root, ".github", "workflows", "wails-macos.yml"))

	assert.Contains(t, macOS, "name: Wails macOS")
	assert.Contains(t, macOS, "runs-on: macos-15")
	assert.Contains(t, macOS, "workflow_call:")
	assert.Contains(t, macOS, "go tool -modfile=tools/task/go.mod task release")
	assert.Contains(t, macOS, "Fallout-Terminal-arm64.dmg.sha256")
	assert.Contains(t, macOS, "darwin-verification.json")
	assert.NotContains(t, macOS, "windows-11-arm")
	assert.NotContains(t, macOS, "ubuntu-24.04-arm")
	assert.NotContains(t, portable, "notarytool")
	assert.NotContains(t, portable, "Developer ID Application")
}
