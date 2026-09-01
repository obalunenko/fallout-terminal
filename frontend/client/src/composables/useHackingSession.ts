import { onScopeDispose, readonly, shallowRef, type DeepReadonly, type Ref } from 'vue';

import type { PublicHackState } from '../../gen/fallout/terminal/player/v1/hacking_pb.js';
import { ActionReason, type ActionResult } from '../../gen/fallout/terminal/player/v1/player_pb.js';
import type { PlayerRPCAdapter } from '../adapters/player-rpc.js';

export interface HackingWordView {
  readonly id: string;
  readonly length: number;
  readonly start: number;
}

export interface HackingColumnView {
  readonly addresses: readonly string[];
  readonly text: string;
  readonly words: readonly HackingWordView[];
}

export interface HackingPatternView {
  readonly end: number;
  readonly id: string;
  readonly row: number;
  readonly start: number;
  readonly used: boolean;
}

export interface HackingView {
  readonly attemptsLeft: number;
  readonly attemptsMax: number;
  readonly columns: readonly HackingColumnView[];
  readonly failed: boolean;
  readonly level: number;
  readonly log: readonly string[];
  readonly patterns: readonly HackingPatternView[];
  readonly solved: boolean;
  readonly wordLength: number;
}

export interface HackingPendingAction {
  readonly acceptedRevision: number | null;
  readonly contextKey: string;
  readonly requestID: string;
}

export interface HackingSessionState {
  readonly contextKey: string;
  readonly error: string;
  readonly hack: Readonly<HackingView>;
  readonly pending: HackingPendingAction | null;
  readonly revision: number;
}

export type HackingAction =
  | { readonly kind: 'pattern'; readonly patternID: string }
  | { readonly column: number; readonly character: number; readonly kind: 'filler' }
  | { readonly kind: 'word'; readonly wordID: string };

export interface HackingSessionControllerOptions {
  readonly authorize: (contextKey: string) => boolean;
  readonly broadcastID: () => string;
  readonly recognitionHandle: () => string;
  readonly requestIDFactory?: () => string;
  readonly rpc: Pick<PlayerRPCAdapter, 'activatePattern' | 'guess'>;
  readonly terminalID: () => string;
}

export interface HackingSessionController {
  readonly state: Readonly<HackingSessionState> | null;
  apply(hack: PublicHackState, revision: number, contextKey: string): boolean;
  begin(action: HackingAction, contextKey: string): Promise<boolean>;
  clear(revision: number): boolean;
  dispose(): void;
}

function publicID(value: string): boolean {
  return value !== '' && value.trim() === value && new TextEncoder().encode(value).byteLength <= 256 &&
    !/[\u0000-\u0020\u007f-\uffff]/u.test(value);
}

function safeText(value: string, maximumBytes: number): boolean {
  return new TextEncoder().encode(value).byteLength <= maximumBytes && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/u.test(value);
}

function project(hack: PublicHackState): Readonly<HackingView> | null {
  if (!Number.isInteger(hack.level) || hack.level < 0 || hack.level > 100 ||
      !Number.isInteger(hack.wordLength) || hack.wordLength <= 0 || hack.wordLength > 32 ||
      !Number.isInteger(hack.attemptsMax) || hack.attemptsMax <= 0 || hack.attemptsMax > 64 ||
      !Number.isInteger(hack.attemptsLeft) || hack.attemptsLeft < 0 || hack.attemptsLeft > hack.attemptsMax ||
      (hack.solved && hack.failed) || hack.columns.length === 0 || hack.columns.length > 2 ||
      hack.log.length > 128 || hack.log.some(line => !safeText(line, 1_024))) return null;

  const wordIDs = new Set<string>();
  const columns: HackingColumnView[] = [];
  for (const column of hack.columns) {
    if (!safeText(column.text, 512) || column.text.length > 192 || column.addresses.length > 64 ||
        column.addresses.some(address => !publicID(address))) return null;
    const words: HackingWordView[] = [];
    for (const word of column.words) {
      if (!publicID(word.id) || wordIDs.has(word.id) || !Number.isInteger(word.start) ||
          !Number.isInteger(word.length) || word.length !== hack.wordLength || word.start < 0 ||
          word.start + word.length > column.text.length) return null;
      wordIDs.add(word.id);
      words.push(Object.freeze({ id: word.id, length: word.length, start: word.start }));
    }
    columns.push(Object.freeze({
      addresses: Object.freeze([...column.addresses]),
      text: column.text,
      words: Object.freeze(words),
    }));
  }

  const patternIDs = new Set<string>();
  const patterns: HackingPatternView[] = [];
  const maximumRows = columns.reduce((total, column) => total + column.addresses.length, 0);
  for (const pattern of hack.patterns) {
    if (!publicID(pattern.patternId) || patternIDs.has(pattern.patternId) || !Number.isInteger(pattern.row) ||
        !Number.isInteger(pattern.start) || !Number.isInteger(pattern.end) || pattern.row < 0 ||
        pattern.row >= maximumRows || pattern.start < 0 || pattern.end < pattern.start || pattern.end >= 12) return null;
    patternIDs.add(pattern.patternId);
    patterns.push(Object.freeze({
      end: pattern.end,
      id: pattern.patternId,
      row: pattern.row,
      start: pattern.start,
      used: pattern.used,
    }));
  }

  return Object.freeze({
    attemptsLeft: hack.attemptsLeft,
    attemptsMax: hack.attemptsMax,
    columns: Object.freeze(columns),
    failed: hack.failed,
    level: hack.level,
    log: Object.freeze([...hack.log]),
    patterns: Object.freeze(patterns),
    solved: hack.solved,
    wordLength: hack.wordLength,
  });
}

