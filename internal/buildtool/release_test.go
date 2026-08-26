package buildtool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyReleaseCandidateAcceptsExactFiveTargetInventory(t *testing.T) {
	t.Parallel()

	directory := writeReleaseCandidateFixture(t)
	files, err := verifyReleaseCandidate(t.Context(), directory, verificationSourceRevision)
	require.NoError(t, err)
	assert.Equal(t, releaseCandidateFileNames(), files)
}

func TestVerifyReleaseCandidateRejectsUnexpectedAndCorruptFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*testing.T, string)
		wantError string
	}{
		{
			name: "unexpected file",
			mutate: func(t *testing.T, directory string) {
				require.NoError(t, os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("unexpected\n"), 0o644))
			},
			wantError: "inventory mismatch",
		},
		{
			name: "corrupt Darwin archive",
			mutate: func(t *testing.T, directory string) {
				require.NoError(t, os.WriteFile(
					filepath.Join(directory, darwinReleaseArchiveName),
					[]byte("corrupt\n"),
					0o644,
				))
			},
			wantError: "archive checksum mismatch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := writeReleaseCandidateFixture(t)
			test.mutate(t, directory)

			_, err := verifyReleaseCandidate(t.Context(), directory, verificationSourceRevision)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func writeReleaseCandidateFixture(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	records := make([]AggregateTargetRecord, 0, len(portableMatrixTargets()))
	for _, target := range portableMatrixTargets() {
		fixture := newVerificationFixture(t, target)
		fixture.write(t)
		copyReleaseCandidateFixtureFile(t, fixture.archivePath, filepath.Join(directory, target.ArchiveName()))
		copyReleaseCandidateFixtureFile(t, fixture.checksumPath, filepath.Join(directory, target.ArchiveName()+".sha256"))
		records = append(records, AggregateTargetRecord{
			Target:      target,
			SourceSHA:   verificationSourceRevision,
			Status:      AggregateTargetEligible,
			ArchiveName: target.ArchiveName(),
			Checksum:    fixture.archiveDigest,
		})
	}
	require.NoError(t, writeLocalAggregateIndex(
		directory,
		"local-release-test",
		verificationSourceRevision,
		records,
	))

	dmgPath := filepath.Join(directory, darwinReleaseArchiveName)
	require.NoError(t, os.WriteFile(dmgPath, []byte("unsigned dmg fixture\n"), 0o644))
	digest, err := hashFile(t.Context(), dmgPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		dmgPath+".sha256",
		[]byte(digest+"  "+darwinReleaseArchiveName+"\n"),
		0o644,
	))
	return directory
}

func copyReleaseCandidateFixtureFile(t *testing.T, source, destination string) {
	t.Helper()
	contents, err := os.ReadFile(source)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(destination, contents, 0o644))
}
