import { expect, test } from '@playwright/test';

const TOKEN_KEY = 'fallout-terminal.player-token';
const FIXTURE = '/__fixture/state-changing-command-sync';
const OVERSEER_URL = `${FIXTURE}/overseer`;
const REACTOR_ENTRY_NAME = 'СОСТОЯНИЕ РЕАКТОРА';
const REACTOR_COMMANDS = Object.freeze({
  power: {
    id: 'n_reactor_power',
    name: 'Включить питание реактора',
    completedName: 'Питание реактора включено',
    confirmation: 'Подтвердить включение питания реактора?',
    result: 'Питание реактора включено.',
    blockId: 'b_reactor_power',
    completedText: 'ПИТАНИЕ: ВКЛЮЧЕНО',
  },
  cooling: {
    id: 'n_reactor_cooling',
    name: 'Запустить охлаждение',
    completedName: 'Охлаждение работает',
    confirmation: 'Подтвердить запуск охлаждения реактора?',
    result: 'Охлаждение реактора запущено.',
    blockId: 'b_reactor_cooling',
    completedText: 'ОХЛАЖДЕНИЕ: НОРМА',
  },
  air: {
    id: 'n_reactor_air',
    name: 'Проверить вентиляцию',
    completedName: 'Вентиляция проверена',
    confirmation: 'Подтвердить проверку вентиляции реактора?',
    result: 'Вентиляция проверена.',
    blockId: 'b_reactor_air',
    completedText: 'ВЕНТИЛЯЦИЯ: НОРМА',
  },
  note: {
    id: 'n_reactor_note',
    name: 'Очистить примечание',
    completedName: 'Примечание очищено',
    confirmation: 'Удалить служебное примечание?',
    result: 'Примечание очищено.',
    blockId: 'b_reactor_note',
    completedText: '',
  },
  lock: {
    id: 'n_reactor_lock',
    name: 'Снять блокировку реактора',
    completedName: 'Блокировка реактора снята',
    confirmation: 'Подтвердить снятие блокировки реактора?',
    result: 'Блокировка реактора снята.',
    blockId: 'b_reactor_lock',
    completedText: 'БЛОКИРОВКА: СНЯТА',
  },
});
const REACTOR_INITIAL_LINES = [
  'ПИТАНИЕ: ОТКЛЮЧЕНО', '',
  'СТАТУС: НЕИЗВЕСТЕН', '',
  'СТАТУС: НЕИЗВЕСТЕН', '', '', '',
  'БЛОКИРОВКА: ВКЛЮЧЕНА',
];
const REACTOR_COMPLETED_LINES = [
  'ПИТАНИЕ: ВКЛЮЧЕНО', '',
  'ОХЛАЖДЕНИЕ: НОРМА', '',
  'ВЕНТИЛЯЦИЯ: НОРМА', '', '', '',
  'БЛОКИРОВКА: СНЯТА',
];
const REACTOR_BLOCK_LINE_INDEX = Object.freeze({
  b_reactor_power: 0,
  b_reactor_cooling: 2,
  b_reactor_air: 4,
  b_reactor_lock: 8,
});

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
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(OVERSEER_URL);
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

async function chooseTargetedCommand(journey, request, command) {
  await journey.players[0].page.locator('.term-row', { hasText: command.name }).click();
  await Promise.all(journey.players.map(player =>
    expectFullScreenCommandSurface(player.page, 'Выполняется запрос')));

  const dialog = journey.overseer.page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
  await expect(dialog).toBeVisible();
  const stateResponse = await request.get(`${FIXTURE}/state`);
  expect(stateResponse.ok()).toBe(true);
  const state = await stateResponse.json();
  expect(state.pendingCommandExecution).toMatchObject({
    terminalId: 'terminal-stateful',
    commandId: command.id,
    commandName: command.name,
    mode: 'state-change',
  });
  await expect(dialog.locator('#commandExecutionDialogDescription')).toHaveText(command.confirmation);
  return dialog;
}

