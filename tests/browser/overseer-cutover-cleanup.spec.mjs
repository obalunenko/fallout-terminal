import { expect, test } from '@playwright/test';

const overseerAppModuleURL = 'http://127.0.0.1:34121/@fs' + new URL('./fixtures/overseer-app.ts', import.meta.url).pathname;

test.use({ bypassCSP: true });

async function mountFixture(page, key) {
  await page.goto('/__fixture/state-changing-command-authoring');
  await page.evaluate(url => import(url), `${overseerAppModuleURL}?${key}`);
}

test('production overseer selects one Vue root without candidate route', async ({ page }) => {
  await mountFixture(page, 'cutover-root');

  await expect(page.locator('#overseerApp')).toHaveCount(1);
  await expect(page.locator('#overseerApp #startScreen')).toHaveCount(1);
  await expect(page.locator('[id="overseerVueLeaves"], [id="legacyOverseerRoot"]')).toHaveCount(0);
});

test('production overseer cleanup releases every legacy and coexistence owner', async ({ page }) => {
  await mountFixture(page, 'cutover-cleanup');
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();

  const released = await page.evaluate(() => {
    __overseerAppFixture.unmount();
    __overseerAppFixture.unmount();
    return {
      applicationUpdate: __desktopFixture.releaseCount('application-update-status'),
      clientCount: __desktopFixture.releaseCount('client-count'),
      coordination: __desktopFixture.releaseCount('coordination-state'),
      publicAccess: __desktopFixture.releaseCount('public-access-status'),
      serverInfo: __desktopFixture.releaseCount('server-info'),
      sessionState: __desktopFixture.releaseCount('session-state'),
    };
  });

  expect(Object.values(released).every(count => count > 0)).toBe(true);
  await expect(page.locator('#overseerApp')).toBeEmpty();
});
