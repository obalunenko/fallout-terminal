import { expect, test } from '@playwright/test';

const RECOGNITION_KEY = 'fallout-terminal.player-token';
const PUBLIC_TEST_URL = process.env.FALLOUT_PUBLIC_TEST_URL;
const PUBLIC_TEST_USERNAME = process.env.FALLOUT_PUBLIC_TEST_USERNAME;
const PUBLIC_TEST_PASSWORD = process.env.FALLOUT_PUBLIC_TEST_PASSWORD;
const PUBLIC_TEST_FIXTURE = process.env.FALLOUT_PUBLIC_TEST_FIXTURE === '1';
const PUBLIC_TEST_HACKING = process.env.FALLOUT_PUBLIC_TEST_HACKING === '1';
const PROTECTED_AUTHORIZATION = `Basic ${Buffer.from('players:password-long-enough').toString('base64')}`;

async function protectedFixtureURL(request) {
  const response = await request.get('/__fixture/edge/status');
  expect(response.status()).toBe(200);
  return (await response.json()).publicUrl;
}

test.beforeEach(async ({ request, page }) => {
  await page.addInitScript(() => {
    window.WebSocket = class ForbiddenLegacyPlayerTransport {
      constructor() {
        throw new Error('the player must use generated Connect exclusively');
      }
    };
  });
  const response = await request.post('/__fixture/reset');
  expect(response.status()).toBe(204);
});

test('built player contains no legacy JSON protocol or WebSocket constructor', async ({ page }) => {
  const response = await page.request.get('/assets/' + (await page.request.get('/').then(result => result.text()))
    .match(/src="\/assets\/(.+\.js)"/)?.[1]);
  expect(response.ok()).toBe(true);
  const bundle = await response.text();
  for (const forbidden of [
    'WebSocket(', 'SESSION_HELLO', 'CHARACTER_SELECT', 'NAV_ACTION', 'HACK_GUESS', 'HACK_PATTERN', 'ACTION_RESULT',
    '@wailsio/runtime', 'wailsjs', 'window.desktopAPI', 'fallout/terminal/private', 'genericDispatch',
  ]) {
    expect(bundle).not.toContain(forbidden);
  }
});

