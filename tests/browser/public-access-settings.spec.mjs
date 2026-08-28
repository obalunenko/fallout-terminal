import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.goto('/__fixture/public-access-settings');
  await expect(page.locator('#publicAccessSection')).toBeVisible();
});

async function openPublicAccessSettings(page) {
  await page.locator('#btnOpenPublicAccessSettings').click();
  const dialog = page.locator('#publicAccessSettingsDialog');
  await expect(dialog).toBeVisible();
  return dialog;
}

async function openConfiguredProviderTokenDialog(page) {
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: false, reservedDomain: '', username: 'players', revision: 1 },
    providerTokenPresence: 'present',
    playerPasswordPresence: 'present',
    status: { state: 'disabled', generation: 1, settingsRevision: 1 },
  }));
  await openPublicAccessSettings(page);
  await page.locator('#btnOpenPublicAccessProviderToken').click();
  const dialog = page.locator('#publicAccessProviderTokenDialog');
  await expect(dialog).toBeVisible();
  return dialog;
}

async function openPlayerCredentialsDialog(page) {
  await page.locator('#btnOpenPublicAccessPlayerCredentials').click();
  const dialog = page.locator('#publicAccessPlayerCredentialsDialog');
  await expect(dialog).toBeVisible();
  return dialog;
}

test('settings form is labelled, keyboard reachable, and defaults without revealing secrets', async ({ page }) => {
  const dialog = await openPublicAccessSettings(page);
  await expect(page.locator('#btnClosePublicAccessSettings')).toBeFocused();
  const guide = page.locator('#publicAccessGuide');
  await expect(guide).not.toHaveAttribute('open', '');
  await guide.getByText('КАК НАСТРОИТЬ ЧЕРЕЗ NGROK').click();
  await expect(guide).toHaveAttribute('open', '');
  await expect(guide).toContainText('Сохраните настройки');
  await expect(guide).toContainText('Basic Auth');

  await expect(page.getByLabel('Зарезервированный домен')).toHaveValue('');
  await expect(page.locator('#publicAccessUsernameSummary')).toHaveText('players');
  await expect(page.locator('#publicAccessProviderToken')).toHaveAttribute('type', 'password');
  await expect(page.locator('#publicAccessPasswordMask')).toBeHidden();
  await expect(page.getByRole('button', { name: /показать|reveal/i })).toHaveCount(0);
  await expect(page.locator('#publicAccessProviderPresence')).toHaveText(/не сохранен|недоступен/i);
  await expect(page.locator('#publicAccessPasswordPresence')).toHaveText(/не сохранен|недоступен/i);
  const groupOrder = await page.evaluate(() => [
    'publicAccessConnectionGroup', 'publicAccessPlayerLoginGroup',
    'btnCancelPublicAccessSettings',
  ].map(id => document.getElementById(id).getBoundingClientRect().top));
  expect(groupOrder).toEqual([...groupOrder].sort((left, right) => left - right));
  await expect(page.locator('.public-access-settings-group')).toHaveCount(2);
  await expect(page.locator('#publicAccessBehaviorGroup')).toHaveCount(0);
  await expect(page.getByText('Включать публичный доступ при запуске приложения')).toHaveCount(0);
  await expect(page.locator('#publicAccessProviderConfigured')).toBeHidden();

  const credentialsDialog = await openPlayerCredentialsDialog(page);
  await expect(page.getByLabel('Логин игроков')).toBeFocused();
  await expect(page.getByLabel('Логин игроков')).toHaveValue('players');
  await expect(page.getByLabel('Новый пароль игроков')).toHaveAttribute('type', 'password');
  await expect(page.locator('#btnDeletePublicAccessPlayerCredentials')).toBeHidden();
  await page.keyboard.press('Escape');
  await expect(credentialsDialog).toBeHidden();
  await expect(page.locator('#btnOpenPublicAccessPlayerCredentials')).toBeFocused();

  await page.keyboard.press('Tab');
  await expect(page.locator(':focus')).not.toHaveAttribute('type', 'hidden');

  await page.locator('#btnClosePublicAccessSettings').click();
  await expect(dialog).toBeHidden();
  await expect(page.locator('#btnOpenPublicAccessSettings')).toBeFocused();
});

