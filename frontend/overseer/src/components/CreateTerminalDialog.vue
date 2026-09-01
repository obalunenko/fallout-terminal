<script setup lang="ts">
import { inject, nextTick, onUnmounted, ref } from 'vue';

import { overseerControllerKey } from '../controllers/overseer-controller.js';

const controller = inject(overseerControllerKey, null);
const dialog = ref<HTMLDialogElement | null>(null);
const nameInput = ref<HTMLInputElement | null>(null);
const name = ref('');
const error = ref('');
const open = ref(false);
const pending = ref(false);
const revision = ref(-1);

function closeNativeDialog(): void {
  if (dialog.value?.open) dialog.value.close();
}

const release = controller?.subscribeState(message => {
  if (message.kind !== 'create-terminal-snapshot'
    || !Number.isSafeInteger(message.revision) || Number(message.revision) <= revision.value) return;
  revision.value = Number(message.revision);
  pending.value = message.pending === true;
  const nextOpen = message.open === true;
  if (nextOpen && !open.value) {
    name.value = '';
    error.value = '';
  }
  open.value = nextOpen;
  if (!nextOpen) {
    closeNativeDialog();
    return;
  }
  void nextTick(() => {
    if (dialog.value !== null && !dialog.value.open) dialog.value.showModal();
    nameInput.value?.focus();
  });
});

function request(action: 'cancel' | 'create', terminalName = ''): void {
  controller?.dispatch({
    action,
    kind: 'create-terminal-action-request',
    name: terminalName,
    revision: revision.value,
  });
}

function cancel(): void {
  if (!pending.value) request('cancel');
}

function submit(): void {
  if (pending.value) return;
  const trimmedName = name.value.trim();
  if (trimmedName === '') {
    error.value = 'УКАЖИТЕ НАЗВАНИЕ ТЕРМИНАЛА';
    void nextTick(() => nameInput.value?.focus());
    return;
  }
  error.value = '';
  request('create', trimmedName);
}

function nativeCancel(event: Event): void {
  event.preventDefault();
  cancel();
}

onUnmounted(() => {
  release?.();
  closeNativeDialog();
  dialog.value = null;
  nameInput.value = null;
});
</script>

<template>
  <dialog
    id="createTerminalDialog"
    ref="dialog"
    class="terminal-switch-dialog create-terminal-dialog"
    aria-modal="true"
    aria-labelledby="createTerminalDialogTitle"
    aria-describedby="createTerminalDialogDescription createTerminalError"
    :hidden="!open"
    v-bind="{
      'data-create-revision': String(revision),
      'data-stale-suppression': 'enforced',
    }"
    @cancel="nativeCancel"
  >
    <form id="createTerminalForm" class="terminal-switch-dialog-panel" method="dialog" novalidate @submit.prevent="submit">
      <h2 id="createTerminalDialogTitle" class="terminal-switch-dialog-title">СОЗДАТЬ ТЕРМИНАЛ</h2>
      <p id="createTerminalDialogDescription" class="terminal-switch-dialog-description">Новый терминал будет сохранён как черновик и не появится у игроков, пока вы не сделаете его активным.</p>
      <label class="field-label-inline" for="createTerminalName">НАЗВАНИЕ ТЕРМИНАЛА</label>
      <input id="createTerminalName" ref="nameInput" v-model="name" class="field-input" name="terminalName" type="text" maxlength="80" autocomplete="off" required aria-describedby="createTerminalError">
      <div id="createTerminalError" class="terminal-switch-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
      <div class="dialog-actions" role="group" aria-label="Создание терминала">
        <button id="btnCancelCreateTerminal" class="btn btn-secondary" type="button" :disabled="pending" @click="cancel">ОТМЕНА</button>
        <button id="btnConfirmCreateTerminal" class="btn btn-primary" type="submit" :disabled="pending">СОЗДАТЬ ТЕРМИНАЛ</button>
      </div>
    </form>
  </dialog>
</template>
