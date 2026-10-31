import { expect, test } from '@playwright/test';

import { expectRussianMessage, openDeviceDialog, renderedError } from './helpers/facility-editors.mjs';
import { FIXTURE, facilityState, fixtureSession, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

// User Story 7: errors in Russian with a next step
// (specs/041-facility-editors-language/spec.md, contracts/overseer-ui.md «Errors»,
// contracts/private-desktop.md «Browser fixture hooks»).

const DETAILS = 'подробности для разработчика';
const DOOR = 'vault-door-main';
const DOOR_NAME = 'Гермодверь Убежища 76';

const MESSAGES = {
  playerConfigUnreadable: 'Не удалось прочитать файл игроков. Выберите его заново или создайте новый.',
  playerConfigGeneral: 'Не удалось выполнить действие с файлом игроков. Выберите файл заново или создайте новый.',
  sessionNotWritten: 'Не удалось записать файл сессии. Выберите папку, доступную для записи, и создайте сессию снова.',
  sessionGeneral: 'Не удалось выполнить действие с файлом сессии. Повторите попытку. Если ошибка повторится, сохраните журнал.',
  persistenceFailed: 'Не удалось записать файл сессии. Проверьте, что диск доступен и файл не защищён от записи, затем повторите.',
  conflict: 'Правка противоречит текущим данным локации. Проверьте связанные элементы и повторите.',
  publicAccess: 'Не удалось изменить публичный доступ. Повторите попытку. Если ошибка повторится, сохраните журнал.',
  update: 'Не удалось выполнить обновление приложения. Повторите попытку позже. Если ошибка повторится, сохраните журнал.',
};

function pageErrors(page) {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  return errors;
}

// The sink's text without its developer disclosure.
function textOutsideDetails(sink) {
  return sink.evaluate(element => {
    const copy = element.cloneNode(true);
    copy.querySelectorAll('.overseer-error-details').forEach(details => details.remove());
    return copy.textContent;
  });
}

// expectOverseerError checks one rendered error against SC-008: a Russian
// message of at least two sentences that ends with the next step, Latin text
// only inside the collapsed «подробности для разработчика», and the prefix
// the sink gives it.
async function expectOverseerError(sink, { area, key, message, original, prefix = '' }) {
  const rendered = await renderedError(sink);
  expect(rendered).toEqual({ area, key, message, original, summary: original ? DETAILS : '' });
  expectRussianMessage(rendered.message);
  expect(rendered.message.split(/(?<=\.)\s+/).filter(Boolean).length).toBeGreaterThanOrEqual(2);
  expectRussianMessage(await textOutsideDetails(sink));
  const details = sink.locator('.overseer-error-details');
  if (original) expect(await details.evaluate(element => element.open)).toBe(false);
  else await expect(details).toHaveCount(0);
  if (prefix) await expect(sink.locator('.overseer-error-prefix')).toHaveText(prefix);
  else await expect(sink.locator('.overseer-error-prefix')).toHaveCount(0);
  return rendered;
}

// openFixturePage loads the facility fixture Overseer on its start screen
// once the startup status settles, so a queued result is not overwritten.
async function openFixturePage(page) {
  await page.goto(`${FIXTURE}/overseer`);
  await expect.poll(() => page.evaluate(() => typeof window.__desktopFixture)).toBe('object');
  await expect(page.locator('#startStatus')).toHaveAttribute('data-state', 'ready-local');
}

async function queueResult(page, method, result) {
  await page.evaluate(([name, value]) => window.__desktopFixture.setNextTerminalActionResult(name, value), [method, result]);
}

async function desktopCalls(page, method) {
  return page.evaluate(name => window.__desktopFixture.calls.filter(call => call.method === name).length, method);
}

// answerNextSave answers the next SaveAuthoring request with a rejection of
// failure and issue, as the session store rejects one, and saves nothing.
async function answerNextSave(page, request, failure, issue) {
  await page.route(`**${FIXTURE}/save-authoring`, async intercepted => {
    const { correlationId } = intercepted.request().postDataJSON();
    const state = await facilityState(request);
    await intercepted.fulfill({ json: {
      ok: false, changed: false, correlationId, failure, issues: [{ code: failure, ...issue }],
      sessionRevision: state.sessionRevision, authoringRevision: state.authoringRevision,
      previousFacilityRevision: state.facility.revision, resultingFacilityRevision: state.facility.revision,
      session: await fixtureSession(request),
    } });
  }, { times: 1 });
}

test('an unreadable player configuration reads the FR-040 message on the terminals tab (AS1, FR-039, FR-040, SC-008)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await seedDemo(request);
  await openFixturePage(page);
  // The demo references its player configuration, so opening it loads that file.
  await queueResult(page, 'LoadReferencedPlayerConfig', { ok: false, error: 'could not read the player config' });
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ', exact: true }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
  await page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true }).click();
  await expect.poll(() => desktopCalls(page, 'LoadReferencedPlayerConfig')).toBe(1);

  const unreadable = {
    area: 'player-config',
    key: 'could not read the player config',
    message: MESSAGES.playerConfigUnreadable,
    original: 'could not read the player config',
  };
  await expectOverseerError(page.locator('#playerConfigError'), unreadable);
  // The coordination panel mirrors the same message.
  await expectOverseerError(page.locator('#coordinationError'), unreadable);
  // The rendered text hides the original until the disclosure is expanded.
  expect(await page.locator('#playerConfigError').innerText()).not.toContain('could not read');

  // The original text stays one click away for developers.
  const details = page.locator('#playerConfigError .overseer-error-details');
  await expect(details.locator('.overseer-error-original')).toBeHidden();
  await details.locator('summary').click();
  await expect(details.locator('.overseer-error-original')).toBeVisible();
  await expect(details.locator('.overseer-error-original')).toHaveText('could not read the player config');

  // A wrapped Go error has no catalog line, so the general message shows it.
  const raw = 'open /Users/fixture/players.json: permission denied';
  await queueResult(page, 'OpenPlayerConfig', { ok: false, error: raw });
  await page.locator('#btnOpenPlayerConfig').click();
  await expectOverseerError(page.locator('#playerConfigError'), {
    area: 'player-config', key: 'general', message: MESSAGES.playerConfigGeneral, original: raw,
  });
  expect(errors).toEqual([]);
});

