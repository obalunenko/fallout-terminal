package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const helperTestParentTimeout = 250 * time.Millisecond

type helperOperationLog struct {
	mu      sync.Mutex
	entries []string
}

func (log *helperOperationLog) add(entry string) {
	log.mu.Lock()
	defer log.mu.Unlock()

	log.entries = append(log.entries, entry)
}

func (log *helperOperationLog) snapshot() []string {
	log.mu.Lock()
	defer log.mu.Unlock()

	return append([]string(nil), log.entries...)
}

func TestReplacementHelperWaitsForParentWithBoundedContextBeforeMutation(t *testing.T) {
	t.Parallel()

	request := newHelperTestRequest(t)
	operations := &helperOperationLog{}
	dependencies := helperDependencies{
		waitForParent: func(ctx context.Context, parentPID int) error {
			operations.add("wait")
			assert.Equal(t, request.ParentPID, parentPID)
			deadline, ok := ctx.Deadline()
			require.True(t, ok, "parent wait context has no deadline")
			remaining := time.Until(deadline)
			assert.Positive(t, remaining)
			assert.LessOrEqual(t, remaining, request.ParentExitTimeout)
			return errors.New("parent still running")
		},
		rename: func(_, _ string) error {
			operations.add("rename")
			return nil
		},
		removeAll: func(string) error {
			operations.add("remove")
			return nil
		},
		relaunch: func(context.Context, string, string) error {
			operations.add("relaunch")
			return nil
		},
		writeRecovery: func(context.Context, UpdateRecoveryRecord) error {
			operations.add("journal")
			return nil
		},
	}

	err := applyPreparedApplication(t.Context(), request, dependencies)
	require.ErrorContains(t, err, "parent")
	assert.Equal(t, []string{"wait", "journal"}, operations.snapshot())
}

func TestReplacementHelperBacksUpPromotesRelaunchesAndCleansUp(t *testing.T) {
	t.Parallel()

	request := newHelperTestRequest(t)
	operations := &helperOperationLog{}
	var backup string
	var records []UpdateRecoveryRecord
	dependencies := helperDependencies{
		waitForParent: func(context.Context, int) error {
			operations.add("wait")
			return nil
		},
		rename: func(oldPath, newPath string) error {
			operations.add("rename:" + filepath.Base(oldPath) + "->" + filepath.Base(newPath))
			if oldPath == request.Prepared.InstalledUnit {
				backup = newPath
			}
			return nil
		},
		removeAll: func(path string) error {
			operations.add("remove:" + filepath.Base(path))
			return nil
		},
		relaunch: func(ctx context.Context, unit, relativePath string) error {
			require.NoError(t, ctx.Err())
			operations.add("relaunch")
			assert.Equal(t, request.Prepared.InstalledUnit, unit)
			assert.Equal(t, request.Prepared.LaunchRelativePath, relativePath)
			return nil
		},
		writeRecovery: func(ctx context.Context, record UpdateRecoveryRecord) error {
			require.NoError(t, ctx.Err())
			operations.add("journal:" + string(record.State))
			records = append(records, record)
			return nil
		},
	}

	require.NoError(t, applyPreparedApplication(t.Context(), request, dependencies))
	require.NotEmpty(t, backup)
	assertSafeSiblingPath(t, backup, request.Prepared.InstalledUnit, request.AttemptID)
	assert.Equal(t, []string{
		"wait",
		"rename:Fallout Terminal->" + filepath.Base(backup),
		"rename:" + filepath.Base(request.Prepared.StagedUnit) + "->Fallout Terminal",
		"relaunch",
		"journal:applied",
		"remove:" + filepath.Base(backup),
	}, operations.snapshot())
	require.Len(t, records, 1)
	assert.Equal(t, RecoveryStateApplied, records[0].State)
	assert.Empty(t, records[0].FailedStage)
}

