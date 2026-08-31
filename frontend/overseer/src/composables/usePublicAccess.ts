import { computed, inject, onUnmounted, readonly, ref, shallowRef } from 'vue';

import { overseerCoexistenceBridgeKey } from '../mount.js';
import type {
  DesktopCommandResult,
  DesktopPublicAccessSnapshot,
  DesktopRecord,
} from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const PUBLIC_ACCESS_STATES: Readonly<Record<string, string>> = Object.freeze({
  disabled: 'stopped',
  error: 'error',
  failed: 'error',
  ready: 'ready',
  starting: 'starting',
  stopped: 'stopped',
  stopping: 'stopping',
});
const SECRET_PRESENCES = new Set(['absent', 'present', 'unknown']);

const SECURE_STORE_FAILURES: Readonly<Record<string, string>> = Object.freeze({
  secret_store_locked: 'Unlock the secure credential store and try again.',
  secret_store_denied: 'Allow secure credential store access and try again.',
  secret_store_unavailable: 'The secure credential store is unavailable; local access remains available.',
});

export interface PublicAccessViewSnapshot {
  readonly generation: number;
  readonly playerPasswordPresence: string;
  readonly preferences: Readonly<{
    readonly reservedDomain: string;
    readonly revision: number;
    readonly username: string;
  }>;
  readonly providerTokenPresence: string;
  readonly settingsRevision: number;
  readonly status: Readonly<{
    readonly errorCategory: string;
    readonly errorMessage: string;
    readonly generation: number;
    readonly publicUrl: string;
    readonly settingsRevision: number;
    readonly state: string;
  }>;
}

const EMPTY_SNAPSHOT: PublicAccessViewSnapshot = Object.freeze({
  generation: -1,
  playerPasswordPresence: 'unknown',
  preferences: Object.freeze({ reservedDomain: '', revision: 0, username: 'players' }),
  providerTokenPresence: 'unknown',
  settingsRevision: -1,
  status: Object.freeze({
    errorCategory: '',
    errorMessage: '',
    generation: -1,
    publicUrl: '',
    settingsRevision: -1,
    state: '',
  }),
});

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function safeInteger(value: unknown, fallback = 0): number {
  return Number.isSafeInteger(value) && Number(value) >= 0 ? Number(value) : fallback;
}

