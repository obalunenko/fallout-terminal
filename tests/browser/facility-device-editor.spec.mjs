import { expect, test } from '@playwright/test';

import { blockedReason, expectSaved, undo } from './helpers/authoring-bar.mjs';
import {
  deviceDialog, matrixCells, openDeviceDialog, openTransitionCell, stateChips,
} from './helpers/facility-editors.mjs';
import { forceState, openDemoObjects } from './helpers/facility-play.mjs';
import { fixtureSession, openOverseer, resetFixture, seedDemo } from './helpers/overseer-forms.mjs';

// User Story 1: edit a device's states and transitions on one screen
// (specs/041-facility-editors-language/spec.md, contracts/overseer-ui.md «Device dialog»).

test.use({ viewport: { width: 1440, height: 900 } });

const DOOR = 'vault-door-main';
const DOOR_NAME = 'Гермодверь Убежища 76';
const REACTOR = 'reactor-main';
const GRID = 'power-grid-main';
const VENTILATION = 'ventilation-main';
const IDENTIFIER_INPUTS = '#facilityDeviceID, .facility-state-id, .facility-transition-id';

const openRow = page => page.locator('#facilityTransitionPanel .facility-transition-row:not([hidden])');

async function saveCalls(page) {
  return page.evaluate(() => window.__desktopFixture.calls.filter(call => call.method === 'SaveAuthoring').length);
}

async function savedDevice(request, id) {
  return (await fixtureSession(request)).facility.devices.find(device => device.id === id);
}

// The identifier inputs of the device dialog that render at all.
async function visibleIdentifierFields(page) {
  return page.locator('#facilityDeviceForm').evaluate((form, selector) => [...form.querySelectorAll(selector)]
    .filter(input => input.checkVisibility({ visibilityProperty: true }))
    .map(input => input.id || input.className), IDENTIFIER_INPUTS);
}

const values = locator => locator.evaluateAll(inputs => inputs.map(input => input.value));

// addState presses «+ СОСТОЯНИЕ» and renames the new state in its editor.
async function addState(page, name) {
  await page.locator('#btnAddFacilityState').click();
  const input = page.locator('#facilityStateEditorName');
  await expect(input).toBeFocused();
  await input.fill(name);
}

// closeStateEditor closes the open chip editor by choosing its chip again.
async function closeStateEditor(page) {
  await page.locator('#facilityDeviceStateChips button.facility-state-chip[aria-expanded="true"]').click();
  await expect(page.locator('#facilityStateEditor')).toBeHidden();
}

async function selectedText(input) {
  return input.evaluate(element => element.value.slice(element.selectionStart, element.selectionEnd));
}

// stateRuleCount counts the display rules that read deviceId = stateId.
function stateRuleCount(session, deviceId, stateId) {
  const reads = when => when?.deviceId === deviceId && when.stateId === stateId;
  let count = 0;
  const walk = node => {
    count += (node.facilityNameVariants || []).filter(variant => reads(variant.when)).length;
    for (const block of node.blocks || []) count += (block.facilityTextVariants || []).filter(variant => reads(variant.when)).length;
    count += Number(reads(node.visibleWhen)) + Number(reads(node.availableWhen));
    for (const child of node.children || []) walk(child);
  };
  for (const terminal of session.terminals) walk(terminal.root);
  return count;
}

async function stateUses(page) {
  return page.locator('#facilityStateUses li.facility-state-use').evaluateAll(items => items.map(item => ({
    destination: item.dataset.destination,
    sentence: item.querySelector('span').textContent,
    button: item.querySelector('button').textContent,
  })));
}

test('the door shows two starred chips and its «из → в» matrix in the dialog order (AS1, FR-001, FR-002, FR-004)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const dialog = await openDeviceDialog(page, DOOR);
  await expect(page.locator('#facilityDeviceDialogTitle')).toHaveText('УСТРОЙСТВО');
  await expect(dialog.name).toBeFocused();

  // FR-001: name, type, template offer, states, transitions, technical block, status, actions.
  const order = await page.locator('#facilityDeviceForm').evaluate(form => [...form.children]
    .map(child => child.id || child.className).filter(Boolean));
  expect(order).toEqual([
    'facilityDeviceDialogTitle', 'facilityDeviceDialogDescription', 'field-label-inline', 'facilityDeviceName',
    'field-label-inline', 'facilityDeviceKind', 'facilityDeviceTemplate', 'facilityDeviceStatesField',
    'facilityDeviceTransitionsField', 'facilityDeviceTechnical', 'facilityDeviceStatus', 'facilityDeviceError',
    'facility-dialog-actions',
  ]);
  await expect(dialog.technical).not.toHaveAttribute('open', '');

  expect(await stateChips(page)).toEqual([
    { id: 'sealed', initial: true, name: 'Заперта' },
    { id: 'open', initial: false, name: 'Открыта' },
  ]);
  await expect(dialog.chip('sealed').locator('.facility-state-star')).toHaveText('★');
  await expect(dialog.chip('sealed')).toHaveAttribute('aria-label', 'Заперта, начальное состояние');
  await expect(dialog.chip('open').locator('.facility-state-star')).toHaveCount(0);

  await expect(dialog.matrix.locator('caption')).toHaveText('ИЗ → В');
  await expect(dialog.matrix.locator('thead th[scope="col"]')).toHaveText(['Заперта', 'Открыта']);
  await expect(dialog.matrix.locator('tbody th[scope="row"]')).toHaveText(['Заперта', 'Открыта']);
  expect(await matrixCells(page)).toEqual([
    { from: 'sealed', to: 'open', count: 1, text: expect.stringContaining('Открыть гермодверь') },
    { from: 'open', to: 'sealed', count: 1, text: 'Запереть гермодверь' },
  ]);
  await expect(dialog.cell('sealed', 'open').locator('.facility-transition-cell-summary'))
    .toHaveText('1 предусловие · снимает 1 неисправность');
  await expect(dialog.cell('sealed', 'open')).toHaveAttribute('aria-label', 'Заперта → Открыта: Открыть гермодверь');
  for (const state of ['sealed', 'open']) {
    await expect(dialog.unavailable(state, state)).toHaveText('—');
    await expect(dialog.unavailable(state, state).locator('button')).toHaveCount(0);
  }
  await expect(page.locator('#facilityTransitionMatrixHint')).toBeHidden();
  await expect(dialog.panel).toBeHidden();
});

