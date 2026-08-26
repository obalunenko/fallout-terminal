package buildtool

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLocalPackageAllHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		host      Host
		wantError string
	}{
		{name: "darwin arm64", host: NewHost(goosDarwin, goarchARM64)},
		{name: "darwin amd64", host: NewHost(goosDarwin, goarchAMD64), wantError: "requires the supported darwin/arm64 host"},
		{name: "linux arm64", host: NewHost(goosLinux, goarchARM64), wantError: "requires the supported darwin/arm64 host"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateLocalPackageAllHost(test.host)
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func TestCopyDarwinBundle(t *testing.T) {
	t.Parallel()

	t.Run("complete bundle", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		source := createDarwinBundleFixture(t, filepath.Join(root, "source", applicationName+".app"))
		destinationParent := filepath.Join(root, "destination", "darwin-arm64")
		require.NoError(t, os.MkdirAll(destinationParent, 0o755))
		destination := filepath.Join(destinationParent, applicationName+".app")

		require.NoError(t, copyDarwinBundle(t.Context(), source, destination))
		require.NoError(t, verifyDarwinBundleInventory(destination))
		contents, err := os.ReadFile(filepath.Join(destination, "Contents", "Resources", "sessions", "demo.json"))
		require.NoError(t, err)
		assert.Equal(t, "demo\n", string(contents))
		executableInfo, err := os.Stat(filepath.Join(destination, "Contents", "MacOS", applicationName))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), executableInfo.Mode().Perm())
	})

	t.Run("symlink entry", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation may require elevated Windows privileges")
		}
		root := t.TempDir()
		source := createDarwinBundleFixture(t, filepath.Join(root, "source", applicationName+".app"))
		require.NoError(t, os.Symlink(
			filepath.Join(source, "Contents", "Info.plist"),
			filepath.Join(source, "Contents", "Resources", "linked-info.plist"),
		))
		destinationParent := filepath.Join(root, "destination", "darwin-arm64")
		require.NoError(t, os.MkdirAll(destinationParent, 0o755))

		err := copyDarwinBundle(
			t.Context(),
			source,
			filepath.Join(destinationParent, applicationName+".app"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not a regular file or directory")
	})

	t.Run("cancelled context", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		source := createDarwinBundleFixture(t, filepath.Join(root, "source", applicationName+".app"))
		destinationParent := filepath.Join(root, "destination", "darwin-arm64")
		require.NoError(t, os.MkdirAll(destinationParent, 0o755))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := copyDarwinBundle(ctx, source, filepath.Join(destinationParent, applicationName+".app"))
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestVerifyDarwinBundleInventory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		prepare   func(*testing.T, string)
		wantError string
	}{
		{name: "complete bundle"},
		{
			name: "missing signature resources",
			prepare: func(t *testing.T, bundle string) {
				require.NoError(t, os.Remove(filepath.Join(bundle, "Contents", "_CodeSignature", "CodeResources")))
			},
			wantError: "CodeResources",
		},
		{
			name: "non executable binary",
			prepare: func(t *testing.T, bundle string) {
				require.NoError(t, os.Chmod(filepath.Join(bundle, "Contents", "MacOS", applicationName), 0o644))
			},
			wantError: "is not executable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bundle := createDarwinBundleFixture(t, filepath.Join(t.TempDir(), applicationName+".app"))
			if test.prepare != nil {
				test.prepare(t, bundle)
			}
			err := verifyDarwinBundleInventory(bundle)
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func createDarwinBundleFixture(t *testing.T, bundle string) string {
	t.Helper()
	files := map[string]struct {
		contents string
		mode     os.FileMode
	}{
		filepath.Join("Contents", "Info.plist"):                                 {contents: "plist\n", mode: 0o644},
		filepath.Join("Contents", "MacOS", applicationName):                     {contents: "mach-o\n", mode: 0o755},
		filepath.Join("Contents", "Resources", "icon.icns"):                     {contents: "icon\n", mode: 0o644},
		filepath.Join("Contents", "Resources", "THIRD_PARTY_NOTICES.md"):        {contents: "notices\n", mode: 0o444},
		filepath.Join("Contents", "Resources", "sessions", "demo.json"):         {contents: "demo\n", mode: 0o444},
		filepath.Join("Contents", "Resources", "sessions", "demo-players.json"): {contents: "players\n", mode: 0o444},
		filepath.Join("Contents", "_CodeSignature", "CodeResources"):            {contents: "signature\n", mode: 0o644},
	}
	for relative, file := range files {
		path := filepath.Join(bundle, relative)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(file.contents), file.mode))
		require.NoError(t, os.Chmod(path, file.mode))
	}
	return bundle
}

func TestValidateDockerAggregateOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		output      func(string) string
		prepare     func(*testing.T, string)
		wantError   string
		skipWindows bool
	}{
		{
			name:   "absent output",
			output: func(root string) string { return filepath.Join(root, "build", "portable") },
		},
		{
			name:   "existing repository default",
			output: func(root string) string { return filepath.Join(root, "build", "dist") },
			prepare: func(t *testing.T, output string) {
				require.NoError(t, os.MkdirAll(output, 0o755))
			},
		},
		{
			name:   "recognized custom aggregate",
			output: func(root string) string { return filepath.Join(root, "build", "portable") },
			prepare: func(t *testing.T, output string) {
				require.NoError(t, os.MkdirAll(output, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(output, localAggregateIndexName), []byte("{}\n"), 0o644))
			},
		},
		{
			name:      "unrecognized custom directory",
			output:    func(root string) string { return filepath.Join(root, "build", "portable") },
			prepare:   func(t *testing.T, output string) { require.NoError(t, os.MkdirAll(output, 0o755)) },
			wantError: "refusing to replace unrecognized",
		},
		{
			name:   "regular file",
			output: func(root string) string { return filepath.Join(root, "build", "portable") },
			prepare: func(t *testing.T, output string) {
				require.NoError(t, os.MkdirAll(filepath.Dir(output), 0o755))
				require.NoError(t, os.WriteFile(output, []byte("owned by user"), 0o644))
			},
			wantError: "must be a directory",
		},
		{
			name:        "symlink",
			output:      func(root string) string { return filepath.Join(root, "build", "portable") },
			skipWindows: true,
			prepare: func(t *testing.T, output string) {
				target := filepath.Join(filepath.Dir(filepath.Dir(output)), "target")
				require.NoError(t, os.MkdirAll(target, 0o755))
				require.NoError(t, os.MkdirAll(filepath.Dir(output), 0o755))
				require.NoError(t, os.Symlink(target, output))
			},
			wantError: "must not be a symlink",
		},
		{
			name:      "repository root",
			output:    func(root string) string { return root },
			wantError: "must not contain the repository root",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.skipWindows && runtime.GOOS == "windows" {
				t.Skip("symlink creation may require elevated Windows privileges")
			}
			root := t.TempDir()
			output := test.output(root)
			if test.prepare != nil {
				test.prepare(t, output)
			}

			err := validateDockerAggregateOutput(root, output)
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func TestPublishDockerAggregateOutput(t *testing.T) {
	t.Parallel()

	t.Run("first publication", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		workRoot := filepath.Join(parent, "work")
		publish := filepath.Join(workRoot, "publish")
		output := filepath.Join(parent, "dist")
		require.NoError(t, os.MkdirAll(publish, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(publish, "new"), []byte("new"), 0o644))

		require.NoError(t, publishDockerAggregateOutput(publish, output, workRoot))
		contents, err := os.ReadFile(filepath.Join(output, "new"))
		require.NoError(t, err)
		assert.Equal(t, "new", string(contents))
	})

	t.Run("replacement", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		workRoot := filepath.Join(parent, "work")
		publish := filepath.Join(workRoot, "publish")
		output := filepath.Join(parent, "dist")
		require.NoError(t, os.MkdirAll(publish, 0o755))
		require.NoError(t, os.MkdirAll(output, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(output, "old"), []byte("old"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(publish, "new"), []byte("new"), 0o644))

		require.NoError(t, publishDockerAggregateOutput(publish, output, workRoot))
		assert.NoFileExists(t, filepath.Join(output, "old"))
		contents, err := os.ReadFile(filepath.Join(output, "new"))
		require.NoError(t, err)
		assert.Equal(t, "new", string(contents))
		assert.FileExists(t, filepath.Join(workRoot, "previous-output", "old"))
	})

	t.Run("rollback", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		workRoot := filepath.Join(parent, "work")
		publish := filepath.Join(workRoot, "missing-publish")
		output := filepath.Join(parent, "dist")
		require.NoError(t, os.MkdirAll(workRoot, 0o755))
		require.NoError(t, os.MkdirAll(output, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(output, "old"), []byte("old"), 0o644))

		err := publishDockerAggregateOutput(publish, output, workRoot)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "publish replacement")
		contents, readErr := os.ReadFile(filepath.Join(output, "old"))
		require.NoError(t, readErr)
		assert.Equal(t, "old", string(contents))
		assert.NoDirExists(t, filepath.Join(workRoot, "previous-output"))
	})
}