test('a session file that cannot be written names the file problem and a next step (AS2, FR-039, SC-008)', async ({ page }) => {
  const errors = pageErrors(page);
  await openFixturePage(page);
  const status = page.locator('#startStatus');

  await queueResult(page, 'NewSession', { ok: false, error: 'could not write the new session' });
  await page.getByRole('button', { name: 'НОВАЯ СЕССИЯ', exact: true }).click();
  await expectOverseerError(status, {
    area: 'session',
    key: 'could not write the new session',
    message: MESSAGES.sessionNotWritten,
    original: 'could not write the new session',
  });
  await expect(page.locator('#startScreen')).toBeVisible();
  await expect(page.locator('#mainLayout')).toBeHidden();

  const raw = 'write /private/tmp/campaign.json: no space left on device';
  await queueResult(page, 'NewSession', { ok: false, error: raw });
  await page.getByRole('button', { name: 'НОВАЯ СЕССИЯ', exact: true }).click();
  await expectOverseerError(status, {
    area: 'session', key: 'general', message: MESSAGES.sessionGeneral, original: raw,
  });
  expect(await desktopCalls(page, 'NewSession')).toBe(2);
  expect(errors).toEqual([]);
});

test('a facility save that cannot be written keeps the dialog draft and names the next step (FR-039, edge case, SC-008)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await seedDemo(request);
  await openOverseer(page);
  const dialog = await openDeviceDialog(page, DOOR);
  const typed = 'Гермодверь 76 · ремонт';
  await dialog.name.fill(typed);

  const failed = await request.post(`${FIXTURE}/next-operation`, { data: { failure: 'persistence-failed' } });
  expect(failed.status()).toBe(204);
  await dialog.save.click();

  // The dialog comes back with what the author typed and the failure in its sink.
  await expect(dialog.dialog).toBeVisible();
  await expect(dialog.name).toHaveValue(typed);
  await expectOverseerError(dialog.error, {
    area: 'facility',
    key: 'persistence-failed',
    message: MESSAGES.persistenceFailed,
    original: 'failure: persistence-failed',
  });
  // The shared status keeps its 039 form and has no disclosure.
  const saveStatus = page.locator('#saveStatus');
  await expect(saveStatus).toHaveAttribute('data-state', 'error');
  await expect(saveStatus).toContainText(`ОШИБКА СОХРАНЕНИЯ: ${MESSAGES.persistenceFailed}`);
  await expect(saveStatus.locator('.overseer-error-details')).toHaveCount(0);
  const saved = await fixtureSession(request);
  expect(saved.facility.devices.find(device => device.id === DOOR).name).toBe(DOOR_NAME);

  // The next step works: pressing «ГОТОВО» again saves the same draft.
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  await expect(saveStatus).toHaveAttribute('data-state', 'saved');
  await expect.poll(async () => (await fixtureSession(request)).facility.devices.find(device => device.id === DOOR).name)
    .toBe(typed);
  expect(errors).toEqual([]);
});

