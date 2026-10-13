package main

import (
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	sessionservice "github.com/obalunenko/Fallout-Terminal/v2/internal/session"
)

// desktopService is the complete generated desktop allowlist. It forwards to
// the transport-neutral core so validation, protobuf adapters, authorization,
// persistence, and redaction remain owned by existing application services.
// The core App itself is deliberately never registered with Wails.
type desktopService struct {
	core *App
}

func newDesktopService(core *App) *desktopService {
	return &desktopService{core: core}
}

func (service *desktopService) GetRuntimeStatus() RuntimeStatus {
	return service.core.GetRuntimeStatus()
}

func (service *desktopService) GetApplicationUpdateStatus() ApplicationUpdateSnapshot {
	return service.core.GetApplicationUpdateStatus()
}

func (service *desktopService) ResolveApplicationUpdateOffer(payload ApplicationUpdateOfferDecisionPayload) ApplicationUpdateCommandResult {
	return service.core.ResolveApplicationUpdateOffer(payload)
}

func (service *desktopService) ResolveApplicationUpdateRestart(payload ApplicationUpdateRestartDecisionPayload) ApplicationUpdateCommandResult {
	return service.core.ResolveApplicationUpdateRestart(payload)
}

func (service *desktopService) NewSession() sessionservice.SessionResult {
	return service.core.NewSession()
}

func (service *desktopService) OpenSession() sessionservice.SessionResult {
	return service.core.OpenSession()
}

func (service *desktopService) CopyDemo() sessionservice.SessionResult {
	return service.core.CopyDemo()
}

func (service *desktopService) SaveSession(session domain.Session) sessionservice.SaveResult {
	return service.core.SaveSession(session)
}

func (service *desktopService) ReplaceTerminalGroups(payload TerminalGroupReplacementPayload) TerminalGroupReplacementResult {
	return service.core.ReplaceTerminalGroups(payload)
}

func (service *desktopService) LoadReferencedPlayerConfig() PlayerConfigCommandResult {
	return service.core.LoadReferencedPlayerConfig()
}

func (service *desktopService) NewPlayerConfig() PlayerConfigCommandResult {
	return service.core.NewPlayerConfig()
}

func (service *desktopService) OpenPlayerConfig() PlayerConfigCommandResult {
	return service.core.OpenPlayerConfig()
}

func (service *desktopService) RequestTerminalActivation(payload LiveTerminalPayload) TerminalSwitchCommandResult {
	return service.core.RequestTerminalActivation(payload)
}

func (service *desktopService) UpdateLiveTerminal(payload LiveUpdatePayload) CoordinationCommandResult {
	return service.core.UpdateLiveTerminal(payload)
}

func (service *desktopService) RequestTerminalClear() TerminalSwitchCommandResult {
	return service.core.RequestTerminalClear()
}

func (service *desktopService) ResolveTerminalSwitch(payload TerminalSwitchDecisionPayload) TerminalSwitchCommandResult {
	return service.core.ResolveTerminalSwitch(payload)
}

func (service *desktopService) ResolveCommandExecution(payload CommandExecutionDecisionPayload) ResolveCommandExecutionResult {
	return service.core.ResolveCommandExecution(payload)
}

func (service *desktopService) ResolveTerminalNavigation(payload TerminalNavigationDecisionPayload) ResolveTerminalNavigationResult {
	return service.core.ResolveTerminalNavigation(payload)
}

func (service *desktopService) ForceHackSuccess() CommandResult {
	return service.core.ForceHackSuccess()
}

func (service *desktopService) ResetFailedHack(payload LiveTerminalPayload) CoordinationCommandResult {
	return service.core.ResetFailedHack(payload)
}

func (service *desktopService) ResetCommandState(payload ResetCommandStatePayload) SessionStateResult {
	return service.core.ResetCommandState(payload)
}

func (service *desktopService) ResetTerminalCommandStates(payload ResetTerminalCommandStatesPayload) SessionStateResult {
	return service.core.ResetTerminalCommandStates(payload)
}

func (service *desktopService) AddCharacter(payload CharacterCreatePayload) CoordinationCommandResult {
	return service.core.AddCharacter(payload)
}

func (service *desktopService) UpdateCharacter(payload CharacterUpdatePayload) CoordinationCommandResult {
	return service.core.UpdateCharacter(payload)
}

func (service *desktopService) DeleteCharacter(payload CharacterDeletePayload) CoordinationCommandResult {
	return service.core.DeleteCharacter(payload)
}

func (service *desktopService) RenameLogicalSession(payload LogicalSessionRenamePayload) CoordinationCommandResult {
	return service.core.RenameLogicalSession(payload)
}

func (service *desktopService) AssignCharacter(payload AssignmentPayload) CoordinationCommandResult {
	return service.core.AssignCharacter(payload)
}

func (service *desktopService) ReleaseCharacter(sessionID string) CoordinationCommandResult {
	return service.core.ReleaseCharacter(sessionID)
}

func (service *desktopService) MoveCharacter(payload MoveCharacterPayload) CoordinationCommandResult {
	return service.core.MoveCharacter(payload)
}

func (service *desktopService) SetActiveController(sessionID string) CoordinationCommandResult {
	return service.core.SetActiveController(sessionID)
}

func (service *desktopService) StartBroadcast() CoordinationCommandResult {
	return service.core.StartBroadcast()
}

func (service *desktopService) EndBroadcast() CoordinationCommandResult {
	return service.core.EndBroadcast()
}

func (service *desktopService) OpenURL(rawURL string) CommandResult {
	return service.core.OpenURL(rawURL)
}

func (service *desktopService) OpenLogLocation() LogAccessResult {
	return service.core.OpenLogLocation()
}

func (service *desktopService) GetPublicAccess() PublicAccessSnapshot {
	return service.core.GetPublicAccess()
}

func (service *desktopService) CopyPublicAccessCredentials() CommandResult {
	return service.core.CopyPublicAccessCredentials()
}

func (service *desktopService) SavePublicAccessSettings(payload SavePublicAccessSettingsPayload) PublicAccessCommandResult {
	return service.core.SavePublicAccessSettings(payload)
}

func (service *desktopService) GeneratePlayerPassword(payload PublicAccessCommandPayload) GeneratedPlayerPasswordResult {
	return service.core.GeneratePlayerPassword(payload)
}

func (service *desktopService) StartPublicAccess(payload PublicAccessCommandPayload) PublicAccessCommandResult {
	return service.core.StartPublicAccess(payload)
}

func (service *desktopService) StopPublicAccess(payload PublicAccessCommandPayload) PublicAccessCommandResult {
	return service.core.StopPublicAccess(payload)
}
