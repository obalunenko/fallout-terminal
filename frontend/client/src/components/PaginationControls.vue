<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  readonly canControl: boolean;
  readonly pageCount: number;
  readonly pageIndex: number;
}>();

const emit = defineEmits<{
  page: [pageIndex: number];
}>();

const normalizedCount = computed(() => Math.max(1, props.pageCount));
const previous = (): void => {
  if (props.canControl && props.pageIndex > 0) emit('page', props.pageIndex - 1);
};
const next = (): void => {
  if (props.canControl && props.pageIndex + 1 < normalizedCount.value) emit('page', props.pageIndex + 1);
};
</script>

<template>
  <nav id="pageNav" class="page-nav" aria-label="Навигация по страницам" :hidden="pageCount <= 1">
    <button
      id="pagePrev"
      class="page-btn"
      type="button"
      aria-label="Предыдущая страница"
      :disabled="!canControl"
      :hidden="pageIndex <= 0"
      @click="previous"
    >[ ПРЕД. ]</button>
    <output id="pageIndicator" class="page-indicator" aria-live="polite">{{ pageIndex + 1 }} / {{ normalizedCount }}</output>
    <button
      id="pageNext"
      class="page-btn"
      type="button"
      aria-label="Следующая страница"
      :disabled="!canControl"
      :hidden="pageIndex + 1 >= normalizedCount"
      @click="next"
    >[ СЛЕД. ]</button>
  </nav>
</template>
