import { readFile, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { app } from 'electron'
import type { BrowserWindow } from 'electron'
import { MIN_HEIGHT, MIN_WIDTH, type WindowBounds } from './windowOptions'

// Window-state persistence.
// {width, height, x, y} JSON stored in app.getPath('userData').
// 500ms debounced write on resize/move.
// Corrupted or missing file falls back to default bounds without crashing.

export const DEFAULT_STATE_DEBOUNCE_MS = 500

/**
 * Returns the default file path for window state storage in userData.
 */
export function getWindowStatePath(): string {
  return join(app.getPath('userData'), 'window-state.json')
}

/**
 * Pure parser for window state JSON strings.
 * Returns undefined if JSON is corrupted, missing required fields, or below minimum size floor.
 */
export function parseWindowState(raw: string): WindowBounds | undefined {
  try {
    const data = JSON.parse(raw) as unknown
    if (typeof data !== 'object' || data === null || Array.isArray(data)) {
      return undefined
    }

    const { width, height, x, y } = data as Record<string, unknown>

    if (
      typeof width !== 'number' ||
      typeof height !== 'number' ||
      typeof x !== 'number' ||
      typeof y !== 'number' ||
      !Number.isFinite(width) ||
      !Number.isFinite(height) ||
      !Number.isFinite(x) ||
      !Number.isFinite(y)
    ) {
      return undefined
    }

    if (width < MIN_WIDTH || height < MIN_HEIGHT) {
      return undefined
    }

    return {
      width: Math.round(width),
      height: Math.round(height),
      x: Math.round(x),
      y: Math.round(y),
    }
  } catch {
    return undefined
  }
}

/**
 * Loads the saved window state from disk.
 * Returns undefined if file does not exist or content is corrupted.
 */
export async function loadWindowState(filePath: string): Promise<WindowBounds | undefined> {
  try {
    const content = await readFile(filePath, 'utf8')
    return parseWindowState(content)
  } catch {
    return undefined
  }
}

/**
 * Saves window bounds to disk as JSON.
 */
export async function saveWindowState(filePath: string, bounds: WindowBounds): Promise<void> {
  const json = JSON.stringify(bounds, null, 2)
  await writeFile(filePath, json, 'utf8')
}

export interface WindowStateTracker {
  dispose: () => void
}

/**
 * Listens to resize and move events on a BrowserWindow and saves bounds
 * debounced by debounceMs.
 */
export function trackWindowState(
  win: BrowserWindow,
  filePath: string,
  debounceMs = DEFAULT_STATE_DEBOUNCE_MS,
): WindowStateTracker {
  let timer: ReturnType<typeof setTimeout> | undefined

  function onBoundsChanged() {
    if (timer !== undefined) {
      clearTimeout(timer)
    }

    timer = setTimeout(() => {
      if (win.isDestroyed()) return
      const bounds = win.getBounds()
      if (bounds.width >= MIN_WIDTH && bounds.height >= MIN_HEIGHT) {
        void saveWindowState(filePath, bounds).catch(() => {
          // Ignore write errors; non-fatal persistence failure
        })
      }
    }, debounceMs)
  }

  win.on('resize', onBoundsChanged)
  win.on('move', onBoundsChanged)

  return {
    dispose() {
      if (timer !== undefined) {
        clearTimeout(timer)
      }
      if (!win.isDestroyed()) {
        win.removeListener('resize', onBoundsChanged)
        win.removeListener('move', onBoundsChanged)
      }
    },
  }
}
