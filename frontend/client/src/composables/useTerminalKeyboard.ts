import { onScopeDispose } from 'vue';

export type TerminalKeyboardMode = 'command' | 'entry' | 'hacking' | 'menu';

export interface TerminalKeyboardState {
  readonly blocked: boolean;
  readonly contextKey: string;
  readonly hackingComplete: boolean;
  readonly menuCount: number;
  readonly menuIndex: number;
  readonly mode: TerminalKeyboardMode;
  readonly pageCount: number;
  readonly pageIndex: number;
  readonly typed: string;
}

export interface TerminalKeyboardOptions {
  readonly authorize: (contextKey: string) => boolean;
  readonly contextKey: () => string;
  readonly onActivate: () => void;
  readonly onBack: () => void;
  readonly onMenuIndex: (index: number) => void;
  readonly onPageIndex: (index: number) => void;
  readonly onTyped: (value: string) => void;
  readonly state: () => Readonly<TerminalKeyboardState> | null;
  readonly target?: Pick<Document, 'addEventListener' | 'removeEventListener'>;
}

export interface TerminalKeyboardController {
  dispose(): void;
}

const controlledKeys = new Set([
  'ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight', 'PageUp', 'PageDown',
  'Home', 'End', 'Enter', 'Escape', 'Backspace',
]);

function isEditable(target: EventTarget | null): boolean {
  return target instanceof HTMLElement &&
    (target.isContentEditable || target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement ||
      target instanceof HTMLSelectElement);
}

function controlled(event: KeyboardEvent): boolean {
  return controlledKeys.has(event.key) || event.key.length === 1;
}

function consume(event: KeyboardEvent): void {
  event.preventDefault();
}

export function createTerminalKeyboardController(options: TerminalKeyboardOptions): TerminalKeyboardController {
  const target = options.target ?? document;
  let disposed = false;
  const keydown = (event: Event): void => {
    if (!(event instanceof KeyboardEvent) || event.isComposing || event.metaKey || event.ctrlKey || event.altKey ||
        isEditable(event.target)) return;
    const state = options.state();
    if (state === null || !controlled(event)) return;
    const currentContext = options.contextKey();
    if (state.contextKey === '' || state.contextKey !== currentContext || !options.authorize(currentContext) || state.blocked) {
      consume(event);
      return;
    }

    if (state.mode === 'hacking') {
      if (state.hackingComplete) return;
      if (event.key === 'Enter' || event.key === 'Escape') options.onTyped('');
      else if (event.key === 'Backspace') options.onTyped(state.typed.slice(0, -1));
      else if (event.key.length === 1 && state.typed.length < 24) options.onTyped(`${state.typed}${event.key}`);
      else return;
      consume(event);
      return;
    }

    if (state.mode === 'entry' || state.mode === 'command') {
      let page = state.pageIndex;
      if (event.key === 'ArrowLeft' || event.key === 'PageUp') page -= 1;
      else if (event.key === 'ArrowRight' || event.key === 'PageDown') page += 1;
      else if (event.key === 'Home') page = 0;
      else if (event.key === 'End') page = state.pageCount - 1;
      else if (event.key === 'Enter' || event.key === 'Escape' || event.key === 'Backspace') {
        options.onBack();
        consume(event);
        return;
      } else return;
      options.onPageIndex(Math.max(0, Math.min(Math.max(0, state.pageCount - 1), page)));
      consume(event);
      return;
    }

    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      const delta = event.key === 'ArrowDown' ? 1 : -1;
      options.onMenuIndex(Math.max(0, Math.min(Math.max(0, state.menuCount - 1), state.menuIndex + delta)));
      consume(event);
    } else if (event.key === 'Enter') {
      options.onActivate();
      consume(event);
    } else if (event.key === 'Escape' || event.key === 'Backspace') {
      options.onBack();
      consume(event);
    }
  };
  target.addEventListener('keydown', keydown);

  return Object.freeze({
    dispose(): void {
      if (disposed) return;
      disposed = true;
      target.removeEventListener('keydown', keydown);
    },
  });
}

export function useTerminalKeyboard(options: TerminalKeyboardOptions): TerminalKeyboardController {
  const controller = createTerminalKeyboardController(options);
  onScopeDispose(controller.dispose, true);
  return controller;
}
