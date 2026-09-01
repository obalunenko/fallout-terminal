import { create } from '@bufbuild/protobuf';
import { onScopeDispose } from 'vue';

import {
  SetPresentationRequestSchema,
  type ActionResult,
  type PresentationIntent,
  type PresentationUplinkResult,
} from '../../gen/fallout/terminal/player/v1/player_pb.js';
import {
  createPresentationIntent,
  createPresentationOpen,
  validatePresentationResult,
  type PresentationUplinkTransport,
} from '../adapters/presentation-uplink-transport.js';
import { createPresentationQueue, type PresentationQueue } from './usePresentationQueue.js';

export interface PresentationUplinkClock {
  clearTimeout(handle: number): void;
  setTimeout(callback: () => void, delay: number): number;
}

export interface PresentationUplinkOptions {
  readonly authorize: (contextKey: string) => boolean;
  readonly clientInstanceID: string;
  readonly clock?: PresentationUplinkClock;
  readonly onResult: (result: ActionResult) => void;
  readonly recognitionHandle: () => string;
  readonly retryMilliseconds?: number;
  readonly transport: PresentationUplinkTransport;
}

export interface PresentationUplinkController {
  readonly active: boolean;
  readonly generation: number;
  apply(result: PresentationUplinkResult): boolean;
  dispose(): void;
  offer(intent: PresentationIntent): boolean;
  start(contextKey: string): boolean;
  stop(): boolean;
}

const defaultClock: PresentationUplinkClock = Object.freeze({
  clearTimeout: (handle: number) => globalThis.clearTimeout(handle),
  setTimeout: (callback: () => void, delay: number) => globalThis.setTimeout(callback, delay),
});

function validFallbackResult(result: ActionResult, requestID: string): boolean {
  return result.requestId === requestID && result.revision >= 0n &&
    result.revision <= BigInt(Number.MAX_SAFE_INTEGER);
}

