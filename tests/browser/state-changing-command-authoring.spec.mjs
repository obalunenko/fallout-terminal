import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';

const FIXTURE_URL = '/__fixture/state-changing-command-authoring';
const TERMINAL_ID = 'terminal-stateful';
const BUNDLED_DEMO_URL = new URL('../../sessions/demo.json', import.meta.url);
const BLOCK_ENTRY_ID = 'entry-reactor-state';
const BLOCK_FOLDER_ID = 'folder-reactor-state';
const BLOCK_OWNER_COMMAND_ID = 'command-reactor-power';
const BLOCK_IDS = Object.freeze({
  power: 'block-reactor-power',
  duplicateOne: 'block-reactor-status-one',
  duplicateTwo: 'block-reactor-status-two',
  empty: 'block-reactor-empty',
  long: 'block-reactor-long',
});
const LONG_BLOCK_TEXT = '  Аварийный\nконтур   реактора — проверка стабилизации давления и температуры завершена  ';

async function openAuthoringFixture(page) {
  await page.goto(FIXTURE_URL);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
  await expect(page.locator('#editingTermName')).toHaveText('Терминал охраны');
}

async function selectCommand(page, displayedName) {
  const row = page.locator('.tree-row', { hasText: displayedName });
  await expect(row).toHaveCount(1);
  await row.click();
  await expect(page.locator('#nodeForm')).toContainText('КОМАНДА');
}

async function desktopCallCount(page, method) {
  return page.evaluate(name => __desktopFixture.calls.filter(call => call.method === name).length, method);
}

async function lastDesktopCall(page, method) {
  return page.evaluate(name => __desktopFixture.calls.filter(call => call.method === name).at(-1) ?? null, method);
}

async function commandFromLastSave(page, commandID) {
  return page.evaluate(({ id }) => {
    const call = __desktopFixture.calls.filter(candidate => candidate.method === 'SaveSession').at(-1);
    const session = call?.args?.[0];
    const visit = (node) => {
      if (!node || typeof node !== 'object') return null;
      if (node.id === id) return node;
      for (const child of node.children ?? []) {
        const found = visit(child);
        if (found) return found;
      }
      return null;
    };
    return visit(session?.terminals?.[0]?.root);
  }, { id: commandID });
}

async function nodeFromLastSave(page, nodeID) {
  return commandFromLastSave(page, nodeID);
}

function findFixtureNode(node, nodeID) {
  if (node?.id === nodeID) return node;
  for (const child of node?.children ?? []) {
    const found = findFixtureNode(child, nodeID);
    if (found) return found;
  }
  return null;
}

async function seedBlockAuthoringSession(page, {
  completedOwner = false,
  completedSecondary = false,
} = {}) {
  const snapshotResponse = await page.request.get(`${FIXTURE_URL}/session`);
  expect(snapshotResponse.ok()).toBe(true);
  const snapshot = await snapshotResponse.json();
  const session = snapshot.session;
  const terminal = session.terminals.find(candidate => candidate.id === TERMINAL_ID);
  terminal.root.children.push(
    {
      id: BLOCK_FOLDER_ID,
      type: 'folder',
      name: 'РЕАКТОРНЫЙ КОНТУР',
      children: [{
        id: BLOCK_ENTRY_ID,
        type: 'entry',
        name: 'СОСТОЯНИЕ РЕАКТОРА',
        blocks: [
          { id: BLOCK_IDS.power, initialText: 'ПИТАНИЕ: ОТКЛЮЧЕНО' },
          { id: BLOCK_IDS.duplicateOne, initialText: 'СТАТУС: НЕИЗВЕСТЕН' },
          { id: BLOCK_IDS.duplicateTwo, initialText: 'СТАТУС: НЕИЗВЕСТЕН' },
          { id: BLOCK_IDS.empty, initialText: ' \n\t ' },
          { id: BLOCK_IDS.long, initialText: LONG_BLOCK_TEXT },
        ],
      }],
    },
    {
      id: BLOCK_OWNER_COMMAND_ID,
      type: 'command',
      name: 'Включить питание реактора',
      text: 'Питание реактора включено.',
      stateChange: {
        completedName: 'Питание реактора включено',
        confirmationText: 'Подтвердить включение питания?',
        entryContentChange: {
          blockId: BLOCK_IDS.power,
          completedText: 'ПИТАНИЕ: ВКЛЮЧЕНО',
        },
      },
    },
    {
      id: 'command-reactor-secondary',
      type: 'command',
      name: 'Проверить резервный контур',
      text: 'Резервный контур проверен.',
      stateChange: {
        completedName: 'Резервный контур проверен',
        confirmationText: 'Подтвердить проверку резервного контура?',
      },
    },
    {
      id: 'entry-legacy-record',
      type: 'entry',
      name: 'СТАРАЯ ЗАПИСЬ',
      description: 'НАСЛЕДУЕМОЕ ОПИСАНИЕ БЕЗ БЛОКОВ',
    },
  );
  if (completedOwner) {
    terminal.commandStates[BLOCK_OWNER_COMMAND_ID] = {
      completedName: 'Питание реактора включено',
      resultText: 'Питание реактора включено.',
      entryContentChange: {
        blockId: BLOCK_IDS.power,
        completedText: 'ПИТАНИЕ: ВКЛЮЧЕНО',
      },
    };
  }
  if (completedSecondary) {
    const secondary = findFixtureNode(terminal.root, 'command-reactor-secondary');
    secondary.stateChange.entryContentChange = {
      blockId: BLOCK_IDS.empty,
      completedText: '',
    };
    terminal.commandStates['command-reactor-secondary'] = {
      completedName: 'Резервный контур проверен',
      resultText: 'Резервный контур проверен.',
      entryContentChange: {
        blockId: BLOCK_IDS.empty,
        completedText: '',
      },
    };
  }
  session.terminals.push({
    id: 'terminal-foreign-blocks',
    name: 'Соседний терминал',
    hackLevel: 0,
    introText: '',
    root: {
      id: 'root',
      type: 'folder',
      name: 'ROOT',
      children: [{
        id: 'foreign-entry',
        type: 'entry',
        name: 'ЧУЖАЯ ЗАПИСЬ',
        blocks: [{ id: 'foreign-block', initialText: 'ЧУЖОЙ БЛОК' }],
      }],
    },
  });
  session.terminalGroups.push({
    id: 'foreign-blocks-group',
    name: 'Соседний терминал',
    terminalIds: ['terminal-foreign-blocks'],
  });

  const saveResponse = await page.request.post(`${FIXTURE_URL}/save`, { data: session });
  expect(saveResponse.ok()).toBe(true);
  await page.reload();
  await openAuthoringFixture(page);
}

