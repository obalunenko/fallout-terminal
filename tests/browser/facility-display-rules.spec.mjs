import { expect, test } from '@playwright/test';

import { blockedReason, expectSaved, switchTab } from './helpers/authoring-bar.mjs';
import { chooseWhere, commandActionDialog, fillRule, ruleDialog } from './helpers/facility-editors.mjs';
import { PLAYER_FIXTURE, openPlayerTable, playCard, selectDevice } from './helpers/facility-play.mjs';
import {
  FIXTURE, expectNoOrphanLabels, fixtureSession, openOverseer, seedDemo,
} from './helpers/overseer-forms.mjs';

// US4: a display rule reads «когда — где — что», and a command's device
// action has its own dialog (FR-022 to FR-029, FR-045, SC-005).

const DOOR = 'vault-door-main';
const WARDEN = 't_demo1';
const WARDEN_NAME = 'Терминал смотрителя — Убежище 76';
const SUMMARY_PATH = `${WARDEN_NAME} › ДИРЕКТИВЫ VAULT-TEC › СВОДКА ЭВАКУАЦИИ`;
const EVACUATION_PATH = `${WARDEN_NAME} › ПРОТОКОЛ ВЫХОДА › Открыть выход и подать сигнал`;
const AIR_PATH = `${WARDEN_NAME} › ПРОТОКОЛ ВЫХОДА › Загрузить кассету очистки воздуха`;
const FIXED_TAB = 'ЧТОБЫ СМЕНИТЬ ТИП, СОЗДАЙТЕ НОВОЕ ПРАВИЛО';
const KINDS = ['entry-content', 'menu-name', 'visibility', 'availability'];
const LABELS = { when: 'КОГДА', where: 'ГДЕ', block: 'БЛОК', text: 'ТЕКСТ' };
const SENTENCES = {
  'entry-content': {
    tab: 'ТЕКСТ ЗАПИСИ', clauses: ['when', 'where', 'block', 'text'],
    otherStates: 'В остальных состояниях останется исходный текст блока.',
    offers: element => element.type === 'entry' && element.blocks.length > 0,
  },
  'menu-name': {
    tab: 'НАЗВАНИЕ В МЕНЮ', clauses: ['when', 'where', 'text'],
    otherStates: 'В остальных состояниях останется исходное название.',
    offers: element => !element.isRoot,
  },
  visibility: {
    tab: 'ПОКАЗАТЬ ЭЛЕМЕНТ', clauses: ['when', 'where'],
    otherStates: 'В остальных состояниях элемент скрыт.',
    offers: element => !element.isRoot,
  },
  availability: {
    tab: 'РАЗРЕШИТЬ КОМАНДУ', clauses: ['when', 'where'],
    otherStates: 'В остальных состояниях команда видна, но её нельзя запросить.',
    offers: element => element.type === 'command',
  },
};

function walk(node, visit, path = []) {
  visit(node, path);
  for (const child of node.children || []) walk(child, visit, [...path, child]);
}

function findNode(session, terminalID, nodeID) {
  let found = null;
  walk(session.terminals.find(terminal => terminal.id === terminalID).root, node => {
    if (node.id === nodeID) found = node;
  });
  return found;
}

// elementCatalog lists every element of every terminal with the path «ГДЕ»
// shows: the terminal, then each folder below the root, then the element.
function elementCatalog(session) {
  const elements = [];
  for (const terminal of session.terminals) {
    walk(terminal.root, (node, path) => elements.push({
      terminalId: terminal.id, nodeId: node.id, type: node.type, isRoot: node === terminal.root,
      blocks: node.blocks || [], label: [terminal.name, ...path.map(value => value.name)].join(' › '),
    }));
  }
  return elements;
}

// blockLabel names a block by its number and opening text (FR-025).
function blockLabel(block, index) {
  const text = String(block.initialText ?? '').trim().replace(/\s+/gu, ' ');
  const points = Array.from(text);
  const opening = !text ? 'ПУСТО' : points.length > 48 ? `${points.slice(0, 48).join('')}…` : text;
  return `${index + 1} · «${opening}»`;
}

