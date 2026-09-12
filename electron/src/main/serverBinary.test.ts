import { join } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'

// One function resolves the Go server binary path:
// dev reads <repo-root>/bin/alexandryn-server, a packaged build reads
// process.resourcesPath/server/<platformBinaryName()>. `.exe` on Windows,
// nothing elsewhere. One call site (the spawn call), never re-derived.

const hoisted = vi.hoisted(() => ({
  isPackaged: false,
  appPath: '/repo/electron',
}))

vi.mock('electron', () => ({
  app: {
    get isPackaged() {
      return hoisted.isPackaged
    },
    getAppPath: () => hoisted.appPath,
  },
}))

afterEach(() => {
  vi.resetModules()
  hoisted.isPackaged = false
  hoisted.appPath = '/repo/electron'
  delete (process as { resourcesPath?: string }).resourcesPath
})

async function load(platform: NodeJS.Platform) {
  vi.stubGlobal('process', { ...process, platform })
  const mod = await import('./serverBinary')
  return mod
}

describe('resolveServerBinaryPath', () => {
  it('dev: <repo-root>/bin/alexandryn-server, repo root one level above the electron package', async () => {
    hoisted.isPackaged = false
    hoisted.appPath = '/repo/electron'
    const { resolveServerBinaryPath } = await load('linux')
    expect(resolveServerBinaryPath()).toBe(join('/repo', 'bin', 'alexandryn-server'))
  })

  it('dev on Windows: appends .exe', async () => {
    hoisted.isPackaged = false
    const { resolveServerBinaryPath } = await load('win32')
    expect(resolveServerBinaryPath()).toBe(join('/repo', 'bin', 'alexandryn-server.exe'))
  })

  it('packaged: process.resourcesPath/server/alexandryn-server', async () => {
    hoisted.isPackaged = true
    ;(process as { resourcesPath?: string }).resourcesPath =
      '/Applications/Alexandryn.app/Contents/Resources'
    const { resolveServerBinaryPath } = await load('darwin')
    expect(resolveServerBinaryPath()).toBe(
      join('/Applications/Alexandryn.app/Contents/Resources', 'server', 'alexandryn-server'),
    )
  })

  it('packaged on Windows: server/alexandryn-server.exe', async () => {
    hoisted.isPackaged = true
    ;(process as { resourcesPath?: string }).resourcesPath =
      'C:\\Program Files\\Alexandryn\\resources'
    const { resolveServerBinaryPath } = await load('win32')
    expect(resolveServerBinaryPath()).toBe(
      join('C:\\Program Files\\Alexandryn\\resources', 'server', 'alexandryn-server.exe'),
    )
  })

  it('packaged but resourcesPath missing: throws rather than returning a broken path', async () => {
    hoisted.isPackaged = true
    const { resolveServerBinaryPath } = await load('linux')
    expect(() => resolveServerBinaryPath()).toThrow(/resourcesPath/)
  })
})
