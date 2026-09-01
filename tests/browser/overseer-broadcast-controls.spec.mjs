import { expect, test } from '@playwright/test';

const overseerAppModuleURL = 'http://127.0.0.1:34121/@fs' + new URL('./fixtures/overseer-app.ts', import.meta.url).pathname;

const FIXTURE_URL = '/__fixture/state-changing-command-authoring';

test.use({ bypassCSP: true });

async function mountOverseerFixture(page) {
  await page.evaluate(url => import(url), overseerAppModuleURL + '?broadcast-controls');
}

async function openFixture(page) {
  await page.goto(FIXTURE_URL);
  await mountOverseerFixture(page);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
}

async function emitCoordination(page, revision, { activeTerminalId = null, broadcast = true } = {}) {
  await page.evaluate(({ nextRevision, nextActiveTerminalID, hasBroadcast }) => {
    __desktopFixture.emit('coordination-state', {
      revision: nextRevision,
      playerConfig: {
        name: 'Игроки теста',
        filePath: '/private/tmp/fallout-overseer-broadcast-players.json',
        revision: 1,
      },
      roster: [],
      sessions: [
        { id: 'connected', fallbackName: 'Подключён', connected: true },
        { id: 'offline', fallbackName: 'Отключён', connected: false },
      ],
      broadcast: hasBroadcast ? {
        id: 'broadcast-controls',
        activeTerminalId: nextActiveTerminalID,
      } : null,
    });
  }, { nextRevision: revision, nextActiveTerminalID: activeTerminalId, hasBroadcast: broadcast });
}

async function desktopCallCount(page, method) {
  return page.evaluate(name => __desktopFixture.calls.filter(call => call.method === name).length, method);
}

test.beforeEach(async ({ page }) => {
  const reset = await page.request.post(`${FIXTURE_URL}/reset`);
  expect(reset.ok()).toBe(true);
  await openFixture(page);
});

test('broadcast controls preserve revision ordering and cleanup', async ({ page }) => {
  await emitCoordination(page, 20, { broadcast: false });
  const panel = page.locator('#coordinationPanel');
  await expect(panel).toHaveAttribute('data-coordination-revision', '20');
  await expect(page.locator('#broadcastSummary')).toHaveText('ТРАНСЛЯЦИЯ НЕ ЗАПУЩЕНА');
  await expect(page.locator('#btnStartBroadcast')).toBeEnabled();
  await expect(page.locator('#activeLogicalSessionCount')).toHaveText('1');

  await emitCoordination(page, 19, { activeTerminalId: 'stale-terminal' });
  await expect(panel).toHaveAttribute('data-coordination-revision', '20');
  await expect(page.locator('#broadcastSummary')).toHaveText('ТРАНСЛЯЦИЯ НЕ ЗАПУЩЕНА');

  await emitCoordination(page, 21, { activeTerminalId: 'terminal-stateful' });
  await expect(panel).toHaveAttribute('data-coordination-revision', '21');
  await expect(page.locator('#broadcastSummary')).toContainText('ТЕРМИНАЛ terminal-stateful');
  await expect(page.locator('#btnStartBroadcast')).toBeDisabled();
  await expect(page.locator('#btnEndBroadcast')).toBeVisible();
  await expect(page.locator('#btnStopBroadcast')).toBeVisible();

  await page.locator('#btnEndBroadcast').click();
  const endDialog = page.getByRole('dialog', { name: 'ЗАВЕРШИТЬ ТРАНСЛЯЦИЮ?' });
  await expect(endDialog).toBeVisible();
  await endDialog.getByRole('button', { name: 'ОТМЕНА' }).click();
  await expect(page.locator('#btnEndBroadcast')).toBeFocused();

  await page.locator('#btnStopBroadcast').click();
  const stopDialog = page.getByRole('dialog', { name: 'СНЯТЬ ТЕРМИНАЛ С ЭФИРА?' });
  await expect(stopDialog).toBeVisible();
  await stopDialog.getByRole('button', { name: 'ОТМЕНА' }).click();
  await expect(page.locator('#btnStopBroadcast')).toBeFocused();

  await page.evaluate(() => {
    __overseerAppFixture.unmount();
    __overseerAppFixture.unmount();
    __overseerAppFixture.controller.publish({
      coordination: {
        revision: Number.MAX_SAFE_INTEGER,
        playerConfig: {},
        roster: [],
        sessions: [],
        broadcast: null,
      },
      error: '',
      kind: 'coordination-state',
      pending: false,
      status: 'LATE',
    });
  });
  await expect(page.locator('#overseerApp')).toBeEmpty();
});

test('take-off-air and end-broadcast confirmations resolve exactly once', async ({ page }) => {
  await emitCoordination(page, 30, { activeTerminalId: 'terminal-stateful' });
  const endTrigger = page.locator('#btnEndBroadcast');
  const stopTrigger = page.locator('#btnStopBroadcast');
  const endDialog = page.getByRole('dialog', { name: 'ЗАВЕРШИТЬ ТРАНСЛЯЦИЮ?' });
  const stopDialog = page.getByRole('dialog', { name: 'СНЯТЬ ТЕРМИНАЛ С ЭФИРА?' });

  await endTrigger.click();
  await expect(endDialog).toBeVisible();
  await expect(page.locator('#btnCancelEndBroadcast')).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(endDialog).toBeHidden();
  await expect(endTrigger).toBeFocused();
  await expect.poll(() => desktopCallCount(page, 'EndBroadcast')).toBe(0);

  await page.evaluate(() => __desktopFixture.deferTerminalAction('RequestTerminalClear'));
  await stopTrigger.click();
  await expect(stopDialog).toBeVisible();
  await page.locator('#btnConfirmTakeOffAir').click();
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalClear')).toBe(1);
  await page.locator('#btnConfirmTakeOffAir').click({ force: true });
  await expect.poll(() => desktopCallCount(page, 'RequestTerminalClear')).toBe(1);

  await emitCoordination(page, 31, { activeTerminalId: 'terminal-stateful' });
  await page.evaluate(() => __desktopFixture.resolveTerminalAction('RequestTerminalClear', {
    ok: true,
    status: 'cleared',
    state: {
      revision: 31,
      playerConfig: { name: 'Игроки теста', revision: 1 },
      roster: [],
      sessions: [],
      broadcast: { id: 'broadcast-controls', activeTerminalId: null },
    },
  }));
  await expect(stopDialog).toBeVisible();
  await expect(page.locator('#btnConfirmTakeOffAir')).toBeEnabled();
  await page.keyboard.press('Escape');
  await expect(stopDialog).toBeHidden();
  await expect(stopTrigger).toBeFocused();

  await endTrigger.click();
  await expect(endDialog).toBeVisible();
  await page.evaluate(() => __overseerAppFixture.unmount());
  await expect(page.locator('#endBroadcastDialog')).toHaveCount(0);
  await expect(page.locator('#takeOffAirDialog')).toHaveCount(0);
});
