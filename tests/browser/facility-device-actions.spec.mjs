import { expect, test } from '@playwright/test';

import { expectSaved } from './helpers/authoring-bar.mjs';
import {
  forceState, openDemoObjects, playCard, selectDevice, setConditionActive,
} from './helpers/facility-play.mjs';
import { FIXTURE, facilityState, fixtureSession, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

// User Story 6: find device settings by role
// (specs/040-facility-play-controls/spec.md, contracts/overseer-ui.md).

const DOOR = 'vault-door-main';
const DOOR_NAME = 'Гермодверь Убежища 76';
const VENTILATION = 'ventilation-main';
const TABS = [
  ['actions', 'facilityTabActions', 'facilityTabPanelActions', '#facilityActionBindingList', 'ЧТО ДЕЛАЮТ ИГРОКИ'],
  ['information', 'facilityTabInformation', 'facilityTabPanelInformation', '#facilityInformationBindingList', 'ЧТО ВИДЯТ ИГРОКИ'],
  ['conditions', 'facilityTabConditions', 'facilityTabPanelConditions', '#facilityConditionList', 'НЕИСПРАВНОСТИ'],
  ['programs', 'facilityTabPrograms', 'facilityTabPanelPrograms', '#facilityRecoveryProgramList', 'ПРОГРАММЫ'],
];
const REMOVED_ELEMENTS = [
  'facilitySelectedDeviceName', 'btnSaveFacilityDeviceChanges', 'btnCancelFacilityDeviceChanges',
  'facilityDeviceSummary', 'facilitySelectionHeading', 'facilityBindingList',
];
const PLAY_CONTROLS = [
  '#btnForceFacilityDeviceState', '#btnResetFacilityDevice',
  'button.facility-play-transition-run', 'button.facility-play-fault-switch',
];

function pageErrors(page) {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  return errors;
}

function deviceRow(page, id) {
  return page.locator(`#facilityDeviceList button[data-facility-kind="device"][data-facility-id="${id}"]`);
}

function visitNodes(node, visit) {
  if (!node) return;
  visit(node);
  for (const child of node.children ?? []) visitNodes(child, visit);
}

// expectedDetails lists, per tab, the row IDs the session holds for deviceID,
// or for the whole session when deviceID is empty (FR-024).
function expectedDetails(session, deviceID = '') {
  const facility = session.facility;
  const uses = ids => !deviceID || ids.includes(deviceID);
  const details = { actions: [], information: [], conditions: [], programs: [] };
  for (const terminal of session.terminals) {
    visitNodes(terminal.root, node => {
      const shown = (property, owner, index, when) => {
        if (uses([when.deviceId])) details.information.push(`${terminal.id}/${owner}/${property}/${index}`);
      };
      for (const [index, variant] of (node.facilityNameVariants ?? []).entries()) shown('menu-name', node.id, index, variant.when);
      for (const block of node.blocks ?? []) {
        for (const [index, variant] of (block.facilityTextVariants ?? []).entries()) shown('entry-content', block.id, index, variant.when);
      }
      if (node.visibleWhen) shown('visibility', node.id, 0, node.visibleWhen);
      if (node.availableWhen) shown('availability', node.id, 0, node.availableWhen);
      const action = node.stateChange?.facilityAction;
      const requests = action?.transitions?.transitions ?? [];
      if (requests.length && uses(requests.map(request => request.deviceId))) {
        details.actions.push(`${terminal.id}/${node.id}/command-action/0`);
      }
      const program = facility.recoveryPrograms?.find(value => value.id === action?.recoveryProgramId);
      if (action?.recoveryProgramId && uses((program?.transitions ?? []).map(request => request.deviceId))) {
        details.actions.push(`${terminal.id}/${node.id}/recovery-action/0`);
      }
    });
  }
  details.conditions = facility.conditions.filter(value => !deviceID || value.device?.deviceId === deviceID).map(value => value.id);
  details.programs = facility.recoveryPrograms
    .filter(value => !deviceID || value.transitions?.some(request => request.deviceId === deviceID)).map(value => value.id);
  return details;
}

async function listedIDs(page, list) {
  return page.locator(`${list} > [data-facility-kind]`).evaluateAll(rows => rows.map(row => row.dataset.facilityId));
}

// expectTabs checks every tab's count and rows against the session, and that
// only the open tab's panel is shown.
async function expectTabs(page, details, open) {
  for (const [id, tabID, panelID, list, label] of TABS) {
    const tab = page.locator(`#${tabID}`);
    const count = details[id].length;
    await expect(page.getByRole('tab', { name: `${label} ${count}`, exact: true }), id).toBeVisible();
    await expect(tab, id).toHaveAttribute('data-count', String(count));
    await expect(tab.locator('.facility-tab-count'), id).toHaveText(String(count));
    await expect(tab, id).toHaveAttribute('aria-selected', String(id === open));
    await expect(tab, id).toHaveAttribute('tabindex', id === open ? '0' : '-1');
    await expect(tab, id).toHaveAttribute('aria-controls', panelID);
    if (id === open) await expect(page.locator(`#${panelID}`), id).toBeVisible();
    else await expect(page.locator(`#${panelID}`), id).toBeHidden();
    expect((await listedIDs(page, list)).sort(), id).toEqual([...details[id]].sort());
  }
}

async function openTab(page, id) {
  const [, tabID, panelID] = TABS.find(([tab]) => tab === id);
  await page.locator(`#${tabID}`).click();
  await expect(page.locator(`#${tabID}`)).toHaveAttribute('aria-selected', 'true');
  await expect(page.locator(`#${panelID}`)).toBeVisible();
}

// rowState is the device row text the session implies (FR-025).
function rowState(facility, device) {
  const faults = facility.conditions.filter(value => value.device?.deviceId === device.id && value.currentActive).length;
  const name = device.states.find(value => value.id === device.currentStateId).name;
  return { text: faults ? `${name} · ● ${faults}` : name, faults };
}

async function expectRow(page, id, text, faults) {
  const row = deviceRow(page, id);
  await expect(row.locator('.facility-device-row-state'), id).toHaveText(text);
  await expect(row, id).toHaveAttribute('data-active-faults', String(faults));
  if (faults) {
    await expect(row.locator('span.facility-fault-count'), id).toHaveText(`● ${faults}`);
    await expect(row.locator('span.facility-fault-count'), id).toHaveAttribute('aria-label', `Активных неисправностей: ${faults}`);
  } else {
    await expect(row.locator('.facility-fault-count'), id).toHaveCount(0);
  }
}

test('actions are grouped by role, and no destructive control sits outside «⋯» (AS1, FR-022, SC-007)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openDemoObjects(page, request);
  await selectDevice(page, DOOR);

  const actions = page.locator('#facilityDeviceActions');
  expect(await actions.locator(':scope > [role="group"]').evaluateAll(groups => groups.map(group => group.id)))
    .toEqual(['facilityDeviceInspectActions', 'facilityDeviceChangeActions']);
  const inspect = page.getByRole('group', { name: 'Проверка', exact: true });
  await expect(inspect).toHaveAttribute('id', 'facilityDeviceInspectActions');
  await expect(inspect.getByRole('button')).toHaveText(['ПРЕДПРОСМОТР СОСТОЯНИЯ', 'ЗАВИСИМОСТИ']);
  expect(await inspect.getByRole('button').evaluateAll(buttons => buttons.map(button => button.id)))
    .toEqual(['btnPreviewFacilityDevice', 'btnInspectFacilityDependencies']);

  const change = page.getByRole('group', { name: 'Изменение', exact: true });
  await expect(change).toHaveAttribute('id', 'facilityDeviceChangeActions');
  const edit = change.locator('#btnEditFacilityDevice');
  await expect(edit).toHaveText('ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ');
  await expect(edit).toHaveClass(/\bbtn-secondary\b/);
  await expect(edit).toHaveAttribute('aria-haspopup', 'dialog');
  await expect(edit).toHaveAttribute('aria-controls', 'facilityDeviceDialog');
  const more = change.locator('#btnFacilityDeviceMenu');
  await expect(more).toHaveText('⋯');
  await expect(more).toHaveAccessibleName('ДРУГИЕ ДЕЙСТВИЯ С УСТРОЙСТВОМ');
  await expect(more).toHaveAttribute('aria-haspopup', 'menu');
  await expect(more).toHaveAttribute('aria-expanded', 'false');
  await expect(more).toHaveAttribute('aria-controls', 'facilityDeviceMenu');
  const menu = page.locator('#facilityDeviceMenu');
  await expect(menu).toHaveAttribute('role', 'menu');
  await expect(menu).toBeHidden();
  expect(await menu.locator('[role="menuitem"]').evaluateAll(items => items.map(item => [item.id, item.textContent.trim()])))
    .toEqual([['btnDeleteFacilityDevice', 'УДАЛИТЬ УСТРОЙСТВО']]);

  // SC-007: the only destructive control of the selected device is in the menu.
  const destructive = await page.locator('#facilitySelectionEditor .btn-danger').evaluateAll(buttons => buttons
    .map(button => ({ id: button.id, inMenu: Boolean(button.closest('#facilityDeviceMenu')) })));
  expect(destructive).toEqual([{ id: 'btnDeleteFacilityDevice', inMenu: true }]);
  await expect(page.locator('#facilitySelectionEditor').getByRole('button', { name: /УДАЛИТЬ/ })).toHaveCount(0);
  await expect(page.locator('#facilitySelectionEditor').getByRole('menuitem', { name: /УДАЛИТЬ/ })).toHaveCount(0);

  // Play controls live only on the play card.
  const card = page.locator('#facilityPlayCard');
  for (const control of PLAY_CONTROLS) {
    const everywhere = await page.locator(control).count();
    expect(everywhere, control).toBeGreaterThan(0);
    await expect(card.locator(control), control).toHaveCount(everywhere);
  }
  for (const copy of [/СБРОСИТЬ К НАЧАЛЬНОМУ/, /ПРИНУДИТЕЛЬНО В ДРУГОЕ СОСТОЯНИЕ/]) {
    await expect(page.getByRole('button', { name: copy }), String(copy)).toHaveCount(1);
    await expect(card.getByRole('button', { name: copy }), String(copy)).toHaveCount(1);
  }
  await expect(card.getByRole('button', { name: /ПРЕДПРОСМОТР|ЗАВИСИМОСТИ|ИЗМЕНИТЬ СОСТОЯНИЯ/ })).toHaveCount(0);

  // The removed device form and its copy are gone.
  for (const id of REMOVED_ELEMENTS) await expect(page.locator(`#${id}`), id).toHaveCount(0);
  expect(await page.content()).not.toContain('РЕДАКТИРОВАТЬ ГРАФ');
  for (const source of [`${FIXTURE}/overseer`, '/__fixture/overseer.js']) {
    expect(await (await request.get(source)).text(), source).not.toContain('РЕДАКТИРОВАТЬ ГРАФ');
  }

  // Deleting takes two actions: «⋯», then «УДАЛИТЬ УСТРОЙСТВО».
  await more.click();
  await page.getByRole('menuitem', { name: 'УДАЛИТЬ УСТРОЙСТВО', exact: true }).click();
  await expect(page.locator('#facilityRepairDialog')).toBeVisible();
  await expect(menu).toBeHidden();
  await expect(more).toHaveAttribute('aria-expanded', 'false');
  await page.locator('#btnCancelFacilityRepair').click();
  await expect(page.locator('#facilityRepairDialog')).toBeHidden();
  expect((await fixtureSession(request)).facility.devices.some(device => device.id === DOOR)).toBe(true);
  expect(errors).toEqual([]);
});

