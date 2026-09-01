import { onScopeDispose, readonly, shallowRef, type DeepReadonly, type Ref } from 'vue';

import type { PlayerSubscriptionState } from './usePlayerSubscription.js';

export interface ConnectionOverlayState {
  readonly generation: number;
  readonly message: string;
  readonly visible: boolean;
}

export interface ConnectionOverlayController {
  readonly state: Readonly<ConnectionOverlayState>;
  apply(subscription: Readonly<PlayerSubscriptionState>): boolean;
  dispose(): void;
}

const initialState: Readonly<ConnectionOverlayState> = Object.freeze({
  generation: 0,
  message: 'УСТАНОВКА СВЯЗИ...',
  visible: true,
});

function project(subscription: Readonly<PlayerSubscriptionState>): Readonly<ConnectionOverlayState> {
  switch (subscription.phase) {
    case 'ready':
      return Object.freeze({ generation: subscription.generation, message: '', visible: false });
    case 'reconnecting':
      return Object.freeze({
        generation: subscription.generation,
        message: 'СВЯЗЬ ПОТЕРЯНА — ПЕРЕПОДКЛЮЧЕНИЕ...',
        visible: true,
      });
    case 'stopped':
      return Object.freeze({ generation: subscription.generation, message: 'УСТАНОВКА СВЯЗИ...', visible: false });
    case 'connecting':
    case 'idle':
      return Object.freeze({
        generation: subscription.generation,
        message: 'УСТАНОВКА СВЯЗИ...',
        visible: true,
      });
  }
}

export function createConnectionOverlayController(
  onState?: (state: Readonly<ConnectionOverlayState>) => void,
): ConnectionOverlayController {
  let disposed = false;
  let state = initialState;
  return Object.freeze({
    get state() { return state; },
    apply(subscription: Readonly<PlayerSubscriptionState>): boolean {
      if (disposed || subscription.generation < state.generation) return false;
      state = project(subscription);
      onState?.(state);
      return true;
    },
    dispose(): void {
      disposed = true;
    },
  });
}

export interface ConnectionOverlayComposable {
  readonly state: DeepReadonly<Ref<Readonly<ConnectionOverlayState>>>;
  apply(subscription: Readonly<PlayerSubscriptionState>): boolean;
  dispose(): void;
}

export function useConnectionOverlay(): ConnectionOverlayComposable {
  const state = shallowRef(initialState);
  const controller = createConnectionOverlayController(next => { state.value = next; });
  onScopeDispose(controller.dispose, true);
  return Object.freeze({
    apply: controller.apply,
    dispose: controller.dispose,
    state: readonly(state),
  });
}
