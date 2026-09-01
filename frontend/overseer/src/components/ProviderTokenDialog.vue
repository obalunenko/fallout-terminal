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
const tokenInput = ref<HTMLInputElement | null>(null);
const token = ref('');
const error = ref('');
const open = ref(false);
const pending = ref(false);
const controller = inject(overseerControllerKey, null);
let active = true;
let invocation = 0;
let opener: HTMLElement | null = null;

const saveDisabled = computed(() => pending.value || token.value.trim() === '');

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

async function show(): Promise<void> {
  if (pending.value || props.snapshot.providerTokenPresence !== 'present') return;
  opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  token.value = '';
  error.value = '';
  open.value = true;
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  element.hidden = false;
  if (typeof element.showModal === 'function' && !element.open) element.showModal();
  else element.setAttribute('open', '');
  tokenInput.value?.focus();
}

function close(restoreFocus = true): void {
  if (pending.value) return;
  invocation += 1;
  token.value = '';
  error.value = '';
  const element = dialog.value;
  if (element?.open) element.close();
  else element?.removeAttribute('open');
  if (element !== null) element.hidden = true;
  open.value = false;
  const target = opener;
  opener = null;
  if (restoreFocus && target?.isConnected === true) target.focus();
  else if (restoreFocus) controller?.publish({ kind: 'public-access-settings-focus-provider' });
}

async function mutate(deleteProviderToken = false): Promise<void> {
  if (pending.value || !open.value || (!deleteProviderToken && token.value.trim() === '')) return;
  if (props.snapshot.status.state === 'ready' && !window.confirm(
    'ПУБЛИЧНЫЙ ДОСТУП АКТИВЕН. ИЗМЕНЕНИЕ ОСТАНОВИТ И ПЕРЕЗАПУСТИТ ССЫЛКУ. ПРОДОЛЖИТЬ?',
  )) return;
  const request: Record<string, unknown> = {
    expectedRevision: props.snapshot.preferences.revision,
    enabledPreference: false,
    reservedDomain: props.snapshot.preferences.reservedDomain,
    username: props.snapshot.preferences.username,
    replacementProviderToken: deleteProviderToken ? '' : token.value,
    deleteProviderToken,
    replacementPlayerPassword: '',
    deletePlayerPassword: false,
  };
  const currentInvocation = ++invocation;
  pending.value = true;
  error.value = '';
  const pendingResult = props.port.savePublicAccessSettings(request);
  token.value = '';
  request.replacementProviderToken = '';
  const result = await pendingResult;
  if (!active || currentInvocation !== invocation || !open.value) return;
  pending.value = false;
  const next = commandSnapshot(result.snapshot);
  if (next !== null) emit('snapshot', next);
  controller?.dispatch({ kind: 'public-access-settings-command-finished', result });
  if (result.ok !== true) {
    error.value = typeof result.error === 'string' && result.error !== ''
      ? result.error
      : 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ТОКЕН NGROK';
    await nextTick();
    tokenInput.value?.focus();
    return;
  }
  close();
}

function cancel(event?: Event): void {
  event?.preventDefault();
  close();
}

const release = controller?.subscribeState(message => {
  if (message.kind === 'public-access-provider-token-open') void show();
});

onUnmounted(() => {
  active = false;
  invocation += 1;
  pending.value = false;
  token.value = '';
  error.value = '';
  release?.();
  close(false);
});
</script>

<template>
  <dialog
    id="publicAccessProviderTokenDialog"
    ref="dialog"
    v-bind="{ 'data-secret-lifecycle': 'cleared' }"
    class="terminal-switch-dialog public-access-provider-token-dialog"
    aria-modal="true"
    aria-labelledby="publicAccessProviderTokenDialogTitle"
    aria-describedby="publicAccessProviderTokenDialogDescription"
    hidden
    @cancel="cancel"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="publicAccessProviderTokenDialogTitle" class="terminal-switch-dialog-title">ИЗМЕНИТЬ ТОКЕН NGROK</h2>
      <p id="publicAccessProviderTokenDialogDescription" class="terminal-switch-dialog-description">Введите новый токен. Сохранённый токен нельзя посмотреть.</p>
      <form id="publicAccessProviderTokenForm" class="public-access-provider-token-form" autocomplete="off" @submit.prevent="mutate(false)">
        <label class="field-label-inline" for="publicAccessReplacementProviderToken">Новый токен ngrok</label>
        <input
          id="publicAccessReplacementProviderToken"
          ref="tokenInput"
          v-model="token"
          class="field-input"
          type="password"
          name="publicAccessReplacementProviderToken"
          autocomplete="new-password"
          aria-describedby="publicAccessReplacementProviderTokenHint publicAccessProviderTokenError"
          :disabled="pending"
        >
        <div id="publicAccessReplacementProviderTokenHint" class="public-access-hint">После сохранения старый токен будет заменён.</div>
        <div id="publicAccessProviderTokenError" class="public-access-settings-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error.length === 0">{{ error }}</div>
        <div class="public-access-provider-token-actions">
          <button id="btnCancelPublicAccessProviderToken" class="btn btn-secondary" type="button" :disabled="pending" @click="close()">ОТМЕНА</button>
          <button id="btnSavePublicAccessProviderToken" class="btn btn-primary" type="submit" :disabled="saveDisabled">СОХРАНИТЬ ТОКЕН</button>
        </div>
        <button id="btnDeletePublicAccessProviderToken" class="btn btn-danger public-access-provider-token-delete" type="button" :disabled="pending" @click="mutate(true)">УДАЛИТЬ СОХРАНЁННЫЙ ТОКЕН</button>
      </form>
    </div>
  </dialog>
</template>
