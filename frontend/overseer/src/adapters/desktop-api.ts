import { Clipboard, Events } from '@wailsio/runtime';
import * as desktopService from '#wails-service';

import type {
  DesktopApplicationUpdateSnapshot,
  DesktopCommandResult,
  DesktopDocumentResult,
  DesktopPublicAccessSnapshot,
  DesktopRecord,
  DesktopRuntimeStatus,
} from '../models/overseer-view-state.js';
import type {
  DesktopEventListener,
  DesktopPort,
  DesktopUnsubscribe,
} from '../ports/desktop-port.js';

const APP_METHODS = Object.freeze({
  getRuntimeStatus: desktopService.GetRuntimeStatus,
  getApplicationUpdateStatus: desktopService.GetApplicationUpdateStatus,
  newSession: desktopService.NewSession,
  openSession: desktopService.OpenSession,
  saveSession: desktopService.SaveSession,
  loadReferencedPlayerConfig: desktopService.LoadReferencedPlayerConfig,
  newPlayerConfig: desktopService.NewPlayerConfig,
  openPlayerConfig: desktopService.OpenPlayerConfig,
  requestTerminalActivation: desktopService.RequestTerminalActivation,
  updateLiveTerminal: desktopService.UpdateLiveTerminal,
  requestTerminalClear: desktopService.RequestTerminalClear,
  resolveTerminalSwitch: desktopService.ResolveTerminalSwitch,
  resolveCommandExecution: desktopService.ResolveCommandExecution,
  resolveTerminalNavigation: desktopService.ResolveTerminalNavigation,
  forceHackSuccess: desktopService.ForceHackSuccess,
  resetFailedHack: desktopService.ResetFailedHack,
  resetCommandState: desktopService.ResetCommandState,
  resetTerminalCommandStates: desktopService.ResetTerminalCommandStates,
  resolveApplicationUpdateOffer: desktopService.ResolveApplicationUpdateOffer,
  resolveApplicationUpdateRestart: desktopService.ResolveApplicationUpdateRestart,
  replaceTerminalGroups: desktopService.ReplaceTerminalGroups,
  addCharacter: desktopService.AddCharacter,
  updateCharacter: desktopService.UpdateCharacter,
  deleteCharacter: desktopService.DeleteCharacter,
  renameLogicalSession: desktopService.RenameLogicalSession,
  assignCharacter: desktopService.AssignCharacter,
  releaseCharacter: desktopService.ReleaseCharacter,
  moveCharacter: desktopService.MoveCharacter,
  setActiveController: desktopService.SetActiveController,
  startBroadcast: desktopService.StartBroadcast,
  endBroadcast: desktopService.EndBroadcast,
  openUrl: desktopService.OpenURL,
  getPublicAccess: desktopService.GetPublicAccess,
  copyPublicAccessCredentials: desktopService.CopyPublicAccessCredentials,
  savePublicAccessSettings: desktopService.SavePublicAccessSettings,
  generatePlayerPassword: desktopService.GeneratePlayerPassword,
  startPublicAccess: desktopService.StartPublicAccess,
  stopPublicAccess: desktopService.StopPublicAccess,
});

type MutableRecord = Record<string, unknown>;
type StatusField = 'serverInfo' | 'clientCount' | 'hackState' | 'coordinationState' | 'sessionState';

interface StatusSubscription {
  readonly deliver: (value: unknown) => void;
}

function isRecord(value: unknown): value is MutableRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function detached(value: unknown): unknown {
  if (typeof globalThis.structuredClone === 'function') {
    return globalThis.structuredClone(value);
  }
  return JSON.parse(JSON.stringify(value));
}

function detachedRecord(value: unknown): DesktopRecord | null {
  if (!isRecord(value)) return null;
  const copy = detached(value);
  return isRecord(copy) ? Object.freeze(copy) : null;
}

function failure(message: string): DesktopCommandResult {
  return Object.freeze({ ok: false, error: message });
}

async function invoke<TArgs extends unknown[]>(
  binding: (...args: TArgs) => Promise<unknown>,
  ...args: TArgs
): Promise<unknown> {
  try {
    return await binding(...args);
  } catch (error: unknown) {
    return failure(error instanceof Error ? error.message : String(error));
  }
}

