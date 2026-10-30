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
  authoringSessionRevision: 1,
  // The authoring revision the fakes check SaveAuthoring against: 1 after an
  // open, +1 per accepted save that changes the document (data-model.md).
  authoringRevision: 0,
  // The canonical session of a page whose fixture server stores none.
  authoringMemory: { session: null, sessionRevision: 0 },
  authoringQueue: Promise.resolve(),
  inFlight: new Map(),
  peakInFlight: new Map(),
  heldCalls: new Map(),
  terminalActionNextResults: new Map(),
  terminalActionDeferred: new Map(),
};
let playerPassword = 'synthetic-player-share-value';
let pendingPlayerPassword = null;
if (!state.authoringSession) state.authoringSession = stateChangingAuthoringSession();
if (!Number.isSafeInteger(state.authoringSessionRevision)) state.authoringSessionRevision = 1;
if (!Number.isSafeInteger(state.authoringRevision)) state.authoringRevision = 0;
if (!state.authoringMemory || typeof state.authoringMemory !== 'object') {
  state.authoringMemory = { session: null, sessionRevision: 0 };
}
if (!(state.authoringQueue instanceof Promise)) state.authoringQueue = Promise.resolve();
if (!(state.inFlight instanceof Map)) state.inFlight = new Map();
if (!(state.peakInFlight instanceof Map)) state.peakInFlight = new Map();
if (!(state.heldCalls instanceof Map)) state.heldCalls = new Map();
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
    state.authoringSessionRevision = durableAuthoring.revision;
  }
} catch {
  // A fresh fixture remains available when browser storage is unavailable.
}

