import '../src/overseer.css';

import {
  createOverseerCoexistenceBridge,
  mountOverseerApp,
  mountOverseerLeaves,
  type OverseerCoexistenceBridge,
} from '../src/mount.js';
import type { DesktopPort } from '../src/ports/desktop-port.js';
import { fakeDesktopPort } from './fake-desktop-port.js';

declare global {
  interface Window {
    readonly __attachOverseerLegacyBridge?: (bridge: OverseerCoexistenceBridge) => void;
    readonly desktopAPI?: DesktopPort;
  }
}

const root = document.getElementById('overseerApp') ?? document.getElementById('overseerVueLeaves');
if (!(root instanceof HTMLElement)) throw new Error('Overseer candidate root is unavailable');

const coexistencePort = root.id === 'overseerVueLeaves' ? window.desktopAPI : undefined;
if (root.id === 'overseerVueLeaves' && coexistencePort === undefined) {
  throw new Error('Overseer coexistence desktop port is unavailable');
}

const bridge = root.id === 'overseerVueLeaves' ? createOverseerCoexistenceBridge() : null;
if (bridge !== null) {
  Object.defineProperty(globalThis, '__overseerCoexistenceBridge', {
    configurable: true,
    value: bridge,
  });
}
const app = bridge === null
  ? mountOverseerApp(root, fakeDesktopPort)
  : mountOverseerLeaves(root, coexistencePort ?? fakeDesktopPort, bridge);
if (bridge !== null) window.__attachOverseerLegacyBridge?.(bridge);
let mounted = true;
const browserFixture = Object.freeze({
  unmount(): void {
    if (!mounted) return;
    mounted = false;
    app.unmount();
  },
});

Object.defineProperty(globalThis, '__overseerVueFixture', {
  configurable: true,
  value: browserFixture,
});
