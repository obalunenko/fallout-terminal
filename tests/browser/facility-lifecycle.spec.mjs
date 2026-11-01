import { expect, test } from '@playwright/test';

import { FACILITY_IDS } from './fixtures/facility-session.mjs';

const FIXTURE = '/__fixture/facility-lifecycle';
const TOKEN_KEY = 'fallout-terminal.player-token';

async function resetFixture(request, scenario = 'persisted') {
  const response = await request.post(`${FIXTURE}/reset`, { data: { scenario } });
  expect(response.status()).toBe(204);
}

async function lifecycle(request, action) {
  const response = await request.post(`${FIXTURE}/${action}`);
  expect(response.status()).toBe(204);
}

async function lifecycleState(request) {
  const response = await request.get(`${FIXTURE}/state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

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

async function resumeParticipant(participant) {
  await expect(participant.page.locator('#characterSelect')).toBeVisible();
  await participant.page.locator('#characterOptions button:not([disabled])').first().click();
  await expect(participant.page.locator('#characterSelect')).toBeHidden();
  await expect(participant.page.locator('#termList')).toBeVisible();
}

test('broadcast stop and start preserve the committed facility before players resume', async ({ browser, request }) => {
  await resetFixture(request);
  const participant = await openParticipant(browser);
  try {
    const before = await lifecycleState(request);
    await expect(participant.page.locator('#termList')).toContainText('SECURITY DOOR: OPEN');

    await lifecycle(request, 'stop-start-broadcast');
    await resumeParticipant(participant);
    const after = await lifecycleState(request);
    expect(after.facility).toEqual(before.facility);
    expect(after.hydratedBeforePublication).toBe(true);
    await expect(participant.page.locator('#termList')).toContainText('SECURITY DOOR: OPEN');
  } finally {
    await participant.context.close();
  }
});

test('session replacement invalidates an old pending request and restores the loaded facility', async ({ browser, request }) => {
  await resetFixture(request, 'pending');
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await overseer.goto(`${FIXTURE}/overseer`);
  await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  const participant = await openParticipant(browser);
  try {
    await participant.page.locator('.term-row', { hasText: 'SECURE SECURITY DOOR' }).click();
    const approval = overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
    await expect(approval).toBeVisible();

    await lifecycle(request, 'reload-session');
    await expect(approval).toBeHidden();
    const state = await lifecycleState(request);
    expect(state.pendingRequests).toBe(0);
    expect(state.staleResolutions).toBe(1);
    expect(state.facility.revision).toBe(12);
    expect(state.facility.deviceStates[FACILITY_IDS.devices.door]).toBe('open');
  } finally {
    await participant.context.close();
    await overseerContext.close();
  }
});

test('fresh-process and self-update handoff restore the same complete facility before reconnect', async ({ browser, request }) => {
  await resetFixture(request);
  const before = await lifecycleState(request);

  for (const action of ['restart-process', 'self-update-handoff']) {
    await lifecycle(request, action);
    const participant = await openParticipant(browser);
    try {
      const after = await lifecycleState(request);
      expect(after.facility).toEqual(before.facility);
      expect(after.restoreSequence.at(-1)).toMatchObject({ action, facilityBeforePublic: true });
      await expect(participant.page.locator('#termList')).toContainText('SECURITY DOOR: OPEN');
    } finally {
      await participant.context.close();
    }
  }
});

test('a version-1 session without facility data loads without inventing world state', async ({ browser, request }) => {
  await resetFixture(request, 'legacy-v1');
  const participant = await openParticipant(browser);
  try {
    const state = await lifecycleState(request);
    expect(state.facility).toBeNull();
    expect(state.persistedFacility).toBeNull();
    await expect(participant.page.locator('#termList')).toContainText('LEGACY TERMINAL READY');
    await expect(participant.page.locator('#backBtn')).toBeEnabled();
  } finally {
    await participant.context.close();
  }
});
