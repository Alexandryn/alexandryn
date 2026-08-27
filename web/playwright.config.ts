import { defineConfig } from '@playwright/test'

// Two suites, each with its own dev server (D3, tasks/plan-p04-tier3 and
// -tier4). `benchmark` measures GeneratedCover on a bare harness
// (frontend-generated-covers.md FR-4); `app` drives the real application
// — routing, the shell reflow, a keyboard-only walkthrough, and
// @axe-core/playwright — against MSW-mocked data
// (frontend-shell-and-routing.md Test strategy, frontend-accessibility.md
// FR-4). The app server is `vite` itself, so MSW's dev worker starts
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
  retries: process.env.CI ? 2 : 0,
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
  ],
})
