import { computed, onUnmounted, readonly, ref, shallowRef } from 'vue';

import type {
  DesktopApplicationUpdateSnapshot,
  DesktopCommandResult,
  DesktopRecord,
} from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

const MAX_APPLICATION_UPDATE_TEXT = 16_384;
const SILENT_STATES = new Set(['', 'disabled', 'idle', 'current']);

const STATUS_LABELS: Readonly<Record<string, string>> = Object.freeze({
  checking: 'ПРОВЕРКА ОБНОВЛЕНИЙ…',
  available: 'ДОСТУПНО ОБНОВЛЕНИЕ ПРИЛОЖЕНИЯ',
  deferred: 'ОБНОВЛЕНИЕ ОТЛОЖЕНО ДО СЛЕДУЮЩЕГО ЗАПУСКА',
  downloading: 'ЗАГРУЗКА ОБНОВЛЕНИЯ…',
  verifying: 'ПРОВЕРКА ЗАГРУЖЕННОГО ОБНОВЛЕНИЯ…',
  staging: 'ПОДГОТОВКА ОБНОВЛЕНИЯ…',
  'ready-to-restart': 'ОБНОВЛЕНИЕ ГОТОВО К ПЕРЕЗАПУСКУ',
  applying: 'ПРИМЕНЕНИЕ ОБНОВЛЕНИЯ…',
  failed: 'НЕ УДАЛОСЬ ПОДГОТОВИТЬ ОБНОВЛЕНИЕ',
});

const FAILURE_STAGE_LABELS: Readonly<Record<string, string>> = Object.freeze({
  check: 'ПРОВЕРКА ОБНОВЛЕНИЙ',
  download: 'ЗАГРУЗКА ОБНОВЛЕНИЯ',
  verify: 'ПРОВЕРКА ЗАГРУЖЕННОГО ОБНОВЛЕНИЯ',
  stage: 'ПОДГОТОВКА ОБНОВЛЕНИЯ',
  apply: 'ПРИМЕНЕНИЕ ОБНОВЛЕНИЯ',
  relaunch: 'ПЕРЕЗАПУСК ОБНОВЛЁННОГО ПРИЛОЖЕНИЯ',
  recovery: 'ВОССТАНОВЛЕНИЕ РАБОЧЕЙ ВЕРСИИ',
});

export interface ApplicationUpdateViewSnapshot {
  readonly attemptId: string;
  readonly availableVersion: string;
  readonly bytesDownloaded: number | null;
  readonly downloadSize: number | null;
  readonly errorMessage: string;
  readonly failedStage: string;
  readonly installedVersion: string;
  readonly recoveryAction: string;
  readonly releaseNotes: string;
  readonly revision: number;
  readonly state: string;
}

const EMPTY_SNAPSHOT: ApplicationUpdateViewSnapshot = Object.freeze({
  attemptId: '',
  availableVersion: '',
  bytesDownloaded: null,
  downloadSize: null,
  errorMessage: '',
  failedStage: '',
  installedVersion: '',
  recoveryAction: '',
  releaseNotes: '',
  revision: -1,
  state: '',
});

function boundedText(value: unknown, fallback = ''): string {
  if (typeof value !== 'string') return fallback;
  if (value.length <= MAX_APPLICATION_UPDATE_TEXT) return value;
  return `${value.slice(0, MAX_APPLICATION_UPDATE_TEXT)}\n\n[Описание выпуска сокращено]`;
}

function optionalInteger(value: unknown): number | null {
  return Number.isSafeInteger(value) ? Number(value) : null;
}

function projectedSnapshot(
  value: DesktopApplicationUpdateSnapshot,
  previous: ApplicationUpdateViewSnapshot,
): ApplicationUpdateViewSnapshot | null {
  if (!Number.isSafeInteger(value.revision) || value.revision < 0) return null;
  const record: DesktopRecord = value;
  return Object.freeze({
    attemptId: Object.hasOwn(record, 'attemptId')
      ? boundedText(record.attemptId)
      : previous.attemptId,
    availableVersion: Object.hasOwn(record, 'availableVersion')
      ? boundedText(record.availableVersion)
      : previous.availableVersion,
    bytesDownloaded: Object.hasOwn(record, 'bytesDownloaded')
      ? optionalInteger(record.bytesDownloaded)
      : previous.bytesDownloaded,
    downloadSize: Object.hasOwn(record, 'downloadSize')
      ? optionalInteger(record.downloadSize)
      : previous.downloadSize,
    errorMessage: Object.hasOwn(record, 'errorMessage')
      ? boundedText(record.errorMessage)
      : previous.errorMessage,
    failedStage: Object.hasOwn(record, 'failedStage')
      ? boundedText(record.failedStage)
      : previous.failedStage,
    installedVersion: Object.hasOwn(record, 'installedVersion')
      ? boundedText(record.installedVersion)
      : previous.installedVersion,
    recoveryAction: Object.hasOwn(record, 'recoveryAction')
      ? boundedText(record.recoveryAction)
      : previous.recoveryAction,
    releaseNotes: Object.hasOwn(record, 'releaseNotes')
      ? boundedText(record.releaseNotes)
      : previous.releaseNotes,
    revision: value.revision,
    state: Object.hasOwn(record, 'state') ? boundedText(record.state) : previous.state,
  });
}

