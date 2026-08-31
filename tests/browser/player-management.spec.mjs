import { expect, test } from '@playwright/test';

const FIXTURE = '/__fixture/player-management';
const INITIAL_REVISION = 41;

test.describe.configure({ mode: 'serial' });
test.use({ bypassCSP: true });

async function mountOverseerCandidate(page) {
  await page.evaluate(() => import('http://127.0.0.1:34120/candidate-main.ts?logical-session-player-management'));
}

test.beforeEach(async ({ request, page }) => {
  const reset = await request.post(`${FIXTURE}/reset`);
  expect(reset.status()).toBe(204);
  await page.goto(FIXTURE);
  await mountOverseerCandidate(page);
  await expect(page.locator('#mainLayout')).toBeVisible();
  await expect(page.locator('#btnManagePlayers')).toBeEnabled();
});

async function openPlayerManagement(page) {
  const opener = page.locator('#btnManagePlayers');
  await opener.click();
  const dialog = page.locator('#playerManagementDialog');
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAttribute('open', '');
  return dialog;
}

test('session player and group leaves have one owner and release resources', async ({ page }) => {
  const dialogIDs = [
    'logicalSessionDialog',
    'playerManagementDialog',
    'playerDeleteDialog',
    'terminalGroupDraftDialog',
    'terminalGroupImpactDialog',
  ];
  for (const id of dialogIDs) {
    await expect(page.locator(`#${id}`)).toHaveCount(1);
    await expect(page.locator(`#legacyOverseerRoot #${id}`)).toHaveCount(0);
  }
  await expect(page.locator('#playerConfigVueLeaf #btnManagePlayers')).toHaveCount(1);

  expect(await page.evaluate(() => __desktopFixture.releaseCount('coordination-state'))).toBe(0);
  const released = await page.evaluate(() => {
    __overseerVueFixture.unmount();
    return __desktopFixture.releaseCount('coordination-state');
  });
  expect(released).toBeGreaterThan(0);
  await page.evaluate(() => __overseerVueFixture.unmount());
  expect(await page.evaluate(() => __desktopFixture.releaseCount('coordination-state'))).toBe(released);
  await expect(page.locator('#overseerVueLeaves')).toBeEmpty();
  await expect(page.locator('#playerConfigVueLeaf')).toBeEmpty();
});

test('closed or rebound dialog ignores stale result and releases listener', async ({ page }) => {
  const vueDialog = page.locator('#overseerVueLeaves #playerManagementDialog');
  if (await vueDialog.count() === 0) {
    process.stderr.write('AssertionError: closed or rebound dialog ignores stale result and releases listener\n');
    throw new Error('Vue-owned session/player lifecycle is not implemented');
  }
  await expect(vueDialog).toHaveAttribute('data-stale-result-guard', 'released');
});

test('player configuration preserves references and validation', async ({ page, request }) => {
  const target = page.locator('#playerConfigVueLeaf');
  await expect(target.locator('#btnOpenPlayerConfig')).toHaveCount(1);
  await expect(target.locator('#btnNewPlayerConfig')).toHaveCount(1);

  await target.locator('#btnOpenPlayerConfig').click();
  await expect.poll(() => page.evaluate(() => (
    __desktopFixture.calls.filter(call => call.method === 'OpenPlayerConfig').length
  ))).toBe(1);
  await expect(target.locator('#playerConfigStatus')).toContainText('/private/tmp/open-players.json');
  const associatedSession = await page.evaluate(() => __desktopFixture.authoringSession());
  expect(associatedSession.playerConfig).toBe('open-players.json');
  await page.evaluate(session => window.desktopAPI.saveSession(session), associatedSession);
  await expect.poll(() => page.evaluate(() => (
    __desktopFixture.calls.filter(call => call.method === 'SaveSession').at(-1)?.args?.[0]?.playerConfig
  ))).toBe('open-players.json');

  await target.locator('#btnNewPlayerConfig').click();
  await expect.poll(() => page.evaluate(() => (
    __desktopFixture.calls.filter(call => call.method === 'NewPlayerConfig').length
  ))).toBe(1);
  await expect(target.locator('#playerConfigStatus')).toContainText('/private/tmp/new-players.json');

  await target.locator('#btnManagePlayers').click();
  await expect(page.locator('#playerManagementDialog')).toBeVisible();
  await page.locator('#btnClosePlayerManagement').click();

  await setBroadcast(request, page, true);
  await expect(target.locator('#btnOpenPlayerConfig')).toBeDisabled();
  await expect(target.locator('#btnNewPlayerConfig')).toBeDisabled();
  await expect(target.locator('#playerConfigError')).toBeHidden();

  await page.evaluate(() => __overseerVueFixture.unmount());
  await expect(target).toBeEmpty();
});

