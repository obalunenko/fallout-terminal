<script setup lang="ts">
import { computed } from 'vue';

import type { ContentNode } from '../../gen/fallout/terminal/player/v1/terminal_pb.js';

const props = defineProps<{
  readonly canControl: boolean;
  readonly index: number;
  readonly node: ContentNode;
  readonly selected: boolean;
}>();

const emit = defineEmits<{
  activate: [node: ContentNode];
  preview: [node: ContentNode];
}>();

const publicSelectorAttributes = computed(() => ({
  'data-idx': String(props.index),
  'data-node-id': props.node.id,
}));
const activate = (): void => {
  if (props.canControl) emit('activate', props.node);
};
</script>

<template>
  <div
    class="term-row"
    :class="{ sel: selected }"
    role="option"
    v-bind="publicSelectorAttributes"
    :aria-disabled="!canControl"
    :aria-selected="selected"
    :tabindex="canControl ? 0 : -1"
    @click="activate"
    @mouseover="emit('preview', node)"
    @focus="emit('preview', node)"
    @keydown.enter.prevent="activate"
    @keydown.space.prevent="activate"
  >&gt; {{ node.name }}</div>
</template>
