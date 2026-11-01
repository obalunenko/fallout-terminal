import { expect, test } from '@playwright/test';

import { blockedReason, expectSaved, undo } from './helpers/authoring-bar.mjs';
import {
  deviceDialog, deviceList, faultDialog, faultWays, openDeviceDialog, openTransitionCell, programDialog,
} from './helpers/facility-editors.mjs';
import { openDemoObjects, openPlayerTable, playCard, PLAYER_FIXTURE, selectDevice } from './helpers/facility-play.mjs';
import { expectNoOrphanLabels, facilityState, fixtureSession } from './helpers/overseer-forms.mjs';

// User Story 2: configure a fault in one place
// (specs/041-facility-editors-language/spec.md, contracts/overseer-ui.md «Fault dialog»).

const DOOR = 'vault-door-main';
const DOOR_FAULT = 'door-offline';
const DOOR_FAULT_NAME = 'Контроллер гермодвери не отвечает';
const SPORES = 'air-spore-bloom';
const SPORES_NAME = 'Споровое загрязнение воздушного контура';
const NETWORK = 'network-isolated';
const NETWORK_NAME = 'Закрытый канал изолирован';
const OVERSEER = 'Смотритель вручную';
const WORKING_WARNING = 'Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.';
const ACTIVE_WARNING = 'Ни один переход не включает эту неисправность снова. После устранения её можно включить только вручную.';
const DEVICE_HINT = 'во время игры — переключатель на карточке устройства';
const TERMINAL_HINT = 'во время игры — переключатель в списке «Неисправности терминалов»';

// openFault opens a fault from a row of the «НЕИСПРАВНОСТИ» tab (FR-019).
async function openFault(page, id) {
  await page.locator('#facilityTabConditions').click();
  await page.locator(`#facilityConditionList button[data-facility-kind="condition"][data-facility-id="${id}"]`).click();
  const fault = faultDialog(page);
  await expect(fault.dialog).toBeVisible();
  return fault;
}

// waysOf reads the rows of «КАК ПОЯВЛЯЕТСЯ» or «КАК УСТРАНЯЕТСЯ» with their names.
async function waysOf(list) {
  const ways = await faultWays(list);
  const names = await list.locator('.facility-fault-way-name').allTextContents();
  return ways.map((way, index) => ({
    kind: way.kind, deviceId: way.deviceId, transitionId: way.transitionId, programId: way.programId, name: names[index],
  }));
}

const transitionWay = (device, transition) => ({
  kind: 'transition', deviceId: device.id, transitionId: transition.id, programId: '',
  name: `${device.name} — ${transition.name}`,
});
const overseerWay = { kind: 'overseer', deviceId: '', transitionId: '', programId: '', name: OVERSEER };

// expectedWays derives both lists of a fault from the session data (SC-004):
// the transitions with an effect on it, the programs that run a clearing one,
// and the Overseer by hand.
function expectedWays(session, condition) {
  const appears = [];
  const clears = [];
  for (const device of session.facility.devices) {
    for (const transition of device.transitions || []) {
      const effect = transition.conditionEffects?.find(value => value.conditionId === condition.id);
      if (effect) (effect.active === true ? appears : clears).push(transitionWay(device, transition));
    }
  }
  const clearing = new Set(clears.map(way => JSON.stringify([way.deviceId, way.transitionId])));
  const programs = (session.facility.recoveryPrograms || [])
    .filter(program => program.transitions.some(request => clearing.has(JSON.stringify([request.deviceId, request.transitionId]))))
    .map(program => ({ kind: 'program', deviceId: '', transitionId: '', programId: program.id, name: program.name }));
  return { appears, clears: [...clears, ...programs, overseerWay] };
}

function pickerOption(picker, deviceId, transitionId) {
  return picker.locator(`option[data-device-id="${deviceId}"][data-transition-id="${transitionId}"]`);
}

// chooseWay picks a transition in a fault picker, whose option values are opaque.
async function chooseWay(picker, deviceId, transitionId) {
  const option = pickerOption(picker, deviceId, transitionId);
  await expect(option).toHaveCount(1);
  await picker.selectOption(await option.getAttribute('value'));
}

const transitionOf = (session, deviceId, transitionId) => session.facility.devices
  .find(device => device.id === deviceId).transitions.find(transition => transition.id === transitionId);
const authored = session => {
  const { devices, conditions, recoveryPrograms } = session.facility;
  return { devices, conditions, recoveryPrograms };
};

