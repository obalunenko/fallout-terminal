import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const paginationModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePaginationMeasurement.ts',
  import.meta.url,
))}`;
const terminalRecordModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/TerminalRecord.vue',
  import.meta.url,
))}`;
const terminalFooterModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/TerminalFooter.vue',
  import.meta.url,
))}`;

test.use({ bypassCSP: true });

test('pagination measurement disconnects observers frames and late font work', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const container = document.createElement('div');
    const original = document.createElement('button');
    original.textContent = 'ORIGINAL';
    let clicks = 0;
    original.addEventListener('click', () => { clicks += 1; });
    container.append(original);
    document.body.append(container);
    Object.defineProperties(container, {
      clientHeight: { configurable: true, value: 40 },
      clientWidth: { configurable: true, value: 80 },
      scrollHeight: { configurable: true, get: () => (container.textContent?.length ?? 0) > 12 ? 80 : 20 },
      scrollWidth: { configurable: true, value: 40 },
    });

    let observerCallback;
    let disconnects = 0;
    let observes = 0;
    let fontResolve;
    const fontReady = new Promise(resolve => { fontResolve = resolve; });
    const activeFrames = new Set();
    const frameScheduler = {
      cancelAnimationFrame(handle) { cancelAnimationFrame(handle); activeFrames.delete(handle); },
      requestAnimationFrame(callback) {
        const handle = requestAnimationFrame(time => { activeFrames.delete(handle); callback(time); });
        activeFrames.add(handle);
        return handle;
      },
    };
    const pageSets = [];
    const controller = module.createPaginationMeasurementController({
      fontReady,
      frameScheduler,
      observerFactory: callback => {
        observerCallback = callback;
        return {
          disconnect() { disconnects += 1; },
          observe(target) { if (target === container) observes += 1; },
        };
      },
    }, pages => pageSets.push([...pages]));
    controller.setContent(container, 'alpha beta gamma delta');
    await new Promise(resolve => requestAnimationFrame(resolve));
    const restoredNode = container.firstChild === original;
    original.click();
    observerCallback([], {});
    controller.setContent(container, 'next page body');
    controller.dispose();
    controller.dispose();
    fontResolve();
    await Promise.resolve();
    await new Promise(resolve => requestAnimationFrame(resolve));
    return {
      activeFrames: activeFrames.size,
      clicks,
      disconnects,
      observes,
      pageSets,
      restoredNode,
    };
  }, paginationModuleURL);

  expect(observation.observes).toBe(1);
  expect(observation.disconnects).toBe(1);
  expect(observation.activeFrames).toBe(0);
  expect(observation.restoredNode).toBe(true);
  expect(observation.clicks).toBe(1);
  expect(observation.pageSets).toHaveLength(1);
  expect(observation.pageSets[0].join('')).toBe('alpha beta gamma delta');
  expect(observation.pageSets[0].length).toBeGreaterThan(1);
});

test('terminal record footer and pagination preserve layout focus and cleanup', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  await page.evaluate(async urls => {
    const [footer, record] = await Promise.all([
      import(urls.footer),
      import(urls.record),
    ]);
    const compiled = await (await fetch(urls.record)).text();
    const runtimePath = compiled.match(/from "([^"]*\/node_modules\/\.vite\/deps\/vue\.js\?v=[^"]+)"/u)?.[1];
    if (runtimePath === undefined) throw new Error('compiled terminal record Vue runtime was not found');
    const { createApp, h, ref } = await import(new URL(runtimePath, location.origin).href);
    const host = document.createElement('div');
    host.id = 'paginationFixture';
    document.body.append(host);
    const events = [];
    const pageIndex = ref(0);
    const app = createApp({
      render: () => h('div', [
        h(record.default, {
          pageIndex: pageIndex.value,
          pending: true,
          text: 'LINE ONE\nLINE TWO',
          title: 'REPORT',
          visibleLines: 2,
        }),
        h(footer.default, {
          backLabel: '[ НАЗАД ]',
          canControl: true,
          onBack: () => events.push('back'),
          onPage: next => { events.push(`page:${next}`); pageIndex.value = next; },
          pageCount: 3,
          pageIndex: pageIndex.value,
          showBack: true,
        }),
      ]),
    });
    app.mount(host);
    window.__paginationFixture = { events, release: () => app.unmount() };
  }, { footer: terminalFooterModuleURL, record: terminalRecordModuleURL });

  await expect(page.locator('#paginationFixture #termEntry[aria-busy="true"]')).toContainText('REPORT');
  await expect(page.locator('#paginationFixture #entryBody')).toContainText('LINE ONE');
  const next = page.locator('#paginationFixture #pageNext');
  await next.focus();
  await expect(next).toBeFocused();
  await next.press('Enter');
  await expect(page.locator('#paginationFixture #pageIndicator')).toHaveText('2 / 3');
  await expect(next).toBeFocused();
  await page.locator('#paginationFixture #pagePrev').focus();
  await page.locator('#paginationFixture #pagePrev').press('Enter');
  await page.locator('#paginationFixture #backBtn').focus();
  await page.locator('#paginationFixture #backBtn').press('Enter');
  expect(await page.evaluate(() => window.__paginationFixture.events)).toEqual(['page:1', 'page:0', 'back']);
  await page.evaluate(() => window.__paginationFixture.release());
  await expect(page.locator('#paginationFixture').locator('*')).toHaveCount(0);
});
