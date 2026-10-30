import { expect, test } from '@playwright/test';

import { blockedReason, expectSaved, redo, saveStatus, switchTab, undo } from './helpers/authoring-bar.mjs';
import {
  FIXTURE, facilityState, fixtureSession, openOverseer, seedDemo, seedSession,
} from './helpers/overseer-forms.mjs';

// User Story 1: one save model across ТЕРМИНАЛЫ and ОБЪЕКТЫ
// (specs/039-unified-facility-authoring/spec.md, contracts/overseer-ui.md).

const PLAYER_FIXTURE = '/__fixture/facility-player-state';
const TOKEN_KEY = 'fallout-terminal.player-token';
const PLAYER_CONVERGENCE_TIMEOUT_MS = 1_000;
const EXIT_FOLDER = 'ПРОТОКОЛ ВЫХОДА';
const DOOR = 'vault-door-main';
const DOOR_NAME = 'Гермодверь Убежища 76';
const EVACUATION = 'n_cmd_open_evacuation';
const EVACUATION_NAME = 'Открыть выход и подать сигнал';
// Any of these in the status means a draft or a stale-draft model survived.
const DRAFT_OR_STALE = /ЧЕРНОВИК|СЕССИЯ ИЗМЕНИЛАСЬ|ОТМЕНИТЕ|УСТАРЕЛ/i;
const REMOVED_COPY = [
  'ПРИМЕНИТЬ К ЧЕРНОВИКУ',
  'СОХРАНИТЬ ИЗМЕНЕНИЯ СЕССИИ',
  'СОХРАНИТЬ ЧЕРНОВИК СЦЕНАРИЯ',
  'СОХРАНИТЬ ЧЕРНОВИК И ОТКРЫТЬ ТЕРМИНАЛЫ',
  'ОТМЕНИТЬ ИЗМЕНЕНИЯ',
  'ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ',
  'СБРОСИТЬ ВСЕ СОСТОЯНИЯ',
  'СБРОСИТЬ ВЕСЬ ОБЪЕКТ',
];
const REMOVED_ELEMENTS = [
  'btnSaveFacility', 'btnDiscardFacility', 'btnResetFacility', 'facilityResetDialog', 'commandDraftGuard',
  'btnPublish',
];

function findNode(node, id) {
  if (node?.id === id) return node;
  for (const child of node?.children ?? []) {
    const found = findNode(child, id);
    if (found) return found;
  }
  return null;
}

async function savedNode(request, id) {
  const session = await fixtureSession(request);
  for (const terminal of session.terminals) {
    const node = findNode(terminal.root, id);
    if (node) return node;
  }
  return null;
}

async function savedDevice(request, id) {
  return (await fixtureSession(request)).facility.devices.find(device => device.id === id);
}

async function savedCondition(request, id) {
  return (await fixtureSession(request)).facility.conditions.find(condition => condition.id === id);
}

async function saveCalls(page) {
  return page.evaluate(() => window.__desktopFixture.calls.filter(call => call.method === 'SaveAuthoring').length);
}

function deviceRow(page, id) {
  return page.locator(`[data-facility-kind="device"][data-facility-id="${id}"]`);
}

// renameDevice applies a name through the device detail form.
async function renameDevice(page, id, name) {
  await deviceRow(page, id).click();
  await page.locator('#facilitySelectedDeviceName').fill(name);
  await page.locator('#btnSaveFacilityDeviceChanges').click();
}

function commandRow(page, authoredName) {
  return page.locator('.tree-row', { has: page.locator(`.tree-label[data-authored-name="${authoredName}"]`) });
}

async function openCommand(page, authoredName) {
  const row = commandRow(page, authoredName);
  if (!await row.isVisible()) await page.locator('.tree-row', { hasText: EXIT_FOLDER }).click();
  await row.click();
  await expect(page.locator('#fldName')).toHaveValue(authoredName);
}

async function editCommandText(page, authoredName, text) {
  await openCommand(page, authoredName);
  await page.locator('#fldText').fill(text);
  await page.locator('#btnApplyNode').click();
}

// statusOnBothTabs reads the shared status on the current tab and on the
// other one, expects them to match, and returns to the current tab (SC-002).
async function statusOnBothTabs(page) {
  const current = (await page.locator('[role="tab"][aria-selected="true"]').textContent()).trim();
  const other = current === 'ОБЪЕКТЫ' ? 'ТЕРМИНАЛЫ' : 'ОБЪЕКТЫ';
  await expect(page.locator('#authoringBar')).toBeVisible();
  const here = await saveStatus(page);
  await switchTab(page, other);
  await expect(page.locator('#authoringBar')).toBeVisible();
  const there = await saveStatus(page);
  await switchTab(page, current);
  expect(there).toEqual(here);
  return here;
}

async function holdSaves(page) {
  await page.evaluate(() => window.__desktopFixture.holdCalls('SaveAuthoring'));
}

async function releaseSaves(page) {
  await page.evaluate(() => window.__desktopFixture.releaseHeldCalls('SaveAuthoring'));
}