async function selectEntry(page, displayedName) {
  const row = page.locator('.tree-row', { hasText: displayedName });
  if (await row.count() === 0) {
    const folder = page.locator('.tree-row', { hasText: 'РЕАКТОРНЫЙ КОНТУР' });
    if (await folder.count() === 1) await folder.click();
  }
  await expect(row).toHaveCount(1);
  await row.click();
  await expect(page.locator('#nodeForm')).toContainText('ЗАПИСЬ');
}

function normalizedPreview(value) {
  const collapsed = value.trim().replace(/\s+/gu, ' ');
  if (!collapsed) return 'ПУСТО';
  const codePoints = Array.from(collapsed);
  return codePoints.length > 48 ? `${codePoints.slice(0, 48).join('')}…` : collapsed;
}

function entryBlockOwner(page, blockID, commandID) {
  return page.locator(
    `.entry-content-block[data-block-id="${blockID}"] [data-entry-block-owner][data-command-id="${commandID}"]`,
  );
}

async function authoringDurableState(page) {
  return page.evaluate(() => __desktopFixture.authoringDurableState());
}

async function emitCoordination(page, revision, activeTerminalId = null, { roster = [], sessions = [] } = {}) {
  await page.evaluate(({ nextRevision, liveTerminalId, nextRoster, nextSessions }) => {
    __desktopFixture.emit('coordination-state', {
      revision: nextRevision,
      playerConfig: {
        name: 'Игроки теста',
        filePath: '/private/tmp/fallout-overseer-actions-players.json',
        revision: 1,
      },
      roster: nextRoster,
      sessions: nextSessions,
      broadcast: {
        id: 'broadcast-overseer-actions',
        activeTerminalId: liveTerminalId,
      },
    });
  }, {
    nextRevision: revision,
    liveTerminalId: activeTerminalId,
    nextRoster: roster,
    nextSessions: sessions,
  });
}

test.beforeEach(async ({ page }) => {
  const reset = await page.request.post(`${FIXTURE_URL}/reset`);
  expect(reset.ok()).toBe(true);
  await openAuthoringFixture(page);
});

test('terminal actions follow selected-terminal, editor, and broadcast context', async ({ page }) => {
  const makeLive = page.locator('#btnMakeLive');
  const liveFlag = page.locator('#liveFlag');
  const publish = page.locator('#btnPublish');
  const settings = page.locator('#terminalSettingsMenu');
  const takeOffAir = page.locator('#btnStopBroadcast');

  await expect(page.locator('#btnAddTerminal')).toHaveText('+ СОЗДАТЬ ТЕРМИНАЛ');
  await expect(makeLive).toHaveText('СДЕЛАТЬ АКТИВНЫМ');
  await expect(makeLive).toBeDisabled();
  await expect(liveFlag).toBeHidden();
  await expect(publish).toBeHidden();
  await expect(settings).toBeHidden();
  await expect(takeOffAir).toBeHidden();

  await emitCoordination(page, 20, 'another-terminal');
  await expect(makeLive).toBeVisible();
  await expect(makeLive).toBeEnabled();
  await expect(publish).toBeHidden();
  await expect(settings).toBeHidden();
  await expect(takeOffAir).toBeVisible();
  await makeLive.click();
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalActivation')).toBe(1);

  await emitCoordination(page, 21, TERMINAL_ID);
  await expect(makeLive).toBeHidden();
  await expect(liveFlag).toBeVisible();
  await expect(liveFlag).toContainText('В ЭФИРЕ');
  await expect(publish).toBeVisible();
  await expect(publish).toHaveText('ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ');
  await expect(settings).toBeVisible();

  const disclosure = settings.getByText('ДОПОЛНИТЕЛЬНО', { exact: true });
  await disclosure.focus();
  await page.keyboard.press('Enter');
  await expect(settings).toHaveAttribute('open', '');
  await expect(settings).toContainText(/все настройки.*не только содержимое/i);
  await settings.getByRole('button', { name: 'ПЕРЕПРИМЕНИТЬ НАСТРОЙКИ' }).click();
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalActivation')).toBe(2);
  await expect(settings).not.toHaveAttribute('open', '');
});

test('logical-session details and controls live in a reactive keyboard-accessible dialog', async ({ page }) => {
  const roster = [
    { id: 'character-1', name: 'Амата', claimedBySessionId: 'session-1' },
    { id: 'character-2', name: 'Буч', claimedBySessionId: '' },
  ];
  const sessions = [
    {
      id: 'session-1',
      fallbackName: 'Убежище 101',
      connected: true,
      role: 'active',
      character: { id: 'character-1', name: 'Амата' },
    },
    {
      id: 'session-2',
      fallbackName: 'Пип-Бой гостя',
      connected: true,
      role: 'observer',
      character: null,
    },
    {
      id: 'session-3',
      fallbackName: 'Старая сессия',
      connected: false,
      role: 'unassigned',
      character: null,
    },
  ];

  const opener = page.locator('#btnManageLogicalSessions');
  const dialog = page.getByRole('dialog', { name: 'ЛОГИЧЕСКИЕ СЕССИИ' });
  await expect(page.locator('#activeLogicalSessionCount')).toHaveText('0');
  await expect(page.locator('#coordinationPanel #logicalSessionList')).toHaveCount(0);
  await expect(page.locator('#coordinationPanel .logical-session-row')).toHaveCount(0);
  await opener.click();
  await expect(dialog.locator('.session-empty')).toHaveText('СЕССИИ НЕ ПОДКЛЮЧЕНЫ');
  await page.keyboard.press('Escape');
  await expect(opener).toBeFocused();

  await emitCoordination(page, 60, TERMINAL_ID, { roster, sessions });
  await expect(page.locator('#activeLogicalSessionCount')).toHaveText('2');
  await expect(page.locator('#coordinationPanel')).not.toContainText('Убежище 101');
  await expect(page.locator('#coordinationPanel')).not.toContainText('Пип-Бой гостя');

  await opener.click();
  await expect(dialog).toBeVisible();
  await expect(page.locator('#btnCloseLogicalSessions')).toBeFocused();
  await expect(dialog.locator('.logical-session-row')).toHaveCount(3);
  await expect(dialog).toContainText('Убежище 101');
  await expect(dialog).toContainText('Амата');
  await expect(dialog).toContainText('УПРАВЛЯЮЩИЙ');
  await expect(dialog).toContainText('Старая сессия');
  await expect(dialog).toContainText('ОТКЛЮЧЕН');

  const observerRow = dialog.locator('[data-session-id="session-2"]');
  await expect(observerRow.locator('.session-assign')).toBeEnabled();
  await observerRow.locator('.session-name-input').fill('Гостевой терминал');
  await observerRow.locator('.session-rename').click();
  await expect.poll(() => desktopCallCount(page, 'RenameLogicalSession')).toBe(1);
  await expect(dialog.locator('#logicalSessionDialogStatus')).toHaveText('МЕТКА СЕССИИ ОБНОВЛЕНА');

  await emitCoordination(page, 61, TERMINAL_ID, {
    roster,
    sessions: [
      { ...sessions[0], connected: false, role: 'observer' },
      {
        id: 'session-4',
        fallbackName: 'Новый контроллер',
        connected: true,
        role: 'active',
        character: null,
      },
    ],
  });
  await expect(page.locator('#activeLogicalSessionCount')).toHaveText('1');
  await expect(dialog.locator('.logical-session-row')).toHaveCount(2);
  await expect(dialog).toContainText('Новый контроллер');
  await expect(dialog).not.toContainText('Пип-Бой гостя');

  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(opener).toBeFocused();

  await opener.click();
  await page.locator('#btnCloseLogicalSessions').click();
  await expect(dialog).toBeHidden();
  await expect(opener).toBeFocused();
});

