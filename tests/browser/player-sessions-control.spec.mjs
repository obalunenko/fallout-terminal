import { expect, test } from '@playwright/test';
import {
  installMovementCueDiagnostics,
  movementCueDiagnostics,
  resetMovementCueDiagnostics,
} from './helpers/movement-cue-diagnostics.mjs';

const overseerAppModuleURL = 'http://127.0.0.1:34121/@fs' + new URL('./fixtures/overseer-app.ts', import.meta.url).pathname;

const TOKEN_KEY = 'fallout-terminal.player-token';
const PLAYER_SERVICE = '/fallout.terminal.player.v1.PlayerService/';
const OVERSEER_AUTHORING_FIXTURE = '/__fixture/state-changing-command-authoring';

test.beforeEach(async ({ request }) => {
  const response = await request.post('/__fixture/reset');
  expect(response.status()).toBe(204);
});

async function installPlayerDiagnostics(target, storedToken = null, { audioFailure = false } = {}) {
  await target.addInitScript(({ tokenKey, token, failAudio }) => {
    window.__webSocketConstructions = 0;
    window.__playerWarnings = [];
    const originalWarn = console.warn.bind(console);
    console.warn = (...values) => {
      window.__playerWarnings.push(values.map(value => String(value)).join(' '));
      originalWarn(...values);
    };
    window.WebSocket = class ForbiddenLegacyPlayerTransport {
      constructor() {
        window.__webSocketConstructions += 1;
        throw new Error('the player must not construct a WebSocket');
      }
    };
    if (token !== null && localStorage.getItem(tokenKey) === null) {
      localStorage.setItem(tokenKey, token);
    }

    if (failAudio) {
      window.AudioContext = class UnavailableAudioContext {
        constructor() { throw new Error('simulated audio-device failure'); }
      };
      HTMLMediaElement.prototype.play = () => Promise.reject(new DOMException('blocked', 'NotAllowedError'));
    } else {
      HTMLMediaElement.prototype.play = () => Promise.resolve();
    }
  }, { tokenKey: TOKEN_KEY, token: storedToken, failAudio: audioFailure });
}

async function openPlayer(page, storedToken = null, options = {}) {
  await installPlayerDiagnostics(page, storedToken, options);
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await expect.poll(() => page.evaluate(() => window.__webSocketConstructions)).toBe(0);
}

async function mountOverseerFixture(page, instance) {
  await page.goto(OVERSEER_AUTHORING_FIXTURE);
  await page.evaluate(
    ({ key, moduleURL }) => import(`${moduleURL}?session-document-${key}`),
    { key: instance, moduleURL: overseerAppModuleURL },
  );
}

async function selectFirstAvailable(page) {
  const character = page.locator('#characterOptions button:not([disabled])').first();
  await expect(character).toBeVisible();
  const name = await character.textContent();
  await character.click();
  await expect(page.locator('#characterSelect')).not.toHaveAttribute('aria-busy', 'true');
  await expect(page.locator('#playerCharacterName')).toHaveText(name.trim());
  return name.trim();
}

async function submitUnsuccessfulHackGuess(controller, observers = []) {
  const initialAttempts = await controller.locator('#attemptsLine').textContent();
  await controller.locator('.hcell.word').first().click();
  await expect.poll(async () => controller.locator('#attemptsLine').textContent()).not.toBe(initialAttempts);
  const retainedAttempts = await controller.locator('#attemptsLine').textContent();
  await Promise.all(observers.map(observer =>
    expect(observer.locator('#attemptsLine')).toHaveText(retainedAttempts)));
}

async function expectNoPlayerNotice(pages) {
  await Promise.all(pages.map(page => expect(page.locator('#playerNotice')).toBeHidden()));
}

function typedPlayerRequests(page) {
  const requests = [];
  page.on('request', request => {
    if (!request.url().includes(PLAYER_SERVICE)) return;
    requests.push({
      procedure: new URL(request.url()).pathname.split('/').at(-1),
      contentType: request.headers()['content-type'] || '',
    });
  });
  return requests;
}

function trackFirstPartyPageDiagnostics(page) {
  const consoleEntries = [];
  const failedResponses = [];
  const pageErrors = [];

  page.on('console', message => {
    if (!['warning', 'error'].includes(message.type())) return;
    consoleEntries.push({
      type: message.type(),
      text: message.text(),
      url: message.location().url,
    });
  });
  page.on('response', response => {
    if (response.status() < 400) return;
    failedResponses.push({ status: response.status(), url: response.url() });
  });
  page.on('pageerror', error => pageErrors.push(error.message));

  return () => {
    const documentURL = new URL(page.url());
    const isFirstParty = rawURL => {
      if (!rawURL) return true;
      try {
        return new URL(rawURL, documentURL).origin === documentURL.origin;
      } catch {
        return false;
      }
    };
    return {
      consoleEntries: consoleEntries.filter(entry => isFirstParty(entry.url)),
      failedResponses: failedResponses.filter(entry => isFirstParty(entry.url)),
      pageErrors,
    };
  };
}

test('session document controls preserve open new save close and stale completion', async ({ browser }) => {
  const context = await browser.newContext({ bypassCSP: true });

  const openPage = await context.newPage();
  await mountOverseerFixture(openPage, 'open');
  await openPage.locator('#btnOpenSession').click();
  await expect(openPage.locator('#mainLayout')).toBeVisible();
  await expect(openPage.locator('#sessionFileLabel')).toHaveText('/private/tmp/fallout-state-changing-authoring.json');
  await openPage.locator('#hackLevelSelect').selectOption('1');
  await openPage.locator('#btnApplySettings').click();
  await expect.poll(() => openPage.evaluate(() => (
    __desktopFixture.calls.filter(call => call.method === 'SaveSession').length
  ))).toBe(1);

  const newPage = await context.newPage();
  await mountOverseerFixture(newPage, 'new');
  await newPage.locator('#btnNewSession').click();
  await expect(newPage.locator('#mainLayout')).toBeVisible();
  await expect(newPage.locator('#sessionFileLabel')).toHaveText('/private/tmp/fallout-state-changing-authoring-new.json');
  await expect.poll(() => newPage.evaluate(() => (
    __desktopFixture.calls.filter(call => call.method === 'NewSession').length
  ))).toBe(1);

  const stalePage = await context.newPage();
  let releaseSession;
  const sessionGate = new Promise(resolve => { releaseSession = resolve; });
  let sessionRequested = false;
  await stalePage.route(`**${OVERSEER_AUTHORING_FIXTURE}/session`, async route => {
    sessionRequested = true;
    await sessionGate;
    await route.continue();
  });
  await mountOverseerFixture(stalePage, 'stale');
  await stalePage.locator('#btnOpenSession').click();
  await expect.poll(() => sessionRequested).toBe(true);
  await stalePage.evaluate(() => {
    __overseerAppFixture.unmount();
    __overseerAppFixture.unmount();
  });
  await expect(stalePage.locator('#overseerApp')).toBeEmpty();

  const sessionResponse = stalePage.waitForResponse(response => (
    response.url().endsWith(`${OVERSEER_AUTHORING_FIXTURE}/session`)
  ));
  releaseSession();
  await sessionResponse;
  await stalePage.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  await expect(stalePage.locator('#mainLayout')).toBeHidden();

  await context.close();
});

