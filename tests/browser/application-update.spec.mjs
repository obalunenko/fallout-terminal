import { expect, test } from '@playwright/test';

const availableUpdate = Object.freeze({
  revision: 2,
  attemptId: 'attempt-discovery',
  state: 'available',
  installedVersion: '2.0.0',
  availableVersion: '2.1.0',
  releaseNotes: '## Версия 2.1.0\n\nИсправлено сохранение сессии.',
  bytesDownloaded: 0,
  downloadSize: 8192,
  failedStage: '',
  errorMessage: '',
  recoveryAction: '',
});

async function openUpdateFixture(page) {
  await page.goto('/__fixture/public-access-settings');
  await expect(page.locator('#mainLayout')).toBeVisible();
  await expect.poll(() => page.evaluate(() => __desktopFixture.timeline
    .some(entry => entry.method === 'event:on:application-update-status'))).toBe(true);
}

async function emitUpdate(page, snapshot) {
  await page.evaluate(value => __desktopFixture.emit('application-update-status', value), snapshot);
}

async function expectLocalControlsEnabled(page) {
  await expect(page.locator('#btnOpenPlayerConfig')).toBeEnabled();
  await expect(page.locator('#btnNewPlayerConfig')).toBeEnabled();
  await expect(page.locator('#btnManagePlayers')).toBeEnabled();
}

async function expectNonfatalUpdateFailure(page, {
  errorMessage,
  recoveryAction,
  stagePattern,
}) {
  await expect(page.locator('#mainLayout')).toBeVisible();
  await expect(page.locator('#applicationUpdateStatusPanel')).toHaveAttribute('data-state', 'failed');
  const alert = page.locator('#applicationUpdateError');
  await expect(alert).toBeVisible();
  await expect(alert).toHaveAttribute('role', 'alert');
  await expect(alert).toContainText(stagePattern);
  await expect(alert).toContainText(errorMessage);
  await expect(alert).toContainText(recoveryAction);
  await expect(page.locator('#applicationUpdateProgress')).not.toBeVisible();
  await expect(page.locator('#applicationUpdateDialog')).not.toBeVisible();
  await expect(page.locator('#applicationUpdateRestartDialog')).not.toBeVisible();
  await expectLocalControlsEnabled(page);
}

test.beforeEach(async ({ page }) => {
  await openUpdateFixture(page);
});

test('discovery stays nonblocking and presents one complete versioned offer', async ({ page }) => {
  await emitUpdate(page, {
    ...availableUpdate,
    revision: 1,
    state: 'checking',
    attemptId: 'attempt-discovery',
    availableVersion: '',
    releaseNotes: '',
    downloadSize: undefined,
  });

  await expect(page.locator('#applicationUpdateStatusPanel')).toBeVisible();
  await expect(page.locator('#applicationUpdateStatus')).toContainText(/провер|обнов/i);
  await expect(page.locator('#btnOpenPlayerConfig')).toBeEnabled();
  await expect(page.locator('#btnNewPlayerConfig')).toBeEnabled();

  await emitUpdate(page, availableUpdate);

  const dialog = page.locator('#applicationUpdateDialog');
  await expect(dialog).toBeVisible();
  await expect(page.locator('#applicationUpdateInstalledVersion')).toHaveText('2.0.0');
  await expect(page.locator('#applicationUpdateAvailableVersion')).toHaveText('2.1.0');
  await expect(page.locator('#applicationUpdateReleaseNotes')).toContainText('Исправлено сохранение сессии.');
  await expect(page.locator('#btnAcceptApplicationUpdate')).toBeEnabled();
  await expect(page.locator('#btnDeferApplicationUpdate')).toBeEnabled();

  // Replaying the same revision must not create a second prompt or move focus
  // away from the safe default action.
  await emitUpdate(page, availableUpdate);
  await expect(page.locator('#applicationUpdateDialog')).toHaveCount(1);
  await expect(page.locator('#btnDeferApplicationUpdate')).toBeFocused();
});