// rejectSave answers the nth SaveAuthoring request from now (1-based) with
// failure for that request, as the session store rejects one; the others
// reach the fixture.
async function rejectSave(page, request, { nth = 1, failure = 'invalid-configuration', issue = {} } = {}) {
  let seen = 0;
  await page.route(`**${FIXTURE}/save-authoring`, async route => {
    seen++;
    if (seen !== nth) {
      await route.continue();
      return;
    }
    const { correlationId } = route.request().postDataJSON();
    const state = await facilityState(request);
    await route.fulfill({ json: {
      ok: false, changed: false, correlationId, failure,
      issues: [{ code: failure, entityKind: 'session', ...issue }],
      sessionRevision: state.sessionRevision, authoringRevision: state.authoringRevision,
      previousFacilityRevision: state.facility.revision, resultingFacilityRevision: state.facility.revision,
      session: await fixtureSession(request),
    } });
  });
}

test('edits on both tabs both save, the status matches on both tabs, and undo and redo walk one history (independent test, AS1, AS5, AS7, AS8)', async ({ page, request }) => {
  const demo = await seedDemo(request);
  await openOverseer(page, 'ОБЪЕКТЫ');
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'idle');
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('НЕЧЕГО ОТМЕНЯТЬ');
  expect(await blockedReason(page.locator('#btnRedo'))).toBe('НЕЧЕГО ПОВТОРЯТЬ');

  await renameDevice(page, DOOR, 'Гермодверь 76');
  await expectSaved(page);
  await expect(page.locator('#saveStatus')).toHaveText(/^СОХРАНЕНО · \d{2}:\d{2}:\d{2}$/);
  await statusOnBothTabs(page);

  await switchTab(page, 'ТЕРМИНАЛЫ');
  await openCommand(page, EVACUATION_NAME);
  // A command with a device action has no draft guard (AS5, FR-006).
  await expect(page.locator('#commandDraftGuard')).toHaveCount(0);
  await expect(page.locator('#fldCommandMode')).toBeEnabled();
  await page.locator('#fldText').fill('Выход открыт. Сигнал эвакуации подан.');
  await page.locator('#btnApplyNode').click();
  await expectSaved(page);
  await expect.poll(async () => (await savedNode(request, EVACUATION)).text).toBe('Выход открыт. Сигнал эвакуации подан.');
  expect((await savedDevice(request, DOOR)).name).toBe('Гермодверь 76');

  await switchTab(page, 'ОБЪЕКТЫ');
  await expect(deviceRow(page, DOOR)).toContainText('Гермодверь 76');
  expect((await statusOnBothTabs(page)).text).not.toMatch(DRAFT_OR_STALE);
  await expect(page.locator('#facilityStatus')).not.toContainText(DRAFT_OR_STALE);

  // A live change between the edits: the spore fault is recovered.
  await deviceRow(page, 'ventilation-main').click();
  await page.locator('[data-facility-kind="condition"][data-facility-id="air-spore-bloom"]').click();
  await page.locator('#btnRecoverFacilityCondition').click();
  await page.locator('#btnConfirmFacilityRecovery').click();
  await expect(page.locator('#facilityStatus')).toHaveText('УСЛОВИЕ ВОССТАНОВЛЕНО');
  expect((await savedCondition(request, 'air-spore-bloom')).currentActive).toBe(false);

  // ОТМЕНИТЬ reverses newest first; each reversal is a save (AS7).
  await expect(page.locator('#btnUndoReason')).toHaveText(`ОТМЕНИТЬ: Команда «${EVACUATION_NAME}»`);
  await undo(page);
  await expect.poll(async () => (await savedNode(request, EVACUATION)).text)
    .toBe(findNode(demo.terminals[0].root, EVACUATION).text);
  expect((await savedDevice(request, DOOR)).name).toBe('Гермодверь 76');
  await expect(page.locator('#btnUndoReason')).toHaveText('ОТМЕНИТЬ: Устройство «Гермодверь 76»');
  await expect(page.locator('#btnRedoReason')).toHaveText(`ПОВТОРИТЬ: Команда «${EVACUATION_NAME}»`);
  await undo(page);
  await expect.poll(async () => (await savedDevice(request, DOOR)).name).toBe(DOOR_NAME);
  await expect(deviceRow(page, DOOR)).toContainText(DOOR_NAME);
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('НЕЧЕГО ОТМЕНЯТЬ');

  // Live state is never reversed: the fault stays recovered and the executed
  // commands stay executed.
  const live = await fixtureSession(request);
  expect(live.facility.conditions.find(condition => condition.id === 'air-spore-bloom').currentActive).toBe(false);
  expect(Object.keys(live.terminals[0].commandStates).sort()).toEqual(
    Object.keys(demo.terminals[0].commandStates).sort(),
  );

  // ПОВТОРИТЬ reapplies the newest reversed edit first (AS8).
  await expect(page.locator('#btnRedoReason')).toHaveText('ПОВТОРИТЬ: Устройство «Гермодверь 76»');
  await redo(page);
  await expect.poll(async () => (await savedDevice(request, DOOR)).name).toBe('Гермодверь 76');
  expect((await savedNode(request, EVACUATION)).text).toBe(findNode(demo.terminals[0].root, EVACUATION).text);
  await expect(page.locator('#btnRedoReason')).toHaveText(`ПОВТОРИТЬ: Команда «${EVACUATION_NAME}»`);

  // A new edit leaves nothing to reapply.
  await renameDevice(page, 'evacuation-alarm', 'Сирена эвакуации');
  await expectSaved(page);
  expect(await blockedReason(page.locator('#btnRedo'))).toBe('НЕЧЕГО ПОВТОРЯТЬ');
  expect(await saveCalls(page)).toBe(6);
  expect((await statusOnBothTabs(page)).text).not.toMatch(DRAFT_OR_STALE);
});