function persistAuthoringState() {
  try {
    globalThis.localStorage?.setItem('fallout-fixture-authoring-session', JSON.stringify({
      session: state.authoringSession,
      revision: state.authoringSessionRevision,
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
    state.authoringSessionRevision = Number(result.revision || state.authoringSessionRevision);
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

function runtimeLogsFailureFixtureActive() {
	return new URLSearchParams(globalThis.location?.search ?? '').has('runtime-logs-failure');
}

function runtimeLogsUnavailableFixtureActive() {
	return new URLSearchParams(globalThis.location?.search ?? '').has('runtime-logs-unavailable');
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
    revision: state.authoringSessionRevision,
    session: structuredClone(state.authoringSession),
  };
}

// track records one call for journeys and counts it in flight until it
// settles. A deferred method answers with the result given to
// resolveTerminalAction; a held method waits for releaseHeldCalls; an
// injected next result replaces run. run receives a detached copy of args.
function track(method, args, run) {
  state.calls.push({ method, args: structuredClone(args) });
  const count = (state.inFlight.get(method) ?? 0) + 1;
  state.inFlight.set(method, count);
  state.peakInFlight.set(method, Math.max(count, state.peakInFlight.get(method) ?? 0));
  return answerTracked(method, () => run(structuredClone(args))).finally(() => {
    state.inFlight.set(method, state.inFlight.get(method) - 1);
  });
}

async function answerTracked(method, run) {
  const deferred = state.terminalActionDeferred.get(method);
  if (deferred) return deferred.promise;
  await state.heldCalls.get(method)?.promise;
  if (state.terminalActionNextResults.has(method)) {
    const result = structuredClone(state.terminalActionNextResults.get(method));
    state.terminalActionNextResults.delete(method);
    return result;
  }
  return run();
}

// serializeAuthoring runs authoring operations one at a time, as the app's
// coordinator mutex does.
function serializeAuthoring(operation) {
  const result = state.authoringQueue.then(operation);
  state.authoringQueue = result.catch(() => {});
  return result;
}

// The authoring stores hold a page's canonical session: the fixture server's
// when it stores one, otherwise page memory. load returns { session,
// sessionRevision }; save stores a complete session once and returns the
// canonical result, or { ok: false, failure } when the store rejects it.
const authoringStores = {
  stateChangingAuthoring: {
    async load() {
      await authoringFixtureCommand('session');
      return { session: structuredClone(state.authoringSession), sessionRevision: state.authoringSessionRevision };
    },
    async save(session) {
      const result = await authoringFixtureCommand('save', session);
      if (result.ok !== true) return { ok: false, failure: 'persistence-failed' };
      return { ok: true, session: structuredClone(state.authoringSession), sessionRevision: state.authoringSessionRevision };
    },
  },
  terminalNavigation: {
    async load() {
      const response = await fetch('/__fixture/terminal-navigation/session');
      if (!response.ok) throw new Error('terminal navigation session fixture is unavailable');
      return { session: await response.json(), sessionRevision: state.authoringMemory.sessionRevision };
    },
    async save(session) {
      const response = await fetch('/__fixture/terminal-navigation/save', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(session),
      });
      if (!response.ok) return { ok: false, failure: 'invalid-configuration' };
      state.authoringMemory.sessionRevision = Number((await response.json()).revision);
      return { ok: true, ...await this.load() };
    },
  },
  memory: {
    async load() {
      return structuredClone(state.authoringMemory);
    },
    async save(session) {
      state.authoringMemory = {
        session: structuredClone(session),
        sessionRevision: state.authoringMemory.sessionRevision + 1,
      };
      return { ok: true, ...structuredClone(state.authoringMemory) };
    },
  },
};

function authoringStore() {
  if (authoringFixtureActive()) return authoringStores.stateChangingAuthoring;
  if (terminalNavigationFixtureActive()) return authoringStores.terminalNavigation;
  return authoringStores.memory;
}

// adoptOpenedSession makes an opened session the page's canonical one, as the
// app does on open: the session revision is unknown to the page and the
// authoring revision restarts at 1.
function adoptOpenedSession(session) {
  state.authoringMemory = { session: structuredClone(session), sessionRevision: 0 };
  state.authoringRevision = 1;
  return state.authoringRevision;
}

// compareIds orders identifiers by code unit, as Go orders their bytes.
function compareIds(left, right) {
  return left < right ? -1 : Number(left > right);
}

// canonicalJSON encodes value with sorted object keys, so equal sessions
// encode equally whatever order their fields were written in.
function canonicalJSON(value) {
  return JSON.stringify(value, (_, nested) => (nested && typeof nested === 'object' && !Array.isArray(nested)
    ? Object.fromEntries(Object.entries(nested).sort(([left], [right]) => compareIds(left, right)))
    : nested));
}

function facilityRevisionOf(session) {
  return Number(session?.facility?.revision ?? 0);
}

function authoringIssue(entityKind, entityId) {
  return {
    code: 'invalid-configuration',
    entityKind,
    ...(entityId === undefined ? {} : { entityId }),
  };
}

function contentNode(node, id) {
  if (!node || typeof node !== 'object') return null;
  if (node.id === id) return node;
  for (const child of node.children ?? []) {
    const found = contentNode(child, id);
    if (found) return found;
  }
  return null;
}

// mergeLiveValues returns a detached candidate whose live values are the
// canonical ones (internal/session/authoring.go): command states, terminal
// groups, device current states, fault activity, and the facility revision.
// Entities new to the candidate start at their initial values. Without a
// canonical session the candidate is taken as it is.
function mergeLiveValues(candidate, canonical) {
  const merged = structuredClone(candidate);
  if (!canonical) return { merged };

  const terminalIds = new Set((merged.terminals ?? []).map(terminal => terminal.id));
  const groups = (canonical.terminalGroups ?? [])
    .map(group => ({ ...structuredClone(group), terminalIds: (group.terminalIds ?? []).filter(id => terminalIds.has(id)) }))
    .filter(group => group.terminalIds.length !== 0);
  if (groups.length === 0) delete merged.terminalGroups;
  else merged.terminalGroups = groups;

  const commandStates = new Map((canonical.terminals ?? []).map(terminal => [terminal.id, terminal.commandStates ?? {}]));
  for (const terminal of merged.terminals ?? []) {
    const states = commandStates.get(terminal.id) ?? {};
    delete terminal.commandStates;
    for (const commandId of Object.keys(states).sort()) {
      const node = contentNode(terminal.root, commandId);
      if (!node) continue;
      if (node.type !== 'command' || !node.stateChange) return { issues: [authoringIssue('command', commandId)] };
      terminal.commandStates ??= {};
      terminal.commandStates[commandId] = structuredClone(states[commandId]);
    }
  }

  if (canonical.facility && !merged.facility) return { issues: [authoringIssue('facility')] };
  if (merged.facility) {
    const devices = new Map((canonical.facility?.devices ?? []).map(device => [device.id, device]));
    for (const device of merged.facility.devices ?? []) {
      device.currentStateId = devices.get(device.id)?.currentStateId ?? device.initialStateId;
    }
    const conditions = new Map((canonical.facility?.conditions ?? []).map(condition => [condition.id, condition]));
    for (const condition of merged.facility.conditions ?? []) {
      condition.currentActive = conditions.has(condition.id)
        ? conditions.get(condition.id).currentActive === true
        : condition.initialActive === true;
    }
    merged.facility.revision = facilityRevisionOf(canonical);
  }
  return { merged };
}

// changedEntityIds lists, sorted, the facility entities whose authored
// definition differs, ignoring the live field.
function changedEntityIds(current = [], candidate = [], liveField) {
  const authored = entities => new Map(entities.map(entity => {
    const { [liveField]: _live, ...definition } = entity;
    return [entity.id, canonicalJSON(definition)];
  }));
  const before = authored(current);
  const after = authored(candidate);
  return [...new Set([...before.keys(), ...after.keys()])]
    .filter(id => before.get(id) !== after.get(id))
    .sort();
}

function facilityOperationResult({
  ok = false,
  changed = false,
  correlationId,
  failure = '',
  issues = [],
  sessionRevision,
  previousFacilityRevision,
  resultingFacilityRevision = previousFacilityRevision,
  affectedDeviceIds = [],
  affectedConditionIds = [],
  session = null,
  resetCommands = [],
}) {
  return {
    ok,
    changed,
    correlationId,
    failure,
    issues,
    sessionRevision,
    previousFacilityRevision,
    resultingFacilityRevision,
    affectedDeviceIds,
    affectedConditionIds,
    session: session ? structuredClone(session) : null,
    authoringRevision: state.authoringRevision,
    resetCommands,
  };
}

function correlationIdOf(request) {
  return typeof request?.correlationId === 'string' ? request.correlationId : '';
}

// saveAuthoring mirrors App.SaveAuthoring: checked against the authoring
// revision only, with the candidate's live values replaced by canonical ones.
// A page with no canonical session adopts the first candidate at the
// authoring revision the request expected.
async function saveAuthoring(request) {
  const store = authoringStore();
  const canonical = await store.load();
  const correlationId = correlationIdOf(request);
  const previousFacilityRevision = facilityRevisionOf(canonical.session ?? request.session);
  const reject = (failure, issues = []) => facilityOperationResult({
    correlationId, failure, issues, sessionRevision: canonical.sessionRevision, previousFacilityRevision,
    session: canonical.session,
  });

  if (!request.session || typeof request.session !== 'object' || Array.isArray(request.session)) {
    return reject('invalid-configuration', [authoringIssue('session')]);
  }
  const expected = Number(request.expectedAuthoringRevision);
  if (canonical.session && expected !== state.authoringRevision) return reject('stale-revision');
  if (!correlationId.trim()) return reject('invalid-configuration', [authoringIssue('request')]);
  const { merged, issues } = mergeLiveValues(request.session, canonical.session);
  if (issues) return reject('invalid-configuration', issues);
  if (canonical.session && canonicalJSON(merged) === canonicalJSON(canonical.session)) {
    return facilityOperationResult({
      ok: true, correlationId, sessionRevision: canonical.sessionRevision, previousFacilityRevision,
      session: canonical.session,
    });
  }

  let affectedDeviceIds = [];
  let affectedConditionIds = [];
  if (canonicalJSON(merged.facility ?? null) !== canonicalJSON(canonical.session?.facility ?? null)) {
    const before = canonical.session?.facility;
    affectedDeviceIds = changedEntityIds(before?.devices, merged.facility?.devices, 'currentStateId');
    affectedConditionIds = changedEntityIds(before?.conditions, merged.facility?.conditions, 'currentActive');
    if (merged.facility) merged.facility.revision = previousFacilityRevision + 1;
  }
  const saved = await store.save(merged);
  if (!saved.ok) return reject(saved.failure, [{ code: saved.failure, entityKind: 'session' }]);

  const baseRevision = canonical.session || !Number.isSafeInteger(expected) ? state.authoringRevision : expected;
  state.authoringRevision = baseRevision + 1;
  emitFixtureEvent('session-state', { revision: saved.sessionRevision, session: saved.session });
  return facilityOperationResult({
    ok: true, changed: true, correlationId, sessionRevision: saved.sessionRevision, previousFacilityRevision,
    resultingFacilityRevision: facilityRevisionOf(saved.session), affectedDeviceIds, affectedConditionIds,
    session: saved.session,
  });
}

// resetScene mirrors App.ResetScene: every device and fault returns to its
// starting value and every terminal's executed commands clear in one write.
// The authoring revision never changes.
async function resetScene(request) {
  const store = authoringStore();
  const canonical = await store.load();
  const correlationId = correlationIdOf(request);
  const previousFacilityRevision = facilityRevisionOf(canonical.session);
  const fail = (failure, entityKind) => facilityOperationResult({
    correlationId, failure, issues: [{ code: failure, entityKind }], sessionRevision: canonical.sessionRevision,
    previousFacilityRevision, session: canonical.session,
  });

  if (!canonical.session) return fail('runtime-context-ended', 'session');
  if (Number(request.expectedSessionRevision) !== canonical.sessionRevision) return fail('stale-revision', 'session');
  if (Number(request.expectedFacilityRevision) !== previousFacilityRevision) return fail('stale-revision', 'facility');
  if (!correlationId.trim()) return fail('invalid-configuration', 'request');

  const candidate = structuredClone(canonical.session);
  const affectedDeviceIds = [];
  for (const device of candidate.facility?.devices ?? []) {
    if (device.currentStateId === device.initialStateId) continue;
    device.currentStateId = device.initialStateId;
    affectedDeviceIds.push(device.id);
  }
  const affectedConditionIds = [];
  for (const condition of candidate.facility?.conditions ?? []) {
    const initialActive = condition.initialActive === true;
    if ((condition.currentActive === true) === initialActive) continue;
    condition.currentActive = initialActive;
    affectedConditionIds.push(condition.id);
  }
  const resetCommands = [];
  for (const terminal of candidate.terminals ?? []) {
    for (const commandId of Object.keys(terminal.commandStates ?? {})) {
      resetCommands.push({ terminalId: terminal.id, commandId });
    }
    delete terminal.commandStates;
  }
  affectedDeviceIds.sort();
  affectedConditionIds.sort();
  resetCommands.sort((left, right) => compareIds(left.terminalId, right.terminalId)
    || compareIds(left.commandId, right.commandId));
  const facilityChanged = affectedDeviceIds.length !== 0 || affectedConditionIds.length !== 0;
  if (!facilityChanged && resetCommands.length === 0) {
    return facilityOperationResult({
      ok: true, correlationId, sessionRevision: canonical.sessionRevision, previousFacilityRevision,
      session: canonical.session,
    });
  }

  if (facilityChanged) candidate.facility.revision = previousFacilityRevision + 1;
  const saved = await store.save(candidate);
  if (!saved.ok) return fail('persistence-failed', 'persistence');
  emitFixtureEvent('session-state', { revision: saved.sessionRevision, session: saved.session });
  return facilityOperationResult({
    ok: true, changed: true, correlationId, sessionRevision: saved.sessionRevision, previousFacilityRevision,
    resultingFacilityRevision: facilityRevisionOf(saved.session), affectedDeviceIds, affectedConditionIds,
    session: saved.session, resetCommands,
  });
}

// describePresentation mirrors App.DescribePresentation's revisions and
// failures. Explanations come from internal/live, so this page-level fake
// explains no terminal; fixture servers and injected results provide them.
async function describePresentation(request) {
  const canonical = await authoringStore().load();
  const result = {
    ok: false,
    failure: '',
    sessionRevision: canonical.sessionRevision,
    facilityRevision: 0,
    authoringRevision: state.authoringRevision,
    terminals: [],
  };
  if (!canonical.session) return { ...result, failure: 'runtime-context-ended' };
  result.facilityRevision = facilityRevisionOf(canonical.session);
  const expected = Number(request.expectedAuthoringRevision ?? 0);
  if (expected !== 0 && expected !== state.authoringRevision) return { ...result, failure: 'stale-revision' };
  return { ...result, ok: true };
}

globalThis.__desktopFixture = {
  calls: state.calls,
  timeline: state.calls,
  async executeStateChangingCommand(commandId) {
    if (!syncFixtureActive()) throw new Error('state-changing synchronization fixture is unavailable');
    const response = await fetch('/__fixture/state-changing-command-sync/execute-command', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ commandId }),
    });
    if (!response.ok) throw new Error((await response.text()).trim() || 'fixture command execution failed');
  },
  async authoringDurableState() {
    if (authoringFixtureActive()) await authoringFixtureCommand('session');
    return {
      revision: state.authoringSessionRevision,
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
  // Shims record their own SaveAuthoring, ResetScene, and DescribePresentation
  // through track, so journeys read one call log and one in-flight count.
  track,
  inFlightCount(method) { return state.inFlight.get(method) ?? 0; },
  peakInFlight(method) { return state.peakInFlight.get(method) ?? 0; },
  // holdCalls keeps every later tracked call of method in flight, recorded but
  // unanswered, until releaseHeldCalls lets them run in arrival order.
  holdCalls(method) {
    if (state.heldCalls.has(method)) return;
    let release;
    const promise = new Promise(done => { release = done; });
    state.heldCalls.set(method, { promise, release });
  },
  releaseHeldCalls(method) {
    const held = state.heldCalls.get(method);
    if (!held) return;
    state.heldCalls.delete(method);
    held.release();
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
	if (runtimeLogsFailureFixtureActive()) {
		return Promise.resolve({ ...state.status, startupError: 'startup degraded', serverInfo: null });
	}
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
  if (authoringFixtureActive()) {
    // ResetScene checks the session revision this status reports.
    return authoringFixtureCommand('session').then(result => ({
      ...state.status,
      savedRevision: Number(result.revision),
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
export const LoadReferencedPlayerConfig = (...args) => record('LoadReferencedPlayerConfig', args);
export const MoveCharacter = (...args) => record('MoveCharacter', args);
export const NewPlayerConfig = (...args) => record('NewPlayerConfig', args);
export const NewSession = (...args) => record('NewSession', args);
export const OpenPlayerConfig = (...args) => record('OpenPlayerConfig', args);
async function openFixtureSession(path, filePath, unavailable) {
  const response = await fetch(path);
  if (!response.ok) return { ok: false, error: unavailable, filePath, session: null };
  const session = await response.json();
  return { ok: true, error: '', filePath, session, authoringRevision: adoptOpenedSession(session) };
}

export async function OpenSession(...args) {
	if (!authoringFixtureActive() && !stateChangingLifecycleBase()) return record('OpenSession', args);
  state.calls.push({ method: 'OpenSession', args: [] });
  if (syncFixtureActive()) {
    return openFixtureSession(
      '/__fixture/state-changing-command-sync/session',
      '/private/tmp/fallout-state-changing-sync.json',
      'synchronization session fixture is unavailable',
    );
  }
  if (approvalFixtureActive()) {
    return openFixtureSession(
      '/__fixture/state-changing-command-approval/session',
      '/private/tmp/fallout-state-changing-approval.json',
      'approval session fixture is unavailable',
    );
  }
  if (terminalNavigationFixtureActive()) {
    return openFixtureSession(
      '/__fixture/terminal-navigation/session',
      '/private/tmp/fallout-terminal-navigation.json',
      'terminal navigation session fixture is unavailable',
    );
  }
  const result = await authoringFixtureCommand('session');
  return {
    ok: true,
    filePath: '/private/tmp/fallout-state-changing-authoring.json',
    session: structuredClone(result.session),
    authoringRevision: adoptOpenedSession(result.session),
  };
}
export const OpenURL = (...args) => record('OpenURL', args);
export function OpenLogLocation() {
  state.calls.push({ method: 'OpenLogLocation', args: [] });
	if (runtimeLogsFailureFixtureActive()) {
		return Promise.resolve({
			ok: false,
			error: 'Could not open the application log directory.',
			directoryPath: '/Users/fixture/Application Support/Fallout Terminal/logs',
			activeLogPath: '',
		});
	}
	if (runtimeLogsUnavailableFixtureActive()) {
		return Promise.resolve({
			ok: true,
			error: '',
			directoryPath: '/Users/fixture/Application Support/Fallout Terminal/logs',
			activeLogPath: '',
		});
	}
	const deferred = state.terminalActionDeferred.get('OpenLogLocation');
	if (deferred) return deferred.promise;
	if (state.terminalActionNextResults.has('OpenLogLocation')) {
		const result = structuredClone(state.terminalActionNextResults.get('OpenLogLocation'));
		state.terminalActionNextResults.delete('OpenLogLocation');
		return Promise.resolve(result);
	}
  return Promise.resolve({
    ok: true,
    error: '',
    directoryPath: '/Users/fixture/Application Support/Fallout Terminal/logs',
    activeLogPath: '/Users/fixture/Application Support/Fallout Terminal/logs/application-current.log',
  });
}
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
export function InspectFacilityDependencies(payload) {
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'InspectFacilityDependencies', args: [retained] });
  if (state.terminalActionNextResults.has('InspectFacilityDependencies')) {
    const result = structuredClone(state.terminalActionNextResults.get('InspectFacilityDependencies'));
    state.terminalActionNextResults.delete('InspectFacilityDependencies');
    return Promise.resolve(result);
  }
  return Promise.resolve({
    ok: true,
    failure: '',
    issues: [],
    sessionRevision: retained.expectedSessionRevision,
    facilityRevision: retained.expectedFacilityRevision,
    report: {
      target: structuredClone(retained.target),
      dependencies: [{
        kind: 'command-action',
        sourceId: 'open-door',
        targetId: retained.target?.entityId ?? '',
        property: 'stateChange.facilityAction.transitions[0].deviceId',
        terminalId: 'terminal-security',
      }],
    },
  });
}
export function PreviewFacility(payload) {
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'PreviewFacility', args: [retained] });
  if (state.terminalActionNextResults.has('PreviewFacility')) {
    const result = structuredClone(state.terminalActionNextResults.get('PreviewFacility'));
    state.terminalActionNextResults.delete('PreviewFacility');
    return Promise.resolve(result);
  }
  return Promise.resolve({
    ok: true,
    failure: '',
    issues: [],
    facilityRevision: retained.expectedFacilityRevision,
    terminal: {
      terminalId: retained.terminalId,
      terminalName: 'Preview terminal',
      revision: 31,
      tree: { id: 'root', type: 'folder', name: 'ROOT', children: [] },
    },
  });
}
function facilityOperationFixture(method, payload, affectedDeviceIds, affectedConditionIds) {
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method, args: [retained] });
  if (state.terminalActionNextResults.has(method)) {
    const result = structuredClone(state.terminalActionNextResults.get(method));
    state.terminalActionNextResults.delete(method);
    return Promise.resolve(result);
  }
  const resultingFacilityRevision = Number(retained.expectedFacilityRevision ?? 0) + 1;
  return Promise.resolve({
    ok: true,
    changed: true,
    correlationId: retained.correlationId ?? '',
    failure: '',
    issues: [],
    sessionRevision: 19,
    previousFacilityRevision: retained.expectedFacilityRevision ?? 0,
    resultingFacilityRevision,
    affectedDeviceIds,
    affectedConditionIds,
    session: {
      version: 1,
      name: 'Facility operation',
      terminals: [],
      facility: { revision: resultingFacilityRevision, devices: [], conditions: [], recoveryPrograms: [] },
    },
  });
}
export const RecoverFacilityCondition = payload => facilityOperationFixture(
  'RecoverFacilityCondition', payload, [], [payload?.conditionId ?? ''],
);
export const ResetFacilityDevice = payload => facilityOperationFixture(
  'ResetFacilityDevice', payload, [payload?.deviceId ?? ''], [],
);
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
  if (!authoringFixtureActive()) {
    // Tracked so journeys can hold the reset in flight.
    return track('ResetCommandState', [payload], args => ({ ok: true, method: 'ResetCommandState', args }));
  }
  const retained = structuredClone(payload ?? {});
  state.calls.push({ method: 'ResetCommandState', args: [retained] });
  const result = await authoringFixtureCommand('reset-command', retained);
  emitFixtureEvent('session-state', { revision: result.revision, session: result.session });
  return result;
}
export const SaveAuthoring = payload => track(
  'SaveAuthoring', [payload ?? {}], ([request]) => serializeAuthoring(() => saveAuthoring(request)),
);
export const ResetScene = payload => track(
  'ResetScene', [payload ?? {}], ([request]) => serializeAuthoring(() => resetScene(request)),
);
export const DescribePresentation = payload => track(
  'DescribePresentation', [payload ?? {}], ([request]) => serializeAuthoring(() => describePresentation(request)),
);
export const ResolveTerminalSwitch = (...args) => record('ResolveTerminalSwitch', args);
export const SetActiveController = (...args) => record('SetActiveController', args);
export const StartBroadcast = (...args) => record('StartBroadcast', args);
