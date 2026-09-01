import { mountOverseerApp } from '../../../frontend/overseer/src/mount.js';
import type { OverseerController } from '../../../frontend/overseer/src/controllers/overseer-controller.js';
import { desktopPort } from '../../../frontend/overseer/src/adapters/desktop-api.js';
import type { DesktopPort } from '../../../frontend/overseer/src/ports/desktop-port.js';

interface OverseerBrowserFixture {
  readonly controller: OverseerController;
  readonly port: DesktopPort;
  readonly unmount: () => void;
}

declare global {
  interface Window {
    readonly __overseerAppFixture?: OverseerBrowserFixture;
    readonly __desktopFixture?: {
      readonly authoringSession: () => Readonly<Record<string, unknown>>;
    };
  }
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

window.__overseerAppFixture?.unmount();

document.querySelector('#overseerApp')?.remove();
document.querySelector('#overseerVueLeaves')?.remove();
document.querySelector('#legacyOverseerRoot')?.remove();

const root = document.createElement('div');
root.id = 'overseerApp';
document.body.append(root);

const preloadedFixturePaths = new Set([
  '/__fixture/player-management',
  '/__fixture/public-access-settings',
]);
const preloaded = preloadedFixturePaths.has(window.location.pathname);
const terminalGrouping = window.location.pathname === '/__fixture/terminal-grouping/overseer';
const fixturePort: DesktopPort = preloaded || terminalGrouping
  ? Object.freeze({
      ...desktopPort,
      openSession: async () => {
        if (!terminalGrouping) {
          return Object.freeze({
            canceled: false,
            filePath: '/private/tmp/fallout-browser-preloaded-session.json',
            ok: true,
            session: window.__desktopFixture?.authoringSession() ?? null,
          });
        }
        const response = await fetch('/__fixture/terminal-grouping/open-session');
        const value: unknown = await response.json();
        const result = isRecord(value) ? value : Object.freeze({});
        return Object.freeze({
          canceled: false,
          error: typeof result.error === 'string' ? result.error : '',
          filePath: '/private/tmp/fallout-terminal-grouping.json',
          ok: response.ok && result.ok === true,
          session: isRecord(result.session) ? result.session : null,
        });
      },
      requestTerminalActivation: async request => {
        if (!terminalGrouping) return desktopPort.requestTerminalActivation(request);
        const response = await fetch('/__fixture/terminal-grouping/activate-terminal', {
          body: JSON.stringify(request),
          headers: { 'Content-Type': 'application/json' },
          method: 'POST',
        });
        const value: unknown = await response.json();
        return isRecord(value)
          ? Object.freeze(value)
          : Object.freeze({ error: 'terminal grouping activation fixture is unavailable', ok: false });
      },
      saveSession: async session => {
        if (!terminalGrouping) return desktopPort.saveSession(session);
        const response = await fetch('/__fixture/terminal-grouping/save', {
          body: JSON.stringify(session),
          headers: { 'Content-Type': 'application/json' },
          method: 'POST',
        });
        const value: unknown = await response.json();
        const result = isRecord(value) ? value : Object.freeze({});
        return Object.freeze({
          error: typeof result.error === 'string' ? result.error : '',
          ok: response.ok && result.ok === true,
          savedRevision: Number.isSafeInteger(result.revision) ? Number(result.revision) : 0,
        });
      },
    })
  : desktopPort;
const mounted = mountOverseerApp(root, fixturePort);
if (preloaded) {
  document.querySelector<HTMLButtonElement>('#btnOpenSession')?.click();
}
let active = true;
const fixture: OverseerBrowserFixture = Object.freeze({
  controller: mounted.controller,
  port: fixturePort,
  unmount(): void {
    if (!active) return;
    active = false;
    mounted.unmount();
  },
});

Object.defineProperty(window, '__overseerAppFixture', {
  configurable: true,
  value: fixture,
});