test('player management rows reject stale save and release modal resources', async ({ page }) => {
  let releaseUpdate;
  const updateGate = new Promise(resolve => { releaseUpdate = resolve; });
  let observeUpdate;
  const updateObserved = new Promise(resolve => { observeUpdate = resolve; });
  await page.route('**/__fixture/player-management/update', async route => {
    observeUpdate();
    await updateGate;
    await route.continue();
  });

  let dialog = await openPlayerManagement(page);
  await addPlayer(dialog, { name: 'Piper', intelligence: 8, hackerPerkAvailable: false });
  let row = dialog.locator('[data-character-id="fixture-player-1"]');
  await row.locator('.player-name-input').fill('Stale Draft');
  await row.locator('.player-intelligence-input').fill('9');
  await row.locator('.player-hacker-perk-availability').selectOption('true');
  await row.locator('.player-save').click();
  await updateObserved;

  await dialog.locator('#btnClosePlayerManagement').click();
  dialog = await openPlayerManagement(page);
  releaseUpdate();
  await expect(dialog.locator('[data-character-id="fixture-player-1"] .player-name-input')).toHaveValue('Stale Draft');
  await expect(dialog.locator('#playerManagementStatus')).toHaveText('');
  await expect(dialog.locator('#playerManagementError')).toBeHidden();

  await page.evaluate(() => __overseerVueFixture.unmount());
  await expect(page.locator('#playerManagementDialog')).toHaveCount(0);
  await page.evaluate(() => __overseerCoexistenceBridge.legacyToVue({ kind: 'player-management-open-request' }));
  await expect(page.locator('#playerManagementDialog')).toHaveCount(0);
});

async function addPlayer(dialog, { name, intelligence, hackerPerkAvailable }) {
  await dialog.locator('#playerNameInput').fill(name);
  await dialog.locator('#playerIntelligenceInput').fill(String(intelligence));
  await dialog.locator('#playerHackerPerkAvailability').selectOption(String(hackerPerkAvailable));
  await dialog.locator('#btnAddPlayer').click();
}

async function addCalls(page) {
  return page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'AddCharacter'));
}

async function playerManagementState(request) {
  const response = await request.get(`${FIXTURE}/state`);
  expect(response.ok()).toBe(true);
  return response.json();
}

async function setBroadcast(request, page, active) {
  const response = await request.post(`${FIXTURE}/set-broadcast`, {
    data: { active },
  });
  expect(response.ok()).toBe(true);
  const coordination = await response.json();
  const playerConfigRevision = await page.evaluate(() => __desktopFixture.playerConfigRevision());
  coordination.revision = Math.max(coordination.revision, playerConfigRevision + 1);
  await page.evaluate(value => __desktopFixture.emit('coordination-state', value), coordination);
  return coordination;
}

async function updateCalls(page) {
  return page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'UpdateCharacter'));
}

async function deleteCalls(page) {
  return page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'DeleteCharacter'));
}

async function profileMutationCalls(page) {
  return page.evaluate(() => __desktopFixture.calls.filter(call =>
    ['AddCharacter', 'UpdateCharacter', 'DeleteCharacter'].includes(call.method)));
}

async function coordinationCorrectionCalls(page) {
  return page.evaluate(() => __desktopFixture.calls.filter(call =>
    ['AssignCharacter', 'ReleaseCharacter', 'MoveCharacter', 'SetActiveController'].includes(call.method)));
}

async function publishCoordination(page, coordination) {
  await page.evaluate(value => __desktopFixture.emit('coordination-state', value), coordination);
}

function populatedCoordination(roster, { revision = INITIAL_REVISION + 1, broadcast = null, sessions = [] } = {}) {
  return {
    revision,
    playerConfig: {
      status: 'loaded',
      name: 'Player management fixture',
      filePath: '/private/tmp/fallout-player-management.json',
      version: 1,
    },
    roster,
    sessions,
    broadcast,
  };
}