test('the status reads the same on both tabs while saving, with unapplied input, and after a rejected save (AS2, AS6, SC-002)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ОБЪЕКТЫ');
  const status = page.locator('#saveStatus');

  // Saving: the bar says so on both tabs, and controls that need the saved
  // session say why they wait (FR-005).
  await holdSaves(page);
  await renameDevice(page, DOOR, 'Гермодверь 76');
  await expect(status).toHaveAttribute('data-state', 'saving');
  await expect(status).toHaveText('СОХРАНЕНИЕ…');
  expect((await statusOnBothTabs(page)).state).toBe('saving');
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('ДОЖДИТЕСЬ СОХРАНЕНИЯ');
  expect(await blockedReason(page.locator('#btnResetScene'))).toBe('ДОЖДИТЕСЬ СОХРАНЕНИЯ');
  await page.locator('#btnResetScene').press('Enter');
  await expect(page.locator('#sceneResetDialog')).toBeHidden();
  // A preview opens at once and asks the server only after the save.
  const previewsBefore = (await facilityState(request)).previewCalls;
  await deviceRow(page, DOOR).click();
  await expect(page.locator('#btnPreviewFacilityDevice')).not.toHaveAttribute('aria-disabled', 'true');
  await page.locator('#btnPreviewFacilityDevice').click();
  const preview = page.locator('#facilityPreviewDialog');
  await expect(preview).toBeVisible();
  await expect(preview.locator('#facilityPreviewStatus')).toHaveText('ПОДГОТОВКА ПРЕДПРОСМОТРА…');
  expect((await facilityState(request)).previewCalls).toBe(previewsBefore);
  await releaseSaves(page);
  await expect(preview.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
  await expect(preview.locator('#facilityPreviewCurrentHeading')).toContainText('Гермодверь 76');
  expect((await facilityState(request)).previewCalls).toBe(previewsBefore + 1);
  await preview.getByRole('button', { name: 'ЗАКРЫТЬ', exact: true }).click();
  await expectSaved(page);
  const saved = (await statusOnBothTabs(page)).text;
  expect(saved).toMatch(/^СОХРАНЕНО · \d{2}:\d{2}:\d{2}$/);

  // Unapplied input is named on both tabs and kept across tab switches.
  await switchTab(page, 'ТЕРМИНАЛЫ');
  await openCommand(page, EVACUATION_NAME);
  await page.locator('#fldText').fill('Неприменённый результат');
  await expect(status).toHaveText(`${saved} · НЕ ПРИМЕНЕНО: Команда «${EVACUATION_NAME}»`);
  await expect(page.locator('#fldText')).toHaveAttribute('data-unapplied', 'true');
  await switchTab(page, 'ОБЪЕКТЫ');
  await deviceRow(page, 'evacuation-alarm').click();
  await page.locator('#facilitySelectedDeviceName').fill('Неприменённое название');
  await expect(status).toHaveAttribute('data-unapplied-count', '2');
  await expect(status).toHaveText(
    `${saved} · НЕ ПРИМЕНЕНО: Команда «${EVACUATION_NAME}», Устройство «Сигнал эвакуации»`,
  );
  await statusOnBothTabs(page);
  await switchTab(page, 'ТЕРМИНАЛЫ');
  await expect(page.locator('#fldText')).toHaveValue('Неприменённый результат');
  await expect(page.locator('#fldText')).toHaveAttribute('data-unapplied', 'true');
  await page.locator('#btnCancelNodeInput').click();
  await expect(page.locator('#fldText')).not.toHaveValue('Неприменённый результат');
  await expect(status).toHaveAttribute('data-unapplied-count', '1');
  await switchTab(page, 'ОБЪЕКТЫ');
  await expect(page.locator('#facilitySelectedDeviceName')).toHaveValue('Неприменённое название');
  await page.locator('#btnCancelFacilityDeviceChanges').click();
  await expect(page.locator('#facilitySelectedDeviceName')).toHaveValue('Сигнал эвакуации');
  await expect(status).toHaveText(saved);
  const before = await facilityState(request);

  // A rejected save shows its reason on both tabs and keeps the input (AS6).
  await rejectSave(page, request, { issue: { entityKind: 'device', entityId: 'evacuation-alarm' } });
  await renameDevice(page, 'evacuation-alarm', 'Сирена эвакуации');
  await expect(status).toHaveAttribute('data-state', 'error');
  await expect(status).toHaveText(
    'ОШИБКА СОХРАНЕНИЯ: ДЕЙСТВИЕ ОБЪЕКТА НАСТРОЕНО НЕКОРРЕКТНО · device · evacuation-alarm'
    + ' · НЕ ПРИМЕНЕНО: Устройство «Сигнал эвакуации»',
  );
  expect((await statusOnBothTabs(page)).state).toBe('error');
  await expect(deviceRow(page, 'evacuation-alarm')).toContainText('Сигнал эвакуации');
  await expect(page.locator('#facilitySelectedDeviceName')).toHaveValue('Сирена эвакуации');
  await expect(page.locator('#facilitySelectedDeviceName')).toHaveAttribute('data-unapplied', 'true');
  expect((await facilityState(request)).facility).toEqual(before.facility);
});

