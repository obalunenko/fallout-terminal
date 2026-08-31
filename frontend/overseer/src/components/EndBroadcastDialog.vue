<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from 'vue';

const props = defineProps<{
  readonly open: boolean;
  readonly pending: boolean;
}>();

const emit = defineEmits<{
  cancel: [];
  confirm: [];
}>();

const dialog = ref<HTMLDialogElement | null>(null);
const cancelButton = ref<HTMLButtonElement | null>(null);

watch(() => props.open, async open => {
  if (!open) {
    if (dialog.value?.open) dialog.value.close();
    return;
  }
  await nextTick();
  if (dialog.value !== null && !dialog.value.open) dialog.value.showModal();
  cancelButton.value?.focus();
}, { immediate: true });

function nativeCancel(event: Event): void {
  event.preventDefault();
  if (!props.pending) emit('cancel');
}

onUnmounted(() => {
  if (dialog.value?.open) dialog.value.close();
  dialog.value = null;
  cancelButton.value = null;
});
</script>

<template>
  <dialog
    id="endBroadcastDialog"
    ref="dialog"
    class="terminal-switch-dialog end-broadcast-dialog"
    aria-modal="true"
    aria-labelledby="endBroadcastDialogTitle"
    aria-describedby="endBroadcastDialogDescription"
    :hidden="!open"
    @cancel="nativeCancel"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="endBroadcastDialogTitle" class="terminal-switch-dialog-title">ЗАВЕРШИТЬ ТРАНСЛЯЦИЮ?</h2>
      <p id="endBroadcastDialogDescription" class="terminal-switch-dialog-description">Активный терминал будет убран у всех игроков. Назначения персонажей и управление будут сброшены, а сессии и список персонажей сохранятся.</p>
      <div class="end-broadcast-actions" role="group" aria-label="Подтверждение завершения трансляции">
        <button id="btnCancelEndBroadcast" ref="cancelButton" class="btn btn-secondary" type="button" :disabled="pending" @click="emit('cancel')">ОТМЕНА</button>
        <button id="btnConfirmEndBroadcast" class="btn btn-danger" type="button" :disabled="pending" @click="emit('confirm')">ЗАВЕРШИТЬ ТРАНСЛЯЦИЮ</button>
      </div>
    </div>
  </dialog>
</template>
