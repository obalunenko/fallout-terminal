<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onUnmounted, ref, watch } from 'vue';

import { dialogFocus } from '../directives/dialog-focus.js';
import { overseerControllerKey, type OverseerControllerMessage } from '../controllers/overseer-controller.js';
import type { DesktopCommandResult } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const props = defineProps<{ readonly port: DesktopPort }>();
const controller = inject(overseerControllerKey, null);
const vDialogFocus = dialogFocus;
const cancelButton = ref<HTMLButtonElement | null>(null);
const characterId = ref('');
const dialog = ref<HTMLDialogElement | null>(null);
const error = ref('');
const expectedRevision = ref(0);
const name = ref('');
const open = ref(false);
const pending = ref(false);
let active = true;
let lifecycle = 0;
const focusBinding = computed(() => ({ active: open.value, initialFocus: () => cancelButton.value, onCancel: cancel }));
const guardAttributes = Object.freeze({ 'data-stale-result-guard': 'released' });

function handleMessage(message: OverseerControllerMessage): void {
  if (message.kind !== 'player-delete-requested' || typeof message.characterId !== 'string'
    || typeof message.name !== 'string' || !Number.isSafeInteger(message.expectedRevision)) return;
  lifecycle += 1;
  characterId.value = message.characterId;
  name.value = message.name;
  expectedRevision.value = Number(message.expectedRevision);
  error.value = '';
  pending.value = false;
  open.value = true;
}

function finish(restoreFocus: boolean): void {
  const deletedCharacterId = characterId.value;
  lifecycle += 1;
  open.value = false;
  pending.value = false;
  if (restoreFocus && deletedCharacterId !== '') {
    controller?.dispatch({ characterId: deletedCharacterId, kind: 'player-delete-focus-request' });
  }
}

function cancel(): void {
  if (!pending.value) finish(true);
}

async function confirm(): Promise<void> {
  if (pending.value || characterId.value === '') return;
  const requestLifecycle = lifecycle;
  const requestCharacterId = characterId.value;
  pending.value = true;
  error.value = '';
  controller?.dispatch({ expectedRevision: expectedRevision.value, kind: 'player-delete-started' });
  let result: DesktopCommandResult;
  try {
    result = await props.port.deleteCharacter({ characterId: requestCharacterId, expectedRevision: expectedRevision.value });
  } catch (cause) {
    result = { error: cause instanceof Error ? cause.message : String(cause), ok: false };
  }
  controller?.dispatch({ kind: 'player-delete-finished', result });
  if (!active || requestLifecycle !== lifecycle || requestCharacterId !== characterId.value) return;
  pending.value = false;
  if (result.ok) finish(false);
  else error.value = result.error || 'НЕ УДАЛОСЬ УДАЛИТЬ ИГРОКА';
}

async function syncDialog(): Promise<void> {
  await nextTick();
  if (open.value && dialog.value?.open !== true) dialog.value?.showModal();
  else if (!open.value && dialog.value?.open === true) dialog.value.close();
}

const release = controller?.subscribeState(handleMessage) ?? (() => {});
watch(open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
onUnmounted(() => { active = false; lifecycle += 1; release(); });
</script>

<template>
  <dialog id="playerDeleteDialog" ref="dialog" v-bind="guardAttributes" v-dialog-focus="focusBinding" class="terminal-switch-dialog player-delete-dialog" aria-modal="true" aria-labelledby="playerDeleteDialogTitle" aria-describedby="playerDeleteDialogDescription playerDeleteError" :hidden="!open">
    <div class="terminal-switch-dialog-panel">
      <h2 id="playerDeleteDialogTitle" class="terminal-switch-dialog-title">УДАЛИТЬ ИГРОКА?</h2>
      <p id="playerDeleteDialogDescription" class="terminal-switch-dialog-description">Удалить игрока «{{ name }}»? Это действие нельзя отменить.</p>
      <div id="playerDeleteError" class="terminal-switch-error" role="alert" :hidden="error === ''">{{ error }}</div>
      <div class="player-delete-actions" role="group" aria-label="Подтверждение удаления игрока">
        <button id="btnConfirmPlayerDelete" class="btn btn-danger" type="button" :disabled="pending" @click="confirm">УДАЛИТЬ ИГРОКА</button>
        <button id="btnCancelPlayerDelete" ref="cancelButton" class="btn btn-secondary" type="button" :disabled="pending" @click="cancel">ОТМЕНА</button>
      </div>
    </div>
  </dialog>
</template>
