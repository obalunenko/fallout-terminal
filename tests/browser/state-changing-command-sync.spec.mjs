import { expect, test } from '@playwright/test';

const TOKEN_KEY = 'fallout-terminal.player-token';
const FIXTURE = '/__fixture/state-changing-command-sync';
const OVERSEER_URL = `${FIXTURE}/overseer`;

async function mountOverseerCandidate(page) {
  await page.evaluate(() => import('http://127.0.0.1:34120/candidate-main.ts?command-state-reset'));
}

async function postLifecycle(request, action) {
  const response = await request.post(`${FIXTURE}/${action}`);
  expect(response.status()).toBe(204);
}

async function installPlayerDiagnostics(target, storedToken = null) {
  await target.addInitScript(({ tokenKey, token }) => {
    window.__webSocketConstructions = 0;
    window.WebSocket = class ForbiddenLegacyPlayerTransport {
      constructor() {
        window.__webSocketConstructions += 1;
        throw new Error('the player must use generated ConnectRPC');
      }
    };
    HTMLMediaElement.prototype.play = () => Promise.resolve();
    if (token === null) localStorage.removeItem(tokenKey);
    else localStorage.setItem(tokenKey, token);
  }, { tokenKey: TOKEN_KEY, token: storedToken });
}

async function openPlayer(browser, storedToken = null) {
  const context = await browser.newContext();
  await installPlayerDiagnostics(context, storedToken);
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await expect.poll(() => page.evaluate(() => window.__webSocketConstructions)).toBe(0);
  return { context, page };
}

async function openOverseer(browser) {
  const context = await browser.newContext({ bypassCSP: true });
  const page = await context.newPage();
  await page.goto(OVERSEER_URL);
  await mountOverseerCandidate(page);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
  return { context, page };
}

async function selectFirstAvailable(page) {
  const option = page.locator('#characterOptions button:not([disabled])').first();
  await expect(option).toBeVisible();
  await option.click();
  await expect(page.locator('#characterSelect')).not.toHaveAttribute('aria-busy', 'true');
}