async function entryLines(page) {
  await completeVisibleReveal(page);
  return page.locator('#entryBody > div').allTextContents();
}

async function expectOpenReactorEntry(players, expectedLines, timeout = 5000) {
  await Promise.all(players.map(async (player) => {
    await expect(player.page.locator('#termEntry')).toBeVisible({ timeout });
    await expect(player.page.locator('#termList')).toBeHidden({ timeout });
    await expect(player.page.locator('#entryTitle')).toHaveText(REACTOR_ENTRY_NAME, { timeout });
    await expect.poll(() => entryLines(player.page), { timeout }).toEqual(expectedLines);
  }));
}

async function openReactorEntry(players) {
  await players[0].page.locator('.term-row', { hasText: REACTOR_ENTRY_NAME }).click();
  await expectOpenReactorEntry(players, REACTOR_INITIAL_LINES);
}

async function executeFiveTargetedCommands(journey, request, order) {
  const expectedLines = [...REACTOR_INITIAL_LINES];
  for (const command of order) {
    const dialog = await chooseTargetedCommand(journey, request, command);
    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
    await Promise.all(journey.players.map(player =>
      expectFullScreenCommandSurface(player.page, command.result)));
    await journey.players[0].page.keyboard.press('Enter');
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: command.completedName })).toBeVisible()));

    const changedLine = REACTOR_BLOCK_LINE_INDEX[command.blockId];
    if (changedLine !== undefined) expectedLines[changedLine] = command.completedText;
    await journey.players[0].page.locator('.term-row', { hasText: REACTOR_ENTRY_NAME }).click();
    await expectOpenReactorEntry(journey.players, expectedLines);
    await journey.players[0].page.locator('#backBtn').click();
  }

  const response = await request.get(`${FIXTURE}/session`);
  expect(response.ok()).toBe(true);
  const session = await response.json();
  const snapshots = session.terminals[0].commandStates;
  expect(Object.keys(snapshots).sort()).toEqual(order.map(command => command.id).sort());
  for (const command of order) {
    expect(snapshots[command.id]).toMatchObject({
      completedName: command.completedName,
      resultText: command.result,
      entryContentChange: {
        blockId: command.blockId,
        completedText: command.completedText,
      },
    });
  }
}

async function audit(request) {
  const response = await request.get(`${FIXTURE}/audit`);
  expect(response.ok()).toBe(true);
  return response.json();
}

