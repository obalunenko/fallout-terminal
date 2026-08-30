import { createApp, type InjectionKey } from 'vue';

import App from './App.vue';
import type { DesktopRecord } from './models/overseer-view-state.js';
import type { DesktopPort } from './ports/desktop-port.js';
import type { DesktopUnsubscribe } from './ports/desktop-port.js';

export interface OverseerCoexistenceMessage extends DesktopRecord {
  readonly kind: string;
}

export interface OverseerCoexistenceBridge {
  readonly legacyToVue: (value: unknown) => boolean;
  readonly subscribeLegacyState: (
    listener: (message: OverseerCoexistenceMessage) => void,
  ) => DesktopUnsubscribe;
  readonly subscribeVueRequests: (
    listener: (message: OverseerCoexistenceMessage) => void,
  ) => DesktopUnsubscribe;
  readonly vueToLegacy: (value: unknown) => boolean;
}

export const overseerCoexistenceBridgeKey: InjectionKey<OverseerCoexistenceBridge> =
  Symbol('overseer-coexistence-bridge');

type CoexistenceListener = (message: OverseerCoexistenceMessage) => void;

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function coexistenceMessage(value: unknown): OverseerCoexistenceMessage | null {
  if (!isRecord(value)) return null;
  const copy: unknown = globalThis.structuredClone(value);
  if (!isRecord(copy) || typeof copy.kind !== 'string' || copy.kind.length === 0) return null;
  return Object.freeze({ ...copy, kind: copy.kind });
}

function subscribe(
  listeners: Set<CoexistenceListener>,
  listener: CoexistenceListener,
): DesktopUnsubscribe {
  listeners.add(listener);
  let active = true;
  return () => {
    if (!active) return;
    active = false;
    listeners.delete(listener);
  };
}

function deliver(listeners: ReadonlySet<CoexistenceListener>, value: unknown): boolean {
  const message = coexistenceMessage(value);
  if (message === null) return false;
  for (const listener of listeners) listener(message);
  return true;
}

export function createOverseerCoexistenceBridge(): OverseerCoexistenceBridge {
  const legacyStateListeners = new Set<CoexistenceListener>();
  const vueRequestListeners = new Set<CoexistenceListener>();
  return Object.freeze({
    legacyToVue: (value: unknown) => deliver(legacyStateListeners, value),
    subscribeLegacyState: (listener: CoexistenceListener) => subscribe(legacyStateListeners, listener),
    subscribeVueRequests: (listener: CoexistenceListener) => subscribe(vueRequestListeners, listener),
    vueToLegacy: (value: unknown) => deliver(vueRequestListeners, value),
  });
}

export function mountOverseerApp(root: HTMLElement, port: DesktopPort) {
  const app = createApp(App, { port });
  app.mount(root);
  return app;
}

export function mountOverseerLeaves(
  root: HTMLElement,
  port: DesktopPort,
  bridge: OverseerCoexistenceBridge,
) {
  const app = createApp(App, { port });
  app.provide(overseerCoexistenceBridgeKey, bridge);
  app.mount(root);
  return app;
}
