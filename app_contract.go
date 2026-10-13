package main

import (
	"fmt"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	persistencev1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/persistence/v1"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	privatev1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/private/v1"
	sessionservice "github.com/obalunenko/Fallout-Terminal/v2/internal/session"
	tunnelservice "github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	updateservice "github.com/obalunenko/Fallout-Terminal/v2/internal/update"
)

// The private protobuf graph governs trusted desktop semantics only. These
// adapters are invoked inside App while Wails continues carrying the existing
// native DTOs; no protobuf bytes, ProtoJSON, or generic envelope crosses Wails.

func applicationUpdateSnapshotToPrivate(snapshot updateservice.UpdateSnapshot) *privatev1.ApplicationUpdateSnapshot {
	semantic := &privatev1.ApplicationUpdateSnapshot{
		Revision:         snapshot.Revision,
		State:            applicationUpdateStateToPrivate(snapshot.State),
		InstalledVersion: snapshot.InstalledVersion,
		BytesDownloaded:  snapshot.Progress.BytesDownloaded,
		FailedStage:      applicationUpdateFailureStageToPrivate(snapshot.Failure.Stage),
	}
	if snapshot.AttemptID != "" {
		semantic.AttemptId = &snapshot.AttemptID
	}
	if snapshot.AvailableVersion != "" {
		semantic.AvailableVersion = &snapshot.AvailableVersion
	}
	if snapshot.ReleaseNotes != "" {
		semantic.ReleaseNotes = &snapshot.ReleaseNotes
	}
	if snapshot.Progress.DownloadSizeKnown {
		semantic.DownloadSize = &snapshot.Progress.DownloadSize
	}
	if snapshot.Failure.Message != "" {
		semantic.ErrorMessage = &snapshot.Failure.Message
	}
	if snapshot.Failure.RecoveryAction != "" {
		semantic.RecoveryAction = &snapshot.Failure.RecoveryAction
	}

	return semantic
}

func applicationUpdateSnapshotFromPrivate(snapshot *privatev1.ApplicationUpdateSnapshot) ApplicationUpdateSnapshot {
	if snapshot == nil {
		return ApplicationUpdateSnapshot{}
	}

	native := ApplicationUpdateSnapshot{
		Revision:         snapshot.GetRevision(),
		AttemptID:        snapshot.GetAttemptId(),
		State:            applicationUpdateStateFromPrivate(snapshot.GetState()),
		InstalledVersion: snapshot.GetInstalledVersion(),
		AvailableVersion: snapshot.GetAvailableVersion(),
		ReleaseNotes:     snapshot.GetReleaseNotes(),
		BytesDownloaded:  snapshot.GetBytesDownloaded(),
		FailedStage:      applicationUpdateFailureStageFromPrivate(snapshot.GetFailedStage()),
		ErrorMessage:     snapshot.GetErrorMessage(),
		RecoveryAction:   snapshot.GetRecoveryAction(),
	}
	if snapshot.DownloadSize != nil {
		downloadSize := snapshot.GetDownloadSize()
		native.DownloadSize = &downloadSize
	}

	return native
}

func routeApplicationUpdateOfferDecisionRequest(payload ApplicationUpdateOfferDecisionPayload) (ApplicationUpdateOfferDecisionPayload, error) {
	decision := applicationUpdateOfferDecisionToPrivate(payload.Decision)
	if decision == privatev1.ApplicationUpdateOfferDecision_APPLICATION_UPDATE_OFFER_DECISION_UNSPECIFIED {
		return ApplicationUpdateOfferDecisionPayload{}, fmt.Errorf("unsupported application update offer decision")
	}
	semantic := &privatev1.ResolveApplicationUpdateOfferRequest{AttemptId: payload.AttemptID, Decision: decision}

	return ApplicationUpdateOfferDecisionPayload{
		AttemptID: semantic.GetAttemptId(),
		Decision:  applicationUpdateOfferDecisionFromPrivate(semantic.GetDecision()),
	}, nil
}

func routeApplicationUpdateRestartDecisionRequest(payload ApplicationUpdateRestartDecisionPayload) (ApplicationUpdateRestartDecisionPayload, error) {
	decision := applicationUpdateRestartDecisionToPrivate(payload.Decision)
	if decision == privatev1.ApplicationUpdateRestartDecision_APPLICATION_UPDATE_RESTART_DECISION_UNSPECIFIED {
		return ApplicationUpdateRestartDecisionPayload{}, fmt.Errorf("unsupported application update restart decision")
	}
	semantic := &privatev1.ResolveApplicationUpdateRestartRequest{AttemptId: payload.AttemptID, Decision: decision}

	return ApplicationUpdateRestartDecisionPayload{
		AttemptID: semantic.GetAttemptId(),
		Decision:  applicationUpdateRestartDecisionFromPrivate(semantic.GetDecision()),
	}, nil
}

func applicationUpdateCommandResultToPrivate(result ApplicationUpdateCommandResult) *privatev1.ApplicationUpdateCommandResult {
	semantic := &privatev1.ApplicationUpdateCommandResult{
		Ok:       result.OK,
		Snapshot: applicationUpdateNativeSnapshotToPrivate(result.Snapshot),
	}
	if result.Error != "" {
		semantic.Error = &result.Error
	}

	return semantic
}

func applicationUpdateCommandResultFromPrivate(result *privatev1.ApplicationUpdateCommandResult) ApplicationUpdateCommandResult {
	if result == nil {
		return ApplicationUpdateCommandResult{}
	}

	return ApplicationUpdateCommandResult{
		OK:       result.GetOk(),
		Error:    result.GetError(),
		Snapshot: applicationUpdateSnapshotFromPrivate(result.GetSnapshot()),
	}
}

func applicationUpdateNativeSnapshotToPrivate(snapshot ApplicationUpdateSnapshot) *privatev1.ApplicationUpdateSnapshot {
	semantic := &privatev1.ApplicationUpdateSnapshot{
		Revision:         snapshot.Revision,
		State:            applicationUpdateStateToPrivate(updateservice.UpdateState(snapshot.State)),
		InstalledVersion: snapshot.InstalledVersion,
		BytesDownloaded:  snapshot.BytesDownloaded,
		FailedStage:      applicationUpdateFailureStageToPrivate(updateservice.FailureStage(snapshot.FailedStage)),
	}
	if snapshot.AttemptID != "" {
		semantic.AttemptId = &snapshot.AttemptID
	}
	if snapshot.AvailableVersion != "" {
		semantic.AvailableVersion = &snapshot.AvailableVersion
	}
	if snapshot.ReleaseNotes != "" {
		semantic.ReleaseNotes = &snapshot.ReleaseNotes
	}
	if snapshot.DownloadSize != nil {
		downloadSize := *snapshot.DownloadSize
		semantic.DownloadSize = &downloadSize
	}
	if snapshot.ErrorMessage != "" {
		semantic.ErrorMessage = &snapshot.ErrorMessage
	}
	if snapshot.RecoveryAction != "" {
		semantic.RecoveryAction = &snapshot.RecoveryAction
	}

	return semantic
}

func applicationUpdateStateToPrivate(state updateservice.UpdateState) privatev1.ApplicationUpdateState {
	switch state {
	case updateservice.UpdateStateDisabled:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_DISABLED
	case updateservice.UpdateStateIdle:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_IDLE
	case updateservice.UpdateStateChecking:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_CHECKING
	case updateservice.UpdateStateCurrent:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_CURRENT
	case updateservice.UpdateStateAvailable:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_AVAILABLE
	case updateservice.UpdateStateDeferred:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_DEFERRED
	case updateservice.UpdateStateDownloading:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_DOWNLOADING
	case updateservice.UpdateStateVerifying:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_VERIFYING
	case updateservice.UpdateStateStaging:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_STAGING
	case updateservice.UpdateStateReadyToRestart:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_READY_TO_RESTART
	case updateservice.UpdateStateApplying:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_APPLYING
	case updateservice.UpdateStateFailed:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_FAILED
	default:
		return privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_UNSPECIFIED
	}
}

func applicationUpdateStateFromPrivate(state privatev1.ApplicationUpdateState) string {
	switch state {
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_DISABLED:
		return string(updateservice.UpdateStateDisabled)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_IDLE:
		return string(updateservice.UpdateStateIdle)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_CHECKING:
		return string(updateservice.UpdateStateChecking)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_CURRENT:
		return string(updateservice.UpdateStateCurrent)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_AVAILABLE:
		return string(updateservice.UpdateStateAvailable)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_DEFERRED:
		return string(updateservice.UpdateStateDeferred)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_DOWNLOADING:
		return string(updateservice.UpdateStateDownloading)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_VERIFYING:
		return string(updateservice.UpdateStateVerifying)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_STAGING:
		return string(updateservice.UpdateStateStaging)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_READY_TO_RESTART:
		return string(updateservice.UpdateStateReadyToRestart)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_APPLYING:
		return string(updateservice.UpdateStateApplying)
	case privatev1.ApplicationUpdateState_APPLICATION_UPDATE_STATE_FAILED:
		return string(updateservice.UpdateStateFailed)
	default:
		return ""
	}
}

