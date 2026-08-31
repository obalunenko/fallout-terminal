import { readonly, ref, shallowRef } from 'vue';

import type { DesktopRecord, DesktopRuntimeStatus } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function useRuntimeStatus(port: DesktopPort) {
  const snapshot = shallowRef<DesktopRuntimeStatus>(Object.freeze({ ok: true }));
  const fatal = ref(false);
  const state = ref('starting');
  const text = ref('ЗАПУСК ЛОКАЛЬНОГО СЕРВЕРА…');
  let active = true;

  function project(value: DesktopRuntimeStatus): void {
    const info = isRecord(value.serverInfo) ? value.serverInfo : null;
    const startupError = typeof value.startupError === 'string' ? value.startupError : '';
    const tunnelError = typeof info?.tunnelError === 'string' ? info.tunnelError : '';
    fatal.value = info === null && startupError !== '';

    if (fatal.value) {
      state.value = 'failed';
      text.value = `ЗАПУСК НЕ ЗАВЕРШЁН: ${startupError}`;
      return;
    }
    if (info?.tunnel === true && typeof info.url === 'string' && info.url !== '') {
      const localURL = typeof info.localUrl === 'string' ? info.localUrl : '';
      state.value = 'ready-public';
      text.value = `ГОТОВО · ПУБЛИЧНЫЙ И ЛОКАЛЬНЫЙ ДОСТУП${localURL ? ` · ${localURL}` : ''}`;
      return;
    }
    if (info !== null) {
      const warning = tunnelError || startupError;
      const localURL = typeof info.localUrl === 'string' && info.localUrl !== ''
        ? info.localUrl
        : typeof info.url === 'string' ? info.url : '';
      state.value = warning ? 'warning' : 'ready-local';
      text.value = warning
        ? `ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ · ПУБЛИЧНЫЙ ДОСТУП НЕДОСТУПЕН: ${warning}`
        : `ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ · ${localURL}`;
      return;
    }
    state.value = 'starting';
    text.value = 'ЗАПУСК ЛОКАЛЬНОГО СЕРВЕРА…';
  }

  function apply(value: DesktopRuntimeStatus): void {
    if (!active) return;
    snapshot.value = Object.freeze({ ...value });
    project(snapshot.value);
  }

  function applyServerInfo(serverInfo: DesktopRecord): void {
    apply(Object.freeze({ ...snapshot.value, serverInfo }));
  }

  void port.getRuntimeStatus().then(apply).catch(cause => {
    if (!active) return;
    apply(Object.freeze({
      ok: false,
      startupError: cause instanceof Error ? cause.message : String(cause),
    }));
  });

  return {
    fatal: readonly(fatal),
    state: readonly(state),
    text: readonly(text),
    applyServerInfo,
    dispose: () => {
      active = false;
    },
  };
}
