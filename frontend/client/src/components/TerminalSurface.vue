<script setup lang="ts">
import type { ContentNode } from '../../gen/fallout/terminal/player/v1/terminal_pb.js';
import TerminalMenu from './TerminalMenu.vue';

defineProps<{
  readonly canControl: boolean;
  readonly nodes: readonly ContentNode[];
  readonly observerReadOnly?: boolean;
  readonly pending?: boolean;
  readonly selectedID: string;
  readonly visibleCount?: number | undefined;
}>();

const emit = defineEmits<{
  activate: [node: ContentNode];
  preview: [node: ContentNode];
}>();
</script>

<template>
  <div
    class="terminal-surface"
    :class="{ 'observer-read-only': observerReadOnly, 'shared-input-pending': pending }"
    :aria-readonly="observerReadOnly === true"
  >
    <TerminalMenu
      :can-control="canControl && pending !== true"
      :nodes="nodes"
      :selected-i-d="selectedID"
      :visible-count="visibleCount"
      @activate="emit('activate', $event)"
      @preview="emit('preview', $event)"
    />
  </div>
</template>
