import { defineConfig } from '@playwright/test'

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
  timeout: 30_000,
  // One worker, never parallel: the benchmark project asserts a tight
  // initial-paint budget (frontend-generated-covers.md FR-4) and is
  // starved when it shares the machine with other browsers. Retries
  // cover a transient cold-start slow frame on a shared CI runner
  // without weakening the budget itself — a real regression fails every
  // attempt.
  fullyParallel: false,
  workers: 1,
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
      testMatch: /\.app\.spec\.ts$/,
      use: { baseURL: 'http://localhost:5175' },
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
      timeout: 60_000,
    },
    {
      command: 'npx vite --config e2e/a11y-gallery/vite.config.ts --port 5176 --strictPort',
      url: 'http://localhost:5176',
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
  ],
})
