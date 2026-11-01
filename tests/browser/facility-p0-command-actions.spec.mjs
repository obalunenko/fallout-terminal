import { expect, test } from '@playwright/test';

import { expectSaved, saveStatus, switchTab } from './helpers/authoring-bar.mjs';
import { commandActionDialog, openDeviceDialog } from './helpers/facility-editors.mjs';
import { facilityState, fixtureSession, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

// A command's device action is edited in «ДЕЙСТВИЕ КОМАНДЫ» (FR-029, R7).

const EVACUATION = 'Открыть выход и подать сигнал';
const ORDINARY = 'Проверить наружные датчики';
const RENAMED_DOOR = 'Гермодверь 76';
const KEEP_OR_REMOVE = 'ВЫБЕРИТЕ, ЧТО СДЕЛАТЬ С ДЕЙСТВИЕМ КОМАНДЫ. ОНО РАБОТАЕТ ТОЛЬКО В РЕЖИМЕ «ИЗМЕНЯЕТ СОСТОЯНИЕ».';
const WILL_REMOVE = 'ДЕЙСТВИЕ КОМАНДЫ БУДЕТ УДАЛЕНО, КОГДА ВЫ ПРИМЕНИТЕ ФОРМУ.';
const TEXTS_MISSING = 'ЗАПОЛНИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ И ЗАПРОС ПОДТВЕРЖДЕНИЯ';

function findNode(node, id) {
  if (node.id === id) return node;
  for (const child of node.children || []) {
    const found = findNode(child, id);
    if (found) return found;
  }
  return null;
}

async function savedCommand(request, id) {
  const session = await fixtureSession(request);
  for (const terminal of session.terminals) {
    const node = findNode(terminal.root, id);
    if (node) return node;
  }
  return null;
}

async function openExitProtocolCommand(page, name) {
  await page.locator('.tree-row', { hasText: 'ПРОТОКОЛ ВЫХОДА' }).click();
  await page.locator('.tree-row', { hasText: name }).click();
  await expect(page.locator('#fldName')).toBeVisible();
}

async function saveCount(page) {
  return page.evaluate(() => window.__desktopFixture.calls.filter(call => call.method === 'SaveAuthoring').length);
}

// chooseCommand picks a «КОМАНДА» option by its terminal and node IDs, since
// option values are opaque indexes.
async function chooseCommand(page, terminalId, nodeId) {
  const option = page.locator(
    `#facilityCommandActionCommand option[data-terminal-id="${terminalId}"][data-node-id="${nodeId}"]`);
  await expect(option).toHaveCount(1);
  await page.locator('#facilityCommandActionCommand').selectOption(await option.getAttribute('value'));
}

async function renameDoor(page, request) {
  await switchTab(page, 'ОБЪЕКТЫ');
  const device = await openDeviceDialog(page, 'vault-door-main');
  await device.name.fill(RENAMED_DOOR);
  await device.save.click();
  await expect(device.dialog).toBeHidden();
  await expect.poll(async () => (await facilityState(request)).facility.devices
    .find(value => value.id === 'vault-door-main').name).toBe(RENAMED_DOOR);
  await expectSaved(page);
  await switchTab(page, 'ТЕРМИНАЛЫ');
}

test('leaving state-changing mode with a command action requires keep or remove before applying (FR-029)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await openExitProtocolCommand(page, EVACUATION);
  const mode = page.locator('#fldCommandMode');
  const decision = page.locator('#commandModeDecision');
  const apply = page.locator('#btnApplyNode');
  const action = page.locator('.command-facility-action');
  const remove = page.locator('#btnRemoveDeviceAction');
  await expect(decision).toBeHidden();
  await expect(action).toBeVisible();
  await expect(page.locator('#commandFacilityActionHeading')).toHaveText('ДЕЙСТВИЕ КОМАНДЫ');

  await mode.selectOption('ordinary');
  await expect(decision).toBeVisible();
  await expect(decision.locator('[data-decision-message]')).toHaveText(KEEP_OR_REMOVE);
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-unapplied-count', '1');
  await expect(decision).toContainText('Гермодверь Убежища 76');
  await expect(decision).toContainText('Сигнал эвакуации');
  await expect(action).toBeHidden();
  await expect(apply).toBeDisabled();

  await page.locator('#btnKeepStateChangeMode').click();
  await expect(mode).toHaveValue('state-change');
  // «Оставить» restores the applied mode, so nothing is left unapplied.
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-unapplied-count', '0');
  await expect(page.locator('#saveStatus')).not.toContainText('НЕ ПРИМЕНЕНО');
  await expect(decision).toBeHidden();
  await expect(action).toBeVisible();
  await expect(apply).toBeEnabled();

  await mode.selectOption('terminal-transition');
  await expect(decision).toBeVisible();
  await mode.selectOption('state-change');
  await expect(decision).toBeHidden();

  const before = await facilityState(request);
  await mode.selectOption('ordinary');
  await remove.click();
  await expect(decision.locator('[data-decision-message]')).toHaveText(WILL_REMOVE);
  await expect(remove).toBeHidden();
  await expect(apply).toBeEnabled();
  expect((await facilityState(request)).sessionRevision).toBe(before.sessionRevision);
  await apply.click();
  await expect.poll(async () => (await savedCommand(request, 'n_cmd_open_evacuation'))?.stateChange).toBeUndefined();

  await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  await page.locator('[data-facility-kind="device"][data-facility-id="vault-door-main"]').click();
  await expect(page.getByRole('list', { name: 'Что делают игроки', exact: true })).not.toContainText(EVACUATION);
});

