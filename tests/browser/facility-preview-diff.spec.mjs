import { expect, test } from '@playwright/test';

import { expectSaved } from './helpers/authoring-bar.mjs';
import {
  demoSession, facilityState, fixtureSession, openOverseer, resetFixture, seedSession,
} from './helpers/overseer-forms.mjs';

// US3: the preview compares what players see now with the previewed state,
// side by side, for the applied configuration (FR-016–FR-020, SC-006).

const WARDEN_TERMINAL = 't_demo1';
const DOOR = 'vault-door-main';
const DOOR_BINDING = 't_demo1/b_vault_door_status/entry-content/0';
const KIND_LABELS = {
  'name-changed': 'ПЕРЕИМЕНУЕТСЯ',
  'text-changed': 'ТЕКСТ ИЗМЕНИТСЯ',
  appears: 'ПОЯВИТСЯ',
  disappears: 'ИСЧЕЗНЕТ',
  'becomes-available': 'СТАНЕТ ДОСТУПНО',
  'becomes-unavailable': 'СТАНЕТ НЕДОСТУПНО',
};

// seedDemoWith seeds the demo with its current live values. The fixture
// starts a new device at its initial state and keeps a known device's
// current state, so it seeds the wanted current values as starting values
// first, then the authored starting values.
async function seedDemoWith(request, mutate = () => {}) {
  await resetFixture(request);
  const session = await demoSession();
  mutate(session);
  const live = structuredClone(session);
  for (const device of live.facility.devices) device.initialStateId = device.currentStateId;
  for (const condition of live.facility.conditions) condition.initialActive = condition.currentActive;
  await seedSession(request, live);
  await seedSession(request, session);
  return session;
}

function device(session, id) {
  return session.facility.devices.find(value => value.id === id);
}

function nodeByID(node, id) {
  if (node.id === id) return node;
  for (const child of node.children || []) {
    const found = nodeByID(child, id);
    if (found) return found;
  }
  return null;
}

async function openDevicePreview(page, deviceID) {
  await page.locator(`[data-facility-kind="device"][data-facility-id="${deviceID}"]`).click();
  await page.locator('#btnPreviewFacilityDevice').click();
  await expect(page.locator('#facilityPreviewDialog')).toBeVisible();
}

async function waitForPreview(page) {
  await expect(page.locator('#facilityPreviewStatus')).toHaveText('ПРЕДПРОСМОТР ГОТОВ · МИР ДЛЯ ИГРОКОВ НЕ ИЗМЕНЁН');
}

async function summaryItems(page) {
  return page.locator('#facilityPreviewSummary li').evaluateAll(items => items.map(item => ({
    kind: item.dataset.kind, text: item.textContent.trim(),
  })));
}

// expectNamedDifferences checks that the summary counts its items and that
// every item marks a tree node with the same kind and word (FR-018).
async function expectNamedDifferences(page) {
  const items = await summaryItems(page);
  const summary = page.locator('#facilityPreviewSummary');
  if (!items.length) {
    await expect(summary).toHaveText('ИГРОКИ НЕ УВИДЯТ ИЗМЕНЕНИЙ');
    await expect(page.locator('#facilityPreviewDialog .facility-preview-node[data-kind]')).toHaveCount(0);
    return items;
  }
  await expect(summary.locator('div').first()).toHaveText(`ИЗМЕНЕНИЙ: ${items.length}`);
  for (const item of items) {
    expect(item.text.startsWith(`${KIND_LABELS[item.kind]}: `), item.text).toBe(true);
    const marked = page.locator(`#facilityPreviewDialog .facility-preview-node[data-kind~="${item.kind}"]`);
    expect(await marked.count(), item.text).toBeGreaterThan(0);
    await expect(marked.first().locator(':scope > .facility-preview-node-status', {
      hasText: new RegExp(`^${KIND_LABELS[item.kind]}$`),
    })).toHaveCount(1);
  }
  return items;
}

