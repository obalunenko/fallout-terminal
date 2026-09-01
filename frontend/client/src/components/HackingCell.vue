<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  readonly character: number | undefined;
  readonly column: number | undefined;
  readonly disabled: boolean;
  readonly highlighted: boolean;
  readonly offset: number | undefined;
  readonly row: number | undefined;
  readonly target: string;
  readonly text: string;
  readonly type: 'filler' | 'word';
}>();

const geometryAttributes = computed(() => ({
  'data-character': props.character,
  'data-column': props.column,
  'data-offset': props.offset,
  'data-row': props.row,
  'data-target': props.target,
}));
</script>

<template>
  <span
    class="hcell"
    :class="[type, { hi: highlighted }]"
    v-bind="geometryAttributes"
    :aria-disabled="disabled"
    role="button"
    :tabindex="disabled ? -1 : 0"
  >{{ text }}</span>
</template>