func TestReplacementHelperRestoresBackupWhenPromotionFails(t *testing.T) {
	t.Parallel()

	request := newHelperTestRequest(t)
	operations := &helperOperationLog{}
	var backup string
	renameCalls := 0
	var record UpdateRecoveryRecord
	dependencies := helperDependencies{
		waitForParent: func(context.Context, int) error { return nil },
		rename: func(oldPath, newPath string) error {
			renameCalls++
			operations.add("rename:" + filepath.Base(oldPath) + "->" + filepath.Base(newPath))
			switch renameCalls {
			case 1:
				backup = newPath
				return nil
			case 2:
				return errors.New("injected promotion failure")
			default:
				assert.Equal(t, backup, oldPath)
				assert.Equal(t, request.Prepared.InstalledUnit, newPath)
				return nil
			}
		},
		removeAll: func(path string) error {
			operations.add("remove:" + filepath.Base(path))
			return nil
		},
		relaunch: func(_ context.Context, unit, relativePath string) error {
			operations.add("relaunch-restored")
			assert.Equal(t, request.Prepared.InstalledUnit, unit)
			assert.Equal(t, request.Prepared.LaunchRelativePath, relativePath)
			return nil
		},
		writeRecovery: func(_ context.Context, got UpdateRecoveryRecord) error {
			operations.add("journal:failed")
			record = got
			return nil
		},
	}

	err := applyPreparedApplication(t.Context(), request, dependencies)
	require.ErrorContains(t, err, "promotion")
	assertSafeSiblingPath(t, backup, request.Prepared.InstalledUnit, request.AttemptID)
	assert.Equal(t, []string{
		"rename:Fallout Terminal->" + filepath.Base(backup),
		"rename:" + filepath.Base(request.Prepared.StagedUnit) + "->Fallout Terminal",
		"rename:" + filepath.Base(backup) + "->Fallout Terminal",
		"relaunch-restored",
		"journal:failed",
	}, operations.snapshot())
	assert.Equal(t, RecoveryStateFailed, record.State)
	assert.Equal(t, FailureStageApply, record.FailedStage)
	assert.NotEmpty(t, record.RecoveryAction)
}

func TestReplacementHelperRestoresAndRelaunchesBackupWhenUpdatedRelaunchFails(t *testing.T) {
	t.Parallel()

	request := newHelperTestRequest(t)
	operations := &helperOperationLog{}
	var backup string
	relaunchCalls := 0
	var record UpdateRecoveryRecord
	dependencies := helperDependencies{
		waitForParent: func(context.Context, int) error { return nil },
		rename: func(oldPath, newPath string) error {
			if oldPath == request.Prepared.InstalledUnit {
				backup = newPath
			}
			operations.add("rename:" + filepath.Base(oldPath) + "->" + filepath.Base(newPath))
			return nil
		},
		removeAll: func(path string) error {
			operations.add("remove:" + filepath.Base(path))
			return nil
		},
		relaunch: func(_ context.Context, unit, relativePath string) error {
			relaunchCalls++
			assert.Equal(t, request.Prepared.InstalledUnit, unit)
			assert.Equal(t, request.Prepared.LaunchRelativePath, relativePath)
			if relaunchCalls == 1 {
				operations.add("relaunch-updated")
				return errors.New("injected relaunch failure")
			}
			operations.add("relaunch-restored")
			return nil
		},
		writeRecovery: func(_ context.Context, got UpdateRecoveryRecord) error {
			operations.add("journal:failed")
			record = got
			return nil
		},
	}

	err := applyPreparedApplication(t.Context(), request, dependencies)
	require.ErrorContains(t, err, "relaunch")
	assertSafeSiblingPath(t, backup, request.Prepared.InstalledUnit, request.AttemptID)
	assert.Equal(t, []string{
		"rename:Fallout Terminal->" + filepath.Base(backup),
		"rename:" + filepath.Base(request.Prepared.StagedUnit) + "->Fallout Terminal",
		"relaunch-updated",
		"remove:Fallout Terminal",
		"rename:" + filepath.Base(backup) + "->Fallout Terminal",
		"relaunch-restored",
		"journal:failed",
	}, operations.snapshot())
	assert.Equal(t, RecoveryStateFailed, record.State)
	assert.Equal(t, FailureStageRelaunch, record.FailedStage)
	assert.NotEmpty(t, record.RecoveryAction)
}