func applicationUpdateFailureStageToPrivate(stage updateservice.FailureStage) privatev1.ApplicationUpdateFailureStage {
	switch stage {
	case updateservice.FailureStageCheck:
		return privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_CHECK
	case updateservice.FailureStageDownload:
		return privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_DOWNLOAD
	case updateservice.FailureStageVerify:
		return privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_VERIFY
	case updateservice.FailureStageStage:
		return privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_STAGE
	case updateservice.FailureStageApply:
		return privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_APPLY
	case updateservice.FailureStageRelaunch:
		return privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_RELAUNCH
	case updateservice.FailureStageRecovery:
		return privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_RECOVERY
	default:
		return privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_UNSPECIFIED
	}
}

func applicationUpdateFailureStageFromPrivate(stage privatev1.ApplicationUpdateFailureStage) string {
	switch stage {
	case privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_CHECK:
		return string(updateservice.FailureStageCheck)
	case privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_DOWNLOAD:
		return string(updateservice.FailureStageDownload)
	case privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_VERIFY:
		return string(updateservice.FailureStageVerify)
	case privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_STAGE:
		return string(updateservice.FailureStageStage)
	case privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_APPLY:
		return string(updateservice.FailureStageApply)
	case privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_RELAUNCH:
		return string(updateservice.FailureStageRelaunch)
	case privatev1.ApplicationUpdateFailureStage_APPLICATION_UPDATE_FAILURE_STAGE_RECOVERY:
		return string(updateservice.FailureStageRecovery)
	default:
		return ""
	}
}

func applicationUpdateOfferDecisionFromPrivate(decision privatev1.ApplicationUpdateOfferDecision) string {
	switch decision {
	case privatev1.ApplicationUpdateOfferDecision_APPLICATION_UPDATE_OFFER_DECISION_ACCEPT:
		return "accept"
	case privatev1.ApplicationUpdateOfferDecision_APPLICATION_UPDATE_OFFER_DECISION_DEFER:
		return "defer"
	default:
		return ""
	}
}

func applicationUpdateOfferDecisionToPrivate(decision string) privatev1.ApplicationUpdateOfferDecision {
	switch decision {
	case "accept":
		return privatev1.ApplicationUpdateOfferDecision_APPLICATION_UPDATE_OFFER_DECISION_ACCEPT
	case "defer":
		return privatev1.ApplicationUpdateOfferDecision_APPLICATION_UPDATE_OFFER_DECISION_DEFER
	default:
		return privatev1.ApplicationUpdateOfferDecision_APPLICATION_UPDATE_OFFER_DECISION_UNSPECIFIED
	}
}

func applicationUpdateRestartDecisionFromPrivate(decision privatev1.ApplicationUpdateRestartDecision) string {
	switch decision {
	case privatev1.ApplicationUpdateRestartDecision_APPLICATION_UPDATE_RESTART_DECISION_RESTART:
		return "restart"
	case privatev1.ApplicationUpdateRestartDecision_APPLICATION_UPDATE_RESTART_DECISION_POSTPONE:
		return "postpone"
	default:
		return ""
	}
}

func applicationUpdateRestartDecisionToPrivate(decision string) privatev1.ApplicationUpdateRestartDecision {
	switch decision {
	case "restart":
		return privatev1.ApplicationUpdateRestartDecision_APPLICATION_UPDATE_RESTART_DECISION_RESTART
	case "postpone":
		return privatev1.ApplicationUpdateRestartDecision_APPLICATION_UPDATE_RESTART_DECISION_POSTPONE
	default:
		return privatev1.ApplicationUpdateRestartDecision_APPLICATION_UPDATE_RESTART_DECISION_UNSPECIFIED
	}
}

func routeSavePublicAccessSettingsRequest(payload SavePublicAccessSettingsPayload) (SavePublicAccessSettingsPayload, error) {
	if payload.ReplacementProviderToken != "" && payload.DeleteProviderToken {
		return SavePublicAccessSettingsPayload{}, fmt.Errorf("provider credential change is ambiguous")
	}
	if payload.ReplacementPlayerPassword != "" && payload.DeletePlayerPassword {
		return SavePublicAccessSettingsPayload{}, fmt.Errorf("player credential change is ambiguous")
	}
	semantic := &privatev1.SavePublicAccessSettingsRequest{
		ExpectedRevision: payload.ExpectedRevision, EnabledPreference: payload.EnabledPreference,
		Username: payload.Username,
	}
	if payload.ReservedDomain != "" {
		domain := payload.ReservedDomain
		semantic.ReservedDomain = &domain
	}
	switch {
	case payload.ReplacementProviderToken != "":
		semantic.ProviderTokenChange = &privatev1.SavePublicAccessSettingsRequest_ReplacementProviderToken{ReplacementProviderToken: payload.ReplacementProviderToken}
	case payload.DeleteProviderToken:
		semantic.ProviderTokenChange = &privatev1.SavePublicAccessSettingsRequest_DeleteProviderToken{DeleteProviderToken: true}
	}
	switch {
	case payload.ReplacementPlayerPassword != "":
		semantic.PlayerPasswordChange = &privatev1.SavePublicAccessSettingsRequest_ReplacementPlayerPassword{ReplacementPlayerPassword: payload.ReplacementPlayerPassword}
	case payload.DeletePlayerPassword:
		semantic.PlayerPasswordChange = &privatev1.SavePublicAccessSettingsRequest_DeletePlayerPassword{DeletePlayerPassword: true}
	}
	routed := SavePublicAccessSettingsPayload{
		ExpectedRevision: semantic.GetExpectedRevision(), EnabledPreference: semantic.GetEnabledPreference(),
		ReservedDomain: semantic.GetReservedDomain(), Username: semantic.GetUsername(),
	}
	switch change := semantic.ProviderTokenChange.(type) {
	case nil:
	case *privatev1.SavePublicAccessSettingsRequest_ReplacementProviderToken:
		routed.ReplacementProviderToken = change.ReplacementProviderToken
	case *privatev1.SavePublicAccessSettingsRequest_DeleteProviderToken:
		routed.DeleteProviderToken = change.DeleteProviderToken
	default:
		return SavePublicAccessSettingsPayload{}, fmt.Errorf("unsupported provider credential change")
	}
	switch change := semantic.PlayerPasswordChange.(type) {
	case nil:
	case *privatev1.SavePublicAccessSettingsRequest_ReplacementPlayerPassword:
		routed.ReplacementPlayerPassword = change.ReplacementPlayerPassword
	case *privatev1.SavePublicAccessSettingsRequest_DeletePlayerPassword:
		routed.DeletePlayerPassword = change.DeletePlayerPassword
	default:
		return SavePublicAccessSettingsPayload{}, fmt.Errorf("unsupported player credential change")
	}
	return routed, nil
}

func routePublicAccessCommandRequest(payload PublicAccessCommandPayload) PublicAccessCommandPayload {
	semantic := &privatev1.PublicAccessCommandRequest{ExpectedRevision: payload.ExpectedRevision}
	return PublicAccessCommandPayload{ExpectedRevision: semantic.GetExpectedRevision()}
}

func publicAccessSnapshotToPrivate(snapshot tunnelservice.PublicAccessSnapshot) *privatev1.PublicAccessSnapshot {
	return &privatev1.PublicAccessSnapshot{
		Preferences:            tunnelservice.PreferencesToProto(snapshot.Preferences),
		ProviderTokenPresence:  secretPresenceToPrivate(snapshot.ProviderTokenPresence),
		PlayerPasswordPresence: secretPresenceToPrivate(snapshot.PlayerPasswordPresence),
		Status:                 publicAccessStatusToPrivate(snapshot.Status),
	}
}

func routePublicAccessSnapshot(snapshot tunnelservice.PublicAccessSnapshot) PublicAccessSnapshot {
	semantic := (&privatev1.PublicAccessStatusEvent{Snapshot: publicAccessSnapshotToPrivate(snapshot)}).GetSnapshot()
	preferences, err := tunnelservice.PreferencesFromProto(semantic.GetPreferences())
	if err != nil {
		preferences = tunnelservice.DefaultPublicAccessPreferences()
	}
	return PublicAccessSnapshot{
		Preferences: PublicAccessPreferences{
			Version: preferences.Version, EnabledPreference: preferences.EnabledPreference,
			ReservedDomain: preferences.ReservedDomain, Username: preferences.Username,
			ProviderTokenPresentHint:  preferences.ProviderTokenPresentHint,
			PlayerPasswordPresentHint: preferences.PlayerPasswordPresentHint, Revision: preferences.Revision,
		},
		ProviderTokenPresence:  secretPresenceFromPrivate(semantic.GetProviderTokenPresence()),
		PlayerPasswordPresence: secretPresenceFromPrivate(semantic.GetPlayerPasswordPresence()),
		Status:                 publicAccessStatusFromPrivate(semantic.GetStatus()),
	}
}

