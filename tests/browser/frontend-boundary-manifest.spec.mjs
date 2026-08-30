import { readFile } from 'node:fs/promises';

import { expect, test } from '@playwright/test';

const manifestPath = new URL('./fixtures/frontend-boundary-manifest.json', import.meta.url);

test('frontend boundary manifest skeleton is well formed and mapped', async () => {
  const manifest = JSON.parse(await readFile(manifestPath, 'utf8'));
  expect(manifest.schemaVersion).toBe(1);
  expect(manifest.fixtures).toHaveLength(1);
  expect(manifest.fixtures[0]).toEqual({
    boundaryClass: 'Wails method result',
    fixtureId: 'desktop-invalid-runtime-status-rejected',
    owner: 'frontend/overseer/src/adapters/desktop-api.ts',
    expectedResult: 'reject',
    trustedOutcome: 'no-state-change',
    wave: 'c',
    focusedTest: 'tests/browser/desktop-api.spec.mjs',
  });
});
