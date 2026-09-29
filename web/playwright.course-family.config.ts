import { defineConfig, devices } from '@playwright/test';

const apiPort = process.env.HTTP_PORT ?? '8897';
const webPort = process.env.WEB_PORT ?? '5177';

export default defineConfig({
  testDir: './tests',
  testMatch: ['course-family.e2e.spec.ts', 'admin.e2e.spec.ts'],
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: 'list',
  use: { baseURL: `http://127.0.0.1:${webPort}`, actionTimeout: 10_000, screenshot: 'only-on-failure', trace: 'retain-on-failure' },
  webServer: [
    { command: `cd ../learning-api && HTTP_PORT=${apiPort} go run ./cmd/api`, url: `http://127.0.0.1:${apiPort}/api/health`, timeout: 120_000, reuseExistingServer: false },
    { command: `HTTP_PORT=${apiPort} node ./node_modules/vite/bin/vite.js --host 127.0.0.1 --port ${webPort} --strictPort`, url: `http://127.0.0.1:${webPort}`, timeout: 120_000, reuseExistingServer: false }
  ],
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }]
});
