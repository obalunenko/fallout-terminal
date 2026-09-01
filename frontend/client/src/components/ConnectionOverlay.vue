<script setup lang="ts">
defineProps<{
  readonly message: string;
  readonly visible: boolean;
}>();
</script>

<template>
  <div id="connOverlay" class="conn-overlay" :class="{ hidden: !visible }" role="status" aria-live="assertive" aria-atomic="true">
    <div id="connText" class="conn-text">{{ message }}</div>
  </div>
</template>