test('opens a dedicated labelled dialog whose complete add profile is required', async ({ page }) => {
  const dialog = await openPlayerManagement(page);

  await expect(dialog).toHaveAccessibleName(/игрок/i);
  await expect(dialog.locator('#playerManagementDialogTitle')).toContainText(/игрок/i);
  await expect(dialog.locator('#playerManagementMode')).toBeVisible();
  await expect(dialog.locator('#playerManagementEmpty')).toBeVisible();

  const name = dialog.locator('#playerNameInput');
  const intelligence = dialog.locator('#playerIntelligenceInput');
  const hacker = dialog.locator('#playerHackerPerkAvailability');
  await expect(name).toHaveAccessibleName(/имя/i);
  await expect(name).toHaveAttribute('required', '');
  await expect(intelligence).toHaveAccessibleName(/интеллект|intelligence/i);
  await expect(intelligence).toHaveAttribute('type', 'number');
  await expect(intelligence).toHaveAttribute('required', '');
  await expect(intelligence).toHaveAttribute('min', '1');
  await expect(intelligence).toHaveAttribute('max', '10');
  await expect(intelligence).toHaveAttribute('step', '1');
  await expect(hacker).toHaveAccessibleName(/хакер|hacker/i);
  await expect(hacker).toHaveAttribute('required', '');
  await expect(hacker).toHaveValue('');

  await dialog.locator('#btnAddPlayer').click();
  expect(await addCalls(page)).toEqual([]);
});

test('accepts Intelligence boundaries and records both explicit Hacker choices', async ({ page }) => {
  const dialog = await openPlayerManagement(page);

  await addPlayer(dialog, { name: 'Low', intelligence: 1, hackerPerkAvailable: false });
  await expect(dialog.locator('[data-character-id="fixture-player-1"]')).toBeVisible();
  await addPlayer(dialog, { name: 'High', intelligence: 10, hackerPerkAvailable: true });
  await expect(dialog.locator('[data-character-id="fixture-player-2"]')).toBeVisible();

  expect(await addCalls(page)).toEqual([
    {
      method: 'AddCharacter',
      args: [{
        name: 'Low',
        intelligence: 1,
        hackerPerkAvailable: false,
        expectedRevision: INITIAL_REVISION,
      }],
    },
    {
      method: 'AddCharacter',
      args: [{
        name: 'High',
        intelligence: 10,
        hackerPerkAvailable: true,
        expectedRevision: INITIAL_REVISION + 1,
      }],
    },
  ]);
});

test('refuses Intelligence 0, 11, fractions, and non-numeric input before the desktop boundary', async ({ page }) => {
  const dialog = await openPlayerManagement(page);
  const name = dialog.locator('#playerNameInput');
  const intelligence = dialog.locator('#playerIntelligenceInput');
  const hacker = dialog.locator('#playerHackerPerkAvailability');
  await name.fill('Invalid Intelligence');
  await hacker.selectOption('false');

  for (const invalid of ['0', '11', '1.5', 'not-a-number']) {
    await intelligence.evaluate((input, value) => {
      input.value = value;
      input.dispatchEvent(new Event('input', { bubbles: true }));
      input.dispatchEvent(new Event('change', { bubbles: true }));
    }, invalid);
    expect(await intelligence.evaluate(input => input.checkValidity())).toBe(false);
    await dialog.locator('#btnAddPlayer').click();
    expect(await addCalls(page)).toEqual([]);
  }
});

