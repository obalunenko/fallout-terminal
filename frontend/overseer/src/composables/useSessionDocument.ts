import { inject, onUnmounted, readonly, ref } from 'vue';

import { overseerCoexistenceBridgeKey, type OverseerCoexistenceMessage } from '../mount.js';
import type { DesktopDocumentResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function useSessionDocument(port: DesktopPort) {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const fatal = ref(false);
  const loaded = ref(false);
  const pending = ref(false);
  const error = ref('');
  const startupState = ref('starting');
  const status = ref('ЗАПУСК ЛОКАЛЬНОГО СЕРВЕРА…');
  let active = true;

  function handleLegacyMessage(message: OverseerCoexistenceMessage): void {
    if (message.kind !== 'startup-status') return;
    fatal.value = message.fatal === true;
    startupState.value = typeof message.state === 'string' ? message.state : 'starting';
    status.value = typeof message.text === 'string' ? message.text : '';
  }

  async function acquire(command: () => Promise<DesktopDocumentResult>): Promise<void> {
    if (pending.value) return;
    pending.value = true;
    error.value = '';
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
    if (!active) return;
    pending.value = false;
    if (result.canceled) return;
    const filePath = typeof result.filePath === 'string' ? result.filePath : '';
    if (!result.ok || !isRecord(result.session) || filePath.length === 0) {
      error.value = result.error || 'ДОКУМЕНТ СЕССИИ НЕ ЗАГРУЖЕН';
      return;
    }
    if (bridge?.vueToLegacy({
      filePath,
      kind: 'session-document-loaded',
      session: result.session,
    }) !== true) {
      error.value = 'ДОКУМЕНТ СЕССИИ НЕ ПЕРЕДАН ПРИЛОЖЕНИЮ';
      return;
    }
    loaded.value = true;
  }

  const release = bridge?.subscribeLegacyState(handleLegacyMessage) ?? (() => {});
  onUnmounted(() => {
    active = false;
    release();
  });

  return {
    error: readonly(error),
    fatal: readonly(fatal),
    loaded: readonly(loaded),
    pending: readonly(pending),
    startupState: readonly(startupState),
    status: readonly(status),
    create: () => acquire(() => port.newSession()),
    open: () => acquire(() => port.openSession()),
  };
}