test('ten alternating edits across tabs all save with no stale message, and a burst keeps one save in flight (SC-001)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ОБЪЕКТЫ');
  const devices = ['power-grid-main', DOOR, 'evacuation-alarm', 'network-overseer', 'security-turret'];
  const commands = [
    ['n_cmd_ordinary_1', 'Проверить наружные датчики'],
    [EVACUATION, EVACUATION_NAME],
    ['n_cmd_air_recovery', 'Загрузить кассету очистки воздуха'],
    ['n_cmd_1', 'Активировать маршрут выхода'],
    ['n_cmd_prime_grid', 'Запустить резервную сеть'],
  ];
  for (let index = 0; index < 5; index++) {
    await switchTab(page, 'ОБЪЕКТЫ');
    await renameDevice(page, devices[index], `Устройство ${index + 1} · проверка`);
    await expectSaved(page);
    expect((await statusOnBothTabs(page)).text).not.toMatch(DRAFT_OR_STALE);

    await switchTab(page, 'ТЕРМИНАЛЫ');
    await editCommandText(page, commands[index][1], `Результат ${index + 1} · проверка`);
    await expectSaved(page);
    expect((await statusOnBothTabs(page)).text).not.toMatch(DRAFT_OR_STALE);
  }
  expect(await saveCalls(page)).toBe(10);
  const session = await fixtureSession(request);
  for (const [index, id] of devices.entries()) {
    expect(session.facility.devices.find(device => device.id === id).name).toBe(`Устройство ${index + 1} · проверка`);
  }
  for (const [index, [id]] of commands.entries()) {
    expect(findNode(session.terminals[0].root, id).text).toBe(`Результат ${index + 1} · проверка`);
  }

  // A burst of five edits: one request at a time, in order.
  await switchTab(page, 'ОБЪЕКТЫ');
  await holdSaves(page);
  for (let index = 1; index <= 5; index++) await renameDevice(page, DOOR, `Гермодверь · ${index}`);
  await expect(deviceRow(page, DOOR)).toContainText('Гермодверь · 5');
  await expect.poll(() => page.evaluate(() => window.__desktopFixture.inFlightCount('SaveAuthoring'))).toBe(1);
  expect(await saveCalls(page)).toBe(11);
  await releaseSaves(page);
  await expectSaved(page);
  expect(await saveCalls(page)).toBe(15);
  expect(await page.evaluate(() => window.__desktopFixture.peakInFlight('SaveAuthoring'))).toBe(1);
  const names = await page.evaluate(() => window.__desktopFixture.calls.filter(call => call.method === 'SaveAuthoring')
    .slice(-5).map(call => call.args[0].session.facility.devices.find(device => device.id === 'vault-door-main').name));
  expect(names).toEqual([1, 2, 3, 4, 5].map(index => `Гермодверь · ${index}`));
  expect((await savedDevice(request, DOOR)).name).toBe('Гермодверь · 5');
  await expect(page.locator('#btnUndoReason')).toHaveText('ОТМЕНИТЬ: Устройство «Гермодверь · 5»');
});

test('of two quick edits the valid one saves and the rejected one reopens its dialog with the input (SC-001, AS6)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ОБЪЕКТЫ');
  await holdSaves(page);
  await renameDevice(page, DOOR, 'Гермодверь 76');

  await deviceRow(page, 'power-grid-main').click();
  await page.getByRole('button', { name: 'РЕДАКТИРОВАТЬ ГРАФ', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
  await dialog.locator('.facility-state-name').first().fill('Обесточена · проверка');
  await dialog.getByRole('button', { name: 'ГОТОВО', exact: true }).click();
  await expect(dialog).toBeHidden();

  await rejectSave(page, request, { nth: 2, issue: { entityKind: 'device', entityId: 'power-grid-main' } });
  await releaseSaves(page);
  const reason = 'ДЕЙСТВИЕ ОБЪЕКТА НАСТРОЕНО НЕКОРРЕКТНО · device · power-grid-main';
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#facilityDeviceError')).toHaveText(reason);
  await expect(dialog.locator('.facility-state-name').first()).toHaveValue('Обесточена · проверка');
  await expect(page.locator('#saveStatus')).toHaveText(`ОШИБКА СОХРАНЕНИЯ: ${reason}`);
  expect((await savedDevice(request, DOOR)).name).toBe('Гермодверь 76');
  expect((await savedDevice(request, 'power-grid-main')).states[0].name).toBe('Нет питания');

  await page.unroute(`**${FIXTURE}/save-authoring`);
  await dialog.getByRole('button', { name: 'ГОТОВО', exact: true }).click();
  await expect(dialog).toBeHidden();
  await expectSaved(page);
  expect((await savedDevice(request, 'power-grid-main')).states[0].name).toBe('Обесточена · проверка');
  expect((await savedDevice(request, DOOR)).name).toBe('Гермодверь 76');
});

