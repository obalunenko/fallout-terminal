import { expect, test } from '@playwright/test';

import { FACILITY_IDS } from './fixtures/facility-session.mjs';
import { expectSaved } from './helpers/authoring-bar.mjs';

const FIXTURE = '/__fixture/facility-authoring';
const DONE = 'ГОТОВО';

async function resetFixture(request, scenario = 'authored') {
  const response = await request.post(`${FIXTURE}/reset`, { data: { scenario } });
  expect(response.status()).toBe(204);
}

async function openFacilityWorkspace(page) {
  await page.goto(`${FIXTURE}/overseer`);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
  await page.getByRole('tab', { name: 'ОБЪЕКТЫ' }).click();
  await expect(page.locator('#facilityWorkspace')).toBeVisible();
}

async function facilityState(request) {
  const response = await request.get(`${FIXTURE}/state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

function rowById(page, kind, id) {
  return page.locator(`[data-facility-kind="${kind}"][data-facility-id="${id}"]`);
}

// Every applied facility edit autosaves through SaveAuthoring; the shared bar
// reports it once the save queue is idle.
async function expectAutosaved(page, request, saveCalls) {
  await expect.poll(async () => (await facilityState(request)).saveCalls).toBe(saveCalls);
  await expectSaved(page);
  await expect(page.locator('#saveStatus')).toHaveText(/^СОХРАНЕНО · \d{2}:\d{2}:\d{2}$/);
}

test('authors a complete reusable device graph, bindings, condition, and recovery program without JSON', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);

  await page.getByRole('button', { name: 'ДОБАВИТЬ УСТРОЙСТВО' }).click();
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА' });
  await device.getByLabel('ИДЕНТИФИКАТОР').fill('device-aux-reactor');
  await device.getByLabel('НАЗВАНИЕ').fill('Auxiliary reactor');
  await device.getByLabel('ТИП').selectOption('reactor');

  for (const [id, name] of [['offline', 'Offline'], ['online', 'Online']]) {
    await device.getByRole('button', { name: 'ДОБАВИТЬ СОСТОЯНИЕ' }).click();
    const state = device.locator('.facility-state-row').last();
    await state.getByLabel('ИДЕНТИФИКАТОР СОСТОЯНИЯ').fill(id);
    await state.getByLabel('НАЗВАНИЕ СОСТОЯНИЯ').fill(name);
  }
  await device.getByLabel('НАЧАЛЬНОЕ СОСТОЯНИЕ').selectOption('offline');
  await device.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД' }).click();
  const transition = device.locator('.facility-transition-row').last();
  await transition.getByLabel('ИДЕНТИФИКАТОР ПЕРЕХОДА').fill('start');
  await transition.getByLabel('НАЗВАНИЕ ПЕРЕХОДА').fill('Start auxiliary reactor');
  await transition.getByLabel('ИЗ СОСТОЯНИЯ').selectOption('offline');
  await transition.getByLabel('В СОСТОЯНИЕ').selectOption('online');
  await device.getByRole('button', { name: DONE }).click();
  await expect(device).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 1);

  await page.getByRole('button', { name: 'ДОБАВИТЬ УСЛОВИЕ' }).click();
  const condition = page.getByRole('dialog', { name: 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ' });
  await condition.getByLabel('ИДЕНТИФИКАТОР').fill('condition-aux-unpowered');
  await condition.getByLabel('НАЗВАНИЕ').fill('Auxiliary reactor unpowered');
  await condition.getByLabel('КАТЕГОРИЯ').selectOption('unpowered');
  await condition.getByLabel('УСТРОЙСТВО').selectOption('device-aux-reactor');
  await condition.getByLabel('БЛОКИРУЕМАЯ ВОЗМОЖНОСТЬ').selectOption('execute-command');
  await condition.getByLabel('ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ').check();
  await condition.getByRole('button', { name: DONE }).click();
  await expectAutosaved(page, request, before.saveCalls + 2);

  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ' }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ' });
  await program.getByLabel('ИДЕНТИФИКАТОР').fill('program-aux-start');
  await program.getByLabel('НАЗВАНИЕ').fill('Auxiliary startup');
  await program.getByLabel('УСТРОЙСТВО').selectOption('device-aux-reactor');
  await program.getByLabel('ПЕРЕХОД').selectOption('start');
  await program.getByRole('button', { name: DONE }).click();
  await expectAutosaved(page, request, before.saveCalls + 3);

  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРИВЯЗКУ' }).click();
  const binding = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА' });
  await binding.getByLabel('ТЕРМИНАЛ').selectOption(FACILITY_IDS.terminals.reactor);
  await binding.getByLabel('ЭЛЕМЕНТ').selectOption(FACILITY_IDS.nodes.startReactor);
  await binding.getByLabel('УСТРОЙСТВО').selectOption('device-aux-reactor');
  await binding.getByLabel('СОСТОЯНИЕ').selectOption('online');
  await binding.getByLabel('ТЕКСТ').fill('AUXILIARY REACTOR ONLINE');
  await binding.getByRole('button', { name: DONE }).click();
  await expectAutosaved(page, request, before.saveCalls + 4);

  const saved = await facilityState(request);
  expect(await page.evaluate(() => window.__desktopFixture.calls
    .filter(call => call.method === 'SaveAuthoring').length)).toBe(4);
  expect(saved.facility.devices).toContainEqual(expect.objectContaining({
    id: 'device-aux-reactor', initialStateId: 'offline', currentStateId: 'offline',
  }));
  expect(saved.facility.conditions).toContainEqual(expect.objectContaining({ id: 'condition-aux-unpowered' }));
  expect(saved.facility.recoveryPrograms).toContainEqual(expect.objectContaining({ id: 'program-aux-start' }));
  expect(saved.bindingCount).toBeGreaterThan(0);

  await page.reload();
  await openFacilityWorkspace(page);
  await expect(rowById(page, 'device', 'device-aux-reactor')).toContainText('Auxiliary reactor');
});

test('dependency inspection protects stable identity and applies one complete reassignment repair', async ({ page, request }) => {
  await resetFixture(request, 'referenced-device');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);
  const device = rowById(page, 'device', FACILITY_IDS.devices.power);
  await device.click();
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ' }).click();

  const report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ ОБЪЕКТА' });
  await expect(report).toContainText('FACILITY STATUS');
  await expect(report).toContainText('START MAIN REACTOR');
  await expect(report).toContainText('VAULT-TEC NETWORK RECOVERY');
  await report.getByRole('button', { name: 'ЗАКРЫТЬ' }).click();

  await page.getByLabel('НАЗВАНИЕ УСТРОЙСТВА').fill('Primary grid renamed');
  await page.locator('#btnSaveFacilityDeviceChanges').click();
  await expect(device).toContainText('Primary grid renamed');
  await expectAutosaved(page, request, before.saveCalls + 1);
  const renamed = (await facilityState(request)).facility.devices.find(candidate => candidate.id === FACILITY_IDS.devices.power);
  expect(renamed).toMatchObject({ id: FACILITY_IDS.devices.power, name: 'Primary grid renamed' });

  await page.getByRole('button', { name: 'УДАЛИТЬ УСТРОЙСТВО' }).click();
  const repair = page.getByRole('dialog', { name: 'ИСПРАВЛЕНИЕ ССЫЛОК' });
  await expect(repair.locator('[role="alert"]')).toContainText('используется');
  await expect(repair.getByRole('button', { name: 'УДАЛИТЬ', exact: true })).toBeDisabled();
  await repair.getByLabel('ПЕРЕНАЗНАЧИТЬ НА').selectOption(FACILITY_IDS.devices.cooling);
  await repair.getByRole('button', { name: 'ПРОВЕРИТЬ ИСПРАВЛЕНИЕ' }).click();
  await expect(repair.locator('#facilityRepairImpact')).not.toBeEmpty();
  await repair.getByRole('button', { name: 'ИСПРАВИТЬ И УДАЛИТЬ' }).click();
  await expect(repair).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 2);

  const state = await facilityState(request);
  expect(state.repairWrites).toBe(1);
  expect(state.facility.devices.some(candidate => candidate.id === FACILITY_IDS.devices.power)).toBe(false);
  expect(state.brokenReferenceCount).toBe(0);
});

test('cancel and invalid drafts never mutate the canonical facility and report accessible errors', async ({ page, request }) => {
  await resetFixture(request, 'authored');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);

  await page.getByRole('button', { name: 'ДОБАВИТЬ УСТРОЙСТВО' }).click();
  const dialog = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА' });
  await dialog.getByLabel('ИДЕНТИФИКАТОР').fill(FACILITY_IDS.devices.door);
  await dialog.getByLabel('НАЗВАНИЕ').fill('Duplicate identity');
  await dialog.getByRole('button', { name: DONE }).click();
  await expect(dialog.locator('[role="alert"]')).toContainText('идентификатор');
  await expect(dialog.locator('[aria-invalid="true"]')).toBeFocused();
  expect((await facilityState(request)).facility).toEqual(before.facility);
  expect(await page.evaluate(() => window.__desktopFixture.calls
    .filter(call => call.method === 'SaveAuthoring').length)).toBe(0);

  await dialog.getByRole('button', { name: 'ОТМЕНА' }).click();
  await expect(dialog).toBeHidden();
  expect((await facilityState(request)).facility).toEqual(before.facility);
  expect((await facilityState(request)).saveCalls).toBe(before.saveCalls);
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'idle');
});

test('edits graph relationships and assigns one atomic multi-device command action', async ({ page, request }) => {
  await resetFixture(request, 'authored');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);

  await rowById(page, 'device', FACILITY_IDS.devices.power).click();
  await page.getByRole('button', { name: 'РЕДАКТИРОВАТЬ ГРАФ' }).click();
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА' });
  const advanced = device.locator('details.facility-transition-references').first();
  if (!await advanced.evaluate(element => element.open)) await advanced.locator('summary').click();
  await expect(device.getByRole('button', { name: 'ДОБАВИТЬ ПРЕДУСЛОВИЕ' }).first()).toBeVisible();
  await expect(device.getByRole('button', { name: 'ДОБАВИТЬ ЭФФЕКТ УСЛОВИЯ' }).first()).toBeVisible();
  await device.getByRole('button', { name: 'ОТМЕНА' }).click();

  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРИВЯЗКУ' }).click();
  const binding = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА' });
  await expect(binding.getByLabel('ТИП ПРИВЯЗКИ').locator('option')).toHaveText([
    'НАЗВАНИЕ В МЕНЮ', 'ТЕКСТ БЛОКА ЗАПИСИ', 'ВИДИМОСТЬ ЭЛЕМЕНТА',
    'ДОСТУПНОСТЬ КОМАНДЫ', 'ДЕЙСТВИЕ КОМАНДЫ',
  ]);
  await binding.getByLabel('ТИП ПРИВЯЗКИ').selectOption('command-action');
  await binding.getByLabel('ТЕРМИНАЛ').selectOption(FACILITY_IDS.terminals.reactor);
  await binding.getByLabel('ЭЛЕМЕНТ').selectOption(FACILITY_IDS.nodes.startReactor);

  const first = binding.locator('.facility-action-request-row').first();
  await first.getByLabel('УСТРОЙСТВО ПЕРЕХОДА').selectOption(FACILITY_IDS.devices.power);
  await first.getByLabel('ПЕРЕХОД КОМАНДЫ').selectOption('restore');
  await binding.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД К ДЕЙСТВИЮ' }).click();
  const second = binding.locator('.facility-action-request-row').last();
  await second.getByLabel('УСТРОЙСТВО ПЕРЕХОДА').selectOption(FACILITY_IDS.devices.cooling);
  await second.getByLabel('ПЕРЕХОД КОМАНДЫ').selectOption('restore');
  await binding.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ', { exact: true }).fill('MAIN REACTOR ONLINE');
  await binding.getByLabel('ЗАПРОС ПОДТВЕРЖДЕНИЯ', { exact: true }).fill('Start the main reactor?');
  await binding.getByRole('button', { name: DONE }).click();

  await expect(page.locator('#facilityBindingList')).toContainText('ДЕЙСТВИЕ КОМАНДЫ');
  await expectAutosaved(page, request, before.saveCalls + 1);
});

test('previews a device state and diagnostic fault without publishing or mutating the facility', async ({ page, request }) => {
  await resetFixture(request, 'operations');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);

  await rowById(page, 'device', FACILITY_IDS.devices.power).click();
  await page.getByRole('button', { name: 'ПРЕДПРОСМОТР СОСТОЯНИЯ' }).click();
  const statePreview = page.getByRole('dialog', { name: 'ПРЕДПРОСМОТР ОБЪЕКТА' });
  await statePreview.getByLabel('СОСТОЯНИЕ').selectOption('online');
  await statePreview.getByLabel('ТЕРМИНАЛ').selectOption(FACILITY_IDS.terminals.reactor);
  await expect(statePreview.locator('#facilityPreviewContext'))
    .toHaveText('ПРЕДПРОСМОТР ПОКАЗЫВАЕТ ПРИМЕНЁННЫЕ ПРАВКИ И НЕ МЕНЯЕТ МИР ДЛЯ ИГРОКОВ.');
  await statePreview.getByRole('button', { name: 'ОБНОВИТЬ ПРЕДПРОСМОТР' }).click();
  await expect(statePreview.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
  await expect(statePreview.locator('#facilityPreviewHypothetical [data-preview-node-id="command-start-reactor"]'))
    .toContainText('START MAIN REACTOR');
  expect((await facilityState(request)).facility).toEqual(before.facility);
  await statePreview.getByRole('button', { name: 'ЗАКРЫТЬ' }).click();

  await rowById(page, 'condition', FACILITY_IDS.conditions.unpowered).click();
  await page.getByRole('button', { name: 'ПРЕДПРОСМОТР УСЛОВИЯ' }).click();
  const faultPreview = page.getByRole('dialog', { name: 'ПРЕДПРОСМОТР ОБЪЕКТА' });
  await faultPreview.getByLabel('АКТИВНО').check();
  await faultPreview.getByRole('button', { name: 'ОБНОВИТЬ ПРЕДПРОСМОТР' }).click();
  await expect(faultPreview.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
  await faultPreview.getByRole('button', { name: 'ЗАКРЫТЬ' }).click();

  const after = await facilityState(request);
  expect(after.facility).toEqual(before.facility);
  expect(after.sessionRevision).toBe(before.sessionRevision);
  expect(after.previewCalls).toBeGreaterThan(1);
  expect(after.publishedEvents).toBe(0);
});

test('cancels then confirms one-device reset without changing unrelated facility values', async ({ page, request }) => {
  await resetFixture(request, 'operations');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);

  await rowById(page, 'device', FACILITY_IDS.devices.power).click();
  await page.getByRole('button', { name: 'СБРОСИТЬ УСТРОЙСТВО' }).click();
  const confirmation = page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ СБРОСА УСТРОЙСТВА' });
  await confirmation.getByRole('button', { name: 'ОТМЕНА' }).click();
  expect((await facilityState(request)).facility).toEqual(before.facility);

  await page.getByRole('button', { name: 'СБРОСИТЬ УСТРОЙСТВО' }).click();
  await confirmation.getByRole('button', { name: 'СБРОСИТЬ' }).click();
  await expect(page.locator('#facilityStatus')).toContainText('УСТРОЙСТВО СБРОШЕНО');

  const after = await facilityState(request);
  expect(after.facility.revision).toBe(before.facility.revision + 1);
  expect(after.facility.devices.find(device => device.id === FACILITY_IDS.devices.power).currentStateId)
    .toBe(after.facility.devices.find(device => device.id === FACILITY_IDS.devices.power).initialStateId);
  expect(after.facility.devices.find(device => device.id === FACILITY_IDS.devices.cooling).currentStateId)
    .toBe(before.facility.devices.find(device => device.id === FACILITY_IDS.devices.cooling).currentStateId);
  expect(after.facility.conditions.find(condition => condition.id === FACILITY_IDS.conditions.unpowered).currentActive)
    .toBe(after.facility.conditions.find(condition => condition.id === FACILITY_IDS.conditions.unpowered).initialActive);
  expect(after.resetWrites).toBe(1);
});

test('keeps the scene intact on stale and failed scene resets, then commits one confirmed scene reset', async ({ page, request }) => {
  await resetFixture(request, 'operations');
  await openFacilityWorkspace(page);
  const before = await facilityState(request);
  const dialog = page.locator('#sceneResetDialog');

  for (const [failure, reason] of [
    ['stale-revision', 'СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС'],
    ['persistence-failure', 'НЕ УДАЛОСЬ СОХРАНИТЬ СОСТОЯНИЕ ОБЪЕКТА'],
  ]) {
    const configure = await request.post(`${FIXTURE}/next-operation`, { data: { failure } });
    expect(configure.status()).toBe(204);
    await page.getByRole('button', { name: 'СБРОСИТЬ СЦЕНУ', exact: true }).click();
    await expect(dialog).toBeVisible();
    await dialog.locator('#btnConfirmSceneReset').click();
    await expect(dialog.locator('#sceneResetStatus')).toHaveText(`СЦЕНА НЕ СБРОШЕНА: ${reason}`);
    expect((await facilityState(request)).facility).toEqual(before.facility);
    await dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
    await expect(dialog).toBeHidden();
  }

  await page.getByRole('button', { name: 'СБРОСИТЬ СЦЕНУ', exact: true }).click();
  await dialog.locator('#btnConfirmSceneReset').click();
  await expect(dialog).toBeHidden();
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-wails-command', 'ResetScene');
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-wails-result', 'ok');

  const after = await facilityState(request);
  expect(after.facility.revision).toBe(before.facility.revision + 1);
  expect(after.facility.devices.every(device => device.currentStateId === device.initialStateId)).toBe(true);
  expect(after.facility.conditions.every(condition => condition.currentActive === condition.initialActive)).toBe(true);
  expect(after.resetWrites).toBe(1);
  expect(after.saveCalls).toBe(before.saveCalls);
});

test('private recovery stays available from the facility workspace and restores focus after confirmation', async ({ page, request }) => {
  await resetFixture(request, 'operations');
  await openFacilityWorkspace(page);
  const before = await facilityState(request);

  const condition = rowById(page, 'condition', FACILITY_IDS.conditions.authorizationCorrupted);
  await condition.click();
  await page.getByRole('button', { name: 'ВОССТАНОВИТЬ УСЛОВИЕ' }).click();
  const confirmation = page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ ВОССТАНОВЛЕНИЯ' });
  await confirmation.getByRole('button', { name: 'ВОССТАНОВИТЬ' }).click();
  await expect(page.locator('#facilityStatus')).toContainText('УСЛОВИЕ ВОССТАНОВЛЕНО');
  await expect(condition).toBeFocused();

  const after = await facilityState(request);
  expect(after.facility.revision).toBe(before.facility.revision + 1);
  expect(after.facility.conditions.find(candidate => candidate.id === FACILITY_IDS.conditions.authorizationCorrupted).currentActive)
    .toBe(false);
  expect(after.recoveryWrites).toBe(1);
});