func publicAccessStatusToPrivate(status tunnelservice.PublicAccessStatus) *privatev1.PublicAccessStatus {
	semantic := &privatev1.PublicAccessStatus{
		State: publicAccessLifecycleToPrivate(status.State), Generation: status.Generation,
		SettingsRevision: status.SettingsRevision, ErrorCategory: publicAccessErrorToPrivate(status.ErrorCategory),
	}
	if status.PublicURL != "" {
		semantic.PublicUrl = &status.PublicURL
	}
	if status.ErrorMessage != "" {
		semantic.ErrorMessage = &status.ErrorMessage
	}
	return semantic
}

func publicAccessStatusFromPrivate(status *privatev1.PublicAccessStatus) PublicAccessStatus {
	if status == nil {
		return PublicAccessStatus{}
	}
	return PublicAccessStatus{
		State: publicAccessLifecycleFromPrivate(status.GetState()), Generation: status.GetGeneration(),
		SettingsRevision: status.GetSettingsRevision(), PublicURL: status.GetPublicUrl(),
		ErrorCategory: publicAccessErrorFromPrivate(status.GetErrorCategory()), ErrorMessage: status.GetErrorMessage(),
	}
}

func routePublicAccessCommandResult(result PublicAccessCommandResult) PublicAccessCommandResult {
	semantic := &privatev1.PublicAccessCommandResult{Ok: result.OK}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	nativeSnapshot := tunnelservice.PublicAccessSnapshot{
		Preferences: tunnelservice.PublicAccessPreferences{
			Version: result.Snapshot.Preferences.Version, EnabledPreference: result.Snapshot.Preferences.EnabledPreference,
			ReservedDomain: result.Snapshot.Preferences.ReservedDomain, Username: result.Snapshot.Preferences.Username,
			ProviderTokenPresentHint:  result.Snapshot.Preferences.ProviderTokenPresentHint,
			PlayerPasswordPresentHint: result.Snapshot.Preferences.PlayerPasswordPresentHint, Revision: result.Snapshot.Preferences.Revision,
		},
		ProviderTokenPresence:  secretPresenceFromNative(result.Snapshot.ProviderTokenPresence),
		PlayerPasswordPresence: secretPresenceFromNative(result.Snapshot.PlayerPasswordPresence),
		Status:                 publicAccessStatusFromNative(result.Snapshot.Status),
	}
	semantic.Snapshot = publicAccessSnapshotToPrivate(nativeSnapshot)
	return PublicAccessCommandResult{OK: semantic.GetOk(), Error: semantic.GetError(), Snapshot: routePublicAccessSnapshot(nativeSnapshot)}
}

func routeGeneratedPlayerPasswordResult(result GeneratedPlayerPasswordResult) GeneratedPlayerPasswordResult {
	semantic := &privatev1.GeneratedPlayerPasswordResult{Ok: result.OK, SettingsRevision: result.SettingsRevision}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	if result.GeneratedPassword != "" {
		semantic.GeneratedPassword = &result.GeneratedPassword
	}
	return GeneratedPlayerPasswordResult{OK: semantic.GetOk(), Error: semantic.GetError(), GeneratedPassword: semantic.GetGeneratedPassword(), SettingsRevision: semantic.GetSettingsRevision()}
}

func publicAccessLifecycleToPrivate(state tunnelservice.LifecycleState) privatev1.PublicAccessLifecycleState {
	switch state {
	case tunnelservice.LifecycleDisabled:
		return privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_DISABLED
	case tunnelservice.LifecycleStarting:
		return privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_STARTING
	case tunnelservice.LifecycleReady:
		return privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_READY
	case tunnelservice.LifecycleStopping:
		return privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_STOPPING
	case tunnelservice.LifecycleFailed:
		return privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_FAILED
	default:
		return privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_UNSPECIFIED
	}
}

func publicAccessLifecycleFromPrivate(state privatev1.PublicAccessLifecycleState) string {
	switch state {
	case privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_DISABLED:
		return "stopped"
	case privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_STARTING:
		return "starting"
	case privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_READY:
		return "ready"
	case privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_STOPPING:
		return "stopping"
	case privatev1.PublicAccessLifecycleState_PUBLIC_ACCESS_LIFECYCLE_STATE_FAILED:
		return "error"
	default:
		return ""
	}
}

func secretPresenceToPrivate(presence tunnelservice.SecretPresence) privatev1.SecretPresence {
	switch presence {
	case tunnelservice.SecretAbsent:
		return privatev1.SecretPresence_SECRET_PRESENCE_ABSENT
	case tunnelservice.SecretPresent:
		return privatev1.SecretPresence_SECRET_PRESENCE_PRESENT
	case tunnelservice.SecretUnknown:
		return privatev1.SecretPresence_SECRET_PRESENCE_UNKNOWN
	default:
		return privatev1.SecretPresence_SECRET_PRESENCE_UNSPECIFIED
	}
}

func secretPresenceFromPrivate(presence privatev1.SecretPresence) string {
	switch presence {
	case privatev1.SecretPresence_SECRET_PRESENCE_ABSENT:
		return "absent"
	case privatev1.SecretPresence_SECRET_PRESENCE_PRESENT:
		return "present"
	case privatev1.SecretPresence_SECRET_PRESENCE_UNKNOWN:
		return "unknown"
	default:
		return ""
	}
}

func secretPresenceFromNative(presence string) tunnelservice.SecretPresence {
	switch presence {
	case "absent":
		return tunnelservice.SecretAbsent
	case "present":
		return tunnelservice.SecretPresent
	default:
		return tunnelservice.SecretUnknown
	}
}

func publicAccessErrorToPrivate(category tunnelservice.ErrorCategory) privatev1.PublicAccessErrorCategory {
	if !category.Valid() {
		return privatev1.PublicAccessErrorCategory_PUBLIC_ACCESS_ERROR_CATEGORY_UNSPECIFIED
	}
	return privatev1.PublicAccessErrorCategory(category)
}

func publicAccessErrorFromPrivate(category privatev1.PublicAccessErrorCategory) string {
	if category == privatev1.PublicAccessErrorCategory_PUBLIC_ACCESS_ERROR_CATEGORY_UNSPECIFIED {
		return ""
	}
	native := tunnelservice.ErrorCategory(category)
	if !native.Valid() {
		return ""
	}
	names := [...]string{"", "validation", "settings_corrupt", "secret_store_locked", "secret_store_denied", "secret_store_unavailable", "credential_missing", "provider_authentication", "domain_unavailable", "network_unavailable", "timeout", "provider_failure", "shutdown_timeout", "conflict"}
	return names[native]
}

func publicAccessErrorFromNative(category string) tunnelservice.ErrorCategory {
	for candidate := tunnelservice.ErrorValidation; candidate <= tunnelservice.ErrorConflict; candidate++ {
		if publicAccessErrorFromPrivate(privatev1.PublicAccessErrorCategory(candidate)) == category {
			return candidate
		}
	}
	return 0
}

func publicAccessStatusFromNative(status PublicAccessStatus) tunnelservice.PublicAccessStatus {
	states := map[string]tunnelservice.LifecycleState{
		"disabled": tunnelservice.LifecycleDisabled, "stopped": tunnelservice.LifecycleDisabled,
		"starting": tunnelservice.LifecycleStarting, "ready": tunnelservice.LifecycleReady,
		"stopping": tunnelservice.LifecycleStopping,
		"failed":   tunnelservice.LifecycleFailed, "error": tunnelservice.LifecycleFailed,
	}
	return tunnelservice.PublicAccessStatus{State: states[status.State], Generation: status.Generation, SettingsRevision: status.SettingsRevision, PublicURL: status.PublicURL, ErrorCategory: publicAccessErrorFromNative(status.ErrorCategory), ErrorMessage: status.ErrorMessage}
}

func routeAddCharacterRequest(payload CharacterCreatePayload) CharacterCreatePayload {
	semantic := &privatev1.AddCharacterRequest{
		DisplayName:      payload.Name,
		Intelligence:     int32(payload.Intelligence),
		ExpectedRevision: payload.ExpectedRevision,
	}
	if payload.HackerPerkAvailable != nil {
		value := *payload.HackerPerkAvailable
		semantic.HackerPerkAvailable = &value
	}
	routed := CharacterCreatePayload{
		Name:             semantic.GetDisplayName(),
		Intelligence:     int(semantic.GetIntelligence()),
		ExpectedRevision: semantic.GetExpectedRevision(),
	}
	if semantic.HackerPerkAvailable != nil {
		value := semantic.GetHackerPerkAvailable()
		routed.HackerPerkAvailable = &value
	}
	return routed
}

