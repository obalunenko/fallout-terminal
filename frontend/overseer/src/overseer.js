'use strict';

const desktopAPI = window.desktopAPI;
const legacyOverseerRoot = document.getElementById('legacyOverseerRoot');
if (!(legacyOverseerRoot instanceof HTMLElement)) {
  throw new Error('Legacy Overseer root is unavailable');
}

function publishLegacyProjection(message) {
  return globalThis.__overseerCoexistenceBridge?.legacyToVue(message) === true;
}

// ── State ─────────────────────────────────────────────────
const state = {
  session:        null,   // { version, name, terminals: [{id,name,hackLevel,introText,root}] }
  filePath:       null,
  liveTerminalId: null,
  editTerminalId: null,
  selectedNodeId: null,
  expanded:       new Set(['root']),
  coordination:   null,   // authoritative roster/session/broadcast projection
};

let idCounter = 0;
function uid(prefix) {
  idCounter++;
  return `${prefix}_${Date.now().toString(36)}_${idCounter}`;
}

// ── DOM refs ──────────────────────────────────────────────
const mainLayout        = legacyOverseerRoot.querySelector('#mainLayout');
const saveStatus        = legacyOverseerRoot.querySelector('#saveStatus');
const btnAddTerminal    = legacyOverseerRoot.querySelector('#btnAddTerminal');
const btnCreateTerminalGroup = legacyOverseerRoot.querySelector('#btnCreateTerminalGroup');
const terminalGroupError = legacyOverseerRoot.querySelector('#terminalGroupError');
let saveGeneration = 0;
let saveInvocation = 0;
let latestRenderedSave = 0;
let newestDurableRevision = 0;
let coordinationCommandPending = false;
let coordinationStatusMessage = '';
let coordinationErrorMessage = '';
let coordinationStatusKind = '';
let createTerminalOpen = false;
let createTerminalPending = false;
let coordinationProjection = null;
let publicAccessSnapshot = null;
let terminalAuthoringProjection = null;
let terminalAuthoringRevision = 0;
let terminalPublishAcknowledgement = 0;
let sessionStateCommandPending = false;
let terminalGroupDraft = null;
let pendingTerminalGroupImpact = null;
let terminalGroupDialogOpener = null;

let resetConfirmationSequence = 0;
let resetConfirmationPending = false;
function confirmCommandStateReset(message) {
  const bridge = globalThis.__overseerCoexistenceBridge;
  if (resetConfirmationPending || !bridge) return Promise.resolve(false);
  resetConfirmationSequence += 1;
  const requestId = `command-state-reset-${resetConfirmationSequence}`;
  resetConfirmationPending = true;
  return new Promise((resolve) => {
    let settled = false;
    const release = bridge.subscribeVueRequests((response) => {
      if (response?.kind !== 'command-state-reset-resolved' || response.requestId !== requestId || settled) return;
      settled = true;
      release();
      resetConfirmationPending = false;
      resolve(response.confirmed === true);
    });
    if (!publishLegacyProjection({ kind: 'command-state-reset-required', message, requestId })) {
      settled = true;
      release();
      resetConfirmationPending = false;
      resolve(false);
    }
  });
}

desktopAPI.onCoordinationState((coordination) => {
  applyCoordinationState(coordination);
  renderCoordination();
  if (state.session) {
    renderTerminalAuthoringProjection();
  }
});
if (typeof desktopAPI.onSessionState === 'function') {
  desktopAPI.onSessionState((event) => {
    const revision = Number(event?.revision);
    if (!event?.session || !Number.isSafeInteger(revision)) return;
    saveStatus.dataset.sessionStateRevision = String(revision);
    updateSessionStateEvidenceDescription();
    if (!state.session || revision <= newestDurableRevision) return;
    state.session = event.session;
    newestDurableRevision = revision;
    saveStatus.textContent = `СОСТОЯНИЕ СЕССИИ ОБНОВЛЕНО · ревизия ${revision}`;
    saveStatus.dataset.savedRevision = String(revision);
    saveStatus.classList.remove('err');
    renderAll();
  });
}
// ── Public access: trusted settings and explicit lifecycle ──
function renderPublicAccess(snapshot) {
  if (!snapshot || typeof snapshot !== 'object') return;
  if (publicAccessSnapshot) {
    const candidateGeneration = Number(snapshot.status?.generation || 0);
    const currentGeneration = Number(publicAccessSnapshot.status?.generation || 0);
    const candidateRevision = Number(snapshot.status?.settingsRevision || snapshot.preferences?.revision || 0);
    const currentRevision = Number(publicAccessSnapshot.status?.settingsRevision || publicAccessSnapshot.preferences?.revision || 0);
    if (candidateGeneration < currentGeneration ||
      (candidateGeneration === currentGeneration && candidateRevision < currentRevision)) return;
  }
  publicAccessSnapshot = snapshot;
  publishLegacyProjection({
    kind: 'public-access-settings-snapshot',
    pending: false,
    snapshot,
  });
}

function showPublicAccessSettings({ setupRequired = false } = {}) {
  publishLegacyProjection({ kind: 'public-access-settings-open', setupRequired });
}

function loadSession(session, filePath, sessionRevision = 0) {
  saveGeneration++;
  saveInvocation = 0;
  latestRenderedSave = 0;
  newestDurableRevision = Number.isSafeInteger(sessionRevision) && sessionRevision >= 0
    ? sessionRevision
    : 0;
  delete saveStatus.dataset.savedRevision;
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  saveStatus.textContent = '';
  saveStatus.classList.remove('err');
  state.session        = session;
  state.filePath        = filePath;
  state.liveTerminalId  = state.coordination?.broadcast?.activeTerminalId || null;
  state.editTerminalId  = (session.terminals[0] && session.terminals[0].id) || null;
  state.selectedNodeId  = null;
  state.expanded         = new Set(['root']);

  mainLayout.style.display  = 'flex';
  renderAll();

  if (session.playerConfig) {
    publishLegacyProjection({ kind: 'player-configuration-load-referenced' });
  } else {
    publishLegacyProjection({ kind: 'player-configuration-missing' });
  }
}

// ── Autosave (writes to the currently open session file) ──
async function autosave() {
  if (!state.filePath) return;
  const generation = saveGeneration;
  const invocation = ++saveInvocation;
  const res = await desktopAPI.saveSession(state.session);

  // A completion from a previously-open session or an older durable revision
  // must never replace status for newer work.
  if (generation !== saveGeneration) return;
  const durableRevision = Number(res.savedRevision || 0);
  if (durableRevision < newestDurableRevision || invocation < latestRenderedSave) return;
  if (!res.ok && invocation < saveInvocation) return;

  latestRenderedSave = invocation;
  newestDurableRevision = Math.max(newestDurableRevision, durableRevision);
  if (res.ok) {
    const revisionLabel = newestDurableRevision > 0
      ? ` · ревизия ${newestDurableRevision}`
      : '';
    saveStatus.textContent = 'Сохранено' + revisionLabel + ' · ' + new Date().toLocaleTimeString();
    saveStatus.dataset.savedRevision = String(newestDurableRevision);
    saveStatus.classList.remove('err');
  } else {
    saveStatus.textContent = 'Ошибка сохранения: ' + (res.error || '');
    saveStatus.dataset.savedRevision = String(newestDurableRevision);
    saveStatus.classList.add('err');
  }
}

// ── Helpers ─────────────────────────────────────────────────
function getEditTerminal() {
  if (!state.session) return null;
  return state.session.terminals.find(t => t.id === state.editTerminalId) || null;
}

function locateNode(root, id) {
  function walk(node, parent) {
    if (node.id === id) return { node, parent };
    if (node.children) {
      for (const child of node.children) {
        const res = walk(child, node);
        if (res) return res;
      }
    }
    return null;
  }
  return walk(root, null);
}

function commandExecutionState(term, commandID) {
  const commandStates = term?.commandStates;
  if (!commandStates || typeof commandStates !== 'object') return null;
  const snapshot = commandStates[commandID];
  return snapshot && typeof snapshot === 'object' ? snapshot : null;
}

function effectiveNodeName(term, node) {
  if (node?.type !== 'command') return node?.name || '';
  const completedName = commandExecutionState(term, node.id)?.completedName;
  return typeof completedName === 'string' && completedName ? completedName : node.name;
}

