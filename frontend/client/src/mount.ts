import { createApp } from 'vue';

import App from './App.vue';
import type { PlayerTransport } from './ports/player-transport.js';

export interface PlayerCandidateMountOptions {
  readonly transport: PlayerTransport;
}

export function mountPlayerCandidate(root: HTMLElement, options: PlayerCandidateMountOptions) {
  return createApp(App, { transport: options.transport }).mount(root);
}
