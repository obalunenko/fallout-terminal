import { expect, test } from '@playwright/test';

import { expectSaved } from './helpers/authoring-bar.mjs';
import { deviceDialog, openTransitionCell } from './helpers/facility-editors.mjs';
import { facilityState, openOverseer, resetFixture } from './helpers/overseer-forms.mjs';

// Identifiers live in the folded «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ» block of every
// facility dialog (specs/041-facility-editors-language/spec.md FR-007, SC-002).

async function openNewDevice(page) {
  await page.locator('#btnAddFacilityDevice').click();
  await expect(page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true })).toBeVisible();
  return deviceDialog(page);
}

// addState presses «+ СОСТОЯНИЕ» and renames the new state in its chip editor.
async function addState(page, name) {
  await page.locator('#btnAddFacilityState').click();
  await page.locator('#facilityStateEditorName').fill(name);
}

// unfold opens a folded «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ» block.
async function unfold(details) {
  await expect(details).not.toHaveAttribute('open', '');
  await expect(details.locator('summary')).toHaveText('ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ');
  await details.locator('summary').click();
  await expect(details).toHaveAttribute('open', '');
}

async function fold(details) {
  if (await details.evaluate(element => element.open)) await details.locator('summary').click();
  await expect(details).not.toHaveAttribute('open', '');
}

const values = locator => locator.evaluateAll(inputs => inputs.map(input => input.value));

