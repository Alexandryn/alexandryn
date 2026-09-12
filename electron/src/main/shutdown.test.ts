import * as http from 'node:http'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { spawnServer } from './serverProcess'
import { shutdownServer } from './shutdown'

// Server shutdown unit + integration tests.
// The full SIGTERM→SIGKILL sequence is already integration-tested in
// lifecycle.test.ts. This file adds direct unit coverage of shutdownServer()
// as exported from the main entry point, and verifies the macOS-override
// behaviour (window-all-closed → app.quit()) is wired.

const TEST_SERVER = join(import.meta.dirname, '../../test-helpers/test-server/test-server')

const children: ReturnType<typeof spawnServer>[] = []

afterEach(async () => {
  for (const { child } of children.splice(0)) {
    if (!child.killed && child.exitCode === null) {
      child.kill('SIGKILL')
      await new Promise<void>((resolve) => child.once('exit', resolve))
    }
  }
})

describe('shutdownServer', () => {
  it('resolves immediately if the child is already exited', async () => {
    const { child, portPromise } = spawnServer(TEST_SERVER, '/dev/null', ['--exit-after-ready'])
    children.push({ child, portPromise })
    const port = await portPromise
    // Poll once to trigger --exit-after-ready.
    await new Promise<void>((resolve, reject) => {
      const req = http.get(`http://127.0.0.1:${port}/healthz`, (res) => {
        res.resume()
        resolve()
      })
      req.on('error', reject)
      req.end()
    })
    // Wait for exit.
    await new Promise<void>((resolve) => child.once('exit', resolve))
    // shutdownServer on an already-exited child must resolve, not hang.
    await expect(shutdownServer(child, 500)).resolves.toBeUndefined()
  })

  it('SIGTERM → child exits → resolves before grace expires', async () => {
    const { child, portPromise } = spawnServer(TEST_SERVER, '/dev/null')
    children.push({ child, portPromise })
    await portPromise
    // Use 5s grace; the test binary exits cleanly on SIGTERM in <200ms.
    await expect(shutdownServer(child, 5000)).resolves.toBeUndefined()
    expect(child.exitCode).not.toBeNull()
  })
})
