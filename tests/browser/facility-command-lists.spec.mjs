import { expect, test } from '@playwright/test';

import { expectSaved, switchTab } from './helpers/authoring-bar.mjs';
import {
  FIXTURE, demoSession, fixtureSession, openOverseer, resetFixture, seedSession,
} from './helpers/overseer-forms.mjs';

// US2: one place per command lists what it changes and what players see
// (FR-008–FR-015, SC-004, SC-005).

const COMMAND = 'Открыть выход и подать сигнал';
const PLAYER_FIXTURE = '/__fixture/facility-player-state';
const TOKEN_KEY = 'fallout-terminal.player-token';

function walk(node, visit, path = []) {
  visit(node, path);
  for (const child of node.children || []) walk(child, visit, [...path, node]);
}

function findNode(session, nodeID) {
  for (const terminal of session.terminals) {
    let found = null;
    walk(terminal.root, (node, path) => {
      if (node.id === nodeID) found = { terminal, node, path };
    });
    if (found) return found;
  }
  return null;
}

function commandByName(session, name) {
  for (const terminal of session.terminals) {
    let found = null;
    walk(terminal.root, node => {
      if (node.type === 'command' && node.name === name) found = node;
    });
    if (found) return found;
  }
  return null;
}

function setDeviceState(session, deviceID, stateID) {
  session.facility.devices.find(device => device.id === deviceID).currentStateId = stateID;
}

function setConditionActive(session, conditionID, active) {
  session.facility.conditions.find(condition => condition.id === conditionID).currentActive = active;
}

// expectedEffects derives the «ПРИ ОДОБРЕНИИ» items of command from the
// authored session, as FR-008 defines them.
function expectedEffects(session, terminal, command) {
  const devices = new Map(session.facility.devices.map(device => [device.id, device]));
  const stateName = (device, id) => device.states.find(value => value.id === id)?.name || id;
  const transition = request => {
    const device = devices.get(request.deviceId);
    const value = device.transitions.find(candidate => candidate.id === request.transitionId);
    return { kind: 'transition', text: `${device.name}: ${stateName(device, value.sourceStateId)} → ${stateName(device, value.destinationStateId)}` };
  };
  if (command.terminalTransition) {
    const target = session.terminals.find(value => value.id === command.terminalTransition.targetTerminalId);
    return [{ kind: 'terminal-transition', text: `ПЕРЕХОД: ${target.name}` }];
  }
  const change = command.stateChange;
  const items = (change.facilityAction?.transitions?.transitions || []).map(transition);
  const programID = change.facilityAction?.recoveryProgramId;
  if (programID) {
    const program = session.facility.recoveryPrograms.find(value => value.id === programID);
    items.push({ kind: 'recovery', text: `ПРОГРАММА ВОССТАНОВЛЕНИЯ: ${program.name}` });
    items.push(...(program.transitions || []).map(transition));
  }
  if (change.completedName) items.push({ kind: 'completed-name', text: `НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ: ${change.completedName}` });
  const blockChange = change.entryContentChange;
  if (blockChange?.blockId) {
    let entry = null;
    walk(terminal.root, node => {
      if (node.blocks?.some(block => block.id === blockChange.blockId)) entry = node;
    });
    const index = entry.blocks.findIndex(block => block.id === blockChange.blockId);
    items.push({ kind: 'entry-block', text: `${entry.name} · блок ${index + 1}: ${blockChange.completedText}` });
  }
  return items;
}

// openCommand selects the command's terminal and expands its folders by
// authored name, so the label players see can differ.
async function openCommand(page, session, commandID) {
  const { terminal, path } = findNode(session, commandID);
  await switchTab(page, 'ТЕРМИНАЛЫ');
  await page.locator(`.term-row[data-terminal-id="${terminal.id}"]`).click();
  for (const folder of path.slice(1)) {
    await page.locator(`.tree-label[data-authored-name="${folder.name}"]`).first().click();
  }
  const { node } = findNode(session, commandID);
  await page.locator(`.tree-label[data-authored-name="${node.name}"]`).first().click();
  await expect(page.locator('#fldName')).toHaveValue(node.name);
}

async function effectItems(page) {
  return page.locator('#commandEffects li.command-effect').evaluateAll(items => items.map(item => ({
    kind: item.dataset.kind, text: item.textContent.trim(),
  })));
}