test('a command switched to state-changing mode adds its action at once, with the form\'s player texts (FR-029)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await openExitProtocolCommand(page, ORDINARY);
  const section = page.locator('.command-facility-action');
  const opener = page.locator('#btnEditCommandFacilityAction');
  await expect(section).toBeHidden();
  await expect(page.locator('#commandModeDecision')).toBeHidden();

  // The mode is not applied yet, and the action is still available.
  await page.locator('#fldCommandMode').selectOption('state-change');
  await expect(section).toBeVisible();
  await expect(section).not.toContainText('ПРИМЕНИТЕ РЕЖИМ');
  await expect(opener).toHaveText('ДОБАВИТЬ ДЕЙСТВИЕ КОМАНДЫ');
  await expect(opener).toBeEnabled();
  await expect(opener).toHaveAttribute('aria-haspopup', 'dialog');
  await expect(opener).toHaveAttribute('aria-controls', 'facilityCommandActionDialog');
  await page.locator('#fldCompletedName').fill('Датчики опрошены');
  await page.locator('#fldConfirmationText').fill('Опросить наружные датчики?');

  await opener.click();
  const action = commandActionDialog(page);
  await expect(page.getByRole('dialog', { name: 'ДЕЙСТВИЕ КОМАНДЫ', exact: true })).toBeVisible();
  await expect(action.commandFixed).toHaveText(`Терминал смотрителя — Убежище 76 › ПРОТОКОЛ ВЫХОДА › ${ORDINARY}`);
  await expect(action.command).toBeHidden();
  await expect(action.modeTransitions).toBeChecked();
  // The command has no stateChange yet, so the dialog asks for the texts the form holds.
  await expect(action.completedName).toHaveValue('Датчики опрошены');
  await expect(action.confirmationText).toHaveValue('Опросить наружные датчики?');
  await expect(action.rows).toHaveCount(1);
  await action.rows.first().locator('.facility-action-device').selectOption('evacuation-alarm');
  await action.rows.first().locator('.facility-action-transition').selectOption('sound-evacuation');
  await action.save.click();
  await expect(action.dialog).toBeHidden();
  await expect(opener).toBeFocused();
  await expect.poll(async () => (await savedCommand(request, 'n_cmd_ordinary_1'))?.stateChange).toBeTruthy();
  await expectSaved(page);
  expect((await savedCommand(request, 'n_cmd_ordinary_1')).stateChange).toEqual({
    completedName: 'Датчики опрошены',
    confirmationText: 'Опросить наружные датчики?',
    facilityAction: { transitions: { transitions: [{ deviceId: 'evacuation-alarm', transitionId: 'sound-evacuation' }] } },
  });
  await expect(page.locator('#fldCommandMode')).toHaveValue('state-change');
  await expect(opener).toHaveText('ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ');
});

test('an executed command with a command action keeps its mode locked and shows no decision', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await openExitProtocolCommand(page, 'Резервная сеть запущена');
  await expect(page.locator('#fldCommandMode')).toBeDisabled();
  await expect(page.locator('#commandModeDecision')).toBeHidden();
});

