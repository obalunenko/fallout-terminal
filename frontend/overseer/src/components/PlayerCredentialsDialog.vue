<script setup lang="ts">
import { computed, inject, nextTick, onUnmounted, ref } from 'vue';

import type { PublicAccessViewSnapshot } from '../composables/usePublicAccess.js';
import { overseerControllerKey } from '../controllers/overseer-controller.js';
import type { DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const props = defineProps<{
  readonly port: DesktopPort;
  readonly snapshot: PublicAccessViewSnapshot;
}>();

const emit = defineEmits<{
  snapshot: [value: PublicAccessViewSnapshot];
}>();

const dialog = ref<HTMLDialogElement | null>(null);
const usernameInput = ref<HTMLInputElement | null>(null);
const passwordInput = ref<HTMLInputElement | null>(null);
const username = ref('players');
const password = ref('');
const error = ref('');
const open = ref(false);
const pending = ref(false);
const sharing = ref(false);
const controller = inject(overseerControllerKey, null);
let active = true;
let invocation = 0;
let shareInvocation = 0;
let opener: HTMLElement | null = null;

const configured = computed(() => props.snapshot.playerPasswordPresence === 'present');
const passwordValid = computed(() => password.value === '' || Array.from(password.value).length >= 8);
const saveDisabled = computed(() => pending.value || username.value.trim() === '' || !passwordValid.value
  || (!configured.value && password.value === ''));
const passwordHint = computed(() => configured.value
  ? 'Оставьте пустым, чтобы сохранить текущий пароль.'
  : 'Введите пароль не короче 8 символов или сгенерируйте новый.');

function commandSnapshot(value: unknown): PublicAccessViewSnapshot | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const record = value as DesktopRecord;
  const preferences: DesktopRecord = typeof record.preferences === 'object' && record.preferences !== null
    && !Array.isArray(record.preferences) ? record.preferences as DesktopRecord : Object.freeze({});
  const status = typeof record.status === 'object' && record.status !== null && !Array.isArray(record.status)
    ? record.status as DesktopRecord : record;
  const generation = Number.isSafeInteger(status.generation) ? Number(status.generation) : -1;
  const settingsRevision = Number.isSafeInteger(status.settingsRevision) ? Number(status.settingsRevision) : -1;
  if (generation < 0 || settingsRevision < 0) return null;
  const stateMap: Readonly<Record<string, string>> = Object.freeze({
    disabled: 'stopped', error: 'error', failed: 'error', ready: 'ready',
    starting: 'starting', stopped: 'stopped', stopping: 'stopping',
  });
  const state = stateMap[typeof status.state === 'string' ? status.state : ''] ?? '';
  const presence = (candidate: unknown): string => ['absent', 'present', 'unknown'].includes(String(candidate))
    ? String(candidate)
    : 'unknown';
  return Object.freeze({
    generation,
    playerPasswordPresence: presence(record.playerPasswordPresence),
    preferences: Object.freeze({
      reservedDomain: typeof preferences.reservedDomain === 'string' ? preferences.reservedDomain : '',
      revision: Number.isSafeInteger(preferences.revision) ? Number(preferences.revision) : settingsRevision,
      username: typeof preferences.username === 'string' && preferences.username !== ''
        ? preferences.username
        : 'players',
    }),
    providerTokenPresence: presence(record.providerTokenPresence),
    settingsRevision,
    status: Object.freeze({
      errorCategory: state === 'error' && typeof status.errorCategory === 'string' ? status.errorCategory : '',
      errorMessage: state === 'error' && typeof status.errorMessage === 'string' ? status.errorMessage : '',
      generation,
      publicUrl: state === 'ready' && typeof status.publicUrl === 'string' ? status.publicUrl : '',
      settingsRevision,
      state,
    }),
  });
}

function emitSnapshot(value: unknown): void {
  const next = commandSnapshot(value);
  if (next !== null) emit('snapshot', next);
}

async function show(): Promise<void> {
  if (pending.value) return;
  opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  username.value = props.snapshot.preferences.username;
  password.value = '';
  error.value = '';
  open.value = true;
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  element.hidden = false;
  if (typeof element.showModal === 'function' && !element.open) element.showModal();
  else element.setAttribute('open', '');
  usernameInput.value?.focus();
}