function ruleTarget(page, target) {
  return page.locator(`#commandRules section.command-rule-target[data-target="${target}"]`);
}

async function ruleItems(page, target) {
  return ruleTarget(page, target).locator('li.command-rule').evaluateAll(items => items.map(item => ({
    kind: item.dataset.kind,
    text: item.querySelector('span').textContent.trim(),
    playersSee: Boolean(item.querySelector('.rule-players-see')),
    overrides: item.querySelector('.rule-overrides')?.textContent.trim() || '',
  })));
}

async function expectOneWinner(page, target) {
  await expect(ruleTarget(page, target).locator('.rule-players-see')).toHaveCount(1);
}

// seedDemoWith seeds the demo with its current live values. The fixture
// starts a new device at its initial state and keeps a known device's
// current state, so it seeds the wanted current values as starting values
// first, then the authored starting values.
async function seedDemoWith(request, mutate = () => {}) {
  await resetFixture(request);
  const session = await demoSession();
  mutate(session);
  const live = structuredClone(session);
  for (const device of live.facility.devices) device.initialStateId = device.currentStateId;
  for (const condition of live.facility.conditions) condition.initialActive = condition.currentActive;
  await seedSession(request, live);
  await seedSession(request, session);
  return session;
}

test('the form lists every change on approval and every rule players see (FR-008–FR-010)', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page, '');
  const command = commandByName(session, COMMAND);
  await openCommand(page, session, command.id);

  await expect(page.locator('#commandEffectsHeading')).toHaveText('ПРИ ОДОБРЕНИИ');
  expect(await effectItems(page)).toEqual([
    { kind: 'transition', text: 'Гермодверь Убежища 76: Заперта → Открыта' },
    { kind: 'transition', text: 'Сигнал эвакуации: Тишина → Эвакуация' },
    { kind: 'completed-name', text: 'НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ: Выход открыт' },
    { kind: 'entry-block', text: `СВОДКА ЭВАКУАЦИИ · блок 2: ${command.stateChange.entryContentChange.completedText}` },
  ]);

  await expect(page.locator('#commandRulesHeading')).toHaveText('ЧТО ВИДЯТ ИГРОКИ');
  await expect(page.locator('#commandRules')).toHaveAttribute('aria-busy', 'false');
  expect(await ruleItems(page, 'name')).toEqual([
    { kind: 'authored-name', text: `ИСХОДНОЕ НАЗВАНИЕ · ВСЕГДА · ${COMMAND}`, playersSee: true, overrides: '' },
    { kind: 'name-after-execution', text: 'НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ · ПОСЛЕ ВЫПОЛНЕНИЯ · Выход открыт', playersSee: false, overrides: '' },
    { kind: 'state-name', text: 'НАЗВАНИЕ ПО СОСТОЯНИЮ · КОГДА Гермодверь Убежища 76 = Открыта · Выход уже открыт', playersSee: false, overrides: '' },
  ]);
  const availability = await ruleItems(page, 'availability');
  expect(availability[0]).toEqual({
    kind: 'state-availability',
    text: 'ДОСТУПНОСТЬ ПО СОСТОЯНИЮ · КОГДА Резервная сеть Убежища 76 = В работе · РАЗРЕШИТЬ',
    playersSee: true,
    overrides: '',
  });
  expect(availability.slice(1).map(rule => rule.kind)).toEqual(['diagnostic-capability-block', 'diagnostic-capability-block']);
  const blockID = command.stateChange.entryContentChange.blockId;
  expect((await ruleItems(page, `block:${blockID}`)).map(rule => [rule.kind, rule.playersSee])).toEqual([
    ['initial-text', true], ['text-after-execution', false],
  ]);
  for (const target of ['name', 'availability', `block:${blockID}`]) await expectOneWinner(page, target);

  // The tree shows what players see; the authored name stays in the form.
  const label = page.locator(`.tree-label[data-authored-name="${COMMAND}"]`);
  await expect(label).toHaveText(COMMAND);
  await expect(label).not.toHaveAttribute('title', /.+/);
});

