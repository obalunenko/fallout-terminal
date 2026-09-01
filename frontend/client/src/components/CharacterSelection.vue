<script setup lang="ts">
import type { PlayerRosterIdentity } from '../composables/usePlayerIdentity.js';
import CharacterOption from './CharacterOption.vue';

defineProps<{
  readonly pending?: boolean;
  readonly roster: readonly PlayerRosterIdentity[];
}>();

const emit = defineEmits<{
  select: [characterID: string];
}>();
</script>

<template>
  <section
    id="characterSelect"
    class="character-select"
    :class="{ pending: pending === true }"
    aria-labelledby="characterSelectTitle"
    aria-describedby="playerNotice"
    :aria-busy="pending === true"
  >
    <h1 id="characterSelectTitle" class="character-select-title">ВЫБЕРИТЕ ПЕРСОНАЖА</h1>
    <div id="characterOptions" class="character-options" role="list">
      <CharacterOption
        v-for="entry in roster"
        :key="entry.id"
        :entry="entry"
        :pending="pending"
        @select="emit('select', $event)"
      />
    </div>
  </section>
</template>
