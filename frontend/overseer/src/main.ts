import { desktopPort } from './adapters/desktop-api.js';
import {
  createOverseerCoexistenceBridge,
  mountOverseerLeaves,
} from './mount.js';

const coexistenceBridge = createOverseerCoexistenceBridge();

const root = document.getElementById('overseerVueLeaves');
if (!(root instanceof HTMLElement)) throw new Error('Overseer Vue leaf root is unavailable');

Object.defineProperty(globalThis, '__overseerCoexistenceBridge', {
  configurable: true,
  value: coexistenceBridge,
});
mountOverseerLeaves(root, desktopPort, coexistenceBridge);