test('a filled cell opens its transition with preconditions and read-only fault effects (AS3, FR-005, FR-006)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const dialog = await openDeviceDialog(page, DOOR);
  const panel = await openTransitionCell(page, 'sealed', 'open');
  await expect(dialog.cell('sealed', 'open')).toHaveAttribute('aria-expanded', 'true');
  await expect(dialog.panelHeading).toHaveText('ПЕРЕХОД: Заперта → Открыта');
  await expect(page.locator('#facilityTransitionCellList')).toBeHidden();
  await expect(dialog.addCellTransition).toBeVisible();

  const row = openRow(page);
  await expect(row).toHaveCount(1);
  await expect(row).toHaveAttribute('data-transition-id', 'open');
  await expect(row.locator('.facility-transition-name')).toHaveValue('Открыть гермодверь');
  await expect(row.locator('.facility-transition-name')).toBeFocused();
  await expect(row.locator('.facility-transition-source')).toHaveValue('sealed');
  await expect(row.locator('.facility-transition-destination')).toHaveValue('open');

  const precondition = row.locator('.facility-precondition-row');
  await expect(precondition).toHaveCount(1);
  const chosen = select => select.evaluate(element => element.selectedOptions[0]?.textContent);
  expect(await chosen(precondition.locator('.facility-precondition-device'))).toBe('Резервная сеть Убежища 76');
  expect(await chosen(precondition.locator('.facility-precondition-state'))).toBe('В работе');
  // The device list leaves out the door itself.
  expect(await precondition.locator('.facility-precondition-device option').evaluateAll(options => options.map(option => option.value)))
    .not.toContain(DOOR);
  await expect(row.locator('.facility-transition-preconditions legend')).toHaveText('ПРЕДУСЛОВИЯ');
  await expect(row.locator('button.facility-add-precondition')).toHaveText('+ ПРЕДУСЛОВИЕ');

  // FR-006, FR-017: each effect names the fault and its direction; nothing here edits it.
  const effects = row.locator('li.facility-transition-fault-effect');
  await expect(effects).toHaveCount(1);
  await expect(effects).toHaveAttribute('data-condition-id', 'door-offline');
  await expect(effects).toHaveAttribute('data-active', 'false');
  await expect(effects.locator('span').first()).toHaveText('Снимает «Контроллер гермодвери не отвечает»');
  await expect(row.locator('.facility-transition-fault-effects').locator('button, input, select'))
    .toHaveCount(1);
  const link = effects.locator('button.facility-fault-link');
  await expect(link).toHaveText('К НЕИСПРАВНОСТИ');

  // Another transition of the door has no effect and says where effects are set.
  await page.locator('#btnCloseFacilityTransitionPanel').click();
  await expect(dialog.panel).toBeHidden();
  await expect(dialog.cell('sealed', 'open')).toBeFocused();
  await openTransitionCell(page, 'open', 'sealed');
  await expect(openRow(page).locator('.facility-transition-fault-effects .facility-empty'))
    .toHaveText('Переход не включает и не снимает неисправности. Это задаётся в окне неисправности.');

  // A changed draft blocks the link until «ГОТОВО»; an unchanged one leads to the fault.
  await openTransitionCell(page, 'sealed', 'open');
  await openRow(page).locator('.facility-transition-name').fill('Открыть гермодверь вручную');
  await expect(dialog.cell('sealed', 'open').locator('.facility-transition-cell-name')).toHaveText('Открыть гермодверь вручную');
  expect(await blockedReason(link)).toBe('СНАЧАЛА НАЖМИТЕ «ГОТОВО»');
  await link.press('Enter');
  await expect(dialog.dialog).toBeVisible();
  await openRow(page).locator('.facility-transition-name').fill('Открыть гермодверь');
  await expect(link).not.toHaveAttribute('aria-disabled', 'true');
  await link.click();
  await expect(dialog.dialog).toBeHidden();
  const fault = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await expect(fault).toBeVisible();
  await expect(page.locator('#facilityConditionName')).toHaveValue('Контроллер гермодвери не отвечает');
  expect(await saveCalls(page)).toBe(0);
  await expect(panel).toBeHidden();
});

