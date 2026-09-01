import { desktopPort } from './adapters/desktop-api';
import { mountOverseerApp } from './mount.js';
import './overseer.css';

const root = document.getElementById('overseerApp');
if (!(root instanceof HTMLElement)) throw new Error('Overseer application root is unavailable');

mountOverseerApp(root, desktopPort);
