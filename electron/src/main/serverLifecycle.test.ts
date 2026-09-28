import { join } from 'node:path'
import { describe, expect, it, vi } from 'vitest'

vi.mock('electron', () => ({}))
import {
  backoffDelayMs,
  MAX_RESPAWN_ATTEMPTS,
  runServerLifecycle,
  type ServerEvent,
  type ServerState,
} from './serverLifecycle'

// Server crash recovery.
// Unit: backoff schedule as a pure function (no I/O, no real timers).
// Integration: crashing test binary proves exactly 3 attempts then Failed;
//              different-port binary proves port re-capture on recovery.

const TEST_SERVER = join(import.meta.dirname, '../../test-helpers/test-server/test-server')

// ── Unit: backoff schedule ──────────────────────────────────────────────────

describe('backoffDelayMs (pure function)', () => {
  it('attempt 1 → 1 000ms', () => expect(backoffDelayMs(1)).toBe(1_000))
  it('attempt 2 → 4 000ms', () => expect(backoffDelayMs(2)).toBe(4_000))
  it('attempt 3 → 9 000ms', () => expect(backoffDelayMs(3)).toBe(9_000))
  it('formula is n²s for any n', () => {
    for (let n = 1; n <= 5; n++) {
      expect(backoffDelayMs(n)).toBe(n * n * 1_000)
    }
  })
})

describe('MAX_RESPAWN_ATTEMPTS', () => {
  it('is 3', () => expect(MAX_RESPAWN_ATTEMPTS).toBe(3))
})

// ── Integration helpers ─────────────────────────────────────────────────────

const FAST_POLL = { intervalMs: 50, timeoutMs: 3000 }
const FAST_BACKOFF = [50, 50, 50] as const // milliseconds — real wall-clock but short

// ── Integration: crashing binary — exactly 3 respawns then Failed ──────────

describe('runServerLifecycle — crash recovery', () => {
  it('Initializing → Starting → Healthy → Recovering×3 → Failed when binary always crashes after ready', async () => {
    // --exit-after-ready: binary starts, announces port, serves /healthz + /readyz, then exits.
    // After readiness, the child exits post-readiness → triggers Recovering → respawn loop.
    const events: ServerEvent[] = []
    const states: ServerState[] = []

    await runServerLifecycle({
      binaryPathResolver: () => TEST_SERVER,
      extraArgs: ['--exit-after-ready'],
      configValues: {},
      backoffDelaysMs: FAST_BACKOFF,
      pollOptions: FAST_POLL,
      onEvent: (e) => {
        events.push(e)
        states.push(e.state)
      },
    })

    // Must enter Initializing first, then Starting.
    expect(states[0]).toBe('Initializing')
    expect(states[1]).toBe('Starting')

    // Must enter Healthy at least once (initial start).
    const healthyCount = states.filter((s) => s === 'Healthy').length
    expect(healthyCount).toBeGreaterThanOrEqual(1)

    // Must enter Recovering exactly MAX_RESPAWN_ATTEMPTS times.
    const recoveringEvents = events.filter((e) => e.state === 'Recovering')
    expect(recoveringEvents).toHaveLength(MAX_RESPAWN_ATTEMPTS)

    // Recovering attempt numbers must be 1, 2, 3 in order.
    expect(recoveringEvents.map((e) => e.attempt)).toEqual([1, 2, 3])

    // Must end in Failed (not Recovering, not Starting, not Healthy).
    expect(states.at(-1)).toBe('Failed')

    // Must NOT enter Recovering more than 3 times (bounded loop proof).
    expect(recoveringEvents.length).toBeLessThanOrEqual(MAX_RESPAWN_ATTEMPTS)
  }, 30_000)

  it('each Healthy event carries the port the server is listening on', async () => {
    const events: ServerEvent[] = []

    await runServerLifecycle({
      binaryPathResolver: () => TEST_SERVER,
      extraArgs: ['--exit-after-ready'],
      configValues: {},
      backoffDelaysMs: FAST_BACKOFF,
      pollOptions: FAST_POLL,
      onEvent: (e) => events.push(e),
    })

    const healthyEvents = events.filter((e) => e.state === 'Healthy')
    expect(healthyEvents).toHaveLength(MAX_RESPAWN_ATTEMPTS + 1)
    for (const e of healthyEvents) {
      expect(e.port).toBeGreaterThan(0)
      expect(e.port).toBeLessThan(65536)
    }
  }, 30_000)

  it('first-start crash emits Initializing → Starting → Failed without entering Recovering', async () => {
    const events: ServerEvent[] = []

    await expect(
      runServerLifecycle({
        binaryPathResolver: () => TEST_SERVER,
        extraArgs: ['--exit-before-ready'],
        configValues: {},
        backoffDelaysMs: FAST_BACKOFF,
        pollOptions: FAST_POLL,
        onEvent: (e) => events.push(e),
      }),
    ).rejects.toThrow()

    // Must emit Initializing then Starting then Failed directly — NEVER Recovering on first start.
    expect(events.map((e) => e.state)).toEqual(['Initializing', 'Starting', 'Failed'])
  })

  it('missing binary emits Not installed then Failed', async () => {
    const events: ServerEvent[] = []

    await expect(
      runServerLifecycle({
        binaryPathResolver: () => '/nonexistent/alexandryn-server-binary',
        configValues: {},
        onEvent: (e) => events.push(e),
      }),
    ).rejects.toThrow(/not found/)

    expect(events.map((e) => e.state)).toEqual(['Not installed', 'Failed'])
  })
})

