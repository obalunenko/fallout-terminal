<script setup lang="ts">
import { computed } from 'vue';

import type { HackingColumnView, HackingPatternView } from '../composables/useHackingSession.js';
import HackingRow from './HackingRow.vue';

const props = defineProps<{
  readonly canControl: boolean;
  readonly column: Readonly<HackingColumnView>;
  readonly columnIndex: number;
  readonly highlightedKey: string;
  readonly patterns: readonly HackingPatternView[];
  readonly rowBase: number;
  readonly visibleRows?: number | undefined;
}>();

const rowCount = computed(() => Math.ceil(props.column.text.length / 12));
const publicSelectorAttributes = computed(() => ({ 'data-hack-column': String(props.columnIndex) }));
</script>

<template>
  <div class="hack-col" v-bind="publicSelectorAttributes">
    <HackingRow
      v-for="rowIndex in Math.min(rowCount, Math.max(0, (visibleRows ?? rowBase + rowCount) - rowBase))"
      :key="`${columnIndex}:${rowIndex - 1}`"
      :can-control="canControl"
      :column="column"
      :column-index="columnIndex"
      :highlighted-key="highlightedKey"
      :patterns="patterns"
      :row-base="rowBase"
      :row-index="rowIndex - 1"
    />
  </div>
</template>