func TestReplacementHelperReportsBackupCleanupFailureWithoutBreakingUpdatedRelaunch(t *testing.T) {
	t.Parallel()

	request := newHelperTestRequest(t)
	backup := helperBackupPath(request)
	var records []UpdateRecoveryRecord
	dependencies := helperDependencies{
		waitForParent: func(context.Context, int) error { return nil },
		rename:        func(string, string) error { return nil },
		removeAll: func(path string) error {
			assert.Equal(t, backup, path)
			return errors.New("injected backup cleanup failure: /Users/private/backup")
		},
		relaunch: func(context.Context, string, string) error { return nil },
		writeRecovery: func(_ context.Context, record UpdateRecoveryRecord) error {
			records = append(records, record)
			return nil
		},
	}

	require.NoError(t, applyPreparedApplication(t.Context(), request, dependencies))
	require.Len(t, records, 2)
	assert.Equal(t, RecoveryStateApplied, records[0].State)
	assert.Equal(t, RecoveryStateFailed, records[1].State)
	assert.Equal(t, FailureStageRecovery, records[1].FailedStage)
	assert.NotEmpty(t, records[1].RecoveryAction)
	assert.NotContains(t, records[1].Message, "/Users/private")
}

func TestReplacementHelperReportsRecoveryWhenRestoredApplicationCannotRelaunch(t *testing.T) {
	t.Parallel()

	request := newHelperTestRequest(t)
	renameCalls := 0
	var record UpdateRecoveryRecord
	dependencies := helperDependencies{
		waitForParent: func(context.Context, int) error { return nil },
		rename: func(string, string) error {
			renameCalls++
			if renameCalls == 2 {
				return errors.New("injected promotion failure")
			}
			return nil
		},
		removeAll: func(string) error { return nil },
		relaunch: func(context.Context, string, string) error {
			return errors.New("injected restored relaunch failure: /Users/private/Fallout Terminal")
		},
		writeRecovery: func(_ context.Context, got UpdateRecoveryRecord) error {
			record = got
			return nil
		},
	}

	err := applyPreparedApplication(t.Context(), request, dependencies)
	require.Error(t, err)
	assert.Equal(t, RecoveryStateFailed, record.State)
	assert.Equal(t, FailureStageRecovery, record.FailedStage)
	assert.NotEmpty(t, record.RecoveryAction)
	assert.NotContains(t, record.Message, "/Users/private")
}

func TestReplacementHelperNeverMutatesUserOwnedData(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	request := newHelperTestRequestAtRoot(root)
	writeHelperTestFile(t, filepath.Join(request.Prepared.InstalledUnit, request.Prepared.LaunchRelativePath), "old")
	writeHelperTestFile(t, filepath.Join(request.Prepared.StagedUnit, request.Prepared.LaunchRelativePath), "new")
	userFiles := map[string]string{
		filepath.Join(root, "Documents", "session.json"):                 "session-owned",
		filepath.Join(root, "Application Support", "player-config.json"): "config-owned",
		filepath.Join(root, "credentials", "token"):                      "credential-owned",
	}
	for path, contents := range userFiles {
		writeHelperTestFile(t, path, contents)
	}

	dependencies := helperDependencies{
		waitForParent: func(context.Context, int) error { return nil },
		rename:        os.Rename,
		removeAll:     os.RemoveAll,
		relaunch:      func(context.Context, string, string) error { return nil },
		writeRecovery: func(context.Context, UpdateRecoveryRecord) error { return nil },
	}
	require.NoError(t, applyPreparedApplication(t.Context(), request, dependencies))

	for path, want := range userFiles {
		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, want, string(contents))
	}
	assert.FileExists(t, filepath.Join(request.Prepared.InstalledUnit, request.Prepared.LaunchRelativePath))
}

