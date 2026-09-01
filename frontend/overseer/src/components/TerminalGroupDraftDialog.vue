<script setup lang="ts">
import { inject, nextTick, onBeforeUnmount, onUnmounted, ref, watch } from 'vue';

import { overseerControllerKey, type OverseerControllerMessage } from '../controllers/overseer-controller.js';
import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

interface TerminalChoice {
  readonly id: string;
  readonly name: string;
}

interface DestinationChoice extends TerminalChoice {
  readonly newSingleton: boolean;
}

type DraftMode = 'create' | 'move' | 'rename';

const props = defineProps<{ readonly port: DesktopPort }>();
const controller = inject(overseerControllerKey, null);
const closeButton = ref<HTMLButtonElement | null>(null);
const destinationGroupId = ref('');
const destinationSelect = ref<HTMLSelectElement | null>(null);
const destinations = ref<readonly DestinationChoice[]>(Object.freeze([]));
const dialog = ref<HTMLDialogElement | null>(null);
const error = ref('');
const mode = ref<DraftMode>('create');
const name = ref('');
const nameInput = ref<HTMLInputElement | null>(null);
const open = ref(false);
const pending = ref(false);
const selectedTerminalIds = ref<readonly string[]>(Object.freeze([]));
const terminals = ref<readonly TerminalChoice[]>(Object.freeze([]));
let active = true;
let lifecycle = 0;
let restoreOnClose = false;

const guardAttributes = Object.freeze({ 'data-stale-result-guard': 'released' });
const action = (value: string) => Object.freeze({ 'data-action': value });
const singletonMarker = (enabled: boolean) => enabled
  ? Object.freeze({ 'data-new-singleton': 'true' })
  : Object.freeze({});

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function choice(value: unknown): TerminalChoice | null {
  if (!isRecord(value) || typeof value.id !== 'string' || value.id === ''
    || typeof value.name !== 'string') return null;
  return Object.freeze({ id: value.id, name: value.name });
}

function choices(value: unknown): readonly TerminalChoice[] {
  if (!Array.isArray(value)) return Object.freeze([]);
  const parsed: TerminalChoice[] = [];
  const ids = new Set<string>();
  for (const item of value) {
    const candidate = choice(item);
    if (candidate === null || ids.has(candidate.id)) continue;
    ids.add(candidate.id);
    parsed.push(candidate);
  }
  return Object.freeze(parsed);
}

function destinationChoices(value: unknown): readonly DestinationChoice[] {
  if (!Array.isArray(value)) return Object.freeze([]);
  const parsed: DestinationChoice[] = [];
  const ids = new Set<string>();
  for (const item of value) {
    const candidate = choice(item);
    if (candidate === null || ids.has(candidate.id) || !isRecord(item)) continue;
    ids.add(candidate.id);
    parsed.push(Object.freeze({ ...candidate, newSingleton: item.newSingleton === true }));
  }
  return Object.freeze(parsed);
}

function stringList(value: unknown): readonly string[] {
  if (!Array.isArray(value)) return Object.freeze([]);
  return Object.freeze([...new Set(value.filter(item => typeof item === 'string'))]);
}

function setOpen(nextOpen: boolean): void {
  if (open.value === nextOpen) return;
  lifecycle += 1;
  open.value = nextOpen;
  if (!nextOpen) pending.value = false;
}

async function submitCandidate(message: OverseerControllerMessage): Promise<void> {
  if (!open.value || pending.value || !Array.isArray(message.candidate)
    || !Number.isSafeInteger(message.expectedSessionRevision)
    || !Number.isSafeInteger(message.expectedCoordinationRevision)) return;
  const requestLifecycle = lifecycle;
  pending.value = true;
  error.value = '';
  let result: DesktopCommandResult;
  try {
    result = await props.port.replaceTerminalGroups({
      terminalGroups: structuredClone(message.candidate),
      expectedSessionRevision: Number(message.expectedSessionRevision),
      expectedCoordinationRevision: Number(message.expectedCoordinationRevision),
    });
  } catch (cause) {
    result = { error: cause instanceof Error ? cause.message : String(cause), ok: false };
  }
  controller?.dispatch({ kind: 'terminal-group-command-finished', result, source: 'draft' });
  if (!active || requestLifecycle !== lifecycle || !open.value) return;
  pending.value = false;
}

function handleMessage(message: OverseerControllerMessage): void {
  if (message.kind === 'terminal-group-draft-open') {
    const nextMode = message.mode;
    if (nextMode !== 'create' && nextMode !== 'move' && nextMode !== 'rename') return;
    mode.value = nextMode;
    name.value = typeof message.name === 'string' ? message.name : '';
    terminals.value = choices(message.terminals);
    selectedTerminalIds.value = stringList(message.selectedTerminalIds);
    destinations.value = destinationChoices(message.destinations);
    destinationGroupId.value = '';
    error.value = '';
    pending.value = false;
    setOpen(true);
    return;
  }
  if (message.kind === 'terminal-group-draft-dismiss') {
    restoreOnClose = false;
    setOpen(false);
    return;
  }
  if (message.kind === 'terminal-group-draft-submit') {
    void submitCandidate(message);
    return;
  }
  if (message.kind !== 'terminal-group-command-feedback' || message.target !== 'draft') return;
  pending.value = false;
  error.value = typeof message.error === 'string' ? message.error : '';
  if (message.close === true) {
    restoreOnClose = true;
    setOpen(false);
  }
}

