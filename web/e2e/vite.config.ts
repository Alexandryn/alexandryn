import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// A separate, minimal Vite config for the benchmark harness only
// (frontend-generated-covers.md FR-4) — kept out of the main app's own
// vite.config.ts/App.tsx so the benchmark fixture never leaks into the
// real production entry point.
export default defineConfig({
  root: 'e2e/benchmark',
  plugins: [react(), tailwindcss()],
})
