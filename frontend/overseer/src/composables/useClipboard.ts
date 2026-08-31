import { onUnmounted, readonly, ref } from 'vue';

import type { DesktopPort } from '../ports/desktop-port.js';

const STATUS_LIFETIME_MS = 4_000;

export function useClipboard(port: DesktopPort) {
  const status = ref('');
  let active = true;
  let statusTimer: ReturnType<typeof setTimeout> | null = null;

  function clear(): void {
    if (statusTimer !== null) clearTimeout(statusTimer);
    statusTimer = null;
    status.value = '';
  }

  function setStatus(value: string): void {
    clear();
    status.value = value;
    statusTimer = setTimeout(() => {
      statusTimer = null;
      if (active) status.value = '';
    }, STATUS_LIFETIME_MS);
  }

  async function copy(value: unknown, successMessage = 'СКОПИРОВАНО'): Promise<boolean> {
    if (typeof value !== 'string' || value === '') {
      setStatus('НЕ УДАЛОСЬ СКОПИРОВАТЬ');
      return false;
    }
    let copied = false;
    try {
      if (typeof navigator.clipboard?.writeText === 'function') {
        await navigator.clipboard.writeText(value);
        copied = true;
      }
    } catch {
      // The native Wails port is the bounded fallback for packaged WebViews.
    }
    if (!copied) copied = await port.writeClipboardText(value);
    if (!active) return false;
    setStatus(copied ? successMessage : 'НЕ УДАЛОСЬ СКОПИРОВАТЬ');
    return copied;
  }

  onUnmounted(() => {
    active = false;
    clear();
  });

  return { clear, copy, status: readonly(status) };
}
