import { join } from 'node:path'
import type { ChildProcess } from 'node:child_process'
import { app, BrowserWindow, Menu } from 'electron'
import { acquireSingleInstanceLock } from './singleInstance'
import { shutdownServer } from './shutdown'
import { getBrowserWindowOptions } from './windowOptions'

import { getWindowStatePath, loadWindowState, trackWindowState } from './windowState'
import { setupWindowNavigation } from './navigation'

import { WindowServingController } from './windowServing'
import { runServerLifecycle } from './serverLifecycle'
import { registerIpcHandlers } from './ipc'

// Phase 05: Full lifecycle orchestration (Tiers 1–4).
// Spawns the Go server, polls /healthz, displays boot asset before ready,
// loads real UI on ready, handles crash recovery with Recovering banner,
// exposes type-safe Zod-validated IPC surface, and shuts down cleanly on quit.

// Explicitly support --user-data-dir override for isolated test runs
const userDataArg = process.argv.find((arg) => arg.startsWith('--user-data-dir='))
if (userDataArg) {
  const customUserData = userDataArg.slice('--user-data-dir='.length)
  app.setPath('userData', customUserData)
}

// E8 — FR-7: single-instance lock BEFORE window creation and BEFORE any
// spawn call, so a second process can never start a second Go server.
if (!acquireSingleInstanceLock()) {
  app.quit()
}


let triggerRetry: (() => void) | undefined

// E20 — Register declared IPC handlers
registerIpcHandlers({
  onRetryStartup: () => {
    triggerRetry?.()
  },
})

const BOOT_HTML = join(import.meta.dirname, '../renderer/index.html')

// The spawned Go server child process. Set by the spawn call (E9/Tier 1–3).
// Accessed by the before-quit shutdown handler (E11).
let serverChild: ChildProcess | undefined

async function createWindow(): Promise<{ window: BrowserWindow; serving: WindowServingController }> {
  // desktop-host-window-and-serving.md FR-1: no application menu bar on any platform for v1
  Menu.setApplicationMenu(null)

  const statePath = getWindowStatePath()
  const savedBounds = await loadWindowState(statePath)
  const window = new BrowserWindow(getBrowserWindowOptions({ bounds: savedBounds }))

  // desktop-host-window-and-serving.md FR-2: persist size and position across sessions
  trackWindowState(window, statePath)

  const serving = new WindowServingController(window, BOOT_HTML)

  // desktop-host-window-and-serving.md FR-4: external link interception and dynamic origin locking
  setupWindowNavigation(window.webContents, {
    getAllowedOrigin: () => {
      const port = serving.getCurrentPort()
      return port !== undefined ? `http://127.0.0.1:${port}` : undefined
    },
  })

  void window.loadFile(BOOT_HTML)
  window.once('ready-to-show', () => window.show())
  return { window, serving }
}

app.whenReady().then(async () => {
  const { serving } = await createWindow()

  // Allow static smoke / boot-asset specs to test the initial window state in isolation
  if (process.env.ALEXANDRYN_SKIP_SERVER_LIFECYCLE === '1') {
    return
  }

  let isRunning = false

  async function startLifecycle(): Promise<void> {
    if (isRunning) return
    isRunning = true

    try {
      await runServerLifecycle({
        onChildSpawned: (child) => {
          serverChild = child
        },
        onEvent: (event) => {
          void serving.handleServerEvent(event)
        },
      })
    } catch (err) {
      console.error('[deskhost] Server lifecycle terminated:', err)
    } finally {
      isRunning = false
    }
  }

  triggerRetry = () => {
    if (!serving.isRealUiLoaded() && !isRunning) {
      void startLifecycle()
    }
  }

  void startLifecycle()
})







// desktop-host-process-model.md FR-5: `before-quit` defers the default
// quit until the Go server shutdown sequence completes (SIGTERM + SIGKILL).
// Without `preventDefault()`, Electron would exit while the child is still
// running.
app.on('before-quit', (event) => {
  if (serverChild === undefined || serverChild.exitCode !== null || serverChild.killed) {
    // No live child — nothing to wait for; let quit proceed immediately.
    return
  }

  event.preventDefault()
  void shutdownServer(serverChild).then(() => app.quit())
})

// desktop-host-process-model.md FR-5 / architecture-desktop-host.md FR-11:
// close-means-quit on every platform, including macOS (whose framework
// default is the opposite — `window-all-closed` does NOT quit the app).
app.on('window-all-closed', () => {
  app.quit()
})

// Expose for Tier 1–3 lifecycle management to set after a successful spawn.
export function setServerChild(child: ChildProcess): void {
  serverChild = child
}
