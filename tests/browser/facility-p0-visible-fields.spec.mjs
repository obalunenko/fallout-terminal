import { expect, test } from '@playwright/test';

import { switchTab } from './helpers/authoring-bar.mjs';
import { expectNoOrphanLabels, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

const BINDING_KINDS = ['menu-name', 'entry-content', 'visibility', 'availability', 'command-action'];
const BINDING_NODES = ['n_entry_vault_shutdown', 'n_cmd_ordinary_1', 'n_cmd_open_evacuation'];
// Opened above through their real routes, where their lists and modes are populated.
const NATURALLY_COVERED = new Set([
  'facilityBindingDialog', 'facilityConditionDialog', 'facilityDeviceDialog', 'facilityRecoveryProgramDialog',
  'facilityPreviewDialog', 'facilityDependencyDialog', 'facilityRepairDialog', 'facilityDeviceResetDialog',
  'sceneResetDialog', 'facilityRecoveryConfirmationDialog', 'createTerminalDialog', 'logicalSessionDialog',
  'terminalGroupDraftDialog', 'entryBlockCommandDialog', 'resetConfirmationDialog',
]);

// openDetailTab opens the ОБЪЕКТЫ detail tab that holds a list or its add control.
async function openDetailTab(page, id) {
  await page.locator(`#${id}`).click();
  await expect(page.locator(`#${id}`)).toHaveAttribute('aria-selected', 'true');
}

async function within(page, opener, dialogID, check, closeID) {
  await opener();
  const dialog = page.locator(`#${dialogID}`);
  await expect(dialog).toBeVisible();
  await check(dialog);
  await page.locator(`#${closeID}`).click();
  await expect(dialog).toBeHidden();
}

test('facility dialogs show no orphan labels in any kind, scope, or mode', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await page.locator('[data-facility-kind="device"][data-facility-id="vault-door-main"]').click();

  await openDetailTab(page, 'facilityTabInformation');
  await within(page, () => page.locator('#btnAddFacilityBinding').click(), 'facilityBindingDialog', async dialog => {
    await dialog.locator('#facilityBindingTerminal').selectOption('t_demo1');
    for (const kind of BINDING_KINDS) {
      await dialog.locator('#facilityBindingKind').selectOption(kind);
      for (const node of BINDING_NODES) {
        await dialog.locator('#facilityBindingNode').selectOption(node);
        await expectNoOrphanLabels(dialog, `binding ${kind} ${node}`);
      }
    }
  }, 'btnCancelFacilityBinding');

  await openDetailTab(page, 'facilityTabConditions');
  await within(page, () => page.locator('#btnAddFacilityCondition').click(), 'facilityConditionDialog', async dialog => {
    await expectNoOrphanLabels(dialog, 'condition device scope');
    await dialog.locator('#facilityConditionScope').selectOption('terminal');
    await expectNoOrphanLabels(dialog, 'condition terminal scope');
  }, 'btnCancelFacilityCondition');

  await within(page, () => page.locator('#btnAddFacilityDevice').click(), 'facilityDeviceDialog', async dialog => {
    await dialog.getByRole('button', { name: 'ДОБАВИТЬ СОСТОЯНИЕ', exact: true }).click();
    await dialog.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД', exact: true }).click();
    await expectNoOrphanLabels(dialog, 'new device');
  }, 'btnCancelFacilityDevice');

  await within(page, () => page.locator('#btnEditFacilityDevice').click(),
    'facilityDeviceDialog', async dialog => {
      for (const summary of await dialog.locator('details.facility-transition-references:not([open]) > summary').all()) {
        await summary.click();
      }
      await expectNoOrphanLabels(dialog, 'existing device');
    }, 'btnCancelFacilityDevice');

  await openDetailTab(page, 'facilityTabPrograms');
  await within(page, () => page.locator('#btnAddFacilityRecoveryProgram').click(), 'facilityRecoveryProgramDialog', async dialog => {
    await expectNoOrphanLabels(dialog, 'program');
    await dialog.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД ПРОГРАММЫ', exact: true }).click();
    await expectNoOrphanLabels(dialog, 'program with extra transition');
  }, 'btnCancelFacilityRecoveryProgram');

  await within(page, () => page.locator('#btnPreviewFacilityDevice').click(), 'facilityPreviewDialog', async dialog => {
    await expect(dialog.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
    await expectNoOrphanLabels(dialog, 'device preview');
  }, 'btnCloseFacilityPreview');

  await within(page, () => page.locator('#btnInspectFacilityDependencies').click(), 'facilityDependencyDialog',
    dialog => expectNoOrphanLabels(dialog, 'dependencies'), 'btnCloseFacilityDependencies');
  await within(page, async () => {
    await page.locator('#btnFacilityDeviceMenu').click();
    await expectNoOrphanLabels(page.locator('#facilityDeviceActions'), 'device actions with the menu open');
    await page.locator('#btnDeleteFacilityDevice').click();
  }, 'facilityRepairDialog', dialog => expectNoOrphanLabels(dialog, 'repair'), 'btnCancelFacilityRepair');
  await within(page, () => page.locator('#btnResetFacilityDevice').click(), 'facilityDeviceResetDialog',
    dialog => expectNoOrphanLabels(dialog, 'device reset'), 'btnCancelFacilityDeviceReset');
  await within(page, () => page.locator('#btnResetScene').click(), 'sceneResetDialog', async dialog => {
    await expect(dialog.locator('#sceneResetCounts li')).toHaveCount(3);
    await expectNoOrphanLabels(dialog, 'scene reset');
  }, 'btnCancelSceneReset');

  await page.locator('[data-facility-kind="device"][data-facility-id="ventilation-main"]').click();
  await openDetailTab(page, 'facilityTabConditions');
  await page.locator('#facilityConditionList').getByRole('button', { name: /Споровое загрязнение/ }).click();
  await within(page, () => page.locator('#btnPreviewFacilityCondition').click(), 'facilityPreviewDialog', async dialog => {
    await expect(dialog.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
    await expectNoOrphanLabels(dialog, 'condition preview');
  }, 'btnCloseFacilityPreview');
  await within(page, () => page.locator('#btnRecoverFacilityCondition').click(), 'facilityRecoveryConfirmationDialog',
    dialog => expectNoOrphanLabels(dialog, 'recovery confirmation'), 'btnCancelFacilityRecovery');
});

test('the shared authoring bar shows no orphan labels on either tab and keeps no removed control', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  const bar = page.locator('#authoringBar');
  for (const id of ['btnSaveFacility', 'btnDiscardFacility', 'btnResetFacility', 'facilityResetDialog',
    'commandDraftGuard', 'btnPublish']) {
    await expect(page.locator(`#${id}`), id).toHaveCount(0);
  }
  await expect(bar.getByRole('button')).toHaveText(['ОТМЕНИТЬ', 'ПОВТОРИТЬ', 'СБРОСИТЬ СЦЕНУ']);
  await expectNoOrphanLabels(bar, 'shared bar on ТЕРМИНАЛЫ');

  // Unapplied input adds its name to the status and an ОТМЕНА next to its form.
  await page.locator('#introTextArea').fill('Неприменённый текст терминала');
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-unapplied-count', '1');
  await expectNoOrphanLabels(bar, 'shared bar with unapplied input');
  await expectNoOrphanLabels(page.locator('#termSettings'), 'terminal settings with unapplied input');

  await switchTab(page, 'ОБЪЕКТЫ');
  await expect(bar).toBeVisible();
  await expectNoOrphanLabels(bar, 'shared bar on ОБЪЕКТЫ');
  // The selected device has no name field of its own: its header, play card,
  // and grouped actions carry no orphan label.
  await page.locator('[data-facility-kind="device"][data-facility-id="vault-door-main"]').click();
  for (const id of ['facilitySelectedDeviceName', 'btnSaveFacilityDeviceChanges', 'btnCancelFacilityDeviceChanges']) {
    await expect(page.locator(`#${id}`), id).toHaveCount(0);
  }
  await expect(page.locator('#btnEditFacilityDevice')).toBeVisible();
  await expect(page.locator('#facilityPlayCard #btnResetFacilityDevice')).toBeVisible();
  await expectNoOrphanLabels(page.locator('#facilitySelectionEditor'), 'selected device');
});

test('terminal workspace dialogs show no orphan labels', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await within(page, () => page.locator('#btnAddTerminal').click(), 'createTerminalDialog',
    dialog => expectNoOrphanLabels(dialog, 'create terminal'), 'btnCancelCreateTerminal');
  await within(page, () => page.locator('#btnManageLogicalSessions').click(), 'logicalSessionDialog',
    dialog => expectNoOrphanLabels(dialog, 'logical sessions'), 'btnCloseLogicalSessions');

  await page.locator('#btnCreateTerminalGroup').click();
  const group = page.locator('#terminalGroupDraftDialog');
  await expect(group).toBeVisible();
  await expectNoOrphanLabels(group, 'terminal group draft');
  await group.locator('[data-action="cancel-terminal-group-draft"]').click();
  await expect(group).toBeHidden();

  await page.locator('.tree-row', { hasText: 'ДИРЕКТИВЫ VAULT-TEC' }).click();
  await page.locator('.tree-row', { hasText: 'СВОДКА ЭВАКУАЦИИ' }).click();
  // Block 4 has no owning command, so both configuration modes are editable.
  await page.locator('[data-entry-block-action="configure-command"]').nth(3).click();
  const blockCommand = page.locator('#entryBlockCommandDialog');
  await expect(blockCommand).toBeVisible();
  await expectNoOrphanLabels(blockCommand, 'block command existing');
  await blockCommand.locator('input[name="entryBlockCommandMode"][value="new"]').check();
  await expectNoOrphanLabels(blockCommand, 'block command new');
  await blockCommand.locator('[data-entry-block-command-action="cancel"]').click();
  await expect(blockCommand).toBeHidden();

  await page.locator('.tree-row', { hasText: 'ПРОТОКОЛ ВЫХОДА' }).click();
  await page.locator('.tree-row', { hasText: 'Маршрут выхода активирован' }).click();
  await within(page, () => page.locator('#btnResetCommandState').click(), 'resetConfirmationDialog',
    dialog => expectNoOrphanLabels(dialog, 'command state reset'), 'btnCancelCommandStateReset');
});

test('every other Overseer dialog shows no orphan labels in its default state', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  const all = await page.locator('dialog').evaluateAll(dialogs => dialogs.map(dialog => dialog.id));
  expect(all.length).toBeGreaterThanOrEqual(27);
  const ids = all.filter(id => !NATURALLY_COVERED.has(id));
  expect(ids).toHaveLength(all.length - NATURALLY_COVERED.size);
  for (const id of ids) {
    const dialog = page.locator(`#${id}`);
    await dialog.evaluate(element => { element.hidden = false; element.showModal(); });
    await expectNoOrphanLabels(dialog, `${id} default state`);
    await dialog.evaluate(element => { element.close(); element.hidden = true; });
  }
});
