import { defineConfig, devices } from '@playwright/test';

// Run against an isolated local API/database; see the validation record.
export default defineConfig({
  testDir: './tests',
  testMatch: 'grade-subject-levels.e2e.spec.ts',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: 'list',
  use: { baseURL: process.env.LEVELS_TEST_BASE_URL || 'http://127.0.0.1:15177', trace: 'retain-on-failure', screenshot: 'only-on-failure', actionTimeout: 10_000 },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], channel: 'chrome' } }]
});