test('live changes never reject a save; a stale save is reapplied with «ПРИМЕНИТЬ СНОВА», and undo says why it no longer applies (FR-007)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ОБЪЕКТЫ');
  await renameDevice(page, DOOR, 'Гермодверь 76');
  await expectSaved(page);

  // A live device reset between edits does not make the next save stale.
  await deviceRow(page, 'power-grid-main').click();
  await page.locator('#btnResetFacilityDevice').click();
  await page.locator('#btnConfirmFacilityDeviceReset').click();
  await expect(page.locator('#facilityStatus')).toHaveText('УСТРОЙСТВО СБРОШЕНО');
  await renameDevice(page, 'evacuation-alarm', 'Сирена эвакуации');
  await expectSaved(page);
  expect((await savedDevice(request, 'evacuation-alarm')).name).toBe('Сирена эвакуации');

  // Another author changes the authored configuration.
  const external = await fixtureSession(request);
  external.facility.devices.find(device => device.id === 'security-turret').name = 'Турель · внешняя правка';
  await seedSession(request, external);

  await renameDevice(page, 'network-overseer', 'Сеть Смотрителя');
  const status = page.locator('#saveStatus');
  await expect(status).toHaveAttribute('data-state', 'error');
  await expect(status).toHaveText('ОШИБКА СОХРАНЕНИЯ: СЕССИЯ ИЗМЕНИЛАСЬ ДРУГИМ ДЕЙСТВИЕМ — ПРАВКА НЕ ПРИМЕНЕНА');
  const reapply = page.locator('#btnReapplyEdit');
  await expect(reapply).toBeVisible();
  await expect(reapply).toHaveText('ПРИМЕНИТЬ СНОВА');
  await expect(deviceRow(page, 'security-turret')).toContainText('Турель · внешняя правка');
  await expect(deviceRow(page, 'network-overseer')).toContainText('Служебная сеть Смотрителя');
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('СЕССИЯ ИЗМЕНИЛАСЬ ПОСЛЕ ЭТОЙ ПРАВКИ');

  await reapply.click();
  await expectSaved(page);
  await expect(reapply).toBeHidden();
  expect((await savedDevice(request, 'network-overseer')).name).toBe('Сеть Смотрителя');
  expect((await savedDevice(request, 'security-turret')).name).toBe('Турель · внешняя правка');

  // Undo reverses the reapplied edit on the newer session, then says why the
  // older edit can no longer be undone instead of skipping it.
  await expect(page.locator('#btnUndoReason')).toHaveText('ОТМЕНИТЬ: Устройство «Сеть Смотрителя»');
  await undo(page);
  expect((await savedDevice(request, 'network-overseer')).name).toBe('Служебная сеть Смотрителя');
  expect((await savedDevice(request, 'security-turret')).name).toBe('Турель · внешняя правка');
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('СЕССИЯ ИЗМЕНИЛАСЬ ПОСЛЕ ЭТОЙ ПРАВКИ');
  await page.locator('#btnUndo').press('Enter');
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'saved');
  expect(await saveCalls(page)).toBe(5);
  expect((await savedDevice(request, 'evacuation-alarm')).name).toBe('Сирена эвакуации');
  // The live reset is never reversed.
  const power = await savedDevice(request, 'power-grid-main');
  expect(power.currentStateId).toBe(power.initialStateId);
});

test('opening a session clears the history, and the history keeps the 20 newest edits (FR-001a)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ОБЪЕКТЫ');
  for (let index = 1; index <= 21; index++) await renameDevice(page, DOOR, `Гермодверь · ${index}`);
  await expectSaved(page);
  expect(await saveCalls(page)).toBe(21);
  for (let index = 20; index >= 1; index--) {
    await expect(page.locator('#btnUndoReason')).toHaveText(`ОТМЕНИТЬ: Устройство «Гермодверь · ${index + 1}»`);
    await undo(page);
  }
  await expect(deviceRow(page, DOOR)).toContainText('Гермодверь · 1');
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('ИСТОРИЯ ХРАНИТ 20 ПОСЛЕДНИХ ПРАВОК');
  await expect(page.locator('#btnRedoReason')).toHaveText('ПОВТОРИТЬ: Устройство «Гермодверь · 2»');

  await deviceRow(page, 'evacuation-alarm').click();
  await page.locator('#facilitySelectedDeviceName').fill('Неприменённое название');
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-unapplied-count', '1');

  // Opening a session again starts with no history and no unapplied input.
  await page.locator('#btnOpenSession').evaluate(button => button.click());
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'idle');
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-unapplied-count', '0');
  expect(await blockedReason(page.locator('#btnUndo'))).toBe('НЕЧЕГО ОТМЕНЯТЬ');
  expect(await blockedReason(page.locator('#btnRedo'))).toBe('НЕЧЕГО ПОВТОРЯТЬ');
});

