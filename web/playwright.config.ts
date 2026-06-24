import { defineConfig, devices } from '@playwright/test';

const apiPort = process.env.HTTP_PORT ?? '8892';
const webPort = process.env.WEB_PORT ?? '5173';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  expect: {
    timeout: 10_000
  },
  fullyParallel: false,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${webPort}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure'
  },
  webServer: [
    {
      command: `HTTP_PORT=${apiPort} ../scripts/e2e-api.sh`,
      url: `http://127.0.0.1:${apiPort}/api/health`,
      timeout: 120_000,
      reuseExistingServer: true
    },
    {
      command: `HTTP_PORT=${apiPort} node ./node_modules/vite/bin/vite.js --host 127.0.0.1 --port ${webPort} --strictPort`,
      url: `http://127.0.0.1:${webPort}`,
      timeout: 120_000,
      reuseExistingServer: true
    }
  ],
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] }
    }
  ]
});