async function assignControllerAndObservers(players, expectedMenuTitle = 'Открыть двери') {
  for (const player of players) await selectFirstAvailable(player.page);
  await expect(players[0].page.locator('#roleBadge')).toContainText('АКТИВЕН');
  await Promise.all(players.slice(1).map(player =>
    expect(player.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ')));
  await Promise.all(players.map(player =>
    expect(player.page.locator('.term-row', { hasText: expectedMenuTitle })).toBeVisible()));
}

async function openThreePlayerJourney(browser) {
  const overseer = await openOverseer(browser);
  const players = [];
  for (let index = 0; index < 3; index += 1) players.push(await openPlayer(browser));
  await assignControllerAndObservers(players);
  return { overseer, players };
}

async function closeJourney(journey) {
  for (const player of journey.players) {
    await player.context.close().catch(() => {});
  }
  await journey.overseer.context.close().catch(() => {});
}

async function expectFullScreenCommandSurface(page, text, timeout) {
  await expect(page.locator('#termEntry')).toBeVisible({ timeout });
  await expect(page.locator('#entryBody')).toContainText(text, { timeout });
  await expect(page.locator('#termOutput')).toBeHidden({ timeout });
  await expect(page.locator('#termList')).toBeHidden({ timeout });
  await expect(page.locator('#termPrompt')).toBeVisible({ timeout });
}

async function completeVisibleReveal(page) {
  await page.keyboard.press('Shift');
}

async function recordRendererSnapshot(page) {
  return page.evaluate(() => {
    const surface = document.querySelector('#termEntry');
    const body = document.querySelector('#entryBody');
    const surfaceStyle = getComputedStyle(surface);
    const bodyStyle = getComputedStyle(body);
    const roundedBounds = bounds => ({
      x: Math.round(bounds.x),
      y: Math.round(bounds.y),
      width: Math.round(bounds.width),
      height: Math.round(bounds.height),
    });
    return {
      surface: {
        bounds: roundedBounds(surface.getBoundingClientRect()),
        display: surfaceStyle.display,
        flexDirection: surfaceStyle.flexDirection,
        overflow: surfaceStyle.overflow,
        padding: surfaceStyle.padding,
      },
      body: {
        bounds: roundedBounds(body.getBoundingClientRect()),
        fontFamily: bodyStyle.fontFamily,
        fontSize: bodyStyle.fontSize,
        lineHeight: bodyStyle.lineHeight,
        overflow: bodyStyle.overflow,
        overflowWrap: bodyStyle.overflowWrap,
        whiteSpace: bodyStyle.whiteSpace,
      },
    };
  });
}

async function expectMatchingRecordRenderers(players) {
  await Promise.all(players.map(player => completeVisibleReveal(player.page)));
  const snapshots = await Promise.all(players.map(player => recordRendererSnapshot(player.page)));
  for (const snapshot of snapshots.slice(1)) expect(snapshot).toEqual(snapshots[0]);
}

async function pageCount(page) {
  const value = await page.locator('#pageIndicator').textContent();
  return Number.parseInt(value.split('/')[1], 10);
}

async function chooseStateChangingCommand(journey, request) {
  await journey.players[0].page.locator('.term-row', { hasText: 'Открыть двери' }).click();
  await Promise.all(journey.players.map(player =>
    expectFullScreenCommandSurface(player.page, 'Выполняется запрос')));
  await expectMatchingRecordRenderers(journey.players);
  await Promise.all(journey.players.slice(1).map(player =>
    expect(player.page.locator('#backBtn')).toBeHidden()));

  const dialog = journey.overseer.page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
  await expect(dialog).toBeVisible();
  const stateResponse = await request.get(`${FIXTURE}/state`);
  expect(stateResponse.ok()).toBe(true);
  const state = await stateResponse.json();
  expect(state.pendingCommandExecution).toMatchObject({
    commandName: 'Открыть двери',
    mode: 'state-change',
  });
  await expect(dialog.locator('#commandExecutionDialogStatus')).toHaveText(
    `ЗАПРОС: ${state.pendingCommandExecution.requestId} · РЕЖИМ: ИЗМЕНЕНИЕ СОСТОЯНИЯ · КОМАНДА: Открыть двери`,
  );
  await expect(dialog.locator('#commandExecutionDialogDescription')).toHaveText('Разрешить доступ в защищённый сектор?');
  return dialog;
}

async function audit(request) {
  const response = await request.get(`${FIXTURE}/audit`);
  expect(response.ok()).toBe(true);
  return response.json();
}

test.beforeEach(async ({ request }) => {
  await postLifecycle(request, 'reset');
});

test('command-state reset is atomic idempotent and focus-safe', async ({ browser }) => {
  const overseer = await openOverseer(browser);
  try {
    await overseer.page.evaluate(() => {
      globalThis.__resetDecisions = [];
      __overseerCoexistenceBridge.subscribeVueRequests((message) => {
        if (message.kind === 'command-state-reset-resolved') __resetDecisions.push(message);
      });
    });
    const opener = overseer.page.locator('#btnAddTerminal');
    await opener.focus();
    await overseer.page.evaluate(() => {
      __overseerCoexistenceBridge.legacyToVue({
        kind: 'command-state-reset-required',
        message: 'Сбросить выполненное состояние команды "Двери открыты"?',
        requestId: 'reset-focused-1',
      });
    });

    const dialog = overseer.page.locator('#overseerVueLeaves #resetConfirmationDialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('#btnCancelCommandStateReset')).toBeFocused();
    await dialog.locator('#btnConfirmCommandStateReset').evaluate((button) => {
      button.click();
      button.click();
    });
    await expect(dialog).toBeHidden();
    await expect(opener).toBeFocused();
    expect(await overseer.page.evaluate(() => __resetDecisions)).toEqual([
      expect.objectContaining({ confirmed: true, requestId: 'reset-focused-1' }),
    ]);

    await overseer.page.evaluate(() => {
      __overseerCoexistenceBridge.legacyToVue({
        kind: 'command-state-reset-required',
        message: 'stale',
        requestId: 'reset-focused-1',
      });
    });
    await expect(dialog).toBeHidden();
    expect(await overseer.page.evaluate(() => __resetDecisions)).toHaveLength(1);
  } finally {
    await overseer.context.close();
  }
});

test('controller and two observers converge on completed result and title within one second', async ({ browser, request }) => {
  const journey = await openThreePlayerJourney(browser);
  try {
    const dialog = await chooseStateChangingCommand(journey, request);
    const startedAt = Date.now();
    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
    await Promise.all(journey.players.map(player =>
      expectFullScreenCommandSurface(player.page, 'Доступ в сектор разрешён.', 1000)));
    expect(Date.now() - startedAt).toBeLessThanOrEqual(1000);
    await expectMatchingRecordRenderers(journey.players);
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('#pageNext')).toBeVisible()));

    const widePageCount = await pageCount(journey.players[0].page);
    await Promise.all(journey.players.map(player =>
      player.page.setViewportSize({ width: 720, height: 520 })));
    await Promise.all(journey.players.map(player =>
      expect.poll(() => pageCount(player.page)).not.toBe(widePageCount)));
    await expectMatchingRecordRenderers(journey.players);

    const firstPage = await journey.players[0].page.locator('#pageIndicator').textContent();
    await journey.players[0].page.keyboard.press('PageDown');
    await expect(journey.players[0].page.locator('#pageIndicator')).not.toHaveText(firstPage);
    const authoritativePage = await journey.players[0].page.locator('#pageIndicator').textContent();
    await Promise.all(journey.players.slice(1).map(async player => {
      await player.page.keyboard.press('Enter');
      await player.page.keyboard.press('Backspace');
      await expect(player.page.locator('#termEntry')).toBeVisible();
      await expect(player.page.locator('#pageIndicator')).toHaveText(authoritativePage);
      await expect(player.page.locator('#backBtn')).toBeHidden();
    }));

    const acknowledgementStartedAt = Date.now();
    await journey.players[0].page.keyboard.press('Enter');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Двери открыты' })).toBeVisible({ timeout: 1000 })));
    expect(Date.now() - acknowledgementStartedAt).toBeLessThanOrEqual(1000);
    expect(await audit(request)).toMatchObject({ executeWrites: 1, pendingRequests: 0, completed: true });
  } finally {
    await closeJourney(journey);
  }
});

test('controller disconnect keeps one pending request and durable completion survives every shared lifecycle', async ({ browser, request }) => {
  const journey = await openThreePlayerJourney(browser);
  try {
    const dialog = await chooseStateChangingCommand(journey, request);
    const controllerToken = await journey.players[0].page.evaluate(key => localStorage.getItem(key), TOKEN_KEY);
    expect(controllerToken).toMatch(/\S+/);

    await journey.players[0].context.close();
    await Promise.all(journey.players.slice(1).map(player =>
      expectFullScreenCommandSurface(player.page, 'Выполняется запрос')));
    await expect(dialog).toBeVisible();
    await expect.poll(async () => (await audit(request)).pendingRequests).toBe(1);

    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
    await expect(dialog).toBeHidden();
    await Promise.all(journey.players.slice(1).map(player =>
      expectFullScreenCommandSurface(player.page, 'Доступ в сектор разрешён.')));

    const reconnected = await openPlayer(browser, controllerToken);
    journey.players[0] = reconnected;
    await expect(reconnected.page.locator('#roleBadge')).toContainText('АКТИВЕН');
    await expectFullScreenCommandSurface(reconnected.page, 'Доступ в сектор разрешён.');
    await expectMatchingRecordRenderers(journey.players);
    expect(await reconnected.page.evaluate(key => localStorage.getItem(key), TOKEN_KEY)).toBe(controllerToken);

    await reconnected.page.keyboard.press('Backspace');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Двери открыты' })).toBeVisible()));
    await expect(reconnected.page.locator('.term-row', { hasText: 'Открыть двери' })).toHaveCount(0);

    for (let cycle = 0; cycle < 10; cycle += 1) {
      await reconnected.page.locator('.term-row', { hasText: 'АРХИВ' }).click();
      await Promise.all(journey.players.map(player =>
        expect(player.page.locator('.term-row', { hasText: 'ЖУРНАЛ' })).toBeVisible()));
      await expect(reconnected.page.locator('#screen')).not.toHaveClass(/shared-input-pending/);
      await reconnected.page.locator('#backBtn').click();
      await Promise.all(journey.players.map(player =>
        expect(player.page.locator('.term-row', { hasText: 'Двери открыты' })).toBeVisible()));
    }

    await postLifecycle(request, 'switch-away');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'РЕЗЕРВНЫЙ СТАТУС' })).toBeVisible()));
    await postLifecycle(request, 'switch-back');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Двери открыты' })).toBeVisible()));

    await postLifecycle(request, 'restart-broadcast');
    await Promise.all(journey.players.map(player => expect(player.page.locator('#characterSelect')).toBeVisible()));
    await assignControllerAndObservers(journey.players, 'Двери открыты');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Двери открыты' })).toBeVisible()));

    await postLifecycle(request, 'reopen-session');
    await Promise.all(journey.players.map(player => expect(player.page.locator('#characterSelect')).toBeVisible()));
    await assignControllerAndObservers(journey.players, 'Двери открыты');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Двери открыты' })).toBeVisible()));

    const state = await audit(request);
    expect(state).toMatchObject({ executeWrites: 1, pendingRequests: 0, completed: true });
    for (const player of journey.players) {
      const keys = await player.page.evaluate(() =>
        Array.from({ length: localStorage.length }, (_, index) => localStorage.key(index)));
      expect(keys).toEqual([TOKEN_KEY]);
    }
  } finally {
    await closeJourney(journey);
  }
});