function commandResult(value: unknown): DesktopCommandResult {
  const record = detachedRecord(value);
  if (record === null || typeof record.ok !== 'boolean') {
    return failure('Wails command returned an invalid result');
  }
  if (record.error !== undefined && typeof record.error !== 'string') {
    return failure('Wails command returned an invalid error');
  }
  return Object.freeze({ ...record, ok: record.ok, error: record.error ?? '' });
}

async function command<TArgs extends unknown[]>(
  binding: (...args: TArgs) => Promise<unknown>,
  ...args: TArgs
): Promise<DesktopCommandResult> {
  return commandResult(await invoke(binding, ...args));
}

function documentResult(value: unknown): DesktopDocumentResult {
  const result = commandResult(value);
  if (!result.ok) return Object.freeze({ ...result, canceled: false, session: null });
  const canceled = result.canceled === undefined ? false : result.canceled;
  const session = result.session === null || result.session === undefined
    ? null
    : detachedRecord(result.session);
  if (typeof canceled !== 'boolean' || (result.session != null && session === null)) {
    return Object.freeze({ ...failure('Wails document command returned an invalid result'), canceled: false, session: null });
  }
  return Object.freeze({ ...result, canceled, session });
}

async function documentCommand(binding: () => Promise<unknown>): Promise<DesktopDocumentResult> {
  return documentResult(await invoke(binding));
}

function runtimeStatus(value: unknown): DesktopRuntimeStatus {
  const record = detachedRecord(value);
  if (record === null || (record.ok !== undefined && typeof record.ok !== 'boolean')) {
    return Object.freeze({ ok: false });
  }
  return Object.freeze({ ...record, ok: record.ok !== false });
}

function eventRecord(value: unknown): DesktopRecord | null {
  return detachedRecord(value);
}

function serverInfo(value: unknown): DesktopRecord | null {
  const record = detachedRecord(value);
  if (record === null || typeof record.url !== 'string' || typeof record.tunnel !== 'boolean') {
    return null;
  }
  if (record.port !== undefined && !Number.isInteger(record.port)) return null;
  for (const field of ['ip', 'localUrl', 'tunnelError']) {
    if (record[field] !== undefined && typeof record[field] !== 'string') return null;
  }
  return record;
}

function eventCount(value: unknown): number | null {
  return Number.isSafeInteger(value) && Number(value) >= 0 ? Number(value) : null;
}

function publicAccessSnapshot(value: unknown): DesktopPublicAccessSnapshot | null {
  const record = detachedRecord(value);
  if (record === null) return null;
  const status = isRecord(record.status) ? record.status : record;
  const generation = status.generation;
  const settingsRevision = status.settingsRevision;
  if (!Number.isSafeInteger(generation) || Number(generation) < 0
    || !Number.isSafeInteger(settingsRevision) || Number(settingsRevision) < 0) {
    return null;
  }
  return Object.freeze({
    ...record,
    generation: Number(generation),
    settingsRevision: Number(settingsRevision),
  });
}

function applicationUpdateSnapshot(value: unknown): DesktopApplicationUpdateSnapshot | null {
  const record = detachedRecord(value);
  if (record === null || !Number.isSafeInteger(record.revision) || Number(record.revision) < 0) {
    return null;
  }
  return Object.freeze({ ...record, revision: Number(record.revision) });
}

function eventData(event: unknown): unknown {
  return isRecord(event) && Object.hasOwn(event, 'data') ? event.data : event;
}

function cloneRequest(request: DesktopRecord): DesktopRecord {
  return detachedRecord(request) ?? Object.freeze({});
}

const requiredStatusFields: readonly StatusField[] = [
  'serverInfo',
  'clientCount',
  'hackState',
  'coordinationState',
];
const statusSubscriptions = new Map<StatusField, Set<StatusSubscription>>();
const releases = new Set<DesktopUnsubscribe>();
let runtimeSnapshotStarted = false;

function beginRuntimeSnapshot(): void {
  if (runtimeSnapshotStarted || !requiredStatusFields.every(field => statusSubscriptions.has(field))) return;
  runtimeSnapshotStarted = true;
  void invoke(APP_METHODS.getRuntimeStatus).then(value => {
    if (!isRecord(value)) return;
    for (const field of requiredStatusFields) {
      for (const subscription of statusSubscriptions.get(field) ?? []) {
        subscription.deliver(value[field]);
      }
    }
  });
}