test('every demo fault lists exactly the transitions and programs that turn it on and off (SC-004, FR-014, FR-015, FR-019)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const session = await fixtureSession(request);
  expect(session.facility.conditions.length).toBeGreaterThan(0);
  for (const condition of session.facility.conditions) {
    const fault = await openFault(page, condition.id);
    const expected = expectedWays(session, condition);
    await expect(fault.title).toHaveText('НЕИСПРАВНОСТЬ');
    await expect(fault.name).toHaveValue(condition.name);
    expect(await waysOf(fault.appears), `${condition.name}: КАК ПОЯВЛЯЕТСЯ`).toEqual(expected.appears);
    expect(await waysOf(fault.clears), `${condition.name}: КАК УСТРАНЯЕТСЯ`).toEqual(expected.clears);
    if (expected.appears.length) await expect(fault.appearsWarning).toBeHidden();
    else await expect(fault.appearsWarning).toHaveText(condition.initialActive ? ACTIVE_WARNING : WORKING_WARNING);
    await expect(page.locator('#facilityFaultAppearsInitial')).toBeVisible({ visible: Boolean(condition.initialActive) });
    await expect(fault.dialog.locator('#facilityFaultClearsList li[data-kind="overseer"] .facility-fault-way-hint'))
      .toHaveText(condition.terminal ? TERMINAL_HINT : DEVICE_HINT);
    await fault.cancel.click();
    await expect(fault.dialog).toBeHidden();
  }
});

test('a fault no transition turns on warns that only the Overseer can (AS1, FR-013, FR-014, FR-020)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const before = await fixtureSession(request);
  const fault = await openFault(page, DOOR_FAULT);
  await expect(fault.title).toHaveText('НЕИСПРАВНОСТЬ');
  await expect(fault.name).toBeFocused();
  await expect(fault.appears).toHaveCount(0);
  await expect(fault.appearsWarning).toHaveText(WORKING_WARNING);
  await expect(page.locator('#facilityFaultAppearsInitial')).toBeHidden();
  await expect(fault.clears.locator('.facility-fault-way-name'))
    .toHaveText(['Гермодверь Убежища 76 — Открыть гермодверь', OVERSEER]);

  // Every setting sits in this one dialog, and the operator checkbox is gone.
  for (const label of ['НАЗВАНИЕ', 'КАТЕГОРИЯ', 'ГДЕ ДЕЙСТВУЕТ', 'УСТРОЙСТВО', 'ЧТО БЛОКИРУЕТ']) {
    await expect(fault.dialog.getByLabel(label, { exact: true })).toBeVisible();
  }
  await expect(fault.dialog.locator('fieldset.facility-condition-initial legend')).toHaveText('В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА');
  await expect(page.locator('#facilityFaultAppearsHeading')).toHaveText('КАК ПОЯВЛЯЕТСЯ');
  await expect(page.locator('#facilityFaultClearsHeading')).toHaveText('КАК УСТРАНЯЕТСЯ');
  await expect(page.locator('#facilityConditionPrivateRecovery')).toHaveCount(0);
  await expect(fault.dialog.getByText('ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ')).toHaveCount(0);
  expect(await fault.technical.evaluate(details => details.open)).toBe(false);
  await expectNoOrphanLabels(fault.dialog, 'in the fault dialog');

  // Edge case: a fault active from the start that no transition turns on again.
  await page.locator('#facilityConditionInitialActive').check();
  await expect(page.locator('#facilityFaultAppearsInitial')).toHaveText('Активна с начала игры.');
  await expect(fault.appearsWarning).toHaveText(ACTIVE_WARNING);
  await fault.cancel.click();
  await expect(fault.dialog).toBeHidden();
  expect(authored(await fixtureSession(request))).toEqual(authored(before));
});

