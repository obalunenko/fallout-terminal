package update

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	replacementHelperModeEnvironment    = "FALLOUT_TERMINAL_UPDATE_HELPER_MODE"
	replacementHelperRequestEnvironment = "FALLOUT_TERMINAL_UPDATE_HELPER_REQUEST"
	replacementHelperModeValue          = "1"

	defaultParentExitTimeout = 30 * time.Second
	maximumParentExitTimeout = 2 * time.Minute
)

var (
	errHelperParentWait = errors.New("replacement helper parent wait failed")
	errHelperBackup     = errors.New("replacement helper backup failed")
	errHelperPromotion  = errors.New("replacement helper promotion failed")
	errHelperRelaunch   = errors.New("replacement helper relaunch failed")
	errHelperRecovery   = errors.New("replacement helper recovery failed")

	helperAttemptPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

const (
	helperApplyFailureMessage    = "The prepared application could not replace the installed version."
	helperRelaunchFailureMessage = "The updated application could not be relaunched."
	helperRecoveryFailureMessage = "The previous application could not be restored automatically."
	helperParentFailureMessage   = "The application did not finish shutting down before replacement."
	helperFailureRecoveryAction  = "Continue using the restored application and try the update again on the next launch."
	helperManualRecoveryAction   = "Keep the installed application closed and restore the previous application backup."
	helperShutdownRecoveryAction = "Close the application completely and try the update again on the next launch."
	helperCleanupFailureMessage  = "The updated application is running, but its previous backup could not be removed."
	helperCleanupRecoveryAction  = "Continue using the updated application and retry backup cleanup on the next launch."
)

// HelperRequest is the private handoff from the running application to its
// copied replacement helper. It must never be projected through the desktop
// service, events, logs, or command results.
type HelperRequest struct {
	AttemptID         string                  `json:"attemptID"`
	ExpectedVersion   string                  `json:"expectedVersion"`
	ParentPID         int                     `json:"parentPID"`
	ParentExitTimeout time.Duration           `json:"parentExitTimeout"`
	RecoveryPath      string                  `json:"recoveryPath"`
	Prepared          PreparedApplicationUnit `json:"prepared"`
	CleanupPath       string                  `json:"cleanupPath,omitempty"`
}

// Keep the implementation vocabulary private while allowing root composition
// to construct a handoff without duplicating its representation.
type helperRequest = HelperRequest

type helperDependencies struct {
	waitForParent func(context.Context, int) error
	rename        func(string, string) error
	removeAll     func(string) error
	relaunch      func(context.Context, string, string) error
	writeRecovery func(context.Context, UpdateRecoveryRecord) error
}

// HelperMode reports whether the process was launched as a copied update
// helper. The marker is private process state rather than a public CLI flag.
func HelperMode(lookup func(string) (string, bool)) bool {
	if lookup == nil {
		return false
	}
	value, ok := lookup(replacementHelperModeEnvironment)
	return ok && value == replacementHelperModeValue
}

// RunReplacementHelperFromEnvironment handles copied helper mode. The bool is
// false for an ordinary application launch. The encoded request is removed
// from the environment inherited by the relaunched application.
func RunReplacementHelperFromEnvironment(
	ctx context.Context,
	lookup func(string) (string, bool),
) (bool, error) {
	if !HelperMode(lookup) {
		return false, nil
	}

	request, err := decodeHelperRequest(lookup)
	if err != nil {
		return true, err
	}
	if err := validateConcreteHelperRequest(request); err != nil {
		return true, err
	}

	dependencies := helperDependencies{
		waitForParent: platformWaitForParent,
		rename:        os.Rename,
		removeAll:     os.RemoveAll,
		relaunch:      platformRelaunch,
		writeRecovery: func(ctx context.Context, record UpdateRecoveryRecord) error {
			return writeHelperRecovery(ctx, request.RecoveryPath, record)
		},
	}
	err = applyPreparedApplication(ctx, request, dependencies)
	if request.CleanupPath != "" {
		_ = os.RemoveAll(request.CleanupPath)
	}
	return true, err
}

// LaunchCopiedReplacementHelper copies the running executable outside the
// replacement unit and starts it with a validated private environment handoff.
// The caller may then request the host's normal ordered shutdown.
func LaunchCopiedReplacementHelper(ctx context.Context, request HelperRequest) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return errors.New("replacement helper launch canceled")
	}
	if request.ParentExitTimeout <= 0 {
		request.ParentExitTimeout = defaultParentExitTimeout
	}
	if err := validateHelperRequest(request); err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return errors.New("resolve replacement helper executable")
	}
	if err := requireRegularPath(executable); err != nil {
		return err
	}
	if err := requireExistingUnit(request.Prepared.InstalledUnit); err != nil {
		return err
	}
	if err := requireExistingUnit(request.Prepared.StagedUnit); err != nil {
		return err
	}

	helperDirectory, err := os.MkdirTemp("", "fallout-terminal-update-helper-"+request.AttemptID+"-")
	if err != nil {
		return errors.New("create replacement helper")
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(helperDirectory)
		}
	}()

	helperName := "fallout-terminal-update-helper"
	if strings.EqualFold(filepath.Ext(executable), ".exe") {
		helperName += ".exe"
	}
	helperExecutable := filepath.Join(helperDirectory, helperName)
	if err := copyHelperExecutable(executable, helperExecutable); err != nil {
		return err
	}

	request.CleanupPath = helperDirectory
	payload, err := encodeHelperRequest(request)
	if err != nil {
		return err
	}
	environment := sanitizedHelperEnvironment(os.Environ())
	environment = append(environment,
		replacementHelperModeEnvironment+"="+replacementHelperModeValue,
		replacementHelperRequestEnvironment+"="+payload,
	)
	if err := writeHelperRecovery(ctx, request.RecoveryPath, UpdateRecoveryRecord{
		SchemaVersion:   RecoverySchemaVersion,
		AttemptID:       request.AttemptID,
		ExpectedVersion: request.ExpectedVersion,
		State:           RecoveryStateApplying,
		UpdatedAt:       time.Now().UTC(),
	}); err != nil {
		return errors.New("write replacement recovery record")
	}
	if err := platformStartHelper(helperExecutable, environment); err != nil {
		_ = writeHelperRecovery(ctx, request.RecoveryPath, failedHelperRecovery(
			request,
			FailureStageApply,
			"The replacement helper could not be started.",
			"Continue using this version and try the update again on the next launch.",
		))
		return errors.New("start replacement helper")
	}

	cleanup = false
	return nil
}

