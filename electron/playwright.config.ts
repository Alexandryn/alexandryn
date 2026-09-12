import { defineConfig } from '@playwright/test'

// Electron end-to-end tests use
// @playwright/test's own `_electron` support — never the Playwright MCP —
// run on a Linux CI runner behind a virtual display server (xvfb).
// The suite launches the built app from electron/out, so `npm run build`
// must run first (the CI `desktop` job and the local scripts both do).
export default defineConfig({
  testDir: './e2e',
  testMatch: /.*\.electron\.spec\.ts$/,
  fullyParallel: false,
  workers: 1,
  // Fail CI if test.only was left in any spec file.
  forbidOnly: !!process.env.CI,
  // A cold Electron launch on a shared CI runner can miss a timing
  // budget once; a real regression fails every attempt.
  retries: process.env.CI ? 2 : 0,
  timeout: 30_000,
  reporter: process.env.CI ? [['github'], ['list']] : [['list']],
  projects: [{ name: 'electron' }],
})
