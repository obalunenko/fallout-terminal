<script setup lang="ts">
import { computed, onBeforeUpdate, onUpdated, ref } from 'vue';

import type { HackingAction, HackingView } from '../composables/useHackingSession.js';
import { useHackingBoardFit } from '../composables/useHackingBoardFit.js';
import {
  useHackingPointer,
  type HackingFocusIdentity,
  type HackingPointerTarget,
} from '../composables/useHackingPointer.js';
import HackingColumn from './HackingColumn.vue';

const props = defineProps<{
  readonly canControl: boolean;
  readonly contextKey: string;
  readonly generationKey?: string | undefined;
  readonly hack: Readonly<HackingView>;
  readonly highlightedKey: string;
  readonly hidden?: boolean;
  readonly visibleRows?: number | undefined;
}>();
const emit = defineEmits<{
  activate: [action: HackingAction];
  preview: [target: Readonly<HackingPointerTarget> | null];
}>();

const board = ref<HTMLElement | null>(null);
const contextAttributes = computed(() => ({ 'data-hacking-context': props.contextKey }));
const rowBases = computed<readonly number[]>(() => {
  let base = 0;
  return props.hack.columns.map(column => {
    const current = base;
    base += Math.ceil(column.text.length / 12);
    return current;
  });
});
const fit = useHackingBoardFit(board, {
  completeRowCounts: () => props.hack.columns.map(column => Math.ceil(column.text.length / 12)),
});
const pointer = useHackingPointer(board, {
  authorize: contextKey => props.canControl && contextKey === props.contextKey && !props.hack.solved && !props.hack.failed,
  contextKey: () => props.contextKey,
  onActivate: target => emit('activate', target.action),
  onPreview: target => emit('preview', target),
  patterns: () => props.hack.patterns,
});
let focus: Readonly<HackingFocusIdentity> | null = null;
onBeforeUpdate(() => { focus = pointer.captureFocus(); });
onUpdated(() => {
  pointer.restoreFocus(focus);
  focus = null;
  fit.schedule();
});
</script>

<template>
  <section
    id="hackBoard"
    ref="board"
    class="hack-board"
    :hidden="hidden === true"
    v-bind="contextAttributes"
    :aria-busy="!canControl"
  >
    <div id="hackColumns" class="hack-columns">
      <HackingColumn
        v-for="(column, columnIndex) in hack.columns"
        :key="`${generationKey ?? ''}:${columnIndex}`"
        :can-control="canControl"
        :column="column"
        :column-index="columnIndex"
        :highlighted-key="highlightedKey"
        :patterns="hack.patterns"
        :row-base="rowBases[columnIndex] ?? 0"
        :visible-rows="visibleRows"
      />
    </div>
    <slot />
  </section>
</template>