async function whereOptions(page) {
  return page.locator('#facilityBindingNode option[data-node-id]').evaluateAll(options => options.map(option => ({
    terminalId: option.dataset.terminalId, nodeId: option.dataset.nodeId, label: option.textContent,
  })));
}

// visibleClauses reads the shown clauses of the sentence with their label
// and position, in DOM order.
async function visibleClauses(page) {
  return page.locator('#facilityRuleSentence .facility-rule-clause').evaluateAll(clauses => clauses
    .filter(clause => clause.getClientRects().length > 0)
    .map(clause => {
      const box = clause.getBoundingClientRect();
      return {
        clause: clause.dataset.clause,
        label: clause.querySelector('legend, label')?.textContent.trim() ?? '',
        top: box.top,
        left: box.left,
      };
    }));
}

// expectSentence checks that the open tab shows its clauses, and only
// those, in the reading order «КОГДА», «ГДЕ», then what to show (SC-005).
async function expectSentence(page, kind) {
  const rule = ruleDialog(page);
  const { clauses, otherStates } = SENTENCES[kind];
  await expect(rule.tab(kind)).toHaveAttribute('aria-selected', 'true');
  await expect(rule.sentence).toHaveAttribute('aria-labelledby', (await rule.tab(kind).getAttribute('id')));
  const shown = await visibleClauses(page);
  expect(shown.map(value => value.clause), kind).toEqual(clauses);
  expect(shown.map(value => value.label), kind).toEqual(clauses.map(clause => LABELS[clause]));
  for (const [index, value] of shown.entries()) {
    if (!index) continue;
    const previous = shown[index - 1];
    const after = value.top > previous.top + 1 || (Math.abs(value.top - previous.top) <= 1 && value.left > previous.left);
    expect(after, `${kind}: «${value.label}» follows «${previous.label}»`).toBe(true);
  }
  await expect(rule.otherStates).toHaveText(otherStates);
  await expectNoOrphanLabels(rule.dialog, `rule tab ${kind}`);
}

async function saveCount(page) {
  return page.evaluate(() => window.__desktopFixture.calls.filter(call => call.method === 'SaveAuthoring').length);
}

async function openNewRule(page) {
  await page.locator('#facilityTabInformation').click();
  await page.locator('#btnAddFacilityBinding').click();
  const rule = ruleDialog(page);
  await expect(rule.dialog).toBeVisible();
  return rule;
}

async function openRuleRow(page, id) {
  await page.locator('#facilityTabInformation').click();
  await page.locator(`[data-facility-kind="binding"][data-facility-id="${id}"]`)
    .getByRole('button', { name: /^Изменить/ }).click();
  const rule = ruleDialog(page);
  await expect(rule.dialog).toBeVisible();
  return rule;
}

async function closeRule(page) {
  const rule = ruleDialog(page);
  await rule.cancel.click();
  await expect(rule.dialog).toBeHidden();
}

async function openGuideStep(page, step) {
  const guide = page.locator('#facilityStoryGuide');
  if (await guide.getAttribute('open') === null) await page.locator('#facilityStoryGuideSummary').click();
  await expect(guide).toHaveAttribute('open', '');
  await guide.getByRole('button', { name: step, exact: true }).click();
  return page.locator('#facilityStoryActions');
}

async function expectOpensCommandAction(opener) {
  await expect(opener).toHaveAttribute('aria-haspopup', 'dialog');
  await expect(opener).toHaveAttribute('aria-controls', 'facilityCommandActionDialog');
}

