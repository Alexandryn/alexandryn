import { join } from 'node:path'
import { app, BrowserWindow } from 'electron'

// Phase 05 scaffold (E2). The real lifecycle — spawn the Go server, poll
// /healthz, load the real UI only once ready, crash recovery — is
// Tiers 1–3. For now this opens one window on the disk-loaded boot asset
// so the _electron harness (E4) has a target and can assert the security
// flags.

const BOOT_HTML = join(import.meta.dirname, '../renderer/index.html')

function createWindow(): BrowserWindow {
  const window = new BrowserWindow({
    show: false,
    width: 1024,
    height: 720,
    webPreferences: {
      // architecture-desktop-host.md FR-2 — the privilege boundary, set
      // as literal constructor options a test verifies (E4, E14). No
      // per-window exception, ever.
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      preload: join(import.meta.dirname, '../preload/index.js'),
    },
  })

  void window.loadFile(BOOT_HTML)
  window.once('ready-to-show', () => window.show())
  return window
}

app.whenReady().then(() => {
  createWindow()
})

// architecture-desktop-host.md FR-11 / desktop-host-process-model.md FR-5:
// close-means-quit on every platform, including macOS (whose framework
// default is the opposite). Tier 1 (E11) adds the child-process shutdown
// this must also wait for.
app.on('window-all-closed', () => {
  app.quit()
})
