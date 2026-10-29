import { expect, test } from '@playwright/test';

test('startup and main controls open retained logs with shared accessible feedback', async ({ page }) => {
  await page.goto('/__fixture/public-access-settings');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
  const buttons = page.locator('[data-action="open-log-location"]');
  await expect(buttons).toHaveCount(2);

  await buttons.nth(1).click();
  await expect(page.locator('[data-log-access-status]').nth(1)).toContainText('application-current.log');

  await page.evaluate(() => {
    const startup = document.querySelector('[data-action="open-log-location"]');
    startup.click();
    startup.click();
  });
  await expect(page.locator('[data-log-access-status]').first()).toContainText('ЛОГИ ОТКРЫТЫ');
  const calls = await page.evaluate(() => __desktopFixture.calls.filter(call => call.method === 'OpenLogLocation'));
  expect(calls).toHaveLength(2);
  expect(calls.every(call => call.args.length === 0)).toBe(true);
});

test('log access remains available during degraded startup and shows the exact manual path on failure', async ({ page }) => {
  await page.goto('/__fixture/public-access-settings?runtime-logs-failure=1');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
  await expect(page.locator('#startStatus')).toContainText('ЗАПУСК НЕ ЗАВЕРШЁН');
  const button = page.locator('[data-action="open-log-location"]:visible').first();
  await expect(button).toBeEnabled();
  await button.click();
  const status = page.locator('[data-log-access-status]:visible').first();
  await expect(status).toContainText('Could not open the application log directory.');
  await expect(status).toContainText('/Users/fixture/Application Support/Fallout Terminal/logs');
});

test('log access failure names the directory even when an active log file exists', async ({ page }) => {
  await page.goto('/__fixture/public-access-settings');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
  await page.evaluate(() => __desktopFixture.setNextTerminalActionResult('OpenLogLocation', {
    ok: false,
    error: 'Could not open the application log directory.',
    directoryPath: '/Users/fixture/Application Support/Fallout Terminal/logs',
    activeLogPath: '/Users/fixture/Application Support/Fallout Terminal/logs/application-current.log',
  }));
  await page.locator('[data-action="open-log-location"]').nth(1).click();
  const status = page.locator('[data-log-access-status]').nth(1);
  await expect(status).toHaveText('Could not open the application log directory. · /Users/fixture/Application Support/Fallout Terminal/logs');
});

test('log access reports degraded retained output when no active log file exists', async ({ page }) => {
  await page.goto('/__fixture/public-access-settings');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
  await page.evaluate(() => __desktopFixture.setNextTerminalActionResult('OpenLogLocation', {
    ok: true,
    error: '',
    directoryPath: '/Users/fixture/Application Support/Fallout Terminal/logs',
    activeLogPath: '',
  }));
  await page.locator('[data-action="open-log-location"]').nth(1).click();
  const status = page.locator('[data-log-access-status]').nth(1);
  await expect(status).toContainText('ЛОГИ ОТКРЫТЫ');
  await expect(status).toContainText('/Users/fixture/Application Support/Fallout Terminal/logs');
  await expect(status).toContainText('ЗАПИСЬ ЛОГОВ В ФАЙЛ НЕДОСТУПНА');

  await page.locator('[data-action="open-log-location"]').nth(1).click();
  await expect(status).toContainText('application-current.log');
  await expect(status).not.toContainText('ЗАПИСЬ ЛОГОВ В ФАЙЛ НЕДОСТУПНА');
});
