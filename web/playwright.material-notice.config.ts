import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  outputDir: '/tmp/starline-material-notice-results',
  testDir: './tests',
  testMatch: 'material-update-notices.spec.ts',
  use: { baseURL: 'http://127.0.0.1:5187', trace: 'retain-on-failure', ...devices['Desktop Chrome'], channel: 'chrome', viewport: { width: 1440, height: 1000 } },
  webServer: { command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5187 --strictPort', url: 'http://127.0.0.1:5187', reuseExistingServer: false },
});