test('renders the authoritative add result and restores its persisted profile after reopen', async ({ page }) => {
  let dialog = await openPlayerManagement(page);
  await addPlayer(dialog, { name: '  Piper  ', intelligence: 8, hackerPerkAvailable: false });

  let row = dialog.locator('[data-character-id="fixture-player-1"]');
  await expect(row).toBeVisible();
  await expect(row.locator('.player-name-input')).toHaveValue('Piper');
  await expect(row.locator('.player-intelligence-input')).toHaveValue('8');
  await expect(row.locator('.player-hacker-perk-availability')).toHaveValue('false');
  await expect(dialog.locator('#playerManagementStatus')).not.toHaveText('');
  await expect(dialog.locator('#playerManagementError')).toBeHidden();

  expect(await addCalls(page)).toEqual([{
    method: 'AddCharacter',
    args: [{
      name: 'Piper',
      intelligence: 8,
      hackerPerkAvailable: false,
      expectedRevision: INITIAL_REVISION,
    }],
  }]);

  await dialog.locator('#btnClosePlayerManagement').click();
  await expect(dialog).toBeHidden();
  await page.reload();
  await mountOverseerCandidate(page);
  await expect(page.locator('#mainLayout')).toBeVisible();
  dialog = await openPlayerManagement(page);

  row = dialog.locator('[data-character-id="fixture-player-1"]');
  await expect(row.locator('.player-name-input')).toHaveValue('Piper');
  await expect(row.locator('.player-intelligence-input')).toHaveValue('8');
  await expect(row.locator('.player-hacker-perk-availability')).toHaveValue('false');
});

test('updates one complete profile with explicit Hacker false without changing stable identity or order', async ({ page }) => {
  const dialog = await openPlayerManagement(page);
  await addPlayer(dialog, { name: 'Mara', intelligence: 4, hackerPerkAvailable: true });
  await addPlayer(dialog, { name: 'Boone', intelligence: 6, hackerPerkAvailable: false });

  const first = dialog.locator('[data-character-id="fixture-player-1"]');
  await first.locator('.player-name-input').fill('  Arcade  ');
  await first.locator('.player-intelligence-input').fill('9');
  await first.locator('.player-hacker-perk-availability').selectOption('false');
  await first.locator('.player-save').click();

  expect(await updateCalls(page)).toEqual([{
    method: 'UpdateCharacter',
    args: [{
      characterId: 'fixture-player-1',
      name: 'Arcade',
      intelligence: 9,
      hackerPerkAvailable: false,
      expectedRevision: INITIAL_REVISION + 2,
    }],
  }]);
  await expect(dialog.locator('[data-character-id]')).toHaveCount(2);
  expect(await dialog.locator('[data-character-id]').evaluateAll(rows => rows.map(row => row.dataset.characterId)))
    .toEqual(['fixture-player-1', 'fixture-player-2']);
  await expect(first.locator('.player-name-input')).toHaveValue('Arcade');
  await expect(first.locator('.player-intelligence-input')).toHaveValue('9');
  await expect(first.locator('.player-hacker-perk-availability')).toHaveValue('false');
  await expect(dialog.locator('[data-character-id="fixture-player-2"] .player-name-input')).toHaveValue('Boone');
});

test('uses custom deletion confirmation, cancels without a call, then confirms with the latest revision', async ({ page }) => {
  const dialog = await openPlayerManagement(page);
  await addPlayer(dialog, { name: 'Mara', intelligence: 8, hackerPerkAvailable: false });
  await addPlayer(dialog, { name: 'Boone', intelligence: 5, hackerPerkAvailable: true });

  const row = dialog.locator('[data-character-id="fixture-player-1"]');
  const deleteButton = row.locator('.player-delete');
  await deleteButton.click();
  const confirmation = page.locator('#playerDeleteDialog');
  await expect(confirmation).toBeVisible();
  await expect(confirmation).toHaveAccessibleName(/удалить/i);
  await expect(confirmation.locator('#playerDeleteDialogDescription')).toContainText('Mara');

  await confirmation.locator('#btnCancelPlayerDelete').click();
  await expect(confirmation).toBeHidden();
  await expect(deleteButton).toBeFocused();
  expect(await deleteCalls(page)).toEqual([]);
  await expect(row).toBeVisible();

  await deleteButton.click();
  await confirmation.locator('#btnConfirmPlayerDelete').click();
  expect(await deleteCalls(page)).toEqual([{
    method: 'DeleteCharacter',
    args: [{ characterId: 'fixture-player-1', expectedRevision: INITIAL_REVISION + 2 }],
  }]);
  await expect(confirmation).toBeHidden();
  await expect(dialog.locator('[data-character-id="fixture-player-1"]')).toHaveCount(0);
  expect(await dialog.locator('[data-character-id]').evaluateAll(rows => rows.map(candidate => candidate.dataset.characterId)))
    .toEqual(['fixture-player-2']);
});

