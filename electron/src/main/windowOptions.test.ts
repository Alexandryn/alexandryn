import { describe, expect, it } from 'vitest'
import {
  DEFAULT_HEIGHT,
  DEFAULT_WIDTH,
  getBrowserWindowOptions,
  MIN_HEIGHT,
  MIN_WIDTH,
} from './windowOptions'

// desktop-host-window-and-serving.md FR-1 — BrowserWindow construction options.
// Security flags (contextIsolation, sandbox, nodeIntegration), platform titleBarStyle,
// minWidth/minHeight floor, default dimensions, bounds restoration.

describe('getBrowserWindowOptions (FR-1)', () => {
  it('enforces the three mandatory security flags on webPreferences', () => {
    const opts = getBrowserWindowOptions()
    expect(opts.webPreferences).toBeDefined()
    expect(opts.webPreferences?.contextIsolation).toBe(true)
    expect(opts.webPreferences?.sandbox).toBe(true)
    expect(opts.webPreferences?.nodeIntegration).toBe(false)
  })

  it('sets minWidth to 768 (reflow breakpoint) and minHeight floor to 500', () => {
    const opts = getBrowserWindowOptions()
    expect(opts.minWidth).toBe(MIN_WIDTH)
    expect(opts.minWidth).toBe(768)
    expect(opts.minHeight).toBe(MIN_HEIGHT)
    expect(opts.minHeight).toBe(500)
  })

  it('uses default dimensions 1024x720 when no saved bounds provided', () => {
    const opts = getBrowserWindowOptions()
    expect(opts.width).toBe(DEFAULT_WIDTH)
    expect(opts.width).toBe(1024)
    expect(opts.height).toBe(DEFAULT_HEIGHT)
    expect(opts.height).toBe(720)
    expect(opts.show).toBe(false)
  })

  it('applies custom bounds when provided', () => {
    const opts = getBrowserWindowOptions({
      bounds: { width: 1200, height: 800, x: 100, y: 150 },
    })
    expect(opts.width).toBe(1200)
    expect(opts.height).toBe(800)
    expect(opts.x).toBe(100)
    expect(opts.y).toBe(150)
  })

  it('falls back to default dimensions when provided bounds are below minimum floor', () => {
    const opts = getBrowserWindowOptions({
      bounds: { width: 400, height: 300, x: 50, y: 50 },
    })
    expect(opts.width).toBe(DEFAULT_WIDTH)
    expect(opts.height).toBe(DEFAULT_HEIGHT)
  })


  it('macOS (darwin): titleBarStyle is hiddenInset', () => {
    const opts = getBrowserWindowOptions({ platform: 'darwin' })
    expect(opts.titleBarStyle).toBe('hiddenInset')
  })

  it('Linux / Windows: default native titlebar (titleBarStyle undefined)', () => {
    const linuxOpts = getBrowserWindowOptions({ platform: 'linux' })
    expect(linuxOpts.titleBarStyle).toBeUndefined()

    const winOpts = getBrowserWindowOptions({ platform: 'win32' })
    expect(winOpts.titleBarStyle).toBeUndefined()
  })

  it('sets custom preload path when specified', () => {
    const opts = getBrowserWindowOptions({ preloadPath: '/custom/preload.js' })
    expect(opts.webPreferences?.preload).toBe('/custom/preload.js')
  })
})
