import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const projectionModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePlayerProjection.ts',
  import.meta.url,
))}`;
const overlayModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/useConnectionOverlay.ts',
  import.meta.url,
))}`;

test.use({ bypassCSP: true });

test('candidate shell preserves crt screen semantics and server authority', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  await expect(page.locator('.crt > #screen.screen')).toHaveCount(1);
  await expect(page.locator('#screen > .scanlines[aria-hidden="true"]')).toHaveCount(1);
  await expect(page.locator('#screen > .vignette[aria-hidden="true"]')).toHaveCount(1);
  await expect(page.locator('#termIdle')).toContainText('ОЖИДАНИЕ ТРАНСЛЯЦИИ');
  await expect(page.locator('#normalHeader')).toBeHidden();
  await expect(page.locator('#normalHeader')).toContainText('ROBCO INDUSTRIES UNIFIED OPERATING SYSTEM');
  await expect(page.locator('#normalHeader')).toContainText('COPYRIGHT 2075-2077 ROBCO INDUSTRIES');

  const projection = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const controller = module.createPlayerProjectionController();
    const playerState = { fallbackName: 'SERVER NAME', logicalSessionId: 'logical-1' };
    const terminalPresentation = {
      presentation: {
        case: 'liveTerminal',
        value: { introText: 'SERVER INTRO', terminalId: 'terminal-1', terminalName: 'SERVER TERMINAL' },
      },
    };
    const snapshotAccepted = controller.applySnapshot({
      playerState,
      recognitionHandle: 'opaque',
      revision: 7n,
      terminalPresentation,
    });
    const staleAccepted = controller.applyUpdate({
      playerState: { ...playerState, fallbackName: 'OPTIMISTIC NAME' },
      revision: 6n,
      terminalPresentation: { presentation: { case: 'noLiveTerminal', value: {} } },
    });
    const afterStale = {
      fallbackName: controller.state.playerState?.fallbackName,
      introText: controller.state.liveTerminal?.introText,
      revision: controller.state.revision,
    };
    const updateAccepted = controller.applyUpdate({
      playerState: { ...playerState, fallbackName: 'NEXT SERVER NAME' },
      revision: 8n,
      terminalPresentation: { presentation: { case: 'noLiveTerminal', value: {} } },
    });
    const afterUpdate = {
      fallbackName: controller.state.playerState?.fallbackName,
      liveTerminal: controller.state.liveTerminal,
      revision: controller.state.revision,
    };
    controller.dispose();
    const afterDisposeAccepted = controller.applyUpdate({
      playerState,
      revision: 9n,
      terminalPresentation,
    });
    return { afterDisposeAccepted, afterStale, afterUpdate, snapshotAccepted, staleAccepted, updateAccepted };
  }, projectionModuleURL);

  expect(projection).toEqual({
    afterDisposeAccepted: false,
    afterStale: { fallbackName: 'SERVER NAME', introText: 'SERVER INTRO', revision: 7 },
    afterUpdate: { fallbackName: 'NEXT SERVER NAME', liveTerminal: null, revision: 8 },
    snapshotAccepted: true,
    staleAccepted: false,
    updateAccepted: true,
  });
});

test('connection overlay preserves ordering live regions and cleanup', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  await expect(page.locator('#connOverlay[role="status"][aria-live="assertive"][aria-atomic="true"]')).toBeVisible();
  await expect(page.locator('#connText')).toHaveText(/^(?:УСТАНОВКА СВЯЗИ|СВЯЗЬ ПОТЕРЯНА — ПЕРЕПОДКЛЮЧЕНИЕ)\.\.\.$/u);
  await expect(page.locator('#playerNotice[role="status"][aria-live="polite"][aria-atomic="true"]')).toBeHidden();

  const observation = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const states = [];
    const controller = module.createConnectionOverlayController(state => states.push({ ...state }));
    const connecting = controller.apply({ error: '', generation: 1, phase: 'connecting', revision: 0 });
    const ready = controller.apply({ error: '', generation: 1, phase: 'ready', revision: 4 });
    const reconnecting = controller.apply({ error: 'lost', generation: 1, phase: 'reconnecting', revision: 4 });
    const nextConnecting = controller.apply({ error: '', generation: 2, phase: 'connecting', revision: 0 });
    const staleReady = controller.apply({ error: '', generation: 1, phase: 'ready', revision: 5 });
    controller.dispose();
    controller.dispose();
    const afterDispose = controller.apply({ error: '', generation: 2, phase: 'ready', revision: 5 });
    return { afterDispose, connecting, nextConnecting, ready, reconnecting, staleReady, states };
  }, overlayModuleURL);

  expect(observation).toEqual({
    afterDispose: false,
    connecting: true,
    nextConnecting: true,
    ready: true,
    reconnecting: true,
    staleReady: false,
    states: [
      { generation: 1, message: 'УСТАНОВКА СВЯЗИ...', visible: true },
      { generation: 1, message: '', visible: false },
      { generation: 1, message: 'СВЯЗЬ ПОТЕРЯНА — ПЕРЕПОДКЛЮЧЕНИЕ...', visible: true },
      { generation: 2, message: 'УСТАНОВКА СВЯЗИ...', visible: true },
    ],
  });
});
