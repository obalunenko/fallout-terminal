import { readFile } from 'node:fs/promises';

import { expect, test } from '@playwright/test';

const manifestPath = new URL('./fixtures/frontend-boundary-manifest.json', import.meta.url);
const desktopTestPath = new URL('./desktop-api.spec.mjs', import.meta.url);
const playerCandidateTestPath = new URL('./player-candidate-boundary.spec.mjs', import.meta.url);

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

const waveFFixtureIds = [
  'player-invalid-incomplete-snapshot-rejected',
  'player-valid-complete-snapshot-accepted',
  'player-invalid-recognition-record-rejected',
  'player-valid-recognition-record-accepted',
  'player-unauthorized-dom-action-rejected',
  'player-authorized-dom-action-accepted',
  'player-invalid-navigation-state-rejected',
  'player-valid-navigation-state-accepted',
];

const waveGFixtureIds = [
  'player-unauthorized-pointer-target-rejected',
  'player-current-pointer-target-accepted',
  'player-observer-keyboard-input-rejected',
  'player-controller-keyboard-input-accepted',
  'player-unsafe-sound-asset-rejected',
  'player-safe-sound-asset-accepted',
  'player-stale-uplink-result-rejected',
  'player-correlated-uplink-result-accepted',
];

function validateWaveCManifest(manifest, desktopTestSource) {
  if (manifest.schemaVersion !== 1) throw new Error('frontend boundary manifest schemaVersion must be 1');
  const ids = manifest.fixtures.map(fixture => fixture.fixtureId);
  if (new Set(ids).size !== ids.length) throw new Error('frontend boundary manifest contains duplicate fixture IDs');
  const missing = waveCFixtureIds.filter(fixtureId => !ids.includes(fixtureId));
  if (missing.length > 0) throw new Error(`frontend boundary manifest missing wave-c mapping: ${missing.join(', ')}`);
  const unexpected = manifest.fixtures
    .filter(fixture => fixture.wave === 'c')
    .map(fixture => fixture.fixtureId)
    .filter(fixtureId => !waveCFixtureIds.includes(fixtureId));
  if (unexpected.length > 0) throw new Error(`frontend boundary manifest has unexpected wave-c mapping: ${unexpected.join(', ')}`);

  for (const fixture of manifest.fixtures.filter(candidate => candidate.wave === 'c')) {
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

function validateWaveFManifest(manifest, playerCandidateTestSource) {
  if (manifest.schemaVersion !== 1) throw new Error('frontend boundary manifest schemaVersion must be 1');
  const ids = manifest.fixtures.map(fixture => fixture.fixtureId);
  if (new Set(ids).size !== ids.length) throw new Error('frontend boundary manifest contains duplicate fixture IDs');
  const missing = waveFFixtureIds.filter(fixtureId => !ids.includes(fixtureId));
  if (missing.length > 0) throw new Error(`frontend boundary manifest missing wave-f mapping: ${missing.join(', ')}`);
  const waveFFixtures = manifest.fixtures.filter(fixture => fixture.wave === 'f');
  const unexpected = waveFFixtures.map(fixture => fixture.fixtureId)
    .filter(fixtureId => !waveFFixtureIds.includes(fixtureId));
  if (unexpected.length > 0) throw new Error(`frontend boundary manifest has unexpected wave-f mapping: ${unexpected.join(', ')}`);
  for (const fixture of waveFFixtures) {
    if (!fixture.owner.startsWith('frontend/client/src/')) {
      throw new Error(`frontend boundary fixture has wrong Player owner: ${fixture.fixtureId}`);
    }
    if (fixture.focusedTest !== 'tests/browser/player-candidate-boundary.spec.mjs') {
      throw new Error(`frontend boundary fixture has wrong Player focused test: ${fixture.fixtureId}`);
    }
    if (!['accept', 'reject'].includes(fixture.expectedResult)
      || typeof fixture.trustedOutcome !== 'string' || fixture.trustedOutcome.length === 0) {
      throw new Error(`frontend boundary fixture has invalid expected outcome: ${fixture.fixtureId}`);
    }
    if (!playerCandidateTestSource.includes(`test('${fixture.focusedTitle}'`)) {
      throw new Error(`frontend boundary fixture focused title is not implemented: ${fixture.fixtureId}`);
    }
  }
}

function validateWaveGManifest(manifest, playerCandidateTestSource) {
  const ids = manifest.fixtures.map(fixture => fixture.fixtureId);
  const missing = waveGFixtureIds.filter(fixtureId => !ids.includes(fixtureId));
  if (missing.length > 0) throw new Error(`frontend boundary manifest missing wave-g mapping: ${missing.join(', ')}`);
  const fixtures = manifest.fixtures.filter(fixture => fixture.wave === 'g');
  const unexpected = fixtures.map(fixture => fixture.fixtureId).filter(fixtureId => !waveGFixtureIds.includes(fixtureId));
  if (unexpected.length > 0) throw new Error(`frontend boundary manifest has unexpected wave-g mapping: ${unexpected.join(', ')}`);
  for (const fixture of fixtures) {
    if (!fixture.owner.startsWith('frontend/client/src/')) {
      throw new Error(`frontend boundary fixture has wrong Player owner: ${fixture.fixtureId}`);
    }
    if (fixture.focusedTest !== 'tests/browser/player-candidate-boundary.spec.mjs' ||
        !['accept', 'reject'].includes(fixture.expectedResult) || fixture.trustedOutcome.length === 0) {
      throw new Error(`frontend boundary fixture has invalid wave-g mapping: ${fixture.fixtureId}`);
    }
    if (!playerCandidateTestSource.includes(`test('${fixture.focusedTitle}'`)) {
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
  expect(manifest.fixtures.filter(fixture => fixture.wave === 'c').map(fixture => fixture.fixtureId).sort())
    .toEqual([...waveCFixtureIds].sort());

  const missingMapping = {
    ...manifest,
    fixtures: manifest.fixtures.filter(fixture => fixture.fixtureId !== waveCFixtureIds[0]),
  };
  expect(() => validateWaveCManifest(missingMapping, desktopTestSource))
    .toThrow(`frontend boundary manifest missing wave-c mapping: ${waveCFixtureIds[0]}`);
});

test('Player candidate boundary mappings are complete', async () => {
  const [manifest, playerCandidateTestSource] = await Promise.all([
    readFile(manifestPath, 'utf8').then(JSON.parse),
    readFile(playerCandidateTestPath, 'utf8'),
  ]);
  expect(() => validateWaveFManifest(manifest, playerCandidateTestSource)).not.toThrow();
  expect(manifest.fixtures.filter(fixture => fixture.wave === 'f').map(fixture => fixture.fixtureId).sort())
    .toEqual([...waveFFixtureIds].sort());

  const missingMapping = {
    ...manifest,
    fixtures: manifest.fixtures.filter(fixture => fixture.fixtureId !== waveFFixtureIds[0]),
  };
  expect(() => validateWaveFManifest(missingMapping, playerCandidateTestSource))
    .toThrow(`frontend boundary manifest missing wave-f mapping: ${waveFFixtureIds[0]}`);
});

test('Player wave-g boundary mappings are complete', async () => {
  const [manifest, playerCandidateTestSource] = await Promise.all([
    readFile(manifestPath, 'utf8').then(JSON.parse),
    readFile(playerCandidateTestPath, 'utf8'),
  ]);
  expect(() => validateWaveGManifest(manifest, playerCandidateTestSource)).not.toThrow();
  expect(manifest.fixtures.filter(fixture => fixture.wave === 'g').map(fixture => fixture.fixtureId).sort())
    .toEqual([...waveGFixtureIds].sort());
  const missingMapping = {
    ...manifest,
    fixtures: manifest.fixtures.filter(fixture => fixture.fixtureId !== waveGFixtureIds[0]),
  };
  expect(() => validateWaveGManifest(missingMapping, playerCandidateTestSource))
    .toThrow(`frontend boundary manifest missing wave-g mapping: ${waveGFixtureIds[0]}`);
});