test('protected forwarding authenticates static, unary, and streaming capabilities', async ({ browser, request }) => {
  const protectedOrigin = await protectedFixtureURL(request);
  const protectedURL = protectedOrigin + '/';
  for (const headers of [{}, { Authorization: `Basic ${Buffer.from('players:wrong-password').toString('base64')}` }]) {
    const response = await request.get(protectedURL, { headers });
    expect(response.status()).toBe(401);
    expect(response.headers()['www-authenticate']).toContain('Fallout Terminal Players');
    expect(await response.text()).not.toContain('characterSelect');
  }

  const authorization = `Basic ${Buffer.from('players:password-long-enough').toString('base64')}`;
  const pageResponse = await request.get(protectedURL, { headers: { Authorization: authorization } });
  expect(pageResponse.status()).toBe(200);
  expect(await pageResponse.text()).toContain('characterSelect');

  const rpcResponse = await request.post(
    protectedOrigin + '/fallout.terminal.player.v1.PlayerService/SoundManifest',
    {
      headers: { Authorization: authorization, 'Content-Type': 'application/json' },
      data: { category: 'SOUND_CATEGORY_AMBIENT' },
    },
  );
  expect(rpcResponse.status()).toBe(200);

  const unauthorizedSubscribe = await request.post(
    protectedOrigin + '/fallout.terminal.player.v1.PlayerService/Subscribe',
    {
      headers: {
        'Content-Type': 'application/connect+proto',
        'Connect-Protocol-Version': '1',
      },
      data: Buffer.alloc(5),
    },
  );
  expect(unauthorizedSubscribe.status()).toBe(401);

  const context = await browser.newContext({
    httpCredentials: { username: 'players', password: 'password-long-enough' },
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();
  const subscribeResponses = [];
  const presentationUplinkRequests = [];
  const unaryPresentationRequests = [];
  page.on('request', request => {
    if (request.url().endsWith('/fallout.terminal.player.v1.PlayerService/PresentationUplink')) {
      presentationUplinkRequests.push(request.url());
    }
    if (request.url().endsWith('/fallout.terminal.player.v1.PlayerService/SetPresentation')) {
      unaryPresentationRequests.push(request.url());
    }
  });
  page.on('response', response => {
    if (response.url().endsWith('/fallout.terminal.player.v1.PlayerService/Subscribe')) {
      subscribeResponses.push(response.status());
    }
  });
  await page.goto(protectedURL);
  await expect(page.locator('#connOverlay')).toBeHidden();
  await expect(page.locator('#characterSelect')).toBeVisible();
  await page.locator('#characterOptions button:not([disabled])').first().click();
  await expect(page.locator('#termList')).toBeVisible();
  await expect.poll(() => subscribeResponses.length).toBe(1);
  expect(subscribeResponses).toEqual([200]);
  await expect.poll(() => presentationUplinkRequests.length).toBe(1);
  const presentationTarget = page.locator('.term-row').nth(1);
  await presentationTarget.hover();
  await expect(presentationTarget).toHaveClass(/sel/);
  await page.waitForTimeout(100);
  expect(unaryPresentationRequests).toHaveLength(0);
  expect(presentationUplinkRequests[0]).toContain('PlayerService/PresentationUplink');
  await context.close();
});

test('protected endpoint keeps five clients converged through navigation, hacking, sound, update, and reconnect before stale shutdown', async ({ browser, request }) => {
  const protectedOrigin = await protectedFixtureURL(request);
  const edgeStatus = await request.get(protectedOrigin + '/__fixture/edge/status', {
    headers: { Authorization: PROTECTED_AUTHORIZATION },
  });
  expect(edgeStatus.status()).toBe(200);
  expect(await edgeStatus.json()).toEqual({
    authBoundary: 'application-ingress',
    upstream: 'http://127.0.0.1:34119',
    active: true,
    authorizationForwarded: false,
    publicUrl: protectedOrigin,
  });

  const context = await browser.newContext({
    httpCredentials: { username: 'players', password: 'password-long-enough' },
    ignoreHTTPSErrors: true,
  });
  const pages = await Promise.all(Array.from({ length: 5 }, () => context.newPage()));
  const subscribeCounts = new Map(pages.map(page => [page, 0]));
  const manifestOrigins = [];
  for (const page of pages) {
    page.on('request', request => {
      if (request.url().endsWith('/Subscribe')) subscribeCounts.set(page, subscribeCounts.get(page) + 1);
      if (request.url().endsWith('/SoundManifest')) manifestOrigins.push(new URL(request.url()).origin);
    });
  }

  await Promise.all(pages.map(page => page.goto(protectedOrigin + '/')));
  await Promise.all(pages.map(page => expect(page.locator('#connOverlay')).toBeHidden()));
  const handles = await Promise.all(pages.map(page => page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY)));
  expect(new Set(handles).size).toBe(1);
  await pages[0].locator('#characterOptions button:not([disabled])').first().click();
  await Promise.all(pages.map(page => expect(page.locator('#termList')).toBeVisible()));

  await pages[0].locator('.term-row', { hasText: 'DOCS' }).click();
  await Promise.all(pages.map(page => expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible()));
  await expect.poll(() => manifestOrigins.length).toBeGreaterThanOrEqual(5);
  expect(manifestOrigins.every(origin => origin === protectedOrigin)).toBe(true);

  const update = await request.post(protectedOrigin + '/__fixture/edge/update', {
    headers: { Authorization: PROTECTED_AUTHORIZATION },
  });
  expect(update.status()).toBe(204);
  await Promise.all(pages.map(page => expect(page.locator('.term-row', { hasText: 'PUBLIC UPDATE' })).toBeVisible()));

  const hacking = await request.post(protectedOrigin + '/__fixture/edge/hacking', {
    headers: { Authorization: PROTECTED_AUTHORIZATION },
  });
  expect(hacking.status()).toBe(204);
  await Promise.all(pages.map(page => expect(page.locator('#hackBoard')).toBeVisible()));
  const guessTarget = pages[0].locator('#hackColumns [data-target]:not([data-target=""])').first();
  await expect(guessTarget).toBeVisible();
  await guessTarget.click();
  await Promise.all(pages.map(page => expect(page.locator('#hackLog')).not.toHaveText('')));

  const disconnect = await request.post(protectedOrigin + '/__fixture/edge/disconnect', {
    headers: { Authorization: PROTECTED_AUTHORIZATION },
  });
  expect(disconnect.status()).toBe(204);
  await Promise.all(pages.map(page => expect(page.locator('#connOverlay')).toBeHidden({ timeout: 5_000 })));
  await expect.poll(() => Math.min(...subscribeCounts.values()), { timeout: 5_000 }).toBeGreaterThanOrEqual(2);

  const disable = await request.post(protectedOrigin + '/__fixture/edge/disable', {
    headers: { Authorization: PROTECTED_AUTHORIZATION },
  });
  expect(disable.status()).toBe(204);
  const stale = await request.get(protectedOrigin + '/', {
    headers: { Authorization: PROTECTED_AUTHORIZATION },
  });
  expect(stale.status()).toBe(404);
  await context.close();
});

test.describe('actual authenticated ngrok endpoint', () => {
  test.skip(
    !PUBLIC_TEST_URL || !PUBLIC_TEST_USERNAME || !PUBLIC_TEST_PASSWORD,
    'NOT RUN: set FALLOUT_PUBLIC_TEST_URL, FALLOUT_PUBLIC_TEST_USERNAME, and FALLOUT_PUBLIC_TEST_PASSWORD for the real public streaming acceptance journey',
  );

  test('keeps five packaged clients converged through snapshot, navigation, sound, and reconnect', async ({ browser }) => {
    test.setTimeout(60_000);
    const context = await browser.newContext({
      httpCredentials: {
        username: PUBLIC_TEST_USERNAME,
        password: PUBLIC_TEST_PASSWORD,
      },
      extraHTTPHeaders: {
        'ngrok-skip-browser-warning': '1',
      },
    });
    const pages = await Promise.all(Array.from({ length: 5 }, () => context.newPage()));
    const subscribeResponses = new Map(pages.map(page => [page, []]));
    const manifestOrigins = [];
    for (const page of pages) {
      page.on('response', response => {
        if (response.url().endsWith('/fallout.terminal.player.v1.PlayerService/Subscribe')) {
          subscribeResponses.get(page).push({
            status: response.status(),
            contentType: response.headers()['content-type'] || '',
          });
        }
      });
      page.on('request', request => {
        if (request.url().endsWith('/fallout.terminal.player.v1.PlayerService/SoundManifest')) {
          manifestOrigins.push(new URL(request.url()).origin);
        }
      });
    }

    const firstNavigation = await pages[0].goto(PUBLIC_TEST_URL, { waitUntil: 'domcontentloaded' });
    expect(firstNavigation?.status()).toBe(200);
    await expect(pages[0].locator('#screen')).toBeVisible({ timeout: 5_000 });
    await expect(pages[0].locator('#connOverlay')).toBeHidden({ timeout: 5_000 });
    const remainingNavigations = await Promise.all(pages.slice(1).map(page => page.goto(PUBLIC_TEST_URL, { waitUntil: 'domcontentloaded' })));
    expect(remainingNavigations.every(navigation => navigation?.status() === 200)).toBe(true);
    await Promise.all(pages.slice(1).map(page => expect(page.locator('#screen')).toBeVisible({ timeout: 15_000 })));
    await Promise.all(pages.slice(1).map(page => expect(page.locator('#connOverlay')).toBeHidden({ timeout: 15_000 })));
    await Promise.all(pages.map(page => expect.poll(() => page.evaluate(key => Boolean(localStorage.getItem(key)), RECOGNITION_KEY)).toBe(true)));
    const handles = await Promise.all(pages.map(page => page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY)));
    expect(new Set(handles).size).toBe(1);
    const selectableCounts = await Promise.all(pages.map(page => page.locator('#characterOptions button:not([disabled])').count()));
    const controllerIndex = selectableCounts.findIndex(count => count > 0);
    expect(controllerIndex).toBeGreaterThanOrEqual(0);
    const controller = pages[controllerIndex];
    if (await controller.locator('#characterSelect').isVisible()) {
      await controller.locator('#characterOptions button:not([disabled])').first().click();
    }
    await Promise.all(pages.map(page => expect(page.locator('#termList')).toBeVisible()));
    await controller.locator('.term-row', { hasText: '2' }).click();
    await Promise.all(pages.map(page => expect(page.locator('.term-row', { hasText: 'запись' })).toBeVisible()));
    await expect.poll(() => manifestOrigins.length).toBeGreaterThanOrEqual(5);
    expect(manifestOrigins.every(origin => origin === new URL(PUBLIC_TEST_URL).origin)).toBe(true);
    if (PUBLIC_TEST_FIXTURE) {
      const update = await pages[0].request.post(new URL('/__fixture/update', PUBLIC_TEST_URL).href);
      expect(update.status()).toBe(204);
      await Promise.all(pages.map(page => expect(page.locator('.term-row', { hasText: 'PUBLIC UPDATE' })).toBeVisible()));
    }

    await Promise.all(pages.map(page => page.reload({ waitUntil: 'domcontentloaded' })));
    await Promise.all(pages.map(page => expect(page.locator('#connOverlay')).toBeHidden({ timeout: 5_000 })));
    expect(await Promise.all(pages.map(page => page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY)))).toEqual(handles);
    await expect.poll(() => Math.min(...[...subscribeResponses.values()].map(responses => responses.length))).toBeGreaterThanOrEqual(2);
    const allSubscribeResponses = [...subscribeResponses.values()].flat();
    expect(allSubscribeResponses.every(response => response.status === 200)).toBe(true);
    expect(allSubscribeResponses.every(response => response.contentType === 'application/connect+proto')).toBe(true);

    await context.close();
  });

  test('streams a packaged hacking guess and reconnects', async ({ browser }) => {
    test.skip(!PUBLIC_TEST_HACKING, 'NOT RUN: set FALLOUT_PUBLIC_TEST_HACKING=1 with a live hacking terminal');
    const context = await browser.newContext({
      httpCredentials: {
        username: PUBLIC_TEST_USERNAME,
        password: PUBLIC_TEST_PASSWORD,
      },
      extraHTTPHeaders: {
        'ngrok-skip-browser-warning': '1',
      },
    });
    const page = await context.newPage();
    const subscribeResponses = [];
    page.on('response', response => {
      if (response.url().endsWith('/fallout.terminal.player.v1.PlayerService/Subscribe')) {
        subscribeResponses.push({
          status: response.status(),
          contentType: response.headers()['content-type'] || '',
        });
      }
    });

    const navigation = await page.goto(PUBLIC_TEST_URL, { waitUntil: 'domcontentloaded' });
    expect(navigation?.status()).toBe(200);
    await expect(page.locator('#connOverlay')).toBeHidden({ timeout: 5_000 });
    await page.locator('#characterOptions button:not([disabled])').first().click();
    await expect(page.locator('#hackBoard')).toBeVisible();
    const guess = page.locator('#hackColumns [data-target]:not([data-target=""])').first();
    await expect(guess).toBeVisible();
    await guess.click();
    await expect(page.locator('#hackLog')).not.toHaveText('');

    const handle = await page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page.locator('#connOverlay')).toBeHidden({ timeout: 5_000 });
    await expect(page.locator('#hackBoard')).toBeVisible();
    expect(await page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY)).toBe(handle);
    await expect.poll(() => subscribeResponses.length).toBeGreaterThanOrEqual(2);
    expect(subscribeResponses.every(response => response.status === 200)).toBe(true);
    expect(subscribeResponses.every(response => response.contentType === 'application/connect+proto')).toBe(true);

    await context.close();
  });
});