test('a new rule opens «ПРАВИЛО ОТОБРАЖЕНИЯ» with four type tabs and no command action (US4 AS1, FR-022, FR-029)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  const rule = await openNewRule(page);

  await expect(page.getByRole('dialog', { name: 'ПРАВИЛО ОТОБРАЖЕНИЯ', exact: true })).toBeVisible();
  await expect(page.locator('#facilityBindingDialogDescription'))
    .toHaveText('Когда устройство в состоянии, где и что показать игрокам.');
  // The tabs sit directly under the title.
  expect(await rule.title.evaluate(title => title.nextElementSibling.id)).toBe('facilityRuleTabs');
  await expect(page.getByRole('tablist', { name: 'Тип правила', exact: true })).toBeVisible();
  await expect(rule.tabs).toHaveText(KINDS.map(kind => SENTENCES[kind].tab));
  await expect(rule.dialog.getByRole('tab', { name: /ДЕЙСТВИЕ/ })).toHaveCount(0);
  await expect(rule.dialog).not.toContainText('ДЕЙСТВИЕ КОМАНДЫ');
  for (const removed of ['facilityBindingKind', 'facilityBindingTerminal', 'facilityBindingAction', 'facilityBindingProgram']) {
    await expect(page.locator(`#${removed}`), removed).toHaveCount(0);
  }

  // Without an element the rule starts on «НАЗВАНИЕ В МЕНЮ», on the selected device.
  await expect(rule.tab('menu-name')).toHaveAttribute('aria-selected', 'true');
  await expect(rule.tab('menu-name')).toBeFocused();
  await expect(rule.device).toHaveValue(DOOR);

  // Automatic activation with wrap, Home and End, and a roving tabindex.
  for (const [key, kind] of [['ArrowRight', 'visibility'], ['End', 'availability'], ['ArrowRight', 'entry-content'],
    ['ArrowLeft', 'availability'], ['Home', 'entry-content'], ['ArrowRight', 'menu-name']]) {
    await page.keyboard.press(key);
    await expect(rule.tab(kind), key).toBeFocused();
    await expect(rule.tab(kind), key).toHaveAttribute('aria-selected', 'true');
    for (const other of KINDS) {
      await expect(rule.tab(other)).toHaveAttribute('tabindex', other === kind ? '0' : '-1');
    }
  }
  await closeRule(page);
  await expect(page.locator('#btnAddFacilityBinding')).toBeFocused();

  // A new rule for a record opens on «ТЕКСТ ЗАПИСИ» with the record in «ГДЕ».
  const actions = await openGuideStep(page, '3. Результат');
  await actions.getByRole('button', { name: 'ДОБАВИТЬ ДРУГОЕ ПРАВИЛО', exact: true }).click();
  await expect(rule.dialog).toBeVisible();
  await expect(rule.tab('entry-content')).toHaveAttribute('aria-selected', 'true');
  await expect(rule.where.locator('option:checked')).toHaveText(SUMMARY_PATH);
  await closeRule(page);
});

test('each tab reads «КОГДА», «ГДЕ», then what to show, with no label without a field (US4 AS2, AS3, FR-023, FR-027, SC-005)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  const rule = await openNewRule(page);

  for (const kind of KINDS) {
    await rule.tab(kind).click();
    await expectSentence(page, kind);
  }
  // «КОГДА» is device «=» state.
  const when = rule.clause('when');
  await expect(when.locator('.facility-rule-equals')).toHaveText('=');
  await expect(when.getByRole('combobox', { name: 'Устройство', exact: true })).toBeVisible();
  await expect(when.getByRole('combobox', { name: 'Состояние', exact: true })).toBeVisible();
  // «РАЗРЕШИТЬ КОМАНДУ» shows only «КОГДА» and «ГДЕ».
  await expect(rule.block).toBeHidden();
  await expect(rule.text).toBeHidden();
  await closeRule(page);
});

test('«ГДЕ» offers only what each tab applies to by its folder path, and «БЛОК» numbers each block (US4 AS2, AS3, FR-024, FR-025)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  const rule = await openNewRule(page);
  // The Overseer lists the session as the fixture saved it.
  const session = await fixtureSession(request);
  const elements = elementCatalog(session);

  for (const kind of KINDS) {
    await rule.tab(kind).click();
    const expected = elements.filter(SENTENCES[kind].offers)
      .map(({ terminalId, nodeId, label }) => ({ terminalId, nodeId, label }));
    expect(expected.length, kind).toBeGreaterThan(0);
    expect(await whereOptions(page), kind).toEqual(expected);
  }
  await expect(rule.where.locator('option').first()).toHaveText('ВЫБЕРИТЕ ЭЛЕМЕНТ');

  await rule.tab('entry-content').click();
  await chooseWhere(page, WARDEN, 'n_entry_vault_shutdown');
  await expect(rule.where.locator('option:checked')).toHaveText(SUMMARY_PATH);
  const summary = findNode(session, WARDEN, 'n_entry_vault_shutdown');
  await expect(rule.block.locator('option')).toHaveText(['ВЫБЕРИТЕ БЛОК', ...summary.blocks.map(blockLabel)]);
  await expect(rule.block.locator('option[value="b_vault_door_status"]')).toHaveText('3 · «ГЕРМОДВЕРЬ: ЗАПЕРТА»');
  // A long opening text is cut after 48 characters.
  await expect(rule.block.locator('option').nth(2)).toHaveText(/^2 · «.{48}…»$/u);
  await closeRule(page);
});

