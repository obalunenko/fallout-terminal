import { expect, test } from '@playwright/test';

const RUNTIME_LOG_FIXTURE = '/__fixture/runtime-logs';

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

test('an unavailable retained file reports a safe warning while preserving directory access', async ({ page }) => {
  await page.goto('/__fixture/public-access-settings?runtime-logs-unavailable=1');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
  const button = page.locator('[data-action="open-log-location"]:visible').first();
  await button.click();
  const status = page.locator('[data-log-access-status]:visible').first();
  await expect(status).toContainText('ТЕКУЩИЙ ФАЙЛ ЛОГА НЕДОСТУПЕН');
  await expect(status).toContainText('/Users/fixture/Application Support/Fallout Terminal/logs');
});

test('current retained log correlates player and facility activity without protected values', async ({ page, request }) => {
  const correlationId = 'facility-browser-action-96';
  const forbidden = {
    playerName: 'PRIVATE PLAYER NAME T096',
    authoredLabel: 'AUTHORED REACTOR LABEL T096',
    authoredContent: 'PRIVATE REACTOR ENTRY CONTENT T096',
    secret: 'PLAYER-PASSWORD-CANARY-T096',
    rawError: 'dial tcp provider.internal: raw persistence error /private/campaign-t096.json',
  };
  const seeded = await request.post(`${RUNTIME_LOG_FIXTURE}/seed`, {
    data: { correlationId, forbidden },
  });
  expect(seeded.status()).toBe(204);

  await page.goto('/__fixture/public-access-settings');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
  const openLogs = page.locator('[data-action="open-log-location"]').nth(1);
  await expect(openLogs).toBeEnabled();
  await openLogs.click();
  const accessStatus = page.locator('[data-log-access-status]').nth(1);
  await expect(accessStatus).toContainText('application-current.log');

  const currentResponse = await request.get(`${RUNTIME_LOG_FIXTURE}/current`);
  expect(currentResponse.ok()).toBe(true);
  const current = await currentResponse.json();
  expect(current.path).toContain('/logs/application-current.log');
  expect(await accessStatus.textContent()).toContain('application-current.log');

  const lines = current.contents.trim().split('\n');
  expect(lines).toHaveLength(3);
  expect(lines[0]).toContain('event=command.request_received');
  expect(lines[0]).toContain(`request_id=${correlationId}`);
  expect(lines[0]).toContain('role=active');
  expect(lines[1]).toContain('event=facility.request_received');
  expect(lines[1]).toContain(`correlation_id=${correlationId}`);
  expect(lines[1]).toContain('facility_action=command');
  expect(lines[1]).toContain('outcome=pending');
  expect(lines[1]).toContain('previous_facility_revision=7');
  expect(lines[1]).toContain('resulting_facility_revision=7');
  expect(lines[2]).toContain('event=facility.decision');
  expect(lines[2]).toContain(`correlation_id=${correlationId}`);
  expect(lines[2]).toContain('decision=approve');
  expect(lines[2]).toContain('outcome=succeeded');
  expect(lines[2]).toContain('previous_facility_revision=7');
  expect(lines[2]).toContain('resulting_facility_revision=8');
  expect(lines.filter(line => line.includes(correlationId))).toHaveLength(3);

  for (const value of Object.values(forbidden)) {
    expect(current.contents).not.toContain(value);
    await expect(page.locator('body')).not.toContainText(value);
  }
  expect(current.contents).not.toContain('provider.internal');
  expect(current.contents).not.toContain('/private/campaign-t096.json');

  const calls = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'OpenLogLocation'));
  expect(calls).toHaveLength(1);
  expect(calls[0].args).toEqual([]);
});
