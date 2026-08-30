function stateChangingAuthoringSession() {
  return {
    version: 1,
    name: 'State-changing authoring fixture',
    terminals: [{
      id: 'terminal-stateful',
      name: 'Терминал охраны',
      hackLevel: 0,
      introText: '',
      root: {
        id: 'root',
        type: 'folder',
        name: 'ROOT',
        children: [
          {
            id: 'emergency-lights',
            type: 'command',
            name: 'Включить аварийный свет',
            text: 'Аварийное освещение включено.',
          },
          {
            id: 'doors',
            type: 'command',
            name: 'Открыть двери',
            text: 'Новая редакция результата открытия.',
            stateChange: {
              completedName: 'Двери разблокированы',
              confirmationText: 'Открыть двери?',
            },
          },
          {
            id: 'alarm',
            type: 'command',
            name: 'Включить тревогу',
            text: 'Сигнал тревоги активирован.',
            stateChange: {
              completedName: 'Сигнал тревоги активен',
              confirmationText: 'Включить тревогу?',
            },
          },
        ],
      },
      commandStates: {
        doors: {
          completedName: 'Двери открыты',
          resultText: 'Доступ в сектор разрешён.',
        },
        alarm: {
          completedName: 'Тревога включена',
          resultText: 'Охрана сектора предупреждена.',
        },
      },
    }],
  };
}

function disabledApplicationUpdate() {
  return {
    revision: 0,
    attemptId: '',
    state: 'disabled',
    installedVersion: 'development',
    availableVersion: '',
    releaseNotes: '',
    bytesDownloaded: 0,
    downloadSize: null,
    failedStage: '',
    errorMessage: '',
    recoveryAction: '',
  };
}

const state = globalThis.__desktopFixtureState ??= {
  calls: [],
  listeners: new Map(),
  releases: new Map(),
  status: {
    serverInfo: { url: 'http://127.0.0.1:3690', localUrl: '', tunnel: false, port: 3690 },
    clientCount: 1,
    hackState: null,
    coordinationState: {
      revision: 1,
      roster: [],
      sessions: [],
      broadcast: null,
      playerConfig: { name: 'Fixture players', filePath: 'fixture-players.json' },
    },
  },
  statusPromise: null,
  resolveStatus: null,
  publicAccess: {
    preferences: { version: 1, enabledPreference: false, reservedDomain: '', username: 'players', revision: 0 },
    providerTokenPresence: 'absent',
    playerPasswordPresence: 'absent',
    status: { state: 'disabled', generation: 0, settingsRevision: 0 },
  },
  publicAccessPromise: null,
  resolvePublicAccess: null,
  applicationUpdate: disabledApplicationUpdate(),
  applicationUpdatePromise: null,
  resolveApplicationUpdate: null,
  applicationUpdateDownloads: 0,
  applicationUpdatePreparationFailure: null,
  applicationUpdateRestartHandoffs: 0,
  savePublicAccessPromise: null,
  resolveSavePublicAccess: null,
  pendingSavePublicAccess: null,
  publicAccessCredentialShareError: '',
  clipboardText: '',
  authoringSession: stateChangingAuthoringSession(),
  authoringRevision: 1,
  terminalActionNextResults: new Map(),
  terminalActionDeferred: new Map(),
};
let playerPassword = 'synthetic-player-share-value';
let pendingPlayerPassword = null;
if (!state.authoringSession) state.authoringSession = stateChangingAuthoringSession();
if (!Number.isSafeInteger(state.authoringRevision)) state.authoringRevision = 1;
if (!(state.terminalActionNextResults instanceof Map)) state.terminalActionNextResults = new Map();
if (!(state.terminalActionDeferred instanceof Map)) state.terminalActionDeferred = new Map();
if (!state.applicationUpdate || typeof state.applicationUpdate !== 'object') {
  state.applicationUpdate = disabledApplicationUpdate();
}
if (!Number.isSafeInteger(state.applicationUpdateDownloads)) state.applicationUpdateDownloads = 0;
if (!Number.isSafeInteger(state.applicationUpdateRestartHandoffs)) state.applicationUpdateRestartHandoffs = 0;
try {
  const durableAuthoring = JSON.parse(globalThis.localStorage?.getItem('fallout-fixture-authoring-session') ?? 'null');
  if (durableAuthoring?.session && Number.isSafeInteger(durableAuthoring.revision)) {
    state.authoringSession = durableAuthoring.session;
    state.authoringRevision = durableAuthoring.revision;
  }
} catch {
  // A fresh fixture remains available when browser storage is unavailable.
}

