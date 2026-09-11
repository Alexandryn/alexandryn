import { defineConfig, devices } from '@playwright/test'

// Three suites, each with its own dev server (D3, tasks/plan-p04-tier3/-tier4/
// -tier5):
//  - `benchmark` (`*.benchmark.spec.ts`) measures GeneratedCover on a bare
//    harness (frontend-generated-covers.md FR-4)
//  - `app` (`*.app.spec.ts`) drives the real application — routing, the
//    shell reflow, the keyboard walkthrough, @axe-core/playwright on the
//    shell — against MSW-mocked data
//  - `gallery` (`*.gallery.spec.ts`) scans every primitive on one harness
//    page with @axe-core/playwright (frontend-accessibility.md FR-4)
// The `app` server is `vite` itself, so MSW's dev worker starts
// automatically (src/main.tsx).
export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  // Retained only on failure (audit 0017 #325): the Firefox-specific
  // pairing.spec.ts timeout couldn't be reproduced locally (no Firefox
  // available in this dev sandbox without a privileged install), so a
  // trace/screenshot from the next CI failure is the only way to see
  // what state the page is actually in when it times out.
  use: { baseURL: 'http://localhost:5175', trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  // One worker, never parallel: the benchmark project asserts a tight
  // initial-paint budget (frontend-generated-covers.md FR-4) and is
  // starved when it shares the machine with other browsers. Retries
  // cover a transient cold-start slow frame on a shared CI runner
  // without weakening the budget itself — a real regression fails every
  // attempt.
  fullyParallel: false,
  workers: 1,
  // Fail CI if test.only was left in any spec file (audit 0016 #209).
  forbidOnly: !!process.env.CI,
  // A retry also covers the rare cold-server dynamic-import race on a
  // fresh dev server, not only CI-runner timing — so keep one locally.
  retries: process.env.CI ? 2 : 1,
  projects: [
    {
      name: 'benchmark',
      testDir: './e2e',
      testMatch: /benchmark\.spec\.ts$/,
      use: { baseURL: 'http://localhost:5174' },
    },
    {
      name: 'app',
      testDir: './e2e',
      testMatch: /(?:\.app|pairing)\.spec\.ts$/,
      use: { baseURL: 'http://localhost:5175' },
    },
    // Phase 17 (accessibility-and-qa/README.md, Gate 0 G0-4) cross-browser
    // matrix: the same `.app.spec.ts` / `pairing.spec.ts` flows, run against
    // the other engines and the two mobile viewports. Only the `app` project
    // gets this treatment — `gallery` and `benchmark` are synthetic harnesses,
    // not user-facing flows, so they stay Chromium-only.
    {
      name: 'app-firefox',
      testDir: './e2e',
      testMatch: /(?:\.app|pairing)\.spec\.ts$/,
      use: { ...devices['Desktop Firefox'], baseURL: 'http://localhost:5175' },
    },
    {
      name: 'app-webkit',
      testDir: './e2e',
      testMatch: /(?:\.app|pairing)\.spec\.ts$/,
      use: { ...devices['Desktop Safari'], baseURL: 'http://localhost:5175' },
    },
    {
      name: 'app-mobile-chrome',
      testDir: './e2e',
      testMatch: /(?:\.app|pairing)\.spec\.ts$/,
      use: { ...devices['Pixel 5'], baseURL: 'http://localhost:5175' },
    },
    {
      name: 'app-mobile-safari',
      testDir: './e2e',
      testMatch: /(?:\.app|pairing)\.spec\.ts$/,
      use: { ...devices['iPhone 13'], baseURL: 'http://localhost:5175' },
    },
    {
      name: 'gallery',
      testDir: './e2e',
      testMatch: /\.gallery\.spec\.ts$/,
      use: { baseURL: 'http://localhost:5176' },
    },
  ],
  webServer: [
    {
      command: 'npx vite --config e2e/vite.config.ts --port 5174 --strictPort',
      url: 'http://localhost:5174',
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
    {
      command: 'npx vite --port 5175 --strictPort',
      url: 'http://localhost:5175',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
    {
      command: 'npx vite --config e2e/a11y-gallery/vite.config.ts --port 5176 --strictPort',
      url: 'http://localhost:5176',
      reuseExistingServer: !process.env.CI,
      // Radix + axe + every primitive is a heavy first-run dep pre-bundle
      // into a cold cacheDir, and all three servers boot at once — give it
      // the same headroom the app server gets.
      timeout: 120_000,
    },
  ],
})
