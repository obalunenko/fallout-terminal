package update

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"sync"
	"syscall"
	"time"
)

const developmentVersion = "development"

const (
	defaultCheckTimeout = 2 * time.Minute
	maximumCheckTimeout = 5 * time.Minute
)

var errCheckTimeout = errors.New("application update check timed out")

const (
	commandErrorAttemptRequired = "The update request is missing its attempt identifier."
	commandErrorAttemptStale    = "The update request no longer matches the active attempt."
	commandErrorOfferState      = "The update offer is no longer available."
	commandErrorOfferDecision   = "The update offer decision is not supported."
	commandErrorPrepareMissing  = "Update preparation is not available."
	commandErrorRestartState    = "The prepared update is no longer awaiting restart."
	commandErrorRestartDecision = "The update restart decision is not supported."
	commandErrorRestartMissing  = "Update restart is not available."

	checkFailureMessage         = "Unable to check for application updates."
	checkRecoveryAction         = "Continue using this version and try again on the next launch."
	prepareFailureMessage       = "Unable to prepare the application update."
	prepareRecoveryAction       = "Continue using this version and try the update again on the next launch."
	stageSpaceFailureMessage    = "The application update could not be staged because the installation volume has insufficient space."
	stageSpaceRecoveryAction    = "Free space on the installation volume, then try the update again."
	stageReadOnlyFailureMessage = "The application update could not be staged because the installation location is not writable."
	stageReadOnlyRecoveryAction = "Use a writable installation location or correct its permissions, then try the update again."
	restartFailureMessage       = "Unable to restart into the prepared application update."
	restartRecoveryAction       = "Continue using this version and try restarting the update again."
)

// CheckFunc discovers the single eligible update candidate for this launch.
// A nil candidate means the installed release is current.
type CheckFunc func(context.Context) (*UpdateCandidate, error)

// PrepareFunc prepares an accepted candidate and reports externally visible
// download, verification, and staging progress. The returned unit must belong
// to the active attempt and candidate before it can be retained for restart.
type PrepareFunc func(
	context.Context,
	UpdateCandidate,
	func(UpdateState, UpdateProgress),
) (PreparedApplicationUnit, error)

// RestartFunc hands a prepared application unit to the replacement owner.
type RestartFunc func(context.Context, PreparedApplicationUnit) error

// PublishFunc receives each externally visible snapshot revision.
type PublishFunc func(UpdateSnapshot)

// IDFunc returns an opaque launch-scoped attempt identifier.
type IDFunc func() string

// ManagerConfig supplies framework-independent update dependencies.
type ManagerConfig struct {
	InstalledVersion string
	Packaged         bool
	CheckTimeout     time.Duration
	Check            CheckFunc
	Prepare          PrepareFunc
	Restart          RestartFunc
	Publish          PublishFunc
	IDs              IDFunc
	InitialFailure   *UpdateFailure
}

// Manager owns one launch-scoped application update attempt.
type Manager struct {
	mu sync.Mutex

	checkOnce sync.Once
	operation bool

	check CheckFunc
	// checkTimeout bounds provider discovery independently of whether the
	// caller supplied a deadline. It is capped during construction so a
	// configuration error cannot leave launch discovery running indefinitely.
	checkTimeout time.Duration
	prepare      PrepareFunc
	restart      RestartFunc
	publish      PublishFunc
	ids          IDFunc

	snapshot  UpdateSnapshot
	candidate *UpdateCandidate
	// deferredVersion is intentionally launch scoped. It records the offer that
	// must not be surfaced again after the backend candidate has been released.
	deferredVersion string
	prepared        *PreparedApplicationUnit
}

// NewManager creates an idle packaged-release manager or a permanently
// disabled manager for development and unpackaged builds.
func NewManager(config ManagerConfig) (*Manager, error) {
	state := UpdateStateDisabled
	revision := uint64(0)
	failure := UpdateFailure{}
	eligible := config.Packaged && config.InstalledVersion != "" && config.InstalledVersion != developmentVersion
	if eligible {
		if config.InitialFailure != nil {
			if !validInitialFailure(*config.InitialFailure) {
				return nil, errors.New("initial update failure is invalid")
			}
			state = UpdateStateFailed
			revision = 1
			failure = *config.InitialFailure
		} else {
			if config.Check == nil {
				return nil, errors.New("update check dependency is required for packaged releases")
			}
			if config.IDs == nil {
				return nil, errors.New("update attempt identifier dependency is required for packaged releases")
			}
			state = UpdateStateIdle
		}
	}

	return &Manager{
		check:        config.Check,
		checkTimeout: boundedCheckTimeout(config.CheckTimeout),
		prepare:      config.Prepare,
		restart:      config.Restart,
		publish:      config.Publish,
		ids:          config.IDs,
		snapshot: UpdateSnapshot{
			Revision:         revision,
			State:            state,
			InstalledVersion: config.InstalledVersion,
			Failure:          failure,
		},
	}, nil
}

