import { expect, test } from '@playwright/test';

import { facilityState, openOverseer, resetFixture } from './helpers/overseer-forms.mjs';

const APPLY = 'ПРИМЕНИТЬ К ЧЕРНОВИКУ';
const SAVE = 'СОХРАНИТЬ ИЗМЕНЕНИЯ СЕССИИ';

async function openNewDevice(page) {
  await page.locator('#btnAddFacilityDevice').click();
  const dialog = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function addState(dialog, name) {
  await dialog.getByRole('button', { name: 'ДОБАВИТЬ СОСТОЯНИЕ', exact: true }).click();
  await dialog.locator('.facility-state-name').last().fill(name);
}

const values = locator => locator.evaluateAll(inputs => inputs.map(input => input.value));

test('Russian names produce readable, unique identifiers that survive save and rename', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  await openOverseer(page);
  const dialog = await openNewDevice(page);
  const deviceID = dialog.locator('#facilityDeviceID');

  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Очень длинное название шлюза '.repeat(12));
  expect((await deviceID.inputValue()).length).toBeLessThanOrEqual(256);
  await expect(deviceID).toHaveValue(/^device-ochen-dlinnoe-nazvanie-shlyuza-/);
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Дверь в убежище');
  await expect(deviceID).toHaveValue('device-dver-v-ubezhishche');
  await dialog.getByLabel('ТИП', { exact: true }).selectOption('door');

  for (const name of ['Заперта', 'Открыта', 'Открыта!', '★★★', 'Ґанок', 'Café']) await addState(dialog, name);
  expect(await values(dialog.locator('.facility-state-id')))
    .toEqual(['zaperta', 'otkryta', 'otkryta-2', 'state-1', 'ganok', 'cafe']);

  await dialog.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД', exact: true }).click();
  const transition = dialog.locator('.facility-transition-row').last();
  await transition.locator('.facility-transition-name').fill('Открыть дверь');
  await expect(transition.locator('.facility-transition-id')).toHaveValue('otkryt-dver');
  await transition.locator('.facility-transition-source').selectOption('zaperta');
  await transition.locator('.facility-transition-destination').selectOption('otkryta');
  await dialog.getByLabel('НАЧАЛЬНОЕ СОСТОЯНИЕ', { exact: true }).selectOption('zaperta');
  await dialog.getByRole('button', { name: APPLY, exact: true }).click();
  await expect(dialog).toBeHidden();
  await page.getByRole('button', { name: SAVE, exact: true }).click();
  await expect(page.locator('#facilityStatus')).toHaveText('ИЗМЕНЕНИЯ СЕССИИ СОХРАНЕНЫ');

  const saved = (await facilityState(request)).facility.devices[0];
  expect(saved.id).toBe('device-dver-v-ubezhishche');
  expect(saved.states.map(value => value.id)).toEqual(['zaperta', 'otkryta', 'otkryta-2', 'state-1', 'ganok', 'cafe']);
  expect(saved.transitions.map(value => value.id)).toEqual(['otkryt-dver']);

  await page.getByRole('button', { name: 'РЕДАКТИРОВАТЬ ГРАФ', exact: true }).click();
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Гермодверь');
  await dialog.locator('.facility-state-name').first().fill('Закрыта');
  await dialog.locator('.facility-transition-name').first().fill('Распахнуть дверь');
  await expect(deviceID).toHaveValue('device-dver-v-ubezhishche');
  await expect(dialog.locator('.facility-state-id').first()).toHaveValue('zaperta');
  await expect(dialog.locator('.facility-transition-id').first()).toHaveValue('otkryt-dver');
  await dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();

  await page.locator('#btnAddFacilityCondition').click();
  const condition = page.getByRole('dialog', { name: 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', exact: true });
  await condition.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Отказ привода');
  await expect(condition.locator('#facilityConditionID')).toHaveValue('condition-otkaz-privoda');
  await condition.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();

  await page.locator('#btnAddFacilityRecoveryProgram').click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await program.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Перезапуск контроллера');
  await expect(program.locator('#facilityRecoveryProgramID')).toHaveValue('program-perezapusk-kontrollera');
});

test('invalid state rows get their own messages and the first one receives focus', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  await openOverseer(page);
  const dialog = await openNewDevice(page);
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Шлюз');
  await dialog.getByLabel('ТИП', { exact: true }).selectOption('door');
  for (const name of ['Открыт', 'Закрыт', 'Аварийный', ' открыт ']) await addState(dialog, name);
  await dialog.getByLabel('НАЧАЛЬНОЕ СОСТОЯНИЕ', { exact: true }).selectOption('otkryt');
  const rows = dialog.locator('.facility-state-row');
  await rows.nth(1).locator('.facility-state-name').fill('');
  await rows.nth(2).locator('.facility-state-id').fill('otkryt');

  await dialog.getByRole('button', { name: APPLY, exact: true }).click();
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#facilityDeviceError')).toHaveText('ИСПРАВЬТЕ ОТМЕЧЕННЫЕ СТРОКИ: 3');
  await expect(rows.nth(0).locator('.facility-row-error')).toHaveCount(0);
  await expect(rows.nth(1).locator('.facility-row-error')).toHaveText('УКАЖИТЕ НАЗВАНИЕ СОСТОЯНИЯ');
  await expect(rows.nth(2).locator('.facility-row-error')).toHaveText('ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: otkryt');
  await expect(rows.nth(3).locator('.facility-row-error')).toHaveText('НАЗВАНИЕ УЖЕ ИСПОЛЬЗУЕТСЯ: открыт');
  await expect(rows.nth(1).locator('.facility-state-name')).toBeFocused();
  await expect(rows.nth(1).locator('.facility-state-name')).toHaveAttribute('aria-invalid', 'true');
  await expect(rows.nth(2).locator('.facility-state-id')).toHaveAttribute('aria-invalid', 'true');

  await rows.nth(1).locator('.facility-state-name').fill('Закрыт');
  await expect(rows.nth(1).locator('.facility-row-error')).toHaveCount(0);
  await rows.nth(2).locator('.facility-state-id').fill('avariynyy');
  await rows.nth(3).locator('.facility-state-name').fill('Заблокирован');
  await dialog.getByRole('button', { name: APPLY, exact: true }).click();
  await expect(dialog).toBeHidden();
});
