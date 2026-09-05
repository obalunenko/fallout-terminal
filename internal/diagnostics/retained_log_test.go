package diagnostics

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingRetainedFile struct {
	err error
}

func (file *failingRetainedFile) Write([]byte) (int, error) { return 0, file.err }
func (*failingRetainedFile) Close() error                   { return nil }

func TestRunLogMirrorsRotatesAndRetainsOwnedFiles(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "logs")
	unknown := filepath.Join(directory, "keep-me.txt")
	require.NoError(t, os.MkdirAll(directory, 0o700))
	require.NoError(t, os.WriteFile(unknown, []byte("owner data"), 0o600))
	var fallback bytes.Buffer
	clock := time.Date(2026, 9, 3, 10, 11, 12, 0, time.UTC)
	log, err := Open(Options{Directory: directory, Fallback: &fallback, Now: func() time.Time { return clock }, RunID: "run-a", MaxSegmentBytes: 8, MaxSegments: 3})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	for _, line := range []string{"one\n", "two-two\n", "three\n", "four-four\n"} {
		_, err := log.Write([]byte(line))
		require.NoError(t, err)
	}
	assert.Equal(t, "one\ntwo-two\nthree\nfour-four\n", fallback.String())
	assert.Equal(t, "run-a", log.RunID())
	assert.NotEmpty(t, log.CurrentPath())
	_, err = os.Stat(unknown)
	require.NoError(t, err)
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(entries), 4)
}

func TestRunLogIsConcurrentAndCloseIsIdempotent(t *testing.T) {
	t.Parallel()
	log, err := Open(Options{Directory: filepath.Join(t.TempDir(), "logs"), Fallback: io.Discard, RunID: "concurrent", MaxSegmentBytes: 64})
	require.NoError(t, err)
	var group sync.WaitGroup
	for range 20 {
		group.Go(func() {
			_, writeErr := log.Write([]byte(strings.Repeat("x", 10) + "\n"))
			assert.NoError(t, writeErr)
			_ = log.CurrentPath()
		})
	}
	group.Wait()
	require.NoError(t, log.Close())
	require.NoError(t, log.Close())
}

func TestRunLogRejectsSymlinkAndDegradesOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	link := filepath.Join(root, "logs")
	require.NoError(t, os.Symlink(target, link))
	var fallback bytes.Buffer
	log, err := Open(Options{Directory: link, Fallback: &fallback, RunID: "unsafe"})
	require.Error(t, err)
	_, writeErr := log.Write([]byte("record\n"))
	require.NoError(t, writeErr)
	assert.Equal(t, 1, strings.Count(fallback.String(), "diagnostics.retention"))
	assert.Contains(t, fallback.String(), "run_id=unsafe")
	assert.Contains(t, fallback.String(), "record\n")
}

