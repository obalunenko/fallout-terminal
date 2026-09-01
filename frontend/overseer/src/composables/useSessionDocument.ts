import { inject, onUnmounted, readonly, ref } from 'vue';

import { overseerControllerKey } from '../controllers/overseer-controller.js';
import type { DesktopDocumentResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function useSessionDocument(port: DesktopPort) {
  const controller = inject(overseerControllerKey, null);
  const documentPath = ref('');
  const loaded = ref(false);
  const pending = ref(false);
  const error = ref('');
  let active = true;

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
    let sessionRevision = 0;
    try {
      const status = await port.getRuntimeStatus();
      const savedRevision = Number(status.savedRevision);
      if (Number.isSafeInteger(savedRevision) && savedRevision >= 0) sessionRevision = savedRevision;
    } catch {
      // Document acquisition remains usable when optional status presentation is unavailable.
    }
    if (!active) return;
    if (controller?.dispatch({
      filePath,
      kind: 'session-document-loaded',
      session: result.session,
      sessionRevision,
    }) !== true) {
      error.value = 'ДОКУМЕНТ СЕССИИ НЕ ПЕРЕДАН ПРИЛОЖЕНИЮ';
      return;
    }
    documentPath.value = filePath;
    loaded.value = true;
  }

  onUnmounted(() => {
    active = false;
  });

  return {
    error: readonly(error),
    filePath: readonly(documentPath),
    loaded: readonly(loaded),
    pending: readonly(pending),
    create: () => acquire(() => port.newSession()),
    open: () => acquire(() => port.openSession()),
  };
}