func routeUpdateCharacterRequest(payload CharacterUpdatePayload) CharacterUpdatePayload {
	semantic := &privatev1.RenameCharacterRequest{
		CharacterId:      string(payload.CharacterID),
		DisplayName:      payload.Name,
		Intelligence:     int32(payload.Intelligence),
		ExpectedRevision: payload.ExpectedRevision,
	}
	if payload.HackerPerkAvailable != nil {
		value := *payload.HackerPerkAvailable
		semantic.HackerPerkAvailable = &value
	}
	routed := CharacterUpdatePayload{
		CharacterID:      domain.CharacterID(semantic.GetCharacterId()),
		Name:             semantic.GetDisplayName(),
		Intelligence:     int(semantic.GetIntelligence()),
		ExpectedRevision: semantic.GetExpectedRevision(),
	}
	if semantic.HackerPerkAvailable != nil {
		value := semantic.GetHackerPerkAvailable()
		routed.HackerPerkAvailable = &value
	}
	return routed
}

func routeDeleteCharacterRequest(payload CharacterDeletePayload) CharacterDeletePayload {
	semantic := &privatev1.DeleteCharacterRequest{
		CharacterId:      string(payload.CharacterID),
		ExpectedRevision: payload.ExpectedRevision,
	}
	return CharacterDeletePayload{
		CharacterID:      domain.CharacterID(semantic.GetCharacterId()),
		ExpectedRevision: semantic.GetExpectedRevision(),
	}
}

func routeRenameLogicalSessionRequest(payload LogicalSessionRenamePayload) LogicalSessionRenamePayload {
	semantic := &privatev1.RenameLogicalSessionRequest{LogicalSessionId: string(payload.SessionID), FallbackName: payload.FallbackName}
	return LogicalSessionRenamePayload{SessionID: domain.LogicalSessionID(semantic.GetLogicalSessionId()), FallbackName: semantic.GetFallbackName()}
}

func routeAssignCharacterRequest(payload AssignmentPayload) AssignmentPayload {
	semantic := &privatev1.AssignCharacterRequest{LogicalSessionId: string(payload.SessionID), CharacterId: string(payload.CharacterID)}
	return AssignmentPayload{SessionID: domain.LogicalSessionID(semantic.GetLogicalSessionId()), CharacterID: domain.CharacterID(semantic.GetCharacterId())}
}

func routeReleaseCharacterRequest(sessionID string) string {
	return (&privatev1.ReleaseCharacterRequest{LogicalSessionId: sessionID}).GetLogicalSessionId()
}

func routeMoveCharacterRequest(payload MoveCharacterPayload) MoveCharacterPayload {
	semantic := &privatev1.MoveCharacterRequest{CharacterId: string(payload.CharacterID), DestinationSessionId: string(payload.ToSessionID)}
	return MoveCharacterPayload{CharacterID: domain.CharacterID(semantic.GetCharacterId()), ToSessionID: domain.LogicalSessionID(semantic.GetDestinationSessionId())}
}

func routeSetActiveControllerRequest(sessionID string) string {
	return (&privatev1.SetActiveControllerRequest{LogicalSessionId: sessionID}).GetLogicalSessionId()
}

func routeOpenURLRequest(rawURL string) string {
	return (&privatev1.OpenUrlRequest{Url: rawURL}).GetUrl()
}

func routeTerminalSwitchDecisionRequest(payload TerminalSwitchDecisionPayload) (TerminalSwitchDecisionPayload, error) {
	choice, err := terminalSwitchChoiceToPrivate(payload.Decision)
	if err != nil {
		return payload, err
	}
	semantic := &privatev1.TerminalSwitchDecisionRequest{SwitchId: string(payload.SwitchID), Choice: choice}
	return TerminalSwitchDecisionPayload{SwitchID: domain.SwitchID(semantic.GetSwitchId()), Decision: terminalSwitchChoiceFromPrivate(semantic.GetChoice())}, nil
}

func routeCommandExecutionDecisionRequest(payload CommandExecutionDecisionPayload) (CommandExecutionDecisionPayload, error) {
	decision, err := commandExecutionDecisionToPrivate(payload.Decision)
	if err != nil {
		return payload, err
	}
	semantic := &privatev1.ResolveCommandExecutionRequest{RequestId: payload.RequestID, Decision: decision}
	return CommandExecutionDecisionPayload{
		RequestID: semantic.GetRequestId(), Decision: commandExecutionDecisionFromPrivate(semantic.GetDecision()),
	}, nil
}

func routeTerminalNavigationDecisionRequest(payload TerminalNavigationDecisionPayload) (TerminalNavigationDecisionPayload, error) {
	decision, err := terminalNavigationDecisionToPrivate(payload.Decision)
	if err != nil {
		return payload, err
	}
	semantic := &privatev1.ResolveTerminalNavigationRequest{RequestId: payload.RequestID, Decision: decision}
	return TerminalNavigationDecisionPayload{
		RequestID: semantic.GetRequestId(), Decision: terminalNavigationDecisionFromPrivate(semantic.GetDecision()),
	}, nil
}

func routeResetCommandStateRequest(payload ResetCommandStatePayload) ResetCommandStatePayload {
	semantic := &privatev1.ResetCommandStateRequest{TerminalId: payload.TerminalID, CommandId: payload.CommandID}
	return ResetCommandStatePayload{TerminalID: semantic.GetTerminalId(), CommandID: semantic.GetCommandId()}
}

func routeResetTerminalCommandStatesRequest(payload ResetTerminalCommandStatesPayload) ResetTerminalCommandStatesPayload {
	semantic := &privatev1.ResetTerminalCommandStatesRequest{TerminalId: payload.TerminalID}
	return ResetTerminalCommandStatesPayload{TerminalID: semantic.GetTerminalId()}
}

func routeTerminalActivationRequest(payload LiveTerminalPayload, reset bool) (LiveTerminalPayload, error) {
	tree, err := contentNodeToPrivate(payload.Tree)
	if err != nil {
		return payload, err
	}
	if reset {
		semantic := &privatev1.ResetFailedHackRequest{
			TerminalId: payload.TerminalID, TerminalName: payload.TerminalName, Tree: tree,
			HackLevel: int32(payload.HackLevel), IntroText: payload.IntroText,
		}
		return liveTerminalFromPrivate(semantic.GetTerminalId(), semantic.GetTerminalName(), semantic.GetTree(), semantic.GetHackLevel(), semantic.GetIntroText(), payload.Tree)
	}
	semantic := &privatev1.TerminalActivationRequest{
		TerminalId: payload.TerminalID, TerminalName: payload.TerminalName, Tree: tree,
		HackLevel: int32(payload.HackLevel), IntroText: payload.IntroText,
	}
	return liveTerminalFromPrivate(semantic.GetTerminalId(), semantic.GetTerminalName(), semantic.GetTree(), semantic.GetHackLevel(), semantic.GetIntroText(), payload.Tree)
}

func routeLiveTerminalUpdateRequest(payload LiveUpdatePayload) (LiveUpdatePayload, error) {
	tree, err := contentNodeToPrivate(payload.Tree)
	if err != nil {
		return payload, err
	}
	semantic := &privatev1.LiveTerminalUpdateRequest{Tree: tree, IntroText: payload.IntroText}
	routedTree, err := contentNodeFromPrivate(semantic.GetTree(), payload.Tree)
	if err != nil {
		return payload, err
	}
	result := LiveUpdatePayload{Tree: routedTree}
	if semantic.IntroText != nil {
		value := semantic.GetIntroText()
		result.IntroText = &value
	}
	return result, nil
}

func liveTerminalFromPrivate(id, name string, tree *persistencev1.ContentNode, level int32, intro string, template domain.ContentNode) (LiveTerminalPayload, error) {
	routedTree, err := contentNodeFromPrivate(tree, template)
	if err != nil {
		return LiveTerminalPayload{}, err
	}
	return LiveTerminalPayload{TerminalID: id, TerminalName: name, Tree: routedTree, HackLevel: int(level), IntroText: intro}, nil
}

func contentNodeToPrivate(node domain.ContentNode) (*persistencev1.ContentNode, error) {
	node = cloneTreeForBridgeValidation(node)
	return sessionservice.ContentNodeToProto(node)
}

func contentNodeFromPrivate(node *persistencev1.ContentNode, template domain.ContentNode) (domain.ContentNode, error) {
	result, err := sessionservice.ContentNodeFromProto(node, template)
	if err != nil {
		return domain.ContentNode{}, err
	}
	restoreContentNodeShape(&result, template)
	return result, nil
}