test('an empty cell of the three-state reactor starts a named transition that «ГОТОВО» saves (AS2, FR-005, FR-043)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const dialog = await openDeviceDialog(page, REACTOR);
  const before = await savedDevice(request, REACTOR);
  await expect(dialog.matrix.locator('thead th[scope="col"]')).toHaveText(['Аварийная остановка', 'Дежурный режим', 'Критический перегрев']);
  expect((await matrixCells(page)).filter(cell => cell.count === 0).map(cell => `${cell.from}>${cell.to}`)).toEqual([
    'scrammed>critical-overheat', 'standby>critical-overheat', 'critical-overheat>scrammed', 'critical-overheat>standby',
  ]);
  await expect(dialog.cell('standby', 'critical-overheat')).toHaveText('+');
  await expect(dialog.cell('standby', 'critical-overheat'))
    .toHaveAttribute('aria-label', 'Новый переход: Дежурный режим → Критический перегрев');

  await openTransitionCell(page, 'standby', 'critical-overheat');
  const name = openRow(page).locator('.facility-transition-name');
  await expect(name).toHaveValue('Перевести в «Критический перегрев»');
  await expect(name).toBeFocused();
  expect(await selectedText(name)).toBe('Перевести в «Критический перегрев»');
  await expect(dialog.panelHeading).toHaveText('ПЕРЕХОД: Дежурный режим → Критический перегрев');
  await expect(dialog.cell('standby', 'critical-overheat')).toHaveAttribute('data-count', '1');
  await expect(dialog.cell('standby', 'critical-overheat').locator('.facility-transition-cell-name'))
    .toHaveText('Перевести в «Критический перегрев»');
  await name.fill('Разогнать реактор');
  await expect(dialog.cell('standby', 'critical-overheat').locator('.facility-transition-cell-name')).toHaveText('Разогнать реактор');

  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);
  expect(await saveCalls(page)).toBe(1);
  await expect(page.locator('#btnUndoReason')).toHaveText('ОТМЕНИТЬ: Устройство «Реактор Убежища 76»');
  const saved = await savedDevice(request, REACTOR);
  expect(saved.transitions).toEqual([
    ...before.transitions,
    { id: 'razognat-reaktor', name: 'Разогнать реактор', sourceStateId: 'standby', destinationStateId: 'critical-overheat' },
  ].map(transition => expect.objectContaining(transition)));

  await openDeviceDialog(page, REACTOR);
  await expect(dialog.cell('standby', 'critical-overheat')).toHaveAttribute('data-count', '1');
  await expect(dialog.cell('standby', 'critical-overheat').locator('.facility-transition-cell-name')).toHaveText('Разогнать реактор');
  await dialog.cancel.click();

  // The edit is one «ОТМЕНИТЬ» entry.
  await undo(page);
  expect((await savedDevice(request, REACTOR)).transitions).toEqual(before.transitions);
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('НЕЧЕГО ОТМЕНЯТЬ');
});

test('the reactor cell with two transitions lists both and offers a third (edge case, FR-005)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const dialog = await openDeviceDialog(page, REACTOR);
  const cell = dialog.cell('scrammed', 'standby');
  await expect(cell).toHaveAttribute('data-count', '2');
  await expect(cell.locator('.facility-transition-cell-name')).toHaveText(['Перевести в дежурный режим', 'Пуск от резервной сети']);
  await expect(cell.locator('.facility-transition-cell-summary').nth(1)).toHaveText('1 предусловие');
  await expect(cell).toHaveAttribute('aria-label', 'Аварийная остановка → Дежурный режим: Перевести в дежурный режим, Пуск от резервной сети');

  await openTransitionCell(page, 'scrammed', 'standby');
  const choices = page.locator('#facilityTransitionCellList button.facility-transition-choice');
  await expect(choices).toHaveText(['Перевести в дежурный режим', 'Пуск от резервной сети']);
  await expect(choices.first()).toBeFocused();
  await expect(openRow(page)).toHaveCount(0);
  await expect(dialog.addCellTransition).toHaveText('+ ПЕРЕХОД');

  await page.locator('#facilityTransitionCellList button[data-transition-id="backup-grid-start"]').click();
  await expect(openRow(page)).toHaveAttribute('data-transition-id', 'backup-grid-start');
  await expect(choices.nth(1)).toHaveAttribute('aria-pressed', 'true');
  await expect(choices.nth(0)).toHaveAttribute('aria-pressed', 'false');
  await expect(openRow(page).locator('.facility-precondition-row')).toHaveCount(1);

  await dialog.addCellTransition.click();
  await expect(cell).toHaveAttribute('data-count', '3');
  await expect(choices).toHaveCount(3);
  await expect(choices.nth(2)).toHaveAttribute('aria-pressed', 'true');
  const third = openRow(page).locator('.facility-transition-name');
  await expect(third).toBeFocused();
  await expect(openRow(page).locator('.facility-transition-source')).toHaveValue('scrammed');
  await expect(openRow(page).locator('.facility-transition-destination')).toHaveValue('standby');
  await third.fill('Ручной пуск');
  await expect(cell.locator('.facility-transition-cell-name')).toHaveText(['Перевести в дежурный режим', 'Пуск от резервной сети', 'Ручной пуск']);

  // «ОТМЕНА» keeps the session as it was.
  await dialog.cancel.click();
  await expect(dialog.dialog).toBeHidden();
  expect(await saveCalls(page)).toBe(0);
  expect((await savedDevice(request, REACTOR)).transitions.map(transition => transition.id))
    .toEqual(['standby', 'backup-grid-start', 'scram']);
});