function subscribeStatus<T>(
  eventName: keyof desktopService.WailsDesktopEventMap,
  field: StatusField,
  project: (value: unknown) => T | null,
  listener: DesktopEventListener<T>,
): DesktopUnsubscribe {
  if (typeof listener !== 'function') throw new TypeError(`${eventName} listener must be a function`);
  let active = true;
  let latestRevision = -1;
  const bucket = statusSubscriptions.get(field) ?? new Set<StatusSubscription>();
  const subscription: StatusSubscription = {
    deliver(value: unknown) {
      if (!active) return;
      const projected = project(value);
      if (projected === null) return;
      if (field === 'coordinationState') {
        const record = isRecord(projected) ? projected : null;
        const revision = record?.revision;
        if (!Number.isSafeInteger(revision) || Number(revision) <= latestRevision) return;
        latestRevision = Number(revision);
      }
      listener(projected);
    },
  };
  bucket.add(subscription);
  statusSubscriptions.set(field, bucket);
  const releaseRuntime = Events.On(eventName, event => subscription.deliver(eventData(event)));
  const unsubscribe = (): void => {
    if (!active) return;
    active = false;
    bucket.delete(subscription);
    releases.delete(unsubscribe);
    releaseRuntime();
  };
  releases.add(unsubscribe);
  beginRuntimeSnapshot();
  return unsubscribe;
}

function subscribeSnapshot<T>(
  eventName: keyof desktopService.WailsDesktopEventMap,
  project: (value: unknown) => T | null,
  getter: () => Promise<unknown>,
  listener: DesktopEventListener<T>,
  version: (value: T) => readonly number[],
): DesktopUnsubscribe {
  if (typeof listener !== 'function') throw new TypeError(`${eventName} listener must be a function`);
  let active = true;
  let latest: readonly number[] = [];
  const deliver = (value: unknown): void => {
    if (!active) return;
    const projected = project(value);
    if (projected === null) return;
    const candidate = version(projected);
    if (latest.length > 0 && candidate.every((part, index) => part <= (latest[index] ?? -1))) return;
    latest = candidate;
    listener(projected);
  };
  const releaseRuntime = Events.On(eventName, event => deliver(eventData(event)));
  void invoke(getter).then(deliver);
  const unsubscribe = (): void => {
    if (!active) return;
    active = false;
    releases.delete(unsubscribe);
    releaseRuntime();
  };
  releases.add(unsubscribe);
  return unsubscribe;
}

async function publicSnapshot(): Promise<DesktopPublicAccessSnapshot> {
  const snapshot = publicAccessSnapshot(await invoke(APP_METHODS.getPublicAccess));
  return snapshot ?? Object.freeze({ generation: 0, settingsRevision: 0 });
}

function clearSecrets(request: DesktopRecord): void {
  for (const field of ['replacementProviderToken', 'replacementPlayerPassword']) {
    if (Object.hasOwn(request, field)) Reflect.set(request, field, '');
  }
}

