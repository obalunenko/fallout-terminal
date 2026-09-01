import '../../../frontend/client/client.css';

import type { PlayerRPCAdapter } from '../../../frontend/client/src/adapters/player-rpc.js';
import type { RecognitionStorageAdapter } from '../../../frontend/client/src/adapters/recognition-storage.js';
import { mountPlayerApp } from '../../../frontend/client/src/mount.js';

interface PlayerBrowserDependencies {
  readonly clientInstanceID: string;
  readonly rpc: PlayerRPCAdapter;
  readonly storage: RecognitionStorageAdapter;
}

interface PlayerBrowserFixture {
  readonly unmount: () => void;
}

declare global {
  interface Window {
    __playerAppFixture?: PlayerBrowserFixture;
    __playerAppTestDependencies?: PlayerBrowserDependencies;
  }
}

window.__playerAppFixture?.unmount();

const dependencies = window.__playerAppTestDependencies;
if (dependencies === undefined) throw new Error('Player browser dependencies are unavailable');

const root = document.getElementById('playerApp');
if (!(root instanceof HTMLElement)) throw new Error('Player browser root is unavailable');
root.dataset.lifecycleOwner = 'vue';

const app = mountPlayerApp(root, dependencies);
let active = true;
Object.defineProperty(window, '__playerAppFixture', {
  configurable: true,
  value: Object.freeze({
    unmount(): void {
      if (!active) return;
      active = false;
      app.unmount();
    },
  }),
});
