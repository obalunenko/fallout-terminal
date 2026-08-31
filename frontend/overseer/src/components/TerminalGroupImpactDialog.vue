<script setup lang="ts">
import { inject, nextTick, onBeforeUnmount, onUnmounted, ref, watch } from 'vue';

import { overseerCoexistenceBridgeKey, type OverseerCoexistenceMessage } from '../mount.js';
import type { DesktopCommandResult } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

interface ImpactSummary {
  readonly destinationGroup: string;
  readonly groups: string;
  readonly kind: string;
  readonly membership: string;
  readonly orderAfter: string;
  readonly orderBefore: string;
  readonly sourceGroup: string;
  readonly terminals: string;
}

const emptySummary: ImpactSummary = Object.freeze({
  destinationGroup: '—',
  groups: '—',
  kind: '—',
  membership: '—',
  orderAfter: '—',
  orderBefore: '—',
  sourceGroup: '—',
  terminals: '—',
});

const props = defineProps<{ readonly port: DesktopPort }>();
const bridge = inject(overseerCoexistenceBridgeKey, null);
const amendButton = ref<HTMLButtonElement | null>(null);
const canAmend = ref(false);
const candidate = ref<readonly unknown[]>(Object.freeze([]));
const cancelButton = ref<HTMLButtonElement | null>(null);
const dialog = ref<HTMLDialogElement | null>(null);
const error = ref('');
const expectedCoordinationRevision = ref(0);
const expectedSessionRevision = ref(0);
const open = ref(false);
const pending = ref(false);
const summary = ref<ImpactSummary>(emptySummary);
let active = true;
let lifecycle = 0;
let restoreOnClose = false;

const guardAttributes = Object.freeze({ 'data-stale-result-guard': 'released' });
const action = (value: string) => Object.freeze({ 'data-action': value });
const impact = (value: string) => Object.freeze({ 'data-impact': value, role: 'definition' });

function text(value: unknown): string {
  return typeof value === 'string' && value !== '' ? value : '—';
}

function setOpen(nextOpen: boolean): void {
  if (open.value === nextOpen) return;
  lifecycle += 1;
  open.value = nextOpen;
  if (!nextOpen) pending.value = false;
}

function handleMessage(message: OverseerCoexistenceMessage): void {
  if (message.kind === 'terminal-group-impact-open') {
    if (!Array.isArray(message.candidate)
      || !Number.isSafeInteger(message.expectedSessionRevision)
      || !Number.isSafeInteger(message.expectedCoordinationRevision)) return;
    candidate.value = Object.freeze(structuredClone(message.candidate));
    expectedSessionRevision.value = Number(message.expectedSessionRevision);
    expectedCoordinationRevision.value = Number(message.expectedCoordinationRevision);
    summary.value = Object.freeze({
      destinationGroup: text(message.destinationGroup),
      groups: text(message.groups),
      kind: text(message.changeKind),
      membership: text(message.membership),
      orderAfter: text(message.orderAfter),
      orderBefore: text(message.orderBefore),
      sourceGroup: text(message.sourceGroup),
      terminals: text(message.terminals),
    });
    error.value = '';
    canAmend.value = false;
    pending.value = false;
    setOpen(true);
    return;
  }
  if (message.kind === 'terminal-group-impact-dismiss') {
    restoreOnClose = false;
    setOpen(false);
    return;
  }
  if (message.kind !== 'terminal-group-command-feedback' || message.target !== 'impact') return;
  pending.value = false;
  error.value = typeof message.error === 'string' ? message.error : '';
  canAmend.value = message.canAmend === true;
  if (message.close === true) {
    restoreOnClose = true;
    setOpen(false);
  }
  else if (canAmend.value) void nextTick(() => amendButton.value?.focus());
}

function close(): void {
  if (!open.value || pending.value) return;
  restoreOnClose = true;
  setOpen(false);
}

function amend(): void {
  if (!open.value || pending.value || !canAmend.value) return;
  bridge?.vueToLegacy({ kind: 'terminal-group-impact-amend-requested' });
}