func applyPreparedApplication(ctx context.Context, request helperRequest, dependencies helperDependencies) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateHelperRequest(request); err != nil {
		return err
	}
	if err := validateHelperDependencies(dependencies); err != nil {
		return err
	}

	waitContext, cancel := context.WithTimeout(ctx, request.ParentExitTimeout)
	waitErr := dependencies.waitForParent(waitContext, request.ParentPID)
	cancel()
	if waitErr != nil {
		recordErr := dependencies.writeRecovery(ctx, failedHelperRecovery(
			request, FailureStageApply, helperParentFailureMessage, helperShutdownRecoveryAction,
		))
		return errors.Join(errHelperParentWait, recoveryWriteError(recordErr))
	}

	backup := helperBackupPath(request)
	if err := dependencies.rename(request.Prepared.InstalledUnit, backup); err != nil {
		recordErr := dependencies.writeRecovery(ctx, failedHelperRecovery(
			request, FailureStageApply, helperApplyFailureMessage, helperShutdownRecoveryAction,
		))
		return errors.Join(errHelperBackup, recoveryWriteError(recordErr))
	}

	if err := dependencies.rename(request.Prepared.StagedUnit, request.Prepared.InstalledUnit); err != nil {
		restoreErr := dependencies.rename(backup, request.Prepared.InstalledUnit)
		restoredRelaunchErr := error(nil)
		if restoreErr == nil {
			restoredRelaunchErr = dependencies.relaunch(
				ctx, request.Prepared.InstalledUnit, request.Prepared.LaunchRelativePath,
			)
		}
		recoveryErr := errors.Join(restoreErr, restoredRelaunchErr)
		failureStage, message, action := restoredFailure(FailureStageApply, recoveryErr)
		recordErr := dependencies.writeRecovery(ctx, failedHelperRecovery(request, failureStage, message, action))
		return errors.Join(errHelperPromotion, recoveryOperationError(recoveryErr), recoveryWriteError(recordErr))
	}

	if err := dependencies.relaunch(ctx, request.Prepared.InstalledUnit, request.Prepared.LaunchRelativePath); err != nil {
		removeErr := dependencies.removeAll(request.Prepared.InstalledUnit)
		restoreErr := error(nil)
		if removeErr == nil {
			restoreErr = dependencies.rename(backup, request.Prepared.InstalledUnit)
		}
		restoredRelaunchErr := error(nil)
		if removeErr == nil && restoreErr == nil {
			restoredRelaunchErr = dependencies.relaunch(
				ctx, request.Prepared.InstalledUnit, request.Prepared.LaunchRelativePath,
			)
		}
		recoveryErr := errors.Join(removeErr, restoreErr, restoredRelaunchErr)
		failureStage, message, action := restoredFailure(FailureStageRelaunch, recoveryErr)
		recordErr := dependencies.writeRecovery(ctx, failedHelperRecovery(request, failureStage, message, action))
		return errors.Join(errHelperRelaunch, recoveryOperationError(recoveryErr), recoveryWriteError(recordErr))
	}

	record := UpdateRecoveryRecord{
		SchemaVersion:   RecoverySchemaVersion,
		AttemptID:       request.AttemptID,
		ExpectedVersion: request.ExpectedVersion,
		State:           RecoveryStateApplied,
		UpdatedAt:       time.Now().UTC(),
	}
	if err := dependencies.writeRecovery(ctx, record); err != nil {
		return errors.Join(errHelperRecovery, recoveryWriteError(err))
	}
	if err := dependencies.removeAll(backup); err != nil {
		_ = dependencies.writeRecovery(ctx, failedHelperRecovery(
			request,
			FailureStageRecovery,
			helperCleanupFailureMessage,
			helperCleanupRecoveryAction,
		))
	}
	return nil
}