test('a holding higher rule is marked as the winner and names what it overrides (FR-010, FR-011)', async ({ page, request }) => {
  const blockText = 'Гермодверь открыта: жители выходят наружу.';
  const session = await seedDemoWith(request, value => {
    setDeviceState(value, 'vault-door-main', 'open');
    setConditionActive(value, 'grid-unpowered', true);
    const { node } = findNode(value, 'n_entry_vault_shutdown');
    node.blocks.find(block => block.id === 'b_vault_shutdown_body').facilityTextVariants = [
      { when: { deviceId: 'vault-door-main', stateId: 'open' }, text: blockText },
    ];
  });
  await openOverseer(page, '');
  const command = commandByName(session, COMMAND);
  await openCommand(page, session, command.id);

  const name = await ruleItems(page, 'name');
  expect(name.map(rule => [rule.kind, rule.playersSee])).toEqual([
    ['authored-name', false], ['name-after-execution', false], ['state-name', true],
  ]);
  expect(name[2].overrides).toBe('ПЕРЕКРЫВАЕТ: ИСХОДНОЕ НАЗВАНИЕ, НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ');

  // A state text over a rewritten block outranks the text after execution.
  const block = await ruleItems(page, 'block:b_vault_shutdown_body');
  expect(block.map(rule => [rule.kind, rule.playersSee])).toEqual([
    ['initial-text', false], ['text-after-execution', false], ['state-text', true],
  ]);
  expect(block[2]).toMatchObject({
    text: `ТЕКСТ ПО СОСТОЯНИЮ · КОГДА Гермодверь Убежища 76 = Открыта · ${blockText}`,
    overrides: 'ПЕРЕКРЫВАЕТ: ИСХОДНЫЙ ТЕКСТ, ТЕКСТ ПОСЛЕ ВЫПОЛНЕНИЯ',
  });

  // An active power fault outranks the state-based availability.
  const availability = await ruleItems(page, 'availability');
  const blocking = availability.find(rule => rule.text.includes('Резервная сеть обесточена'));
  expect(blocking).toMatchObject({
    kind: 'diagnostic-capability-block',
    text: 'БЛОКИРОВКА НЕИСПРАВНОСТЬЮ · КОГДА АКТИВНА «Резервная сеть обесточена» · ЗАПРЕТИТЬ',
    overrides: 'ПЕРЕКРЫВАЕТ: ДОСТУПНОСТЬ ПО СОСТОЯНИЮ',
  });
  for (const target of ['name', 'availability', 'block:b_vault_shutdown_body']) await expectOneWinner(page, target);

  const label = page.locator(`.tree-label[data-authored-name="${COMMAND}"]`);
  await expect(label).toHaveText('Выход уже открыт');
  await expect(label).toHaveAttribute('title', `В РЕДАКТОРЕ: ${COMMAND}`);
});

test('a fault that forces a command visible outranks its state visibility (FR-009, FR-010)', async ({ page, request }) => {
  const session = await seedDemoWith(request, value => {
    findNode(value, 'n_cmd_open_evacuation').node.visibleWhen = { deviceId: 'vault-door-main', stateId: 'open' };
    value.facility.conditions.find(condition => condition.id === 'door-offline').effects
      .push({ diagnosticPath: { terminalId: 't_demo1', nodeId: 'n_cmd_open_evacuation' } });
    setConditionActive(value, 'door-offline', true);
  });
  const fault = session.facility.conditions.find(condition => condition.id === 'door-offline');
  await openOverseer(page, '');
  await openCommand(page, session, 'n_cmd_open_evacuation');

  // The door is sealed, so the state rule alone would hide the command.
  expect(await ruleItems(page, 'visibility')).toEqual([
    {
      kind: 'state-visibility',
      text: 'ВИДИМОСТЬ ПО СОСТОЯНИЮ · КОГДА Гермодверь Убежища 76 = Открыта · ПОКАЗАТЬ',
      playersSee: false,
      overrides: '',
    },
    {
      kind: 'diagnostic-forced-visibility',
      text: `ВИДИМОСТЬ ПРИ НЕИСПРАВНОСТИ · КОГДА АКТИВНА «${fault.name}» · ПОКАЗАТЬ`,
      playersSee: true,
      overrides: 'ПЕРЕКРЫВАЕТ: ВИДИМОСТЬ ПО СОСТОЯНИЮ',
    },
  ]);
  await expectOneWinner(page, 'visibility');
  const row = page.locator('.tree-row', { has: page.locator(`.tree-label[data-authored-name="${COMMAND}"]`) });
  await expect(row.locator('.tree-badge-hidden')).toHaveCount(0);
});