test('first enable opens required setup without requesting tunnel startup', async ({ page }) => {
  const callsBefore = await page.evaluate(() => __desktopFixture.calls.length);
  await page.locator('#btnStartPublicAccess').click();
  await expect(page.locator('#publicAccessSettingsDialog')).toBeVisible();
  await expect(page.locator('#publicAccessSetupRequired')).toBeVisible();
  await expect(page.locator('#publicAccessGuide')).toHaveAttribute('open', '');
  const startCalls = await page.evaluate(start => __desktopFixture.calls.slice(start)
    .filter(call => call.method === 'StartPublicAccess'), callsBefore);
  expect(startCalls).toEqual([]);
});

test('development override prefill is presence-only and does not save or start implicitly', async ({ page }) => {
  const callsBefore = await page.evaluate(() => __desktopFixture.calls.length);
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: false, reservedDomain: 'override.example', username: 'override-players', revision: 0 },
    providerTokenPresence: 'present',
    playerPasswordPresence: 'present',
    status: { state: 'disabled', generation: 0, settingsRevision: 0 },
  }));
  await openPublicAccessSettings(page);

  await expect(page.getByLabel('Зарезервированный домен')).toHaveValue('override.example');
  await expect(page.locator('#publicAccessUsernameSummary')).toHaveText('override-players');
  await expect(page.locator('#publicAccessProviderSetup')).toBeHidden();
  await expect(page.locator('#publicAccessProviderConfigured')).toBeVisible();
  await expect(page.locator('#publicAccessPasswordMask')).toHaveText('*****');
  await expect(page.locator('#publicAccessProviderPresence')).toHaveText(/настроен/i);
  await expect(page.locator('#publicAccessPasswordPresence')).toBeHidden();
  const implicitCalls = await page.evaluate(start => __desktopFixture.calls.slice(start)
    .filter(call => call.method === 'SavePublicAccessSettings' || call.method === 'StartPublicAccess'), callsBefore);
  expect(implicitCalls).toEqual([]);

  await openPlayerCredentialsDialog(page);
  await expect(page.getByLabel('Логин игроков')).toHaveValue('override-players');
  await expect(page.getByLabel('Новый пароль игроков')).toHaveValue('');
  await page.getByRole('button', { name: 'СГЕНЕРИРОВАТЬ' }).click();
  const saveCallsAfterGenerate = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'SavePublicAccessSettings'));
  expect(saveCallsAfterGenerate).toEqual([]);
  await page.reload();
  await openPublicAccessSettings(page);
  await expect(page.getByLabel('Зарезервированный домен')).toHaveValue('');
  await expect(page.locator('#publicAccessUsernameSummary')).toHaveText('players');

  await page.getByLabel('Зарезервированный домен').fill('override.example');
  await page.getByRole('button', { name: 'СОХРАНИТЬ НАСТРОЙКИ' }).click();
  const saved = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'SavePublicAccessSettings').at(-1));
  expect(saved.args[0]).toMatchObject({
    reservedDomain: 'override.example', username: 'players',
    replacementProviderToken: '', replacementPlayerPassword: '',
  });
});

test('configured token is replaced in a focused dialog without secret readback', async ({ page }) => {
  const dialog = await openConfiguredProviderTokenDialog(page);
  await expect(page.locator('#publicAccessProviderSetup')).toBeHidden();
  await expect(page.locator('#publicAccessProviderConfigured')).toBeVisible();
  await expect(page.locator('#publicAccessProviderPresence')).toHaveText('НАСТРОЕН');
  await expect(dialog).toContainText('Сохранённый токен нельзя посмотреть');
  await expect(page.locator('#publicAccessReplacementProviderToken')).toBeFocused();

  await page.locator('#publicAccessReplacementProviderToken').fill('synthetic-provider-replacement');
  await page.getByRole('button', { name: 'СОХРАНИТЬ ТОКЕН' }).click();

  await expect(dialog).toBeHidden();
  await expect(page.locator('#publicAccessProviderConfigured')).toBeVisible();
  await expect(page.locator('#publicAccessProviderPresence')).toHaveText('НАСТРОЕН');
  await expect(page.locator('#btnOpenPublicAccessProviderToken')).toBeFocused();
  await expect(page.locator('body')).not.toContainText('synthetic-provider-replacement');
  const saved = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'SavePublicAccessSettings').at(-1));
  expect(saved.args[0]).toMatchObject({ replacementProviderToken: '', deleteProviderToken: false });
});

