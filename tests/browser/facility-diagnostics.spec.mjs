import { expect, test } from '@playwright/test';

import { FACILITY_IDS } from './fixtures/facility-session.mjs';

const FIXTURE = '/__fixture/facility-diagnostics';
const TOKEN_KEY = 'fallout-terminal.player-token';

const conditionScenarios = [
  {
    scenario: 'offline',
    conditionId: FACILITY_IDS.conditions.offline,
    category: 'offline',
    capability: 'view-entry',
    blockedLabel: 'AFFECTED ENTRY',
  },
  {
    scenario: 'unpowered',
    conditionId: FACILITY_IDS.conditions.unpowered,
    category: 'unpowered',
    capability: 'execute-command',
    blockedLabel: 'AFFECTED COMMAND',
  },
  {
    scenario: 'network-isolated',
    conditionId: FACILITY_IDS.conditions.networkIsolated,
    category: 'network-isolated',
    capability: 'terminal-transition',
    blockedLabel: 'REMOTE TERMINAL',
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
    capability: 'execute-command',
    blockedLabel: 'SECURITY OVERRIDE',
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
    capability: 'hack',
    blockedLabel: 'AFFECTED HACK',
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

async function expectBlockedWithEscape(page, label) {
  await page.locator('.term-row', { hasText: label }).click();
  await expect(page.locator('#entryBody')).toHaveText('Ошибка доступа');
  await expect(page.locator('#backBtn')).toBeVisible();
  await page.locator('#backBtn').click();
  await expect(page.locator('#termList')).toBeVisible();
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
      expect(initial.blockedCapabilities).toEqual(scenario.capability ? [scenario.capability] : []);
      expect(initial.presentationEffects).toEqual(scenario.presentationEffect ? [scenario.presentationEffect] : []);

      if (scenario.blockedLabel) {
        await expectBlockedWithEscape(participant.page, scenario.blockedLabel);
      }
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

test('private Overseer recovery remains available when every player capability is blocked', async ({ browser, request }) => {
  await resetDiagnosticFixture(request, 'private-recovery-escape');
  const participant = await openParticipant(browser);
  try {
    const before = await diagnosticState(request);
    await expectBlockedWithEscape(participant.page, 'AFFECTED COMMAND');
    const response = await request.post(`${FIXTURE}/recover-private`, {
      data: {
        conditionId: FACILITY_IDS.conditions.authorizationCorrupted,
        expectedFacilityRevision: before.facility.revision,
        correlationId: 'browser-private-recovery',
      },
    });
    expect(response.ok()).toBe(true);
    expect(await response.json()).toMatchObject({
      ok: true,
      changed: true,
      correlationId: 'browser-private-recovery',
      previousFacilityRevision: before.facility.revision,
      resultingFacilityRevision: before.facility.revision + 1,
    });

    await expect.poll(async () => (await diagnosticState(request)).facility.revision)
      .toBe(before.facility.revision + 1);
    const after = await diagnosticState(request);
    expect(after.facility.conditionStates[FACILITY_IDS.conditions.authorizationCorrupted]).toBe(false);
    expect(after.audit).toMatchObject({ durableWrites: 1, privateRecoveries: 1 });
    await expect(participant.page.locator('.term-row', { hasText: 'AFFECTED COMMAND' }))
      .not.toHaveAttribute('aria-disabled', 'true');
  } finally {
    await participant.context.close();
  }
});