test('the door rule reads as one sentence, and an existing rule keeps its tab (US4 story, FR-026, FR-027)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  const before = await saveCount(page);
  const rule = await openRuleRow(page, `${WARDEN}/b_vault_door_status/entry-content/0`);

  await expect(rule.device.locator('option:checked')).toHaveText('Гермодверь Убежища 76');
  await expect(rule.state.locator('option:checked')).toHaveText('Открыта');
  await expect(rule.where.locator('option:checked')).toHaveText(SUMMARY_PATH);
  await expect(rule.block.locator('option:checked')).toHaveText('3 · «ГЕРМОДВЕРЬ: ЗАПЕРТА»');
  await expect(rule.text).toHaveValue('ГЕРМОДВЕРЬ: ОТКРЫТА // МАРШРУТ СВОБОДЕН');
  await expectSentence(page, 'entry-content');

  for (const kind of KINDS.filter(value => value !== 'entry-content')) {
    expect(await blockedReason(rule.tab(kind)), kind).toBe(FIXED_TAB);
    // A blocked tab does nothing when activated.
    await rule.tab(kind).dispatchEvent('click');
    await expect(rule.tab('entry-content')).toHaveAttribute('aria-selected', 'true');
    await expect(rule.block).toBeVisible();
  }
  await rule.tab('entry-content').focus();
  await page.keyboard.press('ArrowRight');
  await expect(rule.tab('menu-name')).toBeFocused();
  await expect(rule.tab('entry-content')).toHaveAttribute('aria-selected', 'true');
  await closeRule(page);
  expect(await saveCount(page)).toBe(before);
});

test('an edited rule keeps its place among the element\'s rules (FR-026, R5)', async ({ page, request }) => {
  await seedDemo(request);
  const session = await fixtureSession(request);
  await openOverseer(page);
  await selectDevice(page, 'reactor-main');
  const rule = await openRuleRow(page, `${WARDEN}/b_reactor_status/entry-content/0`);
  await expect(rule.state).toHaveValue('standby');
  await fillRule(page, { stateId: 'scrammed', text: 'РЕАКТОР: ОСТАНОВЛЕН ВРУЧНУЮ' });
  await rule.save.click();
  await expect(rule.dialog).toBeHidden();
  await expectSaved(page);

  const variants = value => findNode(value, WARDEN, 'n_entry_facility_status').blocks
    .find(block => block.id === 'b_reactor_status').facilityTextVariants;
  expect(variants(await fixtureSession(request))).toEqual([
    { when: { deviceId: 'reactor-main', stateId: 'scrammed' }, text: 'РЕАКТОР: ОСТАНОВЛЕН ВРУЧНУЮ' },
    variants(session)[1],
  ]);
});

test('switching tabs on a new rule keeps the device, the state, and an element the tab accepts (US4 AS4, FR-026)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  const rule = await openNewRule(page);
  const where = () => rule.where.locator('option:checked');

  await fillRule(page, {
    deviceId: DOOR, stateId: 'open', terminalId: WARDEN, nodeId: 'n_cmd_ordinary_1', text: 'Датчики у открытой двери',
  });
  for (const kind of ['availability', 'visibility', 'menu-name']) {
    await rule.tab(kind).click();
    await expect(rule.device, kind).toHaveValue(DOOR);
    await expect(rule.state, kind).toHaveValue('open');
    await expect(where(), kind).toHaveAttribute('data-node-id', 'n_cmd_ordinary_1');
  }
  await expect(rule.text).toHaveValue('Датчики у открытой двери');

  // A record tab does not take a command, so «ГДЕ» clears and the rest stays.
  await rule.tab('entry-content').click();
  await expect(rule.where).toHaveValue('');
  await expect(rule.device).toHaveValue(DOOR);
  await expect(rule.state).toHaveValue('open');
  await fillRule(page, { terminalId: WARDEN, nodeId: 'n_entry_vault_shutdown', block: 'b_vault_shutdown_heading' });
  await rule.tab('menu-name').click();
  await expect(where()).toHaveAttribute('data-node-id', 'n_entry_vault_shutdown');
  await rule.tab('entry-content').click();
  await expect(rule.block).toHaveValue('b_vault_shutdown_heading');
  await expect(rule.text).toHaveValue('Датчики у открытой двери');

  await rule.save.click();
  await expect(rule.dialog).toBeHidden();
  await expectSaved(page);
  const heading = findNode(await fixtureSession(request), WARDEN, 'n_entry_vault_shutdown').blocks
    .find(block => block.id === 'b_vault_shutdown_heading');
  expect(heading.facilityTextVariants).toEqual([
    { when: { deviceId: DOOR, stateId: 'open' }, text: 'Датчики у открытой двери' },
  ]);
});

