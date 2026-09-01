<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  readonly blocked: boolean;
  readonly hoverText: string;
  readonly typed: string;
}>();

const preview = computed(() => props.blocked ? '' : (props.typed !== '' ? props.typed : props.hoverText));
</script>

<template>
  <div class="hack-input-line" :aria-disabled="blocked">
    <span aria-hidden="true">&gt; </span>
    <output id="hackInputPreview" aria-live="polite" aria-atomic="true">{{ preview }}</output>
    <span v-if="!blocked" class="blink" aria-hidden="true">_</span>
  </div>
</template>
