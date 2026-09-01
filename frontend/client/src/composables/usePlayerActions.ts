import { onScopeDispose, readonly, shallowRef, type DeepReadonly, type Ref } from 'vue';

import { ActionReason, type ActionResult } from '../../gen/fallout/terminal/player/v1/player_pb.js';
import type { PlayerRPCAdapter } from '../adapters/player-rpc.js';

type SelectCharacterInput = Parameters<PlayerRPCAdapter['selectCharacter']>[0];
type NavigateInput = Parameters<PlayerRPCAdapter['navigate']>[0];
export type PlayerActionInput =
  | { readonly input: Omit<SelectCharacterInput, '$typeName' | 'requestId'>; readonly kind: 'selectCharacter' }
  | { readonly input: Omit<NavigateInput, '$typeName' | 'requestId'>; readonly kind: 'navigate' };

export interface PlayerActionPending {
  readonly acceptedRevision: number | null;
  readonly kind: PlayerActionInput['kind'];
  readonly requestID: string;
}

export interface PlayerActionsState {
  readonly error: string;
  readonly pending: PlayerActionPending | null;
  readonly revision: number;
}

export interface PlayerActionsControllerOptions {
  readonly authorize: () => boolean;
  readonly requestIDFactory?: () => string;
  readonly rpc: Pick<PlayerRPCAdapter, 'navigate' | 'selectCharacter'>;
}

export interface PlayerActionsController {
  readonly state: Readonly<PlayerActionsState>;
  applyRevision(revision: number): boolean;
  begin(action: PlayerActionInput): Promise<boolean>;
  dispose(): void;
}

function requestID(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

function acceptedRevision(result: ActionResult): number | null {
  if (result.revision < 0n || result.revision > BigInt(Number.MAX_SAFE_INTEGER)) return null;
  return Number(result.revision);
}

function actionReasonName(reason: ActionReason): string {
  return (ActionReason[reason] ?? 'action rejected').toLowerCase().replaceAll('_', '-');
}

export function createPlayerActionsController(
  options: PlayerActionsControllerOptions,
  onState?: (state: Readonly<PlayerActionsState>) => void,
): PlayerActionsController {
  let abortController: AbortController | null = null;
  let disposed = false;
  let generation = 0;
  let state: Readonly<PlayerActionsState> = Object.freeze({ error: '', pending: null, revision: 0 });
  const publish = (next: Readonly<PlayerActionsState>): void => {
    state = next;
    onState?.(state);
  };

  const currentPending = (): PlayerActionPending | null => state.pending;

  const begin = async (action: PlayerActionInput): Promise<boolean> => {
    if (disposed || state.pending !== null || !options.authorize()) return false;
    const id = (options.requestIDFactory ?? requestID)();
    const activeGeneration = ++generation;
    const controller = new AbortController();
    abortController = controller;
    publish(Object.freeze({
      ...state,
      error: '',
      pending: Object.freeze({ acceptedRevision: null, kind: action.kind, requestID: id }),
    }));
    try {
      const result = action.kind === 'selectCharacter'
        ? await options.rpc.selectCharacter({ ...action.input, requestId: id }, { signal: controller.signal })
        : await options.rpc.navigate({ ...action.input, requestId: id }, { signal: controller.signal });
      const pending = currentPending();
      if (disposed || controller.signal.aborted || activeGeneration !== generation ||
          pending?.requestID !== id || result.requestId !== id) return false;
      const revision = acceptedRevision(result);
      if (revision === null || result.accepted !== (result.reason === ActionReason.ACCEPTED)) {
        publish(Object.freeze({ ...state, error: 'invalid action result', pending: null }));
        return false;
      }
      if (!result.accepted) {
        publish(Object.freeze({ ...state, error: actionReasonName(result.reason), pending: null }));
        return false;
      }
      if (state.revision >= revision) {
        publish(Object.freeze({ ...state, pending: null }));
      } else {
        publish(Object.freeze({
          ...state,
          pending: Object.freeze({ ...pending, acceptedRevision: revision }),
        }));
      }
      return true;
    } catch (error) {
      if (!disposed && !controller.signal.aborted && activeGeneration === generation) {
        publish(Object.freeze({
          ...state,
          error: error instanceof Error ? error.message : String(error),
          pending: null,
        }));
      }
      return false;
    } finally {
      if (abortController === controller) abortController = null;
    }
  };

  return Object.freeze({
    get state() { return state; },
    applyRevision(revision: number): boolean {
      if (disposed || !Number.isSafeInteger(revision) || revision < state.revision) return false;
      const pending = state.pending?.acceptedRevision !== null &&
        state.pending?.acceptedRevision !== undefined && revision >= state.pending.acceptedRevision
        ? null
        : state.pending;
      publish(Object.freeze({ ...state, error: revision > state.revision ? '' : state.error, pending, revision }));
      return true;
    },
    begin,
    dispose(): void {
      if (disposed) return;
      disposed = true;
      generation += 1;
      abortController?.abort();
      abortController = null;
      publish(Object.freeze({ ...state, error: '', pending: null }));
    },
  });
}

export interface PlayerActionsComposable {
  readonly state: DeepReadonly<Ref<Readonly<PlayerActionsState>>>;
  applyRevision(revision: number): boolean;
  begin(action: PlayerActionInput): Promise<boolean>;
  dispose(): void;
}

export function usePlayerActions(options: PlayerActionsControllerOptions): PlayerActionsComposable {
  const state = shallowRef<Readonly<PlayerActionsState>>(Object.freeze({ error: '', pending: null, revision: 0 }));
  const controller = createPlayerActionsController(options, next => { state.value = next; });
  onScopeDispose(controller.dispose, true);
  return Object.freeze({
    applyRevision: controller.applyRevision,
    begin: controller.begin,
    dispose: controller.dispose,
    state: readonly(state),
  });
}