test('a conflicting rule names the existing rule, offers to open it, and saves nothing (US4 AS6, FR-028)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  const sessionBefore = await fixtureSession(request);
  const before = await saveCount(page);
  const rule = ruleDialog(page);
  const open = page.locator('#btnOpenConflictingFacilityRule');

  const conflicts = [
    {
      kind: 'menu-name', fill: { deviceId: 'evacuation-alarm', stateId: 'evacuation', nodeId: 'n_cmd_open_evacuation', text: 'Сигнал звучит' },
      message: 'Название этого элемента уже зависит от устройства «Гермодверь Убежища 76». Все правила одного элемента должны зависеть от одного устройства.',
      existing: { deviceId: DOOR, stateId: 'open', text: 'Выход уже открыт' },
    },
    {
      kind: 'entry-content', fill: { deviceId: 'evacuation-alarm', stateId: 'evacuation', nodeId: 'n_entry_vault_shutdown', block: 'b_vault_door_status', text: 'ГЕРМОДВЕРЬ: ПОД СИГНАЛОМ' },
      message: 'Текст этого блока уже зависит от устройства «Гермодверь Убежища 76». Все правила одного блока должны зависеть от одного устройства.',
      existing: { deviceId: DOOR, stateId: 'open', text: 'ГЕРМОДВЕРЬ: ОТКРЫТА // МАРШРУТ СВОБОДЕН' },
    },
    {
      kind: 'menu-name', fill: { deviceId: DOOR, stateId: 'open', nodeId: 'n_cmd_open_evacuation', text: 'Выход распахнут' },
      message: 'Для «Гермодверь Убежища 76 = Открыта» у этого элемента уже есть правило.',
      existing: { deviceId: DOOR, stateId: 'open', text: 'Выход уже открыт' },
    },
    {
      kind: 'visibility', fill: { deviceId: DOOR, stateId: 'sealed', nodeId: 'n_entry_open_route' },
      message: 'У элемента уже есть правило «Показать элемент»: Гермодверь Убежища 76 = Открыта. Измените его вместо нового.',
      existing: { deviceId: DOOR, stateId: 'open' },
    },
    {
      kind: 'availability', fill: { deviceId: DOOR, stateId: 'sealed', nodeId: 'n_cmd_open_evacuation' },
      message: 'У элемента уже есть правило «Разрешить команду»: Резервная сеть Убежища 76 = В работе. Измените его вместо нового.',
      existing: { deviceId: 'power-grid-main', stateId: 'online' },
    },
  ];
  for (const { kind, fill, message, existing } of conflicts) {
    await openNewRule(page);
    await rule.tab(kind).click();
    await fillRule(page, { ...fill, terminalId: WARDEN });
    await rule.save.click();
    await expect(rule.dialog, message).toBeVisible();
    await expect(rule.conflict).toHaveText(message);
    await expect(rule.conflict).toHaveAttribute('role', 'alert');
    await expect(open).toHaveText('ОТКРЫТЬ ПРАВИЛО');

    await open.click();
    await expect(rule.conflict).toBeHidden();
    await expect(open).toBeHidden();
    await expect(rule.tab(kind)).toHaveAttribute('aria-selected', 'true');
    expect(await blockedReason(rule.tab(KINDS.find(value => value !== kind)))).toBe(FIXED_TAB);
    await expect(rule.device).toHaveValue(existing.deviceId);
    await expect(rule.state).toHaveValue(existing.stateId);
    await expect(rule.where.locator('option:checked')).toHaveAttribute('data-node-id', fill.nodeId);
    if (existing.text) await expect(rule.text).toHaveValue(existing.text);
    await closeRule(page);
  }
  expect(await saveCount(page)).toBe(before);
  expect(await fixtureSession(request)).toEqual(sessionBefore);
});