func restoreContentNodeShape(node *domain.ContentNode, template domain.ContentNode) {
	if node == nil {
		return
	}
	if template.Children == nil && len(node.Children) == 0 {
		node.Children = nil
	}
	for index := range node.Children {
		if index < len(template.Children) {
			restoreContentNodeShape(&node.Children[index], template.Children[index])
		}
	}
}

func runtimeStatusToPrivate(status RuntimeStatus) *privatev1.RuntimeStatus {
	// Lifecycle phase is intentionally not serialized. Existing server-info and
	// startup-error fields are the complete Overseer-visible startup projection.
	result := &privatev1.RuntimeStatus{
		ClientCount: uint32(max(status.ClientCount, 0)), SaveState: status.SaveState,
		RequestedRevision: status.RequestedRevision,
		CoordinationState: coordinationStateToPrivate(status.CoordinationState),
	}
	if status.ServerInfo != nil {
		result.ServerInfo = serverInfoToPrivate(*status.ServerInfo)
	}
	if status.HackState != nil {
		result.HackState = publicHackToPrivate(status.HackState)
	}
	if status.StartupError != "" {
		result.StartupError = &status.StartupError
	}
	if status.SavedRevision != 0 {
		result.SavedRevision = &status.SavedRevision
	}
	return result
}

func runtimeStatusFromPrivate(status *privatev1.RuntimeStatus) RuntimeStatus {
	if status == nil {
		return RuntimeStatus{}
	}
	result := RuntimeStatus{
		ClientCount:       int(status.GetClientCount()),
		HackState:         publicHackFromPrivate(status.GetHackState()),
		SaveState:         status.GetSaveState(),
		RequestedRevision: status.GetRequestedRevision(),
		CoordinationState: coordinationStateFromPrivate(status.GetCoordinationState()),
	}
	if status.ServerInfo != nil {
		value := serverInfoFromPrivate(status.ServerInfo)
		result.ServerInfo = &value
	}
	if status.StartupError != nil {
		result.StartupError = status.GetStartupError()
	}
	if status.SavedRevision != nil {
		result.SavedRevision = status.GetSavedRevision()
	}
	return result
}

func routeRuntimeStatus(status RuntimeStatus) RuntimeStatus {
	return runtimeStatusFromPrivate(runtimeStatusToPrivate(status))
}

func commandResultToPrivate(result CommandResult) *privatev1.CommandResult {
	semantic := &privatev1.CommandResult{Ok: result.OK}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	return semantic
}

func routeCommandResult(result CommandResult) CommandResult {
	semantic := commandResultToPrivate(result)
	return CommandResult{OK: semantic.GetOk(), Error: semantic.GetError()}
}

func routeLogAccessResult(result LogAccessResult) LogAccessResult {
	semantic := &privatev1.LogAccessResult{
		Ok:            result.OK,
		DirectoryPath: result.DirectoryPath,
	}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	if result.ActiveLogPath != "" {
		semantic.ActiveLogPath = &result.ActiveLogPath
	}
	return LogAccessResult{
		OK:            semantic.GetOk(),
		Error:         semantic.GetError(),
		DirectoryPath: semantic.GetDirectoryPath(),
		ActiveLogPath: semantic.GetActiveLogPath(),
	}
}

func routeSessionOperationResult(result sessionservice.SessionResult) sessionservice.SessionResult {
	semantic := &privatev1.SessionOperationResult{Ok: result.OK, Canceled: result.Canceled}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	if result.FilePath != "" {
		semantic.FilePath = &result.FilePath
	}
	if result.Session != nil {
		semantic.Session, _ = sessionservice.SessionToProto(*result.Session)
	}
	routed := sessionservice.SessionResult{OK: semantic.GetOk(), Canceled: semantic.GetCanceled(), Error: semantic.GetError(), FilePath: semantic.GetFilePath()}
	if semantic.Session != nil {
		template := domain.Session{}
		if result.Session != nil {
			template = *result.Session
		}
		if value, err := sessionservice.SessionFromProto(semantic.Session, template); err == nil {
			routed.Session = &value
		}
	}
	return routed
}

func routeSaveSessionResult(result sessionservice.SaveResult) sessionservice.SaveResult {
	semantic := &privatev1.SaveSessionResult{Ok: result.OK, RequestedRevision: result.RequestedRevision}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	if result.SavedRevision != 0 {
		semantic.SavedRevision = &result.SavedRevision
	}
	return sessionservice.SaveResult{
		OK: semantic.GetOk(), Error: semantic.GetError(), RequestedRevision: semantic.GetRequestedRevision(), SavedRevision: semantic.GetSavedRevision(),
	}
}

func routeSessionStateResult(result SessionStateResult) SessionStateResult {
	semantic := &privatev1.SessionStateResult{Ok: result.OK, Revision: result.Revision}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	if result.Session != nil {
		semantic.Session, _ = sessionservice.SessionToProto(*result.Session)
	}
	routed := SessionStateResult{OK: semantic.GetOk(), Error: semantic.GetError(), Revision: semantic.GetRevision()}
	if semantic.Session != nil {
		template := domain.Session{}
		if result.Session != nil {
			template = *result.Session
		}
		if value, err := sessionservice.SessionFromProto(semantic.Session, template); err == nil {
			restoreSessionNativeShape(&value, template)
			routed.Session = &value
		}
	}
	return routed
}

func routeTerminalGroupReplacementRequest(payload TerminalGroupReplacementPayload) TerminalGroupReplacementPayload {
	semantic := &privatev1.ReplaceTerminalGroupsRequest{
		TerminalGroups:               make([]*persistencev1.TerminalGroup, 0, len(payload.TerminalGroups)),
		ExpectedSessionRevision:      payload.ExpectedSessionRevision,
		ExpectedCoordinationRevision: payload.ExpectedCoordinationRevision,
	}
	for _, group := range payload.TerminalGroups {
		semantic.TerminalGroups = append(semantic.TerminalGroups, &persistencev1.TerminalGroup{
			Id: group.ID, Name: group.Name, TerminalIds: append([]string(nil), group.TerminalIDs...),
		})
	}
	routed := TerminalGroupReplacementPayload{
		TerminalGroups:               make([]domain.TerminalGroup, 0, len(semantic.GetTerminalGroups())),
		ExpectedSessionRevision:      semantic.GetExpectedSessionRevision(),
		ExpectedCoordinationRevision: semantic.GetExpectedCoordinationRevision(),
	}
	for _, group := range semantic.GetTerminalGroups() {
		routed.TerminalGroups = append(routed.TerminalGroups, domain.TerminalGroup{
			ID: group.GetId(), Name: group.GetName(), TerminalIDs: append([]string(nil), group.GetTerminalIds()...),
		})
	}
	return routed
}

func routeTerminalGroupReplacementResult(result TerminalGroupReplacementResult) TerminalGroupReplacementResult {
	semantic := &privatev1.ReplaceTerminalGroupsResult{
		Ok:                result.OK,
		SessionRevision:   result.SessionRevision,
		CoordinationState: coordinationStateToPrivate(result.CoordinationState),
	}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	if result.Session != nil {
		semantic.Session, _ = sessionservice.SessionToProto(*result.Session)
	}
	routed := TerminalGroupReplacementResult{
		OK:                semantic.GetOk(),
		Error:             semantic.GetError(),
		SessionRevision:   semantic.GetSessionRevision(),
		CoordinationState: coordinationStateFromPrivate(semantic.GetCoordinationState()),
	}
	if semantic.Session != nil {
		template := domain.Session{}
		if result.Session != nil {
			template = *result.Session
		}
		if value, err := sessionservice.SessionFromProto(semantic.Session, template); err == nil {
			restoreSessionNativeShape(&value, template)
			routed.Session = &value
		}
	}
	return routed
}

func routePlayerConfigResult(result PlayerConfigCommandResult) PlayerConfigCommandResult {
	semantic := &privatev1.PlayerConfigOperationResult{Ok: result.OK, Canceled: result.Canceled, State: coordinationStateToPrivate(result.State)}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	if result.Session != nil {
		semantic.Session, _ = sessionservice.SessionToProto(*result.Session)
	}
	if result.Config != nil {
		semantic.PlayerConfigMetadata = playerConfigMetadataToPrivate(result.Config)
	}
	routed := PlayerConfigCommandResult{
		OK: semantic.GetOk(), Canceled: semantic.GetCanceled(), Error: semantic.GetError(),
		Config: playerConfigMetadataFromPrivate(semantic.GetPlayerConfigMetadata()),
		State:  coordinationStateFromPrivate(semantic.GetState()),
	}
	if semantic.Session != nil {
		template := domain.Session{}
		if result.Session != nil {
			template = *result.Session
		}
		if value, err := sessionservice.SessionFromProto(semantic.Session, template); err == nil {
			routed.Session = &value
		}
	}
	return routed
}

