<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  readonly kind?: 'error' | '';
  readonly message?: string;
}>();

const publicSelectorAttributes = computed(() => ({ 'data-kind': props.kind ?? '' }));
</script>

<template>
  <div
    id="playerNotice"
    class="player-notice"
    role="status"
    aria-live="polite"
    aria-atomic="true"
    v-bind="publicSelectorAttributes"
    :hidden="!message"
  >{{ message ?? '' }}</div>
</template>
