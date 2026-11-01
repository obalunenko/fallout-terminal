import { expect, test } from '@playwright/test';

import { FACILITY_IDS } from './fixtures/facility-session.mjs';

const FIXTURE = '/__fixture/facility-diagnostics';
const TOKEN_KEY = 'fallout-terminal.player-token';
// The player reads one reason for every fault-blocked request, without the
// Overseer-only fault name (042 FR-002).
const FAULT_BLOCKED_NOTICE = 'ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.';
// The blockable entry's own text, which no refusal or error message shares.
const AFFECTED_ENTRY_TEXT = 'AFFECTED ENTRY TEXT // READABLE';

const conditionScenarios = [
  {
    scenario: 'offline',
    conditionId: FACILITY_IDS.conditions.offline,
    category: 'offline',
    capabilities: ['view-entry'],
    blockedLabels: ['AFFECTED ENTRY'],
  },
  {
    scenario: 'unpowered',
    conditionId: FACILITY_IDS.conditions.unpowered,
    category: 'unpowered',
    capabilities: ['execute-command'],
    blockedLabels: ['AFFECTED COMMAND'],
  },
  {
    scenario: 'network-isolated',
    conditionId: FACILITY_IDS.conditions.networkIsolated,
    category: 'network-isolated',
    capabilities: ['terminal-transition', 'run-recovery-program'],
    blockedLabels: ['REMOTE TERMINAL', 'RUN RECOVERY PROGRAM'],
    diagnosticLabel: 'ISOLATION DIAGNOSTICS',
  },
  {
    scenario: 'storage-damaged',
    conditionId: FACILITY_IDS.conditions.storageDamaged,
    category: 'storage-damaged',
    recordLabel: 'DAMAGED RECORD',
  },
  {
    scenario: 'authorization-corrupted',
    conditionId: FACILITY_IDS.conditions.authorizationCorrupted,
    category: 'authorization-corrupted',
    capabilities: ['execute-command'],
    blockedLabels: ['SECURITY OVERRIDE'],
  },
  {
    scenario: 'display-unstable',
    conditionId: FACILITY_IDS.conditions.displayUnstable,
    category: 'display-unstable',
    presentationEffect: 'display-unstable',
  },
  {
    scenario: 'custom',
    conditionId: FACILITY_IDS.conditions.custom,
    category: 'custom',
    customCategory: 'coolant-contamination',
    capabilities: ['hack'],
    blockedHack: true,
  },
];

async function resetDiagnosticFixture(request, scenario) {
  const response = await request.post(`${FIXTURE}/reset`, { data: { scenario } });
  expect(response.status()).toBe(204);
}

async function diagnosticState(request) {
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
  await expect(page.locator('#roleBadge')).toContainText('АКТИВЕН');
  return { context, page };
}

async function openOverseer(browser) {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(`${FIXTURE}/overseer`);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
  return { context, page };
}

// expectRefused selects label and checks that the server refused it with the
// fault reason: the notice shows, and the list stays with nothing opened.
async function expectRefused(page, label) {
  await page.locator('.term-row', { hasText: label }).click();
  await expect(page.locator('#playerNotice')).toHaveText(FAULT_BLOCKED_NOTICE);
  await expect(page.locator('#termList')).toBeVisible();
  await expect(page.locator('#termEntry')).toBeHidden();
  await expect(page.locator('body')).not.toContainText(AFFECTED_ENTRY_TEXT);
}

// expectHackRefused makes one guess on the locked terminal and checks that
// the server refused it with the fault reason.
async function expectHackRefused(page) {
  await page.locator('#hackColumns [data-target]:not([data-target=""])').first().click();
  await expect(page.locator('#playerNotice')).toHaveText(FAULT_BLOCKED_NOTICE);
}

async function switchCondition(request, conditionId, active, correlationId) {
  const before = await diagnosticState(request);
  const response = await request.post(`${FIXTURE}/set-condition-active`, {
    data: { conditionId, active, expectedFacilityRevision: before.facility.revision, correlationId },
  });
  expect(response.ok()).toBe(true);
  await expect.poll(async () => (await diagnosticState(request)).facility.conditionStates[conditionId]).toBe(active);
}

