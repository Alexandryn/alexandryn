import { afterEach, describe, expect, it, vi } from 'vitest'

// Single-instance lock unit tests.
// These are unit tests with a mocked Electron API. The integration test
// (real second process) runs in the Playwright Electron suite.

const hoisted = vi.hoisted(() => ({
  isPrimary: true as boolean,
  windows: [] as Array<{ isMinimized: () => boolean; restore: () => void; focus: () => void }>,
  handlers: {} as Record<string, (...args: unknown[]) => void>,
}))

vi.mock('electron', () => ({
  app: {
    requestSingleInstanceLock: () => hoisted.isPrimary,
    on: (event: string, handler: (...args: unknown[]) => void) => {
      hoisted.handlers[event] = handler
    },
  },
  BrowserWindow: {
    getAllWindows: () => hoisted.windows,
  },
}))

afterEach(() => {
  vi.resetModules()
  hoisted.isPrimary = true
  hoisted.windows = []
  hoisted.handlers = {}
})

async function load() {
  return import('./singleInstance')
}

describe('acquireSingleInstanceLock', () => {
  it('returns true when this process is the primary instance', async () => {
    hoisted.isPrimary = true
    const { acquireSingleInstanceLock } = await load()
    expect(acquireSingleInstanceLock()).toBe(true)
  })

  it('returns false when another instance already holds the lock', async () => {
    hoisted.isPrimary = false
    const { acquireSingleInstanceLock } = await load()
    expect(acquireSingleInstanceLock()).toBe(false)
  })

  it('registers second-instance handler on the primary', async () => {
    hoisted.isPrimary = true
    const { acquireSingleInstanceLock } = await load()
    acquireSingleInstanceLock()
    expect(hoisted.handlers['second-instance']).toBeDefined()
  })

  it('does NOT register second-instance handler on the secondary (no window to manage)', async () => {
    hoisted.isPrimary = false
    const { acquireSingleInstanceLock } = await load()
    acquireSingleInstanceLock()
    expect(hoisted.handlers['second-instance']).toBeUndefined()
  })

  describe('second-instance handler', () => {
    it('focuses the existing window when it is not minimized', async () => {
      hoisted.isPrimary = true
      const focus = vi.fn()
      const restore = vi.fn()
      hoisted.windows = [{ isMinimized: () => false, restore, focus }]

      const { acquireSingleInstanceLock } = await load()
      acquireSingleInstanceLock()
      hoisted.handlers['second-instance']?.()

      expect(focus).toHaveBeenCalledOnce()
      expect(restore).not.toHaveBeenCalled()
    })

    it('restores then focuses when the window is minimized', async () => {
      hoisted.isPrimary = true
      const focus = vi.fn()
      const restore = vi.fn()
      hoisted.windows = [{ isMinimized: () => true, restore, focus }]

      const { acquireSingleInstanceLock } = await load()
      acquireSingleInstanceLock()
      hoisted.handlers['second-instance']?.()

      expect(restore).toHaveBeenCalledOnce()
      expect(focus).toHaveBeenCalledOnce()
    })

    it('does nothing when no window exists yet', async () => {
      hoisted.isPrimary = true
      hoisted.windows = []

      const { acquireSingleInstanceLock } = await load()
      acquireSingleInstanceLock()
      // Must not throw even with empty window list
      expect(() => hoisted.handlers['second-instance']?.()).not.toThrow()
    })
  })
})
