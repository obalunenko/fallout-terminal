import type {
  DesktopApplicationUpdateSnapshot,
  DesktopCommandResult,
  DesktopDocumentResult,
  DesktopPublicAccessSnapshot,
  DesktopRecord,
  DesktopRuntimeStatus,
} from '../models/overseer-view-state.js';

export type DesktopUnsubscribe = () => void;
export type DesktopEventListener<T> = (value: Readonly<T>) => void;

export interface DesktopPort {
  onServerInfo(listener: DesktopEventListener<DesktopRecord>): DesktopUnsubscribe;
  onClientCount(listener: DesktopEventListener<number>): DesktopUnsubscribe;
  onHackState(listener: DesktopEventListener<DesktopRecord | null>): DesktopUnsubscribe;
  onCoordinationState(listener: DesktopEventListener<DesktopRecord>): DesktopUnsubscribe;
  onSessionState(listener: DesktopEventListener<DesktopRecord>): DesktopUnsubscribe;
  onPublicAccessStatus(listener: DesktopEventListener<DesktopPublicAccessSnapshot>): DesktopUnsubscribe;
  onApplicationUpdateStatus(listener: DesktopEventListener<DesktopApplicationUpdateSnapshot>): DesktopUnsubscribe;

  getRuntimeStatus(): Promise<DesktopRuntimeStatus>;
  openUrl(url: string): Promise<DesktopCommandResult>;
  writeClipboardText(value: string): Promise<boolean>;
  openSession(): Promise<DesktopDocumentResult>;
  newSession(): Promise<DesktopDocumentResult>;
  saveSession(session: DesktopRecord): Promise<DesktopCommandResult>;
  loadReferencedPlayerConfig(): Promise<DesktopDocumentResult>;
  newPlayerConfig(): Promise<DesktopDocumentResult>;
  openPlayerConfig(): Promise<DesktopDocumentResult>;
  requestTerminalActivation(request: DesktopRecord): Promise<DesktopCommandResult>;
  updateLiveTerminal(request: DesktopRecord): Promise<DesktopCommandResult>;
  requestTerminalClear(): Promise<DesktopCommandResult>;
  resolveTerminalSwitch(request: DesktopRecord): Promise<DesktopCommandResult>;
  resolveCommandExecution(request: DesktopRecord): Promise<DesktopCommandResult>;
  resolveTerminalNavigation(request: DesktopRecord): Promise<DesktopCommandResult>;
  forceHackSuccess(): Promise<DesktopCommandResult>;
  resetFailedHack(request: DesktopRecord): Promise<DesktopCommandResult>;
  resetCommandState(request: DesktopRecord): Promise<DesktopCommandResult>;
  resetTerminalCommandStates(request: DesktopRecord): Promise<DesktopCommandResult>;
  replaceTerminalGroups(request: DesktopRecord): Promise<DesktopCommandResult>;
  addCharacter(request: DesktopRecord): Promise<DesktopCommandResult>;
  updateCharacter(request: DesktopRecord): Promise<DesktopCommandResult>;
  deleteCharacter(request: DesktopRecord): Promise<DesktopCommandResult>;
  renameLogicalSession(request: DesktopRecord): Promise<DesktopCommandResult>;
  assignCharacter(request: DesktopRecord): Promise<DesktopCommandResult>;
  releaseCharacter(sessionId: string): Promise<DesktopCommandResult>;
  moveCharacter(request: DesktopRecord): Promise<DesktopCommandResult>;
  setActiveController(sessionId: string): Promise<DesktopCommandResult>;
  startBroadcast(): Promise<DesktopCommandResult>;
  endBroadcast(): Promise<DesktopCommandResult>;
  getPublicAccess(): Promise<DesktopPublicAccessSnapshot>;
  copyPublicAccessCredentials(): Promise<DesktopCommandResult>;
  savePublicAccessSettings(request: DesktopRecord): Promise<DesktopCommandResult>;
  generatePlayerPassword(request: DesktopRecord): Promise<DesktopCommandResult>;
  startPublicAccess(request: DesktopRecord): Promise<DesktopCommandResult>;
  stopPublicAccess(request: DesktopRecord): Promise<DesktopCommandResult>;
  resolveApplicationUpdateOffer(request: DesktopRecord): Promise<DesktopCommandResult>;
  resolveApplicationUpdateRestart(request: DesktopRecord): Promise<DesktopCommandResult>;
}
