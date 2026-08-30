import { join } from 'node:path'
import type { BrowserWindowConstructorOptions } from 'electron'

// desktop-host-window-and-serving.md FR-1 — BrowserWindow construction.
// Minimum width is the reflow breakpoint from web/src/breakpoints.ts (768).
// Minimum height is the responsive layout floor (500).
// Default window size is 1024x720.

export const MIN_WIDTH = 768
export const MIN_HEIGHT = 500
export const DEFAULT_WIDTH = 1024
export const DEFAULT_HEIGHT = 720

export interface WindowBounds {
  width?: number
  height?: number
  x?: number
  y?: number
}

export interface WindowOptionsParams {
  platform?: NodeJS.Platform
  preloadPath?: string
  bounds?: WindowBounds
}

/**
 * Produces BrowserWindowConstructorOptions conforming to desktop-host-window-and-serving.md FR-1.
 *
 * - Mandatory security flags: contextIsolation: true, sandbox: true, nodeIntegration: false
 * - macOS: titleBarStyle: 'hiddenInset'
 * - Linux/Windows: native titlebar
 * - minWidth: 768 (reflow token), minHeight: 500
 * - Default size: 1024x720, show: false (shown on ready-to-show)
 */
export function getBrowserWindowOptions(params?: WindowOptionsParams): BrowserWindowConstructorOptions {
  const platform = params?.platform ?? process.platform
  const preloadPath = params?.preloadPath ?? join(import.meta.dirname, '../preload/index.js')
  const bounds = params?.bounds

  const options: BrowserWindowConstructorOptions = {
    show: false,
    width: bounds?.width !== undefined && bounds.width >= MIN_WIDTH ? bounds.width : DEFAULT_WIDTH,
    height: bounds?.height !== undefined && bounds.height >= MIN_HEIGHT ? bounds.height : DEFAULT_HEIGHT,
    minWidth: MIN_WIDTH,
    minHeight: MIN_HEIGHT,
    autoHideMenuBar: true,
    webPreferences: {
      // architecture-desktop-host.md FR-2 — the privilege boundary
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      preload: preloadPath,
    },
  }

  if (bounds?.x !== undefined) {
    options.x = bounds.x
  }
  if (bounds?.y !== undefined) {
    options.y = bounds.y
  }

  if (platform === 'darwin') {
    options.titleBarStyle = 'hiddenInset'
  }

  return options
}
