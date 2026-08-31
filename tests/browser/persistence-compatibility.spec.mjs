import { expect, test } from '@playwright/test';

const assertion = 'legacy/current session and player config round-trip through migrated boundary';
const authoringURL = '/__fixture/state-changing-command-authoring';

test.use({ bypassCSP: true });

async function mountCandidate(page) {
  await page.evaluate(() => import('http://127.0.0.1:34120/candidate-main.ts?persistence-compatibility'));
}

async function openCompatibilitySession(page) {
  await page.goto(authoringURL);
  await mountCandidate(page);
  await page.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
  await expect(page.locator('#mainLayout')).toBeVisible();
}

test(assertion, async ({ page }) => {
  await page.goto('/__fixture/persistence-compatibility');
  await page.locator('#runCompatibility').click();
  await expect(page.locator('#compatibilityStatus')).toHaveText('PASS');

  const rendered = page.locator('#compatibilityDocuments > li');
  await expect(rendered).toHaveCount(4);
  await expect(rendered.nth(0)).toHaveAttribute('data-fixture', 'session-current-v1.json');
  await expect(rendered.nth(1)).toHaveAttribute('data-fixture', 'session-legacy-v1.json');
  await expect(rendered.nth(2)).toHaveAttribute('data-fixture', 'player-config-current-v1.json');
  await expect(rendered.nth(3)).toHaveAttribute('data-fixture', 'player-config-legacy-v1.json');

  const response = await page.request.post('/__fixture/persistence-compatibility/check');
  expect(response.ok()).toBe(true);
  const result = await response.json();
  expect(result.ok).toBe(true);
  expect(result.documents).toHaveLength(4);
  for (const documentResult of result.documents) {
    expect(documentResult.location).toContain(documentResult.fixture);
    expect(documentResult.nameAfter).toBe(`${documentResult.nameBefore} · compatibility edit`);
  }

  const sessions = result.documents.filter(documentResult => documentResult.kind === 'session');
  expect(sessions.map(documentResult => documentResult.playerConfigReference)).toEqual([
    'player-config-current-v1.json',
    'player-config-legacy-v1.json',
  ]);
  expect(sessions.every(documentResult => documentResult.sessionExtrasPreserved)).toBe(true);

  const playerConfigs = result.documents.filter(documentResult => documentResult.kind === 'player-config');
  expect(playerConfigs.every(documentResult => documentResult.unknownFieldsRejected)).toBe(true);
  expect(playerConfigs.find(documentResult => documentResult.fixture === 'player-config-legacy-v1.json'))
    .toMatchObject({ legacyDefaultsPreserved: true });

  const sessionCases = [
    {
      fixture: 'session-current-v1.json',
      name: 'Current Overseer',
      playerConfig: 'player-config-current-v1.json',
    },
    {
      fixture: 'session-legacy-v1.json',
      name: 'Legacy Terminal',
      playerConfig: 'player-config-legacy-v1.json',
    },
  ];
  for (const sessionCase of sessionCases) {
    const load = await page.request.post(`/__fixture/persistence-compatibility/load/${sessionCase.fixture}`);
    expect(load.ok()).toBe(true);
    await openCompatibilitySession(page);
    await expect(page.locator('#editingTermName')).toHaveText(sessionCase.name);
    await expect(page.locator('#playerConfigStatus')).toContainText(sessionCase.playerConfig);

    const editedIntro = `${sessionCase.fixture} · migrated boundary edit`;
    await page.locator('#introTextArea').fill(editedIntro);
    await page.locator('#btnApplySettings').click();
    await expect(page.locator('#saveStatus')).toContainText('Сохранено');

    await openCompatibilitySession(page);
    await expect(page.locator('#editingTermName')).toHaveText(sessionCase.name);
    await expect(page.locator('#introTextArea')).toHaveValue(editedIntro);
    await expect(page.locator('#playerConfigStatus')).toContainText(sessionCase.playerConfig);
  }
});
