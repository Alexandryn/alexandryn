import { defineConfig } from '@playwright/test'

// frontend-generated-covers.md FR-4's own measurement mechanism — a real
// CI dependency (frontend-shell-and-routing.md/frontend-accessibility.md
// use the same one for their own later stages, D3). testDir is scoped to
// this directory only, so it never collides with Vitest's own
// src/**/*.test.tsx glob (vite.config.ts).
export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  use: {
    baseURL: 'http://localhost:5174',
  },
  webServer: {
    command: 'npx vite --config e2e/vite.config.ts --port 5174 --strictPort',
    url: 'http://localhost:5174',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
})