test('terminal switch, broadcast restart, and reopen cancel transient pending or rejected state without a write', async ({ browser, request }) => {
  const journey = await openThreePlayerJourney(browser);
  try {
    let dialog = await chooseStateChangingCommand(journey, request);
    await postLifecycle(request, 'switch-away');
    await expect(dialog).toBeHidden();
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'РЕЗЕРВНЫЙ СТАТУС' })).toBeVisible()));
    expect(await audit(request)).toMatchObject({ executeWrites: 0, pendingRequests: 0, completed: false });

    await postLifecycle(request, 'switch-back');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible()));

    dialog = await chooseStateChangingCommand(journey, request);
    await dialog.getByRole('button', { name: 'ОТКЛОНИТЬ' }).click();
    await expect(dialog).toBeHidden();
    await Promise.all(journey.players.map(player =>
      expectFullScreenCommandSurface(player.page, 'Ошибка доступа')));
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('#entryBody')).toHaveText('Ошибка доступа')));
    await expectMatchingRecordRenderers(journey.players);
    await Promise.all(journey.players.slice(1).map(async player => {
      await player.page.keyboard.press('Enter');
      await player.page.keyboard.press('Backspace');
      await expectFullScreenCommandSurface(player.page, 'Ошибка доступа');
    }));
    await journey.players[0].page.keyboard.press('Enter');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible()));

    await postLifecycle(request, 'restart-broadcast');
    await Promise.all(journey.players.map(player => expect(player.page.locator('#characterSelect')).toBeVisible()));
    await assignControllerAndObservers(journey.players);
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible()));
    await expect(journey.players[0].page.locator('#entryBody')).not.toContainText('Ошибка доступа');

    dialog = await chooseStateChangingCommand(journey, request);
    await dialog.press('Escape');
    await expect(dialog).toBeHidden();
    await Promise.all(journey.players.map(player =>
      expectFullScreenCommandSurface(player.page, 'Ошибка доступа')));
    await expectMatchingRecordRenderers(journey.players);
    await journey.players[0].page.locator('#backBtn').click();
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible()));

    dialog = await chooseStateChangingCommand(journey, request);
    await postLifecycle(request, 'reopen-session');
    await expect(dialog).toBeHidden();
    await Promise.all(journey.players.map(player => expect(player.page.locator('#characterSelect')).toBeVisible()));
    await assignControllerAndObservers(journey.players);
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible()));

    expect(await audit(request)).toMatchObject({ executeWrites: 0, pendingRequests: 0, completed: false });
  } finally {
    await closeJourney(journey);
  }
});