test('storage and stale-revision failures keep the dialog open and restore authoritative details', async ({ request, page }) => {
  const dialog = await openPlayerManagement(page);
  await addPlayer(dialog, { name: 'Piper', intelligence: 8, hackerPerkAvailable: false });

  let row = dialog.locator('[data-character-id="fixture-player-1"]');
  const failedSave = await request.post(`${FIXTURE}/fail-next-save`, {
    data: { error: 'active player configuration is missing, unreadable, or changed; reopen or reselect it' },
  });
  expect(failedSave.status()).toBe(204);
  await row.locator('.player-name-input').fill('Corrupted Draft');
  await row.locator('.player-intelligence-input').fill('2');
  await row.locator('.player-hacker-perk-availability').selectOption('true');
  await row.locator('.player-save').click();

  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#playerManagementError')).toContainText(/reopen or reselect/i);
  row = dialog.locator('[data-character-id="fixture-player-1"]');
  await expect(row.locator('.player-name-input')).toHaveValue('Piper');
  await expect(row.locator('.player-intelligence-input')).toHaveValue('8');
  await expect(row.locator('.player-hacker-perk-availability')).toHaveValue('false');

  const advanced = await request.post(`${FIXTURE}/advance-revision`);
  expect(advanced.ok()).toBe(true);
  expect((await advanced.json()).revision).toBe(INITIAL_REVISION + 2);
  await row.locator('.player-name-input').fill('Stale Draft');
  await row.locator('.player-intelligence-input').fill('3');
  await row.locator('.player-hacker-perk-availability').selectOption('true');
  await row.locator('.player-save').click();

  await expect(dialog.locator('#playerManagementError')).toContainText(/state changed|latest player list/i);
  row = dialog.locator('[data-character-id="fixture-player-1"]');
  await expect(row.locator('.player-name-input')).toHaveValue('Piper');
  await expect(row.locator('.player-intelligence-input')).toHaveValue('8');
  await expect(row.locator('.player-hacker-perk-availability')).toHaveValue('false');
  expect((await updateCalls(page)).map(call => call.args[0].expectedRevision))
    .toEqual([INITIAL_REVISION + 1, INITIAL_REVISION + 1]);
});

test('a live coordination event makes details read-only and crafted active mutations leave state unchanged', async ({ request, page }) => {
  const dialog = await openPlayerManagement(page);
  await addPlayer(dialog, { name: 'Veronica', intelligence: 10, hackerPerkAvailable: true });
  const before = await playerManagementState(request);
  const active = await setBroadcast(request, page, true);

  await expect(dialog).toHaveAttribute('aria-readonly', 'true');
  await expect(dialog.locator('#playerManagementMode')).toContainText(/просмотр|read.?only/i);
  const row = dialog.locator('[data-character-id="fixture-player-1"]');
  await expect(row.locator('.player-name-input')).toHaveValue('Veronica');
  await expect(row.locator('.player-intelligence-input')).toHaveValue('10');
  await expect(row.locator('.player-hacker-perk-availability')).toHaveValue('true');
  await expect(dialog.locator('#btnAddPlayer')).toBeDisabled();
  await expect(row.locator('.player-save')).toBeDisabled();
  await expect(row.locator('.player-delete')).toBeDisabled();

  const results = await page.evaluate(async ({ revision }) => ({
    update: await desktopAPI.updateCharacter({
      characterId: 'fixture-player-1',
      name: 'Active Draft',
      intelligence: 1,
      hackerPerkAvailable: false,
      expectedRevision: revision,
    }),
    delete: await desktopAPI.deleteCharacter({
      characterId: 'fixture-player-1',
      expectedRevision: revision,
    }),
    add: await desktopAPI.addCharacter({
      name: 'Active Addition',
      intelligence: 7,
      hackerPerkAvailable: false,
      expectedRevision: revision,
    }),
  }), { revision: active.revision });

  for (const result of Object.values(results)) {
    expect(result.ok).toBe(false);
    expect(result.error).toMatch(/broadcast|трансляц/i);
    expect(result.state.roster).toEqual(before.roster);
  }
  const after = await playerManagementState(request);
  expect(after.revision).toBe(active.revision);
  expect(after.roster).toEqual(before.roster);
  await expect(row.locator('.player-name-input')).toHaveValue('Veronica');
  await expect(row.locator('.player-intelligence-input')).toHaveValue('10');
  await expect(row.locator('.player-hacker-perk-availability')).toHaveValue('true');
});

