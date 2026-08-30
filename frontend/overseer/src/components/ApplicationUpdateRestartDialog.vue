<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';

const props = defineProps<{
  readonly focusRequest: number;
  readonly open: boolean;
  readonly pending: boolean;
}>();

const emit = defineEmits<{
  postpone: [];
  restart: [];
}>();

const dialog = ref<HTMLDialogElement | null>(null);
const postponeButton = ref<HTMLButtonElement | null>(null);

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (props.open) {
    if (!element.open) element.showModal();
    postponeButton.value?.focus();
  } else if (element.open) {
    element.close();
  }
}

watch(() => props.open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
watch(() => props.focusRequest, () => { if (props.open) postponeButton.value?.focus(); });
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
</script>

<template>
  <dialog
    id="applicationUpdateRestartDialog"
    ref="dialog"
    class="application-update-dialog application-update-restart-dialog"
    aria-modal="true"
    aria-labelledby="applicationUpdateRestartDialogTitle"
    aria-describedby="applicationUpdateRestartDialogDescription"
    :aria-busy="pending"
    :hidden="!open"
    @cancel.prevent="emit('postpone')"
  >
    <div class="application-update-dialog-panel">
      <h2 id="applicationUpdateRestartDialogTitle" class="application-update-dialog-title">ОБНОВЛЕНИЕ ГОТОВО</h2>
      <p id="applicationUpdateRestartDialogDescription" class="application-update-dialog-description">Перезапустите приложение, чтобы применить подготовленное обновление. Перезапуск можно отложить и продолжить текущую работу.</p>
      <div class="application-update-actions" role="group" aria-label="Решение о перезапуске приложения">
        <button id="btnRestartApplicationUpdate" class="btn btn-primary" type="button" :disabled="pending" @click="emit('restart')">ПЕРЕЗАПУСТИТЬ И ОБНОВИТЬ</button>
        <button id="btnPostponeApplicationUpdate" ref="postponeButton" class="btn btn-secondary" type="button" :disabled="pending" @click="emit('postpone')">ПЕРЕЗАПУСТИТЬ ПОЗЖЕ</button>
      </div>
    </div>
  </dialog>
</template>
