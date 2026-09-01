import { onScopeDispose, readonly, shallowRef, type DeepReadonly, type Ref } from 'vue';

import type { PlayerIdentityState } from './usePlayerIdentity.js';

export interface PlayerAuthorityInput {
  readonly activeTerminalID: string | null;
  readonly broadcastID: string | null;
  readonly contextKey: string;
  readonly identity: Readonly<PlayerIdentityState>;
  readonly presentedTerminalID: string | null;
}

export interface PlayerAuthorityState {
  readonly canControl: boolean;
  readonly contextKey: string;
  readonly observerReadOnly: boolean;
  readonly revision: number;
  readonly transientFeedback: string;
}

export interface PlayerAuthorityController {
  readonly state: Readonly<PlayerAuthorityState>;
  apply(input: PlayerAuthorityInput): boolean;
  clearContext(contextKey: string): boolean;
  dispose(): void;
  showTransientFeedback(contextKey: string, message: string): boolean;
}

const initialState: Readonly<PlayerAuthorityState> = Object.freeze({
  canControl: false,
  contextKey: '',
  observerReadOnly: false,
  revision: 0,
  transientFeedback: '',
});

function validContextKey(value: string): boolean {
  return new TextEncoder().encode(value).byteLength <= 512 && !/[\u0000-\u001f\u007f]/u.test(value);
}

function project(input: PlayerAuthorityInput, previous: Readonly<PlayerAuthorityState>): Readonly<PlayerAuthorityState> | null {
  if (!validContextKey(input.contextKey) || input.identity.revision < previous.revision) return null;
  const observerReadOnly = input.identity.role === 'observer';
  const terminalMatches = input.activeTerminalID !== null && input.presentedTerminalID !== null &&
    input.activeTerminalID === input.presentedTerminalID;
  const canControl = input.identity.role === 'active' && input.identity.phase === 'controlling' &&
    input.broadcastID !== null && terminalMatches;
  return Object.freeze({
    canControl,
    contextKey: input.contextKey,
    observerReadOnly,
    revision: input.identity.revision,
    transientFeedback: canControl && input.contextKey === previous.contextKey
      ? previous.transientFeedback
      : '',
  });
}

export function createPlayerAuthorityController(
  onState?: (state: Readonly<PlayerAuthorityState>) => void,
): PlayerAuthorityController {
  let disposed = false;
  let state = initialState;
  const publish = (next: Readonly<PlayerAuthorityState>): void => {
    state = next;
    onState?.(state);
  };
  return Object.freeze({
    get state() { return state; },
    apply(input: PlayerAuthorityInput): boolean {
      if (disposed) return false;
      const next = project(input, state);
      if (next === null) return false;
      publish(next);
      return true;
    },
    clearContext(contextKey: string): boolean {
      if (disposed || contextKey === '' || contextKey !== state.contextKey) return false;
      publish(Object.freeze({ ...state, contextKey: '', transientFeedback: '' }));
      return true;
    },
    dispose(): void { disposed = true; },
    showTransientFeedback(contextKey: string, message: string): boolean {
      if (disposed || !state.canControl || contextKey === '' || contextKey !== state.contextKey ||
          message.trim() === '' || !validContextKey(message)) return false;
      publish(Object.freeze({ ...state, transientFeedback: message }));
      return true;
    },
  });
}

export interface PlayerAuthorityComposable {
  readonly state: DeepReadonly<Ref<Readonly<PlayerAuthorityState>>>;
  apply(input: PlayerAuthorityInput): boolean;
  clearContext(contextKey: string): boolean;
  dispose(): void;
  showTransientFeedback(contextKey: string, message: string): boolean;
}

export function usePlayerAuthority(): PlayerAuthorityComposable {
  const state = shallowRef(initialState);
  const controller = createPlayerAuthorityController(next => { state.value = next; });
  onScopeDispose(controller.dispose, true);
  return Object.freeze({
    apply: controller.apply,
    clearContext: controller.clearContext,
    dispose: controller.dispose,
    showTransientFeedback: controller.showTransientFeedback,
    state: readonly(state),
  });
}
