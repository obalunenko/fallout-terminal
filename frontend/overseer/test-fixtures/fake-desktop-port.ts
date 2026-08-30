import type {
  DesktopApplicationUpdateSnapshot,
  DesktopCommandResult,
  DesktopDocumentResult,
  DesktopPublicAccessSnapshot,
  DesktopRecord,
  DesktopRuntimeStatus,
} from '../src/models/overseer-view-state.js';
import type {
  DesktopEventListener,
  DesktopPort,
  DesktopUnsubscribe,
} from '../src/ports/desktop-port.js';

export interface FakeDesktopPortEvents {
  emitApplicationUpdateStatus(value: DesktopApplicationUpdateSnapshot): void;
  emitClientCount(value: number): void;
  emitCoordinationState(value: DesktopRecord): void;
  emitHackState(value: DesktopRecord): void;
  emitPublicAccessStatus(value: DesktopPublicAccessSnapshot): void;
  emitServerInfo(value: DesktopRecord): void;
  emitSessionState(value: DesktopRecord): void;
}

export interface FakeDesktopPortFixture {
  readonly events: FakeDesktopPortEvents;
  readonly port: DesktopPort;
}

function subscribe<T>(
  listeners: Set<DesktopEventListener<T>>,
  listener: DesktopEventListener<T>,
): DesktopUnsubscribe {
  listeners.add(listener);
  let active = true;
  return () => {
    if (!active) return;
    active = false;
    listeners.delete(listener);
  };
}

function emit<T>(listeners: ReadonlySet<DesktopEventListener<T>>, value: T): void {
  for (const listener of listeners) listener(value);
}

function command(): Promise<DesktopCommandResult> {
  return Promise.resolve(Object.freeze({ ok: true, error: '' }));
}

function document(): Promise<DesktopDocumentResult> {
  return Promise.resolve(Object.freeze({ ok: true, error: '', canceled: false, session: null }));
}

export function createFakeDesktopPort(): FakeDesktopPortFixture {
  const applicationUpdateListeners = new Set<DesktopEventListener<DesktopApplicationUpdateSnapshot>>();
  const clientCountListeners = new Set<DesktopEventListener<number>>();
  const coordinationStateListeners = new Set<DesktopEventListener<DesktopRecord>>();
  const hackStateListeners = new Set<DesktopEventListener<DesktopRecord>>();
  const publicAccessListeners = new Set<DesktopEventListener<DesktopPublicAccessSnapshot>>();
  const serverInfoListeners = new Set<DesktopEventListener<DesktopRecord>>();
  const sessionStateListeners = new Set<DesktopEventListener<DesktopRecord>>();

  const runtimeStatus: DesktopRuntimeStatus = Object.freeze({ ok: true });
  const publicAccess: DesktopPublicAccessSnapshot = Object.freeze({ generation: 0, settingsRevision: 0 });

  const port: DesktopPort = {
    onServerInfo: listener => subscribe(serverInfoListeners, listener),
    onClientCount: listener => subscribe(clientCountListeners, listener),
    onHackState: listener => subscribe(hackStateListeners, listener),
    onCoordinationState: listener => subscribe(coordinationStateListeners, listener),
    onSessionState: listener => subscribe(sessionStateListeners, listener),
    onPublicAccessStatus: listener => subscribe(publicAccessListeners, listener),
    onApplicationUpdateStatus: listener => subscribe(applicationUpdateListeners, listener),
    getRuntimeStatus: () => Promise.resolve(runtimeStatus),
    openUrl: command,
    writeClipboardText: value => Promise.resolve(value.length > 0),
    openSession: document,
    newSession: document,
    saveSession: command,
    loadReferencedPlayerConfig: document,
    newPlayerConfig: document,
    openPlayerConfig: document,
    requestTerminalActivation: command,
    updateLiveTerminal: command,
    requestTerminalClear: command,
    resolveTerminalSwitch: command,
    resolveCommandExecution: command,
    resolveTerminalNavigation: command,
    forceHackSuccess: command,
    resetFailedHack: command,
    resetCommandState: command,
    resetTerminalCommandStates: command,
    replaceTerminalGroups: command,
    addCharacter: command,
    updateCharacter: command,
    deleteCharacter: command,
    renameLogicalSession: command,
    assignCharacter: command,
    releaseCharacter: command,
    moveCharacter: command,
    setActiveController: command,
    startBroadcast: command,
    endBroadcast: command,
    getPublicAccess: () => Promise.resolve(publicAccess),
    copyPublicAccessCredentials: command,
    savePublicAccessSettings: command,
    generatePlayerPassword: command,
    startPublicAccess: command,
    stopPublicAccess: command,
    resolveApplicationUpdateOffer: command,
    resolveApplicationUpdateRestart: command,
  };

  const events: FakeDesktopPortEvents = {
    emitApplicationUpdateStatus: value => emit(applicationUpdateListeners, value),
    emitClientCount: value => emit(clientCountListeners, value),
    emitCoordinationState: value => emit(coordinationStateListeners, value),
    emitHackState: value => emit(hackStateListeners, value),
    emitPublicAccessStatus: value => emit(publicAccessListeners, value),
    emitServerInfo: value => emit(serverInfoListeners, value),
    emitSessionState: value => emit(sessionStateListeners, value),
  };
  return Object.freeze({ port: Object.freeze(port), events: Object.freeze(events) });
}

export const fakeDesktopPort = createFakeDesktopPort().port;
