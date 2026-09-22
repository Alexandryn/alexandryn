import { mkdirSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'

// Bundled PostgreSQL (ADR 0007) is located the same way the Go server binary
// is (serverBinary.ts): dev reads <repo-root>/electron/resources/postgres/bin,
// a packaged build reads process.resourcesPath/postgres/bin. Absence is not
// an error — it means "rely on whatever postgres/initdb the system already
// has on PATH", which is the only path today's dev environment and CI use.

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

const dirs: string[] = []
afterEach(() => {
  vi.resetModules()
  hoisted.isPackaged = false
  hoisted.appPath = '/repo/electron'
  delete (process as { resourcesPath?: string }).resourcesPath
  for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true })
})

function realDir(...segments: string[]): string {
  const root = mkdtempSync(join(tmpdir(), 'pgbin-'))
  dirs.push(root)
  const full = join(root, ...segments)
  mkdirSync(full, { recursive: true })
  return root
}

async function load() {
  return import('./postgresBinaries')
}

describe('resolvePostgresBinDir', () => {
  it('dev: <electron-package>/resources/postgres/bin, when that directory exists', async () => {
    const root = realDir('electron', 'resources', 'postgres', 'bin')
    hoisted.isPackaged = false
    hoisted.appPath = join(root, 'electron')
    const { resolvePostgresBinDir } = await load()
    expect(resolvePostgresBinDir()).toBe(join(root, 'electron', 'resources', 'postgres', 'bin'))
  })

  it('dev: undefined when the directory does not exist — falls back to the system PATH', async () => {
    hoisted.isPackaged = false
    hoisted.appPath = '/definitely/not/a/real/path/electron'
    const { resolvePostgresBinDir } = await load()
    expect(resolvePostgresBinDir()).toBeUndefined()
  })

  it('packaged: process.resourcesPath/postgres/bin, when that directory exists', async () => {
    const root = realDir('postgres', 'bin')
    hoisted.isPackaged = true
    ;(process as { resourcesPath?: string }).resourcesPath = root
    const { resolvePostgresBinDir } = await load()
    expect(resolvePostgresBinDir()).toBe(join(root, 'postgres', 'bin'))
  })

  it('packaged: undefined when resourcesPath is unset or the directory was not bundled', async () => {
    hoisted.isPackaged = true
    const { resolvePostgresBinDir } = await load()
    expect(resolvePostgresBinDir()).toBeUndefined()
  })
})

describe('pathWithPostgresBinFirst', () => {
  it('prepends the bin dir to PATH using the platform delimiter', async () => {
    const { pathWithPostgresBinFirst } = await load()
    expect(pathWithPostgresBinFirst('/pg/bin', '/usr/bin:/bin', ':')).toBe('/pg/bin:/usr/bin:/bin')
  })

  it('is just the bin dir when PATH was empty or unset', async () => {
    const { pathWithPostgresBinFirst } = await load()
    expect(pathWithPostgresBinFirst('/pg/bin', undefined, ':')).toBe('/pg/bin')
    expect(pathWithPostgresBinFirst('/pg/bin', '', ':')).toBe('/pg/bin')
  })
})