test('token dialog dismissal clears transient input and submits no mutation', async ({ page }) => {
  const dialog = await openConfiguredProviderTokenDialog(page);
  const callsBefore = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'SavePublicAccessSettings').length);
  await page.locator('#publicAccessReplacementProviderToken').fill('synthetic-dismissed-provider-token');
  await page.keyboard.press('Escape');

  await expect(dialog).toBeHidden();
  await expect(page.locator('#publicAccessReplacementProviderToken')).toHaveValue('');
  await expect(page.locator('#btnOpenPublicAccessProviderToken')).toBeFocused();
  const callsAfter = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'SavePublicAccessSettings').length);
  expect(callsAfter).toBe(callsBefore);
  await expect(page.locator('body')).not.toContainText('synthetic-dismissed-provider-token');
});

test('configured token can be deleted without affecting the player password', async ({ page }) => {
  const dialog = await openConfiguredProviderTokenDialog(page);
  await page.getByRole('button', { name: 'УДАЛИТЬ СОХРАНЁННЫЙ ТОКЕН' }).click();

  await expect(dialog).toBeHidden();
  await expect(page.locator('#publicAccessProviderSetup')).toBeVisible();
  await expect(page.locator('#publicAccessProviderConfigured')).toBeHidden();
  await expect(page.locator('#publicAccessProviderPresence')).toHaveText(/не сохранен/i);
  await expect(page.locator('#publicAccessPasswordMask')).toHaveText('*****');
  const saved = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'SavePublicAccessSettings').at(-1));
  expect(saved.args[0]).toMatchObject({ deleteProviderToken: true, deletePlayerPassword: false });
});

test('token replacement failure stays open, clears the secret, and preserves the settings draft', async ({ page }) => {
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: false, reservedDomain: 'before.example', username: 'players', revision: 3 },
    providerTokenPresence: 'present', playerPasswordPresence: 'present',
    status: { state: 'disabled', generation: 3, settingsRevision: 3 },
  }));
  await openPublicAccessSettings(page);
  await page.getByLabel('Зарезервированный домен').fill('draft.example');
  await page.evaluate(() => __desktopFixture.deferSavePublicAccess());
  await page.locator('#btnOpenPublicAccessProviderToken').click();
  await page.locator('#publicAccessReplacementProviderToken').fill('synthetic-failed-provider-token');
  await page.getByRole('button', { name: 'СОХРАНИТЬ ТОКЕН' }).click();
  await page.evaluate(() => __desktopFixture.resolveSavePublicAccess({
    ok: false,
    error: 'Keychain is unavailable; local access remains available.',
    snapshot: {
      preferences: { version: 1, enabledPreference: false, reservedDomain: 'before.example', username: 'players', revision: 3 },
      providerTokenPresence: 'present', playerPasswordPresence: 'present',
      status: { state: 'disabled', generation: 4, settingsRevision: 3 },
    },
  }));

  await expect(page.locator('#publicAccessProviderTokenDialog')).toBeVisible();
  await expect(page.locator('#publicAccessProviderTokenError')).toContainText('Keychain is unavailable');
  await expect(page.locator('#publicAccessReplacementProviderToken')).toHaveValue('');
  await expect(page.getByLabel('Зарезервированный домен')).toHaveValue('draft.example');
  await expect(page.locator('body')).not.toContainText('synthetic-failed-provider-token');
});

test('unknown token presence uses the safe initial-entry state', async ({ page }) => {
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: false, reservedDomain: '', username: 'players', revision: 2 },
    providerTokenPresence: 'unknown', playerPasswordPresence: 'present',
    status: { state: 'disabled', generation: 2, settingsRevision: 2 },
  }));
  await openPublicAccessSettings(page);

  await expect(page.locator('#publicAccessProviderSetup')).toBeVisible();
  await expect(page.locator('#publicAccessProviderConfigured')).toBeHidden();
  await expect(page.locator('#publicAccessProviderToken')).toBeVisible();
  await expect(page.locator('#publicAccessProviderPresence')).toHaveText('НЕДОСТУПЕН');
});

