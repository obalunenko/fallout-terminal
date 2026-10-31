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

// openSporeCondition follows the ventilation loop's scope link to its fault
// dialog (FR-019, FR-037).
async function openSporeCondition(page) {
  await selectDevice(page, 'ventilation-main');
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  const report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ УСТРОЙСТВА', exact: true });
  const link = report.locator('.facility-dependency-row[data-kind="diagnostic-scope"]')
    .filter({ hasText: `Неисправность «${SPORES}» действует на это устройство.` });
  await expect(link).toHaveAttribute('data-destination', 'condition');
  await link.getByRole('button', { name: `К НЕИСПРАВНОСТИ: ${SPORES}`, exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await expect(dialog.locator('#facilityConditionName')).toHaveValue(SPORES);
  return dialog;
}

test('a new fault can start the game active and stays active after resets (FR-013, FR-020)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await selectDevice(page, 'vault-door-main');
  await openFaultsTab(page);
  await page.locator('#btnAddFacilityCondition').click();
  const dialog = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await expect(dialog.locator('#facilityConditionInitialWorking')).toBeChecked();
  // The Overseer clears any fault by hand, so the dialog has no operator-recovery checkbox.
  await expect(dialog.locator('#facilityConditionPrivateRecovery')).toHaveCount(0);
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Привод заклинило');
  await dialog.getByLabel('КАТЕГОРИЯ', { exact: true }).selectOption('offline');
  await dialog.getByLabel('ЧТО БЛОКИРУЕТ', { exact: true }).selectOption('execute-command');
  await dialog.locator('#facilityConditionInitialActive').check();
  await expect(dialog.locator('#facilityFaultAppearsInitial')).toHaveText('Активна с начала игры.');
  await dialog.getByRole('button', { name: DONE, exact: true }).click();
  await expect(dialog).toBeHidden();
  const row = page.locator('#facilityConditionList').getByRole('button', { name: /Привод заклинило/ });
  await expect(row).toContainText('СЕЙЧАС: АКТИВНА · В НАЧАЛЕ: АКТИВНА');

  await expect.poll(() => conditionByName(request, 'Привод заклинило'))
    .toMatchObject({ initialActive: true, currentActive: true });
  await expectSaved(page);
  expect((await conditionByName(request, 'Привод заклинило')).recovery).toBeUndefined();

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

test('changing an existing fault starting activity keeps its current activity (FR-013)', async ({ page, request }) => {
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
  // The row opens the fault dialog, which shows the new starting position (FR-019).
  await row.click();
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#facilityConditionInitialWorking')).toBeChecked();
  await expect(dialog.locator('#facilityFaultAppearsInitial')).toBeHidden();
  await dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
  await expect(dialog).toBeHidden();
});

test('a saved fault without a starting activity opens as working at game start (FR-013)', async ({ page, request }) => {
  await resetFixture(request);
  const demo = await demoSession();
  delete demo.facility.conditions.find(condition => condition.name === SPORES).initialActive;
  await seedSession(request, demo);
  await openOverseer(page);
  const dialog = await openSporeCondition(page);
  await expect(dialog.locator('#facilityConditionInitialWorking')).toBeChecked();
});