test('defer downloads nothing, suppresses the offer for this run, and keeps local controls enabled', async ({ page }) => {
  await emitUpdate(page, availableUpdate);
  await expect(page.locator('#applicationUpdateDialog')).toBeVisible();

  await page.locator('#btnDeferApplicationUpdate').click();

  await expect(page.locator('#applicationUpdateDialog')).not.toBeVisible();
  await expect(page.locator('#btnOpenPlayerConfig')).toBeEnabled();
  await expect(page.locator('#btnNewPlayerConfig')).toBeEnabled();

  const deferred = await page.evaluate(() => ({
    decisions: __desktopFixture.calls.filter(entry => entry.method === 'ResolveApplicationUpdateOffer'),
    downloads: __desktopFixture.applicationUpdateDownloadCount(),
  }));
  expect(deferred.decisions).toEqual([{
    method: 'ResolveApplicationUpdateOffer',
    args: [{ attemptId: 'attempt-discovery', decision: 'defer' }],
  }]);
  expect(deferred.downloads).toBe(0);

  await emitUpdate(page, availableUpdate);
  await expect(page.locator('#applicationUpdateDialog')).not.toBeVisible();
  expect(await page.evaluate(() => __desktopFixture.calls
    .filter(entry => entry.method === 'ResolveApplicationUpdateOffer').length)).toBe(1);
});

test('Escape chooses defer and restores focus to the prior local control', async ({ page }) => {
  const localControl = page.locator('#btnOpenPlayerConfig');
  await localControl.focus();
  await expect(localControl).toBeFocused();

  await emitUpdate(page, availableUpdate);
  await expect(page.locator('#applicationUpdateDialog')).toBeVisible();
  await expect(page.locator('#btnDeferApplicationUpdate')).toBeFocused();

  await page.keyboard.press('Escape');

  await expect(page.locator('#applicationUpdateDialog')).not.toBeVisible();
  await expect(localControl).toBeFocused();
  await expect.poll(() => page.evaluate(() => __desktopFixture.calls
    .filter(entry => entry.method === 'ResolveApplicationUpdateOffer').at(-1))).toEqual({
    method: 'ResolveApplicationUpdateOffer',
    args: [{ attemptId: 'attempt-discovery', decision: 'defer' }],
  });
});

test('accept is the sole download gate and ordered preparation stays nonmodal', async ({ page }) => {
  await emitUpdate(page, availableUpdate);
  await expect(page.locator('#applicationUpdateDialog')).toBeVisible();
  expect(await page.evaluate(() => __desktopFixture.applicationUpdateDownloadCount())).toBe(0);

  await page.locator('#btnAcceptApplicationUpdate').click();
  await expect(page.locator('#applicationUpdateDialog')).not.toBeVisible();
  await expect.poll(() => page.evaluate(() => __desktopFixture.applicationUpdateDownloadCount())).toBe(1);

  await emitUpdate(page, {
    ...availableUpdate,
    revision: 3,
    state: 'downloading',
    bytesDownloaded: 1024,
    downloadSize: 8192,
  });
  await expect(page.locator('#applicationUpdateStatusPanel')).toHaveAttribute('data-state', 'downloading');
  await expect(page.locator('#applicationUpdateProgress')).toBeVisible();
  await expect(page.locator('#applicationUpdateProgress')).toHaveJSProperty('max', 8192);
  await expect(page.locator('#applicationUpdateProgress')).toHaveJSProperty('value', 1024);
  await expectLocalControlsEnabled(page);

  await emitUpdate(page, {
    ...availableUpdate,
    revision: 4,
    state: 'downloading',
    bytesDownloaded: 6144,
    downloadSize: 8192,
  });
  await expect(page.locator('#applicationUpdateProgress')).toHaveJSProperty('value', 6144);

  await emitUpdate(page, {
    ...availableUpdate,
    revision: 5,
    state: 'verifying',
    bytesDownloaded: 8192,
    downloadSize: 8192,
  });
  await expect(page.locator('#applicationUpdateStatusPanel')).toHaveAttribute('data-state', 'verifying');
  await expect(page.locator('#applicationUpdateStatus')).toContainText(/провер/i);
  await expect(page.locator('#applicationUpdateProgress')).toBeVisible();
  await expectLocalControlsEnabled(page);

  await emitUpdate(page, {
    ...availableUpdate,
    revision: 6,
    state: 'staging',
    bytesDownloaded: 8192,
    downloadSize: 8192,
  });
  await expect(page.locator('#applicationUpdateStatusPanel')).toHaveAttribute('data-state', 'staging');
  await expect(page.locator('#applicationUpdateStatus')).toContainText(/подготов/i);
  await expect(page.locator('#applicationUpdateProgress')).toBeVisible();
  await expect(page.locator('#applicationUpdateDialog')).not.toBeVisible();
  await expect(page.locator('#applicationUpdateRestartDialog')).not.toBeVisible();
  await expectLocalControlsEnabled(page);
});

