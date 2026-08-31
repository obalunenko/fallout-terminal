<script setup lang="ts">
import type { TerminalSelectionRow } from '../composables/useTerminalSelection.js';
import TerminalActionMenu from './TerminalActionMenu.vue';

defineProps<{
  readonly terminal: TerminalSelectionRow;
}>();

const emit = defineEmits<{
  select: [terminalID: string];
}>();

function activate(terminalID: string, event?: KeyboardEvent): void {
  event?.preventDefault();
  emit('select', terminalID);
}
</script>

<template>
  <div
    class="term-row"
    :class="{ editing: terminal.selected, 'is-live': terminal.live }"
    :aria-current="terminal.selected"
    v-bind="{
      'data-group-id': terminal.groupID,
      'data-terminal-id': terminal.id,
    }"
    role="listitem"
    tabindex="0"
    @click="activate(terminal.id)"
    @keydown.enter="activate(terminal.id, $event)"
    @keydown.space="activate(terminal.id, $event)"
  >
    <div class="term-row-name">{{ terminal.name }}</div>
    <div class="term-row-meta">● В ЭФИРЕ</div>
    <TerminalActionMenu :terminal="terminal" />
  </div>
</template>
