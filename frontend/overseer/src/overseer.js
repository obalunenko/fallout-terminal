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
  collapsedTerminalGroupIds: new Set(),
  liveHack:       null,   // last known hack-state of the live terminal, or null
  coordination:   null,   // authoritative roster/session/broadcast projection
};

let idCounter = 0;
function uid(prefix) {
  idCounter++;
  return `${prefix}_${Date.now().toString(36)}_${idCounter}`;
}

// ── DOM refs ──────────────────────────────────────────────
const mainLayout        = legacyOverseerRoot.querySelector('#mainLayout');
const sessionFileLabel  = legacyOverseerRoot.querySelector('#sessionFileLabel');
const serverUrlEl       = legacyOverseerRoot.querySelector('#serverUrl');
const clientCountEl     = legacyOverseerRoot.querySelector('#clientCount');
const termList          = legacyOverseerRoot.querySelector('#termList');
const saveStatus        = legacyOverseerRoot.querySelector('#saveStatus');
const editingTermName   = legacyOverseerRoot.querySelector('#editingTermName');
const liveFlag          = legacyOverseerRoot.querySelector('#liveFlag');
const btnMakeLive       = legacyOverseerRoot.querySelector('#btnMakeLive');
const btnPublish        = legacyOverseerRoot.querySelector('#btnPublish');
const terminalSettingsMenu = legacyOverseerRoot.querySelector('#terminalSettingsMenu');
const btnReapplySettings = legacyOverseerRoot.querySelector('#btnReapplySettings');
const treeView          = legacyOverseerRoot.querySelector('#treeView');
const nodeForm          = legacyOverseerRoot.querySelector('#nodeForm');
const toolbarHint       = legacyOverseerRoot.querySelector('#toolbarHint');
const btnAddFolder      = legacyOverseerRoot.querySelector('#btnAddFolder');
const btnAddCommand     = legacyOverseerRoot.querySelector('#btnAddCommand');
const btnAddEntry       = legacyOverseerRoot.querySelector('#btnAddEntry');
const btnAddTerminal    = legacyOverseerRoot.querySelector('#btnAddTerminal');
const btnCreateTerminalGroup = legacyOverseerRoot.querySelector('#btnCreateTerminalGroup');
const terminalGroupError = legacyOverseerRoot.querySelector('#terminalGroupError');
const terminalGroupDraftDialog = legacyOverseerRoot.querySelector('#terminalGroupDraftDialog');
const terminalGroupDraftForm = legacyOverseerRoot.querySelector('#terminalGroupDraftForm');
const terminalGroupNameInput = legacyOverseerRoot.querySelector('#terminalGroupNameInput');
const terminalGroupTerminalChoices = legacyOverseerRoot.querySelector('#terminalGroupTerminalChoices .terminal-group-terminal-choice-list');
const terminalGroupTerminalChoiceTemplate = legacyOverseerRoot.querySelector('#terminalGroupTerminalChoiceTemplate');
const terminalGroupDestinationSelect = legacyOverseerRoot.querySelector('#terminalGroupDestinationSelect');
const terminalGroupImpactDialog = legacyOverseerRoot.querySelector('#terminalGroupImpactDialog');
const terminalGroupImpactSummary = legacyOverseerRoot.querySelector('#terminalGroupImpactSummary');
const terminalGroupImpactError = legacyOverseerRoot.querySelector('#terminalGroupImpactError');
const amendTerminalGroupChangeButton = terminalGroupImpactDialog.querySelector('[data-action="amend-terminal-group-change"]');
const btnStopBroadcast  = legacyOverseerRoot.querySelector('#btnStopBroadcast');
const createTerminalDialog = legacyOverseerRoot.querySelector('#createTerminalDialog');
const createTerminalForm = legacyOverseerRoot.querySelector('#createTerminalForm');
const createTerminalName = legacyOverseerRoot.querySelector('#createTerminalName');
const createTerminalError = legacyOverseerRoot.querySelector('#createTerminalError');
const btnCancelCreateTerminal = legacyOverseerRoot.querySelector('#btnCancelCreateTerminal');
const btnConfirmCreateTerminal = legacyOverseerRoot.querySelector('#btnConfirmCreateTerminal');
const takeOffAirDialog = legacyOverseerRoot.querySelector('#takeOffAirDialog');
const takeOffAirError = legacyOverseerRoot.querySelector('#takeOffAirError');
const btnCancelTakeOffAir = legacyOverseerRoot.querySelector('#btnCancelTakeOffAir');
const btnConfirmTakeOffAir = legacyOverseerRoot.querySelector('#btnConfirmTakeOffAir');
const hackStatus        = legacyOverseerRoot.querySelector('#hackStatus');
const hackStatusLine    = legacyOverseerRoot.querySelector('#hackStatusLine');
const btnHackSuccess    = legacyOverseerRoot.querySelector('#btnHackSuccess');
const btnResetFailedHack = legacyOverseerRoot.querySelector('#btnResetFailedHack');
const hackLevelSelect   = legacyOverseerRoot.querySelector('#hackLevelSelect');
const introTextArea     = legacyOverseerRoot.querySelector('#introTextArea');
const btnApplySettings  = legacyOverseerRoot.querySelector('#btnApplySettings');
const termSettings      = legacyOverseerRoot.querySelector('#termSettings');
const broadcastSummary = legacyOverseerRoot.querySelector('#broadcastSummary');
const coordinationPanel = legacyOverseerRoot.querySelector('#coordinationPanel');
const btnStartBroadcast = legacyOverseerRoot.querySelector('#btnStartBroadcast');
const btnEndBroadcast = legacyOverseerRoot.querySelector('#btnEndBroadcast');
const endBroadcastDialog = legacyOverseerRoot.querySelector('#endBroadcastDialog');
const btnCancelEndBroadcast = legacyOverseerRoot.querySelector('#btnCancelEndBroadcast');
const btnConfirmEndBroadcast = legacyOverseerRoot.querySelector('#btnConfirmEndBroadcast');
const coordinationStatus = legacyOverseerRoot.querySelector('#coordinationStatus');
const coordinationError = legacyOverseerRoot.querySelector('#coordinationError');
const activeLogicalSessionCount = legacyOverseerRoot.querySelector('#activeLogicalSessionCount');
const btnManageLogicalSessions = legacyOverseerRoot.querySelector('#btnManageLogicalSessions');
const publicAccessSection = legacyOverseerRoot.querySelector('#publicAccessSection');
const publicAccessStateRow = legacyOverseerRoot.querySelector('.public-access-state-row');
const publicAccessSettingsDialog = legacyOverseerRoot.querySelector('#publicAccessSettingsDialog');
const publicAccessSetupRequired = legacyOverseerRoot.querySelector('#publicAccessSetupRequired');
const publicAccessSettingsError = legacyOverseerRoot.querySelector('#publicAccessSettingsError');
const publicAccessGuide = legacyOverseerRoot.querySelector('#publicAccessGuide');
const publicAccessForm = legacyOverseerRoot.querySelector('#publicAccessForm');
const publicAccessDomain = legacyOverseerRoot.querySelector('#publicAccessDomain');
const publicAccessUsernameSummary = legacyOverseerRoot.querySelector('#publicAccessUsernameSummary');
const publicAccessPasswordMask = legacyOverseerRoot.querySelector('#publicAccessPasswordMask');
const publicAccessProviderSetup = legacyOverseerRoot.querySelector('#publicAccessProviderSetup');
const publicAccessProviderConfigured = legacyOverseerRoot.querySelector('#publicAccessProviderConfigured');
const publicAccessProviderToken = legacyOverseerRoot.querySelector('#publicAccessProviderToken');
const publicAccessProviderPresence = legacyOverseerRoot.querySelector('#publicAccessProviderPresence');
const publicAccessPasswordPresence = legacyOverseerRoot.querySelector('#publicAccessPasswordPresence');
const publicAccessNgrokDocLinks = legacyOverseerRoot.querySelectorAll('[data-ngrok-doc-url]');
const publicAccessProviderTokenDialog = legacyOverseerRoot.querySelector('#publicAccessProviderTokenDialog');
const publicAccessProviderTokenForm = legacyOverseerRoot.querySelector('#publicAccessProviderTokenForm');
const publicAccessReplacementProviderToken = legacyOverseerRoot.querySelector('#publicAccessReplacementProviderToken');
const publicAccessProviderTokenError = legacyOverseerRoot.querySelector('#publicAccessProviderTokenError');
const publicAccessPlayerCredentialsDialog = legacyOverseerRoot.querySelector('#publicAccessPlayerCredentialsDialog');
const publicAccessPlayerCredentialsForm = legacyOverseerRoot.querySelector('#publicAccessPlayerCredentialsForm');
const publicAccessReplacementUsername = legacyOverseerRoot.querySelector('#publicAccessReplacementUsername');
const publicAccessReplacementPlayerPassword = legacyOverseerRoot.querySelector('#publicAccessReplacementPlayerPassword');
const publicAccessReplacementPlayerPasswordHint = legacyOverseerRoot.querySelector('#publicAccessReplacementPlayerPasswordHint');
const publicAccessPlayerCredentialsError = legacyOverseerRoot.querySelector('#publicAccessPlayerCredentialsError');
const publicAccessStatus = legacyOverseerRoot.querySelector('#publicAccessStatus');
const publicAccessError = legacyOverseerRoot.querySelector('#publicAccessError');
const publicAccessURL = legacyOverseerRoot.querySelector('#publicAccessURL');
const publicAccessCopyStatus = legacyOverseerRoot.querySelector('#publicAccessCopyStatus');
const publicAccessSettingsCopyStatus = legacyOverseerRoot.querySelector('#publicAccessSettingsCopyStatus');
const btnSavePublicAccess = legacyOverseerRoot.querySelector('#btnSavePublicAccess');
const btnStartPublicAccess = legacyOverseerRoot.querySelector('#btnStartPublicAccess');
const btnStopPublicAccess = legacyOverseerRoot.querySelector('#btnStopPublicAccess');
const btnCopyPublicURL = legacyOverseerRoot.querySelector('#btnCopyPublicURL');
const btnOpenPublicAccessSettings = legacyOverseerRoot.querySelector('#btnOpenPublicAccessSettings');
const btnClosePublicAccessSettings = legacyOverseerRoot.querySelector('#btnClosePublicAccessSettings');
const btnCancelPublicAccessSettings = legacyOverseerRoot.querySelector('#btnCancelPublicAccessSettings');
const btnOpenPublicAccessProviderToken = legacyOverseerRoot.querySelector('#btnOpenPublicAccessProviderToken');
const btnCancelPublicAccessProviderToken = legacyOverseerRoot.querySelector('#btnCancelPublicAccessProviderToken');
const btnSavePublicAccessProviderToken = legacyOverseerRoot.querySelector('#btnSavePublicAccessProviderToken');
const btnDeletePublicAccessProviderToken = legacyOverseerRoot.querySelector('#btnDeletePublicAccessProviderToken');
const btnOpenPublicAccessPlayerCredentials = legacyOverseerRoot.querySelector('#btnOpenPublicAccessPlayerCredentials');
const btnSharePublicAccessCredentials = legacyOverseerRoot.querySelector('#btnSharePublicAccessCredentials');
const btnCancelPublicAccessPlayerCredentials = legacyOverseerRoot.querySelector('#btnCancelPublicAccessPlayerCredentials');
const btnSavePublicAccessPlayerCredentials = legacyOverseerRoot.querySelector('#btnSavePublicAccessPlayerCredentials');
const btnDeletePublicAccessPlayerCredentials = legacyOverseerRoot.querySelector('#btnDeletePublicAccessPlayerCredentials');
const btnGeneratePlayerPassword = legacyOverseerRoot.querySelector('#btnGeneratePlayerPassword');
const generatedPasswordDialog = legacyOverseerRoot.querySelector('#generatedPasswordDialog');
const generatedPasswordValue = legacyOverseerRoot.querySelector('#generatedPasswordValue');
const btnCopyGeneratedPassword = legacyOverseerRoot.querySelector('#btnCopyGeneratedPassword');
const btnDismissGeneratedPassword = legacyOverseerRoot.querySelector('#btnDismissGeneratedPassword');

