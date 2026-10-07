import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './tests', testMatch: 'admin-staff-lifecycle.spec.ts', reporter: 'list', outputDir: '/tmp/starline-admin-staff-results',
  use: { baseURL: 'http://127.0.0.1:5189', screenshot: 'only-on-failure' },
  webServer: {
    command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5189 --strictPort',
    url: 'http://127.0.0.1:5189', reuseExistingServer: false
  }
});
