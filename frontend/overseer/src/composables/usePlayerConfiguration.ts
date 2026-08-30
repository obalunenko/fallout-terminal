import { computed, inject, onUnmounted, readonly, ref } from 'vue';

import { overseerCoexistenceBridgeKey, type OverseerCoexistenceMessage } from '../mount.js';
import type { DesktopDocumentResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function usePlayerConfiguration(port: DesktopPort) {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const active = ref(false);
  const broadcastActive = ref(false);
  const externalPending = ref(false);
  const localPending = ref(false);
  const error = ref('');
  const revision = ref(0);
  const status = ref('НЕ ВЫБРАНА · СОЗДАЙТЕ ИЛИ ВЫБЕРИТЕ ФАЙЛ');
  const blocked = computed(() => broadcastActive.value || externalPending.value || localPending.value);
  let mounted = true;
  let generation = 0;

  function applyCoordination(coordination: DesktopRecord | null, pending: boolean): void {
    externalPending.value = pending;
    if (coordination === null) {
      active.value = false;
      broadcastActive.value = false;
      revision.value = 0;
      status.value = 'НЕ ВЫБРАНА · СОЗДАЙТЕ ИЛИ ВЫБЕРИТЕ ФАЙЛ';
      return;
    }
    const nextRevision = Number(coordination.revision ?? 0);
    if (!Number.isSafeInteger(nextRevision) || nextRevision < revision.value) return;
    revision.value = nextRevision;
    broadcastActive.value = isRecord(coordination.broadcast);
    const config = isRecord(coordination.playerConfig) ? coordination.playerConfig : null;
    active.value = config !== null;
    status.value = config === null
      ? 'НЕ ВЫБРАНА · СОЗДАЙТЕ ИЛИ ВЫБЕРИТЕ ФАЙЛ'
      : `${typeof config.name === 'string' ? config.name : 'Игроки'} · ${typeof config.filePath === 'string' ? config.filePath : ''}`;
  }

  function handleLegacyMessage(message: OverseerCoexistenceMessage): void {
    if (message.kind === 'coordination-state') {
      applyCoordination(isRecord(message.coordination) ? message.coordination : null, message.pending === true);
      return;
    }
    if (message.kind === 'player-configuration-load-referenced') {
      void run(() => port.loadReferencedPlayerConfig(), 'КОНФИГУРАЦИЯ ИГРОКОВ ЗАГРУЖЕНА');
    } else if (message.kind === 'player-configuration-missing') {
      error.value = 'ВЫБЕРИТЕ ИЛИ СОЗДАЙТЕ КОНФИГУРАЦИЮ ИГРОКОВ';
    }
  }

  async function run(command: () => Promise<DesktopDocumentResult>, successMessage: string): Promise<void> {
    if (blocked.value) return;
    const requestGeneration = ++generation;
    const expectedRevision = revision.value;
    localPending.value = true;
    error.value = '';
    bridge?.vueToLegacy({ expectedRevision, kind: 'player-configuration-command-started' });
    let result: DesktopDocumentResult;
    try {
      result = await command();
    } catch (cause) {
      result = {
        canceled: false,
        error: cause instanceof Error ? cause.message : String(cause),
        ok: false,
        session: null,
      };
    }
    if (!mounted || requestGeneration !== generation) return;
    localPending.value = false;
    if (result.canceled) {
      status.value = 'ВЫБОР КОНФИГУРАЦИИ ОТМЕНЁН';
    } else if (!result.ok || !isRecord(result.session)
      || typeof result.session.playerConfig !== 'string' || result.session.playerConfig === '') {
      error.value = result.error || 'НЕ УДАЛОСЬ ЗАГРУЗИТЬ КОНФИГУРАЦИЮ ИГРОКОВ';
    } else {
      status.value = successMessage;
    }
    bridge?.vueToLegacy({
      expectedRevision,
      kind: 'player-configuration-command-finished',
      result,
      successMessage,
    });
  }

  const release = bridge?.subscribeLegacyState(handleLegacyMessage) ?? (() => {});
  onUnmounted(() => {
    mounted = false;
    generation += 1;
    release();
  });

  return {
    active: readonly(active),
    blocked: readonly(blocked),
    create: () => run(() => port.newPlayerConfig(), 'КОНФИГУРАЦИЯ ИГРОКОВ СОЗДАНА'),
    error: readonly(error),
    manage: () => { bridge?.vueToLegacy({ kind: 'player-management-open-request' }); },
    open: () => run(() => port.openPlayerConfig(), 'КОНФИГУРАЦИЯ ИГРОКОВ ВЫБРАНА'),
    status: readonly(status),
  };
}