test('a saved transition edit is one «ОТМЕНИТЬ» entry and a reopened device keeps transition.recovery (FR-043, R8)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const original = await savedDevice(request, DOOR);
  const originalOpen = original.transitions.find(transition => transition.id === 'open');
  expect(originalOpen.recovery).toBe(true);

  const dialog = await openDeviceDialog(page, DOOR);
  await openTransitionCell(page, 'sealed', 'open');
  const row = openRow(page);
  await row.locator('.facility-transition-name').fill('Открыть гермодверь с пульта');
  await row.locator('button.facility-add-precondition').click();
  const added = row.locator('.facility-precondition-row').nth(1);
  await expect(added.locator('.facility-precondition-device')).toBeFocused();
  // The second row leaves out the grid that the first row already uses.
  expect(await added.locator('.facility-precondition-device option').evaluateAll(options => options.map(option => option.value)))
    .not.toContain(GRID);
  await added.locator('.facility-precondition-device').selectOption('evacuation-alarm');
  await added.locator('.facility-precondition-state').selectOption('quiet');
  await expect(dialog.cell('sealed', 'open').locator('.facility-transition-cell-summary'))
    .toHaveText('2 предусловия · снимает 1 неисправность');
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);
  expect(await saveCalls(page)).toBe(1);
  await expect(page.locator('#btnUndoReason')).toHaveText(`ОТМЕНИТЬ: Устройство «${DOOR_NAME}»`);

  const edited = (await savedDevice(request, DOOR)).transitions.find(transition => transition.id === 'open');
  expect(edited).toEqual({
    ...originalOpen,
    name: 'Открыть гермодверь с пульта',
    preconditions: [...originalOpen.preconditions, { deviceId: 'evacuation-alarm', stateId: 'quiet' }],
  });

  // Reopening the saved device and saving another change keeps the stored flag and effects.
  await openDeviceDialog(page, DOOR);
  await expect(dialog.cell('sealed', 'open').locator('.facility-transition-cell-name')).toHaveText('Открыть гермодверь с пульта');
  await dialog.name.fill('Гермодверь 76');
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);
  expect(await saveCalls(page)).toBe(2);
  const reopened = await savedDevice(request, DOOR);
  expect(reopened.name).toBe('Гермодверь 76');
  expect(reopened.transitions.find(transition => transition.id === 'open')).toEqual(edited);

  // Each «ГОТОВО» is one entry: two reversals restore the door.
  await undo(page);
  await expect(page.locator('#btnUndoReason')).toHaveText(`ОТМЕНИТЬ: Устройство «${DOOR_NAME}»`);
  await undo(page);
  expect(await savedDevice(request, DOOR)).toEqual(original);
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('НЕЧЕГО ОТМЕНЯТЬ');
});

test('starring «Открыта» makes it the starting state with no separate list (AS4, FR-002, FR-003)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  const dialog = await openDeviceDialog(page, DOOR);
  await expect(page.locator('#facilityDeviceInitialState')).toHaveCount(0);
  await expect(dialog.dialog.getByLabel('НАЧАЛЬНОЕ СОСТОЯНИЕ', { exact: true })).toHaveCount(0);
  const visibleSelects = () => page.locator('#facilityDeviceForm').evaluate(form => [...form.querySelectorAll('select')]
    .filter(select => select.checkVisibility({ visibilityProperty: true })).map(select => select.id || select.className));
  expect(await visibleSelects()).toEqual(['facilityDeviceKind']);

  await dialog.chip('sealed').click();
  await expect(dialog.stateEditor).toBeVisible();
  await expect(dialog.chip('sealed')).toHaveAttribute('aria-expanded', 'true');
  await expect(dialog.stateName).toHaveValue('Заперта');
  expect(await blockedReason(page.locator('#btnMakeFacilityStateInitial'))).toBe('УЖЕ НАЧАЛЬНОЕ');

  await dialog.chip('open').click();
  await expect(dialog.chip('sealed')).toHaveAttribute('aria-expanded', 'false');
  await expect(dialog.stateName).toHaveValue('Открыта');
  await expect(page.locator('#btnMakeFacilityStateInitial')).toHaveText('СДЕЛАТЬ НАЧАЛЬНЫМ');
  await page.locator('#btnMakeFacilityStateInitial').click();
  expect(await stateChips(page)).toEqual([
    { id: 'sealed', initial: false, name: 'Заперта' },
    { id: 'open', initial: true, name: 'Открыта' },
  ]);
  await expect(dialog.chip('open')).toHaveAttribute('aria-label', 'Открыта, начальное состояние');
  await expect(dialog.chip('sealed').locator('.facility-state-star')).toHaveCount(0);
  expect(await blockedReason(page.locator('#btnMakeFacilityStateInitial'))).toBe('УЖЕ НАЧАЛЬНОЕ');
  expect(await visibleSelects()).toEqual(['facilityDeviceKind']);

  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);
  const saved = await savedDevice(request, DOOR);
  expect(saved.initialStateId).toBe('open');
  expect(saved.currentStateId).toBe('sealed');
});

