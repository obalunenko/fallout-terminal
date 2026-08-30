import type { Directive, DirectiveBinding } from 'vue';

import {
  createDialogFocusController,
  type DialogFocusController,
} from '../composables/useDialogFocus.js';

export interface DialogFocusBinding {
  readonly active: boolean;
  readonly initialFocus: () => Element | null;
  readonly onCancel?: () => void;
  readonly opener?: Element | null;
}

interface DialogFocusState {
  binding: DialogFocusBinding;
  readonly controller: DialogFocusController;
  readonly cancel: (event: Event) => void;
  readonly keydown: (event: KeyboardEvent) => void;
}

const states = new WeakMap<HTMLElement, DialogFocusState>();

function activate(state: DialogFocusState): void {
  const opener = state.binding.opener ?? document.activeElement;
  state.controller.capture(opener);
  state.controller.focusInitial(state.binding.initialFocus());
}

function mounted(element: HTMLElement, binding: DirectiveBinding<DialogFocusBinding>): void {
  const controller = createDialogFocusController();
  const state: DialogFocusState = {
    binding: binding.value,
    controller,
    cancel(event: Event): void {
      if (!state.binding.active) return;
      event.preventDefault();
      state.binding.onCancel?.();
    },
    keydown(event: KeyboardEvent): void {
      if (event.key !== 'Escape' || element instanceof HTMLDialogElement) return;
      state.cancel(event);
    },
  };
  states.set(element, state);
  element.addEventListener('cancel', state.cancel);
  element.addEventListener('keydown', state.keydown);
  if (state.binding.active) activate(state);
}

function updated(element: HTMLElement, binding: DirectiveBinding<DialogFocusBinding>): void {
  const state = states.get(element);
  if (state === undefined) return;
  const wasActive = state.binding.active;
  state.binding = binding.value;
  if (!wasActive && state.binding.active) activate(state);
  else if (wasActive && !state.binding.active) state.controller.restore();
}

function beforeUnmount(element: HTMLElement): void {
  const state = states.get(element);
  if (state === undefined) return;
  element.removeEventListener('cancel', state.cancel);
  element.removeEventListener('keydown', state.keydown);
  state.controller.dispose();
  states.delete(element);
}

export const dialogFocus: Directive<HTMLElement, DialogFocusBinding> = {
  beforeUnmount,
  mounted,
  updated,
};
