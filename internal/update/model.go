// Package update owns the framework-independent application update lifecycle.
package update

import (
	"crypto/sha256"
	"time"
)

const (
	// RecoverySchemaVersion is the only recovery-journal schema understood by
	// the application and replacement helper.
	RecoverySchemaVersion uint32 = 1

	ReleaseAssetStateUploaded = "uploaded"
	DigestAlgorithmSHA256     = "sha256"
)

// UpdateState identifies one externally observable update lifecycle state.
type UpdateState string

const (
	UpdateStateDisabled       UpdateState = "disabled"
	UpdateStateIdle           UpdateState = "idle"
	UpdateStateChecking       UpdateState = "checking"
	UpdateStateCurrent        UpdateState = "current"
	UpdateStateAvailable      UpdateState = "available"
	UpdateStateDeferred       UpdateState = "deferred"
	UpdateStateDownloading    UpdateState = "downloading"
	UpdateStateVerifying      UpdateState = "verifying"
	UpdateStateStaging        UpdateState = "staging"
	UpdateStateReadyToRestart UpdateState = "ready-to-restart"
	UpdateStateApplying       UpdateState = "applying"
	UpdateStateFailed         UpdateState = "failed"
)

// Valid reports whether state belongs to the update lifecycle vocabulary.
func (state UpdateState) Valid() bool {
	switch state {
	case UpdateStateDisabled, UpdateStateIdle, UpdateStateChecking, UpdateStateCurrent,
		UpdateStateAvailable, UpdateStateDeferred, UpdateStateDownloading, UpdateStateVerifying,
		UpdateStateStaging, UpdateStateReadyToRestart, UpdateStateApplying, UpdateStateFailed:
		return true
	default:
		return false
	}
}

// FailureStage identifies the operation that produced a safe update failure.
type FailureStage string

const (
	FailureStageCheck    FailureStage = "check"
	FailureStageDownload FailureStage = "download"
	FailureStageVerify   FailureStage = "verify"
	FailureStageStage    FailureStage = "stage"
	FailureStageApply    FailureStage = "apply"
	FailureStageRelaunch FailureStage = "relaunch"
	FailureStageRecovery FailureStage = "recovery"
)

// Valid reports whether stage identifies an approved failure boundary.
func (stage FailureStage) Valid() bool {
	switch stage {
	case FailureStageCheck, FailureStageDownload, FailureStageVerify, FailureStageStage,
		FailureStageApply, FailureStageRelaunch, FailureStageRecovery:
		return true
	default:
		return false
	}
}

// Channel identifies the release stream accepted by an installed version.
type Channel string

const (
	ChannelStable     Channel = "stable"
	ChannelPrerelease Channel = "prerelease"
)

// Valid reports whether channel is an approved release stream.
func (channel Channel) Valid() bool {
	return channel == ChannelStable || channel == ChannelPrerelease
}

// RecoveryState identifies replacement progress persisted across launches.
type RecoveryState string

const (
	RecoveryStateApplying RecoveryState = "applying"
	RecoveryStateApplied  RecoveryState = "applied"
	RecoveryStateFailed   RecoveryState = "failed"
)

// Valid reports whether state can be persisted in the recovery journal.
func (state RecoveryState) Valid() bool {
	return state == RecoveryStateApplying || state == RecoveryStateApplied || state == RecoveryStateFailed
}

// Target is the exact operating-system and architecture pair for an update
// artifact and prepared application unit.
type Target struct {
	OS   string
	Arch string
}

// ReleaseAsset is backend-only provider metadata for one governed archive.
// Digest contains decoded SHA-256 evidence; DownloadURL must never be projected
// into an Overseer-facing snapshot.
type ReleaseAsset struct {
	ID              int64
	Name            string
	State           string
	Size            int64
	DigestAlgorithm string
	Digest          [sha256.Size]byte
	DownloadURL     string
	Target          Target
}

// UpdateCandidate is a complete release selected for the running target.
type UpdateCandidate struct {
	Version      string
	Channel      Channel
	Name         string
	ReleaseNotes string
	PublishedAt  time.Time
	Artifact     ReleaseAsset
}

// UpdateProgress is monotonic transfer progress. DownloadSizeKnown separates
// an unknown provider size from a known empty value.
type UpdateProgress struct {
	BytesDownloaded   uint64
	DownloadSize      uint64
	DownloadSizeKnown bool
}

// UpdateFailure is safe to log and project to the Overseer.
type UpdateFailure struct {
	Stage          FailureStage
	Message        string
	RecoveryAction string
}

// UpdateSnapshot is the detached, Overseer-safe view of current update state.
// Empty optional strings and a zero failure stage mean the corresponding value
// is absent.
type UpdateSnapshot struct {
	Revision         uint64
	AttemptID        string
	State            UpdateState
	InstalledVersion string
	AvailableVersion string
	ReleaseNotes     string
	Progress         UpdateProgress
	Failure          UpdateFailure
}

// OfferDecision is the only decision accepted while an update is available.
type OfferDecision string

const (
	OfferDecisionAccept OfferDecision = "accept"
	OfferDecisionDefer  OfferDecision = "defer"
)

// Valid reports whether decision belongs to the offer decision vocabulary.
func (decision OfferDecision) Valid() bool {
	return decision == OfferDecisionAccept || decision == OfferDecisionDefer
}

// RestartDecision is the only decision accepted after an update is prepared.
type RestartDecision string

const (
	RestartDecisionRestart  RestartDecision = "restart"
	RestartDecisionPostpone RestartDecision = "postpone"
)

// Valid reports whether decision belongs to the restart decision vocabulary.
func (decision RestartDecision) Valid() bool {
	return decision == RestartDecisionRestart || decision == RestartDecisionPostpone
}

// CommandResult returns a detached authoritative snapshot with a safe command
// outcome. Error never contains a provider error or backend path.
type CommandResult struct {
	OK       bool
	Error    string
	Snapshot UpdateSnapshot
}

// PreparedApplicationUnit is backend-only metadata for one validated adjacent
// stage. InstalledUnit and StagedUnit must never be logged or projected.
type PreparedApplicationUnit struct {
	AttemptID          string
	Version            string
	Target             Target
	InstalledUnit      string
	StagedUnit         string
	LaunchRelativePath string
}

// UpdateRecoveryRecord is the non-sensitive replacement journal consumed by
// the next normal application launch.
type UpdateRecoveryRecord struct {
	SchemaVersion   uint32        `json:"schemaVersion"`
	AttemptID       string        `json:"attemptID"`
	ExpectedVersion string        `json:"expectedVersion"`
	State           RecoveryState `json:"state"`
	FailedStage     FailureStage  `json:"failedStage,omitempty"`
	Message         string        `json:"message,omitempty"`
	RecoveryAction  string        `json:"recoveryAction,omitempty"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}
