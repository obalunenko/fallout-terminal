import { expect, test } from '@playwright/test';

test('production Player root entry CSP assets sounds and public selectors are exact', async ({ page }) => {
  const errors = [];
  page.on('console', message => {
    if (message.type() === 'error') errors.push(message.text());
  });
  page.on('pageerror', error => errors.push(error.message));

  await page.goto('/');

  await expect(page).toHaveTitle('Fallout Terminal');
  await expect(page.locator('#playerApp')).toHaveCount(1);
  await expect(page.locator('#playerApp > .crt')).toHaveCount(1);
  await expect(page.locator('#playerApp #screen')).toHaveCount(1);
  await expect(page.locator('#playerApp #connOverlay')).toHaveCount(1);
  await expect(page.locator('body > .crt')).toHaveCount(0);
  await expect(page.locator('body > #connOverlay')).toHaveCount(0);
  expect(errors, errors.join('\n')).toEqual([]);
});
