import { createApp } from 'vue';

import App from './App.vue';
import type { PlayerRPCAdapter } from './adapters/player-rpc.js';
import type { RecognitionStorageAdapter } from './adapters/recognition-storage.js';

export interface PlayerMountOptions {
  readonly clientInstanceID: string;
  readonly rpc: PlayerRPCAdapter;
  readonly storage: RecognitionStorageAdapter;
}

export function mountPlayerApp(root: HTMLElement, options: PlayerMountOptions) {
  const app = createApp(App, {
    clientInstanceID: options.clientInstanceID,
    rpc: options.rpc,
    storage: options.storage,
  });
  app.mount(root);
  return app;
}