test('the «⋯» menu opens from the pointer and keyboard and closes on Escape, Tab, an outside click, and a choice (FR-022)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openDemoObjects(page, request);
  await selectDevice(page, DOOR);
  const more = page.locator('#btnFacilityDeviceMenu');
  const menu = page.locator('#facilityDeviceMenu');
  const item = page.locator('#btnDeleteFacilityDevice');
  const expectOpen = async how => {
    await expect(menu, how).toBeVisible();
    await expect(more, how).toHaveAttribute('aria-expanded', 'true');
    await expect(item, how).toBeFocused();
  };
  const expectClosed = async how => {
    await expect(menu, how).toBeHidden();
    await expect(more, how).toHaveAttribute('aria-expanded', 'false');
  };

  // A click opens the menu on its first item; Escape returns to «⋯».
  await more.click();
  await expectOpen('click');
  await page.keyboard.press('Escape');
  await expectClosed('Escape');
  await expect(more).toBeFocused();
  await expect(page.locator('#facilityRepairDialog')).toBeHidden();

  for (const key of ['Enter', 'Space', 'ArrowDown']) {
    await more.focus();
    await page.keyboard.press(key);
    await expectOpen(key);
    await page.keyboard.press('Escape');
    await expectClosed(`${key} then Escape`);
    await expect(more).toBeFocused();
  }

  await more.press('Enter');
  await expectOpen('Enter before Tab');
  await page.keyboard.press('Tab');
  await expectClosed('Tab');

  await more.click();
  await expectOpen('click before an outside click');
  await page.locator('#facilityDeviceHeaderName').click();
  await expectClosed('outside click');

  // Choosing the item closes the menu and opens the repair dialog.
  await more.press('ArrowDown');
  await expectOpen('ArrowDown before a choice');
  await page.keyboard.press('Enter');
  await expectClosed('choice');
  await expect(page.locator('#facilityRepairDialog')).toBeVisible();
  await page.locator('#btnCancelFacilityRepair').click();
  await expect(page.locator('#facilityRepairDialog')).toBeHidden();
  expect(errors).toEqual([]);
});

