export interface PresentationQueueOptions<T> {
  readonly authorize: (contextKey: string) => boolean;
  readonly onDiscard?: (value: T) => void;
}

export interface PresentationQueue<T> extends AsyncIterable<T> {
  readonly closed: boolean;
  readonly pending: boolean;
  close(): void;
  invalidate(contextKey: string): void;
  offer(value: T, contextKey: string): boolean;
}

export function createPresentationQueue<T>(options: PresentationQueueOptions<T>): PresentationQueue<T> {
  let closed = false;
  let contextKey = '';
  let pending: T | null = null;
  let waiter: ((result: IteratorResult<T>) => void) | null = null;

  const discardPending = (): void => {
    if (pending === null) return;
    options.onDiscard?.(pending);
    pending = null;
  };
  const close = (): void => {
    if (closed) return;
    closed = true;
    discardPending();
    waiter?.({ done: true, value: undefined });
    waiter = null;
  };

  return Object.freeze({
    get closed() { return closed; },
    get pending() { return pending !== null; },
    [Symbol.asyncIterator](): AsyncIterator<T> {
      return Object.freeze({
        next(): Promise<IteratorResult<T>> {
          if (pending !== null) {
            const value = pending;
            pending = null;
            return Promise.resolve({ done: false, value });
          }
          if (closed) return Promise.resolve({ done: true, value: undefined });
          if (waiter !== null) return Promise.reject(new Error('presentation queue supports one consumer'));
          return new Promise(resolve => { waiter = resolve; });
        },
        return(): Promise<IteratorResult<T>> {
          close();
          return Promise.resolve({ done: true, value: undefined });
        },
      });
    },
    close,
    invalidate(invalidContextKey: string): void {
      if (invalidContextKey !== contextKey) return;
      contextKey = '';
      discardPending();
    },
    offer(value: T, nextContextKey: string): boolean {
      if (closed || nextContextKey === '' || !options.authorize(nextContextKey)) return false;
      if (contextKey !== '' && contextKey !== nextContextKey) discardPending();
      contextKey = nextContextKey;
      if (waiter !== null) {
        const resolve = waiter;
        waiter = null;
        resolve({ done: false, value });
      } else {
        discardPending();
        pending = value;
      }
      return true;
    },
  });
}