test('every state-changing demo command lists exactly its authored effects (SC-004)', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page, '');
  const commands = [];
  for (const terminal of session.terminals) {
    walk(terminal.root, node => {
      if (node.type === 'command' && (node.stateChange || node.terminalTransition)) commands.push({ terminal, node });
    });
  }
  // The demo covers a legacy-only, a device-only, a program, and a transition command.
  const kinds = commands.map(({ node }) => [
    node.terminalTransition ? 'terminal-transition' : '',
    node.stateChange?.facilityAction?.recoveryProgramId ? 'program' : '',
    node.stateChange?.facilityAction?.transitions ? 'device' : '',
    node.stateChange && !node.stateChange.facilityAction ? 'legacy' : '',
  ].filter(Boolean));
  for (const kind of ['terminal-transition', 'program', 'device', 'legacy']) {
    expect(kinds.flat(), `demo has a ${kind} command`).toContain(kind);
  }

  for (const { terminal, node } of commands) {
    await openCommand(page, session, node.id);
    expect(await effectItems(page), node.name).toEqual(expectedEffects(session, terminal, node));
  }
});

test('a legacy-only executed command shows its name after execution winning (FR-012)', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page, '');
  const command = findNode(session, 'n_cmd_1').node;
  await openCommand(page, session, command.id);

  expect((await effectItems(page)).map(effect => effect.kind)).toEqual(['completed-name', 'entry-block']);
  const name = await ruleItems(page, 'name');
  expect(name.map(rule => [rule.kind, rule.playersSee])).toEqual([
    ['authored-name', false], ['name-after-execution', true],
  ]);
  expect(name[1].overrides).toBe('ПЕРЕКРЫВАЕТ: ИСХОДНОЕ НАЗВАНИЕ');
  await expect(page.locator(`.tree-label[data-authored-name="${command.name}"]`))
    .toHaveText(command.stateChange.completedName);
});

test('a diagnostic substitution over a rewritten block wins while the fault is active', async ({ page, request }) => {
  const completedText = 'СЕТЬ-76: КАНАЛ ВОССТАНОВЛЕН.';
  const session = await seedDemoWith(request, value => {
    const { node } = findNode(value, 'n2_cmd_network_recovery');
    node.stateChange.entryContentChange = { blockId: 'b2_network_status', completedText };
  });
  await openOverseer(page, '');
  await openCommand(page, session, 'n2_cmd_network_recovery');

  const block = await ruleItems(page, 'block:b2_network_status');
  expect(block.map(rule => [rule.kind, rule.playersSee])).toEqual([
    ['initial-text', false], ['text-after-execution', false], ['state-text', false], ['diagnostic-substitution', true],
  ]);
  expect(block[3].text).toContain('КОГДА АКТИВНА «Закрытый канал изолирован»');
  expect(block[3].overrides).toBe('ПЕРЕКРЫВАЕТ: ИСХОДНЫЙ ТЕКСТ, ТЕКСТ ПОСЛЕ ВЫПОЛНЕНИЯ, ТЕКСТ ПО СОСТОЯНИЮ');
});

test('a terminal-transition command lists only its transition', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page, '');
  const { terminal, node } = findNode(session, 'n_cmd_state_change_1');
  await openCommand(page, session, node.id);
  expect(await effectItems(page)).toEqual(expectedEffects(session, terminal, node));
  expect(await effectItems(page)).toHaveLength(1);
});

test('a rule edited from the command form is the same rule on ОБЪЕКТЫ (FR-013)', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page, '');
  const command = commandByName(session, COMMAND);
  await openCommand(page, session, command.id);

  await ruleTarget(page, 'name').locator('li.command-rule[data-kind="state-name"]')
    .getByRole('button', { name: 'ИЗМЕНИТЬ' }).click();
  const dialog = page.locator('#facilityBindingDialog');
  await expect(dialog).toBeVisible();
  await expect(page.locator('#facilityBindingText')).toHaveValue('Выход уже открыт');
  await page.locator('#facilityBindingText').fill('Выход распахнут');
  await dialog.getByRole('button', { name: 'ГОТОВО' }).click();
  await expect(dialog).toBeHidden();
  await expectSaved(page);

  await expect.poll(async () => findNode(await fixtureSession(request), command.id).node.facilityNameVariants[0].text)
    .toBe('Выход распахнут');
  await expect(ruleTarget(page, 'name').locator('li.command-rule[data-kind="state-name"]'))
    .toContainText('Выход распахнут');
  await switchTab(page, 'ОБЪЕКТЫ');
  await expect(page.locator('#facilityBindingList')).toContainText('Выход распахнут');
});