async function confirm(): Promise<void> {
  if (!open.value || pending.value) return;
  const requestLifecycle = lifecycle;
  pending.value = true;
  error.value = '';
  canAmend.value = false;
  let result: DesktopCommandResult;
  try {
    result = await props.port.replaceTerminalGroups({
      terminalGroups: structuredClone(candidate.value),
      expectedSessionRevision: expectedSessionRevision.value,
      expectedCoordinationRevision: expectedCoordinationRevision.value,
    });
  } catch (cause) {
    result = { error: cause instanceof Error ? cause.message : String(cause), ok: false };
  }
  bridge?.vueToLegacy({ kind: 'terminal-group-command-finished', result, source: 'impact' });
  if (!active || requestLifecycle !== lifecycle || !open.value) return;
  pending.value = false;
}

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (open.value && !element.open) {
    element.showModal();
    cancelButton.value?.focus();
  }
  else if (!open.value && element.open) {
    element.close();
    if (restoreOnClose) bridge?.vueToLegacy({ kind: 'terminal-group-impact-closed' });
    restoreOnClose = false;
  }
}

const release = bridge?.subscribeLegacyState(handleMessage) ?? (() => {});
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
    id="terminalGroupImpactDialog"
    ref="dialog"
    v-bind="guardAttributes"
    class="terminal-group-dialog terminal-group-impact-dialog"
    aria-modal="true"
    aria-labelledby="terminalGroupImpactDialogTitle"
    aria-describedby="terminalGroupImpactDialogDescription terminalGroupImpactSummary terminalGroupImpactError"
    :aria-busy="pending"
    :hidden="!open"
    @cancel.prevent="close"
  >
    <div class="terminal-group-dialog-panel">
      <header class="terminal-group-dialog-header">
        <div>
          <h2 id="terminalGroupImpactDialogTitle" class="terminal-group-dialog-title">ПОДТВЕРДИТЕ ИЗМЕНЕНИЕ ГРУПП</h2>
          <p id="terminalGroupImpactDialogDescription" class="terminal-group-dialog-description">Проверьте затронутые группы, терминалы и порядок навигации. Изменение применится целиком только после подтверждения.</p>
        </div>
        <button v-bind="action('close-terminal-group-change')" class="btn btn-secondary" type="button" :disabled="pending" aria-label="Закрыть подтверждение изменения групп" @click="close">ЗАКРЫТЬ</button>
      </header>

      <dl id="terminalGroupImpactSummary" class="terminal-group-impact">
        <div class="terminal-group-impact-row"><dt>ДЕЙСТВИЕ</dt><dd v-bind="impact('kind')">{{ summary.kind }}</dd></div>
        <div class="terminal-group-impact-row"><dt>ЗАТРОНУТЫЕ ГРУППЫ</dt><dd v-bind="impact('groups')">{{ summary.groups }}</dd></div>
        <div class="terminal-group-impact-row"><dt>ЗАТРОНУТЫЕ ТЕРМИНАЛЫ</dt><dd v-bind="impact('terminals')">{{ summary.terminals }}</dd></div>
        <div class="terminal-group-impact-row"><dt>ИСХОДНАЯ ГРУППА</dt><dd v-bind="impact('source-group')">{{ summary.sourceGroup }}</dd></div>
        <div class="terminal-group-impact-row"><dt>ГРУППА НАЗНАЧЕНИЯ</dt><dd v-bind="impact('destination-group')">{{ summary.destinationGroup }}</dd></div>
        <div class="terminal-group-impact-row"><dt>ПОЛНЫЙ ИТОГОВЫЙ СОСТАВ</dt><dd v-bind="impact('membership')">{{ summary.membership }}</dd></div>
        <div class="terminal-group-impact-row"><dt>ПОРЯДОК ДО</dt><dd v-bind="impact('order-before')">{{ summary.orderBefore }}</dd></div>
        <div class="terminal-group-impact-row"><dt>ПОРЯДОК ПОСЛЕ</dt><dd v-bind="impact('order-after')">{{ summary.orderAfter }}</dd></div>
      </dl>

      <div id="terminalGroupImpactError" class="terminal-group-impact-rejection" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
      <div class="terminal-group-impact-actions" role="group" aria-label="Подтверждение разрушительного изменения групп">
        <button ref="cancelButton" v-bind="action('cancel-terminal-group-change')" class="btn btn-secondary" type="button" :disabled="pending" @click="close">ОТМЕНА</button>
        <button ref="amendButton" v-bind="action('amend-terminal-group-change')" class="btn btn-primary" type="button" :disabled="pending" :hidden="!canAmend" @click="amend">ДОБАВИТЬ СВЯЗАННЫЕ ТЕРМИНАЛЫ</button>
        <button v-bind="action('confirm-terminal-group-change')" class="btn btn-danger" type="button" :disabled="pending" @click="confirm">ПРИМЕНИТЬ ИЗМЕНЕНИЕ</button>
      </div>
    </div>
  </dialog>
</template>
