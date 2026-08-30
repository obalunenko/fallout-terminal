<script setup lang="ts">
import type { PlayerTransport } from './ports/player-transport.js';

defineProps<{
  readonly transport: PlayerTransport;
}>();
</script>

<template>
  <div v-if="false" />
</template>