test('a transition added under «КАК ПОЯВЛЯЕТСЯ» turns the fault on and shows in its editor (AS2, FR-014, FR-017, FR-019)', async ({ page, request }) => {
  const demo = await openDemoObjects(page, request);
  const fault = await openFault(page, DOOR_FAULT);
  await fault.addActivation.click();
  await expect(fault.activationPicker).toBeVisible();
  await expect(fault.activationPicker).toBeFocused();
  // The picker leaves out a listed transition and one a command runs with a clearing one (R3).
  await expect(pickerOption(fault.activationPicker, DOOR, 'open')).toHaveCount(0);
  await expect(pickerOption(fault.activationPicker, 'evacuation-alarm', 'sound-evacuation')).toHaveCount(0);
  await expect(pickerOption(fault.activationPicker, DOOR, 'seal')).toHaveText('Гермодверь Убежища 76 — Запереть гермодверь');
  await chooseWay(fault.activationPicker, DOOR, 'seal');
  await expect(fault.activationPicker).toBeHidden();
  await expect(fault.addActivation).toBeFocused();
  await expect(fault.appearsWarning).toBeHidden();
  const seal = transitionWay(demo.facility.devices.find(device => device.id === DOOR), { id: 'seal', name: 'Запереть гермодверь' });
  expect(await waysOf(fault.appears)).toEqual([seal]);

  await fault.save.click();
  await expect(fault.dialog).toBeHidden();
  await expectSaved(page);
  await expect(page.locator('#btnUndoReason')).toHaveText(`ОТМЕНИТЬ: Неисправность «${DOOR_FAULT_NAME}»`);
  const saved = await fixtureSession(request);
  expect(transitionOf(saved, DOOR, 'seal').conditionEffects).toEqual([{ conditionId: DOOR_FAULT, active: true }]);
  expect(transitionOf(saved, DOOR, 'open').conditionEffects).toEqual([{ conditionId: DOOR_FAULT, active: false }]);
  const storedRecovery = demo.facility.conditions.find(condition => condition.id === DOOR_FAULT).recovery;
  expect(saved.facility.conditions.find(condition => condition.id === DOOR_FAULT).recovery).toEqual(storedRecovery);

  // The transition's editor lists the effect read-only, with the way back to the fault.
  await openDeviceDialog(page, DOOR);
  const editor = deviceDialog(page);
  await expect(editor.cell('open', 'sealed')).toContainText('включает 1 неисправность');
  const panel = await openTransitionCell(page, 'open', 'sealed');
  const effect = panel.locator(`.facility-transition-row[data-transition-id="seal"] li.facility-transition-fault-effect[data-condition-id="${DOOR_FAULT}"]`);
  await expect(effect).toHaveAttribute('data-active', 'true');
  await expect(effect).toContainText(`Включает «${DOOR_FAULT_NAME}»`);
  await effect.getByRole('button', { name: 'К НЕИСПРАВНОСТИ', exact: true }).click();
  await expect(editor.dialog).toBeHidden();
  await expect(fault.dialog).toBeVisible();
  await expect(fault.name).toHaveValue(DOOR_FAULT_NAME);
  await expect(fault.appearsWarning).toBeHidden();
  expect(await waysOf(fault.appears)).toEqual([seal]);
});

test('a fault turned on by one transition and cleared by another and a program lists every way (AS3, FR-015, FR-020)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const fault = await openFault(page, SPORES);
  await expect(fault.name).toHaveValue(SPORES_NAME);
  await expect(fault.appears.locator('.facility-fault-way-name')).toHaveText(['Контур вентиляции теплицы — Смоделировать загрязнение']);
  await expect(fault.appearsWarning).toBeHidden();
  await expect(page.locator('#facilityFaultAppearsInitial')).toHaveText('Активна с начала игры.');
  await expect(fault.clears.locator('.facility-fault-way-name'))
    .toHaveText(['Контур вентиляции теплицы — Продуть воздушный контур', 'ВОЗДУХ-76', OVERSEER]);
  expect((await waysOf(fault.clears)).map(way => way.kind)).toEqual(['transition', 'program', 'overseer']);

  const program = page.locator('#facilityFaultClearsList li[data-kind="program"]');
  await expect(program).toHaveAttribute('data-program-id', 'air-recovery-76');
  await expect(program.locator('.facility-fault-way-kind')).toHaveText('программа восстановления');
  await expect(program.locator('button.facility-fault-way-open')).toHaveText('К ПРОГРАММЕ');
  // Program and Overseer rows follow from the data, so only transition rows can be removed.
  await expect(program.locator('button.facility-fault-way-remove')).toHaveCount(0);
  const overseer = page.locator('#facilityFaultClearsList li[data-kind="overseer"]');
  await expect(overseer.locator('.facility-fault-way-hint')).toHaveText(DEVICE_HINT);
  await expect(overseer.locator('button')).toHaveCount(0);
  await expect(fault.clears.first().locator('button.facility-fault-way-remove'))
    .toHaveAccessibleName('Убрать: Контур вентиляции теплицы — Продуть воздушный контур');
});

