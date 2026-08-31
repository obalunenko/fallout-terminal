<script setup lang="ts">
defineProps<{
  readonly loaded: boolean;
}>();
</script>

<template>
  <div id="overseerRuntimeShell" v-bind="{ 'data-view': loaded ? 'main' : 'start' }" style="display: contents">
    <slot />
  </div>
</template>