test('counts zero through seven sessions and keeps a seven-session mobile dialog scrollable', async ({ page }) => {
  let latestSessions = [];
  for (let total = 0; total <= 7; total += 1) {
    latestSessions = Array.from({ length: total }, (_, index) => ({
      id: `mobile-session-${index + 1}`,
      fallbackName: `Мобильная сессия ${index + 1}`,
      connected: index % 2 === 0,
      role: index === 0 ? 'active' : 'observer',
      character: null,
    }));
    await emitCoordination(page, 70 + total, TERMINAL_ID, { sessions: latestSessions });
    await expect(page.locator('#activeLogicalSessionCount')).toHaveText(String(Math.ceil(total / 2)));
  }

  await page.setViewportSize({ width: 390, height: 640 });
  await page.locator('#btnManageLogicalSessions').click();
  const dialog = page.getByRole('dialog', { name: 'ЛОГИЧЕСКИЕ СЕССИИ' });
  const list = dialog.locator('#logicalSessionList');
  await expect(dialog).toBeVisible();
  await expect(list.locator('.logical-session-row')).toHaveCount(7);
  await list.locator('[data-session-id="mobile-session-7"]').scrollIntoViewIfNeeded();
  await expect(list.locator('[data-session-id="mobile-session-7"]')).toBeVisible();

  const metrics = await dialog.evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    const sessionList = element.querySelector('#logicalSessionList');
    const style = getComputedStyle(sessionList);
    return {
      top: bounds.top,
      right: bounds.right,
      bottom: bounds.bottom,
      left: bounds.left,
      clientHeight: sessionList.clientHeight,
      scrollHeight: sessionList.scrollHeight,
      overflowY: style.overflowY,
    };
  });
  expect(metrics.left).toBeGreaterThanOrEqual(0);
  expect(metrics.top).toBeGreaterThanOrEqual(0);
  expect(metrics.right).toBeLessThanOrEqual(390);
  expect(metrics.bottom).toBeLessThanOrEqual(640);
  expect(metrics.scrollHeight).toBeGreaterThan(metrics.clientHeight);
  expect(metrics.overflowY).toMatch(/auto|scroll/);
});

test('publishing uses only the live-content command and preserves active identity', async ({ page }) => {
  await emitCoordination(page, 30, TERMINAL_ID);
  const publish = page.locator('#btnPublish');
  await expect(publish).toBeVisible();

  await page.evaluate(() => __desktopFixture.deferTerminalAction('UpdateLiveTerminal'));
  await publish.click();
  await expect.poll(() => desktopCallCount(page, 'UpdateLiveTerminal')).toBe(1);
  await expect(publish).toBeDisabled();
  await publish.click({ force: true });
  await expect.poll(() => desktopCallCount(page, 'UpdateLiveTerminal')).toBe(1);
  await page.evaluate(() => __desktopFixture.resolveTerminalAction('UpdateLiveTerminal', {
    ok: true,
    status: 'updated',
  }));
  await expect(publish).toBeEnabled();
  await expect(publish).toHaveText('ОБНОВЛЕНО ✓');

  const publishCall = await lastDesktopCall(page, 'UpdateLiveTerminal');
  expect(publishCall.args).toHaveLength(1);
  expect(publishCall.args[0]).toEqual(expect.objectContaining({
    introText: expect.any(String),
    tree: expect.objectContaining({ id: 'root', type: 'folder' }),
  }));
  expect(await desktopCallCount(page, 'RequestTerminalActivation')).toBe(0);
  await expect(page.locator('#liveFlag')).toBeVisible();

  await page.evaluate(() => __desktopFixture.setNextTerminalActionResult('UpdateLiveTerminal', {
    ok: false,
    error: 'fixture publish failed',
  }));
  await publish.click();
  await expect(page.locator('#coordinationError')).toContainText('fixture publish failed');
  await expect(page.locator('#liveFlag')).toBeVisible();

  await emitCoordination(page, 31, 'another-terminal');
  await expect(publish).toBeHidden();
});

test('terminal creation validates a name and mutates the session only after confirmation', async ({ page }) => {
  const createButton = page.locator('#btnAddTerminal');
  const dialog = page.getByRole('dialog', { name: 'СОЗДАТЬ ТЕРМИНАЛ' });
  const nameInput = page.locator('#createTerminalName');
  const saveCountBefore = await desktopCallCount(page, 'SaveSession');

  await createButton.click();
  await expect(dialog).toBeVisible();
  await expect(nameInput).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(createButton).toBeFocused();
  await expect.poll(() => desktopCallCount(page, 'SaveSession')).toBe(saveCountBefore);

  await createButton.click();
  await dialog.getByRole('button', { name: 'СОЗДАТЬ ТЕРМИНАЛ' }).click();
  await expect(page.locator('#createTerminalError')).toHaveText('УКАЖИТЕ НАЗВАНИЕ ТЕРМИНАЛА');
  await expect(dialog).toBeVisible();
  await expect(nameInput).toBeFocused();
  await expect.poll(() => desktopCallCount(page, 'SaveSession')).toBe(saveCountBefore);

  await nameInput.fill('  Технический терминал  ');
  await dialog.getByRole('button', { name: 'СОЗДАТЬ ТЕРМИНАЛ' }).click();
  await expect(dialog).toBeHidden();
  await expect(page.locator('#editingTermName')).toHaveText('Технический терминал');
  await expect(page.locator('.term-row', { hasText: 'Технический терминал' })).toHaveCount(1);
  await expect.poll(() => desktopCallCount(page, 'SaveSession')).toBe(saveCountBefore + 1);
  expect((await lastDesktopCall(page, 'SaveSession')).args[0].terminals.at(-1)).toEqual(expect.objectContaining({
    name: 'Технический терминал',
    hackLevel: 0,
    introText: '',
  }));
  expect(await desktopCallCount(page, 'RequestTerminalActivation')).toBe(0);
});

