import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.mjs',
  fullyParallel: false,
  workers: 1,
  use: {
    baseURL: 'http://127.0.0.1:34119',
    headless: true,
    ignoreHTTPSErrors: true,
  },
  webServer: {
    command: 'npm run build:client --prefix ../../frontend && GOCACHE=/private/tmp/fallout-browser-fixture-cache go run ./fixture-server',
    url: 'http://127.0.0.1:34119/__fixture/desktop-api',
    reuseExistingServer: false,
  },
});