let serverUrl = null;
let serverUrlTitle = '';
let saveGeneration = 0;
let saveInvocation = 0;
let latestRenderedSave = 0;
let newestDurableRevision = 0;
let coordinationCommandPending = false;
let createTerminalSubmitting = false;
let takeOffAirPending = false;
let startupStatus = null;
let startupProjection = null;
let coordinationProjection = null;
let publicAccessSnapshot = null;
let publicAccessCommandPending = false;
let publicAccessSettingsDialogOpener = null;
let publicAccessProviderTokenDialogOpener = null;
let publicAccessPlayerCredentialsDialogOpener = null;
let sessionStateCommandPending = false;
let terminalGroupDraft = null;
let pendingTerminalGroupImpact = null;
let terminalGroupSubmitting = false;
let terminalGroupDialogOpener = null;

const commandStateActions = document.createElement('div');
commandStateActions.className = 'settings-row command-state-terminal-actions';
commandStateActions.hidden = true;
commandStateActions.innerHTML = `
  <button class="btn btn-mini btn-danger" id="btnResetTerminalCommandStates" type="button">
    СБРОСИТЬ ВСЕ СОСТОЯНИЯ
  </button>`;
termSettings.appendChild(commandStateActions);
const btnResetTerminalCommandStates = legacyOverseerRoot.querySelector('#btnResetTerminalCommandStates');

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

function renderStartupPresentation(status) {
  startupStatus = status && typeof status === 'object' ? status : {};
  const info = startupStatus.serverInfo && typeof startupStatus.serverInfo === 'object'
    ? startupStatus.serverInfo
    : null;
  const startupError = typeof startupStatus.startupError === 'string' ? startupStatus.startupError : '';
  const tunnelError = typeof info?.tunnelError === 'string' ? info.tunnelError : '';
  const fatal = !info && Boolean(startupError);

  let presentation;
  if (fatal) {
    presentation = { fatal, state: 'failed', text: `ЗАПУСК НЕ ЗАВЕРШЁН: ${startupError}` };
  } else if (info?.tunnel && info.url) {
    presentation = {
      fatal,
      state: 'ready-public',
      text: `ГОТОВО · ПУБЛИЧНЫЙ И ЛОКАЛЬНЫЙ ДОСТУП${info.localUrl ? ` · ${info.localUrl}` : ''}`,
    };
  } else if (info) {
    const warning = tunnelError || startupError;
    presentation = {
      fatal,
      state: warning ? 'warning' : 'ready-local',
      text: warning
        ? `ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ · ПУБЛИЧНЫЙ ДОСТУП НЕДОСТУПЕН: ${warning}`
        : `ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ · ${info.localUrl || info.url}`,
    };
  } else {
    presentation = { fatal, state: 'starting', text: 'ЗАПУСК ЛОКАЛЬНОГО СЕРВЕРА…' };
  }
  startupProjection = { kind: 'startup-status', ...presentation };
  publishLegacyProjection(startupProjection);
}

// ── Server info / connection count ─────────────────────────
desktopAPI.onServerInfo((info) => {
  renderStartupPresentation({ ...(startupStatus || {}), serverInfo: info });
  const publicUrl = info.tunnel && info.url ? info.url : '';
  const localUrl = info.localUrl || (!info.tunnel ? info.url : '');
  const tunnelUnavailable = info.tunnel && !publicUrl;

  serverUrl = publicUrl || localUrl || null;
  serverUrlEl.classList.toggle('server-url-error', Boolean(info.tunnelError) || tunnelUnavailable);

  if (info.tunnelError) {
    serverUrlEl.textContent = localUrl
      ? `NGROK: ОШИБКА · ЛОКАЛЬНО: ${localUrl}`
      : 'NGROK: ОШИБКА';
    serverUrlTitle = localUrl
      ? `${info.tunnelError}\nЛокальная ссылка остаётся доступна (нажмите, чтобы открыть)`
      : info.tunnelError;
  } else if (publicUrl) {
    serverUrlEl.textContent = publicUrl;
    serverUrlTitle = localUrl
      ? `Публичная ссылка (нажмите, чтобы открыть)\nЛокально: ${localUrl}`
      : 'Публичная ссылка (нажмите, чтобы открыть)';
  } else if (localUrl) {
    serverUrlEl.textContent = localUrl;
    serverUrlTitle = tunnelUnavailable
      ? 'Публичная ссылка недоступна; локальная ссылка остаётся доступна'
      : 'Локальная ссылка (нажмите, чтобы открыть)';
  } else {
    serverUrlEl.textContent = '—';
    serverUrlTitle = 'Адрес игроков пока недоступен';
  }
  serverUrlEl.title = serverUrlTitle;
});
desktopAPI.onClientCount((count) => {
  clientCountEl.textContent = count;
});
desktopAPI.onHackState((hack) => {
  state.liveHack = hack;
  renderHackStatus();
});
desktopAPI.onCoordinationState((coordination) => {
  applyCoordinationState(coordination);
  renderCoordination();
  if (state.session) {
    renderTermList();
    renderTreeHeader();
    renderHackStatus();
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
void desktopAPI.getRuntimeStatus().then(renderStartupPresentation);

serverUrlEl.addEventListener('click', async () => {
  const requestedUrl = serverUrl;
  if (!requestedUrl) return;

  // The frontend never opens a URL directly. The Go command parses the final
  // value and rejects malformed or non-HTTP(S) protocols at the privilege edge.
  const result = await desktopAPI.openUrl(requestedUrl);
  if (requestedUrl === serverUrl && result && result.ok === false) {
    const detail = result.error ? `: ${result.error}` : '';
    serverUrlEl.title = `${serverUrlTitle}\nНе удалось открыть ссылку${detail}`;
  }
});

// ── Public access: trusted settings and explicit lifecycle ──
const publicAccessStateLabels = Object.freeze({
  stopped: 'ОСТАНОВЛЕН',
  starting: 'ЗАПУСК…',
  ready: 'ГОТОВ',
  stopping: 'ОСТАНОВКА…',
  error: 'ОШИБКА',
});

const secureCredentialStoreFailures = Object.freeze({
  secret_store_locked: 'Unlock the secure credential store and try again.',
  secret_store_denied: 'Allow secure credential store access and try again.',
  secret_store_unavailable: 'The secure credential store is unavailable; local access remains available.',
});

function publicAccessFailureMessage(status) {
  return secureCredentialStoreFailures[status.errorCategory]
    || status.errorMessage
    || 'ПУБЛИЧНЫЙ ДОСТУП НЕДОСТУПЕН';
}

function renderSecretPresence(element, presence, presentLabel = 'СОХРАНЕН') {
  element.dataset.presence = presence;
  element.textContent = presence === 'present'
    ? presentLabel
    : presence === 'absent' ? 'НЕ СОХРАНЕН' : 'НЕДОСТУПЕН';
}

function publicAccessDisplayURL(preferences, status) {
  if (status.state === 'ready' && status.publicUrl) return status.publicUrl;
  if (status.state === 'error') return '';
  const reservedDomain = String(preferences.reservedDomain || '').trim();
  if (!reservedDomain) return '';
  return /^https?:\/\//i.test(reservedDomain) ? reservedDomain : `https://${reservedDomain}`;
}

function syncPublicAccessPlayerCredentialControls() {
  const configured = publicAccessSnapshot?.playerPasswordPresence === 'present';
  const transitioning = publicAccessSnapshot?.status?.state === 'starting' ||
    publicAccessSnapshot?.status?.state === 'stopping';
  const disabled = publicAccessCommandPending || transitioning;
  const username = publicAccessReplacementUsername.value.trim();
  const password = publicAccessReplacementPlayerPassword.value;
  const passwordValid = password === '' || Array.from(password).length >= 8;
  publicAccessReplacementUsername.disabled = disabled;
  publicAccessReplacementPlayerPassword.disabled = disabled;
  btnGeneratePlayerPassword.disabled = disabled;
  btnCancelPublicAccessPlayerCredentials.disabled = publicAccessCommandPending;
  btnDeletePublicAccessPlayerCredentials.hidden = !configured;
  btnDeletePublicAccessPlayerCredentials.disabled = disabled || !configured;
  btnSavePublicAccessPlayerCredentials.disabled = disabled || username === '' ||
    !passwordValid || (!configured && password === '');
  publicAccessReplacementPlayerPasswordHint.textContent = configured
    ? 'Оставьте пустым, чтобы сохранить текущий пароль.'
    : 'Введите пароль не короче 8 символов или сгенерируйте новый.';
}

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
  publicAccessSection.hidden = false;
  publicAccessSnapshot = snapshot;
  const preferences = snapshot.preferences || {};
  const status = snapshot.status || {};
  publicAccessDomain.value = preferences.reservedDomain || '';
  publicAccessUsernameSummary.textContent = preferences.username || 'players';
  const providerTokenConfigured = snapshot.providerTokenPresence === 'present';
  const playerPasswordConfigured = snapshot.playerPasswordPresence === 'present';
  publicAccessProviderSetup.hidden = providerTokenConfigured;
  publicAccessProviderConfigured.hidden = !providerTokenConfigured;
  if (providerTokenConfigured) publicAccessProviderToken.value = '';
  renderSecretPresence(publicAccessProviderPresence, snapshot.providerTokenPresence, 'НАСТРОЕН');
  renderSecretPresence(publicAccessPasswordPresence, snapshot.playerPasswordPresence);
  publicAccessPasswordPresence.hidden = playerPasswordConfigured;
  publicAccessPasswordMask.hidden = !playerPasswordConfigured;
  publicAccessPasswordMask.textContent = playerPasswordConfigured ? '*****' : '';
  publicAccessStatus.textContent = publicAccessStateLabels[status.state] || 'ЗАГРУЗКА…';
  publicAccessStatus.dataset.state = status.state || 'loading';
  publicAccessStateRow.dataset.state = status.state || 'loading';
  publicAccessStatus.dataset.generation = String(Number(status.generation || 0));
  publicAccessStatus.dataset.settingsRevision = String(Number(status.settingsRevision || preferences.revision || 0));
  const publicFailure = publicAccessFailureMessage(status);
  publicAccessError.textContent = status.state === 'error'
    ? `${publicFailure} · ЛОКАЛЬНЫЙ РЕЖИМ ПРОДОЛЖАЕТ РАБОТАТЬ`
    : '';
  publicAccessError.hidden = publicAccessError.textContent === '';
  const displayURL = publicAccessDisplayURL(preferences, status);
  publicAccessURL.textContent = displayURL || 'ПОЯВИТСЯ ПОСЛЕ ЗАПУСКА';
  publicAccessURL.dataset.available = String(Boolean(displayURL));
  btnCopyPublicURL.hidden = !displayURL;
  const transitioning = status.state === 'starting' || status.state === 'stopping';
  const disabled = publicAccessCommandPending || transitioning;
  for (const control of [
    publicAccessDomain, publicAccessProviderToken, publicAccessReplacementProviderToken,
    btnSavePublicAccess,
    btnOpenPublicAccessProviderToken, btnSavePublicAccessProviderToken,
    btnDeletePublicAccessProviderToken, btnOpenPublicAccessPlayerCredentials,
  ]) control.disabled = disabled;
  const stopping = status.state === 'ready' || status.state === 'stopping';
  btnStartPublicAccess.hidden = stopping;
  btnStopPublicAccess.hidden = !stopping;
  btnStartPublicAccess.textContent = status.state === 'starting' ? 'ЗАПУСК…' : 'ВКЛЮЧИТЬ ДОСТУП';
  btnStopPublicAccess.textContent = status.state === 'stopping' ? 'ОСТАНОВКА…' : 'ОСТАНОВИТЬ ДОСТУП';
  btnStartPublicAccess.disabled = disabled || status.state === 'ready';
  btnStopPublicAccess.disabled = disabled || status.state === 'stopped';
  btnOpenPublicAccessSettings.disabled = disabled;
  btnClosePublicAccessSettings.disabled = publicAccessCommandPending;
  btnCancelPublicAccessSettings.disabled = publicAccessCommandPending;
  btnCancelPublicAccessProviderToken.disabled = publicAccessCommandPending;
  btnSavePublicAccessProviderToken.disabled = disabled || publicAccessReplacementProviderToken.value.trim() === '';
  btnSharePublicAccessCredentials.disabled = disabled || !playerPasswordConfigured ||
    publicAccessUsernameSummary.textContent.trim() === '';
  syncPublicAccessPlayerCredentialControls();
}

function publicAccessRevision() {
  return Number(publicAccessSnapshot?.preferences?.revision || 0);
}

async function copyTransientText(value, successMessage, statusElement = publicAccessCopyStatus) {
  if (!value) return false;
  let copied = false;
  try {
    if (typeof navigator.clipboard?.writeText === 'function') {
      await navigator.clipboard.writeText(value);
      copied = true;
    }
  } catch {
    // Packaged WebViews may deny the browser Clipboard API. The native Wails
    // runtime is the bounded fallback and returns no copy of the value.
  }
  if (!copied) {
    copied = await desktopAPI.writeClipboardText(value);
  }
  if (copied) {
    statusElement.textContent = successMessage;
    return true;
  }
  statusElement.textContent = 'НЕ УДАЛОСЬ СКОПИРОВАТЬ';
  return false;
}

function showPublicAccessSettings({ setupRequired = false } = {}) {
  if (publicAccessCommandPending) return;
  publicAccessSettingsDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessSettings;
  publicAccessSetupRequired.hidden = !setupRequired;
  if (setupRequired) publicAccessGuide.open = true;
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  publicAccessSettingsCopyStatus.textContent = '';
  publicAccessSettingsDialog.hidden = false;
  if (typeof publicAccessSettingsDialog.showModal === 'function' && !publicAccessSettingsDialog.open) {
    publicAccessSettingsDialog.showModal();
  } else {
    publicAccessSettingsDialog.setAttribute('open', '');
  }
  queueMicrotask(() => btnClosePublicAccessSettings.focus());
}

function hidePublicAccessSettings() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessSettingsDialog.close === 'function' && publicAccessSettingsDialog.open) {
    publicAccessSettingsDialog.close();
  } else {
    publicAccessSettingsDialog.removeAttribute('open');
  }
  publicAccessSettingsDialog.hidden = true;
  publicAccessSetupRequired.hidden = true;
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  publicAccessSettingsCopyStatus.textContent = '';
  publicAccessProviderToken.value = '';
  renderPublicAccess(publicAccessSnapshot);
  const opener = publicAccessSettingsDialogOpener;
  publicAccessSettingsDialogOpener = null;
  if (opener?.isConnected) opener.focus();
}