test('save replaces secrets without echo and clears transient fields', async ({ page }) => {
  const dialog = await openPublicAccessSettings(page);
  await page.locator('#publicAccessProviderToken').fill('synthetic-provider-input');
  await page.getByRole('button', { name: 'СОХРАНИТЬ НАСТРОЙКИ' }).click();

  await expect(dialog).toBeHidden();
  await expect(page.locator('#publicAccessProviderToken')).toHaveValue('');
  await expect(page.locator('#publicAccessProviderPresence')).toHaveText(/настроен/i);
  await expect(page.locator('body')).not.toContainText('synthetic-provider-input');

  await openPublicAccessSettings(page);
  const credentialsDialog = await openPlayerCredentialsDialog(page);
  await page.getByLabel('Новый пароль игроков').fill('synthetic-player-input');
  await page.getByRole('button', { name: 'СОХРАНИТЬ ДАННЫЕ' }).click();
  await expect(credentialsDialog).toBeHidden();
  await expect(page.getByLabel('Новый пароль игроков')).toHaveValue('');
  await expect(page.locator('#publicAccessPasswordMask')).toHaveText('*****');
  await expect(page.locator('#publicAccessPasswordPresence')).toBeHidden();
  await expect(page.locator('body')).not.toContainText('synthetic-player-input');
});

test('generated password is copied once, dismissed, and removed from DOM references', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await openPublicAccessSettings(page);
  await openPlayerCredentialsDialog(page);
  await page.getByRole('button', { name: 'СГЕНЕРИРОВАТЬ' }).click();
  const dialog = page.getByRole('dialog', { name: 'НОВЫЙ ПАРОЛЬ ИГРОКОВ' });
  await expect(dialog).toBeVisible();
  await expect(page.locator('#btnCopyGeneratedPassword')).toBeFocused();
  const generated = await page.locator('#generatedPasswordValue').textContent();
  expect(generated.length).toBeGreaterThanOrEqual(8);

  await page.locator('#btnCopyGeneratedPassword').click();
  await expect(dialog).toBeHidden();
  await expect(page.locator('#generatedPasswordValue')).toHaveText('');
  await expect(page.locator('#publicAccessPasswordMask')).toHaveText('*****');
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(generated);
  await expect(page.locator('#btnGeneratePlayerPassword')).toBeFocused();
});

test('legacy automatic-start preference is inert and new saves disable it', async ({ page }) => {
  const startCallsBefore = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'StartPublicAccess').length);
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: {
      version: 1, enabledPreference: true, reservedDomain: 'vault.example', username: 'wanderers', revision: 4,
    },
    providerTokenPresence: 'present',
    playerPasswordPresence: 'present',
    status: { state: 'disabled', generation: 4, settingsRevision: 4 },
  }));

  await expect(page.locator('#publicAccessStatus')).toHaveText('ОСТАНОВЛЕН');
  await expect(page.locator('#btnStartPublicAccess')).toBeVisible();
  const startCallsAfterLoad = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'StartPublicAccess').length);
  expect(startCallsAfterLoad).toBe(startCallsBefore);

  await openPublicAccessSettings(page);
  await page.getByLabel('Зарезервированный домен').fill('vault.example');
  await page.getByRole('button', { name: 'СОХРАНИТЬ НАСТРОЙКИ' }).click();
  const saved = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'SavePublicAccessSettings').at(-1));
  expect(saved.args[0]).toMatchObject({ enabledPreference: false });
  const startCallsAfterSave = await page.evaluate(() => __desktopFixture.calls
    .filter(call => call.method === 'StartPublicAccess').length);
  expect(startCallsAfterSave).toBe(startCallsBefore);
});

