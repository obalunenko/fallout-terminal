<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';

import { dialogFocus } from '../directives/dialog-focus.js';

const props = defineProps<{
  readonly error: string;
  readonly pending: boolean;
  readonly switchId: string;
}>();

const emit = defineEmits<{
  cancel: [];
  discard: [];
  preserve: [];
}>();

const vDialogFocus = dialogFocus;
const dialog = ref<HTMLDialogElement | null>(null);
const preserveButton = ref<HTMLButtonElement | null>(null);
const open = computed(() => props.switchId !== '');
const focusBinding = computed(() => ({
  active: open.value,
  initialFocus: () => preserveButton.value,
  onCancel: () => emit('cancel'),
}));

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (open.value && !element.open) element.showModal();
  else if (!open.value && element.open) element.close();
}

watch(open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
</script>

<template>
  <dialog
    id="terminalSwitchDialog"
    ref="dialog"
    v-dialog-focus="focusBinding"
    class="terminal-switch-dialog"
    aria-modal="true"
    aria-labelledby="terminalSwitchDialogTitle"
    aria-describedby="terminalSwitchDialogDescription terminalSwitchStatus terminalSwitchError"
    :hidden="!open"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="terminalSwitchDialogTitle" class="terminal-switch-dialog-title">НЕЗАВЕРШЁННЫЙ ВЗЛОМ</h2>
      <p id="terminalSwitchDialogDescription" class="terminal-switch-dialog-description">
        Перед переключением терминала выберите, что сделать с текущей головоломкой.
      </p>
      <div class="terminal-switch-actions" role="group" aria-label="Решение для незавершённой головоломки">
        <button id="btnPreserveTerminalSwitch" ref="preserveButton" class="btn btn-primary" type="button" :disabled="pending" @click="emit('preserve')">СОХРАНИТЬ И ПЕРЕКЛЮЧИТЬ</button>
        <button id="btnDiscardTerminalSwitch" class="btn btn-danger terminal-switch-discard" type="button" :disabled="pending" @click="emit('discard')">СБРОСИТЬ И ПЕРЕКЛЮЧИТЬ</button>
        <button id="btnCancelTerminalSwitch" class="btn btn-secondary" type="button" :disabled="pending" @click="emit('cancel')">ОСТАТЬСЯ В ТЕРМИНАЛЕ</button>
      </div>
      <div id="terminalSwitchStatus" class="terminal-switch-status" role="status" aria-live="polite" aria-atomic="true">
        {{ pending ? 'ПРИМЕНЕНИЕ РЕШЕНИЯ...' : 'ИСХОДНЫЙ ТЕРМИНАЛ ОСТАЁТСЯ АКТИВНЫМ ДО ВЫБОРА' }}
      </div>
      <div id="terminalSwitchError" class="terminal-switch-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="!error">{{ error }}</div>
    </div>
  </dialog>
</template>
