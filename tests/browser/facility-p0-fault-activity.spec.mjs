import { expect, test } from '@playwright/test';

import { expectSaved } from './helpers/authoring-bar.mjs';
import {
  demoSession, facilityState, openOverseer, resetFixture, seedDemo, seedSession,
} from './helpers/overseer-forms.mjs';

const DONE = 'ГОТОВО';
const SPORES = 'Споровое загрязнение воздушного контура';

const conditionByName = async (request, name) =>
  (await facilityState(request)).facility.conditions.find(condition => condition.name === name);

async function selectDevice(page, id) {
  await page.locator(`[data-facility-kind="device"][data-facility-id="${id}"]`).click();
}

// Faults live on the «НЕИСПРАВНОСТИ» tab of the selected device.
async function openFaultsTab(page) {
  await page.locator('#facilityTabConditions').click();
}

async function openSporeCondition(page) {
  await selectDevice(page, 'ventilation-main');
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  const report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ ОБЪЕКТА', exact: true });
  await report.locator('.facility-dependency-row').filter({ hasText: 'Область неисправности' })
    .getByRole('button', { name: /^Открыть ссылку/ }).click();
  const dialog = page.getByRole('dialog', { name: 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', exact: true });
  await expect(dialog.locator('#facilityConditionName')).toHaveValue(SPORES);
  return dialog;
}

test('a new fault can start the game active and stays active after resets', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await selectDevice(page, 'vault-door-main');
  await openFaultsTab(page);
  await page.locator('#btnAddFacilityCondition').click();
  const dialog = page.getByRole('dialog', { name: 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', exact: true });
  await expect(dialog.locator('#facilityConditionInitialWorking')).toBeChecked();
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Привод заклинило');
  await dialog.getByLabel('КАТЕГОРИЯ', { exact: true }).selectOption('offline');
  await dialog.getByLabel('БЛОКИРУЕМАЯ ВОЗМОЖНОСТЬ', { exact: true }).selectOption('execute-command');
  await dialog.locator('#facilityConditionPrivateRecovery').check();
  await dialog.locator('#facilityConditionInitialActive').check();
  await dialog.getByRole('button', { name: DONE, exact: true }).click();
  await expect(dialog).toBeHidden();
  const row = page.locator('#facilityConditionList').getByRole('button', { name: /Привод заклинило/ });
  await expect(row).toContainText('СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: АКТИВНА');

  await expect.poll(() => conditionByName(request, 'Привод заклинило'))
    .toMatchObject({ initialActive: true, currentActive: true });
  await expectSaved(page);

  // «СБРОСИТЬ К НАЧАЛЬНОМУ…» on the play card restores the starting value, which is active.
  await page.locator('#btnResetFacilityDevice').click();
  await page.locator('#btnConfirmFacilityDeviceReset').click();
  await expect(page.locator('#facilityPlayStatus')).toHaveText('УСТРОЙСТВО СБРОШЕНО');
  expect(await conditionByName(request, 'Привод заклинило')).toMatchObject({ initialActive: true, currentActive: true });

  // «СБРОСИТЬ СЦЕНУ» restores every starting value, so the fault stays active.
  await page.getByRole('button', { name: 'СБРОСИТЬ СЦЕНУ', exact: true }).click();
  const sceneReset = page.locator('#sceneResetDialog');
  await expect(sceneReset).toBeVisible();
  await sceneReset.locator('#btnConfirmSceneReset').click();
  await expect(sceneReset).toBeHidden();
  await expect(page.locator('#coordinationStatus')).toHaveText('СЦЕНА СБРОШЕНА');
  const reset = await facilityState(request);
  for (const device of reset.facility.devices) expect(device.currentStateId, device.id).toBe(device.initialStateId);
  for (const condition of reset.facility.conditions) {
    expect(Boolean(condition.currentActive), condition.id).toBe(Boolean(condition.initialActive));
  }
  expect(await conditionByName(request, 'Привод заклинило')).toMatchObject({ initialActive: true, currentActive: true });
  await expect(row).toContainText('СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: АКТИВНА');
});

test('changing an existing fault starting activity keeps its current activity', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  const dialog = await openSporeCondition(page);
  await expect(dialog.locator('#facilityConditionInitialActive')).toBeChecked();
  await dialog.locator('#facilityConditionInitialWorking').check();
  await dialog.getByRole('button', { name: DONE, exact: true }).click();
  await openFaultsTab(page);
  const row = page.locator('#facilityConditionList').getByRole('button', { name: new RegExp(SPORES) });
  await expect(row).toContainText('СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: ИСПРАВНО');
  await expect.poll(() => conditionByName(request, SPORES)).toMatchObject({ initialActive: false, currentActive: true });
  await expectSaved(page);
  await row.click();
  await expect(page.locator('#facilitySelectedConditionSummary')).toContainText('СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: ИСПРАВНО');
});

test('a saved fault without a starting activity opens as working at game start', async ({ page, request }) => {
  await resetFixture(request);
  const demo = await demoSession();
  delete demo.facility.conditions.find(condition => condition.name === SPORES).initialActive;
  await seedSession(request, demo);
  await openOverseer(page);
  const dialog = await openSporeCondition(page);
  await expect(dialog.locator('#facilityConditionInitialWorking')).toBeChecked();
});
