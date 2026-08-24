import { expect, test } from '@playwright/test';
import {
  installMovementCueDiagnostics,
  movementCueDiagnostics,
  resetMovementCueDiagnostics,
} from './helpers/movement-cue-diagnostics.mjs';

const PLAYER_SERVICE = '/fallout.terminal.player.v1.PlayerService/';

async function installLocalDiagnostics(context) {
  await context.addInitScript(() => {
    window.__fallbackAudioPlays = 0;
    window.__fallbackLegacySockets = 0;
    window.WebSocket = class ForbiddenLegacyPlayerTransport {
      constructor() {
        window.__fallbackLegacySockets += 1;
        throw new Error('local fallback must keep generated Connect transport');
      }
    };
    HTMLMediaElement.prototype.play = () => {
      window.__fallbackAudioPlays += 1;
      return Promise.resolve();
    };
  });
}

test.beforeEach(async ({ request }) => {
  const response = await request.post('/__fixture/reset');
  expect(response.status()).toBe(204);
});

test('direct HTTP and an HTTPS browser without request streams stay on bounded unary presentation', async ({ browser, request }) => {
  const direct = await browser.newPage();
  const directProcedures = [];
  direct.on('request', observed => {
    if (observed.url().includes(PLAYER_SERVICE)) directProcedures.push(observed.url());
  });
  await direct.goto('/');
  await expect(direct.locator('#connOverlay')).toBeHidden();
  await direct.locator('#characterOptions button:not([disabled])').first().click();
  await expect(direct.locator('#termList')).toBeVisible();
  await direct.locator('.term-row:not(.sel)').first().hover();
  await expect.poll(() => directProcedures.filter(url => url.endsWith('/SetPresentation')).length).toBe(1);
  expect(directProcedures.some(url => url.endsWith('/PresentationUplink'))).toBe(false);
  await direct.close();

  expect((await request.post('/__fixture/reset')).status()).toBe(204);

  const edgeStatus = await request.get('/__fixture/edge/status');
  const protectedOrigin = (await edgeStatus.json()).publicUrl;
  const unsupportedContext = await browser.newContext({
    httpCredentials: { username: 'players', password: 'password-long-enough' },
    ignoreHTTPSErrors: true,
  });
  await unsupportedContext.addInitScript(() => {
    Object.defineProperty(window, 'Request', {
      configurable: true,
      value: class UnsupportedStreamingRequest {
        constructor() { throw new TypeError('synthetic request-stream support missing'); }
      },
    });
  });
  const unsupported = await unsupportedContext.newPage();
  const unsupportedProcedures = [];
  unsupported.on('request', observed => {
    if (observed.url().includes(PLAYER_SERVICE)) unsupportedProcedures.push(observed.url());
  });
  await unsupported.goto(protectedOrigin + '/');
  await expect(unsupported.locator('#connOverlay')).toBeHidden();
  await unsupported.locator('#characterOptions button:not([disabled])').first().click();
  await expect(unsupported.locator('#termList')).toBeVisible();
  await unsupported.locator('.term-row:not(.sel)').first().hover();
  await expect.poll(() => unsupportedProcedures.filter(url => url.endsWith('/SetPresentation')).length).toBe(1);
  expect(unsupportedProcedures.some(url => url.endsWith('/PresentationUplink'))).toBe(false);
  await unsupportedContext.close();
});

