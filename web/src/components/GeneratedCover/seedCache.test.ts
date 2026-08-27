import { afterEach, describe, expect, it, vi } from 'vitest'
import * as fnv1aModule from '../../lib/fnv1a'
import { __clearSeedCacheForTests, getCachedSeed } from './seedCache'

afterEach(() => {
  __clearSeedCacheForTests()
  vi.restoreAllMocks()
})

describe('getCachedSeed', () => {
  it('computes the seed on first call', () => {
    const expected = fnv1aModule.deriveSeed('work-1') // computed before spying, so it isn't itself counted
    const spy = vi.spyOn(fnv1aModule, 'deriveSeed')
    const seed = getCachedSeed('work-1')
    expect(spy).toHaveBeenCalledTimes(1)
    expect(seed).toEqual(expected)
  })

  it('reuses the cached value on a repeated call — proven by call count, not just output equality', () => {
    const spy = vi.spyOn(fnv1aModule, 'deriveSeed')
    getCachedSeed('work-1')
    getCachedSeed('work-1')
    getCachedSeed('work-1')
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('computes a fresh seed for a different identifier', () => {
    const spy = vi.spyOn(fnv1aModule, 'deriveSeed')
    getCachedSeed('work-1')
    getCachedSeed('work-2')
    expect(spy).toHaveBeenCalledTimes(2)
  })

  it('returns the same seed value deriveSeed itself would produce', () => {
    expect(getCachedSeed('work-99')).toEqual(fnv1aModule.deriveSeed('work-99'))
  })
})
