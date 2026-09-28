import { createServer, type RequestListener } from 'node:http'
import { afterEach, describe, expect, it } from 'vitest'
import {
  checkHealth,
  POLL_INTERVAL_MS,
  READINESS_TIMEOUT_MS,
  startHealthMonitor,
} from './healthPoller'

describe('healthPoller constants', () => {
  it('POLL_INTERVAL_MS is 250ms — frequent enough for near-instant ready transition', () => {
    expect(POLL_INTERVAL_MS).toBe(250)
  })

  it('READINESS_TIMEOUT_MS is 15 000ms — default readiness timeout floor', () => {
    expect(READINESS_TIMEOUT_MS).toBe(15_000)
  })
})

describe('checkHealth', () => {
  const servers: ReturnType<typeof createServer>[] = []

  afterEach(async () => {
    for (const server of servers.splice(0)) {
      await new Promise<void>((resolve) => server.close(() => resolve()))
    }
  })

  function listen(handler: RequestListener): Promise<number> {
    const server = createServer(handler)
    servers.push(server)
    return new Promise<number>((resolve) => {
      server.listen(0, '127.0.0.1', () => {
        const addr = server.address()
        resolve(typeof addr === 'object' && addr ? addr.port : 0)
      })
    })
  }

  it('reports healthy when both /healthz and /readyz return 200', async () => {
    const port = await listen((_req, res) => {
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end('{}')
    })

    const status = await checkHealth(port)
    expect(status.state).toBe('healthy')
    expect(status.live).toBe(true)
    expect(status.ready).toBe(true)
  })

  it('reports degraded when /healthz is 200 but /readyz is 503', async () => {
    const port = await listen((req, res) => {
      if (req.url === '/healthz') {
        res.writeHead(200, { 'Content-Type': 'application/json' })
        res.end('{"status":"ok"}')
      } else {
        res.writeHead(503, { 'Content-Type': 'application/json' })
        res.end('{"error":{"code":"Unavailable","message":"database unreachable"}}')
      }
    })

    const status = await checkHealth(port)
    expect(status.state).toBe('degraded')
    expect(status.live).toBe(true)
    expect(status.ready).toBe(false)
    expect(status.message).toBe('database unreachable')
  })

  it('reports unreachable when port has nothing listening', async () => {
    // Port 1 is guaranteed closed
    const status = await checkHealth(1)
    expect(status.state).toBe('unreachable')
    expect(status.live).toBe(false)
    expect(status.ready).toBe(false)
  })

  it('reports unreachable when /healthz returns non-200', async () => {
    const port = await listen((_req, res) => {
      res.writeHead(500)
      res.end('fatal')
    })

    const status = await checkHealth(port)
    expect(status.state).toBe('unreachable')
    expect(status.live).toBe(false)
    expect(status.ready).toBe(false)
  })
})

describe('startHealthMonitor', () => {
  const servers: ReturnType<typeof createServer>[] = []

  afterEach(async () => {
    for (const server of servers.splice(0)) {
      await new Promise<void>((resolve) => server.close(() => resolve()))
    }
  })

  it('calls onStateChange when health transitions from healthy to degraded', async () => {
    let isDegraded = false
    const port = await new Promise<number>((resolve) => {
      const server = createServer((req, res) => {
        if (req.url === '/healthz') {
          res.writeHead(200, { 'Content-Type': 'application/json' })
          res.end('{}')
        } else if (req.url === '/readyz') {
          if (isDegraded) {
            res.writeHead(503, { 'Content-Type': 'application/json' })
            res.end('{"error":{"code":"Unavailable","message":"lost the connection"}}')
          } else {
            res.writeHead(200, { 'Content-Type': 'application/json' })
            res.end('{}')
          }
        }
      })
      servers.push(server)
      server.listen(0, '127.0.0.1', () => {
        const addr = server.address()
        resolve(typeof addr === 'object' && addr ? addr.port : 0)
      })
    })

    const states: string[] = []
    let onStatePromiseResolve: () => void
    const transitionPromise = new Promise<void>((r) => (onStatePromiseResolve = r))

    const monitor = startHealthMonitor(port, {
      intervalMs: 50,
      onStateChange: (status) => {
        states.push(status.state)
        if (status.state === 'degraded') {
          onStatePromiseResolve()
        }
      },
    })

    // Wait a brief moment for initial healthy state check
    await new Promise((r) => setTimeout(r, 80))
    expect(states).toContain('healthy')

    // Transition to degraded
    isDegraded = true
    await transitionPromise
    monitor.stop()

    expect(states).toEqual(['healthy', 'degraded'])
  })
})
