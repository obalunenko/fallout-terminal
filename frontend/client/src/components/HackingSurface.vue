<script setup lang="ts">
import { computed } from 'vue';

import type { HackingAction, HackingView } from '../composables/useHackingSession.js';
import type { HackingPointerTarget } from '../composables/useHackingPointer.js';
import HackingBlocked from './HackingBlocked.vue';
import HackingBoard from './HackingBoard.vue';

const props = defineProps<{
  readonly canControl: boolean;
  readonly contextKey: string;
  readonly generationKey?: string | undefined;
  readonly hack: Readonly<HackingView> | null;
  readonly highlightedKey: string;
  readonly visibleRows?: number | undefined;
}>();
const emptyHack: Readonly<HackingView> = Object.freeze({
  attemptsLeft: 0,
  attemptsMax: 0,
  columns: Object.freeze([]),
  failed: false,
  level: 0,
  log: Object.freeze([]),
  patterns: Object.freeze([]),
  solved: false,
  wordLength: 0,
});
const activeHack = computed(() => props.hack ?? emptyHack);
const emit = defineEmits<{
  activate: [action: HackingAction];
  preview: [target: Readonly<HackingPointerTarget> | null];
}>();
</script>

<template>
  <HackingBoard
    :can-control="canControl"
    :context-key="contextKey"
    :generation-key="generationKey"
    :hack="activeHack"
    :hidden="hack === null || hack.failed"
    :highlighted-key="highlightedKey"
    :visible-rows="visibleRows"
    @activate="emit('activate', $event)"
    @preview="emit('preview', $event)"
  >
    <slot />
  </HackingBoard>
  <HackingBlocked :visible="hack?.failed === true" />
</template>
