// Server readiness polling and continuous health monitoring.
// GET http://127.0.0.1:<port>/healthz (liveness)
// GET http://127.0.0.1:<port>/readyz (readiness & database operational status)
// Uses only Node's built-in `http` module — no extra dependencies.

import * as http from 'node:http'

/** Interval between readiness poll attempts (250ms). */
export const POLL_INTERVAL_MS = 250

/** Timeout before moving to Failed state (15s). */
export const READINESS_TIMEOUT_MS = 15_000

export type HealthState = 'healthy' | 'degraded' | 'unreachable'

export interface HealthStatus {
  state: HealthState
  live: boolean
  ready: boolean
  statusCode?: number
  message?: string
}

function httpGet(url: string): Promise<{ statusCode?: number; body: string }> {
  return new Promise((resolve, reject) => {
    const req = http.get(url, (res) => {
      let data = ''
      res.setEncoding('utf8')
      res.on('data', (chunk) => {
        data += chunk
      })
      res.on('end', () => {
        resolve({ statusCode: res.statusCode, body: data })
      })
    })
    req.on('error', reject)
    req.end()
  })
}

function extractErrorMessage(body: string): string | undefined {
  try {
    const parsed = JSON.parse(body) as { error?: { message?: string } }
    if (typeof parsed?.error?.message === 'string') {
      return parsed.error.message
    }
  } catch {
    // not JSON
  }
  return undefined
}

/**
 * Checks server health against both /healthz (liveness) and /readyz (readiness).
 *
 * - /healthz fails -> unreachable (process down or not accepting connections)
 * - /healthz 200, /readyz 200 -> healthy
 * - /healthz 200, /readyz non-200 -> degraded (alive, but DB unready or connection lost)
 */
export async function checkHealth(port: number): Promise<HealthStatus> {
  try {
    const healthz = await httpGet(`http://127.0.0.1:${port}/healthz`)
    if (healthz.statusCode !== 200) {
      return {
        state: 'unreachable',
        live: false,
        ready: false,
        statusCode: healthz.statusCode,
        message: `Liveness check returned HTTP ${healthz.statusCode}`,
      }
    }
  } catch (err) {
    return {
      state: 'unreachable',
      live: false,
      ready: false,
      message: (err as Error).message,
    }
  }

  try {
    const readyz = await httpGet(`http://127.0.0.1:${port}/readyz`)
    if (readyz.statusCode === 200) {
      return {
        state: 'healthy',
        live: true,
        ready: true,
        statusCode: 200,
      }
    }

    const msg =
      extractErrorMessage(readyz.body) || `Readiness check returned HTTP ${readyz.statusCode}`
    return {
      state: 'degraded',
      live: true,
      ready: false,
      statusCode: readyz.statusCode,
      message: msg,
    }
  } catch (err) {
    return {
      state: 'degraded',
      live: true,
      ready: false,
      message: (err as Error).message,
    }
  }
}

/**
 * Polls `http://127.0.0.1:<port>` until both /healthz and /readyz respond HTTP 200
 * or timeoutMs elapses.
 *
 * Resolves when the server is ready. Rejects with an error whose message
 * starts with "Readiness timeout" if the timeout expires first.
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

    async function attempt() {
      if (settled) return

      const elapsed = Date.now() - started
      if (elapsed >= timeoutMs) {
        settle(
          new Error(
            `Readiness timeout: server did not respond on port ${port} within ${timeoutMs}ms`,
          ),
        )
        return
      }

      const status = await checkHealth(port)
      if (settled) return

      if (status.ready) {
        settle()
        return
      }

      const remaining = timeoutMs - (Date.now() - started)
      if (remaining <= 0) {
        const detail = status.message ? ` (${status.message})` : ''
        settle(
          new Error(
            `Readiness timeout: server did not become ready on port ${port} within ${timeoutMs}ms${detail}`,
          ),
        )
        return
      }

      timer = setTimeout(attempt, Math.min(intervalMs, remaining))
    }

    void attempt()
  })
}

export interface HealthMonitorOptions {
  intervalMs?: number
  onStateChange?: (status: HealthStatus) => void
}

export interface HealthMonitor {
  stop(): void
}

/**
 * Starts continuous health monitoring for a running server.
 * Polls at `intervalMs` and calls `onStateChange` whenever the health state shifts.
 */
export function startHealthMonitor(
  port: number,
  options: HealthMonitorOptions = {},
): HealthMonitor {
  const intervalMs = options.intervalMs ?? 3000
  let isStopped = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let lastState: HealthState | undefined

  async function check() {
    if (isStopped) return
    const status = await checkHealth(port)
    if (isStopped) return

    if (status.state !== lastState) {
      lastState = status.state
      options.onStateChange?.(status)
    }

    if (!isStopped) {
      timer = setTimeout(check, intervalMs)
    }
  }

  timer = setTimeout(check, intervalMs)

  return {
    stop() {
      isStopped = true
      if (timer !== undefined) {
        clearTimeout(timer)
      }
    },
  }
}