// Snapshot returns a detached copy of the authoritative visible state.
func (manager *Manager) Snapshot() UpdateSnapshot {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	return manager.snapshot
}

// Status returns immediately and arms the packaged-release check at most once.
func (manager *Manager) Status(ctx context.Context) UpdateSnapshot {
	manager.checkOnce.Do(func() {
		manager.mu.Lock()
		if manager.snapshot.State != UpdateStateIdle {
			manager.mu.Unlock()
			return
		}

		manager.snapshot.AttemptID = manager.ids()
		snapshot := manager.transitionLocked(UpdateStateChecking)
		manager.mu.Unlock()

		manager.publishSnapshot(snapshot)
		checkContext, cancel := context.WithTimeoutCause(
			nonNilContext(ctx), manager.checkTimeout, errCheckTimeout,
		)
		go manager.runCheck(checkContext, cancel)
	})

	return manager.Snapshot()
}

// ResolveOffer applies a decision only to the current available attempt.
func (manager *Manager) ResolveOffer(ctx context.Context, attemptID string, decision OfferDecision) CommandResult {
	manager.mu.Lock()
	if err := manager.validateAttemptLocked(attemptID); err != "" {
		result := manager.failureResultLocked(err)
		manager.mu.Unlock()
		return result
	}
	if manager.snapshot.State != UpdateStateAvailable {
		result := manager.failureResultLocked(commandErrorOfferState)
		manager.mu.Unlock()
		return result
	}
	if !decision.Valid() {
		result := manager.failureResultLocked(commandErrorOfferDecision)
		manager.mu.Unlock()
		return result
	}

	if decision == OfferDecisionDefer {
		if manager.candidate != nil {
			manager.deferredVersion = manager.candidate.Version
			manager.candidate = nil
		}
		snapshot := manager.transitionLocked(UpdateStateDeferred)
		manager.mu.Unlock()
		manager.publishSnapshot(snapshot)
		return CommandResult{OK: true, Snapshot: snapshot}
	}
	if manager.operation || manager.prepare == nil {
		message := commandErrorOfferState
		if manager.prepare == nil {
			message = commandErrorPrepareMissing
		}
		result := manager.failureResultLocked(message)
		manager.mu.Unlock()
		return result
	}

	manager.operation = true
	manager.prepared = nil
	manager.snapshot.Progress = UpdateProgress{}
	manager.snapshot.Failure = UpdateFailure{}
	candidate := *manager.candidate
	snapshot := manager.transitionLocked(UpdateStateDownloading)
	manager.mu.Unlock()
	manager.publishSnapshot(snapshot)

	// Serialize provider callbacks with completion so revisions are published in
	// order and a callback cannot race the ready-to-restart transition.
	var reportMu sync.Mutex
	report := func(state UpdateState, progress UpdateProgress) {
		reportMu.Lock()
		defer reportMu.Unlock()
		manager.reportPreparation(attemptID, state, progress)
	}
	prepared, err := manager.prepare(nonNilContext(ctx), candidate, report)
	reportMu.Lock()
	result := manager.completePreparation(attemptID, candidate, prepared, err)
	reportMu.Unlock()
	return result
}

// ResolveRestart applies a decision only to the current prepared attempt.
// Postponing deliberately leaves the visible state and revision unchanged.
func (manager *Manager) ResolveRestart(ctx context.Context, attemptID string, decision RestartDecision) CommandResult {
	manager.mu.Lock()
	if err := manager.validateAttemptLocked(attemptID); err != "" {
		result := manager.failureResultLocked(err)
		manager.mu.Unlock()
		return result
	}
	if manager.snapshot.State != UpdateStateReadyToRestart {
		result := manager.failureResultLocked(commandErrorRestartState)
		manager.mu.Unlock()
		return result
	}
	if !decision.Valid() {
		result := manager.failureResultLocked(commandErrorRestartDecision)
		manager.mu.Unlock()
		return result
	}
	if decision == RestartDecisionPostpone {
		result := CommandResult{OK: true, Snapshot: manager.snapshot}
		manager.mu.Unlock()
		return result
	}
	if manager.operation || manager.restart == nil || manager.prepared == nil {
		result := manager.failureResultLocked(commandErrorRestartMissing)
		manager.mu.Unlock()
		return result
	}

	manager.operation = true
	prepared := *manager.prepared
	snapshot := manager.transitionLocked(UpdateStateApplying)
	manager.mu.Unlock()
	manager.publishSnapshot(snapshot)

	err := manager.restart(nonNilContext(ctx), prepared)
	return manager.completeRestart(attemptID, err)
}

