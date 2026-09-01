import { createApp } from 'vue';

import App from './App.vue';
import { createOverseerController, overseerControllerKey } from './controllers/overseer-controller.js';
import type { DesktopPort } from './ports/desktop-port.js';

export function mountOverseerApp(root: HTMLElement, port: DesktopPort) {
  const controller = createOverseerController(port);
  const app = createApp(App, { port });
  app.provide(overseerControllerKey, controller);
  app.onUnmount(controller.dispose);
  app.mount(root);
  return Object.freeze({
    controller,
    unmount: () => { app.unmount(); },
  });
}