test('the starting state and the current state refuse deletion (edge case, FR-008)', async ({ page, request }) => {
  await seedDemo(request);
  // A seeded device starts in its starting state; move the reactor to «Дежурный режим».
  await forceState(request, REACTOR, 'standby');
  await openOverseer(page);
  const dialog = await openDeviceDialog(page, DOOR);
  await dialog.chip('sealed').click();
  const remove = page.locator('#btnDeleteFacilityState');
  await expect(remove).toHaveText('УДАЛИТЬ');
  expect(await blockedReason(remove)).toBe('НАЧАЛЬНОЕ СОСТОЯНИЕ: СНАЧАЛА ОТМЕТЬТЕ ЗВЕЗДОЙ ДРУГОЕ');
  await remove.press('Enter');
  await expect(dialog.chips).toHaveCount(2);
  await expect(page.locator('#facilityStateDeleteConfirm')).toBeHidden();
  await dialog.cancel.click();

  // «Дежурный режим» is the reactor's current state but not its starting state.
  await openDeviceDialog(page, REACTOR);
  await dialog.chip('standby').click();
  expect(await blockedReason(remove)).toBe('УСТРОЙСТВО СЕЙЧАС В ЭТОМ СОСТОЯНИИ: СНАЧАЛА ПЕРЕВЕДИТЕ ЕГО В ДРУГОЕ');
  await remove.press('Enter');
  await expect(dialog.chips).toHaveCount(3);
  await expect(page.locator('#facilityStateDeleteConfirm')).toBeHidden();
  await expect(page.locator('#facilityStateUses')).toBeHidden();
  await dialog.cancel.click();
  expect(await saveCalls(page)).toBe(0);
});

test('a state used outside the device names every use and is not deleted (edge case, FR-008)', async ({ page, request }) => {
  const demo = await openDemoObjects(page, request);
  // Keep the grid at «Нет питания», so only the outside uses of «В работе» can refuse the delete.
  await forceState(request, GRID, 'offline');
  await openOverseer(page);
  const dialog = await openDeviceDialog(page, GRID);
  await dialog.chip('online').click();
  await page.locator('#btnDeleteFacilityState').click();

  const uses = await stateUses(page);
  const preconditionUses = demo.facility.devices.flatMap(device => (device.transitions || [])
    .filter(transition => (transition.preconditions || []).some(value => value.deviceId === GRID && value.stateId === 'online'))
    .map(transition => `Переход «${transition.name}» устройства «${device.name}» требует: Резервная сеть Убежища 76 = В работе.`));
  expect(preconditionUses.length).toBeGreaterThan(0);
  expect(uses.filter(use => use.destination === 'device').map(use => use.sentence)).toEqual(preconditionUses);
  expect(uses.filter(use => use.destination === 'rule')).toHaveLength(stateRuleCount(demo, GRID, 'online'));
  // «Запустить резервную сеть» goes with the state, and a command and a fault's recovery name it.
  expect(uses.filter(use => use.destination === 'command').map(use => use.sentence))
    .toEqual(['Команда «Запустить резервную сеть» запускает переход «Запустить резервную сеть».']);
  expect(uses.filter(use => use.destination === 'condition').map(use => use.sentence))
    .toEqual(['Неисправность «Резервная сеть обесточена» устраняется переходом «Запустить резервную сеть».']);
  const buttons = { device: 'К УСТРОЙСТВУ', rule: 'К ПРАВИЛУ ОТОБРАЖЕНИЯ', command: 'К КОМАНДЕ', condition: 'К НЕИСПРАВНОСТИ' };
  for (const use of uses) expect(use.button, use.sentence).toBe(buttons[use.destination]);
  await expect(page.locator('#facilityStateUses button').first()).toBeFocused();
  await expect(page.locator('#facilityStateDeleteConfirm')).toBeHidden();
  await expect(dialog.chips).toHaveCount(2);
  expect((await matrixCells(page)).map(cell => cell.count)).toEqual([1, 1]);

  // A use leads to its place: the door's transition opens in the door's dialog.
  await page.locator('#facilityStateUses li.facility-state-use[data-destination="device"]')
    .filter({ hasText: 'Открыть гермодверь' }).getByRole('button', { name: 'К УСТРОЙСТВУ', exact: true }).click();
  await expect(dialog.dialog).toBeVisible();
  await expect(dialog.name).toHaveValue(DOOR_NAME);
  await expect(openRow(page)).toHaveAttribute('data-transition-id', 'open');
  await expect(openRow(page).locator('.facility-transition-name')).toBeFocused();
  await dialog.cancel.click();

  // Through its transitions, the ventilation's «Очищен» is also used by a recovery program.
  await openDeviceDialog(page, VENTILATION);
  await dialog.chip('filtered').click();
  await page.locator('#btnDeleteFacilityState').click();
  const ventilationUses = await stateUses(page);
  expect(ventilationUses.filter(use => use.destination === 'rule')).toHaveLength(stateRuleCount(demo, VENTILATION, 'filtered'));
  expect(ventilationUses.filter(use => use.destination === 'program').map(use => use.sentence))
    .toEqual(['Программа восстановления «ВОЗДУХ-76» запускает переход «Продуть воздушный контур».']);
  await page.locator('#facilityStateUses li.facility-state-use[data-destination="program"]')
    .getByRole('button', { name: 'К ПРОГРАММЕ', exact: true }).click();
  await expect(dialog.dialog).toBeHidden();
  await expect(page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true })).toBeVisible();
  await expect(page.locator('#facilityRecoveryProgramName')).toHaveValue('ВОЗДУХ-76');
  expect(await saveCalls(page)).toBe(0);
  expect((await savedDevice(request, VENTILATION)).states.map(value => value.id)).toEqual(['contaminated', 'filtered']);
});