func (manager *Manager) runCheck(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	candidate, err := manager.check(ctx)

	manager.mu.Lock()
	if manager.snapshot.State != UpdateStateChecking {
		manager.mu.Unlock()
		return
	}

	var snapshot UpdateSnapshot
	switch {
	case err != nil:
		manager.snapshot.Failure = UpdateFailure{
			Stage: FailureStageCheck, Message: checkFailureMessage, RecoveryAction: checkRecoveryAction,
		}
		snapshot = manager.transitionLocked(UpdateStateFailed)
	case candidate == nil:
		snapshot = manager.transitionLocked(UpdateStateCurrent)
	default:
		candidateCopy := *candidate
		manager.candidate = &candidateCopy
		manager.snapshot.AvailableVersion = candidateCopy.Version
		manager.snapshot.ReleaseNotes = candidateCopy.ReleaseNotes
		snapshot = manager.transitionLocked(UpdateStateAvailable)
	}
	manager.mu.Unlock()

	manager.publishSnapshot(snapshot)
}

func (manager *Manager) reportPreparation(
	attemptID string,
	state UpdateState,
	progress UpdateProgress,
) {
	manager.mu.Lock()
	if !manager.operation || manager.snapshot.AttemptID != attemptID ||
		!validPreparationTransition(manager.snapshot.State, state) ||
		!validProgressAdvance(manager.snapshot.Progress, progress) {
		manager.mu.Unlock()
		return
	}
	if manager.snapshot.State == state && manager.snapshot.Progress == progress {
		manager.mu.Unlock()
		return
	}

	manager.snapshot.Progress = progress
	snapshot := manager.transitionLocked(state)
	manager.mu.Unlock()
	manager.publishSnapshot(snapshot)
}

func (manager *Manager) completePreparation(
	attemptID string,
	candidate UpdateCandidate,
	prepared PreparedApplicationUnit,
	prepareErr error,
) CommandResult {
	manager.mu.Lock()
	manager.operation = false
	if manager.snapshot.AttemptID != attemptID || !preparationState(manager.snapshot.State) {
		result := manager.failureResultLocked(commandErrorOfferState)
		manager.mu.Unlock()
		return result
	}
	if prepareErr == nil && manager.validPreparedUnitLocked(attemptID, candidate, prepared) {
		preparedCopy := prepared
		manager.prepared = &preparedCopy
		snapshot := manager.transitionLocked(UpdateStateReadyToRestart)
		manager.mu.Unlock()
		manager.publishSnapshot(snapshot)
		return CommandResult{OK: true, Snapshot: snapshot}
	}

	manager.snapshot.Failure = sanitizedPreparationFailure(
		manager.snapshot.State, prepareErr, prepareErr == nil,
	)
	snapshot := manager.transitionLocked(UpdateStateFailed)
	manager.mu.Unlock()
	manager.publishSnapshot(snapshot)
	return CommandResult{Error: prepareFailureMessage, Snapshot: snapshot}
}

func (manager *Manager) validPreparedUnitLocked(
	attemptID string,
	candidate UpdateCandidate,
	prepared PreparedApplicationUnit,
) bool {
	if manager.candidate == nil || manager.candidate.Version != candidate.Version ||
		prepared.AttemptID != attemptID || prepared.Version != candidate.Version ||
		prepared.Target.OS == "" || prepared.Target.Arch == "" {
		return false
	}

	candidateTarget := candidate.Artifact.Target
	activeTarget := manager.candidate.Artifact.Target
	if candidateTarget != activeTarget {
		return false
	}
	if candidateTarget != (Target{}) && prepared.Target != candidateTarget {
		return false
	}
	return true
}

