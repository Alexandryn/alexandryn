import { deriveSeed, type CoverSeed } from '../../lib/fnv1a'

// Session-scoped (module-level, cleared only by a full page reload) —
// correctness is guaranteed by fnv1a's own determinism (FR-2): the same
// identifier always derives the same seed, so caching introduces no
// staleness risk. Not localStorage/sessionStorage — nothing here needs to
// survive a reload, only to avoid recomputing within one page's lifetime.
const cache = new Map<string, CoverSeed>()

export function getCachedSeed(identifier: string): CoverSeed {
  const cached = cache.get(identifier)
  if (cached) return cached
  const seed = deriveSeed(identifier)
  cache.set(identifier, seed)
  return seed
}

/** Test-only: reset the cache between test cases so they don't leak state into each other. */
export function __clearSeedCacheForTests(): void {
  cache.clear()
}
