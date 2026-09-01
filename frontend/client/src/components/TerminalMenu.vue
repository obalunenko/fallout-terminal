<script setup lang="ts">
import type { ContentNode } from '../../gen/fallout/terminal/player/v1/terminal_pb.js';
import TerminalMenuRow from './TerminalMenuRow.vue';

defineProps<{
  readonly canControl: boolean;
  readonly nodes: readonly ContentNode[];
  readonly selectedID: string;
  readonly visibleCount?: number | undefined;
}>();

const emit = defineEmits<{
  activate: [node: ContentNode];
  preview: [node: ContentNode];
}>();
</script>

<template>
  <div id="termList" class="term-list" role="listbox" aria-label="Содержимое терминала">
    <div v-if="nodes.length === 0" class="term-empty">[ ДИРЕКТОРИЯ ПУСТА ]</div>
    <TerminalMenuRow
      v-for="(node, index) in nodes.slice(0, visibleCount ?? nodes.length)"
      v-else
      :key="node.id"
      :can-control="canControl"
      :index="index"
      :node="node"
      :selected="node.id === selectedID"
      @activate="emit('activate', $event)"
      @preview="emit('preview', $event)"
    />
  </div>
</template>