func (manager *Manager) completeRestart(attemptID string, restartErr error) CommandResult {
	manager.mu.Lock()
	manager.operation = false
	if manager.snapshot.AttemptID != attemptID || manager.snapshot.State != UpdateStateApplying {
		result := manager.failureResultLocked(commandErrorRestartState)
		manager.mu.Unlock()
		return result
	}
	if restartErr == nil {
		result := CommandResult{OK: true, Snapshot: manager.snapshot}
		manager.mu.Unlock()
		return result
	}

	manager.snapshot.Failure = UpdateFailure{
		Stage: FailureStageApply, Message: restartFailureMessage, RecoveryAction: restartRecoveryAction,
	}
	snapshot := manager.transitionLocked(UpdateStateFailed)
	manager.mu.Unlock()
	manager.publishSnapshot(snapshot)
	return CommandResult{Error: restartFailureMessage, Snapshot: snapshot}
}

func (manager *Manager) validateAttemptLocked(attemptID string) string {
	if attemptID == "" {
		return commandErrorAttemptRequired
	}
	if attemptID != manager.snapshot.AttemptID {
		return commandErrorAttemptStale
	}
	return ""
}

func (manager *Manager) failureResultLocked(message string) CommandResult {
	return CommandResult{Error: message, Snapshot: manager.snapshot}
}

func (manager *Manager) transitionLocked(state UpdateState) UpdateSnapshot {
	manager.snapshot.Revision++
	manager.snapshot.State = state
	return manager.snapshot
}

func (manager *Manager) publishSnapshot(snapshot UpdateSnapshot) {
	if manager.publish != nil {
		manager.publish(snapshot)
	}
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func boundedCheckTimeout(configured time.Duration) time.Duration {
	if configured <= 0 {
		return defaultCheckTimeout
	}
	if configured > maximumCheckTimeout {
		return maximumCheckTimeout
	}
	return configured
}

func preparationState(state UpdateState) bool {
	return preparationStateRank(state) >= 0
}

func validPreparationTransition(current, next UpdateState) bool {
	currentRank := preparationStateRank(current)
	nextRank := preparationStateRank(next)
	return currentRank >= 0 && nextRank >= currentRank
}

func preparationStateRank(state UpdateState) int {
	switch state {
	case UpdateStateDownloading:
		return 0
	case UpdateStateVerifying:
		return 1
	case UpdateStateStaging:
		return 2
	default:
		return -1
	}
}

func validProgressAdvance(current, next UpdateProgress) bool {
	if next.BytesDownloaded < current.BytesDownloaded {
		return false
	}
	if current.DownloadSizeKnown && !next.DownloadSizeKnown {
		return false
	}
	if next.DownloadSizeKnown {
		if next.DownloadSize == 0 || next.BytesDownloaded > next.DownloadSize {
			return false
		}
		if current.DownloadSizeKnown && next.DownloadSize < current.DownloadSize {
			return false
		}
	}
	return true
}

func preparationFailureStage(state UpdateState, invalidPreparedUnit bool) FailureStage {
	if invalidPreparedUnit {
		return FailureStageStage
	}
	switch state {
	case UpdateStateVerifying:
		return FailureStageVerify
	case UpdateStateStaging:
		return FailureStageStage
	default:
		return FailureStageDownload
	}
}

func sanitizedPreparationFailure(
	state UpdateState,
	prepareErr error,
	invalidPreparedUnit bool,
) UpdateFailure {
	stage := preparationFailureStage(state, invalidPreparedUnit)
	if stage == FailureStageStage {
		switch {
		case insufficientSpaceError(prepareErr):
			return UpdateFailure{
				Stage: stage, Message: stageSpaceFailureMessage, RecoveryAction: stageSpaceRecoveryAction,
			}
		case readOnlyLocationError(prepareErr):
			return UpdateFailure{
				Stage: stage, Message: stageReadOnlyFailureMessage, RecoveryAction: stageReadOnlyRecoveryAction,
			}
		}
	}
	return UpdateFailure{
		Stage: stage, Message: prepareFailureMessage, RecoveryAction: prepareRecoveryAction,
	}
}

func insufficientSpaceError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.ENOSPC) || errorChainContainsText(
		err, "no space left", "insufficient space", "disk full",
	)
}

func readOnlyLocationError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, fs.ErrPermission) || errorChainContainsText(
		err, "read-only", "read only", "permission denied",
	)
}

func errorChainContainsText(err error, fragments ...string) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range fragments {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, nested := range joined.Unwrap() {
			if errorChainContainsText(nested, fragments...) {
				return true
			}
		}
		return false
	}
	return errorChainContainsText(errors.Unwrap(err), fragments...)
}

func validInitialFailure(failure UpdateFailure) bool {
	return failure.Stage.Valid() && strings.TrimSpace(failure.Message) != "" &&
		strings.TrimSpace(failure.RecoveryAction) != ""
}
