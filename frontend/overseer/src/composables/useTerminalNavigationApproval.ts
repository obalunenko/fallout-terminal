import { computed, onUnmounted, readonly, ref, shallowRef } from 'vue';

import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const MAX_RESOLVED_REQUESTS = 128;

export interface TerminalNavigationRequest {
  readonly commandId: string;
  readonly commandName: string;
  readonly direction: string;
  readonly requestId: string;
  readonly sourceTerminalId: string;
  readonly sourceTerminalName: string;
  readonly targetTerminalId: string;
  readonly targetTerminalName: string;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function record(value: unknown): DesktopRecord | null {
  return isRecord(value) ? value : null;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function revision(value: DesktopRecord | null): number {
  const candidate = value?.revision;
  return Number.isSafeInteger(candidate) && Number(candidate) >= 0 ? Number(candidate) : 0;
}

function requestFrom(value: DesktopRecord | null): TerminalNavigationRequest | null {
  const pending = record(value?.pendingTerminalNavigation);
  const requestId = text(pending?.requestId);
  if (requestId === '') return null;
  return Object.freeze({
    commandId: text(pending?.commandId),
    commandName: text(pending?.commandName),
    direction: text(pending?.direction),
    requestId,
    sourceTerminalId: text(pending?.sourceTerminalId),
    sourceTerminalName: text(pending?.sourceTerminalName),
    targetTerminalId: text(pending?.targetTerminalId),
    targetTerminalName: text(pending?.targetTerminalName),
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

export function useTerminalNavigationApproval(port: DesktopPort) {
  const current = shallowRef<TerminalNavigationRequest | null>(null);
  const pending = ref(false);
  const outcomeError = ref('');
  const decision = ref<'approve' | 'reject' | null>(null);
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
      decision.value = null;
      return;
    }
    if (current.value?.requestId === next.requestId) return;
    generation += 1;
    current.value = next;
    pending.value = false;
    outcomeError.value = '';
    decision.value = null;
  }

  async function resolve(nextDecision: 'approve' | 'reject'): Promise<void> {
    const request = current.value;
    if (request === null || pending.value || resolved.has(request.requestId)) return;
    pending.value = true;
    decision.value = nextDecision;
    const requestGeneration = generation;

    let result: DesktopCommandResult;
    try {
      result = await port.resolveTerminalNavigation({
        decision: nextDecision,
        requestId: request.requestId,
      });
    } catch (error) {
      result = {
        error: error instanceof Error ? error.message : String(error),
        ok: false,
      };
    }
    if (!active || requestGeneration !== generation || current.value?.requestId !== request.requestId) return;
    pending.value = false;
    remember(resolved, request.requestId);
    const state = resultState(result);
    if (state !== null && revision(state) >= currentRevision) applyCoordination(state);
    if (current.value?.requestId === request.requestId) {
      generation += 1;
      current.value = null;
    }
    decision.value = null;
    if (result.ok !== true && !record(result.state)?.terminalNavigationNotice) {
      outcomeError.value = text(result.error) || 'ПЕРЕХОД НЕ ВЫПОЛНЕН';
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
    if (pending.value) {
      return decision.value === 'approve' ? 'ВЫПОЛНЕНИЕ ПЕРЕХОДА...' : 'ОТКЛОНЕНИЕ ПЕРЕХОДА...';
    }
    return current.value === null ? '' : 'ИСХОДНЫЙ ТЕРМИНАЛ ОСТАЁТСЯ АКТИВНЫМ ДО РЕШЕНИЯ';
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
