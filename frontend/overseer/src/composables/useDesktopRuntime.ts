import { computed, onUnmounted, readonly, ref, shallowRef } from 'vue';

import type { DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

export function useDesktopRuntime(
  port: DesktopPort,
  applyServerInfo: (value: DesktopRecord) => void,
  disposeStatus: () => void,
) {
  const serverInfo = shallowRef<DesktopRecord | null>(null);
  const clientCount = ref(0);
  const openFailure = ref('');
  let active = true;

  const publicURL = computed(() => serverInfo.value?.tunnel === true
    && typeof serverInfo.value.url === 'string' ? serverInfo.value.url : '');
  const localURL = computed(() => {
    const info = serverInfo.value;
    if (info === null) return '';
    if (typeof info.localUrl === 'string' && info.localUrl !== '') return info.localUrl;
    return info.tunnel !== true && typeof info.url === 'string' ? info.url : '';
  });
  const playerURL = computed(() => publicURL.value || localURL.value);
  const tunnelUnavailable = computed(() => serverInfo.value?.tunnel === true && publicURL.value === '');
  const tunnelError = computed(() => typeof serverInfo.value?.tunnelError === 'string'
    ? serverInfo.value.tunnelError
    : '');
  const label = computed(() => {
    if (tunnelError.value !== '') {
      return localURL.value ? `NGROK: ОШИБКА · ЛОКАЛЬНО: ${localURL.value}` : 'NGROK: ОШИБКА';
    }
    return playerURL.value || '—';
  });
  const title = computed(() => {
    let value = 'Адрес игроков пока недоступен';
    if (tunnelError.value !== '') {
      value = localURL.value
        ? `${tunnelError.value}\nЛокальная ссылка остаётся доступна (нажмите, чтобы открыть)`
        : tunnelError.value;
    } else if (publicURL.value !== '') {
      value = localURL.value
        ? `Публичная ссылка (нажмите, чтобы открыть)\nЛокально: ${localURL.value}`
        : 'Публичная ссылка (нажмите, чтобы открыть)';
    } else if (localURL.value !== '') {
      value = tunnelUnavailable.value
        ? 'Публичная ссылка недоступна; локальная ссылка остаётся доступна'
        : 'Локальная ссылка (нажмите, чтобы открыть)';
    }
    return openFailure.value === '' ? value : `${value}\n${openFailure.value}`;
  });

  const releaseServerInfo = port.onServerInfo(value => {
    serverInfo.value = value;
    openFailure.value = '';
    applyServerInfo(value);
  });
  const releaseClientCount = port.onClientCount(value => {
    clientCount.value = value;
  });

  async function openPlayerURL(): Promise<void> {
    const requestedURL = playerURL.value;
    if (requestedURL === '') return;
    openFailure.value = '';
    const result = await port.openUrl(requestedURL);
    if (!active || requestedURL !== playerURL.value || result.ok === true) return;
    openFailure.value = `Не удалось открыть ссылку${result.error ? `: ${result.error}` : ''}`;
  }

  onUnmounted(() => {
    active = false;
    disposeStatus();
    releaseClientCount();
    releaseServerInfo();
  });

  return {
    clientCount: readonly(clientCount),
    label,
    playerURL,
    serverError: computed(() => tunnelError.value !== '' || tunnelUnavailable.value),
    title,
    openPlayerURL,
  };
}
