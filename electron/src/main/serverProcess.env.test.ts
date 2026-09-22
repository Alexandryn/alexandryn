import { EventEmitter } from 'node:events'
import { Readable } from 'node:stream'
import { describe, expect, it, vi } from 'vitest'

// spawnServer's `env` option: unit-level (a mocked child_process.spawn),
// separate from serverProcess.test.ts's structural-source checks and
// lifecycle.test.ts's real-process integration tests, neither of which
// inspects spawn options.

const spawnMock = vi.hoisted(() => vi.fn())
vi.mock('node:child_process', () => ({ spawn: spawnMock }))

function fakeChild() {
  const child = new EventEmitter() as EventEmitter & {
    stdout: Readable
    stderr: Readable
  }
  // createInterface needs a real Readable — never emitting 'data' is fine,
  // these tests only check the spawn() call's arguments.
  child.stdout = new Readable({ read() {} })
  child.stderr = new Readable({ read() {} })
  return child
}

describe('spawnServer env option', () => {
  it('defaults to the current process env when none is given, as before', async () => {
    spawnMock.mockReturnValue(fakeChild())
    const { spawnServer } = await import('./serverProcess')
    spawnServer('/bin/x', '/cfg.toml')
    expect(spawnMock).toHaveBeenCalledWith(
      '/bin/x',
      expect.any(Array),
      expect.objectContaining({ env: process.env }),
    )
  })

  it('passes a given env through unchanged, so a caller can put the bundled', async () => {
    spawnMock.mockReturnValue(fakeChild())
    const { spawnServer } = await import('./serverProcess')
    const env = { ...process.env, PATH: '/pg/bin:/usr/bin' }
    spawnServer('/bin/x', '/cfg.toml', [], env)
    expect(spawnMock).toHaveBeenCalledWith(
      '/bin/x',
      expect.any(Array),
      expect.objectContaining({ env }),
    )
  })
})