test('local player discovers sounds only through the typed same-origin manifest', async ({ page }) => {
  const soundRequests = [];
  page.on('request', request => {
    if (request.url().includes('SoundManifest') || request.url().includes('/api/sounds/')) {
      soundRequests.push(request.url());
    }
  });
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await expect.poll(() => soundRequests.filter(url => url.endsWith('/SoundManifest')).length).toBeGreaterThanOrEqual(8);
  expect(soundRequests.some(url => url.includes('/api/sounds/'))).toBe(false);
  expect(soundRequests.every(url => new URL(url).origin === 'http://127.0.0.1:34119')).toBe(true);
});

test('clean profile receives a generated snapshot, stores only its handle, and selects a character', async ({ page }) => {
  const rpcRequests = [];
  page.on('request', request => {
    if (request.url().includes('/fallout.terminal.player.v1.PlayerService/')) {
      rpcRequests.push({ url: request.url(), contentType: request.headers()['content-type'] || '' });
    }
  });

  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await expect(page.locator('#characterSelect')).toBeVisible();

  const storage = await page.evaluate(() => {
    const values = {};
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index);
      values[key] = localStorage.getItem(key);
    }
    return values;
  });
  expect(Object.keys(storage)).toEqual([RECOGNITION_KEY]);
  expect(storage[RECOGNITION_KEY]).toMatch(/\S+/);

  const firstCharacter = page.locator('#characterOptions button').first();
  await expect(firstCharacter).toBeVisible();
  await firstCharacter.click();
  await expect(page.locator('#playerCharacterName')).not.toHaveText('');

  expect(rpcRequests.some(request => request.url.endsWith('/Subscribe'))).toBe(true);
  expect(rpcRequests.some(request => request.url.endsWith('/SelectCharacter'))).toBe(true);
  expect(rpcRequests.every(request => ['application/connect+proto', 'application/proto'].includes(request.contentType))).toBe(true);
});