test('ready update survives postpone and can reopen the separate restart decision', async ({ page }) => {
  const ready = {
    ...availableUpdate,
    revision: 7,
    state: 'ready-to-restart',
    bytesDownloaded: 8192,
    downloadSize: 8192,
  };
  await emitUpdate(page, ready);

  const restartDialog = page.locator('#applicationUpdateRestartDialog');
  await expect(restartDialog).toBeVisible();
  await expect(page.locator('#btnPostponeApplicationUpdate')).toBeFocused();
  await expect(page.locator('#btnRestartApplicationUpdate')).toBeEnabled();
  await expect(page.locator('#btnPostponeApplicationUpdate')).toBeEnabled();

  await page.locator('#btnPostponeApplicationUpdate').click();
  await expect(restartDialog).not.toBeVisible();
  await expect(page.locator('#applicationUpdateStatusPanel')).toHaveAttribute('data-state', 'ready-to-restart');
  await expect(page.locator('#btnShowApplicationUpdate')).toBeVisible();
  await expect.poll(() => page.evaluate(() => __desktopFixture.calls
    .filter(entry => entry.method === 'ResolveApplicationUpdateRestart'))).toEqual([{
    method: 'ResolveApplicationUpdateRestart',
    args: [{ attemptId: 'attempt-discovery', decision: 'postpone' }],
  }]);

  await page.locator('#btnShowApplicationUpdate').click();
  await expect(restartDialog).toBeVisible();
  await expect(page.locator('#btnPostponeApplicationUpdate')).toBeFocused();
});

test('restart approval is handed off exactly once despite duplicate activation', async ({ page }) => {
  await emitUpdate(page, {
    ...availableUpdate,
    revision: 7,
    state: 'ready-to-restart',
    bytesDownloaded: 8192,
    downloadSize: 8192,
  });
  await expect(page.locator('#applicationUpdateRestartDialog')).toBeVisible();

  await page.locator('#btnRestartApplicationUpdate').evaluate((button) => {
    button.click();
    button.click();
  });

  await expect.poll(() => page.evaluate(() => __desktopFixture.calls
    .filter(entry => entry.method === 'ResolveApplicationUpdateRestart'))).toEqual([{
    method: 'ResolveApplicationUpdateRestart',
    args: [{ attemptId: 'attempt-discovery', decision: 'restart' }],
  }]);
  await expect(page.locator('#applicationUpdateRestartDialog')).not.toBeVisible();
  await expect(page.locator('#applicationUpdateStatusPanel')).toHaveAttribute('data-state', 'applying');
});

test('offline discovery failure is actionable and never becomes a startup failure', async ({ page }) => {
  const errorMessage = 'Сервис обновлений временно недоступен.';
  const recoveryAction = 'Продолжайте работу и повторите проверку при следующем запуске.';

  await emitUpdate(page, {
    ...availableUpdate,
    revision: 8,
    attemptId: 'attempt-offline',
    state: 'failed',
    availableVersion: '',
    releaseNotes: '',
    downloadSize: undefined,
    failedStage: 'check',
    errorMessage,
    recoveryAction,
  });

  await expectNonfatalUpdateFailure(page, {
    errorMessage,
    recoveryAction,
    stagePattern: /проверк.*обнов/i,
  });
  await expect(page.locator('#startStatus')).not.toContainText(/запуск не заверш[её]н/i);
  expect(await page.evaluate(() => __desktopFixture.applicationUpdateDownloadCount())).toBe(0);
});

