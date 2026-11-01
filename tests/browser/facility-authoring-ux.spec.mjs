import { readFile } from 'node:fs/promises';

import { expect, test } from '@playwright/test';

import { buildFacilityAuthoringSession, FACILITY_IDS } from './fixtures/facility-session.mjs';
import { expectSaved, switchTab, undo } from './helpers/authoring-bar.mjs';
import {
  commandActionDialog, deviceDialog, fillRule, openTransitionCell, ruleDialog, stateChips,
} from './helpers/facility-editors.mjs';

const FIXTURE = '/__fixture/facility-authoring';
const DONE = 'ГОТОВО';
const DEMO_URL = new URL('../../sessions/demo.json', import.meta.url);

async function facilityState(request) {
  const response = await request.get(`${FIXTURE}/state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

async function resetFixture(request) {
  const response = await request.post(`${FIXTURE}/reset`, { data: { scenario: 'operations' } });
  expect(response.status()).toBe(204);
}

async function seedMultiTransitionDevice(request) {
  await resetFixture(request);
  const before = await facilityState(request);
  const response = await request.get(`${FIXTURE}/session`);
  expect(response.ok()).toBe(true);
  const session = await response.json();
  const device = session.facility.devices.find(value => value.id === FACILITY_IDS.devices.power);
  device.transitions = buildFacilityAuthoringSession().facility.devices
    .find(value => value.id === device.id).transitions;
  const reactor = session.terminals.find(value => value.id === FACILITY_IDS.terminals.reactor);
  const startReactor = reactor.root.children.find(value => value.id === FACILITY_IDS.nodes.startReactor);
  startReactor.stateChange = {
    completedName: 'PRIMARY POWER RESTORED',
    confirmationText: 'Restore primary power?',
    facilityAction: {
      transitions: { transitions: [{ deviceId: FACILITY_IDS.devices.power, transitionId: 'restore' }] },
    },
  };
  const saved = await request.post(`${FIXTURE}/save`, {
    data: {
      session,
      expectedSessionRevision: before.sessionRevision,
      expectedFacilityRevision: before.facility.revision,
      correlationId: 'seed-multi-transition-ux-regression',
    },
  });
  expect(saved.ok()).toBe(true);
  expect(await saved.json()).toMatchObject({ ok: true });
  return facilityState(request);
}

async function seedDemoAuthoringSession(request) {
  await resetFixture(request);
  const before = await facilityState(request);
  const demo = JSON.parse(await readFile(DEMO_URL, 'utf8'));
  const response = await request.post(`${FIXTURE}/save`, {
    data: {
      session: demo,
      expectedSessionRevision: before.sessionRevision,
      expectedFacilityRevision: before.facility.revision,
      correlationId: 'seed-full-demo-authoring-ux',
    },
  });
  expect(response.ok()).toBe(true);
  expect(await response.json()).toMatchObject({ ok: true });
  return demo;
}

async function openWorkspace(page) {
  await page.goto(`${FIXTURE}/overseer`);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ', exact: true }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
  await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  await expect(page.locator('#facilityWorkspace')).toBeVisible();
}

// expectAutosaved waits for the facility store to count saveCalls changed
// writes and for the shared bar to report the save.
async function expectAutosaved(page, request, saveCalls) {
  await expect.poll(async () => (await facilityState(request)).saveCalls).toBe(saveCalls);
  await expectSaved(page);
}

// authoredFacility is a facility without the values that change with every
// save or live action, for comparing authored content across saves.
function authoredFacility(state) {
  const { revision: _revision, ...facility } = state.facility;
  return facility;
}

// rejectNextSave makes the server reject the next SaveAuthoring request with
// failure, answering for that request's correlation ID with the canonical
// session, as the session store does.
async function rejectNextSave(page, request, failure = 'persistence-failed') {
  await page.route(`**${FIXTURE}/save-authoring`, async route => {
    const { correlationId } = route.request().postDataJSON();
    const state = await facilityState(request);
    const session = await (await request.get(`${FIXTURE}/session`)).json();
    await route.fulfill({ json: {
      ok: false, changed: false, correlationId, failure,
      issues: [{ code: failure, entityKind: 'session' }],
      sessionRevision: state.sessionRevision, authoringRevision: state.authoringRevision,
      previousFacilityRevision: state.facility.revision, resultingFacilityRevision: state.facility.revision,
      session,
    } });
  }, { times: 1 });
}

function deviceRow(page, id = FACILITY_IDS.devices.power) {
  return page.locator(`[data-facility-kind="device"][data-facility-id="${id}"]`);
}

// openPowerGraph selects the power grid and opens its device dialog, where its
// name, states, and transitions are edited.
async function openPowerGraph(page) {
  await deviceRow(page).click();
  await page.getByRole('button', { name: 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

// openDetailTab opens one of the «ОБЪЕКТЫ» detail tabs. Only the open tab's
// list and add button are visible.
async function openDetailTab(page, id) {
  await page.locator(`#${id}`).click();
  await expect(page.locator(`#${id}`)).toHaveAttribute('aria-selected', 'true');
}

// openGuide expands the scenario guide, which is collapsed to its summary
// line while the session has devices.
async function openGuide(page) {
  const guide = page.locator('#facilityStoryGuide');
  const toggle = page.locator('#facilityStoryGuideToggle');
  await expect(guide).not.toHaveAttribute('open', '');
  await expect(toggle).toHaveText('ПОКАЗАТЬ');
  await page.locator('#facilityStoryGuideSummary').click();
  await expect(guide).toHaveAttribute('open', '');
  await expect(toggle).toHaveText('СКРЫТЬ');
  return guide;
}

// openTransitionRow is the transition the device dialog's panel shows.
function openTransitionRow(page) {
  return page.locator('#facilityTransitionPanel .facility-transition-row:not([hidden])');
}

// openTransition opens the matrix cell from → to and, when the cell lists
// several transitions, the one with id. It returns the open transition.
async function openTransition(page, from, to, id) {
  await openTransitionCell(page, from, to);
  const list = page.locator('#facilityTransitionCellList');
  if (await list.isVisible()) await list.locator(`.facility-transition-choice[data-transition-id="${id}"]`).click();
  const transition = openTransitionRow(page);
  await expect(transition).toHaveAttribute('data-transition-id', id);
  return transition;
}

// addStates adds one state per name through «+ СОСТОЯНИЕ» and its chip editor.
async function addStates(page, names) {
  const device = deviceDialog(page);
  for (const name of names) {
    await device.addState.click();
    await expect(device.stateName).toBeFocused();
    await device.stateName.fill(name);
  }
}

// addTransition starts a transition in the empty matrix cell from → to and
// names it.
async function addTransition(page, from, to, name) {
  const device = deviceDialog(page);
  await expect(device.cell(from, to)).toHaveAttribute('data-count', '0');
  await device.cell(from, to).click();
  const transition = openTransitionRow(page);
  await expect(transition.locator('.facility-transition-name')).toBeFocused();
  await transition.locator('.facility-transition-name').fill(name);
  await expect(device.cell(from, to).locator('.facility-transition-cell-name')).toHaveText(name);
  return transition;
}

// fillProgramTransition picks the device and transition of the program
// dialog's first transition row.
async function fillProgramTransition(page, deviceID, transitionID) {
  const row = page.locator('#facilityProgramTransitions .facility-action-request-row').first();
  await row.locator('.facility-action-device').selectOption(deviceID);
  await row.locator('.facility-action-transition').selectOption(transitionID);
}

// fillTechnicalID opens a dialog's folded «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ» and
// types a new entity's ID.
async function fillTechnicalID(dialog, technicalID, value) {
  const technical = dialog.locator(`#${technicalID}`);
  await expect(technical).not.toHaveAttribute('open', '');
  await technical.locator('summary').click();
  await technical.getByLabel('ИДЕНТИФИКАТОР', { exact: true }).fill(value);
}

// expectPowerReferences checks that both power transitions keep their
// precondition and their fault effects.
async function expectPowerReferences(page) {
  const restore = await openTransition(page, 'offline', 'online', 'restore');
  await expect(restore.locator('.facility-precondition-device')).toHaveValue(FACILITY_IDS.devices.cooling);
  await expect(restore.locator('.facility-precondition-state')).toHaveValue('online');
  await expect(restore.locator('.facility-transition-fault-effect'))
    .toHaveAttribute('data-condition-id', FACILITY_IDS.conditions.unpowered);
  const disconnect = await openTransition(page, 'online', 'offline', 'disconnect');
  await expect(disconnect.locator('.facility-transition-fault-effect'))
    .toHaveAttribute('data-condition-id', FACILITY_IDS.conditions.unpowered);
}

test('preserves multi-transition references and recovery metadata through state edits, autosave, and reopen', async ({ page, request }) => {
  const before = await seedMultiTransitionDevice(request);
  const original = before.facility.devices.find(value => value.id === FACILITY_IDS.devices.power);
  expect(original.transitions).toHaveLength(2);
  expect(original.currentStateId).not.toBe(original.initialStateId);
  await openWorkspace(page);
  const dialog = await openPowerGraph(page);
  const device = deviceDialog(page);

  await expectPowerReferences(page);
  await device.chip('offline').click();
  await device.stateName.fill('Power isolated');
  await expect(device.chip('offline')).toContainText('Power isolated');
  await expectPowerReferences(page);
  // A second transition in the filled cell «Online → Offline», with the ID
  // typed in the folded technical block.
  await openTransition(page, 'online', 'offline', 'disconnect');
  await device.addCellTransition.click();
  await openTransitionRow(page).locator('.facility-transition-name').fill('Restart primary grid');
  await device.technical.locator('summary').click();
  await device.technical.locator('.facility-transition-id-row').last().locator('.facility-transition-id').fill('restart');
  await expect(device.cell('online', 'offline')).toHaveAttribute('data-count', '2');
  await expectPowerReferences(page);
  await expect(device.save).toHaveText(DONE);
  await device.save.click();
  await expect(dialog).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 1);
  const saved = await facilityState(request);
  const power = saved.facility.devices.find(value => value.id === original.id);
  expect(power).toMatchObject({
    ...original,
    states: [{ ...original.states[0], name: 'Power isolated' }, original.states[1]],
    transitions: [...original.transitions, expect.objectContaining({
      id: 'restart', name: 'Restart primary grid', sourceStateId: 'online', destinationStateId: 'offline',
    })],
  });
  expect(saved.facility.conditions).toEqual(before.facility.conditions);
  expect(saved.facility.recoveryPrograms).toEqual(before.facility.recoveryPrograms);
  expect(saved.brokenReferenceCount).toBe(0);
  await openWorkspace(page);
  await openPowerGraph(page);
  await expectPowerReferences(page);
  await expect(deviceDialog(page).cell('online', 'offline')).toHaveAttribute('data-count', '2');
});

