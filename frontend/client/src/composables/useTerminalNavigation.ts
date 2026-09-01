import { onScopeDispose, readonly, shallowRef, type DeepReadonly, type Ref } from 'vue';

import {
  NavigationMode,
  type NavigationState,
} from '../../gen/fallout/terminal/player/v1/navigation_pb.js';

export type TerminalNavigationMode = 'entry' | 'list';

export interface TerminalNavigationView {
  readonly commandNodeID: string | null;
  readonly mode: TerminalNavigationMode;
  readonly path: readonly string[];
  readonly pending: boolean;
  readonly revision: number;
  readonly viewEntryID: string | null;
}

export interface TerminalNavigationController {
  readonly state: Readonly<TerminalNavigationView> | null;
  apply(navigation: NavigationState, revision: number, pending: boolean): boolean;
  capturePendingFocus(opener: HTMLElement): boolean;
  dispose(): void;
}

function publicID(value: string): boolean {
  if (value === '' || value.trim() !== value || new TextEncoder().encode(value).byteLength > 256) return false;
  return !/[\u0000-\u0020\u007f-\uffff]/u.test(value);
}

function project(
  navigation: NavigationState,
  revision: number,
  pending: boolean,
): Readonly<TerminalNavigationView> | null {
  if (!Number.isSafeInteger(revision) || revision < 0 || navigation.path.length > 256 ||
      navigation.path.some(segment => !publicID(segment))) return null;
  if (navigation.mode === NavigationMode.LIST) {
    const commandNodeID = navigation.commandNodeId ?? null;
    if (navigation.viewEntryId !== undefined ||
        (commandNodeID !== null && !publicID(commandNodeID))) return null;
    return Object.freeze({
      commandNodeID,
      mode: 'list',
      path: Object.freeze([...navigation.path]),
      pending,
      revision,
      viewEntryID: null,
    });
  }
  if (navigation.mode !== NavigationMode.ENTRY) return null;
  const viewEntryID = navigation.viewEntryId ?? null;
  const commandNodeID = navigation.commandNodeId ?? null;
  if ((viewEntryID === null) === (commandNodeID === null) ||
      (viewEntryID !== null && !publicID(viewEntryID)) ||
      (commandNodeID !== null && !publicID(commandNodeID))) return null;
  return Object.freeze({
    commandNodeID,
    mode: 'entry',
    path: Object.freeze([...navigation.path]),
    pending,
    revision,
    viewEntryID,
  });
}

export function createTerminalNavigationController(
  onState?: (state: Readonly<TerminalNavigationView>) => void,
): TerminalNavigationController {
  let disposed = false;
  let focusGeneration = 0;
  let opener: HTMLElement | null = null;
  let state: Readonly<TerminalNavigationView> | null = null;
  const restoreFocus = (): void => {
    const target = opener;
    opener = null;
    const generation = ++focusGeneration;
    queueMicrotask(() => {
      if (!disposed && generation === focusGeneration && target?.isConnected === true) target.focus();
    });
  };
  return Object.freeze({
    get state() { return state; },
    apply(navigation: NavigationState, revision: number, pending: boolean): boolean {
      if (disposed || (state !== null && revision < state.revision)) return false;
      const next = project(navigation, revision, pending);
      if (next === null) return false;
      const completed = state?.pending === true && !pending;
      state = next;
      onState?.(state);
      if (completed) restoreFocus();
      return true;
    },
    capturePendingFocus(nextOpener: HTMLElement): boolean {
      if (disposed || state?.pending === true || !nextOpener.isConnected) return false;
      opener = nextOpener;
      return true;
    },
    dispose(): void {
      disposed = true;
      focusGeneration += 1;
      opener = null;
    },
  });
}

export interface TerminalNavigationComposable {
  readonly state: DeepReadonly<Ref<Readonly<TerminalNavigationView> | null>>;
  apply(navigation: NavigationState, revision: number, pending: boolean): boolean;
  capturePendingFocus(opener: HTMLElement): boolean;
  dispose(): void;
}

export function useTerminalNavigation(): TerminalNavigationComposable {
  const state = shallowRef<Readonly<TerminalNavigationView> | null>(null);
  const controller = createTerminalNavigationController(next => { state.value = next; });
  onScopeDispose(controller.dispose, true);
  return Object.freeze({
    apply: controller.apply,
    capturePendingFocus: controller.capturePendingFocus,
    dispose: controller.dispose,
    state: readonly(state),
  });
}
