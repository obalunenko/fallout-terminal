<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';

import type { TerminalNavigationRequest } from '../composables/useTerminalNavigationApproval.js';
import { dialogFocus } from '../directives/dialog-focus.js';

const props = defineProps<{
  readonly outcomeError: string;
  readonly pending: boolean;
  readonly request: TerminalNavigationRequest | null;
  readonly status: string;
}>();

const emit = defineEmits<{
  approve: [];
  reject: [];
}>();

const vDialogFocus = dialogFocus;
const dialog = ref<HTMLDialogElement | null>(null);
const approveButton = ref<HTMLButtonElement | null>(null);
const open = computed(() => props.request !== null);
const direction = computed(() => props.request?.direction === 'return' ? 'ВОЗВРАТ' : 'ПЕРЕХОД');
const focusBinding = computed(() => ({
  active: open.value,
  initialFocus: () => approveButton.value,
  onCancel: () => emit('reject'),
}));

function label(primary: string | undefined, fallback: string | undefined): string {
  return primary || fallback || '—';
}

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (open.value && !element.open) element.showModal();
  else if (!open.value && element.open) element.close();
}

function moveFocus(event: KeyboardEvent): void {
  if (event.key === 'ArrowLeft') {
    event.preventDefault();
    approveButton.value?.focus();
  } else if (event.key === 'ArrowRight') {
    event.preventDefault();
    dialog.value?.querySelector<HTMLButtonElement>('#btnRejectTerminalNavigation')?.focus();
  }
}

watch(open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
</script>

<template>
  <dialog
    id="terminalNavigationDialog"
    ref="dialog"
    v-dialog-focus="focusBinding"
    class="terminal-switch-dialog terminal-navigation-dialog"
    aria-modal="true"
    aria-labelledby="terminalNavigationDialogTitle"
    :hidden="!open"
    @keydown="moveFocus"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="terminalNavigationDialogTitle" class="terminal-switch-dialog-title">ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ</h2>
      <div id="terminalNavigationSummary" class="terminal-navigation-summary">
        <div>ЗАПРОС: {{ request?.requestId }}</div>
        <div class="terminal-navigation-direction">{{ direction }}</div>
        <div>ИЗ: {{ label(request?.sourceTerminalName, request?.sourceTerminalId) }}</div>
        <div>КОМАНДА: {{ label(request?.commandName, request?.commandId) }}</div>
        <div>В: {{ label(request?.targetTerminalName, request?.targetTerminalId) }}</div>
      </div>
      <div class="terminal-switch-actions" role="group" aria-label="Решение мастера по переходу" style="grid-template-columns:repeat(2,minmax(0,1fr))">
        <button id="btnApproveTerminalNavigation" ref="approveButton" class="btn btn-primary" type="button" :disabled="pending" @click="emit('approve')">ОДОБРИТЬ</button>
        <button id="btnRejectTerminalNavigation" class="btn btn-danger" type="button" :disabled="pending" @click="emit('reject')">ОТКЛОНИТЬ</button>
      </div>
      <div id="terminalNavigationStatus" class="terminal-switch-status" role="status" aria-live="polite">{{ status }}</div>
      <div id="terminalNavigationError" class="terminal-switch-error" role="alert" hidden />
    </div>
  </dialog>
  <div v-if="outcomeError" class="coord-error" role="alert" aria-live="assertive">{{ outcomeError }}</div>
</template>
