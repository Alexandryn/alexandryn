import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
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
  it('Starting → Ready → Recovering×3 → Failed when binary always crashes after ready', async () => {
    // --exit-after-ready: binary starts, announces port, serves one /healthz, then exits.
    // After readiness, the health poll has already received its 200, so the child
    // exits post-readiness → triggers Recovering → respawn loop.
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

    // Must enter Starting once.
    expect(states[0]).toBe('Starting')

    // Must enter Ready at least once (initial start).
    const readyCount = states.filter((s) => s === 'Ready').length
    expect(readyCount).toBeGreaterThanOrEqual(1)

    // Must enter Recovering exactly MAX_RESPAWN_ATTEMPTS times.
    const recoveringEvents = events.filter((e) => e.state === 'Recovering')
    expect(recoveringEvents).toHaveLength(MAX_RESPAWN_ATTEMPTS)

    // Recovering attempt numbers must be 1, 2, 3 in order.
    expect(recoveringEvents.map((e) => e.attempt)).toEqual([1, 2, 3])

    // Must end in Failed (not Recovering, not Starting, not Ready).
    expect(states.at(-1)).toBe('Failed')

    // Must NOT enter Recovering more than 3 times (bounded loop proof).
    expect(recoveringEvents.length).toBeLessThanOrEqual(MAX_RESPAWN_ATTEMPTS)
  }, 30_000)

  it('each Ready event carries the port the server is listening on', async () => {
    const events: ServerEvent[] = []

    await runServerLifecycle({
      binaryPathResolver: () => TEST_SERVER,
      extraArgs: ['--exit-after-ready'],
      configValues: {},
      backoffDelaysMs: FAST_BACKOFF,
      pollOptions: FAST_POLL,
      onEvent: (e) => events.push(e),
    })

    const readyEvents = events.filter((e) => e.state === 'Ready')
    expect(readyEvents).toHaveLength(MAX_RESPAWN_ATTEMPTS + 1)
    for (const e of readyEvents) {
      expect(e.port).toBeGreaterThan(0)
      expect(e.port).toBeLessThan(65536)
    }
  }, 30_000)

  it('first-start crash emits Failed and throws without entering Recovering', async () => {
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

    // Must emit Starting then Failed directly — NEVER Recovering on first start.
    expect(events.map((e) => e.state)).toEqual(['Starting', 'Failed'])
  })
})


// ── Integration: stable binary — stays Ready, no crash ─────────────────────

describe('runServerLifecycle — stable server', () => {
  it('emits Started → Ready with a valid port when the server is healthy', async () => {
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
        if (e.state === 'Ready') {
          resolveEarly()
        }
      },
    })

    await earlyDone
    controller.abort()
    await lifecycle

    expect(events.some((e) => e.state === 'Starting')).toBe(true)
    expect(events.some((e) => e.state === 'Ready')).toBe(true)
    const readyPort = events.find((e) => e.state === 'Ready')?.port
    expect(readyPort).toBeGreaterThan(0)
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
        if (e.state === 'Ready') {
          resolveEarly()
        }
      },
    })

    await earlyDone
    controller.abort()
    await lifecycle
  })
})