test('Start and Stop map exact lifecycle states and never expose a pre-ready URL', async ({ page }) => {
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, username: 'players', revision: 0 },
    providerTokenPresence: 'present',
    playerPasswordPresence: 'present',
    status: { state: 'starting', generation: 1, settingsRevision: 0, publicUrl: 'https://must-not-render.invalid' },
  }));
  await expect(page.locator('#publicAccessStatus')).toHaveText('ЗАПУСК…');
  await expect(page.locator('#publicAccessURL')).toHaveText('ПОЯВИТСЯ ПОСЛЕ ЗАПУСКА');
  await expect(page.locator('#btnSavePublicAccess')).toBeDisabled();
  await expect(page.locator('#btnStartPublicAccess')).toBeDisabled();
  await expect(page.locator('#btnStopPublicAccess')).toBeHidden();

  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, username: 'players', revision: 0 },
    providerTokenPresence: 'present',
    playerPasswordPresence: 'present',
    status: { state: 'disabled', generation: 2, settingsRevision: 0 },
  }));
  await expect(page.locator('#publicAccessStatus')).toHaveText('ОСТАНОВЛЕН');
  await page.locator('#btnStartPublicAccess').click();
  await expect(page.locator('#publicAccessStatus')).toHaveText('ГОТОВ');
  await expect(page.locator('#publicAccessURL')).toHaveText('https://fixture.example');
  await expect(page.locator('#btnStartPublicAccess')).toBeHidden();
  await expect(page.locator('#btnStopPublicAccess')).toBeVisible();
  await page.locator('#btnStopPublicAccess').click();
  await expect(page.locator('#publicAccessStatus')).toHaveText('ОСТАНОВЛЕН');
  await expect(page.locator('#publicAccessURL')).toHaveText('ПОЯВИТСЯ ПОСЛЕ ЗАПУСКА');
});

test('random and reserved ready outcomes support URL copying and native credential sharing', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  for (const [index, publicUrl] of ['https://random.example', 'https://vault.example'].entries()) {
    await page.evaluate(({ url, index: eventIndex }) => __desktopFixture.emit('public-access-status', {
      preferences: { version: 1, reservedDomain: url.includes('vault') ? 'vault.example' : '', username: 'wanderers', revision: 3 + eventIndex },
      providerTokenPresence: 'present',
      playerPasswordPresence: 'present',
      status: { state: 'ready', generation: 4 + eventIndex, settingsRevision: 3 + eventIndex, publicUrl: url },
    }), { url: publicUrl, index });
    await expect(page.locator('#publicAccessURL')).toHaveText(publicUrl);
    await page.locator('#btnCopyPublicURL').click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(publicUrl);
  }
  await openPublicAccessSettings(page);
  await expect(page.locator('#publicAccessUsernameSummary')).toHaveText('wanderers');
  await expect(page.locator('#publicAccessPasswordMask')).toHaveText('*****');
  await page.locator('#btnSharePublicAccessCredentials').click();
  await expect(page.locator('#publicAccessSettingsCopyStatus')).toHaveText('ЛОГИН И ПАРОЛЬ СКОПИРОВАНЫ');
  expect(await page.evaluate(() => __desktopFixture.takeClipboardText()))
    .toBe('Логин: wanderers\nПароль: synthetic-player-share-value');
  await expect(page.locator('body')).not.toContainText('synthetic-player-share-value');
});

test('credential sharing stays native when WebView clipboard permission is unavailable', async ({ page }) => {
  await page.evaluate(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: () => Promise.reject(new DOMException('denied', 'NotAllowedError')) },
    });
  });

  await openPublicAccessSettings(page);
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: false, username: 'players', revision: 2 },
    providerTokenPresence: 'present', playerPasswordPresence: 'present',
    status: { state: 'disabled', generation: 2, settingsRevision: 2 },
  }));
  await page.locator('#btnSharePublicAccessCredentials').click();
  await expect(page.locator('#publicAccessSettingsCopyStatus')).toHaveText('ЛОГИН И ПАРОЛЬ СКОПИРОВАНЫ');
  expect(await page.evaluate(() => __desktopFixture.takeClipboardText()))
    .toBe('Логин: players\nПароль: synthetic-player-share-value');
});

test('failed domain state is redacted, URL-free, and rendered as error', async ({ page }) => {
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, reservedDomain: 'unavailable.example', username: 'players', revision: 8 },
    providerTokenPresence: 'present',
    playerPasswordPresence: 'present',
    status: {
      state: 'failed', generation: 9, settingsRevision: 8,
      errorCategory: 'domain_unavailable', errorMessage: 'The reserved domain is unavailable for this account.',
    },
  }));
  await expect(page.locator('#publicAccessStatus')).toHaveText('ОШИБКА');
  await expect(page.locator('#publicAccessError')).toContainText('unavailable');
  await expect(page.locator('#publicAccessURL')).toHaveText('ПОЯВИТСЯ ПОСЛЕ ЗАПУСКА');
  await expect(page.locator('body')).not.toContainText(/authtoken|password-test-value/i);
});

