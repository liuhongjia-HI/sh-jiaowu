import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './tests', testMatch: ['permission-ux.spec.ts', 'student-openings.spec.ts', 'teacher-scopes.spec.ts'], reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:5187', channel: 'chrome' },
  webServer: { command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5187 --strictPort', url: 'http://127.0.0.1:5187', reuseExistingServer: false }
});
