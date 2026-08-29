import { defineConfig, devices } from '@playwright/test';
import process from 'node:process';

const outputDir = process.env.GUARDIAN_E2E_OUTPUT_DIR;
if (!outputDir) throw new Error('GUARDIAN_E2E_OUTPUT_DIR is required');
const resultsDir = process.env.GUARDIAN_E2E_RESULTS_DIR;
if (!resultsDir) throw new Error('GUARDIAN_E2E_RESULTS_DIR is required');

export default defineConfig({
  testDir: '.',
  testMatch: 'onboarding.spec.ts',
  timeout: 90_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  outputDir: resultsDir,
  use: {
    baseURL: process.env.GUARDIAN_E2E_BASE_URL,
    ignoreHTTPSErrors: true,
    trace: 'off',
    video: 'off',
    screenshot: 'off',
    contextOptions: { reducedMotion: 'reduce' },
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
});
