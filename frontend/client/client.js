import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import {
  ActionReason,
  PlayerPhase,
  PlayerNoticeKind,
  PlayerRole,
  PlayerService,
  RosterAvailability,
} from './gen/fallout/terminal/player/v1/player_pb.js';
import {
  CommandExecutionPhase,
  TerminalNavigationDirection,
  TerminalPresentationEffect,
} from './gen/fallout/terminal/player/v1/terminal_pb.js';
import {
  createPresentationUplinkTransport,
  LatestPresentationMailbox,
  supportsPresentationRequestStreaming,
} from './presentation-uplink.js';
import {
  playCharScroll,
  playEnter,
  playHackBad,
  playHackGood,
  playMenuFocus,
  playMultiple,
  playSingle,
  setAmbientActive,
} from './sound.js';

const MODE = { LIST: 'list', ENTRY: 'entry', HACK: 'hack' };
const ROW_WIDTH = 12; // must match server/hack.js
const PLAYER_TOKEN_KEY = 'fallout-terminal.player-token';
const PLAYER_SESSION_INIT_LOCK = 'fallout-terminal.player-session-init';
const PLAYER_SESSION_INIT_LEASE_KEY = 'fallout-terminal.player-session-init-lease';
const PLAYER_SESSION_INIT_CONTENDER_PREFIX = 'fallout-terminal.player-session-init-contender.';
const PLAYER_SESSION_INIT_LEASE_MS = 5000;
const PLAYER_SESSION_INIT_RETRY_MS = 100;
const PLAYER_SESSION_INIT_ELECTION_MS = 100;
const PRESENTATION_RESULT_TIMEOUT_MS = 1500;

function reportPresentationDiagnostic(stage, details = {}) {
  try {
    if (typeof window.__falloutTerminalPresentationObserver === 'function') {
      window.__falloutTerminalPresentationObserver({ stage, ...details });
    }
  } catch {
    // Test/diagnostic observation is optional and cannot affect control.
  }
}

// ── State ─────────────────────────────────────────────────
// Navigation and semantic selection/page/preview are authoritative mirrors.
// Only the connected active controller may request presentation changes.
let hasLive       = false;
let terminalID    = '';
let terminalBroadcastID = '';
let terminalName  = '';
let introText     = '';
let serverNum     = 1;
let tree          = null;   // root node {id:'root', type:'folder', name, children:[]}
let navStack      = ['root'];
let selIndex      = 0;
let mode          = MODE.LIST;
let viewEntryId   = null;
let commandOutput = null;
let currentCommandNodeId = null;
let commandExecution = null;
let terminalNavigation = null;
let controllerPresentation = { kind: 'none', contextKey: '', targetId: '', patternId: '', pageIndex: 0 };
let localControllerPresentation = null;
let localPresentationFrame = null;

// Typewriter reveal: only replay when the shown content actually changed.
let lastRenderedFolderKey  = null;
let lastRenderedEntryId    = null;
let lastRenderedCommandKey = null;
let lastRenderedHackKey    = null;
let lastRenderedHackRows = new Map();
let hackBoardFit = null;
const activeRevealControllers = new Set();
let consumedRevealKey = null;

let pagedView = {
  kind: null,
  key: null,
  text: '',
  container: null,
  pages: [''],
  index: 0,
};
let paginationFrame = null;
let hackFitFrame = null;

let hackLevel        = 0;
let hack              = null;  // public hack state from server, or null
let hackSolvedTimer   = null;
let hackTyped         = '';
let hackHoverKey      = null;
let hackHoverText     = '';
let hackHoverClearTimer = null;
let hackWasSolved     = false;
let lastAttemptsLeft  = null;
let terminalLiveBaselinePending = true;

// Player identity and assignment are complete server projections. Selection
// only creates a pending request; it never changes this state optimistically.
let sessionReady = false;
let playerState = null;
let pendingSelection = null;
let pendingSharedAction = null;
let pendingPresentationAction = null;
let desiredPresentationAction = null;
let presentationDrainScheduled = false;
let presentationUplinkGeneration = 0;
let activePresentationUplink = null;
let presentationUplinkRetryTimer = null;
let appliedSharedRevision = 0;
let transientPlayerNotice = '';
let transientPlayerNoticeContextKey = '';
let activeHackingContextKey = '';

// ── DOM refs ──────────────────────────────────────────────
const normalHeader = document.getElementById('normalHeader');
const introTextEl  = document.getElementById('introTextEl');
const serverLine   = document.getElementById('serverLine');
const hackHeader   = document.getElementById('hackHeader');
const attemptsLine = document.getElementById('attemptsLine');

const termIdle   = document.getElementById('termIdle');
const termBody   = document.getElementById('termBody');
const termList   = document.getElementById('termList');
const termEntry  = document.getElementById('termEntry');
const entryTitle = document.getElementById('entryTitle');
const entryBody  = document.getElementById('entryBody');

const hackBoard        = document.getElementById('hackBoard');
const hackColumns      = document.getElementById('hackColumns');
const hackLog          = document.getElementById('hackLog');
const hackInputPreview = document.getElementById('hackInputPreview');
const hackBlocked      = document.getElementById('hackBlocked');
const hackLogPanel     = hackLog.closest('.hack-log-panel');
const hackInputLine    = hackInputPreview.closest('.hack-input-line');
const screen           = document.getElementById('screen');

const termOutput  = document.getElementById('termOutput');
const termPrompt  = document.getElementById('termPrompt');
const backBtn     = document.getElementById('backBtn');
const pageNav     = document.getElementById('pageNav');
const pagePrev    = document.getElementById('pagePrev');
const pageNext    = document.getElementById('pageNext');
const pageIndicator = document.getElementById('pageIndicator');
const connOverlay = document.getElementById('connOverlay');
const connText    = document.getElementById('connText');

const playerIdentity     = document.getElementById('playerIdentity');
const playerCharacterName = document.getElementById('playerCharacterName');
const playerCharacterSeparator = document.getElementById('playerCharacterSeparator');
const playerFallbackName = document.getElementById('playerFallbackName');
const roleBadge          = document.getElementById('roleBadge');
const characterSelect    = document.getElementById('characterSelect');
const characterOptions   = document.getElementById('characterOptions');
const assignedWaiting    = document.getElementById('assignedWaiting');
const playerNotice       = document.getElementById('playerNotice');