func TestVerifyDockerExecutablePayloadMatchesEveryPortableArchive(t *testing.T) {
	t.Parallel()

	for _, target := range portableTestTargets(t) {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()

			fixture := newVerificationFixture(t, target)
			fixture.write(t)
			payload := writeDockerPayloadFixture(t, fixture)

			require.NoError(t, verifyDockerExecutablePayload(
				t.Context(),
				payload,
				fixture.archivePath,
				target,
			))
		})
	}
}

func TestVerifyDockerExecutablePayloadRejectsInventoryAndHashDrift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*testing.T, string, Target)
		wantError string
	}{
		{
			name: "missing resource",
			mutate: func(t *testing.T, payload string, _ Target) {
				require.NoError(t, os.Remove(filepath.Join(payload, "resources", "appicon.png")))
			},
			wantError: "inventory mismatch",
		},
		{
			name: "extra file",
			mutate: func(t *testing.T, payload string, _ Target) {
				require.NoError(t, os.WriteFile(filepath.Join(payload, "unexpected.txt"), []byte("unexpected"), 0o444))
			},
			wantError: "inventory mismatch",
		},
		{
			name: "content differs from archive",
			mutate: func(t *testing.T, payload string, target Target) {
				require.NoError(t, os.WriteFile(filepath.Join(payload, target.ExecutableName()), []byte("corrupt"), 0o755))
			},
			wantError: "does not match its verified archive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target := mustParseTarget(t, goosLinux, goarchAMD64)
			fixture := newVerificationFixture(t, target)
			fixture.write(t)
			payload := writeDockerPayloadFixture(t, fixture)
			test.mutate(t, payload, target)

			err := verifyDockerExecutablePayload(t.Context(), payload, fixture.archivePath, target)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func TestCollectDockerTargetRequiresCompleteQuarantinedOutput(t *testing.T) {
	t.Parallel()

	t.Run("complete output", func(t *testing.T) {
		t.Parallel()

		target := mustParseTarget(t, goosWindows, goarchARM64)
		fixture := newVerificationFixture(t, target)
		fixture.write(t)
		root := t.TempDir()
		source, publish, executableRoot := writeDockerExportFixture(t, root, fixture)

		payload, err := collectDockerTarget(source, publish, executableRoot, target)
		require.NoError(t, err)
		assert.DirExists(t, payload)
		assert.FileExists(t, filepath.Join(publish, target.ArchiveName()))
		assert.FileExists(t, filepath.Join(publish, target.ArchiveName()+".sha256"))
	})

	t.Run("malformed output publishes nothing", func(t *testing.T) {
		t.Parallel()

		target := mustParseTarget(t, goosLinux, goarchARM64)
		root := t.TempDir()
		source := filepath.Join(root, "source")
		publish := filepath.Join(root, "publish")
		executableRoot := filepath.Join(root, "bin")
		for _, directory := range []string{source, publish, executableRoot} {
			require.NoError(t, os.MkdirAll(directory, 0o755))
		}
		require.NoError(t, os.WriteFile(filepath.Join(source, "partial.zip"), []byte("partial"), 0o644))

		_, err := collectDockerTarget(source, publish, executableRoot, target)
		require.Error(t, err)
		entries, readErr := os.ReadDir(publish)
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})
}

func TestDockerPrerequisiteDiagnosticsPreserveCauseAndRecovery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		run       dockerInfoRunner
		wantParts []string
	}{
		{
			name: "missing Docker executable",
			run: func(context.Context, string) (string, error) {
				return "", exec.ErrNotFound
			},
			wantParts: []string{"docker is required", "install Docker", "start its daemon"},
		},
		{
			name: "stopped Docker daemon",
			run: func(context.Context, string) (string, error) {
				return "Cannot connect to the Docker daemon at unix:///var/run/docker.sock", errors.New("exit status 1")
			},
			wantParts: []string{"Cannot connect to the Docker daemon", "start Docker", "docker info"},
		},
		{
			name:      "missing prerequisite probe",
			wantParts: []string{"probe is unavailable", "install Docker", "start its daemon"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := requireDockerWith(t.Context(), t.TempDir(), test.run)
			require.Error(t, err)
			for _, part := range test.wantParts {
				assert.Contains(t, err.Error(), part)
			}
		})
	}

	t.Run("unsupported build platform", func(t *testing.T) {
		t.Parallel()
		target := mustParseTarget(t, goosLinux, goarchARM64)
		err := dockerBuildFailure(target, "no matching manifest for linux/arm64", errors.New("exit status 1"))
		require.Error(t, err)
		for _, part := range []string{"linux/arm64", "no matching manifest", "BuildKit", "binfmt", "exit status 1"} {
			assert.Contains(t, err.Error(), part)
		}
		assert.NotContains(t, err.Error(), "\n")
	})
}