func validateHelperRequest(request helperRequest) error {
	if !helperAttemptPattern.MatchString(request.AttemptID) {
		return errors.New("replacement helper request has an invalid attempt")
	}
	if request.Prepared.AttemptID != request.AttemptID {
		return errors.New("replacement helper request has mismatched ownership")
	}
	if request.ExpectedVersion == "" || request.Prepared.Version != request.ExpectedVersion {
		return errors.New("replacement helper request has a mismatched version")
	}
	if !validHelperTarget(request.Prepared.Target) {
		return errors.New("replacement helper request has an invalid target")
	}
	if request.ParentPID <= 0 {
		return errors.New("replacement helper request has no parent process")
	}
	if request.ParentExitTimeout <= 0 || request.ParentExitTimeout > maximumParentExitTimeout {
		return errors.New("replacement helper request has an invalid parent wait")
	}
	if err := validateAbsoluteUnitPath(request.Prepared.InstalledUnit); err != nil {
		return err
	}
	if err := validateAbsoluteUnitPath(request.Prepared.StagedUnit); err != nil {
		return err
	}
	if filepath.Clean(request.Prepared.InstalledUnit) == filepath.Clean(request.Prepared.StagedUnit) ||
		filepath.Dir(request.Prepared.InstalledUnit) != filepath.Dir(request.Prepared.StagedUnit) {
		return errors.New("replacement helper request units are not distinct siblings")
	}
	if !strings.Contains(filepath.Base(request.Prepared.StagedUnit), request.AttemptID) {
		return errors.New("replacement helper request stage is not attempt owned")
	}
	if err := validateLaunchRelativePath(request.Prepared.LaunchRelativePath); err != nil {
		return err
	}
	if err := validateAbsoluteUnitPath(request.RecoveryPath); err != nil {
		return errors.New("replacement helper request has an invalid recovery record")
	}
	if pathWithin(request.RecoveryPath, request.Prepared.InstalledUnit) ||
		pathWithin(request.RecoveryPath, request.Prepared.StagedUnit) {
		return errors.New("replacement helper recovery record is inside a replacement unit")
	}
	if request.CleanupPath != "" {
		if err := validateAbsoluteUnitPath(request.CleanupPath); err != nil {
			return errors.New("replacement helper request has an invalid cleanup path")
		}
		if filepath.Dir(request.CleanupPath) != filepath.Clean(os.TempDir()) ||
			!strings.Contains(filepath.Base(request.CleanupPath), request.AttemptID) {
			return errors.New("replacement helper cleanup is not attempt owned")
		}
		if pathWithin(request.Prepared.InstalledUnit, request.CleanupPath) ||
			pathWithin(request.Prepared.StagedUnit, request.CleanupPath) {
			return errors.New("replacement helper cleanup includes an application unit")
		}
	}
	return nil
}

