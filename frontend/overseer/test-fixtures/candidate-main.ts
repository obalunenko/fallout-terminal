import '../src/overseer.css';

import { mountOverseerApp } from '../src/mount.js';
import { fakeDesktopPort } from './fake-desktop-port.js';

const root = document.getElementById('overseerApp');
if (!(root instanceof HTMLElement)) throw new Error('Overseer candidate root is unavailable');

mountOverseerApp(root, fakeDesktopPort);