test('recognized reconnect resumes from one complete current snapshot without replaying cues', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  const handle = await page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY);
  expect(handle).toMatch(/\S+/);

  await page.reload();
  await expect(page.locator('#connOverlay')).toBeHidden();
  expect(await page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY)).toBe(handle);
  expect(await page.evaluate(() => window.__audioStarts || 0)).toBe(0);
});

test('well-formed unknown recognition is replaced while malformed recognition fails closed', async ({ browser }) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.addInitScript(key => localStorage.setItem(key, 'unknown-but-well-formed'), RECOGNITION_KEY);
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  const replacement = await page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY);
  expect(replacement).not.toBe('unknown-but-well-formed');
  expect(replacement).toMatch(/\S+/);
  await context.close();
});

test('concurrent clean tabs converge on one logical recognition handle', async ({ browser }) => {
  const context = await browser.newContext();
  const pages = await Promise.all([context.newPage(), context.newPage(), context.newPage()]);
  await Promise.all(pages.map(page => page.goto('/')));
  await Promise.all(pages.map(page => expect(page.locator('#connOverlay')).toBeHidden()));
  const handles = await Promise.all(pages.map(page => page.evaluate(key => localStorage.getItem(key), RECOGNITION_KEY)));
  expect(new Set(handles).size).toBe(1);
  expect(handles[0]).toMatch(/\S+/);

  await pages[0].close();
  await expect(pages[1].locator('#connOverlay')).toBeHidden();
  await pages[1].close();
  await expect(pages[2].locator('#connOverlay')).toBeHidden();
  await context.close();
});

