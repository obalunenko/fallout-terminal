<script setup lang="ts">
import { computed, inject, nextTick, onUnmounted, ref, watch } from 'vue';

import type { PublicAccessViewSnapshot } from '../composables/usePublicAccess.js';
import { overseerCoexistenceBridgeKey } from '../mount.js';
import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const props = defineProps<{
  readonly port: DesktopPort;
  readonly snapshot: PublicAccessViewSnapshot;
}>();

const dialog = ref<HTMLDialogElement | null>(null);
const closeButton = ref<HTMLButtonElement | null>(null);
const domainInput = ref<HTMLInputElement | null>(null);
const open = ref(false);
const pending = ref(false);
const setupRequired = ref(false);
const guideOpen = ref(false);
const reservedDomain = ref('');
const providerToken = ref('');
const error = ref('');
const copyStatus = ref('');
const bridge = inject(overseerCoexistenceBridgeKey, null);
let active = true;
let invocation = 0;
let opener: HTMLElement | null = null;

const providerConfigured = computed(() => props.snapshot.providerTokenPresence === 'present');
const playerConfigured = computed(() => props.snapshot.playerPasswordPresence === 'present');
const transitioning = computed(() => ['starting', 'stopping'].includes(props.snapshot.status.state));
const controlsDisabled = computed(() => pending.value || transitioning.value);
const providerPresence = computed(() => providerConfigured.value
  ? 'НАСТРОЕН'
  : props.snapshot.providerTokenPresence === 'absent' ? 'НЕ СОХРАНЕН' : 'НЕДОСТУПЕН');
const passwordPresence = computed(() => playerConfigured.value
  ? 'СОХРАНЕН'
  : props.snapshot.playerPasswordPresence === 'absent' ? 'НЕ СОХРАНЕН' : 'НЕДОСТУПЕН');

function versionOf(value: unknown): readonly [number, number] {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return [-1, -1];
  const record = value as DesktopRecord;
  const status = typeof record.status === 'object' && record.status !== null && !Array.isArray(record.status)
    ? record.status as DesktopRecord
    : record;
  const generation = Number.isSafeInteger(status.generation) ? Number(status.generation) : -1;
  const revision = Number.isSafeInteger(status.settingsRevision) ? Number(status.settingsRevision) : -1;
  return [generation, revision];
}

function newer(left: readonly [number, number], right: readonly [number, number]): boolean {
  return left[0] > right[0] || (left[0] === right[0] && left[1] > right[1]);
}

function currentVersion(): readonly [number, number] {
  return [props.snapshot.generation, props.snapshot.settingsRevision];
}

function clearTransientState(): void {
  providerToken.value = '';
  error.value = '';
  copyStatus.value = '';
}

async function show(required: boolean): Promise<void> {
  if (pending.value) return;
  opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  setupRequired.value = required;
  guideOpen.value = required;
  reservedDomain.value = props.snapshot.preferences.reservedDomain;
  clearTransientState();
  open.value = true;
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  element.hidden = false;
  if (typeof element.showModal === 'function' && !element.open) element.showModal();
  else element.setAttribute('open', '');
  closeButton.value?.focus();
}

function close(restoreFocus = true): void {
  if (pending.value) return;
  invocation += 1;
  const element = dialog.value;
  if (element?.open) element.close();
  else element?.removeAttribute('open');
  if (element !== null) element.hidden = true;
  open.value = false;
  setupRequired.value = false;
  guideOpen.value = false;
  clearTransientState();
  const target = opener;
  opener = null;
  if (restoreFocus && target?.isConnected === true) target.focus();
}

