import { expect, test } from '@playwright/test';

const overseerAppModuleURL = 'http://127.0.0.1:34121/@fs' + new URL('./fixtures/overseer-app.ts', import.meta.url).pathname;

const FIXTURE_URL = '/__fixture/state-changing-command-authoring';

test.use({ bypassCSP: true });

async function mountOverseerFixture(page) {
  await page.evaluate(url => import(url), overseerAppModuleURL + '?hack-controls');
}

async function openFixture(page) {
  await page.goto(FIXTURE_URL);
  await mountOverseerFixture(page);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
}

async function emitCoordination(page, revision, activeTerminalId) {
  await page.evaluate(({ nextRevision, nextTerminalID }) => {
    __desktopFixture.emit('coordination-state', {
      revision: nextRevision,
      playerConfig: {
        name: 'Игроки теста',
        filePath: '/private/tmp/fallout-overseer-hack-players.json',
        revision: 1,
      },
      roster: [],
      sessions: [],
      broadcast: { id: 'hack-controls', activeTerminalId: nextTerminalID },
    });
  }, { nextRevision: revision, nextTerminalID: activeTerminalId });
}

async function emitHack(page, revision, snapshot) {
  await page.evaluate(({ nextRevision, nextSnapshot }) => {
    __desktopFixture.emit('hack-state', { ...nextSnapshot, revision: nextRevision });
  }, { nextRevision: revision, nextSnapshot: snapshot });
}

async function desktopCallCount(page, method) {
  return page.evaluate(name => __desktopFixture.calls.filter(call => call.method === name).length, method);
}

test.beforeEach(async ({ page }) => {
  const reset = await page.request.post(`${FIXTURE_URL}/reset`);
  expect(reset.ok()).toBe(true);
  await openFixture(page);
});

test('Overseer hacking controls preserve authoritative revisions and cleanup', async ({ page }) => {
  await emitCoordination(page, 20, 'terminal-stateful');
  await page.locator('#hackLevelSelect').selectOption('2');
  await page.locator('#btnApplySettings').click();
  await expect(page.locator('#coordinationStatus')).toHaveText('АКТИВНЫЙ ТЕРМИНАЛ ОБНОВЛЁН');

  await emitHack(page, 10, {
    attemptsLeft: 3,
    attemptsMax: 4,
    failed: false,
    solved: false,
  });
  const panel = page.locator('#hackStatus');
  await expect(panel).toBeVisible();
  await expect(panel).toHaveAttribute('data-hack-revision', '10');
  await expect(page.locator('#hackStatusLine')).toHaveText('ВЗЛОМ: осталось попыток 3/4');

  await emitHack(page, 9, {
    attemptsLeft: 0,
    attemptsMax: 4,
    failed: true,
    solved: false,
  });
  await expect(panel).toHaveAttribute('data-hack-revision', '10');
  await expect(page.locator('#btnHackSuccess')).toBeEnabled();

  await page.evaluate(() => {
    document.querySelector('#btnHackSuccess')?.click();
    document.querySelector('#btnHackSuccess')?.click();
  });
  await expect.poll(() => desktopCallCount(page, 'ForceHackSuccess')).toBe(1);
  await expect(page.locator('#hackStatusLine')).toHaveText('ВЗЛОМ: осталось попыток 3/4');

  await emitHack(page, 11, {
    attemptsLeft: 3,
    attemptsMax: 4,
    failed: false,
    solved: true,
  });
  await expect(page.locator('#hackStatusLine')).toHaveText('ВЗЛОМ: ПРОЙДЕН');
  await expect(page.locator('#btnHackSuccess')).toBeDisabled();

  await emitHack(page, 12, {
    attemptsLeft: 0,
    attemptsMax: 4,
    failed: true,
    solved: false,
  });
  const resetButton = page.locator('#btnResetFailedHack');
  await expect(resetButton).toBeVisible();
  await page.evaluate(() => {
    document.querySelector('#btnResetFailedHack')?.click();
    document.querySelector('#btnResetFailedHack')?.click();
  });
  await expect.poll(() => desktopCallCount(page, 'ResetFailedHack')).toBe(1);
  await expect(page.locator('#hackStatusLine')).toHaveText('ВЗЛОМ: ЗАБЛОКИРОВАН');
  const resetRequest = await page.evaluate(() => {
    const call = __desktopFixture.calls.find(candidate => candidate.method === 'ResetFailedHack');
    return call?.args?.[0] ?? null;
  });
  expect(resetRequest).toMatchObject({
    hackLevel: 2,
    terminalId: 'terminal-stateful',
    terminalName: 'Терминал охраны',
  });
  expect(resetRequest.tree).toMatchObject({ id: 'root', type: 'folder' });

  await emitHack(page, 13, {
    attemptsLeft: 4,
    attemptsMax: 4,
    failed: false,
    solved: false,
  });
  await expect(page.locator('#hackStatusLine')).toHaveText('ВЗЛОМ: осталось попыток 4/4');

  const releasesBefore = await page.evaluate(() => __desktopFixture.releaseCount('hack-state'));
  await page.evaluate(() => {
    __overseerAppFixture.unmount();
    __overseerAppFixture.unmount();
    __desktopFixture.emit('hack-state', {
      attemptsLeft: 0,
      attemptsMax: 4,
      failed: true,
      revision: Number.MAX_SAFE_INTEGER,
      solved: false,
    });
  });
  await expect(page.locator('#overseerApp')).toBeEmpty();
  expect(await page.evaluate(() => __desktopFixture.releaseCount('hack-state'))).toBe(releasesBefore + 1);
});
