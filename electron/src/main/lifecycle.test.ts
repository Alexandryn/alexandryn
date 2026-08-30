import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { spawnServer } from './serverProcess'
import { pollUntilReady } from './healthPoller'
import { SERVER_SHUTDOWN_GRACE_MS } from './serverConfig'

// Integration tests for E9 (spawnServer), E10 (pollUntilReady), E11 (shutdown).
// All run against the Go test binary built at electron/test-helpers/test-server/.
// Timing is real wall-clock — the test binary is fast (<200ms startup).
//
// desktop-host-process-model.md FR-2, FR-3, FR-5, Test strategy.

const TEST_SERVER = join(import.meta.dirname, '../../test-helpers/test-server/test-server')

// Track child processes to ensure cleanup even on test failure.
const children: ReturnType<typeof spawnServer>[] = []

afterEach(async () => {
  for (const { child } of children.splice(0)) {
    if (!child.killed && child.exitCode === null) {
      child.kill('SIGTERM')
      await new Promise<void>((resolve) => child.once('exit', resolve))
    }
  }
})

function spawn(args: string[] = []) {
  const s = spawnServer(TEST_SERVER, '/dev/null', args)
  children.push(s)
  return s
}

describe('spawnServer integration (E9 / FR-2)', () => {
  it('announces PORT=<n> on stdout and resolves portPromise', async () => {
    const { portPromise } = spawn()
    const port = await portPromise
    expect(port).toBeGreaterThan(0)
    expect(port).toBeLessThan(65536)
  })

  it('rejects portPromise when process exits before announcing a port', async () => {
    const { portPromise } = spawn(['--exit-before-ready'])
    await expect(portPromise).rejects.toThrow(/before announcing a port/)
  })
})

describe('pollUntilReady integration (E10 / FR-3)', () => {
  it('resolves when the server is ready', async () => {
    const { portPromise } = spawn()
    const port = await portPromise
    await expect(pollUntilReady(port, { timeoutMs: 5000 })).resolves.toBeUndefined()
  })

  it('rejects with "Readiness timeout" when nothing is listening', async () => {
    // Port 1 is reserved and guaranteed to refuse connections.
    await expect(
      pollUntilReady(1, { intervalMs: 50, timeoutMs: 200 }),
    ).rejects.toThrow(/Readiness timeout/)
  })
})

describe('shutdown integration (E11 / FR-5)', () => {
  it('SIGTERM causes the server to exit cleanly', async () => {
    const { child, portPromise } = spawn()
    await portPromise // wait until ready
    child.kill('SIGTERM')
    const code = await new Promise<number | null>((resolve) =>
      child.once('exit', (c) => resolve(c)),
    )
    expect(code).toBe(0)
  })

  it('SERVER_SHUTDOWN_GRACE_MS is 10 000 (matches the Go compiled default FR-5)', () => {
    expect(SERVER_SHUTDOWN_GRACE_MS).toBe(10_000)
  })

  it('slow-shutdown binary: SIGKILL after grace period expires', async () => {
    // The test binary ignores SIGTERM for 2s. We set a 1s grace → expect SIGKILL.
    const { child, portPromise } = spawn(['--slow-shutdown', '2'])
    await portPromise

    const startMs = Date.now()

    // Mimic E11 shutdown: SIGTERM, wait graceMs, then SIGKILL.
    const graceMs = 1000 // shortened from the real 10s for test speed
    child.kill('SIGTERM')

    const exitCode = await new Promise<number | null>((resolve) => {
      const killerTimer = setTimeout(() => {
        child.kill('SIGKILL')
      }, graceMs)
      child.once('exit', (c) => {
        clearTimeout(killerTimer)
        resolve(c)
      })
    })

    const elapsed = Date.now() - startMs
    // Must have been killed within ~1.5s (grace=1s + fuzz), not 2s.
    expect(elapsed).toBeLessThan(1800)
    // SIGKILL exit code is null on POSIX (child.exitCode is null, signal is SIGKILL).
    // exitCode null = killed by signal.
    expect(exitCode).toBeNull()
  })
})
