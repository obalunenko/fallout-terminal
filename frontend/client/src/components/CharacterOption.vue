<script setup lang="ts">
import { computed } from 'vue';

import type { PlayerRosterIdentity } from '../composables/usePlayerIdentity.js';

const props = defineProps<{
  readonly entry: PlayerRosterIdentity;
  readonly pending?: boolean;
}>();

const emit = defineEmits<{
  select: [characterID: string];
}>();

const publicSelectorAttributes = computed(() => ({
  'data-character-id': props.entry.id,
  'data-status': props.entry.status,
}));
</script>

<template>
  <button
    type="button"
    class="character-option"
    role="listitem"
    v-bind="publicSelectorAttributes"
    :disabled="pending === true || entry.status !== 'available'"
    @click="emit('select', entry.id)"
  >{{ entry.name }}<span v-if="entry.status === 'claimed'" class="character-option-status">ЗАНЯТ</span></button>
</template>