test('interrupted accepted download retains the failure and current installation controls', async ({ page }) => {
  const errorMessage = 'Загрузка обновления была прервана.';
  const recoveryAction = 'Текущая версия не изменена. Повторите попытку при следующем запуске.';

  await emitUpdate(page, {
    ...availableUpdate,
    revision: 20,
    attemptId: 'attempt-interrupted-download',
  });
  await expect(page.locator('#applicationUpdateDialog')).toBeVisible();
  await page.evaluate(({ errorMessage: error, recoveryAction: recovery }) => {
    __desktopFixture.failNextApplicationUpdatePreparation({
      failedStage: 'download',
      errorMessage: error,
      recoveryAction: recovery,
    });
  }, { errorMessage, recoveryAction });

  await page.locator('#btnAcceptApplicationUpdate').click();

  await expect.poll(() => page.evaluate(() => __desktopFixture.applicationUpdateSnapshot())).toEqual(
    expect.objectContaining({
      revision: 21,
      state: 'failed',
      failedStage: 'download',
      errorMessage,
      recoveryAction,
    }),
  );
  expect(await page.evaluate(() => __desktopFixture.applicationUpdateDownloadCount())).toBe(1);
  await expectNonfatalUpdateFailure(page, {
    errorMessage,
    recoveryAction,
    stagePattern: /загруз/i,
  });
});

for (const failure of [
  {
    stage: 'verify',
    stagePattern: /проверк.*загруж/i,
    recoveryAction: 'Загруженный файл удалён. Повторите попытку при следующем запуске.',
  },
  {
    stage: 'stage',
    stagePattern: /подготов/i,
    recoveryAction: 'Текущая версия не изменена. Освободите место и повторите попытку позже.',
  },
  {
    stage: 'apply',
    stagePattern: /примен/i,
    recoveryAction: 'Рабочая версия восстановлена. Продолжайте работу и повторите попытку позже.',
  },
]) {
  test(`${failure.stage} failure identifies its stage and recovery action without blocking local work`, async ({ page }) => {
    const errorMessage = 'Операция обновления не завершена.';
    await emitUpdate(page, {
      ...availableUpdate,
      revision: 30,
      attemptId: `attempt-${failure.stage}-failure`,
      state: 'failed',
      bytesDownloaded: 8192,
      failedStage: failure.stage,
      errorMessage,
      recoveryAction: failure.recoveryAction,
    });

    await expectNonfatalUpdateFailure(page, {
      errorMessage,
      recoveryAction: failure.recoveryAction,
      stagePattern: failure.stagePattern,
    });
  });
}

test('stale update events cannot erase recovery status or reopen an obsolete offer', async ({ page }) => {
  const errorMessage = 'Подготовка обновления не завершена.';
  const recoveryAction = 'Текущая версия готова к работе. Повторите попытку позже.';
  const failed = {
    ...availableUpdate,
    revision: 42,
    attemptId: 'attempt-current-failure',
    state: 'failed',
    bytesDownloaded: 8192,
    failedStage: 'stage',
    errorMessage,
    recoveryAction,
  };
  await emitUpdate(page, failed);
  await expectNonfatalUpdateFailure(page, {
    errorMessage,
    recoveryAction,
    stagePattern: /подготов/i,
  });

  await emitUpdate(page, {
    ...availableUpdate,
    revision: 41,
    attemptId: 'attempt-obsolete-offer',
  });
  await emitUpdate(page, {
    ...availableUpdate,
    revision: 42,
    attemptId: 'attempt-obsolete-progress',
    state: 'downloading',
    bytesDownloaded: 2048,
  });

  await expectNonfatalUpdateFailure(page, {
    errorMessage,
    recoveryAction,
    stagePattern: /подготов/i,
  });
  await expect(page.locator('#btnShowApplicationUpdate')).not.toBeVisible();
  expect(await page.evaluate(() => __desktopFixture.calls
    .filter(entry => entry.method === 'ResolveApplicationUpdateOffer'))).toEqual([]);
});

for (const scenario of [
  { name: 'current', state: 'current', installedVersion: '2.1.0' },
  { name: 'development', state: 'disabled', installedVersion: 'development' },
]) {
  test(`${scenario.name} builds remain silent while local controls stay enabled`, async ({ page }) => {
    await emitUpdate(page, {
      revision: 1,
      state: scenario.state,
      installedVersion: scenario.installedVersion,
      bytesDownloaded: 0,
      failedStage: '',
    });

    await expect(page.locator('#applicationUpdateDialog')).not.toBeVisible();
    await expect(page.locator('#btnShowApplicationUpdate')).not.toBeVisible();
    await expect(page.locator('#btnOpenPlayerConfig')).toBeEnabled();
    await expect(page.locator('#btnNewPlayerConfig')).toBeEnabled();
    expect(await page.evaluate(() => __desktopFixture.calls
      .filter(entry => entry.method === 'ResolveApplicationUpdateOffer'))).toEqual([]);
  });
}
