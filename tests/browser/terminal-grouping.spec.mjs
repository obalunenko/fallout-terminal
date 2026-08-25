import { expect, test } from '@playwright/test';

const FIXTURE = '/__fixture/terminal-grouping';
const OVERSEER = `${FIXTURE}/overseer`;
const PLAYER_TOKEN_KEY = 'fallout-terminal.player-token';

async function resetFixture(request, scenario) {
  const response = await request.post(`${FIXTURE}/reset`, { data: { scenario } });
  expect(response.ok()).toBe(true);
}

async function fixtureSession(request) {
  const response = await request.get(`${FIXTURE}/session`);
  expect(response.ok()).toBe(true);
  const snapshot = await response.json();
  return snapshot.session ?? snapshot;
}

async function activeFixtureSession(request) {
  const response = await request.get(`${FIXTURE}/open-session`);
  expect(response.ok()).toBe(true);
  const snapshot = await response.json();
  expect(snapshot.ok).toBe(true);
  return snapshot.session;
}

async function openOverseer(page) {
  await page.goto(OVERSEER);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
}

async function openParticipant(browser, token = '') {
  const context = await browser.newContext();
  await context.addInitScript(({ tokenKey, retainedToken }) => {
    HTMLMediaElement.prototype.play = () => Promise.resolve();
    try {
      if (retainedToken) localStorage.setItem(tokenKey, retainedToken);
      else localStorage.removeItem(tokenKey);
    } catch { /* about:blank */ }
  }, { tokenKey: PLAYER_TOKEN_KEY, retainedToken: token });
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  if (await page.locator('#characterSelect').isVisible()) {
    await page.locator('#characterOptions button:not([disabled])').first().click();
  }
  await expect(page.locator('#roleBadge')).toContainText(/АКТИВЕН|НАБЛЮДАТЕЛЬ/);
  return { context, page };
}