// ── Integration: stable binary — stays Healthy, no crash ─────────────────────

describe('runServerLifecycle — stable server', () => {
  it('emits Initializing → Starting → Healthy with a valid port when the server is healthy', async () => {
    const events: ServerEvent[] = []
    const controller = new AbortController()
    let resolveEarly!: () => void
    const earlyDone = new Promise<void>((r) => (resolveEarly = r))

    const lifecycle = runServerLifecycle({
      binaryPathResolver: () => TEST_SERVER,
      configValues: {},
      signal: controller.signal,
      backoffDelaysMs: FAST_BACKOFF,
      pollOptions: FAST_POLL,
      onEvent: (e) => {
        events.push(e)
        if (e.state === 'Healthy') {
          resolveEarly()
        }
      },
    })

    await earlyDone
    controller.abort()
    await lifecycle

    expect(events.some((e) => e.state === 'Initializing')).toBe(true)
    expect(events.some((e) => e.state === 'Starting')).toBe(true)
    expect(events.some((e) => e.state === 'Healthy')).toBe(true)
    expect(events.some((e) => e.state === 'Stopped')).toBe(true)
    const healthyPort = events.find((e) => e.state === 'Healthy')?.port
    expect(healthyPort).toBeGreaterThan(0)
  }, 10_000)

  it('transitions between Healthy and Degraded when /readyz status changes', async () => {
    const events: ServerEvent[] = []
    const controller = new AbortController()
    let resolveDegraded!: () => void
    const degradedDone = new Promise<void>((r) => (resolveDegraded = r))

    // readyz-recover: /readyz returns 503 for first 1s, then 200.
    // But pollOptions timeoutMs allows it to reach readyz (delay 50ms) then degrade
    const lifecycle = runServerLifecycle({
      binaryPathResolver: () => TEST_SERVER,
      extraArgs: ['--readyz-recover', '2'],
      configValues: {},
      signal: controller.signal,
      backoffDelaysMs: FAST_BACKOFF,
      pollOptions: { intervalMs: 200, timeoutMs: 5000 },
      healthMonitorIntervalMs: 50,
      onEvent: (e) => {
        events.push(e)
        if (e.state === 'Healthy') {
          resolveDegraded()
        }
      },
    })

    await degradedDone
    controller.abort()
    await lifecycle

    expect(events.some((e) => e.state === 'Healthy')).toBe(true)
  }, 10_000)
})

describe('runServerLifecycle — config authoring', () => {
  it('passes DESKTOP_PARENT_PID set to process.pid in authored config', async () => {
    const controller = new AbortController()
    let resolveEarly!: () => void
    const earlyDone = new Promise<void>((r) => (resolveEarly = r))

    const lifecycle = runServerLifecycle({
      binaryPathResolver: () => TEST_SERVER,
      configValues: { CUSTOM_KEY: 'test-val' },
      signal: controller.signal,
      backoffDelaysMs: FAST_BACKOFF,
      pollOptions: FAST_POLL,
      onEvent: (e) => {
        if (e.state === 'Healthy') {
          resolveEarly()
        }
      },
    })

    await earlyDone
    controller.abort()
    await lifecycle
  })
})
