<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';

import type { CommandApprovalRequest } from '../composables/useCommandApproval.js';
import { dialogFocus } from '../directives/dialog-focus.js';

const props = defineProps<{
  readonly outcomeError: string;
  readonly pending: boolean;
  readonly request: CommandApprovalRequest | null;
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
const focusBinding = computed(() => ({
  active: open.value,
  initialFocus: () => approveButton.value,
  onCancel: () => emit('reject'),
}));

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
    dialog.value?.querySelector<HTMLButtonElement>('#btnRejectCommandExecution')?.focus();
  }
}

watch(open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
</script>

<template>
  <dialog
    id="commandExecutionDialog"
    ref="dialog"
    v-dialog-focus="focusBinding"
    class="terminal-switch-dialog command-execution-dialog"
    aria-modal="true"
    aria-labelledby="commandExecutionDialogTitle"
    aria-describedby="commandExecutionDialogDescription commandExecutionDialogStatus commandExecutionDialogError"
    :hidden="!open"
    @keydown="moveFocus"
  >
    <div class="terminal-switch-dialog-panel">
      <h2 id="commandExecutionDialogTitle" class="terminal-switch-dialog-title">ПОДТВЕРЖДЕНИЕ КОМАНДЫ</h2>
      <p id="commandExecutionDialogDescription" class="terminal-switch-dialog-description">{{ request?.confirmationText }}</p>
      <div class="terminal-switch-actions" role="group" aria-label="Решение мастера по выполнению команды" style="grid-template-columns:repeat(2,minmax(0,1fr))">
        <button id="btnApproveCommandExecution" ref="approveButton" class="btn btn-primary" type="button" :disabled="pending" @click="emit('approve')">ОДОБРИТЬ</button>
        <button id="btnRejectCommandExecution" class="btn btn-danger" type="button" :disabled="pending" @click="emit('reject')">ОТКЛОНИТЬ</button>
      </div>
      <div id="commandExecutionDialogStatus" class="terminal-switch-status" role="status" aria-live="polite" aria-atomic="true">{{ status }}</div>
      <div id="commandExecutionDialogError" class="terminal-switch-error" role="alert" aria-live="assertive" aria-atomic="true" hidden />
    </div>
  </dialog>
  <div v-if="outcomeError" class="coord-error" role="alert" aria-live="assertive" aria-atomic="true">{{ outcomeError }}</div>
</template>
