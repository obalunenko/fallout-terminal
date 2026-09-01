<script setup lang="ts">
import { computed } from 'vue';

import type { HackingColumnView, HackingPatternView } from '../composables/useHackingSession.js';
import HackingCell from './HackingCell.vue';

interface CellSegment {
  readonly character?: number;
  readonly key: string;
  readonly offset?: number;
  readonly target: string;
  readonly text: string;
  readonly type: 'filler' | 'word';
}

const props = defineProps<{
  readonly canControl: boolean;
  readonly column: Readonly<HackingColumnView>;
  readonly columnIndex: number;
  readonly highlightedKey: string;
  readonly patterns: readonly HackingPatternView[];
  readonly rowBase: number;
  readonly rowIndex: number;
}>();

const rowWidth = 12;
const rowStart = computed(() => props.rowIndex * rowWidth);
const rowEnd = computed(() => Math.min(rowStart.value + rowWidth, props.column.text.length));
const publicSelectorAttributes = computed(() => ({ 'data-hack-row': `${props.columnIndex}:${props.rowIndex}` }));
const segments = computed<readonly CellSegment[]>(() => {
  const result: CellSegment[] = [];
  let index = rowStart.value;
  while (index < rowEnd.value) {
    const word = props.column.words.find(candidate => index >= candidate.start && index < candidate.start + candidate.length);
    if (word !== undefined) {
      const end = Math.min(rowEnd.value, word.start + word.length);
      result.push(Object.freeze({
        key: `word:${word.id}:${index}`,
        target: word.id,
        text: props.column.text.slice(index, end),
        type: 'word',
      }));
      index = end;
      continue;
    }
    result.push(Object.freeze({
      character: index,
      key: `filler:${props.columnIndex}:${index}`,
      offset: index - rowStart.value,
      target: `${props.columnIndex}:${index}`,
      text: props.column.text[index] ?? '',
      type: 'filler',
    }));
    index += 1;
  }
  return Object.freeze(result);
});
const highlighted = (segment: CellSegment): boolean => {
  if (segment.type === 'word') return props.highlightedKey === `word:${segment.target}`;
  if (props.highlightedKey === `filler:${props.columnIndex}:${segment.character}`) return true;
  const patternID = props.highlightedKey.startsWith('pattern:')
    ? props.highlightedKey.slice('pattern:'.length)
    : '';
  const pattern = props.patterns.find(candidate => candidate.id === patternID);
  return pattern !== undefined && pattern.row === props.rowBase + props.rowIndex &&
    segment.offset !== undefined && segment.offset >= pattern.start && segment.offset <= pattern.end;
};
</script>

<template>
  <div class="hack-row" v-bind="publicSelectorAttributes">
    <span class="hack-addr">{{ column.addresses[rowIndex] ?? '' }}</span>
    <span class="hack-cells">
      <HackingCell
        v-for="segment in segments"
        :key="segment.key"
        :character="segment.character"
        :column="segment.type === 'filler' ? columnIndex : undefined"
        :disabled="!canControl"
        :highlighted="highlighted(segment)"
        :offset="segment.offset"
        :row="segment.type === 'filler' ? rowBase + rowIndex : undefined"
        :target="segment.target"
        :text="segment.text"
        :type="segment.type"
      />
    </span>
  </div>
</template>