test('active edits require confirmation and render stopped-to-starting replacement with disabled actions', async ({ page }) => {
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: true, reservedDomain: 'before.example', username: 'players', revision: 7 },
    providerTokenPresence: 'present',
    playerPasswordPresence: 'present',
    status: { state: 'ready', generation: 10, settingsRevision: 7, publicUrl: 'https://before.example' },
  }));
  await openPublicAccessSettings(page);
  await page.evaluate(() => __desktopFixture.deferSavePublicAccess());
  await page.getByLabel('Зарезервированный домен').fill('after.example');
  await openPlayerCredentialsDialog(page);
  await page.getByLabel('Логин игроков').fill('friends');
  await page.getByLabel('Новый пароль игроков').fill('synthetic-active-password-replacement');

  let confirmation = '';
  page.on('dialog', async dialog => {
    confirmation = dialog.message();
    await dialog.accept();
  });
  await page.getByRole('button', { name: 'СОХРАНИТЬ ДАННЫЕ' }).click();
  await expect.poll(() => confirmation).toMatch(/останов|перезапуск|актив/i);
  await expect(page.getByLabel('Новый пароль игроков')).toHaveValue('');
  await expect(page.locator('body')).not.toContainText('synthetic-active-password-replacement');

  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: true, reservedDomain: 'before.example', username: 'players', revision: 7 },
    providerTokenPresence: 'present', playerPasswordPresence: 'present',
    status: { state: 'stopping', generation: 11, settingsRevision: 7 },
  }));
  await expect(page.locator('#publicAccessStatus')).toHaveText('ОСТАНОВКА…');
  await expect(page.locator('#btnSavePublicAccess')).toBeDisabled();
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: true, reservedDomain: 'after.example', username: 'friends', revision: 8 },
    providerTokenPresence: 'present', playerPasswordPresence: 'present',
    status: { state: 'starting', generation: 12, settingsRevision: 8 },
  }));
  await expect(page.locator('#publicAccessStatus')).toHaveText('ЗАПУСК…');
  await expect(page.locator('#btnStartPublicAccess')).toBeDisabled();
  await expect(page.locator('#btnStopPublicAccess')).toBeHidden();
  await page.evaluate(() => __desktopFixture.resolveSavePublicAccess({
    ok: true,
    snapshot: {
      preferences: { version: 1, enabledPreference: true, reservedDomain: 'after.example', username: 'friends', revision: 8 },
      providerTokenPresence: 'present', playerPasswordPresence: 'present',
      status: { state: 'ready', generation: 12, settingsRevision: 8, publicUrl: 'https://after.example' },
    },
  }));
  await expect(page.locator('#publicAccessStatus')).toHaveText('ГОТОВ');
  await expect(page.locator('#publicAccessURL')).toHaveText('https://after.example');
});

test('cancelled active deletion sends no mutation and saved secrets are never reconstructed', async ({ page }) => {
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: true, username: 'players', revision: 4 },
    providerTokenPresence: 'present', playerPasswordPresence: 'present',
    status: { state: 'ready', generation: 6, settingsRevision: 4, publicUrl: 'https://fixture.example' },
  }));
  await openPublicAccessSettings(page);
  const callsBefore = await page.evaluate(() => __desktopFixture.calls.filter(call => call.method === 'SavePublicAccessSettings').length);
  page.once('dialog', dialog => dialog.dismiss());
  await openPlayerCredentialsDialog(page);
  await page.getByRole('button', { name: 'УДАЛИТЬ СОХРАНЁННЫЕ ДАННЫЕ' }).click();
  const callsAfter = await page.evaluate(() => __desktopFixture.calls.filter(call => call.method === 'SavePublicAccessSettings').length);
  expect(callsAfter).toBe(callsBefore);
  await expect(page.locator('#publicAccessPlayerCredentialsDialog')).toBeVisible();
  await expect(page.getByLabel('Новый пароль игроков')).toHaveValue('');
  await expect(page.locator('#publicAccessProviderPresence')).toHaveText(/настроен/i);
  await expect(page.locator('#publicAccessPasswordMask')).toHaveText('*****');
});