function renderSessionStateResult(result, successMessage, acceptsCanonicalResult = null) {
  if (!result?.ok || !result.session) {
    saveStatus.textContent = 'Ошибка изменения состояния: ' + (result?.error || 'сессия не обновлена');
    saveStatus.classList.add('err');
    return false;
  }
  if (typeof acceptsCanonicalResult === 'function' && !acceptsCanonicalResult(result)) {
    saveStatus.textContent = 'Ошибка изменения состояния: backend не подтвердил канонический сброс';
    saveStatus.classList.add('err');
    return false;
  }

  state.session = result.session;
  const revision = Number(result.revision || result.savedRevision || 0);
  newestDurableRevision = Math.max(newestDurableRevision, revision);
  saveStatus.textContent = successMessage + (revision > 0 ? ` · ревизия ${revision}` : '');
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  saveStatus.classList.remove('err');
  renderAll();
  return true;
}

function updateSessionStateEvidenceDescription() {
  const evidence = [];
  if (saveStatus.dataset.wailsCommand) {
    evidence.push(`Wails command ${saveStatus.dataset.wailsCommand} ${saveStatus.dataset.wailsResult || 'unknown'}`);
  }
  if (saveStatus.dataset.wailsTerminalId) evidence.push(`terminal ${saveStatus.dataset.wailsTerminalId}`);
  if (saveStatus.dataset.wailsRevision) evidence.push(`document revision ${saveStatus.dataset.wailsRevision}`);
  if (saveStatus.dataset.sessionStateRevision) {
    evidence.push(`session-state revision ${saveStatus.dataset.sessionStateRevision}`);
  }
  if (evidence.length) {
    const accessibleEvidence = evidence.join('; ');
    saveStatus.setAttribute('aria-label', accessibleEvidence);
    saveStatus.setAttribute('aria-description', accessibleEvidence);
  } else {
    saveStatus.removeAttribute('aria-label');
    saveStatus.removeAttribute('aria-description');
  }
}

async function runSessionStateCommand(command, successMessage, acceptsCanonicalResult = null) {
  if (sessionStateCommandPending) return;
  sessionStateCommandPending = true;
  renderTerminalAuthoringProjection();
  try {
    const result = await command();
    renderSessionStateResult(result, successMessage, acceptsCanonicalResult);
  } catch (error) {
    saveStatus.textContent = 'Ошибка изменения состояния: '
      + (error instanceof Error ? error.message : String(error));
    saveStatus.classList.add('err');
  } finally {
    sessionStateCommandPending = false;
    renderTerminalAuthoringProjection();
  }
}

function currentAddTarget() {
  const term = getEditTerminal();
  if (!term) return null;
  if (!state.selectedNodeId) return term.root;
  const loc = locateNode(term.root, state.selectedNodeId);
  if (!loc) return term.root;
  return loc.node.type === 'folder' ? loc.node : (loc.parent || term.root);
}

// ── Render: everything ──────────────────────────────────────
function renderAll() {
  renderTerminalAuthoringProjection();
  renderCoordination();
}

// ── Render: authoritative roster and broadcast state ────────
function renderCoordination() {
  const coordination = state.coordination;
  coordinationProjection = {
    coordination,
    error: coordinationErrorMessage,
    kind: 'coordination-state',
    pending: coordinationCommandPending,
    status: coordinationStatusMessage,
  };
  publishLegacyProjection(coordinationProjection);
}

function playerMutationAllowed() {
  return !coordinationCommandPending &&
    !state.coordination?.broadcast &&
    Boolean(state.coordination?.playerConfig);
}

function setPlayerManagementFeedback(message = '', isError = false) {
  publishLegacyProjection({
    error: isError ? message : '',
    kind: 'player-management-feedback',
    status: isError ? '' : message,
  });
}

function coordinationRevision(coordination) {
  const revision = Number(coordination?.revision || 0);
  return Number.isSafeInteger(revision) && revision >= 0 ? revision : 0;
}

function applyCoordinationState(coordination) {
  if (coordination && state.coordination &&
      coordinationRevision(coordination) <= coordinationRevision(state.coordination)) {
    return false;
  }
  state.coordination = coordination || null;
  state.liveTerminalId = coordination?.broadcast?.activeTerminalId || null;
  syncTerminalNavigationNotice(coordination);
  return true;
}

function syncTerminalNavigationNotice(coordination) {
  const notice = coordination?.terminalNavigationNotice;
  if (!notice) {
    if (coordinationStatusKind === 'terminal-navigation') {
      coordinationStatusKind = '';
      setCoordinationStatus('');
    }
    return;
  }
  const labels = {
    'target-missing': 'ЦЕЛЕВОЙ ТЕРМИНАЛ ПЕРЕХОДА БОЛЬШЕ НЕ СУЩЕСТВУЕТ',
    'self-target': 'КОМАНДА ПЕРЕХОДА НЕ МОЖЕТ ССЫЛАТЬСЯ НА ТЕКУЩИЙ ТЕРМИНАЛ',
    'command-stale': 'КОМАНДА ПЕРЕХОДА БЫЛА ИЗМЕНЕНА ИЛИ УДАЛЕНА',
    'target-changed': 'ЦЕЛЬ КОМАНДЫ ПЕРЕХОДА ИЗМЕНИЛАСЬ',
  };
  const detail = [notice.sourceTerminalId, notice.commandId, notice.targetTerminalId].filter(Boolean).join(' · ');
  coordinationStatusKind = 'terminal-navigation';
  setCoordinationStatus(`${labels[notice.reason] || 'ПЕРЕХОД БОЛЬШЕ НЕ ДЕЙСТВИТЕЛЕН'}${detail ? ` · ${detail}` : ''}`, true);
}

function setCoordinationStatus(message, isError = false) {
  coordinationStatusMessage = isError ? '' : (message || '');
  coordinationErrorMessage = isError ? (message || '') : '';
}