func validateConcreteHelperRequest(request helperRequest) error {
	if err := validateHelperRequest(request); err != nil {
		return err
	}
	if err := requireExistingUnit(request.Prepared.InstalledUnit); err != nil {
		return err
	}
	if err := requireExistingUnit(request.Prepared.StagedUnit); err != nil {
		return err
	}
	if err := requireRegularPath(filepath.Join(
		request.Prepared.InstalledUnit, request.Prepared.LaunchRelativePath,
	)); err != nil {
		return err
	}
	return requireRegularPath(filepath.Join(request.Prepared.StagedUnit, request.Prepared.LaunchRelativePath))
}

func validHelperTarget(target Target) bool {
	switch target.OS {
	case "windows":
		return target.Arch == "amd64"
	case "linux", "darwin":
		return target.Arch == "amd64" || target.Arch == "arm64"
	default:
		return false
	}
}

func validateHelperDependencies(dependencies helperDependencies) error {
	if dependencies.waitForParent == nil || dependencies.rename == nil || dependencies.removeAll == nil ||
		dependencies.relaunch == nil || dependencies.writeRecovery == nil {
		return errors.New("replacement helper dependencies are incomplete")
	}
	return nil
}

func validateAbsoluteUnitPath(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("replacement helper request contains an unsafe path")
	}
	volume := filepath.VolumeName(path)
	root := volume + string(filepath.Separator)
	if filepath.Clean(path) == filepath.Clean(root) {
		return errors.New("replacement helper request contains a filesystem root")
	}
	return nil
}

func validateLaunchRelativePath(path string) error {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || path == "." {
		return errors.New("replacement helper request has an invalid launch path")
	}
	if slices.Contains(strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	}), "..") {
		return errors.New("replacement helper request has a traversing launch path")
	}
	return nil
}

func helperBackupPath(request helperRequest) string {
	return filepath.Join(
		filepath.Dir(request.Prepared.InstalledUnit),
		"."+filepath.Base(request.Prepared.InstalledUnit)+".backup-"+request.AttemptID,
	)
}

func failedHelperRecovery(
	request helperRequest,
	stage FailureStage,
	message string,
	recoveryAction string,
) UpdateRecoveryRecord {
	return UpdateRecoveryRecord{
		SchemaVersion:   RecoverySchemaVersion,
		AttemptID:       request.AttemptID,
		ExpectedVersion: request.ExpectedVersion,
		State:           RecoveryStateFailed,
		FailedStage:     stage,
		Message:         message,
		RecoveryAction:  recoveryAction,
		UpdatedAt:       time.Now().UTC(),
	}
}

func restoredFailure(originalStage FailureStage, restoreErr error) (FailureStage, string, string) {
	if restoreErr != nil {
		return FailureStageRecovery, helperRecoveryFailureMessage, helperManualRecoveryAction
	}
	if originalStage == FailureStageRelaunch {
		return originalStage, helperRelaunchFailureMessage, helperFailureRecoveryAction
	}
	return originalStage, helperApplyFailureMessage, helperFailureRecoveryAction
}