function requestID(): string {
  return typeof globalThis.crypto?.randomUUID === 'function'
    ? globalThis.crypto.randomUUID()
    : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

function resultRevision(result: ActionResult): number | null {
  return result.revision >= 0n && result.revision <= BigInt(Number.MAX_SAFE_INTEGER)
    ? Number(result.revision)
    : null;
}

export function createHackingSessionController(
  options: HackingSessionControllerOptions,
  onState?: (state: Readonly<HackingSessionState> | null) => void,
): HackingSessionController {
  let abortController: AbortController | null = null;
  let disposed = false;
  let generation = 0;
  let state: Readonly<HackingSessionState> | null = null;
  const publish = (next: Readonly<HackingSessionState>): void => { state = next; onState?.(next); };
  const currentState = (): Readonly<HackingSessionState> | null => state;

  return Object.freeze({
    get state() { return state; },
    apply(hack: PublicHackState, revision: number, contextKey: string): boolean {
      if (disposed || !Number.isSafeInteger(revision) || revision < 0 || !publicID(contextKey) ||
          (state !== null && revision <= state.revision)) return false;
      const nextHack = project(hack);
      if (nextHack === null) return false;
      const pending = state?.pending?.acceptedRevision !== null &&
        state?.pending?.acceptedRevision !== undefined && revision >= state.pending.acceptedRevision
        ? null
        : state?.contextKey === contextKey ? state.pending : null;
      publish(Object.freeze({ contextKey, error: '', hack: nextHack, pending, revision }));
      return true;
    },
    async begin(action: HackingAction, contextKey: string): Promise<boolean> {
      const current = currentState();
      if (disposed || current === null || current.pending !== null || current.contextKey !== contextKey ||
          current.hack.solved || current.hack.failed || !options.authorize(contextKey)) return false;
      if (action.kind === 'word' && !current.hack.columns.some(column => column.words.some(word => word.id === action.wordID))) return false;
      if (action.kind === 'pattern' && !current.hack.patterns.some(pattern => pattern.id === action.patternID && !pattern.used)) return false;
      if (action.kind === 'filler') {
        const column = current.hack.columns.at(action.column);
        if (!Number.isInteger(action.column) || action.column < 0 || column === undefined ||
            !Number.isInteger(action.character) || action.character < 0 ||
            action.character >= column.text.length) return false;
      }

      const id = (options.requestIDFactory ?? requestID)();
      const activeGeneration = ++generation;
      const controller = new AbortController();
      abortController = controller;
      publish(Object.freeze({
        ...current,
        pending: Object.freeze({ acceptedRevision: null, contextKey, requestID: id }),
      }));
      try {
        const common = {
          broadcastId: options.broadcastID(),
          recognitionHandle: options.recognitionHandle(),
          requestId: id,
          terminalId: options.terminalID(),
        };
        const result = action.kind === 'pattern'
          ? await options.rpc.activatePattern({ ...common, patternId: action.patternID }, { signal: controller.signal })
          : await options.rpc.guess({
            ...common,
            target: action.kind === 'word'
              ? { case: 'wordId', value: action.wordID }
              : { case: 'filler', value: { character: action.character, column: action.column } },
          }, { signal: controller.signal });
        const active = currentState();
        if (disposed || controller.signal.aborted || activeGeneration !== generation ||
            active?.pending?.requestID !== id || result.requestId !== id) return false;
        const revision = resultRevision(result);
        if (revision === null || result.accepted !== (result.reason === ActionReason.ACCEPTED)) {
          publish(Object.freeze({ ...active, error: 'invalid action result', pending: null }));
          return false;
        }
        if (!result.accepted) {
          publish(Object.freeze({ ...active, error: ActionReason[result.reason] ?? 'action rejected', pending: null }));
          return false;
        }
        publish(Object.freeze({
          ...active,
          pending: active.revision >= revision
            ? null
            : Object.freeze({ ...active.pending, acceptedRevision: revision }),
        }));
        return true;
      } catch (error) {
        const active = currentState();
        if (!disposed && active !== null && activeGeneration === generation && !controller.signal.aborted) {
          publish(Object.freeze({
            ...active,
            error: error instanceof Error ? error.message : String(error),
            pending: null,
          }));
        }
        return false;
      } finally {
        if (abortController === controller) abortController = null;
      }
    },
    clear(revision: number): boolean {
      if (disposed || state === null || !Number.isSafeInteger(revision) || revision < state.revision) return false;
      generation += 1;
      abortController?.abort();
      abortController = null;
      state = null;
      onState?.(null);
      return true;
    },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      generation += 1;
      abortController?.abort();
      abortController = null;
    },
  });
}

export interface HackingSessionComposable {
  readonly state: DeepReadonly<Ref<Readonly<HackingSessionState> | null>>;
  apply(hack: PublicHackState, revision: number, contextKey: string): boolean;
  begin(action: HackingAction, contextKey: string): Promise<boolean>;
  clear(revision: number): boolean;
  dispose(): void;
}

export function useHackingSession(options: HackingSessionControllerOptions): HackingSessionComposable {
  const state = shallowRef<Readonly<HackingSessionState> | null>(null);
  const controller = createHackingSessionController(options, next => { state.value = next; });
  onScopeDispose(controller.dispose, true);
  return Object.freeze({
    apply: controller.apply,
    begin: controller.begin,
    clear: controller.clear,
    dispose: controller.dispose,
    state: readonly(state),
  });
}
