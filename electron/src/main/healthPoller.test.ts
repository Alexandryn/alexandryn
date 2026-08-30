import { describe, expect, it } from 'vitest'
import { POLL_INTERVAL_MS, READINESS_TIMEOUT_MS } from './healthPoller'

// desktop-host-process-model.md FR-3 — readiness polling constants.
// Integration tests (real poll against the test binary) live in lifecycle.test.ts.

describe('healthPoller constants (FR-3)', () => {
  it('POLL_INTERVAL_MS is 250ms — frequent enough for near-instant ready transition', () => {
    expect(POLL_INTERVAL_MS).toBe(250)
  })

  it('READINESS_TIMEOUT_MS is 15 000ms — the placeholder value named in the spec', () => {
    // Explicitly a named placeholder (desktop-host-process-model.md FR-3
    // Open questions). Phase 05 implementation should measure a real cold
    // start before replacing this number.
    expect(READINESS_TIMEOUT_MS).toBe(15_000)
  })
})
