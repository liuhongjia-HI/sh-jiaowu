import { defineConfig } from '@playwright/test';
import base from './playwright.real-plan.config';
export default defineConfig({ ...base, testMatch: ['teacher-scopes-real.spec.ts'], outputDir: '/tmp/starline-real-teacher-ui-results' });
