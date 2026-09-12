// Server readiness polling.
// GET http://127.0.0.1:<port>/healthz at 250ms intervals until 200 or timeout.
// Uses only Node's built-in `http` module — no extra dependencies.

import * as http from 'node:http'

/** Interval between poll attempts (250ms). */
export const POLL_INTERVAL_MS = 250

/** Timeout before moving to Failed state (15s). */
export const READINESS_TIMEOUT_MS = 15_000

/**
 * Polls `GET http://127.0.0.1:<port>/healthz` at {@link POLL_INTERVAL_MS}
 * intervals until the server responds with HTTP 200 or
 * {@link READINESS_TIMEOUT_MS} elapses.
 *
 * Resolves when the server is ready. Rejects with an error whose message
 * starts with "Readiness timeout" if the timeout expires first.
 *
 * @param port     - The port announced by the spawned server.
 * @param options  - Override interval/timeout for testing.
 */
export function pollUntilReady(
  port: number,
  options?: { intervalMs?: number; timeoutMs?: number },
): Promise<void> {
  const intervalMs = options?.intervalMs ?? POLL_INTERVAL_MS
  const timeoutMs = options?.timeoutMs ?? READINESS_TIMEOUT_MS

  return new Promise<void>((resolve, reject) => {
    const started = Date.now()
    let timer: ReturnType<typeof setTimeout> | undefined
    let settled = false

    function settle(err?: Error) {
      if (settled) return
      settled = true
      if (timer !== undefined) clearTimeout(timer)
      if (err !== undefined) reject(err)
      else resolve()
    }

    function attempt() {
      if (settled) return

      const elapsed = Date.now() - started
      if (elapsed >= timeoutMs) {
        settle(new Error(`Readiness timeout: server did not respond on port ${port} within ${timeoutMs}ms`))
        return
      }

      const req = http.get(`http://127.0.0.1:${port}/healthz`, (res) => {
        res.resume() // drain so the socket closes
        if (res.statusCode === 200) {
          settle()
        } else {
          // Non-200 means the server is up but not yet ready; poll again.
          timer = setTimeout(attempt, intervalMs)
        }
      })

      req.on('error', () => {
        // ECONNREFUSED or similar — server not up yet; poll again.
        if (!settled) {
          timer = setTimeout(attempt, intervalMs)
        }
      })

      req.end()
    }

    attempt()
  })
}