test('take-off-air always confirms, exposes failures, and chains unfinished progress', async ({ page }) => {
  await emitCoordination(page, 40, TERMINAL_ID);
  const takeOffAir = page.locator('#btnStopBroadcast');
  const dialog = page.getByRole('dialog', { name: 'СНЯТЬ ТЕРМИНАЛ С ЭФИРА?' });

  await takeOffAir.click();
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText('Трансляция, подключения, роли, назначения и сохранённый терминал останутся без изменений.');
  await expect(page.locator('#btnCancelTakeOffAir')).toBeFocused();
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalClear')).toBe(0);
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(takeOffAir).toBeFocused();

  await page.evaluate(() => __desktopFixture.setNextTerminalActionResult('RequestTerminalClear', {
    ok: false,
    error: 'fixture clear failed',
  }));
  await takeOffAir.click();
  await page.locator('#btnConfirmTakeOffAir').click();
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalClear')).toBe(1);
  await expect(dialog).toBeVisible();
  await expect(page.locator('#takeOffAirError')).toContainText('fixture clear failed');
  await expect(page.locator('#liveFlag')).toBeVisible();

  await page.evaluate(() => __desktopFixture.setNextTerminalActionResult('RequestTerminalClear', {
    ok: true,
    status: 'decision-required',
    switchId: 'switch-clear-1',
  }));
  await page.locator('#btnConfirmTakeOffAir').click();
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalClear')).toBe(2);
  await expect(dialog).toBeHidden();
  await expect(page.getByRole('dialog', { name: 'НЕЗАВЕРШЁННЫЙ ВЗЛОМ' })).toBeVisible();
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalClear')).toBe(2);
});

test('take-off-air prevents duplicates and focuses the surviving broadcast control', async ({ page }) => {
  await emitCoordination(page, 50, TERMINAL_ID);
  const dialog = page.getByRole('dialog', { name: 'СНЯТЬ ТЕРМИНАЛ С ЭФИРА?' });
  await page.evaluate(() => __desktopFixture.deferTerminalAction('RequestTerminalClear'));
  await page.locator('#btnStopBroadcast').click();
  await page.locator('#btnConfirmTakeOffAir').click();
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalClear')).toBe(1);
  await expect(page.locator('#btnConfirmTakeOffAir')).toBeDisabled();
  await page.locator('#btnConfirmTakeOffAir').click({ force: true });
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalClear')).toBe(1);

  await page.evaluate(() => __desktopFixture.resolveTerminalAction('RequestTerminalClear', {
    ok: true,
    status: 'cleared',
    state: {
      revision: 51,
      playerConfig: {
        name: 'Игроки теста',
        filePath: '/private/tmp/fallout-overseer-actions-players.json',
        revision: 1,
      },
      roster: [],
      sessions: [],
      broadcast: { id: 'broadcast-overseer-actions', activeTerminalId: null },
    },
  }));
  await expect(dialog).toBeHidden();
  await expect(page.locator('#btnStopBroadcast')).toBeHidden();
  await expect(page.locator('#btnEndBroadcast')).toBeFocused();
  await expect(page.locator('#broadcastSummary')).toContainText('ОЖИДАНИЕ ТЕРМИНАЛА');
});

test('bundled read-only demo exposes every configurable command mode and a completed example', async () => {
  const demo = JSON.parse(await readFile(BUNDLED_DEMO_URL, 'utf8'));
  const terminalIDs = new Set(demo.terminals.map(terminal => terminal.id));
  expect([...terminalIDs]).toEqual(['t_demo1', 't_demo2']);
  expect(demo.playerConfig).toBe('demo-players.json');
  expect(demo.terminals.some(terminal => terminal.hackLevel === 0)).toBe(true);
  expect(demo.terminals.some(terminal => terminal.hackLevel > 0 && terminal.hackLevel <= 5)).toBe(true);
  expect(demo.terminals.some(terminal => terminal.introText.trim() !== '')).toBe(true);
  expect(demo.terminals.some(terminal => terminal.introText === '')).toBe(true);
  const nodes = demo.terminals.flatMap((terminal) => {
    const collected = [];
    const visit = (node, isRoot = false) => {
      if (!isRoot) collected.push(node);
      for (const child of node.children ?? []) visit(child);
    };
    visit(terminal.root, true);
    return collected;
  });
  const commands = nodes.filter(node => node.type === 'command');
  const ordinaryCommands = commands.filter(command => !command.stateChange && !command.terminalTransition);
  const stateChangingCommands = commands.filter(command => command.stateChange);
  const terminalTransitionCommands = commands.filter(command => command.terminalTransition);
  const completed = demo.terminals.flatMap(terminal =>
    Object.entries(terminal.commandStates ?? {}).map(([commandId, state]) => ({ commandId, state })));

  expect(nodes.some(node => node.type === 'folder')).toBe(true);
  expect(nodes.some(node => node.type === 'entry')).toBe(true);
  expect(ordinaryCommands.length).toBeGreaterThan(0);
  for (const command of ordinaryCommands) {
    expect(command.name.trim()).not.toBe('');
    expect(command.text.trim()).not.toBe('');
  }
  expect(stateChangingCommands.length).toBeGreaterThan(0);
  for (const command of stateChangingCommands) {
    expect(command.name.trim()).not.toBe('');
    expect(command.text.trim()).not.toBe('');
    expect(command.stateChange?.completedName?.trim()).not.toBe('');
    expect(command.stateChange?.confirmationText?.trim()).not.toBe('');
    expect(command.stateChange.confirmationText.trim()).not.toBe(command.name.trim());
    expect(command.terminalTransition).toBeUndefined();
  }
  expect(terminalTransitionCommands.length).toBeGreaterThan(0);
  for (const command of terminalTransitionCommands) {
    expect(command.name.trim()).not.toBe('');
    expect(command.stateChange).toBeUndefined();
    expect(command.terminalTransition.targetTerminalId.trim()).not.toBe('');
    expect(terminalIDs.has(command.terminalTransition.targetTerminalId)).toBe(true);
  }
  const stateChangingCommandIDs = new Set(stateChangingCommands.map(command => command.id));
  expect(completed.length).toBeGreaterThan(0);
  for (const snapshot of completed) {
    expect(stateChangingCommandIDs.has(snapshot.commandId)).toBe(true);
    expect(snapshot.state.completedName.trim()).not.toBe('');
    expect(snapshot.state.resultText.trim()).not.toBe('');
  }
});