test('logical-session rows ignore stale close and rebind', async ({ browser }) => {
  const context = await browser.newContext({ bypassCSP: true });
  const page = await context.newPage();
  await mountOverseerFixture(page, 'logical-sessions');
  await page.locator('#btnOpenSession').click();
  await expect(page.locator('#mainLayout')).toBeVisible();

  const roster = [
    { id: 'character-1', name: 'Амата', claimedBySessionId: 'session-1' },
    { id: 'character-2', name: 'Буч', claimedBySessionId: '' },
  ];
  const sessions = [
    {
      id: 'session-1', fallbackName: 'Убежище 101', connected: true, role: 'active',
      character: { id: 'character-1', name: 'Амата' },
    },
    {
      id: 'session-2', fallbackName: 'Пип-Бой гостя', connected: true, role: 'observer', character: null,
    },
  ];
  const emitCoordination = (revision, nextSessions) => page.evaluate(({ nextRevision, nextRoster, values }) => {
    __desktopFixture.emit('coordination-state', {
      revision: nextRevision,
      broadcast: { id: 'broadcast-logical', activeTerminalId: 'terminal-stateful' },
      playerConfig: { name: 'Игроки', filePath: '/private/tmp/players.json' },
      roster: nextRoster,
      sessions: values,
    });
  }, { nextRevision: revision, nextRoster: roster, values: nextSessions });

  await emitCoordination(90, sessions);
  const opener = page.locator('#btnManageLogicalSessions');
  const dialog = page.locator('#logicalSessionDialog');
  await opener.click();
  await expect(dialog).toBeVisible();
  const retainedRow = dialog.locator('[data-session-id="session-2"]');
  await retainedRow.evaluate(element => { element.dataset.stableMarker = 'retained'; });
  await emitCoordination(91, [sessions[1], sessions[0]]);
  await expect(retainedRow).toHaveAttribute('data-stable-marker', 'retained');

  await page.evaluate(() => {
    const row = document.querySelector('[data-session-id="session-2"]');
    const input = row.querySelector('.session-name-input');
    input.value = 'Закрытая сессия';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    row.querySelector('.session-rename').click();
    document.querySelector('#btnCloseLogicalSessions').click();
  });
  await expect(dialog).toBeHidden();
  await expect(opener).toBeFocused();

  await opener.click();
  await expect(dialog).toBeVisible();
  await page.evaluate(({ nextRoster }) => {
    const row = document.querySelector('[data-session-id="session-2"]');
    const input = row.querySelector('.session-name-input');
    input.value = 'Устаревшая сессия';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    row.querySelector('.session-rename').click();
    __desktopFixture.emit('coordination-state', {
      revision: 92,
      broadcast: { id: 'broadcast-logical', activeTerminalId: 'terminal-stateful' },
      playerConfig: { name: 'Игроки', filePath: '/private/tmp/players.json' },
      roster: nextRoster,
      sessions: [{
        id: 'session-3', fallbackName: 'Новая сессия', connected: true, role: 'observer', character: null,
      }],
    });
  }, { nextRoster: roster });
  await expect(dialog.locator('[data-session-id="session-2"]')).toHaveCount(0);
  await expect(dialog.locator('[data-session-id="session-3"]')).toHaveCount(1);
  await expect(page.locator('#logicalSessionDialogStatus')).toBeEmpty();
  await expect(page.locator('#logicalSessionDialogError')).toBeHidden();

  await page.evaluate(() => {
    const row = document.querySelector('[data-session-id="session-3"]');
    row.querySelector('.session-rename').click();
    __overseerAppFixture.unmount();
  });
  await expect(page.locator('#overseerApp')).toBeEmpty();

  await context.close();
});

test('initial player page has no first-party console or static-request diagnostics', async ({ page }) => {
  const diagnostics = trackFirstPartyPageDiagnostics(page);

  await openPlayer(page);
  await expect(page.locator('meta[http-equiv="Content-Security-Policy"]')).not.toHaveAttribute('content', /frame-ancestors/);
  await expect(page.locator('link[rel~="icon"]')).toHaveAttribute('href', 'data:,');
  await page.evaluate(() => new Promise(resolve => {
    requestAnimationFrame(() => requestAnimationFrame(resolve));
  }));

  expect(diagnostics()).toEqual({
    consoleEntries: [],
    failedResponses: [],
    pageErrors: [],
  });
});

test('selection uses a generated unary procedure and remains pending until its typed result converges with the stream', async ({ page }) => {
  const requests = typedPlayerRequests(page);
  let releaseSelection;
  const selectionGate = new Promise(resolve => { releaseSelection = resolve; });
  let selectionObserved = false;
  await page.route(`**${PLAYER_SERVICE}SelectCharacter`, async route => {
    selectionObserved = true;
    await selectionGate;
    await route.continue();
  });

  await openPlayer(page);
  const character = page.locator('#characterOptions button:not([disabled])').first();
  const expectedName = (await character.textContent()).trim();
  await character.click();
  await expect.poll(() => selectionObserved).toBe(true);
  await expect(page.locator('#characterSelect')).toHaveAttribute('aria-busy', 'true');
  await expect(page.locator('#playerCharacterName')).not.toHaveText(expectedName);

  releaseSelection();
  await expect(page.locator('#characterSelect')).not.toHaveAttribute('aria-busy', 'true');
  await expect(page.locator('#playerCharacterName')).toHaveText(expectedName);
  expect(requests.map(request => request.procedure)).toContain('Subscribe');
  expect(requests.map(request => request.procedure)).toContain('SelectCharacter');
  expect(requests.every(request => ['application/connect+proto', 'application/proto'].includes(request.contentType))).toBe(true);
});

