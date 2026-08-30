import { expect, test } from '@playwright/test';

const TOKEN_KEY = 'fallout-terminal.player-token';
const FIXTURE = '/__fixture/state-changing-command-approval';
const OVERSEER_URL = `${FIXTURE}/overseer`;
const REQUEST_ID = 'approval-request-1';
const COMMAND_NAME = 'Открыть двери';
const CONFIRMATION_TEXT = 'Разрешить доступ в защищённый сектор?';
const ORDINARY_COMMAND_NAME = 'Запустить диагностику';

async function resetApprovalFixture(request) {
  const response = await request.post(`${FIXTURE}/reset`);
  expect(response.status()).toBe(204);
}

async function installPlayerDiagnostics(context) {
  await context.addInitScript((tokenKey) => {
    window.__webSocketConstructions = 0;
    window.WebSocket = class ForbiddenLegacyPlayerTransport {
      constructor() {
        window.__webSocketConstructions += 1;
        throw new Error('the player must use generated ConnectRPC');
      }
    };
    HTMLMediaElement.prototype.play = () => Promise.resolve();
    localStorage.removeItem(tokenKey);
  }, TOKEN_KEY);
}

async function mountOverseerCandidate(page) {
  await page.evaluate(() => import('http://127.0.0.1:34120/candidate-main.ts?command-approval'));
}

async function openApprovalJourney(browser) {
  const overseerContext = await browser.newContext({ bypassCSP: true });
  const playerContext = await browser.newContext();
  await installPlayerDiagnostics(playerContext);

  const overseer = await overseerContext.newPage();
  await overseer.goto(OVERSEER_URL);
  await mountOverseerCandidate(overseer);
  await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(overseer.locator('#mainLayout')).toBeVisible();

  const player = await playerContext.newPage();
  await player.goto('/');
  await expect(player.locator('#connOverlay')).toBeHidden();
  await expect.poll(() => player.evaluate(() => window.__webSocketConstructions)).toBe(0);
  const character = player.locator('#characterOptions button:not([disabled])').first();
  await expect(character).toBeVisible();
  await character.click();
  await expect(player.locator('#roleBadge')).toContainText('АКТИВЕН');
  await expect(player.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible();

  return { overseer, overseerContext, player, playerContext };
}

async function closeApprovalJourney(journey) {
  await journey.playerContext.close();
  await journey.overseerContext.close();
}

async function openApprovalParticipant(browser, token = '') {
  const context = await browser.newContext();
  await installPlayerDiagnostics(context);
  if (token) {
    await context.addInitScript(({ tokenKey, retainedToken }) => {
      localStorage.setItem(tokenKey, retainedToken);
    }, { tokenKey: TOKEN_KEY, retainedToken: token });
  }
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  if (await page.locator('#characterSelect').isVisible()) {
    await page.locator('#characterOptions button:not([disabled])').first().click();
  }
  await expect(page.locator('#roleBadge')).toContainText(/АКТИВЕН|НАБЛЮДАТЕЛЬ/);
  return { context, page };
}

async function chooseStateChangingCommand(journey) {
  await journey.player.locator('.term-row', { hasText: COMMAND_NAME }).click();
  await expectFullScreenCommandSurface(journey.player, 'Выполняется запрос');

  const dialogs = journey.overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
  await expect(dialogs).toHaveCount(1);
  const dialog = dialogs.first();
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#commandExecutionDialogStatus')).toHaveText(
    `ЗАПРОС: ${REQUEST_ID} · РЕЖИМ: ИЗМЕНЕНИЕ СОСТОЯНИЯ · КОМАНДА: ${COMMAND_NAME}`,
  );
  await expect(dialog.locator('#commandExecutionDialogDescription')).toHaveText(CONFIRMATION_TEXT);
  await expect(dialog.locator('#commandExecutionDialogDescription')).not.toContainText(COMMAND_NAME);
  return dialog;
}

async function chooseOrdinaryCommand(journey) {
  await journey.player.locator('.term-row', { hasText: ORDINARY_COMMAND_NAME }).click();
  await expectFullScreenCommandSurface(journey.player, 'Выполняется запрос');

  const dialog = journey.overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#commandExecutionDialogStatus')).toContainText(
    `РЕЖИМ: ОБЫЧНАЯ · КОМАНДА: ${ORDINARY_COMMAND_NAME}`,
  );
  await expect(dialog.locator('#commandExecutionDialogDescription')).toHaveText('Выполнить команду?');
  return dialog;
}

