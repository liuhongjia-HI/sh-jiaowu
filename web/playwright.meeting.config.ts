import { defineConfig, devices } from '@playwright/test';

// These tests mock only HTTP responses; they exercise the actual React pages.
// Store/router tests separately verify permissions and persistence.
export default defineConfig({
  testDir: './tests', testMatch: ['teacher-lifecycle.spec.ts', 'teacher-disable.spec.ts', 'teacher-unread.spec.ts', 'material-downloads.spec.ts', 'teacher-scopes.spec.ts', 'meeting-feedback.spec.ts', 'scheduling-calendar.spec.ts'],
  fullyParallel: false, workers: 1, timeout: 30000,
  outputDir: '/tmp/starline-meeting-ui-results',
  use: { baseURL: 'http://127.0.0.1:5186', trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  webServer: { command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5186 --strictPort', url: 'http://127.0.0.1:5186', reuseExistingServer: false },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }]
});