test('state-change mode requires all four authored texts and persists one config', async ({ page }) => {
  await selectCommand(page, 'Включить аварийный свет');

  const form = page.locator('#nodeForm');
  const mode = form.getByLabel('РЕЖИМ КОМАНДЫ');
  const initialName = form.getByLabel('ИСХОДНОЕ НАЗВАНИЕ');
  const completedName = form.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ');
  const confirmationText = form.getByLabel('ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ');
  const successText = form.getByLabel('ТЕКСТ УСПЕШНОГО ВЫПОЛНЕНИЯ');

  await expect(mode).toHaveValue('ordinary');
  await expect(completedName).toBeHidden();
  await expect(confirmationText).toBeHidden();
  await mode.selectOption('state-change');
  await expect(completedName).toBeVisible();
  await expect(confirmationText).toBeVisible();

  const authored = [
    { field: initialName, value: 'Включить красный свет', error: /исходн.*назван/i },
    { field: completedName, value: 'Красный свет включён', error: /назван.*после/i },
    { field: confirmationText, value: 'Включить аварийное освещение?', error: /подтвержден|запрос/i },
    { field: successText, value: 'Аварийное освещение включено.', error: /успешн|результат/i },
  ];
  for (const item of authored) await item.field.fill(item.value);

  const saveCountBeforeValidation = await desktopCallCount(page, 'SaveSession');
  for (const item of authored) {
    await item.field.fill(' \t ');
    await form.getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
    await expect(form.getByRole('alert')).toContainText(item.error);
    await expect.poll(() => desktopCallCount(page, 'SaveSession')).toBe(saveCountBeforeValidation);
    await item.field.fill(item.value);
  }

  await form.getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
  await expect.poll(() => desktopCallCount(page, 'SaveSession')).toBe(saveCountBeforeValidation + 1);
  await expect.poll(() => commandFromLastSave(page, 'emergency-lights')).toEqual(expect.objectContaining({
    id: 'emergency-lights',
    type: 'command',
    name: 'Включить красный свет',
    text: 'Аварийное освещение включено.',
    stateChange: {
      completedName: 'Красный свет включён',
      confirmationText: 'Включить аварийное освещение?',
    },
  }));

  await mode.selectOption('ordinary');
  await expect(completedName).toBeHidden();
  await expect(confirmationText).toBeHidden();
  await form.getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
  const ordinary = await commandFromLastSave(page, 'emergency-lights');
  expect(ordinary).toMatchObject({
    id: 'emergency-lights',
    name: 'Включить красный свет',
    text: 'Аварийное освещение включено.',
  });
  expect(ordinary).not.toHaveProperty('stateChange');
});

test('completed command displays its frozen snapshot while authored fields remain editable', async ({ page }) => {
  await selectCommand(page, 'Двери открыты');

  const form = page.locator('#nodeForm');
  await expect(form.getByLabel('РЕЖИМ КОМАНДЫ')).toHaveValue('state-change');
  await expect(form.getByLabel('ИСХОДНОЕ НАЗВАНИЕ')).toHaveValue('Открыть двери');
  await expect(form.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ')).toHaveValue('Двери разблокированы');
  await expect(form.getByLabel('ТЕКСТ УСПЕШНОГО ВЫПОЛНЕНИЯ')).toHaveValue('Новая редакция результата открытия.');

  const frozen = form.getByRole('status', { name: 'СОХРАНЁННОЕ СОСТОЯНИЕ КОМАНДЫ' });
  await expect(frozen).toContainText('ВЫПОЛНЕНО');
  await expect(frozen).toContainText('Двери открыты');
  await expect(frozen).toContainText('Доступ в сектор разрешён.');

  await form.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ').fill('Новый заголовок для следующего выполнения');
  await form.getByLabel('ТЕКСТ УСПЕШНОГО ВЫПОЛНЕНИЯ').fill('Новый результат для следующего выполнения.');
  await form.getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();

  await expect(page.locator('.tree-row', { hasText: 'Двери открыты' })).toHaveCount(1);
  await expect(frozen).toContainText('Двери открыты');
  await expect(frozen).toContainText('Доступ в сектор разрешён.');
  const save = await lastDesktopCall(page, 'SaveSession');
  expect(save.args[0].terminals[0].commandStates.doors).toEqual({
    completedName: 'Двери открыты',
    resultText: 'Доступ в сектор разрешён.',
  });
});

test('individual and terminal resets require confirmation and update only the intended snapshots', async ({ page }) => {
  await selectCommand(page, 'Двери открыты');
  const form = page.locator('#nodeForm');
  const resetOne = form.getByRole('button', { name: 'СБРОСИТЬ СОСТОЯНИЕ' });
  const initialDurable = await authoringDurableState(page);
  expect(Object.keys(initialDurable.commandStates).sort()).toEqual(['alarm', 'doors']);

  await resetOne.click();
  const resetConfirmation = page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ СБРОСА' });
  await expect(resetConfirmation).toContainText(/сбросить.*двер/i);
  await resetConfirmation.getByRole('button', { name: 'ОТМЕНИТЬ' }).click();
  await expect.poll(() => desktopCallCount(page, 'ResetCommandState')).toBe(0);
  await expect.poll(() => authoringDurableState(page)).toEqual(initialDurable);
  await expect(page.locator('.tree-row', { hasText: 'Двери открыты' })).toHaveCount(1);
  await expect(page.locator('.tree-row', { hasText: 'Тревога включена' })).toHaveCount(1);

  await resetOne.click();
  await expect(resetConfirmation).toContainText(/сбросить.*двер/i);
  await resetConfirmation.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
  await expect.poll(() => desktopCallCount(page, 'ResetCommandState')).toBe(1);
  expect(await lastDesktopCall(page, 'ResetCommandState')).toMatchObject({
    args: [{ terminalId: TERMINAL_ID, commandId: 'doors' }],
  });
  await expect.poll(() => authoringDurableState(page)).toEqual({
    revision: initialDurable.revision + 1,
    commandStates: { alarm: initialDurable.commandStates.alarm },
  });
  await expect(page.locator('.tree-row', { hasText: 'Открыть двери' })).toHaveCount(1);
  await expect(page.locator('.tree-row', { hasText: 'Тревога включена' })).toHaveCount(1);

  const resetAll = page.getByRole('button', { name: 'СБРОСИТЬ ВСЕ СОСТОЯНИЯ' });
  await resetAll.click();
  await expect(resetConfirmation).toContainText(/сбросить.*все.*терминал/i);
  await resetConfirmation.getByRole('button', { name: 'ОТМЕНИТЬ' }).click();
  await expect.poll(() => desktopCallCount(page, 'ResetTerminalCommandStates')).toBe(0);
  await expect.poll(() => authoringDurableState(page)).toEqual({
    revision: initialDurable.revision + 1,
    commandStates: { alarm: initialDurable.commandStates.alarm },
  });
  await expect(page.locator('.tree-row', { hasText: 'Тревога включена' })).toHaveCount(1);

  await resetAll.click();
  await expect(resetConfirmation).toContainText(/сбросить.*все.*терминал/i);
  await resetConfirmation.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
  await expect.poll(() => desktopCallCount(page, 'ResetTerminalCommandStates')).toBe(1);
  expect(await lastDesktopCall(page, 'ResetTerminalCommandStates')).toMatchObject({
    args: [{ terminalId: TERMINAL_ID }],
  });
  await expect.poll(() => authoringDurableState(page)).toEqual({
    revision: initialDurable.revision + 2,
    commandStates: {},
  });
  await expect(page.locator('.tree-row', { hasText: 'Включить тревогу' })).toHaveCount(1);
  await expect(page.locator('.tree-row', { hasText: 'Тревога включена' })).toHaveCount(0);

  await page.reload();
  await openAuthoringFixture(page);
  await expect(page.locator('.tree-row', { hasText: 'Открыть двери' })).toHaveCount(1);
  await expect(page.locator('.tree-row', { hasText: 'Включить тревогу' })).toHaveCount(1);
  await expect.poll(() => authoringDurableState(page)).toEqual({
    revision: initialDurable.revision + 2,
    commandStates: {},
  });
});

test('terminal reset rejects a stale backend success even when its session looks reset', async ({ page }) => {
  const durableBefore = await authoringDurableState(page);
  const snapshotResponse = await page.request.get(`${FIXTURE_URL}/session`);
  expect(snapshotResponse.ok()).toBe(true);
  const stale = await snapshotResponse.json();
  stale.revision = 0;
  stale.session.terminals.find(terminal => terminal.id === TERMINAL_ID).commandStates = {};

  await page.route(`**${FIXTURE_URL}/reset-terminal`, route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(stale),
  }));
  await page.getByRole('button', { name: 'СБРОСИТЬ ВСЕ СОСТОЯНИЯ' }).click();
  await page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ СБРОСА' })
    .getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();

  await expect(page.locator('#saveStatus')).toHaveClass(/err/);
  await expect(page.locator('#saveStatus')).toContainText(/не подтвердил канонический сброс/i);
  await expect(page.locator('.tree-row', { hasText: 'Двери открыты' })).toHaveCount(1);
  await expect(page.locator('.tree-row', { hasText: 'Тревога включена' })).toHaveCount(1);
  await expect.poll(() => authoringDurableState(page)).toEqual(durableBefore);
});