test('a program started from «КАК УСТРАНЯЕТСЯ» holds the clearing transition and returns to the fault (AS4, FR-016, FR-043)', async ({ page, request }) => {
  const demo = await openDemoObjects(page, request);
  const fault = await openFault(page, SPORES);
  await fault.addProgram.click();
  const program = programDialog(page);
  await expect(program.dialog).toBeVisible();
  await expect(fault.dialog).toBeVisible();
  await expect(program.name).toBeFocused();
  await expect(page.locator('#facilityRecoveryProgramFault')).toHaveText(`Для неисправности «${SPORES_NAME}»`);
  await expect(program.rows).toHaveCount(1);
  await expect(program.rows.locator('select.facility-action-device')).toHaveValue('ventilation-main');
  await expect(program.rows.locator('select.facility-action-transition')).toHaveValue('purge');
  await expect(program.commandsWarning).toHaveText(
    'Ни одна команда не запускает эту программу. Выберите её в действии команды, чтобы игроки могли её запустить.',
  );

  await program.name.fill('ПРОДУВКА-76');
  await program.save.click();
  await expect(program.dialog).toBeHidden();
  await expect(fault.dialog).toBeVisible();
  await expect(fault.addProgram).toBeFocused();
  await expectSaved(page);
  await expect(fault.clears.locator('.facility-fault-way-name'))
    .toHaveText(['Контур вентиляции теплицы — Продуть воздушный контур', 'ВОЗДУХ-76', 'ПРОДУВКА-76', OVERSEER]);
  await expect(page.locator('#btnUndoReason')).toHaveText('ОТМЕНИТЬ: Программа «ПРОДУВКА-76»');
  const saved = await fixtureSession(request);
  expect(saved.facility.recoveryPrograms.find(value => value.name === 'ПРОДУВКА-76'))
    .toMatchObject({ transitions: [{ deviceId: 'ventilation-main', transitionId: 'purge' }] });
  // The program is its own edit: the fault keeps its stored recovery entries.
  expect(saved.facility.conditions.find(condition => condition.id === SPORES).recovery)
    .toEqual(demo.facility.conditions.find(condition => condition.id === SPORES).recovery);
});

test('a program started from a fault and then canceled saves nothing (edge case, FR-016)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const before = await facilityState(request);
  const session = await fixtureSession(request);
  const fault = await openFault(page, SPORES);
  const ways = await waysOf(fault.clears);
  await fault.addProgram.click();
  const program = programDialog(page);
  await expect(program.dialog).toBeVisible();
  await program.name.fill('НЕНУЖНАЯ ПРОГРАММА');
  await program.cancel.click();
  await expect(program.dialog).toBeHidden();
  await expect(fault.dialog).toBeVisible();
  await expect(fault.addProgram).toBeFocused();
  await expect(fault.name).toHaveValue(SPORES_NAME);
  expect(await waysOf(fault.clears)).toEqual(ways);
  await fault.cancel.click();
  await expect(fault.dialog).toBeHidden();
  const after = await facilityState(request);
  expect(after.saveCalls).toBe(before.saveCalls);
  expect(authored(await fixtureSession(request))).toEqual(authored(session));
});

