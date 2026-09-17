import { defineConfig } from '@playwright/test';
export default defineConfig({
 testDir: './tests', testMatch: 'student-openings.spec.ts', reporter: 'list',
 use: { baseURL: 'http://127.0.0.1:5187' },
 webServer: { command: 'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5187 --strictPort', url: 'http://127.0.0.1:5187', reuseExistingServer: false }
});