test('terminal reset rejects a newer backend result that still contains completed snapshots', async ({ page }) => {
  const durableBefore = await authoringDurableState(page);
  const snapshotResponse = await page.request.get(`${FIXTURE_URL}/session`);
  expect(snapshotResponse.ok()).toBe(true);
  const nonCanonical = await snapshotResponse.json();
  nonCanonical.revision = durableBefore.revision + 1;

  await page.route(`**${FIXTURE_URL}/reset-terminal`, route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(nonCanonical),
  }));
  await page.getByRole('button', { name: 'СБРОСИТЬ ВСЕ СОСТОЯНИЯ' }).click();
  await page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ СБРОСА' })
    .getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();

  await expect(page.locator('#saveStatus')).toHaveClass(/err/);
  await expect(page.locator('#saveStatus')).toContainText(/не подтвердил канонический сброс/i);
  await expect(page.locator('.tree-row', { hasText: 'Двери открыты' })).toHaveCount(1);
  await expect(page.locator('.tree-row', { hasText: 'Тревога включена' })).toHaveCount(1);
  await expect.poll(() => authoringDurableState(page)).toEqual(durableBefore);
});

test('entry blocks add, edit, reorder, delete, and reopen with stable identities', async ({ page }) => {
  await seedBlockAuthoringSession(page);
  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');

  const blockList = page.locator('#entryContentBlocks');
  const block = id => blockList.locator(`.entry-content-block[data-block-id="${id}"]`);
  await expect(blockList.locator('.entry-content-block')).toHaveCount(5);
  await expect(block(BLOCK_IDS.power).locator('[data-entry-block-content]')).toHaveValue('ПИТАНИЕ: ОТКЛЮЧЕНО');

  await block(BLOCK_IDS.power).locator('[data-entry-block-action="move-down"]').click();
  await block(BLOCK_IDS.duplicateTwo).locator('[data-entry-block-content]').fill('ОХЛАЖДЕНИЕ: ГОТОВО');
  await page.locator('#btnAddEntryContentBlock').click();
  const added = blockList.locator('.entry-content-block').last();
  const addedID = await added.getAttribute('data-block-id');
  expect(addedID).toMatch(/\S+/);
  expect(Object.values(BLOCK_IDS)).not.toContain(addedID);
  await added.locator('[data-entry-block-content]').fill('НОВЫЙ ДИАГНОСТИЧЕСКИЙ БЛОК');

  page.once('dialog', dialog => dialog.accept());
  await block(BLOCK_IDS.long).locator('[data-entry-block-action="delete"]').click();
  await page.locator('#nodeForm').getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();

  const saved = await nodeFromLastSave(page, BLOCK_ENTRY_ID);
  expect(saved.description ?? '').toBe('');
  expect(saved.blocks).toEqual([
    { id: BLOCK_IDS.duplicateOne, initialText: 'СТАТУС: НЕИЗВЕСТЕН' },
    { id: BLOCK_IDS.power, initialText: 'ПИТАНИЕ: ОТКЛЮЧЕНО' },
    { id: BLOCK_IDS.duplicateTwo, initialText: 'ОХЛАЖДЕНИЕ: ГОТОВО' },
    { id: BLOCK_IDS.empty, initialText: ' \n\t ' },
    { id: addedID, initialText: 'НОВЫЙ ДИАГНОСТИЧЕСКИЙ БЛОК' },
  ]);
  expect((await commandFromLastSave(page, BLOCK_OWNER_COMMAND_ID)).stateChange.entryContentChange.blockId)
    .toBe(BLOCK_IDS.power);

  await page.reload();
  await openAuthoringFixture(page);
  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  await expect(blockList.locator('.entry-content-block')).toHaveCount(5);
  await expect.poll(() => blockList.locator('.entry-content-block').evaluateAll(rows =>
    rows.map(row => row.getAttribute('data-block-id')))).toEqual([
      BLOCK_IDS.duplicateOne,
      BLOCK_IDS.power,
      BLOCK_IDS.duplicateTwo,
      BLOCK_IDS.empty,
      addedID,
    ]);
});