test('a new device hides its identifiers until the technical block opens, and its states and cells build the graph (AS5, SC-002, FR-003, FR-007)', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  await openOverseer(page);
  await page.locator('#btnAddFacilityDevice').click();
  const dialog = deviceDialog(page);
  await expect(dialog.dialog).toBeVisible();
  await expect(page.locator('#facilityDeviceDialogTitle')).toHaveText('УСТРОЙСТВО');
  await expect(dialog.technical).not.toHaveAttribute('open', '');
  expect(await visibleIdentifierFields(page)).toEqual([]);
  await expect(page.locator('.facility-transition-matrix-frame')).toBeHidden();
  await expect(page.locator('#facilityTransitionMatrixHint')).toHaveText('Добавьте состояния, чтобы задать переходы.');

  await dialog.name.fill('Дверь в убежище');
  await dialog.kind.selectOption('door');
  await addState(page, 'Заперта');
  await expect(dialog.chip('zaperta')).toHaveAttribute('data-initial', 'true');
  await addState(page, 'Открыта');
  await addState(page, 'Шлюз');
  expect(await stateChips(page)).toEqual([
    { id: 'zaperta', initial: true, name: 'Заперта' },
    { id: 'otkryta', initial: false, name: 'Открыта' },
    { id: 'shlyuz', initial: false, name: 'Шлюз' },
  ]);
  await expect(dialog.matrix.locator('thead th[scope="col"]')).toHaveText(['Заперта', 'Открыта', 'Шлюз']);
  await closeStateEditor(page);

  // An empty cell suggests the type's name for the pair, or one after the destination.
  const suggestions = [['zaperta', 'otkryta', 'Открыть'], ['otkryta', 'zaperta', 'Запереть'], ['otkryta', 'shlyuz', 'Перевести в «Шлюз»']];
  for (const [from, to, name] of suggestions) {
    await openTransitionCell(page, from, to);
    const input = openRow(page).locator('.facility-transition-name');
    await expect(input).toHaveValue(name);
    expect(await selectedText(input)).toBe(name);
    await expect(dialog.cell(from, to).locator('.facility-transition-cell-name')).toHaveText(name);
  }
  await page.locator('#btnCloseFacilityTransitionPanel').click();
  expect(await visibleIdentifierFields(page)).toEqual([]);

  await dialog.technical.locator('summary').click();
  await expect(dialog.technical).toHaveAttribute('open', '');
  await expect(dialog.technical.locator('summary')).toHaveText('ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ');
  await expect(page.locator('#facilityDeviceID')).toHaveValue('device-dver-v-ubezhishche');
  await expect(page.locator('#facilityDeviceID')).toBeVisible();
  expect(await values(dialog.technical.locator('.facility-state-row .facility-state-id')))
    .toEqual(['zaperta', 'otkryta', 'shlyuz']);
  await expect(dialog.technical.locator('.facility-state-row label')).toHaveText(['Заперта', 'Открыта', 'Шлюз']);
  await expect(dialog.technical.locator('.facility-transition-id-row label'))
    .toHaveText(['Открыть: Заперта → Открыта', 'Запереть: Открыта → Заперта', 'Перевести в «Шлюз»: Открыта → Шлюз']);
  expect(await values(dialog.technical.locator('.facility-transition-id-row .facility-transition-id')))
    .toEqual(['otkryt', 'zaperet', 'perevesti-v-shlyuz']);
  expect(await visibleIdentifierFields(page)).toHaveLength(7);

  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);
  expect(await saveCalls(page)).toBe(1);
  const saved = await savedDevice(request, 'device-dver-v-ubezhishche');
  expect(saved).toMatchObject({
    name: 'Дверь в убежище', kind: 'door', initialStateId: 'zaperta', currentStateId: 'zaperta',
    states: [{ id: 'zaperta', name: 'Заперта' }, { id: 'otkryta', name: 'Открыта' }, { id: 'shlyuz', name: 'Шлюз' }],
    transitions: [
      { id: 'otkryt', name: 'Открыть', sourceStateId: 'zaperta', destinationStateId: 'otkryta' },
      { id: 'zaperet', name: 'Запереть', sourceStateId: 'otkryta', destinationStateId: 'zaperta' },
      { id: 'perevesti-v-shlyuz', name: 'Перевести в «Шлюз»', sourceStateId: 'otkryta', destinationStateId: 'shlyuz' },
    ],
  });
});