test('presents populated profile details as an accessible labelled list', async ({ page }) => {
  await publishCoordination(page, populatedCoordination([
    { id: 'detail-piper', name: 'Piper', intelligence: 8, hackerPerkAvailable: false },
    { id: 'detail-veronica', name: 'Veronica', intelligence: 10, hackerPerkAvailable: true },
  ]));

  const dialog = await openPlayerManagement(page);
  const roster = dialog.locator('#playerManagementRoster');
  await expect(roster).toHaveRole('list');
  await expect(roster).toHaveAccessibleName(/подробный список игроков/i);
  await expect(dialog.locator('#playerManagementEmpty')).toBeHidden();
  await expect(roster.getByRole('listitem')).toHaveCount(2);

  const piper = roster.locator('[data-character-id="detail-piper"]');
  await expect(piper.locator('.player-name-input')).toHaveAccessibleName(/^имя$/i);
  await expect(piper.locator('.player-name-input')).toHaveValue('Piper');
  await expect(piper.locator('.player-intelligence-input')).toHaveAccessibleName(/интеллект|intelligence/i);
  await expect(piper.locator('.player-intelligence-input')).toHaveValue('8');
  await expect(piper.locator('.player-hacker-perk-availability')).toHaveAccessibleName(/хакер|hacker/i);
  await expect(piper.locator('.player-hacker-perk-availability')).toHaveValue('false');

  const veronica = roster.locator('[data-character-id="detail-veronica"]');
  await expect(veronica.locator('.player-name-input')).toHaveValue('Veronica');
  await expect(veronica.locator('.player-intelligence-input')).toHaveValue('10');
  await expect(veronica.locator('.player-hacker-perk-availability')).toHaveValue('true');
});

test('empty close button and populated Escape close restore focus without profile mutations', async ({ page }) => {
  const opener = page.locator('#btnManagePlayers');
  let dialog = await openPlayerManagement(page);
  await expect(dialog.locator('#playerManagementEmpty')).toBeVisible();
  await expect(dialog.locator('#playerManagementRoster')).toHaveAccessibleName(/подробный список игроков/i);
  await expect(dialog.locator('#btnClosePlayerManagement')).toBeFocused();
  await dialog.locator('#btnClosePlayerManagement').click();
  await expect(dialog).toBeHidden();
  await expect(opener).toBeFocused();
  expect(await profileMutationCalls(page)).toEqual([]);

  await publishCoordination(page, populatedCoordination([
    { id: 'escape-player', name: 'Arcade', intelligence: 9, hackerPerkAvailable: true },
  ]));
  dialog = await openPlayerManagement(page);
  await expect(dialog.locator('[data-character-id="escape-player"]')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(opener).toBeFocused();
  expect(await profileMutationCalls(page)).toEqual([]);
});

test('contains a long roster within a scrollable mobile dialog', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 640 });
  const roster = Array.from({ length: 12 }, (_, index) => ({
    id: `mobile-player-${index + 1}`,
    name: `Mobile Player ${index + 1}`,
    intelligence: (index % 10) + 1,
    hackerPerkAvailable: index % 2 === 0,
  }));
  await publishCoordination(page, populatedCoordination(roster));

  const dialog = await openPlayerManagement(page);
  await expect(dialog.locator('[data-character-id]')).toHaveCount(12);
  const metrics = await dialog.evaluate(element => {
    const bounds = element.getBoundingClientRect();
    const style = getComputedStyle(element);
    element.scrollTop = element.scrollHeight;
    return {
      top: bounds.top,
      right: bounds.right,
      bottom: bounds.bottom,
      left: bounds.left,
      clientHeight: element.clientHeight,
      scrollHeight: element.scrollHeight,
      scrollTop: element.scrollTop,
      overflowY: style.overflowY,
    };
  });
  expect(metrics.left).toBeGreaterThanOrEqual(0);
  expect(metrics.top).toBeGreaterThanOrEqual(0);
  expect(metrics.right).toBeLessThanOrEqual(390);
  expect(metrics.bottom).toBeLessThanOrEqual(640);
  expect(metrics.scrollHeight).toBeGreaterThan(metrics.clientHeight);
  expect(metrics.scrollTop).toBeGreaterThan(0);
  expect(metrics.overflowY).toMatch(/auto|scroll/);
});