func TestRunLogProtectsCurrentAndNewestPreviousRun(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "logs")
	require.NoError(t, os.MkdirAll(directory, 0o700))
	old := filepath.Join(directory, "application-20260901T000000.000000000Z-old-run-000.log")
	previous := filepath.Join(directory, "application-20260902T000000.000000000Z-previous-run-000.log")
	malformed := filepath.Join(directory, "application-not-owned.log")
	for _, path := range []string{old, previous, malformed} {
		require.NoError(t, os.WriteFile(path, []byte("historical\n"), 0o600))
	}
	oldTime := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(old, oldTime, oldTime))
	previousTime := oldTime.Add(24 * time.Hour)
	require.NoError(t, os.Chtimes(previous, previousTime, previousTime))
	log, err := Open(Options{
		Directory: directory, Fallback: io.Discard, RunID: "current-run", MaxSegments: 2,
		Now: func() time.Time { return previousTime.Add(24 * time.Hour) },
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })
	_, err = os.Stat(old)
	assert.ErrorIs(t, err, os.ErrNotExist)
	for _, path := range []string{previous, malformed, log.CurrentPath()} {
		_, err = os.Stat(path)
		require.NoError(t, err)
	}
	info, err := os.Stat(log.CurrentPath())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestRunLogKeepsOversizedRecordsOutOfTheFiniteRetainedBoundary(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "logs")
	var fallback bytes.Buffer
	log, err := Open(Options{
		Directory: directory, Fallback: &fallback, RunID: "bounded",
		MaxSegmentBytes: 8, MaxSegments: 2,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	oversized := []byte("oversized-record\n")
	written, err := log.Write(oversized)
	require.NoError(t, err)
	assert.Equal(t, len(oversized), written)
	for _, record := range []string{"12345678", "abcdefgh", "87654321"} {
		_, err = log.Write([]byte(record))
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	var retainedBytes int64
	for _, entry := range entries {
		if _, owned := parseOwnedName(entry.Name()); !owned {
			continue
		}
		info, infoErr := entry.Info()
		require.NoError(t, infoErr)
		assert.LessOrEqual(t, info.Size(), int64(8))
		retainedBytes += info.Size()
	}
	assert.LessOrEqual(t, retainedBytes, int64(16))
	assert.Contains(t, fallback.String(), string(oversized))
}

func TestRunLogKeepsLongRunningSegmentsWithinFiniteBoundary(t *testing.T) {
	t.Parallel()
	const (
		maxSegmentBytes = 1
		maxSegments     = 3
	)
	directory := filepath.Join(t.TempDir(), "logs")
	log, err := Open(Options{
		Directory: directory, Fallback: io.Discard, RunID: "long-running",
		MaxSegmentBytes: maxSegmentBytes, MaxSegments: maxSegments,
		Now: func() time.Time { return time.Date(2026, 9, 3, 10, 11, 12, 0, time.UTC) },
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	for range 1_002 {
		_, err = log.Write([]byte("x"))
		require.NoError(t, err)

		entries, readErr := os.ReadDir(directory)
		require.NoError(t, readErr)
		require.LessOrEqual(t, len(entries), maxSegments)
		var retainedBytes int64
		for _, entry := range entries {
			_, owned := parseOwnedName(entry.Name())
			require.True(t, owned, "generated segment is not recognized: %s", entry.Name())
			info, infoErr := entry.Info()
			require.NoError(t, infoErr)
			retainedBytes += info.Size()
		}
		require.LessOrEqual(t, retainedBytes, int64(maxSegmentBytes*maxSegments))
	}

	require.GreaterOrEqual(t, log.segment, 1_000)
	assert.NotEmpty(t, log.CurrentPath())
	_, err = os.Stat(log.CurrentPath())
	require.NoError(t, err)
}

func TestRunLogPreservesCurrentAndPreviousAcrossCleanAndUnexpectedRestarts(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "logs")
	openRun := func(runID string, second int) *RunLog {
		log, err := Open(Options{
			Directory: directory, Fallback: io.Discard, RunID: runID,
			MaxSegmentBytes: 64, MaxSegments: 2,
			Now: func() time.Time { return time.Date(2026, 9, 3, 10, 11, second, 0, time.UTC) },
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, log.Close()) })
		_, err = log.Write([]byte(runID + "\n"))
		require.NoError(t, err)
		return log
	}

	first := openRun("unexpected-run", 1)
	firstPath := first.CurrentPath()
	second := openRun("clean-run", 2)
	secondPath := second.CurrentPath()
	require.NoError(t, second.Close())
	third := openRun("current-run", 3)

	for _, path := range []string{secondPath, third.CurrentPath()} {
		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotEmpty(t, contents)
	}
	_, err := os.Stat(firstPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunLogDegradesSafelyOnRotationWriteAndCleanupFailures(t *testing.T) {
	t.Run("rotation", func(t *testing.T) {
		directory := filepath.Join(t.TempDir(), "logs")
		var fallback bytes.Buffer
		log, err := Open(Options{Directory: directory, Fallback: &fallback, RunID: "rotation", MaxSegmentBytes: 4})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, log.Close()) })
		_, err = log.Write([]byte("1234"))
		require.NoError(t, err)
		log.mu.Lock()
		log.directory = "relative"
		log.mu.Unlock()
		_, err = log.Write([]byte("5"))
		require.NoError(t, err)
		assert.Empty(t, log.CurrentPath())
		assert.Equal(t, 1, strings.Count(fallback.String(), "diagnostics.retention"))
	})

	t.Run("write", func(t *testing.T) {
		var fallback bytes.Buffer
		log, err := Open(Options{Directory: filepath.Join(t.TempDir(), "logs"), Fallback: &fallback, RunID: "disk-full"})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, log.Close()) })
		log.mu.Lock()
		require.NoError(t, log.file.Close())
		log.file = &failingRetainedFile{err: errors.New("disk full: private volume")}
		log.mu.Unlock()
		_, err = log.Write([]byte("safe record\n"))
		require.NoError(t, err)
		assert.Empty(t, log.CurrentPath())
		assert.NotContains(t, fallback.String(), "private volume")
		assert.Equal(t, 1, strings.Count(fallback.String(), "diagnostics.retention"))
	})

	t.Run("cleanup", func(t *testing.T) {
		directory := filepath.Join(t.TempDir(), "logs")
		require.NoError(t, os.MkdirAll(directory, 0o700))
		previous := filepath.Join(directory, "application-20260901T000000.000000000Z-previous-000.log")
		require.NoError(t, os.WriteFile(previous, []byte("12345678"), 0o600))
		stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(previous, stamp, stamp))
		var fallback bytes.Buffer
		log, err := Open(Options{
			Directory: directory, Fallback: &fallback, RunID: "current",
			MaxSegmentBytes: 8, MaxSegments: 2,
			Now: func() time.Time { return time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC) },
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, log.Close()) })
		_, err = log.Write([]byte("abcdefgh"))
		require.NoError(t, err)

		log.mu.Lock()
		log.remove = func(string) error { return errors.New("cleanup denied: private path") }
		log.mu.Unlock()
		for _, record := range []string{"1", "2", "3", "4"} {
			_, err = log.Write([]byte(record))
			require.NoError(t, err)
		}

		assert.Empty(t, log.CurrentPath())
		assert.NotContains(t, fallback.String(), "private path")
		assert.Equal(t, 1, strings.Count(fallback.String(), "diagnostics.retention"))
		entries, err := os.ReadDir(directory)
		require.NoError(t, err)
		var ownedCount int
		var retainedBytes int64
		for _, entry := range entries {
			if _, owned := parseOwnedName(entry.Name()); !owned {
				continue
			}
			ownedCount++
			info, infoErr := entry.Info()
			require.NoError(t, infoErr)
			retainedBytes += info.Size()
		}
		assert.Equal(t, 2, ownedCount)
		assert.LessOrEqual(t, retainedBytes, int64(16))
	})
}