async function coordinationState(request) {
  const response = await request.get(`${FIXTURE}/state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

async function durableCommandStates(request) {
  const response = await request.get(`${FIXTURE}/session`);
  expect(response.ok()).toBe(true);
  const session = await response.json();
  return session.terminals.find(terminal => terminal.id === 'terminal-stateful').commandStates ?? {};
}

async function executeFixtureCommands(request, commands) {
  const revisions = [];
  for (const command of commands) {
    const response = await request.post(`${FIXTURE}/execute-command`, {
      data: { commandId: command.id },
    });
    expect(response.status()).toBe(204);
    revisions.push((await coordinationState(request)).revision);
  }
  return revisions;
}

function effectiveReactorLines(commands) {
  const lines = [...REACTOR_INITIAL_LINES];
  for (const command of commands) {
    const index = REACTOR_BLOCK_LINE_INDEX[command.blockId];
    if (index !== undefined) lines[index] = command.completedText;
  }
  return lines;
}

function expectStrictlyIncreasing(values) {
  for (let index = 1; index < values.length; index += 1) {
    expect(values[index]).toBeGreaterThan(values[index - 1]);
  }
}

function expectFrozenCommands(states, commands) {
  for (const command of commands) {
    expect(states[command.id]).toEqual({
      completedName: command.completedName,
      resultText: command.result,
      entryContentChange: {
        blockId: command.blockId,
        completedText: command.completedText,
      },
    });
  }
}

test.beforeEach(async ({ request }) => {
  await postLifecycle(request, 'reset');
});

for (const [name, order] of [
  ['empty-first interleaving', [
    REACTOR_COMMANDS.note,
    REACTOR_COMMANDS.power,
    REACTOR_COMMANDS.lock,
    REACTOR_COMMANDS.cooling,
    REACTOR_COMMANDS.air,
  ]],
  ['reverse block order', [
    REACTOR_COMMANDS.lock,
    REACTOR_COMMANDS.note,
    REACTOR_COMMANDS.air,
    REACTOR_COMMANDS.cooling,
    REACTOR_COMMANDS.power,
  ]],
]) {
  test(`five block commands converge independently in ${name}`, async ({ browser, request }) => {
    const journey = await openThreePlayerJourney(browser);
    try {
      await executeFiveTargetedCommands(journey, request, order);
      await journey.players[0].page.locator('.term-row', { hasText: REACTOR_ENTRY_NAME }).click();
      await expectOpenReactorEntry(journey.players, REACTOR_COMPLETED_LINES);
      await expectMatchingRecordRenderers(journey.players);
      expect(await audit(request)).toMatchObject({ executeWrites: 5, pendingRequests: 0 });
    } finally {
      await closeJourney(journey);
    }
  });
}

test('an already-open entry updates atomically for observers and a reconnecting observer', async ({ browser, request }) => {
  const journey = await openThreePlayerJourney(browser);
  try {
    await openReactorEntry(journey.players);
    const retainedToken = await journey.players[2].page.evaluate(key => localStorage.getItem(key), TOKEN_KEY);
    expect(retainedToken).toMatch(/\S+/);

    const response = await request.post(`${FIXTURE}/execute-command`, {
      data: { commandId: REACTOR_COMMANDS.power.id },
    });
    expect(response.ok()).toBe(true);
    const powerCompletedLines = [...REACTOR_INITIAL_LINES];
    powerCompletedLines[0] = REACTOR_COMMANDS.power.completedText;
    await expectOpenReactorEntry(journey.players, powerCompletedLines, 1000);
    await expectMatchingRecordRenderers(journey.players);

    await journey.players[2].context.close();
    const reconnected = await openPlayer(browser, retainedToken);
    journey.players[2] = reconnected;
    await expect(reconnected.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    await expectOpenReactorEntry(journey.players, powerCompletedLines);
    expect(await reconnected.page.evaluate(key => localStorage.getItem(key), TOKEN_KEY)).toBe(retainedToken);
    expect(await audit(request)).toMatchObject({ executeWrites: 1, pendingRequests: 0 });
  } finally {
    await closeJourney(journey);
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

test('targeted frozen outcomes survive terminal switching and broadcast stop-start with monotonic revisions', async ({ browser, request }) => {
  const journey = await openThreePlayerJourney(browser);
  const completed = [REACTOR_COMMANDS.power, REACTOR_COMMANDS.note];
  const expectedLines = effectiveReactorLines(completed);
  try {
    const revisions = [
      (await coordinationState(request)).revision,
      ...await executeFixtureCommands(request, completed),
    ];
    expectStrictlyIncreasing(revisions);
    expectFrozenCommands(await durableCommandStates(request), completed);

    await journey.players[0].page.locator('.term-row', { hasText: REACTOR_ENTRY_NAME }).click();
    await expectOpenReactorEntry(journey.players, expectedLines);

    await postLifecycle(request, 'switch-away');
    revisions.push((await coordinationState(request)).revision);
    await Promise.all(journey.players.map(player =>
      expect(player.page.locator('.term-row', { hasText: 'РЕЗЕРВНЫЙ СТАТУС' })).toBeVisible()));
    expectFrozenCommands(await durableCommandStates(request), completed);

    await postLifecycle(request, 'switch-back');
    revisions.push((await coordinationState(request)).revision);
    // Switching back restores the shared navigation checkpoint, so every
    // participant returns directly to the entry that was open before the switch.
    await expectOpenReactorEntry(journey.players, expectedLines);

    await postLifecycle(request, 'restart-broadcast');
    revisions.push((await coordinationState(request)).revision);
    await Promise.all(journey.players.map(player => expect(player.page.locator('#characterSelect')).toBeVisible()));
    await assignControllerAndObservers(journey.players, REACTOR_COMMANDS.power.completedName);
    await journey.players[0].page.locator('.term-row', { hasText: REACTOR_ENTRY_NAME }).click();
    await expectOpenReactorEntry(journey.players, expectedLines);

    expectStrictlyIncreasing(revisions);
    expectFrozenCommands(await durableCommandStates(request), completed);
    expect(await audit(request)).toMatchObject({ executeWrites: 2, pendingRequests: 0 });
  } finally {
    await closeJourney(journey);
  }
});

test('application and session reopen, authored publication, and reconnect retain valid frozen block outcomes', async ({ browser, request }) => {
  const journey = await openThreePlayerJourney(browser);
  const completed = [REACTOR_COMMANDS.cooling, REACTOR_COMMANDS.lock];
  const expectedLines = effectiveReactorLines(completed);
  try {
    const revisions = [
      (await coordinationState(request)).revision,
      ...await executeFixtureCommands(request, completed),
    ];
    expectStrictlyIncreasing(revisions);
    const frozenBeforePublication = await durableCommandStates(request);
    expectFrozenCommands(frozenBeforePublication, completed);

    await journey.players[0].page.locator('.term-row', { hasText: REACTOR_ENTRY_NAME }).click();
    await expectOpenReactorEntry(journey.players, expectedLines);

    await journey.overseer.page.reload();
    await journey.overseer.page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
    await expect(journey.overseer.page.locator('#mainLayout')).toBeVisible();
    const publish = journey.overseer.page.getByRole('button', { name: 'ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ' });
    await expect(publish).toBeVisible();
    await publish.click();
    await expect.poll(() => journey.overseer.page.evaluate(() =>
      __desktopFixture.calls.filter(call => call.method === 'UpdateLiveTerminal').length)).toBe(1);
    revisions.push((await coordinationState(request)).revision);
    await expectOpenReactorEntry(journey.players, expectedLines);
    expect(await durableCommandStates(request)).toEqual(frozenBeforePublication);

    await postLifecycle(request, 'reopen-session');
    revisions.push((await coordinationState(request)).revision);
    await Promise.all(journey.players.map(player => expect(player.page.locator('#characterSelect')).toBeVisible()));
    await assignControllerAndObservers(journey.players, REACTOR_COMMANDS.cooling.completedName);

    const retainedToken = await journey.players[2].page.evaluate(key => localStorage.getItem(key), TOKEN_KEY);
    expect(retainedToken).toMatch(/\S+/);
    await journey.players[2].context.close();
    const reconnected = await openPlayer(browser, retainedToken);
    journey.players[2] = reconnected;
    await expect(reconnected.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    await expect(reconnected.page.locator('.term-row', { hasText: REACTOR_COMMANDS.lock.completedName })).toBeVisible();

    await journey.players[0].page.locator('.term-row', { hasText: REACTOR_ENTRY_NAME }).click();
    await expectOpenReactorEntry(journey.players, expectedLines);
    expect(await reconnected.page.evaluate(key => localStorage.getItem(key), TOKEN_KEY)).toBe(retainedToken);
    expect(await durableCommandStates(request)).toEqual(frozenBeforePublication);

    for (let index = 1; index < revisions.length; index += 1) {
      expect(revisions[index]).toBeGreaterThanOrEqual(revisions[index - 1]);
    }
    expect(revisions.at(-1)).toBeGreaterThan(revisions[0]);
    expect(await audit(request)).toMatchObject({ executeWrites: 2, pendingRequests: 0 });
  } finally {
    await closeJourney(journey);
  }
});
