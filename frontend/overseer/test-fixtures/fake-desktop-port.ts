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

export interface FakeDesktopCall {
  readonly args: readonly DesktopRecord[];
  readonly method: string;
}

export interface FakeApplicationUpdateFailure {
  readonly errorMessage: string;
  readonly failedStage: string;
  readonly recoveryAction: string;
}

export interface FakeDesktopPortFixture {
  readonly applicationUpdateDownloadCount: () => number;
  readonly applicationUpdateReleaseCount: () => number;
  readonly applicationUpdateRestartHandoffCount: () => number;
  readonly applicationUpdateSnapshot: () => DesktopApplicationUpdateSnapshot;
  readonly calls: readonly FakeDesktopCall[];
  readonly events: FakeDesktopPortEvents;
  readonly failNextApplicationUpdatePreparation: (failure: FakeApplicationUpdateFailure) => void;
  readonly port: DesktopPort;
}

function subscribe<T>(
  listeners: Set<DesktopEventListener<T>>,
  listener: DesktopEventListener<T>,
  onRelease?: () => void,
): DesktopUnsubscribe {
  listeners.add(listener);
  let active = true;
  return () => {
    if (!active) return;
    active = false;
    listeners.delete(listener);
    onRelease?.();
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

function updateResult(
  ok: boolean,
  snapshot: DesktopApplicationUpdateSnapshot,
  error = '',
): DesktopCommandResult {
  return Object.freeze({ ok, error, snapshot: structuredClone(snapshot) });
}

export function createFakeDesktopPort(): FakeDesktopPortFixture {
  const applicationUpdateListeners = new Set<DesktopEventListener<DesktopApplicationUpdateSnapshot>>();
  const clientCountListeners = new Set<DesktopEventListener<number>>();
  const coordinationStateListeners = new Set<DesktopEventListener<DesktopRecord>>();
  const hackStateListeners = new Set<DesktopEventListener<DesktopRecord>>();
  const publicAccessListeners = new Set<DesktopEventListener<DesktopPublicAccessSnapshot>>();
  const serverInfoListeners = new Set<DesktopEventListener<DesktopRecord>>();
  const sessionStateListeners = new Set<DesktopEventListener<DesktopRecord>>();
  const calls: FakeDesktopCall[] = [];
  let applicationUpdate: DesktopApplicationUpdateSnapshot = Object.freeze({
    revision: 0,
    state: 'disabled',
  });
  let applicationUpdateDownloads = 0;
  let applicationUpdateReleases = 0;
  let applicationUpdateRestartHandoffs = 0;
  let applicationUpdatePreparationFailure: FakeApplicationUpdateFailure | null = null;

  const runtimeStatus: DesktopRuntimeStatus = Object.freeze({ ok: true });
  const publicAccess: DesktopPublicAccessSnapshot = Object.freeze({ generation: 0, settingsRevision: 0 });

  function retainApplicationUpdate(value: DesktopApplicationUpdateSnapshot): void {
    if (!Number.isSafeInteger(value.revision) || value.revision < 0) return;
    if (value.revision < applicationUpdate.revision) return;
    applicationUpdate = Object.freeze(structuredClone(value));
  }

  function applicationUpdateCommand(
    method: string,
    request: DesktopRecord,
    decisions: readonly string[],
  ): string | null {
    const attemptId = typeof request.attemptId === 'string' ? request.attemptId : '';
    const decision = typeof request.decision === 'string' ? request.decision : '';
    calls.push(Object.freeze({ method, args: [Object.freeze({ attemptId, decision })] }));
    if (attemptId !== applicationUpdate.attemptId || !decisions.includes(decision)) return null;
    return decision;
  }

  async function resolveApplicationUpdateOffer(request: DesktopRecord): Promise<DesktopCommandResult> {
    const decision = applicationUpdateCommand(
      'ResolveApplicationUpdateOffer',
      request,
      ['accept', 'defer'],
    );
    if (decision === null || applicationUpdate.state !== 'available') {
      return updateResult(false, applicationUpdate, 'Application update offer is no longer available');
    }
    if (decision === 'defer') {
      retainApplicationUpdate({
        ...applicationUpdate,
        revision: applicationUpdate.revision + 1,
        state: 'deferred',
      });
    } else {
      applicationUpdateDownloads += 1;
      if (applicationUpdatePreparationFailure !== null) {
        const failure = applicationUpdatePreparationFailure;
        applicationUpdatePreparationFailure = null;
        retainApplicationUpdate({
          ...applicationUpdate,
          ...failure,
          revision: applicationUpdate.revision + 1,
          state: 'failed',
        });
      }
    }
    return updateResult(true, applicationUpdate);
  }

  async function resolveApplicationUpdateRestart(request: DesktopRecord): Promise<DesktopCommandResult> {
    const decision = applicationUpdateCommand(
      'ResolveApplicationUpdateRestart',
      request,
      ['postpone', 'restart'],
    );
    if (decision === null || applicationUpdate.state !== 'ready-to-restart') {
      return updateResult(false, applicationUpdate, 'Application update is not ready to restart');
    }
    if (decision === 'restart') {
      applicationUpdateRestartHandoffs += 1;
      retainApplicationUpdate({
        ...applicationUpdate,
        revision: applicationUpdate.revision + 1,
        state: 'applying',
      });
    }
    return updateResult(true, applicationUpdate);
  }

  const port: DesktopPort = {
    onServerInfo: listener => subscribe(serverInfoListeners, listener),
    onClientCount: listener => subscribe(clientCountListeners, listener),
    onHackState: listener => subscribe(hackStateListeners, listener),
    onCoordinationState: listener => subscribe(coordinationStateListeners, listener),
    onSessionState: listener => subscribe(sessionStateListeners, listener),
    onPublicAccessStatus: listener => subscribe(publicAccessListeners, listener),
    onApplicationUpdateStatus: listener => {
      calls.push(Object.freeze({ method: 'event:on:application-update-status', args: [] }));
      return subscribe(
        applicationUpdateListeners,
        listener,
        () => { applicationUpdateReleases += 1; },
      );
    },
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
    resolveApplicationUpdateOffer,
    resolveApplicationUpdateRestart,
  };

  const events: FakeDesktopPortEvents = {
    emitApplicationUpdateStatus: value => {
      retainApplicationUpdate(value);
      emit(applicationUpdateListeners, value);
    },
    emitClientCount: value => emit(clientCountListeners, value),
    emitCoordinationState: value => emit(coordinationStateListeners, value),
    emitHackState: value => emit(hackStateListeners, value),
    emitPublicAccessStatus: value => emit(publicAccessListeners, value),
    emitServerInfo: value => emit(serverInfoListeners, value),
    emitSessionState: value => emit(sessionStateListeners, value),
  };
  return Object.freeze({
    applicationUpdateDownloadCount: () => applicationUpdateDownloads,
    applicationUpdateReleaseCount: () => applicationUpdateReleases,
    applicationUpdateRestartHandoffCount: () => applicationUpdateRestartHandoffs,
    applicationUpdateSnapshot: () => structuredClone(applicationUpdate),
    calls,
    events: Object.freeze(events),
    failNextApplicationUpdatePreparation: (failure: FakeApplicationUpdateFailure) => {
      applicationUpdatePreparationFailure = Object.freeze(structuredClone(failure));
    },
    port: Object.freeze(port),
  });
}

export const fakeDesktopFixture = createFakeDesktopPort();
export const fakeDesktopPort = fakeDesktopFixture.port;