async function runCoordinationCommand(command, successMessage, pendingMessage) {
  if (coordinationCommandPending) return null;
  coordinationCommandPending = true;
  setCoordinationStatus(pendingMessage || 'ВЫПОЛНЕНИЕ ОПЕРАЦИИ...');
  renderCoordination();
  if (state.session) renderTerminalAuthoringProjection();
  let result;
  try {
    result = await command();
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  if (state.session) renderTerminalAuthoringProjection();
  if (!result?.ok) {
    if (result?.state) applyCoordinationState(result.state);
    setCoordinationStatus(result?.error || 'ОПЕРАЦИЯ ОТКЛОНЕНА', true);
    renderCoordination();
    return result;
  }
  if (result.state) applyCoordinationState(result.state);
  setCoordinationStatus(successMessage || 'ОПЕРАЦИЯ ВЫПОЛНЕНА');
  renderCoordination();
  return result;
}

function publishTerminalSwitchRequest(result) {
  const switchId = typeof result?.switchId === 'string' ? result.switchId : '';
  if (!switchId) return;
  publishLegacyProjection({ kind: 'terminal-switch-required', switchId });
}

function dismissTerminalSwitchRequest() {
  publishLegacyProjection({ kind: 'terminal-switch-dismissed' });
}

function applyTerminalSwitchResolution(message) {
  if (message?.kind !== 'terminal-switch-resolved' || !message.result?.ok) return;
  const result = message.result;
  if (result.state) applyCoordinationState(result.state);
  setCoordinationStatus(message.decision === 'cancel' ? 'ПЕРЕКЛЮЧЕНИЕ ОТМЕНЕНО' : 'РЕШЕНИЕ ПРИМЕНЕНО');
  renderAll();
}

function applyHackControlRequest(message) {
  if (message?.kind !== 'hack-command-started' && message?.kind !== 'hack-command-finished') return false;
  const expectedRevision = Number(message.expectedRevision);
  if (!Number.isSafeInteger(expectedRevision)) return true;
  if (message.kind === 'hack-command-started') {
    if (coordinationRevision(state.coordination) !== expectedRevision || coordinationCommandPending) return true;
    coordinationCommandPending = true;
    setCoordinationStatus(typeof message.status === 'string' ? message.status : 'ВЫПОЛНЕНИЕ ОПЕРАЦИИ...');
    renderCoordination();
    return true;
  }
  coordinationCommandPending = false;
  if (coordinationRevision(state.coordination) !== expectedRevision) {
    renderCoordination();
    return true;
  }
  const result = message.result && typeof message.result === 'object' ? message.result : { ok: false };
  if (result.state) applyCoordinationState(result.state);
  const success = typeof message.successMessage === 'string' ? message.successMessage : 'ОПЕРАЦИЯ ВЫПОЛНЕНА';
  setCoordinationStatus(result.ok ? success : (result.error || 'ОПЕРАЦИЯ ВЗЛОМА ОТКЛОНЕНА'), result.ok !== true);
  renderCoordination();
  if (state.session) renderTerminalAuthoringProjection();
  return true;
}

function applyBroadcastControlRequest(message) {
  if (message?.kind === 'logical-session-open-request') {
    publishLegacyProjection({ kind: 'logical-session-open-request' });
    return true;
  }
  if (message?.kind === 'broadcast-end-confirmation-request') {
    return true;
  }
  if (message?.kind === 'broadcast-take-off-confirmation-request') {
    return true;
  }
  const expectedRevision = Number(message?.expectedRevision);
  if (!Number.isSafeInteger(expectedRevision)) return false;
  if (message.kind === 'broadcast-command-started') {
    if (coordinationRevision(state.coordination) !== expectedRevision || coordinationCommandPending) return true;
    coordinationCommandPending = true;
    setCoordinationStatus(typeof message.status === 'string' ? message.status : 'ВЫПОЛНЕНИЕ ОПЕРАЦИИ...');
    renderCoordination();
    return true;
  }
  if (message.kind !== 'broadcast-command-finished') return false;
  coordinationCommandPending = false;
  if (coordinationRevision(state.coordination) !== expectedRevision) {
    renderCoordination();
    return true;
  }
  const result = message.result && typeof message.result === 'object' ? message.result : { ok: false };
  if (result.state) applyCoordinationState(result.state);
  const success = typeof message.successMessage === 'string' ? message.successMessage : 'ОПЕРАЦИЯ ВЫПОЛНЕНА';
  setCoordinationStatus(result.ok ? success : (result.error || 'ОПЕРАЦИЯ ОТКЛОНЕНА'), result.ok !== true);
  renderCoordination();
  if (state.session) renderTerminalAuthoringProjection();
  return true;
}

function applyLogicalSessionRequest(message) {
  const expectedRevision = Number(message?.expectedRevision);
  if (!Number.isSafeInteger(expectedRevision)) return false;
  if (message.kind === 'logical-session-command-started') {
    if (coordinationRevision(state.coordination) !== expectedRevision || coordinationCommandPending) return true;
    coordinationCommandPending = true;
    setCoordinationStatus(typeof message.status === 'string' ? message.status : 'ВЫПОЛНЕНИЕ ОПЕРАЦИИ...');
    renderCoordination();
    return true;
  }
  if (message.kind !== 'logical-session-command-finished') return false;
  coordinationCommandPending = false;
  if (coordinationRevision(state.coordination) !== expectedRevision) {
    renderCoordination();
    return true;
  }
  const result = message.result && typeof message.result === 'object' ? message.result : { ok: false };
  if (result.state) applyCoordinationState(result.state);
  const success = typeof message.successMessage === 'string' ? message.successMessage : 'ОПЕРАЦИЯ ВЫПОЛНЕНА';
  setCoordinationStatus(result.ok ? success : (result.error || 'ОПЕРАЦИЯ ОТКЛОНЕНА'), result.ok !== true);
  renderCoordination();
  return true;
}

function applyPlayerConfigurationRequest(message) {
  if (message?.kind === 'player-management-open-request') {
    publishLegacyProjection({ kind: 'player-management-open-request' });
    return true;
  }
  const expectedRevision = Number(message?.expectedRevision);
  if (!Number.isSafeInteger(expectedRevision)) return false;
  if (message.kind === 'player-configuration-command-started') {
    if (coordinationRevision(state.coordination) !== expectedRevision || coordinationCommandPending) return true;
    coordinationCommandPending = true;
    setCoordinationStatus('ЗАГРУЗКА КОНФИГУРАЦИИ ИГРОКОВ...');
    renderCoordination();
    return true;
  }
  if (message.kind !== 'player-configuration-command-finished') return false;
  coordinationCommandPending = false;
  if (coordinationRevision(state.coordination) !== expectedRevision) {
    renderCoordination();
    return true;
  }
  const result = message.result && typeof message.result === 'object' ? message.result : { ok: false };
  if (result.canceled) {
    setCoordinationStatus('ВЫБОР КОНФИГУРАЦИИ ОТМЕНЁН');
    renderCoordination();
    return true;
  }
  const valid = result.ok === true && result.session && typeof result.session === 'object'
    && typeof result.session.playerConfig === 'string' && result.session.playerConfig !== '';
  if (!valid) {
    setCoordinationStatus(result.error || 'НЕ УДАЛОСЬ ЗАГРУЗИТЬ КОНФИГУРАЦИЮ ИГРОКОВ', true);
    renderCoordination();
    return true;
  }
  state.session = result.session;
  if (result.state) applyCoordinationState(result.state);
  setCoordinationStatus(typeof message.successMessage === 'string' ? message.successMessage : 'КОНФИГУРАЦИЯ ИГРОКОВ ОБНОВЛЕНА');
  renderAll();
  return true;
}

function applyPlayerManagementRequest(message) {
  if (message?.kind === 'player-management-closed') return true;
  if (message?.kind === 'player-management-delete-request'
    && typeof message.characterId === 'string' && typeof message.name === 'string') {
    if (playerMutationAllowed()) publishLegacyProjection({
      characterId: message.characterId,
      expectedRevision: coordinationRevision(state.coordination),
      kind: 'player-delete-requested',
      name: message.name,
    });
    return true;
  }
  if (message?.kind === 'player-delete-focus-request' && typeof message.characterId === 'string') {
    publishLegacyProjection({ characterId: message.characterId, kind: 'player-management-delete-focus-request' });
    return true;
  }
  if (message?.kind === 'player-delete-started') {
    const expectedRevision = Number(message.expectedRevision);
    if (Number.isSafeInteger(expectedRevision)
      && coordinationRevision(state.coordination) === expectedRevision && !coordinationCommandPending) {
      coordinationCommandPending = true;
      setCoordinationStatus('УДАЛЕНИЕ ИГРОКА...');
      renderCoordination();
    }
    return true;
  }
  if (message?.kind === 'player-delete-finished') {
    coordinationCommandPending = false;
    const result = message.result && typeof message.result === 'object' ? message.result : { ok: false };
    if (result.state) applyCoordinationState(result.state);
    const feedback = result.ok ? 'ИГРОК УДАЛЁН' : (result.error || 'НЕ УДАЛОСЬ УДАЛИТЬ ИГРОКА');
    setPlayerManagementFeedback(feedback, result.ok !== true);
    setCoordinationStatus(feedback, result.ok !== true);
    renderCoordination();
    return true;
  }
  const expectedRevision = Number(message?.expectedRevision);
  if (!Number.isSafeInteger(expectedRevision)) return false;
  if (message.kind === 'player-management-command-started') {
    if (coordinationRevision(state.coordination) !== expectedRevision || coordinationCommandPending) return true;
    coordinationCommandPending = true;
    const pendingMessage = typeof message.status === 'string' ? message.status : 'ОПЕРАЦИЯ СО СПИСКОМ ИГРОКОВ...';
    setCoordinationStatus(pendingMessage);
    renderCoordination();
    return true;
  }
  if (message.kind !== 'player-management-command-finished') return false;
  coordinationCommandPending = false;
  if (coordinationRevision(state.coordination) !== expectedRevision) {
    renderCoordination();
    return true;
  }
  const result = message.result && typeof message.result === 'object' ? message.result : { ok: false };
  if (result.state) applyCoordinationState(result.state);
  const success = typeof message.successMessage === 'string' ? message.successMessage : 'ОПЕРАЦИЯ ВЫПОЛНЕНА';
  setCoordinationStatus(result.ok ? success : (result.error || 'ОПЕРАЦИЯ СО СПИСКОМ ИГРОКОВ ОТКЛОНЕНА'), result.ok !== true);
  renderCoordination();
  return true;
}

function applyTerminalGroupRequest(message) {
  if (message?.kind === 'terminal-group-action-request') {
    if (message.revision !== terminalAuthoringRevision || typeof message.groupID !== 'string') return true;
    const groupIndex = state.session?.terminalGroups.findIndex(group => group.id === message.groupID) ?? -1;
    if (groupIndex < 0) return true;
    terminalGroupDialogOpener = { scope: 'terminal-group', ownerID: message.groupID };
    if (message.action === 'rename-terminal-group') showTerminalGroupRename(message.groupID);
    else if (message.action === 'move-terminal-group-up' && groupIndex > 0) prepareTerminalGroupOrder(message.groupID, -1);
    else if (message.action === 'move-terminal-group-down'
      && groupIndex < state.session.terminalGroups.length - 1) prepareTerminalGroupOrder(message.groupID, 1);
    else if (message.action === 'dissolve-terminal-group') prepareTerminalGroupDissolution(message.groupID);
    if (!terminalGroupDraft && !pendingTerminalGroupImpact) terminalGroupDialogOpener = null;
    return true;
  }
  if (message?.kind === 'terminal-group-draft-reviewed') {
    reviewTerminalGroupDraft(message);
    return true;
  }
  if (message?.kind === 'terminal-group-rename-requested') {
    saveTerminalGroupRename(message);
    return true;
  }
  if (message?.kind === 'terminal-group-draft-closed') {
    closeTerminalGroupDraft({ publish: false });
    return true;
  }
  if (message?.kind === 'terminal-group-impact-closed') {
    closeTerminalGroupImpact({ publish: false });
    return true;
  }
  if (message?.kind === 'terminal-group-impact-amend-requested') {
    amendRejectedTerminalGroupImpact();
    return true;
  }
  if (message?.kind !== 'terminal-group-command-finished') return false;
  applyTerminalGroupCommandResult(message);
  return true;
}

function applyPublicAccessRequest(message) {
  if (message?.kind === 'public-access-snapshot') {
    renderPublicAccess(message.snapshot);
    return true;
  }
  if (message?.kind === 'public-access-settings-open') {
    showPublicAccessSettings({ setupRequired: message.setupRequired === true });
    return true;
  }
  if (message?.kind === 'public-access-settings-command-finished') {
    renderPublicAccess(message.result?.snapshot || publicAccessSnapshot);
    return true;
  }
  if (message?.kind === 'public-access-provider-token-open') {
    publishLegacyProjection({ kind: 'public-access-provider-token-open' });
    return true;
  }
  if (message?.kind === 'public-access-player-credentials-open') {
    publishLegacyProjection({ kind: 'public-access-player-credentials-open' });
    return true;
  }
  if (message?.kind === 'public-access-credentials-share') {
    publishLegacyProjection({ kind: 'public-access-credentials-share' });
    return true;
  }
  if (message?.kind === 'public-access-generated-password-open'
    && typeof message.generatedPassword === 'string' && message.generatedPassword !== '') {
    publishLegacyProjection({
      generatedPassword: message.generatedPassword,
      kind: 'public-access-generated-password-open',
    });
    return true;
  }
  return false;
}

async function runTerminalEditorAction(message, term) {
  if (message.action === 'apply-settings') {
    const hackLevel = Number(message.hackLevel);
    if (!Number.isInteger(hackLevel) || hackLevel < 0 || hackLevel > 5
      || typeof message.introText !== 'string') return;
    term.hackLevel = hackLevel;
    term.introText = message.introText;
    await autosave();
    if (term.id === state.liveTerminalId) {
      await runCoordinationCommand(
        () => desktopAPI.updateLiveTerminal({ tree: term.root, introText: term.introText }),
        'АКТИВНЫЙ ТЕРМИНАЛ ОБНОВЛЁН',
        'ОБНОВЛЕНИЕ АКТИВНОГО ТЕРМИНАЛА...'
      );
    }
    renderAll();
    return;
  }
  if (message.action === 'make-live') {
    if (term.id === state.liveTerminalId || !state.coordination?.broadcast || coordinationCommandPending) return;
    const result = await runTerminalSwitchRequest(
      () => terminalActivationRequest(term),
      'АКТИВНЫЙ ТЕРМИНАЛ ВЫБРАН',
      'ПЕРЕКЛЮЧЕНИЕ АКТИВНОГО ТЕРМИНАЛА...'
    );
    renderAll();
    return;
  }
  if (message.action === 'reapply-settings') {
    if (term.id !== state.liveTerminalId || !state.coordination?.broadcast || coordinationCommandPending) return;
    const result = await runTerminalSwitchRequest(
      () => terminalActivationRequest(term),
      'НАСТРОЙКИ АКТИВНОГО ТЕРМИНАЛА ПЕРЕПРИМЕНЕНЫ',
      'ПЕРЕПРИМЕНЕНИЕ НАСТРОЕК...'
    );
    renderAll();
    return;
  }
  if (message.action === 'publish') {
    if (term.id !== state.liveTerminalId || coordinationCommandPending) return;
    const result = await runTerminalSwitchRequest(
      () => desktopAPI.updateLiveTerminal({ tree: term.root, introText: term.introText || '' }),
      'ИЗМЕНЕНИЯ ОПУБЛИКОВАНЫ У ИГРОКОВ',
      'ПУБЛИКАЦИЯ ИЗМЕНЕНИЙ...'
    );
    if (result?.ok) terminalPublishAcknowledgement += 1;
    renderAll();
    return;
  }
  if (message.action !== 'reset-command-states' || sessionStateCommandPending) return;
  if (!await confirmCommandStateReset(`Сбросить все выполненные состояния команд терминала "${term.name}"?`)) return;
  const revisionBeforeReset = newestDurableRevision;
  await runSessionStateCommand(
    async () => {
      const result = await desktopAPI.resetTerminalCommandStates({ terminalId: term.id });
      saveStatus.dataset.wailsCommand = 'ResetTerminalCommandStates';
      saveStatus.dataset.wailsResult = result?.ok ? 'ok' : 'error';
      saveStatus.dataset.wailsTerminalId = term.id;
      saveStatus.dataset.wailsRevision = String(Number(result?.revision || 0));
      updateSessionStateEvidenceDescription();
      return result;
    },
    'СОСТОЯНИЯ КОМАНД ТЕРМИНАЛА СБРОШЕНЫ',
    (result) => {
      const revision = Number(result?.revision || 0);
      const canonicalTerminal = result?.session?.terminals?.find(candidate => candidate.id === term.id);
      return revision > revisionBeforeReset && canonicalTerminal
        && Object.keys(canonicalTerminal.commandStates ?? {}).length === 0;
    }
  );
}

function applyTerminalEditorRequest(message) {
  if (message?.kind !== 'terminal-editor-action-request') return false;
  if (message.revision !== terminalAuthoringRevision || typeof message.terminalID !== 'string') return true;
  const term = state.session?.terminals.find(candidate => candidate.id === message.terminalID);
  if (!term || term.id !== state.editTerminalId) return true;
  void runTerminalEditorAction(message, term);
  return true;
}

function applyVueRequest(message) {
  if (message?.kind === 'session-document-loaded' && message.session && typeof message.session === 'object') {
    loadSession(
      message.session,
      typeof message.filePath === 'string' ? message.filePath : '',
      Number(message.sessionRevision),
    );
    return;
  }
  if (applyCreateTerminalRequest(message)) return;
  if (applyTerminalEditorRequest(message)) return;
  if (applyTerminalTreeRequest(message)) return;
  if (applyTerminalSelectionRequest(message)) return;
  if (applyHackControlRequest(message)) return;
  if (applyBroadcastControlRequest(message)) return;
  if (applyPlayerManagementRequest(message)) return;
  if (applyPlayerConfigurationRequest(message)) return;
  if (applyLogicalSessionRequest(message)) return;
  if (applyTerminalGroupRequest(message)) return;
  if (applyPublicAccessRequest(message)) return;
  applyTerminalSwitchResolution(message);
}

let releaseLegacyBridge = null;
function attachOverseerLegacyBridge(bridge) {
  releaseLegacyBridge?.();
  releaseLegacyBridge = null;
  if (!bridge || typeof bridge.subscribeVueRequests !== 'function') return;
  releaseLegacyBridge = bridge.subscribeVueRequests(applyVueRequest);
  if (coordinationProjection) bridge.legacyToVue(coordinationProjection);
  if (terminalAuthoringProjection) bridge.legacyToVue(terminalAuthoringProjection);
}

Object.defineProperty(globalThis, '__attachOverseerLegacyBridge', {
  configurable: true,
  value: attachOverseerLegacyBridge,
});
attachOverseerLegacyBridge(globalThis.__overseerCoexistenceBridge);

async function runTerminalSwitchRequest(command, completedMessage, pendingMessage) {
  const result = await runCoordinationCommand(command, completedMessage, pendingMessage);
  if (result?.ok && result.status === 'decision-required' && result.switchId) {
    publishTerminalSwitchRequest(result);
  }
  return result;
}

// ── Render: terminal list ────────────────────────────────────
function inboundTerminalTransitions(targetTerminalID) {
  const inbound = [];
  const visit = (source, node) => {
    if (node?.terminalTransition?.targetTerminalId === targetTerminalID) {
      inbound.push(`${source.name}: ${node.name}`);
    }
    for (const child of node?.children ?? []) visit(source, child);
  };
  for (const source of state.session?.terminals ?? []) visit(source, source.root);
  return inbound;
}

function terminalListSnapshot(revision) {
  if (!state.session) return { groups: [], terminals: [] };
  ensureSessionTerminalGroups();
  const terminalsByID = new Map(state.session.terminals.map(term => [term.id, term]));
  return {
    groups: state.session.terminalGroups.map(group => ({
      id: group.id,
      name: group.name,
      terminalIDs: [...group.terminalIds],
    })),
    terminals: state.session.terminalGroups.flatMap(group => group.terminalIds.flatMap((terminalID, memberIndex) => {
      const terminal = terminalsByID.get(terminalID);
      if (!terminal) return [];
      return [{
        groupID: group.id,
        groupName: group.name,
        id: terminal.id,
        live: terminal.id === state.liveTerminalId,
        memberCount: group.terminalIds.length,
        memberIndex,
        name: terminal.name,
        revision,
        selected: terminal.id === state.editTerminalId,
      }];
    })),
  };
}

function applyTerminalSelectionRequest(message) {
  if (message?.kind === 'terminal-action-request') {
    if (message.revision !== terminalAuthoringRevision || typeof message.terminalID !== 'string') return true;
    const terminal = state.session?.terminals.find(candidate => candidate.id === message.terminalID);
    const group = state.session?.terminalGroups.find(candidate => candidate.terminalIds.includes(message.terminalID));
    if (!terminal || !group) return true;
    const memberIndex = group.terminalIds.indexOf(terminal.id);
    terminalGroupDialogOpener = { scope: 'terminal', ownerID: terminal.id };
    if (message.action === 'rename-terminal') {
      const name = typeof message.name === 'string' ? message.name.trim() : '';
      if (name !== '') {
        terminal.name = name;
        void autosave();
        renderAll();
      }
    } else if (message.action === 'move-terminal') {
      showTerminalMoveDraft(terminal.id);
    } else if (message.action === 'move-terminal-up' && memberIndex > 0) {
      prepareTerminalMemberOrder(group.id, terminal.id, -1);
    } else if (message.action === 'move-terminal-down' && memberIndex < group.terminalIds.length - 1) {
      prepareTerminalMemberOrder(group.id, terminal.id, 1);
    } else if (message.action === 'delete-terminal') {
      deleteTerminal(terminal);
    }
    if (!terminalGroupDraft && !pendingTerminalGroupImpact) terminalGroupDialogOpener = null;
    return true;
  }
  if (message?.kind !== 'terminal-selection-request') return false;
  if (message.revision !== terminalAuthoringRevision || typeof message.terminalID !== 'string') return true;
  const terminal = state.session?.terminals.find(candidate => candidate.id === message.terminalID);
  if (!terminal || state.editTerminalId === terminal.id) return true;
  state.editTerminalId = terminal.id;
  state.selectedNodeId = null;
  state.expanded = new Set(['root']);
  renderAll();
  return true;
}

function ensureSessionTerminalGroups() {
  if (!state.session) return;
  const terminalIDs = new Set(state.session.terminals.map(term => term.id));
  const represented = new Set();
  const groups = [];
  for (const group of state.session.terminalGroups ?? []) {
    const members = (group.terminalIds ?? []).filter(id => terminalIDs.has(id) && !represented.has(id));
    members.forEach(id => represented.add(id));
    if (members.length) groups.push({ id: group.id, name: group.name, terminalIds: members });
  }
  const usedNames = new Set(groups.map(group => group.name.trim().toLocaleLowerCase()));
  for (const term of state.session.terminals) {
    if (represented.has(term.id)) continue;
    let name = term.name.trim() || 'Terminal';
    let suffix = 2;
    while (usedNames.has(name.toLocaleLowerCase())) name = `${term.name.trim() || 'Terminal'} (${suffix++})`;
    usedNames.add(name.toLocaleLowerCase());
    groups.push({ id: uid('g'), name, terminalIds: [term.id] });
  }
  state.session.terminalGroups = groups;
}

function setTerminalGroupError(message = '') {
  terminalGroupError.textContent = message;
  terminalGroupError.hidden = !message;
}

function retainTerminalGroupDialogOpener() {
  if (!terminalGroupDialogOpener) {
    terminalGroupDialogOpener = { element: document.activeElement };
  }
}

function restoreTerminalGroupDialogFocus() {
  const opener = terminalGroupDialogOpener;
  terminalGroupDialogOpener = null;
  if (!opener) return;
  if (opener.element?.isConnected) {
    opener.element.focus();
    return;
  }
  publishLegacyProjection({
    kind: 'terminal-selection-focus-request',
    ownerID: opener.ownerID,
    scope: opener.scope,
  });
}

function closeTerminalGroupDraft({ restoreFocus = true, publish = true } = {}) {
  terminalGroupDraft = null;
  if (publish) publishLegacyProjection({ kind: 'terminal-group-draft-dismiss' });
  if (restoreFocus) restoreTerminalGroupDialogFocus();
}

function closeTerminalGroupImpact({ restoreFocus = true, publish = true } = {}) {
  pendingTerminalGroupImpact = null;
  if (publish) publishLegacyProjection({ kind: 'terminal-group-impact-dismiss' });
  if (restoreFocus) restoreTerminalGroupDialogFocus();
}

function showTerminalGroupCreate() {
  if (!state.session) return;
  setTerminalGroupError();
  terminalGroupDraft = { kind: 'create' };
  retainTerminalGroupDialogOpener();
  publishLegacyProjection({
    kind: 'terminal-group-draft-open',
    mode: 'create',
    terminals: structuredClone(state.session.terminals),
  });
}

function showTerminalGroupRename(groupID) {
  const group = state.session.terminalGroups.find(candidate => candidate.id === groupID);
  if (!group) return;
  setTerminalGroupError();
  terminalGroupDraft = { kind: 'rename', groupID };
  retainTerminalGroupDialogOpener();
  publishLegacyProjection({
    kind: 'terminal-group-draft-open',
    mode: 'rename',
    name: group.name,
  });
}

function showTerminalMoveDraft(terminalID) {
  const source = state.session.terminalGroups.find(group => group.terminalIds.includes(terminalID));
  if (!source) return;
  setTerminalGroupError();
  terminalGroupDraft = { kind: 'move', terminalID, sourceGroupID: source.id };
  const destinations = [];
  if (source.terminalIds.length > 1) {
    destinations.push({ id: 'new-singleton', name: 'НОВАЯ ОДИНОЧНАЯ ГРУППА', newSingleton: true });
  }
  for (const group of state.session.terminalGroups) {
    if (group.id === source.id) continue;
    destinations.push({ id: group.id, name: group.name, newSingleton: false });
  }
  retainTerminalGroupDialogOpener();
  publishLegacyProjection({ kind: 'terminal-group-draft-open', mode: 'move', destinations });
}

function terminalName(terminalID) {
  return state.session.terminals.find(terminal => terminal.id === terminalID)?.name || terminalID;
}

function groupName(groups, groupID) {
  return groups.find(group => group.id === groupID)?.name || groupID || '—';
}

function uniqueLocalGroupName(base, groups) {
  const used = new Set(groups.map(group => group.name.trim().toLocaleLowerCase()));
  const encoder = new TextEncoder();
  const truncate = (value, byteLimit) => {
    let result = '';
    let byteCount = 0;
    for (const symbol of value) {
      const symbolBytes = encoder.encode(symbol).length;
      if (byteCount + symbolBytes > byteLimit) break;
      result += symbol;
      byteCount += symbolBytes;
    }
    return result;
  };
  const rawBase = String(base || 'Terminal').trim() || 'Terminal';
  let candidate = truncate(rawBase, 256);
  let suffix = 2;
  while (used.has(candidate.toLocaleLowerCase())) {
    const ending = ` (${suffix++})`;
    candidate = truncate(rawBase, 256 - encoder.encode(ending).length) + ending;
  }
  return candidate;
}

function newLocalSingletonGroup(terminalID, groups) {
  return {
    id: uid('group'),
    name: uniqueLocalGroupName(terminalName(terminalID), groups),
    terminalIds: [terminalID],
  };
}

function rejectTerminalGroupDraft(message) {
  setTerminalGroupError(message);
  publishLegacyProjection({ kind: 'terminal-group-command-feedback', target: 'draft', error: message });
}

function reviewTerminalGroupDraft(message) {
  if (!terminalGroupDraft || !state.session) return;
  const before = structuredClone(state.session.terminalGroups);
  if (terminalGroupDraft.kind === 'create') {
    const name = typeof message?.name === 'string' ? message.name.trim() : '';
    const knownTerminalIDs = new Set(state.session.terminals.map(terminal => terminal.id));
    const terminalIDs = Array.isArray(message?.selectedTerminalIds)
      ? [...new Set(message.selectedTerminalIds.filter(id => typeof id === 'string' && knownTerminalIDs.has(id)))]
      : [];
    if (!name || terminalIDs.length < 2) {
      rejectTerminalGroupDraft('УКАЖИТЕ УНИКАЛЬНОЕ НАЗВАНИЕ И ВЫБЕРИТЕ НЕ МЕНЕЕ ДВУХ ТЕРМИНАЛОВ');
      return;
    }
    const selected = new Set(terminalIDs);
    const sourceNames = before.filter(group => group.terminalIds.some(id => selected.has(id))).map(group => group.name);
    const candidate = before
      .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => !selected.has(id)) }))
      .filter(group => group.terminalIds.length);
    if (candidate.some(group => group.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
      rejectTerminalGroupDraft('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ');
      return;
    }
    const reusableGroup = before.find(group =>
      group.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase()
      && group.terminalIds.every(id => selected.has(id)));
    candidate.push({ id: reusableGroup?.id || uid('group'), name, terminalIds: terminalIDs });
    showTerminalGroupImpact({
      kind: 'СОЗДАНИЕ ГРУППЫ', candidate, affectedGroupNames: [...new Set([...sourceNames, name])],
      affectedTerminalIDs: terminalIDs, membership: `${name}: ${terminalIDs.map(terminalName).join(' → ')}`,
      orderBefore: before.flatMap(group => group.terminalIds).map(terminalName),
      orderAfter: candidate.flatMap(group => group.terminalIds).map(terminalName),
    });
    return;
  }
  if (terminalGroupDraft.kind === 'move') {
    const destinationGroupID = typeof message?.destinationGroupId === 'string'
      ? message.destinationGroupId
      : '';
    const sourceGroupID = terminalGroupDraft.sourceGroupID;
    const terminalID = terminalGroupDraft.terminalID;
    if (!destinationGroupID) {
      rejectTerminalGroupDraft('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ');
      return;
    }
    const candidate = before
      .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => id !== terminalID) }))
      .filter(group => group.terminalIds.length);
    const splitToSingleton = destinationGroupID === 'new-singleton';
    if (splitToSingleton) {
      const singleton = newLocalSingletonGroup(terminalID, candidate);
      candidate.push(singleton);
      const source = candidate.find(group => group.id === sourceGroupID);
      showTerminalGroupImpact({
        kind: 'ОТДЕЛЕНИЕ ТЕРМИНАЛА', candidate,
        affectedGroupNames: [groupName(before, sourceGroupID), singleton.name],
        affectedTerminalIDs: [terminalID], sourceGroupID, destinationGroupID: singleton.id,
        destinationGroupName: singleton.name,
        membership: `${source.name}: ${source.terminalIds.map(terminalName).join(' → ')} · ${singleton.name}: ${terminalName(terminalID)}`,
        orderBefore: before.find(group => group.id === sourceGroupID)?.terminalIds.map(terminalName) || [],
        orderAfter: [...source.terminalIds, terminalID].map(terminalName),
      });
      return;
    }
    const destination = candidate.find(group => group.id === destinationGroupID);
    if (!destination) {
      rejectTerminalGroupDraft('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ');
      return;
    }
    destination.terminalIds.push(terminalID);
    showTerminalGroupImpact({
      kind: 'ПЕРЕМЕЩЕНИЕ ТЕРМИНАЛА', candidate,
      affectedGroupNames: [groupName(before, sourceGroupID), groupName(before, destinationGroupID)],
      affectedTerminalIDs: [terminalID], sourceGroupID, destinationGroupID,
      membership: `${groupName(before, destinationGroupID)}: ${destination.terminalIds.map(terminalName).join(' → ')}`,
      orderBefore: before.find(group => group.id === sourceGroupID)?.terminalIds.map(terminalName) || [],
      orderAfter: destination.terminalIds.map(terminalName),
    });
  }
}