function publicAccessNonSecretDraft() {
  return {
    reservedDomain: publicAccessDomain.value,
  };
}

function restorePublicAccessNonSecretDraft(draft) {
  publicAccessDomain.value = draft.reservedDomain;
}

function confirmActivePublicAccessChange() {
  return publicAccessSnapshot?.status?.state !== 'ready' || window.confirm(
    'ПУБЛИЧНЫЙ ДОСТУП АКТИВЕН. ИЗМЕНЕНИЕ ОСТАНОВИТ И ПЕРЕЗАПУСТИТ ССЫЛКУ. ПРОДОЛЖИТЬ?',
  );
}

function showPublicAccessProviderTokenDialog() {
  if (publicAccessCommandPending || publicAccessSnapshot?.providerTokenPresence !== 'present') return;
  publicAccessProviderTokenDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessProviderToken;
  publicAccessReplacementProviderToken.value = '';
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  publicAccessProviderTokenDialog.hidden = false;
  if (typeof publicAccessProviderTokenDialog.showModal === 'function' && !publicAccessProviderTokenDialog.open) {
    publicAccessProviderTokenDialog.showModal();
  } else {
    publicAccessProviderTokenDialog.setAttribute('open', '');
  }
  queueMicrotask(() => publicAccessReplacementProviderToken.focus());
}

function hidePublicAccessProviderTokenDialog() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessProviderTokenDialog.close === 'function' && publicAccessProviderTokenDialog.open) {
    publicAccessProviderTokenDialog.close();
  } else {
    publicAccessProviderTokenDialog.removeAttribute('open');
  }
  publicAccessProviderTokenDialog.hidden = true;
  publicAccessReplacementProviderToken.value = '';
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  const opener = publicAccessProviderTokenDialogOpener;
  publicAccessProviderTokenDialogOpener = null;
  if (opener?.isConnected && !opener.closest('[hidden]')) {
    opener.focus();
  } else {
    publicAccessProviderToken.focus();
  }
}

async function runPublicAccessProviderTokenMutation({ replacementProviderToken = '', deleteProviderToken = false }) {
  if (publicAccessCommandPending || !confirmActivePublicAccessChange()) return;
  const preferences = publicAccessSnapshot?.preferences || {};
  const draft = publicAccessNonSecretDraft();
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: preferences.reservedDomain || '',
    username: preferences.username || 'players',
    replacementProviderToken,
    deleteProviderToken,
    replacementPlayerPassword: '',
    deletePlayerPassword: false,
  };
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessReplacementProviderToken.value = '';
  request.replacementProviderToken = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  restorePublicAccessNonSecretDraft(draft);
  if (!result.ok) {
    publicAccessProviderTokenError.textContent = result.error || 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ТОКЕН NGROK';
    publicAccessProviderTokenError.hidden = false;
    return;
  }
  hidePublicAccessProviderTokenDialog();
}

function showPublicAccessPlayerCredentialsDialog() {
  if (publicAccessCommandPending) return;
  publicAccessPlayerCredentialsDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessPlayerCredentials;
  publicAccessReplacementUsername.value = publicAccessSnapshot?.preferences?.username || 'players';
  publicAccessReplacementPlayerPassword.value = '';
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  syncPublicAccessPlayerCredentialControls();
  publicAccessPlayerCredentialsDialog.hidden = false;
  if (typeof publicAccessPlayerCredentialsDialog.showModal === 'function' && !publicAccessPlayerCredentialsDialog.open) {
    publicAccessPlayerCredentialsDialog.showModal();
  } else {
    publicAccessPlayerCredentialsDialog.setAttribute('open', '');
  }
  queueMicrotask(() => publicAccessReplacementUsername.focus());
}

function hidePublicAccessPlayerCredentialsDialog() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessPlayerCredentialsDialog.close === 'function' && publicAccessPlayerCredentialsDialog.open) {
    publicAccessPlayerCredentialsDialog.close();
  } else {
    publicAccessPlayerCredentialsDialog.removeAttribute('open');
  }
  publicAccessPlayerCredentialsDialog.hidden = true;
  publicAccessReplacementPlayerPassword.value = '';
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  const opener = publicAccessPlayerCredentialsDialogOpener;
  publicAccessPlayerCredentialsDialogOpener = null;
  if (opener?.isConnected && !opener.closest('[hidden]')) {
    opener.focus();
  } else {
    btnOpenPublicAccessPlayerCredentials.focus();
  }
}

async function runPublicAccessPlayerCredentialMutation({
  username,
  replacementPlayerPassword = '',
  deletePlayerPassword = false,
}) {
  if (publicAccessCommandPending || !confirmActivePublicAccessChange()) return;
  const preferences = publicAccessSnapshot?.preferences || {};
  const mainDraft = publicAccessNonSecretDraft();
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: preferences.reservedDomain || '',
    username,
    replacementProviderToken: '',
    deleteProviderToken: false,
    replacementPlayerPassword,
    deletePlayerPassword,
  };
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessReplacementPlayerPassword.value = '';
  request.replacementPlayerPassword = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  restorePublicAccessNonSecretDraft(mainDraft);
  if (!result.ok) {
    publicAccessReplacementUsername.value = username;
    publicAccessPlayerCredentialsError.textContent = result.error || 'НЕ УДАЛОСЬ СОХРАНИТЬ ДАННЫЕ ИГРОКОВ';
    publicAccessPlayerCredentialsError.hidden = false;
    syncPublicAccessPlayerCredentialControls();
    return;
  }
  hidePublicAccessPlayerCredentialsDialog();
}

function showGeneratedPassword(oneTimeValue) {
  let transientValue = oneTimeValue;
  generatedPasswordValue.textContent = transientValue;
  const clearAndClose = () => {
    transientValue = '';
    generatedPasswordValue.textContent = '';
    btnCopyGeneratedPassword.onclick = null;
    btnDismissGeneratedPassword.onclick = null;
    generatedPasswordDialog.oncancel = null;
    if (generatedPasswordDialog.open) generatedPasswordDialog.close();
    btnGeneratePlayerPassword.focus();
  };
  btnCopyGeneratedPassword.onclick = async () => {
    await copyTransientText(transientValue, 'ПАРОЛЬ СКОПИРОВАН', publicAccessSettingsCopyStatus);
    clearAndClose();
  };
  btnDismissGeneratedPassword.onclick = clearAndClose;
  generatedPasswordDialog.oncancel = (event) => {
    event.preventDefault();
    clearAndClose();
  };
  generatedPasswordDialog.showModal();
  btnCopyGeneratedPassword.focus();
}

// The facade owns exact `public-access-status` event ordering and stale-snapshot suppression.
desktopAPI.onPublicAccessStatus(renderPublicAccess);

publicAccessForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  if (publicAccessCommandPending) return;
  if (!confirmActivePublicAccessChange()) return;
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: publicAccessDomain.value,
    username: publicAccessSnapshot?.preferences?.username || 'players',
    replacementProviderToken: publicAccessProviderToken.value,
    deleteProviderToken: false,
    replacementPlayerPassword: '',
    deletePlayerPassword: false,
  };
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessProviderToken.value = '';
  request.replacementProviderToken = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  if (!result.ok) {
    publicAccessDomain.value = request.reservedDomain;
    publicAccessSettingsError.textContent = result.error || 'НЕ УДАЛОСЬ СОХРАНИТЬ НАСТРОЙКИ';
    publicAccessSettingsError.hidden = false;
    return;
  }
  hidePublicAccessSettings();
});

async function runPublicAccessLifecycle(command) {
  if (publicAccessCommandPending) return;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await command({ expectedRevision: publicAccessRevision() });
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  if (!result.ok) {
    publicAccessError.textContent = result.error || 'ОПЕРАЦИЯ ПУБЛИЧНОГО ДОСТУПА НЕ ВЫПОЛНЕНА';
    publicAccessError.hidden = false;
  }
}

