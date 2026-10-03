import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests', testMatch: ['teaching-plan-real.spec.ts'],
  workers: 1, timeout: 90000,
  outputDir: '/tmp/starline-real-plan-ui-results',
  use: { baseURL: 'http://127.0.0.1:5187', actionTimeout: 10000, trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  webServer: { command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5187 --strictPort', url: 'http://127.0.0.1:5187', reuseExistingServer: false },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], channel: 'chromium' } }]
});