test('an element deleted while the dialog is open is reported on «ГОТОВО» and nothing is saved (FR-028)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  // The new record's save is held and then fails, so the record vanishes
  // from the session after the rule dialog listed it.
  const failure = await request.post(`${FIXTURE}/next-operation`, { data: { failure: 'persistence-failed' } });
  expect(failure.status()).toBe(204);
  await page.evaluate(() => window.__desktopFixture.holdCalls('SaveAuthoring'));
  await page.locator('#btnAddEntry').click();
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'saving');

  await switchTab(page, 'ОБЪЕКТЫ');
  await selectDevice(page, DOOR);
  const rule = await openNewRule(page);
  const record = rule.where.locator('option', { hasText: 'Новая запись' });
  await expect(record).toHaveCount(1);
  await expect(record).toHaveText(`${WARDEN_NAME} › Новая запись`);
  await rule.where.selectOption(await record.getAttribute('value'));
  await fillRule(page, { deviceId: DOOR, stateId: 'open', text: 'Запись у открытой двери' });

  await page.evaluate(() => window.__desktopFixture.releaseHeldCalls('SaveAuthoring'));
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'error');
  const before = await saveCount(page);
  await rule.save.click();
  await expect(rule.dialog).toBeVisible();
  await expect(rule.error).toHaveText('Элемент «Новая запись» удалён. Выберите другой элемент в «ГДЕ».');
  await expect(rule.where).toBeFocused();
  expect(await saveCount(page)).toBe(before);
  const saved = await fixtureSession(request);
  expect(elementCatalog(saved).some(element => element.label.endsWith('› Новая запись'))).toBe(false);
  await closeRule(page);
});