func writeDockerPayloadFixture(t *testing.T, fixture *verificationFixture) string {
	t.Helper()
	payload := filepath.Join(t.TempDir(), fixture.target.OS()+"-"+fixture.target.Arch())
	for relative, file := range fixture.files {
		path := filepath.Join(payload, filepath.FromSlash(relative))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, file.contents, file.mode))
		require.NoError(t, os.Chmod(path, file.mode))
	}
	return payload
}

func writeDockerExportFixture(
	t *testing.T,
	root string,
	fixture *verificationFixture,
) (source string, publish string, executableRoot string) {
	t.Helper()
	targetName := fixture.target.OS() + "-" + fixture.target.Arch()
	source = filepath.Join(root, "source")
	publish = filepath.Join(root, "publish")
	executableRoot = filepath.Join(root, "quarantined-bin")
	for _, directory := range []string{
		filepath.Join(source, "dist"),
		filepath.Join(source, "bin", targetName),
		publish,
		executableRoot,
	} {
		require.NoError(t, os.MkdirAll(directory, 0o755))
	}

	for _, sourcePath := range []string{fixture.archivePath, fixture.checksumPath} {
		contents, err := os.ReadFile(sourcePath)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(source, "dist", filepath.Base(sourcePath)), contents, 0o644))
	}
	payload := writeDockerPayloadFixture(t, fixture)
	entries, err := os.ReadDir(payload)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NoError(t, os.Rename(
			filepath.Join(payload, entry.Name()),
			filepath.Join(source, "bin", targetName, entry.Name()),
		))
	}
	return source, publish, executableRoot
}