test('every facility dialog that edits data confirms with «ГОТОВО», and no removed save wording remains (AS3, SC-003, FR-004)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ОБЪЕКТЫ');
  await deviceRow(page, DOOR).click();
  await expect(page.locator('#btnSaveFacilityDeviceChanges')).toHaveText('ГОТОВО');

  const editingDialogs = [
    ['new device', () => page.locator('#btnAddFacilityDevice').click(), 'facilityDeviceDialog'],
    ['device graph', () => page.getByRole('button', { name: 'РЕДАКТИРОВАТЬ ГРАФ', exact: true }).click(), 'facilityDeviceDialog'],
    ['condition', () => page.locator('#btnAddFacilityCondition').click(), 'facilityConditionDialog'],
    ['recovery program', () => page.locator('#btnAddFacilityRecoveryProgram').click(), 'facilityRecoveryProgramDialog'],
    ['new binding', () => page.locator('#btnAddFacilityBinding').click(), 'facilityBindingDialog'],
    ['existing binding', () => page.locator('#facilityBindingList [data-facility-kind="binding"]').first()
      .getByRole('button', { name: /^Изменить:/ }).click(), 'facilityBindingDialog'],
  ];
  for (const [name, open, id] of editingDialogs) {
    await open();
    const dialog = page.locator(`#${id}`);
    await expect(dialog, name).toBeVisible();
    await expect(dialog.locator('button[type="submit"]'), name).toHaveText(['ГОТОВО']);
    await dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
    await expect(dialog, name).toBeHidden();
  }

  // Confirmation-only dialogs run live operations and keep their verbs.
  await page.locator('#btnResetFacilityDevice').click();
  await expect(page.locator('#btnConfirmFacilityDeviceReset')).toHaveText('СБРОСИТЬ');
  await page.locator('#btnCancelFacilityDeviceReset').click();
  await page.locator('#btnResetScene').click();
  await expect(page.locator('#btnConfirmSceneReset')).toHaveText('СБРОСИТЬ СЦЕНУ');
  await page.locator('#btnCancelSceneReset').click();
  expect(await saveCalls(page)).toBe(0);

  for (const id of REMOVED_ELEMENTS) await expect(page.locator(`#${id}`), id).toHaveCount(0);
  const sweep = async context => {
    const html = await page.content();
    for (const copy of REMOVED_COPY) expect(html, `${copy} in ${context}`).not.toContain(copy);
    expect(html, `ЧЕРНОВИК in ${context}`).not.toMatch(/черновик/i);
  };
  await sweep('ОБЪЕКТЫ');
  await switchTab(page, 'ТЕРМИНАЛЫ');
  await openCommand(page, EVACUATION_NAME);
  await sweep('ТЕРМИНАЛЫ');
  for (const source of [`${FIXTURE}/overseer`, '/__fixture/overseer.js']) {
    const text = await (await request.get(source)).text();
    for (const copy of REMOVED_COPY) expect(text, `${copy} in ${source}`).not.toContain(copy);
    expect(text, `ЧЕРНОВИК in ${source}`).not.toMatch(/черновик/i);
  }
});

test('reset, preview, and private-recovery controls run or say on the control why not (AS4, FR-005)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ОБЪЕКТЫ');
  await deviceRow(page, DOOR).click();
  for (const id of ['btnPreviewFacilityDevice', 'btnResetFacilityDevice', 'btnResetScene']) {
    await expect(page.locator(`#${id}`), id).not.toHaveAttribute('aria-disabled', 'true');
  }

  // An inactive fault cannot be recovered; the control stays focusable and
  // its visible reason says why.
  await page.locator('[data-facility-kind="condition"][data-facility-id="door-offline"]').click();
  const recover = page.locator('#btnRecoverFacilityCondition');
  expect(await blockedReason(recover)).toBe('НЕИСПРАВНОСТЬ НЕ АКТИВНА');
  await expect(page.locator('#btnRecoverFacilityConditionReason')).toBeVisible();
  await recover.focus();
  await expect(recover).toBeFocused();
  await recover.press('Enter');
  await expect(page.locator('#facilityRecoveryConfirmationDialog')).toBeHidden();
  await expect(page.locator('#btnPreviewFacilityCondition')).not.toHaveAttribute('aria-disabled', 'true');

  // An active fault with operator recovery runs.
  await deviceRow(page, 'ventilation-main').click();
  await page.locator('[data-facility-kind="condition"][data-facility-id="air-spore-bloom"]').click();
  await expect(recover).not.toHaveAttribute('aria-disabled', 'true');
  await recover.click();
  await expect(page.locator('#facilityRecoveryConfirmationDialog')).toBeVisible();
  await page.locator('#btnConfirmFacilityRecovery').click();
  await expect(page.locator('#facilityStatus')).toHaveText('УСЛОВИЕ ВОССТАНОВЛЕНО');
  expect(await blockedReason(recover)).toBe('НЕИСПРАВНОСТЬ НЕ АКТИВНА');
});