function prepareTerminalMemberOrder(groupID, terminalID, delta) {
  const before = structuredClone(state.session.terminalGroups);
  const candidate = structuredClone(before);
  const group = candidate.find(item => item.id === groupID);
  const index = group?.terminalIds.indexOf(terminalID) ?? -1;
  const next = index + delta;
  if (!group || index < 0 || next < 0 || next >= group.terminalIds.length) return;
  [group.terminalIds[index], group.terminalIds[next]] = [group.terminalIds[next], group.terminalIds[index]];
  showTerminalGroupImpact({
    kind: 'ИЗМЕНЕНИЕ ПОРЯДКА', candidate, affectedGroupNames: [group.name],
    affectedTerminalIDs: [terminalID, group.terminalIds[index]],
    membership: `${group.name}: ${group.terminalIds.map(terminalName).join(' → ')}`,
    orderBefore: before.find(item => item.id === groupID).terminalIds.map(terminalName),
    orderAfter: group.terminalIds.map(terminalName),
  });
}

function prepareTerminalGroupOrder(groupID, delta) {
  const before = structuredClone(state.session.terminalGroups);
  const candidate = structuredClone(before);
  const index = candidate.findIndex(group => group.id === groupID);
  const next = index + delta;
  if (index < 0 || next < 0 || next >= candidate.length) return;
  [candidate[index], candidate[next]] = [candidate[next], candidate[index]];
  showTerminalGroupImpact({
    kind: 'ИЗМЕНЕНИЕ ПОРЯДКА', candidate,
    affectedGroupNames: [candidate[index].name, candidate[next].name], affectedTerminalIDs: [],
    membership: candidate.map(group => group.name).join(' → '),
    orderBefore: before.map(group => group.name), orderAfter: candidate.map(group => group.name),
  });
}