export function createPresentationUplinkController(options: PresentationUplinkOptions): PresentationUplinkController {
  const clock = options.clock ?? defaultClock;
  const retryMilliseconds = options.retryMilliseconds ?? 1_500;
  let contextKey = '';
  let disposed = false;
  let fallbackController: AbortController | null = null;
  let generation = 0;
  let pending: PresentationIntent | null = null;
  let queue: PresentationQueue<ReturnType<typeof createPresentationOpen>> | null = null;
  let ready = false;
  let resultTimer: number | null = null;
  let streamController: AbortController | null = null;
  let streamRetryTimer: number | null = null;
  const requestIDs = new Set<string>();

  const clearTimer = (): void => {
    if (resultTimer === null) return;
    clock.clearTimeout(resultTimer);
    resultTimer = null;
  };
  const closeStream = (): void => {
    clearTimer();
    streamController?.abort();
    streamController = null;
    queue?.close();
    queue = null;
    ready = false;
  };
  const clearStreamRetry = (): void => {
    if (streamRetryTimer === null) return;
    clock.clearTimeout(streamRetryTimer);
    streamRetryTimer = null;
  };
  function scheduleStreamRecovery(expectedGeneration: number): void {
    if (!options.transport.streaming || streamRetryTimer !== null) return;
    const retry = (): void => {
      streamRetryTimer = null;
      if (disposed || expectedGeneration !== generation || contextKey === '') return;
      if (fallbackController !== null) {
        streamRetryTimer = clock.setTimeout(retry, retryMilliseconds);
        return;
      }
      closeStream();
      generation += 1;
      openStream();
    };
    streamRetryTimer = clock.setTimeout(retry, retryMilliseconds);
  }
  const openStream = (): void => {
    const activeGeneration = generation;
    const controller = new AbortController();
    streamController = controller;
    queue = createPresentationQueue({ authorize: key => !disposed && key === contextKey && options.authorize(key) });
    queue.offer(createPresentationOpen(options.clientInstanceID, generation, options.recognitionHandle()), contextKey);
    armFallback();
    const recover = (): void => {
      if (disposed || controller.signal.aborted || activeGeneration !== generation) return;
      fallback();
      clearStreamRetry();
      scheduleStreamRecovery(activeGeneration);
    };
    void options.transport.open(queue, controller.signal).then(recover).catch(recover);
  };
  const fallback = (): void => {
    const intent = pending;
    if (disposed || fallbackController !== null || intent === null || !options.authorize(intent.contextKey) ||
        intent.contextKey !== contextKey) return;
    closeStream();
    const controller = new AbortController();
    fallbackController = controller;
    const activeGeneration = generation;
    const request = create(SetPresentationRequestSchema, {
      broadcastId: intent.broadcastId,
      contextKey: intent.contextKey,
      presentation: intent.presentation,
      recognitionHandle: intent.recognitionHandle,
      requestId: intent.requestId,
      terminalId: intent.terminalId,
    });
    void options.transport.fallback(request, controller.signal).then(result => {
      if (disposed || controller.signal.aborted || activeGeneration !== generation || pending?.requestId !== intent.requestId ||
          !validFallbackResult(result, intent.requestId)) return;
      pending = null;
      requestIDs.delete(intent.requestId);
      options.onResult(result);
    }).catch(() => undefined).finally(() => {
      if (fallbackController !== controller) return;
      fallbackController = null;
      if (pending !== null) {
        fallback();
        return;
      }
      scheduleStreamRecovery(activeGeneration);
    });
  };
  const armFallback = (): void => {
    clearTimer();
    resultTimer = clock.setTimeout(() => {
      resultTimer = null;
      fallback();
    }, retryMilliseconds);
  };
  const dispatchPending = (): void => {
    if (!ready || queue === null || pending === null) return;
    if (!queue.offer(createPresentationIntent(pending), contextKey)) {
      fallback();
      return;
    }
    armFallback();
  };

  return Object.freeze({
    get active() { return !disposed && contextKey !== ''; },
    get generation() { return generation; },
    apply(result: PresentationUplinkResult): boolean {
      if (disposed || contextKey === '') return false;
      const validated = validatePresentationResult(result, {
        clientInstanceID: options.clientInstanceID,
        generation,
        requestIDs,
      });
      if (validated === null) return false;
      if (validated.kind === 'ready') {
        ready = true;
        clearTimer();
        dispatchPending();
        return true;
      }
      if (pending?.requestId !== validated.action.requestId) return false;
      clearTimer();
      pending = null;
      requestIDs.delete(validated.action.requestId);
      options.onResult(validated.action);
      return true;
    },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      generation += 1;
      clearStreamRetry();
      closeStream();
      fallbackController?.abort();
      fallbackController = null;
      pending = null;
      requestIDs.clear();
      contextKey = '';
    },
    offer(intent: PresentationIntent): boolean {
      if (disposed || intent.contextKey !== contextKey || !options.authorize(contextKey) || intent.requestId === '') return false;
      if (pending !== null) requestIDs.delete(pending.requestId);
      pending = intent;
      requestIDs.add(intent.requestId);
      if (!options.transport.streaming) fallback();
      else dispatchPending();
      return true;
    },
    start(nextContextKey: string): boolean {
      if (disposed || nextContextKey === '' || !options.authorize(nextContextKey)) return false;
      if (options.transport.streaming && contextKey.startsWith('selection:') && streamController !== null) {
        contextKey = nextContextKey;
        return true;
      }
      clearStreamRetry();
      closeStream();
      fallbackController?.abort();
      fallbackController = null;
      pending = null;
      requestIDs.clear();
      contextKey = nextContextKey;
      generation += 1;
      if (!options.transport.streaming) return true;
      openStream();
      return true;
    },
    stop(): boolean {
      if (disposed || contextKey === '') return false;
      generation += 1;
      clearStreamRetry();
      closeStream();
      fallbackController?.abort();
      fallbackController = null;
      pending = null;
      requestIDs.clear();
      contextKey = '';
      return true;
    },
  });
}

export function usePresentationUplink(options: PresentationUplinkOptions): PresentationUplinkController {
  const controller = createPresentationUplinkController(options);
  onScopeDispose(controller.dispose, true);
  return controller;
}
