import { createApp } from 'vue';

import App from './App.vue';
import type { DesktopPort } from './ports/desktop-port.js';

export function mountOverseerApp(root: HTMLElement, port: DesktopPort) {
  return createApp(App, { port }).mount(root);
}
