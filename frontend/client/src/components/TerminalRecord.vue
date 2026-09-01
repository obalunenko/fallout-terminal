<script setup lang="ts">
import { computed, ref, toRef, watch } from 'vue';

import { usePaginationMeasurement } from '../composables/usePaginationMeasurement.js';

const props = defineProps<{
  readonly pageIndex: number;
  readonly pending?: boolean;
  readonly text: string;
  readonly title: string;
  readonly visibleLines?: number;
}>();

const emit = defineEmits<{
  pageCount: [count: number];
}>();

const body = ref<HTMLElement | null>(null);
const pagination = usePaginationMeasurement(body, toRef(props, 'text'));
const page = computed(() => pagination.pages.value[Math.min(
  Math.max(0, props.pageIndex),
  pagination.pages.value.length - 1,
)] ?? '');
const pageLines = computed(() => page.value.split('\n'));
const pageStart = computed(() => pagination.pages.value
  .slice(0, Math.min(Math.max(0, props.pageIndex), pagination.pages.value.length - 1))
  .reduce((total, value) => total + value.split('\n').length, 0));
const visiblePageLines = computed(() => pageLines.value.slice(
  0,
  Math.max(0, (props.visibleLines ?? Number.MAX_SAFE_INTEGER) - pageStart.value),
));
watch(pagination.pages, pages => { emit('pageCount', Math.max(1, pages.length)); });
</script>

<template>
  <section id="termEntry" class="term-entry" :aria-busy="pending === true">
    <h1 id="entryTitle" class="entry-title">{{ title }}</h1>
    <div class="entry-divider">════════════════════════════</div>
    <div id="entryBody" ref="body" class="entry-body">
      <div v-for="(line, index) in visiblePageLines" :key="index">{{ line || '\u00a0' }}</div>
    </div>
  </section>
</template>
