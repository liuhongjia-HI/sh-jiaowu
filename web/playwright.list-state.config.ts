import { defineConfig } from '@playwright/test';
import meeting from './playwright.meeting.config';
export default defineConfig({ ...meeting, testMatch: ['list-state.spec.ts'], outputDir: '/tmp/starline-list-state-results' });