function prepareTerminalGroupDissolution(groupID) {
  const before = structuredClone(state.session.terminalGroups);
  const group = before.find(candidate => candidate.id === groupID);
  if (!group) return;
  if (group.terminalIds.length === 1) {
    setTerminalGroupError('ОДИНОЧНУЮ ГРУППУ НЕЛЬЗЯ РАСФОРМИРОВАТЬ: ТЕРМИНАЛ ДОЛЖЕН ОСТАТЬСЯ В ГРУППЕ');
    return;
  }
  const candidate = before.filter(candidate => candidate.id !== groupID);
  const resultantSingletons = [];
  for (const terminalID of group.terminalIds) {
    const singleton = newLocalSingletonGroup(terminalID, candidate);
    candidate.push(singleton);
    resultantSingletons.push(singleton);
  }
  showTerminalGroupImpact({
    kind: 'РАСФОРМИРОВАНИЕ ГРУППЫ', candidate,
    affectedGroupNames: [group.name, ...resultantSingletons.map(singleton => singleton.name)],
    affectedTerminalIDs: group.terminalIds, sourceGroupID: group.id,
    membership: resultantSingletons
      .map(singleton => `${terminalName(singleton.terminalIds[0])} → ${singleton.name}`)
      .join(' · '),
    orderBefore: group.terminalIds.map(terminalName), orderAfter: group.terminalIds.map(terminalName),
  });
}