test('deleting a state takes this device\'s transitions with it after confirmation (edge case, FR-008)', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  await openOverseer(page);
  await page.locator('#btnAddFacilityDevice').click();
  const dialog = deviceDialog(page);
  await dialog.name.fill('Дверь в убежище');
  await dialog.kind.selectOption('door');
  for (const name of ['Заперта', 'Открыта', 'Шлюз']) await addState(page, name);
  for (const [from, to] of [['zaperta', 'otkryta'], ['otkryta', 'zaperta'], ['otkryta', 'shlyuz'], ['shlyuz', 'zaperta']]) {
    await openTransitionCell(page, from, to);
  }
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);

  await page.locator('#btnEditFacilityDevice').click();
  await expect(dialog.dialog).toBeVisible();
  await dialog.chip('shlyuz').click();
  const remove = page.locator('#btnDeleteFacilityState');
  await remove.click();
  const confirm = page.locator('#facilityStateDeleteConfirm');
  await expect(confirm).toBeVisible();
  await expect(page.locator('#facilityStateDeleteSummary'))
    .toHaveText('Вместе с состоянием удалятся переходы: «Перевести в «Шлюз»», «Перевести в «Заперта»».');
  await expect(page.locator('#btnConfirmDeleteFacilityState')).toBeFocused();
  await expect(page.locator('#btnConfirmDeleteFacilityState')).toHaveText('УДАЛИТЬ С ПЕРЕХОДАМИ');
  await expect(page.locator('#facilityStateUses')).toBeHidden();

  // Cancelling changes nothing.
  await page.locator('#btnCancelDeleteFacilityState').click();
  await expect(confirm).toBeHidden();
  await expect(remove).toBeFocused();
  await expect(dialog.chips).toHaveCount(3);

  await remove.click();
  await page.locator('#btnConfirmDeleteFacilityState').click();
  expect((await stateChips(page)).map(chip => chip.id)).toEqual(['zaperta', 'otkryta']);
  expect(await matrixCells(page)).toEqual([
    { from: 'zaperta', to: 'otkryta', count: 1, text: 'Открыть' },
    { from: 'otkryta', to: 'zaperta', count: 1, text: 'Запереть' },
  ]);
  await expect(dialog.stateEditor).toBeHidden();
  await expect(dialog.addState).toBeFocused();
  expect(await saveCalls(page)).toBe(1);

  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);
  const saved = await savedDevice(request, 'device-dver-v-ubezhishche');
  expect(saved.states.map(value => value.id)).toEqual(['zaperta', 'otkryta']);
  expect(saved.transitions.map(value => value.id)).toEqual(['otkryt', 'zaperet']);
});

test('a device with one state has a single unavailable cell and says a transition needs a second state (edge case, FR-004)', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  await openOverseer(page);
  await page.locator('#btnAddFacilityDevice').click();
  const dialog = deviceDialog(page);
  await dialog.name.fill('Аварийный маяк');
  await dialog.kind.selectOption('custom');
  await addState(page, 'Мигает');
  await expect(page.locator('.facility-transition-matrix-frame')).toBeVisible();
  await expect(dialog.matrix.locator('td.facility-transition-cell-unavailable')).toHaveCount(1);
  await expect(dialog.unavailable('migaet', 'migaet')).toHaveText('—');
  await expect(dialog.cells).toHaveCount(0);
  await expect(page.locator('#facilityTransitionMatrixHint')).toHaveText('Для перехода нужно второе состояние.');
  await expect(page.locator('#facilityTransitionMatrixHint')).toBeVisible();

  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);
  const saved = await savedDevice(request, 'device-avariynyy-mayak');
  expect(saved).toMatchObject({ initialStateId: 'migaet', states: [{ id: 'migaet', name: 'Мигает' }] });
  expect(saved.transitions ?? []).toEqual([]);
});