func coordinationResultToPrivate(result CoordinationCommandResult) *privatev1.CoordinationResult {
	semantic := &privatev1.CoordinationResult{Ok: result.OK, State: coordinationStateToPrivate(result.State)}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	return semantic
}

func routeCoordinationResult(result CoordinationCommandResult) CoordinationCommandResult {
	semantic := coordinationResultToPrivate(result)
	return CoordinationCommandResult{OK: semantic.GetOk(), Error: semantic.GetError(), State: coordinationStateFromPrivate(semantic.GetState())}
}

func routeResolveCommandExecutionResult(result ResolveCommandExecutionResult) ResolveCommandExecutionResult {
	semantic := &privatev1.ResolveCommandExecutionResult{
		Ok: result.OK, State: coordinationStateToPrivate(result.State),
	}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	return ResolveCommandExecutionResult{
		OK: semantic.GetOk(), Error: semantic.GetError(), State: coordinationStateFromPrivate(semantic.GetState()),
	}
}

func routeResolveTerminalNavigationResult(result ResolveTerminalNavigationResult) ResolveTerminalNavigationResult {
	semantic := &privatev1.ResolveTerminalNavigationResult{Ok: result.OK, State: coordinationStateToPrivate(result.State)}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	return ResolveTerminalNavigationResult{
		OK: semantic.GetOk(), Error: semantic.GetError(), State: coordinationStateFromPrivate(semantic.GetState()),
	}
}

func terminalSwitchResultToPrivate(result TerminalSwitchCommandResult) *privatev1.TerminalSwitchResult {
	semantic := &privatev1.TerminalSwitchResult{Ok: result.OK, State: coordinationStateToPrivate(result.State), Status: terminalSwitchStatusToPrivate(result.Status)}
	if result.Error != "" {
		semantic.Error = &result.Error
	}
	if result.SwitchID != "" {
		value := string(result.SwitchID)
		semantic.SwitchId = &value
	}
	return semantic
}

func routeTerminalSwitchResult(result TerminalSwitchCommandResult) TerminalSwitchCommandResult {
	semantic := terminalSwitchResultToPrivate(result)
	return TerminalSwitchCommandResult{
		OK: semantic.GetOk(), Error: semantic.GetError(), Status: terminalSwitchStatusFromPrivate(semantic.GetStatus()),
		SwitchID: domain.SwitchID(semantic.GetSwitchId()), State: coordinationStateFromPrivate(semantic.GetState()),
	}
}

func routeServerInfoEvent(info domain.ServerInfo) domain.ServerInfo {
	semantic := &privatev1.ServerInformationEvent{ServerInfo: serverInfoToPrivate(info)}
	return serverInfoFromPrivate(semantic.GetServerInfo())
}

func routeClientCountEvent(count int) int {
	semantic := &privatev1.ClientCountEvent{ClientCount: uint32(max(count, 0))}
	return int(semantic.GetClientCount())
}

func routeHackStateEvent(state *domain.PublicHackState) *domain.PublicHackState {
	semantic := &privatev1.HackStateEvent{HackState: publicHackToPrivate(state)}
	return publicHackFromPrivate(semantic.GetHackState())
}

func routeCoordinationEvent(state *domain.MasterCoordinationState) *domain.MasterCoordinationState {
	semantic := &privatev1.CoordinationStateEvent{CoordinationState: coordinationStateToPrivate(state)}
	return coordinationStateFromPrivate(semantic.GetCoordinationState())
}

func routeSessionStateEvent(event SessionStateEvent) SessionStateEvent {
	semantic := &privatev1.SessionStateEvent{Revision: event.Revision}
	if event.Session != nil {
		semantic.Session, _ = sessionservice.SessionToProto(*event.Session)
	}
	routed := SessionStateEvent{Revision: semantic.GetRevision()}
	if semantic.Session != nil {
		template := domain.Session{}
		if event.Session != nil {
			template = *event.Session
		}
		if value, err := sessionservice.SessionFromProto(semantic.Session, template); err == nil {
			restoreSessionNativeShape(&value, template)
			routed.Session = &value
		}
	}
	return routed
}

func restoreSessionNativeShape(session *domain.Session, template domain.Session) {
	if session == nil {
		return
	}
	for index := range session.Terminals {
		for templateIndex := range template.Terminals {
			if template.Terminals[templateIndex].ID != session.Terminals[index].ID {
				continue
			}
			if template.Terminals[templateIndex].CommandStates != nil && session.Terminals[index].CommandStates == nil {
				session.Terminals[index].CommandStates = make(map[string]domain.CommandExecutionState)
			}
			restoreContentNodeNativeShape(&session.Terminals[index].Root, template.Terminals[templateIndex].Root)
			break
		}
	}
}

func restoreContentNodeNativeShape(node *domain.ContentNode, template domain.ContentNode) {
	if node == nil {
		return
	}
	if template.Children == nil {
		node.Children = nil
		return
	}
	for index := range node.Children {
		if index < len(template.Children) {
			restoreContentNodeNativeShape(&node.Children[index], template.Children[index])
		}
	}
}

func coordinationStateToPrivate(state *domain.MasterCoordinationState) *privatev1.CoordinationState {
	if state == nil {
		return nil
	}
	result := &privatev1.CoordinationState{
		Roster:          make([]*privatev1.CharacterState, 0, len(state.Roster)),
		LogicalSessions: make([]*privatev1.LogicalSessionState, 0, len(state.Sessions)),
		Revision:        state.Revision,
		PlayerConfig:    playerConfigMetadataToPrivate(state.PlayerConfig),
	}
	for _, entry := range state.Roster {
		mapped := &privatev1.CharacterState{
			CharacterId:         string(entry.ID),
			DisplayName:         entry.Name,
			Intelligence:        int32(entry.Intelligence),
			HackerPerkAvailable: entry.HackerPerkAvailable,
		}
		if entry.ClaimedBySessionID != nil {
			value := string(*entry.ClaimedBySessionID)
			mapped.LogicalSessionId = &value
		}
		result.Roster = append(result.Roster, mapped)
	}
	for _, session := range state.Sessions {
		mapped := &privatev1.LogicalSessionState{LogicalSessionId: string(session.ID), FallbackName: session.FallbackName, Connected: session.Connected, Role: playerRoleToPrivate(session.Role)}
		if session.Connected {
			mapped.ActiveStreams = 1
		}
		if session.Character != nil {
			value := string(session.Character.ID)
			mapped.CharacterId = &value
		}
		result.LogicalSessions = append(result.LogicalSessions, mapped)
	}
	if state.Broadcast != nil {
		result.Broadcast = &privatev1.BroadcastState{BroadcastId: string(state.Broadcast.ID), Revision: state.Revision}
		if state.Broadcast.ControllerSessionID != nil {
			value := string(*state.Broadcast.ControllerSessionID)
			result.Broadcast.ActiveControllerSessionId = &value
		}
		if state.Broadcast.ActiveTerminalID != nil {
			value := *state.Broadcast.ActiveTerminalID
			result.Broadcast.ActiveTerminalId = &value
		}
	}
	if state.PendingSwitch != nil {
		targetID := ""
		if state.PendingSwitch.TargetTerminalID != nil {
			targetID = *state.PendingSwitch.TargetTerminalID
		}
		result.PendingTerminalSwitch = &privatev1.PendingTerminalSwitch{
			SwitchId: string(state.PendingSwitch.SwitchID), TerminalId: targetID,
			BroadcastId: string(state.PendingSwitch.BroadcastID), SourceTerminalId: state.PendingSwitch.SourceTerminalID,
		}
		if state.PendingSwitch.TargetTerminalID != nil {
			value := *state.PendingSwitch.TargetTerminalID
			result.PendingTerminalSwitch.TargetTerminalId = &value
		}
	}
	if state.PendingCommandExecution != nil {
		pending := state.PendingCommandExecution
		result.PendingCommandExecution = &privatev1.PendingCommandExecution{
			RequestId: pending.RequestID, BroadcastId: string(pending.BroadcastID),
			TerminalId: pending.TerminalID, CommandId: pending.CommandID,
			CommandName: pending.CommandName, ConfirmationText: pending.ConfirmationText,
			CommandMode: string(pending.Mode),
		}
	}
	if state.PendingTerminalNavigation != nil {
		pending := state.PendingTerminalNavigation
		result.PendingTerminalNavigation = &privatev1.PendingTerminalNavigation{
			RequestId: pending.RequestID, BroadcastId: string(pending.BroadcastID),
			Direction:        terminalNavigationDirectionToPrivate(pending.Direction),
			SourceTerminalId: pending.SourceTerminalID, SourceTerminalName: pending.SourceTerminalName,
			CommandId: pending.CommandID, CommandName: pending.CommandName,
			TargetTerminalId: pending.TargetTerminalID, TargetTerminalName: pending.TargetTerminalName,
			RouteDepth: pending.RouteDepth,
		}
	}
	if state.TerminalNavigationNotice != nil {
		notice := state.TerminalNavigationNotice
		result.TerminalNavigationNotice = &privatev1.TerminalNavigationNotice{
			Reason: terminalNavigationNoticeToPrivate(notice.Reason), SourceTerminalId: notice.SourceTerminalID, CommandId: notice.CommandID,
		}
		if notice.TargetTerminalID != nil {
			value := *notice.TargetTerminalID
			result.TerminalNavigationNotice.TargetTerminalId = &value
		}
	}
	return result
}

