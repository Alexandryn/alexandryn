import { afterEach, describe, expect, it, vi } from 'vitest'
import { ipcMain } from 'electron'
import { registerIpcHandlers, unregisterIpcHandlers } from './ipc'
import {
  OPERATIONS,
  SOURCE_PICK_LOCAL_FOLDER,
  SYSTEM_GET_APP_VERSION,
} from '../shared/operations'

// desktop-host-ipc-surface.md FR-2, FR-3, FR-5, FR-6.
// Tests for main process IPC registration and handler execution.

// Mock ipcMain
const handlers = new Map<string, (event: unknown, ...args: unknown[]) => Promise<unknown>>()

vi.mock('electron', () => ({
  app: {
    getVersion: () => '1.2.3',
  },
  dialog: {
    showOpenDialog: vi.fn(),
  },
  ipcMain: {
    handle: vi.fn((channel: string, listener: (event: unknown, ...args: unknown[]) => Promise<unknown>) => {
      handlers.set(channel, listener)
    }),
    removeHandler: vi.fn((channel: string) => {
      handlers.delete(channel)
    }),
  },
}))

afterEach(() => {
  unregisterIpcHandlers()
  handlers.clear()
  vi.clearAllMocks()
})

describe('registerIpcHandlers (FR-2 / FR-3)', () => {
  it('registers an ipcMain handler for every declared operation in OPERATIONS', () => {
    registerIpcHandlers()

    for (const op of OPERATIONS) {
      expect(ipcMain.handle).toHaveBeenCalledWith(op.name, expect.any(Function))
      expect(handlers.has(op.name)).toBe(true)
    }
  })

  it('system.getAppVersion returns the version string on valid call (FR-5 / E21)', async () => {
    registerIpcHandlers({ getAppVersion: () => '0.4.0' })
    const handler = handlers.get(SYSTEM_GET_APP_VERSION.name)!
    expect(handler).toBeDefined()

    const result = await handler({}, undefined)
    expect(result).toBe('0.4.0')
  })

  it('source.pickLocalFolder returns path when user selects a directory (FR-6 / E22)', async () => {
    const mockShowOpenDialog = vi.fn().mockResolvedValue({
      canceled: false,
      filePaths: ['/home/user/Books/MyLibrary'],
    })

    registerIpcHandlers({ showOpenDialog: mockShowOpenDialog })
    const handler = handlers.get(SOURCE_PICK_LOCAL_FOLDER.name)!
    expect(handler).toBeDefined()

    const result = await handler({}, undefined)
    expect(result).toEqual({ path: '/home/user/Books/MyLibrary' })
    expect(mockShowOpenDialog).toHaveBeenCalledWith({
      properties: ['openDirectory'],
    })
  })

  it('source.pickLocalFolder returns null when user cancels dialog (FR-6 / E22)', async () => {
    const mockShowOpenDialog = vi.fn().mockResolvedValue({
      canceled: true,
      filePaths: [],
    })

    registerIpcHandlers({ showOpenDialog: mockShowOpenDialog })
    const handler = handlers.get(SOURCE_PICK_LOCAL_FOLDER.name)!
    expect(handler).toBeDefined()

    const result = await handler({}, undefined)
    expect(result).toBeNull()
  })

  it('rejects hostile / unexpected arguments with generic error before business logic (FR-3 / E23)', async () => {
    const mockShowOpenDialog = vi.fn().mockResolvedValue({
      canceled: false,
      filePaths: ['/some/path'],
    })

    registerIpcHandlers({ showOpenDialog: mockShowOpenDialog })
    const handler = handlers.get(SOURCE_PICK_LOCAL_FOLDER.name)!

    // Hostile call attempting to pass an arbitrary path or object
    await expect(handler({}, { path: '/etc/passwd' })).rejects.toThrow('Invalid arguments')
    await expect(handler({}, 'malicious-string')).rejects.toThrow('Invalid arguments')
    await expect(handler({}, 12345)).rejects.toThrow('Invalid arguments')

    // Dialog must NEVER be called when argument shape is rejected
    expect(mockShowOpenDialog).not.toHaveBeenCalled()
  })
})
