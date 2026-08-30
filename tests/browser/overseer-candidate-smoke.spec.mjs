import { expect, test } from '@playwright/test';

test('overseer candidate route resolves from test-only entry', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');

  const root = page.locator('#overseerApp');
  await expect(root).toHaveCount(1);
  await expect(root).toBeAttached();
});