// ════════════════════════════════════════════════════
// GENERATED CONNECT CLIENT
// ════════════════════════════════════════════════════
let reconnectTimer = null;
let streamAbortController = null;
let streamGeneration = 0;
const RECONNECT_DELAY_MS = 3000;
function createPageLifetimeID() {
  return (window.crypto && typeof window.crypto.randomUUID === 'function')
    ? window.crypto.randomUUID()
    : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

const sessionInitOwner = createPageLifetimeID();
const recognitionWaiters = new Set();
const playerClientInstanceID = createPageLifetimeID();
const presentationRequestStreamingSupported = supportsPresentationRequestStreaming(window);

function readBrowserToken() {
  try {
    return localStorage.getItem(PLAYER_TOKEN_KEY) || '';
  } catch (error) {
    console.warn('Player recognition storage is unavailable', error);
    return '';
  }
}

const connectWebTransport = createConnectTransport({
  baseUrl: window.location.origin,
  useBinaryFormat: true,
});
const playerTransport = createPresentationUplinkTransport({
  baseUrl: window.location.origin,
  fallback: connectWebTransport,
  method: PlayerService.method.presentationUplink,
});
const playerRPC = createClient(PlayerService, playerTransport);

function signalRecognitionChange() {
  const waiters = Array.from(recognitionWaiters);
  recognitionWaiters.clear();
  for (const resolve of waiters) resolve();
}

function waitForRecognitionChange() {
  return new Promise(resolve => {
    let timer = null;
    const finish = () => {
      recognitionWaiters.delete(finish);
      if (timer !== null) clearTimeout(timer);
      resolve();
    };
    recognitionWaiters.add(finish);
    timer = setTimeout(finish, PLAYER_SESSION_INIT_RETRY_MS);
  });
}

window.addEventListener('storage', event => {
  if (event.key === PLAYER_TOKEN_KEY || event.key === PLAYER_SESSION_INIT_LEASE_KEY ||
      (event.key && event.key.startsWith(PLAYER_SESSION_INIT_CONTENDER_PREFIX))) {
    signalRecognitionChange();
  }
});

function readSessionInitLease() {
  try {
    const raw = localStorage.getItem(PLAYER_SESSION_INIT_LEASE_KEY);
    if (!raw) return null;
    const lease = JSON.parse(raw);
    if (!lease || typeof lease.owner !== 'string' || !Number.isFinite(lease.expiresAt)) return null;
    return lease;
  } catch (error) {
    return null;
  }
}

function acquireSessionInitLease() {
  const now = Date.now();
  const current = readSessionInitLease();
  if (current && current.owner !== sessionInitOwner && current.expiresAt > now) return false;

  try {
    localStorage.setItem(PLAYER_SESSION_INIT_LEASE_KEY, JSON.stringify({
      owner: sessionInitOwner,
      expiresAt: now + PLAYER_SESSION_INIT_LEASE_MS,
    }));
    const confirmed = readSessionInitLease();
    return Boolean(confirmed && confirmed.owner === sessionInitOwner);
  } catch (error) {
    return true;
  }
}

function sessionInitContenderKey(owner) {
  return `${PLAYER_SESSION_INIT_CONTENDER_PREFIX}${owner}`;
}

function registerSessionInitContender() {
  try {
    localStorage.setItem(sessionInitContenderKey(sessionInitOwner), JSON.stringify({
      owner: sessionInitOwner,
      expiresAt: Date.now() + PLAYER_SESSION_INIT_LEASE_MS,
    }));
  } catch (error) {
    // Without shared storage the best available behavior is one session per tab.
  }
}

function releaseSessionInitContender() {
  try {
    localStorage.removeItem(sessionInitContenderKey(sessionInitOwner));
  } catch (error) {
    // Storage-unavailable tabs have no contender record to release.
  }
}

function electedSessionInitOwner() {
  const now = Date.now();
  const owners = [];
  try {
    for (let index = localStorage.length - 1; index >= 0; index -= 1) {
      const key = localStorage.key(index);
      if (!key || !key.startsWith(PLAYER_SESSION_INIT_CONTENDER_PREFIX)) continue;
      let contender = null;
      try { contender = JSON.parse(localStorage.getItem(key)); } catch (error) { /* stale */ }
      if (!contender || typeof contender.owner !== 'string' ||
          !Number.isFinite(contender.expiresAt) || contender.expiresAt <= now) {
        localStorage.removeItem(key);
        continue;
      }
      owners.push(contender.owner);
    }
  } catch (error) {
    return sessionInitOwner;
  }
  owners.sort();
  return owners[0] || sessionInitOwner;
}

function waitForSessionInitElection() {
  return new Promise(resolve => setTimeout(resolve, PLAYER_SESSION_INIT_ELECTION_MS));
}

function renewSessionInitLease() {
  const current = readSessionInitLease();
  if (!current || current.owner !== sessionInitOwner) return false;
  try {
    localStorage.setItem(PLAYER_SESSION_INIT_LEASE_KEY, JSON.stringify({
      owner: sessionInitOwner,
      expiresAt: Date.now() + PLAYER_SESSION_INIT_LEASE_MS,
    }));
    return true;
  } catch (error) {
    return false;
  }
}

function releaseSessionInitLease() {
  const current = readSessionInitLease();
  if (!current || current.owner !== sessionInitOwner) return;
  try {
    localStorage.removeItem(PLAYER_SESSION_INIT_LEASE_KEY);
  } catch (error) {
    // Storage-unavailable tabs have no shared lease to release.
  }
  signalRecognitionChange();
}

async function subscribeWithStorageLease() {
  while (!sessionReady) {
    const browserToken = readBrowserToken();
    if (browserToken) {
      await establishSubscription(browserToken);
      return;
    }

    const activeLease = readSessionInitLease();
    if (activeLease && activeLease.owner !== sessionInitOwner && activeLease.expiresAt > Date.now()) {
      await waitForRecognitionChange();
      continue;
    }

    registerSessionInitContender();
    await waitForSessionInitElection();

    const tokenAfterElection = readBrowserToken();
    if (tokenAfterElection) {
      releaseSessionInitContender();
      await establishSubscription(tokenAfterElection);
      return;
    }

    const leaseAfterElection = readSessionInitLease();
    if ((leaseAfterElection && leaseAfterElection.owner !== sessionInitOwner &&
         leaseAfterElection.expiresAt > Date.now()) || electedSessionInitOwner() !== sessionInitOwner ||
        !acquireSessionInitLease()) {
      releaseSessionInitContender();
      await waitForRecognitionChange();
      continue;
    }
    releaseSessionInitContender();

    try {
      await establishSubscription('');
    } finally {
      releaseSessionInitContender();
      releaseSessionInitLease();
    }
    return;
  }
}

async function beginSubscription() {
  const browserToken = readBrowserToken();
  if (browserToken) {
    await establishSubscription(browserToken);
    return;
  }

  if (navigator.locks && typeof navigator.locks.request === 'function') {
    try {
      await navigator.locks.request(PLAYER_SESSION_INIT_LOCK, async () => {
        const tokenAfterLock = readBrowserToken();
        await establishSubscription(tokenAfterLock);
      });
      return;
    } catch (error) {
      console.warn('Player session lock unavailable; using storage coordination', error);
    }
  }

  await subscribeWithStorageLease();
}

async function connectPlayer() {
  clearTimeout(reconnectTimer);
  reconnectTimer = null;

  sessionReady = false;
  invalidatePresentationUplink();
  clearControllerPresentationDispatch();
  terminalLiveBaselinePending = true;
  connOverlay.classList.remove('hidden');
  connText.textContent = 'УСТАНОВКА СВЯЗИ...';
  try {
    await beginSubscription();
  } catch (error) {
    console.warn('Player Connect subscription failed', error);
    scheduleReconnect();
  }
}

async function establishSubscription(recognitionHandle) {
  if (streamAbortController) streamAbortController.abort();
  const controller = new AbortController();
  streamAbortController = controller;
  const generation = ++streamGeneration;
  const request = { clientInstanceId: playerClientInstanceID };
  if (recognitionHandle) request.recognitionHandle = recognitionHandle;
  const iterator = playerRPC.subscribe(request, { signal: controller.signal })[Symbol.asyncIterator]();
  const first = await iterator.next();
  if (generation !== streamGeneration || controller.signal.aborted || first.done) {
    throw new Error('subscription ended before the complete snapshot');
  }
  applySubscriptionMessage(first.value, true);
  void drainSubscription(iterator, controller, generation);
  startPresentationUplink(readBrowserToken());
}

async function drainSubscription(iterator, controller, generation) {
  try {
    for (;;) {
      const next = await iterator.next();
      if (next.done) break;
      if (generation !== streamGeneration || controller.signal.aborted) return;
      applySubscriptionMessage(next.value, false);
    }
  } catch (error) {
    if (!controller.signal.aborted) console.warn('Player Connect stream ended', error);
  }
  if (generation === streamGeneration && !controller.signal.aborted) scheduleReconnect();
}

function scheduleReconnect() {
  sessionReady = false;
  invalidatePresentationUplink();
  clearControllerPresentationDispatch();
  terminalLiveBaselinePending = true;
  signalRecognitionChange();
  connOverlay.classList.remove('hidden');
  connText.textContent = 'СВЯЗЬ ПОТЕРЯНА — ПЕРЕПОДКЛЮЧЕНИЕ...';
  clearTimeout(reconnectTimer);
  reconnectTimer = setTimeout(() => { void connectPlayer(); }, RECONNECT_DELAY_MS);
}

function applySubscriptionMessage(message, first) {
  if (first && message.payload.case !== 'snapshot') {
    throw new Error('first subscription value is not a complete snapshot');
  }
  if (message.payload.case === 'snapshot') {
    applyGeneratedSnapshot(message.payload.value);
  } else if (message.payload.case === 'update') {
    applyGeneratedUpdate(message.payload.value);
  } else if (message.payload.case === 'presentationUplinkResult') {
    applyPresentationUplinkResult(message.payload.value);
  }
}

function startPresentationUplink(recognitionHandle) {
  invalidatePresentationUplink();
  if (!recognitionHandle || !presentationRequestStreamingSupported) return;
  const generation = ++presentationUplinkGeneration;
  const abortController = new AbortController();
  const mailbox = new LatestPresentationMailbox();
  const uplink = {
    generation,
    abortController,
    mailbox,
    ready: false,
    probeTimer: setTimeout(() => failPresentationUplink(generation), 2000),
  };
  activePresentationUplink = uplink;
  const open = {
    payload: {
      case: 'open',
      value: {
        recognitionHandle,
        clientInstanceId: playerClientInstanceID,
        uplinkGeneration: BigInt(generation),
      },
    },
  };
  const frames = (async function* () {
    yield open;
    yield* mailbox;
  })();
  void playerRPC.presentationUplink(frames, { signal: abortController.signal }).then(
    () => failPresentationUplink(generation),
    () => failPresentationUplink(generation),
  );
}

function applyPresentationUplinkResult(result) {
  const uplink = activePresentationUplink;
  if (!uplink || result.clientInstanceId !== playerClientInstanceID ||
      Number(result.uplinkGeneration) !== uplink.generation) {
    return;
  }
  if (result.payload.case === 'ready') {
    clearTimeout(uplink.probeTimer);
    uplink.probeTimer = null;
    uplink.ready = true;
    return;
  }
  if (result.payload.case === 'action') {
    const action = result.payload.value;
    applyActionResult({
      requestId: action.requestId,
      accepted: action.accepted,
      reason: actionReasonName(action.reason),
      revision: Number(action.revision),
    });
  }
}

function failPresentationUplink(generation) {
  const uplink = activePresentationUplink;
  if (!uplink || uplink.generation !== generation) {
    reportPresentationDiagnostic('uplink-failure-ignored', { generation: Number(generation || 0) });
    return;
  }
  const pending = pendingPresentationAction?.transport === 'stream'
    ? pendingPresentationAction.presentation
    : null;
  reportPresentationDiagnostic('uplink-failed', {
    generation,
    pending: pending !== null,
    desired: desiredPresentationAction !== null,
  });
  invalidatePresentationUplink();
  if (pending) {
    clearPendingPresentationAction();
    desiredPresentationAction = desiredPresentationAction || pending;
    scheduleDesiredPresentationDispatch();
  }
  if (sessionReady && presentationRequestStreamingSupported) {
    presentationUplinkRetryTimer = setTimeout(() => {
      presentationUplinkRetryTimer = null;
      if (sessionReady && !activePresentationUplink) {
        startPresentationUplink(readBrowserToken());
      }
    }, RECONNECT_DELAY_MS);
  }
}

function invalidatePresentationUplink() {
  clearTimeout(presentationUplinkRetryTimer);
  presentationUplinkRetryTimer = null;
  const uplink = activePresentationUplink;
  activePresentationUplink = null;
  if (!uplink) return;
  clearTimeout(uplink.probeTimer);
  uplink.mailbox.close();
  uplink.abortController.abort();
}

function applyGeneratedSnapshot(snapshot) {
  // The first value of a new physical stream is a complete authoritative
  // baseline. Never let revision or transient command UI retained from the
  // previous stream suppress pending/rejected/completed recovery.
  appliedSharedRevision = 0;
  screen.dataset.runtimeRevision = '0';
  clearTerminalPresentationEffects();
  terminalLiveBaselinePending = true;
  commandExecution = null;
  terminalNavigation = null;
  applyRecognitionSnapshot(snapshot.recognitionHandle, generatedPlayerState(snapshot.playerState, snapshot.revision));
  applyGeneratedTerminal(snapshot.terminalPresentation, snapshot.revision);
}

function applyGeneratedUpdate(update) {
  if (update.playerState) {
  applyPlayerState(generatedPlayerState(update.playerState, update.revision));
  }
  if (update.terminalPresentation) applyGeneratedTerminal(update.terminalPresentation, update.revision);
  if (update.navigation) {
  applyNavigationProjection(generatedNavigation(update.navigation), Number(update.revision), terminalID);
  }
  if (update.hacking) {
  applyHackingProjection(generatedHack(update.hacking), Number(update.revision), terminalID);
  }
}

function applyGeneratedTerminal(presentation, revision) {
  if (!presentation) return;
  if (presentation.presentation.case === 'noLiveTerminal') {
  applyNoLiveTerminal(Number(revision));
    return;
  }
  if (presentation.presentation.case !== 'liveTerminal') return;
  const live = presentation.presentation.value;
  applyLiveTerminal({
    revision: Number(revision),
    terminalId: live.terminalId,
    terminalName: live.terminalName,
    tree: generatedContentNode(live.tree),
    hackLevel: live.hackLevel,
    introText: live.introText,
    nav: generatedNavigation(live.navigation),
    hack: generatedHack(live.hacking),
    commandExecution: generatedCommandExecution(live.commandExecution),
    terminalNavigation: generatedTerminalNavigation(live.terminalNavigation),
    presentation: generatedControllerPresentation(live.controllerPresentation),
    effects: generatedTerminalPresentationEffects(live.effects),
  });
}

function generatedTerminalPresentationEffects(effects) {
  if (!Array.isArray(effects)) return [];
  return effects.includes(TerminalPresentationEffect.DISPLAY_UNSTABLE)
    ? ['display-unstable']
    : [];
}

function clearTerminalPresentationEffects() {
  // The effect loop is CSS-owned. Removing its sole selector cancels that
  // loop without an animation callback that could change terminal state.
  screen.removeAttribute('data-presentation-effect');
}

function applyTerminalPresentationEffects(effects) {
  clearTerminalPresentationEffects();
  if (Array.isArray(effects) && effects.includes('display-unstable')) {
    screen.dataset.presentationEffect = 'display-unstable';
  }
}

function generatedControllerPresentation(presentation) {
  if (!presentation) return { kind: 'none', contextKey: '', targetId: '', patternId: '', pageIndex: 0 };
  const result = {
    kind: presentation.presentation.case || 'none',
    contextKey: presentation.contextKey || '',
    targetId: '', patternId: '', pageIndex: 0,
  };
  if (presentation.presentation.case === 'menu') result.targetId = presentation.presentation.value.targetId || '';
  if (presentation.presentation.case === 'page') result.pageIndex = Number(presentation.presentation.value.pageIndex || 0);
  if (presentation.presentation.case === 'hacking') {
    const target = presentation.presentation.value.target;
    if (target.case === 'targetId') result.targetId = target.value || '';
    if (target.case === 'patternId') result.patternId = target.value || '';
  }
  return result;
}

function generatedTerminalNavigation(navigation) {
  if (!navigation) return null;
  return {
    routeDepth: Number(navigation.routeDepth || 0),
    returnTarget: navigation.returnTarget ? {
      terminalId: navigation.returnTarget.terminalId,
      terminalName: navigation.returnTarget.terminalName,
    } : null,
    pending: navigation.pending ? {
      direction: navigation.pending.direction === TerminalNavigationDirection.RETURN ? 'return' : 'forward',
      targetTerminalId: navigation.pending.targetTerminalId,
      targetTerminalName: navigation.pending.targetTerminalName,
    } : null,
  };
}

function generatedPlayerState(state, revision) {
  if (!state) return null;
  return {
    revision: Number(revision),
    sessionId: state.logicalSessionId,
    fallbackName: state.fallbackName,
    character: state.assignedCharacter ? {
      id: state.assignedCharacter.characterId,
      name: state.assignedCharacter.displayName,
    } : null,
    role: ({
      [PlayerRole.UNASSIGNED]: 'unassigned',
      [PlayerRole.ACTIVE]: 'active',
      [PlayerRole.OBSERVER]: 'observer',
    })[state.role] || 'unassigned',
    phase: ({
      [PlayerPhase.NO_BROADCAST]: 'no-broadcast',
      [PlayerPhase.SELECTING]: 'selecting',
      [PlayerPhase.WAITING]: 'waiting',
      [PlayerPhase.CONTROLLING]: 'controlling',
      [PlayerPhase.OBSERVING]: 'observing',
    })[state.phase] || 'no-broadcast',
    broadcastId: state.broadcastId || null,
    activeTerminalId: state.activeTerminalId || null,
    notice: state.notice?.kind === PlayerNoticeKind.COMMAND_PERSISTENCE_FAILED
      ? 'command-persistence-failed'
      : null,
    roster: (state.roster || []).map(entry => ({
      id: entry.characterId,
      name: entry.displayName,
      status: entry.availability === RosterAvailability.AVAILABLE ? 'available' : 'claimed',
    })),
  };
}

function generatedCommandExecution(presentation) {
  if (!presentation) return null;
  const phase = ({
    [CommandExecutionPhase.PENDING]: 'pending',
    [CommandExecutionPhase.REJECTED]: 'rejected',
  })[presentation.phase];
  if (!phase || !presentation.commandNodeId) return null;
  return { phase, commandNodeId: presentation.commandNodeId };
}

function generatedContentNode(node) {
  if (!node) return null;
  const result = { id: node.id, name: node.name, type: node.content.case };
  if (node.content.case === 'folder') result.children = (node.content.value.children || []).map(generatedContentNode);
  if (node.content.case === 'command') {
    result.text = node.content.value.text;
    if (typeof node.content.value.available === 'boolean') {
      result.available = node.content.value.available;
    }
  }
  if (node.content.case === 'entry') result.description = node.content.value.description;
  return result;
}

function generatedNavigation(nav) {
  if (!nav) return { path: ['root'], mode: 'list', viewEntryId: null, commandNodeId: null };
  return {
    path: Array.from(nav.path || []),
    mode: nav.mode === 2 ? 'entry' : 'list',
    viewEntryId: nav.viewEntryId || null,
    commandNodeId: nav.commandNodeId || null,
  };
}

function generatedHack(hackState) {
  if (!hackState) return null;
  return {
    level: hackState.level,
    wordLength: hackState.wordLength,
    attemptsMax: hackState.attemptsMax,
    attemptsLeft: hackState.attemptsLeft,
    solved: hackState.solved,
    failed: hackState.failed,
    log: Array.from(hackState.log || []),
    columns: (hackState.columns || []).map(column => ({
      addresses: Array.from(column.addresses || []), text: column.text,
      words: (column.words || []).map(word => ({ id: word.id, start: word.start, length: word.length })),
    })),
    patterns: (hackState.patterns || []).map(pattern => ({
      id: pattern.patternId, row: pattern.row, start: pattern.start, end: pattern.end, used: pattern.used,
    })),
  };
}

async function applyMutationResult(operation) {
  if (!sessionReady) return;
  try {
  const result = await operation();
  if (result) applyActionResult({
    requestId: result.requestId, accepted: result.accepted,
      reason: actionReasonName(result.reason), revision: Number(result.revision),
    });
  } catch (error) {
    console.warn('Player mutation failed', error);
    pendingSelection = null;
    pendingSharedAction = null;
    showPlayerNotice('ДЕЙСТВИЕ ОТКЛОНЕНО');
    render();
  }
}

async function applyPresentationMutationResult(requestId, operation) {
  if (!sessionReady) return;
  try {
    const result = await operation();
    if (result) applyActionResult({
      requestId: result.requestId,
      accepted: result.accepted,
      reason: actionReasonName(result.reason),
      revision: Number(result.revision),
    });
  } catch (error) {
    console.warn('Player presentation mutation failed', error);
    if (pendingPresentationAction?.requestId === requestId) {
      const failedPresentation = pendingPresentationAction.presentation;
      clearPendingPresentationAction();
      if (!desiredPresentationAction &&
          sameControllerPresentation(localControllerPresentation, failedPresentation)) {
        clearLocalControllerPresentation();
        render();
      }
      showPlayerNotice('ДЕЙСТВИЕ ОТКЛОНЕНО');
      renderPlayerContext();
      scheduleDesiredPresentationDispatch();
    }
  }
}

function selectCharacterRPC(requestId, broadcastId, characterId) {
  return applyMutationResult(() => playerRPC.selectCharacter({
    recognitionHandle: readBrowserToken(), requestId, broadcastId, characterId,
  }));
}

function navigateRPC(requestId, broadcastId, terminalId, actionName, nodeId) {
  const action = actionName === 'back'
    ? { case: 'back', value: {} }
    : { case: actionName, value: { nodeId } };
  return applyMutationResult(() => playerRPC.navigate({
    recognitionHandle: readBrowserToken(), requestId, broadcastId, terminalId, action,
  }));
}

function guessRPC(requestId, broadcastId, terminalId, targetId) {
  const filler = /^(\d+):(\d+)$/.exec(targetId);
  const target = filler
    ? { case: 'filler', value: { column: Number(filler[1]), character: Number(filler[2]) } }
    : { case: 'wordId', value: targetId };
  return applyMutationResult(() => playerRPC.guess({
    recognitionHandle: readBrowserToken(), requestId, broadcastId, terminalId, target,
  }));
}

function activatePatternRPC(requestId, broadcastId, terminalId, patternId) {
  return applyMutationResult(() => playerRPC.activatePattern({
    recognitionHandle: readBrowserToken(), requestId, broadcastId, terminalId, patternId,
  }));
}

function setPresentationRPC(requestId, broadcastId, terminalId, presentation) {
  const generatedPresentation = generatedPresentationRequest(presentation);
  return playerRPC.setPresentation({
    recognitionHandle: readBrowserToken(), requestId, broadcastId, terminalId,
    contextKey: presentation.contextKey,
    presentation: generatedPresentation,
  });
}

function generatedPresentationRequest(presentation) {
  let variant = { case: 'none', value: {} };
  if (presentation.kind === 'menu') {
    variant = { case: 'menu', value: { targetId: presentation.targetId } };
  } else if (presentation.kind === 'page') {
    variant = { case: 'page', value: { pageIndex: presentation.pageIndex } };
  } else if (presentation.kind === 'hacking') {
    const target = presentation.patternId
      ? { case: 'patternId', value: presentation.patternId }
      : { case: 'targetId', value: presentation.targetId };
    variant = { case: 'hacking', value: { target } };
  }
  return { contextKey: presentation.contextKey, presentation: variant };
}

function offerPresentationIntent(requestId, broadcastId, terminalId, presentation) {
  const uplink = activePresentationUplink;
  if (!uplink?.ready) return false;
  return uplink.mailbox.offer({
    payload: {
      case: 'intent',
      value: {
        recognitionHandle: readBrowserToken(),
        requestId,
        broadcastId,
        terminalId,
        contextKey: presentation.contextKey,
        presentation: generatedPresentationRequest(presentation),
      },
    },
  });
}

function actionReasonName(reason) {
  return ({
    [ActionReason.ACCEPTED]: 'accepted',
    [ActionReason.INVALID_SESSION]: 'invalid-session',
    [ActionReason.STALE_BROADCAST]: 'stale-broadcast',
    [ActionReason.UNASSIGNED]: 'unassigned',
    [ActionReason.NOT_CONTROLLER]: 'not-controller',
    [ActionReason.CONTROLLER_DISCONNECTED]: 'controller-disconnected',
    [ActionReason.STALE_TERMINAL]: 'stale-terminal',
    [ActionReason.INVALID_ACTION]: 'invalid-action',
    [ActionReason.CONFLICT]: 'conflict',
    [ActionReason.DUPLICATE]: 'duplicate',
  })[reason] || 'invalid-action';
}

// Applies a nav position pushed by the server. Never touches MODE.HACK —
// the hack gate always wins until it resolves.
function applyNavFromServer(nav) {
  navStack      = nav.path.slice();
  viewEntryId   = nav.viewEntryId || null;
  currentCommandNodeId = nav.commandNodeId || null;
  commandOutput = nav.commandNodeId ? ((findNodeById(tree, nav.commandNodeId) || {}).text ?? null) : null;
  if (mode !== MODE.HACK) {
    mode = nav.mode === 'entry' ? MODE.ENTRY : MODE.LIST;
  }
  applyPresentationSelection();
}

function acceptSharedSnapshot(message) {
  const revision = Number(message.revision);
  if (!Number.isFinite(revision)) return true;
  const playerRevision = playerState ? Number(playerState.revision) : -1;
  if (Number.isFinite(playerRevision) && revision < playerRevision) return false;
  if (revision < appliedSharedRevision) return false;
  appliedSharedRevision = revision;
  screen.dataset.runtimeRevision = String(revision);
  completeAcceptedSharedAction();
  completeAcceptedPresentationAction();
  return true;
}

function matchesActiveTerminal(message) {
  const activeTerminalID = playerState?.activeTerminalId;
  if (!activeTerminalID || terminalID !== activeTerminalID) return false;
  return !message.terminalId || message.terminalId === activeTerminalID;
}

function matchesExpectedTerminalLive(message) {
  return sessionReady && playerState !== null && playerState.character !== null &&
    playerState.activeTerminalId !== null &&
    message.terminalId === playerState.activeTerminalId;
}

function expectsTerminalClear() {
  return sessionReady && playerState !== null && playerState.activeTerminalId === null;
}

function hasCurrentTerminalMirror() {
  return hasLive && playerState !== null && playerState.character !== null &&
    playerState.broadcastId !== null && playerState.activeTerminalId !== null &&
    playerState.broadcastId === terminalBroadcastID &&
    playerState.activeTerminalId === terminalID;
}

function completeAcceptedSharedAction() {
  if (!pendingSharedAction || pendingSharedAction.acceptedRevision == null) return;
  if (appliedSharedRevision < pendingSharedAction.acceptedRevision) return;
  pendingSharedAction = null;
  showPlayerNotice('');
}

function completeAcceptedPresentationAction() {
  if (!pendingPresentationAction || pendingPresentationAction.acceptedRevision == null) return;
  if (appliedSharedRevision < pendingPresentationAction.acceptedRevision) return;
  clearPendingPresentationAction();
  showPlayerNotice('');
  scheduleDesiredPresentationDispatch();
}

function clearBroadcastMirrors() {
  clearTerminalPresentationEffects();
  hasLive = false;
  terminalID = '';
  terminalBroadcastID = '';
  terminalName = '';
  introText = '';
  serverNum = 1;
  tree = null;
  navStack = ['root'];
  selIndex = 0;
  mode = MODE.LIST;
  viewEntryId = null;
  commandOutput = null;
  currentCommandNodeId = null;
  commandExecution = null;
  terminalNavigation = null;
  controllerPresentation = { kind: 'none', contextKey: '', targetId: '', patternId: '', pageIndex: 0 };
  activeHackingContextKey = '';
  lastRenderedFolderKey = null;
  lastRenderedEntryId = null;
  lastRenderedCommandKey = null;
  lastRenderedHackKey = null;
  lastRenderedHackRows = new Map();
  hackBoardFit = null;
  hackLevel = 0;
  hack = null;
  hackTyped = '';
  hackHoverKey = null;
  hackHoverText = '';
  cancelHackHoverClear();
  hackWasSolved = false;
  lastAttemptsLeft = null;
  terminalLiveBaselinePending = true;
  appliedSharedRevision = 0;
  screen.dataset.runtimeRevision = '0';
  consumedRevealKey = null;

  clearTimeout(hackSolvedTimer);
  hackSolvedTimer = null;
  setAmbientActive(false);
  deactivatePagination();

  for (const container of [termList, entryBody, termOutput]) {
    cancelReveal(container);
    container._revealedContentIdentity = null;
    container.replaceChildren();
  }
  cancelReveal(hackColumns);
  hackColumns._revealedContentIdentity = null;
  hackColumns.replaceChildren();
  hackBoard.style.removeProperty('--hack-row-font');
  hackBoard.classList.remove('hack-compact', 'hack-stacked', 'hack-tight');
  hackLog.replaceChildren();
  introTextEl.textContent = '';
  entryTitle.textContent = '';
  attemptsLine.textContent = '';
  hackInputPreview.textContent = '';
}

function playHackOutcomeTransition(previousHack, nextHack, revision = appliedSharedRevision) {
  if (!previousHack || !nextHack) return;
  if (!nextHack.solved && nextHack.attemptsLeft < previousHack.attemptsLeft &&
      window.__falloutTerminalShouldPlayAuthoritativeCue?.(revision, 'hack-bad')) {
    playHackBad();
  }
  if (nextHack.solved && !previousHack.solved &&
      window.__falloutTerminalShouldPlayAuthoritativeCue?.(revision, 'hack-good')) {
    playHackGood();
  }
}

function isHackingContextKey(contextKey) {
  return typeof contextKey === 'string' && contextKey.startsWith('hack:');
}

function hackingContextKey(presentation = controllerPresentation) {
  const contextKey = typeof presentation?.contextKey === 'string'
    ? presentation.contextKey
    : '';
  return isHackingContextKey(contextKey) ? contextKey : '';
}

function reconcileSolvedHack(previousHack, nextHack, contextKey) {
  if (!previousHack || previousHack.solved || !nextHack?.solved) return;

  const endedContextKeys = new Set([
    activeHackingContextKey,
    contextKey,
    hackingContextKey(pendingPresentationAction?.presentation),
    hackingContextKey(desiredPresentationAction),
    hackingContextKey(localControllerPresentation),
    pendingSharedAction?.contextKey,
    transientPlayerNoticeContextKey,
  ].filter(isHackingContextKey));

  if (endedContextKeys.has(pendingSharedAction?.contextKey)) pendingSharedAction = null;
  if (pendingPresentationAction?.presentation.kind === 'hacking' &&
      endedContextKeys.has(pendingPresentationAction.presentation.contextKey)) {
    clearPendingPresentationAction();
  }
  if (desiredPresentationAction?.kind === 'hacking' &&
      endedContextKeys.has(desiredPresentationAction.contextKey)) {
    desiredPresentationAction = null;
  }
  if (localControllerPresentation?.kind === 'hacking' &&
      endedContextKeys.has(localControllerPresentation.contextKey)) {
    clearLocalControllerPresentation();
  }
  if (endedContextKeys.has(transientPlayerNoticeContextKey)) showPlayerNotice('');
  activeHackingContextKey = '';
}

function scheduleHackSolvedNavigation() {
  if (!hack || !hack.solved || hackSolvedTimer) return;
  hackSolvedTimer = setTimeout(() => {
    hackSolvedTimer = null;
    mode = MODE.LIST;
    render();
  }, 2600);
}

function applyRecognitionSnapshot(recognitionHandle, state) {
    if (!state || typeof recognitionHandle !== 'string' || !recognitionHandle) return;
    try {
      localStorage.setItem(PLAYER_TOKEN_KEY, recognitionHandle);
    } catch (error) {
      console.warn('Player recognition token could not be stored', error);
    }
    signalRecognitionChange();
    sessionReady = true;
    terminalLiveBaselinePending = true;
    pendingSelection = null;
    pendingSharedAction = null;
    clearControllerPresentationDispatch();
    showPlayerNotice('');
    applyPlayerState(state, { authoritativeWelcome: true });
    if (!hasCurrentTerminalMirror()) appliedSharedRevision = 0;
    connOverlay.classList.add('hidden');
}

function applyLiveTerminal(msg) {
    if (!matchesExpectedTerminalLive(msg) || !acceptSharedSnapshot(msg)) return;
    applyTerminalPresentationEffects(msg.effects);
    const previousHack = hack;
    const nextTerminalID = msg.terminalId || '';
    const nextBroadcastID = playerState?.broadcastId || '';
    const terminalIdentityChanged = !hasLive || terminalID !== nextTerminalID ||
      terminalBroadcastID !== nextBroadcastID;
    const isContinuousTerminalUpdate = !terminalLiveBaselinePending && hasLive &&
      terminalID === msg.terminalId && mode === MODE.HACK;
    hasLive       = true;
    terminalID    = nextTerminalID;
    terminalBroadcastID = nextBroadcastID;
    terminalName  = msg.terminalName || '';
    introText     = msg.introText || '';
    tree          = msg.tree;
    hackLevel     = msg.hackLevel || 0;
    hack          = msg.hack || null;
    commandExecution = msg.commandExecution || null;
    terminalNavigation = msg.terminalNavigation || null;
    const previousPresentation = controllerPresentation;
    controllerPresentation = msg.presentation || { kind: 'none', contextKey: '', targetId: '', patternId: '', pageIndex: 0 };
    const nextHackingContextKey = hackingContextKey();
    const solvedHackContextKey = activeHackingContextKey ||
      hackingContextKey(previousPresentation) || nextHackingContextKey;
    if (!hack?.solved && nextHackingContextKey) activeHackingContextKey = nextHackingContextKey;
    reconcileSolvedHack(previousHack, hack, solvedHackContextKey);
    if (pendingPresentationAction?.transport === 'stream' &&
        sameControllerPresentation(pendingPresentationAction.presentation, controllerPresentation)) {
      clearTimeout(pendingPresentationAction.resultTimer);
      pendingPresentationAction.resultTimer = null;
    }
    if (localControllerPresentation &&
        (localControllerPresentation.contextKey !== controllerPresentation.contextKey ||
         sameControllerPresentation(localControllerPresentation, controllerPresentation))) {
      clearLocalControllerPresentation();
    }
    if (desiredPresentationAction &&
        desiredPresentationAction.contextKey !== controllerPresentation.contextKey) {
      desiredPresentationAction = null;
    }
    serverNum     = 1 + Math.floor(Math.random() * 9);
    if (!isContinuousTerminalUpdate) {
      hackTyped     = '';
      hackHoverKey  = null;
      hackHoverText = '';
    }
    hackWasSolved = !!(hack && hack.solved);
    lastAttemptsLeft = hack ? hack.attemptsLeft : null;
    if (!isContinuousTerminalUpdate) {
      clearTimeout(hackSolvedTimer);
      hackSolvedTimer = null;
    }

    const nav = msg.nav || { path: ['root'], mode: 'list', viewEntryId: null, commandNodeId: null };
    navStack      = nav.path.slice();
    viewEntryId   = nav.viewEntryId || null;
    currentCommandNodeId = nav.commandNodeId || null;
    commandOutput = nav.commandNodeId ? ((findNodeById(tree, nav.commandNodeId) || {}).text ?? null) : null;
    if (terminalIdentityChanged) {
      lastRenderedFolderKey  = null;
      lastRenderedEntryId    = null;
      lastRenderedCommandKey = null;
      lastRenderedHackKey    = null;
      lastRenderedHackRows   = new Map();
    }

    mode = (hackLevel > 0 && hack && (!hack.solved || isContinuousTerminalUpdate))
      ? MODE.HACK
      : (nav.mode === 'entry' ? MODE.ENTRY : MODE.LIST);

    applyPresentationSelection();
    const canonicalPresentationIsSuperseded = localControllerPresentation &&
      localControllerPresentation.contextKey === controllerPresentation.contextKey &&
      !sameControllerPresentation(localControllerPresentation, controllerPresentation);
    if (!canonicalPresentationIsSuperseded) {
      playControllerPresentationCue(previousPresentation, controllerPresentation, terminalLiveBaselinePending);
    }

    if (isContinuousTerminalUpdate) playHackOutcomeTransition(previousHack, hack);
    if (isContinuousTerminalUpdate) scheduleHackSolvedNavigation();
    terminalLiveBaselinePending = false;

    setAmbientActive(true);
    render();
}

function applyPresentationSelection() {
  const presentation = effectiveControllerPresentation();
  if (presentation.kind !== 'menu') return;
  const kids = currentFolderNode()?.children || [];
  const index = kids.findIndex(node => node.id === presentation.targetId);
  selIndex = index >= 0 ? index : 0;
}

function playControllerPresentationCue(previous, next, baseline) {
  if (baseline || !next || JSON.stringify(previous) === JSON.stringify(next)) return;
  if (next.kind === 'menu') {
    playMenuFocus();
  } else if (next.kind === 'hacking') {
    if (next.patternId || !/^\d+:\d+$/.test(next.targetId)) playMultiple();
    else playSingle();
  } else if (next.kind === 'page') {
    playEnter();
  }
}

function applyNavigationProjection(nav, revision, projectedTerminalID) {
    const projection = { nav, revision, terminalId: projectedTerminalID };
    if (!matchesActiveTerminal(projection) || !acceptSharedSnapshot(projection)) return;
    applyNavFromServer(nav);
    render();
}

function applyHackingProjection(nextHack, revision, projectedTerminalID) {
    const projection = { hack: nextHack, revision, terminalId: projectedTerminalID };
    const msg = projection;
    if (!matchesActiveTerminal(msg) || !acceptSharedSnapshot(msg)) return;
    const previousHack = hack;
    hack = nextHack;
    if (mode !== MODE.HACK || !hack) return;
    reconcileSolvedHack(
      previousHack,
      hack,
      hackingContextKey(),
    );
    playHackOutcomeTransition(previousHack, hack);
    lastAttemptsLeft = hack.attemptsLeft;
    hackWasSolved = hack.solved;

    scheduleHackSolvedNavigation();
    render();
}

function applyNoLiveTerminal(revision) {
    const projection = { revision };
    if (!expectsTerminalClear() || !acceptSharedSnapshot(projection)) return;
    clearTerminalPresentationEffects();
    hasLive = false;
    terminalID = '';
    terminalBroadcastID = '';
    tree = null;
    hack = null;
    commandExecution = null;
  terminalNavigation = null;
  controllerPresentation = { kind: 'none', contextKey: '', targetId: '', patternId: '', pageIndex: 0 };
  activeHackingContextKey = '';
    clearLocalControllerPresentation();
    clearTimeout(hackSolvedTimer);
    hackSolvedTimer = null;
    setAmbientActive(false);
    render();
}

function applyPlayerState(nextState, { authoritativeWelcome = false } = {}) {
  const revision = Number(nextState.revision);
  const currentRevision = playerState ? Number(playerState.revision) : -1;
  if (!authoritativeWelcome && Number.isFinite(revision) &&
      Number.isFinite(currentRevision) && revision < currentRevision) return;

  const previousState = playerState;
  const roster = [];
  const rosterByID = new Map();
  if (Array.isArray(nextState.roster)) {
    for (const entry of nextState.roster) {
      if (!entry || typeof entry !== 'object') continue;
      const id = String(entry.id || '');
      if (!id || rosterByID.has(id)) continue;
      const normalized = {
        id,
        name: String(entry.name || ''),
        status: entry.status === 'available' ? 'available' : 'claimed',
      };
      roster.push(normalized);
      rosterByID.set(id, normalized);
    }
  }

  let character = null;
  if (nextState.character && typeof nextState.character === 'object') {
    const id = String(nextState.character.id || '');
    if (id) {
      const rosterCharacter = rosterByID.get(id);
      character = {
        id,
        name: rosterCharacter ? rosterCharacter.name : String(nextState.character.name || ''),
      };
    }
  }

  const nextPlayerState = {
    revision: Number.isFinite(revision) ? revision : 0,
    sessionId: String(nextState.sessionId || ''),
    fallbackName: String(nextState.fallbackName || ''),
    character,
    role: String(nextState.role || 'unassigned'),
    phase: String(nextState.phase || 'no-broadcast'),
    broadcastId: nextState.broadcastId == null ? null : String(nextState.broadcastId),
    activeTerminalId: nextState.activeTerminalId == null ? null : String(nextState.activeTerminalId),
    notice: nextState.notice === 'command-persistence-failed'
      ? 'command-persistence-failed'
      : null,
    roster,
  };
  playerState = nextPlayerState;
  if (nextPlayerState.notice !== null) {
    transientPlayerNotice = '';
    transientPlayerNoticeContextKey = '';
  }

  const broadcastChanged = previousState !== null &&
    previousState.broadcastId !== nextPlayerState.broadcastId;
  const activeTerminalChanged = previousState !== null &&
    previousState.activeTerminalId !== nextPlayerState.activeTerminalId;
  const outsideBroadcast = nextPlayerState.broadcastId === null ||
    nextPlayerState.phase === 'no-broadcast';
  if (broadcastChanged || outsideBroadcast) {
    pendingSelection = null;
    pendingSharedAction = null;
    clearControllerPresentationDispatch();
    clearBroadcastMirrors();
    showPlayerNotice('');
  } else if (pendingSelection && nextPlayerState.phase !== 'selecting') {
    pendingSelection = null;
    showPlayerNotice('');
  }

  if (activeTerminalChanged) clearControllerPresentationDispatch();

  if (nextPlayerState.role !== 'active' || nextPlayerState.phase !== 'controlling') {
    clearControllerPresentationDispatch();
  }

  if (authoritativeWelcome || !hasCurrentTerminalMirror()) {
    setAmbientActive(false);
  }

  if (pendingSelection && pendingSelection.acceptedRevision != null &&
      playerState.revision >= pendingSelection.acceptedRevision) {
    pendingSelection = null;
    showPlayerNotice('');
  }

  const terminalPhases = new Set(['controlling', 'observing']);
  const retainsCurrentTerminalSurface = previousState !== null && hasLive &&
    previousState.character !== null && nextPlayerState.character !== null &&
    terminalPhases.has(previousState.phase) && terminalPhases.has(nextPlayerState.phase) &&
    previousState.broadcastId === nextPlayerState.broadcastId &&
    nextPlayerState.broadcastId === terminalBroadcastID &&
    previousState.activeTerminalId === nextPlayerState.activeTerminalId &&
    nextPlayerState.activeTerminalId === terminalID;
  if (retainsCurrentTerminalSurface) {
    renderPlayerContext();
    return;
  }
  render();
}

function applyActionResult(result) {
  if (pendingPresentationAction && result.requestId === pendingPresentationAction.requestId) {
    if (!result.accepted) {
      const rejectedPresentation = pendingPresentationAction.presentation;
      clearPendingPresentationAction();
      if (!desiredPresentationAction &&
          sameControllerPresentation(localControllerPresentation, rejectedPresentation)) {
        clearLocalControllerPresentation();
        render();
      }
      const contextKey = hackingContextKey(rejectedPresentation);
      showPlayerNotice(
        `ДЕЙСТВИЕ ОТКЛОНЕНО: ${String(result.reason || 'invalid-action')}`,
        contextKey,
      );
      scheduleDesiredPresentationDispatch();
    } else {
      pendingPresentationAction.acceptedRevision = Number(result.revision) || 0;
      completeAcceptedPresentationAction();
    }
    renderPlayerContext();
    return;
  }

  if (pendingSharedAction && result.requestId === pendingSharedAction.requestId) {
    if (!result.accepted) {
      const contextKey = pendingSharedAction.contextKey;
      pendingSharedAction = null;
      showPlayerNotice(
        `ДЕЙСТВИЕ ОТКЛОНЕНО: ${String(result.reason || 'invalid-action')}`,
        contextKey,
      );
    } else {
      pendingSharedAction.acceptedRevision = Number(result.revision) || 0;
      completeAcceptedSharedAction();
    }
    // The authoritative stream may render the accepted navigation before the
    // unary result arrives. Re-render the complete surface after clearing the
    // request so controls disabled during that earlier render are restored.
    render();
    return;
  }

  if (!pendingSelection || result.requestId !== pendingSelection.requestId) return;

  if (!result.accepted) {
    pendingSelection = null;
    const messages = {
      conflict: 'ПЕРСОНАЖ УЖЕ ЗАНЯТ (conflict). ВЫБЕРИТЕ ДРУГОГО.',
      'stale-broadcast': 'ТРАНСЛЯЦИЯ ИЗМЕНИЛАСЬ. ДОЖДИТЕСЬ НОВОГО СПИСКА.',
      duplicate: 'ЗАПРОС УЖЕ БЫЛ ОБРАБОТАН.',
    };
    showPlayerNotice(messages[result.reason] || `ВЫБОР ОТКЛОНЕН: ${String(result.reason || 'invalid-action')}`);
    render();
    return;
  }

  pendingSelection.acceptedRevision = Number(result.revision) || 0;
  if (playerState && playerState.revision >= pendingSelection.acceptedRevision) {
    pendingSelection = null;
    showPlayerNotice('');
  }
  render();
}

function createRequestID() {
  if (window.crypto && typeof window.crypto.randomUUID === 'function') {
    return window.crypto.randomUUID();
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

function canControlSharedTerminalAction(action) {
  const executionAllowsAction = commandExecution === null ||
    (commandExecution.phase === 'rejected' && action === 'back');
  return sessionReady && playerState !== null && hasCurrentTerminalMirror() &&
    executionAllowsAction &&
  terminalNavigation?.pending == null &&
    playerState.role === 'active' &&
    playerState.phase === 'controlling' &&
    playerState.broadcastId !== null &&
    playerState.activeTerminalId !== null &&
    playerState.activeTerminalId === terminalID &&
    pendingSharedAction === null;
}

function canControlSharedTerminal() {
  return canControlSharedTerminalAction('');
}

function canControlTerminalPresentation() {
  return sessionReady && playerState !== null && hasCurrentTerminalMirror() &&
    terminalNavigation?.pending == null &&
    playerState.role === 'active' &&
    playerState.phase === 'controlling' &&
    playerState.broadcastId !== null &&
    playerState.activeTerminalId !== null &&
    playerState.activeTerminalId === terminalID;
}

function beginSharedMutation(procedure, invoke) {
  if (!canControlSharedTerminal()) return false;

  const requestId = createRequestID();
  const contextKey = mode === MODE.HACK ? hackingContextKey() : '';
  pendingSharedAction = { requestId, procedure, acceptedRevision: null, contextKey };
  showPlayerNotice('');
  renderPlayerContext();
  void invoke(requestId, playerState.broadcastId, playerState.activeTerminalId);
  return true;
}

function beginSharedMutationForAction(procedure, invoke, action) {
  if (!canControlSharedTerminalAction(action)) return false;

  const requestId = createRequestID();
  const contextKey = mode === MODE.HACK ? hackingContextKey() : '';
  pendingSharedAction = { requestId, procedure, acceptedRevision: null, contextKey };
  showPlayerNotice('');
  renderPlayerContext();
  void invoke(requestId, playerState.broadcastId, playerState.activeTerminalId);
  return true;
}

function beginControllerPresentation(next) {
  if (!canControlTerminalPresentation() || !controllerPresentation.contextKey) return false;
  const presentation = {
    kind: next.kind || 'none',
    contextKey: controllerPresentation.contextKey,
    targetId: next.targetId || '',
    patternId: next.patternId || '',
    pageIndex: Number(next.pageIndex || 0),
  };

  if (sameControllerPresentation(presentation, effectiveControllerPresentation())) return false;
  showLocalControllerPresentation(presentation);

  if (pendingPresentationAction) {
    if (sameControllerPresentation(presentation, pendingPresentationAction.presentation)) {
      desiredPresentationAction = null;
      return true;
    }
    if (sameControllerPresentation(presentation, desiredPresentationAction)) return false;
    desiredPresentationAction = presentation;
    return true;
  }

  desiredPresentationAction = null;
  return dispatchControllerPresentation(presentation);
}

function sameControllerPresentation(left, right) {
  return !!left && !!right && left.kind === right.kind &&
    left.contextKey === right.contextKey &&
    left.targetId === right.targetId &&
    left.patternId === right.patternId &&
    left.pageIndex === right.pageIndex;
}

function clearControllerPresentationDispatch() {
  clearPendingPresentationAction();
  desiredPresentationAction = null;
  presentationDrainScheduled = false;
  clearLocalControllerPresentation();
}

function clearPendingPresentationAction() {
  const pending = pendingPresentationAction;
  if (pending?.resultTimer != null) clearTimeout(pending.resultTimer);
  pendingPresentationAction = null;
}

function effectiveControllerPresentation() {
  const local = localControllerPresentation;
  if (local && local.contextKey === controllerPresentation.contextKey && canControlTerminalPresentation()) {
    return local;
  }
  return controllerPresentation;
}

function showLocalControllerPresentation(presentation) {
  localControllerPresentation = { ...presentation };
  if (localPresentationFrame !== null) return;
  localPresentationFrame = requestAnimationFrame(() => {
    localPresentationFrame = null;
    if (!localControllerPresentation || !canControlTerminalPresentation() ||
        localControllerPresentation.contextKey !== controllerPresentation.contextKey) {
      clearLocalControllerPresentation();
      return;
    }
    applyPresentationSelection();
    render();
  });
}

function clearLocalControllerPresentation() {
  localControllerPresentation = null;
  if (localPresentationFrame !== null) {
    cancelAnimationFrame(localPresentationFrame);
    localPresentationFrame = null;
  }
}

function scheduleDesiredPresentationDispatch() {
  if (presentationDrainScheduled || !desiredPresentationAction) return;
  presentationDrainScheduled = true;
  queueMicrotask(() => {
    presentationDrainScheduled = false;
    if (pendingPresentationAction || !desiredPresentationAction) return;
    const presentation = desiredPresentationAction;
    desiredPresentationAction = null;
    if (!canControlTerminalPresentation() ||
        presentation.contextKey !== controllerPresentation.contextKey ||
        sameControllerPresentation(presentation, controllerPresentation)) {
      return;
    }
    dispatchControllerPresentation(presentation);
  });
}

function dispatchControllerPresentation(presentation) {
  if (!canControlTerminalPresentation() || !controllerPresentation.contextKey ||
      presentation.contextKey !== controllerPresentation.contextKey ||
      sameControllerPresentation(presentation, controllerPresentation)) {
    return false;
  }
  const requestId = createRequestID();
  const streamed = offerPresentationIntent(
    requestId,
    playerState.broadcastId,
    playerState.activeTerminalId,
    presentation,
  );
  const streamedGeneration = streamed ? activePresentationUplink.generation : 0;
  reportPresentationDiagnostic('presentation-dispatched', {
    transport: streamed ? 'stream' : 'unary',
    generation: streamedGeneration,
  });
  pendingPresentationAction = {
    requestId,
    acceptedRevision: null,
    presentation,
    transport: streamed ? 'stream' : 'unary',
    resultTimer: streamed
      ? setTimeout(() => {
        reportPresentationDiagnostic('presentation-result-timeout', { generation: streamedGeneration });
        failPresentationUplink(streamedGeneration);
      }, PRESENTATION_RESULT_TIMEOUT_MS)
      : null,
  };
  showPlayerNotice('');
  renderPlayerContext();
  if (!streamed) {
    void applyPresentationMutationResult(requestId, () => setPresentationRPC(
      requestId,
      playerState.broadcastId,
      playerState.activeTerminalId,
      presentation,
    ));
  }
  return true;
}

function beginNavigation(action, nodeId = '') {
  const invoke = (requestId, broadcastId, terminalId) =>
    navigateRPC(requestId, broadcastId, terminalId, action, nodeId);
  if (action === 'back' && commandExecution?.phase === 'rejected') {
    return beginSharedMutationForAction('navigate', invoke, action);
  }
  return beginSharedMutation('navigate', invoke);
}

function beginGuess(targetId) {
  return beginSharedMutation('guess', (requestId, broadcastId, terminalId) =>
    guessRPC(requestId, broadcastId, terminalId, targetId));
}

function beginPattern(patternId) {
  return beginSharedMutation('activatePattern', (requestId, broadcastId, terminalId) =>
    activatePatternRPC(requestId, broadcastId, terminalId, patternId));
}

function selectCharacter(characterID) {
  if (!sessionReady || !playerState || pendingSelection ||
      playerState.phase !== 'selecting' || !playerState.broadcastId) return;

  const entry = playerState.roster.find(candidate =>
    candidate.id === characterID && candidate.status === 'available'
  );
  if (!entry) return;

  const requestId = createRequestID();
  pendingSelection = { requestId, acceptedRevision: null };
  showPlayerNotice('');
  render();
  void selectCharacterRPC(requestId, playerState.broadcastId, entry.id);
}

function showPlayerNotice(message, contextKey = '') {
  transientPlayerNotice = message;
  transientPlayerNoticeContextKey = message ? contextKey : '';
  renderPlayerNotice();
}

function isTerminalNavigationPending() {
  return terminalNavigation?.pending != null;
}

function renderPlayerNotice() {
  const authoritativeMessage = playerState?.role === 'active' &&
    playerState.notice === 'command-persistence-failed'
    ? 'НЕ УДАЛОСЬ СОХРАНИТЬ СОСТОЯНИЕ КОМАНДЫ. СОСТОЯНИЕ КОМАНДЫ НЕ ИЗМЕНЕНО.'
    : '';
  const message = isTerminalNavigationPending()
    ? ''
    : authoritativeMessage || transientPlayerNotice;
  playerNotice.textContent = message;
  playerNotice.hidden = !message;
  playerNotice.dataset.kind = authoritativeMessage ? 'error' : '';
}

function renderRoster() {
  characterOptions.replaceChildren();
  for (const entry of playerState.roster) {
    const option = document.createElement('button');
    option.type = 'button';
    option.className = 'character-option';
    option.dataset.characterId = entry.id;
    option.dataset.status = entry.status;
    option.setAttribute('role', 'listitem');
    option.textContent = entry.name;
    option.disabled = entry.status !== 'available' || pendingSelection !== null;
    option.addEventListener('click', () => selectCharacter(entry.id));
    characterOptions.appendChild(option);
  }
}

function compactPlayerInputLabel(fallbackName) {
  const label = fallbackName.trim();
  const defaultPlayer = /^PLAYER\s+(\d+)$/i.exec(label);
  if (defaultPlayer) return `P${defaultPlayer[1]}`;
  return label || 'P?';
}

function renderPlayerContext() {
  const hasState = sessionReady && playerState !== null;
  playerIdentity.hidden = !hasState;
  characterSelect.hidden = !hasState || playerState.phase !== 'selecting';
  assignedWaiting.hidden = !hasState || playerState.phase !== 'waiting';

  const observerReadOnly = hasState && playerState.role === 'observer';
  const commandRequestPending = commandExecution?.phase === 'pending';
  const terminalNavigationPending = terminalNavigation?.pending != null;
  const commandRequestRejected = commandExecution?.phase === 'rejected';
  const blockingSharedInputPending = pendingSharedAction !== null || commandRequestPending || terminalNavigationPending;
  roleBadge.dataset.role = hasState ? playerState.role : '';
  screen.classList.toggle('observer-read-only', observerReadOnly);
  screen.classList.toggle('shared-input-pending', blockingSharedInputPending);
  screen.classList.toggle('shared-command-rejected', commandRequestRejected);
  screen.setAttribute('aria-readonly', String(observerReadOnly));
  renderPlayerNotice();

  if (!hasState) return false;

  const characterName = playerState.character?.name?.trim() || '';
  playerFallbackName.textContent = compactPlayerInputLabel(playerState.fallbackName || '');
  playerCharacterName.textContent = characterName;
  playerCharacterName.hidden = !characterName;
  playerCharacterSeparator.hidden = !characterName;
  const roleLabels = {
    active: 'АКТИВЕН',
    observer: 'НАБЛЮДАТЕЛЬ',
    unassigned: 'НЕ НАЗНАЧЕН',
  };
  roleBadge.textContent = roleLabels[playerState.role] || playerState.role;

  const selecting = playerState.phase === 'selecting';
  characterSelect.classList.toggle('pending', selecting && pendingSelection !== null);
  characterSelect.setAttribute('aria-busy', String(selecting && pendingSelection !== null));
  if (selecting) renderRoster();

  return selecting || playerState.phase === 'waiting';
}

function hideTerminalSurface() {
  deactivatePagination();
  cancelReveal(termList);
  cancelReveal(entryBody);
  cancelReveal(termOutput);
  normalHeader.hidden = true;
  hackHeader.hidden = true;
  termIdle.hidden = true;
  termList.hidden = true;
  termEntry.hidden = true;
  hackBoard.hidden = true;
  hackBlocked.hidden = true;
  termOutput.hidden = true;
  termOutput.classList.remove('command-screen', 'command-execution-status', 'command-result-screen');
  termPrompt.hidden = true;
  backBtn.hidden = true;
  pageNav.hidden = true;
}

// ════════════════════════════════════════════════════
// TREE HELPERS
// ════════════════════════════════════════════════════
function findNodeById(root, id) {
  if (!root) return null;
  if (root.id === id) return root;
  if (root.children) {
    for (const c of root.children) {
      const found = findNodeById(c, id);
      if (found) return found;
    }
  }
  return null;
}

function currentFolderNode() {
  let cur = tree;
  for (let i = 1; i < navStack.length; i++) {
    cur = cur.children.find(c => c.id === navStack[i]);
    if (!cur) { cur = tree; break; }
  }
  return cur;
}

// ════════════════════════════════════════════════════
// NORMAL-SCREEN NAVIGATION ACTIONS — the shared position changes only after
// the generated subscription carries the applicable authoritative revision.
// ════════════════════════════════════════════════════
function activateRow(node) {
  if (node.type === 'folder') {
  beginNavigation('enter', node.id);
  } else if (node.type === 'command') {
  if (node.available === false) return;
  beginNavigation('command', node.id);
  } else if (node.type === 'entry') {
  beginNavigation('entry', node.id);
  }
}

function goBack() {
  if (mode === MODE.HACK) return;
  beginNavigation('back');
}

backBtn.addEventListener('click', goBack);
pagePrev.addEventListener('click', () => changePage(-1));
pageNext.addEventListener('click', () => changePage(1));

// ════════════════════════════════════════════════════
// MENU HOVER: highlight the entry under the cursor + focus sound
// ════════════════════════════════════════════════════
let lastMenuHoverIdx = null;
termList.addEventListener('mouseover', (e) => {
  const row = e.target.closest('.term-row');
  if (!row || row.dataset.idx == null) return;
  if (!canControlTerminalPresentation()) return;
  if (row.dataset.idx === lastMenuHoverIdx) return;
  lastMenuHoverIdx = row.dataset.idx;
  beginControllerPresentation({ kind: 'menu', targetId: row.dataset.nodeId });
});
termList.addEventListener('mouseleave', () => { lastMenuHoverIdx = null; });

// ════════════════════════════════════════════════════
// HACKING MINIGAME — hover/click on the board
// ════════════════════════════════════════════════════
function cssEscape(s) {
  return (window.CSS && CSS.escape) ? CSS.escape(s) : String(s).replace(/[^a-zA-Z0-9_-]/g, '\\$&');
}

function setHackHover(key, force = false) {
  if (!force && hackHoverKey === key) return;
  hackColumns.querySelectorAll('.hcell.hi').forEach(el => el.classList.remove('hi'));
  hackHoverKey = key;
  hackHoverText = '';
  if (key != null) {
    const els = hackColumns.querySelectorAll(`[data-target="${cssEscape(key)}"]`);
    els.forEach(el => el.classList.add('hi'));
    const word = (hack?.columns || []).flatMap(column =>
      (column.words || []).map(candidate => ({ column, candidate })),
    ).find(({ candidate }) => candidate.id === key);
    hackHoverText = word
      ? word.column.text.slice(word.candidate.start, word.candidate.start + word.candidate.length)
      : Array.from(els).map(el => el.textContent).join('');
  }
  renderHackInputPreview();
}

function cancelHackHoverClear() {
  if (hackHoverClearTimer === null) return;
  clearTimeout(hackHoverClearTimer);
  hackHoverClearTimer = null;
}

function scheduleHackHoverClear(key) {
  cancelHackHoverClear();
  hackHoverClearTimer = setTimeout(() => {
    hackHoverClearTimer = null;
    if (hackHoverKey === key && canControlTerminalPresentation()) {
      beginControllerPresentation({ kind: 'none' });
    }
  }, 0);
}

function patternAtCell(cell) {
  if (!hack || !cell || cell.dataset.row == null || cell.dataset.offset == null) return null;
  const row = Number(cell.dataset.row);
  const offset = Number(cell.dataset.offset);
  return (hack.patterns || []).find(pattern =>
    pattern.row === row && pattern.start === offset
  ) || null;
}

function setHackPatternHover(pattern) {
  hackColumns.querySelectorAll('.hcell.hi').forEach(el => el.classList.remove('hi'));
  hackHoverKey = pattern ? pattern.id : null;
  hackHoverText = '';
  if (pattern && !pattern.used) {
    const cells = hackColumns.querySelectorAll(`[data-row="${pattern.row}"][data-offset]`);
    cells.forEach(cell => {
      const offset = Number(cell.dataset.offset);
      if (offset >= pattern.start && offset <= pattern.end) cell.classList.add('hi');
    });
    hackHoverText = Array.from(cells)
      .filter(cell => {
        const offset = Number(cell.dataset.offset);
        return offset >= pattern.start && offset <= pattern.end;
      })
      .map(cell => cell.textContent)
      .join('');
  }
  renderHackInputPreview();
}

function previewHackCell(cell) {
  if (!cell || !hack || hack.solved || hack.failed || !canControlTerminalPresentation()) return;
  cancelHackHoverClear();
  const pattern = patternAtCell(cell);
  if (pattern) {
    if (!pattern.used) beginControllerPresentation({ kind: 'hacking', patternId: pattern.id });
    return;
  }
  const key = cell.dataset.target;
  beginControllerPresentation({ kind: 'hacking', targetId: key });
}

hackColumns.addEventListener('mouseover', (e) => {
  previewHackCell(e.target.closest('.hcell'));
});
hackColumns.addEventListener('mouseout', (e) => {
  const cell = e.target.closest('.hcell');
  if (!cell || !cell.isConnected) return;
  const pattern = patternAtCell(cell);
  if (pattern) {
    const related = e.relatedTarget && e.relatedTarget.closest ? e.relatedTarget.closest('.hcell') : null;
    const relatedPattern = patternAtCell(related);
    if (!relatedPattern || relatedPattern.id !== pattern.id) scheduleHackHoverClear(pattern.id);
    return;
  }
  const related = e.relatedTarget && e.relatedTarget.closest ? e.relatedTarget.closest('.hcell') : null;
  if (!related || related.dataset.target !== cell.dataset.target) scheduleHackHoverClear(cell.dataset.target);
});
hackColumns.addEventListener('focusin', (e) => {
  previewHackCell(e.target.closest('.hcell'));
});
hackColumns.addEventListener('focusout', (e) => {
  const cell = e.target.closest('.hcell');
  if (!cell || !cell.isConnected) return;
  const pattern = patternAtCell(cell);
  scheduleHackHoverClear(pattern ? pattern.id : cell.dataset.target);
});
hackColumns.addEventListener('click', (e) => {
  const cell = e.target.closest('.hcell');
  if (!cell || !hack || hack.solved || hack.failed) return;
  const pattern = patternAtCell(cell);
  if (pattern && !pattern.used) {
  if (beginPattern(pattern.id)) playEnter();
    return;
  }
  if (pattern) return;
  if (beginGuess(cell.dataset.target)) playEnter();
});

// ════════════════════════════════════════════════════
// KEYBOARD
// ════════════════════════════════════════════════════
function revealPhysicalKey(event) {
  return event.code || event.key || 'Unidentified';
}

function visibleRevealController(controller) {
  const container = controller?.container;
  return Boolean(container && container.isConnected && !container.hidden && !container.closest('[hidden]'));
}

function completeVisibleReveals() {
  let completed = false;
  for (const controller of Array.from(activeRevealControllers)) {
    if (visibleRevealController(controller) && controller.complete()) completed = true;
  }
  return completed;
}

function consumeKeyboardEvent(event) {
  event.preventDefault();
  event.stopImmediatePropagation();
}

function consumeRevealKeydown(event) {
  const acknowledgesCommandScreen = commandExecution?.phase === 'rejected' || commandOutput !== null;
  if (acknowledgesCommandScreen &&
      (event.key === 'Enter' || event.key === 'Escape' || event.key === 'Backspace')) {
    return;
  }
  const key = revealPhysicalKey(event);
  if (consumedRevealKey !== null && event.repeat && key === consumedRevealKey) {
    consumeKeyboardEvent(event);
    return;
  }
  if (!completeVisibleReveals()) return;
  consumedRevealKey = key;
  consumeKeyboardEvent(event);
}

function releaseConsumedRevealKey(event) {
  if (revealPhysicalKey(event) === consumedRevealKey) consumedRevealKey = null;
}

document.addEventListener('keydown', consumeRevealKeydown, { capture: true });
document.addEventListener('keyup', releaseConsumedRevealKey, { capture: true });

document.addEventListener('keydown', (e) => {
  if (!hasLive) return;

  if (!canControlTerminalPresentation()) {
    if (['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight', 'PageUp', 'PageDown', 'Home', 'End', 'Enter', 'Escape', 'Backspace'].includes(e.key) || e.key.length === 1) {
      e.preventDefault();
    }
    return;
  }

  if (mode === MODE.HACK) {
    if (!hack || hack.solved || hack.failed) return;
    if (e.key === 'Enter') {
      hackTyped = '';
      renderHackInputPreview();
      e.preventDefault();
    } else if (e.key === 'Backspace') {
      hackTyped = hackTyped.slice(0, -1);
      renderHackInputPreview();
      e.preventDefault();
    } else if (e.key === 'Escape') {
      hackTyped = '';
      renderHackInputPreview();
      e.preventDefault();
    } else if (e.key.length === 1 && hackTyped.length < 24) {
      hackTyped += e.key;
      renderHackInputPreview();
      e.preventDefault();
    }
    return;
  }

  if (commandExecution?.phase === 'pending' || isTerminalNavigationPending()) {
    if (e.key === 'Enter' || e.key === 'Escape' || e.key === 'Backspace') {
      e.preventDefault();
    }
    return;
  }

  if ((commandExecution?.phase === 'rejected' || commandOutput !== null) &&
      (e.key === 'Enter' || e.key === 'Escape' || e.key === 'Backspace')) {
    goBack();
    e.preventDefault();
    return;
  }

  if (mode === MODE.ENTRY) {
    if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
      changePage(-1);
      e.preventDefault();
    } else if (e.key === 'ArrowRight' || e.key === 'PageDown') {
      changePage(1);
      e.preventDefault();
    } else if (e.key === 'Home') {
      changePage(-pagedView.index);
      e.preventDefault();
    } else if (e.key === 'End') {
      changePage(pagedView.pages.length - pagedView.index - 1);
      e.preventDefault();
    } else if (e.key === 'Escape' || e.key === 'Backspace') {
      goBack();
      e.preventDefault();
    }
    return;
  }

  if (pagedView.kind === 'command') {
    if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
      changePage(-1);
      e.preventDefault();
      return;
    }
    if (e.key === 'ArrowRight' || e.key === 'PageDown') {
      changePage(1);
      e.preventDefault();
      return;
    }
  }

  const kids = (currentFolderNode().children || []);
  if (e.key === 'ArrowDown') {
    if (kids.length) {
      const next = Math.min(kids.length - 1, selIndex + 1);
      if (next !== selIndex) beginControllerPresentation({ kind: 'menu', targetId: kids[next].id });
    }
    e.preventDefault();
  } else if (e.key === 'ArrowUp') {
    if (kids.length) {
      const next = Math.max(0, selIndex - 1);
      if (next !== selIndex) beginControllerPresentation({ kind: 'menu', targetId: kids[next].id });
    }
    e.preventDefault();
  } else if (e.key === 'Enter') {
    if (kids[selIndex]) activateRow(kids[selIndex]);
    e.preventDefault();
  } else if (e.key === 'Escape' || e.key === 'Backspace') {
    goBack();
    e.preventDefault();
  }
});

// ════════════════════════════════════════════════════
// RENDER
// ════════════════════════════════════════════════════
function esc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

// ── Typewriter reveal: append elements one at a time (fast), with a
// charscroll sound per new line. `animate=false` just shows everything
// instantly — used when re-rendering content that hasn't actually changed.
const REVEAL_DELAY_MS = 40;

function revealInto(container, elements, animate, contentIdentity = '', options = {}) {
  const currentController = container._revealController;
  if (!animate && currentController?.state === 'revealing' &&
      currentController.contentIdentity === contentIdentity) {
    return currentController;
  }
  if (!animate && container._revealedContentIdentity === contentIdentity) return null;
  cancelReveal(container);
  container._revealGeneration = (container._revealGeneration || 0) + 1;
  const generation = container._revealGeneration;
  container._revealedContentIdentity = null;
  container.replaceChildren();
  const appendElement = options.appendElement || (element => container.appendChild(element));
  const afterAppend = options.afterAppend || (() => {});
  if (options.prepareContainer) options.prepareContainer(container);
  const append = element => {
    appendElement(element);
    afterAppend(element);
  };
  if (!animate) {
    elements.forEach(append);
    container._revealedContentIdentity = contentIdentity;
    return null;
  }

  const controller = {
    container,
    elements,
    contentIdentity,
    generation,
    nextIndex: 0,
    startedAt: performance.now(),
    timer: null,
    state: 'revealing',
    complete: null,
    cancel: null,
  };

  function settle(state) {
    if (controller.timer !== null) clearTimeout(controller.timer);
    controller.timer = null;
    controller.state = state;
    activeRevealControllers.delete(controller);
    if (container._revealController === controller) container._revealController = null;
    if (state === 'complete') container._revealedContentIdentity = contentIdentity;
    if (state === 'complete' && pagedView.container === container) scheduleRepagination();
  }

  controller.complete = () => {
    if (controller.state !== 'revealing' || container._revealGeneration !== generation) return false;
    while (controller.nextIndex < elements.length) {
      append(elements[controller.nextIndex]);
      controller.nextIndex += 1;
    }
    settle('complete');
    return true;
  };

  controller.cancel = () => {
    if (controller.state !== 'revealing') return false;
    if (container._revealGeneration === generation) container._revealGeneration += 1;
    settle('cancelled');
    return true;
  };

  container._revealController = controller;
  activeRevealControllers.add(controller);

  function next() {
    if (controller.state !== 'revealing' || container._revealGeneration !== generation) return;
    if (controller.nextIndex >= elements.length) {
      settle('complete');
      return;
    }
    append(elements[controller.nextIndex]);
    playCharScroll();
    controller.nextIndex += 1;
    if (controller.nextIndex < elements.length) {
      const targetTime = controller.startedAt + controller.nextIndex * REVEAL_DELAY_MS;
      controller.timer = setTimeout(next, Math.max(0, targetTime - performance.now()));
    } else {
      settle('complete');
    }
  }
  next();
  return controller;
}

function cancelReveal(container) {
  const controller = container?._revealController;
  if (!controller) return false;
  return controller.cancel();
}

function lineToDiv(text) {
  const d = document.createElement('div');
  d.textContent = text;
  return d;
}

function revealContentIdentity(kind, key, text, detail = '') {
  return `${kind}:${JSON.stringify([key, String(text == null ? '' : text), detail])}`;
}

function replaceWithText(container, text) {
  cancelReveal(container);
  container._revealedContentIdentity = null;
  container.replaceChildren();
  String(text).split('\n').forEach(line => container.appendChild(lineToDiv(line)));
}

function textFits(container, text) {
  replaceWithText(container, text);
  return container.scrollHeight <= container.clientHeight + 1 &&
    container.scrollWidth <= container.clientWidth + 1;
}

function naturalPageBreak(text, start, fittedEnd) {
  if (fittedEnd >= text.length) return text.length;

  const minimumBreak = start + Math.floor((fittedEnd - start) * .6);
  for (let i = fittedEnd; i > minimumBreak; i--) {
    if (/\s/.test(text[i - 1])) return i;
  }
  return fittedEnd;
}

function paginateText(container, text) {
  const source = String(text == null ? '' : text).replace(/\r\n?/g, '\n');
  if (!source) return [''];
  if (container.clientHeight <= 0 || container.clientWidth <= 0) return [source];

  const pages = [];
  let start = 0;
  while (start < source.length) {
    let low = start + 1;
    let high = source.length;
    let fittedEnd = start;

    while (low <= high) {
      const midpoint = Math.floor((low + high) / 2);
      if (textFits(container, source.slice(start, midpoint))) {
        fittedEnd = midpoint;
        low = midpoint + 1;
      } else {
        high = midpoint - 1;
      }
    }

    if (fittedEnd === start) fittedEnd = start + 1;
    const end = naturalPageBreak(source, start, fittedEnd);
    pages.push(source.slice(start, end));
    start = end;
  }
  return pages;
}

function updatePageControls() {
  if (pagedView.kind === null) {
    pageNav.hidden = true;
    return;
  }

  pageNav.hidden = false;
  pagePrev.hidden = pagedView.index === 0;
  pageNext.hidden = pagedView.index >= pagedView.pages.length - 1;
  pageIndicator.value = `${pagedView.index + 1} / ${pagedView.pages.length}`;
  pageIndicator.textContent = pageIndicator.value;
}

function renderPagedView(animate) {
  if (!pagedView.container) return;
  const page = pagedView.pages[pagedView.index] || '';
  const lines = page.split('\n').map(lineToDiv);
  const identity = revealContentIdentity(pagedView.kind, pagedView.key, page, pagedView.index);
  revealInto(pagedView.container, lines, animate, identity);
  updatePageControls();
}

function recalculatePagination(resetPage, animate) {
  if (pagedView.kind === null || !pagedView.container) return;
  const previousIndex = resetPage ? 0 : pagedView.index;
  pagedView.pages = paginateText(pagedView.container, pagedView.text);
  const presentation = effectiveControllerPresentation();
  const authoritativeIndex = presentation.kind === 'page'
    ? presentation.pageIndex
    : previousIndex;
  pagedView.index = Math.min(authoritativeIndex, pagedView.pages.length - 1);
  renderPagedView(animate && pagedView.index === 0);
}

function activatePagination(kind, key, text, container, animate) {
  const source = String(text == null ? '' : text);
  const identityChanged = pagedView.kind !== kind || pagedView.key !== key;
  const contentChanged = pagedView.text !== source || pagedView.container !== container;

  pagedView.kind = kind;
  pagedView.key = key;
  pagedView.text = source;
  pagedView.container = container;

  if (identityChanged || contentChanged) {
    recalculatePagination(identityChanged, animate);
  } else {
    const presentation = effectiveControllerPresentation();
    if (presentation.kind === 'page') {
      pagedView.index = Math.min(presentation.pageIndex, pagedView.pages.length - 1);
    }
    renderPagedView(false);
  }
}

function deactivatePagination() {
  if (paginationFrame !== null) {
    cancelAnimationFrame(paginationFrame);
    paginationFrame = null;
  }
  cancelReveal(pagedView.container);
  pagedView = {
    kind: null,
    key: null,
    text: '',
    container: null,
    pages: [''],
    index: 0,
  };
  updatePageControls();
}

function changePage(delta) {
  if (pagedView.kind === null || !delta || !canControlTerminalPresentation()) return;
  const nextIndex = Math.max(0, Math.min(pagedView.pages.length - 1, pagedView.index + delta));
  if (nextIndex === pagedView.index) return;
  beginControllerPresentation({ kind: 'page', pageIndex: nextIndex });
}

function scheduleRepagination() {
  if (pagedView.kind === null) return;
  if (pagedView.container?._revealController?.state === 'revealing') return;
  if (paginationFrame !== null) cancelAnimationFrame(paginationFrame);
  paginationFrame = requestAnimationFrame(() => {
    paginationFrame = null;
    recalculatePagination(false, false);
  });
}

function regionOverflows(region) {
  return region.scrollHeight > region.clientHeight + 1 ||
    region.scrollWidth > region.clientWidth + 1;
}

function regionContains(parent, child) {
  const tolerance = 1;
  const parentBounds = parent.getBoundingClientRect();
  const childBounds = child.getBoundingClientRect();
  return childBounds.top >= parentBounds.top - tolerance &&
    childBounds.left >= parentBounds.left - tolerance &&
    childBounds.right <= parentBounds.right + tolerance &&
    childBounds.bottom <= parentBounds.bottom + tolerance;
}

function hackLayoutParts(board = hackBoard) {
  const columnsContainer = board.querySelector('.hack-columns');
  const log = board.querySelector('.hack-log');
  const logPanel = board.querySelector('.hack-log-panel');
  const inputLine = board.querySelector('.hack-input-line');
  return { columnsContainer, log, logPanel, inputLine };
}

function hackContentOverflows(board = hackBoard) {
  const { columnsContainer, log, logPanel, inputLine } = hackLayoutParts(board);
  const columns = Array.from(columnsContainer.children);
  const rows = Array.from(columnsContainer.querySelectorAll('.hack-row'));
  const logLines = Array.from(log.children);
  const regions = [board, columnsContainer, logPanel, log, inputLine, ...columns, ...rows, ...logLines];
  if (regions.some(regionOverflows)) return true;

  const containedRegions = [
    [board, columnsContainer],
    [board, logPanel],
    [logPanel, log],
    [logPanel, inputLine],
    ...columns.map(column => [columnsContainer, column]),
    ...rows.map(row => [columnsContainer, row]),
    ...logLines.map(line => [log, line]),
  ];
  if (board === hackBoard) containedRegions.push([screen, hackHeader], [screen, hackBoard]);
  return containedRegions.some(([parent, child]) => !regionContains(parent, child));
}

function hackRowsFitColumns(board = hackBoard) {
  const { columnsContainer } = hackLayoutParts(board);
  const tolerance = 0.5;
  return Array.from(columnsContainer.children).every(column => {
    const columnBounds = column.getBoundingClientRect();
    return Array.from(column.querySelectorAll('.hack-row')).every(row => {
      const addressBounds = row.querySelector('.hack-addr').getBoundingClientRect();
      const cells = row.querySelectorAll('.hcell');
      const finalBounds = cells.length
        ? cells[cells.length - 1].getBoundingClientRect()
        : row.querySelector('.hack-cells').getBoundingClientRect();
      const rowBounds = row.getBoundingClientRect();
      return addressBounds.left >= columnBounds.left - tolerance &&
        finalBounds.right <= columnBounds.right + tolerance &&
        rowBounds.top >= columnBounds.top - tolerance &&
        rowBounds.bottom <= columnBounds.bottom + tolerance;
    });
  });
}

function fitHackRowFont(board = hackBoard) {
  board.style.removeProperty('--hack-row-font');
  const { columnsContainer } = hackLayoutParts(board);
  const rows = Array.from(columnsContainer.querySelectorAll('.hack-row'));
  const columns = Array.from(columnsContainer.children);
  if (!rows.length || !columns.length) return null;

  const baseSize = Number.parseFloat(getComputedStyle(board).fontSize);
  if (!Number.isFinite(baseSize) || baseSize <= 0) return null;

  const applySize = size => board.style.setProperty('--hack-row-font', `${size}px`);
  const fitsAt = size => {
    applySize(size);
    return hackRowsFitColumns(board) && !hackContentOverflows(board);
  };

  if (!fitsAt(baseSize)) {
    applySize(baseSize);
    return baseSize;
  }

  let low = baseSize;
  const narrowerColumnWidth = Math.min(...columns.map(column => column.getBoundingClientRect().width));
  let high = Math.max(baseSize * 2, narrowerColumnWidth);
  for (let attempt = 0; attempt < 8 && fitsAt(high); attempt++) {
    low = high;
    high *= 2;
  }

  while (high - low > 0.25) {
    const candidate = (low + high) / 2;
    if (fitsAt(candidate)) low = candidate;
    else high = candidate;
  }
  applySize(low);
  return low;
}

function applyHackLayout(board) {
  board.style.removeProperty('--hack-row-font');
  board.classList.remove('hack-compact', 'hack-stacked', 'hack-tight');
  const preferStacked = board.clientWidth <= 700 || board.clientHeight <= 300;
  board.classList.toggle('hack-stacked', preferStacked);
  board.classList.toggle('hack-compact', preferStacked || hackContentOverflows(board));

  if (!preferStacked && hackContentOverflows(board)) {
    board.classList.add('hack-compact', 'hack-stacked');
  }
  if (hackContentOverflows(board)) {
    board.classList.add('hack-tight');
  }
  const fontSize = fitHackRowFont(board);
  return {
    fontSize,
    compact: board.classList.contains('hack-compact'),
    stacked: board.classList.contains('hack-stacked'),
    tight: board.classList.contains('hack-tight'),
  };
}

function applyHackFit(fit) {
  hackBoard.classList.toggle('hack-compact', Boolean(fit?.compact));
  hackBoard.classList.toggle('hack-stacked', Boolean(fit?.stacked));
  hackBoard.classList.toggle('hack-tight', Boolean(fit?.tight));
  if (Number.isFinite(fit?.fontSize)) {
    hackBoard.style.setProperty('--hack-row-font', `${fit.fontSize}px`);
  } else {
    hackBoard.style.removeProperty('--hack-row-font');
  }
}

function createHackFitProbe() {
  if (lastRenderedHackRows.size === 0) return null;
  const bounds = hackBoard.getBoundingClientRect();
  if (bounds.width <= 0 || bounds.height <= 0) return null;

  const probe = hackBoard.cloneNode(true);
  probe.hidden = false;
  probe.inert = true;
  probe.setAttribute('aria-hidden', 'true');
  probe.removeAttribute('id');
  probe.querySelectorAll('[id]').forEach(element => element.removeAttribute('id'));
  const probeColumns = probe.querySelector('.hack-columns');
  probeColumns.replaceChildren();

  const clonedColumns = new Map();
  for (const descriptor of lastRenderedHackRows.values()) {
    let probeColumn = clonedColumns.get(descriptor.parent);
    if (!probeColumn) {
      probeColumn = descriptor.parent.cloneNode(false);
      clonedColumns.set(descriptor.parent, probeColumn);
      probeColumns.appendChild(probeColumn);
    }
    probeColumn.appendChild(descriptor.row.cloneNode(true));
  }

  Object.assign(probe.style, {
    position: 'fixed',
    left: '-10000px',
    top: '0',
    width: `${bounds.width}px`,
    height: `${bounds.height}px`,
    margin: '0',
    visibility: 'hidden',
    pointerEvents: 'none',
    zIndex: '-1',
  });
  document.body.appendChild(probe);
  return probe;
}

function fitCompleteHackBoard() {
  const probe = createHackFitProbe();
  if (!probe) return null;
  try {
    const fit = applyHackLayout(probe);
    hackBoardFit = fit;
    applyHackFit(hackBoardFit);
    return fit;
  } finally {
    probe.remove();
  }
}

function fitHackLayout() {
  hackFitFrame = null;
  if (mode !== MODE.HACK || hackBoard.hidden) {
    hackBoardFit = null;
    applyHackFit(null);
    return;
  }
  fitCompleteHackBoard();
}

function scheduleHackFit() {
  if (hackFitFrame !== null) cancelAnimationFrame(hackFitFrame);
  hackFitFrame = requestAnimationFrame(fitHackLayout);
}

function render() {
  if (renderPlayerContext()) {
    hideTerminalSurface();
    return;
  }

  if (!hasCurrentTerminalMirror()) {
    deactivatePagination();
    cancelReveal(termList);
    normalHeader.hidden = true;
    hackHeader.hidden   = true;
    termIdle.hidden     = false;
    termList.hidden     = true;
    termEntry.hidden    = true;
    hackBoard.hidden    = true;
    hackBlocked.hidden  = true;
    termOutput.hidden   = true;
    termPrompt.hidden   = true;
    backBtn.hidden      = true;
    return;
  }

  termIdle.hidden = true;

  if (commandExecution !== null) {
    renderCommandExecutionScreen();
  } else if (isTerminalNavigationPending()) {
    renderTerminalNavigationPendingScreen();
  } else if (mode === MODE.HACK) {
    renderHackScreen();
  } else {
    renderNormalScreen();
  }
}

function renderCommandExecutionScreen() {
  hackHeader.hidden = true;
  hackBoard.hidden = true;
  hackBlocked.hidden = true;
  normalHeader.hidden = false;
  serverLine.textContent = `-Server ${serverNum}-`;
  introTextEl.textContent = introText;
  const commandID = commandExecution.commandNodeId;
  const command = findNodeById(tree, commandID);
  const phase = commandExecution.phase;
  const text = phase === 'pending'
    ? 'Выполняется запрос'
    : 'Ошибка доступа';
  renderCommandRecordSurface({
    kind: `command-${phase}`,
    key: `${phase}:${commandID}`,
    title: command ? command.name : '',
    text,
    showBack: phase === 'rejected' &&
      playerState?.role === 'active' && playerState?.phase === 'controlling',
  });
}

function renderTerminalNavigationPendingScreen() {
  hackHeader.hidden = true;
  hackBoard.hidden = true;
  hackBlocked.hidden = true;
  normalHeader.hidden = false;
  serverLine.textContent = `-Server ${serverNum}-`;
  introTextEl.textContent = introText;
  const pending = terminalNavigation.pending;
  const target = pending.targetTerminalName || pending.targetTerminalId;
  renderCommandRecordSurface({
    kind: 'terminal-navigation-pending',
    key: `${pending.direction}:${pending.targetTerminalId}`,
    title: `${pending.direction === 'return' ? 'ВОЗВРАТ' : 'ПЕРЕХОД'} В ${target}`,
    text: 'Выполняется запрос',
    showBack: false,
  });
}

function resetBackControl() {
  backBtn.classList.remove('terminal-return');
  backBtn.textContent = '[ НАЗАД ]';
  backBtn.removeAttribute('aria-label');
}

function renderCommandRecordSurface({ kind, key, title, text, showBack }) {
  cancelReveal(termList);
  cancelReveal(termOutput);
  termList.hidden = true;
  termEntry.hidden = false;
  termOutput.hidden = true;
  termOutput.classList.remove('command-screen', 'command-execution-status', 'command-result-screen');
  termPrompt.hidden = false;
  backBtn.hidden = false;
  resetBackControl();
  backBtn.classList.toggle('layout-placeholder', !showBack);
  backBtn.disabled = !showBack;
  backBtn.setAttribute('aria-hidden', String(!showBack));
  entryTitle.textContent = title;
  lastRenderedEntryId = null;
  lastRenderedFolderKey = null;
  lastMenuHoverIdx = null;

  const commandKey = revealContentIdentity(kind, key, text);
  const isNewCommand = commandKey !== lastRenderedCommandKey;
  lastRenderedCommandKey = commandKey;
  activatePagination(kind, key, text, entryBody, isNewCommand);
}

function renderNormalScreen() {
  termOutput.classList.remove('command-screen', 'command-execution-status', 'command-result-screen');
  backBtn.classList.remove('layout-placeholder');
  resetBackControl();
  backBtn.disabled = false;
  backBtn.removeAttribute('aria-hidden');
  cancelReveal(hackColumns);
  hackColumns._revealedContentIdentity = null;
  hackColumns.replaceChildren();
  lastRenderedHackKey = null;
  lastRenderedHackRows = new Map();
  hackHeader.hidden  = true;
  hackBoard.hidden   = true;
  hackBlocked.hidden = true;

  normalHeader.hidden         = false;
  serverLine.textContent     = `-Server ${serverNum}-`;
  introTextEl.textContent    = introText;
  termPrompt.hidden          = false;

  if (mode === MODE.ENTRY) {
    cancelReveal(termList);
    const node = findNodeById(tree, viewEntryId);
    termList.hidden   = true;
    termEntry.hidden  = false;
    termOutput.hidden = true;
    backBtn.hidden    = false;
    entryTitle.textContent  = node ? node.name : '';

    const entryText = node ? (node.description || '') : '';
    const isNewEntry = viewEntryId !== lastRenderedEntryId;
    lastRenderedEntryId = viewEntryId;
    lastRenderedFolderKey = null;
    lastRenderedCommandKey = null;

    activatePagination('entry', viewEntryId, entryText, entryBody, isNewEntry);
    return;
  }

  if (commandOutput !== null) {
    const command = findNodeById(tree, currentCommandNodeId);
    renderCommandRecordSurface({
      kind: 'command',
      key: currentCommandNodeId,
      title: command ? command.name : '',
      text: commandOutput,
      showBack: playerState?.role === 'active' && playerState?.phase === 'controlling',
    });
    return;
  }

  // MODE.LIST
  cancelReveal(entryBody);
  termEntry.hidden = true;
  termList.hidden  = false;
  const atRoot = navStack.length === 1 && navStack[0] === 'root';
  const returnTarget = atRoot ? terminalNavigation?.returnTarget : null;
  backBtn.hidden = atRoot && !returnTarget;
  backBtn.disabled = pendingSharedAction !== null || terminalNavigation?.pending != null;
  if (returnTarget) {
    backBtn.classList.add('terminal-return');
    backBtn.textContent = `[ НАЗАД В ${returnTarget.terminalName || returnTarget.terminalId} ]`;
    backBtn.setAttribute('aria-label', `НАЗАД В ${returnTarget.terminalName || returnTarget.terminalId}`);
  } else {
    backBtn.removeAttribute('aria-label');
  }
  lastRenderedEntryId = null;
  lastMenuHoverIdx = null;

  const folder = currentFolderNode();
  const kids = folder.children || [];

  const folderPath = navStack.join('/');
  const folderText = JSON.stringify(kids.map(node => [node.id, node.type, node.name, node.available === false]));
  const folderKey = revealContentIdentity('folder', folderPath, folderText);
  const isNewFolder = folderKey !== lastRenderedFolderKey;
  lastRenderedFolderKey = folderKey;

  let rows;
  if (!kids.length) {
    const empty = document.createElement('div');
    empty.className = 'term-empty';
    empty.textContent = '[ ДИРЕКТОРИЯ ПУСТА ]';
    rows = [empty];
  } else {
    rows = kids.map((node, i) => {
      const row = document.createElement('div');
      row.className = 'term-row' + (i === selIndex ? ' sel' : '');
      row.dataset.idx = String(i);
      row.dataset.nodeId = node.id;
      row.textContent = '> ' + node.name;
      if (node.type === 'command' && node.available === false) {
        row.setAttribute('aria-disabled', 'true');
      }
      row.addEventListener('click', () => {
        activateRow(node);
      });
      return row;
    });
  }
  revealInto(termList, rows, isNewFolder, `${folderKey}:selection:${selIndex}`);

  termOutput.hidden = true;
  lastRenderedCommandKey = null;
  deactivatePagination();
}

// ── Hacking screen ─────────────────────────────────────────
function pluralAttempts(n) {
  const mod10 = n % 10, mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return 'ПОПЫТКА';
  if ([2, 3, 4].includes(mod10) && ![12, 13, 14].includes(mod100)) return 'ПОПЫТКИ';
  return 'ПОПЫТОК';
}

function attemptsLineHtml(h) {
  const squares = Array.from({ length: h.attemptsMax }, (_, i) =>
    `<span class="atsq ${i < h.attemptsLeft ? 'full' : 'empty'}">■</span>`
  ).join(' ');
  return `${h.attemptsLeft} ${pluralAttempts(h.attemptsLeft)} ОСТАЛОСЬ: ${squares}`;
}

function renderHackScreen() {
  deactivatePagination();
  cancelReveal(termList);
  normalHeader.hidden = true;
  termList.hidden     = true;
  termEntry.hidden    = true;
  termOutput.hidden   = true;
  termOutput.classList.remove('command-screen', 'command-execution-status', 'command-result-screen');
  termPrompt.hidden   = true;
  backBtn.hidden      = true;
  hackHeader.hidden   = false;

  if (!hack) {
    cancelReveal(hackColumns);
    lastRenderedHackKey = null;
    lastRenderedHackRows = new Map();
    hackBoardFit = null;
    applyHackFit(null);
    hackBoard.hidden   = true;
    hackBlocked.hidden = true;
    return;
  }

  attemptsLine.innerHTML = attemptsLineHtml(hack);

  if (hack.failed) {
    cancelReveal(hackColumns);
    hackBoardFit = null;
    applyHackFit(null);
    hackBoard.hidden   = true;
    hackBlocked.hidden = false;
    return;
  }

  hackBlocked.hidden = true;
  hackBoard.hidden   = false;
  const hackKey = hackRevealIdentity(hack);
  const isNewHack = hackKey !== lastRenderedHackKey;
  if (isNewHack) hackBoardFit = null;
  lastRenderedHackKey = hackKey;
  renderHackLog();
  renderHackInputPreview();
  renderHackColumns(isNewHack, hackKey);
}

function hackRevealIdentity(hackState) {
  const generationKey = (hackState.patterns || [])
    .map(pattern => String(pattern.id || ''))
    .sort()
    .join('|');
  return revealContentIdentity('hack', generationKey, '');
}

function createHackCell(className, target, text) {
  const cell = document.createElement('span');
  cell.className = `hcell ${className}`;
  cell.dataset.target = String(target);
  cell.tabIndex = 0;
  cell.textContent = text;
  return cell;
}

function buildHackColumn(col, colIndex, rowBase) {
  const wordAt = new Array(col.text.length).fill(null);
  col.words.forEach(w => { for (let i = w.start; i < w.start + w.length; i++) wordAt[i] = w.id; });

  const column = document.createElement('div');
  column.className = 'hack-col';
  const rowDescriptors = [];
  const rowCount = Math.ceil(col.text.length / ROW_WIDTH);
  for (let rowIndex = 0; rowIndex < rowCount; rowIndex++) {
    const rowStart = rowIndex * ROW_WIDTH;
    const rowEnd = Math.min(rowStart + ROW_WIDTH, col.text.length);
    const row = document.createElement('div');
    row.className = 'hack-row';
    const address = document.createElement('span');
    address.className = 'hack-addr';
    address.textContent = col.addresses[rowIndex] || '';
    const cells = document.createElement('span');
    cells.className = 'hack-cells';
    let i = rowStart;
    while (i < rowEnd) {
      const wid = wordAt[i];
      if (wid) {
        let j = i;
        while (j < rowEnd && wordAt[j] === wid) j++;
        cells.appendChild(createHackCell('word', wid, col.text.slice(i, j)));
        i = j;
      } else {
        const cell = createHackCell('filler', `${colIndex}:${i}`, col.text[i]);
        cell.dataset.row = String(rowBase + rowIndex);
        cell.dataset.offset = String(i - rowStart);
        cells.appendChild(cell);
        i++;
      }
    }
    row.append(address, cells);
    const key = `${colIndex}:${rowIndex}`;
    row.dataset.hackRow = key;
    const signature = JSON.stringify(Array.from(row.children, element => [
      element.className,
      element.textContent,
      Array.from(element.children, child => [
        child.className,
        child.dataset.target || '',
        child.dataset.row || '',
        child.dataset.offset || '',
        child.textContent,
      ]),
    ]));
    rowDescriptors.push({ key, parent: column, row, signature });
  }
  return { column, rows: rowDescriptors, rowCount };
}

function buildHackColumns(hackState) {
  const columns = [];
  const rows = [];
  let rowBase = 0;
  for (const [columnIndex, source] of (hackState.columns || []).entries()) {
    const built = buildHackColumn(source, columnIndex, rowBase);
    columns.push(built.column);
    rows.push(...built.rows);
    rowBase += built.rowCount;
  }
  return { columns, rows };
}

function hackBoardSnapshot(hackState) {
  return buildHackColumns(hackState);
}

function reconcileHackRow(current, replacement) {
  const activeCell = current.row.contains(document.activeElement)
    ? {
        target: document.activeElement.dataset.target,
        row: document.activeElement.dataset.row,
        offset: document.activeElement.dataset.offset,
      }
    : null;
  current.row.replaceChildren(...replacement.row.childNodes);
  current.signature = replacement.signature;
  if (!activeCell) return;
  const replacementCell = Array.from(current.row.querySelectorAll('.hcell')).find(cell =>
    cell.dataset.target === activeCell.target &&
    cell.dataset.row === activeCell.row &&
    cell.dataset.offset === activeCell.offset
  );
  replacementCell?.focus({ preventScroll: true });
}

function reconcileHackColumns(hackState) {
  const snapshot = hackBoardSnapshot(hackState);
  for (const replacement of snapshot.rows) {
    const current = lastRenderedHackRows.get(replacement.key);
    if (!current || current.signature === replacement.signature) continue;
    if (current.row.isConnected) {
      reconcileHackRow(current, replacement);
    } else {
      current.row = replacement.row;
      current.signature = replacement.signature;
    }
  }
}

function renderHackColumns(animate, hackKey) {
  if (!animate && lastRenderedHackRows.size !== 0) {
    reconcileHackColumns(hack);
  } else {
    const built = hackBoardSnapshot(hack);
    lastRenderedHackRows = new Map(built.rows.map(descriptor => [descriptor.key, descriptor]));
    fitCompleteHackBoard();
    revealInto(hackColumns, built.rows, animate, hackKey, {
      prepareContainer: container => built.columns.forEach(column => container.appendChild(column)),
      appendElement: descriptor => descriptor.parent.appendChild(descriptor.row),
    });
  }

  if (!hackBoardFit) fitCompleteHackBoard();

  const presentation = effectiveControllerPresentation();
  if (presentation.kind !== 'hacking') {
    setHackHover(null, true);
    return;
  }
  if (presentation.patternId) {
    const pattern = (hack.patterns || []).find(candidate =>
      candidate.id === presentation.patternId && !candidate.used
    );
    setHackPatternHover(pattern || null);
    return;
  }
  const target = presentation.targetId;
  const hoveredCells = hackColumns.querySelectorAll(`[data-target="${cssEscape(target)}"]`);
  setHackHover(hoveredCells.length ? target : null, true);
}

function renderHackLog() {
  const lines = Array.isArray(hack.log) ? hack.log : [];
  hackLog.replaceChildren(...lines.map(lineToDiv));
}

function renderHackInputPreview() {
  hackInputPreview.textContent = hackTyped.length ? hackTyped : hackHoverText;
}

// ════════════════════════════════════════════════════
// BOOT
// ════════════════════════════════════════════════════
window.addEventListener('resize', scheduleRepagination);
window.addEventListener('resize', scheduleHackFit);
window.addEventListener('pagehide', clearTerminalPresentationEffects);
if ('ResizeObserver' in window) {
  const paginationObserver = new ResizeObserver(scheduleRepagination);
  paginationObserver.observe(termBody);
  const hackFitObserver = new ResizeObserver(scheduleHackFit);
  hackFitObserver.observe(termBody);
}
if (document.fonts && document.fonts.ready) {
  document.fonts.ready.then(scheduleRepagination);
  document.fonts.ready.then(scheduleHackFit);
}
render();
void connectPlayer();