test('the single-command reset says on the control why it waits for another reset (FR-005)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  await openCommand(page, 'Активировать маршрут выхода');
  const reset = page.locator('#btnResetCommandState');
  const confirmation = page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ СБРОСА' });
  await expect(reset).not.toHaveAttribute('aria-disabled', 'true');

  await page.evaluate(() => window.__desktopFixture.holdCalls('ResetCommandState'));
  await reset.click();
  await confirmation.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
  expect(await blockedReason(reset)).toBe('ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА');
  await expect(page.locator('#btnResetCommandStateReason')).toBeVisible();
  await reset.focus();
  await expect(reset).toBeFocused();
  await reset.press('Enter');
  await expect(confirmation).toBeHidden();

  await page.evaluate(() => window.__desktopFixture.releaseHeldCalls('ResetCommandState'));
  await expect(reset).not.toHaveAttribute('aria-disabled', 'true');
  await expect(page.locator('#btnResetCommandStateReason')).toBeHidden();
});

test('a rejected terminal dialog edit reopens the dialog with its input and the reason (AS6)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page, 'ТЕРМИНАЛЫ');
  const reason = 'ДЕЙСТВИЕ ОБЪЕКТА НАСТРОЕНО НЕКОРРЕКТНО · session';

  // The entry-block command dialog, creating a new command for block 4.
  await page.locator('.tree-row', { hasText: 'ДИРЕКТИВЫ VAULT-TEC' }).click();
  await page.locator('.tree-row', { hasText: 'СВОДКА ЭВАКУАЦИИ' }).click();
  await page.locator('[data-entry-block-action="configure-command"]').nth(3).click();
  const blockCommand = page.locator('#entryBlockCommandDialog');
  await expect(blockCommand).toBeVisible();
  await blockCommand.locator('input[name="entryBlockCommandMode"][value="new"]').check();
  await page.locator('#entryBlockNewCommandName').fill('Запечатать архив');
  await page.locator('#entryBlockNewCompletedName').fill('Архив запечатан');
  await page.locator('#entryBlockNewConfirmationText').fill('Запечатать архив эвакуации?');
  await page.locator('#entryBlockNewResultText').fill('АРХИВ ЗАПЕЧАТАН.');
  await page.locator('#entryBlockNewDestinationFolder').selectOption({ index: 1 });
  await page.locator('#entryBlockCompletedText').fill('АРХИВ ЭВАКУАЦИИ ЗАПЕЧАТАН');
  await rejectSave(page, request);
  await blockCommand.locator('[data-entry-block-command-action="apply"]').click();
  await expect(page.locator('#entryBlockCommandError')).toHaveText(reason);
  await expect(blockCommand).toBeVisible();
  await expect(page.locator('#entryBlockNewCommandName')).toHaveValue('Запечатать архив');
  await expect(page.locator('#entryBlockCompletedText')).toHaveValue('АРХИВ ЭВАКУАЦИИ ЗАПЕЧАТАН');
  await expect(page.locator('#saveStatus')).toHaveText(`ОШИБКА СОХРАНЕНИЯ: ${reason}`);

  await page.unroute(`**${FIXTURE}/save-authoring`);
  await blockCommand.locator('[data-entry-block-command-action="apply"]').click();
  await expect(blockCommand).toBeHidden();
  await expectSaved(page);
  await expect.poll(async () => (await fixtureSession(request)).terminals
    .some(terminal => JSON.stringify(terminal.root).includes('Запечатать архив'))).toBe(true);

  // The create-terminal dialog.
  await rejectSave(page, request);
  await page.locator('#btnAddTerminal').click();
  const create = page.locator('#createTerminalDialog');
  await page.locator('#createTerminalName').fill('Пост охраны');
  await page.locator('#btnConfirmCreateTerminal').click();
  await expect(page.locator('#createTerminalError')).toHaveText(reason);
  await expect(create).toBeVisible();
  await expect(page.locator('#createTerminalName')).toHaveValue('Пост охраны');

  await page.unroute(`**${FIXTURE}/save-authoring`);
  await page.locator('#btnConfirmCreateTerminal').click();
  await expect(create).toBeHidden();
  await expectSaved(page);
  expect((await fixtureSession(request)).terminals.map(terminal => terminal.name)).toContain('Пост охраны');
});