test('a command with a command action has no draft guard and its edit saves after a facility edit (FR-006)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await renameDoor(page, request);
  const afterRename = await saveCount(page);

  await openExitProtocolCommand(page, EVACUATION);
  await expect(page.locator('#commandDraftGuard')).toHaveCount(0);
  await expect(page.locator('#nodeForm')).not.toContainText('ЧЕРНОВИК');
  await expect(page.locator('#fldCommandMode')).toBeEnabled();
  await expect(page.locator('#btnApplyNode')).toBeEnabled();
  // The command form already names the renamed device.
  await expect(page.locator('.command-facility-action')).toContainText(RENAMED_DOOR);

  await page.locator('#fldText').fill('Выход открыт, сигнал подан.');
  await page.locator('#btnApplyNode').click();
  await expect.poll(() => saveCount(page)).toBe(afterRename + 1);
  await expectSaved(page);
  await expect.poll(async () => (await savedCommand(request, 'n_cmd_open_evacuation'))?.text)
    .toBe('Выход открыт, сигнал подан.');
  const saved = await fixtureSession(request);
  expect(saved.facility.devices.find(device => device.id === 'vault-door-main').name).toBe(RENAMED_DOOR);
  expect((await savedCommand(request, 'n_cmd_open_evacuation')).stateChange.facilityAction).toBeTruthy();
  expect((await saveStatus(page)).text).not.toMatch(/СЕССИЯ ИЗМЕНИЛАСЬ|ЧЕРНОВИК/i);

  await switchTab(page, 'ОБЪЕКТЫ');
  await expect(page.locator('[data-facility-kind="device"][data-facility-id="vault-door-main"]')).toContainText(RENAMED_DOOR);
  expect((await saveStatus(page)).text).not.toMatch(/СЕССИЯ ИЗМЕНИЛАСЬ|ЧЕРНОВИК/i);
});

test('attaching an action to an ordinary command asks for both player texts and never invents them (FR-029)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  const before = await saveCount(page);
  await page.locator('#btnAddFacilityActionBinding').click();
  const action = commandActionDialog(page);
  await expect(page.getByRole('dialog', { name: 'ДЕЙСТВИЕ КОМАНДЫ', exact: true })).toBeVisible();
  await expect(action.command).toBeVisible();
  await expect(action.commandFixed).toBeHidden();
  // A command that moves players to another terminal takes no action.
  await expect(action.command.locator('option[data-node-id="n_cmd_state_change_1"]')).toHaveCount(0);

  // A command with a stateChange keeps its player texts.
  await chooseCommand(page, 't_demo1', 'n_cmd_open_evacuation');
  await expect(action.modeTransitions).toBeChecked();
  await expect(action.rows).toHaveCount(2);
  await expect(action.completedName).toBeHidden();
  await expect(action.confirmationText).toBeHidden();

  await chooseCommand(page, 't_demo1', 'n_cmd_ordinary_1');
  await expect(action.completedName).toBeVisible();
  await expect(action.completedName).toHaveValue('');
  await expect(action.confirmationText).toHaveValue('');
  await expect(action.rows).toHaveCount(1);
  await action.rows.first().locator('.facility-action-device').selectOption('evacuation-alarm');
  await action.rows.first().locator('.facility-action-transition').selectOption('sound-evacuation');
  await action.save.click();
  await expect(action.dialog).toBeVisible();
  await expect(action.completedName).toBeFocused();
  await expect(action.error).toHaveText(TEXTS_MISSING);
  expect(await saveCount(page)).toBe(before);

  await action.completedName.fill('Датчики опрошены');
  await action.save.click();
  await expect(action.confirmationText).toBeFocused();
  await expect(action.error).toHaveText(TEXTS_MISSING);
  await action.confirmationText.fill('Опросить наружные датчики и подать сигнал?');
  await action.save.click();
  await expect(action.dialog).toBeHidden();
  await expect.poll(async () => (await savedCommand(request, 'n_cmd_ordinary_1')).stateChange).toBeTruthy();
  await expectSaved(page);
  expect(await saveCount(page)).toBe(before + 1);
  expect((await savedCommand(request, 'n_cmd_ordinary_1')).stateChange).toEqual({
    completedName: 'Датчики опрошены',
    confirmationText: 'Опросить наружные датчики и подать сигнал?',
    facilityAction: { transitions: { transitions: [{ deviceId: 'evacuation-alarm', transitionId: 'sound-evacuation' }] } },
  });
});