test('a facility validation problem names the entity in Russian and keeps identifiers under the disclosure (FR-038, SC-008)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await seedDemo(request);
  await openOverseer(page);
  const dialog = await openDeviceDialog(page, DOOR);
  const typed = 'Гермодверь 76';
  await dialog.name.fill(typed);

  const cases = [
    {
      issue: { entityKind: 'device', entityId: 'power-grid-main' },
      phrase: 'устройство «Резервная сеть Убежища 76»',
      original: 'failure: conflict · device power-grid-main',
    },
    {
      issue: { entityKind: 'device-state', entityId: 'sealed', referenceKind: 'owner', referenceId: DOOR },
      phrase: `состояние «Заперта» устройства «${DOOR_NAME}»`,
      original: `failure: conflict · device-state sealed · owner ${DOOR}`,
    },
    {
      issue: { entityKind: 'device-transition', entityId: 'open', referenceKind: 'owner', referenceId: DOOR },
      phrase: `переход «Открыть гермодверь» устройства «${DOOR_NAME}»`,
      original: `failure: conflict · device-transition open · owner ${DOOR}`,
    },
    {
      issue: { entityKind: 'condition', entityId: 'door-offline' },
      phrase: 'неисправность «Контроллер гермодвери не отвечает»',
      original: 'failure: conflict · condition door-offline',
    },
    {
      issue: { entityKind: 'recovery-program', entityId: 'network-recovery-76' },
      phrase: 'программа восстановления «СЕТЬ-76»',
      original: 'failure: conflict · recovery-program network-recovery-76',
    },
  ];
  for (const { issue, phrase, original } of cases) {
    await answerNextSave(page, request, 'conflict', issue);
    await dialog.save.click();
    await expect(dialog.dialog).toBeVisible();
    await expect(dialog.name).toHaveValue(typed);
    const rendered = await expectOverseerError(dialog.error, {
      area: 'facility', key: 'conflict', message: `${MESSAGES.conflict} Затронуто: ${phrase}.`, original,
    });
    // Internal kinds and identifiers never reach the message (FR-038).
    expect(rendered.message).not.toContain(issue.entityKind);
    expect(rendered.message).not.toContain(issue.entityId);
  }

  // A kind that is no authored entity adds no phrase.
  await answerNextSave(page, request, 'conflict', { entityKind: 'session' });
  await dialog.save.click();
  await expect(dialog.dialog).toBeVisible();
  await expectOverseerError(dialog.error, {
    area: 'facility', key: 'conflict', message: MESSAGES.conflict, original: 'failure: conflict · session',
  });
  const saved = await fixtureSession(request);
  expect(saved.facility.devices.find(device => device.id === DOOR).name).toBe(DOOR_NAME);
  expect(errors).toEqual([]);
});

