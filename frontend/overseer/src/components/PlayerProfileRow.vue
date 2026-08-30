<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';

import type { PlayerProfile } from '../composables/usePlayerManagement.js';

const props = defineProps<{
  readonly focusDeleteRequest: number;
  readonly pending: boolean;
  readonly profile: PlayerProfile;
  readonly readOnly: boolean;
}>();

const emit = defineEmits<{
  delete: [characterId: string, name: string];
  save: [characterId: string, name: string, intelligence: number, hackerPerkAvailable: boolean];
}>();

const deleteButton = ref<HTMLButtonElement | null>(null);
const hackerPerkAvailable = ref(String(props.profile.hackerPerkAvailable));
const intelligence = ref(String(props.profile.intelligence));
const name = ref(props.profile.name);
const rowAttributes = computed(() => ({ 'data-character-id': props.profile.id }));

watch(() => props.profile, (profile) => {
  name.value = profile.name;
  intelligence.value = String(profile.intelligence);
  hackerPerkAvailable.value = String(profile.hackerPerkAvailable);
});

watch(() => props.focusDeleteRequest, async (request) => {
  if (request === 0) return;
  await nextTick();
  deleteButton.value?.focus();
});

function save(): void {
  const trimmedName = name.value.trim();
  const parsedIntelligence = Number(intelligence.value);
  if (trimmedName === '' || !Number.isInteger(parsedIntelligence)
    || parsedIntelligence < 1 || parsedIntelligence > 10
    || (hackerPerkAvailable.value !== 'true' && hackerPerkAvailable.value !== 'false')) return;
  emit('save', props.profile.id, trimmedName, parsedIntelligence, hackerPerkAvailable.value === 'true');
}
</script>

<template>
  <article v-bind="rowAttributes" class="player-management-row" role="listitem">
    <div class="player-management-row-fields">
      <label class="field-label-inline">
        ИМЯ
        <input v-model="name" class="field-input player-name-input" name="playerName" type="text" maxlength="80" autocomplete="off" required :disabled="pending || readOnly">
      </label>
      <label class="field-label-inline">
        ИНТЕЛЛЕКТ
        <input v-model="intelligence" class="field-input player-intelligence-input" name="playerIntelligence" type="number" min="1" max="10" step="1" inputmode="numeric" required :disabled="pending || readOnly">
      </label>
      <label class="field-label-inline">
        ПЕРК «ХАКЕР»
        <select v-model="hackerPerkAvailable" class="field-input player-hacker-perk-availability" name="playerHackerPerkAvailable" required :disabled="pending || readOnly">
          <option value="">ВЫБЕРИТЕ ЗНАЧЕНИЕ</option>
          <option value="false">НЕДОСТУПЕН</option>
          <option value="true">ДОСТУПЕН</option>
        </select>
      </label>
    </div>
    <div class="player-management-row-actions" role="group" aria-label="Изменение игрока">
      <button class="btn btn-primary player-save" type="button" :disabled="pending || readOnly" @click="save">СОХРАНИТЬ</button>
      <button ref="deleteButton" class="btn btn-danger player-delete" type="button" :disabled="pending || readOnly" @click="emit('delete', profile.id, profile.name)">УДАЛИТЬ</button>
    </div>
  </article>
</template>
