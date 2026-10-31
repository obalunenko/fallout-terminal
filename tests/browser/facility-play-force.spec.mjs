import { expect, test } from '@playwright/test';

import {
  feedEntries, forceState, openPlayerTable, playCard, PLAYER_FIXTURE, selectDevice,
} from './helpers/facility-play.mjs';
import { demoSession, facilityState, fixtureSession, openOverseer, seedDemo, seedSession } from './helpers/overseer-forms.mjs';

// User Story 3: force a device into any state
// (specs/040-facility-play-controls/spec.md, contracts/overseer-ui.md).

const REACTOR = 'reactor-main';
const STALE = 'СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС';

async function openReactorAtStandby(page, request) {
  await seedDemo(request);
  const reactor = (await facilityState(request)).facility.devices.find(device => device.id === REACTOR);
  if (reactor.currentStateId !== 'standby') await forceState(request, REACTOR, 'standby');
  await openOverseer(page);
  await selectDevice(page, REACTOR);
}

const dialog = page => page.locator('#facilityForceStateDialog');

test('the dialog offers every other state, including one no transition reaches (AS1)', async ({ page, request }) => {
  await openReactorAtStandby(page, request);
  await playCard(page).force.click();
  await expect(dialog(page)).toBeVisible();
  await expect(page.locator('#facilityForceStateDialogTitle')).toHaveText('ПРИНУДИТЕЛЬНАЯ СМЕНА СОСТОЯНИЯ');
  const options = await page.locator('#facilityForceStateTarget option').allTextContents();
  expect(options).toEqual(['Аварийная остановка', 'Критический перегрев']);
  await expect(page.locator('#facilityForceStateWarning'))
    .toHaveText('ПЕРЕХОДЫ И ПРЕДУСЛОВИЯ НЕ ПРОВЕРЯЮТСЯ. ЭФФЕКТЫ НЕИСПРАВНОСТЕЙ НЕ ПРИМЕНЯЮТСЯ.');
});

test('confirming moves the device without effects and records a forced change (AS2, FR-008)', async ({ page, request }) => {
  await openReactorAtStandby(page, request);
  const conditionsBefore = (await fixtureSession(request)).facility.conditions;
  const card = playCard(page);
  await card.force.click();
  await page.locator('#facilityForceStateTarget').selectOption('critical-overheat');
  await expect(page.locator('#facilityForceStateSummary'))
    .toHaveText('Реактор Убежища 76: Дежурный режим → Критический перегрев');
  await page.locator('#btnConfirmFacilityForceState').click();

  await expect(dialog(page)).toBeHidden();
  await expect(card.status).toHaveText('СОСТОЯНИЕ ИЗМЕНЕНО ПРИНУДИТЕЛЬНО');
  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Критический перегрев');
  const facility = (await fixtureSession(request)).facility;
  expect(facility.devices.find(device => device.id === REACTOR).currentStateId).toBe('critical-overheat');
  expect(facility.conditions).toEqual(conditionsBefore);
  await expect.poll(async () => (await feedEntries(page))[0]?.kind).toBe('forced');
  const [entry] = await feedEntries(page);
  expect(entry.author).toBe('overseer');
  expect(entry.text).toMatch(/^\d\d:\d\d · принудительно: Дежурный режим → Критический перегрев · Смотритель$/);
});

test('cancel and Escape change nothing and record nothing (AS3)', async ({ page, request }) => {
  await openReactorAtStandby(page, request);
  const before = (await facilityState(request)).facility.revision;
  const card = playCard(page);
  const feedBefore = await feedEntries(page);
  await card.force.click();
  await page.locator('#facilityForceStateTarget').selectOption('critical-overheat');
  await page.locator('#btnCancelFacilityForceState').click();
  await expect(dialog(page)).toBeHidden();
  await expect(card.force).toBeFocused();
  await card.force.click();
  await page.keyboard.press('Escape');
  await expect(dialog(page)).toBeHidden();
  expect((await facilityState(request)).facility.revision).toBe(before);
  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Дежурный режим');
  expect(await feedEntries(page)).toEqual(feedBefore);
});

test('a scene that changed while the dialog was open is refused as stale', async ({ page, request }) => {
  await openReactorAtStandby(page, request);
  const card = playCard(page);
  await card.force.click();
  await page.locator('#facilityForceStateTarget').selectOption('critical-overheat');
  await forceState(request, REACTOR, 'scrammed');
  await page.locator('#btnConfirmFacilityForceState').click();
  await expect(dialog(page)).toBeHidden();
  await expect(card.status).toHaveText(STALE);
  await expect(page.locator('#facilityDeviceHeaderCurrent')).toHaveText('СЕЙЧАС: Аварийная остановка');
});

test('a one-state device cannot be forced and says why (edge case)', async ({ page, request }) => {
  const demo = await demoSession();
  demo.facility.devices.push({
    id: 'beacon', name: 'Маяк', kind: 'custom', customKind: 'beacon',
    initialStateId: 'lit', currentStateId: 'lit', states: [{ id: 'lit', name: 'Горит' }], transitions: [],
  });
  await seedDemo(request);
  await seedSession(request, demo);
  await openOverseer(page);
  await selectDevice(page, 'beacon');
  const force = playCard(page).force;
  await expect(force).toHaveAttribute('aria-disabled', 'true');
  await expect(page.locator('#btnForceFacilityDeviceStateReason')).toHaveText('У УСТРОЙСТВА ОДНО СОСТОЯНИЕ');
  await force.dispatchEvent('click');
  await expect(dialog(page)).toBeHidden();
});

test('a forced change reaches players and runs no transition effect', async ({ browser, request }) => {
  const table = await openPlayerTable(browser, request);
  try {
    const { overseer, player } = table;
    await selectDevice(overseer, 'device-security-door');
    const card = playCard(overseer);
    await expect(card.fault('condition-security-authorization')).toHaveAttribute('data-active', 'true');
    await card.force.click();
    await overseer.locator('#facilityForceStateTarget').selectOption('open');
    await overseer.locator('#btnConfirmFacilityForceState').click();
    await expect(card.status).toHaveText('СОСТОЯНИЕ ИЗМЕНЕНО ПРИНУДИТЕЛЬНО');
    await expect(player.locator('.term-row', { hasText: 'SECURITY DOOR OPEN' })).toBeVisible();
    // The authored «Open» transition would clear the fault; a forced change does not.
    await expect(card.fault('condition-security-authorization')).toHaveAttribute('data-active', 'true');
    const session = await (await request.get(`${PLAYER_FIXTURE}/session`)).json();
    expect(session.facility.conditions.find(value => value.id === 'condition-security-authorization').currentActive).toBe(true);
  } finally {
    await table.close();
  }
});
