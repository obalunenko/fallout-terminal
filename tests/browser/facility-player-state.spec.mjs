import { expect, test } from '@playwright/test';

import { FACILITY_IDS } from './fixtures/facility-session.mjs';

const FIXTURE = '/__fixture/facility-player-state';
const OVERSEER_URL = `${FIXTURE}/overseer`;
const TOKEN_KEY = 'fallout-terminal.player-token';
const FACILITY_CONVERGENCE_TIMEOUT_MS = 1_000;

async function resetFacilityFixture(request, scenario = 'ready') {
  const response = await request.post(`${FIXTURE}/reset`, { data: { scenario } });
  expect(response.status()).toBe(204);
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

async function openJourney(browser) {
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  await overseer.goto(OVERSEER_URL);
  await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(overseer.locator('#mainLayout')).toBeVisible();

  const controller = await openParticipant(browser);
  const observer = await openParticipant(browser);
  await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
  await expect(observer.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');
  return { overseerContext, overseer, controller, observer };
}

async function closeJourney(journey) {
  await journey.controller.context.close();
  await journey.observer.context.close();
  await journey.overseerContext.close();
}

async function facilityState(request) {
  const response = await request.get(`${FIXTURE}/state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

async function activateFacilityTerminal(request, terminalId) {
  const response = await request.post(`${FIXTURE}/activate-terminal`, { data: { terminalId } });
  expect(response.status()).toBe(204);
}

async function applyProjectionTransition(request) {
  const response = await request.post(`${FIXTURE}/apply-projection-transition`);
  expect(response.status()).toBe(204);
}

async function moveFacilityTerminal(request, terminalId, groupId) {
  const response = await request.post(`${FIXTURE}/move-terminal`, { data: { terminalId, groupId } });
  expect(response.status()).toBe(204);
}

async function requestDoorAction(journey) {
  await journey.controller.page.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' }).click();
  const dialog = journey.overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText(FACILITY_IDS.devices.door);
  await expect(dialog).toContainText(FACILITY_IDS.devices.alarm);
  for (const participant of [journey.controller, journey.observer]) {
    await expect(participant.page.locator('#entryBody')).toContainText('Выполняется запрос');
  }
  return dialog;
}

test('rejection is explicit and leaves the complete facility pre-state authoritative', async ({ browser, request }) => {
  await resetFacilityFixture(request);
  const before = await facilityState(request);
  const journey = await openJourney(browser);
  try {
    const dialog = await requestDoorAction(journey);
    expect((await facilityState(request)).facility).toEqual(before.facility);

    await dialog.getByRole('button', { name: 'ОТКЛОНИТЬ' }).click();
    for (const participant of [journey.controller, journey.observer]) {
      await expect(participant.page.locator('#entryBody')).toHaveText('Ошибка доступа');
    }
    const after = await facilityState(request);
    expect(after.facility).toEqual(before.facility);
    expect(after.lastFacilityResult).toMatchObject({ ok: false, failure: 'rejected' });
  } finally {
    await closeJourney(journey);
  }
});

test('approval commits the door and alarm together once after durability', async ({ browser, request }) => {
  await resetFacilityFixture(request);
  const journey = await openJourney(browser);
  try {
    const dialog = await requestDoorAction(journey);
    await dialog.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();

    await expect.poll(async () => (await facilityState(request)).facility.revision).toBe(1);
    const after = await facilityState(request);
    expect(after.facility.deviceStates[FACILITY_IDS.devices.door]).toBe('open');
    expect(after.facility.deviceStates[FACILITY_IDS.devices.alarm]).toBe('silent');
    expect(after.audit).toMatchObject({ durableWrites: 1, successfulWorldActions: 1 });
    for (const participant of [journey.controller, journey.observer]) {
      await expect(participant.page.locator('.term-row', { hasText: 'SECURITY DOOR OPEN' })).toBeVisible();
    }
  } finally {
    await closeJourney(journey);
  }
});

for (const { scenario, failure } of [
  { scenario: 'stale-revision', failure: 'stale-revision' },
  { scenario: 'persistence-failure', failure: 'persistence-failed' },
]) {
  test(`${scenario} returns a structured failure without partial mutation`, async ({ browser, request }) => {
    await resetFacilityFixture(request, scenario);
    const before = await facilityState(request);
    const journey = await openJourney(browser);
    try {
      const dialog = await requestDoorAction(journey);
      await dialog.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
      await expect(journey.controller.page.locator('#entryBody')).toHaveText('Ошибка доступа');
      const after = await facilityState(request);
      expect(after.facility).toEqual(before.facility);
      expect(after.lastFacilityResult).toMatchObject({ ok: false, failure });
      expect(after.audit.durableWrites).toBe(0);
    } finally {
      await closeJourney(journey);
    }
  });
}

test('repeated and concurrent resolutions cannot duplicate one world action', async ({ browser, request }) => {
  await resetFacilityFixture(request, 'concurrent-resolution');
  const journey = await openJourney(browser);
  try {
    const dialog = await requestDoorAction(journey);
    await Promise.all([
      dialog.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click(),
      request.post(`${FIXTURE}/repeat-current-decision`),
      request.post(`${FIXTURE}/repeat-current-decision`),
    ]);
    await expect.poll(async () => (await facilityState(request)).audit.resolutionAttempts).toBe(3);
    const after = await facilityState(request);
    expect(after.facility.revision).toBe(1);
    expect(after.audit.durableWrites).toBe(1);
    expect(after.audit.successfulWorldActions).toBe(1);
    expect(after.audit.duplicateResults).toBe(2);
  } finally {
    await closeJourney(journey);
  }
});

test('one facility projection converges across five terminals, observers, and reconnects', async ({ browser, request }) => {
  await resetFacilityFixture(request, 'shared-projection');
  const journey = await openJourney(browser);
  let reconnect;
  try {
    await activateFacilityTerminal(request, FACILITY_IDS.terminals.security);
    await journey.controller.page.locator('.term-row', { hasText: 'FACILITY STATUS' }).click();
    await expect(journey.controller.page.locator('#entryBody')).toContainText('SECURITY DOOR: LOCKED');

    await applyProjectionTransition(request);
    await Promise.all([journey.controller, journey.observer].flatMap(participant => [
      expect(participant.page.locator('#entryBody')).toContainText(
        'SECURITY DOOR: OPEN',
        { timeout: FACILITY_CONVERGENCE_TIMEOUT_MS },
      ),
      expect(participant.page.locator('#entryBody')).not.toContainText(
        'SECURITY DOOR: LEGACY COMMAND COMPLETE',
        { timeout: FACILITY_CONVERGENCE_TIMEOUT_MS },
      ),
    ]));

    await journey.controller.page.locator('#backBtn').click();
    for (const participant of [journey.controller, journey.observer]) {
      await expect(participant.page.locator('.term-row', { hasText: 'FACILITY STATUS // ACCESS OPEN' }))
        .toBeVisible({ timeout: FACILITY_CONVERGENCE_TIMEOUT_MS });
      await expect(participant.page.locator('.term-row', { hasText: 'RESTRICTED ARCHIVE' }))
        .toBeVisible({ timeout: FACILITY_CONVERGENCE_TIMEOUT_MS });
      await expect(participant.page.locator('.term-row', { hasText: 'SECURITY DOOR OPEN' }))
        .not.toHaveAttribute('aria-disabled', 'true', { timeout: FACILITY_CONVERGENCE_TIMEOUT_MS });
    }

    const terminalChecks = [
      [FACILITY_IDS.terminals.security, 'FACILITY STATUS // ACCESS OPEN'],
      [FACILITY_IDS.terminals.reactor, 'REACTOR STATUS'],
      [FACILITY_IDS.terminals.maintenance, 'DIAGNOSTIC TOOLS'],
      [FACILITY_IDS.terminals.network, 'NETWORK STATUS'],
      [FACILITY_IDS.terminals.archive, 'RECORD 04-B'],
    ];
    for (const [terminalId, visibleLabel] of terminalChecks) {
      await activateFacilityTerminal(request, terminalId);
      await expect(journey.controller.page.locator('.term-row', { hasText: visibleLabel }))
        .toBeVisible({ timeout: FACILITY_CONVERGENCE_TIMEOUT_MS });
      await expect(journey.observer.page.locator('.term-row', { hasText: visibleLabel }))
        .toBeVisible({ timeout: FACILITY_CONVERGENCE_TIMEOUT_MS });
    }

    reconnect = await openParticipant(browser);
    await expect(reconnect.page.locator('.term-row', { hasText: 'RECORD 04-B' })).toBeVisible();
    const state = await facilityState(request);
    expect(state.facility.terminalCount).toBe(5);
    expect(state.facility.groupCount).toBe(3);
    expect(state.facility.deviceStates[FACILITY_IDS.devices.door]).toBe('open');
  } finally {
    await reconnect?.context.close();
    await closeJourney(journey);
  }
});

test('open EntryContent repaginates after a live device change and group moves preserve facility state', async ({ browser, request }) => {
  await resetFacilityFixture(request, 'shared-projection');
  const journey = await openJourney(browser);
  try {
    await journey.controller.page.setViewportSize({ width: 520, height: 640 });
    await activateFacilityTerminal(request, FACILITY_IDS.terminals.security);
    await journey.controller.page.locator('.term-row', { hasText: 'FACILITY STATUS' }).click();
    const rowsBefore = await journey.controller.page.locator('#entryBody > div').count();

    await applyProjectionTransition(request);
    await expect(journey.controller.page.locator('#entryBody')).toContainText('SECURITY DOOR: OPEN');
    await journey.controller.page.setViewportSize({ width: 390, height: 520 });
    await expect.poll(() => journey.controller.page.locator('#entryBody > div').count()).not.toBe(rowsBefore);

    const beforeMove = (await facilityState(request)).facility;
    await moveFacilityTerminal(request, FACILITY_IDS.terminals.security, FACILITY_IDS.groups.engineering);
    const afterMove = (await facilityState(request)).facility;
    expect(afterMove.revision).toBe(beforeMove.revision);
    expect(afterMove.deviceStates).toEqual(beforeMove.deviceStates);
    expect(afterMove.conditionStates).toEqual(beforeMove.conditionStates);
    expect(afterMove.deviceIds).toEqual(beforeMove.deviceIds);
  } finally {
    await closeJourney(journey);
  }
});