function close(restoreFocus = true): void {
  if (pending.value) return;
  invocation += 1;
  password.value = '';
  error.value = '';
  const element = dialog.value;
  if (element?.open) element.close();
  else element?.removeAttribute('open');
  if (element !== null) element.hidden = true;
  open.value = false;
  const target = opener;
  opener = null;
  if (restoreFocus && target?.isConnected === true) target.focus();
  else if (restoreFocus) controller?.publish({ kind: 'public-access-settings-focus-player' });
}

function confirmActiveChange(): boolean {
  return props.snapshot.status.state !== 'ready' || window.confirm(
    'ПУБЛИЧНЫЙ ДОСТУП АКТИВЕН. ИЗМЕНЕНИЕ ОСТАНОВИТ И ПЕРЕЗАПУСТИТ ССЫЛКУ. ПРОДОЛЖИТЬ?',
  );
}

async function mutate(deletePlayerPassword = false): Promise<void> {
  if (pending.value || !open.value || !confirmActiveChange()) return;
  const trimmedUsername = deletePlayerPassword ? 'players' : username.value.trim();
  if (trimmedUsername === '' || (!deletePlayerPassword && !passwordValid.value)
    || (!deletePlayerPassword && !configured.value && password.value === '')) {
    error.value = 'ПАРОЛЬ ДОЛЖЕН СОДЕРЖАТЬ НЕ МЕНЕЕ 8 СИМВОЛОВ';
    passwordInput.value?.focus();
    return;
  }
  const request: Record<string, unknown> = {
    expectedRevision: props.snapshot.preferences.revision,
    enabledPreference: false,
    reservedDomain: props.snapshot.preferences.reservedDomain,
    username: trimmedUsername,
    replacementProviderToken: '',
    deleteProviderToken: false,
    replacementPlayerPassword: deletePlayerPassword ? '' : password.value,
    deletePlayerPassword,
  };
  const currentInvocation = ++invocation;
  pending.value = true;
  error.value = '';
  const pendingResult = props.port.savePublicAccessSettings(request);
  password.value = '';
  request.replacementPlayerPassword = '';
  const result = await pendingResult;
  if (!active || currentInvocation !== invocation || !open.value) return;
  pending.value = false;
  emitSnapshot(result.snapshot);
  controller?.dispatch({ kind: 'public-access-settings-command-finished', result });
  if (result.ok !== true) {
    username.value = trimmedUsername;
    error.value = typeof result.error === 'string' && result.error !== ''
      ? result.error
      : 'НЕ УДАЛОСЬ СОХРАНИТЬ ДАННЫЕ ИГРОКОВ';
    await nextTick();
    passwordInput.value?.focus();
    return;
  }
  close();
}

async function generate(): Promise<void> {
  if (pending.value || !open.value || !confirmActiveChange()) return;
  const currentInvocation = ++invocation;
  pending.value = true;
  error.value = '';
  const result = await props.port.generatePlayerPassword({
    expectedRevision: props.snapshot.preferences.revision,
  });
  password.value = '';
  if (!active || currentInvocation !== invocation || !open.value) return;
  pending.value = false;
  const generatedPassword = typeof result.generatedPassword === 'string' ? result.generatedPassword : '';
  if (result.ok !== true || generatedPassword === '') {
    error.value = typeof result.error === 'string' && result.error !== ''
      ? result.error
      : 'НЕ УДАЛОСЬ СОЗДАТЬ ПАРОЛЬ';
    await nextTick();
    passwordInput.value?.focus();
    return;
  }
  controller?.dispatch({ kind: 'public-access-generated-password-open', generatedPassword });
  const refreshed = await props.port.getPublicAccess();
  if (active && currentInvocation === invocation) emitSnapshot(refreshed);
}

