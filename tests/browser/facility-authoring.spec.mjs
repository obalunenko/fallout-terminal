import { expect, test } from '@playwright/test';

import { FACILITY_IDS } from './fixtures/facility-session.mjs';
import { expectSaved } from './helpers/authoring-bar.mjs';
import {
  commandActionDialog, deviceDialog, fillRule, openTransitionCell, programDialog, renderedError, ruleDialog, stateChips,
} from './helpers/facility-editors.mjs';
import { playCard } from './helpers/facility-play.mjs';

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

// openDetailTab opens an ОБЪЕКТЫ detail tab by its label, whatever its count.
async function openDetailTab(page, label) {
  const tab = page.getByRole('tab', { name: new RegExp(`^${label} \\d+$`) });
  await tab.click();
  await expect(tab).toHaveAttribute('aria-selected', 'true');
}

// Every applied facility edit autosaves through SaveAuthoring; the shared bar
// reports it once the save queue is idle.
async function expectAutosaved(page, request, saveCalls) {
  await expect.poll(async () => (await facilityState(request)).saveCalls).toBe(saveCalls);
  await expectSaved(page);
  await expect(page.locator('#saveStatus')).toHaveText(/^СОХРАНЕНО · \d{2}:\d{2}:\d{2}$/);
}

test('authors a complete reusable device graph, display rule, fault, and recovery program without JSON (FR-020, FR-043)', async ({ page, request }) => {
  await resetFixture(request, 'empty');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);

  await page.getByRole('button', { name: 'ДОБАВИТЬ УСТРОЙСТВО' }).click();
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true });
  const editor = deviceDialog(page);
  await device.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Auxiliary reactor');
  await device.getByLabel('ТИП', { exact: true }).selectOption('reactor');
  for (const name of ['Offline', 'Online']) {
    await editor.addState.click();
    await expect(editor.stateName).toBeFocused();
    await editor.stateName.fill(name);
  }
  await expect.poll(async () => (await stateChips(page)).map(chip => [chip.name, chip.initial]))
    .toEqual([['Offline', true], ['Online', false]]);
  const [offline, online] = await stateChips(page);
  const panel = await openTransitionCell(page, offline.id, online.id);
  await panel.locator('input.facility-transition-name').fill('Start auxiliary reactor');
  await expect(editor.cell(offline.id, online.id)).toContainText('Start auxiliary reactor');
  await device.getByRole('button', { name: DONE }).click();
  await expect(device).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 1);
  const reactor = (await facilityState(request)).facility.devices.find(candidate => candidate.name === 'Auxiliary reactor');
  const stateID = name => reactor.states.find(candidate => candidate.name === name).id;
  expect(reactor).toMatchObject({ kind: 'reactor', initialStateId: stateID('Offline'), currentStateId: stateID('Offline') });
  const [start] = reactor.transitions;
  expect(start).toMatchObject({
    name: 'Start auxiliary reactor', sourceStateId: stateID('Offline'), destinationStateId: stateID('Online'),
  });

  // A fault saves without a clearing transition or the operator checkbox (FR-020).
  await openDetailTab(page, 'НЕИСПРАВНОСТИ');
  await page.getByRole('button', { name: 'ДОБАВИТЬ НЕИСПРАВНОСТЬ' }).click();
  const condition = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await expect(condition.getByLabel('ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ')).toHaveCount(0);
  await condition.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Auxiliary reactor unpowered');
  await condition.getByLabel('КАТЕГОРИЯ', { exact: true }).selectOption('unpowered');
  await condition.getByLabel('УСТРОЙСТВО', { exact: true }).selectOption(reactor.id);
  await condition.getByLabel('ЧТО БЛОКИРУЕТ', { exact: true }).selectOption('execute-command');
  await expect(condition.locator('#facilityFaultClearsList li .facility-fault-way-name')).toHaveText(['Смотритель вручную']);
  await condition.getByRole('button', { name: DONE }).click();
  await expectAutosaved(page, request, before.saveCalls + 2);

  await openDetailTab(page, 'ПРОГРАММЫ');
  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ' }).click();
  const program = programDialog(page);
  await expect(program.dialog).toBeVisible();
  await program.name.fill('Auxiliary startup');
  await expect(program.rows).toHaveCount(1);
  await program.rows.locator('select.facility-action-device').selectOption(reactor.id);
  await program.rows.locator('select.facility-action-transition').selectOption(start.id);
  await program.save.click();
  await expectAutosaved(page, request, before.saveCalls + 3);

  await openDetailTab(page, 'ЧТО ВИДЯТ ИГРОКИ');
  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРАВИЛО' }).click();
  const rule = ruleDialog(page);
  await expect(rule.dialog).toBeVisible();
  await expect(rule.title).toHaveText('ПРАВИЛО ОТОБРАЖЕНИЯ');
  await rule.tab('menu-name').click();
  await fillRule(page, {
    deviceId: reactor.id, stateId: stateID('Online'),
    terminalId: FACILITY_IDS.terminals.reactor, nodeId: FACILITY_IDS.nodes.startReactor, text: 'AUXILIARY REACTOR ONLINE',
  });
  await rule.save.click();
  await expectAutosaved(page, request, before.saveCalls + 4);

  const saved = await facilityState(request);
  expect(await page.evaluate(() => window.__desktopFixture.calls
    .filter(call => call.method === 'SaveAuthoring').length)).toBe(4);
  const fault = saved.facility.conditions.find(candidate => candidate.name === 'Auxiliary reactor unpowered');
  expect(fault).toMatchObject({ device: { deviceId: reactor.id } });
  expect(fault.recovery).toBeUndefined();
  expect(saved.facility.recoveryPrograms).toContainEqual(expect.objectContaining({
    name: 'Auxiliary startup', transitions: [{ deviceId: reactor.id, transitionId: start.id }],
  }));
  expect(saved.bindingCount).toBeGreaterThan(0);

  await page.reload();
  await openFacilityWorkspace(page);
  await expect(rowById(page, 'device', reactor.id)).toContainText('Auxiliary reactor');
});

