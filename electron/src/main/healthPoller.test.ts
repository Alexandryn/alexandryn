import { describe, expect, it } from 'vitest'
import { POLL_INTERVAL_MS, READINESS_TIMEOUT_MS } from './healthPoller'

// Readiness polling constants.
// Integration tests (real poll against the test binary) live in lifecycle.test.ts.

describe('healthPoller constants', () => {
  it('POLL_INTERVAL_MS is 250ms — frequent enough for near-instant ready transition', () => {
    expect(POLL_INTERVAL_MS).toBe(250)
  })

  it('READINESS_TIMEOUT_MS is 15 000ms — default readiness timeout floor', () => {
    expect(READINESS_TIMEOUT_MS).toBe(15_000)
  })
})
