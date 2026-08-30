import { expect, test } from '@playwright/test';

const FIXTURE_URL = '/__fixture/state-changing-command-authoring';

test.use({ bypassCSP: true });

async function resolveCalls(page) {
  return page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'ResolveTerminalSwitch'));
}

test.beforeEach(async ({ page, request }) => {
  const reset = await request.post(`${FIXTURE_URL}/reset`);
  expect(reset.ok()).toBe(true);
  await page.goto(FIXTURE_URL);
  await page.evaluate(() => import('http://127.0.0.1:34120/candidate-main.ts?terminal-switch-focused'));
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
});

test('terminal switch approval resolves once and restores focus', async ({ page }) => {
  const opener = page.locator('#btnAddTerminal');
  await opener.focus();
  await page.evaluate(() => {
    __overseerCoexistenceBridge.legacyToVue({
      kind: 'terminal-switch-required',
      switchId: 'switch-focused-1',
    });
  });

  const dialog = page.locator('#overseerVueLeaves #terminalSwitchDialog');
  await expect(dialog).toBeVisible();
  const preserve = dialog.locator('#btnPreserveTerminalSwitch');
  await expect(preserve).toBeFocused();
  await preserve.evaluate((button) => {
    button.click();
    button.click();
  });

  await expect.poll(() => resolveCalls(page)).toEqual([
    expect.objectContaining({
      args: [{ decision: 'preserve', switchId: 'switch-focused-1' }],
    }),
  ]);
  await expect(dialog).toBeHidden();
  await expect(opener).toBeFocused();

  await page.evaluate(() => {
    __overseerCoexistenceBridge.legacyToVue({
      kind: 'terminal-switch-required',
      switchId: 'switch-focused-1',
    });
  });
  await expect(dialog).toBeHidden();
  expect(await resolveCalls(page)).toHaveLength(1);

  await page.evaluate(() => {
    __overseerVueFixture.unmount();
    __overseerVueFixture.unmount();
    __overseerCoexistenceBridge.legacyToVue({
      kind: 'terminal-switch-required',
      switchId: 'switch-after-unmount',
    });
  });
  await expect(page.locator('#overseerVueLeaves')).toBeEmpty();
  expect(await resolveCalls(page)).toHaveLength(1);
});