test('Russian names suggest readable, unique identifiers in the folded block that survive save and rename (FR-007)', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  await openOverseer(page);
  const dialog = await openNewDevice(page);
  const deviceID = page.locator('#facilityDeviceID');
  const stateIDs = dialog.technical.locator('.facility-state-row .facility-state-id');
  const transitionIDs = dialog.technical.locator('.facility-transition-id-row .facility-transition-id');

  await dialog.name.fill('Очень длинное название шлюза '.repeat(12));
  await expect(deviceID).toBeHidden();
  await unfold(dialog.technical);
  await expect(deviceID).toBeVisible();
  expect((await deviceID.inputValue()).length).toBeLessThanOrEqual(256);
  await expect(deviceID).toHaveValue(/^device-ochen-dlinnoe-nazvanie-shlyuza-/);
  await dialog.name.fill('Дверь в убежище');
  await expect(deviceID).toHaveValue('device-dver-v-ubezhishche');
  await dialog.kind.selectOption('door');

  for (const name of ['Заперта', 'Открыта', 'Открыта!', '★★★', 'Ґанок', 'Café']) await addState(page, name);
  expect(await values(stateIDs)).toEqual(['zaperta', 'otkryta', 'otkryta-2', 'state-1', 'ganok', 'cafe']);
  await expect(dialog.technical.locator('.facility-state-row label'))
    .toHaveText(['Заперта', 'Открыта', 'Открыта!', '★★★', 'Ґанок', 'Café']);

  // The matrix cell suggests the door's «Открыть»; the identifier follows the typed name.
  await openTransitionCell(page, 'zaperta', 'otkryta');
  const transitionName = page.locator('#facilityTransitionPanel .facility-transition-row:not([hidden]) .facility-transition-name');
  await expect(transitionName).toHaveValue('Открыть');
  expect(await values(transitionIDs)).toEqual(['otkryt']);
  await transitionName.fill('Открыть дверь');
  expect(await values(transitionIDs)).toEqual(['otkryt-dver']);
  await expect(dialog.technical.locator('.facility-transition-id-row label')).toHaveText(['Открыть дверь: Заперта → Открыта']);
  // The first state is the starting state without any list.
  await expect(dialog.chip('zaperta')).toHaveAttribute('data-initial', 'true');
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expect.poll(async () => (await facilityState(request)).facility.devices.length).toBe(1);
  await expectSaved(page);

  const saved = (await facilityState(request)).facility.devices[0];
  expect(saved.id).toBe('device-dver-v-ubezhishche');
  expect(saved.initialStateId).toBe('zaperta');
  expect(saved.states.map(value => value.id)).toEqual(['zaperta', 'otkryta', 'otkryta-2', 'state-1', 'ganok', 'cafe']);
  expect(saved.transitions.map(value => value.id)).toEqual(['otkryt-dver']);

  // Renaming a saved device, state, or transition keeps every saved identifier, read-only.
  await page.getByRole('button', { name: 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', exact: true }).click();
  await expect(dialog.dialog).toBeVisible();
  await expect(deviceID).toBeHidden();
  await dialog.name.fill('Гермодверь');
  await dialog.chip('zaperta').click();
  await dialog.stateName.fill('Закрыта');
  await openTransitionCell(page, 'zaperta', 'otkryta');
  await transitionName.fill('Распахнуть дверь');
  await unfold(dialog.technical);
  await expect(deviceID).toHaveValue('device-dver-v-ubezhishche');
  await expect(stateIDs.first()).toHaveValue('zaperta');
  await expect(transitionIDs.first()).toHaveValue('otkryt-dver');
  for (const input of [deviceID, stateIDs.first(), transitionIDs.first()]) {
    await expect(input).toHaveAttribute('readonly', '');
  }
  await expect(dialog.technical.locator('.facility-state-row label').first()).toHaveText('Закрыта');
  await expect(dialog.technical.locator('.facility-transition-id-row label')).toHaveText(['Распахнуть дверь: Закрыта → Открыта']);
  await dialog.cancel.click();
  // Cancel discards the dialog input; the saved identifiers and names stay.
  expect((await facilityState(request)).facility.devices[0]).toEqual(saved);

  await page.locator('#facilityTabConditions').click();
  await page.locator('#btnAddFacilityCondition').click();
  const condition = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await expect(condition).toBeVisible();
  await condition.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Отказ привода');
  const conditionID = condition.locator('#facilityConditionID');
  await expect(conditionID).toBeHidden();
  await unfold(condition.locator('#facilityConditionTechnical'));
  await expect(conditionID).toHaveValue('condition-otkaz-privoda');
  await condition.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();

  await page.locator('#facilityTabPrograms').click();
  await page.locator('#btnAddFacilityRecoveryProgram').click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await expect(program).toBeVisible();
  await program.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Перезапуск контроллера');
  const programID = program.locator('#facilityRecoveryProgramID');
  await expect(programID).toBeHidden();
  await unfold(program.locator('#facilityRecoveryProgramTechnical'));
  await expect(programID).toHaveValue('program-perezapusk-kontrollera');
});

test('invalid states get their own messages, and an identifier conflict unfolds the block and focuses its row (FR-007)', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  await openOverseer(page);
  const dialog = await openNewDevice(page);
  await dialog.name.fill('Шлюз');
  await dialog.kind.selectOption('door');
  for (const name of ['Открыт', 'Закрыт', 'Аварийный', ' открыт ']) await addState(page, name);
  const rows = dialog.technical.locator('.facility-state-row');
  await dialog.chips.nth(1).click();
  await dialog.stateName.fill('');
  await unfold(dialog.technical);
  await rows.nth(2).locator('.facility-state-id').fill('otkryt');
  await fold(dialog.technical);

  await dialog.save.click();
  await expect(dialog.dialog).toBeVisible();
  await expect(dialog.error).toHaveText('ИСПРАВЬТЕ ОТМЕЧЕННОЕ: 3');
  // The first problem, the empty name, opens its chip's editor and takes focus.
  await expect(dialog.chips.nth(1)).toHaveAttribute('aria-expanded', 'true');
  await expect(dialog.stateName).toBeFocused();
  await expect(dialog.stateName).toHaveAttribute('aria-invalid', 'true');
  await expect(dialog.stateEditor.locator('.facility-row-error')).toHaveText('УКАЖИТЕ НАЗВАНИЕ СОСТОЯНИЯ');
  expect(await dialog.chips.evaluateAll(chips => chips.map(chip => chip.dataset.problem === 'true')))
    .toEqual([false, true, true, true]);
  await expect(rows.nth(0).locator('.facility-row-error')).toHaveCount(0);
  await expect(rows.nth(2).locator('.facility-row-error')).toHaveText('ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: otkryt');
  await expect(rows.nth(2).locator('.facility-state-id')).toHaveAttribute('aria-invalid', 'true');
  await dialog.chips.nth(3).click();
  await expect(dialog.stateEditor.locator('.facility-row-error')).toHaveText('НАЗВАНИЕ УЖЕ ИСПОЛЬЗУЕТСЯ: открыт');

  await dialog.stateName.fill('Заблокирован');
  await expect(dialog.stateEditor.locator('.facility-row-error')).toHaveCount(0);
  await dialog.chips.nth(1).click();
  await dialog.stateName.fill('Закрыт');
  await expect(dialog.stateEditor.locator('.facility-row-error')).toHaveCount(0);

  // With only the identifier conflict left, «ГОТОВО» unfolds the block and focuses that row.
  await fold(dialog.technical);
  await dialog.save.click();
  await expect(dialog.error).toHaveText('ИСПРАВЬТЕ ОТМЕЧЕННОЕ: 1');
  await expect(dialog.technical).toHaveAttribute('open', '');
  await expect(rows.nth(2).locator('.facility-state-id')).toBeFocused();
  await expect(rows.nth(2).locator('.facility-state-id')).toHaveAttribute('aria-invalid', 'true');
  await expect(rows.nth(2).locator('.facility-row-error')).toHaveText('ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: otkryt');
  await expect(dialog.chips.nth(2)).toHaveAttribute('data-problem', 'true');

  await rows.nth(2).locator('.facility-state-id').fill('avariynyy');
  await expect(rows.nth(2).locator('.facility-row-error')).toHaveCount(0);
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expect.poll(async () => (await facilityState(request)).facility.devices.map(device => device.name))
    .toEqual(['Шлюз']);
  await expectSaved(page);
  const saved = (await facilityState(request)).facility.devices[0];
  expect(saved.states.map(value => [value.id, value.name]))
    .toEqual([['otkryt', 'Открыт'], ['zakryt', 'Закрыт'], ['avariynyy', 'Аварийный'], ['zablokirovan', 'Заблокирован']]);
  expect(saved.initialStateId).toBe('otkryt');
});
