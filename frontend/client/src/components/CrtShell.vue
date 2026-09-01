<script setup lang="ts">
import CrtEffects from './CrtEffects.vue';

withDefaults(defineProps<{
  readonly contextKey?: string;
  readonly observerReadOnly?: boolean;
  readonly pending?: boolean;
  readonly revealing?: boolean;
}>(), {
  contextKey: '',
  observerReadOnly: false,
  pending: false,
  revealing: false,
});
</script>

<template>
  <div class="crt">
    <div
      id="screen"
      class="screen"
      :class="{ 'observer-read-only': observerReadOnly, 'shared-input-pending': pending }"
      :aria-readonly="observerReadOnly === true"
    >
      <slot />
      <CrtEffects :context-key="contextKey" :revealing="revealing" />
    </div>
  </div>
</template>