test('dependency inspection protects stable identity and applies one complete reassignment repair', async ({ page, request }) => {
  await resetFixture(request, 'referenced-device');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);
  const device = rowById(page, 'device', FACILITY_IDS.devices.power);
  await device.click();
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ' }).click();

  const report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ УСТРОЙСТВА', exact: true });
  await expect(report).toContainText('FACILITY STATUS');
  await expect(report).toContainText('START MAIN REACTOR');
  await expect(report).toContainText('VAULT-TEC NETWORK RECOVERY');
  // Each link names its destination; the technical path stays folded (FR-037).
  expect((await report.locator('button.facility-dependency-navigation').allTextContents()).sort())
    .toEqual(['К ПРАВИЛУ ОТОБРАЖЕНИЯ', 'К ПРАВИЛУ ОТОБРАЖЕНИЯ', 'К ПРОГРАММЕ']);
  await expect(report.getByRole('button', { name: 'ОТКРЫТЬ', exact: true })).toHaveCount(0);
  expect(await report.locator('details.facility-technical-details').evaluateAll(items => items.some(item => item.open)))
    .toBe(false);
  await report.getByRole('button', { name: 'ЗАКРЫТЬ' }).click();

  await page.getByRole('button', { name: 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ' }).click();
  const graph = page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true });
  await graph.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Primary grid renamed');
  await graph.getByRole('button', { name: DONE }).click();
  await expect(graph).toBeHidden();
  await expect(device).toContainText('Primary grid renamed');
  await expectAutosaved(page, request, before.saveCalls + 1);
  const renamed = (await facilityState(request)).facility.devices.find(candidate => candidate.id === FACILITY_IDS.devices.power);
  expect(renamed).toMatchObject({ id: FACILITY_IDS.devices.power, name: 'Primary grid renamed' });

  await page.getByRole('button', { name: 'ДРУГИЕ ДЕЙСТВИЯ С УСТРОЙСТВОМ' }).click();
  await page.getByRole('menuitem', { name: 'УДАЛИТЬ УСТРОЙСТВО' }).click();
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
  const dialog = page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true });
  const editor = deviceDialog(page);
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Duplicate identity');
  await dialog.getByLabel('ТИП', { exact: true }).selectOption('door');
  await editor.addState.click();
  await editor.technical.locator('summary').click();
  await page.locator('#facilityDeviceID').fill(FACILITY_IDS.devices.door);
  await dialog.getByRole('button', { name: DONE }).click();
  await expect(dialog.locator('[role="alert"]')).toContainText('идентификатор');
  await expect(dialog.locator('[aria-invalid="true"]')).toBeFocused();
  expect(await editor.technical.evaluate(details => details.open)).toBe(true);
  expect((await facilityState(request)).facility).toEqual(before.facility);
  expect(await page.evaluate(() => window.__desktopFixture.calls
    .filter(call => call.method === 'SaveAuthoring').length)).toBe(0);

  await dialog.getByRole('button', { name: 'ОТМЕНА' }).click();
  await expect(dialog).toBeHidden();
  expect((await facilityState(request)).facility).toEqual(before.facility);
  expect((await facilityState(request)).saveCalls).toBe(before.saveCalls);
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'idle');
});

