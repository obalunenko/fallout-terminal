import { expect, test } from '@playwright/test';

const RECOGNITION_KEY = 'fallout-terminal.player-token';

test.beforeEach(async ({ request }) => {
  expect((await request.post('/__fixture/reset')).status()).toBe(204);
});

test('production player converges through action revision hacking reconnect sound and cleanup stress', async ({ page, request }) => {
  const errors = [];
  let subscriptions = 0;
  let manifests = 0;
  page.on('console', message => {
    if (message.type() === 'error') errors.push(message.text());
  });
  page.on('pageerror', error => errors.push(error.message));
  page.on('request', request => {
    if (request.url().endsWith('/Subscribe')) subscriptions += 1;
    if (request.url().endsWith('/SoundManifest')) manifests += 1;
  });

  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await expect.poll(() => manifests).toBeGreaterThanOrEqual(8);
  await page.locator('#characterOptions button:not([disabled])').first().click();
  await expect(page.locator('#termList')).toBeVisible();
  await page.locator('.term-row', { hasText: 'DOCS' }).click();
  await expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible();
  expect((await request.post('/__fixture/update')).status()).toBe(204);
  await page.locator('#backBtn').click();
  await expect(page.locator('.term-row', { hasText: 'PUBLIC UPDATE' })).toBeVisible();

  expect((await request.post('/__fixture/local/hacking')).status()).toBe(204);
  await expect(page.locator('#hackBoard')).toBeVisible();
  const target = page.locator('#hackColumns [data-target]:not([data-target=""])').first();
  await target.click();
  await expect(page.locator('#hackLog')).not.toHaveText('');
  const geometry = await page.locator('#screen').evaluate(screen => ({
    horizontalOverflow: screen.scrollWidth - screen.clientWidth,
    verticalOverflow: screen.scrollHeight - screen.clientHeight,
  }));
  expect(geometry.horizontalOverflow).toBeLessThanOrEqual(1);
  expect(geometry.verticalOverflow).toBeLessThanOrEqual(1);

  expect((await request.post('/__fixture/local/disconnect')).status()).toBe(204);
  await expect.poll(() => subscriptions, { timeout: 5_000 }).toBeGreaterThanOrEqual(2);
  await expect(page.locator('#connOverlay')).toBeHidden({ timeout: 5_000 });
  expect(await page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY)).toMatch(/\S+/u);
  expect(errors, errors.join('\n')).toEqual([]);
});

test('production player preserves multi-tab authority unary fallback and context cleanup', async ({ browser }) => {
  const controllerContext = await browser.newContext();
  const pages = await Promise.all([controllerContext.newPage(), controllerContext.newPage()]);
  await Promise.all(pages.map(page => page.goto('/')));
  await Promise.all(pages.map(page => expect(page.locator('#connOverlay')).toBeHidden()));
  const handles = await Promise.all(pages.map(page => page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY)));
  expect(new Set(handles).size).toBe(1);

  await pages[0].locator('#characterOptions button:not([disabled])').first().click();
  await Promise.all(pages.map(page => expect(page.locator('#termList')).toBeVisible()));
  const fallbackRequests = [];
  pages[0].on('request', request => {
    if (request.url().endsWith('/SetPresentation')) fallbackRequests.push(request.url());
  });
  await pages[0].locator('.term-row', { hasText: 'STATUS' }).hover();
  await expect(pages[0].locator('.term-row', { hasText: 'STATUS' })).toHaveClass(/sel/u);
  await expect.poll(() => fallbackRequests.length).toBe(1);

  const observerContext = await browser.newContext();
  const observer = await observerContext.newPage();
  await observer.goto('/');
  await expect(observer.locator('#connOverlay')).toBeHidden();
  await observer.locator('#characterOptions button:not([disabled])').nth(1).click();
  await expect(observer.locator('#termList')).toBeVisible();
  await expect(observer.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
  await expect(observer.locator('.term-row').first()).toHaveAttribute('aria-disabled', 'true');

  await observerContext.close();
  await controllerContext.close();
});
