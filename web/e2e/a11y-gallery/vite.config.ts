import { resolve } from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// Minimal Vite config for the accessibility gallery harness only —
// every primitive on one page, scanned by @axe-core/playwright in a
// real browser. Kept out of the app's own entry point, same as the
// benchmark harness.
//
// Its own cacheDir — see the note in e2e/vite.config.ts: three Playwright
// dev servers boot together and must not share the dep-optimization
// cache.
export default defineConfig({
  root: 'e2e/a11y-gallery',
  cacheDir: resolve(import.meta.dirname, '../../node_modules/.vite-gallery'),
  plugins: [react(), tailwindcss()],
})
