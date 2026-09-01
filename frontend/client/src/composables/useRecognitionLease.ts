import { onScopeDispose } from 'vue';

import {
  PLAYER_SESSION_INIT_LOCK_NAME,
  RECOGNITION_RECORD_VERSION,
  type RecognitionCoordinationRecord,
  type RecognitionStorageAdapter,
} from '../adapters/recognition-storage.js';

export const PLAYER_SESSION_INIT_LEASE_MS = 5_000;
export const PLAYER_SESSION_INIT_RETRY_MS = 100;
export const PLAYER_SESSION_INIT_ELECTION_MS = 100;

type TimerHandle = ReturnType<typeof globalThis.setTimeout>;

export interface RecognitionLeaseScheduler {
  clearTimeout(handle: TimerHandle): void;
  now(): number;
  setTimeout(callback: () => void, delay: number): TimerHandle;
}

export interface RecognitionLockManager {
  request(name: string, signal: AbortSignal, callback: () => Promise<void>): Promise<void>;
}

export interface RecognitionLeaseControllerOptions {
  readonly electionMilliseconds?: number;
  readonly leaseMilliseconds?: number;
  readonly lockManager?: RecognitionLockManager | null;
  readonly owner?: string;
  readonly retryMilliseconds?: number;
  readonly scheduler?: RecognitionLeaseScheduler;
  readonly startSubscription: (recognitionHandle: string) => Promise<void>;
  readonly storage: RecognitionStorageAdapter;
  readonly tokenFactory?: () => string;
}

export interface RecognitionLeaseController {
  dispose(): void;
  start(): Promise<void>;
}

const defaultScheduler: RecognitionLeaseScheduler = Object.freeze({
  clearTimeout: (handle: TimerHandle) => globalThis.clearTimeout(handle),
  now: () => Date.now(),
  setTimeout: (callback: () => void, delay: number) => globalThis.setTimeout(callback, delay),
});

