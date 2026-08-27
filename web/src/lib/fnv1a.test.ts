import { describe, expect, it } from 'vitest'
import { deriveSeed, fnv1a } from './fnv1a'

describe('fnv1a', () => {
  it('is deterministic — the same string always hashes to the same number', () => {
    expect(fnv1a('work-123')).toBe(fnv1a('work-123'))
  })

  it('produces different hashes for different inputs (no collision on this set)', () => {
    const inputs = ['work-1', 'work-2', 'edition-1', 'edition-2', '', 'a', 'aa']
    const hashes = new Set(inputs.map(fnv1a))
    expect(hashes.size).toBe(inputs.length)
  })

  it('returns an unsigned 32-bit integer', () => {
    const hash = fnv1a('some-identifier')
    expect(hash).toBeGreaterThanOrEqual(0)
    expect(hash).toBeLessThanOrEqual(0xffffffff)
    expect(Number.isInteger(hash)).toBe(true)
  })
})

describe('deriveSeed', () => {
  it('is deterministic — the same identifier always derives the same seed', () => {
    expect(deriveSeed('work-123')).toEqual(deriveSeed('work-123'))
  })

  it('derives a hue within a valid CSS hue range', () => {
    const { hue } = deriveSeed('work-123')
    expect(hue).toBeGreaterThanOrEqual(0)
    expect(hue).toBeLessThan(360)
  })

  it('derives a pattern from the fixed known set', () => {
    const { pattern } = deriveSeed('work-123')
    expect(['flat', 'diagonal-stripe', 'dot-grid']).toContain(pattern)
  })

  it('derives visibly different seeds for different identifiers', () => {
    const seeds = new Set(
      ['work-1', 'work-2', 'work-3', 'work-4', 'work-5'].map(
        (id) => `${deriveSeed(id).hue}:${deriveSeed(id).pattern}`,
      ),
    )
    expect(seeds.size).toBeGreaterThan(1)
  })
})
