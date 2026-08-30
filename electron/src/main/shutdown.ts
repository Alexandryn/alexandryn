import type { ChildProcess } from 'node:child_process'
import { SERVER_SHUTDOWN_GRACE_MS } from './serverConfig'

// desktop-host-process-model.md FR-5 — shutdown sequence.
// Extracted from index.ts so it can be unit-tested without importing the
// Electron-dependent top-level side-effects in index.ts.

/**
 * Shuts down the Go server child process gracefully.
 *
 * desktop-host-process-model.md FR-5:
 * - Sends SIGTERM; waits up to `graceMs` for the child to exit.
 * - If the grace period expires, sends SIGKILL.
 * - Resolves once the child has exited (by either signal).
 *
 * @param child   - The spawned Go server process.
 * @param graceMs - Grace period before SIGKILL (default: SERVER_SHUTDOWN_GRACE_MS = 10s).
 */
export function shutdownServer(child: ChildProcess, graceMs = SERVER_SHUTDOWN_GRACE_MS): Promise<void> {
  return new Promise<void>((resolve) => {
    if (child.exitCode !== null || child.killed) {
      resolve()
      return
    }

    let resolved = false
    function done() {
      if (resolved) return
      resolved = true
      clearTimeout(killTimer)
      resolve()
    }

    child.once('exit', done)

    child.kill('SIGTERM')

    const killTimer = setTimeout(() => {
      if (child.exitCode === null && !child.killed) {
        child.kill('SIGKILL')
      }
    }, graceMs)
  })
}