async function openParticipant(browser) {
  const context = await browser.newContext();
  await context.addInitScript(tokenKey => {
    localStorage.removeItem(tokenKey);
    HTMLMediaElement.prototype.play = () => Promise.resolve();
  }, TOKEN_KEY);
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  if (await page.locator('#characterSelect').isVisible()) {
    await page.locator('#characterOptions button:not([disabled])').first().click();
  }
  return { context, page };
}

async function applyNodeName(overseer, name) {
  await overseer.locator('#fldName').fill(name);
  await overseer.locator('#btnApplyNode').click();
  await expectSaved(overseer);
}

test('players see each applied edit at once and keep their place and hacking progress (FR-001c)', async ({ browser, request }) => {
  const reset = await request.post(`${PLAYER_FIXTURE}/reset`, { data: { scenario: 'ready' } });
  expect(reset.status()).toBe(204);
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  const players = [];
  try {
    await overseer.goto(`${PLAYER_FIXTURE}/overseer`);
    await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ', exact: true }).click();
    await expect(overseer.locator('#mainLayout')).toBeVisible();
    players.push(await openParticipant(browser), await openParticipant(browser));
    const [controller, observer] = players.map(participant => participant.page);
    await expect(controller.locator('#roleBadge')).toContainText('АКТИВЕН');
    await expect(observer.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
    await expect(controller.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' })).toBeVisible();

    // A new folder and record reach players at once, with no publish step.
    await expect(overseer.locator('#btnPublish')).toHaveCount(0);
    await overseer.locator('#btnAddFolder').click();
    await applyNodeName(overseer, 'АРХИВ ОХРАНЫ');
    for (const player of [controller, observer]) {
      await expect(player.locator('.term-row', { hasText: 'АРХИВ ОХРАНЫ' }))
        .toBeVisible({ timeout: PLAYER_CONVERGENCE_TIMEOUT_MS });
    }
    await overseer.locator('#btnAddEntry').click();
    await applyNodeName(overseer, 'ЖУРНАЛ СМЕНЫ');
    await controller.locator('.term-row', { hasText: 'АРХИВ ОХРАНЫ' }).click();
    for (const player of [controller, observer]) {
      await expect(player.locator('.term-row', { hasText: 'ЖУРНАЛ СМЕНЫ' })).toBeVisible();
    }
    // Players stay inside the folder while it is renamed under them.
    await applyNodeName(overseer, 'ЖУРНАЛ НОЧНОЙ СМЕНЫ');
    for (const player of [controller, observer]) {
      await expect(player.locator('.term-row', { hasText: 'ЖУРНАЛ НОЧНОЙ СМЕНЫ' }))
        .toBeVisible({ timeout: PLAYER_CONVERGENCE_TIMEOUT_MS });
      await expect(player.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' })).toHaveCount(0);
    }

    // A hacking terminal: an applied edit keeps the puzzle and its progress.
    await overseer.locator('#btnAddTerminal').click();
    await overseer.locator('#createTerminalName').fill('Архив смотрителя');
    await overseer.getByRole('dialog', { name: 'СОЗДАТЬ ТЕРМИНАЛ' })
      .getByRole('button', { name: 'СОЗДАТЬ ТЕРМИНАЛ' }).click();
    await expect(overseer.locator('#editingTermName')).toHaveText('Архив смотрителя');
    await expectSaved(overseer);
    await overseer.locator('#hackLevelSelect').selectOption('1');
    await overseer.locator('#btnApplySettings').click();
    await expectSaved(overseer);
    const session = await (await request.get(`${PLAYER_FIXTURE}/session`)).json();
    const archive = session.terminals.find(terminal => terminal.name === 'Архив смотрителя');
    expect(archive.hackLevel).toBe(1);
    const activated = await request.post(`${PLAYER_FIXTURE}/activate-terminal`, { data: { terminalId: archive.id } });
    expect(activated.status()).toBe(204);
    for (const player of [controller, observer]) await expect(player.locator('#hackHeader')).toBeVisible();
    const initialAttempts = await controller.locator('#attemptsLine').textContent();
    await controller.locator('.hcell.word').first().click();
    await expect.poll(() => controller.locator('#attemptsLine').textContent()).not.toBe(initialAttempts);
    const attempts = await controller.locator('#attemptsLine').textContent();
    const log = await controller.locator('#hackLog').textContent();

    await overseer.locator('#btnAddEntry').click();
    await applyNodeName(overseer, 'ПРОТОКОЛ ДОСТУПА');
    for (const player of [controller, observer]) {
      await expect(player.locator('#hackHeader')).toBeVisible();
      await expect(player.locator('#attemptsLine')).toHaveText(attempts);
    }
    await expect(controller.locator('#hackLog')).toHaveText(log);

    expect((await request.post('/__fixture/force-hack')).ok()).toBe(true);
    for (const player of [controller, observer]) {
      await expect(player.locator('#hackHeader')).toBeHidden({ timeout: 5_000 });
      await expect(player.locator('.term-row', { hasText: 'ПРОТОКОЛ ДОСТУПА' })).toBeVisible();
    }
  } finally {
    for (const participant of players) await participant.context.close();
    await overseerContext.close();
  }
});