function text(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

function projectSnapshot(value: DesktopPublicAccessSnapshot): PublicAccessViewSnapshot | null {
  const preferences: DesktopRecord = isRecord(value.preferences) ? value.preferences : Object.freeze({});
  const status = isRecord(value.status) ? value.status : value;
  const generation = safeInteger(status.generation, -1);
  const settingsRevision = safeInteger(status.settingsRevision, -1);
  if (generation < 0 || settingsRevision < 0) return null;
  const state = PUBLIC_ACCESS_STATES[text(status.state)] ?? '';
  const presence = (candidate: unknown): string => {
    const projected = text(candidate, 'unknown');
    return SECRET_PRESENCES.has(projected) ? projected : 'unknown';
  };
  return Object.freeze({
    generation,
    playerPasswordPresence: presence(value.playerPasswordPresence),
    preferences: Object.freeze({
      reservedDomain: text(preferences.reservedDomain),
      revision: safeInteger(preferences.revision, settingsRevision),
      username: text(preferences.username, 'players') || 'players',
    }),
    providerTokenPresence: presence(value.providerTokenPresence),
    settingsRevision,
    status: Object.freeze({
      errorCategory: state === 'error' ? text(status.errorCategory) : '',
      errorMessage: state === 'error' ? text(status.errorMessage) : '',
      generation,
      publicUrl: state === 'ready' ? text(status.publicUrl) : '',
      settingsRevision,
      state,
    }),
  });
}

function normalizedSnapshot(value: unknown): DesktopPublicAccessSnapshot | null {
  if (!isRecord(value)) return null;
  const status = isRecord(value.status) ? value.status : value;
  const generation = status.generation;
  const settingsRevision = status.settingsRevision;
  if (!Number.isSafeInteger(generation) || Number(generation) < 0
    || !Number.isSafeInteger(settingsRevision) || Number(settingsRevision) < 0) return null;
  return Object.freeze({
    ...value,
    generation: Number(generation),
    settingsRevision: Number(settingsRevision),
  });
}

function isNewer(candidate: PublicAccessViewSnapshot, baseline: PublicAccessViewSnapshot): boolean {
  return candidate.generation > baseline.generation
    || (candidate.generation === baseline.generation
      && candidate.settingsRevision > baseline.settingsRevision);
}

function displayURL(snapshot: PublicAccessViewSnapshot): string {
  if (snapshot.status.state === 'ready') return snapshot.status.publicUrl;
  if (snapshot.status.state === 'error') return '';
  const reservedDomain = snapshot.preferences.reservedDomain.trim();
  if (reservedDomain === '') return '';
  return /^https?:\/\//i.test(reservedDomain) ? reservedDomain : `https://${reservedDomain}`;
}

export function usePublicAccess(port: DesktopPort) {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const snapshot = shallowRef<PublicAccessViewSnapshot>(EMPTY_SNAPSHOT);
  const loaded = ref(false);
  const pending = ref(false);
  const commandError = ref('');
  const copyStatus = ref('');
  let active = true;
  let commandGeneration = 0;

  function publishToLegacy(value: PublicAccessViewSnapshot): void {
    bridge?.vueToLegacy({ kind: 'public-access-snapshot', snapshot: value });
  }

  function applyProjectedSnapshot(projected: PublicAccessViewSnapshot, publish = true): void {
    if (loaded.value && !isNewer(projected, snapshot.value)
      && (projected.generation !== snapshot.value.generation
        || projected.settingsRevision !== snapshot.value.settingsRevision)) return;
    snapshot.value = projected;
    loaded.value = true;
    if (publish) publishToLegacy(projected);
  }

  function applyDesktopSnapshot(value: DesktopPublicAccessSnapshot, publish = true): void {
    const projected = projectSnapshot(value);
    if (projected !== null) applyProjectedSnapshot(projected, publish);
  }

  async function runLifecycle(
    command: (request: DesktopRecord) => Promise<DesktopCommandResult>,
  ): Promise<void> {
    if (pending.value || !loaded.value) return;
    const invocation = ++commandGeneration;
    pending.value = true;
    commandError.value = '';
    copyStatus.value = '';
    const result = await command({ expectedRevision: snapshot.value.preferences.revision });
    if (!active || invocation !== commandGeneration) return;
    pending.value = false;
    const next = normalizedSnapshot(result.snapshot);
    if (next !== null) applyDesktopSnapshot(next);
    if (result.ok !== true) {
      commandError.value = text(result.error, 'ОПЕРАЦИЯ ПУБЛИЧНОГО ДОСТУПА НЕ ВЫПОЛНЕНА');
    }
  }

  function openSettings(setupRequired = false): void {
    if (pending.value) return;
    bridge?.vueToLegacy({ kind: 'public-access-settings-open', setupRequired });
  }

  async function start(): Promise<void> {
    const configured = snapshot.value.providerTokenPresence === 'present'
      && snapshot.value.playerPasswordPresence === 'present';
    if (!configured) {
      openSettings(true);
      return;
    }
    await runLifecycle(request => port.startPublicAccess(request));
  }

  async function copyURL(): Promise<void> {
    const value = displayURL(snapshot.value);
    if (value === '' || pending.value) return;
    copyStatus.value = '';
    let copied = false;
    try {
      if (typeof navigator.clipboard?.writeText === 'function') {
        await navigator.clipboard.writeText(value);
        copied = true;
      }
    } catch {
      // The native clipboard is the bounded fallback for packaged WebViews.
    }
    if (!copied) copied = await port.writeClipboardText(value);
    if (!active) return;
    copyStatus.value = copied ? 'URL СКОПИРОВАН' : 'НЕ УДАЛОСЬ СКОПИРОВАТЬ';
  }

  const releaseStatus = port.onPublicAccessStatus(applyDesktopSnapshot);
  const releaseBridge = bridge?.subscribeLegacyState(message => {
    if (message.kind !== 'public-access-settings-snapshot') return;
    const next = normalizedSnapshot(message.snapshot);
    if (next !== null) applyDesktopSnapshot(next, false);
  });
  onUnmounted(() => {
    active = false;
    commandGeneration += 1;
    releaseBridge?.();
    releaseStatus();
  });

  const transitioning = computed(() => ['starting', 'stopping'].includes(snapshot.value.status.state));
  const controlsDisabled = computed(() => pending.value || transitioning.value || !loaded.value);
  const publicURL = computed(() => displayURL(snapshot.value));
  const failure = computed(() => {
    if (commandError.value !== '') return commandError.value;
    if (snapshot.value.status.state !== 'error') return '';
    const detail = SECURE_STORE_FAILURES[snapshot.value.status.errorCategory]
      || snapshot.value.status.errorMessage
      || 'ПУБЛИЧНЫЙ ДОСТУП НЕДОСТУПЕН';
    return `${detail} · ЛОКАЛЬНЫЙ РЕЖИМ ПРОДОЛЖАЕТ РАБОТАТЬ`;
  });

  return {
    controlsDisabled,
    copyStatus: readonly(copyStatus),
    failure,
    loaded: readonly(loaded),
    pending: readonly(pending),
    publicURL,
    snapshot: readonly(snapshot),
    applySnapshot: applyProjectedSnapshot,
    copyURL,
    openSettings: () => openSettings(),
    start,
    stop: () => runLifecycle(request => port.stopPublicAccess(request)),
  };
}
