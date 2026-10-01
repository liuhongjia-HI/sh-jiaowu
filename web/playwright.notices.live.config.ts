import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
 testDir: './tests', testMatch: 'automatic-notices.live.spec.ts', timeout: 90_000, reporter: 'list',
 use: { baseURL: 'http://127.0.0.1:5190', screenshot: 'only-on-failure', trace: 'retain-on-failure' },
 webServer: [
  { command: 'STARLINE_NOTICE_BROWSER_TEST=1 go -C ../learning-api test ./internal/infrastructure/store -run ^TestBusinessNoticeBrowserHarness$ -count=1 -timeout=5m -v', url: 'http://127.0.0.1:18992/api/health', timeout: 60_000, reuseExistingServer: false },
  { command: 'HTTP_PORT=18992 node ./node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5190 --strictPort', url: 'http://127.0.0.1:5190', reuseExistingServer: false }
 ],
 projects: [{ name:'chromium',use:{...devices['Desktop Chrome'],viewport:{width:1440,height:1000},timezoneId:'Asia/Shanghai'} }]
});