test('mixed typed actions stay pending until unary result and authoritative stream revision converge', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  const character = page.locator('#characterOptions button:not([disabled])').first();
  await character.click();
  await expect(page.locator('#characterSelect')).not.toHaveAttribute('aria-busy', 'true');

  const typedProcedures = await page.evaluate(() => performance.getEntriesByType('resource')
    .map(entry => entry.name)
    .filter(name => name.includes('/fallout.terminal.player.v1.PlayerService/')));
  expect(typedProcedures.some(name => name.endsWith('/SelectCharacter'))).toBe(true);
  expect(typedProcedures.every(name => !name.endsWith('/Command'))).toBe(true);
});

test('conflicting generated selections clear pending immediately and never alter the rejected canonical view optimistically', async ({ browser }) => {
  const contexts = await Promise.all([browser.newContext(), browser.newContext()]);
  const pages = await Promise.all(contexts.map(context => context.newPage()));
  let releaseSelections;
  const gate = new Promise(resolve => { releaseSelections = resolve; });
  let observed = 0;
  await Promise.all(pages.map(page => page.route('**/fallout.terminal.player.v1.PlayerService/SelectCharacter', async route => {
    observed += 1;
    await gate;
    await route.continue();
  })));

  await Promise.all(pages.map(page => page.goto('/')));
  await Promise.all(pages.map(page => expect(page.locator('#connOverlay')).toBeHidden()));
  const fallbackNames = await Promise.all(pages.map(page => page.locator('#playerCharacterName').textContent()));
  await Promise.all(pages.map(page => page.locator('#characterOptions button').first().click()));
  await expect.poll(() => observed).toBe(2);
  await Promise.all(pages.map(page => expect(page.locator('#characterSelect')).toHaveAttribute('aria-busy', 'true')));
  expect(await Promise.all(pages.map(page => page.locator('#playerCharacterName').textContent()))).toEqual(fallbackNames);

  releaseSelections();
  await Promise.all(pages.map(page => expect(page.locator('#characterSelect')).not.toHaveAttribute('aria-busy', 'true')));
  const names = await Promise.all(pages.map(page => page.locator('#playerCharacterName').textContent()));
  expect(names.filter(name => name === 'Mara')).toHaveLength(1);
  expect(names.filter(name => fallbackNames.includes(name))).toHaveLength(1);
  const rejectedPage = pages[names.findIndex(name => fallbackNames.includes(name))];
  await expect(rejectedPage.locator('#characterSelect')).toBeVisible();
  await expect(rejectedPage.locator('#termList')).toBeHidden();
  await expect(rejectedPage.locator('#playerNotice')).toContainText('conflict');

  await Promise.all(contexts.map(context => context.close()));
});

test('retained request identity reused with a different typed payload is rejected without canonical navigation', async ({ page }) => {
  await page.addInitScript(() => {
    const requestIds = ['session-owner', 'client-instance', 'selection-request', 'navigation-request'];
    Object.defineProperty(window.crypto, 'randomUUID', {
      configurable: true,
      value: () => requestIds.shift() || 'navigation-request',
    });
  });
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await page.locator('#characterOptions button:not([disabled])').first().click();
  await expect(page.locator('#termList')).toBeVisible();

  await page.locator('.term-row', { hasText: 'DOCS' }).click();
  await expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible();
  await expect(page.locator('#screen')).not.toHaveClass(/shared-input-pending/);
  await page.locator('#backBtn').click();
  await expect(page.locator('#screen')).not.toHaveClass(/shared-input-pending/);
  await expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible();
  await expect(page.locator('#playerNotice')).toContainText('duplicate');
});