test('legacy description converts only when an accepted block edit is applied', async ({ page }) => {
  await seedBlockAuthoringSession(page);
  await selectEntry(page, 'СТАРАЯ ЗАПИСЬ');

  const blocks = page.locator('#entryContentBlocks .entry-content-block');
  await expect(blocks).toHaveCount(1);
  await expect(blocks.locator('[data-entry-block-content]')).toHaveValue('НАСЛЕДУЕМОЕ ОПИСАНИЕ БЕЗ БЛОКОВ');
  await expect(blocks).not.toHaveAttribute('data-block-id', /\S/);

  await page.reload();
  await openAuthoringFixture(page);
  const beforeConversion = await page.request.get(`${FIXTURE_URL}/session`).then(response => response.json());
  const legacyBefore = findFixtureNode(
    beforeConversion.session.terminals.find(terminal => terminal.id === TERMINAL_ID).root,
    'entry-legacy-record',
  );
  expect(legacyBefore.description).toBe('НАСЛЕДУЕМОЕ ОПИСАНИЕ БЕЗ БЛОКОВ');
  expect(legacyBefore.blocks).toBeUndefined();

  await selectEntry(page, 'СТАРАЯ ЗАПИСЬ');
  await page.locator('[data-entry-block-content]').fill('ПРЕОБРАЗОВАННОЕ ОПИСАНИЕ');
  await page.locator('#nodeForm').getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
  const converted = await nodeFromLastSave(page, 'entry-legacy-record');
  expect(converted.description ?? '').toBe('');
  expect(converted.blocks).toHaveLength(1);
  expect(converted.blocks[0]).toEqual({
    id: expect.stringMatching(/\S+/),
    initialText: 'ПРЕОБРАЗОВАННОЕ ОПИСАНИЕ',
  });

  await page.reload();
  await openAuthoringFixture(page);
  await selectEntry(page, 'СТАРАЯ ЗАПИСЬ');
  await expect(page.locator('#entryContentBlocks .entry-content-block')).toHaveAttribute(
    'data-block-id', converted.blocks[0].id,
  );
});

test('command targets expose terminal-local entry, position, normalized preview, and reverse owner labels', async ({ page }) => {
  await seedBlockAuthoringSession(page);
  await selectCommand(page, 'Проверить резервный контур');

  const target = page.locator('#fldEntryBlockTarget');
  const options = await target.locator('option').allTextContents();
  expect(options).toContain(`СОСТОЯНИЕ РЕАКТОРА · БЛОК 1 · ${normalizedPreview('ПИТАНИЕ: ОТКЛЮЧЕНО')}`);
  expect(options).toContain('СОСТОЯНИЕ РЕАКТОРА · БЛОК 2 · СТАТУС: НЕИЗВЕСТЕН');
  expect(options).toContain('СОСТОЯНИЕ РЕАКТОРА · БЛОК 3 · СТАТУС: НЕИЗВЕСТЕН');
  expect(options).toContain('СОСТОЯНИЕ РЕАКТОРА · БЛОК 4 · ПУСТО');
  expect(options).toContain(`СОСТОЯНИЕ РЕАКТОРА · БЛОК 5 · ${normalizedPreview(LONG_BLOCK_TEXT)}`);
  expect(options.join('\n')).not.toContain('ЧУЖАЯ ЗАПИСЬ');
  expect(options.join('\n')).not.toContain('ЧУЖОЙ БЛОК');
  expect(Array.from(normalizedPreview(LONG_BLOCK_TEXT).replace(/…$/u, ''))).toHaveLength(48);

  await target.selectOption(BLOCK_IDS.empty);
  await page.locator('#fldCompletedEntryBlockContent').fill('');
  await page.locator('#nodeForm').getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
  const command = await commandFromLastSave(page, 'command-reactor-secondary');
  expect(command.stateChange.entryContentChange).toEqual({
    blockId: BLOCK_IDS.empty,
    completedText: '',
  });

  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  const owner = page.locator(
    `.entry-content-block[data-block-id="${BLOCK_IDS.empty}"] [data-entry-block-owner][data-command-id="command-reactor-secondary"]`,
  );
  await expect(owner).toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Проверить резервный контур');

  await selectCommand(page, 'Проверить резервный контур');
  await page.locator('#fldName').fill('Проверить вспомогательный контур');
  await page.locator('#nodeForm').getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  await expect(owner).toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Проверить вспомогательный контур');
});

test('duplicate targets and deletion of targeted blocks or containing nodes identify the owning command', async ({ page }) => {
  await seedBlockAuthoringSession(page);
  await selectCommand(page, 'Проверить резервный контур');

  const savesBeforeConflict = await desktopCallCount(page, 'SaveSession');
  await page.locator('#fldEntryBlockTarget').selectOption(BLOCK_IDS.power);
  await page.locator('#fldCompletedEntryBlockContent').fill('РЕЗЕРВ: ВКЛЮЧЕН');
  await page.locator('#nodeForm').getByRole('button', { name: 'ПРИМЕНИТЬ' }).click();
  await expect(page.locator('#nodeValidationError')).toContainText('Включить питание реактора');
  await expect.poll(() => desktopCallCount(page, 'SaveSession')).toBe(savesBeforeConflict);

  page.on('dialog', dialog => dialog.accept());
  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  const powerBlock = page.locator(`.entry-content-block[data-block-id="${BLOCK_IDS.power}"]`);
  await powerBlock.locator('[data-entry-block-action="delete"]').click();
  await expect(page.locator('#nodeValidationError')).toContainText('Включить питание реактора');
  await expect(powerBlock).toHaveCount(1);

  await page.locator('#btnDeleteNode').click();
  await expect(page.locator('#nodeValidationError')).toContainText('Включить питание реактора');
  await expect(page.locator('.tree-row', { hasText: 'СОСТОЯНИЕ РЕАКТОРА' })).toHaveCount(1);

  await page.locator('.tree-row', { hasText: 'РЕАКТОРНЫЙ КОНТУР' }).click();
  await page.locator('#btnDeleteNode').click();
  await expect(page.locator('#nodeValidationError')).toContainText('Включить питание реактора');
  await expect(page.locator('.tree-row', { hasText: 'РЕАКТОРНЫЙ КОНТУР' })).toHaveCount(1);
});

