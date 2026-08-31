<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from 'vue';

const props = defineProps<{
  readonly error: string;
  readonly open: boolean;
  readonly pending: boolean;
}>();

const emit = defineEmits<{
  cancel: [];
  confirm: [];
}>();

const dialog = ref<HTMLDialogElement | null>(null);
const cancelButton = ref<HTMLButtonElement | null>(null);
const confirmButton = ref<HTMLButtonElement | null>(null);

watch(() => props.open, async open => {
  if (!open) {
    if (dialog.value?.open) dialog.value.close();
    return;
  }
  await nextTick();
  if (dialog.value !== null && !dialog.value.open) dialog.value.showModal();
  cancelButton.value?.focus();
}, { immediate: true });

watch(() => props.error, async error => {
  if (error === '' || !props.open) return;
  await nextTick();
  confirmButton.value?.focus();
});

function nativeCancel(event: Event): void {
  event.preventDefault();
  if (!props.pending) emit('cancel');
}

onUnmounted(() => {
  if (dialog.value?.open) dialog.value.close();
  dialog.value = null;
  cancelButton.value = null;
  confirmButton.value = null;
});
</script>

<template>
  <dialog
    id="takeOffAirDialog"
    ref="dialog"
    class="terminal-switch-dialog take-off-air-dialog"
    aria-modal="true"
    aria-labelledby="takeOffAirDialogTitle"
    aria-describedby="takeOffAirDialogDescription takeOffAirError"
    :hidden="!open"
    @cancel="nativeCancel"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="takeOffAirDialogTitle" class="terminal-switch-dialog-title">СНЯТЬ ТЕРМИНАЛ С ЭФИРА?</h2>
      <p id="takeOffAirDialogDescription" class="terminal-switch-dialog-description">Игроки перестанут видеть активный терминал. Трансляция, подключения, роли, назначения и сохранённый терминал останутся без изменений.</p>
      <div id="takeOffAirError" class="terminal-switch-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
      <div class="dialog-actions" role="group" aria-label="Подтверждение снятия терминала с эфира">
        <button id="btnCancelTakeOffAir" ref="cancelButton" class="btn btn-secondary" type="button" :disabled="pending" @click="emit('cancel')">ОТМЕНА</button>
        <button id="btnConfirmTakeOffAir" ref="confirmButton" class="btn btn-danger" type="button" :disabled="pending" @click="emit('confirm')">СНЯТЬ С ЭФИРА</button>
      </div>
    </div>
  </dialog>
</template>
