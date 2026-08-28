import { defineConfig } from 'vitest/config'

// Unit tests only (src/**). e2e/ is @playwright/test's own testDir
// (playwright.config.ts) — the two runners' test() APIs collide if
// Vitest's default include glob also picks up e2e/*.electron.spec.ts,
// the same scoping web/'s vite.config.ts uses.
export default defineConfig({
  test: {
    include: ['src/**/*.{test,spec}.ts'],
  },
})
