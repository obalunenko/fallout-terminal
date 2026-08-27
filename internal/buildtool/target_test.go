package buildtool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTargetAcceptsExactPortableMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		os   string
		arch string
	}{
		{name: "Windows ARM64", os: "windows", arch: "arm64"},
		{name: "Windows AMD64", os: "windows", arch: "amd64"},
		{name: "Linux ARM64", os: "linux", arch: "arm64"},
		{name: "Linux AMD64", os: "linux", arch: "amd64"},
		{name: "Darwin ARM64", os: "darwin", arch: "arm64"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target, err := ParseTarget(test.os, test.arch)
			require.NoError(t, err)
			assert.Equal(t, test.os, target.OS())
			assert.Equal(t, test.arch, target.Arch())
			assert.Equal(t, test.os+"/"+test.arch, target.String())
			assert.True(t, target.Portable())
		})
	}
}

func TestParseTargetRejectsAliasesCaseChangesAndUnsupportedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		os   string
		arch string
	}{
		{name: "missing operating system", os: "", arch: "amd64"},
		{name: "missing architecture", os: "windows", arch: ""},
		{name: "Windows alias", os: "win", arch: "amd64"},
		{name: "AMD64 alias", os: "windows", arch: "x64"},
		{name: "ARM64 alias", os: "linux", arch: "aarch64"},
		{name: "Linux AMD64 alias", os: "linux", arch: "x86_64"},
		{name: "operating system case change", os: "Windows", arch: "amd64"},
		{name: "architecture case change", os: "windows", arch: "AMD64"},
		{name: "all-uppercase value", os: "LINUX", arch: "ARM64"},
		{name: "operating system whitespace", os: " linux", arch: "arm64"},
		{name: "architecture whitespace", os: "linux", arch: "arm64 "},
		{name: "unsupported operating system", os: "freebsd", arch: "amd64"},
		{name: "unsupported architecture", os: "linux", arch: "386"},
		{name: "unsupported Darwin AMD64", os: "darwin", arch: "amd64"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseTarget(test.os, test.arch)
			require.Error(t, err)
			assert.ErrorContains(t, err, test.os+"/"+test.arch)
		})
	}
}

func TestValidateHostRequiresExactOSAndArchitecture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		targetOS   string
		targetArch string
		hostOS     string
		hostArch   string
		wantErr    bool
	}{
		{name: "matching Windows ARM64", targetOS: "windows", targetArch: "arm64", hostOS: "windows", hostArch: "arm64"},
		{name: "matching Windows AMD64", targetOS: "windows", targetArch: "amd64", hostOS: "windows", hostArch: "amd64"},
		{name: "matching Linux ARM64", targetOS: "linux", targetArch: "arm64", hostOS: "linux", hostArch: "arm64"},
		{name: "matching Linux AMD64", targetOS: "linux", targetArch: "amd64", hostOS: "linux", hostArch: "amd64"},
		{name: "matching Darwin ARM64", targetOS: "darwin", targetArch: "arm64", hostOS: "darwin", hostArch: "arm64"},
		{name: "operating system mismatch", targetOS: "windows", targetArch: "amd64", hostOS: "linux", hostArch: "amd64", wantErr: true},
		{name: "architecture mismatch", targetOS: "linux", targetArch: "arm64", hostOS: "linux", hostArch: "amd64", wantErr: true},
		{name: "operating system and architecture mismatch", targetOS: "windows", targetArch: "arm64", hostOS: "linux", hostArch: "amd64", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target, err := ParseTarget(test.targetOS, test.targetArch)
			require.NoError(t, err)

			host := NewHost(test.hostOS, test.hostArch)
			assert.Equal(t, test.hostOS, host.OS())
			assert.Equal(t, test.hostArch, host.Arch())
			assert.Equal(t, test.hostOS+"/"+test.hostArch, host.String())

			err = ValidateHost(target, host)
			if !test.wantErr {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorContains(t, err, target.String())
			assert.ErrorContains(t, err, host.String())
		})
	}
}

func TestDefaultTargetPreservesMacOSARM64Compatibility(t *testing.T) {
	t.Parallel()

	target := DefaultTarget()

	assert.Equal(t, "darwin", target.OS())
	assert.Equal(t, "arm64", target.Arch())
	assert.Equal(t, "darwin/arm64", target.String())
	assert.True(t, target.Portable())
	require.NoError(t, ValidateHost(target, NewHost("darwin", "arm64")))
}

func TestTargetArchiveContractIsStableAndCollisionFree(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos       string
		goarch     string
		format     ArchiveFormat
		archive    string
		executable string
	}{
		{goos: "windows", goarch: "amd64", format: ArchiveFormatZIP, archive: "Fallout-Terminal-windows-amd64.zip", executable: "Fallout Terminal.exe"},
		{goos: "windows", goarch: "arm64", format: ArchiveFormatZIP, archive: "Fallout-Terminal-windows-arm64.zip", executable: "Fallout Terminal.exe"},
		{goos: "linux", goarch: "amd64", format: ArchiveFormatTarGzip, archive: "Fallout-Terminal-linux-amd64.tar.gz", executable: "Fallout Terminal"},
		{goos: "linux", goarch: "arm64", format: ArchiveFormatTarGzip, archive: "Fallout-Terminal-linux-arm64.tar.gz", executable: "Fallout Terminal"},
		{goos: "darwin", goarch: "arm64", format: ArchiveFormatZIP, archive: "Fallout-Terminal-darwin-arm64.zip", executable: "Fallout Terminal.app/Contents/MacOS/Fallout Terminal"},
	}

	archives := make(map[string]string, len(tests))
	for _, test := range tests {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			target, err := ParseTarget(test.goos, test.goarch)
			require.NoError(t, err)
			assert.Equal(t, test.format, target.ArchiveFormat())
			assert.Equal(t, test.archive, target.ArchiveName())
			assert.Equal(t, test.executable, target.ExecutablePath())
			assert.NotEmpty(t, target.RequiredResourcePaths())
			require.NoError(t, ValidateHost(target, NewHost(test.goos, test.goarch)))
		})

		require.NotContains(t, archives, test.archive)
		archives[test.archive] = test.goos + "/" + test.goarch
	}
	assert.Len(t, archives, 5)
}
