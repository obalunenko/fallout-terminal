import { expect, test } from '@playwright/test';

test('production player selects one Vue root without candidate route', async ({ page }) => {
  await page.goto('/');

  await expect(page.locator('#playerApp')).toHaveCount(1);
  await expect(page.locator('#playerApp > .crt')).toHaveCount(1);
  await expect(page.locator('#playerApp #screen')).toHaveCount(1);
  await expect(page.locator('#playerApp #connOverlay')).toHaveCount(1);
  await expect(page.locator('script[src$="client.js"]')).toHaveCount(0);
  await expect(page.locator('script[src$="sound.js"]')).toHaveCount(0);
  await expect(page.locator('script[src$="presentation-uplink.js"]')).toHaveCount(0);
});

test('production player cleanup releases every legacy and staging owner', async ({ page }) => {
  const errors = [];
  const requests = [];
  page.on('console', message => {
    if (message.type() === 'error') errors.push(message.text());
  });
  page.on('pageerror', error => errors.push(error.message));
  page.on('request', request => requests.push(new URL(request.url()).pathname));

  await page.goto('/');
  await expect(page.locator('#playerApp > .crt')).toHaveCount(1);
  await page.reload();
  await expect(page.locator('#playerApp > .crt')).toHaveCount(1);

  expect(requests.some(path => /(?:client|sound|presentation-uplink)\.js$/u.test(path))).toBe(false);
  expect(errors, errors.join('\n')).toEqual([]);
});
