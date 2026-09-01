import { expect, test } from '@playwright/test';

const overseerAppModuleURL = 'http://127.0.0.1:34121/@fs' + new URL('./fixtures/overseer-app.ts', import.meta.url).pathname;

test.use({ bypassCSP: true });

const expectedAssertion = 'Vue runtime shell preserves start/main/status semantics and releases runtime subscription';

test(expectedAssertion, async ({ page }) => {
  await page.goto('/__fixture/state-changing-command-authoring');
  await page.evaluate(url => import(url), overseerAppModuleURL + '?runtime-shell');

  const vueRoot = page.locator('#overseerApp');
  const runtimeHeader = page.locator('#runtimeHeaderVueLeaf #runtimeHeader');
  if (await runtimeHeader.count() === 0) {
    process.stderr.write(`AssertionError: ${expectedAssertion}\n`);
    throw new Error('Vue-owned Overseer runtime shell is not implemented');
  }

  await expect(page.locator('#startScreen')).toHaveCount(1);
  await expect(vueRoot.locator('#startScreen')).toBeVisible();
  await expect(vueRoot.locator('#startStatus')).toHaveAttribute('data-state', 'ready-local');
  await expect(vueRoot.locator('#startStatus')).toContainText('ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ');
  await expect(page.locator('#mainLayout')).toHaveCount(1);

  await page.evaluate(() => {
    __desktopFixture.emit('server-info', {
      url: 'https://runtime.example',
      localUrl: 'http://127.0.0.1:3690',
      tunnel: true,
      port: 3690,
    });
    __desktopFixture.emit('client-count', 3);
  });
  await page.locator('#btnNewSession').click();

  await expect(vueRoot.locator('#startScreen')).toHaveCount(0);
  await expect(page.locator('#mainLayout')).toBeVisible();
  await expect(runtimeHeader).toBeVisible();
  await expect(runtimeHeader.locator('#sessionFileLabel')).toContainText('fallout-state-changing-authoring-new.json');
  await expect(runtimeHeader.locator('#serverUrl')).toHaveText('https://runtime.example');
  await expect(runtimeHeader.locator('#clientCount')).toHaveText('3');

  await runtimeHeader.locator('#serverUrl').click();
  await expect.poll(() => page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'OpenURL').at(-1))).toEqual({
    method: 'OpenURL',
    args: ['https://runtime.example'],
  });

  expect(await page.evaluate(() => ({
    clientCount: __desktopFixture.releaseCount('client-count'),
    serverInfo: __desktopFixture.releaseCount('server-info'),
  }))).toEqual({ clientCount: 0, serverInfo: 0 });

  const released = await page.evaluate(() => {
    __overseerAppFixture.unmount();
    return {
      clientCount: __desktopFixture.releaseCount('client-count'),
      serverInfo: __desktopFixture.releaseCount('server-info'),
    };
  });
  expect(released).toEqual({ clientCount: 1, serverInfo: 1 });
  await page.evaluate(() => __overseerAppFixture.unmount());
  expect(await page.evaluate(() => ({
    clientCount: __desktopFixture.releaseCount('client-count'),
    serverInfo: __desktopFixture.releaseCount('server-info'),
  }))).toEqual(released);
  await expect(vueRoot).toBeEmpty();
});