func recoveryOperationError(err error) error {
	if err == nil {
		return nil
	}
	return errHelperRecovery
}

func recoveryWriteError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New("write replacement recovery record")
}

func encodeHelperRequest(request helperRequest) (string, error) {
	if err := validateHelperRequest(request); err != nil {
		return "", err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return "", errors.New("encode replacement helper request")
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeHelperRequest(lookup func(string) (string, bool)) (helperRequest, error) {
	if lookup == nil {
		return helperRequest{}, errors.New("replacement helper environment is unavailable")
	}
	encoded, ok := lookup(replacementHelperRequestEnvironment)
	if !ok || encoded == "" {
		return helperRequest{}, errors.New("replacement helper request is missing")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return helperRequest{}, errors.New("replacement helper request is invalid")
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	var request helperRequest
	if err := decoder.Decode(&request); err != nil {
		return helperRequest{}, errors.New("replacement helper request is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return helperRequest{}, errors.New("replacement helper request is invalid")
	}
	if err := validateHelperRequest(request); err != nil {
		return helperRequest{}, err
	}
	return request, nil
}

func sanitizedHelperEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if key == replacementHelperModeEnvironment || key == replacementHelperRequestEnvironment {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func copyHelperExecutable(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return errors.New("open replacement helper source")
	}
	defer func() { _ = input.Close() }()

	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return errors.New("create replacement helper copy")
	}
	closeOutput := true
	defer func() {
		if closeOutput {
			_ = output.Close()
		}
	}()
	if _, err := io.Copy(output, input); err != nil {
		return errors.New("copy replacement helper executable")
	}
	if err := output.Sync(); err != nil {
		return errors.New("sync replacement helper executable")
	}
	if err := output.Close(); err != nil {
		return errors.New("close replacement helper executable")
	}
	closeOutput = false
	return nil
}

func writeHelperRecovery(ctx context.Context, path string, record UpdateRecoveryRecord) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return errors.New("recovery record write canceled")
	}
	if record.SchemaVersion != RecoverySchemaVersion || !record.State.Valid() ||
		!helperAttemptPattern.MatchString(record.AttemptID) || record.ExpectedVersion == "" {
		return errors.New("recovery record is invalid")
	}
	if record.State == RecoveryStateFailed && (!record.FailedStage.Valid() || record.RecoveryAction == "") {
		return errors.New("recovery failure record is incomplete")
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return errors.New("create recovery record directory")
	}
	temporary, err := os.CreateTemp(directory, ".update-recovery-*")
	if err != nil {
		return errors.New("create recovery record")
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return errors.New("secure recovery record")
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(record); err != nil {
		return errors.New("encode recovery record")
	}
	if err := temporary.Sync(); err != nil {
		return errors.New("sync recovery record")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("close recovery record")
	}
	closed = true
	if err := os.Rename(temporaryPath, path); err != nil {
		return errors.New("replace recovery record")
	}
	return nil
}

func requireExistingUnit(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("replacement application unit is unavailable")
	}
	return nil
}

func requireRegularPath(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("replacement helper executable is unavailable")
	}
	return nil
}

func pathWithin(path, parent string) bool {
	relative, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

// HelperRequestForPrepared creates the bounded private handoff used by root
// composition after restart consent.
func HelperRequestForPrepared(
	prepared PreparedApplicationUnit,
	parentPID int,
	parentExitTimeout time.Duration,
	recoveryPath string,
) HelperRequest {
	if parentExitTimeout <= 0 {
		parentExitTimeout = defaultParentExitTimeout
	}
	return HelperRequest{
		AttemptID:         prepared.AttemptID,
		ExpectedVersion:   prepared.Version,
		ParentPID:         parentPID,
		ParentExitTimeout: parentExitTimeout,
		RecoveryPath:      recoveryPath,
		Prepared:          prepared,
	}
}