test('newer reconfigure event wins over a stale command result and stale secret fields stay empty', async ({ page }) => {
  await page.evaluate(() => {
    __desktopFixture.emit('public-access-status', {
      preferences: { version: 1, enabledPreference: true, reservedDomain: 'before.example', username: 'players', revision: 7 },
      providerTokenPresence: 'present', playerPasswordPresence: 'present',
      status: { state: 'ready', generation: 20, settingsRevision: 7, publicUrl: 'https://before.example' },
    });
    __desktopFixture.deferSavePublicAccess();
  });
  await openPublicAccessSettings(page);
  page.on('dialog', dialog => dialog.accept());
  await openPlayerCredentialsDialog(page);
  await page.getByLabel('Логин игроков').fill('requested');
  await page.getByLabel('Новый пароль игроков').fill('synthetic-stale-result-password');
  await page.getByRole('button', { name: 'СОХРАНИТЬ ДАННЫЕ' }).click();
  await page.evaluate(() => __desktopFixture.emit('public-access-status', {
    preferences: { version: 1, enabledPreference: true, reservedDomain: 'newer.example', username: 'newer', revision: 8 },
    providerTokenPresence: 'present', playerPasswordPresence: 'present',
    status: { state: 'ready', generation: 22, settingsRevision: 8, publicUrl: 'https://newer.example' },
  }));
  await page.evaluate(() => __desktopFixture.resolveSavePublicAccess({
    ok: true,
    snapshot: {
      preferences: { version: 1, enabledPreference: true, reservedDomain: 'stale.example', username: 'stale', revision: 8 },
      providerTokenPresence: 'present', playerPasswordPresence: 'present',
      status: { state: 'disabled', generation: 21, settingsRevision: 8 },
    },
  }));
  await expect(page.locator('#publicAccessStatus')).toHaveText('ГОТОВ');
  await expect(page.locator('#publicAccessURL')).toHaveText('https://newer.example');
  await expect(page.locator('#publicAccessUsernameSummary')).toHaveText('newer');
  await expect(page.locator('#publicAccessProviderConfigured')).not.toHaveAttribute('hidden', '');
  await expect(page.locator('#publicAccessReplacementProviderToken')).toHaveValue('');
  await expect(page.getByLabel('Новый пароль игроков')).toHaveValue('');
  await expect(page.locator('body')).not.toContainText('synthetic-stale-result-password');
});

test('active mutation failure stays URL-free, redacted, and keeps transition actions usable for retry', async ({ page }) => {
  await page.evaluate(() => {
    __desktopFixture.emit('public-access-status', {
      preferences: { version: 1, enabledPreference: true, username: 'players', revision: 3 },
      providerTokenPresence: 'present', playerPasswordPresence: 'present',
      status: { state: 'ready', generation: 4, settingsRevision: 3, publicUrl: 'https://before.example' },
    });
    __desktopFixture.deferSavePublicAccess();
  });
  await openPublicAccessSettings(page);
  page.on('dialog', dialog => dialog.accept());
  await openPlayerCredentialsDialog(page);
  await page.getByLabel('Логин игроков').fill('friends');
  await page.getByRole('button', { name: 'СОХРАНИТЬ ДАННЫЕ' }).click();
  await page.evaluate(() => __desktopFixture.resolveSavePublicAccess({
    ok: false,
    error: 'Keychain is unavailable; local access remains available.',
    snapshot: {
      preferences: { version: 1, enabledPreference: true, username: 'players', revision: 3 },
      providerTokenPresence: 'unknown', playerPasswordPresence: 'present',
      status: {
        state: 'failed', generation: 5, settingsRevision: 3,
        errorCategory: 'secret_store_unavailable', errorMessage: 'Keychain is unavailable; local access remains available.',
      },
    },
  }));
  await expect(page.locator('#publicAccessStatus')).toHaveText('ОШИБКА');
  await expect(page.locator('#publicAccessURL')).toHaveText('ПОЯВИТСЯ ПОСЛЕ ЗАПУСКА');
  await expect(page.locator('#publicAccessError')).toContainText('secure credential store is unavailable');
  await expect(page.locator('#publicAccessSettingsDialog')).toBeVisible();
  await expect(page.locator('#publicAccessPlayerCredentialsDialog')).toBeVisible();
  await expect(page.getByLabel('Логин игроков')).toHaveValue('friends');
  await expect(page.locator('#publicAccessPlayerCredentialsError')).toContainText('Keychain is unavailable');
  await expect(page.locator('#btnSavePublicAccessPlayerCredentials')).toBeEnabled();
  await expect(page.locator('#btnStartPublicAccess')).toBeEnabled();
});