func coordinationStateFromPrivate(state *privatev1.CoordinationState) *domain.MasterCoordinationState {
	if state == nil {
		return nil
	}
	result := &domain.MasterCoordinationState{Revision: state.GetRevision(), PlayerConfig: playerConfigMetadataFromPrivate(state.GetPlayerConfig())}
	for _, entry := range state.GetRoster() {
		mapped := domain.MasterRosterEntry{
			ID:                  domain.CharacterID(entry.GetCharacterId()),
			Name:                entry.GetDisplayName(),
			Intelligence:        int(entry.GetIntelligence()),
			HackerPerkAvailable: entry.GetHackerPerkAvailable(),
		}
		if entry.LogicalSessionId != nil {
			value := domain.LogicalSessionID(entry.GetLogicalSessionId())
			mapped.ClaimedBySessionID = &value
		}
		result.Roster = append(result.Roster, mapped)
	}
	for _, session := range state.GetLogicalSessions() {
		mapped := domain.MasterSessionEntry{
			ID: domain.LogicalSessionID(session.GetLogicalSessionId()), FallbackName: session.GetFallbackName(),
			Connected: session.GetConnected(), Role: playerRoleFromPrivate(session.GetRole()),
		}
		if session.CharacterId != nil {
			characterID := domain.CharacterID(session.GetCharacterId())
			mapped.Character = &domain.PlayerCharacter{ID: characterID}
			for _, roster := range result.Roster {
				if roster.ID == characterID {
					mapped.Character.Name = roster.Name
					break
				}
			}
		}
		result.Sessions = append(result.Sessions, mapped)
	}
	if state.Broadcast != nil {
		result.Broadcast = &domain.MasterBroadcastState{ID: domain.BroadcastID(state.Broadcast.GetBroadcastId())}
		if state.Broadcast.ActiveControllerSessionId != nil {
			value := domain.LogicalSessionID(state.Broadcast.GetActiveControllerSessionId())
			result.Broadcast.ControllerSessionID = &value
		}
		if state.Broadcast.ActiveTerminalId != nil {
			value := state.Broadcast.GetActiveTerminalId()
			result.Broadcast.ActiveTerminalID = &value
		}
	}
	if state.PendingTerminalSwitch != nil {
		pending := state.PendingTerminalSwitch
		result.PendingSwitch = &domain.MasterPendingSwitch{
			SwitchID: domain.SwitchID(pending.GetSwitchId()), BroadcastID: domain.BroadcastID(pending.GetBroadcastId()),
			SourceTerminalID: pending.GetSourceTerminalId(),
		}
		if pending.TargetTerminalId != nil {
			value := pending.GetTargetTerminalId()
			result.PendingSwitch.TargetTerminalID = &value
		}
	}
	if state.PendingCommandExecution != nil {
		pending := state.PendingCommandExecution
		result.PendingCommandExecution = &domain.MasterPendingCommandExecution{
			RequestID: pending.GetRequestId(), BroadcastID: domain.BroadcastID(pending.GetBroadcastId()),
			TerminalID: pending.GetTerminalId(), CommandID: pending.GetCommandId(),
			CommandName: pending.GetCommandName(), Mode: domain.CommandApprovalMode(pending.GetCommandMode()),
			ConfirmationText: pending.GetConfirmationText(),
		}
	}
	if state.PendingTerminalNavigation != nil {
		pending := state.PendingTerminalNavigation
		result.PendingTerminalNavigation = &domain.MasterPendingTerminalNavigation{
			RequestID: pending.GetRequestId(), BroadcastID: domain.BroadcastID(pending.GetBroadcastId()),
			Direction:        terminalNavigationDirectionFromPrivate(pending.GetDirection()),
			SourceTerminalID: pending.GetSourceTerminalId(), SourceTerminalName: pending.GetSourceTerminalName(),
			CommandID: pending.GetCommandId(), CommandName: pending.GetCommandName(),
			TargetTerminalID: pending.GetTargetTerminalId(), TargetTerminalName: pending.GetTargetTerminalName(),
			RouteDepth: pending.GetRouteDepth(),
		}
	}
	if state.TerminalNavigationNotice != nil {
		notice := state.TerminalNavigationNotice
		result.TerminalNavigationNotice = &domain.MasterTerminalNavigationNotice{
			Reason: terminalNavigationNoticeFromPrivate(notice.GetReason()), SourceTerminalID: notice.GetSourceTerminalId(), CommandID: notice.GetCommandId(),
		}
		if notice.TargetTerminalId != nil {
			value := notice.GetTargetTerminalId()
			result.TerminalNavigationNotice.TargetTerminalID = &value
		}
	}
	return result
}

func terminalNavigationDirectionToPrivate(direction domain.TerminalNavigationDirection) playerv1.TerminalNavigationDirection {
	switch direction {
	case domain.TerminalNavigationForward:
		return playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_FORWARD
	case domain.TerminalNavigationReturn:
		return playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_RETURN
	default:
		return playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_UNSPECIFIED
	}
}

func terminalNavigationDirectionFromPrivate(direction playerv1.TerminalNavigationDirection) domain.TerminalNavigationDirection {
	switch direction {
	case playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_FORWARD:
		return domain.TerminalNavigationForward
	case playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_RETURN:
		return domain.TerminalNavigationReturn
	default:
		return ""
	}
}

func terminalNavigationNoticeToPrivate(reason domain.TerminalNavigationNoticeReason) privatev1.TerminalNavigationNoticeReason {
	switch reason {
	case domain.TerminalNavigationNoticeTargetMissing:
		return privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_TARGET_MISSING
	case domain.TerminalNavigationNoticeSelfTarget:
		return privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_SELF_TARGET
	case domain.TerminalNavigationNoticeCommandStale:
		return privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_COMMAND_STALE
	case domain.TerminalNavigationNoticeTargetChanged:
		return privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_TARGET_CHANGED
	default:
		return privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_UNSPECIFIED
	}
}

func terminalNavigationNoticeFromPrivate(reason privatev1.TerminalNavigationNoticeReason) domain.TerminalNavigationNoticeReason {
	switch reason {
	case privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_TARGET_MISSING:
		return domain.TerminalNavigationNoticeTargetMissing
	case privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_SELF_TARGET:
		return domain.TerminalNavigationNoticeSelfTarget
	case privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_COMMAND_STALE:
		return domain.TerminalNavigationNoticeCommandStale
	case privatev1.TerminalNavigationNoticeReason_TERMINAL_NAVIGATION_NOTICE_REASON_TARGET_CHANGED:
		return domain.TerminalNavigationNoticeTargetChanged
	default:
		return ""
	}
}

func serverInfoToPrivate(info domain.ServerInfo) *privatev1.ServerInformation {
	localURL := info.LocalURL
	if localURL == "" && !info.Tunnel {
		localURL = info.URL
	}
	result := &privatev1.ServerInformation{LocalUrl: localURL, TunnelEnabled: info.Tunnel, Ip: info.IP, Port: int32(info.Port), Url: info.URL}
	if info.Tunnel && info.URL != "" {
		result.PublicUrl = &info.URL
	}
	if info.TunnelError != "" {
		result.TunnelError = &info.TunnelError
	}
	return result
}

func serverInfoFromPrivate(info *privatev1.ServerInformation) domain.ServerInfo {
	if info == nil {
		return domain.ServerInfo{}
	}
	result := domain.ServerInfo{IP: info.GetIp(), Port: int(info.GetPort()), URL: info.GetUrl(), LocalURL: info.GetLocalUrl(), Tunnel: info.GetTunnelEnabled(), TunnelError: info.GetTunnelError()}
	return result
}

func playerConfigMetadataToPrivate(metadata *domain.PlayerConfigMetadata) *privatev1.PlayerConfigMetadata {
	if metadata == nil {
		return nil
	}
	return &privatev1.PlayerConfigMetadata{Status: metadata.Status, FilePath: metadata.FilePath, Version: int32(metadata.Version), Name: metadata.Name}
}