func TestRunLogRetainsCurrentAndPreviousEvidenceWhenFallbackSinkFails(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "logs")
	require.NoError(t, os.MkdirAll(directory, 0o700))
	previousPath := filepath.Join(directory, "application-20260902T000000.000000000Z-previous-run-000.log")
	require.NoError(t, os.WriteFile(previousPath, []byte("previous evidence\n"), 0o600))
	previousTime := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(previousPath, previousTime, previousTime))

	log, err := Open(Options{
		Directory: directory, Fallback: &failingRetainedFile{err: errors.New("fallback unavailable")},
		RunID: "current-run", MaxSegmentBytes: 64, MaxSegments: 2,
		Now: func() time.Time { return previousTime.Add(24 * time.Hour) },
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	currentRecord := []byte("current evidence\n")
	written, err := log.Write(currentRecord)
	require.NoError(t, err, "a failed fallback must not affect an available retained sink")
	assert.Equal(t, len(currentRecord), written)

	previous, err := os.ReadFile(previousPath)
	require.NoError(t, err)
	assert.Equal(t, "previous evidence\n", string(previous))
	current, err := os.ReadFile(log.CurrentPath())
	require.NoError(t, err)
	assert.Equal(t, currentRecord, current)
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(entries), 2)
}

func TestRunLogRetainedSinkFailureFallsBackWithOneSafeWarning(t *testing.T) {
	t.Parallel()
	var fallback bytes.Buffer
	log, err := Open(Options{
		Directory: filepath.Join(t.TempDir(), "logs"), Fallback: &fallback, RunID: "safe-run",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	log.mu.Lock()
	require.NoError(t, log.file.Close())
	log.file = &failingRetainedFile{err: errors.New("SECRET raw retained failure")}
	log.mu.Unlock()

	for _, record := range []string{"first audit\n", "second audit\n"} {
		written, writeErr := log.Write([]byte(record))
		require.NoError(t, writeErr, "retained-log failure must not escape when fallback remains available")
		assert.Equal(t, len(record), written)
	}

	assert.Equal(t, "first audit\n"+
		"level=WARN run_id=safe-run operation=diagnostics.retention outcome=degraded error_category=storage_unavailable\n"+
		"second audit\n", fallback.String())
	assert.NotContains(t, fallback.String(), "SECRET")
}
