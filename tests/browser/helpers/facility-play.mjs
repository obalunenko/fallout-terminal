import { expect } from '@playwright/test';

import { FIXTURE, facilityState, openOverseer, seedDemo } from './overseer-forms.mjs';

// Journey helpers for the facility play card
// (specs/040-facility-play-controls/contracts/overseer-ui.md).

export const PLAYER_FIXTURE = '/__fixture/facility-player-state';

// Opens «ОБЪЕКТЫ» on a fresh demo copy and returns the seeded demo session.
export async function openDemoObjects(page, request) {
  const demo = await seedDemo(request);
  await openOverseer(page);
  return demo;
}

export async function selectDevice(page, deviceId) {
  await page.locator(`#facilityDeviceList button[data-facility-kind="device"][data-facility-id="${deviceId}"]`).click();
  await expect(page.locator('#facilityPlayCard')).toBeVisible();
}

export function playCard(page) {
  const card = page.locator('#facilityPlayCard');
  return {
    card,
    status: page.locator('#facilityPlayStatus'),
    transitions: card.locator('li.facility-play-transition'),
    transition: id => card.locator(`li.facility-play-transition[data-transition-id="${id}"]`),
    run: id => card.locator(`li.facility-play-transition[data-transition-id="${id}"] button.facility-play-transition-run`),
    force: page.locator('#btnForceFacilityDeviceState'),
    faults: card.locator('li.facility-play-fault'),
    fault: id => card.locator(`li.facility-play-fault[data-condition-id="${id}"]`),
    faultSwitch: id => card.locator(`li.facility-play-fault[data-condition-id="${id}"] button.facility-play-fault-switch`),
    reset: page.locator('#btnResetFacilityDevice'),
    feed: page.locator('#facilityPlayFeedList li.facility-feed-entry'),
    feedEmpty: page.locator('#facilityPlayFeedEmpty'),
  };
}

// The feed entries of the selected device, newest first, as rendered.
export async function feedEntries(page) {
  return page.locator('#facilityPlayFeedList li.facility-feed-entry').evaluateAll(items => items.map(item => ({
    sequence: Number(item.dataset.sequence),
    kind: item.dataset.kind,
    author: item.dataset.author,
    reason: item.dataset.reason ?? '',
    text: item.textContent.trim().replace(/\s+/g, ' '),
  })));
}

async function postPlayChange(request, path, data) {
  const before = await facilityState(request);
  const response = await request.post(`${FIXTURE}/${path}`, {
    data: { ...data, expectedFacilityRevision: before.facility.revision, correlationId: `journey-${Date.now()}` },
  });
  expect(response.ok()).toBe(true);
  const result = await response.json();
  expect(result).toMatchObject({ ok: true });
  return result;
}

// Sets device states and fault positions behind the Overseer's back, as a player command would.
export async function forceState(request, deviceId, stateId) {
  return postPlayChange(request, 'force-state', { deviceId, stateId });
}

export async function setConditionActive(request, conditionId, active) {
  return postPlayChange(request, 'set-condition-active', { conditionId, active });
}

// Opens the shared player-state fixture with an Overseer and one connected player.
export async function openPlayerTable(browser, request) {
  const reset = await request.post(`${PLAYER_FIXTURE}/reset`, { data: { scenario: 'ready' } });
  expect(reset.status()).toBe(204);
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await overseer.goto(`${PLAYER_FIXTURE}/overseer`);
  await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ', exact: true }).click();
  await expect(overseer.locator('#mainLayout')).toBeVisible();
  await overseer.getByRole('tab', { name: 'ОБЪЕКТЫ', exact: true }).click();
  const playerContext = await browser.newContext();
  await playerContext.addInitScript(() => {
    localStorage.removeItem('fallout-terminal.player-token');
    HTMLMediaElement.prototype.play = () => Promise.resolve();
  });
  const player = await playerContext.newPage();
  await player.goto('/');
  await expect(player.locator('#connOverlay')).toBeHidden();
  if (await player.locator('#characterSelect').isVisible()) {
    await player.locator('#characterOptions button:not([disabled])').first().click();
  }
  return {
    overseer,
    player,
    close: async () => {
      await playerContext.close();
      await overseerContext.close();
    },
  };
}
