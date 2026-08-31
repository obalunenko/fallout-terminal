<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';

import type { TerminalGroupAction, TerminalGroupRow } from '../composables/useTerminalGroups.js';
import type { TerminalSelectionRow } from '../composables/useTerminalSelection.js';
import TerminalRow from './TerminalRow.vue';

const props = defineProps<{
  readonly collapsed: boolean;
  readonly first: boolean;
  readonly group: TerminalGroupRow;
  readonly last: boolean;
  readonly menuOpen: boolean;
  readonly terminals: readonly TerminalSelectionRow[];
}>();

const emit = defineEmits<{
  action: [groupID: string, action: TerminalGroupAction];
  menuToggle: [groupID: string, open: boolean];
  select: [terminalID: string];
  toggle: [groupID: string];
}>();

const actionMenu = ref<HTMLDetailsElement | null>(null);

watch(() => props.menuOpen, open => {
  if (actionMenu.value !== null && actionMenu.value.open !== open) actionMenu.value.open = open;
});

function memberCount(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return `${count} ТЕРМИНАЛ`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${count} ТЕРМИНАЛА`;
  return `${count} ТЕРМИНАЛОВ`;
}

function menuToggled(event: Event): void {
  emit('menuToggle', props.group.id, (event.currentTarget as HTMLDetailsElement).open);
}

function request(action: TerminalGroupAction): void {
  emit('menuToggle', props.group.id, false);
  emit('action', props.group.id, action);
}

function closeFromKeyboard(): void {
  if (actionMenu.value !== null) actionMenu.value.open = false;
  emit('menuToggle', props.group.id, false);
  void nextTick(() => actionMenu.value?.querySelector<HTMLElement>('summary')?.focus());
}
</script>

<template>
  <section
    class="terminal-group"
    v-bind="{
      'data-collapsed': String(collapsed),
      'data-group-id': group.id,
      'data-singleton': String(group.terminalIDs.length === 1),
    }"
    role="listitem"
  >
    <header class="terminal-group-header">
      <button
        class="terminal-group-toggle"
        type="button"
        v-bind="{ 'data-action': 'toggle-terminal-group' }"
        :aria-controls="`terminal-group-members-${group.id}`"
        :aria-expanded="!collapsed"
        :aria-label="`${collapsed ? 'Развернуть' : 'Свернуть'} группу ${group.name}`"
        @click="emit('toggle', group.id)"
      >
        <span class="terminal-group-caret" aria-hidden="true">{{ collapsed ? '▸' : '▾' }}</span>
        <span class="terminal-group-name">{{ group.name }}</span>
      </button>
      <span class="terminal-group-member-count">{{ memberCount(group.terminalIDs.length) }}</span>
      <details ref="actionMenu" class="terminal-action-menu" @click.stop @keydown.esc.prevent.stop="closeFromKeyboard" @toggle="menuToggled">
        <summary
          class="terminal-action-menu-trigger"
          v-bind="{
            'data-action-menu-owner-id': group.id,
            'data-action-menu-trigger': 'terminal-group',
          }"
          :aria-label="`Действия: ${group.name}`"
        >•••</summary>
        <div class="terminal-action-menu-panel" role="menu" :aria-label="`Действия: ${group.name}`">
          <button v-bind="{ 'data-action': 'rename-terminal-group' }" class="btn btn-mini terminal-action-menu-item" type="button" role="menuitem" @click="request('rename-terminal-group')">ПЕРЕИМЕНОВАТЬ ГРУППУ</button>
          <button v-bind="{ 'data-action': 'move-terminal-group-up' }" class="btn btn-mini terminal-action-menu-item" type="button" role="menuitem" :disabled="first" @click="request('move-terminal-group-up')">ПЕРЕМЕСТИТЬ ГРУППУ ВВЕРХ</button>
          <button v-bind="{ 'data-action': 'move-terminal-group-down' }" class="btn btn-mini terminal-action-menu-item" type="button" role="menuitem" :disabled="last" @click="request('move-terminal-group-down')">ПЕРЕМЕСТИТЬ ГРУППУ ВНИЗ</button>
          <button v-bind="{ 'data-action': 'dissolve-terminal-group' }" class="btn btn-mini btn-danger terminal-action-menu-item terminal-action-menu-destructive" type="button" role="menuitem" @click="request('dissolve-terminal-group')">РАСФОРМИРОВАТЬ ГРУППУ</button>
        </div>
      </details>
    </header>
    <div
      :id="`terminal-group-members-${group.id}`"
      class="terminal-group-members"
      role="list"
      :aria-label="`Терминалы группы ${group.name}`"
      :hidden="collapsed"
    >
      <TerminalRow
        v-for="terminal in terminals"
        :key="terminal.id"
        :terminal="terminal"
        @select="emit('select', $event)"
      />
    </div>
  </section>
</template>
