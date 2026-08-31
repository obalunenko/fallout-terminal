<script setup lang="ts">
import type { TerminalTreeNodeView } from './TerminalTree.vue';

defineOptions({ name: 'TerminalTreeNode' });

defineProps<{
  readonly expandedIDs: ReadonlySet<string>;
  readonly isRoot?: boolean;
  readonly node: TerminalTreeNodeView;
  readonly selectedNodeID: string | null;
}>();

const emit = defineEmits<{
  select: [nodeID: string];
  toggle: [nodeID: string];
}>();
</script>

<template>
  <div class="tree-node" v-bind="{ 'data-node-id': node.id }">
    <div
      class="tree-row"
      :class="{
        selected: selectedNodeID === node.id,
        'command-completed': node.completed,
      }"
      v-bind="{ 'data-node-key': node.id }"
      tabindex="-1"
      @click="emit('select', node.id)"
    >
      <button
        v-if="node.type === 'folder'"
        class="tree-caret tree-caret-button"
        type="button"
        :aria-expanded="expandedIDs.has(node.id)"
        :aria-label="`${expandedIDs.has(node.id) ? 'Свернуть' : 'Развернуть'} ${isRoot ? 'ROOT' : node.name}`"
        :disabled="node.children.length === 0"
        @click.stop="emit('toggle', node.id)"
      >{{ node.children.length === 0 ? '·' : expandedIDs.has(node.id) ? '▾' : '▸' }}</button>
      <span v-else class="tree-caret" aria-hidden="true"></span>
      <span class="tree-icon" :class="node.type">{{ node.type === 'folder' ? '[ПАПКА]' : node.type === 'command' ? '[КОМАНДА]' : '[ЗАПИСЬ]' }}</span>
      <span class="tree-label">{{ isRoot ? 'ROOT' : node.displayName }}</span>
    </div>
    <div v-if="node.type === 'folder' && expandedIDs.has(node.id)" class="tree-children">
      <div v-if="node.children.length === 0" class="tree-empty-hint">(пусто)</div>
      <TerminalTreeNode
        v-for="child in node.children"
        :key="child.id"
        :expanded-i-ds="expandedIDs"
        :node="child"
        :selected-node-i-d="selectedNodeID"
        @select="emit('select', $event)"
        @toggle="emit('toggle', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.tree-caret-button {
  border: 0;
  padding: 0;
  background: transparent;
  font: inherit;
  text-align: left;
}
</style>