test('edits graph relationships, offers the four rule tabs, and assigns one atomic multi-device command action (FR-017, FR-022, FR-029)', async ({ page, request }) => {
  await resetFixture(request, 'authored');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);

  await rowById(page, 'device', FACILITY_IDS.devices.power).click();
  await page.getByRole('button', { name: 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ' }).click();
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true });
  const panel = await openTransitionCell(page, 'offline', 'online');
  const transition = panel.locator('.facility-transition-row[data-transition-id="restore"]');
  await expect(transition.getByRole('button', { name: '+ ПРЕДУСЛОВИЕ', exact: true })).toBeVisible();
  // Fault effects are read-only here and edited in the fault dialog (FR-017).
  await expect(transition.locator('.facility-transition-fault-effects .facility-empty'))
    .toHaveText('Переход не включает и не снимает неисправности. Это задаётся в окне неисправности.');
  await expect(device.getByRole('button', { name: 'ДОБАВИТЬ ЭФФЕКТ УСЛОВИЯ' })).toHaveCount(0);
  await device.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
  await expect(device).toBeHidden();

  await openDetailTab(page, 'ЧТО ВИДЯТ ИГРОКИ');
  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРАВИЛО' }).click();
  const rule = ruleDialog(page);
  await expect(page.getByRole('dialog', { name: 'ПРАВИЛО ОТОБРАЖЕНИЯ', exact: true })).toBeVisible();
  await expect(rule.tabs).toHaveText(['ТЕКСТ ЗАПИСИ', 'НАЗВАНИЕ В МЕНЮ', 'ПОКАЗАТЬ ЭЛЕМЕНТ', 'РАЗРЕШИТЬ КОМАНДУ']);
  await expect(page.locator('#facilityBindingKind')).toHaveCount(0);
  await rule.cancel.click();
  await expect(rule.dialog).toBeHidden();

  await openDetailTab(page, 'ЧТО ДЕЛАЮТ ИГРОКИ');
  await page.getByRole('button', { name: 'ДОБАВИТЬ ДЕЙСТВИЕ КОМАНДЫ' }).click();
  const action = commandActionDialog(page);
  await expect(page.getByRole('dialog', { name: 'ДЕЙСТВИЕ КОМАНДЫ', exact: true })).toBeVisible();
  const command = page.locator(`#facilityCommandActionCommand option[data-terminal-id="${FACILITY_IDS.terminals.reactor}"][data-node-id="${FACILITY_IDS.nodes.startReactor}"]`);
  await action.command.selectOption(await command.getAttribute('value'));
  await expect(action.modeTransitions).toBeChecked();
  await expect(action.rows).toHaveCount(1);
  const first = action.rows.first();
  await first.locator('select.facility-action-device').selectOption(FACILITY_IDS.devices.power);
  await first.locator('select.facility-action-transition').selectOption('restore');
  await action.addTransition.click();
  const second = action.rows.last();
  await second.locator('select.facility-action-device').selectOption(FACILITY_IDS.devices.cooling);
  await second.locator('select.facility-action-transition').selectOption('restore');
  await action.completedName.fill('MAIN REACTOR ONLINE');
  await action.confirmationText.fill('Start the main reactor?');
  await action.save.click();
  await expect(action.dialog).toBeHidden();

  await expect(page.locator('#facilityActionBindingList')).toContainText('ДЕЙСТВИЕ КОМАНДЫ');
  await expectAutosaved(page, request, before.saveCalls + 1);
  const session = await (await request.get(`${FIXTURE}/session`)).json();
  const reactor = session.terminals.find(terminal => terminal.id === FACILITY_IDS.terminals.reactor);
  const node = reactor.root.children.find(child => child.id === FACILITY_IDS.nodes.startReactor);
  expect(node.stateChange).toMatchObject({
    completedName: 'MAIN REACTOR ONLINE', confirmationText: 'Start the main reactor?',
    facilityAction: { transitions: { transitions: [
      { deviceId: FACILITY_IDS.devices.power, transitionId: 'restore' },
      { deviceId: FACILITY_IDS.devices.cooling, transitionId: 'restore' },
    ] } },
  });
});