test('the device is renamed in its dialog, and no name field sits under the header (AS2, FR-023)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openDemoObjects(page, request);
  await selectDevice(page, DOOR);
  const editor = page.locator('#facilitySelectionEditor');
  await expect(page.locator('#facilitySelectedDeviceName')).toHaveCount(0);
  await expect(editor.locator('input, textarea, select')).toHaveCount(0);
  await expect(editor.getByRole('button', { name: /^(ГОТОВО|ОТМЕНА)$/ })).toHaveCount(0);
  await expect(page.locator('#facilityDeviceHeaderName')).toHaveText(DOOR_NAME);
  const before = (await fixtureSession(request)).facility.devices.find(device => device.id === DOOR);

  await page.locator('#btnEditFacilityDevice').click();
  const dialog = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
  await expect(dialog).toBeVisible();
  const name = dialog.locator('#facilityDeviceName');
  await expect(name).toBeFocused();
  await expect(name).toHaveValue(DOOR_NAME);
  await expect(dialog.locator('#facilityDeviceID')).toHaveValue(DOOR);
  await name.fill('Гермодверь 76');
  // Dialog input is not tracked as unapplied device input any more.
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-unapplied-count', '0');
  await dialog.getByRole('button', { name: 'ГОТОВО', exact: true }).click();
  await expect(dialog).toBeHidden();
  await expectSaved(page);

  await expect(page.locator('#facilityDeviceHeaderName')).toHaveText('Гермодверь 76');
  await expect(deviceRow(page, DOOR).locator('strong')).toHaveText('Гермодверь 76');
  await expect(page.locator('#btnUndoReason')).toHaveText('ОТМЕНИТЬ: Устройство «Гермодверь 76»');
  // The rename keeps the device's identity, graph, and live state.
  const saved = (await fixtureSession(request)).facility.devices.find(device => device.id === DOOR);
  expect(saved).toMatchObject({
    id: DOOR, name: 'Гермодверь 76', kind: before.kind, initialStateId: before.initialStateId,
    currentStateId: before.currentStateId, states: before.states,
  });
  expect(saved.transitions.map(value => value.id)).toEqual(before.transitions.map(value => value.id));

  // ОТМЕНА in the dialog leaves the saved name.
  await page.locator('#btnEditFacilityDevice').click();
  await name.fill('Неприменённое название');
  await dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.locator('#facilityDeviceHeaderName')).toHaveText('Гермодверь 76');
  expect((await fixtureSession(request)).facility.devices.find(device => device.id === DOOR).name).toBe('Гермодверь 76');
  expect(errors).toEqual([]);
});