test('a recovery program lists its transitions as one uniform list and names the commands that run it (AS5, FR-018)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  await page.locator('#facilityTabPrograms').click();
  await page.locator('#facilityRecoveryProgramList').getByRole('button', { name: 'Изменить: СЕТЬ-76', exact: true }).click();
  const program = programDialog(page);
  await expect(program.dialog).toBeVisible();
  await expect(page.locator('#facilityRecoveryProgramDialogTitle')).toHaveText('ПРОГРАММА ВОССТАНОВЛЕНИЯ');
  await expect(page.locator('#facilityRecoveryProgramDialogDescription'))
    .toHaveText('Переходы выполняются вместе, когда команда запускает программу.');
  await expect(page.locator('#facilityRecoveryProgramFault')).toBeHidden();
  await expect(program.name).toHaveValue('СЕТЬ-76');

  await expect(program.rows).toHaveCount(2);
  const shapes = await program.rows.evaluateAll(rows => rows.map(row => [...row.querySelectorAll('*')]
    .map(element => `${element.tagName}.${element.className}`)));
  expect(shapes[1]).toEqual(shapes[0]);
  await expect(program.rows.locator('select.facility-action-device')).toHaveCount(2);
  await expect(program.rows.nth(0).locator('select.facility-action-device')).toHaveValue('network-overseer');
  await expect(program.rows.nth(0).locator('select.facility-action-transition')).toHaveValue('connect');
  await expect(program.rows.nth(1).locator('select.facility-action-device')).toHaveValue('security-turret');
  await expect(program.rows.nth(1).locator('select.facility-action-transition')).toHaveValue('track');
  await expect(program.rows.locator('button.facility-action-remove')).toHaveText(['УДАЛИТЬ', 'УДАЛИТЬ']);
  await expect(program.rows.nth(0).locator('button.facility-action-remove'))
    .toHaveAccessibleName('Удалить переход: Служебная сеть Смотрителя — Восстановить служебную сеть');
  await expect(program.rows.nth(1).locator('button.facility-action-remove'))
    .toHaveAccessibleName('Удалить переход: Турель внешнего поста — Включить сопровождение');

  // One add control, and none of the removed fields of the old dialog.
  await expect(program.dialog.getByRole('button', { name: /^\+/ })).toHaveText(['+ ПЕРЕХОД']);
  await expect(program.addTransition).toBeVisible();
  for (const removed of ['#facilityRecoveryProgramDevice', '#facilityRecoveryProgramTransition', '#facilityProgramExtraTransitions']) {
    await expect(page.locator(removed)).toHaveCount(0);
  }
  await expect(program.dialog.getByText('ДОБАВИТЬ ПЕРЕХОД ПРОГРАММЫ')).toHaveCount(0);

  await expect(page.locator('#facilityProgramCommandsHeading')).toHaveText('ЗАПУСКАЮТ КОМАНДЫ');
  await expect(page.locator('#facilityProgramCommandList li'))
    .toHaveText(['Терминал смотрителя — аварийный удалённый доступ › Запустить СЕТЬ-76']);
  await expect(program.commandsWarning).toBeHidden();
  await program.cancel.click();
  await expect(program.dialog).toBeHidden();
});

test('a terminal fault sits in «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» with the device-card switch (AS6, FR-019, FR-021)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  await selectDevice(page, DOOR);
  const list = deviceList(page);
  const session = await fixtureSession(request);
  const terminalFaults = session.facility.conditions.filter(condition => condition.terminal);
  await expect(page.locator('#facilityTerminalFaultsHeading')).toHaveText('НЕИСПРАВНОСТИ ТЕРМИНАЛОВ');
  expect(await list.terminalFaults.evaluateAll(items => items.map(item => item.dataset.conditionId)))
    .toEqual(terminalFaults.map(condition => condition.id));
  // The group follows the device list.
  expect(await page.locator('#facilityDeviceList').evaluate(element => Boolean(element
    .compareDocumentPosition(document.getElementById('facilityTerminalFaults')) & Node.DOCUMENT_POSITION_FOLLOWING))).toBe(true);

  const row = list.terminalFault(NETWORK);
  const toggle = list.terminalFaultSwitch(NETWORK);
  await expect(row).toHaveAttribute('data-facility-kind', 'terminal-condition');
  await expect(row).toHaveAttribute('data-active', 'true');
  await expect(toggle).toHaveAttribute('role', 'switch');
  await expect(toggle).toHaveAttribute('aria-checked', 'true');
  await expect(toggle).toHaveAccessibleName(NETWORK_NAME);
  await expect(row.locator('.facility-play-fault-current')).toHaveText('АКТИВНА');
  await expect(row.locator('.facility-terminal-fault-terminal'))
    .toHaveText('ТЕРМИНАЛ: Терминал смотрителя — аварийный удалённый доступ');
  // Device cards keep only their own faults.
  await expect(playCard(page).faults).toHaveCount(1);
  await expect(playCard(page).fault(DOOR_FAULT)).toBeVisible();

  // Search and filters never narrow the group.
  await list.search.fill('нет такого устройства');
  await expect(list.rows).toHaveCount(0);
  await expect(list.terminalFaults).toHaveCount(terminalFaults.length);
  await list.reset.click();
  await list.filter('faulty').click();
  await expect(list.terminalFaults).toHaveCount(terminalFaults.length);
  await list.filter('all').click();

  await toggle.click();
  await expect(list.terminalFaultStatus).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
  await expect(list.terminalFaultStatus).toHaveAttribute('data-state', 'ok');
  await expect(toggle).toHaveAttribute('aria-checked', 'false');
  await expect(row.locator('.facility-play-fault-current')).toHaveText('НЕАКТИВНА');
  await expect.poll(async () => (await fixtureSession(request)).facility.conditions
    .find(condition => condition.id === NETWORK).currentActive ?? false).toBe(false);
  await list.terminalFaultSwitch(NETWORK).click();
  await expect(list.terminalFaultStatus).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await expect(list.terminalFaultSwitch(NETWORK)).toHaveAttribute('aria-checked', 'true');
  // The selected device stays selected through both presses.
  await expect(list.row(DOOR)).toHaveAttribute('aria-pressed', 'true');
  await expect(playCard(page).card).toBeVisible();

  const open = list.terminalFaultOpen(NETWORK);
  await expect(open).toHaveText('ИЗМЕНИТЬ');
  await expect(open).toHaveAccessibleName(`Изменить неисправность: ${NETWORK_NAME}`);
  await open.click();
  const fault = faultDialog(page);
  await expect(fault.dialog).toBeVisible();
  await expect(fault.name).toHaveValue(NETWORK_NAME);
  await expect(fault.scope).toHaveValue('terminal');
  await expect(page.locator('#facilityConditionTerminal')).toHaveValue('t_demo2');
  await expect(page.locator('#facilityFaultClearsList li[data-kind="overseer"] .facility-fault-way-hint'))
    .toHaveText(TERMINAL_HINT);
  await fault.cancel.click();
  await expect(fault.dialog).toBeHidden();
});

