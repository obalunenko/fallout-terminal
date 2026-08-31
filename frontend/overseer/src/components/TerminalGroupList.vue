<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue';

import type { TerminalGroupAction, TerminalGroupRow } from '../composables/useTerminalGroups.js';
import type { TerminalSelectionRow } from '../composables/useTerminalSelection.js';
import TerminalGroupRowComponent from './TerminalGroupRow.vue';

const props = defineProps<{
  readonly collapsedGroupIDs: ReadonlySet<string>;
  readonly focusRequest: Readonly<{ ownerID: string; scope: string }> | null;
  readonly groups: readonly TerminalGroupRow[];
  readonly terminals: readonly TerminalSelectionRow[];
}>();

const emit = defineEmits<{
  action: [groupID: string, action: TerminalGroupAction];
  select: [terminalID: string];
  toggle: [groupID: string];
}>();

const openGroupID = ref<string | null>(null);

function ownedList(): HTMLElement | null {
  return document.getElementById('termList');
}

function focusTrigger(scope: string, ownerID: string): void {
  const escaped = CSS.escape(ownerID);
  ownedList()?.querySelector<HTMLElement>(
    `[data-action-menu-trigger="${scope}"][data-action-menu-owner-id="${escaped}"]`,
  )?.focus();
}

function closeMenu(restoreFocus: boolean): void {
  const groupID = openGroupID.value;
  if (groupID === null) return;
  openGroupID.value = null;
  if (restoreFocus) void nextTick(() => focusTrigger('terminal-group', groupID));
}

function documentClicked(event: MouseEvent): void {
  if (event.target instanceof Element && event.target.closest('.terminal-action-menu')) return;
  closeMenu(false);
}

function documentKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Escape' || openGroupID.value === null) return;
  event.preventDefault();
  closeMenu(true);
}

function menuChanged(groupID: string, open: boolean): void {
  if (open) openGroupID.value = groupID;
  else if (openGroupID.value === groupID) openGroupID.value = null;
}

function forwardAction(groupID: string, action: TerminalGroupAction): void {
  emit('action', groupID, action);
}

function terminalsFor(group: TerminalGroupRow): readonly TerminalSelectionRow[] {
  const byID = new Map(props.terminals.map(terminal => [terminal.id, terminal]));
  return group.terminalIDs.flatMap(id => {
    const terminal = byID.get(id);
    return terminal ? [terminal] : [];
  });
}

watch(() => props.focusRequest, async request => {
  if (request === null) return;
  await nextTick();
  focusTrigger(request.scope, request.ownerID);
});

watch(() => props.groups, groups => {
  if (openGroupID.value !== null && !groups.some(group => group.id === openGroupID.value)) {
    openGroupID.value = null;
  }
});

onMounted(() => {
  document.addEventListener('click', documentClicked);
  document.addEventListener('keydown', documentKeydown);
});

onUnmounted(() => {
  document.removeEventListener('click', documentClicked);
  document.removeEventListener('keydown', documentKeydown);
  openGroupID.value = null;
});
</script>

<template>
  <div v-if="groups.length === 0" class="tree-empty-hint">Нет терминалов</div>
  <TerminalGroupRowComponent
    v-for="(group, index) in groups"
    :key="group.id"
    :collapsed="collapsedGroupIDs.has(group.id)"
    :first="index === 0"
    :group="group"
    :last="index === groups.length - 1"
    :menu-open="openGroupID === group.id"
    :terminals="terminalsFor(group)"
    @action="forwardAction"
    @menu-toggle="menuChanged"
    @select="emit('select', $event)"
    @toggle="emit('toggle', $event)"
  />
</template>
