import { defineConfig } from '@playwright/test';
export default defineConfig({
  outputDir: '/tmp/starline-material-order-results',
  testDir: './tests', testMatch: 'material-file-order.spec.ts', reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:5193', channel: 'chrome' },
  webServer: { command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5193 --strictPort', url: 'http://127.0.0.1:5193', reuseExistingServer: false }
});
