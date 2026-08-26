import { describe, expect, it } from 'vitest'
import { deriveScale } from './deriveScale.ts'

describe('deriveScale', () => {
  it('keeps values at or above the occurrence threshold, sorted ascending, named by size', () => {
    const counts = { 20: 15, 8: 100, 2: 50, 40: 1, 12: 30 }

    // threshold 5 excludes 40 (count 1)
    expect(deriveScale(counts, 5)).toEqual([
      { name: 'xs', px: 2, count: 50 },
      { name: 'sm', px: 8, count: 100 },
      { name: 'md', px: 12, count: 30 },
      { name: 'lg', px: 20, count: 15 },
    ])
  })

  it('falls back past the named-label sequence with numeric suffixes if there are more values than labels', () => {
    const counts = Object.fromEntries(Array.from({ length: 9 }, (_, i) => [i + 1, 10]))

    const scale = deriveScale(counts, 5)

    expect(scale.map((t) => t.name)).toEqual([
      '3xs',
      '2xs',
      'xs',
      'sm',
      'md',
      'lg',
      'xl',
      '2xl',
      '3xl',
    ])
  })

  it('returns an empty scale when nothing meets the threshold', () => {
    expect(deriveScale({ 4: 1, 8: 2 }, 5)).toEqual([])
  })
})
