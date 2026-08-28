import { readFile, stat } from 'node:fs/promises'
import { dirname } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import {
  SERVER_SHUTDOWN_GRACE_MS,
  writeServerConfig,
  type ServerConfigHandle,
} from './serverConfig'

// architecture-desktop-host.md FR-5 / desktop-host-process-model.md FR-5:
// configuration reaches the Go server through a file, not argv or an
// inherited env var. Owner-only (0600) permissions, created under an
// unpredictable directory (no fixed shared-temp path — TOCTOU / symlink
// surface), passed by path only, deleted once readiness succeeds or the
// startup timeout expires.

const handles: ServerConfigHandle[] = []

afterEach(async () => {
  await Promise.all(handles.splice(0).map((h) => h.cleanup()))
})

async function write(values: Record<string, string>) {
  const handle = await writeServerConfig(values)
  handles.push(handle)
  return handle
}

describe('writeServerConfig', () => {
  it('writes a 0600 file inside a private (0700) directory', async () => {
    const { path } = await write({ LOG_LEVEL: 'info' })
    expect((await stat(path)).mode & 0o777).toBe(0o600)
    expect((await stat(dirname(path))).mode & 0o777).toBe(0o700)
  })

  it('uses an unpredictable path — two calls never collide', async () => {
    const a = await write({})
    const b = await write({})
    expect(a.path).not.toBe(b.path)
    expect(dirname(a.path)).not.toBe(dirname(b.path))
  })

  it('serialises the values as TOML the Go loader reads', async () => {
    const { path } = await write({
      DATABASE_URL: 'postgres://u:p@127.0.0.1:5432/db',
      LOG_LEVEL: 'debug',
    })
    const toml = await readFile(path, 'utf8')
    expect(toml).toContain('DATABASE_URL = "postgres://u:p@127.0.0.1:5432/db"')
    expect(toml).toContain('LOG_LEVEL = "debug"')
  })

  it('an empty value set writes a valid (comment-only) file', async () => {
    const { path } = await write({})
    const toml = await readFile(path, 'utf8')
    expect(toml.trim().startsWith('#')).toBe(true)
  })

  it('cleanup deletes the file and its directory, and is safe to call twice', async () => {
    const handle = await write({ LOG_LEVEL: 'info' })
    await handle.cleanup()
    await expect(stat(handle.path)).rejects.toThrow()
    await expect(stat(dirname(handle.path))).rejects.toThrow()
    await expect(handle.cleanup()).resolves.toBeUndefined()
  })

  it('escapes a value containing a quote or backslash', async () => {
    const { path } = await write({ WEIRD: 'a"b\\c' })
    const toml = await readFile(path, 'utf8')
    expect(toml).toContain('WEIRD = "a\\"b\\\\c"')
  })
})

describe('SERVER_SHUTDOWN_GRACE_MS', () => {
  it('matches the Go compiled default (10s) — retained in memory for FR-5 shutdown', () => {
    expect(SERVER_SHUTDOWN_GRACE_MS).toBe(10_000)
  })
})