test('previews a device state and a fault without publishing or mutating the facility', async ({ page, request }) => {
  await resetFixture(request, 'operations');
  const before = await facilityState(request);
  await openFacilityWorkspace(page);

  await rowById(page, 'device', FACILITY_IDS.devices.power).click();
  await page.getByRole('button', { name: 'ПРЕДПРОСМОТР СОСТОЯНИЯ' }).click();
  const statePreview = page.getByRole('dialog', { name: 'ПРЕДПРОСМОТР УСТРОЙСТВА', exact: true });
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

  // A fault's preview opens from its dialog, over it.
  await openDetailTab(page, 'НЕИСПРАВНОСТИ');
  await rowById(page, 'condition', FACILITY_IDS.conditions.unpowered).click();
  const condition = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await condition.getByRole('button', { name: 'ПРЕДПРОСМОТР НЕИСПРАВНОСТИ' }).click();
  const faultPreview = page.getByRole('dialog', { name: 'ПРЕДПРОСМОТР НЕИСПРАВНОСТИ', exact: true });
  await faultPreview.getByLabel('АКТИВНО').check();
  await faultPreview.getByRole('button', { name: 'ОБНОВИТЬ ПРЕДПРОСМОТР' }).click();
  await expect(faultPreview.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
  await faultPreview.getByRole('button', { name: 'ЗАКРЫТЬ' }).click();
  await expect(faultPreview).toBeHidden();
  await expect(condition).toBeVisible();
  await condition.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
  await expect(condition).toBeHidden();

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
  await page.getByRole('button', { name: 'СБРОСИТЬ К НАЧАЛЬНОМУ…' }).click();
  const confirmation = page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ СБРОСА УСТРОЙСТВА' });
  await confirmation.getByRole('button', { name: 'ОТМЕНА' }).click();
  expect((await facilityState(request)).facility).toEqual(before.facility);

  await page.getByRole('button', { name: 'СБРОСИТЬ К НАЧАЛЬНОМУ…' }).click();
  await confirmation.getByRole('button', { name: 'СБРОСИТЬ' }).click();
  await expect(page.locator('#facilityPlayStatus')).toContainText('УСТРОЙСТВО СБРОШЕНО');

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

  for (const [failure, key, message] of [
    ['stale-revision', 'stale-revision', 'Локация изменилась, пока выполнялся запрос. Проверьте новое состояние и повторите.'],
    ['persistence-failure', 'persistence-failed',
      'Не удалось записать файл сессии. Проверьте, что диск доступен и файл не защищён от записи, затем повторите.'],
  ]) {
    const configure = await request.post(`${FIXTURE}/next-operation`, { data: { failure } });
    expect(configure.status()).toBe(204);
    await page.getByRole('button', { name: 'СБРОСИТЬ СЦЕНУ', exact: true }).click();
    await expect(dialog).toBeVisible();
    await dialog.locator('#btnConfirmSceneReset').click();
    const status = dialog.locator('#sceneResetStatus');
    await expect(status.locator('.overseer-error-prefix')).toHaveText('СЦЕНА НЕ СБРОШЕНА:');
    expect(await renderedError(status)).toMatchObject({
      area: 'facility', key, message, original: `failure: ${key}`, summary: 'подробности для разработчика',
    });
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

test('the fault switch clears a fault from the facility workspace (FR-020)', async ({ page, request }) => {
  await resetFixture(request, 'operations');
  await openFacilityWorkspace(page);
  const before = await facilityState(request);
  const conditionId = FACILITY_IDS.conditions.authorizationCorrupted;
  expect(before.facility.conditions.find(candidate => candidate.id === conditionId).currentActive).toBe(true);

  // «ВОССТАНОВИТЬ УСЛОВИЕ» and its confirmation are gone; rows open the fault dialog.
  await openDetailTab(page, 'НЕИСПРАВНОСТИ');
  await expect(page.getByRole('button', { name: 'ВОССТАНОВИТЬ УСЛОВИЕ' })).toHaveCount(0);
  await expect(rowById(page, 'condition', conditionId)).toHaveAttribute('aria-haspopup', 'dialog');

  await rowById(page, 'device', FACILITY_IDS.devices.door).click();
  const card = playCard(page);
  await expect(card.faultSwitch(conditionId)).toHaveAttribute('aria-checked', 'true');
  await card.faultSwitch(conditionId).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
  await expect(card.faultSwitch(conditionId)).toHaveAttribute('aria-checked', 'false');
  await expect(page.getByRole('dialog')).toHaveCount(0);

  const after = await facilityState(request);
  expect(after.facility.revision).toBe(before.facility.revision + 1);
  expect(after.facility.conditions.find(candidate => candidate.id === conditionId).currentActive ?? false).toBe(false);
  expect(after.saveCalls).toBe(before.saveCalls);
});
