import { onScopeDispose, readonly, shallowRef, type DeepReadonly, type Ref } from 'vue';

export type TypewriterRevealPhase = 'cancelled' | 'complete' | 'idle' | 'revealing';

export interface TypewriterRevealState {
  readonly identity: string;
  readonly phase: TypewriterRevealPhase;
  readonly total: number;
  readonly visible: number;
}

export interface TypewriterClock {
  clearTimeout(handle: number): void;
  now(): number;
  setTimeout(callback: () => void, delay: number): number;
}

export interface TypewriterRevealOptions {
  readonly clock?: TypewriterClock;
  readonly interval?: number;
  readonly onCancel?: (identity: string) => void;
  readonly onComplete?: (identity: string) => void;
  readonly onCue?: (index: number, identity: string) => void;
  readonly target?: EventTarget;
}

export interface TypewriterRevealController {
  readonly state: Readonly<TypewriterRevealState>;
  cancel(): boolean;
  complete(): boolean;
  dispose(): void;
  start(identity: string, total: number, animate?: boolean): boolean;
}

const defaultClock: TypewriterClock = Object.freeze({
  clearTimeout: (handle: number) => globalThis.clearTimeout(handle),
  now: () => performance.now(),
  setTimeout: (callback: () => void, delay: number) => globalThis.setTimeout(callback, delay),
});

function physicalKey(event: KeyboardEvent): string {
  return event.code || event.key || 'Unidentified';
}

function consume(event: KeyboardEvent): void {
  event.preventDefault();
  event.stopImmediatePropagation();
}

export function createTypewriterRevealController(
  options: TypewriterRevealOptions = {},
  onState?: (state: Readonly<TypewriterRevealState>) => void,
): TypewriterRevealController {
  const clock = options.clock ?? defaultClock;
  const interval = options.interval ?? 40;
  const target = options.target ?? document;
  if (!Number.isFinite(interval) || interval <= 0) throw new RangeError('typewriter interval must be positive');
  let completedIdentity = '';
  let consumedKey: string | null = null;
  let disposed = false;
  let generation = 0;
  let startedAt = 0;
  let state: Readonly<TypewriterRevealState> = Object.freeze({ identity: '', phase: 'idle', total: 0, visible: 0 });
  let timer: number | null = null;

  const publish = (next: Readonly<TypewriterRevealState>): void => {
    state = Object.freeze(next);
    onState?.(state);
  };
  const clearTimer = (): void => {
    if (timer === null) return;
    clock.clearTimeout(timer);
    timer = null;
  };
  const settle = (phase: 'cancelled' | 'complete'): boolean => {
    if (state.phase !== 'revealing') return false;
    clearTimer();
    generation += 1;
    publish(Object.freeze({
      ...state,
      phase,
      visible: phase === 'complete' ? state.total : state.visible,
    }));
    if (phase === 'complete') {
      completedIdentity = state.identity;
      options.onComplete?.(state.identity);
    } else {
      options.onCancel?.(state.identity);
    }
    return true;
  };
  const schedule = (activeGeneration: number): void => {
    const targetTime = startedAt + state.visible * interval;
    timer = clock.setTimeout(() => {
      timer = null;
      if (disposed || generation !== activeGeneration || state.phase !== 'revealing') return;
      const index = state.visible;
      publish(Object.freeze({ ...state, visible: index + 1 }));
      options.onCue?.(index, state.identity);
      if (state.visible >= state.total) settle('complete');
      else schedule(activeGeneration);
    }, Math.max(0, targetTime - clock.now()));
  };
  const keydown = (event: Event): void => {
    if (!(event instanceof KeyboardEvent)) return;
    const key = physicalKey(event);
    if (consumedKey !== null && event.repeat && key === consumedKey) {
      consume(event);
      return;
    }
    if (state.phase !== 'revealing') return;
    if (!settle('complete')) return;
    consumedKey = key;
    consume(event);
  };
  const keyup = (event: Event): void => {
    if (event instanceof KeyboardEvent && physicalKey(event) === consumedKey) consumedKey = null;
  };
  target.addEventListener('keydown', keydown, { capture: true });
  target.addEventListener('keyup', keyup, { capture: true });

  return Object.freeze({
    get state() { return state; },
    cancel(): boolean { return settle('cancelled'); },
    complete(): boolean { return settle('complete'); },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      settle('cancelled');
      clearTimer();
      generation += 1;
      consumedKey = null;
      target.removeEventListener('keydown', keydown, { capture: true });
      target.removeEventListener('keyup', keyup, { capture: true });
    },
    start(identity: string, total: number, animate = true): boolean {
      if (disposed || identity === '' || !Number.isSafeInteger(total) || total < 0 ||
          (identity === state.identity && state.total === total && state.phase !== 'cancelled') ||
          (identity === completedIdentity && state.total === total)) return false;
      settle('cancelled');
      clearTimer();
      const activeGeneration = ++generation;
      startedAt = clock.now();
      if (!animate || total === 0) {
        publish(Object.freeze({ identity, phase: 'complete', total, visible: total }));
        completedIdentity = identity;
        options.onComplete?.(identity);
        return true;
      }
      publish(Object.freeze({ identity, phase: 'revealing', total, visible: 1 }));
      options.onCue?.(0, identity);
      if (total === 1) settle('complete');
      else schedule(activeGeneration);
      return true;
    },
  });
}

export interface TypewriterRevealComposable {
  readonly state: DeepReadonly<Ref<Readonly<TypewriterRevealState>>>;
  cancel(): boolean;
  complete(): boolean;
  dispose(): void;
  start(identity: string, total: number, animate?: boolean): boolean;
}

export function useTypewriterReveal(options: TypewriterRevealOptions = {}): TypewriterRevealComposable {
  const state = shallowRef<Readonly<TypewriterRevealState>>(Object.freeze({
    identity: '', phase: 'idle', total: 0, visible: 0,
  }));
  const controller = createTypewriterRevealController(options, next => { state.value = next; });
  const dispose = (): void => controller.dispose();
  onScopeDispose(dispose, true);
  return Object.freeze({
    cancel: controller.cancel,
    complete: controller.complete,
    dispose,
    start: controller.start,
    state: readonly(state),
  });
}
