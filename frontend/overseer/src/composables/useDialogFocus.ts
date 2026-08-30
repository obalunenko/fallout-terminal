import { onUnmounted } from 'vue';

export interface DialogFocusController {
  readonly capture: (opener: Element | null) => void;
  readonly dispose: () => void;
  readonly focusInitial: (target: Element | null) => void;
  readonly restore: () => void;
}

type FocusScheduler = (callback: () => void) => void;

function focusable(value: Element | null): HTMLElement | null {
  return value instanceof HTMLElement ? value : null;
}

export function createDialogFocusController(
  schedule: FocusScheduler = queueMicrotask,
): DialogFocusController {
  let active = true;
  let generation = 0;
  let opener: HTMLElement | null = null;

  function scheduleFocus(target: HTMLElement | null): void {
    const scheduledGeneration = generation;
    schedule(() => {
      if (!active || generation !== scheduledGeneration || target?.isConnected !== true) return;
      target.focus();
    });
  }

  return Object.freeze({
    capture(value: Element | null): void {
      generation += 1;
      opener = focusable(value);
    },
    dispose(): void {
      if (!active) return;
      active = false;
      generation += 1;
      opener = null;
    },
    focusInitial(target: Element | null): void {
      scheduleFocus(focusable(target));
    },
    restore(): void {
      const target = opener;
      opener = null;
      scheduleFocus(target);
    },
  });
}

export function useDialogFocus(): DialogFocusController {
  const controller = createDialogFocusController();
  onUnmounted(controller.dispose);
  return controller;
}
