import { onScopeDispose, shallowRef, type Ref } from 'vue';

import type {
  CompoundUpdate,
  PersonalizedSnapshot,
  PlayerState,
} from '../../gen/fallout/terminal/player/v1/player_pb.js';
import type {
  LiveTerminal,
  TerminalPresentation,
} from '../../gen/fallout/terminal/player/v1/terminal_pb.js';

export interface PlayerProjectionState {
  readonly liveTerminal: LiveTerminal | null;
  readonly playerState: PlayerState | null;
  readonly revision: number;
  readonly terminalPresentation: TerminalPresentation | null;
}

export interface PlayerProjectionController {
  readonly state: Readonly<PlayerProjectionState>;
  applySnapshot(snapshot: PersonalizedSnapshot): boolean;
  applyUpdate(update: CompoundUpdate): boolean;
  dispose(): void;
}

function revisionNumber(value: bigint): number | null {
  if (value < 0n || value > BigInt(Number.MAX_SAFE_INTEGER)) return null;
  return Number(value);
}

function liveTerminal(presentation: TerminalPresentation | null): LiveTerminal | null {
  return presentation?.presentation.case === 'liveTerminal'
    ? presentation.presentation.value
    : null;
}

function projectedState(
  revision: number,
  playerState: PlayerState,
  terminalPresentation: TerminalPresentation,
): Readonly<PlayerProjectionState> {
  return Object.freeze({
    liveTerminal: liveTerminal(terminalPresentation),
    playerState,
    revision,
    terminalPresentation,
  });
}

function mergeTerminalPresentation(
  current: TerminalPresentation,
  update: CompoundUpdate,
): TerminalPresentation {
  const presentation = update.terminalPresentation ?? current;
  if ((update.navigation === undefined && update.hacking === undefined) ||
      presentation.presentation.case !== 'liveTerminal') return presentation;
  const terminal = presentation.presentation.value;
  return {
    ...presentation,
    presentation: {
      case: 'liveTerminal',
      value: {
        ...terminal,
        navigation: update.navigation ?? terminal.navigation,
        hacking: update.hacking ?? terminal.hacking,
      },
    },
  };
}

export function createPlayerProjectionController(
  onState?: (state: Readonly<PlayerProjectionState>) => void,
): PlayerProjectionController {
  let disposed = false;
  let state: Readonly<PlayerProjectionState> = Object.freeze({
    liveTerminal: null,
    playerState: null,
    revision: 0,
    terminalPresentation: null,
  });

  const publish = (next: Readonly<PlayerProjectionState>): void => {
    state = next;
    onState?.(state);
  };

  return Object.freeze({
    get state() { return state; },
    applySnapshot(snapshot: PersonalizedSnapshot): boolean {
      if (disposed) return false;
      const revision = revisionNumber(snapshot.revision);
      if (revision === null || snapshot.playerState === undefined ||
          snapshot.terminalPresentation === undefined ||
          (state.playerState !== null && revision <= state.revision)) return false;
      publish(projectedState(revision, snapshot.playerState, snapshot.terminalPresentation));
      return true;
    },
    applyUpdate(update: CompoundUpdate): boolean {
      if (disposed || state.playerState === null || state.terminalPresentation === null) return false;
      const revision = revisionNumber(update.revision);
      if (revision === null || revision <= state.revision) return false;
      publish(projectedState(
        revision,
        update.playerState ?? state.playerState,
        mergeTerminalPresentation(state.terminalPresentation, update),
      ));
      return true;
    },
    dispose(): void {
      disposed = true;
    },
  });
}

export interface PlayerProjectionComposable {
  readonly state: Readonly<Ref<Readonly<PlayerProjectionState>>>;
  applySnapshot(snapshot: PersonalizedSnapshot): boolean;
  applyUpdate(update: CompoundUpdate): boolean;
  dispose(): void;
}

export function usePlayerProjection(): PlayerProjectionComposable {
  const state = shallowRef<Readonly<PlayerProjectionState>>(Object.freeze({
    liveTerminal: null,
    playerState: null,
    revision: 0,
    terminalPresentation: null,
  }));
  const controller = createPlayerProjectionController(next => { state.value = next; });
  onScopeDispose(controller.dispose, true);
  return Object.freeze({
    applySnapshot: controller.applySnapshot,
    applyUpdate: controller.applyUpdate,
    dispose: controller.dispose,
    state,
  });
}
