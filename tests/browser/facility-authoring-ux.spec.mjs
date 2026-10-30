import { readFile } from 'node:fs/promises';

import { expect, test } from '@playwright/test';

import { buildFacilityAuthoringSession, FACILITY_IDS } from './fixtures/facility-session.mjs';
import { expectSaved, undo } from './helpers/authoring-bar.mjs';

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

async function openPowerGraph(page) {
  await deviceRow(page).click();
  await page.getByRole('button', { name: 'РЕДАКТИРОВАТЬ ГРАФ', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function expandReferences(transition) {
  const collapsed = transition.locator('details.facility-transition-references:not([open]) > summary');
  if (await collapsed.count()) await collapsed.click();
}

async function expectPowerReferences(dialog) {
  const first = dialog.locator('.facility-transition-row').first();
  await expect(first.locator('.facility-transition-id')).toHaveValue('restore');
  await expect(first.locator('.facility-precondition-device')).toHaveValue(FACILITY_IDS.devices.cooling);
  await expect(first.locator('.facility-precondition-state')).toHaveValue('online');
  await expect(first.locator('.facility-effect-condition')).toHaveValue(FACILITY_IDS.conditions.unpowered);
  await expect(dialog.locator('.facility-transition-row').nth(1).locator('.facility-effect-condition'))
    .toHaveValue(FACILITY_IDS.conditions.unpowered);
}

test('preserves multi-transition references and recovery metadata through state edits, autosave, and reopen', async ({ page, request }) => {
  const before = await seedMultiTransitionDevice(request);
  const original = before.facility.devices.find(value => value.id === FACILITY_IDS.devices.power);
  expect(original.transitions).toHaveLength(2);
  expect(original.currentStateId).not.toBe(original.initialStateId);
  await openWorkspace(page);
  const dialog = await openPowerGraph(page);

  await expectPowerReferences(dialog);
  await dialog.locator('.facility-state-name').first().fill('Power isolated');
  await expectPowerReferences(dialog);
  await dialog.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД', exact: true }).click();
  const added = dialog.locator('.facility-transition-row').last();
  await added.locator('.facility-transition-id').fill('restart');
  await added.locator('.facility-transition-name').fill('Restart primary grid');
  await added.locator('.facility-transition-source').selectOption('online');
  await added.locator('.facility-transition-destination').selectOption('offline');
  await expectPowerReferences(dialog);
  await expect(dialog.locator('#btnSaveFacilityDevice')).toHaveText(DONE);
  await dialog.locator('#btnSaveFacilityDevice').click();
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
  await expectPowerReferences(await openPowerGraph(page));
});

for (const missing of ['prerequisite device', 'prerequisite state', 'condition effect']) {
  test(`rejects an incomplete ${missing} row with focused feedback and no lost draft row`, async ({ page, request }) => {
    await resetFixture(request);
    const before = await facilityState(request);
    await openWorkspace(page);
    const dialog = await openPowerGraph(page);
    const transition = dialog.locator('.facility-transition-row').first();
    await expandReferences(transition);
    let invalid;
    if (missing === 'condition effect') {
      await transition.getByRole('button', { name: 'ДОБАВИТЬ ЭФФЕКТ УСЛОВИЯ', exact: true }).click();
      invalid = transition.locator('.facility-effect-condition').last();
    } else {
      await transition.getByRole('button', { name: 'ДОБАВИТЬ ПРЕДУСЛОВИЕ', exact: true }).click();
      const row = transition.locator('.facility-precondition-row').last();
      invalid = row.locator('.facility-precondition-device');
      if (missing === 'prerequisite state') {
        await invalid.selectOption(FACILITY_IDS.devices.cooling);
        invalid = row.locator('.facility-precondition-state');
      }
    }
    await dialog.locator('#btnSaveFacilityDevice').click();
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('[role="alert"]')).toBeVisible();
    await expect(dialog.locator('[role="alert"]')).not.toBeEmpty();
    await expect(invalid).toHaveAttribute('aria-invalid', 'true');
    await expect(invalid).toBeFocused();
    expect(await facilityState(request)).toEqual(before);
    await dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
    expect(await facilityState(request)).toEqual(before);
  });
}

async function applyName(page, name) {
  await deviceRow(page).click();
  const editor = page.locator('#facilitySelectionEditor');
  await editor.getByLabel('НАЗВАНИЕ УСТРОЙСТВА', { exact: true }).fill(name);
  await editor.getByRole('button', { name: DONE, exact: true }).click();
  await expect(deviceRow(page)).toContainText(name);
}

async function applyCondition(page) {
  await page.getByRole('button', { name: 'ДОБАВИТЬ УСЛОВИЕ', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', exact: true });
  await dialog.getByLabel('ИДЕНТИФИКАТОР', { exact: true }).fill('condition-staged-ux');
  await dialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Staged power fault');
  await dialog.getByLabel('КАТЕГОРИЯ', { exact: true }).selectOption('unpowered');
  await dialog.getByLabel('УСТРОЙСТВО', { exact: true }).selectOption(FACILITY_IDS.devices.power);
  await dialog.getByLabel('БЛОКИРУЕМАЯ ВОЗМОЖНОСТЬ', { exact: true }).selectOption('execute-command');
  await dialog.getByLabel('ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ', { exact: true }).check();
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

  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await program.getByLabel('ИДЕНТИФИКАТОР', { exact: true }).fill('program-staged-ux');
  await program.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Staged recovery');
  await program.getByLabel('УСТРОЙСТВО', { exact: true }).selectOption(FACILITY_IDS.devices.power);
  await program.getByLabel('ПЕРЕХОД', { exact: true }).selectOption('restore');
  await program.getByRole('button', { name: DONE, exact: true }).click();
  await expect(program).toBeHidden();
  await expectAutosaved(page, request, before.saveCalls + 3);

  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРИВЯЗКУ', exact: true }).click();
  const binding = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА', exact: true });
  await binding.getByLabel('ТЕРМИНАЛ', { exact: true }).selectOption(FACILITY_IDS.terminals.reactor);
  await binding.getByLabel('ЭЛЕМЕНТ', { exact: true }).selectOption(FACILITY_IDS.nodes.startReactor);
  await binding.getByLabel('УСТРОЙСТВО', { exact: true }).selectOption(FACILITY_IDS.devices.power);
  await binding.getByLabel('СОСТОЯНИЕ', { exact: true }).selectOption('online');
  await binding.getByLabel('ТЕКСТ', { exact: true }).fill('Staged startup label');
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
  await deviceRow(page).click();
  const name = page.locator('#facilitySelectedDeviceName');
  await name.fill('Retry primary grid');
  await page.locator('#btnSaveFacilityDeviceChanges').click();

  const status = page.locator('#saveStatus');
  await expect(status).toHaveAttribute('data-state', 'error');
  await expect(status).toContainText('ОШИБКА СОХРАНЕНИЯ: НЕ УДАЛОСЬ СОХРАНИТЬ СОСТОЯНИЕ ОБЪЕКТА');
  await expect(status).toContainText('НЕ ПРИМЕНЕНО: Устройство «Primary power grid»');
  // Only the rejected edit is reverted, and its input comes back unapplied.
  await expect(deviceRow(page)).toContainText('Primary power grid');
  await expect(name).toHaveValue('Retry primary grid');
  await expect(name).toHaveAttribute('data-unapplied', 'true');
  expect(await facilityState(request)).toEqual(before);

  await page.locator('#btnSaveFacilityDeviceChanges').click();
  await expectAutosaved(page, request, before.saveCalls + 1);
  await expect(status).toHaveAttribute('data-unapplied-count', '0');
  await expect(name).not.toHaveAttribute('data-unapplied', 'true');
  expect((await facilityState(request)).facility.devices.find(value => value.id === FACILITY_IDS.devices.power).name)
    .toBe('Retry primary grid');
});

test('unapplied input survives a live session change and still saves on «ГОТОВО»', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  const name = page.locator('#facilitySelectedDeviceName');
  await name.fill('Pending primary grid');
  await expect(page.locator('#saveStatus')).toHaveText('НЕ ПРИМЕНЕНО: Устройство «Primary power grid»');

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

  await expect(page.locator('#facilityDeviceSummary')).toContainText('Сейчас: Offline');
  await expect(deviceRow(page)).toContainText('Primary power grid');
  await expect(name).toHaveValue('Pending primary grid');
  await expect(name).toHaveAttribute('data-unapplied', 'true');
  await expect(page.locator('#saveStatus')).not.toContainText('СЕССИЯ ИЗМЕНИЛАСЬ');
  expect(await facilityState(request)).toEqual(before);

  await page.locator('#btnSaveFacilityDeviceChanges').click();
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
  await expect(page.getByLabel('НАЗВАНИЕ УСТРОЙСТВА', { exact: true })).toHaveValue('Primary power grid');

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
  await expect(page.getByLabel('НАЗВАНИЕ УСТРОЙСТВА', { exact: true }))
    .toHaveValue('Резервная сеть Убежища 76');
  await expect(page.locator('#facilityBindingList [data-facility-kind="binding"]')).not.toHaveCount(0);
  await page.screenshot({ path: 'test-results/facility-authoring-ux-demo-desktop.png' });
  const guide = page.locator('#facilityStoryGuide');
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

test('explains transitions, exposes optional rules by keyboard, and makes fault activation explicit', async ({ page, request }) => {
  await seedMultiTransitionDevice(request);
  await openWorkspace(page);
  const dialog = await openPowerGraph(page);
  const transition = dialog.locator('.facility-transition-row').first();
  await expect(transition.locator('.facility-transition-summary'))
    .toContainText('Restore primary power: Offline → Online');
  const rules = transition.locator('details.facility-transition-references');
  await expect(rules.locator('summary')).toContainText('ПРЕДУСЛОВИЯ: 1 · ЭФФЕКТЫ: 1');
  await rules.locator('summary').focus();
  await rules.locator('summary').press('Enter');
  await expect(rules).not.toHaveAttribute('open', '');
  await rules.locator('summary').press('Enter');
  await expect(rules).toHaveAttribute('open', '');
  const effect = transition.getByLabel('ДЕЙСТВИЕ С НЕИСПРАВНОСТЬЮ', { exact: true });
  await expect(effect.locator('option')).toHaveText(['АКТИВИРОВАТЬ НЕИСПРАВНОСТЬ', 'СНЯТЬ НЕИСПРАВНОСТЬ']);
  await expect(effect).toHaveValue('remove');
  expect(await dialog.locator('.facility-dialog-actions').evaluate(element => getComputedStyle(element).position)).toBe('sticky');
});

test('edits and removes bindings and an unreferenced recovery program, saving each edit', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  const textBinding = page.locator('[data-facility-kind="binding"]').filter({ hasText: 'PRIMARY POWER: ONLINE' });
  await textBinding.getByRole('button', { name: /^Изменить:/ }).click();
  const bindingDialog = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА', exact: true });
  await expect(bindingDialog.getByLabel('ТЕКСТ', { exact: true })).toHaveValue('PRIMARY POWER: ONLINE');
  await bindingDialog.getByLabel('ТЕКСТ', { exact: true }).fill('PRIMARY POWER: RESTORED');
  await bindingDialog.getByRole('button', { name: DONE, exact: true }).click();
  const editedBinding = page.locator('[data-facility-kind="binding"]').filter({ hasText: 'PRIMARY POWER: RESTORED' });
  await expect(editedBinding).toBeVisible();
  await expectAutosaved(page, request, before.saveCalls + 1);
  await editedBinding.getByRole('button', { name: /^Удалить:/ }).click();
  await expect(editedBinding).toHaveCount(0);
  await expectAutosaved(page, request, before.saveCalls + 2);

  await page.getByRole('button', { name: 'ДОБАВИТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true }).click();
  const programDialog = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await programDialog.getByLabel('ИДЕНТИФИКАТОР', { exact: true }).fill('program-temporary');
  await programDialog.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Temporary recovery');
  await programDialog.getByLabel('УСТРОЙСТВО', { exact: true }).selectOption(FACILITY_IDS.devices.power);
  await programDialog.getByLabel('ПЕРЕХОД', { exact: true }).selectOption('restore');
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

test('opens readable dependencies at blocks, commands, and composite transitions', async ({ page, request }) => {
  await seedMultiTransitionDevice(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  let report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ ОБЪЕКТА', exact: true });
  const blockDependency = report.locator('.facility-dependency-row').filter({ hasText: 'Текст блока записи' });
  await expect(blockDependency).toContainText('Security control › FACILITY STATUS');
  await blockDependency.getByRole('button', { name: /^Открыть ссылку/ }).click();
  await expect(page.locator(`[data-block-id="${FACILITY_IDS.nodes.securityPowerBlock}"] textarea`)).toBeFocused();

  await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  await deviceRow(page).click();
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ ОБЪЕКТА', exact: true });
  const commandDependency = report.locator('.facility-dependency-row').filter({ hasText: 'Доступность команды' });
  await commandDependency.getByRole('button', { name: /^Открыть ссылку/ }).click();
  await expect(page.locator('.command-facility-action-summary')).toContainText('Primary power grid — Restore primary power');
  await expect(page.locator('.command-facility-action-summary')).toContainText('Требуется: Reactor cooling loop: Online');
  await page.getByRole('button', { name: 'ИЗМЕНИТЬ ДЕЙСТВИЕ УСТРОЙСТВ', exact: true }).click();
  const binding = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА', exact: true });
  await expect(binding.getByLabel('ТИП ПРИВЯЗКИ', { exact: true })).toHaveValue('command-action');
  await expect(binding.getByLabel('ТЕРМИНАЛ', { exact: true })).toHaveValue(FACILITY_IDS.terminals.reactor);
  await expect(binding.getByLabel('ЭЛЕМЕНТ', { exact: true })).toHaveValue(FACILITY_IDS.nodes.startReactor);
  await binding.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();

  await page.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  await deviceRow(page, FACILITY_IDS.devices.cooling).click();
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ ОБЪЕКТА', exact: true });
  const transitionDependency = report.locator('.facility-dependency-row').filter({ hasText: 'Предусловие перехода' }).first();
  await transitionDependency.getByRole('button', { name: /^Открыть ссылку/ }).click();
  const deviceDialog = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
  await expect(deviceDialog.locator('.facility-transition-id:focus')).toHaveValue('restore');
});

test('opens condition and recovery-program dependencies for editing and reports a missing target', async ({ page, request }) => {
  const before = await seedMultiTransitionDevice(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  let report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ ОБЪЕКТА', exact: true });
  const programDependency = report.locator('.facility-dependency-row')
    .filter({ hasText: 'Программа восстановления' });
  await programDependency.getByRole('button', { name: /^Открыть ссылку/ }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await expect(program.getByLabel('ИДЕНТИФИКАТОР', { exact: true }))
    .toHaveValue(FACILITY_IDS.programs.networkRecovery);
  await program.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();

  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ ОБЪЕКТА', exact: true });
  const conditionDependency = report.locator('.facility-dependency-row')
    .filter({ hasText: 'Область неисправности' });
  await conditionDependency.getByRole('button', { name: /^Открыть ссылку/ }).click();
  const condition = page.getByRole('dialog', { name: 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', exact: true });
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
  await page.getByRole('button', { name: 'ЗАВИСИМОСТИ', exact: true }).click();
  report = page.getByRole('dialog', { name: 'ЗАВИСИМОСТИ ОБЪЕКТА', exact: true });
  await expect(report).toContainText('missing-command');
  await report.getByRole('button', { name: /^Открыть ссылку/ }).click();
  await expect(report.locator('#facilityDependencyError')).toContainText('БОЛЬШЕ НЕ СУЩЕСТВУЕТ');
});

test('compares preview effects side by side without publication, and unapplied input never blocks it', async ({ page, request }) => {
  await resetFixture(request);
  const before = await facilityState(request);
  await openWorkspace(page);
  await deviceRow(page).click();
  await page.getByRole('button', { name: 'ПРЕДПРОСМОТР СОСТОЯНИЯ', exact: true }).click();
  const preview = page.getByRole('dialog', { name: 'ПРЕДПРОСМОТР ОБЪЕКТА', exact: true });
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
  await page.locator('#facilitySelectedDeviceName').fill('Unapplied preview name');
  await page.getByRole('button', { name: 'ПРЕДПРОСМОТР СОСТОЯНИЯ', exact: true }).click();
  await expect(preview).toBeVisible();
  await expect(preview.locator('#facilityPreviewStatus')).toContainText('ПРЕДПРОСМОТР ГОТОВ');
  await expect(preview.locator('#facilityPreviewCurrentHeading')).toContainText('Primary power grid');
  await preview.getByRole('button', { name: 'ЗАКРЫТЬ', exact: true }).click();
  await expect(page.locator('#facilitySelectedDeviceName')).toHaveValue('Unapplied preview name');
  await expect(page.locator('#saveStatus')).toHaveText('НЕ ПРИМЕНЕНО: Устройство «Primary power grid»');
  expect((await facilityState(request)).saveCalls).toBe(before.saveCalls);
});

test('guides an empty facility through a door story, command, and visible result, saving each step', async ({ page, request }) => {
  const reset = await request.post(`${FIXTURE}/reset`, { data: { scenario: 'empty' } });
  expect(reset.status()).toBe(204);
  const before = await facilityState(request);
  await openWorkspace(page);
  const guide = page.locator('#facilityStoryGuide');
  await expect(guide).toHaveAttribute('open', '');
  await expect(guide).toContainText('Устройств в сессии: 0');
  await guide.getByRole('button', { name: 'СОЗДАТЬ УСТРОЙСТВО', exact: true }).click();
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
  await device.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Дверь в убежище');
  await device.getByLabel('ТИП', { exact: true }).selectOption('door');
  for (const [id, name] of [['closed', 'Заперта'], ['open', 'Открыта']]) {
    await device.getByRole('button', { name: 'ДОБАВИТЬ СОСТОЯНИЕ', exact: true }).click();
    const row = device.locator('.facility-state-row').last();
    await row.locator('.facility-state-id').fill(id);
    await row.locator('.facility-state-name').fill(name);
  }
  await device.getByLabel('НАЧАЛЬНОЕ СОСТОЯНИЕ', { exact: true }).selectOption('closed');
  await device.getByRole('button', { name: DONE, exact: true }).click();
  await expect(guide).toContainText('Настраиваем «Дверь в убежище»');
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await expectAutosaved(page, request, before.saveCalls + 1);
  await guide.getByRole('button', { name: 'СЛЕДУЮЩИЙ ШАГ', exact: true }).click();
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await expect(guide).toContainText('Сначала добавьте переход');
  await guide.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД В УСТРОЙСТВО', exact: true }).click();
  await device.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД', exact: true }).click();
  const transition = device.locator('.facility-transition-row').last();
  await transition.locator('.facility-transition-id').fill('open');
  await transition.locator('.facility-transition-name').fill('Открыть дверь');
  await transition.locator('.facility-transition-source').selectOption('closed');
  await transition.locator('.facility-transition-destination').selectOption('open');
  await device.getByRole('button', { name: DONE, exact: true }).click();
  await expectAutosaved(page, request, before.saveCalls + 2);
  await guide.getByRole('button', { name: 'СВЯЗАТЬ С КОМАНДОЙ', exact: true }).click();
  const binding = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА', exact: true });
  await expect(binding.getByLabel('ТИП ПРИВЯЗКИ', { exact: true })).toHaveValue('command-action');
  await expect(binding.locator('.facility-action-device option:checked')).toHaveText('Дверь в убежище');
  await binding.locator('.facility-action-transition').selectOption('open');
  await binding.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ', { exact: true }).fill('Дверь открыта');
  await binding.getByLabel('ЗАПРОС ПОДТВЕРЖДЕНИЯ', { exact: true }).fill('Открыть дверь в убежище?');
  await binding.getByRole('button', { name: DONE, exact: true }).click();
  await expect(guide).toContainText('команд: 1');
  await expectAutosaved(page, request, before.saveCalls + 3);
  await guide.getByRole('button', { name: '3. Результат', exact: true }).click();
  await guide.getByRole('button', { name: 'НАСТРОИТЬ РЕЗУЛЬТАТ НА ТЕРМИНАЛЕ', exact: true }).click();
  await expect(binding.getByLabel('ТИП ПРИВЯЗКИ', { exact: true })).toHaveValue('entry-content');
  await binding.getByLabel('БЛОК ЗАПИСИ', { exact: true }).selectOption({ index: 1 });
  await binding.getByLabel('СОСТОЯНИЕ', { exact: true }).selectOption('open');
  await binding.getByLabel('ТЕКСТ', { exact: true }).fill('Путь свободен');
  await binding.getByRole('button', { name: DONE, exact: true }).click();
  await expect(page.getByRole('list', { name: 'Что делают игроки', exact: true })).toContainText('Открыть дверь');
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
  const preview = page.getByRole('dialog', { name: 'ПРЕДПРОСМОТР ОБЪЕКТА', exact: true });
  await expect(preview.getByLabel('СОСТОЯНИЕ', { exact: true })).toHaveValue('open');
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
  const guide = page.locator('#facilityStoryGuide');
  await guide.getByRole('button', { name: '4. Осложнения', exact: true }).click();
  await guide.getByRole('button', { name: 'ДОБАВИТЬ НЕИСПРАВНОСТЬ', exact: true }).click();
  const condition = page.getByRole('dialog', { name: 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', exact: true });
  await expect(condition.getByLabel('УСТРОЙСТВО', { exact: true })).toHaveValue(FACILITY_IDS.devices.power);
  await condition.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await guide.getByRole('button', { name: 'СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await program.getByLabel('ИДЕНТИФИКАТОР', { exact: true }).fill('program-story');
  await program.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Восстановить путь');
  await program.getByLabel('ПЕРЕХОД', { exact: true }).selectOption('restore');
  await program.getByRole('button', { name: DONE, exact: true }).click();
  await expectAutosaved(page, request, before.saveCalls + 1);
  await guide.getByRole('button', { name: '2. Действие', exact: true }).click();
  await guide.getByRole('button', { name: 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ', exact: true }).click();
  const binding = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА', exact: true });
  await binding.getByLabel('ЧТО ЗАПУСТИТЬ', { exact: true }).selectOption('program-story');
  await expect(binding.locator('#facilityBindingActionRequests')).toBeHidden();
  await binding.getByRole('button', { name: DONE, exact: true }).click();
  await expectAutosaved(page, request, before.saveCalls + 2);
  await expect(page.getByRole('list', { name: 'Что делают игроки', exact: true })).toContainText('Восстановить путь');
  await guide.getByRole('button', { name: '2. Действие', exact: true }).click();
  await guide.getByRole('button', { name: 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ', exact: true }).click();
  await expect(binding.getByLabel('ЧТО ЗАПУСТИТЬ', { exact: true })).toHaveValue('program-story');
  await binding.getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
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
  await guide.getByRole('button', { name: '3. Результат', exact: true }).focus();
  await page.keyboard.press('Enter');
  await expect(guide.getByRole('button', { name: '3. Результат', exact: true })).toHaveAttribute('aria-current', 'step');
  await guide.locator(':scope > summary').focus();
  await page.keyboard.press('Enter');
  await expect(guide).not.toHaveAttribute('open', '');
  await expect(page.getByRole('button', { name: 'РЕДАКТИРОВАТЬ ГРАФ', exact: true })).toBeVisible();
  await page.keyboard.press('Enter');
  await expect(guide).toHaveAttribute('open', '');
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
  const guide = page.locator('#facilityStoryGuide');
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
    const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
    await device.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Дверь нового маршрута');
    await device.getByLabel('ТИП', { exact: true }).selectOption('door');
    await device.getByLabel('ИДЕНТИФИКАТОР', { exact: true }).fill('door-checkpoint');
    for (const [id, name] of [['closed', 'Заперта'], ['open', 'Открыта']]) {
      await device.getByRole('button', { name: 'ДОБАВИТЬ СОСТОЯНИЕ', exact: true }).click();
      const row = device.locator('.facility-state-row').last();
      await row.locator('.facility-state-id').fill(id);
      await row.locator('.facility-state-name').fill(name);
    }
    await device.getByLabel('НАЧАЛЬНОЕ СОСТОЯНИЕ', { exact: true }).selectOption('closed');
    await device.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД', exact: true }).click();
    const transition = device.locator('.facility-transition-row').last();
    await transition.locator('.facility-transition-id').fill('open');
    await transition.locator('.facility-transition-name').fill('Открыть дверь');
    await transition.locator('.facility-transition-source').selectOption('closed');
    await transition.locator('.facility-transition-destination').selectOption('open');
    await device.getByRole('button', { name: DONE, exact: true }).click();
    await expect(device).not.toBeVisible();
    await expectAutosaved(page, request, baseline.saveCalls + 1);
    const checkpoint = await facilityState(request);
    expect(checkpoint.facility.devices[0]).toMatchObject({ id: 'door-checkpoint', transitions: [{ id: 'open' }] });

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
    const binding = page.getByRole('dialog', { name: 'ПРИВЯЗКА ОБЪЕКТА', exact: true });
    if (kind === 'command') {
      await binding.locator('.facility-action-transition').selectOption('open');
      await binding.getByLabel('НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ', { exact: true }).fill('Дверь открыта');
      await binding.getByLabel('ЗАПРОС ПОДТВЕРЖДЕНИЯ', { exact: true }).fill('Открыть дверь?');
    } else {
      await binding.getByLabel('БЛОК ЗАПИСИ', { exact: true }).selectOption({ index: 1 });
      await binding.getByLabel('СОСТОЯНИЕ', { exact: true }).selectOption('open');
      await binding.getByLabel('ТЕКСТ', { exact: true }).fill('Путь свободен');
    }
    await binding.getByRole('button', { name: DONE, exact: true }).click();
    await expectAutosaved(page, request, checkpoint.saveCalls + 3);
    const saved = await facilityState(request);
    expect(saved.brokenReferenceCount).toBe(0);
    expect(saved.facility.devices).toEqual(checkpoint.facility.devices);
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
  const guide = page.locator('#facilityStoryGuide');
  await guide.getByRole('button', { name: '4. Осложнения', exact: true }).click();
  await expect(guide).toContainText('Если нужна программа восстановления, сначала добавьте переход устройства');
  await expect(guide).toContainText('Для простой сцены этот шаг можно пропустить');
  await expect(guide.getByRole('button', { name: 'СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true })).toHaveCount(0);
  const addTransition = guide.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД ДЛЯ ПРОГРАММЫ', exact: true });
  await addTransition.focus();
  await page.screenshot({ path: 'test-results/facility-story-recovery-prerequisite-narrow.png' });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.keyboard.press('Enter');
  const device = page.getByRole('dialog', { name: 'УСТРОЙСТВО ОБЪЕКТА', exact: true });
  await expect(device.getByLabel('НАЗВАНИЕ', { exact: true })).toHaveValue('Дверь без переходов');
  await device.getByRole('button', { name: 'ДОБАВИТЬ ПЕРЕХОД', exact: true }).click();
  const transition = device.locator('.facility-transition-row').last();
  await transition.locator('.facility-transition-id').fill('open');
  await transition.locator('.facility-transition-name').fill('Открыть дверь');
  await transition.locator('.facility-transition-source').selectOption('closed');
  await transition.locator('.facility-transition-destination').selectOption('open');
  await device.getByRole('button', { name: DONE, exact: true }).click();
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await expect(guide).toContainText('переходов: 1');
  await expectAutosaved(page, request, baseline.saveCalls + 1);
  await expect(guide).not.toContainText('сначала добавьте переход устройства');
  await guide.getByRole('button', { name: 'СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', exact: true }).click();
  const program = page.getByRole('dialog', { name: 'ПРОГРАММА ВОССТАНОВЛЕНИЯ', exact: true });
  await expect(program.getByLabel('УСТРОЙСТВО', { exact: true })).toHaveValue('door-no-transitions');
  await program.getByLabel('НАЗВАНИЕ', { exact: true }).fill('Восстановить проход');
  await program.getByLabel('ПЕРЕХОД', { exact: true }).selectOption('open');
  await program.getByRole('button', { name: DONE, exact: true }).click();
  await expect(page.locator('#facilityStoryTitle')).toBeFocused();
  await expectAutosaved(page, request, baseline.saveCalls + 2);
  expect((await facilityState(request)).facility.recoveryPrograms).toContainEqual(expect.objectContaining({
    name: 'Восстановить проход', transitions: [{ deviceId: 'door-no-transitions', transitionId: 'open' }],
  }));
});
