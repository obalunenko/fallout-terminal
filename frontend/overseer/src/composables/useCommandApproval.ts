import { computed, onUnmounted, readonly, ref, shallowRef } from 'vue';

import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const MAX_RESOLVED_REQUESTS = 128;

export interface CommandApprovalRequest {
  readonly commandId: string;
  readonly commandName: string;
  readonly confirmationText: string;
  readonly mode: string;
  readonly requestId: string;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function record(value: unknown): DesktopRecord | null {
  return isRecord(value) ? value : null;
}

function text(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

function revision(value: DesktopRecord | null): number {
  const candidate = value?.revision;
  return Number.isSafeInteger(candidate) && Number(candidate) >= 0 ? Number(candidate) : 0;
}

function requestFrom(value: DesktopRecord | null): CommandApprovalRequest | null {
  const pending = record(value?.pendingCommandExecution);
  const requestId = text(pending?.requestId);
  if (requestId === '') return null;
  return Object.freeze({
    commandId: text(pending?.commandId),
    commandName: text(pending?.commandName),
    confirmationText: text(pending?.confirmationText, 'Выполнить команду?'),
    mode: text(pending?.mode),
    requestId,
  });
}

function remember(resolved: Set<string>, requestId: string): void {
  resolved.add(requestId);
  if (resolved.size <= MAX_RESOLVED_REQUESTS) return;
  const oldest = resolved.values().next().value;
  if (oldest !== undefined) resolved.delete(oldest);
}

function resultState(result: DesktopCommandResult): DesktopRecord | null {
  return record(result.state);
}

function modeLabel(mode: string): string {
  if (mode === 'ordinary') return 'ОБЫЧНАЯ';
  if (mode === 'state-change') return 'ИЗМЕНЕНИЕ СОСТОЯНИЯ';
  if (mode === 'completed-state-change') return 'ЗАВЕРШЁННОЕ ИЗМЕНЕНИЕ СОСТОЯНИЯ';
  return mode || 'НЕИЗВЕСТЕН';
}

export function useCommandApproval(port: DesktopPort) {
  const current = shallowRef<CommandApprovalRequest | null>(null);
  const pending = ref(false);
  const outcomeError = ref('');
  const statusOverride = ref('');
  const resolved = new Set<string>();
  let active = true;
  let currentRevision = 0;
  let generation = 0;

  function applyCoordination(value: DesktopRecord): void {
    const nextRevision = revision(value);
    if (nextRevision < currentRevision) return;
    const next = requestFrom(value);
    if (nextRevision === currentRevision
      && current.value !== null
      && next?.requestId !== current.value.requestId) return;
    currentRevision = nextRevision;
    if (next === null || resolved.has(next.requestId)) {
      if (current.value !== null) generation += 1;
      current.value = null;
      pending.value = false;
      statusOverride.value = '';
      return;
    }
    if (current.value?.requestId === next.requestId) return;
    generation += 1;
    current.value = next;
    pending.value = false;
    outcomeError.value = '';
    statusOverride.value = '';
  }

  async function resolve(decision: 'approve' | 'reject'): Promise<void> {
    const request = current.value;
    if (request === null || pending.value || resolved.has(request.requestId)) return;
    pending.value = true;
    const requestGeneration = generation;
    statusOverride.value = decision === 'approve'
      ? (request.mode === 'state-change' ? 'СОХРАНЕНИЕ И ВЫПОЛНЕНИЕ...' : 'ВЫПОЛНЕНИЕ КОМАНДЫ...')
      : 'ОТКЛОНЕНИЕ ЗАПРОСА...';

    const result = await port.resolveCommandExecution({ requestId: request.requestId, decision });
    if (!active || requestGeneration !== generation || current.value?.requestId !== request.requestId) return;
    pending.value = false;
    remember(resolved, request.requestId);
    const state = resultState(result);
    if (state !== null && revision(state) >= currentRevision) applyCoordination(state);
    if (current.value?.requestId === request.requestId) {
      generation += 1;
      current.value = null;
    }
    if (result.ok !== true) {
      outcomeError.value = text(result.error, 'СОСТОЯНИЕ КОМАНДЫ НЕ УДАЛОСЬ СОХРАНИТЬ');
    }
  }

  const release = port.onCoordinationState(applyCoordination);
  onUnmounted(() => {
    active = false;
    generation += 1;
    release();
    current.value = null;
    resolved.clear();
  });

  const status = computed(() => {
    if (statusOverride.value !== '') return statusOverride.value;
    const request = current.value;
    if (request === null) return '';
    const commandName = request.commandName || request.commandId || '—';
    return `ЗАПРОС: ${request.requestId} · РЕЖИМ: ${modeLabel(request.mode)} · КОМАНДА: ${commandName}`;
  });

  return {
    current: readonly(current),
    outcomeError: readonly(outcomeError),
    pending: readonly(pending),
    status,
    approve: () => resolve('approve'),
    reject: () => resolve('reject'),
  };
}