function pageLifetimeID(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

function browserLockManager(): RecognitionLockManager | null {
  if (typeof navigator === 'undefined' || navigator.locks?.request === undefined) return null;
  return Object.freeze({
    async request(name: string, signal: AbortSignal, callback: () => Promise<void>): Promise<void> {
      await navigator.locks.request(name, { signal }, async () => callback());
    },
  });
}

function checkedDuration(value: number, label: string): number {
  if (!Number.isSafeInteger(value) || value < 0) {
    throw new TypeError(`${label} must be a nonnegative safe integer`);
  }
  return value;
}

export function createRecognitionLeaseController(
  options: RecognitionLeaseControllerOptions,
): RecognitionLeaseController {
  const scheduler = options.scheduler ?? defaultScheduler;
  const leaseMilliseconds = checkedDuration(
    options.leaseMilliseconds ?? PLAYER_SESSION_INIT_LEASE_MS,
    'lease duration',
  );
  const retryMilliseconds = checkedDuration(
    options.retryMilliseconds ?? PLAYER_SESSION_INIT_RETRY_MS,
    'retry duration',
  );
  const electionMilliseconds = checkedDuration(
    options.electionMilliseconds ?? PLAYER_SESSION_INIT_ELECTION_MS,
    'election duration',
  );
  const lockManager = options.lockManager === undefined ? browserLockManager() : options.lockManager;
  const owner = options.owner ?? pageLifetimeID();
  const tokenFactory = options.tokenFactory ?? pageLifetimeID;
  const abortController = new AbortController();
  const timers = new Set<TimerHandle>();
  const wakeWaiters = new Set<() => void>();
  let activeToken = '';
  let disposed = false;
  let started = false;

  const wake = (): void => {
    for (const waiter of [...wakeWaiters]) waiter();
  };
  const unsubscribe = options.storage.subscribe(wake);

  const wait = (delay: number): Promise<void> => new Promise(resolve => {
    if (disposed) {
      resolve();
      return;
    }
    let timer: TimerHandle | null = null;
    const finish = (): void => {
      wakeWaiters.delete(finish);
      if (timer !== null) {
        scheduler.clearTimeout(timer);
        timers.delete(timer);
        timer = null;
      }
      resolve();
    };
    wakeWaiters.add(finish);
    timer = scheduler.setTimeout(finish, delay);
    timers.add(timer);
  });

  const coordinationRecord = (token: string): RecognitionCoordinationRecord => Object.freeze({
    expiresAt: scheduler.now() + leaseMilliseconds,
    owner,
    token,
    version: RECOGNITION_RECORD_VERSION,
  });

  const releaseOwnership = (): void => {
    if (activeToken === '') return;
    options.storage.removeContender(owner, activeToken);
    options.storage.removeLease(owner, activeToken);
    activeToken = '';
    wake();
  };

  const startWithStorageCoordination = async (): Promise<void> => {
    while (!disposed) {
      const handle = options.storage.readRecognitionHandle();
      if (handle !== null) {
        await options.startSubscription(handle);
        return;
      }

      const now = scheduler.now();
      const currentLease = options.storage.readLease();
      if (currentLease !== null && currentLease.owner !== owner && currentLease.expiresAt > now) {
        await wait(retryMilliseconds);
        continue;
      }

      activeToken = tokenFactory();
      const contenderWritten = options.storage.writeContender(coordinationRecord(activeToken));
      if (contenderWritten) await wait(electionMilliseconds);
      if (disposed) return;

      const handleAfterElection = options.storage.readRecognitionHandle();
      if (handleAfterElection !== null) {
        releaseOwnership();
        await options.startSubscription(handleAfterElection);
        return;
      }

      const leaseAfterElection = options.storage.readLease();
      const electedOwner = options.storage.listContenders(scheduler.now())[0]?.owner ?? owner;
      if ((leaseAfterElection !== null && leaseAfterElection.owner !== owner &&
          leaseAfterElection.expiresAt > scheduler.now()) || electedOwner !== owner) {
        releaseOwnership();
        await wait(retryMilliseconds);
        continue;
      }

      const leaseWritten = options.storage.writeLease(coordinationRecord(activeToken));
      const confirmedLease = options.storage.readLease();
      if (leaseWritten && (confirmedLease?.owner !== owner || confirmedLease.token !== activeToken)) {
        releaseOwnership();
        await wait(retryMilliseconds);
        continue;
      }

      options.storage.removeContender(owner, activeToken);
      try {
        await options.startSubscription('');
      } finally {
        releaseOwnership();
      }
      return;
    }
  };

  const start = async (): Promise<void> => {
    if (started) throw new Error('recognition lease controller already started');
    if (disposed) throw new Error('recognition lease controller is stopped');
    started = true;

    const handle = options.storage.readRecognitionHandle();
    if (handle !== null) {
      await options.startSubscription(handle);
      return;
    }

    if (lockManager !== null) {
      try {
        await lockManager.request(PLAYER_SESSION_INIT_LOCK_NAME, abortController.signal, async () => {
          if (disposed) return;
          await options.startSubscription(options.storage.readRecognitionHandle() ?? '');
        });
        return;
      } catch (error) {
        if (disposed || abortController.signal.aborted) return;
        void error;
      }
    }
    await startWithStorageCoordination();
  };

  return Object.freeze({
    dispose(): void {
      if (disposed) return;
      disposed = true;
      abortController.abort();
      unsubscribe();
      for (const timer of timers) scheduler.clearTimeout(timer);
      timers.clear();
      wake();
      releaseOwnership();
    },
    start,
  });
}

export function useRecognitionLease(
  options: RecognitionLeaseControllerOptions,
): RecognitionLeaseController {
  const controller = createRecognitionLeaseController(options);
  onScopeDispose(controller.dispose, true);
  return controller;
}