function validateDomain(): boolean {
  const value = reservedDomain.value.trim();
  if (value === '' || (!value.includes('://') && !/[/?#\s]/u.test(value))) return true;
  error.value = 'УКАЖИТЕ ДОМЕН БЕЗ HTTPS://, ПУТИ, ПАРАМЕТРОВ И ПРОБЕЛОВ';
  domainInput.value?.focus();
  return false;
}

async function save(): Promise<void> {
  if (controlsDisabled.value || !validateDomain()) return;
  if (props.snapshot.status.state === 'ready' && !window.confirm(
    'ПУБЛИЧНЫЙ ДОСТУП АКТИВЕН. ИЗМЕНЕНИЕ ОСТАНОВИТ И ПЕРЕЗАПУСТИТ ССЫЛКУ. ПРОДОЛЖИТЬ?',
  )) return;

  const request: Record<string, unknown> = {
    expectedRevision: props.snapshot.preferences.revision,
    enabledPreference: false,
    reservedDomain: reservedDomain.value.trim(),
    username: props.snapshot.preferences.username,
    replacementProviderToken: providerToken.value,
    deleteProviderToken: false,
    replacementPlayerPassword: '',
    deletePlayerPassword: false,
  };
  const currentInvocation = ++invocation;
  const baseline = currentVersion();
  pending.value = true;
  error.value = '';
  copyStatus.value = '';
  const pendingResult = props.port.savePublicAccessSettings(request);
  providerToken.value = '';
  request.replacementProviderToken = '';
  const result = await pendingResult;
  if (!active || currentInvocation !== invocation || !open.value) return;
  pending.value = false;
  const resultVersion = versionOf(result.snapshot);
  const latest = currentVersion();
  if (newer(latest, baseline) && !newer(resultVersion, latest)
    && (resultVersion[0] !== latest[0] || resultVersion[1] !== latest[1])) {
    error.value = 'НАСТРОЙКИ ИЗМЕНИЛИСЬ В ДРУГОЙ ОПЕРАЦИИ. ПРОВЕРЬТЕ ДАННЫЕ И ПОВТОРИТЕ СОХРАНЕНИЕ.';
    domainInput.value?.focus();
    return;
  }
  bridge?.vueToLegacy({ kind: 'public-access-settings-command-finished', result });
  if (result.ok !== true) {
    error.value = typeof result.error === 'string' && result.error !== ''
      ? result.error
      : 'НЕ УДАЛОСЬ СОХРАНИТЬ НАСТРОЙКИ';
    domainInput.value?.focus();
    return;
  }
  close();
}

async function openDocumentation(url: string): Promise<void> {
  const result = await props.port.openUrl(url);
  if (active && result.ok !== true) error.value = result.error || 'НЕ УДАЛОСЬ ОТКРЫТЬ ДОКУМЕНТАЦИЮ';
}

function cancel(event?: Event): void {
  event?.preventDefault();
  close();
}

const release = bridge?.subscribeLegacyState(message => {
  if (message.kind === 'public-access-settings-open') {
    void show(message.setupRequired === true);
    return;
  }
  if (message.kind === 'public-access-settings-copy-status') {
    copyStatus.value = typeof message.status === 'string' ? message.status : '';
    return;
  }
  if (message.kind === 'public-access-settings-focus-provider') {
    document.getElementById('btnOpenPublicAccessProviderToken')?.focus();
    return;
  }
  if (message.kind === 'public-access-settings-focus-player') {
    document.getElementById('btnOpenPublicAccessPlayerCredentials')?.focus();
  }
});

watch(() => props.snapshot.preferences.reservedDomain, value => {
  if (!open.value) reservedDomain.value = value;
});

onUnmounted(() => {
  active = false;
  invocation += 1;
  pending.value = false;
  clearTransientState();
  release?.();
  close(false);
});
</script>

<template>
  <dialog
    id="publicAccessSettingsDialog"
    ref="dialog"
    class="terminal-switch-dialog public-access-settings-dialog"
    aria-modal="true"
    aria-labelledby="publicAccessSettingsDialogTitle"
    aria-describedby="publicAccessSettingsDialogDescription publicAccessSetupRequired"
    v-bind="{ 'data-stale-result-guard': 'released' }"
    hidden
    @cancel="cancel"
  >
    <div class="terminal-switch-dialog-panel">
      <header class="public-access-settings-header">
        <div>
          <h2 id="publicAccessSettingsDialogTitle" class="terminal-switch-dialog-title">НАСТРОЙКИ ПУБЛИЧНОГО ДОСТУПА</h2>
          <p id="publicAccessSettingsDialogDescription" class="terminal-switch-dialog-description">Настройте ngrok и данные, которые игроки используют для входа.</p>
        </div>
        <button
          id="btnClosePublicAccessSettings"
          ref="closeButton"
          class="btn btn-secondary"
          type="button"
          :disabled="pending"
          @click="close()"
        >ЗАКРЫТЬ</button>
      </header>
      <div
        id="publicAccessSetupRequired"
        class="public-access-setup-required"
        role="status"
        aria-live="polite"
        :hidden="!setupRequired"
      >СНАЧАЛА СОХРАНИТЕ ТОКЕН NGROK И ПАРОЛЬ ИГРОКОВ</div>
      <details id="publicAccessGuide" class="public-access-guide" :open="guideOpen">
        <summary>КАК НАСТРОИТЬ ЧЕРЕЗ NGROK</summary>
        <ol>
          <li>Создайте личный токен в своём аккаунте ngrok. Устанавливать ngrok или запускать Terminal не нужно.</li>
          <li>По умолчанию оставьте домен пустым: ngrok автоматически назначит случайный хост для публичной ссылки.</li>
          <li>Для фиксированного хоста укажите доступный вашему аккаунту домен без <code>https://</code>. Он начнёт использоваться только после сохранения настроек.</li>
          <li>Сохраните токен ngrok, затем откройте <strong>ИЗМЕНИТЬ ДАННЫЕ</strong> и задайте логин и пароль игроков от 8 символов либо сгенерируйте пароль.</li>
          <li>Сохраните настройки и включите доступ из основной панели.</li>
          <li>Дождитесь статуса <strong>ГОТОВ</strong>. Скопируйте URL и нажмите <strong>ПОДЕЛИТЬСЯ</strong>, чтобы получить логин и пароль, которые браузер запросит через Basic Auth.</li>
        </ol>
        <p>Сохранённые токен и пароль не показываются: пароль обозначается маской ***** и попадает только в системный буфер после команды «Поделиться».</p>
      </details>
      <form id="publicAccessForm" class="public-access-form" autocomplete="off" @submit.prevent="save">
        <fieldset id="publicAccessConnectionGroup" class="public-access-settings-group" :disabled="controlsDisabled">
          <legend>ПОДКЛЮЧЕНИЕ NGROK</legend>
          <div id="publicAccessProviderSetup" class="public-access-provider-setup" :hidden="providerConfigured">
            <label class="field-label-inline" for="publicAccessProviderToken">Токен ngrok</label>
            <input
              id="publicAccessProviderToken"
              v-model="providerToken"
              class="field-input"
              type="password"
              name="publicAccessProviderToken"
              autocomplete="new-password"
              aria-describedby="publicAccessProviderPresence"
            >
          </div>
          <div id="publicAccessProviderConfigured" class="public-access-credential-row" :hidden="!providerConfigured">
            <span class="field-label-inline">Токен ngrok</span>
            <button
              id="btnOpenPublicAccessProviderToken"
              class="btn btn-secondary"
              type="button"
              @click="bridge?.vueToLegacy({ kind: 'public-access-provider-token-open' })"
            >ИЗМЕНИТЬ ТОКЕН</button>
          </div>
          <div id="publicAccessProviderPresence" v-bind="{ 'data-presence': snapshot.providerTokenPresence }" class="public-access-credential-status">{{ providerPresence }}</div>

          <label class="field-label-inline" for="publicAccessDomain">Зарезервированный домен</label>
          <input
            id="publicAccessDomain"
            ref="domainInput"
            v-model="reservedDomain"
            class="field-input"
            name="publicAccessDomain"
            type="text"
            inputmode="url"
            autocapitalize="none"
            spellcheck="false"
            aria-describedby="publicAccessDomainHint publicAccessSettingsError"
          >
          <div id="publicAccessDomainHint" class="public-access-hint">Пустое поле — случайный хост от ngrok. Фиксированный хост применяется только после заполнения поля и сохранения · без https://</div>
          <div class="public-access-doc-links" aria-label="Документация ngrok о доменах">
            <button id="btnOpenNgrokAutoAssignedDomainDocs" class="public-access-doc-link" type="button" @click="openDocumentation('https://ngrok.com/docs/gateway/domains/#auto-assigned-domains')">СЛУЧАЙНЫЕ ХОСТЫ · NGROK</button>
            <button id="btnOpenNgrokFixedDomainDocs" class="public-access-doc-link" type="button" @click="openDocumentation('https://ngrok.com/docs/gateway/domains/#domains')">ФИКСИРОВАННЫЕ ХОСТЫ · NGROK</button>
          </div>
        </fieldset>

        <fieldset id="publicAccessPlayerLoginGroup" class="public-access-settings-group" :disabled="controlsDisabled">
          <legend>ВХОД ДЛЯ ИГРОКОВ</legend>
          <div class="public-access-player-summary">
            <div class="public-access-player-summary-row">
              <span class="field-label-inline">Имя игрока</span>
              <output id="publicAccessUsernameSummary" class="public-access-player-summary-value">{{ snapshot.preferences.username }}</output>
            </div>
            <div class="public-access-player-summary-row">
              <span class="field-label-inline">Пароль игроков</span>
              <output id="publicAccessPasswordMask" class="public-access-player-password-mask" aria-label="Сохранённый пароль игроков" :hidden="!playerConfigured">{{ playerConfigured ? '*****' : '' }}</output>
              <span id="publicAccessPasswordPresence" v-bind="{ 'data-presence': snapshot.playerPasswordPresence }" class="public-access-credential-status" :hidden="playerConfigured">{{ passwordPresence }}</span>
            </div>
          </div>
          <div class="public-access-player-summary-actions" role="group" aria-label="Управление данными входа игроков">
            <button id="btnOpenPublicAccessPlayerCredentials" class="btn btn-secondary" type="button" @click="bridge?.vueToLegacy({ kind: 'public-access-player-credentials-open' })">ИЗМЕНИТЬ ДАННЫЕ</button>
            <button id="btnSharePublicAccessCredentials" class="btn btn-primary" type="button" :disabled="!playerConfigured || snapshot.preferences.username.trim() === ''" @click="bridge?.vueToLegacy({ kind: 'public-access-credentials-share' })">ПОДЕЛИТЬСЯ</button>
          </div>
          <div id="publicAccessSettingsCopyStatus" class="public-access-copy-status" role="status" aria-live="polite" aria-atomic="true">{{ copyStatus }}</div>
        </fieldset>

        <div id="publicAccessSettingsError" class="public-access-settings-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error.length === 0">{{ error }}</div>
        <div class="public-access-actions">
          <button id="btnCancelPublicAccessSettings" class="btn btn-secondary" type="button" :disabled="pending" @click="close()">ОТМЕНА</button>
          <button id="btnSavePublicAccess" class="btn btn-primary" type="submit" :disabled="controlsDisabled">СОХРАНИТЬ НАСТРОЙКИ</button>
        </div>
      </form>
    </div>
  </dialog>
</template>
