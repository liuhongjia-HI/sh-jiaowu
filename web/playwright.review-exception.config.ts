import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
 testDir:'./tests', testMatch:'review-exception-real.spec.ts', workers:1, timeout:60000,
 outputDir:'/tmp/starline-review-exception-ui-results',
 use:{baseURL:'http://127.0.0.1:5189',actionTimeout:10000,screenshot:'only-on-failure',trace:'retain-on-failure'},
 webServer:{command:'node node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5189 --strictPort',url:'http://127.0.0.1:5189',reuseExistingServer:false},
 projects:[{name:'chromium',use:{...devices['Desktop Chrome'],channel:'chromium'}}]
});
