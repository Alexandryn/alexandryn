import { describe, expect, it, vi } from 'vitest'
import { WindowServingController } from './windowServing'

// desktop-host-window-and-serving.md FR-5, FR-6 — loadURL sequencing & Recovering banner.
// Starting → loadFile boot asset
// Ready → loadURL http://127.0.0.1:<port>
// Recovering → insertCSS + executeJavaScript (role="status", aria-live="polite")
// Ready after Recovering (same port) → remove banner, no reload
// Ready after Recovering (diff port) → reload new port, then remove banner
// Failed → loadFile error state

describe('WindowServingController (FR-5, FR-6)', () => {
  function createMockWindow() {
    return {
      loadFile: vi.fn().mockResolvedValue(undefined),
      loadURL: vi.fn().mockResolvedValue(undefined),
      webContents: {
        insertCSS: vi.fn().mockResolvedValue('css-key-123'),
        removeInsertedCSS: vi.fn().mockResolvedValue(undefined),
        executeJavaScript: vi.fn().mockResolvedValue(undefined),
      },
    } as unknown as import('electron').BrowserWindow
  }

  const BOOT_PATH = '/app/renderer/index.html'

  it('Starting: loads the boot HTML file (FR-3)', async () => {
    const win = createMockWindow()
    const controller = new WindowServingController(win, BOOT_PATH)

    await controller.handleServerEvent({ state: 'Starting' })

    expect(win.loadFile).toHaveBeenCalledWith(BOOT_PATH)
    expect(win.loadURL).not.toHaveBeenCalled()
  })

  it('Ready (first start): loads the loopback URL with announced port (FR-5)', async () => {
    const win = createMockWindow()
    const controller = new WindowServingController(win, BOOT_PATH)

    await controller.handleServerEvent({ state: 'Starting' })
    await controller.handleServerEvent({ state: 'Ready', port: 42100 })

    expect(win.loadURL).toHaveBeenCalledWith('http://127.0.0.1:42100')
    expect(controller.getCurrentPort()).toBe(42100)
    expect(controller.isRealUiLoaded()).toBe(true)
  })

  it('loadURL is never called before a Ready event (illegal transition check)', async () => {
    const win = createMockWindow()
    const controller = new WindowServingController(win, BOOT_PATH)

    await controller.handleServerEvent({ state: 'Starting' })
    expect(win.loadURL).not.toHaveBeenCalled()
    expect(controller.isRealUiLoaded()).toBe(false)
  })

  it('Recovering: injects CSS and banner DOM node with role=status and aria-live=polite (FR-6)', async () => {
    const win = createMockWindow()
    const controller = new WindowServingController(win, BOOT_PATH)

    await controller.handleServerEvent({ state: 'Ready', port: 42100 })
    await controller.handleServerEvent({ state: 'Recovering', attempt: 1 })

    expect(win.webContents.insertCSS).toHaveBeenCalledOnce()
    expect(win.webContents.executeJavaScript).toHaveBeenCalledWith(
      expect.stringContaining("setAttribute('role', 'status')"),
    )
    expect(win.webContents.executeJavaScript).toHaveBeenCalledWith(
      expect.stringContaining("setAttribute('aria-live', 'polite')"),
    )
    expect(win.webContents.executeJavaScript).toHaveBeenCalledWith(
      expect.stringContaining('Attempt 1 of 3'),
    )
  })

  it('Ready after Recovering (SAME port): removes banner without reloading page (FR-6)', async () => {
    const win = createMockWindow()
    const controller = new WindowServingController(win, BOOT_PATH)

    await controller.handleServerEvent({ state: 'Ready', port: 42100 })
    win.loadURL = vi.fn().mockResolvedValue(undefined)

    await controller.handleServerEvent({ state: 'Recovering', attempt: 1 })
    await controller.handleServerEvent({ state: 'Ready', port: 42100 })

    // No fresh loadURL because port did not change
    expect(win.loadURL).not.toHaveBeenCalled()
    expect(win.webContents.removeInsertedCSS).toHaveBeenCalledWith('css-key-123')
    expect(win.webContents.executeJavaScript).toHaveBeenCalledWith(
      expect.stringContaining('.remove()'),
    )
  })

  it('Ready after Recovering (DIFFERENT port): fresh loadURL BEFORE banner removal (FR-6)', async () => {
    const win = createMockWindow()
    const controller = new WindowServingController(win, BOOT_PATH)

    await controller.handleServerEvent({ state: 'Ready', port: 42100 })
    await controller.handleServerEvent({ state: 'Recovering', attempt: 2 })

    win.loadURL = vi.fn().mockResolvedValue(undefined)
    await controller.handleServerEvent({ state: 'Ready', port: 45200 })

    // Fresh loadURL against new port
    expect(win.loadURL).toHaveBeenCalledWith('http://127.0.0.1:45200')
    expect(controller.getCurrentPort()).toBe(45200)
    expect(win.webContents.removeInsertedCSS).toHaveBeenCalledWith('css-key-123')
  })

  it('Failed: loads error asset variant via loadFile (FR-3 / FR-6)', async () => {
    const win = createMockWindow()
    const controller = new WindowServingController(win, BOOT_PATH)

    await controller.handleServerEvent({ state: 'Ready', port: 42100 })
    await controller.handleServerEvent({ state: 'Recovering', attempt: 3 })
    await controller.handleServerEvent({ state: 'Failed' })

    expect(win.loadFile).toHaveBeenCalledWith(
      BOOT_PATH,
      expect.objectContaining({
        query: { state: 'error' },
        hash: '#error',
      }),
    )
    expect(controller.isRealUiLoaded()).toBe(false)
  })
})
