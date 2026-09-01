import { onScopeDispose, watch, type Ref } from 'vue';

import type { HackingAction, HackingPatternView } from './useHackingSession.js';

export interface HackingPointerTarget {
  readonly action: HackingAction;
  readonly cells: readonly HTMLElement[];
  readonly key: string;
  readonly text: string;
}

export interface HackingFocusIdentity {
  readonly character: string;
  readonly column: string;
  readonly offset: string;
  readonly row: string;
  readonly target: string;
}

export interface HackingPointerClock {
  clearTimeout(handle: number): void;
  setTimeout(callback: () => void, delay: number): number;
}

export interface HackingPointerOptions {
  readonly authorize: (contextKey: string) => boolean;
  readonly clock?: HackingPointerClock;
  readonly contextKey: () => string;
  readonly onActivate: (target: Readonly<HackingPointerTarget>, contextKey: string) => void;
  readonly onPreview: (target: Readonly<HackingPointerTarget> | null, contextKey: string) => void;
  readonly patterns: () => readonly HackingPatternView[];
}

export interface HackingPointerController {
  captureFocus(): Readonly<HackingFocusIdentity> | null;
  dispose(): void;
  restoreFocus(identity: Readonly<HackingFocusIdentity> | null): boolean;
  setRoot(root: HTMLElement | null): void;
}

const defaultClock: HackingPointerClock = Object.freeze({
  clearTimeout: (handle: number) => globalThis.clearTimeout(handle),
  setTimeout: (callback: () => void, delay: number) => globalThis.setTimeout(callback, delay),
});

function cellFromEvent(root: HTMLElement, event: Event): HTMLElement | null {
  const target = event.target;
  if (!(target instanceof Element)) return null;
  const cell = target.closest<HTMLElement>('.hcell');
  return cell !== null && root.contains(cell) ? cell : null;
}

function integerData(value: string | undefined): number | null {
  if (value === undefined || value === '' || !/^\d+$/u.test(value)) return null;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) ? parsed : null;
}

function matchingCells(root: HTMLElement, predicate: (cell: HTMLElement) => boolean): readonly HTMLElement[] {
  return Object.freeze([...root.querySelectorAll<HTMLElement>('.hcell')].filter(predicate));
}

function targetAt(
  root: HTMLElement,
  cell: HTMLElement,
  patterns: readonly HackingPatternView[],
): Readonly<HackingPointerTarget> | null {
  if (!cell.isConnected || !root.contains(cell)) return null;
  const row = integerData(cell.dataset.row);
  const offset = integerData(cell.dataset.offset);
  if (row !== null && offset !== null) {
    const pattern = patterns.find(candidate => !candidate.used && candidate.row === row &&
      offset >= candidate.start && offset <= candidate.end);
    if (pattern !== undefined) {
      const cells = matchingCells(root, candidate => {
        const candidateRow = integerData(candidate.dataset.row);
        const candidateOffset = integerData(candidate.dataset.offset);
        return candidateRow === pattern.row && candidateOffset !== null &&
          candidateOffset >= pattern.start && candidateOffset <= pattern.end;
      });
      if (cells.length === 0) return null;
      return Object.freeze({
        action: Object.freeze({ kind: 'pattern', patternID: pattern.id }),
        cells,
        key: `pattern:${pattern.id}`,
        text: cells.map(candidate => candidate.textContent ?? '').join(''),
      });
    }
  }

  const target = cell.dataset.target;
  if (target === undefined || target === '') return null;
  const grouped = matchingCells(root, candidate => candidate.dataset.target === target);
  if (cell.classList.contains('word')) {
    return Object.freeze({
      action: Object.freeze({ kind: 'word', wordID: target }),
      cells: grouped,
      key: `word:${target}`,
      text: grouped.map(candidate => candidate.textContent ?? '').join(''),
    });
  }
  const column = integerData(cell.dataset.column);
  const character = integerData(cell.dataset.character);
  if (column === null || character === null) return null;
  return Object.freeze({
    action: Object.freeze({ character, column, kind: 'filler' }),
    cells: Object.freeze([cell]),
    key: `filler:${column}:${character}`,
    text: cell.textContent ?? '',
  });
}

function sameTarget(left: Readonly<HackingPointerTarget> | null, right: Readonly<HackingPointerTarget> | null): boolean {
  return left?.key === right?.key;
}

function focusIdentity(cell: HTMLElement): Readonly<HackingFocusIdentity> {
  return Object.freeze({
    character: cell.dataset.character ?? '',
    column: cell.dataset.column ?? '',
    offset: cell.dataset.offset ?? '',
    row: cell.dataset.row ?? '',
    target: cell.dataset.target ?? '',
  });
}

