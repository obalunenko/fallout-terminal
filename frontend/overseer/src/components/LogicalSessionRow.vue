<script setup lang="ts">
import { computed, ref, watch } from 'vue';

import type {
  LogicalRosterCharacter,
  LogicalSession,
} from '../composables/useLogicalSessions.js';

const props = defineProps<{
  readonly broadcastActive: boolean;
  readonly pending: boolean;
  readonly playerConfigActive: boolean;
  readonly roster: readonly LogicalRosterCharacter[];
  readonly session: LogicalSession;
  readonly sessions: readonly LogicalSession[];
}>();

const emit = defineEmits<{
  assign: [sessionId: string, characterId: string];
  controller: [sessionId: string];
  move: [sessionId: string, destinationId: string];
  release: [sessionId: string];
  rename: [sessionId: string, fallbackName: string];
}>();

const fallbackName = ref(props.session.fallbackName);
const selectedCharacter = ref('');
const selectedDestination = ref('');
const assigned = computed(() => props.session.character !== null);
const availableCharacters = computed(() => props.roster.filter(character => character.claimedBySessionId === ''));
const destinationSessions = computed(() => props.sessions.filter(session => session.character === null));
const role = computed(() => props.session.role || 'unassigned');
const roleLabel = computed(() => {
  if (role.value === 'active') {
    return props.session.connected ? 'УПРАВЛЯЮЩИЙ' : 'УПРАВЛЯЮЩИЙ · НЕТ СВЯЗИ';
  }
  return role.value === 'observer' ? 'НАБЛЮДАТЕЛЬ' : 'БЕЗ РОЛИ';
});
const rowAttributes = computed(() => ({
  'data-connected': String(props.session.connected),
  'data-role': role.value,
  'data-session-id': props.session.id,
}));
const presenceAttributes = computed(() => ({
  'data-presence': props.session.connected ? 'connected' : 'disconnected',
}));
const roleAttributes = computed(() => ({
  'data-session-role': role.value,
}));
const assignedAttributes = { 'data-assigned-only': '' };
const unassignedAttributes = { 'data-unassigned-only': '' };
const renameAttributes = { 'data-action': 'rename-logical-session' };
const assignAttributes = { 'data-action': 'assign-character' };
const releaseAttributes = { 'data-action': 'release-character' };
const controllerAttributes = { 'data-action': 'set-active-controller' };
const moveAttributes = { 'data-action': 'move-character' };

watch(availableCharacters, characters => {
  if (!characters.some(character => character.id === selectedCharacter.value)) {
    selectedCharacter.value = characters[0]?.id ?? '';
  }
}, { immediate: true });
watch(destinationSessions, sessions => {
  if (!sessions.some(session => session.id === selectedDestination.value)) {
    selectedDestination.value = sessions[0]?.id ?? '';
  }
}, { immediate: true });
watch(() => props.session.fallbackName, value => { fallbackName.value = value; });
</script>

<template>
  <article v-bind="rowAttributes" class="session-row logical-session-row" role="listitem">
    <header class="session-summary">
      <span class="session-primary-name">{{ session.character?.name || session.fallbackName }}</span>
      <span v-bind="presenceAttributes" class="session-presence">{{ session.connected ? 'ПОДКЛЮЧЕН' : 'ОТКЛЮЧЕН' }}</span>
      <span v-bind="roleAttributes" class="session-role">{{ roleLabel }}</span>
    </header>
    <div class="session-identity">
      <span class="session-character-name">{{ session.character ? `ПЕРСОНАЖ: ${session.character.name}` : 'ПЕРСОНАЖ НЕ НАЗНАЧЕН' }}</span>
      <span class="session-fallback-label">СЕССИЯ: {{ session.fallbackName }}</span>
    </div>
    <div class="session-controls" role="group" aria-label="Исправление логической сессии и назначения">
      <label class="field-label-inline session-name-field">
        МЕТКА СЕССИИ
        <input v-model="fallbackName" class="field-input session-name-input" name="sessionFallbackName" type="text" maxlength="80" autocomplete="off" :disabled="pending">
      </label>
      <button v-bind="renameAttributes" class="btn btn-mini session-rename" type="button" :disabled="pending" @click="emit('rename', session.id, fallbackName)">ПЕРЕИМЕНОВАТЬ СЕССИЮ</button>
      <div v-bind="unassignedAttributes" class="session-assignment-controls" :hidden="assigned || !broadcastActive">
        <label class="field-label-inline">
          НАЗНАЧИТЬ ПЕРСОНАЖА
          <select v-model="selectedCharacter" class="field-input session-character-select" name="assignCharacter" :disabled="pending">
            <option v-if="availableCharacters.length === 0" value="">НЕТ ДОСТУПНЫХ ПЕРСОНАЖЕЙ</option>
            <option v-for="character in availableCharacters" :key="character.id" :value="character.id">{{ character.name }}</option>
          </select>
        </label>
        <button v-bind="assignAttributes" class="btn btn-mini session-assign" type="button" :disabled="pending || selectedCharacter === ''" @click="emit('assign', session.id, selectedCharacter)">НАЗНАЧИТЬ</button>
      </div>
      <div v-bind="assignedAttributes" class="session-claimed-controls" :hidden="!assigned">
        <button v-bind="releaseAttributes" class="btn btn-mini session-release" type="button" :disabled="pending || !assigned" @click="emit('release', session.id)">ОСВОБОДИТЬ ПЕРСОНАЖА</button>
        <button v-bind="controllerAttributes" class="btn btn-mini session-controller" type="button" :hidden="role === 'active'" :disabled="pending || !assigned || !session.connected" @click="emit('controller', session.id)">СДЕЛАТЬ УПРАВЛЯЮЩИМ</button>
        <div class="session-move-controls">
          <label class="field-label-inline">
            ПЕРЕМЕСТИТЬ В СЕССИЮ
            <select v-model="selectedDestination" class="field-input session-move-session-select" name="moveCharacterToSession" :disabled="pending || !assigned || !playerConfigActive">
              <option v-if="destinationSessions.length === 0" value="">НЕТ СВОБОДНЫХ СЕССИЙ</option>
              <option v-for="destination in destinationSessions" :key="destination.id" :value="destination.id">{{ destination.character?.name || destination.fallbackName }}</option>
            </select>
          </label>
          <button v-bind="moveAttributes" class="btn btn-mini session-move" type="button" :disabled="pending || !assigned || !playerConfigActive || selectedDestination === ''" @click="emit('move', session.id, selectedDestination)">ПЕРЕМЕСТИТЬ</button>
        </div>
      </div>
    </div>
  </article>
</template>