function resultSnapshot(result: DesktopCommandResult): DesktopApplicationUpdateSnapshot | null {
  const value = result.snapshot;
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const revision = Reflect.get(value, 'revision');
  if (!Number.isSafeInteger(revision) || Number(revision) < 0) return null;
  return Object.freeze({ ...value, revision: Number(revision) });
}

function failureText(snapshot: ApplicationUpdateViewSnapshot): string {
  const stage = FAILURE_STAGE_LABELS[snapshot.failedStage] ?? 'ОБНОВЛЕНИЕ ПРИЛОЖЕНИЯ';
  const message = boundedText(snapshot.errorMessage, 'Операция обновления не завершена.');
  const recovery = boundedText(
    snapshot.recoveryAction,
    'Продолжайте работу и повторите попытку при следующем запуске.',
  );
  return `ЭТАП: ${stage}.\n${message}\n${recovery}`;
}

function connectedElement(value: Element | null): HTMLElement | null {
  return value instanceof HTMLElement ? value : null;
}

export function useApplicationUpdate(port: DesktopPort) {
  const snapshot = shallowRef<ApplicationUpdateViewSnapshot>(EMPTY_SNAPSHOT);
  const offerOpen = ref(false);
  const restartOpen = ref(false);
  const offerPending = ref(false);
  const restartPending = ref(false);
  const commandError = ref('');
  const statusOverride = ref('');
  const offerFocusRequest = ref(0);
  const restartFocusRequest = ref(0);
  const promptedOffers = new Set<string>();
  const promptedRestarts = new Set<string>();
  const suppressedAttempts = new Set<string>();
  let offerAttemptId = '';
  let restartAttemptId = '';
  let offerOpener: HTMLElement | null = null;
  let restartOpener: HTMLElement | null = null;
  let active = true;
  let focusGeneration = 0;

  function restoreFocus(opener: HTMLElement | null): void {
    const generation = ++focusGeneration;
    queueMicrotask(() => {
      if (!active || generation !== focusGeneration || opener?.isConnected !== true) return;
      opener.focus();
    });
  }

  function closeOffer(restore = true): void {
    if (!offerOpen.value && offerAttemptId === '') return;
    offerOpen.value = false;
    offerAttemptId = '';
    const opener = offerOpener;
    offerOpener = null;
    if (restore) restoreFocus(opener);
  }

  function closeRestart(restore = true): void {
    if (!restartOpen.value && restartAttemptId === '') return;
    restartOpen.value = false;
    restartAttemptId = '';
    const opener = restartOpener;
    restartOpener = null;
    if (restore) restoreFocus(opener);
  }

  function openOffer(automatic = false): void {
    const current = snapshot.value;
    if (current.state !== 'available' || current.attemptId === '') return;
    if (suppressedAttempts.has(current.attemptId)) return;
    const key = `${current.attemptId}:${current.revision}`;
    if (automatic && promptedOffers.has(key)) return;
    promptedOffers.add(key);
    offerAttemptId = current.attemptId;
    if (offerOpen.value) return;
    offerOpener = connectedElement(document.activeElement);
    offerOpen.value = true;
    offerFocusRequest.value += 1;
  }

  function openRestart(automatic = false): void {
    const current = snapshot.value;
    if (current.state !== 'ready-to-restart' || current.attemptId === '') return;
    const key = `${current.attemptId}:${current.revision}`;
    if (automatic && promptedRestarts.has(key)) return;
    promptedRestarts.add(key);
    restartAttemptId = current.attemptId;
    if (restartOpen.value) return;
    restartOpener = connectedElement(document.activeElement);
    restartOpen.value = true;
    restartFocusRequest.value += 1;
  }

  function applySnapshot(value: DesktopApplicationUpdateSnapshot, prompt = true): void {
    const projected = projectedSnapshot(value, snapshot.value);
    if (projected === null || projected.revision < snapshot.value.revision) return;
    if (projected.revision === snapshot.value.revision
      && (projected.attemptId !== snapshot.value.attemptId
        || projected.state !== snapshot.value.state)) return;

    snapshot.value = projected;
    commandError.value = '';
    statusOverride.value = '';
    const offerAvailable = projected.state === 'available'
      && !suppressedAttempts.has(projected.attemptId);
    const restartAvailable = projected.state === 'ready-to-restart';
    if (!offerAvailable) closeOffer();
    if (!restartAvailable) closeRestart();
    if (!prompt) return;
    if (offerAvailable) openOffer(true);
    else if (restartAvailable) openRestart(true);
  }

  async function resolveOffer(decision: 'accept' | 'defer'): Promise<void> {
    const current = snapshot.value;
    if (offerPending.value || current.attemptId === '' || current.attemptId !== offerAttemptId) return;
    offerPending.value = true;
    const result = await port.resolveApplicationUpdateOffer({
      attemptId: current.attemptId,
      decision,
    });
    if (!active) return;
    offerPending.value = false;
    const next = resultSnapshot(result);
    if (result.ok !== true) {
      if (next !== null) applySnapshot(next, false);
      commandError.value = boundedText(result.error, 'Не удалось сохранить решение об обновлении.');
      offerFocusRequest.value += 1;
      return;
    }

    suppressedAttempts.add(current.attemptId);
    closeOffer();
    if (next !== null
      && (next.revision > current.revision || next.state !== current.state)) {
      applySnapshot(next, false);
    } else if (decision === 'defer') {
      snapshot.value = Object.freeze({ ...current, state: 'deferred' });
    } else {
      statusOverride.value = 'ПОДГОТОВКА ОБНОВЛЕНИЯ ЗАПРОШЕНА…';
    }
  }

  async function resolveRestart(decision: 'postpone' | 'restart'): Promise<void> {
    const current = snapshot.value;
    if (restartPending.value || current.attemptId === '' || current.attemptId !== restartAttemptId) return;
    restartPending.value = true;
    const result = await port.resolveApplicationUpdateRestart({
      attemptId: current.attemptId,
      decision,
    });
    if (!active) return;
    restartPending.value = false;
    const next = resultSnapshot(result);
    if (result.ok !== true) {
      if (next !== null) applySnapshot(next, false);
      commandError.value = boundedText(result.error, 'Не удалось сохранить решение о перезапуске.');
      restartFocusRequest.value += 1;
      return;
    }

    closeRestart();
    if (next !== null) applySnapshot(next, false);
  }

  const release = port.onApplicationUpdateStatus(value => applySnapshot(value));
  onUnmounted(() => {
    active = false;
    focusGeneration += 1;
    release();
    closeOffer(false);
    closeRestart(false);
    promptedOffers.clear();
    promptedRestarts.clear();
    suppressedAttempts.clear();
  });

  const silent = computed(() => SILENT_STATES.has(snapshot.value.state));
  const failure = computed(() => commandError.value
    || (snapshot.value.state === 'failed' ? failureText(snapshot.value) : ''));
  const showButton = computed(() => {
    const current = snapshot.value;
    const canOpen = (current.state === 'available' && !suppressedAttempts.has(current.attemptId))
      || current.state === 'ready-to-restart';
    return canOpen && !offerOpen.value && !restartOpen.value;
  });

  return {
    snapshot: readonly(snapshot),
    offerOpen: readonly(offerOpen),
    restartOpen: readonly(restartOpen),
    offerPending: readonly(offerPending),
    restartPending: readonly(restartPending),
    offerFocusRequest: readonly(offerFocusRequest),
    restartFocusRequest: readonly(restartFocusRequest),
    silent,
    failure,
    showButton,
    statusText: computed(() => statusOverride.value || STATUS_LABELS[snapshot.value.state] || ''),
    showCurrent: () => {
      if (snapshot.value.state === 'ready-to-restart') openRestart();
      else openOffer();
    },
    accept: () => resolveOffer('accept'),
    defer: () => resolveOffer('defer'),
    restart: () => resolveRestart('restart'),
    postpone: () => resolveRestart('postpone'),
  };
}
