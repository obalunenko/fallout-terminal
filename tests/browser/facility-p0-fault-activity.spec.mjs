import { expect, test } from '@playwright/test';

import {
  demoSession, facilityState, openOverseer, resetFixture, seedDemo, seedSession,
} from './helpers/overseer-forms.mjs';

const APPLY = 'ПРИМЕНИТЬ К ЧЕРНОВИКУ';
const SAVE = 'СОХРАНИТЬ ИЗМЕНЕНИЯ СЕССИИ';
const SPORES = 'Споровое загрязнение воздушного контура';

const conditionByName = async (request, name) =>
  (await facilityState(request)).facility.conditions.find(condition => condition.name === name);

async function selectDevice(page, id) {
  await page.locator(`[data-facility-kind="device"][data-facility-id="${id}"]`).click();
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
  await page.locator('#btnAddFacilityCondition').click();
  const dialog = page.getByRole('dialog', { name: 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', exact: true });
  await expect(dialog.locator('#facilityConditionInitialWorking')).toBeChecked();
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Привод заклинило');
  await dialog.getByLabel('КАТЕГОРИЯ', { exact: true }).selectOption('offline');
  await dialog.getByLabel('БЛОКИРУЕМАЯ ВОЗМОЖНОСТЬ', { exact: true }).selectOption('execute-command');
  await dialog.locator('#facilityConditionPrivateRecovery').check();
  await dialog.locator('#facilityConditionInitialActive').check();
  await dialog.getByRole('button', { name: APPLY, exact: true }).click();
  await expect(dialog).toBeHidden();
  const row = page.locator('#facilityConditionList').getByRole('button', { name: /Привод заклинило/ });
  await expect(row).toContainText('СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: АКТИВНА');

  await page.getByRole('button', { name: SAVE, exact: true }).click();
  await expect(page.locator('#facilityStatus')).toHaveText('ИЗМЕНЕНИЯ СЕССИИ СОХРАНЕНЫ');
  expect(await conditionByName(request, 'Привод заклинило')).toMatchObject({ initialActive: true, currentActive: true });

  await page.locator('#btnResetFacilityDevice').click();
  await page.locator('#btnConfirmFacilityDeviceReset').click();
  await expect(page.locator('#facilityStatus')).toHaveText('УСТРОЙСТВО СБРОШЕНО');
  expect(await conditionByName(request, 'Привод заклинило')).toMatchObject({ initialActive: true, currentActive: true });

  await page.locator('#btnResetFacility').click();
  await page.locator('#btnConfirmFacilityReset').click();
  await expect(page.locator('#facilityStatus')).toHaveText('ОБЪЕКТ СБРОШЕН');
  expect(await conditionByName(request, 'Привод заклинило')).toMatchObject({ initialActive: true, currentActive: true });
});

test('changing an existing fault starting activity keeps its current activity', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  const dialog = await openSporeCondition(page);
  await expect(dialog.locator('#facilityConditionInitialActive')).toBeChecked();
  await dialog.locator('#facilityConditionInitialWorking').check();
  await dialog.getByRole('button', { name: APPLY, exact: true }).click();
  const row = page.locator('#facilityConditionList').getByRole('button', { name: new RegExp(SPORES) });
  await expect(row).toContainText('СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: ИСПРАВНО');
  await page.getByRole('button', { name: SAVE, exact: true }).click();
  await expect(page.locator('#facilityStatus')).toHaveText('ИЗМЕНЕНИЯ СЕССИИ СОХРАНЕНЫ');
  expect(await conditionByName(request, SPORES)).toMatchObject({ initialActive: false, currentActive: true });
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