for (const scenario of conditionScenarios) {
  test(`${scenario.category} condition has only its authored deterministic effect`, async ({ browser, request }) => {
    await resetDiagnosticFixture(request, scenario.scenario);
    const participant = await openParticipant(browser);
    try {
      const initial = await diagnosticState(request);
      expect(initial.activeCondition).toEqual({
        id: scenario.conditionId,
        category: scenario.category,
        ...(scenario.customCategory ? { customCategory: scenario.customCategory } : {}),
      });
      expect(initial.facility.conditionStates[scenario.conditionId]).toBe(true);
      expect(initial.blockedCapabilities).toEqual(scenario.capabilities ?? []);
      expect(initial.presentationEffects).toEqual(scenario.presentationEffect ? [scenario.presentationEffect] : []);

      for (const label of scenario.blockedLabels ?? []) await expectRefused(participant.page, label);
      if (scenario.blockedHack) await expectHackRefused(participant.page);
      if (scenario.diagnosticLabel) {
        const diagnostic = participant.page.locator('.term-row', { hasText: scenario.diagnosticLabel });
        await expect(diagnostic).toBeVisible();
        await diagnostic.click();
        await expect(participant.page.locator('#backBtn')).toBeVisible();
        await participant.page.locator('#backBtn').click();
      }
      if (scenario.recordLabel) {
        await expect(participant.page.locator('.term-row', { hasText: scenario.recordLabel })).toBeVisible();
      }
      if (scenario.presentationEffect) {
        await expect(participant.page.locator('#screen')).toHaveAttribute(
          'data-presentation-effect',
          new RegExp(`(^|\\s)${scenario.presentationEffect}(\\s|$)`),
        );
      }

      const after = await diagnosticState(request);
      expect(after.facility).toEqual(initial.facility);
      expect(after.audit.durableWrites).toBe(0);
    } finally {
      await participant.context.close();
    }
  });
}

test('damaged authored records remain deterministic and navigable across multiple pages', async ({ browser, request }) => {
  await resetDiagnosticFixture(request, 'storage-damaged-multipage');
  const participant = await openParticipant(browser);
  try {
    await participant.page.setViewportSize({ width: 390, height: 460 });
    const before = await diagnosticState(request);
    await participant.page.locator('.term-row', { hasText: 'DAMAGED RECORD' }).click();
    await expect(participant.page.locator('#entryBody')).not.toContainText('CORRIDOR PRESSURE NOMINAL');
    await expect(participant.page.locator('#pageIndicator')).toHaveText(/1 \/ [2-9]/);
    const firstPage = await participant.page.locator('#entryBody').innerText();
    expect(firstPage).toMatch(/[_?]{2,}/);

    await participant.page.getByRole('button', { name: 'Следующая страница' }).click();
    await expect(participant.page.locator('#pageIndicator')).toHaveText(/2 \/ [2-9]/);
    const secondPage = await participant.page.locator('#entryBody').innerText();
    expect(secondPage).toMatch(/[_?]{2,}/);
    expect(secondPage).not.toBe(firstPage);

    await participant.page.locator('#backBtn').click();
    await participant.page.locator('.term-row', { hasText: 'DAMAGED RECORD' }).click();
    await expect(participant.page.locator('#entryBody')).toHaveText(firstPage);
    const after = await diagnosticState(request);
    expect(after.facility).toEqual(before.facility);
    expect(after.authoredRecordDigest).toBe(before.authoredRecordDigest);
    expect(after.audit.durableWrites).toBe(0);
  } finally {
    await participant.context.close();
  }
});

test('replaying display instability is deterministic and cannot mutate or delete content', async ({ browser, request }) => {
  await resetDiagnosticFixture(request, 'display-unstable');
  const participant = await openParticipant(browser);
  try {
    const before = await diagnosticState(request);
    const presentations = [];
    for (let replay = 0; replay < 5; replay++) {
      const response = await request.post(`${FIXTURE}/replay-projection`);
      expect(response.status()).toBe(204);
      await expect(participant.page.locator('#screen')).toHaveAttribute(
        'data-presentation-effect', /(^|\s)display-unstable(\s|$)/,
      );
      await expect(participant.page.locator('.term-row', { hasText: 'DAMAGED RECORD' })).toBeVisible();
      presentations.push(await participant.page.locator('#termList').innerText());
    }
    expect(new Set(presentations).size).toBe(1);
    expect(presentations[0]).toContain('STABLE REFERENCE');

    const after = await diagnosticState(request);
    expect(after.facility).toEqual(before.facility);
    expect(after.authoredContentDigest).toBe(before.authoredContentDigest);
    expect(after.audit).toMatchObject({ durableWrites: 0, visualStateMutations: 0 });
    expect(after.audit.projectionReplays).toBe(5);
  } finally {
    await participant.context.close();
  }
});

