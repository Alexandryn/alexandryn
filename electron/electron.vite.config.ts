import { copyFileSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig, externalizeDepsPlugin } from 'electron-vite'

const webTokensPath = resolve(import.meta.dirname, '../web/src/tokens.css')
const bootTokensPath = resolve(import.meta.dirname, 'src/renderer/boot/tokens.css')

/**
 * Asserts web/src/tokens.css exists at build time (hard failure if missing)
 * and copies it into electron/src/renderer/boot/ for disk-loaded asset bundling (D4 / FR-3).
 */
export function syncTokensPlugin() {
  return {
    name: 'assert-and-sync-tokens',
    buildStart() {
      if (!existsSync(webTokensPath)) {
        throw new Error(
          `Build failed: required tokens file not found at ${webTokensPath}. Run token generation in web/ first.`,
        )
      }
      copyFileSync(webTokensPath, bootTokensPath)
    },
  }
}

// Three build targets (D2, tasks/plan-phase05.md). electron-vite auto-
// detects src/main/index.ts and src/preload/index.ts and sets each
// target's outDir (out/main, out/preload, out/renderer).
//  - main    → the Electron main process (Tiers 1–3, 5)
//  - preload → contextBridge / the IPC surface (Tier 4)
//  - renderer (boot) → the disk-loaded loading/error asset
//    (architecture-desktop-host.md FR-6/FR-7) — `loadFile` only, never
//    `loadURL`; NOT the real web UI, which the Go server serves at runtime.
//
// `build.target` is set explicitly per target: Electron 44 bundles
// Node 22 / Chromium 140, so main and preload compile to `node22` and the
// boot renderer to a recent Chromium baseline. electron-vite 5 requires
// these to be explicit against Vite 7.3+.
export default defineConfig({
  main: {
    build: { target: 'node22' },
    plugins: [externalizeDepsPlugin()],
  },
  preload: {
    build: { target: 'node22' },
    plugins: [externalizeDepsPlugin()],
  },
  renderer: {
    root: 'src/renderer/boot',
    plugins: [syncTokensPlugin()],
    build: {
      target: 'chrome140',
      rollupOptions: {
        input: { boot: resolve(import.meta.dirname, 'src/renderer/boot/index.html') },
      },
    },
  },
})