test('active navigation applies the authoritative compound update while awaiting the delayed unary result', async ({ page }) => {
  const requests = typedPlayerRequests(page);
  await openPlayer(page);
  await selectFirstAvailable(page);
  await expect(page.locator('#termList')).toBeVisible();

  let releaseNavigate;
  const navigateGate = new Promise(resolve => { releaseNavigate = resolve; });
  let navigateObserved = false;
  await page.route(`**${PLAYER_SERVICE}Navigate`, async route => {
    navigateObserved = true;
    await navigateGate;
    await route.continue();
  });

  await page.locator('.term-row', { hasText: 'DOCS' }).click();
  await expect.poll(() => navigateObserved).toBe(true);
  await expect(page.locator('#screen')).toHaveClass(/shared-input-pending/);
  await expect(page.locator('.term-row').first()).toHaveCSS('cursor', 'not-allowed');
  await expect(page.locator('.term-row:not(.sel)').first()).toHaveCSS('opacity', '0.72');
  releaseNavigate();

  await expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible();
  await expect(page.locator('#screen')).not.toHaveClass(/shared-input-pending/);
  await expect(page.locator('#connOverlay')).toBeHidden();
  expect(requests.map(request => request.procedure)).toEqual(expect.arrayContaining([
    'Subscribe', 'SelectCharacter', 'Navigate',
  ]));
});

test('three tabs share one recognition handle and converge after one generated selection', async ({ browser }) => {
  const context = await browser.newContext();
  await installPlayerDiagnostics(context);
  const pages = await Promise.all([context.newPage(), context.newPage(), context.newPage()]);
  await Promise.all(pages.map(page => page.goto('/')));
  await Promise.all(pages.map(page => expect(page.locator('#connOverlay')).toBeHidden()));

  const handles = await Promise.all(pages.map(page => page.evaluate(key => localStorage.getItem(key), TOKEN_KEY)));
  expect(new Set(handles).size).toBe(1);
  const characterName = await selectFirstAvailable(pages[0]);
  await Promise.all(pages.map(page => expect(page.locator('#playerCharacterName')).toHaveText(characterName)));

  await pages[0].close();
  await expect(pages[1].locator('#termList')).toBeVisible();
  await pages[1].close();
  await expect(pages[2].locator('#termList')).toBeVisible();
  await context.close();
});

