package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsumeApplicationUpdateRecovery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		record            UpdateRecoveryRecord
		runningVersion    string
		wantFailureStage  FailureStage
		wantMessage       string
		wantJournalExists bool
	}{
		{
			name:           "matching applied clears silently",
			record:         recoveryTestRecord(RecoveryStateApplied),
			runningVersion: "2.5.0",
		},
		{
			name:              "mismatched applied remains for expected version",
			record:            recoveryTestRecord(RecoveryStateApplied),
			runningVersion:    "2.4.0",
			wantFailureStage:  FailureStageRecovery,
			wantMessage:       recoveryVersionMessage,
			wantJournalExists: true,
		},
		{
			name: "failed becomes safe initial diagnostic",
			record: func() UpdateRecoveryRecord {
				record := recoveryTestRecord(RecoveryStateFailed)
				record.FailedStage = FailureStageRelaunch
				record.Message = "private provider detail: /Users/example/application"
				record.RecoveryAction = "private token ghp_example"
				return record
			}(),
			runningVersion:   "2.4.0",
			wantFailureStage: FailureStageRelaunch,
			wantMessage:      recoveryFailureMessage,
		},
		{
			name:             "stale applying becomes safe recovery diagnostic",
			record:           recoveryTestRecord(RecoveryStateApplying),
			runningVersion:   "2.4.0",
			wantFailureStage: FailureStageRecovery,
			wantMessage:      recoveryStaleMessage,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "metadata", "application-update-recovery.json")
			require.NoError(t, writeHelperRecovery(t.Context(), path, test.record))

			outcome := ConsumeApplicationUpdateRecovery(t.Context(), path, test.runningVersion)
			if test.wantFailureStage == "" {
				assert.Nil(t, outcome.Failure)
			} else {
				require.NotNil(t, outcome.Failure)
				assert.Equal(t, test.wantFailureStage, outcome.Failure.Stage)
				assert.Equal(t, test.wantMessage, outcome.Failure.Message)
				assert.Equal(t, recoveryFailureAction, outcome.Failure.RecoveryAction)
				assert.NotContains(t, outcome.Failure.Message, "/Users/example")
				assert.NotContains(t, outcome.Failure.RecoveryAction, "ghp_example")
			}
			if test.wantJournalExists {
				assert.FileExists(t, path)
			} else {
				assert.NoFileExists(t, path)
			}
		})
	}
}

func TestConsumeApplicationUpdateRecoveryIgnoresInvalidRecordsWithoutCleanup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "malformed", payload: []byte(`{"schemaVersion":`)},
		{name: "unsupported schema", payload: recoveryTestPayload(t, func(record *UpdateRecoveryRecord) {
			record.SchemaVersion++
		})},
		{name: "unknown field", payload: []byte(`{"schemaVersion":1,"attemptID":"attempt-42","expectedVersion":"2.5.0","state":"applied","updatedAt":"2026-08-27T00:00:00Z","providerURL":"https://secret.example"}`)},
		{name: "oversized", payload: make([]byte, maximumRecoveryRecordBytes+1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "application-update-recovery.json")
			require.NoError(t, os.WriteFile(path, test.payload, 0o600))

			outcome := ConsumeApplicationUpdateRecovery(t.Context(), path, "2.5.0")
			assert.Nil(t, outcome.Failure)
			assert.FileExists(t, path)
		})
	}
}

func TestConsumeApplicationUpdateRecoveryNeverFollowsSymlinkOrRemovesSiblingData(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	require.NoError(t, os.WriteFile(target, recoveryTestPayload(t, nil), 0o600))
	journal := filepath.Join(root, "application-update-recovery.json")
	require.NoError(t, os.Symlink(target, journal))
	sibling := filepath.Join(root, "sessions.json")
	require.NoError(t, os.WriteFile(sibling, []byte("user-owned"), 0o600))

	outcome := ConsumeApplicationUpdateRecovery(t.Context(), journal, "2.5.0")
	assert.Nil(t, outcome.Failure)
	assert.FileExists(t, journal)
	contents, err := os.ReadFile(sibling)
	require.NoError(t, err)
	assert.Equal(t, "user-owned", string(contents))
}

func TestConsumeApplicationUpdateRecoveryCancellationIsInert(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "application-update-recovery.json")
	require.NoError(t, writeHelperRecovery(t.Context(), path, recoveryTestRecord(RecoveryStateApplied)))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	outcome := ConsumeApplicationUpdateRecovery(ctx, path, "2.5.0")
	assert.Nil(t, outcome.Failure)
	assert.FileExists(t, path)
}

func recoveryTestRecord(state RecoveryState) UpdateRecoveryRecord {
	return UpdateRecoveryRecord{
		SchemaVersion:   RecoverySchemaVersion,
		AttemptID:       "attempt-42",
		ExpectedVersion: "2.5.0",
		State:           state,
		UpdatedAt:       time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC),
	}
}

func recoveryTestPayload(t *testing.T, mutate func(*UpdateRecoveryRecord)) []byte {
	t.Helper()
	record := recoveryTestRecord(RecoveryStateApplied)
	if mutate != nil {
		mutate(&record)
	}
	payload, err := json.Marshal(record)
	require.NoError(t, err)
	return payload
}