function currentSessionRevision() {
  return Number(saveStatus.dataset.savedRevision || newestDurableRevision || 0);
}

function terminalGroupCandidateMembership(groups) {
  return groups
    .map(group => `${group.name}: ${group.terminalIds.map(terminalName).join(' → ')}`)
    .join(' · ');
}

function authoredTransitionRejections(message) {
  const pattern = /terminal transition command "([^"]+)" in terminal "([^"]+)" targets terminal "([^"]+)" and crosses groups "([^"]+)" and "([^"]+)"/g;
  return [...String(message || '').matchAll(pattern)].map(match => ({
    commandID: match[1],
    sourceTerminalID: match[2],
    targetTerminalID: match[3],
    sourceGroupID: match[4],
    targetGroupID: match[5],
  }));
}

function actionableTransitionRejection(rejections, candidate) {
  const edges = rejections.map(rejection =>
    `КОМАНДА «${rejection.commandID}»: «${terminalName(rejection.sourceTerminalID)}» → «${terminalName(rejection.targetTerminalID)}» ОСТАЁТСЯ МЕЖДУ ГРУППАМИ «${groupName(candidate, rejection.sourceGroupID)}» И «${groupName(candidate, rejection.targetGroupID)}»`);
  return `${edges.join(' · ')}. ДОБАВЬТЕ СВЯЗАННЫЕ ТЕРМИНАЛЫ В ОДНО ПРЕДЛОЖЕНИЕ И ПРОВЕРЬТЕ ЕГО ПЕРЕД ПРИМЕНЕНИЕМ.`;
}

function amendRejectedTerminalGroupImpact() {
  const impact = pendingTerminalGroupImpact;
  if (!impact?.rejections?.length) return;
  const involvedGroupIDs = new Set(impact.rejections.flatMap(rejection =>
    [rejection.sourceGroupID, rejection.targetGroupID]));
  const selectedTerminalIDs = impact.candidate
    .filter(group => involvedGroupIDs.has(group.id))
    .flatMap(group => group.terminalIds);
  const preferredGroup = impact.candidate.find(group => group.id === impact.destinationGroupID)
    || impact.candidate.find(group => group.id === impact.rejections[0].sourceGroupID);

  closeTerminalGroupImpact({ restoreFocus: false });
  setTerminalGroupError();
  terminalGroupDraft = { kind: 'create' };
  publishLegacyProjection({
    kind: 'terminal-group-draft-open',
    mode: 'create',
    name: preferredGroup?.name || '',
    selectedTerminalIds: selectedTerminalIDs,
    terminals: structuredClone(state.session.terminals),
  });
}