test('the door\'s details open as counted tabs, and only the open tab is listed (AS3, FR-024)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openDemoObjects(page, request);
  const session = await fixtureSession(request);
  await expect(page.getByRole('tablist', { name: 'Сведения об устройстве', exact: true }))
    .toHaveAttribute('id', 'facilityDetailTabs');

  // With no device selected, the tabs list the whole session.
  await expect(page.locator('#facilitySelectionEditor')).toBeHidden();
  await expect(page.locator('#facilitySelectionEmpty')).toBeVisible();
  await expectTabs(page, expectedDetails(session), 'actions');

  await selectDevice(page, DOOR);
  const door = expectedDetails(session, DOOR);
  expect(Object.values(door).map(ids => ids.length)).toEqual([1, 3, 1, 0]);
  for (const name of ['ЧТО ДЕЛАЮТ ИГРОКИ 1', 'ЧТО ВИДЯТ ИГРОКИ 3', 'НЕИСПРАВНОСТИ 1', 'ПРОГРАММЫ 0']) {
    await expect(page.getByRole('tab', { name, exact: true })).toBeVisible();
  }
  await expectTabs(page, door, 'actions');
  for (const [id, , panelID] of TABS) {
    await openTab(page, id);
    await expectTabs(page, door, id);
    const panels = page.locator('#facilityDetailPanel').getByRole('tabpanel');
    await expect(panels).toHaveCount(1);
    await expect(panels).toHaveAttribute('id', panelID);
  }

  // Another device and the whole session recount under the open tab.
  await selectDevice(page, VENTILATION);
  await expectTabs(page, expectedDetails(session, VENTILATION), 'programs');
  await page.locator('#btnClearFacilitySelection').click();
  await expect(page.locator('#facilitySelectionEditor')).toBeHidden();
  await expectTabs(page, expectedDetails(session), 'programs');
  expect(errors).toEqual([]);
});

