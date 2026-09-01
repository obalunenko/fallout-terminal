<script setup lang="ts">
import { computed } from 'vue';

import PaginationControls from './PaginationControls.vue';

const props = defineProps<{
  readonly backLabel?: string;
  readonly canControl: boolean;
  readonly pageCount: number;
  readonly pageIndex: number;
  readonly showBack?: boolean;
}>();

const backAccessibleLabel = computed(() => props.backLabel?.replace(/^\[\s*|\s*\]$/gu, ''));

const emit = defineEmits<{
  back: [];
  page: [pageIndex: number];
}>();
</script>

<template>
  <footer class="term-footer">
    <button
      id="backBtn"
      class="back-btn"
      type="button"
      :aria-label="backAccessibleLabel"
      :disabled="!canControl"
      :hidden="showBack !== true"
      @click="emit('back')"
    >{{ backLabel ?? '[ НАЗАД ]' }}</button>
    <PaginationControls
      :can-control="canControl"
      :page-count="pageCount"
      :page-index="pageIndex"
      @page="emit('page', $event)"
    />
  </footer>
</template>
