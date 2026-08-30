import { readFile } from 'node:fs/promises';

import { expect, test } from '@playwright/test';

const manifestPath = new URL('./fixtures/frontend-boundary-manifest.json', import.meta.url);
const desktopTestPath = new URL('./desktop-api.spec.mjs', import.meta.url);

const waveCFixtureIds = [
  'desktop-invalid-runtime-status-rejected',
  'desktop-valid-runtime-status-accepted',
  'desktop-invalid-server-info-event-rejected',
  'desktop-valid-server-info-event-accepted',
  'desktop-stale-coordination-revision-rejected',
  'desktop-newer-coordination-revision-accepted',
  'desktop-event-release-exact-once',
  'desktop-invalid-clipboard-input-rejected',
  'desktop-valid-clipboard-input-accepted',
  'desktop-method-event-inventory-accepted',
];

function validateWaveCManifest(manifest, desktopTestSource) {
  if (manifest.schemaVersion !== 1) throw new Error('frontend boundary manifest schemaVersion must be 1');
  const ids = manifest.fixtures.map(fixture => fixture.fixtureId);
  if (new Set(ids).size !== ids.length) throw new Error('frontend boundary manifest contains duplicate fixture IDs');
  const missing = waveCFixtureIds.filter(fixtureId => !ids.includes(fixtureId));
  if (missing.length > 0) throw new Error(`frontend boundary manifest missing wave-c mapping: ${missing.join(', ')}`);
  const unexpected = ids.filter(fixtureId => !waveCFixtureIds.includes(fixtureId));
  if (unexpected.length > 0) throw new Error(`frontend boundary manifest has unexpected wave-c mapping: ${unexpected.join(', ')}`);

  for (const fixture of manifest.fixtures) {
    if (fixture.owner !== 'frontend/overseer/src/adapters/desktop-api.ts') {
      throw new Error(`frontend boundary fixture has wrong owner: ${fixture.fixtureId}`);
    }
    if (fixture.wave !== 'c' || fixture.focusedTest !== 'tests/browser/desktop-api.spec.mjs') {
      throw new Error(`frontend boundary fixture has wrong wave or focused test: ${fixture.fixtureId}`);
    }
    if (!['accept', 'reject'].includes(fixture.expectedResult)
      || typeof fixture.trustedOutcome !== 'string' || fixture.trustedOutcome.length === 0) {
      throw new Error(`frontend boundary fixture has invalid expected outcome: ${fixture.fixtureId}`);
    }
    if (!desktopTestSource.includes(`test('${fixture.focusedTitle}'`)) {
      throw new Error(`frontend boundary fixture focused title is not implemented: ${fixture.fixtureId}`);
    }
  }
}

test('DesktopPort adapter mappings are complete', async () => {
  const [manifest, desktopTestSource] = await Promise.all([
    readFile(manifestPath, 'utf8').then(JSON.parse),
    readFile(desktopTestPath, 'utf8'),
  ]);
  expect(() => validateWaveCManifest(manifest, desktopTestSource)).not.toThrow();
  expect(manifest.fixtures.map(fixture => fixture.fixtureId).sort()).toEqual([...waveCFixtureIds].sort());

  const missingMapping = {
    ...manifest,
    fixtures: manifest.fixtures.filter(fixture => fixture.fixtureId !== waveCFixtureIds[0]),
  };
  expect(() => validateWaveCManifest(missingMapping, desktopTestSource))
    .toThrow(`frontend boundary manifest missing wave-c mapping: ${waveCFixtureIds[0]}`);
});
