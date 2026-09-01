import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const actionsModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePlayerActions.ts',
  import.meta.url,
))}`;
const navigationModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/useTerminalNavigation.ts',
  import.meta.url,
))}`;
const vueModuleURL = 'http://127.0.0.1:34120/@fs' + fileURLToPath(new URL(
  '../../frontend/node_modules/vue/dist/vue.esm-browser.js',
  import.meta.url,
));
const terminalSurfaceModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/TerminalSurface.vue',
  import.meta.url,
))}`;

test.use({ bypassCSP: true });

test('Player actions correlate results and await authoritative revision', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const calls = [];
    let releaseNavigate;
    const navigateResult = new Promise(resolve => { releaseNavigate = resolve; });
    const rpc = {
      async navigate(input, options) {
        calls.push({ abortedAtCall: options.signal.aborted, input });
        return navigateResult;
      },
      async selectCharacter(input, options) {
        calls.push({ abortedAtCall: options.signal.aborted, input });
        return { accepted: true, reason: 1, requestId: input.requestId, revision: 12n };
      },
    };
    const ids = ['selection-request', 'navigation-request'];
    const controller = module.createPlayerActionsController({
      authorize: () => true,
      requestIDFactory: () => ids.shift(),
      rpc,
    });
    controller.applyRevision(10);
    const selected = await controller.begin({
      input: { broadcastId: 'broadcast', characterId: 'character', recognitionHandle: 'handle' },
      kind: 'selectCharacter',
    });
    const pendingAfterResult = JSON.parse(JSON.stringify(controller.state));
    controller.applyRevision(11);
    const pendingBeforeRevision = controller.state.pending !== null;
    controller.applyRevision(12);
    const pendingAfterRevision = controller.state.pending;

    const navigationPromise = controller.begin({
      input: {
        action: { case: 'back', value: {} },
        broadcastId: 'broadcast',
        recognitionHandle: 'handle',
        terminalId: 'terminal',
      },
      kind: 'navigate',
    });
    await Promise.resolve();
    controller.dispose();
    releaseNavigate({ accepted: true, reason: 1, requestId: 'navigation-request', revision: 13n });
    const navigationAccepted = await navigationPromise;
    return {
      calls,
      navigationAccepted,
      pendingAfterDispose: controller.state.pending,
      pendingAfterResult,
      pendingAfterRevision,
      pendingBeforeRevision,
      selected,
    };
  }, actionsModuleURL);

  expect(observation.selected).toBe(true);
  expect(observation.pendingAfterResult.pending).toEqual({
    acceptedRevision: 12,
    kind: 'selectCharacter',
    requestID: 'selection-request',
  });
  expect(observation.pendingBeforeRevision).toBe(true);
  expect(observation.pendingAfterRevision).toBeNull();
  expect(observation.navigationAccepted).toBe(false);
  expect(observation.pendingAfterDispose).toBeNull();
  expect(observation.calls.map(call => call.input.requestId)).toEqual(['selection-request', 'navigation-request']);
  expect(observation.calls.every(call => call.abortedAtCall === false)).toBe(true);
});

test('terminal menu preserves selectors keyboard authority and stable rows', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  await page.evaluate(async urls => {
    const [{ createApp, h, nextTick, ref }, surface] = await Promise.all([
      import(urls.vue),
      import(urls.surface),
    ]);
    const host = document.createElement('div');
    host.id = 'terminalSurfaceFixture';
    document.body.append(host);
    const events = [];
    const nodes = ref([
      { content: { case: 'folder', value: { children: [] } }, id: 'docs', name: 'DOCS' },
      { content: { case: 'entry', value: { description: 'Report' } }, id: 'report', name: 'REPORT' },
    ]);
    const canControl = ref(true);
    const app = createApp({
      render: () => h(surface.default, {
        canControl: canControl.value,
        nodes: nodes.value,
        onActivate: node => events.push(node.id),
        selectedID: 'docs',
      }),
    });
    app.mount(host);
    window.__terminalSurfaceFixture = {
      canControl,
      events,
      release: () => app.unmount(),
      reorder: async () => {
        nodes.value = [nodes.value[1], nodes.value[0]];
        await nextTick();
      },
    };
  }, { surface: terminalSurfaceModuleURL, vue: vueModuleURL });

  const docs = page.locator('#terminalSurfaceFixture .term-row[data-node-id="docs"]');
  const report = page.locator('#terminalSurfaceFixture .term-row[data-node-id="report"]');
  await expect(page.locator('#terminalSurfaceFixture #termList[role="listbox"]')).toHaveCount(1);
  await expect(docs).toHaveClass(/\bsel\b/);
  await report.focus();
  await expect(report).toBeFocused();
  await report.press('Enter');
  const stable = await page.evaluate(async () => {
    const before = document.querySelector('#terminalSurfaceFixture .term-row[data-node-id="docs"]');
    await window.__terminalSurfaceFixture.reorder();
    return before === document.querySelector('#terminalSurfaceFixture .term-row[data-node-id="docs"]');
  });
  expect(stable).toBe(true);
  await page.evaluate(() => { window.__terminalSurfaceFixture.canControl.value = false; });
  await page.waitForTimeout(0);
  await docs.click({ force: true });
  expect(await page.evaluate(() => window.__terminalSurfaceFixture.events)).toEqual(['report']);
  await expect(docs).toHaveAttribute('aria-disabled', 'true');
  await page.evaluate(() => window.__terminalSurfaceFixture.release());
});

test('terminal navigation validates pending revision and focus restoration', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const opener = document.createElement('button');
    opener.id = 'navigationOpener';
    document.body.append(opener);
    const other = document.createElement('button');
    document.body.append(other);
    const controller = module.createTerminalNavigationController();
    const list = { mode: 1, path: ['root'] };
    const initial = controller.apply(list, 5, false);
    const captured = controller.capturePendingFocus(opener);
    const pending = controller.apply(list, 5, true);
    other.focus();
    const duplicateCapture = controller.capturePendingFocus(other);
    const invalid = controller.apply({ mode: 2, path: ['root'] }, 6, true);
    const stale = controller.apply({ mode: 1, path: [] }, 4, false);
    const unchanged = JSON.parse(JSON.stringify(controller.state));
    const completed = controller.apply({ mode: 2, path: ['root'], viewEntryId: 'entry-a' }, 6, false);
    await Promise.resolve();
    const restored = document.activeElement === opener;

    const disconnected = document.createElement('button');
    document.body.append(disconnected);
    controller.capturePendingFocus(disconnected);
    controller.apply({ mode: 2, path: ['root'], viewEntryId: 'entry-a' }, 7, true);
    disconnected.remove();
    controller.apply(list, 8, false);
    await Promise.resolve();
    const disconnectedFocused = document.activeElement === disconnected;
    controller.dispose();
    return {
      captured,
      completed,
      disconnectedFocused,
      duplicateCapture,
      initial,
      invalid,
      pending,
      restored,
      stale,
      unchanged,
    };
  }, navigationModuleURL);

  expect(observation).toEqual({
    captured: true,
    completed: true,
    disconnectedFocused: false,
    duplicateCapture: false,
    initial: true,
    invalid: false,
    pending: true,
    restored: true,
    stale: false,
    unchanged: {
      commandNodeID: null,
      mode: 'list',
      path: ['root'],
      pending: true,
      revision: 5,
      viewEntryID: null,
    },
  });
});
