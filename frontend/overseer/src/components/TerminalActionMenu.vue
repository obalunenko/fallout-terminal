<script setup lang="ts">
import { inject, nextTick, onMounted, onUnmounted, ref } from 'vue';

import type { TerminalSelectionRow } from '../composables/useTerminalSelection.js';
import { overseerControllerKey } from '../controllers/overseer-controller.js';

const props = defineProps<{
  readonly terminal: TerminalSelectionRow;
}>();

const details = ref<HTMLDetailsElement | null>(null);
const renameInput = ref<HTMLInputElement | null>(null);
const renaming = ref(false);
const renameDraft = ref('');
let cancelingRename = false;

const controller = inject(overseerControllerKey, null);

function closeMenu(restoreFocus = false): void {
  if (details.value === null) return;
  details.value.open = false;
  if (restoreFocus) details.value.querySelector<HTMLElement>('summary')?.focus();
}

function opened(): void {
  if (!details.value?.open) return;
  window.dispatchEvent(new CustomEvent('overseer-terminal-action-menu-open', {
    detail: props.terminal.id,
  }));
}

function otherMenuOpened(event: Event): void {
  const ownerID = (event as CustomEvent<unknown>).detail;
  if (ownerID !== props.terminal.id) closeMenu();
}

function documentClicked(event: MouseEvent): void {
  if (event.target instanceof Node && details.value?.contains(event.target)) return;
  closeMenu();
}

function documentKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Escape' || !details.value?.open) return;
  event.preventDefault();
  closeMenu(true);
}

function request(action: string): void {
  closeMenu(true);
  controller?.dispatch({
    action,
    kind: 'terminal-action-request',
    revision: props.terminal.revision,
    terminalID: props.terminal.id,
  });
}

function beginRename(): void {
  closeMenu();
  cancelingRename = false;
  renameDraft.value = props.terminal.name;
  renaming.value = true;
  void nextTick(() => {
    renameInput.value?.focus();
    renameInput.value?.select();
  });
}

function commitRename(): void {
  if (!renaming.value || cancelingRename) return;
  const name = renameDraft.value.trim();
  renaming.value = false;
  if (name !== '' && name !== props.terminal.name) {
    controller?.dispatch({
      action: 'rename-terminal',
      kind: 'terminal-action-request',
      name,
      revision: props.terminal.revision,
      terminalID: props.terminal.id,
    });
  }
  void nextTick(() => details.value?.querySelector<HTMLElement>('summary')?.focus());
}

function cancelRename(): void {
  if (!renaming.value) return;
  cancelingRename = true;
  renaming.value = false;
  void nextTick(() => {
    cancelingRename = false;
    details.value?.querySelector<HTMLElement>('summary')?.focus();
  });
}

onMounted(() => {
  document.addEventListener('click', documentClicked);
  document.addEventListener('keydown', documentKeydown);
  window.addEventListener('overseer-terminal-action-menu-open', otherMenuOpened);
});

onUnmounted(() => {
  document.removeEventListener('click', documentClicked);
  document.removeEventListener('keydown', documentKeydown);
  window.removeEventListener('overseer-terminal-action-menu-open', otherMenuOpened);
  renaming.value = false;
});
</script>

<template>
  <details ref="details" class="terminal-action-menu" @click.stop @toggle="opened">
    <summary
      class="terminal-action-menu-trigger"
      v-bind="{
        'data-action-menu-owner-id': terminal.id,
        'data-action-menu-trigger': 'terminal',
      }"
      :aria-label="`Действия: ${terminal.name}`"
    >•••</summary>
    <div class="terminal-action-menu-panel" role="menu" :aria-label="`Действия: ${terminal.name}`">
      <button v-bind="{ 'data-action': 'rename-terminal' }" class="btn btn-mini terminal-action-menu-item" type="button" role="menuitem" @click="beginRename">ПЕРЕИМЕНОВАТЬ ТЕРМИНАЛ</button>
      <button v-bind="{ 'data-action': 'move-terminal' }" class="btn btn-mini terminal-action-menu-item" type="button" role="menuitem" @click="request('move-terminal')">ПЕРЕМЕСТИТЬ В ДРУГУЮ ГРУППУ</button>
      <button v-bind="{ 'data-action': 'move-terminal-up' }" class="btn btn-mini terminal-action-menu-item" type="button" role="menuitem" :disabled="terminal.memberIndex === 0" @click="request('move-terminal-up')">ПЕРЕМЕСТИТЬ ТЕРМИНАЛ ВВЕРХ</button>
      <button v-bind="{ 'data-action': 'move-terminal-down' }" class="btn btn-mini terminal-action-menu-item" type="button" role="menuitem" :disabled="terminal.memberIndex === terminal.memberCount - 1" @click="request('move-terminal-down')">ПЕРЕМЕСТИТЬ ТЕРМИНАЛ ВНИЗ</button>
      <button v-bind="{ 'data-action': 'delete-terminal' }" class="btn btn-mini btn-danger terminal-action-menu-item terminal-action-menu-destructive" type="button" role="menuitem" @click="request('delete-terminal')">УДАЛИТЬ ТЕРМИНАЛ</button>
    </div>
  </details>
  <input
    v-if="renaming"
    ref="renameInput"
    v-model="renameDraft"
    class="field-input terminal-rename-input"
    maxlength="80"
    :aria-label="`Новое название: ${terminal.name}`"
    @blur="commitRename"
    @keydown.enter.prevent="commitRename"
    @keydown.esc.prevent="cancelRename"
  >
</template>