function close(): void {
  if (!open.value || pending.value) return;
  restoreOnClose = true;
  setOpen(false);
}

function review(): void {
  if (pending.value) return;
  controller?.dispatch({
    destinationGroupId: destinationGroupId.value,
    kind: 'terminal-group-draft-reviewed',
    name: name.value,
    selectedTerminalIds: [...selectedTerminalIds.value],
  });
}

function rename(): void {
  if (pending.value) return;
  controller?.dispatch({ kind: 'terminal-group-rename-requested', name: name.value });
}

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (open.value && !element.open) {
    element.showModal();
    if (mode.value === 'move') destinationSelect.value?.focus();
    else nameInput.value?.focus();
  }
  else if (!open.value && element.open) {
    element.close();
    if (restoreOnClose) controller?.dispatch({ kind: 'terminal-group-draft-closed' });
    restoreOnClose = false;
  }
}

const release = controller?.subscribeState(handleMessage) ?? (() => {});
watch(open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
onUnmounted(() => {
  active = false;
  lifecycle += 1;
  release();
});
</script>

<template>
  <dialog
    id="terminalGroupDraftDialog"
    ref="dialog"
    v-bind="guardAttributes"
    class="terminal-group-dialog terminal-group-draft-dialog"
    aria-modal="true"
    aria-labelledby="terminalGroupDraftDialogTitle"
    aria-describedby="terminalGroupDraftDialogDescription terminalGroupDraftError"
    :aria-busy="pending"
    :hidden="!open"
    @cancel.prevent="close"
  >
    <form id="terminalGroupDraftForm" class="terminal-group-dialog-panel" novalidate @submit.prevent>
      <header class="terminal-group-dialog-header">
        <div>
          <h2 id="terminalGroupDraftDialogTitle" class="terminal-group-dialog-title">ИЗМЕНЕНИЕ ГРУППЫ ТЕРМИНАЛОВ</h2>
          <p id="terminalGroupDraftDialogDescription" class="terminal-group-dialog-description">Подготовьте полный состав или новое положение терминала. Разрушительное изменение будет показано отдельно до сохранения.</p>
        </div>
        <button ref="closeButton" v-bind="action('close-terminal-group-draft')" class="btn btn-secondary" type="button" :disabled="pending" aria-label="Закрыть изменение группы" @click="close">ЗАКРЫТЬ</button>
      </header>

      <label class="field-label-inline" for="terminalGroupNameInput" :hidden="mode === 'move'">НАЗВАНИЕ ГРУППЫ</label>
      <input id="terminalGroupNameInput" ref="nameInput" v-model="name" class="field-input" name="groupName" type="text" maxlength="256" autocomplete="off" required aria-describedby="terminalGroupDraftError" :disabled="pending" :hidden="mode === 'move'">

      <fieldset id="terminalGroupTerminalChoices" class="terminal-group-terminal-choices" :disabled="pending" :hidden="mode !== 'create'">
        <legend>ТЕРМИНАЛЫ ГРУППЫ</legend>
        <div class="terminal-group-terminal-choice-list" role="group" aria-label="Выбор терминалов">
          <label v-for="terminal in terminals" :key="terminal.id" class="terminal-group-terminal-choice">
            <input v-model="selectedTerminalIds" name="terminalIds" type="checkbox" :value="terminal.id">
            <span class="terminal-group-terminal-choice-name">{{ terminal.name }}</span>
          </label>
        </div>
      </fieldset>

      <label class="field-label-inline" for="terminalGroupDestinationSelect" :hidden="mode !== 'move'">ГРУППА НАЗНАЧЕНИЯ</label>
      <select id="terminalGroupDestinationSelect" ref="destinationSelect" v-model="destinationGroupId" class="field-input" name="destinationGroupId" :disabled="pending" :hidden="mode !== 'move'">
        <option value="">ВЫБЕРИТЕ ГРУППУ</option>
        <option v-for="destination in destinations" :key="destination.id" v-bind="singletonMarker(destination.newSingleton)" :value="destination.id">{{ destination.name }}</option>
      </select>

      <div id="terminalGroupDraftError" class="terminal-group-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
      <div class="terminal-group-draft-actions" role="group" aria-label="Подготовка изменения группы">
        <button v-bind="action('cancel-terminal-group-draft')" class="btn btn-secondary" type="button" :disabled="pending" @click="close">ОТМЕНА</button>
        <button v-bind="action('review-terminal-group-change')" class="btn btn-primary" type="button" :disabled="pending" :hidden="mode === 'rename'" @click="review">ПРОСМОТРЕТЬ ИЗМЕНЕНИЯ</button>
        <button v-bind="action('save-terminal-group-rename')" class="btn btn-primary" type="button" :disabled="pending" :hidden="mode !== 'rename'" @click="rename">СОХРАНИТЬ НАЗВАНИЕ</button>
      </div>
    </form>
  </dialog>
</template>