test('scene reset counts what changes, and a failure resets nothing (FR-014, SC-005)', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page, '');
  const devices = session.facility.devices.filter(device => device.currentStateId !== device.initialStateId).length;
  const faults = session.facility.conditions
    .filter(condition => Boolean(condition.currentActive) !== Boolean(condition.initialActive)).length;
  const executed = session.terminals
    .map(terminal => ({ name: terminal.name, count: Object.keys(terminal.commandStates || {}).length }))
    .filter(terminal => terminal.count);
  const commandCount = executed.reduce((sum, terminal) => sum + terminal.count, 0);

  await page.locator('#btnResetScene').click();
  const dialog = page.locator('#sceneResetDialog');
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#sceneResetCounts li')).toHaveText([
    `УСТРОЙСТВА: ${devices}`,
    `НЕИСПРАВНОСТИ: ${faults}`,
    `ВЫПОЛНЕННЫЕ КОМАНДЫ: ${commandCount} (${executed.map(value => `${value.name}: ${value.count}`).join(', ')})`,
  ]);
  await expect(dialog).toContainText('ИГРОКИ ОСТАНУТСЯ НА ТЕКУЩЕМ ТЕРМИНАЛЕ; МАРШРУТ И ПРОГРЕСС ВЗЛОМА СОХРАНЯТСЯ.');
  await expect(page.locator('#sceneResetPending')).toBeHidden();

  const failed = await request.post(`${FIXTURE}/next-operation`, { data: { failure: 'persistence-failed' } });
  expect(failed.status()).toBe(204);
  await page.locator('#btnConfirmSceneReset').click();
  await expect(page.locator('#sceneResetStatus')).toHaveText('СЦЕНА НЕ СБРОШЕНА: НЕ УДАЛОСЬ СОХРАНИТЬ СОСТОЯНИЕ ОБЪЕКТА');
  const unchanged = await fixtureSession(request);
  expect(unchanged.facility.devices.map(device => device.currentStateId))
    .toEqual(session.facility.devices.map(device => device.currentStateId));
  expect(unchanged.terminals.map(terminal => terminal.commandStates || {}))
    .toEqual(session.terminals.map(terminal => terminal.commandStates || {}));

  await page.locator('#btnConfirmSceneReset').click();
  await expect(dialog).toBeHidden();
  const reset = await fixtureSession(request);
  for (const device of reset.facility.devices) expect(device.currentStateId, device.id).toBe(device.initialStateId);
  for (const condition of reset.facility.conditions) {
    expect(Boolean(condition.currentActive), condition.id).toBe(Boolean(condition.initialActive));
  }
  for (const terminal of reset.terminals) expect(terminal.commandStates || {}, terminal.id).toEqual({});
  // Authored configuration is untouched by a scene reset.
  expect(reset.terminals.map(terminal => terminal.root)).toEqual(session.terminals.map(terminal => terminal.root));

  await expect(page.locator('#btnResetScene')).toHaveAttribute('aria-disabled', 'true');
  await expect(page.locator('#btnResetSceneReason')).toHaveText('СЦЕНА УЖЕ В НАЧАЛЬНОМ СОСТОЯНИИ');
});