btnStartPublicAccess.addEventListener('click', () => {
  const configured = publicAccessSnapshot?.providerTokenPresence === 'present'
    && publicAccessSnapshot?.playerPasswordPresence === 'present';
  if (!configured) {
    showPublicAccessSettings({ setupRequired: true });
    return;
  }
  void runPublicAccessLifecycle(desktopAPI.startPublicAccess);
});
btnStopPublicAccess.addEventListener('click', () => runPublicAccessLifecycle(desktopAPI.stopPublicAccess));
btnOpenPublicAccessSettings.addEventListener('click', () => showPublicAccessSettings());
btnClosePublicAccessSettings.addEventListener('click', hidePublicAccessSettings);
btnCancelPublicAccessSettings.addEventListener('click', hidePublicAccessSettings);
publicAccessSettingsDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessSettings();
});
btnOpenPublicAccessProviderToken.addEventListener('click', showPublicAccessProviderTokenDialog);
btnCancelPublicAccessProviderToken.addEventListener('click', hidePublicAccessProviderTokenDialog);
publicAccessProviderTokenDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessProviderTokenDialog();
});
publicAccessProviderTokenForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const replacementProviderToken = publicAccessReplacementProviderToken.value;
  if (!replacementProviderToken.trim()) return;
  void runPublicAccessProviderTokenMutation({ replacementProviderToken });
});
btnDeletePublicAccessProviderToken.addEventListener('click', () => {
  void runPublicAccessProviderTokenMutation({ deleteProviderToken: true });
});
publicAccessReplacementProviderToken.addEventListener('input', () => {
  btnSavePublicAccessProviderToken.disabled = publicAccessCommandPending ||
    publicAccessReplacementProviderToken.value.trim() === '';
});
btnOpenPublicAccessPlayerCredentials.addEventListener('click', showPublicAccessPlayerCredentialsDialog);
btnCancelPublicAccessPlayerCredentials.addEventListener('click', hidePublicAccessPlayerCredentialsDialog);
publicAccessPlayerCredentialsDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessPlayerCredentialsDialog();
});
publicAccessPlayerCredentialsForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const username = publicAccessReplacementUsername.value.trim();
  const replacementPlayerPassword = publicAccessReplacementPlayerPassword.value;
  const configured = publicAccessSnapshot?.playerPasswordPresence === 'present';
  if (!username) return;
  if ((!configured && replacementPlayerPassword === '') ||
    (replacementPlayerPassword !== '' && Array.from(replacementPlayerPassword).length < 8)) {
    publicAccessPlayerCredentialsError.textContent = 'ПАРОЛЬ ДОЛЖЕН СОДЕРЖАТЬ НЕ МЕНЕЕ 8 СИМВОЛОВ';
    publicAccessPlayerCredentialsError.hidden = false;
    return;
  }
  void runPublicAccessPlayerCredentialMutation({ username, replacementPlayerPassword });
});
btnDeletePublicAccessPlayerCredentials.addEventListener('click', () => {
  void runPublicAccessPlayerCredentialMutation({ username: 'players', deletePlayerPassword: true });
});
publicAccessReplacementUsername.addEventListener('input', syncPublicAccessPlayerCredentialControls);
publicAccessReplacementPlayerPassword.addEventListener('input', syncPublicAccessPlayerCredentialControls);
btnGeneratePlayerPassword.addEventListener('click', async () => {
  if (publicAccessCommandPending) return;
  if (!confirmActivePublicAccessChange()) return;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await desktopAPI.generatePlayerPassword({ expectedRevision: publicAccessRevision() });
  publicAccessCommandPending = false;
  if (!result.ok || !result.generatedPassword) {
    renderPublicAccess(publicAccessSnapshot);
    publicAccessPlayerCredentialsError.textContent = result.error || 'НЕ УДАЛОСЬ СОЗДАТЬ ПАРОЛЬ';
    publicAccessPlayerCredentialsError.hidden = false;
    return;
  }
  showGeneratedPassword(result.generatedPassword);
  const refreshed = await desktopAPI.getPublicAccess();
  renderPublicAccess(refreshed);
});
btnCopyPublicURL.addEventListener('click', () => {
  if (publicAccessURL.dataset.available !== 'true') return;
  void copyTransientText(publicAccessURL.textContent, 'URL СКОПИРОВАН');
});
btnSharePublicAccessCredentials.addEventListener('click', async () => {
  if (publicAccessCommandPending || publicAccessSnapshot?.playerPasswordPresence !== 'present') return;
  publicAccessSettingsCopyStatus.textContent = '';
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await desktopAPI.copyPublicAccessCredentials();
  publicAccessCommandPending = false;
  renderPublicAccess(publicAccessSnapshot);
  publicAccessSettingsCopyStatus.textContent = result.ok
    ? 'ЛОГИН И ПАРОЛЬ СКОПИРОВАНЫ'
    : (result.error || 'НЕ УДАЛОСЬ СКОПИРОВАТЬ ДАННЫЕ ИГРОКОВ');
});
for (const link of publicAccessNgrokDocLinks) {
  link.addEventListener('click', () => {
    void desktopAPI.openUrl(link.dataset.ngrokDocUrl);
  });
}
function loadSession(session, filePath) {
  saveGeneration++;
  saveInvocation = 0;
  latestRenderedSave = 0;
  newestDurableRevision = 0;
  delete saveStatus.dataset.savedRevision;
  saveStatus.textContent = '';
  saveStatus.classList.remove('err');
  state.session        = session;
  state.filePath        = filePath;
  state.liveTerminalId  = state.coordination?.broadcast?.activeTerminalId || null;
  state.editTerminalId  = (session.terminals[0] && session.terminals[0].id) || null;
  state.selectedNodeId  = null;
  state.expanded         = new Set(['root']);
  state.collapsedTerminalGroupIds = new Set();
  state.liveHack         = null;

  sessionFileLabel.textContent = filePath;
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
function escHtml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
function escAttr(s) {
  return escHtml(s).replace(/"/g, '&quot;');
}

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
  renderSettingsPanel();
  renderNodeForm();
  try {
    const result = await command();
    renderSessionStateResult(result, successMessage, acceptsCanonicalResult);
  } catch (error) {
    saveStatus.textContent = 'Ошибка изменения состояния: '
      + (error instanceof Error ? error.message : String(error));
    saveStatus.classList.add('err');
  } finally {
    sessionStateCommandPending = false;
    renderSettingsPanel();
    renderNodeForm();
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
  renderTermList();
  renderTreeHeader();
  renderSettingsPanel();
  renderToolbarState();
  renderTree();
  renderNodeForm();
  renderToolbarHint();
  renderHackStatus();
  renderCoordination();
}

// ── Render: authoritative roster and broadcast state ────────
function renderCoordination() {
  const coordination = state.coordination;
  const sessions = Array.isArray(coordination?.sessions) ? coordination.sessions : [];
  const broadcast = coordination?.broadcast || null;
  const playerConfig = coordination?.playerConfig || null;
  broadcastSummary.textContent = broadcast
    ? (broadcast.activeTerminalId
      ? `ТРАНСЛЯЦИЯ АКТИВНА · ТЕРМИНАЛ ${broadcast.activeTerminalId}`
      : `ТРАНСЛЯЦИЯ АКТИВНА · ОЖИДАНИЕ ТЕРМИНАЛА · ${broadcast.id}`)
    : 'ТРАНСЛЯЦИЯ НЕ ЗАПУЩЕНА';
  broadcastSummary.classList.toggle('is-live', Boolean(broadcast));
  coordinationPanel.dataset.playerConfigActive = String(Boolean(playerConfig));
  btnStartBroadcast.disabled = coordinationCommandPending || Boolean(broadcast) || !playerConfig;
  btnEndBroadcast.hidden = !broadcast;
  btnEndBroadcast.disabled = coordinationCommandPending || !broadcast;
  activeLogicalSessionCount.textContent = String(sessions.filter(session => Boolean(session.connected)).length);

  coordinationProjection = {
    coordination,
    kind: 'coordination-state',
    pending: coordinationCommandPending,
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
    if (coordinationError.dataset.kind === 'terminal-navigation') {
      coordinationError.dataset.kind = '';
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
  coordinationError.dataset.kind = 'terminal-navigation';
  setCoordinationStatus(`${labels[notice.reason] || 'ПЕРЕХОД БОЛЬШЕ НЕ ДЕЙСТВИТЕЛЕН'}${detail ? ` · ${detail}` : ''}`, true);
}

function setCoordinationStatus(message, isError = false) {
  coordinationStatus.textContent = isError ? '' : (message || '');
  coordinationStatus.classList.remove('err');
  coordinationError.textContent = isError ? (message || '') : '';
  coordinationError.hidden = !isError || !message;
}

async function runCoordinationCommand(command, successMessage, pendingMessage) {
  if (coordinationCommandPending) return null;
  coordinationCommandPending = true;
  setCoordinationStatus(pendingMessage || 'ВЫПОЛНЕНИЕ ОПЕРАЦИИ...');
  renderCoordination();
  if (state.session) renderTreeHeader();
  renderHackStatus();
  let result;
  try {
    result = await command();
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  renderHackStatus();
  if (state.session) renderTreeHeader();
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
  if (result.status === 'activated' || result.status === 'cleared') state.liveHack = null;
  setCoordinationStatus(message.decision === 'cancel' ? 'ПЕРЕКЛЮЧЕНИЕ ОТМЕНЕНО' : 'РЕШЕНИЕ ПРИМЕНЕНО');
  renderAll();
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

function applyVueRequest(message) {
  if (message?.kind === 'session-document-loaded' && message.session && typeof message.session === 'object') {
    loadSession(message.session, typeof message.filePath === 'string' ? message.filePath : '');
    return;
  }
  if (applyPlayerManagementRequest(message)) return;
  if (applyPlayerConfigurationRequest(message)) return;
  if (applyLogicalSessionRequest(message)) return;
  applyTerminalSwitchResolution(message);
}

let releaseLegacyBridge = null;
function attachOverseerLegacyBridge(bridge) {
  releaseLegacyBridge?.();
  releaseLegacyBridge = null;
  if (!bridge || typeof bridge.subscribeVueRequests !== 'function') return;
  releaseLegacyBridge = bridge.subscribeVueRequests(applyVueRequest);
  if (startupProjection) bridge.legacyToVue(startupProjection);
  if (coordinationProjection) bridge.legacyToVue(coordinationProjection);
}

Object.defineProperty(globalThis, '__attachOverseerLegacyBridge', {
  configurable: true,
  value: attachOverseerLegacyBridge,
});
attachOverseerLegacyBridge(globalThis.__overseerCoexistenceBridge);

function showEndBroadcastConfirmation() {
  endBroadcastDialog.hidden = false;
  if (typeof endBroadcastDialog.showModal === 'function' && !endBroadcastDialog.open) {
    endBroadcastDialog.showModal();
  } else {
    endBroadcastDialog.setAttribute('open', '');
  }
  btnCancelEndBroadcast.focus();
}

function hideEndBroadcastConfirmation({ restoreFocus = true } = {}) {
  endBroadcastDialog.hidden = true;
  if (typeof endBroadcastDialog.close === 'function' && endBroadcastDialog.open) {
    endBroadcastDialog.close();
  } else {
    endBroadcastDialog.removeAttribute('open');
  }
  btnCancelEndBroadcast.disabled = false;
  btnConfirmEndBroadcast.disabled = false;
  if (restoreFocus && !btnEndBroadcast.hidden) btnEndBroadcast.focus();
}

function showTakeOffAirConfirmation() {
  if (takeOffAirPending || !state.coordination?.broadcast || !state.liveTerminalId) return;
  takeOffAirError.textContent = '';
  takeOffAirError.hidden = true;
  btnCancelTakeOffAir.disabled = false;
  btnConfirmTakeOffAir.disabled = false;
  takeOffAirDialog.hidden = false;
  if (typeof takeOffAirDialog.showModal === 'function' && !takeOffAirDialog.open) {
    takeOffAirDialog.showModal();
  } else {
    takeOffAirDialog.setAttribute('open', '');
  }
  btnCancelTakeOffAir.focus();
}

function hideTakeOffAirConfirmation({ restoreFocus = true } = {}) {
  if (typeof takeOffAirDialog.close === 'function' && takeOffAirDialog.open) {
    takeOffAirDialog.close();
  } else {
    takeOffAirDialog.removeAttribute('open');
  }
  takeOffAirDialog.hidden = true;
  takeOffAirPending = false;
  btnCancelTakeOffAir.disabled = false;
  btnConfirmTakeOffAir.disabled = false;
  if (restoreFocus && !btnStopBroadcast.hidden) btnStopBroadcast.focus();
}

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

function renderTermList() {
  termList.innerHTML = '';
  if (!state.session.terminals.length) {
    termList.innerHTML = '<div class="tree-empty-hint">Нет терминалов</div>';
    return;
  }

  ensureSessionTerminalGroups();
  const currentGroupIDs = new Set(state.session.terminalGroups.map(group => group.id));
  for (const groupID of state.collapsedTerminalGroupIds) {
    if (!currentGroupIDs.has(groupID)) state.collapsedTerminalGroupIds.delete(groupID);
  }
  const terminalsByID = new Map(state.session.terminals.map(term => [term.id, term]));
  state.session.terminalGroups.forEach((group, groupIndex) => {
    const collapsed = state.collapsedTerminalGroupIds.has(group.id);
    const groupEl = document.createElement('section');
    groupEl.className = 'terminal-group' + (group.terminalIds.length === 1 ? ' is-singleton' : '');
    groupEl.dataset.groupId = group.id;
    groupEl.dataset.singleton = String(group.terminalIds.length === 1);
    groupEl.dataset.collapsed = String(collapsed);
    groupEl.setAttribute('role', 'listitem');

    const header = document.createElement('header');
    header.className = 'terminal-group-header';

    const members = document.createElement('div');
    members.className = 'terminal-group-members';
    members.id = `terminal-group-members-${groupIndex}`;
    members.hidden = collapsed;
    members.setAttribute('role', 'list');
    members.setAttribute('aria-label', `Терминалы группы ${group.name}`);

    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'terminal-group-toggle';
    toggle.dataset.action = 'toggle-terminal-group';
    toggle.setAttribute('aria-controls', members.id);
    toggle.setAttribute('aria-expanded', String(!collapsed));
    toggle.setAttribute('aria-label', `${collapsed ? 'Развернуть' : 'Свернуть'} группу ${group.name}`);

    const caret = document.createElement('span');
    caret.className = 'terminal-group-caret';
    caret.setAttribute('aria-hidden', 'true');
    caret.textContent = collapsed ? '▸' : '▾';
    const heading = document.createElement('span');
    heading.className = 'terminal-group-name';
    heading.textContent = group.name;
    toggle.append(caret, heading);
    toggle.addEventListener('click', () => {
      const nextCollapsed = !state.collapsedTerminalGroupIds.has(group.id);
      if (nextCollapsed) state.collapsedTerminalGroupIds.add(group.id);
      else state.collapsedTerminalGroupIds.delete(group.id);
      members.hidden = nextCollapsed;
      groupEl.dataset.collapsed = String(nextCollapsed);
      caret.textContent = nextCollapsed ? '▸' : '▾';
      toggle.setAttribute('aria-expanded', String(!nextCollapsed));
      toggle.setAttribute('aria-label', `${nextCollapsed ? 'Развернуть' : 'Свернуть'} группу ${group.name}`);
    });

    const memberCount = document.createElement('span');
    memberCount.className = 'terminal-group-member-count';
    memberCount.textContent = terminalCountLabel(group.terminalIds.length);

    const groupActions = terminalActionMenu({
      scope: 'terminal-group',
      ownerID: group.id,
      ownerName: group.name,
      items: [
        { label: 'ПЕРЕИМЕНОВАТЬ ГРУППУ', action: 'rename-terminal-group', handler: () => showTerminalGroupRename(group.id) },
        { label: 'ПЕРЕМЕСТИТЬ ГРУППУ ВВЕРХ', action: 'move-terminal-group-up', handler: () => prepareTerminalGroupOrder(group.id, -1), disabled: groupIndex === 0 },
        { label: 'ПЕРЕМЕСТИТЬ ГРУППУ ВНИЗ', action: 'move-terminal-group-down', handler: () => prepareTerminalGroupOrder(group.id, 1), disabled: groupIndex === state.session.terminalGroups.length - 1 },
        { label: 'РАСФОРМИРОВАТЬ ГРУППУ', action: 'dissolve-terminal-group', handler: () => prepareTerminalGroupDissolution(group.id), destructive: true },
      ],
    });
    header.append(toggle, memberCount, groupActions);
    groupEl.append(header, members);

    group.terminalIds.forEach((terminalID, memberIndex) => {
      const term = terminalsByID.get(terminalID);
      if (term) members.appendChild(buildTerminalRow(term, group, memberIndex));
    });
    termList.appendChild(groupEl);
  });
}

function terminalCountLabel(count) {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return `${count} ТЕРМИНАЛ`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${count} ТЕРМИНАЛА`;
  return `${count} ТЕРМИНАЛОВ`;
}

function terminalActionMenu({ scope, ownerID, ownerName, items }) {
  const menu = document.createElement('details');
  menu.className = 'terminal-action-menu';
  menu.addEventListener('click', event => event.stopPropagation());
  menu.addEventListener('toggle', () => {
    if (!menu.open) return;
    for (const other of termList.querySelectorAll('.terminal-action-menu[open]')) {
      if (other !== menu) other.open = false;
    }
  });

  const trigger = document.createElement('summary');
  trigger.className = 'terminal-action-menu-trigger';
  trigger.dataset.actionMenuTrigger = scope;
  trigger.dataset.actionMenuOwnerId = ownerID;
  trigger.setAttribute('aria-label', `Действия: ${ownerName}`);
  trigger.textContent = '•••';

  const panel = document.createElement('div');
  panel.className = 'terminal-action-menu-panel';
  panel.setAttribute('role', 'menu');
  panel.setAttribute('aria-label', `Действия: ${ownerName}`);
  for (const item of items) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'btn btn-mini terminal-action-menu-item'
      + (item.destructive ? ' btn-danger terminal-action-menu-destructive' : '');
    button.textContent = item.label;
    button.dataset.action = item.action;
    button.disabled = Boolean(item.disabled);
    button.setAttribute('role', 'menuitem');
    button.addEventListener('click', event => {
      event.stopPropagation();
      terminalGroupDialogOpener = { scope, ownerID };
      menu.open = false;
      item.handler();
      if (!terminalGroupDraftDialog.open && !terminalGroupImpactDialog.open) {
        terminalGroupDialogOpener = null;
        if (document.activeElement === button) trigger.focus();
      }
    });
    panel.appendChild(button);
  }
  menu.append(trigger, panel);
  return menu;
}

function closeTerminalActionMenus({ restoreFocus = false } = {}) {
  const openMenu = termList.querySelector('.terminal-action-menu[open]');
  if (!openMenu) return false;
  const trigger = openMenu.querySelector('.terminal-action-menu-trigger');
  openMenu.open = false;
  if (restoreFocus) trigger?.focus();
  return true;
}

legacyOverseerRoot.addEventListener('click', event => {
  if (!event.target.closest('.terminal-action-menu')) closeTerminalActionMenus();
});
legacyOverseerRoot.addEventListener('keydown', event => {
  if (event.key === 'Escape' && closeTerminalActionMenus({ restoreFocus: true })) event.preventDefault();
});

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

function setTerminalGroupImpactError(message = '', canAmend = false) {
  terminalGroupImpactError.textContent = message;
  terminalGroupImpactError.hidden = !message;
  amendTerminalGroupChangeButton.hidden = !canAmend;
}

function showTerminalGroupDialog(dialog, focusTarget) {
  if (!terminalGroupDialogOpener) {
    terminalGroupDialogOpener = { element: document.activeElement };
  }
  dialog.hidden = false;
  if (typeof dialog.showModal === 'function' && !dialog.open) dialog.showModal();
  else dialog.setAttribute('open', '');
  focusTarget?.focus();
}

function hideTerminalGroupDialog(dialog) {
  if (dialog.open && typeof dialog.close === 'function') dialog.close();
  else dialog.removeAttribute('open');
  dialog.hidden = true;
}

function restoreTerminalGroupDialogFocus() {
  const opener = terminalGroupDialogOpener;
  terminalGroupDialogOpener = null;
  if (!opener) return;
  if (opener.element?.isConnected) {
    opener.element.focus();
    return;
  }
  const trigger = [...termList.querySelectorAll(`[data-action-menu-trigger="${opener.scope}"]`)]
    .find(candidate => candidate.dataset.actionMenuOwnerId === opener.ownerID);
  trigger?.focus();
}

function closeTerminalGroupDraft({ restoreFocus = true } = {}) {
  terminalGroupDraft = null;
  terminalGroupDraftForm.reset();
  hideTerminalGroupDialog(terminalGroupDraftDialog);
  if (restoreFocus) restoreTerminalGroupDialogFocus();
}

function closeTerminalGroupImpact({ restoreFocus = true } = {}) {
  pendingTerminalGroupImpact = null;
  hideTerminalGroupDialog(terminalGroupImpactDialog);
  if (restoreFocus) restoreTerminalGroupDialogFocus();
}

function populateTerminalChoices(selectedIDs = []) {
  terminalGroupTerminalChoices.innerHTML = '';
  const selected = new Set(selectedIDs);
  for (const terminal of state.session.terminals) {
    const fragment = terminalGroupTerminalChoiceTemplate.content.cloneNode(true);
    const input = fragment.querySelector('input[name="terminalIds"]');
    input.value = terminal.id;
    input.checked = selected.has(terminal.id);
    fragment.querySelector('.terminal-group-terminal-choice-name').textContent = terminal.name;
    terminalGroupTerminalChoices.appendChild(fragment);
  }
}

function configureTerminalGroupDraft(mode) {
  const terminalChoicesFieldset = legacyOverseerRoot.querySelector('#terminalGroupTerminalChoices');
  const destinationLabel = legacyOverseerRoot.querySelector('label[for="terminalGroupDestinationSelect"]');
  const reviewButton = terminalGroupDraftDialog.querySelector('[data-action="review-terminal-group-change"]');
  const renameButton = terminalGroupDraftDialog.querySelector('[data-action="save-terminal-group-rename"]');
  terminalChoicesFieldset.hidden = mode !== 'create';
  terminalGroupNameInput.hidden = mode === 'move';
  legacyOverseerRoot.querySelector('label[for="terminalGroupNameInput"]').hidden = mode === 'move';
  terminalGroupDestinationSelect.hidden = mode !== 'move';
  destinationLabel.hidden = mode !== 'move';
  reviewButton.hidden = mode === 'rename';
  renameButton.hidden = mode !== 'rename';
}

function showTerminalGroupCreate() {
  if (!state.session) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'create' };
  configureTerminalGroupDraft('create');
  populateTerminalChoices();
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalGroupRename(groupID) {
  const group = state.session.terminalGroups.find(candidate => candidate.id === groupID);
  if (!group) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'rename', groupID };
  configureTerminalGroupDraft('rename');
  terminalGroupNameInput.value = group.name;
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalMoveDraft(terminalID) {
  const source = state.session.terminalGroups.find(group => group.terminalIds.includes(terminalID));
  if (!source) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'move', terminalID, sourceGroupID: source.id };
  configureTerminalGroupDraft('move');
  terminalGroupDestinationSelect.innerHTML = '<option value="">ВЫБЕРИТЕ ГРУППУ</option>';
  if (source.terminalIds.length > 1) {
    const singletonOption = document.createElement('option');
    singletonOption.value = 'new-singleton';
    singletonOption.dataset.newSingleton = 'true';
    singletonOption.textContent = 'НОВАЯ ОДИНОЧНАЯ ГРУППА';
    terminalGroupDestinationSelect.appendChild(singletonOption);
  }
  for (const group of state.session.terminalGroups) {
    if (group.id === source.id) continue;
    const option = document.createElement('option');
    option.value = group.id;
    option.textContent = group.name;
    terminalGroupDestinationSelect.appendChild(option);
  }
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupDestinationSelect);
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

function reviewTerminalGroupDraft() {
  if (!terminalGroupDraft || !state.session) return;
  const before = structuredClone(state.session.terminalGroups);
  if (terminalGroupDraft.kind === 'create') {
    const name = terminalGroupNameInput.value.trim();
    const terminalIDs = [...terminalGroupDraftDialog.querySelectorAll('input[name="terminalIds"]:checked')]
      .map(input => input.value);
    if (!name || terminalIDs.length < 2) {
      setTerminalGroupError('УКАЖИТЕ УНИКАЛЬНОЕ НАЗВАНИЕ И ВЫБЕРИТЕ НЕ МЕНЕЕ ДВУХ ТЕРМИНАЛОВ');
      return;
    }
    const selected = new Set(terminalIDs);
    const sourceNames = before.filter(group => group.terminalIds.some(id => selected.has(id))).map(group => group.name);
    const candidate = before
      .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => !selected.has(id)) }))
      .filter(group => group.terminalIds.length);
    if (candidate.some(group => group.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
      setTerminalGroupError('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ');
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
    const destinationGroupID = terminalGroupDestinationSelect.value;
    const sourceGroupID = terminalGroupDraft.sourceGroupID;
    const terminalID = terminalGroupDraft.terminalID;
    if (!destinationGroupID) {
      setTerminalGroupError('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ');
      return;
    }
    const candidate = before
      .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => id !== terminalID) }))
      .filter(group => group.terminalIds.length);
    const splitToSingleton = terminalGroupDestinationSelect.selectedOptions[0]?.dataset.newSingleton === 'true';
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
      setTerminalGroupError('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ');
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
  return Number(saveStatus.dataset.savedRevision || startupStatus?.savedRevision || newestDurableRevision || 0);
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

  pendingTerminalGroupImpact = null;
  hideTerminalGroupDialog(terminalGroupImpactDialog);
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'create' };
  configureTerminalGroupDraft('create');
  populateTerminalChoices(selectedTerminalIDs);
  terminalGroupNameInput.value = preferredGroup?.name || '';
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalGroupImpact(impact) {
  setTerminalGroupError();
  setTerminalGroupImpactError();
  closeTerminalGroupDraft({ restoreFocus: false });
  pendingTerminalGroupImpact = {
    ...impact,
    expectedSessionRevision: currentSessionRevision(),
    expectedCoordinationRevision: coordinationRevision(state.coordination),
  };
  const values = {
    kind: impact.kind,
    groups: impact.affectedGroupNames.join(' · ') || '—',
    terminals: impact.affectedTerminalIDs.map(terminalName).join(' · ') || '—',
    'source-group': groupName(state.session.terminalGroups, impact.sourceGroupID),
    'destination-group': impact.destinationGroupName || groupName(state.session.terminalGroups, impact.destinationGroupID),
    membership: terminalGroupCandidateMembership(impact.candidate) || '—',
    'order-before': (impact.orderBefore || []).join(' → ') || '—',
    'order-after': (impact.orderAfter || []).join(' → ') || '—',
  };
  for (const [key, value] of Object.entries(values)) {
    terminalGroupImpactSummary.querySelector(`[data-impact="${key}"]`).textContent = value;
  }
  showTerminalGroupDialog(
    terminalGroupImpactDialog,
    terminalGroupImpactDialog.querySelector('[data-action="cancel-terminal-group-change"]'),
  );
}

async function submitTerminalGroupCandidate(candidate, expectedSessionRevision, expectedCoordinationRevision) {
  if (terminalGroupSubmitting) return false;
  terminalGroupSubmitting = true;
  const controls = terminalGroupImpactDialog.querySelectorAll('button');
  controls.forEach(control => { control.disabled = true; });
  const result = await desktopAPI.replaceTerminalGroups({
    terminalGroups: structuredClone(candidate), expectedSessionRevision, expectedCoordinationRevision,
  });
  terminalGroupSubmitting = false;
  controls.forEach(control => { control.disabled = false; });
  if (result?.session) state.session = result.session;
  const revision = Number(result?.sessionRevision || 0);
  newestDurableRevision = Math.max(newestDurableRevision, revision);
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  if (result?.coordinationState) applyCoordinationState(result.coordinationState);
  if (!result?.ok) {
    const error = result?.error || 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ГРУППЫ ТЕРМИНАЛОВ';
    setTerminalGroupError(error);
    const rejections = authoredTransitionRejections(error);
    if (pendingTerminalGroupImpact && rejections.length) {
      pendingTerminalGroupImpact.rejections = rejections;
      setTerminalGroupImpactError(
        actionableTransitionRejection(rejections, pendingTerminalGroupImpact.candidate),
        true,
      );
    }
    renderAll();
    return false;
  }
  setTerminalGroupError();
  saveStatus.textContent = `ГРУППЫ СОХРАНЕНЫ · ревизия ${revision}`;
  saveStatus.classList.remove('err');
  renderAll();
  return true;
}

async function saveTerminalGroupRename() {
  const group = state.session.terminalGroups.find(candidate => candidate.id === terminalGroupDraft?.groupID);
  const name = terminalGroupNameInput.value.trim();
  if (!group || !name) {
    setTerminalGroupError('НАЗВАНИЕ ГРУППЫ НЕ ДОЛЖНО БЫТЬ ПУСТЫМ');
    return;
  }
  if (state.session.terminalGroups.some(candidate => candidate.id !== group.id && candidate.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
    setTerminalGroupError('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ');
    return;
  }
  const candidate = structuredClone(state.session.terminalGroups);
  candidate.find(item => item.id === group.id).name = name;
  const ok = await submitTerminalGroupCandidate(candidate, currentSessionRevision(), coordinationRevision(state.coordination));
  if (ok) closeTerminalGroupDraft();
}

btnCreateTerminalGroup.addEventListener('click', showTerminalGroupCreate);
terminalGroupDraftDialog.querySelector('[data-action="review-terminal-group-change"]').addEventListener('click', reviewTerminalGroupDraft);
terminalGroupDraftDialog.querySelector('[data-action="save-terminal-group-rename"]').addEventListener('click', () => { void saveTerminalGroupRename(); });
for (const action of ['close-terminal-group-draft', 'cancel-terminal-group-draft']) {
  terminalGroupDraftDialog.querySelector(`[data-action="${action}"]`).addEventListener('click', () => closeTerminalGroupDraft());
}
for (const action of ['close-terminal-group-change', 'cancel-terminal-group-change']) {
  terminalGroupImpactDialog.querySelector(`[data-action="${action}"]`).addEventListener('click', () => closeTerminalGroupImpact());
}
terminalGroupImpactDialog.querySelector('[data-action="confirm-terminal-group-change"]').addEventListener('click', async () => {
  const impact = pendingTerminalGroupImpact;
  if (!impact || terminalGroupSubmitting) return;
  const ok = await submitTerminalGroupCandidate(impact.candidate, impact.expectedSessionRevision, impact.expectedCoordinationRevision);
  if (ok || !impact.rejections?.length) closeTerminalGroupImpact();
  else amendTerminalGroupChangeButton.focus();
});
amendTerminalGroupChangeButton.addEventListener('click', amendRejectedTerminalGroupImpact);
terminalGroupDraftDialog.addEventListener('cancel', event => { event.preventDefault(); closeTerminalGroupDraft(); });
terminalGroupImpactDialog.addEventListener('cancel', event => { event.preventDefault(); closeTerminalGroupImpact(); });

function buildTerminalRow(term, group, memberIndex) {
  const row = document.createElement('div');
  const selected = term.id === state.editTerminalId;
  row.className = 'term-row'
    + (selected ? ' editing' : '')
    + (term.id === state.liveTerminalId ? ' is-live' : '');
  row.dataset.terminalId = term.id;
  row.tabIndex = 0;
  row.setAttribute('role', 'listitem');
  row.setAttribute('aria-current', String(selected));

  const nameRow = document.createElement('div');
  nameRow.className = 'term-row-name';
  nameRow.textContent = term.name;
  row.appendChild(nameRow);

  const metaRow = document.createElement('div');
  metaRow.className = 'term-row-meta';
  metaRow.textContent = '● В ЭФИРЕ';
  row.appendChild(metaRow);

  const actions = terminalActionMenu({
    scope: 'terminal',
    ownerID: term.id,
    ownerName: term.name,
    items: [
      { label: 'ПЕРЕИМЕНОВАТЬ ТЕРМИНАЛ', action: 'rename-terminal', handler: () => startRenameTerminal(term, nameRow) },
      { label: 'ПЕРЕМЕСТИТЬ В ДРУГУЮ ГРУППУ', action: 'move-terminal', handler: () => showTerminalMoveDraft(term.id) },
      { label: 'ПЕРЕМЕСТИТЬ ТЕРМИНАЛ ВВЕРХ', action: 'move-terminal-up', handler: () => prepareTerminalMemberOrder(group.id, term.id, -1), disabled: memberIndex === 0 },
      { label: 'ПЕРЕМЕСТИТЬ ТЕРМИНАЛ ВНИЗ', action: 'move-terminal-down', handler: () => prepareTerminalMemberOrder(group.id, term.id, 1), disabled: memberIndex === group.terminalIds.length - 1 },
      { label: 'УДАЛИТЬ ТЕРМИНАЛ', action: 'delete-terminal', handler: () => deleteTerminal(term), destructive: true },
    ],
  });
  row.appendChild(actions);

  const selectTerminal = () => {
    if (state.editTerminalId === term.id) return;
    state.editTerminalId = term.id;
    state.selectedNodeId = null;
    state.expanded = new Set(['root']);
    renderAll();
  };
  row.addEventListener('click', selectTerminal);
  row.addEventListener('keydown', event => {
    if (event.target !== row || (event.key !== 'Enter' && event.key !== ' ')) return;
    event.preventDefault();
    selectTerminal();
  });
  return row;
}

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

function startRenameTerminal(term, nameRow) {
  const input = document.createElement('input');
  input.className = 'field-input';
  input.value = term.name;
  nameRow.replaceWith(input);
  input.focus();
  input.select();

  const commit = () => {
    const val = input.value.trim();
    if (val) {
      term.name = val;
      // Renaming does not re-broadcast: doing so via terminal:set-live would
      // regenerate an in-progress hack board. The new name reaches players
      // next time the terminal is (re)made live.
      autosave();
    }
    renderAll();
  };
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') input.blur();
    if (e.key === 'Escape') renderAll();
  });
  input.addEventListener('blur', commit);
}

// ── Render: tree header + toolbar state ──────────────────────
function renderTreeHeader() {
  const term = getEditTerminal();
  editingTermName.textContent = term ? term.name : '—';
  const isLive = !!term && term.id === state.liveTerminalId;
  const broadcastActive = Boolean(state.coordination?.broadcast);
  liveFlag.hidden = !isLive;
  btnMakeLive.hidden = isLive;
  btnMakeLive.textContent = 'СДЕЛАТЬ АКТИВНЫМ';
  btnMakeLive.disabled = !term || !broadcastActive || coordinationCommandPending;
  terminalSettingsMenu.hidden = !isLive;
  if (!isLive) terminalSettingsMenu.open = false;
  btnReapplySettings.disabled = !isLive || coordinationCommandPending;
  btnPublish.hidden = !isLive;
  btnPublish.disabled = !isLive || coordinationCommandPending;
  btnPublish.setAttribute('aria-busy', String(isLive && coordinationCommandPending));
  btnStopBroadcast.hidden = !broadcastActive || !state.liveTerminalId;
  btnStopBroadcast.disabled = !broadcastActive || !state.liveTerminalId || coordinationCommandPending;
}

// ── Render: per-terminal settings (hack level / intro text) ──
function renderSettingsPanel() {
  const term = getEditTerminal();
  hackLevelSelect.disabled = !term;
  introTextArea.disabled   = !term;
  btnApplySettings.disabled = !term;
  hackLevelSelect.value = term ? String(term.hackLevel || 0) : '0';
  introTextArea.value   = term ? (term.introText || '') : '';
  const completedCount = term?.commandStates && typeof term.commandStates === 'object'
    ? Object.keys(term.commandStates).length
    : 0;
  commandStateActions.hidden = !term;
  btnResetTerminalCommandStates.disabled = !term || completedCount === 0 || sessionStateCommandPending;
}

btnResetTerminalCommandStates.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term || sessionStateCommandPending) return;
  if (!await confirmCommandStateReset(`Сбросить все выполненные состояния команд терминала "${term.name}"?`)) return;
  const revisionBeforeReset = newestDurableRevision;
  runSessionStateCommand(
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
      return revision > revisionBeforeReset && canonicalTerminal &&
        Object.keys(canonicalTerminal.commandStates ?? {}).length === 0;
    }
  );
});

// ── Render: live hack status (term panel footer) ──────────────
function renderHackStatus() {
  const liveTerm = state.session && state.liveTerminalId
    ? state.session.terminals.find(t => t.id === state.liveTerminalId)
    : null;

  if (!liveTerm || !liveTerm.hackLevel || !state.liveHack) {
    hackStatus.style.display = 'none';
    btnResetFailedHack.hidden = true;
    return;
  }

  hackStatus.style.display = '';
  const h = state.liveHack;
  if (h.solved) {
    hackStatusLine.textContent = 'ВЗЛОМ: ПРОЙДЕН';
  } else if (h.failed) {
    hackStatusLine.textContent = 'ВЗЛОМ: ЗАБЛОКИРОВАН';
  } else {
    hackStatusLine.textContent = `ВЗЛОМ: осталось попыток ${h.attemptsLeft}/${h.attemptsMax}`;
  }
  btnHackSuccess.disabled = h.solved || h.failed;
  btnHackSuccess.hidden = h.failed;
  btnResetFailedHack.hidden = !h.failed;
  btnResetFailedHack.disabled = !h.failed || coordinationCommandPending;
}

function renderToolbarState() {
  const term = getEditTerminal();
  const disabled = !term;
  btnAddFolder.disabled  = disabled;
  btnAddCommand.disabled = disabled;
  btnAddEntry.disabled   = disabled;
}

function renderToolbarHint() {
  const target = currentAddTarget();
  toolbarHint.textContent = target ? `Добавление в: ${target.id === 'root' ? 'ROOT' : target.name}` : '';
}

// ── Render: tree view ─────────────────────────────────────────
function renderTree() {
  const term = getEditTerminal();
  treeView.innerHTML = '';
  if (!term) {
    treeView.innerHTML = '<div class="tree-empty-hint">Нет терминала — создайте его слева</div>';
    return;
  }
  treeView.appendChild(renderNode(term.root, true));
}

function renderNode(node, isRoot) {
  const term = getEditTerminal();
  const wrap = document.createElement('div');
  wrap.className = 'tree-node';

  const row = document.createElement('div');
  const completed = commandExecutionState(term, node.id);
  row.className = 'tree-row'
    + (state.selectedNodeId === node.id ? ' selected' : '')
    + (completed ? ' command-completed' : '');

  const hasChildren = node.type === 'folder' && node.children && node.children.length > 0;
  const isExpanded   = state.expanded.has(node.id);

  const caret = document.createElement('span');
  caret.className = 'tree-caret';
  if (node.type === 'folder') {
    caret.textContent = hasChildren ? (isExpanded ? '▾' : '▸') : '·';
    caret.addEventListener('click', (e) => {
      e.stopPropagation();
      if (!hasChildren) return;
      if (isExpanded) state.expanded.delete(node.id); else state.expanded.add(node.id);
      renderTree();
    });
  }
  row.appendChild(caret);

  const icon = document.createElement('span');
  icon.className = 'tree-icon ' + node.type;
  icon.textContent = node.type === 'folder' ? '[ПАПКА]' : node.type === 'command' ? '[КОМАНДА]' : '[ЗАПИСЬ]';
  row.appendChild(icon);

  const label = document.createElement('span');
  label.className = 'tree-label';
  label.textContent = isRoot ? 'ROOT' : effectiveNodeName(term, node);
  row.appendChild(label);

  row.addEventListener('click', () => {
    state.selectedNodeId = node.id;
    if (node.type === 'folder') state.expanded.add(node.id);
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });

  wrap.appendChild(row);

  if (node.type === 'folder' && isExpanded) {
    const childrenWrap = document.createElement('div');
    childrenWrap.className = 'tree-children';
    if (!node.children || node.children.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'tree-empty-hint';
      empty.textContent = '(пусто)';
      childrenWrap.appendChild(empty);
    } else {
      node.children.forEach(child => childrenWrap.appendChild(renderNode(child, false)));
    }
    wrap.appendChild(childrenWrap);
  }

  return wrap;
}

// ── Render: node property form ────────────────────────────────
function renderNodeForm() {
  const term = getEditTerminal();
  if (!term || !state.selectedNodeId) {
    nodeForm.innerHTML = '<div class="node-empty">Выберите узел дерева слева</div>';
    return;
  }
  const loc = locateNode(term.root, state.selectedNodeId);
  if (!loc) {
    nodeForm.innerHTML = '<div class="node-empty">Выберите узел дерева слева</div>';
    return;
  }
  const node = loc.node;

  if (node.id === 'root') {
    nodeForm.innerHTML = `
      <div class="node-type-label">КОРЕНЬ ТЕРМИНАЛА</div>
      <div class="node-empty">Это главный экран терминала. Добавляйте папки, команды и записи через панель инструментов сверху.</div>`;
    return;
  }

  const typeLabel = node.type === 'folder' ? 'ПАПКА' : node.type === 'command' ? 'КОМАНДА' : 'ЗАПИСЬ';
  const snapshot = commandExecutionState(term, node.id);
  let html = `<div class="node-type-label">${typeLabel}</div>
    <label class="field-label" for="fldName">${node.type === 'command' ? 'ИСХОДНОЕ НАЗВАНИЕ' : 'НАЗВАНИЕ'}</label>
    <input class="field-input" id="fldName" value="${escAttr(node.name)}">`;

  if (node.type === 'command') {
    const commandMode = node.stateChange
      ? 'state-change'
      : node.terminalTransition
        ? 'terminal-transition'
        : 'ordinary';
    const authoredGroup = state.session.terminalGroups?.find(group => group.terminalIds.includes(term.id));
    const eligibleTransitionIDs = new Set(authoredGroup?.terminalIds ?? []);
    const transitionOptions = state.session.terminals
      .filter(candidate => candidate.id !== term.id && eligibleTransitionIDs.has(candidate.id))
      .map(candidate => `<option value="${escAttr(candidate.id)}"${node.terminalTransition?.targetTerminalId === candidate.id ? ' selected' : ''}>${escHtml(candidate.name)}</option>`)
      .join('');
    html += `
      <label class="field-label" for="fldCommandMode">РЕЖИМ КОМАНДЫ</label>
      <select class="field-input command-mode-select" id="fldCommandMode"${snapshot ? ' disabled' : ''}>
        <option value="ordinary"${commandMode === 'ordinary' ? ' selected' : ''}>ОБЫЧНАЯ КОМАНДА</option>
        <option value="state-change"${commandMode === 'state-change' ? ' selected' : ''}>ИЗМЕНЯЕТ СОСТОЯНИЕ</option>
        <option value="terminal-transition"${commandMode === 'terminal-transition' ? ' selected' : ''}>ПЕРЕХОД В ДРУГОЙ ТЕРМИНАЛ</option>
      </select>
      ${snapshot ? '<div class="command-mode-hint">Сначала сбросьте выполненное состояние команды, чтобы изменить режим.</div>' : ''}
      <div class="state-change-fields" id="stateChangeFields"${commandMode === 'state-change' ? '' : ' hidden'}>
        <label class="field-label" for="fldCompletedName">НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ</label>
        <input class="field-input" id="fldCompletedName" value="${escAttr(node.stateChange?.completedName || '')}">
        <label class="field-label" for="fldConfirmationText">ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ</label>
        <textarea class="field-textarea state-change-textarea" id="fldConfirmationText">${escHtml(node.stateChange?.confirmationText || '')}</textarea>
      </div>
      <div class="state-change-fields terminal-transition-fields" id="terminalTransitionFields"${commandMode === 'terminal-transition' ? '' : ' hidden'}>
        <label class="field-label" for="fldTerminalTransitionTarget">ЦЕЛЕВОЙ ТЕРМИНАЛ</label>
        <select class="field-input" id="fldTerminalTransitionTarget">
          <option value="">ВЫБЕРИТЕ ТЕРМИНАЛ</option>${transitionOptions}
        </select>
      </div>
      <label class="field-label" for="fldText">ТЕКСТ УСПЕШНОГО ВЫПОЛНЕНИЯ</label>
      <textarea class="field-textarea" id="fldText">${escHtml(node.text || '')}</textarea>`;
    if (snapshot) {
      html += `
        <div class="command-execution-snapshot" role="status" aria-label="СОХРАНЁННОЕ СОСТОЯНИЕ КОМАНДЫ">
          <div class="command-execution-heading">ВЫПОЛНЕНО</div>
          <div class="command-execution-label">ЗАФИКСИРОВАННОЕ НАЗВАНИЕ</div>
          <div class="command-execution-value">${escHtml(snapshot.completedName || '')}</div>
          <div class="command-execution-label">ЗАФИКСИРОВАННЫЙ РЕЗУЛЬТАТ</div>
          <div class="command-execution-value command-execution-result">${escHtml(snapshot.resultText || '')}</div>
        </div>`;
    }
  } else if (node.type === 'entry') {
    html += `<label class="field-label" for="fldText">ОПИСАНИЕ ЗАПИСИ</label>
      <textarea class="field-textarea" id="fldText">${escHtml(node.description || '')}</textarea>`;
  } else if (node.type === 'folder') {
    const count = node.children ? node.children.length : 0;
    html += `<div class="field-label">СОДЕРЖИМОЕ</div><div class="node-empty">${count} элемент(ов)</div>`;
  }

  html += '<div class="node-validation-error" id="nodeValidationError" role="alert" hidden></div>';
  html += `<div class="node-actions">
      <button class="btn btn-primary" id="btnApplyNode">ПРИМЕНИТЬ</button>
      ${snapshot ? '<button class="btn btn-secondary" id="btnResetCommandState" type="button">СБРОСИТЬ СОСТОЯНИЕ</button>' : ''}
      <button class="btn btn-danger" id="btnDeleteNode">УДАЛИТЬ</button>
    </div>`;

  nodeForm.innerHTML = html;

  const validationError = legacyOverseerRoot.querySelector('#nodeValidationError');
  const showValidationError = (message, field) => {
    validationError.textContent = message;
    validationError.hidden = false;
    field?.focus();
  };

  if (node.type === 'command') {
    const mode = legacyOverseerRoot.querySelector('#fldCommandMode');
    const fields = legacyOverseerRoot.querySelector('#stateChangeFields');
    const transitionFields = legacyOverseerRoot.querySelector('#terminalTransitionFields');
    mode.addEventListener('change', () => {
      fields.hidden = mode.value !== 'state-change';
      transitionFields.hidden = mode.value !== 'terminal-transition';
      validationError.hidden = true;
      validationError.textContent = '';
    });
  }

  legacyOverseerRoot.querySelector('#btnApplyNode').addEventListener('click', () => {
    const nameEl = legacyOverseerRoot.querySelector('#fldName');
    const name = nameEl.value.trim();
    if (!name) {
      showValidationError(
        node.type === 'command' ? 'УКАЖИТЕ ИСХОДНОЕ НАЗВАНИЕ КОМАНДЫ' : 'УКАЖИТЕ НАЗВАНИЕ',
        nameEl
      );
      return;
    }

    if (node.type === 'command') {
      const commandMode = legacyOverseerRoot.querySelector('#fldCommandMode').value;
      const textEl = legacyOverseerRoot.querySelector('#fldText');
      let nextStateChange = null;
      let nextTerminalTransition = null;
      if (commandMode === 'state-change') {
        const completedNameEl = legacyOverseerRoot.querySelector('#fldCompletedName');
        const confirmationTextEl = legacyOverseerRoot.querySelector('#fldConfirmationText');
        if (!completedNameEl.value.trim()) {
          showValidationError('УКАЖИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ', completedNameEl);
          return;
        }
        if (!confirmationTextEl.value.trim()) {
          showValidationError('УКАЖИТЕ ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ', confirmationTextEl);
          return;
        }
        if (!textEl.value.trim()) {
          showValidationError('УКАЖИТЕ ТЕКСТ УСПЕШНОГО РЕЗУЛЬТАТА', textEl);
          return;
        }
        nextStateChange = {
          completedName: completedNameEl.value,
          confirmationText: confirmationTextEl.value,
        };
      }
      if (commandMode === 'terminal-transition') {
        const targetEl = legacyOverseerRoot.querySelector('#fldTerminalTransitionTarget');
        const targetID = targetEl.value;
        if (!targetID || targetID === term.id || !state.session.terminals.some(candidate => candidate.id === targetID)) {
          showValidationError('ВЫБЕРИТЕ ДРУГОЙ СУЩЕСТВУЮЩИЙ ТЕРМИНАЛ', targetEl);
          return;
        }
        nextTerminalTransition = { targetTerminalId: targetID };
      }
      if (nextStateChange) node.stateChange = nextStateChange;
      else delete node.stateChange;
      if (nextTerminalTransition) node.terminalTransition = nextTerminalTransition;
      else delete node.terminalTransition;
      node.text = textEl.value;
    }

    node.name = name;
    if (node.type === 'entry')   node.description = legacyOverseerRoot.querySelector('#fldText').value;
    autosave();
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });

  const btnResetCommandState = legacyOverseerRoot.querySelector('#btnResetCommandState');
  if (btnResetCommandState) {
    btnResetCommandState.disabled = sessionStateCommandPending;
    btnResetCommandState.addEventListener('click', async () => {
      if (sessionStateCommandPending) return;
      const displayedName = snapshot?.completedName || node.name;
      if (!await confirmCommandStateReset(`Сбросить выполненное состояние команды "${displayedName}"?`)) return;
      const revisionBeforeReset = newestDurableRevision;
      runSessionStateCommand(
        () => desktopAPI.resetCommandState({ terminalId: term.id, commandId: node.id }),
        'СОСТОЯНИЕ КОМАНДЫ СБРОШЕНО',
        (result) => {
          const revision = Number(result?.revision || 0);
          const canonicalTerminal = result?.session?.terminals?.find(candidate => candidate.id === term.id);
          return revision > revisionBeforeReset && canonicalTerminal &&
            !Object.hasOwn(canonicalTerminal.commandStates ?? {}, node.id);
        }
      );
    });
  }

  legacyOverseerRoot.querySelector('#btnDeleteNode').addEventListener('click', () => {
    const childCount = (node.type === 'folder' && node.children) ? node.children.length : 0;
    const msg = childCount > 0
      ? `Удалить "${node.name}" вместе со всем содержимым (${childCount} элемент(ов))?`
      : `Удалить "${node.name}"?`;
    if (!window.confirm(msg)) return;
    const siblings = loc.parent.children;
    const idx = siblings.findIndex(c => c.id === node.id);
    if (idx >= 0) siblings.splice(idx, 1);
    state.selectedNodeId = null;
    autosave();
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });
}

// ── Toolbar: add nodes ────────────────────────────────────────
function addNode(type) {
  const target = currentAddTarget();
  if (!target) return;

  const node = {
    id:   uid('n'),
    type,
    name: type === 'folder' ? 'Новая папка' : type === 'command' ? 'Новая команда' : 'Новая запись',
  };
  if (type === 'folder')  node.children = [];
  if (type === 'command') node.text = '';
  if (type === 'entry')   node.description = '';

  if (!target.children) target.children = [];
  target.children.push(node);
  state.expanded.add(target.id);
  state.selectedNodeId = node.id;

  autosave();
  renderTree();
  renderNodeForm();
  renderToolbarHint();
}

btnAddFolder.addEventListener('click', () => addNode('folder'));
btnAddCommand.addEventListener('click', () => addNode('command'));
btnAddEntry.addEventListener('click', () => addNode('entry'));

// ── Player roster and broadcast management ──────────────────
btnManageLogicalSessions.addEventListener('click', () => {
  publishLegacyProjection({ kind: 'logical-session-open-request' });
});
btnStartBroadcast.addEventListener('click', async () => {
  if (coordinationCommandPending || state.coordination?.broadcast || !state.coordination?.playerConfig) return;
  coordinationCommandPending = true;
  setCoordinationStatus('ЗАПУСК ТРАНСЛЯЦИИ...');
  renderCoordination();
  const result = await desktopAPI.startBroadcast();
  coordinationCommandPending = false;
  if (!result?.ok) {
    setCoordinationStatus(result?.error || 'НЕ УДАЛОСЬ ЗАПУСТИТЬ ТРАНСЛЯЦИЮ', true);
    renderCoordination();
    return;
  }

  state.coordination = result.state || state.coordination;
  setCoordinationStatus('ТРАНСЛЯЦИЯ ЗАПУЩЕНА');
  renderCoordination();
  renderTreeHeader();
});

btnEndBroadcast.addEventListener('click', () => {
  if (coordinationCommandPending || !state.coordination?.broadcast) return;
  showEndBroadcastConfirmation();
});

btnCancelEndBroadcast.addEventListener('click', () => hideEndBroadcastConfirmation());
endBroadcastDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!coordinationCommandPending) hideEndBroadcastConfirmation();
});

btnConfirmEndBroadcast.addEventListener('click', async () => {
  if (coordinationCommandPending || !state.coordination?.broadcast) return;
  btnCancelEndBroadcast.disabled = true;
  btnConfirmEndBroadcast.disabled = true;
  const result = await runCoordinationCommand(
    () => desktopAPI.endBroadcast(),
    'ТРАНСЛЯЦИЯ ЗАВЕРШЕНА · СЕССИИ И ПЕРСОНАЖИ СОХРАНЕНЫ',
    'ЗАВЕРШЕНИЕ ТРАНСЛЯЦИИ...'
  );
  if (!result?.ok) {
    hideEndBroadcastConfirmation();
    return;
  }
  if (!result.state || result.state.broadcast) {
    setCoordinationStatus('ЗАВЕРШЕНИЕ НЕ ПОДТВЕРЖДЕНО АВТОРИТЕТНЫМ СОСТОЯНИЕМ', true);
    renderCoordination();
    hideEndBroadcastConfirmation();
    return;
  }
  hideEndBroadcastConfirmation({ restoreFocus: false });
  dismissTerminalSwitchRequest();
  state.liveHack = null;
  renderAll();
  btnStartBroadcast.focus();
});

// ── Terminal management ───────────────────────────────────────
function showCreateTerminalDialog() {
  if (!state.session || createTerminalSubmitting) return;
  createTerminalName.value = '';
  createTerminalError.textContent = '';
  createTerminalError.hidden = true;
  btnCancelCreateTerminal.disabled = false;
  btnConfirmCreateTerminal.disabled = false;
  createTerminalDialog.hidden = false;
  if (typeof createTerminalDialog.showModal === 'function' && !createTerminalDialog.open) {
    createTerminalDialog.showModal();
  } else {
    createTerminalDialog.setAttribute('open', '');
  }
  createTerminalName.focus();
}

function hideCreateTerminalDialog({ restoreFocus = true } = {}) {
  if (typeof createTerminalDialog.close === 'function' && createTerminalDialog.open) {
    createTerminalDialog.close();
  } else {
    createTerminalDialog.removeAttribute('open');
  }
  createTerminalDialog.hidden = true;
  createTerminalSubmitting = false;
  btnCancelCreateTerminal.disabled = false;
  btnConfirmCreateTerminal.disabled = false;
  if (restoreFocus && !btnAddTerminal.hidden) btnAddTerminal.focus();
}

btnAddTerminal.addEventListener('click', showCreateTerminalDialog);

btnCancelCreateTerminal.addEventListener('click', () => {
  if (!createTerminalSubmitting) hideCreateTerminalDialog();
});

createTerminalDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!createTerminalSubmitting) hideCreateTerminalDialog();
});

createTerminalForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  if (!state.session || createTerminalSubmitting) return;
  const name = createTerminalName.value.trim();
  if (!name) {
    createTerminalError.textContent = 'УКАЖИТЕ НАЗВАНИЕ ТЕРМИНАЛА';
    createTerminalError.hidden = false;
    createTerminalName.focus();
    return;
  }

  createTerminalSubmitting = true;
  btnCancelCreateTerminal.disabled = true;
  btnConfirmCreateTerminal.disabled = true;
  const term = {
    id:        uid('t'),
    name,
    hackLevel: 0,
    introText: '',
    root:      { id: 'root', type: 'folder', name: 'ROOT', children: [] },
  };
  state.session.terminals.push(term);
  ensureSessionTerminalGroups();
  state.editTerminalId = term.id;
  state.selectedNodeId = null;
  state.expanded = new Set(['root']);
  await autosave();
  renderAll();
  hideCreateTerminalDialog({ restoreFocus: false });
  hackLevelSelect.focus();
});

btnApplySettings.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term) return;
  term.hackLevel = Number(hackLevelSelect.value) || 0;
  term.introText = introTextArea.value;
  autosave();
  if (term.id === state.liveTerminalId) {
    // Intro text can refresh live immediately; hackLevel only takes effect
    // on the next (re)broadcast so it never disrupts an in-progress hack.
    await runCoordinationCommand(
      () => desktopAPI.updateLiveTerminal({ tree: term.root, introText: term.introText }),
      'АКТИВНЫЙ ТЕРМИНАЛ ОБНОВЛЁН',
      'ОБНОВЛЕНИЕ АКТИВНОГО ТЕРМИНАЛА...'
    );
  }
});

function terminalActivationRequest(term) {
  return desktopAPI.requestTerminalActivation({
    terminalId: term.id,
    terminalName: term.name,
    tree: term.root,
    hackLevel: term.hackLevel || 0,
    introText: term.introText || '',
  });
}

btnMakeLive.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term || term.id === state.liveTerminalId || !state.coordination?.broadcast || coordinationCommandPending) return;
  const result = await runTerminalSwitchRequest(
    () => terminalActivationRequest(term),
    'АКТИВНЫЙ ТЕРМИНАЛ ВЫБРАН',
    'ПЕРЕКЛЮЧЕНИЕ АКТИВНОГО ТЕРМИНАЛА...'
  );
  if (result?.ok && result.status === 'activated') state.liveHack = null;
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
});

btnReapplySettings.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term || term.id !== state.liveTerminalId || !state.coordination?.broadcast || coordinationCommandPending) return;
  terminalSettingsMenu.open = false;
  const result = await runTerminalSwitchRequest(
    () => terminalActivationRequest(term),
    'НАСТРОЙКИ АКТИВНОГО ТЕРМИНАЛА ПЕРЕПРИМЕНЕНЫ',
    'ПЕРЕПРИМЕНЕНИЕ НАСТРОЕК...'
  );
  if (result?.ok && result.status === 'activated') state.liveHack = null;
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
});

btnPublish.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term || term.id !== state.liveTerminalId || coordinationCommandPending) return;
  const result = await runTerminalSwitchRequest(
    () => desktopAPI.updateLiveTerminal({ tree: term.root, introText: term.introText || '' }),
    'ИЗМЕНЕНИЯ ОПУБЛИКОВАНЫ У ИГРОКОВ',
    'ПУБЛИКАЦИЯ ИЗМЕНЕНИЙ...'
  );
  if (!result?.ok) return;
  btnPublish.textContent = 'ОБНОВЛЕНО ✓';
  setTimeout(() => { btnPublish.textContent = 'ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ'; }, 1200);
});

btnStopBroadcast.addEventListener('click', showTakeOffAirConfirmation);

btnCancelTakeOffAir.addEventListener('click', () => {
  if (!takeOffAirPending) hideTakeOffAirConfirmation();
});

takeOffAirDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!takeOffAirPending) hideTakeOffAirConfirmation();
});

btnConfirmTakeOffAir.addEventListener('click', async () => {
  if (takeOffAirPending || coordinationCommandPending || !state.coordination?.broadcast || !state.liveTerminalId) return;
  takeOffAirPending = true;
  takeOffAirError.textContent = '';
  takeOffAirError.hidden = true;
  btnCancelTakeOffAir.disabled = true;
  btnConfirmTakeOffAir.disabled = true;
  const result = await runCoordinationCommand(
    () => desktopAPI.requestTerminalClear(),
    'АКТИВНЫЙ ТЕРМИНАЛ УБРАН · ТРАНСЛЯЦИЯ ПРОДОЛЖАЕТСЯ',
    'ОЧИСТКА АКТИВНОГО ТЕРМИНАЛА...'
  );
  takeOffAirPending = false;
  if (!result?.ok) {
    takeOffAirError.textContent = result?.error || 'НЕ УДАЛОСЬ СНЯТЬ ТЕРМИНАЛ С ЭФИРА';
    takeOffAirError.hidden = false;
    btnCancelTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.focus();
    return;
  }
  if (result.status === 'decision-required' && result.switchId) {
    hideTakeOffAirConfirmation({ restoreFocus: false });
    publishTerminalSwitchRequest(result);
    return;
  }
  if (result.status !== 'cleared') {
    takeOffAirError.textContent = 'СНЯТИЕ С ЭФИРА НЕ ПОДТВЕРЖДЕНО';
    takeOffAirError.hidden = false;
    btnCancelTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.focus();
    return;
  }
  state.liveHack = null;
  hideTakeOffAirConfirmation({ restoreFocus: false });
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
  if (!btnEndBroadcast.hidden) btnEndBroadcast.focus();
});

btnHackSuccess.addEventListener('click', () => {
  if (!state.liveHack || state.liveHack.solved || state.liveHack.failed) return;
  desktopAPI.forceHackSuccess();
});

btnResetFailedHack.addEventListener('click', async () => {
  const term = state.session && state.liveTerminalId
    ? state.session.terminals.find(candidate => candidate.id === state.liveTerminalId)
    : null;
  if (!term || !state.liveHack?.failed || coordinationCommandPending) return;
  const result = await runCoordinationCommand(
    () => desktopAPI.resetFailedHack({
      terminalId: term.id,
      terminalName: term.name,
      tree: term.root,
      hackLevel: term.hackLevel || 0,
      introText: term.introText || '',
    }),
    'СОЗДАНА НОВАЯ ГОЛОВОЛОМКА',
    'ПОДГОТОВКА НОВОЙ ГОЛОВОЛОМКИ...'
  );
  if (!result?.ok) renderHackStatus();
});

publishLegacyProjection({ kind: 'legacy-ready' });
