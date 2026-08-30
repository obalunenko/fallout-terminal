declare module '#wails-service' {
  export type WailsServiceResult = Promise<unknown>;

  export interface WailsDesktopEventMap {
    readonly 'application-update-status': unknown;
    readonly 'client-count': unknown;
    readonly 'coordination-state': unknown;
    readonly 'hack-state': unknown;
    readonly 'public-access-status': unknown;
    readonly 'server-info': unknown;
    readonly 'session-state': unknown;
  }

  export function AddCharacter(payload: unknown): WailsServiceResult;
  export function AssignCharacter(payload: unknown): WailsServiceResult;
  export function CopyDemo(): WailsServiceResult;
  export function CopyPublicAccessCredentials(): WailsServiceResult;
  export function DeleteCharacter(payload: unknown): WailsServiceResult;
  export function EndBroadcast(): WailsServiceResult;
  export function ForceHackSuccess(): WailsServiceResult;
  export function GeneratePlayerPassword(payload: unknown): WailsServiceResult;
  export function GetApplicationUpdateStatus(): WailsServiceResult;
  export function GetPublicAccess(): WailsServiceResult;
  export function GetRuntimeStatus(): WailsServiceResult;
  export function LoadReferencedPlayerConfig(): WailsServiceResult;
  export function MoveCharacter(payload: unknown): WailsServiceResult;
  export function NewPlayerConfig(): WailsServiceResult;
  export function NewSession(): WailsServiceResult;
  export function OpenPlayerConfig(): WailsServiceResult;
  export function OpenSession(): WailsServiceResult;
  export function OpenURL(rawURL: string): WailsServiceResult;
  export function ReleaseCharacter(sessionID: string): WailsServiceResult;
  export function RenameLogicalSession(payload: unknown): WailsServiceResult;
  export function ReplaceTerminalGroups(payload: unknown): WailsServiceResult;
  export function RequestTerminalActivation(payload: unknown): WailsServiceResult;
  export function RequestTerminalClear(): WailsServiceResult;
  export function ResetCommandState(payload: unknown): WailsServiceResult;
  export function ResetFailedHack(payload: unknown): WailsServiceResult;
  export function ResetTerminalCommandStates(payload: unknown): WailsServiceResult;
  export function ResolveApplicationUpdateOffer(payload: unknown): WailsServiceResult;
  export function ResolveApplicationUpdateRestart(payload: unknown): WailsServiceResult;
  export function ResolveCommandExecution(payload: unknown): WailsServiceResult;
  export function ResolveTerminalNavigation(payload: unknown): WailsServiceResult;
  export function ResolveTerminalSwitch(payload: unknown): WailsServiceResult;
  export function SavePublicAccessSettings(payload: unknown): WailsServiceResult;
  export function SaveSession(session: unknown): WailsServiceResult;
  export function SetActiveController(sessionID: string): WailsServiceResult;
  export function StartBroadcast(): WailsServiceResult;
  export function StartPublicAccess(payload: unknown): WailsServiceResult;
  export function StopPublicAccess(payload: unknown): WailsServiceResult;
  export function UpdateCharacter(payload: unknown): WailsServiceResult;
  export function UpdateLiveTerminal(payload: unknown): WailsServiceResult;
}