test('an identifier conflict on «ГОТОВО» opens the technical block and focuses the marked row (AS6, FR-007)', async ({ page, request }) => {
  await openDemoObjects(page, request);
  await page.locator('#btnAddFacilityDevice').click();
  const dialog = deviceDialog(page);
  await dialog.name.fill('Дверь в убежище');
  await dialog.kind.selectOption('door');
  for (const name of ['Заперта', 'Открыта']) await addState(page, name);
  await closeStateEditor(page);

  await dialog.technical.locator('summary').click();
  const second = dialog.technical.locator('.facility-state-row').nth(1).locator('.facility-state-id');
  await second.fill('zaperta');
  await dialog.technical.locator('summary').click();
  await expect(dialog.technical).not.toHaveAttribute('open', '');

  await dialog.save.click();
  await expect(dialog.dialog).toBeVisible();
  await expect(dialog.error).toHaveText('ИСПРАВЬТЕ ОТМЕЧЕННОЕ: 1');
  await expect(dialog.technical).toHaveAttribute('open', '');
  await expect(second).toBeFocused();
  await expect(second).toHaveAttribute('aria-invalid', 'true');
  await expect(dialog.technical.locator('.facility-state-row').nth(1).locator('.facility-row-error'))
    .toHaveText('ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: zaperta');
  await expect(dialog.chips.nth(1)).toHaveAttribute('data-problem', 'true');
  expect(await saveCalls(page)).toBe(0);

  // The device identifier conflicts with a saved device.
  await second.fill('otkryta');
  await page.locator('#facilityDeviceID').fill(DOOR);
  await dialog.technical.locator('summary').click();
  await dialog.save.click();
  await expect(dialog.technical).toHaveAttribute('open', '');
  await expect(page.locator('#facilityDeviceID')).toBeFocused();
  await expect(page.locator('#facilityDeviceID')).toHaveAttribute('aria-invalid', 'true');
  await expect(dialog.error).toHaveText('Такой идентификатор устройства уже используется');
  expect(await saveCalls(page)).toBe(0);

  await page.locator('#facilityDeviceID').fill('device-dver-v-ubezhishche');
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expectSaved(page);
  expect((await savedDevice(request, 'device-dver-v-ubezhishche')).states.map(value => value.id)).toEqual(['zaperta', 'otkryta']);
});

test('every demo device shows its states and whole matrix without scrolling and no identifier at 1440×900 (SC-001, SC-002)', async ({ page, request }) => {
  const demo = await openDemoObjects(page, request);
  for (const device of demo.facility.devices) {
    const dialog = await openDeviceDialog(page, device.id);
    await expect(dialog.panel).toBeHidden();
    expect(await visibleIdentifierFields(page), device.id).toEqual([]);
    await expect(dialog.chips).toHaveCount(device.states.length);
    const layout = await page.locator('#facilityDeviceForm').evaluate(form => {
      const rect = element => {
        const box = element.getBoundingClientRect();
        return { top: box.top, bottom: box.bottom, left: box.left, right: box.right };
      };
      const frame = form.querySelector('.facility-transition-matrix-frame');
      return {
        scrollTop: form.scrollTop,
        form: rect(form),
        chips: rect(form.querySelector('#facilityDeviceStateChips')),
        matrix: rect(form.querySelector('#facilityTransitionMatrix')),
        actions: rect(form.querySelector('.facility-dialog-actions')),
        frameScrolls: frame.scrollHeight > frame.clientHeight + 1 || frame.scrollWidth > frame.clientWidth + 1,
        viewport: { width: innerWidth, height: innerHeight },
      };
    });
    expect(layout.scrollTop, device.id).toBe(0);
    expect(layout.frameScrolls, `${device.id} matrix frame scrolls`).toBe(false);
    const visibleTop = Math.max(0, layout.form.top);
    const visibleBottom = Math.min(layout.viewport.height, layout.form.bottom);
    for (const name of ['chips', 'matrix', 'actions']) {
      const box = layout[name];
      expect(box.top, `${device.id} ${name} top`).toBeGreaterThanOrEqual(visibleTop);
      expect(box.bottom, `${device.id} ${name} bottom`).toBeLessThanOrEqual(visibleBottom + 0.5);
      expect(box.left, `${device.id} ${name} left`).toBeGreaterThanOrEqual(Math.max(0, layout.form.left));
      expect(box.right, `${device.id} ${name} right`).toBeLessThanOrEqual(Math.min(layout.viewport.width, layout.form.right) + 0.5);
    }
    // The sticky actions do not cover the matrix.
    expect(layout.matrix.bottom, `${device.id} matrix under the actions`).toBeLessThanOrEqual(layout.actions.top + 0.5);
    await dialog.cancel.click();
    await expect(dialog.dialog).toBeHidden();
  }
});

test('with eight states the matrix scrolls in its own frame and the actions stay in view (edge case, SC-001)', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  await openOverseer(page);
  await page.locator('#btnAddFacilityDevice').click();
  const dialog = deviceDialog(page);
  await dialog.name.fill('Распределительный щит');
  await dialog.kind.selectOption('custom');
  for (let number = 1; number <= 8; number++) await addState(page, `Режим ${number}`);
  await closeStateEditor(page);
  await expect(dialog.cells).toHaveCount(8 * 7);

  const layout = await page.locator('#facilityDeviceForm').evaluate(form => {
    const frame = form.querySelector('.facility-transition-matrix-frame');
    const actions = form.querySelector('.facility-dialog-actions').getBoundingClientRect();
    return {
      frameScrolls: frame.scrollHeight > frame.clientHeight + 1,
      formScrolls: form.scrollHeight > form.clientHeight + 1,
      actionsBottom: actions.bottom,
      height: innerHeight,
      headerPosition: getComputedStyle(form.querySelector('#facilityTransitionMatrix thead th')).position,
      rowHeaderPosition: getComputedStyle(form.querySelector('#facilityTransitionMatrix tbody th')).position,
    };
  });
  expect(layout).toMatchObject({ frameScrolls: true, formScrolls: false, headerPosition: 'sticky', rowHeaderPosition: 'sticky' });
  expect(layout.actionsBottom).toBeLessThanOrEqual(layout.height);
  await expect(dialog.save).toBeInViewport();
});
