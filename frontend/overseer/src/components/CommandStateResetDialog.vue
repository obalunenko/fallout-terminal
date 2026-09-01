<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, ref, watch } from 'vue';

import { dialogFocus } from '../directives/dialog-focus.js';
import { overseerControllerKey, type OverseerControllerMessage } from '../controllers/overseer-controller.js';

interface ResetRequest {
  readonly message: string;
  readonly requestId: string;
}

const MAX_RESOLVED_REQUESTS = 128;

const controller = inject(overseerControllerKey, null);
const vDialogFocus = dialogFocus;
const dialog = ref<HTMLDialogElement | null>(null);
const cancelButton = ref<HTMLButtonElement | null>(null);
const request = ref<ResetRequest | null>(null);
const resolved = new Set<string>();
const open = computed(() => request.value !== null);
const focusBinding = computed(() => ({
  active: open.value,
  initialFocus: () => cancelButton.value,
  onCancel: () => settle(false),
}));

function handleControllerMessage(message: OverseerControllerMessage): void {
  if (message.kind !== 'command-state-reset-required') return;
  const requestId = typeof message.requestId === 'string' ? message.requestId : '';
  const description = typeof message.message === 'string' ? message.message : '';
  if (requestId === '' || resolved.has(requestId) || request.value !== null) return;
  request.value = Object.freeze({ message: description, requestId });
}

function settle(confirmed: boolean): void {
  const current = request.value;
  if (current === null || resolved.has(current.requestId)) return;
  resolved.add(current.requestId);
  if (resolved.size > MAX_RESOLVED_REQUESTS) {
    const oldest = resolved.values().next().value;
    if (oldest !== undefined) resolved.delete(oldest);
  }
  request.value = null;
  controller?.dispatch({ confirmed, kind: 'command-state-reset-resolved', requestId: current.requestId });
}

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (open.value && !element.open) element.showModal();
  else if (!open.value && element.open) element.close();
}

const release = controller?.subscribeState(handleControllerMessage) ?? (() => {});
watch(open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
onBeforeUnmount(() => {
  settle(false);
  release();
  if (dialog.value?.open === true) dialog.value.close();
  resolved.clear();
});
</script>

<template>
  <dialog
    id="resetConfirmationDialog"
    ref="dialog"
    v-dialog-focus="focusBinding"
    class="terminal-switch-dialog command-state-reset-dialog"
    aria-modal="true"
    aria-labelledby="resetConfirmationDialogTitle"
    aria-describedby="resetConfirmationDialogDescription"
    :hidden="!open"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="resetConfirmationDialogTitle" class="terminal-switch-dialog-title">ПОДТВЕРЖДЕНИЕ СБРОСА</h2>
      <p id="resetConfirmationDialogDescription" class="terminal-switch-dialog-description">{{ request?.message }}</p>
      <div class="terminal-switch-actions" role="group" aria-label="Подтверждение сброса состояния команды" style="grid-template-columns:repeat(2,minmax(0,1fr))">
        <button id="btnConfirmCommandStateReset" class="btn btn-danger" type="button" @click="settle(true)">ПОДТВЕРДИТЬ</button>
        <button id="btnCancelCommandStateReset" ref="cancelButton" autofocus class="btn" type="button" @click="settle(false)">ОТМЕНИТЬ</button>
      </div>
    </div>
  </dialog>
</template>