// Fault effects are edited in the fault dialog, whose pickers add only whole
// effects, so the device dialog's incomplete rows are its preconditions and
// its transition names.
for (const missing of ['prerequisite device', 'prerequisite state', 'transition name']) {
  test(`rejects an incomplete ${missing} with focused feedback and no lost draft row`, async ({ page, request }) => {
    await resetFixture(request);
    const before = await facilityState(request);
    await openWorkspace(page);
    const dialog = await openPowerGraph(page);
    const transition = await openTransition(page, 'offline', 'online', 'restore');
    let invalid;
    if (missing === 'transition name') {
      invalid = transition.locator('.facility-transition-name');
      await invalid.fill('');
    } else {
      await transition.getByRole('button', { name: '+ ПРЕДУСЛОВИЕ', exact: true }).click();
      const row = transition.locator('.facility-precondition-row').last();
      invalid = row.locator('.facility-precondition-device');
      if (missing === 'prerequisite state') {
        await invalid.selectOption(FACILITY_IDS.devices.cooling);
        invalid = row.locator('.facility-precondition-state');
      }
    }
    // The folded panel keeps the draft row, and «ГОТОВО» reopens it on the problem.
    await page.locator('#btnCloseFacilityTransitionPanel').click();
    await expect(page.locator('#facilityTransitionPanel')).toBeHidden();
    await dialog.locator('#btnSaveFacilityDevice').click();
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('#facilityDeviceError')).toBeVisible();
    await expect(dialog.locator('#facilityDeviceError')).not.toBeEmpty();
    await expect(page.locator('#facilityTransitionPanel')).toBeVisible();
    await expect(invalid).toHaveAttribute('aria-invalid', 'true');
    await expect(invalid).toBeFocused();
    expect(await facilityState(request)).toEqual(before);
    await dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
    expect(await facilityState(request)).toEqual(before);
  });
}

