package buildtool

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectReleaseArchiveVersionAcceptsExactTargetEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		target   Target
		expected ReleaseVersion
		evidence NativeVersionEvidence
		plist    []byte
	}{
		{
			name:     "Linux stable",
			target:   mustParseTarget(t, goosLinux, goarchAMD64),
			expected: releaseVersionFixture("2.0.0", "2.0.0", "2.0.0.0"),
			evidence: NativeVersionEvidence{ExecutableOutput: "2.0.0\n"},
		},
		{
			name:     "Darwin prerelease",
			target:   mustParseTarget(t, goosDarwin, goarchARM64),
			expected: releaseVersionFixture("2.0.0-rc.1", "2.0.0", "2.0.0.0"),
			evidence: NativeVersionEvidence{ExecutableOutput: "2.0.0-rc.1\n"},
			plist:    darwinVersionPlist("2.0.0-rc.1", "2.0.0"),
		},
		{
			name:     "Windows prerelease",
			target:   mustParseTarget(t, goosWindows, goarchAMD64),
			expected: releaseVersionFixture("2.0.0-rc.1", "2.0.0", "2.0.0.0"),
			evidence: NativeVersionEvidence{
				ExecutableOutput:    "2.0.0-rc.1\n",
				FileVersion:         "2.0.0-rc.1",
				ProductVersion:      "2.0.0-rc.1",
				FixedFileVersion:    "2.0.0.0",
				FixedProductVersion: "2.0.0.0",
				AssemblyVersion:     "2.0.0.0",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			archivePath := writeVersionArchiveFixture(t, test.target, test.plist)
			probe := func(ctx context.Context, target Target, executablePath string, arguments []string) (NativeVersionEvidence, error) {
				require.NoError(t, ctx.Err())
				assert.Equal(t, test.target, target)
				assert.Equal(t, test.target.ExecutableName(), path.Base(strings.ReplaceAll(executablePath, `\`, "/")))
				assert.Equal(t, []string{"--version"}, arguments)
				return test.evidence, nil
			}

			require.NoError(t, inspectReleaseArchiveVersion(t.Context(), test.target, archivePath, test.expected, probe))
		})
	}
}

func TestInspectReleaseArchiveVersionRejectsInexactExecutableReport(t *testing.T) {
	t.Parallel()

	target := mustParseTarget(t, goosLinux, goarchAMD64)
	expected := releaseVersionFixture("2.0.0", "2.0.0", "2.0.0.0")
	tests := []struct {
		name     string
		output   string
		stderr   string
		probeErr error
	}{
		{name: "missing", output: ""},
		{name: "development", output: "development\n"},
		{name: "malformed", output: "v2.0.0\n"},
		{name: "mismatch", output: "2.0.1\n"},
		{name: "missing newline", output: "2.0.0"},
		{name: "additional output", output: "2.0.0\nextra\n"},
		{name: "stderr output", output: "2.0.0\n", stderr: "warning\n"},
		{name: "execution failure", probeErr: fmt.Errorf("exit status 2")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			archivePath := writeVersionArchiveFixture(t, target, nil)
			probe := func(_ context.Context, _ Target, _ string, arguments []string) (NativeVersionEvidence, error) {
				assert.Equal(t, []string{"--version"}, arguments)
				return NativeVersionEvidence{
					ExecutableOutput: test.output,
					ExecutableStderr: test.stderr,
				}, test.probeErr
			}

			require.Error(t, inspectReleaseArchiveVersion(t.Context(), target, archivePath, expected, probe))
		})
	}
}

func TestInspectReleaseArchiveVersionRejectsDarwinMetadataMismatch(t *testing.T) {
	t.Parallel()

	target := mustParseTarget(t, goosDarwin, goarchARM64)
	expected := releaseVersionFixture("2.0.0-rc.1", "2.0.0", "2.0.0.0")
	valid := string(darwinVersionPlist("2.0.0-rc.1", "2.0.0"))
	tests := []struct {
		name  string
		plist string
	}{
		{
			name:  "missing human-readable version",
			plist: strings.Replace(valid, "<key>CFBundleShortVersionString</key>\n<string>2.0.0-rc.1</string>\n", "", 1),
		},
		{name: "development human-readable version", plist: string(darwinVersionPlist("development", "2.0.0"))},
		{name: "malformed human-readable version", plist: string(darwinVersionPlist("v2.0.0-rc.1", "2.0.0"))},
		{name: "mismatched human-readable version", plist: string(darwinVersionPlist("2.0.0", "2.0.0"))},
		{name: "missing numeric version", plist: strings.Replace(valid, "<key>CFBundleVersion</key>\n<string>2.0.0</string>\n", "", 1)},
		{name: "mismatched numeric version", plist: string(darwinVersionPlist("2.0.0-rc.1", "2.0.1"))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			archivePath := writeVersionArchiveFixture(t, target, []byte(test.plist))
			probe := func(_ context.Context, _ Target, _ string, arguments []string) (NativeVersionEvidence, error) {
				assert.Equal(t, []string{"--version"}, arguments)
				return NativeVersionEvidence{ExecutableOutput: "2.0.0-rc.1\n"}, nil
			}

			require.Error(t, inspectReleaseArchiveVersion(t.Context(), target, archivePath, expected, probe))
		})
	}
}

func TestInspectReleaseArchiveVersionRejectsWindowsMetadataMismatch(t *testing.T) {
	t.Parallel()

	target := mustParseTarget(t, goosWindows, goarchAMD64)
	expected := releaseVersionFixture("2.0.0-rc.1", "2.0.0", "2.0.0.0")
	valid := NativeVersionEvidence{
		ExecutableOutput:    "2.0.0-rc.1\n",
		FileVersion:         "2.0.0-rc.1",
		ProductVersion:      "2.0.0-rc.1",
		FixedFileVersion:    "2.0.0.0",
		FixedProductVersion: "2.0.0.0",
		AssemblyVersion:     "2.0.0.0",
	}
	tests := []struct {
		name   string
		mutate func(*NativeVersionEvidence)
	}{
		{name: "missing file version", mutate: func(evidence *NativeVersionEvidence) { evidence.FileVersion = "" }},
		{name: "development product version", mutate: func(evidence *NativeVersionEvidence) { evidence.ProductVersion = "development" }},
		{name: "malformed file version", mutate: func(evidence *NativeVersionEvidence) { evidence.FileVersion = "v2.0.0-rc.1" }},
		{name: "mismatched product version", mutate: func(evidence *NativeVersionEvidence) { evidence.ProductVersion = "2.0.0" }},
		{name: "mismatched fixed file version", mutate: func(evidence *NativeVersionEvidence) { evidence.FixedFileVersion = "2.0.1.0" }},
		{name: "missing fixed product version", mutate: func(evidence *NativeVersionEvidence) { evidence.FixedProductVersion = "" }},
		{name: "mismatched assembly version", mutate: func(evidence *NativeVersionEvidence) { evidence.AssemblyVersion = "1.0.0.0" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			archivePath := writeVersionArchiveFixture(t, target, nil)
			evidence := valid
			test.mutate(&evidence)
			probe := func(_ context.Context, _ Target, _ string, arguments []string) (NativeVersionEvidence, error) {
				assert.Equal(t, []string{"--version"}, arguments)
				return evidence, nil
			}

			require.Error(t, inspectReleaseArchiveVersion(t.Context(), target, archivePath, expected, probe))
		})
	}
}

func releaseVersionFixture(canonical, numericCore, numericFourPart string) ReleaseVersion {
	return ReleaseVersion{
		Canonical:       canonical,
		NumericCore:     numericCore,
		NumericFourPart: numericFourPart,
		Prerelease:      strings.Contains(canonical, "-"),
		IsRelease:       true,
	}
}

func writeVersionArchiveFixture(t *testing.T, target Target, plist []byte) string {
	t.Helper()

	root := t.TempDir()
	archivePath := filepath.Join(root, target.ArchiveName())
	entries := releaseArchiveEntries(target)
	entries[path.Join(applicationName, target.ExecutablePath())] = []byte("native executable fixture")
	if target.OS() == goosDarwin {
		require.NotEmpty(t, plist)
		entries[path.Join(applicationName, "Fallout Terminal.app/Contents/Info.plist")] = plist
	}
	writeReleaseArchiveFixture(t, archivePath, target.ArchiveFormat(), entries)
	return archivePath
}

func darwinVersionPlist(humanReadable, numeric string) []byte {
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
<key>CFBundleShortVersionString</key>
<string>%s</string>
<key>CFBundleVersion</key>
<string>%s</string>
</dict>
</plist>
`, humanReadable, numeric)
}