test('four through seven generated players converge across mixed navigation, reconnect, replay, and sound-safe state', async ({ browser, request }) => {
  let acceptedActions = 0;
  let reconnects = 0;

  for (let playerCount = 4; playerCount <= 7; playerCount += 1) {
    const reset = await request.post('/__fixture/reset');
    expect(reset.status()).toBe(204);
    const contexts = await Promise.all(Array.from({ length: playerCount }, () => browser.newContext()));
    await Promise.all(contexts.map(context => installPlayerDiagnostics(context)));
    const pages = await Promise.all(contexts.map(context => context.newPage()));
    await Promise.all(pages.map(page => page.goto('/')));
    await Promise.all(pages.map(page => expect(page.locator('#connOverlay')).toBeHidden()));

    const handles = await Promise.all(pages.map(page => page.evaluate(key => localStorage.getItem(key), TOKEN_KEY)));
    expect(new Set(handles).size).toBe(playerCount);
    const characters = [];
    for (const page of pages) characters.push(await selectFirstAvailable(page));
    expect(new Set(characters).size).toBe(playerCount);
    await expect(pages[0].locator('#roleBadge')).toContainText('АКТИВЕН');
    await Promise.all(pages.slice(1).map(page => expect(page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ')));
    const observerRequests = typedPlayerRequests(pages[1]);

    for (let round = 0; round < 4; round += 1) {
      await pages[0].locator('.term-row', { hasText: 'DOCS' }).click();
      await Promise.all(pages.map(page => expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible()));
      await expect(pages[0].locator('#screen')).not.toHaveClass(/shared-input-pending/);
      acceptedActions += 1;

      const observerRequestCount = observerRequests.length;
      await pages[1].locator('#backBtn').click({ force: true });
      expect(observerRequests.length).toBe(observerRequestCount);
      await pages[0].locator('#backBtn').click();
      await Promise.all(pages.map(page => expect(page.locator('.term-row', { hasText: 'DOCS' })).toBeVisible()));
      acceptedActions += 1;
    }

    if (reconnects < 3) {
      const page = pages[reconnects % playerCount];
      const handle = await page.evaluate(key => localStorage.getItem(key), TOKEN_KEY);
      await page.reload();
      await expect(page.locator('#connOverlay')).toBeHidden();
      expect(await page.evaluate(key => localStorage.getItem(key), TOKEN_KEY)).toBe(handle);
      expect(await page.evaluate(() => window.__audioStarts || 0)).toBe(0);
      reconnects += 1;
    }

    for (const page of pages) {
      const storage = await page.evaluate(() => Object.fromEntries(
        Array.from({ length: localStorage.length }, (_, index) => {
          const key = localStorage.key(index);
          return [key, localStorage.getItem(key)];
        }),
      ));
      expect(Object.keys(storage)).toEqual([TOKEN_KEY]);
    }
    await Promise.all(contexts.map(context => context.close()));
  }

  expect(acceptedActions).toBeGreaterThanOrEqual(25);
  expect(reconnects).toBe(3);
});

test('recognized reload retains identity while an unknown opaque handle receives a safe replacement', async ({ browser, page }) => {
  await openPlayer(page);
  const handle = await page.evaluate(key => localStorage.getItem(key), TOKEN_KEY);
  expect(handle).toMatch(/\S+/);

  await page.reload();
  await expect(page.locator('#connOverlay')).toBeHidden();
  expect(await page.evaluate(key => localStorage.getItem(key), TOKEN_KEY)).toBe(handle);

  const staleContext = await browser.newContext();
  await installPlayerDiagnostics(staleContext, 'unknown-but-well-formed');
  const stalePage = await staleContext.newPage();
  await stalePage.goto('/');
  await expect(stalePage.locator('#connOverlay')).toBeHidden();
  const replacement = await stalePage.evaluate(key => localStorage.getItem(key), TOKEN_KEY);
  expect(replacement).not.toBe('unknown-but-well-formed');
  expect(replacement).toMatch(/\S+/);
  await staleContext.close();
});

test('observer projection is visibly read-only and emits no typed mutation', async ({ browser }) => {
  const controllerContext = await browser.newContext();
  await installPlayerDiagnostics(controllerContext);
  const controller = await controllerContext.newPage();
  await controller.goto('/');
  await expect(controller.locator('#connOverlay')).toBeHidden();
  await selectFirstAvailable(controller);

  const observerContext = await browser.newContext();
  await installPlayerDiagnostics(observerContext);
  const observer = await observerContext.newPage();
  const observerRequests = typedPlayerRequests(observer);
  await observer.goto('/');
  await expect(observer.locator('#connOverlay')).toBeHidden();
  await selectFirstAvailable(observer);
  await expect(observer.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
  await expect(observer.locator('#screen')).toHaveAttribute('aria-readonly', 'true');
  await expect(observer.locator('.term-row').first()).toHaveCSS('cursor', 'not-allowed');
  await expect(observer.locator('.term-row:not(.sel)').first()).toHaveCSS('opacity', '0.72');

  const before = observerRequests.length;
  await observer.locator('.term-row', { hasText: 'DOCS' }).click({ force: true });
  await observer.waitForTimeout(100);
  expect(observerRequests.slice(before).map(request => request.procedure)).not.toContain('Navigate');
  await expect(observer.locator('.term-row', { hasText: 'DOCS' })).toBeVisible();

  await observerContext.close();
  await controllerContext.close();
});

test('controller selection is authoritative while observer pointer and keyboard input stay inert', async ({ browser }) => {
  const controllerContext = await browser.newContext();
  const observerContext = await browser.newContext();
  await installPlayerDiagnostics(controllerContext);
  await installPlayerDiagnostics(observerContext);
  const controller = await controllerContext.newPage();
  const observer = await observerContext.newPage();
  const controllerRequests = typedPlayerRequests(controller);
  const observerRequests = typedPlayerRequests(observer);

  await Promise.all([controller.goto('/'), observer.goto('/')]);
  await Promise.all([
    expect(controller.locator('#connOverlay')).toBeHidden(),
    expect(observer.locator('#connOverlay')).toBeHidden(),
  ]);
  await selectFirstAvailable(controller);
  await selectFirstAvailable(observer);
  await expect(controller.locator('#roleBadge')).toHaveAttribute('data-role', 'active');
  await expect(observer.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');

  await controller.locator('.term-row', { hasText: 'STATUS' }).hover();
  await expect.poll(() => controllerRequests.map(request => request.procedure)).toContain('SetPresentation');
  expect(await controller.evaluate(() => window.__playerWarnings)).toEqual([]);
  await expect(controller.locator('#playerNotice')).toHaveText('');
  await expect(controller.locator('.term-row.sel')).toContainText('STATUS');
  await expect(observer.locator('.term-row.sel')).toContainText('STATUS');

  const requestCount = observerRequests.length;
  await observer.evaluate(() => {
    const rows = Array.from(document.querySelectorAll('.term-row'));
    for (let attempt = 0; attempt < 100; attempt += 1) {
      rows[attempt % rows.length].dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
      document.dispatchEvent(new KeyboardEvent('keydown', {
        key: attempt % 2 === 0 ? 'ArrowUp' : 'ArrowDown',
        bubbles: true,
        cancelable: true,
      }));
    }
  });
  await expect(observer.locator('.term-row.sel')).toContainText('STATUS');
  expect(observerRequests).toHaveLength(requestCount);

  await observerContext.close();
  await controllerContext.close();
});

test('presentation-only latency keeps the active controller visually actionable and accepts newer movement', async ({ page, request }) => {
  const requests = typedPlayerRequests(page);
  await openPlayer(page);
  await selectFirstAvailable(page);
  await expect(page.locator('#roleBadge')).toHaveAttribute('data-role', 'active');
  expect((await request.post('/__fixture/local/crt/content')).status()).toBe(204);

  const rows = page.locator('.term-row');
  await expect.poll(() => rows.count()).toBeGreaterThanOrEqual(3);
  const rowCount = await rows.count();
  const initialIndex = await rows.evaluateAll(items => items.findIndex(item => item.classList.contains('sel')));
  const firstTargetIndex = (initialIndex + 1) % rowCount;
  const firstTarget = rows.nth(firstTargetIndex);
  const latestTarget = rows.nth((initialIndex + 2) % rowCount);
  const latestText = (await latestTarget.innerText()).trim();

  await page.evaluate(() => {
    const screen = document.querySelector('#screen');
    window.__bug009BlockingTransitions = 0;
    window.__bug009ClassObserver = new MutationObserver(() => {
      if (screen.classList.contains('shared-input-pending')) {
        window.__bug009BlockingTransitions += 1;
      }
    });
    window.__bug009ClassObserver.observe(screen, { attributes: true, attributeFilter: ['class'] });
  });

  let releasePresentation;
  const presentationGate = new Promise(resolve => { releasePresentation = resolve; });
  let presentationRequests = 0;
  await page.route(`**${PLAYER_SERVICE}SetPresentation`, async route => {
    presentationRequests += 1;
    await presentationGate;
    await route.continue();
  });

  try {
    await firstTarget.hover();
    await expect.poll(() => presentationRequests).toBe(1);
    await page.evaluate(targetIndex => {
      const target = document.querySelectorAll('.term-row')[targetIndex];
      for (let attempt = 0; attempt < 100; attempt += 1) {
        target.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
        document.dispatchEvent(new KeyboardEvent('keydown', {
          key: 'ArrowDown',
          bubbles: true,
          cancelable: true,
        }));
      }
    }, firstTargetIndex);
    await expect.poll(() => presentationRequests).toBe(1);
    expect(await page.evaluate(() => window.__bug009BlockingTransitions)).toBe(0);
    await latestTarget.hover();
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
    expect(presentationRequests).toBe(1);
    await expect(page.locator('.term-row.sel')).toHaveText(latestText);
    await expect(page.locator('#screen')).not.toHaveClass(/shared-input-pending/);
    await expect(latestTarget).toHaveCSS('cursor', 'pointer');
    await expect(latestTarget).toHaveCSS('opacity', '1');
    await expect(latestTarget).toHaveCSS('filter', 'none');
  } finally {
    await page.evaluate(() => window.__bug009ClassObserver?.disconnect());
    releasePresentation();
  }

  await expect(page.locator('.term-row.sel')).toHaveText(latestText);
  await expect.poll(() => presentationRequests).toBe(2);
  expect(requests.filter(request => request.procedure === 'SetPresentation')).toHaveLength(2);
});

test('controller reassignment atomically transfers presentation and gameplay authority without resetting the shared view', async ({ browser, request }) => {
  const firstContext = await browser.newContext();
  const secondContext = await browser.newContext();
  await installPlayerDiagnostics(firstContext);
  await installPlayerDiagnostics(secondContext);
  const first = await firstContext.newPage();
  const second = await secondContext.newPage();
  const firstRequests = typedPlayerRequests(first);
  const secondRequests = typedPlayerRequests(second);

  await Promise.all([first.goto('/'), second.goto('/')]);
  await Promise.all([
    expect(first.locator('#connOverlay')).toBeHidden(),
    expect(second.locator('#connOverlay')).toBeHidden(),
  ]);
  const firstCharacter = await selectFirstAvailable(first);
  const secondCharacter = await selectFirstAvailable(second);

  await first.locator('.term-row', { hasText: 'STATUS' }).hover();
  await Promise.all([
    expect(first.locator('.term-row.sel')).toContainText('STATUS'),
    expect(second.locator('.term-row.sel')).toContainText('STATUS'),
  ]);

  const reassignment = await request.post('/__fixture/reassign-controller');
  expect(reassignment.status()).toBe(200);
  await Promise.all([
    expect(first.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ'),
    expect(second.locator('#roleBadge')).toHaveAttribute('data-role', 'active'),
  ]);
  await expect(first.locator('#screen')).toHaveAttribute('aria-readonly', 'true');
  await expect(second.locator('#screen')).toHaveAttribute('aria-readonly', 'false');
  await expect(first.locator('#playerCharacterName')).toHaveText(firstCharacter);
  await expect(second.locator('#playerCharacterName')).toHaveText(secondCharacter);

  const formerControllerRequestCount = firstRequests.length;
  await first.locator('.term-row', { hasText: 'DOCS' }).hover();
  await first.keyboard.press('ArrowDown');
  await expect(first.locator('.term-row.sel')).toContainText('STATUS');
  expect(firstRequests).toHaveLength(formerControllerRequestCount);

  await second.locator('.term-row', { hasText: 'DOCS' }).hover();
  await expect.poll(() => secondRequests.map(item => item.procedure)).toContain('SetPresentation');
  await Promise.all([
    expect(first.locator('.term-row.sel')).toContainText('DOCS'),
    expect(second.locator('.term-row.sel')).toContainText('DOCS'),
  ]);

  await first.reload();
  await expect(first.locator('#connOverlay')).toBeHidden();
  await expect(first.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
  await expect(first.locator('.term-row.sel')).toContainText('DOCS');

  await second.locator('.term-row', { hasText: 'DOCS' }).click();
  await Promise.all([
    expect(first.locator('.term-row', { hasText: 'REPORT' })).toBeVisible(),
    expect(second.locator('.term-row', { hasText: 'REPORT' })).toBeVisible(),
  ]);

  await secondContext.close();
  await firstContext.close();
});

test('controller reassignment discards a queued presentation before it can be sent', async ({ browser, request }) => {
  const controllerContext = await browser.newContext();
  const observerContext = await browser.newContext();
  await installPlayerDiagnostics(controllerContext);
  await installPlayerDiagnostics(observerContext);
  const controller = await controllerContext.newPage();
  const observer = await observerContext.newPage();
  await Promise.all([controller.goto('/'), observer.goto('/')]);
  await Promise.all([
    expect(controller.locator('#connOverlay')).toBeHidden(),
    expect(observer.locator('#connOverlay')).toBeHidden(),
  ]);
  await selectFirstAvailable(controller);
  await selectFirstAvailable(observer);
  const initialSelection = (await controller.locator('.term-row.sel').textContent()).trim();

  let releasePresentation;
  const presentationGate = new Promise(resolve => { releasePresentation = resolve; });
  let presentationRequests = 0;
  await controller.route(`**${PLAYER_SERVICE}SetPresentation`, async route => {
    presentationRequests += 1;
    await presentationGate;
    await route.continue();
  });

  try {
    await controller.locator('.term-row', { hasText: 'STATUS' }).hover();
    await expect.poll(() => presentationRequests).toBe(1);
    await controller.locator('.term-row', { hasText: 'DOCS' }).hover();
    await controller.waitForTimeout(100);
    expect(presentationRequests).toBe(1);

    const reassignment = await request.post('/__fixture/reassign-controller');
    expect(reassignment.status()).toBe(200);
    await Promise.all([
      expect(controller.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ'),
      expect(observer.locator('#roleBadge')).toHaveAttribute('data-role', 'active'),
    ]);
  } finally {
    releasePresentation();
  }

  await controller.waitForTimeout(150);
  expect(presentationRequests).toBe(1);
  await Promise.all([
    expect(controller.locator('.term-row.sel')).toHaveText(initialSelection),
    expect(observer.locator('.term-row.sel')).toHaveText(initialSelection),
  ]);

  await observer.locator('.term-row', { hasText: 'DOCS' }).hover();
  await Promise.all([
    expect(controller.locator('.term-row.sel')).toContainText('DOCS'),
    expect(observer.locator('.term-row.sel')).toContainText('DOCS'),
  ]);
  await observerContext.close();
  await controllerContext.close();
});

test('navigation context replacement discards queued presentation intent', async ({ page }) => {
  await openPlayer(page);
  await selectFirstAvailable(page);

  let releasePresentation;
  const presentationGate = new Promise(resolve => { releasePresentation = resolve; });
  let presentationRequests = 0;
  await page.route(`**${PLAYER_SERVICE}SetPresentation`, async route => {
    presentationRequests += 1;
    await presentationGate;
    await route.continue();
  });

  try {
    await page.locator('.term-row', { hasText: 'STATUS' }).hover();
    await expect.poll(() => presentationRequests).toBe(1);
    await page.evaluate(() => {
      const docs = Array.from(document.querySelectorAll('.term-row'))
        .find(row => row.textContent.includes('DOCS'));
      docs?.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
      docs?.click();
    });
    await expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible();
  } finally {
    releasePresentation();
  }

  await page.waitForTimeout(150);
  expect(presentationRequests).toBe(1);
  await expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible();
});

test('latest presentation drains after an in-flight transport failure', async ({ page, request }) => {
  await openPlayer(page);
  await selectFirstAvailable(page);
  await page.mouse.move(0, 0);
  expect((await request.post('/__fixture/local/crt/content')).status()).toBe(204);

  let releaseFailure;
  const failureGate = new Promise(resolve => { releaseFailure = resolve; });
  let presentationRequests = 0;
  await page.route(`**${PLAYER_SERVICE}SetPresentation`, async route => {
    presentationRequests += 1;
    if (presentationRequests === 1) {
      await failureGate;
      await route.fulfill({ status: 503, contentType: 'text/plain', body: 'simulated presentation failure' });
      return;
    }
    await route.continue();
  });

  const rows = page.locator('.term-row');
  await expect(rows).toHaveCount(25);
  const rowCount = await rows.count();
  const initialIndex = await rows.evaluateAll(items => items.findIndex(item => item.classList.contains('sel')));
  const firstTargetIndex = (initialIndex + 1) % rowCount;
  const latestTargetIndex = (initialIndex + 2) % rowCount;
  const latestText = (await rows.nth(latestTargetIndex).innerText()).trim();
  await page.evaluate(targetIndex => {
    document.querySelectorAll('.term-row')[targetIndex]
      ?.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
  }, firstTargetIndex);
  await expect.poll(() => presentationRequests).toBe(1);
  await page.evaluate(targetIndex => {
    document.querySelectorAll('.term-row')[targetIndex]
      ?.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
  }, latestTargetIndex);
  releaseFailure();

  await expect.poll(() => presentationRequests).toBeGreaterThanOrEqual(2);
  await expect(page.locator('.term-row.sel')).toHaveText(latestText);
  await expect(page.locator('#screen')).not.toHaveClass(/shared-input-pending/);
});

test('controller paging and hacking preview are authoritative while observer controls remain locally inert', async ({ browser, request }) => {
  const controllerContext = await browser.newContext();
  const observerContext = await browser.newContext();
  await installPlayerDiagnostics(controllerContext);
  await installPlayerDiagnostics(observerContext);
  const controller = await controllerContext.newPage();
  const observer = await observerContext.newPage();
  const observerRequests = typedPlayerRequests(observer);

  await Promise.all([controller.goto('/'), observer.goto('/')]);
  await Promise.all([
    expect(controller.locator('#connOverlay')).toBeHidden(),
    expect(observer.locator('#connOverlay')).toBeHidden(),
  ]);
  await selectFirstAvailable(controller);
  await selectFirstAvailable(observer);

  expect((await request.post('/__fixture/local/crt/content')).status()).toBe(204);
  await controller.locator('.term-row', { hasText: 'LONG RECORD' }).click();
  await Promise.all([
    expect(controller.locator('#pageNext')).toBeVisible(),
    expect(observer.locator('#pageNext')).toBeVisible(),
  ]);
  const firstPage = await controller.locator('#pageIndicator').textContent();
  await controller.locator('#pageNext').click();
  await expect(controller.locator('#pageIndicator')).not.toHaveText(firstPage);
  const authoritativePage = await controller.locator('#pageIndicator').textContent();
  await expect(observer.locator('#pageIndicator')).toHaveText(authoritativePage);

  const observerPageRequests = observerRequests.length;
  await observer.locator('#pageNext').click({ force: true });
  await observer.keyboard.press('PageUp');
  await expect(observer.locator('#pageIndicator')).toHaveText(authoritativePage);
  expect(observerRequests).toHaveLength(observerPageRequests);

  expect((await request.post('/__fixture/local/crt/hacking')).status()).toBe(204);
  await Promise.all([
    expect(controller.locator('#hackBoard')).toBeVisible(),
    expect(observer.locator('#hackBoard')).toBeVisible(),
  ]);
  await controller.locator('.hcell.word').first().hover();
  await expect(controller.locator('#hackInputPreview')).not.toHaveText('');
  const authoritativePreview = await controller.locator('#hackInputPreview').textContent();
  await expect(observer.locator('#hackInputPreview')).toHaveText(authoritativePreview);

  const observerHackRequests = observerRequests.length;
  await observer.locator('.hcell.word').last().hover();
  await observer.keyboard.press('ArrowRight');
  await expect(observer.locator('#hackInputPreview')).toHaveText(authoritativePreview);
  expect(observerRequests).toHaveLength(observerHackRequests);

  await observerContext.close();
  await controllerContext.close();
});

test('rapid hacking hover keeps one presentation in flight and follows with only the latest target', async ({ browser, page, request }) => {
  await installMovementCueDiagnostics(page);

  await openPlayer(page);
  await selectFirstAvailable(page);
  const observerContext = await browser.newContext();
  await installPlayerDiagnostics(observerContext);
  const observer = await observerContext.newPage();
  await observer.goto('/');
  await expect(observer.locator('#connOverlay')).toBeHidden();
  await selectFirstAvailable(observer);
  await expect(observer.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
  expect((await request.post('/__fixture/local/crt/hacking')).status()).toBe(204);
  await Promise.all([
    expect(page.locator('#hackBoard')).toBeVisible(),
    expect(observer.locator('#hackBoard')).toBeVisible(),
  ]);
  await expect.poll(() => page.locator('.hcell.filler').count()).toBeGreaterThan(100);
  await expect.poll(() => page.locator('.hcell.word').count()).toBeGreaterThan(1);

  await page.evaluate(() => {
    const preview = document.querySelector('#hackInputPreview');
    window.__movementCueURLs = [];
    window.__bug010PreviewTransitions = [];
    window.__bug010PreviewObserver = new MutationObserver(() => {
      window.__bug010PreviewTransitions.push(preview.textContent || '');
    });
    window.__bug010PreviewObserver.observe(preview, { childList: true, characterData: true, subtree: true });
  });

  let releasePresentation;
  const presentationGate = new Promise(resolve => { releasePresentation = resolve; });
  let presentationRequests = 0;
  let blockedRequests = 0;
  let maximumBlockedRequests = 0;
  await page.route(`**${PLAYER_SERVICE}SetPresentation`, async route => {
    presentationRequests += 1;
    blockedRequests += 1;
    maximumBlockedRequests = Math.max(maximumBlockedRequests, blockedRequests);
    await presentationGate;
    try {
      await route.continue();
    } finally {
      blockedRequests -= 1;
    }
  });

  const firstTarget = page.locator('.hcell.filler').first();
  let finalTargetID = '';
  try {
    await firstTarget.hover();
    await expect.poll(() => presentationRequests).toBe(1);
    finalTargetID = await page.evaluate(() => {
      const targets = Array.from(document.querySelectorAll('.hcell.filler')).slice(1, 101);
      for (const target of targets) {
        target.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
      }
      const words = document.querySelectorAll('.hcell.word');
      const finalTarget = words[words.length - 1];
      finalTarget?.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
      return finalTarget?.dataset.target || '';
    });
    const nextFrameTargetID = await page.evaluate(() => new Promise(resolve => {
      requestAnimationFrame(() => {
        resolve(document.querySelector('.hcell.hi')?.dataset.target || '');
      });
    }));
    expect(nextFrameTargetID).toBe(finalTargetID);
    await page.waitForTimeout(100);
    expect(presentationRequests).toBe(1);
    expect(maximumBlockedRequests).toBe(1);
  } finally {
    releasePresentation();
  }

  await expect.poll(() => page.locator('.hcell.hi').first().getAttribute('data-target')).toBe(finalTargetID);
  await expect.poll(() => observer.locator('.hcell.hi').first().getAttribute('data-target')).toBe(finalTargetID);
  await expect(page.locator('#hackInputPreview')).not.toHaveText('');
  await expect(observer.locator('#hackInputPreview')).toHaveText(await page.locator('#hackInputPreview').textContent());
  await expect.poll(() => presentationRequests).toBeLessThanOrEqual(2);
  const diagnostics = await page.evaluate(() => {
    window.__bug010PreviewObserver?.disconnect();
    return {
      previews: window.__bug010PreviewTransitions,
      cues: window.__movementCueURLs.filter(url => /\/sounds\/(?:single|multiple)\//.test(url)),
    };
  });
  expect(
    new Set(diagnostics.previews.filter(Boolean)).size,
    JSON.stringify(diagnostics),
  ).toBeLessThanOrEqual(2);
  expect(diagnostics.cues, JSON.stringify(diagnostics)).toHaveLength(1);
  expect(diagnostics.cues[0]).toContain('/sounds/multiple/');
  await observerContext.close();
});

test('late unary hacking presentation rejection cannot outlive trusted success', async ({ browser, request }) => {
  const controllerContext = await browser.newContext();
  const observerContext = await browser.newContext();
  await controllerContext.route(`**${PLAYER_SERVICE}PresentationUplink`, route => route.abort());
  await installPlayerDiagnostics(controllerContext);
  await installPlayerDiagnostics(observerContext);
  const controller = await controllerContext.newPage();
  const observer = await observerContext.newPage();
  await Promise.all([controller.goto('/'), observer.goto('/')]);
  await Promise.all([controller, observer].map(page => expect(page.locator('#connOverlay')).toBeHidden()));
  await selectFirstAvailable(controller);
  await selectFirstAvailable(observer);
  await expect(observer.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
  expect((await request.post('/__fixture/local/crt/hacking')).status()).toBe(204);
  await Promise.all([controller, observer].map(page => expect(page.locator('#hackBoard')).toBeVisible()));
  await submitUnsuccessfulHackGuess(controller, [observer]);

  let unaryPresentationRequests = 0;
  controller.on('request', observed => {
    if (observed.url().endsWith(`${PLAYER_SERVICE}SetPresentation`)) unaryPresentationRequests += 1;
  });
  expect((await request.post('/__fixture/presentation-uplinks/close')).status()).toBe(204);
  await controller.waitForTimeout(100);
  expect((await request.post('/__fixture/edge/presentation-gate/arm-unary')).status()).toBe(204);
  await controller.locator('.hcell.word').last().hover();
  await expect.poll(() => unaryPresentationRequests).toBe(1);
  await expect.poll(async () => (await request.get('/__fixture/edge/presentation-gate/blocked')).status()).toBe(204);
  expect((await request.post('/__fixture/force-hack')).status()).toBe(204);
  await expectNoPlayerNotice([controller, observer]);

  expect((await request.post('/__fixture/edge/presentation-gate/release')).status()).toBe(204);
  await expect.poll(async () => (await request.get('/__fixture/edge/presentation-gate/blocked')).status()).toBe(409);
  await controller.waitForTimeout(100);
  await expect(controller.locator('#hackHeader')).toBeVisible();
  await expectNoPlayerNotice([controller, observer]);
  await Promise.all([controller, observer].map(page =>
    expect(page.locator('#termList')).toBeVisible({ timeout: 5000 })));
  await expectNoPlayerNotice([controller, observer]);

  await observerContext.close();
  await controllerContext.close();
});

test('late streamed hacking presentation rejection cannot outlive trusted success', async ({ browser, request }) => {
  const edgeStatus = await request.get('/__fixture/edge/status');
  const protectedOrigin = (await edgeStatus.json()).publicUrl;
  const contextOptions = {
    httpCredentials: { username: 'players', password: 'password-long-enough' },
    ignoreHTTPSErrors: true,
  };
  const controllerContext = await browser.newContext(contextOptions);
  const observerContext = await browser.newContext(contextOptions);
  await installPlayerDiagnostics(controllerContext);
  await installPlayerDiagnostics(observerContext);
  const controller = await controllerContext.newPage();
  const observer = await observerContext.newPage();
  let presentationUplinks = 0;
  controller.on('request', observed => {
    if (observed.url().endsWith(`${PLAYER_SERVICE}PresentationUplink`)) presentationUplinks += 1;
  });
  await Promise.all([controller.goto(`${protectedOrigin}/`), observer.goto(`${protectedOrigin}/`)]);
  await Promise.all([controller, observer].map(page => expect(page.locator('#connOverlay')).toBeHidden()));
  await selectFirstAvailable(controller);
  await selectFirstAvailable(observer);
  await expect(observer.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
  await expect.poll(() => presentationUplinks).toBeGreaterThanOrEqual(1);
  expect((await request.post('/__fixture/edge/hacking')).status()).toBe(204);
  await Promise.all([controller, observer].map(page => expect(page.locator('#hackBoard')).toBeVisible()));
  await submitUnsuccessfulHackGuess(controller, [observer]);

  expect((await request.post('/__fixture/edge/presentation-gate/arm')).status()).toBe(204);
  await controller.locator('.hcell.word').last().hover();
  await expect.poll(async () => (await request.get('/__fixture/edge/presentation-gate/blocked')).status()).toBe(204);
  expect((await request.post('/__fixture/force-hack')).status()).toBe(204);
  await expectNoPlayerNotice([controller, observer]);

  expect((await request.post('/__fixture/edge/presentation-gate/release')).status()).toBe(204);
  await expect.poll(async () => (await request.get('/__fixture/edge/presentation-gate/blocked')).status()).toBe(409);
  await controller.waitForTimeout(100);
  await expect(controller.locator('#hackHeader')).toBeVisible();
  await expectNoPlayerNotice([controller, observer]);
  await Promise.all([controller, observer].map(page =>
    expect(page.locator('#termList')).toBeVisible({ timeout: 5000 })));
  await expectNoPlayerNotice([controller, observer]);

  await observerContext.close();
  await controllerContext.close();
});

test('public stream rotation preserves one final authoritative menu and hacking cue', async ({ browser, request }) => {
  const edgeStatus = await request.get('/__fixture/edge/status');
  const protectedOrigin = (await edgeStatus.json()).publicUrl;
  const context = await browser.newContext({
    httpCredentials: { username: 'players', password: 'password-long-enough' },
    ignoreHTTPSErrors: true,
  });
  await installPlayerDiagnostics(context);
  await installMovementCueDiagnostics(context);
  const [controller, observerOne, observerTwo] = await Promise.all([
    context.newPage(), context.newPage(), context.newPage(),
  ]);
  let controllerUplinks = 0;
  controller.on('request', observed => {
    if (observed.url().endsWith(`${PLAYER_SERVICE}PresentationUplink`)) controllerUplinks += 1;
  });

  await Promise.all([controller, observerOne, observerTwo].map(page => page.goto(protectedOrigin + '/')));
  await Promise.all([controller, observerOne, observerTwo].map(page => expect(page.locator('#connOverlay')).toBeHidden()));
  await selectFirstAvailable(controller);
  await Promise.all([controller, observerOne, observerTwo].map(page => expect(page.locator('#termList')).toBeVisible()));
  expect((await request.post('/__fixture/local/crt/content')).status()).toBe(204);
  await Promise.all([controller, observerOne, observerTwo].map(page => expect(page.locator('.term-row')).toHaveCount(25)));
  await expect.poll(() => controllerUplinks).toBeGreaterThanOrEqual(1);
  await controller.waitForTimeout(250);

  await resetMovementCueDiagnostics(controller);
  expect((await request.post('/__fixture/edge/presentation-gate/arm')).status()).toBe(204);
  const unselectedMenuRows = controller.locator('.term-row:not(.sel)');
  await expect.poll(() => unselectedMenuRows.count()).toBeGreaterThanOrEqual(2);
  const supersededMenu = unselectedMenuRows.nth(0);
  const finalMenu = unselectedMenuRows.nth(1);
  await supersededMenu.hover();
  await expect.poll(async () => (await request.get('/__fixture/edge/presentation-gate/blocked')).status()).toBe(204);
  const finalMenuText = (await finalMenu.textContent()).trim();
  await finalMenu.hover();
  expect((await request.post('/__fixture/edge/presentation-gate/cancel-uplinks')).status()).toBe(204);

  await Promise.all([controller, observerOne, observerTwo].map(page =>
    expect(page.locator('.term-row.sel')).toHaveText(finalMenuText)));
  await expect.poll(async () => (await movementCueDiagnostics(controller)).urls
    .filter(url => url.includes('/sounds/menu-focus/')).length).toBe(1);
  let diagnostics = await movementCueDiagnostics(controller);
  expect(diagnostics.urls.filter(url => url.includes('/sounds/menu-focus/')), JSON.stringify(diagnostics)).toHaveLength(1);
  expect(diagnostics.stages.filter(event => event.stage === 'dispatch' && event.folder === 'menu-focus')).toHaveLength(1);
  expect(diagnostics.stages.filter(event => event.stage === 'source-started' && event.url.includes('/sounds/menu-focus/'))).toHaveLength(1);

  expect((await request.post('/__fixture/edge/hacking')).status()).toBe(204);
  await Promise.all([controller, observerOne, observerTwo].map(page => expect(page.locator('#hackBoard')).toBeVisible()));
  await expect.poll(() => controllerUplinks, { timeout: 5_000 }).toBeGreaterThanOrEqual(2);
  await controller.waitForTimeout(250);
  await resetMovementCueDiagnostics(controller);
  expect((await request.post('/__fixture/edge/presentation-gate/arm')).status()).toBe(204);
  await controller.locator('.hcell.filler').first().hover();
  await expect.poll(async () => (await request.get('/__fixture/edge/presentation-gate/blocked')).status()).toBe(204);
  const finalHackTarget = controller.locator('.hcell.word').last();
  const finalHackID = await finalHackTarget.getAttribute('data-target');
  await finalHackTarget.hover();
  expect((await request.post('/__fixture/edge/presentation-gate/cancel-uplinks')).status()).toBe(204);

  await Promise.all([controller, observerOne, observerTwo].map(page =>
    expect.poll(() => page.locator('.hcell.hi').first().getAttribute('data-target')).toBe(finalHackID)));
  await expect.poll(async () => (await movementCueDiagnostics(controller)).urls
    .filter(url => /\/sounds\/(?:single|multiple)\//.test(url)).length).toBe(1);
  diagnostics = await movementCueDiagnostics(controller);
  const hackingCues = diagnostics.urls.filter(url => /\/sounds\/(?:single|multiple)\//.test(url));
  expect(hackingCues, JSON.stringify(diagnostics)).toHaveLength(1);
  expect(hackingCues[0]).toContain('/sounds/multiple/');
  expect(diagnostics.stages.filter(event => event.stage === 'dispatch' && event.folder === 'multiple')).toHaveLength(1);
  expect(diagnostics.stages.filter(event => event.stage === 'source-started' && event.url.includes('/sounds/multiple/'))).toHaveLength(1);

  await context.close();
});

test('optional audio failures never block typed selection or navigation', async ({ page }) => {
  await openPlayer(page, null, { audioFailure: true });
  await selectFirstAvailable(page);
  await page.locator('.term-row', { hasText: 'DOCS' }).click();
  await expect(page.locator('.term-row', { hasText: 'REPORT' })).toBeVisible();
  await expect(page.locator('#connOverlay')).toBeHidden();
  await expect.poll(() => page.evaluate(() => window.__webSocketConstructions)).toBe(0);
});

test('player persistence contains only the opaque recognition handle', async ({ page }) => {
  await openPlayer(page);
  const storage = await page.evaluate(() => {
    const values = {};
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index);
      values[key] = localStorage.getItem(key);
    }
    return values;
  });
  expect(Object.keys(storage)).toEqual([TOKEN_KEY]);
  expect(storage[TOKEN_KEY]).toMatch(/\S+/);
});