test('a new capability-blocking fault saves with only «Смотритель вручную» and switches by hand (FR-020)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  await selectDevice(page, DOOR);
  await page.locator('#facilityTabConditions').click();
  await page.locator('#btnAddFacilityCondition').click();
  const fault = faultDialog(page);
  await expect(fault.dialog).toBeVisible();
  await expect(page.locator('#facilityConditionPrivateRecovery')).toHaveCount(0);
  await fault.name.fill('Привод заклинило');
  await fault.category.selectOption('offline');
  await expect(page.locator('#facilityConditionDevice')).toHaveValue(DOOR);
  await fault.capability.selectOption('execute-command');
  expect(await waysOf(fault.clears)).toEqual([overseerWay]);
  await expect(fault.clears.locator('.facility-fault-way-hint')).toHaveText(DEVICE_HINT);
  await expect(fault.appearsWarning).toHaveText(WORKING_WARNING);
  await fault.save.click();
  await expect(fault.dialog).toBeHidden();
  await expectSaved(page);

  const saved = await fixtureSession(request);
  const condition = saved.facility.conditions.find(value => value.name === 'Привод заклинило');
  expect(condition).toMatchObject({
    device: { deviceId: DOOR }, category: 'offline', effects: [{ capabilityBlock: { capability: 'execute-command' } }],
  });
  expect(condition.recovery).toBeUndefined();
  expect(saved.facility.devices.flatMap(device => device.transitions || [])
    .filter(transition => transition.conditionEffects?.some(effect => effect.conditionId === condition.id))).toEqual([]);

  // The Overseer turns it on and clears it by hand from the device card.
  const card = playCard(page);
  await card.faultSwitch(condition.id).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await card.faultSwitch(condition.id).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
  await expect(card.faultSwitch(condition.id)).toHaveAttribute('aria-checked', 'false');
  expect((await fixtureSession(request)).facility.conditions.find(value => value.id === condition.id).currentActive ?? false)
    .toBe(false);
});

