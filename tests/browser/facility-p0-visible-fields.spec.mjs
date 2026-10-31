import { expect, test } from '@playwright/test';

import { switchTab } from './helpers/authoring-bar.mjs';
import { RULE_TABS, chooseWhere } from './helpers/facility-editors.mjs';
import { expectNoOrphanLabels, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

// A record, an ordinary command, and a command with an action, each picked
// on every rule tab that offers it.
const RULE_NODES = ['n_entry_vault_shutdown', 'n_cmd_ordinary_1', 'n_cmd_open_evacuation'];
// Opened above through their real routes, where their lists and modes are populated.
const NATURALLY_COVERED = new Set([
  'facilityBindingDialog', 'facilityCommandActionDialog', 'facilityConditionDialog', 'facilityDeviceDialog',
  'facilityRecoveryProgramDialog', 'facilityPreviewDialog', 'facilityDependencyDialog', 'facilityRepairDialog',
  'facilityDeviceResetDialog', 'sceneResetDialog', 'createTerminalDialog', 'logicalSessionDialog',
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

async function chooseCommand(page, terminalId, nodeId) {
  const option = page.locator(
    `#facilityCommandActionCommand option[data-terminal-id="${terminalId}"][data-node-id="${nodeId}"]`);
  await page.locator('#facilityCommandActionCommand').selectOption(await option.getAttribute('value'));
}

test('facility dialogs show no orphan labels on any tab, scope, or mode (FR-023, SC-005)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  await page.locator('[data-facility-kind="device"][data-facility-id="vault-door-main"]').click();

  await openDetailTab(page, 'facilityTabInformation');
  await within(page, () => page.locator('#btnAddFacilityBinding').click(), 'facilityBindingDialog', async dialog => {
    for (const [kind, tabID] of Object.entries(RULE_TABS)) {
      await dialog.locator(tabID).click();
      await expect(dialog.locator(tabID)).toHaveAttribute('aria-selected', 'true');
      await expectNoOrphanLabels(dialog, `rule ${kind} without element`);
      for (const node of RULE_NODES) {
        const offered = dialog.locator(`#facilityBindingNode option[data-terminal-id="t_demo1"][data-node-id="${node}"]`);
        if (!await offered.count()) continue;
        await chooseWhere(page, 't_demo1', node);
        if (kind === 'entry-content') await dialog.locator('#facilityBindingBlock').selectOption({ index: 1 });
        await expectNoOrphanLabels(dialog, `rule ${kind} ${node}`);
      }
    }
  }, 'btnCancelFacilityBinding');
  // An existing rule keeps its tab, and its blocked tabs carry no orphan reason.
  await page.locator('[data-facility-kind="binding"][data-facility-id="t_demo1/b_vault_door_status/entry-content/0"]')
    .getByRole('button', { name: /^Изменить/ }).click();
  await expect(page.locator('#facilityBindingDialog')).toBeVisible();
  await expectNoOrphanLabels(page.locator('#facilityBindingDialog'), 'existing rule');
  await page.locator('#btnCancelFacilityBinding').click();

  await openDetailTab(page, 'facilityTabActions');
  await within(page, () => page.locator('#btnAddFacilityActionBinding').click(), 'facilityCommandActionDialog', async dialog => {
    await expectNoOrphanLabels(dialog, 'command action with a picked command');
    await chooseCommand(page, 't_demo1', 'n_cmd_ordinary_1');
    await expect(dialog.locator('#facilityCommandActionCompletedName')).toBeVisible();
    await expectNoOrphanLabels(dialog, 'command action without stateChange');
    await dialog.locator('#facilityCommandActionModeProgram').check();
    await expect(dialog.locator('#facilityCommandActionProgram')).toBeVisible();
    await expectNoOrphanLabels(dialog, 'command action in program mode');
    await dialog.locator('#facilityCommandActionModeTransitions').check();
    await dialog.locator('#btnAddFacilityCommandActionTransition').click();
    await expectNoOrphanLabels(dialog, 'command action with two transitions');
  }, 'btnCancelFacilityCommandAction');
  await within(page, () => page.locator('#facilityActionBindingList [role="listitem"]').first()
    .getByRole('button', { name: /^Изменить/ }).click(), 'facilityCommandActionDialog', async dialog => {
    await expect(dialog.locator('#facilityCommandActionCommandFixed')).toBeVisible();
    await expectNoOrphanLabels(dialog, 'command action with a fixed command');
  }, 'btnCancelFacilityCommandAction');

  await openDetailTab(page, 'facilityTabConditions');
  await within(page, () => page.locator('#btnAddFacilityCondition').click(), 'facilityConditionDialog', async dialog => {
    await expectNoOrphanLabels(dialog, 'condition device scope');
    await dialog.locator('#btnAddFaultActivation').click();
    await expect(dialog.locator('#facilityFaultActivationPicker')).toBeVisible();
    await expectNoOrphanLabels(dialog, 'condition with the activation picker');
    await dialog.locator('#facilityConditionScope').selectOption('terminal');
    await expectNoOrphanLabels(dialog, 'condition terminal scope');
  }, 'btnCancelFacilityCondition');

  await within(page, () => page.locator('#btnAddFacilityDevice').click(), 'facilityDeviceDialog', async dialog => {
    // With fewer than two states the matrix holds only its hint, so the
    // check waits for a second state, whose editor stays open.
    await dialog.locator('#btnAddFacilityState').click();
    await dialog.locator('#btnAddFacilityState').click();
    await expect(dialog.locator('#facilityStateEditor')).toBeVisible();
    await expectNoOrphanLabels(dialog, 'new device with the state editor');
    await dialog.locator('#facilityTransitionMatrix button.facility-transition-cell').first().click();
    await expect(dialog.locator('#facilityTransitionPanel')).toBeVisible();
    await dialog.locator('#facilityTransitionPanel .facility-transition-row:not([hidden]) .facility-add-precondition').click();
    await expectNoOrphanLabels(dialog, 'new device with a transition');
  }, 'btnCancelFacilityDevice');

  await within(page, () => page.locator('#btnEditFacilityDevice').click(),
    'facilityDeviceDialog', async dialog => {
      await dialog.locator('#facilityTransitionMatrix button.facility-transition-cell[data-from="sealed"][data-to="open"]').click();
      await expect(dialog.locator('#facilityTransitionPanel')).toBeVisible();
      await dialog.locator('#facilityDeviceTechnical > summary').click();
      await expectNoOrphanLabels(dialog, 'existing device');
    }, 'btnCancelFacilityDevice');

  await openDetailTab(page, 'facilityTabPrograms');
  await within(page, () => page.locator('#btnAddFacilityRecoveryProgram').click(), 'facilityRecoveryProgramDialog', async dialog => {
    await expectNoOrphanLabels(dialog, 'program');
    await dialog.locator('#btnAddFacilityProgramTransition').click();
    await expectNoOrphanLabels(dialog, 'program with a second transition');
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

  // A fault row opens its dialog, and the fault preview opens over it.
  await page.locator('[data-facility-kind="device"][data-facility-id="ventilation-main"]').click();
  await openDetailTab(page, 'facilityTabConditions');
  await within(page, () => page.locator('#facilityConditionList').getByRole('button', { name: /Споровое загрязнение/ }).click(),
    'facilityConditionDialog', async dialog => {
      await expectNoOrphanLabels(dialog, 'existing condition');
      await within(page, () => dialog.locator('#btnPreviewFacilityCondition').click(), 'facilityPreviewDialog', async preview => {
        await expect(preview.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
        await expectNoOrphanLabels(preview, 'condition preview');
      }, 'btnCloseFacilityPreview');
      await expect(dialog).toBeVisible();
    }, 'btnCancelFacilityCondition');
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
