<script setup lang="ts">
import { inject, nextTick, onUnmounted, ref } from 'vue';

import { useClipboard } from '../composables/useClipboard.js';
import { overseerCoexistenceBridgeKey } from '../mount.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const props = defineProps<{
  readonly port: DesktopPort;
}>();

const dialog = ref<HTMLDialogElement | null>(null);
const copyButton = ref<HTMLButtonElement | null>(null);
const oneTimeValue = ref('');
const open = ref(false);
const copying = ref(false);
const bridge = inject(overseerCoexistenceBridgeKey, null);
const clipboard = useClipboard(props.port);
let active = true;
let invocation = 0;

async function show(value: string): Promise<void> {
  if (copying.value || value === '') return;
  oneTimeValue.value = value;
  clipboard.clear();
  open.value = true;
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  element.hidden = false;
  if (typeof element.showModal === 'function' && !element.open) element.showModal();
  else element.setAttribute('open', '');
  copyButton.value?.focus();
}

function close(restoreFocus = true): void {
  invocation += 1;
  copying.value = false;
  oneTimeValue.value = '';
  const element = dialog.value;
  if (element?.open) element.close();
  else element?.removeAttribute('open');
  if (element !== null) element.hidden = true;
  open.value = false;
  clipboard.clear();
  if (restoreFocus) bridge?.legacyToVue({ kind: 'public-access-player-generate-focus' });
}

async function copyAndClose(): Promise<void> {
  if (copying.value || !open.value || oneTimeValue.value === '') return;
  const currentInvocation = ++invocation;
  const transientValue = oneTimeValue.value;
  copying.value = true;
  await clipboard.copy(transientValue, 'ПАРОЛЬ СКОПИРОВАН');
  if (!active || currentInvocation !== invocation) return;
  const status = clipboard.status.value;
  close();
  bridge?.legacyToVue({ kind: 'public-access-settings-copy-status', status });
}

function cancel(event?: Event): void {
  event?.preventDefault();
  close();
}

const release = bridge?.subscribeLegacyState(message => {
  if (message.kind !== 'public-access-generated-password-open'
    || typeof message.generatedPassword !== 'string' || message.generatedPassword === '') return;
  void show(message.generatedPassword);
});

onUnmounted(() => {
  active = false;
  invocation += 1;
  release?.();
  close(false);
});
</script>

<template>
  <dialog
    id="generatedPasswordDialog"
    ref="dialog"
    v-bind="{
      'data-clipboard-failure': 'isolated',
      'data-secret-lifecycle': 'cleared',
    }"
    class="terminal-switch-dialog generated-password-dialog"
    aria-modal="true"
    :aria-labelledby="open ? 'generatedPasswordDialogTitle' : undefined"
    aria-describedby="generatedPasswordDialogDescription"
    hidden
    @cancel="cancel"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="generatedPasswordDialogTitle" class="terminal-switch-dialog-title">НОВЫЙ ПАРОЛЬ ИГРОКОВ</h2>
      <p id="generatedPasswordDialogDescription" class="terminal-switch-dialog-description">Скопируйте пароль сейчас. После закрытия он больше не будет показан.</p>
      <output id="generatedPasswordValue" class="generated-password-value" aria-label="Одноразовый новый пароль">{{ oneTimeValue }}</output>
      <div class="generated-password-actions">
        <button id="btnCopyGeneratedPassword" ref="copyButton" class="btn btn-primary" type="button" :disabled="copying" @click="copyAndClose">КОПИРОВАТЬ И ЗАКРЫТЬ</button>
        <button id="btnDismissGeneratedPassword" class="btn btn-secondary" type="button" :disabled="copying" @click="close()">ЗАКРЫТЬ</button>
      </div>
    </div>
  </dialog>
</template>