async function share(): Promise<void> {
  if (sharing.value || props.snapshot.playerPasswordPresence !== 'present') return;
  const currentInvocation = ++shareInvocation;
  sharing.value = true;
  controller?.publish({ kind: 'public-access-settings-copy-status', status: '' });
  const result = await props.port.copyPublicAccessCredentials();
  if (!active || currentInvocation !== shareInvocation) return;
  sharing.value = false;
  controller?.publish({
    kind: 'public-access-settings-copy-status',
    status: result.ok === true
      ? 'ЛОГИН И ПАРОЛЬ СКОПИРОВАНЫ'
      : (result.error || 'НЕ УДАЛОСЬ СКОПИРОВАТЬ ДАННЫЕ ИГРОКОВ'),
  });
}

function cancel(event?: Event): void {
  event?.preventDefault();
  close();
}

const release = controller?.subscribeState(message => {
  if (message.kind === 'public-access-player-credentials-open') void show();
  if (message.kind === 'public-access-credentials-share') void share();
  if (message.kind === 'public-access-player-generate-focus' && open.value) {
    document.getElementById('btnGeneratePlayerPassword')?.focus();
  }
});

onUnmounted(() => {
  active = false;
  invocation += 1;
  shareInvocation += 1;
  pending.value = false;
  sharing.value = false;
  username.value = '';
  password.value = '';
  error.value = '';
  release?.();
  close(false);
});
</script>

<template>
  <dialog
    id="publicAccessPlayerCredentialsDialog"
    ref="dialog"
    v-bind="{ 'data-secret-lifecycle': 'cleared' }"
    class="terminal-switch-dialog public-access-player-credentials-dialog"
    aria-modal="true"
    aria-labelledby="publicAccessPlayerCredentialsDialogTitle"
    aria-describedby="publicAccessPlayerCredentialsDialogDescription"
    hidden
    @cancel="cancel"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="publicAccessPlayerCredentialsDialogTitle" class="terminal-switch-dialog-title">ИЗМЕНИТЬ ДАННЫЕ ИГРОКОВ</h2>
      <p id="publicAccessPlayerCredentialsDialogDescription" class="terminal-switch-dialog-description">Измените логин и при необходимости задайте новый пароль. Сохранённый пароль посмотреть нельзя.</p>
      <form id="publicAccessPlayerCredentialsForm" class="public-access-player-credentials-form" autocomplete="off" @submit.prevent="mutate(false)">
        <label class="field-label-inline" for="publicAccessReplacementUsername">Логин игроков</label>
        <input id="publicAccessReplacementUsername" ref="usernameInput" v-model="username" class="field-input" type="text" name="publicAccessReplacementUsername" autocomplete="username" autocapitalize="none" spellcheck="false" required :disabled="pending">
        <label class="field-label-inline" for="publicAccessReplacementPlayerPassword">Новый пароль игроков</label>
        <div class="public-access-password-row">
          <input id="publicAccessReplacementPlayerPassword" ref="passwordInput" v-model="password" class="field-input" type="password" name="publicAccessReplacementPlayerPassword" autocomplete="new-password" minlength="8" aria-describedby="publicAccessReplacementPlayerPasswordHint publicAccessPlayerCredentialsError" :disabled="pending">
          <button id="btnGeneratePlayerPassword" class="btn btn-secondary" type="button" :disabled="pending" @click="generate">СГЕНЕРИРОВАТЬ</button>
        </div>
        <div id="publicAccessReplacementPlayerPasswordHint" class="public-access-hint">{{ passwordHint }}</div>
        <div id="publicAccessPlayerCredentialsError" class="public-access-settings-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error.length === 0">{{ error }}</div>
        <div class="public-access-player-credentials-actions">
          <button id="btnCancelPublicAccessPlayerCredentials" class="btn btn-secondary" type="button" :disabled="pending" @click="close()">ОТМЕНА</button>
          <button id="btnSavePublicAccessPlayerCredentials" class="btn btn-primary" type="submit" :disabled="saveDisabled">СОХРАНИТЬ ДАННЫЕ</button>
        </div>
        <button id="btnDeletePublicAccessPlayerCredentials" class="btn btn-danger public-access-player-credentials-delete" type="button" :hidden="!configured" :disabled="pending || !configured" @click="mutate(true)">УДАЛИТЬ СОХРАНЁННЫЕ ДАННЫЕ</button>
      </form>
    </div>
  </dialog>
</template>
