import { expect, test } from '@playwright/test';

const FIXTURE = '/__fixture/terminal-navigation';
const OVERSEER = `${FIXTURE}/overseer`;

async function openOverseer(page) {
  await page.goto(OVERSEER);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
}

async function selectCommand(page, name) {
  await page.locator('.tree-row', { hasText: name }).first().click();
  await expect(page.locator('#nodeForm')).toContainText('КОМАНДА');
}

async function openPlayer(browser) {
  const context = await browser.newContext();
  await context.addInitScript(() => {
    HTMLMediaElement.prototype.play = () => Promise.resolve();
    try { localStorage.removeItem('fallout-terminal.player-token'); } catch { /* about:blank */ }
  });
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await page.locator('#characterOptions button:not([disabled])').first().click();
  await expect(page.locator('#roleBadge')).toContainText('АКТИВЕН');
  return { context, page };
}

async function openParticipant(browser, token = '') {
  const context = await browser.newContext();
  await context.addInitScript(({ tokenKey, retainedToken }) => {
    HTMLMediaElement.prototype.play = () => Promise.resolve();
    try {
      if (retainedToken) localStorage.setItem(tokenKey, retainedToken);
      else localStorage.removeItem(tokenKey);
    } catch { /* about:blank */ }
  }, { tokenKey: 'fallout-terminal.player-token', retainedToken: token });
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  if (await page.locator('#characterSelect').isVisible()) {
    await page.locator('#characterOptions button:not([disabled])').first().click();
  }
  await expect(page.locator('#roleBadge')).toContainText(/АКТИВЕН|НАБЛЮДАТЕЛЬ/);
  return { context, page };
}

async function approveForwardTransition(overseer, player) {
  await player.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).click();
  await expectPendingTransitionSurface(player);
  const dialog = overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' });
  await expect(dialog).toHaveCount(1);
  await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
  await expect(player.locator('#hackHeader')).toBeVisible();
  await expect(player.locator('#playerNotice')).toBeHidden();
}

async function decideNavigation(overseer, decision) {
  const dialog = overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' });
  await expect(dialog).toHaveCount(1);
  await dialog.getByRole('button', { name: decision === 'approve' ? 'ОДОБРИТЬ' : 'ОТКЛОНИТЬ' }).click();
  await expect(dialog).toBeHidden();
}

async function finishHack(request, player) {
  expect((await request.post(`${FIXTURE}/force-hack`)).ok()).toBe(true);
  await expect(player.locator('#hackHeader')).toBeHidden({ timeout: 5000 });
  await expect(player.locator('#termList')).toBeVisible();
}