function collectFixtureNodes(session) {
  const nodes = [];
  const visit = (node, isRoot = false) => {
    if (!isRoot) nodes.push(node);
    for (const child of node.children ?? []) visit(child);
  };
  for (const terminal of session.terminals ?? []) visit(terminal.root, true);
  return nodes;
}

async function expectFullScreenCommandSurface(page, text) {
  await expect(page.locator('#termEntry')).toBeVisible();
  await expect(page.locator('#entryBody')).toContainText(text);
  await expect(page.locator('#termOutput')).toBeHidden();
  await expect(page.locator('#termList')).toBeHidden();
  await expect(page.locator('#termPrompt')).toBeVisible();
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
    const surfaceBounds = surface.getBoundingClientRect();
    const bodyBounds = body.getBoundingClientRect();
    const roundedBounds = bounds => ({
      x: Math.round(bounds.x),
      y: Math.round(bounds.y),
      width: Math.round(bounds.width),
      height: Math.round(bounds.height),
    });
    return {
      surface: {
        bounds: roundedBounds(surfaceBounds),
        display: surfaceStyle.display,
        flexDirection: surfaceStyle.flexDirection,
        overflow: surfaceStyle.overflow,
        padding: surfaceStyle.padding,
      },
      body: {
        bounds: roundedBounds(bodyBounds),
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

async function pageCount(page) {
  const value = await page.locator('#pageIndicator').textContent();
  return Number.parseInt(value.split('/')[1], 10);
}

async function resolveCalls(overseer) {
  return overseer.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'ResolveCommandExecution'));
}

async function sourceMenuSnapshot(page) {
  return page.locator('#termList').evaluate((element) => {
    const clone = element.cloneNode(true);
    for (const row of clone.querySelectorAll('.term-row')) row.classList.remove('sel');
    return clone.innerHTML;
  });
}

async function expectOrdinaryRejectionJourney(browser, reject, acknowledge) {
  const journey = await openApprovalJourney(browser);
  const firstObserver = await openApprovalParticipant(browser);
  let secondObserver = await openApprovalParticipant(browser);
  try {
    const originalMenu = await sourceMenuSnapshot(journey.player);
    const reconnectToken = await secondObserver.page.evaluate(tokenKey =>
      localStorage.getItem(tokenKey), TOKEN_KEY);
    const dialog = await chooseOrdinaryCommand(journey);

    await Promise.all([firstObserver, secondObserver].map(participant =>
      expectFullScreenCommandSurface(participant.page, 'Выполняется запрос')));
    await reject(dialog);
    await expect.poll(() => resolveCalls(journey.overseer)).toEqual([
      expect.objectContaining({
        args: [expect.objectContaining({ decision: 'reject' })],
      }),
    ]);

    for (const participant of [
      { page: journey.player }, firstObserver, secondObserver,
    ]) {
      await expectFullScreenCommandSurface(participant.page, 'Ошибка доступа');
      await expect(participant.page.locator('#entryBody')).toHaveText('Ошибка доступа');
      await expect(participant.page.locator('#termList')).toBeHidden();
    }
    await expect(firstObserver.page.locator('#backBtn')).toBeHidden();
    await expect(secondObserver.page.locator('#backBtn')).toBeHidden();

    await secondObserver.context.close();
    secondObserver = await openApprovalParticipant(browser, reconnectToken);
    await expectFullScreenCommandSurface(secondObserver.page, 'Ошибка доступа');
    await expect(secondObserver.page.locator('#backBtn')).toBeHidden();

    const audit = await journey.player.evaluate(async (endpoint) => {
      const response = await fetch(endpoint);
      return response.json();
    }, `${FIXTURE}/audit`);
    expect(audit).toEqual({ executeWrites: 0, completed: false });

    await acknowledge(journey.player);
    for (const participant of [
      { page: journey.player }, firstObserver, secondObserver,
    ]) {
      await expect(participant.page.locator('.term-row', { hasText: ORDINARY_COMMAND_NAME })).toBeVisible();
      expect(await sourceMenuSnapshot(participant.page)).toBe(originalMenu);
      await expect(participant.page.locator('#entryBody')).not.toContainText('Диагностика завершена.');
    }
  } finally {
    await firstObserver.context.close();
    await secondObserver.context.close();
    await closeApprovalJourney(journey);
  }
}

test.beforeEach(async ({ request }) => {
  await resetApprovalFixture(request);
});

test('duplicate/stale request resolves exactly once', async ({ browser }) => {
  const journey = await openApprovalJourney(browser);
  try {
    await chooseStateChangingCommand(journey);
    const vueDialog = journey.overseer.locator('#overseerVueLeaves #commandExecutionDialog');
    if (await vueDialog.count() === 0) {
      process.stderr.write('AssertionError: duplicate/stale request resolves exactly once\n');
      throw new Error('Vue-owned command approval is not implemented');
    }

    await vueDialog.locator('#btnApproveCommandExecution').evaluate((button) => {
      button.click();
      button.click();
    });
    await expect.poll(() => resolveCalls(journey.overseer)).toHaveLength(1);
    expect(await journey.overseer.evaluate(() => __desktopFixture.releaseCount('coordination-state'))).toBe(0);
    const released = await journey.overseer.evaluate(() => {
      __overseerVueFixture.unmount();
      return __desktopFixture.releaseCount('coordination-state');
    });
    expect(released).toBeGreaterThan(0);
    await journey.overseer.evaluate(() => __overseerVueFixture.unmount());
    expect(await journey.overseer.evaluate(() => __desktopFixture.releaseCount('coordination-state'))).toBe(released);
  } finally {
    await closeApprovalJourney(journey);
  }
});

test('canonical approval input has explicit folder, entry, ordinary, and initial state-changing commands', async ({ request }) => {
  const response = await request.get(`${FIXTURE}/session`);
  expect(response.ok()).toBe(true);
  const session = await response.json();
  const nodes = collectFixtureNodes(session);
  const commands = nodes.filter(node => node.type === 'command');

  expect(nodes.some(node => node.type === 'folder')).toBe(true);
  expect(nodes.some(node => node.type === 'entry')).toBe(true);
  expect(commands.length).toBeGreaterThan(0);
  expect(session.terminals.flatMap(terminal => Object.keys(terminal.commandStates ?? {}))).toEqual([]);
  const ordinaryCommands = commands.filter(command => command.stateChange == null);
  const stateChangingCommands = commands.filter(command => command.stateChange != null);
  expect(ordinaryCommands.length).toBeGreaterThan(0);
  expect(stateChangingCommands.length).toBeGreaterThan(0);
  for (const command of commands) {
    expect(command.name.trim()).not.toBe('');
    expect(command.text.trim()).not.toBe('');
  }
  for (const command of stateChangingCommands) {
    expect(command.stateChange?.completedName?.trim()).not.toBe('');
    expect(command.stateChange?.confirmationText?.trim()).not.toBe('');
    expect(command.stateChange.confirmationText).not.toContain(command.name);
  }
});

test('ordinary explicit reject remains an access error for controller, observers, and reconnect until Back', async ({ browser }) => {
  await expectOrdinaryRejectionJourney(
    browser,
    dialog => dialog.getByRole('button', { name: 'ОТКЛОНИТЬ' }).click(),
    page => page.locator('#backBtn').click(),
  );
});

test('ordinary dialog close remains an access error for controller, observers, and reconnect until Enter', async ({ browser }) => {
  await expectOrdinaryRejectionJourney(
    browser,
    dialog => dialog.press('Escape'),
    page => page.keyboard.press('Enter'),
  );
});

test('one pending request opens exactly one overseer dialog and approve publishes a full-screen durable result', async ({ browser, request }) => {
  const journey = await openApprovalJourney(browser);
  try {
    const dialog = await chooseStateChangingCommand(journey);

    const replay = await request.post(`${FIXTURE}/reemit-pending`);
    expect(replay.status()).toBe(204);
    await expect(journey.overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' })).toHaveCount(1);
    expect(await resolveCalls(journey.overseer)).toEqual([]);

    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
    await expect.poll(() => resolveCalls(journey.overseer)).toEqual([
      expect.objectContaining({
        args: [{ requestId: REQUEST_ID, decision: 'approve' }],
      }),
    ]);
    await expect(dialog).toBeHidden();
    await expectFullScreenCommandSurface(journey.player, 'Доступ в сектор разрешён.');

    await journey.player.keyboard.press('Enter');
    await expect(journey.player.locator('.term-row', { hasText: 'Двери открыты' })).toBeVisible();
    await expect(journey.player.locator('.term-row', { hasText: 'Открыть двери' })).toHaveCount(0);
  } finally {
    await closeApprovalJourney(journey);
  }
});

test('initial state-changing approval converges for controller, two observers, and reconnect before any effect', async ({ browser, request }) => {
  const journey = await openApprovalJourney(browser);
  const firstObserver = await openApprovalParticipant(browser);
  let secondObserver = await openApprovalParticipant(browser);
  try {
    await expect(firstObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    await expect(secondObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    const dialog = await chooseStateChangingCommand(journey);

    await Promise.all([firstObserver, secondObserver].map(participant =>
      expectFullScreenCommandSurface(participant.page, 'Выполняется запрос')));
    const pendingResponse = await request.get(`${FIXTURE}/state`);
    expect(pendingResponse.ok()).toBe(true);
    const pendingState = await pendingResponse.json();
    expect(pendingState.pendingCommandExecution).toMatchObject({
      terminalId: 'terminal-stateful',
      commandId: 'doors',
      commandName: COMMAND_NAME,
    });

    const retainedToken = await secondObserver.page.evaluate(tokenKey =>
      localStorage.getItem(tokenKey), TOKEN_KEY);
    await secondObserver.context.close();
    secondObserver = await openApprovalParticipant(browser, retainedToken);
    await expectFullScreenCommandSurface(secondObserver.page, 'Выполняется запрос');

    for (const participant of [
      { page: journey.player }, firstObserver, secondObserver,
    ]) {
      await participant.page.keyboard.press('Enter');
      await participant.page.keyboard.press('Backspace');
      await expectFullScreenCommandSurface(participant.page, 'Выполняется запрос');
    }
    await expect(dialog).toBeVisible();
    expect(await resolveCalls(journey.overseer)).toEqual([]);
  } finally {
    await firstObserver.context.close();
    await secondObserver.context.close();
    await closeApprovalJourney(journey);
  }
});

test('pending full-screen request ignores Enter and Back until the overseer decides', async ({ browser }) => {
  const journey = await openApprovalJourney(browser);
  try {
    const dialog = await chooseStateChangingCommand(journey);
    await journey.player.keyboard.press('Enter');
    await journey.player.keyboard.press('Backspace');

    await expectFullScreenCommandSurface(journey.player, 'Выполняется запрос');
    await expect(dialog).toBeVisible();
    expect(await resolveCalls(journey.overseer)).toEqual([]);
    await expect(journey.player.locator('#backBtn')).toBeHidden();
  } finally {
    await closeApprovalJourney(journey);
  }
});

test('reject leaves the command initial and lets the controller return to the same menu', async ({ browser }) => {
  const journey = await openApprovalJourney(browser);
  try {
    const dialog = await chooseStateChangingCommand(journey);
    await dialog.getByRole('button', { name: 'ОТКЛОНИТЬ' }).click();

    await expect.poll(() => resolveCalls(journey.overseer)).toEqual([
      expect.objectContaining({
        args: [{ requestId: REQUEST_ID, decision: 'reject' }],
      }),
    ]);
    await expect(dialog).toBeHidden();
    await expectFullScreenCommandSurface(journey.player, 'Ошибка доступа');
    await expect(journey.player.locator('#entryBody')).toHaveText('Ошибка доступа');
    await expect(journey.player.locator('#backBtn')).toBeVisible();

    await journey.player.locator('#backBtn').click();
    await expect(journey.player.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible();
    await expect(journey.player.locator('.term-row', { hasText: 'Двери открыты' })).toHaveCount(0);
  } finally {
    await closeApprovalJourney(journey);
  }
});

test('closing the overseer dialog is exactly one rejection and never leaves players pending', async ({ browser }) => {
  const journey = await openApprovalJourney(browser);
  try {
    const dialog = await chooseStateChangingCommand(journey);
    await dialog.press('Escape');

    await expect.poll(() => resolveCalls(journey.overseer)).toEqual([
      expect.objectContaining({
        args: [{ requestId: REQUEST_ID, decision: 'reject' }],
      }),
    ]);
    await expect(dialog).toBeHidden();
    await expectFullScreenCommandSurface(journey.player, 'Ошибка доступа');
    await expect(journey.player.locator('#entryBody')).toHaveText('Ошибка доступа');
    await expect(journey.player.locator('#entryBody')).not.toContainText('Выполняется запрос');

    await journey.player.keyboard.press('Enter');
    await expect(journey.player.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible();
  } finally {
    await closeApprovalJourney(journey);
  }
});

test('approve persistence failure exposes no completed result and reports safe errors', async ({ browser, request }) => {
  const armed = await request.post(`${FIXTURE}/fail-next-save`);
  expect(armed.status()).toBe(204);

  const journey = await openApprovalJourney(browser);
  try {
    const dialog = await chooseStateChangingCommand(journey);
    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();

    await expect.poll(() => resolveCalls(journey.overseer)).toEqual([
      expect.objectContaining({
        args: [{ requestId: REQUEST_ID, decision: 'approve' }],
      }),
    ]);
    await expect(dialog).toBeHidden();
    const overseerError = journey.overseer.getByRole('alert').filter({ hasText: /сохран|состояни/i });
    await expect(overseerError).toBeVisible();
    await expect(overseerError).not.toContainText(/\/private\/|rename|fsync|temporary file/i);

    await expect(journey.player.locator('#playerNotice')).toContainText(/сохран|состояние команды не изменено/i);
    await expect(journey.player.locator('#entryBody')).not.toContainText('Доступ в сектор разрешён.');
    await expect(journey.player.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible();
    await expect(journey.player.locator('.term-row', { hasText: 'Двери открыты' })).toHaveCount(0);
  } finally {
    await closeApprovalJourney(journey);
  }
});

test('pending, rejected, and completed command states match the selected-record renderer', async ({ browser, request }) => {
  let journey = await openApprovalJourney(browser);
  try {
    await journey.player.locator('.term-row', { hasText: 'ЭТАЛОН РЕНДЕРА' }).click();
    await expect(journey.player.locator('#termEntry')).toBeVisible();
    await completeVisibleReveal(journey.player);
    const referenceWide = await recordRendererSnapshot(journey.player);
    const referenceWidePages = await pageCount(journey.player);
    expect(referenceWidePages).toBeGreaterThan(1);
    await journey.player.locator('#backBtn').click();
    await expect(journey.player.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible();

    let dialog = await chooseStateChangingCommand(journey);
    await completeVisibleReveal(journey.player);
    expect(await recordRendererSnapshot(journey.player)).toEqual(referenceWide);

    await dialog.getByRole('button', { name: 'ОТКЛОНИТЬ' }).click();
    await expectFullScreenCommandSurface(journey.player, 'Ошибка доступа');
    await completeVisibleReveal(journey.player);
    expect(await recordRendererSnapshot(journey.player)).toEqual(referenceWide);
    await journey.player.keyboard.press('Enter');
    await expect(journey.player.locator('.term-row', { hasText: 'Открыть двери' })).toBeVisible();

    await closeApprovalJourney(journey);
    await resetApprovalFixture(request);
    journey = await openApprovalJourney(browser);

    dialog = await chooseStateChangingCommand(journey);
    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
    await expectFullScreenCommandSurface(journey.player, 'Доступ в сектор разрешён.');
    await completeVisibleReveal(journey.player);
    expect(await recordRendererSnapshot(journey.player)).toEqual(referenceWide);
    await expect(journey.player.locator('#pageNext')).toBeVisible();

    await journey.player.setViewportSize({ width: 720, height: 520 });
    await expect.poll(() => pageCount(journey.player)).not.toBe(referenceWidePages);
    const completedNarrow = await recordRendererSnapshot(journey.player);
    const completedNarrowPages = await pageCount(journey.player);
    expect(completedNarrowPages).toBeGreaterThan(1);
    await journey.player.keyboard.press('Enter');

    await journey.player.locator('.term-row', { hasText: 'ЭТАЛОН РЕНДЕРА' }).click();
    await expect(journey.player.locator('#termEntry')).toBeVisible();
    await completeVisibleReveal(journey.player);
    expect(await recordRendererSnapshot(journey.player)).toEqual(completedNarrow);
    expect(await pageCount(journey.player)).toBe(completedNarrowPages);
    await journey.player.locator('#backBtn').click();

    await journey.overseer.reload();
    await mountOverseerCandidate(journey.overseer);
    await journey.overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
    await expect(journey.overseer.locator('#mainLayout')).toBeVisible();
    await journey.player.locator('.term-row', { hasText: 'Двери открыты' }).click();
    await expectFullScreenCommandSurface(journey.player, 'Выполняется запрос');
    dialog = journey.overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' }).first();
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('#commandExecutionDialogStatus')).toHaveText(
      `ЗАПРОС: ${REQUEST_ID} · РЕЖИМ: ЗАВЕРШЁННОЕ ИЗМЕНЕНИЕ СОСТОЯНИЯ · КОМАНДА: Двери открыты`,
    );
    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
    await expectFullScreenCommandSurface(journey.player, 'Доступ в сектор разрешён.');
    await completeVisibleReveal(journey.player);
    expect(await recordRendererSnapshot(journey.player)).toEqual(completedNarrow);
    expect(await pageCount(journey.player)).toBe(completedNarrowPages);
  } finally {
    await closeApprovalJourney(journey);
  }
});