async function navigationSnapshot(request) {
  const response = await request.get(`${FIXTURE}/navigation-state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

async function expectPendingNavigation(page) {
  await expect(page.locator('#termEntry')).toBeVisible({ timeout: 2000 });
  await expect(page.locator('#entryBody')).toHaveText('Выполняется запрос');
  await expect(page.locator('#termList')).toBeHidden();
  await expect(page.locator('#backBtn')).toBeHidden();
}

async function expectJourneySurface(participants, { forwardCommand = '', returnTarget = '' }) {
  await Promise.all(participants.map(async participant => {
    await expect(participant.page.locator('#termList')).toBeVisible({ timeout: 2000 });
    if (forwardCommand) {
      await expect(participant.page.locator('.term-row', { hasText: forwardCommand })).toBeVisible();
    }
    const returnButton = participant.page.getByRole('button', { name: /НАЗАД В/i });
    if (returnTarget) await expect(returnButton).toHaveAccessibleName(`НАЗАД В ${returnTarget}`);
    else await expect(returnButton).toHaveCount(0);
  }));
}

async function approveJourneyStep({
  overseer,
  participants,
  request,
  action,
  direction,
  source,
  target,
  history,
}) {
  await action.click();
  await Promise.all(participants.map(participant => expectPendingNavigation(participant.page)));

  await expect.poll(async () => (await navigationSnapshot(request)).pendingTerminalNavigation)
    .toMatchObject({ direction, sourceTerminalId: source.id, targetTerminalId: target.id });
  const pending = await navigationSnapshot(request);
  expect(pending.activeTerminalId).toBe(source.id);
  expect(pending.activationHistory).toEqual(history);

  const dialog = overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' });
  await expect(dialog).toContainText(`ИЗ: ${source.name}`);
  await expect(dialog).toContainText(`В: ${target.name}`);
  await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
  await expect(dialog).toBeHidden();

  const expectedHistory = [...history, target.id];
  await expect.poll(async () => (await navigationSnapshot(request)).activeTerminalId).toBe(target.id);
  const approved = await navigationSnapshot(request);
  expect(approved.pendingTerminalNavigation).toBeNull();
  expect(approved.activationHistory).toEqual(expectedHistory);
  await Promise.all(participants.map(participant =>
    expect(participant.page.locator('#termList')).toBeVisible({ timeout: 2000 })));
  return expectedHistory;
}

function groupRows(page) {
  return page.locator('#termList > .terminal-group');
}

function groupRow(page, groupID) {
  return page.locator(`#termList > .terminal-group[data-group-id="${groupID}"]`);
}

function terminalRow(page, terminalID) {
  return page.locator(`.terminal-group-members > .term-row[data-terminal-id="${terminalID}"]`);
}

function groupActionTrigger(page, groupID) {
  return groupRow(page, groupID).locator('[data-action-menu-trigger="terminal-group"]');
}

function terminalActionTrigger(page, terminalID) {
  return terminalRow(page, terminalID).locator('[data-action-menu-trigger="terminal"]');
}

async function chooseGroupAction(page, groupID, action) {
  const group = groupRow(page, groupID);
  await groupActionTrigger(page, groupID).click();
  await group.locator(`.terminal-group-header > .terminal-action-menu [data-action="${action}"]`).click();
}

async function chooseTerminalAction(page, terminalID, action) {
  const terminal = terminalRow(page, terminalID);
  await terminalActionTrigger(page, terminalID).click();
  await terminal.locator(`:scope > .terminal-action-menu [data-action="${action}"]`).click();
}

function groupDraftDialog(page) {
  return page.locator('#terminalGroupDraftDialog');
}

function groupImpactDialog(page) {
  return page.locator('#terminalGroupImpactDialog');
}

async function mutationCallCount(page) {
  return page.evaluate(() => (__desktopFixture?.calls ?? [])
    .filter(call => call.method === 'ReplaceTerminalGroups').length);
}

async function reviewGroupDraft(page) {
  await groupDraftDialog(page).locator('[data-action="review-terminal-group-change"]').click();
  await expect(groupDraftDialog(page)).toBeHidden();
  await expect(groupImpactDialog(page)).toBeVisible();
}

async function confirmGroupImpact(page) {
  await groupImpactDialog(page).locator('[data-action="confirm-terminal-group-change"]').click();
  await expect(groupImpactDialog(page)).toBeHidden();
}

async function renderedGroups(page) {
  return groupRows(page).evaluateAll(groups => groups.map(group => ({
    id: group.dataset.groupId,
    name: group.querySelector('.terminal-group-name')?.textContent?.trim() ?? '',
    singleton: group.dataset.singleton === 'true',
    terminalIds: [...group.querySelectorAll('.terminal-group-members > .term-row')]
      .map(row => row.dataset.terminalId),
    terminalNames: [...group.querySelectorAll('.terminal-group-members > .term-row .term-row-name')]
      .map(name => name.textContent?.trim() ?? ''),
  })));
}

function expectedGroups(session) {
  const terminalNames = new Map(session.terminals.map(terminal => [terminal.id, terminal.name]));
  return session.terminalGroups.map(group => ({
    id: group.id,
    name: group.name,
    singleton: group.terminalIds.length === 1,
    terminalIds: group.terminalIds,
    terminalNames: group.terminalIds.map(id => terminalNames.get(id)),
  }));
}

test('renders groups as the only high-level terminal representation with exact-one membership', async ({ page, request }) => {
  await resetFixture(request, 'canonical');
  const session = await fixtureSession(request);

  await openOverseer(page);

  await expect(page.locator('#termList')).toHaveAttribute('aria-label', 'Группы терминалов');
  await expect(page.locator('#termList > .term-row')).toHaveCount(0);
  await expect(groupRows(page)).toHaveCount(session.terminalGroups.length);
  expect(await renderedGroups(page)).toEqual(expectedGroups(session));

  const renderedTerminalIDs = (await renderedGroups(page)).flatMap(group => group.terminalIds);
  expect(renderedTerminalIDs).toHaveLength(session.terminals.length);
  expect(new Set(renderedTerminalIDs).size).toBe(session.terminals.length);
  expect(new Set(renderedTerminalIDs)).toEqual(new Set(session.terminals.map(terminal => terminal.id)));
});

test('keeps the group hierarchy readable and contextual actions reachable at supported desktop viewports', async ({ page, request }) => {
  await resetFixture(request, 'ordered');

  for (const viewport of [{ width: 1280, height: 720 }, { width: 1600, height: 900 }]) {
    await page.setViewportSize(viewport);
    await openOverseer(page);

    await expect(page.locator('.term-panel > .panel-hdr').first()).toHaveText('ГРУППЫ И ТЕРМИНАЛЫ');
    await expect(groupRow(page, 'north-route').locator('.terminal-group-member-count')).toHaveText('2 ТЕРМИНАЛА');
    await expect(groupRow(page, 'north-route').locator('.terminal-group-name')).toHaveText('Северный маршрут');
    await expect(terminalRow(page, 'gamma').locator('.term-row-name')).toHaveText('Терминал Гамма');

    const layout = await page.locator('.term-panel').evaluate(panel => ({
      panelWidth: panel.getBoundingClientRect().width,
      panelOverflow: panel.scrollWidth > panel.clientWidth,
      clippedLabels: [...panel.querySelectorAll('.terminal-group-name, .term-row-name')]
        .filter(label => label.scrollWidth > label.clientWidth || label.scrollHeight > label.clientHeight)
        .map(label => label.textContent.trim()),
      overflowingRows: [...panel.querySelectorAll('.terminal-group-header, .term-row')]
        .filter(row => row.scrollWidth > row.clientWidth).length,
    }));
    expect(layout.panelWidth).toBeGreaterThanOrEqual(300);
    expect(layout.panelOverflow).toBe(false);
    expect(layout.clippedLabels).toEqual([]);
    expect(layout.overflowingRows).toBe(0);

    await expect(groupRow(page, 'north-route').locator('[data-action="rename-terminal-group"]')).toBeHidden();
    await groupActionTrigger(page, 'north-route').click();
    const groupMenu = groupRow(page, 'north-route')
      .locator('.terminal-group-header > .terminal-action-menu > .terminal-action-menu-panel');
    await expect(groupMenu).toBeVisible();
    await expect(groupMenu.getByRole('menuitem', { name: 'ПЕРЕИМЕНОВАТЬ ГРУППУ' })).toBeVisible();
    await expect(groupMenu.getByRole('menuitem', { name: 'ПЕРЕМЕСТИТЬ ГРУППУ ВНИЗ' })).toBeVisible();
    await expect(groupMenu.getByRole('menuitem', { name: 'РАСФОРМИРОВАТЬ ГРУППУ' }))
      .toHaveClass(/terminal-action-menu-destructive/);
    await page.keyboard.press('Escape');
    await expect(groupMenu).toBeHidden();
    await expect(groupActionTrigger(page, 'north-route')).toBeFocused();

    await terminalActionTrigger(page, 'gamma').click();
    const terminalMenu = terminalRow(page, 'gamma')
      .locator(':scope > .terminal-action-menu > .terminal-action-menu-panel');
    await expect(terminalMenu).toBeVisible();
    await expect(terminalMenu.getByRole('menuitem', { name: 'ПЕРЕИМЕНОВАТЬ ТЕРМИНАЛ' })).toBeVisible();
    await expect(terminalMenu.getByRole('menuitem', { name: 'ПЕРЕМЕСТИТЬ В ДРУГУЮ ГРУППУ' })).toBeVisible();
    await expect(terminalMenu.getByRole('menuitem', { name: 'УДАЛИТЬ ТЕРМИНАЛ' }))
      .toHaveClass(/terminal-action-menu-destructive/);
    await page.locator('.tree-hdr').click();
    await expect(terminalMenu).toBeHidden();
    await terminalActionTrigger(page, 'gamma').click();
    await page.keyboard.press('Escape');
    await expect(terminalMenu).toBeHidden();
    await expect(terminalActionTrigger(page, 'gamma')).toBeFocused();

    await chooseGroupAction(page, 'north-route', 'rename-terminal-group');
    await expect(groupDraftDialog(page)).toBeVisible();
    await groupDraftDialog(page).locator('[data-action="cancel-terminal-group-draft"]').click();
    await expect(groupDraftDialog(page)).toBeHidden();
    await expect(groupActionTrigger(page, 'north-route')).toBeFocused();

    const northToggle = groupRow(page, 'north-route').locator('[data-action="toggle-terminal-group"]');
    await expect(northToggle).toHaveAttribute('aria-expanded', 'true');
    await northToggle.focus();
    await page.keyboard.press('Enter');
    await expect(northToggle).toHaveAttribute('aria-expanded', 'false');
    await expect(terminalRow(page, 'gamma')).toBeHidden();

    await terminalRow(page, 'delta').click();
    await expect(northToggle).toHaveAttribute('aria-expanded', 'false');
    await northToggle.focus();
    await page.keyboard.press('Space');
    await expect(northToggle).toHaveAttribute('aria-expanded', 'true');
    await expect(terminalRow(page, 'gamma')).toBeVisible();
    await expect(terminalRow(page, 'delta')).toHaveClass(/editing/);

    await page.goto('about:blank');
  }
});

test('renders every standalone terminal through an ordinary singleton group', async ({ page, request }) => {
  await resetFixture(request, 'singleton');
  const session = await fixtureSession(request);
  expect(session.terminalGroups).toHaveLength(session.terminals.length);
  expect(session.terminalGroups.every(group => group.terminalIds.length === 1)).toBe(true);

  await openOverseer(page);

  const groups = await renderedGroups(page);
  expect(groups).toEqual(expectedGroups(session));
  expect(groups.every(group => group.singleton)).toBe(true);
  await expect(page.locator('.terminal-group[data-singleton="true"]')).toHaveCount(session.terminals.length);
});

test('preserves group and member order across a terminal save and reopen', async ({ page, request }) => {
  await resetFixture(request, 'ordered');
  const initial = await fixtureSession(request);
  await openOverseer(page);

  expect(await renderedGroups(page)).toEqual(expectedGroups(initial));

  await page.locator('#btnAddTerminal').click();
  const dialog = page.getByRole('dialog', { name: 'СОЗДАТЬ ТЕРМИНАЛ' });
  await expect(dialog).toBeVisible();
  await page.locator('#createTerminalName').fill('Резервный терминал');
  await dialog.getByRole('button', { name: 'СОЗДАТЬ ТЕРМИНАЛ' }).click();
  await expect(dialog).toBeHidden();
  await expect(page.locator('#saveStatus')).toContainText('Сохранено');

  const saved = await fixtureSession(request);
  const created = saved.terminals.find(terminal => terminal.name === 'Резервный терминал');
  expect(created).toBeTruthy();
  expect(saved.terminalGroups.find(group => group.terminalIds.includes(created.id))).toEqual(
    expect.objectContaining({ terminalIds: [created.id] }),
  );
  expect(saved.terminalGroups.slice(0, initial.terminalGroups.length)).toEqual(initial.terminalGroups);

  await page.reload();
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
  expect(await renderedGroups(page)).toEqual(expectedGroups(saved));
});

test('creates a group only after reviewing complete impact and renames it without destructive confirmation', async ({ page, request }) => {
  await resetFixture(request, 'canonical');
  const initial = await fixtureSession(request);
  await openOverseer(page);

  await page.locator('#btnCreateTerminalGroup').click();
  const draft = groupDraftDialog(page);
  await expect(draft).toBeVisible();
  await draft.locator('[name="groupName"]').fill('Северный маршрут');
  await draft.locator('[name="terminalIds"][value="security"]').check();
  await draft.locator('[name="terminalIds"][value="vault"]').check();
  await reviewGroupDraft(page);

  const impact = groupImpactDialog(page);
  await expect(impact).toContainText('СОЗДАНИЕ ГРУППЫ');
  await expect(impact).toContainText('Маршрут хранилища');
  await expect(impact).toContainText('Отдельное хранилище');
  await expect(impact).toContainText('Терминал охраны');
  await expect(impact).toContainText('Терминал хранилища');
  await expect(impact.locator('[data-impact="membership"]')).toContainText('Северный маршрут');
  await expect.poll(() => mutationCallCount(page)).toBe(0);

  await impact.locator('[data-action="cancel-terminal-group-change"]').click();
  await expect(impact).toBeHidden();
  expect(await renderedGroups(page)).toEqual(expectedGroups(initial));
  await expect.poll(() => mutationCallCount(page)).toBe(0);

  await page.locator('#btnCreateTerminalGroup').click();
  await draft.locator('[name="groupName"]').fill('Северный маршрут');
  await draft.locator('[name="terminalIds"][value="security"]').check();
  await draft.locator('[name="terminalIds"][value="vault"]').check();
  await reviewGroupDraft(page);
  await confirmGroupImpact(page);
  await expect.poll(() => mutationCallCount(page)).toBe(1);

  const created = groupRows(page).filter({ hasText: 'Северный маршрут' });
  await expect(created).toHaveCount(1);
  const createdID = await created.getAttribute('data-group-id');
  expect(createdID).toBeTruthy();
  await expect(created.locator('.term-row')).toHaveCount(2);

  await chooseGroupAction(page, createdID, 'rename-terminal-group');
  await expect(draft).toBeVisible();
  await draft.locator('[name="groupName"]').fill('Северный путь');
  await draft.locator('[data-action="save-terminal-group-rename"]').click();
  await expect(draft).toBeHidden();
  await expect(impact).toBeHidden();
  await expect(groupRow(page, createdID).locator('.terminal-group-name')).toHaveText('Северный путь');
  await expect(groupRow(page, createdID).locator('.term-row')).toHaveCount(2);
  await expect.poll(() => mutationCallCount(page)).toBe(2);
});

test('move and reorder previews are cancellable or closable with zero mutation', async ({ page, request }) => {
  await resetFixture(request, 'ordered');
  const initial = await fixtureSession(request);
  await openOverseer(page);

  await chooseTerminalAction(page, 'delta', 'move-terminal');
  const draft = groupDraftDialog(page);
  await expect(draft).toBeVisible();
  await draft.locator('[name="destinationGroupId"]').selectOption('north-route');
  await reviewGroupDraft(page);

  const impact = groupImpactDialog(page);
  await expect(impact).toContainText('ПЕРЕМЕЩЕНИЕ ТЕРМИНАЛА');
  await expect(impact).toContainText('Терминал Дельта');
  await expect(impact).toContainText('Южный маршрут');
  await expect(impact).toContainText('Северный маршрут');
  await page.keyboard.press('Escape');
  await expect(impact).toBeHidden();
  expect(await renderedGroups(page)).toEqual(expectedGroups(initial));
  await expect.poll(() => mutationCallCount(page)).toBe(0);

  await chooseTerminalAction(page, 'alpha', 'move-terminal-up');
  await expect(impact).toBeVisible();
  await expect(impact).toContainText('ИЗМЕНЕНИЕ ПОРЯДКА');
  await expect(impact.locator('[data-impact="order-before"]')).toContainText(/Гамма[\s\S]*Альфа/);
  await expect(impact.locator('[data-impact="order-after"]')).toContainText(/Альфа[\s\S]*Гамма/);
  await impact.locator('[data-action="close-terminal-group-change"]').click();
  await expect(impact).toBeHidden();
  expect(await renderedGroups(page)).toEqual(expectedGroups(initial));
  await expect.poll(() => mutationCallCount(page)).toBe(0);
});

test('splits one terminal into a collision-safe singleton only after confirmation', async ({ page, request }) => {
  await resetFixture(request, 'ordered');
  const initial = await fixtureSession(request);
  await openOverseer(page);

  await chooseTerminalAction(page, 'gamma', 'move-terminal');
  const draft = groupDraftDialog(page);
  await draft.locator('[name="destinationGroupId"]').selectOption({ label: 'НОВАЯ ОДИНОЧНАЯ ГРУППА' });
  await reviewGroupDraft(page);

  const impact = groupImpactDialog(page);
  await expect(impact).toContainText('ОТДЕЛЕНИЕ ТЕРМИНАЛА');
  await expect(impact.locator('[data-impact="source-group"]')).toHaveText('Северный маршрут');
  await expect(impact.locator('[data-impact="destination-group"]')).toHaveText('Терминал Гамма (2)');
  await expect(impact.locator('[data-impact="membership"]')).toContainText('Северный маршрут: Терминал Альфа');
  await expect(impact.locator('[data-impact="membership"]')).toContainText('Терминал Гамма (2): Терминал Гамма');
  expect(await renderedGroups(page)).toEqual(expectedGroups(initial));
  await expect.poll(() => mutationCallCount(page)).toBe(0);

  await confirmGroupImpact(page);
  await expect.poll(() => mutationCallCount(page)).toBe(1);

  const saved = await fixtureSession(request);
  expect(saved.terminalGroups.find(group => group.id === 'north-route')?.terminalIds).toEqual(['alpha']);
  expect(saved.terminalGroups.find(group => group.terminalIds.includes('gamma'))).toEqual(
    expect.objectContaining({ name: 'Терминал Гамма (2)', terminalIds: ['gamma'] }),
  );
  expect(saved.terminals).toEqual(initial.terminals);
});

test('moves, reorders, and dissolves groups atomically without losing terminal content', async ({ page, request }) => {
  await resetFixture(request, 'ordered');
  const initial = await fixtureSession(request);
  const initialTerminals = structuredClone(initial.terminals);
  await openOverseer(page);

  await chooseTerminalAction(page, 'delta', 'move-terminal');
  await groupDraftDialog(page).locator('[name="destinationGroupId"]').selectOption('north-route');
  await reviewGroupDraft(page);
  await confirmGroupImpact(page);
  await expect.poll(() => mutationCallCount(page)).toBe(1);
  expect((await renderedGroups(page)).find(group => group.id === 'north-route')?.terminalIds)
    .toEqual(['gamma', 'alpha', 'delta']);

  await chooseTerminalAction(page, 'delta', 'move-terminal-up');
  await confirmGroupImpact(page);
  await expect.poll(() => mutationCallCount(page)).toBe(2);
  expect((await renderedGroups(page)).find(group => group.id === 'north-route')?.terminalIds)
    .toEqual(['gamma', 'delta', 'alpha']);

  await chooseGroupAction(page, 'north-route', 'dissolve-terminal-group');
  const impact = groupImpactDialog(page);
  await expect(impact).toBeVisible();
  await expect(impact).toContainText('РАСФОРМИРОВАНИЕ ГРУППЫ');
  for (const name of ['Терминал Гамма', 'Терминал Дельта', 'Терминал Альфа']) {
    await expect(impact).toContainText(name);
  }
  await expect(impact.locator('[data-impact="groups"]')).toContainText('Северный маршрут');
  for (const resultantGroup of ['Терминал Гамма (2)', 'Терминал Дельта', 'Терминал Альфа']) {
    await expect(impact.locator('[data-impact="groups"]')).toContainText(resultantGroup);
  }
  const membership = impact.locator('[data-impact="membership"]');
  await expect(membership).toContainText('Терминал Гамма → Терминал Гамма (2)');
  await expect(membership).toContainText('Терминал Дельта → Терминал Дельта');
  await expect(membership).toContainText('Терминал Альфа → Терминал Альфа');
  await confirmGroupImpact(page);
  await expect.poll(() => mutationCallCount(page)).toBe(3);
  await expect(groupRow(page, 'north-route')).toHaveCount(0);

  const saved = await fixtureSession(request);
  expect(saved.terminals).toEqual(initialTerminals);
  const resultantNames = new Map([
    ['gamma', 'Терминал Гамма (2)'],
    ['delta', 'Терминал Дельта'],
    ['alpha', 'Терминал Альфа'],
  ]);
  for (const [terminalID, groupName] of resultantNames) {
    const resultant = saved.terminalGroups.find(group => group.terminalIds.includes(terminalID));
    expect(resultant?.terminalIds).toEqual([terminalID]);
    expect(resultant?.name).toBe(groupName);
  }
});

test('rejects a stale proposal, refreshes canonical state, and applies a reviewed retry once', async ({ page, request }) => {
  await resetFixture(request, 'ordered');
  const initial = await fixtureSession(request);
  await openOverseer(page);

  await chooseTerminalAction(page, 'delta', 'move-terminal');
  await groupDraftDialog(page).locator('[name="destinationGroupId"]').selectOption('north-route');
  await reviewGroupDraft(page);

  const advanced = await request.post(`${FIXTURE}/advance-revisions`, {
    data: { session: true, coordination: true },
  });
  expect(advanced.ok()).toBe(true);
  await groupImpactDialog(page).locator('[data-action="confirm-terminal-group-change"]').click();
  await expect(groupImpactDialog(page)).toBeHidden();
  await expect(page.locator('#terminalGroupError')).toContainText(/изменилось|устарел/i);
  expect(await renderedGroups(page)).toEqual(expectedGroups(initial));

  await chooseTerminalAction(page, 'delta', 'move-terminal');
  await groupDraftDialog(page).locator('[name="destinationGroupId"]').selectOption('north-route');
  await reviewGroupDraft(page);
  const confirm = groupImpactDialog(page).locator('[data-action="confirm-terminal-group-change"]');
  await confirm.evaluate(button => {
    button.click();
    button.click();
  });
  await expect(groupImpactDialog(page)).toBeHidden();
  await expect.poll(() => mutationCallCount(page)).toBe(2);
  expect((await renderedGroups(page)).find(group => group.id === 'north-route')?.terminalIds)
    .toEqual(['gamma', 'alpha', 'delta']);
});

test('rejects dissolving a singleton group without leaving its terminal ungrouped', async ({ page, request }) => {
  await resetFixture(request, 'singleton');
  const initial = await fixtureSession(request);
  await openOverseer(page);

  await chooseGroupAction(page, 'medical-singleton', 'dissolve-terminal-group');
  await expect(page.locator('#terminalGroupError')).toContainText(/одиночн|единственн|singleton/i);
  expect(await renderedGroups(page)).toEqual(expectedGroups(initial));
  const saved = await fixtureSession(request);
  expect(saved.terminalGroups.find(group => group.id === 'medical-singleton')?.terminalIds)
    .toEqual(['medical']);
});

test('rejects a group proposal that would split an authored transition', async ({ page, request }) => {
  await resetFixture(request, 'ordered-navigation');
  const initial = await fixtureSession(request);
  await openOverseer(page);

  await page.locator('#btnCreateTerminalGroup').click();
  const draft = groupDraftDialog(page);
  await draft.locator('[name="groupName"]').fill('Разорванный маршрут');
  await draft.locator('[name="terminalIds"][value="gamma"]').check();
  await draft.locator('[name="terminalIds"][value="delta"]').check();
  await reviewGroupDraft(page);
  await confirmGroupImpact(page);

  await expect.poll(() => mutationCallCount(page)).toBe(1);
  await expect(page.locator('#terminalGroupError')).toContainText('go-gamma');
  await expect(page.locator('#terminalGroupError')).toContainText('go-gamma-backup');
  await expect(page.locator('#terminalGroupError')).toContainText('beta');
  await expect(page.locator('#terminalGroupError')).toContainText('gamma');
  await expect(page.locator('#terminalGroupError')).toContainText(/crosses groups/i);
  expect(await renderedGroups(page)).toEqual(expectedGroups(initial));
  expect(await fixtureSession(request)).toEqual(initial);
});

test('legacy load keeps content intact, projects singleton groups, and leaves its cross-link dormant', async ({ browser, request }) => {
  await resetFixture(request, 'legacy');
  const persisted = await fixtureSession(request);
  expect(persisted.terminalGroups ?? []).toHaveLength(0);

  const normalized = await activeFixtureSession(request);
  expect(normalized.terminals).toEqual(persisted.terminals);
  expect(normalized.terminalGroups).toHaveLength(normalized.terminals.length);
  expect(normalized.terminalGroups.every(group => group.terminalIds.length === 1)).toBe(true);
  expect(new Set(normalized.terminalGroups.flatMap(group => group.terminalIds)))
    .toEqual(new Set(normalized.terminals.map(terminal => terminal.id)));

  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const controller = await openParticipant(browser);
  try {
    expect(await renderedGroups(overseer)).toEqual(expectedGroups(normalized));
    await expect(overseer.locator('.terminal-group[data-singleton="true"]'))
      .toHaveCount(normalized.terminals.length);

    const before = await navigationSnapshot(request);
    expect(before).toMatchObject({
      activeTerminalId: 'legacy-one',
      pendingTerminalNavigation: null,
      activationHistory: ['legacy-one'],
    });
    const oldLink = controller.page.locator('.term-row', { hasText: 'СТАРЫЙ ПЕРЕХОД' });
    await expect(oldLink).toBeVisible();
    await oldLink.click();

    await expect(overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' })).toBeHidden();
    await expect.poll(async () => (await navigationSnapshot(request)).pendingTerminalNavigation).toBeNull();
    const after = await navigationSnapshot(request);
    expect(after).toMatchObject({
      activeTerminalId: 'legacy-one',
      activationHistory: ['legacy-one'],
    });
    await expect(controller.page.locator('#termList')).toBeVisible();
    await expect(oldLink).toBeVisible();
    expect(await fixtureSession(request)).toEqual(persisted);
  } finally {
    await controller.context.close();
    await overseerContext.close();
  }
});

test('repairs a dormant legacy transition by moving its target into the source singleton', async ({ browser, request }) => {
  await resetFixture(request, 'legacy');
  const normalized = await activeFixtureSession(request);
  const sourceGroup = normalized.terminalGroups.find(group => group.terminalIds.includes('legacy-one'));
  const targetGroup = normalized.terminalGroups.find(group => group.terminalIds.includes('legacy-two'));
  expect(sourceGroup).toBeTruthy();
  expect(targetGroup).toBeTruthy();

  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  try {
    await chooseTerminalAction(overseer, 'legacy-two', 'move-terminal');
    await groupDraftDialog(overseer).locator('[name="destinationGroupId"]').selectOption(sourceGroup.id);
    await reviewGroupDraft(overseer);

    const impact = groupImpactDialog(overseer);
    await expect(impact.locator('[data-impact="source-group"]')).toHaveText(targetGroup.name);
    await expect(impact.locator('[data-impact="destination-group"]')).toHaveText(sourceGroup.name);
    await expect(impact.locator('[data-impact="membership"]'))
      .toHaveText(`${sourceGroup.name}: Старый терминал 1 → Старый терминал 2`);
    await expect(impact.locator('[data-impact="groups"]')).toContainText(targetGroup.name);
    await expect(impact.locator('[data-impact="groups"]')).toContainText(sourceGroup.name);

    await confirmGroupImpact(overseer);
    await expect.poll(() => mutationCallCount(overseer)).toBe(1);
    const repaired = await fixtureSession(request);
    expect(repaired.terminalGroups).toEqual([{
      id: sourceGroup.id,
      name: sourceGroup.name,
      terminalIds: ['legacy-one', 'legacy-two'],
    }, normalized.terminalGroups.find(group => group.terminalIds.includes('legacy-three'))]);
    expect(repaired.terminalGroups.some(group => group.id === targetGroup.id)).toBe(false);
    expect(repaired.terminals).toEqual(normalized.terminals);

    await overseer.reload();
    await openOverseer(overseer);
    expect(await renderedGroups(overseer)).toEqual(expectedGroups(repaired));

    const controller = await openParticipant(browser);
    try {
      const repairedLink = controller.page.locator('.term-row', { hasText: 'СТАРЫЙ ПЕРЕХОД' });
      await expect(repairedLink).toBeVisible();
      await repairedLink.click();
      await expectPendingNavigation(controller.page);
      await expect.poll(async () => (await navigationSnapshot(request)).pendingTerminalNavigation)
        .toMatchObject({
          direction: 'forward',
          sourceTerminalId: 'legacy-one',
          targetTerminalId: 'legacy-two',
        });
    } finally {
      await controller.context.close();
    }
  } finally {
    await overseerContext.close();
  }
});

test('ordinary command keeps its approval and reconnect behavior without changing the grouped route', async ({ browser, request }) => {
  await resetFixture(request, 'ordered-navigation');
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const controller = await openParticipant(browser);
  let observer = await openParticipant(browser);
  try {
    const before = await navigationSnapshot(request);
    expect(before).toMatchObject({
      activeTerminalId: 'gamma',
      pendingCommandExecution: null,
      pendingTerminalNavigation: null,
      activationHistory: ['gamma'],
    });

    await controller.page.locator('.term-row', { hasText: 'ПРОВЕРИТЬ СВЯЗЬ' }).click();
    await Promise.all([controller, observer].map(participant => expectPendingNavigation(participant.page)));
    await expect.poll(async () => (await navigationSnapshot(request)).pendingCommandExecution)
      .toMatchObject({ terminalId: 'gamma', commandName: 'ПРОВЕРИТЬ СВЯЗЬ' });
    await expect(overseer.getByRole('dialog', { name: 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ' })).toBeHidden();

    const retainedToken = await observer.page.evaluate(tokenKey => localStorage.getItem(tokenKey), PLAYER_TOKEN_KEY);
    await observer.context.close();
    observer = await openParticipant(browser, retainedToken);
    await expect(observer.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    await expectPendingNavigation(observer.page);

    const dialog = overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
    await expect(dialog).toContainText('РЕЖИМ: ОБЫЧНАЯ');
    await expect(dialog).toContainText('КОМАНДА: ПРОВЕРИТЬ СВЯЗЬ');
    await dialog.getByRole('button', { name: 'ОДОБРИТЬ' }).click();
	await expect(dialog).toBeHidden();
	await Promise.all([controller, observer].map(participant =>
	  expect(participant.page.locator('#entryBody')).toContainText('СВЯЗЬ СТАБИЛЬНА', { timeout: 2000 })));

    const approved = await navigationSnapshot(request);
    expect(approved).toMatchObject({
      activeTerminalId: 'gamma',
      pendingCommandExecution: null,
      pendingTerminalNavigation: null,
      activationHistory: ['gamma'],
    });
    await controller.page.keyboard.press('Backspace');
    await expectJourneySurface([controller, observer], {
      forwardCommand: 'ПЕРЕЙТИ В ТЕРМИНАЛ ДЕЛЬТА',
      returnTarget: 'Терминал Бета',
    });
  } finally {
    await controller.context.close();
    await observer.context.close();
    await overseerContext.close();
  }
});

test('ordered middle start completes C to B to A to B to C to D once across reconnect', async ({ browser, request }) => {
  await resetFixture(request, 'ordered-navigation');
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const controller = await openParticipant(browser);
  let observer = await openParticipant(browser);

  const terminals = {
    alpha: { id: 'alpha', name: 'Терминал Альфа' },
    beta: { id: 'beta', name: 'Терминал Бета' },
    gamma: { id: 'gamma', name: 'Терминал Гамма' },
    delta: { id: 'delta', name: 'Терминал Дельта' },
  };
  let history = ['gamma'];

  try {
    await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
    await expect(observer.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    expect(await navigationSnapshot(request)).toMatchObject({
      activeTerminalId: 'gamma',
      pendingTerminalNavigation: null,
      activationHistory: history,
    });
    await expectJourneySurface([controller, observer], {
      forwardCommand: 'ПЕРЕЙТИ В ТЕРМИНАЛ ДЕЛЬТА',
      returnTarget: terminals.beta.name,
    });

    history = await approveJourneyStep({
      overseer,
      participants: [controller, observer],
      request,
      action: controller.page.getByRole('button', { name: `НАЗАД В ${terminals.beta.name}` }),
      direction: 'return',
      source: terminals.gamma,
      target: terminals.beta,
      history,
    });
    await expectJourneySurface([controller, observer], {
      forwardCommand: 'ПЕРЕЙТИ В ТЕРМИНАЛ ГАММА',
      returnTarget: terminals.alpha.name,
    });

    history = await approveJourneyStep({
      overseer,
      participants: [controller, observer],
      request,
      action: controller.page.getByRole('button', { name: `НАЗАД В ${terminals.alpha.name}` }),
      direction: 'return',
      source: terminals.beta,
      target: terminals.alpha,
      history,
    });
    await expectJourneySurface([controller, observer], {
      forwardCommand: 'ПЕРЕЙТИ В ТЕРМИНАЛ БЕТА',
    });

    history = await approveJourneyStep({
      overseer,
      participants: [controller, observer],
      request,
      action: controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ТЕРМИНАЛ БЕТА' }),
      direction: 'forward',
      source: terminals.alpha,
      target: terminals.beta,
      history,
    });
    await expectJourneySurface([controller, observer], {
      forwardCommand: 'ПЕРЕЙТИ В ТЕРМИНАЛ ГАММА',
      returnTarget: terminals.alpha.name,
    });

    history = await approveJourneyStep({
      overseer,
      participants: [controller, observer],
      request,
      action: controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ТЕРМИНАЛ ГАММА' }),
      direction: 'forward',
      source: terminals.beta,
      target: terminals.gamma,
      history,
    });
    await expectJourneySurface([controller, observer], {
      forwardCommand: 'ПЕРЕЙТИ В ТЕРМИНАЛ ДЕЛЬТА',
      returnTarget: terminals.beta.name,
    });

    const retainedToken = await observer.page.evaluate(tokenKey => localStorage.getItem(tokenKey), PLAYER_TOKEN_KEY);
    await observer.context.close();
    observer = await openParticipant(browser, retainedToken);
    await expect(observer.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    await expectJourneySurface([controller, observer], {
      forwardCommand: 'ПЕРЕЙТИ В ТЕРМИНАЛ ДЕЛЬТА',
      returnTarget: terminals.beta.name,
    });
    expect(await navigationSnapshot(request)).toMatchObject({
      activeTerminalId: 'gamma',
      pendingTerminalNavigation: null,
      activationHistory: history,
    });

    history = await approveJourneyStep({
      overseer,
      participants: [controller, observer],
      request,
      action: controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ТЕРМИНАЛ ДЕЛЬТА' }),
      direction: 'forward',
      source: terminals.gamma,
      target: terminals.delta,
      history,
    });
    await expectJourneySurface([controller, observer], { returnTarget: terminals.gamma.name });

    expect(history).toEqual(['gamma', 'beta', 'alpha', 'beta', 'gamma', 'delta']);
    await expect.poll(() => overseer.evaluate(() => (__desktopFixture?.calls ?? [])
      .filter(call => call.method === 'ResolveTerminalNavigation' && call.args?.[0]?.decision === 'approve')
      .length)).toBe(5);
  } finally {
    await controller.context.close();
    await observer.context.close();
    await overseerContext.close();
  }
});

test('cross-group direct Overseer activation clears the player-created return route', async ({ browser, request }) => {
  await resetFixture(request, 'ordered-navigation');
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await openOverseer(overseer);
  const controller = await openParticipant(browser);
  const gamma = { id: 'gamma', name: 'Терминал Гамма' };
  const delta = { id: 'delta', name: 'Терминал Дельта' };

  try {
    let history = ['gamma'];
    history = await approveJourneyStep({
      overseer,
      participants: [controller],
      request,
      action: controller.page.locator('.term-row', { hasText: 'ПЕРЕЙТИ В ТЕРМИНАЛ ДЕЛЬТА' }),
      direction: 'forward',
      source: gamma,
      target: delta,
      history,
    });
    await expectJourneySurface([controller], { returnTarget: gamma.name });

    await terminalRow(overseer, 'epsilon').locator('.term-row-name').click();
    await expect(overseer.locator('#btnMakeLive')).toBeVisible();
    await overseer.locator('#btnMakeLive').click();

    await expect.poll(async () => (await navigationSnapshot(request)).activeTerminalId).toBe('epsilon');
    await expectJourneySurface([controller], {});
    const activated = await navigationSnapshot(request);
    expect(activated).toMatchObject({
      activeTerminalId: 'epsilon',
      pendingTerminalNavigation: null,
      activationHistory: [...history, 'epsilon'],
    });
  } finally {
    await controller.context.close();
    await overseerContext.close();
  }
});
