import { expect, test } from '@playwright/test';

import { expectSaved, saveStatus, switchTab } from './helpers/authoring-bar.mjs';
import { facilityState, fixtureSession, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

const EVACUATION = 'Открыть выход и подать сигнал';
const ORDINARY = 'Проверить наружные датчики';
const DONE = 'ГОТОВО';
const RENAMED_DOOR = 'Гермодверь 76';

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

async function renameDoor(page, request) {
  await switchTab(page, 'ОБЪЕКТЫ');
  await page.locator('[data-facility-kind="device"][data-facility-id="vault-door-main"]').click();
  await page.locator('#facilitySelectedDeviceName').fill(RENAMED_DOOR);
  await page.getByRole('button', { name: DONE, exact: true }).click();
  await expect.poll(async () => (await facilityState(request)).facility.devices
    .find(device => device.id === 'vault-door-main').name).toBe(RENAMED_DOOR);
  await expectSaved(page);
  await switchTab(page, 'ТЕРМИНАЛЫ');
}

test('leaving state-changing mode with a device action requires keep or remove before applying', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await openExitProtocolCommand(page, EVACUATION);
  const mode = page.locator('#fldCommandMode');
  const decision = page.locator('#commandModeDecision');
  const apply = page.locator('#btnApplyNode');
  const action = page.locator('.command-facility-action');
  await expect(decision).toBeHidden();
  await expect(action).toBeVisible();

  await mode.selectOption('ordinary');
  await expect(decision).toBeVisible();
  await expect(decision).toContainText('Гермодверь Убежища 76');
  await expect(decision).toContainText('Сигнал эвакуации');
  await expect(decision).toContainText('ВЫБЕРИТЕ, ЧТО СДЕЛАТЬ С ДЕЙСТВИЕМ УСТРОЙСТВ');
  await expect(action).toBeHidden();
  await expect(apply).toBeDisabled();

  await page.locator('#btnKeepStateChangeMode').click();
  await expect(mode).toHaveValue('state-change');
  await expect(decision).toBeHidden();
  await expect(action).toBeVisible();
  await expect(apply).toBeEnabled();

  await mode.selectOption('terminal-transition');
  await expect(decision).toBeVisible();
  await mode.selectOption('state-change');
  await expect(decision).toBeHidden();

  const before = await facilityState(request);
  await mode.selectOption('ordinary');
  await page.locator('#btnRemoveDeviceAction').click();
  await expect(apply).toBeEnabled();
  expect((await facilityState(request)).sessionRevision).toBe(before.sessionRevision);
  await apply.click();
  await expect.poll(async () => (await savedCommand(request, 'n_cmd_open_evacuation'))?.stateChange).toBeUndefined();

  await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  await page.locator('[data-facility-kind="device"][data-facility-id="vault-door-main"]').click();
  await expect(page.getByRole('list', { name: 'Что делают игроки', exact: true })).not.toContainText(EVACUATION);
});

test('the device-action block appears only in state-changing mode and waits for that mode to be applied', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await openExitProtocolCommand(page, ORDINARY);
  const action = page.locator('.command-facility-action');
  await expect(action).toBeHidden();
  await expect(page.locator('#commandModeDecision')).toBeHidden();

  await page.locator('#fldCommandMode').selectOption('state-change');
  await expect(action).toBeVisible();
  await expect(action).toContainText('ПРИМЕНИТЕ РЕЖИМ, ЧТОБЫ ДОБАВИТЬ ДЕЙСТВИЕ УСТРОЙСТВ');
  await expect(page.locator('#btnEditCommandFacilityAction')).toBeDisabled();
});

test('an executed command with a device action keeps its mode locked and shows no decision', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await openExitProtocolCommand(page, 'Резервная сеть запущена');
  await expect(page.locator('#fldCommandMode')).toBeDisabled();
  await expect(page.locator('#commandModeDecision')).toBeHidden();
});

test('a command with a device action has no draft guard and its edit saves after a facility edit (FR-006)', async ({ page, request }) => {
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

test('attaching an action to an ordinary command asks for both player texts and never invents them', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await page.locator('#btnAddFacilityBinding').click();
  const binding = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА', exact: true });
  await binding.getByLabel('ТИП ПРИВЯЗКИ', { exact: true }).selectOption('command-action');
  await binding.getByLabel('ТЕРМИНАЛ', { exact: true }).selectOption('t_demo1');
  const completed = binding.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ', { exact: true });
  const confirmation = binding.getByLabel('ЗАПРОС ПОДТВЕРЖДЕНИЯ', { exact: true });

  await binding.getByLabel('ЭЛЕМЕНТ', { exact: true }).selectOption('n_cmd_open_evacuation');
  await expect(completed).toBeHidden();
  await expect(confirmation).toBeHidden();

  await binding.getByLabel('ЭЛЕМЕНТ', { exact: true }).selectOption('n_cmd_ordinary_1');
  await expect(completed).toBeVisible();
  await expect(completed).toHaveValue('');
  await expect(confirmation).toHaveValue('');
  const row = binding.locator('.facility-action-request-row').first();
  await row.getByLabel('УСТРОЙСТВО ПЕРЕХОДА').selectOption('evacuation-alarm');
  await row.getByLabel('ПЕРЕХОД КОМАНДЫ').selectOption('sound-evacuation');
  await binding.getByRole('button', { name: DONE, exact: true }).click();
  await expect(binding).toBeVisible();
  await expect(completed).toBeFocused();
  await expect(binding.locator('#facilityBindingError')).toContainText('ЗАПОЛНИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ И ЗАПРОС ПОДТВЕРЖДЕНИЯ');

  await completed.fill('Датчики опрошены');
  await confirmation.fill('Опросить наружные датчики и подать сигнал?');
  await binding.getByRole('button', { name: DONE, exact: true }).click();
  await expect(binding).toBeHidden();
  await expect.poll(async () => (await savedCommand(request, 'n_cmd_ordinary_1')).stateChange).toBeTruthy();
  await expectSaved(page);
  expect((await savedCommand(request, 'n_cmd_ordinary_1')).stateChange).toEqual({
    completedName: 'Датчики опрошены',
    confirmationText: 'Опросить наружные датчики и подать сигнал?',
    facilityAction: { transitions: { transitions: [{ deviceId: 'evacuation-alarm', transitionId: 'sound-evacuation' }] } },
  });
});