for (const recovery of [
  { scenario: 'transition-recovery', label: 'RESTORE PRIMARY POWER', conditionId: FACILITY_IDS.conditions.unpowered },
  { scenario: 'program-recovery', label: 'RUN NETWORK RECOVERY HOLOTAPE', conditionId: FACILITY_IDS.conditions.networkIsolated },
]) {
  test(`${recovery.scenario} clears its condition only after private approval`, async ({ browser, request }) => {
    await resetDiagnosticFixture(request, recovery.scenario);
    const overseer = await openOverseer(browser);
    const participant = await openParticipant(browser);
    try {
      const before = await diagnosticState(request);
      await participant.page.locator('.term-row', { hasText: recovery.label }).click();
      const approval = overseer.page.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
      await expect(approval).toBeVisible();
      expect((await diagnosticState(request)).facility).toEqual(before.facility);
      await approval.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();

      await expect.poll(async () => (await diagnosticState(request)).facility.revision)
        .toBe(before.facility.revision + 1);
      const after = await diagnosticState(request);
      expect(after.facility.conditionStates[recovery.conditionId]).toBe(false);
      expect(after.audit).toMatchObject({ durableWrites: 1, approvedRecoveries: 1 });
    } finally {
      await participant.context.close();
      await overseer.context.close();
    }
  });
}

// The Overseer's fault switch, SetFacilityConditionActive, is the escape hatch:
// it clears a terminal fault that leaves players no command to recover with.
test('the terminal-fault switch clears a fault while every player capability is blocked (FR-021)', async ({ browser, request }) => {
  await resetDiagnosticFixture(request, 'private-recovery-escape');
  const participant = await openParticipant(browser);
  const conditionId = FACILITY_IDS.conditions.authorizationCorrupted;
  try {
    const before = await diagnosticState(request);
    expect(before.facility.conditionStates[conditionId]).toBe(true);
    expect(before.blockedCapabilities).toEqual(['execute-command']);
    await expectRefused(participant.page, 'AFFECTED COMMAND');
    const response = await request.post(`${FIXTURE}/set-condition-active`, {
      data: {
        conditionId,
        active: false,
        expectedFacilityRevision: before.facility.revision,
        correlationId: 'browser-terminal-fault-switch',
      },
    });
    expect(response.ok()).toBe(true);
    expect(await response.json()).toMatchObject({
      ok: true,
      changed: true,
      correlationId: 'browser-terminal-fault-switch',
      previousFacilityRevision: before.facility.revision,
      resultingFacilityRevision: before.facility.revision + 1,
    });

    await expect.poll(async () => (await diagnosticState(request)).facility.revision)
      .toBe(before.facility.revision + 1);
    const after = await diagnosticState(request);
    expect(after.facility.conditionStates[conditionId]).toBe(false);
    expect(after.blockedCapabilities).toEqual([]);
    expect(after.audit).toMatchObject({ durableWrites: 1, conditionSwitches: 1, approvedRecoveries: 0 });
    // The same request now reaches the Overseer instead of being refused.
    await participant.page.locator('.term-row', { hasText: 'AFFECTED COMMAND' }).click();
    await expect(participant.page.locator('#entryBody')).toContainText('Выполняется запрос');
    await expect(participant.page.locator('#playerNotice')).not.toHaveText(FAULT_BLOCKED_NOTICE);
  } finally {
    await participant.context.close();
  }
});

test('a blocked entry stays closed until the Overseer switches its fault off (042 FR-014)', async ({ browser, request }) => {
  await resetDiagnosticFixture(request, 'offline');
  const participant = await openParticipant(browser);
  try {
    await expectRefused(participant.page, 'AFFECTED ENTRY');
    await expectRefused(participant.page, 'AFFECTED ENTRY');

    await switchCondition(request, FACILITY_IDS.conditions.offline, false, 'browser-offline-switch');
    await participant.page.locator('.term-row', { hasText: 'AFFECTED ENTRY' }).click();
    await expect(participant.page.locator('#entryBody')).toContainText(AFFECTED_ENTRY_TEXT);
    await expect(participant.page.locator('#playerNotice')).toBeHidden();
  } finally {
    await participant.context.close();
  }
});
