<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';

import type {
  LogicalRosterCharacter,
  LogicalSession,
} from '../composables/useLogicalSessions.js';
import { dialogFocus } from '../directives/dialog-focus.js';
import LogicalSessionRow from './LogicalSessionRow.vue';

const props = defineProps<{
  readonly broadcastActive: boolean;
  readonly error: string;
  readonly open: boolean;
  readonly pending: boolean;
  readonly playerConfigActive: boolean;
  readonly roster: readonly LogicalRosterCharacter[];
  readonly sessions: readonly LogicalSession[];
  readonly status: string;
}>();

const emit = defineEmits<{
  assign: [sessionId: string, characterId: string];
  close: [];
  controller: [sessionId: string];
  move: [sessionId: string, destinationId: string];
  release: [sessionId: string];
  rename: [sessionId: string, fallbackName: string];
}>();

const vDialogFocus = dialogFocus;
const dialog = ref<HTMLDialogElement | null>(null);
const closeButton = ref<HTMLButtonElement | null>(null);
const focusBinding = computed(() => ({
  active: props.open,
  initialFocus: () => closeButton.value,
  onCancel: () => emit('close'),
}));

function forwardAssign(sessionId: string, characterId: string): void {
  emit('assign', sessionId, characterId);
}

function forwardMove(sessionId: string, destinationId: string): void {
  emit('move', sessionId, destinationId);
}

function forwardRename(sessionId: string, fallbackName: string): void {
  emit('rename', sessionId, fallbackName);
}

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (props.open && !element.open) element.showModal();
  else if (!props.open && element.open) element.close();
}

watch(() => props.open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
</script>

<template>
  <dialog
    id="logicalSessionDialog"
    ref="dialog"
    v-dialog-focus="focusBinding"
    class="logical-session-dialog"
    aria-modal="true"
    aria-labelledby="logicalSessionDialogTitle"
    aria-describedby="logicalSessionDialogStatus logicalSessionDialogError"
    :hidden="!open"
  >
    <div class="logical-session-dialog-panel">
      <header class="logical-session-dialog-header">
        <div>
          <h2 id="logicalSessionDialogTitle" class="logical-session-dialog-title">ЛОГИЧЕСКИЕ СЕССИИ</h2>
          <p class="logical-session-dialog-description">ПОДРОБНАЯ ИНФОРМАЦИЯ И УПРАВЛЕНИЕ РАСПОЗНАННЫМИ СЕССИЯМИ</p>
        </div>
        <button id="btnCloseLogicalSessions" ref="closeButton" class="btn btn-secondary" type="button" @click="emit('close')">ЗАКРЫТЬ</button>
      </header>

      <div id="logicalSessionList" class="session-list logical-session-dialog-list" role="list" aria-label="Распознанные логические сессии">
        <div v-if="sessions.length === 0" class="session-empty" role="listitem">СЕССИИ НЕ ПОДКЛЮЧЕНЫ</div>
        <LogicalSessionRow
          v-for="session in sessions"
          :key="session.id"
          :broadcast-active="broadcastActive"
          :pending="pending"
          :player-config-active="playerConfigActive"
          :roster="roster"
          :session="session"
          :sessions="sessions"
          @assign="forwardAssign"
          @controller="emit('controller', $event)"
          @move="forwardMove"
          @release="emit('release', $event)"
          @rename="forwardRename"
        />
      </div>

      <div class="logical-session-dialog-feedback" aria-label="Состояние операций с логическими сессиями">
        <div id="logicalSessionDialogStatus" class="logical-session-dialog-status" role="status" aria-live="polite" aria-atomic="true">{{ status }}</div>
        <div id="logicalSessionDialogError" class="logical-session-dialog-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
      </div>
    </div>
  </dialog>
</template>