test('the door preview preselects «Открыта» and names every difference side by side (FR-016–FR-018)', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page);
  await openDevicePreview(page, DOOR);

  await expect(page.locator('#facilityPreviewContext'))
    .toHaveText('ПРЕДПРОСМОТР ПОКАЗЫВАЕТ ПРИМЕНЁННЫЕ ПРАВКИ И НЕ МЕНЯЕТ МИР ДЛЯ ИГРОКОВ.');
  await expect(page.locator('#facilityPreviewTerminal')).toHaveValue(WARDEN_TERMINAL);
  await expect(page.locator('#facilityPreviewState')).toHaveValue('open');
  await waitForPreview(page);

  await expect(page.locator('#facilityPreviewCurrentHeading')).toHaveText('СЕЙЧАС: Гермодверь Убежища 76: Заперта');
  await expect(page.locator('#facilityPreviewHypotheticalHeading')).toHaveText('ЕСЛИ: Гермодверь Убежища 76: Открыта');
  const current = await page.locator('#facilityPreviewCurrent').boundingBox();
  const hypothetical = await page.locator('#facilityPreviewHypothetical').boundingBox();
  expect(Math.abs(current.y - hypothetical.y)).toBeLessThan(2);
  expect(hypothetical.x).toBeGreaterThan(current.x + current.width - 1);

  expect(await expectNamedDifferences(page)).toEqual([
    { kind: 'text-changed', text: 'ТЕКСТ ИЗМЕНИТСЯ: СВОДКА ЭВАКУАЦИИ · блок 3' },
    { kind: 'appears', text: 'ПОЯВИТСЯ: НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН' },
    { kind: 'name-changed', text: 'ПЕРЕИМЕНУЕТСЯ: Выход уже открыт' },
  ]);
  // An appearing element is marked only where it exists.
  await expect(page.locator('#facilityPreviewCurrent .facility-preview-node[data-kind="appears"]')).toHaveCount(0);
  await expect(page.locator('#facilityPreviewHypothetical .facility-preview-node[data-kind="appears"]'))
    .toContainText('НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН');
  for (const pane of ['#facilityPreviewCurrent', '#facilityPreviewHypothetical']) {
    await expect(page.locator(`${pane} .facility-preview-node[data-kind="name-changed"]`)).toHaveCount(1);
  }
  await expect(page.locator('#facilityPreviewHypothetical')).toContainText('ГЕРМОДВЕРЬ: ОТКРЫТА // МАРШРУТ СВОБОДЕН');
  await expect(page.locator('#facilityPreviewCurrent')).toContainText('ГЕРМОДВЕРЬ: ЗАПЕРТА');
  expect(device(session, DOOR).currentStateId).toBe('sealed');
});

test('an element with two differences carries both marks in both panes (FR-018)', async ({ page, request }) => {
  await seedDemoWith(request, session => {
    const terminal = session.terminals.find(value => value.id === WARDEN_TERMINAL);
    nodeByID(terminal.root, 'n_cmd_open_evacuation').availableWhen = { deviceId: DOOR, stateId: 'sealed' };
  });
  await openOverseer(page);
  await openDevicePreview(page, DOOR);
  await waitForPreview(page);

  const items = await expectNamedDifferences(page);
  expect(items).toEqual(expect.arrayContaining([
    { kind: 'name-changed', text: 'ПЕРЕИМЕНУЕТСЯ: Выход уже открыт' },
    { kind: 'becomes-unavailable', text: 'СТАНЕТ НЕДОСТУПНО: Выход уже открыт' },
  ]));
  for (const pane of ['#facilityPreviewCurrent', '#facilityPreviewHypothetical']) {
    const command = page.locator(`${pane} .facility-preview-node[data-preview-node-id="n_cmd_open_evacuation"]`);
    await expect(command).toHaveAttribute('data-kind', 'name-changed becomes-unavailable');
    await expect(command.locator(':scope > .facility-preview-node-status'))
      .toHaveText(['ПЕРЕИМЕНУЕТСЯ', 'СТАНЕТ НЕДОСТУПНО']);
  }
});

test('the panes stack on a narrow window', async ({ page, request }) => {
  await seedDemoWith(request);
  await page.setViewportSize({ width: 420, height: 900 });
  await openOverseer(page);
  await openDevicePreview(page, DOOR);
  await waitForPreview(page);
  const current = await page.locator('#facilityPreviewCurrent').boundingBox();
  const hypothetical = await page.locator('#facilityPreviewHypothetical').boundingBox();
  expect(hypothetical.y).toBeGreaterThan(current.y + current.height - 1);
});

test('a fault preview preselects the opposite activity', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page);
  // Faults are listed on the «НЕИСПРАВНОСТИ» tab.
  await page.locator('#facilityTabConditions').click();
  for (const condition of session.facility.conditions.slice(0, 3)) {
    await page.locator(`[data-facility-kind="condition"][data-facility-id="${condition.id}"]`).click();
    await page.locator('#btnPreviewFacilityCondition').click();
    await expect(page.locator('#facilityPreviewDialog')).toBeVisible();
    if (condition.currentActive) await expect(page.locator('#facilityPreviewActive')).not.toBeChecked();
    else await expect(page.locator('#facilityPreviewActive')).toBeChecked();
    await waitForPreview(page);
    await expectNamedDifferences(page);
    await page.locator('#btnCloseFacilityPreview').click();
    await expect(page.locator('#facilityPreviewDialog')).toBeHidden();
  }
});