// applyName renames the power grid in its device dialog.
async function applyName(page, name) {
  const dialog = await openPowerGraph(page);
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill(name);
  await dialog.getByRole('button', { name: DONE, exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(deviceRow(page)).toContainText(name);
}

async function applyCondition(page) {
  await openDetailTab(page, 'facilityTabConditions');
  await page.getByRole('button', { name: 'ДОБАВИТЬ НЕИСПРАВНОСТЬ', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Staged power fault');
  await dialog.getByLabel('КАТЕГОРИЯ', { exact: true }).selectOption('unpowered');
  await dialog.getByLabel('УСТРОЙСТВО', { exact: true }).selectOption(FACILITY_IDS.devices.power);
  await dialog.getByLabel('ЧТО БЛОКИРУЕТ', { exact: true }).selectOption('execute-command');
  await fillTechnicalID(dialog, 'facilityConditionTechnical', 'condition-staged-ux');
  await dialog.getByRole('button', { name: DONE, exact: true }).click();
  await expect(dialog).toBeHidden();
}

test('every facility form autosaves on «ГОТОВО», and ОТМЕНИТЬ reverses the edits newest first', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  await applyName(page, 'Saved primary grid');
  await expectAutosaved(page, request, before.saveCalls + 1);
  await applyCondition(page);
  await expectAutosaved(page, request, before.saveCalls + 2);

  await openDetailTab(page, 'facilityTabPrograms');
  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await program.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Staged recovery');
  await fillProgramTransition(page, FACILITY_IDS.devices.power, 'restore');
  await fillTechnicalID(program, 'facilityRecoveryProgramTechnical', 'program-staged-ux');
  await program.getByRole('button', { name: DONE, exact: true }).click();
  await expect(program).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 3);

  await openDetailTab(page, 'facilityTabInformation');
  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРАВИЛО', exact: true }).click();
  const binding = page.getByRole('dialog', { name: 'ПРАВИЛО ОТОБРАЖЕНИЯ', exact: true });
  await ruleDialog(page).tab('menu-name').click();
  await fillRule(page, {
    deviceId: FACILITY_IDS.devices.power, stateId: 'online',
    terminalId: FACILITY_IDS.terminals.reactor, nodeId: FACILITY_IDS.nodes.startReactor, text: 'Staged startup label',
  });
  await binding.getByRole('button', { name: DONE, exact: true }).click();
  await expect(binding).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 4);

  const saved = await facilityState(request);
  expect(saved.facility.devices.find(value => value.id === FACILITY_IDS.devices.power))
    .toMatchObject({ name: 'Saved primary grid', currentStateId: 'online', initialStateId: 'offline' });
  expect(saved.facility.conditions).toContainEqual(expect.objectContaining({ id: 'condition-staged-ux' }));
  expect(saved.facility.recoveryPrograms).toContainEqual(expect.objectContaining({ id: 'program-staged-ux' }));
  expect(saved.bindingCount).toBe(before.bindingCount + 1);
  await openWorkspace(page);
  await expect(deviceRow(page)).toContainText('Saved primary grid');
  expect((await facilityState(request)).saveCalls).toBe(before.saveCalls + 4);

  // A new open has no history; edits made after it are reversed newest first,
  // each as a save of its own.
  await expect(page.locator('#btnUndo')).toHaveAttribute('aria-disabled', 'true');
  await expect(page.locator('#btnUndoReason')).toHaveText('НЕЧЕГО ОТМЕНЯТЬ');
  await applyName(page, 'Renamed after reopen');
  await expectAutosaved(page, request, before.saveCalls + 5);
  await expect(page.locator('#btnUndoReason')).toHaveText('ОТМЕНИТЬ: Устройство «Renamed after reopen»');
  await undo(page);
  await expect(deviceRow(page)).toContainText('Saved primary grid');
  await expectAutosaved(page, request, before.saveCalls + 6);
  expect(authoredFacility(await facilityState(request))).toEqual(authoredFacility(saved));
});

test('a rejected autosave shows its reason, reopens the edit with its input, and a retry saves it', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  await rejectNextSave(page, request);
  const dialog = await openPowerGraph(page);
  const name = dialog.getByLabel('НАЗВАНИЕ', { exact: true });
  await name.fill('Retry primary grid');
  await dialog.getByRole('button', { name: DONE, exact: true }).click();

  const status = page.locator('#saveStatus');
  await expect(status).toHaveAttribute('data-state', 'error');
  const reason = 'Не удалось записать файл сессии. Проверьте, что диск доступен и файл не защищён от записи, затем повторите.';
  await expect(status).toHaveText(`ОШИБКА СОХРАНЕНИЯ: ${reason}`);
  // Only the rejected edit is reverted, and the device dialog reopens with its
  // input and the reason.
  await expect(deviceRow(page)).toContainText('Primary power grid');
  await expect(dialog).toBeVisible();
  await expect(name).toHaveValue('Retry primary grid');
  await expect(dialog.locator('#facilityDeviceError .overseer-error-message')).toHaveText(reason);
  expect(await facilityState(request)).toEqual(before);

  await dialog.getByRole('button', { name: DONE, exact: true }).click();
  await expect(dialog).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 1);
  await expect(status).toHaveAttribute('data-unapplied-count', '0');
  expect((await facilityState(request)).facility.devices.find(value => value.id === FACILITY_IDS.devices.power).name)
    .toBe('Retry primary grid');
});

test('device dialog input survives a live session change and still saves on «ГОТОВО»', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  const dialog = await openPowerGraph(page);
  const name = dialog.getByLabel('НАЗВАНИЕ', { exact: true });
  await name.fill('Pending primary grid');

  // A live change arrives: the device goes offline. The authored name in the
  // event is ignored; only live values are taken.
  const sessionResponse = await request.get(`${FIXTURE}/session`);
  expect(sessionResponse.ok()).toBe(true);
  const canonical = await sessionResponse.json();
  const power = canonical.facility.devices.find(value => value.id === FACILITY_IDS.devices.power);
  power.currentStateId = 'offline';
  power.name = 'Ignored authored change';
  await page.evaluate(({ revision, session }) => {
    globalThis.__desktopFixture.emit('session-state', { revision, session });
  }, { revision: before.sessionRevision + 1, session: canonical });

  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Offline');
  await expect(deviceRow(page)).toContainText('Primary power grid');
  await expect(dialog).toBeVisible();
  await expect(name).toHaveValue('Pending primary grid');
  await expect(page.locator('#saveStatus')).not.toContainText('СЕССИЯ ИЗМЕНИЛАСЬ');
  expect(await facilityState(request)).toEqual(before);

  await dialog.getByRole('button', { name: DONE, exact: true }).click();
  await expect(dialog).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 1);
  const saved = (await facilityState(request)).facility.devices.find(value => value.id === FACILITY_IDS.devices.power);
  expect(saved.name).toBe('Pending primary grid');
});

test('keeps searchable device details adjacent on desktop and stacked without overflow on narrow screens', async ({ page, request }) => {
  await resetFixture(request);
  await page.setViewportSize({ width: 1440, height: 900 });
  await openWorkspace(page);
  await deviceRow(page).click();
  const list = page.locator('#facilityDeviceList');
  const detail = page.locator('#facilityDetailPanel');
  const [listBox, detailBox] = await Promise.all([list.boundingBox(), detail.boundingBox()]);
  expect(listBox).not.toBeNull();
  expect(detailBox).not.toBeNull();
  expect(detailBox.x).toBeGreaterThan(listBox.x + listBox.width - 2);
  await page.getByLabel('Поиск устройств', { exact: true }).fill('cooling');
  await expect(deviceRow(page, FACILITY_IDS.devices.cooling)).toBeVisible();
  await expect(deviceRow(page)).toHaveCount(0);
  await expect(page.locator('#facilityDeviceHeaderName')).toHaveText('Primary power grid');

  await page.setViewportSize({ width: 800, height: 700 });
  const noOverflow = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  expect(noOverflow).toBe(true);
  const [narrowList, narrowDetail] = await Promise.all([list.boundingBox(), detail.boundingBox()]);
  expect(narrowDetail.y).toBeGreaterThan(narrowList.y);
  const facilityTab = page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true });
  await facilityTab.focus();
  await facilityTab.press('ArrowLeft');
  await expect(page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true })).toHaveAttribute('aria-selected', 'true');
  await page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true }).press('ArrowRight');
  await expect(facilityTab).toHaveAttribute('aria-selected', 'true');
});

