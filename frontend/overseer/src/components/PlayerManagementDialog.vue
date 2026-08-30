<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';

import type { PlayerProfile } from '../composables/usePlayerManagement.js';
import { dialogFocus } from '../directives/dialog-focus.js';
import PlayerProfileRow from './PlayerProfileRow.vue';

const props = defineProps<{
  readonly addResetRequest: number;
  readonly deleteFocusCharacterId: string;
  readonly deleteFocusRequest: number;
  readonly error: string;
  readonly open: boolean;
  readonly pending: boolean;
  readonly profiles: readonly PlayerProfile[];
  readonly readOnly: boolean;
  readonly status: string;
}>();

const emit = defineEmits<{
  add: [name: string, intelligence: number, hackerPerkAvailable: boolean];
  close: [];
  delete: [characterId: string, name: string];
  save: [characterId: string, name: string, intelligence: number, hackerPerkAvailable: boolean];
}>();

const vDialogFocus = dialogFocus;
const closeButton = ref<HTMLButtonElement | null>(null);
const dialog = ref<HTMLDialogElement | null>(null);
const form = ref<HTMLFormElement | null>(null);
const hackerPerkAvailable = ref('');
const intelligence = ref('');
const name = ref('');
const focusBinding = computed(() => ({
  active: props.open,
  initialFocus: () => closeButton.value,
  onCancel: () => emit('close'),
}));
const guardAttributes = Object.freeze({ 'data-stale-result-guard': 'released' });

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (props.open && !element.open) element.showModal();
  else if (!props.open && element.open) element.close();
}

function add(): void {
  const element = form.value;
  if (element === null || !element.checkValidity()) {
    element?.reportValidity();
    return;
  }
  const parsedIntelligence = Number(intelligence.value);
  if (!Number.isInteger(parsedIntelligence) || hackerPerkAvailable.value === '') return;
  emit('add', name.value.trim(), parsedIntelligence, hackerPerkAvailable.value === 'true');
}

function resetAddForm(): void {
  form.value?.reset();
  name.value = '';
  intelligence.value = '';
  hackerPerkAvailable.value = '';
}

function forwardSave(
  characterId: string,
  profileName: string,
  profileIntelligence: number,
  profileHackerPerkAvailable: boolean,
): void {
  emit('save', characterId, profileName, profileIntelligence, profileHackerPerkAvailable);
}

watch(() => props.open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
watch(() => props.addResetRequest, resetAddForm);
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
</script>

<template>
  <dialog
    id="playerManagementDialog"
    ref="dialog"
    v-bind="guardAttributes"
    v-dialog-focus="focusBinding"
    class="player-management-dialog"
    aria-modal="true"
    :aria-readonly="readOnly"
    aria-labelledby="playerManagementDialogTitle"
    aria-describedby="playerManagementMode playerManagementStatus playerManagementError"
    :hidden="!open"
  >
    <div class="player-management-dialog-panel">
      <header class="player-management-dialog-header">
        <div>
          <h2 id="playerManagementDialogTitle" class="player-management-dialog-title">УПРАВЛЕНИЕ ИГРОКАМИ</h2>
          <p id="playerManagementMode" class="player-management-mode" role="status" aria-live="polite" aria-atomic="true">{{ readOnly ? 'ТРАНСЛЯЦИЯ АКТИВНА · ПРОСМОТР БЕЗ РЕДАКТИРОВАНИЯ' : 'РЕДАКТИРОВАНИЕ ДОСТУПНО' }}</p>
        </div>
        <button id="btnClosePlayerManagement" ref="closeButton" class="btn btn-secondary" type="button" @click="emit('close')">ЗАКРЫТЬ</button>
      </header>

      <section class="player-management-roster-section" aria-labelledby="playerManagementRosterHeading">
        <h3 id="playerManagementRosterHeading" class="player-management-section-title">СПИСОК ИГРОКОВ</h3>
        <div id="playerManagementRoster" class="player-management-roster" role="list" aria-label="Подробный список игроков">
          <PlayerProfileRow
            v-for="profile in profiles"
            :key="profile.id"
            :focus-delete-request="deleteFocusCharacterId === profile.id ? deleteFocusRequest : 0"
            :pending="pending"
            :profile="profile"
            :read-only="readOnly"
            @delete="emit('delete', $event, profile.name)"
            @save="forwardSave"
          />
        </div>
        <p id="playerManagementEmpty" class="player-management-empty" :hidden="profiles.length > 0">ИГРОКИ ЕЩЁ НЕ ДОБАВЛЕНЫ</p>
      </section>

      <form id="playerManagementAddForm" ref="form" class="player-management-add-form" aria-labelledby="playerManagementAddHeading" aria-describedby="playerManagementStatus playerManagementError" @submit.prevent="add">
        <h3 id="playerManagementAddHeading" class="player-management-section-title">ДОБАВИТЬ ИГРОКА</h3>
        <div class="player-management-field-grid">
          <label class="field-label-inline" for="playerNameInput">ИМЯ
            <input id="playerNameInput" v-model="name" class="field-input" name="playerName" type="text" maxlength="80" autocomplete="off" required :disabled="pending || readOnly">
          </label>
          <label class="field-label-inline" for="playerIntelligenceInput">ИНТЕЛЛЕКТ
            <input id="playerIntelligenceInput" v-model="intelligence" class="field-input" name="playerIntelligence" type="number" min="1" max="10" step="1" inputmode="numeric" required :disabled="pending || readOnly">
          </label>
          <label class="field-label-inline" for="playerHackerPerkAvailability">ПЕРК «ХАКЕР»
            <select id="playerHackerPerkAvailability" v-model="hackerPerkAvailable" class="field-input" name="playerHackerPerkAvailable" required :disabled="pending || readOnly">
              <option value="">ВЫБЕРИТЕ ЗНАЧЕНИЕ</option>
              <option value="false">НЕДОСТУПЕН</option>
              <option value="true">ДОСТУПЕН</option>
            </select>
          </label>
        </div>
        <button id="btnAddPlayer" class="btn btn-primary" type="submit" :disabled="pending || readOnly">ДОБАВИТЬ ИГРОКА</button>
      </form>

      <div class="player-management-feedback" aria-label="Состояние операций со списком игроков">
        <div id="playerManagementStatus" class="player-management-status" role="status" aria-live="polite" aria-atomic="true">{{ status }}</div>
        <div id="playerManagementError" class="player-management-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
      </div>
    </div>
  </dialog>
</template>
