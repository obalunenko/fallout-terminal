<script setup lang="ts">
defineProps<{
  readonly lines: readonly string[];
}>();
</script>

<template>
  <div id="hackLog" class="hack-log" role="log" aria-live="polite" aria-relevant="additions text">
    <div v-for="(line, index) in lines" :key="`${index}:${line}`">{{ line }}</div>
  </div>
</template>
