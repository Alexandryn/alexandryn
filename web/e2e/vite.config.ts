import { resolve } from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// A separate, minimal Vite config for the benchmark harness only
// (frontend-generated-covers.md FR-4) — kept out of the main app's own
// vite.config.ts/App.tsx so the benchmark fixture never leaks into the
// real production entry point.
//
// Its own cacheDir: the three Playwright dev servers (app, benchmark,
// gallery) boot at once and would otherwise race the shared
// node_modules/.vite dep-optimization cache, corrupting each other's
// pre-bundled chunks (symptom: "Failed to fetch dynamically imported
// module" on the app server's msw import).
export default defineConfig({
  root: 'e2e/benchmark',
  cacheDir: resolve(import.meta.dirname, '../node_modules/.vite-benchmark'),
  plugins: [react(), tailwindcss()],
})
