import { defineConfig, devices } from '@playwright/test';
import process from 'node:process';

const outputDir = process.env.GUARDIAN_E2E_OUTPUT_DIR;
if (!outputDir) throw new Error('GUARDIAN_E2E_OUTPUT_DIR is required');
const resultsDir = process.env.GUARDIAN_E2E_RESULTS_DIR;
if (!resultsDir) throw new Error('GUARDIAN_E2E_RESULTS_DIR is required');

export default defineConfig({
  testDir: '.',
  // The three browser specs. `onboarding` is the Phase 1 gate journey;
  // `operator-lifecycle` is `WCX-09`'s step-up gate and `decoy-management` is
  // `WCX-11`'s, carrying the Phase 2 exit-gate evidence. Both of the latter run
  // on one engine, because recovery codes are a bounded fixture.
  testMatch: /(onboarding|operator-lifecycle|decoy-management).spec.ts$/,
  // `WCX-06` added the keyboard traversal at two viewports, three more axe
  // scans, and the hostile display-name round trip to the onboarding flow.
  // Each is real browser work on top of an already long journey.
  timeout: 150_000,
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