test('keeps assignment, release, controller, and claimed transfer corrections in logical-session rows during broadcast', async ({ page }) => {
  const activeCharacter = {
    id: 'character-active', name: 'Piper', intelligence: 8, hackerPerkAvailable: false,
    claimedBySessionId: 'session-active',
  };
  const observerCharacter = {
    id: 'character-observer', name: 'Veronica', intelligence: 10, hackerPerkAvailable: true,
    claimedBySessionId: 'session-observer',
  };
  const availableCharacter = {
    id: 'character-available', name: 'Arcade', intelligence: 9, hackerPerkAvailable: true,
  };
  await publishCoordination(page, populatedCoordination(
    [activeCharacter, observerCharacter, availableCharacter],
    {
      broadcast: { id: 'broadcast-live' },
      sessions: [
        {
          id: 'session-active', fallbackName: 'Active terminal', connected: true, role: 'active',
          character: { id: activeCharacter.id, name: activeCharacter.name },
        },
        {
          id: 'session-observer', fallbackName: 'Observer terminal', connected: true, role: 'observer',
          character: { id: observerCharacter.id, name: observerCharacter.name },
        },
        {
          id: 'session-unassigned', fallbackName: 'Spare terminal', connected: true, role: 'unassigned',
          character: null,
        },
      ],
    },
  ));

  const dialog = await openPlayerManagement(page);
  await expect(dialog).toHaveAttribute('aria-readonly', 'true');
  await expect(dialog.locator('[data-character-id="character-observer"] .player-name-input')).toHaveValue('Veronica');
  await dialog.locator('#btnClosePlayerManagement').click();

  await page.locator('#btnManageLogicalSessions').click();
  const sessionDialog = page.getByRole('dialog', { name: 'ЛОГИЧЕСКИЕ СЕССИИ' });
  await expect(sessionDialog).toBeVisible();
  const unassigned = sessionDialog.locator('[data-session-id="session-unassigned"]');
  const observer = sessionDialog.locator('[data-session-id="session-observer"]');
  await expect(unassigned.locator('.session-assignment-controls')).toBeVisible();
  await expect(unassigned.locator('.session-character-select')).toHaveAccessibleName(/назначить персонажа/i);
  await expect(unassigned.locator('.session-assign')).toBeEnabled();
  await unassigned.locator('.session-character-select').selectOption(availableCharacter.id);
  await unassigned.locator('.session-assign').click();

  await expect(observer.locator('.session-release')).toBeVisible();
  await expect(observer.locator('.session-release')).toBeEnabled();
  await observer.locator('.session-release').click();
  await expect(observer.locator('.session-controller')).toBeVisible();
  await expect(observer.locator('.session-controller')).toBeEnabled();
  await observer.locator('.session-controller').click();

  // T028 relocates the established MoveCharacter correction into the claimed
  // logical-session row; these stable classes are the browser-facing contract.
  const moveSelect = observer.locator('.session-move-session-select');
  const moveButton = observer.locator('.session-move');
  await expect(moveSelect).toHaveAccessibleName(/переместить в сессию/i);
  await expect(moveSelect).toHaveValue('session-unassigned');
  await expect(moveButton).toBeEnabled();
  await moveButton.click();

  expect(await coordinationCorrectionCalls(page)).toEqual([
    {
      method: 'AssignCharacter',
      args: [{ sessionId: 'session-unassigned', characterId: 'character-available' }],
    },
    { method: 'ReleaseCharacter', args: ['session-observer'] },
    { method: 'SetActiveController', args: ['session-observer'] },
    {
      method: 'MoveCharacter',
      args: [{ characterId: 'character-observer', toSessionId: 'session-unassigned' }],
    },
  ]);
  expect(await profileMutationCalls(page)).toEqual([]);
  await expect(page.locator('#characterRoster')).toHaveCount(0);
  await expect(page.locator('#characterNameInput')).toHaveCount(0);
  await expect(page.locator('#btnAddCharacter')).toHaveCount(0);
  await expect(page.locator('#characterRosterRowTemplate')).toHaveCount(0);
});