function showTerminalGroupImpact(impact) {
  setTerminalGroupError();
  closeTerminalGroupDraft({ restoreFocus: false });
  pendingTerminalGroupImpact = {
    ...impact,
    expectedSessionRevision: currentSessionRevision(),
    expectedCoordinationRevision: coordinationRevision(state.coordination),
  };
  publishLegacyProjection({
    kind: 'terminal-group-impact-open',
    changeKind: impact.kind,
    candidate: structuredClone(impact.candidate),
    expectedSessionRevision: pendingTerminalGroupImpact.expectedSessionRevision,
    expectedCoordinationRevision: pendingTerminalGroupImpact.expectedCoordinationRevision,
    groups: impact.affectedGroupNames.join(' · ') || '—',
    terminals: impact.affectedTerminalIDs.map(terminalName).join(' · ') || '—',
    sourceGroup: groupName(state.session.terminalGroups, impact.sourceGroupID),
    destinationGroup: impact.destinationGroupName || groupName(state.session.terminalGroups, impact.destinationGroupID),
    membership: terminalGroupCandidateMembership(impact.candidate) || '—',
    orderBefore: (impact.orderBefore || []).join(' → ') || '—',
    orderAfter: (impact.orderAfter || []).join(' → ') || '—',
  });
}

function applyTerminalGroupCommandResult(message) {
  const source = message?.source === 'draft' ? 'draft' : 'impact';
  const result = message?.result && typeof message.result === 'object'
    ? message.result
    : { ok: false, error: 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ГРУППЫ ТЕРМИНАЛОВ' };
  if (result?.session) state.session = result.session;
  const revision = Number(result?.sessionRevision || 0);
  newestDurableRevision = Math.max(newestDurableRevision, revision);
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  if (result?.coordinationState) applyCoordinationState(result.coordinationState);
  if (!result?.ok) {
    const error = result?.error || 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ГРУППЫ ТЕРМИНАЛОВ';
    setTerminalGroupError(error);
    const rejections = authoredTransitionRejections(error);
    const canAmend = source === 'impact' && pendingTerminalGroupImpact && rejections.length > 0;
    if (canAmend) {
      pendingTerminalGroupImpact.rejections = rejections;
    }
    renderAll();
    const close = source === 'impact' && !canAmend;
    publishLegacyProjection({
      kind: 'terminal-group-command-feedback',
      target: source,
      close,
      canAmend,
      error: canAmend
        ? actionableTransitionRejection(rejections, pendingTerminalGroupImpact.candidate)
        : error,
    });
    if (close) {
      pendingTerminalGroupImpact = null;
    }
    return;
  }
  setTerminalGroupError();
  saveStatus.textContent = `ГРУППЫ СОХРАНЕНЫ · ревизия ${revision}`;
  saveStatus.classList.remove('err');
  renderAll();
  publishLegacyProjection({ kind: 'terminal-group-command-feedback', target: source, close: true });
  if (source === 'draft') terminalGroupDraft = null;
  else pendingTerminalGroupImpact = null;
}

function saveTerminalGroupRename(message) {
  const group = state.session.terminalGroups.find(candidate => candidate.id === terminalGroupDraft?.groupID);
  const name = typeof message?.name === 'string' ? message.name.trim() : '';
  if (!group || !name) {
    rejectTerminalGroupDraft('НАЗВАНИЕ ГРУППЫ НЕ ДОЛЖНО БЫТЬ ПУСТЫМ');
    return;
  }
  if (state.session.terminalGroups.some(candidate => candidate.id !== group.id && candidate.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
    rejectTerminalGroupDraft('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ');
    return;
  }
  const candidate = structuredClone(state.session.terminalGroups);
  candidate.find(item => item.id === group.id).name = name;
  publishLegacyProjection({
    kind: 'terminal-group-draft-submit',
    candidate,
    expectedSessionRevision: currentSessionRevision(),
    expectedCoordinationRevision: coordinationRevision(state.coordination),
  });
}

btnCreateTerminalGroup.addEventListener('click', showTerminalGroupCreate);

function deleteTerminal(term) {
  if (state.liveTerminalId === term.id || state.coordination?.pendingSwitch?.sourceTerminalId === term.id) {
    setCoordinationStatus('АКТИВНЫЙ ИЛИ СОХРАНЁННЫЙ ТЕРМИНАЛ НЕЛЬЗЯ УДАЛИТЬ', true);
    return;
  }
  const inbound = inboundTerminalTransitions(term.id);
  if (inbound.length) {
    setCoordinationStatus(`ТЕРМИНАЛ ИСПОЛЬЗУЕТСЯ КОМАНДАМИ ПЕРЕХОДА: ${inbound.join(', ')}`, true);
    return;
  }
  if (!window.confirm(`Удалить терминал "${term.name}" целиком?`)) return;
  const idx = state.session.terminals.findIndex(candidate => candidate.id === term.id);
  if (idx >= 0) state.session.terminals.splice(idx, 1);
  state.session.terminalGroups = (state.session.terminalGroups ?? [])
    .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => id !== term.id) }))
    .filter(group => group.terminalIds.length > 0);
  if (state.editTerminalId === term.id) {
    state.editTerminalId = state.session.terminals[0]?.id || null;
    state.selectedNodeId = null;
    state.expanded = new Set(['root']);
  }
  void autosave();
  renderAll();
}

// ── Projection: selected terminal editor/settings ───────────
function terminalEditorSnapshot() {
  const term = getEditTerminal();
  const isLive = !!term && term.id === state.liveTerminalId;
  const snapshot = {
    broadcastActive: Boolean(state.coordination?.broadcast),
    pending: coordinationCommandPending,
    publishAcknowledgement: terminalPublishAcknowledgement,
    resetPending: sessionStateCommandPending,
    terminal: term ? {
      completedCount: term.commandStates && typeof term.commandStates === 'object'
        ? Object.keys(term.commandStates).length
        : 0,
      hackLevel: Number(term.hackLevel || 0),
      id: term.id,
      introText: term.introText || '',
      live: isLive,
      name: term.name,
    } : null,
  };

  return snapshot;
}

function projectTerminalTreeNode(term, node) {
  const execution = commandExecutionState(term, node.id);
  return {
    children: node.type === 'folder' && Array.isArray(node.children)
      ? node.children.map(child => projectTerminalTreeNode(term, child))
      : [],
    description: node.type === 'entry' ? (node.description || '') : '',
    displayName: effectiveNodeName(term, node),
    execution: execution ? {
      completedName: typeof execution.completedName === 'string' ? execution.completedName : '',
      resultText: typeof execution.resultText === 'string' ? execution.resultText : '',
    } : null,
    id: node.id,
    name: node.name,
    stateChange: node.type === 'command' && node.stateChange ? {
      completedName: node.stateChange.completedName || '',
      confirmationText: node.stateChange.confirmationText || '',
    } : null,
    terminalTransition: node.type === 'command' && node.terminalTransition ? {
      targetTerminalId: node.terminalTransition.targetTerminalId || '',
    } : null,
    text: node.type === 'command' ? (node.text || '') : '',
    type: node.type,
  };
}

function terminalTreeSnapshot() {
  const term = getEditTerminal();
  if (!term) {
    return {
      available: false,
    };
  }
  const selected = state.selectedNodeId ? locateNode(term.root, state.selectedNodeId) : null;
  if (state.selectedNodeId && !selected) state.selectedNodeId = null;
  const target = currentAddTarget();
  const group = state.session.terminalGroups?.find(candidate => candidate.terminalIds.includes(term.id));
  return {
    available: true,
    addTargetName: target?.id === 'root' ? 'ROOT' : (target?.name || ''),
    expandedIDs: [...state.expanded],
    pending: sessionStateCommandPending,
    root: projectTerminalTreeNode(term, term.root),
    selectedNodeID: state.selectedNodeId,
    terminalID: term.id,
    terminalOptions: state.session.terminals
      .filter(candidate => candidate.id !== term.id && group?.terminalIds.includes(candidate.id))
      .map(candidate => ({ id: candidate.id, name: candidate.name })),
  };
}

