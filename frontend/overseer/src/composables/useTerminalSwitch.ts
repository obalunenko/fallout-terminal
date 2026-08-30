import { inject, onUnmounted, readonly, ref } from 'vue';

import {
  overseerCoexistenceBridgeKey,
  type OverseerCoexistenceMessage,
} from '../mount.js';
import type { DesktopCommandResult } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const MAX_RESOLVED_REQUESTS = 128;

function messageSwitchId(message: OverseerCoexistenceMessage): string {
  return typeof message.switchId === 'string' ? message.switchId : '';
}

function remember(resolved: Set<string>, switchId: string): void {
  resolved.add(switchId);
  if (resolved.size <= MAX_RESOLVED_REQUESTS) return;
  const oldest = resolved.values().next().value;
  if (oldest !== undefined) resolved.delete(oldest);
}

export function useTerminalSwitch(port: DesktopPort) {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const switchId = ref('');
  const pending = ref(false);
  const error = ref('');
  const resolved = new Set<string>();
  let active = true;
  let generation = 0;

  function handleLegacyMessage(message: OverseerCoexistenceMessage): void {
    if (message.kind === 'terminal-switch-dismissed') {
      if (switchId.value !== '') generation += 1;
      switchId.value = '';
      pending.value = false;
      error.value = '';
      return;
    }
    if (message.kind !== 'terminal-switch-required') return;
    const nextSwitchId = messageSwitchId(message);
    if (nextSwitchId === '' || resolved.has(nextSwitchId) || switchId.value === nextSwitchId) return;
    generation += 1;
    switchId.value = nextSwitchId;
    pending.value = false;
    error.value = '';
  }

  async function resolve(decision: 'preserve' | 'discard' | 'cancel'): Promise<void> {
    const requestId = switchId.value;
    if (requestId === '' || pending.value || resolved.has(requestId)) return;
    pending.value = true;
    error.value = '';
    const requestGeneration = generation;

    let result: DesktopCommandResult;
    try {
      result = await port.resolveTerminalSwitch({ decision, switchId: requestId });
    } catch (cause) {
      result = {
        error: cause instanceof Error ? cause.message : String(cause),
        ok: false,
      };
    }
    if (!active || requestGeneration !== generation || switchId.value !== requestId) return;
    pending.value = false;
    if (result.ok !== true) {
      error.value = result.error || 'РЕШЕНИЕ ОТКЛОНЕНО';
      return;
    }
    remember(resolved, requestId);
    generation += 1;
    switchId.value = '';
    bridge?.vueToLegacy({
      decision,
      kind: 'terminal-switch-resolved',
      result,
      switchId: requestId,
    });
  }

  const release = bridge?.subscribeLegacyState(handleLegacyMessage) ?? (() => {});
  onUnmounted(() => {
    active = false;
    generation += 1;
    release();
    switchId.value = '';
    resolved.clear();
  });

  return {
    error: readonly(error),
    pending: readonly(pending),
    switchId: readonly(switchId),
    cancel: () => resolve('cancel'),
    discard: () => resolve('discard'),
    preserve: () => resolve('preserve'),
  };
}
