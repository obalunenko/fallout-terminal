<script setup lang="ts">
import type { TerminalGroupAction, TerminalGroupRow } from '../composables/useTerminalGroups.js';
import type { TerminalSelectionRow } from '../composables/useTerminalSelection.js';
import TerminalGroupList from './TerminalGroupList.vue';

defineProps<{
  readonly collapsedGroupIDs: ReadonlySet<string>;
  readonly focusRequest: Readonly<{ ownerID: string; scope: string }> | null;
  readonly groupRevision: number;
  readonly groups: readonly TerminalGroupRow[];
  readonly revision: number;
  readonly terminals: readonly TerminalSelectionRow[];
}>();

const emit = defineEmits<{
  action: [groupID: string, action: TerminalGroupAction];
  select: [terminalID: string];
  toggle: [groupID: string];
}>();

function forwardAction(groupID: string, action: TerminalGroupAction): void {
  emit('action', groupID, action);
}
</script>

<template>
  <Teleport to="#termList">
    <span
      hidden
      v-bind="{
        'data-group-revision': String(groupRevision),
        'data-selection-revision': String(revision),
        'data-stale-suppression': 'enforced',
      }"
    ></span>
    <TerminalGroupList
      :collapsed-group-i-ds="collapsedGroupIDs"
      :focus-request="focusRequest"
      :groups="groups"
      :terminals="terminals"
      @action="forwardAction"
      @select="emit('select', $event)"
      @toggle="emit('toggle', $event)"
    />
  </Teleport>
</template>