test('«ДЕЙСТВИЕ КОМАНДЫ» opens from the tab, the guide, and the command form, program-backed actions included (US4 AS5, FR-029)', async ({ page, request }) => {
  await seedDemo(request);
  const session = await fixtureSession(request);
  await openOverseer(page);
  const action = commandActionDialog(page);
  const chosen = () => action.command.locator('option:checked');
  const close = async () => {
    await action.cancel.click();
    await expect(action.dialog).toBeHidden();
  };

  // «ЧТО ДЕЛАЮТ ИГРОКИ»: a program-backed action opens in program mode.
  await selectDevice(page, 'ventilation-main');
  await page.locator('#facilityTabActions').click();
  const programRow = page.locator(`[data-facility-kind="binding"][data-facility-id="${WARDEN}/n_cmd_air_recovery/recovery-action/0"]`)
    .getByRole('button', { name: /^Изменить/ });
  await expectOpensCommandAction(programRow);
  await programRow.click();
  await expect(page.getByRole('dialog', { name: 'ДЕЙСТВИЕ КОМАНДЫ', exact: true })).toBeVisible();
  await expect(page.locator('#facilityCommandActionDialogDescription'))
    .toHaveText('Что запускает команда после подтверждения Смотрителем.');
  await expect(action.commandFixed).toHaveText(AIR_PATH);
  await expect(action.command).toBeHidden();
  await expect(action.modeProgram).toBeChecked();
  await expect(action.program).toBeVisible();
  await expect(action.program).toHaveValue('air-recovery-76');
  await expect(action.program.locator('option')).toHaveText([
    'ВЫБЕРИТЕ ПРОГРАММУ', ...session.facility.recoveryPrograms.map(program => program.name),
  ]);
  await expect(action.rows).toHaveCount(0);
  await expect(action.addTransition).toBeHidden();
  await expect(action.completedName).toBeHidden();
  await expect(action.confirmationText).toBeHidden();
  await close();
  await expect(programRow).toBeFocused();

  // A transition action opens in transition mode with its rows.
  await selectDevice(page, DOOR);
  await page.locator('#facilityTabActions').click();
  const transitionRow = page.locator(`[data-facility-kind="binding"][data-facility-id="${WARDEN}/n_cmd_open_evacuation/command-action/0"]`)
    .getByRole('button', { name: /^Изменить/ });
  await expectOpensCommandAction(transitionRow);
  await transitionRow.click();
  await expect(action.commandFixed).toHaveText(EVACUATION_PATH);
  await expect(action.modeTransitions).toBeChecked();
  await expect(action.program).toBeHidden();
  expect(await action.rows.evaluateAll(rows => rows.map(row => [
    row.querySelector('.facility-action-device').value, row.querySelector('.facility-action-transition').value,
  ]))).toEqual([[DOOR, 'open'], ['evacuation-alarm', 'sound-evacuation']]);
  await close();

  // «ДОБАВИТЬ ДЕЙСТВИЕ КОМАНДЫ» picks the first command without an action.
  const add = page.locator('#btnAddFacilityActionBinding');
  await expectOpensCommandAction(add);
  await add.click();
  await expect(action.command).toBeVisible();
  await expect(action.commandFixed).toBeHidden();
  await expect(chosen()).toHaveAttribute('data-node-id', 'n_cmd_1');
  await close();

  // The guide's «ДЕЙСТВИЕ» step edits the device's action or picks another command.
  const guide = await openGuideStep(page, '2. Действие');
  const edit = guide.getByRole('button', { name: 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ', exact: true });
  await expectOpensCommandAction(edit);
  await edit.click();
  await expect(action.commandFixed).toHaveText(EVACUATION_PATH);
  await expect(action.command).toBeHidden();
  await close();
  const other = guide.getByRole('button', { name: 'СВЯЗАТЬ С ДРУГОЙ КОМАНДОЙ', exact: true });
  await expectOpensCommandAction(other);
  await other.click();
  await expect(action.command).toBeVisible();
  await expect(chosen()).toHaveAttribute('data-node-id', 'n_cmd_1');
  await close();

  // The command form opens the program-backed action, and its save keeps the shape.
  await switchTab(page, 'ТЕРМИНАЛЫ');
  await page.locator('.tree-row', { hasText: 'ПРОТОКОЛ ВЫХОДА' }).click();
  await page.locator('.tree-row', { hasText: 'Загрузить кассету очистки воздуха' }).click();
  const formOpener = page.locator('#btnEditCommandFacilityAction');
  await expect(formOpener).toHaveText('ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ');
  await expectOpensCommandAction(formOpener);
  await formOpener.click();
  await expect(action.commandFixed).toHaveText(AIR_PATH);
  await expect(action.modeProgram).toBeChecked();
  await expect(action.program).toHaveValue('air-recovery-76');
  await action.program.selectOption('network-recovery-76');
  await action.save.click();
  await expect(action.dialog).toBeHidden();
  await expect(formOpener).toBeFocused();
  await expectSaved(page);
  const original = findNode(session, WARDEN, 'n_cmd_air_recovery').stateChange;
  expect(findNode(await fixtureSession(request), WARDEN, 'n_cmd_air_recovery').stateChange).toEqual({
    ...original, facilityAction: { recoveryProgramId: 'network-recovery-76' },
  });
  await expect(page.locator('#commandEffects li.command-effect[data-kind="recovery"]'))
    .toHaveText('ПРОГРАММА ВОССТАНОВЛЕНИЯ: СЕТЬ-76');
});

// Elements seeded into the player fixture, which has a single command.
const PLAYER_TERMINAL = 'terminal-facility-security';
const PLAYER_DOOR = 'device-security-door';
const PLAYER_ELEMENTS = [
  {
    id: 'entry-archive-log', type: 'entry', name: 'ARCHIVE LOG',
    blocks: [{ id: 'block-archive-door', initialText: 'DOOR LOG: SEALED' }],
  },
  {
    id: 'entry-clearance-note', type: 'entry', name: 'CLEARANCE NOTE',
    blocks: [{ id: 'block-clearance', initialText: 'CLEARANCE GRANTED' }],
  },
  { id: 'command-silence-alarm', type: 'command', name: 'SILENCE ALARM', text: 'Alarm silenced.' },
];

// seedPlayerElements saves the elements through the fixture's authoring
// save, then reopens the Overseer on the newer session.
async function seedPlayerElements(request, overseer) {
  const opened = await (await request.get(`${PLAYER_FIXTURE}/open-session`)).json();
  const session = structuredClone(opened.session);
  session.terminals.find(terminal => terminal.id === PLAYER_TERMINAL).root.children.push(...structuredClone(PLAYER_ELEMENTS));
  const response = await request.post(`${PLAYER_FIXTURE}/save-authoring`, {
    data: { session, expectedAuthoringRevision: opened.authoringRevision, correlationId: 'seed-display-rules' },
  });
  expect(response.ok()).toBe(true);
  expect(await response.json()).toMatchObject({ ok: true });
  await overseer.reload();
  await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ', exact: true }).click();
  await expect(overseer.locator('#mainLayout')).toBeVisible();
  await overseer.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
}

test('one rule of each type, written in sentence order, shows on a player terminal only in its state (US4 independent test, FR-045, SC-005)', async ({ browser, request }) => {
  const table = await openPlayerTable(browser, request);
  try {
    const { overseer, player } = table;
    await seedPlayerElements(request, overseer);
    await selectDevice(overseer, PLAYER_DOOR);
    const rule = ruleDialog(overseer);
    const rules = [
      { kind: 'entry-content', nodeId: 'entry-archive-log', block: 'block-archive-door', text: 'DOOR LOG: OPEN' },
      { kind: 'menu-name', nodeId: 'entry-archive-log', text: 'ARCHIVE LOG // OPEN' },
      { kind: 'visibility', nodeId: 'entry-clearance-note' },
      { kind: 'availability', nodeId: 'command-silence-alarm' },
    ];
    for (const { kind, ...sentence } of rules) {
      await openNewRule(overseer);
      await rule.tab(kind).click();
      await expectSentence(overseer, kind);
      await fillRule(overseer, { deviceId: PLAYER_DOOR, stateId: 'open', terminalId: PLAYER_TERMINAL, ...sentence });
      await rule.save.click();
      await expect(rule.dialog, kind).toBeHidden();
      await expectSaved(overseer);
    }
    const saved = await (await request.get(`${PLAYER_FIXTURE}/session`)).json();
    const when = { deviceId: PLAYER_DOOR, stateId: 'open' };
    const log = findNode(saved, PLAYER_TERMINAL, 'entry-archive-log');
    expect(log.blocks[0].facilityTextVariants).toEqual([{ when, text: 'DOOR LOG: OPEN' }]);
    expect(log.facilityNameVariants).toEqual([{ when, text: 'ARCHIVE LOG // OPEN' }]);
    expect(findNode(saved, PLAYER_TERMINAL, 'entry-clearance-note').visibleWhen).toEqual(when);
    expect(findNode(saved, PLAYER_TERMINAL, 'command-silence-alarm').availableWhen).toEqual(when);

    // The door is locked: players see the original name and text, no note,
    // and a command they cannot request.
    const row = id => player.locator(`.term-row[data-node-id="${id}"]`);
    await expect(row('entry-archive-log')).toHaveText('> ARCHIVE LOG');
    await expect(row('entry-clearance-note')).toHaveCount(0);
    await expect(row('command-silence-alarm')).toHaveAttribute('aria-disabled', 'true');
    await row('entry-archive-log').click();
    await expect(player.locator('#entryBody')).toContainText('DOOR LOG: SEALED');

    // The door opens: every rule applies at once.
    await playCard(overseer).run('open').click();
    await expect(overseer.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Open');
    await expect(player.locator('#entryBody')).toContainText('DOOR LOG: OPEN');
    await expect(player.locator('#entryBody')).not.toContainText('DOOR LOG: SEALED');
    await player.locator('#backBtn').click();
    await expect(row('entry-archive-log')).toHaveText('> ARCHIVE LOG // OPEN');
    await expect(row('entry-clearance-note')).toBeVisible();
    await expect(row('command-silence-alarm')).not.toHaveAttribute('aria-disabled', 'true');
  } finally {
    await table.close();
  }
});