func TestRecoveryJournalAtomicallyReplacesStaleApplyingStateWithSafeFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	recoveryPath := filepath.Join(root, "metadata", "update-recovery.json")
	record := UpdateRecoveryRecord{
		SchemaVersion:   RecoverySchemaVersion,
		AttemptID:       "attempt-stale",
		ExpectedVersion: "2.5.0",
		State:           RecoveryStateApplying,
		UpdatedAt:       time.Now().Add(-time.Hour).UTC(),
	}
	require.NoError(t, writeHelperRecovery(t.Context(), recoveryPath, record))

	require.NoError(t, writeHelperRecovery(t.Context(), recoveryPath, UpdateRecoveryRecord{
		SchemaVersion:   RecoverySchemaVersion,
		AttemptID:       record.AttemptID,
		ExpectedVersion: record.ExpectedVersion,
		State:           RecoveryStateFailed,
		FailedStage:     FailureStageRecovery,
		Message:         "The previous update did not finish applying.",
		RecoveryAction:  "Continue using the installed application and retry the update.",
		UpdatedAt:       time.Now().UTC(),
	}))

	payload, err := os.ReadFile(recoveryPath)
	require.NoError(t, err)
	var consumed UpdateRecoveryRecord
	require.NoError(t, json.Unmarshal(payload, &consumed))
	assert.Equal(t, RecoveryStateFailed, consumed.State)
	assert.Equal(t, FailureStageRecovery, consumed.FailedStage)
	assert.NotEmpty(t, consumed.RecoveryAction)
	assert.NotContains(t, string(payload), root)
}

func TestReplacementHelperRejectsUnsafeRequestsBeforeWaiting(t *testing.T) {
	t.Parallel()

	valid := newHelperTestRequest(t)
	nonSibling := filepath.Join(t.TempDir(), "Fallout Terminal.update-attempt")
	tests := []struct {
		name   string
		mutate func(*helperRequest)
	}{
		{name: "empty attempt", mutate: func(request *helperRequest) { request.AttemptID = "" }},
		{name: "attempt ownership mismatch", mutate: func(request *helperRequest) { request.AttemptID = "other" }},
		{name: "filesystem root target", mutate: func(request *helperRequest) {
			request.Prepared.InstalledUnit = string(filepath.Separator)
		}},
		{name: "filesystem root stage", mutate: func(request *helperRequest) {
			request.Prepared.StagedUnit = string(filepath.Separator)
		}},
		{name: "non sibling stage", mutate: func(request *helperRequest) { request.Prepared.StagedUnit = nonSibling }},
		{name: "same target and stage", mutate: func(request *helperRequest) {
			request.Prepared.StagedUnit = request.Prepared.InstalledUnit
		}},
		{name: "traversing launch path", mutate: func(request *helperRequest) {
			request.Prepared.LaunchRelativePath = filepath.Join("..", "Fallout Terminal")
		}},
		{name: "absolute launch path", mutate: func(request *helperRequest) {
			request.Prepared.LaunchRelativePath = filepath.Join(filepath.Dir(request.Prepared.InstalledUnit), "Fallout Terminal")
		}},
		{name: "missing parent", mutate: func(request *helperRequest) { request.ParentPID = 0 }},
		{name: "unbounded parent wait", mutate: func(request *helperRequest) { request.ParentExitTimeout = 0 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := valid
			test.mutate(&request)
			waits := 0
			dependencies := helperDependencies{
				waitForParent: func(context.Context, int) error { waits++; return nil },
				rename:        func(string, string) error { return nil },
				removeAll:     func(string) error { return nil },
				relaunch:      func(context.Context, string, string) error { return nil },
				writeRecovery: func(context.Context, UpdateRecoveryRecord) error { return nil },
			}

			err := applyPreparedApplication(t.Context(), request, dependencies)
			require.Error(t, err)
			assert.Zero(t, waits)
		})
	}
}

func newHelperTestRequest(t *testing.T) helperRequest {
	t.Helper()

	return newHelperTestRequestAtRoot(t.TempDir())
}

func newHelperTestRequestAtRoot(root string) helperRequest {
	installed := filepath.Join(root, "Fallout Terminal")
	staged := filepath.Join(root, "Fallout Terminal.update-attempt-42")
	return helperRequest{
		AttemptID:         "attempt-42",
		ExpectedVersion:   "2.5.0",
		ParentPID:         4242,
		ParentExitTimeout: helperTestParentTimeout,
		RecoveryPath:      filepath.Join(root, "update-recovery.json"),
		Prepared: PreparedApplicationUnit{
			AttemptID:          "attempt-42",
			Version:            "2.5.0",
			Target:             Target{OS: "linux", Arch: "amd64"},
			InstalledUnit:      installed,
			StagedUnit:         staged,
			LaunchRelativePath: "Fallout Terminal",
		},
	}
}

func writeHelperTestFile(t *testing.T, path, contents string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
}