function addTerminalTreeNode(type) {
  const target = currentAddTarget();
  if (!target) return null;
  const node = {
    id: uid('n'),
    type,
    name: type === 'folder' ? 'Новая папка' : type === 'command' ? 'Новая команда' : 'Новая запись',
  };
  if (type === 'folder') node.children = [];
  if (type === 'command') node.text = '';
  if (type === 'entry') node.description = '';
  if (!target.children) target.children = [];
  target.children.push(node);
  state.expanded.add(target.id);
  state.selectedNodeId = node.id;
  void autosave();
  renderTerminalAuthoringProjection();
  publishLegacyProjection({ kind: 'terminal-tree-focus-request', nodeID: node.id });
  return node;
}

function applyTerminalTreeDraft(term, node, draft) {
  if (!draft || typeof draft !== 'object' || typeof draft.name !== 'string') return false;
  const name = draft.name.trim();
  if (!name) return false;
  if (node.type === 'command') {
    if (typeof draft.commandMode !== 'string' || typeof draft.text !== 'string'
      || typeof draft.completedName !== 'string' || typeof draft.confirmationText !== 'string'
      || typeof draft.targetTerminalID !== 'string') return false;
    let commandMode = draft.commandMode;
    if (commandExecutionState(term, node.id)) {
      commandMode = node.stateChange
        ? 'state-change'
        : node.terminalTransition
          ? 'terminal-transition'
          : 'ordinary';
    }
    if (!['ordinary', 'state-change', 'terminal-transition'].includes(commandMode)) return false;
    if (commandMode === 'state-change') {
      if (!draft.completedName.trim() || !draft.confirmationText.trim() || !draft.text.trim()) return false;
      node.stateChange = {
        completedName: draft.completedName,
        confirmationText: draft.confirmationText,
      };
    } else {
      delete node.stateChange;
    }
    if (commandMode === 'terminal-transition') {
      const group = state.session.terminalGroups?.find(candidate => candidate.terminalIds.includes(term.id));
      if (!draft.targetTerminalID || draft.targetTerminalID === term.id
        || !group?.terminalIds.includes(draft.targetTerminalID)
        || !state.session.terminals.some(candidate => candidate.id === draft.targetTerminalID)) return false;
      node.terminalTransition = { targetTerminalId: draft.targetTerminalID };
    } else {
      delete node.terminalTransition;
    }
    node.text = draft.text;
  } else if (node.type === 'entry') {
    if (typeof draft.text !== 'string') return false;
    node.description = draft.text;
  }
  node.name = name;
  return true;
}

async function runTerminalTreeAction(message, term) {
  if (message.action === 'select-node') {
    if (typeof message.nodeID !== 'string') return;
    const location = locateNode(term.root, message.nodeID);
    if (!location) return;
    state.selectedNodeId = location.node.id;
    if (location.node.type === 'folder') state.expanded.add(location.node.id);
    renderTerminalAuthoringProjection();
    return;
  }
  if (message.action === 'toggle-node') {
    if (typeof message.nodeID !== 'string') return;
    const location = locateNode(term.root, message.nodeID);
    if (!location || location.node.type !== 'folder' || !location.node.children?.length) return;
    if (state.expanded.has(location.node.id)) state.expanded.delete(location.node.id);
    else state.expanded.add(location.node.id);
    renderTerminalAuthoringProjection();
    return;
  }
  if (message.action === 'add-node') {
    if (message.nodeType === 'folder' || message.nodeType === 'command' || message.nodeType === 'entry') {
      addTerminalTreeNode(message.nodeType);
    }
    return;
  }
  if (typeof message.nodeID !== 'string' || message.nodeID !== state.selectedNodeId) return;
  const location = locateNode(term.root, message.nodeID);
  if (!location || location.node.id === 'root') return;
  const node = location.node;
  if (message.action === 'apply-node') {
    if (!applyTerminalTreeDraft(term, node, message.draft)) return;
    void autosave();
    renderTerminalAuthoringProjection();
    return;
  }
  if (message.action === 'delete-node') {
    const siblings = location.parent?.children;
    if (!siblings) return;
    const index = siblings.findIndex(candidate => candidate.id === node.id);
    if (index < 0) return;
    siblings.splice(index, 1);
    state.selectedNodeId = null;
    void autosave();
    renderTerminalAuthoringProjection();
    return;
  }
  if (message.action !== 'reset-command-state' || node.type !== 'command'
    || !commandExecutionState(term, node.id) || sessionStateCommandPending) return;
  const execution = commandExecutionState(term, node.id);
  const displayedName = execution?.completedName || node.name;
  if (!await confirmCommandStateReset(`Сбросить выполненное состояние команды "${displayedName}"?`)) return;
  const revisionBeforeReset = newestDurableRevision;
  await runSessionStateCommand(
    () => desktopAPI.resetCommandState({ terminalId: term.id, commandId: node.id }),
    'СОСТОЯНИЕ КОМАНДЫ СБРОШЕНО',
    (result) => {
      const revision = Number(result?.revision || 0);
      const canonicalTerminal = result?.session?.terminals?.find(candidate => candidate.id === term.id);
      return revision > revisionBeforeReset && canonicalTerminal
        && !Object.hasOwn(canonicalTerminal.commandStates ?? {}, node.id);
    }
  );
}

function applyTerminalTreeRequest(message) {
  if (message?.kind !== 'terminal-tree-action-request') return false;
  if (message.revision !== terminalAuthoringRevision || typeof message.terminalID !== 'string') return true;
  const term = state.session?.terminals.find(candidate => candidate.id === message.terminalID);
  if (!term || term.id !== state.editTerminalId) return true;
  void runTerminalTreeAction(message, term);
  return true;
}

// ── Player roster and broadcast management ──────────────────

// ── Terminal management ───────────────────────────────────────
function createTerminalSnapshot() {
  return {
    open: createTerminalOpen,
    pending: createTerminalPending,
  };
}

function hackControlContextSnapshot() {
  const term = state.session && state.liveTerminalId
    ? state.session.terminals.find(candidate => candidate.id === state.liveTerminalId)
    : null;
  if (!term || !term.hackLevel) return null;
  return {
    coordinationRevision: coordinationRevision(state.coordination),
    hackLevel: Number(term.hackLevel),
    introText: term.introText || '',
    terminalID: term.id,
    terminalName: term.name,
    tree: term.root,
  };
}

function renderTerminalAuthoringProjection() {
  terminalAuthoringRevision += 1;
  const list = terminalListSnapshot(terminalAuthoringRevision);
  terminalAuthoringProjection = {
    create: createTerminalSnapshot(),
    editor: terminalEditorSnapshot(),
    groups: list.groups,
    hackContext: hackControlContextSnapshot(),
    kind: 'terminal-authoring-snapshot',
    revision: terminalAuthoringRevision,
    terminals: list.terminals,
    tree: terminalTreeSnapshot(),
  };
  publishLegacyProjection(terminalAuthoringProjection);
}

btnAddTerminal.addEventListener('click', () => {
  if (!state.session || createTerminalPending) return;
  createTerminalOpen = true;
  renderTerminalAuthoringProjection();
});

async function runCreateTerminalAction(message) {
  if (message.action === 'cancel') {
    if (createTerminalPending) return;
    createTerminalOpen = false;
    renderTerminalAuthoringProjection();
    if (!btnAddTerminal.hidden) btnAddTerminal.focus();
    return;
  }
  if (message.action !== 'create' || createTerminalPending || !state.session
    || typeof message.name !== 'string') return;
  const name = message.name.trim();
  if (!name) return;
  createTerminalPending = true;
  renderTerminalAuthoringProjection();
  const term = {
    id: uid('t'),
    name,
    hackLevel: 0,
    introText: '',
    root: { id: 'root', type: 'folder', name: 'ROOT', children: [] },
  };
  state.session.terminals.push(term);
  ensureSessionTerminalGroups();
  state.editTerminalId = term.id;
  state.selectedNodeId = null;
  state.expanded = new Set(['root']);
  await autosave();
  createTerminalOpen = false;
  createTerminalPending = false;
  renderAll();
  renderTerminalAuthoringProjection();
  publishLegacyProjection({ kind: 'terminal-editor-focus-settings' });
}

function applyCreateTerminalRequest(message) {
  if (message?.kind !== 'create-terminal-action-request') return false;
  if (message.revision !== terminalAuthoringRevision || !createTerminalOpen) return true;
  void runCreateTerminalAction(message);
  return true;
}

function terminalActivationRequest(term) {
  return desktopAPI.requestTerminalActivation({
    terminalId: term.id,
    terminalName: term.name,
    tree: term.root,
    hackLevel: term.hackLevel || 0,
    introText: term.introText || '',
  });
}



publishLegacyProjection({ kind: 'legacy-ready' });
