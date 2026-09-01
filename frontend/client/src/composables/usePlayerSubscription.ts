import { onScopeDispose, readonly, shallowRef, type DeepReadonly, type Ref } from 'vue';

import type { SubscriptionMessage } from '../../gen/fallout/terminal/player/v1/player_pb.js';
import type { PlayerRPCAdapter } from '../adapters/player-rpc.js';

export const PLAYER_RECONNECT_DELAY_MS = 3_000;

type SubscribeInput = Parameters<PlayerRPCAdapter['subscribe']>[0];
type TimerHandle = ReturnType<typeof globalThis.setTimeout>;

export type PlayerSubscriptionPhase = 'idle' | 'connecting' | 'ready' | 'reconnecting' | 'stopped';

export interface PlayerSubscriptionState {
  readonly error: string;
  readonly generation: number;
  readonly phase: PlayerSubscriptionPhase;
  readonly revision: number;
}

export interface PlayerSubscriptionScheduler {
  clearTimeout(handle: TimerHandle): void;
  setTimeout(callback: () => void, delay: number): TimerHandle;
}

export interface PlayerSubscriptionControllerOptions {
  readonly onMessage?: (message: SubscriptionMessage) => void;
  readonly onState?: (state: Readonly<PlayerSubscriptionState>) => void;
  readonly reconnectDelayMilliseconds?: number;
  readonly reconnectInput?: () => SubscribeInput;
  readonly rpc: PlayerRPCAdapter;
  readonly scheduler?: PlayerSubscriptionScheduler;
}

export interface PlayerSubscriptionController {
  readonly state: Readonly<PlayerSubscriptionState>;
  dispose(): void;
  start(input: SubscribeInput): Promise<void>;
}

const defaultScheduler: PlayerSubscriptionScheduler = Object.freeze({
  clearTimeout: (handle: TimerHandle) => globalThis.clearTimeout(handle),
  setTimeout: (callback: () => void, delay: number) => globalThis.setTimeout(callback, delay),
});

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function revisionNumber(value: bigint): number | null {
  if (value < 0n || value > BigInt(Number.MAX_SAFE_INTEGER)) return null;
  return Number(value);
}

export function createPlayerSubscriptionController(
  options: PlayerSubscriptionControllerOptions,
): PlayerSubscriptionController {
  const scheduler = options.scheduler ?? defaultScheduler;
  const reconnectDelay = options.reconnectDelayMilliseconds ?? PLAYER_RECONNECT_DELAY_MS;
  if (!Number.isSafeInteger(reconnectDelay) || reconnectDelay < 0) {
    throw new TypeError('reconnect delay must be a nonnegative safe integer');
  }

  let abortController: AbortController | null = null;
  let disposed = false;
  let generation = 0;
  let input: SubscribeInput | null = null;
  let iterator: AsyncIterator<SubscriptionMessage> | null = null;
  let reconnectTimer: TimerHandle | null = null;
  let state: Readonly<PlayerSubscriptionState> = Object.freeze({
    error: '',
    generation: 0,
    phase: 'idle',
    revision: 0,
  });

  const publish = (patch: Partial<PlayerSubscriptionState>): void => {
    state = Object.freeze({ ...state, ...patch });
    options.onState?.(state);
  };

  const clearReconnect = (): void => {
    if (reconnectTimer === null) return;
    scheduler.clearTimeout(reconnectTimer);
    reconnectTimer = null;
  };

  const cancelStream = (): void => {
    abortController?.abort();
    abortController = null;
    const current = iterator;
    iterator = null;
    if (current?.return !== undefined) {
      void Promise.resolve(current.return()).catch(() => undefined);
    }
  };

  const currentGeneration = (candidate: number, controller: AbortController): boolean =>
    !disposed && generation === candidate && !controller.signal.aborted;

  const scheduleReconnect = (message: string): void => {
    if (disposed || input === null) return;
    clearReconnect();
    publish({ error: message, phase: 'reconnecting' });
    reconnectTimer = scheduler.setTimeout(() => {
      reconnectTimer = null;
      void connect().catch(() => undefined);
    }, reconnectDelay);
  };

  const acceptSnapshot = (message: SubscriptionMessage): number => {
    if (message.payload.case !== 'snapshot') {
      throw new Error('first subscription value is not a complete snapshot');
    }
    const revision = revisionNumber(message.payload.value.revision);
    if (revision === null) throw new Error('snapshot revision is invalid');
    return revision;
  };

  const acceptDelta = (message: SubscriptionMessage): boolean => {
    if (message.payload.case === 'snapshot' || message.payload.case === undefined) return false;
    if (message.payload.case === 'presentationUplinkResult') return true;
    const revision = revisionNumber(message.payload.value.revision);
    if (revision === null || revision <= state.revision) return false;
    publish({ revision });
    return true;
  };

  const drain = async (
    activeIterator: AsyncIterator<SubscriptionMessage>,
    controller: AbortController,
    activeGeneration: number,
  ): Promise<void> => {
    try {
      for (;;) {
        const next = await activeIterator.next();
        if (!currentGeneration(activeGeneration, controller)) return;
        if (next.done) {
          scheduleReconnect('subscription stream ended');
          return;
        }
        if (acceptDelta(next.value)) options.onMessage?.(next.value);
      }
    } catch (error) {
      if (currentGeneration(activeGeneration, controller)) scheduleReconnect(errorMessage(error));
    }
  };

  const connect = async (): Promise<void> => {
    if (disposed || input === null) return;
    clearReconnect();
    cancelStream();
    const activeGeneration = ++generation;
    const controller = new AbortController();
    abortController = controller;
    publish({ error: '', generation: activeGeneration, phase: 'connecting', revision: 0 });

    try {
      const activeIterator = options.rpc.subscribe(
        options.reconnectInput?.() ?? input,
        { signal: controller.signal },
      )[Symbol.asyncIterator]();
      iterator = activeIterator;
      const first = await activeIterator.next();
      if (!currentGeneration(activeGeneration, controller)) return;
      if (first.done) throw new Error('subscription ended before the complete snapshot');
      const revision = acceptSnapshot(first.value);
      publish({ error: '', phase: 'ready', revision });
      options.onMessage?.(first.value);
      void drain(activeIterator, controller, activeGeneration);
    } catch (error) {
      if (currentGeneration(activeGeneration, controller)) scheduleReconnect(errorMessage(error));
      throw error;
    }
  };

  return Object.freeze({
    get state() { return state; },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      generation += 1;
      input = null;
      clearReconnect();
      cancelStream();
      publish({ error: '', generation, phase: 'stopped', revision: 0 });
    },
    async start(nextInput: SubscribeInput): Promise<void> {
      if (disposed) throw new Error('subscription controller is stopped');
      input = nextInput;
      await connect();
    },
  });
}

export interface PlayerSubscriptionComposable {
  readonly state: DeepReadonly<Ref<Readonly<PlayerSubscriptionState>>>;
  dispose(): void;
  start(input: SubscribeInput): Promise<void>;
}

export function usePlayerSubscription(
  options: Omit<PlayerSubscriptionControllerOptions, 'onState'>,
): PlayerSubscriptionComposable {
  const state = shallowRef<Readonly<PlayerSubscriptionState>>(Object.freeze({
    error: '',
    generation: 0,
    phase: 'idle',
    revision: 0,
  }));
  const controller = createPlayerSubscriptionController({
    ...options,
    onState: next => { state.value = next; },
  });
  onScopeDispose(controller.dispose, true);
  return Object.freeze({
    dispose: controller.dispose,
    start: controller.start,
    state: readonly(state),
  });
}
