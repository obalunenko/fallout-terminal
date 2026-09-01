import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { defineConfig } from '@playwright/test';

const browserFixtureGoCache = process.env.GOCACHE || join(tmpdir(), 'fallout-browser-fixture-cache');

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
  webServer: [
    {
      command: 'npm run build:client --prefix ../../frontend && go run ./fixture-server',
      env: {
        GOCACHE: browserFixtureGoCache,
      },
      url: 'http://127.0.0.1:34119/__fixture/desktop-api',
      reuseExistingServer: false,
    },
    {
      command: '../../frontend/node_modules/.bin/vite ../../frontend/overseer/src --config ../../frontend/overseer/vite.config.ts --mode browser-test --host 127.0.0.1 --port 34121 --strictPort',
      url: 'http://127.0.0.1:34121',
      reuseExistingServer: false,
    },
    {
      command: '../../frontend/node_modules/.bin/vite ../../frontend/client --config ../../frontend/client/vite.config.ts --host 127.0.0.1 --port 34120 --strictPort',
      url: 'http://127.0.0.1:34120',
      reuseExistingServer: false,
    },
  ],
});