test('a public access failure shows the general message with the original only under the disclosure (AS3, FR-041, SC-008)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await page.goto('/__fixture/public-access-settings');
  await expect(page.locator('#publicAccessSection')).toBeVisible();

  for (const [kind, original] of [
    ['invalid-token', 'The provider rejected the account credential.'],
    ['domain-conflict', 'The reserved domain is unavailable for this account.'],
    ['keychain-unavailable', 'The secure credential store is unavailable; local access remains available.'],
  ]) {
    const response = await request.post(`/__fixture/public-access/failure/${kind}`);
    expect(response.status(), kind).toBe(200);
    const snapshot = await response.json();
    await page.evaluate(value => window.__desktopFixture.emit('public-access-status', value), snapshot);
    await expect(page.locator('#publicAccessStatus')).toHaveText('ОШИБКА');
    await expect(page.locator('#publicAccessError .overseer-error-original')).toHaveText(original);
    await expectOverseerError(page.locator('#publicAccessError'), {
      area: 'public-access', key: 'general', message: MESSAGES.publicAccess, original,
      prefix: 'ЛОКАЛЬНЫЙ РЕЖИМ ПРОДОЛЖАЕТ РАБОТАТЬ',
    });
  }

  // A failure inside the open settings dialog keeps what the author typed.
  await page.locator('#btnOpenPublicAccessSettings').click();
  const settings = page.locator('#publicAccessSettingsDialog');
  await expect(settings).toBeVisible();
  await page.getByLabel('Зарезервированный домен').fill('vault-76.example');
  await page.evaluate(() => window.__desktopFixture.deferSavePublicAccess());
  await page.getByRole('button', { name: 'СОХРАНИТЬ НАСТРОЙКИ' }).click();
  const raw = 'Public access settings could not be saved: preferences file is read-only';
  await page.evaluate(error => window.__desktopFixture.resolveSavePublicAccess({
    ok: false,
    error,
    snapshot: {
      preferences: { version: 1, enabledPreference: false, reservedDomain: '', username: 'players', revision: 0 },
      providerTokenPresence: 'present', playerPasswordPresence: 'present',
      status: { state: 'disabled', generation: 99, settingsRevision: 0 },
    },
  }), raw);
  await expect(settings).toBeVisible();
  await expect(page.getByLabel('Зарезервированный домен')).toHaveValue('vault-76.example');
  await expectOverseerError(page.locator('#publicAccessSettingsError'), {
    area: 'public-access', key: 'general', message: MESSAGES.publicAccess, original: raw,
  });
  expect(errors).toEqual([]);
});

test('an update failure shows the general message with the original only under the disclosure (AS3, FR-041, SC-008)', async ({ page }) => {
  const errors = pageErrors(page);
  await page.goto('/__fixture/public-access-settings');
  await expect(page.locator('#mainLayout')).toBeVisible();
  await expect.poll(() => page.evaluate(() => window.__desktopFixture.timeline
    .some(entry => entry.method === 'event:on:application-update-status'))).toBe(true);
  const offer = {
    revision: 2,
    attemptId: 'attempt-offered',
    state: 'available',
    installedVersion: '2.0.0',
    availableVersion: '2.1.0',
    releaseNotes: '## Версия 2.1.0\n\nДобавлена группировка терминалов.',
    bytesDownloaded: 0,
    downloadSize: 8192,
    failedStage: '',
    errorMessage: '',
    recoveryAction: '',
  };
  await page.evaluate(value => window.__desktopFixture.emit('application-update-status', value), offer);
  await expect(page.locator('#applicationUpdateDialog')).toBeVisible();

  // A newer attempt replaces the offer before the Overseer decides, so the
  // decision reaches a superseded attempt (the stale-offer path).
  await page.evaluate(value => window.__desktopFixture.resolveApplicationUpdate(value),
    { ...offer, revision: 3, attemptId: 'attempt-superseding' });
  await page.locator('#btnDeferApplicationUpdate').click();
  const sink = page.locator('#applicationUpdateError');
  await expectOverseerError(sink, {
    area: 'update', key: 'general', message: MESSAGES.update,
    original: 'Application update offer is no longer available',
  });

  // A failed stage names the stage first; the Go texts go under the disclosure.
  await page.evaluate(value => window.__desktopFixture.emit('application-update-status', value), {
    ...offer,
    revision: 10,
    attemptId: 'attempt-failed-download',
    state: 'failed',
    failedStage: 'download',
    errorMessage: 'Download interrupted: connection reset by peer',
    recoveryAction: 'The current version is unchanged. Try again later.',
  });
  await expect(page.locator('#applicationUpdateStatusPanel')).toHaveAttribute('data-state', 'failed');
  await expectOverseerError(sink, {
    area: 'update', key: 'general', message: MESSAGES.update,
    original: 'Download interrupted: connection reset by peer\nThe current version is unchanged. Try again later.',
    prefix: 'ЭТАП: ЗАГРУЗКА ОБНОВЛЕНИЯ.',
  });
  expect(errors).toEqual([]);
});
