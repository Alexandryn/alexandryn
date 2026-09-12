import { readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  DEFAULT_STATE_DEBOUNCE_MS,
  loadWindowState,
  parseWindowState,
  saveWindowState,
  trackWindowState,
} from './windowState'

// Window-state persistence.
// {width, height, x, y} JSON stored in userData.
// Debounced 500ms on resize/move.
// Corrupted/missing state file falls back to default size, never crashes.

const tempFiles: string[] = []

function tempStateFile(): string {
  const file = join(tmpdir(), `alexandryn-test-window-state-${Date.now()}-${Math.random().toString(36).slice(2)}.json`)
  tempFiles.push(file)
  return file
}

afterEach(async () => {
  await Promise.all(tempFiles.splice(0).map((f) => rm(f, { force: true })))
})

describe('parseWindowState (pure parser)', () => {
  it('parses valid JSON with width, height, x, y', () => {
    const json = JSON.stringify({ width: 1200, height: 800, x: 100, y: 150 })
    const state = parseWindowState(json)
    expect(state).toEqual({ width: 1200, height: 800, x: 100, y: 150 })
  })

  it('returns undefined for invalid JSON (corrupted file fallback)', () => {
    expect(parseWindowState('{corrupt json')).toBeUndefined()
    expect(parseWindowState('')).toBeUndefined()
  })

  it('returns undefined if width is below MIN_WIDTH (768)', () => {
    const json = JSON.stringify({ width: 500, height: 800, x: 100, y: 150 })
    expect(parseWindowState(json)).toBeUndefined()
  })

  it('returns undefined if height is below MIN_HEIGHT (500)', () => {
    const json = JSON.stringify({ width: 1000, height: 300, x: 100, y: 150 })
    expect(parseWindowState(json)).toBeUndefined()
  })

  it('returns undefined if fields are non-numeric, NaN, Infinity, or missing', () => {
    expect(parseWindowState(JSON.stringify({ width: '1000', height: 800 }))).toBeUndefined()
    expect(parseWindowState(JSON.stringify({ width: 1000 }))).toBeUndefined()
    expect(parseWindowState(JSON.stringify({ width: null, height: 800, x: 0, y: 0 }))).toBeUndefined()
    expect(parseWindowState(JSON.stringify({ width: 1000, height: 800, x: null, y: 0 }))).toBeUndefined()
    expect(parseWindowState(JSON.stringify([]))).toBeUndefined()
    expect(parseWindowState('{"width": "NaN", "height": 800, "x": 0, "y": 0}')).toBeUndefined()
    expect(parseWindowState('{"width": 1000, "height": 800, "x": 1e999, "y": 0}')).toBeUndefined()
  })

})

describe('loadWindowState / saveWindowState (I/O)', () => {
  it('loads saved state correctly after writing', async () => {
    const file = tempStateFile()
    await saveWindowState(file, { width: 1280, height: 900, x: 50, y: 75 })
    const state = await loadWindowState(file)
    expect(state).toEqual({ width: 1280, height: 900, x: 50, y: 75 })
  })

  it('returns undefined and does not throw if file does not exist', async () => {
    const file = join(tmpdir(), 'non-existent-window-state.json')
    const state = await loadWindowState(file)
    expect(state).toBeUndefined()
  })

  it('returns undefined and does not throw if file contains corrupted data', async () => {
    const file = tempStateFile()
    await writeFile(file, 'not valid json!!!', 'utf8')
    const state = await loadWindowState(file)
    expect(state).toBeUndefined()
  })
})

describe('trackWindowState (debounced persistence)', () => {
  it('debounces writes by DEFAULT_STATE_DEBOUNCE_MS (500ms)', async () => {
    vi.useFakeTimers()
    const file = tempStateFile()

    let bounds = { width: 1100, height: 750, x: 10, y: 20 }
    const listeners: Record<string, () => void> = {}

    const mockWin = {
      isDestroyed: () => false,
      getBounds: () => bounds,
      on: (event: string, fn: () => void) => {
        listeners[event] = fn
      },
      removeListener: vi.fn(),
    } as unknown as import('electron').BrowserWindow

    const tracker = trackWindowState(mockWin, file, 500)

    // Trigger multiple resize/move events rapidly
    bounds = { width: 1150, height: 780, x: 15, y: 25 }
    listeners['resize']?.()
    listeners['move']?.()
    bounds = { width: 1200, height: 800, x: 20, y: 30 }
    listeners['resize']?.()

    // Before debounce elapses, file not written yet
    vi.advanceTimersByTime(400)
    expect(await readFile(file, 'utf8').catch(() => null)).toBeNull()

    // Advance past debounce
    vi.advanceTimersByTime(150)

    // File should now be written with the latest bounds
    const content = await readFile(file, 'utf8')
    expect(JSON.parse(content)).toEqual({ width: 1200, height: 800, x: 20, y: 30 })

    tracker.dispose()
    vi.useRealTimers()
  })

  it('DEFAULT_STATE_DEBOUNCE_MS is 500ms', () => {
    expect(DEFAULT_STATE_DEBOUNCE_MS).toBe(500)
  })
})
