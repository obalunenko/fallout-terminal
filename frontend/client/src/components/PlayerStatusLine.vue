<script setup lang="ts">
import { computed } from 'vue';

import type { PlayerIdentityState } from '../composables/usePlayerIdentity.js';

const props = defineProps<{
  readonly identity: Readonly<PlayerIdentityState>;
}>();

const compactFallbackName = computed(() => {
  const match = /^PLAYER\s+(\d+)$/iu.exec(props.identity.fallbackName.trim());
  return match === null ? props.identity.fallbackName.trim() || 'P?' : `P${match[1]}`;
});
const roleLabel = computed(() => ({
  active: 'АКТИВЕН',
  observer: 'НАБЛЮДАТЕЛЬ',
  unassigned: 'НЕ НАЗНАЧЕН',
})[props.identity.role]);
const roleAttributes = computed(() => ({ 'data-role': props.identity.role }));
</script>

<template>
  <section
    id="playerIdentity"
    class="player-status-line"
    aria-label="Идентификация игрока"
    role="status"
    aria-live="polite"
    aria-atomic="true"
  >
    <span class="player-status-prefix">[СИСТЕМА] ВВОД&nbsp;</span>
    <span id="playerFallbackName" class="player-status-input">{{ compactFallbackName }}</span>
    <span id="playerCharacterSeparator" class="player-status-separator" :hidden="identity.assigned === null">&nbsp;//&nbsp;</span>
    <span id="playerCharacterName" class="player-status-character" :hidden="identity.assigned === null">{{ identity.assigned?.name ?? '' }}</span>
    <span class="player-status-separator">&nbsp;//&nbsp;</span>
    <output id="roleBadge" class="player-status-role" v-bind="roleAttributes">{{ roleLabel }}</output>
  </section>
</template>
