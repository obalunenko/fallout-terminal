package update

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
)

const maximumRecoveryRecordBytes = 16 * 1024

const (
	recoveryFailureMessage = "The previous application update did not complete successfully."
	recoveryFailureAction  = "Continue using the installed application and try the update again on the next launch."
	recoveryStaleMessage   = "The previous application update did not finish applying."
	recoveryVersionMessage = "The updated application did not start with the expected version."
)

// RecoveryOutcome is the safe launch-scoped result of consuming the private
// replacement journal. Failure is detached from the decoded record and can be
// used as the initial visible update status without exposing journal text.
type RecoveryOutcome struct {
	Failure *UpdateFailure
}

// ConsumeApplicationUpdateRecovery consumes a valid replacement journal on a
// normal launch. Missing, unreadable, malformed, and unsupported records are
// deliberately inert: recovery metadata can never prevent local startup.
//
// A matching applied record is cleared silently. Failed and interrupted
// records become a safe launch-scoped diagnostic and are then cleared. An
// applied record whose expected version is not running remains in place so it
// can only be cleared by the version it records.
func ConsumeApplicationUpdateRecovery(
	ctx context.Context,
	path string,
	runningVersion string,
) RecoveryOutcome {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil || path == "" || runningVersion == "" {
		return RecoveryOutcome{}
	}

	record, ok := readApplicationUpdateRecovery(ctx, path)
	if !ok {
		return RecoveryOutcome{}
	}

	switch record.State {
	case RecoveryStateApplied:
		if record.ExpectedVersion == runningVersion {
			removeConsumedRecovery(path)
			return RecoveryOutcome{}
		}
		return recoveryOutcome(FailureStageRecovery, recoveryVersionMessage)
	case RecoveryStateFailed:
		outcome := recoveryOutcome(record.FailedStage, recoveryFailureMessage)
		removeConsumedRecovery(path)
		return outcome
	case RecoveryStateApplying:
		outcome := recoveryOutcome(FailureStageRecovery, recoveryStaleMessage)
		removeConsumedRecovery(path)
		return outcome
	default:
		return RecoveryOutcome{}
	}
}

func readApplicationUpdateRecovery(ctx context.Context, path string) (UpdateRecoveryRecord, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Size() <= 0 || info.Size() > maximumRecoveryRecordBytes {
		return UpdateRecoveryRecord{}, false
	}

	file, err := os.Open(path)
	if err != nil {
		return UpdateRecoveryRecord{}, false
	}
	defer func() { _ = file.Close() }()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return UpdateRecoveryRecord{}, false
	}

	limited := &io.LimitedReader{R: file, N: maximumRecoveryRecordBytes + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var record UpdateRecoveryRecord
	if err := decoder.Decode(&record); err != nil || limited.N <= 0 || ctx.Err() != nil {
		return UpdateRecoveryRecord{}, false
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return UpdateRecoveryRecord{}, false
	}
	if !validApplicationUpdateRecovery(record) {
		return UpdateRecoveryRecord{}, false
	}
	return record, true
}

func validApplicationUpdateRecovery(record UpdateRecoveryRecord) bool {
	if record.SchemaVersion != RecoverySchemaVersion ||
		!helperAttemptPattern.MatchString(record.AttemptID) ||
		record.ExpectedVersion == "" || len(record.ExpectedVersion) > 128 ||
		!record.State.Valid() || record.UpdatedAt.IsZero() {
		return false
	}
	if record.State == RecoveryStateFailed {
		return record.FailedStage.Valid() && len(record.Message) <= 512 &&
			record.RecoveryAction != "" &&
			len(record.RecoveryAction) <= 512
	}
	return record.FailedStage == "" && record.Message == "" && record.RecoveryAction == ""
}

func recoveryOutcome(stage FailureStage, message string) RecoveryOutcome {
	failure := UpdateFailure{
		Stage:          stage,
		Message:        message,
		RecoveryAction: recoveryFailureAction,
	}
	return RecoveryOutcome{Failure: &failure}
}

func removeConsumedRecovery(path string) {
	// The exact validated journal is the only path this launch owns. Never
	// sweep the containing Application Support directory or infer sibling paths.
	_ = os.Remove(path)
}
