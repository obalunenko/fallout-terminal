import { readFile } from 'node:fs/promises';

import { expect, test } from '@playwright/test';

import {
  feedEntries, forceState, openPlayerTable, playCard, PLAYER_FIXTURE, selectDevice, setConditionActive,
} from './helpers/facility-play.mjs';
import { FIXTURE, demoSession, facilityState, fixtureSession, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

// User Story 1: run a device's transitions from its card
// (specs/040-facility-play-controls/spec.md, contracts/overseer-ui.md).

const VERDICTS_URL = new URL('../../internal/session/testdata/demo-transition-verdicts.json', import.meta.url);
const STALE = 'СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС';
const DOOR = 'vault-door-main';
const GRID = 'power-grid-main';

// openDemoAt seeds a demo copy, moves the listed devices behind the
// Overseer's back, then opens «ОБЪЕКТЫ» on that state.
async function openDemoAt(page, request, states = {}) {
  await seedDemo(request);
  for (const [deviceId, stateId] of Object.entries(states)) {
    const device = (await facilityState(request)).facility.devices.find(value => value.id === deviceId);
    if (device.currentStateId !== stateId) await forceState(request, deviceId, stateId);
  }
  await openOverseer(page);
}

function pageErrors(page) {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  return errors;
}

test('an available transition runs on one press and reaches players at once (AS1, SC-001, FR-003, FR-005)', async ({ browser, request }) => {
  const table = await openPlayerTable(browser, request);
  try {
    const { overseer, player } = table;
    const errors = pageErrors(overseer);
    await expect(player.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' })).toBeVisible();

    await selectDevice(overseer, 'device-security-door');
    const card = playCard(overseer);
    await expect(card.run('open')).toHaveText('Open → Open');
    await expect(card.transition('open').locator('.facility-play-precondition')).toHaveText('БЕЗ ПРЕДУСЛОВИЙ');
    await card.run('open').click();

    await expect(card.status).toHaveText('ПЕРЕХОД ВЫПОЛНЕН');
    await expect(card.status).toHaveAttribute('data-state', 'ok');
    await expect(overseer.locator('dialog[open]')).toHaveCount(0);
    await expect(overseer.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Open');
    // The card now lists the transitions that leave the new state: none here.
    await expect(overseer.locator('#facilityPlayTransitionsEmpty')).toBeVisible();
    // Players see the open door's name variant and availability without reconnecting.
    await expect(player.locator('.term-row', { hasText: 'SECURITY DOOR OPEN' })).toBeVisible();
    await expect(player.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' })).toHaveCount(0);
    // The card changes only the device: the command stays unexecuted.
    const session = await (await request.get(`${PLAYER_FIXTURE}/session`)).json();
    const terminal = session.terminals.find(value => value.id === 'terminal-facility-security');
    expect(terminal.commandStates?.['command-open-security-door']).toBeUndefined();
    expect(errors).toEqual([]);
  } finally {
    await table.close();
  }
});

test('an unmet precondition makes the transition unavailable and names it (AS2, FR-002)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openDemoAt(page, request, { [GRID]: 'offline', [DOOR]: 'sealed' });
  await selectDevice(page, DOOR);
  const card = playCard(page);
  const precondition = card.transition('open').locator('.facility-play-precondition');
  await expect(card.transition('open')).toHaveAttribute('data-available', 'false');
  await expect(precondition).toHaveAttribute('data-device-id', GRID);
  await expect(precondition).toHaveAttribute('data-state-id', 'online');
  await expect(precondition).toHaveAttribute('data-met', 'false');
  await expect(precondition).toHaveText('Резервная сеть Убежища 76 = В работе · НЕ ВЫПОЛНЕНО');
  await expect(card.run('open')).toHaveAttribute('aria-disabled', 'true');
  const reasonID = await card.run('open').getAttribute('aria-describedby');
  await expect(page.locator(`#${reasonID}`)).toHaveText('НЕ ВЫПОЛНЕНО: Резервная сеть Убежища 76 = В работе');

  const before = (await facilityState(request)).facility.revision;
  await card.run('open').dispatchEvent('click');
  await expect(card.status).toHaveText('');
  expect((await facilityState(request)).facility.revision).toBe(before);
  expect(errors).toEqual([]);
});

test('a transition applies its fault effect together with the state (AS3, FR-003)', async ({ page, request }) => {
  await openDemoAt(page, request, { [GRID]: 'online' });
  await setConditionActive(request, 'grid-unpowered', false);
  await openOverseer(page);
  await selectDevice(page, GRID);
  const card = playCard(page);
  await card.run('shutdown').click();
  await expect(card.status).toHaveText('ПЕРЕХОД ВЫПОЛНЕН');
  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Нет питания');
  await expect(card.fault('grid-unpowered')).toHaveAttribute('data-active', 'true');
  const facility = (await fixtureSession(request)).facility;
  expect(facility.devices.find(value => value.id === GRID).currentStateId).toBe('offline');
  expect(facility.conditions.find(value => value.id === 'grid-unpowered').currentActive).toBe(true);
});

test('a transition based on an old state is refused and the card shows the state that won (AS4, FR-004)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openDemoAt(page, request, { [GRID]: 'online', [DOOR]: 'sealed' });
  await selectDevice(page, DOOR);
  const card = playCard(page);
  await expect(card.transition('open')).toHaveAttribute('data-available', 'true');

  // A player command opens the door after the Overseer opened the card.
  await forceState(request, DOOR, 'open');
  await card.run('open').click();

  await expect(card.status).toHaveText(STALE);
  await expect(card.status).toHaveAttribute('data-state', 'stale');
  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Открыта');
  await expect(card.transition('seal')).toBeVisible();
  await expect(card.transition('open')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('a state with no outgoing transition keeps forced change, faults, and reset (edge case)', async ({ page, request }) => {
  await openDemoAt(page, request, { 'reactor-main': 'critical-overheat' });
  await selectDevice(page, 'reactor-main');
  const card = playCard(page);
  await expect(page.locator('#facilityPlayTransitionsEmpty')).toHaveText('ИЗ ЭТОГО СОСТОЯНИЯ НЕТ ПЕРЕХОДОВ');
  await expect(card.transitions).toHaveCount(0);
  await expect(card.force).not.toHaveAttribute('aria-disabled', 'true');
  await expect(card.reset).not.toHaveAttribute('aria-disabled', 'true');
});

test('a player command the Overseer already performed is refused and fed (edge case)', async ({ page, request }) => {
  await openDemoAt(page, request, { [GRID]: 'online', [DOOR]: 'sealed', 'evacuation-alarm': 'quiet' });
  // No fault may block the command, so only the device state decides the refusal.
  for (const fault of ['door-offline', 'grid-unpowered']) await setConditionActive(request, fault, false);
  await openOverseer(page);
  await selectDevice(page, DOOR);
  const card = playCard(page);
  await card.run('open').click();
  await expect(card.status).toHaveText('ПЕРЕХОД ВЫПОЛНЕН');

  const response = await request.post(`${FIXTURE}/player-command`, {
    data: { terminalId: 't_demo1', commandId: 'n_cmd_open_evacuation', decision: 'approve' },
  });
  expect(response.ok()).toBe(true);
  await expect.poll(async () => (await feedEntries(page))[0]?.reason).toBe('transition-unavailable');
  const [refusal] = await feedEntries(page);
  expect(refusal.kind).toBe('player-refusal');
  expect(refusal.text).toContain('«Открыть выход и подать сигнал» · запрос игрока отклонён: переход больше недоступен');
});

test('the card verdict matches the server for every demo transition (SC-002)', async ({ page, request }) => {
  test.setTimeout(120_000);
  const verdicts = JSON.parse(await readFile(VERDICTS_URL, 'utf8'));
  const demo = await demoSession();
  expect(verdicts.length).toBeGreaterThan(0);
  for (const row of verdicts) {
    const transition = demo.facility.devices
      .find(device => device.id === row.deviceId).transitions.find(value => value.id === row.transitionId);
    // Only the transition's own device and its precondition devices decide the verdict.
    const devices = new Set([row.deviceId, ...(transition.preconditions ?? []).map(value => value.deviceId)]);
    const states = Object.fromEntries([...devices].map(id => [id, row.states[id]]));
    await openDemoAt(page, request, states);
    await selectDevice(page, row.deviceId);
    await expect(playCard(page).transition(row.transitionId), `${row.deviceId}/${row.transitionId}/${row.case}`)
      .toHaveAttribute('data-available', String(row.available));
  }
});