test('the scene reset names what happens to a pending player request', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page, '');
  await page.locator('#btnResetScene').click();
  await expect(page.locator('#sceneResetDialog')).toBeVisible();
  await expect(page.locator('#sceneResetPending')).toBeHidden();

  // A request arrives while the dialog is open: an executed command is reset,
  // so its request is rejected; any other request stays pending.
  const executed = findNode(session, 'n_cmd_1').node;
  const ordinary = findNode(session, 'n_cmd_state_change_1').node;
  const emitPending = (revision, command) => page.evaluate(({ revision, command }) => {
    window.__desktopFixture.emit('coordination-state', {
      revision, roster: [], sessions: [],
      pendingCommandExecution: {
        requestId: `scene-reset-${revision}`, broadcastId: 'fixture-broadcast', terminalId: 't_demo1',
        commandId: command.id, commandName: command.name, mode: 'state-change', confirmationText: 'ПОДТВЕРДИТЬ?',
      },
    });
  }, { revision, command: { id: command.id, name: command.name } });

  await emitPending(1000, executed);
  await expect(page.locator('#sceneResetPending')).toHaveText(`ОЖИДАЮЩИЙ ЗАПРОС ИГРОКА: ${executed.name} — БУДЕТ ОТКЛОНЁН`);
  await emitPending(1001, ordinary);
  await expect(page.locator('#sceneResetPending')).toHaveText(`ОЖИДАЮЩИЙ ЗАПРОС ИГРОКА: ${ordinary.name} — ОСТАНЕТСЯ В ОЖИДАНИИ`);
});

async function openParticipant(browser) {
  const context = await browser.newContext();
  await context.addInitScript(tokenKey => {
    localStorage.removeItem(tokenKey);
    HTMLMediaElement.prototype.play = () => Promise.resolve();
  }, TOKEN_KEY);
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  if (await page.locator('#characterSelect').isVisible()) {
    await page.locator('#characterOptions button:not([disabled])').first().click();
  }
  return { context, page };
}

test('the tree and the marked winner match the player view before and after approval (SC-004)', async ({ browser, request }) => {
  const reset = await request.post(`${PLAYER_FIXTURE}/reset`, { data: { scenario: 'ready' } });
  expect(reset.status()).toBe(204);
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  const player = await openParticipant(browser);
  try {
    await overseer.goto(`${PLAYER_FIXTURE}/overseer`);
    await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
    await expect(overseer.locator('#mainLayout')).toBeVisible();
    await expect(player.page.locator('#roleBadge')).toContainText('АКТИВЕН');

    const label = overseer.locator('.tree-label[data-authored-name="OPEN SECURITY DOOR"]');
    const row = overseer.locator('.tree-row', { has: label });
    const winner = () => ruleTarget(overseer, 'name').locator('li.command-rule:has(.rule-players-see)');
    await expect(label).toHaveText('OPEN SECURITY DOOR');
    await expect(player.page.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' })).toBeVisible();
    await expect(row.locator('.tree-badge')).toHaveCount(0);
    await label.click();
    await expect(winner()).toHaveAttribute('data-kind', 'authored-name');
    await expect(overseer.locator('#btnResetScene')).toHaveAttribute('aria-disabled', 'true');

    await player.page.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' }).click();
    const dialog = overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();

    // The open door renames the command for players and locks it.
    await expect(player.page.locator('.term-row', { hasText: 'SECURITY DOOR OPEN' })).toBeVisible();
    await expect(label).toHaveText('SECURITY DOOR OPEN');
    await expect(winner()).toHaveAttribute('data-kind', 'state-name');
    await expect(winner().locator('.rule-overrides')).toHaveText('ПЕРЕКРЫВАЕТ: ИСХОДНОЕ НАЗВАНИЕ, НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ');
    await expect(row.locator('.tree-badge-unavailable')).toHaveText('НЕДОСТУПНО');

    // Scene reset returns the scene; the player stays on the same terminal.
    const activeTerminal = async () => (await (await request.get(`${PLAYER_FIXTURE}/coordination`)).json())
      .broadcast?.activeTerminalId;
    const before = await activeTerminal();
    expect(before).toBeTruthy();
    await overseer.locator('#btnResetScene').click();
    await expect(overseer.locator('#sceneResetCounts li')).toHaveText([
      'УСТРОЙСТВА: 2', 'НЕИСПРАВНОСТИ: 1', 'ВЫПОЛНЕННЫЕ КОМАНДЫ: 1 (Security control: 1)',
    ]);
    await overseer.locator('#btnConfirmSceneReset').click();
    await expect(overseer.locator('#sceneResetDialog')).toBeHidden();
    await expect(player.page.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' })).toBeVisible();
    await expect(label).toHaveText('OPEN SECURITY DOOR');
    await expect(row.locator('.tree-badge')).toHaveCount(0);
    expect(await activeTerminal()).toBe(before);
  } finally {
    await player.context.close();
    await overseerContext.close();
  }
});