test('keeps the actual ten-device demo usable at desktop and narrow widths', async ({ page, request }) => {
  const demo = await seedDemoAuthoringSession(request);
  expect(demo.facility.devices).toHaveLength(10);
  await page.setViewportSize({ width: 1440, height: 900 });
  await openWorkspace(page);
  const device = page.locator('[data-facility-kind="device"][data-facility-id="power-grid-main"]');
  await expect(page.locator('[data-facility-kind="device"]')).toHaveCount(demo.facility.devices.length);
  await device.click();
  await expect(device).toBeFocused();

  const list = page.locator('#facilityDeviceList');
  const detail = page.locator('#facilityDetailPanel');
  const [listBox, detailBox] = await Promise.all([list.boundingBox(), detail.boundingBox()]);
  expect(listBox).not.toBeNull();
  expect(detailBox).not.toBeNull();
  expect(detailBox.x).toBeGreaterThan(listBox.x + listBox.width - 2);
  await expect(page.locator('#facilityDeviceHeaderName')).toHaveText('Резервная сеть Убежища 76');
  await expect(page.locator('#facilityActionBindingList, #facilityInformationBindingList')
    .locator('[data-facility-kind="binding"]')).not.toHaveCount(0);
  await page.screenshot({ path: 'test-results/facility-authoring-ux-demo-desktop.png' });
  const guide = await openGuide(page);
  await guide.getByRole('button', { name: '2. Действие', exact: true }).click();
  await expect(guide.getByRole('button', { name: 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ', exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/facility-story-guide-desktop.png' });

  await page.setViewportSize({ width: 800, height: 700 });
  const noOverflow = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  expect(noOverflow).toBe(true);
  const [narrowList, narrowDetail] = await Promise.all([list.boundingBox(), detail.boundingBox()]);
  expect(narrowDetail.y).toBeGreaterThan(narrowList.y);
  await page.screenshot({ path: 'test-results/facility-authoring-ux-demo-narrow.png' });
  await guide.getByRole('button', { name: '3. Результат', exact: true }).click();
  await guide.getByRole('button', { name: 'ИЗМЕНИТЬ ПРАВИЛО ОТОБРАЖЕНИЯ', exact: true }).scrollIntoViewIfNeeded();
  await expect(guide.getByRole('button', { name: 'СЛЕДУЮЩИЙ ШАГ', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/facility-story-guide-narrow.png' });
});

test('explains transitions in the matrix, opens them by keyboard, and makes fault activation explicit', async ({ page, request }) => {
  await seedMultiTransitionDevice(request);
  await openWorkspace(page);
  const dialog = await openPowerGraph(page);
  const device = deviceDialog(page);
  // Each filled cell names its transition and sums up its rules.
  const restore = device.cell('offline', 'online');
  await expect(restore).toHaveAttribute('aria-label', 'Offline → Online: Restore primary power');
  await expect(restore.locator('.facility-transition-cell-name')).toHaveText('Restore primary power');
  await expect(restore.locator('.facility-transition-cell-summary')).toHaveText('1 предусловие · снимает 1 неисправность');
  await expect(device.cell('online', 'offline').locator('.facility-transition-cell-summary'))
    .toHaveText('включает 1 неисправность');
  await expect(device.unavailable('offline', 'offline')).toHaveText('—');
  // The keyboard opens a cell, and folding the panel returns to the cell.
  await restore.focus();
  await restore.press('Enter');
  await expect(device.panel).toBeVisible();
  await expect(device.panelHeading).toHaveText('ПЕРЕХОД: Offline → Online');
  const transition = openTransitionRow(page);
  await expect(transition.locator('.facility-transition-name')).toBeFocused();
  // The panel says which fault the transition clears; the fault dialog edits it.
  const effect = transition.locator('.facility-transition-fault-effect');
  await expect(effect).toHaveAttribute('data-active', 'false');
  await expect(effect).toContainText('Снимает «Reactor controls unpowered»');
  await expect(effect.getByRole('button', { name: 'К НЕИСПРАВНОСТИ', exact: true })).toBeVisible();
  await page.locator('#btnCloseFacilityTransitionPanel').focus();
  await page.keyboard.press('Enter');
  await expect(device.panel).toBeHidden();
  await expect(restore).toBeFocused();
  expect(await dialog.locator('.facility-dialog-actions').evaluate(element => getComputedStyle(element).position)).toBe('sticky');
});

test('edits and removes bindings and an unreferenced recovery program, saving each edit', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  await openDetailTab(page, 'facilityTabInformation');
  const textBinding = page.locator('[data-facility-kind="binding"]').filter({ hasText: 'PRIMARY POWER: ONLINE' });
  await textBinding.getByRole('button', { name: /^Изменить:/ }).click();
  const bindingDialog = page.getByRole('dialog', { name: 'ПРАВИЛО ОТОБРАЖЕНИЯ', exact: true });
  await expect(ruleDialog(page).tab('entry-content')).toHaveAttribute('aria-selected', 'true');
  await expect(bindingDialog.getByLabel('ТЕКСТ', { exact: true })).toHaveValue('PRIMARY POWER: ONLINE');
  await bindingDialog.getByLabel('ТЕКСТ', { exact: true }).fill('PRIMARY POWER: RESTORED');
  await bindingDialog.getByRole('button', { name: DONE, exact: true }).click();
  const editedBinding = page.locator('[data-facility-kind="binding"]').filter({ hasText: 'PRIMARY POWER: RESTORED' });
  await expect(editedBinding).toBeVisible();
  await expectAutosaved(page, request, before.saveCalls + 1);
  await editedBinding.getByRole('button', { name: /^Удалить:/ }).click();
  await expect(editedBinding).toHaveCount(0);
  await expectAutosaved(page, request, before.saveCalls + 2);

  await openDetailTab(page, 'facilityTabPrograms');
  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true }).click();
  const programDialog = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await programDialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Temporary recovery');
  await fillProgramTransition(page, FACILITY_IDS.devices.power, 'restore');
  await fillTechnicalID(programDialog, 'facilityRecoveryProgramTechnical', 'program-temporary');
  await programDialog.getByRole('button', { name: DONE, exact: true }).click();
  const program = page.locator('[data-facility-kind="recovery-program"]').filter({ hasText: 'Temporary recovery' });
  await expectAutosaved(page, request, before.saveCalls + 3);
  await program.getByRole('button', { name: /^Изменить:/ }).click();
  await programDialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Temporary recovery updated');
  await programDialog.getByRole('button', { name: DONE, exact: true }).click();
  const updated = page.locator('[data-facility-kind="recovery-program"]').filter({ hasText: 'Temporary recovery updated' });
  await expectAutosaved(page, request, before.saveCalls + 4);
  await updated.getByRole('button', { name: /^Удалить:/ }).click();
  await expect(updated).toHaveCount(0);
  await expectAutosaved(page, request, before.saveCalls + 5);

  const saved = await facilityState(request);
  expect(saved.facility.devices).toEqual(before.facility.devices);
  expect(saved.facility.conditions).toEqual(before.facility.conditions);
  expect(saved.facility.recoveryPrograms).toEqual(before.facility.recoveryPrograms);
  expect(saved.bindingCount).toBe(before.bindingCount - 1);
  expect(saved.brokenReferenceCount).toBe(0);
});

// openDependencies opens «ЗАВИСИМОСТИ» for the selected device and returns
// the dialog and a row finder by dependency kind.
async function openDependencies(page) {
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  const report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ УСТРОЙСТВА', exact: true });
  await expect(report).toBeVisible();
  await expect(report.locator('#facilityDependencyStatus')).toHaveText(/^НАЙДЕНО ССЫЛОК: \d+$/);
  return { report, row: kind => report.locator(`.facility-dependency-row[data-kind="${kind}"]`) };
}

test('opens readable dependencies at display rules, commands, and composite transitions', async ({ page, request }) => {
  await seedMultiTransitionDevice(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  let { report, row } = await openDependencies(page);
  await expect(report.locator('#facilityDependencyDialogDescription'))
    .toHaveText('Что в локации зависит от устройства «Primary power grid».');
  const block = row('entry-content-variant');
  await expect(block.locator('.facility-dependency-sentence'))
    .toHaveText('Текст записи «FACILITY STATUS» (блок 1) меняется, когда Primary power grid = Online.');
  await expect(block.locator('.facility-dependency-place')).toHaveText('Security control › FACILITY STATUS › блок 1');
  await block.getByRole('button', { name: 'К ПРАВИЛУ ОТОБРАЖЕНИЯ: FACILITY STATUS', exact: true }).click();
  const rule = ruleDialog(page);
  await expect(rule.dialog).toBeVisible();
  await expect(rule.tab('entry-content')).toHaveAttribute('aria-selected', 'true');
  await expect(rule.block).toHaveValue(FACILITY_IDS.nodes.securityPowerBlock);
  await expect(rule.text).toHaveValue('PRIMARY POWER: ONLINE');
  await rule.cancel.click();

  ({ report, row } = await openDependencies(page));
  const availability = row('availability');
  await expect(availability.locator('.facility-dependency-sentence'))
    .toHaveText('Команду «START MAIN REACTOR» можно запросить, только когда Primary power grid = Online.');
  await availability.getByRole('button', { name: 'К ПРАВИЛУ ОТОБРАЖЕНИЯ: START MAIN REACTOR', exact: true }).click();
  await expect(rule.tab('availability')).toHaveAttribute('aria-selected', 'true');
  await expect(rule.where.locator('option:checked')).toHaveAttribute('data-node-id', FACILITY_IDS.nodes.startReactor);
  await rule.cancel.click();

  ({ report, row } = await openDependencies(page));
  const command = row('command-action');
  await expect(command.locator('.facility-dependency-sentence'))
    .toHaveText('Команда «START MAIN REACTOR» запускает переход «Restore primary power».');
  await expect(command.locator('.facility-dependency-place')).toHaveText('Reactor control › START MAIN REACTOR');
  await command.getByRole('button', { name: 'К КОМАНДЕ: START MAIN REACTOR', exact: true }).click();
  await expect(page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true })).toHaveAttribute('aria-selected', 'true');
  await expect(page.locator('#btnEditCommandFacilityAction')).toBeFocused();
  await expect(page.locator('.command-facility-action-summary')).toContainText('Primary power grid — Restore primary power');
  await expect(page.locator('.command-facility-action-summary')).toContainText('Требуется: Reactor cooling loop: Online');
  await page.getByRole('button', { name: 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ', exact: true }).click();
  const action = commandActionDialog(page);
  await expect(action.dialog).toBeVisible();
  await expect(action.commandFixed).toHaveText('Reactor control › START MAIN REACTOR');
  await expect(action.modeTransitions).toBeChecked();
  await expect(action.rows.first().locator('.facility-action-device')).toHaveValue(FACILITY_IDS.devices.power);
  await expect(action.rows.first().locator('.facility-action-transition')).toHaveValue('restore');
  await action.cancel.click();

  await switchTab(page, 'ОБЪЕКТЫ');
  await deviceRow(page, FACILITY_IDS.devices.cooling).click();
  ({ report, row } = await openDependencies(page));
  const precondition = row('transition-precondition').first();
  await expect(precondition.locator('.facility-dependency-sentence'))
    .toHaveText('Переход «Restore primary power» устройства «Primary power grid» требует: Reactor cooling loop = Online.');
  await expect(precondition.locator('.facility-dependency-place')).toHaveText('Primary power grid › Restore primary power');
  await precondition.getByRole('button', { name: 'К УСТРОЙСТВУ: Restore primary power', exact: true }).click();
  const device = deviceDialog(page);
  await expect(device.dialog).toBeVisible();
  await expect(page.locator('#facilityDeviceID')).toHaveValue(FACILITY_IDS.devices.power);
  await expect(openTransitionRow(page)).toHaveAttribute('data-transition-id', 'restore');
  await expect(openTransitionRow(page).locator('.facility-transition-name')).toBeFocused();
});

test('opens fault and recovery-program dependencies for editing and reports a missing target', async ({ page, request }) => {
  const before = await seedMultiTransitionDevice(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  let { report, row } = await openDependencies(page);
  const programDependency = row('recovery-program-transition');
  await expect(programDependency.locator('.facility-dependency-sentence'))
    .toHaveText('Программа восстановления «VAULT-TEC NETWORK RECOVERY» запускает переход «Restore primary power».');
  await programDependency.getByRole('button', { name: 'К ПРОГРАММЕ: VAULT-TEC NETWORK RECOVERY', exact: true }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await expect(program.getByLabel('НАЗВАНИЕ', { exact: true })).toHaveValue('VAULT-TEC NETWORK RECOVERY');
  // The program ID waits in the folded technical block.
  await expect(program.locator('#facilityRecoveryProgramTechnical')).not.toHaveAttribute('open', '');
  await expect(program.getByLabel('ИДЕНТИФИКАТОР', { exact: true }))
    .toHaveValue(FACILITY_IDS.programs.networkRecovery);
  await program.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();

  ({ report, row } = await openDependencies(page));
  const conditionDependency = row('diagnostic-scope');
  await expect(conditionDependency.locator('.facility-dependency-sentence'))
    .toHaveText('Неисправность «Reactor controls unpowered» действует на это устройство.');
  await conditionDependency.getByRole('button', { name: 'К НЕИСПРАВНОСТИ: Reactor controls unpowered', exact: true }).click();
  const condition = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await expect(condition.getByLabel('ИДЕНТИФИКАТОР', { exact: true }))
    .toHaveValue(FACILITY_IDS.conditions.unpowered);
  await condition.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Reactor controls unpowered updated');
  await condition.getByRole('button', { name: DONE, exact: true }).click();
  await expectAutosaved(page, request, before.saveCalls + 1);
  const saved = await facilityState(request);
  const originalCondition = before.facility.conditions
    .find(value => value.id === FACILITY_IDS.conditions.unpowered);
  expect(saved.facility.conditions.find(value => value.id === FACILITY_IDS.conditions.unpowered)).toEqual({
    ...originalCondition,
    name: 'Reactor controls unpowered updated',
  });

  await page.route(`**${FIXTURE}/inspect`, route => route.fulfill({
    json: {
      ok: true,
      sessionRevision: saved.sessionRevision,
      facilityRevision: saved.facility.revision,
      report: {
        target: { kind: 'device', entityId: FACILITY_IDS.devices.power },
        dependencies: [{
          kind: 'availability',
          sourceId: 'missing-command',
          targetId: FACILITY_IDS.devices.power,
          terminalId: 'missing-terminal',
          property: 'terminals[missing-terminal].root.children[missing-command].availableWhen',
        }],
      },
    },
  }));
  await deviceRow(page).click();
  ({ report } = await openDependencies(page));
  // An unresolved name falls back to its raw ID in the sentence only.
  const missing = report.locator('.facility-dependency-row');
  await expect(missing.locator('.facility-dependency-sentence')).toContainText('«missing-command»');
  await expect(missing.locator('.facility-dependency-place')).toHaveCount(0);
  await missing.getByRole('button', { name: 'К ПРАВИЛУ ОТОБРАЖЕНИЯ: missing-command', exact: true }).click();
  await expect(report).toBeVisible();
  await expect(report.locator('#facilityDependencyError'))
    .toContainText('Связанный элемент больше не существует. Закройте окно и откройте зависимости снова.');
});

test('compares preview effects side by side without publication, and unapplied input never blocks it', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  await page.getByRole('button', { name: 'ПРЕДПРОСМОТР СОСТОЯНИЯ', exact: true }).click();
  const preview = page.getByRole('dialog', { name: 'ПРЕДПРОСМОТР УСТРОЙСТВА', exact: true });
  await expect(preview.locator('#facilityPreviewContext'))
    .toHaveText('ПРЕДПРОСМОТР ПОКАЗЫВАЕТ ПРИМЕНЁННЫЕ ПРАВКИ И НЕ МЕНЯЕТ МИР ДЛЯ ИГРОКОВ.');
  // The first state that differs from the current «Online» is preselected.
  await expect(preview.getByLabel('СОСТОЯНИЕ', { exact: true })).toHaveValue('offline');
  await preview.getByLabel('ТЕРМИНАЛ', { exact: true }).selectOption(FACILITY_IDS.terminals.reactor);
  await expect(preview.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
  await expect(preview.locator('#facilityPreviewCurrentHeading')).toHaveText('СЕЙЧАС: Primary power grid: Online');
  await expect(preview.locator('#facilityPreviewHypotheticalHeading')).toHaveText('ЕСЛИ: Primary power grid: Offline');
  await expect(preview.locator('#facilityPreviewSummary')).toContainText(/^ИЗМЕНЕНИЙ: \d+/);
  const startReactor = `[data-preview-node-id="${FACILITY_IDS.nodes.startReactor}"]`;
  await expect(preview.locator(`#facilityPreviewSummary li[data-kind="becomes-unavailable"]`))
    .toContainText('СТАНЕТ НЕДОСТУПНО: START MAIN REACTOR');
  for (const pane of ['#facilityPreviewCurrent', '#facilityPreviewHypothetical']) {
    await expect(preview.locator(`${pane} ${startReactor}`)).toHaveAttribute('data-kind', 'becomes-unavailable');
  }
  await preview.getByRole('button', { name: 'ЗАКРЫТЬ', exact: true }).click();
  expect((await facilityState(request)).facility).toEqual(before.facility);
  expect((await facilityState(request)).publishedEvents).toBe(before.publishedEvents);

  // Unapplied input does not guard the preview, and it is still there after.
  // Devices are renamed in their dialog, so the input is a terminal's settings.
  const unapplied = 'НЕ ПРИМЕНЕНО: Настройки «Security control»';
  const introText = page.locator('#introTextArea');
  await page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true }).click();
  await page.locator(`.term-row[data-terminal-id="${FACILITY_IDS.terminals.security}"]`).click();
  await introText.fill('Unapplied preview text');
  await expect(page.locator('#saveStatus')).toHaveText(unapplied);
  await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  await page.getByRole('button', { name: 'ПРЕДПРОСМОТР СОСТОЯНИЯ', exact: true }).click();
  await expect(preview).toBeVisible();
  await expect(preview.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
  await expect(preview.locator('#facilityPreviewCurrentHeading')).toContainText('Primary power grid');
  await preview.getByRole('button', { name: 'ЗАКРЫТЬ', exact: true }).click();
  await expect(page.locator('#saveStatus')).toHaveText(unapplied);
  await page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true }).click();
  await expect(introText).toHaveValue('Unapplied preview text');
  await expect(introText).toHaveAttribute('data-unapplied', 'true');
  expect((await facilityState(request)).saveCalls).toBe(before.saveCalls);
});

test('guides an empty facility through a door story, command, and visible result, saving each step', async ({ page, request }) => {
  const reset = await request.post(`${FIXTURE}/reset`, { data: { scenario: 'empty' } });
  expect(reset.status()).toBe(204);
  const before = await facilityState(request);
  await openWorkspace(page);
  const guide = page.locator('#facilityStoryGuide');
  await expect(guide).toHaveAttribute('open', '');
  await expect(page.locator('#facilityStoryGuideToggle')).toHaveText('СКРЫТЬ');
  await expect(guide).toContainText('Устройств в сессии: 0');
  await expect(page.locator('#facilityStoryGuideStep')).toHaveText('КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 1 ИЗ 5: УСТРОЙСТВО');
  await expect(guide.getByRole('button', { name: '1. Устройство', exact: true })).toHaveAttribute('aria-current', 'step');
  await guide.getByRole('button', { name: 'СОЗДАТЬ УСТРОЙСТВО', exact: true }).click();
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true });
  await device.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Дверь в убежище');
  await device.getByLabel('ТИП', { exact: true }).selectOption('door');
  await addStates(page, ['Заперта', 'Открыта']);
  // The first state starts the device: its chip carries the star.
  expect((await stateChips(page)).map(({ name, initial }) => [name, initial]))
    .toEqual([['Заперта', true], ['Открыта', false]]);
  await device.getByRole('button', { name: DONE, exact: true }).click();
  // The first saved device collapses the guide to its summary line, which
  // reopens it on demand.
  await expect(guide).toContainText('Настраиваем «Дверь в убежище»');
  await expectAutosaved(page, request, before.saveCalls + 1);
  await expect(page.locator('#facilityStoryGuideStep')).toHaveText('КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 1 ИЗ 5: УСТРОЙСТВО');
  const door = (await facilityState(request)).facility.devices[0];
  const stateID = name => door.states.find(value => value.name === name).id;
  expect(door.initialStateId).toBe(stateID('Заперта'));
  await openGuide(page);
  await guide.getByRole('button', { name: 'СЛЕДУЮЩИЙ ШАГ', exact: true }).click();
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await expect(guide).toContainText('Сначала добавьте переход');
  await guide.getByRole('button', { name: 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', exact: true }).click();
  await addTransition(page, stateID('Заперта'), stateID('Открыта'), 'Открыть дверь');
  await device.getByRole('button', { name: DONE, exact: true }).click();
  await expectAutosaved(page, request, before.saveCalls + 2);
  await guide.getByRole('button', { name: 'СВЯЗАТЬ С КОМАНДОЙ', exact: true }).click();
  const action = commandActionDialog(page);
  await expect(action.dialog).toBeVisible();
  await expect(action.modeTransitions).toBeChecked();
  await expect(action.rows.first().locator('.facility-action-device option:checked')).toHaveText('Дверь в убежище');
  await action.rows.first().locator('.facility-action-transition').selectOption({ label: 'Открыть дверь' });
  await action.completedName.fill('Дверь открыта');
  await action.confirmationText.fill('Открыть дверь в убежище?');
  await action.save.click();
  await expect(guide).toContainText('команд: 1');
  await expectAutosaved(page, request, before.saveCalls + 3);
  await guide.getByRole('button', { name: '3. Результат', exact: true }).click();
  await guide.getByRole('button', { name: 'НАСТРОИТЬ РЕЗУЛЬТАТ НА ТЕРМИНАЛЕ', exact: true }).click();
  const rule = ruleDialog(page);
  await expect(rule.tab('entry-content')).toHaveAttribute('aria-selected', 'true');
  await expect(rule.device).toHaveValue(door.id);
  await rule.block.selectOption({ index: 1 });
  await rule.state.selectOption({ label: 'Открыта' });
  await rule.text.fill('Путь свободен');
  await rule.save.click();
  await expect(page.getByRole('list', { name: 'Что делают игроки', exact: true })).toContainText('Открыть дверь');
  await openDetailTab(page, 'facilityTabInformation');
  await expect(page.getByRole('list', { name: 'Что видят игроки', exact: true })).toContainText('Путь свободен');
  await expectAutosaved(page, request, before.saveCalls + 4);
  await guide.getByRole('button', { name: '4. Осложнения', exact: true }).click();
  await expect(guide).toContainText('Необязательно');
  await guide.getByRole('button', { name: '5. Проверка', exact: true }).click();
  await expect(guide).toContainText('Все правки уже сохранены');
  await expect(guide.getByRole('button', { name: 'ОТКРЫТЬ ТЕРМИНАЛЫ', exact: true })).toBeVisible();
  const saved = await facilityState(request);
  expect(saved.saveCalls).toBe(before.saveCalls + 4);
  expect(saved.brokenReferenceCount).toBe(0);
  await guide.getByRole('button', { name: 'ПРОВЕРИТЬ В ПРЕДПРОСМОТРЕ', exact: true }).click();
  const preview = page.getByRole('dialog', { name: 'ПРЕДПРОСМОТР УСТРОЙСТВА', exact: true });
  await expect(preview.getByLabel('СОСТОЯНИЕ', { exact: true })).toHaveValue(stateID('Открыта'));
  await expect(preview.locator('#facilityPreviewHypothetical')).toContainText('Путь свободен');
  await preview.getByRole('button', { name: 'ЗАКРЫТЬ', exact: true }).click();
  const after = await facilityState(request);
  expect(after.facility).toEqual(saved.facility);
  expect(after.publishedEvents).toBe(saved.publishedEvents);
});

test('connects optional recovery from the story guide to a command without losing other program data', async ({ page, request }) => {
  const before = await seedMultiTransitionDevice(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  const guide = await openGuide(page);
  await guide.getByRole('button', { name: '4. Осложнения', exact: true }).click();
  await guide.getByRole('button', { name: 'ДОБАВИТЬ НЕИСПРАВНОСТЬ', exact: true }).click();
  const condition = page.getByRole('dialog', { name: 'НЕИСПРАВНОСТЬ', exact: true });
  await expect(condition.getByLabel('УСТРОЙСТВО', { exact: true })).toHaveValue(FACILITY_IDS.devices.power);
  await condition.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await guide.getByRole('button', { name: 'СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await program.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Восстановить путь');
  await fillProgramTransition(page, FACILITY_IDS.devices.power, 'restore');
  await fillTechnicalID(program, 'facilityRecoveryProgramTechnical', 'program-story');
  await program.getByRole('button', { name: DONE, exact: true }).click();
  await expectAutosaved(page, request, before.saveCalls + 1);
  await guide.getByRole('button', { name: '2. Действие', exact: true }).click();
  await guide.getByRole('button', { name: 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ', exact: true }).click();
  const action = commandActionDialog(page);
  await expect(action.dialog).toBeVisible();
  await action.modeProgram.check();
  await action.program.selectOption('program-story');
  await expect(action.rows.first()).toBeHidden();
  await action.save.click();
  await expectAutosaved(page, request, before.saveCalls + 2);
  await expect(page.getByRole('list', { name: 'Что делают игроки', exact: true })).toContainText('Восстановить путь');
  await guide.getByRole('button', { name: '2. Действие', exact: true }).click();
  await guide.getByRole('button', { name: 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ', exact: true }).click();
  await expect(action.modeProgram).toBeChecked();
  await expect(action.program).toHaveValue('program-story');
  await action.cancel.click();
  const saved = await facilityState(request);
  expect(saved.saveCalls).toBe(before.saveCalls + 2);
  expect(saved.facility.recoveryPrograms).toEqual([...before.facility.recoveryPrograms, {
    id: 'program-story', name: 'Восстановить путь',
    transitions: [{ deviceId: FACILITY_IDS.devices.power, transitionId: 'restore' }],
  }]);
  expect(saved.brokenReferenceCount).toBe(0);
  await openWorkspace(page);
  await deviceRow(page).click();
  await expect(page.getByRole('list', { name: 'Что делают игроки', exact: true })).toContainText('Восстановить путь');
});

test('keeps story guidance contextual after selection and undo and supports keyboard collapse', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  await applyName(page, 'Story guide device');
  const guide = page.locator('#facilityStoryGuide');
  await expect(guide).toContainText('Настраиваем «Story guide device»');
  await expectAutosaved(page, request, before.saveCalls + 1);
  await undo(page);
  await expect(guide).toContainText('Настраиваем «Primary power grid»');
  await expectAutosaved(page, request, before.saveCalls + 2);
  await deviceRow(page, FACILITY_IDS.devices.cooling).click();
  await expect(guide).toContainText('Настраиваем «Reactor cooling loop»');
  // The session has devices, so the guide starts as its one-line summary,
  // which opens and closes from the keyboard.
  const summary = page.locator('#facilityStoryGuideSummary');
  const toggle = page.locator('#facilityStoryGuideToggle');
  await expect(guide).not.toHaveAttribute('open', '');
  await expect(toggle).toHaveText('ПОКАЗАТЬ');
  await summary.focus();
  await page.keyboard.press('Enter');
  await expect(guide).toHaveAttribute('open', '');
  await expect(toggle).toHaveText('СКРЫТЬ');
  await guide.getByRole('button', { name: '3. Результат', exact: true }).focus();
  await page.keyboard.press('Enter');
  await expect(guide.getByRole('button', { name: '3. Результат', exact: true })).toHaveAttribute('aria-current', 'step');
  await summary.focus();
  await page.keyboard.press('Enter');
  await expect(guide).not.toHaveAttribute('open', '');
  await expect(toggle).toHaveText('ПОКАЗАТЬ');
  await expect(page.locator('#facilityStoryGuideStep')).toHaveText('КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 3 ИЗ 5: РЕЗУЛЬТАТ');
  await expect(page.getByRole('button', { name: 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', exact: true })).toBeVisible();
  await page.keyboard.press('Enter');
  await expect(guide).toHaveAttribute('open', '');
  await expect(toggle).toHaveText('СКРЫТЬ');
  expect(authoredFacility(await facilityState(request))).toEqual(authoredFacility(before));
});

test('explains missing terminal destinations in the story guide and opens terminal configuration', async ({ page, request }) => {
  const reset = await request.post(`${FIXTURE}/reset`, { data: { scenario: 'empty' } });
  expect(reset.status()).toBe(204);
  const before = await facilityState(request);
  const response = await request.get(`${FIXTURE}/session`);
  expect(response.ok()).toBe(true);
  const session = await response.json();
  for (const terminal of session.terminals) terminal.root.children = [];
  session.facility.devices = [{
    id: 'door-story', name: 'Дверь без команд', kind: 'door', initialStateId: 'closed', currentStateId: 'closed',
    states: [{ id: 'closed', name: 'Заперта' }, { id: 'open', name: 'Открыта' }],
    transitions: [{ id: 'open', name: 'Открыть дверь', sourceStateId: 'closed', destinationStateId: 'open' }],
  }];
  const seeded = await request.post(`${FIXTURE}/save`, { data: {
    session, expectedSessionRevision: before.sessionRevision, expectedFacilityRevision: before.facility.revision,
    correlationId: 'story-no-terminal-content',
  } });
  expect(await seeded.json()).toMatchObject({ ok: true });
  const baseline = await facilityState(request);
  await openWorkspace(page);
  await deviceRow(page, 'door-story').click();
  const guide = await openGuide(page);
  await guide.getByRole('button', { name: '2. Действие', exact: true }).click();
  await expect(guide).toContainText('Нет свободной команды');
  await guide.getByRole('button', { name: 'ОТКРЫТЬ ТЕРМИНАЛЫ', exact: true }).click();
  await expect(page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true })).toBeFocused();
  await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  await guide.getByRole('button', { name: '3. Результат', exact: true }).click();
  await expect(guide).toContainText('Сначала создайте запись');
  expect(await facilityState(request)).toEqual(baseline);
});

for (const kind of ['command', 'entry']) {
  test(`opens the terminals from the story guide to create a missing ${kind} and resumes binding without losing edits`, async ({ page, request }) => {
    await page.setViewportSize(kind === 'command' ? { width: 1440, height: 900 } : { width: 800, height: 700 });
    await request.post(`${FIXTURE}/reset`, { data: { scenario: 'empty' } });
    const before = await facilityState(request);
    const session = await (await request.get(`${FIXTURE}/session`)).json();
    for (const terminal of session.terminals) terminal.root.children = [];
    const seeded = await request.post(`${FIXTURE}/save`, { data: {
      session, expectedSessionRevision: before.sessionRevision,
      expectedFacilityRevision: before.facility.revision, correlationId: `story-missing-${kind}`,
    } });
    expect(await seeded.json()).toMatchObject({ ok: true });
    const baseline = await facilityState(request);
    await openWorkspace(page);
    const guide = page.locator('#facilityStoryGuide');
    await guide.getByRole('button', { name: 'СОЗДАТЬ УСТРОЙСТВО', exact: true }).click();
    const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true });
    await device.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Дверь нового маршрута');
    await device.getByLabel('ТИП', { exact: true }).selectOption('door');
    await addStates(page, ['Заперта', 'Открыта']);
    const [closed, open] = await stateChips(page);
    expect(closed.initial).toBe(true);
    await addTransition(page, closed.id, open.id, 'Открыть дверь');
    // The device ID waits in the folded technical block.
    await deviceDialog(page).technical.locator('summary').click();
    await page.locator('#facilityDeviceID').fill('door-checkpoint');
    await device.getByRole('button', { name: DONE, exact: true }).click();
    await expect(device).not.toBeVisible();
    await expectAutosaved(page, request, baseline.saveCalls + 1);
    const checkpoint = await facilityState(request);
    expect(checkpoint.facility.devices[0]).toMatchObject({
      id: 'door-checkpoint', initialStateId: closed.id,
      transitions: [{ name: 'Открыть дверь', sourceStateId: closed.id, destinationStateId: open.id }],
    });

    // The first saved device collapsed the guide; the author reopens it.
    await openGuide(page);
    await guide.getByRole('button', { name: kind === 'command' ? '2. Действие' : '3. Результат', exact: true }).click();
    await expect(guide).toContainText(kind === 'command' ? 'Нет свободной команды' : 'Сначала создайте запись');
    const openTerminals = guide.getByRole('button', { name: 'ОТКРЫТЬ ТЕРМИНАЛЫ', exact: true });
    await openTerminals.scrollIntoViewIfNeeded();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({ path: `test-results/facility-story-checkpoint-${kind}.png` });
    // The device is already saved, so opening the terminals saves nothing more.
    await openTerminals.click();
    await expect(page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true })).toBeFocused();
    expect((await facilityState(request)).saveCalls).toBe(checkpoint.saveCalls);
    await expectSaved(page);

    await page.locator(kind === 'command' ? '#btnAddCommand' : '#btnAddEntry').click();
    await expect.poll(async () => (await facilityState(request)).saveCalls).toBe(checkpoint.saveCalls + 1);
    await page.locator('#fldName').fill(kind === 'command' ? 'Открыть дверь' : 'Состояние шлюза');
    if (kind === 'entry') {
      await page.locator('#btnAddEntryContentBlock').click();
      await page.locator('[data-entry-block-content]').fill('Дверь заперта');
    } else {
      await page.locator('#fldText').fill('Дверь открыта');
    }
    await page.locator('#btnApplyNode').click();
    await expect.poll(async () => (await facilityState(request)).saveCalls).toBe(checkpoint.saveCalls + 2);
    await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
    await expect(guide).toContainText('Настраиваем «Дверь нового маршрута»');
    await guide.getByRole('button', {
      name: kind === 'command' ? 'СВЯЗАТЬ С КОМАНДОЙ' : 'НАСТРОИТЬ РЕЗУЛЬТАТ НА ТЕРМИНАЛЕ', exact: true,
    }).click();
    if (kind === 'command') {
      const action = commandActionDialog(page);
      await expect(action.dialog).toBeVisible();
      await action.rows.first().locator('.facility-action-transition').selectOption({ label: 'Открыть дверь' });
      await action.completedName.fill('Дверь открыта');
      await action.confirmationText.fill('Открыть дверь?');
      await action.save.click();
    } else {
      const rule = ruleDialog(page);
      await expect(rule.tab('entry-content')).toHaveAttribute('aria-selected', 'true');
      await rule.block.selectOption({ index: 1 });
      await rule.state.selectOption({ label: 'Открыта' });
      await rule.text.fill('Путь свободен');
      await rule.save.click();
    }
    await expectAutosaved(page, request, checkpoint.saveCalls + 3);
    const saved = await facilityState(request);
    expect(saved.brokenReferenceCount).toBe(0);
    expect(saved.facility.devices).toEqual(checkpoint.facility.devices);
    if (kind === 'entry') await openDetailTab(page, 'facilityTabInformation');
    await expect(page.getByRole('list', { name: kind === 'command' ? 'Что делают игроки' : 'Что видят игроки', exact: true }))
      .toContainText(kind === 'command' ? 'Открыть дверь' : 'Путь свободен');
  });
}

test('explains the optional recovery prerequisite and resumes after adding a transition', async ({ page, request }) => {
  await request.post(`${FIXTURE}/reset`, { data: { scenario: 'empty' } });
  const before = await facilityState(request);
  const session = await (await request.get(`${FIXTURE}/session`)).json();
  session.facility.devices = [{
    id: 'door-no-transitions', name: 'Дверь без переходов', kind: 'door',
    initialStateId: 'closed', currentStateId: 'closed',
    states: [{ id: 'closed', name: 'Заперта' }, { id: 'open', name: 'Открыта' }], transitions: [],
  }];
  const seeded = await request.post(`${FIXTURE}/save`, { data: {
    session, expectedSessionRevision: before.sessionRevision,
    expectedFacilityRevision: before.facility.revision, correlationId: 'story-recovery-prerequisite',
  } });
  expect(await seeded.json()).toMatchObject({ ok: true });
  const baseline = await facilityState(request);
  await page.setViewportSize({ width: 800, height: 700 });
  await openWorkspace(page);
  await deviceRow(page, 'door-no-transitions').click();
  const guide = await openGuide(page);
  await guide.getByRole('button', { name: '4. Осложнения', exact: true }).click();
  await expect(guide).toContainText('Если нужна программа восстановления, сначала добавьте переход устройства');
  await expect(guide).toContainText('Для простой сцены этот шаг можно пропустить');
  await expect(guide.getByRole('button', { name: 'СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true })).toHaveCount(0);
  const editDevice = guide.getByRole('button', { name: 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', exact: true });
  await expect(editDevice).toHaveCount(1);
  await editDevice.focus();
  await page.screenshot({ path: 'test-results/facility-story-recovery-prerequisite-narrow.png' });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.keyboard.press('Enter');
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО', exact: true });
  await expect(device.getByLabel('НАЗВАНИЕ', { exact: true })).toHaveValue('Дверь без переходов');
  await addTransition(page, 'closed', 'open', 'Открыть дверь');
  await deviceDialog(page).technical.locator('summary').click();
  await deviceDialog(page).technical.locator('.facility-transition-id').fill('open');
  await device.getByRole('button', { name: DONE, exact: true }).click();
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await expect(guide).toContainText('переходов: 1');
  await expectAutosaved(page, request, baseline.saveCalls + 1);
  await expect(guide).not.toContainText('сначала добавьте переход устройства');
  await guide.getByRole('button', { name: 'СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await program.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Восстановить проход');
  await fillProgramTransition(page, 'door-no-transitions', 'open');
  await program.getByRole('button', { name: DONE, exact: true }).click();
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await expectAutosaved(page, request, baseline.saveCalls + 2);
  expect((await facilityState(request)).facility.recoveryPrograms).toContainEqual(expect.objectContaining({
    name: 'Восстановить проход', transitions: [{ deviceId: 'door-no-transitions', transitionId: 'open' }],
  }));
});
