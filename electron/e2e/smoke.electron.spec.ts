import { expect, test, type ElectronApplication } from '@playwright/test'
import { launchHost } from './launch'

// Smoke test verifying the privilege boundary
// (contextIsolation / sandbox / nodeIntegration).

let app: ElectronApplication

test.beforeEach(async () => {
  app = await launchHost({ env: { ALEXANDRYN_SKIP_SERVER_LIFECYCLE: '1' } })
})


test.afterEach(async () => {
  await app.close()
})

test('opens a single window on the boot asset', async () => {
  const window = await app.firstWindow()
  await expect(window.locator('.boot__message')).toHaveText('Starting Alexandryn')
  expect(app.windows()).toHaveLength(1)
})

test('the window is constructed with security flags', async () => {
  const window = await app.firstWindow()
  await window.waitForLoadState('domcontentloaded')

  // Read the actual web preferences back from the main process, not the
  // source: whatever the BrowserWindow was really constructed with.
  // `getLastWebPreferences()` is a real Electron API missing from the
  // current typings, hence the cast.
  const prefs = await app.evaluate(({ BrowserWindow }) => {
    const wc = BrowserWindow.getAllWindows()[0]?.webContents as
      | (Electron.WebContents & { getLastWebPreferences(): Electron.WebPreferences | null })
      | undefined
    return wc?.getLastWebPreferences() ?? null
  })
  expect(prefs).not.toBeNull()
  expect(prefs?.contextIsolation).toBe(true)
  expect(prefs?.sandbox).toBe(true)
  expect(prefs?.nodeIntegration).toBe(false)

  // And the behavioural consequence in the renderer's main world: no Node
  // primitives leaked, and the contextBridge namespace is present.
  const renderer = await window.evaluate(() => ({
    hasRequire: typeof (globalThis as { require?: unknown }).require !== 'undefined',
    hasProcess: typeof (globalThis as { process?: unknown }).process !== 'undefined',
    hasModule: typeof (globalThis as { module?: unknown }).module !== 'undefined',
    hasBridge: typeof (globalThis as { alexandryn?: unknown }).alexandryn !== 'undefined',
  }))
  expect(renderer).toEqual({
    hasRequire: false,
    hasProcess: false,
    hasModule: false,
    hasBridge: true,
  })
})