test('the tabs follow the arrow, Home, and End keys, and the open tab is kept for the run (FR-024)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  await selectDevice(page, DOOR);
  const tab = id => page.locator(`#${TABS.find(([value]) => value === id)[1]}`);
  const expectActive = async (id, key) => {
    await expect(tab(id), key).toHaveAttribute('aria-selected', 'true');
    await expect(tab(id), key).toBeFocused();
    await expect(tab(id), key).toHaveAttribute('tabindex', '0');
    await expect(page.locator('#facilityDetailTabs [role="tab"][tabindex="0"]'), key).toHaveCount(1);
    await expect(page.locator('#facilityDetailPanel').getByRole('tabpanel'), key)
      .toHaveAttribute('aria-labelledby', await tab(id).getAttribute('id'));
  };

  await tab('actions').focus();
  for (const [key, id] of [
    ['ArrowRight', 'information'], ['ArrowRight', 'conditions'], ['ArrowRight', 'programs'],
    ['ArrowRight', 'actions'], ['ArrowLeft', 'programs'], ['ArrowLeft', 'conditions'],
    ['Home', 'actions'], ['End', 'programs'], ['Home', 'actions'],
  ]) {
    await page.keyboard.press(key);
    await expectActive(id, `${key} to ${id}`);
  }

  // Selecting another device, clearing the selection, and returning keep the tab.
  await page.keyboard.press('ArrowLeft');
  await expectActive('programs', 'ArrowLeft to programs');
  await selectDevice(page, VENTILATION);
  await expect(tab('programs')).toHaveAttribute('aria-selected', 'true');
  await page.locator('#btnClearFacilitySelection').click();
  await expect(tab('programs')).toHaveAttribute('aria-selected', 'true');
  await selectDevice(page, DOOR);
  await expect(tab('programs')).toHaveAttribute('aria-selected', 'true');
  await expect(page.locator('#facilityTabPanelPrograms')).toBeVisible();
});

test('an empty tab stays visible with count 0, its empty text, and its add control (edge case, FR-024)', async ({ page, request }) => {
  await openDemoObjects(page, request);

  // The door has no recovery program.
  await selectDevice(page, DOOR);
  await openTab(page, 'programs');
  await expect(page.getByRole('tab', { name: 'ПРОГРАММЫ 0', exact: true })).toBeVisible();
  const programs = page.locator('#facilityTabPanelPrograms');
  await expect(programs.locator('#facilityRecoveryProgramList > [data-facility-kind]')).toHaveCount(0);
  await expect(programs.locator('.facility-empty')).toHaveText('Программы восстановления не настроены.');
  await programs.locator('#btnAddFacilityRecoveryProgram').click();
  await expect(page.locator('#facilityRecoveryProgramDialog')).toBeVisible();
  await page.locator('#btnCancelFacilityRecoveryProgram').click();
  await expect(page.locator('#facilityRecoveryProgramDialog')).toBeHidden();

  // The evacuation alarm has no fault.
  await selectDevice(page, 'evacuation-alarm');
  await openTab(page, 'conditions');
  await expect(page.getByRole('tab', { name: 'НЕИСПРАВНОСТИ 0', exact: true })).toBeVisible();
  const conditions = page.locator('#facilityTabPanelConditions');
  await expect(conditions.locator('#facilityConditionList > [data-facility-kind]')).toHaveCount(0);
  await expect(conditions.locator('.facility-empty')).toHaveText('Неисправности не настроены для выбранного устройства.');
  await conditions.locator('#btnAddFacilityCondition').click();
  await expect(page.locator('#facilityConditionDialog')).toBeVisible();
  await page.locator('#btnCancelFacilityCondition').click();
  await expect(page.locator('#facilityConditionDialog')).toBeHidden();

  // No command acts on the reactor; «ДОБАВИТЬ КОМАНДУ» opens a command action.
  await selectDevice(page, 'reactor-main');
  await openTab(page, 'actions');
  await expect(page.getByRole('tab', { name: 'ЧТО ДЕЛАЮТ ИГРОКИ 0', exact: true })).toBeVisible();
  const actions = page.locator('#facilityTabPanelActions');
  await expect(actions.locator('#facilityActionBindingList > [data-facility-kind]')).toHaveCount(0);
  await expect(actions.locator('.facility-empty')).toHaveText('Команды ещё не связаны. Откройте шаг 2 «Действие» в инструкции.');
  const add = actions.locator('#btnAddFacilityActionBinding');
  await expect(add).toHaveText('ДОБАВИТЬ КОМАНДУ');
  await add.click();
  await expect(page.locator('#facilityBindingDialog')).toBeVisible();
  await expect(page.locator('#facilityBindingKind')).toHaveValue('command-action');
  await page.locator('#btnCancelFacilityBinding').click();
  await expect(page.locator('#facilityBindingDialog')).toBeHidden();
});