const typedDesktopPort: DesktopPort = {
  onServerInfo: listener => subscribeStatus('server-info', 'serverInfo', serverInfo, listener),
  onClientCount: listener => subscribeStatus('client-count', 'clientCount', eventCount, listener),
  onHackState: listener => subscribeStatus('hack-state', 'hackState', eventRecord, listener),
  onCoordinationState: listener => subscribeStatus('coordination-state', 'coordinationState', eventRecord, listener),
  onSessionState: listener => subscribeStatus('session-state', 'sessionState', eventRecord, listener),
  onPublicAccessStatus: listener => subscribeSnapshot(
    'public-access-status',
    publicAccessSnapshot,
    APP_METHODS.getPublicAccess,
    listener,
    value => [value.generation, value.settingsRevision],
  ),
  onApplicationUpdateStatus: listener => subscribeSnapshot(
    'application-update-status',
    applicationUpdateSnapshot,
    APP_METHODS.getApplicationUpdateStatus,
    listener,
    value => [value.revision],
  ),

  getRuntimeStatus: async () => runtimeStatus(await invoke(APP_METHODS.getRuntimeStatus)),
  openUrl: url => command(APP_METHODS.openUrl, url),
  writeClipboardText: async value => {
    if (typeof value !== 'string' || value.length === 0) return false;
    try {
      await Clipboard.SetText(value);
      return true;
    } catch {
      return false;
    }
  },
  openSession: () => documentCommand(APP_METHODS.openSession),
  newSession: () => documentCommand(APP_METHODS.newSession),
  saveSession: session => command(APP_METHODS.saveSession, cloneRequest(session)),
  loadReferencedPlayerConfig: () => documentCommand(APP_METHODS.loadReferencedPlayerConfig),
  newPlayerConfig: () => documentCommand(APP_METHODS.newPlayerConfig),
  openPlayerConfig: () => documentCommand(APP_METHODS.openPlayerConfig),
  requestTerminalActivation: request => command(APP_METHODS.requestTerminalActivation, cloneRequest(request)),
  updateLiveTerminal: request => command(APP_METHODS.updateLiveTerminal, cloneRequest(request)),
  requestTerminalClear: () => command(APP_METHODS.requestTerminalClear),
  resolveTerminalSwitch: request => command(APP_METHODS.resolveTerminalSwitch, cloneRequest(request)),
  resolveCommandExecution: request => command(APP_METHODS.resolveCommandExecution, cloneRequest(request)),
  resolveTerminalNavigation: request => command(APP_METHODS.resolveTerminalNavigation, cloneRequest(request)),
  forceHackSuccess: () => command(APP_METHODS.forceHackSuccess),
  resetFailedHack: request => command(APP_METHODS.resetFailedHack, cloneRequest(request)),
  resetCommandState: request => command(APP_METHODS.resetCommandState, cloneRequest(request)),
  resetTerminalCommandStates: request => command(APP_METHODS.resetTerminalCommandStates, cloneRequest(request)),
  replaceTerminalGroups: request => command(APP_METHODS.replaceTerminalGroups, cloneRequest(request)),
  addCharacter: request => command(APP_METHODS.addCharacter, cloneRequest(request)),
  updateCharacter: request => command(APP_METHODS.updateCharacter, cloneRequest(request)),
  deleteCharacter: request => command(APP_METHODS.deleteCharacter, cloneRequest(request)),
  renameLogicalSession: request => command(APP_METHODS.renameLogicalSession, cloneRequest(request)),
  assignCharacter: request => command(APP_METHODS.assignCharacter, cloneRequest(request)),
  releaseCharacter: sessionId => command(APP_METHODS.releaseCharacter, sessionId),
  moveCharacter: request => command(APP_METHODS.moveCharacter, cloneRequest(request)),
  setActiveController: sessionId => command(APP_METHODS.setActiveController, sessionId),
  startBroadcast: () => command(APP_METHODS.startBroadcast),
  endBroadcast: () => command(APP_METHODS.endBroadcast),
  getPublicAccess: publicSnapshot,
  copyPublicAccessCredentials: () => command(APP_METHODS.copyPublicAccessCredentials),
  savePublicAccessSettings: request => {
    const copy = cloneRequest(request);
    const result = command(APP_METHODS.savePublicAccessSettings, copy);
    clearSecrets(copy);
    clearSecrets(request);
    return result;
  },
  generatePlayerPassword: request => command(APP_METHODS.generatePlayerPassword, cloneRequest(request)),
  startPublicAccess: request => command(APP_METHODS.startPublicAccess, cloneRequest(request)),
  stopPublicAccess: request => command(APP_METHODS.stopPublicAccess, cloneRequest(request)),
  resolveApplicationUpdateOffer: request => command(APP_METHODS.resolveApplicationUpdateOffer, cloneRequest(request)),
  resolveApplicationUpdateRestart: request => command(APP_METHODS.resolveApplicationUpdateRestart, cloneRequest(request)),
};

export const desktopPort: DesktopPort = Object.freeze(typedDesktopPort);

export function disposeDesktopPort(): void {
  for (const release of [...releases]) release();
  statusSubscriptions.clear();
  runtimeSnapshotStarted = false;
}
