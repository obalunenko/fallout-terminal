import { computed, inject, onUnmounted, readonly, ref, shallowRef } from 'vue';

import { overseerCoexistenceBridgeKey, type OverseerCoexistenceMessage } from '../mount.js';
import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

export interface HackSnapshot {
  readonly attemptsLeft: number;
  readonly attemptsMax: number;
  readonly failed: boolean;
  readonly revision: number;
  readonly solved: boolean;
}

interface HackContext {
  readonly coordinationRevision: number;
  readonly hackLevel: number;
  readonly introText: string;
  readonly terminalID: string;
  readonly terminalName: string;
  readonly tree: DesktopRecord;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseContext(message: OverseerCoexistenceMessage): HackContext | null | undefined {
  if (message.kind !== 'terminal-authoring-snapshot') return undefined;
  if (message.hackContext === null) return null;
  if (!isRecord(message.hackContext)) return undefined;
  const value = message.hackContext;
  const coordinationRevision = Number(value.coordinationRevision);
  const hackLevel = Number(value.hackLevel);
  if (!Number.isSafeInteger(coordinationRevision) || coordinationRevision < 0
    || !Number.isSafeInteger(hackLevel) || hackLevel < 1 || hackLevel > 5
    || typeof value.terminalID !== 'string' || value.terminalID === ''
    || typeof value.terminalName !== 'string' || !isRecord(value.tree)
    || typeof value.introText !== 'string') return undefined;
  return Object.freeze({
    coordinationRevision,
    hackLevel,
    introText: value.introText,
    terminalID: value.terminalID,
    terminalName: value.terminalName,
    tree: value.tree,
  });
}

function parseHack(value: DesktopRecord, revision: number): HackSnapshot | null {
  const attemptsLeft = Number(value.attemptsLeft);
  const attemptsMax = Number(value.attemptsMax);
  if (!Number.isSafeInteger(attemptsLeft) || !Number.isSafeInteger(attemptsMax)
    || attemptsMax < 1 || attemptsLeft < 0 || attemptsLeft > attemptsMax
    || typeof value.solved !== 'boolean' || typeof value.failed !== 'boolean'
    || (value.solved && value.failed)) return null;
  return Object.freeze({
    attemptsLeft,
    attemptsMax,
    failed: value.failed,
    revision,
    solved: value.solved,
  });
}

export function useHackControls(port: DesktopPort) {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const context = shallowRef<HackContext | null>(null);
  const current = shallowRef<HackSnapshot | null>(null);
  const commandPending = ref(false);
  const coordinationPending = ref(false);
  const error = ref('');
  let active = true;
  let commandGeneration = 0;
  let receiptRevision = 0;

  function replaceContext(next: HackContext | null): void {
    if (next?.terminalID === context.value?.terminalID) {
      context.value = next;
      return;
    }
    context.value = next;
    current.value = null;
    error.value = '';
    commandPending.value = false;
    commandGeneration += 1;
  }

  function handleLegacyMessage(message: OverseerCoexistenceMessage): void {
    const nextContext = parseContext(message);
    if (nextContext !== undefined) {
      replaceContext(nextContext);
      return;
    }
    if (message.kind !== 'coordination-state') return;
    coordinationPending.value = message.pending === true;
  }

  function handleHackState(value: DesktopRecord | null): void {
    if (value === null) return;
    const explicitRevision = Number(value.revision);
    const hasExplicitRevision = Number.isSafeInteger(explicitRevision) && explicitRevision >= 0;
    const revision = hasExplicitRevision ? explicitRevision : receiptRevision + 1;
    if (revision <= receiptRevision) return;
    const next = parseHack(value, revision);
    if (next === null) return;
    receiptRevision = revision;
    current.value = next;
  }

  async function run(
    invoke: (value: HackContext) => Promise<DesktopCommandResult>,
    coordinationCommand: boolean,
    pendingMessage: string,
    successMessage: string,
  ): Promise<void> {
    const actionContext = context.value;
    const hack = current.value;
    if (!active || actionContext === null || hack === null || commandPending.value
      || coordinationPending.value) return;
    const generation = ++commandGeneration;
    commandPending.value = true;
    error.value = '';
    if (coordinationCommand) {
      bridge?.vueToLegacy({
        expectedRevision: actionContext.coordinationRevision,
        kind: 'hack-command-started',
        status: pendingMessage,
      });
    }
    let result: DesktopCommandResult;
    try {
      result = await invoke(actionContext);
    } catch (cause) {
      result = { error: cause instanceof Error ? cause.message : String(cause), ok: false };
    }
    if (coordinationCommand) {
      bridge?.vueToLegacy({
        expectedRevision: actionContext.coordinationRevision,
        kind: 'hack-command-finished',
        result,
        successMessage,
      });
    }
    if (!active || generation !== commandGeneration) return;
    commandPending.value = false;
    if (context.value?.terminalID !== actionContext.terminalID
      || context.value.coordinationRevision !== actionContext.coordinationRevision
      || current.value?.revision !== hack.revision) return;
    error.value = result.ok ? '' : (result.error || 'ОПЕРАЦИЯ ВЗЛОМА ОТКЛОНЕНА');
  }

  function forceSuccess(): void {
    if (current.value?.solved || current.value?.failed) return;
    void run(() => port.forceHackSuccess(), false, '', '');
  }

  function reset(): void {
    if (!current.value?.failed) return;
    void run(value => port.resetFailedHack({
      hackLevel: value.hackLevel,
      introText: value.introText,
      terminalId: value.terminalID,
      terminalName: value.terminalName,
      tree: value.tree,
    }), true, 'ПОДГОТОВКА НОВОЙ ГОЛОВОЛОМКИ...', 'СОЗДАНА НОВАЯ ГОЛОВОЛОМКА');
  }

  const releaseBridge = bridge?.subscribeLegacyState(handleLegacyMessage) ?? (() => {});
  const releaseHackState = port.onHackState(handleHackState);
  onUnmounted(() => {
    active = false;
    commandGeneration += 1;
    releaseBridge();
    releaseHackState();
    context.value = null;
    current.value = null;
    commandPending.value = false;
    coordinationPending.value = false;
    error.value = '';
  });

  return {
    current: readonly(current),
    error: readonly(error),
    forceSuccess,
    pending: computed(() => commandPending.value || coordinationPending.value),
    reset,
    visible: computed(() => context.value !== null && current.value !== null),
  };
}