test('each device row shows its state and active fault count and follows play changes from either side (AS4, FR-025)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await seedDemo(request);
  // The door scene lets the evacuation command run: grid online, door sealed, its faults off.
  const live = (await facilityState(request)).facility;
  for (const [id, stateID] of [['power-grid-main', 'online'], [DOOR, 'sealed'], ['evacuation-alarm', 'quiet']]) {
    if (live.devices.find(device => device.id === id).currentStateId !== stateID) await forceState(request, id, stateID);
  }
  for (const fault of ['door-offline', 'grid-unpowered']) await setConditionActive(request, fault, false);
  await openOverseer(page);

  const facility = (await fixtureSession(request)).facility;
  await expect(page.locator('#facilityDeviceList button[data-facility-kind="device"]')).toHaveCount(facility.devices.length);
  for (const device of facility.devices) {
    const { text, faults } = rowState(facility, device);
    await expectRow(page, device.id, text, faults);
    await expect(deviceRow(page, device.id)).toHaveAttribute('data-current-state-id', device.currentStateId);
    // The row holds the name and the state; the kind label moved to the header.
    expect(await deviceRow(page, device.id).evaluate(row => [...row.children].map(child => child.tagName === 'STRONG'
      ? ['STRONG', child.textContent] : [child.className]))).toEqual([['STRONG', device.name], ['facility-device-row-state']]);
  }
  await expectRow(page, VENTILATION, 'Загрязнён · ● 1', 1);

  // Card changes on the ventilation loop: the fault, then the state.
  await selectDevice(page, VENTILATION);
  const card = playCard(page);
  await card.faultSwitch('air-spore-bloom').click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
  await expectRow(page, VENTILATION, 'Загрязнён', 0);
  await card.faultSwitch('air-spore-bloom').click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await expectRow(page, VENTILATION, 'Загрязнён · ● 1', 1);
  await card.run('purge').click();
  await expect(card.status).toHaveText('ПЕРЕХОД ВЫПОЛНЕН');
  await expectRow(page, VENTILATION, 'Очищен', 0);
  await expect(deviceRow(page, VENTILATION)).toHaveAttribute('data-current-state-id', 'filtered');
  await card.run('contaminate').click();
  await expect(card.status).toHaveText('ПЕРЕХОД ВЫПОЛНЕН');
  await expectRow(page, VENTILATION, 'Загрязнён · ● 1', 1);

  // A player command opens the door from the terminal side.
  await expectRow(page, DOOR, 'Заперта', 0);
  const command = await request.post(`${FIXTURE}/player-command`, {
    data: { terminalId: 't_demo1', commandId: 'n_cmd_open_evacuation', decision: 'approve' },
  });
  expect(command.ok()).toBe(true);
  await expectRow(page, DOOR, 'Открыта', 0);
  await expect(deviceRow(page, DOOR)).toHaveAttribute('data-current-state-id', 'open');

  // Search matches the name, the ID, and the current state name.
  const search = page.locator('#facilityDeviceSearch');
  for (const query of ['Контур вентиляции', VENTILATION, 'Загрязнён']) {
    await search.fill(query);
    expect(await page.locator('#facilityDeviceList button[data-facility-kind="device"]')
      .evaluateAll(rows => rows.map(row => row.dataset.facilityId)), query).toEqual([VENTILATION]);
  }
  await search.fill('');
  expect(errors).toEqual([]);
});