function sameFocusIdentity(cell: HTMLElement, identity: Readonly<HackingFocusIdentity>): boolean {
  const current = focusIdentity(cell);
  return current.character === identity.character && current.column === identity.column &&
    current.offset === identity.offset && current.row === identity.row && current.target === identity.target;
}

export function createHackingPointerController(options: HackingPointerOptions): HackingPointerController {
  const clock = options.clock ?? defaultClock;
  let clearTimer: number | null = null;
  let disposed = false;
  let preview: Readonly<HackingPointerTarget> | null = null;
  let root: HTMLElement | null = null;

  const currentContext = (): string | null => {
    if (root === null) return null;
    const contextKey = options.contextKey();
    return contextKey !== '' && root.dataset.hackingContext === contextKey && options.authorize(contextKey)
      ? contextKey
      : null;
  };
  const cancelClear = (): void => {
    if (clearTimer === null) return;
    clock.clearTimeout(clearTimer);
    clearTimer = null;
  };
  const publish = (next: Readonly<HackingPointerTarget> | null): void => {
    const contextKey = currentContext();
    if (contextKey === null) return;
    if (sameTarget(preview, next)) return;
    preview = next;
    options.onPreview(next, contextKey);
  };
  const scheduleClear = (departing: Readonly<HackingPointerTarget>): void => {
    cancelClear();
    clearTimer = clock.setTimeout(() => {
      clearTimer = null;
      if (preview?.key === departing.key) publish(null);
    }, 0);
  };
  const enter = (event: Event): void => {
    if (root === null || currentContext() === null) return;
    const cell = cellFromEvent(root, event);
    if (cell === null) return;
    cancelClear();
    publish(targetAt(root, cell, options.patterns()));
  };
  const leave = (event: Event): void => {
    if (root === null) return;
    const cell = cellFromEvent(root, event);
    if (cell === null) return;
    const departing = targetAt(root, cell, options.patterns());
    if (departing === null) return;
    const related = event instanceof MouseEvent && event.relatedTarget instanceof Element
      ? event.relatedTarget.closest<HTMLElement>('.hcell')
      : null;
    const arriving = related !== null && root.contains(related)
      ? targetAt(root, related, options.patterns())
      : null;
    if (!sameTarget(departing, arriving)) scheduleClear(departing);
  };
  const click = (event: Event): void => {
    if (root === null) return;
    const contextKey = currentContext();
    const cell = cellFromEvent(root, event);
    if (contextKey === null || cell === null) return;
    const target = targetAt(root, cell, options.patterns());
    if (target !== null) options.onActivate(target, contextKey);
  };
  const removeListeners = (): void => {
    root?.removeEventListener('mouseover', enter);
    root?.removeEventListener('mouseout', leave);
    root?.removeEventListener('focusin', enter);
    root?.removeEventListener('focusout', leave);
    root?.removeEventListener('click', click);
  };

  return Object.freeze({
    captureFocus(): Readonly<HackingFocusIdentity> | null {
      if (root === null || !(document.activeElement instanceof HTMLElement) ||
          !root.contains(document.activeElement) || !document.activeElement.classList.contains('hcell')) return null;
      return focusIdentity(document.activeElement);
    },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      cancelClear();
      removeListeners();
      preview = null;
      root = null;
    },
    restoreFocus(identity: Readonly<HackingFocusIdentity> | null): boolean {
      if (root === null || identity === null || currentContext() === null) return false;
      const replacement = [...root.querySelectorAll<HTMLElement>('.hcell')]
        .find(cell => sameFocusIdentity(cell, identity));
      if (replacement === undefined) return false;
      replacement.focus({ preventScroll: true });
      return document.activeElement === replacement;
    },
    setRoot(next: HTMLElement | null): void {
      if (disposed || next === root) return;
      cancelClear();
      removeListeners();
      preview = null;
      root = next;
      root?.addEventListener('mouseover', enter);
      root?.addEventListener('mouseout', leave);
      root?.addEventListener('focusin', enter);
      root?.addEventListener('focusout', leave);
      root?.addEventListener('click', click);
    },
  });
}

export function useHackingPointer(root: Ref<HTMLElement | null>, options: HackingPointerOptions): HackingPointerController {
  const controller = createHackingPointerController(options);
  const stop = watch(root, next => controller.setRoot(next), { immediate: true });
  const dispose = (): void => {
    stop();
    controller.dispose();
  };
  onScopeDispose(dispose, true);
  return Object.freeze({
    captureFocus: controller.captureFocus,
    dispose,
    restoreFocus: controller.restoreFocus,
    setRoot: controller.setRoot,
  });
}