test('a one-state device has nothing to compare and makes no request', async ({ page, request }) => {
  await seedDemoWith(request, session => {
    const lamp = structuredClone(device(session, 'evacuation-alarm'));
    Object.assign(lamp, {
      id: 'probe-lamp', name: 'Сигнальная лампа', states: [lamp.states[0]], transitions: [],
      initialStateId: lamp.states[0].id, currentStateId: lamp.states[0].id,
    });
    session.facility.devices.push(lamp);
  });
  await openOverseer(page);
  const before = (await facilityState(request)).previewCalls;
  await openDevicePreview(page, 'probe-lamp');
  await expect(page.locator('#facilityPreviewSummary')).toHaveText('НЕЧЕГО СРАВНИВАТЬ: У УСТРОЙСТВА ОДНО СОСТОЯНИЕ');
  await expect(page.locator('#facilityPreviewDialog .facility-preview-node')).toHaveCount(0);
  expect((await facilityState(request)).previewCalls).toBe(before);
});

test('the preview reflects an applied text rule, even while its save is in flight (FR-019)', async ({ page, request }) => {
  await seedDemoWith(request);
  await openOverseer(page);
  // Text rules are listed on the «ЧТО ВИДЯТ ИГРОКИ» tab.
  await page.locator('#facilityTabInformation').click();
  const binding = page.locator(`[data-facility-kind="binding"][data-facility-id="${DOOR_BINDING}"]`);
  const editRule = async text => {
    await binding.getByRole('button', { name: /^Изменить/ }).click();
    await expect(page.locator('#facilityBindingDialog')).toBeVisible();
    await page.locator('#facilityBindingText').fill(text);
    await page.locator('#facilityBindingDialog').getByRole('button', { name: 'ГОТОВО' }).click();
    await expect(page.locator('#facilityBindingDialog')).toBeHidden();
  };

  await editRule('ГЕРМОДВЕРЬ: НАСТЕЖЬ');
  await expectSaved(page);
  await openDevicePreview(page, DOOR);
  await waitForPreview(page);
  await expect(page.locator('#facilityPreviewHypothetical')).toContainText('ГЕРМОДВЕРЬ: НАСТЕЖЬ');
  await page.locator('#btnCloseFacilityPreview').click();

  // A preview opened during a save waits for it, then shows the new text.
  await page.evaluate(() => window.__desktopFixture.holdCalls('SaveAuthoring'));
  await editRule('ГЕРМОДВЕРЬ: ОТКРЫТА НАСТЕЖЬ');
  await expect(page.locator('#saveStatus')).toHaveAttribute('data-state', 'saving');
  const before = (await facilityState(request)).previewCalls;
  await openDevicePreview(page, DOOR);
  await expect(page.locator('#facilityPreviewStatus')).toHaveText('ПОДГОТОВКА ПРЕДПРОСМОТРА…');
  expect((await facilityState(request)).previewCalls).toBe(before);
  await page.evaluate(() => window.__desktopFixture.releaseHeldCalls('SaveAuthoring'));
  await waitForPreview(page);
  await expect(page.locator('#facilityPreviewHypothetical')).toContainText('ГЕРМОДВЕРЬ: ОТКРЫТА НАСТЕЖЬ');
  await expectSaved(page);
});

test('every demo device preview opens on a different state and names its differences (SC-006)', async ({ page, request }) => {
  const session = await seedDemoWith(request);
  await openOverseer(page);
  for (const value of session.facility.devices) {
    await openDevicePreview(page, value.id);
    const preselected = await page.locator('#facilityPreviewState').inputValue();
    expect(preselected, value.id).not.toBe(value.currentStateId);
    for (const terminal of session.terminals) {
      await page.locator('#facilityPreviewTerminal').selectOption(terminal.id);
      await waitForPreview(page);
      await expectNamedDifferences(page);
    }
    await page.locator('#btnCloseFacilityPreview').click();
    await expect(page.locator('#facilityPreviewDialog')).toBeHidden();
  }
});

test('closing a preview leaves live state and revisions unchanged (FR-020)', async ({ page, request }) => {
  await seedDemoWith(request);
  await openOverseer(page);
  const before = await facilityState(request);
  const sessionBefore = await fixtureSession(request);
  await openDevicePreview(page, DOOR);
  await waitForPreview(page);
  await page.locator('#facilityPreviewState').selectOption('open');
  await waitForPreview(page);
  await page.locator('#btnCloseFacilityPreview').click();
  await expect(page.locator('#facilityPreviewDialog')).toBeHidden();

  const after = await facilityState(request);
  expect(after.sessionRevision).toBe(before.sessionRevision);
  expect(after.authoringRevision).toBe(before.authoringRevision);
  expect(after.facility).toEqual(before.facility);
  expect(after.saveCalls).toBe(before.saveCalls);
  expect(await fixtureSession(request)).toEqual(sessionBefore);
  await expect(page.locator(`[data-facility-kind="device"][data-facility-id="${DOOR}"]`)).toContainText('Заперта');
});