test('failed HTTPS stream probe falls back without blocking control', async ({ browser, request }) => {
  const edgeStatus = await request.get('/__fixture/edge/status');
  const protectedOrigin = (await edgeStatus.json()).publicUrl;
  const context = await browser.newContext({
    httpCredentials: { username: 'players', password: 'password-long-enough' },
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();
  let uplinkAttempts = 0;
  let unaryPresentations = 0;
  await page.route(`**${PLAYER_SERVICE}PresentationUplink`, async route => {
    uplinkAttempts += 1;
    await route.abort('failed');
  });
  page.on('request', observed => {
    if (observed.url().endsWith(`${PLAYER_SERVICE}SetPresentation`)) unaryPresentations += 1;
  });
  await page.goto(protectedOrigin + '/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await expect.poll(() => uplinkAttempts).toBeGreaterThanOrEqual(1);
  await page.locator('#characterOptions button:not([disabled])').first().click();
  await expect(page.locator('#termList')).toBeVisible();
  await page.locator('.term-row').nth(1).hover();
  await expect.poll(() => unaryPresentations).toBe(1);
  await expect(page.locator('.term-row').nth(1)).toHaveClass(/sel/);
  await context.close();
});

test('stream cancellation preserves the surviving cue and later fresh-generation cues', async ({ browser, request }) => {
  const edgeStatus = await request.get('/__fixture/edge/status');
  const protectedOrigin = (await edgeStatus.json()).publicUrl;
  const context = await browser.newContext({
    httpCredentials: { username: 'players', password: 'password-long-enough' },
    ignoreHTTPSErrors: true,
  });
  await installMovementCueDiagnostics(context);
  const page = await context.newPage();
  let uplinkRequests = 0;
  let unaryPresentations = 0;
  page.on('request', observed => {
    if (observed.url().endsWith(`${PLAYER_SERVICE}PresentationUplink`)) uplinkRequests += 1;
    if (observed.url().endsWith(`${PLAYER_SERVICE}SetPresentation`)) unaryPresentations += 1;
  });
  await page.goto(protectedOrigin + '/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await page.locator('#characterOptions button:not([disabled])').first().click();
  await expect(page.locator('#termList')).toBeVisible();
  expect((await request.post('/__fixture/local/crt/content')).status()).toBe(204);
  await expect(page.locator('.term-row')).toHaveCount(25);
  await expect.poll(() => uplinkRequests).toBeGreaterThanOrEqual(1);
  await page.waitForTimeout(250);

  await resetMovementCueDiagnostics(page);
  expect((await request.post('/__fixture/edge/presentation-gate/arm')).status()).toBe(204);
  const rows = page.locator('.term-row');
  const unselectedRows = page.locator('.term-row:not(.sel)');
  await expect.poll(() => unselectedRows.count()).toBeGreaterThanOrEqual(3);
  await unselectedRows.nth(0).hover();
  await expect.poll(async () => (await request.get('/__fixture/edge/presentation-gate/blocked')).status()).toBe(204);
  const fallbackTarget = unselectedRows.nth(1);
  const fallbackText = (await fallbackTarget.textContent()).trim();
  await fallbackTarget.hover();
  expect((await request.post('/__fixture/edge/presentation-gate/cancel-uplinks')).status()).toBe(204);
  await expect(page.locator('.term-row.sel')).toHaveText(fallbackText);
  await expect.poll(() => unaryPresentations).toBeGreaterThanOrEqual(1);
  await expect.poll(async () => (await movementCueDiagnostics(page)).urls
    .filter(url => url.includes('/sounds/menu-focus/')).length).toBe(1);

  await expect.poll(() => uplinkRequests, { timeout: 5_000 }).toBeGreaterThanOrEqual(2);
  await page.waitForTimeout(250);
  const unaryBeforeRecovery = unaryPresentations;
  await resetMovementCueDiagnostics(page);
  const recoveredTarget = page.locator('.term-row:not(.sel)').nth(0);
  const recoveredText = (await recoveredTarget.textContent()).trim();
  await recoveredTarget.hover();
  await expect(page.locator('.term-row.sel')).toHaveText(recoveredText);
  await expect.poll(async () => (await movementCueDiagnostics(page)).urls
    .filter(url => url.includes('/sounds/menu-focus/')).length).toBe(1);
  expect(unaryPresentations).toBe(unaryBeforeRecovery);

  expect((await request.post('/__fixture/edge/hacking')).status()).toBe(204);
  await expect(page.locator('#hackBoard')).toBeVisible();
  await resetMovementCueDiagnostics(page);
  const recoveredHackTarget = page.locator('.hcell.word').last();
  await recoveredHackTarget.hover();
  await expect.poll(async () => (await movementCueDiagnostics(page)).urls
    .filter(url => /\/sounds\/(?:single|multiple)\//.test(url)).length).toBe(1);
  expect((await movementCueDiagnostics(page)).urls
    .filter(url => /\/sounds\/(?:single|multiple)\//.test(url))).toHaveLength(1);

  await context.close();
});

test('all public failures leave local gameplay live and a later public generation recovers without restart', async ({ browser, request }) => {
  const playerContext = await browser.newContext();
  await installLocalDiagnostics(playerContext);
  const player = await playerContext.newPage();
  let subscribeCount = 0;
  let soundManifestCount = 0;
  player.on('request', observed => {
    if (!observed.url().includes(PLAYER_SERVICE)) return;
    if (observed.url().endsWith('/Subscribe')) subscribeCount += 1;
    if (observed.url().endsWith('/SoundManifest')) soundManifestCount += 1;
  });
  await player.goto('/');
  await expect(player.locator('#connOverlay')).toBeHidden();
  const character = player.locator('#characterOptions button:not([disabled])').first();
  const characterName = (await character.textContent()).trim();
  await character.click();
  await expect(player.locator('#playerCharacterName')).toHaveText(characterName);
  await expect(player.locator('#termList')).toBeVisible();

  const overseer = await browser.newPage();
  await overseer.goto('/__fixture/public-access-settings');
  await expect(overseer.locator('#publicAccessSection')).toBeVisible();

  const failures = [
    'invalid-token', 'revoked-token', 'no-network', 'dns-timeout', 'domain-conflict',
    'keychain-locked', 'keychain-denied', 'keychain-unavailable', 'policy-failure',
    'provider-failure', 'unexpected-done', 'close-failure', 'stale-completion',
  ];
  for (const [index, failure] of failures.entries()) {
    const response = await request.post(`/__fixture/public-access/failure/${failure}`);
    expect(response.status(), failure).toBe(200);
    const snapshot = await response.json();
    await overseer.evaluate(value => __desktopFixture.emit('public-access-status', value), snapshot);
    await expect(overseer.locator('#publicAccessStatus')).toHaveText('ОШИБКА');
    await expect(overseer.locator('#publicAccessURL')).toHaveText('');
    await expect(overseer.locator('#publicAccessError')).toContainText('ЛОКАЛЬНЫЙ РЕЖИМ ПРОДОЛЖАЕТ РАБОТАТЬ');

    await expect(player.locator('#connOverlay')).toBeHidden();
    await expect(player.locator('#playerCharacterName')).toHaveText(characterName);
    if (index % 2 === 0) {
      await player.locator('.term-row', { hasText: 'DOCS' }).click();
      await expect(player.locator('.term-row', { hasText: 'REPORT' })).toBeVisible();
      await expect(player.locator('#screen')).not.toHaveClass(/shared-input-pending/);
    } else {
      await player.locator('#backBtn').click();
      await expect(player.locator('.term-row', { hasText: 'DOCS' })).toBeVisible();
    }
  }

  const hacking = await request.post('/__fixture/local/hacking');
  expect(hacking.status()).toBe(204);
  await expect(player.locator('#hackBoard')).toBeVisible();
  const guess = player.locator('#hackColumns [data-target]:not([data-target=""])').first();
  await expect(guess).toBeVisible();
  await guess.click();
  await expect(player.locator('#hackLog')).not.toHaveText('');

  const disconnect = await request.post('/__fixture/local/disconnect');
  expect(disconnect.status()).toBe(204);
  await expect(player.locator('#connOverlay')).toBeHidden({ timeout: 5_000 });
  await expect.poll(() => subscribeCount, { timeout: 5_000 }).toBeGreaterThanOrEqual(2);
  await expect.poll(() => soundManifestCount).toBeGreaterThan(0);
  expect(await player.evaluate(() => window.__fallbackLegacySockets)).toBe(0);

  const recoveryResponse = await request.post('/__fixture/public-access/recover');
  expect(recoveryResponse.status()).toBe(200);
  const recovered = await recoveryResponse.json();
  await overseer.evaluate(value => __desktopFixture.emit('public-access-status', value), recovered);
  await expect(overseer.locator('#publicAccessStatus')).toHaveText('ГОТОВ');
  await expect(overseer.locator('#publicAccessURL')).toHaveText('https://recovered.example');
  await expect(player.locator('#connOverlay')).toBeHidden();
  await expect(player.locator('#playerCharacterName')).toHaveText(characterName);

  await overseer.close();
  await playerContext.close();
});
