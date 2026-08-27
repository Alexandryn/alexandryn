import { existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

// MSW's worker script (public/mockServiceWorker.js) is a dev/E2E asset
// only — Vite copies public/ into dist by default, which would put it in
// the shipped bundle and trip check:dist-msw. This removes it from the
// build output after the copy, so the exclusion is a real guarantee, not
// a convention (frontend-shell-and-routing.md Security considerations).
function stripMswWorkerFromBuild(): Plugin {
  return {
    name: 'strip-msw-worker-from-build',
    apply: 'build',
    writeBundle(options) {
      const worker = join(options.dir ?? 'dist', 'mockServiceWorker.js')
      if (existsSync(worker)) rmSync(worker)
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), stripMswWorkerFromBuild()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
    // Scoped to src/scripts only — e2e/ is Playwright's own testDir
    // (playwright.config.ts, D3), and the two runners' test() APIs collide
    // if Vitest's default include glob also picks up e2e/*.spec.ts.
    include: ['src/**/*.{test,spec}.{ts,tsx}', 'scripts/**/*.{test,spec}.{ts,tsx}'],
  },
})
