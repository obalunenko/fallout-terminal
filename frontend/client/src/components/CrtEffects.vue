<script setup lang="ts">
import { ref, watch } from 'vue';

const props = defineProps<{
  readonly contextKey: string;
  readonly revealing: boolean;
}>();

const presenting = ref(props.revealing && props.contextKey !== '');
watch(() => [props.contextKey, props.revealing] as const, ([contextKey, revealing], previous) => {
  if (previous !== undefined && contextKey !== previous[0]) {
    presenting.value = false;
    return;
  }
  presenting.value = contextKey !== '' && revealing;
});
</script>

<template>
  <div class="scanlines" :class="{ 'crt-presenting': presenting }" aria-hidden="true" />
  <div class="vignette" :class="{ 'crt-presenting': presenting }" aria-hidden="true" />
</template>
