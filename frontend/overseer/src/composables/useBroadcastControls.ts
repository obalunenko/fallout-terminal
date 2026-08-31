import { inject, nextTick, onUnmounted, readonly, ref } from 'vue';

import { overseerCoexistenceBridgeKey, type OverseerCoexistenceMessage } from '../mount.js';
import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

export interface BroadcastSnapshot {
  readonly activeTerminalID: string;
  readonly activeLogicalSessionCount: number;
  readonly broadcastID: string;
  readonly playerConfigActive: boolean;
  readonly revision: number;
}

interface BroadcastCommandOutcome {
  readonly applicable: boolean;
  readonly result: DesktopCommandResult;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function snapshot(value: unknown): BroadcastSnapshot | null {
  if (!isRecord(value)) return null;
  const revision = Number(value.revision ?? 0);
  if (!Number.isSafeInteger(revision) || revision < 0) return null;
  const broadcast = isRecord(value.broadcast) ? value.broadcast : null;
  const sessions = Array.isArray(value.sessions) ? value.sessions : [];
  return Object.freeze({
    activeTerminalID: broadcast && typeof broadcast.activeTerminalId === 'string'
      ? broadcast.activeTerminalId
      : '',
    activeLogicalSessionCount: sessions.filter(session => isRecord(session) && session.connected === true).length,
    broadcastID: broadcast && typeof broadcast.id === 'string' ? broadcast.id : '',
    playerConfigActive: isRecord(value.playerConfig),
    revision,
  });
}

export function useBroadcastControls(port: DesktopPort) {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const current = ref<BroadcastSnapshot>(Object.freeze({
    activeLogicalSessionCount: 0,
    activeTerminalID: '',
    broadcastID: '',
    playerConfigActive: false,
    revision: 0,
  }));
  const error = ref('');
  const pending = ref(false);
  const status = ref('');
  const focusRequest = ref<'end' | 'start' | 'stop' | null>(null);
  const endConfirmationOpen = ref(false);
  const takeOffConfirmationOpen = ref(false);
  const takeOffError = ref('');
  let active = true;
  let lifecycle = 0;

  function handleLegacyMessage(message: OverseerCoexistenceMessage): void {
    if (message.kind === 'broadcast-control-focus-request') {
      if (message.control === 'end' || message.control === 'start' || message.control === 'stop') {
        const control = message.control;
        focusRequest.value = null;
        void nextTick(() => {
          if (active) focusRequest.value = control;
        });
      }
      return;
    }
    if (message.kind === 'broadcast-confirmation-closed') {
      if (message.dialog === 'end') endConfirmationOpen.value = false;
      if (message.dialog === 'take-off') takeOffConfirmationOpen.value = false;
      return;
    }
    if (message.kind !== 'coordination-state') return;
    const next = snapshot(message.coordination);
    if (next === null || next.revision < current.value.revision) return;
    current.value = next;
    pending.value = message.pending === true;
    status.value = typeof message.status === 'string' ? message.status : '';
    error.value = typeof message.error === 'string' ? message.error : '';
    if (next.broadcastID === '') {
      endConfirmationOpen.value = false;
      takeOffConfirmationOpen.value = false;
    } else if (next.activeTerminalID === '') {
      takeOffConfirmationOpen.value = false;
    }
  }

  async function command(
    run: () => Promise<DesktopCommandResult>,
    pendingMessage: string,
    successMessage: string,
    normalize: (result: DesktopCommandResult) => DesktopCommandResult = result => result,
  ): Promise<BroadcastCommandOutcome | null> {
    if (!active || pending.value) return null;
    const expectedRevision = current.value.revision;
    const expectedLifecycle = lifecycle;
    pending.value = true;
    status.value = pendingMessage;
    error.value = '';
    bridge?.vueToLegacy({ expectedRevision, kind: 'broadcast-command-started', status: pendingMessage });
    let result: DesktopCommandResult;
    try {
      result = normalize(await run());
    } catch (cause) {
      result = { error: cause instanceof Error ? cause.message : String(cause), ok: false };
    }
    const applicable = active && lifecycle === expectedLifecycle
      && current.value.revision === expectedRevision;
    if (applicable) {
      pending.value = false;
      status.value = result.ok ? successMessage : '';
      error.value = result.ok ? '' : (result.error || 'ОПЕРАЦИЯ ОТКЛОНЕНА');
    }
    bridge?.vueToLegacy({ expectedRevision, kind: 'broadcast-command-finished', result, successMessage });
    return Object.freeze({ applicable, result });
  }

  function start(): void {
    if (current.value.broadcastID !== '' || !current.value.playerConfigActive) return;
    void command(() => port.startBroadcast(), 'ЗАПУСК ТРАНСЛЯЦИИ...', 'ТРАНСЛЯЦИЯ ЗАПУЩЕНА');
  }

  function requestEnd(): void {
    if (pending.value || current.value.broadcastID === '') return;
    lifecycle += 1;
    endConfirmationOpen.value = true;
    bridge?.vueToLegacy({ kind: 'broadcast-end-confirmation-request', revision: current.value.revision });
  }

  function requestTakeOff(): void {
    if (pending.value || current.value.activeTerminalID === '') return;
    lifecycle += 1;
    takeOffError.value = '';
    takeOffConfirmationOpen.value = true;
    bridge?.vueToLegacy({ kind: 'broadcast-take-off-confirmation-request', revision: current.value.revision });
  }

  function manageSessions(): void {
    bridge?.vueToLegacy({ kind: 'logical-session-open-request' });
  }

  function cancelEnd(): void {
    if (pending.value || !endConfirmationOpen.value) return;
    lifecycle += 1;
    endConfirmationOpen.value = false;
    focusRequest.value = 'end';
  }

  function cancelTakeOff(): void {
    if (pending.value || !takeOffConfirmationOpen.value) return;
    lifecycle += 1;
    takeOffConfirmationOpen.value = false;
    takeOffError.value = '';
    focusRequest.value = 'stop';
  }

  async function confirmEnd(): Promise<void> {
    if (!endConfirmationOpen.value || pending.value || current.value.broadcastID === '') return;
    const outcome = await command(
      () => port.endBroadcast(),
      'ЗАВЕРШЕНИЕ ТРАНСЛЯЦИИ...',
      'ТРАНСЛЯЦИЯ ЗАВЕРШЕНА · СЕССИИ И ПЕРСОНАЖИ СОХРАНЕНЫ',
      result => result.ok && (!isRecord(result.state) || result.state.broadcast)
        ? { ...result, error: 'ЗАВЕРШЕНИЕ НЕ ПОДТВЕРЖДЕНО АВТОРИТЕТНЫМ СОСТОЯНИЕМ', ok: false }
        : result,
    );
    if (outcome === null || !outcome.applicable) return;
    endConfirmationOpen.value = false;
    if (outcome.result.ok) {
      bridge?.legacyToVue({ kind: 'terminal-switch-dismissed' });
      focusRequest.value = 'start';
    } else {
      focusRequest.value = 'end';
    }
  }

  async function confirmTakeOff(): Promise<void> {
    if (!takeOffConfirmationOpen.value || pending.value || current.value.activeTerminalID === '') return;
    takeOffError.value = '';
    const outcome = await command(
      () => port.requestTerminalClear(),
      'ОЧИСТКА АКТИВНОГО ТЕРМИНАЛА...',
      'АКТИВНЫЙ ТЕРМИНАЛ УБРАН · ТРАНСЛЯЦИЯ ПРОДОЛЖАЕТСЯ',
      result => {
        if (!result.ok || result.status === 'cleared') return result;
        if (result.status === 'decision-required'
          && typeof result.switchId === 'string' && result.switchId !== '') return result;
        return { ...result, error: 'СНЯТИЕ С ЭФИРА НЕ ПОДТВЕРЖДЕНО', ok: false };
      },
    );
    if (outcome === null || !outcome.applicable) return;
    const result = outcome.result;
    if (!result.ok) {
      takeOffError.value = result.error || 'НЕ УДАЛОСЬ СНЯТЬ ТЕРМИНАЛ С ЭФИРА';
      return;
    }
    if (result.status === 'decision-required' && typeof result.switchId === 'string' && result.switchId !== '') {
      takeOffConfirmationOpen.value = false;
      bridge?.legacyToVue({ kind: 'terminal-switch-required', switchId: result.switchId });
      return;
    }
    takeOffConfirmationOpen.value = false;
    focusRequest.value = 'end';
  }

  const release = bridge?.subscribeLegacyState(handleLegacyMessage) ?? (() => {});
  onUnmounted(() => {
    active = false;
    lifecycle += 1;
    release();
    endConfirmationOpen.value = false;
    takeOffConfirmationOpen.value = false;
    takeOffError.value = '';
    focusRequest.value = null;
  });

  return {
    cancelEnd,
    cancelTakeOff,
    confirmEnd,
    confirmTakeOff,
    current: readonly(current),
    endConfirmationOpen: readonly(endConfirmationOpen),
    error: readonly(error),
    focusRequest: readonly(focusRequest),
    manageSessions,
    pending: readonly(pending),
    requestEnd,
    requestTakeOff,
    start,
    status: readonly(status),
    takeOffConfirmationOpen: readonly(takeOffConfirmationOpen),
    takeOffError: readonly(takeOffError),
  };
}