test('a fault edit that changes transitions of two devices is one «ОТМЕНИТЬ» entry (FR-043)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const before = await fixtureSession(request);
  const { saveCalls } = await facilityState(request);
  const fault = await openFault(page, DOOR_FAULT);
  await fault.addActivation.click();
  await chooseWay(fault.activationPicker, 'evacuation-alarm', 'silence');
  await page.locator('#btnAddFaultClearing').click();
  await expect(fault.clearingPicker).toBeFocused();
  await expect(pickerOption(fault.clearingPicker, 'evacuation-alarm', 'silence')).toHaveCount(0);
  await chooseWay(fault.clearingPicker, 'power-grid-main', 'restore');
  await expect(fault.appears.locator('.facility-fault-way-name')).toHaveText(['Сигнал эвакуации — Отключить сигнал']);
  await expect(fault.clears.locator('.facility-fault-way-name')).toHaveText([
    'Резервная сеть Убежища 76 — Запустить резервную сеть', 'Гермодверь Убежища 76 — Открыть гермодверь', OVERSEER,
  ]);
  await fault.save.click();
  await expect(fault.dialog).toBeHidden();
  await expect.poll(async () => (await facilityState(request)).saveCalls).toBe(saveCalls + 1);
  await expectSaved(page);

  const saved = await fixtureSession(request);
  expect(transitionOf(saved, 'evacuation-alarm', 'silence').conditionEffects).toEqual([{ conditionId: DOOR_FAULT, active: true }]);
  expect(transitionOf(saved, 'power-grid-main', 'restore').conditionEffects).toEqual([
    { conditionId: 'grid-unpowered', active: false }, { conditionId: DOOR_FAULT, active: false },
  ]);
  await expect(page.locator('#btnUndoReason')).toHaveText(`ОТМЕНИТЬ: Неисправность «${DOOR_FAULT_NAME}»`);

  await undo(page);
  expect(authored(await fixtureSession(request))).toEqual(authored(before));
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('НЕЧЕГО ОТМЕНЯТЬ');
});

test('the terminal-fault switch reaches a connected player while a device is selected (FR-021, FR-045)', async ({ browser, request }) => {
  const table = await openPlayerTable(browser, request);
  try {
    const { overseer, player } = table;
    // A terminal fault on the player's terminal that unsettles its display, off at the start.
    const opened = await (await request.get(`${PLAYER_FIXTURE}/open-session`)).json();
    const session = structuredClone(opened.session);
    session.facility.conditions.push({
      id: 'security-display-flicker', name: 'Помехи на экране охраны', category: 'display-unstable',
      terminal: { terminalId: 'terminal-facility-security' }, initialActive: false, currentActive: false,
      effects: [{ displayInstability: {} }],
    });
    const response = await request.post(`${PLAYER_FIXTURE}/save-authoring`, {
      data: { session, expectedAuthoringRevision: opened.authoringRevision, correlationId: 'journey-terminal-fault' },
    });
    expect(await response.json()).toMatchObject({ ok: true });
    await overseer.goto(`${PLAYER_FIXTURE}/overseer`);
    await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ', exact: true }).click();
    await expect(overseer.locator('#mainLayout')).toBeVisible();
    await overseer.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();

    await selectDevice(overseer, 'device-security-door');
    const list = deviceList(overseer);
    const toggle = list.terminalFaultSwitch('security-display-flicker');
    await expect(list.terminalFault('security-display-flicker').locator('.facility-terminal-fault-terminal'))
      .toHaveText('ТЕРМИНАЛ: Security control');
    const screen = player.locator('#screen');
    const unstable = /(^|\s)display-unstable(\s|$)/;
    await expect(screen).not.toHaveAttribute('data-presentation-effect', unstable);

    await toggle.click();
    await expect(list.terminalFaultStatus).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
    await expect(screen).toHaveAttribute('data-presentation-effect', unstable);
    await list.terminalFaultSwitch('security-display-flicker').click();
    await expect(list.terminalFaultStatus).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
    await expect(screen).not.toHaveAttribute('data-presentation-effect', unstable);
    // The selected device's card stays open through both presses.
    await expect(list.row('device-security-door')).toHaveAttribute('aria-pressed', 'true');
    await expect(playCard(overseer).card).toBeVisible();
  } finally {
    await table.close();
  }
});

test('the start position is drawn as Overseer segments, not system radios (042 FR-019)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const fault = faultDialog(page);
  await openFault(page, DOOR_FAULT);
  await expect(fault.dialog).toBeVisible();
  const working = page.locator('#facilityConditionInitialWorking');
  const active = page.locator('#facilityConditionInitialActive');
  for (const radio of [working, active]) {
    expect(await radio.evaluate(input => getComputedStyle(input).appearance)).toBe('none');
  }
  const segments = page.locator('.facility-condition-initial label');
  await expect(segments).toHaveText(['ИСПРАВНО', 'НЕИСПРАВНОСТЬ АКТИВНА']);
  await active.check();
  await expect(active).toBeChecked();
  const colors = await segments.evaluateAll(labels => labels.map(label => getComputedStyle(label).color));
  expect(colors[1]).toBe('rgb(255, 224, 138)');
  expect(colors[0]).not.toBe(colors[1]);
  await fault.cancel.click();
});