async function coordinationSnapshot(request) {
  const response = await request.get(`${FIXTURE}/state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

async function persistedCommandBehavior(page, commandID) {
  return page.evaluate(async ({ endpoint, id }) => {
    const response = await fetch(endpoint);
    if (!response.ok) return '';
    const session = await response.json();
    const command = session?.terminals?.[0]?.root?.children?.find(node => node.id === id);
    if (command?.stateChange) return 'state-change';
    if (command?.terminalTransition) return 'terminal-transition';
    return command ? 'ordinary' : '';
  }, { endpoint: `${FIXTURE}/session`, id: commandID });
}

async function expectPendingTransitionSurface(page, timeout = 2000) {
  await expect(page.locator('#termEntry')).toBeVisible({ timeout });
  await page.keyboard.press('Shift');
  await expect(page.locator('#entryBody')).toHaveText('Выполняется запрос', { timeout });
  await expect(page.locator('#termList')).toBeHidden({ timeout });
  await expect(page.locator('#termOutput')).toBeHidden({ timeout });
  await expect(page.locator('#termPrompt')).toBeVisible({ timeout });
  await expect(page.locator('#backBtn')).toBeHidden({ timeout });
  await expect(page.locator('#playerNotice')).toBeHidden({ timeout });
}

test.beforeEach(async ({ request }) => {
  const response = await request.post(`${FIXTURE}/reset`);
  expect(response.ok()).toBe(true);
});

test('active player identity is an immersive lower system line', async ({ browser }) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  await page.locator('#characterOptions button:not([disabled])').first().click();

  const status = page.locator('#playerIdentity');
  await expect(status).toBeVisible();
  await expect(status).toHaveText(/^\s*\[СИСТЕМА\] ВВОД\s+P\d+\s+\/\/\s+Mara\s+\/\/\s+АКТИВЕН\s*$/);
  await expect(status).not.toContainText('PLAYER');
  await expect(status).toHaveAttribute('role', 'status');
  await expect(page.locator('#roleBadge')).toHaveText('АКТИВЕН');
  await expect(page.locator('.player-identity, .role-badge')).toHaveCount(0);

  const presentation = await status.evaluate(element => {
    const statusBounds = element.getBoundingClientRect();
    const promptBounds = document.querySelector('#termPrompt').getBoundingClientRect();
    const style = getComputedStyle(element);
    return {
      beforePrompt: statusBounds.bottom <= promptBounds.top,
      borderStyle: style.borderStyle,
      backgroundColor: style.backgroundColor,
      textTransform: style.textTransform,
    };
  });
  expect(presentation).toEqual({
    beforePrompt: true,
    borderStyle: 'none',
    backgroundColor: 'rgba(0, 0, 0, 0)',
    textTransform: 'uppercase',
  });

  await context.close();
});

test('lower status stays contained on narrow, short, and hacking surfaces', async ({ browser }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);

  const playerContext = await browser.newContext({ viewport: { width: 520, height: 640 } });
  const player = await playerContext.newPage();
  await player.goto('/');
  await expect(player.locator('#connOverlay')).toBeHidden();
  await player.locator('#characterOptions button:not([disabled])').first().click();
  await expect(player.locator('#roleBadge')).toHaveText('АКТИВЕН');

  const statusGeometry = async contentSelector => player.evaluate(selector => {
    const screen = document.querySelector('#screen').getBoundingClientRect();
    const content = document.querySelector(selector).getBoundingClientRect();
    const status = document.querySelector('#playerIdentity').getBoundingClientRect();
    const promptElement = document.querySelector('#termPrompt');
    const prompt = promptElement.hidden ? null : promptElement.getBoundingClientRect();
    return {
      insideScreen: status.left >= screen.left && status.right <= screen.right &&
        status.top >= screen.top && status.bottom <= screen.bottom,
      afterContent: status.top >= content.bottom,
      beforePrompt: prompt === null || status.bottom <= prompt.top,
      pageOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
    };
  }, contentSelector);

  await expect(player.locator('#termList')).toBeVisible();
  expect(await statusGeometry('#termBody')).toEqual({
    insideScreen: true,
    afterContent: true,
    beforePrompt: true,
    pageOverflow: false,
  });

  await player.setViewportSize({ width: 1180, height: 420 });
  expect(await statusGeometry('#termBody')).toEqual({
    insideScreen: true,
    afterContent: true,
    beforePrompt: true,
    pageOverflow: false,
  });

  await player.setViewportSize({ width: 820, height: 600 });
  await approveForwardTransition(overseer, player);
  await expect(player.locator('#termPrompt')).toBeHidden();
  expect(await statusGeometry('#termBody')).toEqual({
    insideScreen: true,
    afterContent: true,
    beforePrompt: true,
    pageOverflow: false,
  });

  await playerContext.close();
  await overseerContext.close();
});

test('command authoring exposes one exclusive behavior selector', async ({ page }) => {
  await openOverseer(page);
  await selectCommand(page, 'ПЕРЕЙТИ В ОХРАНУ');

  const form = page.locator('#nodeForm');
  const mode = form.getByLabel('РЕЖИМ КОМАНДЫ');
  await expect(mode).toHaveValue('terminal-transition');
  await expect(mode.locator('option')).toHaveText([
    'ОБЫЧНАЯ КОМАНДА',
    'ИЗМЕНЯЕТ СОСТОЯНИЕ',
    'ПЕРЕХОД В ДРУГОЙ ТЕРМИНАЛ',
  ]);

  await mode.selectOption('state-change');
  await expect(form.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ')).toBeVisible();
  await expect(form.getByLabel('ЦЕЛЕВОЙ ТЕРМИНАЛ')).toBeHidden();

  await mode.selectOption('terminal-transition');
  await expect(form.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ')).toBeHidden();
  await expect(form.getByLabel('ЦЕЛЕВОЙ ТЕРМИНАЛ')).toBeVisible();

  await mode.selectOption('ordinary');
  await expect(form.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ')).toBeHidden();
  await expect(form.getByLabel('ЦЕЛЕВОЙ ТЕРМИНАЛ')).toBeHidden();
});

test('transition authoring is mutually exclusive, validates locally, and survives reopen', async ({ page }) => {
  await openOverseer(page);
  await selectCommand(page, 'ПЕРЕЙТИ В ОХРАНУ');
  const form = page.locator('#nodeForm');
  const mode = form.getByLabel('РЕЖИМ КОМАНДЫ');
  await expect(mode).toHaveValue('terminal-transition');

  await mode.selectOption('state-change');
  await form.getByLabel('ТЕКСТ УСПЕШНОГО ВЫПОЛНЕНИЯ').fill('Переход подготовлен.');
  await form.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ').fill('ПЕРЕХОД ПОДГОТОВЛЕН');
  await form.getByLabel('ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ').fill('Подготовить переход?');
  await form.getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
	await expect(page.locator('#saveStatus')).toContainText('Сохранено');
  const stateChangeSave = await page.evaluate(() => __desktopFixture.calls.filter(call => call.method === 'SaveSession').at(-1)?.args?.[0]
    ?.terminals?.[0]?.root?.children?.find(node => node.id === 'go-security'));
	expect(stateChangeSave).toHaveProperty('stateChange');
	expect(stateChangeSave).not.toHaveProperty('terminalTransition');
	await expect.poll(() => persistedCommandBehavior(page, 'go-security')).toBe('state-change');

	await page.reload();
  await openOverseer(page);
  await selectCommand(page, 'ПЕРЕЙТИ В ОХРАНУ');
  await expect(page.getByLabel('РЕЖИМ КОМАНДЫ')).toHaveValue('state-change');

  await page.getByLabel('РЕЖИМ КОМАНДЫ').selectOption('terminal-transition');
  await form.getByLabel('ЦЕЛЕВОЙ ТЕРМИНАЛ').selectOption('security');
  await form.getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
  const transitionSave = await page.evaluate(() => __desktopFixture.calls.filter(call => call.method === 'SaveSession').at(-1)?.args?.[0]
    ?.terminals?.[0]?.root?.children?.find(node => node.id === 'go-security'));
	expect(transitionSave).toHaveProperty('terminalTransition.targetTerminalId', 'security');
	expect(transitionSave).not.toHaveProperty('stateChange');
	await expect.poll(() => persistedCommandBehavior(page, 'go-security')).toBe('terminal-transition');

  await page.reload();
  await openOverseer(page);
  await expect(page.locator('.term-row')).toHaveCount(3);
  await selectCommand(page, 'ПЕРЕЙТИ В ОХРАНУ');
  await expect(page.getByLabel('РЕЖИМ КОМАНДЫ')).toHaveValue('terminal-transition');
  await expect(page.getByLabel('ЦЕЛЕВОЙ ТЕРМИНАЛ')).toHaveValue('security');

  await page.getByLabel('РЕЖИМ КОМАНДЫ').selectOption('ordinary');
  await form.getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
  const ordinarySave = await page.evaluate(() => __desktopFixture.calls.filter(call => call.method === 'SaveSession').at(-1)?.args?.[0]
    ?.terminals?.[0]?.root?.children?.find(node => node.id === 'go-security'));
	expect(ordinarySave).not.toHaveProperty('stateChange');
	expect(ordinarySave).not.toHaveProperty('terminalTransition');
	await expect.poll(() => persistedCommandBehavior(page, 'go-security')).toBe('ordinary');

  await page.reload();
  await openOverseer(page);
  await selectCommand(page, 'ПЕРЕЙТИ В ОХРАНУ');
  await expect(page.getByLabel('РЕЖИМ КОМАНДЫ')).toHaveValue('ordinary');

  await selectCommand(page, 'ЗАВЕРШЁННАЯ КОМАНДА');
  await expect(page.getByLabel('РЕЖИМ КОМАНДЫ')).toBeDisabled();
});

test('deleting a referenced terminal is blocked before local mutation', async ({ page }) => {
  await openOverseer(page);
	const security = page.locator('.term-row', { hasText: 'Терминал охраны' });
  await security.getByRole('button', { name: 'УДАЛИТЬ' }).click();
	await expect(page.locator('#coordinationError')).toContainText(/ссыла|переход/i);
	await expect(page.locator('.term-row', { hasText: 'Терминал охраны' })).toHaveCount(1);
});

test('one forward request opens one exact overseer dialog and close rejects it', async ({ page, request }) => {
  await openOverseer(page);
  const armed = await request.post(`${FIXTURE}/pending-forward`);
  expect(armed.ok()).toBe(true);
  const dialog = page.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' });
  await expect(dialog).toHaveCount(1);
  await expect(dialog).toContainText('ИЗ: Жилой терминал');
  await expect(dialog).toContainText('КОМАНДА: ПЕРЕЙТИ В ОХРАНУ');
  await expect(dialog).toContainText('В: Терминал охраны');
  await expect(dialog.getByRole('button', { name: 'ОДОБРИТЬ' })).toBeFocused();
  await dialog.press('ArrowRight');
  await expect(dialog.getByRole('button', { name: 'ОТКЛОНИТЬ' })).toBeFocused();
  await dialog.press('Escape');
  await expect.poll(() => page.evaluate(() => __desktopFixture.calls.filter(call => call.method === 'ResolveTerminalNavigation'))).toEqual([
    expect.objectContaining({ args: [{ requestId: 'navigation-forward-1', decision: 'reject' }] }),
  ]);
  await expect(dialog).toBeHidden();
});

test('approved first entry opens the destination hack at root without a terminal-switch decision', async ({ browser }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const player = await openPlayer(browser);
  try {
    await approveForwardTransition(overseer, player.page);
    await expect(overseer.getByRole('dialog', { name: /СМЕНА ТЕРМИНАЛА/i })).toHaveCount(0);
    await expect(player.page.locator('#attemptsLine')).toContainText('ОСТАЛОСЬ');
  } finally {
    await player.context.close();
    await overseerContext.close();
  }
});

test('same-group forward remains pending for controller and observer until one Overseer approval', async ({ browser, request }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const controller = await openParticipant(browser);
  const observer = await openParticipant(browser);
  try {
    await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
    await expect(observer.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    const before = await coordinationSnapshot(request);
    expect(before.broadcast.activeTerminalId).toBe('residential');
    expect(before.pendingTerminalNavigation).toBeNull();

    await controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first().click();
    await Promise.all([controller, observer].map(participant =>
      expectPendingTransitionSurface(participant.page)));

    const pending = await coordinationSnapshot(request);
    expect(pending.broadcast.activeTerminalId).toBe('residential');
    expect(pending.pendingTerminalNavigation).toMatchObject({
      direction: 'forward',
      sourceTerminalId: 'residential',
      targetTerminalId: 'security',
      routeDepth: 0,
    });
    const dialog = overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' });
    await expect(dialog).toContainText('ИЗ: Жилой терминал');
    await expect(dialog).toContainText('В: Терминал охраны');

    for (const participant of [controller, observer]) {
      await participant.page.keyboard.press('Enter');
      await participant.page.keyboard.press('Backspace');
      await expectPendingTransitionSurface(participant.page);
    }
    expect((await coordinationSnapshot(request)).broadcast.activeTerminalId).toBe('residential');

    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
    await expect(dialog).toBeHidden();
    await Promise.all([controller, observer].map(participant =>
      expect(participant.page.locator('#hackHeader')).toBeVisible({ timeout: 2000 })));
    const approved = await coordinationSnapshot(request);
    expect(approved.broadcast.activeTerminalId).toBe('security');
    expect(approved.pendingTerminalNavigation).toBeNull();
  } finally {
    await controller.context.close();
    await observer.context.close();
    await overseerContext.close();
  }
});

test('cross-group forward attempts by controller and observer have zero navigation effect', async ({ browser, request }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const controller = await openParticipant(browser);
  const observer = await openParticipant(browser);
  try {
    await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
    await expect(observer.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');

    await controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first().click();
    await decideNavigation(overseer, 'approve');
    await expect(controller.page.locator('#hackHeader')).toBeVisible();
    await finishHack(request, controller.page);
    await expect(observer.page.locator('#termList')).toBeVisible({ timeout: 2000 });

    const returnAction = /НАЗАД В Жилой терминал/i;
    await expect(controller.page.getByRole('button', { name: returnAction })).toHaveCount(1);
    await expect(observer.page.getByRole('button', { name: returnAction })).toHaveCount(1);
    const before = await coordinationSnapshot(request);
    expect(before.broadcast.activeTerminalId).toBe('security');
    expect(before.pendingTerminalNavigation).toBeNull();

    await controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ХРАНИЛИЩЕ' }).click();
    await expect(overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' })).toBeHidden();
    await expect.poll(async () => (await coordinationSnapshot(request)).pendingTerminalNavigation).toBeNull();
    let after = await coordinationSnapshot(request);
    expect(after.broadcast.activeTerminalId).toBe('security');
    await expect(controller.page.locator('#hackHeader')).toBeHidden();
    await expect(controller.page.getByRole('button', { name: returnAction })).toHaveCount(1);

    await observer.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ХРАНИЛИЩЕ' }).click({ force: true });
    await expect(overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' })).toBeHidden();
    await expect.poll(async () => (await coordinationSnapshot(request)).pendingTerminalNavigation).toBeNull();
    after = await coordinationSnapshot(request);
    expect(after.broadcast.activeTerminalId).toBe('security');
    await expect(observer.page.getByRole('button', { name: returnAction })).toHaveCount(1);
    await expect(controller.page.getByRole('button', { name: returnAction })).toHaveCount(1);
  } finally {
    await controller.context.close();
    await observer.context.close();
    await overseerContext.close();
  }
});

for (const command of [
  { mode: 'ordinary', modeLabel: 'ОБЫЧНАЯ', name: 'ЗАПУСТИТЬ ДИАГНОСТИКУ', result: 'СИСТЕМА ИСПРАВНА' },
  { mode: 'completed state-changing', modeLabel: 'ЗАВЕРШЁННОЕ ИЗМЕНЕНИЕ СОСТОЯНИЯ', name: 'ЗАВЕРШЁННАЯ КОМАНДА', result: 'Done' },
]) {
  test(`${command.mode} command preserves its approve/reject/close result across controller, observers, and reconnect`, async ({ browser, request }) => {
    const runDecision = async decision => {
      const overseerContext = await browser.newContext();
      const overseer = await overseerContext.newPage();
      await openOverseer(overseer);
      const controller = await openParticipant(browser);
      const firstObserver = await openParticipant(browser);
      let secondObserver = await openParticipant(browser);
      try {
        await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
        await expect(firstObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
        await expect(secondObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
        const sourceMenu = await controller.page.locator('#termList').textContent();

        await controller.page.locator('.term-row', { hasText: command.name }).click();
        await Promise.all([controller, firstObserver, secondObserver].map(participant =>
          expectPendingTransitionSurface(participant.page)));

        const dialog = overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
        await expect(dialog).toHaveCount(1);

        const stateResponse = await request.get(`${FIXTURE}/state`);
        expect(stateResponse.ok()).toBe(true);
        const state = await stateResponse.json();
        expect(state.broadcast.activeTerminalId).toBe('residential');
        expect(state.pendingCommandExecution).toMatchObject({
          terminalId: 'residential',
          commandName: command.name,
        });
        expect(state.pendingTerminalNavigation).toBeNull();
        await expect(dialog.locator('#commandExecutionDialogStatus')).toHaveText(
          `ЗАПРОС: ${state.pendingCommandExecution.requestId} · РЕЖИМ: ${command.modeLabel} · КОМАНДА: ${command.name}`,
        );

        const retainedToken = await secondObserver.page.evaluate(() =>
          localStorage.getItem('fallout-terminal.player-token'));
        await secondObserver.context.close();
        secondObserver = await openParticipant(browser, retainedToken);
        await expectPendingTransitionSurface(secondObserver.page);

        for (const participant of [controller, firstObserver, secondObserver]) {
          await participant.page.keyboard.press('Enter');
          await participant.page.keyboard.press('Backspace');
          await expectPendingTransitionSurface(participant.page);
        }

        if (decision === 'close') await dialog.press('Escape');
        else if (decision === 'reject') {
          await expect(dialog.getByRole('button', { name: 'ОДОБРИТЬ' })).toBeFocused();
          await dialog.press('ArrowRight');
          await expect(dialog.getByRole('button', { name: 'ОТКЛОНИТЬ' })).toBeFocused();
          await overseer.keyboard.press('Enter');
        } else {
          await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
        }
        await expect(dialog).toBeHidden();

        if (decision === 'approve') {
          await Promise.all([controller, firstObserver, secondObserver].map(async participant => {
            await expect(participant.page.locator('#termEntry')).toBeVisible({ timeout: 2000 });
            await participant.page.keyboard.press('Shift');
            await expect(participant.page.locator('#entryBody')).toHaveText(command.result);
            await expect(participant.page.locator('#termList')).toBeHidden();
          }));
        } else {
          await Promise.all([controller, firstObserver, secondObserver].map(async participant => {
            await expect(participant.page.locator('#termList')).toBeVisible({ timeout: 2000 });
            await expect(participant.page.locator('#termList')).toHaveText(sourceMenu);
            await expect(participant.page.locator('#termEntry')).toBeHidden();
          }));
        }
      } finally {
        await controller.context.close();
        await firstObserver.context.close();
        await secondObserver.context.close();
        await overseerContext.close();
      }
    };

    await runDecision('approve');
    expect((await request.post(FIXTURE + '/reset')).ok()).toBe(true);
    await runDecision('reject');
    expect((await request.post(FIXTURE + '/reset')).ok()).toBe(true);
    await runDecision('close');
  });
}

test('direct pending replaces every player menu with the inert record surface across reconnect and decisions', async ({ browser, request }) => {
  const runDecision = async decision => {
    const overseerContext = await browser.newContext();
    const overseer = await overseerContext.newPage();
    await openOverseer(overseer);
    const controller = await openParticipant(browser);
    const firstObserver = await openParticipant(browser);
    let secondObserver = await openParticipant(browser);
    try {
      await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
      await expect(firstObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
      await expect(secondObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
      const sourceMenu = await controller.page.locator('#termList').textContent();

      await controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first().click();
      await Promise.all([controller, firstObserver, secondObserver].map(participant =>
        expectPendingTransitionSurface(participant.page)));

      const retainedToken = await secondObserver.page.evaluate(() =>
        localStorage.getItem('fallout-terminal.player-token'));
      await secondObserver.context.close();
      secondObserver = await openParticipant(browser, retainedToken);
      await expectPendingTransitionSurface(secondObserver.page);

      for (const participant of [controller, firstObserver, secondObserver]) {
        await participant.page.keyboard.press('Enter');
        await participant.page.keyboard.press('Backspace');
        await expectPendingTransitionSurface(participant.page);
      }

      const dialog = overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' });
      await expect(dialog).toBeVisible();
      if (decision === 'close') await dialog.press('Escape');
      else await dialog.getByRole('button', { name: decision === 'approve' ? 'ОДОБРИТЬ' : 'ОТКЛОНИТЬ' }).click();
      await expect(dialog).toBeHidden();

      if (decision === 'approve') {
        await Promise.all([controller, firstObserver, secondObserver].map(participant =>
          expect(participant.page.locator('#hackHeader')).toBeVisible({ timeout: 2000 })));
      } else {
        await Promise.all([controller, firstObserver, secondObserver].map(async participant => {
          await expect(participant.page.locator('#termList')).toBeVisible({ timeout: 2000 });
          await expect(participant.page.locator('#termList')).toHaveText(sourceMenu);
          await expect(participant.page.locator('#termEntry')).toBeHidden();
        }));
      }
    } finally {
      await controller.context.close();
      await firstObserver.context.close();
      await secondObserver.context.close();
      await overseerContext.close();
    }
  };

  await runDecision('approve');
  expect((await request.post(FIXTURE + '/reset')).ok()).toBe(true);
  await runDecision('reject');
  expect((await request.post(FIXTURE + '/reset')).ok()).toBe(true);
  await runDecision('close');
});

test('return pending replaces every player menu with the inert record surface across reconnect and decisions', async ({ browser, request }) => {
  const runDecision = async decision => {
    const overseerContext = await browser.newContext();
    const overseer = await overseerContext.newPage();
    await openOverseer(overseer);
    const controller = await openParticipant(browser);
    const firstObserver = await openParticipant(browser);
    let secondObserver = await openParticipant(browser);
    try {
      await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
      await expect(firstObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
      await expect(secondObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');

      await controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first().click();
      await decideNavigation(overseer, 'approve');
      await expect(controller.page.locator('#hackHeader')).toBeVisible();
      await finishHack(request, controller.page);
      await Promise.all([firstObserver, secondObserver].map(participant =>
        expect(participant.page.locator('#termList')).toBeVisible({ timeout: 2000 })));

      const currentMenu = await controller.page.locator('#termList').textContent();
      const returnButton = controller.page.getByRole('button', { name: /НАЗАД В Жилой терминал/i });
      await expect(returnButton).toBeVisible();
      await returnButton.click();
      await Promise.all([controller, firstObserver, secondObserver].map(participant =>
        expectPendingTransitionSurface(participant.page)));

      const retainedToken = await secondObserver.page.evaluate(() =>
        localStorage.getItem('fallout-terminal.player-token'));
      await secondObserver.context.close();
      secondObserver = await openParticipant(browser, retainedToken);
      await expectPendingTransitionSurface(secondObserver.page);

      for (const participant of [controller, firstObserver, secondObserver]) {
        await participant.page.keyboard.press('Enter');
        await participant.page.keyboard.press('Backspace');
        await expectPendingTransitionSurface(participant.page);
      }

      const dialog = overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' });
      await expect(dialog).toBeVisible();
      if (decision === 'close') await dialog.press('Escape');
      else await dialog.getByRole('button', { name: decision === 'approve' ? 'ОДОБРИТЬ' : 'ОТКЛОНИТЬ' }).click();
      await expect(dialog).toBeHidden();

      if (decision === 'approve') {
        await Promise.all([controller, firstObserver, secondObserver].map(async participant => {
          await expect(participant.page.locator('#termList')).toBeVisible({ timeout: 2000 });
          await expect(participant.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first()).toBeVisible();
          await expect(participant.page.getByRole('button', { name: /НАЗАД В/i })).toHaveCount(0);
        }));
      } else {
        await Promise.all([controller, firstObserver, secondObserver].map(async participant => {
          await expect(participant.page.locator('#termList')).toBeVisible({ timeout: 2000 });
          await expect(participant.page.locator('#termList')).toHaveText(currentMenu);
          await expect(participant.page.locator('#termEntry')).toBeHidden();
          await expect(participant.page.getByRole('button', { name: /НАЗАД В Жилой терминал/i })).toBeVisible();
        }));
      }
    } finally {
      await controller.context.close();
      await firstObserver.context.close();
      await secondObserver.context.close();
      await overseerContext.close();
    }
  };

  await runDecision('approve');
  expect((await request.post(FIXTURE + '/reset')).ok()).toBe(true);
  await runDecision('reject');
  expect((await request.post(FIXTURE + '/reset')).ok()).toBe(true);
  await runDecision('close');
});

test('an unfinished destination hack resumes with the exact retained progress', async ({ browser, request }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const player = await openPlayer(browser);
  try {
    await approveForwardTransition(overseer, player.page);
    const words = player.page.locator('.hcell.word');
    await expect(words.first()).toBeVisible();
    const initialAttempts = await player.page.locator('#attemptsLine').textContent();
    await words.first().click();
    await expect.poll(async () => player.page.locator('#attemptsLine').textContent()).not.toBe(initialAttempts);
    const retainedAttempts = await player.page.locator('#attemptsLine').textContent();
    const retainedLog = await player.page.locator('#hackLog').textContent();

    expect((await request.post(`${FIXTURE}/switch-source`)).ok()).toBe(true);
    await expect(player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' })).toBeVisible();
    expect((await request.post(`${FIXTURE}/switch-target`)).ok()).toBe(true);
    await expect(player.page.locator('#hackHeader')).toBeVisible();
    await expect(player.page.locator('#attemptsLine')).toHaveText(retainedAttempts);
    await expect(player.page.locator('#hackLog')).toHaveText(retainedLog);
  } finally {
    await player.context.close();
    await overseerContext.close();
  }
});

test('root return stays pending on reject, then restores the nested source menu on approve', async ({ browser, request }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const player = await openPlayer(browser);
  try {
    await player.page.locator('.term-row', { hasText: 'НАВИГАЦИЯ' }).click();
    await player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ ИЗ ПАПКИ' }).click();
    await decideNavigation(overseer, 'approve');
    await expect(player.page.locator('#hackHeader')).toBeVisible();
    await finishHack(request, player.page);
		expect((await request.post(`${FIXTURE}/move-source-folder`)).ok()).toBe(true);

    const returnButton = player.page.getByRole('button', { name: /НАЗАД В Жилой терминал/i });
    await expect(returnButton).toBeVisible();
    await returnButton.click();
    await expectPendingTransitionSurface(player.page);
    await decideNavigation(overseer, 'reject');
    await expect(player.page.locator('#termList')).toBeVisible();
    await expect(returnButton).toBeVisible();
    await expect(returnButton).toBeEnabled();
    await returnButton.click();
    await expectPendingTransitionSurface(player.page);
    await decideNavigation(overseer, 'approve');

    await expect(player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ ИЗ ПАПКИ' })).toBeVisible();
    await expect(player.page.getByRole('button', { name: /НАЗАД В/i })).toHaveCount(0);
  } finally {
    await player.context.close();
    await overseerContext.close();
  }
});

test('deleted source folder falls back to root without losing the terminal route', async ({ browser, request }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const player = await openPlayer(browser);
  try {
    await player.page.locator('.term-row', { hasText: 'НАВИГАЦИЯ' }).click();
    await player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ ИЗ ПАПКИ' }).click();
    await decideNavigation(overseer, 'approve');
    await finishHack(request, player.page);
    expect((await request.post(`${FIXTURE}/delete-source-folder`)).ok()).toBe(true);
    await player.page.getByRole('button', { name: /НАЗАД В Жилой терминал/i }).click();
    await decideNavigation(overseer, 'approve');
    await expect(player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first()).toBeVisible();
    await expect(player.page.locator('.term-row', { hasText: 'НАВИГАЦИЯ' })).toHaveCount(0);
  } finally {
    await player.context.close();
    await overseerContext.close();
  }
});

test('controller and two observers reconnect during pending and converge before new-broadcast cleanup', async ({ browser, request }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const controller = await openParticipant(browser);
  let observer = await openParticipant(browser);
  const secondObserver = await openParticipant(browser);
  try {
    await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
    await expect(observer.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    await expect(secondObserver.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    await controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first().click();
    await Promise.all([controller, observer, secondObserver].map(participant =>
      expectPendingTransitionSurface(participant.page)));
    await Promise.all([controller, observer, secondObserver].map(async participant => {
      await participant.page.keyboard.press('Enter');
      await participant.page.keyboard.press('Backspace');
      await expectPendingTransitionSurface(participant.page);
    }));

    const token = await observer.page.evaluate(() => localStorage.getItem('fallout-terminal.player-token'));
    await observer.context.close();
    observer = await openParticipant(browser, token);
    await expectPendingTransitionSurface(observer.page);
    await decideNavigation(overseer, 'approve');
    await Promise.all([controller, observer, secondObserver].map(participant =>
      expect(participant.page.locator('#hackHeader')).toBeVisible({ timeout: 2000 })));

    expect((await request.post(`${FIXTURE}/new-broadcast`)).ok()).toBe(true);
    await Promise.all([controller, observer, secondObserver].map(participant =>
      expect(participant.page.locator('#characterSelect')).toBeVisible({ timeout: 2000 })));
    await Promise.all([controller, observer, secondObserver].map(participant =>
      expect(participant.page.getByRole('button', { name: /НАЗАД В/i })).toHaveCount(0)));
  } finally {
    await controller.context.close();
    await observer.context.close();
    await secondObserver.context.close();
    await overseerContext.close();
  }
});

test('stale target approval fails safely and keeps the source terminal active', async ({ browser, request }) => {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const player = await openPlayer(browser);
  try {
    await player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first().click();
    expect((await request.post(`${FIXTURE}/remove-target`)).ok()).toBe(true);
    await decideNavigation(overseer, 'approve');
    await expect(overseer.locator('#coordinationError')).toContainText(/ИЗМЕНИЛАСЬ|НЕ СУЩЕСТВУЕТ|no longer|changed/i);
    await expect(player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first()).toBeVisible();
    await expect(player.page.locator('#hackHeader')).toBeHidden();
  } finally {
    await player.context.close();
    await overseerContext.close();
  }
});

test('A to B to C returns unwind exactly B then A', async ({ browser, request }) => {
	expect((await request.post(`${FIXTURE}/group-full-route`)).ok()).toBe(true);
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const player = await openPlayer(browser);
  try {
    await player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first().click();
    await decideNavigation(overseer, 'approve');
    await finishHack(request, player.page);
    await player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ХРАНИЛИЩЕ' }).click();
    await decideNavigation(overseer, 'approve');

    await player.page.getByRole('button', { name: /НАЗАД В Терминал охраны/i }).click();
    await decideNavigation(overseer, 'approve');
    await expect(player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ХРАНИЛИЩЕ' })).toBeVisible();
    await player.page.getByRole('button', { name: /НАЗАД В Жилой терминал/i }).click();
    await decideNavigation(overseer, 'approve');
    await expect(player.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ОХРАНУ' }).first()).toBeVisible();
    await expect(player.page.getByRole('button', { name: /НАЗАД В/i })).toHaveCount(0);
  } finally {
    await player.context.close();
    await overseerContext.close();
  }
});