func playerConfigMetadataFromPrivate(metadata *privatev1.PlayerConfigMetadata) *domain.PlayerConfigMetadata {
	if metadata == nil {
		return nil
	}
	return &domain.PlayerConfigMetadata{Status: metadata.GetStatus(), FilePath: metadata.GetFilePath(), Version: int(metadata.GetVersion()), Name: metadata.GetName()}
}

func publicHackToPrivate(state *domain.PublicHackState) *playerv1.PublicHackState {
	if state == nil {
		return nil
	}
	result := &playerv1.PublicHackState{
		Level: int32(state.Level), WordLength: int32(state.WordLength), AttemptsMax: int32(state.AttemptsMax),
		AttemptsLeft: int32(state.AttemptsLeft), Solved: state.Solved, Failed: state.Failed, Log: append([]string(nil), state.Log...),
	}
	for _, column := range state.Columns {
		mapped := &playerv1.PublicHackColumn{Addresses: append([]string(nil), column.Addresses...), Text: column.Text}
		for _, word := range column.Words {
			mapped.Words = append(mapped.Words, &playerv1.PublicHackWord{Id: word.ID, Start: int32(word.Start), Length: int32(word.Length)})
		}
		result.Columns = append(result.Columns, mapped)
	}
	for _, pattern := range state.Patterns {
		result.Patterns = append(result.Patterns, &playerv1.PublicHackPattern{PatternId: pattern.ID, Row: int32(pattern.Row), Start: int32(pattern.Start), End: int32(pattern.End), Used: pattern.Used})
	}
	return result
}

func publicHackFromPrivate(state *playerv1.PublicHackState) *domain.PublicHackState {
	if state == nil {
		return nil
	}
	result := &domain.PublicHackState{
		Level: int(state.GetLevel()), WordLength: int(state.GetWordLength()), AttemptsMax: int(state.GetAttemptsMax()),
		AttemptsLeft: int(state.GetAttemptsLeft()), Solved: state.GetSolved(), Failed: state.GetFailed(), Log: append([]string(nil), state.GetLog()...),
	}
	for _, column := range state.GetColumns() {
		mapped := domain.HackColumn{Addresses: append([]string(nil), column.GetAddresses()...), Text: column.GetText()}
		for _, word := range column.GetWords() {
			mapped.Words = append(mapped.Words, domain.HackWord{ID: word.GetId(), Start: int(word.GetStart()), Length: int(word.GetLength())})
		}
		result.Columns = append(result.Columns, mapped)
	}
	for _, pattern := range state.GetPatterns() {
		result.Patterns = append(result.Patterns, domain.PublicHackPattern{ID: pattern.GetPatternId(), Row: int(pattern.GetRow()), Start: int(pattern.GetStart()), End: int(pattern.GetEnd()), Used: pattern.GetUsed()})
	}
	return result
}

func playerRoleToPrivate(role domain.PlayerRole) playerv1.PlayerRole {
	switch role {
	case domain.PlayerRoleUnassigned:
		return playerv1.PlayerRole_PLAYER_ROLE_UNASSIGNED
	case domain.PlayerRoleActive:
		return playerv1.PlayerRole_PLAYER_ROLE_ACTIVE
	case domain.PlayerRoleObserver:
		return playerv1.PlayerRole_PLAYER_ROLE_OBSERVER
	default:
		return playerv1.PlayerRole_PLAYER_ROLE_UNSPECIFIED
	}
}

func playerRoleFromPrivate(role playerv1.PlayerRole) domain.PlayerRole {
	switch role {
	case playerv1.PlayerRole_PLAYER_ROLE_UNASSIGNED:
		return domain.PlayerRoleUnassigned
	case playerv1.PlayerRole_PLAYER_ROLE_ACTIVE:
		return domain.PlayerRoleActive
	case playerv1.PlayerRole_PLAYER_ROLE_OBSERVER:
		return domain.PlayerRoleObserver
	default:
		return ""
	}
}

func terminalSwitchStatusToPrivate(status string) privatev1.TerminalSwitchStatus {
	switch status {
	case "activated":
		return privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_ACTIVATED
	case "cleared":
		return privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_CLEARED
	case "pending":
		return privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_PENDING
	case "preserved":
		return privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_PRESERVED
	case "discarded":
		return privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_DISCARDED
	case "cancelled", "canceled":
		return privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_CANCELED
	case "decision-required":
		return privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_DECISION_REQUIRED
	default:
		return privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_UNSPECIFIED
	}
}

func terminalSwitchStatusFromPrivate(status privatev1.TerminalSwitchStatus) string {
	switch status {
	case privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_ACTIVATED:
		return "activated"
	case privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_CLEARED:
		return "cleared"
	case privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_PENDING:
		return "pending"
	case privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_PRESERVED:
		return "preserved"
	case privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_DISCARDED:
		return "discarded"
	case privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_CANCELED:
		return "cancelled"
	case privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_DECISION_REQUIRED:
		return "decision-required"
	default:
		return ""
	}
}

func terminalSwitchChoiceToPrivate(choice domain.TerminalSwitchChoice) (privatev1.TerminalSwitchChoice, error) {
	switch choice {
	case domain.TerminalSwitchPreserve:
		return privatev1.TerminalSwitchChoice_TERMINAL_SWITCH_CHOICE_PRESERVE, nil
	case domain.TerminalSwitchDiscard:
		return privatev1.TerminalSwitchChoice_TERMINAL_SWITCH_CHOICE_DISCARD, nil
	case domain.TerminalSwitchCancel:
		return privatev1.TerminalSwitchChoice_TERMINAL_SWITCH_CHOICE_CANCEL, nil
	default:
		return privatev1.TerminalSwitchChoice_TERMINAL_SWITCH_CHOICE_UNSPECIFIED, fmt.Errorf("unsupported terminal switch choice %q", choice)
	}
}

func terminalSwitchChoiceFromPrivate(choice privatev1.TerminalSwitchChoice) domain.TerminalSwitchChoice {
	switch choice {
	case privatev1.TerminalSwitchChoice_TERMINAL_SWITCH_CHOICE_PRESERVE:
		return domain.TerminalSwitchPreserve
	case privatev1.TerminalSwitchChoice_TERMINAL_SWITCH_CHOICE_DISCARD:
		return domain.TerminalSwitchDiscard
	case privatev1.TerminalSwitchChoice_TERMINAL_SWITCH_CHOICE_CANCEL:
		return domain.TerminalSwitchCancel
	default:
		return ""
	}
}

func commandExecutionDecisionToPrivate(decision domain.CommandExecutionDecision) (privatev1.CommandExecutionDecision, error) {
	switch decision {
	case domain.CommandExecutionApprove:
		return privatev1.CommandExecutionDecision_COMMAND_EXECUTION_DECISION_APPROVE, nil
	case domain.CommandExecutionReject:
		return privatev1.CommandExecutionDecision_COMMAND_EXECUTION_DECISION_REJECT, nil
	default:
		return privatev1.CommandExecutionDecision_COMMAND_EXECUTION_DECISION_UNSPECIFIED, fmt.Errorf("unsupported command execution decision %q", decision)
	}
}

func commandExecutionDecisionFromPrivate(decision privatev1.CommandExecutionDecision) domain.CommandExecutionDecision {
	switch decision {
	case privatev1.CommandExecutionDecision_COMMAND_EXECUTION_DECISION_APPROVE:
		return domain.CommandExecutionApprove
	case privatev1.CommandExecutionDecision_COMMAND_EXECUTION_DECISION_REJECT:
		return domain.CommandExecutionReject
	default:
		return ""
	}
}

func terminalNavigationDecisionToPrivate(decision domain.TerminalNavigationDecision) (privatev1.TerminalNavigationDecision, error) {
	switch decision {
	case domain.TerminalNavigationApprove:
		return privatev1.TerminalNavigationDecision_TERMINAL_NAVIGATION_DECISION_APPROVE, nil
	case domain.TerminalNavigationReject:
		return privatev1.TerminalNavigationDecision_TERMINAL_NAVIGATION_DECISION_REJECT, nil
	default:
		return privatev1.TerminalNavigationDecision_TERMINAL_NAVIGATION_DECISION_UNSPECIFIED, fmt.Errorf("unsupported terminal navigation decision %q", decision)
	}
}

func terminalNavigationDecisionFromPrivate(decision privatev1.TerminalNavigationDecision) domain.TerminalNavigationDecision {
	switch decision {
	case privatev1.TerminalNavigationDecision_TERMINAL_NAVIGATION_DECISION_APPROVE:
		return domain.TerminalNavigationApprove
	case privatev1.TerminalNavigationDecision_TERMINAL_NAVIGATION_DECISION_REJECT:
		return domain.TerminalNavigationReject
	default:
		return ""
	}
}