test('completed command keeps its exact target label and completed block text locked until reset', async ({ page }) => {
  await seedBlockAuthoringSession(page, { completedOwner: true });
  await selectCommand(page, 'Питание реактора включено');

  const target = page.locator('#fldEntryBlockTarget');
  const completedText = page.locator('#fldCompletedEntryBlockContent');
  await expect(target).toBeDisabled();
  await expect(target).toHaveValue(BLOCK_IDS.power);
  await expect(target.locator('option:checked')).toHaveText(
    'СОСТОЯНИЕ РЕАКТОРА · БЛОК 1 · ПИТАНИЕ: ОТКЛЮЧЕНО',
  );
  await expect(completedText).toBeDisabled();
  await expect(completedText).toHaveValue('ПИТАНИЕ: ВКЛЮЧЕНО');
  await expect(page.getByRole('status', { name: 'СОХРАНЁННОЕ СОСТОЯНИЕ КОМАНДЫ' }))
    .toContainText('ПИТАНИЕ: ВКЛЮЧЕНО');

  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  const owner = page.locator(
    `.entry-content-block[data-block-id="${BLOCK_IDS.power}"] [data-entry-block-owner][data-command-id="${BLOCK_OWNER_COMMAND_ID}"]`,
  );
  await expect(owner).toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Питание реактора включено');
});

test('individual block-owner reset cancels and fails atomically before restoring only its command and owner label', async ({ page }) => {
  await seedBlockAuthoringSession(page, { completedOwner: true, completedSecondary: true });
  const initial = await authoringDurableState(page);
  expect(Object.keys(initial.commandStates).sort()).toEqual([
    'alarm',
    BLOCK_OWNER_COMMAND_ID,
    'command-reactor-secondary',
    'doors',
  ]);

  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  await expect(entryBlockOwner(page, BLOCK_IDS.power, BLOCK_OWNER_COMMAND_ID))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Питание реактора включено');
  await expect(entryBlockOwner(page, BLOCK_IDS.empty, 'command-reactor-secondary'))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Резервный контур проверен');

  await selectCommand(page, 'Питание реактора включено');
  const reset = page.getByRole('button', { name: 'СБРОСИТЬ СОСТОЯНИЕ' });
  const confirmation = page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ СБРОСА' });
  await reset.click();
  await expect(confirmation).toContainText('Питание реактора включено');
  await confirmation.getByRole('button', { name: 'ОТМЕНИТЬ' }).click();
  await expect.poll(() => desktopCallCount(page, 'ResetCommandState')).toBe(0);
  await expect.poll(() => authoringDurableState(page)).toEqual(initial);

  await page.route(`**${FIXTURE_URL}/reset-command`, route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      error: 'fixture entry block reset failed',
      revision: initial.revision,
    }),
  }));
  await reset.click();
  await confirmation.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
  await expect(page.locator('#saveStatus')).toContainText('fixture entry block reset failed');
  await expect.poll(() => authoringDurableState(page)).toEqual(initial);
  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  await expect(entryBlockOwner(page, BLOCK_IDS.power, BLOCK_OWNER_COMMAND_ID))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Питание реактора включено');

  await page.unroute(`**${FIXTURE_URL}/reset-command`);
  await selectCommand(page, 'Питание реактора включено');
  await page.getByRole('button', { name: 'СБРОСИТЬ СОСТОЯНИЕ' }).click();
  await confirmation.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
  await expect.poll(() => authoringDurableState(page)).toEqual({
    revision: initial.revision + 1,
    commandStates: {
      alarm: initial.commandStates.alarm,
      'command-reactor-secondary': initial.commandStates['command-reactor-secondary'],
      doors: initial.commandStates.doors,
    },
  });

  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  await expect(entryBlockOwner(page, BLOCK_IDS.power, BLOCK_OWNER_COMMAND_ID))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Включить питание реактора');
  await expect(entryBlockOwner(page, BLOCK_IDS.empty, 'command-reactor-secondary'))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Резервный контур проверен');
  await expect(page.locator(
    `.entry-content-block[data-block-id="${BLOCK_IDS.power}"] [data-entry-block-content]`,
  )).toHaveValue('ПИТАНИЕ: ОТКЛЮЧЕНО');

  await selectCommand(page, 'Включить питание реактора');
  await expect(page.locator('#fldEntryBlockTarget')).toBeEnabled();
  await expect(page.locator('#fldEntryBlockTarget')).toHaveValue(BLOCK_IDS.power);
  await expect(page.locator('#fldCompletedEntryBlockContent')).toBeEnabled();
  await expect(page.locator('#fldCompletedEntryBlockContent')).toHaveValue('ПИТАНИЕ: ВКЛЮЧЕНО');
});

test('terminal-wide block reset cancels and fails atomically before restoring every owner label and initial block', async ({ page }) => {
  await seedBlockAuthoringSession(page, { completedOwner: true, completedSecondary: true });
  const initial = await authoringDurableState(page);
  const resetAll = page.getByRole('button', { name: 'СБРОСИТЬ ВСЕ СОСТОЯНИЯ' });
  const confirmation = page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ СБРОСА' });

  await resetAll.click();
  await expect(confirmation).toContainText(/сбросить.*все.*терминал/i);
  await confirmation.getByRole('button', { name: 'ОТМЕНИТЬ' }).click();
  await expect.poll(() => desktopCallCount(page, 'ResetTerminalCommandStates')).toBe(0);
  await expect.poll(() => authoringDurableState(page)).toEqual(initial);

  await page.route(`**${FIXTURE_URL}/reset-terminal`, route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      error: 'fixture terminal block reset failed',
      revision: initial.revision,
    }),
  }));
  await resetAll.click();
  await confirmation.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
  await expect(page.locator('#saveStatus')).toContainText('fixture terminal block reset failed');
  await expect.poll(() => authoringDurableState(page)).toEqual(initial);
  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  await expect(entryBlockOwner(page, BLOCK_IDS.power, BLOCK_OWNER_COMMAND_ID))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Питание реактора включено');
  await expect(entryBlockOwner(page, BLOCK_IDS.empty, 'command-reactor-secondary'))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Резервный контур проверен');

  await page.unroute(`**${FIXTURE_URL}/reset-terminal`);
  await resetAll.click();
  await confirmation.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
  await expect.poll(() => authoringDurableState(page)).toEqual({
    revision: initial.revision + 1,
    commandStates: {},
  });

  await selectEntry(page, 'СОСТОЯНИЕ РЕАКТОРА');
  await expect(entryBlockOwner(page, BLOCK_IDS.power, BLOCK_OWNER_COMMAND_ID))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Включить питание реактора');
  await expect(entryBlockOwner(page, BLOCK_IDS.empty, 'command-reactor-secondary'))
    .toHaveText('ИЗМЕНЯЕТСЯ КОМАНДОЙ: Проверить резервный контур');
  await expect(page.locator(
    `.entry-content-block[data-block-id="${BLOCK_IDS.power}"] [data-entry-block-content]`,
  )).toHaveValue('ПИТАНИЕ: ОТКЛЮЧЕНО');
  await expect(page.locator(
    `.entry-content-block[data-block-id="${BLOCK_IDS.empty}"] [data-entry-block-content]`,
  )).toHaveValue(' \n\t ');
});
