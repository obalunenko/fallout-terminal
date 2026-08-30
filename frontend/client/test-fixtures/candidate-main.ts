import '../client.css';

import { mountPlayerCandidate } from '../src/mount.js';
import type { PlayerTransport } from '../src/ports/player-transport.js';

const root = document.getElementById('playerApp');
if (!(root instanceof HTMLElement)) throw new Error('Player candidate root is unavailable');

const transport: PlayerTransport = Object.freeze({
  current: Object.freeze({ phase: 'idle', message: '' }),
});

mountPlayerCandidate(root, { transport });
