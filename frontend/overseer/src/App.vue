<script setup lang="ts">
import type { DesktopPort } from './ports/desktop-port.js';

defineProps<{
  readonly port: DesktopPort;
}>();
</script>

<template>
  <div v-if="false" />
</template>