function persistAuthoringState() {
  try {
    globalThis.localStorage?.setItem('fallout-fixture-authoring-session', JSON.stringify({
      session: state.authoringSession,
      revision: state.authoringRevision,
    }));
  } catch {
    // The in-memory authoring fixture remains authoritative for this page.
  }
}

const durablePublicAccess = (() => {
  try {
    if (globalThis.name?.startsWith('fallout-fixture-public-access:')) {
      return JSON.parse(globalThis.name.slice('fallout-fixture-public-access:'.length));
    }
    const raw = globalThis.localStorage?.getItem('fallout-fixture-public-access');
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
})();
if (durablePublicAccess?.preferences) state.publicAccess = durablePublicAccess;

function persistPublicAccess({ preserveVisiblePreferences = false } = {}) {
  try {
    const durable = structuredClone(state.publicAccess);
    if (preserveVisiblePreferences) {
      const priorRaw = globalThis.localStorage?.getItem('fallout-fixture-public-access');
      const prior = priorRaw ? JSON.parse(priorRaw) : null;
      durable.preferences.reservedDomain = prior?.preferences?.reservedDomain ?? '';
      durable.preferences.username = prior?.preferences?.username ?? 'players';
    }
    const serialized = JSON.stringify(durable);
    globalThis.name = `fallout-fixture-public-access:${serialized}`;
    globalThis.localStorage?.setItem('fallout-fixture-public-access', serialized);
  } catch {
    // The fixture remains usable when browser storage is unavailable.
  }
}

function record(method, args) {
  state.calls.push({ method, args });
  return Promise.resolve({ ok: true, method, args });
}

function terminalAction(method, args) {
  const retained = structuredClone(args ?? []);
  state.calls.push({ method, args: retained });
  const deferred = state.terminalActionDeferred.get(method);
  if (deferred) return deferred.promise;
  if (state.terminalActionNextResults.has(method)) {
    const result = structuredClone(state.terminalActionNextResults.get(method));
    state.terminalActionNextResults.delete(method);
    return Promise.resolve(result);
  }
  return Promise.resolve({ ok: true, method, args: retained });
}

function authoringFixtureActive() {
  return globalThis.location?.pathname === '/__fixture/state-changing-command-authoring';
}

function terminalGroupingFixtureActive() {
  return globalThis.location?.pathname === '/__fixture/terminal-grouping/overseer';
}

const authoringFixtureBase = '/__fixture/state-changing-command-authoring';

async function authoringFixtureCommand(path, payload) {
  const response = await fetch(`${authoringFixtureBase}/${path}`, {
    method: payload === undefined ? 'GET' : 'POST',
    headers: payload === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  if (!response.ok) throw new Error(`authoring fixture ${path} failed`);
  const result = await response.json();
  if (result?.session) {
    state.authoringSession = structuredClone(result.session);
    state.authoringRevision = Number(result.revision || state.authoringRevision);
  }
  return result;
}

function approvalFixtureActive() {
  return globalThis.location?.pathname === '/__fixture/state-changing-command-approval/overseer';
}

function terminalNavigationFixtureActive() {
  return globalThis.location?.pathname === '/__fixture/terminal-navigation/overseer';
}

function syncFixtureActive() {
  return globalThis.location?.pathname === '/__fixture/state-changing-command-sync/overseer';
}

function playerManagementFixtureActive() {
  return globalThis.location?.pathname === '/__fixture/player-management';
}

async function playerManagementFixtureCommand(path, payload) {
  const response = await fetch(`/__fixture/player-management/${path}`, {
    method: payload === undefined ? 'GET' : 'POST',
    headers: payload === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  if (!response.ok) throw new Error(`player-management fixture ${path} failed`);
  return response.json();
}

function stateChangingLifecycleBase() {
	if (terminalNavigationFixtureActive()) return '/__fixture/terminal-navigation';
  if (approvalFixtureActive()) return '/__fixture/state-changing-command-approval';
  if (syncFixtureActive()) return '/__fixture/state-changing-command-sync';
  return '';
}

async function stateChangingCoordinationState() {
  const response = await fetch(`${stateChangingLifecycleBase()}/state`);
  if (!response.ok) throw new Error('state-changing coordination fixture is unavailable');
  return response.json();
}

function emitFixtureEvent(name, data) {
  for (const callback of state.listeners.get(name) ?? []) callback({ data: structuredClone(data) });
}

function retainApplicationUpdate(snapshot) {
  if (!snapshot || typeof snapshot !== 'object') return;
  const revision = Number(snapshot.revision);
  const currentRevision = Number(state.applicationUpdate?.revision);
  if (!Number.isSafeInteger(revision) || revision < 0) return;
  if (Number.isSafeInteger(currentRevision) && revision < currentRevision) return;
  state.applicationUpdate = structuredClone(snapshot);
}

function applicationUpdateSnapshot() {
  return structuredClone(state.applicationUpdate);
}

function applicationUpdateResult(ok, error = '') {
  return { ok, error, snapshot: applicationUpdateSnapshot() };
}

function authoringSessionResult() {
  return {
    ok: true,
    revision: state.authoringRevision,
    session: structuredClone(state.authoringSession),
  };
}

async function authoringDocumentResult(filePath) {
  const result = await authoringFixtureCommand('session');
  return {
    canceled: false,
    ok: true,
    filePath,
    session: structuredClone(result.session),
  };
}

async function playerConfigDocumentResult(method, filePath) {
  state.calls.push({ method, args: [] });
  const playerConfig = {
    name: method === 'NewPlayerConfig' ? 'Новые игроки' : 'Игроки теста',
    filePath,
  };
  const currentCoordination = playerManagementFixtureActive()
    ? await playerManagementFixtureCommand('state')
    : structuredClone(state.status.coordinationState);
  const nextRevision = Math.max(
    Number(currentCoordination?.revision ?? 0),
    Number(state.playerConfigRevision ?? 0),
  ) + 1;
  state.playerConfigRevision = nextRevision;
  const session = {
    ...structuredClone(state.authoringSession),
    playerConfig: filePath.split('/').at(-1),
  };
  state.authoringSession = session;
  persistAuthoringState();
  return {
    canceled: false,
    config: playerConfig,
    ok: true,
    session: structuredClone(session),
    state: {
      ...currentCoordination,
      revision: nextRevision,
      playerConfig,
    },
  };
}

globalThis.__desktopFixture = {
  calls: state.calls,
  timeline: state.calls,
  authoringSession() {
    return structuredClone(state.authoringSession);
  },
  playerConfigRevision() {
    return Number(state.playerConfigRevision ?? 0);
  },
  async authoringDurableState() {
    if (authoringFixtureActive()) await authoringFixtureCommand('session');
    return {
      revision: state.authoringRevision,
      commandStates: structuredClone(state.authoringSession.terminals[0].commandStates ?? {}),
    };
  },
  emit(name, data) {
	if (name === 'public-access-status' && data?.preferences && data?.status) {
	  state.publicAccess = structuredClone(data);
	}
	if (name === 'application-update-status') retainApplicationUpdate(data);
    for (const callback of state.listeners.get(name) ?? []) callback({ data });
  },
  setNextTerminalActionResult(method, result) {
    state.terminalActionNextResults.set(method, structuredClone(result));
  },
  deferTerminalAction(method) {
    if (state.terminalActionDeferred.has(method)) return;
    let resolve;
    const promise = new Promise(done => { resolve = done; });
    state.terminalActionDeferred.set(method, { promise, resolve });
  },
  resolveTerminalAction(method, result = { ok: true }) {
    const deferred = state.terminalActionDeferred.get(method);
    if (!deferred) return;
    state.terminalActionDeferred.delete(method);
    deferred.resolve(structuredClone(result));
  },
  deferStatus() {
    state.statusPromise = new Promise(resolve => { state.resolveStatus = resolve; });
  },
  resolveStatus(status = state.status) {
    state.resolveStatus?.(status);
    state.resolveStatus = null;
  },
  setStatus(status) { state.status = status; },
  deferPublicAccess() {
    state.publicAccessPromise = new Promise(resolve => { state.resolvePublicAccess = resolve; });
  },
  resolvePublicAccess(snapshot = state.publicAccess) {
    state.resolvePublicAccess?.(snapshot);
    state.resolvePublicAccess = null;
    state.publicAccessPromise = null;
  },
  deferApplicationUpdate() {
    if (state.applicationUpdatePromise) return;
    state.applicationUpdatePromise = new Promise(resolve => { state.resolveApplicationUpdate = resolve; });
  },
  resolveApplicationUpdate(snapshot = state.applicationUpdate) {
    const resolved = structuredClone(snapshot);
    retainApplicationUpdate(resolved);
    state.resolveApplicationUpdate?.(resolved);
    state.resolveApplicationUpdate = null;
    state.applicationUpdatePromise = null;
  },
  emitApplicationUpdateProgress(snapshot) {
    retainApplicationUpdate(snapshot);
    emitFixtureEvent('application-update-status', applicationUpdateSnapshot());
  },
  failNextApplicationUpdatePreparation(failure = {}) {
    state.applicationUpdatePreparationFailure = {
      failedStage: typeof failure.failedStage === 'string' ? failure.failedStage : 'download',
      errorMessage: typeof failure.errorMessage === 'string'
        ? failure.errorMessage
        : 'Не удалось подготовить обновление.',
      recoveryAction: typeof failure.recoveryAction === 'string'
        ? failure.recoveryAction
        : 'Продолжайте работу и повторите попытку позже.',
    };
  },
  applicationUpdateSnapshot,
  applicationUpdateDownloadCount() { return state.applicationUpdateDownloads; },
  applicationUpdateRestartHandoffCount() { return state.applicationUpdateRestartHandoffs; },
  releaseCount(name) { return state.releases.get(name) ?? 0; },
  takeClipboardText() {
    const value = state.clipboardText;
    state.clipboardText = '';
    return value;
  },
  failNextPublicAccessCredentialShare(error = 'The secure credential store is unavailable.') {
    state.publicAccessCredentialShareError = error;
  },
  deferSavePublicAccess() {
    state.savePublicAccessPromise = new Promise(resolve => { state.resolveSavePublicAccess = resolve; });
  },
  resolveSavePublicAccess(result = state.pendingSavePublicAccess) {
    if (result?.snapshot) {
      state.publicAccess = structuredClone(result.snapshot);
      if (result.ok && pendingPlayerPassword !== null) playerPassword = pendingPlayerPassword;
      persistPublicAccess();
    }
    state.resolveSavePublicAccess?.(result);
    state.resolveSavePublicAccess = null;
    state.savePublicAccessPromise = null;
    state.pendingSavePublicAccess = null;
    pendingPlayerPassword = null;
  },
};

export const Events = {
  On(name, callback) {
    state.calls.push({ method: `event:on:${name}`, args: [] });
    const listeners = state.listeners.get(name) ?? new Set();
    listeners.add(callback);
    state.listeners.set(name, listeners);
    let coordinationPoll = null;
    if ((stateChangingLifecycleBase() || terminalGroupingFixtureActive()) && name === 'coordination-state') {
	  let lastProjection = '';
	  const poll = async () => {
		try {
		  const coordination = terminalGroupingFixtureActive()
			? (await fetch('/__fixture/terminal-grouping/status').then(response => response.json())).coordinationState
			: await stateChangingCoordinationState();
          const projection = JSON.stringify(coordination);
          if (projection === lastProjection) return;
          lastProjection = projection;
          callback({ data: coordination });
        } catch {
          // The next poll retries while the deterministic fixture is running.
        }
      };
      void poll();
      coordinationPoll = setInterval(() => { void poll(); }, 25);
    }
    let active = true;
    return () => {
      if (!active) return;
      active = false;
      if (coordinationPoll !== null) clearInterval(coordinationPoll);
      listeners.delete(callback);
      state.releases.set(name, (state.releases.get(name) ?? 0) + 1);
    };
  },
};

export const Clipboard = {
  SetText(value) {
    state.clipboardText = typeof value === 'string' ? value : '';
    return Promise.resolve();
  },
};

export function GetRuntimeStatus() {
  state.calls.push({ method: 'GetRuntimeStatus', args: [] });
  if (terminalGroupingFixtureActive()) {
    return fetch('/__fixture/terminal-grouping/status').then(async response => ({
      ...state.status,
      ...(response.ok ? await response.json() : {}),
    }));
  }
  if (playerManagementFixtureActive()) {
    return playerManagementFixtureCommand('state').then(coordinationState => ({
      ...state.status,
      coordinationState,
    }));
  }
  if (stateChangingLifecycleBase()) {
    return stateChangingCoordinationState().then(coordinationState => ({
      ...state.status,
      coordinationState,
    }));
  }
  return state.statusPromise ?? Promise.resolve(state.status);
}

export function GetApplicationUpdateStatus() {
  state.calls.push({ method: 'GetApplicationUpdateStatus', args: [] });
  return state.applicationUpdatePromise ?? Promise.resolve(applicationUpdateSnapshot());
}

export function ResolveApplicationUpdateOffer(payload) {
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'ResolveApplicationUpdateOffer', args: [retained] });
  if (retained.attemptId !== state.applicationUpdate.attemptId
    || state.applicationUpdate.state !== 'available') {
    return Promise.resolve(applicationUpdateResult(false, 'Application update offer is no longer available'));
  }
  if (retained.decision !== 'accept' && retained.decision !== 'defer') {
    return Promise.resolve(applicationUpdateResult(false, 'Application update offer decision is invalid'));
  }

  if (retained.decision === 'accept') {
    // Preparation is controlled explicitly by emitApplicationUpdateProgress.
    // Keeping the offered revision here mirrors an in-flight command and lets
    // tests choose every externally observable progress revision.
    state.applicationUpdateDownloads += 1;
  } else {
    retainApplicationUpdate({
      ...state.applicationUpdate,
      revision: state.applicationUpdate.revision + 1,
      state: 'deferred',
    });
  }
  if (retained.decision === 'accept' && state.applicationUpdatePreparationFailure) {
    const failure = state.applicationUpdatePreparationFailure;
    state.applicationUpdatePreparationFailure = null;
    retainApplicationUpdate({
      ...state.applicationUpdate,
      revision: state.applicationUpdate.revision + 1,
      state: 'failed',
      ...failure,
    });
  }
  return Promise.resolve(applicationUpdateResult(true));
}

export function ResolveApplicationUpdateRestart(payload) {
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'ResolveApplicationUpdateRestart', args: [retained] });
  if (retained.attemptId !== state.applicationUpdate.attemptId
    || state.applicationUpdate.state !== 'ready-to-restart') {
    return Promise.resolve(applicationUpdateResult(false, 'Application update is not ready to restart'));
  }
  if (retained.decision !== 'restart' && retained.decision !== 'postpone') {
    return Promise.resolve(applicationUpdateResult(false, 'Application update restart decision is invalid'));
  }
  if (retained.decision === 'restart') {
    state.applicationUpdateRestartHandoffs += 1;
    retainApplicationUpdate({
      ...state.applicationUpdate,
      revision: state.applicationUpdate.revision + 1,
      state: 'applying',
    });
  }
  return Promise.resolve(applicationUpdateResult(true));
}

function snapshot() {
  return structuredClone(state.publicAccess);
}

export function GetPublicAccess() {
  state.calls.push({ method: 'GetPublicAccess', args: [] });
  return state.publicAccessPromise ?? Promise.resolve(snapshot());
}

export function CopyPublicAccessCredentials() {
  state.calls.push({ method: 'CopyPublicAccessCredentials', args: [] });
  if (state.publicAccessCredentialShareError) {
    const error = state.publicAccessCredentialShareError;
    state.publicAccessCredentialShareError = '';
    return Promise.resolve({ ok: false, error });
  }
  const username = state.publicAccess.preferences.username;
  if (state.publicAccess.playerPasswordPresence !== 'present' || !username || !playerPassword) {
    return Promise.resolve({ ok: false, error: 'Player credentials are unavailable.' });
  }
  state.clipboardText = `Логин: ${username}\nПароль: ${playerPassword}`;
  return Promise.resolve({ ok: true });
}

export function SavePublicAccessSettings(request) {
  const proposed = request && typeof request === 'object' ? request : {};
  const providerReplacement = proposed.replacementProviderToken;
  const passwordReplacement = proposed.replacementPlayerPassword;
  const retained = {
    expectedRevision: proposed.expectedRevision,
    enabledPreference: proposed.enabledPreference,
    reservedDomain: proposed.reservedDomain,
    username: proposed.username,
    replacementProviderToken: '',
    deleteProviderToken: proposed.deleteProviderToken === true,
    replacementPlayerPassword: '',
    deletePlayerPassword: proposed.deletePlayerPassword === true,
  };
  state.calls.push({ method: 'SavePublicAccessSettings', args: [retained] });
  const revision = state.publicAccess.preferences.revision + 1;
  const nextPublicAccess = {
    preferences: {
      version: 1,
      enabledPreference: proposed.enabledPreference === true,
      reservedDomain: typeof proposed.reservedDomain === 'string' ? proposed.reservedDomain : '',
      username: typeof proposed.username === 'string' && proposed.username ? proposed.username : 'players',
      revision,
    },
    providerTokenPresence: proposed.deleteProviderToken ? 'absent' : (providerReplacement ? 'present' : state.publicAccess.providerTokenPresence),
    playerPasswordPresence: proposed.deletePlayerPassword ? 'absent' : (passwordReplacement ? 'present' : state.publicAccess.playerPasswordPresence),
    status: { state: 'disabled', generation: state.publicAccess.status.generation + 1, settingsRevision: revision },
  };
  const result = { ok: true, snapshot: structuredClone(nextPublicAccess) };
  const nextPlayerPassword = proposed.deletePlayerPassword
    ? ''
    : (passwordReplacement || playerPassword);
  if (state.savePublicAccessPromise) {
    state.pendingSavePublicAccess = result;
    pendingPlayerPassword = nextPlayerPassword;
    return state.savePublicAccessPromise;
  }
  playerPassword = nextPlayerPassword;
  state.publicAccess = nextPublicAccess;
  persistPublicAccess();
  return Promise.resolve(result);
}

export function GeneratePlayerPassword(request) {
  state.calls.push({ method: 'GeneratePlayerPassword', args: [{ expectedRevision: request?.expectedRevision ?? 0 }] });
  const revision = state.publicAccess.preferences.revision + 1;
  state.publicAccess.preferences.revision = revision;
  state.publicAccess.playerPasswordPresence = 'present';
  playerPassword = 'synthetic-one-time-generated-value';
  state.publicAccess.status = { state: 'disabled', generation: state.publicAccess.status.generation + 1, settingsRevision: revision };
  persistPublicAccess({ preserveVisiblePreferences: true });
  return Promise.resolve({ ok: true, generatedPassword: 'synthetic-one-time-generated-value', settingsRevision: revision });
}

export function StartPublicAccess(request) {
  state.calls.push({ method: 'StartPublicAccess', args: [{ expectedRevision: request?.expectedRevision ?? 0 }] });
  state.publicAccess.status = {
    state: 'ready', generation: state.publicAccess.status.generation + 1,
    settingsRevision: state.publicAccess.preferences.revision, publicUrl: 'https://fixture.example',
  };
  return Promise.resolve({ ok: true, snapshot: snapshot() });
}

export function StopPublicAccess(request) {
  state.calls.push({ method: 'StopPublicAccess', args: [{ expectedRevision: request?.expectedRevision ?? 0 }] });
  state.publicAccess.status = {
    state: 'disabled', generation: state.publicAccess.status.generation + 1,
    settingsRevision: state.publicAccess.preferences.revision,
  };
  return Promise.resolve({ ok: true, snapshot: snapshot() });
}

export async function AddCharacter(payload) {
  if (!playerManagementFixtureActive()) return record('AddCharacter', [payload]);
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'AddCharacter', args: [retained] });
  return playerManagementFixtureCommand('add', retained);
}
export async function UpdateCharacter(payload) {
  if (!playerManagementFixtureActive()) return record('UpdateCharacter', [payload]);
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'UpdateCharacter', args: [retained] });
  return playerManagementFixtureCommand('update', retained);
}
export async function DeleteCharacter(payload) {
  if (!playerManagementFixtureActive()) return record('DeleteCharacter', [payload]);
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'DeleteCharacter', args: [retained] });
  return playerManagementFixtureCommand('delete', retained);
}
export const AssignCharacter = (...args) => record('AssignCharacter', args);
export const CopyDemo = (...args) => record('CopyDemo', args);
export const EndBroadcast = (...args) => record('EndBroadcast', args);
export function ForceHackSuccess(...args) {
  if (!terminalNavigationFixtureActive()) return record('ForceHackSuccess', args);
  state.calls.push({ method: 'ForceHackSuccess', args: structuredClone(args) });
  return fetch('/__fixture/terminal-navigation/force-hack', { method: 'POST' }).then(async response => {
    if (response.ok) {
      emitFixtureEvent('hack-state', {
        solved: true, failed: false, attemptsLeft: 3, attemptsMax: 4,
      });
    }
    return {
      ok: response.ok,
      error: response.ok ? '' : (await response.text()).trim(),
    };
  });
}
export function LoadReferencedPlayerConfig() {
  if (!authoringFixtureActive() && !playerManagementFixtureActive()) {
    return record('LoadReferencedPlayerConfig', []);
  }
  const reference = typeof state.authoringSession.playerConfig === 'string'
    ? state.authoringSession.playerConfig
    : 'fixture-players.json';
  return playerConfigDocumentResult('LoadReferencedPlayerConfig', `/private/tmp/${reference}`);
}
export const MoveCharacter = (...args) => record('MoveCharacter', args);
export function NewPlayerConfig() {
  if (!authoringFixtureActive() && !playerManagementFixtureActive()) return record('NewPlayerConfig', []);
  return playerConfigDocumentResult('NewPlayerConfig', '/private/tmp/new-players.json');
}
export async function NewSession(...args) {
  if (!authoringFixtureActive()) return record('NewSession', args);
  state.calls.push({ method: 'NewSession', args: [] });
  return authoringDocumentResult('/private/tmp/fallout-state-changing-authoring-new.json');
}
export function OpenPlayerConfig() {
  if (!authoringFixtureActive() && !playerManagementFixtureActive()) return record('OpenPlayerConfig', []);
  return playerConfigDocumentResult('OpenPlayerConfig', '/private/tmp/open-players.json');
}
export async function OpenSession(...args) {
	if (!authoringFixtureActive() && !stateChangingLifecycleBase()) return record('OpenSession', args);
  state.calls.push({ method: 'OpenSession', args: [] });
  if (syncFixtureActive()) {
    return fetch('/__fixture/state-changing-command-sync/session').then(async response => ({
      ok: response.ok,
      error: response.ok ? '' : 'synchronization session fixture is unavailable',
      filePath: '/private/tmp/fallout-state-changing-sync.json',
      session: response.ok ? await response.json() : null,
    }));
  }
  if (approvalFixtureActive()) {
    return fetch('/__fixture/state-changing-command-approval/session').then(async response => ({
      ok: response.ok,
      error: response.ok ? '' : 'approval session fixture is unavailable',
      filePath: '/private/tmp/fallout-state-changing-approval.json',
      session: response.ok ? await response.json() : null,
    }));
  }
	if (terminalNavigationFixtureActive()) {
		return fetch('/__fixture/terminal-navigation/session').then(async response => ({
			ok: response.ok,
			error: response.ok ? '' : 'terminal navigation session fixture is unavailable',
			filePath: '/private/tmp/fallout-terminal-navigation.json',
			session: response.ok ? await response.json() : null,
		}));
	}
  return authoringDocumentResult('/private/tmp/fallout-state-changing-authoring.json');
}
export const OpenURL = (...args) => record('OpenURL', args);
export const ReleaseCharacter = (...args) => record('ReleaseCharacter', args);
export const RenameLogicalSession = (...args) => record('RenameLogicalSession', args);
export function ReplaceTerminalGroups(payload) {
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'ReplaceTerminalGroups', args: [retained] });
  if (terminalGroupingFixtureActive()) {
    return fetch('/__fixture/terminal-grouping/replace-groups', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(retained),
    }).then(async response => response.ok
      ? response.json()
      : ({ ok: false, error: 'terminal grouping replacement fixture is unavailable' }));
  }
  return Promise.resolve({
    ok: true,
    error: '',
    sessionRevision: Number(retained.expectedSessionRevision ?? 0) + 1,
    session: {
      version: 1,
      name: 'Terminal groups',
      terminals: [],
      terminalGroups: structuredClone(retained.terminalGroups ?? []),
    },
    coordinationState: {
      revision: Number(retained.expectedCoordinationRevision ?? 0) + 1,
    },
  });
}
export const RequestTerminalActivation = (...args) => terminalAction('RequestTerminalActivation', args);
export const RequestTerminalClear = (...args) => terminalAction('RequestTerminalClear', args);
export const ResetFailedHack = (...args) => record('ResetFailedHack', args);
export async function ResolveCommandExecution(payload) {
  const fixtureBase = stateChangingLifecycleBase();
  if (!fixtureBase && !terminalGroupingFixtureActive()) return record('ResolveCommandExecution', [payload]);
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'ResolveCommandExecution', args: [retained] });
  const endpoint = terminalGroupingFixtureActive()
    ? '/__fixture/terminal-grouping/resolve-command'
    : `${fixtureBase}/resolve`;
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(retained),
  });
  const result = await response.json();
  return result;
}
export async function ResolveTerminalNavigation(payload) {
	const retained = structuredClone(payload ?? {});
	state.calls.push({ method: 'ResolveTerminalNavigation', args: [retained] });
	if (!terminalNavigationFixtureActive() && !terminalGroupingFixtureActive()) return { ok: true };
	const endpoint = terminalGroupingFixtureActive()
		? '/__fixture/terminal-grouping/resolve-navigation'
		: '/__fixture/terminal-navigation/resolve';
	const response = await fetch(endpoint, {
		method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(retained),
	});
	return response.json();
}
export async function ResetCommandState(payload) {
  if (!authoringFixtureActive()) return record('ResetCommandState', [payload]);
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'ResetCommandState', args: [retained] });
  const result = await authoringFixtureCommand('reset-command', retained);
  emitFixtureEvent('session-state', { revision: result.revision, session: result.session });
  return result;
}
export async function ResetTerminalCommandStates(payload) {
  if (!authoringFixtureActive()) return record('ResetTerminalCommandStates', [payload]);
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'ResetTerminalCommandStates', args: [retained] });
  const result = await authoringFixtureCommand('reset-terminal', retained);
  emitFixtureEvent('session-state', { revision: result.revision, session: result.session });
  return result;
}
export const ResolveTerminalSwitch = (...args) => record('ResolveTerminalSwitch', args);
export async function SaveSession(session) {
	if (terminalNavigationFixtureActive()) {
		const retained = structuredClone(session);
		state.calls.push({ method: 'SaveSession', args: [retained] });
		const response = await fetch('/__fixture/terminal-navigation/save', {
			method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(retained),
		});
		const result = response.ok
			? await response.json()
			: { ok: false, error: (await response.text()).trim() || 'terminal navigation save failed' };
		return { ok: result.ok === true, error: result.error || '', savedRevision: result.revision };
	}
	if (!authoringFixtureActive()) return record('SaveSession', [session]);
  const retained = structuredClone(session);
  state.calls.push({ method: 'SaveSession', args: [retained] });
  const result = await authoringFixtureCommand('save', retained);
  return { ok: result.ok === true, error: result.error || '', savedRevision: result.revision };
}
export const SetActiveController = (...args) => record('SetActiveController', args);
export const StartBroadcast = (...args) => record('StartBroadcast', args);
export const UpdateLiveTerminal = (...args) => terminalAction('UpdateLiveTerminal', args);
