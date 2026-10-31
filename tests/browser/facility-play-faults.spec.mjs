import { expect, test } from '@playwright/test';

import { feedEntries, forceState, playCard, selectDevice, setConditionActive } from './helpers/facility-play.mjs';
import { FIXTURE, demoSession, facilityState, fixtureSession, openOverseer, seedDemo, seedSession } from './helpers/overseer-forms.mjs';

// User Story 4: turn a device's faults on and off
// (specs/040-facility-play-controls/spec.md, contracts/overseer-ui.md).

const DOOR = 'vault-door-main';
const FAULT = 'door-offline';

// openDoor opens the door card with the door sealed, the grid online, and no
// fault active, unless session overrides the demo.
async function openDoor(page, request, session = null) {
  await seedDemo(request);
  if (session) await seedSession(request, session);
  const devices = (await facilityState(request)).facility.devices;
  for (const [id, stateID] of [['power-grid-main', 'online'], [DOOR, 'sealed'], ['evacuation-alarm', 'quiet']]) {
    if (devices.find(device => device.id === id).currentStateId !== stateID) await forceState(request, id, stateID);
  }
  for (const fault of [FAULT, 'grid-unpowered']) await setConditionActive(request, fault, false);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  return playCard(page);
}

const savedFault = async request => (await fixtureSession(request)).facility.conditions.find(value => value.id === FAULT);
const doorRow = page => page.locator(`#facilityDeviceList [data-facility-id="${DOOR}"]`);

test('turning a fault on applies its effect and is counted (AS1, FR-009)', async ({ page, request }) => {
  const card = await openDoor(page, request);
  const toggle = card.faultSwitch(FAULT);
  await expect(toggle).toHaveAttribute('role', 'switch');
  await expect(toggle).toHaveAttribute('aria-checked', 'false');
  await expect(toggle).toHaveAccessibleName('Контроллер гермодвери не отвечает');
  await expect(card.fault(FAULT).locator('.facility-play-fault-current')).toHaveText('НЕАКТИВНА');

  await toggle.click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await expect(toggle).toHaveAttribute('aria-checked', 'true');
  await expect(card.fault(FAULT).locator('.facility-play-fault-current')).toHaveText('АКТИВНА');
  await expect(page.locator('#facilityDeviceHeaderFaults')).toHaveText('АКТИВНЫХ НЕИСПРАВНОСТЕЙ: 1');
  await expect(doorRow(page).locator('.facility-device-row-state')).toHaveText('Заперта · ● 1');
  await expect(doorRow(page)).toHaveAttribute('data-active-faults', '1');
  expect(await savedFault(request)).toMatchObject({ currentActive: true, initialActive: false });

  // Its capability block reaches players: the evacuation command is refused.
  const response = await request.post(`${FIXTURE}/player-command`, {
    data: { terminalId: 't_demo1', commandId: 'n_cmd_open_evacuation', decision: 'approve' },
  });
  expect(response.ok()).toBe(true);
  await expect.poll(async () => (await feedEntries(page))[0]?.reason).toBe('fault-blocked');
});

test('turning a fault off keeps its starting position (AS2, FR-010)', async ({ page, request }) => {
  const card = await openDoor(page, request);
  await card.faultSwitch(FAULT).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await card.faultSwitch(FAULT).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
  await expect(card.faultSwitch(FAULT)).toHaveAttribute('aria-checked', 'false');
  await expect(card.fault(FAULT).locator('.facility-play-fault-initial')).toHaveText('В НАЧАЛЕ ИГРЫ: ИСПРАВНО');
  await expect(doorRow(page).locator('.facility-device-row-state')).toHaveText('Заперта');
  expect(await savedFault(request)).toMatchObject({ currentActive: false, initialActive: false });
  const kinds = (await feedEntries(page)).map(entry => entry.kind);
  expect(kinds.slice(0, 2)).toEqual(['fault', 'fault']);
});

test('a device without faults says so and offers no switch (AS3)', async ({ page, request }) => {
  await openDoor(page, request);
  await selectDevice(page, 'evacuation-alarm');
  const card = playCard(page);
  await expect(card.faults).toHaveCount(0);
  await expect(page.locator('#facilityPlayFaultsEmpty')).toHaveText('У УСТРОЙСТВА НЕТ НЕИСПРАВНОСТЕЙ');
});

test('operator recovery does not limit the switch (FR-009)', async ({ page, request }) => {
  const demo = await demoSession();
  const fault = demo.facility.conditions.find(value => value.id === FAULT);
  fault.recovery = fault.recovery.filter(reference => reference.privateOverseerAction !== true);
  const card = await openDoor(page, request, demo);
  await card.faultSwitch(FAULT).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  expect((await savedFault(request)).recovery.some(reference => reference.privateOverseerAction)).toBe(false);
  expect((await savedFault(request)).currentActive).toBe(true);
});

test('a reset that only restores faults succeeds (FR-011)', async ({ page, request }) => {
  const card = await openDoor(page, request);
  await card.faultSwitch(FAULT).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await expect(card.reset).toHaveText('СБРОСИТЬ К НАЧАЛЬНОМУ…');
  await card.reset.click();
  await page.locator('#btnConfirmFacilityDeviceReset').click();
  await expect(card.status).toHaveText('УСТРОЙСТВО СБРОШЕНО');
  await expect(card.status).toHaveAttribute('data-state', 'ok');
  await expect(card.faultSwitch(FAULT)).toHaveAttribute('aria-checked', 'false');
  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Заперта');
  await expect.poll(async () => (await feedEntries(page))[0]?.kind).toBe('reset');
  expect((await feedEntries(page))[0].text).toContain('сброс к начальному: неисправности восстановлены · Смотритель');
});

test('the device row follows a fault switched on another device card', async ({ page, request }) => {
  await openDoor(page, request);
  await setConditionActive(request, 'air-spore-bloom', true);
  await openOverseer(page);
  const ventilation = page.locator('#facilityDeviceList [data-facility-id="ventilation-main"]');
  await expect(ventilation.locator('.facility-device-row-state')).toHaveText(/^\S+ · ● 1$/);
  await selectDevice(page, 'ventilation-main');
  await playCard(page).faultSwitch('air-spore-bloom').click();
  await expect(ventilation.locator('.facility-device-row-state')).not.toContainText('●');
  await expect(ventilation).toHaveAttribute('data-active-faults', '0');
});
