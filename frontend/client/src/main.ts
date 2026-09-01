import '../client.css';

import { createPlayerRPCAdapter } from './adapters/player-rpc.js';
import { createRecognitionStorage } from './adapters/recognition-storage.js';
import { mountPlayerApp } from './mount.js';

const root = document.getElementById('playerApp');
if (!(root instanceof HTMLElement)) throw new Error('Player root is unavailable');

const clientInstanceID = typeof globalThis.crypto?.randomUUID === 'function'
  ? globalThis.crypto.randomUUID()
  : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;

mountPlayerApp(root, {
  clientInstanceID,
  rpc: createPlayerRPCAdapter(),
  storage: createRecognitionStorage(),
});
